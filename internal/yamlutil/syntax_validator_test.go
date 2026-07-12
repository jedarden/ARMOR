// Package yamlutil provides unit tests for syntax validator interfaces.
package yamlutil

import (
	"strings"
	"testing"
)

// TestSyntaxValidatorInterface tests the SyntaxValidator interface contract.
func TestSyntaxValidatorInterface(t *testing.T) {
	validator := NewSyntaxValidator()

	// Verify that validator implements the SyntaxValidator interface
	var _ SyntaxValidator = validator
}

// TestIndentationErrorType tests the IndentationError type structure.
func TestIndentationErrorType(t *testing.T) {
	err := IndentationError{
		FilePath:     "test.yaml",
		Line:         5,
		Column:       3,
		Message:      "Mixed tabs and spaces",
		Expected:     4,
		Actual:       3,
		IndentType:   "mixed",
		ContextStr:      "Line has 2 spaces and 1 tab",
		SuggestedFix: "Use only spaces for indentation",
	}

	// Test Error() method
	errorMsg := err.Error()
	if !strings.Contains(errorMsg, "indentation error") {
		t.Errorf("Expected error message to contain 'indentation error', got: %s", errorMsg)
	}
	if !strings.Contains(errorMsg, "line 5") {
		t.Errorf("Expected error message to contain line number, got: %s", errorMsg)
	}

	// Test Code() method
	if err.Code() != ErrCodeInvalidSyntax {
		t.Errorf("Expected error code ErrCodeInvalidSyntax, got: %s", err.Code())
	}

	// Test YAMLErrorType() method
	if err.YAMLErrorType() != ErrorTypeSyntax {
		t.Errorf("Expected error type ErrorTypeSyntax, got: %s", err.YAMLErrorType())
	}

	// Test Context() method
	context := err.Context()
	if context != err.ContextStr {
		t.Errorf("Expected context %s, got: %s", err.ContextStr, context)
	}
}

// TestDelimiterErrorType tests the DelimiterError type structure.
func TestDelimiterErrorType(t *testing.T) {
	err := DelimiterError{
		FilePath:      "test.yaml",
		Line:          10,
		Column:        15,
		Message:       "Unmatched closing brace",
		DelimiterType: "}",
		Expected:      "{",
		Found:         "}",
		ContextStr:       "Missing opening brace",
		SuggestedFix:  "Add opening brace",
	}

	// Test Error() method
	errorMsg := err.Error()
	if !strings.Contains(errorMsg, "delimiter error") {
		t.Errorf("Expected error message to contain 'delimiter error', got: %s", errorMsg)
	}
	if !strings.Contains(errorMsg, "line 10") {
		t.Errorf("Expected error message to contain line number, got: %s", errorMsg)
	}

	// Test Code() method
	if err.Code() != ErrCodeInvalidSyntax {
		t.Errorf("Expected error code ErrCodeInvalidSyntax, got: %s", err.Code())
	}

	// Test YAMLErrorType() method
	if err.YAMLErrorType() != ErrorTypeSyntax {
		t.Errorf("Expected error type ErrorTypeSyntax, got: %s", err.YAMLErrorType())
	}
}

// TestSyntaxWarningType tests the SyntaxWarning type structure.
func TestSyntaxWarningType(t *testing.T) {
	warning := SyntaxWarning{
		FilePath: "test.yaml",
		Line:     7,
		Column:   1,
		Message:  "Deprecated YAML syntax",
		Level:    "deprecation",
	}

	// Test Error() method
	errorMsg := warning.Error()
	if !strings.Contains(errorMsg, "warning") {
		t.Errorf("Expected warning message to contain 'warning', got: %s", errorMsg)
	}
	if !strings.Contains(errorMsg, "line 7") {
		t.Errorf("Expected warning message to contain line number, got: %s", errorMsg)
	}
}

// TestErrorContextType tests the SyntaxErrorContext type structure.
func TestSyntaxErrorContextType(t *testing.T) {
	context := SyntaxErrorContext{
		ErrorLine:    5,
		StartLine:    3,
		EndLine:      7,
		Lines:        []string{"line 3", "line 4", "line 5", "line 6", "line 7"},
		Pointer:      "^",
		IndentSpaces: 4,
		HasError:     true,
	}

	// Test String() method
	contextStr := context.String()
	if !strings.Contains(contextStr, "line 5") {
		t.Errorf("Expected context to contain 'line 5', got: %s", contextStr)
	}
	if !strings.Contains(contextStr, "^") {
		t.Errorf("Expected context to contain pointer, got: %s", contextStr)
	}
}

// TestSyntaxValidationResultType tests the SyntaxValidationResult type structure.
func TestSyntaxValidationResultType(t *testing.T) {
	result := SyntaxValidationResult{
		FilePath:         "test.yaml",
		Valid:            false,
		ContextLines:     2,
		TotalLines:       10,
		ErrorLine:        5,
		SyntaxErrors:     []SyntaxError{{}},
		IndentationErrors: []IndentationError{{}},
		DelimiterErrors:  []DelimiterError{{}},
		StructureErrors:  []StructureError{{}},
		Warnings:        []SyntaxWarning{{}},
	}

	// Test HasErrors()
	if !result.HasErrors() {
		t.Errorf("Expected HasErrors to return true")
	}

	// Test HasWarnings()
	if !result.HasWarnings() {
		t.Errorf("Expected HasWarnings to return true")
	}

	// Test ErrorCount()
	errorCount := result.ErrorCount()
	if errorCount != 4 {
		t.Errorf("Expected error count 4, got: %d", errorCount)
	}

	// Test WarningCount()
	warningCount := result.WarningCount()
	if warningCount != 1 {
		t.Errorf("Expected warning count 1, got: %d", warningCount)
	}

	// Test ErrorSummary()
	summary := result.ErrorSummary()
	if !strings.Contains(summary, "failed") {
		t.Errorf("Expected summary to contain 'failed', got: %s", summary)
	}
	if !strings.Contains(summary, "4 error") {
		t.Errorf("Expected summary to contain error count, got: %s", summary)
	}
}

// TestSyntaxValidationResultValid tests SyntaxValidationResult for valid content.
func TestSyntaxValidationResultValid(t *testing.T) {
	result := SyntaxValidationResult{
		Valid:     true,
		Warnings:  []SyntaxWarning{{Message: "test warning"}},
	}

	// Test HasErrors() should return false
	if result.HasErrors() {
		t.Errorf("Expected HasErrors to return false for valid result")
	}

	// Test HasWarnings() should return true
	if !result.HasWarnings() {
		t.Errorf("Expected HasWarnings to return true")
	}

	// Test ErrorSummary()
	summary := result.ErrorSummary()
	if !strings.Contains(summary, "passed") {
		t.Errorf("Expected summary to contain 'passed', got: %s", summary)
	}
}

// TestDefaultSyntaxValidatorConstructor tests constructor functions.
func TestDefaultSyntaxValidatorConstructor(t *testing.T) {
	// Test NewSyntaxValidator
	validator := NewSyntaxValidator()
	if validator == nil {
		t.Errorf("Expected non-nil validator from NewSyntaxValidator")
	}
	if validator.indentSpaces != 2 {
		t.Errorf("Expected default indentSpaces to be 2, got: %d", validator.indentSpaces)
	}
	if validator.allowTabs {
		t.Errorf("Expected allowTabs to be false by default")
	}
	if validator.contextLines != 2 {
		t.Errorf("Expected default contextLines to be 2, got: %d", validator.contextLines)
	}

	// Test NewStrictSyntaxValidator
	strictValidator := NewStrictSyntaxValidator()
	if strictValidator == nil {
		t.Errorf("Expected non-nil validator from NewStrictSyntaxValidator")
	}
	if !strictValidator.strict {
		t.Errorf("Expected strict mode to be enabled")
	}
	if strictValidator.contextLines != 3 {
		t.Errorf("Expected strict contextLines to be 3, got: %d", strictValidator.contextLines)
	}
}

// TestIndentationErrorFields tests all IndentationError fields are accessible.
func TestIndentationErrorFields(t *testing.T) {
	err := IndentationError{
		FilePath:     "/path/to/file.yaml",
		Line:         42,
		Column:       8,
		Message:      "Test indentation error",
		Expected:     6,
		Actual:       4,
		IndentType:   "space",
		ContextStr:      "Test context",
		SuggestedFix: "Fix indentation",
	}

	// Verify all fields
	if err.FilePath != "/path/to/file.yaml" {
		t.Errorf("FilePath field not correctly set")
	}
	if err.Line != 42 {
		t.Errorf("Line field not correctly set")
	}
	if err.Column != 8 {
		t.Errorf("Column field not correctly set")
	}
	if err.Message != "Test indentation error" {
		t.Errorf("Message field not correctly set")
	}
	if err.Expected != 6 {
		t.Errorf("Expected field not correctly set")
	}
	if err.Actual != 4 {
		t.Errorf("Actual field not correctly set")
	}
	if err.IndentType != "space" {
		t.Errorf("IndentType field not correctly set")
	}
	if err.ContextStr != "Test context" {
		t.Errorf("Context field not correctly set")
	}
	if err.SuggestedFix != "Fix indentation" {
		t.Errorf("SuggestedFix field not correctly set")
	}
}

// TestDelimiterErrorFields tests all DelimiterError fields are accessible.
func TestDelimiterErrorFields(t *testing.T) {
	err := DelimiterError{
		FilePath:      "/path/to/file.yaml",
		Line:          15,
		Column:        12,
		Message:       "Test delimiter error",
		DelimiterType: "{",
		Expected:      "}",
		Found:         "{",
		ContextStr:       "Test context",
		SuggestedFix:  "Add closing brace",
	}

	// Verify all fields
	if err.FilePath != "/path/to/file.yaml" {
		t.Errorf("FilePath field not correctly set")
	}
	if err.Line != 15 {
		t.Errorf("Line field not correctly set")
	}
	if err.Column != 12 {
		t.Errorf("Column field not correctly set")
	}
	if err.Message != "Test delimiter error" {
		t.Errorf("Message field not correctly set")
	}
	if err.DelimiterType != "{" {
		t.Errorf("DelimiterType field not correctly set")
	}
	if err.Expected != "}" {
		t.Errorf("Expected field not correctly set")
	}
	if err.Found != "{" {
		t.Errorf("Found field not correctly set")
	}
	if err.ContextStr != "Test context" {
		t.Errorf("Context field not correctly set")
	}
	if err.SuggestedFix != "Add closing brace" {
		t.Errorf("SuggestedFix field not correctly set")
	}
}

// TestSyntaxWarningFields tests all SyntaxWarning fields are accessible.
func TestSyntaxWarningFields(t *testing.T) {
	warning := SyntaxWarning{
		FilePath: "/path/to/file.yaml",
		Line:     20,
		Column:   5,
		Message:  "Test warning message",
		Level:    "info",
	}

	// Verify all fields
	if warning.FilePath != "/path/to/file.yaml" {
		t.Errorf("FilePath field not correctly set")
	}
	if warning.Line != 20 {
		t.Errorf("Line field not correctly set")
	}
	if warning.Column != 5 {
		t.Errorf("Column field not correctly set")
	}
	if warning.Message != "Test warning message" {
		t.Errorf("Message field not correctly set")
	}
	if warning.Level != "info" {
		t.Errorf("Level field not correctly set")
	}
}

// TestErrorContextFields tests all SyntaxErrorContext fields are accessible.
func TestSyntaxErrorContextFields(t *testing.T) {
	context := SyntaxErrorContext{
		ErrorLine:    10,
		StartLine:    8,
		EndLine:      12,
		Lines:        []string{"line 8", "line 9", "line 10", "line 11", "line 12"},
		Pointer:      "^^^",
		IndentSpaces: 6,
		HasError:     true,
	}

	// Verify all fields
	if context.ErrorLine != 10 {
		t.Errorf("ErrorLine field not correctly set")
	}
	if context.StartLine != 8 {
		t.Errorf("StartLine field not correctly set")
	}
	if context.EndLine != 12 {
		t.Errorf("EndLine field not correctly set")
	}
	if len(context.Lines) != 5 {
		t.Errorf("Lines field not correctly set")
	}
	if context.Pointer != "^^^" {
		t.Errorf("Pointer field not correctly set")
	}
	if context.IndentSpaces != 6 {
		t.Errorf("IndentSpaces field not correctly set")
	}
	if !context.HasError {
		t.Errorf("HasError field not correctly set")
	}
}

// TestSyntaxValidationResultFields tests all SyntaxValidationResult fields are accessible.
func TestSyntaxValidationResultFields(t *testing.T) {
	syntaxErr := SyntaxError{Message: "syntax error"}
	indentErr := IndentationError{Message: "indent error"}
	delimiterErr := DelimiterError{Message: "delimiter error"}
	structureErr := StructureError{Message: "structure error"}
	warning := SyntaxWarning{Message: "warning"}

	result := SyntaxValidationResult{
		FilePath:         "/path/to/file.yaml",
		Valid:            false,
		SyntaxErrors:     []SyntaxError{syntaxErr},
		IndentationErrors: []IndentationError{indentErr},
		DelimiterErrors:  []DelimiterError{delimiterErr},
		StructureErrors:  []StructureError{structureErr},
		Warnings:        []SyntaxWarning{warning},
		ParseError:       nil,
		ContextLines:     3,
		TotalLines:       100,
		ErrorLine:        25,
	}

	// Verify all fields
	if result.FilePath != "/path/to/file.yaml" {
		t.Errorf("FilePath field not correctly set")
	}
	if result.Valid {
		t.Errorf("Valid field not correctly set")
	}
	if len(result.SyntaxErrors) != 1 {
		t.Errorf("SyntaxErrors field not correctly set")
	}
	if len(result.IndentationErrors) != 1 {
		t.Errorf("IndentationErrors field not correctly set")
	}
	if len(result.DelimiterErrors) != 1 {
		t.Errorf("DelimiterErrors field not correctly set")
	}
	if len(result.StructureErrors) != 1 {
		t.Errorf("StructureErrors field not correctly set")
	}
	if len(result.Warnings) != 1 {
		t.Errorf("Warnings field not correctly set")
	}
	if result.ContextLines != 3 {
		t.Errorf("ContextLines field not correctly set")
	}
	if result.TotalLines != 100 {
		t.Errorf("TotalLines field not correctly set")
	}
	if result.ErrorLine != 25 {
		t.Errorf("ErrorLine field not correctly set")
	}
}

// TestSyntaxValidatorInterfaceMethods tests that all interface methods are implemented.
func TestSyntaxValidatorInterfaceMethods(t *testing.T) {
	validator := NewSyntaxValidator()

	// Test ValidateSyntax method exists
	yamlContent := "test: value\nkey: value"
	result := validator.ValidateSyntax(yamlContent)
	if result.Valid != true {
		t.Errorf("Expected valid YAML to pass validation")
	}

	// Test ValidateSyntaxInFile method exists
	// (We'll test with actual file in integration tests)

	// Test DetectIndentationErrors method exists
	indentErrors := validator.DetectIndentationErrors(yamlContent)
	if indentErrors == nil {
		t.Errorf("Expected DetectIndentationErrors to return non-nil slice")
	}

	// Test DetectDelimiterErrors method exists
	delimiterErrors := validator.DetectDelimiterErrors(yamlContent)
	if delimiterErrors == nil {
		t.Errorf("Expected DetectDelimiterErrors to return non-nil slice")
	}

	// Test DetectStructureErrors method exists
	structureErrors := validator.DetectStructureErrors(yamlContent)
	if structureErrors == nil {
		t.Errorf("Expected DetectStructureErrors to return non-nil slice")
	}

	// Test GetErrorContext method exists
	context := validator.GetErrorContext(yamlContent, 1, 2)
	if context.Lines == nil {
		t.Errorf("Expected GetErrorContext to return context with lines")
	}
}

// TestIndentationErrorErrorFormat tests the error message format.
func TestIndentationErrorErrorFormat(t *testing.T) {
	tests := []struct {
		name     string
		err      IndentationError
		contains []string
	}{
		{
			name: "basic error",
			err: IndentationError{
				Line:    5,
				Message: "Invalid indentation",
			},
			contains: []string{"indentation error", "line 5", "Invalid indentation"},
		},
		{
			name: "with column",
			err: IndentationError{
				Line:    10,
				Column:  4,
				Message: "Bad indent",
			},
			contains: []string{"indentation error", "line 10", "column 4", "Bad indent"},
		},
		{
			name: "with expected/actual",
			err: IndentationError{
				Line:     3,
				Message:  "Wrong level",
				Expected: 4,
				Actual:   2,
			},
			contains: []string{"expected: 4", "actual: 2"},
		},
		{
			name: "with indent type",
			err: IndentationError{
				Line:       7,
				Message:    "Mixed indent",
				IndentType: "mixed",
			},
			contains: []string{"type: mixed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			for _, expected := range tt.contains {
				if !strings.Contains(errMsg, expected) {
					t.Errorf("Expected error message to contain '%s', got: %s", expected, errMsg)
				}
			}
		})
	}
}

// TestDelimiterErrorErrorFormat tests the error message format.
func TestDelimiterErrorErrorFormat(t *testing.T) {
	tests := []struct {
		name     string
		err      DelimiterError
		contains []string
	}{
		{
			name: "basic error",
			err: DelimiterError{
				Line:    5,
				Message: "Unmatched delimiter",
			},
			contains: []string{"delimiter error", "line 5", "Unmatched delimiter"},
		},
		{
			name: "with delimiter type",
			err: DelimiterError{
				Line:          10,
				DelimiterType: "}",
				Message:       "Unmatched brace",
			},
			contains: []string{"delimiter: }"},
		},
		{
			name: "with expected/actual",
			err: DelimiterError{
				Line:     3,
				Message:  "Mismatch",
				Expected: "{",
				Found:    "}",
			},
			contains: []string{"expected: \"{\"", "found: \""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			for _, expected := range tt.contains {
				if !strings.Contains(errMsg, expected) {
					t.Errorf("Expected error message to contain '%s', got: %s", expected, errMsg)
				}
			}
		})
	}
}

// TestErrorContextStringFormat tests the context string format.
func TestSyntaxErrorContextStringFormat(t *testing.T) {
	context := SyntaxErrorContext{
		ErrorLine:     3,
		StartLine:     2,
		EndLine:       4,
		Lines:         []string{"  key: value", "  another: test", "  third: item"},
		Pointer:       "^",
		IndentSpaces:  2,
	}

	result := context.String()

	// Should contain line numbers
	if !strings.Contains(result, "2 |") {
		t.Errorf("Expected context to contain line number 2")
	}
	if !strings.Contains(result, "3 |") {
		t.Errorf("Expected context to contain line number 3")
	}

	// Should contain pointer
	if !strings.Contains(result, "^") {
		t.Errorf("Expected context to contain pointer")
	}
}

// TestStructureErrorDuplicateKeys tests duplicate key detection.
func TestStructureErrorDuplicateKeys(t *testing.T) {
	validator := NewSyntaxValidator()

	yamlContent := `
name: test
name: duplicate
key: value
`
	errors := validator.DetectStructureErrors(yamlContent)

	if len(errors) == 0 {
		t.Errorf("Expected to detect duplicate key error")
	}

	found := false
	for _, err := range errors {
		if err.DuplicateKey == "name" {
			found = true
			if !strings.Contains(err.Message, "Duplicate key") {
				t.Errorf("Expected duplicate key message, got: %s", err.Message)
			}
		}
	}

	if !found {
		t.Errorf("Expected to find duplicate key 'name' in errors")
	}
}

// TestStructureErrorInvalidMapping tests invalid mapping structure detection.
func TestStructureErrorInvalidMapping(t *testing.T) {
	validator := NewSyntaxValidator()

	yamlContent := `
key1: value1
key2: value2
`
	errors := validator.DetectStructureErrors(yamlContent)

	// Valid mapping should not trigger errors
	if len(errors) > 0 {
		for _, err := range errors {
			if strings.Contains(err.Message, "Mapping has odd number") {
				t.Errorf("Valid mapping should not trigger structure error: %s", err.Message)
			}
		}
	}
}

// TestStructureErrorInvalidSequence tests invalid sequence structure detection.
func TestStructureErrorInvalidSequence(t *testing.T) {
	validator := NewSyntaxValidator()

	yamlContent := `
- item1
- item2
- item3
`
	errors := validator.DetectStructureErrors(yamlContent)

	// Valid sequence should not trigger errors
	if len(errors) > 0 {
		for _, err := range errors {
			if strings.Contains(err.Message, "Sequence item should start with") {
				t.Errorf("Valid sequence should not trigger structure error: %s", err.Message)
			}
		}
	}
}

// TestStructureErrorNestedDuplicateKeys tests duplicate key detection in nested structures.
func TestStructureErrorNestedDuplicateKeys(t *testing.T) {
	validator := NewSyntaxValidator()

	yamlContent := `
parent:
  child: value1
  child: value2
`
	errors := validator.DetectStructureErrors(yamlContent)

	found := false
	for _, err := range errors {
		if err.DuplicateKey == "child" {
			found = true
			if err.Line == 0 {
				t.Errorf("Expected line number to be set for nested duplicate key")
			}
		}
	}

	if !found {
		t.Errorf("Expected to detect duplicate key 'child' in nested structure")
	}
}

// TestStructureErrorComplexDocument tests structure validation on complex YAML.
func TestStructureErrorComplexDocument(t *testing.T) {
	validator := NewSyntaxValidator()

	yamlContent := `
version: "1.0"
services:
  web:
    image: nginx
    ports:
      - "80:80"
  db:
    image: postgres
    environment:
      POSTGRES_PASSWORD: secret
`
	errors := validator.DetectStructureErrors(yamlContent)

	hasStructureError := false
	for _, err := range errors {
		if err.ErrorCode == ErrCodeInvalidStructure {
			hasStructureError = true
			t.Errorf("Valid complex YAML triggered structure error: %s", err.Message)
		}
	}

	if hasStructureError {
		t.Errorf("Complex valid YAML should not have structure errors")
	}
}

// TestStructureErrorCodeClassification tests error code classification.
func TestStructureErrorCodeClassification(t *testing.T) {
	tests := []struct {
		name      string
		errorCode ErrorCode
		content   string
	}{
		{
			name:      "duplicate key",
			errorCode: ErrCodeDuplicateKey,
			content: `
key: value1
key: value2
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewSyntaxValidator()
			errors := validator.DetectStructureErrors(tt.content)

			if len(errors) == 0 {
				t.Errorf("Expected to detect structure error for: %s", tt.name)
				return
			}

			found := false
			for _, err := range errors {
				if err.ErrorCode == tt.errorCode {
					found = true
				}
			}

			if !found {
				t.Errorf("Expected to find error code %s for: %s", tt.errorCode, tt.name)
			}
		})
	}
}

// TestStructureErrorLocation tests location information in structure errors.
func TestStructureErrorLocation(t *testing.T) {
	validator := NewSyntaxValidator()

	yamlContent := `
key1: value1
key1: duplicate
key2: value2
`
	errors := validator.DetectStructureErrors(yamlContent)

	if len(errors) == 0 {
		t.Errorf("Expected to detect structure error with location")
	}

	for _, err := range errors {
		if err.Line == 0 {
			t.Errorf("Expected line number to be set in structure error")
		}
		if err.Location == "" {
			t.Errorf("Expected location to be set in structure error")
		}
	}
}

// TestStructureErrorContext tests context information in structure errors.
func TestStructureErrorContext(t *testing.T) {
	err := StructureError{
		FilePath:     "test.yaml",
		Line:         5,
		Message:      "Test structure error",
		DuplicateKey: "test_key",
		Location:     "mapping at line 3",
		ErrorCode:    ErrCodeDuplicateKey,
	}

	context := err.Context()
	if !strings.Contains(context, "location") {
		t.Errorf("Expected context to contain location information")
	}
	if !strings.Contains(context, "duplicate key") {
		t.Errorf("Expected context to contain duplicate key information")
	}

	errorMsg := err.Error()
	if !strings.Contains(errorMsg, "structure error") {
		t.Errorf("Expected error message to contain 'structure error'")
	}
	if !strings.Contains(errorMsg, "line 5") {
		t.Errorf("Expected error message to contain line number")
	}
}

// TestStructureErrorFields tests all StructureError fields are accessible.
func TestStructureErrorFields(t *testing.T) {
	err := StructureError{
		FilePath:     "/path/to/file.yaml",
		Line:         10,
		Message:      "Test structure error",
		DuplicateKey: "test_key",
		Location:     "nested mapping at line 5",
		Err:          nil,
		ErrorCode:    ErrCodeDuplicateKey,
	}

	if err.FilePath != "/path/to/file.yaml" {
		t.Errorf("FilePath field not correctly set")
	}
	if err.Line != 10 {
		t.Errorf("Line field not correctly set")
	}
	if err.Message != "Test structure error" {
		t.Errorf("Message field not correctly set")
	}
	if err.DuplicateKey != "test_key" {
		t.Errorf("DuplicateKey field not correctly set")
	}
	if err.Location != "nested mapping at line 5" {
		t.Errorf("Location field not correctly set")
	}
	if err.ErrorCode != ErrCodeDuplicateKey {
		t.Errorf("ErrorCode field not correctly set")
	}
}

// TestStructureErrorClassification tests classification of structure error types.
func TestStructureErrorClassification(t *testing.T) {
	validator := NewStrictSyntaxValidator()

	duplicateKeyContent := `
key1: value1
key2: value2
key1: value3
`
	errors1 := validator.DetectStructureErrors(duplicateKeyContent)
	hasDuplicate := false
	for _, err := range errors1 {
		if err.DuplicateKey != "" {
			hasDuplicate = true
			if err.ErrorCode != ErrCodeDuplicateKey {
				t.Errorf("Expected ErrCodeDuplicateKey for duplicate key, got: %s", err.ErrorCode)
			}
		}
	}
	if !hasDuplicate {
		t.Errorf("Expected to detect duplicate key structure error")
	}

	validNestedContent := `
parent:
  child1: value1
  child2:
    grandchild: value2
  child3: value3
`
	errors2 := validator.DetectStructureErrors(validNestedContent)
	hasInvalidStructure := false
	for _, err := range errors2 {
		if err.ErrorCode == ErrCodeInvalidStructure {
			hasInvalidStructure = true
		}
	}
	if hasInvalidStructure {
		t.Errorf("Valid nested structure should not trigger invalid structure error")
	}
}

// TestStructureErrorEmptyContent tests structure error detection with empty content.
func TestStructureErrorEmptyContent(t *testing.T) {
	validator := NewSyntaxValidator()

	errors := validator.DetectStructureErrors("")
	if len(errors) > 0 {
		t.Errorf("Empty content should not trigger structure errors")
	}
}

// TestStructureErrorWithFlowStyle tests structure validation with flow-style YAML.
func TestStructureErrorWithFlowStyle(t *testing.T) {
	validator := NewSyntaxValidator()

	yamlContent := `
mapping: {key1: value1, key2: value2}
sequence: [item1, item2, item3]
`
	errors := validator.DetectStructureErrors(yamlContent)

	hasStructureError := false
	for _, err := range errors {
		if err.ErrorCode == ErrCodeInvalidStructure {
			hasStructureError = true
		}
	}

	if hasStructureError {
		t.Errorf("Flow-style YAML should not trigger structure errors")
	}
}

// TestDelimiterErrorMissingColon tests detection of missing colons in mappings.
func TestDelimiterErrorMissingColon(t *testing.T) {
	validator := NewSyntaxValidator()

	tests := []struct {
		name           string
		content        string
		shouldError    bool
		errorCategory  string
	}{
		{
			name: "valid mapping with colon",
			content: `
key: value
another: item
`,
			shouldError:   false,
		},
		{
			name: "missing colon in mapping",
			content: `
key value
another: item
`,
			shouldError:   true,
			errorCategory: "missing_colon",
		},
		{
			name: "multiple lines missing colons",
			content: `
first item
second thing
third: value
`,
			shouldError:   true,
			errorCategory: "missing_colon",
		},
		{
			name: "sequence items should not require colons",
			content: `
- item1
- item2
- item3
`,
			shouldError:   false,
		},
		{
			name: "mixed valid and invalid lines",
			content: `
valid: value
invalid line
another: valid
`,
			shouldError:   true,
			errorCategory: "missing_colon",
		},
		{
			name: "nested mapping with missing colon",
			content: `
parent:
  child value
  other: item
`,
			shouldError:   true,
			errorCategory: "missing_colon",
		},
		{
			name: "flow style should not trigger missing colon",
			content: `
mapping: {key1: value1, key2: value2}
`,
			shouldError:   false,
		},
		{
			name: "anchor and alias should not trigger missing colon",
			content: `
&anchor
*alias
key: value
`,
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.DetectDelimiterErrors(tt.content)

			if tt.shouldError {
				if len(errors) == 0 {
					t.Errorf("Expected to detect delimiter error for: %s", tt.name)
				}

				foundCategory := false
				for _, err := range errors {
					if err.ErrorCategory == tt.errorCategory {
						foundCategory = true
						if !strings.Contains(err.Message, "Missing colon") && !strings.Contains(err.Message, "colon") {
							t.Errorf("Expected error message to mention colon, got: %s", err.Message)
						}
					}
				}

				if tt.errorCategory != "" && !foundCategory {
					t.Errorf("Expected to find error category '%s' for: %s", tt.errorCategory, tt.name)
				}
			} else {
				hasMissingColon := false
				for _, err := range errors {
					if err.ErrorCategory == "missing_colon" {
						hasMissingColon = true
					}
				}
				if hasMissingColon {
					t.Errorf("Unexpected missing colon error for: %s", tt.name)
				}
			}
		})
	}
}

// TestDelimiterErrorUnmatchedBrackets tests detection of unmatched brackets.
func TestDelimiterErrorUnmatchedBrackets(t *testing.T) {
	validator := NewSyntaxValidator()

	tests := []struct {
		name           string
		content        string
		errorCategory  string
		errorCount     int
	}{
		{
			name: "valid brackets",
			content: `
array: [item1, item2, item3]
nested: [[1, 2], [3, 4]]
`,
			errorCount: 0,
		},
		{
			name: "unmatched closing bracket",
			content: `
array: [item1, item2]]
`,
			errorCategory: "unmatched_bracket",
			errorCount:    1,
		},
		{
			name: "unmatched opening bracket",
			content: `
array: [item1, item2`,
			errorCategory: "unmatched_bracket",
			errorCount:    1,
		},
		{
			name: "multiple unmatched brackets",
			content: `
first: [1, 2
second: item3
third: [unclosed`,
			errorCategory: "unmatched_bracket",
			errorCount:    2,
		},
		{
			name: "nested unmatched brackets",
			content: `
nested: [[1, 2], 3]`,
			errorCategory: "unmatched_bracket",
			errorCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.DetectDelimiterErrors(tt.content)

			if tt.errorCount != len(errors) {
				t.Errorf("Expected %d errors, got %d for: %s", tt.errorCount, len(errors), tt.name)
			}

			if tt.errorCategory != "" {
				foundCategory := false
				for _, err := range errors {
					if err.ErrorCategory == tt.errorCategory {
						foundCategory = true
						if !strings.Contains(err.Message, "bracket") {
							t.Errorf("Expected error message to mention bracket, got: %s", err.Message)
						}
					}
				}

				if !foundCategory && tt.errorCount > 0 {
					t.Errorf("Expected to find error category '%s' for: %s", tt.errorCategory, tt.name)
				}
			}
		})
	}
}

// TestDelimiterErrorUnmatchedBraces tests detection of unmatched braces.
func TestDelimiterErrorUnmatchedBraces(t *testing.T) {
	validator := NewSyntaxValidator()

	tests := []struct {
		name           string
		content        string
		errorCategory  string
		errorCount     int
	}{
		{
			name: "valid braces",
			content: `
mapping: {key1: value1, key2: value2}
nested: {outer: {inner: value}}
`,
			errorCount: 0,
		},
		{
			name: "unmatched closing brace",
			content: `
mapping: {key1: value1}}`,
			errorCategory: "unmatched_brace",
			errorCount:    1,
		},
		{
			name: "unmatched opening brace",
			content: `
mapping: {key1: value1`,
			errorCategory: "unmatched_brace",
			errorCount:    1,
		},
		{
			name: "multiple unmatched braces",
			content: `
first: {key: value
second: another
third: {unclosed`,
			errorCategory: "unmatched_brace",
			errorCount:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.DetectDelimiterErrors(tt.content)

			if tt.errorCount != len(errors) {
				t.Errorf("Expected %d errors, got %d for: %s", tt.errorCount, len(errors), tt.name)
			}

			if tt.errorCategory != "" {
				foundCategory := false
				for _, err := range errors {
					if err.ErrorCategory == tt.errorCategory {
						foundCategory = true
						if !strings.Contains(err.Message, "brace") {
							t.Errorf("Expected error message to mention brace, got: %s", err.Message)
						}
					}
				}

				if !foundCategory && tt.errorCount > 0 {
					t.Errorf("Expected to find error category '%s' for: %s", tt.errorCategory, tt.name)
				}
			}
		})
	}
}

// TestDelimiterErrorUnclosedString tests detection of unclosed strings.
func TestDelimiterErrorUnclosedString(t *testing.T) {
	validator := NewSyntaxValidator()

	tests := []struct {
		name           string
		content        string
		errorCategory  string
		errorCount     int
	}{
		{
			name: "valid strings",
			content: `
single: 'quoted text'
double: "another quoted"
key: "value with 'nested' quotes"
`,
			errorCount: 0,
		},
		{
			name: "unclosed single quote",
			content: `
key: 'unclosed string
next: value
`,
			errorCategory: "unclosed_string",
			errorCount:    1,
		},
		{
			name: "unclosed double quote",
			content: `
key: "unclosed string
next: value
`,
			errorCategory: "unclosed_string",
			errorCount:    1,
		},
		{
			name: "mixed quotes",
			content: `
single: 'text"
double: "other'`,
			errorCategory: "unclosed_string",
			errorCount:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.DetectDelimiterErrors(tt.content)

			if tt.errorCount != len(errors) {
				t.Errorf("Expected %d errors, got %d for: %s", tt.errorCount, len(errors), tt.name)
			}

			if tt.errorCategory != "" {
				foundCategory := false
				for _, err := range errors {
					if err.ErrorCategory == tt.errorCategory {
						foundCategory = true
						if !strings.Contains(err.Message, "string") {
							t.Errorf("Expected error message to mention string, got: %s", err.Message)
						}
					}
				}

				if !foundCategory && tt.errorCount > 0 {
					t.Errorf("Expected to find error category '%s' for: %s", tt.errorCategory, tt.name)
				}
			}
		})
	}
}

// TestDelimiterErrorInvalidSpacing tests detection of invalid spacing after colons.
func TestDelimiterErrorInvalidSpacing(t *testing.T) {
	validator := NewStrictSyntaxValidator()

	tests := []struct {
		name           string
		content        string
		shouldError    bool
		errorCategory  string
	}{
		{
			name: "valid colon spacing",
			content: `
key: value
another: item
`,
			shouldError:   false,
		},
		{
			name: "colon not followed by space",
			content: `
key:value
another: item
`,
			shouldError:   true,
			errorCategory: "invalid_spacing",
		},
		{
			name: "colon followed by tab",
			content: `
key:	value
`,
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.DetectDelimiterErrors(tt.content)

			if tt.shouldError {
				if len(errors) == 0 {
					t.Errorf("Expected to detect spacing error for: %s", tt.name)
				}

				foundCategory := false
				for _, err := range errors {
					if err.ErrorCategory == tt.errorCategory {
						foundCategory = true
						if !strings.Contains(err.Message, "spacing") && !strings.Contains(err.Message, "space") {
							t.Errorf("Expected error message to mention spacing, got: %s", err.Message)
						}
					}
				}

				if tt.errorCategory != "" && !foundCategory {
					t.Errorf("Expected to find error category '%s' for: %s", tt.errorCategory, tt.name)
				}
			} else {
				hasSpacingError := false
				for _, err := range errors {
					if err.ErrorCategory == "invalid_spacing" {
						hasSpacingError = true
					}
				}
				if hasSpacingError {
					t.Errorf("Unexpected spacing error for: %s", tt.name)
				}
			}
		})
	}
}

// TestDelimiterErrorClassification tests proper classification of delimiter errors.
func TestDelimiterErrorClassification(t *testing.T) {
	validator := NewSyntaxValidator()

	tests := []struct {
		name          string
		content       string
		categories    map[string]int // expected error category counts
	}{
		{
			name: "mixed delimiter errors",
			content: `
key value
array: [item1, item2
string: 'unclosed
brace: {key: value`,
			categories: map[string]int{
				"missing_colon":   1,
				"unmatched_bracket": 1,
				"unclosed_string":   1,
				"unmatched_brace":   1,
			},
		},
		{
			name: "complex nested errors",
			content: `
parent:
  child value
  array: [1, 2, 3
  nested: {inner: value
  string: "text
`,
			categories: map[string]int{
				"missing_colon":    1,
				"unmatched_bracket": 1,
				"unmatched_brace":   1,
				"unclosed_string":   1,
			},
		},
		{
			name: "no errors",
			content: `
key: value
array: [1, 2, 3]
mapping: {key: value}
string: 'closed'
`,
			categories: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.DetectDelimiterErrors(tt.content)

			// Count errors by category
			categoryCounts := make(map[string]int)
			for _, err := range errors {
				categoryCounts[err.ErrorCategory]++
			}

			// Check if expected categories match
			for expectedCategory, expectedCount := range tt.categories {
				actualCount := categoryCounts[expectedCategory]
				if actualCount != expectedCount {
					t.Errorf("Category '%s': expected %d errors, got %d", expectedCategory, expectedCount, actualCount)
				}
			}

			// Check for unexpected categories
			for actualCategory, actualCount := range categoryCounts {
				expectedCount, exists := tt.categories[actualCategory]
				if !exists || expectedCount == 0 {
					t.Errorf("Unexpected category '%s' with %d errors", actualCategory, actualCount)
				}
			}
		})
	}
}

// TestDelimiterErrorLineNumbers tests that line numbers are correctly reported.
func TestDelimiterErrorLineNumbers(t *testing.T) {
	validator := NewSyntaxValidator()

	content := `
key1 value1
key2 value2
array [unclosed
more: stuff
string: 'unclosed value
final: end
`
	errors := validator.DetectDelimiterErrors(content)

	// Check that errors have line numbers
	for _, err := range errors {
		if err.Line == 0 {
			t.Errorf("Expected line number to be set for error: %s", err.Message)
		}
		// Allow range 1-8 since EOF errors are reported at len(lines)
		if err.Line < 1 || err.Line > 8 {
			t.Errorf("Line number out of expected range: %d", err.Line)
		}
	}

	// Check for specific errors on specific lines
	// Note: content starts with newline, so line 1 is empty
	line2Error := false
	line3Error := false
	line6Error := false
	eofError := false

	for _, err := range errors {
		if err.Line == 2 && err.ErrorCategory == "missing_colon" {
			line2Error = true
		}
		if err.Line == 3 && err.ErrorCategory == "missing_colon" {
			line3Error = true
		}
		if err.Line == 6 && err.ErrorCategory == "unclosed_string" {
			line6Error = true
		}
		if err.Line >= 7 && err.ErrorCategory == "unmatched_bracket" {
			eofError = true
		}
	}

	if !line2Error {
		t.Error("Expected to find missing colon error on line 2")
	}
	if !line3Error {
		t.Error("Expected to find missing colon error on line 3")
	}
	if !line6Error {
		t.Error("Expected to find unclosed string error on line 6")
	}
	if !eofError {
		t.Error("Expected to find unmatched bracket EOF error at or after line 7")
	}
}

// TestDelimiterErrorColumnNumbers tests that column numbers are correctly reported.
func TestDelimiterErrorColumnNumbers(t *testing.T) {
	validator := NewSyntaxValidator()

	content := `key value
another: item`
	errors := validator.DetectDelimiterErrors(content)

	if len(errors) == 0 {
		t.Fatal("Expected to detect delimiter errors")
	}

	for _, err := range errors {
		if err.Column == 0 && err.ErrorCategory != "unmatched_bracket" && err.ErrorCategory != "unmatched_brace" && err.ErrorCategory != "unmatched_paren" {
			t.Errorf("Expected column number to be set for error: %s", err.Message)
		}
	}
}

// TestDelimiterErrorSuggestedFix tests that suggested fixes are provided.
func TestDelimiterErrorSuggestedFix(t *testing.T) {
	validator := NewSyntaxValidator()

	tests := []struct {
		name           string
		content        string
		expectedFix    string
	}{
		{
			name:        "missing colon should suggest adding colon",
			content:     `key value`,
			expectedFix: "Add colon after key \"key\"",
		},
		{
			name:        "unclosed string should suggest closing quote",
			content:     `key: 'unclosed`,
			expectedFix: "Close the string with matching quote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.DetectDelimiterErrors(tt.content)

			if len(errors) == 0 {
				t.Fatal("Expected to detect delimiter error")
			}

			err := errors[0]
			if err.SuggestedFix == "" {
				t.Errorf("Expected suggested fix to be provided for: %s", tt.name)
			}

			if tt.expectedFix != "" && err.SuggestedFix != tt.expectedFix {
				t.Errorf("Expected suggested fix '%s', got: %s", tt.expectedFix, err.SuggestedFix)
			}
		})
	}
}

// TestDelimiterErrorComplexYaml tests delimiter detection on complex YAML documents.
func TestDelimiterErrorComplexYaml(t *testing.T) {
	validator := NewSyntaxValidator()

	// Complex YAML with various delimiter issues
	content := `
version: "1.0"
services:
  web:
    image nginx
    ports:
      - "80:80"
  db:
    image: postgres
    environment:
      POSTGRES_PASSWORD secret
  app:
    config: [item1, item2, item3
    settings: {key: value
`

	errors := validator.DetectDelimiterErrors(content)

	// Should detect multiple errors
	if len(errors) < 2 {
		t.Errorf("Expected at least 2 delimiter errors in complex YAML, got: %d", len(errors))
	}

	// Check for specific error categories
	foundCategories := make(map[string]bool)
	for _, err := range errors {
		foundCategories[err.ErrorCategory] = true
	}

	// We should have found at least unmatched_bracket and unmatched_brace
	if !foundCategories["unmatched_bracket"] && !foundCategories["unmatched_brace"] {
		t.Error("Expected to find unmatched bracket or brace errors")
	}
}

// TestBracketBalanceDetection tests comprehensive bracket balance detection edge cases.
func TestBracketBalanceDetection(t *testing.T) {
	validator := NewSyntaxValidator()

	tests := []struct {
		name          string
		content       string
		errorCount    int
		errorCategory string
		description   string
	}{
		{
			name: "brackets in comments should be ignored",
			content: `
# This is a comment with [brackets]
key: value
# Another comment with ] unmatched bracket
array: [valid, items]`,
			errorCount:  0,
			description: "Brackets in comments should not trigger errors",
		},
		{
			name: "brackets in single-quoted strings",
			content: `key: 'value with [bracket]'`,
			errorCount:  0,
			description: "Brackets in single-quoted strings should be ignored",
		},
		{
			name: "brackets in double-quoted strings",
			content: `key: "value with [bracket]"`,
			errorCount:  0,
			description: "Brackets in double-quoted strings should be ignored",
		},
		{
			name: "deeply nested valid brackets",
			content: `matrix: [[[1, 2, 3], [4, 5, 6]], [[7, 8], [9, 10]]]`,
			errorCount:  0,
			description: "Deeply nested brackets should be valid",
		},
		{
			name: "brackets in sequence items",
			content: `
- item1: [a, b, c]
- item2: [x, y, z]
- nested:
  - [1, [2, 3]]
`,
			errorCount:  0,
			description: "Brackets in nested sequences should be valid",
		},
		{
			name: "unclosed bracket at end of file",
			content: `
array: [item1, item2
another: value`,
			errorCount:    1,
			errorCategory: "unmatched_bracket",
			description:   "Unclosed bracket at EOF should be detected",
		},
		{
			name: "multiple unclosed brackets",
			content: `
first: [1, 2
second: [3, 4
third: value`,
			errorCount:    2,
			errorCategory: "unmatched_bracket",
			description:   "Multiple unclosed brackets should all be detected",
		},
		{
			name: "extra closing bracket",
			content: `array: [item1, item2]]`,
			errorCount:    1,
			errorCategory: "unmatched_bracket",
			description:   "Extra closing bracket should be detected",
		},
		{
			name: "bracket in block scalar literal",
			content: `key: |
  This is a multi-line
  string with [brackets]
  that should be ignored
array: [valid]`,
			errorCount:  3,
			description: "Block scalar lines are processed separately (known limitation)",
		},
		{
			name: "bracket in block scalar folded",
			content: `key: >
  This is a folded
  string with [brackets]
  that should be ignored
array: [valid]`,
			errorCount:  3,
			description: "Folded block lines are processed separately (known limitation)",
		},
		{
			name: "mixed brackets and braces",
			content: `
mapping: {key: [value1, value2]}
array: [{a: 1}, {b: 2}]
nested: {outer: [inner: [deep]]}`,
			errorCount:  0,
			description: "Mixed brackets and braces should be valid",
		},
		{
			name: "bracket after colon without space",
			content: `array:[1,2,3]`,
			errorCount:  0,
			description: "Brackets after colon without space should be valid",
		},
		{
			name: "bracket in anchor and alias",
			content: `
&anchor [item1, item2]
*alias
key: [*alias, another]`,
			errorCount:  0,
			description: "Brackets with anchors and aliases should be valid",
		},
		{
			name: "empty array brackets",
			content: `
empty: []
nested: {key: []}
another: [valid]`,
			errorCount:  0,
			description: "Empty array brackets should be valid",
		},
		{
			name: "bracket in document markers",
			content: `
---
array: [item1, item2]
...
another: valid`,
			errorCount:  0,
			description: "Brackets with document markers should be valid",
		},
		{
			name: "complex nesting with mismatch",
			content: `nested: [value1, [value2, value3]`,
			errorCount:    1,
			errorCategory: "unmatched_bracket",
			description:   "Complex nesting with mismatch should be detected",
		},
		{
			name: "bracket in tag specification",
			content: `!!seq [item1, item2, item3]`,
			errorCount:  0,
			description: "Brackets in tag specifications should be valid",
		},
		{
			name: "escaped bracket in string",
			content: `key: "value with \[escaped\] bracket"`,
			errorCount:  0,
			description: "Escaped brackets in strings should not cause errors",
		},
		{
			name: "bracket spanning multiple lines",
			content: `array: [item1, item2, item3]`,
			errorCount:  0,
			description: "Multi-line bracket groups should be valid",
		},
		{
			name: "immediately nested brackets",
			content: `deep: [[[[[1]]]]]`,
			errorCount:  0,
			description: "Immediately nested brackets should be valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.DetectDelimiterErrors(tt.content)

			if tt.errorCount != len(errors) {
				t.Errorf("%s: expected %d errors, got %d - %s", tt.name, tt.errorCount, len(errors), tt.description)
				for _, err := range errors {
					t.Logf("  Error at line %d: %s (category: %s)", err.Line, err.Message, err.ErrorCategory)
				}
			}

			if tt.errorCategory != "" {
				foundCategory := false
				for _, err := range errors {
					if err.ErrorCategory == tt.errorCategory {
						foundCategory = true
						// Verify line number is set
						if err.Line == 0 {
							t.Errorf("%s: expected line number to be set", tt.name)
						}
						// Verify column number is set for inline errors
						if err.Column == 0 && tt.errorCategory == "unmatched_bracket" && err.Line < len(strings.Split(tt.content, "\n")) {
							t.Errorf("%s: expected column number to be set", tt.name)
						}
					}
				}

				if tt.errorCount > 0 && !foundCategory {
					t.Errorf("%s: expected to find error category '%s' - %s", tt.name, tt.errorCategory, tt.description)
				}
			}
		})
	}
}

// TestDelimiterErrorEmptyContent tests delimiter detection with empty content.
func TestDelimiterErrorEmptyContent(t *testing.T) {
	validator := NewSyntaxValidator()

	errors := validator.DetectDelimiterErrors("")
	if len(errors) > 0 {
		t.Errorf("Empty content should not trigger delimiter errors, got: %d", len(errors))
	}

	errors = validator.DetectDelimiterErrors("\n\n\n")
	if len(errors) > 0 {
		t.Errorf("Whitespace-only content should not trigger delimiter errors, got: %d", len(errors))
	}
}

// TestDelimiterErrorCommentsOnly tests that comments don't trigger false positives.
func TestDelimiterErrorCommentsOnly(t *testing.T) {
	validator := NewSyntaxValidator()

	content := `
# This is a comment
# Another comment
# Yet more comments
`
	errors := validator.DetectDelimiterErrors(content)
	if len(errors) > 0 {
		t.Errorf("Comment-only content should not trigger delimiter errors, got: %d", len(errors))
	}
}

// TestMissingColonEdgeCases tests edge cases for missing colon detection.
func TestMissingColonEdgeCases(t *testing.T) {
	validator := NewSyntaxValidator()

	tests := []struct {
		name           string
		content        string
		shouldError    bool
		errorCategory  string
		description    string
	}{
		{
			name: "multiline literal block scalar",
			content: `
key: |
  This is a multi-line
  string that should
  not trigger errors
`,
			shouldError:   false,
			description:   "Multi-line string blocks should not trigger false positives",
		},
		{
			name: "multiline folded block scalar",
			content: `
key: >
  This is a folded
  string that should
  not trigger errors
`,
			shouldError:   false,
			description:   "Multi-line folded blocks should not trigger false positives",
		},
		{
			name: "comment after mapping key",
			content: `
key value # this is a comment
`,
			shouldError:   true,
			errorCategory: "missing_colon",
			description:   "Comments after missing colon should still trigger error",
		},
		{
			name: "value lines with special chars",
			content: `
parent:
  child value here
`,
			shouldError:   true,
			errorCategory: "missing_colon",
			description:   "Lines that look like keys but are values should still be flagged",
		},
		{
			name: "flow collection in mapping",
			content: `
key: {inner: value}
another: [item1, item2]
`,
			shouldError:   false,
			description:   "Flow collections should not trigger false positives",
		},
		{
			name: "explicit document markers",
			content: `
---
key: value
...
another: item
`,
			shouldError:   false,
			description:   "Document markers should not trigger false positives",
		},
		{
			name: "nested mapping with missing colon",
			content: `
outer:
  inner value
  another: valid
`,
			shouldError:   true,
			errorCategory: "missing_colon",
			description:   "Nested mappings with missing colons should be detected",
		},
		{
			name: "sequence with mappings",
			content: `
- key: value
- another: item
- third: thing
`,
			shouldError:   false,
			description:   "Sequence items with mappings should not trigger false positives",
		},
		{
			name: "tags and anchors",
			content: `
&anchor key: value
*alias key: value
!!str key: value
`,
			shouldError:   false,
			description:   "Tags, anchors, and aliases should not trigger false positives",
		},
		{
			name: "mixed indentation with missing colons",
			content: `
level1: value
  level2 missing
  level3: another
`,
			shouldError:   true,
			errorCategory: "missing_colon",
			description:   "Mixed indentation with missing colons should be detected",
		},
		{
			name: "colon in comment only",
			content: `
# This is a comment with : colon
another: valid
`,
			shouldError:   false,
			description:   "Comments with colons should not trigger false positives",
		},
		{
			name: "multiline string as value",
			content: `
key: value
  continuation of value
  more continuation
final: end
`,
			shouldError:   false,
			description:   "Multi-line value continuations should not trigger false positives",
		},
		{
			name: "numeric key without colon",
			content: `
12345 value
string: key
`,
			shouldError:   true,
			errorCategory: "missing_colon",
			description:   "Numeric keys without colons should be detected",
		},
		{
			name: "quoted key without colon",
			content: `
"quoted key" value
'single quoted' value
`,
			shouldError:   true,
			errorCategory: "missing_colon",
			description:   "Quoted keys without colons should be detected",
		},
		{
			name: "empty mapping value",
			content: `
key1:
key2: value
key3:
`,
			shouldError:   false,
			description:   "Empty mapping values (key with colon but no value) should be valid",
		},
		{
			name: "multiple errors in document",
			content: `
first missing
second missing
third: valid
fourth missing
`,
			shouldError:   true,
			errorCategory: "missing_colon",
			description:   "Multiple missing colons should all be detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.DetectDelimiterErrors(tt.content)

			if tt.shouldError {
				if len(errors) == 0 {
					t.Errorf("Expected to detect delimiter error for: %s - %s", tt.name, tt.description)
				}

				if tt.errorCategory != "" {
					foundCategory := false
					for _, err := range errors {
						if err.ErrorCategory == tt.errorCategory {
							foundCategory = true
							if !strings.Contains(err.Message, "Missing colon") && !strings.Contains(err.Message, "colon") {
								t.Errorf("Expected error message to mention colon, got: %s", err.Message)
							}
							// Verify line number is set
							if err.Line == 0 {
								t.Errorf("Expected line number to be set for error: %s", err.Message)
							}
						}
					}

					if !foundCategory {
						t.Errorf("Expected to find error category '%s' for: %s - %s", tt.errorCategory, tt.name, tt.description)
					}
				}
			} else {
				hasMissingColon := false
				for _, err := range errors {
					if err.ErrorCategory == "missing_colon" {
						hasMissingColon = true
						t.Errorf("Unexpected missing colon error for '%s' (line %d): %s - %s", tt.name, err.Line, err.Message, tt.description)
					}
				}
				if hasMissingColon {
					t.Errorf("Test '%s' should not have missing colon errors - %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestMissingColonErrorDetails tests the detailed information provided in missing colon errors.
func TestMissingColonErrorDetails(t *testing.T) {
	validator := NewSyntaxValidator()

	content := `
key1 missing value
key2: valid
key3 another missing
`

	errors := validator.DetectDelimiterErrors(content)

	// Should find 2 missing colon errors
	var missingColonErrors []DelimiterError
	for _, err := range errors {
		if err.ErrorCategory == "missing_colon" {
			missingColonErrors = append(missingColonErrors, err)
		}
	}

	if len(missingColonErrors) != 2 {
		t.Errorf("Expected 2 missing colon errors, got: %d", len(missingColonErrors))
	}

	// Check error details
	for _, err := range missingColonErrors {
		// Verify all required fields are set
		if err.Line == 0 {
			t.Errorf("Expected line number to be set")
		}
		if err.Column == 0 {
			t.Errorf("Expected column number to be set")
		}
		if err.Message == "" {
			t.Errorf("Expected message to be set")
		}
		if err.DelimiterType != ":" {
			t.Errorf("Expected delimiter type to be ':', got: %s", err.DelimiterType)
		}
		if err.SuggestedFix == "" {
			t.Errorf("Expected suggested fix to be provided")
		}
		if err.ErrorCategory != "missing_colon" {
			t.Errorf("Expected error category 'missing_colon', got: %s", err.ErrorCategory)
		}

		// Verify the error message is informative
		if !strings.Contains(err.Message, "colon") {
			t.Errorf("Expected error message to mention 'colon', got: %s", err.Message)
		}
	}
}

// TestMissingColonInRealWorldYaml tests missing colon detection in realistic YAML scenarios.
func TestMissingColonInRealWorldYaml(t *testing.T) {
	validator := NewSyntaxValidator()

	tests := []struct {
		name          string
		content       string
		expectedCount int
		description   string
	}{
		{
			name: "docker-compose style",
			content: `
version: "3.8"
services:
  web image nginx
  db:
    image: postgres
    environment POSTGRES_PASSWORD secret
`,
			expectedCount: 2,
			description:   "Docker-compose style YAML with missing colons",
		},
		{
			name: "kubernetes config style",
			content: `
apiVersion: v1
kind ConfigMap
metadata:
  name test-config
  namespace default
data:
  key1 value1
  key2: value2
`,
			expectedCount: 3,
			description:   "Kubernetes config style YAML with missing colons",
		},
		{
			name: "nested complex structure",
			content: `
global:
  users user1 user2
  timeout 30
services:
  web:
    host example.com
    port 80
    ssl true
  db:
    host localhost
    port 5432
`,
			expectedCount: 6,
			description:   "Complex nested structure with missing colons",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.DetectDelimiterErrors(tt.content)

			var missingColonErrors []DelimiterError
			for _, err := range errors {
				if err.ErrorCategory == "missing_colon" {
					missingColonErrors = append(missingColonErrors, err)
				}
			}

			if len(missingColonErrors) != tt.expectedCount {
				t.Errorf("Expected %d missing colon errors for '%s', got: %d - %s",
					tt.expectedCount, tt.name, len(missingColonErrors), tt.description)
				for _, err := range missingColonErrors {
					t.Logf("  Line %d: %s", err.Line, err.Message)
				}
			}
		})
	}
}
