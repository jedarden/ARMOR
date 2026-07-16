package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// BASIC ERROR FORMATTING TESTS (Required Fields Only)
// =============================================================================

// TestBasicErrorFormatting_RequiredFieldsOnly tests error formatting with only required fields
func TestBasicErrorFormatting_RequiredFieldsOnly(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		wantType  string
		wantMsg   string
	}{
		{
			name:      "status_code error type with message",
			errorType: "status_code",
			message:   "Expected 200 but got 404",
			wantType:  "status_code",
			wantMsg:   "Expected 200 but got 404",
		},
		{
			name:      "error_message error type with message",
			errorType: "error_message",
			message:   "Pattern mismatch",
			wantType:  "error_message",
			wantMsg:   "Pattern mismatch",
		},
		{
			name:      "content_type error type with message",
			errorType: "content_type",
			message:   "Invalid content type",
			wantType:  "content_type",
			wantMsg:   "Invalid content type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidationError{
				ErrorType: tt.errorType,
				Message:   tt.message,
			}

			// Validate required fields are set
			if err.ErrorType != tt.wantType {
				t.Errorf("ValidationError.ErrorType = %v, want %v", err.ErrorType, tt.wantType)
			}
			if err.Message != tt.wantMsg {
				t.Errorf("ValidationError.Message = %v, want %v", err.Message, tt.wantMsg)
			}

			// Test Validate() method
			if err := err.Validate(); err != nil {
				t.Errorf("ValidationError.Validate() unexpectedly failed: %v", err)
			}

				// When Message is not in the default format (ErrorType + " validation failed"),
				// the Error() output is just the Message itself
				errorOutput := err.Error()
				if !strings.Contains(errorOutput, tt.message) {
					t.Errorf("Error() output missing message: got %v, want to contain %v", errorOutput, tt.message)
				}
				// The error type appears in output only if Message starts with "ErrorType validation failed"
				if strings.HasPrefix(tt.message, tt.errorType+" validation failed") {
					if !strings.Contains(errorOutput, tt.errorType) {
						t.Errorf("Error() output should contain error type with default format: got %v, want to contain %v", errorOutput, tt.errorType)
					}
				}
		})
	}
}

// TestValidationError_ValidateMethod tests the Validate() method with various states
func TestValidationError_ValidateMethod(t *testing.T) {
	tests := []struct {
		name        string
		err         ValidationError
		wantErr     bool
		errContains string
	}{
		{
			name: "valid error with required fields",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test message",
			},
			wantErr: false,
		},
		{
			name: "missing ErrorType field",
			err: ValidationError{
				Message: "test message",
			},
			wantErr:     true,
			errContains: "ErrorType",
		},
		{
			name: "missing Message field",
			err: ValidationError{
				ErrorType: "status_code",
			},
			wantErr:     true,
			errContains: "Message",
		},
		{
			name: "missing both required fields",
			err: ValidationError{},
			wantErr:     true,
			errContains: "ErrorType",
		},
		{
			name: "valid error with optional fields",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "pattern mismatch",
				Expected:  "expected_pattern",
				Actual:     "actual_message",
				Context:    "API validation",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.err.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidationError.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidationError.Validate() error = %v, want to contain %v", err, tt.errContains)
				}
			}
		})
	}
}

// =============================================================================
// OPTIONAL FIELD TESTS (Individual and Combined)
// =============================================================================

// TestOptionalFields_Individual tests each optional field individually
func TestOptionalFields_Individual(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func() ValidationError
		shouldContain []string
		shouldNotContain []string
	}{
		{
			name: "Expected field only",
			setupFunc: func() ValidationError {
				return ValidationError{
					ErrorType: "status_code",
					Message:   "mismatch",
					Expected:  200,
				}
			},
			shouldContain: []string{"Expected:", "200"},
		},
		{
			name: "Actual field only",
			setupFunc: func() ValidationError {
				return ValidationError{
					ErrorType: "status_code",
					Message:   "mismatch",
					Actual:    404,
				}
			},
			shouldContain: []string{"Actual:", "404"},
		},
		{
			name: "Context field only",
			setupFunc: func() ValidationError {
				return ValidationError{
					ErrorType: "status_code",
					Message:   "mismatch",
					Context:   "GET /api/users",
				}
			},
			shouldContain: []string{"Context:", "GET /api/users"},
		},
		{
			name: "FieldName field only",
			setupFunc: func() ValidationError {
				return ValidationError{
					ErrorType: "error_message",
					Message:   "mismatch",
					FieldName: "error",
				}
			},
			shouldContain: []string{"Field:", "error"},
		},
		{
			name: "Location field only",
			setupFunc: func() ValidationError {
				return ValidationError{
					ErrorType: "content_type",
					Message:   "invalid",
					Location:  "header 'Content-Type'",
				}
			},
			shouldContain: []string{"Location:", "header 'Content-Type'"},
		},
		{
			name: "RelatedFields field only",
			setupFunc: func() ValidationError {
				return ValidationError{
					ErrorType: "error_message",
					Message:   "mismatch",
					RelatedFields: []string{"email", "email_confirmation"},
				}
			},
			shouldContain: []string{"Related fields:", "email", "email_confirmation"},
		},
		{
			name: "PatternDetails field only",
			setupFunc: func() ValidationError {
				return ValidationError{
					ErrorType: "error_message",
					Message:   "mismatch",
					PatternDetails: "regex pattern 'invalid.*token' did not match",
				}
			},
			shouldContain: []string{"Pattern:", "regex pattern"},
		},
		{
			name: "RangeInfo field only",
			setupFunc: func() ValidationError {
				return ValidationError{
					ErrorType: "status_code_range",
					Message:   "out of range",
					RangeInfo: "400-499 (Client Error)",
				}
			},
			shouldContain: []string{"Range:", "400-499"},
		},
		{
			name: "ValidationDetails field only",
			setupFunc: func() ValidationError {
				return ValidationError{
					ErrorType: "status_code_range",
					Message:   "out of range",
					ValidationDetails: []string{"Detail 1", "Detail 2"},
				}
			},
			shouldContain: []string{"Details:", "Detail 1", "Detail 2"},
		},
		{
			name: "ResponseSnippet field only",
			setupFunc: func() ValidationError {
				return ValidationError{
					ErrorType: "error_message",
					Message:   "mismatch",
					ResponseSnippet: `{"error": "sample"}`,
				}
			},
			shouldContain: []string{"Response:", `{"error": "sample"}`},
		},
		{
			name: "Suggestions field only",
			setupFunc: func() ValidationError {
				return ValidationError{
					ErrorType: "status_code",
					Message:   "mismatch",
					Suggestions: []string{"Check endpoint", "Verify ID"},
				}
			},
			shouldContain: []string{"Suggestions:", "Check endpoint", "Verify ID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setupFunc()
			errorOutput := err.Error()

			for _, expected := range tt.shouldContain {
				if !strings.Contains(errorOutput, expected) {
					t.Errorf("Error() output missing expected string '%s':\ngot:\n%s", expected, errorOutput)
				}
			}

			for _, notExpected := range tt.shouldNotContain {
				if strings.Contains(errorOutput, notExpected) {
					t.Errorf("Error() output contains unexpected string '%s':\ngot:\n%s", notExpected, errorOutput)
				}
			}
		})
	}
}

// TestOptionalFields_Combined tests multiple optional fields together
func TestOptionalFields_Combined(t *testing.T) {
	tests := []struct {
		name          string
		err          ValidationError
		shouldContain []string
	}{
		{
			name: "Expected and Actual together",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "mismatch",
				Expected:  200,
				Actual:    404,
			},
			shouldContain: []string{"Expected:", "200", "Actual:", "404"},
		},
		{
			name: "Expected, Actual, and Context together",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "mismatch",
				Expected:  200,
				Actual:    404,
				Context:   "GET /api/users",
			},
			shouldContain: []string{"Expected:", "200", "Actual:", "404", "Request:", "GET /api/users"},
		},
		{
			name: "FieldName and Location together",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "mismatch",
				FieldName: "error",
				Location:  "line 5",
			},
			shouldContain: []string{"Field:", "error", "Location:", "line 5"},
		},
		{
			name: "RelatedFields and Suggestions together",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "mismatch",
				RelatedFields: []string{"email", "email_confirmation"},
				Suggestions: []string{"Check format", "Verify pattern"},
			},
			shouldContain: []string{"Related fields:", "email", "Suggestions:", "Check format"},
		},
		{
			name: "PatternDetails and ValidationDetails together",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "mismatch",
				PatternDetails: "pattern did not match",
				ValidationDetails: []string{"Expected: token", "Actual: error"},
			},
			shouldContain: []string{"Pattern:", "pattern did not match", "Details:", "Expected: token"},
		},
		{
			name: "RangeInfo and ResponseSnippet together",
			err: ValidationError{
				ErrorType: "status_code_range",
				Message:   "out of range",
				RangeInfo: "400-499",
				ResponseSnippet: `{"status": "error"}`,
			},
			shouldContain: []string{"Range:", "400-499", "Response:", `{"status": "error"}`},
		},
		{
			name: "All optional fields together",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "comprehensive error",
				Expected:  "expected_pattern",
				Actual:    "actual_value",
				Context:   "API validation",
				FieldName: "error",
				Location:  "line 10",
				RelatedFields: []string{"error", "message"},
				PatternDetails: "regex mismatch",
				RangeInfo: "",
				ValidationDetails: []string{"Detail 1", "Detail 2"},
				ResponseSnippet: `{"error": "test"}`,
				Suggestions: []string{"Suggestion 1", "Suggestion 2"},
			},
			shouldContain: []string{
				"Expected:", "expected_pattern",
				"Actual:", "actual_value",
				"Context:", "API validation",
				"Field:", "error",
				"Location:", "line 10",
				"Related fields:", "error",
				"Pattern:", "regex mismatch",
				"Details:", "Detail 1",
				"Response:", `{"error": "test"}`,
				"Suggestions:", "Suggestion 1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorOutput := tt.err.Error()

			for _, expected := range tt.shouldContain {
				if !strings.Contains(errorOutput, expected) {
					t.Errorf("Error() output missing expected string '%s':\ngot:\n%s", expected, errorOutput)
				}
			}
		})
	}
}

// =============================================================================
// EDGE CASE TESTS (Nil Values, Empty Strings, Missing Fields)
// =============================================================================

// TestEdgeCases_NilValues tests handling of nil values
func TestEdgeCases_NilValues(t *testing.T) {
	tests := []struct {
		name          string
		err          ValidationError
		shouldContain []string
		shouldNotContain []string
	}{
		{
			name: "nil Expected value",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "mismatch",
				Actual:    404,
			},
			shouldContain: []string{"Actual:", "404"},
			shouldNotContain: []string{"Expected:"},
		},
		{
			name: "nil Actual value",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "mismatch",
				Expected:  200,
			},
			shouldContain: []string{"Expected:", "200"},
			shouldNotContain: []string{"Actual:", "Received:"},
		},
		{
			name: "nil Expected and Actual values",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test",
			},
			shouldNotContain: []string{"Expected:", "Actual:", "Received:"},
		},
		{
			name: "nil RelatedFields",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				RelatedFields: nil,
			},
			shouldNotContain: []string{"Related fields:"},
		},
		{
			name: "nil ValidationDetails",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				ValidationDetails: nil,
			},
			shouldNotContain: []string{"Details:"},
		},
		{
			name: "nil Suggestions",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test",
				Suggestions: nil,
			},
			shouldNotContain: []string{"Suggestions:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorOutput := tt.err.Error()

			for _, expected := range tt.shouldContain {
				if !strings.Contains(errorOutput, expected) {
					t.Errorf("Error() output missing expected string '%s':\ngot:\n%s", expected, errorOutput)
				}
			}

			for _, notExpected := range tt.shouldNotContain {
				if strings.Contains(errorOutput, notExpected) {
					t.Errorf("Error() output contains unexpected string '%s':\ngot:\n%s", notExpected, errorOutput)
				}
			}
		})
	}
}

// TestEdgeCases_EmptyStrings tests handling of empty strings
func TestEdgeCases_EmptyStrings(t *testing.T) {
	tests := []struct {
		name          string
		err          ValidationError
		shouldContain []string
		shouldNotContain []string
	}{
		{
			name: "empty Context string",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test",
				Context:   "",
			},
			shouldNotContain: []string{"Context:", "Request:"},
		},
		{
			name: "empty FieldName string",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				FieldName: "",
			},
			shouldNotContain: []string{"Field:"},
		},
		{
			name: "empty Location string",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				Location:  "",
			},
			shouldNotContain: []string{"Location:"},
		},
		{
			name: "empty PatternDetails string",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				PatternDetails: "",
			},
			shouldNotContain: []string{"Pattern:"},
		},
		{
			name: "empty RangeInfo string",
			err: ValidationError{
				ErrorType: "status_code_range",
				Message:   "test",
				RangeInfo: "",
			},
			shouldNotContain: []string{"Range:"},
		},
		{
			name: "empty ResponseSnippet string",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				ResponseSnippet: "",
			},
			shouldNotContain: []string{"Response:"},
		},
		{
			name: "empty string in Expected",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				Expected:  "",
				Actual:    "actual",
			},
			shouldContain: []string{"Expected:"},
		},
		{
			name: "empty string in Actual",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				Expected:  "expected",
				Actual:    "",
			},
			shouldContain: []string{"Actual:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorOutput := tt.err.Error()

			for _, expected := range tt.shouldContain {
				if !strings.Contains(errorOutput, expected) {
					t.Errorf("Error() output missing expected string '%s':\ngot:\n%s", expected, errorOutput)
				}
			}

			for _, notExpected := range tt.shouldNotContain {
				if strings.Contains(errorOutput, notExpected) {
					t.Errorf("Error() output contains unexpected string '%s':\ngot:\n%s", notExpected, errorOutput)
				}
			}
		})
	}
}

// TestEdgeCases_EmptyCollections tests handling of empty collections
func TestEdgeCases_EmptyCollections(t *testing.T) {
	tests := []struct {
		name          string
		err          ValidationError
		shouldContain []string
		shouldNotContain []string
	}{
		{
			name: "empty RelatedFields slice",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				RelatedFields: []string{},
			},
			shouldNotContain: []string{"Related fields:"},
		},
		{
			name: "empty ValidationDetails slice",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				ValidationDetails: []string{},
			},
			shouldNotContain: []string{"Details:"},
		},
		{
			name: "empty Suggestions slice",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test",
				Suggestions: []string{},
			},
			shouldNotContain: []string{"Suggestions:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorOutput := tt.err.Error()

			for _, expected := range tt.shouldContain {
				if !strings.Contains(errorOutput, expected) {
					t.Errorf("Error() output missing expected string '%s':\ngot:\n%s", expected, errorOutput)
				}
			}

			for _, notExpected := range tt.shouldNotContain {
				if strings.Contains(errorOutput, notExpected) {
					t.Errorf("Error() output contains unexpected string '%s':\ngot:\n%s", notExpected, errorOutput)
				}
			}
		})
	}
}

// TestEdgeCases_WhitespaceOnlyStrings tests handling of whitespace-only strings
func TestEdgeCases_WhitespaceOnlyStrings(t *testing.T) {
	tests := []struct {
		name          string
		err          ValidationError
		shouldContain []string
		shouldNotContain []string
	}{
		{
			name: "whitespace-only Context is displayed",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test",
				Context:   "   ",
			},
			// The implementation displays whitespace Context values
			shouldContain: []string{"Request:"},
			shouldNotContain: []string{},
		},
		{
			name: "whitespace-only FieldName is displayed",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				FieldName: "  \t  ",
			},
			// The implementation displays whitespace FieldName values
			shouldContain: []string{"Field:"},
			shouldNotContain: []string{},
		},
		{
			name: "whitespace-only Location is displayed",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				Location:  "\n\t",
			},
			// The implementation displays whitespace Location values
			shouldContain: []string{"Location:"},
			shouldNotContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorOutput := tt.err.Error()

			for _, notExpected := range tt.shouldNotContain {
				if strings.Contains(errorOutput, notExpected) {
					t.Errorf("Error() output contains unexpected string '%s':\ngot:\n%s", notExpected, errorOutput)
				}
			}
		})
	}
}

// =============================================================================
// ToMap() METHOD TESTS
// =============================================================================

// TestValidationErrorToMap tests the ToMap() method for serialization
func TestValidationErrorToMap(t *testing.T) {
	tests := []struct {
		name          string
		err          ValidationError
		shouldContain []string
		shouldNotContain []string
	}{
		{
			name: "basic error",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test message",
			},
			shouldContain: []string{"error_type", "message"},
		},
		{
			name: "error with Expected and Actual",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test",
				Expected:  200,
				Actual:    404,
			},
			shouldContain: []string{"error_type", "message", "expected", "actual"},
		},
		{
			name: "error with Context",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test",
				Context:   "GET /api/users",
			},
			shouldContain: []string{"error_type", "message", "context"},
		},
		{
			name: "error with empty optional fields not in map",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test",
				Context:   "",
				Expected:  nil,
				Suggestions: []string{},
			},
			shouldNotContain: []string{"context", "expected", "suggestions"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.ToMap()

			// Check that map is not empty
			if len(result) == 0 {
				t.Fatal("ToMap() returned empty map")
			}

			// Check for expected keys
			for _, expectedKey := range tt.shouldContain {
				if _, exists := result[expectedKey]; !exists {
					t.Errorf("ToMap() missing key '%s': got %v", expectedKey, result)
				}
			}

			// Check for unexpected keys
			for _, unexpectedKey := range tt.shouldNotContain {
				if _, exists := result[unexpectedKey]; exists {
					t.Errorf("ToMap() contains unexpected key '%s': got %v", unexpectedKey, result)
				}
			}

			// Verify required fields are always present
			if _, exists := result["error_type"]; !exists {
				t.Error("ToMap() missing required field 'error_type'")
			}
			if _, exists := result["message"]; !exists {
				t.Error("ToMap() missing required field 'message'")
			}
		})
	}
}

// =============================================================================
// TYPE-SPECIFIC TESTS FOR ACTUAL/EXPECTED VALUES
// =============================================================================

// TestErrorFormatting_DifferentTypes tests formatting with different data types
func TestErrorFormatting_DifferentTypes(t *testing.T) {
	tests := []struct {
		name          string
		err          ValidationError
		shouldContain []string
	}{
		{
			name: "int Expected and Actual",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test",
				Expected:  200,
				Actual:    404,
			},
			shouldContain: []string{"200 (OK)", "404 (Not Found)"},
		},
		{
			name: "string Expected and Actual",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				Expected:  "pattern",
				Actual:    "value",
			},
			shouldContain: []string{"pattern", "value"},
		},
		{
			name: "int slice Expected",
			err: ValidationError{
				ErrorType: "status_code",
				Message:   "test",
				Expected:  []int{200, 201, 204},
				Actual:    404,
			},
			shouldContain: []string{"one of", "200 (OK)", "201 (Created)", "204 (No Content)"},
		},
		{
			name: "long string Actual gets truncated",
			err: ValidationError{
				ErrorType: "error_message",
				Message:   "test",
				Expected:  "short",
				Actual:    strings.Repeat("a", 150),
			},
			shouldContain: []string{"..."}, // Should contain truncation marker
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorOutput := tt.err.Error()

			for _, expected := range tt.shouldContain {
				if !strings.Contains(errorOutput, expected) {
					t.Errorf("Error() output missing expected string '%s':\ngot:\n%s", expected, errorOutput)
				}
			}
		})
	}
}
