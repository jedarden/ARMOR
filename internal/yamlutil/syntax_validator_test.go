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
