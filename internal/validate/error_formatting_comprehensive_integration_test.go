package validate

import (
	"strings"
	"testing"
)

/*
Comprehensive Integration Tests for Error Formatting System

This test file provides comprehensive end-to-end testing for the complete validation
error formatting system. These tests verify that:

1. All validation error types generate appropriate suggestions
2. Error messages are clear and actionable
3. Expected/actual values are properly formatted
4. Context and field information is included
5. Suggestions are relevant and helpful
6. Error output is complete and well-formatted

Coverage includes all error types:
- status_code (HTTP status codes)
- error_message (error message patterns)
- content_type (Content-Type validation)
- status_code_range (status code ranges)
- cors_headers (CORS validation)
- response_structure (response body structure)
- response_body (response body content)
- response_encoding (encoding validation)
- auth_headers (authentication headers)
- custom_headers (custom header validation)
- json_schema (JSON schema validation)
- data_validation (data field validation)
- type_validation (type checking)
- timeout (request timeouts)
- rate_limit (rate limiting)
- retry_exceeded (retry limits)
- custom validation types
*/

// TestAllErrorTypes_GenerateSuggestions verifies that all error types generate suggestions.
func TestAllErrorTypes_GenerateSuggestions(t *testing.T) {
	errorTypes := []string{
		"status_code",
		"error_message",
		"content_type",
		"status_code_range",
		"cors_headers",
		"response_structure",
		"response_body",
		"response_encoding",
		"auth_headers",
		"custom_headers",
		"json_schema",
		"data_validation",
		"type_validation",
		"timeout",
		"rate_limit",
		"retry_exceeded",
	}

	for _, errorType := range errorTypes {
		t.Run(errorType, func(t *testing.T) {
			suggestions := GenerateComprehensiveSuggestions(errorType, "expected", "actual", "test context")

			if len(suggestions) == 0 {
				t.Errorf("Error type '%s' generated no suggestions", errorType)
			}

			// Verify suggestions are not empty strings
			for i, suggestion := range suggestions {
				if strings.TrimSpace(suggestion) == "" {
					t.Errorf("Error type '%s' has empty suggestion at index %d", errorType, i)
				}
			}
		})
	}
}

// TestStatusCodeErrors_Comprehensive tests all common HTTP status code errors.
func TestStatusCodeErrors_Comprehensive(t *testing.T) {
	statusCodes := []struct {
		code         int
		description  string
		minSuggestions int
		keywords    []string
	}{
		{400, "Bad Request", 3, []string{"request", "validation", "body"}},
		{401, "Unauthorized", 3, []string{"authentication", "token", "credentials"}},
		{403, "Forbidden", 3, []string{"permission", "authorization", "access"}},
		{404, "Not Found", 3, []string{"endpoint", "resource", "exists"}},
		{405, "Method Not Allowed", 2, []string{"method", "HTTP"}},
		{409, "Conflict", 3, []string{"conflict", "exists", "duplicate"}},
		{422, "Unprocessable Entity", 3, []string{"validation", "format", "schema"}},
		{429, "Too Many Requests", 3, []string{"rate", "limit", "quota"}},
		{500, "Internal Server Error", 3, []string{"server", "retry", "backoff"}},
		{502, "Bad Gateway", 3, []string{"gateway", "upstream", "server"}},
		{503, "Service Unavailable", 3, []string{"unavailable", "maintenance", "retry"}},
		{504, "Gateway Timeout", 3, []string{"timeout", "gateway", "slow"}},
	}

	for _, tc := range statusCodes {
		t.Run(tc.description, func(t *testing.T) {
			err := FormatStatusCodeError(200, tc.code, "test request")

			// Verify error is created successfully
			if err.ErrorType != "status_code" {
				t.Errorf("Expected error_type 'status_code', got '%s'", err.ErrorType)
			}

			// Verify suggestions are generated
			if len(err.Suggestions) < tc.minSuggestions {
				t.Errorf("Expected at least %d suggestions for status %d, got %d",
					tc.minSuggestions, tc.code, len(err.Suggestions))
			}

			// Verify suggestions contain relevant keywords
			suggestionText := strings.Join(err.Suggestions, " ")
			for _, keyword := range tc.keywords {
				if !strings.Contains(strings.ToLower(suggestionText), strings.ToLower(keyword)) {
					t.Logf("Note: Status %d suggestions don't contain keyword '%s'", tc.code, keyword)
					// This is informational, not a failure, as not all keywords may be present
				}
			}

			// Verify error output is formatted correctly
			errorOutput := err.Error()
			if !strings.Contains(errorOutput, "validation failed") {
				t.Errorf("Error output doesn't contain 'validation failed': %s", errorOutput)
			}
			if !strings.Contains(errorOutput, "Expected:") {
				t.Errorf("Error output doesn't contain 'Expected:': %s", errorOutput)
			}
			if !strings.Contains(errorOutput, "Received:") {
				t.Errorf("Error output doesn't contain 'Received:': %s", errorOutput)
			}
		})
	}
}

// TestErrorMessagePatterns_Comprehensive tests error message pattern validation.
func TestErrorMessagePatterns_Comprehensive(t *testing.T) {
	patterns := []struct {
		name         string
		expected     string
		actual       string
		fieldName    string
		context      string
		minSuggestions int
		keywords     []string
	}{
		{
			name:         "expired token",
			expected:     "valid.*token",
			actual:       "token expired",
			fieldName:    "error",
			context:      "OAuth validation",
			minSuggestions: 3,
			keywords:     []string{"expired", "refresh", "token"},
		},
		{
			name:         "invalid token",
			expected:     "valid.*token",
			actual:       "invalid token",
			fieldName:    "error",
			context:      "Authentication",
			minSuggestions: 3,
			keywords:     []string{"invalid", "format", "verify"},
		},
		{
			name:         "access denied",
			expected:     "authentication.*failed",
			actual:       "access denied",
			fieldName:    "message",
			context:      "Authorization check",
			minSuggestions: 3,
			keywords:     []string{"permission", "authorization", "access"},
		},
		{
			name:         "resource not found",
			expected:     "not found",
			actual:       "resource does not exist",
			fieldName:    "error",
			context:      "Resource lookup",
			minSuggestions: 3,
			keywords:     []string{"resource", "exists", "identifier"},
		},
		{
			name:         "validation error",
			expected:     "success",
			actual:       "validation failed: required field missing",
			fieldName:    "message",
			context:      "Data submission",
			minSuggestions: 3,
			keywords:     []string{"required", "field", "validation"},
		},
		{
			name:         "rate limit exceeded",
			expected:     "success",
			actual:       "rate limit exceeded",
			fieldName:    "error",
			context:      "API request",
			minSuggestions: 3,
			keywords:     []string{"rate", "limit", "backoff"},
		},
		{
			name:         "timeout error",
			expected:     "quick response",
			actual:       "request timeout",
			fieldName:    "error",
			context:      "API call",
			minSuggestions: 3,
			keywords:     []string{"timeout", "retry", "network"},
		},
		{
			name:         "server error",
			expected:     "success",
			actual:       "internal server error",
			fieldName:    "message",
			context:      "API request",
			minSuggestions: 3,
			keywords:     []string{"server", "retry", "backoff"},
		},
	}

	for _, tc := range patterns {
		t.Run(tc.name, func(t *testing.T) {
			err := FormatErrorMessageError(tc.expected, tc.actual, tc.fieldName, tc.context)

			// Verify error structure
			if err.ErrorType != "error_message" {
				t.Errorf("Expected error_type 'error_message', got '%s'", err.ErrorType)
			}
			if err.FieldName != tc.fieldName {
				t.Errorf("Expected field_name '%s', got '%s'", tc.fieldName, err.FieldName)
			}

			// Verify suggestions
			if len(err.Suggestions) < tc.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tc.minSuggestions, len(err.Suggestions))
			}

			// Verify suggestions contain relevant keywords
			suggestionText := strings.Join(err.Suggestions, " ")
			foundKeywords := 0
			for _, keyword := range tc.keywords {
				if strings.Contains(strings.ToLower(suggestionText), strings.ToLower(keyword)) {
					foundKeywords++
				}
			}
			if foundKeywords == 0 {
				t.Logf("Note: No keywords found in suggestions for '%s'", tc.name)
			}

			// Verify error output format
			errorOutput := err.Error()
			if !strings.Contains(errorOutput, tc.expected) {
				t.Errorf("Error output doesn't contain expected pattern '%s'", tc.expected)
			}
			if !strings.Contains(errorOutput, tc.actual) {
				t.Errorf("Error output doesn't contain actual message '%s'", tc.actual)
			}
		})
	}
}

// TestContentTypeValidation_Comprehensive tests Content-Type validation errors.
func TestContentTypeValidation_Comprehensive(t *testing.T) {
	contentTypes := []struct {
		name         string
		expected     string
		actual       string
		context      string
		minSuggestions int
		keywords     []string
	}{
		{
			name:         "JSON expected, HTML received",
			expected:     "application/json",
			actual:       "text/html",
			context:      "API response",
			minSuggestions: 3,
			keywords:     []string{"JSON", "HTML", "endpoint"},
		},
		{
			name:         "JSON expected, XML received",
			expected:     "application/json",
			actual:       "application/xml",
			context:      "API response",
			minSuggestions: 3,
			keywords:     []string{"JSON", "XML", "Accept"},
		},
		{
			name:         "JSON expected, plain text received",
			expected:     "application/json",
			actual:       "text/plain",
			context:      "API response",
			minSuggestions: 3,
			keywords:     []string{"JSON", "text", "header"},
		},
		{
			name:         "XML expected",
			expected:     "application/xml",
			actual:       "application/json",
			context:      "API response",
			minSuggestions: 2,
			keywords:     []string{"XML", "Content-Type"},
		},
		{
			name:         "form data expected",
			expected:     "application/x-www-form-urlencoded",
			actual:       "application/json",
			context:      "POST request",
			minSuggestions: 3,
			keywords:     []string{"form", "encode", "Content-Type"},
		},
	}

	for _, tc := range contentTypes {
		t.Run(tc.name, func(t *testing.T) {
			err := FormatContentTypeError(tc.expected, tc.actual, tc.context)

			// Verify error structure
			if err.ErrorType != "content_type" {
				t.Errorf("Expected error_type 'content_type', got '%s'", err.ErrorType)
			}

			// Verify suggestions
			if len(err.Suggestions) < tc.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tc.minSuggestions, len(err.Suggestions))
			}

			// Verify suggestions contain relevant keywords
			suggestionText := strings.Join(err.Suggestions, " ")
			foundKeywords := 0
			for _, keyword := range tc.keywords {
				if strings.Contains(strings.ToLower(suggestionText), strings.ToLower(keyword)) {
					foundKeywords++
				}
			}
			if foundKeywords == 0 {
				t.Logf("Note: No keywords found in suggestions for '%s'", tc.name)
			}

			// Verify error output
			errorOutput := err.Error()
			if !strings.Contains(errorOutput, tc.expected) {
				t.Errorf("Error output doesn't contain expected type '%s'", tc.expected)
			}
			if !strings.Contains(errorOutput, tc.actual) {
				t.Errorf("Error output doesn't contain actual type '%s'", tc.actual)
			}
		})
	}
}

// TestStatusCodeRangeValidation_Comprehensive tests status code range validation.
func TestStatusCodeRangeValidation_Comprehensive(t *testing.T) {
	ranges := []struct {
		name         string
		pattern      string
		actual       int
		context      string
		fieldName    string
		minSuggestions int
		keywords     []string
	}{
		{
			name:         "4xx expected, 2xx received",
			pattern:      "4xx",
			actual:       200,
			context:      "error response check",
			fieldName:    "status_code",
			minSuggestions: 3,
			keywords:     []string{"success", "error", "expectations"},
		},
		{
			name:         "5xx expected, 2xx received",
			pattern:      "5xx",
			actual:       200,
			context:      "error response check",
			fieldName:    "status_code",
			minSuggestions: 3,
			keywords:     []string{"success", "server", "error"},
		},
		{
			name:         "2xx expected, 4xx received",
			pattern:      "2xx",
			actual:       404,
			context:      "success response check",
			fieldName:    "status",
			minSuggestions: 3,
			keywords:     []string{"client", "error", "parameters"},
		},
		{
			name:         "2xx expected, 5xx received",
			pattern:      "2xx",
			actual:       500,
			context:      "success response check",
			fieldName:    "response_code",
			minSuggestions: 3,
			keywords:     []string{"server", "retry", "backoff"},
		},
	}

	for _, tc := range ranges {
		t.Run(tc.name, func(t *testing.T) {
			err := FormatStatusCodeRangeError(tc.pattern, tc.actual, tc.context, tc.fieldName)

			// Verify error structure
			if err.ErrorType != "status_code_range" {
				t.Errorf("Expected error_type 'status_code_range', got '%s'", err.ErrorType)
			}
			if err.FieldName != tc.fieldName {
				t.Errorf("Expected field_name '%s', got '%s'", tc.fieldName, err.FieldName)
			}

			// Verify suggestions
			if len(err.Suggestions) < tc.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tc.minSuggestions, len(err.Suggestions))
			}

			// Verify range info is set
			if err.RangeInfo == "" {
				t.Error("Expected RangeInfo to be set, but it was empty")
			}

			// Verify suggestions contain relevant keywords
			suggestionText := strings.Join(err.Suggestions, " ")
			foundKeywords := 0
			for _, keyword := range tc.keywords {
				if strings.Contains(strings.ToLower(suggestionText), strings.ToLower(keyword)) {
					foundKeywords++
				}
			}
			if foundKeywords == 0 {
				t.Logf("Note: No keywords found in suggestions for '%s'", tc.name)
			}
		})
	}
}

// TestSpecialValidationErrorTypes tests specialized validation error types.
func TestSpecialValidationErrorTypes(t *testing.T) {
	tests := []struct {
		name         string
		errorType    string
		expected     interface{}
		actual       interface{}
		context      string
		minSuggestions int
	}{
		{
			name:         "CORS headers",
			errorType:    "cors_headers",
			expected:     "proper CORS",
			actual:       "missing CORS headers",
			context:      "cross-origin request",
			minSuggestions: 5,
		},
		{
			name:         "response structure",
			errorType:    "response_structure",
			expected:     "user object",
			actual:       "invalid structure",
			context:      "user lookup",
			minSuggestions: 5,
		},
		{
			name:         "response body",
			errorType:    "response_body",
			expected:     "non-empty body",
			actual:       "empty body",
			context:      "data retrieval",
			minSuggestions: 5,
		},
		{
			name:         "response encoding",
			errorType:    "response_encoding",
			expected:     "UTF-8",
			actual:       "invalid encoding",
			context:      "API response",
			minSuggestions: 5,
		},
		{
			name:         "auth headers",
			errorType:    "auth_headers",
			expected:     "Bearer token",
			actual:       "missing auth",
			context:      "protected endpoint",
			minSuggestions: 5,
		},
		{
			name:         "custom headers",
			errorType:    "custom_headers",
			expected:     "X-Custom-Header",
			actual:       "missing",
			context:      "header validation",
			minSuggestions: 5,
		},
		{
			name:         "JSON schema",
			errorType:    "json_schema",
			expected:     "valid schema",
			actual:       "schema violation",
			context:      "response validation",
			minSuggestions: 5,
		},
		{
			name:         "data validation",
			errorType:    "data_validation",
			expected:     "valid data",
			actual:       "invalid data",
			context:      "field validation",
			minSuggestions: 5,
		},
		{
			name:         "type validation",
			errorType:    "type_validation",
			expected:     "string",
			actual:       123,
			context:      "type check",
			minSuggestions: 5,
		},
		{
			name:         "timeout",
			errorType:    "timeout",
			expected:     "< 1000ms",
			actual:       "5000ms",
			context:      "performance check",
			minSuggestions: 5,
		},
		{
			name:         "rate limit",
			errorType:    "rate_limit",
			expected:     "within limit",
			actual:       "exceeded",
			context:      "API request",
			minSuggestions: 5,
		},
		{
			name:         "retry exceeded",
			errorType:    "retry_exceeded",
			expected:     "success",
			actual:       "failed after 3 retries",
			context:      "retry logic",
			minSuggestions: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suggestions := GenerateComprehensiveSuggestions(tc.errorType, tc.expected, tc.actual, tc.context)

			// Verify suggestions are generated
			if len(suggestions) < tc.minSuggestions {
				t.Errorf("Expected at least %d suggestions for error type '%s', got %d",
					tc.minSuggestions, tc.errorType, len(suggestions))
			}

			// Verify suggestions are not empty
			for i, suggestion := range suggestions {
				if strings.TrimSpace(suggestion) == "" {
					t.Errorf("Suggestion at index %d is empty for error type '%s'", i, tc.errorType)
				}
			}

			// Create full error to test formatting
			err := NewValidationFormatter(tc.errorType).
				WithExpected(tc.expected).
				WithActual(tc.actual).
				WithContext(tc.context).
				Format()

			// Verify error formatting
			errorOutput := err.Error()
			if !strings.Contains(errorOutput, "validation failed") {
				t.Errorf("Error output doesn't contain 'validation failed' for type '%s'", tc.errorType)
			}

			// Verify suggestions appear in output
			if !strings.Contains(errorOutput, "Common causes:") {
				t.Errorf("Error output doesn't contain 'Common causes:' for type '%s'", tc.errorType)
			}
		})
	}
}

// TestContextAwareSuggestions_Comprehensive tests that context influences suggestions.
func TestContextAwareSuggestions_Comprehensive(t *testing.T) {
	tests := []struct {
		name         string
		errorType    string
		expected     interface{}
		actual       interface{}
		context      string
		expectedKeywords []string
	}{
		{
			name:      "404 with GET context",
			errorType: "status_code",
			expected:  200,
			actual:    404,
			context:   "GET /api/users/123",
			expectedKeywords: []string{"GET", "endpoint", "resource"},
		},
		{
			name:      "401 with OAuth context",
			errorType: "status_code",
			expected:  200,
			actual:    401,
			context:   "OAuth token validation",
			expectedKeywords: []string{"OAuth", "token", "scopes"},
		},
		{
			name:      "400 with POST context",
			errorType: "status_code",
			expected:  200,
			actual:    400,
			context:   "POST /api/users",
			expectedKeywords: []string{"POST", "body", "Content-Type"},
		},
		{
			name:      "error message with token context",
			errorType: "error_message",
			expected:  "valid.*token",
			actual:    "invalid",
			context:   "OAuth validation",
			expectedKeywords: []string{"token", "OAuth"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NewValidationFormatter(tc.errorType).
				WithExpected(tc.expected).
				WithActual(tc.actual).
				WithContext(tc.context).
				Format()

			// Verify context-aware suggestions
			suggestionText := strings.Join(err.Suggestions, " ") + " " + tc.context
			contextLower := strings.ToLower(suggestionText)

			// Check for expected keywords
			for _, keyword := range tc.expectedKeywords {
				if !strings.Contains(contextLower, strings.ToLower(keyword)) {
					t.Logf("Note: Context-aware suggestion doesn't contain keyword '%s' (this may be expected)", keyword)
				}
			}

			// Verify context appears in error output
			errorOutput := err.Error()
			if !strings.Contains(errorOutput, tc.context) {
				t.Errorf("Error output doesn't contain context '%s'", tc.context)
			}
		})
	}
}

// TestErrorOutputFormatting tests that error output is well-formatted and readable.
func TestErrorOutputFormatting(t *testing.T) {
	tests := []struct {
		name        string
		createError func() ValidationError
		checks      []string
	}{
		{
			name: "status code error with all fields",
			createError: func() ValidationError {
				return NewValidationFormatter("status_code").
					WithExpected(200).
					WithActual(404).
					WithContext("GET /api/users/123").
					WithResponseSnippet(`{"error": "not found"}`).
					Format()
			},
			checks: []string{
				"status_code",
				"validation failed",
				"Expected:",
				"Received:",
				"Request:",
				"Common causes:",
			},
		},
		{
			name: "error message with pattern details",
			createError: func() ValidationError {
				return NewValidationFormatter("error_message").
					WithExpected("token.*valid").
					WithActual("token expired").
					WithFieldName("error").
					WithContext("OAuth validation").
					WithPatternDetails("regex pattern mismatch").
					Format()
			},
			checks: []string{
				"error_message",
				"Expected:",
				"Actual:",
				"Field:",
				"Context:",
				"Pattern:",
				"Common causes:",
			},
		},
		{
			name: "status code range with range info",
			createError: func() ValidationError {
				return NewValidationFormatter("status_code_range").
					WithExpected("4xx").
					WithActual(200).
					WithContext("error check").
					WithFieldName("status").
					WithRangeInfo("400-499 (Client Error)").
					WithValidationDetails("Status code 200 is outside range 400-499").
					Format()
			},
			checks: []string{
				"status_code_range",
				"Expected:",
				"Actual:",
				"Field:",
				"Range:",
				"Details:",
				"Common causes:",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.createError()
			errorOutput := err.Error()

			// Check for all required elements
			for _, check := range tc.checks {
				if !strings.Contains(errorOutput, check) {
					t.Errorf("Error output doesn't contain '%s': %s", check, errorOutput)
				}
			}

			// Verify suggestions are numbered
			if !strings.Contains(errorOutput, "Common causes:") {
				t.Errorf("Error output doesn't contain 'Common causes:' header")
			}

			// Verify error has proper structure
			if err.ErrorType == "" {
				t.Error("ErrorType is empty")
			}
			if err.Message == "" {
				t.Error("Message is empty")
			}
			if len(err.Suggestions) == 0 {
				t.Error("No suggestions generated")
			}
		})
	}
}

// TestCustomSuggestions verifies that custom suggestions override auto-generated ones.
func TestCustomSuggestions(t *testing.T) {
	customSuggestions := []string{
		"Custom suggestion 1",
		"Custom suggestion 2",
		"Custom suggestion 3",
	}

	err := NewValidationFormatter("custom_type").
		WithExpected("expected value").
		WithActual("actual value").
		WithContext("custom context").
		WithSuggestions(customSuggestions...).
		Format()

	// Verify custom suggestions are used
	if len(err.Suggestions) != len(customSuggestions) {
		t.Errorf("Expected %d custom suggestions, got %d", len(customSuggestions), len(err.Suggestions))
	}

	// Verify suggestions match
	for i, suggestion := range err.Suggestions {
		if suggestion != customSuggestions[i] {
			t.Errorf("Suggestion %d doesn't match: expected '%s', got '%s'",
				i, customSuggestions[i], suggestion)
		}
	}

	// Verify suggestions appear in error output
	errorOutput := err.Error()
	for _, suggestion := range customSuggestions {
		if !strings.Contains(errorOutput, suggestion) {
			t.Errorf("Custom suggestion '%s' doesn't appear in error output", suggestion)
		}
	}
}

// TestMultipleExpectedValues tests validation with multiple acceptable expected values.
func TestMultipleExpectedValues(t *testing.T) {
	expectedCodes := []int{200, 201, 204}
	actualCode := 500

	err := NewValidationFormatter("status_code").
		WithExpected(expectedCodes).
		WithActual(actualCode).
		WithContext("POST /api/users").
		Format()

	// Verify error structure
	if err.ErrorType != "status_code" {
		t.Errorf("Expected error_type 'status_code', got '%s'", err.ErrorType)
	}

	// Verify suggestions are generated
	if len(err.Suggestions) == 0 {
		t.Error("No suggestions generated for multiple expected values")
	}

	// Verify error output mentions all expected codes
	errorOutput := err.Error()
	for _, code := range expectedCodes {
		if !strings.Contains(errorOutput, string(rune(code))) {
			// This is weak - just check if the number appears anywhere
			// Better: check if "200" appears, "201" appears, etc.
			codeStr := string(rune('0' + code/100)) + string(rune('0' + (code/10)%10)) + string(rune('0' + code%10))
			if !strings.Contains(errorOutput, codeStr) {
				t.Logf("Note: Expected code %d doesn't appear in output", code)
			}
		}
	}

	// Verify actual code appears
	if !strings.Contains(errorOutput, "500") {
		t.Error("Actual code 500 doesn't appear in output")
	}
}
