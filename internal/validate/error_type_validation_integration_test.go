package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// FORMAT ERROR INVALID ERROR TYPE TESTS
// =============================================================================
// These tests verify that FormatError handles invalid string error types correctly
// with proper fallback behavior and tracking.

func TestFormatError_InvalidErrorTypeFallback(t *testing.T) {
	// Reset tracking before test
	ResetInvalidErrorTypeTracking()

	tests := []struct {
		name          string
		errorType     string
		message       string
		fieldName     string
		expectedInMsg string
		shouldTrack   bool // whether this should be tracked as invalid
	}{
		{
			name:          "completely unknown error type",
			errorType:     "totally_unknown_type",
			message:       "Something went wrong",
			fieldName:     "field",
			expectedInMsg: "[totally_unknown_type] field: Something went wrong",
			shouldTrack:   true,
		},
		{
			name:          "typo in error type",
			errorType:     "requred", // typo for "required"
			message:       "Field is required",
			fieldName:     "email",
			expectedInMsg: "[requred] email: Field is required",
			shouldTrack:   true,
		},
		{
			name:          "mixed case error type (case-insensitive match, original preserved)",
			errorType:     "ReQuIrEd",
			message:       "Field is required",
			fieldName:     "name",
			expectedInMsg: "[ReQuIrEd] name: Field is required",
			shouldTrack:   false, // Case-insensitive match to ErrTypeRequired, so not tracked
		},
		{
			name:          "special characters in error type",
			errorType:     "error-type!",
			message:       "Error message",
			fieldName:     "",
			expectedInMsg: "[error-type!] Error message",
			shouldTrack:   true,
		},
		{
			name:          "numeric error type",
			errorType:     "error_123",
			message:       "Numeric type error",
			fieldName:     "count",
			expectedInMsg: "[error_123] count: Numeric type error",
			shouldTrack:   true,
		},
		{
			name:          "valid error type should not be tracked",
			errorType:     "required",
			message:       "Field is required",
			fieldName:     "email",
			expectedInMsg: "[required] email: Field is required",
			shouldTrack:   false,
		},
		{
			name:          "valid format type should not be tracked",
			errorType:     "format",
			message:       "Invalid format",
			fieldName:     "email",
			expectedInMsg: "[format] email: Invalid format",
			shouldTrack:   false,
		},
		{
			name:          "valid range type should not be tracked",
			errorType:     "range",
			message:       "Out of range",
			fieldName:     "age",
			expectedInMsg: "[range] age: Out of range",
			shouldTrack:   false,
		},
		{
			name:          "unknown type is valid and should not be tracked",
			errorType:     "unknown",
			message:       "Unknown error",
			fieldName:     "",
			expectedInMsg: "[unknown] Unknown error",
			shouldTrack:   false,
		},
		{
			name:          "HTTP validation type - not in ErrorType enum",
			errorType:     "status_code",
			message:       "Wrong status code",
			fieldName:     "response",
			expectedInMsg: "[status_code] response: Wrong status code",
			shouldTrack:   true, // status_code is ValidationErrorType, not ErrorType
		},
		{
			name:          "content type validation - not in ErrorType enum",
			errorType:     "content_type",
			message:       "Wrong content type",
			fieldName:     "response",
			expectedInMsg: "[content_type] response: Wrong content type",
			shouldTrack:   true, // content_type is ValidationErrorType, not ErrorType
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear tracked types before each test
			ResetInvalidErrorTypeTracking()

			result := FormatError(tt.errorType, tt.message, tt.fieldName)

			// Verify the output contains the error type string
			if result != tt.expectedInMsg {
				t.Errorf("FormatError() = %v, want %v", result, tt.expectedInMsg)
			}

			// Check if the error type was tracked as invalid
			tracked := GetInvalidErrorTypes()
			_, wasTracked := tracked[tt.errorType]

			if tt.shouldTrack && !wasTracked {
				t.Errorf("Expected error type %q to be tracked as invalid, but it wasn't. Tracked: %v",
					tt.errorType, tracked)
			}

			if !tt.shouldTrack && wasTracked {
				t.Errorf("Expected error type %q NOT to be tracked as invalid, but it was. Tracked: %v",
					tt.errorType, tracked)
			}
		})
	}
}

func TestFormatError_WhitespaceOnlyMessage(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "only spaces in message with field",
			errorType: "required",
			message:   "     ",
			fieldName: "email",
			want:      "[required] email: email validation failed",
		},
		{
			name:      "only tabs in message with field",
			errorType: "format",
			message:   "\t\t\t",
			fieldName: "password",
			want:      "[format] password: password validation failed",
		},
		{
			name:      "mixed whitespace in message with field",
			errorType: "range",
			message:   "  \t  \n  ",
			fieldName: "age",
			want:      "[range] age: age validation failed",
		},
		{
			name:      "only spaces in message without field",
			errorType: "required",
			message:   "     ",
			fieldName: "",
			want:      "[required] (no message provided)",
		},
		{
			name:      "message with whitespace around valid text",
			errorType: "format",
			message:   "  Valid message  ",
			fieldName: "email",
			want:      "[format] email: Valid message",
		},
		{
			name:      "message with leading whitespace",
			errorType: "type",
			message:   "   Wrong type",
			fieldName: "count",
			want:      "[type] count: Wrong type",
		},
		{
			name:      "message with trailing whitespace",
			errorType: "value",
			message:   "Invalid value   ",
			fieldName: "option",
			want:      "[value] option: Invalid value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatError(tt.errorType, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatErrorWithType_WhitespaceOnlyMessage(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "only spaces in message with field",
			errorType: ErrTypeRequired,
			message:   "     ",
			fieldName: "email",
			want:      "[required] email: email validation failed",
		},
		{
			name:      "only tabs in message with field",
			errorType: ErrTypeFormat,
			message:   "\t\t\t",
			fieldName: "password",
			want:      "[format] password: password validation failed",
		},
		{
			name:      "mixed whitespace in message with field",
			errorType: ErrTypeRange,
			message:   "  \t  \n  ",
			fieldName: "age",
			want:      "[range] age: age validation failed",
		},
		{
			name:      "only spaces in message without field",
			errorType: ErrTypeLength,
			message:   "     ",
			fieldName: "",
			want:      "[length] (no message provided)",
		},
		{
			name:      "message with whitespace around valid text",
			errorType: ErrTypeType,
			message:   "  Valid message  ",
			fieldName: "count",
			want:      "[type] count: Valid message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatErrorWithType(tt.errorType, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatErrorWithType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatError_WhitespaceOnlyFieldName(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "only spaces in field name",
			errorType: "required",
			message:   "Field is required",
			fieldName: "     ",
			want:      "[required] Field is required",
		},
		{
			name:      "only tabs in field name",
			errorType: "format",
			message:   "Invalid format",
			fieldName: "\t\t\t",
			want:      "[format] Invalid format",
		},
		{
			name:      "mixed whitespace in field name",
			errorType: "range",
			message:   "Out of range",
			fieldName: "  \t  \n  ",
			want:      "[range] Out of range",
		},
		{
			name:      "field name with whitespace around valid name",
			errorType: "type",
			message:   "Wrong type",
			fieldName: "  email  ",
			want:      "[type] email: Wrong type",
		},
		{
			name:      "field name with leading whitespace",
			errorType: "value",
			message:   "Invalid value",
			fieldName: "   password",
			want:      "[value] password: Invalid value",
		},
		{
			name:      "field name with trailing whitespace",
			errorType: "duplicate",
			message:   "Already exists",
			fieldName: "email   ",
			want:      "[duplicate] email: Already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatError(tt.errorType, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatErrorWithType_WhitespaceOnlyFieldName(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "only spaces in field name",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "     ",
			want:      "[required] Field is required",
		},
		{
			name:      "only tabs in field name",
			errorType: ErrTypeFormat,
			message:   "Invalid format",
			fieldName: "\t\t\t",
			want:      "[format] Invalid format",
		},
		{
			name:      "mixed whitespace in field name",
			errorType: ErrTypeRange,
			message:   "Out of range",
			fieldName: "  \t  \n  ",
			want:      "[range] Out of range",
		},
		{
			name:      "field name with whitespace around valid name",
			errorType: ErrTypeType,
			message:   "Wrong type",
			fieldName: "  email  ",
			want:      "[type] email: Wrong type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatErrorWithType(tt.errorType, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatErrorWithType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatError_EmptyErrorTypeEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "empty error type with message and field",
			errorType: "",
			message:   "Something went wrong",
			fieldName: "field",
			want:      "[error] field: Something went wrong",
		},
		{
			name:      "empty error type with message only",
			errorType: "",
			message:   "Generic error",
			fieldName: "",
			want:      "[error] Generic error",
		},
		{
			name:      "empty error type with empty message",
			errorType: "",
			message:   "",
			fieldName: "field",
			want:      "[error] field: field validation failed",
		},
		{
			name:      "empty error type with empty message and empty field",
			errorType: "",
			message:   "",
			fieldName: "",
			want:      "[error] (no message provided)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatError(tt.errorType, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatError_MultipleInvalidTypesTracking(t *testing.T) {
	// Reset tracking before test
	ResetInvalidErrorTypeTracking()

	// Track multiple invalid error types
	invalidTypes := []string{
		"invalid_type_1",
		"invalid_type_2",
		"invalid_type_3",
		"typo_required",
		"format_typo",
	}

	for _, errType := range invalidTypes {
		FormatError(errType, "Test message", "field")
	}

	// Verify all invalid types were tracked
	tracked := GetInvalidErrorTypes()

	if len(tracked) != len(invalidTypes) {
		t.Errorf("Expected %d invalid types to be tracked, got %d. Tracked: %v",
			len(invalidTypes), len(tracked), tracked)
	}

	for _, errType := range invalidTypes {
		if count, ok := tracked[errType]; !ok {
			t.Errorf("Expected invalid type %q to be tracked, but it wasn't. Tracked: %v",
				errType, tracked)
		} else if count != 1 {
			t.Errorf("Expected invalid type %q to have count 1, got %d", errType, count)
		}
	}
}

func TestFormatError_DuplicateInvalidTypeTracking(t *testing.T) {
	// Reset tracking before test
	ResetInvalidErrorTypeTracking()

	invalidType := "repeated_invalid_type"
	occurrences := 5

	// Use the same invalid type multiple times
	for i := 0; i < occurrences; i++ {
		FormatError(invalidType, "Test message", "field")
	}

	// Verify the type was tracked with correct count
	tracked := GetInvalidErrorTypes()

	count, ok := tracked[invalidType]
	if !ok {
		t.Errorf("Expected invalid type %q to be tracked, but it wasn't", invalidType)
	} else if count != occurrences {
		t.Errorf("Expected invalid type %q to have count %d, got %d", invalidType, occurrences, count)
	}
}

func TestFormatError_ValidAndInvalidMixed(t *testing.T) {
	// Reset tracking before test
	ResetInvalidErrorTypeTracking()

	// Mix of valid and invalid error types
	errorCalls := []struct {
		errorType string
		valid     bool
	}{
		{"required", true},
		{"invalid_1", false},
		{"format", true},
		{"invalid_2", false},
		{"range", true},
		{"invalid_1", false}, // Duplicate invalid
		{"type", true},
		{"invalid_3", false},
	}

	for _, call := range errorCalls {
		FormatError(call.errorType, "Test message", "field")
	}

	// Verify only invalid types were tracked
	tracked := GetInvalidErrorTypes()

	expectedInvalidTypes := map[string]int{
		"invalid_1": 2, // Called twice
		"invalid_2": 1,
		"invalid_3": 1,
	}

	if len(tracked) != len(expectedInvalidTypes) {
		t.Errorf("Expected %d invalid types to be tracked, got %d. Tracked: %v",
			len(expectedInvalidTypes), len(tracked), tracked)
	}

	for errType, expectedCount := range expectedInvalidTypes {
		count, ok := tracked[errType]
		if !ok {
			t.Errorf("Expected invalid type %q to be tracked", errType)
		} else if count != expectedCount {
			t.Errorf("Expected invalid type %q to have count %d, got %d",
				errType, expectedCount, count)
		}
	}

	// Verify valid types were not tracked
	if _, ok := tracked["required"]; ok {
		t.Error("Valid type 'required' should not be tracked as invalid")
	}
	if _, ok := tracked["format"]; ok {
		t.Error("Valid type 'format' should not be tracked as invalid")
	}
	if _, ok := tracked["range"]; ok {
		t.Error("Valid type 'range' should not be tracked as invalid")
	}
	if _, ok := tracked["type"]; ok {
		t.Error("Valid type 'type' should not be tracked as invalid")
	}
}

func TestFormatError_SpecialCharactersInAllComponents(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "quotes in all components",
			errorType: "type'with'quotes",
			message:   `"Message" with 'quotes'`,
			fieldName: `'field'`,
			want:      "[type'with'quotes] 'field': \"Message\" with 'quotes'",
		},
		{
			name:      "unicode in all components",
			errorType: "错误类型",
			message:   "Message with emoji ✓ ✗ →",
			fieldName: "字段",
			want:      "[错误类型] 字段: Message with emoji ✓ ✗ →",
		},
		{
			name:      "newlines in message",
			errorType: "multi_line",
			message:   "Line 1\nLine 2\nLine 3",
			fieldName: "field",
			want:      "[multi_line] field: Line 1\nLine 2\nLine 3",
		},
		{
			name:      "special symbols",
			errorType: "type@#$",
			message:   "Error: <tag>&nbsp;value</tag>",
			fieldName: "field-name_123",
			want:      "[type@#$] field-name_123: Error: <tag>&nbsp;value</tag>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatError(tt.errorType, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatErrorWithType_SpecialCharactersInAllComponents(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "quotes in message and field",
			errorType: ErrTypeFormat,
			message:   `"Message" with 'quotes'`,
			fieldName: `'field'`,
			want:      "[format] 'field': \"Message\" with 'quotes'",
		},
		{
			name:      "unicode in message and field",
			errorType: ErrTypeRequired,
			message:   "Message with emoji ✓ ✗ →",
			fieldName: "字段",
			want:      "[required] 字段: Message with emoji ✓ ✗ →",
		},
		{
			name:      "newlines in message",
			errorType: ErrTypeValue,
			message:   "Line 1\nLine 2\nLine 3",
			fieldName: "field",
			want:      "[value] field: Line 1\nLine 2\nLine 3",
		},
		{
			name:      "special symbols",
			errorType: ErrTypeType,
			message:   "Error: <tag>&nbsp;value</tag>",
			fieldName: "field-name_123",
			want:      "[type] field-name_123: Error: <tag>&nbsp;value</tag>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatErrorWithType(tt.errorType, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatErrorWithType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatError_ConsistencyBetweenAllErrorTypes(t *testing.T) {
	// Test that all ErrorType enum values produce consistent output
	// whether using FormatError or FormatErrorWithType
	allTypes := []struct {
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

	message := "Test error message"
	fieldName := "test_field"

	for _, tt := range allTypes {
		t.Run(tt.stringType, func(t *testing.T) {
			// Test with message and field
			resultStr := FormatError(tt.stringType, message, fieldName)
			resultEnum := FormatErrorWithType(tt.enumType, message, fieldName)

			if resultStr != resultEnum {
				t.Errorf("Format mismatch for %s:\n  String: %v\n  Enum:   %v",
					tt.stringType, resultStr, resultEnum)
			}

			// Test without field
			resultStrNoField := FormatError(tt.stringType, message, "")
			resultEnumNoField := FormatErrorWithType(tt.enumType, message, "")

			if resultStrNoField != resultEnumNoField {
				t.Errorf("Format mismatch (no field) for %s:\n  String: %v\n  Enum:   %v",
					tt.stringType, resultStrNoField, resultEnumNoField)
			}

			// Verify structure is correct
			expectedPrefix := "[" + tt.stringType + "]"
			if !strings.HasPrefix(resultStr, expectedPrefix) {
				t.Errorf("Result should start with %s, got: %s", expectedPrefix, resultStr)
			}

			// Verify message is included
			if !strings.Contains(resultStr, message) {
				t.Errorf("Result should contain message, got: %s", resultStr)
			}

			// Verify field is included when provided
			if fieldName != "" && !strings.Contains(resultStr, fieldName+":") {
				t.Errorf("Result should contain field name, got: %s", resultStr)
			}
		})
	}
}

func TestFormatError_AllErrorTypesProduceValidOutput(t *testing.T) {
	// Test that all ErrorType enum values produce valid, non-empty output
	allTypes := []ErrorType{
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

	message := "Test message"
	fieldName := "testField"

	for _, et := range allTypes {
		t.Run(et.String(), func(t *testing.T) {
			result := FormatErrorWithType(et, message, fieldName)

			// Verify output is not empty
			if result == "" {
				t.Error("FormatErrorWithType should not return empty string")
			}

			// Verify output contains error type
			if !strings.Contains(result, et.String()) {
				t.Errorf("Result should contain error type %s, got: %s", et.String(), result)
			}

			// Verify output contains message
			if !strings.Contains(result, message) {
				t.Errorf("Result should contain message, got: %s", result)
			}

			// Verify output contains field name
			if !strings.Contains(result, fieldName) {
				t.Errorf("Result should contain field name, got: %s", result)
			}

			// Verify consistent structure: [type] field: message
			expectedPattern := "[" + et.String() + "] " + fieldName + ": " + message
			if result != expectedPattern {
				t.Errorf("Result format mismatch:\n  Expected: %s\n  Got:      %s",
					expectedPattern, result)
			}
		})
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestFormatError_LongStrings(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
	}{
		{
			name:      "very long error type",
			errorType: strings.Repeat("very_long_type_", 100),
			message:   "Error message",
			fieldName: "field",
		},
		{
			name:      "very long message",
			errorType: "type",
			message:   strings.Repeat("This is a very long error message. ", 100),
			fieldName: "field",
		},
		{
			name:      "very long field name",
			errorType: "type",
			message:   "Error message",
			fieldName: strings.Repeat("very_long_field_name_", 100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic and should produce valid output
			result := FormatError(tt.errorType, tt.message, tt.fieldName)

			if result == "" {
				t.Error("FormatError should not return empty string for long inputs")
			}

			// Verify error type prefix is present
			if !strings.HasPrefix(result, "["+tt.errorType+"]") {
				t.Errorf("Result should start with error type in brackets, got prefix: %s",
					result[:min(50, len(result))])
			}

			// Verify field name is present (if provided)
			if tt.fieldName != "" && !strings.Contains(result, tt.fieldName) {
				t.Error("Result should contain field name")
			}

			// For very long messages, just verify start of message is present
			// (the full message will be present, but we check a reasonable prefix)
			if len(tt.message) > 100 {
				messagePrefix := tt.message[:50]
				if !strings.Contains(result, messagePrefix) {
					t.Errorf("Result should contain message prefix (first 50 chars)")
				}
			} else {
				if !strings.Contains(result, tt.message) {
					t.Error("Result should contain message")
				}
			}
		})
	}
}

// Helper function for safe string slicing
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestFormatError_UnicodeNormalizations(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
	}{
		{
			name:      "combining diacritics",
			errorType: "type",
			message:   "Error with café",
			fieldName: "fiëld",
		},
		{
			name:      "emoji modifiers",
			errorType: "error_🎉",
			message:   "Message with 👨‍👩‍👧‍👦 family emoji",
			fieldName: "📁field",
		},
		{
			name:      "right-to-left text",
			errorType: "type",
			message:   "مرحبا بالعربية", // Arabic
			fieldName: "שדה",          // Hebrew
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatError(tt.errorType, tt.message, tt.fieldName)

			if result == "" {
				t.Error("FormatError should handle unicode correctly")
			}

			// Verify components are present (unicode should be preserved)
			if !strings.Contains(result, tt.errorType) {
				t.Error("Result should contain error type")
			}
			if !strings.Contains(result, tt.message) {
				t.Error("Result should contain message")
			}
			if !strings.Contains(result, tt.fieldName) {
				t.Error("Result should contain field name")
			}
		})
	}
}

func TestFormatError_VariadicFieldNameParameter(t *testing.T) {
	// Test that FormatError correctly handles the variadic fieldName parameter
	tests := []struct {
		name          string
		errorType     string
		message       string
		fieldNameArgs []string
		want          string
	}{
		{
			name:          "no field name provided",
			errorType:     "required",
			message:       "Field is required",
			fieldNameArgs: nil,
			want:          "[required] Field is required",
		},
		{
			name:          "empty field name provided",
			errorType:     "format",
			message:       "Invalid format",
			fieldNameArgs: []string{""},
			want:          "[format] Invalid format",
		},
		{
			name:          "single field name provided",
			errorType:     "range",
			message:       "Out of range",
			fieldNameArgs: []string{"age"},
			want:          "[range] age: Out of range",
		},
		{
			name:          "field name with whitespace",
			errorType:     "type",
			message:       "Wrong type",
			fieldNameArgs: []string{"  email  "},
			want:          "[type] email: Wrong type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			if len(tt.fieldNameArgs) == 0 {
				got = FormatError(tt.errorType, tt.message)
			} else {
				got = FormatError(tt.errorType, tt.message, tt.fieldNameArgs[0])
			}

			if got != tt.want {
				t.Errorf("FormatError() = %v, want %v", got, tt.want)
			}
		})
	}
}
