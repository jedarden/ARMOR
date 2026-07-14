package validate

import (
	"fmt"
	"strings"
	"testing"
)

/*
FormatError String Validation Tests

This test file provides comprehensive unit tests for FormatError function
specifically focusing on string error type validation behavior.

Test Coverage:
1. Valid string error types - all basic ErrorType enum values
2. Invalid string error types - fallback behavior and tracking
3. Empty/missing error types - fallback to default "error" type
4. Case sensitivity - case-insensitive matching for recognized types
5. Edge cases - whitespace, special characters, etc.

Acceptance Criteria:
- Test FormatError with valid string error types ✓
- Test FormatError with invalid string error types (fallback behavior) ✓
- Verify fallback to default error type when invalid type provided ✓
- Test case sensitivity of error type strings ✓
- All new tests pass ✓
*/

// =============================================================================
// VALID STRING ERROR TYPES
// =============================================================================

// TestFormatError_ValidStringErrorTypes tests FormatError with all valid
// basic ErrorType enum values to ensure they are recognized and formatted correctly.
func TestFormatError_ValidStringErrorTypes(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		expected  string
	}{
		// All 8 basic ErrorType enum values
		{
			name:      "required error type",
			errorType: "required",
			message:   "Field is required",
			fieldName: "email",
			expected:  "[required] email: Field is required",
		},
		{
			name:      "format error type",
			errorType: "format",
			message:   "Invalid email format",
			fieldName: "",
			expected:  "[format] Invalid email format",
		},
		{
			name:      "range error type",
			errorType: "range",
			message:   "Value out of range",
			fieldName: "age",
			expected:  "[range] age: Value out of range",
		},
		{
			name:      "length error type",
			errorType: "length",
			message:   "String too short",
			fieldName: "password",
			expected:  "[length] password: String too short",
		},
		{
			name:      "type error type",
			errorType: "type",
			message:   "Expected number, got string",
			fieldName: "count",
			expected:  "[type] count: Expected number, got string",
		},
		{
			name:      "value error type",
			errorType: "value",
			message:   "Invalid value",
			fieldName: "status",
			expected:  "[value] status: Invalid value",
		},
		{
			name:      "duplicate error type",
			errorType: "duplicate",
			message:   "Email already exists",
			fieldName: "email",
			expected:  "[duplicate] email: Email already exists",
		},
		{
			name:      "conflict error type",
			errorType: "conflict",
			message:   "Values conflict",
			fieldName: "",
			expected:  "[conflict] Values conflict",
		},
		{
			name:      "unknown error type",
			errorType: "unknown",
			message:   "Unknown error occurred",
			fieldName: "system",
			expected:  "[unknown] system: Unknown error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tracking before each test to ensure clean state
			ResetInvalidErrorTypeTracking()

			var result string
			if tt.fieldName != "" {
				result = FormatError(tt.errorType, tt.message, tt.fieldName)
			} else {
				result = FormatError(tt.errorType, tt.message)
			}

			// Verify output matches expected format
			if result != tt.expected {
				t.Errorf("FormatError(%q, %q, %q) = %q, want %q",
					tt.errorType, tt.message, tt.fieldName, result, tt.expected)
			}

			// Verify that valid error types are NOT tracked as invalid
			invalidTypes := GetInvalidErrorTypes()
			if len(invalidTypes) > 0 {
				t.Errorf("Valid error type %q was incorrectly tracked as invalid: %v",
					tt.errorType, invalidTypes)
			}
		})
	}
}

// =============================================================================
// INVALID STRING ERROR TYPES - FALLBACK BEHAVIOR
// =============================================================================

// TestFormatError_InvalidStringErrorTypes tests FormatError with invalid
// string error types to verify fallback behavior (backward compatibility).
func TestFormatError_InvalidStringErrorTypes(t *testing.T) {
	tests := []struct {
		name           string
		errorType      string
		message        string
		fieldName      string
		expectedOutput string
		shouldTrack    bool // Whether the invalid type should be tracked
	}{
		{
			name:           "custom error type",
			errorType:      "custom_validation",
			message:        "Custom validation failed",
			fieldName:      "field",
			expectedOutput: "[custom_validation] field: Custom validation failed",
			shouldTrack:    true,
		},
		{
			name:           "typo in error type (requird instead of required)",
			errorType:      "requird",
			message:        "Field is required",
			fieldName:      "email",
			expectedOutput: "[requird] email: Field is required",
			shouldTrack:    true,
		},
		{
			name:           "unknown error type",
			errorType:      "unknown_type_xyz",
			message:        "Unknown error",
			fieldName:      "",
			expectedOutput: "[unknown_type_xyz] Unknown error",
			shouldTrack:    true,
		},
		{
			name:           "HTTP-specific error type (not a basic ErrorType)",
			errorType:      "status_code",
			message:        "Status code mismatch",
			fieldName:      "response",
			expectedOutput: "[status_code] response: Status code mismatch",
			shouldTrack:    true,
		},
		{
			name:           "error_message type (not a basic ErrorType)",
			errorType:      "error_message",
			message:        "Error message pattern not found",
			fieldName:      "",
			expectedOutput: "[error_message] Error message pattern not found",
			shouldTrack:    true,
		},
		{
			name:           "random string as error type",
			errorType:      "xyz123",
			message:        "Random error type test",
			fieldName:      "test",
			expectedOutput: "[xyz123] test: Random error type test",
			shouldTrack:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tracking before each test
			ResetInvalidErrorTypeTracking()

			var result string
			if tt.fieldName != "" {
				result = FormatError(tt.errorType, tt.message, tt.fieldName)
			} else {
				result = FormatError(tt.errorType, tt.message)
			}

			// Verify output maintains backward compatibility (original string is used)
			if result != tt.expectedOutput {
				t.Errorf("FormatError(%q, %q, %q) = %q, want %q",
					tt.errorType, tt.message, tt.fieldName, result, tt.expectedOutput)
			}

			// Check if error type was tracked for debugging
			invalidTypes := GetInvalidErrorTypes()
			if tt.shouldTrack {
				if len(invalidTypes) == 0 {
					t.Errorf("Expected invalid error type %q to be tracked", tt.errorType)
				} else if count, ok := invalidTypes[tt.errorType]; !ok || count == 0 {
					t.Errorf("Expected error type %q to be tracked with count > 0, got: %v",
						tt.errorType, invalidTypes)
				}
			} else {
				if len(invalidTypes) > 0 {
					t.Errorf("Expected valid error type %q to NOT be tracked, but got: %v",
						tt.errorType, invalidTypes)
				}
			}
		})
	}
}

// =============================================================================
// FALLBACK TO DEFAULT ERROR TYPE
// =============================================================================

// TestFormatError_FallbackToDefaultErrorType tests that FormatError falls
// back to the default "error" type when an empty or invalid type is provided.
func TestFormatError_FallbackToDefaultErrorType(t *testing.T) {
	tests := []struct {
		name         string
		errorType    string
		message      string
		fieldName    string
		expectedType string // The error type that should appear in output
		description  string
	}{
		{
			name:         "empty error type with message",
			errorType:    "",
			message:      "Something went wrong",
			fieldName:    "",
			expectedType: "error",
			description:  "Empty error type should default to 'error'",
		},
		{
			name:         "empty error type with message and field",
			errorType:    "",
			message:      "Field is required",
			fieldName:    "email",
			expectedType: "error",
			description:  "Empty error type with field should default to 'error'",
		},
		{
			name:         "whitespace-only error type",
			errorType:    "   ",
			message:      "Test message",
			fieldName:    "",
			expectedType: "error",
			description:  "Whitespace-only error type should be trimmed to empty, then default to 'error'",
		},
		{
			name:         "error type with leading/trailing whitespace",
			errorType:    "  required  ",
			message:      "Field required",
			fieldName:    "test",
			expectedType: "required",
			description:  "Whitespace should be trimmed from error type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetInvalidErrorTypeTracking()

			var result string
			if tt.fieldName != "" {
				result = FormatError(tt.errorType, tt.message, tt.fieldName)
			} else {
				result = FormatError(tt.errorType, tt.message)
			}

			// Verify the expected error type appears in the output
			expectedPrefix := fmt.Sprintf("[%s] ", tt.expectedType)
			if !strings.HasPrefix(result, expectedPrefix) {
				t.Errorf("%s: Expected result to start with %q, got: %q",
					tt.description, expectedPrefix, result)
			}

			// Verify message is included
			if !strings.Contains(result, tt.message) {
				t.Errorf("Expected result to contain message %q, got: %q", tt.message, result)
			}
		})
	}
}

// TestFormatError_EmptyMessageTypeFallback tests that empty messages
// trigger appropriate fallback behavior.
func TestFormatError_EmptyMessageTypeFallback(t *testing.T) {
	tests := []struct {
		name         string
		errorType    string
		message      string
		fieldName    string
		mustContain  []string
		description  string
	}{
		{
			name:        "empty message without field",
			errorType:   "validation",
			message:     "",
			fieldName:   "",
			mustContain: []string{"[validation]", "(no message provided)"},
			description: "Empty message should use fallback message",
		},
		{
			name:        "empty message with field name",
			errorType:   "validation",
			message:     "",
			fieldName:   "email",
			mustContain: []string{"[validation]", "email:", "email validation failed"},
			description: "Empty message with field should use field-based fallback",
		},
		{
			name:        "whitespace-only message without field",
			errorType:   "format",
			message:     "   ",
			fieldName:   "",
			mustContain: []string{"[format]", "(no message provided)"},
			description: "Whitespace-only message should use fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.fieldName != "" {
				result = FormatError(tt.errorType, tt.message, tt.fieldName)
			} else {
				result = FormatError(tt.errorType, tt.message)
			}

			// Verify all required strings are present
			for _, required := range tt.mustContain {
				if !strings.Contains(result, required) {
					t.Errorf("%s: Expected result to contain %q, got: %q",
						tt.description, required, result)
				}
			}
		})
	}
}

// =============================================================================
// CASE SENSITIVITY
// =============================================================================

// TestFormatError_CaseSensitivity tests that FormatError performs
// case-insensitive matching for recognized ErrorType enum values.
func TestFormatError_CaseSensitivity(t *testing.T) {
	tests := []struct {
		name        string
		errorType   string
		shouldTrack bool // Whether this should be tracked as invalid
		description string
	}{
		// Uppercase variants of valid types
		{
			name:        "REQUIRED (uppercase)",
			errorType:   "REQUIRED",
			shouldTrack: false,
			description: "Uppercase REQUIRED should be recognized as 'required'",
		},
		{
			name:        "FORMAT (uppercase)",
			errorType:   "FORMAT",
			shouldTrack: false,
			description: "Uppercase FORMAT should be recognized as 'format'",
		},
		{
			name:        "RANGE (uppercase)",
			errorType:   "RANGE",
			shouldTrack: false,
			description: "Uppercase RANGE should be recognized as 'range'",
		},
		{
			name:        "LENGTH (uppercase)",
			errorType:   "LENGTH",
			shouldTrack: false,
			description: "Uppercase LENGTH should be recognized as 'length'",
		},
		{
			name:        "TYPE (uppercase)",
			errorType:   "TYPE",
			shouldTrack: false,
			description: "Uppercase TYPE should be recognized as 'type'",
		},
		{
			name:        "VALUE (uppercase)",
			errorType:   "VALUE",
			shouldTrack: false,
			description: "Uppercase VALUE should be recognized as 'value'",
		},
		{
			name:        "DUPLICATE (uppercase)",
			errorType:   "DUPLICATE",
			shouldTrack: false,
			description: "Uppercase DUPLICATE should be recognized as 'duplicate'",
		},
		{
			name:        "CONFLICT (uppercase)",
			errorType:   "CONFLICT",
			shouldTrack: false,
			description: "Uppercase CONFLICT should be recognized as 'conflict'",
		},
		// Mixed case variants
		{
			name:        "ReQuIrEd (mixed case)",
			errorType:   "ReQuIrEd",
			shouldTrack: false,
			description: "Mixed case ReQuIrEd should be recognized as 'required'",
		},
		{
			name:        "FoRmAt (mixed case)",
			errorType:   "FoRmAt",
			shouldTrack: false,
			description: "Mixed case FoRmAt should be recognized as 'format'",
		},
		{
			name:        "RaNgE (mixed case)",
			errorType:   "RaNgE",
			shouldTrack: false,
			description: "Mixed case RaNgE should be recognized as 'range'",
		},
		// Custom uppercase types (should be tracked as invalid)
		{
			name:        "CUSTOM_TYPE (custom uppercase)",
			errorType:   "CUSTOM_TYPE",
			shouldTrack: true,
			description: "Custom CUSTOM_TYPE should be tracked as invalid",
		},
		{
			name:        "MyCustomError (custom mixed case)",
			errorType:   "MyCustomError",
			shouldTrack: true,
			description: "Custom MyCustomError should be tracked as invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tracking before each test
			ResetInvalidErrorTypeTracking()

			// Call FormatError with the test error type
			result := FormatError(tt.errorType, "Test message", "field")

			// Verify result contains the original error type string (backward compatibility)
			if !strings.Contains(result, tt.errorType) {
				t.Errorf("Expected result to contain error type %q, got: %q",
					tt.errorType, result)
			}

			// Check if error type was tracked
			tracked := GetInvalidErrorTypes()
			isTracked := false
			for trackedType := range tracked {
				if strings.EqualFold(trackedType, tt.errorType) {
					isTracked = true
					break
				}
			}

			if tt.shouldTrack && !isTracked {
				t.Errorf("%s: Expected error type %q to be tracked as invalid",
					tt.description, tt.errorType)
			} else if !tt.shouldTrack && isTracked {
				t.Errorf("%s: Expected error type %q to NOT be tracked (should be recognized)",
					tt.description, tt.errorType)
			}
		})
	}
}

// =============================================================================
// COMPREHENSIVE INTEGRATION TESTS
// =============================================================================

// TestFormatError_ComprehensiveStringValidation combines all aspects
// of string validation in comprehensive integration tests.
func TestFormatError_ComprehensiveStringValidation(t *testing.T) {
	tests := []struct {
		name           string
		errorType      string
		message        string
		fieldName      string
		expectedFormat string // Pattern to match in output
		shouldTrack    bool
	}{
		{
			name:           "valid lowercase type with field",
			errorType:      "required",
			message:        "Email is required",
			fieldName:      "email",
			expectedFormat: "[required] email: Email is required",
			shouldTrack:    false,
		},
		{
			name:           "valid uppercase type without field",
			errorType:      "FORMAT",
			message:        "Invalid format",
			fieldName:      "",
			expectedFormat: "[FORMAT] Invalid format",
			shouldTrack:    false,
		},
		{
			name:           "valid mixed case type with field",
			errorType:      "RaNgE",
			message:        "Value out of range",
			fieldName:      "age",
			expectedFormat: "[RaNgE] age: Value out of range",
			shouldTrack:    false,
		},
		{
			name:           "empty error type defaults to 'error'",
			errorType:      "",
			message:        "Something failed",
			fieldName:      "system",
			expectedFormat: "[error] system: Something failed",
			shouldTrack:    false, // Empty types are not tracked
		},
		{
			name:           "invalid type is still used in output (backward compat)",
			errorType:      "custom_validation",
			message:        "Custom check failed",
			fieldName:      "field",
			expectedFormat: "[custom_validation] field: Custom check failed",
			shouldTrack:    true,
		},
		{
			name:           "empty message with field uses field-based fallback",
			errorType:      "format",
			message:        "",
			fieldName:      "username",
			expectedFormat: "[format] username: username validation failed",
			shouldTrack:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetInvalidErrorTypeTracking()

			var result string
			if tt.fieldName != "" {
				result = FormatError(tt.errorType, tt.message, tt.fieldName)
			} else {
				result = FormatError(tt.errorType, tt.message)
			}

			// Verify output format
			if result != tt.expectedFormat {
				t.Errorf("FormatError() = %q, want %q", result, tt.expectedFormat)
			}

			// Verify tracking behavior
			tracked := GetInvalidErrorTypes()
			isTracked := len(tracked) > 0

			if tt.shouldTrack && !isTracked {
				t.Errorf("Expected error type %q to be tracked as invalid", tt.errorType)
			} else if !tt.shouldTrack && isTracked {
				t.Errorf("Expected error type %q to NOT be tracked, got: %v",
					tt.errorType, tracked)
			}
		})
	}
}

// =============================================================================
// EDGE CASES
// =============================================================================

// TestFormatError_EdgeCases tests edge cases and boundary conditions
// for FormatError string validation.
func TestFormatError_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		errorType   string
		message     string
		fieldName   string
		description string
		validate    func(t *testing.T, result string)
	}{
		{
			name:        "all empty inputs",
			errorType:   "",
			message:     "",
			fieldName:   "",
			description: "All empty should produce default format",
			validate: func(t *testing.T, result string) {
				expected := "[error] (no message provided)"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:        "special characters in message",
			errorType:   "format",
			message:     "Error: value \"test\" has\nnewlines and\ttabs",
			fieldName:   "field",
			description: "Special characters should be preserved",
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "Error: value \"test\" has") {
					t.Errorf("Special characters not preserved in: %q", result)
				}
			},
		},
		{
			name:        "unicode characters in message",
			errorType:   "value",
			message:     "Error: ✓ ✗ → unicode",
			fieldName:   "status",
			description: "Unicode characters should be preserved",
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "✓ ✗ →") {
					t.Errorf("Unicode characters not preserved in: %q", result)
				}
			},
		},
		{
			name:        "very long error type string",
			errorType:   "very_long_custom_error_type_name_that_exceeds_normal_length",
			message:     "Test",
			fieldName:   "",
			description: "Long error type strings should be preserved",
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "very_long_custom_error_type_name_that_exceeds_normal_length") {
					t.Errorf("Long error type not preserved in: %q", result)
				}
			},
		},
		{
			name:        "error type with numbers",
			errorType:   "error_type_123",
			message:     "Test message",
			fieldName:   "field",
			description: "Error types with numbers should be preserved",
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "error_type_123") {
					t.Errorf("Error type with numbers not preserved in: %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetInvalidErrorTypeTracking()

			var result string
			if tt.fieldName != "" {
				result = FormatError(tt.errorType, tt.message, tt.fieldName)
			} else {
				result = FormatError(tt.errorType, tt.message)
			}

			tt.validate(t, result)
		})
	}
}
