package yamlutil

import (
	"testing"
)

// TestExtractTypeNameBasicMalformedErrors tests malformed error messages
// and ensures the basic extractor handles them gracefully (bf-492g6).
func TestExtractTypeNameBasicMalformedErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty strings
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only spaces",
			input:    "    ",
			expected: "",
		},
		{
			name:     "only tabs",
			input:    "\t\t\t",
			expected: "",
		},
		{
			name:     "only newlines",
			input:    "\n\n\n",
			expected: "",
		},
		{
			name:     "mixed whitespace",
			input:    "  \t\n\r  ",
			expected: "",
		},

		// Special characters only
		{
			name:     "only punctuation",
			input:    "!@#$%^&*()",
			expected: "",
		},
		{
			name:     "only brackets",
			input:    "[]{}()<>",
			expected: "",
		},
		{
			name:     "only asterisks",
			input:    "***",
			expected: "",
		},
		{
			name:     "only dots",
			input:    "...",
			expected: "",
		},
		{
			name:     "only special chars mixed",
			input:    "!@#$$%^&*()_+-=[]{}|;':\",./<>?",
			expected: "",
		},

		// Messages with no type name
		{
			name:     "random text no type",
			input:    "something went wrong",
			expected: "",
		},
		{
			name:     "error message without type",
			input:    "error: something failed",
			expected: "",
		},
		{
			name:     "warning message no type",
			input:    "warning: check your input",
			expected: "",
		},
		{
			name:     "generic failure message",
			input:    "operation failed: internal error",
			expected: "",
		},
		{
			name:     "validation error without type",
			input:    "validation failed: invalid value",
			expected: "",
		},

		// Malformed type patterns - these should still extract what they can
		{
			name:     "incomplete YAML tag",
			input:    "cannot unmarshal !",
			expected: "",
		},
		{
			name:     "incomplete YAML tag with bracket",
			input:    "cannot unmarshal [",
			expected: "",
		},
		{
			name:     "incomplete expected pattern",
			input:    "expected",
			expected: "",
		},
		{
			name:     "incomplete expected with comma",
			input:    "expected,",
			expected: "",
		},
		{
			name:     "incomplete unmarshal pattern",
			input:    "cannot unmarshal into",
			expected: "",
		},
		{
			name:     "close bracket without open - still matches type",
			input:    "string]",
			expected: "string", // Basic pattern matches "string"
		},
		{
			name:     "asterisk without type",
			input:    "*",
			expected: "",
		},
		{
			name:     "map without close bracket - still matches element type",
			input:    "map[string",
			expected: "string", // Basic pattern matches "string"
		},
		{
			name:     "unmatched brackets in map",
			input:    "map[string]int]",
			expected: "map[string]int", // Pattern matches anyway
		},

		// Unicode and non-ASCII
		{
			name:     "unicode characters no type",
			input:    "エラーメッセージ",
			expected: "",
		},
		{
			name:     "mixed unicode and valid type",
			input:    "エラー string エラー",
			expected: "string",
		},
		{
			name:     "emoji with no type",
			input:    "error 🚫 💥",
			expected: "",
		},
		{
			name:     "emoji with valid type",
			input:    "error 🚫 int 💥",
			expected: "int",
		},

		// Type-like strings that aren't actual types (false positives to check)
		{
			name:     "map as verb - not matched",
			input:    "map the data to something",
			expected: "",
		},
		{
			name:     "internal - contains int but not matched",
			input:    "internal server error",
			expected: "", // "int" is not a separate word
		},
		{
			name:     "interval - contains int but not matched",
			input:    "retry interval exceeded",
			expected: "", // "int" is not a separate word
		},
		{
			name:     "interface as verb - matches anyway",
			input:    "interface with the system",
			expected: "interface", // Pattern matches "interface"
		},
		{
			name:     "byte as noun - matches",
			input:    "byte array too large",
			expected: "byte", // Pattern matches "byte"
		},
		{
			name:     "rune as word - matches",
			input:    "rune stone discovered",
			expected: "rune", // Pattern matches "rune"
		},
		{
			name:     "bool in boolean - not matched",
			input:    "boolean logic is complex",
			expected: "", // "bool" is not a separate word
		},
		{
			name:     "string in stringing - not matched",
			input:    "stringing along",
			expected: "", // "string" is not a separate word
		},

		// Numbers that might look like types
		{
			name:     "integer number",
			input:    "value is 12345",
			expected: "",
		},
		{
			name:     "float number",
			input:    "value is 123.45",
			expected: "",
		},
		{
			name:     "hex number",
			input:    "value is 0xABCD",
			expected: "",
		},
		{
			name:     "binary number",
			input:    "value is 0b1010",
			expected: "",
		},

		// Case sensitivity - basic types are case-sensitive
		{
			name:     "uppercase STRING not matched",
			input:    "expected STRING, got int",
			expected: "int", // Only matches "int"
		},
		{
			name:     "mixed case String not matched",
			input:    "expected String, got int",
			expected: "int", // Only matches "int"
		},
		{
			name:     "lowercase string matched",
			input:    "expected string, got int",
			expected: "string", // Matches "string" first
		},

		// Real-world error messages that shouldn't match
		{
			name:     "nil dereference error",
			input:    "nil pointer dereference",
			expected: "",
		},
		{
			name:     "index out of range",
			input:    "index out of range",
			expected: "",
		},
		{
			name:     "panic message",
			input:    "panic: runtime error",
			expected: "",
		},
		{
			name:     "fatal error",
			input:    "fatal error: cannot continue",
			expected: "",
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

// TestExtractTypeNameAdvancedMalformedErrors tests malformed error messages
// for the advanced extractor that handles different positions (bf-492g6).
func TestExtractTypeNameAdvancedMalformedErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty strings
		{
			name:     "empty string advanced",
			input:    "",
			expected: "",
		},
		{
			name:     "only spaces advanced",
			input:    "    ",
			expected: "",
		},
		{
			name:     "only tabs advanced",
			input:    "\t\t\t",
			expected: "",
		},
		{
			name:     "mixed whitespace advanced",
			input:    "  \t\n\r  ",
			expected: "",
		},

		// Special characters only
		{
			name:     "only special chars advanced",
			input:    "!@#$%^&*()",
			expected: "",
		},
		{
			name:     "only brackets advanced",
			input:    "[]{}()<>",
			expected: "",
		},

		// Messages with no type name
		{
			name:     "random text no type advanced",
			input:    "something went wrong",
			expected: "",
		},
		{
			name:     "error without type advanced - not matched",
			input:    "error: something failed",
			expected: "", // Pattern 5 excludes "error:" as a common prefix
		},

		// Malformed patterns - actual behavior of extractTypeName
		{
			name:     "expected without type - matches got pattern fallback",
			input:    "expected, got string",
			expected: "string", // Pattern 10 matches "got string"
		},
		{
			name:     "expected with only whitespace - matches got pattern fallback",
			input:    "expected  , got string",
			expected: "string", // Pattern 10 matches "got string"
		},
		{
			name:     "want without type - matches got pattern fallback",
			input:    "want, got string",
			expected: "string", // Pattern 10 matches "got string"
		},
		{
			name:     "unmarshal without into - no match",
			input:    "cannot unmarshal !!str",
			expected: "", // Pattern 1 requires "into" keyword
		},
		{
			name:     "unmarshal without type tag - no match",
			input:    "cannot unmarshal into string",
			expected: "", // Pattern 1 requires YAML tag
		},
		{
			name:     "unmarshal with incomplete tag - no match",
			input:    "cannot unmarshal !! into string",
			expected: "", // Pattern 1 requires valid YAML tag
		},

		// Type-like strings that should NOT match in advanced extractor
		{
			name:     "map as verb advanced",
			input:    "map the data to something",
			expected: "",
		},
		{
			name:     "expected with non-type word - matches expected pattern",
			input:    "expected value, got something",
			expected: "value", // Pattern 8 matches "expected value"
		},
		{
			name:     "want with non-type word - matches want pattern",
			input:    "want result, got error",
			expected: "result", // Pattern 9 matches "want result"
		},
		{
			name:     "interface used as verb",
			input:    "interface the system now",
			expected: "",
		},
		{
			name:     "channel used as noun",
			input:    "the channel is closed",
			expected: "",
		},

		// Common English phrases that shouldn't match
		{
			name:     "want as common verb",
			input:    "I want to go home",
			expected: "",
		},
		{
			name:     "expected as adjective",
			input:    "the expected result is good",
			expected: "",
		},
		{
			name:     "got as common verb",
			input:    "I got a new car",
			expected: "",
		},
		{
			name:     "into as preposition - not matched without unmarshal context",
			input:    "go into the room",
			expected: "", // Pattern 7 requires unmarshal context
		},
		{
			name:     "type as common noun",
			input:    "what type of error",
			expected: "",
		},

		// Unicode without types
		{
			name:     "unicode no type advanced",
			input:    "エラーメッセージ",
			expected: "",
		},
		{
			name:     "emoji no type advanced",
			input:    "error 🚫 💥",
			expected: "",
		},

		// Case sensitivity - advanced patterns are case-sensitive
		{
			name:     "uppercase STRING advanced - matches STRING",
			input:    "expected STRING, got int",
			expected: "STRING", // Pattern 8 matches "expected STRING"
		},
		{
			name:     "mixed case String advanced - matches String",
			input:    "expected String, got int",
			expected: "String", // Pattern 8 matches "expected String"
		},
		{
			name:     "lowercase string advanced",
			input:    "expected string, got int",
			expected: "string",
		},

		// Real-world errors without types
		{
			name:     "nil dereference advanced",
			input:    "nil pointer dereference",
			expected: "",
		},
		{
			name:     "index out of range advanced",
			input:    "index out of range",
			expected: "",
		},
		{
			name:     "panic advanced - not matched",
			input:    "panic: runtime error",
			expected: "", // Pattern 5 excludes "panic:" as a common prefix
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

// TestExtractTypeNameNormalizationEdgeCases tests edge cases for type
// normalization with malformed and unusual inputs (bf-492g6).
func TestExtractTypeNameNormalizationEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty and whitespace
		{
			name:     "empty string normalization",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only normalization",
			input:    "   ",
			expected: "",
		},

		// Types with trailing punctuation - should NOT be trimmed
		{
			name:     "type with trailing comma - kept as-is",
			input:    "string,",
			expected: "string,", // Normalization doesn't trim punctuation
		},
		{
			name:     "type with trailing period - kept as-is",
			input:    "int.",
			expected: "int.", // Normalization doesn't trim punctuation
		},
		{
			name:     "type with trailing punctuation combo - kept as-is",
			input:    "bool,.",
			expected: "bool,.", // Normalization doesn't trim punctuation
		},

		// Malformed type representations
		{
			name:     "double pointer",
			input:    "**string",
			expected: "pointer to pointer to string",
		},
		{
			name:     "triple pointer",
			input:    "***int",
			expected: "pointer to pointer to pointer to integer",
		},
		{
			name:     "slice of pointer",
			input:    "[]*string",
			expected: "array of pointer to string",
		},
		{
			name:     "pointer to slice",
			input:    "*[]int",
			expected: "pointer to array of integer",
		},

		// Complex nested types
		{
			name:     "map with pointer value",
			input:    "map[string]*int",
			expected: "map[string]pointer to integer",
		},
		{
			name:     "map with slice value",
			input:    "map[int][]string",
			expected: "map[integer]array of string",
		},
		{
			name:     "slice of maps",
			input:    "[]map[string]int",
			expected: "array of map[string]integer",
		},

		// Channel types
		{
			name:     "bidirectional channel",
			input:    "chan string",
			expected: "channel of string",
		},
		{
			name:     "send-only channel",
			input:    "chan<- int",
			expected: "send-only channel of integer",
		},
		{
			name:     "receive-only channel",
			input:    "<-chan bool",
			expected: "receive-only channel of boolean",
		},

		// Interface types
		{
			name:     "empty interface",
			input:    "interface{}",
			expected: "interface",
		},
		{
			name:     "pointer to interface",
			input:    "*interface{}",
			expected: "pointer to interface",
		},

		// Fixed-size arrays
		{
			name:     "fixed array",
			input:    "[10]string",
			expected: "array of string",
		},
		{
			name:     "fixed array of pointers",
			input:    "[5]*int",
			expected: "array of pointer to integer",
		},

		// Package-qualified types
		{
			name:     "time.Time",
			input:    "time.Time",
			expected: "Time",
		},
		{
			name:     "pointer to time.Time",
			input:    "*time.Time",
			expected: "pointer to Time",
		},
		{
			name:     "slice of time.Time",
			input:    "[]time.Time",
			expected: "array of Time",
		},

		// Unicode in type names (custom types)
		{
			name:     "unicode in custom type",
			input:    "stringエラー",
			expected: "stringエラー",
		},

		// Very long type names
		{
			name:     "very long custom type",
			input:    "VeryLongComplexTypeNameThatKeepsGoingAndGoing",
			expected: "VeryLongComplexTypeNameThatKeepsGoingAndGoing",
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
