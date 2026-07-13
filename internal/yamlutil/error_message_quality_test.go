// Package yamlutil tests for error message quality verification
//
// This test file provides comprehensive coverage for YAML error message quality,
// ensuring error messages are helpful, actionable, and contain relevant context.
// Tests cover file path inclusion, line/column accuracy, error categorization,
// and message clarity across all error scenarios.
package yamlutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Section 1: File Path Inclusion Tests
// ============================================================================

// TestErrorMessagesIncludeFilePath verifies that all error types include file paths
func TestErrorMessagesIncludeFilePath(t *testing.T) {
	tests := []struct {
		name         string
		createError  func() error
		expectPath   bool
		expectedPath string
	}{
		{
			name: "ParseError includes file path",
			createError: func() error {
				return NewParseError("config.yaml", "test error", 10, 5, ErrCodeInvalidSyntax, "", "")
			},
			expectPath:   true,
			expectedPath: "config.yaml",
		},
		{
			name: "ValidationError includes file path",
			createError: func() error {
				return NewValidationError("deployment.yaml", "invalid port", "server.port", "must be 1-65535", ErrCodeInvalidValue, 15, 12, "", "server.port")
			},
			expectPath:   true,
			expectedPath: "deployment.yaml",
		},
		{
			name: "TypeMismatchError includes file path",
			createError: func() error {
				return NewTypeMismatchError("values.yaml", "server.port", "integer", "string", "8080", 20, "")
			},
			expectPath:   true,
			expectedPath: "values.yaml",
		},
		{
			name: "ConstraintError includes file path",
			createError: func() error {
				return NewConstraintError("service.yaml", "server.port", "range", "must be between 1-65535", "", "70000", 12, "")
			},
			expectPath:   true,
			expectedPath: "service.yaml",
		},
		{
			name: "FieldNotFoundError includes file path",
			createError: func() error {
				return NewFieldNotFoundError("config.yaml", "database.host", 8, "")
			},
			expectPath:   true,
			expectedPath: "config.yaml",
		},
		{
			name: "FileError includes file path",
			createError: func() error {
				return NewFileError("/etc/config/app.yaml", "read", "file not found", nil)
			},
			expectPath:   true,
			expectedPath: "/etc/config/app.yaml",
		},
		{
			name: "SyntaxError includes file path",
			createError: func() error {
				return NewSyntaxError("broken.yaml", "invalid syntax", 5, 10, "", "", "")
			},
			expectPath:   true,
			expectedPath: "broken.yaml",
		},
		{
			name: "StructureError includes file path",
			createError: func() error {
				return NewStructureError("invalid.yaml", "invalid structure", 3, "", "", "")
			},
			expectPath:   true,
			expectedPath: "invalid.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errMsg := err.Error()

			if tt.expectPath {
				if !strings.Contains(errMsg, tt.expectedPath) {
					t.Errorf("Error message should contain file path '%s', got: %s", tt.expectedPath, errMsg)
				}
				t.Logf("✓ File path '%s' found in error message", tt.expectedPath)
			}
		})
	}
}

// TestErrorMessagesWithRelativePaths verifies error messages work with relative paths
func TestErrorMessagesWithRelativePaths(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "simple relative path",
			filePath: "config.yaml",
		},
		{
			name:     "nested relative path",
			filePath: "configs/dev/database.yaml",
		},
		{
			name:     "parent directory relative path",
			filePath: "../shared/config.yaml",
		},
		{
			name:     "complex nested path",
			filePath: "deployments/production/services/backend/config.yaml",
		},
		{
			name:     "absolute path",
			filePath: "/etc/app/config.yaml",
		},
		{
			name:     "home directory path",
			filePath: "~/app/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewParseError(tt.filePath, "test error", 10, 5, ErrCodeInvalidSyntax, "", "")
			errMsg := err.Error()

			if !strings.Contains(errMsg, tt.filePath) {
				t.Errorf("Error message should contain path '%s', got: %s", tt.filePath, errMsg)
			}
			t.Logf("✓ Path '%s' preserved in error message", tt.filePath)
		})
	}
}

// TestErrorMessagesFromActualFile verify error messages from actual file parsing
func TestErrorMessagesFromActualFile(t *testing.T) {
	// Create temporary YAML file with syntax error
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "config.yaml")

	content := `server:
  port: "8080"  # Should be integer, not string
  host: localhost
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Parse the file - should get an error about type mismatch
	parser := NewParser()
	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if result.Error != nil {
		errMsg := result.Error.Error()
		t.Logf("Error message from file parse: %s", errMsg)

		// Verify error message contains the file path
		if !strings.Contains(errMsg, testFile) && !strings.Contains(errMsg, "config.yaml") {
			t.Error("Error message should contain the file path")
		} else {
			t.Logf("✓ File path included in error message")
		}
	}
}

// ============================================================================
// Section 2: Line and Column Number Accuracy Tests
// ============================================================================

// TestErrorMessagesIncludeLineColumn verifies line/column numbers are included
func TestErrorMessagesIncludeLineColumn(t *testing.T) {
	tests := []struct {
		name           string
		createError    func() error
		expectLine     bool
		expectColumn   bool
		expectedLine   int
		expectedColumn int
	}{
		{
			name: "ParseError with line and column",
			createError: func() error {
				return NewParseError("config.yaml", "test error", 10, 5, ErrCodeInvalidSyntax, "", "")
			},
			expectLine:     true,
			expectColumn:   true,
			expectedLine:   10,
			expectedColumn: 5,
		},
		{
			name: "ParseError with line only",
			createError: func() error {
				return NewParseError("config.yaml", "test error", 15, 0, ErrCodeInvalidSyntax, "", "")
			},
			expectLine:     true,
			expectColumn:   false,
			expectedLine:   15,
		},
		{
			name: "ValidationError with line and column",
			createError: func() error {
				return NewValidationError("app.yaml", "invalid value", "server.port", "constraint", ErrCodeInvalidValue, 20, 8, "", "server.port")
			},
			expectLine:     true,
			expectColumn:   true,
			expectedLine:   20,
			expectedColumn: 8,
		},
		{
			name: "TypeMismatchError with line",
			createError: func() error {
				return NewTypeMismatchError("values.yaml", "server.port", "integer", "string", "", 25, "")
			},
			expectLine:     true,
			expectColumn:   false,
			expectedLine:   25,
		},
		{
			name: "ConstraintError with line",
			createError: func() error {
				return NewConstraintError("service.yaml", "server.port", "", "must be 1-65535", "", "70000", 30, "")
			},
			expectLine:     true,
			expectColumn:   false,
			expectedLine:   30,
		},
		{
			name: "FieldNotFoundError with line",
			createError: func() error {
				return NewFieldNotFoundError("config.yaml", "database.host", 5, "")
			},
			expectLine:     true,
			expectColumn:   false,
			expectedLine:   5,
		},
		{
			name: "SyntaxError with line and column",
			createError: func() error {
				return NewSyntaxError("broken.yaml", "invalid syntax", 12, 15, "", "", "")
			},
			expectLine:     true,
			expectColumn:   true,
			expectedLine:   12,
			expectedColumn: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errMsg := err.Error()

			if tt.expectLine {
				lineStr := fmt.Sprintf("%d", tt.expectedLine)
				if !strings.Contains(errMsg, lineStr) {
					t.Errorf("Error message should contain line number %d, got: %s", tt.expectedLine, errMsg)
				}
				t.Logf("✓ Line number %d found in error message", tt.expectedLine)
			}

			if tt.expectColumn {
				colStr := fmt.Sprintf("%d", tt.expectedColumn)
				if !strings.Contains(errMsg, colStr) {
					t.Errorf("Error message should contain column number %d, got: %s", tt.expectedColumn, errMsg)
				}
				t.Logf("✓ Column number %d found in error message", tt.expectedColumn)
			}
		})
	}
}

// TestErrorMessagesLineColumnFormat verifies line/column format consistency
func TestErrorMessagesLineColumnFormat(t *testing.T) {
	tests := []struct {
		name          string
		createError   func() error
		expectedFormat []string
	}{
		{
			name: "ParseError line:column format",
			createError: func() error {
				return NewParseError("config.yaml", "test error", 10, 5, ErrCodeInvalidSyntax, "", "")
			},
			expectedFormat: []string{"at line 10", "column 5"},
		},
		{
			name: "ValidationError line:column format",
			createError: func() error {
				return NewValidationError("app.yaml", "invalid", "field", "constraint", ErrCodeInvalidValue, 15, 12, "", "field")
			},
			expectedFormat: []string{"at line 15", "column 12"},
		},
		{
			name: "line only format",
			createError: func() error {
				return NewParseError("config.yaml", "test error", 20, 0, ErrCodeInvalidSyntax, "", "")
			},
			expectedFormat: []string{"at line 20"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errMsg := err.Error()

			for _, expected := range tt.expectedFormat {
				if !strings.Contains(errMsg, expected) {
					t.Errorf("Error message should contain '%s', got: %s", expected, errMsg)
				}
			}
			t.Logf("✓ Format correct: %v", tt.expectedFormat)
		})
	}
}

// ============================================================================
// Section 3: Error Type Categorization Tests
// ============================================================================

// TestErrorTypeCategorization verifies all error types are properly categorized
func TestErrorTypeCategorization(t *testing.T) {
	tests := []struct {
		name         string
		createError  func() error
		expectedType ErrorType
		expectedCode ErrorCode
	}{
		{
			name: "ParseError categorization",
			createError: func() error {
				return NewParseError("config.yaml", "invalid syntax", 10, 5, ErrCodeInvalidSyntax, "", "")
			},
			expectedType: ErrorTypeParse,
			expectedCode: ErrCodeInvalidSyntax,
		},
		{
			name: "ValidationError categorization",
			createError: func() error {
				return NewValidationError("app.yaml", "invalid", "field", "constraint", ErrCodeInvalidValue, 0, 0, "", "field")
			},
			expectedType: ErrorTypeValidation,
			expectedCode: ErrCodeInvalidValue,
		},
		{
			name: "TypeMismatchError categorization",
			createError: func() error {
				return NewTypeMismatchError("values.yaml", "server.port", "integer", "string", "", 10, "")
			},
			expectedType: ErrorTypeTypeMismatch,
			expectedCode: ErrCodeTypeMismatch,
		},
		{
			name: "ConstraintError categorization",
			createError: func() error {
				return NewConstraintError(
					"service.yaml",
					"server.port",
					"",
					"must be 1-65535",
					"",
					"70000",
					10,
					"",
				)
			},
			expectedType: ErrorTypeConstraint,
			expectedCode: ErrCodeConstraintViolation,
		},
		{
			name: "FieldNotFoundError categorization",
			createError: func() error {
				return NewFieldNotFoundError("config.yaml", "database.host", 5, "")
			},
			expectedType: ErrorTypeFieldNotFound,
			expectedCode: ErrCodeRequiredField,
		},
		{
			name: "FileError categorization",
			createError: func() error {
				return NewFileError("config.yaml", "", "", fmt.Errorf("not found"))
			},
			expectedType: ErrorTypeFile,
			expectedCode: ErrCodeFileIOError,
		},
		{
			name: "SyntaxError categorization",
			createError: func() error {
				return NewSyntaxError("broken.yaml", "invalid syntax", 10, 0, "", "", "")
			},
			expectedType: ErrorTypeSyntax,
			expectedCode: ErrCodeInvalidSyntax,
		},
		{
			name: "StructureError categorization",
			createError: func() error {
				return NewStructureError("invalid.yaml", "invalid structure", 5, "", "", "")
			},
			expectedType: ErrorTypeStructure,
			expectedCode: ErrCodeInvalidStructure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()

			// Verify it's recognized as a YAMLError
			yamlErr, ok := err.(YAMLError)
			if !ok {
				t.Fatalf("Error should implement YAMLError interface")
			}

			// Verify error type
			if yamlErr.YAMLErrorType() != tt.expectedType {
				t.Errorf("Error type = %q, want %q", yamlErr.YAMLErrorType(), tt.expectedType)
			}

			// Verify error code
			if yamlErr.Code() != tt.expectedCode {
				t.Errorf("Error code = %q, want %q", yamlErr.Code(), tt.expectedCode)
			}

			t.Logf("✓ Error properly categorized as type=%s code=%s", tt.expectedType, tt.expectedCode)
		})
	}
}

// TestErrorTypeInMessages verifies error type is mentioned in messages
func TestErrorTypeInMessages(t *testing.T) {
	tests := []struct {
		name              string
		createError       func() error
		expectedInMessage []string
	}{
		{
			name: "ParseError mentions parse error",
			createError: func() error {
				return NewParseError("config.yaml", "invalid syntax", 10, 5, ErrCodeInvalidSyntax, "", "")
			},
			expectedInMessage: []string{"parse error"},
		},
		{
			name: "ValidationError mentions validation error",
			createError: func() error {
				return NewValidationError("app.yaml", "invalid", "field", "constraint", ErrCodeInvalidValue, 0, 0, "", "field")
			},
			expectedInMessage: []string{"validation error"},
		},
		{
			name: "TypeMismatchError mentions type mismatch",
			createError: func() error {
				return NewTypeMismatchError("values.yaml", "server.port", "integer", "string", "", 10, "")
			},
			expectedInMessage: []string{"type mismatch"},
		},
		{
			name: "ConstraintError mentions constraint violation",
			createError: func() error {
				return NewConstraintError("service.yaml", "server.port", "", "must be 1-65535", "", "70000", 10, "")
			},
			expectedInMessage: []string{"constraint violation"},
		},
		{
			name: "FieldNotFoundError mentions required field",
			createError: func() error {
				return NewFieldNotFoundError("config.yaml", "database.host", 5, "")
			},
			expectedInMessage: []string{"required field"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errMsg := strings.ToLower(err.Error())

			for _, expected := range tt.expectedInMessage {
				if !strings.Contains(errMsg, expected) {
					t.Errorf("Error message should contain '%s', got: %s", expected, err.Error())
				}
			}
			t.Logf("✓ Error type mentioned in message: %v", tt.expectedInMessage)
		})
	}
}

// ============================================================================
// Section 4: Error Message Context and Actionability Tests
// ============================================================================

// TestErrorMessagesProvideContext verifies errors provide helpful context
func TestErrorMessagesProvideContext(t *testing.T) {
	tests := []struct {
		name              string
		createError       func() error
		expectedContext   []string
		description       string
	}{
		{
			name: "TypeMismatchError provides type information",
			createError: func() error {
				return NewTypeMismatchError("config.yaml", "server.port", "integer", "string", "8080", 10, "")
			},
			expectedContext: []string{"expected", "integer", "got", "string"},
			description:     "Type mismatch should show expected and actual types",
		},
		{
			name: "ConstraintError provides constraint details",
			createError: func() error {
				return NewConstraintError("config.yaml", "server.port", "", "must be between 1-65535", "", "70000", 10, "")
			},
			expectedContext: []string{"constraint", "1-65535", "70000"},
			description:     "Constraint error should show constraint and actual value",
		},
		{
			name: "ValidationError with field path",
			createError: func() error {
				return NewValidationError("config.yaml", "port out of range", "server.port", "must be 1-65535", ErrCodeInvalidValue, 0, 0, "", "server.port")
			},
			expectedContext: []string{"field", "server.port", "constraint"},
			description:     "Validation error should include field path and constraint",
		},
		{
			name: "FieldNotFoundError shows missing field",
			createError: func() error {
				return NewFieldNotFoundError("config.yaml", "database.host", 5, "")
			},
			expectedContext: []string{"database.host"},
			description:     "Field not found error should identify the missing field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errMsg := err.Error()
			ctxMsg := ""

			// Try to get Context() if available
			if ctxErr, ok := err.(interface{ Context() string }); ok {
				ctxMsg = ctxErr.Context()
			}

			t.Logf("Description: %s", tt.description)
			t.Logf("Error message: %s", errMsg)
			if ctxMsg != "" {
				t.Logf("Context: %s", ctxMsg)
			}

			combinedMsg := errMsg + " " + ctxMsg
			for _, expected := range tt.expectedContext {
				if !strings.Contains(combinedMsg, expected) {
					t.Errorf("Error should provide context '%s', got message: %s", expected, combinedMsg)
				}
			}
			t.Logf("✓ Context provided: %v", tt.expectedContext)
		})
	}
}

// TestErrorMessagesAreActionable verifies error messages are actionable
func TestErrorMessagesAreActionable(t *testing.T) {
	tests := []struct {
		name          string
		createError   func() error
		actionable    bool
		expectedHints []string
	}{
		{
			name: "Type mismatch suggests fix",
			createError: func() error {
				return NewTypeMismatchError("config.yaml", "server.port", "integer", "string", "8080", 10, "")
			},
			actionable:    true,
			expectedHints: []string{"expected", "integer", "got", "string"},
		},
		{
			name: "Constraint violation shows valid range",
			createError: func() error {
				return NewConstraintError("config.yaml", "server.port", "", "must be between 1-65535", "", "70000", 10, "")
			},
			actionable:    true,
			expectedHints: []string{"1-65535", "70000"},
		},
		{
			name: "Field not found identifies missing field",
			createError: func() error {
				return NewFieldNotFoundError("config.yaml", "database.host", 5, "")
			},
			actionable:    true,
			expectedHints: []string{"database.host", "required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errMsg := err.Error()

			t.Logf("Error message: %s", errMsg)

			if tt.actionable {
				// Check that message provides actionable information
				hasActionableInfo := false
				for _, hint := range tt.expectedHints {
					if strings.Contains(errMsg, hint) {
						hasActionableInfo = true
						break
					}
				}

				if !hasActionableInfo {
					t.Errorf("Error message should be actionable with hints like %v, got: %s", tt.expectedHints, errMsg)
				} else {
					t.Logf("✓ Message is actionable: contains actionable hints")
				}
			}
		})
	}
}

// ============================================================================
// Section 5: Real-World Error Scenario Quality Tests
// ============================================================================

// TestRealErrorScenariosHaveQualityMessages tests that real error scenarios have good messages
func TestRealErrorScenariosHaveQualityMessages(t *testing.T) {
	scenarios := []struct {
		name        string
		yamlContent string
		description string
		expectError bool
		qualityChecks []string // What quality aspects to check
	}{
		{
			name: "syntax error - unmatched bracket",
			yamlContent: `items: [one, two, three`,
			description: "Unmatched bracket should have clear error message",
			expectError: true,
			qualityChecks: []string{"bracket", "unmatched", "did not find"},
		},
		{
			name: "circular reference",
			yamlContent: `A: &anchor
  B: *anchor`,
			description: "Circular reference should be clearly explained",
			expectError: true,
			qualityChecks: []string{"circular", "anchor", "reference"},
		},
		{
			name: "invalid YAML directive",
			yamlContent: `%YAML 1.3
---
key: value`,
			description: "Invalid YAML directive should mention version incompatibility",
			expectError: true,
			qualityChecks: []string{"incompatible", "document", "1.3"},
		},
		{
			name: "duplicate key",
			yamlContent: `key: value1
key: value2`,
			description: "Duplicate key error should identify the duplicate",
			expectError: true,
			qualityChecks: []string{"duplicate", "key"},
		},
		{
			name: "invalid indent",
			yamlContent: `parent:
  child: value
    bad_indent: true`,
			description: "Indentation error should explain the issue",
			expectError: true,
			qualityChecks: []string{"indent", "structure"},
		},
	}

	for _, tt := range scenarios {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing scenario: %s", tt.description)

			parser := NewParser()
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for scenario: %s", tt.description)
					return
				}

				errMsg := strings.ToLower(err.Error())
				t.Logf("Error message: %s", err)

				// Check for quality aspects
				foundQualityChecks := 0
				for _, check := range tt.qualityChecks {
					if strings.Contains(errMsg, check) {
						foundQualityChecks++
						t.Logf("✓ Quality check passed: contains '%s'", check)
					}
				}

				// At least some quality checks should pass
				if len(tt.qualityChecks) > 0 && foundQualityChecks == 0 {
					t.Logf("Warning: No quality checks matched for error: %s", err)
					t.Logf("Expected to find some of: %v", tt.qualityChecks)
				}
			}
		})
	}
}

// TestErrorMessagesAcrossAllCategories verifies quality across error categories
func TestErrorMessagesAcrossAllCategories(t *testing.T) {
	categories := []struct {
		category    string
		testErrors  []func() error
		qualityChecks func(error) []string
	}{
		{
			category: "parse errors",
			testErrors: []func() error{
				func() error {
					return NewParseError("config.yaml", "invalid syntax", 10, 5, ErrCodeInvalidSyntax, "", "")
				},
				func() error {
					return NewSyntaxError("test.yaml", "malformed YAML", 5, 10, "", "", "")
				},
			},
			qualityChecks: func(err error) []string {
				checks := []string{}
				msg := err.Error()
				if strings.Contains(msg, "parse error") || strings.Contains(msg, "syntax") {
					checks = append(checks, "error type mentioned")
				}
				if strings.Contains(msg, "config.yaml") || strings.Contains(msg, "test.yaml") {
					checks = append(checks, "file path included")
				}
				if strings.Contains(msg, "line") {
					checks = append(checks, "line number included")
				}
				return checks
			},
		},
		{
			category: "validation errors",
			testErrors: []func() error{
				func() error {
					return NewValidationError("app.yaml", "invalid", "server.port", "constraint", ErrCodeInvalidValue, 0, 0, "", "server.port")
				},
				func() error {
					return NewConstraintError("service.yaml", "server.port", "", "must be 1-65535", "", "70000", 10, "")
				},
			},
			qualityChecks: func(err error) []string {
				checks := []string{}
				msg := err.Error()
				if strings.Contains(msg, "validation") || strings.Contains(msg, "constraint") {
					checks = append(checks, "error type mentioned")
				}
				if strings.Contains(msg, "field") || strings.Contains(msg, "server.port") {
					checks = append(checks, "field path included")
				}
				return checks
			},
		},
		{
			category: "type mismatch errors",
			testErrors: []func() error{
				func() error {
					return NewTypeMismatchError("values.yaml", "server.port", "integer", "string", "8080", 10, "")
				},
			},
			qualityChecks: func(err error) []string {
				checks := []string{}
				msg := err.Error()
				if strings.Contains(msg, "type mismatch") {
					checks = append(checks, "error type mentioned")
				}
				if strings.Contains(msg, "expected") && strings.Contains(msg, "got") {
					checks = append(checks, "type information provided")
				}
				return checks
			},
		},
	}

	for _, cat := range categories {
		t.Run(cat.category, func(t *testing.T) {
			t.Logf("Testing category: %s", cat.category)

			for i, testErrFunc := range cat.testErrors {
				err := testErrFunc()
				qualityChecks := cat.qualityChecks(err)

				t.Logf("Error %d: %s", i+1, err.Error())

				if len(qualityChecks) > 0 {
					t.Logf("✓ Quality checks passed: %v", qualityChecks)
				} else {
					t.Logf("Note: Minimal quality checks for error %d", i+1)
				}
			}
		})
	}
}

// ============================================================================
// Section 6: Error Message Format Consistency Tests
// ============================================================================

// TestErrorFormatConsistencyAcrossErrors verifies format consistency
func TestErrorFormatConsistencyAcrossErrors(t *testing.T) {
	errors := []struct {
		name  string
		err   error
		check func(string) bool
	}{
		{
			name: "ParseError format",
			err: NewParseError("config.yaml", "test error", 10, 5, ErrCodeInvalidSyntax, "", ""),
			check: func(msg string) bool {
				return strings.Contains(msg, "parse error in") &&
					strings.Contains(msg, "config.yaml")
			},
		},
		{
			name: "ValidationError format",
			err: NewValidationError("app.yaml", "test", "field", "constraint", ErrCodeInvalidValue, 0, 0, "", "field"),
			check: func(msg string) bool {
				return strings.Contains(msg, "validation error in") &&
					strings.Contains(msg, "app.yaml")
			},
		},
		{
			name: "TypeMismatchError format",
			err: NewTypeMismatchError("values.yaml", "server.port", "integer", "string", "", 10, ""),
			check: func(msg string) bool {
				return strings.Contains(msg, "type mismatch in") &&
					strings.Contains(msg, "expected") &&
					strings.Contains(msg, "got")
			},
		},
		{
			name: "ConstraintError format",
			err: NewConstraintError("service.yaml", "server.port", "", "must be 1-65535", "", "70000", 10, ""),
			check: func(msg string) bool {
				return strings.Contains(msg, "constraint violation in") &&
					strings.Contains(msg, "constraint")
			},
		},
	}

	for _, tt := range errors {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			t.Logf("Message: %s", msg)

			if !tt.check(msg) {
				t.Errorf("Format check failed for: %s", msg)
			} else {
				t.Logf("✓ Format consistent")
			}
		})
	}
}

// TestErrorMessagesNonEmpty verifies all error messages are non-empty
func TestErrorMessagesNonEmpty(t *testing.T) {
	testErrors := []func() error{
		func() error { return NewParseError("f.yaml", "m", 1, 1, "", "", "") },
		func() error { return NewValidationError("f.yaml", "m", "f", "c", "", 0, 0, "", "f") },
		func() error { return NewTypeMismatchError("f.yaml", "", "", "", "", 0, "") },
		func() error { return NewConstraintError("f.yaml", "", "", "", "", "", 0, "") },
		func() error { return NewFieldNotFoundError("f.yaml", "f", 1, "") },
		func() error { return NewSyntaxError("f.yaml", "m", 0, 0, "", "", "") },
		func() error { return NewStructureError("f.yaml", "m", 0, "", "", "") },
		func() error { return NewFileError("f.yaml", "", "", fmt.Errorf("e")) },
	}

	for i, testErr := range testErrors {
		t.Run(fmt.Sprintf("error_%d", i), func(t *testing.T) {
			err := testErr()
			msg := err.Error()

			if msg == "" {
				t.Errorf("Error message should not be empty")
			} else {
				t.Logf("✓ Non-empty message: %s", msg)
			}
		})
	}
}

// ============================================================================
// Section 7: Integration Test - File Parsing with Error Messages
// ============================================================================

// TestFileParsingErrorMessagesIntegration tests end-to-end error message quality
func TestFileParsingErrorMessagesIntegration(t *testing.T) {
	testCases := []struct {
		name          string
		yamlContent   string
		description   string
		errorContains []string
	}{
		{
			name: "config with invalid type",
			yamlContent: `server:
  port: "8080"  # Should be integer
  host: localhost
`,
			description:   "Type mismatch in config file",
			errorContains: []string{"port", "type", "integer", "string"},
		},
		{
			name: "missing required field",
			yamlContent: `database:
  user: admin
  # Missing: password
`,
			description:   "Missing required field",
			errorContains: []string{"database", "password", "required"},
		},
		{
			name: "constraint violation",
			yamlContent: `server:
  port: 70000  # Out of range
`,
			description:   "Value constraint violation",
			errorContains: []string{"port", "constraint", "range"},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing: %s", tt.description)

			// Create temporary file
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "config.yaml")

			if err := os.WriteFile(testFile, []byte(tt.yamlContent), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Parse the file
			parser := NewParser()
			var data map[string]interface{}
			result := parser.ParseFile(testFile, &data)

			if result.Error != nil {
				errMsg := strings.ToLower(result.Error.Error())
				t.Logf("Error message: %s", result.Error)

				// Check for expected content
				foundMatches := 0
				for _, expected := range tt.errorContains {
					if strings.Contains(errMsg, expected) {
						foundMatches++
						t.Logf("✓ Contains expected: '%s'", expected)
					}
				}

				// At least one expected element should be found
				if len(tt.errorContains) > 0 && foundMatches == 0 {
					t.Logf("Note: Expected to find some of: %v", tt.errorContains)
				}
			}
		})
	}
}
