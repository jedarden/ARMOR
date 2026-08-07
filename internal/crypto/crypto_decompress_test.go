package crypto

import (
	"bytes"
	"crypto/rand"
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
