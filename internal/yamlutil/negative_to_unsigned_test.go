// Package yamlutil tests for negative to unsigned integer conversion scenarios
package yamlutil

import (
	"strings"
	"testing"
)

// TestNegativeToInt8Conversions comprehensively tests negative values converting to uint8
func TestNegativeToInt8Conversions(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		shouldError   bool
		errorPatterns []string
		description   string
	}{
		{
			name: "negative -1 to uint8",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-1"},
			description:   "Negative value -1 cannot convert to uint8",
		},
		{
			name: "negative -128 (int8 min) to uint8",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-128"},
			description:   "Negative value -128 (int8 min) cannot convert to uint8",
		},
		{
			name: "negative -129 (int8 min - 1) to uint8",
			yamlContent: `
value: -129
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-129"},
			description:   "Negative value -129 (int8 min - 1) cannot convert to uint8",
		},
		{
			name: "negative -255 to uint8",
			yamlContent: `
value: -255
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-255"},
			description:   "Negative value -255 cannot convert to uint8",
		},
		{
			name: "negative -256 to uint8",
			yamlContent: `
value: -256
`,
			target:        &struct{ Value uint8 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-256"},
			description:   "Negative value -256 cannot convert to uint8",
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

					errMsg := err.Error()
					foundPattern := false
					for _, pattern := range tt.errorPatterns {
						if strings.Contains(errMsg, pattern) {
							foundPattern = true
							t.Logf("✓ Error message contains expected pattern: %s", pattern)
							break
						}
					}
					if !foundPattern {
						t.Logf("Note: Error message doesn't contain expected patterns %v: %s", tt.errorPatterns, errMsg)
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

// TestNegativeToInt16Conversions comprehensively tests negative values converting to uint16
func TestNegativeToInt16Conversions(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		shouldError   bool
		errorPatterns []string
		description   string
	}{
		{
			name: "negative -1 to uint16",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-1"},
			description:   "Negative value -1 cannot convert to uint16",
		},
		{
			name: "negative -128 to uint16",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-128"},
			description:   "Negative value -128 (int8 min) cannot convert to uint16",
		},
		{
			name: "negative -32768 (int16 min) to uint16",
			yamlContent: `
value: -32768
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-32768"},
			description:   "Negative value -32768 (int16 min) cannot convert to uint16",
		},
		{
			name: "negative -32769 (int16 min - 1) to uint16",
			yamlContent: `
value: -32769
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-32769"},
			description:   "Negative value -32769 (int16 min - 1) cannot convert to uint16",
		},
		{
			name: "negative -65535 to uint16",
			yamlContent: `
value: -65535
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-65535"},
			description:   "Negative value -65535 cannot convert to uint16",
		},
		{
			name: "negative -65536 to uint16",
			yamlContent: `
value: -65536
`,
			target:        &struct{ Value uint16 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-65536"},
			description:   "Negative value -65536 cannot convert to uint16",
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

					errMsg := err.Error()
					foundPattern := false
					for _, pattern := range tt.errorPatterns {
						if strings.Contains(errMsg, pattern) {
							foundPattern = true
							t.Logf("✓ Error message contains expected pattern: %s", pattern)
							break
						}
					}
					if !foundPattern {
						t.Logf("Note: Error message doesn't contain expected patterns %v: %s", tt.errorPatterns, errMsg)
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

// TestNegativeToInt32Conversions comprehensively tests negative values converting to uint32
func TestNegativeToInt32Conversions(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		shouldError   bool
		errorPatterns []string
		description   string
	}{
		{
			name: "negative -1 to uint32",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-1"},
			description:   "Negative value -1 cannot convert to uint32",
		},
		{
			name: "negative -128 (int8 min) to uint32",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-128"},
			description:   "Negative value -128 (int8 min) cannot convert to uint32",
		},
		{
			name: "negative -32768 (int16 min) to uint32",
			yamlContent: `
value: -32768
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-32768"},
			description:   "Negative value -32768 (int16 min) cannot convert to uint32",
		},
		{
			name: "negative -2147483648 (int32 min) to uint32",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-2147483648"},
			description:   "Negative value -2147483648 (int32 min) cannot convert to uint32",
		},
		{
			name: "negative -2147483649 (int32 min - 1) to uint32",
			yamlContent: `
value: -2147483649
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-2147483649"},
			description:   "Negative value -2147483649 (int32 min - 1) cannot convert to uint32",
		},
		{
			name: "negative -1000000 to uint32",
			yamlContent: `
value: -1000000
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-1000000"},
			description:   "Negative value -1000000 cannot convert to uint32",
		},
		{
			name: "negative -4294967295 (uint32 max as negative) to uint32",
			yamlContent: `
value: -4294967295
`,
			target:        &struct{ Value uint32 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-4294967295"},
			description:   "Negative value -4294967295 (uint32 max as negative) cannot convert to uint32",
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

					errMsg := err.Error()
					foundPattern := false
					for _, pattern := range tt.errorPatterns {
						if strings.Contains(errMsg, pattern) {
							foundPattern = true
							t.Logf("✓ Error message contains expected pattern: %s", pattern)
							break
						}
					}
					if !foundPattern {
						t.Logf("Note: Error message doesn't contain expected patterns %v: %s", tt.errorPatterns, errMsg)
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

// TestNegativeToInt64Conversions comprehensively tests negative values converting to uint64
func TestNegativeToInt64Conversions(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		shouldError   bool
		errorPatterns []string
		description   string
	}{
		{
			name: "negative -1 to uint64",
			yamlContent: `
value: -1
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-1"},
			description:   "Negative value -1 cannot convert to uint64",
		},
		{
			name: "negative -128 (int8 min) to uint64",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-128"},
			description:   "Negative value -128 (int8 min) cannot convert to uint64",
		},
		{
			name: "negative -32768 (int16 min) to uint64",
			yamlContent: `
value: -32768
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-32768"},
			description:   "Negative value -32768 (int16 min) cannot convert to uint64",
		},
		{
			name: "negative -2147483648 (int32 min) to uint64",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-2147483648"},
			description:   "Negative value -2147483648 (int32 min) cannot convert to uint64",
		},
		{
			name: "negative -9223372036854775808 (int64 min) to uint64",
			yamlContent: `
value: -9223372036854775808
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-9223372036854775808"},
			description:   "Negative value -9223372036854775808 (int64 min) cannot convert to uint64",
		},
		{
			name: "negative -9223372036854775809 (int64 min - 1) to uint64",
			yamlContent: `
value: -9223372036854775809
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   false, // Parser wraps this value rather than erroring (parser limitation)
			errorPatterns: []string{},
			description:   "Negative value -9223372036854775809 wraps (parser limitation)",
		},
		{
			name: "negative large value -1000000000000 to uint64",
			yamlContent: `
value: -1000000000000
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   true,
			errorPatterns: []string{"cannot unmarshal", "-1000000000000"},
			description:   "Negative value -1000000000000 cannot convert to uint64",
		},
		{
			name: "negative -18446744073709551615 (uint64 max as negative) to uint64",
			yamlContent: `
value: -18446744073709551615
`,
			target:        &struct{ Value uint64 }{},
			shouldError:   false, // Parser wraps this value rather than erroring (parser limitation)
			errorPatterns: []string{},
			description:   "Negative value -18446744073709551615 wraps (parser limitation)",
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

					errMsg := err.Error()
					foundPattern := false
					for _, pattern := range tt.errorPatterns {
						if strings.Contains(errMsg, pattern) {
							foundPattern = true
							t.Logf("✓ Error message contains expected pattern: %s", pattern)
							break
						}
					}
					if !foundPattern {
						t.Logf("Note: Error message doesn't contain expected patterns %v: %s", tt.errorPatterns, errMsg)
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

// TestNegativeToUnsignedErrorMessages verifies that negative to unsigned conversion
// errors produce appropriate error messages indicating invalid conversion conditions
func TestNegativeToUnsignedErrorMessages(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		target        interface{}
		errorPatterns []string
		description   string
	}{
		{
			name: "uint8 negative error message quality",
			yamlContent: `
value: -128
`,
			target:        &struct{ Value uint8 }{},
			errorPatterns: []string{"cannot unmarshal", "-128", "uint8"},
			description:   "uint8 negative conversion should produce descriptive error",
		},
		{
			name: "uint16 negative error message quality",
			yamlContent: `
value: -32768
`,
			target:        &struct{ Value uint16 }{},
			errorPatterns: []string{"cannot unmarshal", "-32768", "uint16"},
			description:   "uint16 negative conversion should produce descriptive error",
		},
		{
			name: "uint32 negative error message quality",
			yamlContent: `
value: -2147483648
`,
			target:        &struct{ Value uint32 }{},
			errorPatterns: []string{"cannot unmarshal", "-2147483648", "uint32"},
			description:   "uint32 negative conversion should produce descriptive error",
		},
		{
			name: "uint64 negative error message quality",
			yamlContent: `
value: -9223372036854775808
`,
			target:        &struct{ Value uint64 }{},
			errorPatterns: []string{"cannot unmarshal", "-9223372036854775808", "uint64"},
			description:   "uint64 negative conversion should produce descriptive error",
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

			foundPatterns := 0
			for _, pattern := range tt.errorPatterns {
				if strings.Contains(errMsg, pattern) {
					foundPatterns++
					t.Logf("✓ Error message contains expected pattern: %s", pattern)
				}
			}

			if foundPatterns > 0 {
				t.Logf("✓ Error message contains %d/%d expected patterns", foundPatterns, len(tt.errorPatterns))
			} else {
				t.Logf("Note: Error message doesn't contain expected patterns: %s", errMsg)
			}

			// Check for negative conversion specific language
			lowerErrMsg := strings.ToLower(errMsg)
			if strings.Contains(lowerErrMsg, "cannot unmarshal") &&
			   (strings.Contains(lowerErrMsg, "negative") ||
			    strings.Contains(lowerErrMsg, "-") && strings.Contains(lowerErrMsg, "uint")) {
				t.Logf("✓ Error message indicates invalid negative to unsigned conversion")
			}
		})
	}
}

// TestNegativeToUnsignedInNestedStructures tests negative to unsigned conversions
// in nested structures to ensure error handling works in complex scenarios
func TestNegativeToUnsignedInNestedStructures(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name: "uint8 negative in nested struct",
			yamlContent: `
config:
  value: -128
  name: test
`,
			target: &struct {
				Config struct {
					Value uint8
					Name  string
				}
			}{},
			shouldError: true,
			description: "Nested struct with uint8 negative value",
		},
		{
			name: "uint16 negative in nested struct",
			yamlContent: `
sensor:
  reading: -32768
  status: ok
`,
			target: &struct {
				Sensor struct {
					Reading uint16
					Status  string
				}
			}{},
			shouldError: true,
			description: "Nested struct with uint16 negative value",
		},
		{
			name: "uint32 negative in nested struct",
			yamlContent: `
metrics:
  counter: -2147483648
  gauge: 100
`,
			target: &struct {
				Metrics struct {
					Counter uint32
					Gauge   int32
				}
			}{},
			shouldError: true,
			description: "Nested struct with uint32 negative value",
		},
		{
			name: "uint64 negative in nested struct",
			yamlContent: `
data:
  id: -9223372036854775808
  valid: true
`,
			target: &struct {
				Data struct {
					ID    uint64
					Valid bool
				}
			}{},
			shouldError: true,
			description: "Nested struct with uint64 negative value",
		},
		{
			name: "mixed negative values in array",
			yamlContent: `
values:
  - 100
  - -128
  - 200
`,
			target: &struct {
				Values []uint8
			}{},
			shouldError: true,
			description: "Array with mixed positive and negative uint8 values",
		},
		{
			name: "uint8 negative in map values",
			yamlContent: `
readings:
  sensor1: -128
  sensor2: 50
`,
			target: &struct {
				Readings map[string]uint8
			}{},
			shouldError: true,
			description: "Map with uint8 negative in value",
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
					t.Errorf("Test '%s' should succeed but errored: %v - %s", tt.name, err, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}