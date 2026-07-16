package validate

import (
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// COMPREHENSIVE TESTS FOR ERROR FORMATTING FUNCTIONS
//
// This test suite covers edge cases, optional fields, and functions with
// incomplete test coverage to achieve >90% coverage for error formatting.
// =============================================================================

// =============================================================================
// FormatFieldLocation Tests (0% coverage before this test)
// =============================================================================

func TestFormatFieldLocation(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		location  string
		expected  string
	}{
		{
			name:      "field with location",
			fieldName: "email",
			location:  "line 5",
			expected:  "email at line 5",
		},
		{
			name:      "field without location",
			fieldName: "email",
			location:  "",
			expected:  "email",
		},
		{
			name:      "empty field name with location",
			fieldName: "",
			location:  "line 5",
			expected:  "(unknown field) at line 5",
		},
		{
			name:      "empty field name without location",
			fieldName: "",
			location:  "",
			expected:  "(unknown field)",
		},
		{
			name:      "whitespace handling",
			fieldName: "  email  ",
			location:  "  line 5  ",
			expected:  "email at line 5",
		},
		{
			name:      "location with position",
			fieldName: "user.email",
			location:  "position 123",
			expected:  "user.email at position 123",
		},
		{
			name:      "location with byte offset",
			fieldName: "data.items",
			location:  "byte offset 42",
			expected:  "data.items at byte offset 42",
		},
		{
			name:      "location with line and column",
			fieldName: "content",
			location:  "line 10, column 5",
			expected:  "content at line 10, column 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldLocation(tt.fieldName, tt.location)
			if result != tt.expected {
				t.Errorf("FormatFieldLocation(%q, %q) = %q, want %q",
					tt.fieldName, tt.location, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// TruncateValue Tests (0% coverage before this test)
// =============================================================================

func TestTruncateValue(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		maxLength int
		expected  string
	}{
		{
			name:      "short value",
			value:     "short",
			maxLength: 20,
			expected:  "short",
		},
		{
			name:      "exact length",
			value:     "exactlyten!",
			maxLength: 13,
			expected:  "exactlyten!",
		},
		{
			name:      "truncate needed",
			value:     "this is a very long string that needs truncation",
			maxLength: 20,
			expected:  "this is a very lo...",
		},
		{
			name:      "empty value",
			value:     "",
			maxLength: 20,
			expected:  "",
		},
		{
			name:      "minimum length",
			value:     "abc",
			maxLength: 3,
			expected:  "abc",
		},
		{
			name:      "below minimum - gets clamped to 3",
			value:     "abcd",
			maxLength: 2,
			expected:  "...",
		},
		{
			name:      "whitespace value",
			value:     "   ",
			maxLength: 10,
			expected:  "   ",
		},
		{
			name:      "special characters",
			value:     "hello\nworld\t!",
			maxLength: 15,
			expected:  "hello\nworld\t!",
		},
		{
			name:      "unicode characters",
			value:     "Hello 世界 🌍",
			maxLength: 15,
			expected:  "Hello 世界...",
		},
		{
			name:      "truncate at word boundary",
			value:     "one two three four five",
			maxLength: 18,
			expected:  "one two three f...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateValue(tt.value, tt.maxLength)
			if result != tt.expected {
				t.Errorf("TruncateValue(%q, %d) = %q, want %q",
					tt.value, tt.maxLength, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// TruncateString Edge Cases Tests (improve 80% coverage)
// =============================================================================

func TestTruncateString_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		maxLength int
		expected  string
	}{
		{
			name:      "truncate single character",
			value:     "abcdef",
			maxLength: 4,
			expected:  "a...",
		},
		{
			name:      "truncate to minimum",
			value:     "abcde",
			maxLength: 3,
			expected:  "...",
		},
		{
			name:      "below minimum gets clamped",
			value:     "abcd",
			maxLength: 1,
			expected:  "...",
		},
		{
			name:      "zero length gets clamped - string shorter than min",
			value:     "abc",
			maxLength: 0,
			expected:  "abc",
		},
		{
			name:      "negative length gets clamped - string shorter than min",
			value:     "abc",
			maxLength: -5,
			expected:  "abc",
		},
		{
			name:      "empty string",
			value:     "",
			maxLength: 10,
			expected:  "",
		},
		{
			name:      "exact length no truncation",
			value:     "abc",
			maxLength: 3,
			expected:  "abc",
		},
		{
			name:      "one less than truncation threshold",
			value:     "ab",
			maxLength: 3,
			expected:  "ab",
		},
		{
			name:      "at truncation threshold",
			value:     "abc",
			maxLength: 4,
			expected:  "abc",
		},
		{
			name:      "long string needs truncation",
			value:     strings.Repeat("a", 100),
			maxLength: 10,
			expected:  "aaaaaaa...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.value, tt.maxLength)
			if result != tt.expected {
				t.Errorf("TruncateString(%q, %d) = %q, want %q",
					tt.value, tt.maxLength, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatFieldRef Edge Cases (improve 75% coverage)
// =============================================================================

func TestFormatFieldRef_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		parent    string
		expected  string
	}{
		{
			name:      "parent that normalizes to unknown field",
			fieldName: "",
			parent:    "...",
			expected:  "...",
		},
		{
			name:      "parent with only whitespace",
			fieldName: "email",
			parent:    "   ",
			expected:  "email",
		},
		{
			name:      "field name normalizes to unknown field with parent",
			fieldName: "...",
			parent:    "request",
			expected:  "request",
		},
		{
			name:      "both normalize to unknown field",
			fieldName: "...",
			parent:    "...",
			expected:  "...",
		},
		{
			name:      "field name with dots only",
			fieldName: "...",
			parent:    "",
			expected:  "(unknown field)",
		},
		{
			name:      "complex array indices with empty components",
			fieldName: ".0.email",
			parent:    "response",
			expected:  "response.0.email",
		},
		{
			name:      "parent with trailing dot",
			fieldName: "email",
			parent:    "response.",
			expected:  "response.email",
		},
		{
			name:      "field name starting with dot",
			fieldName: ".email",
			parent:    "request",
			expected:  "request.email",
		},
		{
			name:      "numeric parent",
			fieldName: "email",
			parent:    "0",
			expected:  "0.email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldRef(tt.fieldName, tt.parent)
			if result != tt.expected {
				t.Errorf("FormatFieldRef(%q, %q) = %q, want %q",
					tt.fieldName, tt.parent, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatExpectedActual Edge Cases (improve 92.7% coverage)
// =============================================================================

func TestFormatExpectedActual_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		ea       ExpectedActual
		expected string
	}{
		{
			name:     "empty ExpectedActual",
			ea:       ExpectedActual{},
			expected: "",
		},
		{
			name:     "only expected value - string",
			ea:       NewExpectedActual("test@example.com", nil),
			expected: "expected: 'test@example.com'",
		},
		{
			name:     "only actual value - string",
			ea:       NewExpectedActual(nil, "actual_value"),
			expected: "actual: 'actual_value'",
		},
		{
			name:     "only expected value - int",
			ea:       NewExpectedActual(42, nil),
			expected: "expected: 42 (Unknown)",
		},
		{
			name:     "only actual value - int",
			ea:       NewExpectedActual(nil, 99),
			expected: "actual: 99 (Unknown)",
		},
		{
			name:     "expected and actual - both strings",
			ea:       NewExpectedActual("expected", "actual"),
			expected: "expected: 'expected', actual: 'actual'",
		},
		{
			name:     "long string gets truncated",
			ea:       NewExpectedActual(nil, strings.Repeat("a", 150)),
			expected: fmt.Sprintf("actual: '%s...'", strings.Repeat("a", 100)),
		},
		{
			name: "float64 precision",
			ea: NewExpectedActual(nil, 3.14159265359),
			expected: "actual: 3.14",
		},
		{
			name:     "nil interface values",
			ea:       ExpectedActual{Expected: nil, Actual: nil},
			expected: "",
		},
		{
			name:     "empty slice as expected",
			ea:       NewExpectedActual([]string{}, nil),
			expected: "expected: []",
		},
		{
			name:     "empty map as expected",
			ea:       NewExpectedActual(map[string]interface{}{}, nil),
			expected: "expected: {}",
		},
		{
			name: "large slice gets limited",
			ea: NewExpectedActual(nil, []interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}),
			expected: "actual: [1, 2, 3, 4, 5, ...]",
		},
		{
			name: "large map gets limited",
			ea: NewExpectedActual(nil, map[string]interface{}{
				"k1": "v1", "k2": "v2", "k3": "v3", "k4": "v4", "k5": "v5", "k6": "v6",
			}),
			expected: "actual: {k1: v1, k2: v2, k3: v3, k4: v4, k5: v5, ...}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatExpectedActual(tt.ea)
			if result != tt.expected {
				t.Errorf("FormatExpectedActual(%+v) = %q, want %q",
					tt.ea, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatExpectedActualInline Edge Cases (improve 90% coverage)
// =============================================================================

func TestFormatExpectedActualInline_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		ea       ExpectedActual
		expected string
	}{
		{
			name:     "empty ExpectedActual",
			ea:       ExpectedActual{},
			expected: "",
		},
		{
			name:     "only expected value",
			ea:       NewExpectedActual(200, nil),
			expected: "(expected 200)",
		},
		{
			name:     "only actual value",
			ea:       NewExpectedActual(nil, 404),
			expected: "(got 404)",
		},
		{
			name:     "both expected and actual",
			ea:       NewExpectedActual(200, 404),
			expected: "(expected 200, got 404)",
		},
		{
			name:     "string values",
			ea:       NewExpectedActual("required", "actual"),
			expected: "(expected required, got actual)",
		},
		{
			name:     "nil interface values",
			ea:       ExpectedActual{Expected: nil, Actual: nil},
			expected: "",
		},
		{
			name:     "complex types",
			ea:       NewExpectedActual([]int{1, 2, 3}, "error"),
			expected: "(expected [1 2 3], got error)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatExpectedActualInline(tt.ea)
			if result != tt.expected {
				t.Errorf("FormatExpectedActualInline(%+v) = %q, want %q",
					tt.ea, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// isNumericIndex Edge Cases (improve 80% coverage)
// =============================================================================

func TestIsNumericIndex_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "positive integer",
			input:    "123",
			expected: true,
		},
		{
			name:     "zero",
			input:    "0",
			expected: true,
		},
		{
			name:     "negative integer",
			input:    "-5",
			expected: true,
		},
		{
			name:     "negative sign only",
			input:    "-",
			expected: false,
		},
		{
			name:     "with leading zeros",
			input:    "007",
			expected: true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "with decimal point",
			input:    "12.5",
			expected: false,
		},
		{
			name:     "with letters",
			input:    "12a",
			expected: false,
		},
		{
			name:     "with spaces",
			input:    "12 5",
			expected: false,
		},
		{
			name:     "very large number",
			input:    "999999999999999999",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumericIndex(tt.input)
			if result != tt.expected {
				t.Errorf("isNumericIndex(%q) = %v, want %v",
					tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// normalizeFieldPath Edge Cases (improve 94.7% coverage)
// =============================================================================

func TestNormalizeFieldPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only dots",
			input:    "...",
			expected: "(unknown field)",
		},
		{
			name:     "dots with spaces",
			input:    ". . .",
			expected: "(unknown field)",
		},
		{
			name:     "numeric only",
			input:    "123",
			expected: "123",
		},
		{
			name:     "mixed numeric and text",
			input:    "users.123.email",
			expected: "users[123].email",
		},
		{
			name:     "trailing dot",
			input:    "field.",
			expected: "field",
		},
		{
			name:     "leading dot",
			input:    ".field",
			expected: "field",
		},
		{
			name:     "consecutive dots",
			input:    "field..name",
			expected: "field.name",
		},
		{
			name:     "array at start",
			input:    "0.field",
			expected: "0.field",
		},
		{
			name:     "negative index",
			input:    "field.-1.name",
			expected: "field[-1].name",
		},
		{
			name:     "already bracketed",
			input:    "items[5]",
			expected: "items[5]",
		},
		{
			name:     "bracketed with dot notation",
			input:    "users.0.items[5]",
			expected: "users[0].items[5]",
		},
		{
			name:     "spaces around parts",
			input:    " field . 0 . name ",
			expected: "field[0].name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeFieldPath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeFieldPath(%q) = %q, want %q",
					tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// QuoteValue Tests (ensure 100% coverage maintained)
// =============================================================================

func TestQuoteValue_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "non-empty string",
			value:    "email",
			expected: "'email'",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "(empty)",
		},
		{
			name:     "whitespace only",
			value:    "   ",
			expected: "'   '",
		},
		{
			name:     "string with quotes",
			value:    `"quoted"`,
			expected: `'"quoted"'`,
		},
		{
			name:     "string with newlines",
			value:    "line1\nline2",
			expected: "'line1\nline2'",
		},
		{
			name:     "unicode string",
			value:    "Hello 世界",
			expected: "'Hello 世界'",
		},
		{
			name:     "special characters",
			value:    "\t\r\n",
			expected: "'\t\r\n'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := QuoteValue(tt.value)
			if result != tt.expected {
				t.Errorf("QuoteValue(%q) = %q, want %q",
					tt.value, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatErrorType Edge Cases (improve 90.9% coverage)
// =============================================================================

func TestFormatErrorType_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		errorType ErrorType
		expected string
	}{
		{
			name:     "ErrTypeRequired",
			errorType: ErrTypeRequired,
			expected: "Required Field",
		},
		{
			name:     "ErrTypeFormat",
			errorType: ErrTypeFormat,
			expected: "Format Error",
		},
		{
			name:     "ErrTypeRange",
			errorType: ErrTypeRange,
			expected: "Range Error",
		},
		{
			name:     "ErrTypeLength",
			errorType: ErrTypeLength,
			expected: "Length Error",
		},
		{
			name:     "ErrTypeType",
			errorType: ErrTypeType,
			expected: "Type Error",
		},
		{
			name:     "ErrTypeValue",
			errorType: ErrTypeValue,
			expected: "Invalid Value",
		},
		{
			name:     "ErrTypeDuplicate",
			errorType: ErrTypeDuplicate,
			expected: "Duplicate Value",
		},
		{
			name:     "ErrTypeConflict",
			errorType: ErrTypeConflict,
			expected: "Conflict",
		},
		{
			name:     "ErrTypeUnknown",
			errorType: ErrTypeUnknown,
			expected: "Unknown Error",
		},
		{
			name:     "custom error type",
			errorType: ErrorType("custom_validation"),
			expected: "custom_validation",
		},
		{
			name:     "empty error type",
			errorType: ErrorType(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorType(tt.errorType)
			if result != tt.expected {
				t.Errorf("FormatErrorType(%v) = %q, want %q",
					tt.errorType, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatCategory Edge Cases (improve 87.5% coverage)
// =============================================================================

func TestFormatCategory_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		expected string
	}{
		{
			name:     "CategoryHTTP",
			category: CategoryHTTP,
			expected: "HTTP Protocol",
		},
		{
			name:     "CategoryContent",
			category: CategoryContent,
			expected: "Content",
		},
		{
			name:     "CategoryValidation",
			category: CategoryValidation,
			expected: "Data Validation",
		},
		{
			name:     "CategoryPerformance",
			category: CategoryPerformance,
			expected: "Performance",
		},
		{
			name:     "CategorySecurity",
			category: CategorySecurity,
			expected: "Security",
		},
		{
			name:     "CategoryCustom",
			category: CategoryCustom,
			expected: "Custom",
		},
		{
			name:     "custom category",
			category: ErrorCategory("backend_logic"),
			expected: "backend_logic",
		},
		{
			name:     "empty category",
			category: ErrorCategory(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCategory(tt.category)
			if result != tt.expected {
				t.Errorf("FormatCategory(%v) = %q, want %q",
					tt.category, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatSeverity Edge Cases (improve 85.7% coverage)
// =============================================================================

func TestFormatSeverity_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		expected string
	}{
		{
			name:     "SeverityCritical",
			severity: SeverityCritical,
			expected: "CRITICAL",
		},
		{
			name:     "SeverityHigh",
			severity: SeverityHigh,
			expected: "High",
		},
		{
			name:     "SeverityMedium",
			severity: SeverityMedium,
			expected: "Medium",
		},
		{
			name:     "SeverityLow",
			severity: SeverityLow,
			expected: "Low",
		},
		{
			name:     "SeverityInfo",
			severity: SeverityInfo,
			expected: "Info",
		},
		{
			name:     "custom severity",
			severity: ErrorSeverity("moderate"),
			expected: "moderate",
		},
		{
			name:     "empty severity",
			severity: ErrorSeverity(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSeverity(tt.severity)
			if result != tt.expected {
				t.Errorf("FormatSeverity(%v) = %q, want %q",
					tt.severity, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatSeverityWithIndicator Edge Cases (improve 80% coverage)
// =============================================================================

func TestFormatSeverityWithIndicator_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		expected string
	}{
		{
			name:     "SeverityCritical",
			severity: SeverityCritical,
			expected: "[🚨] CRITICAL",
		},
		{
			name:     "SeverityHigh",
			severity: SeverityHigh,
			expected: "[⚠️] High",
		},
		{
			name:     "SeverityMedium",
			severity: SeverityMedium,
			expected: "[⚡] Medium",
		},
		{
			name:     "SeverityLow",
			severity: SeverityLow,
			expected: "[ℹ️] Low",
		},
		{
			name:     "SeverityInfo",
			severity: SeverityInfo,
			expected: "[💡] Info",
		},
		{
			name:     "custom severity",
			severity: ErrorSeverity("moderate"),
			expected: "[❓] moderate",
		},
		{
			name:     "empty severity",
			severity: ErrorSeverity(""),
			expected: "[❓] ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSeverityWithIndicator(tt.severity)
			if result != tt.expected {
				t.Errorf("FormatSeverityWithIndicator(%v) = %q, want %q",
					tt.severity, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Integration Tests for Optional Fields
// =============================================================================

func TestValidationError_OptionalFieldsIntegration(t *testing.T) {
	// Test ValidationError with all optional fields populated
	fullErr := ValidationError{
		ErrorType:         "required",
		Message:           "Field is required",
		FieldName:         "email",
		Expected:          "valid@email.com",
		Actual:            "",
		Context:           "user registration",
		ResponseSnippet:   `{"user": {"email": ""}}`,
		PatternDetails:    "email format validation",
		RangeInfo:         "min length 1",
		ValidationDetails: []string{"Field is empty", "Field is required"},
		Location:          "line 10",
		Suggestions:       []string{"Provide a valid email", "Email cannot be empty"},
	}

	// Verify all fields are populated
	if fullErr.ErrorType == "" {
		t.Error("ErrorType should be populated")
	}
	if fullErr.Message == "" {
		t.Error("Message should be populated")
	}
	if len(fullErr.ValidationDetails) == 0 {
		t.Error("ValidationDetails should be populated")
	}
	if len(fullErr.Suggestions) == 0 {
		t.Error("Suggestions should be populated")
	}

	// Test ValidationError with minimal fields
	minimalErr := ValidationError{
		ErrorType: "required",
		Message:   "Field is required",
	}

	if minimalErr.ErrorType == "" {
		t.Error("ErrorType should be populated")
	}
	if minimalErr.Message == "" {
		t.Error("Message should be populated")
	}
	// Optional fields should be nil/empty
	if minimalErr.FieldName != "" {
		t.Error("FieldName should be empty for minimal error")
	}
	if minimalErr.Context != "" {
		t.Error("Context should be empty for minimal error")
	}
}

// =============================================================================
// ValidationFormatter with Optional Fields Tests
// =============================================================================

func TestValidationFormatter_OptionalFieldsChaining(t *testing.T) {
	tests := []struct {
		name      string
		formatter *ValidationFormatter
		check     func(ValidationError)
	}{
		{
			name: "all optional fields",
			formatter: NewValidationFormatter("status_code").
				WithExpected(200).
				WithActual(404).
				WithContext("GET /api/users").
				WithResponseSnippet(`{"error": "not found"}`).
				WithFieldName("status").
				WithPatternDetails("4xx pattern").
				WithRangeInfo("200-499").
				WithValidationDetails("Status code mismatch", "Expected 2xx").
				WithSuggestions("Check endpoint URL", "Verify resource exists"),
			check: func(err ValidationError) {
				if err.ErrorType != "status_code" {
					t.Errorf("ErrorType = %q, want status_code", err.ErrorType)
				}
				if err.Expected != 200 {
					t.Errorf("Expected = %v, want 200", err.Expected)
				}
				if err.Actual != 404 {
					t.Errorf("Actual = %v, want 404", err.Actual)
				}
				if err.Context != "GET /api/users" {
					t.Errorf("Context = %q, want 'GET /api/users'", err.Context)
				}
				if err.FieldName != "status" {
					t.Errorf("FieldName = %q, want 'status'", err.FieldName)
				}
				if len(err.ValidationDetails) != 2 {
					t.Errorf("ValidationDetails length = %d, want 2", len(err.ValidationDetails))
				}
				if len(err.Suggestions) != 2 {
					t.Errorf("Suggestions length = %d, want 2", len(err.Suggestions))
				}
			},
		},
		{
			name: "only required fields",
			formatter: NewValidationFormatter("required").
				WithExpected("present").
				WithActual(""),
			check: func(err ValidationError) {
				if err.ErrorType != "required" {
					t.Errorf("ErrorType = %q, want required", err.ErrorType)
				}
				// Optional fields should be empty
				if err.Context != "" {
					t.Errorf("Context should be empty, got %q", err.Context)
				}
				if len(err.ValidationDetails) != 0 {
					t.Errorf("ValidationDetails should be empty, got %d", len(err.ValidationDetails))
				}
			},
		},
		{
			name: "chaining order independence",
			formatter: NewValidationFormatter("format").
				WithSuggestions("Check format").
				WithFieldName("email").
				WithExpected("email@format").
				WithActual("invalid").
				WithContext("form validation").
				WithPatternDetails("must contain @"),
			check: func(err ValidationError) {
				// All fields should be set regardless of chaining order
				if err.FieldName != "email" {
					t.Errorf("FieldName = %q, want 'email'", err.FieldName)
				}
				if err.PatternDetails != "must contain @" {
					t.Errorf("PatternDetails = %q, want 'must contain @'", err.PatternDetails)
				}
				if len(err.Suggestions) != 1 {
					t.Errorf("Suggestions length = %d, want 1", len(err.Suggestions))
				}
			},
		},
		{
			name: "nil and empty values",
			formatter: NewValidationFormatter("type").
				WithExpected(nil).
				WithActual("").
				WithContext("").
				WithFieldName(""),
			check: func(err ValidationError) {
				// Should handle nil and empty values gracefully
				if err.ErrorType != "type" {
					t.Errorf("ErrorType = %q, want 'type'", err.ErrorType)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.formatter.Format()
			if tt.check != nil {
				tt.check(err)
			}
		})
	}
}

// =============================================================================
// Convenience Functions Tests with Optional Fields
// =============================================================================

func TestConvenienceFunctions_OptionalFields(t *testing.T) {
	// Test FormatStatusCodeError with minimal context
	err1 := FormatStatusCodeError(200, 404, "")
	if err1.Context != "" {
		t.Errorf("FormatStatusCodeError with empty context should have empty Context, got %q", err1.Context)
	}

	// Test FormatStatusCodeError with full context
	err2 := FormatStatusCodeError(200, 404, "GET /api/users")
	if err2.Context != "GET /api/users" {
		t.Errorf("Context = %q, want 'GET /api/users'", err2.Context)
	}
	if len(err2.Suggestions) == 0 {
		t.Error("Suggestions should be auto-generated")
	}

	// Test FormatErrorMessageError with all parameters
	err3 := FormatErrorMessageError("invalid.*token", "access_denied", "error", "OAuth validation")
	if err3.FieldName != "error" {
		t.Errorf("FieldName = %q, want 'error'", err3.FieldName)
	}
	if err3.Context != "OAuth validation" {
		t.Errorf("Context = %q, want 'OAuth validation'", err3.Context)
	}

	// Test FormatErrorMessageError with minimal parameters
	err4 := FormatErrorMessageError("pattern", "actual", "", "")
	if err4.FieldName != "" {
		t.Errorf("FieldName should be empty, got %q", err4.FieldName)
	}
	if err4.Context != "" {
		t.Errorf("Context should be empty, got %q", err4.Context)
	}
}

// =============================================================================
// WithTimeout Option Tests (0% coverage before)
// =============================================================================

func TestWithTimeoutOption(t *testing.T) {
	config := &FormatConfig{}
	WithTimeout(5000)(config)

	if config.Timeout != 5000 {
		t.Errorf("Timeout = %d, want 5000", config.Timeout)
	}
}

// =============================================================================
// formatSeverityPrefix Tests (improve 44.4% coverage)
// =============================================================================

func TestFormatSeverityPrefix_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		expected string
	}{
		{
			name:     "SeverityCritical",
			severity: SeverityCritical,
			expected: "[🚨 CRIT]",
		},
		{
			name:     "SeverityHigh",
			severity: SeverityHigh,
			expected: "[⚠️ HIGH]",
		},
		{
			name:     "SeverityMedium",
			severity: SeverityMedium,
			expected: "[⚡ MED]",
		},
		{
			name:     "SeverityLow",
			severity: SeverityLow,
			expected: "[ℹ️ LOW]",
		},
		{
			name:     "SeverityInfo",
			severity: SeverityInfo,
			expected: "[💡 INFO]",
		},
		{
			name:     "custom severity",
			severity: ErrorSeverity("custom"),
			expected: "[❓ UNK]",
		},
		{
			name:     "empty severity",
			severity: ErrorSeverity(""),
			expected: "[❓ UNK]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSeverityPrefix(tt.severity)
			if result != tt.expected {
				t.Errorf("formatSeverityPrefix(%v) = %q, want %q",
					tt.severity, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// formatCategorySuffix Tests (improve 66.7% coverage)
// =============================================================================

func TestFormatCategorySuffix_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		config   *FormatConfig
		expected string
	}{
		{
			name:     "HTTP with status code",
			category: CategoryHTTP,
			config:   &FormatConfig{StatusCode: 404},
			expected: " (status: 404)",
		},
		{
			name:     "HTTP without status code",
			category: CategoryHTTP,
			config:   &FormatConfig{},
			expected: "",
		},
		{
			name:     "Performance with timeout",
			category: CategoryPerformance,
			config:   &FormatConfig{Timeout: 5000},
			expected: " (timeout: 5000ms)",
		},
		{
			name:     "Performance without timeout",
			category: CategoryPerformance,
			config:   &FormatConfig{},
			expected: "",
		},
		{
			name:     "Security with context",
			category: CategorySecurity,
			config:   &FormatConfig{SecurityContext: "authentication"},
			expected: " (security: authentication)",
		},
		{
			name:     "Security without context",
			category: CategorySecurity,
			config:   &FormatConfig{},
			expected: "",
		},
		{
			name:     "Custom category",
			category: CategoryCustom,
			config:   &FormatConfig{StatusCode: 404},
			expected: "",
		},
		{
			name:     "Validation category",
			category: CategoryValidation,
			config:   &FormatConfig{},
			expected: "",
		},
		{
			name:     "Content category",
			category: CategoryContent,
			config:   &FormatConfig{Timeout: 1000},
			expected: "",
		},
		{
			name:     "nil config",
			category: CategoryHTTP,
			config:   nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCategorySuffix(tt.category, tt.config)
			if result != tt.expected {
				t.Errorf("formatCategorySuffix(%v, %+v) = %q, want %q",
					tt.category, tt.config, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatFieldReference Edge Cases (improve 92.6% coverage)
// =============================================================================

func TestFormatFieldReference_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		fieldPath string
		prefix   string
		options  []FieldRefOption
		expected string
	}{
		{
			name:     "empty path with prefix and option override",
			fieldPath: "",
			prefix:   "original",
			options:  []FieldRefOption{WithPrefix("override")},
			expected: "override",
		},
		{
			name:     "empty prefix gets overridden",
			fieldPath: "email",
			prefix:   "",
			options:  []FieldRefOption{WithPrefix("request")},
			expected: "request.email",
		},
		{
			name:     "non-empty prefix gets overridden",
			fieldPath: "email",
			prefix:   "original",
			options:  []FieldRefOption{WithPrefix("override")},
			expected: "override.email",
		},
		{
			name:     "array notation with prefix option",
			fieldPath: "users.0.email",
			prefix:   "",
			options:  []FieldRefOption{WithPrefix("response")},
			expected: "response.users[0].email",
		},
		{
			name:     "backtick style with array indices",
			fieldPath: "users.0.email",
			prefix:   "",
			options:  []FieldRefOption{WithQuoteStyle(Backtick)},
			expected: "`users`[0].`email`",
		},
		{
			name:     "double quote style with prefix",
			fieldPath: "user.email",
			prefix:   "request",
			options:  []FieldRefOption{WithQuoteStyle(DoubleQuote)},
			expected: "request.\"user\".\"email\"",
		},
		{
			name:     "single quote with empty path",
			fieldPath: "",
			prefix:   "data",
			options:  []FieldRefOption{WithQuoteStyle(SingleQuote)},
			expected: "data",
		},
		{
			name:     "multiple options - prefix and quote",
			fieldPath: "field.name",
			prefix:   "original",
			options: []FieldRefOption{
				WithPrefix("override"),
				WithQuoteStyle(DoubleQuote),
			},
			expected: "override.\"field\".\"name\"",
		},
		{
			name:     "nested arrays with brackets",
			fieldPath: "data.0.items.1",
			prefix:   "response",
			options:  []FieldRefOption{WithQuoteStyle(SingleQuote)},
			expected: "response.'data'[0].'items'[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath, tt.prefix, tt.options...)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, %q, options...) = %q, want %q",
					tt.fieldPath, tt.prefix, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatValidationErrorWithExpectedActual Optional Field Tests
// =============================================================================

func TestFormatValidationErrorWithExpectedActual_OptionalFields(t *testing.T) {
	tests := []struct {
		name           string
		err            ValidationError
		includeSeverity bool
		context        *ValidationErrorContext
		expectedActual ExpectedActual
		checkContains  []string
		notContains    []string
	}{
		{
			name: "empty ExpectedActual - no value comparison",
			err: ValidationError{
				ErrorType: "required",
				Message:   "Field is required",
				FieldName: "email",
			},
			includeSeverity: true,
			context:        nil,
			expectedActual: ExpectedActual{},
			checkContains:  []string{"[required]", "email:", "Field is required"},
			notContains:    []string{"expected:", "actual:", "expected:", "got"},
		},
		{
			name: "ExpectedActual with both values",
			err: ValidationError{
				ErrorType: "format",
				Message:   "Invalid format",
				FieldName: "status",
			},
			includeSeverity: true,
			context:        nil,
			expectedActual: NewExpectedActual(200, 404),
			checkContains:  []string{"[format]", "status:", "200", "404"},
			notContains:    []string{},
		},
		{
			name: "ExpectedActual with only expected",
			err: ValidationError{
				ErrorType: "value",
				Message:   "Invalid value",
				FieldName: "count",
			},
			includeSeverity: false,
			context:        nil,
			expectedActual: NewExpectedActual("required", nil),
			checkContains:  []string{"[value]", "count:", "required"},
			notContains:    []string{"got", "actual:"},
		},
		{
			name: "ExpectedActual with only actual",
			err: ValidationError{
				ErrorType: "type",
				Message:   "Type mismatch",
				FieldName: "age",
			},
			includeSeverity: true,
			context:        nil,
			expectedActual: NewExpectedActual(nil, "string"),
			checkContains:  []string{"[type]", "age:", "got string"},
			notContains:    []string{"expected:"},
		},
		{
			name: "with context containing location",
			err: ValidationError{
				ErrorType: "pattern",
				Message:   "Pattern mismatch",
				FieldName: "email",
			},
			includeSeverity: true,
			context:        func() *ValidationErrorContext { ctx := NewValidationErrorContext("line 5"); return &ctx }(),
			expectedActual: NewExpectedActual(nil, nil),
			checkContains:  []string{"[pattern]", "location: line 5"},
			notContains:    []string{},
		},
		{
			name: "with context containing related fields",
			err: ValidationError{
				ErrorType: "duplicate",
				Message:   "Duplicate value",
				FieldName: "email",
			},
			includeSeverity: true,
			context: func() *ValidationErrorContext { ctx := NewValidationErrorContext("").WithRelatedFields([]string{"email", "email_confirmation"}); return &ctx }(),
			expectedActual: ExpectedActual{},
			checkContains:  []string{"related fields:", "email", "email_confirmation"},
			notContains:    []string{},
		},
		{
			name: "with suggestions",
			err: ValidationError{
				ErrorType:         "range",
				Message:           "Out of range",
				FieldName:         "age",
				Suggestions:       []string{"Check age range", "Verify input"},
			},
			includeSeverity: true,
			context:        nil,
			expectedActual: NewExpectedActual("0-120", 150),
			checkContains:  []string{"Suggestions:", "Check age range", "Verify input"},
			notContains:    []string{},
		},
		{
			name: "with location in error itself",
			err: ValidationError{
				ErrorType: "required",
				Message:   "Required field",
				FieldName: "name",
				Location:  "line 10",
			},
			includeSeverity: true,
			context:        nil,
			expectedActual: ExpectedActual{},
			checkContains:  []string{"[line 10]"},
			notContains:    []string{},
		},
		{
			name: "all optional fields populated",
			err: ValidationError{
				ErrorType:         "custom",
				Message:           "Custom validation failed",
				FieldName:         "field",
				Location:          "position 42",
				Suggestions:       []string{"Suggestion 1", "Suggestion 2"},
			},
			includeSeverity: true,
			context: func() *ValidationErrorContext { ctx := NewValidationErrorContext("line 5").WithRelatedFields([]string{"related"}); return &ctx }(),
			expectedActual: NewExpectedActual("expected", "actual"),
			checkContains:  []string{"[custom]", "position 42", "location: line 5", "related fields:", "Suggestions:"},
			notContains:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorWithExpectedActual(
				tt.err, tt.includeSeverity, tt.context, tt.expectedActual,
			)

			// Check that expected substrings are present
			for _, substr := range tt.checkContains {
				if !strings.Contains(result, substr) {
					t.Errorf("Expected result to contain %q, but got:\n%s", substr, result)
				}
			}

			// Check that unwanted substrings are NOT present
			for _, substr := range tt.notContains {
				if strings.Contains(result, substr) {
					t.Errorf("Expected result NOT to contain %q, but got:\n%s", substr, result)
				}
			}
		})
	}
}

// =============================================================================
// Edge Case: Nil and Empty Values
// =============================================================================

func TestFormatting_NilAndEmptyValues(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(*testing.T)
	}{
		{
			name: "FormatFieldRef with nil equivalent values",
			testFunc: func(t *testing.T) {
				result := FormatFieldRef("", "")
				if result != "(unknown field)" {
					t.Errorf("Empty field and parent should return '(unknown field)', got %q", result)
				}
			},
		},
		{
			name: "FormatErrorMessage with empty message",
			testFunc: func(t *testing.T) {
				result := FormatErrorMessage("required", "", "field")
				if !strings.Contains(result, "required") {
					t.Errorf("Empty message should still include error type")
				}
			},
		},
		{
			name: "FormatErrorList with empty slice",
			testFunc: func(t *testing.T) {
				result := FormatErrorList([]ValidationError{}, true)
				if result != "No errors" {
					t.Errorf("Empty error list should return 'No errors', got %q", result)
				}
			},
		},
		{
			name: "FormatErrorListWithContext with nil contexts",
			testFunc: func(t *testing.T) {
				errs := []ValidationError{
					{ErrorType: "required", Message: "Required", FieldName: "field"},
				}
				result := FormatErrorListWithContext(errs, nil, true)
				if !strings.Contains(result, "required") {
					t.Errorf("Result should contain error type")
				}
			},
		},
		{
			name: "FormatFieldListWith with empty slice",
			testFunc: func(t *testing.T) {
				result := FormatFieldListWith([]string{}, "and")
				if result != "" {
					t.Errorf("Empty slice should return empty string, got %q", result)
				}
			},
		},
		{
			name: "FormatFieldLocationWith with empty location",
			testFunc: func(t *testing.T) {
				result := FormatFieldLocationWith("field", "", "parent")
				expected := "parent.field"
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			},
		},
		{
			name: "ValidationErrorContext with empty fields",
			testFunc: func(t *testing.T) {
				ctx := NewValidationErrorContext("")
				if !ctx.IsEmpty() {
					t.Error("Empty context should be empty")
				}
			},
		},
		{
			name: "ExpectedActual with nil values",
			testFunc: func(t *testing.T) {
				ea := NewExpectedActual(nil, nil)
				if !ea.IsEmpty() {
					t.Error("ExpectedActual with nil values should be empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// =============================================================================
// Test main formatter functions with nil context
// =============================================================================

func TestFormatValidationErrorFull_WithNilContext(t *testing.T) {
	err := ValidationError{
		ErrorType: "required",
		Message:   "Field is required",
		FieldName: "email",
	}

	result := FormatValidationErrorFull(err, true, nil)

	if !strings.Contains(result, "[required]") {
		t.Errorf("Result should contain error type")
	}
	if !strings.Contains(result, "email:") {
		t.Errorf("Result should contain field name")
	}
	if !strings.Contains(result, "Field is required") {
		t.Errorf("Result should contain message")
	}
}

// =============================================================================
// Private Helper Function Tests (formatMapValue, formatSliceValue)
// These are tested indirectly through FormatExpectedActual, but we add explicit
// tests to ensure they handle edge cases properly.
// =============================================================================

func TestFormatHelperFunctions_PrivateHelpers(t *testing.T) {
	// Test formatMapValue and formatSliceValue through ExpectedActual
	t.Run("formatMapValue through ExpectedActual", func(t *testing.T) {
		tests := []struct {
			name          string
			input         map[string]interface{}
			checkContains []string
		}{
			{
				name:         "empty map",
				input:        map[string]interface{}{},
				checkContains: []string{"{}"},
			},
			{
				name:         "single item",
				input:        map[string]interface{}{"key": "value"},
				checkContains: []string{"{key: value}"},
			},
			{
				name:         "multiple items (check each part present)",
				input:        map[string]interface{}{"k1": "v1", "k2": "v2", "k3": "v3"},
				checkContains: []string{"k1: v1", "k2: v2", "k3: v3", "{", "}"},
			},
			{
				name:         "large map gets limited",
				input:        map[string]interface{}{"k1": "v1", "k2": "v2", "k3": "v3", "k4": "v4", "k5": "v5", "k6": "v6"},
				checkContains: []string{"..."},
			},
			{
				name:         "nested values",
				input:        map[string]interface{}{"count": 5, "name": "test", "active": true},
				checkContains: []string{"count: 5", "name: test", "active: true"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ea := NewExpectedActual(tt.input, nil)
				result := FormatExpectedActual(ea)

				for _, substr := range tt.checkContains {
					if !strings.Contains(result, substr) {
						t.Errorf("Expected result to contain %q, got: %s", substr, result)
					}
				}
			})
		}
	})

	t.Run("formatSliceValue through ExpectedActual", func(t *testing.T) {
		tests := []struct {
			name     string
			input    []interface{}
			expected string
		}{
			{
				name:     "empty slice",
				input:    []interface{}{},
				expected: "[]",
			},
			{
				name:     "single item",
				input:    []interface{}{"value"},
				expected: "[value]",
			},
			{
				name:     "multiple items",
				input:    []interface{}{"v1", "v2", "v3"},
				expected: "[v1, v2, v3]",
			},
			{
				name:     "large slice gets limited",
				input:    []interface{}{1, 2, 3, 4, 5, 6},
				expected: "[1, 2, 3, 4, 5, ...]",
			},
			{
				name:     "mixed types",
				input:    []interface{}{"string", 42, true, 3.14},
				expected: "[string, 42, true, 3.14]",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ea := NewExpectedActual(tt.input, nil)
				result := FormatExpectedActual(ea)
				if !strings.Contains(result, tt.expected) {
					t.Errorf("Expected result to contain %q, got: %s", tt.expected, result)
				}
			})
		}
	})
}

// =============================================================================
// Additional Edge Cases for Existing Functions
// =============================================================================

func TestFormatErrorMessage_AdditionalEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		expected  string
	}{
		{
			name:      "unicode characters in field name",
			errorType: "required",
			message:   "Field is required",
			fieldName: "用户邮箱",
			expected:  "[required] 用户邮箱: Field is required",
		},
		{
			name:      "very long field name",
			errorType: "format",
			message:   "Invalid format",
			fieldName: strings.Repeat("a", 200),
			expected:  "[format] " + strings.Repeat("a", 200) + ": Invalid format",
		},
		{
			name:      "multiline message",
			errorType: "validation",
			message:   "Line 1\nLine 2\nLine 3",
			fieldName: "field",
			expected:  "field: Line 1\nLine 2\nLine 3",
		},
		{
			name:      "special regex characters",
			errorType: "pattern",
			message:   "Does not match [a-z]+",
			fieldName: "test.field",
			expected:  "test.field: Does not match [a-z]+",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorMessageString(tt.errorType, tt.message, tt.fieldName)
			if result != tt.expected {
				t.Errorf("FormatErrorMessageString(%q, %q, %q) = %q, want %q",
					tt.errorType, tt.message, tt.fieldName, result, tt.expected)
			}
		})
	}
}

func TestFormatFieldListWith_MoreEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		fields      []string
		conjunction string
		expected    string
	}{
		{
			name:        "fields with unicode",
			fields:      []string{"email", "姓名", "phone"},
			conjunction: "and",
			expected:    "email, 姓名, and phone",
		},
		{
			name:        "fields with brackets",
			fields:      []string{"items[0]", "items[1]", "items[2]"},
			conjunction: "or",
			expected:    "items[0], items[1], or items[2]",
		},
		{
			name:        "empty string in list",
			fields:      []string{"email", "", "password"},
			conjunction: "and",
			expected:    "email, , and password",
		},
		{
			name:        "very long field names",
			fields:      []string{strings.Repeat("a", 100), strings.Repeat("b", 100)},
			conjunction: "and",
			expected:    strings.Repeat("a", 100) + " and " + strings.Repeat("b", 100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldListWith(tt.fields, tt.conjunction)
			if result != tt.expected {
				t.Errorf("FormatFieldListWith(%v, %q) = %q, want %q",
					tt.fields, tt.conjunction, result, tt.expected)
			}
		})
	}
}

func TestFormatErrorWithValues_MoreEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		fieldName    string
		expected     interface{}
		actual       interface{}
		customMsg    string
		checkContains []string
	}{
		{
			name:        "boolean values",
			fieldName:   "active",
			expected:    true,
			actual:      false,
			customMsg:   "Status incorrect",
			checkContains: []string{"active:", "Status incorrect", "expected true", "got false"},
		},
		{
			name:        "float precision",
			fieldName:   "price",
			expected:    3.14159,
			actual:      2.71828,
			customMsg:   "",
			checkContains: []string{"price:", "expected 3.14159", "got 2.71828"},
		},
		{
			name:        "nested structures",
			fieldName:   "config",
			expected:    map[string]interface{}{"key": "value"},
			actual:      "simple",
			customMsg:   "",
			checkContains: []string{"config:", "expected map", "got simple"},
		},
		{
			name:        "zero values",
			fieldName:   "count",
			expected:    0,
			actual:      0,
			customMsg:   "Values are zero",
			checkContains: []string{"count:", "Values are zero", "expected 0", "got 0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithValues(tt.fieldName, tt.expected, tt.actual, tt.customMsg)

			for _, substr := range tt.checkContains {
				if !strings.Contains(result, substr) {
					t.Errorf("Expected result to contain %q, got: %s", substr, result)
				}
			}
		})
	}
}

// =============================================================================
// FormatErrorMessage Enum Handling Tests
// =============================================================================

func TestFormatErrorMessage_EnumHandling(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		expected  string
	}{
		// Test with all standard ErrorType enum values
		{
			name:      "ErrTypeRequired with all parameters",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "email",
			expected:  "[required] email: Field is required",
		},
		{
			name:      "ErrTypeFormat with all parameters",
			errorType: ErrTypeFormat,
			message:   "Invalid email format",
			fieldName: "email",
			expected:  "[format] email: Invalid email format",
		},
		{
			name:      "ErrTypeRange with all parameters",
			errorType: ErrTypeRange,
			message:   "Value out of range",
			fieldName: "age",
			expected:  "[range] age: Value out of range",
		},
		{
			name:      "ErrTypeLength with all parameters",
			errorType: ErrTypeLength,
			message:   "String too long",
			fieldName: "password",
			expected:  "[length] password: String too long",
		},
		{
			name:      "ErrTypeType with all parameters",
			errorType: ErrTypeType,
			message:   "Expected number, got string",
			fieldName: "count",
			expected:  "[type] count: Expected number, got string",
		},
		{
			name:      "ErrTypeValue with all parameters",
			errorType: ErrTypeValue,
			message:   "Invalid value",
			fieldName: "status",
			expected:  "[value] status: Invalid value",
		},
		{
			name:      "ErrTypeDuplicate with all parameters",
			errorType: ErrTypeDuplicate,
			message:   "Duplicate entry",
			fieldName: "email",
			expected:  "[duplicate] email: Duplicate entry",
		},
		{
			name:      "ErrTypeConflict with all parameters",
			errorType: ErrTypeConflict,
			message:   "Conflict with existing data",
			fieldName: "timestamp",
			expected:  "[conflict] timestamp: Conflict with existing data",
		},
		// Test without field name
		{
			name:      "ErrTypeRequired without field name",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "",
			expected:  "[required] Field is required",
		},
		{
			name:      "ErrTypeFormat without field name",
			errorType: ErrTypeFormat,
			message:   "Invalid format",
			fieldName: "",
			expected:  "[format] Invalid format",
		},
		// Test ErrTypeUnknown handling
		{
			name:      "ErrTypeUnknown should not include brackets",
			errorType: ErrTypeUnknown,
			message:   "Unknown error",
			fieldName: "field",
			expected:  "field: Unknown error",
		},
		// Test empty string ErrorType
		{
			name:      "Empty ErrorType should not include brackets",
			errorType: "",
			message:   "Test message",
			fieldName: "field",
			expected:  "field: Test message",
		},
		// Test with whitespace
		{
			name:      "Trim whitespace from message",
			errorType: ErrTypeRequired,
			message:   "  Field is required  ",
			fieldName: "email",
			expected:  "[required] email: Field is required",
		},
		{
			name:      "Trim whitespace from field name",
			errorType: ErrTypeFormat,
			message:   "Invalid format",
			fieldName: "  email  ",
			expected:  "[format] email: Invalid format",
		},
		// Test with empty message
		{
			name:      "Empty message with field name",
			errorType: ErrTypeRequired,
			message:   "",
			fieldName: "email",
			expected:  "[required] email: ",
		},
		// Test complex field names
		{
			name:      "Nested field name",
			errorType: ErrTypeRequired,
			message:   "Required",
			fieldName: "user.profile.email",
			expected:  "[required] user.profile.email: Required",
		},
		{
			name:      "Array index field name",
			errorType: ErrTypeFormat,
			message:   "Invalid format",
			fieldName: "items[0].email",
			expected:  "[format] items[0].email: Invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorMessage(tt.errorType, tt.message, tt.fieldName)
			if result != tt.expected {
				t.Errorf("FormatErrorMessage(%v, %q, %q) = %q, want %q",
					tt.errorType, tt.message, tt.fieldName, result, tt.expected)
			}
		})
	}
}

func TestFormatErrorMessage_EnumMethods(t *testing.T) {
	// Test that ErrorType enum methods work correctly
	t.Run("ErrorType String() method", func(t *testing.T) {
		tests := []struct {
			errorType ErrorType
			expected  string
		}{
			{ErrTypeRequired, "required"},
			{ErrTypeFormat, "format"},
			{ErrTypeRange, "range"},
			{ErrTypeLength, "length"},
			{ErrTypeType, "type"},
			{ErrTypeValue, "value"},
			{ErrTypeDuplicate, "duplicate"},
			{ErrTypeConflict, "conflict"},
			{ErrTypeUnknown, "unknown"},
		}

		for _, tt := range tests {
			if got := tt.errorType.String(); got != tt.expected {
				t.Errorf("ErrorType.String() = %q, want %q", got, tt.expected)
			}
		}
	})

	t.Run("ErrorType IsValid() method", func(t *testing.T) {
		validTypes := []ErrorType{
			ErrTypeRequired, ErrTypeFormat, ErrTypeRange, ErrTypeLength,
			ErrTypeType, ErrTypeValue, ErrTypeDuplicate, ErrTypeConflict,
			ErrTypeUnknown,
		}

		for _, et := range validTypes {
			if !et.IsValid() {
				t.Errorf("ErrorType %v should be valid", et)
			}
		}

		invalidType := ErrorType("invalid_type")
		if invalidType.IsValid() {
			t.Error("Custom ErrorType should not be valid")
		}
	})

	t.Run("ErrorType Description() method", func(t *testing.T) {
		tests := []struct {
			errorType     ErrorType
			shouldContain string
		}{
			{ErrTypeRequired, "required"},
			{ErrTypeFormat, "format"},
			{ErrTypeRange, "range"},
			{ErrTypeLength, "length"},
			{ErrTypeType, "type"},
			{ErrTypeValue, "value"},
			{ErrTypeDuplicate, "duplicate"},
			{ErrTypeConflict, "conflict"},
			{ErrTypeUnknown, "Unknown"},
		}

		for _, tt := range tests {
			desc := tt.errorType.Description()
			if !strings.Contains(strings.ToLower(desc), strings.ToLower(tt.shouldContain)) {
				t.Errorf("ErrorType.Description() for %v should contain %q, got %q",
					tt.errorType, tt.shouldContain, desc)
			}
		}
	})
}

func TestFormatErrorMessage_BackwardCompatibility(t *testing.T) {
	// Test that FormatErrorMessageString provides backward compatibility
	t.Run("FormatErrorMessageString with valid types", func(t *testing.T) {
		tests := []struct {
			errorTypeStr string
			message      string
			fieldName    string
			expected     string
		}{
			{
				errorTypeStr: "required",
				message:      "Field is required",
				fieldName:    "email",
				expected:     "[required] email: Field is required",
			},
			{
				errorTypeStr: "format",
				message:      "Invalid format",
				fieldName:    "email",
				expected:     "[format] email: Invalid format",
			},
			{
				errorTypeStr: "range",
				message:      "Out of range",
				fieldName:    "age",
				expected:     "[range] age: Out of range",
			},
		}

		for _, tt := range tests {
			result := FormatErrorMessageString(tt.errorTypeStr, tt.message, tt.fieldName)
			if result != tt.expected {
				t.Errorf("FormatErrorMessageString(%q, %q, %q) = %q, want %q",
					tt.errorTypeStr, tt.message, tt.fieldName, result, tt.expected)
			}
		}
	})

	t.Run("FormatErrorMessageString with unknown types", func(t *testing.T) {
		// Unknown types should be converted to ErrTypeUnknown
		result := FormatErrorMessageString("unknown_type", "Test message", "field")
		expected := "field: Test message" // ErrTypeUnknown doesn't add brackets
		if result != expected {
			t.Errorf("FormatErrorMessageString with unknown type = %q, want %q", result, expected)
		}
	})

	t.Run("Enum and string versions produce same output", func(t *testing.T) {
		message := "Test message"
		fieldName := "email"

		// Test all standard enum values
		enumTypes := []ErrorType{
			ErrTypeRequired, ErrTypeFormat, ErrTypeRange, ErrTypeLength,
			ErrTypeType, ErrTypeValue, ErrTypeDuplicate, ErrTypeConflict,
		}

		for _, et := range enumTypes {
			enumResult := FormatErrorMessage(et, message, fieldName)
			stringResult := FormatErrorMessageString(et.String(), message, fieldName)

			if enumResult != stringResult {
				t.Errorf("Enum and string versions differ for %v: enum=%q, string=%q",
					et, enumResult, stringResult)
			}
		}
	})
}

func TestFormatErrorMessage_EnumIntegration(t *testing.T) {
	// Test integration with ValidationError
	t.Run("FormatErrorMessage with ValidationError", func(t *testing.T) {
		ve := ValidationError{
			ErrorType: string(ErrTypeRequired),
			Message:   "Field is required",
			FieldName: "email",
		}

		// Convert string ErrorType back to enum and format
		et := ErrorTypeFromString(ve.ErrorType)
		formatted := FormatErrorMessage(et, ve.Message, ve.FieldName)

		expected := "[required] email: Field is required"
		if formatted != expected {
			t.Errorf("Integration test failed: got %q, want %q", formatted, expected)
		}
	})

	t.Run("ErrorTypeFromString conversion", func(t *testing.T) {
		tests := []struct {
			input    string
			expected ErrorType
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
			{"invalid_type", ErrTypeUnknown},
		}

		for _, tt := range tests {
			result := ErrorTypeFromString(tt.input)
			if result != tt.expected {
				t.Errorf("ErrorTypeFromString(%q) = %v, want %v",
					tt.input, result, tt.expected)
			}
		}
	})
}
