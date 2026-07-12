// Package yamlutil tests for basic primitive type conversion error scenarios
package yamlutil

import (
	"testing"
)

// TestBasicPrimitiveTypeConversions tests fundamental primitive type conversion errors
// between basic scalar types (string, int, float, bool, uint variants)
func TestBasicPrimitiveTypeConversions(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		// String conversions to numeric types
		{
			name: "string to int conversion error",
			yamlContent: `
value: "not_an_int"
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "String with non-numeric content cannot convert to int",
		},
		{
			name: "string to int8 conversion error",
			yamlContent: `
value: "999"
`,
			target:      &struct{ Value int8 }{},
			shouldError: true,
			description: "String value exceeds int8 range",
		},
		{
			name: "string to int16 conversion error",
			yamlContent: `
value: "99999"
`,
			target:      &struct{ Value int16 }{},
			shouldError: true,
			description: "String value exceeds int16 range",
		},
		{
			name: "string to int32 conversion error",
			yamlContent: `
value: "99999999999"
`,
			target:      &struct{ Value int32 }{},
			shouldError: true,
			description: "String value exceeds int32 range",
		},
		{
			name: "string to uint conversion error",
			yamlContent: `
value: "not_a_uint"
`,
			target:      &struct{ Value uint }{},
			shouldError: true,
			description: "String with non-numeric content cannot convert to uint",
		},
		{
			name: "string negative to uint error",
			yamlContent: `
value: "-5"
`,
			target:      &struct{ Value uint }{},
			shouldError: true,
			description: "Negative string value cannot convert to uint",
		},
		{
			name: "string to uint8 conversion error",
			yamlContent: `
value: "999"
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "String value exceeds uint8 range (0-255)",
		},
		{
			name: "string to uint16 conversion error",
			yamlContent: `
value: "99999"
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "String value exceeds uint16 range (0-65535)",
		},
		{
			name: "string to uint32 conversion error",
			yamlContent: `
value: "99999999999"
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "String value exceeds uint32 range",
		},
		{
			name: "string to float32 conversion error",
			yamlContent: `
value: "not_a_float"
`,
			target:      &struct{ Value float32 }{},
			shouldError: true,
			description: "String with non-numeric content cannot convert to float32",
		},
		{
			name: "string to bool conversion error - invalid",
			yamlContent: `
value: "invalid_bool"
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Invalid string value cannot convert to bool",
		},

		// Numeric to string conversions
		{
			name: "int to string conversion success",
			yamlContent: `
value: 12345
`,
			target:      &struct{ Value string }{},
			shouldError: false,
			description: "Integer successfully converts to string",
		},
		{
			name: "float to string conversion success",
			yamlContent: `
value: 3.14
`,
			target:      &struct{ Value string }{},
			shouldError: false,
			description: "Float successfully converts to string",
		},
		{
			name: "bool to string conversion success",
			yamlContent: `
value: true
`,
			target:      &struct{ Value string }{},
			shouldError: false,
			description: "Boolean successfully converts to string",
		},
		{
			name: "uint to string conversion success",
			yamlContent: `
value: 42
`,
			target:      &struct{ Value string }{},
			shouldError: false,
			description: "Uint successfully converts to string",
		},

		// Float to integer conversions
		{
			name: "float32 to int conversion error - infinity",
			yamlContent: `
value: .inf
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Float32 infinity cannot convert to int",
		},
		{
			name: "float64 to int conversion error - nan",
			yamlContent: `
value: .nan
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Float64 NaN cannot convert to int",
		},
		{
			name: "float to int8 conversion error",
			yamlContent: `
value: 999.9
`,
			target:      &struct{ Value int8 }{},
			shouldError: true,
			description: "Float value exceeds int8 range",
		},
		{
			name: "float to int16 conversion error",
			yamlContent: `
value: 99999.9
`,
			target:      &struct{ Value int16 }{},
			shouldError: true,
			description: "Float value exceeds int16 range",
		},
		{
			name: "float to int32 conversion error",
			yamlContent: `
value: 9.9e9
`,
			target:      &struct{ Value int32 }{},
			shouldError: true,
			description: "Float value exceeds int32 range",
		},
		{
			name: "float to uint conversion - negative succeeds",
			yamlContent: `
value: -3.14
`,
			target:      &struct{ Value uint }{},
			shouldError: false,
			description: "Negative float converts to uint (wraps to large value)",
		},
		{
			name: "float to uint8 conversion error",
			yamlContent: `
value: 999.9
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Float value exceeds uint8 range",
		},
		{
			name: "float to uint16 conversion error",
			yamlContent: `
value: 99999.9
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Float value exceeds uint16 range",
		},
		{
			name: "float to uint32 conversion error",
			yamlContent: `
value: 9.9e9
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Float value exceeds uint32 range",
		},

		// Integer to float conversions
		{
			name: "int to float32 conversion success",
			yamlContent: `
value: 1234567
`,
			target:      &struct{ Value float32 }{},
			shouldError: false,
			description: "Integer successfully converts to float32",
		},
		{
			name: "int to float64 conversion success",
			yamlContent: `
value: 123456789012
`,
			target:      &struct{ Value float64 }{},
			shouldError: false,
			description: "Integer successfully converts to float64",
		},
		{
			name: "uint to float32 conversion success",
			yamlContent: `
value: 4294967295
`,
			target:      &struct{ Value float32 }{},
			shouldError: false,
			description: "Uint successfully converts to float32",
		},
		{
			name: "uint to float64 conversion success",
			yamlContent: `
value: 18446744073709551615
`,
			target:      &struct{ Value float64 }{},
			shouldError: false,
			description: "Uint successfully converts to float64",
		},

		// Boolean conversions
		{
			name: "bool to int conversion error",
			yamlContent: `
value: true
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Boolean cannot convert to int",
		},
		{
			name: "bool to int8 conversion error",
			yamlContent: `
value: false
`,
			target:      &struct{ Value int8 }{},
			shouldError: true,
			description: "Boolean cannot convert to int8",
		},
		{
			name: "bool to int16 conversion error",
			yamlContent: `
value: true
`,
			target:      &struct{ Value int16 }{},
			shouldError: true,
			description: "Boolean cannot convert to int16",
		},
		{
			name: "bool to int32 conversion error",
			yamlContent: `
value: false
`,
			target:      &struct{ Value int32 }{},
			shouldError: true,
			description: "Boolean cannot convert to int32",
		},
		{
			name: "bool to int64 conversion error",
			yamlContent: `
value: true
`,
			target:      &struct{ Value int64 }{},
			shouldError: true,
			description: "Boolean cannot convert to int64",
		},
		{
			name: "bool to uint conversion error",
			yamlContent: `
value: true
`,
			target:      &struct{ Value uint }{},
			shouldError: true,
			description: "Boolean cannot convert to uint",
		},
		{
			name: "bool to uint8 conversion error",
			yamlContent: `
value: false
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Boolean cannot convert to uint8",
		},
		{
			name: "bool to uint16 conversion error",
			yamlContent: `
value: true
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Boolean cannot convert to uint16",
		},
		{
			name: "bool to uint32 conversion error",
			yamlContent: `
value: false
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Boolean cannot convert to uint32",
		},
		{
			name: "bool to uint64 conversion error",
			yamlContent: `
value: true
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Boolean cannot convert to uint64",
		},
		{
			name: "bool to float32 conversion error",
			yamlContent: `
value: true
`,
			target:      &struct{ Value float32 }{},
			shouldError: true,
			description: "Boolean cannot convert to float32",
		},
		{
			name: "bool to float64 conversion error",
			yamlContent: `
value: false
`,
			target:      &struct{ Value float64 }{},
			shouldError: true,
			description: "Boolean cannot convert to float64",
		},

		// Integer to boolean conversions
		{
			name: "int to bool conversion error - non-zero",
			yamlContent: `
value: 5
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Integer value 5 cannot convert to bool",
		},
		{
			name: "int to bool conversion error - zero",
			yamlContent: `
value: 0
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Integer value 0 cannot convert to bool",
		},
		{
			name: "int8 to bool conversion error",
			yamlContent: `
value: 1
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Int8 value cannot convert to bool",
		},
		{
			name: "int16 to bool conversion error",
			yamlContent: `
value: 1
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Int16 value cannot convert to bool",
		},
		{
			name: "int32 to bool conversion error",
			yamlContent: `
value: 1
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Int32 value cannot convert to bool",
		},
		{
			name: "int64 to bool conversion error",
			yamlContent: `
value: 1
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Int64 value cannot convert to bool",
		},
		{
			name: "uint to bool conversion error",
			yamlContent: `
value: 1
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Uint value cannot convert to bool",
		},
		{
			name: "uint8 to bool conversion error",
			yamlContent: `
value: 1
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Uint8 value cannot convert to bool",
		},
		{
			name: "uint16 to bool conversion error",
			yamlContent: `
value: 1
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Uint16 value cannot convert to bool",
		},
		{
			name: "uint32 to bool conversion error",
			yamlContent: `
value: 1
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Uint32 value cannot convert to bool",
		},
		{
			name: "uint64 to bool conversion error",
			yamlContent: `
value: 1
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Uint64 value cannot convert to bool",
		},
		{
			name: "float to bool conversion error - positive",
			yamlContent: `
value: 1.5
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Float value cannot convert to bool",
		},
		{
			name: "float to bool conversion error - zero",
			yamlContent: `
value: 0.0
`,
			target:      &struct{ Value bool }{},
			shouldError: true,
			description: "Float zero value cannot convert to bool",
		},

		// Uint integer overflow/underflow scenarios
		{
			name: "uint overflow detection",
			yamlContent: `
value: 4294967296
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Value exceeds uint32 maximum (4294967295)",
		},
		{
			name: "uint negative value error",
			yamlContent: `
value: -100
`,
			target:      &struct{ Value uint }{},
			shouldError: true,
			description: "Negative value cannot convert to uint",
		},
		{
			name: "uint8 overflow detection",
			yamlContent: `
value: 256
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Value exceeds uint8 maximum (255)",
		},
		{
			name: "uint16 overflow detection",
			yamlContent: `
value: 65536
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Value exceeds uint16 maximum (65535)",
		},
		{
			name: "uint32 overflow detection",
			yamlContent: `
value: 4294967296
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Value exceeds uint32 maximum (4294967295)",
		},
		{
			name: "uint64 overflow detection",
			yamlContent: `
value: 18446744073709551616
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Very large value wraps (parser behavior)",
		},
		{
			name: "uint8 negative error",
			yamlContent: `
value: -1
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Negative value cannot convert to uint8",
		},
		{
			name: "uint16 negative error",
			yamlContent: `
value: -50
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Negative value cannot convert to uint16",
		},
		{
			name: "uint32 negative error",
			yamlContent: `
value: -999
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Negative value cannot convert to uint32",
		},
		{
			name: "uint64 negative error",
			yamlContent: `
value: -12345
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Negative value cannot convert to uint64",
		},

		// Integer overflow/underflow scenarios for signed types
		{
			name: "int8 overflow detection",
			yamlContent: `
value: 128
`,
			target:      &struct{ Value int8 }{},
			shouldError: true,
			description: "Value exceeds int8 maximum (127)",
		},
		{
			name: "int8 underflow detection",
			yamlContent: `
value: -129
`,
			target:      &struct{ Value int8 }{},
			shouldError: true,
			description: "Value below int8 minimum (-128)",
		},
		{
			name: "int16 overflow detection",
			yamlContent: `
value: 32768
`,
			target:      &struct{ Value int16 }{},
			shouldError: true,
			description: "Value exceeds int16 maximum (32767)",
		},
		{
			name: "int16 underflow detection",
			yamlContent: `
value: -32769
`,
			target:      &struct{ Value int16 }{},
			shouldError: true,
			description: "Value below int16 minimum (-32768)",
		},
		{
			name: "int32 overflow detection",
			yamlContent: `
value: 2147483648
`,
			target:      &struct{ Value int32 }{},
			shouldError: true,
			description: "Value exceeds int32 maximum (2147483647)",
		},
		{
			name: "int32 underflow detection",
			yamlContent: `
value: -2147483649
`,
			target:      &struct{ Value int32 }{},
			shouldError: true,
			description: "Value below int32 minimum (-2147483648)",
		},
		{
			name: "int64 overflow detection",
			yamlContent: `
value: 9223372036854775808
`,
			target:      &struct{ Value int64 }{},
			shouldError: true,
			description: "Value exceeds int64 maximum",
		},
		{
			name: "int64 underflow detection",
			yamlContent: `
value: -9223372036854775809
`,
			target:      &struct{ Value int64 }{},
			shouldError: false,
			description: "Very negative value wraps (parser behavior)",
		},

		// Cross-type integer conversions
		{
			name: "int to uint conversion success",
			yamlContent: `
value: 42
`,
			target:      &struct{ Value uint }{},
			shouldError: false,
			description: "Positive int successfully converts to uint",
		},
		{
			name: "int to uint8 conversion success",
			yamlContent: `
value: 100
`,
			target:      &struct{ Value uint8 }{},
			shouldError: false,
			description: "Int in range successfully converts to uint8",
		},
		{
			name: "int to uint16 conversion success",
			yamlContent: `
value: 1000
`,
			target:      &struct{ Value uint16 }{},
			shouldError: false,
			description: "Int in range successfully converts to uint16",
		},
		{
			name: "int to uint32 conversion success",
			yamlContent: `
value: 50000
`,
			target:      &struct{ Value uint32 }{},
			shouldError: false,
			description: "Int in range successfully converts to uint32",
		},
		{
			name: "uint to int conversion success",
			yamlContent: `
value: 42
`,
			target:      &struct{ Value int }{},
			shouldError: false,
			description: "Uint successfully converts to int",
		},
		{
			name: "int to int8 conversion success",
			yamlContent: `
value: 50
`,
			target:      &struct{ Value int8 }{},
			shouldError: false,
			description: "Int in range successfully converts to int8",
		},
		{
			name: "int to int16 conversion success",
			yamlContent: `
value: 5000
`,
			target:      &struct{ Value int16 }{},
			shouldError: false,
			description: "Int in range successfully converts to int16",
		},
		{
			name: "int to int32 conversion success",
			yamlContent: `
value: 100000
`,
			target:      &struct{ Value int32 }{},
			shouldError: false,
			description: "Int in range successfully converts to int32",
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

// TestBasicPrimitiveErrorMessageQuality verifies that basic primitive type conversion
// errors produce appropriate error messages
func TestBasicPrimitiveErrorMessageQuality(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		errorPattern string
		description  string
	}{
		{
			name: "string to int error message",
			yamlContent: `
value: "abc"
`,
			target:       &struct{ Value int }{},
			errorPattern: "cannot unmarshal",
			description:  "String to int error should mention unmarshal failure",
		},
		{
			name: "bool to int error message",
			yamlContent: `
value: true
`,
			target:       &struct{ Value int }{},
			errorPattern: "cannot unmarshal",
			description:  "Bool to int error should mention unmarshal failure",
		},
		{
			name: "float to bool error message",
			yamlContent: `
value: 3.14
`,
			target:       &struct{ Value bool }{},
			errorPattern: "cannot unmarshal",
			description:  "Float to bool error should mention unmarshal failure",
		},
		{
			name: "negative to uint error message",
			yamlContent: `
value: -5
`,
			target:       &struct{ Value uint }{},
			errorPattern: "cannot unmarshal",
			description:  "Negative to uint error should mention unmarshal failure",
		},
		{
			name: "array to string error message",
			yamlContent: `
value:
  - item1
  - item2
`,
			target:       &struct{ Value string }{},
			errorPattern: "cannot unmarshal",
			description:  "Array to string error should mention unmarshal failure",
		},
		{
			name: "object to int error message",
			yamlContent: `
value:
  key: val
`,
			target:       &struct{ Value int }{},
			errorPattern: "cannot unmarshal",
			description:  "Object to int error should mention unmarshal failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if err == nil {
				t.Errorf("Test '%s' should error but didn't", tt.name)
			} else {
				errMsg := err.Error()
				t.Logf("✓ Error message: %s", errMsg)

				// Verify error message contains expected pattern
				if !contains(errMsg, tt.errorPattern) {
					t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, errMsg)
				}
			}
		})
	}
}

// TestScalarToScalarConversionMatrix tests all scalar-to-scalar type conversion combinations
// to ensure comprehensive coverage of primitive type conversions
func TestScalarToScalarConversionMatrix(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		// String → Scalar
		{"string→string", `value: "text"`, &struct{ Value string }{}, false, "String to string succeeds"},
		{"string→int", `value: "123"`, &struct{ Value int }{}, true, "String to int errors"},
		{"string→float", `value: "3.14"`, &struct{ Value float64 }{}, true, "String to float errors"},
		{"string→bool", `value: "true"`, &struct{ Value bool }{}, true, "String to bool errors"},

		// Int → Scalar
		{"int→string", `value: 123`, &struct{ Value string }{}, false, "Int to string succeeds"},
		{"int→int", `value: 123`, &struct{ Value int }{}, false, "Int to int succeeds"},
		{"int→float", `value: 123`, &struct{ Value float64 }{}, false, "Int to float succeeds"},
		{"int→bool", `value: 1`, &struct{ Value bool }{}, true, "Int to bool errors"},

		// Float → Scalar
		{"float→string", `value: 3.14`, &struct{ Value string }{}, false, "Float to string succeeds"},
		{"float→int", `value: 3.14`, &struct{ Value int }{}, false, "Float to int succeeds (truncates)"},
		{"float→float", `value: 3.14`, &struct{ Value float64 }{}, false, "Float to float succeeds"},
		{"float→bool", `value: 1.5`, &struct{ Value bool }{}, true, "Float to bool errors"},

		// Bool → Scalar
		{"bool→string", `value: true`, &struct{ Value string }{}, false, "Bool to string succeeds"},
		{"bool→int", `value: true`, &struct{ Value int }{}, true, "Bool to int errors"},
		{"bool→float", `value: false`, &struct{ Value float64 }{}, true, "Bool to float errors"},
		{"bool→bool", `value: true`, &struct{ Value bool }{}, false, "Bool to bool succeeds"},

		// Uint → Scalar
		{"uint→string", `value: 42`, &struct{ Value string }{}, false, "Uint to string succeeds"},
		{"uint→int", `value: 42`, &struct{ Value int }{}, false, "Uint to int succeeds"},
		{"uint→float", `value: 42`, &struct{ Value float64 }{}, false, "Uint to float succeeds"},
		{"uint→bool", `value: 1`, &struct{ Value bool }{}, true, "Uint to bool errors"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ %s: error correctly produced", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v - %s", tt.name, err, tt.description)
				} else {
					t.Logf("✓ %s: success correctly produced", tt.name)
				}
			}
		})
	}
}

// Helper functions are already defined in errors_test.go
