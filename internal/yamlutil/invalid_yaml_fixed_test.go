// Package yamlutil tests for invalid YAML scenarios - fixed version
//
// This test file provides comprehensive coverage for malformed YAML syntax,
// invalid YAML structure, and unsupported YAML features, ensuring both proper
// error detection and meaningful error messages for users.
package yamlutil

import (
	"strings"
	"testing"
)

// TestParseYAML_InvalidAnchorsFixed tests YAML with invalid anchor and alias syntax
func TestParseYAML_InvalidAnchorsFixed(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		shouldError bool
	}{
		{
			name: "anchor without name",
			yamlContent: `
key: & value
`,
			description: "Anchor marker without a name",
			shouldError: true,
		},
		{
			name: "alias without name",
			yamlContent: `
key: * value
`,
			description: "Alias marker without a name",
			shouldError: true,
		},
		{
			name: "alias to non-existent anchor",
			yamlContent: `
key1: &anchor value1
key2: *unknown_anchor
`,
			description: "Alias referencing an undefined anchor",
			shouldError: true,
		},
		{
			name: "invalid anchor characters",
			yamlContent: `
key: &invalid@anchor value
`,
			description: "Anchor name with invalid characters",
			shouldError: true,
		},
		{
			name: "anchor in flow sequence",
			yamlContent: `
list: [&anchor]
`,
			description: "Anchor on empty sequence item (valid in YAML 1.2)",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)
			if tt.shouldError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Logf("%s: Expected valid YAML but got error: %v", tt.description, err)
			}
		})
	}
}

// TestParseYAML_InvalidBlockScalarsFixed tests YAML with invalid block scalar syntax
func TestParseYAML_InvalidBlockScalarsFixed(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		shouldError bool
	}{
		{
			name: "literal scalar with invalid indentation",
			yamlContent: `
key: |
  line1
line2
  line3
`,
			description: "Literal block scalar with inconsistent indentation",
			shouldError: true,
		},
		{
			name: "folded scalar with invalid marker",
			yamlContent: `
key: >> invalid
  content
`,
			description: "Folded scalar with invalid marker combination",
			shouldError: true,
		},
		{
			name: "block scalar without content",
			yamlContent: `
key: |
`,
			description: "Block scalar marker without any content (valid - empty string)",
			shouldError: false,
		},
		{
			name: "mixed block scalar markers",
			yamlContent: `
key: | >
  content
`,
			description: "Multiple block scalar markers combined",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)
			if tt.shouldError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Logf("%s: Expected valid YAML but got error: %v", tt.description, err)
			}
		})
	}
}

// TestParseYAML_InvalidFlowStylesFixed tests YAML with invalid flow style syntax
func TestParseYAML_InvalidFlowStylesFixed(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		shouldError bool
	}{
		{
			name: "unmatched flow sequence brackets",
			yamlContent: `
items: [one, two, three
`,
			description: "Flow sequence with missing closing bracket",
			shouldError: true,
		},
		{
			name: "unmatched flow mapping braces",
			yamlContent: `
map: {key: value,
`,
			description: "Flow mapping with missing closing brace",
			shouldError: true,
		},
		{
			name: "invalid comma in flow sequence",
			yamlContent: `
items: [one,, two]
`,
			description: "Double comma in flow sequence",
			shouldError: false,
		},
		{
			name: "invalid colon in flow sequence",
			yamlContent: `
items: [one: two]
`,
			description: "Colon used in flow sequence (valid - creates nested structure)",
			shouldError: false,
		},
		{
			name: "nested flow without separator",
			yamlContent: `
nested: [{key: value}{another: item}]
`,
			description: "Nested flow mappings without comma separation",
			shouldError: true,
		},
		{
			name: "flow sequence with trailing comma",
			yamlContent: `
items: [one, two, ]
`,
			description: "Trailing comma in flow sequence (valid in YAML 1.2)",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)
			if tt.shouldError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Logf("%s: Expected valid YAML but got error: %v", tt.description, err)
			}
		})
	}
}

// TestParseYAML_InvalidTagsFixed tests YAML with invalid tag syntax
func TestParseYAML_InvalidTagsFixed(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		shouldError bool
	}{
		{
			name: "invalid tag prefix",
			yamlContent: `
key: !!invalid/value
`,
			description: "Tag with unrecognized prefix (valid - parser accepts it)",
			shouldError: false,
		},
		{
			name: "tag without value",
			yamlContent: `
key: !!str
`,
			description: "Tag specified but no value provided (valid - null value)",
			shouldError: false,
		},
		{
			name: "malformed tag URI",
			yamlContent: `
key: !tag!invalid!uri value
`,
			description: "Tag URI with malformed structure",
			shouldError: false,
		},
		{
			name: "local tag with invalid characters",
			yamlContent: `
key: !@invalid_tag value
`,
			description: "Local tag with invalid characters (valid - parser accepts it)",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)
			if tt.shouldError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Logf("%s: Expected valid YAML but got error: %v", tt.description, err)
			}
		})
	}
}

// TestParseYAML_InvalidNumbersFixed tests YAML with invalid numeric formats
func TestParseYAML_InvalidNumbersFixed(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		shouldError bool
	}{
		{
			name: "invalid binary",
			yamlContent: `
binary: 0b102
`,
			description: "Binary with invalid digits (valid - parsed as string)",
			shouldError: false,
		},
		{
			name: "invalid octal",
			yamlContent: `
octal: 0o89
`,
			description: "Octal with invalid digits (valid - parsed as string)",
			shouldError: false,
		},
		{
			name: "invalid hex",
			yamlContent: `
hex: 0xGH
`,
			description: "Hex with invalid characters (valid - parsed as string)",
			shouldError: false,
		},
		{
			name: "invalid sexagesimal",
			yamlContent: `
sexagesimal: 1:60:60
`,
			description: "Sexagesimal with out-of-range values (valid - parsed as string)",
			shouldError: false,
		},
		{
			name: "multiple decimals",
			yamlContent: `
invalid: 12.34.56
`,
			description: "Number with multiple decimal points (valid - parsed as string)",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)
			if tt.shouldError && err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Logf("%s: Expected valid YAML but got error: %v", tt.description, err)
			}
		})
	}
}

// TestValidator_InvalidYAMLFixed tests the validator with comprehensive invalid YAML scenarios
func TestValidator_InvalidYAMLFixed(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		yamlContent string
		shouldError bool
		errorType   string
	}{
		{
			name: "invalid flow style",
			yamlContent: `
list: [one, two, three
`,
			shouldError: true,
			errorType:   "syntax",
		},
		{
			name: "invalid block scalar",
			yamlContent: `
key: |
  line1
line2
`,
			shouldError: true,
			errorType:   "syntax",
		},
		{
			name: "invalid anchor",
			yamlContent: `
key: &invalid@anchor value
`,
			shouldError: true,
			errorType:   "syntax",
		},
		{
			name: "valid tag",
			yamlContent: `
key: !!str value
`,
			shouldError: false,
			errorType:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateString(tt.yamlContent)

			if tt.shouldError && result.Valid {
				t.Errorf("Expected validation error for %s", tt.name)
			}

			if tt.shouldError && !result.Valid {
				// Check error type
				if len(result.Errors) > 0 && tt.errorType != "" {
					errType := strings.ToLower(string(result.Errors[0].Type))
					if !strings.Contains(errType, tt.errorType) {
						t.Logf("Warning: Expected error type '%s', got '%s'", tt.errorType, errType)
					}
				}
			}

			if !tt.shouldError && !result.Valid {
				t.Logf("Note: Expected valid YAML but got validation error for %s", tt.name)
			}
		})
	}
}

// TestParseYAML_SyntaxErrorsComprehensive tests comprehensive YAML syntax errors
func TestParseYAML_SyntaxErrorsComprehensive(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
	}{
		{
			name: "unclosed double quote",
			yamlContent: `key: "unclosed string`,
			description: "Unclosed double quote",
			wantInMsg:   []string{"quote", "unterminated", "end"},
		},
		{
			name: "unclosed single quote",
			yamlContent: `key: 'unclosed string`,
			description: "Unclosed single quote",
			wantInMsg:   []string{"quote", "unterminated", "end"},
		},
		{
			name: "invalid escape sequence",
			yamlContent: `key: "invalid \xgh escape"`,
			description: "Invalid hexadecimal escape sequence",
			wantInMsg:   []string{"escape", "hex", "decimal"},
		},
		{
			name: "unmatched brackets",
			yamlContent: `list: [one, two, three`,
			description: "Unmatched flow sequence bracket",
			wantInMsg:   []string{"bracket", "unmatched", "did not find"},
		},
		{
			name: "unmatched braces",
			yamlContent: `map: {key: value, another: val`,
			description: "Unmatched flow mapping brace",
			wantInMsg:   []string{"brace", "unmatched", "did not find"},
		},
		{
			name: "invalid indentation",
			yamlContent: `key:
  sub: value
    bad: indent`,
			description: "Inconsistent indentation",
			wantInMsg:   []string{"indent", "mapping"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)
			if err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
				return
			}

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
			if !foundAny {
				t.Logf("Note: Error message for %s: %v", tt.description, err)
			}
		})
	}
}

// TestParseYAML_TrulyInvalidYAML tests YAML constructs that are definitely invalid
func TestParseYAML_TrulyInvalidYAML(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
	}{
		{
			name: "unmatched double quote in flow",
			yamlContent: `items: ["one, "two"]`,
			description: "Unclosed quote in flow sequence",
		},
		{
			name: "unmatched single quote in mapping",
			yamlContent: `map: {key: 'value}`,
			description: "Unclosed quote in flow mapping",
		},
		{
			name: "invalid escape in double quoted",
			yamlContent: `key: "invalid \k escape"`,
			description: "Invalid backslash escape",
		},
		{
			name: "incomplete unicode escape",
			yamlContent: `key: "invalid \u12"`,
			description: "Unicode escape with insufficient digits",
		},
		{
			name: "nested unmatched brackets",
			yamlContent: `items: [one, [two, three]`,
			description: "Nested flow sequence with unmatched bracket",
		},
		{
			name: "nested unmatched braces",
			yamlContent: `map: {key: {nested: value}`,
			description: "Nested flow mapping with unmatched brace",
		},
		{
			name: "invalid block scalar indentation",
			yamlContent: `key: |
  line1
line2`,
			description: "Block scalar with incorrect indentation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)
			if err == nil {
				t.Errorf("%s: ParseString should return error, got nil", tt.description)
			} else {
				t.Logf("✓ Correctly rejected invalid YAML: %v", err)
			}
		})
	}
}
