package validate

import (
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// FIELD REFERENCE FORMATTING TESTS
// =============================================================================

func TestFormatFieldRef(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		parent    string
		expected  string
	}{
		{
			name:      "field with parent",
			fieldName: "email",
			parent:    "request",
			expected:  "request.email",
		},
		{
			name:      "field without parent",
			fieldName: "email",
			parent:    "",
			expected:  "email",
		},
		{
			name:      "empty field name with parent",
			fieldName: "",
			parent:    "request",
			expected:  "request",
		},
		{
			name:      "empty field name without parent",
			fieldName: "",
			parent:    "",
			expected:  "(unknown field)",
		},
		{
			name:      "nested field",
			fieldName: "address.city",
			parent:    "user",
			expected:  "user.address.city",
		},
		{
			name:      "whitespace handling",
			fieldName: "  email  ",
			parent:    "  request  ",
			expected:  "request.email",
		},
		{
			name:      "array index in field name",
			fieldName: "users.0.email",
			parent:    "response",
			expected:  "response.users[0].email",
		},
		{
			name:      "array index without parent",
			fieldName: "users.0.email",
			parent:    "",
			expected:  "users[0].email",
		},
		{
			name:      "array index with bracket notation",
			fieldName: "users[0].email",
			parent:    "data",
			expected:  "data.users[0].email",
		},
		{
			name:      "multiple array indices",
			fieldName: "data.0.items.1.name",
			parent:    "response",
			expected:  "response.data[0].items[1].name",
		},
		{
			name:      "parent with array index",
			fieldName: "email",
			parent:    "users.0",
			expected:  "users[0].email",
		},
		{
			name:      "both parent and field with array indices",
			fieldName: "profile.email",
			parent:    "users.0",
			expected:  "users[0].profile.email",
		},
		{
			name:      "array index at start of field",
			fieldName: "0.email",
			parent:    "users",
			expected:  "users.0.email",
		},
		{
			name:      "empty field name with array parent",
			fieldName: "",
			parent:    "users.0",
			expected:  "users[0]",
		},
		{
			name:      "deeply nested with array indices",
			fieldName: "items.0.data.values.1.field",
			parent:    "response",
			expected:  "response.items[0].data.values[1].field",
		},
		{
			name:      "negative array index",
			fieldName: "users.-1.email",
			parent:    "response",
			expected:  "response.users[-1].email",
		},
		{
			name:      "large array index",
			fieldName: "users.12345.name",
			parent:    "",
			expected:  "users[12345].name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldRef(tt.fieldName, tt.parent)
			if result != tt.expected {
				t.Errorf("FormatFieldRef() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatFieldLocationWith(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		location  string
		parent    string
		expected  string
	}{
		{
			name:      "field with location",
			fieldName: "email",
			location:  "line 5",
			parent:    "request",
			expected:  "request.email at line 5",
		},
		{
			name:      "field without location",
			fieldName: "email",
			location:  "",
			parent:    "request",
			expected:  "request.email",
		},
		{
			name:      "field with location no parent",
			fieldName: "email",
			location:  "position 123",
			parent:    "",
			expected:  "email at position 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldLocationWith(tt.fieldName, tt.location, tt.parent)
			if result != tt.expected {
				t.Errorf("FormatFieldLocationWith() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatFieldListWith(t *testing.T) {
	tests := []struct {
		name        string
		fields      []string
		conjunction string
		expected    string
	}{
		{
			name:        "empty list",
			fields:      []string{},
			conjunction: "and",
			expected:    "",
		},
		{
			name:        "single field",
			fields:      []string{"email"},
			conjunction: "and",
			expected:    "email",
		},
		{
			name:        "two fields",
			fields:      []string{"email", "password"},
			conjunction: "and",
			expected:    "email and password",
		},
		{
			name:        "three fields",
			fields:      []string{"email", "password", "name"},
			conjunction: "and",
			expected:    "email, password, and name",
		},
		{
			name:        "four fields",
			fields:      []string{"email", "password", "name", "age"},
			conjunction: "and",
			expected:    "email, password, name, and age",
		},
		{
			name:        "or conjunction",
			fields:      []string{"email", "phone"},
			conjunction: "or",
			expected:    "email or phone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldListWith(tt.fields, tt.conjunction)
			if result != tt.expected {
				t.Errorf("FormatFieldListWith() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// ERROR MESSAGE FORMATTING TESTS
// =============================================================================

func TestFormatErrorMessage(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		expected  string
	}{
		{
			name:      "with all components",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "email",
			expected:  "[required] email: Field is required",
		},
		{
			name:      "without field name",
			errorType: ErrTypeFormat,
			message:   "Invalid email format",
			fieldName: "",
			expected:  "[format] Invalid email format",
		},
		{
			name:      "message only",
			errorType: ErrTypeUnknown,
			message:   "Some error occurred",
			fieldName: "",
			expected:  "Some error occurred",
		},
		{
			name:      "whitespace handling",
			errorType: ErrTypeRequired,
			message:   "  Field is required  ",
			fieldName: "  email  ",
			expected:  "[required] email: Field is required",
		},
		{
			name:      "range error type",
			errorType: ErrTypeRange,
			message:   "Value out of range",
			fieldName: "age",
			expected:  "[range] age: Value out of range",
		},
		{
			name:      "type error type",
			errorType: ErrTypeType,
			message:   "Wrong type",
			fieldName: "count",
			expected:  "[type] count: Wrong type",
		},
		{
			name:      "length error type",
			errorType: ErrTypeLength,
			message:   "Too short",
			fieldName: "password",
			expected:  "[length] password: Too short",
		},
		{
			name:      "value error type",
			errorType: ErrTypeValue,
			message:   "Invalid value",
			fieldName: "status",
			expected:  "[value] status: Invalid value",
		},
		{
			name:      "duplicate error type",
			errorType: ErrTypeDuplicate,
			message:   "Already exists",
			fieldName: "email",
			expected:  "[duplicate] email: Already exists",
		},
		{
			name:      "conflict error type",
			errorType: ErrTypeConflict,
			message:   "Conflicting values",
			fieldName: "dates",
			expected:  "[conflict] dates: Conflicting values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorMessage(tt.errorType, tt.message, tt.fieldName)
			if result != tt.expected {
				t.Errorf("FormatErrorMessage() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatErrorMessageString(t *testing.T) {
	// Test the backward-compatible wrapper that accepts strings
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		expected  string
	}{
		{
			name:      "with all components",
			errorType: "required",
			message:   "Field is required",
			fieldName: "email",
			expected:  "[required] email: Field is required",
		},
		{
			name:      "without field name",
			errorType: "format",
			message:   "Invalid email format",
			fieldName: "",
			expected:  "[format] Invalid email format",
		},
		{
			name:      "message only",
			errorType: "unknown",
			message:   "Some error occurred",
			fieldName: "",
			expected:  "Some error occurred",
		},
		{
			name:      "whitespace handling",
			errorType: "  required  ",
			message:   "  Field is required  ",
			fieldName: "  email  ",
			expected:  "[required] email: Field is required",
		},
		{
			name:      "range error type",
			errorType: "range",
			message:   "Value out of range",
			fieldName: "age",
			expected:  "[range] age: Value out of range",
		},
		{
			name:      "type error type",
			errorType: "type",
			message:   "Wrong type",
			fieldName: "count",
			expected:  "[type] count: Wrong type",
		},
		{
			name:      "unknown error type",
			errorType: "custom_error",
			message:   "Custom error",
			fieldName: "field",
			expected:  "field: Custom error",
		},
		{
			name:      "empty error type",
			errorType: "",
			message:   "Some error occurred",
			fieldName: "email",
			expected:  "email: Some error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorMessageString(tt.errorType, tt.message, tt.fieldName)
			if result != tt.expected {
				t.Errorf("FormatErrorMessageString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatErrorMessageEnumHandling(t *testing.T) {
	// Comprehensive enum handling tests
	t.Run("all ErrorType enum values work correctly", func(t *testing.T) {
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
			result := FormatErrorMessage(et, "Test message", "field")

			// All valid types except ErrTypeUnknown should include error type in brackets
			if et != ErrTypeUnknown {
				if !strings.Contains(result, fmt.Sprintf("[%s]", et.String())) {
					t.Errorf("ErrorType %v should produce output with error type in brackets, got %v", et, result)
				}
			} else {
				// ErrTypeUnknown should not include error type in brackets
				if strings.Contains(result, "[unknown]") {
					t.Errorf("ErrTypeUnknown should not produce output with error type in brackets, got %v", result)
				}
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

	t.Run("enum and string wrapper produce consistent results", func(t *testing.T) {
		message := "Field is required"
		fieldName := "email"

		// Test with ErrorType enum
		enumResult := FormatErrorMessage(ErrTypeRequired, message, fieldName)

		// Test with string wrapper
		stringResult := FormatErrorMessageString("required", message, fieldName)

		if enumResult != stringResult {
			t.Errorf("Enum and string wrapper should produce same result:\n  Enum:   %v\n  String: %v", enumResult, stringResult)
		}
	})

	t.Run("string wrapper converts all valid types correctly", func(t *testing.T) {
		typeMappings := []struct {
			str      string
			enum     ErrorType
			message  string
			field    string
		}{
			{"required", ErrTypeRequired, "Required", "field1"},
			{"format", ErrTypeFormat, "Invalid format", "field2"},
			{"range", ErrTypeRange, "Out of range", "field3"},
			{"length", ErrTypeLength, "Too long", "field4"},
			{"type", ErrTypeType, "Wrong type", "field5"},
			{"value", ErrTypeValue, "Invalid value", "field6"},
			{"duplicate", ErrTypeDuplicate, "Duplicate", "field7"},
			{"conflict", ErrTypeConflict, "Conflict", "field8"},
			{"unknown", ErrTypeUnknown, "Unknown error", "field9"},
		}

		for _, tm := range typeMappings {
			enumResult := FormatErrorMessage(tm.enum, tm.message, tm.field)
			stringResult := FormatErrorMessageString(tm.str, tm.message, tm.field)

			if enumResult != stringResult {
				t.Errorf("Type mapping mismatch for %s:\n  Enum:   %v\n  String: %v", tm.str, enumResult, stringResult)
			}
		}
	})

	t.Run("ErrTypeUnknown special handling", func(t *testing.T) {
		// ErrTypeUnknown should not include error type in brackets
		result := FormatErrorMessage(ErrTypeUnknown, "Test message", "field")

		if strings.Contains(result, "[unknown]") {
			t.Errorf("ErrTypeUnknown should not include error type in brackets, got: %v", result)
		}

		// But should still include field and message
		if !strings.Contains(result, "field") {
			t.Errorf("Result should contain field name, got: %v", result)
		}
		if !strings.Contains(result, "Test message") {
			t.Errorf("Result should contain message, got: %v", result)
		}

		expected := "field: Test message"
		if result != expected {
			t.Errorf("ErrTypeUnknown handling incorrect, got: %v, want: %v", result, expected)
		}
	})

	t.Run("case-insensitive string conversion", func(t *testing.T) {
		// Test case-insensitive conversion in string wrapper
		lowercaseResult := FormatErrorMessageString("required", "Test", "field")
		uppercaseResult := FormatErrorMessageString("REQUIRED", "Test", "field")
		mixedcaseResult := FormatErrorMessageString("ReQuIrEd", "Test", "field")

		// All should produce the same result
		if lowercaseResult != uppercaseResult {
			t.Errorf("Case-insensitive conversion failed: 'required' vs 'REQUIRED'\n  %v\n  %v", lowercaseResult, uppercaseResult)
		}
		if lowercaseResult != mixedcaseResult {
			t.Errorf("Case-insensitive conversion failed: 'required' vs 'ReQuIrEd'\n  %v\n  %v", lowercaseResult, mixedcaseResult)
		}

		expected := "[required] field: Test"
		if lowercaseResult != expected {
			t.Errorf("Case-insensitive result incorrect, got: %v, want: %v", lowercaseResult, expected)
		}
	})

	t.Run("invalid string types convert to ErrTypeUnknown", func(t *testing.T) {
		invalidTypes := []string{"invalid_type", "custom_error", "random_type", ""}

		for _, invalidType := range invalidTypes {
			result := FormatErrorMessageString(invalidType, "Test message", "field")

			// Should not include error type in brackets for invalid types
			if strings.Contains(result, "[invalid_type]") || strings.Contains(result, "[custom_error]") {
				t.Errorf("Invalid type '%s' should not appear in brackets, got: %v", invalidType, result)
			}

			// But should still include field and message
			if !strings.Contains(result, "field") {
				t.Errorf("Result should contain field name for invalid type '%s', got: %v", invalidType, result)
			}
			if !strings.Contains(result, "Test message") {
				t.Errorf("Result should contain message for invalid type '%s', got: %v", invalidType, result)
			}
		}
	})

	t.Run("empty field and message handling with enum", func(t *testing.T) {
		// Test empty field name
		result1 := FormatErrorMessage(ErrTypeRequired, "Test message", "")
		if !strings.Contains(result1, "[required]") {
			t.Errorf("Empty field name should still include error type, got: %v", result1)
		}
		if !strings.Contains(result1, "Test message") {
			t.Errorf("Empty field name should still include message, got: %v", result1)
		}

		// Test with only error type (empty message and field)
		result2 := FormatErrorMessage(ErrTypeFormat, "", "")
		// Should just be empty since message is empty
		if result2 != "" {
			t.Errorf("Empty message and field should produce empty result, got: %v", result2)
		}
	})
}

func TestFormatErrorWithValues(t *testing.T) {
	tests := []struct {
		name          string
		fieldName     string
		expected      interface{}
		actual        interface{}
		customMessage string
		expectedMsg   string
	}{
		{
			name:          "with field name",
			fieldName:     "status_code",
			expected:      200,
			actual:        404,
			customMessage: "",
			expectedMsg:   "status_code: expected 200, got 404",
		},
		{
			name:          "with custom message",
			fieldName:     "count",
			expected:      5,
			actual:        3,
			customMessage: "Not enough items",
			expectedMsg:   "count: Not enough items (expected 5, got 3)",
		},
		{
			name:          "without field name",
			fieldName:     "",
			expected:      100,
			actual:        50,
			customMessage: "",
			expectedMsg:   "expected 100, got 50",
		},
		{
			name:          "string values",
			fieldName:     "type",
			expected:      "number",
			actual:        "string",
			customMessage: "",
			expectedMsg:   "type: expected number, got string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithValues(tt.fieldName, tt.expected, tt.actual, tt.customMessage)
			if result != tt.expectedMsg {
				t.Errorf("FormatErrorWithValues() = %v, want %v", result, tt.expectedMsg)
			}
		})
	}
}

func TestFormatErrorWithRange(t *testing.T) {
	tests := []struct {
		name        string
		fieldName   string
		minValue   interface{}
		maxValue   interface{}
		actualValue interface{}
		expected    string
	}{
		{
			name:        "with field name",
			fieldName:   "age",
			minValue:    0,
			maxValue:    120,
			actualValue: 150,
			expected:    "age: value 150 is outside valid range [0, 120]",
		},
		{
			name:        "without field name",
			fieldName:   "",
			minValue:    1,
			maxValue:    10,
			actualValue: 15,
			expected:    "value 15 is outside valid range [1, 10]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithRange(tt.fieldName, tt.minValue, tt.maxValue, tt.actualValue)
			if result != tt.expected {
				t.Errorf("FormatErrorWithRange() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatErrorWithPattern(t *testing.T) {
	tests := []struct {
		name        string
		fieldName   string
		pattern     string
		actualValue interface{}
		expected    string
	}{
		{
			name:        "with field name",
			fieldName:   "email",
			pattern:     "must contain @",
			actualValue: "invalidemail",
			expected:    "email: value 'invalidemail' does not match pattern (must contain @)",
		},
		{
			name:        "without field name",
			fieldName:   "",
			pattern:     "^[a-z]+$",
			actualValue: "ABC123",
			expected:    "value 'ABC123' does not match pattern (^[a-z]+$)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithPattern(tt.fieldName, tt.pattern, tt.actualValue)
			if result != tt.expected {
				t.Errorf("FormatErrorWithPattern() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// ERROR TYPE FORMATTING TESTS
// =============================================================================

func TestFormatErrorType(t *testing.T) {
	tests := []struct {
		name     string
		errorType ErrorType
		expected string
	}{
		{
			name:     "required",
			errorType: ErrTypeRequired,
			expected: "Required Field",
		},
		{
			name:     "format",
			errorType: ErrTypeFormat,
			expected: "Format Error",
		},
		{
			name:     "range",
			errorType: ErrTypeRange,
			expected: "Range Error",
		},
		{
			name:     "length",
			errorType: ErrTypeLength,
			expected: "Length Error",
		},
		{
			name:     "type",
			errorType: ErrTypeType,
			expected: "Type Error",
		},
		{
			name:     "value",
			errorType: ErrTypeValue,
			expected: "Invalid Value",
		},
		{
			name:     "duplicate",
			errorType: ErrTypeDuplicate,
			expected: "Duplicate Value",
		},
		{
			name:     "conflict",
			errorType: ErrTypeConflict,
			expected: "Conflict",
		},
		{
			name:     "unknown",
			errorType: ErrTypeUnknown,
			expected: "Unknown Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorType(tt.errorType)
			if result != tt.expected {
				t.Errorf("FormatErrorType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatErrorTypeFrom(t *testing.T) {
	tests := []struct {
		name        string
		errorTypeStr string
		expected    string
	}{
		{
			name:        "required",
			errorTypeStr: "required",
			expected:    "Required Field",
		},
		{
			name:        "format",
			errorTypeStr: "format",
			expected:    "Format Error",
		},
		{
			name:        "unknown type",
			errorTypeStr: "unknown_type",
			expected:    "Unknown Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorTypeFrom(tt.errorTypeStr)
			if result != tt.expected {
				t.Errorf("FormatErrorTypeFrom() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// CATEGORY FORMATTING TESTS
// =============================================================================

func TestFormatCategory(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		expected string
	}{
		{
			name:     "HTTP",
			category: CategoryHTTP,
			expected: "HTTP Protocol",
		},
		{
			name:     "Content",
			category: CategoryContent,
			expected: "Content",
		},
		{
			name:     "Validation",
			category: CategoryValidation,
			expected: "Data Validation",
		},
		{
			name:     "Performance",
			category: CategoryPerformance,
			expected: "Performance",
		},
		{
			name:     "Security",
			category: CategorySecurity,
			expected: "Security",
		},
		{
			name:     "Custom",
			category: CategoryCustom,
			expected: "Custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCategory(tt.category)
			if result != tt.expected {
				t.Errorf("FormatCategory() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// SEVERITY FORMATTING TESTS
// =============================================================================

func TestFormatSeverity(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		expected string
	}{
		{
			name:     "critical",
			severity: SeverityCritical,
			expected: "CRITICAL",
		},
		{
			name:     "high",
			severity: SeverityHigh,
			expected: "High",
		},
		{
			name:     "medium",
			severity: SeverityMedium,
			expected: "Medium",
		},
		{
			name:     "low",
			severity: SeverityLow,
			expected: "Low",
		},
		{
			name:     "info",
			severity: SeverityInfo,
			expected: "Info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSeverity(tt.severity)
			if result != tt.expected {
				t.Errorf("FormatSeverity() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatSeverityWithIndicator(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		expected string
	}{
		{
			name:     "critical",
			severity: SeverityCritical,
			expected: "[🚨] CRITICAL",
		},
		{
			name:     "high",
			severity: SeverityHigh,
			expected: "[⚠️] High",
		},
		{
			name:     "medium",
			severity: SeverityMedium,
			expected: "[⚡] Medium",
		},
		{
			name:     "low",
			severity: SeverityLow,
			expected: "[ℹ️] Low",
		},
		{
			name:     "info",
			severity: SeverityInfo,
			expected: "[💡] Info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSeverityWithIndicator(tt.severity)
			if result != tt.expected {
				t.Errorf("FormatSeverityWithIndicator() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// COMPREHENSIVE ERROR FORMATTING TESTS
// =============================================================================

func TestFormatValidationErrorFull(t *testing.T) {
	tests := []struct {
		name            string
		err             ValidationError
		includeSeverity bool
		expected        string
	}{
		{
			name: "with all fields and severity",
			err: ValidationError{
				ErrorType: string(ErrTypeRequired),
				Message:   "Field is required",
				FieldName: "email",
				Expected:  "value",
				Actual:    nil,
				Location:  "line 5",
			},
			includeSeverity: true,
			expected:        "[⚠️] High [required] email: Field is required (expected: value) [line 5]",
		},
		{
			name: "with severity",
			err: ValidationError{
				ErrorType: string(ErrTypeRequired),
				Message:   "Field is required",
				FieldName: "email",
			},
			includeSeverity: true,
			expected:        "[⚠️] High [required] email: Field is required",
		},
		{
			name: "without severity",
			err: ValidationError{
				ErrorType: string(ErrTypeFormat),
				Message:   "Invalid format",
				FieldName: "email",
			},
			includeSeverity: false,
			expected:        "[format] email: Invalid format",
		},
		{
			name: "with expected and actual",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "Wrong status code",
				FieldName: "response",
				Expected:  200,
				Actual:    404,
			},
			includeSeverity: true,
			expected:        "[⚠️] High [status_code] response: Wrong status code (expected: 200, actual: 404)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorFull(tt.err, tt.includeSeverity, nil)
			if result != tt.expected {
				t.Errorf("FormatValidationErrorFull() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatValidationErrorBrief(t *testing.T) {
	tests := []struct {
		name     string
		err      ValidationError
		expected string
	}{
		{
			name: "with field name",
			err: ValidationError{
				ErrorType: string(ErrTypeRequired),
				Message:   "Field is required",
				FieldName: "email",
			},
			expected: "email: required",
		},
		{
			name: "without field name",
			err: ValidationError{
				ErrorType: string(ErrTypeFormat),
				Message:   "Invalid format",
			},
			expected: "format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorBrief(tt.err)
			if result != tt.expected {
				t.Errorf("FormatValidationErrorBrief() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// ERROR LIST FORMATTING TESTS
// =============================================================================

func TestFormatErrorList(t *testing.T) {
	tests := []struct {
		name            string
		errors          []ValidationError
		includeSeverity bool
		contains        []string
	}{
		{
			name:            "empty list",
			errors:          []ValidationError{},
			includeSeverity: true,
			contains:        []string{"No errors"},
		},
		{
			name: "multiple errors with severity",
			errors: []ValidationError{
				{
					ErrorType: string(ErrTypeRequired),
					Message:   "Field is required",
					FieldName: "email",
				},
				{
					ErrorType: string(ErrTypeFormat),
					Message:   "Invalid format",
					FieldName: "password",
				},
			},
			includeSeverity: true,
			contains: []string{
				"1.",
				"2.",
				"[required] email",
				"[format] password",
				"High",
			},
		},
		{
			name: "multiple errors without severity",
			errors: []ValidationError{
				{
					ErrorType: string(ErrTypeRequired),
					Message:   "Field is required",
					FieldName: "email",
				},
			},
			includeSeverity: false,
			contains: []string{
				"1.",
				"[required] email",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorList(tt.errors, tt.includeSeverity)
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("FormatErrorList() expected to contain %v, got %v", expected, result)
				}
			}
		})
	}
}

func TestFormatErrorListSummary(t *testing.T) {
	tests := []struct {
		name     string
		errors   []ValidationError
		expected string
	}{
		{
			name:     "empty list",
			errors:   []ValidationError{},
			expected: "No errors",
		},
		{
			name: "multiple errors of same type",
			errors: []ValidationError{
				{ErrorType: string(ErrTypeRequired), FieldName: "email"},
				{ErrorType: string(ErrTypeRequired), FieldName: "password"},
			},
			expected: "2 error(s): required (2)",
		},
		{
			name: "multiple errors of different types",
			errors: []ValidationError{
				{ErrorType: string(ErrTypeRequired), FieldName: "email"},
				{ErrorType: string(ErrTypeFormat), FieldName: "password"},
			},
			expected: "2 error(s):",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorListSummary(tt.errors)
			if !strings.Contains(result, tt.expected) && tt.expected != "No errors" {
				// For specific checks, verify key parts
				if !strings.Contains(result, "error(s)") {
					t.Errorf("FormatErrorListSummary() expected to contain 'error(s)', got %v", result)
				}
			}
			if tt.expected == "No errors" && result != tt.expected {
				t.Errorf("FormatErrorListSummary() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestQuoteValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "normal value",
			value:    "email",
			expected: "'email'",
		},
		{
			name:     "empty value",
			value:    "",
			expected: "(empty)",
		},
		{
			name:     "whitespace",
			value:    "  ",
			expected: "'  '",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := QuoteValue(tt.value)
			if result != tt.expected {
				t.Errorf("QuoteValue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		maxLength int
		expected  string
	}{
		{
			name:      "short value",
			value:     "short",
			maxLength: 10,
			expected:  "short",
		},
		{
			name:      "exact length",
			value:     "exact",
			maxLength: 5,
			expected:  "exact",
		},
		{
			name:      "needs truncation",
			value:     "very long string here",
			maxLength: 10,
			expected:  "very lo...",
		},
		{
			name:      "minimum length",
			value:     "abcdefghijk",
			maxLength: 3,
			expected:  "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.value, tt.maxLength)
			if result != tt.expected {
				t.Errorf("TruncateString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

func TestFormattingIntegration(t *testing.T) {
	// Test that formatting functions work well together
	err := ValidationError{
		ErrorType: string(ErrTypeRequired),
		Message:   "Field is required",
		FieldName: "email",
		Expected:  "value",
		Actual:    nil,
		Location:  "line 5",
	}

	// Format with severity
	fullMsg := FormatValidationErrorFull(err, true, nil)
	if !strings.Contains(fullMsg, "High") {
		t.Error("Expected severity in formatted error")
	}
	if !strings.Contains(fullMsg, "required") {
		t.Error("Expected error type in formatted error")
	}
	if !strings.Contains(fullMsg, "email") {
		t.Error("Expected field name in formatted error")
	}

	// Format brief
	briefMsg := FormatValidationErrorBrief(err)
	if !strings.Contains(briefMsg, "email") {
		t.Error("Expected field name in brief error")
	}
	if !strings.Contains(briefMsg, "required") {
		t.Error("Expected error type in brief error")
	}
}

// =============================================================================
// FORMATERROR FUNCTION TESTS (from format_helper.go)
// =============================================================================

func TestFormatError(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "required error with field",
			errorType: "required",
			message:   "Field is required",
			fieldName: "email",
			want:      "[required] email: Field is required",
		},
		{
			name:      "format error without field",
			errorType: "format",
			message:   "Invalid email format",
			fieldName: "",
			want:      "[format] Invalid email format",
		},
		{
			name:      "range error with field",
			errorType: "range",
			message:   "Value out of range",
			fieldName: "age",
			want:      "[range] age: Value out of range",
		},
		{
			name:      "type error",
			errorType: "type",
			message:   "Wrong type",
			fieldName: "count",
			want:      "[type] count: Wrong type",
		},
		{
			name:      "empty error type uses default",
			errorType: "",
			message:   "Something went wrong",
			fieldName: "",
			want:      "[error] Something went wrong",
		},
		{
			name:      "empty message uses fallback with field",
			errorType: "validation",
			message:   "",
			fieldName: "email",
			want:      "[validation] email: email validation failed",
		},
		{
			name:      "empty message uses fallback without field",
			errorType: "",
			message:   "",
			fieldName: "",
			want:      "[error] (no message provided)",
		},
		{
			name:      "status_code error",
			errorType: "status_code",
			message:   "Expected 200 but got 404",
			fieldName: "response",
			want:      "[status_code] response: Expected 200 but got 404",
		},
		{
			name:      "unknown error type",
			errorType: "custom_type",
			message:   "Custom error message",
			fieldName: "field",
			want:      "[custom_type] field: Custom error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatErrorString(tt.errorType, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatErrorBackwardCompatibility(t *testing.T) {
	// Test that string-based FormatError produces consistent results
	// with ErrorType-based FormatErrorWithType
	t.Run("string and ErrorType functions produce consistent results", func(t *testing.T) {
		message := "Field is required"
		fieldName := "email"

		// String-based function
		stringResult := FormatErrorString("required", message, fieldName)

		// ErrorType-based function
		typeResult := FormatError(ErrTypeRequired, message, fieldName)

		if stringResult != typeResult {
			t.Errorf("String and ErrorType functions should produce same result:\n  String: %v\n  Type:   %v", stringResult, typeResult)
		}
	})

	t.Run("format error consistency", func(t *testing.T) {
		message := "Invalid format"
		fieldName := "email"

		stringResult := FormatErrorString("format", message, fieldName)
		typeResult := FormatError(ErrTypeFormat, message, fieldName)

		if stringResult != typeResult {
			t.Errorf("Format inconsistency:\n  String: %v\n  Type:   %v", stringResult, typeResult)
		}
	})

	t.Run("range error consistency", func(t *testing.T) {
		message := "Value out of range"
		fieldName := "age"

		stringResult := FormatErrorString("range", message, fieldName)
		typeResult := FormatError(ErrTypeRange, message, fieldName)

		if stringResult != typeResult {
			t.Errorf("Format inconsistency:\n  String: %v\n  Type:   %v", stringResult, typeResult)
		}
	})
}

func TestFormatErrorConsistentClassification(t *testing.T) {
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
			result := FormatErrorString(etStr, "Test message", "field")

			// Check that result contains error type
			if !strings.Contains(result, etStr) {
				t.Errorf("Error type %v should produce output containing error type, got %v", etStr, result)
			}
		}
	})
}


// =============================================================================
// ERROR CONTEXT FORMATTING TESTS
// =============================================================================

func TestFormatValidationErrorFullWithContext(t *testing.T) {
	tests := []struct {
		name            string
		err             ValidationError
		includeSeverity bool
		context         *ValidationErrorContext
		expected        string
	}{
		{
			name: "with full context (location and related fields)",
			err: ValidationError{
				ErrorType: string(ErrTypeRequired),
				Message:   "Field is required",
				FieldName: "email",
			},
			includeSeverity: true,
			context: func() *ValidationErrorContext {
				ctx := ValidationErrorContext{Location: "line 5"}
				ctx.RelatedFields = []string{"email_confirmation", "user.email"}
				return &ctx
			}(),
			expected: "[⚠️] High [required] email: Field is required (location: line 5, related fields: email_confirmation and user.email)",
		},
		{
			name: "with location only",
			err: ValidationError{
				ErrorType: string(ErrTypeFormat),
				Message:   "Invalid format",
				FieldName: "email",
			},
			includeSeverity: true,
			context: func() *ValidationErrorContext {
				ctx := ValidationErrorContext{Location: "field 'user.email' in line 42"}
				return &ctx
			}(),
			expected: "[⚡] Medium [format] email: Invalid format (location: field 'user.email' in line 42)",
		},
		{
			name: "with related fields only",
			err: ValidationError{
				ErrorType: string(ErrTypeRequired),
				Message:   "Field is required",
				FieldName: "access_token",
			},
			includeSeverity: true,
			context: func() *ValidationErrorContext {
				ctx := ValidationErrorContext{RelatedFields: []string{"refresh_token", "client_id"}}
				return &ctx
			}(),
			expected: "[⚠️] High [required] access_token: Field is required (related fields: refresh_token and client_id)",
		},
		{
			name: "with nil context",
			err: ValidationError{
				ErrorType: string(ErrTypeRequired),
				Message:   "Field is required",
				FieldName: "email",
			},
			includeSeverity: true,
			context:         nil,
			expected:        "[⚠️] High [required] email: Field is required",
		},
		{
			name: "with empty context",
			err: ValidationError{
				ErrorType: string(ErrTypeFormat),
				Message:   "Invalid format",
				FieldName: "password",
			},
			includeSeverity: false,
			context: func() *ValidationErrorContext {
				ctx := ValidationErrorContext{}
				return &ctx
			}(),
			expected: "[format] password: Invalid format",
		},
		{
			name: "with expected/actual values and context",
			err: ValidationError{
				ErrorType: string(ErrTypeRange),
				Message:   "Value out of range",
				FieldName: "age",
				Expected:  0,
				Actual:    150,
			},
			includeSeverity: true,
			context: func() *ValidationErrorContext {
				ctx := ValidationErrorContext{Location: "position 123"}
				return &ctx
			}(),
			expected: "[⚡] Medium [range] age: Value out of range (expected: 0, actual: 150) (location: position 123)",
		},
		{
			name: "with error location and context location",
			err: ValidationError{
				ErrorType: string(ErrTypeRequired),
				Message:   "Field is required",
				FieldName: "email",
				Location:  "header 'Authorization'",
			},
			includeSeverity: true,
			context: func() *ValidationErrorContext {
				ctx := ValidationErrorContext{Location: "line 5"}
				return &ctx
			}(),
			expected: "[⚠️] High [required] email: Field is required [header 'Authorization'] (location: line 5)",
		},
		{
			name: "with multiple related fields",
			err: ValidationError{
				ErrorType: string(ErrTypeValue),
				Message:   "Invalid value",
				FieldName: "status",
			},
			includeSeverity: false,
			context: func() *ValidationErrorContext {
				ctx := ValidationErrorContext{RelatedFields: []string{"status_code", "http_status", "response.status"}}
				return &ctx
			}(),
			expected: "[value] status: Invalid value (related fields: status_code, http_status, and response.status)",
		},
		{
			name: "with single related field",
			err: ValidationError{
				ErrorType: string(ErrTypeDuplicate),
				Message:   "Duplicate value",
				FieldName: "email",
			},
			includeSeverity: false,
			context: func() *ValidationErrorContext {
				ctx := ValidationErrorContext{RelatedFields: []string{"email"}}
				return &ctx
			}(),
			expected: "[duplicate] email: Duplicate value (related fields: email)",
		},
		{
			name: "without severity and with context",
			err: ValidationError{
				ErrorType: string(ErrTypeFormat),
				Message:   "Invalid format",
				FieldName: "url",
			},
			includeSeverity: false,
			context: func() *ValidationErrorContext {
				ctx := ValidationErrorContext{Location: "line 10"}
				ctx.RelatedFields = []string{"endpoint", "api_url"}
				return &ctx
			}(),
			expected: "[format] url: Invalid format (location: line 10, related fields: endpoint and api_url)",
		},
		{
			name: "with no field name but with context",
			err: ValidationError{
				ErrorType: string(ErrTypeRequired),
				Message:   "Required field missing",
			},
			includeSeverity: false,
			context: func() *ValidationErrorContext {
				ctx := ValidationErrorContext{Location: "request body"}
				return &ctx
			}(),
			expected: "[required] Required field missing (location: request body)",
		},
		{
			name:            "complex error with all fields and full context",
			err: ValidationError{
				ErrorType:  string(ErrTypeRange),
				Message:    "Age validation failed",
				FieldName:  "user.age",
				Expected:   0,
				Actual:     150,
				Location:   "JSON path: $.user.age",
			},
			includeSeverity: true,
			context: func() *ValidationErrorContext {
				ctx := ValidationErrorContext{Location: "line 15, column 8"}
				ctx.RelatedFields = []string{"user.min_age", "user.max_age"}
				return &ctx
			}(),
			expected: "[⚡] Medium [range] user.age: Age validation failed (expected: 0, actual: 150) [JSON path: $.user.age] (location: line 15, column 8, related fields: user.min_age and user.max_age)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorFull(tt.err, tt.includeSeverity, tt.context)
			if result != tt.expected {
				t.Errorf("FormatValidationErrorFull() =\n  got: %v\n  want: %v", result, tt.expected)
			}
		})
	}
}

func TestFormatValidationErrorToStringBackwardCompatibility(t *testing.T) {
	// Test that the alias function works correctly
	err := ValidationError{
		ErrorType: string(ErrTypeRequired),
		Message:   "Field is required",
		FieldName: "email",
	}

	result := FormatValidationErrorToString(err, true)
	expected := "[⚠️] High [required] email: Field is required"

	if result != expected {
		t.Errorf("FormatValidationErrorToString() = %v, want %v", result, expected)
	}
}

func TestFormatValidationErrorWithContext(t *testing.T) {
	// Test the convenience function
	err := ValidationError{
		ErrorType: string(ErrTypeFormat),
		Message:   "Invalid format",
		FieldName: "email",
	}

	ctx := ValidationErrorContext{Location: "line 5"}
	result := FormatValidationErrorWithContext(err, true, &ctx)
	expected := "[⚡] Medium [format] email: Invalid format (location: line 5)"

	if result != expected {
		t.Errorf("FormatValidationErrorWithContext() = %v, want %v", result, expected)
	}
}

func TestFormatErrorListWithContext(t *testing.T) {
	tests := []struct {
		name            string
		errors          []ValidationError
		contexts        []*ValidationErrorContext
		includeSeverity bool
		expected        string
	}{
		{
			name: "with contexts for all errors",
			errors: []ValidationError{
				{
					ErrorType: string(ErrTypeRequired),
					Message:   "Field is required",
					FieldName: "email",
				},
				{
					ErrorType: string(ErrTypeFormat),
					Message:   "Invalid format",
					FieldName: "password",
				},
			},
			contexts: func() []*ValidationErrorContext {
				ctx1 := ValidationErrorContext{Location: "line 5"}
				ctx1.RelatedFields = []string{"email_confirmation"}
				ctx2 := ValidationErrorContext{Location: "line 10"}
				return []*ValidationErrorContext{&ctx1, &ctx2}
			}(),
			includeSeverity: true,
			expected: "1. [⚠️] High [required] email: Field is required (location: line 5, related fields: email_confirmation)\n2. [⚠️] High [format] password: Invalid format (location: line 10)",
		},
		{
			name: "with mixed nil and non-nil contexts",
			errors: []ValidationError{
				{
					ErrorType: string(ErrTypeRequired),
					Message:   "Field is required",
					FieldName: "email",
				},
				{
					ErrorType: string(ErrTypeFormat),
					Message:   "Invalid format",
					FieldName: "password",
				},
				{
					ErrorType: string(ErrTypeRange),
					Message:   "Out of range",
					FieldName: "age",
				},
			},
			contexts: func() []*ValidationErrorContext {
				ctx1 := ValidationErrorContext{Location: "line 5"}
				ctx3 := ValidationErrorContext{Location: "line 15"}
				return []*ValidationErrorContext{&ctx1, nil, &ctx3}
			}(),
			includeSeverity: false,
			expected: "1. [required] email: Field is required (location: line 5)\n2. [format] password: Invalid format\n3. [range] age: Out of range (location: line 15)",
		},
		{
			name: "with nil contexts slice",
			errors: []ValidationError{
				{
					ErrorType: string(ErrTypeRequired),
					Message:   "Field is required",
					FieldName: "email",
				},
				{
					ErrorType: string(ErrTypeFormat),
					Message:   "Invalid format",
					FieldName: "password",
				},
			},
			contexts:        nil,
			includeSeverity: true,
			expected:        "1. [⚠️] High [required] email: Field is required\n2. [⚠️] High [format] password: Invalid format",
		},
		{
			name: "with fewer contexts than errors",
			errors: []ValidationError{
				{
					ErrorType: string(ErrTypeRequired),
					Message:   "Field is required",
					FieldName: "email",
				},
				{
					ErrorType: string(ErrTypeFormat),
					Message:   "Invalid format",
					FieldName: "password",
				},
				{
					ErrorType: string(ErrTypeRange),
					Message:   "Out of range",
					FieldName: "age",
				},
			},
			contexts: func() []*ValidationErrorContext {
				ctx1 := ValidationErrorContext{Location: "line 5"}
				return []*ValidationErrorContext{&ctx1}
			}(),
			includeSeverity: false,
			expected:        "1. [required] email: Field is required (location: line 5)\n2. [format] password: Invalid format\n3. [range] age: Out of range",
		},
		{
			name:            "with empty errors slice",
			errors:          []ValidationError{},
			contexts:        []*ValidationErrorContext{},
			includeSeverity: true,
			expected:        "No errors",
		},
		{
			name: "with empty contexts",
			errors: []ValidationError{
				{
					ErrorType: string(ErrTypeRequired),
					Message:   "Field is required",
					FieldName: "email",
				},
			},
			contexts: func() []*ValidationErrorContext {
				ctx := ValidationErrorContext{}
				return []*ValidationErrorContext{&ctx}
			}(),
			includeSeverity: true,
			expected:        "1. [⚠️] High [required] email: Field is required",
		},
		{
			name: "complex scenario with all context types",
			errors: []ValidationError{
				{
					ErrorType: string(ErrTypeRequired),
					Message:   "Field is required",
					FieldName: "email",
				},
				{
					ErrorType: string(ErrTypeFormat),
					Message:   "Invalid format",
					FieldName: "password",
					Expected:  "min 8 chars",
					Actual:    "abc",
				},
				{
					ErrorType: string(ErrTypeRange),
					Message:   "Out of range",
					FieldName: "age",
					Expected:  0,
					Actual:    150,
				},
			},
			contexts: func() []*ValidationErrorContext {
				ctx1 := ValidationErrorContext{Location: "line 5"}
				ctx1.RelatedFields = []string{"email_confirmation", "user.email"}
				ctx2 := ValidationErrorContext{Location: "line 10"}
				ctx3 := ValidationErrorContext{RelatedFields: []string{"min_age", "max_age"}}
				return []*ValidationErrorContext{&ctx1, &ctx2, &ctx3}
			}(),
			includeSeverity: true,
			expected: "1. [⚠️] High [required] email: Field is required (location: line 5, related fields: email_confirmation and user.email)\n2. [⚡] Medium [format] password: Invalid format (expected: min 8 chars, actual: abc) (location: line 10)\n3. [⚡] Medium [range] age: Out of range (expected: 0, actual: 150) (related fields: min_age and max_age)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorListWithContext(tt.errors, tt.contexts, tt.includeSeverity)
			if result != tt.expected {
				t.Errorf("FormatErrorListWithContext() =\n  got: %v\n  want: %v", result, tt.expected)
			}
		})
	}
}

func TestValidationErrorContextInFormatting(t *testing.T) {
	// Test that ValidationErrorContext methods work correctly with formatting
	t.Run("HasLocation", func(t *testing.T) {
		ctx := ValidationErrorContext{Location: "line 5"}
		if !ctx.HasLocation() {
			t.Error("Expected HasLocation to return true")
		}

		emptyCtx := ValidationErrorContext{}
		if emptyCtx.HasLocation() {
			t.Error("Expected HasLocation to return false for empty context")
		}
	})

	t.Run("HasRelatedFields", func(t *testing.T) {
		ctx := ValidationErrorContext{RelatedFields: []string{"field1", "field2"}}
		if !ctx.HasRelatedFields() {
			t.Error("Expected HasRelatedFields to return true")
		}

		emptyCtx := ValidationErrorContext{}
		if emptyCtx.HasRelatedFields() {
			t.Error("Expected HasRelatedFields to return false for empty context")
		}
	})

	t.Run("IsEmpty", func(t *testing.T) {
		ctx1 := ValidationErrorContext{Location: "line 5"}
		if ctx1.IsEmpty() {
			t.Error("Expected IsEmpty to return false for context with location")
		}

		ctx2 := ValidationErrorContext{RelatedFields: []string{"field1"}}
		if ctx2.IsEmpty() {
			t.Error("Expected IsEmpty to return false for context with related fields")
		}

		emptyCtx := ValidationErrorContext{}
		if !emptyCtx.IsEmpty() {
			t.Error("Expected IsEmpty to return true for empty context")
		}
	})

	t.Run("String representation", func(t *testing.T) {
		ctx := ValidationErrorContext{Location: "line 5"}
		ctx.RelatedFields = []string{"field1", "field2"}
		str := ctx.String()

		if !strings.Contains(str, "location: line 5") {
			t.Error("String representation should contain location")
		}
		if !strings.Contains(str, "related_fields:") {
			t.Error("String representation should contain related_fields")
		}
	})
}

// =============================================================================
// SUGGESTIONS FORMATTING TESTS
// =============================================================================

func TestFormatValidationErrorFull_Suggestions(t *testing.T) {
	tests := []struct {
		name              string
		err               ValidationError
		includeSeverity   bool
		context           *ValidationErrorContext
		wantSuggestions   bool
		suggestionStrings []string
	}{
		{
			name: "with single suggestion",
			err: ValidationError{
				ErrorType:   "required",
				Message:     "Field is required",
				FieldName:   "email",
				Suggestions: []string{"Provide a valid email address"},
			},
			includeSeverity:   true,
			context:           nil,
			wantSuggestions:   true,
			suggestionStrings: []string{"Provide a valid email address"},
		},
		{
			name: "with multiple suggestions",
			err: ValidationError{
				ErrorType:   "format",
				Message:     "Invalid email format",
				FieldName:   "email",
				Actual:      "invalid-email",
				Suggestions: []string{
					"Check the email format",
					"Verify the email contains @ symbol",
					"Ensure email domain is valid",
				},
			},
			includeSeverity:   true,
			context:           nil,
			wantSuggestions:   true,
			suggestionStrings: []string{
				"Check the email format",
				"Verify the email contains @ symbol",
				"Ensure email domain is valid",
			},
		},
		{
			name: "without suggestions",
			err: ValidationError{
				ErrorType: "required",
				Message:   "Field is required",
				FieldName: "email",
			},
			includeSeverity:   true,
			context:          nil,
			wantSuggestions:  false,
			suggestionStrings: nil,
		},
		{
			name: "with empty suggestions slice",
			err: ValidationError{
				ErrorType:    "required",
				Message:      "Field is required",
				FieldName:    "email",
				Suggestions:  []string{},
			},
			includeSeverity:   true,
			context:           nil,
			wantSuggestions:   false,
			suggestionStrings: nil,
		},
		{
			name: "with context and suggestions",
			err: ValidationError{
				ErrorType:   "format",
				Message:     "Invalid format",
				FieldName:   "email",
				Suggestions: []string{"Check email format"},
			},
			includeSeverity: true,
			context: func() *ValidationErrorContext {
				ctx := NewValidationErrorContext("line 5")
				ctx.WithRelatedFields([]string{"email_confirmation"})
				return &ctx
			}(),
			wantSuggestions:  true,
			suggestionStrings: []string{"Check email format"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorFull(tt.err, tt.includeSeverity, tt.context)

			if tt.wantSuggestions {
				// Should contain "Suggestions:" header
				if !strings.Contains(result, "Suggestions:") {
					t.Errorf("Expected 'Suggestions:' in output, got:\n%s", result)
				}

				// Should contain all suggestion strings
				for _, suggestion := range tt.suggestionStrings {
					if !strings.Contains(result, suggestion) {
						t.Errorf("Expected suggestion '%s' in output, got:\n%s", suggestion, result)
					}
				}

				// Should use bullet format "- "
				if !strings.Contains(result, "\n  - ") {
					t.Errorf("Expected bullet format '\\n  - ' in suggestions, got:\n%s", result)
				}
			} else {
				// Should NOT contain "Suggestions:" header
				if strings.Contains(result, "Suggestions:") {
					t.Errorf("Unexpected 'Suggestions:' in output when no suggestions provided, got:\n%s", result)
				}
			}
		})
	}
}

func TestFormatValidationErrorWithExpectedActual_Suggestions(t *testing.T) {
	tests := []struct {
		name              string
		err               ValidationError
		includeSeverity   bool
		context           *ValidationErrorContext
		expectedActual    ExpectedActual
		wantSuggestions   bool
		suggestionStrings []string
	}{
		{
			name: "with suggestions and ExpectedActual",
			err: ValidationError{
				ErrorType:   "status_code",
				Message:     "Status code mismatch",
				FieldName:   "response",
				Suggestions: []string{"Check API endpoint", "Verify authentication"},
			},
			includeSeverity: true,
			context:         nil,
			expectedActual:  NewExpectedActual(200, 404),
			wantSuggestions: true,
			suggestionStrings: []string{
				"Check API endpoint",
				"Verify authentication",
			},
		},
		{
			name: "without suggestions",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "Status code mismatch",
				FieldName: "response",
			},
			includeSeverity:   true,
			context:          nil,
			expectedActual:    NewExpectedActual(200, 404),
			wantSuggestions:  false,
			suggestionStrings: nil,
		},
		{
			name: "with context and suggestions",
			err: ValidationError{
				ErrorType:   "format",
				Message:     "Invalid format",
				FieldName:   "email",
				Suggestions: []string{"Verify format", "Check pattern"},
			},
			includeSeverity: true,
			context: func() *ValidationErrorContext {
				ctx := NewValidationErrorContext("line 10")
				return &ctx
			}(),
			expectedActual:  ExpectedActual{},
			wantSuggestions: true,
			suggestionStrings: []string{
				"Verify format",
				"Check pattern",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorWithExpectedActual(tt.err, tt.includeSeverity, tt.context, tt.expectedActual)

			if tt.wantSuggestions {
				// Should contain "Suggestions:" header
				if !strings.Contains(result, "Suggestions:") {
					t.Errorf("Expected 'Suggestions:' in output, got:\n%s", result)
				}

				// Should contain all suggestion strings
				for _, suggestion := range tt.suggestionStrings {
					if !strings.Contains(result, suggestion) {
						t.Errorf("Expected suggestion '%s' in output, got:\n%s", suggestion, result)
					}
				}

				// Should use bullet format "- "
				if !strings.Contains(result, "\n  - ") {
					t.Errorf("Expected bullet format '\\n  - ' in suggestions, got:\n%s", result)
				}
			} else {
				// Should NOT contain "Suggestions:" header
				if strings.Contains(result, "Suggestions:") {
					t.Errorf("Unexpected 'Suggestions:' in output when no suggestions provided, got:\n%s", result)
				}
			}
		})
	}
}

func TestSuggestionsFormatting_BulletList(t *testing.T) {
	tests := []struct {
		name              string
		suggestions      []string
		expectedContains []string
	}{
		{
			name:         "single suggestion",
			suggestions:  []string{"Check documentation"},
			expectedContains: []string{
				"Suggestions:",
				"\n  - Check documentation",
			},
		},
		{
			name:        "multiple suggestions",
			suggestions: []string{"First suggestion", "Second suggestion", "Third suggestion"},
			expectedContains: []string{
				"Suggestions:",
				"\n  - First suggestion",
				"\n  - Second suggestion",
				"\n  - Third suggestion",
			},
		},
		{
			name:              "no suggestions",
			suggestions:       nil,
			expectedContains:  []string{},
		},
		{
			name:              "empty suggestions",
			suggestions:       []string{},
			expectedContains:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidationError{
				ErrorType:   "test",
				Message:     "Test error",
				Suggestions: tt.suggestions,
			}

			result := FormatValidationErrorFull(err, false, nil)

			if len(tt.expectedContains) == 0 {
				// Should not contain Suggestions: section
				if strings.Contains(result, "Suggestions:") {
					t.Errorf("Expected no suggestions section, got:\n%s", result)
				}
			} else {
				// Should contain all expected strings
				for _, expected := range tt.expectedContains {
					if !strings.Contains(result, expected) {
						t.Errorf("Expected '%s' in result, got:\n%s", expected, result)
					}
				}
			}
		})
	}
}
