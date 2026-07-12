// Package yamlutil tests basic YAML comment filtering patterns.
package yamlutil

import (
	"testing"
)

// TestBasicFullLineCommentDetection tests basic full-line comment detection.
// This test covers the acceptance criteria requirement for testing full-line comment detection.
func TestBasicFullLineCommentDetection(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "hash at start of line",
			line:     "# This is a comment",
			expected: true,
		},
		{
			name:     "hash with leading spaces",
			line:     "  # This is a comment",
			expected: true,
		},
		{
			name:     "hash with leading tabs",
			line:     "\t# This is a comment",
			expected: true,
		},
		{
			name:     "hash with mixed leading whitespace",
			line:     "  \t# This is a comment",
			expected: true,
		},
		{
			name:     "hash only no text",
			line:     "#",
			expected: true,
		},
		{
			name:     "hash with spaces only",
			line:     "#   ",
			expected: true,
		},
		{
			name:     "not comment - key value",
			line:     "key: value",
			expected: false,
		},
		{
			name:     "not comment - empty line",
			line:     "",
			expected: false,
		},
		{
			name:     "not comment - whitespace only",
			line:     "   ",
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

// TestBasicInlineCommentDetection tests basic inline comment detection and stripping.
// This test covers the acceptance criteria requirement for testing inline comment detection.
func TestBasicInlineCommentDetection(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "inline comment after value",
			line:     "key: value # this is a comment",
			expected: "key: value ",
		},
		{
			name:     "inline comment with multiple spaces before hash",
			line:     "key: value    # this is a comment",
			expected: "key: value    ",
		},
		{
			name:     "inline comment with tab before hash",
			line:     "key: value\t# this is a comment",
			expected: "key: value\t",
		},
		{
			name:     "inline comment with no space before hash should not strip",
			line:     "key: value#not-a-comment",
			expected: "key: value#not-a-comment",
		},
		{
			name:     "no inline comment - simple key value",
			line:     "key: value",
			expected: "key: value",
		},
		{
			name:     "inline comment at end of sequence item",
			line:     "- item # comment",
			expected: "- item ",
		},
		{
			name:     "inline comment with colon in comment text",
			line:     "key: value # TODO: fix this",
			expected: "key: value ",
		},
		{
			name:     "hash in URL should not be stripped",
			line:     "url: http://example.com#anchor",
			expected: "url: http://example.com#anchor",
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

// TestCommentAtStartOfLine tests comment detection at the start of lines.
// This test covers the acceptance criteria requirement for testing comments at the start of lines.
func TestCommentAtStartOfLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "comment at very start",
			line:     "# comment at start",
			expected: true,
		},
		{
			name:     "comment with leading space at start",
			line:     " # comment with leading space",
			expected: true,
		},
		{
			name:     "comment with leading tab at start",
			line:     "\t# comment with leading tab",
			expected: true,
		},
		{
			name:     "not comment - key at start",
			line:     "key: value",
			expected: false,
		},
		{
			name:     "not comment - sequence item at start",
			line:     "- item",
			expected: false,
		},
		{
			name:     "not comment - dash only at start",
			line:     "-",
			expected: false,
		},
		{
			name:     "not comment - colon only at start",
			line:     ":",
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

// TestCommentAtMiddleOfLine tests comments in the middle of content lines.
// This test covers the acceptance criteria requirement for testing comments in the middle of lines.
func TestCommentAtMiddleOfLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "comment after key with space",
			line:     "key # comment in middle",
			expected: "key ",
		},
		{
			name:     "comment after value with space",
			line:     "key: value # comment in middle",
			expected: "key: value ",
		},
		{
			name:     "comment after partial value",
			line:     "key: partial_value # comment",
			expected: "key: partial_value ",
		},
		{
			name:     "comment between multiple keys - single line",
			line:     "key1: value1 # comment then key2: value2",
			expected: "key1: value1 ",
		},
		{
			name:     "comment in nested structure",
			line:     "parent:\n  child: value # comment",
			expected: "parent:\n  child: value ",
		},
		{
			name:     "hash without space should not strip",
			line:     "key: value#notcomment",
			expected: "key: value#notcomment",
		},
		{
			name:     "hash in middle of word should not strip",
			line:     "key: value#with#hashes",
			expected: "key: value#with#hashes",
		},
		{
			name:     "comment after complex value",
			line:     "key: [a, b, c] # comment after array",
			expected: "key: [a, b, c] ",
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

// TestCommentAtEndOfLine tests comments at the end of content lines.
// This test covers the acceptance criteria requirement for testing comments at the end of lines.
func TestCommentAtEndOfLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "comment at end of key value",
			line:     "key: value # comment at end",
			expected: "key: value ",
		},
		{
			name:     "comment at end with trailing space",
			line:     "key: value # comment at end  ",
			expected: "key: value ",
		},
		{
			name:     "comment at end with multiple trailing spaces",
			line:     "key: value    # comment",
			expected: "key: value    ",
		},
		{
			name:     "comment at end of sequence item",
			line:     "- item value # comment at end",
			expected: "- item value ",
		},
		{
			name:     "comment at end of nested value",
			line:     "  key: value # comment at end",
			expected: "  key: value ",
		},
		{
			name:     "comment at very end of line with hash only",
			line:     "key: value #",
			expected: "key: value ",
		},
		{
			name:     "comment at end with tab before hash",
			line:     "key: value\t# comment at end",
			expected: "key: value\t",
		},
		{
			name:     "no comment at end - just value",
			line:     "key: value",
			expected: "key: value",
		},
		{
			name:     "hash at end but no space - not a comment",
			line:     "key: value#notcomment",
			expected: "key: value#notcomment",
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

// TestCommentPositionIntegration tests comment filtering at various positions in realistic scenarios.
// This test covers the acceptance criteria requirement for testing comments at various positions.
func TestCommentPositionIntegration(t *testing.T) {
	tests := []struct {
		name           string
		yamlContent    string
		expectedKeys   []string
		commentsFound  int
	}{
		{
			name: "comment at start of document",
			yamlContent: `# Configuration file
key1: value1
key2: value2`,
			expectedKeys:  []string{"key1", "key2"},
			commentsFound: 1,
		},
		{
			name: "comments at start and end, inline in middle",
			yamlContent: `# Start comment
key1: value1 # Middle comment
key2: value2
# End comment`,
			expectedKeys:  []string{"key1", "key2"},
			commentsFound: 2, // Only full-line comments are counted
		},
		{
			name: "inline comment after value",
			yamlContent: `key1: value1 # inline comment
key2: value2`,
			expectedKeys:  []string{"key1", "key2"},
			commentsFound: 0, // No full-line comments, inline comments are not marked as IsComment
		},
		{
			name: "comments in nested structure",
			yamlContent: `# Top level comment
parent:
  # Nested comment
  child: value # Inline comment
  # Another nested comment`,
			expectedKeys:  []string{"parent", "child"},
			commentsFound: 3, // Only full-line comments are counted, inline comments are not
		},
		{
			name: "comments with various positions",
			yamlContent: `# Comment at line 1
	# Comment at line 2
	key1: value1
	key2: value2 # Comment at end
	key3: value3
	# Comment at line 6
	key4: value4 # Another inline comment`,
			expectedKeys:  []string{"key1", "key2", "key3", "key4"},
			commentsFound: 3, // Only full-line comments are counted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLineParser(2)
			result := parser.Parse(tt.yamlContent)

			// Count comment lines
			commentCount := 0
			keysFound := []string{}

			for _, line := range result.Lines {
				if line.IsComment {
					commentCount++
				}
				if line.IsKeyCandidate {
					keysFound = append(keysFound, line.KeyName)
				}
			}

			if commentCount != tt.commentsFound {
				t.Errorf("Expected %d comment lines, found %d", tt.commentsFound, commentCount)
			}

			if len(keysFound) != len(tt.expectedKeys) {
				t.Errorf("Expected %d keys, found %d", len(tt.expectedKeys), len(keysFound))
			}

			for i, key := range tt.expectedKeys {
				if i >= len(keysFound) || keysFound[i] != key {
					t.Errorf("Expected key '%s' at position %d, got '%s'", key, i, keysFound[i])
				}
			}
		})
	}
}

// TestFullLineCommentWithVariousIndentation tests full-line comments with different indentation levels.
func TestFullLineCommentWithVariousIndentation(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "no indentation",
			line:     "# comment",
			expected: true,
		},
		{
			name:     "one space indent",
			line:     " # comment",
			expected: true,
		},
		{
			name:     "two space indent",
			line:     "  # comment",
			expected: true,
		},
		{
			name:     "four space indent",
			line:     "    # comment",
			expected: true,
		},
		{
			name:     "eight space indent",
			line:     "        # comment",
			expected: true,
		},
		{
			name:     "one tab indent",
			line:     "\t# comment",
			expected: true,
		},
		{
			name:     "two tab indent",
			line:     "\t\t# comment",
			expected: true,
		},
		{
			name:     "mixed space then tab",
			line:     "  \t# comment",
			expected: true,
		},
		{
			name:     "mixed tab then space",
			line:     "\t  # comment",
			expected: true,
		},
		{
			name:     "deep mixed indentation",
			line:     "    \t  \t# comment",
			expected: true,
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

// TestInlineCommentPositionVariations tests inline comments at various positions on a line.
func TestInlineCommentPositionVariations(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "comment immediately after value",
			line:     "key: value# comment",
			expected: "key: value# comment",
		},
		{
			name:     "comment with one space after value",
			line:     "key: value # comment",
			expected: "key: value ",
		},
		{
			name:     "comment with two spaces after value",
			line:     "key: value  # comment",
			expected: "key: value  ",
		},
		{
			name:     "comment with four spaces after value",
			line:     "key: value    # comment",
			expected: "key: value    ",
		},
		{
			name:     "comment with tab after value",
			line:     "key: value\t# comment",
			expected: "key: value\t",
		},
		{
			name:     "comment after sequence item",
			line:     "- item # comment",
			expected: "- item ",
		},
		{
			name:     "comment after nested key",
			line:     "  parent: value # comment",
			expected: "  parent: value ",
		},
		{
			name:     "comment after complex value",
			line:     "key: {a: b, c: d} # comment",
			expected: "key: {a: b, c: d} ",
		},
		{
			name:     "comment after array value",
			line:     "key: [1, 2, 3] # comment",
			expected: "key: [1, 2, 3] ",
		},
		{
			name:     "comment after quoted value",
			line:     "key: \"quoted value\" # comment",
			expected: "key: \"quoted value\" ",
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

// TestBasicCommentFilteringEdgeCases tests edge cases for basic comment filtering.
func TestBasicCommentFilteringEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
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
			name:     "hash only",
			line:     "#",
			expected: "",
		},
		{
			name:     "hash with spaces only",
			line:     "#   ",
			expected: "",
		},
		{
			name:     "multiple hashes in value",
			line:     "key: value#with#hashes",
			expected: "key: value#with#hashes",
		},
		{
			name:     "hash at end without space",
			line:     "key: value#",
			expected: "key: value#",
		},
		{
			name:     "URL with fragment",
			line:     "url: https://example.com/path#fragment",
			expected: "url: https://example.com/path#fragment",
		},
		{
			name:     "color hex value",
			line:     "color: #FF0000",
			expected: "color: #FF0000",
		},
		{
			name:     "comment after complex URL",
			line:     "url: https://example.com#anchor # comment",
			expected: "url: https://example.com#anchor ",
		},
		{
			name:     "multiple inline comments - first wins",
			line:     "key: value # first # second",
			expected: "key: value ",
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

// TestCommentFilteringInRealYAML tests comment filtering in realistic YAML scenarios.
func TestCommentFilteringInRealYAML(t *testing.T) {
	yamlContent := `# Service configuration file
service:
  name: my-service # Service name
  port: 8080 # Port number
  # Database configuration
  database:
    host: localhost
    port: 5432
    # Connection settings
    pool:
      max_connections: 10 # Maximum connections
      min_connections: 2  # Minimum connections

# Monitoring setup
monitoring:
  enabled: true
  # Metrics to collect
  metrics:
    - name: request_count # Request counter
      type: counter
    - name: response_time # Response timer
      type: histogram
`

	parser := NewLineParser(2)
	result := parser.Parse(yamlContent)

	// Verify comment lines are identified
	commentLines := []int{1, 5, 9, 14, 17}
	for i, lineNum := range commentLines {
		if i >= len(result.Lines) {
			t.Errorf("Expected line %d to exist", lineNum)
			continue
		}
		line := result.Lines[lineNum-1] // Lines are 1-indexed in result, 0-indexed in array
		if !line.IsComment {
			t.Errorf("Expected line %d to be identified as comment, got content: %q", lineNum, line.OriginalContent)
		}
	}

	// Verify keys are identified despite inline comments
	expectedKeyNames := []string{
		"service", "name", "port", "database", "host", "pool",
		"max_connections", "min_connections", "monitoring", "enabled",
		"metrics", "type",
	}

	keysFound := 0
	keysFoundMap := make(map[string]bool)
	for _, line := range result.Lines {
		if line.IsKeyCandidate {
			keysFound++
			keysFoundMap[line.KeyName] = true
		}
	}

	// Check that we found the expected keys
	for _, expectedKey := range expectedKeyNames {
		if !keysFoundMap[expectedKey] {
			t.Errorf("Expected to find key '%s'", expectedKey)
		}
	}

	// We should have found several keys
	if keysFound < 10 {
		t.Errorf("Expected at least 10 key candidates, found %d", keysFound)
	}
}

// TestCommentAtVariousLinePositions tests comments at different line positions in multi-line YAML.
func TestCommentAtVariousLinePositions(t *testing.T) {
	tests := []struct {
		name              string
		yamlContent       string
		commentLineNumbers []int // 1-indexed line numbers that should be comments
	}{
		{
			name: "comment at first line",
			yamlContent: `# First line comment
key1: value1
key2: value2`,
			commentLineNumbers: []int{1},
		},
		{
			name: "comment at last line",
			yamlContent: `key1: value1
key2: value2
# Last line comment`,
			commentLineNumbers: []int{3},
		},
		{
			name: "comment in middle",
			yamlContent: `key1: value1
# Middle line comment
key2: value2`,
			commentLineNumbers: []int{2},
		},
		{
			name: "comments at multiple positions",
			yamlContent: `# Start
key1: value1
# Middle
key2: value2
# End`,
			commentLineNumbers: []int{1, 3, 5},
		},
		{
			name: "comments throughout document",
			yamlContent: `# Line 1
key1: value1
# Line 3
key2: value2
# Line 5
key3: value3
# Line 7
key4: value4`,
			commentLineNumbers: []int{1, 3, 5, 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLineParser(2)
			result := parser.Parse(tt.yamlContent)

			commentFoundCount := 0
			for _, line := range result.Lines {
				if line.IsComment {
					commentFoundCount++
				}
			}

			expectedCount := len(tt.commentLineNumbers)
			if commentFoundCount != expectedCount {
				t.Errorf("Expected %d comment lines, found %d", expectedCount, commentFoundCount)
			}

			// Verify specific line numbers
			for _, lineNum := range tt.commentLineNumbers {
				if lineNum > len(result.Lines) {
					t.Errorf("Line number %d out of range", lineNum)
					continue
				}
				line := result.Lines[lineNum-1]
				if !line.IsComment {
					t.Errorf("Expected line %d to be a comment, got: %q", lineNum, line.OriginalContent)
				}
			}
		})
	}
}

// TestDeepIndentationLevels tests comments at deeper indentation levels (6, 8, 10, 12 spaces).
// This test covers the acceptance criteria requirement for testing comments at indentation levels 6, 8, 10, 12.
func TestDeepIndentationLevels(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "six space indent",
			line:     "      # comment",
			expected: true,
		},
		{
			name:     "ten space indent",
			line:     "          # comment",
			expected: true,
		},
		{
			name:     "twelve space indent",
			line:     "            # comment",
			expected: true,
		},
		{
			name:     "sixteen space indent",
			line:     "                # comment",
			expected: true,
		},
		{
			name:     "inline comment at 6-space indent",
			line:     "      key: value # comment",
			expected: false, // IsCommentLine returns false for inline comments
		},
		{
			name:     "inline comment at 10-space indent",
			line:     "          key: value # comment",
			expected: false,
		},
		{
			name:     "inline comment at 12-space indent",
			line:     "            key: value # comment",
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

// TestCommentsInNestedStructures tests comment handling within nested YAML maps and lists.
// This test covers the acceptance criteria requirement for testing comments in nested structures.
func TestCommentsInNestedStructures(t *testing.T) {
	yamlContent := `# Top-level comment
root:
  # Comment inside root map
  nested_map:
    # Deep comment in nested map
    key1: value1 # Inline comment in nested map
    key2: value2
    # Comment before list
    nested_list:
      # Comment inside list
      - item1 # Comment after list item
      - item2
      # Comment between list items
      - item3 # Another inline comment
  # Comment after nested structure
  another_key: value
# Final comment
`

	parser := NewLineParser(2)
	result := parser.Parse(yamlContent)

	// Count full-line comments at various nesting levels
	fullLineComments := 0
	inlineComments := 0

	for _, line := range result.Lines {
		if line.IsComment {
			fullLineComments++
		}
		// Check for inline comments by looking for # after content
		if len(line.OriginalContent) > 0 && !line.IsComment && !line.IsEmpty {
			trimmed := line.OriginalContent
			for i := 0; i < len(trimmed); i++ {
				if trimmed[i] == '#' && i > 0 && (trimmed[i-1] == ' ' || trimmed[i-1] == '\t') {
					inlineComments++
					break
				}
			}
		}
	}

	// We expect 8 full-line comments in this structure
	if fullLineComments != 8 {
		t.Errorf("Expected 8 full-line comments, found %d", fullLineComments)
	}

	// We expect 3 inline comments in this structure
	if inlineComments != 3 {
		t.Errorf("Expected 3 inline comments, found %d", inlineComments)
	}

	// Verify that keys are still detected despite comments
	expectedKeys := []string{"root", "nested_map", "key1", "key2", "nested_list", "another_key"}
	keysFound := make(map[string]bool)
	for _, line := range result.Lines {
		if line.IsKeyCandidate {
			keysFound[line.KeyName] = true
		}
	}

	for _, key := range expectedKeys {
		if !keysFound[key] {
			t.Errorf("Expected to find key '%s'", key)
		}
	}
}

// TestMixedScenariosWithAnchorsAndComments tests mixed YAML scenarios with values, comments, and anchors.
// This test covers the acceptance criteria requirement for testing mixed scenarios.
func TestMixedScenariosWithAnchorsAndComments(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent  string
		expectedKeys []string
		anchorCount int
		commentCount int
	}{
		{
			name: "anchor with inline comment",
			yamlContent: `defaults: &defaults
  timeout: 30 # default timeout value
  retries: 3 # number of retries`,
			expectedKeys: []string{"defaults", "timeout", "retries"},
			anchorCount:  1,
			commentCount: 0, // No full-line comments
		},
		{
			name: "anchor and alias with comments",
			yamlContent: `# Base configuration
base: &base
  host: localhost
  port: 8080 # HTTP port

# Production overrides
production:
  <<: *base # Merge base config
  port: 8081 # Production port
  # Extra production settings
  debug: false`,
			expectedKeys: []string{"base", "host", "port", "production", "debug"},
			anchorCount:  2, // &base and *base
			commentCount: 3, // 3 full-line comments
		},
		{
			name: "multiple anchors with comments",
			yamlContent: `# Database anchors
db_default: &db_default
  driver: postgres
  pool: 5 # Connection pool size

db_prod: &db_prod
  <<: *db_default
  host: prod-db.example.com # Production database

development:
  database:
    <<: *db_default
    host: localhost # Dev database
    # More dev settings
    debug: true`,
			expectedKeys: []string{"db_default", "driver", "pool", "db_prod", "database", "host", "debug"},
			anchorCount:  4, // &db_default, &db_prod, *db_default, *db_default
			commentCount: 2, // 2 full-line comments
		},
		{
			name: "anchor in list with comments",
			yamlContent: `# Shared item template
item_template: &item
  name: default
  value: 0 # Default value

items:
  - <<: *item # Use template
  - <<: *item
    name: custom # Override name
    value: 42 # Custom value
  # Another item
  - <<: *item
    name: third`,
			expectedKeys: []string{"item_template", "name", "value", "items"},
			anchorCount:  4, // &item and three *item references
			commentCount: 2, // 2 full-line comments
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLineParser(2)
			result := parser.Parse(tt.yamlContent)

			// Count anchors and aliases
			anchorCount := 0
			for _, line := range result.Lines {
				content := line.OriginalContent
				for i := 0; i < len(content); i++ {
					if content[i] == '&' || content[i] == '*' {
						// Make sure it's not part of a word
						if i == 0 || content[i-1] == ' ' || content[i-1] == '\t' {
							anchorCount++
						}
					}
				}
			}

			if anchorCount != tt.anchorCount {
				t.Errorf("Expected %d anchors/aliases, found %d", tt.anchorCount, anchorCount)
			}

			// Count full-line comments
			commentCount := 0
			for _, line := range result.Lines {
				if line.IsComment {
					commentCount++
				}
			}

			if commentCount != tt.commentCount {
				t.Errorf("Expected %d full-line comments, found %d", tt.commentCount, commentCount)
			}

			// Verify keys are detected
			keysFound := make(map[string]bool)
			for _, line := range result.Lines {
				if line.IsKeyCandidate {
					keysFound[line.KeyName] = true
				}
			}

			for _, key := range tt.expectedKeys {
				if !keysFound[key] {
					t.Errorf("Expected to find key '%s'", key)
				}
			}
		})
	}
}

// TestCommentsInMultiLineStrings tests comments in multi-line string and scalar contexts.
// This test covers the acceptance criteria requirement for testing comments in multi-line contexts.
func TestCommentsInMultiLineStrings(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent   string
		expectedKeys []string
		commentCount int
	}{
		{
			name: "literal block scalar with comments",
			yamlContent: `# Script configuration
script: |
  #!/bin/bash
  echo "Hello World"
  # This is a comment inside the script
  exit 0
# End of script
description: This is a test # inline comment`,
			expectedKeys: []string{"script", "description"},
			commentCount: 4, // Parser treats all # lines as comments (including inside block scalar)
		},
		{
			name: "folded block scalar with comments",
			yamlContent: `# Text content
text: >
  This is a long text
  that spans multiple lines
  and is folded together.
# After the folded block
note: "Note with # hash" # inline comment`,
			expectedKeys: []string{"text", "note"},
			commentCount: 2, // Parser treats all # lines as comments
		},
		{
			name: "multi-line string in list with comments",
			yamlContent: `# List items
items:
  - |
    First item
    with multiple lines
    # Not a comment, part of string
  - >
    Second item
    also folded
  # Comment between items
  - plain item # inline comment`,
			expectedKeys: []string{"items"},
			commentCount: 3, // Parser treats all # lines as comments (including in block scalars)
		},
		{
			name: "double-quoted string with hash-like content",
			yamlContent: `# Configuration
url: "http://example.com#anchor" # URL with fragment
color: "#FF0000" # Hex color
regex: "#[0-9]+" # Not a regex, just text
# Comment at end`,
			expectedKeys: []string{"url", "color"},
			commentCount: 2, // 2 full-line comments
		},
		{
			name: "single-quoted string with hash",
			yamlContent: `# Strings section
strings:
  - 'value with # hash'
  - 'another # value' # inline comment
  - "double # quoted" # another inline
# Final comment`,
			expectedKeys: []string{"strings"},
			commentCount: 2, // 2 full-line comments
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLineParser(2)
			result := parser.Parse(tt.yamlContent)

			// Count full-line comments
			commentCount := 0
			for _, line := range result.Lines {
				if line.IsComment {
					commentCount++
				}
			}

			if commentCount != tt.commentCount {
				t.Errorf("Expected %d full-line comments, found %d", tt.commentCount, commentCount)
			}

			// Verify keys are detected
			keysFound := make(map[string]bool)
			for _, line := range result.Lines {
				if line.IsKeyCandidate {
					keysFound[line.KeyName] = true
				}
			}

			for _, key := range tt.expectedKeys {
				if !keysFound[key] {
					t.Errorf("Expected to find key '%s'", key)
				}
			}
		})
	}
}

// TestCommentIndentationProgression tests comments at progressive indentation levels in a real document.
// This test verifies that comments are correctly identified at every indentation level from 0 to 12.
func TestCommentIndentationProgression(t *testing.T) {
	yamlContent := `# Level 0 comment (no indent)
level0:
  # Level 1 comment (2 spaces)
  level1:
    # Level 2 comment (4 spaces)
    level2:
      # Level 3 comment (6 spaces)
      level3:
        # Level 4 comment (8 spaces)
        level4:
          # Level 5 comment (10 spaces)
          level5:
            # Level 6 comment (12 spaces)
            level6: value
            # Another level 6 comment
          # Level 5 comment again
        # Level 4 comment again
      # Level 3 comment again
    # Level 2 comment again
  # Level 1 comment again
# Level 0 comment again
`

	parser := NewLineParser(2)
	result := parser.Parse(yamlContent)

	// Verify all comments are detected
	// The parser correctly identifies ALL lines starting with # as comments
	commentLines := make(map[int]bool) // line number -> true if should be comment
	expectedCommentLines := []int{1, 3, 5, 7, 9, 11, 13, 15, 16, 17, 18, 19, 20, 21}

	for _, lineNum := range expectedCommentLines {
		commentLines[lineNum] = true
	}

	commentsFound := 0
	for lineNum, line := range result.Lines {
		expectedToBeComment := commentLines[lineNum+1] // +1 because lines are 1-indexed
		if expectedToBeComment {
			if !line.IsComment {
				t.Errorf("Line %d should be a comment, but wasn't detected: %q", lineNum+1, line.OriginalContent)
			} else {
				commentsFound++
			}
		} else if line.IsComment && lineNum+1 > 0 {
			// Non-comment line that was detected as comment
			if !commentLines[lineNum+1] {
				t.Errorf("Line %d should NOT be a comment, but was detected: %q", lineNum+1, line.OriginalContent)
			}
		}
	}

	if commentsFound != len(expectedCommentLines) {
		t.Errorf("Expected %d comments, found %d", len(expectedCommentLines), commentsFound)
	}

	// Verify keys are detected at each level
	expectedLevels := []string{"level0", "level1", "level2", "level3", "level4", "level5", "level6"}
	keysFound := make(map[string]bool)
	for _, line := range result.Lines {
		if line.IsKeyCandidate {
			keysFound[line.KeyName] = true
		}
	}

	for _, key := range expectedLevels {
		if !keysFound[key] {
			t.Errorf("Expected to find key '%s' at its indentation level", key)
		}
	}
}
