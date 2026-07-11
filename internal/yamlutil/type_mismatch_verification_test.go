package yamlutil

import (
	"fmt"
	"testing"
)

// TestTypeMismatchAcceptanceCriteria verifies that type mismatch errors
// meet all acceptance criteria for bead bf-325eg
func TestTypeMismatchAcceptanceCriteria(t *testing.T) {
	tests := []struct {
		name         string
		expectedType string
		actualType   string
		value        string
	}{
		{
			name:         "string vs integer",
			expectedType: "string",
			actualType:   "integer",
			value:        "123",
		},
		{
			name:         "integer vs string",
			expectedType: "integer",
			actualType:   "string",
			value:        "abc",
		},
		{
			name:         "array vs scalar",
			expectedType: "array",
			actualType:   "string",
			value:        "not an array",
		},
		{
			name:         "object vs scalar",
			expectedType: "object",
			actualType:   "integer",
			value:        "42",
		},
		{
			name:         "boolean vs string",
			expectedType: "boolean",
			actualType:   "string",
			value:        "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				"test.yaml",
				"test.field",
				tt.expectedType,
				tt.actualType,
				tt.value,
				10,
				"",
			)

			errorMsg := err.Error()

			// Acceptance Criteria 1: Type mismatch errors include expected type
			if !contains(errorMsg, tt.expectedType) {
				t.Errorf("Error message should include expected type %q, got: %s", tt.expectedType, errorMsg)
			}

			// Acceptance Criteria 2: Type mismatch errors include actual type
			if !contains(errorMsg, tt.actualType) {
				t.Errorf("Error message should include actual type %q, got: %s", tt.actualType, errorMsg)
			}

			// Acceptance Criteria 3: Format is "expected <type>, got <type>"
			expected := fmt.Sprintf("expected %s, got %s", tt.expectedType, tt.actualType)
			if !contains(errorMsg, expected) {
				t.Errorf("Error message should contain format %q, got: %s", expected, errorMsg)
			}

			// Verify the Context method also includes type information
			ctxMsg := err.Context()
			if !contains(ctxMsg, tt.expectedType) || !contains(ctxMsg, tt.actualType) {
				t.Errorf("Context should include both expected and actual types, got: %s", ctxMsg)
			}
		})
	}
}

// TestTypeMismatchComplexTypes verifies type mismatch works for complex types
func TestTypeMismatchComplexTypes(t *testing.T) {
	tests := []struct {
		name         string
		expectedType string
		actualType   string
		description  string
	}{
		{
			name:         "scalar types - string",
			expectedType: "string",
			actualType:   "integer",
			description:  "string expected but got integer",
		},
		{
			name:         "scalar types - integer",
			expectedType: "integer",
			actualType:   "boolean",
			description:  "integer expected but got boolean",
		},
		{
			name:         "scalar types - boolean",
			expectedType: "boolean",
			actualType:   "string",
			description:  "boolean expected but got string",
		},
		{
			name:         "array type",
			expectedType: "array",
			actualType:   "object",
			description:  "array expected but got object",
		},
		{
			name:         "object type",
			expectedType: "object",
			actualType:   "array",
			description:  "object expected but got array",
		},
		{
			name:         "nested array field",
			expectedType: "array",
			actualType:   "string",
			description:  "array expected in nested field but got string",
		},
		{
			name:         "nested object field",
			expectedType: "object",
			actualType:   "integer",
			description:  "object expected in nested field but got integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				"config.yaml",
				"field.nested.path",
				tt.expectedType,
				tt.actualType,
				"value",
				42,
				"",
			)

			errorMsg := err.Error()

			// Verify both types are present
			if !contains(errorMsg, tt.expectedType) {
				t.Errorf("Error should include expected type %q, got: %s", tt.expectedType, errorMsg)
			}
			if !contains(errorMsg, tt.actualType) {
				t.Errorf("Error should include actual type %q, got: %s", tt.actualType, errorMsg)
			}

			// Verify the standard format
			if !contains(errorMsg, "expected ") || !contains(errorMsg, ", got ") {
				t.Errorf("Error should use 'expected X, got Y' format, got: %s", errorMsg)
			}

			t.Logf("✓ %s: %s", tt.description, errorMsg)
		})
	}
}
