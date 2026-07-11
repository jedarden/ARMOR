package yamlutil

import (
	"testing"
)

// TestNewValidationErrorPathHandling verifies that NewValidationError handles
// nil and empty path values correctly without crashing
func TestNewValidationErrorPathHandling(t *testing.T) {
	tests := []struct {
		name          string
		filePath      string
		message       string
		fieldPath     string
		constraint    string
		code          ErrorCode
		line          int
		column        int
		errorType     ErrorType
		path          string
		wantPath      string
		wantErrorCode ErrorCode
	}{
		{
			name:      "empty string path uses fieldPath as fallback",
			filePath:  "config.yaml",
			message:   "invalid value",
			fieldPath: "server.port",
			constraint: "must be between 1-65535",
			code:      ErrCodeInvalidValue,
			line:      10,
			column:    5,
			errorType: ErrorTypeValidation,
			path:      "",
			wantPath:  "server.port",
			wantErrorCode: ErrCodeInvalidValue,
		},
		{
			name:      "non-empty path is stored correctly",
			filePath:  "config.yaml",
			message:   "invalid value",
			fieldPath: "server.port",
			constraint: "must be between 1-65535",
			code:      ErrCodeInvalidValue,
			line:      10,
			column:    5,
			errorType: ErrorTypeValidation,
			path:      "spec.replicas",
			wantPath:  "spec.replicas",
			wantErrorCode: ErrCodeInvalidValue,
		},
		{
			name:      "both path and fieldPath empty stays empty",
			filePath:  "test.yaml",
			message:   "validation failed",
			fieldPath: "",
			constraint: "",
			code:      ErrCodeValidationFailed,
			line:      0,
			column:    0,
			errorType: "",
			path:      "",
			wantPath:  "",
			wantErrorCode: ErrCodeValidationFailed,
		},
		{
			name:      "path with nested field path",
			filePath:  "deployment.yaml",
			message:   "replicas must be positive",
			fieldPath: "spec.replicas",
			constraint: "must be > 0",
			code:      ErrCodeConstraintViolation,
			line:      15,
			column:    10,
			errorType: ErrorTypeConstraint,
			path:      "spec.template.spec.replicas",
			wantPath:  "spec.template.spec.replicas",
			wantErrorCode: ErrCodeConstraintViolation,
		},
		{
			name:      "empty path with fieldPath uses fieldPath as fallback",
			filePath:  "service.yaml",
			message:   "port validation error",
			fieldPath: "spec.ports[0].port",
			constraint: "must be between 1-65535",
			code:      ErrCodeInvalidValue,
			line:      20,
			column:    15,
			errorType: ErrorTypeValidation,
			path:      "",
			wantPath:  "spec.ports[0].port",
			wantErrorCode: ErrCodeInvalidValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call NewValidationError with the test parameters
			err := NewValidationError(
				tt.filePath,
				tt.message,
				tt.fieldPath,
				tt.constraint,
				tt.code,
				tt.line,
				tt.column,
				tt.errorType,
				tt.path,
			)

			// Verify the error was created without crashing
			if err == nil {
				t.Fatal("NewValidationError should not return nil")
			}

			// Verify the Path field is set correctly
			if err.Path != tt.wantPath {
				t.Errorf("Path field = %q, want %q", err.Path, tt.wantPath)
			}

			// Verify other fields are also set correctly
			if err.FilePath != tt.filePath {
				t.Errorf("FilePath field = %q, want %q", err.FilePath, tt.filePath)
			}

			if err.Message != tt.message {
				t.Errorf("Message field = %q, want %q", err.Message, tt.message)
			}

			if err.FieldPath != tt.fieldPath {
				t.Errorf("FieldPath field = %q, want %q", err.FieldPath, tt.fieldPath)
			}

			if err.Code() != tt.wantErrorCode {
				t.Errorf("Code() = %q, want %q", err.Code(), tt.wantErrorCode)
			}

			// Verify the error implements YAMLError interface
			if !IsYAMLError(err) {
				t.Error("ValidationError should implement YAMLError interface")
			}

			// Verify it's recognized as a ValidationError
			if !IsValidationError(err) {
				t.Error("Should be recognized as ValidationError")
			}

			t.Logf("✓ Test passed: Path=%q, FilePath=%q, Message=%q", err.Path, err.FilePath, err.Message)
		})
	}
}
