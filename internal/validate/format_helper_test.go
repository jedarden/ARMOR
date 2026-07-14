package validate

import (
	"fmt"
	"strings"
	"testing"
)

// TestValidationFormatter_BasicFunctionality verifies that the ValidationFormatter
// produces properly formatted validation errors with expected structure.
func TestValidationFormatter_BasicFunctionality(t *testing.T) {
	tests := []struct {
		name           string
		validationType string
		expected       interface{}
		actual         interface{}
		mustContain    []string
	}{
		{
			name:           "status code validation",
			validationType: "status_code",
			expected:       200,
			actual:         404,
			mustContain:    []string{"status_code", "200", "404"},
		},
		{
			name:           "error message validation",
			validationType: "error_message",
			expected:       "invalid.*token",
			actual:         "access_denied",
			mustContain:    []string{"error_message", "invalid.*token", "access_denied"},
		},
		{
			name:           "content type validation",
			validationType: "content_type",
			expected:       "application/json",
			actual:         "text/html",
			mustContain:    []string{"content_type", "application/json", "text/html"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationFormatter(tt.validationType).
				WithExpected(tt.expected).
				WithActual(tt.actual).
				Format()

			errMsg := err.Error()
			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s', got: %s", required, errMsg)
				}
			}
		})
	}
}

// TestValidationFormatter_WithAllOptions verifies that all formatter options work correctly.
func TestValidationFormatter_WithAllOptions(t *testing.T) {
	err := NewValidationFormatter("custom_validation").
		WithExpected("expected_value").
		WithActual("actual_value").
		WithContext("test context").
		WithResponseSnippet(`{"field": "actual_value"}`).
		WithFieldName("test_field").
		WithPatternDetails("pattern 'expected.*' did not match").
		WithRangeInfo("100-199").
		WithValidationDetails("Detail 1", "Detail 2").
		WithSuggestions("Custom suggestion 1", "Custom suggestion 2").
		Format()

	errMsg := err.Error()

	requiredStrings := []string{
		"custom_validation",
		"expected_value",
		"actual_value",
		"test context",
		`{"field": "actual_value"}`,
		"test_field",
		"pattern 'expected.*' did not match",
		"100-199",
		"Detail 1",
		"Detail 2",
		"Custom suggestion 1",
		"Custom suggestion 2",
	}

	for _, required := range requiredStrings {
		if !strings.Contains(errMsg, required) {
			t.Errorf("Expected error message to contain '%s', got: %s", required, errMsg)
		}
	}

	// Verify custom suggestions were used instead of auto-generated
	if !strings.Contains(errMsg, "Custom suggestion 1") {
		t.Error("Expected custom suggestions to be used")
	}
}

// TestValidationFormatter_WithValidationDetails verifies that validation details
// are properly added to the error message.
func TestValidationFormatter_WithValidationDetails(t *testing.T) {
	details := []string{
		"Status code 404 is outside range 200-299",
		"Expected success status code",
		"Request method: GET",
	}

	err := NewValidationFormatter("status_code_range").
		WithExpected("2xx").
		WithActual(404).
		WithValidationDetails(details...).
		Format()

	errMsg := err.Error()

	// Verify all details appear in the error message
	for _, detail := range details {
		if !strings.Contains(errMsg, detail) {
			t.Errorf("Expected error message to contain detail '%s', got: %s", detail, errMsg)
		}
	}

	// Verify details are under the "Details:" section
	lines := strings.Split(errMsg, "\n")
	inDetailsSection := false
	foundDetailCount := 0

	for _, line := range lines {
		// Stop counting when we reach another section (like "Suggestions:")
		if inDetailsSection && strings.Contains(line, ":") && !strings.HasPrefix(line, "    ") {
			break
		}
		if strings.Contains(line, "Details:") {
			inDetailsSection = true
			continue
		}
		if inDetailsSection && strings.HasPrefix(line, "    -") {
			foundDetailCount++
		}
	}

	if foundDetailCount != len(details) {
		t.Errorf("Expected %d detail lines, found %d", len(details), foundDetailCount)
	}
}

// TestValidationFormatter_Chaining verifies that method chaining works correctly.
func TestValidationFormatter_Chaining(t *testing.T) {
	// Test that methods return the formatter for chaining
	formatter := NewValidationFormatter("test")

	if formatter == nil {
		t.Fatal("NewValidationFormatter returned nil")
	}

	// Verify each With* method returns a non-nil formatter
	result := formatter.
		WithExpected("test").
		WithActual("test").
		WithContext("test").
		WithResponseSnippet("test").
		WithFieldName("test").
		WithPatternDetails("test").
		WithRangeInfo("test").
		WithValidationDetails("test").
		WithSuggestions("test")

	if result == nil {
		t.Error("Method chaining returned nil")
	}

	// Verify the final Format() call works
	validationErr := result.Format()
	if validationErr.Error() == "" {
		t.Error("Format() returned ValidationError with empty Error() string")
	}
}

// TestFormatStatusCodeError verifies that the convenience function correctly
// formats status code validation errors.
func TestFormatStatusCodeError(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   int
		context  string
		mustContain []string
	}{
		{
			name:     "single expected code",
			expected: 200,
			actual:   404,
			context:  "GET /api/users",
			mustContain: []string{
				"status_code",
				"200",
				"404",
				"GET /api/users",
			},
		},
		{
			name:     "multiple expected codes",
			expected: []int{200, 201, 204},
			actual:   500,
			context:  "POST /api/orders",
			mustContain: []string{
				"status_code",
				"200",
				"201",
				"204",
				"500",
				"POST /api/orders",
			},
		},
		{
			name:     "no context provided",
			expected: 200,
			actual:   403,
			context:  "",
			mustContain: []string{
				"status_code",
				"200",
				"403",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatStatusCodeError(tt.expected, tt.actual, tt.context)
			errMsg := err.Error()

			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s', got: %s", required, errMsg)
				}
			}

			// Verify suggestions are present
			if !strings.Contains(errMsg, "Suggestions:") {
				t.Error("Expected suggestions section in error message")
			}
		})
	}
}

// TestFormatErrorMessageError verifies that the convenience function correctly
// formats error message validation errors.
func TestFormatErrorMessageError(t *testing.T) {
	tests := []struct {
		name           string
		expectedPattern string
		actualMessage  string
		fieldName      string
		context        string
		mustContain    []string
	}{
		{
			name:           "regex pattern mismatch",
			expectedPattern: "invalid.*token",
			actualMessage:  "access_denied",
			fieldName:      "error",
			context:       "OAuth token validation",
			mustContain: []string{
				"error_message",
				"invalid.*token",
				"access_denied",
				"error",
				"OAuth token validation",
			},
		},
		{
			name:           "substring pattern mismatch",
			expectedPattern: "not found",
			actualMessage:  "internal server error",
			fieldName:      "message",
			context:       "resource access",
			mustContain: []string{
				"error_message",
				"not found",
				"internal server error",
				"message",
				"resource access",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatErrorMessageError(tt.expectedPattern, tt.actualMessage, tt.fieldName, tt.context)
			errMsg := err.Error()

			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s', got: %s", required, errMsg)
				}
			}

			// Verify field name is present
			if !strings.Contains(errMsg, "Field:") {
				t.Error("Expected field section in error message")
			}
		})
	}
}

// TestFormatStatusCodeRangeError verifies that the convenience function correctly
// formats status code range validation errors.
func TestFormatStatusCodeRangeError(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		actual      int
		context     string
		mustContain []string
		mustNotContain []string
	}{
		{
			name:    "4xx range mismatch",
			pattern: "4xx",
			actual:  200,
			context: "error response check",
			mustContain: []string{
				"status_code_range",
				"400-499",
				"200",
				"Client Error",
			},
		},
		{
			name:    "5xx range mismatch",
			pattern: "5xx",
			actual:  404,
			context: "server error test",
			mustContain: []string{
				"status_code_range",
				"500-599",
				"404",
				"Server Error",
			},
		},
		{
			name:    "2xx range mismatch",
			pattern: "2xx",
			actual:  500,
			context: "success check",
			mustContain: []string{
				"status_code_range",
				"200-299",
				"500",
				"Success",
			},
		},
		{
			name:    "invalid pattern",
			pattern: "invalid",
			actual:  200,
			context: "invalid pattern test",
			mustContain: []string{
				"status_code_range",
				"Pattern error",
			},
			mustNotContain: []string{
				"Range:",
				"200-299",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatStatusCodeRangeError(tt.pattern, tt.actual, tt.context)
			errMsg := err.Error()

			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s', got: %s", required, errMsg)
				}
			}

			// Check for strings that should NOT be present
			for _, prohibited := range tt.mustNotContain {
				if strings.Contains(errMsg, prohibited) {
					t.Errorf("Expected error message to NOT contain '%s', got: %s", prohibited, errMsg)
				}
			}

			// Verify range info is present (for valid patterns)
			if tt.pattern == "4xx" || tt.pattern == "5xx" || tt.pattern == "2xx" {
				if !strings.Contains(errMsg, "Range:") {
					t.Error("Expected range section in error message for valid pattern")
				}
			}
		})
	}
}

// TestFormatContentTypeError verifies that the convenience function correctly
// formats Content-Type validation errors.
func TestFormatContentTypeError(t *testing.T) {
	tests := []struct {
		name        string
		expected    string
		actual      string
		context     string
		mustContain []string
	}{
		{
			name:     "JSON vs HTML",
			expected: "application/json",
			actual:   "text/html",
			context:  "API response",
			mustContain: []string{
				"content_type",
				"application/json",
				"text/html",
				"API response",
			},
		},
		{
			name:     "JSON vs XML",
			expected: "application/json",
			actual:   "application/xml",
			context:  "data format check",
			mustContain: []string{
				"content_type",
				"application/json",
				"application/xml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatContentTypeError(tt.expected, tt.actual, tt.context)
			errMsg := err.Error()

			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s', got: %s", required, errMsg)
				}
			}

			// Verify content-type specific suggestions
			if !strings.Contains(errMsg, "Content-Type") {
				t.Error("Expected Content-Type specific suggestions")
			}
		})
	}
}

// TestFormatCustomValidationError verifies that the custom validation error
// function works with various option combinations.
func TestFormatCustomValidationError(t *testing.T) {
	tests := []struct {
		name           string
		validationType string
		expected       interface{}
		actual         interface{}
		options        []FormatOption
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:           "with context only",
			validationType: "custom_field",
			expected:       "value1",
			actual:         "value2",
			options:        []FormatOption{WithContext("test context")},
			mustContain:    []string{"custom_field", "value1", "value2", "test context"},
			mustNotContain: []string{"Response:", "Field:", "Pattern:", "Range:"},
		},
		{
			name:           "with response snippet",
			validationType: "custom_validation",
			expected:       "expected",
			actual:         "actual",
			options:        []FormatOption{WithResponseSnippet(`{"key": "actual"}`)},
			mustContain:    []string{"custom_validation", `{"key": "actual"}`, "Response:"},
			mustNotContain: []string{"Field:", "Pattern:", "Range:"},
		},
		{
			name:           "with field name",
			validationType: "field_validation",
			expected:       "required",
			actual:         "missing",
			options:        []FormatOption{WithFieldName("user_id")},
			mustContain:    []string{"field_validation", "user_id", "Field:"},
			mustNotContain: []string{"Response:", "Pattern:", "Range:"},
		},
		{
			name:           "with pattern details",
			validationType: "pattern_validation",
			expected:       "^[A-Z]{2,}$",
			actual:         "ab",
			options:        []FormatOption{WithPatternDetails("regex pattern for uppercase letters")},
			mustContain:    []string{"pattern_validation", "Pattern:", "regex pattern"},
			mustNotContain: []string{"Response:", "Field:", "Range:"},
		},
		{
			name:           "with range info",
			validationType: "range_validation",
			expected:       "1-100",
			actual:         150,
			options:        []FormatOption{WithRangeInfo("1-100 (numeric range)")},
			mustContain:    []string{"range_validation", "Range:", "1-100", "numeric"},
			mustNotContain: []string{"Response:", "Field:", "Pattern:"},
		},
		{
			name:           "with validation details",
			validationType: "detailed_validation",
			expected:       "valid",
			actual:         "invalid",
			options: []FormatOption{
				WithValidationDetails("Check 1 failed", "Check 2 failed", "Check 3 passed"),
			},
			mustContain:    []string{"detailed_validation", "Details:", "Check 1", "Check 2", "Check 3"},
			mustNotContain: []string{"Response:", "Field:", "Pattern:", "Range:"},
		},
		{
			name:           "with custom suggestions",
			validationType: "suggested_validation",
			expected:       "expected_value",
			actual:         "actual_value",
			options: []FormatOption{
				WithSuggestions("Custom suggestion 1", "Custom suggestion 2", "Custom suggestion 3"),
			},
			mustContain:    []string{"suggested_validation", "Suggestions:", "Custom suggestion 1", "Custom suggestion 2", "Custom suggestion 3"},
			mustNotContain: []string{"Response:", "Field:", "Pattern:", "Range:"},
		},
		{
			name:           "with all options",
			validationType: "comprehensive_validation",
			expected:       "expected",
			actual:         "actual",
			options: []FormatOption{
				WithContext("full context"),
				WithResponseSnippet(`{"result": "actual"}`),
				WithFieldName("custom_field"),
				WithPatternDetails("pattern details"),
				WithRangeInfo("range info"),
				WithValidationDetails("detail 1", "detail 2"),
				WithSuggestions("suggestion 1", "suggestion 2"),
			},
			mustContain: []string{
				"comprehensive_validation",
				"expected",
				"actual",
				"full context",
				"Response:",
				"Field:",
				"custom_field",
				"Pattern:",
				"Range:",
				"Details:",
				"detail 1",
				"detail 2",
				"Suggestions:",
				"suggestion 1",
				"suggestion 2",
			},
			mustNotContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatCustomValidationError(tt.validationType, tt.expected, tt.actual, tt.options...)
			errMsg := err.Error()

			// Check for required content
			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s', got: %s", required, errMsg)
				}
			}

			// Check for prohibited content
			for _, prohibited := range tt.mustNotContain {
				if strings.Contains(errMsg, prohibited) {
					t.Errorf("Expected error message to NOT contain '%s', got: %s", prohibited, errMsg)
				}
			}
		})
	}
}

// TestValidationFormatter_AutoSuggestions verifies that auto-generated suggestions
// are used when no custom suggestions are provided.
func TestValidationFormatter_AutoSuggestions(t *testing.T) {
	tests := []struct {
		name                string
		validationType      string
		expected            interface{}
		actual              interface{}
		expectedSuggestion  string
	}{
		{
			name:               "404 status code",
			validationType:     "status_code",
			expected:           200,
			actual:             404,
			expectedSuggestion: "Verify the endpoint URL is correct",
		},
		{
			name:               "401 status code",
			validationType:     "status_code",
			expected:           200,
			actual:             401,
			expectedSuggestion: "authentication",
		},
		{
			name:               "500 status code",
			validationType:     "status_code",
			expected:           200,
			actual:             500,
			expectedSuggestion: "retry",
		},
		{
			name:               "token error message",
			validationType:     "error_message",
			expected:           "invalid.*token",
			actual:             "expired",
			expectedSuggestion: "token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't provide custom suggestions - should use auto-generated
			err := NewValidationFormatter(tt.validationType).
				WithExpected(tt.expected).
				WithActual(tt.actual).
				Format()

			errMsg := err.Error()

			if !strings.Contains(errMsg, tt.expectedSuggestion) {
				t.Errorf("Expected auto-generated suggestion to contain '%s', got: %s", tt.expectedSuggestion, errMsg)
			}
		})
	}
}

// TestValidationFormatter_CustomSuggestionsOverride verifies that custom
// suggestions override auto-generated ones.
func TestValidationFormatter_CustomSuggestionsOverride(t *testing.T) {
	customSuggestions := []string{
		"Check API endpoint configuration",
		"Verify service deployment",
		"Review network connectivity",
	}

	err := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithSuggestions(customSuggestions...).
		Format()

	errMsg := err.Error()

	// Verify custom suggestions are present
	for _, suggestion := range customSuggestions {
		if !strings.Contains(errMsg, suggestion) {
			t.Errorf("Expected custom suggestion '%s' to be present, got: %s", suggestion, errMsg)
		}
	}

	// Verify typical auto-generated 404 suggestions are NOT present
	autoSuggestions := []string{
		"Verify the endpoint URL is correct",
		"Check if the resource ID or identifier exists",
	}

	for _, autoSuggestion := range autoSuggestions {
		if strings.Contains(errMsg, autoSuggestion) {
			t.Errorf("Expected custom suggestions to override, but found auto-generated suggestion '%s'", autoSuggestion)
		}
	}
}

// TestFormatOption_Chaining verifies that multiple FormatOptions can be combined.
func TestFormatOption_Chaining(t *testing.T) {
	err := FormatCustomValidationError(
		"multi_option_test",
		"expected",
		"actual",
		WithContext("context 1"),
		WithContext("context 2"), // This should override context 1
		WithValidationDetails("detail 1"),
		WithValidationDetails("detail 2"), // This should add to details
		WithSuggestions("suggestion 1"),
		WithSuggestions("suggestion 2"), // This should add to suggestions
	)

	errMsg := err.Error()

	// Last WithContext should win (context 2)
	if !strings.Contains(errMsg, "context 2") {
		t.Error("Expected last WithContext to override previous value")
	}

	// WithValidationDetails and WithSuggestions should accumulate
	if !strings.Contains(errMsg, "detail 1") || !strings.Contains(errMsg, "detail 2") {
		t.Error("Expected WithValidationDetails to accumulate")
	}

	if !strings.Contains(errMsg, "suggestion 1") || !strings.Contains(errMsg, "suggestion 2") {
		t.Error("Expected WithSuggestions to accumulate")
	}
}

// TestValidationFormatter_ErrorInterface verifies that the ValidationError
// properly implements the error interface.
func TestValidationFormatter_ErrorInterface(t *testing.T) {
	err := NewValidationFormatter("test").
		WithExpected("expected").
		WithActual("actual").
		Format()

	// Verify it implements error interface
	if err.Error() == "" {
		t.Error("Expected Error() to return non-empty string")
	}

	// Verify it can be used as an error
	var asError error = err
	if asError == nil {
		t.Error("Expected ValidationError to be usable as error type")
	}
}

// TestValidationFormatter_ValidationTypeCategories verifies that all common
// validation types are properly handled.
func TestValidationFormatter_ValidationTypeCategories(t *testing.T) {
	validationTypes := []string{
		"status_code",
		"error_message",
		"content_type",
		"status_code_range",
		"custom_validation",
	}

	for _, vt := range validationTypes {
		t.Run(vt, func(t *testing.T) {
			err := NewValidationFormatter(vt).
				WithExpected("expected").
				WithActual("actual").
				Format()

			errMsg := err.Error()

			// Verify validation type appears in error message
			if !strings.Contains(errMsg, vt) {
				t.Errorf("Expected error message to contain validation type '%s'", vt)
			}

			// Verify "validation failed" suffix is present
			expectedPrefix := vt + " validation failed"
			if !strings.Contains(errMsg, expectedPrefix) {
				t.Errorf("Expected error message to contain '%s'", expectedPrefix)
			}
		})
	}
}

// TestValidationFormatter_EmptyValues verifies that the formatter handles
// empty and nil values gracefully.
func TestValidationFormatter_EmptyValues(t *testing.T) {
	tests := []struct {
		name     string
		builder  *ValidationFormatter
		shouldPanic bool
	}{
		{
			name:    "empty expected value",
			builder: NewValidationFormatter("test").WithActual("actual"),
			shouldPanic: false,
		},
		{
			name:    "empty actual value",
			builder: NewValidationFormatter("test").WithExpected("expected"),
			shouldPanic: false,
		},
		{
			name:    "empty string values",
			builder: NewValidationFormatter("test").WithExpected("").WithActual(""),
			shouldPanic: false,
		},
		{
			name:    "nil expected value",
			builder: NewValidationFormatter("test").WithExpected(nil).WithActual("actual"),
			shouldPanic: false,
		},
		{
			name:    "nil actual value",
			builder: NewValidationFormatter("test").WithExpected("expected").WithActual(nil),
			shouldPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if tt.shouldPanic {
						// Expected to panic
						return
					}
					t.Errorf("Unexpected panic: %v", r)
				}
			}()

			err := tt.builder.Format()
			if err.Error() == "" {
				t.Error("Expected Error() to return non-empty string even with empty values")
			}
		})
	}
}

// =============================================================================
// BASIC FormatError FUNCTION TESTS
// =============================================================================

// TestFormatError_BasicFormatting verifies that FormatError produces
// correctly formatted error messages with consistent structure.
func TestFormatError_BasicFormatting(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		expected  string
	}{
		{
			name:      "status code error without field",
			errorType: "status_code",
			message:   "Expected 200 but got 404",
			fieldName: "",
			expected:  "[status_code] Expected 200 but got 404",
		},
		{
			name:      "error message with field name",
			errorType: "error_message",
			message:   "Pattern not found in response",
			fieldName: "response",
			expected:  "[error_message] response: Pattern not found in response",
		},
		{
			name:      "content type error without field",
			errorType: "content_type",
			message:   "Expected application/json but got text/html",
			fieldName: "",
			expected:  "[content_type] Expected application/json but got text/html",
		},
		{
			name:      "validation error with field",
			errorType: "validation",
			message:   "Field 'email' is required",
			fieldName: "email",
			expected:  "[validation] email: Field 'email' is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.fieldName != "" {
				result = FormatError(tt.errorType, tt.message, tt.fieldName)
			} else {
				result = FormatError(tt.errorType, tt.message)
			}
			if result != tt.expected {
				t.Errorf("FormatError() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestFormatError_EmptyErrorType verifies that FormatError handles
// empty error type gracefully by using the default "error" type.
func TestFormatError_EmptyErrorType(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		expected  string
	}{
		{
			name:      "empty error type with message",
			errorType: "",
			message:   "Something went wrong",
			fieldName: "",
			expected:  "[error] Something went wrong",
		},
		{
			name:      "empty error type with message and field",
			errorType: "",
			message:   "Field is required",
			fieldName: "email",
			expected:  "[error] email: Field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.fieldName != "" {
				result = FormatError(tt.errorType, tt.message, tt.fieldName)
			} else {
				result = FormatError(tt.errorType, tt.message)
			}
			if result != tt.expected {
				t.Errorf("FormatError() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestFormatError_EmptyMessage verifies that FormatError handles
// empty message gracefully by using a default message or field-based fallback.
func TestFormatError_EmptyMessage(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		expected  string
	}{
		{
			name:      "empty message without field",
			errorType: "status_code",
			message:   "",
			fieldName: "",
			expected:  "[status_code] (no message provided)",
		},
		{
			name:      "empty message with field name",
			errorType: "validation",
			message:   "",
			fieldName: "email",
			expected:  "[validation] email: email validation failed",
		},
		{
			name:      "whitespace message without field",
			errorType: "validation",
			message:   "   ",
			fieldName: "",
			expected:  "[validation]    ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.fieldName != "" {
				result = FormatError(tt.errorType, tt.message, tt.fieldName)
			} else {
				result = FormatError(tt.errorType, tt.message)
			}
			if result != tt.expected {
				t.Errorf("FormatError() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestFormatError_BothEmpty verifies that FormatError handles
// both error type and message being empty gracefully.
func TestFormatError_BothEmpty(t *testing.T) {
	result := FormatError("", "")
	expected := "[error] (no message provided)"

	if result != expected {
		t.Errorf("FormatError() = %q, want %q", result, expected)
	}

	// Test with both empty and field name provided
	resultWithField := FormatError("", "", "email")
	expectedWithField := "[error] email: email validation failed"

	if resultWithField != expectedWithField {
		t.Errorf("FormatError() = %q, want %q", resultWithField, expectedWithField)
	}
}

// TestFormatError_ConsistentStructure verifies that FormatError
// produces consistent structure across different error types.
func TestFormatError_ConsistentStructure(t *testing.T) {
	errorTypes := []string{
		"status_code",
		"error_message",
		"content_type",
		"validation",
		"authentication",
		"authorization",
		"network",
		"timeout",
	}

	for _, errType := range errorTypes {
		t.Run(errType, func(t *testing.T) {
			result := FormatError(errType, "Test error message")

			// Verify structure starts with [error_type]
			if !strings.HasPrefix(result, "["+errType+"] ") {
				t.Errorf("FormatError(%q, ...) should start with [%q], got: %q",
					errType, errType, result)
			}

			// Verify message is included
			if !strings.Contains(result, "Test error message") {
				t.Errorf("FormatError(%q, ...) should contain message, got: %q",
					errType, result)
			}
		})
	}

	// Test with field name as well
	t.Run("with field names", func(t *testing.T) {
		result := FormatError("validation", "Test error", "email")

		// Should include field name
		if !strings.Contains(result, "email:") {
			t.Errorf("FormatError with field should include field name, got: %q", result)
		}

		// Should still have consistent structure
		if !strings.HasPrefix(result, "[validation]") {
			t.Errorf("FormatError with field should start with [validation], got: %q", result)
		}
	})
}

// TestFormatError_SpecialCharacters verifies that FormatError
// handles special characters in messages correctly.
func TestFormatError_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
	}{
		{
			name:      "newlines in message",
			errorType: "validation",
			message:   "Line 1\nLine 2",
		},
		{
			name:      "tabs in message",
			errorType: "status_code",
			message:   "Error:\t404",
		},
		{
			name:      "unicode characters",
			errorType: "error",
			message:   "Error: ✓ ✗ →",
		},
		{
			name:      "quotes in message",
			errorType: "message",
			message:   `"quoted" and 'single'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatError(tt.errorType, tt.message)

			// Should not panic and should contain the message
			if !strings.Contains(result, tt.message) {
				t.Errorf("FormatError() should contain special characters, got: %q", result)
			}

			// Should have consistent structure
			if !strings.HasPrefix(result, "["+tt.errorType+"] ") {
				t.Errorf("FormatError() should start with [%q], got: %q",
					tt.errorType, result)
			}
		})
	}
}

// TestFormatError_NoNilPanic verifies that FormatError does not panic
// with nil or empty inputs and handles them gracefully.
func TestFormatError_NoNilPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("FormatError should not panic, got: %v", r)
		}
	}()

	// Test with empty strings
	FormatError("", "")

	// Test with one empty
	FormatError("status_code", "")
	FormatError("", "message only")

	// Test with normal values
	FormatError("validation", "test message")
}

// =============================================================================
// FormatFieldPath FUNCTION TESTS
// =============================================================================

// TestFormatFieldPath_BasicPaths verifies that FormatFieldPath handles
// basic field paths correctly.
func TestFormatFieldPath_BasicPaths(t *testing.T) {
	tests := []struct {
		name     string
		fieldPath string
		expected string
	}{
		{
			name:     "simple field name",
			fieldPath: "email",
			expected: "email",
		},
		{
			name:     "nested field path",
			fieldPath: "user.email",
			expected: "user.email",
		},
		{
			name:     "deeply nested path",
			fieldPath: "data.user.profile.email",
			expected: "data.user.profile.email",
		},
		{
			name:     "field with underscore",
			fieldPath: "user_email",
			expected: "user_email",
		},
		{
			name:     "camelCase field",
			fieldPath: "userEmail",
			expected: "userEmail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldPath(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldPath(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldPath_ArrayIndices verifies that FormatFieldPath handles
// array indices correctly in various formats.
func TestFormatFieldPath_ArrayIndices(t *testing.T) {
	tests := []struct {
		name     string
		fieldPath string
		expected string
	}{
		{
			name:     "array index with dot notation",
			fieldPath: "users.0.email",
			expected: "users[0].email",
		},
		{
			name:     "array index at start",
			fieldPath: "0.name",
			expected: "0.name",
		},
		{
			name:     "multiple array indices",
			fieldPath: "data.0.items.1.name",
			expected: "data[0].items[1].name",
		},
		{
			name:     "large array index",
			fieldPath: "users.12345.name",
			expected: "users[12345].name",
		},
		{
			name:     "array index already in bracket notation",
			fieldPath: "users[0].email",
			expected: "users[0].email",
		},
		{
			name:     "mixed notation (bracket and dot)",
			fieldPath: "users[0].emails.1.address",
			expected: "users[0].emails[1].address",
		},
		{
			name:     "nested arrays with dot notation",
			fieldPath: "matrix.0.1.value",
			expected: "matrix[0][1].value",
		},
		{
			name:     "array index in middle of path",
			fieldPath: "data.users.0.profile.email",
			expected: "data.users[0].profile.email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldPath(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldPath(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldPath_EmptyAndInvalidPaths verifies that FormatFieldPath handles
// empty and invalid field paths gracefully.
func TestFormatFieldPath_EmptyAndInvalidPaths(t *testing.T) {
	tests := []struct {
		name     string
		fieldPath string
		expected string
	}{
		{
			name:     "empty string",
			fieldPath: "",
			expected: "(unknown field)",
		},
		{
			name:     "whitespace only",
			fieldPath: "   ",
			expected: "(unknown field)",
		},
		{
			name:     "whitespace with content",
			fieldPath: "  user.email  ",
			expected: "user.email",
		},
		{
			name:     "dots only",
			fieldPath: "...",
			expected: "(unknown field)",
		},
		{
			name:     "multiple consecutive dots",
			fieldPath: "user..email",
			expected: "user.email",
		},
		{
			name:     "leading dot",
			fieldPath: ".email",
			expected: "email",
		},
		{
			name:     "trailing dot",
			fieldPath: "email.",
			expected: "email",
		},
		{
			name:     "single dot",
			fieldPath: ".",
			expected: "(unknown field)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldPath(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldPath(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldPath_NegativeIndices verifies that FormatFieldPath handles
// negative array indices (though unusual).
func TestFormatFieldPath_NegativeIndices(t *testing.T) {
	tests := []struct {
		name     string
		fieldPath string
		expected string
	}{
		{
			name:     "negative index",
			fieldPath: "users.-1.email",
			expected: "users[-1].email",
		},
		{
			name:     "negative zero",
			fieldPath: "users.-0.name",
			expected: "users[-0].name",
		},
		{
			name:     "multiple negative indices",
			fieldPath: "data.-1.items.-2.value",
			expected: "data[-1].items[-2].value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldPath(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldPath(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldPath_SpecialCharacters verifies that FormatFieldPath handles
// field names with special characters correctly.
func TestFormatFieldPath_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		fieldPath string
		expected string
	}{
		{
			name:     "field with hyphen",
			fieldPath: "user-email",
			expected: "user-email",
		},
		{
			name:     "field with number in middle",
			fieldPath: "user2email",
			expected: "user2email",
		},
		{
			name:     "field starting with number",
			fieldPath: "2email",
			expected: "2email",
		},
		{
			name:     "complex field names",
			fieldPath: "user_first_name.email_address.work",
			expected: "user_first_name.email_address.work",
		},
		{
			name:     "field names with special chars in brackets",
			fieldPath: "users[0].email_address",
			expected: "users[0].email_address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldPath(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldPath(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldPath_EdgeCases verifies that FormatFieldPath handles
// edge cases and unusual but valid inputs.
func TestFormatFieldPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		fieldPath string
		expected string
	}{
		{
			name:     "zero index",
			fieldPath: "users.0.name",
			expected: "users[0].name",
		},
		{
			name:     "very long path",
			fieldPath: "a.b.c.d.e.f.g.h.i.j",
			expected: "a.b.c.d.e.f.g.h.i.j",
		},
		{
			name:     "path with multiple zero indices",
			fieldPath: "data.0.0.0.value",
			expected: "data[0][0][0].value",
		},
		{
			name:     "single character field names",
			fieldPath: "a.b.c",
			expected: "a.b.c",
		},
		{
			name:     "array at end only",
			fieldPath: "users.0",
			expected: "users[0]",
		},
		{
			name:     "bracket notation at start",
			fieldPath: "[0].name",
			expected: "[0].name",
		},
		{
			name:     "empty brackets with dot notation",
			fieldPath: "users..0.name",
			expected: "users[0].name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldPath(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldPath(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldPath_RealWorldExamples tests realistic field paths
// that would be used in actual error messages.
func TestFormatFieldPath_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name     string
		fieldPath string
		expected string
	}{
		{
			name:     "API response field",
			fieldPath: "response.data.users.0.email",
			expected: "response.data.users[0].email",
		},
		{
			name:     "JSON path with array",
			fieldPath: "order.items.5.price",
			expected: "order.items[5].price",
		},
		{
			name:     "nested object with array",
			fieldPath: "user.addresses.1.street",
			expected: "user.addresses[1].street",
		},
		{
			name:     "form field path",
			fieldPath: "form.email",
			expected: "form.email",
		},
		{
			name:     "request parameter",
			fieldPath: "request.query.page",
			expected: "request.query.page",
		},
		{
			name:     "database field",
			fieldPath: "users.0.profile.settings.notification_email",
			expected: "users[0].profile.settings.notification_email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldPath(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldPath(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldPath_Consistency verifies that FormatFieldPath produces
// consistent output across equivalent inputs.
func TestFormatFieldPath_Consistency(t *testing.T) {
	// Different ways to represent the same field path should normalize to the same output
	equivalentPaths := [][]string{
		// Array access with dot notation vs bracket notation
		{"users.0.email", "users[0].email"},
		// Multiple array indices
		{"data.0.items.1.name", "data[0].items[1].name"},
		// Mixed notation
		{"users[0].emails.1.address", "users.0.emails.1.address"},
	}

	for _, paths := range equivalentPaths {
		if len(paths) < 2 {
			continue
		}

		firstResult := FormatFieldPath(paths[0])
		for _, path := range paths[1:] {
			result := FormatFieldPath(path)
			if result != firstResult {
				t.Errorf("FormatFieldPath(%q) = %q, but FormatFieldPath(%q) = %q (expected same result)",
					paths[0], firstResult, path, result)
			}
		}
	}
}

// =============================================================================
// FormatFieldRef FUNCTION TESTS
// =============================================================================

// TestFormatFieldRef_BasicFormatting verifies that FormatFieldRef produces
// correctly formatted field references with default options.
func TestFormatFieldReference_BasicFormatting(t *testing.T) {
	tests := []struct {
		name     string
		fieldPath string
		expected string
	}{
		{
			name:     "simple field",
			fieldPath: "email",
			expected: "field 'email'",
		},
		{
			name:     "nested field",
			fieldPath: "user.email",
			expected: "field 'user.email'",
		},
		{
			name:     "deeply nested field",
			fieldPath: "data.user.profile.email",
			expected: "field 'data.user.profile.email'",
		},
		{
			name:     "field with array index (dot notation)",
			fieldPath: "users.0.email",
			expected: "field 'users[0].email'",
		},
		{
			name:     "field with array index (bracket notation)",
			fieldPath: "users[0].email",
			expected: "field 'users[0].email'",
		},
		{
			name:     "multiple array indices",
			fieldPath: "data.0.items.1.name",
			expected: "field 'data[0].items[1].name'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldRef_EmptyAndInvalidPaths verifies that FormatFieldRef handles
// empty and invalid field paths gracefully.
func TestFormatFieldReference_EmptyAndInvalidPaths(t *testing.T) {
	tests := []struct {
		name     string
		fieldPath string
		expected string
	}{
		{
			name:     "empty string",
			fieldPath: "",
			expected: "(unknown field)",
		},
		{
			name:     "whitespace only",
			fieldPath: "   ",
			expected: "(unknown field)",
		},
		{
			name:     "tabs and whitespace",
			fieldPath: "\t  \t",
			expected: "(unknown field)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldRef_QuoteStyles verifies that FormatFieldRef handles
// different quote styles correctly.
func TestFormatFieldReference_QuoteStyles(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		option    FieldRefOption
		expected  string
	}{
		{
			name:      "single quote (default)",
			fieldPath: "user.email",
			option:    nil,
			expected:  "field 'user.email'",
		},
		{
			name:      "no quote",
			fieldPath: "user.email",
			option:    WithQuoteStyle(NoQuote),
			expected:  "field user.email",
		},
		{
			name:      "double quote",
			fieldPath: "user.email",
			option:    WithQuoteStyle(DoubleQuote),
			expected:  `field "user.email"`,
		},
		{
			name:      "backtick",
			fieldPath: "user.email",
			option:    WithQuoteStyle(Backtick),
			expected:  "field `user.email`",
		},
		{
			name:      "no quote with array",
			fieldPath: "users.0.email",
			option:    WithQuoteStyle(NoQuote),
			expected:  "field users[0].email",
		},
		{
			name:      "backtick with array",
			fieldPath: "users.0.email",
			option:    WithQuoteStyle(Backtick),
			expected:  "field `users[0].email`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.option == nil {
				result = FormatFieldReference(tt.fieldPath)
			} else {
				result = FormatFieldReference(tt.fieldPath, tt.option)
			}
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, WithQuoteStyle(...)) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldRef_CustomPrefix verifies that FormatFieldRef handles
// custom prefixes correctly.
func TestFormatFieldReference_CustomPrefix(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		prefix    string
		expected  string
	}{
		{
			name:      "parameter prefix",
			fieldPath: "page",
			prefix:    "parameter",
			expected:  "parameter 'page'",
		},
		{
			name:      "property prefix",
			fieldPath: "user.email",
			prefix:    "property",
			expected:  "property 'user.email'",
		},
		{
			name:      "attribute prefix",
			fieldPath: "class.method",
			prefix:    "attribute",
			expected:  "attribute 'class.method'",
		},
		{
			name:      "empty prefix (no prefix)",
			fieldPath: "email",
			prefix:    "",
			expected:  "'email'",
		},
		{
			name:      "key prefix",
			fieldPath: "request.query.page",
			prefix:    "key",
			expected:  "key 'request.query.page'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath, WithPrefix(tt.prefix))
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, WithPrefix(%q)) = %q, want %q", tt.fieldPath, tt.prefix, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldRef_NoNormalization verifies that FormatFieldRef can
// skip path normalization when requested.
func TestFormatFieldReference_NoNormalization(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		normalize bool
		expected  string
	}{
		{
			name:      "normalize array index (default)",
			fieldPath: "users.0.email",
			normalize: true,
			expected:  "field 'users[0].email'",
		},
		{
			name:      "no normalization - keep dot notation",
			fieldPath: "users.0.email",
			normalize: false,
			expected:  "field 'users.0.email'",
		},
		{
			name:      "no normalization - keep bracket notation",
			fieldPath: "users[0].email",
			normalize: false,
			expected:  "field 'users[0].email'",
		},
		{
			name:      "normalize mixed notation",
			fieldPath: "users[0].emails.1.address",
			normalize: true,
			expected:  "field 'users[0].emails[1].address'",
		},
		{
			name:      "no normalization - keep mixed notation",
			fieldPath: "users[0].emails.1.address",
			normalize: false,
			expected:  "field 'users[0].emails.1.address'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath, WithNormalizePath(tt.normalize))
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, WithNormalizePath(%v)) = %q, want %q", tt.fieldPath, tt.normalize, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldRef_CombinedOptions verifies that FormatFieldRef handles
// multiple options combined correctly.
func TestFormatFieldReference_CombinedOptions(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		options   []FieldRefOption
		expected  string
	}{
		{
			name:      "custom prefix with double quote",
			fieldPath: "user.email",
			options:   []FieldRefOption{WithPrefix("parameter"), WithQuoteStyle(DoubleQuote)},
			expected:  `parameter "user.email"`,
		},
		{
			name:      "no prefix with backtick",
			fieldPath: "data.value",
			options:   []FieldRefOption{WithPrefix(""), WithQuoteStyle(Backtick)},
			expected:  "`data.value`",
		},
		{
			name:      "custom prefix with no quote and normalize",
			fieldPath: "users.0.email",
			options:   []FieldRefOption{WithPrefix("element"), WithQuoteStyle(NoQuote), WithNormalizePath(true)},
			expected:  "element users[0].email",
		},
		{
			name:      "empty prefix with single quote and no normalize",
			fieldPath: "users.0.email",
			options:   []FieldRefOption{WithPrefix(""), WithQuoteStyle(SingleQuote), WithNormalizePath(false)},
			expected:  "'users.0.email'",
		},
		{
			name:      "all options combined",
			fieldPath: "request.data.items.5.name",
			options:   []FieldRefOption{WithPrefix("field"), WithQuoteStyle(DoubleQuote), WithNormalizePath(true)},
			expected:  `field "request.data.items[5].name"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath, tt.options...)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, options...) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldRef_RealWorldExamples tests realistic field references
// that would be used in actual error messages.
func TestFormatFieldReference_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		options   []FieldRefOption
		expected  string
	}{
		{
			name:      "API validation error",
			fieldPath: "response.data.users.0.email",
			expected:  "field 'response.data.users[0].email'",
		},
		{
			name:      "form field error",
			fieldPath: "form.email",
			expected:  "field 'form.email'",
		},
		{
			name:      "request parameter error",
			fieldPath: "request.query.page",
			options:   []FieldRefOption{WithPrefix("parameter")},
			expected:  "parameter 'request.query.page'",
		},
		{
			name:      "JSON path error",
			fieldPath: "order.items.5.price",
			expected:  "field 'order.items[5].price'",
		},
		{
			name:      "config field error with backticks",
			fieldPath: "server.port",
			options:   []FieldRefOption{WithQuoteStyle(Backtick)},
			expected:  "field `server.port`",
		},
		{
			name:      "database field error",
			fieldPath: "users.0.profile.settings.notification_email",
			expected:  "field 'users[0].profile.settings.notification_email'",
		},
		{
			name:      "header field error",
			fieldPath: "headers.Authorization",
			options:   []FieldRefOption{WithPrefix("header")},
			expected:  "header 'headers.Authorization'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.options == nil {
				result = FormatFieldReference(tt.fieldPath)
			} else {
				result = FormatFieldReference(tt.fieldPath, tt.options...)
			}
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldRef_WhitespaceHandling verifies that FormatFieldRef handles
// whitespace in field paths correctly.
func TestFormatFieldReference_WhitespaceHandling(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		expected  string
	}{
		{
			name:      "leading and trailing whitespace",
			fieldPath: "  user.email  ",
			expected:  "field 'user.email'",
		},
		{
			name:      "tabs around path",
			fieldPath: "\tuser.email\t",
			expected:  "field 'user.email'",
		},
		{
			name:      "mixed whitespace",
			fieldPath: " \t user.email \t ",
			expected:  "field 'user.email'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldRef_SpecialCharacters verifies that FormatFieldRef handles
// field names with special characters correctly.
func TestFormatFieldReference_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		expected  string
	}{
		{
			name:      "field with hyphen",
			fieldPath: "user-email",
			expected:  "field 'user-email'",
		},
		{
			name:      "field with underscore",
			fieldPath: "user_email",
			expected:  "field 'user_email'",
		},
		{
			name:      "field with numbers",
			fieldPath: "user2email",
			expected:  "field 'user2email'",
		},
		{
			name:      "complex field names",
			fieldPath: "user_first_name.email_address.work",
			expected:  "field 'user_first_name.email_address.work'",
		},
		{
			name:      "camelCase field",
			fieldPath: "userEmail",
			expected:  "field 'userEmail'",
		},
		{
			name:      "PascalCase field",
			fieldPath: "UserProfile",
			expected:  "field 'UserProfile'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldRef_NegativeIndices verifies that FormatFieldRef handles
// negative array indices correctly.
func TestFormatFieldReference_NegativeIndices(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		expected  string
	}{
		{
			name:      "negative index",
			fieldPath: "users.-1.email",
			expected:  "field 'users[-1].email'",
		},
		{
			name:      "negative zero",
			fieldPath: "users.-0.name",
			expected:  "field 'users[-0].name'",
		},
		{
			name:      "multiple negative indices",
			fieldPath: "data.-1.items.-2.value",
			expected:  "field 'data[-1].items[-2].value'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q) = %q, want %q", tt.fieldPath, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldRef_ErrorMessageIntegration verifies that FormatFieldRef
// integrates well with error message construction.
func TestFormatFieldReference_ErrorMessageIntegration(t *testing.T) {
	tests := []struct {
		name          string
		fieldPath     string
		errorTemplate string
		options       []FieldRefOption
		expectedParts []string
	}{
		{
			name:          "required field error",
			fieldPath:     "user.email",
			errorTemplate: "%s is required",
			expectedParts: []string{"field 'user.email'", "is required"},
		},
		{
			name:          "invalid type error",
			fieldPath:     "user.age",
			errorTemplate: "%s must be a number",
			expectedParts: []string{"field 'user.age'", "must be a number"},
		},
		{
			name:          "parameter error",
			fieldPath:     "page",
			errorTemplate: "%s must be positive",
			options:       []FieldRefOption{WithPrefix("parameter")},
			expectedParts: []string{"parameter 'page'", "must be positive"},
		},
		{
			name:          "array field error",
			fieldPath:     "users.0.email",
			errorTemplate: "%s contains invalid email",
			expectedParts: []string{"field 'users[0].email'", "contains invalid email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fieldRef string
			if tt.options == nil {
				fieldRef = FormatFieldReference(tt.fieldPath)
			} else {
				fieldRef = FormatFieldReference(tt.fieldPath, tt.options...)
			}
			errorMsg := fmt.Sprintf(tt.errorTemplate, fieldRef)

			for _, expectedPart := range tt.expectedParts {
				if !strings.Contains(errorMsg, expectedPart) {
					t.Errorf("Error message %q does not contain expected part %q", errorMsg, expectedPart)
				}
			}
		})
	}
}

// TestFormatFieldRef_Consistency verifies that FormatFieldRef produces
// consistent output across equivalent inputs.
func TestFormatFieldReference_Consistency(t *testing.T) {
	// Different ways to represent the same field path should normalize to the same output
	equivalentPaths := [][]string{
		// Array access with dot notation vs bracket notation
		{"users.0.email", "users[0].email"},
		// Multiple array indices
		{"data.0.items.1.name", "data[0].items[1].name"},
		// Mixed notation
		{"users[0].emails.1.address", "users.0.emails.1.address"},
	}

	for _, paths := range equivalentPaths {
		if len(paths) < 2 {
			continue
		}

		firstResult := FormatFieldReference(paths[0])
		for _, path := range paths[1:] {
			result := FormatFieldReference(path)
			if result != firstResult {
				t.Errorf("FormatFieldReference(%q) = %q, but FormatFieldReference(%q) = %q (expected same result)",
					paths[0], firstResult, path, result)
			}
		}
	}
}
