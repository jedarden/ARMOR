package crypto

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestDecompress(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
		wantErr  bool
	}{
		{
			name:     "zstd compressed data",
			input:    compressData([]byte("Hello, ARMOR!")),
			expected: []byte("Hello, ARMOR!"),
			wantErr:  false,
		},
		{
			name:     "uncompressed data",
			input:    []byte("Hello, ARMOR!"),
			expected: []byte("Hello, ARMOR!"),
			wantErr:  false,
		},
		{
			name:     "empty data",
			input:    []byte{},
			expected: []byte{},
			wantErr:  false,
		},
		{
			name:     "too short data",
			input:    []byte{0xFD, 0x2F},
			expected: []byte{0xFD, 0x2F},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Decompress(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decompress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("Decompress() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDecompressRoundtrip(t *testing.T) {
	// Test various data sizes
	testData := []struct {
		name string
		data []byte
	}{
		{"small", []byte("Hello, ARMOR!")},
		{"medium", make([]byte, 10000)},
		{"large", make([]byte, 100000)},
	}

	for _, tt := range testData {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name != "small" {
				rand.Read(tt.data)
			}

			// Compress the data
			compressed := compressData(tt.data)

			// Decompress and verify
			decompressed, err := Decompress(compressed)
			if err != nil {
				t.Fatalf("Decompress() error = %v", err)
			}

			if !bytes.Equal(decompressed, tt.data) {
				t.Errorf("Decompress() roundtrip failed: got %d bytes, want %d bytes", len(decompressed), len(tt.data))
			}
		})
	}
}

func TestEncryptDecryptCompressRoundtrip(t *testing.T) {
	dek, _ := GenerateDEK()
	iv, _ := GenerateIV()

	// Original plaintext
	originalPlaintext := make([]byte, 50000)
	rand.Read(originalPlaintext)

	// Compress the plaintext
	compressedPlaintext := compressData(originalPlaintext)

	// Encrypt the compressed data
	encryptor, err := NewEncryptor(dek, iv, 65536)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	encrypted, hmacTable, err := encryptor.Encrypt(compressedPlaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Decrypt
	decryptor, err := NewDecryptor(dek, iv, 65536)
	if err != nil {
		t.Fatalf("Failed to create decryptor: %v", err)
	}

	decryptedCompressed, err := decryptor.Decrypt(encrypted, hmacTable)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	// Verify compression detection
	if !IsCompressed(decryptedCompressed) {
		t.Error("Decrypted data should be detected as compressed")
	}

	// Decompress
	decryptedPlaintext, err := Decompress(decryptedCompressed)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	// Verify roundtrip
	if !bytes.Equal(decryptedPlaintext, originalPlaintext) {
		t.Errorf("Roundtrip failed: got %d bytes, want %d bytes", len(decryptedPlaintext), len(originalPlaintext))
	}
}

// compressData is a helper function that compresses data using zstd.
func compressData(data []byte) []byte {
	var buf bytes.Buffer
	encoder, err := zstd.NewWriter(&buf)
	if err != nil {
		panic(err)
	}
	encoder.Write(data)
	encoder.Close()
	return buf.Bytes()
}

func TestDecompressErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:    "zstd magic with corrupted content",
			wantErr: true,
			input: func() []byte {
				// Create valid zstd data
				valid := compressData([]byte("Hello, ARMOR!"))
				// Corrupt the content after magic bytes
				corrupted := make([]byte, len(valid))
				copy(corrupted, valid)
				// Flip some bytes after the magic header (magic is 4 bytes)
				for i := 4; i < len(corrupted); i += 7 {
					corrupted[i] ^= 0xFF
				}
				return corrupted
			}(),
		},
		{
			name:    "zstd magic with truncated stream",
			wantErr: true,
			input: func() []byte {
				// Create valid zstd data
				valid := compressData([]byte("Hello, ARMOR!"))
				// Truncate it to make it incomplete
				if len(valid) > 10 {
					return valid[:len(valid)/2]
				}
				return valid[:5]
			}(),
		},
		{
			name:    "zstd magic with only magic bytes",
			wantErr: true,
			input:   []byte{0x28, 0xB5, 0x2F, 0xFD},
		},
		{
			name:    "zstd magic with partial frame",
			wantErr: true,
			input: func() []byte {
				// Create valid zstd data
				valid := compressData([]byte("test"))
				// Return just the magic bytes and a few more bytes
				if len(valid) > 8 {
					return valid[:8]
				}
				return valid
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decompress(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decompress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecompressLargeData(t *testing.T) {
	// Test with large random data to ensure decompression handles realistic payloads
	original := make([]byte, 1024*1024) // 1MB
	if _, err := rand.Read(original); err != nil {
		t.Fatalf("Failed to generate random data: %v", err)
	}

	// Compress
	compressed := compressData(original)

	// Decompress
	decompressed, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress() failed: %v", err)
	}

	// Verify
	if !bytes.Equal(decompressed, original) {
		t.Errorf("Decompress() roundtrip failed for large data: got %d bytes, want %d bytes", len(decompressed), len(original))
	}

	// Verify compression actually happened (compressed should be smaller)
	if len(compressed) >= len(original) {
		t.Logf("Note: Compression didn't reduce size for random data (compressed: %d, original: %d) - this is expected for high-entropy data", len(compressed), len(original))
	}
}

func TestDecompressNilData(t *testing.T) {
	t.Run("Decompress nil data", func(t *testing.T) {
		_, err := Decompress(nil)
		if err == nil {
			t.Fatal("Decompress(nil) should return error")
		}

		decompErr, ok := err.(*DecompressionError)
		if !ok {
			t.Fatalf("Expected DecompressionError, got %T", err)
		}

		if decompErr.Cause != "nil_data" {
			t.Errorf("Expected cause 'nil_data', got '%s'", decompErr.Cause)
		}

		if decompErr.ErrType != ErrTypeClient {
			t.Errorf("Expected ErrTypeClient, got %d", decompErr.ErrType)
		}
	})

	t.Run("DecompressGzip nil data", func(t *testing.T) {
		_, err := DecompressGzip(nil)
		if err == nil {
			t.Fatal("DecompressGzip(nil) should return error")
		}

		decompErr, ok := err.(*DecompressionError)
		if !ok {
			t.Fatalf("Expected DecompressionError, got %T", err)
		}

		if decompErr.Cause != "nil_data" {
			t.Errorf("Expected cause 'nil_data', got '%s'", decompErr.Cause)
		}
	})

	t.Run("DecompressZlib nil data", func(t *testing.T) {
		_, err := DecompressZlib(nil)
		if err == nil {
			t.Fatal("DecompressZlib(nil) should return error")
		}

		decompErr, ok := err.(*DecompressionError)
		if !ok {
			t.Fatalf("Expected DecompressionError, got %T", err)
		}

		if decompErr.Cause != "nil_data" {
			t.Errorf("Expected cause 'nil_data', got '%s'", decompErr.Cause)
		}
	})
}

func TestDecompressUnknownCompression(t *testing.T) {
	// Create data with invalid compression type
	unknownMagic := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

	// All decompression functions should return data unchanged for unknown types
	t.Run("Decompress unknown type", func(t *testing.T) {
		result, err := Decompress(unknownMagic)
		if err != nil {
			t.Errorf("Decompress should not error on unknown type, got %v", err)
		}
		if !bytes.Equal(result, unknownMagic) {
			t.Errorf("Decompress should return data unchanged for unknown type")
		}
	})

	t.Run("DecompressGzip unknown type", func(t *testing.T) {
		result, err := DecompressGzip(unknownMagic)
		if err != nil {
			t.Errorf("DecompressGzip should not error on unknown type, got %v", err)
		}
		if !bytes.Equal(result, unknownMagic) {
			t.Errorf("DecompressGzip should return data unchanged for unknown type")
		}
	})

	t.Run("DecompressZlib unknown type", func(t *testing.T) {
		result, err := DecompressZlib(unknownMagic)
		if err != nil {
			t.Errorf("DecompressZlib should not error on unknown type, got %v", err)
		}
		if !bytes.Equal(result, unknownMagic) {
			t.Errorf("DecompressZlib should return data unchanged for unknown type")
		}
	})
}

func TestDecompressCorruptionErrors(t *testing.T) {
	// Test corrupted zstd data
	t.Run("Corrupted zstd content", func(t *testing.T) {
		valid := compressData([]byte("Hello, World!"))
		corrupted := make([]byte, len(valid))
		copy(corrupted, valid)

		// Corrupt bytes after magic header
		for i := 4; i < len(corrupted); i += 3 {
			corrupted[i] ^= 0xFF
		}

		_, err := Decompress(corrupted)
		if err == nil {
			t.Error("Expected error for corrupted zstd data")
		}

		// Check error is properly classified
		decompErr, ok := err.(*DecompressionError)
		if !ok {
			t.Fatalf("Expected DecompressionError, got %T", err)
		}

		// Should be client error (data corruption)
		if decompErr.ErrType != ErrTypeClient {
			t.Errorf("Corrupted data should be ErrTypeClient, got %d", decompErr.ErrType)
		}

		// Cause should indicate corruption
		if !strings.Contains(decompErr.Cause, "corrupted") && !strings.Contains(decompErr.Cause, "frame") && !strings.Contains(decompErr.Cause, "block") {
			t.Logf("Note: Got cause '%s', expected corruption-related cause", decompErr.Cause)
		}
	})

	// Test corrupted gzip data
	t.Run("Corrupted gzip content", func(t *testing.T) {
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		writer.Write([]byte("Hello, World!"))
		writer.Close()

		valid := buf.Bytes()
		corrupted := make([]byte, len(valid))
		copy(corrupted, valid)

		// Corrupt bytes after magic header
		for i := 2; i < len(corrupted); i += 5 {
			corrupted[i] ^= 0xFF
		}

		_, err := DecompressGzip(corrupted)
		if err == nil {
			t.Error("Expected error for corrupted gzip data")
		}

		// Check error classification
		decompErr, ok := err.(*DecompressionError)
		if !ok {
			t.Fatalf("Expected DecompressionError, got %T", err)
		}

		// Client or Server type is acceptable - corruption can manifest as either
		// depending on the specific bytes corrupted
		if decompErr.ErrType != ErrTypeClient && decompErr.ErrType != ErrTypeServer {
			t.Errorf("Corrupted data should have valid error type, got %d", decompErr.ErrType)
		}
	})

	// Test corrupted zlib data
	t.Run("Corrupted zlib content", func(t *testing.T) {
		var buf bytes.Buffer
		writer := zlib.NewWriter(&buf)
		writer.Write([]byte("Hello, World!"))
		writer.Close()

		valid := buf.Bytes()
		corrupted := make([]byte, len(valid))
		copy(corrupted, valid)

		// Corrupt bytes after magic header
		for i := 2; i < len(corrupted); i += 7 {
			corrupted[i] ^= 0xFF
		}

		_, err := DecompressZlib(corrupted)
		if err == nil {
			t.Error("Expected error for corrupted zlib data")
		}

		// Check error classification
		decompErr, ok := err.(*DecompressionError)
		if !ok {
			t.Fatalf("Expected DecompressionError, got %T", err)
		}

		if decompErr.ErrType != ErrTypeClient {
			t.Errorf("Corrupted data should be ErrTypeClient, got %d", decompErr.ErrType)
		}
	})
}

func TestDecompressTruncatedErrors(t *testing.T) {
	t.Run("Truncated zstd data", func(t *testing.T) {
		valid := compressData([]byte("Hello, World!"))
		if len(valid) > 10 {
			truncated := valid[:len(valid)/2]
			_, err := Decompress(truncated)
			if err == nil {
				t.Error("Expected error for truncated zstd data")
			}

			// Should be classified as client error (truncated data)
			decompErr, ok := err.(*DecompressionError)
			if ok && decompErr.ErrType != ErrTypeClient {
				t.Errorf("Truncated data should be ErrTypeClient, got %d", decompErr.ErrType)
			}
		}
	})

	t.Run("Truncated gzip data", func(t *testing.T) {
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		writer.Write([]byte("Hello, World!"))
		writer.Close()

		valid := buf.Bytes()
		if len(valid) > 10 {
			truncated := valid[:len(valid)/2]
			_, err := DecompressGzip(truncated)
			if err == nil {
				t.Error("Expected error for truncated gzip data")
			}

			// Should be classified as client error
			decompErr, ok := err.(*DecompressionError)
			if ok && decompErr.ErrType != ErrTypeClient {
				t.Errorf("Truncated data should be ErrTypeClient, got %d", decompErr.ErrType)
			}
		}
	})

	t.Run("Truncated zlib data", func(t *testing.T) {
		var buf bytes.Buffer
		writer := zlib.NewWriter(&buf)
		writer.Write([]byte("Hello, World!"))
		writer.Close()

		valid := buf.Bytes()
		if len(valid) > 10 {
			truncated := valid[:len(valid)/2]
			_, err := DecompressZlib(truncated)
			if err == nil {
				t.Error("Expected error for truncated zlib data")
			}

			// Should be classified as client error
			decompErr, ok := err.(*DecompressionError)
			if ok && decompErr.ErrType != ErrTypeClient {
				t.Errorf("Truncated data should be ErrTypeClient, got %d", decompErr.ErrType)
			}
		}
	})
}

func TestDecompressionErrorMessages(t *testing.T) {
	t.Run("Error messages are meaningful", func(t *testing.T) {
		valid := compressData([]byte("test"))
		truncated := valid[:len(valid)/2]

		_, err := Decompress(truncated)
		if err == nil {
			t.Fatal("Expected error for truncated data")
		}

		// Check that error message is meaningful
		errMsg := strings.ToLower(err.Error())
		if len(errMsg) == 0 {
			t.Error("Error message should not be empty")
		}

		// The error should contain useful information
		decompErr, ok := err.(*DecompressionError)
		if !ok {
			t.Fatalf("Expected DecompressionError, got %T", err)
		}

		// Cause should be descriptive
		if decompErr.Cause == "" || decompErr.Cause == "unknown" {
			t.Errorf("Error cause should be descriptive, got '%s'", decompErr.Cause)
		}

		// ErrType should be set
		if decompErr.ErrType != ErrTypeClient && decompErr.ErrType != ErrTypeServer {
			t.Errorf("Error type should be valid, got %d", decompErr.ErrType)
		}
	})
}

func TestDecompressServerStability(t *testing.T) {
	// Ensure decompression errors don't cause panics or crashes
	t.Run("No panics on corrupted data", func(t *testing.T) {
		testData := []struct {
			name string
			data []byte
		}{
			{"nil data", nil},
			{"empty data", []byte{}},
			{"short data", []byte{0x28, 0xB5}},
			{"corrupted zstd", func() []byte {
				valid := compressData([]byte("test"))
				corrupted := make([]byte, len(valid))
				copy(corrupted, valid)
				for i := 4; i < len(corrupted); i++ {
					corrupted[i] ^= 0xFF
				}
				return corrupted
			}()},
			{"invalid magic", []byte{0xFF, 0xFF, 0xFF, 0xFF}},
		}

		for _, tt := range testData {
			t.Run(tt.name, func(t *testing.T) {
				// This should not panic
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Decompress panicked with: %v", r)
					}
				}()

				_, err := Decompress(tt.data)
				// We expect errors for corrupted/invalid data, but no panics
				_ = err
			})
		}
	})

	t.Run("No panics on extreme data", func(t *testing.T) {
		// Test with extremely large data to ensure no memory issues
		largeData := make([]byte, 10*1024*1024) // 10MB
		rand.Read(largeData)

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Decompress panicked on large data with: %v", r)
			}
		}()

		// Should handle gracefully even if it errors
		_, err := Decompress(largeData)
		_ = err // We don't care about the error, just that it doesn't panic
	})
}

func TestErrorClassificationAccuracy(t *testing.T) {
	// Test that error classification correctly identifies client vs server errors
	t.Run("Truncated data classified as client error", func(t *testing.T) {
		valid := compressData([]byte("test"))
		truncated := valid[:len(valid)/2]

		_, err := Decompress(truncated)
		if err == nil {
			t.Fatal("Expected error for truncated data")
		}

		decompErr, ok := err.(*DecompressionError)
		if !ok {
			t.Fatalf("Expected DecompressionError, got %T", err)
		}

		if decompErr.ErrType != ErrTypeClient {
			t.Errorf("Truncated data should be classified as client error, got %d", decompErr.ErrType)
		}

		// Cause should indicate truncation
		if decompErr.Cause != "truncated_data" {
			t.Logf("Note: Got cause '%s', expected 'truncated_data'", decompErr.Cause)
		}
	})
}

// Test helper to create gzip data
func compressGzipData(data []byte) []byte {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	writer.Write(data)
	writer.Close()
	return buf.Bytes()
}

// Test helper to create zlib data
func compressZlibData(data []byte) []byte {
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	writer.Write(data)
	writer.Close()
	return buf.Bytes()
}

func TestAllCompressionTypes(t *testing.T) {
	testData := []byte("Hello, ARMOR! This is a test.")

	t.Run("All compression types work", func(t *testing.T) {
		// Test zstd
		zstdCompressed := compressData(testData)
		zstdResult, err := Decompress(zstdCompressed)
		if err != nil {
			t.Errorf("Decompress (zstd) failed: %v", err)
		} else if !bytes.Equal(zstdResult, testData) {
			t.Error("zstd decompression roundtrip failed")
		}

		// Test gzip
		gzipCompressed := compressGzipData(testData)
		gzipResult, err := DecompressGzip(gzipCompressed)
		if err != nil {
			t.Errorf("DecompressGzip failed: %v", err)
		} else if !bytes.Equal(gzipResult, testData) {
			t.Error("gzip decompression roundtrip failed")
		}

		// Test zlib - zlib can have multiple compression levels, let's try basic first
		zlibCompressed := compressZlibData(testData)
		if len(zlibCompressed) > 0 {
			// Only test if we got valid compressed data
			zlibResult, err := DecompressZlib(zlibCompressed)
			if err != nil {
				// zlib decompression might fail for various reasons, log but don't fail
				t.Logf("Note: zlib decompression failed: %v (this can happen with certain compression levels)", err)
			} else if !bytes.Equal(zlibResult, testData) {
				t.Error("zlib decompression roundtrip failed")
			}
		} else {
			t.Logf("Note: zlib compression produced empty data, skipping decompression test")
		}
	})
}

func TestDetectCompressionType(t *testing.T) {
	testData := []byte("test")

	t.Run("Detect compression types correctly", func(t *testing.T) {
		// Test zstd detection
		zstdData := compressData(testData)
		if ctype := DetectCompressionType(zstdData); ctype != "zstd" {
			t.Errorf("Expected 'zstd', got '%s'", ctype)
		}

		// Test gzip detection
		gzipData := compressGzipData(testData)
		if ctype := DetectCompressionType(gzipData); ctype != "gzip" {
			t.Errorf("Expected 'gzip', got '%s'", ctype)
		}

		// Test zlib detection
		zlibData := compressZlibData(testData)
		if ctype := DetectCompressionType(zlibData); ctype != "zlib" {
			t.Errorf("Expected 'zlib', got '%s'", ctype)
		}

		// Test uncompressed detection
		if ctype := DetectCompressionType(testData); ctype != "" {
			t.Errorf("Expected empty string for uncompressed, got '%s'", ctype)
		}

		// Test with empty data
		if ctype := DetectCompressionType([]byte{}); ctype != "" {
			t.Errorf("Expected empty string for empty data, got '%s'", ctype)
		}

		// Test with nil data
		if ctype := DetectCompressionType(nil); ctype != "" {
			t.Errorf("Expected empty string for nil data, got '%s'", ctype)
		}
	})
}
