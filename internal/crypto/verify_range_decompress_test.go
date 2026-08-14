package crypto

import (
	"bytes"
	"testing"
)

// TestVerifyRangeDecompressionWithBounds_BasicFunctionality tests the basic happy path.
func TestVerifyRangeDecompressionWithBounds_BasicFunctionality(t *testing.T) {
	// Create a 1KB original object with predictable pattern
	original := make([]byte, 1024)
	for i := range original {
		original[i] = byte(i % 256)
	}

	tests := []struct {
		name         string
		rangeOffset  int64
		rangeLength  int64
		expectPass   bool
		description  string
	}{
		{
			name:        "start of object",
			rangeOffset: 0,
			rangeLength: 256,
			expectPass:  true,
			description: "Verify range at the beginning of the object",
		},
		{
			name:        "middle of object",
			rangeOffset: 512,
			rangeLength: 256,
			expectPass:  true,
			description: "Verify range in the middle of the object",
		},
		{
			name:        "end of object",
			rangeOffset: 768,
			rangeLength: 256,
			expectPass:  true,
			description: "Verify range at the end of the object",
		},
		{
			name:        "entire object",
			rangeOffset: 0,
			rangeLength: 1024,
			expectPass:  true,
			description: "Verify the entire object as a single range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract the expected range from original
			expectedRange := original[tt.rangeOffset : tt.rangeOffset+tt.rangeLength]

			// Verify
			result := VerifyRangeDecompressionWithBounds(original, expectedRange, tt.rangeOffset, tt.rangeLength)

			if result.Pass != tt.expectPass {
				t.Errorf("VerifyRangeDecompressionWithBounds() Pass = %v, want %v. Description: %s",
					result.Pass, tt.expectPass, tt.description)
			}

			if result.Pass && result.Diagnostic == "" {
				t.Error("Success case should have diagnostic message")
			}
		})
	}
}

// TestVerifyRangeDecompressionWithBounds_EmptyRange tests empty range handling.
func TestVerifyRangeDecompressionWithBounds_EmptyRange(t *testing.T) {
	original := make([]byte, 1024)
	for i := range original {
		original[i] = byte(i % 256)
	}

	t.Run("empty range with empty decompressed", func(t *testing.T) {
		result := VerifyRangeDecompressionWithBounds(original, []byte{}, 512, 0)

		if !result.Pass {
			t.Errorf("Empty range with empty decompressed should pass: %s", result.Diagnostic)
		}

		if result.Error != nil {
			t.Error("Empty range success should not have error")
		}
	})

	t.Run("empty range with non-empty decompressed", func(t *testing.T) {
		decompressed := []byte{0x01, 0x02, 0x03}
		result := VerifyRangeDecompressionWithBounds(original, decompressed, 512, 0)

		if result.Pass {
			t.Error("Empty range with non-empty decompressed should fail")
		}

		if result.Error == nil {
			t.Error("Should have error for length mismatch")
		}

		if result.Error != nil && result.Error.Offset != -2 {
			t.Errorf("Length mismatch should have offset -2, got %d", result.Error.Offset)
		}
	})
}

// TestVerifyRangeDecompressionWithBounds_SingleByteRange tests single-byte range verification.
func TestVerifyRangeDecompressionWithBounds_SingleByteRange(t *testing.T) {
	original := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}

	tests := []struct {
		name         string
		offset       int64
		decompressed []byte
		expectPass   bool
	}{
		{
			name:         "single byte at start matches",
			offset:       0,
			decompressed: []byte{0x00},
			expectPass:   true,
		},
		{
			name:         "single byte in middle matches",
			offset:       4,
			decompressed: []byte{0x44},
			expectPass:   true,
		},
		{
			name:         "single byte at end matches",
			offset:       7,
			decompressed: []byte{0x77},
			expectPass:   true,
		},
		{
			name:         "single byte mismatch",
			offset:       2,
			decompressed: []byte{0xFF}, // Wrong byte
			expectPass:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyRangeDecompressionWithBounds(original, tt.decompressed, tt.offset, 1)

			if result.Pass != tt.expectPass {
				t.Errorf("Pass = %v, want %v. Diagnostic: %s", result.Pass, tt.expectPass, result.Diagnostic)
			}

			if !tt.expectPass {
				if result.Error == nil {
					t.Error("Failure case should have error details")
				} else {
					if result.Error.Offset != 0 { // Relative offset should be 0
						t.Errorf("Relative offset should be 0 for single-byte range, got %d", result.Error.Offset)
					}
					// Check that we got expected and actual bytes
					if len(result.Error.Expected) == 0 {
						t.Error("Error should include expected byte")
					}
					if len(result.Error.Actual) == 0 {
						t.Error("Error should include actual byte")
					}
				}
			}
		})
	}
}

// TestVerifyRangeDecompressionWithBounds_RangeAtEndOfObject tests ranges at object boundaries.
func TestVerifyRangeDecompressionWithBounds_RangeAtEndOfObject(t *testing.T) {
	// Create a smaller object for boundary testing
	original := make([]byte, 100)
	for i := range original {
		original[i] = byte(i)
	}

	tests := []struct {
		name         string
		offset       int64
		length       int64
		decompressed  []byte
		expectPass   bool
		description  string
	}{
		{
			name:        "last 10 bytes",
			offset:      90,
			length:      10,
			decompressed: func() []byte {
				result := make([]byte, 10)
				for i := range result {
					result[i] = byte(90 + i)
				}
				return result
			}(),
			expectPass:  true,
			description: "Range ending exactly at object boundary",
		},
		{
			name:        "last byte",
			offset:      99,
			length:      1,
			decompressed: []byte{99},
			expectPass:  true,
			description: "Single-byte range at very end",
		},
		{
			name:        "partial at end",
			offset:      95,
			length:      5,
			decompressed: func() []byte {
				result := make([]byte, 5)
				for i := range result {
					result[i] = byte(95 + i)
				}
				return result
			}(),
			expectPass:  true,
			description: "Range partially at end",
		},
		{
			name:        "last 10 bytes with corruption",
			offset:      90,
			length:      10,
			decompressed: func() []byte {
				result := make([]byte, 10)
				for i := range result {
					result[i] = byte(90 + i)
				}
				result[5] = 0xFF // Corrupt byte 5 of the range
				return result
			}(),
			expectPass:  false,
			description: "Range at end with corrupted byte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyRangeDecompressionWithBounds(original, tt.decompressed, tt.offset, tt.length)

			if result.Pass != tt.expectPass {
				t.Errorf("%s: Pass = %v, want %v. Diagnostic: %s", tt.description, result.Pass, tt.expectPass, result.Diagnostic)
			}

			// For ranges at end, verify context window is clipped correctly
			if !tt.expectPass && result.Error != nil {
				// ContextAfter should be smaller when near end of range
				if result.Error.ContextAfter < 0 || result.Error.ContextAfter > int(tt.length) {
					t.Errorf("Invalid ContextAfter %d for range length %d", result.Error.ContextAfter, tt.length)
				}
			}
		})
	}
}

// TestVerifyRangeDecompressionWithBounds_RangeAtStartOfObject tests ranges at object start.
func TestVerifyRangeDecompressionWithBounds_RangeAtStartOfObject(t *testing.T) {
	original := make([]byte, 100)
	for i := range original {
		original[i] = byte(i)
	}

	tests := []struct {
		name         string
		offset       int64
		length       int64
		corruptByte  int // index within range to corrupt (0-based)
		expectPass   bool
	}{
		{
			name:        "first 10 bytes",
			offset:      0,
			length:      10,
			corruptByte: -1, // no corruption
			expectPass:  true,
		},
		{
			name:        "first byte corrupted",
			offset:      0,
			length:      10,
			corruptByte: 0, // corrupt first byte
			expectPass:  false,
		},
		{
			name:        "middle byte corrupted",
			offset:      0,
			length:      10,
			corruptByte: 5, // corrupt middle byte
			expectPass:  false,
		},
		{
			name:        "last byte of range corrupted",
			offset:      0,
			length:      10,
			corruptByte: 9, // corrupt last byte of range
			expectPass:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the range
			decompressed := make([]byte, tt.length)
			copy(decompressed, original[tt.offset:tt.offset+tt.length])

			// Corrupt if needed
			if tt.corruptByte >= 0 {
				decompressed[tt.corruptByte] = 0xFF
			}

			result := VerifyRangeDecompressionWithBounds(original, decompressed, tt.offset, tt.length)

			if result.Pass != tt.expectPass {
				t.Errorf("Pass = %v, want %v. Diagnostic: %s", result.Pass, tt.expectPass, result.Diagnostic)
			}

			// For ranges at start, verify context window is clipped correctly
			if !tt.expectPass && result.Error != nil {
				// ContextBefore should be smaller when near start of range (offset 0)
				if result.Error.ContextBefore < 0 || result.Error.ContextBefore > int(tt.length) {
					t.Errorf("Invalid ContextBefore %d for range at offset 0", result.Error.ContextBefore)
				}
			}
		})
	}
}

// TestVerifyRangeDecompressionWithBounds_LengthMismatch tests length mismatch detection.
func TestVerifyRangeDecompressionWithBounds_LengthMismatch(t *testing.T) {
	original := make([]byte, 1024)

	tests := []struct {
		name          string
		offset        int64
		length        int64
		decompressed  []byte
		expectError   bool
		expectedCode  int64 // Expected Offset value in error
	}{
		{
			name:         "decompressed too short",
			offset:       100,
			length:       256,
			decompressed: make([]byte, 200), // 56 bytes too short
			expectError:  true,
			expectedCode: -2, // Length mismatch code
		},
		{
			name:         "decompressed too long",
			offset:       100,
			length:       256,
			decompressed: make([]byte, 300), // 44 bytes too long
			expectError:  true,
			expectedCode: -2, // Length mismatch code
		},
		{
			name:         "decompressed empty when length non-zero",
			offset:       100,
			length:       256,
			decompressed: []byte{},
			expectError:  true,
			expectedCode: -2,
		},
		{
			name:         "correct length",
			offset:       100,
			length:       256,
			decompressed: make([]byte, 256),
			expectError:  false,
			expectedCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyRangeDecompressionWithBounds(original, tt.decompressed, tt.offset, tt.length)

			hasError := !result.Pass
			if hasError != tt.expectError {
				t.Errorf("Expect error = %v, got %v (Pass=%v). Diagnostic: %s",
					tt.expectError, hasError, result.Pass, result.Diagnostic)
			}

			if tt.expectError && result.Error != nil {
				if result.Error.Offset != tt.expectedCode {
					t.Errorf("Expected error code %d, got %d", tt.expectedCode, result.Error.Offset)
				}
			}
		})
	}
}

// TestVerifyRangeDecompressionWithBounds_InvalidRange tests invalid range parameters.
func TestVerifyRangeDecompressionWithBounds_InvalidRange(t *testing.T) {
	original := make([]byte, 100)

	tests := []struct {
		name         string
		offset       int64
		length       int64
		decompressed []byte
		expectPass   bool
		expectedCode int64 // Expected Offset value in error
	}{
		{
			name:         "negative offset",
			offset:       -1,
			length:       10,
			decompressed: make([]byte, 10),
			expectPass:   false,
			expectedCode: -3, // Invalid range code
		},
		{
			name:         "negative length",
			offset:       10,
			length:       -1,
			decompressed: make([]byte, 10),
			expectPass:   false,
			expectedCode: -3,
		},
		{
			name:         "offset exceeds object length",
			offset:       100,
			length:       10,
			decompressed: make([]byte, 10),
			expectPass:   false,
			expectedCode: -3,
		},
		{
			name:         "range exceeds object bounds",
			offset:       90,
			length:       20, // 90+20=110 > 100
			decompressed: make([]byte, 20),
			expectPass:   false,
			expectedCode: -3,
		},
		{
			name:         "range exactly at boundary",
			offset:       90,
			length:       10, // 90+10=100 == object length
			decompressed: make([]byte, 10),
			expectPass:   true, // Should pass if decompressed matches
			expectedCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyRangeDecompressionWithBounds(original, tt.decompressed, tt.offset, tt.length)

			if result.Pass != tt.expectPass {
				t.Errorf("Pass = %v, want %v. Diagnostic: %s", result.Pass, tt.expectPass, result.Diagnostic)
			}

			if !tt.expectPass && result.Error != nil {
				if result.Error.Offset != tt.expectedCode {
					t.Errorf("Expected error code %d, got %d", tt.expectedCode, result.Error.Offset)
				}
			}
		})
	}
}

// TestVerifyRangeDecompressionWithBounds_ByteMismatch tests byte-level mismatch detection.
func TestVerifyRangeDecompressionWithBounds_ByteMismatch(t *testing.T) {
	original := make([]byte, 256)
	for i := range original {
		original[i] = byte(i)
	}

	tests := []struct {
		name          string
		offset        int64
		length        int64
		corruptAt     int // offset within range to corrupt (0-based)
		corruptWith   byte
		expectPass    bool
		expectedRelOff int // Expected relative offset in error
	}{
		{
			name:        "first byte of range corrupted",
			offset:      50,
			length:      100,
			corruptAt:   0,
			corruptWith: 0xFF,
			expectPass:  false,
			expectedRelOff: 0,
		},
		{
			name:        "middle byte corrupted",
			offset:      50,
			length:      100,
			corruptAt:   50,
			corruptWith: 0xFF,
			expectPass:  false,
			expectedRelOff: 50,
		},
		{
			name:        "last byte of range corrupted",
			offset:      50,
			length:      100,
			corruptAt:   99,
			corruptWith: 0xFF,
			expectPass:  false,
			expectedRelOff: 99,
		},
		{
			name:        "no corruption",
			offset:      50,
			length:      100,
			corruptAt:   -1, // no corruption
			corruptWith: 0,
			expectPass:  true,
			expectedRelOff: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the range from original
			decompressed := make([]byte, tt.length)
			copy(decompressed, original[tt.offset:tt.offset+tt.length])

			// Corrupt if needed
			if tt.corruptAt >= 0 {
				decompressed[tt.corruptAt] = tt.corruptWith
			}

			result := VerifyRangeDecompressionWithBounds(original, decompressed, tt.offset, tt.length)

			if result.Pass != tt.expectPass {
				t.Errorf("Pass = %v, want %v. Diagnostic: %s", result.Pass, tt.expectPass, result.Diagnostic)
			}

			if !tt.expectPass {
				if result.Error == nil {
					t.Error("Failure should have error details")
				} else {
					// Check relative offset
					if int(result.Error.Offset) != tt.expectedRelOff {
						t.Errorf("Expected relative offset %d, got %d",
							tt.expectedRelOff, result.Error.Offset)
					}

					// Check we have expected and actual bytes
					if len(result.Error.Expected) == 0 {
						t.Error("Error should include expected bytes")
					}
					if len(result.Error.Actual) == 0 {
						t.Error("Error should include actual bytes")
					}

					// Check context is provided
					if result.Error.ContextBytes <= 0 {
						t.Error("Error should include context bytes")
					}
				}
			}
		})
	}
}

// TestVerifyRangeDecompressionWithBounds_ContextBytes tests context window handling.
func TestVerifyRangeDecompressionWithBounds_ContextBytes(t *testing.T) {
	// Create a larger object to test context windows
	original := make([]byte, 1000)
	for i := range original {
		original[i] = byte(i % 256)
	}

	t.Run("corruption at range start with context", func(t *testing.T) {
		offset := int64(100)
		length := int64(100)
		decompressed := make([]byte, length)
		copy(decompressed, original[offset:offset+length])
		decompressed[0] = 0xFF // Corrupt first byte of range

		result := VerifyRangeDecompressionWithBounds(original, decompressed, offset, length)

		if result.Pass {
			t.Error("Should detect corruption at range start")
		}

		if result.Error != nil {
			// ContextBefore should be 0 since corruption is at range start
			if result.Error.ContextBefore != 0 {
				t.Errorf("Expected ContextBefore=0 for corruption at range start, got %d",
					result.Error.ContextBefore)
			}

			// ContextAfter should be positive (up to contextBytes or remaining range)
			if result.Error.ContextAfter <= 0 {
				t.Errorf("Expected positive ContextAfter, got %d", result.Error.ContextAfter)
			}
		}
	})

	t.Run("corruption at range end with context", func(t *testing.T) {
		offset := int64(100)
		length := int64(100)
		decompressed := make([]byte, length)
		copy(decompressed, original[offset:offset+length])
		decompressed[length-1] = 0xFF // Corrupt last byte of range

		result := VerifyRangeDecompressionWithBounds(original, decompressed, offset, length)

		if result.Pass {
			t.Error("Should detect corruption at range end")
		}

		if result.Error != nil {
			// ContextAfter should be 0 since corruption is at range end
			if result.Error.ContextAfter != 0 {
				t.Errorf("Expected ContextAfter=0 for corruption at range end, got %d",
					result.Error.ContextAfter)
			}

			// ContextBefore should be positive (up to contextBytes or range start)
			if result.Error.ContextBefore <= 0 {
				t.Errorf("Expected positive ContextBefore, got %d", result.Error.ContextBefore)
			}
		}
	})

	t.Run("corruption in middle with full context", func(t *testing.T) {
		offset := int64(100)
		length := int64(100)
		decompressed := make([]byte, length)
		copy(decompressed, original[offset:offset+length])
		decompressed[50] = 0xFF // Corrupt middle byte

		result := VerifyRangeDecompressionWithBounds(original, decompressed, offset, length)

		if result.Pass {
			t.Error("Should detect corruption in middle")
		}

		if result.Error != nil {
			// Should have context on both sides
			if result.Error.ContextBefore <= 0 {
				t.Errorf("Expected positive ContextBefore, got %d", result.Error.ContextBefore)
			}
			if result.Error.ContextAfter <= 0 {
				t.Errorf("Expected positive ContextAfter, got %d", result.Error.ContextAfter)
			}

			// Both should be at least contextBytes (16) or up to range boundary
			if result.Error.ContextBefore > 16 {
				t.Errorf("ContextBefore %d exceeds expected 16", result.Error.ContextBefore)
			}
			if result.Error.ContextAfter > 16 {
				t.Errorf("ContextAfter %d exceeds expected 16", result.Error.ContextAfter)
			}
		}
	})
}

// TestVerifyRangeDecompressionWithBounds_DiagnosticMessages verifies diagnostic messages are informative.
func TestVerifyRangeDecompressionWithBounds_DiagnosticMessages(t *testing.T) {
	original := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}

	t.Run("success diagnostic", func(t *testing.T) {
		result := VerifyRangeDecompressionWithBounds(original, original[2:5], 2, 3)

		if !result.Pass {
			t.Error("Should pass for matching range")
		}

		if result.Diagnostic == "" {
			t.Error("Success should have diagnostic message")
		}

		// Should mention range details
		if !bytes.Contains([]byte(result.Diagnostic), []byte("range")) {
			t.Error("Diagnostic should mention 'range'")
		}
	})

	t.Run("length mismatch diagnostic", func(t *testing.T) {
		result := VerifyRangeDecompressionWithBounds(original, []byte{0x00}, 2, 3)

		if result.Pass {
			t.Error("Should fail for length mismatch")
		}

		if result.Diagnostic == "" {
			t.Error("Failure should have diagnostic message")
		}

		// Should mention length mismatch
		if !bytes.Contains([]byte(result.Diagnostic), []byte("length mismatch")) {
			t.Error("Diagnostic should mention 'length mismatch'")
		}
	})

	t.Run("byte mismatch diagnostic", func(t *testing.T) {
		wrongRange := []byte{0xFF, 0x11, 0x22}
		result := VerifyRangeDecompressionWithBounds(original, wrongRange, 2, 3)

		if result.Pass {
			t.Error("Should fail for byte mismatch")
		}

		if result.Diagnostic == "" {
			t.Error("Failure should have diagnostic message")
		}

		// Should mention mismatch
		if !bytes.Contains([]byte(result.Diagnostic), []byte("mismatch")) {
			t.Error("Diagnostic should mention 'mismatch'")
		}

		// Should mention offset
		if !bytes.Contains([]byte(result.Diagnostic), []byte("offset")) {
			t.Error("Diagnostic should mention 'offset'")
		}
	})

	t.Run("invalid range diagnostic", func(t *testing.T) {
		result := VerifyRangeDecompressionWithBounds(original, []byte{0x00}, -1, 3)

		if result.Pass {
			t.Error("Should fail for negative offset")
		}

		if result.Diagnostic == "" {
			t.Error("Failure should have diagnostic message")
		}

		// Should mention invalid/negative
		if !bytes.Contains([]byte(result.Diagnostic), []byte("invalid")) &&
		   !bytes.Contains([]byte(result.Diagnostic), []byte("negative")) {
			t.Error("Diagnostic should mention 'invalid' or 'negative'")
		}
	})
}

// TestVerifyRangeDecompressionWithBounds_NilInputs tests nil input handling.
func TestVerifyRangeDecompressionWithBounds_NilInputs(t *testing.T) {
	original := make([]byte, 100)

	t.Run("nil original with valid range", func(t *testing.T) {
		// This should handle gracefully - empty original means any non-zero range is out of bounds
		result := VerifyRangeDecompressionWithBounds(nil, []byte{0x00}, 0, 1)

		if result.Pass {
			t.Error("Should fail when original is nil and range is requested")
		}
	})

	t.Run("nil decompressed with empty range", func(t *testing.T) {
		// Empty range (length 0) with nil decompressed should pass
		result := VerifyRangeDecompressionWithBounds(original, nil, 50, 0)

		if !result.Pass {
			t.Errorf("Empty range with nil decompressed should pass: %s", result.Diagnostic)
		}
	})

	t.Run("nil decompressed with non-empty range", func(t *testing.T) {
		// Non-empty range with nil decompressed is a length mismatch
		result := VerifyRangeDecompressionWithBounds(original, nil, 50, 10)

		if result.Pass {
			t.Error("Should fail for length mismatch")
		}

		if result.Error != nil && result.Error.Offset != -2 {
			t.Errorf("Expected length mismatch code -2, got %d", result.Error.Offset)
		}
	})
}

// TestVerifyRangeDecompressionWithBounds_LargeObject tests with larger objects.
func TestVerifyRangeDecompressionWithBounds_LargeObject(t *testing.T) {
	// Create a 1MB object
	original := make([]byte, 1024*1024)
	for i := range original {
		original[i] = byte(i % 256)
	}

	// Extract a range in the middle
	offset := int64(100 * 1024) // 100KB offset
	length := int64(50 * 1024)  // 50KB length
	expectedRange := original[offset : offset+length]

	result := VerifyRangeDecompressionWithBounds(original, expectedRange, offset, length)

	if !result.Pass {
		t.Errorf("Large range verification failed: %s", result.Diagnostic)
	}

	// Corrupt a byte in the middle of the range
	corruptedRange := make([]byte, length)
	copy(corruptedRange, expectedRange)
	corruptedRange[length/2] = 0xFF

	result = VerifyRangeDecompressionWithBounds(original, corruptedRange, offset, length)

	if result.Pass {
		t.Error("Should detect corruption in large range")
	}

	if result.Error != nil {
		// Relative offset should be near middle
		relOffset := int(result.Error.Offset)
		expectedRelOffset := int(length / 2)
		if relOffset != expectedRelOffset {
			t.Errorf("Expected relative offset %d, got %d", expectedRelOffset, relOffset)
		}
	}
}

// TestVerifyRangeDecompressionWithBounds_VerificationErrorStructure verifies VerificationError fields.
func TestVerifyRangeDecompressionWithBounds_VerificationErrorStructure(t *testing.T) {
	original := make([]byte, 100)
	for i := range original {
		original[i] = byte(i)
	}

	t.Run("error structure on byte mismatch", func(t *testing.T) {
		offset := int64(20)
		length := int64(50)
		decompressed := make([]byte, length)
		copy(decompressed, original[offset:offset+length])
		decompressed[25] = 0xFF // Corrupt at relative offset 25

		result := VerifyRangeDecompressionWithBounds(original, decompressed, offset, length)

		if result.Pass {
			t.Fatal("Should fail for corrupted byte")
		}

		err := result.Error
		if err == nil {
			t.Fatal("Should have error details")
		}

		// Verify all expected fields are populated
		if err.Offset < 0 {
			t.Error("Offset should be non-negative for byte mismatch")
		}

		if len(err.Expected) == 0 {
			t.Error("Expected bytes should be populated")
		}

		if len(err.Actual) == 0 {
			t.Error("Actual bytes should be populated")
		}

		if err.ContextBytes <= 0 {
			t.Error("ContextBytes should be positive")
		}

		if err.ContextBefore < 0 || err.ContextBefore > err.ContextBytes {
			t.Errorf("ContextBefore %d invalid (ContextBytes=%d)", err.ContextBefore, err.ContextBytes)
		}

		if err.ContextAfter < 0 || err.ContextAfter > err.ContextBytes {
			t.Errorf("ContextAfter %d invalid (ContextBytes=%d)", err.ContextAfter, err.ContextBytes)
		}

		if err.ExpectedLength != int(length) {
			t.Errorf("ExpectedLength %d != range length %d", err.ExpectedLength, length)
		}

		if err.ActualLength != int(length) {
			t.Errorf("ActualLength %d != range length %d", err.ActualLength, length)
		}

		// Verify the corrupted byte is in the context
		expectedCorruptVal := byte((offset + 25) % 256)
		found := false
		for _, b := range err.Expected {
			if b == expectedCorruptVal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected byte 0x%02X not found in Expected context", expectedCorruptVal)
		}

		// Verify actual corrupted byte is in context
		found = false
		for _, b := range err.Actual {
			if b == 0xFF {
				found = true
				break
			}
		}
		if !found {
			t.Error("Corrupted byte 0xFF not found in Actual context")
		}
	})
}
