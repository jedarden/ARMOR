package crypto

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"
)

// TestCTRKeystreamNoReuseV2 validates that Version2 prevents keystream reuse.
func TestCTRKeystreamNoReuseV2(t *testing.T) {
	// Generate test DEK and IV
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		t.Fatal(err)
	}

	blockSize := DefaultBlockSize // 65536 bytes

	// Create Version2 encryptor (fixed)
	enc, err := NewEncryptorV2(dek, iv, blockSize)
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt two all-zero blocks to extract keystream
	plaintext0 := make([]byte, blockSize) // All zeros
	plaintext1 := make([]byte, blockSize) // All zeros

	encrypted0, _, err := enc.Encrypt(plaintext0)
	if err != nil {
		t.Fatal(err)
	}

	encrypted1, _, err := enc.Encrypt(plaintext1)
	if err != nil {
		t.Fatal(err)
	}

	// Since plaintext is all zeros, ciphertext = keystream
	keystream0 := encrypted0
	keystream1 := encrypted1

	// With Version2 fix: keystreams should NOT overlap
	// Check that keystream1[0:65520] != keystream0[16:65536]
	overlapSize := blockSize - 16 // 65520 bytes
	keystream0Overlap := keystream0[16:]
	keystream1Overlap := keystream1[:overlapSize]

	if bytes.Equal(keystream0Overlap, keystream1Overlap) {
		t.Errorf("Version2 FAILED: keystream reuse detected! The fix is not working.")
	} else {
		t.Logf("Version2 SUCCESS: no keystream reuse detected (fix is working)")
	}
}

// TestCTRV1V2Compatibility ensures V1 encrypts can be decrypted by V1 decryptor,
// and V2 encrypts can be decrypted by V2 decryptor.
func TestCTRV1V2Compatibility(t *testing.T) {
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		t.Fatal(err)
	}

	// Test data (4 blocks)
	plaintext := make([]byte, DefaultBlockSize*4)
	if _, err := io.ReadFull(rand.Reader, plaintext); err != nil {
		t.Fatal(err)
	}

	// Test Version1 round-trip
	t.Run("Version1 round-trip", func(t *testing.T) {
		enc, err := NewEncryptorWithVersion(dek, iv, DefaultBlockSize, Version1)
		if err != nil {
			t.Fatal(err)
		}

		encrypted, hmacTable, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatal(err)
		}

		dec, err := NewDecryptorWithVersion(dek, iv, DefaultBlockSize, Version1)
		if err != nil {
			t.Fatal(err)
		}

		decrypted, err := dec.Decrypt(encrypted, hmacTable)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(plaintext, decrypted) {
			t.Errorf("Version1 round-trip failed")
		}
	})

	// Test Version2 round-trip
	t.Run("Version2 round-trip", func(t *testing.T) {
		enc, err := NewEncryptorWithVersion(dek, iv, DefaultBlockSize, Version2)
		if err != nil {
			t.Fatal(err)
		}

		encrypted, hmacTable, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatal(err)
		}

		dec, err := NewDecryptorWithVersion(dek, iv, DefaultBlockSize, Version2)
		if err != nil {
			t.Fatal(err)
		}

		decrypted, err := dec.Decrypt(encrypted, hmacTable)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(plaintext, decrypted) {
			t.Errorf("Version2 round-trip failed")
		}
	})

	// Test that V1 and V2 produce DIFFERENT ciphertext (as they should)
	t.Run("V1 and V2 produce different ciphertext", func(t *testing.T) {
		encV1, err := NewEncryptorWithVersion(dek, iv, DefaultBlockSize, Version1)
		if err != nil {
			t.Fatal(err)
		}

		encV2, err := NewEncryptorWithVersion(dek, iv, DefaultBlockSize, Version2)
		if err != nil {
			t.Fatal(err)
		}

		encryptedV1, _, err := encV1.Encrypt(plaintext)
		if err != nil {
			t.Fatal(err)
		}

		encryptedV2, _, err := encV2.Encrypt(plaintext)
		if err != nil {
			t.Fatal(err)
		}

		// Ciphertext should be different (different counter derivation)
		if bytes.Equal(encryptedV1, encryptedV2) {
			t.Errorf("V1 and V2 produced identical ciphertext - fix may not be working")
		}
	})
}

// TestCTRCounterStrideV2 validates the counter stride calculation.
func TestCTRCounterStrideV2(t *testing.T) {
	iv := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		t.Fatal(err)
	}

	blockSize := DefaultBlockSize
	aesBlocksPerArmorBlock := blockSize / 16 // 4096

	// Create Version2 encryptor
	enc, err := NewEncryptorWithVersion(make([]byte, 32), iv, blockSize, Version2)
	if err != nil {
		t.Fatal(err)
	}

	// Check counters for consecutive blocks
	counters := make([][]byte, 4)
	for i := range counters {
		counters[i] = enc.makeCounter(uint32(i))
	}

	t.Logf("Block 0 counter: %x", counters[0])
	t.Logf("Block 1 counter: %x", counters[1])
	t.Logf("Block 2 counter: %x", counters[2])
	t.Logf("Block 3 counter: %x", counters[3])

	// Validate counter stride
	for i := 0; i < len(counters)-1; i++ {
		diff := counterDiff(counters[i], counters[i+1])
		if diff != uint64(aesBlocksPerArmorBlock) {
			t.Errorf("Counter stride between blocks %d and %d: got %d, want %d",
				i, i+1, diff, aesBlocksPerArmorBlock)
		}
	}

	t.Logf("Counter stride validated: %d AES blocks between consecutive ARMOR blocks", aesBlocksPerArmorBlock)
}
