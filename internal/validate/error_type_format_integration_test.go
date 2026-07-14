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
