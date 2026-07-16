package crypto

import (
	"bytes"
	"testing"
)

// TestMultipartDecryptWithSidecar tests the full multipart decrypt path
// using the sidecar HMAC table, which is how production handles multipart objects.
func TestMultipartDecryptWithSidecar(t *testing.T) {
	blockSize := 65536 // 64KB
	partSize := int64(5 * 1024 * 1024) // 5MB = 80 blocks per part
	numParts := 4

	// Generate test DEK and IV
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	for i := range dek { dek[i] = byte(i) }
	for i := range iv { iv[i] = byte(i + 10) }

	encryptor, err := NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create and encrypt each part, collecting HMACs
	var allEncrypted []byte
	var allHMACs []byte
	var allPlaintext []byte

	for partNum := 0; partNum < numParts; partNum++ {
		startBlockIndex := uint32(partNum * int(partSize / int64(blockSize)))

		// Create part plaintext with distinguishable pattern
		partPlaintext := make([]byte, partSize)
		for i := range partPlaintext {
			partPlaintext[i] = byte(startBlockIndex + uint32(i/blockSize))
		}
		allPlaintext = append(allPlaintext, partPlaintext...)

		// Encrypt with absolute block indexing
		encrypted, hmacTable, err := encryptor.EncryptWithStartingCounter(partPlaintext, startBlockIndex)
		if err != nil {
			t.Fatalf("Failed to encrypt part %d: %v", partNum+1, err)
		}

		t.Logf("Part %d: blocks %d-%d (%d bytes, %d HMACs)",
			partNum+1, startBlockIndex, startBlockIndex+uint32(len(encrypted)/blockSize)-1,
			len(encrypted), len(hmacTable)/HMACSize)

		allEncrypted = append(allEncrypted, encrypted...)
		allHMACs = append(allHMACs, hmacTable...)
	}

	totalBlocks := len(allEncrypted) / blockSize
	t.Logf("Total: %d blocks, %d bytes encrypted, %d HMACs", totalBlocks, len(allEncrypted), len(allHMACs)/HMACSize)

	// Create decryptor
	decryptor, err := NewDecryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("Failed to create decryptor: %v", err)
	}

	// Test 1: Decrypt the entire object using the concatenated HMAC table
	decrypted, err := decryptor.Decrypt(allEncrypted, allHMACs)
	if err != nil {
		t.Fatalf("Failed to decrypt entire object: %v", err)
	}

	if !bytes.Equal(decrypted, allPlaintext) {
		t.Errorf("Decrypted content doesn't match plaintext")
	}
	t.Logf("✓ Full decryption successful: %d bytes", len(decrypted))

	// Test 2: Decrypt block 256 specifically (this is where the production bug occurred)
	// Block 256 would be the first block of part 4 (blocks 240-319)
	if totalBlocks > 256 {
		testBlock := 256
		start := testBlock * blockSize
		end := start + blockSize
		if end > len(allEncrypted) {
			end = len(allEncrypted)
		}

		encryptedBlock := allEncrypted[start:end]
		hmacOffset := testBlock * HMACSize
		expectedHMAC := allHMACs[hmacOffset : hmacOffset+HMACSize]

		// Verify HMAC for block 256
		err := decryptor.verifyBlockHMAC(encryptedBlock, uint32(testBlock), expectedHMAC)
		if err != nil {
			t.Errorf("Block %d HMAC verification failed: %v", testBlock, err)
		} else {
			t.Logf("✓ Block %d HMAC verified successfully", testBlock)
		}

		// Decrypt just this block using DecryptRange (production path)
		blockPlaintext, err := decryptor.DecryptRange(encryptedBlock, allHMACs, int64(start), int64(end-1), int64(len(allPlaintext)), true)
		if err != nil {
			t.Errorf("Failed to decrypt block %d: %v", testBlock, err)
		} else {
			t.Logf("✓ Block %d decrypted successfully: %d bytes", testBlock, len(blockPlaintext))

			// Verify the content matches
			expectedBlockContent := allPlaintext[start:end]
			if !bytes.Equal(blockPlaintext, expectedBlockContent) {
				t.Errorf("Block %d content mismatch", testBlock)
			}
		}
	}

	// Test 3: Decrypt across part boundaries (block 79-80, 159-160, 239-240)
	boundaryBlocks := []struct {
		before, after int
	}{
		{79, 80},   // Part 1-2 boundary
		{159, 160}, // Part 2-3 boundary
		{239, 240}, // Part 3-4 boundary
	}

	for _, boundary := range boundaryBlocks {
		for _, blockNum := range []int{boundary.before, boundary.after} {
			if blockNum >= totalBlocks {
				continue
			}

			start := blockNum * blockSize
			end := start + blockSize
			encryptedBlock := allEncrypted[start:end]
			hmacOffset := blockNum * HMACSize
			expectedHMAC := allHMACs[hmacOffset : hmacOffset+HMACSize]

			err := decryptor.verifyBlockHMAC(encryptedBlock, uint32(blockNum), expectedHMAC)
			if err != nil {
				t.Errorf("Block %d (boundary test) HMAC verification failed: %v", blockNum, err)
			}
		}
		t.Logf("✓ Boundary %d-%d verified", boundary.before, boundary.after)
	}

	// Test 4: DecryptRange to simulate a range request
	// Request a range that spans part boundaries
	rangeStart := int64(10 * 1024 * 1024) // 10MB (middle of part 3)
	rangeEnd := int64(12 * 1024 * 1024)    // 12MB (middle of part 4)

	// Calculate encrypted range (like production does)
	rangeBlockStart := rangeStart / int64(blockSize)
	rangeBlockEnd := rangeEnd / int64(blockSize)
	encStart := rangeBlockStart * int64(blockSize)
	encEnd := (rangeBlockEnd + 1) * int64(blockSize)
	if encEnd > int64(len(allEncrypted)) {
		encEnd = int64(len(allEncrypted))
	}
	encryptedRange := allEncrypted[encStart:encEnd]

	rangeDecrypted, err := decryptor.DecryptRange(encryptedRange, allHMACs, rangeStart, rangeEnd, int64(len(allPlaintext)), true)
	if err != nil {
		t.Errorf("DecryptRange failed: %v", err)
	} else {
		expectedRangeContent := allPlaintext[rangeStart:rangeEnd+1]
		if !bytes.Equal(rangeDecrypted, expectedRangeContent) {
			t.Errorf("Range decrypted content mismatch")
		} else {
			t.Logf("✓ DecryptRange successful: bytes %d-%d (%d bytes)", rangeStart, rangeEnd, len(rangeDecrypted))
		}
	}
}

// TestMultipartDecryptStream tests stream decryption with multipart HMACs
func TestMultipartDecryptStream(t *testing.T) {
	blockSize := 65536
	partSize := int64(5 * 1024 * 1024)
	numParts := 3

	dek := make([]byte, 32)
	iv := make([]byte, 16)
	for i := range dek { dek[i] = byte(i) }
	for i := range iv { iv[i] = byte(i + 10) }

	encryptor, err := NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create and encrypt all parts
	var allEncrypted []byte
	var allHMACs []byte
	var allPlaintext []byte

	for partNum := 0; partNum < numParts; partNum++ {
		startBlockIndex := uint32(partNum * int(partSize / int64(blockSize)))

		partPlaintext := make([]byte, partSize)
		for i := range partPlaintext {
			partPlaintext[i] = byte(startBlockIndex + uint32(i/blockSize))
		}
		allPlaintext = append(allPlaintext, partPlaintext...)

		encrypted, hmacTable, err := encryptor.EncryptWithStartingCounter(partPlaintext, startBlockIndex)
		if err != nil {
			t.Fatalf("Failed to encrypt part %d: %v", partNum+1, err)
		}

		allEncrypted = append(allEncrypted, encrypted...)
		allHMACs = append(allHMACs, hmacTable...)
	}

	decryptor, err := NewDecryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("Failed to create decryptor: %v", err)
	}

	// Stream decrypt
	totalBlocks := len(allEncrypted) / blockSize
	var plaintextBuf bytes.Buffer

	err = decryptor.DecryptStream(bytes.NewReader(allEncrypted), &plaintextBuf, allHMACs, totalBlocks)
	if err != nil {
		t.Fatalf("DecryptStream failed: %v", err)
	}

	decrypted := plaintextBuf.Bytes()
	if !bytes.Equal(decrypted, allPlaintext) {
		t.Errorf("Stream decrypted content doesn't match plaintext")
	} else {
		t.Logf("✓ Stream decryption successful: %d bytes", len(decrypted))
	}
}

// TestHMACTablePositioning verifies that HMACs are stored in the correct
// position in the table (absolute block indexing).
func TestHMACTablePositioning(t *testing.T) {
	blockSize := 65536
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	for i := range dek { dek[i] = byte(i) }
	for i := range iv { iv[i] = byte(i + 10) }

	encryptor, err := NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Encrypt 3 parts
	partSize := int64(5 * 1024 * 1024)
	var allHMACs []byte

	for partNum := 0; partNum < 3; partNum++ {
		startBlockIndex := uint32(partNum * int(partSize / int64(blockSize)))

		partPlaintext := make([]byte, partSize)
		_, hmacTable, err := encryptor.EncryptWithStartingCounter(partPlaintext, startBlockIndex)
		if err != nil {
			t.Fatalf("Failed to encrypt part %d: %v", partNum+1, err)
		}

		// Verify that HMAC at position 0 of this part's table is for startBlockIndex
		hmacForFirstBlock := hmacTable[:HMACSize]

		// Verify they match by checking that this HMAC works for the first block
		decryptor, _ := NewDecryptor(dek, iv, blockSize)

		// Encrypt a single block to get the encrypted data for verification
		singleBlock := make([]byte, blockSize)
		singleEncrypted, _, err := encryptor.EncryptWithStartingCounter(singleBlock, startBlockIndex)
		if err != nil {
			t.Fatalf("Failed to encrypt single block: %v", err)
		}

		err = decryptor.verifyBlockHMAC(singleEncrypted, startBlockIndex, hmacForFirstBlock)
		if err != nil {
			t.Errorf("Part %d: HMAC at position 0 doesn't match HMAC for absolute block %d", partNum+1, startBlockIndex)
		}

		allHMACs = append(allHMACs, hmacTable...)
	}

	// Verify the concatenated table has HMACs in the right order
	// Position N should contain HMAC for block N
	testBlocks := []int{0, 79, 80, 159, 160, 239}
	decryptor, _ := NewDecryptor(dek, iv, blockSize)

	for _, blockNum := range testBlocks {
		// Encrypt a single block at this index to get expected HMAC
		singleBlock := make([]byte, blockSize)
		singleEncrypted, singleHmacTable, err := encryptor.EncryptWithStartingCounter(singleBlock, uint32(blockNum))
		if err != nil {
			t.Fatalf("Failed to encrypt block %d: %v", blockNum, err)
		}
		expectedHmac := singleHmacTable[:HMACSize]

		// Get HMAC from concatenated table at position blockNum
		hmacOffset := blockNum * HMACSize
		actualHmac := allHMACs[hmacOffset : hmacOffset+HMACSize]

		if !bytes.Equal(actualHmac, expectedHmac) {
			t.Errorf("HMAC at position %d doesn't match expected HMAC for block %d", blockNum, blockNum)
		}

		// Verify it actually works
		err = decryptor.verifyBlockHMAC(singleEncrypted, uint32(blockNum), actualHmac)
		if err != nil {
			t.Errorf("Failed to verify block %d with HMAC from position %d: %v", blockNum, blockNum, err)
		}

		// Verify it actually works
		err = decryptor.verifyBlockHMAC(singleEncrypted, uint32(blockNum), actualHmac)
		if err != nil {
			t.Errorf("Failed to verify block %d with HMAC from position %d: %v", blockNum, blockNum, err)
		}
	}

	t.Logf("✓ HMAC table positioning verified for blocks: %v", testBlocks)
}
