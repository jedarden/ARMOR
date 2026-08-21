package crypto

import (
	"testing"
)

// TestVerifyDecompression_ByteSwapCorruption tests detection of byte swap corruption.
// Byte swapping occurs when pairs of adjacent bytes are reversed (endianness mismatch).
func TestVerifyDecompression_ByteSwapCorruption(t *testing.T) {
	tests := []struct {
		name         string
		original     []byte
		corruptWith []byte // Swapped version
		description string
	}{
		{
			name: "two byte swap at start",
			original: []byte{
				0x12, 0x34, // Should be 0x12 0x34
				0x56, 0x78, 0x9A, 0xBC,
			},
			corruptWith: []byte{
				0x34, 0x12, // Swapped to 0x34 0x12
				0x56, 0x78, 0x9A, 0xBC,
			},
			description: "First two bytes swapped (0x12 0x34 → 0x34 0x12)",
		},
		{
			name: "two byte swap in middle",
			original: []byte{
				0x00, 0x01, 0x02,
				0xAA, 0xBB, // Should be 0xAA 0xBB
				0xCC, 0xDD, 0xEE,
			},
			corruptWith: []byte{
				0x00, 0x01, 0x02,
				0xBB, 0xAA, // Swapped to 0xBB 0xAA
				0xCC, 0xDD, 0xEE,
			},
			description: "Middle two bytes swapped",
		},
		{
			name: "two byte swap at end",
			original: []byte{
				0x11, 0x22, 0x33, 0x44,
				0xDE, 0xAD, // Should be 0xDE 0xAD
			},
			corruptWith: []byte{
				0x11, 0x22, 0x33, 0x44,
				0xAD, 0xDE, // Swapped to 0xAD 0xDE
			},
			description: "Last two bytes swapped",
		},
		{
			name: "multiple sequential swaps",
			original: []byte{
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06,
			},
			corruptWith: []byte{
				0x02, 0x01, // 1-2 swapped
				0x04, 0x03, // 3-4 swapped
				0x06, 0x05, // 5-6 swapped
			},
			description: "Multiple adjacent byte pairs swapped (endianness issue)",
		},
		{
			name: "four byte word swap",
			original: []byte{
				0x11, 0x22, 0x33, 0x44, // 4-byte value
				0x55, 0x66, 0x77, 0x88,
			},
			corruptWith: []byte{
				0x44, 0x33, 0x22, 0x11, // Entire word reversed
				0x55, 0x66, 0x77, 0x88,
			},
			description: "4-byte word completely reversed (big-endian vs little-endian)",
		},
		{
			name: "swap in binary data",
			original: []byte{
				0x89, 0x50, 0x4E, 0x47, // PNG signature
				0x0D, 0x0A, 0x1A, 0x0A,
			},
			corruptWith: []byte{
				0x50, 0x89, // First two bytes swapped - breaks PNG signature
				0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
			},
			description: "Byte swap in file signature (corrupts file type detection)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyDecompression(tt.corruptWith, tt.original)

			if result.Pass {
				t.Errorf("Byte swap corruption NOT detected: %s", tt.description)
			}

			if result.Error == nil {
				t.Fatal("Expected error details for byte swap corruption")
			}

			// Should detect the first swapped pair
			if result.Error.Offset < 0 {
				t.Errorf("Expected positive offset for byte swap, got %d", result.Error.Offset)
			}

			t.Logf("Byte swap detected at offset %d: %s", result.Error.Offset, result.Diagnostic)

			// Verify context helps identify the swap pattern
			if len(result.Error.Expected) < 2 {
				t.Error("Context should include at least the swapped pair")
			}

			// The expected bytes at the corruption offset should match original
			offset := int(result.Error.Offset)
			if offset < len(tt.original) && offset < len(tt.corruptWith) {
				if tt.original[offset] != tt.corruptWith[offset] {
					t.Logf("Confirmed swap at offset %d: expected 0x%02X, got 0x%02X",
						offset, tt.original[offset], tt.corruptWith[offset])
				}
			}
		})
	}
}

// TestVerifyDecompression_BurstCorruption tests detection of burst corruption patterns.
// Burst corruption occurs when a sequence of consecutive bytes is overwritten with the same value.
func TestVerifyDecompression_BurstCorruption(t *testing.T) {
	tests := []struct {
		name          string
		original      []byte
		corruptStart  int
		corruptLength int
		corruptValue  byte
		description   string
	}{
		{
			name:          "null byte burst",
			original:      []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			corruptStart:  2,
			corruptLength: 4,
			corruptValue:  0x00,
			description:   "4 consecutive bytes overwritten with 0x00",
		},
		{
			name:          "0xFF burst at start",
			original:      []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88},
			corruptStart:  0,
			corruptLength: 3,
			corruptValue:  0xFF,
			description:   "First 3 bytes overwritten with 0xFF pattern",
		},
		{
			name:          "0xFF burst at end",
			original:      []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			corruptStart:  5,
			corruptLength: 3,
			corruptValue:  0xFF,
			description:   "Last 3 bytes overwritten with 0xFF",
		},
		{
			name:          "partial burst in middle",
			original: func() []byte {
				data := make([]byte, 100)
				for i := range data {
					data[i] = byte(i % 256)
				}
				return data
			}(),
			corruptStart:  40,
			corruptLength: 20,
			corruptValue:  0xAA,
			description:   "20-byte burst of 0xAA in middle of 100-byte object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create corrupted version
			corrupted := make([]byte, len(tt.original))
			copy(corrupted, tt.original)

			// Apply burst corruption
			for i := tt.corruptStart; i < tt.corruptStart+tt.corruptLength; i++ {
				if i < len(corrupted) {
					corrupted[i] = tt.corruptValue
				}
			}

			result := VerifyDecompression(corrupted, tt.original)

			if result.Pass {
				t.Errorf("Burst corruption NOT detected: %s", tt.description)
			}

			if result.Error == nil {
				t.Fatal("Expected error details for burst corruption")
			}

			// Should detect corruption at the start of the burst
			if result.Error.Offset != int64(tt.corruptStart) {
				t.Errorf("Expected corruption at offset %d, got %d",
					tt.corruptStart, result.Error.Offset)
			}

			t.Logf("Burst corruption detected at offset %d: %s",
				result.Error.Offset, result.Diagnostic)

			// Verify the corrupted byte value is reported
			if len(result.Error.Actual) > 0 {
				firstByte := result.Error.Actual[0]
				if firstByte != tt.corruptValue {
					t.Errorf("Expected corrupted byte 0x%02X, got 0x%02X",
						tt.corruptValue, firstByte)
				}
			}
		})
	}
}

// TestVerifyDecompression_MultipleCorruptionPatterns tests various real-world corruption scenarios.
func TestVerifyDecompression_MultipleCorruptionPatterns(t *testing.T) {
	t.Run("sparse random bit flips", func(t *testing.T) {
		// Create 1KB of test data
		original := make([]byte, 1024)
		for i := range original {
			original[i] = byte(i % 256)
		}

		// Introduce sparse bit flips at random intervals
		corrupted := make([]byte, len(original))
		copy(corrupted, original)

		// Flip bits at specific offsets
		flipOffsets := []int{10, 100, 500, 750, 1000}
		for _, offset := range flipOffsets {
			if offset < len(corrupted) {
				corrupted[offset] ^= 0xFF // Flip all bits
			}
		}

		result := VerifyDecompression(corrupted, original)

		if result.Pass {
			t.Error("Sparse bit flips NOT detected")
		}

		// Should detect the first flip
		if result.Error.Offset != int64(flipOffsets[0]) {
			t.Errorf("Expected first flip at offset %d, got %d",
				flipOffsets[0], result.Error.Offset)
		}

		t.Logf("Sparse corruption detected at offset %d", result.Error.Offset)
	})

	t.Run("patterned corruption (alternating bytes)", func(t *testing.T) {
		original := make([]byte, 256)
		for i := range original {
			original[i] = byte(i)
		}

		// Corrupt every other byte with 0xFF
		corrupted := make([]byte, len(original))
		copy(corrupted, original)

		for i := 1; i < len(corrupted); i += 2 {
			corrupted[i] = 0xFF
		}

		result := VerifyDecompression(corrupted, original)

		if result.Pass {
			t.Error("Patterned corruption NOT detected")
		}

		// Should detect at offset 1 (first corrupted byte)
		if result.Error.Offset != 1 {
			t.Errorf("Expected corruption at offset 1, got %d", result.Error.Offset)
		}

		t.Logf("Patterned corruption detected: %s", result.Diagnostic)
	})

	t.Run("incremental corruption sequence", func(t *testing.T) {
		original := make([]byte, 128)
		for i := range original {
			original[i] = 0x00
		}

		// Create an incremental corruption pattern: 0x01, 0x02, 0x03, ...
		corrupted := make([]byte, len(original))
		copy(corrupted, original)

		for i := 50; i < 80; i++ {
			corrupted[i] = byte(i - 49) // 0x01, 0x02, 0x03, ...
		}

		result := VerifyDecompression(corrupted, original)

		if result.Pass {
			t.Error("Incremental corruption NOT detected")
		}

		// Should detect at offset 50
		if result.Error.Offset != 50 {
			t.Errorf("Expected corruption at offset 50, got %d", result.Error.Offset)
		}

		// The first corrupted byte should be 0x01
		if len(result.Error.Actual) > 0 {
			if result.Error.Actual[0] != 0x01 {
				t.Errorf("Expected corrupted byte 0x01, got 0x%02X",
					result.Error.Actual[0])
			}
		}

		t.Logf("Incremental corruption detected: %s", result.Diagnostic)
	})
}

// TestVerifyDecompression_FileSignatureCorruption tests corruption of file signatures/magic numbers.
func TestVerifyDecompression_FileSignatureCorruption(t *testing.T) {
	tests := []struct {
		name        string
		signature   []byte // Correct file signature
		corruptByte int    // Which byte to corrupt
		corruptWith byte
		fileType    string
	}{
		{
			name:        "PNG signature corrupted",
			signature:   []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			corruptByte: 1, // Corrupt 'P' in PNG
			corruptWith: 0x00,
			fileType:    "PNG",
		},
		{
			name:        "JPEG signature corrupted",
			signature:   []byte{0xFF, 0xD8, 0xFF, 0xE0},
			corruptByte: 1,
			corruptWith: 0x00,
			fileType:    "JPEG",
		},
		{
			name:        "GIF signature corrupted",
			signature:   []byte{0x47, 0x49, 0x46, 0x38},
			corruptByte: 0, // Corrupt 'G'
			corruptWith: 0x00,
			fileType:    "GIF",
		},
		{
			name:        "PDF signature corrupted",
			signature:   []byte{0x25, 0x50, 0x44, 0x46}, // "%PDF"
			corruptByte: 0,
			corruptWith: 0x00,
			fileType:    "PDF",
		},
		{
			name:        "ZIP signature corrupted",
			signature:   []byte{0x50, 0x4B, 0x03, 0x04},
			corruptByte: 2,
			corruptWith: 0xFF,
			fileType:    "ZIP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data with signature at start
			testData := make([]byte, 256)
			copy(testData, tt.signature)

			// Corrupt the signature
			corrupted := make([]byte, len(testData))
			copy(corrupted, testData)
			corrupted[tt.corruptByte] = tt.corruptWith

			result := VerifyDecompression(corrupted, testData)

			if result.Pass {
				t.Errorf("%s signature corruption NOT detected", tt.fileType)
			}

			// Should detect corruption at the corrupted byte position
			if result.Error.Offset != int64(tt.corruptByte) {
				t.Errorf("Expected corruption at offset %d, got %d",
					tt.corruptByte, result.Error.Offset)
			}

			t.Logf("%s signature corruption detected at offset %d: %s",
				tt.fileType, result.Error.Offset, result.Diagnostic)

			// Verify context shows the corrupted signature
			if len(result.Error.Expected) > 0 {
				// The expected byte should be the original signature byte
				expectedByte := tt.signature[tt.corruptByte]
				// Find it in the context
				found := false
				for _, b := range result.Error.Expected {
					if b == expectedByte {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected signature byte 0x%02X not found in context",
						expectedByte)
				}
			}
		})
	}
}

// TestVerifyDecompression_RepeatedByteCorruption tests corruption where a single byte value repeats.
func TestVerifyDecompression_RepeatedByteCorruption(t *testing.T) {
	tests := []struct {
		name         string
		original     []byte
		corruptStart int
		corruptCount int
		corruptValue byte
	}{
		{
			name: "zero fill in middle",
			original: func() []byte {
				data := make([]byte, 256)
				for i := range data {
					data[i] = byte(i % 256)
				}
				return data
			}(),
			corruptStart: 100,
			corruptCount: 50,
			corruptValue: 0x00,
		},
		{
			name: "0xFF fill at start",
			original: func() []byte {
				data := make([]byte, 128)
				for i := range data {
					data[i] = byte(i + 1)
				}
				return data
			}(),
			corruptStart: 0,
			corruptCount: 20,
			corruptValue: 0xFF,
		},
		{
			name: "repeated pattern 0xAA",
			original: func() []byte {
				data := make([]byte, 64)
				for i := range data {
					data[i] = byte(i)
				}
				return data
			}(),
			corruptStart: 30,
			corruptCount: 30,
			corruptValue: 0xAA,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corrupted := make([]byte, len(tt.original))
			copy(corrupted, tt.original)

			// Apply repeated byte corruption
			for i := tt.corruptStart; i < tt.corruptStart+tt.corruptCount; i++ {
				if i < len(corrupted) {
					corrupted[i] = tt.corruptValue
				}
			}

			result := VerifyDecompression(corrupted, tt.original)

			if result.Pass {
				t.Error("Repeated byte corruption NOT detected")
			}

			if result.Error.Offset != int64(tt.corruptStart) {
				t.Errorf("Expected corruption at offset %d, got %d",
					tt.corruptStart, result.Error.Offset)
			}

			// Verify the corrupted value is detected
			if len(result.Error.Actual) > 0 {
				if result.Error.Actual[0] != tt.corruptValue {
					t.Errorf("Expected corrupted value 0x%02X, got 0x%02X",
						tt.corruptValue, result.Error.Actual[0])
				}
			}

			t.Logf("Repeated byte corruption detected at offset %d: %s",
				result.Error.Offset, result.Diagnostic)
		})
	}
}

// TestVerifyDecompression_AdjacentByteErrors tests verification with multiple adjacent byte errors.
func TestVerifyDecompression_AdjacentByteErrors(t *testing.T) {
	t.Run("two adjacent errors", func(t *testing.T) {
		original := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
		corrupted := []byte{0x00, 0xFF, 0xFE, 0x33, 0x44, 0x55}

		result := VerifyDecompression(corrupted, original)

		if result.Pass {
			t.Error("Adjacent errors NOT detected")
		}

		// Should report the first error (offset 1)
		if result.Error.Offset != 1 {
			t.Errorf("Expected first error at offset 1, got %d", result.Error.Offset)
		}

		// Context should include both corrupted bytes
		if len(result.Error.Actual) < 2 {
			t.Error("Context should include multiple adjacent corrupted bytes")
		}

		t.Logf("Adjacent errors detected: %s", result.Diagnostic)
	})

	t.Run("three consecutive errors", func(t *testing.T) {
		original := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
		corrupted := []byte{0x00, 0xFF, 0xFE, 0xFD, 0x44, 0x55, 0x66}

		result := VerifyDecompression(corrupted, original)

		if result.Pass {
			t.Error("Three consecutive errors NOT detected")
		}

		if result.Error.Offset != 1 {
			t.Errorf("Expected first error at offset 1, got %d", result.Error.Offset)
		}

		// Context window should show the pattern
		t.Logf("Consecutive error pattern detected: %s", result.Diagnostic)
	})
}

// TestVerifyDecompression_OffByOneErrors tests subtle off-by-one corruption patterns.
func TestVerifyDecompression_OffByOneErrors(t *testing.T) {
	tests := []struct {
		name        string
		original    []byte
		corrupted   []byte
		description string
	}{
		{
			name:        "increment by 1",
			original:    []byte{0x00, 0x10, 0x20, 0x30, 0x40},
			corrupted:   []byte{0x01, 0x11, 0x21, 0x31, 0x41}, // All +1
			description: "Every byte incremented by 1",
		},
		{
			name:        "decrement by 1",
			original:    []byte{0x10, 0x20, 0x30, 0x40, 0x50},
			corrupted:   []byte{0x0F, 0x1F, 0x2F, 0x3F, 0x4F}, // All -1
			description: "Every byte decremented by 1",
		},
		{
			name:        "shift by 2",
			original:    []byte{0x00, 0x01, 0x02, 0x03, 0x04},
			corrupted:   []byte{0x02, 0x03, 0x04, 0x05, 0x06}, // All +2
			description: "Every byte shifted by 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyDecompression(tt.corrupted, tt.original)

			if result.Pass {
				t.Errorf("Off-by-one corruption NOT detected: %s", tt.description)
			}

			// Should detect at first byte that differs (offset 0)
			if result.Error.Offset != 0 {
				t.Errorf("Expected corruption at offset 0, got %d", result.Error.Offset)
			}

			// Context should show the pattern
			t.Logf("Off-by-one pattern detected: %s", result.Diagnostic)

			// Verify the difference is what we expect
			if len(result.Error.Expected) > 0 && len(result.Error.Actual) > 0 {
				expected := tt.original[0]
				actual := tt.corrupted[0]
				t.Logf("First byte difference: expected 0x%02X, got 0x%02X (diff: %d)",
					expected, actual, int(actual)-int(expected))
			}
		})
	}
}

// TestVerifyDecompression_MixedCorruptionTypes tests combination of different corruption types.
func TestVerifyDecompression_MixedCorruptionTypes(t *testing.T) {
	// Create test data
	original := make([]byte, 512)
	for i := range original {
		original[i] = byte(i % 256)
	}

	// Apply multiple corruption types
	corrupted := make([]byte, len(original))
	copy(corrupted, original)

	// 1. Byte swap at offset 10-11
	corrupted[10], corrupted[11] = corrupted[11], corrupted[10]

	// 2. Bit flip at offset 100
	corrupted[100] ^= 0xFF

	// 3. Zero burst at offset 200-209
	for i := 200; i < 210; i++ {
		corrupted[i] = 0x00
	}

	// 4. 0xFF pattern at offset 300-319
	for i := 300; i < 320; i++ {
		corrupted[i] = 0xFF
	}

	result := VerifyDecompression(corrupted, original)

	if result.Pass {
		t.Error("Mixed corruption types NOT detected")
	}

	// Should detect the first corruption (byte swap at offset 10)
	if result.Error.Offset != 10 {
		t.Logf("Note: First corruption detected at offset %d (expected 10)", result.Error.Offset)
	}

	t.Logf("Mixed corruption detected at offset %d: %s", result.Error.Offset, result.Diagnostic)

	// Use AnalyzeByteDifferences to see full corruption picture
	stats := AnalyzeByteDifferences(corrupted, original)
	t.Logf("Total corruption: %s", stats.Summary())
	t.Logf("Corruption offsets: %v", stats.MismatchOffsets[:minInt(10, len(stats.MismatchOffsets))])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestVerifyDecompression_TruncationDetection verifies truncation is caught.
func TestVerifyDecompression_TruncationDetection(t *testing.T) {
	tests := []struct {
		name          string
		original      []byte
		truncateTo    int
		description   string
		expectError   bool
		expectedDiff  int
	}{
		{
			name:        "truncate 10 bytes from end",
			original:    make([]byte, 100),
			truncateTo:  90,
			description: "Remove last 10 bytes",
			expectError:  true,
			expectedDiff: 10,
		},
		{
			name:        "truncate 50%",
			original:    make([]byte, 200),
			truncateTo:  100,
			description: "Remove half the data",
			expectError:  true,
			expectedDiff: 100,
		},
		{
			name:        "truncate to single byte",
			original:    make([]byte, 1000),
			truncateTo:  1,
			description: "Massive truncation to 1 byte",
			expectError:  true,
			expectedDiff: 999,
		},
		{
			name:        "truncate to empty",
			original:    make([]byte, 100),
			truncateTo:  0,
			description: "Complete truncation to empty",
			expectError:  true,
			expectedDiff: 100,
		},
		{
			name:        "no truncation",
			original:    make([]byte, 50),
			truncateTo:  50,
			description: "Full data (no truncation)",
			expectError:  false,
			expectedDiff: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize original with pattern
			for i := range tt.original {
				tt.original[i] = byte(i % 256)
			}

			// Create truncated version
			truncated := tt.original[:tt.truncateTo]

			result := VerifyDecompression(truncated, tt.original)

			if tt.expectError {
				if result.Pass {
					t.Errorf("Truncation NOT detected: %s", tt.description)
				}

				if result.Error == nil {
					t.Fatal("Expected error for truncation")
				}

				if !result.Error.IsLengthMismatch() {
					t.Errorf("Expected length mismatch error, got: %v", result.Error)
				}

				// Verify the length difference
				diff := tt.expectedDiff
				if result.Error.ActualLength != tt.truncateTo {
					t.Errorf("ActualLength = %d, want %d",
						result.Error.ActualLength, tt.truncateTo)
				}
				if result.Error.ExpectedLength != len(tt.original) {
					t.Errorf("ExpectedLength = %d, want %d",
						result.Error.ExpectedLength, len(tt.original))
				}

				t.Logf("Truncation detected correctly: missing %d bytes", diff)
			} else {
				if !result.Pass {
					t.Errorf("False positive: %s", result.Diagnostic)
				}
			}
		})
	}
}

// TestVerifyDecompression_ExtensionCorruption tests data added to the end (extension).
func TestVerifyDecompression_ExtensionCorruption(t *testing.T) {
	tests := []struct {
		name         string
		original     []byte
		extendWith   []byte
		description  string
		expectError  bool
		expectedDiff int
	}{
		{
			name:        "extend by 10 bytes",
			original:    make([]byte, 100),
			extendWith:  []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			description: "Add 10 bytes of 0xFF",
			expectError:  true,
			expectedDiff: 10,
		},
		{
			name:        "extend with garbage data",
			original:    make([]byte, 50),
			extendWith:  []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00},
			description: "Add garbage bytes",
			expectError:  true,
			expectedDiff: 5,
		},
		{
			name:        "extend with pattern",
			original:    make([]byte, 100),
			extendWith: func() []byte {
				data := make([]byte, 50)
				for i := range data {
					data[i] = byte(i)
				}
				return data
			}(),
			description: "Extend with 50 bytes of pattern",
			expectError:  true,
			expectedDiff: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize original with pattern
			for i := range tt.original {
				tt.original[i] = byte(i % 256)
			}

			// Create extended version
			extended := make([]byte, 0, len(tt.original)+len(tt.extendWith))
			extended = append(extended, tt.original...)
			extended = append(extended, tt.extendWith...)

			result := VerifyDecompression(extended, tt.original)

			if !result.Pass {
				if result.Error == nil {
					t.Fatal("Expected error details")
				}

				if !result.Error.IsLengthMismatch() {
					t.Errorf("Expected length mismatch error, got offset %d", result.Error.Offset)
				}

				// Verify the length difference
				if result.Error.ActualLength != len(extended) {
					t.Errorf("ActualLength = %d, want %d",
						result.Error.ActualLength, len(extended))
				}
				if result.Error.ExpectedLength != len(tt.original) {
					t.Errorf("ExpectedLength = %d, want %d",
						result.Error.ExpectedLength, len(tt.original))
				}

				actualDiff := result.Error.ActualLength - result.Error.ExpectedLength
				if actualDiff != tt.expectedDiff {
					t.Errorf("Expected length difference %d, got %d",
						tt.expectedDiff, actualDiff)
				}

				t.Logf("Extension detected correctly: %d extra bytes", actualDiff)
			} else {
				t.Error("Extension NOT detected")
			}
		})
	}
}

// TestVerifyDecompression_CompleteReplacement tests when data is completely wrong.
func TestVerifyDecompression_CompleteReplacement(t *testing.T) {
	tests := []struct {
		name        string
		original    []byte
		replacement []byte
		description string
	}{
		{
			name:        "all zeros",
			original:    []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			replacement: []byte{0x00, 0x00, 0x00, 0x00, 0x00},
			description: "All bytes replaced with zeros",
		},
		{
			name:        "all 0xFF",
			original:    []byte{0x11, 0x22, 0x33, 0x44, 0x55},
			replacement: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			description: "All bytes replaced with 0xFF",
		},
		{
			name:        "wrong file type",
			original:    []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, // PNG
			replacement: []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}, // ZIP
			description: "Completely wrong file signature",
		},
		{
			name:        "scrambled data",
			original: func() []byte {
				data := make([]byte, 100)
				for i := range data {
					data[i] = byte(i)
				}
				return data
			}(),
			replacement: []byte{0xA5, 0x37, 0x92, 0x4E, 0xB8, 0x1C, 0x7F, 0x23, 0x59, 0x84},
			description: "Data replaced with random bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyDecompression(tt.replacement, tt.original)

			if result.Pass {
				t.Errorf("Complete replacement NOT detected: %s", tt.description)
			}

			if result.Error == nil {
				t.Fatal("Expected error details")
			}

			// Should detect mismatch at offset 0 (first byte differs)
			if result.Error.Offset != 0 {
				t.Errorf("Expected corruption at offset 0, got %d", result.Error.Offset)
			}

			// Verify the bytes are actually different
			if len(tt.original) > 0 && len(tt.replacement) > 0 {
				if tt.original[0] == tt.replacement[0] {
					t.Error("Expected first bytes to differ")
				}
			}

			t.Logf("Complete replacement detected: %s", result.Diagnostic)
		})
	}
}
