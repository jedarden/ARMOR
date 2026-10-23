// Package crypto provides encryption, decryption, and key management for ARMOR.
package crypto

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

var (
	// ErrInvalidBlockTableEntry is returned when a block table entry is malformed.
	ErrInvalidBlockTableEntry = errors.New("invalid block table entry")
	// ErrCiphertextTooLarge is returned when ciphertext length exceeds blockSize (raw blocks).
	ErrCiphertextTooLarge = errors.New("ciphertext too large")
	// ErrInvalidTableSize is returned when the block table size doesn't match expected block count.
	ErrInvalidTableSize = errors.New("invalid table size")
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
		return nil, fmt.Errorf("expected %d bytes, got %d: %w", BlockTableEntrySize, len(data), ErrInvalidBlockTableEntry)
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
		return fmt.Errorf("ciphertext length %d exceeds block size %d: %w", rawLen, blockSize, ErrCiphertextTooLarge)
	}

	// Compressed blocks should also be reasonable (compressed data can be larger than input for incompressible data)
	// We allow up to blockSize + overhead for compressed blocks
	if e.IsCompressed() && int(rawLen) > blockSize+1024 {
		return fmt.Errorf("compressed ciphertext length %d exceeds reasonable limit: %w", rawLen, ErrInvalidBlockTableEntry)
	}

	// Ciphertext length must be non-zero
	if rawLen == 0 {
		return fmt.Errorf("ciphertext length cannot be zero: %w", ErrInvalidBlockTableEntry)
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
	Entries   []*BlockTableEntry
	BlockSize int // Block size in bytes
	// offsets[i] is the cumulative ciphertext byte offset of block i within
	// the ciphertext section (offsets[0] == 0). Summed in 64-bit: the
	// concatenated ciphertext of a large object exceeds 4GiB, which silently
	// wraps a uint32 accumulator and addresses the wrong bytes.
	offsets []int64
}

// NewBlockTable creates a new block table.
func NewBlockTable(blockSize int, expectedEntries int) *BlockTable {
	return &BlockTable{
		Entries:   make([]*BlockTableEntry, 0, expectedEntries),
		BlockSize: blockSize,
		// offsets[0] is always 0: the offset before the first block.
		offsets: make([]int64, 1, expectedEntries+1),
	}
}

// AddEntry appends a new entry to the block table.
func (t *BlockTable) AddEntry(entry *BlockTableEntry) error {
	if err := entry.Validate(t.BlockSize); err != nil {
		return err
	}

	t.Entries = append(t.Entries, entry)
	t.offsets = appendOffset(t.offsets, entry.RawLength())
	return nil
}

// Encode serializes the entire block table to bytes.
func (t *BlockTable) Encode() ([]byte, error) {
	if len(t.Entries) == 0 {
		return nil, fmt.Errorf("cannot encode empty block table: %w", ErrInvalidBlockTableEntry)
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
		return nil, fmt.Errorf("empty block table data: %w", ErrInvalidBlockTableEntry)
	}

	// Verify table size matches expected block count
	entryCount := len(data) / BlockTableEntrySize
	if entryCount != int(expectedBlockCount) {
		return nil, fmt.Errorf("expected %d entries (%d bytes), got %d entries (%d bytes): %w",
			expectedBlockCount, expectedBlockCount*BlockTableEntrySize, entryCount, len(data), ErrInvalidTableSize)
	}

	if len(data)%BlockTableEntrySize != 0 {
		return nil, fmt.Errorf("table size %d is not a multiple of entry size %d: %w",
			len(data), BlockTableEntrySize, ErrInvalidBlockTableEntry)
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
		table.offsets = appendOffset(table.offsets, entry.RawLength())
	}

	return table, nil
}

// appendOffset appends the cumulative offset that follows a block of the
// given raw length. The last element is always the total ciphertext length.
func appendOffset(offsets []int64, rawLength uint32) []int64 {
	var prev int64
	if n := len(offsets); n > 0 {
		prev = offsets[n-1]
	}
	return append(offsets, prev+int64(rawLength))
}

// GetPrefixSums returns the cumulative ciphertext offsets of every block,
// plus the total ciphertext length as the final element: element i is the
// byte offset of block i within the ciphertext section, and element
// len(result)-1 is one past the last block's last byte. Summed in 64-bit:
// offsets above 4GiB silently wrap a uint32 accumulator.
func (t *BlockTable) GetPrefixSums() []int64 {
	// Return a copy to prevent external modification
	sums := make([]int64, len(t.offsets))
	copy(sums, t.offsets)
	return sums
}

// CiphertextSpan returns the half-open ciphertext byte span [start, end)
// covering blocks startBlock..endBlock inclusive. ok is false when either
// index is out of range.
func (t *BlockTable) CiphertextSpan(startBlock, endBlock int) (start, end int64, ok bool) {
	if startBlock < 0 || endBlock < startBlock || endBlock >= len(t.Entries) {
		return 0, 0, false
	}
	return t.offsets[startBlock], t.offsets[endBlock+1], true
}

// BlockOffset returns the ciphertext byte offset for a given block index.
// Returns (offset, true) on success; (0, false) if blockIndex is out of range
// or the offset is not representable in uint32 (ciphertext section above
// 4GiB — use CiphertextSpan for those objects instead of guessing).
func (t *BlockTable) BlockOffset(blockIndex uint32) (uint32, bool) {
	if int(blockIndex) >= len(t.Entries) {
		return 0, false
	}

	offset := t.offsets[blockIndex]
	if offset > math.MaxUint32 {
		return 0, false
	}
	return uint32(offset), true
}

// TotalCiphertextLength returns the total length of all ciphertext blocks.
// It saturates at math.MaxUint32 rather than silently wrapping; use
// TotalCiphertextLength64 for objects whose ciphertext section can exceed
// 4GiB.
func (t *BlockTable) TotalCiphertextLength() uint32 {
	total := t.TotalCiphertextLength64()
	if total > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(total)
}

// TotalCiphertextLength64 returns the total length of all ciphertext blocks
// in 64 bits.
func (t *BlockTable) TotalCiphertextLength64() int64 {
	if len(t.offsets) == 0 {
		return 0
	}
	return t.offsets[len(t.offsets)-1]
}

// BlockRange returns the (offset, length) for a given block within the ciphertext section.
// Returns (offset, length, true) on success; (0, 0, false) if blockIndex is out of range
// or the offset is not representable in uint32 (see BlockOffset).
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
