package crypto

import (
	"testing"
)

// TestVerifyDecompression_Success tests successful verification cases
func TestVerifyDecompression_Success(t *testing.T) {
	tests := []struct {
		name         string
		decompressed []byte
		expected     []byte
	}{
		{
			name:         "identical small data",
			decompressed: []byte("Hello, ARMOR!"),
			expected:     []byte("Hello, ARMOR!"),
		},
		{
			name:         "identical empty data",
			decompressed: []byte{},
			expected:     []byte{},
		},
		{
			name:         "identical larger data",
			decompressed: []byte("The quick brown fox jumps over the lazy dog. ARMOR encryption test data."),
			expected:     []byte("The quick brown fox jumps over the lazy dog. ARMOR encryption test data."),
		},
		{
			name:         "both nil",
			decompressed: nil,
			expected:     nil,
		},
		{
			name:         "binary data",
			decompressed: []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
			expected:     []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyDecompression(tt.decompressed, tt.expected)

			if !result.Pass {
				t.Errorf("VerifyDecompression() failed unexpectedly: %s", result.Diagnostic)
			}

			if result.Error != nil {
				t.Errorf("VerifyDecompression() returned error on success: %v", result.Error)
			}
		})
	}
}

// TestVerifyDecompression_LengthMismatch tests length mismatch detection
func TestVerifyDecompression_LengthMismatch(t *testing.T) {
	tests := []struct {
		name         string
		decompressed []byte
		expected     []byte
		expectedDiff int
	}{
		{
			name:         "decompressed too short",
			decompressed: []byte("Hello"),
			expected:     []byte("Hello, ARMOR!"),
			expectedDiff: 8,
		},
		{
			name:         "decompressed too long",
			decompressed: []byte("Hello, ARMOR! Extra data"),
			expected:     []byte("Hello, ARMOR!"),
			expectedDiff: 11,
		},
		{
			name:         "decompressed empty, expected not",
			decompressed: []byte{},
			expected:     []byte("data"),
			expectedDiff: 4,
		},
		{
			name:         "decompressed not empty, expected empty",
			decompressed: []byte("data"),
			expected:     []byte{},
			expectedDiff: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyDecompression(tt.decompressed, tt.expected)

			if result.Pass {
				t.Error("VerifyDecompression() passed unexpectedly on length mismatch")
			}

			if result.Error == nil {
				t.Fatal("VerifyDecompression() returned nil error on length mismatch")
			}

			if !result.Error.IsLengthMismatch() {
				t.Errorf("Expected length mismatch error, got: %v", result.Error)
			}

			// Verify the error contains the correct lengths
			if result.Error.ExpectedLength != len(tt.expected) {
				t.Errorf("ExpectedLength = %d, want %d", result.Error.ExpectedLength, len(tt.expected))
			}
			if result.Error.ActualLength != len(tt.decompressed) {
				t.Errorf("ActualLength = %d, want %d", result.Error.ActualLength, len(tt.decompressed))
			}
		})
	}
}

// TestVerifyDecompression_ByteMismatch tests byte mismatch detection
func TestVerifyDecompression_ByteMismatch(t *testing.T) {
	tests := []struct {
		name              string
		decompressed      []byte
		expected          []byte
		expectedOffset    int64
		expectedByte      byte
		actualByte        byte
	}{
		{
			name:              "single byte mismatch at start",
			decompressed:      []byte("Xello, ARMOR!"),
			expected:          []byte("Hello, ARMOR!"),
			expectedOffset:    0,
			expectedByte:      'H',
			actualByte:        'X',
		},
		{
			name:              "single byte mismatch in middle",
			decompressed:      []byte("Hello, XRMOR!"),
			expected:          []byte("Hello, ARMOR!"),
			expectedOffset:    7,
			expectedByte:      'A',
			actualByte:        'X',
		},
		{
			name:              "single byte mismatch at end",
			decompressed:      []byte("Hello, ARMOR?"),
			expected:          []byte("Hello, ARMOR!"),
			expectedOffset:    12, // Last character at index 12
			expectedByte:      '!',
			actualByte:        '?',
		},
		{
			name:              "null byte corruption",
			decompressed:      []byte{0x00, 0x01, 0x02, 0x00, 0x04},
			expected:          []byte{0x00, 0x01, 0x02, 0x03, 0x04},
			expectedOffset:    3,
			expectedByte:      0x03,
			actualByte:        0x00,
		},
		{
			name:              "bit flip corruption",
			decompressed:      []byte{0x55, 0x54}, // 01010101, 01010100
			expected:          []byte{0x55, 0x55}, // 01010101, 01010101
			expectedOffset:    1,
			expectedByte:      0x55,
			actualByte:        0x54,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyDecompression(tt.decompressed, tt.expected)

			if result.Pass {
				t.Error("VerifyDecompression() passed unexpectedly on byte mismatch")
			}

			if result.Error == nil {
				t.Fatal("VerifyDecompression() returned nil error on byte mismatch")
			}

			if !result.Error.IsByteMismatch() {
				t.Errorf("Expected byte mismatch error, got: %v", result.Error)
			}

			// Verify the offset
			if result.Error.Offset != tt.expectedOffset {
				t.Errorf("Offset = %d, want %d", result.Error.Offset, tt.expectedOffset)
			}

			// Verify the context contains the mismatched byte
			if len(result.Error.Expected) == 0 {
				t.Error("Expected context is empty")
			}
			if len(result.Error.Actual) == 0 {
				t.Error("Actual context is empty")
			}

			// Find the mismatched byte in the context
			// The context should include bytes before and after the mismatch
			contextContainsExpected := false
			contextContainsActual := false
			for _, b := range result.Error.Expected {
				if b == tt.expectedByte {
					contextContainsExpected = true
					break
				}
			}
			for _, b := range result.Error.Actual {
				if b == tt.actualByte {
					contextContainsActual = true
					break
				}
			}

			if !contextContainsExpected {
				t.Errorf("Expected context (length %d) does not contain expected byte 0x%02X",
					len(result.Error.Expected), tt.expectedByte)
			}
			if !contextContainsActual {
				t.Errorf("Actual context (length %d) does not contain actual byte 0x%02X",
					len(result.Error.Actual), tt.actualByte)
			}
		})
	}
}

// TestVerifyDecompression_ContextExtraction tests context extraction around mismatches
func TestVerifyDecompression_ContextExtraction(t *testing.T) {
	// Create test data with a mismatch in the middle
	testData := make([]byte, 100)
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	expected := make([]byte, len(testData))
	copy(expected, testData)

	// Introduce a single byte corruption at offset 50
	expected[50] = 0x42 // Should be 50
	testData[50] = 0xFF // Corrupted value

	result := VerifyDecompression(testData, expected)

	if result.Pass {
		t.Fatal("Expected verification to fail")
	}

	if result.Error == nil {
		t.Fatal("Expected non-nil error")
	}

	if result.Error.Offset != 50 {
		t.Errorf("Offset = %d, want 50", result.Error.Offset)
	}

	// Verify context was captured
	if result.Error.ContextBytes != 16 {
		t.Errorf("ContextBytes = %d, want 16", result.Error.ContextBytes)
	}

	// We should have context before and after (since we're in the middle)
	if result.Error.ContextBefore != 16 {
		t.Errorf("ContextBefore = %d, want 16", result.Error.ContextBefore)
	}
	if result.Error.ContextAfter != 16 { // Available: 100 - 50 - 1 = 49 bytes, we want 16
		t.Errorf("ContextAfter = %d, want 16", result.Error.ContextAfter)
	}

	// Verify context slices are correct size
	expectedContextLen := result.Error.ContextBefore + 1 + result.Error.ContextAfter
	if len(result.Error.Expected) != expectedContextLen {
		t.Errorf("Expected context length = %d, want %d", len(result.Error.Expected), expectedContextLen)
	}
	if len(result.Error.Actual) != expectedContextLen {
		t.Errorf("Actual context length = %d, want %d", len(result.Error.Actual), expectedContextLen)
	}
}

// TestVerifyDecompression_EarlyExit tests that comparison exits on first mismatch
func TestVerifyDecompression_EarlyExit(t *testing.T) {
	// Create data where only the first byte differs
	decompressed := []byte{0xFF, 0x01, 0x02, 0x03, 0x04, 0x05}
	expected := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}

	result := VerifyDecompression(decompressed, expected)

	if result.Pass {
		t.Fatal("Expected verification to fail")
	}

	// Should stop at first mismatch (offset 0)
	if result.Error.Offset != 0 {
		t.Errorf("Offset = %d, want 0 (first mismatch)", result.Error.Offset)
	}

	// Verify the diagnostic message is meaningful
	if result.Diagnostic == "" {
		t.Error("Diagnostic message is empty")
	}
}

// TestVerifyDecompression_SmallDataAtBoundaries tests context handling at data boundaries
func TestVerifyDecompression_SmallDataAtBoundaries(t *testing.T) {
	tests := []struct {
		name              string
		decompressed      []byte
		expected          []byte
		expectedOffset    int64
		expectedBefore    int
		expectedAfter     int
	}{
		{
			name:              "mismatch at very start with short data",
			decompressed:      []byte{0xFF, 0x01, 0x02},
			expected:          []byte{0x00, 0x01, 0x02},
			expectedOffset:    0,
			expectedBefore:    0, // No bytes before position 0
			expectedAfter:     2,
		},
		{
			name:              "mismatch near end with short data",
			decompressed:      []byte{0x00, 0x01, 0xFF},
			expected:          []byte{0x00, 0x01, 0x02},
			expectedOffset:    2,
			expectedBefore:    2,
			expectedAfter:     0, // No bytes after last position
		},
		{
			name:              "mismatch in tiny data",
			decompressed:      []byte{0x00, 0xFF},
			expected:          []byte{0x00, 0x01},
			expectedOffset:    1,
			expectedBefore:    1,
			expectedAfter:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyDecompression(tt.decompressed, tt.expected)

			if result.Pass {
				t.Error("Expected verification to fail")
			}

			if result.Error.Offset != tt.expectedOffset {
				t.Errorf("Offset = %d, want %d", result.Error.Offset, tt.expectedOffset)
			}

			if result.Error.ContextBefore != tt.expectedBefore {
				t.Errorf("ContextBefore = %d, want %d", result.Error.ContextBefore, tt.expectedBefore)
			}

			if result.Error.ContextAfter != tt.expectedAfter {
				t.Errorf("ContextAfter = %d, want %d", result.Error.ContextAfter, tt.expectedAfter)
			}
		})
	}
}

// TestVerifyDecompression_IntegrationWithBackend tests integration with backend GET patterns
func TestVerifyDecompression_IntegrationWithBackend(t *testing.T) {
	// Simulate getting data from backend.GET
	originalPlaintext := []byte("Original ARMOR encrypted object data")

	// Simulate the decompression pipeline
	// In real usage: ciphertext -> decrypt -> decompress -> verify
	decompressed := originalPlaintext

	// Verify against original
	result := VerifyDecompression(decompressed, originalPlaintext)

	if !result.Pass {
		t.Errorf("Integration test failed: %s", result.Diagnostic)
	}

	// Test with corrupted data (simulating decompression error)
	corrupted := make([]byte, len(originalPlaintext))
	copy(corrupted, originalPlaintext)
	corrupted[5] = 0xFF // Corrupt a byte

	result = VerifyDecompression(corrupted, originalPlaintext)

	if result.Pass {
		t.Error("Expected verification to fail on corrupted data")
	}

	if result.Error == nil {
		t.Fatal("Expected error on corrupted data")
	}

	// Verify error details
	if result.Error.Offset != 5 {
		t.Errorf("Offset = %d, want 5", result.Error.Offset)
	}
}

// TestVerifyDecompression_ErrorMessage tests that error messages are meaningful
func TestVerifyDecompression_ErrorMessage(t *testing.T) {
	tests := []struct {
		name         string
		decompressed []byte
		expected     []byte
		checkMessage func(t *testing.T, msg string)
	}{
		{
			name:         "length mismatch message",
			decompressed: []byte("short"),
			expected:     []byte("much longer expected data"),
			checkMessage: func(t *testing.T, msg string) {
				if len(msg) == 0 {
					t.Error("Error message is empty")
				}
				// Should mention "length mismatch"
				t.Logf("Length mismatch message: %s", msg)
			},
		},
		{
			name:         "byte mismatch message",
			decompressed: []byte{0x00, 0x01, 0x02},
			expected:     []byte{0x00, 0x01, 0x03},
			checkMessage: func(t *testing.T, msg string) {
				if len(msg) == 0 {
					t.Error("Error message is empty")
				}
				// Should contain offset and byte values
				t.Logf("Byte mismatch message: %s", msg)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyDecompression(tt.decompressed, tt.expected)

			if result.Pass {
				t.Error("Expected verification to fail")
			}

			tt.checkMessage(t, result.Diagnostic)
			t.Logf("Full diagnostic: %s", result.Diagnostic)

			// Test Error() method also produces meaningful output
			if result.Error != nil {
				errMsg := result.Error.Error()
				if len(errMsg) == 0 {
					t.Error("VerificationError.Error() returned empty string")
				}
				t.Logf("VerificationError.Error(): %s", errMsg)
			}
		})
	}
}

// TestVerifyDecompression_VerificationResultString tests String() method
func TestVerifyDecompression_VerificationResultString(t *testing.T) {
	tests := []struct {
		name         string
		decompressed []byte
		expected     []byte
		checkPrefix  string
	}{
		{
			name:         "pass case",
			decompressed: []byte("test"),
			expected:     []byte("test"),
			checkPrefix:  "PASS:",
		},
		{
			name:         "fail case",
			decompressed: []byte{0x00},
			expected:     []byte{0x01},
			checkPrefix:  "FAIL:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyDecompression(tt.decompressed, tt.expected)
			str := result.String()

			if len(str) == 0 {
				t.Error("String() returned empty string")
			}

			t.Logf("String() output: %s", str)
		})
	}
}

// TestVerifyDecompression_LargeData tests with larger data sets
func TestVerifyDecompression_LargeData(t *testing.T) {
	// Create 1KB of test data
	largeOriginal := make([]byte, 1024)
	for i := range largeOriginal {
		largeOriginal[i] = byte(i % 256)
	}

	// Test successful verification
	result := VerifyDecompression(largeOriginal, largeOriginal)
	if !result.Pass {
		t.Errorf("Large data verification failed: %s", result.Diagnostic)
	}

	// Test with corruption at various positions
	corruptionOffsets := []int{0, 100, 500, 1023}
	for _, offset := range corruptionOffsets {
		t.Run("corruption at offset", func(t *testing.T) {
			corrupted := make([]byte, len(largeOriginal))
			copy(corrupted, largeOriginal)
			corrupted[offset] ^= 0xFF // Flip bits

			result := VerifyDecompression(corrupted, largeOriginal)

			if result.Pass {
				t.Error("Expected verification to fail")
			}

			if result.Error.Offset != int64(offset) {
				t.Errorf("Offset = %d, want %d", result.Error.Offset, offset)
			}
		})
	}
}
