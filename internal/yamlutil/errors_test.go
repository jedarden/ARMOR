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
			err:      &ParseError{FilePath: "test.yaml"},
			expected: true,
		},
		{
			name:     "ValidationError returns true",
			err:      &ValidationError{FilePath: "test.yaml"},
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
			err:      &ParseError{FilePath: "test.yaml"},
			expected: ErrorTypeParse,
		},
		{
			name:     "ValidationError returns ErrorTypeValidation",
			err:      &ValidationError{FilePath: "test.yaml"},
			expected: ErrorTypeValidation,
		},
		{
			name:     "FileError returns ErrorTypeFile",
			err:      &FileError{Path: "test.yaml"},
			expected: ErrorTypeFile,
		},
		{
			name:     "SchemaValidationError returns ErrorTypeSchema",
			err:      &SchemaValidationError{FilePath: "test.yaml"},
			expected: ErrorTypeSchema,
		},
		{
			name:     "wrapped ParseError returns ErrorTypeParse",
			err:      fmt.Errorf("wrapped: %w", &ParseError{FilePath: "test.yaml"}),
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
			err:      &ParseError{FilePath: "test.yaml"},
			expected: true,
		},
		{
			name:     "wrapped ParseError returns true",
			err:      fmt.Errorf("wrapped: %w", &ParseError{FilePath: "test.yaml"}),
			expected: true,
		},
		{
			name:     "ValidationError returns false",
			err:      &ValidationError{FilePath: "test.yaml"},
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
			wantMessage:  "parse error in config.yaml at line 10: invalid syntax",
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
			wantMessage:  "parse error in data.yaml at line 15: unexpected end of file",
			wantCode:     ErrCodeInvalidStructure,
			wantErrorType: ErrorTypeParse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewParseError(tt.filePath, tt.message, tt.line, tt.column, tt.code)

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
			wantMessage:  "validation error in data.yaml at field api.timeout: constraint violation (constraint: must be positive integer)",
			wantCode:     ErrCodeConstraintViolation,
			wantErrorType: ErrorTypeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(tt.filePath, tt.message, tt.fieldPath, tt.constraint, tt.code)

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
			err: NewValidationError("config.yaml", "invalid port", "server.port", "must be 1-65535", ErrCodeInvalidValue),
			wantFields: []string{
				"Error: invalid port",
				"Type: validation",
				"Field: server.port",
				"Constraint: must be 1-65535",
			},
		},
		{
			name: "validation error without constraint",
			err: NewValidationError("test.yaml", "validation failed", "", "", ErrCodeValidationFailed),
			wantFields: []string{
				"Error: validation failed",
				"Type: validation",
			},
		},
		{
			name: "validation error with line and column",
			err: &ValidationError{
				FilePath:   "data.yaml",
				Message:    "syntax error",
				Line:       10,
				Column:     5,
				Constraint: "must be string",
				ErrorCode:  ErrCodeInvalidValue,
				Type:       ErrorTypeValidation,
			},
			wantFields: []string{
				"Error: syntax error",
				"Type: validation",
				"Location: Line 10, Column 5",
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

