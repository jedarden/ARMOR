// Package crypto provides encryption, decryption, and key management for ARMOR.
package crypto

import (
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ReplayableEnvelopeReader produces the single-PUT envelope stream —
// header || encrypted blocks || trailer table — lazily from a seekable
// plaintext source. The bytes are identical to writing the same plaintext
// through the envelope header followed by EncryptStream (v2) or
// EncryptV3Stream (v3), because block encryption is stateless per block:
// each block's counter binds its index (armor-213bd392).
//
// The streaming PUT path uses this to hand the backend a bounded
// io.ReadSeeker instead of an unseekable pipe. B2 requires a known
// Content-Length (the transport must not fall back to chunked encoding)
// and the SDK rewinds a seeker to retry, so a seekable body uploads
// directly with no second full-object ciphertext spool and no encryption
// goroutine. A forced retry replays byte-identical ciphertext from the
// same key, IV, and plaintext spool.
//
// The only cross-block state is the trailer table. Entries are captured
// as the forward pass encrypts each block and backfilled on demand if the
// trailer region is read first. The plaintext source is not owned: the
// caller closes it.
//
// The same struct also serves v3 multipart parts through
// NewReplayablePartReader: a part stream is the ciphertext blocks alone —
// no header, no trailer — encrypted with the (part, block) counter that
// EncryptPartV3 uses, so the bytes match the buffered part path exactly
// while encryption stays lazy under the backend's read (armor-cb2b0731).
type ReplayableEnvelopeReader struct {
	plaintext io.ReadSeeker

	header        []byte
	blockSize     int
	plaintextSize int64
	blockCount    uint32
	version       uint8

	// partStream serves a v3 multipart part: block counters carry partNumber
	// and the served stream stops after the ciphertext (no header, no
	// trailer). The per-block HMACs are still recorded, for PartBlockHMACs.
	partStream bool
	partNumber uint16

	enc     *Encryptor      // v2 per-segment encryption
	cryptor *V3BlockCryptor // v3 per-block encryption

	entries   [][]byte // encoded trailer-table entries, filled lazily
	tableDone bool

	pos           int64
	plaintextRead int64
	segValid      bool
	segBase       uint32 // block index currently held in segBuf
	segBuf        []byte
}

// NewReplayableEnvelopeReader builds the replayable encrypted source for a
// single-PUT upload. plaintext must be a seekable spool of exactly
// plaintextSize bytes; header is the encoded envelope header. The returned
// reader's stream length equals the envelope size the handler computes from
// the same inputs.
func NewReplayableEnvelopeReader(plaintext io.ReadSeeker, header []byte, dek, iv []byte, blockSize int, plaintextSize int64, version uint8) (*ReplayableEnvelopeReader, error) {
	if version != Version2 && version != Version3 {
		return nil, fmt.Errorf("unsupported envelope version %d", version)
	}
	if plaintext == nil || blockSize <= 0 || plaintextSize < 0 {
		return nil, errors.New("invalid replayable envelope source or size")
	}
	blockCount := ComputeBlockCount(plaintextSize, blockSize)
	r := &ReplayableEnvelopeReader{
		plaintext:     plaintext,
		header:        header,
		blockSize:     blockSize,
		plaintextSize: plaintextSize,
		blockCount:    blockCount,
		version:       version,
	}
	switch version {
	case Version2:
		enc, err := NewEncryptorWithVersion(dek, iv, blockSize, Version2)
		if err != nil {
			return nil, fmt.Errorf("v2 encryptor: %w", err)
		}
		r.enc = enc
	case Version3:
		cryptor, err := NewV3BlockCryptor(dek, iv, blockSize)
		if err != nil {
			return nil, fmt.Errorf("v3 block cryptor: %w", err)
		}
		r.cryptor = cryptor
	}
	return r, nil
}

// NewReplayablePartReader builds the replayable encrypted source for one v3
// multipart part. plaintext must be a seekable spool of exactly plaintextSize
// bytes; the served stream is the part's ciphertext alone (v3 part ciphertext
// has the same length as its plaintext), encrypted with the (partNumber,
// block) counter construction of EncryptPartV3. Because the source is
// seekable, a backend SDK retry rewinds and replays byte-identical ciphertext.
// Block HMACs are recorded as the stream is read and retrieved afterwards
// with PartBlockHMACs; a block the backend never reached is backfilled from
// the spool there.
func NewReplayablePartReader(plaintext io.ReadSeeker, dek, iv []byte, blockSize int, plaintextSize int64, partNumber uint16) (*ReplayableEnvelopeReader, error) {
	if partNumber < 1 {
		return nil, fmt.Errorf("part number must be >= 1, got %d", partNumber)
	}
	if plaintext == nil || blockSize <= 0 || plaintextSize < 0 {
		return nil, errors.New("invalid replayable part source or size")
	}
	cryptor, err := NewV3BlockCryptor(dek, iv, blockSize)
	if err != nil {
		return nil, fmt.Errorf("v3 block cryptor: %w", err)
	}
	// An empty part has no blocks; unlike a v3 envelope (whose trailer table
	// cannot encode zero blocks), a part stream of zero length is well-formed
	// and matches EncryptPartV3 on empty input.
	return &ReplayableEnvelopeReader{
		plaintext:     plaintext,
		blockSize:     blockSize,
		plaintextSize: plaintextSize,
		blockCount:    ComputeBlockCount(plaintextSize, blockSize),
		version:       Version3,
		partStream:    true,
		partNumber:    partNumber,
		cryptor:       cryptor,
	}, nil
}

// entrySize is the per-block trailer-table entry width for the version.
func (r *ReplayableEnvelopeReader) entrySize() int {
	if r.version == Version3 {
		return BlockTableEntrySize
	}
	return HMACSize
}

// total is the exact stream length: header + ciphertext blocks + trailer,
// or for a part stream the ciphertext alone.
func (r *ReplayableEnvelopeReader) total() int64 {
	if r.partStream {
		return r.plaintextSize
	}
	return int64(len(r.header)) + r.plaintextSize + int64(r.blockCount)*int64(r.entrySize())
}

// Len returns the total number of bytes Read serves between rewinds.
func (r *ReplayableEnvelopeReader) Len() int64 { return r.total() }

// PlaintextBytesRead reports how many bytes were consumed from the
// plaintext spool across all passes (serve plus any trailer backfill).
func (r *ReplayableEnvelopeReader) PlaintextBytesRead() int64 { return r.plaintextRead }

// Read implements io.Reader over the envelope stream.
func (r *ReplayableEnvelopeReader) Read(p []byte) (int, error) {
	total := r.total()
	if r.pos >= total {
		return 0, io.EOF
	}
	written := 0
	for written < len(p) && r.pos < total {
		headerEnd := int64(len(r.header))
		dataEnd := headerEnd + r.plaintextSize
		switch {
		case r.pos < headerEnd:
			n := copy(p[written:], r.header[r.pos:])
			r.pos += int64(n)
			written += n
		case r.pos < dataEnd:
			bi := uint32((r.pos - headerEnd) / int64(r.blockSize))
			inOff := int(r.pos - headerEnd - int64(bi)*int64(r.blockSize))
			if !r.segValid || r.segBase != bi {
				if err := r.loadSegment(bi); err != nil {
					return written, err
				}
			}
			n := copy(p[written:], r.segBuf[inOff:])
			r.pos += int64(n)
			written += n
		default:
			if err := r.ensureTable(); err != nil {
				return written, err
			}
			entry := r.entrySize()
			tOff := r.pos - dataEnd
			ei := int(tOff / int64(entry))
			eOff := int(tOff % int64(entry))
			n := copy(p[written:], r.entries[ei][eOff:])
			r.pos += int64(n)
			written += n
		}
	}
	return written, nil
}

// Seek implements io.Seeker. Positions past the end are clamped to the
// end so a subsequent Read returns io.EOF.
func (r *ReplayableEnvelopeReader) Seek(offset int64, whence int) (int64, error) {
	var target int64
	switch whence {
	case io.SeekStart:
		target = offset
	case io.SeekCurrent:
		target = r.pos + offset
	case io.SeekEnd:
		target = r.total() + offset
	default:
		return 0, fmt.Errorf("invalid whence %d", whence)
	}
	if target < 0 {
		return 0, errors.New("seek before start of envelope stream")
	}
	if target > r.total() {
		target = r.total()
	}
	r.pos = target
	r.segValid = false
	return r.pos, nil
}

// loadSegment encrypts block bi from the plaintext spool into segBuf and
// records its trailer-table entry.
func (r *ReplayableEnvelopeReader) loadSegment(bi uint32) error {
	off := int64(bi) * int64(r.blockSize)
	n := int64(r.blockSize)
	if off+n > r.plaintextSize {
		n = r.plaintextSize - off
	}
	if int64(cap(r.segBuf)) < n {
		r.segBuf = make([]byte, n)
	}
	blk := r.segBuf[:n]
	if _, err := r.plaintext.Seek(off, io.SeekStart); err != nil {
		return fmt.Errorf("plaintext seek to block %d: %w", bi, err)
	}
	if _, err := io.ReadFull(r.plaintext, blk); err != nil {
		return fmt.Errorf("plaintext read at block %d: %w", bi, err)
	}
	r.plaintextRead += n
	switch r.version {
	case Version2:
		if err := r.enc.checkCounterSpaceWithStart(1, bi); err != nil {
			return fmt.Errorf("block %d counter: %w", bi, err)
		}
		cipher.NewCTR(r.enc.block, r.enc.makeCounter(bi)).XORKeyStream(blk, blk)
		r.recordEntry(bi, r.enc.computeBlockHMAC(blk, bi), 0)
	case Version3:
		blockHMAC := make([]byte, HMACSize)
		if err := r.cryptor.encryptBlockInto(blk, blockHMAC, r.partNumber, bi, blk); err != nil {
			return fmt.Errorf("block %d encryption: %w", bi, err)
		}
		r.recordEntry(bi, blockHMAC, uint32(len(blk)))
	}
	r.segBuf = blk
	r.segBase = bi
	r.segValid = true
	return nil
}

// recordEntry stores block bi's encoded trailer-table entry, growing the
// table as needed; nil holes are backfilled by ensureTable.
func (r *ReplayableEnvelopeReader) recordEntry(bi uint32, hmac []byte, clen uint32) {
	for int(bi) >= len(r.entries) {
		r.entries = append(r.entries, nil)
	}
	entry := make([]byte, r.entrySize())
	copy(entry, hmac)
	if r.version == Version3 {
		binary.BigEndian.PutUint32(entry[HMACSize:], clen)
	}
	r.entries[bi] = entry
}

// ensureTable backfills any trailer entries the forward pass has not yet
// produced, by encrypting the missing blocks and discarding the ciphertext.
func (r *ReplayableEnvelopeReader) ensureTable() error {
	if r.tableDone {
		return nil
	}
	for bi := 0; bi < int(r.blockCount); bi++ {
		if bi < len(r.entries) && r.entries[bi] != nil {
			continue
		}
		if err := r.loadSegment(uint32(bi)); err != nil {
			return fmt.Errorf("trailer backfill at block %d: %w", bi, err)
		}
	}
	r.tableDone = true
	return nil
}

// PartBlockHMACs returns the per-block HMACs of a part stream, in block
// order, after making sure every block's entry exists. A backend that
// consumed the whole body has already produced them all; any block it did
// not reach is re-encrypted from the spool here, deterministically. The
// caller must keep the plaintext spool alive until after this call.
func (r *ReplayableEnvelopeReader) PartBlockHMACs() ([][]byte, error) {
	if !r.partStream {
		return nil, errors.New("PartBlockHMACs requires a part stream")
	}
	if err := r.ensureTable(); err != nil {
		return nil, err
	}
	hmacs := make([][]byte, int(r.blockCount))
	for bi := 0; bi < int(r.blockCount); bi++ {
		hmacs[bi] = make([]byte, HMACSize)
		copy(hmacs[bi], r.entries[bi][:HMACSize])
	}
	return hmacs, nil
}
