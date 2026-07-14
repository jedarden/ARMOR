package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// FORMATERRORWITHYPE COMPREHENSIVE TESTS
// =============================================================================

func TestFormatErrorWithType_ErrorTypeClassification(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "required error with field",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "email",
			want:      "[required] email: Field is required",
		},
		{
			name:      "format error without field",
			errorType: ErrTypeFormat,
			message:   "Invalid email format",
			fieldName: "",
			want:      "[format] Invalid email format",
		},
		{
			name:      "range error with field",
			errorType: ErrTypeRange,
			message:   "Value out of range",
			fieldName: "age",
			want:      "[range] age: Value out of range",
		},
		{
			name:      "type error",
			errorType: ErrTypeType,
			message:   "Wrong type",
			fieldName: "count",
			want:      "[type] count: Wrong type",
		},
		{
			name:      "length error",
			errorType: ErrTypeLength,
			message:   "Too long",
			fieldName: "password",
			want:      "[length] password: Too long",
		},
		{
			name:      "value error",
			errorType: ErrTypeValue,
			message:   "Invalid value",
			fieldName: "option",
			want:      "[value] option: Invalid value",
		},
		{
			name:      "duplicate error",
			errorType: ErrTypeDuplicate,
			message:   "Already exists",
			fieldName: "email",
			want:      "[duplicate] email: Already exists",
		},
		{
			name:      "conflict error",
			errorType: ErrTypeConflict,
			message:   "Conflicting values",
			fieldName: "dates",
			want:      "[conflict] dates: Conflicting values",
		},
		{
			name:      "unknown error type",
			errorType: ErrTypeUnknown,
			message:   "Unknown error",
			fieldName: "",
			want:      "[unknown] Unknown error",
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

func TestFormatErrorWithType_EmptyMessageHandling(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "empty message with field name",
			errorType: ErrTypeRequired,
			message:   "",
			fieldName: "email",
			want:      "[required] email: email validation failed",
		},
		{
			name:      "empty message without field name",
			errorType: ErrTypeFormat,
			message:   "",
			fieldName: "",
			want:      "[format] (no message provided)",
		},
		{
			name:      "empty message with range type and field",
			errorType: ErrTypeRange,
			message:   "",
			fieldName: "age",
			want:      "[range] age: age validation failed",
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

func TestFormatErrorWithType_ConsistentStructure(t *testing.T) {
	// Test that all ErrorType values produce consistent structure
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

	for _, et := range allTypes {
		t.Run(et.String(), func(t *testing.T) {
			result := FormatErrorWithType(et, "Test message", "field")

			// Verify structure starts with [error_type]
			expectedPrefix := "[" + et.String() + "]"
			if !strings.HasPrefix(result, expectedPrefix) {
				t.Errorf("FormatErrorWithType(%v, ...) should start with %v, got: %q",
					et, expectedPrefix, result)
			}

			// Verify message is included
			if !strings.Contains(result, "Test message") {
				t.Errorf("FormatErrorWithType(%v, ...) should contain message, got: %q",
					et, result)
			}

			// Verify field name is included
			if !strings.Contains(result, "field:") {
				t.Errorf("FormatErrorWithType(%v, ...) should include field name, got: %q",
					et, result)
			}
		})
	}
}

func TestFormatErrorWithType_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
	}{
		{
			name:      "newlines in message",
			errorType: ErrTypeRequired,
			message:   "Line 1\nLine 2",
			fieldName: "field",
		},
		{
			name:      "tabs in message",
			errorType: ErrTypeFormat,
			message:   "Error:\t404",
			fieldName: "status",
		},
		{
			name:      "unicode characters",
			errorType: ErrTypeValue,
			message:   "Error: ✓ ✗ →",
			fieldName: "check",
		},
		{
			name:      "quotes in message",
			errorType: ErrTypeType,
			message:   `"quoted" and 'single'`,
			fieldName: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithType(tt.errorType, tt.message, tt.fieldName)

			// Should not panic and should contain the message
			if !strings.Contains(result, tt.message) {
				t.Errorf("FormatErrorWithType() should contain special characters, got: %q", result)
			}

			// Should have consistent structure
			expectedPrefix := "[" + tt.errorType.String() + "]"
			if !strings.HasPrefix(result, expectedPrefix) {
				t.Errorf("FormatErrorWithType() should start with %v, got: %q",
					expectedPrefix, result)
			}
		})
	}
}

func TestFormatErrorWithType_NoNilPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("FormatErrorWithType should not panic, got: %v", r)
		}
	}()

	// Test with empty message
	FormatErrorWithType(ErrTypeRequired, "", "field")

	// Test with empty field name
	FormatErrorWithType(ErrTypeFormat, "message", "")

	// Test with both empty
	FormatErrorWithType(ErrTypeRange, "", "")

	// Test with normal values
	FormatErrorWithType(ErrTypeType, "test message", "field")
}

func TestFormatErrorWithType_ErrorTypeEnumMethods(t *testing.T) {
	tests := []struct {
		name        string
		errorType   ErrorType
		expectValid bool
		description string
	}{
		{"ErrTypeRequired", ErrTypeRequired, true, "Required field is missing or empty"},
		{"ErrTypeFormat", ErrTypeFormat, true, "Value format is invalid"},
		{"ErrTypeRange", ErrTypeRange, true, "Value is outside acceptable range"},
		{"ErrTypeLength", ErrTypeLength, true, "String length or collection size is invalid"},
		{"ErrTypeType", ErrTypeType, true, "Value type is incorrect"},
		{"ErrTypeValue", ErrTypeValue, true, "Value is invalid"},
		{"ErrTypeDuplicate", ErrTypeDuplicate, true, "Duplicate value detected"},
		{"ErrTypeConflict", ErrTypeConflict, true, "Conflict with existing values or constraints"},
		{"ErrTypeUnknown", ErrTypeUnknown, true, "Unknown error type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IsValid method
			if tt.errorType.IsValid() != tt.expectValid {
				t.Errorf("ErrorType.IsValid() = %v, want %v", tt.errorType.IsValid(), tt.expectValid)
			}

			// Test Description method
			if tt.errorType.Description() != tt.description {
				t.Errorf("ErrorType.Description() = %v, want %v", tt.errorType.Description(), tt.description)
			}

			// Test String method
			if tt.errorType.String() != string(tt.errorType) {
				t.Errorf("ErrorType.String() = %v, want %v", tt.errorType.String(), string(tt.errorType))
			}

			// Test that FormatErrorWithType uses the ErrorType correctly
			result := FormatErrorWithType(tt.errorType, "Test message", "field")
			if !strings.Contains(result, tt.errorType.String()) {
				t.Errorf("FormatErrorWithType result should contain error type string, got: %v", result)
			}
		})
	}
}

func TestFormatErrorWithType_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		scenario  string
	}{
		{
			name:      "API validation - required email",
			errorType: ErrTypeRequired,
			message:   "Email address is required for user registration",
			fieldName: "user.email",
			scenario:  "User registration API",
		},
		{
			name:      "Form validation - invalid email format",
			errorType: ErrTypeFormat,
			message:   "Email must be in format user@domain.com",
			fieldName: "email",
			scenario:  "Contact form submission",
		},
		{
			name:      "Data validation - age out of range",
			errorType: ErrTypeRange,
			message:   "Age must be between 18 and 120",
			fieldName: "user.age",
			scenario:  "User profile update",
		},
		{
			name:      "Password validation - too short",
			errorType: ErrTypeLength,
			message:   "Password must be at least 8 characters",
			fieldName: "password",
			scenario:  "Password creation",
		},
		{
			name:      "Type validation - wrong type",
			errorType: ErrTypeType,
			message:   "Expected number, got string",
			fieldName: "count",
			scenario:  "Data import",
		},
		{
			name:      "Business logic - duplicate email",
			errorType: ErrTypeDuplicate,
			message:   "Email address already registered",
			fieldName: "email",
			scenario:  "User registration",
		},
		{
			name:      "Business logic - date conflict",
			errorType: ErrTypeConflict,
			message:   "Start date must be before end date",
			fieldName: "reservation.dates",
			scenario:  "Reservation system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithType(tt.errorType, tt.message, tt.fieldName)

			// Verify error type is present
			if !strings.Contains(result, tt.errorType.String()) {
				t.Errorf("Result should contain error type %v, got: %v", tt.errorType, result)
			}

			// Verify field name is present
			if !strings.Contains(result, tt.fieldName) {
				t.Errorf("Result should contain field name %v, got: %v", tt.fieldName, result)
			}

			// Verify message is present
			if !strings.Contains(result, tt.message) {
				t.Errorf("Result should contain message %v, got: %v", tt.message, result)
			}

			// Verify consistent structure
			expectedPrefix := "[" + tt.errorType.String() + "]"
			if !strings.HasPrefix(result, expectedPrefix) {
				t.Errorf("Result should start with %v, got: %v", expectedPrefix, result)
			}
		})
	}
}

func TestFormatErrorWithType_AllErrorTypes(t *testing.T) {
	// Test that all ErrorType enum values produce valid output
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

	for _, et := range allTypes {
		t.Run(et.String(), func(t *testing.T) {
			result := FormatErrorWithType(et, "Test message", "field")

			// Check that result contains error type
			if !strings.Contains(result, et.String()) {
				t.Errorf("ErrorType %v should produce output containing %v, got %v", et, et.String(), result)
			}

			// Check that result contains field name
			if !strings.Contains(result, "field") {
				t.Errorf("ErrorType %v should produce output containing field name, got %v", et, result)
			}

			// Check that result contains message
			if !strings.Contains(result, "Test message") {
				t.Errorf("ErrorType %v should produce output containing message, got %v", et, result)
			}
		})
	}
}

// =============================================================================
// BACKWARD COMPATIBILITY TESTS
// =============================================================================

func TestFormatError_BackwardCompatibility(t *testing.T) {
	// Test that string-based FormatError produces consistent results
	// with ErrorType-based FormatErrorWithType
	t.Run("string and ErrorType functions produce consistent results", func(t *testing.T) {
		message := "Field is required"
		fieldName := "email"

		// String-based function
		stringResult := FormatError("required", message, fieldName)

		// ErrorType-based function
		typeResult := FormatErrorWithType(ErrTypeRequired, message, fieldName)

		if stringResult != typeResult {
			t.Errorf("String and ErrorType functions should produce same result:\n  String: %v\n  Type:   %v", stringResult, typeResult)
		}
	})

	t.Run("format error consistency", func(t *testing.T) {
		message := "Invalid format"
		fieldName := "email"

		stringResult := FormatError("format", message, fieldName)
		typeResult := FormatErrorWithType(ErrTypeFormat, message, fieldName)

		if stringResult != typeResult {
			t.Errorf("Format inconsistency:\n  String: %v\n  Type:   %v", stringResult, typeResult)
		}
	})

	t.Run("range error consistency", func(t *testing.T) {
		message := "Value out of range"
		fieldName := "age"

		stringResult := FormatError("range", message, fieldName)
		typeResult := FormatErrorWithType(ErrTypeRange, message, fieldName)

		if stringResult != typeResult {
			t.Errorf("Format inconsistency:\n  String: %v\n  Type:   %v", stringResult, typeResult)
		}
	})

	t.Run("type error consistency", func(t *testing.T) {
		message := "Wrong type"
		fieldName := "count"

		stringResult := FormatError("type", message, fieldName)
		typeResult := FormatErrorWithType(ErrTypeType, message, fieldName)

		if stringResult != typeResult {
			t.Errorf("Format inconsistency:\n  String: %v\n  Type:   %v", stringResult, typeResult)
		}
	})

	t.Run("length error consistency", func(t *testing.T) {
		message := "Too short"
		fieldName := "password"

		stringResult := FormatError("length", message, fieldName)
		typeResult := FormatErrorWithType(ErrTypeLength, message, fieldName)

		if stringResult != typeResult {
			t.Errorf("Format inconsistency:\n  String: %v\n  Type:   %v", stringResult, typeResult)
		}
	})

	t.Run("value error consistency", func(t *testing.T) {
		message := "Invalid value"
		fieldName := "option"

		stringResult := FormatError("value", message, fieldName)
		typeResult := FormatErrorWithType(ErrTypeValue, message, fieldName)

		if stringResult != typeResult {
			t.Errorf("Format inconsistency:\n  String: %v\n  Type:   %v", stringResult, typeResult)
		}
	})

	t.Run("duplicate error consistency", func(t *testing.T) {
		message := "Already exists"
		fieldName := "email"

		stringResult := FormatError("duplicate", message, fieldName)
		typeResult := FormatErrorWithType(ErrTypeDuplicate, message, fieldName)

		if stringResult != typeResult {
			t.Errorf("Format inconsistency:\n  String: %v\n  Type:   %v", stringResult, typeResult)
		}
	})

	t.Run("conflict error consistency", func(t *testing.T) {
		message := "Conflicting values"
		fieldName := "dates"

		stringResult := FormatError("conflict", message, fieldName)
		typeResult := FormatErrorWithType(ErrTypeConflict, message, fieldName)

		if stringResult != typeResult {
			t.Errorf("Format inconsistency:\n  String: %v\n  Type:   %v", stringResult, typeResult)
		}
	})
}

func TestFormatError_ConsistentClassification(t *testing.T) {
	// Test that all ErrorType enum values are properly classified
	t.Run("all ErrorType values produce valid output", func(t *testing.T) {
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

		for _, et := range allTypes {
			result := FormatErrorWithType(et, "Test message", "field")

			// Check that result contains error type
			if !strings.Contains(result, et.String()) {
				t.Errorf("ErrorType %v should produce output containing %v, got %v", et, et.String(), result)
			}

			// Check that result contains field name
			if !strings.Contains(result, "field") {
				t.Errorf("ErrorType %v should produce output containing field name, got %v", et, result)
			}

			// Check that result contains message
			if !strings.Contains(result, "Test message") {
				t.Errorf("ErrorType %v should produce output containing message, got %v", et, result)
			}
		}
	})

	t.Run("string-based error types are properly handled", func(t *testing.T) {
		errorTypes := []string{"required", "format", "range", "type", "unknown"}

		for _, etStr := range errorTypes {
			result := FormatError(etStr, "Test message", "field")

			// Check that result contains error type
			if !strings.Contains(result, etStr) {
				t.Errorf("Error type %v should produce output containing error type, got %v", etStr, result)
			}
		}
	})
}

func TestFormatError_ExistingCallsCompatibility(t *testing.T) {
	// Test that existing FormatError calls continue to work correctly
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "status code error without field",
			errorType: "status_code",
			message:   "Expected 200 but got 404",
			fieldName: "",
			want:      "[status_code] Expected 200 but got 404",
		},
		{
			name:      "error message with field name",
			errorType: "error_message",
			message:   "Pattern not found in response",
			fieldName: "response",
			want:      "[error_message] response: Pattern not found in response",
		},
		{
			name:      "content type error without field",
			errorType: "content_type",
			message:   "Expected application/json but got text/html",
			fieldName: "",
			want:      "[content_type] Expected application/json but got text/html",
		},
		{
			name:      "validation error with field",
			errorType: "validation",
			message:   "Field 'email' is required",
			fieldName: "email",
			want:      "[validation] email: Field 'email' is required",
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
			if result != tt.want {
				t.Errorf("FormatError() = %q, want %q", result, tt.want)
			}
		})
	}
}

// =============================================================================
// INVALID ERROR TYPE TESTS
// =============================================================================

func TestFormatError_InvalidErrorType(t *testing.T) {
	// Test FormatError with invalid/unknown string error types
	// Invalid types should be tracked but still work (fallback behavior)

	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		want      string
		wantTracked bool // Whether the error type should be tracked as invalid
	}{
		{
			name:      "completely invalid error type",
			errorType: "invalid_error_type",
			message:   "Something went wrong",
			fieldName: "field",
			want:      "[invalid_error_type] field: Something went wrong",
			wantTracked: true,
		},
		{
			name:      "custom error type",
			errorType: "custom_validation",
			message:   "Custom check failed",
			fieldName: "email",
			want:      "[custom_validation] email: Custom check failed",
			wantTracked: true,
		},
		{
			name:      "typo in error type",
			errorType: "require", // typo for "required"
			message:   "Field is required",
			fieldName: "name",
			want:      "[require] name: Field is required",
			wantTracked: true,
		},
		{
			name:      "mixed case error type (case-insensitive match, not tracked as invalid)",
			errorType: "Required",
			message:   "Field is required",
			fieldName: "age",
			want:      "[Required] age: Field is required",
			wantTracked: false, // Should NOT be tracked as invalid due to case-insensitive matching
		},
		{
			name:      "numeric error type",
			errorType: "123",
			message:   "Numeric error type",
			fieldName: "",
			want:      "[123] Numeric error type",
			wantTracked: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tracking before each test
			ResetInvalidErrorTypeTracking()

			// Clear any existing tracking
			initialCount := InvalidErrorTypeCount()
			if initialCount > 0 {
				t.Logf("Warning: %d invalid error types already tracked", initialCount)
			}

			result := FormatError(tt.errorType, tt.message, tt.fieldName)

			if result != tt.want {
				t.Errorf("FormatError() = %q, want %q", result, tt.want)
			}

			// Verify tracking behavior matches expectations
			invalidTypes := GetInvalidErrorTypes()
			wasTracked := false
			if count, found := invalidTypes[tt.errorType]; found && count > 0 {
				wasTracked = true
			}

			if tt.wantTracked && !wasTracked {
				t.Errorf("Error type %q should have been tracked as invalid", tt.errorType)
			}
			if !tt.wantTracked && wasTracked && tt.errorType != "" && tt.errorType != "unknown" {
				t.Errorf("Error type %q should NOT have been tracked as invalid", tt.errorType)
			}
		})
	}
}

func TestFormatError_EmptyAndWhitespaceHandling(t *testing.T) {
	// Test handling of empty and whitespace-only inputs
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
			fieldName: "email",
			want:      "[error] email: Something went wrong",
		},
		{
			name:      "empty error type without field",
			errorType: "",
			message:   "Generic error",
			fieldName: "",
			want:      "[error] Generic error",
		},
		{
			name:      "whitespace-only error type (trimmed to empty by FormatErrorMessage)",
			errorType: "   ",
			message:   "Test message",
			fieldName: "field",
			want:      "field: Test message", // No [error_type] prefix because whitespace is trimmed
		},
		{
			name:      "empty message with field name",
			errorType: "required",
			message:   "",
			fieldName: "email",
			want:      "[required] email: email validation failed",
		},
		{
			name:      "empty message without field name",
			errorType: "format",
			message:   "",
			fieldName: "",
			want:      "[format] (no message provided)",
		},
		{
			name:      "whitespace-only message with field",
			errorType: "range",
			message: "   ",
			fieldName: "age",
			want:      "[range] age: age validation failed",
		},
		{
			name:      "whitespace-only message without field",
			errorType: "type",
			message: "   ",
			fieldName: "",
			want:      "[type] (no message provided)",
		},
		{
			name:      "message with leading/trailing whitespace",
			errorType: "value",
			message: "  Invalid value  ",
			fieldName: "option",
			want:      "[value] option: Invalid value",
		},
		{
			name:      "all empty inputs",
			errorType: "",
			message:   "",
			fieldName: "",
			want:      "[error] (no message provided)",
		},
		{
			name:      "empty field name with valid error type and message",
			errorType: "duplicate",
			message:   "Already exists",
			fieldName: "",
			want:      "[duplicate] Already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatError(tt.errorType, tt.message, tt.fieldName)
			if result != tt.want {
				t.Errorf("FormatError() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestFormatErrorWithType_WhitespaceOnlyMessages(t *testing.T) {
	// Test FormatErrorWithType with whitespace-only messages
	// This ensures the fix for whitespace-only message handling works correctly
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "space only message with field",
			errorType: ErrTypeRequired,
			message:   " ",
			fieldName: "email",
			want:      "[required] email: email validation failed",
		},
		{
			name:      "multiple spaces message with field",
			errorType: ErrTypeFormat,
			message: "    ",
			fieldName: "password",
			want:      "[format] password: password validation failed",
		},
		{
			name:      "tab-only message without field",
			errorType: ErrTypeRange,
			message: "\t",
			fieldName: "",
			want:      "[range] (no message provided)",
		},
		{
			name:      "mixed whitespace message",
			errorType: ErrTypeLength,
			message: " \t ",
			fieldName: "username",
			want:      "[length] username: username validation failed",
		},
		{
			name:      "newlines-only message",
			errorType: ErrTypeType,
			message: "\n\n",
			fieldName: "count",
			want:      "[type] count: count validation failed",
		},
		{
			name:      "mixed newlines and spaces",
			errorType: ErrTypeValue,
			message: " \n \n ",
			fieldName: "status",
			want:      "[value] status: status validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithType(tt.errorType, tt.message, tt.fieldName)
			if result != tt.want {
				t.Errorf("FormatErrorWithType() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestFormatError_InvalidErrorTypeTracking(t *testing.T) {
	// Test that invalid error types are properly tracked
	ResetInvalidErrorTypeTracking()

	// Use some invalid error types
	FormatError("invalid_type_1", "message", "field1")
	FormatError("invalid_type_2", "message", "field2")
	FormatError("invalid_type_1", "message", "field3") // duplicate

	invalidTypes := GetInvalidErrorTypes()

	// Check counts
	if count := invalidTypes["invalid_type_1"]; count != 2 {
		t.Errorf("invalid_type_1 count = %d, want 2", count)
	}
	if count := invalidTypes["invalid_type_2"]; count != 1 {
		t.Errorf("invalid_type_2 count = %d, want 1", count)
	}

	// Total unique invalid types
	if count := InvalidErrorTypeCount(); count != 2 {
		t.Errorf("InvalidErrorTypeCount() = %d, want 2", count)
	}

	// Reset and verify
	ResetInvalidErrorTypeTracking()
	if count := InvalidErrorTypeCount(); count != 0 {
		t.Errorf("After reset, InvalidErrorTypeCount() = %d, want 0", count)
	}
}

func TestFormatError_ValidErrorTypesNotTracked(t *testing.T) {
	// Test that valid ErrorType values are NOT tracked as invalid
	ResetInvalidErrorTypeTracking()

	validTypes := []string{
		"required", "format", "range", "length", "type",
		"value", "duplicate", "conflict", "unknown",
	}

	for _, errorType := range validTypes {
		FormatError(errorType, "test message", "field")
	}

	invalidTypes := GetInvalidErrorTypes()
	if count := InvalidErrorTypeCount(); count != 0 {
		t.Errorf("Valid error types should not be tracked, got %d tracked", count)
	}

	for _, errorType := range validTypes {
		if count, found := invalidTypes[errorType]; found {
			t.Errorf("Valid error type %q should not be tracked, but found count %d", errorType, count)
		}
	}
}

func TestFormatError_CaseInsensitiveErrorTypeMatching(t *testing.T) {
	// Test that ErrorType matching is case-insensitive
	tests := []struct {
		name      string
		errorType string
		wantValid bool
	}{
		{"lowercase required", "required", true},
		{"uppercase REQUIRED", "REQUIRED", true},
		{"mixed case ReQuIrEd", "ReQuIrEd", true},
		{"lowercase format", "format", true},
		{"uppercase FORMAT", "FORMAT", true},
		{"mixed case FoRmAt", "FoRmAt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetInvalidErrorTypeTracking()

			result := FormatError(tt.errorType, "test message", "field")

			// Check if it was tracked as invalid
			invalidTypes := GetInvalidErrorTypes()
			wasTracked := false
			for errorType := range invalidTypes {
				if errorType == tt.errorType {
					wasTracked = true
					break
				}
			}

			if tt.wantValid && wasTracked {
				t.Errorf("Error type %q should be valid but was tracked as invalid", tt.errorType)
			}

			// Result should always contain the error type string
			if !contains(result, tt.errorType) {
				t.Errorf("Result should contain error type %q, got %q", tt.errorType, result)
			}
		})
	}
}

func TestFormatErrorWithType_EmptyFieldNameComprehensive(t *testing.T) {
	// Comprehensive tests for empty fieldName handling in FormatErrorWithType
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "empty fieldName with all ErrorType values",
			errorType: ErrTypeRequired,
			message:   "Required field missing",
			fieldName: "",
			want:      "[required] Required field missing",
		},
		{
			name:      "empty fieldName with format error",
			errorType: ErrTypeFormat,
			message:   "Invalid format",
			fieldName: "",
			want:      "[format] Invalid format",
		},
		{
			name:      "empty fieldName with range error",
			errorType: ErrTypeRange,
			message:   "Out of range",
			fieldName: "",
			want:      "[range] Out of range",
		},
		{
			name:      "whitespace-only fieldName treated as empty",
			errorType: ErrTypeLength,
			message:   "Too long",
			fieldName: "   ",
			want:      "[length] Too long",
		},
		{
			name:      "tab-only fieldName",
			errorType: ErrTypeType,
			message:   "Wrong type",
			fieldName: "\t",
			want:      "[type] Wrong type",
		},
		{
			name:      "newline-only fieldName",
			errorType: ErrTypeValue,
			message:   "Invalid value",
			fieldName: "\n",
			want:      "[value] Invalid value",
		},
		{
			name:      "mixed whitespace fieldName",
			errorType: ErrTypeDuplicate,
			message:   "Duplicate found",
			fieldName: " \t\n ",
			want:      "[duplicate] Duplicate found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithType(tt.errorType, tt.message, tt.fieldName)
			if result != tt.want {
				t.Errorf("FormatErrorWithType() = %q, want %q", result, tt.want)
			}

			// Verify that fieldName doesn't appear in result when it's whitespace-only
			if isWhitespace(tt.fieldName) {
				if contains(result, tt.fieldName) {
					t.Errorf("Whitespace-only fieldName should not appear in result: %q", result)
				}
			}
		})
	}
}

func TestFormatError_AllErrorTypeEnumValuesWork(t *testing.T) {
	// Test that ALL ErrorType enum values produce valid output
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

	for _, et := range allTypes {
		t.Run(et.String(), func(t *testing.T) {
			// Test with all parameters
			result := FormatErrorWithType(et, "Test message", "field")

			// Verify result is not empty
			if result == "" {
				t.Errorf("ErrorType %v should produce non-empty output", et)
			}

			// Verify result contains error type
			if !contains(result, et.String()) {
				t.Errorf("ErrorType %v output should contain error type string, got: %q", et, result)
			}

			// Verify result contains message
			if !contains(result, "Test message") {
				t.Errorf("ErrorType %v output should contain message, got: %q", et, result)
			}

			// Verify result contains field name
			if !contains(result, "field") {
				t.Errorf("ErrorType %v output should contain field name, got: %q", et, result)
			}

			// Verify structure starts with [error_type]
			expectedPrefix := "[" + et.String() + "]"
			if !hasPrefix(result, expectedPrefix) {
				t.Errorf("ErrorType %v output should start with %v, got: %q", et, expectedPrefix, result)
			}
		})
	}
}

func TestFormatError_ConsistencyWithTypeVariants(t *testing.T) {
	// Test consistency between string-based FormatError and ErrorType-based FormatErrorWithType
	testCases := []struct {
		errorTypeString string
		errorTypeEnum   ErrorType
		message         string
		fieldName       string
	}{
		{"required", ErrTypeRequired, "Field is required", "email"},
		{"format", ErrTypeFormat, "Invalid format", "email"},
		{"range", ErrTypeRange, "Out of range", "age"},
		{"length", ErrTypeLength, "Too short", "password"},
		{"type", ErrTypeType, "Wrong type", "count"},
		{"value", ErrTypeValue, "Invalid value", "option"},
		{"duplicate", ErrTypeDuplicate, "Already exists", "email"},
		{"conflict", ErrTypeConflict, "Conflicting values", "dates"},
		{"unknown", ErrTypeUnknown, "Unknown error", "field"},
	}

	for _, tc := range testCases {
		t.Run(tc.errorTypeString, func(t *testing.T) {
			// Test with all parameters
			stringResult := FormatError(tc.errorTypeString, tc.message, tc.fieldName)
			enumResult := FormatErrorWithType(tc.errorTypeEnum, tc.message, tc.fieldName)

			if stringResult != enumResult {
				t.Errorf("FormatError and FormatErrorWithType should produce identical results:\n  String: %q\n  Enum:   %q", stringResult, enumResult)
			}

			// Test without field name
			stringResultNoField := FormatError(tc.errorTypeString, tc.message)
			enumResultNoField := FormatErrorWithType(tc.errorTypeEnum, tc.message, "")

			if stringResultNoField != enumResultNoField {
				t.Errorf("Results without field should match:\n  String: %q\n  Enum:   %q", stringResultNoField, enumResultNoField)
			}
		})
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func hasPrefix(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

func isWhitespace(s string) bool {
	return strings.TrimSpace(s) == "" && s != ""
}

