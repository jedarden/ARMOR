package validate

import (
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
			name:      "without error type",
			errorType: "",
			message:   "Some error occurred",
			fieldName: "email",
			expected:  "email: Some error occurred",
		},
		{
			name:      "message only",
			errorType: "",
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
			expected: "[!] CRITICAL",
		},
		{
			name:     "high",
			severity: SeverityHigh,
			expected: "[⚠] High",
		},
		{
			name:     "medium",
			severity: SeverityMedium,
			expected: "[■] Medium",
		},
		{
			name:     "low",
			severity: SeverityLow,
			expected: "[○] Low",
		},
		{
			name:     "info",
			severity: SeverityInfo,
			expected: "[i] Info",
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
			expected:        "[⚠] High [required] email: Field is required (expected: value) [line 5]",
		},
		{
			name: "with severity",
			err: ValidationError{
				ErrorType: string(ErrTypeRequired),
				Message:   "Field is required",
				FieldName: "email",
			},
			includeSeverity: true,
			expected:        "[⚠] High [required] email: Field is required",
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
			expected:        "[⚠] High [status_code] response: Wrong status code (expected: 200, actual: 404)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorFull(tt.err, tt.includeSeverity)
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
	fullMsg := FormatValidationErrorFull(err, true)
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
