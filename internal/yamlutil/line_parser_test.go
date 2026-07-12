// Package yamlutil tests the YAML line parser and key identifier.
package yamlutil

import (
	"strings"
	"testing"
)

// TestNewLineParser verifies line parser creation.
func TestNewLineParser(t *testing.T) {
	parser := NewLineParser(2)
	if parser == nil {
		t.Fatal("NewLineParser returned nil")
	}
	if parser.indentSpaces != 2 {
		t.Errorf("Expected indentSpaces=2, got %d", parser.indentSpaces)
	}
}

// TestNewKeyIdentifier verifies key identifier creation.
func TestNewKeyIdentifier(t *testing.T) {
	identifier := NewKeyIdentifier(2)
	if identifier == nil {
		t.Fatal("NewKeyIdentifier returned nil")
	}
	if identifier.parser == nil {
		t.Error("KeyIdentifier parser is nil")
	}
}

// TestParseSimpleYAML verifies parsing of simple YAML content.
func TestParseSimpleYAML(t *testing.T) {
	parser := NewLineParser(2)
	yamlContent := `
key1: value1
key2: value2
`

	result := parser.Parse(strings.TrimSpace(yamlContent))

	if result.TotalLines != 2 {
		t.Errorf("Expected 2 lines, got %d", result.TotalLines)
	}

	// Verify first line is a key candidate
	if !result.Lines[0].IsKeyCandidate {
		t.Error("First line should be a key candidate")
	}
	if result.Lines[0].KeyName != "key1" {
		t.Errorf("Expected key name 'key1', got '%s'", result.Lines[0].KeyName)
	}
	if result.Lines[0].LineNumber != 1 {
		t.Errorf("Expected line number 1, got %d", result.Lines[0].LineNumber)
	}
}

// TestParseNestedYAML verifies parsing of nested YAML structure.
func TestParseNestedYAML(t *testing.T) {
	parser := NewLineParser(2)
	yamlContent := `
parent:
  child1: value1
  child2: value2
  nested:
    deep: value3
`

	result := parser.Parse(strings.TrimSpace(yamlContent))

	if result.TotalLines != 5 {
		t.Errorf("Expected 5 lines, got %d", result.TotalLines)
	}

	// Check parent key (no indent)
	parentLine := result.Lines[0]
	if !parentLine.IsKeyCandidate {
		t.Error("Parent line should be a key candidate")
	}
	if parentLine.Indentation != 0 {
		t.Errorf("Parent should have 0 indentation, got %d", parentLine.Indentation)
	}

	// Check child keys (2 spaces indent)
	child1Line := result.Lines[1]
	if !child1Line.IsKeyCandidate {
		t.Error("Child1 line should be a key candidate")
	}
	if child1Line.Indentation != 2 {
		t.Errorf("Child1 should have 2 spaces indentation, got %d", child1Line.Indentation)
	}

	// Check nested key (4 spaces indent)
	nestedLine := result.Lines[3]
	if nestedLine.KeyName != "nested" {
		t.Errorf("Expected key name 'nested', got '%s'", nestedLine.KeyName)
	}

	// Check deep key (4 spaces indent - nested under nested which is at 2 spaces)
	deepLine := result.Lines[4]
	if deepLine.Indentation != 4 {
		t.Errorf("Deep should have 4 spaces indentation, got %d", deepLine.Indentation)
	}
}

// TestParseSequenceItems verifies parsing of YAML sequences.
func TestParseSequenceItems(t *testing.T) {
	parser := NewLineParser(2)
	yamlContent := `
items:
  - item1
  - item2
  - name: item3
    value: test
`

	result := parser.Parse(strings.TrimSpace(yamlContent))

	if result.TotalLines != 5 {
		t.Errorf("Expected 5 lines, got %d", result.TotalLines)
	}

	// Check that sequence items are identified
	if result.SequenceItems != 3 {
		t.Errorf("Expected 3 sequence items, got %d", result.SequenceItems)
	}

	// Check first sequence item
	item1Line := result.Lines[1]
	if !item1Line.IsSequenceItem {
		t.Error("Line 2 should be a sequence item")
	}

	// Check nested mapping in sequence
	item3Line := result.Lines[3]
	if !item3Line.IsKeyCandidate {
		t.Error("Nested 'name' key should be a key candidate")
	}
	if item3Line.KeyName != "name" {
		t.Errorf("Expected key name 'name', got '%s'", item3Line.KeyName)
	}
}

// TestParseEmptyAndCommentLines verifies handling of empty and comment lines.
func TestParseEmptyAndCommentLines(t *testing.T) {
	parser := NewLineParser(2)
	yamlContent := `
# This is a comment
key1: value1

# Another comment
key2: value2
`

	result := parser.Parse(strings.TrimSpace(yamlContent))

	if result.TotalLines != 5 {
		t.Errorf("Expected 5 lines, got %d", result.TotalLines)
	}

	// Count empty lines (line 3 is empty)
	if result.EmptyLines != 1 {
		t.Errorf("Expected 1 empty line, got %d", result.EmptyLines)
	}

	// Count comment lines (lines 1 and 4)
	if result.CommentLines != 2 {
		t.Errorf("Expected 2 comment lines, got %d", result.CommentLines)
	}

	// Verify comment lines are identified
	if !result.Lines[0].IsComment {
		t.Error("Line 1 should be identified as comment")
	}
	if !result.Lines[3].IsComment {
		t.Error("Line 4 should be identified as comment")
	}

	// Verify empty line is identified
	if !result.Lines[2].IsEmpty {
		t.Error("Line 3 should be identified as empty")
	}
}

// TestIndentationDetection verifies auto-detection of indentation.
func TestIndentationDetection(t *testing.T) {
	parser := NewLineParser(0) // Auto-detect
	yamlContent := `
key1: value1
  key2: value2
    key3: value3
`

	result := parser.Parse(strings.TrimSpace(yamlContent))

	// Should detect 2-space indentation
	if result.IndentSpaces != 2 {
		t.Errorf("Expected auto-detection to find 2-space indent, got %d", result.IndentSpaces)
	}

	// Verify maximum indent level
	// With 2-space indent and max of 4 spaces, the max level should be 2 (4/2 = 2)
	// Levels: 0 (0 spaces), 1 (2 spaces), 2 (4 spaces)
	if result.MaxIndentLevel != 2 {
		t.Errorf("Expected max indent level 2, got %d", result.MaxIndentLevel)
	}
}

// TestIndentationTypeDetection verifies detection of tab vs space indentation.
func TestIndentationTypeDetection(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectTabs  bool
		expectMixed bool
	}{
		{
			name: "space indentation",
			content: `
key1: value1
  key2: value2
`,
			expectTabs:  false,
			expectMixed: false,
		},
		{
			name: "tab indentation",
			content: `
key1: value1
	key2: value2
`,
			expectTabs:  true,
			expectMixed: false,
		},
		{
			name: "mixed indentation",
			content: `
key1: value1
  key2: value2
	key3: value3
`,
			expectTabs:  true,
			expectMixed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewLineParser(0)
			result := parser.Parse(strings.TrimSpace(tt.content))

			if result.IndentTabs != tt.expectTabs {
				t.Errorf("Expected IndentTabs=%v, got %v", tt.expectTabs, result.IndentTabs)
			}
			if result.HasMixedIndent != tt.expectMixed {
				t.Errorf("Expected HasMixedIndent=%v, got %v", tt.expectMixed, result.HasMixedIndent)
			}
		})
	}
}

// TestKeyExtraction verifies extraction of key names from lines.
func TestKeyExtraction(t *testing.T) {
	parser := NewLineParser(2)
	yamlContent := `
string_key: value1
underscore_key: value2
hyphen-key: value3
dotted.key: value4
'single_quoted': value5
"double_quoted": value6
key123: value7
`

	result := parser.Parse(strings.TrimSpace(yamlContent))

	expectedKeys := []string{
		"string_key",
		"underscore_key",
		"hyphen-key",
		"dotted.key",
		"single_quoted",
		"double_quoted",
		"key123",
	}

	if result.KeyCandidates != len(expectedKeys) {
		t.Errorf("Expected %d key candidates, got %d", len(expectedKeys), result.KeyCandidates)
	}

	for i, line := range result.Lines {
		if line.IsKeyCandidate {
			if i < len(expectedKeys) {
				if line.KeyName != expectedKeys[i] {
					t.Errorf("Line %d: expected key '%s', got '%s'", i, expectedKeys[i], line.KeyName)
				}
			}
		}
	}
}

// TestDocumentMarkers verifies detection of YAML document markers.
func TestDocumentMarkers(t *testing.T) {
	parser := NewLineParser(2)
	yamlContent := `---
key1: value1
---
key2: value2
...
`

	result := parser.Parse(strings.TrimSpace(yamlContent))

	// Check document start marker
	if !result.Lines[0].IsDocumentStart {
		t.Error("Line 1 should be identified as document start")
	}

	// Check second document start marker
	if !result.Lines[2].IsDocumentStart {
		t.Error("Line 3 should be identified as document start")
	}

	// Check document end marker
	if !result.Lines[4].IsDocumentEnd {
		t.Error("Line 5 should be identified as document end")
	}
}

// TestFlowStyleDetection verifies detection of flow-style YAML.
func TestFlowStyleDetection(t *testing.T) {
	parser := NewLineParser(2)
	yamlContent := `
flow_mapping: {key1: value1, key2: value2}
flow_sequence: [item1, item2, item3]
normal: value
`

	result := parser.Parse(strings.TrimSpace(yamlContent))

	// Verify we have 3 lines
	if result.TotalLines != 3 {
		t.Errorf("Expected 3 lines, got %d", result.TotalLines)
	}

	// Flow mapping line (line 0)
	flowMappingLine := result.Lines[0]
	if !flowMappingLine.InFlowStyle {
		t.Error("Line 1 should be identified as flow style")
	}

	// Flow sequence line (line 1)
	flowSequenceLine := result.Lines[1]
	if !flowSequenceLine.InFlowStyle {
		t.Error("Line 2 should be identified as flow style")
	}

	// Normal block style line (line 2)
	normalLine := result.Lines[2]
	if normalLine.InFlowStyle {
		t.Error("Line 3 should NOT be identified as flow style")
	}
}

// TestKeyIdentifierIdentifyKeys verifies key identification.
func TestKeyIdentifierIdentifyKeys(t *testing.T) {
	identifier := NewKeyIdentifier(2)
	yamlContent := `
parent:
  child1: value1
  child2: value2
`

	result := identifier.IdentifyKeys(strings.TrimSpace(yamlContent))

	if result.KeyCandidates != 3 {
		t.Errorf("Expected 3 key candidates, got %d", result.KeyCandidates)
	}

	// Verify key names
	keyLines := identifier.GetKeyLines(result)
	if len(keyLines) != 3 {
		t.Fatalf("Expected 3 key lines, got %d", len(keyLines))
	}

	expectedKeys := []string{"parent", "child1", "child2"}
	for i, line := range keyLines {
		if line.KeyName != expectedKeys[i] {
			t.Errorf("Key %d: expected '%s', got '%s'", i, expectedKeys[i], line.KeyName)
		}
	}
}

// TestGetKeyHierarchy verifies key hierarchy organization.
func TestGetKeyHierarchy(t *testing.T) {
	identifier := NewKeyIdentifier(2)
	yamlContent := `
level0:
  level1_a:
    level2_a: value
  level1_b: value
`

	result := identifier.IdentifyKeys(strings.TrimSpace(yamlContent))
	hierarchy := identifier.GetKeyHierarchy(result)

	// Should have 3 levels (0, 1, 2)
	if len(hierarchy) != 3 {
		t.Errorf("Expected 3 hierarchy levels, got %d", len(hierarchy))
	}

	// Level 0 should have 1 key (level0)
	if len(hierarchy[0]) != 1 {
		t.Errorf("Expected 1 key at level 0, got %d", len(hierarchy[0]))
	}
	if hierarchy[0][0].KeyName != "level0" {
		t.Errorf("Expected 'level0' at hierarchy level 0, got '%s'", hierarchy[0][0].KeyName)
	}

	// Level 1 should have 2 keys (level1_a, level1_b)
	if len(hierarchy[1]) != 2 {
		t.Errorf("Expected 2 keys at level 1, got %d", len(hierarchy[1]))
	}

	// Level 2 should have 1 key (level2_a)
	if len(hierarchy[2]) != 1 {
		t.Errorf("Expected 1 key at level 2, got %d", len(hierarchy[2]))
	}
	if hierarchy[2][0].KeyName != "level2_a" {
		t.Errorf("Expected 'level2_a' at hierarchy level 2, got '%s'", hierarchy[2][0].KeyName)
	}
}

// TestComplexRealWorldYAML tests parsing of a realistic YAML configuration.
func TestComplexRealWorldYAML(t *testing.T) {
	parser := NewLineParser(2)
	yamlContent := `# Service configuration
service:
  name: my-service
  port: 8080
  # Database settings
  database:
    host: localhost
    port: 5432
    # Connection pool
    pool:
      max_connections: 10
      min_connections: 2

# Monitoring configuration
monitoring:
  enabled: true
  metrics:
    - name: request_count
      type: counter
    - name: response_time
      type: histogram
`

	result := parser.Parse(yamlContent)

	// Verify line count (22 lines: 2 empty, 4 comments, 16 keys, 2 sequence items)
	if result.TotalLines != 22 {
		t.Errorf("Expected 22 lines, got %d", result.TotalLines)
	}

	// Count comments (4 comment lines: lines 1, 5, 9, 14)
	if result.CommentLines != 4 {
		t.Errorf("Expected 4 comment lines, got %d", result.CommentLines)
	}

	// Count empty lines (2 empty lines: lines 13 and 22)
	if result.EmptyLines != 2 {
		t.Errorf("Expected 2 empty lines, got %d", result.EmptyLines)
	}

	// Count key candidates (16 keys: 10 top-level + 2 nested + 4 in sequences)
	if result.KeyCandidates != 16 {
		t.Errorf("Expected 16 key candidates, got %d", result.KeyCandidates)
	}

	// Count sequence items (2 sequence items in metrics)
	if result.SequenceItems != 2 {
		t.Errorf("Expected 2 sequence items, got %d", result.SequenceItems)
	}

	// Verify some specific keys (adjusting for actual line numbers)
	expectedKeys := map[string]int{
		"service":           2,
		"name":              3,
		"port":              4,
		"database":          6,
		"host":              7,
		"pool":             10,
		"max_connections":  11,
		"min_connections":  12,
		"monitoring":       15,
		"enabled":          16,
		"metrics":          17,
	}

	for keyName, expectedLine := range expectedKeys {
		found := false
		for _, line := range result.Lines {
			if line.IsKeyCandidate && line.KeyName == keyName {
				found = true
				if line.LineNumber != expectedLine {
					t.Errorf("Key '%s': expected line %d, got %d", keyName, expectedLine, line.LineNumber)
				}
				break
			}
		}
		if !found {
			t.Errorf("Key '%s' not found in parsed lines", keyName)
		}
	}
}

// TestKeyCandidateValidation verifies key validation logic.
func TestKeyCandidateValidation(t *testing.T) {
	parser := NewLineParser(2)

	tests := []struct {
		name     string
		line     string
		isValid  bool
		keyName  string
	}{
		{
			name:    "valid simple key",
			line:    "my_key: value",
			isValid: true,
			keyName: "my_key",
		},
		{
			name:    "valid numeric key",
			line:    "key123: value",
			isValid: true,
			keyName: "key123",
		},
		{
			name:    "valid dotted key",
			line:    "my.key: value",
			isValid: true,
			keyName: "my.key",
		},
		{
			name:    "valid hyphenated key",
			line:    "my-key: value",
			isValid: true,
			keyName: "my-key",
		},
		{
			name:    "valid single quoted key",
			line:    "'quoted key': value",
			isValid: true,
			keyName: "quoted key",
		},
		{
			name:    "valid double quoted key",
			line:    `"quoted key": value`,
			isValid: true,
			keyName: "quoted key",
		},
		{
			name:    "no colon",
			line:    "not_a_key",
			isValid: false,
			keyName: "",
		},
		{
			name:    "empty key",
			line:    ": value",
			isValid: false,
			keyName: "",
		},
		{
			name:    "colon only",
			line:    ":",
			isValid: false,
			keyName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := parser.parseLine(1, tt.line)
			if line.IsKeyCandidate != tt.isValid {
				t.Errorf("Expected IsKeyCandidate=%v for line '%s', got %v",
					tt.isValid, tt.line, line.IsKeyCandidate)
			}
			if tt.isValid && line.KeyName != tt.keyName {
				t.Errorf("Expected key name '%s', got '%s'", tt.keyName, line.KeyName)
			}
		})
	}
}

// TestInvalidKeyHandling verifies handling of invalid key patterns.
func TestInvalidKeyHandling(t *testing.T) {
	parser := NewLineParser(2)
	yamlContent := `
normal_key: value1
  starts with space: value2
:key_without_name: value3
@@invalid_chars@@: value4
`

	result := parser.Parse(strings.TrimSpace(yamlContent))

	// Only normal_key should be valid
	if result.KeyCandidates != 1 {
		t.Errorf("Expected 1 key candidate, got %d", result.KeyCandidates)
	}

	// Verify the valid key (at index 0 after TrimSpace)
	if result.Lines[0].KeyName != "normal_key" {
		t.Errorf("Expected 'normal_key', got '%s'", result.Lines[0].KeyName)
	}
}

// TestLineMetadata verifies comprehensive line metadata.
func TestLineMetadata(t *testing.T) {
	parser := NewLineParser(2)
	yamlContent := `  key: value`

	result := parser.Parse(yamlContent)

	line := result.Lines[0]

	// Verify all metadata fields
	if line.LineNumber != 1 {
		t.Errorf("Expected LineNumber=1, got %d", line.LineNumber)
	}

	if line.OriginalContent != "  key: value" {
		t.Errorf("Expected original content '  key: value', got '%s'", line.OriginalContent)
	}

	if line.TrimmedContent != "key: value" {
		t.Errorf("Expected trimmed content 'key: value', got '%s'", line.TrimmedContent)
	}

	if line.Indentation != 2 {
		t.Errorf("Expected Indentation=2, got %d", line.Indentation)
	}

	if line.IndentType != "space" {
		t.Errorf("Expected IndentType='space', got '%s'", line.IndentType)
	}

	if !line.IsKeyCandidate {
		t.Error("Expected IsKeyCandidate=true")
	}

	if line.KeyName != "key" {
		t.Errorf("Expected KeyName='key', got '%s'", line.KeyName)
	}

	if !line.HasColon {
		t.Error("Expected HasColon=true")
	}

	if line.IsEmpty || line.IsComment || line.IsSequenceItem {
		t.Error("Line should not be empty, comment, or sequence item")
	}
}

// TestMultipleDocuments verifies handling of multiple YAML documents.
func TestMultipleDocuments(t *testing.T) {
	parser := NewLineParser(2)
	yamlContent := `---
document1_key: value
...
---
document2_key: value
...
`

	result := parser.Parse(strings.TrimSpace(yamlContent))

	// Count document markers
	docStartCount := 0
	docEndCount := 0

	for _, line := range result.Lines {
		if line.IsDocumentStart {
			docStartCount++
		}
		if line.IsDocumentEnd {
			docEndCount++
		}
	}

	if docStartCount != 2 {
		t.Errorf("Expected 2 document start markers, got %d", docStartCount)
	}

	if docEndCount != 2 {
		t.Errorf("Expected 2 document end markers, got %d", docEndCount)
	}
}

// TestLineParserEdgeCases verifies handling of edge cases.
func TestLineParserEdgeCases(t *testing.T) {
	parser := NewLineParser(2)

	tests := []struct {
		name     string
		content  string
		check    func(*testing.T, LineParserResult)
	}{
		{
			name:    "empty content",
			content: "",
			check: func(t *testing.T, result LineParserResult) {
				if result.TotalLines != 1 {
					t.Errorf("Expected 1 line (empty), got %d", result.TotalLines)
				}
			},
		},
		{
			name:    "only whitespace",
			content: "   \n\t\n  ",
			check: func(t *testing.T, result LineParserResult) {
				if result.EmptyLines != 3 {
					t.Errorf("Expected 3 empty lines, got %d", result.EmptyLines)
				}
			},
		},
		{
			name:    "only comments",
			content: "# comment 1\n# comment 2\n# comment 3",
			check: func(t *testing.T, result LineParserResult) {
				if result.CommentLines != 3 {
					t.Errorf("Expected 3 comment lines, got %d", result.CommentLines)
				}
				if result.KeyCandidates != 0 {
					t.Errorf("Expected 0 key candidates, got %d", result.KeyCandidates)
				}
			},
		},
		{
			name:    "deeply nested",
			content: "k1:\n  k2:\n    k3:\n      k4:\n        k5: value",
			check: func(t *testing.T, result LineParserResult) {
				if result.MaxIndentLevel != 4 {
					t.Errorf("Expected max indent level 4, got %d", result.MaxIndentLevel)
				}
				if result.KeyCandidates != 5 {
					t.Errorf("Expected 5 key candidates, got %d", result.KeyCandidates)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.content)
			tt.check(t, result)
		})
	}
}

// BenchmarkSimpleParsing benchmarks parsing of simple YAML.
func BenchmarkSimpleParsing(b *testing.B) {
	parser := NewLineParser(2)
	yamlContent := `
key1: value1
key2: value2
key3: value3
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Parse(strings.TrimSpace(yamlContent))
	}
}

// BenchmarkComplexParsing benchmarks parsing of complex YAML.
func BenchmarkComplexParsing(b *testing.B) {
	parser := NewLineParser(2)
	yamlContent := `
service:
  name: my-service
  port: 8080
  database:
    host: localhost
    port: 5432
    pool:
      max_connections: 10
monitoring:
  enabled: true
  metrics:
    - name: request_count
      type: counter
    - name: response_time
      type: histogram
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Parse(strings.TrimSpace(yamlContent))
	}
}
