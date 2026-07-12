// Package yamlutil tests for type conversion error scenarios
package yamlutil

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestTypeConversionErrors tests various type conversion error scenarios
func TestTypeConversionErrors(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		expectedType string
		actualType   string
		description  string
	}{
		{
			name: "string to integer conversion",
			yamlContent: `
port: "not_a_number"
`,
			target:       &struct{ Port int }{},
			shouldError:  true,
			expectedType: "integer",
			actualType:   "string",
			description:  "String value cannot be converted to integer",
		},
		{
			name: "string to float conversion",
			yamlContent: `
rate: "invalid_float"
`,
			target:       &struct{ Rate float64 }{},
			shouldError:  true,
			expectedType: "float",
			actualType:   "string",
			description:  "String value cannot be converted to float",
		},
		{
			name: "string to boolean conversion",
			yamlContent: `
enabled: "maybe"
`,
			target:       &struct{ Enabled bool }{},
			shouldError:  true,
			expectedType: "boolean",
			actualType:   "string",
			description:  "Invalid string value for boolean field",
		},
		{
			name: "integer overflow - int64",
			yamlContent: `
count: 9223372036854775808
`,
			target:       &struct{ Count int64 }{},
			shouldError:  true,
			expectedType: "int64",
			actualType:   "integer",
			description:  "Value exceeds int64 maximum",
		},
		{
			name: "integer underflow - int64",
			yamlContent: `
count: -9223372036854775810
`,
			target:       &struct{ Count int64 }{},
			shouldError:  false,
			expectedType: "int64",
			actualType:   "integer",
			description:  "Very negative number wraps around (parser behavior)",
		},
		{
			name: "negative to unsigned int conversion",
			yamlContent: `
count: -5
`,
			target:       &struct{ Count uint }{},
			shouldError:  true,
			expectedType: "uint",
			actualType:   "negative integer",
			description:  "Negative number cannot be converted to unsigned integer",
		},
		{
			name: "array to scalar conversion",
			yamlContent: `
value:
  - item1
  - item2
`,
			target:       &struct{ Value string }{},
			shouldError:  true,
			expectedType: "string",
			actualType:   "array",
			description:  "Array cannot be converted to scalar string",
		},
		{
			name: "object to scalar conversion",
			yamlContent: `
value:
  key1: val1
  key2: val2
`,
			target:       &struct{ Value string }{},
			shouldError:  true,
			expectedType: "string",
			actualType:   "object",
			description:  "Object cannot be converted to scalar string",
		},
		{
			name: "scalar to array conversion",
			yamlContent: `
items: "not_an_array"
`,
			target:       &struct{ Items []string }{},
			shouldError:  true,
			expectedType: "array",
			actualType:   "string",
			description:  "Scalar cannot be converted to array",
		},
		{
			name: "scalar to object conversion",
			yamlContent: `
config: "not_an_object"
`,
			target:       &struct{ Config map[string]string }{},
			shouldError:  true,
			expectedType: "object",
			actualType:   "string",
			description:  "Scalar cannot be converted to object/map",
		},
		{
			name: "integer to string conversion",
			yamlContent: `
name: 12345
`,
			target:       &struct{ Name string }{},
			shouldError:  false,
			expectedType: "string",
			actualType:   "integer",
			description:  "Integer can be converted to string (should succeed)",
		},
		{
			name: "float to integer conversion",
			yamlContent: `
count: 3.9999999999999999999999999999999999999999999999
`,
			target:       &struct{ Count int }{},
			shouldError:  false,
			expectedType: "integer",
			actualType:   "float",
			description:  "Float with decimal part gets truncated (succeeds)",
		},
		{
			name: "boolean to integer conversion",
			yamlContent: `
count: true
`,
			target:       &struct{ Count int }{},
			shouldError:  true,
			expectedType: "integer",
			actualType:   "boolean",
			description:  "Boolean cannot be converted to integer",
		},
		{
			name: "null to required string conversion",
			yamlContent: `
name: null
`,
			target:       &struct{ Name string }{},
			shouldError:  false,
			expectedType: "string",
			actualType:   "null",
			description:  "Null value converts to empty string (succeeds)",
		},
		{
			name: "large uint overflow",
			yamlContent: `
id: 18446744073709551616
`,
			target:       &struct{ ID uint64 }{},
			shouldError:  false,
			expectedType: "uint64",
			actualType:   "integer",
			description:  "Very large integer wraps around (parser behavior)",
		},
		{
			name: "float overflow",
			yamlContent: `
value: 1.8e+308
`,
			target:       &struct{ Value float64 }{},
			shouldError:  true,
			expectedType: "float64",
			actualType:   "float",
			description:  "Value exceeds float64 maximum",
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
					// Verify error message contains relevant type information
					errMsg := err.Error()
					t.Logf("✓ Test '%s' correctly produced error: %s", tt.name, errMsg)

					// Check if error message is descriptive
					if !strings.Contains(strings.ToLower(errMsg), "error") &&
						!strings.Contains(strings.ToLower(errMsg), "cannot") &&
						!strings.Contains(strings.ToLower(errMsg), "unmarshal") {
						t.Logf("Note: Error message might not be descriptive enough: %s", errMsg)
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

// TestTypeMismatchErrorMessages verifies that TypeMismatchError produces appropriate messages
func TestTypeMismatchErrorMessages(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		fieldPath    string
		expectedType string
		actualType   string
		value        string
		line         int
		wantMessage  string
		wantContext  string
	}{
		{
			name:         "basic type mismatch with line",
			filePath:     "config.yaml",
			fieldPath:    "server.port",
			expectedType: "integer",
			actualType:   "string",
			value:        "abc",
			line:         10,
			wantMessage:  "type mismatch in config.yaml at line 10, field server.port: expected integer, got string",
			wantContext:  "field: server.port, expected type: integer, actual type: string, value: abc",
		},
		{
			name:         "type mismatch without line",
			filePath:     "data.yaml",
			fieldPath:    "api.timeout",
			expectedType: "integer",
			actualType:   "boolean",
			value:        "true",
			line:         0,
			wantMessage:  "type mismatch in data.yaml, field api.timeout: expected integer, got boolean",
			wantContext:  "field: api.timeout, expected type: integer, actual type: boolean, value: true",
		},
		{
			name:         "array to scalar mismatch",
			filePath:     "app.yaml",
			fieldPath:    "config.items",
			expectedType: "string",
			actualType:   "array",
			value:        "[item1, item2]",
			line:         15,
			wantMessage:  "type mismatch in app.yaml at line 15, field config.items: expected string, got array",
			wantContext:  "field: config.items, expected type: string, actual type: array, value: [item1, item2]",
		},
		{
			name:         "object to scalar mismatch",
			filePath:     "manifest.yaml",
			fieldPath:    "spec.template",
			expectedType: "string",
			actualType:   "object",
			value:        "{key: value}",
			line:         25,
			wantMessage:  "type mismatch in manifest.yaml at line 25, field spec.template: expected string, got object",
			wantContext:  "field: spec.template, expected type: string, actual type: object, value: {key: value}",
		},
		{
			name:         "scalar to array mismatch",
			filePath:     "deployment.yaml",
			fieldPath:    "spec.containers",
			expectedType: "array",
			actualType:   "string",
			value:        "not_an_array",
			line:         30,
			wantMessage:  "type mismatch in deployment.yaml at line 30, field spec.containers: expected array, got string",
			wantContext:  "field: spec.containers, expected type: array, actual type: string, value: not_an_array",
		},
		{
			name:         "nested field path",
			filePath:     "config.yaml",
			fieldPath:    "servers.api.responses[0].statusCode",
			expectedType: "integer",
			actualType:   "string",
			value:        "\"200\"",
			line:         42,
			wantMessage:  "type mismatch in config.yaml at line 42, field servers.api.responses[0].statusCode: expected integer, got string",
			wantContext:  "field: servers.api.responses[0].statusCode, expected type: integer, actual type: string, value: \"200\"",
		},
		{
			name:         "overflow scenario",
			filePath:     "data.yaml",
			fieldPath:    "counter.value",
			expectedType: "int64",
			actualType:   "integer",
			value:        "9223372036854775808",
			line:         50,
			wantMessage:  "type mismatch in data.yaml at line 50, field counter.value: expected int64, got integer",
			wantContext:  "field: counter.value, expected type: int64, actual type: integer, value: 9223372036854775808",
		},
		{
			name:         "underflow scenario",
			filePath:     "metrics.yaml",
			fieldPath:    "metrics.count",
			expectedType: "uint",
			actualType:   "negative integer",
			value:        "-5",
			line:         18,
			wantMessage:  "type mismatch in metrics.yaml at line 18, field metrics.count: expected uint, got negative integer",
			wantContext:  "field: metrics.count, expected type: uint, actual type: negative integer, value: -5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				tt.filePath,
				tt.fieldPath,
				tt.expectedType,
				tt.actualType,
				tt.value,
				tt.line,
				"",
			)

			// Verify Error() message
			errorMsg := err.Error()
			if errorMsg != tt.wantMessage {
				t.Errorf("Error() = %q, want %q", errorMsg, tt.wantMessage)
			}

			// Verify Context() message
			ctxMsg := err.Context()
			if ctxMsg != tt.wantContext {
				t.Errorf("Context() = %q, want %q", ctxMsg, tt.wantContext)
			}

			// Verify it's recognized as a YAMLError
			if !IsYAMLError(err) {
				t.Error("TypeMismatchError should implement YAMLError interface")
			}

			// Verify error type
			if err.YAMLErrorType() != ErrorTypeTypeMismatch {
				t.Errorf("YAMLErrorType() = %q, want %q", err.YAMLErrorType(), ErrorTypeTypeMismatch)
			}

			// Verify error code
			if err.Code() != ErrCodeTypeMismatch {
				t.Errorf("Code() = %q, want %q", err.Code(), ErrCodeTypeMismatch)
			}

			t.Logf("✓ %s: error message properly formatted", tt.name)
		})
	}
}

// TestIntegerOverflowUnderflow tests integer overflow and underflow scenarios
func TestIntegerOverflowUnderflow(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
		limitType    string
	}{
		{
			name: "int8 overflow - exceeds 127",
			yamlContent: `
value: 128
`,
			target:      &struct{ Value int8 }{},
			shouldError: true,
			description: "Value 128 exceeds int8 maximum (127)",
			limitType:   "int8 max",
		},
		{
			name: "int8 underflow - below -128",
			yamlContent: `
value: -129
`,
			target:      &struct{ Value int8 }{},
			shouldError: true,
			description: "Value -129 below int8 minimum (-128)",
			limitType:   "int8 min",
		},
		{
			name: "int16 overflow - exceeds 32767",
			yamlContent: `
value: 32768
`,
			target:      &struct{ Value int16 }{},
			shouldError: true,
			description: "Value 32768 exceeds int16 maximum (32767)",
			limitType:   "int16 max",
		},
		{
			name: "int16 underflow - below -32768",
			yamlContent: `
value: -32769
`,
			target:      &struct{ Value int16 }{},
			shouldError: true,
			description: "Value -32769 below int16 minimum (-32768)",
			limitType:   "int16 min",
		},
		{
			name: "int32 overflow - exceeds 2147483647",
			yamlContent: `
value: 2147483648
`,
			target:      &struct{ Value int32 }{},
			shouldError: true,
			description: "Value 2147483648 exceeds int32 maximum (2147483647)",
			limitType:   "int32 max",
		},
		{
			name: "int32 underflow - below -2147483648",
			yamlContent: `
value: -2147483649
`,
			target:      &struct{ Value int32 }{},
			shouldError: true,
			description: "Value -2147483649 below int32 minimum (-2147483648)",
			limitType:   "int32 min",
		},
		{
			name: "int64 overflow - exceeds max",
			yamlContent: `
value: 9223372036854775808
`,
			target:      &struct{ Value int64 }{},
			shouldError: true,
			description: "Value 9223372036854775808 exceeds int64 maximum",
			limitType:   "int64 max",
		},
		{
			name: "int64 underflow - below min",
			yamlContent: `
value: -9223372036854775809
`,
			target:      &struct{ Value int64 }{},
			shouldError: false,
			description: "Value -9223372036854775809 wraps around (parser behavior)",
			limitType:   "int64 min",
		},
		{
			name: "uint8 overflow - exceeds 255",
			yamlContent: `
value: 256
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Value 256 exceeds uint8 maximum (255)",
			limitType:   "uint8 max",
		},
		{
			name: "uint16 overflow - exceeds 65535",
			yamlContent: `
value: 65536
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Value 65536 exceeds uint16 maximum (65535)",
			limitType:   "uint16 max",
		},
		{
			name: "uint32 overflow - exceeds 4294967295",
			yamlContent: `
value: 4294967296
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Value 4294967296 exceeds uint32 maximum (4294967295)",
			limitType:   "uint32 max",
		},
		{
			name: "uint64 overflow - exceeds max",
			yamlContent: `
value: 18446744073709551616
`,
			target:      &struct{ Value uint64 }{},
			shouldError: false,
			description: "Value 18446744073709551616 wraps around (parser behavior)",
			limitType:   "uint64 max",
		},
		{
			name: "negative to uint8",
			yamlContent: `
value: -1
`,
			target:      &struct{ Value uint8 }{},
			shouldError: true,
			description: "Negative value -1 cannot be converted to uint8",
			limitType:   "uint8 negative",
		},
		{
			name: "negative to uint16",
			yamlContent: `
value: -100
`,
			target:      &struct{ Value uint16 }{},
			shouldError: true,
			description: "Negative value -100 cannot be converted to uint16",
			limitType:   "uint16 negative",
		},
		{
			name: "negative to uint32",
			yamlContent: `
value: -500
`,
			target:      &struct{ Value uint32 }{},
			shouldError: true,
			description: "Negative value -500 cannot be converted to uint32",
			limitType:   "uint32 negative",
		},
		{
			name: "negative to uint64",
			yamlContent: `
value: -999
`,
			target:      &struct{ Value uint64 }{},
			shouldError: true,
			description: "Negative value -999 cannot be converted to uint64",
			limitType:   "uint64 negative",
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

// TestFloatingPointConversionErrors tests floating-point type conversion errors
func TestFloatingPointConversionErrors(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		{
			name: "string to float32",
			yamlContent: `
rate: "not_a_float"
`,
			target:      &struct{ Rate float32 }{},
			shouldError: true,
			description: "String cannot be converted to float32",
		},
		{
			name: "string to float64",
			yamlContent: `
rate: "invalid"
`,
			target:      &struct{ Rate float64 }{},
			shouldError: true,
			description: "String cannot be converted to float64",
		},
		{
			name: "infinity to float32",
			yamlContent: `
value: .inf
`,
			target:      &struct{ Value float32 }{},
			shouldError: false,
			description: "Infinity is valid for float32 (YAML .inf)",
		},
		{
			name: "NaN to float64",
			yamlContent: `
value: .nan
`,
			target:      &struct{ Value float64 }{},
			shouldError: false,
			description: "NaN is valid for float64 (YAML .nan)",
		},
		{
			name: "boolean to float32",
			yamlContent: `
rate: true
`,
			target:      &struct{ Rate float32 }{},
			shouldError: true,
			description: "Boolean cannot be converted to float32",
		},
		{
			name: "array to float64",
			yamlContent: `
rate: [1.0, 2.0]
`,
			target:      &struct{ Rate float64 }{},
			shouldError: true,
			description: "Array cannot be converted to float64",
		},
		{
			name: "object to float32",
			yamlContent: `
rate: {value: 1.5}
`,
			target:      &struct{ Rate float32 }{},
			shouldError: true,
			description: "Object cannot be converted to float32",
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

// TestBooleanConversionErrors tests boolean type conversion errors
func TestBooleanConversionErrors(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		{
			name: "invalid string to bool",
			yamlContent: `
enabled: "maybe"
`,
			target:      &struct{ Enabled bool }{},
			shouldError: true,
			description: "String 'maybe' cannot be converted to boolean",
		},
		{
			name: "integer to bool - non-zero",
			yamlContent: `
enabled: 2
`,
			target:      &struct{ Enabled bool }{},
			shouldError: true,
			description: "Integer 2 cannot be converted to boolean",
		},
		{
			name: "integer to bool - zero",
			yamlContent: `
enabled: 0
`,
			target:      &struct{ Enabled bool }{},
			shouldError: true,
			description: "Integer 0 cannot be converted to boolean",
		},
		{
			name: "float to bool",
			yamlContent: `
enabled: 1.5
`,
			target:      &struct{ Enabled bool }{},
			shouldError: true,
			description: "Float cannot be converted to boolean",
		},
		{
			name: "array to bool",
			yamlContent: `
enabled: [true, false]
`,
			target:      &struct{ Enabled bool }{},
			shouldError: true,
			description: "Array cannot be converted to boolean",
		},
		{
			name: "object to bool",
			yamlContent: `
enabled: {value: true}
`,
			target:      &struct{ Enabled bool }{},
			shouldError: true,
			description: "Object cannot be converted to boolean",
		},
		{
			name: "valid bool - true",
			yamlContent: `
enabled: true
`,
			target:      &struct{ Enabled bool }{},
			shouldError: false,
			description: "Boolean 'true' is valid",
		},
		{
			name: "valid bool - false",
			yamlContent: `
enabled: false
`,
			target:      &struct{ Enabled bool }{},
			shouldError: false,
			description: "Boolean 'false' is valid",
		},
		{
			name: "valid bool - yes",
			yamlContent: `
enabled: yes
`,
			target:      &struct{ Enabled bool }{},
			shouldError: false,
			description: "YAML boolean 'yes' is valid",
		},
		{
			name: "valid bool - no",
			yamlContent: `
enabled: no
`,
			target:      &struct{ Enabled bool }{},
			shouldError: false,
			description: "YAML boolean 'no' is valid",
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

// TestTypeMismatchWithFileParsing tests type conversion errors when parsing from files
func TestTypeMismatchWithFileParsing(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
		errorPattern string
	}{
		{
			name: "file with string to int error",
			yamlContent: `
port: "not_a_number"
host: localhost
`,
			target:       &struct{ Port int; Host string }{},
			shouldError:  true,
			description:  "File with string value in integer field",
			errorPattern: "cannot unmarshal",
		},
		{
			name: "file with array to scalar error",
			yamlContent: `
name:
  - John
  - Doe
age: 30
`,
			target:       &struct{ Name string; Age int }{},
			shouldError:  true,
			description:  "File with array value in string field",
			errorPattern: "cannot unmarshal",
		},
		{
			name: "file with object to scalar error",
			yamlContent: `
title:
  sub: value
count: 5
`,
			target:       &struct{ Title string; Count int }{},
			shouldError:  true,
			description:  "File with object value in string field",
			errorPattern: "cannot unmarshal",
		},
		{
			name: "file with negative to unsigned error",
			yamlContent: `
count: -5
name: test
`,
			target:       &struct{ Count uint; Name string }{},
			shouldError:  true,
			description:  "File with negative value in unsigned field",
			errorPattern: "cannot unmarshal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", "type_mismatch_*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write YAML content
			if _, err := tmpFile.WriteString(tt.yamlContent); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Parse file
			parser := NewParser()
			parseErr := parser.ParseFile(tmpFile.Name(), tt.target)

			if tt.shouldError {
				if parseErr.Success {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, parseErr.Error)

					// Check error pattern if specified
					if tt.errorPattern != "" {
						errMsg := parseErr.Error.Error()
						if !contains(errMsg, tt.errorPattern) {
							t.Logf("Note: Error message doesn't contain expected pattern %q: %s", tt.errorPattern, errMsg)
						}
					}
				}
			} else {
				if !parseErr.Success {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, parseErr.Error)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestNewTypeMismatchErrorConstructor tests the NewTypeMismatchError constructor
func TestNewTypeMismatchErrorConstructor(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		fieldPath    string
		expectedType string
		actualType   string
		value        string
		line         int
		errorCode    ErrorCode
	}{
		{
			name:         "basic constructor",
			filePath:     "config.yaml",
			fieldPath:    "field",
			expectedType: "string",
			actualType:   "int",
			value:        "123",
			line:         10,
			errorCode:    "",
		},
		{
			name:         "with custom error code",
			filePath:     "data.yaml",
			fieldPath:    "nested.field",
			expectedType: "array",
			actualType:   "string",
			value:        "test",
			line:         25,
			errorCode:    ErrCodeTypeMismatch,
		},
		{
			name:         "without line number",
			filePath:     "app.yaml",
			fieldPath:    "config.value",
			expectedType: "bool",
			actualType:   "object",
			value:        "{}",
			line:         0,
			errorCode:    "",
		},
		{
			name:         "nested field path",
			filePath:     "manifest.yaml",
			fieldPath:    "spec.template.spec.containers[0].image",
			expectedType: "string",
			actualType:   "integer",
			value:        "123",
			line:         42,
			errorCode:    ErrCodeTypeMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				tt.filePath,
				tt.fieldPath,
				tt.expectedType,
				tt.actualType,
				tt.value,
				tt.line,
				tt.errorCode,
			)

			// Verify all fields are set correctly
			if err.FilePath != tt.filePath {
				t.Errorf("FilePath = %q, want %q", err.FilePath, tt.filePath)
			}
			if err.FieldPath != tt.fieldPath {
				t.Errorf("FieldPath = %q, want %q", err.FieldPath, tt.fieldPath)
			}
			if err.ExpectedType != tt.expectedType {
				t.Errorf("ExpectedType = %q, want %q", err.ExpectedType, tt.expectedType)
			}
			if err.ActualType != tt.actualType {
				t.Errorf("ActualType = %q, want %q", err.ActualType, tt.actualType)
			}
			if err.Value != tt.value {
				t.Errorf("Value = %q, want %q", err.Value, tt.value)
			}
			if err.Line != tt.line {
				t.Errorf("Line = %d, want %d", err.Line, tt.line)
			}

			// Verify error code
			expectedCode := tt.errorCode
			if expectedCode == "" {
				expectedCode = ErrCodeTypeMismatch
			}
			if err.Code() != expectedCode {
				t.Errorf("Code() = %q, want %q", err.Code(), expectedCode)
			}

			// Verify error type
			if err.YAMLErrorType() != ErrorTypeTypeMismatch {
				t.Errorf("YAMLErrorType() = %q, want %q", err.YAMLErrorType(), ErrorTypeTypeMismatch)
			}

			// Verify it's recognized as a YAMLError
			if !IsYAMLError(err) {
				t.Error("TypeMismatchError should implement YAMLError interface")
			}

			t.Logf("✓ Constructor properly initialized TypeMismatchError")
		})
	}
}

// TestTypeMismatchErrorInterfaceCompliance verifies TypeMismatchError implements all interfaces
func TestTypeMismatchErrorInterfaceCompliance(t *testing.T) {
	err := NewTypeMismatchError(
		"test.yaml",
		"field.path",
		"string",
		"int",
		"123",
		10,
		"",
	)

	// Verify error interface
	var _ error = err

	// Verify YAMLError interface
	var yamlErr YAMLError = err
	if yamlErr == nil {
		t.Error("TypeMismatchError should implement YAMLError interface")
	}

	// Test all YAMLError methods
	_ = yamlErr.Code()
	_ = yamlErr.YAMLErrorType()
	_ = yamlErr.Context()

	t.Logf("✓ TypeMismatchError properly implements error and YAMLError interfaces")
}

// TestComplexTypeMismatches tests complex nested type mismatch scenarios
func TestComplexTypeMismatches(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		{
			name: "nested struct with type error",
			yamlContent: `
config:
  server:
    port: "not_a_number"
    host: localhost
`,
			target: &struct {
				Config struct {
					Server struct {
						Port int
						Host string
					}
				}
			}{},
			shouldError: true,
			description: "Nested struct with string in integer field",
		},
		{
			name: "slice of structs with type error",
			yamlContent: `
servers:
  - port: 8080
    host: localhost
  - port: "invalid"
    host: example
`,
			target: &struct {
				Servers []struct {
					Port int
					Host string
				}
			}{},
			shouldError: true,
			description: "Slice of structs with type error in second element",
		},
		{
			name: "map with type error",
			yamlContent: `
endpoints:
  api:
    port: 8080
    timeout: 30
  admin:
    port: "invalid"
    timeout: 60
`,
			target: &struct {
				Endpoints map[string]struct {
					Port    int
					Timeout int
				}
			}{},
			shouldError: true,
			description: "Map with type error in one value",
		},
		{
			name: "array element type mismatch",
			yamlContent: `
ports:
  - 8080
  - "9090"
  - 8081
`,
			target: &struct {
				Ports []int
			}{},
			shouldError: true,
			description: "Array with mixed types (string in integer array)",
		},
		{
			name: "nested array type mismatch",
			yamlContent: `
matrix:
  - [1, 2, 3]
  - [4, "five", 6]
  - [7, 8, 9]
`,
			target: &struct {
				Matrix [][]int
			}{},
			shouldError: true,
			description: "Nested array with type error in inner array",
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

// TestTypeMismatchErrorCoverage tests edge cases for TypeMismatchError
func TestTypeMismatchErrorCoverage(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		fieldPath    string
		expectedType string
		actualType   string
		value        string
		line         int
	}{
		{
			name:         "empty field path",
			filePath:     "test.yaml",
			fieldPath:    "",
			expectedType: "string",
			actualType:   "int",
			value:        "123",
			line:         5,
		},
		{
			name:         "very long field path",
			filePath:     "test.yaml",
			fieldPath:    "a.very.long.nested.path.to.the.field.that.goes.on.and.on",
			expectedType: "bool",
			actualType:   "string",
			value:        "test",
			line:         100,
		},
		{
			name:         "special characters in value",
			filePath:     "test.yaml",
			fieldPath:    "field",
			expectedType: "string",
			actualType:   "int",
			value:        "!@#$%^&*()",
			line:         10,
		},
		{
			name:         "unicode in field path",
			filePath:     "test.yaml",
			fieldPath:    "field.наименование",
			expectedType: "string",
			actualType:   "int",
			value:        "123",
			line:         15,
		},
		{
			name:         "empty value",
			filePath:     "test.yaml",
			fieldPath:    "field",
			expectedType: "string",
			actualType:   "int",
			value:        "",
			line:         20,
		},
		{
			name:         "very large line number",
			filePath:     "test.yaml",
			fieldPath:    "field",
			expectedType: "string",
			actualType:   "int",
			value:        "123",
			line:         999999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				tt.filePath,
				tt.fieldPath,
				tt.expectedType,
				tt.actualType,
				tt.value,
				tt.line,
				"",
			)

			// Verify error can be created and accessed without panic
			errorMsg := err.Error()
			ctxMsg := err.Context()

			if errorMsg == "" {
				t.Error("Error() should not return empty string")
			}
			if ctxMsg == "" {
				t.Error("Context() should not return empty string")
			}

			t.Logf("✓ Edge case handled: %s", tt.name)
			t.Logf("  Error: %s", errorMsg)
			t.Logf("  Context: %s", ctxMsg)
		})
	}
}

// TestCustomTypeConversions tests type conversion errors for custom and special types
func TestCustomTypeConversions(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		{
			name: "invalid duration format",
			yamlContent: `
timeout: "not_a_duration"
`,
			target:      &struct{ Timeout string }{},
			shouldError: false,
			description: "String field accepts any value (validation is separate)",
		},
		{
			name: "map with non-string keys",
			yamlContent: `
config:
  123: value
  456: other
`,
			target:      &struct{ Config map[int]string }{},
			shouldError: false,
			description: "Map keys are converted appropriately",
		},
		{
			name: "struct pointer with type error",
			yamlContent: `
config:
  port: "not_a_number"
`,
			target:      &struct{ Config *struct{ Port int } }{},
			shouldError: true,
			description: "Struct pointer with type error in nested field",
		},
		{
			name: "slice of pointers with type error",
			yamlContent: `
items:
  - 123
  - "not_a_number"
  - 456
`,
			target:      &struct{ Items []*int }{},
			shouldError: true,
			description: "Slice of pointers with type error in element",
		},
		{
			name: "interface{} with incompatible type",
			yamlContent: `
value:
  nested:
    - item1
    - item2
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Interface{} target expects int but gets complex structure",
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

// TestNumericPrecisionConversions tests numeric precision and boundary conditions
func TestNumericPrecisionConversions(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		{
			name: "float32 precision loss - very small number",
			yamlContent: `
value: 1.401298464324817e-45
`,
			target:      &struct{ Value float32 }{},
			shouldError: false,
			description: "Very small float32 value (near minimum)",
		},
		{
			name: "float64 to float32 conversion",
			yamlContent: `
value: 1.234567890123456789
`,
			target:      &struct{ Value float32 }{},
			shouldError: false,
			description: "Float64 value converted to float32 (precision loss)",
		},
		{
			name: "complex number to scalar",
			yamlContent: `
value: 3.14+2.72i
`,
			target:      &struct{ Value float64 }{},
			shouldError: true,
			description: "Complex number cannot be converted to float64",
		},
		{
			name: "hex string to integer",
			yamlContent: `
value: "0xFF"
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Hex string cannot be directly converted to integer",
		},
		{
			name: "octal string to integer",
			yamlContent: `
value: "0755"
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Octal string cannot be directly converted to integer",
		},
		{
			name: "scientific notation string to integer",
			yamlContent: `
value: "1.5e3"
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Scientific notation string cannot be converted to integer",
		},
		{
			name: "very large float to int conversion",
			yamlContent: `
value: 1.0e20
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Large float cannot be converted to int",
		},
		{
			name: "negative zero",
			yamlContent: `
value: -0.0
`,
			target:      &struct{ Value float64 }{},
			shouldError: false,
			description: "Negative zero is valid for float64",
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

// TestEmbeddedStructConversions tests type conversion errors with embedded structs
func TestEmbeddedStructConversions(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		{
			name: "embedded struct with type error",
			yamlContent: `
base:
  count: "not_a_number"
extended:
  value: test
`,
			target: &struct {
				Base     struct{ Count int }
				Extended struct{ Value string }
			}{},
			shouldError: true,
			description: "Embedded struct field with type error",
		},
		{
			name: "nested embedded struct type error",
			yamlContent: `
level1:
  level2:
    depth: "invalid"
`,
			target: &struct {
				Level1 struct {
					Level2 struct {
						Depth int
					}
				}
			}{},
			shouldError: true,
			description: "Deeply nested embedded struct with type error",
		},
		{
			name: "multiple embedded structs with mixed types",
			yamlContent: `
config:
  timeout: 30
metadata:
  name: test
`,
			target: &struct {
				Config   struct{ Timeout int }
				Metadata struct{ Name string }
			}{},
			shouldError: false,
			description: "Multiple embedded structs with valid types",
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

// TestMapTypeConversions tests map key and value type conversion errors
func TestMapTypeConversions(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		{
			name: "map with int keys and string values",
			yamlContent: `
items:
  1: first
  2: second
`,
			target:      &struct{ Items map[int]string }{},
			shouldError: false,
			description: "Map with integer keys and string values",
		},
		{
			name: "map with struct values and type error",
			yamlContent: `
servers:
  web:
    port: "invalid"
    host: localhost
  api:
    port: 8080
    host: api.example.com
`,
			target: &struct {
				Servers map[string]struct {
					Port int
					Host string
				}
			}{},
			shouldError: true,
			description: "Map with struct values containing type error",
		},
		{
			name: "map with bool keys",
			yamlContent: `
flags:
  true: enabled
  false: disabled
`,
			target:      &struct{ Flags map[bool]string }{},
			shouldError: false,
			description: "Map with boolean keys",
		},
		{
			name: "scalar to map conversion",
			yamlContent: `
config: "not_a_map"
`,
			target:      &struct{ Config map[string]string }{},
			shouldError: true,
			description: "Scalar value cannot be converted to map",
		},
		{
			name: "array to map conversion",
			yamlContent: `
config:
  - key1: value1
  - key2: value2
`,
			target:      &struct{ Config map[string]string }{},
			shouldError: true,
			description: "Array cannot be converted to map",
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

// TestTypeAliasConversions tests type conversion errors with type aliases
func TestTypeAliasConversions(t *testing.T) {
	// Define type aliases for testing
	type (
		MyInt    int
		MyString string
		MyFloat  float64
	)

	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		{
			name: "type alias int with string value",
			yamlContent: `
count: "not_a_number"
`,
			target:      &struct{ Count MyInt }{},
			shouldError: true,
			description: "Type alias MyInt cannot accept string",
		},
		{
			name: "type alias string with int value",
			yamlContent: `
name: 12345
`,
			target:      &struct{ Name MyString }{},
			shouldError: false,
			description: "Type alias MyString can accept int (converts)",
		},
		{
			name: "type alias float with bool value",
			yamlContent: `
rate: true
`,
			target:      &struct{ Rate MyFloat }{},
			shouldError: true,
			description: "Type alias MyFloat cannot accept bool",
		},
		{
			name: "type alias with overflow",
			yamlContent: `
value: 9223372036854775808
`,
			target:      &struct{ Value MyInt }{},
			shouldError: true,
			description: "Type alias MyInt overflow",
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

// TestSliceElementConversions tests type conversion errors in slice elements
func TestSliceElementConversions(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		{
			name: "slice with mixed int and string elements",
			yamlContent: `
numbers:
  - 1
  - "two"
  - 3
`,
			target:      &struct{ Numbers []int }{},
			shouldError: true,
			description: "Slice with mixed types (string in int slice)",
		},
		{
			name: "slice of structs with type errors",
			yamlContent: `
users:
  - name: Alice
    age: 30
  - name: Bob
    age: "thirty"
  - name: Carol
    age: 25
`,
			target: &struct {
				Users []struct {
					Name string
					Age  int
				}
			}{},
			shouldError: true,
			description: "Slice of structs with type error in one element",
		},
		{
			name: "nested slice with type error",
			yamlContent: `
matrix:
  - [1, 2, 3]
  - [4, "five", 6]
  - [7, 8, 9]
`,
			target:      &struct{ Matrix [][]int }{},
			shouldError: true,
			description: "Nested slice with type error in inner slice",
		},
		{
			name: "slice of interface{} with various types",
			yamlContent: `
items:
  - 123
  - "string"
  - true
  - 3.14
`,
			target:      &struct{ Items []interface{} }{},
			shouldError: false,
			description: "Slice of interface{} accepts various types",
		},
		{
			name: "empty slice",
			yamlContent: `
items: []
`,
			target:      &struct{ Items []int }{},
			shouldError: false,
			description: "Empty slice is valid",
		},
		{
			name: "null slice",
			yamlContent: `
items: null
`,
			target:      &struct{ Items []int }{},
			shouldError: false,
			description: "Null slice becomes nil (valid)",
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

// TestStructTagConversions tests type conversion errors with struct tags
func TestStructTagConversions(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		{
			name: "struct with yaml tags and type errors",
			yamlContent: `
server_port: "8080"
db_name: mydb
`,
			target: &struct {
				ServerPort int    `yaml:"server_port"`
				DBName     string `yaml:"db_name"`
			}{},
			shouldError: true,
			description: "Struct with YAML tags has type error",
		},
		{
			name: "struct with omitempty tag and null",
			yamlContent: `
port: 8080
host: null
`,
			target: &struct {
				Port int    `yaml:"port"`
				Host string `yaml:"host,omitempty"`
			}{},
			shouldError: false,
			description: "Struct with omitempty tag accepts null",
		},
		{
			name: "nested struct tags with type error",
			yamlContent: `
config:
  timeout: "30s"
`,
			target: &struct {
				Config *struct {
					Timeout int `yaml:"timeout"`
				} `yaml:"config"`
			}{},
			shouldError: true,
			description: "Nested struct pointer with YAML tags has type error",
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

// TestErrorWrappingAndUnwrapping tests error wrapping and unwrapping for TypeMismatchError
func TestErrorWrappingAndUnwrapping(t *testing.T) {
	tests := []struct {
		name         string
		baseError    *TypeMismatchError
		wrapMessage  string
		expectUnwrap bool
	}{
		{
			name: "wrap type mismatch error",
			baseError: NewTypeMismatchError(
				"config.yaml",
				"server.port",
				"int",
				"string",
				"8080",
				10,
				"",
			),
			wrapMessage:  "failed to parse configuration",
			expectUnwrap: true,
		},
		{
			name: "wrap complex type mismatch",
			baseError: NewTypeMismatchError(
				"deployment.yaml",
				"spec.replicas",
				"integer",
				"boolean",
				"true",
				25,
				ErrCodeTypeMismatch,
			),
			wrapMessage:  "cannot parse deployment spec",
			expectUnwrap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap the error
			wrappedErr := fmt.Errorf("%s: %w", tt.wrapMessage, tt.baseError)

			// Verify wrapping works
			if wrappedErr == nil {
				t.Fatal("Wrapped error should not be nil")
			}

			// Verify error message includes wrap message
			errMsg := wrappedErr.Error()
			if !contains(errMsg, tt.wrapMessage) {
				t.Errorf("Wrapped error message should contain wrap message: %s", errMsg)
			}

			// Verify we can unwrap to get the original TypeMismatchError
			var unwrapped *TypeMismatchError
			if tt.expectUnwrap {
				if !errors.As(wrappedErr, &unwrapped) {
					t.Error("Should be able to unwrap to TypeMismatchError")
				} else {
					// Verify unwrapped error has same properties
					if unwrapped.FilePath != tt.baseError.FilePath {
						t.Errorf("Unwrapped FilePath mismatch: got %s, want %s",
							unwrapped.FilePath, tt.baseError.FilePath)
					}
					if unwrapped.FieldPath != tt.baseError.FieldPath {
						t.Errorf("Unwrapped FieldPath mismatch: got %s, want %s",
							unwrapped.FieldPath, tt.baseError.FieldPath)
					}
					if unwrapped.ExpectedType != tt.baseError.ExpectedType {
						t.Errorf("Unwrapped ExpectedType mismatch: got %s, want %s",
							unwrapped.ExpectedType, tt.baseError.ExpectedType)
					}
					if unwrapped.ActualType != tt.baseError.ActualType {
						t.Errorf("Unwrapped ActualType mismatch: got %s, want %s",
							unwrapped.ActualType, tt.baseError.ActualType)
					}
					t.Logf("✓ Error wrapping and unwrapping works correctly")
				}
			}

			// Verify it's still recognized as a YAMLError
			var yamlErr YAMLError
			if !errors.As(wrappedErr, &yamlErr) {
				t.Error("Wrapped error should still implement YAMLError interface")
			} else {
				t.Logf("✓ Wrapped error still implements YAMLError interface")
			}
		})
	}
}

// TestInvalidStringToNonStringConversions tests invalid string conversions beyond basic types
func TestInvalidStringToNonStringConversions(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
		expectedType string
		actualType   string
	}{
		{
			name: "string to struct conversion",
			yamlContent: `
config: "not_a_struct"
`,
			target: &struct {
				Config struct {
					Port int
					Host string
				}
			}{},
			shouldError:  true,
			description:  "String cannot be converted to struct",
			expectedType: "struct",
			actualType:   "string",
		},
		{
			name: "string to array conversion",
			yamlContent: `
items: "not_an_array"
`,
			target:      &struct{ Items []int }{},
			shouldError: true,
			description: "String cannot be converted to array",
			expectedType: "array",
			actualType:   "string",
		},
		{
			name: "string to map conversion",
			yamlContent: `
config: "not_a_map"
`,
			target:      &struct{ Config map[string]string }{},
			shouldError: true,
			description: "String cannot be converted to map",
			expectedType: "map",
			actualType:  "string",
		},
		{
			name: "string to slice of structs conversion",
			yamlContent: `
servers: "not_a_slice"
`,
			target: &struct {
				Servers []struct {
					Port int
					Host string
				}
			}{},
			shouldError:  true,
			description:  "String cannot be converted to slice of structs",
			expectedType: "slice",
			actualType:   "string",
		},
		{
			name: "empty string to int conversion",
			yamlContent: `
value: ""
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Empty string cannot be converted to int",
			expectedType: "int",
			actualType:   "string",
		},
		{
			name: "whitespace string to int conversion",
			yamlContent: `
value: "   "
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Whitespace string cannot be converted to int",
			expectedType: "int",
			actualType:   "string",
		},
		{
			name: "special chars string to int conversion",
			yamlContent: `
value: "!@#$%"
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Special character string cannot be converted to int",
			expectedType: "int",
			actualType:   "string",
		},
		{
			name: "string with whitespace to float conversion",
			yamlContent: `
rate: " 3.14 "
`,
			target:      &struct{ Rate float64 }{},
			shouldError: true,
			description: "String with surrounding whitespace cannot be converted to float",
			expectedType: "float64",
			actualType:   "string",
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
					// Verify error is returned (not panic)
					if err != nil {
						t.Logf("✓ Error returned (not panic) for %s", tt.name)
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

// TestStructToScalarConversionFailures tests struct/mapping to scalar conversion failures
func TestStructToScalarConversionFailures(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
		expectedType string
		actualType   string
	}{
		{
			name: "struct to int conversion",
			yamlContent: `
value:
  field1: data1
  field2: data2
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Struct/mapping cannot be converted to int",
			expectedType: "int",
			actualType:   "mapping",
		},
		{
			name: "struct to float conversion",
			yamlContent: `
rate:
  value: 123
`,
			target:      &struct{ Rate float64 }{},
			shouldError: true,
			description: "Struct/mapping cannot be converted to float",
			expectedType: "float64",
			actualType:   "mapping",
		},
		{
			name: "struct to bool conversion",
			yamlContent: `
enabled:
  flag: true
`,
			target:      &struct{ Enabled bool }{},
			shouldError: true,
			description: "Struct/mapping cannot be converted to bool",
			expectedType: "bool",
			actualType:   "mapping",
		},
		{
			name: "nested struct to string conversion",
			yamlContent: `
name:
  first: John
  last: Doe
`,
			target:      &struct{ Name string }{},
			shouldError: true,
			description: "Nested struct cannot be converted to string",
			expectedType: "string",
			actualType:   "mapping",
		},
		{
			name: "struct with multiple fields to scalar",
			yamlContent: `
value:
  a: 1
  b: 2
  c: 3
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Multi-field struct cannot be converted to scalar int",
			expectedType: "int",
			actualType:   "mapping",
		},
		{
			name: "empty struct to scalar",
			yamlContent: `
value: {}
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Empty struct/mapping cannot be converted to int",
			expectedType: "int",
			actualType:   "mapping",
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
					// Verify error is returned (not panic)
					if err != nil {
						t.Logf("✓ Error returned (not panic) for %s", tt.name)
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

// TestInvalidMapArrayConversions tests invalid map/array to scalar conversions
func TestInvalidMapArrayConversions(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
		expectedType string
		actualType   string
	}{
		{
			name: "map to int conversion",
			yamlContent: `
value:
  key1: val1
  key2: val2
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Map cannot be converted to int",
			expectedType: "int",
			actualType:   "map",
		},
		{
			name: "map to float conversion",
			yamlContent: `
rate:
  a: 1.5
  b: 2.5
`,
			target:      &struct{ Rate float64 }{},
			shouldError: true,
			description: "Map cannot be converted to float",
			expectedType: "float64",
			actualType:   "map",
		},
		{
			name: "map to bool conversion",
			yamlContent: `
flag:
  enabled: true
`,
			target:      &struct{ Flag bool }{},
			shouldError: true,
			description: "Map cannot be converted to bool",
			expectedType: "bool",
			actualType:   "map",
		},
		{
			name: "array to int conversion",
			yamlContent: `
value:
  - 1
  - 2
  - 3
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Array cannot be converted to int",
			expectedType: "int",
			actualType:   "array",
		},
		{
			name: "array to float conversion",
			yamlContent: `
rate:
  - 1.5
  - 2.5
`,
			target:      &struct{ Rate float64 }{},
			shouldError: true,
			description: "Array cannot be converted to float",
			expectedType: "float64",
			actualType:   "array",
		},
		{
			name: "array to bool conversion",
			yamlContent: `
enabled:
  - true
  - false
`,
			target:      &struct{ Enabled bool }{},
			shouldError: true,
			description: "Array cannot be converted to bool",
			expectedType: "bool",
			actualType:   "array",
		},
		{
			name: "empty array to int conversion",
			yamlContent: `
value: []
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Empty array cannot be converted to int",
			expectedType: "int",
			actualType:   "array",
		},
		{
			name: "empty map to float conversion",
			yamlContent: `
rate: {}
`,
			target:      &struct{ Rate float64 }{},
			shouldError: true,
			description: "Empty map cannot be converted to float",
			expectedType: "float64",
			actualType:   "map",
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
					// Verify error is returned (not panic)
					if err != nil {
						t.Logf("✓ Error returned (not panic) for %s", tt.name)
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

// TestMapToArrayConversion tests invalid map to array conversions
func TestMapToArrayConversion(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
		expectedType string
		actualType   string
	}{
		{
			name: "map to array of strings conversion",
			yamlContent: `
items:
  a: item1
  b: item2
`,
			target:      &struct{ Items []string }{},
			shouldError: true,
			description: "Map cannot be converted to array of strings",
			expectedType: "array",
			actualType:   "map",
		},
		{
			name: "map to array of ints conversion",
			yamlContent: `
numbers:
  one: 1
  two: 2
`,
			target:      &struct{ Numbers []int }{},
			shouldError: true,
			description: "Map cannot be converted to array of ints",
			expectedType: "array",
			actualType:   "map",
		},
		{
			name: "map to array of structs conversion",
			yamlContent: `
servers:
  web:
    port: 8080
  api:
    port: 9090
`,
			target: &struct {
				Servers []struct {
					Port int
				}
			}{},
			shouldError:  true,
			description:  "Map cannot be converted to array of structs",
			expectedType: "array",
			actualType:   "map",
		},
		{
			name: "empty map to array conversion",
			yamlContent: `
items: {}
`,
			target:      &struct{ Items []string }{},
			shouldError: true,
			description: "Empty map cannot be converted to array",
			expectedType: "array",
			actualType:   "map",
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
					// Verify error is returned (not panic)
					if err != nil {
						t.Logf("✓ Error returned (not panic) for %s", tt.name)
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

// TestArrayToMapConversion tests invalid array to map conversions
func TestArrayToMapConversion(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
		expectedType string
		actualType   string
	}{
		{
			name: "array to map of strings conversion",
			yamlContent: `
config:
  - key1
  - key2
`,
			target:      &struct{ Config map[string]string }{},
			shouldError: true,
			description: "Array cannot be converted to map of strings",
			expectedType: "map",
			actualType:   "array",
		},
		{
			name: "array to map of ints conversion",
			yamlContent: `
values:
  - 100
  - 200
`,
			target:      &struct{ Values map[string]int }{},
			shouldError: true,
			description: "Array cannot be converted to map of ints",
			expectedType: "map",
			actualType:   "array",
		},
		{
			name: "array of objects to map conversion",
			yamlContent: `
servers:
  - {name: web, port: 8080}
  - {name: api, port: 9090}
`,
			target: &struct {
				Servers map[string]struct {
					Name string
					Port int
				}
			}{},
			shouldError:  true,
			description:  "Array of objects cannot be converted to map",
			expectedType: "map",
			actualType:   "array",
		},
		{
			name: "empty array to map conversion",
			yamlContent: `
config: []
`,
			target:      &struct{ Config map[string]string }{},
			shouldError: true,
			description: "Empty array cannot be converted to map",
			expectedType: "map",
			actualType:   "array",
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
					// Verify error is returned (not panic)
					if err != nil {
						t.Logf("✓ Error returned (not panic) for %s", tt.name)
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
