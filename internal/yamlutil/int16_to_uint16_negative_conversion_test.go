// Package yamlutil tests for int16 to uint16 negative conversion scenarios
package yamlutil

import (
	"strings"
	"testing"
)

// TestInt16ToUint16NegativeConversion tests converting negative int16 values to uint16
func TestInt16ToUint16NegativeConversion(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		shouldError   bool
		description   string
		expectedInMsg []string
	}{
		// Edge case: -1 (common negative value)
		{
			name: "int16 -1 to uint16 should error",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -1 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal", "-1"},
		},

		// Edge case: -32768 (minimum int16 value)
		{
			name: "int16 -32768 to uint16 should error",
			yamlContent: `
value: -32768
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Minimum int16 value -32768 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal", "-32768"},
		},

		// Additional negative values
		{
			name: "int16 -32767 to uint16 should error",
			yamlContent: `
value: -32767
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -32767 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int16 -16384 to uint16 should error",
			yamlContent: `
value: -16384
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -16384 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int16 -1000 to uint16 should error",
			yamlContent: `
value: -1000
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -1000 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int16 -256 to uint16 should error",
			yamlContent: `
value: -256
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -256 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int16 -128 to uint16 should error",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -128 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int16 -100 to uint16 should error",
			yamlContent: `
value: -100
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -100 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int16 -10 to uint16 should error",
			yamlContent: `
value: -10
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -10 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int16 -2 to uint16 should error",
			yamlContent: `
value: -2
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Negative value -2 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Extreme negative values beyond int16 range
		{
			name: "int16 -32769 to uint16 should error",
			yamlContent: `
value: -32769
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Value -32769 is below int16 minimum, cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int16 -65536 to uint16 should error",
			yamlContent: `
value: -65536
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Value -65536 is far below int16 minimum, cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Verify positive values still work
		{
			name: "int16 0 to uint16 should succeed",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Zero successfully converts to uint16",
		},

		{
			name: "int16 100 to uint16 should succeed",
			yamlContent: `
value: 100
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Positive value 100 successfully converts to uint16",
		},

		{
			name: "int16 32767 to uint16 should succeed",
			yamlContent: `
value: 32767
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Maximum int16 value 32767 successfully converts to uint16",
		},

		{
			name: "int16 65535 to uint16 should succeed",
			yamlContent: `
value: 65535
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Maximum uint16 value 65535 successfully converts",
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
					lowerErrMsg := strings.ToLower(errMsg)

					// Check for expected patterns in error message
					for _, expected := range tt.expectedInMsg {
						if !strings.Contains(errMsg, expected) && !strings.Contains(lowerErrMsg, strings.ToLower(expected)) {
							t.Logf("Note: Error message doesn't contain expected pattern %q: %s", expected, errMsg)
						}
					}

					// Verify negative value indication in error
					if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "out of range", "invalid"}) {
						t.Logf("✓ Error message indicates invalid conversion for negative value")
					} else {
						t.Logf("Note: Error message doesn't explicitly mention negative value: %s", errMsg)
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

// TestInt16ToUint16NegativeInNestedStructs tests negative int16 to uint16 conversions in nested structures
func TestInt16ToUint16NegativeInNestedStructs(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "int16 negative in nested struct to uint16 field",
			yamlContent: `
config:
  port: -8080
  status: active
`,
			target: &struct {
				Config struct {
					Port   uint16
					Status string
				}
			}{},
			shouldError: true,
			description: "Nested struct with negative int16 value for uint16 field",
		},
		{
			name: "int16 negative in array to uint16",
			yamlContent: `
ports:
  - 8080
  - -443
  - 3000
`,
			target: &struct {
				Ports []uint16
			}{},
			shouldError: true,
			description: "Array of uint16 with negative value",
		},
		{
			name: "int16 negative in map to uint16",
			yamlContent: `
services:
  http: 8080
  https: -443
  grpc: 5000
`,
			target: &struct {
				Services map[string]uint16
			}{},
			shouldError: true,
			description: "Map with negative value for uint16",
		},
		{
			name: "int16 negative in slice of structs",
			yamlContent: `
servers:
  - name: primary
    port: 8080
  - name: backup
    port: -8443
`,
			target: &struct {
				Servers []struct {
					Name string
					Port uint16
				}
			}{},
			shouldError: true,
			description: "Slice of structs with negative uint16 value",
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

// TestInt16ToUint16NegativeWithDifferentFormats tests negative int16 to uint16 conversions with various YAML formats
func TestInt16ToUint16NegativeWithDifferentFormats(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "int16 negative decimal format to uint16",
			yamlContent: `
value: -100.0
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Negative decimal format should error for uint16",
		},
		{
			name: "int16 negative zero-padded to uint16",
			yamlContent: `
value: -00050
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Negative zero-padded value should error for uint16",
		},
		{
			name: "int16 negative string to uint16",
			yamlContent: `
value: "-256"
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Negative string value should error for uint16",
		},
		{
			name: "int16 negative octal string to uint16",
			yamlContent: `
value: "-0400"
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Negative octal string should error for uint16",
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

// TestInt16ToUint16BoundaryValues tests boundary values for int16 to uint16 conversion
func TestInt16ToUint16BoundaryValues(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		shouldError   bool
		description   string
		expectedInMsg []string
	}{
		// Negative boundary values
		{
			name: "int16 minimum -32768 to uint16",
			yamlContent: `
value: -32768
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Minimum int16 value cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16 one above minimum -32767 to uint16",
			yamlContent: `
value: -32767
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "One above minimum int16 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16 -256 to uint16",
			yamlContent: `
value: -256
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Value -256 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16 -128 to uint16",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Value -128 cannot convert to uint16",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Positive boundary values that should work
		{
			name: "int16 0 to uint16 boundary minimum",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Zero is minimum valid uint16 value",
		},
		{
			name: "int16 255 to uint16",
			yamlContent: `
value: 255
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Value 255 (uint8 max) is valid for uint16",
		},
		{
			name: "int16 256 to uint16",
			yamlContent: `
value: 256
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Value 256 is valid for uint16",
		},
		{
			name: "int16 32767 to uint16 boundary",
			yamlContent: `
value: 32767
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Maximum int16 value is valid for uint16",
		},
		{
			name: "uint16 maximum 65535",
			yamlContent: `
value: 65535
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Maximum uint16 value",
		},
		{
			name: "uint16 overflow 65536 should error",
			yamlContent: `
value: 65536
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			description:   "Value 65536 exceeds uint16 maximum",
			expectedInMsg: []string{"cannot unmarshal"},
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

					// Verify error message quality
					errMsg := err.Error()
					for _, expected := range tt.expectedInMsg {
						if !strings.Contains(errMsg, expected) && !strings.Contains(strings.ToLower(errMsg), strings.ToLower(expected)) {
							t.Logf("Note: Error message doesn't contain expected pattern %q: %s", expected, errMsg)
						}
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

// TestInt16ToUint16ErrorMessageQuality verifies that negative conversion errors produce appropriate error messages
func TestInt16ToUint16ErrorMessageQuality(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		errorPatterns []string
		description   string
	}{
		{
			name: "int16 -1 error message quality",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint16 }{},
			errorPatterns: []string{"cannot unmarshal", "-1"},
			description:   "Error for -1 should mention the value",
		},
		{
			name: "int16 -32768 error message quality",
			yamlContent: `
value: -32768
`,
			target:        &struct{ Value uint16 }{},
			errorPatterns: []string{"cannot unmarshal", "-32768"},
			description:   "Error for minimum int16 should mention the value",
		},
		{
			name: "int16 -1000 error message quality",
			yamlContent: `
value: -1000
`,
			target:        &struct{ Value uint16 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Error for -1000 should indicate unmarshal failure",
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
			t.Logf("✓ Error message: %s", errMsg)

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

			// Check for negative value indication
			lowerErrMsg := strings.ToLower(errMsg)
			if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "invalid", "out of range"}) {
				t.Logf("✓ Error message indicates invalid conversion")
			}
		})
	}
}
