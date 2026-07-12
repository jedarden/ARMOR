// Package yamlutil tests the key indentation validation functions.
package yamlutil

import (
	"strings"
	"testing"
)

// TestIndentationContextCreation tests the creation and initialization of IndentationContext.
func TestIndentationContextCreation(t *testing.T) {
	tests := []struct {
		name            string
		spacesPerLevel  int
		expectedSpaces  int
	}{
		{
			name:           "default 2 spaces",
			spacesPerLevel: 0, // Should default to 2
			expectedSpaces: 2,
		},
		{
			name:           "explicit 2 spaces",
			spacesPerLevel: 2,
			expectedSpaces: 2,
		},
		{
			name:           "4 spaces per level",
			spacesPerLevel: 4,
			expectedSpaces: 4,
		},
		{
			name:           "negative defaults to 2",
			spacesPerLevel: -1,
			expectedSpaces: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewIndentationContext(tt.spacesPerLevel)
			if ctx.spacesPerLevel != tt.expectedSpaces {
				t.Errorf("NewIndentationContext(%d).spacesPerLevel = %d, want %d",
					tt.spacesPerLevel, ctx.spacesPerLevel, tt.expectedSpaces)
			}
			if ctx.lastLevel != -1 {
				t.Errorf("NewIndentationContext(%d).lastLevel = %d, want -1",
					tt.spacesPerLevel, ctx.lastLevel)
			}
			if ctx.seenKeys {
				t.Errorf("NewIndentationContext(%d).seenKeys = true, want false",
					tt.spacesPerLevel)
			}
		})
	}
}

// TestValidateMappingKeyIndentTopLevel tests validation of top-level keys.
func TestValidateMappingKeyIndentTopLevel(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		isMappingKey  bool
		expectedValid bool
	}{
		{
			name:          "top-level key with no indent",
			line:          "name: John",
			isMappingKey:  true,
			expectedValid: true,
		},
		{
			name:          "top-level key with spaces in key",
			line:          "first name: John",
			isMappingKey:  true,
			expectedValid: true,
		},
		{
			name:          "top-level numeric key",
			line:          "123: value",
			isMappingKey:  true,
			expectedValid: true,
		},
		{
			name:          "empty line should be skipped",
			line:          "",
			isMappingKey:  false,
			expectedValid: true,
		},
		{
			name:          "whitespace-only line should be skipped",
			line:          "    ",
			isMappingKey:  false,
			expectedValid: true,
		},
		{
			name:          "comment line should be skipped",
			line:          "# This is a comment",
			isMappingKey:  false,
			expectedValid: true,
		},
		{
			name:          "indented comment should be skipped",
			line:          "  # indented comment",
			isMappingKey:  false,
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewIndentationContext(2)
			result := ctx.ValidateMappingKeyIndent(tt.line, tt.isMappingKey)
			if result != tt.expectedValid {
				t.Errorf("ValidateMappingKeyIndent(%q, %v) = %v, want %v",
					tt.line, tt.isMappingKey, result, tt.expectedValid)
			}
		})
	}
}

// TestValidateMappingKeyIndentNested tests validation of nested keys.
func TestValidateMappingKeyIndentNested(t *testing.T) {
	tests := []struct {
		name          string
		lines         []string
		expectedValid bool
	}{
		{
			name: "valid parent-child relationship (2-space indent)",
			lines: []string{
				"parent: value",
				"  child: value",
			},
			expectedValid: true,
		},
		{
			name: "valid parent-child relationship (4-space indent)",
			lines: []string{
				"parent: value",
				"    child: value",
			},
			expectedValid: false, // Context uses 2-space indent, 4-space would be level 2
		},
		{
			name: "valid multi-level nesting",
			lines: []string{
				"level1: value",
				"  level2: value",
				"    level3: value",
				"      level4: value",
			},
			expectedValid: true,
		},
		{
			name: "valid siblings at same level",
			lines: []string{
				"  sibling1: value",
				"  sibling2: value",
				"  sibling3: value",
			},
			expectedValid: true,
		},
		{
			name: "valid return to parent level",
			lines: []string{
				"level1: value",
				"  level2: value",
				"level1_sibling: value",
			},
			expectedValid: true,
		},
		{
			name: "invalid - indentation too deep (skip level)",
			lines: []string{
				"level1: value",
				"      level3: value", // Skips level 2
			},
			expectedValid: false,
		},
		{
			name: "invalid - inconsistent indentation",
			lines: []string{
				"level1: value",
				" level2: value", // 1 space instead of 2
			},
			expectedValid: false, // Should fail because 1 space is not a multiple of 2
		},
		{
			name: "valid deep nesting then return to top",
			lines: []string{
				"level1: value",
				"  level2: value",
				"    level3: value",
				"level1_again: value",
			},
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewIndentationContext(2)
			for _, line := range tt.lines {
				isKey := IsMappingKey(line) && !IsCommentLine(line) && !IsBlankLine(line)
				if !ctx.ValidateMappingKeyIndent(line, isKey) {
					if tt.expectedValid {
						t.Errorf("Validation failed for line: %q", line)
					}
					return // Early exit on first failure
				}
			}
			if !tt.expectedValid {
				t.Errorf("Validation passed but should have failed")
			}
		})
	}
}

// TestValidateMappingKeyIndentMixedIndentation tests handling of mixed tabs and spaces.
func TestValidateMappingKeyIndentMixedIndentation(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		isMappingKey  bool
		expectedValid bool
	}{
		{
			name:          "mixed tabs and spaces at start",
			line:          "  \tkey: value",
			isMappingKey:  true,
			expectedValid: false,
		},
		{
			name:          "mixed spaces and tabs (tabs then spaces)",
			line:          "\t  key: value",
			isMappingKey:  true,
			expectedValid: false,
		},
		{
			name:          "spaces only (valid)",
			line:          "  key: value",
			isMappingKey:  true,
			expectedValid: true,
		},
		{
			name:          "tabs only (valid)",
			line:          "\tkey: value",
			isMappingKey:  true,
			expectedValid: true,
		},
		{
			name:          "no indentation (valid)",
			line:          "key: value",
			isMappingKey:  true,
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewIndentationContext(2)
			result := ctx.ValidateMappingKeyIndent(tt.line, tt.isMappingKey)
			if result != tt.expectedValid {
				t.Errorf("ValidateMappingKeyIndent(%q, %v) = %v, want %v",
					tt.line, tt.isMappingKey, result, tt.expectedValid)
			}
		})
	}
}

// TestIndentationContextReset tests the Reset functionality.
func TestIndentationContextReset(t *testing.T) {
	ctx := NewIndentationContext(2)

	// Add some state
	ctx.ValidateMappingKeyIndent("level1: value", true)
	ctx.ValidateMappingKeyIndent("  level2: value", true)
	ctx.ValidateMappingKeyIndent("    level3: value", true)

	// Verify state is set
	if ctx.lastLevel != 2 {
		t.Errorf("Before reset: lastLevel = %d, want 2", ctx.lastLevel)
	}
	if len(ctx.parentLevels) != 2 {
		t.Errorf("Before reset: parentLevels length = %d, want 2", len(ctx.parentLevels))
	}

	// Reset
	ctx.Reset()

	// Verify state is cleared
	if ctx.lastLevel != -1 {
		t.Errorf("After reset: lastLevel = %d, want -1", ctx.lastLevel)
	}
	if len(ctx.parentLevels) != 0 {
		t.Errorf("After reset: parentLevels length = %d, want 0", len(ctx.parentLevels))
	}
	if ctx.seenKeys {
		t.Errorf("After reset: seenKeys = true, want false")
	}
}

// TestIndentationContextGetCurrentLevel tests GetCurrentLevel.
func TestIndentationContextGetCurrentLevel(t *testing.T) {
	ctx := NewIndentationContext(2)

	// Before any keys
	if level := ctx.GetCurrentLevel(); level != -1 {
		t.Errorf("GetCurrentLevel() = %d, want -1 (before keys)", level)
	}

	// After first key
	ctx.ValidateMappingKeyIndent("key: value", true)
	if level := ctx.GetCurrentLevel(); level != 0 {
		t.Errorf("GetCurrentLevel() = %d, want 0 (after first key)", level)
	}

	// After nested key
	ctx.ValidateMappingKeyIndent("  nested: value", true)
	if level := ctx.GetCurrentLevel(); level != 1 {
		t.Errorf("GetCurrentLevel() = %d, want 1 (after nested key)", level)
	}
}

// TestIndentationContextGetParentLevels tests GetParentLevels.
func TestIndentationContextGetParentLevels(t *testing.T) {
	ctx := NewIndentationContext(2)

	// Initially empty
	parents := ctx.GetParentLevels()
	if len(parents) != 0 {
		t.Errorf("GetParentLevels() length = %d, want 0 (initially)", len(parents))
	}

	// After nesting
	ctx.ValidateMappingKeyIndent("level1: value", true)
	ctx.ValidateMappingKeyIndent("  level2: value", true)
	ctx.ValidateMappingKeyIndent("    level3: value", true)

	parents = ctx.GetParentLevels()
	if len(parents) != 2 {
		t.Errorf("GetParentLevels() length = %d, want 2", len(parents))
	}
	if parents[0] != 0 {
		t.Errorf("GetParentLevels()[0] = %d, want 0", parents[0])
	}
	if parents[1] != 1 {
		t.Errorf("GetParentLevels()[1] = %d, want 1", parents[1])
	}

	// Verify it's a copy, not the original
	parents[0] = 999
	originalParents := ctx.GetParentLevels()
	if originalParents[0] == 999 {
		t.Errorf("GetParentLevels() returned mutable array, should return copy")
	}
}

// TestGetIndentationLevel tests GetIndentationLevel function.
func TestGetIndentationLevel(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected int
	}{
		{
			name:     "no indentation",
			line:     "key: value",
			expected: 0,
		},
		{
			name:     "2-space indent",
			line:     "  key: value",
			expected: 1,
		},
		{
			name:     "4-space indent",
			line:     "    key: value",
			expected: 2,
		},
		{
			name:     "6-space indent",
			line:     "      key: value",
			expected: 3,
		},
		{
			name:     "tab indent",
			line:     "\tkey: value",
			expected: 1,
		},
		{
			name:     "multiple tab indent",
			line:     "\t\tkey: value",
			expected: 2,
		},
		{
			name:     "empty line",
			line:     "",
			expected: 0,
		},
		{
			name:     "comment with indent",
			line:     "  # comment",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetIndentationLevel(tt.line)
			if result != tt.expected {
				t.Errorf("GetIndentationLevel(%q) = %d, want %d",
					tt.line, result, tt.expected)
			}
		})
	}
}

// TestValidateMappingKeyIndentLine tests ValidateMappingKeyIndentLine function.
func TestValidateMappingKeyIndentLine(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		spacesPerLevel int
		expectedValid bool
	}{
		{
			name:           "top-level key",
			line:           "name: John",
			spacesPerLevel: 2,
			expectedValid:  true,
		},
		{
			name:           "nested key (2 spaces)",
			line:           "  child: value",
			spacesPerLevel: 2,
			expectedValid:  true,
		},
		{
			name:           "nested key (4 spaces)",
			line:           "    child: value",
			spacesPerLevel: 2,
			expectedValid:  true,
		},
		{
			name:           "invalid indentation (3 spaces)",
			line:           "   child: value",
			spacesPerLevel: 2,
			expectedValid:  false,
		},
		{
			name:           "invalid indentation (1 space)",
			line:           " child: value",
			spacesPerLevel: 2,
			expectedValid:  false,
		},
		{
			name:           "mixed indentation",
			line:           "  \tchild: value",
			spacesPerLevel: 2,
			expectedValid:  false,
		},
		{
			name:           "empty line should be skipped",
			line:           "",
			spacesPerLevel: 2,
			expectedValid:  true,
		},
		{
			name:           "comment should be skipped",
			line:           "# comment",
			spacesPerLevel: 2,
			expectedValid:  true,
		},
		{
			name:           "non-mapping key should be skipped",
			line:           "just some text",
			spacesPerLevel: 2,
			expectedValid:  true,
		},
		{
			name:           "tab-based indent with space expectation",
			line:           "\tkey: value",
			spacesPerLevel: 2,
			expectedValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateMappingKeyIndentLine(tt.line, tt.spacesPerLevel)
			if result != tt.expectedValid {
				t.Errorf("ValidateMappingKeyIndentLine(%q, %d) = %v, want %v",
					tt.line, tt.spacesPerLevel, result, tt.expectedValid)
			}
		})
	}
}

// TestValidateKeyIndentationSequence tests ValidateKeyIndentationSequence function.
func TestValidateKeyIndentationSequence(t *testing.T) {
	tests := []struct {
		name          string
		lines         []string
		spacesPerLevel int
		expectedValid bool
	}{
		{
			name: "valid simple sequence",
			lines: []string{
				"name: John",
				"age: 30",
				"city: NYC",
			},
			spacesPerLevel: 2,
			expectedValid:  true,
		},
		{
			name: "valid nested sequence",
			lines: []string{
				"person:",
				"  name: John",
				"  age: 30",
				"address:",
				"  city: NYC",
				"  state: NY",
			},
			spacesPerLevel: 2,
			expectedValid:  true,
		},
		{
			name: "valid complex nesting with comments",
			lines: []string{
				"# Top level configuration",
				"database:",
				"  # Database connection settings",
				"  host: localhost",
				"  port: 5432",
				"  credentials:",
				"    username: admin",
				"    password: secret",
			},
			spacesPerLevel: 2,
			expectedValid:  true,
		},
		{
			name: "valid sequence with empty lines",
			lines: []string{
				"key1: value1",
				"",
				"  nested: value",
				"",
				"",
				"key2: value2",
			},
			spacesPerLevel: 2,
			expectedValid:  true,
		},
		{
			name: "invalid - skip indentation level",
			lines: []string{
				"level1: value",
				"    level3: value", // Skips level 2
			},
			spacesPerLevel: 2,
			expectedValid:  false,
		},
		{
			name: "invalid - inconsistent indentation",
			lines: []string{
				"level1: value",
				" level2: value", // 1 space instead of 2
			},
			spacesPerLevel: 2,
			expectedValid:  false,
		},
		{
			name: "invalid - mixed indentation in sequence",
			lines: []string{
				"level1: value",
				"  level2: value",
				"  \tlevel3: value", // Mixed tabs and spaces
			},
			spacesPerLevel: 2,
			expectedValid:  false,
		},
		{
			name: "valid deep nesting",
			lines: []string{
				"l1: value",
				"  l2: value",
				"    l3: value",
				"      l4: value",
				"        l5: value",
				"          l6: value",
			},
			spacesPerLevel: 2,
			expectedValid:  true,
		},
		{
			name: "valid 4-space indentation",
			lines: []string{
				"level1: value",
				"    level2: value",
				"        level3: value",
			},
			spacesPerLevel: 4,
			expectedValid:  true,
		},
		{
			name: "valid return to multiple parent levels",
			lines: []string{
				"l1: value",
				"  l2: value",
				"    l3: value",
				"  l2_again: value", // Return to level 2
				"l1_again: value",   // Return to level 1
			},
			spacesPerLevel: 2,
			expectedValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateKeyIndentationSequence(tt.lines, tt.spacesPerLevel)
			if result != tt.expectedValid {
				t.Errorf("ValidateKeyIndentationSequence() = %v, want %v\nLines: %v",
					result, tt.expectedValid, tt.lines)
			}
		})
	}
}

// TestIndentationValidationEdgeCases tests edge cases in indentation validation.
func TestIndentationValidationEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		isMappingKey  bool
		expectedValid bool
	}{
		{
			name:          "only whitespace",
			line:          "     ",
			isMappingKey:  false,
			expectedValid: true,
		},
		{
			name:          "only tabs",
			line:          "\t\t\t",
			isMappingKey:  false,
			expectedValid: true,
		},
		{
			name:          "key with trailing spaces",
			line:          "key: value    ",
			isMappingKey:  true,
			expectedValid: true,
		},
		{
			name:          "key with inline comment",
			line:          "key: value # comment",
			isMappingKey:  true,
			expectedValid: true,
		},
		{
			name:          "very deep indentation",
			line: strings.Repeat(" ", 20) + "key: value",
			isMappingKey:  true,
			expectedValid: true,
		},
		{
			name:          "quoted key with indent",
			line:          "  \"key name\": value",
			isMappingKey:  true,
			expectedValid: true,
		},
		{
			name:          "sequence item (not a mapping key)",
			line:          "- item",
			isMappingKey:  false,
			expectedValid: true,
		},
		{
			name:          "sequence item with mapping key",
			line:          "- name: value",
			isMappingKey:  true,
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewIndentationContext(2)
			result := ctx.ValidateMappingKeyIndent(tt.line, tt.isMappingKey)
			if result != tt.expectedValid {
				t.Errorf("ValidateMappingKeyIndent(%q, %v) = %v, want %v",
					tt.line, tt.isMappingKey, result, tt.expectedValid)
			}
		})
	}
}

// BenchmarkIndentationValidation benchmarks the validation functions.
func BenchmarkIndentationValidation(b *testing.B) {
	ctx := NewIndentationContext(2)
	line := "  key: value"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.ValidateMappingKeyIndent(line, true)
	}
}

// BenchmarkValidateKeyIndentationSequence benchmarks sequence validation.
func BenchmarkValidateKeyIndentationSequence(b *testing.B) {
	lines := []string{
		"level1: value",
		"  level2: value",
		"    level3: value",
		"  level2_again: value",
		"level1_again: value",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateKeyIndentationSequence(lines, 2)
	}
}

// BenchmarkGetIndentationLevel benchmarks GetIndentationLevel.
func BenchmarkGetIndentationLevel(b *testing.B) {
	line := "    key: value"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetIndentationLevel(line)
	}
}
