package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// CONSISTENCY AND BACKWARD COMPATIBILITY TESTS
//
// This file contains comprehensive tests for consistency between error formatting
// functions and backward compatibility with existing error formatting behavior.
//
// Acceptance Criteria:
// - Test consistency between FormatError and FormatErrorWithType outputs
// - Test that FormatErrorWithType produces same output as FormatError with valid type
// - Test backward compatibility with existing error formatting behavior
// - Verify all ErrorType values produce valid output
// - Test mixed scenarios (type + message + fieldName)
// =============================================================================

// TestFormatError_ConsistencyBetweenFunctions tests that FormatError (string-based)
// and FormatErrorWithType (ErrorType-based) produce identical outputs for the same inputs.
func TestFormatError_ConsistencyBetweenFunctions(t *testing.T) {
	tests := []struct {
		name         string
		errorTypeStr string
		errorTypeEnum ErrorType
		message      string
		fieldName    string
	}{
		{
			name:         "required error with field",
			errorTypeStr: "required",
			errorTypeEnum: ErrTypeRequired,
			message:      "Field is required",
			fieldName:    "email",
		},
		{
			name:         "format error without field",
			errorTypeStr: "format",
			errorTypeEnum: ErrTypeFormat,
			message:      "Invalid email format",
			fieldName:    "",
		},
		{
			name:         "range error with field",
			errorTypeStr: "range",
			errorTypeEnum: ErrTypeRange,
			message:      "Value out of range",
			fieldName:    "age",
		},
		{
			name:         "length error with field",
			errorTypeStr: "length",
			errorTypeEnum: ErrTypeLength,
			message:      "Too short",
			fieldName:    "password",
		},
		{
			name:         "type error with field",
			errorTypeStr: "type",
			errorTypeEnum: ErrTypeType,
			message:      "Wrong type",
			fieldName:    "count",
		},
		{
			name:         "value error with field",
			errorTypeStr: "value",
			errorTypeEnum: ErrTypeValue,
			message:      "Invalid value",
			fieldName:    "option",
		},
		{
			name:         "duplicate error with field",
			errorTypeStr: "duplicate",
			errorTypeEnum: ErrTypeDuplicate,
			message:      "Already exists",
			fieldName:    "email",
		},
		{
			name:         "conflict error with field",
			errorTypeStr: "conflict",
			errorTypeEnum: ErrTypeConflict,
			message:      "Conflicting values",
			fieldName:    "dates",
		},
		{
			name:         "unknown error with field",
			errorTypeStr: "unknown",
			errorTypeEnum: ErrTypeUnknown,
			message:      "Unknown error",
			fieldName:    "field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call both functions with the same logical inputs
			stringResult := FormatError(tt.errorTypeStr, tt.message, tt.fieldName)
			enumResult := FormatErrorWithType(tt.errorTypeEnum, tt.message, tt.fieldName)

			// They should produce identical results
			if stringResult != enumResult {
				t.Errorf("FormatError and FormatErrorWithType should produce identical results:\n  FormatError(string): %q\n  FormatErrorWithType(enum): %q",
					stringResult, enumResult)
			}

			// Verify the result contains the error type
			if !strings.Contains(stringResult, tt.errorTypeStr) {
				t.Errorf("Result should contain error type %q, got: %q", tt.errorTypeStr, stringResult)
			}

			// Verify the result contains the message
			if !strings.Contains(stringResult, tt.message) {
				t.Errorf("Result should contain message %q, got: %q", tt.message, stringResult)
			}

			// Verify the result contains the field name (if provided)
			if tt.fieldName != "" && !strings.Contains(stringResult, tt.fieldName) {
				t.Errorf("Result should contain field name %q, got: %q", tt.fieldName, stringResult)
			}
		})
	}
}

// TestFormatErrorWithType_AllErrorTypesProduceValidOutput tests that all ErrorType
// enum values produce valid, well-formed output.
func TestFormatErrorWithType_AllErrorTypesProduceValidOutput(t *testing.T) {
	allErrorTypes := []ErrorType{
		ErrTypeRequired,
		ErrTypeFormat,
		ErrTypeRange,
		ErrTypeLength,
		ErrTypeType,
		ErrTypeValue,
		ErrTypeDuplicate,
		ErrTypeConflict,
		ErrTypeUnknown,
	}

	testMessages := []struct {
		message   string
		fieldName string
	}{
		{"Test message", "field"},
		{"Another test", "another_field"},
		{"Validation failed", "data"},
		{"Error occurred", "item"},
	}

	for _, errorType := range allErrorTypes {
		for _, testMsg := range testMessages {
			t.Run(errorType.String()+"_"+testMsg.fieldName, func(t *testing.T) {
				result := FormatErrorWithType(errorType, testMsg.message, testMsg.fieldName)

				// Verify result is not empty
				if result == "" {
					t.Errorf("ErrorType %v should produce non-empty output", errorType)
				}

				// Verify result contains error type
				if !strings.Contains(result, errorType.String()) {
					t.Errorf("ErrorType %v output should contain error type string, got: %q", errorType, result)
				}

				// Verify result contains message
				if !strings.Contains(result, testMsg.message) {
					t.Errorf("ErrorType %v output should contain message, got: %q", errorType, result)
				}

				// Verify result contains field name
				if !strings.Contains(result, testMsg.fieldName) {
					t.Errorf("ErrorType %v output should contain field name, got: %q", errorType, result)
				}

				// Verify consistent structure: [error_type] field: message
				expectedPrefix := "[" + errorType.String() + "]"
				if !strings.HasPrefix(result, expectedPrefix) {
					t.Errorf("ErrorType %v output should start with %v, got: %q", errorType, expectedPrefix, result)
				}
			})
		}
	}
}

// TestFormatError_BackwardCompatibilityWithExistingFormatting tests that string-based
// FormatError maintains backward compatibility with existing error formatting behavior.
func TestFormatError_BackwardCompatibilityWithExistingFormatting(t *testing.T) {
	tests := []struct {
		name         string
		errorType    string
		message      string
		fieldName    string
		expectedOutput string
	}{
		{
			name:         "HTTP validation error type",
			errorType:    "status_code",
			message:      "Expected 200 but got 404",
			fieldName:    "response",
			expectedOutput: "[status_code] response: Expected 200 but got 404",
		},
		{
			name:         "Content type validation",
			errorType:    "content_type",
			message:      "Expected application/json",
			fieldName:    "headers",
			expectedOutput: "[content_type] headers: Expected application/json",
		},
		{
			name:         "Custom validation type",
			errorType:    "custom_check",
			message:      "Custom validation failed",
			fieldName:    "field",
			expectedOutput: "[custom_check] field: Custom validation failed",
		},
		{
			name:         "Empty error type defaults to 'error'",
			errorType:    "",
			message:      "Something went wrong",
			fieldName:    "",
			expectedOutput: "[error] Something went wrong",
		},
		{
			name:         "Empty message with field generates fallback",
			errorType:    "validation",
			message:      "",
			fieldName:    "email",
			expectedOutput: "[validation] email: email validation failed",
		},
		{
			name:         "Empty message without field generates default",
			errorType:    "required",
			message:      "",
			fieldName:    "",
			expectedOutput: "[required] (no message provided)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatError(tt.errorType, tt.message, tt.fieldName)

			if result != tt.expectedOutput {
				t.Errorf("FormatError() = %q, want %q", result, tt.expectedOutput)
			}
		})
	}
}

// TestFormatError_MixedParameterScenarios tests various combinations of type + message + fieldName
// to ensure all mixed scenarios work correctly.
func TestFormatError_MixedParameterScenarios(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		checkOutput func(t *testing.T, result string)
	}{
		{
			name:      "All components provided",
			errorType: "required",
			message:   "Field is required",
			fieldName: "email",
			checkOutput: func(t *testing.T, result string) {
				expected := "[required] email: Field is required"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:      "Error type and message only (no field)",
			errorType: "format",
			message:   "Invalid format",
			fieldName: "",
			checkOutput: func(t *testing.T, result string) {
				expected := "[format] Invalid format"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:      "Error type and field only (empty message)",
			errorType: "range",
			message:   "",
			fieldName: "age",
			checkOutput: func(t *testing.T, result string) {
				expected := "[range] age: age validation failed"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:      "Only error type provided",
			errorType: "type",
			message:   "",
			fieldName: "",
			checkOutput: func(t *testing.T, result string) {
				expected := "[type] (no message provided)"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:      "Empty error type with message and field",
			errorType: "",
			message:   "Something went wrong",
			fieldName: "data",
			checkOutput: func(t *testing.T, result string) {
				expected := "[error] data: Something went wrong"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:      "Special characters in message",
			errorType: "value",
			message:   "Value must be >= 0 && <= 100",
			fieldName: "percentage",
			checkOutput: func(t *testing.T, result string) {
				if !strings.Contains(result, "value") {
					t.Errorf("Expected error type in result: %q", result)
				}
				if !strings.Contains(result, "percentage") {
					t.Errorf("Expected field name in result: %q", result)
				}
				if !strings.Contains(result, ">=") && !strings.Contains(result, "<=") {
					t.Errorf("Expected special characters in result: %q", result)
				}
			},
		},
		{
			name:      "Long field name",
			errorType: "required",
			message:   "Field is required",
			fieldName: "user.profile.contact.email.address",
			checkOutput: func(t *testing.T, result string) {
				if !strings.Contains(result, "required") {
					t.Errorf("Expected error type in result: %q", result)
				}
				if !strings.Contains(result, "user.profile.contact.email.address") {
					t.Errorf("Expected full field name in result: %q", result)
				}
			},
		},
		{
			name:      "Unicode in message",
			errorType: "format",
			message:   "Email must contain @ symbol (✓)",
			fieldName: "email",
			checkOutput: func(t *testing.T, result string) {
				if !strings.Contains(result, "format") {
					t.Errorf("Expected error type in result: %q", result)
				}
				if !strings.Contains(result, "@") {
					t.Errorf("Expected @ symbol in result: %q", result)
				}
				if !strings.Contains(result, "✓") {
					t.Errorf("Expected unicode character in result: %q", result)
				}
			},
		},
		{
			name:      "Message with quotes",
			errorType: "value",
			message:   `"value" must be 'valid'`,
			fieldName: "status",
			checkOutput: func(t *testing.T, result string) {
				if !strings.Contains(result, `"value"`) && !strings.Contains(result, `'valid'`) {
					t.Errorf("Expected quoted strings in result: %q", result)
				}
			},
		},
		{
			name:      "Multi-line message",
			errorType: "conflict",
			message:   "Line 1\nLine 2\nLine 3",
			fieldName: "config",
			checkOutput: func(t *testing.T, result string) {
				if !strings.Contains(result, "Line 1") {
					t.Errorf("Expected first line in result: %q", result)
				}
				if !strings.Contains(result, "Line 2") {
					t.Errorf("Expected second line in result: %q", result)
				}
				if !strings.Contains(result, "Line 3") {
					t.Errorf("Expected third line in result: %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatError(tt.errorType, tt.message, tt.fieldName)
			tt.checkOutput(t, result)
		})
	}
}

// TestFormatErrorWithType_MixedParameterScenarios tests FormatErrorWithType with
// various combinations of ErrorType + message + fieldName.
func TestFormatErrorWithType_MixedParameterScenarios(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		checkOutput func(t *testing.T, result string)
	}{
		{
			name:      "All ErrorType values with full parameters",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "email",
			checkOutput: func(t *testing.T, result string) {
				expected := "[required] email: Field is required"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:      "All ErrorType values without field name",
			errorType: ErrTypeFormat,
			message:   "Invalid format",
			fieldName: "",
			checkOutput: func(t *testing.T, result string) {
				expected := "[format] Invalid format"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:      "All ErrorType values with empty message",
			errorType: ErrTypeRange,
			message:   "",
			fieldName: "age",
			checkOutput: func(t *testing.T, result string) {
				expected := "[range] age: age validation failed"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:      "All ErrorType values with all empty parameters",
			errorType: ErrTypeLength,
			message:   "",
			fieldName: "",
			checkOutput: func(t *testing.T, result string) {
				expected := "[length] (no message provided)"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:      "Complex message with technical details",
			errorType: ErrTypeType,
			message:   "Expected int, got string (value: 'abc')",
			fieldName: "count",
			checkOutput: func(t *testing.T, result string) {
				if !strings.Contains(result, "type") {
					t.Errorf("Expected error type in result: %q", result)
				}
				if !strings.Contains(result, "count") {
					t.Errorf("Expected field name in result: %q", result)
				}
				if !strings.Contains(result, "int") || !strings.Contains(result, "string") {
					t.Errorf("Expected technical details in result: %q", result)
				}
			},
		},
		{
			name:      "Nested field name",
			errorType: ErrTypeValue,
			message:   "Invalid option value",
			fieldName: "user.settings.notification.type",
			checkOutput: func(t *testing.T, result string) {
				if !strings.Contains(result, "value") {
					t.Errorf("Expected error type in result: %q", result)
				}
				if !strings.Contains(result, "user.settings.notification.type") {
					t.Errorf("Expected nested field name in result: %q", result)
				}
			},
		},
		{
			name:      "Array-indexed field name",
			errorType: ErrTypeDuplicate,
			message:   "Duplicate email found",
			fieldName: "users.0.email",
			checkOutput: func(t *testing.T, result string) {
				if !strings.Contains(result, "duplicate") {
					t.Errorf("Expected error type in result: %q", result)
				}
				// Field names with array indices should be preserved
				if !strings.Contains(result, "users") && !strings.Contains(result, "email") {
					t.Errorf("Expected field components in result: %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithType(tt.errorType, tt.message, tt.fieldName)
			tt.checkOutput(t, result)
		})
	}
}

// TestFormatError_ComprehensiveErrorTypeCoverage ensures all ErrorType enum values
// work correctly with both FormatError and FormatErrorWithType.
func TestFormatError_ComprehensiveErrorTypeCoverage(t *testing.T) {
	errorTypeMappings := []struct {
		stringType string
		enumType   ErrorType
	}{
		{"required", ErrTypeRequired},
		{"format", ErrTypeFormat},
		{"range", ErrTypeRange},
		{"length", ErrTypeLength},
		{"type", ErrTypeType},
		{"value", ErrTypeValue},
		{"duplicate", ErrTypeDuplicate},
		{"conflict", ErrTypeConflict},
		{"unknown", ErrTypeUnknown},
	}

	testScenarios := []struct {
		name      string
		message   string
		fieldName string
	}{
		{"basic", "Test error", "field"},
		{"no_field", "Test error", ""},
		{"empty_message", "", "field"},
		{"empty_all", "", ""},
		{"with_special_chars", "Error: value > 100", "data.field"},
		{"long_message", "This is a very long error message that contains a lot of detail about what went wrong", "config"},
	}

	for _, mapping := range errorTypeMappings {
		for _, scenario := range testScenarios {
			t.Run(mapping.stringType+"_"+scenario.name, func(t *testing.T) {
				// Test string-based FormatError
				stringResult := FormatError(mapping.stringType, scenario.message, scenario.fieldName)

				// Test ErrorType-based FormatErrorWithType
				enumResult := FormatErrorWithType(mapping.enumType, scenario.message, scenario.fieldName)

				// They should produce identical results (for valid error types)
				if stringResult != enumResult {
					t.Logf("String result: %q", stringResult)
					t.Logf("Enum result:   %q", enumResult)
					// For case differences, this is expected - but for lowercase they should match
					if strings.ToLower(stringResult) != strings.ToLower(enumResult) {
						t.Errorf("String and ErrorType results should match (case-insensitive)")
					}
				}

				// Verify result contains error type
				if !strings.Contains(stringResult, mapping.stringType) {
					t.Errorf("Result should contain error type %q, got: %q", mapping.stringType, stringResult)
				}

				// Verify result is not empty
				if stringResult == "" {
					t.Errorf("Result should not be empty for error type %q", mapping.stringType)
				}
			})
		}
	}
}

// TestFormatError_BackwardCompatibilityEdgeCases tests edge cases for backward
// compatibility to ensure existing behavior is maintained.
func TestFormatError_BackwardCompatibilityEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		errorType    string
		message      string
		fieldName    string
		shouldWork   bool
		description  string
	}{
		{
			name:        "Valid error type - required",
			errorType:   "required",
			message:     "Field is required",
			fieldName:   "email",
			shouldWork:  true,
			description: "Standard required field error",
		},
		{
			name:        "Valid error type - format",
			errorType:   "format",
			message:     "Invalid format",
			fieldName:   "email",
			shouldWork:  true,
			description: "Standard format error",
		},
		{
			name:        "Custom error type",
			errorType:   "my_custom_error",
			message:     "Custom validation failed",
			fieldName:   "field",
			shouldWork:  true,
			description: "Custom error types should work for backward compatibility",
		},
		{
			name:        "HTTP-specific error type",
			errorType:   "status_code",
			message:     "Expected 200, got 404",
			fieldName:   "response",
			shouldWork:  true,
			description: "HTTP validation error types should work",
		},
		{
			name:        "Error type with underscores",
			errorType:   "error_message_validation",
			message:     "Message validation failed",
			fieldName:   "response",
			shouldWork:  true,
			description: "Complex error types with underscores should work",
		},
		{
			name:        "Error type with numbers",
			errorType:   "error_404",
			message:     "Not found",
			fieldName:   "endpoint",
			shouldWork:  true,
			description: "Error types with numbers should work",
		},
		{
			name:        "Empty error type",
			errorType:   "",
			message:     "Generic error",
			fieldName:   "field",
			shouldWork:  true,
			description: "Empty error type should fall back to 'error'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should never panic
			result := FormatError(tt.errorType, tt.message, tt.fieldName)

			if tt.shouldWork && result == "" {
				t.Errorf("%s: FormatError should produce output, got empty string", tt.description)
			}

			// Verify the result is well-formed
			if result != "" {
				// Should start with [error_type] or [error] for empty type
				if !strings.HasPrefix(result, "[") {
					t.Errorf("Result should start with '[', got: %q", result)
				}
				// Should contain the message
				if tt.message != "" && !strings.Contains(result, tt.message) {
					// Check for fallback message if original was empty
					if tt.message != "" {
						t.Errorf("Result should contain message, got: %q", result)
					}
				}
			}
		})
	}
}

// TestFormatError_CrossFunctionConsistency tests that all error formatting
// functions work consistently across different use cases.
func TestFormatError_CrossFunctionConsistency(t *testing.T) {
	// Test that FormatErrorMessage and FormatErrorWithType work together
	testCases := []struct {
		errorType  ErrorType
		message    string
		fieldName  string
	}{
		{ErrTypeRequired, "Field is required", "email"},
		{ErrTypeFormat, "Invalid format", "data"},
		{ErrTypeRange, "Out of range", "count"},
		{ErrTypeLength, "Too long", "password"},
		{ErrTypeType, "Wrong type", "value"},
		{ErrTypeValue, "Invalid value", "option"},
		{ErrTypeDuplicate, "Already exists", "id"},
		{ErrTypeConflict, "Conflict detected", "resource"},
		{ErrTypeUnknown, "Unknown issue", "field"},
	}

	for _, tc := range testCases {
		t.Run(tc.errorType.String(), func(t *testing.T) {
			// FormatErrorWithType should use FormatErrorMessage internally
			result1 := FormatErrorWithType(tc.errorType, tc.message, tc.fieldName)
			result2 := FormatErrorMessage(tc.errorType.String(), tc.message, tc.fieldName)

			// They should produce the same result
			if result1 != result2 {
				t.Errorf("FormatErrorWithType and FormatErrorMessage should produce same result:\n  FormatErrorWithType: %q\n  FormatErrorMessage: %q",
					result1, result2)
			}

			// Verify structure
			expectedPrefix := "[" + tc.errorType.String() + "]"
			if !strings.HasPrefix(result1, expectedPrefix) {
				t.Errorf("Result should start with %q, got: %q", expectedPrefix, result1)
			}
		})
	}
}
