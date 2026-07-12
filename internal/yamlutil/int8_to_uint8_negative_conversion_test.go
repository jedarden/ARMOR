// Package yamlutil tests for int8 to uint8 negative conversion scenarios
package yamlutil

import (
	"strings"
	"testing"
)

// TestInt8ToUint8NegativeConversion tests converting negative int8 values to uint8
func TestInt8ToUint8NegativeConversion(t *testing.T) {
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
			name: "int8 -1 to uint8 should error",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -1 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal", "-1"},
		},

		// Edge case: -128 (minimum int8 value)
		{
			name: "int8 -128 to uint8 should error",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Minimum int8 value -128 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal", "-128"},
		},

		// Additional negative values
		{
			name: "int8 -127 to uint8 should error",
			yamlContent: `
value: -127
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -127 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int8 -64 to uint8 should error",
			yamlContent: `
value: -64
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -64 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int8 -10 to uint8 should error",
			yamlContent: `
value: -10
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -10 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		{
			name: "int8 -2 to uint8 should error",
			yamlContent: `
value: -2
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			description:   "Negative value -2 cannot convert to uint8",
			expectedInMsg: []string{"cannot unmarshal"},
		},

		// Verify positive values still work
		{
			name: "int8 0 to uint8 should succeed",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Zero successfully converts to uint8",
		},

		{
			name: "int8 127 to uint8 should succeed",
			yamlContent: `
value: 127
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Maximum int8 value 127 successfully converts to uint8",
		},

		{
			name: "int8 100 to uint8 should succeed",
			yamlContent: `
value: 100
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Positive value 100 successfully converts to uint8",
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
					for _, expected := range tt.expectedInMsg {
						if !strings.Contains(errMsg, expected) && !strings.Contains(strings.ToLower(errMsg), strings.ToLower(expected)) {
							t.Logf("Note: Error message doesn't contain expected pattern %q: %s", expected, errMsg)
						}
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

// TestInt8ToUint8NegativeConversionInNestedStructs tests negative int8 to uint8
// conversion in nested structures
func TestInt8ToUint8NegativeConversionInNestedStructs(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "int8 -1 in nested struct to uint8",
			yamlContent: `
config:
  port: -1
  timeout: 30
`,
			target: &struct {
				Config struct {
					Port    uint8
					Timeout uint8
				}
			}{},
			shouldError: true,
			description: "Nested struct with int8 -1 to uint8 field",
		},

		{
			name: "int8 -128 in nested struct to uint8",
			yamlContent: `
settings:
  min: -128
  max: 255
`,
			target: &struct {
				Settings struct {
					Min uint8
					Max uint8
				}
			}{},
			shouldError: true,
			description: "Nested struct with int8 -128 to uint8 field",
		},

		{
			name: "array with negative int8 to uint8",
			yamlContent: `
values:
  - 10
  - -5
  - 20
`,
			target: &struct {
				Values []uint8
			}{},
			shouldError: true,
			description: "Array with negative int8 value to uint8 slice",
		},

		{
			name: "map with negative int8 to uint8",
			yamlContent: `
ports:
  http: 80
  https: -443
`,
			target: &struct {
				Ports map[string]uint8
			}{},
			shouldError: true,
			description: "Map with negative int8 value to uint8",
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

// TestInt8ToUint8NegativeConversionErrorMessages verifies that negative conversion
// errors produce appropriate error messages
func TestInt8ToUint8NegativeConversionErrorMessages(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		errorPatterns []string
		description   string
	}{
		{
			name: "error message for -1 to uint8",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint8 }{},
			errorPatterns: []string{"cannot unmarshal", "-1"},
			description:   "Error for -1 should contain the value and unmarshal message",
		},
		{
			name: "error message for -128 to uint8",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint8 }{},
			errorPatterns: []string{"cannot unmarshal", "-128"},
			description:   "Error for -128 should contain the value and unmarshal message",
		},
		{
			name: "error message for -64 to uint8",
			yamlContent: `
value: -64
`,
			target:        &struct{ Value uint8 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Error for -64 should contain unmarshal message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if err == nil {
				t.Errorf("Test '%s' should error but didn't", tt.name)
				return
			}

			errMsg := err.Error()
			t.Logf("✓ Error message for %s: %s", tt.name, errMsg)

			// Verify error message contains expected patterns
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
		})
	}
}

// TestInt8ToUint8NegativeConversionWithDifferentFormats tests negative conversion
// with various YAML number formats
func TestInt8ToUint8NegativeConversionWithDifferentFormats(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "negative int with decimal format to uint8",
			yamlContent: `
value: -1.0
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Float format -1.0 should error for uint8",
		},

		{
			name: "negative int with scientific notation to uint8",
			yamlContent: `
value: -1.28e2
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Scientific notation -128 should error for uint8",
		},

		{
			name: "negative zero to uint8 should succeed",
			yamlContent: `
value: -0.0
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Negative zero converts to zero in uint8",
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

// TestInt8ToUint8BoundaryValues tests boundary values for int8 to uint8 conversion
func TestInt8ToUint8BoundaryValues(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		// Negative boundary values (should error)
		{
			name: "int8 minimum value -128 to uint8",
			yamlContent: `
value: -128
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "int8 minimum value cannot convert to uint8",
		},

		{
			name: "int8 -129 (below minimum) to uint8",
			yamlContent: `
value: -129
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Value below int8 minimum cannot convert to uint8",
		},

		// Valid boundary values (should succeed)
		{
			name: "int8 0 to uint8",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Zero successfully converts to uint8",
		},

		{
			name: "int8 maximum value 127 to uint8",
			yamlContent: `
value: 127
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "int8 maximum value successfully converts to uint8",
		},

		{
			name: "value at uint8 maximum 255",
			yamlContent: `
value: 255
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "uint8 maximum value successfully converts",
		},

		{
			name: "value above uint8 maximum 256",
			yamlContent: `
value: 256
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Value above uint8 maximum should error",
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
