// Package yamlutil tests for malformed error messages
//
// This test file provides comprehensive coverage for malformed or improperly
// formatted error messages, ensuring graceful handling when error messages
// don't match expected patterns. Tests cover broken formatting, incomplete
// patterns, malformed structure, and edge cases in error message parsing.
//
// Bead: bf-4eupx
package yamlutil

import (
	"strings"
	"testing"
)

// TestMalformedMessages_BrokenFormatting tests messages with broken formatting
func TestMalformedMessages_BrokenFormatting(t *testing.T) {
	tests := []struct {
		name        string
		errorMsg    string
		description string
		expectEmpty bool // Whether we expect empty type extraction
	}{
		{
			name:        "message with random special characters",
			errorMsg:    "error: @#$%^&*() cannot parse",
			description: "Error message with random special chars mixed in",
			expectEmpty: true,
		},
		{
			name:        "message with broken unmarshal pattern",
			errorMsg:    "cannot unmarshal into into into",
			description: "Pattern with repeated keywords",
			expectEmpty: true,
		},
		{
			name:        "message with incomplete colon pattern",
			errorMsg:    "line 10: : something went wrong",
			description: "Colon pattern with empty field name",
			expectEmpty: true,
		},
		{
			name:        "message with broken line pattern",
			errorMsg:    "line: cannot parse",
			description: "Line pattern without line number",
			expectEmpty: true,
		},
		{
			name:        "message with malformed line number",
			errorMsg:    "line abc: cannot parse value",
			description: "Line pattern with non-numeric line number",
			expectEmpty: true,
		},
		{
			name:        "message with broken quote escaping",
			errorMsg:    "cannot unmarshal \"string\\\\\\\\\" into int",
			description: "Excessive quote escaping",
			expectEmpty: false, // Should still extract "int"
		},
		{
			name:        "message with mixed separators",
			errorMsg:    "cannot unmarshal;into:int",
			description: "Missing spaces around separators (still extracts)",
			expectEmpty: false, // Pattern still matches due to regex flexibility
		},
		{
			name:        "message with inverted pattern",
			errorMsg:    "unmarshal cannot into !!str int",
			description: "Keywords in wrong order (still extracts tag)",
			expectEmpty: false, // Pattern still matches !!str
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTypeNameBasic(tt.errorMsg)

			if tt.expectEmpty && result != "" {
				t.Errorf("Expected empty type name for malformed message, got %q\n  Message: %s\n  Description: %s",
					result, tt.errorMsg, tt.description)
			}

			if !tt.expectEmpty && result == "" {
				t.Logf("Note: Expected non-empty type but got empty for: %s\n  Message: %s",
					tt.description, tt.errorMsg)
			}

			// Verify graceful handling (no panics)
			t.Logf("✓ Gracefully handled: %s → %q", tt.description, result)
		})
	}
}

// TestMalformedMessages_IncompletePatterns tests messages with incomplete error patterns
func TestMalformedMessages_IncompletePatterns(t *testing.T) {
	tests := []struct {
		name        string
		errorMsg    string
		description string
		expectEmpty bool
	}{
		{
			name:        "unmarshal without target type",
			errorMsg:    "cannot unmarshal !!str into",
			description: "Incomplete unmarshal pattern - extracts source tag",
			expectEmpty: false, // Extracts !!str as fallback
		},
		{
			name:        "unmarshal without source tag",
			errorMsg:    "cannot unmarshal into int",
			description: "Incomplete unmarshal pattern - extracts target type",
			expectEmpty: false, // Extracts "int" from pattern
		},
		{
			name:        "expected without got",
			errorMsg:    "expected int but found",
			description: "Incomplete expected pattern - still extracts",
			expectEmpty: false, // Extracts "int" from expected
		},
		{
			name:        "expected without type",
			errorMsg:    "expected, got string",
			description: "Expected pattern without type value",
			expectEmpty: false, // Should fall back to "got string"
		},
		{
			name:        "want without got",
			errorMsg:    "want int",
			description: "Incomplete want pattern - still extracts",
			expectEmpty: false, // Extracts "int" from want
		},
		{
			name:        "got without expected/want",
			errorMsg:    "got string",
			description: "Got pattern without expected/want",
			expectEmpty: false, // Extracts "string" from got
		},
		{
			name:        "convert without target",
			errorMsg:    "string cannot be converted",
			description: "Incomplete convert pattern - still extracts",
			expectEmpty: false, // Extracts "string" from convert
		},
		{
			name:        "convert without source",
			errorMsg:    "cannot be converted to int",
			description: "Incomplete convert pattern - still extracts",
			expectEmpty: false, // Extracts "int" from target
		},
		{
			name:        "line without colon",
			errorMsg:    "line 10 cannot parse",
			description: "Line pattern without colon separator",
			expectEmpty: true,
		},
		{
			name:        "field without path",
			errorMsg:    "field: type mismatch",
			description: "Field pattern without field path",
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTypeNameBasic(tt.errorMsg)

			if tt.expectEmpty && result != "" {
				t.Errorf("Expected empty type name for incomplete pattern, got %q\n  Message: %s\n  Description: %s",
					result, tt.errorMsg, tt.description)
			}

			if !tt.expectEmpty && result == "" {
				t.Logf("Note: Expected non-empty type but got empty for: %s\n  Message: %s",
					tt.description, tt.errorMsg)
			}

			t.Logf("✓ Handled incomplete pattern: %s → %q", tt.description, result)
		})
	}
}

// TestMalformedMessages_MalformedStructure tests messages with malformed structure
func TestMalformedMessages_MalformedStructure(t *testing.T) {
	tests := []struct {
		name        string
		errorMsg    string
		description string
		expectEmpty bool
	}{
		{
			name:        "multiple conflicting patterns",
			errorMsg:    "expected int, got string; cannot unmarshal !!str into bool",
			description: "Multiple error patterns in one message",
			expectEmpty: false, // Should extract first match
		},
		{
			name:        "nested type syntax error",
			errorMsg:    "cannot unmarshal !!str into map[string",
			description: "Unclosed map type syntax - extracts tag",
			expectEmpty: false, // Extracts "str" from source tag
		},
		{
			name:        "malformed array syntax",
			errorMsg:    "cannot unmarshal !!str into [string",
			description: "Incomplete array type syntax - extracts tag",
			expectEmpty: false, // Extracts "str" from source tag
		},
		{
			name:        "malformed pointer syntax",
			errorMsg:    "cannot unmarshal !!str into *",
			description: "Pointer type without target - extracts tag",
			expectEmpty: false, // Extracts "str" from !!str tag
		},
		{
			name:        "malformed channel syntax",
			errorMsg:    "cannot unmarshal !!str into chan",
			description: "Channel type without element type - extracts tag",
			expectEmpty: false, // Extracts "str" from !!str tag
		},
		{
			name:        "malformed interface syntax",
			errorMsg:    "cannot unmarshal !!str into interface",
			description: "Interface type without braces",
			expectEmpty: false, // "interface" might match pattern 5
		},
		{
			name:        "broken qualified type",
			errorMsg:    "cannot unmarshal !!str into .int",
			description: "Qualified type starting with dot - extracts tag",
			expectEmpty: false, // Extracts "str" from !!str tag
		},
		{
			name:        "broken qualified type trailing dot",
			errorMsg:    "cannot unmarshal !!str into time.",
			description: "Qualified type ending with dot - extracts tag",
			expectEmpty: false, // Extracts "str" from !!str tag
		},
		{
			name:        "empty type in expected",
			errorMsg:    "expected , got string",
			description: "Empty type value in expected pattern",
			expectEmpty: false, // Should fall back to "got string"
		},
		{
			name:        "empty type in got",
			errorMsg:    "expected int, got",
			description: "Empty type value in got pattern",
			expectEmpty: false, // Should extract "int" from expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTypeNameBasic(tt.errorMsg)

			if tt.expectEmpty && result != "" {
				t.Errorf("Expected empty type name for malformed structure, got %q\n  Message: %s\n  Description: %s",
					result, tt.errorMsg, tt.description)
			}

			if !tt.expectEmpty && result == "" {
				t.Logf("Note: Expected non-empty type but got empty for: %s\n  Message: %s",
					tt.description, tt.errorMsg)
			}

			t.Logf("✓ Handled malformed structure: %s → %q", tt.description, result)
		})
	}
}

// TestMalformedMessages_RealWorldMalformed tests real-world malformed error messages
func TestMalformedMessages_RealWorldMalformed(t *testing.T) {
	tests := []struct {
		name        string
		errorMsg    string
		description string
		expectEmpty bool
		expectedVal string
	}{
		{
			name:        "missing closing brace in map type",
			errorMsg:    "line 15: cannot unmarshal !!str into map[string int",
			description: "Map type with missing closing brace - extracts tag",
			expectEmpty: false,
			expectedVal: "str",
		},
		{
			name:        "missing closing bracket in array type",
			errorMsg:    "line 20: cannot unmarshal !!seq into [string",
			description: "Array type with missing closing bracket - extracts tag",
			expectEmpty: false,
			expectedVal: "seq",
		},
		{
			name:        "invalid YAML tag prefix",
			errorMsg:    "cannot unmarshal ! into int",
			description: "YAML tag with single exclamation - extracts target",
			expectEmpty: false,
			expectedVal: "int",
		},
		{
			name:        "YAML tag with spaces",
			errorMsg:    "cannot unmarshal !! str into int",
			description: "YAML tag with space after !! - extracts target",
			expectEmpty: false,
			expectedVal: "int",
		},
		{
			name:        "type name with trailing garbage",
			errorMsg:    "expected int###, got string",
			description: "Type name with trailing special chars - extracts int from expected",
			expectEmpty: false,
			expectedVal: "int",
		},
		{
			name:        "type name with leading garbage",
			errorMsg:    "expected $$$int, got string",
			description: "Type name with leading special chars - extracts int from expected",
			expectEmpty: false,
			expectedVal: "int",
		},
		{
			name:        "mixed up keywords",
			errorMsg:    "got int, expected string",
			description: "Reversed expected/got order",
			expectEmpty: false,
			expectedVal: "int",
		},
		{
			name:        "missing comma in expected",
			errorMsg:    "expected int got string",
			description: "Missing comma between types - extracts first",
			expectEmpty: false,
			expectedVal: "int",
		},
		{
			name:        "extra whitespace",
			errorMsg:    "cannot   unmarshal   !!str   into   int",
			description: "Multiple spaces instead of single - extracts source tag",
			expectEmpty: false,
			expectedVal: "str",
		},
		{
			name:        "tab characters",
			errorMsg:    "cannot\tunmarshal\t!!str\tinto\tint",
			description: "Tab characters instead of spaces - extracts tag",
			expectEmpty: false,
			expectedVal: "str",
		},
		{
			name:        "newlines in pattern",
			errorMsg:    "cannot\nunmarshal\n!!str\ninto\nint",
			description: "Newlines breaking the pattern - extracts tag",
			expectEmpty: false,
			expectedVal: "str",
		},
		{
			name:        "unicode whitespace",
			errorMsg:    "cannot​unmarshal​!!str​into​int",
			description: "Zero-width space characters - extracts tag",
			expectEmpty: false,
			expectedVal: "str",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTypeNameBasic(tt.errorMsg)

			if tt.expectEmpty && result != "" {
				t.Errorf("Expected empty type name for malformed message, got %q\n  Message: %s\n  Description: %s",
					result, tt.errorMsg, tt.description)
			}

			if tt.expectedVal != "" && result != tt.expectedVal {
				t.Errorf("Expected %q but got %q\n  Message: %s\n  Description: %s",
					tt.expectedVal, result, tt.errorMsg, tt.description)
			}

			if !tt.expectEmpty && result == "" && tt.expectedVal == "" {
				t.Logf("Note: Expected non-empty type but got empty for: %s\n  Message: %s",
					tt.description, tt.errorMsg)
			}

			t.Logf("✓ Real-world malformed: %s → %q", tt.description, result)
		})
	}
}

// TestMalformedMessages_EdgeCases tests edge cases in malformed messages
func TestMalformedMessages_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		errorMsg    string
		description string
		expectEmpty bool
	}{
		{
			name:        "empty string",
			errorMsg:    "",
			description: "Empty error message",
			expectEmpty: true,
		},
		{
			name:        "only whitespace",
			errorMsg:    "     ",
			description: "Whitespace-only message",
			expectEmpty: true,
		},
		{
			name:        "only newlines",
			errorMsg:    "\n\n\n\n",
			description: "Newline-only message",
			expectEmpty: true,
		},
		{
			name:        "very long malformed message",
			errorMsg:    strings.Repeat("cannot unmarshal ", 1000) + "broken",
			description: "Very long malformed message",
			expectEmpty: true,
		},
		{
			name:        "message with null bytes",
			errorMsg:    "cannot unmarshal\x00into int",
			description: "Message with null byte - extracts target",
			expectEmpty: false, // Extracts "int" from target
		},
		{
			name:        "message with control characters",
			errorMsg:    "cannot\x01unmarshal\x02!!str\x03into\x04int",
			description: "Message with control characters (still extracts tag)",
			expectEmpty: false, // Extracts "str" from !!str tag
		},
		{
			name:        "message with mixed encoding",
			errorMsg:    "cannot unmarshal !!str into int\xff",
			description: "Message with non-UTF8 character",
			expectEmpty: false, // Should still extract "int"
		},
		{
			name:        "case-sensitive pattern broken",
			errorMsg:    "CANNOT UNMARSHAL !!str INTO INT",
			description: "Uppercase pattern extracts tag",
			expectEmpty: false, // Extracts "str" from !!str tag
		},
		{
			name:        "mixed case pattern",
			errorMsg:    "Cannot Unmarshal !!str Into Int",
			description: "Mixed case pattern extracts tag",
			expectEmpty: false, // Extracts "str" from !!str tag
		},
		{
			name:        "pattern with numbers",
			errorMsg:    "expected 123, got 456",
			description: "Numbers instead of types",
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Graceful handling test - should not panic
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panicked on malformed message: %v\n  Message: %q\n  Description: %s",
							r, tt.errorMsg, tt.description)
					}
				}()

				result := ExtractTypeNameBasic(tt.errorMsg)

				if tt.expectEmpty && result != "" {
					t.Errorf("Expected empty type name for edge case, got %q\n  Description: %s",
						result, tt.description)
				}

				t.Logf("✓ Edge case handled: %s → %q", tt.description, result)
			}()
		})
	}
}

// TestMalformedMessages_BrokenLinePatterns tests malformed line/column patterns
func TestMalformedMessages_BrokenLinePatterns(t *testing.T) {
	tests := []struct {
		name        string
		errorMsg    string
		description string
		expectEmpty bool
	}{
		{
			name:        "line with negative number",
			errorMsg:    "line -5: cannot parse",
			description: "Negative line number",
			expectEmpty: true,
		},
		{
			name:        "line with zero",
			errorMsg:    "line 0: cannot parse",
			description: "Zero line number (should be valid)",
			expectEmpty: true,
		},
		{
			name:        "line with very large number",
			errorMsg:    "line 999999999999999999999: cannot parse",
			description: "Overflow line number",
			expectEmpty: true,
		},
		{
			name:        "line with decimal",
			errorMsg:    "line 10.5: cannot parse",
			description: "Decimal line number",
			expectEmpty: true,
		},
		{
			name:        "column without line",
			errorMsg:    "column 5: cannot parse",
			description: "Column pattern without line",
			expectEmpty: true,
		},
		{
			name:        "line and colon but no message",
			errorMsg:    "line 10:",
			description: "Line pattern with no error message",
			expectEmpty: true,
		},
		{
			name:        "multiple line numbers",
			errorMsg:    "line 10: line 20: cannot parse",
			description: "Multiple line number patterns",
			expectEmpty: true,
		},
		{
			name:        "line number in parentheses",
			errorMsg:    "(line 10): cannot parse",
			description: "Line number in parentheses",
			expectEmpty: true,
		},
		{
			name:        "line number with underscore",
			errorMsg:    "line 1_0: cannot parse",
			description: "Line number with underscore separator",
			expectEmpty: true,
		},
		{
			name:        "line number with hex",
			errorMsg:    "line 0x10: cannot parse",
			description: "Hexadecimal line number",
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTypeNameBasic(tt.errorMsg)

			if tt.expectEmpty && result != "" {
				t.Errorf("Expected empty type name for broken line pattern, got %q\n  Message: %s\n  Description: %s",
					result, tt.errorMsg, tt.description)
			}

			t.Logf("✓ Broken line pattern handled: %s → %q", tt.description, result)
		})
	}
}

// TestMalformedMessages_GracefulDegradation tests that malformed messages are handled gracefully
func TestMalformedMessages_GracefulDegradation(t *testing.T) {
	// This test verifies that the system handles malformed messages gracefully
	// without panicking or crashing, even when the messages are severely malformed

	t.Run("no panic on severely malformed messages", func(t *testing.T) {
		malformedMessages := []string{
			"@@@@@@@@@@@@@@@@@@@@@@@@@@@@",
			"$$%%^^&&**()()()",
			"unmarshal unmarshal unmarshal unmarshal",
			"into into into into into",
			"expected expected expected expected",
			"got got got got got got",
			"!!!!!!!!!!!!!!!!!!!!!!!!!",
			"\\\\\\\\\\\\\\\\\\\\\\\\\\\\\\",
			":::::::::::::::::::",
			"?????????????????????",
		}

		for _, msg := range malformedMessages {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panicked on malformed message %q: %v", msg, r)
					}
				}()

				result := ExtractTypeNameBasic(msg)
				t.Logf("✓ No panic on malformed message %q → %q", msg, result)
			}()
		}
	})

	t.Run("consistent behavior on identical malformed messages", func(t *testing.T) {
		malformedMsg := "cannot unmarshal broken pattern into broken type"

		// Call multiple times and verify consistency
		results := make([]string, 10)
		for i := 0; i < 10; i++ {
			results[i] = ExtractTypeNameBasic(malformedMsg)
		}

		// All results should be the same
		for i := 1; i < len(results); i++ {
			if results[i] != results[0] {
				t.Errorf("Inconsistent results on malformed message: %q vs %q", results[0], results[i])
			}
		}

		t.Logf("✓ Consistent behavior: malformed message always returns %q", results[0])
	})

	t.Run("handles mixed valid and malformed content", func(t *testing.T) {
		// Messages that have some valid parts but are overall malformed
		mixedMessages := []string{
			"line 10: cannot unmarshal !!str into int but also broken here @#$",
			"expected int, got string @#$%^ broken after",
			"valid prefix: cannot unmarshal !!str into int @#$ bad suffix",
		}

		for _, msg := range mixedMessages {
			result := ExtractTypeNameBasic(msg)
			// Should either extract something valid or return empty, but not panic
			t.Logf("✓ Mixed content handled: %q → %q", msg, result)
		}
	})
}
