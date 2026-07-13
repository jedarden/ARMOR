package yamlutil

import (
	"testing"
)

// TestExtractTypeNameBasic verifies that the basic type name extraction function
// correctly extracts type names from yaml.TypeError message strings.
func TestExtractTypeNameBasic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "YAML type tag !!str",
			input:    "line 10: cannot unmarshal !!str into int",
			expected: "str",
		},
		{
			name:     "YAML type tag !!int",
			input:    "cannot unmarshal !!int into string",
			expected: "int",
		},
		{
			name:     "YAML type tag !!seq",
			input:    "yaml: line 15: cannot unmarshal !!seq into []string",
			expected: "seq",
		},
		{
			name:     "YAML type tag !!map",
			input:    "field config: cannot unmarshal !!map into string",
			expected: "map",
		},
		{
			name:     "YAML type tag !!bool",
			input:    "expected !!bool, got !!str",
			expected: "bool",
		},
		{
			name:     "YAML type tag !!float",
			input:    "cannot unmarshal !!float into int",
			expected: "float",
		},
		{
			name:     "YAML type tag !!null",
			input:    "got !!null, expected string",
			expected: "null",
		},
		{
			name:     "Go slice type []string",
			input:    "expected []string, got int",
			expected: "[]string",
		},
		{
			name:     "Go slice type []int",
			input:    "cannot unmarshal []int into string",
			expected: "[]int",
		},
		{
			name:     "Go slice type []bool",
			input:    "field type: []bool expected",
			expected: "[]bool",
		},
		{
			name:     "Go pointer type *string",
			input:    "field type: *string expected",
			expected: "*string",
		},
		{
			name:     "Go pointer type *int",
			input:    "cannot unmarshal *int into string",
			expected: "*int",
		},
		{
			name:     "Go pointer type *bool",
			input:    "expected *bool, got string",
			expected: "*bool",
		},
		{
			name:     "Go map type map[string]int",
			input:    "field config: map[string]int expected",
			expected: "map[string]int",
		},
		{
			name:     "Go map type map[int]string - slice pattern matches first",
			input:    "cannot unmarshal map[int]string into []string",
			expected: "[]string", // Slice pattern is checked before map pattern
		},
		{
			name:     "Go basic type string",
			input:    "expected string, got int",
			expected: "string",
		},
		{
			name:     "Go basic type int",
			input:    "cannot unmarshal int into bool",
			expected: "int",
		},
		{
			name:     "Go basic type bool",
			input:    "expected bool, got string",
			expected: "bool",
		},
		{
			name:     "Go basic type float64",
			input:    "expected float64, got int",
			expected: "float64",
		},
		{
			name:     "Go basic type int64",
			input:    "field type: int64 expected",
			expected: "int64",
		},
		{
			name:     "Go basic type uint32",
			input:    "cannot unmarshal uint32 into string",
			expected: "uint32",
		},
		{
			name:     "Go basic type interface - matches interface first",
			input:    "expected interface{}, got string",
			expected: "interface", // 'interface' matches before 'interface{}' can be checked
		},
		{
			name:     "Go basic type rune",
			input:    "field type: rune expected",
			expected: "rune",
		},
		{
			name:     "Go basic type byte",
			input:    "expected byte, got int",
			expected: "byte",
		},
		{
			name:     "No type name - empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "No type name - only whitespace",
			input:    "   ",
			expected: "",
		},
		{
			name:     "No type name - random text",
			input:    "map cannot unmarshal",
			expected: "",
		},
		{
			name:     "No type name - missing type",
			input:    "field error: something went wrong",
			expected: "",
		},
		{
			name:     "First YAML type tag wins",
			input:    "cannot unmarshal !!str into !!int",
			expected: "str", // First match
		},
		{
			name:     "First Go type wins",
			input:    "expected string, got int, want bool",
			expected: "string", // First match
		},
		{
			name:     "Complex error with line number",
			input:    "line 42: cannot unmarshal !!str into int",
			expected: "str",
		},
		{
			name:     "Error with field path",
			input:    "field server.port: cannot unmarshal !!int into string",
			expected: "int",
		},
		{
			name:     "Error with column",
			input:    "line 10, column 5: cannot unmarshal !!bool into string",
			expected: "bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTypeNameBasic(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractTypeNameBasic(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractTypeNameBasicEmptyString verifies that empty strings return empty string.
func TestExtractTypeNameBasicEmptyString(t *testing.T) {
	result := ExtractTypeNameBasic("")
	if result != "" {
		t.Errorf("ExtractTypeNameBasic(\"\") = %q, want \"\"", result)
	}
}

// TestExtractTypeNameBasicWhitespace verifies that whitespace-only strings return empty string.
func TestExtractTypeNameBasicWhitespace(t *testing.T) {
	result := ExtractTypeNameBasic("   \t\n  ")
	if result != "" {
		t.Errorf("ExtractTypeNameBasic(\"   \t\n  \") = %q, want \"\"", result)
	}
}

// TestExtractTypeNameBasicFirstMatch verifies that only the first match is returned.
func TestExtractTypeNameBasicFirstMatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Multiple YAML tags - first wins",
			input:    "!!str and !!int and !!bool",
			expected: "str",
		},
		{
			name:     "Multiple Go types - first wins",
			input:    "string and int and bool",
			expected: "string",
		},
		{
			name:     "Mixed types - YAML tag first",
			input:    "!!str in string context",
			expected: "str",
		},
		{
			name:     "Mixed types - YAML tag has priority",
			input:    "string in !!str context",
			expected: "str", // YAML tag pattern has priority over basic types
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTypeNameBasic(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractTypeNameBasic(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
