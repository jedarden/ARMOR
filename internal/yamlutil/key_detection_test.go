// Package yamlutil tests the basic key detection functions.
package yamlutil

import (
	"testing"
)

// TestIsMappingKey verifies the IsMappingKey function.
func TestIsMappingKey(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "simple key-value pair",
			line:     "name: John",
			expected: true,
		},
		{
			name:     "key with spaces",
			line:     "first name: John",
			expected: true,
		},
		{
			name:     "line without colon",
			line:     "just some text",
			expected: false,
		},
		{
			name:     "empty string",
			line:     "",
			expected: false,
		},
		{
			name:     "colon at start",
			line:     ": value",
			expected: true,
		},
		{
			name:     "colon only",
			line:     ":",
			expected: true,
		},
		{
			name:     "multiple colons",
			line:     "key: value: with: colons",
			expected: true,
		},
		{
			name:     "colon in middle of word",
			line:     "key:value",
			expected: true,
		},
		{
			name:     "indented key with colon",
			line:     "  name: John",
			expected: true,
		},
		{
			name:     "tab indented key with colon",
			line:     "\tname: John",
			expected: true,
		},
		{
			name:     "quoted key with colon",
			line:     `"name": John`,
			expected: true,
		},
		{
			name:     "single quoted key with colon",
			line:     `'name': John`,
			expected: true,
		},
		{
			name:     "comment line",
			line:     "# This is a comment",
			expected: false,
		},
		{
			name:     "comment with colon",
			line:     "# TODO: fix this",
			expected: false, // Comment lines should be filtered out
		},
		{
			name:     "sequence item",
			line:     "- item",
			expected: false,
		},
		{
			name:     "sequence item with colon",
			line:     "- name: value",
			expected: true,
		},
		{
			name:     "whitespace only",
			line:     "   ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMappingKey(tt.line)
			if result != tt.expected {
				t.Errorf("IsMappingKey(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

// TestExtractKey verifies the ExtractKey function.
func TestExtractKey(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "simple key-value pair",
			line:     "name: John",
			expected: "name",
		},
		{
			name:     "key with spaces",
			line:     "first name: John",
			expected: "first name",
		},
		{
			name:     "line without colon",
			line:     "just some text",
			expected: "",
		},
		{
			name:     "empty string",
			line:     "",
			expected: "",
		},
		{
			name:     "colon at start",
			line:     ": value",
			expected: "",
		},
		{
			name:     "colon only",
			line:     ":",
			expected: "",
		},
		{
			name:     "key with trailing spaces",
			line:     "  key  : value",
			expected: "  key  ",
		},
		{
			name:     "key with leading spaces",
			line:     "  key: value",
			expected: "  key",
		},
		{
			name:     "indented key with colon",
			line:     "    name: John",
			expected: "    name",
		},
		{
			name:     "tab indented key with colon",
			line:     "\tname: John",
			expected: "\tname",
		},
		{
			name:     "quoted key",
			line:     `"quoted key": value`,
			expected: `"quoted key"`,
		},
		{
			name:     "single quoted key",
			line:     `'quoted key': value`,
			expected: `'quoted key'`,
		},
		{
			name:     "multiple colons returns first key",
			line:     "key1: key2: value",
			expected: "key1",
		},
		{
			name:     "key with hyphens",
			line:     "my-key-name: value",
			expected: "my-key-name",
		},
		{
			name:     "key with underscores",
			line:     "my_key_name: value",
			expected: "my_key_name",
		},
		{
			name:     "key with dots",
			line:     "my.key.name: value",
			expected: "my.key.name",
		},
		{
			name:     "key with numbers",
			line:     "key123: value",
			expected: "key123",
		},
		{
			name:     "numeric key",
			line:     "123: value",
			expected: "123",
		},
		{
			name:     "sequence item with colon",
			line:     "- name: value",
			expected: "- name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractKey(tt.line)
			if result != tt.expected {
				t.Errorf("ExtractKey(%q) = %q, want %q", tt.line, result, tt.expected)
			}
		})
	}
}

// TestKeyDetectionIntegration tests IsMappingKey and ExtractKey together.
func TestKeyDetectionIntegration(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		isKey          bool
		extractedKey   string
	}{
		{
			name:         "valid mapping key",
			line:         "name: John",
			isKey:        true,
			extractedKey: "name",
		},
		{
			name:         "key with spaces",
			line:         "first name: John",
			isKey:        true,
			extractedKey: "first name",
		},
		{
			name:         "not a key - no colon",
			line:         "just some text",
			isKey:        false,
			extractedKey: "",
		},
		{
			name:         "empty string",
			line:         "",
			isKey:        false,
			extractedKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IsMappingKey
			isKey := IsMappingKey(tt.line)
			if isKey != tt.isKey {
				t.Errorf("IsMappingKey(%q) = %v, want %v", tt.line, isKey, tt.isKey)
			}

			// Test ExtractKey
			extractedKey := ExtractKey(tt.line)
			if extractedKey != tt.extractedKey {
				t.Errorf("ExtractKey(%q) = %q, want %q", tt.line, extractedKey, tt.extractedKey)
			}

			// Verify consistency: if IsMappingKey returns true, ExtractKey should return non-empty
			if isKey && extractedKey == "" {
				t.Errorf("Inconsistency: IsMappingKey(%q) = true but ExtractKey returned empty string", tt.line)
			}
		})
	}
}

// TestStripInlineComment verifies the StripInlineComment function.
func TestStripInlineComment(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "inline comment with space before hash",
			line:     "key: value # this is a comment",
			expected: "key: value ",
		},
		{
			name:     "inline comment with tab before hash",
			line:     "key: value\t# this is a comment",
			expected: "key: value\t",
		},
		{
			name:     "inline comment with multiple spaces",
			line:     "key: value    # comment",
			expected: "key: value    ",
		},
		{
			name:     "hash in URL - not a comment",
			line:     "url: http://example.com#anchor",
			expected: "url: http://example.com#anchor",
		},
		{
			name:     "hash in value without preceding space",
			line:     "key: value#not-a-comment",
			expected: "key: value#not-a-comment",
		},
		{
			name:     "hash in text preceded by space",
			line:     "text: some # text",
			expected: "text: some ",
		},
		{
			name:     "no comment - simple key value",
			line:     "key: value",
			expected: "key: value",
		},
		{
			name:     "full-line comment - should not be stripped",
			line:     "# This is a comment",
			expected: "# This is a comment",
		},
		{
			name:     "indented full-line comment - should not be stripped",
			line:     "  # This is a comment",
			expected: "  # This is a comment",
		},
		{
			name:     "empty string",
			line:     "",
			expected: "",
		},
		{
			name:     "whitespace only",
			line:     "   ",
			expected: "   ",
		},
		{
			name:     "key with trailing space before inline comment",
			line:     "key: value # comment",
			expected: "key: value ",
		},
		{
			name:     "inline comment with colon in comment text",
			line:     "key: value # TODO: fix this",
			expected: "key: value ",
		},
		{
			name:     "multiple potential comment markers",
			line:     "key: value # first comment # second comment",
			expected: "key: value ",
		},
		{
			name:     "hash character as part of value syntax",
			line:     "color: #FF0000",
			expected: "color: #FF0000",
		},
		{
			name:     "indented key with inline comment",
			line:     "  key: value # comment",
			expected: "  key: value ",
		},
		{
			name:     "sequence item with inline comment",
			line:     "- item # comment",
			expected: "- item ",
		},
		{
			name:     "complex URL with query string and fragment",
			line:     "url: https://example.com/path?query=value#fragment",
			expected: "url: https://example.com/path?query=value#fragment",
		},
		{
			name:     "key with colon in value and inline comment",
			line:     "time: 10:30 # morning meeting",
			expected: "time: 10:30 ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripInlineComment(tt.line)
			if result != tt.expected {
				t.Errorf("StripInlineComment(%q) = %q, want %q", tt.line, result, tt.expected)
			}
		})
	}
}

// TestCommentFilteringIntegration tests the full comment filtering workflow.
func TestCommentFilteringIntegration(t *testing.T) {
	tests := []struct {
		name              string
		line              string
		isComment         bool
		stripped          string
		detectAsKeyAfterStrip bool
	}{
		{
			name:                  "full comment line",
			line:                  "# This is a comment",
			isComment:             true,
			stripped:              "# This is a comment",
			detectAsKeyAfterStrip: false,
		},
		{
			name:                  "indented comment line",
			line:                  "  # indented comment",
			isComment:             true,
			stripped:              "  # indented comment",
			detectAsKeyAfterStrip: false,
		},
		{
			name:                  "inline comment should be stripped",
			line:                  "key: value # comment",
			isComment:             false,
			stripped:              "key: value ",
			detectAsKeyAfterStrip: true,
		},
		{
			name:                  "hash in URL preserved",
			line:                  "url: http://example.com#anchor",
			isComment:             false,
			stripped:              "url: http://example.com#anchor",
			detectAsKeyAfterStrip: true,
		},
		{
			name:                  "simple key value no comment",
			line:                  "name: John",
			isComment:             false,
			stripped:              "name: John",
			detectAsKeyAfterStrip: true,
		},
		{
			name:                  "comment line with colon is filtered out",
			line:                  "# TODO: fix this issue",
			isComment:             true,
			stripped:              "# TODO: fix this issue",
			detectAsKeyAfterStrip: false, // Comment lines are now properly filtered by IsMappingKey
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IsCommentLine
			isComment := IsCommentLine(tt.line)
			if isComment != tt.isComment {
				t.Errorf("IsCommentLine(%q) = %v, want %v", tt.line, isComment, tt.isComment)
			}

			// Test StripInlineComment
			stripped := StripInlineComment(tt.line)
			if stripped != tt.stripped {
				t.Errorf("StripInlineComment(%q) = %q, want %q", tt.line, stripped, tt.stripped)
			}

			// Test key detection on stripped line
			isKey := IsMappingKey(stripped)
			if isKey != tt.detectAsKeyAfterStrip {
				t.Errorf("After stripping, IsMappingKey(%q) = %v, want %v", stripped, isKey, tt.detectAsKeyAfterStrip)
			}
		})
	}
}

// BenchmarkIsMappingKey benchmarks the IsMappingKey function.
func BenchmarkIsMappingKey(b *testing.B) {
	line := "name: John"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsMappingKey(line)
	}
}

// BenchmarkExtractKey benchmarks the ExtractKey function.
func BenchmarkExtractKey(b *testing.B) {
	line := "name: John"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractKey(line)
	}
}

// TestIsCommentLineComprehensive provides comprehensive testing for IsCommentLine.
func TestIsCommentLineComprehensive(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		// Full-line comments
		{
			name:     "simple comment line",
			line:     "# This is a comment",
			expected: true,
		},
		{
			name:     "comment line with colon",
			line:     "# TODO: fix this",
			expected: true,
		},
		{
			name:     "comment line with multiple colons",
			line:     "# Note: key: value should be: something",
			expected: true,
		},
		{
			name:     "comment line with special characters",
			line:     "# @version: 1.0.0",
			expected: true,
		},
		{
			name:     "comment line with URLs",
			line:     "# https://example.com/path",
			expected: true,
		},
		{
			name:     "comment line with email",
			line:     "# Contact: user@example.com",
			expected: true,
		},
		{
			name:     "comment with hash symbols",
			line:     "# ### heading ###",
			expected: true,
		},
		{
			name:     "comment with hex color reference",
			line:     "# Color should be #FF0000",
			expected: true,
		},
		{
			name:     "comment with URL anchor",
			line:     "# See: http://example.com#section",
			expected: true,
		},

		// Indented comment lines
		{
			name:     "single space indented comment",
			line:     " # comment",
			expected: true,
		},
		{
			name:     "two space indented comment",
			line:     "  # comment",
			expected: true,
		},
		{
			name:     "four space indented comment",
			line:     "    # comment",
			expected: true,
		},
		{
			name:     "eight space indented comment",
			line:     "        # comment",
			expected: true,
		},
		{
			name:     "single tab indented comment",
			line:     "\t# comment",
			expected: true,
		},
		{
			name:     "double tab indented comment",
			line:     "\t\t# comment",
			expected: true,
		},
		{
			name:     "mixed space and tab indented comment",
			line:     "  \t# comment",
			expected: true,
		},
		{
			name:     "tab and space indented comment",
			line:     "\t  # comment",
			expected: true,
		},

		// Non-comment lines (false positives prevention)
		{
			name:     "key value with hash in value",
			line:     "color: #FF0000",
			expected: false,
		},
		{
			name:     "key value with short hex color",
			line:     "color: #FFF",
			expected: false,
		},
		{
			name:     "key value with hex number",
			line:     "value: 0x123ABC",
			expected: false,
		},
		{
			name:     "URL with anchor",
			line:     "url: http://example.com#anchor",
			expected: false,
		},
		{
			name:     "URL with multiple anchors",
			line:     "url: https://example.com/path#anchor1#anchor2",
			expected: false,
		},
		{
			name:     "URL with query and anchor",
			line:     "url: https://example.com/path?query=value#fragment",
			expected: false,
		},
		{
			name:     "key with hash character",
			line:     "key#name: value",
			expected: false,
		},
		{
			name:     "value starting with hash but not comment",
			line:     "symbol: #value",
			expected: false,
		},
		{
			name:     "path with hash",
			line:     "path: /usr/local/bin#backup",
			expected: false,
		},
		{
			name:     "Git ref with hash",
			line:     "ref: abc123def456",
			expected: false,
		},
		{
			name:     "anchor reference in value",
			line:     "reference: &anchor",
			expected: false,
		},
		{
			name:     "alias reference in value",
			line:     "alias: *anchor",
			expected: false,
		},

		// Edge cases
		{
			name:     "empty string",
			line:     "",
			expected: false,
		},
		{
			name:     "whitespace only",
			line:     "   ",
			expected: false,
		},
		{
			name:     "tabs only",
			line:     "\t\t",
			expected: false,
		},
		{
			name:     "mixed whitespace only",
			line:     "  \t  ",
			expected: false,
		},
		{
			name:     "hash only",
			line:     "#",
			expected: true,
		},
		{
			name:     "indented hash only",
			line:     "  #",
			expected: true,
		},
		{
			name:     "hash at end of value with space",
			line:     "text: value #",
			expected: false,
		},
		{
			name:     "hash in middle of value",
			line:     "text: value#in#middle",
			expected: false,
		},
		{
			name:     "multiple hashes in value",
			line:     "tags: #tag1 #tag2 #tag3",
			expected: false,
		},

		// Valid YAML mapping keys
		{
			name:     "simple key value",
			line:     "name: John",
			expected: false,
		},
		{
			name:     "key with spaces",
			line:     "first name: John Doe",
			expected: false,
		},
		{
			name:     "indented key value",
			line:     "  name: John",
			expected: false,
		},
		{
			name:     "deeply indented key value",
			line:     "      name: John",
			expected: false,
		},
		{
			name:     "tab indented key value",
			line:     "\tname: John",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCommentLine(tt.line)
			if result != tt.expected {
				t.Errorf("IsCommentLine(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

// TestStripInlineCommentComprehensive provides comprehensive testing for StripInlineComment.
func TestStripInlineCommentComprehensive(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		// Basic inline comments
		{
			name:     "inline comment after value",
			line:     "key: value # comment",
			expected: "key: value ",
		},
		{
			name:     "inline comment with multiple spaces before hash",
			line:     "key: value     # comment",
			expected: "key: value     ",
		},
		{
			name:     "inline comment with tab before hash",
			line:     "key: value\t# comment",
			expected: "key: value\t",
		},
		{
			name:     "inline comment with multiple tabs",
			line:     "key: value\t\t# comment",
			expected: "key: value\t\t",
		},
		{
			name:     "inline comment with mixed whitespace",
			line:     "key: value \t # comment",
			expected: "key: value \t ",
		},

		// Comments with special content
		{
			name:     "comment with colon",
			line:     "key: value # TODO: fix this",
			expected: "key: value ",
		},
		{
			name:     "comment with URL",
			line:     "key: value # see https://example.com",
			expected: "key: value ",
		},
		{
			name:     "comment with multiple colons",
			line:     "key: value # note: key: value",
			expected: "key: value ",
		},
		{
			name:     "comment with hash symbols",
			line:     "key: value # ### heading ###",
			expected: "key: value ",
		},

		// False positives - hash in values (should NOT be stripped)
		{
			name:     "hex color value",
			line:     "color: #FF0000",
			expected: "color: #FF0000",
		},
		{
			name:     "short hex color",
			line:     "color: #FFF",
			expected: "color: #FFF",
		},
		{
			name:     "hex color in quotes",
			line:     "color: \"#FF0000\"",
			expected: "color: \"#FF0000\"",
		},
		{
			name:     "hex color with alpha",
			line:     "color: #FF0000FF",
			expected: "color: #FF0000FF",
		},
		{
			name:     "URL with anchor",
			line:     "url: http://example.com#anchor",
			expected: "url: http://example.com#anchor",
		},
		{
			name:     "URL with query and fragment",
			line:     "url: https://example.com/path?query=value#fragment",
			expected: "url: https://example.com/path?query=value#fragment",
		},
		{
			name:     "URL with multiple anchors",
			line:     "url: http://example.com#anchor1#anchor2",
			expected: "url: http://example.com#anchor1#anchor2",
		},
		{
			name:     "CSS ID selector",
			line:     "selector: #main-content",
			expected: "selector: #main-content",
		},
		{
			name:     "CSS ID with dashes",
			line:     "selector: #my-custom-id",
			expected: "selector: #my-custom-id",
		},
		{
			name:     "CSS ID with underscores",
			line:     "selector: #my_custom_id",
			expected: "selector: #my_custom_id",
		},
		{
			name:     "HTML ID attribute",
			line:     "id: main-container",
			expected: "id: main-container",
		},
		{
			name:     "symbolic value starting with hash",
			line:     "symbol: #define",
			expected: "symbol: #define",
		},
		{
			name:     "preprocessor directive style",
			line:     "directive: #include",
			expected: "directive: #include",
		},
		{
			name:     "hashtag value",
			line:     "tag: #golang",
			expected: "tag: #golang",
		},
		{
			name:     "multiple hashtag values",
			line:     "tags: #golang #yaml #testing",
			expected: "tags: #golang #yaml #testing",
		},
		{
			name:     "hash in path",
			line:     "path: /usr/local/bin#backup",
			expected: "path: /usr/local/bin#backup",
		},
		{
			name:     "hash as value without space",
			line:     "value: #notacomment",
			expected: "value: #notacomment",
		},
		{
			name:     "hash in middle of word",
			line:     "value: some#thing",
			expected: "value: some#thing",
		},
		{
			name:     "hex-like value (3 chars)",
			line:     "code: #ABC",
			expected: "code: #ABC",
		},
		{
			name:     "hex-like value (6 chars)",
			line:     "code: #ABC123",
			expected: "code: #ABC123",
		},
		{
			name:     "hex-like value (8 chars)",
			line:     "code: #ABC12345",
			expected: "code: #ABC12345",
		},
		{
			name:     "value with hash and number",
			line:     "version: #1",
			expected: "version: #1",
		},
		{
			name:     "complex value with hash",
			line:     "pattern: key#value#pattern",
			expected: "pattern: key#value#pattern",
		},

		// Edge cases
		{
			name:     "hash at end of line with space",
			line:     "value: text #",
			expected: "value: text ",
		},
		{
			name:     "hash at end of line without space",
			line:     "value: text#",
			expected: "value: text#",
		},
		{
			name:     "only hash with space after",
			line:     "# ",
			expected: "",
		},
		{
			name:     "only hash without space after",
			line:     "#",
			expected: "",
		},
		{
			name:     "empty string",
			line:     "",
			expected: "",
		},
		{
			name:     "whitespace only",
			line:     "   ",
			expected:   "   ",
		},

		// Full-line comments (should NOT be stripped)
		{
			name:     "full-line comment",
			line:     "# This is a comment",
			expected: "# This is a comment",
		},
		{
			name:     "indented full-line comment",
			line:     "  # This is a comment",
			expected: "  # This is a comment",
		},
		{
			name:     "tab indented full-line comment",
			line:     "\t# This is a comment",
			expected: "\t# This is a comment",
		},
		{
			name:     "deeply indented full-line comment",
			line:     "    # deeply indented comment",
			expected: "    # deeply indented comment",
		},

		// Values without comments
		{
			name:     "simple key value",
			line:     "key: value",
			expected: "key: value",
		},
		{
			name:     "key with colon in value",
			line:     "time: 10:30",
			expected: "time: 10:30",
		},
		{
			name:     "key with multiple colons in value",
			line:     "time: 10:30:45",
			expected: "time: 10:30:45",
		},
		{
			name:     "value with spaces",
			line:     "key: value with spaces",
			expected: "key: value with spaces",
		},
		{
			name:     "quoted value",
			line:     "key: \"quoted value\"",
			expected: "key: \"quoted value\"",
		},
		{
			name:     "sequence item",
			line:     "- item value",
			expected: "- item value",
		},
		{
			name:     "sequence item with inline comment",
			line:     "- item # comment",
			expected: "- item ",
		},

		// Complex scenarios
		{
			name:     "key value with trailing spaces and comment",
			line:     "key: value    # comment",
			expected: "key: value    ",
		},
		{
			name:     "key with inline comment and quoted value",
			line:     "path: \"/usr/bin\" # system path",
			expected: "path: \"/usr/bin\" ",
		},
		{
			name:     "key with number and comment",
			line:     "count: 42 # item count",
			expected: "count: 42 ",
		},
		{
			name:     "key with boolean and comment",
			line:     "enabled: true # feature flag",
			expected: "enabled: true ",
		},
		{
			name:     "key with null and comment",
			line:     "value: null # empty value",
			expected: "value: null ",
		},

		// Unicode and special characters
		{
			name:     "comment with unicode",
			line:     "key: value # コメント",
			expected: "key: value ",
		},
		{
			name:     "comment with emoji",
			line:     "key: value # ✓ verified",
			expected: "key: value ",
		},
		{
			name:     "value with unicode and comment",
			line:     "name: 日本語 # Japanese text",
			expected: "name: 日本語 ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripInlineComment(tt.line)
			if result != tt.expected {
				t.Errorf("StripInlineComment(%q) = %q, want %q", tt.line, result, tt.expected)
			}
		})
	}
}

// TestCommentFilteringWithIndentation tests comment filtering at various indentation levels.
func TestCommentFilteringWithIndentation(t *testing.T) {
	tests := []struct {
		name              string
		line              string
		isComment         bool
		stripped          string
		detectAsKeyAfterStrip bool
	}{
		// Level 0 (no indentation)
		{
			name:                  "level 0 - comment",
			line:                  "# comment at root",
			isComment:             true,
			stripped:              "# comment at root",
			detectAsKeyAfterStrip: false,
		},
		{
			name:                  "level 0 - key value with comment",
			line:                  "key: value # comment",
			isComment:             false,
			stripped:              "key: value ",
			detectAsKeyAfterStrip: true,
		},
		{
			name:                  "level 0 - URL with anchor",
			line:                  "url: http://example.com#section",
			isComment:             false,
			stripped:              "url: http://example.com#section",
			detectAsKeyAfterStrip: true,
		},

		// Level 1 (2 spaces)
		{
			name:                  "level 1 - comment",
			line:                  "  # indented comment",
			isComment:             true,
			stripped:              "  # indented comment",
			detectAsKeyAfterStrip: false,
		},
		{
			name:                  "level 1 - key value with comment",
			line:                  "  key: value # comment",
			isComment:             false,
			stripped:              "  key: value ",
			detectAsKeyAfterStrip: true,
		},
		{
			name:                  "level 1 - hex color",
			line:                  "  color: #FF0000",
			isComment:             false,
			stripped:              "  color: #FF0000",
			detectAsKeyAfterStrip: true,
		},

		// Level 2 (4 spaces)
		{
			name:                  "level 2 - comment",
			line:                  "    # nested comment",
			isComment:             true,
			stripped:              "    # nested comment",
			detectAsKeyAfterStrip: false,
		},
		{
			name:                  "level 2 - key value with comment",
			line:                  "    key: value # comment",
			isComment:             false,
			stripped:              "    key: value ",
			detectAsKeyAfterStrip: true,
		},
		{
			name:                  "level 2 - URL with anchor",
			line:                  "    url: https://example.com#anchor",
			isComment:             false,
			stripped:              "    url: https://example.com#anchor",
			detectAsKeyAfterStrip: true,
		},

		// Level 3 (6 spaces)
		{
			name:                  "level 3 - comment",
			line:                  "      # deep comment",
			isComment:             true,
			stripped:              "      # deep comment",
			detectAsKeyAfterStrip: false,
		},
		{
			name:                  "level 3 - key value with comment",
			line:                  "      key: value # comment",
			isComment:             false,
			stripped:              "      key: value ",
			detectAsKeyAfterStrip: true,
		},

		// Level 4 (8 spaces)
		{
			name:                  "level 4 - comment",
			line:                  "        # very deep comment",
			isComment:             true,
			stripped:              "        # very deep comment",
			detectAsKeyAfterStrip: false,
		},
		{
			name:                  "level 4 - key value with comment",
			line:                  "        key: value # comment",
			isComment:             false,
			stripped:              "        key: value ",
			detectAsKeyAfterStrip: true,
		},

		// Tab-based indentation
		{
			name:                  "1 tab - comment",
			line:                  "\t# tab comment",
			isComment:             true,
			stripped:              "\t# tab comment",
			detectAsKeyAfterStrip: false,
		},
		{
			name:                  "1 tab - key value with comment",
			line:                  "\tkey: value # comment",
			isComment:             false,
			stripped:              "\tkey: value ",
			detectAsKeyAfterStrip: true,
		},
		{
			name:                  "2 tabs - comment",
			line:                  "\t\t# nested tab comment",
			isComment:             true,
			stripped:              "\t\t# nested tab comment",
			detectAsKeyAfterStrip: false,
		},
		{
			name:                  "2 tabs - key value with comment",
			line:                  "\t\tkey: value # comment",
			isComment:             false,
			stripped:              "\t\tkey: value ",
			detectAsKeyAfterStrip: true,
		},

		// Mixed indentation (edge case)
		{
			name:                  "mixed indent - comment",
			line:                  "  \t# mixed indent comment",
			isComment:             true,
			stripped:              "  \t# mixed indent comment",
			detectAsKeyAfterStrip: false,
		},
		{
			name:                  "mixed indent - key value with comment",
			line:                  "  \tkey: value # comment",
			isComment:             false,
			stripped:              "  \tkey: value ",
			detectAsKeyAfterStrip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IsCommentLine
			isComment := IsCommentLine(tt.line)
			if isComment != tt.isComment {
				t.Errorf("IsCommentLine(%q) = %v, want %v", tt.line, isComment, tt.isComment)
			}

			// Test StripInlineComment
			stripped := StripInlineComment(tt.line)
			if stripped != tt.stripped {
				t.Errorf("StripInlineComment(%q) = %q, want %q", tt.line, stripped, tt.stripped)
			}

			// Test key detection on stripped line
			isKey := IsMappingKey(stripped)
			if isKey != tt.detectAsKeyAfterStrip {
				t.Errorf("After stripping, IsMappingKey(%q) = %v, want %v", stripped, isKey, tt.detectAsKeyAfterStrip)
			}
		})
	}
}

// TestCommentFilteringInYamlDocuments tests comment filtering in realistic YAML document scenarios.
func TestCommentFilteringInYamlDocuments(t *testing.T) {
	// Test complete YAML documents with comments
	yamlDocuments := []struct {
		name  string
		lines []string
	}{
		{
			name: "simple config with comments",
			lines: []string{
				"# Configuration file",
				"version: 1.0",
				"",
				"# Server settings",
				"server:",
				"  host: localhost # default host",
				"  port: 8080 # default port",
				"",
				"# Database settings",
				"database:",
				"  driver: postgres",
				"  # Connection string with URL",
				"  url: postgres://localhost/db#schema",
			},
		},
		{
			name: "colors configuration",
			lines: []string{
				"# Theme colors",
				"theme:",
				"  primary: #FF0000",
				"  secondary: #00FF00",
				"  tertiary: #0000FF # nice blue",
				"",
				"# UI Colors",
				"ui:",
				"  background: #FFFFFF",
				"  foreground: #000000 # text color",
				"  # Accent color - should be vibrant",
				"  accent: #FF00FF",
			},
		},
		{
			name: "URL configuration",
			lines: []string{
				"# API endpoints",
				"api:",
				"  base: https://api.example.com",
				"  # User endpoint",
				"  user: https://api.example.com/v1/users#list",
				"  # Auth endpoint",
				"  auth: https://api.example.com/v1/auth#login",
				"",
				"# Documentation URLs",
				"docs:",
				"  main: https://docs.example.com",
				"  # API reference section",
				"  api: https://docs.example.com/api#reference",
			},
		},
		{
			name: "nested configuration with comments",
			lines: []string{
				"# Main application config",
				"app:",
				"  name: MyApp",
				"  version: 1.0.0",
				"  ",
				"  # Logging configuration",
				"  logging:",
				"    level: debug # for development",
				"    format: json",
				"    ",
				"    # Log file location",
				"    file: /var/log/app.log",
				"",
				"  # Feature flags",
				"  features:",
				"    new_ui: true # enabled for testing",
				"    beta_api: false # not ready yet",
			},
		},
		{
			name: "complex document with all comment types",
			lines: []string{
				"---",
				"# Document 1: Main config",
				"config:",
				"  # This is an indented comment",
				"  setting1: value1",
				"  setting2: value2 # inline comment",
				"  ",
				"  # Comment with colon: key: value",
				"  setting3: value3",
				"  ",
				"  # Colors in hex",
				"  colors:",
				"    red: #FF0000",
				"    green: #00FF00 # pure green",
				"    blue: #0000FF",
				"...",
				"---",
				"# Document 2: Additional config",
				"secondary:",
				"  # URLs with anchors",
				"  endpoints:",
				"    main: http://example.com#main",
				"    backup: http://example.com#backup",
			},
		},
	}

	for _, doc := range yamlDocuments {
		t.Run(doc.name, func(t *testing.T) {
			// Process each line and verify comment filtering
			for i, line := range doc.lines {
				// Test IsCommentLine
				isComment := IsCommentLine(line)

				// Test StripInlineComment
				stripped := StripInlineComment(line)

				// If it's a comment line, stripping should not change it
				if isComment {
					if stripped != line {
						t.Errorf("Line %d: Comment line was modified by StripInlineComment: %q -> %q", i, line, stripped)
					}
					// Comment lines should not be detected as keys
					if IsMappingKey(stripped) {
						t.Errorf("Line %d: Comment line detected as mapping key: %q", i, line)
					}
				}

				// Verify full-line comments are preserved
				if len(line) > 0 && line[0] == '#' {
					if stripped != line {
						t.Errorf("Line %d: Full-line comment was modified: %q -> %q", i, line, stripped)
					}
				}
			}
		})
	}
}

// TestCommentFilteringFalsePositives tests that false positives are prevented.
func TestCommentFilteringFalsePositives(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected struct {
			isComment bool
			stripped  string
			isKey     bool
		}
	}{
		// Hash in URLs (should NOT be treated as comments)
		{
			name: "URL with fragment",
			line: "url: http://example.com#section",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "url: http://example.com#section", true},
		},
		{
			name: "HTTPS URL with fragment",
			line: "url: https://example.com/path/to/resource#anchor",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "url: https://example.com/path/to/resource#anchor", true},
		},
		{
			name: "URL with query and fragment",
			line: "url: https://example.com/api?key=value#section",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "url: https://example.com/api?key=value#section", true},
		},

		// Hex colors (should NOT be treated as comments)
		{
			name: "3-digit hex color",
			line: "color: #FFF",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "color: #FFF", true},
		},
		{
			name: "6-digit hex color",
			line: "color: #FF0000",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "color: #FF0000", true},
		},
		{
			name: "8-digit hex color with alpha",
			line: "color: #FF0000FF",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "color: #FF0000FF", true},
		},
		{
			name: "hex color in quotes",
			line: "color: \"#FF0000\"",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "color: \"#FF0000\"", true},
		},

		// CSS ID selectors (should NOT be treated as comments)
		{
			name: "CSS ID selector",
			line: "selector: #main-content",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "selector: #main-content", true},
		},
		{
			name: "CSS ID with numbers",
			line: "selector: #col1-row2",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "selector: #col1-row2", true},
		},

		// Hashtag-style values (should NOT be treated as comments)
		{
			name: "single hashtag",
			line: "tag: #golang",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "tag: #golang", true},
		},
		{
			name: "multiple hashtags",
			line: "tags: #golang #yaml #testing",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "tags: #golang #yaml #testing", true},
		},

		// Hash in file paths (should NOT be treated as comments)
		{
			name: "hash in path",
			line: "path: /usr/local/bin#backup",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "path: /usr/local/bin#backup", true},
		},
		{
			name: "Windows path with hash",
			line: "path: C:\\Users\\Admin#1\\file.txt",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "path: " + "C:\\Users\\Admin#1\\file.txt", true},
		},

		// Hash in value without space (should NOT be treated as comment)
		{
			name: "hash directly after value",
			line: "symbol: #define",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "symbol: #define", true},
		},
		{
			name: "hash in middle of value",
			line: "value: key#value#pattern",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "value: key#value#pattern", true},
		},

		// Actual comments (SHOULD be treated as comments)
		{
			name: "inline comment with space",
			line: "key: value # this is a comment",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{false, "key: value ", true},
		},
		{
			name: "full-line comment",
			line: "# This is a full-line comment",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{true, "# This is a full-line comment", false},
		},
		{
			name: "indented full-line comment",
			line: "  # This is an indented comment",
			expected: struct {
				isComment bool
				stripped  string
				isKey     bool
			}{true, "  # This is an indented comment", false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IsCommentLine
			isComment := IsCommentLine(tt.line)
			if isComment != tt.expected.isComment {
				t.Errorf("IsCommentLine(%q) = %v, want %v", tt.line, isComment, tt.expected.isComment)
			}

			// Test StripInlineComment
			stripped := StripInlineComment(tt.line)
			if stripped != tt.expected.stripped {
				t.Errorf("StripInlineComment(%q) = %q, want %q", tt.line, stripped, tt.expected.stripped)
			}

			// Test IsMappingKey on stripped line
			isKey := IsMappingKey(stripped)
			if isKey != tt.expected.isKey {
				t.Errorf("IsMappingKey(%q) = %v, want %v", stripped, isKey, tt.expected.isKey)
			}
		})
	}
}

// TestExtractKeyWithCommentFiltering tests that ExtractKey properly handles comments.
func TestExtractKeyWithCommentFiltering(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "key value with inline comment",
			line:     "name: John # comment",
			expected: "name",
		},
		{
			name:     "key with URL anchor and inline comment",
			line:     "url: http://example.com#anchor # comment about URL",
			expected: "url",
		},
		{
			name:     "key with hex color and comment",
			line:     "color: #FF0000 # red color",
			expected: "color",
		},
		{
			name:     "key with complex value and comment",
			line:     "time: 10:30:45 # morning meeting time",
			expected: "time",
		},
		{
			name:     "indented key with comment",
			line:     "  name: John # comment",
			expected: "  name",
		},
		{
			name:     "deeply indented key with comment",
			line:     "    name: John # comment",
			expected: "    name",
		},
		{
			name:     "tab indented key with comment",
			line:     "\tname: John # comment",
			expected: "\tname",
		},
		{
			name:     "key with inline comment containing colon",
			line:     "key: value # TODO: fix this later",
			expected: "key",
		},
		{
			name:     "key with multiple inline comments",
			line:     "key: value # first comment # second comment",
			expected: "key",
		},
		{
			name:     "key with inline comment and URL",
			line:     "docs: https://example.com # see documentation",
			expected: "docs",
		},
		{
			name:     "key value with hash in value and comment",
			line:     "tag: #hashtag # this is a tag",
			expected: "tag",
		},
		{
			name:     "key with trailing space before comment",
			line:     "key: value   # comment",
			expected: "key",
		},
		{
			name:     "key with tab before comment",
			line:     "key: value\t# comment",
			expected: "key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractKey(tt.line)
			if result != tt.expected {
				t.Errorf("ExtractKey(%q) = %q, want %q", tt.line, result, tt.expected)
			}
		})
	}
}

// BenchmarkCommentFiltering benchmarks comment filtering operations.
func BenchmarkCommentFiltering(b *testing.B) {
	tests := []struct {
		name string
		line string
	}{
		{
			name: "simple key value",
			line: "key: value",
		},
		{
			name: "key value with inline comment",
			line: "key: value # comment",
		},
		{
			name: "full-line comment",
			line: "# This is a comment",
		},
		{
			name: "URL with anchor",
			line: "url: http://example.com#anchor",
		},
		{
			name: "hex color",
			line: "color: #FF0000",
		},
		{
			name: "indented key with comment",
			line: "  key: value # comment",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				IsCommentLine(tt.line)
				StripInlineComment(tt.line)
				IsMappingKey(tt.line)
			}
		})
	}
}
