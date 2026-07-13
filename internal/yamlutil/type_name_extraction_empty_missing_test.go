package yamlutil

import (
	"testing"
)

// TestExtractTypeNameBasicEmptyAndMissingTypeNames tests the behavior of
// ExtractTypeNameBasic when given empty strings or strings with no type name.
//
// These tests verify that the extractor:
// 1. Returns empty string for empty/whitespace inputs (not nil, not a default)
// 2. Returns empty string for inputs with no recognizable type name
// 3. Does not extract false positives from non-type text
//
// Bead: bf-65co1
func TestExtractTypeNameBasicEmptyAndMissingTypeNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		reason   string
	}{
		// === EMPTY AND WHITESPACE INPUTS ===
		{
			name:     "empty string returns empty",
			input:    "",
			expected: "",
			reason:   "Empty input should return empty string, not nil or panic",
		},
		{
			name:     "only spaces returns empty",
			input:    "     ",
			expected: "",
			reason:   "Whitespace-only input has no type to extract",
		},
		{
			name:     "only tabs returns empty",
			input:    "\t\t\t\t",
			expected: "",
			reason:   "Tab-only input has no type to extract",
		},
		{
			name:     "only newlines returns empty",
			input:    "\n\n\n\n",
			expected: "",
			reason:   "Newline-only input has no type to extract",
		},
		{
			name:     "mixed whitespace returns empty",
			input:    "  \t\n\r  ",
			expected: "",
			reason:   "Mixed whitespace input has no type to extract",
		},
		{
			name:     "whitespace surrounding non-type text returns empty",
			input:    "   some random text   ",
			expected: "",
			reason:   "Whitespace padding doesn't create a type name",
		},

		// === MESSAGES WITH NO TYPE NAME ===
		{
			name:     "generic error message returns empty",
			input:    "something went wrong",
			expected: "",
			reason:   "Generic error text contains no recognizable type name",
		},
		{
			name:     "error prefix without type returns empty",
			input:    "error: something failed",
			expected: "",
			reason:   "'error:' is a prefix, not a type name",
		},
		{
			name:     "warning message returns empty",
			input:    "warning: check your input",
			expected: "",
			reason:   "'warning:' is a prefix, not a type name",
		},
		{
			name:     "validation error without type returns empty",
			input:    "validation failed: invalid value",
			expected: "",
			reason:   "Validation error contains no type name",
		},
		{
			name:     "generic failure message returns empty",
			input:    "operation failed: internal error",
			expected: "",
			reason:   "Generic failure contains no type name",
		},
		{
			name:     "nil pointer error returns empty",
			input:    "nil pointer dereference",
			expected: "",
			reason:   "Runtime error contains no type name",
		},
		{
			name:     "index out of range returns empty",
			input:    "index out of range",
			expected: "",
			reason:   "Panic message contains no type name",
		},
		{
			name:     "panic message returns empty",
			input:    "panic: runtime error",
			expected: "",
			reason:   "Panic prefix is not a type name",
		},
		{
			name:     "fatal error returns empty",
			input:    "fatal error: cannot continue",
			expected: "",
			reason:   "'fatal' prefix is not a type name",
		},

		// === SPECIAL CHARACTERS ONLY ===
		{
			name:     "only punctuation returns empty",
			input:    "!@#$%^&*()",
			expected: "",
			reason:   "Special chars only, no type name present",
		},
		{
			name:     "only brackets returns empty",
			input:    "[]{}()<>",
			expected: "",
			reason:   "Brackets only, no complete type syntax",
		},
		{
			name:     "only asterisks returns empty",
			input:    "***",
			expected: "",
			reason:   "Asterisks without type identifier is meaningless",
		},
		{
			name:     "only dots returns empty",
			input:    "...",
			expected: "",
			reason:   "Dots alone are not a type name",
		},
		{
			name:     "only mixed special chars returns empty",
			input:    "!@#$$%^&*()_+-=[]{}|;':\",./<>?",
			expected: "",
			reason:   "Mixed special chars with no type identifier",
		},

		// === WORDS THAT CONTAIN TYPE-LIKE SUBSTRINGS ===
		// These test that we don't extract false positives
		{
			name:     "internal contains 'int' but returns empty",
			input:    "internal server error",
			expected: "",
			reason:   "'int' as substring should not match (word boundary)",
		},
		{
			name:     "interval contains 'int' but returns empty",
			input:    "retry interval exceeded",
			expected: "",
			reason:   "'int' as substring should not match",
		},
		{
			name:     "integrate contains 'int' but returns empty",
			input:    "failed to integrate",
			expected: "",
			reason:   "'int' as substring should not match",
		},
		{
			name:     "intentional contains 'int' but returns empty",
			input:    "intentional error",
			expected: "",
			reason:   "'int' as substring should not match",
		},
		{
			name:     "boolean contains 'bool' but returns empty",
			input:    "boolean logic is complex",
			expected: "",
			reason:   "'bool' as substring in 'boolean' should not match",
		},
		{
			name:     "stringing contains 'string' but returns empty",
			input:    "stringing along",
			expected: "",
			reason:   "Word boundary prevents matching 'string' in 'stringing'",
		},
		{
			name:     "mapping contains 'map' but returns empty",
			input:    "mapping failed",
			expected: "",
			reason:   "'map' as substring should not match without type syntax",
		},
		{
			name:     "remap contains 'map' but returns empty",
			input:    "remap failed",
			expected: "",
			reason:   "'map' as substring should not match",
		},
		{
			name:     "bytecode contains 'byte' but returns empty",
			input:    "bytecode interpretation failed",
			expected: "",
			reason:   "Word boundary prevents matching 'byte' in 'bytecode'",
		},
		{
			name:     "interfacing contains 'interface' but returns empty",
			input:    "interfacing with hardware",
			expected: "",
			reason:   "Word boundary prevents matching 'interface' in 'interfacing'",
		},

		// === MALFORMED PATTERNS THAT LOOK LIKE TYPES ===
		{
			name:     "incomplete YAML tag returns empty",
			input:    "cannot unmarshal !",
			expected: "",
			reason:   "Incomplete YAML tag has no valid type",
		},
		{
			name:     "incomplete expected pattern returns empty",
			input:    "expected",
			expected: "",
			reason:   "'expected' without type value is incomplete",
		},
		{
			name:     "expected with comma only returns empty",
			input:    "expected,",
			expected: "",
			reason:   "Pattern 'expected,' has no type value",
		},
		{
			name:     "incomplete unmarshal pattern returns empty",
			input:    "cannot unmarshal into",
			expected: "",
			reason:   "Pattern 'unmarshal into' without type is incomplete",
		},
		{
			name:     "asterisk without type returns empty",
			input:    "*",
			expected: "",
			reason:   "Asterisk alone is not a valid pointer type",
		},
		{
			name:     "map without close bracket matches element type",
			input:    "map[string",
			expected: "string",
			reason:   "Basic pattern matches 'string' even though map syntax is incomplete",
		},

		// === UNICODE AND NON-ASCII WITHOUT TYPES ===
		{
			name:     "unicode text only returns empty",
			input:    "エラーメッセージ",
			expected: "",
			reason:   "Japanese error text contains no type name",
		},
		{
			name:     "emoji only returns empty",
			input:    "error 🚫 💥",
			expected: "",
			reason:   "Emoji-only error contains no type name",
		},
		{
			name:     "mixed unicode no type returns empty",
			input:    "エラー: 失敗しました",
			expected: "",
			reason:   "Unicode error message contains no type name",
		},

		// === NUMBERS THAT AREN'T TYPES ===
		{
			name:     "integer number returns empty",
			input:    "value is 12345",
			expected: "",
			reason:   "Numeric literal is not a type name",
		},
		{
			name:     "float number returns empty",
			input:    "value is 123.45",
			expected: "",
			reason:   "Float literal is not a type name",
		},
		{
			name:     "hex number returns empty",
			input:    "value is 0xABCD",
			expected: "",
			reason:   "Hex literal is not a type name",
		},
		{
			name:     "binary number returns empty",
			input:    "value is 0b1010",
			expected: "",
			reason:   "Binary literal is not a type name",
		},

		// === TYPE WORDS USED AS VERBS/NOUNS (NOT TYPES) ===
		{
			name:     "map as verb returns empty",
			input:    "map the data to something",
			expected: "",
			reason:   "'map' used as verb, not as type",
		},
		{
			name:     "interface as verb matches anyway",
			input:    "interface with the system",
			expected: "interface",
			reason:   "Basic extractor matches 'interface' as it's a valid type keyword",
		},
		{
			name:     "byte as noun matches anyway",
			input:    "byte array too large",
			expected: "byte",
			reason:   "Basic extractor matches 'byte' as it's a valid type keyword",
		},
		{
			name:     "rune as noun matches anyway",
			input:    "rune stone discovered",
			expected: "rune",
			reason:   "Basic extractor matches 'rune' as it's a valid type keyword",
		},

		// === CASE SENSITIVITY ===
		{
			name:     "uppercase STRING not matched",
			input:    "expected STRING, got int",
			expected: "int",
			reason:   "Go types are case-sensitive, 'STRING' != 'string'",
		},
		{
			name:     "mixed case String not matched",
			input:    "expected String, got int",
			expected: "int",
			reason:   "Go types are case-sensitive, 'String' != 'string'",
		},

		// === VERY LONG STRINGS WITHOUT TYPES ===
		{
			name:     "very long string without type returns empty",
			input:    string(make([]byte, 10000)),
			expected: "",
			reason:   "Large input with no type should return empty efficiently",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTypeNameBasic(tt.input)

			// Test that we get exactly what we expect
			if result != tt.expected {
				t.Errorf("ExtractTypeNameBasic(%q) = %q, want %q\n  Reason: %s",
					tt.input, result, tt.expected, tt.reason)
			}

			// Additional verification: ensure we don't get nil
			// (Go strings can't be nil, but this documents intent)
			if tt.expected == "" && result != "" {
				t.Errorf("Expected empty string for input with no type, got %q", result)
			}
		})
	}
}

// TestExtractTypeNameAdvancedEmptyAndMissingTypeNames tests the behavior of
// extractTypeName (the advanced function) when given empty strings or strings
// with no type name.
//
// These tests verify that the advanced extractor:
// 1. Returns empty string for empty/whitespace inputs
// 2. Returns empty string for inputs with no recognizable type name
// 3. Correctly handles false positives from error message prefixes
//
// Bead: bf-65co1
func TestExtractTypeNameAdvancedEmptyAndMissingTypeNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		reason   string
	}{
		// === EMPTY AND WHITESPACE INPUTS ===
		{
			name:     "empty string returns empty - advanced",
			input:    "",
			expected: "",
			reason:   "Empty input should return empty string",
		},
		{
			name:     "only spaces returns empty - advanced",
			input:    "     ",
			expected: "",
			reason:   "Whitespace-only input has no type to extract",
		},
		{
			name:     "only tabs returns empty - advanced",
			input:    "\t\t\t\t",
			expected: "",
			reason:   "Tab-only input has no type to extract",
		},
		{
			name:     "only newlines returns empty - advanced",
			input:    "\n\n\n\n",
			expected: "",
			reason:   "Newline-only input has no type to extract",
		},
		{
			name:     "mixed whitespace returns empty - advanced",
			input:    "  \t\n\r  ",
			expected: "",
			reason:   "Mixed whitespace input has no type to extract",
		},
		{
			name:     "whitespace with non-type returns empty - advanced",
			input:    "   some random text   ",
			expected: "",
			reason:   "Whitespace surrounding non-type text still returns empty",
		},
		{
			name:     "whitespace with type pattern returns empty - advanced",
			input:    "   string   ",
			expected: "",
			reason:   "Pattern 5 requires type at start, not after whitespace",
		},

		// === MESSAGES WITH NO TYPE NAME ===
		{
			name:     "generic error returns empty - advanced",
			input:    "something went wrong",
			expected: "",
			reason:   "Generic text contains no type name",
		},
		{
			name:     "error prefix without type returns empty - advanced",
			input:    "error: something failed",
			expected: "",
			reason:   "'error:' is explicitly filtered as prefix, not type",
		},
		{
			name:     "warning prefix returns empty - advanced",
			input:    "warning: check your input",
			expected: "",
			reason:   "'warning:' is explicitly filtered as prefix",
		},
		{
			name:     "fatal prefix returns empty - advanced",
			input:    "fatal error: cannot continue",
			expected: "",
			reason:   "'fatal' is not a type name in this context",
		},
		{
			name:     "yaml prefix returns empty - advanced",
			input:    "yaml: line 10: something",
			expected: "",
			reason:   "'yaml:' is explicitly filtered as prefix",
		},
		{
			name:     "field prefix returns empty - advanced",
			input:    "field: something",
			expected: "",
			reason:   "'field:' is explicitly filtered as prefix",
		},
		{
			name:     "line prefix returns empty - advanced",
			input:    "line: 10: something",
			expected: "",
			reason:   "'line:' is explicitly filtered as prefix",
		},
		{
			name:     "panic prefix returns empty - advanced",
			input:    "panic: runtime error",
			expected: "",
			reason:   "'panic:' is explicitly filtered as prefix",
		},
		{
			name:     "validation error returns empty - advanced",
			input:    "validation failed: invalid value",
			expected: "",
			reason:   "Validation message contains no type name",
		},
		{
			name:     "nil pointer returns empty - advanced",
			input:    "nil pointer dereference",
			expected: "",
			reason:   "Runtime error contains no type name",
		},
		{
			name:     "index out of range returns empty - advanced",
			input:    "index out of range",
			expected: "",
			reason:   "Panic message contains no type name",
		},

		// === SPECIAL CHARACTERS ONLY ===
		{
			name:     "only punctuation returns empty - advanced",
			input:    "!@#$%^&*()",
			expected: "",
			reason:   "Special chars only, no type name present",
		},
		{
			name:     "only brackets returns empty - advanced",
			input:    "[]{}()<>",
			expected: "",
			reason:   "Brackets without type identifiers is meaningless",
		},
		{
			name:     "only asterisks returns empty - advanced",
			input:    "***",
			expected: "",
			reason:   "Asterisks without type identifier is meaningless",
		},

		// === MALFORMED PATTERNS ===
		{
			name:     "expected without type - fallback to got pattern - advanced",
			input:    "expected, got string",
			expected: "string",
			reason:   "Pattern falls back to matching 'got <type>'",
		},
		{
			name:     "want without type - fallback to got pattern - advanced",
			input:    "want, got string",
			expected: "string",
			reason:   "Pattern falls back to matching 'got <type>'",
		},
		{
			name:     "unmarshal without into returns empty - advanced",
			input:    "cannot unmarshal !!str",
			expected: "",
			reason:   "Pattern requires 'into' keyword",
		},
		{
			name:     "unmarshal into without type returns empty - advanced",
			input:    "cannot unmarshal into",
			expected: "",
			reason:   "Pattern 'unmarshal into' without type is incomplete",
		},
		{
			name:     "unmarshal incomplete tag returns empty - advanced",
			input:    "cannot unmarshal !! into string",
			expected: "",
			reason:   "Pattern 7 requires valid word chars in middle, '!!' is not matched",
		},

		// === TYPE-LIKE WORDS IN WRONG CONTEXT ===
		{
			name:     "map as verb returns empty - advanced",
			input:    "map the data to something",
			expected: "",
			reason:   "'map' used as verb, not as type",
		},
		{
			name:     "channel as noun returns empty - advanced",
			input:    "the channel is closed",
			expected: "",
			reason:   "'channel' used as common noun, not as type",
		},
		{
			name:     "interface as verb returns empty - advanced",
			input:    "interface the system now",
			expected: "",
			reason:   "'interface' used as verb, not as type (no pattern match)",
		},

		// === COMMON ENGLISH PHRASES ===
		{
			name:     "want as common verb returns empty - advanced",
			input:    "I want to go home",
			expected: "",
			reason:   "'want' as common verb should not match type pattern",
		},
		{
			name:     "expected as adjective returns empty - advanced",
			input:    "the expected result is good",
			expected: "",
			reason:   "'expected' as adjective should not match type pattern",
		},
		{
			name:     "got as common verb returns empty - advanced",
			input:    "I got a new car",
			expected: "",
			reason:   "'got' as common verb should not match type pattern",
		},
		{
			name:     "into as preposition returns empty - advanced",
			input:    "go into the room",
			expected: "",
			reason:   "'into' as preposition without unmarshal context should not match",
		},
		{
			name:     "type as common noun returns empty - advanced",
			input:    "what type of error",
			expected: "",
			reason:   "'type' as common noun should not match type pattern",
		},

		// === UNICODE WITHOUT TYPES ===
		{
			name:     "unicode text returns empty - advanced",
			input:    "エラーメッセージ",
			expected: "",
			reason:   "Japanese text contains no type name",
		},
		{
			name:     "emoji returns empty - advanced",
			input:    "error 🚫 💥",
			expected: "",
			reason:   "Emoji error contains no type name",
		},
		{
			name:     "unicode with type at start returns empty - advanced",
			input:    "エラー string エラー",
			expected: "",
			reason:   "Pattern 5 requires type at very start, not after unicode",
		},

		// === VERY LONG STRINGS ===
		{
			name:     "very long without type returns empty - advanced",
			input:    string(make([]byte, 10000)),
			expected: "",
			reason:   "Large input with no type should return empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTypeName(tt.input)

			if result != tt.expected {
				t.Errorf("extractTypeName(%q) = %q, want %q\n  Reason: %s",
					tt.input, result, tt.expected, tt.reason)
			}

			// Verify behavior for expected empty results
			if tt.expected == "" && result != "" {
				t.Errorf("Expected empty string for input with no type, got %q", result)
			}
		})
	}
}

// TestNormalizeYAMLTypeEmptyAndMissingInputs tests the behavior of
// normalizeYAMLType when given empty or unusual inputs.
//
// These tests verify that normalization:
// 1. Returns empty string for empty/whitespace inputs
// 2. Handles unusual type strings gracefully
// 3. Returns the input as-is for unknown types
//
// Bead: bf-65co1
func TestNormalizeYAMLTypeEmptyAndMissingInputs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		reason   string
	}{
		// === EMPTY AND WHITESPACE ===
		{
			name:     "empty string returns empty",
			input:    "",
			expected: "",
			reason:   "Empty input returns empty string, not panic or nil",
		},
		{
			name:     "whitespace only returns empty",
			input:    "   ",
			expected: "",
			reason:   "Whitespace-only input trims to empty string",
		},
		{
			name:     "tabs only returns empty",
			input:    "\t\t",
			expected: "",
			reason:   "Tab-only input trims to empty string",
		},
		{
			name:     "mixed whitespace returns empty",
			input:    "  \t\n\r  ",
			expected: "",
			reason:   "Mixed whitespace trims to empty string",
		},

		// === UNKNOWN/INVALID TYPES ===
		{
			name:     "unknown type returns as-is",
			input:    "CustomType",
			expected: "CustomType",
			reason:   "Unknown types are returned unchanged",
		},
		{
			name:     "unknown type with numbers returns as-is",
			input:    "Type123",
			expected: "Type123",
			reason:   "Unknown types with numbers are returned unchanged",
		},
		{
			name:     "unknown qualified type returns as-is",
			input:    "mypkg.CustomType",
			expected: "CustomType",
			reason:   "Package qualifier is stripped from unknown types",
		},
		{
			name:     "lowercase unknown type returns as-is",
			input:    "unknowntype",
			expected: "unknowntype",
			reason:   "Unknown lowercase types are returned unchanged",
		},

		// === EDGE CASES WITH SPECIAL CHARACTERS ===
		{
			name:     "type with trailing comma returns as-is",
			input:    "string,",
			expected: "string,",
			reason:   "Trailing punctuation is NOT stripped by normalizer",
		},
		{
			name:     "type with trailing period returns as-is",
			input:    "int.",
			expected: "int.",
			reason:   "Trailing punctuation is NOT stripped by normalizer",
		},
		{
			name:     "type with leading whitespace",
			input:    "  string",
			expected: "string",
			reason:   "Leading whitespace is trimmed",
		},
		{
			name:     "type with trailing whitespace",
			input:    "string  ",
			expected: "string",
			reason:   "Trailing whitespace is trimmed",
		},
		{
			name:     "type with both whitespace",
			input:    "  string  ",
			expected: "string",
			reason:   "Both leading and trailing whitespace are trimmed",
		},

		// === UNICODE IN TYPES ===
		{
			name:     "type with unicode characters returns as-is",
			input:    "stringエラー",
			expected: "stringエラー",
			reason:   "Unicode characters in custom types are preserved",
		},

		// === VERY LONG TYPE NAMES ===
		{
			name:     "very long type name returns as-is",
			input:    "VeryLongComplexTypeNameThatKeepsGoingAndGoing",
			expected: "VeryLongComplexTypeNameThatKeepsGoingAndGoing",
			reason:   "Very long custom type names are returned unchanged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeYAMLType(tt.input)

			if result != tt.expected {
				t.Errorf("normalizeYAMLType(%q) = %q, want %q\n  Reason: %s",
					tt.input, result, tt.expected, tt.reason)
			}
		})
	}
}
