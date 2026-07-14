package validate

import (
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
