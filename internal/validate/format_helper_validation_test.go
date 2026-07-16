package validate

import (
	"strings"
	"testing"
)

// TestValidationFormatter_BasicErrorFormatting tests basic error formatting with required fields only
func TestValidationFormatter_BasicErrorFormatting(t *testing.T) {
	tests := []struct {
		name            string
		validationType string
		expected        interface{}
		actual          interface{}
		wantErrorType   string
		wantExpected    interface{}
		wantActual      interface{}
	}{
		{
			name:            "status code validation",
			validationType: "status_code",
			expected:        200,
			actual:          404,
			wantErrorType:   "status_code",
			wantExpected:    200,
			wantActual:      404,
		},
		{
			name:            "content type validation",
			validationType: "content_type",
			expected:        "application/json",
			actual:          "text/html",
			wantErrorType:   "content_type",
			wantExpected:    "application/json",
			wantActual:      "text/html",
		},
		{
			name:            "error message validation",
			validationType: "error_message",
			expected:        "invalid.*token",
			actual:          "access_denied",
			wantErrorType:   "error_message",
			wantExpected:    "invalid.*token",
			wantActual:      "access_denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationFormatter(tt.validationType).
				WithExpected(tt.expected).
				WithActual(tt.actual).
				Format()

			if err.ErrorType != tt.wantErrorType {
				t.Errorf("ErrorType = %v, want %v", err.ErrorType, tt.wantErrorType)
			}
			if err.Expected != tt.wantExpected {
				t.Errorf("Expected = %v, want %v", err.Expected, tt.wantExpected)
			}
			if err.Actual != tt.wantActual {
				t.Errorf("Actual = %v, want %v", err.Actual, tt.wantActual)
			}
		})
	}
}

// TestValidationFormatter_OptionalFieldsIndividually tests each optional field individually
func TestValidationFormatter_OptionalFieldsIndividually(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ValidationFormatter) *ValidationFormatter
		validate func(*testing.T, ValidationError)
	}{
		{
			name: "WithContext",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithContext("GET /api/users")
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.Context != "GET /api/users" {
					t.Errorf("Context = %v, want 'GET /api/users'", err.Context)
				}
			},
		},
		{
			name: "WithResponseSnippet",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithResponseSnippet(`{"error": "not found"}`)
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.ResponseSnippet != `{"error": "not found"}` {
					t.Errorf("ResponseSnippet = %v, want '{\"error\": \"not found\"}'", err.ResponseSnippet)
				}
			},
		},
		{
			name: "WithFieldName",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithFieldName("error")
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.FieldName != "error" {
					t.Errorf("FieldName = %v, want 'error'", err.FieldName)
				}
			},
		},
		{
			name: "WithPatternDetails",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithPatternDetails("Expected pattern 'invalid.*token' not found")
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.PatternDetails != "Expected pattern 'invalid.*token' not found" {
					t.Errorf("PatternDetails = %v, want 'Expected pattern 'invalid.*token' not found'", err.PatternDetails)
				}
			},
		},
		{
			name: "WithRangeInfo",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithRangeInfo("400-499")
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.RangeInfo != "400-499" {
					t.Errorf("RangeInfo = %v, want '400-499'", err.RangeInfo)
				}
			},
		},
		{
			name: "WithValidationDetails",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithValidationDetails("Detail 1", "Detail 2")
			},
			validate: func(t *testing.T, err ValidationError) {
				if len(err.ValidationDetails) != 2 {
					t.Errorf("ValidationDetails length = %v, want 2", len(err.ValidationDetails))
				}
				if err.ValidationDetails[0] != "Detail 1" {
					t.Errorf("ValidationDetails[0] = %v, want 'Detail 1'", err.ValidationDetails[0])
				}
			},
		},
		{
			name: "WithSuggestions",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithSuggestions("Suggestion 1", "Suggestion 2")
			},
			validate: func(t *testing.T, err ValidationError) {
				if len(err.Suggestions) != 2 {
					t.Errorf("Suggestions length = %v, want 2", len(err.Suggestions))
				}
				if err.Suggestions[0] != "Suggestion 1" {
					t.Errorf("Suggestions[0] = %v, want 'Suggestion 1'", err.Suggestions[0])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := tt.setup(NewValidationFormatter("test_type").
				WithExpected("expected").
				WithActual("actual"))
			err := formatter.Format()
			tt.validate(t, err)
		})
	}
}

// TestValidationFormatter_OptionalFieldsCombination tests optional fields in combination
func TestValidationFormatter_OptionalFieldsCombination(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ValidationFormatter) *ValidationFormatter
		validate func(*testing.T, ValidationError)
	}{
		{
			name: "Context and ResponseSnippet",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithContext("POST /api/users").
					WithResponseSnippet(`{"status": "error"}`)
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.Context != "POST /api/users" {
					t.Errorf("Context = %v, want 'POST /api/users'", err.Context)
				}
				if err.ResponseSnippet != `{"status": "error"}` {
					t.Errorf("ResponseSnippet = %v, want '{\"status\": \"error\"}'", err.ResponseSnippet)
				}
			},
		},
		{
			name: "FieldName and PatternDetails",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithFieldName("message").
					WithPatternDetails("Pattern mismatch")
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.FieldName != "message" {
					t.Errorf("FieldName = %v, want 'message'", err.FieldName)
				}
				if err.PatternDetails != "Pattern mismatch" {
					t.Errorf("PatternDetails = %v, want 'Pattern mismatch'", err.PatternDetails)
				}
			},
		},
		{
			name: "All optional fields",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithContext("PUT /api/data").
					WithResponseSnippet(`{"error": "validation failed"}`).
					WithFieldName("email").
					WithPatternDetails("Invalid email pattern").
					WithRangeInfo("0-100").
					WithValidationDetails("Detail 1", "Detail 2").
					WithSuggestions("Check email format", "Verify domain")
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.Context != "PUT /api/data" {
					t.Errorf("Context = %v, want 'PUT /api/data'", err.Context)
				}
				if err.ResponseSnippet != `{"error": "validation failed"}` {
					t.Errorf("ResponseSnippet mismatch")
				}
				if err.FieldName != "email" {
					t.Errorf("FieldName = %v, want 'email'", err.FieldName)
				}
				if err.PatternDetails != "Invalid email pattern" {
					t.Errorf("PatternDetails mismatch")
				}
				if err.RangeInfo != "0-100" {
					t.Errorf("RangeInfo = %v, want '0-100'", err.RangeInfo)
				}
				if len(err.ValidationDetails) != 2 {
					t.Errorf("ValidationDetails length = %v, want 2", len(err.ValidationDetails))
				}
				if len(err.Suggestions) != 2 {
					t.Errorf("Suggestions length = %v, want 2", len(err.Suggestions))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := tt.setup(NewValidationFormatter("test_type").
				WithExpected("expected").
				WithActual("actual"))
			err := formatter.Format()
			tt.validate(t, err)
		})
	}
}

// TestValidationFormatter_EdgeCases tests edge cases like nil values, empty strings, missing fields
func TestValidationFormatter_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ValidationFormatter) *ValidationFormatter
		validate func(*testing.T, ValidationError)
	}{
		{
			name: "Nil values",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithExpected(nil).WithActual(nil)
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.Expected != nil {
					t.Errorf("Expected should be nil, got %v", err.Expected)
				}
				if err.Actual != nil {
					t.Errorf("Actual should be nil, got %v", err.Actual)
				}
			},
		},
		{
			name: "Empty strings",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithContext("").WithResponseSnippet("")
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.Context != "" {
					t.Errorf("Context should be empty, got %v", err.Context)
				}
				if err.ResponseSnippet != "" {
					t.Errorf("ResponseSnippet should be empty, got %v", err.ResponseSnippet)
				}
			},
		},
		{
			name: "Missing optional fields",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.Context != "" {
					t.Errorf("Context should be empty when not set, got %v", err.Context)
				}
				if err.ResponseSnippet != "" {
					t.Errorf("ResponseSnippet should be empty when not set")
				}
				if err.FieldName != "" {
					t.Errorf("FieldName should be empty when not set")
				}
				if err.PatternDetails != "" {
					t.Errorf("PatternDetails should be empty when not set")
				}
				if err.RangeInfo != "" {
					t.Errorf("RangeInfo should be empty when not set")
				}
				if len(err.ValidationDetails) != 0 {
					t.Errorf("ValidationDetails should be empty when not set")
				}
				// Note: Suggestions are auto-generated when not explicitly set
				// So we expect them to be non-empty
				if len(err.Suggestions) == 0 {
					t.Errorf("Suggestions should be auto-generated when not set")
				}
			},
		},
		{
			name: "Zero values",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithExpected(0).WithActual(0)
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.Expected != 0 {
					t.Errorf("Expected should be 0, got %v", err.Expected)
				}
				if err.Actual != 0 {
					t.Errorf("Actual should be 0, got %v", err.Actual)
				}
			},
		},
		{
			name: "Empty slices",
			setup: func(vf *ValidationFormatter) *ValidationFormatter {
				return vf.WithValidationDetails().WithSuggestions()
			},
			validate: func(t *testing.T, err ValidationError) {
				if len(err.ValidationDetails) != 0 {
					t.Errorf("ValidationDetails should be empty, got length %v", len(err.ValidationDetails))
				}
				if len(err.Suggestions) == 0 {
					t.Errorf("Suggestions should be auto-generated (not empty)")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := tt.setup(NewValidationFormatter("test_type"))
			err := formatter.Format()
			tt.validate(t, err)
		})
	}
}

// TestFormatStatusCodeError_Comprehensive tests the FormatStatusCodeError convenience function comprehensively
func TestFormatStatusCodeError_Comprehensive(t *testing.T) {
	tests := []struct {
		name            string
		expected        interface{}
		actual          int
		context         string
		wantErrorType   string
		wantContext     string
	}{
		{
			name:          "simple status code mismatch",
			expected:      200,
			actual:        404,
			context:       "GET /api/users",
			wantErrorType: "status_code",
			wantContext:   "GET /api/users",
		},
		{
			name:          "multiple expected codes",
			expected:      []int{200, 201, 204},
			actual:        401,
			context:       "POST /api/auth",
			wantErrorType: "status_code",
			wantContext:   "POST /api/auth",
		},
		{
			name:          "empty context",
			expected:      200,
			actual:        500,
			context:       "",
			wantErrorType: "status_code",
			wantContext:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatStatusCodeError(tt.expected, tt.actual, tt.context)

			if err.ErrorType != tt.wantErrorType {
				t.Errorf("ErrorType = %v, want %v", err.ErrorType, tt.wantErrorType)
			}
			if err.Context != tt.wantContext {
				t.Errorf("Context = %v, want %v", err.Context, tt.wantContext)
			}
			if err.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", err.Actual, tt.actual)
			}
		})
	}
}

// TestFormatErrorMessageError_Comprehensive tests the FormatErrorMessageError convenience function comprehensively
func TestFormatErrorMessageError_Comprehensive(t *testing.T) {
	tests := []struct {
		name             string
		expectedPattern  string
		actualMessage    string
		fieldName        string
		context          string
		wantErrorType    string
		wantFieldName    string
		wantContext      string
	}{
		{
			name:            "token pattern mismatch",
			expectedPattern: "invalid.*token",
			actualMessage:   "access_denied",
			fieldName:       "error",
			context:         "OAuth validation",
			wantErrorType:   "error_message",
			wantFieldName:   "error",
			wantContext:     "OAuth validation",
		},
		{
			name:            "not found pattern",
			expectedPattern: "not found",
			actualMessage:   "success",
			fieldName:       "message",
			context:         "Resource check",
			wantErrorType:   "error_message",
			wantFieldName:   "message",
			wantContext:     "Resource check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatErrorMessageError(tt.expectedPattern, tt.actualMessage, tt.fieldName, tt.context)

			if err.ErrorType != tt.wantErrorType {
				t.Errorf("ErrorType = %v, want %v", err.ErrorType, tt.wantErrorType)
			}
			if err.FieldName != tt.wantFieldName {
				t.Errorf("FieldName = %v, want %v", err.FieldName, tt.wantFieldName)
			}
			if err.Context != tt.wantContext {
				t.Errorf("Context = %v, want %v", err.Context, tt.wantContext)
			}
		})
	}
}

// TestFormatStatusCodeRangeError_Comprehensive tests the FormatStatusCodeRangeError convenience function comprehensively
func TestFormatStatusCodeRangeError_Comprehensive(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		actual        int
		context       string
		wantErrorType string
		wantRangeInfo string
	}{
		{
			name:          "4xx range with 200",
			pattern:       "4xx",
			actual:        200,
			context:       "Error response check",
			wantErrorType: "status_code_range",
			wantRangeInfo: "400-499",
		},
		{
			name:          "5xx range with 404",
			pattern:       "5xx",
			actual:        404,
			context:       "Server error check",
			wantErrorType: "status_code_range",
			wantRangeInfo: "500-599",
		},
		{
			name:          "2xx range with 500",
			pattern:       "2xx",
			actual:        500,
			context:       "Success check",
			wantErrorType: "status_code_range",
			wantRangeInfo: "200-299",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatStatusCodeRangeError(tt.pattern, tt.actual, tt.context, "status_code")

			if err.ErrorType != tt.wantErrorType {
				t.Errorf("ErrorType = %v, want %v", err.ErrorType, tt.wantErrorType)
			}
			if !strings.Contains(err.RangeInfo, tt.wantRangeInfo) {
				t.Errorf("RangeInfo = %v, want to contain %v", err.RangeInfo, tt.wantRangeInfo)
			}
		})
	}
}

// TestFormatContentTypeError_Comprehensive tests the FormatContentTypeError convenience function comprehensively
func TestFormatContentTypeError_Comprehensive(t *testing.T) {
	tests := []struct {
		name          string
		expected      string
		actual        string
		context       string
		wantErrorType string
	}{
		{
			name:          "JSON vs HTML",
			expected:      "application/json",
			actual:        "text/html",
			context:       "API response",
			wantErrorType: "content_type",
		},
		{
			name:          "JSON vs XML",
			expected:      "application/json",
			actual:        "application/xml",
			context:       "Content negotiation",
			wantErrorType: "content_type",
		},
		{
			name:          "empty context",
			expected:      "application/json",
			actual:        "text/plain",
			context:       "",
			wantErrorType: "content_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatContentTypeError(tt.expected, tt.actual, tt.context)

			if err.ErrorType != tt.wantErrorType {
				t.Errorf("ErrorType = %v, want %v", err.ErrorType, tt.wantErrorType)
			}
			if err.Expected != tt.expected {
				t.Errorf("Expected = %v, want %v", err.Expected, tt.expected)
			}
			if err.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", err.Actual, tt.actual)
			}
		})
	}
}

// TestFormatCustomValidationError_Comprehensive tests the FormatCustomValidationError function with options comprehensively
func TestFormatCustomValidationError_Comprehensive(t *testing.T) {
	tests := []struct {
		name          string
		validationType string
		expected      interface{}
		actual        interface{}
		options       []FormatOption
		validate      func(*testing.T, ValidationError)
	}{
		{
			name:           "no options",
			validationType: "custom_type",
			expected:       "expected_value",
			actual:         "actual_value",
			options:        nil,
			validate: func(t *testing.T, err ValidationError) {
				if err.ErrorType != "custom_type" {
					t.Errorf("ErrorType = %v, want 'custom_type'", err.ErrorType)
				}
				if err.Context != "" {
					t.Errorf("Context should be empty, got %v", err.Context)
				}
			},
		},
		{
			name:           "with context option",
			validationType: "custom_type",
			expected:       "expected_value",
			actual:         "actual_value",
			options:        []FormatOption{WithContext("custom context")},
			validate: func(t *testing.T, err ValidationError) {
				if err.Context != "custom context" {
					t.Errorf("Context = %v, want 'custom context'", err.Context)
				}
			},
		},
		{
			name:           "with multiple options",
			validationType: "custom_type",
			expected:       "expected_value",
			actual:         "actual_value",
			options: []FormatOption{
				WithContext("test context"),
				WithResponseSnippet(`{"test": "data"}`),
				WithFieldName("test_field"),
				WithSuggestions("Custom suggestion 1", "Custom suggestion 2"),
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.Context != "test context" {
					t.Errorf("Context mismatch")
				}
				if err.ResponseSnippet != `{"test": "data"}` {
					t.Errorf("ResponseSnippet mismatch")
				}
				if err.FieldName != "test_field" {
					t.Errorf("FieldName = %v, want 'test_field'", err.FieldName)
				}
				if len(err.Suggestions) != 2 {
					t.Errorf("Suggestions length = %v, want 2", len(err.Suggestions))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatCustomValidationError(tt.validationType, tt.expected, tt.actual, tt.options...)
			tt.validate(t, err)
		})
	}
}

// TestValidationFormatter_ChainedMethods tests method chaining behavior
func TestValidationFormatter_ChainedMethods(t *testing.T) {
	// Test that methods return the formatter for chaining
	formatter := NewValidationFormatter("test_type")

	if formatter == nil {
		t.Fatal("NewValidationFormatter returned nil")
	}

	// Test chaining works
	result := formatter.
		WithExpected("expected").
		WithActual("actual").
		WithContext("context").
		WithResponseSnippet("snippet").
		WithFieldName("field").
		WithPatternDetails("pattern").
		WithRangeInfo("range").
		WithValidationDetails("detail1", "detail2").
		WithSuggestions("suggestion1", "suggestion2")

	if result == nil {
		t.Fatal("Method chaining returned nil")
	}

	// Verify the formatter was modified correctly
	err := result.Format()
	if err.ErrorType != "test_type" {
		t.Errorf("ErrorType = %v, want 'test_type'", err.ErrorType)
	}
	if err.Context != "context" {
		t.Errorf("Context = %v, want 'context'", err.Context)
	}
	if len(err.Suggestions) != 2 {
		t.Errorf("Suggestions length = %v, want 2", len(err.Suggestions))
	}
}

// TestValidationFormatter_Immutability tests that chained calls affect the same instance (mutable pattern)
func TestValidationFormatter_Immutability(t *testing.T) {
	// ValidationFormatter uses a mutable pattern - With methods modify the receiver
	baseFormatter := NewValidationFormatter("test_type").
		WithExpected("base_expected").
		WithActual("base_actual")

	// Add more fields to the same formatter
	baseFormatter = baseFormatter.
		WithContext("new_context").
		WithFieldName("new_field")

	// Format the formatter (should have all fields including context/field name)
	err := baseFormatter.Format()
	if err.Context != "new_context" {
		t.Errorf("Formatter should have context 'new_context', got %v", err.Context)
	}
	if err.FieldName != "new_field" {
		t.Errorf("Formatter should have field name 'new_field', got %v", err.FieldName)
	}
	if err.Expected != "base_expected" {
		t.Errorf("Expected = %v, want 'base_expected'", err.Expected)
	}
	if err.Actual != "base_actual" {
		t.Errorf("Actual = %v, want 'base_actual'", err.Actual)
	}
}

// TestValidationFormatter_MultipleWithSameField tests calling With methods multiple times
func TestValidationFormatter_MultipleWithSameField(t *testing.T) {
	formatter := NewValidationFormatter("test_type").
		WithExpected("first_expected").
		WithExpected("second_expected").
		WithActual("first_actual").
		WithActual("second_actual").
		WithContext("first_context").
		WithContext("second_context")

	err := formatter.Format()

	// Last call should win
	if err.Expected != "second_expected" {
		t.Errorf("Expected = %v, want 'second_expected' (last call should win)", err.Expected)
	}
	if err.Actual != "second_actual" {
		t.Errorf("Actual = %v, want 'second_actual' (last call should win)", err.Actual)
	}
	if err.Context != "second_context" {
		t.Errorf("Context = %v, want 'second_context' (last call should win)", err.Context)
	}
}

// TestValidationFormatter_AppendBehavior tests that WithValidationDetails and WithSuggestions append
func TestValidationFormatter_AppendBehavior(t *testing.T) {
	formatter := NewValidationFormatter("test_type").
		WithValidationDetails("detail1").
		WithValidationDetails("detail2").
		WithSuggestions("suggestion1").
		WithSuggestions("suggestion2")

	err := formatter.Format()

	// Should append, not replace
	if len(err.ValidationDetails) != 2 {
		t.Errorf("ValidationDetails length = %v, want 2", len(err.ValidationDetails))
	}
	if len(err.Suggestions) != 2 {
		t.Errorf("Suggestions length = %v, want 2", len(err.Suggestions))
	}
	if err.ValidationDetails[0] != "detail1" {
		t.Errorf("First detail should be 'detail1', got %v", err.ValidationDetails[0])
	}
	if err.ValidationDetails[1] != "detail2" {
		t.Errorf("Second detail should be 'detail2', got %v", err.ValidationDetails[1])
	}
}

// TestFormatOption_WithSeverityOverride tests the WithSeverityOverride option
func TestFormatOption_WithSeverityOverride(t *testing.T) {
	err := FormatCustomValidationError(
		"test_type",
		"expected",
		"actual",
		WithSeverityOverride(SeverityCritical),
	)

	// Note: This option exists in the config but doesn't directly affect
	// the ValidationError struct in the basic Format() call
	// This test verifies the option can be applied without error
	if err.ErrorType != "test_type" {
		t.Errorf("ErrorType = %v, want 'test_type'", err.ErrorType)
	}
}

// TestFormatOption_WithCategoryHint tests the WithCategoryHint option
func TestFormatOption_WithCategoryHint(t *testing.T) {
	err := FormatCustomValidationError(
		"test_type",
		"expected",
		"actual",
		WithCategoryHint(CategoryValidation),
	)

	// Note: This option exists in the config but doesn't directly affect
	// the ValidationError struct in the basic Format() call
	// This test verifies the option can be applied without error
	if err.ErrorType != "test_type" {
		t.Errorf("ErrorType = %v, want 'test_type'", err.ErrorType)
	}
}

// TestFormatOption_WithStatusCode tests the WithStatusCode option
func TestFormatOption_WithStatusCode(t *testing.T) {
	err := FormatCustomValidationError(
		"test_type",
		"expected",
		"actual",
		WithStatusCode(404),
	)

	// Note: This option exists in the config but doesn't directly affect
	// the ValidationError struct in the basic Format() call
	// This test verifies the option can be applied without error
	if err.ErrorType != "test_type" {
		t.Errorf("ErrorType = %v, want 'test_type'", err.ErrorType)
	}
}

// TestFormatOption_WithTimeout tests the WithTimeout option
func TestFormatOption_WithTimeout(t *testing.T) {
	err := FormatCustomValidationError(
		"test_type",
		"expected",
		"actual",
		WithTimeout(5000),
	)

	// Note: This option exists in the config but doesn't directly affect
	// the ValidationError struct in the basic Format() call
	// This test verifies the option can be applied without error
	if err.ErrorType != "test_type" {
		t.Errorf("ErrorType = %v, want 'test_type'", err.ErrorType)
	}
}

// TestFormatOption_WithSecurityContext tests the WithSecurityContext option
func TestFormatOption_WithSecurityContext(t *testing.T) {
	err := FormatCustomValidationError(
		"test_type",
		"expected",
		"actual",
		WithSecurityContext("authentication"),
	)

	// Note: This option exists in the config but doesn't directly affect
	// the ValidationError struct in the basic Format() call
	// This test verifies the option can be applied without error
	if err.ErrorType != "test_type" {
		t.Errorf("ErrorType = %v, want 'test_type'", err.ErrorType)
	}
}

// TestNewValidationFormatter tests the NewValidationFormatter constructor
func TestNewValidationFormatter(t *testing.T) {
	tests := []struct {
		name            string
		validationType string
	}{
		{
			name:            "standard type",
			validationType: "status_code",
		},
		{
			name:            "custom type",
			validationType: "custom_validation",
		},
		{
			name:            "empty type",
			validationType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewValidationFormatter(tt.validationType)
			if formatter == nil {
				t.Fatal("NewValidationFormatter returned nil")
			}
			if formatter.validationType != tt.validationType {
				t.Errorf("validationType = %v, want %v", formatter.validationType, tt.validationType)
			}
		})
	}
}

// TestValidationError_FormatMethod tests the Format method of ValidationFormatter
func TestValidationError_FormatMethod(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *ValidationFormatter
		validate func(*testing.T, ValidationError)
	}{
		{
			name: "minimal setup",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("test_type")
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.ErrorType != "test_type" {
					t.Errorf("ErrorType = %v, want 'test_type'", err.ErrorType)
				}
			},
		},
		{
			name: "with all fields",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("full_type").
					WithExpected("expected").
					WithActual("actual").
					WithContext("context").
					WithResponseSnippet("snippet").
					WithFieldName("field").
					WithPatternDetails("pattern").
					WithRangeInfo("range").
					WithValidationDetails("detail").
					WithSuggestions("suggestion")
			},
			validate: func(t *testing.T, err ValidationError) {
				if err.ErrorType != "full_type" {
					t.Errorf("ErrorType = %v, want 'full_type'", err.ErrorType)
				}
				if err.Expected != "expected" {
					t.Errorf("Expected = %v, want 'expected'", err.Expected)
				}
				if err.Actual != "actual" {
					t.Errorf("Actual = %v, want 'actual'", err.Actual)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := tt.setup()
			err := formatter.Format()
			tt.validate(t, err)
		})
	}
}

// TestValidationFormatter_CustomSuggestionsOverrideDetailed tests that custom suggestions override auto-generated
func TestValidationFormatter_CustomSuggestionsOverrideDetailed(t *testing.T) {
	// Without custom suggestions (should auto-generate)
	formatter1 := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404)
	err1 := formatter1.Format()

	// Should have auto-generated suggestions
	if len(err1.Suggestions) == 0 {
		t.Error("Expected auto-generated suggestions, got none")
	}

	// With custom suggestions (should override)
	formatter2 := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithSuggestions("Custom suggestion 1", "Custom suggestion 2")
	err2 := formatter2.Format()

	// Should have custom suggestions
	if len(err2.Suggestions) != 2 {
		t.Errorf("Suggestions length = %v, want 2", len(err2.Suggestions))
	}
	if err2.Suggestions[0] != "Custom suggestion 1" {
		t.Errorf("First suggestion = %v, want 'Custom suggestion 1'", err2.Suggestions[0])
	}
	if err2.Suggestions[1] != "Custom suggestion 2" {
		t.Errorf("Second suggestion = %v, want 'Custom suggestion 2'", err2.Suggestions[1])
	}
}

// TestValidationFormatter_EmptyCustomSuggestions tests empty custom suggestions slice
func TestValidationFormatter_EmptyCustomSuggestions(t *testing.T) {
	formatter := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithSuggestions() // Empty slice

	err := formatter.Format()

	// Empty slice should trigger auto-generation
	if len(err.Suggestions) == 0 {
		t.Error("Expected auto-generated suggestions when empty slice provided, got none")
	}
}

// TestValidationFormatter_NilAndZeroValueHandling tests handling of nil and zero values
func TestValidationFormatter_NilAndZeroValueHandling(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
	}{
		{
			name:     "nil values",
			expected: nil,
			actual:   nil,
		},
		{
			name:     "zero int",
			expected: 0,
			actual:   0,
		},
		{
			name:     "zero float",
			expected: 0.0,
			actual:   0.0,
		},
		{
			name:     "empty string",
			expected: "",
			actual:   "",
		},
		{
			name:     "false boolean",
			expected: false,
			actual:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationFormatter("test_type").
				WithExpected(tt.expected).
				WithActual(tt.actual).
				Format()

			if err.Expected != tt.expected {
				t.Errorf("Expected = %v, want %v", err.Expected, tt.expected)
			}
			if err.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", err.Actual, tt.actual)
			}
		})
	}
}
