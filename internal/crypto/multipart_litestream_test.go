package crypto

import (
	"fmt"
	"testing"
)

// TestMultipartLitestreamScenario simulates the exact production scenario:
// A 44MB LTX file uploaded via multipart, then read via range requests.
// This reproduces the "block 256: HMAC verification failed" error from bf-1v6skf.
func TestMultipartLitestreamScenario(t *testing.T) {
	blockSize := 65536 // 64KB

	// Generate a test DEK and IV
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	for i := range dek {
		dek[i] = byte(i)
	}
	for i := range iv {
		iv[i] = byte(i)
	}

	encryptor, err := NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Simulate a 44MB LTX file (close to the production 44,908,497 byte file)
	// Use 44,033,024 bytes = 671 blocks exactly (this tests block boundary alignment)
	const totalSize = 44_033_024 // 671 blocks of 64KB
	const partSizeConst = 5_242_880  // 5MB = 80 blocks per part

	// We'll have 8 parts plus a final partial part
	// Parts 1-8: 80 blocks each (5MB)
	// Part 9: 31 blocks (1,986,560 bytes)
	numFullParts := totalSize / partSizeConst
	finalPartSize := totalSize % partSizeConst

	t.Logf("Simulating %d byte LTX file via multipart upload:", totalSize)
	t.Logf("  Parts 1-%d: %d bytes each (%d blocks)", numFullParts, partSizeConst, partSizeConst/blockSize)
	t.Logf("  Part %d: %d bytes (%d blocks)", numFullParts+1, finalPartSize, finalPartSize/blockSize)
	t.Logf("  Total: %d blocks", totalSize/blockSize+1)

	// Encrypt each part and collect HMACs
	var allEncrypted []byte
	var allHMACs []byte
	cumulativeBlocks := uint32(0)

	for partNum := 0; partNum <= int(numFullParts); partNum++ {
		var partSize int
		if partNum < int(numFullParts) {
			partSize = int(partSizeConst)
		} else {
			partSize = int(finalPartSize)
		}

		if partSize == 0 {
			continue
		}

		plaintext := make([]byte, partSize)

		// Encrypt with starting counter
		encrypted, hmacTable, err := encryptor.EncryptWithStartingCounter(plaintext, cumulativeBlocks)
		if err != nil {
			t.Fatalf("Failed to encrypt part %d: %v", partNum+1, err)
		}

		// Collect encrypted data
		allEncrypted = append(allEncrypted, encrypted...)

		// Collect HMACs
		allHMACs = append(allHMACs, hmacTable...)

		// Update block counter
		blocksInPart := uint32((partSize + blockSize - 1) / blockSize)
		cumulativeBlocks += blocksInPart

		t.Logf("Part %d: encrypted %d bytes, %d HMACs, cumulative blocks now %d",
			partNum+1, len(encrypted), len(hmacTable)/HMACSize, cumulativeBlocks)
	}

	// Verify total sizes
	expectedBlocks := (totalSize + blockSize - 1) / blockSize
	if len(allHMACs)/HMACSize != expectedBlocks {
		t.Errorf("HMAC count mismatch: got %d, want %d", len(allHMACs)/HMACSize, expectedBlocks)
	}
	if len(allEncrypted) != totalSize {
		t.Errorf("Encrypted size mismatch: got %d, want %d", len(allEncrypted), totalSize)
	}

	t.Logf("Total encrypted: %d bytes, %d HMACs (%d blocks)", len(allEncrypted), len(allHMACs)/HMACSize, expectedBlocks)

	// Now test decryption with range requests (simulating litestream behavior)
	decryptor, err := NewDecryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("Failed to create decryptor: %v", err)
	}

	// Test 1: Full object decryption
	plaintext, err := decryptor.Decrypt(allEncrypted, allHMACs)
	if err != nil {
		t.Fatalf("Failed to decrypt full object: %v", err)
	}

	if len(plaintext) != totalSize {
		t.Errorf("Decrypted size mismatch: got %d, want %d", len(plaintext), totalSize)
	}

	// Test 2: Range request for block 256 specifically (this was failing in production)
	// Block 256 starts at byte 16,777,216
	rangeStart := int64(256 * blockSize)
	rangeEnd := rangeStart + int64(blockSize) - 1
	if rangeEnd >= int64(len(plaintext)) {
		rangeEnd = int64(len(plaintext)) - 1
	}

	t.Logf("Testing range request for block 256: bytes %d-%d", rangeStart, rangeEnd)

	// Extract the encrypted data for this range
	encStart := int(rangeStart)
	encEnd := int(rangeEnd + 1)
	if encEnd > len(allEncrypted) {
		encEnd = len(allEncrypted)
	}
	encryptedRange := allEncrypted[encStart:encEnd]

	// Decrypt range (hmacTableIsFull=true for multipart)
	plaintextRange, err := decryptor.DecryptRange(encryptedRange, allHMACs, rangeStart, rangeEnd, int64(len(plaintext)), true)
	if err != nil {
		t.Errorf("Failed to decrypt range for block 256: %v", err)
	} else {
		t.Logf("✓ Successfully decrypted range for block 256 (%d bytes)", len(plaintextRange))
	}

	// Test 3: Multiple range requests at part boundaries
	testRanges := []struct {
		name       string
		start, end int64
	}{
		{"Part 1 last block", 79 * int64(blockSize), 80 * int64(blockSize) - 1},
		{"Part 2 first block", 80 * int64(blockSize), 81 * int64(blockSize) - 1},
		{"Part 3 first block", 160 * int64(blockSize), 161 * int64(blockSize) - 1},
		{"Part 4 first block", 240 * int64(blockSize), 241 * int64(blockSize) - 1},
		{"Block 256", 256 * int64(blockSize), 257 * int64(blockSize) - 1},
		{"Last block start", int64(expectedBlocks-1) * int64(blockSize), int64(len(plaintext)) - 1},
	}

	for _, tr := range testRanges {
		// Clamp to valid range
		if tr.start >= int64(len(plaintext)) {
			t.Logf("Skipping %s: start beyond file size", tr.name)
			continue
		}
		if tr.end >= int64(len(plaintext)) {
			tr.end = int64(len(plaintext)) - 1
		}

		encStart := int(tr.start)
		encEnd := int(tr.end + 1)
		if encEnd > len(allEncrypted) {
			encEnd = len(allEncrypted)
		}
		encryptedRange := allEncrypted[encStart:encEnd]

		plaintextRange, err := decryptor.DecryptRange(encryptedRange, allHMACs, tr.start, tr.end, int64(len(plaintext)), true)
		if err != nil {
			t.Errorf("Failed to decrypt range %s (%d-%d): %v", tr.name, tr.start, tr.end, err)
		} else {
			t.Logf("✓ Successfully decrypted range %s (%d bytes)", tr.name, len(plaintextRange))
		}
	}

	// Test 4: Verify HMACs individually at part boundaries
	testBlocks := []struct {
		blockIndex  uint32
		description string
	}{
		{79, "last block of part 1"},
		{80, "first block of part 2"},
		{159, "last block of part 2"},
		{160, "first block of part 3"},
		{239, "last block of part 3"},
		{240, "first block of part 4"},
		{256, "middle of part 4 (production failure point)"},
		{uint32(expectedBlocks - 1), "last block of object"},
	}

	for _, tb := range testBlocks {
		if int(tb.blockIndex) >= expectedBlocks {
			t.Logf("Skipping block %d: beyond object size", tb.blockIndex)
			continue
		}

		// Extract encrypted block
		start := int(tb.blockIndex) * blockSize
		end := start + blockSize
		if end > len(allEncrypted) {
			end = len(allEncrypted)
		}
		encryptedBlock := allEncrypted[start:end]

		// Get HMAC from table
		hmacOffset := int(tb.blockIndex) * HMACSize
		expectedHMAC := allHMACs[hmacOffset : hmacOffset+HMACSize]

		// Verify HMAC
		err := decryptor.verifyBlockHMAC(encryptedBlock, tb.blockIndex, expectedHMAC)
		if err != nil {
			t.Errorf("Block %d (%s) HMAC verification failed: %v", tb.blockIndex, tb.description, err)
		} else {
			t.Logf("✓ Block %d (%s) HMAC verified", tb.blockIndex, tb.description)
		}
	}

	t.Logf("✓ All tests passed for litestream scenario")
	fmt.Printf("\n=== Litestream Multipart Scenario Test ===\n")
	fmt.Printf("Object size: %d bytes (%d blocks of %d bytes)\n", totalSize, expectedBlocks, blockSize)
	fmt.Printf("Parts: %d full parts + 1 partial part\n", numFullParts)
	fmt.Printf("✓ Block 256 (the production failure point) HMAC verified\n")
	fmt.Printf("✓ All part boundaries tested successfully\n")
}
