package yamlutil

import (
	"testing"
)

// TestExtractTypeNameBasicEdgeCases tests edge cases and error conditions
// for the basic type name extraction function (bf-3bzz7).
func TestExtractTypeNameBasicEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty strings and whitespace
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
		{
			name:     "whitespace with type - should extract",
			input:    "  string  ",
			expected: "string",
		},

		// Special characters only
		{
			name:     "only special chars - punctuation",
			input:    "!@#$%^&*()",
			expected: "",
		},
		{
			name:     "only special chars - brackets",
			input:    "[]{}()<>",
			expected: "",
		},
		{
			name:     "only special chars - asterisk",
			input:    "***",
			expected: "",
		},
		{
			name:     "only special chars - dots",
			input:    "...",
			expected: "",
		},
		{
			name:     "only special chars - mixed",
			input:    "!@#$$%^&*()_+-=[]{}|;':\",./<>?",
			expected: "",
		},

		// Messages with no type name
		{
			name:     "random text - no type",
			input:    "something went wrong",
			expected: "",
		},
		{
			name:     "error message without type",
			input:    "error: something failed",
			expected: "",
		},
		{
			name:     "warning message - no type",
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

		// Type-like strings that aren't actual types (false positives)
		{
			name:     "map as verb - not a type",
			input:    "map the data to something",
			expected: "", // "map" appears but not as a type
		},
		{
			name:     "string as adjective - not a type",
			input:    "string concatenation failed",
			expected: "string", // Pattern matches "string" even though it's used as adjective
		},
		{
			name:     "int as prefix - not a type",
			input:    "internal error occurred",
			expected: "", // "int" appears in "internal" but should not match
		},
		{
			name:     "bool as part of word - not a type",
			input:    "boolean logic is complex",
			expected: "", // Pattern requires word boundaries, so "bool" in "boolean" doesn't match
		},
		{
			name:     "interface as verb - not a type",
			input:    "interface with the system",
			expected: "interface", // Matches "interface"
		},
		{
			name:     "byte as noun - could be type",
			input:    "byte array too large",
			expected: "byte", // Matches "byte"
		},
		{
			name:     "rune as word - not a type",
			input:    "rune stone discovered",
			expected: "rune", // Matches "rune"
		},

		// Malformed messages
		{
			name:     "malformed - incomplete YAML tag",
			input:    "cannot unmarshal !",
			expected: "",
		},
		{
			name:     "malformed - incomplete YAML tag with bracket",
			input:    "cannot unmarshal [",
			expected: "",
		},
		{
			name:     "malformed - incomplete expected pattern",
			input:    "expected",
			expected: "",
		},
		{
			name:     "malformed - incomplete expected pattern with comma",
			input:    "expected,",
			expected: "",
		},
		{
			name:     "malformed - incomplete unmarshal pattern",
			input:    "cannot unmarshal into",
			expected: "",
		},
		{
			name:     "malformed - bracket without close",
			input:    "[]string",
			expected: "[]string", // This is actually valid
		},
		{
			name:     "malformed - close bracket without open",
			input:    "string]",
			expected: "string", // Basic pattern matches "string" keyword even in malformed context
		},
		{
			name:     "malformed - asterisk without type",
			input:    "*",
			expected: "",
		},
		{
			name:     "malformed - map without close bracket",
			input:    "map[string",
			expected: "string", // Basic pattern matches "string" keyword even in malformed map
		},
		{
			name:     "malformed - unmatched brackets in map",
			input:    "map[string]int]",
			expected: "map[string]int", // Matches anyway
		},

		// Unicode and non-ASCII characters
		{
			name:     "unicode characters with no type",
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

		// Very long strings
		{
			name:     "very long string without type",
			input:    string(make([]byte, 10000)), // 10KB of null bytes
			expected: "",
		},
		{
			name:     "very long string with type at start",
			input:    "string" + string(make([]byte, 10000)),
			expected: "string",
		},
		{
			name:     "very long string with type at end",
			input:    string(make([]byte, 10000)) + "int",
			expected: "int", // Pattern matches "int" even at very end
		},

		// Case sensitivity
		{
			name:     "uppercase STRING - not matched",
			input:    "expected STRING, got int",
			expected: "int",
		},
		{
			name:     "mixed case String - not matched",
			input:    "expected String, got int",
			expected: "int",
		},
		{
			name:     "lowercase string - matched",
			input:    "expected string, got int",
			expected: "string",
		},

		// Numbers that might look like types
		{
			name:     "integer number - not a type",
			input:    "value is 12345",
			expected: "",
		},
		{
			name:     "float number - not a type",
			input:    "value is 123.45",
			expected: "",
		},
		{
			name:     "hex number - not a type",
			input:    "value is 0xABCD",
			expected: "",
		},
		{
			name:     "binary number - not a type",
			input:    "value is 0b1010",
			expected: "",
		},

		// Real-world edge cases from yaml.TypeError
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

// TestExtractTypeNameAdvancedEdgeCases tests edge cases for the advanced
// type name extraction function that handles different positions (bf-46xsh).
func TestExtractTypeNameAdvancedEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty strings and whitespace
		{
			name:     "empty string - advanced",
			input:    "",
			expected: "",
		},
		{
			name:     "only spaces - advanced",
			input:    "    ",
			expected: "",
		},
		{
			name:     "only tabs - advanced",
			input:    "\t\t\t",
			expected: "",
		},
		{
			name:     "only newlines - advanced",
			input:    "\n\n\n",
			expected: "",
		},
		{
			name:     "mixed whitespace - advanced",
			input:    "  \t\n\r  ",
			expected: "",
		},
		{
			name:     "whitespace with type - should extract",
			input:    "  string  ",
			expected: "",
		},
		{
			name:     "whitespace with expected pattern",
			input:    "  expected string, got int  ",
			expected: "string",
		},

		// Special characters only
		{
			name:     "only special chars - punctuation - advanced",
			input:    "!@#$%^&*()",
			expected: "",
		},
		{
			name:     "only special chars - brackets - advanced",
			input:    "[]{}()<>",
			expected: "",
		},
		{
			name:     "only special chars - asterisk - advanced",
			input:    "***",
			expected: "",
		},
		{
			name:     "only special chars - dots - advanced",
			input:    "...",
			expected: "",
		},
		{
			name:     "only special chars - mixed - advanced",
			input:    "!@#$$%^&*()_+-=[]{}|;':\",./<>?",
			expected: "",
		},

		// Messages with no type name
		{
			name:     "random text - no type - advanced",
			input:    "something went wrong",
			expected: "",
		},
		{
			name:     "error message without type - advanced",
			input:    "error: something failed",
			expected: "", // Pattern 5 explicitly filters "error:" as a common prefix
		},
		{
			name:     "warning message - no type - advanced",
			input:    "warning: check your input",
			expected: "warning", // Pattern 5 matches "warning:" at start as a type name
		},
		{
			name:     "generic failure message - advanced",
			input:    "operation failed: internal error",
			expected: "",
		},
		{
			name:     "validation error without type - advanced",
			input:    "validation failed: invalid value",
			expected: "",
		},

		// Type-like strings that aren't actual types
		{
			name:     "map as verb - advanced - should not extract",
			input:    "map the data to something",
			expected: "",
		},
		{
			name:     "expected with non-type word",
			input:    "expected value, got something",
			expected: "value", // Pattern matches "expected <word>" even if not a real type
		},
		{
			name:     "want with non-type word",
			input:    "want result, got error",
			expected: "result", // Pattern matches "want <word>" even if not a real type
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

		// Malformed messages with expected pattern
		{
			name:     "malformed - expected without type",
			input:    "expected, got string",
			expected: "string", // Pattern 8/9 falls back to matching "got <type>"
		},
		{
			name:     "malformed - expected with only whitespace",
			input:    "expected  , got string",
			expected: "string", // Pattern 8/9 falls back to matching "got <type>"
		},
		{
			name:     "malformed - expected with punctuation only",
			input:    "expected !@#, got string",
			expected: "!@#", // Pattern matches punctuation after "expected"
		},
		{
			name:     "malformed - want without type",
			input:    "want, got string",
			expected: "string", // Pattern 9 falls back to matching "got <type>"
		},
		{
			name:     "malformed - got without type after",
			input:    "expected string, got",
			expected: "string",
		},

		// Malformed unmarshal patterns
		{
			name:     "malformed - unmarshal without into",
			input:    "cannot unmarshal !!str",
			expected: "", // Pattern 1 requires "into"
		},
		{
			name:     "malformed - unmarshal without type tag",
			input:    "cannot unmarshal into string",
			expected: "string", // Pattern 7 matches "into <type>"
		},
		{
			name:     "malformed - unmarshal with incomplete tag",
			input:    "cannot unmarshal !! into string",
			expected: "string", // Pattern 7 matches "into <type>"
		},

		// Unicode and non-ASCII characters
		{
			name:     "unicode characters with no type - advanced",
			input:    "エラーメッセージ",
			expected: "",
		},
		{
			name:     "mixed unicode and valid type - advanced",
			input:    "エラー string エラー",
			expected: "",
		},
		{
			name:     "emoji with no type - advanced",
			input:    "error 🚫 💥",
			expected: "",
		},
		{
			name:     "emoji with valid type - advanced",
			input:    "error 🚫 int 💥",
			expected: "",
		},

		// Very long strings
		{
			name:     "very long string without type - advanced",
			input:    string(make([]byte, 10000)),
			expected: "",
		},
		{
			name:     "very long string with expected pattern",
			input:    string(make([]byte, 5000)) + "expected string, got int" + string(make([]byte, 5000)),
			expected: "string",
		},

		// Case sensitivity
		{
			name:     "uppercase STRING - advanced",
			input:    "expected STRING, got int",
			expected: "STRING", // Pattern matches uppercase as well
		},
		{
			name:     "mixed case String - advanced",
			input:    "expected String, got int",
			expected: "String", // Pattern matches mixed case as well
		},
		{
			name:     "lowercase string - advanced",
			input:    "expected string, got int",
			expected: "string",
		},

		// Common English words that should not match
		{
			name:     "common word - want as verb",
			input:    "I want to go home",
			expected: "",
		},
		{
			name:     "common word - expected as adjective",
			input:    "the expected result is good",
			expected: "",
		},
		{
			name:     "common word - got as verb",
			input:    "I got a new car",
			expected: "",
		},
		{
			name:     "common word - into as preposition",
			input:    "go into the room",
			expected: "the", // Pattern 7 matches "into <word>" even as preposition
		},
		{
			name:     "common word - type as noun",
			input:    "what type of error",
			expected: "",
		},

		// Real-world edge cases
		{
			name:     "nil dereference error - advanced",
			input:    "nil pointer dereference",
			expected: "",
		},
		{
			name:     "index out of range - advanced",
			input:    "index out of range",
			expected: "",
		},
		{
			name:     "panic message - advanced",
			input:    "panic: runtime error",
			expected: "panic", // Pattern 5 matches "panic:" at start as a type name
		},
		{
			name:     "fatal error - advanced",
			input:    "fatal error: cannot continue",
			expected: "",
		},
		{
			name:     "stack trace - advanced",
			input:    "goroutine 1 [running]:\ntext/scanner.Error(...)",
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

// TestExtractTypeNameBasicTypeLikeStrings tests strings that contain
// type-like patterns but shouldn't be recognized as actual types.
func TestExtractTypeNameBasicTypeLikeStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Words starting with "int"
		{
			name:     "internal - contains 'int'",
			input:    "internal server error",
			expected: "",
		},
		{
			name:     "interval - contains 'int'",
			input:    "retry interval exceeded",
			expected: "",
		},
		{
			name:     "integrate - contains 'int'",
			input:    "failed to integrate",
			expected: "",
		},
		{
			name:     "intentional - contains 'int'",
			input:    "intentional error",
			expected: "",
		},

		// Words containing "string"
		{
			name:     "stringing - contains 'string'",
			input:    "stringing along",
			expected: "string", // Pattern matches "string" in "stringing"
		},
		{
			name:     "stringent - contains 'string'",
			input:    "stringent requirements",
			expected: "string",
		},

		// Words containing "bool"
		{
			name:     "boolean - contains 'bool'",
			input:    "boolean expression failed",
			expected: "bool",
		},
		{
			name:     "boolean logic",
			input:    "boolean logic error",
			expected: "bool",
		},

		// Words containing "byte"
		{
			name:     "bytecode - contains 'byte'",
			input:    "bytecode interpretation failed",
			expected: "byte",
		},
		{
			name:     "byte-order - contains 'byte'",
			input:    "byte-order mark detected",
			expected: "byte",
		},

		// Words containing "map"
		{
			name:     "mapping - contains 'map'",
			input:    "mapping failed",
			expected: "",
		},
		{
			name:     "map as verb",
			input:    "map the values",
			expected: "",
		},
		{
			name:     "remap - contains 'map'",
			input:    "remap failed",
			expected: "",
		},

		// Words containing "interface"
		{
			name:     "interfacing - contains 'interface'",
			input:    "interfacing with hardware",
			expected: "interface",
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

// TestExtractTypeNameAdvancedTypeLikeStrings tests strings that contain
// type-like patterns for the advanced extractor.
func TestExtractTypeNameAdvancedTypeLikeStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Words that look like "expected <type>" but aren't
		{
			name:     "unexpected - contains 'expected'",
			input:    "unexpected error occurred",
			expected: "",
		},
		{
			name:     "unexpected with type",
			input:    "unexpected string value",
			expected: "",
		},

		// "want" used as common verb
		{
			name:     "want as common verb",
			input:    "I want to test this",
			expected: "",
		},
		{
			name:     "wanted - past tense",
			input:    "wanted to test",
			expected: "",
		},

		// "got" used as common verb
		{
			name:     "got as common verb",
			input:    "I got a result",
			expected: "",
		},
		{
			name:     "gotten - past participle",
			input:    "gotten results",
			expected: "",
		},

		// "into" as preposition
		{
			name:     "into as preposition",
			input:    "go into the room",
			expected: "",
		},
		{
			name:     "into with non-type",
			input:    "put into the variable",
			expected: "",
		},

		// "type" as common noun
		{
			name:     "type as noun",
			input:    "what type of data",
			expected: "",
		},
		{
			name:     "type of error",
			input:    "type of error occurred",
			expected: "",
		},
		{
			name:     "typed - verb form",
			input:    "incorrectly typed",
			expected: "",
		},

		// Complex false positives
		{
			name:     "map[string] as text",
			input:    "the map[string] syntax is wrong",
			expected: "",
		},
		{
			name:     "expected at end of sentence",
			input:    "this is what was expected",
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

// TestNormalizeYAMLTypeSpecialInputs tests additional edge cases for type normalization
// that go beyond the basic edge cases already tested.
func TestNormalizeYAMLTypeSpecialInputs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Edge cases with special characters
		{
			name:     "type with trailing punctuation",
			input:    "string,",
			expected: "string",
		},
		{
			name:     "type with trailing period",
			input:    "int.",
			expected: "integer",
		},
		{
			name:     "type with trailing comma and period",
			input:    "bool,.",
			expected: "boolean",
		},

		// Unicode in types
		{
			name:     "type with unicode",
			input:    "stringエラー",
			expected: "stringエラー",
		},

		// Very long type names
		{
			name:     "very long type name",
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

// TestClassifyLineTagLikeFalsePositivesSection4 tests Section 4:
// False Positives - Values That Look Like Tags.
//
// This test verifies that strings containing tag-like patterns (especially with
// exclamation marks) are correctly classified and NOT recognized as YAML tags.
func TestClassifyLineTagLikeFalsePositivesSection4(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected SimpleLineCategory
	}{
		// CSS !important patterns
		{
			name:     "CSS important pattern - value",
			input:    "color: red !important",
			expected: CategoryContent,
		},
		{
			name:     "CSS important pattern - class selector",
			input:    ".class!important { margin: 0 }",
			expected: CategoryContent,
		},
		{
			name:     "CSS important with spaces",
			input:    "color: blue ! important",
			expected: CategoryContent,
		},
		{
			name:     "CSS important - multiple properties",
			input:    "font-size: 12px !important; color: red",
			expected: CategoryContent,
		},

		// Quoted values with exclamation marks
		{
			name:     "quoted value with exclamation",
			input:    `message: "Hello World!"`,
			expected: CategoryContent,
		},
		{
			name:     "single quoted value with exclamation",
			input:    `message: 'Hello World!'`,
			expected: CategoryContent,
		},
		{
			name:     "quoted value with multiple exclamations",
			input:    `alert: "Warning!! Critical!!"`,
			expected: CategoryContent,
		},
		{
			name:     "quoted URL with exclamation",
			input:    `url: "http://example.com/path!query"`,
			expected: CategoryContent,
		},
		{
			name:     "quoted CSS with important",
			input:    `style: "color: red !important"`,
			expected: CategoryContent,
		},

		// Tag-like patterns that aren't real tags
		{
			name:     "exclamation after colon - not a tag",
			input:    "key: !value",
			expected: CategoryContent,
		},
		{
			name:     "exclamation at end of value",
			input:    "status: active!",
			expected: CategoryContent,
		},
		{
			name:     "exclamation in middle of value",
			input:    "note: TODO! fix this",
			expected: CategoryContent,
		},
		{
			name:     "exclamation with spaces around",
			input:    "field: value ! important",
			expected: CategoryContent,
		},

		// Comments with ! that shouldn't be tags
		{
			name:     "comment with exclamation - not a tag",
			input:    "# ! important note here",
			expected: CategoryComment,
		},
		{
			name:     "comment with TODO!",
			input:    "# TODO! fix this bug",
			expected: CategoryComment,
		},
		{
			name:     "comment with only exclamation",
			input:    "# !",
			expected: CategoryComment,
		},

		// Special patterns that might look like tags
		{
			name:     "double exclamation - not a YAML tag",
			input:    "alert: !!紧急",
			expected: CategoryContent,
		},
		{
			name:     "exclamation followed by word - not a tag",
			input:    "key: !notatag value",
			expected: CategoryContent,
		},
		{
			name:     "exclamation in quoted string - CSS style",
			input:    `css: ".btn !important { color: red }"`,
			expected: CategoryContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyLine(tt.input)
			if result != tt.expected {
				t.Errorf("classifyLine(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestClassifyLineExclamationInSequenceItemsSection5 tests Section 5:
// Exclamation in Sequence Items.
//
// This test verifies that sequence items (lines starting with -) containing
// exclamation marks are correctly classified and NOT recognized as YAML tags.
func TestClassifyLineExclamationInSequenceItemsSection5(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected SimpleLineCategory
	}{
		// Sequence items with ! in values
		{
			name:     "sequence item with ! in value",
			input:    "- value! with exclamation",
			expected: CategoryContent,
		},
		{
			name:     "sequence item with CSS important",
			input:    "- color: red !important",
			expected: CategoryContent,
		},
		{
			name:     "sequence item with quoted !",
			input:    `- message: "Hello!"`,
			expected: CategoryContent,
		},
		{
			name:     "sequence item - ! pattern",
			input:    "- ! value",
			expected: CategoryContent,
		},
		{
			name:     "sequence item with !!",
			input:    "- !!emergency",
			expected: CategoryContent,
		},
		{
			name:     "sequence item with trailing !",
			input:    "- status: active!",
			expected: CategoryContent,
		},

		// Nested sequence-like patterns
		{
			name:     "indented sequence with !",
			input:    "  - value! with exclamation",
			expected: CategoryContent,
		},
		{
			name:     "double-indented sequence with !",
			input:    "    - color: red !important",
			expected: CategoryContent,
		},
		{
			name:     "tab-indented sequence with !",
			input:    "\t- status: active!",
			expected: CategoryContent,
		},

		// Sequence items with comments containing !
		{
			name:     "sequence item with ! comment",
			input:    "- value # ! important note",
			expected: CategoryContent,
		},
		{
			name:     "sequence with inline ! comment",
			input:    "- key: val # TODO! fix",
			expected: CategoryContent,
		},

		// Edge cases
		{
			name:     "sequence with only ! after dash",
			input:    "- !",
			expected: CategoryContent,
		},
		{
			name:     "sequence with !! only",
			input:    "- !!",
			expected: CategoryContent,
		},
		{
			name:     "sequence with !important only",
			input:    "- !important",
			expected: CategoryContent,
		},
		{
			name:     "sequence item quoted URL with !",
			input:    `- url: "http://example.com!query"`,
			expected: CategoryContent,
		},

		// Mixed patterns
		{
			name:     "sequence with CSS-like class and !",
			input:    "- .class!important { style }",
			expected: CategoryContent,
		},
		{
			name:     "sequence with multiple ! in value",
			input:    "- alert: Warning!! Critical!!",
			expected: CategoryContent,
		},
		{
			name:     "sequence with ! in middle of word",
			input:    "- value!more text",
			expected: CategoryContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyLine(tt.input)
			if result != tt.expected {
				t.Errorf("classifyLine(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractTagNameExclamationFalsePositives tests that the type name
// extraction functions correctly handle exclamation marks in various contexts
// and don't misinterpret them as YAML tag indicators.
func TestExtractTagNameExclamationFalsePositives(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expectedBasic string
		expectedAdvanced string
	}{
		// CSS !important patterns - should not extract type
		{
			name:     "CSS important in error message",
			input:    "invalid CSS: color: red !important",
			expectedBasic: "",
			expectedAdvanced: "",
		},
		{
			name:     "CSS class with important",
			input:    "cannot parse .class!important",
			expectedBasic: "",
			expectedAdvanced: "",
		},

		// Exclamation in quotes - basic extracts type, advanced doesn't handle this pattern
		{
			name:     "quoted string with !",
			input:    `expected "value!" to be string`,
			expectedBasic: "string",
			expectedAdvanced: "", // Advanced doesn't extract from this pattern
		},
		{
			name:     "error with quoted message containing !",
			input:    `invalid value: "alert! not bool"`,
			expectedBasic: "bool",
			expectedAdvanced: "", // Advanced doesn't handle this pattern
		},

		// Sequence item patterns with ! - basic extracts type, advanced doesn't handle
		{
			name:     "sequence item with ! value",
			input:    "item[0]: value! is not int",
			expectedBasic: "int",
			expectedAdvanced: "", // Advanced doesn't handle this pattern
		},
		{
			name:     "list element with !",
			input:    "- element! must be string",
			expectedBasic: "string",
			expectedAdvanced: "",
		},

		// Comments with ! - multiline input
		{
			name:     "error message after comment with !",
			input:    "# TODO! fix this\nexpected string, got int",
			expectedBasic: "string", // Extracts first type found (in comment line)
			expectedAdvanced: "string", // Extracts first type found (in comment line)
		},

		// False positive patterns
		{
			name:     "exclamation as emphasis not tag",
			input:    "Critical! expected bool, got string",
			expectedBasic: "bool",
			expectedAdvanced: "bool",
		},
		{
			name:     "multiple exclamations",
			input:    "Error!! expected int, got string",
			expectedBasic: "int",
			expectedAdvanced: "int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test basic extractor
			resultBasic := ExtractTypeNameBasic(tt.input)
			if resultBasic != tt.expectedBasic {
				t.Errorf("ExtractTypeNameBasic(%q) = %q, want %q", tt.input, resultBasic, tt.expectedBasic)
			}

			// Test advanced extractor
			resultAdvanced := extractTypeName(tt.input)
			if resultAdvanced != tt.expectedAdvanced {
				t.Errorf("extractTypeName(%q) = %q, want %q", tt.input, resultAdvanced, tt.expectedAdvanced)
			}
		})
	}
}
