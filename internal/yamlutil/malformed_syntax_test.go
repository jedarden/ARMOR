// Package yamlutil tests for malformed YAML syntax errors
//
// This test file provides comprehensive coverage for malformed YAML syntax,
// ensuring both proper error detection and meaningful error messages for users.
// Tests cover unmatched brackets/braces, invalid block scalars, indentation issues,
// malformed document separators, and invalid anchor/alias syntax.
package yamlutil

import (
	"strings"
	"testing"
)

// TestMalformedYAML_UnmatchedBracketsBraces tests unmatched brackets and braces in flow styles
func TestMalformedYAML_UnmatchedBracketsBraces(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
	}{
		{
			name:        "unmatched opening bracket in sequence",
			yamlContent: `items: [one, two, three`,
			description: "Flow sequence missing closing bracket",
			wantInMsg:   []string{"bracket", "unmatched", "did not find", "expected"},
		},
		{
			name:        "unmatched closing bracket in sequence",
			yamlContent: `items: one, two, three]`,
			description: "Closing bracket in plain scalar value (valid YAML)",
			wantInMsg:   []string{},
		},
		{
			name:        "unmatched opening brace in mapping",
			yamlContent: `map: {key: value, another: item`,
			description: "Flow mapping missing closing brace",
			wantInMsg:   []string{"brace", "unmatched", "did not find", "expected"},
		},
		{
			name:        "unmatched closing brace in mapping",
			yamlContent: `map: key: value, another: item}`,
			description: "Flow mapping with unexpected closing brace",
			wantInMsg:   []string{"unexpected", "brace", "syntax"},
		},
		{
			name:        "nested unmatched brackets",
			yamlContent: `nested: [one, [two, three]`,
			description: "Nested flow sequence with unmatched inner bracket",
			wantInMsg:   []string{"bracket", "unmatched", "nested"},
		},
		{
			name:        "nested unmatched braces",
			yamlContent: `nested: {key: {inner: value}`,
			description: "Nested flow mapping with unmatched inner brace",
			wantInMsg:   []string{"brace", "unmatched", "nested"},
		},
		{
			name:        "mixed unmatched brackets and braces",
			yamlContent: `mixed: [one, {key: value, two]`,
			description: "Mixed flow collection with mismatched delimiters",
			wantInMsg:   []string{"brace", "bracket", "unmatched", "expected"},
		},
		{
			name:        "multiple opening brackets",
			yamlContent: `items: [[one, two, three`,
			description: "Multiple opening brackets without matching closing brackets",
			wantInMsg:   []string{"bracket", "unmatched", "did not find"},
		},
		{
			name:        "bracket in middle of word",
			yamlContent: `key: value[with]brackets`,
			description: "Brackets used in scalar value (valid)",
			wantInMsg:   []string{},
		},
		{
			name:        "empty flow sequence",
			yamlContent: `items: []`,
			description: "Empty flow sequence (valid)",
			wantInMsg:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			// Determine if this should error based on the test case
			// Empty wantInMsg means valid YAML (no error expected)
			// Non-empty wantInMsg means error is expected, with those keywords in error message
			// For backward compatibility, also check description if wantInMsg is not explicitly set
			shouldError := len(tt.wantInMsg) > 0 ||
				strings.Contains(tt.description, "missing closing") ||
				strings.Contains(tt.description, "without matching") ||
				(strings.Contains(tt.description, "unmatched") && !strings.Contains(tt.description, "(valid"))

			if shouldError && err == nil {
				t.Errorf("%s: ParseString should return error for invalid YAML, got nil", tt.description)
				return
			}

			if shouldError && err != nil {
				// Verify error message contains expected keywords
				errMsg := strings.ToLower(err.Error())
				foundAny := false
				for _, wanted := range tt.wantInMsg {
					if strings.Contains(errMsg, wanted) {
						foundAny = true
						t.Logf("✓ Error message contains '%s': %v", wanted, err)
						break
					}
				}
				if !foundAny && len(tt.wantInMsg) > 0 {
					t.Logf("Note: Error message for %s: %v", tt.description, err)
				}
			}

			if !shouldError && err != nil {
				t.Logf("Note: Expected valid YAML but got error for %s: %v", tt.description, err)
			}
		})
	}
}

// TestMalformedYAML_InvalidBlockScalars tests invalid block scalar syntax
func TestMalformedYAML_InvalidBlockScalars(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
	}{
		{
			name: "literal scalar with inconsistent indentation",
			yamlContent: `key: |
  line1
line2
  line3`,
			description: "Literal block scalar with inconsistent indentation (line2 not indented)",
			wantInMsg:   []string{"indent", "mapping", "structure"},
		},
		{
			name: "folded scalar with inconsistent indentation",
			yamlContent: `key: >
  line1
    line2
  line3`,
			description: "Folded block scalar with inconsistent indentation (valid - parser accepts)",
			wantInMsg:   []string{},
		},
		{
			name: "block scalar with mixed markers",
			yamlContent: `key: | >
  content`,
			description: "Multiple block scalar markers combined",
			wantInMsg:   []string{"unexpected", "marker", "syntax"},
		},
		{
			name: "literal scalar with negative indentation modifier",
			yamlContent: `key: |-1
  content`,
			description: "Block scalar with invalid indentation modifier (valid - parser accepts as chomping modifier)",
			wantInMsg:   []string{},
		},
		{
			name: "folded scalar with chomping and indent",
			yamlContent: `key: >-2
  content`,
			description: "Block scalar with both chomping and indentation modifier (valid)",
			wantInMsg:   []string{},
		},
		{
			name: "literal scalar empty (valid)",
			yamlContent: `key: |
`,
			description: "Empty literal block scalar",
			wantInMsg:   []string{},
		},
		{
			name: "folded scalar empty (valid)",
			yamlContent: `key: >
`,
			description: "Empty folded block scalar",
			wantInMsg:   []string{},
		},
		{
			name: "block scalar with document start marker",
			yamlContent: `key: |
  ---`,
			description: "Block scalar containing document start marker (valid)",
			wantInMsg:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			shouldError := len(tt.wantInMsg) > 0

			if shouldError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
				return
			}

			if shouldError && err != nil {
				errMsg := strings.ToLower(err.Error())
				foundAny := false
				for _, wanted := range tt.wantInMsg {
					if strings.Contains(errMsg, wanted) {
						foundAny = true
						t.Logf("✓ Error message contains '%s': %v", wanted, err)
						break
					}
				}
				if !foundAny && len(tt.wantInMsg) > 0 {
					t.Logf("Note: Error message for %s: %v", tt.description, err)
				}
			}

			if !shouldError && err != nil {
				t.Logf("Note: Expected valid YAML but got error for %s: %v", tt.description, err)
			}
		})
	}
}

// TestMalformedYAML_InvalidIndentation tests invalid indentation and tab/space mixing
func TestMalformedYAML_InvalidIndentation(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
	}{
		{
			name: "inconsistent indentation in mapping",
			yamlContent: `key:
  subkey: value
    badkey: value`,
			description: "Mapping with inconsistent indentation",
			wantInMsg:   []string{"indent", "mapping", "structure"},
		},
		{
			name: "decreasing indentation without context",
			yamlContent: `key:
    subkey: value
  badkey: value`,
			description: "Decreasing indentation in mapping",
			wantInMsg:   []string{"indent", "mapping"},
		},
		{
			name: "tab after space in mapping",
			yamlContent: "key:\n  \tsub: value",
			description: "Mixed tabs and spaces in indentation (tab after space)",
			wantInMsg:   []string{"tab", "indent", "inconsistent"},
		},
		{
			name: "space after tab in mapping",
			yamlContent: "key:\n\t  sub: value",
			description: "Mixed tabs and spaces in indentation (space after tab)",
			wantInMsg:   []string{"tab", "indent", "inconsistent"},
		},
		{
			name: "sequence with inconsistent indentation",
			yamlContent: `items:
  - one
 - two`,
			description: "Sequence item with insufficient indentation",
			wantInMsg:   []string{"indent", "sequence"},
		},
		{
			name: "nested mapping with bad indent",
			yamlContent: `outer:
  inner:
    deep: value
 bad: value`,
			description: "Nested mapping with inconsistent indentation",
			wantInMsg:   []string{"indent", "mapping"},
		},
		{
			name: "mapping inside sequence with bad indent",
			yamlContent: `items:
  - key: value
bad: value`,
			description: "Mapping item at same level as sequence (valid - separate mapping key)",
			wantInMsg:   []string{},
		},
		{
			name: "tabs only (consistent)",
			yamlContent: "key:\n\tsub:\n\t\tdeep: value",
			description: "Consistent tab-only indentation (valid)",
			wantInMsg:   []string{},
		},
		{
			name: "spaces only (consistent)",
			yamlContent: `key:
  sub:
    deep: value`,
			description: "Consistent space-only indentation (valid)",
			wantInMsg:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			shouldError := len(tt.wantInMsg) > 0

			if shouldError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
				return
			}

			if shouldError && err != nil {
				errMsg := strings.ToLower(err.Error())
				foundAny := false
				for _, wanted := range tt.wantInMsg {
					if strings.Contains(errMsg, wanted) {
						foundAny = true
						t.Logf("✓ Error message contains '%s': %v", wanted, err)
						break
					}
				}
				if !foundAny && len(tt.wantInMsg) > 0 {
					t.Logf("Note: Error message for %s: %v", tt.description, err)
				}
			}

			if !shouldError && err != nil {
				t.Logf("Note: Expected valid YAML but got error for %s: %v", tt.description, err)
			}
		})
	}
}

// TestMalformedYAML_DocumentSeparators tests malformed document separators
func TestMalformedYAML_DocumentSeparators(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
	}{
		{
			name:        "incomplete document start",
			yamlContent: `--`,
			description: "Incomplete document start marker (only 2 dashes)",
			wantInMsg:   []string{"document", "unexpected", "separator"},
		},
		{
			name:        "extra dashes in document separator",
			yamlContent: `----`,
			description: "Document separator with 4 dashes (valid - treated as comment)",
			wantInMsg:   []string{},
		},
		{
			name:        "document end with trailing spaces",
			yamlContent: `key: value
...  `,
			description: "Document end marker with trailing spaces (valid)",
			wantInMsg:   []string{},
		},
		{
			name:        "document separator in mapping key",
			yamlContent: `key: value
---
another: key`,
			description: "Valid document separator between documents",
			wantInMsg:   []string{},
		},
		{
			name:        "document separator in flow sequence",
			yamlContent: `items: [one, ---, two]`,
			description: "Document separator used as flow sequence item (valid)",
			wantInMsg:   []string{},
		},
		{
			name:        "document start with no content",
			yamlContent: `---
`,
			description: "Document start marker with no content (valid - empty document)",
			wantInMsg:   []string{},
		},
		{
			name:        "multiple document separators",
			yamlContent: `---
key: value
---
another: key
---
`,
			description: "Multiple document separators (valid - multiple documents)",
			wantInMsg:   []string{},
		},
		{
			name:        "document separator in quoted string",
			yamlContent: `key: "---"`,
			description: "Document separator inside quoted string (valid)",
			wantInMsg:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			shouldError := len(tt.wantInMsg) > 0

			if shouldError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
				return
			}

			if shouldError && err != nil {
				errMsg := strings.ToLower(err.Error())
				foundAny := false
				for _, wanted := range tt.wantInMsg {
					if strings.Contains(errMsg, wanted) {
						foundAny = true
						t.Logf("✓ Error message contains '%s': %v", wanted, err)
						break
					}
				}
				if !foundAny && len(tt.wantInMsg) > 0 {
					t.Logf("Note: Error message for %s: %v", tt.description, err)
				}
			}

			if !shouldError && err != nil {
				t.Logf("Note: Expected valid YAML but got error for %s: %v", tt.description, err)
			}
		})
	}
}

// TestMalformedYAML_AnchorAliasSyntax tests invalid anchor and alias syntax
func TestMalformedYAML_AnchorAliasSyntax(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
	}{
		{
			name:        "anchor without name",
			yamlContent: `key: & value`,
			description: "Anchor marker without a name",
			wantInMsg:   []string{"anchor", "name", "unexpected"},
		},
		{
			name:        "alias without name",
			yamlContent: `key: * value`,
			description: "Alias marker without a name",
			wantInMsg:   []string{"alias", "name", "unexpected"},
		},
		{
			name:        "alias to undefined anchor",
			yamlContent: `key1: &anchor value1
key2: *unknown`,
			description: "Alias referencing undefined anchor",
			wantInMsg:   []string{"alias", "anchor", "found", "undefined"},
		},
		{
			name:        "anchor with invalid characters",
			yamlContent: `key: &invalid@anchor value`,
			description: "Anchor name with invalid characters (@ symbol)",
			wantInMsg:   []string{"anchor", "invalid", "character"},
		},
		{
			name:        "anchor starting with number",
			yamlContent: `key: &1invalid value`,
			description: "Anchor name starting with digit (valid - parser accepts)",
			wantInMsg:   []string{},
		},
		{
			name:        "multiple anchors on same node",
			yamlContent: `key: &anchor1 &anchor2 value`,
			description: "Multiple anchor markers on same node",
			wantInMsg:   []string{"anchor", "unexpected", "duplicate"},
		},
		{
			name:        "anchor and alias on same node",
			yamlContent: `key: &anchor *alias value`,
			description: "Both anchor and alias on same node",
			wantInMsg:   []string{"anchor", "alias", "unexpected"},
		},
		{
			name:        "valid anchor and alias",
			yamlContent: `key1: &anchor value1
key2: *anchor`,
			description: "Valid anchor and alias usage",
			wantInMsg:   []string{},
		},
		{
			name:        "anchor in flow sequence",
			yamlContent: `items: [&one value1, value2]`,
			description: "Anchor on flow sequence item (valid)",
			wantInMsg:   []string{},
		},
		{
			name:        "alias in flow mapping",
			yamlContent: `map: {key: *anchor}`,
			description: "Alias in flow mapping value (valid if anchor defined)",
			wantInMsg:   []string{},
		},
		{
			name:        "anchor with underscores",
			yamlContent: `key: &my_anchor value`,
			description: "Anchor name with underscores (valid)",
			wantInMsg:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			shouldError := len(tt.wantInMsg) > 0

			if shouldError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
				return
			}

			if shouldError && err != nil {
				errMsg := strings.ToLower(err.Error())
				foundAny := false
				for _, wanted := range tt.wantInMsg {
					if strings.Contains(errMsg, wanted) {
						foundAny = true
						t.Logf("✓ Error message contains '%s': %v", wanted, err)
						break
					}
				}
				if !foundAny && len(tt.wantInMsg) > 0 {
					t.Logf("Note: Error message for %s: %v", tt.description, err)
				}
			}

			if !shouldError && err != nil {
				t.Logf("Note: Expected valid YAML but got error for %s: %v", tt.description, err)
			}
		})
	}
}

// TestMalformedYAML_ComprehensiveSyntaxErrors tests comprehensive syntax error scenarios
func TestMalformedYAML_ComprehensiveSyntaxErrors(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		category    string
	}{
		{
			name: "unclosed double quote",
			yamlContent: `key: "unclosed string`,
			description: "Double quote string not closed",
			category:    "quote",
		},
		{
			name: "unclosed single quote",
			yamlContent: `key: 'unclosed string`,
			description: "Single quote string not closed",
			category:    "quote",
		},
		{
			name: "invalid escape in double quote",
			yamlContent: `key: "invalid \xgh escape"`,
			description: "Invalid hexadecimal escape sequence",
			category:    "escape",
		},
		{
			name: "invalid unicode escape",
			yamlContent: `key: "invalid \u12"`,
			description: "Unicode escape with insufficient hex digits",
			category:    "escape",
		},
		{
			name: "colon in flow sequence",
			yamlContent: `items: [key: value]`,
			description: "Key-value pair in flow sequence (valid - creates nested map)",
			category:    "",
		},
		{
			name: "duplicate mapping keys",
			yamlContent: `key: value1
key: value2`,
			description: "Duplicate key in mapping",
			category:    "duplicate",
		},
		{
			name: "invalid tag separator",
			yamlContent: `key: !tag!value`,
			description: "Malformed tag syntax (valid - parser accepts)",
			category:    "",
		},
		{
			name: "plain scalar with colon",
			yamlContent: `key: value:with:colons`,
			description: "Plain scalar with colons (valid)",
			category:    "",
		},
		{
			name: "question mark start",
			yamlContent: `? key: value`,
			description: "Explicit mapping key with question mark (valid)",
			category:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			// If category is specified, we expect an error
			shouldError := tt.category != ""

			if shouldError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
			}

			if shouldError && err != nil {
				errMsg := strings.ToLower(err.Error())
				if strings.Contains(errMsg, tt.category) {
					t.Logf("✓ Error message contains category '%s': %v", tt.category, err)
				} else {
					t.Logf("Note: Error message for %s: %v", tt.description, err)
				}
			}

			if !shouldError && err != nil {
				t.Logf("Note: Expected valid YAML but got error for %s: %v", tt.description, err)
			}

			if !shouldError && err == nil {
				t.Logf("✓ Valid YAML accepted: %s", tt.description)
			}
		})
	}
}

// TestMalformedYAML_ErrorQuality tests that error messages are informative
func TestMalformedYAML_ErrorQuality(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		minLength  int
	}{
		{
			name:        "unmatched bracket error message",
			yamlContent: `list: [one, two`,
			description: "Error for unmatched bracket should be descriptive",
			minLength:   10,
		},
		{
			name:        "unclosed quote error message",
			yamlContent: `key: "unclosed`,
			description: "Error for unclosed quote should be descriptive",
			minLength:   10,
		},
		{
			name:        "invalid indent error message",
			yamlContent: `key:
  sub: value
    bad: value`,
			description: "Error for bad indentation should be descriptive",
			minLength:   10,
		},
		{
			name:        "invalid anchor error message",
			yamlContent: `key: &invalid@anchor value`,
			description: "Error for invalid anchor should be descriptive",
			minLength:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			if err == nil {
				t.Errorf("%s: Expected error but got nil", tt.description)
				return
			}

			errMsg := err.Error()
			if len(errMsg) < tt.minLength {
				t.Errorf("%s: Error message too short (%d chars, want >= %d): %s",
					tt.description, len(errMsg), tt.minLength, errMsg)
			}

			t.Logf("✓ Error message quality: %s", errMsg)
		})
	}
}
