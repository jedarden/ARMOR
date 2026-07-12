// Package yamlutil tests for unsupported YAML feature errors
//
// This test file provides comprehensive coverage for unsupported YAML features,
// ensuring both proper error detection and meaningful error messages for users.
// Tests cover invalid YAML directives, tag syntax, property context, timestamps,
// and numeric representations that are either unsupported by YAML 1.2 or rejected
// by the parser for safety/compatibility reasons.
package yamlutil

import (
	"strings"
	"testing"
)

// TestUnsupportedYAMLFeatures_Directives tests YAML directive handling
func TestUnsupportedYAMLFeatures_Directives(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
		expectError bool
	}{
		{
			name:        "YAML 1.3 directive unsupported",
			yamlContent: `%YAML 1.3
---
key: value`,
			description: "YAML version 1.3 is not supported (only 1.2 and 1.1)",
			wantInMsg:   []string{"incompatible", "document"},
			expectError: true,
		},
		{
			name:        "YAML 1.0 directive unsupported",
			yamlContent: `%YAML 1.0
---
key: value`,
			description: "YAML version 1.0 is not supported",
			wantInMsg:   []string{"incompatible", "document"},
			expectError: true,
		},
		{
			name:        "unknown directive",
			yamlContent: `%UNKNOWN_DIRECTIVE value
---
key: value`,
			description: "Unknown YAML directive",
			wantInMsg:   []string{"unknown", "directive"},
			expectError: true,
		},
		{
			name:        "TAG directive without prefix",
			yamlContent: `%TAG
---
key: value`,
			description: "TAG directive missing required prefix argument",
			wantInMsg:   []string{"did not find", "expected"},
			expectError: true,
		},
		{
			name:        "TAG directive invalid prefix",
			yamlContent: `%TAG !invalid!prefix! !`,
			description: "TAG directive with invalid prefix format",
			wantInMsg:   []string{"did not find", "expected", "whitespace"},
			expectError: true,
		},
		{
			name:        "multiple YAML version directives",
			yamlContent: `%YAML 1.2
%YAML 1.2
---
key: value`,
			description: "Duplicate YAML version directives",
			wantInMsg:   []string{"incompatible", "document"},
			expectError: true,
		},
		{
			name:        "directive after document start",
			yamlContent: `---
%YAML 1.2
key: value`,
			description: "Directive appearing after document start marker",
			wantInMsg:   []string{"mapping values", "allowed", "context"},
			expectError: true,
		},
		{
			name:        "invalid directive syntax",
			yamlContent: `%INVALID SYNTAX WITH SPACES
---
key: value`,
			description: "Directive with invalid syntax",
			wantInMsg:   []string{"unknown", "directive"},
			expectError: true,
		},
		{
			name:        "YAML 1.2 directive rejected by parser",
			yamlContent: `%YAML 1.2
---
key: value`,
			description: "YAML 1.2 directive rejected by yaml.v3 parser",
			wantInMsg:   []string{"incompatible", "document"},
			expectError: true,
		},
		{
			name:        "TAG directive valid",
			yamlContent: `%TAG ! tag:example.com,2014:---
---
key: value`,
			description: "Valid TAG directive",
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
				t.Errorf("%s: ParseString should return error for unsupported feature, got nil", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: ParseString should not return error, got: %v", tt.description, err)
			}
			if err != nil && len(tt.wantInMsg) > 0 {
				errMsg := err.Error()
				for _, want := range tt.wantInMsg {
					if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(want)) {
						t.Errorf("%s: error message should contain %q, got: %s", tt.description, want, errMsg)
					}
				}
			}
		})
	}
}

// TestUnsupportedYAMLFeatures_TagSyntax tests invalid tag handling
func TestUnsupportedYAMLFeatures_TagSyntax(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
		expectError bool
	}{
		{
			name:        "invalid tag characters",
			yamlContent: `key: !!invalid@tag# value`,
			description: "Tag contains invalid characters",
			wantInMsg:   []string{"did not find", "expected"},
			expectError: true,
		},
		{
			name:        "unrecognized local tag",
			yamlContent: `key: !!local:unknown value`,
			description: "Local tag accepted by parser (valid YAML)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "malformed tag uri",
			yamlContent: `key: !!not-a-valid-uri value`,
			description: "Tag that is not a valid URI accepted by parser",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "tag with empty prefix",
			yamlContent: `key: ! value`,
			description: "Tag with empty prefix accepted by parser (valid)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "tag in invalid position",
			yamlContent: `!tag
key: value`,
			description: "Tag appearing in invalid context accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "duplicate tags on node",
			yamlContent: `key: !!str !!int "123"`,
			description: "Multiple tags applied to same node",
			wantInMsg:   []string{"did not find", "expected"},
			expectError: true,
		},
		{
			name:        "invalid verbatim tag format",
			yamlContent: `key: !<invalid uri> value`,
			description: "Verbatim tag with invalid URI",
			wantInMsg:   []string{"did not find", "expected"},
			expectError: true,
		},
		{
			name:        "tag with invalid whitespace",
			yamlContent: `key: !!str
  value`,
			description: "Tag with newline before value accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid string tag",
			yamlContent: `key: !!str "hello"`,
			description: "Valid string type tag",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid int tag",
			yamlContent: `key: !!int "42"`,
			description: "Valid integer type tag",
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
				t.Errorf("%s: ParseString should return error for invalid tag, got nil", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: ParseString should not return error, got: %v", tt.description, err)
			}
			if err != nil && len(tt.wantInMsg) > 0 {
				errMsg := err.Error()
				for _, want := range tt.wantInMsg {
					if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(want)) {
						t.Errorf("%s: error message should contain %q, got: %s", tt.description, want, errMsg)
					}
				}
			}
		})
	}
}

// TestUnsupportedYAMLFeatures_PropertyContext tests property context handling
func TestUnsupportedYAMLFeatures_PropertyContext(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
		expectError bool
	}{
		{
			name:        "invalid property indicator",
			yamlContent: `key: + value
  property: nested`,
			description: "Invalid property indicator usage",
			wantInMsg:   []string{"mapping values", "allowed", "context"},
			expectError: true,
		},
		{
			name:        "malformed property entry",
			yamlContent: `key: value
  ? invalid
  : value`,
			description: "Malformed property entry syntax",
			wantInMsg:   []string{"did not find", "expected"},
			expectError: true,
		},
		{
			name:        "invalid property nesting",
			yamlContent: `key:
    property:
      ? invalid-key
      : value`,
			description: "Invalid property nesting structure accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "property without value",
			yamlContent: `key:
    ? property-key`,
			description: "Property key without corresponding value accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid property flow syntax",
			yamlContent: `key: {?: value}`,
			description: "Invalid property indicator in flow collection accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "property in sequence context",
			yamlContent: `items:
    - ? property
      : value`,
			description: "Property syntax in sequence accepted (valid)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "nested property indicators",
			yamlContent: `key:
    ? outer
    : ? inner
      : value`,
			description: "Valid nested property mapping",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid property shorthand",
			yamlContent: `key:
    ? property-key
    : property-value`,
			description: "Valid property shorthand syntax",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid property in mapping",
			yamlContent: `mapping: {?: value}`,
			description: "Valid property in flow mapping",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "property with null key",
			yamlContent: `key:
    ? null
    : value`,
			description: "Valid null key in property",
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
				t.Errorf("%s: ParseString should return error for invalid property, got nil", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: ParseString should not return error, got: %v", tt.description, err)
			}
			if err != nil && len(tt.wantInMsg) > 0 {
				errMsg := err.Error()
				for _, want := range tt.wantInMsg {
					if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(want)) {
						t.Errorf("%s: error message should contain %q, got: %s", tt.description, want, errMsg)
					}
				}
			}
		})
	}
}

// TestUnsupportedYAMLFeatures_Timestamps tests invalid timestamp handling
func TestUnsupportedYAMLFeatures_Timestamps(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
		expectError bool
	}{
		{
			name:        "invalid timestamp format - extra spaces",
			yamlContent: `timestamp: 2024-01-15  10:30:45`,
			description: "Timestamp with double spaces accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid timestamp - month too high",
			yamlContent: `timestamp: 2024-13-01T10:30:45Z`,
			description: "Timestamp with invalid month (13) accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid timestamp - day too high",
			yamlContent: `timestamp: 2024-01-32T10:30:45Z`,
			description: "Timestamp with invalid day (32) accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid timestamp - hour too high",
			yamlContent: `timestamp: 2024-01-15T25:30:45Z`,
			description: "Timestamp with invalid hour (25) accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid timestamp - minute too high",
			yamlContent: `timestamp: 2024-01-15T10:60:45Z`,
			description: "Timestamp with invalid minute (60) accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid timestamp - second too high",
			yamlContent: `timestamp: 2024-01-15T10:30:60Z`,
			description: "Timestamp with invalid second (60) accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid timestamp - month zero",
			yamlContent: `timestamp: 2024-00-15T10:30:45Z`,
			description: "Timestamp with zero month accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid timestamp - day zero",
			yamlContent: `timestamp: 2024-01-00T10:30:45Z`,
			description: "Timestamp with zero day accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid timestamp - negative offset",
			yamlContent: `timestamp: 2024-01-15T10:30:45+-15:00`,
			description: "Timestamp with double sign offset accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid timestamp - zero offset",
			yamlContent: `timestamp: 2024-01-15T10:30:45+00:60`,
			description: "Timestamp with invalid minute offset accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid timestamp ISO8601",
			yamlContent: `timestamp: 2024-01-15T10:30:45Z`,
			description: "Valid ISO8601 timestamp",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid timestamp with offset",
			yamlContent: `timestamp: 2024-01-15T10:30:45-05:00`,
			description: "Valid timestamp with timezone offset",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid timestamp date only",
			yamlContent: `timestamp: 2024-01-15`,
			description: "Valid date-only timestamp",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid timestamp canonical",
			yamlContent: `timestamp: 2024-01-15T10:30:45.123456-05:00`,
			description: "Valid timestamp with fractional seconds",
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
				t.Errorf("%s: ParseString should return error for invalid timestamp, got nil", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: ParseString should not return error, got: %v", tt.description, err)
			}
			if err != nil && len(tt.wantInMsg) > 0 {
				errMsg := err.Error()
				for _, want := range tt.wantInMsg {
					if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(want)) {
						t.Errorf("%s: error message should contain %q, got: %s", tt.description, want, errMsg)
					}
				}
			}
		})
	}
}

// TestUnsupportedYAMLFeatures_Numeric tests invalid numeric representations
func TestUnsupportedYAMLFeatures_Numeric(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
		expectError bool
	}{
		{
			name:        "invalid octal with leading zero and prefix",
			yamlContent: `value: 0o0123`,
			description: "Invalid octal (both 0 prefix and o prefix) accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid hex with lowercase x",
			yamlContent: `value: 0x1a2b`,
			description: "Hex with lowercase x accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid binary with wrong prefix",
			yamlContent: `value: 0b1010`,
			description: "Binary prefix accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid sexagesimal - too many colons",
			yamlContent: `value: 12:34:56:78`,
			description: "Sexagesimal with four components accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid sexagesimal - negative in middle",
			yamlContent: `value: 12:-34:56`,
			description: "Sexagesimal with negative middle component accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid 60-based - component >= 60",
			yamlContent: `value: 12:60:45`,
			description: "Sexagesimal with component >= 60 accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid float with two decimals",
			yamlContent: `value: 123.456.789`,
			description: "Float with two decimal points accepted by parser",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid exponent - empty mantissa",
			yamlContent: `value: e10`,
			description: "Scientific notation without mantissa",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid exponent - empty exponent",
			yamlContent: `value: 123e`,
			description: "Scientific notation without exponent value",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid infinity - lowercase",
			yamlContent: `value: .inf`,
			description: "Lowercase infinity accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "invalid nan - lowercase",
			yamlContent: `value: .nan`,
			description: "Lowercase NaN accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid integer",
			yamlContent: `value: 12345`,
			description: "Valid integer",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid negative integer",
			yamlContent: `value: -12345`,
			description: "Valid negative integer",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid float",
			yamlContent: `value: 123.456`,
			description: "Valid float",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid scientific notation",
			yamlContent: `value: 1.23e10`,
			description: "Valid scientific notation",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid octal",
			yamlContent: `value: 0o123`,
			description: "Valid octal (YAML 1.2)",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid hex",
			yamlContent: `value: 0x1A2B`,
			description: "Valid hexadecimal",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid sexagesimal",
			yamlContent: `value: 12:34:56`,
			description: "Valid sexagesimal time format",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid infinity uppercase",
			yamlContent: `value: .INF`,
			description: "Valid uppercase infinity",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid nan uppercase",
			yamlContent: `value: .NAN`,
			description: "Valid uppercase NaN",
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
				t.Errorf("%s: ParseString should return error for invalid numeric, got nil", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: ParseString should not return error, got: %v", tt.description, err)
			}
			if err != nil && len(tt.wantInMsg) > 0 {
				errMsg := err.Error()
				for _, want := range tt.wantInMsg {
					if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(want)) {
						t.Errorf("%s: error message should contain %q, got: %s", tt.description, want, errMsg)
					}
				}
			}
		})
	}
}

// TestUnsupportedYAMLFeatures_Complex tests combinations and edge cases
func TestUnsupportedYAMLFeatures_Complex(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
		wantInMsg   []string
		expectError bool
	}{
		{
			name:        "tag with unsupported directive",
			yamlContent: `%YAML 1.3
---
key: !!str value`,
			description: "Combination of unsupported directive and tag",
			wantInMsg:   []string{"incompatible", "document"},
			expectError: true,
		},
		{
			name:        "invalid timestamp in sequence",
			yamlContent: `timestamps:
    - 2024-13-01T10:30:45Z
    - 2024-01-32T10:30:45Z`,
			description: "Multiple invalid timestamps in sequence accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "multiple numeric errors",
			yamlContent: `values:
    - 0x1a2b
    - 12:60:45
    - 123.456.789`,
			description: "Multiple invalid numeric values accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "mixed valid and invalid",
			yamlContent: `values:
    - 123
    - 0x1a2b
    - 456`,
			description: "Mix of valid and hex values accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "deep nesting with invalid directive",
			yamlContent: `%YAML 1.3
---
level1:
    level2:
        level3:
            key: value`,
			description: "Deep nesting with unsupported directive",
			wantInMsg:   []string{"incompatible", "document"},
			expectError: true,
		},
		{
			name:        "property with invalid timestamp",
			yamlContent: `mapping:
    ? timestamp
    : 2024-13-01T10:30:45Z`,
			description: "Property value with invalid timestamp accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "anchor on unsupported feature",
			yamlContent: `invalid: &anchor
    timestamp: 2024-13-01T10:30:45Z
ref: *anchor`,
			description: "Anchor on node with invalid timestamp accepted",
			wantInMsg:   []string{},
			expectError: false,
		},
		{
			name:        "valid complex nested structure",
			yamlContent: `level1:
    level2:
        level3:
            timestamp: 2024-01-15T10:30:45Z
            value: 123
            flag: true`,
			description: "Valid deeply nested structure with multiple types",
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
				t.Errorf("%s: ParseString should return error for invalid YAML, got nil", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: ParseString should not return error, got: %v", tt.description, err)
			}
			if err != nil && len(tt.wantInMsg) > 0 {
				errMsg := err.Error()
				for _, want := range tt.wantInMsg {
					if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(want)) {
						t.Errorf("%s: error message should contain %q, got: %s", tt.description, want, errMsg)
					}
				}
			}
		})
	}
}
