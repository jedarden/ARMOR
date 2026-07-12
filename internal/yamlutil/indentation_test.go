// Package yamlutil tests the YAML indentation parsing logic.
package yamlutil

import (
	"strings"
	"testing"
)

// TestCalculateIndentation tests the CalculateIndentation function.
func TestCalculateIndentation(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		spacesPerLevel int
		expected      IndentationInfo
	}{
		{
			name:          "no indentation",
			line:          "key: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      0,
				SpaceCount: 0,
				TabCount:   0,
				TotalWidth: 0,
				IndentType: "none",
				IsMixed:    false,
			},
		},
		{
			name:          "single level space indent",
			line:          "  key: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      1,
				SpaceCount: 2,
				TabCount:   0,
				TotalWidth: 2,
				IndentType: "space",
				IsMixed:    false,
			},
		},
		{
			name:          "double level space indent",
			line:          "    key: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      2,
				SpaceCount: 4,
				TabCount:   0,
				TotalWidth: 4,
				IndentType: "space",
				IsMixed:    false,
			},
		},
		{
			name:          "tab indent",
			line:          "\tkey: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      1,
				SpaceCount: 0,
				TabCount:   1,
				TotalWidth: 1,
				IndentType: "tab",
				IsMixed:    false,
			},
		},
		{
			name:          "multiple tab indent",
			line:          "\t\t\tkey: value",
			spacesPerLevel: 1,
			expected: IndentationInfo{
				Level:      3,
				SpaceCount: 0,
				TabCount:   3,
				TotalWidth: 3,
				IndentType: "tab",
				IsMixed:    false,
			},
		},
		{
			name:          "mixed indentation",
			line:          "  \tkey: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      0, // Mixed doesn't calculate level
				SpaceCount: 2,
				TabCount:   1,
				TotalWidth: 3,
				IndentType: "mixed",
				IsMixed:    true,
			},
		},
		{
			name:          "mixed indentation tabs then spaces",
			line:          "\t  key: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      0,
				SpaceCount: 2,
				TabCount:   1,
				TotalWidth: 3,
				IndentType: "mixed",
				IsMixed:    true,
			},
		},
		{
			name:          "partial indent not a full level",
			line:          "   key: value", // 3 spaces with 2 per level
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      1, // 3 / 2 = 1 (integer division)
				SpaceCount: 3,
				TabCount:   0,
				TotalWidth: 3,
				IndentType: "space",
				IsMixed:    false,
			},
		},
		{
			name:          "empty line",
			line:          "",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      0,
				SpaceCount: 0,
				TabCount:   0,
				TotalWidth: 0,
				IndentType: "none",
				IsMixed:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateIndentation(tt.line, tt.spacesPerLevel)

			if result.Level != tt.expected.Level {
				t.Errorf("Expected Level %d, got %d", tt.expected.Level, result.Level)
			}
			if result.SpaceCount != tt.expected.SpaceCount {
				t.Errorf("Expected SpaceCount %d, got %d", tt.expected.SpaceCount, result.SpaceCount)
			}
			if result.TabCount != tt.expected.TabCount {
				t.Errorf("Expected TabCount %d, got %d", tt.expected.TabCount, result.TabCount)
			}
			if result.TotalWidth != tt.expected.TotalWidth {
				t.Errorf("Expected TotalWidth %d, got %d", tt.expected.TotalWidth, result.TotalWidth)
			}
			if result.IndentType != tt.expected.IndentType {
				t.Errorf("Expected IndentType %s, got %s", tt.expected.IndentType, result.IndentType)
			}
			if result.IsMixed != tt.expected.IsMixed {
				t.Errorf("Expected IsMixed %v, got %v", tt.expected.IsMixed, result.IsMixed)
			}
		})
	}
}

// TestClassifyLineType tests the ClassifyLineType function.
func TestClassifyLineType(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected LineType
	}{
		{
			name:     "regular key-value line",
			line:     "key: value",
			expected: LineTypeMappingKey,
		},
		{
			name:     "regular sequence item",
			line:     "- item",
			expected: LineTypeSequenceItem,
		},
		{
			name:     "blank line empty",
			line:     "",
			expected: LineTypeBlank,
		},
		{
			name:     "blank line spaces only",
			line:     "   ",
			expected: LineTypeBlank,
		},
		{
			name:     "blank line tabs only",
			line:     "\t\t",
			expected: LineTypeBlank,
		},
		{
			name:     "blank line mixed whitespace",
			line:     "  \t  ",
			expected: LineTypeBlank,
		},
		{
			name:     "comment line",
			line:     "# this is a comment",
			expected: LineTypeComment,
		},
		{
			name:     "comment line with leading spaces",
			line:     "  # this is a comment",
			expected: LineTypeComment,
		},
		{
			name:     "comment line with leading tabs",
			line:     "\t# this is a comment",
			expected: LineTypeComment,
		},
		{
			name:     "document start marker",
			line:     "---",
			expected: LineTypeDocumentStart,
		},
		{
			name:     "document start marker with content",
			line:     "--- YAML",
			expected: LineTypeDocumentStart,
		},
		{
			name:     "document end marker",
			line:     "...",
			expected: LineTypeDocumentEnd,
		},
		{
			name:     "nested regular line",
			line:     "  nested: value",
			expected: LineTypeMappingKey,
		},
		{
			name:     "complex mapping",
			line:     "key: {nested: mapping}",
			expected: LineTypeMappingKey,
		},
		{
			name:     "complex sequence",
			line:     "- [item1, item2]",
			expected: LineTypeSequenceItem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyLineType(tt.line)
			if result != tt.expected {
				t.Errorf("Expected line type %v (%s), got %v (%s)",
					tt.expected, tt.expected, result, result)
			}
		})
	}
}

// TestLineTypeString tests the LineType String method.
func TestLineTypeString(t *testing.T) {
	tests := []struct {
		lineType LineType
		expected string
	}{
		{LineTypeMappingKey, "mapping key"},
		{LineTypeSequenceItem, "sequence item"},
		{LineTypeBlank, "blank line"},
		{LineTypeComment, "comment line"},
		{LineTypeDocumentStart, "document start marker"},
		{LineTypeDocumentEnd, "document end marker"},
		{LineType(999), "unknown content"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.lineType.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestIsBlankLine tests the IsBlankLine function.
func TestIsBlankLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"empty string", "", true},
		{"spaces only", "   ", true},
		{"tabs only", "\t\t", true},
		{"mixed whitespace", "  \t  ", true},
		{"single space", " ", true},
		{"single tab", "\t", true},
		{"content line", "key: value", false},
		{"comment line", "# comment", false},
		{"line with content after whitespace", "  key: value", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBlankLine(tt.line)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsCommentLine tests the IsCommentLine function.
func TestIsCommentLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"simple comment", "# comment", true},
		{"comment with leading spaces", "  # comment", true},
		{"comment with leading tabs", "\t# comment", true},
		{"comment with mixed leading whitespace", "  \t# comment", true},
		{"not comment - key value", "key: value", false},
		{"not comment - sequence", "- item", false},
		{"not comment - hash in value", "key: # not a comment", false},
		{"not comment - hash after text", "key# value", false},
		{"empty line", "", false},
		{"whitespace only", "   ", false},
		{"hash at start without space", "#comment", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCommentLine(tt.line)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsSequenceItem tests the IsSequenceItem function.
func TestIsSequenceItem(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"simple sequence item", "- item", true},
		{"sequence item with leading spaces", "  - item", true},
		{"sequence item with leading tabs", "\t- item", true},
		{"sequence item with tab after dash", "-\titem", true},
		{"not sequence - dash without space", "-item", false},
		{"not sequence - key value", "key: value", false},
		{"not sequence - comment", "# comment", false},
		{"not sequence - empty", "", false},
		{"not sequence - just dash", "-", false},
		{"complex sequence item", "- key: value", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSequenceItem(tt.line)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExtractLeadingWhitespace tests the ExtractLeadingWhitespace function.
func TestExtractLeadingWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{"no whitespace", "key: value", ""},
		{"spaces only", "  key: value", "  "},
		{"tabs only", "\t\tkey: value", "\t\t"},
		{"mixed whitespace", "  \t  key: value", "  \t  "},
		{"empty line", "", ""},
		{"whitespace only", "   ", "   "},
		{"tabs only content", "\t\t\t", "\t\t\t"},
		{"single space", " key", " "},
		{"single tab", "\tkey", "\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractLeadingWhitespace(tt.line)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestHasValidIndentation tests the HasValidIndentation function.
func TestHasValidIndentation(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"no indentation", "key: value", true},
		{"spaces only", "  key: value", true},
		{"more spaces", "    key: value", true},
		{"tabs only", "\tkey: value", true},
		{"multiple tabs", "\t\tkey: value", true},
		{"mixed spaces then tabs", "  \tkey: value", false},
		{"mixed tabs then spaces", "\t  key: value", false},
		{"empty line", "", true},
		{"whitespace only", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasValidIndentation(tt.line)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestNormalizeIndentation tests the NormalizeIndentation function.
func TestNormalizeIndentation(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		tabWidth  int
		expected  string
	}{
		{"no tabs", "  key: value", 2, "  key: value"},
		{"single tab", "\tkey: value", 2, "  key: value"},
		{"multiple tabs", "\t\tkey: value", 2, "    key: value"},
		{"tab width 4", "\tkey: value", 4, "    key: value"},
		{"tabs with spaces", "  \tkey: value", 2, "    key: value"},
		{"empty line", "", 2, ""},
		{"no indentation", "key: value", 2, "key: value"},
		{"spaces only", "  key: value", 2, "  key: value"},
		{"zero tab width", "\tkey: value", 0, "  key: value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeIndentation(tt.line, tt.tabWidth)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestDetectIndentStyle tests the DetectIndentStyle function.
func TestDetectIndentStyle(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{"no indent", "key: value", "none"},
		{"space indent", "  key: value", "space"},
		{"tab indent", "\tkey: value", "tab"},
		{"mixed indent", "  \tkey: value", "mixed"},
		{"mixed indent tabs then spaces", "\t  key: value", "mixed"},
		{"empty line", "", "none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectIndentStyle(tt.line)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestCountIndentationLevel tests the CountIndentationLevel function.
func TestCountIndentationLevel(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		spacesPerLevel int
		expected       int
	}{
		{"no indent", "key: value", 2, 0},
		{"one level", "  key: value", 2, 1},
		{"two levels", "    key: value", 2, 2},
		{"three levels", "      key: value", 2, 3},
		{"four spaces per level", "    key: value", 4, 1},
		{"partial indent", "   key: value", 2, 1}, // 3 / 2 = 1
		{"tab indent", "\tkey: value", 1, 1},
		{"multiple tabs", "\t\t\tkey: value", 1, 3},
		{"empty line", "", 2, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountIndentationLevel(tt.line, tt.spacesPerLevel)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestTrimLeadingWhitespace tests the TrimLeadingWhitespace function.
func TestTrimLeadingWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{"no whitespace", "key: value", "key: value"},
		{"spaces", "  key: value", "key: value"},
		{"tabs", "\t\tkey: value", "key: value"},
		{"mixed", "  \t  key: value", "key: value"},
		{"whitespace only", "   ", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimLeadingWhitespace(tt.line)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestIsPrintableWithoutContent tests the IsPrintableWithoutContent function.
func TestIsPrintableWithoutContent(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"regular key-value", "key: value", true},
		{"sequence item", "- item", true},
		{"nested content", "  nested: value", true},
		{"document start", "---", true},
		{"blank line", "", false},
		{"spaces only", "   ", false},
		{"tabs only", "\t\t", false},
		{"comment line", "# comment", false},
		{"comment with leading whitespace", "  # comment", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPrintableWithoutContent(tt.line)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestGetTrailingComment tests the GetTrailingComment function.
func TestGetTrailingComment(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{"simple trailing comment", "key: value # comment", " comment"},
		{"comment only", "# full line comment", " full line comment"},
		{"no comment", "key: value", ""},
		{"comment in single quotes", "key: 'value # not comment'", ""},
		{"comment in double quotes", `key: "value # not comment"`, ""},
		{"comment with leading spaces", "  key: value # comment", " comment"},
		{"escaped quote before hash", `key: "value\" # not comment"`, " # not comment"},
		{"single quote before hash", `key: 'value' # comment`, " comment"},
		{"empty line", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTrailingComment(tt.line)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestMeasureIndentWidth tests the MeasureIndentWidth function.
func TestMeasureIndentWidth(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		tabWidth    int
		expected    int
	}{
		{"no indent", "key: value", 4, 0},
		{"spaces only", "  key: value", 4, 2},
		{"single tab", "\tkey: value", 4, 4},
		{"multiple tabs", "\t\tkey: value", 4, 8},
		{"tab width 2", "\tkey: value", 2, 2},
		{"mixed spaces and tabs", "  \tkey: value", 4, 6}, // 2 spaces + 1 tab*4
		{"empty line", "", 4, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MeasureIndentWidth(tt.line, tt.tabWidth)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestIsValidIndentLevel tests the IsValidIndentLevel function.
func TestIsValidIndentLevel(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		spacesPerLevel int
		expected       bool
	}{
		{"no indent", "key: value", 2, true},
		{"valid 2-space indent", "  key: value", 2, true},
		{"valid 4-space indent", "    key: value", 2, true},
		{"invalid 3-space indent", "   key: value", 2, false},
		{"invalid 1-space indent", " key: value", 2, false},
		{"valid with 4-space level", "    key: value", 4, true},
		{"tab indent with space expectation", "\tkey: value", 2, false},
		{"mixed indent", "  \tkey: value", 2, false},
		{"zero spaces per level", "key: value", 0, true},
		{"negative spaces per level", "key: value", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidIndentLevel(tt.line, tt.spacesPerLevel)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestContainsOnlyASCIIWhitespace tests the ContainsOnlyASCIIWhitespace function.
func TestContainsOnlyASCIIWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"empty string", "", true},
		{"spaces only", "   ", true},
		{"tabs only", "\t\t", true},
		{"mixed spaces and tabs", "  \t  ", true},
		{"single space", " ", true},
		{"single tab", "\t", true},
		{"contains letter", "  a", false},
		{"contains number", "  1", false},
		{"contains newline", "  \n ", false},
		{"contains other unicode", "   ", false}, // Unicode space
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsOnlyASCIIWhitespace(tt.line)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestEstimateIndentFromContent tests the EstimateIndentFromContent function.
func TestEstimateIndentFromContent(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected int
	}{
		{
			name:     "empty lines",
			lines:    []string{},
			expected: 2, // Default
		},
		{
			name:     "no indentation",
			lines:    []string{"key1: value1", "key2: value2"},
			expected: 2, // Default
		},
		{
			name:     "2-space indent",
			lines:    []string{"key1: value1", "  key2: value2", "    key3: value3"},
			expected: 2,
		},
		{
			name:     "4-space indent",
			lines:    []string{"key1: value1", "    key2: value2", "        key3: value3"},
			expected: 4,
		},
		{
			name:     "tab indent",
			lines:    []string{"key1: value1", "\tkey2: value2", "\t\tkey3: value3"},
			expected: 0, // Tabs
		},
		{
			name:     "mixed 2 and 4 spaces",
			lines:    []string{"key1: value1", "  key2: value2", "    key3: value3"},
			expected: 2, // GCD of 2 and 4 is 2
		},
		{
			name:     "with comments and blanks",
			lines:    []string{"# comment", "", "key1: value1", "  # nested comment", "  key2: value2"},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateIndentFromContent(tt.lines)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestGetIndentSummary tests the GetIndentSummary function.
func TestGetIndentSummary(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		spacesPerLevel int
		expectedContains string
	}{
		{"no indent", "key: value", 2, "no indent"},
		{"space indent", "  key: value", 2, "space indent, level 1"},
		{"tab indent", "\tkey: value", 2, "tab indent, level 1"},
		{"mixed indent", "  \tkey: value", 2, "mixed indent (invalid)"},
		{"multiple tabs", "\t\tkey: value", 2, "2 tabs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetIndentSummary(tt.line, tt.spacesPerLevel)
			if !strings.Contains(result, tt.expectedContains) {
				t.Errorf("Expected summary to contain %q, got %q", tt.expectedContains, result)
			}
		})
	}
}

// TestScanLineTokens tests the ScanLineTokens function.
func TestScanLineTokens(t *testing.T) {
	tests := []struct {
		name string
		line string
		check func(*testing.T, map[string]interface{})
	}{
		{
			name: "blank line",
			line: "",
			check: func(t *testing.T, tokens map[string]interface{}) {
				if !tokens["is_blank"].(bool) {
					t.Error("Expected is_blank=true")
				}
			},
		},
		{
			name: "comment line",
			line: "# this is a comment",
			check: func(t *testing.T, tokens map[string]interface{}) {
				if tokens["is_comment"].(bool) != true {
					t.Error("Expected is_comment=true")
				}
				if tokens["comment"].(string) != " this is a comment" {
					t.Errorf("Expected comment ' this is a comment', got %q", tokens["comment"])
				}
			},
		},
		{
			name: "key-value pair",
			line: "key: value",
			check: func(t *testing.T, tokens map[string]interface{}) {
				if tokens["is_key_value"].(bool) != true {
					t.Error("Expected is_key_value=true")
				}
				if tokens["key"].(string) != "key" {
					t.Errorf("Expected key 'key', got %q", tokens["key"])
				}
				if tokens["value"].(string) != "value" {
					t.Errorf("Expected value 'value', got %q", tokens["value"])
				}
			},
		},
		{
			name: "indented key-value",
			line: "  key: value",
			check: func(t *testing.T, tokens map[string]interface{}) {
				indent := tokens["indent"].(map[string]int)
				if indent["spaces"] != 2 {
					t.Errorf("Expected 2 spaces, got %d", indent["spaces"])
				}
				if tokens["is_key_value"].(bool) != true {
					t.Error("Expected is_key_value=true")
				}
			},
		},
		{
			name: "sequence item",
			line: "- item",
			check: func(t *testing.T, tokens map[string]interface{}) {
				if tokens["is_sequence_item"].(bool) != true {
					t.Error("Expected is_sequence_item=true")
				}
			},
		},
		{
			name: "key-value with trailing comment",
			line: "key: value # comment",
			check: func(t *testing.T, tokens map[string]interface{}) {
				if tokens["has_trailing_comment"].(bool) != true {
					t.Error("Expected has_trailing_comment=true")
				}
				if tokens["trailing_comment"].(string) != " comment" {
					t.Errorf("Expected trailing comment ' comment', got %q", tokens["trailing_comment"])
				}
			},
		},
		{
			name: "document start",
			line: "---",
			check: func(t *testing.T, tokens map[string]interface{}) {
				if tokens["is_document_start"].(bool) != true {
					t.Error("Expected is_document_start=true")
				}
			},
		},
		{
			name: "document end",
			line: "...",
			check: func(t *testing.T, tokens map[string]interface{}) {
				if tokens["is_document_end"].(bool) != true {
					t.Error("Expected is_document_end=true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := ScanLineTokens(tt.line)
			if tt.check != nil {
				tt.check(t, tokens)
			}
		})
	}
}

// TestIndentationEdgeCases tests edge cases in indentation parsing.
func TestIndentationEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		testFunc func(*testing.T, string)
	}{
		{
			name: "very long space indentation",
			line: strings.Repeat(" ", 100) + "key: value",
			testFunc: func(t *testing.T, line string) {
				info := CalculateIndentation(line, 2)
				if info.SpaceCount != 100 {
					t.Errorf("Expected 100 spaces, got %d", info.SpaceCount)
				}
				if info.Level != 50 {
					t.Errorf("Expected level 50, got %d", info.Level)
				}
			},
		},
		{
			name: "alternating spaces and tabs",
			line: " \t \t \t key: value",
			testFunc: func(t *testing.T, line string) {
				info := CalculateIndentation(line, 2)
				if !info.IsMixed {
					t.Error("Expected IsMixed=true for alternating whitespace")
				}
			},
		},
		{
			name: "unicode whitespace after indent",
			line: "   key: value", // em space after regular spaces
			testFunc: func(t *testing.T, line string) {
				info := CalculateIndentation(line, 2)
				// Should count only regular spaces
				if info.SpaceCount != 2 {
					t.Errorf("Expected 2 spaces, got %d", info.SpaceCount)
				}
			},
		},
		{
			name: "line with only whitespace",
			line: "  \t  ",
			testFunc: func(t *testing.T, line string) {
				if !IsBlankLine(line) {
					t.Error("Expected line to be classified as blank")
				}
				info := CalculateIndentation(line, 2)
				if !info.IsMixed {
					t.Error("Expected mixed indentation for whitespace-only line")
				}
			},
		},
		{
			name: "zero spaces per level",
			line: "  key: value",
			testFunc: func(t *testing.T, line string) {
				info := CalculateIndentation(line, 0)
				// Level should be 0 when spacesPerLevel is 0
				if info.Level != 0 {
					t.Errorf("Expected level 0 with spacesPerLevel=0, got %d", info.Level)
				}
				// But SpaceCount should still be counted
				if info.SpaceCount != 2 {
					t.Errorf("Expected 2 spaces, got %d", info.SpaceCount)
				}
			},
		},
		{
			name: "negative spaces per level",
			line: "  key: value",
			testFunc: func(t *testing.T, line string) {
				info := CalculateIndentation(line, -2)
				// Level should be 0 for negative spacesPerLevel
				if info.Level != 0 {
					t.Errorf("Expected level 0 with negative spacesPerLevel, got %d", info.Level)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t, tt.line)
		})
	}
}

// TestIndentationWithRealYAML tests indentation parsing with real YAML examples.
func TestIndentationWithRealYAML(t *testing.T) {
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

	lines := strings.Split(yamlContent, "\n")

	// Test that we can correctly identify indentation levels
	expectedIndents := map[int]int{ // line number -> expected indent level
		1: 0,  // # Service configuration
		2: 0,  // service:
		3: 1,  //   name: my-service
		4: 1,  //   port: 8080
		5: 1,  //   # Database settings
		6: 1,  //   database:
		7: 2,  //     host: localhost
		8: 2,  //     port: 5432
		9: 2,  //     # Connection pool
		10: 2, //     pool:
		11: 3, //       max_connections: 10
		12: 3, //       min_connections: 2
		13: 0, // (blank)
		14: 0, // # Monitoring configuration
		15: 0, // monitoring:
		16: 1, //   enabled: true
		17: 1, //   metrics:
		18: 2, //     - name: request_count
		19: 2, //     type: counter
		20: 2, //     - name: response_time
		21: 2, //     type: histogram
	}

	for lineNum, expectedLevel := range expectedIndents {
		// Adjust for 0-indexing
		line := lines[lineNum-1]
		actualLevel := CountIndentationLevel(line, 2)

		if actualLevel != expectedLevel {
			t.Errorf("Line %d: expected indent level %d, got %d (line: %q)",
				lineNum, expectedLevel, actualLevel, line)
		}
	}

	// Test line type classification
	commentLines := []int{1, 5, 9, 14}
	blankLines := []int{13}
	mappingKeyLines := []int{2, 3, 4, 6, 7, 8, 10, 11, 12, 15, 16, 17, 19, 21}
	sequenceLines := []int{18, 20}

	for _, lineNum := range commentLines {
		line := lines[lineNum-1]
		if ClassifyLineType(line) != LineTypeComment {
			t.Errorf("Line %d should be a comment: %q", lineNum, line)
		}
	}

	for _, lineNum := range blankLines {
		line := lines[lineNum-1]
		if ClassifyLineType(line) != LineTypeBlank {
			t.Errorf("Line %d should be blank: %q", lineNum, line)
		}
	}

	for _, lineNum := range mappingKeyLines {
		line := lines[lineNum-1]
		lineType := ClassifyLineType(line)
		if lineType != LineTypeMappingKey && lineType != LineTypeSequenceItem {
			t.Errorf("Line %d should be mapping key or sequence item, got %v: %q", lineNum, lineType, line)
		}
	}

	for _, lineNum := range sequenceLines {
		line := lines[lineNum-1]
		if ClassifyLineType(line) != LineTypeSequenceItem {
			t.Errorf("Line %d should be sequence item: %q", lineNum, line)
		}
		if !IsSequenceItem(line) {
			t.Errorf("Line %d should be detected as sequence item: %q", lineNum, line)
		}
	}
}

// BenchmarkCalculateIndentation benchmarks the CalculateIndentation function.
func BenchmarkCalculateIndentation(b *testing.B) {
	line := "    key: value"
	for i := 0; i < b.N; i++ {
		CalculateIndentation(line, 2)
	}
}

// BenchmarkClassifyLineType benchmarks the ClassifyLineType function.
func BenchmarkClassifyLineType(b *testing.B) {
	line := "  key: value"
	for i := 0; i < b.N; i++ {
		ClassifyLineType(line)
	}
}

// TestCalculateIndentationEdgeCases tests edge cases for CalculateIndentation.
func TestCalculateIndentationEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		spacesPerLevel int
		expected      IndentationInfo
	}{
		{
			name:          "line with only hash symbol",
			line:          "#",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      0,
				SpaceCount: 0,
				TabCount:   0,
				TotalWidth: 0,
				IndentType: "none",
				IsMixed:    false,
			},
		},
		{
			name:          "line with only hash symbol with space indent",
			line:          "  #",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      1,
				SpaceCount: 2,
				TabCount:   0,
				TotalWidth: 2,
				IndentType: "space",
				IsMixed:    false,
			},
		},
		{
			name:          "line with only hash symbol with tab indent",
			line:          "\t#",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      1,
				SpaceCount: 0,
				TabCount:   1,
				TotalWidth: 1,
				IndentType: "tab",
				IsMixed:    false,
			},
		},
		{
			name:          "8 space indentation",
			line:          "        key: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      4,
				SpaceCount: 8,
				TabCount:   0,
				TotalWidth: 8,
				IndentType: "space",
				IsMixed:    false,
			},
		},
		{
			name:          "8 space indentation with 4 space levels",
			line:          "        key: value",
			spacesPerLevel: 4,
			expected: IndentationInfo{
				Level:      2,
				SpaceCount: 8,
				TabCount:   0,
				TotalWidth: 8,
				IndentType: "space",
				IsMixed:    false,
			},
		},
		{
			name:          "6 space indentation with 2 space levels",
			line:          "      key: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      3,
				SpaceCount: 6,
				TabCount:   0,
				TotalWidth: 6,
				IndentType: "space",
				IsMixed:    false,
			},
		},
		{
			name:          "odd space count - 1 space",
			line:          " key: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      0,
				SpaceCount: 1,
				TabCount:   0,
				TotalWidth: 1,
				IndentType: "space",
				IsMixed:    false,
			},
		},
		{
			name:          "odd space count - 5 spaces",
			line:          "     key: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      2,
				SpaceCount: 5,
				TabCount:   0,
				TotalWidth: 5,
				IndentType: "space",
				IsMixed:    false,
			},
		},
		{
			name:          "odd space count - 7 spaces",
			line:          "       key: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      3,
				SpaceCount: 7,
				TabCount:   0,
				TotalWidth: 7,
				IndentType: "space",
				IsMixed:    false,
			},
		},
		{
			name:          "single space with tab",
			line:          " \tkey: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      0,
				SpaceCount: 1,
				TabCount:   1,
				TotalWidth: 2,
				IndentType: "mixed",
				IsMixed:    true,
			},
		},
		{
			name:          "tab with single space",
			line:          "\t key: value",
			spacesPerLevel: 2,
			expected: IndentationInfo{
				Level:      0,
				SpaceCount: 1,
				TabCount:   1,
				TotalWidth: 2,
				IndentType: "mixed",
				IsMixed:    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateIndentation(tt.line, tt.spacesPerLevel)

			if result.Level != tt.expected.Level {
				t.Errorf("Expected Level %d, got %d", tt.expected.Level, result.Level)
			}
			if result.SpaceCount != tt.expected.SpaceCount {
				t.Errorf("Expected SpaceCount %d, got %d", tt.expected.SpaceCount, result.SpaceCount)
			}
			if result.TabCount != tt.expected.TabCount {
				t.Errorf("Expected TabCount %d, got %d", tt.expected.TabCount, result.TabCount)
			}
			if result.TotalWidth != tt.expected.TotalWidth {
				t.Errorf("Expected TotalWidth %d, got %d", tt.expected.TotalWidth, result.TotalWidth)
			}
			if result.IndentType != tt.expected.IndentType {
				t.Errorf("Expected IndentType %s, got %s", tt.expected.IndentType, result.IndentType)
			}
			if result.IsMixed != tt.expected.IsMixed {
				t.Errorf("Expected IsMixed %v, got %v", tt.expected.IsMixed, result.IsMixed)
			}
		})
	}
}

// TestClassifyLineTypeEdgeCases tests edge cases for ClassifyLineType.
func TestClassifyLineTypeEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected LineType
	}{
		{
			name:     "line with only hash symbol",
			line:     "#",
			expected: LineTypeComment,
		},
		{
			name:     "line with only hash symbol with space indent",
			line:     "  #",
			expected: LineTypeComment,
		},
		{
			name:     "line with only hash symbol with tab indent",
			line:     "\t#",
			expected: LineTypeComment,
		},
		{
			name:     "line with multiple hash symbols",
			line:     "###",
			expected: LineTypeComment,
		},
		{
			name:     "line with hash and space but no text",
			line:     "# ",
			expected: LineTypeComment,
		},
		{
			name:     "line with indented hash and space but no text",
			line:     "  # ",
			expected: LineTypeComment,
		},
		{
			name:     "empty string",
			line:     "",
			expected: LineTypeBlank,
		},
		{
			name:     "single space",
			line:     " ",
			expected: LineTypeBlank,
		},
		{
			name:     "single tab",
			line:     "\t",
			expected: LineTypeBlank,
		},
		{
			name:     "multiple spaces",
			line:     "    ",
			expected: LineTypeBlank,
		},
		{
			name:     "multiple tabs",
			line:     "\t\t\t",
			expected: LineTypeBlank,
		},
		{
			name:     "mixed whitespace",
			line:     "  \t  ",
			expected: LineTypeBlank,
		},
		{
			name:     "content line with deep indent",
			line:     "          key: value",
			expected: LineTypeMappingKey,
		},
		{
			name:     "sequence item with deep indent",
			line:     "          - item",
			expected: LineTypeSequenceItem,
		},
		{
			name:     "comment with deep indent",
			line:     "          # comment",
			expected: LineTypeComment,
		},
		{
			name:     "document start with indent",
			line:     "  ---",
			expected: LineTypeDocumentStart,
		},
		{
			name:     "document end with indent",
			line:     "  ...",
			expected: LineTypeDocumentEnd,
		},
		{
			name:     "hash followed by non-space colon",
			line:     "#key: value",
			expected: LineTypeComment,
		},
		{
			name:     "hash in middle of text",
			line:     "key#value",
			expected: LineTypeUnknown, // No colon, doesn't match any pattern
		},
		{
			name:     "hash at end of key",
			line:     "key#: value",
			expected: LineTypeMappingKey, // Contains colon, classified as mapping key
		},
		{
			name:     "flow mapping with colon",
			line:     "{key: value}",
			expected: LineTypeMappingKey, // Contains colon, classified as mapping key
		},
		{
			name:     "indented flow mapping",
			line:     "  {key: value}",
			expected: LineTypeMappingKey, // Contains colon, classified as mapping key
		},
		{
			name:     "flow sequence without colon",
			line:     "[item1, item2]",
			expected: LineTypeUnknown, // No colon, doesn't match any pattern
		},
		{
			name:     "indented flow sequence",
			line:     "  [item1, item2]",
			expected: LineTypeUnknown, // No colon, doesn't match any pattern
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyLineType(tt.line)
			if result != tt.expected {
				t.Errorf("Expected line type %v (%s), got %v (%s)",
					tt.expected, tt.expected, result, result)
			}
		})
	}
}

// TestIndentationComprehensive provides comprehensive coverage of indentation scenarios.
func TestIndentationComprehensive(t *testing.T) {
	// Test various indentation levels with 2-space convention
	t.Run("2-space convention", func(t *testing.T) {
		testCases := []struct {
			line     string
			expected struct {
				level  int
				spaces int
				indentType string
			}
		}{
			{"key: value", struct{ level int; spaces int; indentType string }{0, 0, "none"}},
			{"  key: value", struct{ level int; spaces int; indentType string }{1, 2, "space"}},
			{"    key: value", struct{ level int; spaces int; indentType string }{2, 4, "space"}},
			{"      key: value", struct{ level int; spaces int; indentType string }{3, 6, "space"}},
			{"        key: value", struct{ level int; spaces int; indentType string }{4, 8, "space"}},
			{"          key: value", struct{ level int; spaces int; indentType string }{5, 10, "space"}},
		}

		for _, tc := range testCases {
			info := CalculateIndentation(tc.line, 2)
			if info.Level != tc.expected.level {
				t.Errorf("Line %q: expected level %d, got %d", tc.line, tc.expected.level, info.Level)
			}
			if info.SpaceCount != tc.expected.spaces {
				t.Errorf("Line %q: expected %d spaces, got %d", tc.line, tc.expected.spaces, info.SpaceCount)
			}
			if info.IndentType != tc.expected.indentType {
				t.Errorf("Line %q: expected indent type %s, got %s", tc.line, tc.expected.indentType, info.IndentType)
			}
		}
	})

	// Test various indentation levels with 4-space convention
	t.Run("4-space convention", func(t *testing.T) {
		testCases := []struct {
			line     string
			expected struct {
				level  int
				spaces int
				indentType string
			}
		}{
			{"key: value", struct{ level int; spaces int; indentType string }{0, 0, "none"}},
			{"    key: value", struct{ level int; spaces int; indentType string }{1, 4, "space"}},
			{"        key: value", struct{ level int; spaces int; indentType string }{2, 8, "space"}},
			{"            key: value", struct{ level int; spaces int; indentType string }{3, 12, "space"}},
		}

		for _, tc := range testCases {
			info := CalculateIndentation(tc.line, 4)
			if info.Level != tc.expected.level {
				t.Errorf("Line %q: expected level %d, got %d", tc.line, tc.expected.level, info.Level)
			}
			if info.SpaceCount != tc.expected.spaces {
				t.Errorf("Line %q: expected %d spaces, got %d", tc.line, tc.expected.spaces, info.SpaceCount)
			}
			if info.IndentType != tc.expected.indentType {
				t.Errorf("Line %q: expected indent type %s, got %s", tc.line, tc.expected.indentType, info.IndentType)
			}
		}
	})

	// Test tab-based indentation
	t.Run("tab-based indentation", func(t *testing.T) {
		testCases := []struct {
			line     string
			expected struct {
				level    int
				tabs     int
				indentType string
			}
		}{
			{"key: value", struct{ level int; tabs int; indentType string }{0, 0, "none"}},
			{"\tkey: value", struct{ level int; tabs int; indentType string }{1, 1, "tab"}},
			{"\t\tkey: value", struct{ level int; tabs int; indentType string }{2, 2, "tab"}},
			{"\t\t\tkey: value", struct{ level int; tabs int; indentType string }{3, 3, "tab"}},
		}

		for _, tc := range testCases {
			info := CalculateIndentation(tc.line, 1)
			if info.Level != tc.expected.level {
				t.Errorf("Line %q: expected level %d, got %d", tc.line, tc.expected.level, info.Level)
			}
			if info.TabCount != tc.expected.tabs {
				t.Errorf("Line %q: expected %d tabs, got %d", tc.line, tc.expected.tabs, info.TabCount)
			}
			if info.IndentType != tc.expected.indentType {
				t.Errorf("Line %q: expected indent type %s, got %s", tc.line, tc.expected.indentType, info.IndentType)
			}
		}
	})

	// Test mixed indentation (invalid)
	t.Run("mixed indentation", func(t *testing.T) {
		testCases := []string{
			"  \tkey: value",
			"\t  key: value",
			" \t key: value",
			"\t \tkey: value",
		}

		for _, line := range testCases {
			info := CalculateIndentation(line, 2)
			if !info.IsMixed {
				t.Errorf("Line %q: expected mixed indentation, got %s", line, info.IndentType)
			}
			if info.IndentType != "mixed" {
				t.Errorf("Line %q: expected indent type 'mixed', got %s", line, info.IndentType)
			}
		}
	})
}

// TestIsCommentLineEdgeCases tests edge cases for IsCommentLine.
func TestIsCommentLineEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"only hash", "#", true},
		{"hash with trailing space", "# ", true},
		{"multiple hashes", "###", true},
		{"hash with numbers", "#123", true},
		{"indented only hash", "  #", true},
		{"tab indented only hash", "\t#", true},
		{"hash not at start after whitespace", "  # key: value", true},
		{"hash in middle", "key#value", false},
		{"hash at end of key", "key#", false},
		{"hash in value", "key: value#test", false},
		{"hash after colon", "key:# value", false},
		{"empty line", "", false},
		{"whitespace only", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCommentLine(tt.line)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for line %q", tt.expected, result, tt.line)
			}
		})
	}
}

// TestIndentationWithVariousWhitespaceCombinations tests complex whitespace scenarios.
func TestIndentationWithVariousWhitespaceCombinations(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		spacesPerLevel int
		check         func(t *testing.T, info IndentationInfo)
	}{
		{
			name:          "space-tab-space pattern",
			line:          " \t key: value",
			spacesPerLevel: 2,
			check: func(t *testing.T, info IndentationInfo) {
				if !info.IsMixed {
					t.Error("Expected mixed indentation")
				}
				if info.SpaceCount != 2 {
					t.Errorf("Expected 2 spaces, got %d", info.SpaceCount)
				}
				if info.TabCount != 1 {
					t.Errorf("Expected 1 tab, got %d", info.TabCount)
				}
			},
		},
		{
			name:          "tab-space-tab pattern",
			line:          "\t \tkey: value",
			spacesPerLevel: 2,
			check: func(t *testing.T, info IndentationInfo) {
				if !info.IsMixed {
					t.Error("Expected mixed indentation")
				}
				if info.SpaceCount != 1 {
					t.Errorf("Expected 1 space, got %d", info.SpaceCount)
				}
				if info.TabCount != 2 {
					t.Errorf("Expected 2 tabs, got %d", info.TabCount)
				}
			},
		},
		{
			name:          "alternating space-tab-space-tab",
			line:          " \t \tkey: value",
			spacesPerLevel: 2,
			check: func(t *testing.T, info IndentationInfo) {
				if !info.IsMixed {
					t.Error("Expected mixed indentation")
				}
				if info.SpaceCount != 2 {
					t.Errorf("Expected 2 spaces, got %d", info.SpaceCount)
				}
				if info.TabCount != 2 {
					t.Errorf("Expected 2 tabs, got %d", info.TabCount)
				}
			},
		},
		{
			name:          "many spaces then tab",
			line:          "        \tkey: value",
			spacesPerLevel: 2,
			check: func(t *testing.T, info IndentationInfo) {
				if !info.IsMixed {
					t.Error("Expected mixed indentation")
				}
				if info.SpaceCount != 8 {
					t.Errorf("Expected 8 spaces, got %d", info.SpaceCount)
				}
				if info.TabCount != 1 {
					t.Errorf("Expected 1 tab, got %d", info.TabCount)
				}
			},
		},
		{
			name:          "tab then many spaces",
			line:          "\t        key: value",
			spacesPerLevel: 2,
			check: func(t *testing.T, info IndentationInfo) {
				if !info.IsMixed {
					t.Error("Expected mixed indentation")
				}
				if info.SpaceCount != 8 {
					t.Errorf("Expected 8 spaces, got %d", info.SpaceCount)
				}
				if info.TabCount != 1 {
					t.Errorf("Expected 1 tab, got %d", info.TabCount)
				}
			},
		},
		{
			name:          "pure spaces at various levels",
			line:          "                key: value", // 16 spaces
			spacesPerLevel: 2,
			check: func(t *testing.T, info IndentationInfo) {
				if info.IsMixed {
					t.Error("Expected non-mixed indentation")
				}
				if info.SpaceCount != 16 {
					t.Errorf("Expected 16 spaces, got %d", info.SpaceCount)
				}
				if info.Level != 8 {
					t.Errorf("Expected level 8, got %d", info.Level)
				}
			},
		},
		{
			name:          "pure tabs at various levels",
			line:          "\t\t\t\t\t\t\t\tkey: value", // 8 tabs
			spacesPerLevel: 1,
			check: func(t *testing.T, info IndentationInfo) {
				if info.IsMixed {
					t.Error("Expected non-mixed indentation")
				}
				if info.TabCount != 8 {
					t.Errorf("Expected 8 tabs, got %d", info.TabCount)
				}
				if info.Level != 8 {
					t.Errorf("Expected level 8, got %d", info.Level)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := CalculateIndentation(tt.line, tt.spacesPerLevel)
			if tt.check != nil {
				tt.check(t, info)
			}
		})
	}
}
