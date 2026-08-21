package crypto

import (
	"bytes"
	"compress/gzip"
	"compress/flate"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// Compression helper functions for testing
func compressVerifyData(data []byte) []byte {
	var buf bytes.Buffer
	encoder, err := zstd.NewWriter(&buf)
	if err != nil {
		panic(err)
	}
	encoder.Write(data)
	encoder.Close()
	return buf.Bytes()
}

func compressVerifyDataGzip(data []byte) []byte {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	writer.Write(data)
	writer.Close()
	return buf.Bytes()
}

func compressVerifyDataZlib(data []byte) []byte {
	var buf bytes.Buffer
	writer, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	writer.Write(data)
	writer.Close()
	return buf.Bytes()
}

// TestVerifyIntegration_WithGETHelpers demonstrates integration with GET/decompression helpers.
// This shows the complete workflow: GET → Decrypt → Decompress → Verify
func TestVerifyIntegration_WithGETHelpers(t *testing.T) {
	t.Run("full object GET workflow", func(t *testing.T) {
		// Simulate the GET workflow:
		// 1. Original plaintext data
		originalPlaintext := []byte("This is the original ARMOR encrypted object data with some content for testing verification integration.")

		// 2. Compress the original (simulating upload path)
		compressed := compressVerifyDataGzip(originalPlaintext)

		// 3. In real ARMOR: compressed → encrypt → upload to B2
		// For this test, we skip encryption and directly test decompression verification

		// 4. GET path: retrieve → decrypt → decompress
		// Simulating: decompressed = Decompress(ciphertext)
		decompressed, err := DecompressGzip(compressed)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		// 5. Verify the decompressed data matches original
		result := VerifyDecompression(decompressed, originalPlaintext)

		if !result.Pass {
			t.Errorf("GET workflow verification failed: %s", result.Diagnostic)
		}

		t.Logf("GET workflow verified: %s", result.Diagnostic)
	})

	t.Run("GET workflow with corruption detection", func(t *testing.T) {
		original := []byte("Original data for corruption detection test")

		// Compress
		compressed := compressVerifyDataGzip(original)

		// Decompress
		decompressed, err := DecompressGzip(compressed)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		// Simulate corruption in the decompressed output
		// (In real ARMOR, this could come from:
		//  - Bit rot in storage
		//  - Transmission errors
		//  - Decompression bugs
		//  - Memory corruption)
		corrupted := make([]byte, len(decompressed))
		copy(corrupted, decompressed)
		corrupted[5] = 0xFF // Corrupt byte 5

		// Verify - should detect corruption
		result := VerifyDecompression(corrupted, original)

		if result.Pass {
			t.Error("Corruption NOT detected in GET workflow")
		}

		if result.Error == nil {
			t.Fatal("Expected error details for corruption")
		}

		t.Logf("Corruption detected in GET workflow at offset %d: %s",
			result.Error.Offset, result.Diagnostic)
	})

	t.Run("GET workflow with multiple compression formats", func(t *testing.T) {
		original := []byte("Test data for multiple compression format verification")

		formats := []struct {
			name       string
			compress   func([]byte) []byte
			decompress func([]byte) ([]byte, error)
		}{
			{
				name:       "gzip",
				compress:   compressVerifyDataGzip,
				decompress: DecompressGzip,
			},
			{
				name:       "zstd",
				compress:   compressVerifyData,
				decompress: Decompress,
			},
		}

		for _, format := range formats {
			t.Run(format.name, func(t *testing.T) {
				// Compress
				compressed := format.compress(original)

				// Decompress
				decompressed, err := format.decompress(compressed)
				if err != nil {
					t.Fatalf("Failed to decompress with %s: %v", format.name, err)
				}

				// Verify
				result := VerifyDecompression(decompressed, original)

				if !result.Pass {
					t.Errorf("%s GET workflow verification failed: %s",
						format.name, result.Diagnostic)
				}

				t.Logf("%s workflow verified: %s", format.name, result.Diagnostic)
			})
		}
	})
}

// TestVerifyIntegration_WithRangeHelpers demonstrates integration with range request helpers.
// This shows the workflow: Range GET → Decrypt → Decompress → Verify Range
func TestVerifyIntegration_WithRangeHelpers(t *testing.T) {
	t.Run("range request workflow", func(t *testing.T) {
		// Create a 10KB original object
		original := make([]byte, 10*1024)
		for i := range original {
			original[i] = byte(i % 256)
		}

		// Test various range requests
		ranges := []struct {
			name        string
			startOffset int64
			length      int64
		}{
			{
				name:        "first 1KB",
				startOffset: 0,
				length:      1024,
			},
			{
				name:        "middle 1KB",
				startOffset: 4096,
				length:      1024,
			},
			{
				name:        "last 1KB",
				startOffset: 9216,
				length:      1024,
			},
			{
				name:        "small range (64 bytes)",
				startOffset: 5120,
				length:      64,
			},
			{
				name:        "single byte",
				startOffset: 7000,
				length:      1,
			},
		}

		for _, r := range ranges {
			t.Run(r.name, func(t *testing.T) {
				// Extract the expected range from original
				expectedRange := original[r.startOffset : r.startOffset+r.length]

				// Simulate range GET workflow:
				// 1. ARMOR translates plaintext range to encrypted ranges
				//    translation := TranslateRange(startOffset, endOffset, totalSize, blockSize, headerSize)
				// 2. ARMOR fetches encrypted range from B2
				// 3. ARMOR decrypts the range
				// 4. ARMOR decompresses if needed
				// For this test, we directly verify the range

				// Verify the range matches
				result := VerifyRangeDecompressionWithBounds(
					original,
					expectedRange,
					r.startOffset,
					r.length,
				)

				if !result.Pass {
					t.Errorf("Range verification failed: %s", result.Diagnostic)
				}

				t.Logf("Range %s verified: %s", r.name, result.Diagnostic)
			})
		}
	})

	t.Run("range request with corruption", func(t *testing.T) {
		original := make([]byte, 2048)
		for i := range original {
			original[i] = byte(i % 256)
		}

		// Request a range
		startOffset := int64(512)
		length := int64(256)
		expectedRange := original[startOffset : startOffset+length]

		// Simulate corruption in the range
		corruptedRange := make([]byte, length)
		copy(corruptedRange, expectedRange)
		corruptedRange[50] = 0xFF // Corrupt a byte in the middle

		// Verify - should detect corruption
		result := VerifyRangeDecompressionWithBounds(
			original,
			corruptedRange,
			startOffset,
			length,
		)

		if result.Pass {
			t.Error("Range corruption NOT detected")
		}

		// Should report relative offset within the range
		if result.Error == nil {
			t.Fatal("Expected error details")
		}

		// Relative offset should be 50 (within the range)
		relativeOffset := int(result.Error.Offset)
		if relativeOffset != 50 {
			t.Errorf("Expected relative offset 50, got %d", relativeOffset)
		}

		// Absolute offset should be startOffset + 50 = 562
		absoluteOffset := startOffset + int64(relativeOffset)
		t.Logf("Range corruption detected: relative offset %d, absolute offset %d",
			relativeOffset, absoluteOffset)
		t.Logf("Diagnostic: %s", result.Diagnostic)
	})

	t.Run("range request at boundaries", func(t *testing.T) {
		// Create a smaller object for boundary testing
		original := make([]byte, 512)
		for i := range original {
			original[i] = byte(i)
		}

		boundaries := []struct {
			name   string
			offset int64
			length int64
		}{
			{
				name:   "range at very start",
				offset: 0,
				length: 32,
			},
			{
				name:   "range at very end",
				offset: 480,
				length: 32,
			},
			{
				name:   "entire object as range",
				offset: 0,
				length: 512,
			},
		}

		for _, b := range boundaries {
			t.Run(b.name, func(t *testing.T) {
				expectedRange := original[b.offset : b.offset+b.length]

				result := VerifyRangeDecompressionWithBounds(
					original,
					expectedRange,
					b.offset,
					b.length,
				)

				if !result.Pass {
					t.Errorf("Boundary range verification failed: %s", result.Diagnostic)
				}

				t.Logf("Boundary range %s verified: %s", b.name, result.Diagnostic)
			})
		}
	})
}

// TestVerifyIntegration_RangeTranslationIntegration shows how verification integrates
// with the RangeTranslation helper for real ARMOR workflows.
func TestVerifyIntegration_RangeTranslationIntegration(t *testing.T) {
	// Create a 100KB plaintext object
	totalPlaintextSize := int64(100 * 1024)
	plaintext := make([]byte, totalPlaintextSize)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	blockSize := 64 * 1024 // 64KB blocks (ARMOR default)
	headerSize := 1024      // Example header size

	t.Run("translate and verify single range", func(t *testing.T) {
		// Request a plaintext range
		plaintextStart := int64(10 * 1024) // 10KB offset
		plaintextEnd := int64(20 * 1024)   // 20KB offset (10KB range)

		// Translate to encrypted ranges
		translation, err := TranslateRange(
			plaintextStart,
			plaintextEnd,
			totalPlaintextSize,
			blockSize,
			headerSize,
		)

		if err != nil {
			t.Fatalf("TranslateRange failed: %v", err)
		}

		// Verify the translation
		if translation.PlaintextStart != plaintextStart {
			t.Errorf("PlaintextStart mismatch: got %d, want %d",
				translation.PlaintextStart, plaintextStart)
		}

		// In real ARMOR workflow:
		// 1. Fetch encrypted data from B2 using translation.DataOffset and translation.DataLength
		// 2. Decrypt the fetched range
		// 3. Decompress if needed
		// 4. Verify against expected plaintext range

		// For this test, we directly extract and verify the plaintext range
		expectedRange := plaintext[plaintextStart : plaintextEnd+1]

		result := VerifyRangeDecompressionWithBounds(
			plaintext,
			expectedRange,
			plaintextStart,
			plaintextEnd-plaintextStart+1,
		)

		if !result.Pass {
			t.Errorf("Translated range verification failed: %s", result.Diagnostic)
		}

		t.Logf("Translated range verified: %s", result.Diagnostic)
		t.Logf("Translation: blocks %d-%d, data offset %d, data length %d",
			translation.BlockStart, translation.BlockEnd,
			translation.DataOffset, translation.DataLength)
	})

	t.Run("corrupted range detection with translation", func(t *testing.T) {
		// Request a specific range
		plaintextStart := int64(15 * 1024)
		plaintextEnd := int64(25 * 1024)

		// Translate
		translation, err := TranslateRange(
			plaintextStart,
			plaintextEnd,
			totalPlaintextSize,
			blockSize,
			headerSize,
		)

		if err != nil {
			t.Fatalf("TranslateRange failed: %v", err)
		}

		// Extract the range and corrupt it
		expectedRange := plaintext[plaintextStart : plaintextEnd+1]
		corruptedRange := make([]byte, len(expectedRange))
		copy(corruptedRange, expectedRange)

		// Simulate corruption in the middle of the range
		corruptionOffset := len(corruptedRange) / 2
		corruptedRange[corruptionOffset] = 0xFF

		// Verify - should detect corruption
		result := VerifyRangeDecompressionWithBounds(
			plaintext,
			corruptedRange,
			plaintextStart,
			plaintextEnd-plaintextStart+1,
		)

		if result.Pass {
			t.Error("Corrupted range NOT detected")
		}

		// Check that we got the correct relative offset
		relativeOffset := int(result.Error.Offset)
		if relativeOffset != corruptionOffset {
			t.Errorf("Expected corruption at relative offset %d, got %d",
				corruptionOffset, relativeOffset)
		}

		// Calculate absolute offset
		absoluteOffset := plaintextStart + int64(relativeOffset)

		t.Logf("Corruption detected at relative offset %d (absolute %d)",
			relativeOffset, absoluteOffset)
		t.Logf("Affected block: %d (corruption is in block %d of range %d-%d)",
			translation.BlockStart+int(relativeOffset)/blockSize,
			relativeOffset/blockSize,
			translation.BlockStart, translation.BlockEnd)
	})
}

// TestVerifyIntegration_CompressionWithVerification tests compression/decompression
// with integrated verification for real-world scenarios.
func TestVerifyIntegration_CompressionWithVerification(t *testing.T) {
	t.Run("compress and verify round-trip", func(t *testing.T) {
		// Various real-world data patterns
		testData := []struct {
			name  string
			data  []byte
			desc  string
		}{
			{
				name: "text data",
				data: []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. " +
					"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."),
				desc: "Plain text document",
			},
			{
				name: "binary data",
				data: func() []byte {
					data := make([]byte, 1024)
					for i := range data {
						data[i] = byte(i % 256)
					}
					return data
				}(),
				desc: "Binary pattern",
			},
			{
				name: "JSON data",
				data: []byte(`{"name":"test","value":123,"nested":{"key":"value"}}`),
				desc: "JSON document",
			},
			{
				name: "repeated pattern",
				data: func() []byte {
					data := make([]byte, 512)
					pattern := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
					for i := 0; i < len(data); i += len(pattern) {
						copy(data[i:], pattern)
					}
					return data
				}(),
				desc: "Repeating byte pattern",
			},
		}

		for _, test := range testData {
			t.Run(test.name, func(t *testing.T) {
				// Compress the data
				compressed := compressVerifyDataGzip(test.data)

				// Decompress
				decompressed, err := DecompressGzip(compressed)
				if err != nil {
					t.Fatalf("Decompression failed: %v", err)
				}

				// Verify round-trip
				result := VerifyDecompression(decompressed, test.data)

				if !result.Pass {
					t.Errorf("%s round-trip verification failed: %s",
						test.desc, result.Diagnostic)
				}

				t.Logf("%s round-trip verified: %d bytes → %d bytes compressed → %d bytes verified",
					test.desc, len(test.data), len(compressed), len(decompressed))
			})
		}
	})
}

// TestVerifyIntegration_ContextLogging demonstrates using VerifyDecompressionWithContext
// for structured logging in distributed systems.
func TestVerifyIntegration_ContextLogging(t *testing.T) {
	t.Run("contextual verification logging", func(t *testing.T) {
		original := []byte("Test data for contextual logging")

		// Create various contexts for different scenarios
		contexts := []struct {
			name    string
			context string
		}{
			{
				name:    "object context",
				context: "bucket=backup-bucket, key=daily-backup.db, version=3",
			},
			{
				name:    "DR drill context",
				context: "dr-drill=true, mode=direct-only, attempt=2/5",
			},
			{
				name:    "tenant context",
				context: "tenant=acme-corp, object=financial-report.pdf",
			},
			{
				name:    "path context",
				context: "path=armor, checksum=expected_vs_actual, verify=full",
			},
		}

		for _, ctx := range contexts {
			t.Run(ctx.name, func(t *testing.T) {
				// Verify with context
				result := VerifyDecompressionWithContext(
					original,
					original,
					ctx.context,
				)

				if !result.Pass {
					t.Errorf("Verification with context failed: %s", result.Diagnostic)
				}

				// Check that context is in the diagnostic
				if !bytes.Contains([]byte(result.Diagnostic), []byte("["+ctx.context+"]")) {
					t.Errorf("Context not found in diagnostic: %s", result.Diagnostic)
				}

				t.Logf("Contextual verification: %s", result.Diagnostic)
			})
		}
	})

	t.Run("contextual error logging", func(t *testing.T) {
		original := []byte("Original data")
		corrupted := []byte{0xFF, 0xFF, 0xFF, 0xFF} // Completely wrong

		context := "object=my-bucket/key.db, version=123, path=armor"

		result := VerifyDecompressionWithContext(corrupted, original, context)

		if result.Pass {
			t.Error("Corruption NOT detected with context")
		}

		// Verify context is in error message
		if !bytes.Contains([]byte(result.Diagnostic), []byte("["+context+"]")) {
			t.Errorf("Context not found in error diagnostic: %s", result.Diagnostic)
		}

		t.Logf("Contextual error logging: %s", result.Diagnostic)
	})
}

// TestVerifyIntegration_PatternDetection demonstrates using AnalyzeByteDifferences
// for forensic analysis of corruption patterns.
func TestVerifyIntegration_PatternDetection(t *testing.T) {
	t.Run("detect corruption pattern", func(t *testing.T) {
		original := make([]byte, 1024)
		for i := range original {
			original[i] = byte(i % 256)
		}

		// Create various corruption patterns
		patterns := []struct {
			name      string
			corrupt   func([]byte) []byte
			expectMsg string
		}{
			{
				name: "single bit flip",
				corrupt: func(data []byte) []byte {
					corrupted := make([]byte, len(data))
					copy(corrupted, data)
					corrupted[100] ^= 0x01 // Flip one bit
					return corrupted
				},
				expectMsg: "single-bit",
			},
			{
				name: "burst corruption",
				corrupt: func(data []byte) []byte {
					corrupted := make([]byte, len(data))
					copy(corrupted, data)
					for i := 200; i < 250; i++ {
						corrupted[i] = 0x00 // 50-byte zero burst
					}
					return corrupted
				},
				expectMsg: "burst",
			},
			{
				name: "sparse corruption",
				corrupt: func(data []byte) []byte {
					corrupted := make([]byte, len(data))
					copy(corrupted, data)
					for _, offset := range []int{50, 150, 250, 350, 450} {
						corrupted[offset] = 0xFF
					}
					return corrupted
				},
				expectMsg: "sparse",
			},
		}

		for _, pattern := range patterns {
			t.Run(pattern.name, func(t *testing.T) {
				corrupted := pattern.corrupt(original)

				// Fast verification (stops at first error)
				result := VerifyDecompression(corrupted, original)

				if result.Pass {
					t.Errorf("%s pattern NOT detected", pattern.name)
				}

				t.Logf("%s detected at offset %d: %s",
					pattern.name, result.Error.Offset, result.Diagnostic)

				// Forensic analysis (scans entire data)
				stats := AnalyzeByteDifferences(corrupted, original)

				t.Logf("Forensic analysis: %s", stats.Summary())

				// Get top mismatching bytes
				topMismatches := stats.TopMismatches(5)
				if len(topMismatches) > 0 {
					t.Logf("Top mismatching byte values:")
					for _, m := range topMismatches {
						t.Logf("  0x%02X: %d occurrences", m.Byte, m.Count)
					}
				}
			})
		}
	})
}

// TestVerifyIntegration_DecompressionHelpers tests the actual decompression helper functions.
func TestVerifyIntegration_DecompressionHelpers(t *testing.T) {
	t.Run("Decompress function", func(t *testing.T) {
		// Test with zstd-compressed data
		original := []byte("Test data for Decompress() helper")

		compressed := compressVerifyData(original)

		decompressed, err := Decompress(compressed)
		if err != nil {
			t.Fatalf("Decompress failed: %v", err)
		}

		result := VerifyDecompression(decompressed, original)

		if !result.Pass {
			t.Errorf("Decompress() helper verification failed: %s", result.Diagnostic)
		}

		t.Logf("Decompress() helper verified: %s", result.Diagnostic)
	})

	t.Run("IsCompressed detection", func(t *testing.T) {
		// Compressed data should be detected
		original := []byte("Test data")
		compressed := compressVerifyData(original)

		if !IsCompressed(compressed) {
			t.Error("IsCompressed() should return true for compressed data")
		}

		// Uncompressed data should not be detected as compressed
		if IsCompressed(original) {
			t.Error("IsCompressed() should return false for uncompressed data")
		}

		// Empty data should not crash
		if IsCompressed([]byte{}) {
			t.Error("IsCompressed() should return false for empty data")
		}

		t.Logf("IsCompressed() detection working correctly")
	})

	t.Run("decompress various formats", func(t *testing.T) {
		original := []byte("Test data for various decompression formats")

		formats := []struct {
			name       string
			compress   func([]byte) []byte
			decompress func([]byte) ([]byte, error)
		}{
			{
				name:       "gzip",
				compress:   compressVerifyDataGzip,
				decompress: DecompressGzip,
			},
			{
				name:       "zstd",
				compress:   compressVerifyData,
				decompress: Decompress,
			},
		}

		for _, format := range formats {
			t.Run(format.name, func(t *testing.T) {
				// Compress
				compressed := format.compress(original)

				// Verify compressed data is different from original
				if bytes.Equal(compressed, original) {
					t.Errorf("%s compression produced identical output", format.name)
				}

				// Decompress
				decompressed, err := format.decompress(compressed)
				if err != nil {
					t.Fatalf("%s decompression failed: %v", format.name, err)
				}

				// Verify round-trip
				result := VerifyDecompression(decompressed, original)

				if !result.Pass {
					t.Errorf("%s round-trip verification failed: %s",
						format.name, result.Diagnostic)
				}

				// Calculate compression ratio
				ratio := float64(len(compressed)) / float64(len(original)) * 100.0

				t.Logf("%s round-trip verified: %d bytes → %d bytes (%.1f%%)",
					format.name, len(original), len(compressed), ratio)
			})
		}
	})
}

// TestVerifyIntegration_WithStreamDecompression tests verification with streaming decompression.
func TestVerifyIntegration_WithStreamDecompression(t *testing.T) {
	t.Run("stream decompress and verify", func(t *testing.T) {
		// Create test data
		original := []byte("Streaming decompression test data for ARMOR verification integration")

		// Compress
		var compressedBuf bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressedBuf)
		gzipWriter.Write(original)
		gzipWriter.Close()

		compressed := compressedBuf.Bytes()

		// Stream decompress
		gzipReader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			t.Fatalf("Failed to create gzip reader: %v", err)
		}

		decompressed, err := io.ReadAll(gzipReader)
		if err != nil {
			t.Fatalf("Failed to read decompressed data: %v", err)
		}

		// Verify
		result := VerifyDecompression(decompressed, original)

		if !result.Pass {
			t.Errorf("Stream decompression verification failed: %s", result.Diagnostic)
		}

		t.Logf("Stream decompression verified: %s", result.Diagnostic)
	})

	t.Run("stream decompress with corruption", func(t *testing.T) {
		original := []byte("Test data with corruption in stream")

		// Compress
		var compressedBuf bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressedBuf)
		gzipWriter.Write(original)
		gzipWriter.Close()

		compressed := compressedBuf.Bytes()

		// Corrupt the compressed data
		corruptedCompressed := make([]byte, len(compressed))
		copy(corruptedCompressed, compressed)
		corruptedCompressed[len(corruptedCompressed)/2] ^= 0xFF

		// Attempt stream decompress
		gzipReader, err := gzip.NewReader(bytes.NewReader(corruptedCompressed))

		// Either:
		// 1. Reader creation fails (corruption detected early)
		// 2. Reader succeeds but produces wrong output (verification catches it)
		if err != nil {
			t.Logf("Stream corruption detected early: %v", err)
		} else {
			decompressed, err := io.ReadAll(gzipReader)
			if err != nil {
				t.Logf("Stream corruption detected during read: %v", err)
			} else {
				// Reader succeeded - verify output
				result := VerifyDecompression(decompressed, original)

				if result.Pass {
					t.Error("Stream corruption NOT detected")
				}

				t.Logf("Stream corruption detected by verification: %s", result.Diagnostic)
			}
		}
	})
}
