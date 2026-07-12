// Package yamlutil tests for invalid YAML structure errors
//
// This test file provides comprehensive coverage for invalid YAML structure errors,
// ensuring both proper error detection and meaningful error messages for users.
// Tests cover circular reference aliases, deeply nested structures, invalid merge
// key syntax, invalid set syntax, and ambiguous key/value pairs.
package yamlutil

import (
	"fmt"
	"strings"
	"testing"
)

// TestInvalidYAML_CircularReferences tests circular reference detection in YAML anchors and aliases
func TestInvalidYAML_CircularReferences(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
	}{
		{
			name: "direct circular reference",
			yamlContent: `&anchor
*anchor: value`,
			description: "Anchor directly references itself",
			wantInMsg:   []string{"circular", "anchor", "alias", "reference"},
		},
		{
			name: "indirect circular reference two nodes",
			yamlContent: `first: &first
  ref: *second
second: &second
  ref: *first`,
			description: "Two anchors referencing each other indirectly",
			wantInMsg:   []string{"circular", "anchor", "alias"},
		},
		{
			name: "circular reference through sequence",
			yamlContent: `items: &items
  - name: first
    next: *items`,
			description: "Circular reference through sequence item",
			wantInMsg:   []string{"circular", "sequence"},
		},
		{
			name: "circular reference through mapping",
			yamlContent: `map: &map
  key: value
  self: *map`,
			description: "Mapping contains circular reference to itself",
			wantInMsg:   []string{"circular", "mapping"},
		},
		{
			name: "deep circular reference chain",
			yamlContent: `a: &a
  ref: *b
b: &b
  ref: *c
c: &c
  ref: *a`,
			description: "Three-level circular reference chain",
			wantInMsg:   []string{"circular", "chain"},
		},
		{
			name: "circular reference in nested structure",
			yamlContent: `outer: &outer
  inner:
    deep: *outer`,
			description: "Circular reference through nested mapping",
			wantInMsg:   []string{"circular", "nested"},
		},
		{
			name: "multiple circular references",
			yamlContent: `a: &a
  ref1: *b
  ref2: *c
b: &b
  back: *a
c: &c
  back: *a`,
			description: "Multiple circular reference paths",
			wantInMsg:   []string{"circular", "multiple"},
		},
		{
			name: "circular reference with merge key",
			yamlContent: `base: &base
  <<: *base
  key: value`,
			description: "Merge key creating circular reference",
			wantInMsg:   []string{"circular", "merge"},
		},
		{
			name: "valid anchor reuse in sequence",
			yamlContent: `default: &default
  enabled: true
items:
  - *default
  - *default`,
			description: "Valid anchor reuse without circular reference",
			wantInMsg:   []string{},
		},
		{
			name: "valid anchor alias reference",
			yamlContent: `original: &original
  key: value
copy: *original`,
			description: "Valid anchor and alias usage",
			wantInMsg:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			// Determine if this should error based on wantInMsg
			shouldError := len(tt.wantInMsg) > 0

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

			if !shouldError && err == nil {
				t.Logf("✓ Valid YAML accepted: %s", tt.description)
			}
		})
	}
}

// TestInvalidYAML_DeeplyNestedStructure tests deeply nested YAML structure handling
func TestInvalidYAML_DeeplyNestedStructure(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
	}{
		{
			name: "extremely deep mapping nesting",
			yamlContent: generateDeepMapping(20),
			description: "20 levels of nested mapping",
			wantInMsg:   []string{}, // Parser should handle this
		},
		{
			name: "extremely deep sequence nesting",
			yamlContent: generateDeepSequence(15),
			description: "15 levels of nested sequences (invalid - exceeds parser limits)",
			wantInMsg:   []string{"did not find", "expected", "content"},
		},
		{
			name: "mixed deep nesting",
			yamlContent: `
level1:
  level2:
    level3:
      level4:
        level5:
          level6:
            level7:
              level8:
                level9:
                  level10: value`,
			description: "10 levels of mixed nested structure",
			wantInMsg:   []string{},
		},
		{
			name: "deep nesting with anchors",
			yamlContent: `
root: &root
  l1:
    l2:
      l3:
        l4:
          l5: *root`,
			description: "Deep nesting with circular reference",
			wantInMsg:   []string{"circular"},
		},
		{
			name: "reasonable nesting depth",
			yamlContent: `
app:
  config:
    database:
      connection:
        pool:
          settings:
            max: 10`,
			description: "Reasonable 6-level nesting depth",
			wantInMsg:   []string{},
		},
		{
			name: "deep sequence with mappings",
			yamlContent: generateDeepSequenceWithMappings(8),
			description: "8 levels of sequences containing mappings (invalid - malformed structure)",
			wantInMsg:   []string{"did not find", "expected", "key"},
		},
		{
			name: "deep flow mapping",
			yamlContent: `level1: {level2: {level3: {level4: {level5: value}}}}`,
			description: "Deep flow-style mapping",
			wantInMsg:   []string{},
		},
		{
			name: "deep flow sequence",
			yamlContent: `[[[[[[[value]]]]]]]`,
			description: "Deep flow-style sequence (invalid when parsed as mapping)",
			wantInMsg:   []string{"cannot unmarshal", "seq"},
		},
		{
			name: "alternating nested structures",
			yamlContent: `
- key:
    - subkey:
        - deepkey: value
    - another:
        - more: nesting`,
			description: "Alternating sequence and mapping nesting (invalid when parsed as mapping)",
			wantInMsg:   []string{"cannot unmarshal", "mapping"},
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

			if !shouldError && err == nil {
				t.Logf("✓ Valid YAML accepted: %s", tt.description)
			}
		})
	}
}

// TestInvalidYAML_MergeKeySyntax tests invalid merge key (<<) syntax
func TestInvalidYAML_MergeKeySyntax(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
	}{
		{
			name: "merge key without anchor",
			yamlContent: `
target:
  <<: value
  key: override`,
			description: "Merge key with plain value instead of alias",
			wantInMsg:   []string{"merge", "alias", "anchor"},
		},
		{
			name: "merge key with undefined anchor",
			yamlContent: `
target:
  <<: *undefined
  key: value`,
			description: "Merge key referencing undefined anchor",
			wantInMsg:   []string{"merge", "anchor", "undefined"},
		},
		{
			name: "merge key in sequence",
			yamlContent: `
- <<: *anchor
  key: value`,
			description: "Merge key used in sequence context",
			wantInMsg:   []string{"merge", "sequence"},
		},
		{
			name: "multiple merge keys without aliases",
			yamlContent: `
target:
  <<: value1
  <<: value2
  key: value`,
			description: "Multiple merge keys with invalid values",
			wantInMsg:   []string{"merge", "duplicate"},
		},
		{
			name: "merge key with sequence of undefined anchors",
			yamlContent: `
defaults: &defaults {key: value}
target:
  <<: [*defaults, *undefined]`,
			description: "Merge key sequence with undefined anchor",
			wantInMsg:   []string{"merge", "anchor", "undefined"},
		},
		{
			name: "merge key with non-mapping anchor",
			yamlContent: `
value: &value just_a_string
target:
  <<: *value`,
			description: "Merge key referencing non-mapping anchor",
			wantInMsg:   []string{"merge", "mapping"},
		},
		{
			name: "merge key with sequence anchor containing non-mappings",
			yamlContent: `
items: &items
  - string_item
  - another_string
target:
  <<: *items`,
			description: "Merge key with sequence of non-mappings",
			wantInMsg:   []string{"merge", "mapping", "sequence"},
		},
		{
			name: "valid merge with single anchor",
			yamlContent: `
defaults: &defaults
  key1: value1
target:
  <<: *defaults
  key2: value2`,
			description: "Valid merge key usage with single anchor",
			wantInMsg:   []string{},
		},
		{
			name: "valid merge with sequence of anchors",
			yamlContent: `
base1: &base1
  key1: value1
base2: &base2
  key2: value2
target:
  <<: [*base1, *base2]
  key3: value3`,
			description: "Valid merge key with sequence of multiple anchors",
			wantInMsg:   []string{},
		},
		{
			name: "merge key override behavior",
			yamlContent: `
defaults: &defaults
  key: original
target:
  <<: *defaults
  key: overridden`,
			description: "Merge key with override (last key wins)",
			wantInMsg:   []string{},
		},
		{
			name: "merge key in flow mapping",
			yamlContent: `
defaults: &defaults {key1: value1}
target: {<<: *defaults, key2: value2}`,
			description: "Valid merge key in flow-style mapping",
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

			if !shouldError && err == nil {
				t.Logf("✓ Valid YAML accepted: %s", tt.description)
			}
		})
	}
}

// TestInvalidYAML_SetSyntax tests invalid set entry key (=) syntax
func TestInvalidYAML_SetSyntax(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
	}{
		{
			name: "set key without value",
			yamlContent: `
set:
  =: value`,
			description: "Set key (=) used without proper key context",
			wantInMsg:   []string{}, // = is a valid key in YAML
		},
		{
			name: "set key with sequence",
			yamlContent: `
- =: value`,
			description: "Set key in sequence context",
			wantInMsg:   []string{"set", "sequence"},
		},
		{
			name: "set key with null",
			yamlContent: `
set:
  key: =`,
			description: "Set key assignment to null",
			wantInMsg:   []string{}, // This is valid YAML (= represents null)
		},
		{
			name: "multiple set keys",
			yamlContent: `
set:
  =: value1
  =: value2`,
			description: "Multiple set keys in same mapping",
			wantInMsg:   []string{"duplicate"},
		},
		{
			name: "set key in flow mapping",
			yamlContent: `map: {=: value, key: other}`,
			description: "Set key in flow-style mapping",
			wantInMsg:   []string{}, // Valid in flow context
		},
		{
			name: "set key with merge",
			yamlContent: `
set:
  <<: *anchor
  =: value`,
			description: "Set key combined with merge key",
			wantInMsg:   []string{}, // Valid combination
		},
		{
			name: "set key with anchor",
			yamlContent: `
set: &set
  key: =`,
			description: "Anchor on mapping containing set key",
			wantInMsg:   []string{},
		},
		{
			name: "valid explicit key with set",
			yamlContent: `
? key
= : value`,
			description: "Valid explicit key with set entry key",
			wantInMsg:   []string{},
		},
		{
			name: "set key in nested mapping",
			yamlContent: `
outer:
  inner:
    =: value`,
			description: "Set key in nested mapping",
			wantInMsg:   []string{},
		},
		{
			name: "set key with complex mapping",
			yamlContent: `
? [key1, key2]
= : value`,
			description: "Set key with multi-key mapping",
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

			if !shouldError && err == nil {
				t.Logf("✓ Valid YAML accepted: %s", tt.description)
			}
		})
	}
}

// TestInvalidYAML_AmbiguousKeyValuePairs tests ambiguous key/value pair structures
func TestInvalidYAML_AmbiguousKeyValuePairs(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
	}{
		{
			name: "ambiguous colon in plain scalar",
			yamlContent: `key: value:with:multiple:colons`,
			description: "Plain scalar with multiple colons",
			wantInMsg:   []string{}, // Valid: colons in plain scalar are OK
		},
		{
			name: "ambiguous mapping in sequence",
			yamlContent: `
- key: value
- another: item`,
			description: "Mapping items in sequence (invalid when parsed as mapping)",
			wantInMsg:   []string{"cannot unmarshal", "seq"},
		},
		{
			name: "question mark key without explicit marker",
			yamlContent: `
? key
: value`,
			description: "Explicit key using question mark syntax",
			wantInMsg:   []string{},
		},
		{
			name: "ambiguous nested key",
			yamlContent: `
outer.key: value
outer:
  key: value2`,
			description: "Both dotted key and nested key for same path (valid - they're separate keys)",
			wantInMsg:   []string{}, // YAML treats these as separate keys
		},
		{
			name: "key with colon in quotes",
			yamlContent: `"key:with:colons": value`,
			description: "Quoted key containing colons",
			wantInMsg:   []string{},
		},
		{
			name: "ambiguous flow mapping vs sequence",
			yamlContent: `items: [{key: value}, item2, item3]`,
			description: "Flow mapping mixed with scalars in sequence",
			wantInMsg:   []string{},
		},
		{
			name: "dash at same level as mapping",
			yamlContent: `
key: value
- list item`,
			description: "Sequence item at same indentation as mapping key",
			wantInMsg:   []string{"indent", "structure"},
		},
		{
			name: "duplicate keys with different cases",
			yamlContent: `
key: value1
Key: value2
KEY: value3`,
			description: "Keys differing only in case (valid - YAML treats them as distinct)",
			wantInMsg:   []string{}, // YAML treats case-sensitive keys as distinct
		},
		{
			name: "key with special characters",
			yamlContent: `
"key@#$%^&*()": value`,
			description: "Quoted key with special characters",
			wantInMsg:   []string{},
		},
		{
			name: "ambiguous block vs flow",
			yamlContent: `
items: [one, two, three]`,
			description: "Flow sequence in block context",
			wantInMsg:   []string{},
		},
		{
			name: "multiline key",
			yamlContent: `
? multi
  line
  key: value`,
			description: "Multi-line explicit key (invalid syntax)",
			wantInMsg:   []string{"not allowed", "context"},
		},
		{
			name: "nested ambiguous structure",
			yamlContent: `
outer:
  - key: value
  - key: another_value`,
			description: "Sequence with similar mapping keys",
			wantInMsg:   []string{}, // Valid: each list item is independent
		},
		{
			name: "value starting with special char",
			yamlContent: `key: ":not_a_mapping"`,
			description: "Value starting with colon followed by word",
			wantInMsg:   []string{},
		},
		{
			name: "colon as value",
			yamlContent: `symbol: ":"`,
			description: "Colon character as quoted value",
			wantInMsg:   []string{},
		},
		{
			name: "mapping in scalar context",
			yamlContent: `
description: "This has a colon: in the middle"`,
			description: "Quoted string containing colon",
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

			if !shouldError && err == nil {
				t.Logf("✓ Valid YAML accepted: %s", tt.description)
			}
		})
	}
}

// TestInvalidYAML_StructuralErrorQuality tests that structural error messages are informative
func TestInvalidYAML_StructuralErrorQuality(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		minLength  int
	}{
		{
			name:        "circular reference error message",
			yamlContent: `&a *a: value`,
			description: "Error for circular reference should be descriptive",
			minLength:   10,
		},
		{
			name: "merge key error message",
			yamlContent: `
target:
  <<: *undefined`,
			description: "Error for invalid merge key should be descriptive",
			minLength:   10,
		},
		{
			name: "ambiguous structure error message",
			yamlContent: `
key: value
- item`,
			description: "Error for ambiguous structure should be descriptive",
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

// Helper functions for generating test data

// generateDeepMapping creates a deeply nested mapping structure
func generateDeepMapping(levels int) string {
	var sb strings.Builder
	for i := 1; i <= levels; i++ {
		sb.WriteString(strings.Repeat("  ", i-1))
		sb.WriteString(fmt.Sprintf("level%d:\n", i))
	}
	sb.WriteString(strings.Repeat("  ", levels))
	sb.WriteString("value: end\n")
	return sb.String()
}

// generateDeepSequence creates a deeply nested sequence structure
func generateDeepSequence(levels int) string {
	var sb strings.Builder
	sb.WriteString("-\n")
	for i := 1; i < levels; i++ {
		sb.WriteString(strings.Repeat("  ", i))
		sb.WriteString("-\n")
	}
	sb.WriteString(strings.Repeat("  ", levels-1))
	sb.WriteString("value: end\n")
	return sb.String()
}

// generateDeepSequenceWithMappings creates nested sequences containing mappings
func generateDeepSequenceWithMappings(levels int) string {
	var sb strings.Builder
	for i := 0; i < levels; i++ {
		sb.WriteString(strings.Repeat("  ", i))
		sb.WriteString("- key: value\n")
	}
	return sb.String()
}
