// Package yamlutil tests for enhanced ParseError design.
package yamlutil

import (
	"fmt"
	"strings"
	"testing"
)

// ============================================================================
// EnhancedParseError Construction Tests
// ============================================================================

func TestNewSyntaxParseError(t *testing.T) {
	err := NewSyntaxParseError(
		"test.yaml",
		"unexpected token",
		5,
		10,
		":",
		"}",
	)

	if err.Kind != ParseErrorKindSyntax {
		t.Errorf("expected Kind=%s, got %s", ParseErrorKindSyntax, err.Kind)
	}

	if err.FilePath != "test.yaml" {
		t.Errorf("expected FilePath='test.yaml', got '%s'", err.FilePath)
	}

	if err.LocationInfo.Line != 5 {
		t.Errorf("expected Line=5, got %d", err.LocationInfo.Line)
	}

	if err.LocationInfo.Column != 10 {
		t.Errorf("expected Column=10, got %d", err.LocationInfo.Column)
	}

	if err.Detail.Expected != ":" {
		t.Errorf("expected Expected=':', got '%s'", err.Detail.Expected)
	}

	if err.Detail.Found != "}" {
		t.Errorf("expected Found='}', got '%s'", err.Detail.Found)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "syntax error") {
		t.Errorf("expected error message to contain 'syntax error', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "line 5") {
		t.Errorf("expected error message to contain 'line 5', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "column 10") {
		t.Errorf("expected error message to contain 'column 10', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "expected: :") {
		t.Errorf("expected error message to contain 'expected: :', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "found: }") {
		t.Errorf("expected error message to contain 'found: }', got: %s", errMsg)
	}
}

func TestNewStructureParseError(t *testing.T) {
	err := NewStructureParseError(
		"config.yaml",
		"duplicate key found",
		15,
		"port",
		"server.config",
	)

	if err.Kind != ParseErrorKindStructure {
		t.Errorf("expected Kind=%s, got %s", ParseErrorKindStructure, err.Kind)
	}

	if err.Detail.DuplicateKey != "port" {
		t.Errorf("expected DuplicateKey='port', got '%s'", err.Detail.DuplicateKey)
	}

	if err.Detail.Location != "server.config" {
		t.Errorf("expected Location='server.config', got '%s'", err.Detail.Location)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "structure error") {
		t.Errorf("expected error message to contain 'structure error', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "duplicate key: port") {
		t.Errorf("expected error message to contain 'duplicate key: port', got: %s", errMsg)
	}
}

func TestNewTypeMismatchParseError(t *testing.T) {
	err := NewTypeMismatchParseError(
		"data.yaml",
		"cannot convert string to int",
		20,
		"server.port",
		"int",
		"string",
		"\"8080\"",
	)

	if err.Kind != ParseErrorKindTypeMismatch {
		t.Errorf("expected Kind=%s, got %s", ParseErrorKindTypeMismatch, err.Kind)
	}

	if err.Detail.FieldPath != "server.port" {
		t.Errorf("expected FieldPath='server.port', got '%s'", err.Detail.FieldPath)
	}

	if err.Detail.ExpectedType != "int" {
		t.Errorf("expected ExpectedType='int', got '%s'", err.Detail.ExpectedType)
	}

	if err.Detail.ActualType != "string" {
		t.Errorf("expected ActualType='string', got '%s'", err.Detail.ActualType)
	}

	if err.Detail.Value != "\"8080\"" {
		t.Errorf("expected Value='\"8080\"', got '%s'", err.Detail.Value)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "type_mismatch error") {
		t.Errorf("expected error message to contain 'type_mismatch error', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "field: server.port") {
		t.Errorf("expected error message to contain 'field: server.port', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "expected: int") {
		t.Errorf("expected error message to contain 'expected: int', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "got: string") {
		t.Errorf("expected error message to contain 'got: string', got: %s", errMsg)
	}
}

func TestNewIOParseError(t *testing.T) {
	underlyingErr := fmt.Errorf("file not found")
	err := NewIOParseError(
		"missing.yaml",
		"failed to read file",
		0,
		underlyingErr,
	)

	if err.Kind != ParseErrorKindIO {
		t.Errorf("expected Kind=%s, got %s", ParseErrorKindIO, err.Kind)
	}

	if err.UnderlyingErr != underlyingErr {
		t.Errorf("expected UnderlyingErr to be set")
	}

	if err.Unwrap() != underlyingErr {
		t.Errorf("expected Unwrap() to return underlying error")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "io error") {
		t.Errorf("expected error message to contain 'io error', got: %s", errMsg)
	}
}

func TestNewValidationParseError(t *testing.T) {
	err := NewValidationParseError(
		"app.yaml",
		"port number out of range",
		30,
		"server.port",
		"range",
		"port must be between 1 and 65535",
	)

	if err.Kind != ParseErrorKindValidation {
		t.Errorf("expected Kind=%s, got %s", ParseErrorKindValidation, err.Kind)
	}

	if err.Detail.FieldPath != "server.port" {
		t.Errorf("expected FieldPath='server.port', got '%s'", err.Detail.FieldPath)
	}

	if err.Detail.ConstraintType != "range" {
		t.Errorf("expected ConstraintType='range', got '%s'", err.Detail.ConstraintType)
	}

	if err.Detail.Constraint != "port must be between 1 and 65535" {
		t.Errorf("expected Constraint='port must be between 1 and 65535', got '%s'", err.Detail.Constraint)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "validation error") {
		t.Errorf("expected error message to contain 'validation error', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "field: server.port") {
		t.Errorf("expected error message to contain 'field: server.port', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "constraint: port must be between 1 and 65535") {
		t.Errorf("expected error message to contain constraint, got: %s", errMsg)
	}
}

func TestNewSchemaParseError(t *testing.T) {
	err := NewSchemaParseError(
		"data.yaml",
		"schema validation failed",
		10,
		"/schemas/config.json",
		"ConfigSchema",
	)

	if err.Kind != ParseErrorKindSchema {
		t.Errorf("expected Kind=%s, got %s", ParseErrorKindSchema, err.Kind)
	}

	if err.Detail.SchemaPath != "/schemas/config.json" {
		t.Errorf("expected SchemaPath='/schemas/config.json', got '%s'", err.Detail.SchemaPath)
	}

	if err.Detail.SchemaName != "ConfigSchema" {
		t.Errorf("expected SchemaName='ConfigSchema', got '%s'", err.Detail.SchemaName)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "schema error") {
		t.Errorf("expected error message to contain 'schema error', got: %s", errMsg)
	}
}

func TestNewEmptyParseError(t *testing.T) {
	err := NewEmptyParseError("empty.yaml")

	if err.Kind != ParseErrorKindEmpty {
		t.Errorf("expected Kind=%s, got %s", ParseErrorKindEmpty, err.Kind)
	}

	if err.FilePath != "empty.yaml" {
		t.Errorf("expected FilePath='empty.yaml', got '%s'", err.FilePath)
	}

	if err.LocationInfo.Line != 0 {
		t.Errorf("expected Line=0 for empty file, got %d", err.LocationInfo.Line)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "empty error") {
		t.Errorf("expected error message to contain 'empty error', got: %s", errMsg)
	}

	if !strings.Contains(errMsg, "file is empty") {
		t.Errorf("expected error message to contain 'file is empty', got: %s", errMsg)
	}
}

// ============================================================================
// EnhancedParseError Interface Tests
// ============================================================================

func TestEnhancedParseErrorYAMLErrorInterface(t *testing.T) {
	tests := []struct {
		name       string
		err        *EnhancedParseError
		wantCode   ErrorCode
		wantType   ErrorType
		wantContext string
	}{
		{
			name: "syntax error",
			err:  NewSyntaxParseError("test.yaml", "bad syntax", 1, 1, ":", "}"),
			wantCode: ErrCodeInvalidSyntax,
			wantType: ErrorTypeSyntax,
			wantContext: "line: 1, column: 1",
		},
		{
			name: "structure error with duplicate key",
			err:  NewStructureParseError("test.yaml", "dup key", 1, "key", "path"),
			wantCode: ErrCodeDuplicateKey,
			wantType: ErrorTypeStructure,
			wantContext: "line: 1, location: path",
		},
		{
			name: "structure error without duplicate key",
			err:  NewStructureParseError("test.yaml", "bad structure", 1, "", "path"),
			wantCode: ErrCodeInvalidStructure,
			wantType: ErrorTypeStructure,
			wantContext: "line: 1, location: path",
		},
		{
			name: "type mismatch error",
			err:  NewTypeMismatchParseError("test.yaml", "type error", 1, "field", "int", "string", "value"),
			wantCode: ErrCodeTypeMismatch,
			wantType: ErrorTypeTypeMismatch,
			wantContext: "line: 1, field: field",
		},
		{
			name: "IO error",
			err:  NewIOParseError("test.yaml", "read error", 0, nil),
			wantCode: ErrCodeFileIOError,
			wantType: ErrorTypeIO,
			wantContext: "",
		},
		{
			name: "validation error",
			err:  NewValidationParseError("test.yaml", "validation failed", 1, "field", "range", "constraint"),
			wantCode: ErrCodeValidationFailed,
			wantType: ErrorTypeValidation,
			wantContext: "line: 1, field: field",
		},
		{
			name: "schema error",
			err:  NewSchemaParseError("test.yaml", "schema error", 1, "/schema.json", "Schema"),
			wantCode: ErrCodeSchemaValidation,
			wantType: ErrorTypeSchema,
			wantContext: "line: 1",
		},
		{
			name: "empty error",
			err:  NewEmptyParseError("test.yaml"),
			wantCode: ErrCodeFileEmpty,
			wantType: ErrorTypeEmpty,
			wantContext: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Code(); got != tt.wantCode {
				t.Errorf("Code() = %v, want %v", got, tt.wantCode)
			}

			if got := tt.err.YAMLErrorType(); got != tt.wantType {
				t.Errorf("YAMLErrorType() = %v, want %v", got, tt.wantType)
			}

			if got := tt.err.Context(); got != tt.wantContext {
				t.Errorf("Context() = %v, want %v", got, tt.wantContext)
			}
		})
	}
}

// ============================================================================
// EnhancedParseError Kind Checking Tests
// ============================================================================

func TestEnhancedParseErrorKindCheckers(t *testing.T) {
	tests := []struct {
		name      string
		err       *EnhancedParseError
		checkFunc func(*EnhancedParseError) bool
		want      bool
	}{
		{"syntax error is syntax", NewSyntaxParseError("test.yaml", "err", 1, 1, "", ""),
			(*EnhancedParseError).IsSyntaxError, true},
		{"syntax error is not structure", NewSyntaxParseError("test.yaml", "err", 1, 1, "", ""),
			(*EnhancedParseError).IsStructureError, false},
		{"structure error is structure", NewStructureParseError("test.yaml", "err", 1, "", ""),
			(*EnhancedParseError).IsStructureError, true},
		{"structure error is not syntax", NewStructureParseError("test.yaml", "err", 1, "", ""),
			(*EnhancedParseError).IsSyntaxError, false},
		{"type mismatch is type mismatch", NewTypeMismatchParseError("test.yaml", "err", 1, "", "", "", ""),
			(*EnhancedParseError).IsTypeMismatchError, true},
		{"IO error is IO", NewIOParseError("test.yaml", "err", 0, nil),
			(*EnhancedParseError).IsIOError, true},
		{"validation error is validation", NewValidationParseError("test.yaml", "err", 1, "", "", ""),
			(*EnhancedParseError).IsValidationError, true},
		{"schema error is schema", NewSchemaParseError("test.yaml", "err", 1, "", ""),
			(*EnhancedParseError).IsSchemaError, true},
		{"empty error is empty", NewEmptyParseError("test.yaml"),
			(*EnhancedParseError).IsEmpty, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.checkFunc(tt.err); got != tt.want {
				t.Errorf("check function returned %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// EnhancedParseError Legacy Conversion Tests
// ============================================================================

func TestEnhancedParseErrorToLegacyConversions(t *testing.T) {
	t.Run("to syntax error", func(t *testing.T) {
		enhanced := NewSyntaxParseError("test.yaml", "bad syntax", 5, 10, ":", "}")
		legacy := enhanced.ToLegacySyntaxError()

		if legacy == nil {
			t.Fatal("expected non-nil SyntaxError")
		}

		if legacy.FilePath != "test.yaml" {
			t.Errorf("expected FilePath='test.yaml', got '%s'", legacy.FilePath)
		}

		if legacy.Line != 5 {
			t.Errorf("expected Line=5, got %d", legacy.Line)
		}

		if legacy.Column != 10 {
			t.Errorf("expected Column=10, got %d", legacy.Column)
		}

		if legacy.Expected != ":" {
			t.Errorf("expected Expected=':', got '%s'", legacy.Expected)
		}

		if legacy.Found != "}" {
			t.Errorf("expected Found='}', got '%s'", legacy.Found)
		}
	})

	t.Run("non-syntax error to syntax error returns nil", func(t *testing.T) {
		enhanced := NewStructureParseError("test.yaml", "err", 1, "", "")
		legacy := enhanced.ToLegacySyntaxError()

		if legacy != nil {
			t.Error("expected nil for non-syntax error conversion")
		}
	})

	t.Run("to structure error", func(t *testing.T) {
		enhanced := NewStructureParseError("test.yaml", "dup key", 15, "port", "server")
		legacy := enhanced.ToLegacyStructureError()

		if legacy == nil {
			t.Fatal("expected non-nil StructureError")
		}

		if legacy.DuplicateKey != "port" {
			t.Errorf("expected DuplicateKey='port', got '%s'", legacy.DuplicateKey)
		}

		if legacy.Location != "server" {
			t.Errorf("expected Location='server', got '%s'", legacy.Location)
		}
	})

	t.Run("to type mismatch error", func(t *testing.T) {
		enhanced := NewTypeMismatchParseError("test.yaml", "type err", 20, "field", "int", "string", "value")
		legacy := enhanced.ToLegacyTypeMismatchError()

		if legacy == nil {
			t.Fatal("expected non-nil TypeMismatchError")
		}

		if legacy.FieldPath != "field" {
			t.Errorf("expected FieldPath='field', got '%s'", legacy.FieldPath)
		}

		if legacy.ExpectedType != "int" {
			t.Errorf("expected ExpectedType='int', got '%s'", legacy.ExpectedType)
		}

		if legacy.ActualType != "string" {
			t.Errorf("expected ActualType='string', got '%s'", legacy.ActualType)
		}
	})

	t.Run("to parse error", func(t *testing.T) {
		enhanced := NewSyntaxParseError("test.yaml", "err", 1, 1, "", "")
		legacy := enhanced.ToLegacyParseError()

		if legacy == nil {
			t.Fatal("expected non-nil ParseError")
		}

		if legacy.FilePath != "test.yaml" {
			t.Errorf("expected FilePath='test.yaml', got '%s'", legacy.FilePath)
		}

		if legacy.Line != 1 {
			t.Errorf("expected Line=1, got %d", legacy.Line)
		}

		if legacy.Column != 1 {
			t.Errorf("expected Column=1, got %d", legacy.Column)
		}
	})
}

// ============================================================================
// EnhancedParseError String Method Tests
// ============================================================================

func TestEnhancedParseErrorString(t *testing.T) {
	t.Run("error with snippet", func(t *testing.T) {
		err := NewSyntaxParseError("test.yaml", "bad syntax", 5, 10, ":", "}")
		err.LocationInfo.Snippet = "  port: 8080"
		err.LocationInfo.Column = 9

		result := err.String()

		if !strings.Contains(result, "syntax error") {
			t.Errorf("expected string to contain 'syntax error'")
		}

		if !strings.Contains(result, "  port: 8080") {
			t.Errorf("expected string to contain snippet")
		}

		if !strings.Contains(result, "^--- here") {
			t.Errorf("expected string to contain column indicator")
		}
	})

	t.Run("error with surrounding lines", func(t *testing.T) {
		err := NewStructureParseError("test.yaml", "dup key", 5, "port", "server")
		err.LocationInfo.SurroundingLines = []string{
			"server:",
			"  host: localhost",
			"  port: 8080",
			"  port: 8081",
			"  ssl: true",
		}
		err.LocationInfo.SnippetLineIndex = 3

		result := err.String()

		if !strings.Contains(result, "structure error") {
			t.Errorf("expected string to contain 'structure error'")
		}

		if !strings.Contains(result, ">   port: 8081") {
			t.Errorf("expected string to highlight error line with '>'")
		}

		if !strings.Contains(result, "server:") {
			t.Errorf("expected string to contain surrounding context")
		}
	})

	t.Run("error without extra context", func(t *testing.T) {
		err := NewEmptyParseError("empty.yaml")

		result := err.String()

		if !strings.Contains(result, "empty error") {
			t.Errorf("expected string to contain 'empty error'")
		}

		// Should not have multiline context
		if strings.Contains(result, "\n\n") {
			t.Error("unexpected blank lines in error without context")
		}
	})
}
