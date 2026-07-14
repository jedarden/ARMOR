package validate

import (
	"strings"
	"testing"
)

// TestFormatError_ErrorTypeParameter verifies that FormatError accepts ErrorType parameter
// and implements basic ErrorType-based message formatting.
func TestFormatError_ErrorTypeParameter(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "Required field error",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "email",
			want:      "[required] email: Field is required",
		},
		{
			name:      "Format error without field",
			errorType: ErrTypeFormat,
			message:   "Invalid email format",
			fieldName: "",
			want:      "[format] Invalid email format",
		},
		{
			name:      "Range error with field",
			errorType: ErrTypeRange,
			message:   "Value out of range",
			fieldName: "age",
			want:      "[range] age: Value out of range",
		},
		{
			name:      "Length error",
			errorType: ErrTypeLength,
			message:   "String too short",
			fieldName: "password",
			want:      "[length] password: String too short",
		},
		{
			name:      "Type error",
			errorType: ErrTypeType,
			message:   "Wrong type",
			fieldName: "count",
			want:      "[type] count: Wrong type",
		},
		{
			name:      "Value error",
			errorType: ErrTypeValue,
			message:   "Invalid value",
			fieldName: "option",
			want:      "[value] option: Invalid value",
		},
		{
			name:      "Duplicate error",
			errorType: ErrTypeDuplicate,
			message:   "Email already exists",
			fieldName: "email",
			want:      "[duplicate] email: Email already exists",
		},
		{
			name:      "Conflict error",
			errorType: ErrTypeConflict,
			message:   "Start date cannot be after end date",
			fieldName: "date_range",
			want:      "[conflict] date_range: Start date cannot be after end date",
		},
		{
			name:      "Unknown error type",
			errorType: ErrTypeUnknown,
			message:   "Unknown error occurred",
			fieldName: "field",
			want:      "[unknown] field: Unknown error occurred",
		},
		{
			name:      "Empty message with field generates fallback",
			errorType: ErrTypeRequired,
			message:   "",
			fieldName: "email",
			want:      "[required] email: email validation failed",
		},
		{
			name:      "Empty message without field generates default",
			errorType: ErrTypeFormat,
			message:   "",
			fieldName: "",
			want:      "[format] (no message provided)",
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

// TestFormatError_ErrorTypeToMessageMapping documents the ErrorType to message mapping.
func TestFormatError_ErrorTypeToMessageMapping(t *testing.T) {
	// This test documents how each ErrorType maps to error messages:
	// - ErrTypeRequired (required): Required field is missing or empty
	// - ErrTypeFormat (format): Value format is invalid (e.g., email, UUID pattern)
	// - ErrTypeRange (range): Value is outside acceptable numeric range (min/max)
	// - ErrTypeLength (length): String length or collection size is invalid
	// - ErrTypeType (type): Value type is incorrect (e.g., string when int expected)
	// - ErrTypeValue (value): Value is invalid for domain-specific reasons
	// - ErrTypeDuplicate (duplicate): Duplicate value detected
	// - ErrTypeConflict (conflict): Conflict with existing values or constraints
	// - ErrTypeUnknown (unknown): Unknown error type (default/fallback)

	errorTypes := []ErrorType{
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

	for _, et := range errorTypes {
		t.Run(et.String(), func(t *testing.T) {
			// Verify that the error type is correctly formatted
			formatted := FormatError(et, "Test message", "test_field")

			// Check that the formatted string contains the error type
			if !strings.Contains(formatted, "["+et.String()+"]") {
				t.Errorf("FormatError(%s) should contain [%s] in output, got: %s", et, et.String(), formatted)
			}

			// Check that the formatted string contains the field name
			if !strings.Contains(formatted, "test_field") {
				t.Errorf("FormatError(%s) should contain field name in output, got: %s", et, formatted)
			}

			// Check that the formatted string contains the message
			if !strings.Contains(formatted, "Test message") {
				t.Errorf("FormatError(%s) should contain message in output, got: %s", et, formatted)
			}
		})
	}
}

// TestFormatError_ErrorTypeParameterCoverage verifies that all defined ErrorType values
// produce valid, non-empty formatted error messages when using ErrorType parameter.
func TestFormatError_ErrorTypeParameterCoverage(t *testing.T) {
	errorTypes := []ErrorType{
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

	for _, et := range errorTypes {
		t.Run(et.String(), func(t *testing.T) {
			// Test with all parameters provided
			result := FormatError(et, "Test message", "test_field")
			if result == "" {
				t.Errorf("FormatError(%s) returned empty string", et)
			}
			if !strings.Contains(result, et.String()) {
				t.Errorf("FormatError(%s) output doesn't contain error type: %s", et, result)
			}

			// Test with empty field name
			result = FormatError(et, "Test message", "")
			if result == "" {
				t.Errorf("FormatError(%s, empty fieldName) returned empty string", et)
			}

			// Test with empty message
			result = FormatError(et, "", "test_field")
			if result == "" {
				t.Errorf("FormatError(%s, empty message) returned empty string", et)
			}
			if !strings.Contains(result, "validation failed") {
				t.Errorf("FormatError(%s, empty message) should contain 'validation failed': %s", et, result)
			}
		})
	}
}
