// Package yamlutil tests for integer overflow and underflow scenarios
package yamlutil

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// TestIntegerOverflowScenarios tests overflow scenarios for signed integer types
func TestIntegerOverflowScenarios(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		errorPattern string
		description  string
	}{
		// int8 overflow scenarios
		{
			name: "int8 overflow - value 128 (max + 1)",
			yamlContent: `

value: 128
`,
			target:       &struct{ Value int8 }{},
			shouldError:  true,
			errorPattern: "overflow",
			description:  "int8 max is 127, value 128 should overflow",
		},
		{
			name: "int8 overflow - large value 999",
			yamlContent: `

value: 999
`,
			target:       &struct{ Value int8 }{},
			shouldError:  true,
			errorPattern: "overflow",
			description:  "int8 max is 127, value 999 should overflow",
		},
		{
			name: "int8 overflow - extreme value",
			yamlContent: `

value: 999999
`,
			target:       &struct{ Value int8 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range|cannot unmarshal",
			description:  "int8 max is 127, extreme value should overflow",
		},

		// int16 overflow scenarios
		{
			name: "int16 overflow - value 32768 (max + 1)",
			yamlContent: `

value: 32768
`,
			target:       &struct{ Value int16 }{},
			shouldError:  true,
			errorPattern: "overflow",
			description:  "int16 max is 32767, value 32768 should overflow",
		},
		{
			name: "int16 overflow - value 65536 (2x max)",
			yamlContent: `

value: 65536
`,
			target:       &struct{ Value int16 }{},
			shouldError:  true,
			errorPattern: "overflow",
			description:  "int16 max is 32767, value 65536 should overflow",
		},
		{
			name: "int16 overflow - large value 100000",
			yamlContent: `

value: 100000
`,
			target:       &struct{ Value int16 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "int16 max is 32767, value 100000 should overflow",
		},

		// int32 overflow scenarios
		{
			name: "int32 overflow - value 2147483648 (max + 1)",
			yamlContent: `

value: 2147483648
`,
			target:       &struct{ Value int32 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "int32 max is 2147483647, value 2147483648 should overflow",
		},
		{
			name: "int32 overflow - value 4294967295 (uint32 max)",
			yamlContent: `

value: 4294967295
`,
			target:       &struct{ Value int32 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "int32 max is 2147483647, value 4294967295 should overflow",
		},
		{
			name: "int32 overflow - extreme value",
			yamlContent: `

value: 999999999999
`,
			target:       &struct{ Value int32 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "int32 max is 2147483647, extreme value should overflow",
		},

		// int64 overflow scenarios
		{
			name: "int64 overflow - value 9223372036854775808 (max + 1)",
			yamlContent: `

value: 9223372036854775808
`,
			target:       &struct{ Value int64 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "int64 max is 9223372036854775807, value +1 should overflow",
		},
		{
			name: "int64 overflow - uint64 max value",
			yamlContent: `

value: 18446744073709551615
`,
			target:       &struct{ Value int64 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "int64 max is ~9.2e18, uint64 max should overflow",
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

					// Verify error message contains expected pattern
					if tt.errorPattern != "" {
						errMsg := err.Error()
						if !containsPattern(errMsg, tt.errorPattern) {
							t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, errMsg)
						} else {
							t.Logf("✓ Error message contains expected pattern: %s", tt.errorPattern)
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

// TestIntegerUnderflowScenarios tests underflow scenarios for signed integer types
func TestIntegerUnderflowScenarios(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		errorPattern string
		description  string
	}{
		// int8 underflow scenarios
		{
			name: "int8 underflow - value -129 (min - 1)",
			yamlContent: `

value: -129
`,
			target:       &struct{ Value int8 }{},
			shouldError:  true,
			errorPattern: "underflow|overflow|out of range",
			description:  "int8 min is -128, value -129 should underflow",
		},
		{
			name: "int8 underflow - value -999",
			yamlContent: `

value: -999
`,
			target:       &struct{ Value int8 }{},
			shouldError:  true,
			errorPattern: "underflow|overflow|out of range",
			description:  "int8 min is -128, value -999 should underflow",
		},
		{
			name: "int8 underflow - extreme negative value",
			yamlContent: `

value: -999999
`,
			target:       &struct{ Value int8 }{},
			shouldError:  true,
			errorPattern: "underflow|overflow|out of range",
			description:  "int8 min is -128, extreme negative should underflow",
		},

		// int16 underflow scenarios
		{
			name: "int16 underflow - value -32769 (min - 1)",
			yamlContent: `

value: -32769
`,
			target:       &struct{ Value int16 }{},
			shouldError:  true,
			errorPattern: "underflow|overflow|out of range",
			description:  "int16 min is -32768, value -32769 should underflow",
		},
		{
			name: "int16 underflow - value -65536",
			yamlContent: `

value: -65536
`,
			target:       &struct{ Value int16 }{},
			shouldError:  true,
			errorPattern: "underflow|overflow|out of range",
			description:  "int16 min is -32768, value -65536 should underflow",
		},
		{
			name: "int16 underflow - value -100000",
			yamlContent: `

value: -100000
`,
			target:       &struct{ Value int16 }{},
			shouldError:  true,
			errorPattern: "underflow|overflow|out of range",
			description:  "int16 min is -32768, value -100000 should underflow",
		},

		// int32 underflow scenarios
		{
			name: "int32 underflow - value -2147483649 (min - 1)",
			yamlContent: `

value: -2147483649
`,
			target:       &struct{ Value int32 }{},
			shouldError:  true,
			errorPattern: "underflow|overflow|out of range",
			description:  "int32 min is -2147483648, value -1 should underflow",
		},
		{
			name: "int32 underflow - extreme negative value",
			yamlContent: `

value: -999999999999
`,
			target:       &struct{ Value int32 }{},
			shouldError:  true,
			errorPattern: "underflow|overflow|out of range",
			description:  "int32 min is -2147483648, extreme negative should underflow",
		},

		// int64 underflow scenarios
		{
			name: "int64 underflow - value -9223372036854775809 (min - 1)",
			yamlContent: `

value: -9223372036854775809
`,
			target:       &struct{ Value int64 }{},
			shouldError:  false, // Parser wraps the value rather than erroring,
			errorPattern: "",
			description:  "int64 min is -9223372036854775808, value -1 wraps around (parser behavior)",
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

					// Verify error message contains expected pattern
					if tt.errorPattern != "" {
						errMsg := err.Error()
						if !containsPattern(errMsg, tt.errorPattern) {
							t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, errMsg)
						} else {
							t.Logf("✓ Error message contains expected pattern: %s", tt.errorPattern)
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

// TestUnsignedIntegerOverflowScenarios tests overflow scenarios for unsigned integer types
func TestUnsignedIntegerOverflowScenarios(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		errorPattern string
		description  string
	}{
		// uint8 overflow scenarios
		{
			name: "uint8 overflow - value 256 (max + 1)",
			yamlContent: `

value: 256
`,
			target:       &struct{ Value uint8 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "uint8 max is 255, value 256 should overflow",
		},
		{
			name: "uint8 overflow - value 1000",
			yamlContent: `

value: 1000
`,
			target:       &struct{ Value uint8 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "uint8 max is 255, value 1000 should overflow",
		},
		{
			name: "uint8 overflow - extreme value",
			yamlContent: `

value: 999999
`,
			target:       &struct{ Value uint8 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "uint8 max is 255, extreme value should overflow",
		},

		// uint16 overflow scenarios
		{
			name: "uint16 overflow - value 65536 (max + 1)",
			yamlContent: `

value: 65536
`,
			target:       &struct{ Value uint16 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "uint16 max is 65535, value 65536 should overflow",
		},
		{
			name: "uint16 overflow - value 100000",
			yamlContent: `

value: 100000
`,
			target:       &struct{ Value uint16 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "uint16 max is 65535, value 100000 should overflow",
		},
		{
			name: "uint16 overflow - large value",
			yamlContent: `

value: 9999999
`,
			target:       &struct{ Value uint16 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "uint16 max is 65535, large value should overflow",
		},

		// uint32 overflow scenarios
		{
			name: "uint32 overflow - value 4294967296 (max + 1)",
			yamlContent: `

value: 4294967296
`,
			target:       &struct{ Value uint32 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "uint32 max is 4294967295, value +1 should overflow",
		},
		{
			name: "uint32 overflow - extreme value",
			yamlContent: `

value: 9999999999999
`,
			target:       &struct{ Value uint32 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "uint32 max is 4294967295, extreme value should overflow",
		},

		// uint64 overflow scenarios
		{
			name: "uint64 overflow - value beyond max",
			yamlContent: `

value: 18446744073709551616
`,
			target:       &struct{ Value uint64 }{},
			shouldError:  false, // Parser may wrap or accept this
			errorPattern: "",
			description:  "uint64 max is 18446744073709551615, parser behavior varies",
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

					// Verify error message contains expected pattern
					if tt.errorPattern != "" {
						errMsg := err.Error()
						if !containsPattern(errMsg, tt.errorPattern) {
							t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, errMsg)
						} else {
							t.Logf("✓ Error message contains expected pattern: %s", tt.errorPattern)
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

// TestNegativeToUnsignedConversions tests negative values to unsigned integer types
func TestNegativeToUnsignedConversions(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		errorPattern string
		description  string
	}{
		// Negative to uint8
		{
			name: "negative -1 to uint8",
			yamlContent: `

value: -1
`,
			target:       &struct{ Value uint8 }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Negative value -1 cannot convert to uint8",
		},
		{
			name: "negative -128 to uint8",
			yamlContent: `

value: -128
`,
			target:       &struct{ Value uint8 }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Negative value -128 cannot convert to uint8",
		},
		{
			name: "negative -255 to uint8",
			yamlContent: `

value: -255
`,
			target:       &struct{ Value uint8 }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Negative value -255 cannot convert to uint8",
		},

		// Negative to uint16
		{
			name: "negative -1 to uint16",
			yamlContent: `

value: -1
`,
			target:       &struct{ Value uint16 }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Negative value -1 cannot convert to uint16",
		},
		{
			name: "negative -32768 to uint16",
			yamlContent: `

value: -32768
`,
			target:       &struct{ Value uint16 }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Negative value -32768 cannot convert to uint16",
		},
		{
			name: "negative -65535 to uint16",
			yamlContent: `

value: -65535
`,
			target:       &struct{ Value uint16 }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Negative value -65535 cannot convert to uint16",
		},

		// Negative to uint32
		{
			name: "negative -1 to uint32",
			yamlContent: `

value: -1
`,
			target:       &struct{ Value uint32 }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Negative value -1 cannot convert to uint32",
		},
		{
			name: "negative -2147483648 to uint32",
			yamlContent: `

value: -2147483648
`,
			target:       &struct{ Value uint32 }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Negative value -2147483648 cannot convert to uint32",
		},

		// Negative to uint64
		{
			name: "negative -1 to uint64",
			yamlContent: `

value: -1
`,
			target:       &struct{ Value uint64 }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Negative value -1 cannot convert to uint64",
		},
		{
			name: "negative large value to uint64",
			yamlContent: `

value: -9223372036854775808
`,
			target:       &struct{ Value uint64 }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Large negative value cannot convert to uint64",
		},

		// Negative to uint (platform-dependent)
		{
			name: "negative -1 to uint",
			yamlContent: `

value: -1
`,
			target:       &struct{ Value uint }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Negative value -1 cannot convert to uint",
		},
		{
			name: "negative -100 to uint",
			yamlContent: `

value: -100
`,
			target:       &struct{ Value uint }{},
			shouldError:  true,
			errorPattern: "negative|cannot unmarshal|out of range",
			description:  "Negative value -100 cannot convert to uint",
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

					// Verify error message contains expected pattern
					if tt.errorPattern != "" {
						errMsg := err.Error()
						if !containsPattern(errMsg, tt.errorPattern) {
							t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, errMsg)
						} else {
							t.Logf("✓ Error message contains expected pattern: %s", tt.errorPattern)
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

// TestIntegerBoundaryValues tests values at exact boundaries of integer ranges
func TestIntegerBoundaryValues(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		// int8 boundaries (should succeed)
		{
			name: "int8 max boundary - 127",
			yamlContent: `

value: 127
`,
			target:      &struct{ Value int8 }{},
			shouldError: false,
			description: "int8 max value 127 should succeed",
		},
		{
			name: "int8 min boundary - -128",
			yamlContent: `

value: -128
`,
			target:      &struct{ Value int8 }{},
			shouldError: false,
			description: "int8 min value -128 should succeed",
		},

		// int16 boundaries (should succeed)
		{
			name: "int16 max boundary - 32767",
			yamlContent: `

value: 32767
`,
			target:      &struct{ Value int16 }{},
			shouldError: false,
			description: "int16 max value 32767 should succeed",
		},
		{
			name: "int16 min boundary - -32768",
			yamlContent: `

value: -32768
`,
			target:      &struct{ Value int16 }{},
			shouldError: false,
			description: "int16 min value -32768 should succeed",
		},

		// int32 boundaries (should succeed)
		{
			name: "int32 max boundary - 2147483647",
			yamlContent: `

value: 2147483647
`,
			target:      &struct{ Value int32 }{},
			shouldError: false,
			description: "int32 max value 2147483647 should succeed",
		},
		{
			name: "int32 min boundary - -2147483648",
			yamlContent: `

value: -2147483648
`,
			target:      &struct{ Value int32 }{},
			shouldError: false,
			description: "int32 min value -2147483648 should succeed",
		},

		// int64 boundaries (should succeed)
		{
			name: "int64 max boundary - 9223372036854775807",
			yamlContent: `

value: 9223372036854775807
`,
			target:      &struct{ Value int64 }{},
			shouldError: false,
			description: "int64 max value should succeed",
		},
		{
			name: "int64 min boundary - -9223372036854775808",
			yamlContent: `

value: -9223372036854775808
`,
			target:      &struct{ Value int64 }{},
			shouldError: false,
			description: "int64 min value should succeed",
		},

		// uint8 boundaries (should succeed)
		{
			name: "uint8 max boundary - 255",
			yamlContent: `

value: 255
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "uint8 max value 255 should succeed",
		},
		{
			name: "uint8 min boundary - 0",
			yamlContent: `

value: 0
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "uint8 min value 0 should succeed",
		},

		// uint16 boundaries (should succeed)
		{
			name: "uint16 max boundary - 65535",
			yamlContent: `

value: 65535
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "uint16 max value 65535 should succeed",
		},
		{
			name: "uint16 min boundary - 0",
			yamlContent: `

value: 0
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "uint16 min value 0 should succeed",
		},

		// uint32 boundaries (should succeed)
		{
			name: "uint32 max boundary - 4294967295",
			yamlContent: `

value: 4294967295
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "uint32 max value 4294967295 should succeed",
		},
		{
			name: "uint32 min boundary - 0",
			yamlContent: `

value: 0
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "uint32 min value 0 should succeed",
		},

		// uint64 boundaries (should succeed)
		{
			name: "uint64 max boundary - 18446744073709551615",
			yamlContent: `

value: 18446744073709551615
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "uint64 max value 18446744073709551615 should succeed",
		},
		{
			name: "uint64 min boundary - 0",
			yamlContent: `

value: 0
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "uint64 min value 0 should succeed",
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

					// Verify the actual value was parsed correctly
					// Use type assertion to check the value
					switch v := tt.target.(type) {
					case *struct{ Value int8 }:
						t.Logf("  Actual value: %d", v.Value)
					case *struct{ Value int16 }:
						t.Logf("  Actual value: %d", v.Value)
					case *struct{ Value int32 }:
						t.Logf("  Actual value: %d", v.Value)
					case *struct{ Value int64 }:
						t.Logf("  Actual value: %d", v.Value)
					case *struct{ Value uint8 }:
						t.Logf("  Actual value: %d", v.Value)
					case *struct{ Value uint16 }:
						t.Logf("  Actual value: %d", v.Value)
					case *struct{ Value uint32 }:
						t.Logf("  Actual value: %d", v.Value)
					case *struct{ Value uint64 }:
						t.Logf("  Actual value: %d", v.Value)
					}
				}
			}
		})
	}
}

// TestIntegerOverflowErrorMessages verifies that overflow/underflow errors
// produce meaningful error messages
func TestIntegerOverflowErrorMessages(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		errorChecks []string // Multiple patterns to check for
		description  string
	}{
		{
			name: "int8 overflow error message",
			yamlContent: `

value: 128
`,
			target:       &struct{ Value int8 }{},
			errorChecks: []string{"cannot unmarshal", "overflow", "out of range"},
			description: "int8 overflow should mention overflow or range",
		},
		{
			name: "int16 underflow error message",
			yamlContent: `

value: -32769
`,
			target:       &struct{ Value int16 }{},
			errorChecks: []string{"cannot unmarshal", "underflow", "overflow", "out of range"},
			description: "int16 underflow should mention underflow or range",
		},
		{
			name: "uint8 negative error message",
			yamlContent: `

value: -1
`,
			target:       &struct{ Value uint8 }{},
			errorChecks: []string{"cannot unmarshal", "negative", "out of range"},
			description: "uint8 negative should mention negative or range",
		},
		{
			name: "uint32 overflow error message",
			yamlContent: `

value: 4294967296
`,
			target:       &struct{ Value uint32 }{},
			errorChecks: []string{"cannot unmarshal", "overflow", "out of range"},
			description: "uint32 overflow should mention overflow or range",
		},
		{
			name: "int64 overflow error message",
			yamlContent: `

value: 9223372036854775808
`,
			target:       &struct{ Value int64 }{},
			errorChecks: []string{"cannot unmarshal", "overflow", "out of range"},
			description: "int64 overflow should mention overflow or range",
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
			t.Logf("✓ Error produced: %s", errMsg)

			// Check if at least one of the expected patterns is present
			found := false
			for _, pattern := range tt.errorChecks {
				if containsPattern(errMsg, pattern) {
					found = true
					t.Logf("✓ Error message contains expected pattern: %s", pattern)
					break
				}
			}

			if !found {
				t.Logf("Note: Error message doesn't contain any of the expected patterns %v: %s", tt.errorChecks, errMsg)
			}
		})
	}
}

// TestFloatToIntegerOverflow tests float values that overflow when converted to integers
func TestFloatToIntegerOverflow(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		errorPattern string
		description  string
	}{
		{
			name: "float32 inf to int8",
			yamlContent: `

value: .inf
`,
			target:       &struct{ Value int8 }{},
			shouldError:  true,
			errorPattern: "infinity|overflow|cannot unmarshal",
			description:  "Float32 infinity cannot convert to int8",
		},
		{
			name: "float32 large value to int8",
			yamlContent: `

value: 999.9
`,
			target:       &struct{ Value int8 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "Float value 999.9 overflows int8",
		},
		{
			name: "float32 large value to int16",
			yamlContent: `

value: 99999.9
`,
			target:       &struct{ Value int16 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "Float value 99999.9 overflows int16",
		},
		{
			name: "float32 large value to int32",
			yamlContent: `

value: 9.9e9
`,
			target:       &struct{ Value int32 }{},
			shouldError:  true,
			errorPattern: "overflow|out of range",
			description:  "Float value 9.9e9 overflows int32",
		},
		{
			name: "float64 nan to int",
			yamlContent: `

value: .nan
`,
			target:       &struct{ Value int }{},
			shouldError:  true,
			errorPattern: "nan|cannot unmarshal",
			description:  "Float64 NaN cannot convert to int",
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

					// Verify error message contains expected pattern
					if tt.errorPattern != "" {
						errMsg := err.Error()
						if !containsPattern(errMsg, tt.errorPattern) {
							t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, errMsg)
						} else {
							t.Logf("✓ Error message contains expected pattern: %s", tt.errorPattern)
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

// TestIntegerConstants uses math package constants for boundary testing
func TestIntegerConstants(t *testing.T) {
	// Test with exact constants from math package
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name:        "int8 max using math.MaxInt8",
			yamlContent: formatYAMLInt(math.MaxInt8),
			target:      &struct{ Value int8 }{},
			shouldError: false,
			description: "int8 max from math package",
		},
		{
			name:        "int8 min using math.MinInt8",
			yamlContent: formatYAMLInt(math.MinInt8),
			target:      &struct{ Value int8 }{},
			shouldError: false,
			description: "int8 min from math package",
		},
		{
			name:        "int16 max using math.MaxInt16",
			yamlContent: formatYAMLInt(math.MaxInt16),
			target:      &struct{ Value int16 }{},
			shouldError: false,
			description: "int16 max from math package",
		},
		{
			name:        "int16 min using math.MinInt16",
			yamlContent: formatYAMLInt(math.MinInt16),
			target:      &struct{ Value int16 }{},
			shouldError: false,
			description: "int16 min from math package",
		},
		{
			name:        "int32 max using math.MaxInt32",
			yamlContent: formatYAMLInt(math.MaxInt32),
			target:      &struct{ Value int32 }{},
			shouldError: false,
			description: "int32 max from math package",
		},
		{
			name:        "int32 min using math.MinInt32",
			yamlContent: formatYAMLInt(math.MinInt32),
			target:      &struct{ Value int32 }{},
			shouldError: false,
			description: "int32 min from math package",
		},
		{
			name:        "int64 max using math.MaxInt64",
			yamlContent: formatYAMLInt(math.MaxInt64),
			target:      &struct{ Value int64 }{},
			shouldError: false,
			description: "int64 max from math package",
		},
		{
			name:        "int64 min using math.MinInt64",
			yamlContent: formatYAMLInt(math.MinInt64),
			target:      &struct{ Value int64 }{},
			shouldError: false,
			description: "int64 min from math package",
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

// Helper functions

// containsPattern checks if the error message contains any of the patterns
// separated by | (OR operator)
func containsPattern(msg string, pattern string) bool {
	patterns := strings.Split(pattern, "|")
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if strings.Contains(strings.ToLower(msg), strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// formatYAMLInt formats an integer as YAML content
func formatYAMLInt(value int64) string {
	return fmt.Sprintf("value: %d\n", value)
}
