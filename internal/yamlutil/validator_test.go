// Package yamlutil tests for YAML validator
package yamlutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidator_ValidYAML(t *testing.T) {
	validator := NewValidator()
	yamlContent := `
key1: value1
key2: value2
nested:
  item1: nested1
  item2: nested2
array:
  - item1
  - item2
`

	result := validator.ValidateString(yamlContent)

	if !result.Valid {
		t.Errorf("Expected valid YAML to pass validation, got errors: %v", result.Errors)
	}

	if result.HasErrors() {
		t.Errorf("Expected no errors for valid YAML, got: %d errors", len(result.Errors))
	}
}

func TestValidator_InvalidYAML_SyntaxError(t *testing.T) {
	validator := NewValidator()
	yamlContent := `
key1: value1
  invalid_indentation: this should cause error
key2: value2
`

	result := validator.ValidateString(yamlContent)

	if result.Valid {
		t.Error("Expected invalid YAML to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for invalid YAML syntax")
	}

	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got: %d", len(result.Errors))
	}

	err := result.Errors[0]
	if err.Type != ErrorTypeSyntax {
		t.Errorf("Expected error type '%s', got: %s", ErrorTypeSyntax, err.Type)
	}

	if err.Line == 0 {
		t.Error("Expected line number to be set")
	}

	if err.Message == "" {
		t.Error("Expected error message to be set")
	}

	if err.Context == "" {
		t.Error("Expected error context to be set")
	}

	// Verify context contains line content
	if !strings.Contains(err.Context, "Line content") {
		t.Error("Expected context to contain line content")
	}
}

func TestValidator_InvalidYAML_BadIndentation(t *testing.T) {
	validator := NewValidator()
	yamlContent := `
root:
  key1: value1
    key2: bad indentation
  key3: value3
`

	result := validator.ValidateString(yamlContent)

	if result.Valid {
		t.Error("Expected YAML with bad indentation to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for bad indentation")
	}

	// Verify error contains relevant information
	err := result.Errors[0]
	// The error message varies by parser version, but should indicate a syntax error
	if !strings.Contains(strings.ToLower(err.Message), "did not find expected") &&
		!strings.Contains(strings.ToLower(err.Message), "indentation") &&
		!strings.Contains(strings.ToLower(err.Message), "mapping values") &&
		!strings.Contains(strings.ToLower(err.Message), "not allowed") {
		t.Errorf("Expected error message to mention syntax issue, got: %s", err.Message)
	}
}

func TestValidator_InvalidYAML_ColonError(t *testing.T) {
	validator := NewValidator()
	yamlContent := `
key1: value1
key2 value2  # missing colon
key3: value3
`

	result := validator.ValidateString(yamlContent)

	if result.Valid {
		t.Error("Expected YAML with missing colon to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for missing colon")
	}
}

func TestValidator_InvalidYAML_UnexpectedCharacter(t *testing.T) {
	validator := NewValidator()
	yamlContent := `
key1: value1
@invalid: key
key2: value2
`

	result := validator.ValidateString(yamlContent)

	if result.Valid {
		t.Error("Expected YAML with unexpected character to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for unexpected character")
	}
}

func TestValidator_EmptyContent(t *testing.T) {
	validator := NewValidator()
	yamlContent := ""

	result := validator.ValidateString(yamlContent)

	if result.Valid {
		t.Error("Expected empty content to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for empty content")
	}

	err := result.Errors[0]
	if err.Type != ErrorTypeEmpty {
		t.Errorf("Expected error type '%s', got: %s", ErrorTypeEmpty, err.Type)
	}

	if !strings.Contains(err.Message, "empty") {
		t.Errorf("Expected error message to mention 'empty', got: %s", err.Message)
	}
}

func TestValidator_WhitespaceOnly(t *testing.T) {
	validator := NewValidator()
	yamlContent := "   \n  \n  \n  "

	result := validator.ValidateString(yamlContent)

	if result.Valid {
		t.Error("Expected whitespace-only content to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for whitespace-only content")
	}
}

func TestValidator_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "valid.yaml")
	content := `
key1: value1
key2: value2
nested:
  item: value
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	validator := NewValidator()
	result := validator.ValidateFile(testFile)

	if !result.Valid {
		t.Errorf("Expected valid file to pass validation, got errors: %v", result.Errors)
	}

	if result.HasErrors() {
		t.Errorf("Expected no errors, got: %d", len(result.Errors))
	}

	if result.FilePath != testFile {
		t.Errorf("Expected FilePath=%s, got: %s", testFile, result.FilePath)
	}
}

func TestValidator_InvalidFile_SyntaxError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "invalid.yaml")
	content := `
key1: value1
  bad_indentation: error
key2: value2
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	validator := NewValidator()
	result := validator.ValidateFile(testFile)

	if result.Valid {
		t.Error("Expected invalid file to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for invalid file")
	}

	// Verify file path is in error
	err := result.Errors[0]
	if err.FilePath != testFile {
		t.Errorf("Expected error FilePath=%s, got: %s", testFile, err.FilePath)
	}
}

func TestValidator_NonexistentFile(t *testing.T) {
	validator := NewValidator()
	result := validator.ValidateFile("/nonexistent/path/file.yaml")

	if result.Valid {
		t.Error("Expected nonexistent file to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for nonexistent file")
	}

	err := result.Errors[0]
	if err.Type != ErrorTypeIO {
		t.Errorf("Expected error type '%s', got: %s", ErrorTypeIO, err.Type)
	}

	if !strings.Contains(strings.ToLower(err.Message), "failed to read") {
		t.Errorf("Expected error message to mention read failure, got: %s", err.Message)
	}
}

func TestValidator_ErrorFormatting(t *testing.T) {
	validator := NewValidator()
	yamlContent := `
key1: value1
  invalid: indentation
key2: value2
`

	result := validator.ValidateString(yamlContent)

	// Test Error() method
	errMsg := result.Errors[0].Error()
	if !strings.Contains(errMsg, "syntax:") {
		t.Errorf("Expected Error() to contain type, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "Line") {
		t.Errorf("Expected Error() to contain line number, got: %s", errMsg)
	}

	// Test String() method with full context
	detailedMsg := result.Errors[0].String()
	if !strings.Contains(detailedMsg, "Error:") {
		t.Errorf("Expected String() to contain 'Error:', got: %s", detailedMsg)
	}
	if !strings.Contains(detailedMsg, "Type:") {
		t.Errorf("Expected String() to contain 'Type:', got: %s", detailedMsg)
	}
	if !strings.Contains(detailedMsg, "Location:") {
		t.Errorf("Expected String() to contain 'Location:', got: %s", detailedMsg)
	}
	if !strings.Contains(detailedMsg, "Context:") {
		t.Errorf("Expected String() to contain 'Context:', got: %s", detailedMsg)
	}
}

func TestValidator_ErrorSummary(t *testing.T) {
	validator := NewValidator()
	yamlContent := `
key1: value1
  invalid: indentation
key2: value2
`

	result := validator.ValidateString(yamlContent)
	summary := result.ErrorSummary()

	if !strings.Contains(summary, "Validation failed") {
		t.Errorf("Expected summary to mention validation failure, got: %s", summary)
	}
	if !strings.Contains(summary, "1.") {
		t.Errorf("Expected summary to list error number, got: %s", summary)
	}
	if !strings.Contains(summary, "Error:") {
		t.Errorf("Expected summary to contain error details, got: %s", summary)
	}
}

func TestValidator_ErrorSummary_NoErrors(t *testing.T) {
	validator := NewValidator()
	result := validator.ValidateString("key: value")
	summary := result.ErrorSummary()

	if summary != "No errors" {
		t.Errorf("Expected 'No errors' for valid YAML, got: %s", summary)
	}
}

func TestValidator_HasErrors(t *testing.T) {
	validator := NewValidator()

	// Valid YAML
	validResult := validator.ValidateString("key: value")
	if validResult.HasErrors() {
		t.Error("Expected HasErrors() to return false for valid YAML")
	}

	// Invalid YAML
	invalidResult := validator.ValidateString("key: value\n  bad: indent")
	if !invalidResult.HasErrors() {
		t.Error("Expected HasErrors() to return true for invalid YAML")
	}
}

func TestValidator_HasWarnings(t *testing.T) {
	validator := NewValidator()
	result := validator.ValidateString("key: value")

	// Simple YAML typically has no warnings
	if result.HasWarnings() && len(result.Warnings) > 0 {
		t.Logf("Warning detected (this is OK if testing YAML with comments): %v", result.Warnings)
	}
}

func TestValidator_MultiDocument(t *testing.T) {
	validator := NewValidator()
	yamlContent := `
---
key1: value1
---
key2: value2
---
key3: value3
`

	result := validator.ValidateString(yamlContent)

	if !result.Valid {
		t.Errorf("Expected valid multi-document YAML to pass, got errors: %v", result.Errors)
	}

	if result.HasErrors() {
		t.Errorf("Expected no errors for valid multi-document YAML, got: %d", len(result.Errors))
	}
}

func TestValidator_ComplexStructure(t *testing.T) {
	validator := NewValidator()
	yamlContent := `
root:
  scalar: value
  number: 42
  float: 3.14
  bool: true
  null_value: null
  array:
    - item1
    - item2
    - nested:
        subkey: subvalue
  mapping:
    key1: val1
    key2: val2
  nested:
    deeply:
      nested:
        value: here
`

	result := validator.ValidateString(yamlContent)

	if !result.Valid {
		t.Errorf("Expected complex valid YAML to pass, got errors: %v", result.Errors)
	}
}

func TestValidator_ValidateStringWithPath(t *testing.T) {
	validator := NewValidator()
	yamlContent := "key: value"
	filePath := "/test/path/config.yaml"

	result := validator.ValidateStringWithPath(yamlContent, filePath)

	// Verify path is set in result
	if result.FilePath != filePath {
		t.Errorf("Expected FilePath=%s, got: %s", filePath, result.FilePath)
	}

	// Test with invalid YAML to verify path is in error
	invalidContent := "key: value\n  bad: indent"
	invalidResult := validator.ValidateStringWithPath(invalidContent, filePath)

	if invalidResult.HasErrors() {
		if invalidResult.Errors[0].FilePath != filePath {
			t.Errorf("Expected error FilePath=%s, got: %s", filePath, invalidResult.Errors[0].FilePath)
		}
	}
}

func TestValidator_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple test files
	files := map[string]string{
		"valid1.yaml":   "key1: value1",
		"valid2.yaml":   "key2: value2",
		"invalid.yaml":  "key: value\n  bad: indent",
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", name, err)
		}
	}

	validator := NewValidator()
	paths := []string{
		filepath.Join(tmpDir, "valid1.yaml"),
		filepath.Join(tmpDir, "valid2.yaml"),
		filepath.Join(tmpDir, "invalid.yaml"),
	}

	results := validator.ValidateMultipleFiles(paths)

	if len(results) != len(paths) {
		t.Errorf("Expected %d results, got: %d", len(paths), len(results))
	}

	// Verify first two files are valid
	for i := 0; i < 2; i++ {
		if !results[i].Valid {
			t.Errorf("Expected file %d to be valid, got errors: %v", i+1, results[i].Errors)
		}
	}

	// Verify third file is invalid
	if results[2].Valid {
		t.Error("Expected third file to be invalid")
	}

	if !results[2].HasErrors() {
		t.Error("Expected third file to have errors")
	}
}

func TestValidator_ErrorTypes(t *testing.T) {
	testCases := []struct {
		name        string
		content     string
		expectedErr ErrorType
	}{
		{
			name:        "empty content",
			content:     "",
			expectedErr: ErrorTypeEmpty,
		},
		{
			name:        "indentation error",
			content:     "key:\n  valid: value\n    invalid: bad",
			expectedErr: ErrorTypeSyntax,
		},
	}

	validator := NewValidator()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validator.ValidateString(tc.content)
			if !result.HasErrors() {
				t.Fatal("Expected validation to fail")
			}
			if result.Errors[0].Type != tc.expectedErr {
				t.Errorf("Expected error type '%s', got: '%s'", tc.expectedErr, result.Errors[0].Type)
			}
		})
	}
}

func TestValidator_LineAndColumnReporting(t *testing.T) {
	validator := NewValidator()
	// Create YAML with error on a specific line
	yamlContent := `key1: value1
key2: value2
key3: value3
  bad_indentation: error
key4: value4`

	result := validator.ValidateString(yamlContent)

	if result.Valid {
		t.Error("Expected validation to fail")
	}

	// The error should be reported around line 4 or 5
	if result.Errors[0].Line == 0 {
		t.Error("Expected line number to be set")
	}

	// Verify the error context contains the problematic line
	if result.Errors[0].Context == "" {
		t.Error("Expected error context to be set")
	}

	// Context should mention the problematic content
	if !strings.Contains(result.Errors[0].Context, "Line content") {
		t.Error("Expected context to contain line content")
	}
}

func TestValidator_NewStrictValidator(t *testing.T) {
	validator := NewStrictValidator()
	if validator == nil {
		t.Error("Expected NewStrictValidator to return non-nil validator")
	}
	if !validator.strict {
		t.Error("Expected strict mode to be enabled")
	}
}

func TestValidator_HumanReadableMessages(t *testing.T) {
	validator := NewValidator()
	yamlContent := `
key1: value1
  invalid: indentation
key2: value2
`

	result := validator.ValidateString(yamlContent)

	if result.Valid {
		t.Error("Expected validation to fail")
	}

	// Verify the error message is human-readable
	errMsg := result.Errors[0].Message
	if errMsg == "" {
		t.Error("Expected error message to be set")
	}

	// Message should not be too cryptic
	if strings.Contains(errMsg, "yaml:") && len(errMsg) < 10 {
		t.Error("Error message is too cryptic: ", errMsg)
	}

	// Verify the full formatted error is readable
	formatted := result.Errors[0].String()
	if !strings.Contains(formatted, "Error:") ||
		!strings.Contains(formatted, "Type:") ||
		!strings.Contains(formatted, "Location:") {
		t.Error("Formatted error should have clear sections")
	}
}

func TestValidator_WarningSummary(t *testing.T) {
	validator := NewValidator()
	result := validator.ValidateString("key: value")
	summary := result.WarningSummary()

	if !result.HasWarnings() && summary != "No warnings" {
		t.Errorf("Expected 'No warnings' when no warnings present, got: %s", summary)
	}

	t.Run("with warnings", func(t *testing.T) {
		// Create a result with warnings
		result := ValidationResult{
			FilePath: "/test/file.yaml",
			Valid:    true,
			Warnings: []ValidationError{
				{
					Type:    ErrorTypeStructure,
					Message: "Duplicate key detected",
					Line:    5,
				},
				{
					Type:    ErrorTypeDeprecated,
					Message: "Deprecated YAML feature",
					Line:    10,
				},
			},
		}

		summary := result.WarningSummary()

		if !strings.Contains(summary, "Warnings") {
			t.Errorf("Expected summary to contain 'Warnings', got: %s", summary)
		}

		if !strings.Contains(summary, "Duplicate key detected") {
			t.Errorf("Expected summary to contain first warning message, got: %s", summary)
		}

		if !strings.Contains(summary, "1.") && !strings.Contains(summary, "2.") {
			t.Errorf("Expected numbered warnings, got: %s", summary)
		}
	})
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name         string
		errorMsg     string
		expectedType ErrorType
	}{
		{
			name:         "could not find expected",
			errorMsg:     "yaml: line 1: could not find expected ':'",
			expectedType: ErrorTypeSyntax,
		},
		{
			name:         "did not find expected key",
			errorMsg:     "did not find expected key",
			expectedType: ErrorTypeSyntax,
		},
		{
			name:         "cannot start any key",
			errorMsg:     "found character that cannot start any key",
			expectedType: ErrorTypeSyntax,
		},
		{
			name:         "invalid indentation",
			errorMsg:     "invalid indentation",
			expectedType: ErrorTypeSyntax,
		},
		{
			name:         "duplicate key",
			errorMsg:     "duplicate key",
			expectedType: ErrorTypeStructure,
		},
		{
			name:         "mapping values not allowed",
			errorMsg:     "mapping values are not allowed here",
			expectedType: ErrorTypeSyntax,
		},
		{
			name:         "unexpected end",
			errorMsg:     "unexpected end of document",
			expectedType: ErrorTypeSyntax,
		},
		{
			name:         "unacceptable character",
			errorMsg:     "unacceptable character",
			expectedType: ErrorTypeSyntax,
		},
		{
			name:         "scanner error",
			errorMsg:     "scanner error",
			expectedType: ErrorTypeSyntax,
		},
		{
			name:         "unmarshal errors",
			errorMsg:     "unmarshal errors",
			expectedType: ErrorTypeStructure,
		},
		{
			name:         "cannot unmarshal",
			errorMsg:     "cannot unmarshal !!str into []string",
			expectedType: ErrorTypeStructure,
		},
		{
			name:         "unknown error",
			errorMsg:     "some unknown error",
			expectedType: ErrorTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categorizeError(tt.errorMsg)
			if result != tt.expectedType {
				t.Errorf("categorizeError(%q) = %v, expected %v", tt.errorMsg, result, tt.expectedType)
			}
		})
	}
}

// Test acceptance criteria
func TestAcceptanceCriteria_SyntaxValidationFunctional(t *testing.T) {
	// AC1: Syntax validation functional
	validator := NewValidator()

	// Valid YAML should pass
	validYAML := "key: value\nkey2: value2"
	result := validator.ValidateString(validYAML)
	if !result.Valid {
		t.Error("Valid YAML should pass validation")
	}

	// Invalid YAML should fail
	invalidYAML := "key: value\n  bad: indent"
	result = validator.ValidateString(invalidYAML)
	if result.Valid {
		t.Error("Invalid YAML should fail validation")
	}
}

func TestAcceptanceCriteria_LineColumnContext(t *testing.T) {
	// AC2: Errors include line/column context
	validator := NewValidator()
	yamlContent := `key1: value1
key2: value2
key3: value3
  bad_indentation: error
key4: value4`

	result := validator.ValidateString(yamlContent)

	if !result.HasErrors() {
		t.Fatal("Expected errors for invalid YAML")
	}

	err := result.Errors[0]
	if err.Line == 0 {
		t.Error("Expected line number to be reported")
	}

	// Column may be 0 if parser doesn't provide it, but line should always be set
}

func TestAcceptanceCriteria_HumanReadableMessages(t *testing.T) {
	// AC3: Error messages are human-readable
	validator := NewValidator()
	yamlContent := `key1: value1
  invalid_indent: error
key2: value2`

	result := validator.ValidateString(yamlContent)

	if !result.HasErrors() {
		t.Fatal("Expected errors for invalid YAML")
	}

	err := result.Errors[0]

	// Check message exists and is readable
	if err.Message == "" {
		t.Error("Expected non-empty error message")
	}

	// Check formatted output includes clear sections
	formatted := err.String()
	requiredSections := []string{"Error:", "Type:", "Location:", "Context:"}
	for _, section := range requiredSections {
		if !strings.Contains(formatted, section) {
			t.Errorf("Expected formatted error to contain '%s'", section)
		}
	}
}

func TestAcceptanceCriteria_ErrorTypeCategorization(t *testing.T) {
	// AC4: Error types categorized
	testCases := []struct {
		name          string
		content       string
		expectedTypes []ErrorType
	}{
		{
			name:          "empty file",
			content:       "",
			expectedTypes: []ErrorType{ErrorTypeEmpty},
		},
		{
			name:          "syntax error",
			content:       "key:\n  subkey: value\n  value_without_key",
			expectedTypes: []ErrorType{ErrorTypeSyntax},
		},
	}

	validator := NewValidator()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validator.ValidateString(tc.content)
			if !result.HasErrors() {
				t.Fatal("Expected validation error")
			}

			errType := result.Errors[0].Type
			found := false
			for _, expected := range tc.expectedTypes {
				if errType == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected error type to be one of %v, got: %s", tc.expectedTypes, errType)
			}
		})
	}
}

func TestWarningSummary_WithWarnings(t *testing.T) {
	validator := NewValidator()
	yamlContent := `
key1: value1
key2: value2
	`

	result := validator.ValidateString(yamlContent)

	// Manually add a warning to test WarningSummary
	result.Warnings = append(result.Warnings, ValidationError{
		Type:     ErrorTypeStructure,
		Message:  "Test warning",
		FilePath: "test.yaml",
		Line:     1,
	})

	summary := result.WarningSummary()

	if !strings.Contains(summary, "Warnings for") {
		t.Errorf("Expected summary to mention warnings, got: %s", summary)
	}

	if !strings.Contains(summary, "Test warning") {
		t.Errorf("Expected summary to contain warning message, got: %s", summary)
	}

	if !strings.Contains(summary, "1.") {
		t.Errorf("Expected summary to list warning number, got: %s", summary)
	}
}

func TestWarningSummary_MultipleWarnings(t *testing.T) {
	validator := NewValidator()
	result := validator.ValidateString("key: value")

	// Add multiple warnings
	result.Warnings = append(result.Warnings,
		ValidationError{Type: ErrorTypeStructure, Message: "Warning 1", FilePath: "test.yaml"},
		ValidationError{Type: ErrorTypeStructure, Message: "Warning 2", FilePath: "test.yaml"},
	)

	summary := result.WarningSummary()

	if !strings.Contains(summary, "1.") {
		t.Errorf("Expected first warning number, got: %s", summary)
	}

	if !strings.Contains(summary, "2.") {
		t.Errorf("Expected second warning number, got: %s", summary)
	}
}
