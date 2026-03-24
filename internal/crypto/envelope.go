// Package crypto provides encryption, decryption, and key management for ARMOR.
package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const (
	// Magic is the ARMOR envelope magic bytes.
	Magic = "ARMR"

	// HeaderSize is the fixed size of the envelope header.
	// Layout: Magic(4) + Version(1) + BlockSizeLog2(1) + IV(16) + PlaintextSize(8) + PlaintextSHA(32) = 62 bytes
	// Padded to 64 bytes for alignment.
	HeaderSize = 64

	// HMACSize is the size of each HMAC-SHA256 entry.
	HMACSize = 32

	// Version1 is the current envelope format version.
	Version1 = 0x01

	// DefaultBlockSize is the default encryption block size.
	DefaultBlockSize = 65536
)

var (
	ErrInvalidMagic    = errors.New("invalid ARMOR magic")
	ErrInvalidVersion  = errors.New("unsupported envelope version")
	ErrInvalidHeader   = errors.New("invalid envelope header")
	ErrBlockSizePower  = errors.New("block size must be power of 2")
	ErrHMACMismatch    = errors.New("HMAC verification failed")
	ErrInvalidBlock    = errors.New("invalid block index")
	ErrPlaintextMismatch = errors.New("plaintext SHA-256 mismatch")
)

// EnvelopeHeader represents the fixed 64-byte envelope header.
type EnvelopeHeader struct {
	Magic         [4]byte // "ARMR"
	Version       uint8   // 0x01
	BlockSizeLog2 uint8   // log2(block_size), e.g., 16 for 64KB
	IV            [16]byte
	PlaintextSize uint64
	PlaintextSHA  [32]byte // SHA-256 of plaintext before encryption
	Reserved      [2]byte  // Reserved for future use (pad to 64 bytes)
}

// Encode serializes the header to a 64-byte buffer.
func (h *EnvelopeHeader) Encode() ([]byte, error) {
	if string(h.Magic[:]) != Magic {
		return nil, ErrInvalidMagic
	}

	buf := make([]byte, HeaderSize)
	offset := 0

	copy(buf[offset:], h.Magic[:])
	offset += 4

	buf[offset] = h.Version
	offset++

	buf[offset] = h.BlockSizeLog2
	offset++

	copy(buf[offset:], h.IV[:])
	offset += 16

	binary.LittleEndian.PutUint64(buf[offset:], h.PlaintextSize)
	offset += 8

	copy(buf[offset:], h.PlaintextSHA[:])
	// offset += 32 would be 62, plus 2 reserved = 64

	return buf, nil
}

// DecodeHeader parses a 64-byte buffer into an EnvelopeHeader.
func DecodeHeader(data []byte) (*EnvelopeHeader, error) {
	if len(data) < HeaderSize {
		return nil, ErrInvalidHeader
	}

	h := &EnvelopeHeader{}
	offset := 0

	copy(h.Magic[:], data[offset:offset+4])
	offset += 4

	if string(h.Magic[:]) != Magic {
		return nil, ErrInvalidMagic
	}

	h.Version = data[offset]
	offset++

	if h.Version != Version1 {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidVersion, h.Version)
	}

	h.BlockSizeLog2 = data[offset]
	offset++

	copy(h.IV[:], data[offset:offset+16])
	offset += 16

	h.PlaintextSize = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	copy(h.PlaintextSHA[:], data[offset:offset+32])

	return h, nil
}

// BlockSize returns the actual block size in bytes.
func (h *EnvelopeHeader) BlockSize() int {
	return 1 << h.BlockSizeLog2
}

// BlockCount returns the number of blocks in the encrypted data.
func (h *EnvelopeHeader) BlockCount() uint32 {
	return ComputeBlockCount(int64(h.PlaintextSize), h.BlockSize())
}

// EncryptedSize returns the total size of the encrypted object on B2.
func (h *EnvelopeHeader) EncryptedSize() int64 {
	blockCount := h.BlockCount()
	return int64(HeaderSize) + int64(blockCount)*int64(h.BlockSize()) + int64(blockCount)*HMACSize
}

// HMACTableOffset returns the byte offset of the HMAC table in the encrypted object.
// Since AES-CTR preserves plaintext length, the encrypted data has the same size as plaintext.
func (h *EnvelopeHeader) HMACTableOffset() int64 {
	return int64(HeaderSize) + int64(h.PlaintextSize)
}

// PlaintextSHA256Hex returns the hex-encoded plaintext SHA-256.
func (h *EnvelopeHeader) PlaintextSHA256Hex() string {
	return hex.EncodeToString(h.PlaintextSHA[:])
}

// IVHex returns the hex-encoded IV.
func (h *EnvelopeHeader) IVHex() string {
	return hex.EncodeToString(h.IV[:])
}

// ComputeBlockCount calculates the number of blocks for a given plaintext size.
func ComputeBlockCount(plaintextSize int64, blockSize int) uint32 {
	blocks := plaintextSize / int64(blockSize)
	if plaintextSize%int64(blockSize) != 0 {
		blocks++
	}
	return uint32(blocks)
}

// ComputePlaintextSHA256 computes the SHA-256 hash of plaintext data.
func ComputePlaintextSHA256(plaintext []byte) [32]byte {
	return sha256.Sum256(plaintext)
}

// ComputePlaintextSHA256Stream computes SHA-256 from a reader.
func ComputePlaintextSHA256Stream(r io.Reader) ([32]byte, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return [32]byte{}, err
	}
	var sum [32]byte
	h.Sum(sum[:0])
	return sum, nil
}

// ReadEnvelopeHeader reads and parses an envelope header from a reader.
func ReadEnvelopeHeader(r io.Reader) (*EnvelopeHeader, error) {
	headerBuf := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, headerBuf); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}
	return DecodeHeader(headerBuf)
}

// NewEnvelopeHeader creates a new envelope header.
func NewEnvelopeHeader(iv []byte, plaintextSize int64, blockSize int, plaintextSHA [32]byte) (*EnvelopeHeader, error) {
	if len(iv) != 16 {
		return nil, errors.New("IV must be 16 bytes")
	}

	// Validate block size is power of 2
	if blockSize < 4096 || (blockSize&(blockSize-1)) != 0 {
		return nil, ErrBlockSizePower
	}

	// Calculate log2 of block size
	blockSizeLog2 := uint8(0)
	for bs := blockSize; bs > 1; bs >>= 1 {
		blockSizeLog2++
	}

	h := &EnvelopeHeader{
		Version:       Version1,
		BlockSizeLog2: blockSizeLog2,
		PlaintextSize: uint64(plaintextSize),
		PlaintextSHA:  plaintextSHA,
	}
	copy(h.Magic[:], Magic)
	copy(h.IV[:], iv)

	return h, nil
}

// VerifyPlaintextSHA verifies the plaintext against the stored SHA-256.
func (h *EnvelopeHeader) VerifyPlaintextSHA(plaintext []byte) error {
	computed := sha256.Sum256(plaintext)
	if !bytes.Equal(computed[:], h.PlaintextSHA[:]) {
		return ErrPlaintextMismatch
	}
	return nil
}
