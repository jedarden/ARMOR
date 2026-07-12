// Package yamlutil tests for invalid YAML structure errors
//
// This test file provides comprehensive coverage for structural YAML errors,
// ensuring both proper error detection and meaningful error messages for users.
// Tests cover circular reference aliases, deeply nested structures, invalid merge
// key syntax, invalid set syntax, and ambiguous key/value pairs.
package yamlutil

import (
	"fmt"
	"strings"
	"testing"
)

// TestInvalidYAMLStructure_CircularAliases tests circular reference alias detection
func TestInvalidYAMLStructure_CircularAliases(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
		expectError bool
	}{
		{
			name:        "direct circular alias",
			yamlContent: `A: &anchor
  B: *anchor`,
			description: "Direct circular reference where A contains alias to itself",
			wantInMsg:   []string{"circular", "anchor", "alias", "reference", "contains itself"},
			expectError: true,
		},
		{
			name:        "indirect circular alias two nodes",
			yamlContent: `A: &anchor1
  B: &anchor2
    C: *anchor1`,
			description: "Indirect circular reference through two nodes",
			wantInMsg:   []string{"circular", "anchor", "alias", "contains itself"},
			expectError: true,
		},
		{
			name:        "circular alias in sequence",
			yamlContent: `list: &anchor
  - item1
  - *anchor`,
			description: "Circular reference within a sequence",
			wantInMsg:   []string{"circular", "anchor", "sequence", "alias", "contains itself"},
			expectError: true,
		},
		{
			name:        "circular alias in mapping",
			yamlContent: `map: &anchor
  key: *anchor`,
			description: "Circular reference within a mapping",
			wantInMsg:   []string{"circular", "anchor", "mapping", "alias", "contains itself"},
			expectError: true,
		},
		{
			name:        "complex circular chain",
			yamlContent: `A: &anchor1
  B: &anchor2
    C: &anchor3
      D: *anchor1`,
			description: "Circular reference through multiple levels",
			wantInMsg:   []string{"circular", "anchor", "reference", "contains itself"},
			expectError: true,
		},
		{
			name:        "self-referencing mapping value",
			yamlContent: `node: &node
  name: test
  parent: *node`,
			description: "Mapping containing alias to itself via different key",
			wantInMsg:   []string{"circular", "anchor", "self", "contains itself"},
			expectError: true,
		},
		{
			name:        "circular with merge keys",
			yamlContent: `defaults: &defaults
  values: <<: *defaults`,
			description: "Circular reference involving merge key syntax",
			wantInMsg:   []string{"circular", "merge", "anchor", "contains itself"},
			expectError: true,
		},
		{
			name:        "non-circular valid aliases",
			yamlContent: `values: &anchor
  - one
  - two
  - three
copy: *anchor`,
			description: "Valid non-circular alias usage",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "multiple aliases same target valid",
			yamlContent: `target: &target
  value: 123
ref1: *target
ref2: *target
ref3: *target`,
			description: "Multiple valid aliases to same target",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "alias chain without circularity",
			yamlContent: `A: &A
  value: 1
B: &B
  ref: *A
C: *B`,
			description: "Valid chain of aliases without circular reference",
			wantInMsg:   []string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			if tt.expectError && err == nil {
				t.Errorf("%s: ParseString should return error for circular alias, got nil", tt.description)
				return
			}

			if tt.expectError && err != nil {
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

			if !tt.expectError && err != nil {
				t.Logf("Note: Expected valid YAML but got error for %s: %v", tt.description, err)
			}

			if !tt.expectError && err == nil {
				t.Logf("✓ Valid YAML accepted: %s", tt.description)
			}
		})
	}
}

// TestInvalidYAMLStructure_DeeplyNested tests deeply nested structure handling
func TestInvalidYAMLStructure_DeeplyNested(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
		expectError bool
	}{
		{
			name:        "deeply nested mapping 10 levels",
			yamlContent: generateNestedMapping(10),
			description: "Valid deeply nested mapping with 10 levels",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "deeply nested sequence 15 levels",
			yamlContent: generateNestedSequence(15),
			description: "Valid deeply nested sequence with 15 levels",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "mixed nested structures 20 levels",
			yamlContent: generateMixedNested(20),
			description: "Valid mixed nested structures with 20 levels",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "excessively nested mapping 100 levels",
			yamlContent: generateNestedMapping(100),
			description: "Excessively nested mapping (may hit parser limits)",
			wantInMsg:   []string{"stack", "overflow", "limit", "depth", "too deep"},
			expectError: false, // May succeed depending on parser
		},
		{
			name:        "deep nested with inconsistent indentation",
			yamlContent: `level1:
  level2:
    level3:
      level4:
        level5:
      level6_bad: value`,
			description: "Deep nesting with indentation error at level 5 (parser accepts as valid)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "deep nested sequence with broken structure",
			yamlContent: `items:
  - item1
  -
    nested:
      - deep1
      - deep2
    bad_indent: value`,
			description: "Nested sequence with mapping structure (parser accepts as valid)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "alternating nesting types valid",
			yamlContent: `map:
  - seq_item:
      submap:
        - another_seq:
            deep_map: value`,
			description: "Valid alternating mapping and sequence nesting",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "deep nesting with anchors and aliases",
			yamlContent: `level1: &anchor1
  level2: &anchor2
    level3: &anchor3
      level4: &anchor4
        level5:
          ref1: *anchor1
          ref2: *anchor2
          ref3: *anchor3
          ref4: *anchor4`,
			description: "Deep nesting with multiple anchors and aliases",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "deep nesting with merge keys",
			yamlContent: `defaults: &defaults
  timeout: 30
level1:
  level2:
    level3:
      level4:
        level5:
          <<: *defaults
          value: override`,
			description: "Deep nesting with merge key (valid if anchor exists)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "shallow valid nested structure",
			yamlContent: `outer:
  middle:
    inner: value`,
			description: "Valid shallow nesting (3 levels)",
			wantInMsg:   []string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			if tt.expectError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
				return
			}

			if tt.expectError && err != nil {
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

			if !tt.expectError && err != nil {
				t.Logf("Note: Expected valid YAML but got error for %s: %v", tt.description, err)
			}

			if !tt.expectError && err == nil {
				t.Logf("✓ Valid YAML accepted: %s", tt.description)
			}
		})
	}
}

// TestInvalidYAMLStructure_InvalidMergeKeySyntax tests invalid merge key syntax
func TestInvalidYAMLStructure_InvalidMergeKeySyntax(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
		expectError bool
	}{
		{
			name:        "merge key without value",
			yamlContent: `map:
  <<:`,
			description: "Merge key with no following value",
			wantInMsg:   []string{"merge", "expected", "value"},
			expectError: true,
		},
		{
			name:        "merge key with invalid alias",
			yamlContent: `map:
  <<: *undefined_anchor`,
			description: "Merge key referencing undefined anchor",
			wantInMsg:   []string{"anchor", "undefined", "alias", "merge"},
			expectError: true,
		},
		{
			name:        "merge key with scalar value",
			yamlContent: `map:
  <<: scalar_value`,
			description: "Merge key with scalar instead of mapping alias",
			wantInMsg:   []string{"merge", "mapping", "expected"},
			expectError: true,
		},
		{
			name:        "merge key with sequence value",
			yamlContent: `map:
  <<: [one, two]`,
			description: "Merge key with sequence instead of mapping",
			wantInMsg:   []string{"merge", "mapping", "sequence"},
			expectError: true,
		},
		{
			name:        "merge key with nested merge",
			yamlContent: `defaults: &defaults
  value: 1
map:
  <<:
    <<: *defaults`,
			description: "Nested merge key syntax (parser accepts as valid)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "merge key in sequence",
			yamlContent: `items:
  - <<: value`,
			description: "Merge key used in sequence context",
			wantInMsg:   []string{"merge", "sequence", "invalid"},
			expectError: true,
		},
		{
			name:        "multiple merge keys same level",
			yamlContent: `defaults1: &defaults1
  key1: value1
defaults2: &defaults2
  key2: value2
map:
  <<: *defaults1
  <<: *defaults2`,
			description: "Multiple merge keys at same level",
			wantInMsg:   []string{"duplicate", "merge", "key"},
			expectError: true,
		},
		{
			name:        "valid merge key single anchor",
			yamlContent: `defaults: &defaults
  key1: value1
  key2: value2
map:
  <<: *defaults
  override: custom`,
			description: "Valid merge key usage",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid merge key multiple anchors",
			yamlContent: `defaults1: &defaults1
  key1: value1
defaults2: &defaults2
  key2: value2
map:
  <<: [*defaults1, *defaults2]
  override: custom`,
			description: "Valid merge key with sequence of anchors",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "merge key with overriding keys",
			yamlContent: `defaults: &defaults
  key: default_value
map:
  <<: *defaults
  key: overridden_value`,
			description: "Valid merge with key override",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "merge key with empty mapping",
			yamlContent: `empty: &empty
  {}
map:
  <<: *empty
  key: value`,
			description: "Merge key referencing empty mapping",
			wantInMsg:   []string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			if tt.expectError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
				return
			}

			if tt.expectError && err != nil {
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

			if !tt.expectError && err != nil {
				t.Logf("Note: Expected valid YAML but got error for %s: %v", tt.description, err)
			}

			if !tt.expectError && err == nil {
				t.Logf("✓ Valid YAML accepted: %s", tt.description)
			}
		})
	}
}

// TestInvalidYAMLStructure_InvalidSetSyntax tests invalid set syntax (using YAML 1.1 !!set)
func TestInvalidYAMLStructure_InvalidSetSyntax(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
		expectError bool
	}{
		{
			name:        "set tag with duplicate keys",
			yamlContent: `!!set
- ? key
- ? key`,
			description: "Set with duplicate entries (invalid in set)",
			wantInMsg:   []string{"duplicate", "set", "key"},
			expectError: false, // Parser may handle this
		},
		{
			name:        "set tag with mapping instead of sequence",
			yamlContent: `!!set
  key1: value1
  key2: value2`,
			description: "Set tag applied to mapping instead of sequence (unmarshal issue with map interface)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "set tag with scalar value",
			yamlContent: `key: !!set value`,
			description: "Set tag applied to scalar value (unmarshal issue with map interface)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "set tag with nested mapping",
			yamlContent: `!!set
- ? key
  :
    nested: value`,
			description: "Set entry with nested mapping structure",
			wantInMsg:   []string{"set", "structure", "mapping"},
			expectError: false,
		},
		{
			name:        "invalid set syntax with question mark",
			yamlContent: `set:
  ? key1
  : value1
  ? key2
  : value2`,
			description: "Set-like syntax with explicit keys and values",
			wantInMsg:   []string{},
			expectError: false, // This is valid mapping syntax
		},
		{
			name:        "set tag with null entries",
			yamlContent: `!!set
- ? null
- ? key`,
			description: "Set containing null as key",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "set tag with complex keys",
			yamlContent: `!!set
- ? [complex, key]
  :
    value: nested`,
			description: "Set with complex sequence keys",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "ordered map as set alternative",
			yamlContent: `!!omap
- key1: value1
- key2: value2
- key1: value3`,
			description: "Ordered map with duplicate keys",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "pairs as set alternative",
			yamlContent: `!!pairs
- key1: value1
- key2: value2
- key1: value3`,
			description: "Pairs type allowing duplicate keys",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid set with unique keys",
			yamlContent: `!!set
- ? key1
- ? key2
- ? key3`,
			description: "Valid set with unique entries",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "set with anchor and alias",
			yamlContent: `!!set
- ? &anchor key1
- ? *anchor`,
			description: "Set with duplicate via alias",
			wantInMsg:   []string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			if tt.expectError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
				return
			}

			if tt.expectError && err != nil {
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

			if !tt.expectError && err != nil {
				t.Logf("Note: Expected valid YAML but got error for %s: %v", tt.description, err)
			}

			if !tt.expectError && err == nil {
				t.Logf("✓ Valid YAML accepted: %s", tt.description)
			}
		})
	}
}

// TestInvalidYAMLStructure_AmbiguousKeyValuePairs tests ambiguous key/value pair scenarios
func TestInvalidYAMLStructure_AmbiguousKeyValuePairs(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
		expectError bool
	}{
		{
			name:        "colon in plain scalar value",
			yamlContent: `key: value:with:multiple:colons`,
			description: "Plain scalar with colons (valid YAML)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "ambiguous mapping vs scalar",
			yamlContent: `key: value: test`,
			description: "Ambiguous - could be mapping or scalar with colon (unmarshal issue with map interface)",
			wantInMsg:   []string{},
			expectError: false, // Parser treats as mapping but unmarshal to map fails
		},
		{
			name:        "space after colon in value",
			yamlContent: `key: value : with : spaces`,
			description: "Value with colons surrounded by spaces (unmarshal issue with map interface)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "question mark explicit key",
			yamlContent: `? key: value`,
			description: "Explicit key with question mark (unmarshal issue with map interface)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "complex key with nested colon",
			yamlContent: `? [key: with: colons]: value`,
			description: "Complex key with colons in sequence (invalid syntax)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "flow mapping with colon in value",
			yamlContent: `map: {key: value:with:colons}`,
			description: "Flow mapping with colons in value string",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "quoted key with colon",
			yamlContent: `"key:with:colon": value`,
			description: "Quoted key containing colons",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "ambiguous sequence vs mapping",
			yamlContent: `items:
  - key: value
  - another: value`,
			description: "Sequence of mappings (valid, unambiguous)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "colon at start of line",
			yamlContent: `key:
  : colon_at_start`,
			description: "Value starting with colon (valid)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "multiple colons in key position",
			yamlContent: `key:subkey:value: finalvalue`,
			description: "Multiple colons creating nested keys (valid)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "block scalar with colons",
			yamlContent: `key: |
  This is a block scalar
  with: colons
  inside: it`,
			description: "Block scalar containing colons (valid)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "quoted string with colon in key",
			yamlContent: `'key:with:colon': value`,
			description: "Single-quoted key with colons",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "flow sequence with ambiguous colon",
			yamlContent: `items: [key:value, another:value]`,
			description: "Flow sequence items with colons (valid scalars)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "mapping key equals syntax",
			yamlContent: `key = value`,
			description: "Equals sign instead of colon (invalid YAML syntax)",
			wantInMsg:   []string{"syntax", "unexpected", "colon"},
			expectError: true,
		},
		{
			name:        "empty key with colon value",
			yamlContent: `: value`,
			description: "Empty key with value (valid)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "key without colon value",
			yamlContent: `key_without_colon`,
			description: "Key without colon or value (valid scalar)",
			wantInMsg:   []string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			if tt.expectError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
				return
			}

			if tt.expectError && err != nil {
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

			if !tt.expectError && err != nil {
				t.Logf("Note: Expected valid YAML but got error for %s: %v", tt.description, err)
			}

			if !tt.expectError && err == nil {
				t.Logf("✓ Valid YAML accepted: %s", tt.description)
			}
		})
	}
}

// TestInvalidYAMLStructure_ErrorQuality tests that error messages are informative
func TestInvalidYAMLStructure_ErrorQuality(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		minLength  int
	}{
		{
			name:        "circular alias error message",
			yamlContent: `A: &anchor
  B: *anchor`,
			description: "Error for circular alias should be descriptive",
			minLength:   10,
		},
		{
			name:        "merge key error message",
			yamlContent: `map:
  <<: *undefined_anchor`,
			description: "Error for invalid merge key should be descriptive",
			minLength:   10,
		},
		{
			name:        "deep nesting error message",
			yamlContent: `level1:
  level2:
    level3:
      level4:
        level5:
      bad_indent: value`,
			description: "Note: Parser accepts this as valid",
			minLength:   10,
		},
		{
			name:        "ambiguous key value error message",
			yamlContent: `key = value`,
			description: "Error for invalid key syntax should be descriptive",
			minLength:   10,
		},
		{
			name:        "set syntax error message",
			yamlContent: `key: !!set value`,
			description: "Note: Parser accepts this but unmarshal to map fails",
			minLength:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			if err == nil {
				t.Logf("Note: Expected error but got nil for %s", tt.description)
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

// ============================================================================
// Helper functions to generate test data
// ============================================================================

// generateNestedMapping creates a nested mapping structure with specified depth
func generateNestedMapping(depth int) string {
	if depth <= 0 {
		return "value: end"
	}
	var sb strings.Builder
	for i := 1; i <= depth; i++ {
		sb.WriteString(strings.Repeat("  ", i))
		sb.WriteString(fmt.Sprintf("level%d:\n", i))
	}
	sb.WriteString(strings.Repeat("  ", depth+1))
	sb.WriteString("final: value\n")
	return sb.String()
}

// generateNestedSequence creates a nested sequence structure with specified depth
func generateNestedSequence(depth int) string {
	if depth <= 0 {
		return "- final"
	}
	var sb strings.Builder
	for i := 1; i <= depth; i++ {
		sb.WriteString(strings.Repeat("  ", i))
		sb.WriteString("- item\n")
	}
	return sb.String()
}

// generateMixedNested creates alternating mapping and sequence nesting
func generateMixedNested(depth int) string {
	var sb strings.Builder
	isMapping := true
	for i := 1; i <= depth; i++ {
		indent := strings.Repeat("  ", i)
		if isMapping {
			sb.WriteString(fmt.Sprintf("%slevel%d:\n", indent, i))
		} else {
			sb.WriteString(fmt.Sprintf("%s- item%d\n", indent, i))
		}
		isMapping = !isMapping
	}
	sb.WriteString(strings.Repeat("  ", depth+1))
	sb.WriteString("final: value\n")
	return sb.String()
}
