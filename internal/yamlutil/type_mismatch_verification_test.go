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

// TestExpectedIntegerGotBoolean verifies type mismatch errors for integer fields receiving boolean values
func TestExpectedIntegerGotBoolean(t *testing.T) {
	tests := []struct {
		name         string
		fieldPath    string
		booleanValue string
		line         int
	}{
		{
			name:         "boolean true in port field",
			fieldPath:    "server.port",
			booleanValue: "true",
			line:         10,
		},
		{
			name:         "boolean false in count field",
			fieldPath:    "items.count",
			booleanValue: "false",
			line:         15,
		},
		{
			name:         "boolean true in timeout field",
			fieldPath:    "config.timeout",
			booleanValue: "true",
			line:         20,
		},
		{
			name:         "boolean false in size field",
			fieldPath:    "buffer.size",
			booleanValue: "false",
			line:         25,
		},
		{
			name:         "boolean true in limit field",
			fieldPath:    "rate.limit",
			booleanValue: "true",
			line:         30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				"config.yaml",
				tt.fieldPath,
				"integer",
				"boolean",
				tt.booleanValue,
				tt.line,
				"",
			)

			errorMsg := err.Error()
			ctxMsg := err.Context()

			// Verify error message contains expected and actual types
			if !contains(errorMsg, "integer") {
				t.Errorf("Error should mention expected integer type, got: %s", errorMsg)
			}
			if !contains(errorMsg, "boolean") {
				t.Errorf("Error should mention actual boolean type, got: %s", errorMsg)
			}

			// Verify field path is included
			if !contains(errorMsg, tt.fieldPath) {
				t.Errorf("Error should include field path %q, got: %s", tt.fieldPath, errorMsg)
			}

			// Verify line number is included
			if tt.line > 0 && !contains(errorMsg, fmt.Sprintf("%d", tt.line)) {
				t.Errorf("Error should include line number %d, got: %s", tt.line, errorMsg)
			}

			// Verify context method also includes type information
			if !contains(ctxMsg, "integer") || !contains(ctxMsg, "boolean") {
				t.Errorf("Context should include both integer and boolean types, got: %s", ctxMsg)
			}

			t.Logf("✓ %s: %s", tt.name, errorMsg)
		})
	}
}

// TestExpectedStringGotNumber verifies type mismatch errors for string fields receiving numeric values
func TestExpectedStringGotNumber(t *testing.T) {
	tests := []struct {
		name         string
		fieldPath    string
		numericValue string
		valueType    string
		line         int
	}{
		{
			name:         "integer in name field",
			fieldPath:    "server.name",
			numericValue: "123",
			valueType:    "integer",
			line:         10,
		},
		{
			name:         "integer in hostname field",
			fieldPath:    "database.hostname",
			numericValue: "8080",
			valueType:    "integer",
			line:         15,
		},
		{
			name:         "float in label field",
			fieldPath:    "resource.label",
			numericValue: "3.14",
			valueType:    "float",
			line:         20,
		},
		{
			name:         "integer in description field",
			fieldPath:    "service.description",
			numericValue: "42",
			valueType:    "integer",
			line:         25,
		},
		{
			name:         "integer in path field",
			fieldPath:    "config.path",
			numericValue: "999",
			valueType:    "integer",
			line:         30,
		},
		{
			name:         "float in version field",
			fieldPath:    "app.version",
			numericValue: "1.0",
			valueType:    "float",
			line:         35,
		},
		{
			name:         "zero in id field",
			fieldPath:    "user.id",
			numericValue: "0",
			valueType:    "integer",
			line:         40,
		},
		{
			name:         "integer in title field",
			fieldPath:    "page.title",
			numericValue: "100",
			valueType:    "integer",
			line:         45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				"config.yaml",
				tt.fieldPath,
				"string",
				tt.valueType,
				tt.numericValue,
				tt.line,
				"",
			)

			errorMsg := err.Error()
			ctxMsg := err.Context()

			// Verify error message contains expected and actual types
			if !contains(errorMsg, "string") {
				t.Errorf("Error should mention expected string type, got: %s", errorMsg)
			}
			if !contains(errorMsg, tt.valueType) {
				t.Errorf("Error should mention actual %s type, got: %s", tt.valueType, errorMsg)
			}

			// Verify field path is included
			if !contains(errorMsg, tt.fieldPath) {
				t.Errorf("Error should include field path %q, got: %s", tt.fieldPath, errorMsg)
			}

			// Verify context method also includes type information
			if !contains(ctxMsg, "string") || !contains(ctxMsg, tt.valueType) {
				t.Errorf("Context should include both string and %s types, got: %s", tt.valueType, ctxMsg)
			}

			t.Logf("✓ %s: %s", tt.name, errorMsg)
		})
	}
}

// TestExpectedSliceGotScalar verifies type mismatch errors for array/slice fields receiving scalar values
func TestExpectedSliceGotScalar(t *testing.T) {
	tests := []struct {
		name         string
		fieldPath    string
		actualType   string
		actualValue  string
		line         int
	}{
		{
			name:        "integer in servers field",
			fieldPath:   "servers",
			actualType:  "integer",
			actualValue: "42",
			line:        10,
		},
		{
			name:        "string in hosts field",
			fieldPath:   "hosts",
			actualType:  "string",
			actualValue: "single",
			line:        15,
		},
		{
			name:        "boolean in ports field",
			fieldPath:   "ports",
			actualType:  "boolean",
			actualValue: "true",
			line:        20,
		},
		{
			name:        "float in rates field",
			fieldPath:   "rates",
			actualType:  "float",
			actualValue: "3.14",
			line:        25,
		},
		{
			name:        "null in items field",
			fieldPath:   "items",
			actualType:  "null",
			actualValue: "null",
			line:        30,
		},
		{
			name:        "integer zero in tags field",
			fieldPath:   "tags",
			actualType:  "integer",
			actualValue: "0",
			line:        35,
		},
		{
			name:        "integer one in keys field",
			fieldPath:   "keys",
			actualType:  "integer",
			actualValue: "1",
			line:        40,
		},
		{
			name:        "boolean false in values field",
			fieldPath:   "values",
			actualType:  "boolean",
			actualValue: "false",
			line:        45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				"config.yaml",
				tt.fieldPath,
				"array",
				tt.actualType,
				tt.actualValue,
				tt.line,
				"",
			)

			errorMsg := err.Error()
			ctxMsg := err.Context()

			// Verify error message contains expected and actual types
			if !contains(errorMsg, "array") && !contains(errorMsg, "slice") {
				t.Errorf("Error should mention expected array/slice type, got: %s", errorMsg)
			}
			if !contains(errorMsg, tt.actualType) {
				t.Errorf("Error should mention actual %s type, got: %s", tt.actualType, errorMsg)
			}

			// Verify field path is included
			if !contains(errorMsg, tt.fieldPath) {
				t.Errorf("Error should include field path %q, got: %s", tt.fieldPath, errorMsg)
			}

			// Verify context method also includes type information
			if !contains(ctxMsg, "array") && !contains(ctxMsg, "slice") {
				t.Errorf("Context should include array/slice type, got: %s", ctxMsg)
			}
			if !contains(ctxMsg, tt.actualType) {
				t.Errorf("Context should include actual %s type, got: %s", tt.actualType, ctxMsg)
			}

			t.Logf("✓ %s: %s", tt.name, errorMsg)
		})
	}
}

// TestCompatibleButWrongTypes verifies edge cases where types appear compatible but are semantically wrong
func TestCompatibleButWrongTypes(t *testing.T) {
	tests := []struct {
		name         string
		fieldPath    string
		expectedType string
		actualType   string
		value        string
		line         int
		description  string
	}{
		{
			name:         "float where integer expected",
			fieldPath:    "port",
			expectedType: "integer",
			actualType:   "float",
			value:        "3.14",
			line:         10,
			description:  "Float with decimal part where integer needed",
		},
		{
			name:         "integer where float expected",
			fieldPath:    "rate",
			expectedType: "float",
			actualType:   "integer",
			value:        "42",
			line:         15,
			description:  "Integer where float was expected",
		},
		{
			name:         "whole number float as integer",
			fieldPath:    "count",
			expectedType: "integer",
			actualType:   "float",
			value:        "1.0",
			line:         20,
			description:  "Whole number formatted as float",
		},
		{
			name:         "zero as float",
			fieldPath:    "timeout",
			expectedType: "float",
			actualType:   "integer",
			value:        "0",
			line:         25,
			description:  "Zero integer where float expected",
		},
		{
			name:         "truthy integer as boolean",
			fieldPath:    "enabled",
			expectedType: "boolean",
			actualType:   "integer",
			value:        "1",
			line:         30,
			description:  "Integer 1 mistaken for boolean true",
		},
		{
			name:         "falsy integer as boolean",
			fieldPath:    "active",
			expectedType: "boolean",
			actualType:   "integer",
			value:        "0",
			line:         35,
			description:  "Integer 0 mistaken for boolean false",
		},
		{
			name:         "negative integer as boolean",
			fieldPath:    "flag",
			expectedType: "boolean",
			actualType:   "integer",
			value:        "-1",
			line:         40,
			description:  "Negative integer where boolean expected",
		},
		{
			name:         "non-zero integer as boolean",
			fieldPath:    "set",
			expectedType: "boolean",
			actualType:   "integer",
			value:        "2",
			line:         45,
			description:  "Integer 2 where boolean expected",
		},
		{
			name:         "numeric string where integer expected",
			fieldPath:    "port",
			expectedType: "integer",
			actualType:   "string",
			value:        "\"8080\"",
			line:         50,
			description:  "String containing numeric value",
		},
		{
			name:         "numeric string where float expected",
			fieldPath:    "rate",
			expectedType: "float",
			actualType:   "string",
			value:        "\"3.14\"",
			line:         55,
			description:  "String containing float value",
		},
		{
			name:         "integer string where integer expected",
			fieldPath:    "count",
			expectedType: "integer",
			actualType:   "string",
			value:        "\"42\"",
			line:         60,
			description:  "String containing integer value",
		},
		{
			name:         "boolean true as integer",
			fieldPath:    "size",
			expectedType: "integer",
			actualType:   "boolean",
			value:        "true",
			line:         65,
			description:  "Boolean true where integer expected",
		},
		{
			name:         "boolean false as integer",
			fieldPath:    "limit",
			expectedType: "integer",
			actualType:   "boolean",
			value:        "false",
			line:         70,
			description:  "Boolean false where integer expected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				"config.yaml",
				tt.fieldPath,
				tt.expectedType,
				tt.actualType,
				tt.value,
				tt.line,
				"",
			)

			errorMsg := err.Error()
			ctxMsg := err.Context()

			// Verify error message contains expected and actual types
			if !contains(errorMsg, tt.expectedType) {
				t.Errorf("Error should mention expected %s type, got: %s", tt.expectedType, errorMsg)
			}
			if !contains(errorMsg, tt.actualType) {
				t.Errorf("Error should mention actual %s type, got: %s", tt.actualType, errorMsg)
			}

			// Verify field path is included
			if !contains(errorMsg, tt.fieldPath) {
				t.Errorf("Error should include field path %q, got: %s", tt.fieldPath, errorMsg)
			}

			// Verify context method also includes type information
			if !contains(ctxMsg, tt.expectedType) || !contains(ctxMsg, tt.actualType) {
				t.Errorf("Context should include both %s and %s types, got: %s",
					tt.expectedType, tt.actualType, ctxMsg)
			}

			t.Logf("✓ %s (%s): %s", tt.name, tt.description, errorMsg)
		})
	}
}
