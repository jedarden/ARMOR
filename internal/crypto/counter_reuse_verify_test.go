package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"testing"
)

// TestProveCounterReuseDirectly documents the Version1 counter reuse vulnerability
// as a permanent tripwire. It validates that:
// 1. Version1 DOES reuse keystream between adjacent blocks (the documented bug)
// 2. Version2 does NOT reuse keystream (the fix)
// This test must never be skipped or removed - it documents why V1 is deprecated.
func TestProveCounterReuseDirectly(t *testing.T) {
	// Test Version1: PROVE it reuses keystream
	t.Run("Version1_reuses_keystream", func(t *testing.T) {
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
		aesBlocksPerArmorBlock := blockSize / 16 // 4096 AES blocks

		// Create AES cipher
		block, err := aes.NewCipher(dek)
		if err != nil {
			t.Fatal(err)
		}

		// Create counters for block 0 and block 1 using makeCounter (Version1)
		enc, err := NewEncryptorWithVersion(dek, iv, blockSize, Version1)
		if err != nil {
			t.Fatal(err)
		}
		counter0 := enc.makeCounter(0)
		counter1 := enc.makeCounter(1)

		t.Logf("Version1 - Block 0 starting counter: %x", counter0)
		t.Logf("Version1 - Block 1 starting counter: %x", counter1)

		// Generate keystream for both blocks by simulating cipher.NewCTR behavior
		// cipher.NewCTR increments the FULL 16-byte counter for each 16-byte block
		keystream0 := generateCTRKeystream(t, block, counter0, aesBlocksPerArmorBlock)
		keystream1 := generateCTRKeystream(t, block, counter1, aesBlocksPerArmorBlock)

		// The bug: block 1's counter starts only 1 increment ahead
		// After 4096 AES blocks, block 0 used counters [counter0+0 ... counter0+4095]
		// Block 1 starts at counter1 = counter0+1, so it uses [counter0+1 ... counter0+4096]
		// Overlap: [counter0+1 ... counter0+4095] = 4095 out of 4096 counters!

		// Check for overlap in keystream
		// Block 0 keystream at AES block offset 1..4095 should match block 1 keystream at offset 0..4094
		overlapCount := 0
		for i := 1; i < aesBlocksPerArmorBlock; i++ {
			// Block 0's keystream at AES block i
			ks0Offset := (i) * 16
			ks1Offset := (i - 1) * 16

			ks0 := keystream0[ks0Offset : ks0Offset+16]
			ks1 := keystream1[ks1Offset : ks1Offset+16]

			if bytes.Equal(ks0, ks1) {
				overlapCount++
			} else {
				t.Logf("Mismatch at offset %d: ks0=%x ks1=%x", i, ks0, ks1)
			}
		}

		overlapBytes := overlapCount * 16

		// ASSERT: Version1 MUST reuse keystream (this is the documented bug)
		if overlapCount != aesBlocksPerArmorBlock-1 {
			t.Errorf("Version1 TRIPWIRE FAILED: Expected keystream reuse (%d overlapping blocks), got %d",
				aesBlocksPerArmorBlock-1, overlapCount)
			t.Errorf("Version1 may have been accidentally fixed - update this test if V1 is no longer legacy")
		} else {
			t.Logf("Version1 TRIPWIRE PASS: Keystream reuse confirmed (%d AES blocks, %d bytes)",
				overlapCount, overlapBytes)
			t.Logf("This is the documented TWO-TIME PAD vulnerability in Version1")
		}
	})

	// Test Version2: PROVE it does NOT reuse keystream
	t.Run("Version2_no_keystream_reuse", func(t *testing.T) {
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
		aesBlocksPerArmorBlock := blockSize / 16 // 4096 AES blocks

		// Create AES cipher
		block, err := aes.NewCipher(dek)
		if err != nil {
			t.Fatal(err)
		}

		// Create counters for block 0 and block 1 using makeCounter (Version2)
		enc, err := NewEncryptorWithVersion(dek, iv, blockSize, Version2)
		if err != nil {
			t.Fatal(err)
		}
		counter0 := enc.makeCounter(0)
		counter1 := enc.makeCounter(1)

		t.Logf("Version2 - Block 0 starting counter: %x", counter0)
		t.Logf("Version2 - Block 1 starting counter: %x", counter1)

		// Generate keystream for both blocks
		keystream0 := generateCTRKeystream(t, block, counter0, aesBlocksPerArmorBlock)
		keystream1 := generateCTRKeystream(t, block, counter1, aesBlocksPerArmorBlock)

		// Check for overlap in keystream
		overlapCount := 0
		for i := 1; i < aesBlocksPerArmorBlock; i++ {
			ks0Offset := (i) * 16
			ks1Offset := (i - 1) * 16

			ks0 := keystream0[ks0Offset : ks0Offset+16]
			ks1 := keystream1[ks1Offset : ks1Offset+16]

			if bytes.Equal(ks0, ks1) {
				overlapCount++
			}
		}

		// ASSERT: Version2 MUST NOT reuse keystream (this is the fix)
		if overlapCount > 0 {
			t.Errorf("Version2 TRIPWIRE FAILED: Keystream reuse detected (%d overlapping blocks)!", overlapCount)
			t.Errorf("Version2 fix is broken - this is a critical security issue")
		} else {
			t.Logf("Version2 TRIPWIRE PASS: No keystream reuse (fix is working)")
		}
	})
}

// TestDefaultEncryptorIsVersion2 validates that NewEncryptor() returns Version2.
func TestDefaultEncryptorIsVersion2(t *testing.T) {
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	blockSize := DefaultBlockSize

	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		t.Fatal(err)
	}

	enc, err := NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatal(err)
	}

	// ASSERT: Default encryptor MUST be Version2
	if enc.version != Version2 {
		t.Errorf("TRIPWIRE FAILED: NewEncryptor() returned version %d, expected Version2 (%d)",
			enc.version, Version2)
		t.Errorf("All new objects MUST use Version2 by default - this is a security requirement")
	} else {
		t.Logf("TRIPWIRE PASS: NewEncryptor() correctly returns Version2")
	}
}

// generateCTRKeystream simulates cipher.NewCTR keystream generation.
// Returns keystream for the specified number of AES blocks.
func generateCTRKeystream(t *testing.T, block cipher.Block, startCounter []byte, numAESBlocks int) []byte {
	keystream := make([]byte, numAESBlocks*16)

	// Each counter increment generates 16 bytes of keystream
	currentCounter := make([]byte, 16)
	copy(currentCounter, startCounter)

	for i := 0; i < numAESBlocks; i++ {
		// Encrypt the counter to get keystream block
		block.Encrypt(keystream[i*16:(i+1)*16], currentCounter)

		// Increment counter (as 128-bit big-endian integer)
		incrementCounter(currentCounter)
	}

	return keystream
}

// incrementCounter increments a 16-byte counter as a 128-bit big-endian integer.
func incrementCounter(counter []byte) {
	// Start from the last byte and work backwards
	for i := 15; i >= 0; i-- {
		counter[i]++
		if counter[i] != 0 {
			break // No carry, done
		}
		// Carry to next byte
	}
}

// incrementCounterN increments a counter n times.
func incrementCounterN(counter []byte, n int) {
	for i := 0; i < n; i++ {
		incrementCounter(counter)
	}
}
