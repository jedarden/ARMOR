package yamlutil

import (
	"testing"
)

// TestNormalizeYAMLTypeBasicTypes verifies normalization of basic Go types.
func TestNormalizeYAMLTypeBasicTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// String types
		{
			name:     "string type",
			input:    "string",
			expected: "string",
		},

		// Integer types
		{
			name:     "int type",
			input:    "int",
			expected: "integer",
		},
		{
			name:     "int8 type",
			input:    "int8",
			expected: "integer",
		},
		{
			name:     "int16 type",
			input:    "int16",
			expected: "integer",
		},
		{
			name:     "int32 type",
			input:    "int32",
			expected: "integer",
		},
		{
			name:     "int64 type",
			input:    "int64",
			expected: "integer",
		},

		// Unsigned integer types
		{
			name:     "uint type",
			input:    "uint",
			expected: "integer",
		},
		{
			name:     "uint8 type",
			input:    "uint8",
			expected: "unsigned integer",
		},
		{
			name:     "uint16 type",
			input:    "uint16",
			expected: "unsigned integer",
		},
		{
			name:     "uint32 type",
			input:    "uint32",
			expected: "unsigned integer",
		},
		{
			name:     "uint64 type",
			input:    "uint64",
			expected: "unsigned integer",
		},
		{
			name:     "uintptr type",
			input:    "uintptr",
			expected: "unsigned integer",
		},

		// Float types
		{
			name:     "float32 type",
			input:    "float32",
			expected: "float",
		},
		{
			name:     "float64 type",
			input:    "float64",
			expected: "float",
		},

		// Boolean type
		{
			name:     "bool type",
			input:    "bool",
			expected: "boolean",
		},

		// Special types
		{
			name:     "rune type",
			input:    "rune",
			expected: "integer",
		},
		{
			name:     "byte type",
			input:    "byte",
			expected: "unsigned integer",
		},
		{
			name:     "struct type",
			input:    "struct",
			expected: "object",
		},

		// Complex types
		{
			name:     "complex64 type",
			input:    "complex64",
			expected: "complex number",
		},
		{
			name:     "complex128 type",
			input:    "complex128",
			expected: "complex number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeYAMLTypeYAMLTags verifies normalization of YAML type tags.
func TestNormalizeYAMLTypeYAMLTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "YAML !!str tag",
			input:    "!!str",
			expected: "string",
		},
		{
			name:     "YAML !!int tag",
			input:    "!!int",
			expected: "integer",
		},
		{
			name:     "YAML !!float tag",
			input:    "!!float",
			expected: "float",
		},
		{
			name:     "YAML !!bool tag",
			input:    "!!bool",
			expected: "boolean",
		},
		{
			name:     "YAML !!seq tag",
			input:    "!!seq",
			expected: "array",
		},
		{
			name:     "YAML !!map tag",
			input:    "!!map",
			expected: "object",
		},
		{
			name:     "YAML !!null tag",
			input:    "!!null",
			expected: "null",
		},
		// YAML tags without !! prefix
		{
			name:     "str tag without prefix",
			input:    "str",
			expected: "string",
		},
		{
			name:     "int tag without prefix",
			input:    "int",
			expected: "integer",
		},
		{
			name:     "float tag without prefix",
			input:    "float",
			expected: "float",
		},
		{
			name:     "bool tag without prefix",
			input:    "bool",
			expected: "boolean",
		},
		{
			name:     "seq tag without prefix",
			input:    "seq",
			expected: "array",
		},
		{
			name:     "map tag without prefix",
			input:    "map",
			expected: "object",
		},
		{
			name:     "null tag without prefix",
			input:    "null",
			expected: "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeYAMLTypePointer verifies normalization of pointer types.
func TestNormalizeYAMLTypePointer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single pointer to string",
			input:    "*string",
			expected: "pointer to string",
		},
		{
			name:     "single pointer to int",
			input:    "*int",
			expected: "pointer to integer",
		},
		{
			name:     "single pointer to bool",
			input:    "*bool",
			expected: "pointer to boolean",
		},
		{
			name:     "double pointer",
			input:    "**int",
			expected: "pointer to integer",
		},
		{
			name:     "triple pointer",
			input:    "***string",
			expected: "pointer to pointer to string",
		},
		{
			name:     "pointer to slice",
			input:    "*[]string",
			expected: "pointer to array of string",
		},
		{
			name:     "pointer to map",
			input:    "*map[string]int",
			expected: "pointer to map[string]integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeYAMLTypeSlice verifies normalization of slice/array types.
func TestNormalizeYAMLTypeSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "slice of string",
			input:    "[]string",
			expected: "array of string",
		},
		{
			name:     "slice of int",
			input:    "[]int",
			expected: "array of integer",
		},
		{
			name:     "slice of bool",
			input:    "[]bool",
			expected: "array of boolean",
		},
		{
			name:     "slice of pointers",
			input:    "[]*int",
			expected: "array of pointer to integer",
		},
		{
			name:     "slice of slices",
			input:    "[][]string",
			expected: "array of array of string",
		},
		{
			name:     "fixed array of int",
			input:    "[10]int",
			expected: "array of integer",
		},
		{
			name:     "fixed array of string",
			input:    "[32]string",
			expected: "array of string",
		},
		{
			name:     "fixed array with large size",
			input:    "[1000]float64",
			expected: "array of float",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeYAMLTypeMap verifies normalization of map types.
func TestNormalizeYAMLTypeMap(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "map with string keys and int values",
			input:    "map[string]int",
			expected: "map[string]integer",
		},
		{
			name:     "map with int keys and string values",
			input:    "map[int]string",
			expected: "map[integer]string",
		},
		{
			name:     "map with string keys and bool values",
			input:    "map[string]bool",
			expected: "map[string]boolean",
		},
		{
			name:     "map with string keys and pointer values",
			input:    "map[string]*int",
			expected: "map[string]pointer to integer",
		},
		{
			name:     "map with int keys and slice values",
			input:    "map[int][]string",
			expected: "map[integer]array of string",
		},
		{
			name:     "map with complex key type",
			input:    "map[uint64]string",
			expected: "map[unsigned integer]string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeYAMLTypeChannel verifies normalization of channel types.
func TestNormalizeYAMLTypeChannel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bidirectional channel of int",
			input:    "chan int",
			expected: "channel of integer",
		},
		{
			name:     "bidirectional channel of string",
			input:    "chan string",
			expected: "channel of string",
		},
		{
			name:     "send-only channel",
			input:    "chan<- float64",
			expected: "send-only channel of float",
		},
		{
			name:     "receive-only channel",
			input:    "<-chan bool",
			expected: "receive-only channel of boolean",
		},
		{
			name:     "channel of pointers",
			input:    "chan *int",
			expected: "channel of pointer to integer",
		},
		{
			name:     "channel of slices",
			input:    "chan []string",
			expected: "channel of array of string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeYAMLTypeInterface verifies normalization of interface types.
func TestNormalizeYAMLTypeInterface(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "interface{} type",
			input:    "interface{}",
			expected: "interface",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeYAMLTypePackageQualified verifies normalization of package-qualified types.
func TestNormalizeYAMLTypePackageQualified(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "time.Time type",
			input:    "time.Time",
			expected: "Time",
		},
		{
			name:     "time.Duration type",
			input:    "time.Duration",
			expected: "Duration",
		},
		{
			name:     "http.Response type",
			input:    "http.Response",
			expected: "Response",
		},
		{
			name:     "url.URL type",
			input:    "url.URL",
			expected: "URL",
		},
		{
			name:     "json.Marshaler type",
			input:    "encoding/json.Marshaler",
			expected: "Marshaler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeYAMLTypeEdgeCases verifies edge cases and special inputs.
func TestNormalizeYAMLTypeEdgeCases(t *testing.T) {
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
			name:     "whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "whitespace with tabs and newlines",
			input:    "  \t\n  ",
			expected: "",
		},
		{
			name:     "unknown type - returns as-is",
			input:    "CustomType",
			expected: "CustomType",
		},
		{
			name:     "unknown type with numbers",
			input:    "MyType123",
			expected: "MyType123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeYAMLTypeComplex verifies normalization of complex nested types.
func TestNormalizeYAMLTypeComplex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pointer to slice",
			input:    "*[]string",
			expected: "pointer to array of string",
		},
		{
			name:     "slice of pointers",
			input:    "[]*int",
			expected: "array of pointer to integer",
		},
		{
			name:     "map of pointers",
			input:    "map[string]*int",
			expected: "map[string]pointer to integer",
		},
		{
			name:     "map of slices",
			input:    "map[int][]string",
			expected: "map[integer]array of string",
		},
		{
			name:     "channel of pointers",
			input:    "chan *int",
			expected: "channel of pointer to integer",
		},
		{
			name:     "channel of slices",
			input:    "chan []string",
			expected: "channel of array of string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
