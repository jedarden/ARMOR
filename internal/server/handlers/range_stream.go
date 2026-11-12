package handlers

import (
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// footerTailCaptureBytes bounds how many trailing plaintext bytes a streaming
// range read retains so a read that covers the object tail can still warm the
// Parquet footer cache (armor-c3199f1c). A Parquet footer larger than this
// window is simply not cached — the read itself is unaffected. Without the
// bound, a `bytes=0-` read of a multi-GiB object would retain its whole tail
// region, which is the buffering shape this file exists to remove.
const footerTailCaptureBytes = 4 << 20

// rangeStreamWriter emits a decrypted plaintext range block by block
// (armor-31fe90c8). The three range layouts (v1/v2, v3 single-PUT, v3
// multipart) all decrypt-and-write one block at a time through this type
// instead of accumulating the whole range in memory.
//
// Headers are written lazily, when the first verified block is ready to go
// out: time-to-first-byte is the first block, and a failure in that first
// block still gets the ordinary S3 XML error response because nothing has
// been committed yet. Content-Length is the requested range length, declared
// up front, so a later integrity failure is observable: the handler stops
// writing and returns short of the declared length, and net/http closes the
// connection instead of letting the client read a truncated 206 as success —
// the same contract handleV3MultipartGet and handleFullObjectStream apply to
// streamed GET bodies. No block's plaintext is written before that block's
// HMAC has been verified.
type rangeStreamWriter struct {
	h             *Handlers
	w             http.ResponseWriter
	r             *http.Request
	bucket, key   string
	armorMeta     *backend.ARMORMetadata
	plaintextSize int64
	rangeStart    int64
	rangeEnd      int64
	lastModified  time.Time
	length        int64 // rangeEnd - rangeStart + 1
	headerWritten bool
	emitted       int64
	captureTail   bool
	tail          []byte
}

// newRangeStreamWriter builds a writer for one range response. Tail capture
// for footer caching engages only when the range reaches the object tail,
// mirroring storeFooterCacheIfTail's gating.
func (h *Handlers) newRangeStreamWriter(w http.ResponseWriter, r *http.Request, bucket, key string, armorMeta *backend.ARMORMetadata, plaintextSize, start, end int64, lastModified time.Time) *rangeStreamWriter {
	return &rangeStreamWriter{
		h:             h,
		w:             w,
		r:             r,
		bucket:        bucket,
		key:           key,
		armorMeta:     armorMeta,
		plaintextSize: plaintextSize,
		rangeStart:    start,
		rangeEnd:      end,
		lastModified:  lastModified,
		length:        end - start + 1,
		captureTail:   h.footerCache != nil && end >= plaintextSize-8,
	}
}

// emitBlock releases one verified plaintext block. blockOffset is the
// plaintext offset of the block's first byte; only the block's overlap with
// [rangeStart, rangeEnd] is written. The first call writes the 206 headers.
func (s *rangeStreamWriter) emitBlock(blockOffset int64, blockPlaintext []byte) error {
	from := max(s.rangeStart, blockOffset)
	to := min(s.rangeEnd+1, blockOffset+int64(len(blockPlaintext)))
	if from >= to {
		return nil
	}
	if !s.headerWritten {
		s.writeHeaders()
		s.headerWritten = true
	}
	chunk := blockPlaintext[from-blockOffset : to-blockOffset]
	if _, err := s.w.Write(chunk); err != nil {
		return fmt.Errorf("write plaintext at offset %d: %w", from, err)
	}
	s.emitted += int64(len(chunk))
	// Retain the trailing bytes for footer caching. Appending starts only
	// once the bytes still to come fit the capture window, so the buffer
	// never exceeds the window plus one block.
	if s.captureTail && s.emitted+int64(footerTailCaptureBytes) >= s.length {
		s.tail = append(s.tail, chunk...)
	}
	return nil
}

func (s *rangeStreamWriter) writeHeaders() {
	h := s.w.Header()
	h.Set("Content-Length", strconv.FormatInt(s.length, 10))
	h.Set("Content-Type", s.armorMeta.ContentType)
	h.Set("ETag", fmt.Sprintf(`"%s"`, s.armorMeta.ETag))
	h.Set("Accept-Ranges", "bytes")
	h.Set("Last-Modified", s.lastModified.UTC().Format(http.TimeFormat))
	h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s.rangeStart, s.rangeEnd, s.plaintextSize))
	h.Set("X-Armor-Stream", "pipelined")
	s.w.WriteHeader(http.StatusPartialContent)
}

// abort terminates a failed range stream. Before the first block has been
// released the client receives the ordinary S3 XML error; after that the
// declared Content-Length stands and the short body makes the failure
// observable to the client (connection close, not a truncated success).
func (s *rangeStreamWriter) abort(message string) {
	if !s.headerWritten {
		s.h.writeError(s.w, s.r, "InternalError", message, 500)
		return
	}
	log.Printf("GET %s/%s: streaming range aborted after %d of %d plaintext bytes: %s", s.bucket, s.key, s.emitted, s.length, message)
}

// finish completes a range: it verifies that every requested plaintext byte
// was emitted (a shortfall means the stored layout disagrees with its
// metadata — never finish such a response as success) and warms the footer
// cache from the retained tail when this read covered the object tail. It
// reports whether the response completed cleanly.
func (s *rangeStreamWriter) finish() bool {
	if !s.headerWritten || s.emitted != s.length {
		s.abort(fmt.Sprintf("range %d-%d produced %d plaintext bytes, want %d", s.rangeStart, s.rangeEnd, s.emitted, s.length))
		return false
	}
	s.h.storeFooterCacheIfTail(s.bucket, s.key, s.armorMeta, s.tail, s.plaintextSize, s.rangeEnd)
	return true
}

// streamV1V2RangeBlocks decrypts and emits a v1/v2 plaintext range one block
// at a time, replacing the buffered DecryptRange call on the GET range path.
// body must yield exactly the encrypted bytes of blocks blockStart..blockEnd
// in order. hmacTable is the whole object's table for multipart sidecars
// (absolute block indices); hmacBody is a bounded reader for single-PUT
// inline-table entries (relative order). Keeping the inline table as a reader
// avoids retaining an object-sized HMAC buffer for an unbounded range.
//
// Each block is HMAC-verified before its plaintext is released (ADR-011
// placeholder slots on non-uniform multipart boundary blocks are skipped,
// as in DecryptRange and handleFullObjectStream). A short read is an error:
// unlike the old buffered path, which zero-padded a short span, the stream
// never invents plaintext it could not authenticate.
func streamV1V2RangeBlocks(sw *rangeStreamWriter, body io.Reader, hmacTable []byte, hmacBody io.Reader, blockEnd int, hmacTableIsFull bool, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64) error {
	blockSize := armorMeta.BlockSize
	// The single-PUT caller's translation carries the correct block start,
	// while the multipart caller computes the same geometry before opening its
	// range. Derive it from the response range as a defensive guard: the
	// streamed body always begins at the first block containing rangeStart.
	blockStart := int(sw.rangeStart / int64(blockSize))

	for relIdx := 0; relIdx <= blockEnd-blockStart; relIdx++ {
		absBlockIdx := blockStart + relIdx

		remaining := plaintextSize - int64(absBlockIdx)*int64(blockSize)
		if remaining <= 0 {
			return fmt.Errorf("block %d beyond plaintext size %d", absBlockIdx, plaintextSize)
		}
		blockLen := int(min64(int64(blockSize), remaining))

		ciphertext := make([]byte, blockLen)
		n, err := io.ReadFull(body, ciphertext)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("read encrypted block %d: %w", absBlockIdx, err)
		}
		if n == 0 {
			return fmt.Errorf("encrypted data ended before block %d", absBlockIdx)
		}
		ciphertext = ciphertext[:n]

		var expectedHMAC []byte
		if hmacTableIsFull {
			hmacOffset := absBlockIdx * crypto.HMACSize
			if hmacOffset+crypto.HMACSize > len(hmacTable) {
				return fmt.Errorf("HMAC table too short for block %d", absBlockIdx)
			}
			expectedHMAC = hmacTable[hmacOffset : hmacOffset+crypto.HMACSize]
		} else {
			if hmacBody == nil {
				return fmt.Errorf("missing HMAC reader for block %d", absBlockIdx)
			}
			expectedHMAC = make([]byte, crypto.HMACSize)
			if _, err := io.ReadFull(hmacBody, expectedHMAC); err != nil {
				return fmt.Errorf("read HMAC for block %d: %w", absBlockIdx, err)
			}
		}

		if !isPlaceholderHMAC(expectedHMAC) {
			if err := verifyV1V2BlockHMAC(ciphertext, expectedHMAC, decryptor.HMACKey(), uint32(absBlockIdx)); err != nil {
				return fmt.Errorf("block %d: %w", absBlockIdx, err)
			}
		}

		decrypted := make([]byte, n)
		ctr := makeCounter(armorMeta.IV, uint32(absBlockIdx), armorMeta.Version, blockSize)
		stream := cipher.NewCTR(decryptor.CipherBlock(), ctr)
		stream.XORKeyStream(decrypted, ciphertext)

		if err := sw.emitBlock(int64(absBlockIdx)*int64(blockSize), decrypted); err != nil {
			return err
		}
	}
	return nil
}

// verifyV1V2BlockHMAC verifies the v1/v2 per-block HMAC:
// HMAC-SHA256(hmacKey, uint32be(blockIndex) || ciphertext). This is the
// stream-side equivalent of Decryptor.verifyBlockHMAC.
func verifyV1V2BlockHMAC(ciphertext, expectedHMAC, hmacKey []byte, blockIndex uint32) error {
	mac := hmac.New(sha256.New, hmacKey)
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, blockIndex)
	mac.Write(indexBytes)
	mac.Write(ciphertext)
	if !hmac.Equal(mac.Sum(nil), expectedHMAC) {
		return crypto.ErrHMACMismatch
	}
	return nil
}
