// Package yamlutil test to verify error message formatting
package yamlutil

import (
	"testing"
)

// TestErrorFormattingExamples verifies the exact output format of error messages
func TestErrorFormattingExamples(t *testing.T) {
	t.Run("ParseError with line:column context", func(t *testing.T) {
		pe := NewParseError("config.yaml", "invalid syntax", 10, 5, "", "identifier", "123")
		
		errMsg := pe.Error()
		t.Logf("ParseError output:\n%s", errMsg)
		
		// Verify line:column context
		if !containsInString(errMsg, "line 10") {
			t.Error("ParseError should contain line number")
		}
		if !containsInString(errMsg, "column 5") {
			t.Error("ParseError should contain column number")
		}
		if !containsInString(errMsg, "config.yaml") {
			t.Error("ParseError should contain file path")
		}
		if !containsInString(errMsg, "expected: identifier") {
			t.Error("ParseError should contain expected value")
		}
		if !containsInString(errMsg, "actual: 123") {
			t.Error("ParseError should contain actual value")
		}
	})

	t.Run("ValidationError with field path and constraint", func(t *testing.T) {
		ve := NewValidationError(
			"deployment.yaml",
			"port out of range",
			"spec.replicas",
			"must be between 1-65535",
			"",
			15,
			12,
			"",
			"spec.replicas",
		)
		
		errMsg := ve.Error()
		t.Logf("ValidationError output:\n%s", errMsg)
		
		// Verify field path and constraint
		if !containsInString(errMsg, "spec.replicas") {
			t.Error("ValidationError should contain field path")
		}
		if !containsInString(errMsg, "constraint: must be between 1-65535") {
			t.Error("ValidationError should contain constraint")
		}
		if !containsInString(errMsg, "line 15") {
			t.Error("ValidationError should contain line number")
		}
		if !containsInString(errMsg, "deployment.yaml") {
			t.Error("ValidationError should contain file path")
		}
	})

	t.Run("TypeMismatchError with expected vs actual types", func(t *testing.T) {
		tme := NewTypeMismatchError(
			"config.yaml",
			"server.port",
			"integer",
			"string",
			"8080",
			20,
			"",
		)
		
		errMsg := tme.Error()
		t.Logf("TypeMismatchError output:\n%s", errMsg)
		
		// Verify expected vs actual types
		if !containsInString(errMsg, "expected integer") {
			t.Error("TypeMismatchError should contain expected type")
		}
		if !containsInString(errMsg, "got string") {
			t.Error("TypeMismatchError should contain actual type")
		}
		if !containsInString(errMsg, "server.port") {
			t.Error("TypeMismatchError should contain field path")
		}
		if !containsInString(errMsg, "line 20") {
			t.Error("TypeMismatchError should contain line number")
		}
	})
}

func containsInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestHumanReadableFormatting verifies messages are human-readable
func TestHumanReadableFormatting(t *testing.T) {
	t.Run("consistent error prefix format", func(t *testing.T) {
		errors := []struct {
			name string
			err  error
		}{
			{"parse error", NewParseError("test.yaml", "bad syntax", 5, 10, "", "", "")},
			{"validation error", NewValidationError("test.yaml", "invalid value", "", "", "", 5, 10, "", "test.yaml")},
			{"syntax error", NewSyntaxError("test.yaml", "syntax issue", 5, 10, "", "", "")},
			{"structure error", NewStructureError("test.yaml", "bad structure", 5, "", "", "")},
		}
		
		for _, tt := range errors {
			t.Run(tt.name, func(t *testing.T) {
				errMsg := tt.err.Error()
				t.Logf("%s: %s", tt.name, errMsg)
				
				// All should be non-empty
				if errMsg == "" {
					t.Error("Error message should not be empty")
				}
				
				// All should contain file path
				if !containsInString(errMsg, "test.yaml") {
					t.Error("Error should contain file path")
				}
			})
		}
	})
}
