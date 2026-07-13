package yamlutil

import (
	"testing"
)

// TestExtractTypeName tests the basic extractTypeName function with common patterns
func TestExtractTypeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Pattern 1: "cannot unmarshal !!<tag> into <type>"
		{
			name:     "unmarshal basic type",
			input:    "cannot unmarshal !!str into int",
			expected: "int",
		},
		{
			name:     "unmarshal array type",
			input:    "cannot unmarshal !!seq into []string",
			expected: "[]string",
		},
		{
			name:     "unmarshal map type",
			input:    "cannot unmarshal !!map into map[string]int",
			expected: "map[string]int",
		},
		{
			name:     "unmarshal with line prefix",
			input:    "line 10: cannot unmarshal !!str into int",
			expected: "int",
		},

		// Pattern 2: "expected <type>, got <type>"
		{
			name:     "expected got basic types",
			input:    "expected int, got string",
			expected: "int",
		},
		{
			name:     "expected got pointer type",
			input:    "expected *string, got string",
			expected: "*string",
		},
		{
			name:     "expected got array type",
			input:    "expected []bool, got int",
			expected: "[]bool",
		},

		// Pattern 3: "want <type>, got <type>"
		{
			name:     "want got basic types",
			input:    "want float64, got int",
			expected: "float64",
		},
		{
			name:     "want got struct type",
			input:    "want time.Time, got string",
			expected: "time.Time",
		},

		// Pattern 4: "<type> cannot be converted to <type>"
		{
			name:     "cannot be converted basic types",
			input:    "string cannot be converted to int",
			expected: "string",
		},
		{
			name:     "cannot be converted pointer type",
			input:    "int cannot be converted to *string",
			expected: "int",
		},

		// Pattern 5: Simple type name at start
		{
			name:     "simple type prefix",
			input:    "int: value cannot be parsed",
			expected: "int",
		},
		{
			name:     "package qualified type prefix",
			input:    "time.Time: cannot parse duration",
			expected: "time.Time",
		},

		// Pattern 6: Quoted type names
		{
			name:     "quoted type names",
			input:    `cannot unmarshal "string" into "int"`,
			expected: "string",
		},

		// Pattern 7: Type name after "into"
		{
			name:     "into pattern fallback",
			input:    "some error message into float64",
			expected: "float64",
		},

		// Edge cases: empty or invalid input
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no type match",
			input:    "this is an error without type information",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   \t\n  ",
			expected: "",
		},

		// Complex ARMOR-style errors
		{
			name:     "field path type mismatch",
			input:    "field server.port type mismatch: expected int, got string",
			expected: "int",
		},
		{
			name:     "field path with unmarshal",
			input:    "field items[0].name cannot unmarshal !!str into int",
			expected: "int",
		},

		// YAML prefix formats
		{
			name:     "yaml prefix format",
			input:    "yaml: line 15: cannot unmarshal !!seq into []string",
			expected: "[]string",
		},
		{
			name:     "yaml with column",
			input:    "line 10, column 5: cannot unmarshal !!str into int",
			expected: "int",
		},

		// Pattern 8: Type name at end after "expected"
		{
			name:     "expected type at end",
			input:    "invalid type, expected int",
			expected: "int",
		},
		{
			name:     "expected complex type at end",
			input:    "field type mismatch, expected []string",
			expected: "[]string",
		},
		{
			name:     "expected pointer type at end",
			input:    "cannot parse value, expected *string",
			expected: "*string",
		},
		{
			name:     "expected map type at end",
			input:    "conversion error, expected map[string]int",
			expected: "map[string]int",
		},

		// Pattern 9: Type name at end after "want"
		{
			name:     "want type at end",
			input:    "type mismatch, want string",
			expected: "string",
		},
		{
			name:     "want complex type at end",
			input:    "invalid conversion, want time.Time",
			expected: "time.Time",
		},
		{
			name:     "want interface type at end",
			input:    "cannot convert, want interface{}",
			expected: "interface{}",
		},

		// Pattern 10: Type name at end after "got" (actual type)
		{
			name:     "got type at end",
			input:    "parsing failed, got bool",
			expected: "bool",
		},
		{
			name:     "got complex type at end",
			input:    "unexpected value, got chan int",
			expected: "chan int",
		},

		// Pattern 11: Type name at end after "type"
		{
			name:     "type keyword at end",
			input:    "invalid value type float64",
			expected: "float64",
		},
		{
			name:     "type keyword with array at end",
			input:    "field has wrong type []int",
			expected: "[]int",
		},
		}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTypeName(tt.input)
			if result != tt.expected {
				t.Errorf("extractTypeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
