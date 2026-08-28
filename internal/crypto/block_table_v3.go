// Package crypto provides encryption, decryption, and key management for ARMOR.
package crypto

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	// ErrInvalidBlockTableEntry is returned when a block table entry is malformed.
	ErrInvalidBlockTableEntry = errors.New("invalid block table entry")
	// ErrCiphertextTooLarge is returned when ciphertext length exceeds blockSize (raw blocks).
	ErrCiphertextTooLarge = errors.New("ciphertext length exceeds block size for raw block")
	// ErrInvalidTableSize is returned when the block table size doesn't match expected block count.
	ErrInvalidTableSize = errors.New("block table size does not match expected block count")
)

const (
	// BlockTableEntrySize is the size of each block table entry in bytes.
	// Entry = hmac[32] || uint32 clen = 36 bytes
	BlockTableEntrySize = 32 + 4

	// CompressionFlagBit is the high bit of the clen field that indicates compression.
	CompressionFlagBit = 1 << 31
)

// BlockTableEntry represents a single block's HMAC and ciphertext length.
// The high bit of CiphertextLength indicates zstd compression.
type BlockTableEntry struct {
	HMAC             [32]byte // HMAC-SHA256 of the block
	CiphertextLength uint32   // Length in bytes (high bit = compressed)
}

// IsCompressed returns true if the high bit indicates zstd compression.
func (e *BlockTableEntry) IsCompressed() bool {
	return (e.CiphertextLength & CompressionFlagBit) != 0
}

// RawLength returns the ciphertext length without the compression flag.
func (e *BlockTableEntry) RawLength() uint32 {
	return e.CiphertextLength &^ CompressionFlagBit
}

// Encode serializes the entry to a 36-byte buffer.
func (e *BlockTableEntry) Encode() ([]byte, error) {
	buf := make([]byte, BlockTableEntrySize)
	offset := 0

	// Write HMAC (32 bytes)
	copy(buf[offset:], e.HMAC[:])
	offset += 32

	// Write ciphertext length with compression flag (4 bytes, big-endian)
	binary.BigEndian.PutUint32(buf[offset:], e.CiphertextLength)

	return buf, nil
}

// DecodeBlockTableEntry parses a 36-byte buffer into a BlockTableEntry.
func DecodeBlockTableEntry(data []byte) (*BlockTableEntry, error) {
	if len(data) < BlockTableEntrySize {
		return nil, fmt.Errorf("%w: expected %d bytes, got %d", ErrInvalidBlockTableEntry, BlockTableEntrySize, len(data))
	}

	e := &BlockTableEntry{}
	offset := 0

	// Read HMAC (32 bytes)
	copy(e.HMAC[:], data[offset:offset+32])
	offset += 32

	// Read ciphertext length with compression flag (4 bytes, big-endian)
	e.CiphertextLength = binary.BigEndian.Uint32(data[offset:])

	return e, nil
}

// Validate checks that the entry is well-formed.
// If the block is not compressed, ciphertext length must be <= blockSize.
func (e *BlockTableEntry) Validate(blockSize int) error {
	rawLen := e.RawLength()

	// Non-compressed blocks cannot exceed blockSize
	if !e.IsCompressed() && int(rawLen) > blockSize {
		return fmt.Errorf("%w: ciphertext length %d exceeds block size %d", ErrCiphertextTooLarge, rawLen, blockSize)
	}

	// Compressed blocks should also be reasonable (compressed data can be larger than input for incompressible data)
	// We allow up to blockSize + overhead for compressed blocks
	if e.IsCompressed() && int(rawLen) > blockSize+1024 {
		return fmt.Errorf("%w: compressed ciphertext length %d exceeds reasonable limit", ErrInvalidBlockTableEntry, rawLen)
	}

	// Ciphertext length must be non-zero
	if rawLen == 0 {
		return fmt.Errorf("%w: ciphertext length cannot be zero", ErrInvalidBlockTableEntry)
	}

	return nil
}

// NewBlockTableEntry creates a new block table entry.
// If compressed is true, the high bit of ciphertextLength will be set.
func NewBlockTableEntry(hmac [32]byte, ciphertextLength uint32, compressed bool) *BlockTableEntry {
	entry := &BlockTableEntry{
		HMAC:             hmac,
		CiphertextLength: ciphertextLength,
	}
	if compressed && ciphertextLength != 0 {
		entry.CiphertextLength |= CompressionFlagBit
	}
	return entry
}

// BlockTable represents the entire block table for an encrypted object.
// It stores entries for all blocks and provides offset mapping.
type BlockTable struct {
	Entries    []*BlockTableEntry
	BlockSize  int // Block size in bytes
	prefixSums []uint32 // Cached cumulative ciphertext lengths
}

// NewBlockTable creates a new block table.
func NewBlockTable(blockSize int, expectedEntries int) *BlockTable {
	return &BlockTable{
		Entries:    make([]*BlockTableEntry, 0, expectedEntries),
		BlockSize:  blockSize,
		prefixSums: make([]uint32, 0, expectedEntries),
	}
}

// AddEntry appends a new entry to the block table.
func (t *BlockTable) AddEntry(entry *BlockTableEntry) error {
	if err := entry.Validate(t.BlockSize); err != nil {
		return fmt.Errorf("invalid block entry: %w", err)
	}

	t.Entries = append(t.Entries, entry)
	t.prefixSums = append(t.prefixSums, entry.RawLength())
	return nil
}

// Encode serializes the entire block table to bytes.
func (t *BlockTable) Encode() ([]byte, error) {
	if len(t.Entries) == 0 {
		return nil, fmt.Errorf("%w: cannot encode empty block table", ErrInvalidBlockTableEntry)
	}

	buf := make([]byte, len(t.Entries)*BlockTableEntrySize)
	for i, entry := range t.Entries {
		encoded, err := entry.Encode()
		if err != nil {
			return nil, fmt.Errorf("encode entry %d: %w", i, err)
		}
		offset := i * BlockTableEntrySize
		copy(buf[offset:], encoded)
	}
	return buf, nil
}

// DecodeBlockTable parses a block table from bytes.
// It validates that the number of entries matches expectedBlockCount.
func DecodeBlockTable(data []byte, blockSize int, expectedBlockCount uint32) (*BlockTable, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty block table data", ErrInvalidBlockTableEntry)
	}

	// Verify table size matches expected block count
	entryCount := len(data) / BlockTableEntrySize
	if entryCount != int(expectedBlockCount) {
		return nil, fmt.Errorf("%w: expected %d entries (%d bytes), got %d entries (%d bytes)",
			ErrInvalidTableSize, expectedBlockCount, expectedBlockCount*BlockTableEntrySize, entryCount, len(data))
	}

	if len(data)%BlockTableEntrySize != 0 {
		return nil, fmt.Errorf("%w: table size %d is not a multiple of entry size %d",
			ErrInvalidBlockTableEntry, len(data), BlockTableEntrySize)
	}

	table := NewBlockTable(blockSize, entryCount)

	for i := 0; i < entryCount; i++ {
		offset := i * BlockTableEntrySize
		entry, err := DecodeBlockTableEntry(data[offset : offset+BlockTableEntrySize])
		if err != nil {
			return nil, fmt.Errorf("decode entry %d: %w", i, err)
		}

		if err := entry.Validate(blockSize); err != nil {
			return nil, fmt.Errorf("validate entry %d: %w", i, err)
		}

		table.Entries = append(table.Entries, entry)
		table.prefixSums = append(table.prefixSums, entry.RawLength())
	}

	return table, nil
}

// GetPrefixSums returns the cumulative ciphertext lengths for all blocks.
// The i-th element is the byte offset of block i within the ciphertext section.
func (t *BlockTable) GetPrefixSums() []uint32 {
	// Return a copy to prevent external modification
	sums := make([]uint32, len(t.prefixSums))
	copy(sums, t.prefixSums)
	return sums
}

// BlockOffset returns the ciphertext byte offset for a given block index.
// Returns (offset, true) on success; (0, false) if blockIndex is out of range.
func (t *BlockTable) BlockOffset(blockIndex uint32) (uint32, bool) {
	if int(blockIndex) >= len(t.prefixSums) {
		return 0, false
	}

	if blockIndex == 0 {
		return 0, true
	}

	// Sum all lengths before this block
	var sum uint32
	for i := 0; i < int(blockIndex); i++ {
		sum += t.prefixSums[i]
	}
	return sum, true
}

// TotalCiphertextLength returns the total length of all ciphertext blocks.
func (t *BlockTable) TotalCiphertextLength() uint32 {
	if len(t.prefixSums) == 0 {
		return 0
	}
	var sum uint32
	for _, length := range t.prefixSums {
		sum += length
	}
	return sum
}

// BlockRange returns the (offset, length) for a given block within the ciphertext section.
// Returns (offset, length, true) on success; (0, 0, false) if blockIndex is out of range.
func (t *BlockTable) BlockRange(blockIndex uint32) (uint32, uint32, bool) {
	if int(blockIndex) >= len(t.Entries) {
		return 0, 0, false
	}

	offset, ok := t.BlockOffset(blockIndex)
	if !ok {
		return 0, 0, false
	}

	length := t.Entries[blockIndex].RawLength()
	return offset, length, true
}

// EntryCount returns the number of entries in the block table.
func (t *BlockTable) EntryCount() int {
	return len(t.Entries)
}
