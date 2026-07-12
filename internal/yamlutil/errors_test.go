// Package yamlutil tests for error helper functions
package yamlutil

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsYAMLError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
		{
			name:     "standard error returns false",
			err:      errors.New("standard error"),
			expected: false,
		},
		{
			name:     "ParseError returns true",
			err:      NewParseError("test.yaml", "", 0, 0, "", "", ""),
			expected: true,
		},
		{
			name:     "ValidationError returns true",
			err:      NewValidationError("test.yaml", "", "", "", "", 0, 0, "", ""),
			expected: true,
		},
		{
			name:     "FileError returns true",
			err:      &FileError{Path: "test.yaml"},
			expected: true,
		},
		{
			name:     "SchemaValidationError returns true",
			err:      &SchemaValidationError{FilePath: "test.yaml"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsYAMLError(tt.err)
			if result != tt.expected {
				t.Errorf("IsYAMLError() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetYAMLErrorType(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorType
	}{
		{
			name:     "nil error returns empty string",
			err:      nil,
			expected: "",
		},
		{
			name:     "standard error returns empty string",
			err:      errors.New("standard error"),
			expected: "",
		},
		{
			name:     "ParseError returns ErrorTypeParse",
			err:      NewParseError("test.yaml", "", 0, 0, "", "", ""),
			expected: ErrorTypeParse,
		},
		{
			name:     "ValidationError returns ErrorTypeValidation",
			err:      NewValidationError("test.yaml", "", "", "", "", 0, 0, "", ""),
			expected: ErrorTypeValidation,
		},
		{
			name:     "FileError returns ErrorTypeFile",
			err:      &FileError{Path: "test.yaml"},
			expected: ErrorTypeFile,
		},
		{
			name:     "SchemaValidationError returns ErrorTypeSchemaValidate",
			err:      &SchemaValidationError{FilePath: "test.yaml"},
			expected: ErrorTypeSchemaValidate,
		},
		{
			name:     "wrapped ParseError returns ErrorTypeParse",
			err:      fmt.Errorf("wrapped: %w", NewParseError("test.yaml", "", 0, 0, "", "", "")),
			expected: ErrorTypeParse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetYAMLErrorType(tt.err)
			if result != tt.expected {
				t.Errorf("GetYAMLErrorType() = %v, expected %v", result, tt.expected)
			}
		})
	}
}


func TestWrapError(t *testing.T) {
	innerErr := errors.New("inner error")
	wrapped := fmt.Errorf("context message: %w", innerErr)

	if wrapped == nil {
		t.Fatal("wrapped error returned nil")
	}

	// Check that the error message contains the context
	msg := wrapped.Error()
	if !contains(msg, "context message") {
		t.Errorf("wrapped error message should contain context, got: %s", msg)
	}
	if !contains(msg, "inner error") {
		t.Errorf("wrapped error message should contain inner error, got: %s", msg)
	}

	// Check that unwrapping returns the original error
	if unwrapped := errors.Unwrap(wrapped); unwrapped != innerErr {
		t.Errorf("unwrapped error should be the original error, got: %v", unwrapped)
	}
}

func TestIsParseError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
		{
			name:     "standard error returns false",
			err:      errors.New("standard error"),
			expected: false,
		},
		{
			name:     "ParseError returns true",
			err:      NewParseError("test.yaml", "", 0, 0, "", "", ""),
			expected: true,
		},
		{
			name:     "wrapped ParseError returns true",
			err:      fmt.Errorf("wrapped: %w", NewParseError("test.yaml", "", 0, 0, "", "", "")),
			expected: true,
		},
		{
			name:     "ValidationError returns false",
			err:      NewValidationError("test.yaml", "", "", "", "", 0, 0, "", ""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsParseError(tt.err)
			if result != tt.expected {
				t.Errorf("IsParseError() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestNewParseError(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		message      string
		line         int
		column       int
		code         ErrorCode
		expected     string
		actual       string
		wantMessage  string
		wantCode     ErrorCode
		wantErrorType ErrorType
	}{
		{
			name:         "basic parse error",
			filePath:     "config.yaml",
			message:      "invalid syntax",
			line:         10,
			column:       5,
			code:         ErrCodeInvalidSyntax,
			expected:     "",
			actual:       "",
			wantMessage:  "parse error in config.yaml at line 10, column 5: invalid syntax",
			wantCode:     ErrCodeInvalidSyntax,
			wantErrorType: ErrorTypeParse,
		},
		{
			name:         "parse error with default code",
			filePath:     "test.yaml",
			message:      "unexpected token",
			line:         3,
			column:       0,
			code:         "",
			expected:     "",
			actual:       "",
			wantMessage:  "parse error in test.yaml at line 3: unexpected token",
			wantCode:     ErrCodeParseError,
			wantErrorType: ErrorTypeParse,
		},
		{
			name:         "parse error without line number",
			filePath:     "unknown.yaml",
			message:      "file is corrupted",
			line:         0,
			column:       0,
			code:         ErrCodeParseError,
			expected:     "",
			actual:       "",
			wantMessage:  "parse error in unknown.yaml: file is corrupted",
			wantCode:     ErrCodeParseError,
			wantErrorType: ErrorTypeParse,
		},
		{
			name:         "parse error with line and column",
			filePath:     "data.yaml",
			message:      "unexpected end of file",
			line:         15,
			column:       8,
			code:         ErrCodeInvalidStructure,
			expected:     "",
			actual:       "",
			wantMessage:  "parse error in data.yaml at line 15, column 8: unexpected end of file",
			wantCode:     ErrCodeInvalidStructure,
			wantErrorType: ErrorTypeParse,
		},
		{
			name:         "parse error with expected and actual",
			filePath:     "schema.yaml",
			message:      "type mismatch",
			line:         7,
			column:       12,
			code:         ErrCodeTypeMismatch,
			expected:     "string",
			actual:       "integer",
			wantMessage:  "parse error in schema.yaml at line 7, column 12: type mismatch (expected: string, actual: integer)",
			wantCode:     ErrCodeTypeMismatch,
			wantErrorType: ErrorTypeParse,
		},
		{
			name:         "parse error with only expected",
			filePath:     "config.yaml",
			message:      "missing required field",
			line:         5,
			column:       0,
			code:         ErrCodeRequiredField,
			expected:     "identifier",
			actual:       "",
			wantMessage:  "parse error in config.yaml at line 5: missing required field (expected: identifier)",
			wantCode:     ErrCodeRequiredField,
			wantErrorType: ErrorTypeParse,
		},
		{
			name:         "parse error with only actual",
			filePath:     "data.yaml",
			message:      "unexpected value",
			line:         20,
			column:       3,
			code:         ErrCodeInvalidValue,
			expected:     "",
			actual:       "null",
			wantMessage:  "parse error in data.yaml at line 20, column 3: unexpected value (actual: null)",
			wantCode:     ErrCodeInvalidValue,
			wantErrorType: ErrorTypeParse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewParseError(tt.filePath, tt.message, tt.line, tt.column, tt.code, tt.expected, tt.actual)

			// Verify fields are set correctly
			if err.FilePath != tt.filePath {
				t.Errorf("FilePath = %q, want %q", err.FilePath, tt.filePath)
			}
			if err.Message != tt.message {
				t.Errorf("Message = %q, want %q", err.Message, tt.message)
			}
			if err.Line != tt.line {
				t.Errorf("Line = %d, want %d", err.Line, tt.line)
			}
			if err.Column != tt.column {
				t.Errorf("Column = %d, want %d", err.Column, tt.column)
			}
			if err.Expected != tt.expected {
				t.Errorf("Expected = %q, want %q", err.Expected, tt.expected)
			}
			if err.Actual != tt.actual {
				t.Errorf("Actual = %q, want %q", err.Actual, tt.actual)
			}
			if err.Code() != tt.wantCode {
				t.Errorf("Code() = %q, want %q", err.Code(), tt.wantCode)
			}
			if err.YAMLErrorType() != tt.wantErrorType {
				t.Errorf("YAMLErrorType() = %q, want %q", err.YAMLErrorType(), tt.wantErrorType)
			}

			// Verify Error() method returns formatted message with position
			errorMsg := err.Error()
			if errorMsg != tt.wantMessage {
				t.Errorf("Error() = %q, want %q", errorMsg, tt.wantMessage)
			}

			// Verify it's recognized as a YAMLError
			if !IsYAMLError(err) {
				t.Error("NewParseError() should implement YAMLError interface")
			}

			// Verify it's recognized as a ParseError
			if !IsParseError(err) {
				t.Error("NewParseError() should be recognized as ParseError")
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNewValidationError(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		message      string
		fieldPath    string
		constraint   string
		code         ErrorCode
		line         int
		column       int
		wantMessage  string
		wantCode     ErrorCode
		wantErrorType ErrorType
	}{
		{
			name:         "basic validation error",
			filePath:     "config.yaml",
			message:      "invalid value",
			fieldPath:    "server.port",
			constraint:   "must be between 1-65535",
			code:         ErrCodeInvalidValue,
			line:         0,
			column:       0,
			wantMessage:  "validation error in config.yaml at field server.port: invalid value (constraint: must be between 1-65535)",
			wantCode:     ErrCodeInvalidValue,
			wantErrorType: ErrorTypeValidation,
		},
		{
			name:         "validation error with default code",
			filePath:     "test.yaml",
			message:      "required field missing",
			fieldPath:    "database.name",
			constraint:   "required",
			code:         "",
			line:         0,
			column:       0,
			wantMessage:  "validation error in test.yaml at field database.name: required field missing (constraint: required)",
			wantCode:     ErrCodeValidationFailed,
			wantErrorType: ErrorTypeValidation,
		},
		{
			name:         "validation error without field path",
			filePath:     "unknown.yaml",
			message:      "validation failed",
			fieldPath:    "",
			constraint:   "",
			code:         ErrCodeValidationFailed,
			line:         0,
			column:       0,
			wantMessage:  "validation error in unknown.yaml: validation failed",
			wantCode:     ErrCodeValidationFailed,
			wantErrorType: ErrorTypeValidation,
		},
		{
			name:         "validation error with all fields",
			filePath:     "data.yaml",
			message:      "constraint violation",
			fieldPath:    "api.timeout",
			constraint:   "must be positive integer",
			code:         ErrCodeConstraintViolation,
			line:         0,
			column:       0,
			wantMessage:  "validation error in data.yaml at field api.timeout: constraint violation (constraint: must be positive integer)",
			wantCode:     ErrCodeConstraintViolation,
			wantErrorType: ErrorTypeValidation,
		},
		{
			name:         "validation error with line and column",
			filePath:     "app.yaml",
			message:      "port out of range",
			fieldPath:    "server.port",
			constraint:   "must be between 1-65535",
			code:         ErrCodeInvalidValue,
			line:         15,
			column:       12,
			wantMessage:  "validation error in app.yaml at line 15, column 12 at field server.port: port out of range (constraint: must be between 1-65535)",
			wantCode:     ErrCodeInvalidValue,
			wantErrorType: ErrorTypeValidation,
		},
		{
			name:         "validation error with line only",
			filePath:     "manifest.yaml",
			message:      "replicas must be positive",
			fieldPath:    "spec.replicas",
			constraint:   "must be >= 0",
			code:         ErrCodeConstraintViolation,
			line:         8,
			column:       0,
			wantMessage:  "validation error in manifest.yaml at line 8 at field spec.replicas: replicas must be positive (constraint: must be >= 0)",
			wantCode:     ErrCodeConstraintViolation,
			wantErrorType: ErrorTypeValidation,
		},
		{
			name:         "validation error with nested field path and line",
			filePath:     "deployment.yaml",
			message:      "invalid image tag",
			fieldPath:    "spec.template.spec.containers[0].image",
			constraint:   "must match registry/*:tag pattern",
			code:         ErrCodeInvalidValue,
			line:         22,
			column:       18,
			wantMessage:  "validation error in deployment.yaml at line 22, column 18 at field spec.template.spec.containers[0].image: invalid image tag (constraint: must match registry/*:tag pattern)",
			wantCode:     ErrCodeInvalidValue,
			wantErrorType: ErrorTypeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(tt.filePath, tt.message, tt.fieldPath, tt.constraint, tt.code, tt.line, tt.column, tt.wantErrorType, tt.fieldPath)

			// Verify fields are set correctly
			if err.FilePath != tt.filePath {
				t.Errorf("FilePath = %q, want %q", err.FilePath, tt.filePath)
			}
			if err.Message != tt.message {
				t.Errorf("Message = %q, want %q", err.Message, tt.message)
			}
			if err.FieldPath != tt.fieldPath {
				t.Errorf("FieldPath = %q, want %q", err.FieldPath, tt.fieldPath)
			}
			if err.Constraint != tt.constraint {
				t.Errorf("Constraint = %q, want %q", err.Constraint, tt.constraint)
			}
			if err.Line != tt.line {
				t.Errorf("Line = %d, want %d", err.Line, tt.line)
			}
			if err.Column != tt.column {
				t.Errorf("Column = %d, want %d", err.Column, tt.column)
			}
			if err.Code() != tt.wantCode {
				t.Errorf("Code() = %q, want %q", err.Code(), tt.wantCode)
			}
			if err.YAMLErrorType() != tt.wantErrorType {
				t.Errorf("YAMLErrorType() = %q, want %q", err.YAMLErrorType(), tt.wantErrorType)
			}

			// Verify Error() method returns formatted message with path context
			errorMsg := err.Error()
			if errorMsg != tt.wantMessage {
				t.Errorf("Error() = %q, want %q", errorMsg, tt.wantMessage)
			}

			// Verify it's recognized as a YAMLError
			if !IsYAMLError(err) {
				t.Error("NewValidationError() should implement YAMLError interface")
			}

			// Verify it's recognized as a ValidationError
			if !IsValidationError(err) {
				t.Error("NewValidationError() should be recognized as ValidationError")
			}
		})
	}
}

func TestValidationErrorString(t *testing.T) {
	tests := []struct {
		name       string
		err        *ValidationError
		wantFields []string
	}{
		{
			name: "validation error with constraint",
			err: NewValidationError("config.yaml", "invalid port", "server.port", "must be 1-65535", ErrCodeInvalidValue, 0, 0, "", "server.port"),
			wantFields: []string{
				"Error: invalid port",
				"Type: validation",
				"Field: server.port",
				"Constraint: must be 1-65535",
			},
		},
		{
			name: "validation error without constraint",
			err: NewValidationError("test.yaml", "validation failed", "", "", ErrCodeValidationFailed, 0, 0, "", ""),
			wantFields: []string{
				"Error: validation failed",
				"Type: validation",
			},
		},
		{
			name: "validation error with line and column",
			err: NewValidationError("data.yaml", "syntax error", "", "must be string", ErrCodeInvalidValue, 10, 5, ErrorTypeValidation, ""),
			wantFields: []string{
				"Error: syntax error",
				"Type: validation",
				"Location: Line 10, Column 5",
			},
		},
		{
			name: "validation error with line, field path, and constraint",
			err: NewValidationError("app.yaml", "value out of range", "database.connectionTimeout", "must be between 1-300", ErrCodeConstraintViolation, 25, 15, ErrorTypeConstraint, "database.connectionTimeout"),
			wantFields: []string{
				"Error: value out of range",
				"Type: constraint",
				"Location: Line 25, Column 15",
				"Field: database.connectionTimeout",
				"Constraint: must be between 1-300",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.String()
			for _, field := range tt.wantFields {
				if !contains(result, field) {
					t.Errorf("String() = %q, want to contain %q", result, field)
				}
			}
		})
	}
}


// TestTypeMismatchErrorFormatting verifies type mismatch errors include expected and actual types
func TestTypeMismatchErrorFormatting(t *testing.T) {
	tests := []struct {
		name        string
		err         *TypeMismatchError
		wantMessage string
		wantContext string
	}{
		{
			name: "type mismatch with line and field path",
			err: &TypeMismatchError{
				FilePath:     "config.yaml",
				FieldPath:    "server.port",
				ExpectedType: "integer",
				ActualType:   "string",
				Value:        "abc",
				Line:         15,
			},
			wantMessage: "type mismatch in config.yaml at line 15, field server.port: expected integer, got string",
			wantContext: "field: server.port, expected type: integer, actual type: string, value: abc",
		},
		{
			name: "type mismatch without line number",
			err: &TypeMismatchError{
				FilePath:     "data.yaml",
				FieldPath:    "database.timeout",
				ExpectedType: "integer",
				ActualType:   "boolean",
				Value:        "true",
				Line:         0,
			},
			wantMessage: "type mismatch in data.yaml, field database.timeout: expected integer, got boolean",
			wantContext: "field: database.timeout, expected type: integer, actual type: boolean, value: true",
		},
		{
			name: "type mismatch with nested field path",
			err: &TypeMismatchError{
				FilePath:     "app.yaml",
				FieldPath:    "servers.api.responses[0].statusCode",
				ExpectedType: "integer",
				ActualType:   "string",
				Value:        "\"200\"",
				Line:         42,
			},
			wantMessage: "type mismatch in app.yaml at line 42, field servers.api.responses[0].statusCode: expected integer, got string",
			wantContext: "field: servers.api.responses[0].statusCode, expected type: integer, actual type: string, value: \"200\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify Error() message
			errorMsg := tt.err.Error()
			if errorMsg != tt.wantMessage {
				t.Errorf("Error() = %q, want %q", errorMsg, tt.wantMessage)
			}

			// Verify Context() message
			ctxMsg := tt.err.Context()
			if ctxMsg != tt.wantContext {
				t.Errorf("Context() = %q, want %q", ctxMsg, tt.wantContext)
			}

			// Verify it's recognized as a YAMLError
			if !IsYAMLError(tt.err) {
				t.Error("TypeMismatchError should implement YAMLError interface")
			}

			// Verify error type
			if tt.err.YAMLErrorType() != ErrorTypeTypeMismatch {
				t.Errorf("YAMLErrorType() = %q, want %q", tt.err.YAMLErrorType(), ErrorTypeTypeMismatch)
			}

			// Verify error code
			if tt.err.Code() != ErrCodeTypeMismatch {
				t.Errorf("Code() = %q, want %q", tt.err.Code(), ErrCodeTypeMismatch)
			}
		})
	}
}

// TestConstraintErrorFieldPathFormatting verifies constraint errors include field path
func TestConstraintErrorFieldPathFormatting(t *testing.T) {
	tests := []struct {
		name        string
		err         *ConstraintError
		wantMessage string
		wantContext string
	}{
		{
			name: "constraint error with line and field path",
			err: &ConstraintError{
				FilePath:       "config.yaml",
				FieldPath:      "server.port",
				ConstraintType: "range",
				Constraint:     "must be between 1-65535",
				Value:          "70000",
				Line:           12,
			},
			wantMessage: "constraint violation in config.yaml at line 12, field server.port: must be between 1-65535",
			wantContext: "field: server.port, constraint: must be between 1-65535, value: 70000",
		},
		{
			name: "constraint error without line number",
			err: &ConstraintError{
				FilePath:       "data.yaml",
				FieldPath:      "api.version",
				ConstraintType: "pattern",
				Constraint:     "must match ^v\\d+\\.\\d+$",
				Value:          "v1",
				Line:           0,
			},
			wantMessage: "constraint violation in data.yaml, field api.version: must match ^v\\d+\\.\\d+$",
			wantContext: "field: api.version, constraint: must match ^v\\d+\\.\\d+$, value: v1",
		},
		{
			name: "constraint error with nested field path",
			err: &ConstraintError{
				FilePath:       "app.yaml",
				FieldPath:      "database.connectionPool.maxConnections",
				ConstraintType: "range",
				Constraint:     "must be between 1-100",
				Value:          "150",
				Line:           28,
			},
			wantMessage: "constraint violation in app.yaml at line 28, field database.connectionPool.maxConnections: must be between 1-100",
			wantContext: "field: database.connectionPool.maxConnections, constraint: must be between 1-100, value: 150",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify Error() message
			errorMsg := tt.err.Error()
			if errorMsg != tt.wantMessage {
				t.Errorf("Error() = %q, want %q", errorMsg, tt.wantMessage)
			}

			// Verify Context() message
			ctxMsg := tt.err.Context()
			if ctxMsg != tt.wantContext {
				t.Errorf("Context() = %q, want %q", ctxMsg, tt.wantContext)
			}

			// Verify it's recognized as a YAMLError
			if !IsYAMLError(tt.err) {
				t.Error("ConstraintError should implement YAMLError interface")
			}

			// Verify error type
			if tt.err.YAMLErrorType() != ErrorTypeConstraint {
				t.Errorf("YAMLErrorType() = %q, want %q", tt.err.YAMLErrorType(), ErrorTypeConstraint)
			}

			// Verify error code
			if tt.err.Code() != ErrCodeConstraintViolation {
				t.Errorf("Code() = %q, want %q", tt.err.Code(), ErrCodeConstraintViolation)
			}
		})
	}
}

// TestFieldNotFoundErrorFormatting verifies field not found errors include field path
func TestFieldNotFoundErrorFormatting(t *testing.T) {
	tests := []struct {
		name        string
		err         *FieldNotFoundError
		wantMessage string
		wantContext string
	}{
		{
			name: "field not found with line number",
			err: NewFieldNotFoundError("config.yaml", "database.host", 8, ""),
			wantMessage: "required field missing in config.yaml at line 8: database.host",
			wantContext: "required field not found: database.host",
		},
		{
			name: "field not found without line number",
			err: NewFieldNotFoundError("data.yaml", "api.endpoint", 0, ""),
			wantMessage: "required field missing in data.yaml: api.endpoint",
			wantContext: "required field not found: api.endpoint",
		},
		{
			name: "field not found with nested field path",
			err: NewFieldNotFoundError("app.yaml", "servers.api.responses[0].body", 35, ""),
			wantMessage: "required field missing in app.yaml at line 35: servers.api.responses[0].body",
			wantContext: "required field not found: servers.api.responses[0].body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify Error() message
			errorMsg := tt.err.Error()
			if errorMsg != tt.wantMessage {
				t.Errorf("Error() = %q, want %q", errorMsg, tt.wantMessage)
			}

			// Verify Context() message
			ctxMsg := tt.err.Context()
			if ctxMsg != tt.wantContext {
				t.Errorf("Context() = %q, want %q", ctxMsg, tt.wantContext)
			}

			// Verify it's recognized as a YAMLError
			if !IsYAMLError(tt.err) {
				t.Error("FieldNotFoundError should implement YAMLError interface")
			}

			// Verify error type
			if tt.err.YAMLErrorType() != ErrorTypeFieldNotFound {
				t.Errorf("YAMLErrorType() = %q, want %q", tt.err.YAMLErrorType(), ErrorTypeFieldNotFound)
			}

			// Verify error code
			if tt.err.Code() != ErrCodeRequiredField {
				t.Errorf("Code() = %q, want %q", tt.err.Code(), ErrCodeRequiredField)
			}
		})
	}
}

// TestValidationErrorWithTypeInformation verifies that ValidationError includes type information
func TestValidationErrorWithTypeInformation(t *testing.T) {
	tests := []struct {
		name         string
		expectedType string
		actualType   string
		message      string
		fieldPath    string
	}{
		{
			name:         "with both expected and actual types",
			expectedType: "integer",
			actualType:   "string",
			message:      "invalid type",
			fieldPath:    "server.port",
		},
		{
			name:         "with only expected type",
			expectedType: "boolean",
			actualType:   "",
			message:      "type check failed",
			fieldPath:    "feature.enabled",
		},
		{
			name:         "with only actual type",
			expectedType: "",
			actualType:   "array",
			message:      "wrong type found",
			fieldPath:    "config.items",
		},
		{
			name:         "with constraint and types",
			expectedType: "string",
			actualType:   "integer",
			message:      "value validation failed",
			fieldPath:    "database.host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(
				"config.yaml",
				tt.message,
				tt.fieldPath,
				"must be valid",
				ErrCodeInvalidValue,
				10,
				5,
				ErrorTypeValidation,
				tt.fieldPath,
			)
			// Set type information directly
			err.ExpectedType = tt.expectedType
			err.ActualType = tt.actualType

			errorMsg := err.Error()

			// Verify expected type is included if set
			if tt.expectedType != "" && !contains(errorMsg, tt.expectedType) {
				t.Errorf("Error should include expected type %q, got: %s", tt.expectedType, errorMsg)
			}

			// Verify actual type is included if set
			if tt.actualType != "" && !contains(errorMsg, tt.actualType) {
				t.Errorf("Error should include actual type %q, got: %s", tt.actualType, errorMsg)
			}

			// Verify the format includes "expected" and "got" when both types are present
			if tt.expectedType != "" && tt.actualType != "" {
				expectedFormat := fmt.Sprintf("expected %s, got %s", tt.expectedType, tt.actualType)
				if !contains(errorMsg, expectedFormat) {
					t.Errorf("Error should include format %q, got: %s", expectedFormat, errorMsg)
				}
			}

			t.Logf("✓ Error message: %s", errorMsg)
		})
	}
}

// TestValidationErrorStringWithTypeInformation verifies String() method includes type information
func TestValidationErrorStringWithTypeInformation(t *testing.T) {
	err := NewValidationError(
		"config.yaml",
		"type validation failed",
		"server.port",
		"must be integer",
		ErrCodeInvalidValue,
		10,
		5,
		ErrorTypeValidation,
		"server.port",
	)
	err.ExpectedType = "integer"
	err.ActualType = "string"

	stringOutput := err.String()

	// Verify String() output includes type information
	if !contains(stringOutput, "Expected Type: integer") {
		t.Error("String() should include expected type")
	}
	if !contains(stringOutput, "Actual Type: string") {
		t.Error("String() should include actual type")
	}

	t.Logf("String() output:\n%s", stringOutput)
}
