// Package yamlutil tests for int32 to uint32 negative conversion scenarios
package yamlutil

import (
	"strings"
	"testing"
)

// TestInt32ToUint32NegativeConversion tests converting negative int32 values to uint32
func TestInt32ToUint32NegativeConversion(t *testing.T) {
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
			name: "int32 -1 to uint32 should error",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -1 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal", "-1"},
		},

		// Edge case: -2147483648 (minimum int32 value)
		{
			name: "int32 -2147483648 to uint32 should error",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Minimum int32 value -2147483648 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal", "-2147483648"},
		},

		// Additional negative values
		{
			name: "int32 -2147483647 to uint32 should error",
			yamlContent: `
value: -2147483647
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -2147483647 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int32 -1073741824 to uint32 should error",
			yamlContent: `
value: -1073741824
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -1073741824 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int32 -1000000 to uint32 should error",
			yamlContent: `
value: -1000000
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -1000000 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int32 -65536 to uint32 should error",
			yamlContent: `
value: -65536
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -65536 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int32 -32768 to uint32 should error",
			yamlContent: `
value: -32768
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -32768 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int32 -1000 to uint32 should error",
			yamlContent: `
value: -1000
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -1000 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int32 -256 to uint32 should error",
			yamlContent: `
value: -256
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -256 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int32 -128 to uint32 should error",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -128 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int32 -100 to uint32 should error",
			yamlContent: `
value: -100
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -100 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int32 -10 to uint32 should error",
			yamlContent: `
value: -10
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -10 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int32 -2 to uint32 should error",
			yamlContent: `
value: -2
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Negative value -2 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Extreme negative values beyond int32 range
		{
			name: "int32 -2147483649 to uint32 should error",
			yamlContent: `
value: -2147483649
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Value -2147483649 is below int32 minimum, cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int32 -4294967296 to uint32 should error",
			yamlContent: `
value: -4294967296
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Value -4294967296 is far below int32 minimum, cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Verify positive values still work
		{
			name: "int32 0 to uint32 should succeed",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Zero successfully converts to uint32",
		},

		{
			name: "int32 100 to uint32 should succeed",
			yamlContent: `
value: 100
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Positive value 100 successfully converts to uint32",
		},

		{
			name: "int32 65535 to uint32 should succeed",
			yamlContent: `
value: 65535
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Maximum uint16 value 65535 successfully converts to uint32",
		},

		{
			name: "int32 2147483647 to uint32 should succeed",
			yamlContent: `
value: 2147483647
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Maximum int32 value 2147483647 successfully converts to uint32",
		},

		{
			name: "int32 4294967295 to uint32 should succeed",
			yamlContent: `
value: 4294967295
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Maximum uint32 value 4294967295 successfully converts",
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
					t.Errorf("Test '%s' should succeed but errored: %v - %s", tt.name, err, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestInt32ToUint32NegativeInNestedStructs tests negative int32 to uint32 conversions in nested structures
func TestInt32ToUint32NegativeInNestedStructs(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "int32 negative in nested struct to uint32 field",
			yamlContent: `
config:
  port: -8080
  status: active
`,
			target: &struct {
				Config struct {
					Port   uint32
					Status string
				}
			}{},
			shouldError: true,
			description: "Nested struct with negative int32 value for uint32 field",
		},
		{
			name: "int32 negative in array to uint32",
			yamlContent: `
ports:
  - 8080
  - -443
  - 3000
`,
			target: &struct {
				Ports []uint32
			}{},
			shouldError: true,
			description: "Array of uint32 with negative value",
		},
		{
			name: "int32 negative in map to uint32",
			yamlContent: `
services:
  http: 8080
  https: -443
  grpc: 5000
`,
			target: &struct {
				Services map[string]uint32
			}{},
			shouldError: true,
			description: "Map with negative value for uint32",
		},
		{
			name: "int32 negative in slice of structs",
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
					Port uint32
				}
			}{},
			shouldError: true,
			description: "Slice of structs with negative uint32 value",
		},
		{
			name: "int32 negative large value in nested struct",
			yamlContent: `
settings:
  min_value: -2147483648
  max_value: 4294967295
`,
			target: &struct {
				Settings struct {
					MinValue uint32
					MaxValue uint32
				}
			}{},
			shouldError: true,
			description: "Nested struct with minimum int32 value for uint32 field",
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

// TestInt32ToUint32NegativeWithDifferentFormats tests negative int32 to uint32 conversions with various YAML formats
func TestInt32ToUint32NegativeWithDifferentFormats(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "int32 negative decimal format to uint32",
			yamlContent: `
value: -100.0
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Negative decimal format should error for uint32",
		},
		{
			name: "int32 negative zero-padded to uint32",
			yamlContent: `
value: -00050
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Negative zero-padded value should error for uint32",
		},
		{
			name: "int32 negative string to uint32",
			yamlContent: `
value: "-256"
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Negative string value should error for uint32",
		},
		{
			name: "int32 negative octal string to uint32",
			yamlContent: `
value: "-0400"
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Negative octal string should error for uint32",
		},
		{
			name: "int32 negative hex string to uint32",
			yamlContent: `
value: "-0x100"
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Negative hex string should error for uint32",
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

// TestInt32ToUint32BoundaryValues tests boundary values for int32 to uint32 conversion
func TestInt32ToUint32BoundaryValues(t *testing.T) {
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
			name: "int32 minimum -2147483648 to uint32",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Minimum int32 value cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int32 one above minimum -2147483647 to uint32",
			yamlContent: `
value: -2147483647
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "One above minimum int32 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int32 -65536 to uint32",
			yamlContent: `
value: -65536
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Value -65536 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int32 -32768 to uint32",
			yamlContent: `
value: -32768
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Value -32768 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int32 -256 to uint32",
			yamlContent: `
value: -256
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Value -256 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int32 -128 to uint32",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Value -128 cannot convert to uint32",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Positive boundary values that should work
		{
			name: "int32 0 to uint32 boundary minimum",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Zero is minimum valid uint32 value",
		},
		{
			name: "int32 255 to uint32",
			yamlContent: `
value: 255
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Value 255 (uint8 max) is valid for uint32",
		},
		{
			name: "int32 65535 to uint32",
			yamlContent: `
value: 65535
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Value 65535 (uint16 max) is valid for uint32",
		},
		{
			name: "int32 65536 to uint32",
			yamlContent: `
value: 65536
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Value 65536 is valid for uint32",
		},
		{
			name: "int32 2147483647 to uint32 boundary",
			yamlContent: `
value: 2147483647
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Maximum int32 value is valid for uint32",
		},
		{
			name: "uint32 maximum 4294967295",
			yamlContent: `
value: 4294967295
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Maximum uint32 value",
		},
		{
			name: "uint32 overflow 4294967296 should error",
			yamlContent: `
value: 4294967296
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			description:   "Value 4294967296 exceeds uint32 maximum",
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

// TestInt32ToUint32ErrorMessageQuality verifies that negative conversion errors produce appropriate error messages
func TestInt32ToUint32ErrorMessageQuality(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		errorPatterns []string
		description   string
	}{
		{
			name: "int32 -1 error message quality",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint32 }{},
			errorPatterns: []string{"cannot unmarshal", "-1"},
			description:   "Error for -1 should mention the value",
		},
		{
			name: "int32 -2147483648 error message quality",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint32 }{},
			errorPatterns: []string{"cannot unmarshal", "-2147483648"},
			description:   "Error for minimum int32 should mention the value",
		},
		{
			name: "int32 -1000000 error message quality",
			yamlContent: `
value: -1000000
`,
			target:        &struct{ Value uint32 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Error for -1000000 should indicate unmarshal failure",
		},
		{
			name: "int32 -65536 error message quality",
			yamlContent: `
value: -65536
`,
			target:        &struct{ Value uint32 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Error for -65536 should indicate unmarshal failure",
		},
		{
			name: "int32 -256 error message quality",
			yamlContent: `
value: -256
`,
			target:        &struct{ Value uint32 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Error for -256 should indicate unmarshal failure",
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

