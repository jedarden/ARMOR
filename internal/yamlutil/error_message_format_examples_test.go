// Package yamlutil test file documenting error message formats
//
// This test file demonstrates and verifies all error message formats across the ARMOR codebase.
// It serves as both test coverage and documentation for how errors are formatted and displayed.
//
// Error Message Format Categories
//
// 1. ParseError with line:column
//    - Format: "parse error in <file> at line <line>, column <column>: <message>"
//    - Example: "parse error in config.yaml at line 10, column 5: missing colon"
//
// 2. ValidationError with field paths
//    - Format: "validation error in <file> at line <line>, column <column> at field <field-path>: <message> (constraint: <constraint>)"
//    - Example: "validation error in config.yaml at line 15, column 12 at field server.port: port must be between 1 and 65535 (constraint: must be between 1-65535)"
//
// 3. Type mismatch errors
//    - Format: "type mismatch in <file> at line <line>, field <field-path>: expected <expected>, got <actual>"
//    - Example: "type mismatch in config.yaml at line 8, field server.port: expected integer, got string"

package yamlutil

import (
	"testing"
)

// ============================================================================
// Section 1: ParseError with line:column
// ============================================================================

// TestParseErrorLineColumnFullFormat demonstrates complete ParseError format with all location components
//
// Format: "parse error in <file> at line <line>, column <column>: <message>"
//
// Example output:
//
//	parse error in config.yaml at line 10, column 5: missing colon
func TestParseErrorLineColumnFullFormat(t *testing.T) {
	err := NewParseError("config.yaml", "missing colon", 10, 5, ErrCodeInvalidSyntax, "", "")

	expected := "parse error in config.yaml at line 10, column 5: missing colon"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}

	// Verify format components
	errorMsg := err.Error()
	if !contains(errorMsg, "config.yaml") {
		t.Error("Should include file path")
	}
	if !contains(errorMsg, "at line 10") {
		t.Error("Should include line number")
	}
	if !contains(errorMsg, "column 5") {
		t.Error("Should include column number")
	}
	if !contains(errorMsg, "missing colon") {
		t.Error("Should include error message")
	}
}

// TestParseErrorLineColumnVariations documents different location formatting scenarios
func TestParseErrorLineColumnVariations(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		message  string
		line     int
		column   int
		expected string
	}{
		{
			name:     "file + line + column (most specific)",
			filePath: "config.yaml",
			message:  "test",
			line:     10,
			column:   5,
			expected: "parse error in config.yaml at line 10, column 5: test",
		},
		{
			name:     "file + line only",
			filePath: "config.yaml",
			message:  "test",
			line:     10,
			column:   0,
			expected: "parse error in config.yaml at line 10: test",
		},
		{
			name:     "file only",
			filePath: "config.yaml",
			message:  "test",
			line:     0,
			column:   0,
			expected: "parse error in config.yaml: test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewParseError(tt.filePath, tt.message, tt.line, tt.column, "", "", "")
			if err.Error() != tt.expected {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.expected)
			}
		})
	}
}

// TestParseErrorWithExpectedActual demonstrates ParseError with type information
//
// Format: "parse error in <file> at line <line>, column <column>: <message> (expected: <expected>, actual: <actual>)"
//
// Example output:
//
//	parse error in schema.yaml at line 7, column 12: type mismatch (expected: string, actual: integer)
func TestParseErrorWithExpectedActual(t *testing.T) {
	err := NewParseError("schema.yaml", "type mismatch", 7, 12, ErrCodeTypeMismatch, "string", "integer")

	expected := "parse error in schema.yaml at line 7, column 12: type mismatch (expected: string, actual: integer)"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}

	errorMsg := err.Error()
	if !contains(errorMsg, "expected: string") {
		t.Error("Should include expected type")
	}
	if !contains(errorMsg, "actual: integer") {
		t.Error("Should include actual type")
	}
}

// TestParseErrorAllErrorCodes documents all error code formats
func TestParseErrorAllErrorCodes(t *testing.T) {
	tests := []struct {
		name         string
		code         ErrorCode
		expectedType ErrorType
	}{
		{
			name:         "invalid syntax error",
			code:         ErrCodeInvalidSyntax,
			expectedType: ErrorTypeParse,
		},
		{
			name:         "type mismatch error",
			code:         ErrCodeTypeMismatch,
			expectedType: ErrorTypeParse,
		},
		{
			name:         "required field error",
			code:         ErrCodeRequiredField,
			expectedType: ErrorTypeParse,
		},
		{
			name:         "invalid value error",
			code:         ErrCodeInvalidValue,
			expectedType: ErrorTypeParse,
		},
		{
			name:         "invalid structure error",
			code:         ErrCodeInvalidStructure,
			expectedType: ErrorTypeParse,
		},
		{
			name:         "constraint violation error",
			code:         ErrCodeConstraintViolation,
			expectedType: ErrorTypeParse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewParseError("test.yaml", "test message", 1, 1, tt.code, "", "")
			if err.Code() != tt.code {
				t.Errorf("Code() = %q, want %q", err.Code(), tt.code)
			}
			if err.YAMLErrorType() != tt.expectedType {
				t.Errorf("YAMLErrorType() = %q, want %q", err.YAMLErrorType(), tt.expectedType)
			}
		})
	}
}

// ============================================================================
// Section 2: ValidationError with field paths
// ============================================================================

// TestValidationErrorWithFieldPath demonstrates ValidationError format with field path
//
// Format: "validation error in <file> at field <field-path>: <message> (constraint: <constraint>)"
//
// Example output:
//
//	validation error in config.yaml at field server.port: port must be between 1 and 65535 (constraint: must be between 1-65535)
func TestValidationErrorWithFieldPath(t *testing.T) {
	err := NewValidationError("config.yaml", "port must be between 1 and 65535", "server.port", "must be between 1-65535", ErrCodeInvalidValue, 0, 0, "", "server.port")

	expected := "validation error in config.yaml at field server.port: port must be between 1 and 65535 (constraint: must be between 1-65535)"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}

	errorMsg := err.Error()
	if !contains(errorMsg, "validation error in") {
		t.Error("Should include error type")
	}
	if !contains(errorMsg, "at field server.port") {
		t.Error("Should include field path")
	}
	if !contains(errorMsg, "port must be between 1 and 65535") {
		t.Error("Should include message")
	}
	if !contains(errorMsg, "constraint: must be between 1-65535") {
		t.Error("Should include constraint")
	}
}

// TestValidationErrorNestedFieldPaths documents ValidationError with various field path formats
//
// Field paths can be:
// - Simple: "server.port"
// - Nested: "database.connectionPool.maxConnections"
// - Array access: "servers.api.responses[0].statusCode"
func TestValidationErrorNestedFieldPaths(t *testing.T) {
	testCases := []struct {
		name      string
		fieldPath string
		message   string
		constraint string
	}{
		{
			name:      "simple dot-notation path",
			fieldPath: "server.port",
			message:   "port out of range",
			constraint: "must be between 1-65535",
		},
		{
			name:      "nested dot-notation path",
			fieldPath: "database.connectionPool.maxConnections",
			message:   "too many connections",
			constraint: "must be between 1-100",
		},
		{
			name:      "array access path",
			fieldPath: "servers.api.responses[0].statusCode",
			message:   "invalid status code",
			constraint: "must be between 100-599",
		},
		{
			name:      "deep kubernetes-style path",
			fieldPath: "spec.template.spec.containers[0].image",
			message:   "invalid image tag",
			constraint: "must match registry/*:tag pattern",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("config.yaml", tt.message, tt.fieldPath, tt.constraint, ErrCodeConstraintViolation, 0, 0, "", tt.fieldPath)

			errorMsg := err.Error()

			if !contains(errorMsg, tt.fieldPath) {
				t.Errorf("Should include field path '%s'", tt.fieldPath)
			}
			if !contains(errorMsg, tt.message) {
				t.Errorf("Should include message '%s'", tt.message)
			}
			if !contains(errorMsg, tt.constraint) {
				t.Errorf("Should include constraint '%s'", tt.constraint)
			}
		})
	}
}

// TestValidationErrorWithLineAndColumn demonstrates ValidationError with position information
//
// Format: "validation error in <file> at line <line>, column <column> at field <field-path>: <message> (constraint: <constraint>)"
//
// Example output:
//
//	validation error in app.yaml at line 15, column 12 at field server.port: port out of range (constraint: must be between 1-65535)
func TestValidationErrorWithLineAndColumn(t *testing.T) {
	err := NewValidationError("app.yaml", "port out of range", "server.port", "must be between 1-65535", ErrCodeInvalidValue, 15, 12, ErrorTypeValidation, "server.port")

	expected := "validation error in app.yaml at line 15, column 12 at field server.port: port out of range (constraint: must be between 1-65535)"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}

	errorMsg := err.Error()
	if !contains(errorMsg, "at line 15, column 12") {
		t.Error("Should include line and column")
	}
	if !contains(errorMsg, "at field server.port") {
		t.Error("Should include field path")
	}
}

// TestValidationErrorWithoutFieldPath demonstrates ValidationError without field path
//
// Format: "validation error in <file>: <message>"
//
// Example output:
//
//	validation error in config.yaml: validation failed
func TestValidationErrorWithoutFieldPath(t *testing.T) {
	err := NewValidationError("config.yaml", "validation failed", "", "", ErrCodeValidationFailed, 0, 0, "", "config.yaml")

	expected := "validation error in config.yaml: validation failed"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}

	errorMsg := err.Error()
	if !contains(errorMsg, "validation error in") {
		t.Error("Should include error type")
	}
	if !contains(errorMsg, "validation failed") {
		t.Error("Should include message")
	}
}

// TestValidationErrorComplete demonstrates complete ValidationError with all components
//
// Format breakdown:
// - Location: "<file>:<line>:<column>" or "<file>"
// - Error type: "validation error in"
// - Field path: "at field <field-path>"
// - Message: The specific validation failure
// - Constraint: "(constraint: <details>)"
//
// Example:
//
//	validation error in deployment.yaml at line 22, column 18 at field spec.template.spec.containers[0].image: invalid image tag (constraint: must match registry/*:tag pattern)
func TestValidationErrorComplete(t *testing.T) {
	err := NewValidationError("deployment.yaml", "invalid image tag", "spec.template.spec.containers[0].image", "must match registry/*:tag pattern", ErrCodeInvalidValue, 22, 18, ErrorTypeValidation, "spec.template.spec.containers[0].image")

	errorMsg := err.Error()

	// Verify all components
	if !contains(errorMsg, "deployment.yaml") {
		t.Error("Should include file path")
	}
	if !contains(errorMsg, "at line 22, column 18") {
		t.Error("Should include line and column")
	}
	if !contains(errorMsg, "at field spec.template.spec.containers[0].image") {
		t.Error("Should include field path")
	}
	if !contains(errorMsg, "invalid image tag") {
		t.Error("Should include message")
	}
	if !contains(errorMsg, "constraint: must match registry/*:tag pattern") {
		t.Error("Should include constraint")
	}
}

// ============================================================================
// Section 3: Type mismatch errors
// ============================================================================

// TestTypeMismatchErrorFormat demonstrates type mismatch error format
//
// Format: "type mismatch in <file> at line <line>, field <field-path>: expected <expected>, got <actual>"
//
// Example output:
//
//	type mismatch in config.yaml at line 8, field server.port: expected integer, got string
func TestTypeMismatchErrorFormat(t *testing.T) {
	err := NewTypeMismatchError(
		"config.yaml",
		"server.port",
		"integer",
		"string",
		"8080",
		8,
		"",
	)

	expected := "type mismatch in config.yaml at line 8, field server.port: expected integer, got string"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}

	// Verify format
	errorMsg := err.Error()
	if !contains(errorMsg, "type mismatch in") {
		t.Error("Should include error type")
	}
	if !contains(errorMsg, "config.yaml") {
		t.Error("Should include file path")
	}
	if !contains(errorMsg, "at line 8") {
		t.Error("Should include line number")
	}
	if !contains(errorMsg, "field server.port") {
		t.Error("Should include field path")
	}
	if !contains(errorMsg, "expected integer") {
		t.Error("Should include expected type")
	}
	if !contains(errorMsg, "got string") {
		t.Error("Should include actual type")
	}

	// Verify Context() method
	ctxMsg := err.Context()
	expectedCtx := "field: server.port, expected type: integer, actual type: string, value: 8080"
	if ctxMsg != expectedCtx {
		t.Errorf("Context() = %q, want %q", ctxMsg, expectedCtx)
	}
}

// TestTypeMismatchVariousTypes documents type mismatch errors for various type combinations
//
// Common type mismatches:
// - Expected integer, got string
// - Expected string, got integer
// - Expected boolean, got string
// - Expected array, got scalar
// - Expected object, got null
func TestTypeMismatchVariousTypes(t *testing.T) {
	testCases := []struct {
		name         string
		fieldPath    string
		expectedType string
		actualType   string
		value        string
	}{
		{
			name:         "integer expected, string got",
			fieldPath:    "port",
			expectedType: "integer",
			actualType:   "string",
			value:        "8080",
		},
		{
			name:         "integer expected, boolean got",
			fieldPath:    "timeout",
			expectedType: "integer",
			actualType:   "boolean",
			value:        "true",
		},
		{
			name:         "boolean expected, string got",
			fieldPath:    "enabled",
			expectedType: "boolean",
			actualType:   "string",
			value:        "yes",
		},
		{
			name:         "array expected, string got",
			fieldPath:    "hosts",
			expectedType: "array",
			actualType:   "string",
			value:        "localhost",
		},
		{
			name:         "object expected, null got",
			fieldPath:    "config",
			expectedType: "object",
			actualType:   "null",
			value:        "null",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			err := &TypeMismatchError{
				FilePath:     "config.yaml",
				FieldPath:    tt.fieldPath,
				ExpectedType: tt.expectedType,
				ActualType:   tt.actualType,
				Value:        tt.value,
				Line:         10,
			}

			errorMsg := err.Error()

			if !contains(errorMsg, tt.fieldPath) {
				t.Errorf("Should include field path '%s'", tt.fieldPath)
			}
			if !contains(errorMsg, tt.expectedType) {
				t.Errorf("Should include expected type '%s'", tt.expectedType)
			}
			if !contains(errorMsg, tt.actualType) {
				t.Errorf("Should include actual type '%s'", tt.actualType)
			}

			// Verify error type
			if err.YAMLErrorType() != ErrorTypeTypeMismatch {
				t.Errorf("YAMLErrorType() = %q, want %q", err.YAMLErrorType(), ErrorTypeTypeMismatch)
			}

			// Verify error code
			if err.Code() != ErrCodeTypeMismatch {
				t.Errorf("Code() = %q, want %q", err.Code(), ErrCodeTypeMismatch)
			}
		})
	}
}

// TestTypeMismatchNestedFields demonstrates type mismatch in nested field paths
//
// Example:
//
//	type mismatch in app.yaml at line 35, field servers.api.responses[0].statusCode: expected integer, got string
func TestTypeMismatchNestedFields(t *testing.T) {
	err := &TypeMismatchError{
		FilePath:     "app.yaml",
		FieldPath:    "servers.api.responses[0].statusCode",
		ExpectedType: "integer",
		ActualType:   "string",
		Value:        "\"200\"",
		Line:         35,
	}

	errorMsg := err.Error()

	if !contains(errorMsg, "app.yaml") {
		t.Error("Should include file path")
	}
	if !contains(errorMsg, "at line 35") {
		t.Error("Should include line number")
	}
	if !contains(errorMsg, "field servers.api.responses[0].statusCode") {
		t.Error("Should include nested field path")
	}
	if !contains(errorMsg, "expected integer, got string") {
		t.Error("Should include type information")
	}

	// Verify context
	ctxMsg := err.Context()
	if !contains(ctxMsg, "field: servers.api.responses[0].statusCode") {
		t.Error("Context should include field path")
	}
	if !contains(ctxMsg, "expected type: integer") {
		t.Error("Context should include expected type")
	}
	if !contains(ctxMsg, "actual type: string") {
		t.Error("Context should include actual type")
	}
}

// TestTypeMismatchWithoutLine demonstrates type mismatch error without line number
//
// Format: "type mismatch in <file>, field <field-path>: expected <expected>, got <actual>"
//
// Example output:
//
//	type mismatch in data.yaml, field database.timeout: expected integer, got boolean
func TestTypeMismatchWithoutLine(t *testing.T) {
	err := &TypeMismatchError{
		FilePath:     "data.yaml",
		FieldPath:    "database.timeout",
		ExpectedType: "integer",
		ActualType:   "boolean",
		Value:        "true",
		Line:         0,
	}

	expected := "type mismatch in data.yaml, field database.timeout: expected integer, got boolean"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}

	errorMsg := err.Error()
	if !contains(errorMsg, "type mismatch in data.yaml") {
		t.Error("Should include file path without line number")
	}
	if !contains(errorMsg, "field database.timeout") {
		t.Error("Should include field path")
	}
	if !contains(errorMsg, "expected integer, got boolean") {
		t.Error("Should include type information")
	}
}

// ============================================================================
// Section 4: Real-world error scenarios
// ============================================================================

// TestRealWorldConfigFileError demonstrates a realistic configuration file error scenario
//
// Scenario: Invalid port configuration in service config
//
// config.yaml:
//
//	services:
//	  - name: web
//	    port: "8080"  # ERROR: should be integer, not string
//
// Error output:
//
//	type mismatch in config.yaml at line 5, field services[0].port: expected integer, got string
func TestRealWorldConfigFileError(t *testing.T) {
	err := &TypeMismatchError{
		FilePath:     "config.yaml",
		FieldPath:    "services[0].port",
		ExpectedType: "integer",
		ActualType:   "string",
		Value:        "\"8080\"",
		Line:         5,
	}

	errorMsg := err.Error()

	if !contains(errorMsg, "config.yaml") {
		t.Error("Should include file path")
	}
	if !contains(errorMsg, "at line 5") {
		t.Error("Should include line number")
	}
	if !contains(errorMsg, "field services[0].port") {
		t.Error("Should include field with array index")
	}
	if !contains(errorMsg, "expected integer, got string") {
		t.Error("Should include type mismatch")
	}
}

// TestRealWorldValidationError demonstrates a realistic validation error scenario
//
// Scenario: Port number out of valid range
//
// config.yaml:
//
//	server:
//	  port: 70000  # ERROR: out of range (1-65535)
//
// Error output:
//
//	constraint violation in config.yaml at line 10, field server.port: must be between 1-65535
func TestRealWorldValidationError(t *testing.T) {
	err := &ConstraintError{
		FilePath:       "config.yaml",
		FieldPath:      "server.port",
		ConstraintType: "range",
		Constraint:     "must be between 1-65535",
		Value:          "70000",
		Line:           10,
	}

	errorMsg := err.Error()

	if !contains(errorMsg, "config.yaml") {
		t.Error("Should include file path")
	}
	if !contains(errorMsg, "at line 10") {
		t.Error("Should include line number")
	}
	if !contains(errorMsg, "field server.port") {
		t.Error("Should include field path")
	}
	if !contains(errorMsg, "must be between 1-65535") {
		t.Error("Should include constraint")
	}

	// Verify context
	ctxMsg := err.Context()
	if !contains(ctxMsg, "field: server.port") {
		t.Error("Context should include field path")
	}
	if !contains(ctxMsg, "constraint: must be between 1-65535") {
		t.Error("Context should include constraint")
	}
	if !contains(ctxMsg, "value: 70000") {
		t.Error("Context should include actual value")
	}
}

// TestRealWorldFieldNotFoundError demonstrates a realistic missing field error scenario
//
// Scenario: Required field is missing
//
// config.yaml:
//
//	database:
//	  host: localhost
//	  # ERROR: missing required field 'port'
//
// Error output:
//
//	required field missing in config.yaml at line 8: database.port
func TestRealWorldFieldNotFoundError(t *testing.T) {
	err := NewFieldNotFoundError("config.yaml", "database.port", 8, "")

	errorMsg := err.Error()

	if !contains(errorMsg, "required field missing") {
		t.Error("Should include error type")
	}
	if !contains(errorMsg, "config.yaml") {
		t.Error("Should include file path")
	}
	if !contains(errorMsg, "at line 8") {
		t.Error("Should include line number")
	}
	if !contains(errorMsg, "database.port") {
		t.Error("Should include field path")
	}

	// Verify context
	ctxMsg := err.Context()
	if !contains(ctxMsg, "required field not found: database.port") {
		t.Error("Context should include field path")
	}

	// Verify error type
	if err.YAMLErrorType() != ErrorTypeFieldNotFound {
		t.Errorf("YAMLErrorType() = %q, want %q", err.YAMLErrorType(), ErrorTypeFieldNotFound)
	}
}

// ============================================================================
// Section 5: Error message format consistency
// ============================================================================

// TestErrorFormatConsistency verifies that error formats follow consistent patterns
//
// Consistency rules:
// 1. File path always present if available
// 2. Position (line, column) follows file path with "at line <line>, column <column>" format
// 3. Field paths use "at field <path>" or "field <path>:" format
// 4. Type mismatches use "expected <expected>, got <actual>" format
// 5. Constraints use "(constraint: <details>)" or "constraint: <details>" format
func TestErrorFormatConsistency(t *testing.T) {
	// Test ParseError format consistency
	parseErr := NewParseError("config.yaml", "test", 10, 5, "", "", "")
	if !contains(parseErr.Error(), "parse error in") {
		t.Error("ParseError should start with 'parse error in'")
	}
	if !contains(parseErr.Error(), "at line 10, column 5") {
		t.Error("ParseError should include position with 'at line X, column Y'")
	}

	// Test ValidationError format consistency
	validationErr := NewValidationError("config.yaml", "test", "server.port", "constraint", "", 10, 5, "", "server.port")
	if !contains(validationErr.Error(), "validation error in") {
		t.Error("ValidationError should start with 'validation error in'")
	}
	if !contains(validationErr.Error(), "at field server.port") {
		t.Error("ValidationError should include 'at field <path>'")
	}

	// Test TypeMismatchError format consistency
	typeErr := &TypeMismatchError{
		FilePath:     "config.yaml",
		FieldPath:    "server.port",
		ExpectedType: "integer",
		ActualType:   "string",
		Line:         10,
	}
	if !contains(typeErr.Error(), "type mismatch in") {
		t.Error("TypeMismatchError should start with 'type mismatch in'")
	}
	if !contains(typeErr.Error(), "expected integer, got string") {
		t.Error("TypeMismatchError should use 'expected X, got Y' format")
	}

	// Test ConstraintError format consistency
	constraintErr := &ConstraintError{
		FilePath:       "config.yaml",
		FieldPath:      "server.port",
		Constraint:     "must be between 1-65535",
		Line:           10,
	}
	if !contains(constraintErr.Error(), "constraint violation in") {
		t.Error("ConstraintError should start with 'constraint violation in'")
	}
	if !contains(constraintErr.Error(), "must be between 1-65535") {
		t.Error("ConstraintError should include constraint")
	}
}

// TestErrorContextFormat verifies that Context() methods provide structured information
func TestErrorContextFormat(t *testing.T) {
	// Test TypeMismatchError Context()
	typeErr := &TypeMismatchError{
		FieldPath:    "server.port",
		ExpectedType: "integer",
		ActualType:   "string",
		Value:        "8080",
	}
	ctx := typeErr.Context()
	if !contains(ctx, "field: server.port") {
		t.Error("Context should include 'field: <path>'")
	}
	if !contains(ctx, "expected type: integer") {
		t.Error("Context should include 'expected type: <type>'")
	}
	if !contains(ctx, "actual type: string") {
		t.Error("Context should include 'actual type: <type>'")
	}
	if !contains(ctx, "value: 8080") {
		t.Error("Context should include 'value: <value>'")
	}

	// Test ConstraintError Context()
	constraintErr := &ConstraintError{
		FieldPath:  "server.port",
		Constraint: "must be between 1-65535",
		Value:      "70000",
	}
	ctx = constraintErr.Context()
	if !contains(ctx, "field: server.port") {
		t.Error("Context should include 'field: <path>'")
	}
	if !contains(ctx, "constraint: must be between 1-65535") {
		t.Error("Context should include 'constraint: <constraint>'")
	}
	if !contains(ctx, "value: 70000") {
		t.Error("Context should include 'value: <value>'")
	}

	// Test FieldNotFoundError Context()
	fieldErr := NewFieldNotFoundError("config.yaml", "database.host", 5, "")
	ctx = fieldErr.Context()
	if !contains(ctx, "required field not found: database.host") {
		t.Error("Context should include 'required field not found: <path>'")
	}
}

// TestErrorRecognition verifies that error types are correctly recognized
func TestErrorRecognition(t *testing.T) {
	// Test ParseError recognition
	parseErr := NewParseError("test.yaml", "test", 1, 1, "", "", "")
	if !IsParseError(parseErr) {
		t.Error("Should recognize ParseError")
	}
	if !IsYAMLError(parseErr) {
		t.Error("ParseError should be a YAMLError")
	}

	// Test ValidationError recognition
	validationErr := NewValidationError("test.yaml", "test", "field", "constraint", "", 1, 1, "", "field")
	if !IsValidationError(validationErr) {
		t.Error("Should recognize ValidationError")
	}
	if !IsYAMLError(validationErr) {
		t.Error("ValidationError should be a YAMLError")
	}

	// Test TypeMismatchError recognition
	typeErr := &TypeMismatchError{FilePath: "test.yaml"}
	if !IsYAMLError(typeErr) {
		t.Error("TypeMismatchError should be a YAMLError")
	}
	if typeErr.YAMLErrorType() != ErrorTypeTypeMismatch {
		t.Error("TypeMismatchError should have type TypeMismatch")
	}

	// Test ConstraintError recognition
	constraintErr := &ConstraintError{FilePath: "test.yaml"}
	if !IsYAMLError(constraintErr) {
		t.Error("ConstraintError should be a YAMLError")
	}
	if constraintErr.YAMLErrorType() != ErrorTypeConstraint {
		t.Error("ConstraintError should have type Constraint")
	}

	// Test FieldNotFoundError recognition
	fieldErr := NewFieldNotFoundError("test.yaml", "field", 1, "")
	if !IsYAMLError(fieldErr) {
		t.Error("FieldNotFoundError should be a YAMLError")
	}
	if fieldErr.YAMLErrorType() != ErrorTypeFieldNotFound {
		t.Error("FieldNotFoundError should have type FieldNotFound")
	}
}

// TestErrorCodes verifies that error codes are properly set
func TestErrorCodes(t *testing.T) {
	tests := []struct {
		name         string
		err          YAMLError
		wantCode     ErrorCode
		wantType     ErrorType
	}{
		{
			name:     "ParseError with invalid syntax",
			err:      NewParseError("test.yaml", "test", 1, 1, ErrCodeInvalidSyntax, "", ""),
			wantCode: ErrCodeInvalidSyntax,
			wantType: ErrorTypeParse,
		},
		{
			name:     "ValidationError with invalid value",
			err:      NewValidationError("test.yaml", "test", "field", "constraint", ErrCodeInvalidValue, 1, 1, "", "field"),
			wantCode: ErrCodeInvalidValue,
			wantType: ErrorTypeValidation,
		},
		{
			name:     "TypeMismatchError",
			err:      &TypeMismatchError{FilePath: "test.yaml"},
			wantCode: ErrCodeTypeMismatch,
			wantType: ErrorTypeTypeMismatch,
		},
		{
			name:     "ConstraintError",
			err:      &ConstraintError{FilePath: "test.yaml"},
			wantCode: ErrCodeConstraintViolation,
			wantType: ErrorTypeConstraint,
		},
		{
			name:     "FieldNotFoundError",
			err:      NewFieldNotFoundError("test.yaml", "field", 1, ""),
			wantCode: ErrCodeRequiredField,
			wantType: ErrorTypeFieldNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code() != tt.wantCode {
				t.Errorf("Code() = %q, want %q", tt.err.Code(), tt.wantCode)
			}
			if tt.err.YAMLErrorType() != tt.wantType {
				t.Errorf("YAMLErrorType() = %q, want %q", tt.err.YAMLErrorType(), tt.wantType)
			}
		})
	}
}
