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

// TestExtractTypeNamePriority verifies that patterns are checked in the correct priority order
func TestExtractTypeNamePriority(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		reason   string
	}{
		{
			name:     "unmarshal pattern takes priority over expected pattern",
			input:    "cannot unmarshal !!str into int, expected string",
			expected: "int",
			reason:   "Pattern 1 (unmarshal) should match first",
		},
		{
			name:     "expected pattern matches when unmarshal not present",
			input:    "expected int, got string in parsing",
			expected: "int",
			reason:   "Pattern 2 (expected) should match when Pattern 1 doesn't",
		},
		{
			name:     "simple prefix matches when other patterns absent",
			input:    "string: some other error",
			expected: "string",
			reason:   "Pattern 5 (simple prefix) should match as fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTypeName(tt.input)
			if result != tt.expected {
				t.Errorf("%s: extractTypeName(%q) = %q, want %q. Reason: %s",
					tt.name, tt.input, result, tt.expected, tt.reason)
			}
		})
	}
}

// TestExtractTypeNameWithComplexTypes tests extraction of complex Go type signatures
func TestExtractTypeNameWithComplexTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "array type",
			input:    "cannot unmarshal !!seq into []string",
			expected: "[]string",
		},
		{
			name:     "array of arrays",
			input:    "cannot unmarshal !!seq into [][]int",
			expected: "[][]int",
		},
		{
			name:     "map type",
			input:    "cannot unmarshal !!map into map[string]int",
			expected: "map[string]int",
		},
		{
			name:     "pointer type",
			input:    "cannot unmarshal !!str into *string",
			expected: "*string",
		},
		{
			name:     "double pointer",
			input:    "cannot unmarshal !!str into **string",
			expected: "**string",
		},
		{
			name:     "channel type",
			input:    "cannot unmarshal !!str into chan int",
			expected: "chan int",
		},
		{
			name:     "interface type",
			input:    "cannot unmarshal !!str into interface{}",
			expected: "interface{}",
		},
		{
			name:     "fixed array",
			input:    "cannot unmarshal !!seq into [10]string",
			expected: "[10]string",
		},
		{
			name:     "package qualified type",
			input:    "cannot unmarshal !!str into time.Time",
			expected: "time.Time",
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

// TestExtractTypeNamePerformance benchmarks the extraction function
func BenchmarkExtractTypeName(b *testing.B) {
	inputs := []string{
		"cannot unmarshal !!str into int",
		"expected int, got string",
		"line 10: cannot unmarshal !!seq into []string",
		"field server.port type mismatch: expected int, got string",
		"string cannot be converted to int",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			extractTypeName(input)
		}
	}
}
