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
			expected:  "[validation] (no message provided)",
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
// ERROR TYPE VALIDATION TESTS
// =============================================================================

// TestFormatError_WithValidErrorTypes verifies that FormatError validates
// recognized ErrorType enum values and formats them correctly.
func TestFormatError_WithValidErrorTypes(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		expected  string
	}{
		{
			name:      "required error type",
			errorType: "required",
			message:   "Field is required",
			fieldName: "email",
			expected:  "[required] email: Field is required",
		},
		{
			name:      "format error type",
			errorType: "format",
			message:   "Invalid email format",
			fieldName: "",
			expected:  "[format] Invalid email format",
		},
		{
			name:      "range error type",
			errorType: "range",
			message:   "Value out of range",
			fieldName: "age",
			expected:  "[range] age: Value out of range",
		},
		{
			name:      "length error type",
			errorType: "length",
			message:   "String too short",
			fieldName: "password",
			expected:  "[length] password: String too short",
		},
		{
			name:      "type error type",
			errorType: "type",
			message:   "Expected number, got string",
			fieldName: "count",
			expected:  "[type] count: Expected number, got string",
		},
		{
			name:      "value error type",
			errorType: "value",
			message:   "Invalid value",
			fieldName: "status",
			expected:  "[value] status: Invalid value",
		},
		{
			name:      "duplicate error type",
			errorType: "duplicate",
			message:   "Email already exists",
			fieldName: "email",
			expected:  "[duplicate] email: Email already exists",
		},
		{
			name:      "conflict error type",
			errorType: "conflict",
			message:   "Values conflict",
			fieldName: "",
			expected:  "[conflict] Values conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tracking before each test
			ResetInvalidErrorTypeTracking()

			var result string
			if tt.fieldName != "" {
				result = FormatError(tt.errorType, tt.message, tt.fieldName)
			} else {
				result = FormatError(tt.errorType, tt.message)
			}

			if result != tt.expected {
				t.Errorf("FormatError() = %q, want %q", result, tt.expected)
			}

			// Verify that valid error types are NOT tracked as invalid
			invalidTypes := GetInvalidErrorTypes()
			if len(invalidTypes) > 0 {
				t.Errorf("Valid error type %q was incorrectly tracked as invalid", tt.errorType)
			}
		})
	}
}

// TestFormatError_WithInvalidErrorTypes verifies that FormatError tracks
// unrecognized error types while maintaining backward compatibility.
func TestFormatError_WithInvalidErrorTypes(t *testing.T) {
	tests := []struct {
		name           string
		errorType      string
		message        string
		fieldName      string
		expectedOutput string
		shouldTrack    bool
	}{
		{
			name:           "custom error type",
			errorType:      "custom_validation",
			message:        "Custom validation failed",
			fieldName:      "field",
			expectedOutput: "[custom_validation] field: Custom validation failed",
			shouldTrack:    true,
		},
		{
			name:           "typo in error type",
			errorType:      "requird", // typo of "required"
			message:        "Field is required",
			fieldName:      "email",
			expectedOutput: "[requird] email: Field is required",
			shouldTrack:    true,
		},
		{
			name:           "unknown error type",
			errorType:      "unknown_type_xyz",
			message:        "Unknown error",
			fieldName:      "",
			expectedOutput: "[unknown_type_xyz] Unknown error",
			shouldTrack:    true,
		},
		{
			name:           "HTTP-specific error type",
			errorType:      "status_code",
			message:        "Status code mismatch",
			fieldName:      "response",
			expectedOutput: "[status_code] response: Status code mismatch",
			shouldTrack:    true, // status_code is not a basic ErrorType
		},
		{
			name:           "error_message type",
			errorType:      "error_message",
			message:        "Error message pattern not found",
			fieldName:      "",
			expectedOutput: "[error_message] Error message pattern not found",
			shouldTrack:    true, // error_message is not a basic ErrorType
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tracking before each test
			ResetInvalidErrorTypeTracking()

			var result string
			if tt.fieldName != "" {
				result = FormatError(tt.errorType, tt.message, tt.fieldName)
			} else {
				result = FormatError(tt.errorType, tt.message)
			}

			// Verify output is correct (backward compatibility maintained)
			if result != tt.expectedOutput {
				t.Errorf("FormatError() = %q, want %q", result, tt.expectedOutput)
			}

			// Check if error type was tracked
			invalidTypes := GetInvalidErrorTypes()
			if tt.shouldTrack {
				if len(invalidTypes) == 0 {
					t.Errorf("Expected invalid error type %q to be tracked", tt.errorType)
				} else if count, ok := invalidTypes[tt.errorType]; !ok || count == 0 {
					t.Errorf("Expected error type %q to be tracked with count > 0", tt.errorType)
				}
			} else {
				if len(invalidTypes) > 0 {
					t.Errorf("Expected valid error type %q to NOT be tracked", tt.errorType)
				}
			}
		})
	}
}

// TestFormatError_ErrorTypeTracking verifies that the error type tracking
// system correctly accumulates occurrences of invalid types.
func TestFormatError_ErrorTypeTracking(t *testing.T) {
	// Reset tracking before test
	ResetInvalidErrorTypeTracking()

	// Use multiple invalid error types
	invalidTypes := []string{
		"custom_type_1",
		"custom_type_2",
		"custom_type_1", // repeat to test accumulation
		"custom_type_3",
		"custom_type_1", // repeat again
	}

	for _, errType := range invalidTypes {
		FormatError(errType, "test message", "field")
	}

	// Get tracked invalid types
	tracked := GetInvalidErrorTypes()

	// Verify counts
	expectedCounts := map[string]int{
		"custom_type_1": 3,
		"custom_type_2": 1,
		"custom_type_3": 1,
	}

	for errType, expectedCount := range expectedCounts {
		if count, ok := tracked[errType]; !ok {
			t.Errorf("Expected error type %q to be tracked", errType)
		} else if count != expectedCount {
			t.Errorf("Expected %q to have count %d, got %d", errType, expectedCount, count)
		}
	}

	// Verify we tracked exactly 3 unique types
	if InvalidErrorTypeCount() != 3 {
		t.Errorf("Expected 3 unique invalid error types, got %d", InvalidErrorTypeCount())
	}
}

// TestFormatError_CaseInsensitiveValidation verifies that ErrorTypeFromString
// handles case-insensitive matching correctly.
func TestFormatError_CaseInsensitiveValidation(t *testing.T) {
	// Reset tracking before test
	ResetInvalidErrorTypeTracking()

	tests := []struct {
		name        string
		errorType   string
		shouldTrack bool
	}{
		{
			name:        "uppercase REQUIRED",
			errorType:   "REQUIRED",
			shouldTrack: false, // Should be recognized as "required"
		},
		{
			name:        "mixed case Format",
			errorType:   "Format",
			shouldTrack: false, // Should be recognized as "format"
		},
		{
			name:        "lowercase range",
			errorType:   "range",
			shouldTrack: false, // Should be recognized
		},
		{
			name:        "custom type uppercase",
			errorType:   "CUSTOM_TYPE",
			shouldTrack: true, // Not a recognized type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			FormatError(tt.errorType, "test message", "field")

			tracked := GetInvalidErrorTypes()
			isTracked := false
			for trackedType := range tracked {
				if strings.EqualFold(trackedType, tt.errorType) {
					isTracked = true
					break
				}
			}

			if tt.shouldTrack && !isTracked {
				t.Errorf("Expected error type %q to be tracked as invalid", tt.errorType)
			} else if !tt.shouldTrack && isTracked {
				t.Errorf("Expected error type %q to NOT be tracked (should be recognized)", tt.errorType)
			}
		})
	}
}

// TestTrackInvalidErrorType verifies the tracking function directly.
func TestTrackInvalidErrorType(t *testing.T) {
	// Reset tracking before test
	ResetInvalidErrorTypeTracking()

	// Track some invalid error types
	count1 := TrackInvalidErrorType("type1")
	count2 := TrackInvalidErrorType("type2")
	count3 := TrackInvalidErrorType("type1") // Track same type again
	count4 := TrackInvalidErrorType("type3")

	// Verify return values (accumulated counts)
	if count1 != 1 {
		t.Errorf("Expected first occurrence of type1 to return count 1, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("Expected first occurrence of type2 to return count 1, got %d", count2)
	}
	if count3 != 2 {
		t.Errorf("Expected second occurrence of type1 to return count 2, got %d", count3)
	}
	if count4 != 1 {
		t.Errorf("Expected first occurrence of type3 to return count 1, got %d", count4)
	}

	// Verify tracked types
	tracked := GetInvalidErrorTypes()
	expectedCounts := map[string]int{
		"type1": 2,
		"type2": 1,
		"type3": 1,
	}

	for errType, expectedCount := range expectedCounts {
		if count, ok := tracked[errType]; !ok {
			t.Errorf("Expected error type %q to be tracked", errType)
		} else if count != expectedCount {
			t.Errorf("Expected %q to have count %d, got %d", errType, expectedCount, count)
		}
	}
}

// TestGetInvalidErrorTypes verifies that GetInvalidErrorTypes returns
// a copy of the tracking map to prevent concurrent modification.
func TestGetInvalidErrorTypes(t *testing.T) {
	ResetInvalidErrorTypeTracking()

	// Add some tracked types
	TrackInvalidErrorType("type1")
	TrackInvalidErrorType("type2")

	// Get the map
	tracked := GetInvalidErrorTypes()

	// Modify the returned map
	tracked["type1"] = 999
	tracked["new_type"] = 1

	// Get the map again
	trackedAgain := GetInvalidErrorTypes()

	// Verify that modifications to the returned map don't affect the internal state
	if trackedAgain["type1"] == 999 {
		t.Error("Modifying the returned map should not affect internal tracking state")
	}
	if _, ok := trackedAgain["new_type"]; ok {
		t.Error("Adding to the returned map should not affect internal tracking state")
	}

	// Verify actual counts
	if count := trackedAgain["type1"]; count != 1 {
		t.Errorf("Expected type1 count to be 1, got %d", count)
	}
}

// TestResetInvalidErrorTypeTracking verifies that reset clears all tracked types.
func TestResetInvalidErrorTypeTracking(t *testing.T) {
	// Add some tracked types
	TrackInvalidErrorType("type1")
	TrackInvalidErrorType("type2")
	TrackInvalidErrorType("type1")

	// Verify we have tracked types
	if InvalidErrorTypeCount() != 2 {
		t.Errorf("Expected 2 tracked types before reset, got %d", InvalidErrorTypeCount())
	}

	// Reset tracking
	ResetInvalidErrorTypeTracking()

	// Verify all tracking is cleared
	if InvalidErrorTypeCount() != 0 {
		t.Errorf("Expected 0 tracked types after reset, got %d", InvalidErrorTypeCount())
	}

	tracked := GetInvalidErrorTypes()
	if len(tracked) != 0 {
		t.Errorf("Expected empty map after reset, got %v", tracked)
	}
}

// TestInvalidErrorTypeCount verifies the count function.
func TestInvalidErrorTypeCount(t *testing.T) {
	ResetInvalidErrorTypeTracking()

	// Initially should be 0
	if InvalidErrorTypeCount() != 0 {
		t.Errorf("Expected initial count to be 0, got %d", InvalidErrorTypeCount())
	}

	// Add unique types
	TrackInvalidErrorType("type1")
	if InvalidErrorTypeCount() != 1 {
		t.Errorf("Expected count 1 after adding first type, got %d", InvalidErrorTypeCount())
	}

	TrackInvalidErrorType("type2")
	if InvalidErrorTypeCount() != 2 {
		t.Errorf("Expected count 2 after adding second type, got %d", InvalidErrorTypeCount())
	}

	// Add duplicate - count should not change
	TrackInvalidErrorType("type1")
	if InvalidErrorTypeCount() != 2 {
		t.Errorf("Expected count 2 after adding duplicate, got %d", InvalidErrorTypeCount())
	}

	// Add third unique type
	TrackInvalidErrorType("type3")
	if InvalidErrorTypeCount() != 3 {
		t.Errorf("Expected count 3 after adding third type, got %d", InvalidErrorTypeCount())
	}

	// Reset and verify count returns to 0
	ResetInvalidErrorTypeTracking()
	if InvalidErrorTypeCount() != 0 {
		t.Errorf("Expected count 0 after reset, got %d", InvalidErrorTypeCount())
	}
}

// TestFormatError_BackwardCompatibility verifies that existing code using
// FormatError continues to work without changes.

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
// FormatFieldReference FUNCTION TESTS
//=============================================================================

// TestFormatFieldReference_BasicFormatting verifies that FormatFieldReference
// produces correctly formatted field references with and without prefixes.
func TestFormatFieldReference_BasicFormatting(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		prefix    string
		expected  string
	}{
		{
			name:      "field without prefix",
			fieldPath: "email",
			prefix:    "",
			expected:  "email",
		},
		{
			name:      "field with prefix",
			fieldPath: "email",
			prefix:    "request",
			expected:  "request.email",
		},
		{
			name:      "nested field without prefix",
			fieldPath: "user.email",
			prefix:    "",
			expected:  "user.email",
		},
		{
			name:      "nested field with prefix",
			fieldPath: "user.email",
			prefix:    "data",
			expected:  "data.user.email",
		},
		{
			name:      "empty field path with prefix",
			fieldPath: "",
			prefix:    "request",
			expected:  "request",
		},
		{
			name:      "empty field path without prefix",
			fieldPath: "",
			prefix:    "",
			expected:  "(unknown field)",
		},
		{
			name:      "whitespace handling",
			fieldPath: "  email  ",
			prefix:    "  request  ",
			expected:  "request.email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath, tt.prefix)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, %q) = %q, want %q",
					tt.fieldPath, tt.prefix, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldReference_ArrayIndices verifies that FormatFieldReference
// handles array index normalization correctly.
func TestFormatFieldReference_ArrayIndices(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		prefix    string
		expected  string
	}{
		{
			name:      "array index with dot notation",
			fieldPath: "users.0.email",
			prefix:    "",
			expected:  "users[0].email",
		},
		{
			name:      "array index with prefix",
			fieldPath: "users.0.email",
			prefix:    "response",
			expected:  "response.users[0].email",
		},
		{
			name:      "multiple array indices",
			fieldPath: "data.0.items.1.name",
			prefix:    "response",
			expected:  "response.data[0].items[1].name",
		},
		{
			name:      "array index already in bracket notation",
			fieldPath: "users[0].email",
			prefix:    "data",
			expected:  "data.users[0].email",
		},
		{
			name:      "nested arrays with dot notation",
			fieldPath: "matrix.0.1.value",
			prefix:    "response",
			expected:  "response.matrix[0][1].value",
		},
		{
			name:      "large array index",
			fieldPath: "users.12345.name",
			prefix:    "",
			expected:  "users[12345].name",
		},
		{
			name:      "negative array index",
			fieldPath: "users.-1.email",
			prefix:    "response",
			expected:  "response.users[-1].email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath, tt.prefix)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, %q) = %q, want %q",
					tt.fieldPath, tt.prefix, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldReference_EmptyAndInvalidPaths verifies that FormatFieldReference
// handles empty and invalid field paths gracefully.
func TestFormatFieldReference_EmptyAndInvalidPaths(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		prefix    string
		expected  string
	}{
		{
			name:      "empty string without prefix",
			fieldPath: "",
			prefix:    "",
			expected:  "(unknown field)",
		},
		{
			name:      "empty string with prefix",
			fieldPath: "",
			prefix:    "request",
			expected:  "request",
		},
		{
			name:      "whitespace only without prefix",
			fieldPath: "   ",
			prefix:    "",
			expected:  "(unknown field)",
		},
		{
			name:      "whitespace only with prefix",
			fieldPath: "   ",
			prefix:    "request",
			expected:  "request",
		},
		{
			name:      "dots only without prefix",
			fieldPath: "...",
			prefix:    "",
			expected:  "(unknown field)",
		},
		{
			name:      "dots only with prefix",
			fieldPath: "...",
			prefix:    "request",
			expected:  "request",
		},
		{
			name:      "invalid path with prefix",
			fieldPath: "...",
			prefix:    "data.request",
			expected:  "data.request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath, tt.prefix)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, %q) = %q, want %q",
					tt.fieldPath, tt.prefix, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldReference_PrefixWithArrayIndex verifies that FormatFieldReference
// handles prefixes that contain array indices.
func TestFormatFieldReference_PrefixWithArrayIndex(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		prefix    string
		expected  string
	}{
		{
			name:      "prefix with array index",
			fieldPath: "email",
			prefix:    "users.0",
			expected:  "users[0].email",
		},
		{
			name:      "both prefix and field with array indices",
			fieldPath: "profile.email",
			prefix:    "users.0",
			expected:  "users[0].profile.email",
		},
		{
			name:      "prefix with bracket notation",
			fieldPath: "name",
			prefix:    "users[0]",
			expected:  "users[0].name",
		},
		{
			name:      "empty field with array prefix",
			fieldPath: "",
			prefix:    "users.0",
			expected:  "users[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath, tt.prefix)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, %q) = %q, want %q",
					tt.fieldPath, tt.prefix, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldReference_ComplexPaths verifies that FormatFieldReference
// handles complex real-world field paths.
func TestFormatFieldReference_ComplexPaths(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		prefix    string
		expected  string
	}{
		{
			name:      "deeply nested with array indices",
			fieldPath: "items.0.data.values.1.field",
			prefix:    "response",
			expected:  "response.items[0].data.values[1].field",
		},
		{
			name:      "API response field",
			fieldPath: "response.data.users.0.email",
			prefix:    "body",
			expected:  "body.response.data.users[0].email",
		},
		{
			name:      "JSON path with array",
			fieldPath: "order.items.5.price",
			prefix:    "data",
			expected:  "data.order.items[5].price",
		},
		{
			name:      "database field path",
			fieldPath: "users.0.profile.settings.notification_email",
			prefix:    "record",
			expected:  "record.users[0].profile.settings.notification_email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath, tt.prefix)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, %q) = %q, want %q",
					tt.fieldPath, tt.prefix, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldReference_RealWorldUsage verifies that FormatFieldReference
// produces consistent results in realistic error message contexts.
func TestFormatFieldReference_RealWorldUsage(t *testing.T) {
	// Simulate building error messages with field references
	t.Run("building error message with field reference", func(t *testing.T) {
		fieldRef := FormatFieldReference("users.0.email", "response")
		expected := "response.users[0].email"

		if fieldRef != expected {
			t.Errorf("Field reference = %q, want %q", fieldRef, expected)
		}

		// Simulate using it in an error message
		errorMsg := fmt.Sprintf("Validation failed for field: %s", fieldRef)
		if !strings.Contains(errorMsg, "response.users[0].email") {
			t.Error("Error message should contain normalized field reference")
		}
	})

	t.Run("empty path with prefix returns prefix", func(t *testing.T) {
		// When field path is empty but prefix is provided, return prefix
		fieldRef := FormatFieldReference("", "request")
		expected := "request"

		if fieldRef != expected {
			t.Errorf("Field reference = %q, want %q", fieldRef, expected)
		}
	})
}
// =============================================================================
// FormatFieldRef FUNCTION TESTS
// =============================================================================

// TestFormatFieldRef_BasicFormatting verifies that FormatFieldRef produces
// correctly formatted field references with default options.
