package yamlutil

import (
	"testing"
)

// TestExtractTypeNameBasicAtBeginning verifies type name extraction when
// type names appear at the beginning of error messages.
func TestExtractTypeNameBasicAtBeginning(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// YAML type tags at beginning
		{
			name:     "YAML tag at very start - !!str",
			input:    "!!str: cannot unmarshal",
			expected: "str",
		},
		{
			name:     "YAML tag at beginning - !!int",
			input:    "!!int into string failed",
			expected: "int",
		},
		{
			name:     "YAML tag at beginning - !!bool",
			input:    "!!bool cannot convert",
			expected: "bool",
		},
		{
			name:     "YAML tag at beginning - !!seq",
			input:    "!!seq is not valid here",
			expected: "seq",
		},
		{
			name:     "YAML tag at beginning - !!map",
			input:    "!!map expected but got something else",
			expected: "map",
		},

		// Go slice types at beginning
		{
			name:     "Slice type at beginning - []string",
			input:    "[]string cannot be converted",
			expected: "[]string",
		},
		{
			name:     "Slice type at beginning - []int",
			input:    "[]int is required",
			expected: "[]int",
		},
		{
			name:     "Slice type at beginning - []bool",
			input:    "[]bool was expected",
			expected: "[]bool",
		},

		// Go pointer types at beginning
		{
			name:     "Pointer type at beginning - *string",
			input:    "*string is invalid here",
			expected: "*string",
		},
		{
			name:     "Pointer type at beginning - *int",
			input:    "*int cannot be parsed",
			expected: "*int",
		},
		{
			name:     "Pointer type at beginning - *bool",
			input:    "*bool was expected",
			expected: "*bool",
		},

		// Go map types at beginning
		{
			name:     "Map type at beginning - map[string]int",
			input:    "map[string]int failed validation",
			expected: "map[string]int",
		},
		{
			name:     "Map type at beginning - map[int]string",
			input:    "map[int]string is required",
			expected: "map[int]string",
		},

		// Go basic types at beginning
		{
			name:     "Basic type at beginning - string",
			input:    "string cannot be converted",
			expected: "string",
		},
		{
			name:     "Basic type at beginning - int",
			input:    "int is invalid",
			expected: "int",
		},
		{
			name:     "Basic type at beginning - bool",
			input:    "bool was expected",
			expected: "bool",
		},
		{
			name:     "Basic type at beginning - float64",
			input:    "float64 failed",
			expected: "float64",
		},
		{
			name:     "Basic type at beginning - int64",
			input:    "int64 is not valid",
			expected: "int64",
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

// TestExtractTypeNameBasicInMiddle verifies type name extraction when
// type names appear in the middle of error messages.
func TestExtractTypeNameBasicInMiddle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// YAML type tags in middle
		{
			name:     "YAML tag in middle - !!str",
			input:    "cannot unmarshal !!str into int",
			expected: "str",
		},
		{
			name:     "YAML tag in middle - !!int",
			input:    "field error: cannot parse !!int as string",
			expected: "int",
		},
		{
			name:     "YAML tag in middle - !!bool",
			input:    "line 10: found !!bool where string expected",
			expected: "bool",
		},
		{
			name:     "YAML tag in middle - !!seq",
			input:    "validation failed: !!seq is not allowed here",
			expected: "seq",
		},
		{
			name:     "YAML tag in middle - !!map",
			input:    "error processing field: !!map cannot be converted",
			expected: "map",
		},

		// Go slice types in middle
		{
			name:     "Slice type in middle - []string",
			input:    "cannot convert []string to int",
			expected: "[]string",
		},
		{
			name:     "Slice type in middle - []int",
			input:    "field error: []int is invalid",
			expected: "[]int",
		},
		{
			name:     "Slice type in middle - []bool",
			input:    "line 15: found []bool where string expected",
			expected: "[]bool",
		},

		// Go pointer types in middle
		{
			name:     "Pointer type in middle - *string",
			input:    "cannot convert *string to int",
			expected: "*string",
		},
		{
			name:     "Pointer type in middle - *int",
			input:    "field error: *int is invalid",
			expected: "*int",
		},
		{
			name:     "Pointer type in middle - *bool",
			input:    "line 20: found *bool where string expected",
			expected: "*bool",
		},

		// Go map types in middle
		{
			name:     "Map type in middle - map[string]int",
			input:    "cannot convert map[string]int to string",
			expected: "map[string]int",
		},
		{
			name:     "Map type in middle - map[int]string",
			input:    "field error: map[int]string is invalid",
			expected: "map[int]string",
		},

		// Go basic types in middle
		{
			name:     "Basic type in middle - string",
			input:    "cannot convert string to int",
			expected: "string",
		},
		{
			name:     "Basic type in middle - int",
			input:    "field error: int is invalid",
			expected: "int",
		},
		{
			name:     "Basic type in middle - bool",
			input:    "line 25: found bool where string expected",
			expected: "bool",
		},
		{
			name:     "Basic type in middle - float64",
			input:    "cannot convert float64 to string",
			expected: "float64",
		},
		{
			name:     "Basic type in middle - int64",
			input:    "field error: int64 is invalid type",
			expected: "int64",
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

// TestExtractTypeNameBasicAtEnd verifies type name extraction when
// type names appear at the end of error messages.
func TestExtractTypeNameBasicAtEnd(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// YAML type tags at end
		{
			name:     "YAML tag at end - !!str",
			input:    "cannot unmarshal value into !!str",
			expected: "str",
		},
		{
			name:     "YAML tag at end - !!int",
			input:    "field error: expected int, got !!int",
			expected: "int",
		},
		{
			name:     "YAML tag at end - !!bool",
			input:    "line 10: conversion failed to !!bool",
			expected: "bool",
		},
		{
			name:     "YAML tag at end - !!seq",
			input:    "validation error: found !!seq",
			expected: "seq",
		},
		{
			name:     "YAML tag at end - !!map",
			input:    "error: cannot convert field to !!map",
			expected: "map",
		},

		// Go slice types at end
		{
			name:     "Slice type at end - []string",
			input:    "expected array, got []string",
			expected: "[]string",
		},
		{
			name:     "Slice type at end - []int",
			input:    "field error: type mismatch, found []int",
			expected: "[]int",
		},
		{
			name:     "Slice type at end - []bool",
			input:    "line 15: cannot parse value as []bool",
			expected: "[]bool",
		},

		// Go pointer types at end
		{
			name:     "Pointer type at end - *string",
			input:    "expected value, got *string",
			expected: "*string",
		},
		{
			name:     "Pointer type at end - *int",
			input:    "field error: type mismatch, found *int",
			expected: "*int",
		},
		{
			name:     "Pointer type at end - *bool",
			input:    "line 20: cannot parse value as *bool",
			expected: "*bool",
		},

		// Go map types at end
		{
			name:     "Map type at end - map[string]int",
			input:    "expected object, got map[string]int",
			expected: "map[string]int",
		},
		{
			name:     "Map type at end - map[int]string",
			input:    "field error: type mismatch, found map[int]string",
			expected: "map[int]string",
		},

		// Go basic types at end
		{
			name:     "Basic type at end - string",
			input:    "expected value, got string",
			expected: "string",
		},
		{
			name:     "Basic type at end - int",
			input:    "field error: type mismatch, found int",
			expected: "int",
		},
		{
			name:     "Basic type at end - bool",
			input:    "line 25: cannot parse value as bool",
			expected: "bool",
		},
		{
			name:     "Basic type at end - float64",
			input:    "expected number, got float64",
			expected: "float64",
		},
		{
			name:     "Basic type at end - int64",
			input:    "field error: type mismatch, found int64",
			expected: "int64",
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

// TestExtractTypeNameBasicSingleType verifies type name extraction from
// messages containing a single type name.
func TestExtractTypeNameBasicSingleType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Single YAML type tags
		{
			name:     "Single YAML tag - !!str only",
			input:    "cannot unmarshal !!str",
			expected: "str",
		},
		{
			name:     "Single YAML tag - !!int only",
			input:    "error parsing !!int",
			expected: "int",
		},
		{
			name:     "Single YAML tag - !!bool only",
			input:    "found !!bool",
			expected: "bool",
		},
		{
			name:     "Single YAML tag - !!seq only",
			input:    "unexpected type !!seq",
			expected: "seq",
		},
		{
			name:     "Single YAML tag - !!map only",
			input:    "invalid type !!map",
			expected: "map",
		},

		// Single Go slice types
		{
			name:     "Single slice type - []string only",
			input:    "cannot unmarshal []string",
			expected: "[]string",
		},
		{
			name:     "Single slice type - []int only",
			input:    "error parsing []int",
			expected: "[]int",
		},
		{
			name:     "Single slice type - []bool only",
			input:    "found []bool",
			expected: "[]bool",
		},

		// Single Go pointer types
		{
			name:     "Single pointer type - *string only",
			input:    "cannot unmarshal *string",
			expected: "*string",
		},
		{
			name:     "Single pointer type - *int only",
			input:    "error parsing *int",
			expected: "*int",
		},
		{
			name:     "Single pointer type - *bool only",
			input:    "found *bool",
			expected: "*bool",
		},

		// Single Go map types
		{
			name:     "Single map type - map[string]int only",
			input:    "cannot unmarshal map[string]int",
			expected: "map[string]int",
		},
		{
			name:     "Single map type - map[int]string only",
			input:    "error parsing map[int]string",
			expected: "map[int]string",
		},

		// Single Go basic types
		{
			name:     "Single basic type - string only",
			input:    "cannot unmarshal string",
			expected: "string",
		},
		{
			name:     "Single basic type - int only",
			input:    "error parsing int",
			expected: "int",
		},
		{
			name:     "Single basic type - bool only",
			input:    "found bool",
			expected: "bool",
		},
		{
			name:     "Single basic type - float64 only",
			input:    "cannot unmarshal float64",
			expected: "float64",
		},
		{
			name:     "Single basic type - int64 only",
			input:    "error parsing int64",
			expected: "int64",
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

// TestExtractTypeNameBasicMultipleTypes verifies type name extraction from
// messages containing multiple type names (should return the first match).
func TestExtractTypeNameBasicMultipleTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Multiple YAML type tags
		{
			name:     "Multiple YAML tags - !!str first",
			input:    "cannot unmarshal !!str into !!int",
			expected: "str",
		},
		{
			name:     "Multiple YAML tags - !!bool first",
			input:    "found !!bool where !!str expected",
			expected: "bool",
		},
		{
			name:     "Multiple YAML tags - !!seq first",
			input:    "!!seq is not compatible with !!map",
			expected: "seq",
		},
		{
			name:     "Multiple YAML tags - !!int first",
			input:    "error converting !!int to !!float",
			expected: "int",
		},

		// Multiple Go slice types
		{
			name:     "Multiple slice types - []string first",
			input:    "cannot convert []string to []int",
			expected: "[]string",
		},
		{
			name:     "Multiple slice types - []bool first",
			input:    "found []bool where []string expected",
			expected: "[]bool",
		},
		{
			name:     "Multiple slice types - []int first",
			input:    "[]int is not compatible with []float64",
			expected: "[]int",
		},

		// Multiple Go pointer types
		{
			name:     "Multiple pointer types - *string first",
			input:    "cannot convert *string to *int",
			expected: "*string",
		},
		{
			name:     "Multiple pointer types - *bool first",
			input:    "found *bool where *string expected",
			expected: "*bool",
		},
		{
			name:     "Multiple pointer types - *int first",
			input:    "*int is not compatible with *float64",
			expected: "*int",
		},

		// Multiple Go basic types
		{
			name:     "Multiple basic types - string first",
			input:    "cannot convert string to int",
			expected: "string",
		},
		{
			name:     "Multiple basic types - bool first",
			input:    "found bool where string expected",
			expected: "bool",
		},
		{
			name:     "Multiple basic types - int first",
			input:    "int is not compatible with float64",
			expected: "int",
		},
		{
			name:     "Multiple basic types - float64 first",
			input:    "cannot convert float64 to int64",
			expected: "float64",
		},

		// Mixed type categories
		{
			name:     "YAML tag and Go type - YAML tag wins",
			input:    "cannot unmarshal !!str into string",
			expected: "str",
		},
		{
			name:     "YAML tag and slice type - YAML tag wins",
			input:    "found !!bool where []int expected",
			expected: "bool",
		},
		{
			name:     "Slice and pointer types - slice wins",
			input:    "cannot convert []string to *int",
			expected: "[]string",
		},
		{
			name:     "Slice and basic types - slice wins",
			input:    "found []bool where string expected",
			expected: "[]bool",
		},
		{
			name:     "Pointer and basic types - pointer wins",
			input:    "cannot convert *int to string",
			expected: "*int",
		},
		{
			name:     "Map and slice types - slice wins (pattern order)",
			input:    "cannot convert map[string]int to []string",
			expected: "[]string",
		},

		// Three or more types
		{
			name:     "Three YAML tags - first wins",
			input:    "cannot unmarshal !!str into !!int or !!bool",
			expected: "str",
		},
		{
			name:     "Three Go types - first wins",
			input:    "cannot convert string to int or bool",
			expected: "string",
		},
		{
			name:     "Mixed types - first YAML tag wins",
			input:    "found !!str where string or int expected",
			expected: "str",
		},
		{
			name:     "Mixed types - first slice type wins",
			input:    "cannot convert []string to int or bool",
			expected: "[]string",
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

// TestExtractTypeNameBasicComplexMessages verifies type name extraction
// from complex, real-world error messages.
func TestExtractTypeNameBasicComplexMessages(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Complex error with line, column, and type",
			input:    "line 10, column 5: cannot unmarshal !!str into int",
			expected: "str",
		},
		{
			name:     "Complex error with field path and type",
			input:    "field server.port: cannot unmarshal !!int into string",
			expected: "int",
		},
		{
			name:     "Complex error with line number and slice type",
			input:    "yaml: line 15: cannot unmarshal []int into string",
			expected: "[]int",
		},
		{
			name:     "Complex error with field path and pointer type - YAML tag wins",
			input:    "field config.host: expected *string, got !!int",
			expected: "int",
		},
		{
			name:     "Complex error with column and map type",
			input:    "line 20, column 10: found map[string]int where string expected",
			expected: "map[string]int",
		},
		{
			name:     "Complex error with multiple elements - YAML tag",
			input:    "yaml: line 25: field items[0]: cannot unmarshal !!bool into int",
			expected: "bool",
		},
		{
			name:     "Complex error with array index and type",
			input:    "field items[0].name: cannot unmarshal !!str into string",
			expected: "str",
		},
		{
			name:     "Complex error with nested field path",
			input:    "field server.config.port: cannot unmarshal !!int into float64",
			expected: "int",
		},
		{
			name:     "Error with channel type",
			input:    "cannot unmarshal chan string into int",
			expected: "string",
		},
		{
			name:     "Error with directional channel - int matches first",
			input:    "cannot unmarshal chan<- int into string",
			expected: "int",
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
