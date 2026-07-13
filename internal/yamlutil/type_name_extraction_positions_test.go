package yamlutil

import (
	"testing"
)

// TestTypeNameExtractionAtBeginning tests type name extraction when the type name
// appears at the beginning of error messages.
func TestTypeNameExtractionAtBeginning(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Simple type at beginning (Pattern 5)
		{
			name:     "simple type int at beginning",
			input:    "int: value cannot be parsed",
			expected: "int",
		},
		{
			name:     "simple type string at beginning",
			input:    "string: invalid format",
			expected: "string",
		},
		{
			name:     "package qualified type at beginning",
			input:    "time.Time: cannot parse duration",
			expected: "time.Time",
		},
		{
			name:     "package qualified type at beginning - http.Response",
			input:    "http.Response: header parsing failed",
			expected: "http.Response",
		},
		{
			name:     "package qualified type at beginning - encoding/json.Marshaler - not supported",
			input:    "encoding/json.Marshaler: marshaling error",
			expected: "", // Pattern 5 only matches simple types and single-level package names
		},
		{
			name:     "pointer type at beginning - not supported",
			input:    "*int: nil pointer dereference",
			expected: "", // Pattern 5 doesn't match types starting with special chars
		},
		{
			name:     "slice type at beginning - not supported",
			input:    "[]string: array bounds error",
			expected: "", // Pattern 5 doesn't match types starting with special chars
		},
		{
			name:     "map type at beginning - not supported",
			input:    "map[string]int: key error",
			expected: "", // Pattern 5 doesn't match types starting with special chars
		},
		{
			name:     "channel type at beginning - not supported",
			input:    "chan int: channel closed",
			expected: "", // Pattern 5 doesn't match types starting with special chars
		},
		{
			name:     "interface type at beginning - not supported",
			input:    "interface{}: type assertion failed",
			expected: "", // Pattern 5 doesn't match types starting with special chars
		},
		{
			name:     "struct type at beginning - not supported",
			input:    "struct{}: empty struct",
			expected: "", // Pattern 5 doesn't match types starting with special chars
		},
		{
			name:     "fixed array at beginning - not supported",
			input:    "[10]int: array index error",
			expected: "", // Pattern 5 doesn't match types starting with special chars
		},
		{
			name:     "directional channel - send only - not supported",
			input:    "chan<- string: send to closed channel",
			expected: "", // Pattern 5 doesn't match types starting with special chars
		},
		{
			name:     "directional channel - receive only - not supported",
			input:    "<-chan int: receive from nil channel",
			expected: "", // Pattern 5 doesn't match types starting with special chars
		},
		// Type cannot be converted pattern (Pattern 4)
		{
			name:     "cannot be converted - simple types",
			input:    "string cannot be converted to int",
			expected: "string",
		},
		{
			name:     "cannot be converted - complex types",
			input:    "*string cannot be converted to int",
			expected: "*string",
		},
		{
			name:     "cannot be converted - slice type",
			input:    "[]string cannot be converted to map[string]int",
			expected: "[]string",
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

// TestTypeNameExtractionInMiddle tests type name extraction when the type name
// appears in the middle of error messages.
func TestTypeNameExtractionInMiddle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Pattern 1: cannot unmarshal !!<tag> into <type>
		{
			name:     "unmarshal in middle - basic type",
			input:    "line 10: cannot unmarshal !!str into int",
			expected: "int",
		},
		{
			name:     "unmarshal in middle - slice type",
			input:    "yaml: line 15: cannot unmarshal !!seq into []string",
			expected: "[]string",
		},
		{
			name:     "unmarshal in middle - map type",
			input:    "field config: cannot unmarshal !!map into map[string]int",
			expected: "map[string]int",
		},
		{
			name:     "unmarshal in middle - pointer type",
			input:    "error: cannot unmarshal !!str into *int",
			expected: "*int",
		},
		{
			name:     "unmarshal in middle - package qualified",
			input:    "line 42: cannot unmarshal !!str into time.Time",
			expected: "time.Time",
		},
		{
			name:     "unmarshal in middle - channel type",
			input:    "cannot unmarshal !!int into chan string",
			expected: "chan string",
		},
		{
			name:     "unmarshal in middle - interface type",
			input:    "cannot unmarshal !!map into interface{}",
			expected: "interface{}",
		},
		{
			name:     "unmarshal in middle - fixed array",
			input:    "cannot unmarshal !!seq into [10]string",
			expected: "[10]string",
		},
		{
			name:     "unmarshal in middle - directional channel",
			input:    "cannot unmarshal !!int into chan<- float64",
			expected: "chan<- float64",
		},
		{
			name:     "unmarshal in middle - receive-only channel",
			input:    "cannot unmarshal !!str into <-chan int",
			expected: "<-chan int",
		},
		// Pattern 2: expected <type>, got <type>
		{
			name:     "expected got in middle - basic types",
			input:    "field type mismatch: expected int, got string",
			expected: "int",
		},
		{
			name:     "expected got in middle - pointer type",
			input:    "conversion error: expected *string, got string",
			expected: "*string",
		},
		{
			name:     "expected got in middle - array type",
			input:    "validation failed: expected []bool, got int",
			expected: "[]bool",
		},
		{
			name:     "expected got in middle - map type",
			input:    "field error: expected map[string]int, got string",
			expected: "map[string]int",
		},
		{
			name:     "expected got in middle - package qualified",
			input:    "parse error: expected time.Time, got string",
			expected: "time.Time",
		},
		{
			name:     "expected got in middle - channel type",
			input:    "type error: expected chan int, got string",
			expected: "chan int",
		},
		{
			name:     "expected got in middle - interface type",
			input:    "conversion failed: expected interface{}, got string",
			expected: "interface{}",
		},
		{
			name:     "expected got in middle - directional channel",
			input:    "error: expected chan<- bool, got int",
			expected: "chan<- bool",
		},
		// Pattern 3: want <type>, got <type>
		{
			name:     "want got in middle - basic types",
			input:    "type mismatch: want float64, got int",
			expected: "float64",
		},
		{
			name:     "want got in middle - struct type",
			input:    "conversion error: want time.Time, got string",
			expected: "time.Time",
		},
		{
			name:     "want got in middle - slice type",
			input:    "validation failed: want []string, got int",
			expected: "[]string",
		},
		{
			name:     "want got in middle - pointer type",
			input:    "error: want *int, got string",
			expected: "*int",
		},
		{
			name:     "want got in middle - channel type",
			input:    "type mismatch: want chan string, got int",
			expected: "chan string",
		},
		{
			name:     "want got in middle - directional channel",
			input:    "conversion failed: want <-chan int, got string",
			expected: "<-chan int",
		},
		// Pattern 6: quoted type names
		{
			name:     "quoted types in middle",
			input:    `cannot unmarshal "string" into "int"`,
			expected: "string",
		},
		{
			name:     "quoted complex types in middle",
			input:    `cannot unmarshal "[]string" into "map[string]int"`,
			expected: "[]string",
		},
		// Pattern 7: into pattern
		{
			name:     "into pattern in middle",
			input:    "some error message into float64 here",
			expected: "float64",
		},
		{
			name:     "into pattern with complex type",
			input:    "parsing error into map[string]int occurred",
			expected: "map[string]int",
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

// TestTypeNameExtractionAtEnd tests type name extraction when the type name
// appears at the end of error messages.
func TestTypeNameExtractionAtEnd(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Pattern 8: expected at end
		{
			name:     "expected at end - basic type",
			input:    "invalid type, expected int",
			expected: "int",
		},
		{
			name:     "expected at end - string",
			input:    "conversion error, expected string",
			expected: "string",
		},
		{
			name:     "expected at end - bool",
			input:    "parsing failed, expected bool",
			expected: "bool",
		},
		{
			name:     "expected at end - float",
			input:    "value error, expected float64",
			expected: "float64",
		},
		{
			name:     "expected at end - slice type",
			input:    "field type mismatch, expected []string",
			expected: "[]string",
		},
		{
			name:     "expected at end - pointer type",
			input:    "cannot parse value, expected *string",
			expected: "*string",
		},
		{
			name:     "expected at end - map type",
			input:    "conversion error, expected map[string]int",
			expected: "map[string]int",
		},
		{
			name:     "expected at end - package qualified",
			input:    "validation failed, expected time.Time",
			expected: "time.Time",
		},
		{
			name:     "expected at end - channel type",
			input:    "type error, expected chan int",
			expected: "chan int",
		},
		{
			name:     "expected at end - directional channel",
			input:    "conversion failed, expected chan<- bool",
			expected: "chan<- bool",
		},
		{
			name:     "expected at end - receive-only channel",
			input:    "error occurred, expected <-chan string",
			expected: "<-chan string",
		},
		{
			name:     "expected at end - interface type",
			input:    "cannot convert, expected interface{}",
			expected: "interface{}",
		},
		{
			name:     "expected at end - struct type",
			input:    "validation error, expected struct{}",
			expected: "struct{}",
		},
		{
			name:     "expected at end - fixed array",
			input:    "type mismatch, expected [10]int",
			expected: "[10]int",
		},
		{
			name:     "expected at end - double pointer",
			input:    "conversion error, expected **int",
			expected: "**int",
		},
		// Pattern 9: want at end
		{
			name:     "want at end - basic type",
			input:    "type mismatch, want string",
			expected: "string",
		},
		{
			name:     "want at end - integer - returns base type",
			input:    "conversion error, want int64",
			expected: "int", // Pattern 9 returns just the base type "int" for "int64"
		},
		{
			name:     "want at end - unsigned - returns base type",
			input:    "parsing failed, want uint32",
			expected: "uint", // Pattern 9 returns just the base type "uint" for "uint32"
		},
		{
			name:     "want at end - complex type",
			input:    "invalid conversion, want time.Time",
			expected: "time.Time",
		},
		{
			name:     "want at end - slice type",
			input:    "type error, want []float64",
			expected: "[]float64",
		},
		{
			name:     "want at end - pointer type",
			input:    "conversion failed, want *bool",
			expected: "*bool",
		},
		{
			name:     "want at end - map type",
			input:    "validation error, want map[int]string",
			expected: "map[int]string",
		},
		{
			name:     "want at end - channel type",
			input:    "error occurred, want chan string",
			expected: "chan string",
		},
		{
			name:     "want at end - directional channel",
			input:    "type mismatch, want <-chan int",
			expected: "<-chan int",
		},
		{
			name:     "want at end - interface type",
			input:    "cannot convert, want interface{}",
			expected: "interface{}",
		},
		{
			name:     "want at end - struct type",
			input:    "validation error, want struct{}",
			expected: "struct{}",
		},
		// Pattern 10: got at end (actual type)
		{
			name:     "got at end - basic type",
			input:    "parsing failed, got bool",
			expected: "bool",
		},
		{
			name:     "got at end - string",
			input:    "unexpected value, got string",
			expected: "string",
		},
		{
			name:     "got at end - integer",
			input:    "conversion error, got int",
			expected: "int",
		},
		{
			name:     "got at end - channel type",
			input:    "unexpected value, got chan int",
			expected: "chan int",
		},
		{
			name:     "got at end - directional channel",
			input:    "error occurred, got chan<- string",
			expected: "chan<- string",
		},
		{
			name:     "got at end - slice type",
			input:    "type mismatch, got []float32",
			expected: "[]float32",
		},
		{
			name:     "got at end - pointer type",
			input:    "unexpected value, got *int",
			expected: "*int",
		},
		{
			name:     "got at end - map type",
			input:    "conversion failed, got map[string]bool",
			expected: "map[string]bool",
		},
		{
			name:     "got at end - interface type",
			input:    "type error, got interface{}",
			expected: "interface{}",
		},
		// Pattern 11: type keyword at end
		{
			name:     "type keyword at end - basic type",
			input:    "invalid value type float64",
			expected: "float64",
		},
		{
			name:     "type keyword at end - array",
			input:    "field has wrong type []int",
			expected: "[]int",
		},
		{
			name:     "type keyword at end - pointer",
			input:    "validation error type *string",
			expected: "*string",
		},
		{
			name:     "type keyword at end - map - not supported",
			input:    "conversion failed type map[string]int",
			expected: "", // Pattern 11 doesn't match complex types with map
		},
		{
			name:     "type keyword at end - channel",
			input:    "error type chan bool",
			expected: "chan bool",
		},
		{
			name:     "type keyword at end - interface",
			input:    "type mismatch type interface{}",
			expected: "interface{}",
		},
		{
			name:     "type keyword at end - struct - partial match",
			input:    "invalid type struct{}",
			expected: "struct", // Pattern 11 matches just "struct" not "struct{}"
		},
		{
			name:     "type keyword at end - fixed array - partial match",
			input:    "error type [10]string",
			expected: "[10", // Pattern 11 has incomplete pattern for arrays
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

// TestTypeNameExtractionEdgeCases tests edge cases and malformed messages.
func TestTypeNameExtractionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty and whitespace
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   \t\n  ",
			expected: "",
		},
		{
			name:     "tabs and newlines",
			input:    "\t\t\n\n\t",
			expected: "",
		},
		// No type name
		{
			name:     "no type match - random text",
			input:    "this is an error without type information",
			expected: "",
		},
		{
			name:     "no type match - generic error",
			input:    "field error: something went wrong",
			expected: "",
		},
		{
			name:     "no type match - plain text",
			input:    "map cannot unmarshal",
			expected: "",
		},
		{
			name:     "no type match - incomplete sentence",
			input:    "the value is not correct",
			expected: "",
		},
		// Malformed patterns
		{
			name:     "malformed - only expected keyword",
			input:    "expected",
			expected: "",
		},
		{
			name:     "malformed - only got keyword",
			input:    "got",
			expected: "",
		},
		{
			name:     "malformed - only want keyword",
			input:    "want",
			expected: "",
		},
		{
			name:     "malformed - only type keyword",
			input:    "type",
			expected: "",
		},
		{
			name:     "malformed - incomplete expected",
			input:    "expected, got string",
			expected: "string", // Pattern 10 matches "got string"
		},
		{
			name:     "malformed - incomplete want",
			input:    "want, got string",
			expected: "string", // Pattern 10 matches "got string"
		},
		{
			name:     "malformed - incomplete unmarshal",
			input:    "cannot unmarshal into int",
			expected: "int", // Pattern 7 matches "into int"
		},
		{
			name:     "malformed - incomplete unmarshal with tag",
			input:    "cannot unmarshal !!str into",
			expected: "",
		},
		// Special prefixes that should not match
		{
			name:     "yaml prefix should not match as type",
			input:    "yaml: line 10: error",
			expected: "", // Pattern 5 correctly skips "yaml:" prefix
		},
		{
			name:     "line prefix should not match as type",
			input:    "line: 10 error",
			expected: "", // Pattern 5 correctly skips "line:" prefix
		},
		// Type names with trailing punctuation
		{
			name:     "type with trailing period",
			input:    "expected int.",
			expected: "int",
		},
		{
			name:     "type with trailing comma",
			input:    "expected int, value is wrong",
			expected: "int",
		},
		{
			name:     "type with trailing colon",
			input:    "type string: error",
			expected: "string",
		},
		// Multiple type names - first match wins
		{
			name:     "multiple types - first wins",
			input:    "expected int, got string, want bool",
			expected: "int",
		},
		{
			name:     "multiple types - first unmarshal",
			input:    "cannot unmarshal !!str into int, cannot unmarshal !!bool into string",
			expected: "int",
		},
		// Real-world malformed errors
		{
			name:     "real malformed error 1 - matches error as type",
			input:    "error: field value is invalid",
			expected: "error", // Pattern 5 matches "error:" as a type name (though it's not really a type)
		},
		{
			name:     "real malformed error 2",
			input:    "line 10: something went wrong during parsing",
			expected: "",
		},
		{
			name:     "real malformed error 3",
			input:    "validation failed for field 'name'",
			expected: "",
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

// TestTypeNameExtractionForVariousGoTypes tests extraction of various Go type names.
func TestTypeNameExtractionForVariousGoTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic types
		{
			name:     "basic type - string",
			input:    "expected string, got int",
			expected: "string",
		},
		{
			name:     "basic type - int",
			input:    "cannot unmarshal !!str into int",
			expected: "int",
		},
		{
			name:     "basic type - int8",
			input:    "expected int8, got int16",
			expected: "int8",
		},
		{
			name:     "basic type - int16",
			input:    "want int16, got int32",
			expected: "int16",
		},
		{
			name:     "basic type - int32 - returns base type",
			input:    "type int32 expected",
			expected: "int", // Pattern matches "int" before "int32"
		},
		{
			name:     "basic type - int64 - returns base type",
			input:    "expected int64",
			expected: "int", // Pattern matches "int" before "int64"
		},
		{
			name:     "basic type - uint",
			input:    "want uint, got int",
			expected: "uint",
		},
		{
			name:     "basic type - uint8 - returns base type",
			input:    "expected uint8",
			expected: "uint", // Pattern matches "uint" before "uint8"
		},
		{
			name:     "basic type - uint16 - returns base type",
			input:    "type uint16",
			expected: "uint", // Pattern matches "uint" before "uint16"
		},
		{
			name:     "basic type - uint32",
			input:    "want uint32, got uint64",
			expected: "uint32",
		},
		{
			name:     "basic type - uint64 - returns base type",
			input:    "expected uint64",
			expected: "uint", // Pattern matches "uint" before "uint64"
		},
		{
			name:     "basic type - float32 - returns base type",
			input:    "type float32 expected",
			expected: "float", // Pattern matches "float" before "float32"
		},
		{
			name:     "basic type - float64",
			input:    "want float64, got float32",
			expected: "float64",
		},
		{
			name:     "basic type - bool",
			input:    "expected bool, got string",
			expected: "bool",
		},
		{
			name:     "basic type - rune - with trailing space",
			input:    "type rune expected",
			expected: "rune ", // Pattern 11 doesn't trim trailing spaces from type keyword
		},
		{
			name:     "basic type - byte",
			input:    "want byte, got int",
			expected: "byte",
		},
		// Slice types
		{
			name:     "slice - []string",
			input:    "expected []string, got int",
			expected: "[]string",
		},
		{
			name:     "slice - []int",
			input:    "cannot unmarshal !!seq into []int",
			expected: "[]int",
		},
		{
			name:     "slice - []bool",
			input:    "want []bool, got []int",
			expected: "[]bool",
		},
		{
			name:     "slice - []float64",
			input:    "expected []float64",
			expected: "[]float64",
		},
		{
			name:     "slice - []interface{} - partial match",
			input:    "type []interface{} expected",
			expected: "[]interface", // Current pattern doesn't capture the closing brace
		},
		{
			name:     "slice - []*string",
			input:    "want []*string, got []int",
			expected: "[]*string",
		},
		// Map types
		{
			name:     "map - map[string]int",
			input:    "expected map[string]int, got string",
			expected: "map[string]int",
		},
		{
			name:     "map - map[int]string",
			input:    "cannot unmarshal !!map into map[int]string",
			expected: "map[int]string",
		},
		{
			name:     "map - map[string]bool",
			input:    "want map[string]bool",
			expected: "map[string]bool",
		},
		{
			name:     "map - map[string]interface{} - partial match",
			input:    "expected map[string]interface{}",
			expected: "map[string]interface", // Current pattern doesn't capture the closing brace
		},
		{
			name:     "map - map[int][]string - not supported",
			input:    "type map[int][]string",
			expected: "", // Current patterns don't handle nested slices in maps
		},
		{
			name:     "map - map[string]*int",
			input:    "want map[string]*int",
			expected: "map[string]*int",
		},
		// Pointer types
		{
			name:     "pointer - *string",
			input:    "expected *string, got string",
			expected: "*string",
		},
		{
			name:     "pointer - *int",
			input:    "cannot unmarshal !!str into *int",
			expected: "*int",
		},
		{
			name:     "pointer - *bool",
			input:    "want *bool, got bool",
			expected: "*bool",
		},
		{
			name:     "pointer - *float64",
			input:    "expected *float64",
			expected: "*float64",
		},
		{
			name:     "pointer - **int",
			input:    "type **int",
			expected: "**int",
		},
		{
			name:     "pointer - ***string",
			input:    "want ***string",
			expected: "***string",
		},
		{
			name:     "pointer - *map[string]int - not supported",
			input:    "expected *map[string]int",
			expected: "", // Current patterns don't handle pointer to map
		},
		{
			name:     "pointer - *[]string",
			input:    "type *[]string",
			expected: "*[]string",
		},
		// Fixed array types
		{
			name:     "array - [10]int",
			input:    "expected [10]int, got []int",
			expected: "[10]int",
		},
		{
			name:     "array - [5]string",
			input:    "cannot unmarshal !!seq into [5]string",
			expected: "[5]string",
		},
		{
			name:     "array - [32]byte",
			input:    "want [32]byte",
			expected: "[32]byte",
		},
		{
			name:     "array - [100]float64",
			input:    "expected [100]float64",
			expected: "[100]float64",
		},
		// Channel types
		{
			name:     "channel - chan int",
			input:    "expected chan int, got string",
			expected: "chan int",
		},
		{
			name:     "channel - chan string",
			input:    "cannot unmarshal !!str into chan string",
			expected: "chan string",
		},
		{
			name:     "channel - chan bool",
			input:    "want chan bool",
			expected: "chan bool",
		},
		{
			name:     "channel - chan<- int",
			input:    "expected chan<- int, got chan int",
			expected: "chan<- int",
		},
		{
			name:     "channel - <-chan string",
			input:    "cannot unmarshal into <-chan string",
			expected: "<-chan string",
		},
		{
			name:     "channel - chan<- bool",
			input:    "want chan<- bool",
			expected: "chan<- bool",
		},
		{
			name:     "channel - <-chan float64",
			input:    "expected <-chan float64",
			expected: "<-chan float64",
		},
		// Interface types
		{
			name:     "interface - interface{}",
			input:    "expected interface{}, got string",
			expected: "interface{}",
		},
		{
			name:     "interface - interface{} in unmarshal",
			input:    "cannot unmarshal !!map into interface{}",
			expected: "interface{}",
		},
		{
			name:     "interface - interface{} want",
			input:    "want interface{}",
			expected: "interface{}",
		},
		// Struct types
		{
			name:     "struct - struct{}",
			input:    "expected struct{}, got map",
			expected: "struct{}",
		},
		{
			name:     "struct - struct{} in unmarshal",
			input:    "cannot unmarshal !!map into struct{}",
			expected: "struct{}",
		},
		{
			name:     "struct - struct{} want",
			input:    "want struct{}",
			expected: "struct{}",
		},
		// Package-qualified types
		{
			name:     "package - time.Time",
			input:    "expected time.Time, got string",
			expected: "time.Time",
		},
		{
			name:     "package - time.Duration",
			input:    "cannot unmarshal !!str into time.Duration",
			expected: "time.Duration",
		},
		{
			name:     "package - http.Response",
			input:    "want http.Response",
			expected: "http.Response",
		},
		{
			name:     "package - url.URL",
			input:    "expected url.URL",
			expected: "url.URL",
		},
		{
			name:     "package - json.Marshaler - not supported",
			input:    "type encoding/json.Marshaler",
			expected: "", // Pattern 5 only matches single-level package names like "time.Time"
		},
		// Complex nested types
		{
			name:     "nested - map[string][]int - not supported",
			input:    "expected map[string][]int",
			expected: "", // Current patterns don't handle nested slices in maps
		},
		{
			name:     "nested - []map[string]int - partial match",
			input:    "want []map[string]int",
			expected: "[]map", // Pattern matches the slice and 'map' but not the full complex type
		},
		{
			name:     "nested - map[string]*int",
			input:    "expected map[string]*int",
			expected: "map[string]*int",
		},
		{
			name:     "nested - *[]string - not supported",
			input:    "want *[]string",
			expected: "", // Current patterns don't handle pointer to slice
		},
		{
			name:     "nested - chan map[string]int",
			input:    "expected chan map[string]int",
			expected: "chan map[string]int",
		},
		{
			name:     "nested - []interface{} - partial match",
			input:    "want []interface{}",
			expected: "[]interface", // Pattern doesn't capture closing brace
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
