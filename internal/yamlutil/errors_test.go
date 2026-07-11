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
