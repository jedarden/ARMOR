package validate

import (
	"fmt"
	"strings"
	"testing"
)

/*
Comprehensive Error Suggestions Test

This test file provides comprehensive verification that all validation error types
produce clear, actionable error messages with relevant suggestions. It tests:

1. All common validation failure types generate suggestions
2. All suggestions are clear and actionable
3. Error messages are helpful and complete
4. Context-aware suggestions work correctly
5. Custom suggestions override auto-generated ones
6. All error types are covered
*/

// TestAllErrorTypes_HaveActionableSuggestions verifies that every error type produces actionable suggestions.
func TestAllErrorTypes_HaveActionableSuggestions(t *testing.T) {
	errorTypes := []struct {
		errorType      string
		expected       interface{}
		actual         interface{}
		context        string
		minSuggestions int
		mustContainKeywords []string
	}{
		{
			errorType:      "status_code",
			expected:       200,
			actual:         404,
			context:        "GET /api/users/123",
			minSuggestions: 3,
			mustContainKeywords: []string{"endpoint", "resource"},
		},
		{
			errorType:      "error_message",
			expected:       "valid.*token",
			actual:         "token expired",
			context:        "OAuth validation",
			minSuggestions: 2,
			mustContainKeywords: []string{"token", "refresh", "expired"},
		},
		{
			errorType:      "content_type",
			expected:       "application/json",
			actual:         "text/html",
			context:        "API response",
			minSuggestions: 2,
			mustContainKeywords: []string{"content-type", "header", "format"},
		},
		{
			errorType:      "status_code_range",
			expected:       "4xx",
			actual:         200,
			context:        "Error response validation",
			minSuggestions: 2,
			mustContainKeywords: []string{"range", "code"},
		},
		{
			errorType:      "cors_headers",
			expected:       "CORS headers present",
			actual:         "CORS headers missing",
			context:        "Cross-origin request",
			minSuggestions: 2,
			mustContainKeywords: []string{"cors", "origin", "header"},
		},
		{
			errorType:      "response_structure",
			expected:       "valid structure",
			actual:         "invalid structure",
			context:        "Response validation",
			minSuggestions: 2,
			mustContainKeywords: []string{"structure", "field", "response"},
		},
		{
			errorType:      "response_body",
			expected:       "non-empty body",
			actual:         "empty body",
			context:        "Body validation",
			minSuggestions: 2,
			mustContainKeywords: []string{"body", "response", "data"},
		},
		{
			errorType:      "response_encoding",
			expected:       "UTF-8",
			actual:         "invalid encoding",
			context:        "Encoding validation",
			minSuggestions: 2,
			mustContainKeywords: []string{"encoding", "character"},
		},
		{
			errorType:      "auth_headers",
			expected:       "Bearer token",
			actual:         "missing auth",
			context:        "Authentication",
			minSuggestions: 2,
			mustContainKeywords: []string{"auth", "token", "header"},
		},
		{
			errorType:      "custom_headers",
			expected:       "X-Custom-Header: value",
			actual:         "missing",
			context:        "Custom header validation",
			minSuggestions: 2,
			mustContainKeywords: []string{"header", "custom"},
		},
		{
			errorType:      "json_schema",
			expected:       "valid schema",
			actual:         "schema violation",
			context:        "Schema validation",
			minSuggestions: 2,
			mustContainKeywords: []string{"schema", "field", "required"},
		},
		{
			errorType:      "data_validation",
			expected:       "valid data",
			actual:         "invalid data",
			context:        "Data validation",
			minSuggestions: 2,
			mustContainKeywords: []string{"validation", "field", "data"},
		},
		{
			errorType:      "field_validation",
			expected:       "valid field",
			actual:         "invalid field",
			context:        "Field validation",
			minSuggestions: 2,
			mustContainKeywords: []string{"field", "validation", "value"},
		},
		{
			errorType:      "type_validation",
			expected:       "string",
			actual:         123,
			context:        "Type validation",
			minSuggestions: 2,
			mustContainKeywords: []string{"type", "field", "data"},
		},
		{
			errorType:      "timeout",
			expected:       "< 5s",
			actual:         "30s",
			context:        "Request timeout",
			minSuggestions: 2,
			mustContainKeywords: []string{"timeout", "retry", "increase"},
		},
		{
			errorType:      "rate_limit",
			expected:       "under limit",
			actual:         "rate exceeded",
			context:        "Rate limiting",
			minSuggestions: 2,
			mustContainKeywords: []string{"rate", "limit", "backoff"},
		},
		{
			errorType:      "retry_exceeded",
			expected:       "success",
			actual:         "too many retries",
			context:        "Retry logic",
			minSuggestions: 2,
			mustContainKeywords: []string{"retry", "exceeded", "check"},
		},
	}

	for _, tt := range errorTypes {
		t.Run(tt.errorType, func(t *testing.T) {
			// Create validation error
			err := NewValidationFormatter(tt.errorType).
				WithExpected(tt.expected).
				WithActual(tt.actual).
				WithContext(tt.context).
				Format()

			// Verify suggestions were generated
			if len(err.Suggestions) < tt.minSuggestions {
				t.Errorf("Expected at least %d suggestions for %s, got %d",
					tt.minSuggestions, tt.errorType, len(err.Suggestions))
			}

			// Verify suggestions are actionable (contain action words)
			hasActionableSuggestions := false
			for _, suggestion := range err.Suggestions {
				if isActionableSuggestion(suggestion) {
					hasActionableSuggestions = true
					break
				}
			}
			if !hasActionableSuggestions {
				t.Errorf("Expected actionable suggestions for %s, but none found. Suggestions: %v",
					tt.errorType, err.Suggestions)
			}

			// Verify suggestions contain expected keywords
			for _, keyword := range tt.mustContainKeywords {
				found := false
				for _, suggestion := range err.Suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(keyword)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions for %s to contain keyword '%s', but none did. Suggestions: %v",
						tt.errorType, keyword, err.Suggestions)
				}
			}

			// Verify error output is clear and complete
			errorOutput := err.Error()
			if !strings.Contains(errorOutput, tt.errorType) {
				t.Errorf("Expected error output to contain error type '%s', but it didn't. Output: %s",
					tt.errorType, errorOutput)
			}

			// Status code validation uses "Expected:" and "Received:" instead of "Actual:"
			if !strings.Contains(errorOutput, "Expected") {
				t.Errorf("Expected error output to contain 'Expected', but it didn't. Output: %s", errorOutput)
			}

			// Check for either "Received:" (status codes) or "Actual:" (other types)
			if !strings.Contains(errorOutput, "Received") && !strings.Contains(errorOutput, "Actual") {
				t.Errorf("Expected error output to contain 'Received' or 'Actual', but it didn't. Output: %s", errorOutput)
			}

			if !strings.Contains(errorOutput, "Common causes") {
				t.Errorf("Expected error output to contain 'Common causes', but it didn't. Output: %s", errorOutput)
			}
		})
	}
}

// TestStatusCodeErrors_SpecificSuggestions verifies that specific status codes get appropriate suggestions.
func TestStatusCodeErrors_SpecificSuggestions(t *testing.T) {
	statusCodeTests := []struct {
		code           int
		expected       interface{}
		context        string
		mustContain    []string
		mustNotContain []string
	}{
		{
			code:        400,
			expected:    200,
			context:     "POST /api/users",
			mustContain: []string{"request", "body", "Content-Type"},
		},
		{
			code:        401,
			expected:    200,
			context:     "API request",
			mustContain: []string{"authentication", "token", "credentials"},
		},
		{
			code:        403,
			expected:    200,
			context:     "Resource access",
			mustContain: []string{"permission", "authorization", "access"},
		},
		{
			code:        404,
			expected:    200,
			context:     "GET /api/resource",
			mustContain: []string{"endpoint", "resource", "exists"},
		},
		{
			code:        409,
			expected:    201,
			context:     "POST /api/resource",
			mustContain: []string{"conflict", "exists", "duplicate"},
		},
		{
			code:        429,
			expected:    200,
			context:     "API request",
			mustContain: []string{"rate", "limit", "backoff"},
		},
		{
			code:        500,
			expected:    200,
			context:     "API request",
			mustContain: []string{"server", "retry", "backoff"},
		},
		{
			code:        502,
			expected:    200,
			context:     "API request",
			mustContain: []string{"upstream", "gateway", "retry"},
		},
		{
			code:        503,
			expected:    200,
			context:     "API request",
			mustContain: []string{"unavailable", "service", "retry"},
		},
		{
			code:        504,
			expected:    200,
			context:     "API request",
			mustContain: []string{"timeout", "upstream", "gateway"},
		},
	}

	for _, tt := range statusCodeTests {
		t.Run(fmt.Sprintf("Status_%d", tt.code), func(t *testing.T) {
			err := FormatStatusCodeError(tt.expected, tt.code, tt.context)

			// Verify suggestions
			if len(err.Suggestions) == 0 {
				t.Errorf("Expected suggestions for status code %d, but got none", tt.code)
			}

			// Check for must-contain keywords
			for _, keyword := range tt.mustContain {
				found := false
				for _, suggestion := range err.Suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(keyword)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions for status %d to contain '%s', but none did. Suggestions: %v",
						tt.code, keyword, err.Suggestions)
				}
			}

			// Check for must-not-contain keywords
			for _, keyword := range tt.mustNotContain {
				for _, suggestion := range err.Suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(keyword)) {
						t.Errorf("Expected suggestions for status %d NOT to contain '%s', but found: %s",
							tt.code, keyword, suggestion)
					}
				}
			}

			// Verify suggestions are actionable
			hasActionable := false
			for _, suggestion := range err.Suggestions {
				if isActionableSuggestion(suggestion) {
					hasActionable = true
					break
				}
			}
			if !hasActionable {
				t.Errorf("Expected actionable suggestions for status %d, but none found. Suggestions: %v",
					tt.code, err.Suggestions)
			}
		})
	}
}

// TestErrorMessagePatternMatching_Suggestions verifies pattern matching errors get specific suggestions.
func TestErrorMessagePatternMatching_Suggestions(t *testing.T) {
	patternTests := []struct {
		name           string
		expectedPattern string
		actualMessage  string
		context        string
		mustContain    []string
	}{
		{
			name:           "Token expired",
			expectedPattern: "valid.*token",
			actualMessage:  "token expired",
			context:        "OAuth validation",
			mustContain:    []string{"token", "expired", "refresh"},
		},
		{
			name:           "Invalid credentials",
			expectedPattern: "success",
			actualMessage:  "invalid credentials",
			context:        "Authentication",
			mustContain:    []string{"credentials", "authentication", "verify"},
		},
		{
			name:           "Resource not found",
			expectedPattern: "success",
			actualMessage:  "resource not found",
			context:        "Resource access",
			mustContain:    []string{"resource", "exists", "verify"},
		},
		{
			name:           "Rate limit exceeded",
			expectedPattern: "success",
			actualMessage:  "rate limit exceeded",
			context:        "API request",
			mustContain:    []string{"rate", "limit", "backoff"},
		},
		{
			name:           "Validation failed",
			expectedPattern: "success",
			actualMessage:  "validation failed: required field missing",
			context:        "Data validation",
			mustContain:    []string{"validation", "required", "field"},
		},
	}

	for _, tt := range patternTests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatErrorMessageError(tt.expectedPattern, tt.actualMessage, "error", tt.context)

			// Verify suggestions
			if len(err.Suggestions) == 0 {
				t.Errorf("Expected suggestions for pattern '%s', but got none", tt.name)
			}

			// Check for must-contain keywords
			for _, keyword := range tt.mustContain {
				found := false
				for _, suggestion := range err.Suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(keyword)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions for '%s' to contain '%s', but none did. Suggestions: %v",
						tt.name, keyword, err.Suggestions)
				}
			}

			// Verify suggestions are actionable
			hasActionable := false
			for _, suggestion := range err.Suggestions {
				if isActionableSuggestion(suggestion) {
					hasActionable = true
					break
				}
			}
			if !hasActionable {
				t.Errorf("Expected actionable suggestions for '%s', but none found. Suggestions: %v",
					tt.name, err.Suggestions)
			}
		})
	}
}

// TestContentTypeErrors_SpecificSuggestions verifies content-type errors get specific suggestions.
func TestContentTypeErrors_SpecificSuggestions(t *testing.T) {
	contentTypeTests := []struct {
		name        string
		expected    string
		actual      string
		mustContain []string
	}{
		{
			name:        "JSON vs HTML",
			expected:    "application/json",
			actual:      "text/html",
			mustContain: []string{"json", "html", "endpoint"},
		},
		{
			name:        "JSON vs XML",
			expected:    "application/json",
			actual:      "application/xml",
			mustContain: []string{"json", "xml", "accept"},
		},
		{
			name:        "JSON vs plain text",
			expected:    "application/json",
			actual:      "text/plain",
			mustContain: []string{"json", "content-type", "format"},
		},
		{
			name:        "Form vs JSON",
			expected:    "application/x-www-form-urlencoded",
			actual:      "application/json",
			mustContain: []string{"form", "content-type", "format"},
		},
	}

	for _, tt := range contentTypeTests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatContentTypeError(tt.expected, tt.actual, "API response")

			// Verify suggestions
			if len(err.Suggestions) == 0 {
				t.Errorf("Expected suggestions for content-type mismatch '%s', but got none", tt.name)
			}

			// Check for must-contain keywords
			for _, keyword := range tt.mustContain {
				found := false
				for _, suggestion := range err.Suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(keyword)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions for '%s' to contain '%s', but none did. Suggestions: %v",
						tt.name, keyword, err.Suggestions)
				}
			}
		})
	}
}

// TestCustomSuggestions_OverrideGenerated verifies custom suggestions override auto-generated ones.
func TestCustomSuggestions_OverrideGenerated(t *testing.T) {
	customSuggestions := []string{
		"Custom suggestion 1: Check configuration",
		"Custom suggestion 2: Review logs",
		"Custom suggestion 3: Contact support",
	}

	// Test with ValidationFormatter
	err := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users").
		WithSuggestions(customSuggestions...).
		Format()

	// Verify custom suggestions are used
	if len(err.Suggestions) != len(customSuggestions) {
		t.Errorf("Expected %d custom suggestions, got %d", len(customSuggestions), len(err.Suggestions))
	}

	for i, expected := range customSuggestions {
		if err.Suggestions[i] != expected {
			t.Errorf("Expected suggestion %d to be '%s', got '%s'", i, expected, err.Suggestions[i])
		}
	}

	// Verify custom suggestions appear in error output
	errorOutput := err.Error()
	for _, suggestion := range customSuggestions {
		if !strings.Contains(errorOutput, suggestion) {
			t.Errorf("Expected error output to contain custom suggestion '%s', but it didn't. Output: %s",
				suggestion, errorOutput)
		}
	}
}

// TestContextAwareSuggestions verifies context-aware suggestions work correctly.
func TestContextAwareSuggestions(t *testing.T) {
	contextTests := []struct {
		name        string
		errorType   string
		expected    interface{}
		actual      interface{}
		context     string
		mustContain []string
	}{
		{
			name:        "GET request with 404",
			errorType:   "status_code",
			expected:    200,
			actual:      404,
			context:     "GET /api/users/123",
			mustContain: []string{"GET", "endpoint"},
		},
		{
			name:        "POST request with 400",
			errorType:   "status_code",
			expected:    201,
			actual:      400,
			context:     "POST /api/users",
			mustContain: []string{"POST", "body"},
		},
		{
			name:        "OAuth with 401",
			errorType:   "status_code",
			expected:    200,
			actual:      401,
			context:     "OAuth token validation",
			mustContain: []string{"OAuth", "token"},
		},
		{
			name:        "DELETE with 404",
			errorType:   "status_code",
			expected:    200,
			actual:      404,
			context:     "DELETE /api/resource",
			mustContain: []string{"DELETE", "deleted"},
		},
	}

	for _, tt := range contextTests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationFormatter(tt.errorType).
				WithExpected(tt.expected).
				WithActual(tt.actual).
				WithContext(tt.context).
				Format()

			// Check for context-aware suggestions
			hasContextAware := false
			for _, suggestion := range err.Suggestions {
				for _, keyword := range tt.mustContain {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(keyword)) {
						hasContextAware = true
						break
					}
				}
				if hasContextAware {
					break
				}
			}

			if !hasContextAware {
				t.Errorf("Expected context-aware suggestions for '%s' with context '%s', but none found relevant to context. Suggestions: %v",
					tt.name, tt.context, err.Suggestions)
			}
		})
	}
}

// TestErrorOutput_Completeness verifies all error outputs contain required elements.
func TestErrorOutput_Completeness(t *testing.T) {
	errorCases := []struct {
		name     string
		createErr func() ValidationError
	}{
		{
			name: "Status code error",
			createErr: func() ValidationError {
				return FormatStatusCodeError(200, 404, "GET /api/resource")
			},
		},
		{
			name: "Error message validation",
			createErr: func() ValidationError {
				return FormatErrorMessageError("valid", "invalid", "error", "Validation")
			},
		},
		{
			name: "Content type validation",
			createErr: func() ValidationError {
				return FormatContentTypeError("application/json", "text/html", "API response")
			},
		},
		{
			name: "Status code range",
			createErr: func() ValidationError {
				return FormatStatusCodeRangeError("4xx", 200, "Error check", "status")
			},
		},
		{
			name: "Custom validation with details",
			createErr: func() ValidationError {
				return NewValidationFormatter("custom_validation").
					WithExpected("expected").
					WithActual("actual").
					WithContext("Custom context").
					WithValidationDetails("Detail 1", "Detail 2").
					WithResponseSnippet("Snippet...").
					WithFieldName("field").
					Format()
			},
		},
	}

	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createErr()
			errorOutput := err.Error()

			// Verify required elements are present
			requiredElements := []string{
				"validation failed",
				"Expected",
				"Actual",
				"Suggestions",
			}

			for _, element := range requiredElements {
				if !strings.Contains(errorOutput, element) {
					t.Errorf("Expected error output to contain '%s', but it didn't. Output: %s",
						element, errorOutput)
				}
			}

			// Verify error type is present
			if !strings.Contains(errorOutput, err.ErrorType) {
				t.Errorf("Expected error output to contain error type '%s', but it didn't. Output: %s",
					err.ErrorType, errorOutput)
			}

			// Verify context is present if provided
			if err.Context != "" && !strings.Contains(errorOutput, err.Context) {
				t.Errorf("Expected error output to contain context '%s', but it didn't. Output: %s",
					err.Context, errorOutput)
			}

			// Verify suggestions are formatted with bullet points
			if len(err.Suggestions) > 0 {
				hasBulletPoints := false
				for _, suggestion := range err.Suggestions {
					if strings.Contains(errorOutput, "- "+suggestion) {
						hasBulletPoints = true
						break
					}
				}
				if !hasBulletPoints {
					t.Errorf("Expected suggestions to be formatted with bullet points in error output. Output: %s", errorOutput)
				}
			}
		})
	}
}

// isActionableSuggestion checks if a suggestion contains actionable words.
func isActionableSuggestion(suggestion string) bool {
	actionWords := []string{
		"verify", "check", "ensure", "review", "validate", "confirm",
		"implement", "add", "update", "use", "consider", "try",
		"contact", "increase", "reduce", "test", "refresh",
		"verify", "check", "ensure", "add", "implement",
	}

	lowerSuggestion := strings.ToLower(suggestion)
	for _, word := range actionWords {
		if strings.Contains(lowerSuggestion, word) {
			return true
		}
	}
	return false
}

// TestSuggestions_AreClearAndConcise verifies suggestions are clear, concise, and actionable.
func TestSuggestions_AreClearAndConcise(t *testing.T) {
	// Create various error types and check suggestion quality
	errorCases := []ValidationError{
		FormatStatusCodeError(200, 404, "GET /api/resource"),
		FormatErrorMessageError("valid", "invalid", "error", "Validation"),
		FormatContentTypeError("application/json", "text/html", "API response"),
		NewValidationFormatter("timeout").
			WithExpected("<5s").
			WithActual("30s").
			WithContext("Database query").
			Format(),
	}

	for _, err := range errorCases {
		// Check each suggestion
		for _, suggestion := range err.Suggestions {
			// Suggestions should not be empty
			if strings.TrimSpace(suggestion) == "" {
				t.Errorf("Found empty suggestion in error type %s", err.ErrorType)
			}

			// Suggestions should be reasonably concise (< 150 chars)
			if len(suggestion) > 150 {
				t.Errorf("Suggestion too long (%d chars) for error type %s: %s",
					len(suggestion), err.ErrorType, suggestion)
			}

			// Suggestions should be actionable (start with action word)
			if !isActionableSuggestion(suggestion) {
				t.Errorf("Suggestion not actionable for error type %s: %s",
					err.ErrorType, suggestion)
			}

			// Suggestions should not be vague
			vaguePhrases := []string{"something", "thing", "somehow", "maybe"}
			for _, phrase := range vaguePhrases {
				if strings.Contains(strings.ToLower(suggestion), phrase) {
					t.Errorf("Suggestion contains vague phrase '%s' for error type %s: %s",
						phrase, err.ErrorType, suggestion)
				}
			}
		}
	}
}
