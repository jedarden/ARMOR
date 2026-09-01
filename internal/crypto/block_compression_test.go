// Package crypto provides encryption, decryption, and key management for ARMOR.
package crypto

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompressBlock_CompressibleData tests that compressible data is actually compressed.
func TestCompressBlock_CompressibleData(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		minRatio  float64 // Minimum compression ratio (compressed / original)
	}{
		{
			name:     "repetitive zeros",
			data:     bytes.Repeat([]byte{0}, 1024),
			minRatio: 0.1, // Should compress to < 10% of original
		},
		{
			name:     "repetitive pattern",
			data:     bytes.Repeat([]byte("ABCDEFGH"), 128), // 1024 bytes
			minRatio: 0.15,
		},
		{
			name:     "JSON-like data",
			data:     bytes.Repeat([]byte(`{"key":"value","array":[1,2,3],"nested":{"x":1}}`), 20),
			minRatio: 0.25,
		},
		{
			name:     "text content",
			data:     bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), 20),
			minRatio: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, wasCompressed, compType, err := CompressBlock(tt.data)
			require.NoError(t, err)
			require.True(t, wasCompressed, "data should be marked as compressed")
			require.Equal(t, CompressionZstd, compType, "should use zstd compression")

			ratio := float64(len(compressed)) / float64(len(tt.data))
			assert.Less(t, ratio, tt.minRatio, "compression ratio %v should be less than %v", ratio, tt.minRatio)
		})
	}
}

// TestCompressBlock_IncompressibleData tests that incompressible data passes through.
func TestCompressBlock_IncompressibleData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "random data",
			data: generateRandomBytes(1024),
		},
		{
			name: "already compressed",
			data: generateCompressedData(1024),
		},
		{
			name: "high entropy",
			data: generateHighEntropyData(1024),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, wasCompressed, compType, err := CompressBlock(tt.data)
			require.NoError(t, err)
			require.False(t, wasCompressed, "incompressible data should not be marked as compressed")
			require.Equal(t, CompressionNone, compType, "should use no compression")
			require.Equal(t, tt.data, compressed, "should return original data unchanged")
		})
	}
}

// TestCompressBlock_EmptyBlock tests that empty blocks are handled correctly.
func TestCompressBlock_EmptyBlock(t *testing.T) {
	compressed, wasCompressed, compType, err := CompressBlock([]byte{})
	require.NoError(t, err)
	require.False(t, wasCompressed, "empty block should not be marked as compressed")
	require.Equal(t, CompressionNone, compType, "should use no compression")
	require.Empty(t, compressed, "should return empty slice")
}

// TestCompressBlock_SmallBlock tests small blocks near compression threshold.
func TestCompressBlock_SmallBlock(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool // true if compression expected
	}{
		{
			name:     "tiny compressible",
			data:     bytes.Repeat([]byte{0}, 16),
			expected: true, // Should compress even at small size
		},
		{
			name:     "tiny incompressible",
			data:     []byte("ABCDEFGH"), // 8 bytes
			expected: false, // Too small to benefit from compression overhead
		},
		{
			name:     "small pattern",
			data:     bytes.Repeat([]byte("A"), 32),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, wasCompressed, _, err := CompressBlock(tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, wasCompressed, "compression expectation")
			assert.Equal(t, tt.expected, len(compressed) < len(tt.data), "size reduction")
		})
	}
}

// TestDecompressBlock_CompressedData tests decompression of compressed blocks.
func TestDecompressBlock_CompressedData(t *testing.T) {
	original := bytes.Repeat([]byte{0}, 1024)

	// Compress then decompress
	compressed, _, _, err := CompressBlock(original)
	require.NoError(t, err)

	decompressed, err := DecompressBlock(compressed, true)
	require.NoError(t, err)
	require.Equal(t, original, decompressed, "decompressed data should match original")
}

// TestDecompressBlock_UncompressedData tests pass-through of uncompressed blocks.
func TestDecompressBlock_UncompressedData(t *testing.T) {
	original := generateRandomBytes(1024)

	decompressed, err := DecompressBlock(original, false)
	require.NoError(t, err)
	require.Equal(t, original, decompressed, "uncompressed data should pass through unchanged")
}

// TestDecompressBlock_ErrorHandling tests error handling in decompression.
func TestDecompressBlock_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		data          []byte
		isCompressed  bool
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid compressed",
			data:        func() []byte { compressed, _, _, _ := CompressBlock(bytes.Repeat([]byte{0}, 1024)); return compressed }(),
			isCompressed: true,
			expectError:   false,
		},
		{
			name:          "invalid compressed data",
			data:          []byte{0x28, 0xB5, 0x2F, 0xFD, 0xFF}, // Truncated zstd magic
			isCompressed:  true,
			expectError:   true,
			errorContains: "decompression failed",
		},
		{
			name:        "uncompressed flag false",
			data:        []byte("any data"),
			isCompressed: false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decompressed, err := DecompressBlock(tt.data, tt.isCompressed)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, decompressed)
			}
		})
	}
}

// TestEncryptWithBlockCompression_RoundTrip tests full encryption/decryption cycle.
func TestEncryptWithBlockCompression_RoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		plaintext  []byte
		blockSize  int
		compress   bool
	}{
		{
			name:      "single block compressible",
			plaintext: bytes.Repeat([]byte{0}, 1024),
			blockSize: 1024,
			compress:  true,
		},
		{
			name:      "single block incompressible",
			plaintext: generateRandomBytes(1024),
			blockSize: 1024,
			compress:  true,
		},
		{
			name:      "multiple blocks mixed",
			plaintext: generateMixedData(4096),
			blockSize: 1024,
			compress:  true,
		},
		{
			name:      "large object",
			plaintext: generateMixedData(64 * 1024),
			blockSize: 4096,
			compress:  true,
		},
		{
			name:      "exact block boundary",
			plaintext: bytes.Repeat([]byte{0x42}, 8192),
			blockSize: 1024,
			compress:  true,
		},
		{
			name:      "partial final block",
			plaintext: bytes.Repeat([]byte{0x42}, 8500),
			blockSize: 1024,
			compress:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup keys
			dek := make([]byte, 32)
			iv := make([]byte, 16)
			_, err := io.ReadFull(rand.Reader, dek)
			require.NoError(t, err)
			_, err = io.ReadFull(rand.Reader, iv)
			require.NoError(t, err)

			// Create encryptor
			enc, err := NewEncryptorV2(dek, iv, tt.blockSize)
			require.NoError(t, err)

			// Encrypt with block compression
			encrypted, blockTable, err := enc.EncryptWithBlockCompression(tt.plaintext)
			require.NoError(t, err)
			require.NotNil(t, blockTable)
			require.NotEmpty(t, encrypted)

			// Create decryptor
			dec, err := NewDecryptor(dek, iv, tt.blockSize)
			require.NoError(t, err)

			// Decrypt with block decompression
			decrypted, err := dec.DecryptWithBlockDecompression(encrypted, blockTable)
			require.NoError(t, err)

			// Verify round-trip
			require.Equal(t, tt.plaintext, decrypted, "round-trip should preserve data")
		})
	}
}

// TestEncryptWithBlockCompression_BlockTableFlags tests that compression flags are set correctly.
func TestEncryptWithBlockCompression_BlockTableFlags(t *testing.T) {
	// Create plaintext with alternating compressible/incompressible blocks
	plaintext := make([]byte, 4096)
	for i := 0; i < 4; i++ {
		offset := i * 1024
		if i%2 == 0 {
			// Even blocks: compressible (all zeros)
			for j := offset; j < offset+1024; j++ {
				plaintext[j] = 0
			}
		} else {
			// Odd blocks: incompressible (random)
			copy(plaintext[offset:offset+1024], generateRandomBytes(1024))
		}
	}

	// Setup
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	io.ReadFull(rand.Reader, dek)
	io.ReadFull(rand.Reader, iv)

	blockSize := 1024
	enc, err := NewEncryptorV2(dek, iv, blockSize)
	require.NoError(t, err)

	// Encrypt
	encrypted, blockTable, err := enc.EncryptWithBlockCompression(plaintext)
	require.NoError(t, err)

	// Verify block table entries
	entryCount := blockTable.EntryCount()
	assert.Equal(t, 4, entryCount, "should have 4 entries")

	for i := 0; i < entryCount; i++ {
		entry := blockTable.Entries[i]
		if i%2 == 0 {
			// Even blocks should be compressed
			assert.True(t, entry.IsCompressed(), "block %d should be compressed", i)
		} else {
			// Odd blocks should NOT be compressed
			assert.False(t, entry.IsCompressed(), "block %d should not be compressed", i)
		}
	}

	// Verify decryption
	dec, _ := NewDecryptor(dek, iv, blockSize)
	decrypted, err := dec.DecryptWithBlockDecompression(encrypted, blockTable)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

// TestEncryptWithBlockCompression_VariableLengthOutput tests that output is variable-length.
func TestEncryptWithBlockCompression_VariableLengthOutput(t *testing.T) {
	plaintext := bytes.Repeat([]byte{0}, 4096) // Highly compressible

	dek := make([]byte, 32)
	iv := make([]byte, 16)
	io.ReadFull(rand.Reader, dek)
	io.ReadFull(rand.Reader, iv)

	blockSize := 1024
	enc, err := NewEncryptorV2(dek, iv, blockSize)
	require.NoError(t, err)

	// Encrypt with compression
	encrypted, blockTable, err := enc.EncryptWithBlockCompression(plaintext)
	require.NoError(t, err)

	// Compressed output should be significantly smaller
	compressionRatio := float64(len(encrypted)) / float64(len(plaintext))
	assert.Less(t, compressionRatio, 0.3, "compressed encrypted data should be much smaller")

	// Verify it still decrypts correctly
	dec, _ := NewDecryptor(dek, iv, blockSize)
	decrypted, err := dec.DecryptWithBlockDecompression(encrypted, blockTable)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

// TestDecryptWithBlockDecompression_MixedFlags tests decryption of mixed compressed/uncompressed blocks.
func TestDecryptWithBlockDecompression_MixedFlags(t *testing.T) {
	// Create a block table with mixed compression flags
	plaintext := make([]byte, 8192)
	for i := 0; i < 8; i++ {
		offset := i * 1024
		if i < 3 {
			// First 3 blocks: compressible
			for j := offset; j < offset+1024; j++ {
				plaintext[j] = byte(i)
			}
		} else if i < 6 {
			// Next 3 blocks: incompressible
			copy(plaintext[offset:offset+1024], generateRandomBytes(1024))
		} else {
			// Last 2 blocks: compressible
			for j := offset; j < offset+1024; j++ {
				plaintext[j] = byte(i)
			}
		}
	}

	dek := make([]byte, 32)
	iv := make([]byte, 16)
	io.ReadFull(rand.Reader, dek)
	io.ReadFull(rand.Reader, iv)

	blockSize := 1024
	enc, err := NewEncryptorV2(dek, iv, blockSize)
	require.NoError(t, err)

	// Encrypt
	encrypted, blockTable, err := enc.EncryptWithBlockCompression(plaintext)
	require.NoError(t, err)

	// Verify expected compression pattern
	entryCount := blockTable.EntryCount()
	compressedCount := 0
	for i := 0; i < entryCount; i++ {
		if blockTable.Entries[i].IsCompressed() {
			compressedCount++
		}
	}
	assert.GreaterOrEqual(t, compressedCount, 4, "should have at least 4 compressed blocks")

	// Decrypt
	dec, _ := NewDecryptor(dek, iv, blockSize)
	decrypted, err := dec.DecryptWithBlockDecompression(encrypted, blockTable)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

// TestEncryptWithBlockCompression_EmptyPlaintext tests handling of empty input.
func TestEncryptWithBlockCompression_EmptyPlaintext(t *testing.T) {
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	io.ReadFull(rand.Reader, dek)
	io.ReadFull(rand.Reader, iv)

	enc, err := NewEncryptorV2(dek, iv, 1024)
	require.NoError(t, err)

	encrypted, blockTable, err := enc.EncryptWithBlockCompression([]byte{})
	require.NoError(t, err)
	require.Empty(t, encrypted, "empty plaintext should produce empty ciphertext")
	require.NotNil(t, blockTable, "block table should be created")
	require.Equal(t, 0, blockTable.EntryCount(), "block table should have no entries")
}

// TestEncryptWithBlockCompression_SingleBlock tests single block encryption.
func TestEncryptWithBlockCompression_SingleBlock(t *testing.T) {
	plaintext := bytes.Repeat([]byte{0x42}, 512)

	dek := make([]byte, 32)
	iv := make([]byte, 16)
	io.ReadFull(rand.Reader, dek)
	io.ReadFull(rand.Reader, iv)

	enc, err := NewEncryptorV2(dek, iv, 4096)
	require.NoError(t, err)

	encrypted, blockTable, err := enc.EncryptWithBlockCompression(plaintext)
	require.NoError(t, err)
	require.Equal(t, 1, blockTable.EntryCount(), "should have single entry")

	dec, _ := NewDecryptor(dek, iv, 4096)
	decrypted, err := dec.DecryptWithBlockDecompression(encrypted, blockTable)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

// TestEncryptWithBlockCompression_MaxBlockSize tests compression at max block size.
func TestEncryptWithBlockCompression_MaxBlockSize(t *testing.T) {
	// Version 3 max block size is 1 MiB
	plaintext := generateMixedData(V3MaxBlockSize)

	dek := make([]byte, 32)
	iv := make([]byte, 16)
	io.ReadFull(rand.Reader, dek)
	io.ReadFull(rand.Reader, iv)

	enc, err := NewEncryptorWithVersion(dek, iv, V3MaxBlockSize, Version3)
	require.NoError(t, err)

	encrypted, blockTable, err := enc.EncryptWithBlockCompression(plaintext)
	require.NoError(t, err)

	dec, _ := NewDecryptorWithVersion(dek, iv, V3MaxBlockSize, Version3)
	decrypted, err := dec.DecryptWithBlockDecompression(encrypted, blockTable)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

// TestEncryptWithBlockCompression_CorruptedBlockTable tests error handling with corrupted block table.
func TestEncryptWithBlockCompression_CorruptedBlockTable(t *testing.T) {
	plaintext := bytes.Repeat([]byte{0}, 1024)

	dek := make([]byte, 32)
	iv := make([]byte, 16)
	io.ReadFull(rand.Reader, dek)
	io.ReadFull(rand.Reader, iv)

	enc, _ := NewEncryptorV2(dek, iv, 1024)
	encrypted, blockTable, err := enc.EncryptWithBlockCompression(plaintext)
	require.NoError(t, err)

	// Corrupt the block table by modifying an entry
	corruptedTable := NewBlockTable(1024, 1)
	corruptedEntry := &BlockTableEntry{
		HMAC:             blockTable.Entries[0].HMAC,
		CiphertextLength: blockTable.Entries[0].CiphertextLength + 100, // Wrong length
	}
	corruptedTable.AddEntry(corruptedEntry)

	dec, _ := NewDecryptor(dek, iv, 1024)
	_, err = dec.DecryptWithBlockDecompression(encrypted, corruptedTable)
	require.Error(t, err, "should fail with corrupted block table")
}

// TestEncryptWithBlockCompression_Version1 tests that compression works with Version1 (legacy).
func TestEncryptWithBlockCompression_Version1(t *testing.T) {
	plaintext := bytes.Repeat([]byte{0}, 2048)

	dek := make([]byte, 32)
	iv := make([]byte, 16)
	io.ReadFull(rand.Reader, dek)
	io.ReadFull(rand.Reader, iv)

	enc, err := NewEncryptorWithVersion(dek, iv, 1024, Version1)
	require.NoError(t, err)

	encrypted, blockTable, err := enc.EncryptWithBlockCompression(plaintext)
	require.NoError(t, err)

	dec, _ := NewDecryptorWithVersion(dek, iv, 1024, Version1)
	decrypted, err := dec.DecryptWithBlockDecompression(encrypted, blockTable)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

// Helper functions

// generateRandomBytes generates cryptographically random bytes.
func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		panic(fmt.Sprintf("failed to generate random bytes: %v", err))
	}
	return b
}

// generateCompressedData generates high-entropy data (simulating already-compressed content).
func generateCompressedData(n int) []byte {
	return generateRandomBytes(n)
}

// generateHighEntropyData generates data with maximum entropy.
func generateHighEntropyData(n int) []byte {
	return generateRandomBytes(n)
}

// generateMixedData generates data with alternating compressible and incompressible sections.
func generateMixedData(n int) []byte {
	data := make([]byte, n)
	blockSize := 1024
	for i := 0; i < n; i += blockSize {
		end := i + blockSize
		if end > n {
			end = n
		}
		if (i/blockSize)%2 == 0 {
			// Even blocks: compressible (all zeros)
			for j := i; j < end; j++ {
				data[j] = 0
			}
		} else {
			// Odd blocks: incompressible (random)
			copy(data[i:end], generateRandomBytes(end-i))
		}
	}
	return data
}
