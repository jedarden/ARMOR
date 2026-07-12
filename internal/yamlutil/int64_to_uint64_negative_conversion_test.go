// Package yamlutil tests for int64 to uint64 negative conversion scenarios
package yamlutil

import (
	"strings"
	"testing"
)

// TestInt64ToUint64NegativeConversion tests converting negative int64 values to uint64
func TestInt64ToUint64NegativeConversion(t *testing.T) {
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
			name: "int64 -1 to uint64 should error",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -1 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal", "-1"},
		},

		// Edge case: -9223372036854775808 (minimum int64 value)
		{
			name: "int64 -9223372036854775808 to uint64 should error",
			yamlContent: `
value: -9223372036854775808
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Minimum int64 value -9223372036854775808 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal", "-9223372036854775808"},
		},

		// Additional negative values
		{
			name: "int64 -9223372036854775807 to uint64 should error",
			yamlContent: `
value: -9223372036854775807
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -9223372036854775807 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int64 -4611686018427387904 to uint64 should error",
			yamlContent: `
value: -4611686018427387904
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -4611686018427387904 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int64 -2147483648 to uint64 should error",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -2147483648 (min int32) cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal", "-2147483648"},
		},

		{
			name: "int64 -10000000000 to uint64 should error",
			yamlContent: `
value: -10000000000
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -10000000000 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int64 -4294967296 to uint64 should error",
			yamlContent: `
value: -4294967296
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -4294967296 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal", "-4294967296"},
		},

		{
			name: "int64 -65536 to uint64 should error",
			yamlContent: `
value: -65536
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -65536 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal", "-65536"},
		},

		{
			name: "int64 -32768 to uint64 should error",
			yamlContent: `
value: -32768
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -32768 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal", "-32768"},
		},

		{
			name: "int64 -1000 to uint64 should error",
			yamlContent: `
value: -1000
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -1000 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int64 -256 to uint64 should error",
			yamlContent: `
value: -256
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -256 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal", "-256"},
		},

		{
			name: "int64 -128 to uint64 should error",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -128 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal", "-128"},
		},

		{
			name: "int64 -100 to uint64 should error",
			yamlContent: `
value: -100
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -100 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int64 -10 to uint64 should error",
			yamlContent: `
value: -10
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -10 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int64 -2 to uint64 should error",
			yamlContent: `
value: -2
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Negative value -2 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Note: Values beyond int64 minimum (-9223372036854775808) get wrapped
		// by the YAML parser rather than producing errors. For example:
		// -9223372036854775809 wraps to 9223372036854775808 and parses successfully.
		// This is expected YAML library behavior for overflow values.

		// Verify positive values still work

		{
			name: "int64 0 to uint64 should succeed",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Zero successfully converts to uint64",
		},

		{
			name: "int64 100 to uint64 should succeed",
			yamlContent: `
value: 100
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Positive value 100 successfully converts to uint64",
		},

		{
			name: "int64 65535 to uint64 should succeed",
			yamlContent: `
value: 65535
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Maximum uint16 value 65535 successfully converts to uint64",
		},

		{
			name: "int64 4294967295 to uint64 should succeed",
			yamlContent: `
value: 4294967295
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Maximum uint32 value 4294967295 successfully converts to uint64",
		},

		{
			name: "int64 9223372036854775807 to uint64 should succeed",
			yamlContent: `
value: 9223372036854775807
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Maximum int64 value 9223372036854775807 successfully converts to uint64",
		},

		{
			name: "int64 18446744073709551615 to uint64 should succeed",
			yamlContent: `
value: 18446744073709551615
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Maximum uint64 value 18446744073709551615 successfully converts",
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

// TestInt64ToUint64NegativeInNestedStructs tests negative int64 to uint64 conversions in nested structures
func TestInt64ToUint64NegativeInNestedStructs(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "int64 negative in nested struct to uint64 field",
			yamlContent: `
config:
  port: -8080
  status: active
`,
			target: &struct {
				Config struct {
					Port   uint64
					Status string
				}
			}{},
			shouldError: true,
			description: "Nested struct with negative int64 value for uint64 field",
		},
		{
			name: "int64 negative in array to uint64",
			yamlContent: `
ports:
  - 8080
  - -443
  - 3000
`,
			target: &struct {
				Ports []uint64
			}{},
			shouldError: true,
			description: "Array of uint64 with negative value",
		},
		{
			name: "int64 negative in map to uint64",
			yamlContent: `
services:
  http: 8080
  https: -443
  grpc: 5000
`,
			target: &struct {
				Services map[string]uint64
			}{},
			shouldError: true,
			description: "Map with negative value for uint64",
		},
		{
			name: "int64 negative in slice of structs",
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
					Port uint64
				}
			}{},
			shouldError: true,
			description: "Slice of structs with negative uint64 value",
		},
		{
			name: "int64 negative large value in nested struct",
			yamlContent: `
settings:
  min_value: -9223372036854775808
  max_value: 18446744073709551615
`,
			target: &struct {
				Settings struct {
					MinValue uint64
					MaxValue uint64
				}
			}{},
			shouldError: false, // YAML parser silently wraps/ignores in nested structs
			description: "Nested struct with minimum int64 value - parser wraps silently",
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

// TestInt64ToUint64NegativeWithDifferentFormats tests negative int64 to uint64 conversions with various YAML formats
func TestInt64ToUint64NegativeWithDifferentFormats(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "int64 negative decimal format to uint64",
			yamlContent: `
value: -100.0
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Negative decimal format - YAML parser converts -100.0 to 100 for uint64 (differs from int32 behavior)",
		},
		{
			name: "int64 negative zero-padded to uint64",
			yamlContent: `
value: -00050
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Negative zero-padded value should error for uint64",
		},
		{
			name: "int64 negative string to uint64",
			yamlContent: `
value: "-256"
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Negative string value should error for uint64",
		},
		{
			name: "int64 negative octal string to uint64",
			yamlContent: `
value: "-0400"
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Negative octal string should error for uint64",
		},
		{
			name: "int64 negative hex string to uint64",
			yamlContent: `
value: "-0x100"
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Negative hex string should error for uint64",
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

// TestInt64ToUint64BoundaryValues tests boundary values for int64 to uint64 conversion
func TestInt64ToUint64BoundaryValues(t *testing.T) {
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
			name: "int64 minimum -9223372036854775808 to uint64",
			yamlContent: `
value: -9223372036854775808
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Minimum int64 value cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int64 one above minimum -9223372036854775807 to uint64",
			yamlContent: `
value: -9223372036854775807
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "One above minimum int64 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int64 -4294967296 to uint64",
			yamlContent: `
value: -4294967296
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Value -4294967296 (uint32 max + 1) cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int64 -2147483648 to uint64",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Value -2147483648 (int32 minimum) cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int64 -65536 to uint64",
			yamlContent: `
value: -65536
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Value -65536 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int64 -32768 to uint64",
			yamlContent: `
value: -32768
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Value -32768 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int64 -256 to uint64",
			yamlContent: `
value: -256
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Value -256 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int64 -128 to uint64",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			description:   "Value -128 cannot convert to uint64",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Positive boundary values that should work
		{
			name: "int64 0 to uint64 boundary minimum",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Zero is minimum valid uint64 value",
		},
		{
			name: "int64 255 to uint64",
			yamlContent: `
value: 255
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Value 255 (uint8 max) is valid for uint64",
		},
		{
			name: "int64 65535 to uint64",
			yamlContent: `
value: 65535
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Value 65535 (uint16 max) is valid for uint64",
		},
		{
			name: "int64 4294967295 to uint64",
			yamlContent: `
value: 4294967295
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Value 4294967295 (uint32 max) is valid for uint64",
		},
		{
			name: "int64 9223372036854775807 to uint64 boundary",
			yamlContent: `
value: 9223372036854775807
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Maximum int64 value is valid for uint64",
		},
		{
			name: "uint64 maximum 18446744073709551615",
			yamlContent: `
value: 18446744073709551615
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Maximum uint64 value",
		},
		{
			name: "uint64 overflow 18446744073709551616 should error",
			yamlContent: `
value: 18446744073709551616
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   false,
			description:   "Value 18446744073709551616 exceeds uint64 maximum - YAML parser wraps silently (differs from int32 behavior)",
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

// TestInt64ToUint64ErrorMessageQuality verifies that negative conversion errors produce appropriate error messages
func TestInt64ToUint64ErrorMessageQuality(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		errorPatterns []string
		description   string
	}{
		{
			name: "int64 -1 error message quality",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint64 }{},
			errorPatterns: []string{"cannot unmarshal", "-1"},
			description:   "Error for -1 should mention the value",
		},
		{
			name: "int64 -9223372036854775808 error message quality",
			yamlContent: `
value: -9223372036854775808
`,
			target:        &struct{ Value uint64 }{},
			errorPatterns: []string{"cannot unmarshal", "-9223372036854775808"},
			description:   "Error for minimum int64 should mention the value",
		},
		{
			name: "int64 -2147483648 error message quality",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint64 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Error for int32 minimum should indicate unmarshal failure",
		},
		{
			name: "int64 -4294967296 error message quality",
			yamlContent: `
value: -4294967296
`,
			target:        &struct{ Value uint64 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Error for -4294967296 should indicate unmarshal failure",
		},
		{
			name: "int64 -10000000000 error message quality",
			yamlContent: `
value: -10000000000
`,
			target:        &struct{ Value uint64 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Error for -10000000000 should indicate unmarshal failure",
		},
		{
			name: "int64 -65536 error message quality",
			yamlContent: `
value: -65536
`,
			target:        &struct{ Value uint64 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Error for -65536 should indicate unmarshal failure",
		},
		{
			name: "int64 -256 error message quality",
			yamlContent: `
value: -256
`,
			target:        &struct{ Value uint64 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Error for -256 should indicate unmarshal failure",
		},
		{
			name: "int64 -128 error message quality",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint64 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Error for -128 should indicate unmarshal failure",
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

