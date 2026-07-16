package crypto

import (
	"fmt"
	"testing"
)

// TestMultipartHMACAbsoluteIndexing verifies that EncryptWithStartingCounter
// computes HMACs with absolute block indices, not relative indices.
// This is a regression test for the bug tracked in bf-1v6skf.
func TestMultipartHMACAbsoluteIndexing(t *testing.T) {
	blockSize := 65536 // 64KB
	partSize := int64(5 * 1024 * 1024) // 5MB = 80 blocks
	
	// Generate a test DEK and IV
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	for i := range dek { dek[i] = byte(i) }
	for i := range iv { iv[i] = byte(i) }
	
	encryptor, err := NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}
	
	// Test Part 1 (blocks 0-79)
	plaintext1 := make([]byte, partSize)
	encrypted1, hmacTable1, err := encryptor.EncryptWithStartingCounter(plaintext1, 0)
	if err != nil {
		t.Fatalf("Failed to encrypt part 1: %v", err)
	}
	
	// Test Part 2 (blocks 80-159)
	plaintext2 := make([]byte, partSize)
	encrypted2, hmacTable2, err := encryptor.EncryptWithStartingCounter(plaintext2, 80)
	if err != nil {
		t.Fatalf("Failed to encrypt part 2: %v", err)
	}
	
	// Test Part 3 (blocks 160-239)
	plaintext3 := make([]byte, partSize)
	encrypted3, hmacTable3, err := encryptor.EncryptWithStartingCounter(plaintext3, 160)
	if err != nil {
		t.Fatalf("Failed to encrypt part 3: %v", err)
	}
	
	// Verify that each HMAC table has the correct number of HMACs
	expectedHMACsPerPart := int(partSize / int64(blockSize))
	if len(hmacTable1)/HMACSize != expectedHMACsPerPart {
		t.Errorf("Part 1 HMAC count mismatch: got %d, want %d", len(hmacTable1)/HMACSize, expectedHMACsPerPart)
	}
	if len(hmacTable2)/HMACSize != expectedHMACsPerPart {
		t.Errorf("Part 2 HMAC count mismatch: got %d, want %d", len(hmacTable2)/HMACSize, expectedHMACsPerPart)
	}
	if len(hmacTable3)/HMACSize != expectedHMACsPerPart {
		t.Errorf("Part 3 HMAC count mismatch: got %d, want %d", len(hmacTable3)/HMACSize, expectedHMACsPerPart)
	}
	
	// Simulate CompleteMultipartUpload: concatenate all HMACs
	allHMACs := append(append(hmacTable1, hmacTable2...), hmacTable3...)
	allEncrypted := append(append(encrypted1, encrypted2...), encrypted3...)
	
	t.Logf("Simulated multipart upload:")
	t.Logf("  Part 1: blocks 0-79 (5MB)")
	t.Logf("  Part 2: blocks 80-159 (5MB)")
	t.Logf("  Part 3: blocks 160-239 (5MB)")
	t.Logf("  Total: %d blocks, %d bytes encrypted", len(allEncrypted)/blockSize, len(allEncrypted))
	
	// Create decryptor and verify HMACs for blocks across part boundaries
	decryptor, err := NewDecryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("Failed to create decryptor: %v", err)
	}
	
	// Test blocks at various positions
	testBlocks := []struct {
		blockIndex int
		partNum    int
		description string
	}{
		{0, 1, "first block of part 1"},
		{79, 1, "last block of part 1"},
		{80, 2, "first block of part 2"},
		{159, 2, "last block of part 2"},
		{160, 3, "first block of part 3"},
		{239, 3, "last block of part 3"},
	}
	
	for _, tb := range testBlocks {
		// Extract the encrypted block
		start := tb.blockIndex * blockSize
		end := start + blockSize
		if end > len(allEncrypted) {
			end = len(allEncrypted)
		}
		encryptedBlock := allEncrypted[start:end]
		
		// Get the expected HMAC from the concatenated table
		hmacOffset := tb.blockIndex * HMACSize
		expectedHMAC := allHMACs[hmacOffset : hmacOffset+HMACSize]
		
		// Verify the HMAC
		err := decryptor.verifyBlockHMAC(encryptedBlock, uint32(tb.blockIndex), expectedHMAC)
		if err != nil {
			t.Errorf("Block %d (%s) HMAC verification failed: %v", tb.blockIndex, tb.description, err)
		} else {
			t.Logf("✓ Block %d (%s) HMAC verified", tb.blockIndex, tb.description)
		}
	}
	
	// Decrypt and verify the entire concatenated data
	plaintext, err := decryptor.Decrypt(allEncrypted, allHMACs)
	if err != nil {
		t.Fatalf("Failed to decrypt concatenated parts: %v", err)
	}
	
	// Verify the plaintext matches
	expectedPlaintext := append(append(plaintext1, plaintext2...), plaintext3...)
	if len(plaintext) != len(expectedPlaintext) {
		t.Errorf("Decrypted length mismatch: got %d, want %d", len(plaintext), len(expectedPlaintext))
	}
	
	// Spot check some bytes across part boundaries
	checkPoints := []struct {
		offset    int
		partNum   int
		blockInPart int
	}{
		{0, 1, 0},
		{int(partSize - 1), 1, 79},
		{int(partSize), 2, 0},
		{int(2*partSize - 1), 2, 79},
		{int(2*partSize), 3, 0},
		{int(3*partSize - 1), 3, 79},
	}
	
	for _, cp := range checkPoints {
		if plaintext[cp.offset] != expectedPlaintext[cp.offset] {
			t.Errorf("Plaintext mismatch at offset %d (part %d, block %d): got 0x%02x, want 0x%02x",
				cp.offset, cp.partNum, cp.blockInPart, plaintext[cp.offset], expectedPlaintext[cp.offset])
		}
	}
	
	t.Logf("✓ All HMAC verifications passed across part boundaries")
	t.Logf("✓ Decryption produced correct plaintext across all parts")
	
	fmt.Printf("\n=== Multipart HMAC Fix Verification ===\n")
	fmt.Printf("Part 1: blocks 0-79, HMACs computed for absolute indices 0-79\n")
	fmt.Printf("Part 2: blocks 80-159, HMACs computed for absolute indices 80-159\n")
	fmt.Printf("Part 3: blocks 160-239, HMACs computed for absolute indices 160-239\n")
	fmt.Printf("After concatenation: position N contains HMAC for block N\n")
	fmt.Printf("✓ HMAC verification succeeds across part boundaries!\n")
}
