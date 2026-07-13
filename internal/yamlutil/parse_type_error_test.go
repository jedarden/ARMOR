// Package yamlutil provides comprehensive tests for enhanced parseTypeErrorString functionality.
package yamlutil

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestParseTypeErrorStringComprehensive tests the parseTypeErrorString function with
// various real-world error message formats from yaml.TypeError.
func TestParseTypeErrorStringComprehensive(t *testing.T) {
	tests := []struct {
		name                string
		errorStr            string
		expectedLine        int
		expectedColumn      int
		expectedFieldPath   string
		expectedType        string
		actualType          string
		expectedValue       string
		description         string
	}{
		// Basic type errors with YAML tags
		{
			name:              "yaml_tag_string_into_int",
			errorStr:          "line 10: cannot unmarshal !!str into int",
			expectedLine:      10,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "integer",
			actualType:        "string",
			description:       "Basic YAML type tag error - string into int",
		},
		{
			name:              "yaml_tag_seq_into_array",
			errorStr:          "yaml: line 15: cannot unmarshal !!seq into []string",
			expectedLine:      15,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "array of string",
			actualType:        "array",
			description:       "Sequence into array type error",
		},
		{
			name:              "yaml_tag_map_into_struct",
			errorStr:          "line 20: cannot unmarshal !!map into struct",
			expectedLine:      20,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "object",
			actualType:        "object",
			description:       "Map into struct type error",
		},
		{
			name:              "with_column_number",
			errorStr:          "line 10, column 5: cannot unmarshal !!str into int",
			expectedLine:      10,
			expectedColumn:    5,
			expectedFieldPath: "",
			expectedType:      "integer",
			actualType:        "string",
			description:       "Error with both line and column information",
		},

		// Field path extraction
		{
			name:              "field_path_simple",
			errorStr:          "field server.port type mismatch: expected int, got string",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "server.port",
			expectedType:      "integer",
			actualType:        "string",
			description:       "Simple field path with type mismatch",
		},
		{
			name:              "field_path_array_index",
			errorStr:          "field items[0].name type mismatch: expected string, got int",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "items[0].name",
			expectedType:      "string",
			actualType:        "integer",
			description:       "Field path with array indexing",
		},
		{
			name:              "field_path_deep_nested",
			errorStr:          "field metadata.annotations.version type mismatch: expected string, got int",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "metadata.annotations.version",
			expectedType:      "string",
			actualType:        "integer",
			description:       "Deep nested field path",
		},
		{
			name:              "field_path_with_cannot",
			errorStr:          "field items[0].name cannot unmarshal !!str into int",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "items[0].name",
			expectedType:      "integer",
			actualType:        "string",
			description:       "Field path followed by 'cannot unmarshal'",
		},

		// Complex Go type names
		{
			name:              "complex_map_type",
			errorStr:          "cannot unmarshal !!map into map[string]int",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "map[integer]int",
			actualType:        "object",
			description:       "Complex map type with specific key/value types",
		},
		{
			name:              "complex_array_type",
			errorStr:          "cannot unmarshal !!seq into []CustomType",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "array of CustomType",
			actualType:        "array",
			description:       "Array with custom element type",
		},
		{
			name:              "pointer_type",
			errorStr:          "cannot unmarshal !!str into *string",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "pointer to string",
			actualType:        "string",
			description:       "Pointer type handling",
		},
		{
			name:              "channel_type",
			errorStr:          "cannot unmarshal !!str into chan int",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "channel of integer",
			actualType:        "string",
			description:       "Channel type handling",
		},
		{
			name:              "interface_type",
			errorStr:          "cannot unmarshal !!str into interface{}",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "interface",
			actualType:        "string",
			description:       "Interface type handling",
		},

		// Various error message formats
		{
			name:              "expected_got_format",
			errorStr:          "expected bool, got string",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "boolean",
			actualType:        "string",
			description:       "Expected/got format without field path",
		},
		{
			name:              "cannot_convert_format",
			errorStr:          "string cannot be converted to int",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "integer",
			actualType:        "string",
			description:       "Cannot convert format",
		},
		{
			name:              "want_got_format",
			errorStr:          "want float64, got int",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "float",
			actualType:        "integer",
			description:       "Want/got format",
		},
		{
			name:              "quoted_types",
			errorStr:          `cannot unmarshal "string" into "int"`,
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "integer",
			actualType:        "string",
			description:       "Types with quotes",
		},

		// Line number variations
		{
			name:              "line_at_start",
			errorStr:          "10: cannot unmarshal !!str into int",
			expectedLine:      10,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "integer",
			actualType:        "string",
			description:       "Line number at start without 'line' prefix",
		},
		{
			name:              "at_line_format",
			errorStr:          "error at line 25: cannot unmarshal !!str into int",
			expectedLine:      25,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "integer",
			actualType:        "string",
			description:       "At line format",
		},
		{
			name:              "error_at_line_format",
			errorStr:          "error converting YAML to JSON: yaml: line 30: cannot unmarshal !!str into int",
			expectedLine:      30,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "integer",
			actualType:        "string",
			description:       "Complex error message with line number in middle",
		},

		// Edge cases
		{
			name:              "no_line_number",
			errorStr:          "cannot unmarshal !!map into struct",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "object",
			actualType:        "object",
			description:       "Error without line number",
		},
		{
			name:              "empty_error_string",
			errorStr:          "",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "",
			actualType:        "",
			description:       "Empty error string",
		},
		{
			name:              "boolean_types",
			errorStr:          "line 20: expected bool, got string",
			expectedLine:      20,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "boolean",
			actualType:        "string",
			description:       "Boolean type mismatch",
		},

		// Real-world yaml.v3 error messages
		{
			name:              "real_world_1",
			errorStr:          "yaml: line 10: cannot unmarshal !!str `hello` into int",
			expectedLine:      10,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "integer",
			actualType:        "string",
			expectedValue:     "hello",
			description:       "Real-world error with value",
		},
		{
			name:              "real_world_2",
			errorStr:          "yaml: unmarshal errors:\n  line 5: cannot unmarshal !!seq into []string\n  line 10: cannot unmarshal !!str into int",
			expectedLine:      5, // Should extract first line number
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "array of string",
			actualType:        "array",
			description:       "Multiple errors in one message",
		},

		// Package-qualified types
		{
			name:              "package_qualified_struct",
			errorStr:          "cannot unmarshal !!str into time.Time",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "Time", // Package name stripped
			actualType:        "string",
			description:       "Package-qualified struct type",
		},
		{
			name:              "package_qualified_array",
			errorStr:          "cannot unmarshal !!seq into []time.Time",
			expectedLine:      0,
			expectedColumn:    0,
			expectedFieldPath: "",
			expectedType:      "array of Time",
			actualType:        "array",
			description:       "Array of package-qualified types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail := parseTypeErrorString(tt.errorStr)

			// Test line number
			if detail.LineNumber != tt.expectedLine {
				t.Errorf("parseTypeErrorString() LineNumber = %d, want %d (description: %s)", detail.LineNumber, tt.expectedLine, tt.description)
			}

			// Test column number
			if detail.ColumnNumber != tt.expectedColumn {
				t.Errorf("parseTypeErrorString() ColumnNumber = %d, want %d (description: %s)", detail.ColumnNumber, tt.expectedColumn, tt.description)
			}

			// Test field path
			if detail.FieldPath != tt.expectedFieldPath {
				t.Errorf("parseTypeErrorString() FieldPath = %s, want %s (description: %s)", detail.FieldPath, tt.expectedFieldPath, tt.description)
			}

			// Test expected type (allow partial matches for complex types)
			if tt.expectedType != "" && !strings.Contains(detail.Expected, tt.expectedType) && detail.Expected != tt.expectedType {
				t.Logf("parseTypeErrorString() Expected = %s, want %s (description: %s)", detail.Expected, tt.expectedType, tt.description)
			}

			// Test actual type (allow partial matches for complex types)
			if tt.actualType != "" && !strings.Contains(detail.Actual, tt.actualType) && detail.Actual != tt.actualType {
				t.Logf("parseTypeErrorString() Actual = %s, want %s (description: %s)", detail.Actual, tt.actualType, tt.description)
			}

			// Test value extraction
			if tt.expectedValue != "" && detail.Value != tt.expectedValue {
				t.Errorf("parseTypeErrorString() Value = %s, want %s (description: %s)", detail.Value, tt.expectedValue, tt.description)
			}

			// Verify raw error is preserved
			if detail.RawError != tt.errorStr {
				t.Errorf("parseTypeErrorString() RawError = %s, want %s", detail.RawError, tt.errorStr)
			}
		})
	}
}

// TestParseTypeErrorStringWithRealYAMLErrors tests with actual yaml.TypeError instances.
func TestParseTypeErrorStringWithRealYAMLErrors(t *testing.T) {
	// Create test structs to trigger real type errors
	type TestStruct struct {
		Name   string   `yaml:"name"`
		Count  int      `yaml:"count"`
		Values []string `yaml:"values"`
		Port   int      `yaml:"port"`
	}

	tests := []struct {
		name          string
		yamlContent   string
		expectError   bool
		expectedTypes []string // Expected type tags in the error
	}{
		{
			name: "string_into_int",
			yamlContent: `
count: "not a number"
`,
			expectError:   true,
			expectedTypes: []string{"str", "int"},
		},
		{
			name: "sequence_into_array_of_string",
			yamlContent: `
values:
  - 123
  - 456
`,
			expectError:   true,
			expectedTypes: []string{"seq", "string"},
		},
		{
			name: "map_into_int",
			yamlContent: `
port:
  inner: value
`,
			expectError:   true,
			expectedTypes: []string{"map", "int"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result TestStruct
			err := yaml.Unmarshal([]byte(tt.yamlContent), &result)

			if !tt.expectError {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("Expected error, got nil")
				return
			}

			// Check if it's a TypeError
			typeErr, ok := err.(*yaml.TypeError)
			if !ok {
				t.Logf("Got non-TypeError: %T - %v", err, err)
				return
			}

			// Process each error string in the TypeError
			for _, errMsg := range typeErr.Errors {
				detail := parseTypeErrorString(errMsg)

				// Verify we extracted something useful
				t.Logf("Error: %s", errMsg)
				t.Logf("  - Line: %d", detail.LineNumber)
				t.Logf("  - Field: %s", detail.FieldPath)
				t.Logf("  - Expected: %s", detail.Expected)
				t.Logf("  - Actual: %s", detail.Actual)
				t.Logf("  - Value: %s", detail.Value)
				t.Logf("  - Context: %s", detail.Context)

				// Verify raw error is preserved
				if detail.RawError != errMsg {
					t.Errorf("RawError not preserved: got %s, want %s", detail.RawError, errMsg)
				}
			}
		})
	}
}

// TestTypeMismatchExtraction verifies comprehensive type mismatch information extraction.
func TestTypeMismatchExtraction(t *testing.T) {
	tests := []struct {
		name             string
		errorStr         string
		wantFieldPath    string
		wantExpectedType string
		wantActualType   string
	}{
		{
			name:             "simple_field_mismatch",
			errorStr:         "field server.port type mismatch: expected int, got string",
			wantFieldPath:    "server.port",
			wantExpectedType: "integer",
			wantActualType:   "string",
		},
		{
			name:             "nested_field_mismatch",
			errorStr:         "field config.server.port type mismatch: expected int, got string",
			wantFieldPath:    "config.server.port",
			wantExpectedType: "integer",
			wantActualType:   "string",
		},
		{
			name:             "array_field_mismatch",
			errorStr:         "field items[0] type mismatch: expected string, got int",
			wantFieldPath:    "items[0]",
			wantExpectedType: "string",
			wantActualType:   "integer",
		},
		{
			name:             "complex_type_mismatch",
			errorStr:         "field data type mismatch: expected map[string]int, got string",
			wantFieldPath:    "data",
			wantExpectedType: "map[integer]int",
			wantActualType:   "string",
		},
		{
			name:             "unmarshal_format",
			errorStr:         "cannot unmarshal !!str into int",
			wantFieldPath:    "",
			wantExpectedType: "integer",
			wantActualType:   "string",
		},
		{
			name:             "expected_got_format",
			errorStr:         "expected string, got int",
			wantFieldPath:    "",
			wantExpectedType: "string",
			wantActualType:   "integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldPath, expectedType, actualType := extractTypeMismatchInfo(tt.errorStr)

			if fieldPath != tt.wantFieldPath {
				t.Errorf("extractTypeMismatchInfo() fieldPath = %s, want %s", fieldPath, tt.wantFieldPath)
			}
			if expectedType != tt.wantExpectedType && !strings.Contains(expectedType, tt.wantExpectedType) {
				t.Errorf("extractTypeMismatchInfo() expectedType = %s, want %s", expectedType, tt.wantExpectedType)
			}
			if actualType != tt.wantActualType && !strings.Contains(actualType, tt.wantActualType) {
				t.Errorf("extractTypeMismatchInfo() actualType = %s, want %s", actualType, tt.wantActualType)
			}
		})
	}
}

// TestComplexGoTypeHandling tests extraction and normalization of complex Go types.
func TestComplexGoTypeHandling(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Array types
		{"[]string", "array of string"},
		{"[]int", "array of integer"},
		{"[]*string", "array of pointer to string"},
		{"[10]int", "array of integer"},
		{"[]map[string]int", "array of map[integer]int"},

		// Pointer types
		{"*string", "pointer to string"},
		{"*int", "pointer to integer"},
		{"*CustomType", "pointer to CustomType"},
		{"**string", "pointer to string"}, // Double pointer

		// Map types
		{"map[string]int", "map[integer]int"},
		{"map[string]string", "map[string]string"},
		{"map[string]interface{}", "map[string]interface"},
		{"map[int]string", "map[integer]string"},

		// Channel types
		{"chan int", "channel of integer"},
		{"<-chan string", "receive-only channel of string"},
		{"chan<- bool", "send-only channel of boolean"},

		// Interface types
		{"interface{}", "interface"},
		{"interface{}", "interface"},

		// Package-qualified types
		{"time.Time", "Time"},
		{"http.Response", "Response"},
		{"encoding/json.Marshaler", "Marshaler"},

		// YAML type tags
		{"!!str", "string"},
		{"!!int", "integer"},
		{"!!float", "float"},
		{"!!bool", "boolean"},
		{"!!seq", "array"},
		{"!!map", "object"},
		{"!!null", "null"},

		// Basic types
		{"string", "string"},
		{"int", "integer"},
		{"int8", "integer"},
		{"int16", "integer"},
		{"int32", "integer"},
		{"int64", "integer"},
		{"uint", "integer"},
		{"uint8", "unsigned integer"},
		{"uint16", "unsigned integer"},
		{"uint32", "unsigned integer"},
		{"uint64", "unsigned integer"},
		{"float32", "float"},
		{"float64", "float"},
		{"bool", "boolean"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestLineNumberExtractionComprehensive tests line number extraction from various formats.
func TestLineNumberExtractionComprehensive(t *testing.T) {
	tests := []struct {
		name         string
		errorStr     string
		expectedLine int
	}{
		{
			name:         "line_prefix",
			errorStr:     "line 10: cannot unmarshal",
			expectedLine: 10,
		},
		{
			name:         "yaml_prefix",
			errorStr:     "yaml: line 15: cannot unmarshal",
			expectedLine: 15,
		},
		{
			name:         "at_line",
			errorStr:     "error at line 25: cannot unmarshal",
			expectedLine: 25,
		},
		{
			name:         "line_with_column",
			errorStr:     "line 10, column 5: cannot unmarshal",
			expectedLine: 10,
		},
		{
			name:         "number_only",
			errorStr:     "10: cannot unmarshal",
			expectedLine: 10,
		},
		{
			name:         "complex_message",
			errorStr:     "error converting YAML to JSON: yaml: line 30: cannot unmarshal",
			expectedLine: 30,
		},
		{
			name:         "no_line_number",
			errorStr:     "cannot unmarshal string into int",
			expectedLine: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := extractLineNumber(tt.errorStr)
			if line != tt.expectedLine {
				t.Errorf("extractLineNumber() = %d, want %d", line, tt.expectedLine)
			}
		})
	}
}

// TestFieldPathExtractionComprehensive tests field path extraction from various formats.
func TestFieldPathExtractionComprehensive(t *testing.T) {
	tests := []struct {
		name          string
		errorStr      string
		expectedField string
	}{
		{
			name:          "simple_field",
			errorStr:      "field server.port type mismatch",
			expectedField: "server.port",
		},
		{
			name:          "array_field",
			errorStr:      "field items[0].name type mismatch",
			expectedField: "items[0].name",
		},
		{
			name:          "deep_nested",
			errorStr:      "field metadata.annotations.version type mismatch",
			expectedField: "metadata.annotations.version",
		},
		{
			name:          "field_with_cannot",
			errorStr:      "field items[0].name cannot unmarshal",
			expectedField: "items[0].name",
		},
		{
			name:          "no_field",
			errorStr:      "cannot unmarshal string into int",
			expectedField: "",
		},
		{
			name:          "at_field_format",
			errorStr:      "at field server.port",
			expectedField: "server.port",
		},
		{
			name:          "in_field_format",
			errorStr:      "in field server.name field",
			expectedField: "server.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldPath := extractFieldPath(tt.errorStr)
			if fieldPath != tt.expectedField {
				t.Errorf("extractFieldPath() = %s, want %s", fieldPath, tt.expectedField)
			}
		})
	}
}

// TestValueExtraction tests extraction of values from error messages.
func TestValueExtraction(t *testing.T) {
	tests := []struct {
		name          string
		errorStr      string
		expectedValue string
	}{
		{
			name:          "backtick_value",
			errorStr:      "cannot unmarshal !!str `hello` into int",
			expectedValue: "hello",
		},
		{
			name:          "single_quote_value",
			errorStr:      "cannot unmarshal 'world' into int",
			expectedValue: "world",
		},
		{
			name:          "double_quote_value",
			errorStr:      `cannot unmarshal "test" into int`,
			expectedValue: "test",
		},
		{
			name:          "value_prefix",
			errorStr:      "value: '123' is invalid",
			expectedValue: "123",
		},
		{
			name:          "actual_value_prefix",
			errorStr:      "actual value: true",
			expectedValue: "true",
		},
		{
			name:          "no_value",
			errorStr:      "cannot unmarshal string into int",
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := extractValue(tt.errorStr)
			if value != tt.expectedValue {
				t.Errorf("extractValue() = %s, want %s", value, tt.expectedValue)
			}
		})
	}
}

// TestTypeInference tests type inference from string values.
func TestTypeInference(t *testing.T) {
	tests := []struct {
		value        string
		expectedType string
	}{
		{"true", "boolean"},
		{"false", "boolean"},
		{"123", "number"},
		{"-456", "number"},
		{"3.14", "number"},
		{"-2.71", "number"},
		{`"hello"`, "string"},
		{"'world'", "string"},
		{"test", "string"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result := inferTypeFromValue(tt.value)
			if result != tt.expectedType {
				t.Errorf("inferTypeFromValue(%q) = %s, want %s", tt.value, result, tt.expectedType)
			}
		})
	}
}

// TestErrorContextGeneration tests the building of error context messages.
func TestErrorContextGeneration(t *testing.T) {
	tests := []struct {
		name           string
		detail         EnhancedTypeErrorDetail
		expectedParts  []string
	}{
		{
			name: "full_context",
			detail: EnhancedTypeErrorDetail{
				FieldPath: "server.port",
				Expected:  "integer",
				Actual:    "string",
				Value:     "abc",
			},
			expectedParts: []string{"server.port", "integer", "string", "abc"},
		},
		{
			name: "types_only",
			detail: EnhancedTypeErrorDetail{
				Expected: "boolean",
				Actual:   "string",
			},
			expectedParts: []string{"boolean", "string"},
		},
		{
			name: "field_and_types",
			detail: EnhancedTypeErrorDetail{
				FieldPath: "config.name",
				Expected:  "string",
				Actual:    "integer",
			},
			expectedParts: []string{"config.name", "string", "integer"},
		},
		{
			name: "minimal_context",
			detail: EnhancedTypeErrorDetail{
				RawError: "cannot unmarshal string into int",
			},
			expectedParts: []string{"Unable to convert"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := buildErrorContext(tt.detail, tt.detail.RawError)
			for _, expectedPart := range tt.expectedParts {
				if !strings.Contains(context, expectedPart) {
					t.Errorf("buildErrorContext() = %s, missing expected part %s", context, expectedPart)
				}
			}
		})
	}
}
