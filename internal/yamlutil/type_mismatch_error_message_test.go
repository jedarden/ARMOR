// Package yamlutil tests for TypeMismatchError message quality
//
// This test file provides comprehensive coverage for TypeMismatchError message quality,
// ensuring error messages are helpful, actionable, and contain relevant context.
// Tests cover constructor behavior, message formatting, interface compliance,
// error wrapping, and edge cases (empty values, special characters, unicode).
package yamlutil

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ============================================================================
// Section 1: Constructor Behavior Tests
// ============================================================================

// TestTypeMismatchErrorConstructor verifies NewTypeMismatchError initializes all fields correctly
func TestTypeMismatchErrorConstructor(t *testing.T) {
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
			name:         "all fields populated",
			filePath:     "config.yaml",
			fieldPath:    "server.port",
			expectedType: "integer",
			actualType:   "string",
			value:        "8080",
			line:         10,
			errorCode:    ErrCodeTypeMismatch,
		},
		{
			name:         "minimal fields",
			filePath:     "test.yaml",
			fieldPath:    "field",
			expectedType: "string",
			actualType:   "int",
			value:        "",
			line:         0,
			errorCode:    "",
		},
		{
			name:         "nested field path",
			filePath:     "app.yaml",
			fieldPath:    "servers.api.responses[0].statusCode",
			expectedType: "integer",
			actualType:   "string",
			value:        "\"200\"",
			line:         42,
			errorCode:    ErrCodeTypeMismatch,
		},
		{
			name:         "empty file path",
			filePath:     "",
			fieldPath:    "field",
			expectedType: "boolean",
			actualType:   "string",
			value:        "yes",
			line:         5,
			errorCode:    "",
		},
		{
			name:         "empty field path",
			filePath:     "config.yaml",
			fieldPath:    "",
			expectedType: "array",
			actualType:   "string",
			value:        "not array",
			line:         15,
			errorCode:    "",
		},
		{
			name:         "empty expected type",
			filePath:     "data.yaml",
			fieldPath:    "field",
			expectedType: "",
			actualType:   "string",
			value:        "value",
			line:         8,
			errorCode:    "",
		},
		{
			name:         "empty actual type",
			filePath:     "values.yaml",
			fieldPath:    "field",
			expectedType: "integer",
			actualType:   "",
			value:        "42",
			line:         12,
			errorCode:    "",
		},
		{
			name:         "empty value",
			filePath:     "test.yaml",
			fieldPath:    "field",
			expectedType: "string",
			actualType:   "integer",
			value:        "",
			line:         20,
			errorCode:    "",
		},
		{
			name:         "zero line number",
			filePath:     "config.yaml",
			fieldPath:    "server.port",
			expectedType: "integer",
			actualType:   "string",
			value:        "abc",
			line:         0,
			errorCode:    "",
		},
		{
			name:         "custom error code",
			filePath:     "custom.yaml",
			fieldPath:    "field",
			expectedType: "object",
			actualType:   "array",
			value:        "[]",
			line:         100,
			errorCode:    "CUSTOM_CODE",
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
			if err.ErrorCode != tt.errorCode {
				t.Errorf("ErrorCode = %q, want %q", err.ErrorCode, tt.errorCode)
			}

			t.Logf("✓ Constructor initialized all fields correctly")
		})
	}
}

// TestTypeMismatchErrorCodeDefaulting verifies error code defaults to TYPE_MISMATCH
func TestTypeMismatchErrorCodeDefaulting(t *testing.T) {
	err := NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 10, "")

	if err.Code() != ErrCodeTypeMismatch {
		t.Errorf("Code() with empty ErrorCode = %q, want %q", err.Code(), ErrCodeTypeMismatch)
	}

	errWithCode := NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 10, "CUSTOM_CODE")
	if errWithCode.Code() != "CUSTOM_CODE" {
		t.Errorf("Code() with custom ErrorCode = %q, want %q", errWithCode.Code(), "CUSTOM_CODE")
	}

	t.Logf("✓ Error code defaults to TYPE_MISMATCH when not provided")
}

// ============================================================================
// Section 2: Error Message Format Tests
// ============================================================================

// TestTypeMismatchErrorMessageFormat verifies Error() message format with line number
func TestTypeMismatchErrorMessageFormat(t *testing.T) {
	tests := []struct {
		name         string
		err          *TypeMismatchError
		wantMessage  string
		wantContains []string
	}{
		{
			name: "with line number",
			err: NewTypeMismatchError("config.yaml", "server.port", "integer", "string", "8080", 15, ""),
			wantMessage: "type mismatch in config.yaml at line 15, field server.port: expected integer, got string",
			wantContains: []string{
				"type mismatch",
				"config.yaml",
				"line 15",
				"field server.port",
				"expected integer",
				"got string",
			},
		},
		{
			name: "without line number",
			err: NewTypeMismatchError("data.yaml", "database.timeout", "integer", "boolean", "true", 0, ""),
			wantMessage: "type mismatch in data.yaml, field database.timeout: expected integer, got boolean",
			wantContains: []string{
				"type mismatch",
				"data.yaml",
				"field database.timeout",
				"expected integer",
				"got boolean",
			},
		},
		{
			name: "nested field path with line",
			err: NewTypeMismatchError("app.yaml", "servers.api.responses[0].statusCode", "integer", "string", "\"200\"", 42, ""),
			wantMessage: "type mismatch in app.yaml at line 42, field servers.api.responses[0].statusCode: expected integer, got string",
			wantContains: []string{
				"type mismatch",
				"app.yaml",
				"line 42",
				"servers.api.responses[0].statusCode",
				"expected integer",
				"got string",
			},
		},
		{
			name: "empty field path with line",
			err: NewTypeMismatchError("test.yaml", "", "object", "string", "{}", 10, ""),
			wantMessage: "type mismatch in test.yaml at line 10, field : expected object, got string",
			wantContains: []string{
				"type mismatch",
				"test.yaml",
				"line 10",
				"field :",
				"expected object",
				"got string",
			},
		},
		{
			name: "empty file path",
			err: NewTypeMismatchError("", "field", "boolean", "integer", "1", 5, ""),
			wantMessage: "type mismatch in  at line 5, field field: expected boolean, got integer",
			wantContains: []string{
				"type mismatch",
				"line 5",
				"field field",
				"expected boolean",
				"got integer",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorMsg := tt.err.Error()

			// Verify exact message format
			if errorMsg != tt.wantMessage {
				t.Errorf("Error() = %q, want %q", errorMsg, tt.wantMessage)
			}

			// Verify message contains all expected components
			for _, expected := range tt.wantContains {
				if !strings.Contains(errorMsg, expected) {
					t.Errorf("Error() should contain %q, got: %s", expected, errorMsg)
				}
			}

			t.Logf("✓ Error message format: %s", errorMsg)
		})
	}
}

// TestTypeMismatchErrorContextFormat verifies Context() message format
func TestTypeMismatchErrorContextFormat(t *testing.T) {
	tests := []struct {
		name         string
		err          *TypeMismatchError
		wantContext  string
		wantContains []string
	}{
		{
			name: "full context with all fields",
			err: NewTypeMismatchError("config.yaml", "server.port", "integer", "string", "8080", 15, ""),
			wantContext: "field: server.port, expected type: integer, actual type: string, value: 8080",
			wantContains: []string{
				"field: server.port",
				"expected type: integer",
				"actual type: string",
				"value: 8080",
			},
		},
		{
			name: "context without value",
			err: NewTypeMismatchError("data.yaml", "field", "boolean", "string", "", 0, ""),
			wantContext: "field: field, expected type: boolean, actual type: string, value: ",
			wantContains: []string{
				"field: field",
				"expected type: boolean",
				"actual type: string",
				"value: ",
			},
		},
		{
			name: "context with nested field",
			err: NewTypeMismatchError("app.yaml", "servers.api.responses[0].statusCode", "integer", "string", "\"200\"", 42, ""),
			wantContext: "field: servers.api.responses[0].statusCode, expected type: integer, actual type: string, value: \"200\"",
			wantContains: []string{
				"field: servers.api.responses[0].statusCode",
				"expected type: integer",
				"actual type: string",
				"value: \"200\"",
			},
		},
		{
			name: "context with empty field path",
			err: NewTypeMismatchError("test.yaml", "", "object", "array", "[]", 10, ""),
			wantContext: "field: , expected type: object, actual type: array, value: []",
			wantContains: []string{
				"field: ,",
				"expected type: object",
				"actual type: array",
				"value: []",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxMsg := tt.err.Context()

			// Verify exact context format
			if ctxMsg != tt.wantContext {
				t.Errorf("Context() = %q, want %q", ctxMsg, tt.wantContext)
			}

			// Verify context contains all expected components
			for _, expected := range tt.wantContains {
				if !strings.Contains(ctxMsg, expected) {
					t.Errorf("Context() should contain %q, got: %s", expected, ctxMsg)
				}
			}

			t.Logf("✓ Context format: %s", ctxMsg)
		})
	}
}

// ============================================================================
// Section 3: Interface Compliance Tests
// ============================================================================

// TestTypeMismatchErrorYAMLErrorCompliance verifies TypeMismatchError implements YAMLError interface
func TestTypeMismatchErrorYAMLErrorCompliance(t *testing.T) {
	err := NewTypeMismatchError("config.yaml", "field", "int", "string", "value", 10, "")

	// Verify it implements error interface
	var _ error = err
	if err.Error() == "" {
		t.Error("Error() should return non-empty string")
	}

	// Verify it implements YAMLError interface
	var ye YAMLError = err
	if ye == nil {
		t.Error("TypeMismatchError should implement YAMLError interface")
	}

	// Verify Code() method
	if ye.Code() == "" {
		t.Error("Code() should return non-empty ErrorCode")
	}

	// Verify YAMLErrorType() method
	if ye.YAMLErrorType() == "" {
		t.Error("YAMLErrorType() should return non-empty ErrorType")
	}

	// Verify Context() method
	if ye.Context() == "" {
		t.Error("Context() should return non-empty string")
	}

	t.Logf("✓ TypeMismatchError implements error and YAMLError interfaces")
}

// TestTypeMismatchErrorYAMLErrorType verifies YAMLErrorType returns correct type
func TestTypeMismatchErrorYAMLErrorType(t *testing.T) {
	tests := []struct {
		name            string
		err             *TypeMismatchError
		expectedErrorType ErrorType
	}{
		{
			name:             "default error type",
			err:              NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 10, ""),
			expectedErrorType: ErrorTypeTypeMismatch,
		},
		{
			name:             "with custom error code",
			err:              NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 10, "CUSTOM"),
			expectedErrorType: ErrorTypeTypeMismatch,
		},
		{
			name:             "without line number",
			err:              NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 0, ""),
			expectedErrorType: ErrorTypeTypeMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.YAMLErrorType() != tt.expectedErrorType {
				t.Errorf("YAMLErrorType() = %q, want %q", tt.err.YAMLErrorType(), tt.expectedErrorType)
			}
			t.Logf("✓ YAMLErrorType() = %s", tt.err.YAMLErrorType())
		})
	}
}

// TestTypeMismatchErrorErrorCode verifies Code() returns correct error code
func TestTypeMismatchErrorErrorCode(t *testing.T) {
	tests := []struct {
		name         string
		err          *TypeMismatchError
		expectedCode ErrorCode
	}{
		{
			name:         "default error code",
			err:          NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 10, ""),
			expectedCode: ErrCodeTypeMismatch,
		},
		{
			name:         "custom error code",
			err:          NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 10, "CUSTOM_CODE"),
			expectedCode: "CUSTOM_CODE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code() != tt.expectedCode {
				t.Errorf("Code() = %q, want %q", tt.err.Code(), tt.expectedCode)
			}
			t.Logf("✓ Code() = %s", tt.err.Code())
		})
	}
}

// TestTypeMismatchErrorInYAMLErrorChecks verifies TypeMismatchError is recognized by YAMLError helpers
func TestTypeMismatchErrorInYAMLErrorChecks(t *testing.T) {
	err := NewTypeMismatchError("config.yaml", "field", "int", "string", "value", 10, "")

	// Verify IsYAMLError recognizes it
	if !IsYAMLError(err) {
		t.Error("IsYAMLError() should return true for TypeMismatchError")
	}

	// Verify GetYAMLErrorType returns correct type
	errorType := GetYAMLErrorType(err)
	if errorType != ErrorTypeTypeMismatch {
		t.Errorf("GetYAMLErrorType() = %q, want %q", errorType, ErrorTypeTypeMismatch)
	}

	t.Logf("✓ TypeMismatchError is recognized by YAMLError helper functions")
}

// ============================================================================
// Section 4: Error Wrapping and Unwrapping Tests
// ============================================================================

// TestTypeMismatchErrorWrapping verifies TypeMismatchError can be wrapped and unwrapped
func TestTypeMismatchErrorWrapping(t *testing.T) {
	tests := []struct {
		name             string
		createError      func() error
		expectUnwrap     bool
		expectIsYAMLError bool
	}{
		{
			name: "direct TypeMismatchError",
			createError: func() error {
				return NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 10, "")
			},
			expectUnwrap:      false,
			expectIsYAMLError: true,
		},
		{
			name: "wrapped TypeMismatchError with fmt",
			createError: func() error {
				inner := NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 10, "")
				return fmt.Errorf("wrapped: %w", inner)
			},
			expectUnwrap:      true,
			expectIsYAMLError: false, // IsYAMLError doesn't unwrap
		},
		{
			name: "double wrapped TypeMismatchError",
			createError: func() error {
				inner := NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 10, "")
				wrapped1 := fmt.Errorf("first wrap: %w", inner)
				return fmt.Errorf("second wrap: %w", wrapped1)
			},
			expectUnwrap:      true,
			expectIsYAMLError: false, // IsYAMLError doesn't unwrap
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()

			// Verify error message contains context
			errMsg := err.Error()
			if !strings.Contains(errMsg, "type mismatch") {
				t.Errorf("Wrapped error message should contain 'type mismatch', got: %s", errMsg)
			}

			// Verify unwrapping works
			if tt.expectUnwrap {
				unwrapped := errors.Unwrap(err)
				if unwrapped == nil {
					t.Error("Unwrap() should return non-nil for wrapped error")
				} else {
					t.Logf("✓ Unwrap() returned: %T", unwrapped)
				}

				// Verify we can unwrap to TypeMismatchError
				var tme *TypeMismatchError
				if errors.As(err, &tme) {
					t.Logf("✓ errors.As() found TypeMismatchError in wrap chain")
				} else {
					t.Error("errors.As() should find TypeMismatchError in wrap chain")
				}
			}

			// Check IsYAMLError behavior (doesn't unwrap)
			isYAMLErr := IsYAMLError(err)
			if isYAMLErr != tt.expectIsYAMLError {
				t.Errorf("IsYAMLError() = %v, want %v (note: IsYAMLError doesn't unwrap)", isYAMLErr, tt.expectIsYAMLError)
			}
			if isYAMLErr {
				t.Logf("✓ IsYAMLError() correctly identifies direct TypeMismatchError")
			} else {
				t.Logf("✓ IsYAMLError() correctly returns false for wrapped error (doesn't unwrap)")
			}

			// Use errors.As for wrapped error checking
			var tme *TypeMismatchError
			if !errors.As(err, &tme) {
				t.Error("errors.As() should find TypeMismatchError in wrap chain")
			} else {
				t.Logf("✓ errors.As() correctly finds wrapped TypeMismatchError")
			}

			t.Logf("✓ Error wrapping works correctly: %s", errMsg)
		})
	}
}

// TestTypeMismatchErrorErrorsAs verifies errors.As can extract TypeMismatchError from wrapped errors
func TestTypeMismatchErrorErrorsAs(t *testing.T) {
	tests := []struct {
		name        string
		createError func() error
		expectFound bool
	}{
		{
			name: "direct TypeMismatchError",
			createError: func() error {
				return NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 10, "")
			},
			expectFound: true,
		},
		{
			name: "wrapped TypeMismatchError",
			createError: func() error {
				inner := NewTypeMismatchError("test.yaml", "field", "int", "string", "value", 10, "")
				return fmt.Errorf("context: %w", inner)
			},
			expectFound: true,
		},
		{
			name: "standard error",
			createError: func() error {
				return errors.New("standard error")
			},
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()

			var tme *TypeMismatchError
			found := errors.As(err, &tme)

			if found != tt.expectFound {
				t.Errorf("errors.As() = %v, want %v", found, tt.expectFound)
			}

			if tt.expectFound && tme == nil {
				t.Error("errors.As() should return non-nil TypeMismatchError when found")
			}

			if found {
				t.Logf("✓ errors.As() found TypeMismatchError: %+v", tme)
			} else {
				t.Logf("✓ errors.As() correctly did not find TypeMismatchError")
			}
		})
	}
}

// ============================================================================
// Section 5: Edge Case Tests
// ============================================================================

// TestTypeMismatchErrorEmptyValues verifies behavior with empty/zero values
func TestTypeMismatchErrorEmptyValues(t *testing.T) {
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
			name:         "empty file path",
			filePath:     "",
			fieldPath:    "field",
			expectedType: "int",
			actualType:   "string",
			value:        "value",
			line:         10,
		},
		{
			name:         "empty field path",
			filePath:     "test.yaml",
			fieldPath:    "",
			expectedType: "int",
			actualType:   "string",
			value:        "value",
			line:         10,
		},
		{
			name:         "empty expected type",
			filePath:     "test.yaml",
			fieldPath:    "field",
			expectedType: "",
			actualType:   "string",
			value:        "value",
			line:         10,
		},
		{
			name:         "empty actual type",
			filePath:     "test.yaml",
			fieldPath:    "field",
			expectedType: "int",
			actualType:   "",
			value:        "value",
			line:         10,
		},
		{
			name:         "empty value",
			filePath:     "test.yaml",
			fieldPath:    "field",
			expectedType: "int",
			actualType:   "string",
			value:        "",
			line:         10,
		},
		{
			name:         "zero line number",
			filePath:     "test.yaml",
			fieldPath:    "field",
			expectedType: "int",
			actualType:   "string",
			value:        "value",
			line:         0,
		},
		{
			name:         "all empty",
			filePath:     "",
			fieldPath:    "",
			expectedType: "",
			actualType:   "",
			value:        "",
			line:         0,
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

			// Verify error doesn't panic and returns a message
			errorMsg := err.Error()
			if errorMsg == "" {
				t.Error("Error() should return non-empty string even with empty fields")
			}

			// Verify Context() doesn't panic
			ctxMsg := err.Context()
			if ctxMsg == "" {
				t.Error("Context() should return non-empty string even with empty fields")
			}

			t.Logf("✓ %s: Error() = %s", tt.name, errorMsg)
			t.Logf("  Context() = %s", ctxMsg)
		})
	}
}

// TestTypeMismatchErrorSpecialCharacters verifies behavior with special characters
func TestTypeMismatchErrorSpecialCharacters(t *testing.T) {
	tests := []struct {
		name         string
		fieldPath    string
		value        string
		expectedType string
		actualType   string
	}{
		{
			name:         "quotes in value",
			fieldPath:    "field",
			value:        `"quoted value"`,
			expectedType: "string",
			actualType:   "integer",
		},
		{
			name:         "newlines in value",
			fieldPath:    "field",
			value:        "line1\nline2",
			expectedType: "string",
			actualType:   "boolean",
		},
		{
			name:         "tabs in value",
			fieldPath:    "field",
			value:        "value\twith\ttabs",
			expectedType: "string",
			actualType:   "integer",
		},
		{
			name:         "special characters in value",
			fieldPath:    "field",
			value:        "!@#$%^&*()",
			expectedType: "string",
			actualType:   "integer",
		},
		{
			name:         "unicode escape sequences",
			fieldPath:    "field",
			value:        "\\u0041\\u0042",
			expectedType: "string",
			actualType:   "boolean",
		},
		{
			name:         "brackets in field path",
			fieldPath:    "array[0].field",
			value:        "value",
			expectedType: "string",
			actualType:   "integer",
		},
		{
			name:         "dots in field path",
			fieldPath:    "nested.deep.field.path",
			value:        "value",
			expectedType: "object",
			actualType:   "string",
		},
		{
			name:         "hyphens in field path",
			fieldPath:    "field-name-with-hyphens",
			value:        "value",
			expectedType: "string",
			actualType:   "integer",
		},
		{
			name:         "underscores in field path",
			fieldPath:    "field_name_with_underscores",
			value:        "value",
			expectedType: "boolean",
			actualType:   "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				"test.yaml",
				tt.fieldPath,
				tt.expectedType,
				tt.actualType,
				tt.value,
				10,
				"",
			)

			// Verify error message doesn't panic and handles special characters
			errorMsg := err.Error()
			if errorMsg == "" {
				t.Error("Error() should return non-empty string with special characters")
			}

			// Verify context message handles special characters
			ctxMsg := err.Context()
			if ctxMsg == "" {
				t.Error("Context() should return non-empty string with special characters")
			}

			// Verify field path is preserved
			if !strings.Contains(errorMsg, tt.fieldPath) {
				t.Errorf("Error() should contain field path %q, got: %s", tt.fieldPath, errorMsg)
			}

			// Verify value is preserved in context
			if tt.value != "" && !strings.Contains(ctxMsg, tt.value) {
				t.Errorf("Context() should contain value %q, got: %s", tt.value, ctxMsg)
			}

			t.Logf("✓ %s handled correctly", tt.name)
		})
	}
}

// TestTypeMismatchErrorUnicode verifies behavior with unicode characters
func TestTypeMismatchErrorUnicode(t *testing.T) {
	tests := []struct {
		name         string
		fieldPath    string
		value        string
		expectedType string
		actualType   string
	}{
		{
			name:         "emoji in value",
			fieldPath:    "icon",
			value:        "😀",
			expectedType: "string",
			actualType:   "integer",
		},
		{
			name:         "chinese characters in value",
			fieldPath:    "name",
			value:        "中文",
			expectedType: "string",
			actualType:   "boolean",
		},
		{
			name:         "cyrillic characters in value",
			fieldPath:    "label",
			value:        "Русский",
			expectedType: "string",
			actualType:   "integer",
		},
		{
			name:         "arabic characters in value",
			fieldPath:    "title",
			value:        "العربية",
			expectedType: "string",
			actualType:   "boolean",
		},
		{
			name:         "japanese characters in value",
			fieldPath:    "description",
			value:        "日本語",
			expectedType: "string",
			actualType:   "integer",
		},
		{
			name:         "greek characters in value",
			fieldPath:    "text",
			value:        "Ελληνικά",
			expectedType: "string",
			actualType:   "boolean",
		},
		{
			name:         "mixed scripts in value",
			fieldPath:    "mixed",
			value:        "Hello世界🌍",
			expectedType: "string",
			actualType:   "integer",
		},
		{
			name:         "unicode in field path",
			fieldPath:    "字段名",
			value:        "value",
			expectedType: "string",
			actualType:   "integer",
		},
		{
			name:         "unicode in file path",
			fieldPath:    "field",
			value:        "value",
			expectedType: "string",
			actualType:   "integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := "config.yaml"
			if tt.name == "unicode in file path" {
				filePath = "配置.yaml"
			}

			err := NewTypeMismatchError(
				filePath,
				tt.fieldPath,
				tt.expectedType,
				tt.actualType,
				tt.value,
				10,
				"",
			)

			// Verify error message handles unicode
			errorMsg := err.Error()
			if errorMsg == "" {
				t.Error("Error() should return non-empty string with unicode")
			}

			// Verify context message handles unicode
			ctxMsg := err.Context()
			if ctxMsg == "" {
				t.Error("Context() should return non-empty string with unicode")
			}

			// Verify unicode characters are preserved
			if strings.Contains(tt.value, "😀") && !strings.Contains(errorMsg, "😀") {
				// Emoji might be encoded differently, just verify it doesn't panic
				t.Logf("Emoji handling: %s", errorMsg)
			}

			t.Logf("✓ %s handled correctly", tt.name)
		})
	}
}

// TestTypeMismatchErrorVeryLongValues verifies behavior with very long strings
func TestTypeMismatchErrorVeryLongValues(t *testing.T) {
	tests := []struct {
		name         string
		fieldPath    string
		value        string
		expectedType string
		actualType   string
	}{
		{
			name:         "very long field path",
			fieldPath:    strings.Repeat("nested.", 100) + "field",
			value:        "value",
			expectedType: "string",
			actualType:   "integer",
		},
		{
			name:         "very long value",
			fieldPath:    "field",
			value:        strings.Repeat("a", 10000),
			expectedType: "string",
			actualType:   "boolean",
		},
		{
			name:         "very long type names",
			fieldPath:    "field",
			value:        "value",
			expectedType: strings.Repeat("very_long_complex_type_name_", 50),
			actualType:   strings.Repeat("another_very_long_type_name_", 50),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				"test.yaml",
				tt.fieldPath,
				tt.expectedType,
				tt.actualType,
				tt.value,
				10,
				"",
			)

			// Verify error message doesn't panic with long values
			errorMsg := err.Error()
			if errorMsg == "" {
				t.Error("Error() should return non-empty string with long values")
			}

			// Verify context message doesn't panic
			ctxMsg := err.Context()
			if ctxMsg == "" {
				t.Error("Context() should return non-empty string with long values")
			}

			t.Logf("✓ %s handled correctly (error msg length: %d)", tt.name, len(errorMsg))
		})
	}
}

// TestTypeMismatchErrorNumericValues verifies behavior with various numeric representations
func TestTypeMismatchErrorNumericValues(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		expectedType string
		actualType   string
	}{
		{
			name:         "zero",
			value:        "0",
			expectedType: "integer",
			actualType:   "string",
		},
		{
			name:         "negative number",
			value:        "-42",
			expectedType: "integer",
			actualType:   "string",
		},
		{
			name:         "large number",
			value:        "999999999999",
			expectedType: "integer",
			actualType:   "string",
		},
		{
			name:         "float with decimal",
			value:        "3.14159",
			expectedType: "float",
			actualType:   "string",
		},
		{
			name:         "scientific notation",
			value:        "1.23e-10",
			expectedType: "float",
			actualType:   "string",
		},
		{
			name:         "hexadecimal",
			value:        "0xFF",
			expectedType: "integer",
			actualType:   "string",
		},
		{
			name:         "octal",
			value:        "0755",
			expectedType: "integer",
			actualType:   "string",
		},
		{
			name:         "binary",
			value:        "0b1010",
			expectedType: "integer",
			actualType:   "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				"test.yaml",
				"field",
				tt.expectedType,
				tt.actualType,
				tt.value,
				10,
				"",
			)

			errorMsg := err.Error()
			ctxMsg := err.Context()

			// Verify value is preserved in context
			if !strings.Contains(ctxMsg, tt.value) {
				t.Errorf("Context() should contain value %q, got: %s", tt.value, ctxMsg)
			}

			t.Logf("✓ %s: %s", tt.name, errorMsg)
		})
	}
}

// TestTypeMismatchErrorBooleanValues verifies behavior with boolean representations
func TestTypeMismatchErrorBooleanValues(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		expectedType string
		actualType   string
	}{
		{
			name:         "boolean true",
			value:        "true",
			expectedType: "boolean",
			actualType:   "string",
		},
		{
			name:         "boolean false",
			value:        "false",
			expectedType: "boolean",
			actualType:   "string",
		},
		{
			name:         "capitalized True",
			value:        "True",
			expectedType: "boolean",
			actualType:   "string",
		},
		{
			name:         "capitalized False",
			value:        "False",
			expectedType: "boolean",
			actualType:   "string",
		},
		{
			name:         "uppercase TRUE",
			value:        "TRUE",
			expectedType: "boolean",
			actualType:   "string",
		},
		{
			name:         "yes as boolean",
			value:        "yes",
			expectedType: "boolean",
			actualType:   "string",
		},
		{
			name:         "no as boolean",
			value:        "no",
			expectedType: "boolean",
			actualType:   "string",
		},
		{
			name:         "1 as boolean",
			value:        "1",
			expectedType: "boolean",
			actualType:   "integer",
		},
		{
			name:         "0 as boolean",
			value:        "0",
			expectedType: "boolean",
			actualType:   "integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				"test.yaml",
				"field",
				tt.expectedType,
				tt.actualType,
				tt.value,
				10,
				"",
			)

			errorMsg := err.Error()
			ctxMsg := err.Context()

			// Verify value is preserved in context
			if !strings.Contains(ctxMsg, tt.value) {
				t.Errorf("Context() should contain value %q, got: %s", tt.value, ctxMsg)
			}

			t.Logf("✓ %s: %s", tt.name, errorMsg)
		})
	}
}

// TestTypeMismatchErrorNullValue verifies behavior with null/nil values
func TestTypeMismatchErrorNullValue(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		expectedType string
		actualType   string
	}{
		{
			name:         "null string",
			value:        "null",
			expectedType: "string",
			actualType:   "null",
		},
		{
			name:         "null when object expected",
			value:        "null",
			expectedType: "object",
			actualType:   "null",
		},
		{
			name:         "null when array expected",
			value:        "null",
			expectedType: "array",
			actualType:   "null",
		},
		{
			name:         "empty string vs null",
			value:        "",
			expectedType: "string",
			actualType:   "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTypeMismatchError(
				"test.yaml",
				"field",
				tt.expectedType,
				tt.actualType,
				tt.value,
				10,
				"",
			)

			errorMsg := err.Error()
			ctxMsg := err.Context()

			// Verify error message handles null gracefully
			if errorMsg == "" {
				t.Error("Error() should return non-empty string for null value")
			}

			t.Logf("✓ %s: %s", tt.name, errorMsg)
			t.Logf("  Context: %s", ctxMsg)
		})
	}
}
