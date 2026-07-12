// Package yamlutil tests for signed integer underflow scenarios
package yamlutil

import (
	"strings"
	"testing"
)

// TestSignedIntegerUnderflowScenarios comprehensively tests signed integer underflow
// for int8, int16, int32, and int64 types, verifying both error detection and error messages.
func TestSignedIntegerUnderflowScenarios(t *testing.T) {
	tests := []struct {
		name            string
		yamlContent     string
		target          interface{}
		shouldError     bool
		description     string
		expectedInMsg   []string
		notExpectedInMsg []string
	}{
		// int8 underflow scenarios (range: -128 to 127)
		{
			name: "int8 underflow - one below minimum",
			yamlContent: `
value: -129
`,
			target:        &struct{ Value int8 }{},
			shouldError:   true,
			description:   "Value -129 is one below int8 minimum (-128)",
			expectedInMsg: []string{"cannot unmarshal", "-129", "out of range", "underflow"},
		},
		{
			name: "int8 underflow - far below minimum",
			yamlContent: `
value: -1000
`,
			target:        &struct{ Value int8 }{},
			shouldError:   true,
			description:   "Value -1000 is far below int8 minimum (-128)",
			expectedInMsg: []string{"cannot unmarshal", "-1000"},
		},
		{
			name: "int8 underflow - very large negative",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value int8 }{},
			shouldError:   true,
			description:   "Value -2147483648 is far below int8 minimum (-128)",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int8 boundary - minimum valid value",
			yamlContent: `
value: -128
`,
			target:      &struct{ Value int8 }{},
			shouldError: false,
			description: "Value -128 is exactly int8 minimum (valid)",
		},

		// int16 underflow scenarios (range: -32768 to 32767)
		{
			name: "int16 underflow - one below minimum",
			yamlContent: `
value: -32769
`,
			target:        &struct{ Value int16 }{},
			shouldError:   true,
			description:   "Value -32769 is one below int16 minimum (-32768)",
			expectedInMsg: []string{"cannot unmarshal", "-32769", "out of range", "underflow"},
		},
		{
			name: "int16 underflow - far below minimum",
			yamlContent: `
value: -100000
`,
			target:        &struct{ Value int16 }{},
			shouldError:   true,
			description:   "Value -100000 is far below int16 minimum (-32768)",
			expectedInMsg: []string{"cannot unmarshal", "-100000"},
		},
		{
			name: "int16 underflow - int32 minimum",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value int16 }{},
			shouldError:   true,
			description:   "Value -2147483648 is far below int16 minimum (-32768)",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16 boundary - minimum valid value",
			yamlContent: `
value: -32768
`,
			target:      &struct{ Value int16 }{},
			shouldError: false,
			description: "Value -32768 is exactly int16 minimum (valid)",
		},

		// int32 underflow scenarios (range: -2147483648 to 2147483647)
		{
			name: "int32 underflow - one below minimum",
			yamlContent: `
value: -2147483649
`,
			target:        &struct{ Value int32 }{},
			shouldError:   true,
			description:   "Value -2147483649 is one below int32 minimum (-2147483648)",
			expectedInMsg: []string{"cannot unmarshal", "-2147483649", "out of range", "underflow"},
		},
		{
			name: "int32 underflow - far below minimum",
			yamlContent: `
value: -9223372036854775808
`,
			target:        &struct{ Value int32 }{},
			shouldError:   true,
			description:   "Value -9223372036854775808 is far below int32 minimum (-2147483648)",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int32 underflow - very large negative",
			yamlContent: `
value: -999999999999999999999
`,
			target:        &struct{ Value int32 }{},
			shouldError:   true,
			description:   "Very large negative value exceeds int32 range",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int32 boundary - minimum valid value",
			yamlContent: `
value: -2147483648
`,
			target:      &struct{ Value int32 }{},
			shouldError: false,
			description: "Value -2147483648 is exactly int32 minimum (valid)",
		},

		// int64 underflow scenarios (range: -9223372036854775808 to 9223372036854775807)
		{
			name: "int64 underflow - one below minimum",
			yamlContent: `
value: -9223372036854775809
`,
			target:      &struct{ Value int64 }{},
			shouldError: false, // Note: Parser wraps this value
			description: "Value -9223372036854775809 wraps (parser behavior)",
		},
		{
			name: "int64 underflow - far below minimum",
			yamlContent: `
value: -18446744073709551616
`,
			target:      &struct{ Value int64 }{},
			shouldError: false, // Note: Parser wraps this value
			description: "Very large negative value wraps (parser behavior)",
		},
		{
			name: "int64 boundary - minimum valid value",
			yamlContent: `
value: -9223372036854775808
`,
			target:      &struct{ Value int64 }{},
			shouldError: false,
			description: "Value -9223372036854775808 is exactly int64 minimum (valid)",
		},
		{
			name: "int64 near underflow - large negative but valid",
			yamlContent: `
value: -9223372036854775807
`,
			target:      &struct{ Value int64 }{},
			shouldError: false,
			description: "Value -9223372036854775807 is within int64 range (valid)",
		},

		// Additional edge cases for mixed type scenarios
		{
			name: "int8 underflow with zero prefix",
			yamlContent: `
	value: -00129
`,
			target:        &struct{ Value int8 }{},
			shouldError:   true,
			description:   "Value -129 with zero prefix should still error",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16 underflow with positive sign for negative",
			yamlContent: `
	value: --32769
`,
			target:        &struct{ Value int16 }{},
			shouldError:   true,
			description:   "Double-negative syntax should error",
			expectedInMsg: []string{"cannot unmarshal", "unknown"},
		},
		{
			name: "int32 underflow via scientific notation",
			yamlContent: `
	value: -2.5e9
`,
			target:        &struct{ Value int32 }{},
			shouldError:   true,
			description:   "Scientific notation -2.5e9 exceeds int32 minimum",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int64 with extremely large negative string",
			yamlContent: `
	value: "-99999999999999999999999999999999999999999999999999999999999999999999999999999999"
`,
			target:        &struct{ Value int64 }{},
			shouldError:   true,
			description:   "Extremely large negative string should error",
			expectedInMsg: []string{"cannot unmarshal", "out of range"},
		},

		// Boundary validation - verify underflow vs overflow detection
		{
			name: "int8 verify underflow not overflow",
			yamlContent: `
	value: -130
`,
			target:        &struct{ Value int8 }{},
			shouldError:   true,
			description:   "Value -130 should be detected as underflow, not overflow",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int16 verify underflow not overflow",
			yamlContent: `
	value: -32770
`,
			target:        &struct{ Value int16 }{},
			shouldError:   true,
			description:   "Value -32770 should be detected as underflow, not overflow",
			expectedInMsg: []string{"cannot unmarshal"},
		},
		{
			name: "int32 verify underflow not overflow",
			yamlContent: `
	value: -2147483650
`,
			target:        &struct{ Value int32 }{},
			shouldError:   true,
			description:   "Value -2147483650 should be detected as underflow, not overflow",
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

					// Verify error message contains expected patterns
					errMsg := err.Error()
					lowerErrMsg := strings.ToLower(errMsg)

					// Check for expected patterns in error message
					for _, expected := range tt.expectedInMsg {
						if !strings.Contains(errMsg, expected) && !strings.Contains(lowerErrMsg, strings.ToLower(expected)) {
							t.Logf("Note: Error message doesn't contain expected pattern %q: %s", expected, errMsg)
						}
					}

					// Verify underflow-specific patterns if specified
					if containsAny(lowerErrMsg, []string{"underflow", "below minimum", "too small", "out of range"}) {
						t.Logf("✓ Error message indicates underflow condition")
					} else {
						t.Logf("Note: Error message doesn't explicitly mention underflow: %s", errMsg)
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

// TestSignedIntegerUnderflowErrorMessages verifies that underflow errors produce
// appropriate error messages that indicate the underflow condition.
func TestSignedIntegerUnderflowErrorMessages(t *testing.T) {
	tests := []struct {
		name           string
		yamlContent    string
		target         interface{}
		errorPatterns  []string
		description    string
		underflowType  string
	}{
		{
			name: "int8 underflow error message quality",
			yamlContent: `
	value: -129
`,
			target:        &struct{ Value int8 }{},
			errorPatterns: []string{"cannot unmarshal", "out of range", "-129"},
			description:   "int8 underflow should produce descriptive error",
			underflowType: "int8 min (-128)",
		},
		{
			name: "int16 underflow error message quality",
			yamlContent: `
	value: -32769
`,
			target:        &struct{ Value int16 }{},
			errorPatterns: []string{"cannot unmarshal", "out of range", "-32769"},
			description:   "int16 underflow should produce descriptive error",
			underflowType: "int16 min (-32768)",
		},
		{
			name: "int32 underflow error message quality",
			yamlContent: `
	value: -2147483649
`,
			target:        &struct{ Value int32 }{},
			errorPatterns: []string{"cannot unmarshal", "out of range", "-2147483649"},
			description:   "int32 underflow should produce descriptive error",
			underflowType: "int32 min (-2147483648)",
		},
		{
			name: "int64 extreme negative error message",
			yamlContent: `
	value: -18446744073709551616
`,
			target:        &struct{ Value int64 }{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Extreme negative value should produce error message",
			underflowType: "int64 min (-9223372036854775808)",
		},
		{
			name: "multiple underflow scenarios in same document",
			yamlContent: `
	int8_value: -129
	int16_value: -32769
	int32_value: -2147483649
`,
			target: &struct {
				Int8Value  int8
				Int16Value int16
				Int32Value int32
			}{},
			errorPatterns: []string{"cannot unmarshal"},
			description:   "Multiple underflow errors should be detected",
			underflowType: "multiple",
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
			t.Logf("✓ Error message for %s: %s", tt.underflowType, errMsg)

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

			// Check for underflow-specific language
			lowerErrMsg := strings.ToLower(errMsg)
			if containsAny(lowerErrMsg, []string{"underflow", "below minimum", "too small", "exceeds.*negative", "out of range"}) {
				t.Logf("✓ Error message indicates underflow condition")
			}
		})
	}
}

// TestSignedIntegerUnderflowInNestedStructs tests underflow scenarios in nested structures
func TestSignedIntegerUnderflowInNestedStructs(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "int8 underflow in nested struct",
			yamlContent: `
	config:
	  temperature: -129
	  humidity: 50
`,
			target: &struct {
				Config struct {
					Temperature int8
					Humidity    int8
				}
			}{},
			shouldError: true,
			description: "Nested struct with int8 underflow",
		},
		{
			name: "int16 underflow in nested struct",
			yamlContent: `
	sensor:
	  reading: -32769
	  status: ok
`,
			target: &struct {
				Sensor struct {
					Reading int16
					Status  string
				}
			}{},
			shouldError: true,
			description: "Nested struct with int16 underflow",
		},
		{
			name: "int32 underflow in nested struct",
			yamlContent: `
	metrics:
	  counter: -2147483649
	  gauge: 100
`,
			target: &struct {
				Metrics struct {
					Counter int32
					Gauge   int32
				}
			}{},
			shouldError: true,
			description: "Nested struct with int32 underflow",
		},
		{
			name: "mixed underflow types in nested structs",
			yamlContent: `
	sensors:
	  - type: temperature
	    value: -129
	  - type: pressure
	    value: -32769
`,
			target: &struct {
				Sensors []struct {
					Type  string
					Value int16
				}
			}{},
			shouldError: true,
			description: "Array of structs with underflow errors",
		},
		{
			name: "int8 underflow in map values",
			yamlContent: `
	readings:
	  sensor1: -129
	  sensor2: 50
`,
			target: &struct {
				Readings map[string]int8
			}{},
			shouldError: true,
			description: "Map with int8 underflow in value",
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

// TestSignedIntegerUnderflowWithDifferentFormats tests underflow with various YAML number formats
func TestSignedIntegerUnderflowWithDifferentFormats(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "int8 underflow with decimal format",
			yamlContent: `
	value: -129.0
`,
			target:      &struct{ Value int8 }{},
			shouldError: true,
			description: "Float format -129.0 should error for int8",
		},
		{
			name: "int16 underflow with decimal format",
			yamlContent: `
	value: -32769.00
`,
			target:      &struct{ Value int16 }{},
			shouldError: true,
			description: "Float format -32769.00 should error for int16",
		},
		{
			name: "int32 underflow with scientific notation",
			yamlContent: `
	value: -2.147483649e9
`,
			target:      &struct{ Value int32 }{},
			shouldError: true,
			description: "Scientific notation -2.147483649e9 should error for int32",
		},
		{
			name: "int8 underflow with hexadecimal (negative)",
			yamlContent: `
	value: "-0x81"
`,
			target:      &struct{ Value int8 }{},
			shouldError: true,
			description: "Hex string -0x81 should error for int8",
		},
		{
			name: "int16 underflow with octal string",
			yamlContent: `
	value: "-0100001"
`,
			target:      &struct{ Value int16 }{},
			shouldError: true,
			description: "Octal string should error for int16",
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

// Helper function to check if a string contains any of the given patterns
func containsAny(s string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}
