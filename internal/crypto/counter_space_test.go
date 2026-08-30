package crypto

import (
	"crypto/rand"
	"testing"
)

// TestCounterSpaceValidation tests that Version 2 encryption rejects
// objects that would overflow the 32-bit counter space.
//
// Version 2 stores blockIndex * (blockSize / 16) in a uint32.
// At 64 KiB blocks (blockSize/16 = 4096), this wraps at:
//
//	2^32 / 4096 = 2^20 blocks = 1,048,576 blocks
//	1,048,576 * 65536 bytes = 68,719,476,736 bytes = 64 GiB
func TestCounterSpaceValidation(t *testing.T) {
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}
	iv, err := GenerateIV()
	if err != nil {
		t.Fatalf("Failed to generate IV: %v", err)
	}

	blockSize := 65536 // 64 KiB
	aesBlocksPerArmorBlock := blockSize / 16

	// Calculate the maximum block count before overflow
	// counter = blockIndex * aesBlocksPerArmorBlock
	// We need: blockCount * aesBlocksPerArmorBlock < 2^32
	// So: blockCount < 2^32 / aesBlocksPerArmorBlock
	maxBlockCount := (1 << 32) / aesBlocksPerArmorBlock // 2^20 = 1,048,576

	// These subtests exercise the same finalCounterValue math as
	// materializing (maxBlockCount-1)/maxBlockCount full blocks (up to 64
	// GiB) would, but via EncryptWithStartingCounter's startBlockIndex
	// parameter and a single-block plaintext: checkCounterSpaceWithStart
	// computes finalCounterValue = (startBlockIndex+blockCount)*aesBlocksPerArmorBlock
	// purely from the block count/index, before any encryption happens, so
	// a 1-block plaintext at startBlockIndex=N-1 reaches the identical
	// finalCounterValue as N full blocks starting from 0.
	t.Run("encryption at boundary should succeed", func(t *testing.T) {
		// startBlockIndex+1 = maxBlockCount-1, so finalCounterValue is the
		// same as (maxBlockCount-1)*aesBlocksPerArmorBlock -- just under 2^32
		plaintext := make([]byte, blockSize)
		rand.Read(plaintext)

		encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version2)
		if err != nil {
			t.Fatalf("Failed to create encryptor: %v", err)
		}

		// This should succeed (counter space not exceeded)
		_, _, err = encryptor.EncryptWithStartingCounter(plaintext, uint32(maxBlockCount-2))
		if err != nil {
			t.Errorf("Encrypt with blockCount=%d should succeed but got error: %v", maxBlockCount-1, err)
		}
	})

	t.Run("encryption at overflow boundary should fail", func(t *testing.T) {
		// startBlockIndex+1 = maxBlockCount, so finalCounterValue equals
		// 2^32 exactly and would wrap
		plaintext := make([]byte, blockSize)
		rand.Read(plaintext)

		encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version2)
		if err != nil {
			t.Fatalf("Failed to create encryptor: %v", err)
		}

		// This should fail (counter space exceeded)
		_, _, err = encryptor.EncryptWithStartingCounter(plaintext, uint32(maxBlockCount-1))
		if err == nil {
			t.Errorf("Encrypt with blockCount=%d should fail but succeeded", maxBlockCount)
		}
		if err != nil && err.Error() != "object exceeds the Version 2 counter space; envelope v3 removes this limit" {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})

	t.Run("encryption well below boundary should succeed", func(t *testing.T) {
		// Test with a reasonable size (100 MiB)
		plaintextSize := int64(100 * 1024 * 1024)
		plaintext := make([]byte, plaintextSize)
		rand.Read(plaintext)

		encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version2)
		if err != nil {
			t.Fatalf("Failed to create encryptor: %v", err)
		}

		// This should succeed
		_, _, err = encryptor.Encrypt(plaintext)
		if err != nil {
			t.Errorf("Encrypt with 100 MiB should succeed but got error: %v", err)
		}
	})

	t.Run("Version1 should not have counter space limit", func(t *testing.T) {
		// Version1 doesn't use the same counter derivation, so it shouldn't
		// trigger this validation (even though it's insecure and shouldn't be used)
		plaintext := make([]byte, blockSize)
		rand.Read(plaintext)

		encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version1)
		if err != nil {
			t.Fatalf("Failed to create encryptor: %v", err)
		}

		// Version1 should not trigger the counter space check
		// (though it's insecure for other reasons)
		_, _, err = encryptor.EncryptWithStartingCounter(plaintext, uint32(maxBlockCount-1))
		// We don't assert success here because Version1 has other issues,
		// but it should NOT fail with the counter space error
		if err != nil && err.Error() == "object exceeds the Version 2 counter space; envelope v3 removes this limit" {
			t.Errorf("Version1 should not trigger Version 2 counter space check")
		}
	})
}

// TestEncryptWithStartingCounterCounterSpace tests that multipart uploads
// correctly validate counter space when using a starting counter.
func TestEncryptWithStartingCounterCounterSpace(t *testing.T) {
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}
	iv, err := GenerateIV()
	if err != nil {
		t.Fatalf("Failed to generate IV: %v", err)
	}

	blockSize := 65536
	aesBlocksPerArmorBlock := blockSize / 16
	maxBlockCount := (1 << 32) / aesBlocksPerArmorBlock

	t.Run("starting counter at boundary should fail", func(t *testing.T) {
		// Start near the boundary and try to add one more block
		startBlockIndex := uint32(maxBlockCount - 1)
		plaintext := make([]byte, int64(blockSize))
		rand.Read(plaintext)

		encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version2)
		if err != nil {
			t.Fatalf("Failed to create encryptor: %v", err)
		}

		// This should fail because startBlockIndex + 1 would overflow
		_, _, err = encryptor.EncryptWithStartingCounter(plaintext, startBlockIndex)
		if err == nil {
			t.Errorf("EncryptWithStartingCounter at block %d should fail but succeeded", startBlockIndex)
		}
		if err != nil && err.Error() != "object exceeds the Version 2 counter space; envelope v3 removes this limit" {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})

	t.Run("starting counter well below boundary should succeed", func(t *testing.T) {
		// Start at a reasonable block index
		startBlockIndex := uint32(100)
		plaintext := make([]byte, int64(10*blockSize))
		rand.Read(plaintext)

		encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version2)
		if err != nil {
			t.Fatalf("Failed to create encryptor: %v", err)
		}

		// This should succeed
		_, _, err = encryptor.EncryptWithStartingCounter(plaintext, startBlockIndex)
		if err != nil {
			t.Errorf("EncryptWithStartingCounter at block %d should succeed but got error: %v", startBlockIndex, err)
		}
	})
}

// TestCounterSpaceValidationDifferentBlockSizes tests that the counter space
// validation works correctly for different block sizes.
func TestCounterSpaceValidationDifferentBlockSizes(t *testing.T) {
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}
	iv, err := GenerateIV()
	if err != nil {
		t.Fatalf("Failed to generate IV: %v", err)
	}

	testCases := []struct {
		name      string
		blockSize int
		maxBlocks int
	}{
		{
			name:      "4 KiB blocks",
			blockSize: 4096,
			maxBlocks: (1 << 32) / (4096 / 16), // 2^32 / 256 = 16,777,216 blocks (64 GiB)
		},
		{
			name:      "64 KiB blocks",
			blockSize: 65536,
			maxBlocks: (1 << 32) / (65536 / 16), // 2^32 / 4096 = 1,048,576 blocks (64 GiB)
		},
		{
			name:      "256 KiB blocks",
			blockSize: 262144,
			maxBlocks: (1 << 32) / (262144 / 16), // 2^32 / 16384 = 262,144 blocks (64 GiB)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test at the boundary (should fail). startBlockIndex = maxBlocks-1
			// with a single-block plaintext reaches the same finalCounterValue
			// as tc.maxBlocks full blocks starting from 0, without allocating
			// up to 64 GiB (see TestCounterSpaceValidation for the full math).
			plaintext := make([]byte, tc.blockSize)
			rand.Read(plaintext)

			encryptor, err := NewEncryptorWithVersion(dek, iv, tc.blockSize, Version2)
			if err != nil {
				t.Fatalf("Failed to create encryptor: %v", err)
			}

			_, _, err = encryptor.EncryptWithStartingCounter(plaintext, uint32(tc.maxBlocks-1))
			if err == nil {
				t.Errorf("Encrypt with %s at boundary should fail but succeeded", tc.name)
			}
			if err != nil && err.Error() != "object exceeds the Version 2 counter space; envelope v3 removes this limit" {
				t.Errorf("Expected specific error message, got: %v", err)
			}
		})
	}
}

// TestCounterSpaceBoundaryValue tests the exact boundary condition.
func TestCounterSpaceBoundaryValue(t *testing.T) {
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}
	iv, err := GenerateIV()
	if err != nil {
		t.Fatalf("Failed to generate IV: %v", err)
	}

	blockSize := 65536
	aesBlocksPerArmorBlock := blockSize / 16

	// Test the exact boundary where counter == 2^32 - 1
	// blockIndex * aesBlocksPerArmorBlock = 2^32 - 1
	// blockIndex = (2^32 - 1) / aesBlocksPerArmorBlock
	maxBlockIndex := (1<<32 - 1) / aesBlocksPerArmorBlock

	t.Run("exact boundary should succeed", func(t *testing.T) {
		// startBlockIndex = maxBlockIndex-1 with a single-block plaintext
		// gives the same finalCounterValue as maxBlockIndex full blocks
		// starting from 0: counter = 2^32 - 1 (just under overflow)
		plaintext := make([]byte, blockSize)
		rand.Read(plaintext)

		encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version2)
		if err != nil {
			t.Fatalf("Failed to create encryptor: %v", err)
		}

		_, _, err = encryptor.EncryptWithStartingCounter(plaintext, uint32(maxBlockIndex-1))
		if err != nil {
			t.Errorf("Encrypt at exact boundary (counter=2^32-1) should succeed but got error: %v", err)
		}
	})

	t.Run("one block over boundary should fail", func(t *testing.T) {
		// startBlockIndex = maxBlockIndex with a single-block plaintext
		// reaches finalCounterValue >= 2^32 (overflow)
		plaintext := make([]byte, blockSize)
		rand.Read(plaintext)

		encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version2)
		if err != nil {
			t.Fatalf("Failed to create encryptor: %v", err)
		}

		_, _, err = encryptor.EncryptWithStartingCounter(plaintext, uint32(maxBlockIndex))
		if err == nil {
			t.Errorf("Encrypt one block over boundary should fail but succeeded")
		}
	})
}
