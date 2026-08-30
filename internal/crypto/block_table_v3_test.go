package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestBlockTableEntry_EncodeDecodeRoundTrip(t *testing.T) {
	// Test uncompressed entry
	hmac1 := [32]byte{1, 2, 3, 4}
	entry1 := NewBlockTableEntry(hmac1, 65536, false)

	encoded1, err := entry1.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded1, err := DecodeBlockTableEntry(encoded1)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded1.HMAC != entry1.HMAC {
		t.Errorf("HMAC mismatch: got %x, want %x", decoded1.HMAC, entry1.HMAC)
	}

	if decoded1.CiphertextLength != entry1.CiphertextLength {
		t.Errorf("CiphertextLength mismatch: got %d, want %d", decoded1.CiphertextLength, entry1.CiphertextLength)
	}

	if decoded1.IsCompressed() {
		t.Error("Entry should not be marked as compressed")
	}
}

func TestBlockTableEntry_CompressionFlag(t *testing.T) {
	hmac := [32]byte{1, 2, 3, 4}

	// Test compressed entry
	entryCompressed := NewBlockTableEntry(hmac, 32768, true)

	if !entryCompressed.IsCompressed() {
		t.Error("Entry should be marked as compressed")
	}

	// RawLength should return the length without the flag
	if entryCompressed.RawLength() != 32768 {
		t.Errorf("RawLength mismatch: got %d, want %d", entryCompressed.RawLength(), 32768)
	}

	// CiphertextLength should have the high bit set
	if entryCompressed.CiphertextLength&CompressionFlagBit == 0 {
		t.Error("Compression flag bit should be set")
	}

	// Round-trip the compressed entry
	encoded, err := entryCompressed.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeBlockTableEntry(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !decoded.IsCompressed() {
		t.Error("Decoded entry should retain compressed flag")
	}

	if decoded.RawLength() != 32768 {
		t.Errorf("Decoded RawLength mismatch: got %d, want %d", decoded.RawLength(), 32768)
	}
}

func TestBlockTableEntry_ValidateRawBlock(t *testing.T) {
	blockSize := 65536
	hmac := [32]byte{1, 2, 3, 4}

	// Valid raw block (exactly blockSize)
	entry1 := NewBlockTableEntry(hmac, uint32(blockSize), false)
	if err := entry1.Validate(blockSize); err != nil {
		t.Errorf("Validate failed for valid block: %v", err)
	}

	// Valid raw block (smaller than blockSize)
	entry2 := NewBlockTableEntry(hmac, 32768, false)
	if err := entry2.Validate(blockSize); err != nil {
		t.Errorf("Validate failed for valid smaller block: %v", err)
	}

	// Invalid raw block (larger than blockSize)
	entry3 := NewBlockTableEntry(hmac, uint32(blockSize+1), false)
	if err := entry3.Validate(blockSize); err == nil {
		t.Error("Validate should reject raw block exceeding block size")
	} else if err != ErrCiphertextTooLarge {
		t.Errorf("Validate should return ErrCiphertextTooLarge, got: %v", err)
	}
}

func TestBlockTableEntry_ValidateCompressedBlock(t *testing.T) {
	blockSize := 65536
	hmac := [32]byte{1, 2, 3, 4}

	// Valid compressed block (smaller than blockSize)
	entry1 := NewBlockTableEntry(hmac, 32768, true)
	if err := entry1.Validate(blockSize); err != nil {
		t.Errorf("Validate failed for valid compressed block: %v", err)
	}

	// Compressed block can be larger than input but should be reasonable
	entry2 := NewBlockTableEntry(hmac, uint32(blockSize+512), true)
	if err := entry2.Validate(blockSize); err != nil {
		t.Errorf("Validate failed for reasonably sized compressed block: %v", err)
	}

	// Invalid compressed block (unreasonably large)
	entry3 := NewBlockTableEntry(hmac, uint32(blockSize+1024+1), true)
	if err := entry3.Validate(blockSize); err == nil {
		t.Error("Validate should reject unreasonably large compressed block")
	}
}

func TestBlockTableEntry_ValidateZeroLength(t *testing.T) {
	hmac := [32]byte{1, 2, 3, 4}
	entry := NewBlockTableEntry(hmac, 0, false)

	if err := entry.Validate(65536); err == nil {
		t.Error("Validate should reject zero-length ciphertext")
	}
}

func TestBlockTableEntry_InvalidDecode(t *testing.T) {
	// Too short
	shortData := []byte{1, 2, 3}
	if _, err := DecodeBlockTableEntry(shortData); err == nil {
		t.Error("Decode should reject too-short input")
	} else if err != ErrInvalidBlockTableEntry {
		t.Errorf("Decode should return ErrInvalidBlockTableEntry, got: %v", err)
	}

	// Exact entry size (valid)
	validData := make([]byte, BlockTableEntrySize)
	if _, err := DecodeBlockTableEntry(validData); err != nil {
		t.Errorf("Decode should accept exact entry size: %v", err)
	}
}

func TestBlockTable_EncodeDecode(t *testing.T) {
	blockSize := 65536
	expectedCount := 3

	table := NewBlockTable(blockSize, expectedCount)

	// Add three entries
	for i := 0; i < expectedCount; i++ {
		var hmac [32]byte
		hmac[0] = byte(i)
		entry := NewBlockTableEntry(hmac, uint32(blockSize), false)
		if err := table.AddEntry(entry); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	// Encode
	encoded, err := table.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	expectedSize := expectedCount * BlockTableEntrySize
	if len(encoded) != expectedSize {
		t.Errorf("Encoded size mismatch: got %d, want %d", len(encoded), expectedSize)
	}

	// Decode
	decoded, err := DecodeBlockTable(encoded, blockSize, uint32(expectedCount))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.EntryCount() != expectedCount {
		t.Errorf("Entry count mismatch: got %d, want %d", decoded.EntryCount(), expectedCount)
	}

	// Verify entries match
	for i := 0; i < expectedCount; i++ {
		if decoded.Entries[i].HMAC[0] != byte(i) {
			t.Errorf("Entry %d HMAC mismatch", i)
		}
	}
}

func TestBlockTable_CompressedAndMixedEntries(t *testing.T) {
	blockSize := 65536
	table := NewBlockTable(blockSize, 3)

	// Add mixed compressed/uncompressed entries
	var hmac1, hmac2, hmac3 [32]byte
	hmac1[0] = 1
	hmac2[0] = 2
	hmac3[0] = 3

	table.AddEntry(NewBlockTableEntry(hmac1, 65536, false)) // Raw, full block
	table.AddEntry(NewBlockTableEntry(hmac2, 32768, true))  // Compressed
	table.AddEntry(NewBlockTableEntry(hmac3, 16384, false)) // Raw, partial block

	encoded, err := table.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeBlockTable(encoded, blockSize, 3)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify compression flags preserved
	if decoded.Entries[0].IsCompressed() {
		t.Error("Entry 0 should not be compressed")
	}
	if !decoded.Entries[1].IsCompressed() {
		t.Error("Entry 1 should be compressed")
	}
	if decoded.Entries[2].IsCompressed() {
		t.Error("Entry 2 should not be compressed")
	}
}

func TestBlockTable_EmptyTable(t *testing.T) {
	table := NewBlockTable(65536, 0)

	if _, err := table.Encode(); err == nil {
		t.Error("Encode should reject empty table")
	}

	emptyData := []byte{}
	if _, err := DecodeBlockTable(emptyData, 65536, 0); err == nil {
		t.Error("Decode should reject empty data")
	}
}

func TestBlockTable_InvalidSize(t *testing.T) {
	blockSize := 65536

	// Create data with wrong entry count
	wrongCountData := make([]byte, 2*BlockTableEntrySize) // 2 entries worth of data

	if _, err := DecodeBlockTable(wrongCountData, blockSize, 3); err == nil {
		t.Error("Decode should reject mismatched entry count")
	} else if err != ErrInvalidTableSize {
		t.Errorf("Decode should return ErrInvalidTableSize, got: %v", err)
	}

	// Truncated entry (not a multiple of entry size)
	truncatedData := make([]byte, BlockTableEntrySize+10)
	if _, err := DecodeBlockTable(truncatedData, blockSize, 1); err == nil {
		t.Error("Decode should reject truncated entry")
	}
}

func TestBlockTable_InvalidEntryDuringDecode(t *testing.T) {
	blockSize := 65536

	// Create a valid table, then corrupt the HMAC of first entry
	validData := make([]byte, 3*BlockTableEntrySize)
	for i := range validData {
		validData[i] = 0xFF
	}

	// This should pass initial size check but fail during entry decode/validation
	// because the zero HMAC will pass, but we need to test validation failures

	// Create data that would fail validation (ciphertext > blockSize for raw block)
	_ = NewBlockTable(blockSize, 1) // Unused but required for test setup
	var hmac [32]byte
	badEntry := NewBlockTableEntry(hmac, uint32(blockSize+1), false)
	badData, _ := badEntry.Encode()

	tableData := make([]byte, BlockTableEntrySize)
	copy(tableData, badData)

	if _, err := DecodeBlockTable(tableData, blockSize, 1); err == nil {
		t.Error("Decode should reject entry with invalid ciphertext length")
	}
}

func TestBlockTable_PrefixSums(t *testing.T) {
	blockSize := 65536
	table := NewBlockTable(blockSize, 4)

	// Add entries with varying lengths
	lengths := []uint32{65536, 32768, 49152, 16384}
	for i, length := range lengths {
		var hmac [32]byte
		hmac[0] = byte(i)
		entry := NewBlockTableEntry(hmac, length, false)
		if err := table.AddEntry(entry); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	// Test GetPrefixSums
	sums := table.GetPrefixSums()
	if len(sums) != 4 {
		t.Errorf("PrefixSums length mismatch: got %d, want 4", len(sums))
	}

	for i, length := range lengths {
		if sums[i] != length {
			t.Errorf("PrefixSum[%d] mismatch: got %d, want %d", i, sums[i], length)
		}
	}

	// Test BlockOffset
	tests := []struct {
		blockIndex uint32
		wantOffset uint32
		wantOK     bool
	}{
		{0, 0, true},
		{1, 65536, true},
		{2, 65536 + 32768, true},
		{3, 65536 + 32768 + 49152, true},
		{4, 0, false}, // Out of range
	}

	for _, tt := range tests {
		offset, ok := table.BlockOffset(tt.blockIndex)
		if ok != tt.wantOK {
			t.Errorf("BlockOffset(%d) ok mismatch: got %v, want %v", tt.blockIndex, ok, tt.wantOK)
		}
		if ok && offset != tt.wantOffset {
			t.Errorf("BlockOffset(%d) mismatch: got %d, want %d", tt.blockIndex, offset, tt.wantOffset)
		}
	}

	// Test BlockRange
	for i := range lengths {
		var wantOffset uint32
		for j := 0; j < i; j++ {
			wantOffset += lengths[j]
		}

		offset, length, ok := table.BlockRange(uint32(i))
		if !ok {
			t.Errorf("BlockRange(%d) returned false", i)
			continue
		}
		if offset != wantOffset {
			t.Errorf("BlockRange(%d) offset mismatch: got %d, want %d", i, offset, wantOffset)
		}
		if length != lengths[i] {
			t.Errorf("BlockRange(%d) length mismatch: got %d, want %d", i, length, lengths[i])
		}
	}
}

func TestBlockTable_TotalCiphertextLength(t *testing.T) {
	blockSize := 65536
	table := NewBlockTable(blockSize, 3)

	// Empty table
	if table.TotalCiphertextLength() != 0 {
		t.Errorf("TotalCiphertextLength for empty table should be 0, got %d", table.TotalCiphertextLength())
	}

	// Add entries
	var hmac1, hmac2, hmac3 [32]byte
	table.AddEntry(NewBlockTableEntry(hmac1, 65536, false))
	table.AddEntry(NewBlockTableEntry(hmac2, 32768, true))
	table.AddEntry(NewBlockTableEntry(hmac3, 16384, false))

	expected := uint32(65536 + 32768 + 16384)
	if table.TotalCiphertextLength() != expected {
		t.Errorf("TotalCiphertextLength mismatch: got %d, want %d", table.TotalCiphertextLength(), expected)
	}
}

func TestBlockTable_AddEntryValidation(t *testing.T) {
	blockSize := 65536
	table := NewBlockTable(blockSize, 1)

	var hmac [32]byte

	// Valid entry
	validEntry := NewBlockTableEntry(hmac, 32768, false)
	if err := table.AddEntry(validEntry); err != nil {
		t.Errorf("AddEntry failed for valid entry: %v", err)
	}

	// Invalid entry (exceeds block size, raw)
	invalidEntry := NewBlockTableEntry(hmac, uint32(blockSize+1), false)
	if err := table.AddEntry(invalidEntry); err == nil {
		t.Error("AddEntry should reject entry exceeding block size")
	}

	// Zero-length entry
	zeroEntry := NewBlockTableEntry(hmac, 0, false)
	if err := table.AddEntry(zeroEntry); err == nil {
		t.Error("AddEntry should reject zero-length entry")
	}
}

func TestBlockTableEntry_RandomRoundTrip(t *testing.T) {
	// Test with random HMAC values
	for i := 0; i < 100; i++ {
		var hmac [32]byte
		if _, err := rand.Read(hmac[:]); err != nil {
			t.Fatalf("rand.Read failed: %v", err)
		}

		// Random length between 1 and 65536
		length := uint32(1 + (i * 65535 / 99))
		compressed := (i % 2) == 0

		entry := NewBlockTableEntry(hmac, length, compressed)

		encoded, err := entry.Encode()
		if err != nil {
			t.Fatalf("Encode failed for iteration %d: %v", i, err)
		}

		decoded, err := DecodeBlockTableEntry(encoded)
		if err != nil {
			t.Fatalf("Decode failed for iteration %d: %v", i, err)
		}

		if decoded.HMAC != entry.HMAC {
			t.Errorf("HMAC mismatch at iteration %d", i)
		}

		if decoded.CiphertextLength != entry.CiphertextLength {
			t.Errorf("CiphertextLength mismatch at iteration %d", i)
		}

		if decoded.IsCompressed() != entry.IsCompressed() {
			t.Errorf("Compression flag mismatch at iteration %d", i)
		}

		if decoded.RawLength() != entry.RawLength() {
			t.Errorf("RawLength mismatch at iteration %d", i)
		}
	}
}

func TestBlockTable_RoundTripWithTable(t *testing.T) {
	blockSize := 65536
	expectedCount := 10

	originalTable := NewBlockTable(blockSize, expectedCount)

	// Add random entries
	for i := 0; i < expectedCount; i++ {
		var hmac [32]byte
		if _, err := rand.Read(hmac[:]); err != nil {
			t.Fatalf("rand.Read failed: %v", err)
		}

		length := uint32(4096 * (i + 1)) // Varying sizes
		compressed := (i % 3) == 0       // Every third entry compressed

		entry := NewBlockTableEntry(hmac, length, compressed)
		if err := originalTable.AddEntry(entry); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	// Encode
	encoded, err := originalTable.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decodedTable, err := DecodeBlockTable(encoded, blockSize, uint32(expectedCount))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify all entries match
	if decodedTable.EntryCount() != expectedCount {
		t.Fatalf("Entry count mismatch: got %d, want %d", decodedTable.EntryCount(), expectedCount)
	}

	for i := 0; i < expectedCount; i++ {
		orig := originalTable.Entries[i]
		dec := decodedTable.Entries[i]

		if !bytes.Equal(orig.HMAC[:], dec.HMAC[:]) {
			t.Errorf("Entry %d HMAC mismatch", i)
		}

		if orig.CiphertextLength != dec.CiphertextLength {
			t.Errorf("Entry %d CiphertextLength mismatch", i)
		}

		if orig.IsCompressed() != dec.IsCompressed() {
			t.Errorf("Entry %d compression flag mismatch", i)
		}
	}

	// Verify prefix sums match
	origSums := originalTable.GetPrefixSums()
	decSums := decodedTable.GetPrefixSums()

	if len(origSums) != len(decSums) {
		t.Errorf("Prefix sums length mismatch")
	}

	for i := range origSums {
		if origSums[i] != decSums[i] {
			t.Errorf("Prefix sum %d mismatch: got %d, want %d", i, decSums[i], origSums[i])
		}
	}

	// Verify total length
	if originalTable.TotalCiphertextLength() != decodedTable.TotalCiphertextLength() {
		t.Errorf("Total ciphertext length mismatch")
	}
}
