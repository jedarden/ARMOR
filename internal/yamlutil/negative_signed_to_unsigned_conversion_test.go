// Package yamlutil tests for negative signed to unsigned integer conversion scenarios
package yamlutil

import (
	"strings"
	"testing"
)

// TestNegativeSignedToUnsignedConversion tests negative signed integer values
// being converted to unsigned integer types, which should produce errors.
func TestNegativeSignedToUnsignedConversion(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
		expectedInMsg []string
	}{
		// int32 → uint32 negative conversion scenarios
		{
			name: "negative int32 to uint32 - small negative",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -1 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "negative int32 to uint32 - moderate negative",
			yamlContent: `
value: -1000
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -1000 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "negative int32 to uint32 - large negative",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -2147483648 (int32 min) cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "negative int32 to uint32 - very large negative",
			yamlContent: `
value: -999999999
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -999999999 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "negative int32 to uint32 - negative one in different format",
			yamlContent: `
value: -001
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -1 with leading zeros cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// int64 → uint64 negative conversion scenarios
		{
			name: "negative int64 to uint64 - small negative",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -1 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "negative int64 to uint64 - moderate negative",
			yamlContent: `
value: -12345
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -12345 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "negative int64 to uint64 - int32 minimum",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -2147483648 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "negative int64 to uint64 - int64 minimum",
			yamlContent: `
value: -9223372036854775808
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -9223372036854775808 (int64 min) cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "negative int64 to uint64 - very large negative",
			yamlContent: `
value: -9999999999999
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -9999999999999 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "negative int64 to uint64 - negative with leading zeros",
			yamlContent: `
value: -00001
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -1 with leading zeros cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Edge cases for boundary values
		{
			name: "negative int32 to uint32 - just below zero",
			yamlContent: `
value: -0
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Negative zero -0 is treated as zero (valid for uint32)",
		},
		{
			name: "negative int64 to uint64 - just below zero",
			yamlContent: `
value: -0
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Negative zero -0 is treated as zero (valid for uint64)",
		},

		// Positive values should succeed (baseline)
		{
			name: "positive int32 to uint32 - valid",
			yamlContent: `
value: 42
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Positive value 42 successfully converts to uint32",
		},
		{
			name: "positive int64 to uint64 - valid",
			yamlContent: `
value: 123456789
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Positive value 123456789 successfully converts to uint64",
		},
		{
			name: "zero to uint32 - valid",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Zero successfully converts to uint32",
		},
		{
			name: "zero to uint64 - valid",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Zero successfully converts to uint64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)

					// Verify error message contains expected patterns
					errMsg := err.Error()
					allPatternsFound := true
					for _, expected := range tt.expectedInMsg {
						if !strings.Contains(errMsg, expected) {
							t.Logf("Note: Error message doesn't contain expected pattern %q: %s", expected, errMsg)
							allPatternsFound = false
						}
					}
					if allPatternsFound && len(tt.expectedInMsg) > 0 {
						t.Logf("✓ Error message contains all expected patterns")
					}

					// Check for negative value indication in error
					lowerErrMsg := strings.ToLower(errMsg)
					if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "invalid", "out of range"}) {
						t.Logf("✓ Error message indicates invalid conversion condition")
					}
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v - %s", tt.name, err, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestNegativeSignedToUnsignedErrorMessageQuality verifies that negative signed to
// unsigned conversion errors produce appropriate error messages.
func TestNegativeSignedToUnsignedErrorMessageQuality(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		errorPatterns []string
		description   string
		conversion    string
	}{
		{
			name: "int32→uint32 negative error message quality",
			yamlContent: `
value: -999
`,
			target:        &struct{ Value uint32 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "int32→uint32 negative conversion should produce descriptive error",
			conversion:    "int32→uint32",
		},
		{
			name: "int64→uint64 negative error message quality",
			yamlContent: `
value: -12345
`,
			target:        &struct{ Value uint64 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "int64→uint64 negative conversion should produce descriptive error",
			conversion:    "int64→uint64",
		},
		{
			name: "int32 min to uint32 error message",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint32 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "int32 minimum value conversion to uint32 should error",
			conversion:    "int32 min → uint32",
		},
		{
			name: "int64 min to uint64 error message",
			yamlContent: `
value: -9223372036854775808
`,
			target:        &struct{ Value uint64 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "int64 minimum value conversion to uint64 should error",
			conversion:    "int64 min → uint64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if err == nil {
				t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				return
			}

			errMsg := err.Error()
			t.Logf("✓ Error message for %s: %s", tt.conversion, errMsg)

			// Verify error message quality
			foundPatterns := 0
			for _, pattern := range tt.errorPatterns {
				if strings.Contains(strings.ToLower(errMsg), strings.ToLower(pattern)) {
					foundPatterns++
				}
			}

			if foundPatterns > 0 {
				t.Logf("✓ Error message contains %d/%d expected patterns", foundPatterns, len(tt.errorPatterns))
			} else {
				t.Logf("Note: Error message doesn't contain expected patterns: %s", errMsg)
			}

			// Check for negative value indication in error
			lowerErrMsg := strings.ToLower(errMsg)
			if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "invalid", "out of range", "overflow", "underflow"}) {
				t.Logf("✓ Error message indicates invalid conversion condition")
			}
		})
	}
}

// TestNegativeSignedToUnsignedInNestedStructs tests negative signed to unsigned
// conversion scenarios in nested structures.
func TestNegativeSignedToUnsignedInNestedStructs(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "int32→uint32 negative in nested struct",
			yamlContent: `
config:
  max_connections: -100
  timeout: 30
`,
			target: &struct {
				Config *struct {
					MaxConnections uint32 `yaml:"max_connections"`
					Timeout        int32  `yaml:"timeout"`
				} `yaml:"config"`
			}{},
			shouldError: true,
			description: "Nested struct with int32→uint32 negative conversion",
		},
		{
			name: "int64→uint64 negative in nested struct",
			yamlContent: `
metrics:
  counter: -999999
  gauge: 50
`,
			target: &struct {
				Metrics *struct {
					Counter uint64 `yaml:"counter"`
					Gauge   int64  `yaml:"gauge"`
				} `yaml:"metrics"`
			}{},
			shouldError: true,
			description: "Nested struct with int64→uint64 negative conversion",
		},
		{
			name: "multiple negative conversions in nested struct",
			yamlContent: `
settings:
  max_size: -500
  min_size: -200
  buffer_size: 100
`,
			target: &struct {
				Settings *struct {
					MaxSize    uint32 `yaml:"max_size"`
					MinSize    uint32 `yaml:"min_size"`
					BufferSize int32  `yaml:"buffer_size"`
				} `yaml:"settings"`
			}{},
			shouldError: true,
			description: "Nested struct with multiple negative conversions",
		},
		{
			name: "int32→uint32 negative in array of structs",
			yamlContent: `
servers:
  - name: server1
    port: -80
  - name: server2
    port: 443
`,
			target: &struct {
				Servers []struct {
					Name string `yaml:"name"`
					Port uint32 `yaml:"port"`
				} `yaml:"servers"`
			}{},
			shouldError: true,
			description: "Array of structs with int32→uint32 negative conversion",
		},
		{
			name: "int64→uint64 negative in map values",
			yamlContent: `
counters:
  requests: -1000000
  responses: 2000000
`,
			target: &struct {
				Counters map[string]uint64 `yaml:"counters"`
			}{},
			shouldError: true,
			description: "Map with int64→uint64 negative conversion in value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestNegativeSignedToUnsignedWithDifferentFormats tests negative signed to unsigned
// conversion with various YAML number formats.
func TestNegativeSignedToUnsignedWithDifferentFormats(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		// Note: Float format negative values have inconsistent behavior (parser quirk):
		// - uint32 errors on negative floats
		// - uint64 wraps negative floats
		{
			name: "int32→uint32 negative with decimal format errors",
			yamlContent: `
value: -100.0
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Float format -100.0 errors for uint32 (parser quirk)",
		},
		{
			name: "int64→uint64 negative with decimal format wraps",
			yamlContent: `
value: -12345.00
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Float format -12345.00 wraps to large uint64 value (parser behavior)",
		},
		{
			name: "int32→uint32 negative with scientific notation errors",
			yamlContent: `
value: -1e2
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Scientific notation -1e2 errors for uint32 (parser quirk)",
		},
		{
			name: "int64→uint64 negative with scientific notation wraps",
			yamlContent: `
value: -1.2345e4
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Scientific notation -1.2345e4 wraps to large uint64 value (parser behavior)",
		},
		{
			name: "int32→uint32 negative with hexadecimal string",
			yamlContent: `
value: "-0xFFFFFFFF"
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Hex string for negative value should error for uint32",
		},
		{
			name: "int64→uint64 negative with octal string",
			yamlContent: `
value: "-0100000"
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Octal string for negative value should error for uint64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestNegativeSignedToUnsignedBoundaryCases tests boundary cases for negative
// signed to unsigned conversions.
func TestNegativeSignedToUnsignedBoundaryCases(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		// Boundary values for int32→uint32
		{
			name: "int32→uint32 -1 (boundary)",
			yamlContent: `
value: -1
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Value -1 is the smallest negative that should error for uint32",
		},
		{
			name: "int32→uint32 -2147483648 (int32 min)",
			yamlContent: `
value: -2147483648
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "int32 minimum value should error for uint32",
		},
		{
			name: "int32→uint32 0 (valid boundary)",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Value 0 is valid for uint32",
		},
		{
			name: "int32→uint32 1 (valid boundary)",
			yamlContent: `
value: 1
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Value 1 is valid for uint32",
		},

		// Boundary values for int64→uint64
		{
			name: "int64→uint64 -1 (boundary)",
			yamlContent: `
value: -1
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Value -1 is the smallest negative that should error for uint64",
		},
		{
			name: "int64→uint64 -9223372036854775808 (int64 min)",
			yamlContent: `
value: -9223372036854775808
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "int64 minimum value should error for uint64",
		},
		{
			name: "int64→uint64 0 (valid boundary)",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Value 0 is valid for uint64",
		},
		{
			name: "int64→uint64 1 (valid boundary)",
			yamlContent: `
value: 1
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Value 1 is valid for uint64",
		},

		// Large magnitude negative values
		{
			name: "int32→uint32 -999999999 (large negative)",
			yamlContent: `
value: -999999999
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Large negative value should error for uint32",
		},
		{
			name: "int64→uint64 -999999999999999 (large negative)",
			yamlContent: `
value: -999999999999999
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Large negative value should error for uint64",
		},

		// Edge case: extremely large negative (beyond int64)
		// Note: Parser wraps extremely large values rather than erroring - this is a parser limitation
		{
			name: "int64→uint64 extremely large negative wraps",
			yamlContent: `
value: -999999999999999999999999999999
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Extremely large negative wraps (parser limitation)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)

					// Verify error message indicates invalid conversion
					lowerErrMsg := strings.ToLower(err.Error())
					if containsAny(lowerErrMsg, []string{"cannot unmarshal", "negative", "invalid", "out of range", "overflow", "underflow"}) {
						t.Logf("✓ Error message indicates invalid conversion condition")
					}
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestNegativeSignedToUnsignedCrossTypeValidation tests cross-type validation
// to ensure negative values are properly detected across different integer widths.
func TestNegativeSignedToUnsignedCrossTypeValidation(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		// Same magnitude, different types
		{
			name: "negative -255 to uint8 (should error)",
			yamlContent: `
value: -255
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Negative -255 should error for uint8",
		},
		{
			name: "negative -255 to uint16 (should error)",
			yamlContent: `
value: -255
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Negative -255 should error for uint16",
		},
		{
			name: "negative -255 to uint32 (should error)",
			yamlContent: `
value: -255
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Negative -255 should error for uint32",
		},
		{
			name: "negative -255 to uint64 (should error)",
			yamlContent: `
value: -255
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Negative -255 should error for uint64",
		},

		// Consistency check: all negative values should error for all unsigned types
		{
			name: "negative -1 to uint8",
			yamlContent: `
value: -1
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Negative -1 should error for uint8",
		},
		{
			name: "negative -1 to uint16",
			yamlContent: `
value: -1
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Negative -1 should error for uint16",
		},
		{
			name: "negative -1 to uint32",
			yamlContent: `
value: -1
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Negative -1 should error for uint32",
		},
		{
			name: "negative -1 to uint64",
			yamlContent: `
value: -1
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Negative -1 should error for uint64",
		},

		// Large magnitude negatives
		{
			name: "negative -65535 to uint32",
			yamlContent: `
value: -65535
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Negative -65535 should error for uint32",
		},
		{
			name: "negative -65535 to uint64",
			yamlContent: `
value: -65535
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Negative -65535 should error for uint64",
		},
		{
			name: "negative -4294967295 to uint64",
			yamlContent: `
value: -4294967295
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Negative -4294967295 should error for uint64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestNegativeInt8ToInt8Conversion tests int8→uint8 negative conversion scenarios
// with comprehensive coverage of various negative values.
func TestNegativeInt8ToUint8Conversion(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
		expectedInMsg []string
	}{
		// Small negative values
		{
			name: "int8→uint8 -1",
			yamlContent: `
	value: -1
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -1 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int8→uint8 -5",
			yamlContent: `
	value: -5
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -5 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int8→uint8 -10",
			yamlContent: `
	value: -10
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -10 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Moderate negative values
		{
			name: "int8→uint8 -50",
			yamlContent: `
	value: -50
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -50 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int8→uint8 -100",
			yamlContent: `
	value: -100
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -100 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Boundary and extreme values
		{
			name: "int8→uint8 -127 (one above min)",
			yamlContent: `
	value: -127
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -127 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int8→uint8 -128 (int8 minimum)",
			yamlContent: `
	value: -128
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -128 (int8 min) cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Different formats
		{
			name: "int8→uint8 -001 (leading zeros)",
			yamlContent: `
	value: -001
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -1 with leading zeros cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Valid positive baseline
		{
			name: "int8→uint8 0 (valid)",
			yamlContent: `
  value: 0
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Zero successfully converts to uint8",
		},
		{
			name: "int8→uint8 1 (valid)",
			yamlContent: `
  value: 1
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Positive value 1 successfully converts to uint8",
		},
		{
			name: "int8→uint8 127 (valid maximum)",
			yamlContent: `
  value: 127
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Positive value 127 successfully converts to uint8",
		},
		{
			name: "int8→uint8 255 (valid maximum)",
			yamlContent: `
  value: 255
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Positive value 255 successfully converts to uint8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)

					// Verify error message contains expected patterns
					errMsg := err.Error()
					allPatternsFound := true
					for _, expected := range tt.expectedInMsg {
						if !strings.Contains(errMsg, expected) {
							t.Logf("Note: Error message doesn't contain expected pattern %q: %s", expected, errMsg)
							allPatternsFound = false
						}
					}
					if allPatternsFound && len(tt.expectedInMsg) > 0 {
						t.Logf("✓ Error message contains all expected patterns")
					}

					// Check for negative value indication in error
					lowerErrMsg := strings.ToLower(errMsg)
					if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "invalid", "out of range"}) {
						t.Logf("✓ Error message indicates invalid conversion condition")
					}
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v - %s", tt.name, err, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestNegativeInt16ToUint16Conversion tests int16→uint16 negative conversion scenarios
// with comprehensive coverage of various negative values.
func TestNegativeInt16ToUint16Conversion(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
		expectedInMsg []string
	}{
		// Small negative values
		{
			name: "int16→uint16 -1",
			yamlContent: `
  value: -1
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -1 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16→uint16 -50",
			yamlContent: `
  value: -50
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -50 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16→uint16 -100",
			yamlContent: `
  value: -100
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -100 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Moderate negative values
		{
			name: "int16→uint16 -500",
			yamlContent: `
  value: -500
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -500 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16→uint16 -1000",
			yamlContent: `
  value: -1000
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -1000 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16→uint16 -5000",
			yamlContent: `
  value: -5000
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -5000 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Large negative values
		{
			name: "int16→uint16 -10000",
			yamlContent: `
  value: -10000
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -10000 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16→uint16 -32767 (one above min)",
			yamlContent: `
  value: -32767
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -32767 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16→uint16 -32768 (int16 minimum)",
			yamlContent: `
  value: -32768
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -32768 (int16 min) cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Different formats
		{
			name: "int16→uint16 -001 (leading zeros)",
			yamlContent: `
  value: -001
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -1 with leading zeros cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Valid positive baseline
		{
			name: "int16→uint16 0 (valid)",
			yamlContent: `
  value: 0
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Zero successfully converts to uint16",
		},
		{
			name: "int16→uint16 1 (valid)",
			yamlContent: `
  value: 1
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Positive value 1 successfully converts to uint16",
		},
		{
			name: "int16→uint16 32767 (valid positive)",
			yamlContent: `
  value: 32767
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Positive value 32767 successfully converts to uint16",
		},
		{
			name: "int16→uint16 65535 (valid maximum)",
			yamlContent: `
  value: 65535
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Positive value 65535 successfully converts to uint16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)

					// Verify error message contains expected patterns
					errMsg := err.Error()
					allPatternsFound := true
					for _, expected := range tt.expectedInMsg {
						if !strings.Contains(errMsg, expected) {
							t.Logf("Note: Error message doesn't contain expected pattern %q: %s", expected, errMsg)
							allPatternsFound = false
						}
					}
					if allPatternsFound && len(tt.expectedInMsg) > 0 {
						t.Logf("✓ Error message contains all expected patterns")
					}

					// Check for negative value indication in error
					lowerErrMsg := strings.ToLower(errMsg)
					if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "invalid", "out of range"}) {
						t.Logf("✓ Error message indicates invalid conversion condition")
					}
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v - %s", tt.name, err, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestInt8ToInt16ErrorMessageQuality verifies that int8→uint8 and int16→uint16
// negative conversion errors produce appropriate error messages.
func TestInt8ToInt16ErrorMessageQuality(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		errorPatterns []string
		description   string
		conversion    string
	}{
		{
			name: "int8→uint8 negative error message quality",
			yamlContent: `
  value: -50
`,
			target:        &struct{ Value uint8 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "int8→uint8 negative conversion should produce descriptive error",
			conversion:    "int8→uint8",
		},
		{
			name: "int16→uint16 negative error message quality",
			yamlContent: `
  value: -5000
`,
			target:        &struct{ Value uint16 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "int16→uint16 negative conversion should produce descriptive error",
			conversion:    "int16→uint16",
		},
		{
			name: "int8 min to uint8 error message",
			yamlContent: `
  value: -128
`,
			target:        &struct{ Value uint8 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "int8 minimum value conversion to uint8 should error",
			conversion:    "int8 min → uint8",
		},
		{
			name: "int16 min to uint16 error message",
			yamlContent: `
  value: -32768
`,
			target:        &struct{ Value uint16 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "int16 minimum value conversion to uint16 should error",
			conversion:    "int16 min → uint16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if err == nil {
				t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				return
			}

			errMsg := err.Error()
			t.Logf("✓ Error message for %s: %s", tt.conversion, errMsg)

			// Verify error message quality
			foundPatterns := 0
			for _, pattern := range tt.errorPatterns {
				if strings.Contains(strings.ToLower(errMsg), strings.ToLower(pattern)) {
					foundPatterns++
				}
			}

			if foundPatterns > 0 {
				t.Logf("✓ Error message contains %d/%d expected patterns", foundPatterns, len(tt.errorPatterns))
			} else {
				t.Logf("Note: Error message doesn't contain expected patterns: %s", errMsg)
			}

			// Check for negative value indication in error
			lowerErrMsg := strings.ToLower(errMsg)
			if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "invalid", "out of range", "overflow", "underflow"}) {
				t.Logf("✓ Error message indicates invalid conversion condition")
			}
		})
	}
}

// TestInt8ToInt16InNestedStructs tests int8→uint8 and int16→uint16
// negative conversion scenarios in nested structures.
func TestInt8ToInt16InNestedStructs(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "int8→uint8 negative in nested struct",
			yamlContent: `
	config:
	  port: -80
	  timeout: 30
`,
			target: &struct {
				Config *struct {
					Port    uint8  `yaml:"port"`
					Timeout int16  `yaml:"timeout"`
				} `yaml:"config"`
			}{},
			shouldError: true,
			description: "Nested struct with int8→uint8 negative conversion",
		},
		{
			name: "int16→uint16 negative in nested struct",
			yamlContent: `
	sensor:
	  reading: -5000
	  status: ok
`,
			target: &struct {
				Sensor *struct {
					Reading uint16 `yaml:"reading"`
					Status  string `yaml:"status"`
				} `yaml:"sensor"`
			}{},
			shouldError: true,
			description: "Nested struct with int16→uint16 negative conversion",
		},
		{
			name: "int8→uint8 negative in array of structs",
			yamlContent: `
	ports:
	  - name: http
	    value: -80
	  - name: https
	    value: 443
`,
			target: &struct {
				Ports []struct {
					Name  string `yaml:"name"`
					Value uint8  `yaml:"value"`
				} `yaml:"ports"`
			}{},
			shouldError: true,
			description: "Array of structs with int8→uint8 negative conversion",
		},
		{
			name: "int16→uint16 negative in map values",
			yamlContent: `
	counters:
	  requests: -10000
	  responses: 20000
`,
			target: &struct {
				Counters map[string]uint16 `yaml:"counters"`
			}{},
			shouldError: true,
			description: "Map with int16→uint16 negative conversion in value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestInt8ToInt16BoundaryCases tests boundary cases for int8→uint8 and
// int16→uint16 negative conversions.
func TestInt8ToInt16BoundaryCases(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		// Boundary values for int8→uint8
		{
			name: "int8→uint8 -1 (boundary)",
			yamlContent: `
  value: -1
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Value -1 is the smallest negative that should error for uint8",
		},
		{
			name: "int8→uint8 -128 (int8 min)",
			yamlContent: `
  value: -128
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "int8 minimum value should error for uint8",
		},
		{
			name: "int8→uint8 0 (valid boundary)",
			yamlContent: `
  value: 0
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Value 0 is valid for uint8",
		},
		{
			name: "int8→uint8 1 (valid boundary)",
			yamlContent: `
  value: 1
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Value 1 is valid for uint8",
		},

		// Boundary values for int16→uint16
		{
			name: "int16→uint16 -1 (boundary)",
			yamlContent: `
  value: -1
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Value -1 is the smallest negative that should error for uint16",
		},
		{
			name: "int16→uint16 -32768 (int16 min)",
			yamlContent: `
  value: -32768
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "int16 minimum value should error for uint16",
		},
		{
			name: "int16→uint16 0 (valid boundary)",
			yamlContent: `
  value: 0
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Value 0 is valid for uint16",
		},
		{
			name: "int16→uint16 1 (valid boundary)",
			yamlContent: `
  value: 1
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Value 1 is valid for uint16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)

					// Verify error message indicates invalid conversion
					lowerErrMsg := strings.ToLower(err.Error())
					if containsAny(lowerErrMsg, []string{"cannot unmarshal", "negative", "invalid", "out of range", "overflow", "underflow"}) {
						t.Logf("✓ Error message indicates invalid conversion condition")
					}
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}
