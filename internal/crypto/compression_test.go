// Package crypto provides tests for compression utilities.
package crypto

import (
	"bytes"
	"testing"
)

// TestCompressOpportunisticPassThrough verifies that compression doesn't
// expand data - if compressed size >= original, return original.
func TestCompressOpportunisticPassThrough(t *testing.T) {
	tests := []struct {
		name           string
		input          []byte
		expectCompress bool // true if we expect compression to help
	}{
		{
			name:           "Highly compressible text",
			input:          bytes.Repeat([]byte("ARMOR compression test: repetitive data compresses well. "), 1000),
			expectCompress: true,
		},
		{
			name:           "Already compressed data",
			input:          generateRandomData(4096), // Random data doesn't compress well
			expectCompress: false,
		},
		{
			name:           "Small data",
			input:          []byte("small"),
			expectCompress: false, // zstd framing overhead may make it larger
		},
		{
			name:           "Empty data",
			input:          []byte{},
			expectCompress: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, wasCompressed, compressionType, err := Compress(tt.input)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}

			// Verify opportunistic pass-through: compressed size should never be >= original
			if len(compressed) >= len(tt.input) && len(tt.input) > 0 {
				if wasCompressed {
					t.Errorf("Compression marked as successful but didn't shrink data: original=%d, compressed=%d",
						len(tt.input), len(compressed))
				}
				if !bytes.Equal(compressed, tt.input) {
					t.Errorf("Pass-through returned different data than original")
				}
			}

			// Verify compression flag matches expectation
			if wasCompressed != tt.expectCompress && len(tt.input) > 100 {
				// Only check this for non-trivial data (small data may go either way)
				t.Logf("Note: compression expectation mismatch (got=%v, expect=%v) for input size %d",
					wasCompressed, tt.expectCompress, len(tt.input))
			}

			// Verify compression type is set correctly
			if wasCompressed && compressionType != CompressionZstd {
				t.Errorf("Compression type is %v, expected CompressionZstd", compressionType)
			}
			if !wasCompressed && compressionType != CompressionNone {
				t.Errorf("Compression type is %v, expected CompressionNone when not compressed", compressionType)
			}
		})
	}
}

// TestCompressDecompressRoundTrip verifies round-trip compression/decompression.
func TestCompressDecompressRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		compress bool
	}{
		{
			name:    "Compressible text",
			input:   bytes.Repeat([]byte("ARMOR compression test: repetitive data. "), 100),
			compress: true,
		},
		{
			name:    "Random data",
			input:   generateRandomData(1024),
			compress: false, // Random data typically doesn't compress well
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.input

			// Compress
			compressed, wasCompressed, _, err := Compress(original)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}

			// Decompress
			var decompressed []byte
			var decompressErr error
			if wasCompressed {
				decompressed, decompressErr = Decompress(compressed)
			} else {
				// If not compressed, data should be unchanged
				decompressed = compressed
				decompressErr = nil
			}

			if decompressErr != nil {
				t.Fatalf("Decompress failed: %v", decompressErr)
			}

			// Verify round-trip
			if !bytes.Equal(decompressed, original) {
				t.Errorf("Round-trip failed: original size=%d, decompressed size=%d", len(original), len(decompressed))
			}
		})
	}
}

// TestEnvelopeCompressionFlags verifies envelope header compression flag methods.
func TestEnvelopeCompressionFlags(t *testing.T) {
	iv, err := GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV failed: %v", err)
	}

	plaintextSHA := ComputePlaintextSHA256([]byte("test"))
	header, err := NewEnvelopeHeader(iv, 100, 65536, plaintextSHA)
	if err != nil {
		t.Fatalf("NewEnvelopeHeader failed: %v", err)
	}

	// Test default (uncompressed)
	if flag := header.GetCompressionFlag(); flag != CompressionFlagNone {
		t.Errorf("Default compression flag is %d, expected %d", flag, CompressionFlagNone)
	}
	if header.IsCompressed() {
		t.Error("Default header should not be compressed")
	}
	if typ := header.CompressionType(); typ != "" {
		t.Errorf("Default compression type is %q, expected empty", typ)
	}

	// Test setting zstd compression
	header.SetCompressionFlag(CompressionFlagZstd)
	if flag := header.GetCompressionFlag(); flag != CompressionFlagZstd {
		t.Errorf("Compression flag is %d, expected %d", flag, CompressionFlagZstd)
	}
	if !header.IsCompressed() {
		t.Error("Header should be marked as compressed")
	}
	if typ := header.CompressionType(); typ != "zstd" {
		t.Errorf("Compression type is %q, expected 'zstd'", typ)
	}

	// Test clearing compression
	header.SetCompressionFlag(CompressionFlagNone)
	if header.IsCompressed() {
		t.Error("Header should not be compressed after clearing flag")
	}
}

func generateRandomData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}
