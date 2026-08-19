package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"io"
	"testing"
)

// TestProveCounterReuseDirectly directly proves the counter reuse vulnerability.
// This test shows EXACTLY what counters are used and demonstrates the overlap.
func TestProveCounterReuseDirectly(t *testing.T) {
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

	// Create counters for block 0 and block 1 using makeCounter
	enc := &Encryptor{iv: iv, blockSize: blockSize}
	counter0 := enc.makeCounter(0)
	counter1 := enc.makeCounter(1)

	t.Logf("Block 0 starting counter: %x", counter0)
	t.Logf("Block 1 starting counter: %x", counter1)

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

	if overlapCount == aesBlocksPerArmorBlock-1 {
		t.Errorf("PROVEN BUG: Keystream reuse detected!")
		t.Errorf("Block 1 reuses %d AES blocks (%d bytes) of keystream from block 0", overlapCount, overlapBytes)
		t.Errorf("This is a TWO-TIME PAD - plaintext XOR can be recovered from ciphertext alone")
		t.Errorf("Block 0 uses AES counters: [IV+0 ... IV+%d]", aesBlocksPerArmorBlock-1)
		t.Errorf("Block 1 uses AES counters: [IV+1 ... IV+%d]", aesBlocksPerArmorBlock)
		t.Errorf("Overlap: [IV+1 ... IV+%d] = %d counters reused",
			aesBlocksPerArmorBlock-1, aesBlocksPerArmorBlock-1)
	} else {
		t.Logf("No keystream reuse detected (bug may be fixed)")
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

// TestProveCounterWithExplicitValues uses known values to prove the bug.
func TestProveCounterWithExplicitValues(t *testing.T) {
	// Use fixed values for reproducibility
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = 0xAA
	}

	enc := &Encryptor{iv: iv, blockSize: DefaultBlockSize}

	counter0 := enc.makeCounter(0)
	counter1 := enc.makeCounter(1)

	// Counter structure: IV[0:12] || uint32(blockIndex)
	// Expected: counter0 = AAAA... || 0x00000000
	// Expected: counter1 = AAAA... || 0x00000001

	if !bytes.Equal(counter0[0:12], iv[0:12]) {
		t.Errorf("counter0[0:12] != iv[0:12]")
	}

	if !bytes.Equal(counter1[0:12], iv[0:12]) {
		t.Errorf("counter1[0:12] != iv[0:12]")
	}

	blockIndex0 := binary.BigEndian.Uint32(counter0[12:16])
	blockIndex1 := binary.BigEndian.Uint32(counter1[12:16])

	if blockIndex0 != 0 {
		t.Errorf("blockIndex0 = %d, want 0", blockIndex0)
	}

	if blockIndex1 != 1 {
		t.Errorf("blockIndex1 = %d, want 1", blockIndex1)
	}

	t.Logf("counter0 = %x (block index = %d)", counter0, blockIndex0)
	t.Logf("counter1 = %x (block index = %d)", counter1, blockIndex1)

	// Simulate counter increment for 4096 AES blocks
	// Block 0 will use counters 0 through 4095
	// Block 1 starts at 1, so it uses 1 through 4096
	// Overlap: 1 through 4095 = 4095 counters

	// After 4096 increments from counter0, we should be at:
	expectedCounterAfterBlock0 := make([]byte, 16)
	copy(expectedCounterAfterBlock0, counter0)
	incrementCounterN(expectedCounterAfterBlock0, 4096)

	// counter1 should equal what we'd get after 1 increment from counter0
	expectedCounter1 := make([]byte, 16)
	copy(expectedCounter1, counter0)
	incrementCounter(expectedCounter1)

	if !bytes.Equal(counter1, expectedCounter1) {
		t.Errorf("counter1 mismatch")
	}

	t.Logf("After 4096 increments from counter0: %x", expectedCounterAfterBlock0)
	t.Logf("counter1 (start of block 1): %x", counter1)

	// Check if counter1 would be within the range used by block 0
	// Block 0 uses: counter0+0, counter0+1, ..., counter0+4095
	// counter1 = counter0+1, so it REUSES counter0+1 through counter0+4095

	t.Errorf("DEMONSTRATED: counter1 = counter0 + 1")
	t.Errorf("After 4096 AES blocks, counter will be at counter0 + 4096")
	t.Errorf("Block 1 reuses counters counter0+1 through counter0+4095")
	t.Errorf("This is %d out of %d possible counter values - a TWO-TIME PAD vulnerability",
		4095, 4096)
}

// incrementCounterN increments a counter n times.
func incrementCounterN(counter []byte, n int) {
	for i := 0; i < n; i++ {
		incrementCounter(counter)
	}
}
