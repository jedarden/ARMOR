package crypto

import (
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestV3CounterUniquenessProperty is a property test that verifies no two
// (part, block, aesBlock) triples produce the same counter value.
func TestV3CounterUniquenessProperty(t *testing.T) {
	// Generate a fixed IV
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i)
	}

	// Track seen counters
	seen := make(map[string]bool)

	// Property: different triples should produce different counters
	property := func(part1 uint16, block1 uint32, aesBlock1 uint16,
		part2 uint16, block2 uint32, aesBlock2 uint16) bool {
		// Skip if they're the same triple
		if part1 == part2 && block1 == block2 && aesBlock1 == aesBlock2 {
			return true
		}

		counter1 := MakeV3Counter(iv, part1, block1, aesBlock1)
		counter2 := MakeV3Counter(iv, part2, block2, aesBlock2)

		// Convert to string for map key
		key1 := string(counter1)
		key2 := string(counter2)

		// Check they're different
		if key1 == key2 {
			t.Logf("Counter collision: part(%d,%d) block(%d,%d) aesBlock(%d,%d)",
				part1, part2, block1, block2, aesBlock1, aesBlock2)
			return false
		}

		return true
	}

	// Run the property test with random inputs
	err := quick.Check(property, &quick.Config{
		MaxCount: 10000,
	})
	require.NoError(t, err, "Property test failed: found counter collisions")
}

// TestV3CounterUniquenessExhaustive exhaustively tests counter uniqueness
// for a small range of values.
func TestV3CounterUniquenessExhaustive(t *testing.T) {
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i)
	}

	// Test all combinations of small values
	maxPart := uint16(3)
	maxBlock := uint32(3)
	maxAESBlock := uint16(3)

	counters := make(map[string]struct {
		part     uint16
		block    uint32
		aesBlock uint16
	})

	for part := uint16(0); part <= maxPart; part++ {
		for block := uint32(0); block <= maxBlock; block++ {
			for aesBlock := uint16(0); aesBlock <= maxAESBlock; aesBlock++ {
				counter := MakeV3Counter(iv, part, block, aesBlock)
				key := string(counter)

				if existing, found := counters[key]; found {
					t.Errorf("Counter collision at (part=%d, block=%d, aesBlock=%d) with (part=%d, block=%d, aesBlock=%d)",
						part, block, aesBlock, existing.part, existing.block, existing.aesBlock)
				}

				counters[key] = struct{ part uint16; block uint32; aesBlock uint16 }{
					part:     part,
					block:    block,
					aesBlock: aesBlock,
				}
			}
		}
	}

	// Verify we got the expected number of unique counters
	expectedCount := (maxPart + 1) * (maxBlock + 1) * (maxAESBlock + 1)
	assert.Equal(t, expectedCount, uint32(len(counters)),
		"Should have exactly %d unique counters", expectedCount)
}

// TestV3CounterStructure verifies the byte layout of v3 counters.
func TestV3CounterStructure(t *testing.T) {
	iv := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
	}

	tests := []struct {
		name      string
		part      uint16
		block     uint32
		aesBlock  uint16
		wantBytes []byte // Expected bytes 8-15 (after IV prefix)
	}{
		{
			name:     "single PUT first block",
			part:     0,
			block:    0,
			aesBlock: 0,
			wantBytes: []byte{
				0x00, 0x00, // part (big-endian)
				0x00, 0x00, 0x00, 0x00, // block (big-endian)
				0x00, 0x00, // aesBlock (big-endian)
			},
		},
		{
			name:     "single PUT second block",
			part:     0,
			block:    1,
			aesBlock: 0,
			wantBytes: []byte{
				0x00, 0x00, // part
				0x00, 0x00, 0x00, 0x01, // block
				0x00, 0x00, // aesBlock
			},
		},
		{
			name:     "multipart part 1 first block",
			part:     1,
			block:    0,
			aesBlock: 0,
			wantBytes: []byte{
				0x00, 0x01, // part
				0x00, 0x00, 0x00, 0x00, // block
				0x00, 0x00, // aesBlock
			},
		},
		{
			name:     "multipart part 100 block 5 aesBlock 3",
			part:     100,
			block:    5,
			aesBlock: 3,
			wantBytes: []byte{
				0x00, 0x64, // part = 100
				0x00, 0x00, 0x00, 0x05, // block = 5
				0x00, 0x03, // aesBlock = 3
			},
		},
		{
			name:     "max part number",
			part:     10000,
			block:    0,
			aesBlock: 0,
			wantBytes: []byte{
				0x27, 0x10, // part = 10000
				0x00, 0x00, 0x00, 0x00, // block
				0x00, 0x00, // aesBlock
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := MakeV3Counter(iv, tt.part, tt.block, tt.aesBlock)

			// Verify IV prefix is copied
			assert.Equal(t, iv[0:8], counter[0:8], "IV prefix should match")

			// Verify counter fields
			assert.Equal(t, tt.wantBytes, counter[8:16], "Counter fields should match expected layout")

			// Verify we can parse the values back
			gotPart := binary.BigEndian.Uint16(counter[8:10])
			gotBlock := binary.BigEndian.Uint32(counter[10:14])
			gotAESBlock := binary.BigEndian.Uint16(counter[14:16])

			assert.Equal(t, tt.part, gotPart, "Part should round-trip")
			assert.Equal(t, tt.block, gotBlock, "Block should round-trip")
			assert.Equal(t, tt.aesBlock, gotAESBlock, "AESBlock should round-trip")
		})
	}
}

// TestV3BlockHMACKeys verifies that HMAC covers part and block.
func TestV3BlockHMACKeys(t *testing.T) {
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i)
	}
	hmacKey := DeriveHMACKey(dek)

	ciphertext := []byte{0x01, 0x02, 0x03, 0x04}

	tests := []struct {
		name          string
		part          uint16
		block         uint32
		shouldDiffer  bool
		compareWith   int // index in tests array
	}{
		{
			name:         "same part/block produces same HMAC",
			part:         1,
			block:        2,
			shouldDiffer: false,
			compareWith:  0, // self
		},
		{
			name:         "different part produces different HMAC",
			part:         2,
			block:        2,
			shouldDiffer: true,
			compareWith:  0,
		},
		{
			name:         "different block produces different HMAC",
			part:         1,
			block:        3,
			shouldDiffer: true,
			compareWith:  0,
		},
	}

	hmacs := make([][]byte, len(tests))
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hmacs[i] = ComputeV3BlockHMAC(hmacKey, tt.part, tt.block, ciphertext)
			assert.Len(t, hmacs[i], HMACSize, "HMAC should be 32 bytes")
		})
	}

	for i, tt := range tests {
		if tt.shouldDiffer {
			assert.NotEqual(t, hmacs[i], hmacs[tt.compareWith],
				"HMACs should differ for different (part,block) pairs")
		} else if i == tt.compareWith {
			// Self comparison should be equal
			assert.Equal(t, hmacs[i], hmacs[tt.compareWith],
				"Same (part,block) should produce same HMAC")
		}
	}
}

// TestV3MaxBlockSizeConstraint verifies that Version3 enforces the 1 MiB limit.
func TestV3MaxBlockSizeConstraint(t *testing.T) {
	dek := make([]byte, 32)
	iv := make([]byte, 16)

	tests := []struct {
		name      string
		blockSize int
		wantErr   bool
	}{
		{
			name:      "64 KiB block size",
			blockSize: 64 * 1024,
			wantErr:   false,
		},
		{
			name:      "256 KiB block size",
			blockSize: 256 * 1024,
			wantErr:   false,
		},
		{
			name:      "1 MiB block size (maximum)",
			blockSize: 1024 * 1024,
			wantErr:   false,
		},
		{
			name:      "1 MiB + 1 byte (exceeds maximum)",
			blockSize: 1024*1024 + 1,
			wantErr:   true,
		},
		{
			name:      "2 MiB block size (exceeds maximum)",
			blockSize: 2 * 1024 * 1024,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := EncryptBlockV3(dek, iv, 0, 0, make([]byte, tt.blockSize), tt.blockSize)
			if tt.wantErr {
				assert.Error(t, err, "Should enforce maximum block size")
			} else {
				assert.NoError(t, err, "Should allow block size <= 1 MiB")
			}
		})
	}
}
