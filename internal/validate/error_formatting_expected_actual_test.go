package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// EXPECTED VS ACTUAL FORMATTING TESTS
// =============================================================================

func TestFormatExpectedActual(t *testing.T) {
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
			ea:       ExpectedActual{Expected: 200},
			expected: "expected: 200 (OK)",
		},
		{
			name:     "only actual value",
			ea:       ExpectedActual{Actual: 404},
			expected: "actual: 404 (Not Found)",
		},
		{
			name:     "both expected and actual - integers",
			ea:       NewExpectedActual(200, 404),
			expected: "expected: 200 (OK), actual: 404 (Not Found)",
		},
		{
			name:     "both expected and actual - strings",
			ea:       NewExpectedActual("test@example.com", "invalid"),
			expected: "expected: 'test@example.com', actual: 'invalid'",
		},
		{
			name:     "long string is truncated",
			ea:       ExpectedActual{Actual: string(make([]byte, 150))},
			expected: "actual: '",
		},
		{
			name:     "expected as slice of ints",
			ea:       ExpectedActual{Expected: []int{200, 201, 204}},
			expected: "expected: one of [200 (OK), 201 (Created), 204 (No Content)]",
		},
		{
			name:     "expected and actual with slice of strings",
			ea:       NewExpectedActual([]string{"a", "b"}, []string{"c", "d"}),
			expected: "expected: ['a', 'b'], actual: ['c', 'd']",
		},
		{
			name:     "expected as float",
			ea:       NewExpectedActual(3.14159, 2.71828),
			expected: "expected: 3.14, actual: 2.72",
		},
		{
			name:     "expected as map",
			ea:       ExpectedActual{Expected: map[string]interface{}{"key": "value"}},
			expected: "expected: {key: value}",
		},
		{
			name:     "expected as slice of interfaces",
			ea:       ExpectedActual{Expected: []interface{}{1, "two", 3.0}},
			expected: "expected: [1, two, 3]",
		},
		{
			name:     "empty map",
			ea:       ExpectedActual{Expected: map[string]interface{}{}},
			expected: "expected: {}",
		},
		{
			name:     "empty slice",
			ea:       ExpectedActual{Expected: []interface{}{}},
			expected: "expected: []",
		},
		{
			name:     "large map is truncated",
			ea:       ExpectedActual{Expected: map[string]interface{}{
				"k1": "v1", "k2": "v2", "k3": "v3", "k4": "v4", "k5": "v5", "k6": "v6"}},
			expected: "expected: {",
		},
		{
			name:     "large slice is truncated",
			ea:       ExpectedActual{Expected: []interface{}{1, 2, 3, 4, 5, 6}},
			expected: "expected: [1, 2, 3, 4, 5, ...]",
		},
		{
			name:     "nil values handled gracefully",
			ea:       NewExpectedActual(nil, nil),
			expected: "",
		},
		{
			name:     "expected nil, actual present",
			ea:       ExpectedActual{Expected: nil, Actual: "test"},
			expected: "actual: 'test'",
		},
		{
			name:     "expected present, actual nil",
			ea:       ExpectedActual{Expected: "test", Actual: nil},
			expected: "expected: 'test'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatExpectedActual(tt.ea)
			if tt.expected == "" {
				if result != "" {
					t.Errorf("FormatExpectedActual() = %v, want empty string", result)
				}
			} else {
				if !strings.HasPrefix(result, tt.expected) && !strings.Contains(result, tt.expected) {
					t.Errorf("FormatExpectedActual() = %v, want to contain %v", result, tt.expected)
				}
			}
		})
	}
}

func TestFormatExpectedActualInline(t *testing.T) {
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
			name:     "both values",
			ea:       NewExpectedActual(200, 404),
			expected: "(expected 200, got 404)",
		},
		{
			name:     "only expected",
			ea:       ExpectedActual{Expected: "test"},
			expected: "(expected test)",
		},
		{
			name:     "only actual",
			ea:       ExpectedActual{Actual: "result"},
			expected: "(got result)",
		},
		{
			name:     "string values",
			ea:       NewExpectedActual("hello", "world"),
			expected: "(expected hello, got world)",
		},
		{
			name:     "complex types",
			ea:       NewExpectedActual([]int{1, 2}, []int{3, 4}),
			expected: "(expected [1 2], got [3 4])",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatExpectedActualInline(tt.ea)
			if result != tt.expected {
				t.Errorf("FormatExpectedActualInline() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatValidationErrorWithExpectedActual(t *testing.T) {
	tests := []struct {
		name            string
		err             ValidationError
		includeSeverity bool
		context         *ValidationErrorContext
		expectedActual  ExpectedActual
		expectedContains []string
	}{
		{
			name: "without ExpectedActual parameter",
			err: ValidationError{
				ErrorType: "required",
				Message:   "Field is required",
				FieldName: "email",
			},
			includeSeverity: true,
			context:        nil,
			expectedActual: ExpectedActual{},
			expectedContains: []string{"[required]", "email: Field is required"},
		},
		{
			name: "with ExpectedActual parameter - integers",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "Status code mismatch",
				FieldName: "response.status",
			},
			includeSeverity: true,
			context:        nil,
			expectedActual: NewExpectedActual(200, 404),
			expectedContains: []string{"status_code", "response.status", "expected: 200 (OK)", "actual: 404 (Not Found)"},
		},
		{
			name: "with ExpectedActual parameter - strings",
			err: ValidationError{
				ErrorType: "format",
				Message:   "Invalid email format",
				FieldName: "email",
			},
			includeSeverity: false,
			context:        nil,
			expectedActual: NewExpectedActual("test@example.com", "invalid"),
			expectedContains: []string{"[format]", "email", "expected: 'test@example.com'", "actual: 'invalid'"},
		},
		{
			name: "with ExpectedActual parameter - slice of ints",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "Unexpected status code",
			},
			includeSeverity: true,
			context:        nil,
			expectedActual: ExpectedActual{Expected: []int{200, 201, 204}, Actual: 500},
			expectedContains: []string{"expected: one of [200 (OK), 201 (Created), 204 (No Content)]", "actual: 500 (Internal Server Error)"},
		},
		{
			name: "with ExpectedActual parameter and context",
			err: ValidationError{
				ErrorType: "required",
				Message:   "Field is required",
				FieldName: "email",
			},
			includeSeverity: true,
			context:        func() *ValidationErrorContext {
				ctx := NewValidationErrorContext("line 5")
				ctx.RelatedFields = []string{"email_confirmation"}
				return &ctx
			}(),
			expectedActual: NewExpectedActual("user@example.com", ""),
			expectedContains: []string{"email: Field is required", "expected: 'user@example.com'", "location: line 5", "related fields: email_confirmation"},
		},
		{
			name: "ExpectedActual is empty - falls back to err.Expected/err.Actual",
			err: ValidationError{
				ErrorType: "value",
				Message:   "Value mismatch",
				Expected:  42,
				Actual:    24,
			},
			includeSeverity: false,
			context:        nil,
			expectedActual: ExpectedActual{},
			expectedContains: []string{"expected: 42", "actual: 24"},
		},
		{
			name: "ExpectedActual takes precedence over err.Expected/err.Actual",
			err: ValidationError{
				ErrorType: "value",
				Message:   "Value mismatch",
				Expected:  42,
				Actual:    24,
			},
			includeSeverity: false,
			context:        nil,
			expectedActual: NewExpectedActual(100, 200),
			expectedContains: []string{"expected: 100", "actual: 200"},
		},
		{
			name: "with ExpectedActual containing floats",
			err: ValidationError{
				ErrorType: "range",
				Message:   "Value out of range",
			},
			includeSeverity: false,
			context:        nil,
			expectedActual: NewExpectedActual(3.14159, 2.71828),
			expectedContains: []string{"expected: 3.14", "actual: 2.72"},
		},
		{
			name: "with ExpectedActual containing map",
			err: ValidationError{
				ErrorType: "structure",
				Message:   "Structure mismatch",
			},
			includeSeverity: false,
			context:        nil,
			expectedActual: ExpectedActual{Expected: map[string]interface{}{"key": "value"}, Actual: map[string]interface{}{"other": "data"}},
			expectedContains: []string{"expected: {key: value}", "actual: {other: data}"},
		},
		{
			name: "with ExpectedActual containing slice of strings",
			err: ValidationError{
				ErrorType: "values",
				Message:   "Values mismatch",
			},
			includeSeverity: false,
			context:        nil,
			expectedActual: NewExpectedActual([]string{"a", "b"}, []string{"c", "d"}),
			expectedContains: []string{"expected: ['a', 'b']", "actual: ['c', 'd']"},
		},
		{
			name: "with empty ExpectedActual and empty err.Expected/err.Actual",
			err: ValidationError{
				ErrorType: "generic",
				Message:   "Generic error",
			},
			includeSeverity: false,
			context:        nil,
			expectedActual: ExpectedActual{},
			expectedContains: []string{"[generic] Generic error"},
		},
		{
			name: "with location in error",
			err: ValidationError{
				ErrorType: "format",
				Message:   "Format error",
				Location:  "line 10",
			},
			includeSeverity: false,
			context:        nil,
			expectedActual: NewExpectedActual("pattern", "nomatch"),
			expectedContains: []string{"[format] Format error", "expected: 'pattern'", "actual: 'nomatch'", "[line 10]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorWithExpectedActual(tt.err, tt.includeSeverity, tt.context, tt.expectedActual)

			for _, expected := range tt.expectedContains {
				if !strings.Contains(result, expected) {
					t.Errorf("FormatValidationErrorWithExpectedActual() result:\n%v\n\ndoes not contain expected: %v", result, expected)
				}
			}
		})
	}
}

func TestFormatMapValue(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]interface{}
		expected string
	}{
		{
			name:     "empty map",
			m:        map[string]interface{}{},
			expected: "{}",
		},
		{
			name:     "single item",
			m:        map[string]interface{}{"key": "value"},
			expected: "{key: value}",
		},
		{
			name:     "multiple items",
			m:        map[string]interface{}{"k1": "v1", "k2": "v2"},
			expected: "k2: v2",
		},
		{
			name:     "large map is truncated",
			m: map[string]interface{}{
				"k1": "v1", "k2": "v2", "k3": "v3", "k4": "v4", "k5": "v5", "k6": "v6",
			},
			expected: "...}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMapValue(tt.m)
			if tt.expected != "" && !strings.Contains(result, tt.expected) {
				t.Errorf("formatMapValue() = %v, want to contain %v", result, tt.expected)
			}
		})
	}
}

func TestFormatSliceValue(t *testing.T) {
	tests := []struct {
		name     string
		s        []interface{}
		expected string
	}{
		{
			name:     "empty slice",
			s:        []interface{}{},
			expected: "[]",
		},
		{
			name:     "single item",
			s:        []interface{}{1},
			expected: "[1]",
		},
		{
			name:     "multiple items",
			s:        []interface{}{1, "two", 3.0},
			expected: "[1, two, 3]",
		},
		{
			name:     "large slice is truncated",
			s:        []interface{}{1, 2, 3, 4, 5, 6},
			expected: "[1, 2, 3, 4, 5, ...]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSliceValue(tt.s)
			if result != tt.expected {
				t.Errorf("formatSliceValue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Integration test for ExpectedActual with ValidationError
func TestValidationErrorWithExpectedActualIntegration(t *testing.T) {
	t.Run("ExpectedActual is omitted when empty", func(t *testing.T) {
		err := ValidationError{
			ErrorType: "test",
			Message:   "Test error",
		}

		ctx := NewValidationErrorContext("line 1")
		ea := ExpectedActual{} // Empty

		result := FormatValidationErrorWithExpectedActual(err, false, &ctx, ea)

		// Should not contain "expected" or "actual" from ExpectedActual
		if strings.Contains(result, "expected:") && strings.Contains(result, "actual:") {
			// Only check if both are present together from ExpectedActual formatting
			t.Errorf("Expected actual section to be omitted when ExpectedActual is empty, got: %v", result)
		}
	})

	t.Run("ExpectedActual values are displayed when provided", func(t *testing.T) {
		err := ValidationError{
			ErrorType: "test",
			Message:   "Test error",
		}

		ea := NewExpectedActual("expected_value", "actual_value")

		result := FormatValidationErrorWithExpectedActual(err, false, nil, ea)

		if !strings.Contains(result, "expected: 'expected_value'") {
			t.Errorf("Expected expected value to be displayed, got: %v", result)
		}
		if !strings.Contains(result, "actual: 'actual_value'") {
			t.Errorf("Expected actual value to be displayed, got: %v", result)
		}
	})

	t.Run("Different value types are handled correctly", func(t *testing.T) {
		types := []struct {
			name          string
			expected      interface{}
			actual        interface{}
			expectedInStr string
			actualInStr   string
		}{
			{
				name:          "integers",
				expected:      200,
				actual:        404,
				expectedInStr: "200 (OK)",
				actualInStr:   "404 (Not Found)",
			},
			{
				name:          "strings",
				expected:      "test",
				actual:        "result",
				expectedInStr: "'test'",
				actualInStr:   "'result'",
			},
			{
				name:          "floats",
				expected:      3.14,
				actual:        2.71,
				expectedInStr: "3.14",
				actualInStr:   "2.71",
			},
			{
				name:          "slices",
				expected:      []int{1, 2},
				actual:        []int{3, 4},
				expectedInStr: "one of [1 (Unknown), 2 (Unknown)]",
				actualInStr:   "[3 4]",
			},
		}

		for _, tt := range types {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidationError{
					ErrorType: "test",
					Message:   "Test error",
				}

				ea := NewExpectedActual(tt.expected, tt.actual)
				result := FormatValidationErrorWithExpectedActual(err, false, nil, ea)

				if !strings.Contains(result, tt.expectedInStr) {
					t.Errorf("Expected %v in result, got: %v", tt.expectedInStr, result)
				}
				if !strings.Contains(result, tt.actualInStr) {
					t.Errorf("Expected %v in result, got: %v", tt.actualInStr, result)
				}
			})
		}
	})
}
