package validate

import (
	"strings"
	"testing"
)

/*
Integration Tests for Enhanced Error Suggestions

This test file provides comprehensive coverage for all validation error types
and their enhanced suggestion generation. These tests ensure that:

1. All error types generate relevant suggestions
2. Suggestions are clear and actionable
3. Common failure patterns are detected
4. Context-aware recommendations are provided
5. Error messages are helpful and complete

Test Coverage:
- HTTP status code errors (200, 400, 401, 403, 404, 409, 429, 500, 502, 503, 504)
- Error message pattern mismatches
- Content-Type validation errors
- Status code range errors (2xx, 4xx, 5xx)
- CORS header validation
- Response structure validation
- Authentication header validation
- Timeout and rate limit errors
- JSON schema validation
- Data and type validation
- Custom error types
*/

// TestGenerateComprehensiveSuggestions_StatusCodes verifies enhanced suggestions for all common HTTP status codes.
func TestGenerateComprehensiveSuggestions_StatusCodes(t *testing.T) {
	tests := []struct {
		name               string
		errorType          string
		expected           interface{}
		actual             interface{}
		context            string
		minSuggestions     int
		mustContain        []string
		mustNotContain     []string
	}{
		{
			name:        "404 Not Found",
			errorType:   "status_code",
			expected:    200,
			actual:      404,
			context:     "GET /api/users/123",
			minSuggestions: 3,
			mustContain: []string{
				"endpoint URL",
				"resource",
				"deleted or moved",
			},
		},
		{
			name:        "401 Unauthorized",
			errorType:   "status_code",
			expected:    200,
			actual:      401,
			context:     "OAuth token validation",
			minSuggestions: 3,
			mustContain: []string{
				"authentication",
				"token",
				"credentials",
			},
		},
		{
			name:        "403 Forbidden",
			errorType:   "status_code",
			expected:    200,
			actual:      403,
			context:     "User resource access",
			minSuggestions: 3,
			mustContain: []string{
				"permission",
				"authorization",
			},
		},
		{
			name:        "400 Bad Request",
			errorType:   "status_code",
			expected:    200,
			actual:      400,
			context:     "POST /api/users",
			minSuggestions: 3,
			mustContain: []string{
				"request",
				"body",
				"Content-Type",
			},
		},
		{
			name:        "429 Rate Limited",
			errorType:   "status_code",
			expected:    200,
			actual:      429,
			context:     "API request",
			minSuggestions: 3,
			mustContain: []string{
				"rate limit",
				"backoff",
				"Retry-After",
			},
		},
		{
			name:        "500 Internal Server Error",
			errorType:   "status_code",
			expected:    200,
			actual:      500,
			context:     "GET /api/data",
			minSuggestions: 3,
			mustContain: []string{
				"retry",
				"backoff",
				"service status",
			},
		},
		{
			name:        "502 Bad Gateway",
			errorType:   "status_code",
			expected:    200,
			actual:      502,
			context:     "API proxy request",
			minSuggestions: 3,
			mustContain: []string{
				"upstream",
				"retry",
				"load balancer",
			},
		},
		{
			name:        "503 Service Unavailable",
			errorType:   "status_code",
			expected:    200,
			actual:      503,
			context:     "API request",
			minSuggestions: 3,
			mustContain: []string{
				"retry",
				"maintenance",
				"circuit breaker",
			},
		},
		{
			name:        "504 Gateway Timeout",
			errorType:   "status_code",
			expected:    200,
			actual:      504,
			context:     "Slow API request",
			minSuggestions: 3,
			mustContain: []string{
				"timeout",
				"upstream",
				"latency",
			},
		},
		{
			name:        "409 Conflict",
			errorType:   "status_code",
			expected:    201,
			actual:      409,
			context:     "POST /api/users",
			minSuggestions: 3,
			mustContain: []string{
				"resource",
				"conflict",
				"unique",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := GenerateComprehensiveSuggestions(tt.errorType, tt.expected, tt.actual, tt.context)

			// Check minimum suggestion count
			if len(suggestions) < tt.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(suggestions))
			}

			// Check for must-contain strings
			for _, mustHave := range tt.mustContain {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(mustHave)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', but none did. Suggestions: %v", mustHave, suggestions)
				}
			}

			// Check for must-not-contain strings
			for _, mustNotHave := range tt.mustNotContain {
				for _, suggestion := range suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(mustNotHave)) {
						t.Errorf("Expected suggestions NOT to contain '%s', but found it in: %s", mustNotHave, suggestion)
					}
				}
			}

			// Ensure suggestions are actionable (start with verbs or check for keywords)
			actionableKeywords := []string{
				"verify", "check", "ensure", "implement", "review", "consider",
				"use", "add", "update", "validate", "test", "contact",
			}
			hasActionableSuggestion := false
			for _, suggestion := range suggestions {
				suggestionLower := strings.ToLower(suggestion)
				for _, keyword := range actionableKeywords {
					if strings.HasPrefix(suggestionLower, keyword) ||
					   strings.Contains(suggestionLower, " "+keyword) {
						hasActionableSuggestion = true
						break
					}
				}
				if hasActionableSuggestion {
					break
				}
			}
			if !hasActionableSuggestion {
				t.Errorf("Expected at least one actionable suggestion, got: %v", suggestions)
			}
		})
	}
}

// TestGenerateComprehensiveSuggestions_ErrorMessages verifies enhanced suggestions for error message patterns.
func TestGenerateComprehensiveSuggestions_ErrorMessages(t *testing.T) {
	tests := []struct {
		name            string
		errorType       string
		expected        interface{}
		actual          interface{}
		context         string
		minSuggestions  int
		mustContain     []string
	}{
		{
			name:        "Expired Token",
			errorType:   "error_message",
			expected:    "invalid.*token",
			actual:      "token expired",
			context:     "OAuth validation",
			minSuggestions: 3,
			mustContain: []string{
				"expired",
				"refresh",
				"token",
			},
		},
		{
			name:        "Invalid Token",
			errorType:   "error_message",
			expected:    "invalid.*token",
			actual:      "invalid token",
			context:     "API authentication",
			minSuggestions: 3,
			mustContain: []string{
				"invalid",
				"token",
				"format",
			},
		},
		{
			name:        "Access Denied",
			errorType:   "error_message",
			expected:    "success",
			actual:      "access denied",
			context:     "Resource access",
			minSuggestions: 3,
			mustContain: []string{
				"permission",
				"access",
				"authorization",
			},
		},
		{
			name:        "Not Found",
			errorType:   "error_message",
			expected:    "success",
			actual:      "resource not found",
			context:     "GET /api/users",
			minSuggestions: 3,
			mustContain: []string{
				"resource",
				"exists",
				"identifier",
			},
		},
		{
			name:        "Validation Failed",
			errorType:   "error_message",
			expected:    "success",
			actual:      "validation failed: required field missing",
			context:     "POST /api/users",
			minSuggestions: 3,
			mustContain: []string{
				"validation",
				"required",
				"field",
			},
		},
		{
			name:        "Rate Limit Exceeded",
			errorType:   "error_message",
			expected:    "success",
			actual:      "rate limit exceeded",
			context:     "API request",
			minSuggestions: 3,
			mustContain: []string{
				"rate limit",
				"backoff",
				"quota",
			},
		},
		{
			name:        "Timeout Error",
			errorType:   "error_message",
			expected:    "success",
			actual:      "request timeout",
			context:     "Slow API",
			minSuggestions: 3,
			mustContain: []string{
				"timeout",
				"retry",
				"backoff",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := GenerateComprehensiveSuggestions(tt.errorType, tt.expected, tt.actual, tt.context)

			if len(suggestions) < tt.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(suggestions))
			}

			for _, mustHave := range tt.mustContain {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(mustHave)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', but none did. Suggestions: %v", mustHave, suggestions)
				}
			}
		})
	}
}

// TestGenerateComprehensiveSuggestions_ContentType verifies enhanced suggestions for Content-Type errors.
func TestGenerateComprehensiveSuggestions_ContentType(t *testing.T) {
	tests := []struct {
		name            string
		errorType       string
		expected        interface{}
		actual          interface{}
		context         string
		minSuggestions  int
		mustContain     []string
	}{
		{
			name:        "Expected JSON got HTML",
			errorType:   "content_type",
			expected:    "application/json",
			actual:      "text/html",
			context:     "API response",
			minSuggestions: 3,
			mustContain: []string{
				"json",
				"html",
				"endpoint",
			},
		},
		{
			name:        "Expected JSON got XML",
			errorType:   "content_type",
			expected:    "application/json",
			actual:      "application/xml",
			context:     "API response",
			minSuggestions: 3,
			mustContain: []string{
				"json",
				"xml",
				"Accept",
			},
		},
		{
			name:        "Expected JSON got text",
			errorType:   "content_type",
			expected:    "application/json",
			actual:      "text/plain",
			context:     "API response",
			minSuggestions: 3,
			mustContain: []string{
				"json",
				"text",
				"Content-Type",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := GenerateComprehensiveSuggestions(tt.errorType, tt.expected, tt.actual, tt.context)

			if len(suggestions) < tt.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(suggestions))
			}

			for _, mustHave := range tt.mustContain {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(mustHave)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', but none did. Suggestions: %v", mustHave, suggestions)
				}
			}
		})
	}
}

// TestGenerateComprehensiveSuggestions_StatusCodeRange verifies enhanced suggestions for status code range errors.
func TestGenerateComprehensiveSuggestions_StatusCodeRange(t *testing.T) {
	tests := []struct {
		name            string
		errorType       string
		expected        interface{}
		actual          interface{}
		context         string
		minSuggestions  int
		mustContain     []string
	}{
		{
			name:        "Expected 4xx got 2xx",
			errorType:   "status_code_range",
			expected:    "4xx",
			actual:      200,
			context:     "Error response check",
			minSuggestions: 3,
			mustContain: []string{
				"success",
				"test expectations",
				"2xx",
			},
		},
		{
			name:        "Expected 4xx got 5xx",
			errorType:   "status_code_range",
			expected:    "4xx",
			actual:      500,
			context:     "Error response check",
			minSuggestions: 3,
			mustContain: []string{
				"server error",
				"5xx",
			},
		},
		{
			name:        "Expected 2xx got 4xx",
			errorType:   "status_code_range",
			expected:    "2xx",
			actual:      404,
			context:     "Success response check",
			minSuggestions: 3,
			mustContain: []string{
				"client error",
				"4xx",
				"request",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := GenerateComprehensiveSuggestions(tt.errorType, tt.expected, tt.actual, tt.context)

			if len(suggestions) < tt.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(suggestions))
			}

			for _, mustHave := range tt.mustContain {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(mustHave)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', but none did. Suggestions: %v", mustHave, suggestions)
				}
			}
		})
	}
}

// TestGenerateComprehensiveSuggestions_ValidationTypes verifies enhanced suggestions for various validation types.
func TestGenerateComprehensiveSuggestions_ValidationTypes(t *testing.T) {
	tests := []struct {
		name            string
		errorType       string
		expected        interface{}
		actual          interface{}
		context         string
		minSuggestions  int
		mustContain     []string
	}{
		{
			name:        "CORS Headers",
			errorType:   "cors_headers",
			expected:    "CORS headers present",
			actual:      "CORS headers missing",
			context:     "API request",
			minSuggestions: 5,
			mustContain: []string{
				"CORS",
				"Access-Control-Allow-Origin",
				"server",
			},
		},
		{
			name:        "Response Structure",
			errorType:   "response_structure",
			expected:    "valid structure",
			actual:      "invalid structure",
			context:     "API response",
			minSuggestions: 5,
			mustContain: []string{
				"structure",
				"format",
				"documentation",
			},
		},
		{
			name:        "Authentication Headers",
			errorType:   "auth_headers",
			expected:    "valid auth",
			actual:      "missing auth",
			context:     "API request",
			minSuggestions: 5,
			mustContain: []string{
				"authentication",
				"Authorization",
				"token",
			},
		},
		{
			name:        "Timeout Error",
			errorType:   "timeout",
			expected:    "< 1000ms",
			actual:      "5000ms",
			context:     "API request",
			minSuggestions: 5,
			mustContain: []string{
				"timeout",
				"retry",
				"backoff",
			},
		},
		{
			name:        "Rate Limit Error",
			errorType:   "rate_limit",
			expected:    "within limit",
			actual:      "rate exceeded",
			context:     "API request",
			minSuggestions: 5,
			mustContain: []string{
				"rate limit",
				"backoff",
				"quota",
			},
		},
		{
			name:        "JSON Schema Error",
			errorType:   "json_schema",
			expected:    "valid schema",
			actual:      "schema violation",
			context:     "API response",
			minSuggestions: 5,
			mustContain: []string{
				"schema",
				"validation",
				"required",
			},
		},
		{
			name:        "Data Validation Error",
			errorType:   "data_validation",
			expected:    "valid data",
			actual:      "invalid data",
			context:     "API request",
			minSuggestions: 5,
			mustContain: []string{
				"validation",
				"required",
				"format",
			},
		},
		{
			name:        "Type Validation Error",
			errorType:   "type_validation",
			expected:    "number",
			actual:      "string",
			context:     "API field",
			minSuggestions: 5,
			mustContain: []string{
				"type",
				"data type",
				"number",
				"string",
			},
		},
		{
			name:        "Retry Exceeded",
			errorType:   "retry_exceeded",
			expected:    "success",
			actual:      "too many retries",
			context:     "API request",
			minSuggestions: 5,
			mustContain: []string{
				"retry",
				"exceeded",
				"operation",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := GenerateComprehensiveSuggestions(tt.errorType, tt.expected, tt.actual, tt.context)

			if len(suggestions) < tt.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(suggestions))
			}

			for _, mustHave := range tt.mustContain {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(mustHave)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', but none did. Suggestions: %v", mustHave, suggestions)
				}
			}
		})
	}
}

// TestGenerateComprehensiveSuggestions_ContextAware verifies context-aware suggestion generation.
func TestGenerateComprehensiveSuggestions_ContextAware(t *testing.T) {
	tests := []struct {
		name            string
		errorType       string
		expected        interface{}
		actual          interface{}
		context         string
		minSuggestions  int
		mustContain     []string
	}{
		{
			name:        "404 with GET context",
			errorType:   "status_code",
			expected:    200,
			actual:      404,
			context:     "GET /api/users/123",
			minSuggestions: 4,
			mustContain: []string{
				"GET",
				"endpoint",
				"resource",
			},
		},
		{
			name:        "400 with POST context",
			errorType:   "status_code",
			expected:    201,
			actual:      400,
			context:     "POST /api/users",
			minSuggestions: 4,
			mustContain: []string{
				"POST",
				"request body",
				"Content-Type",
			},
		},
		{
			name:        "401 with OAuth context",
			errorType:   "status_code",
			expected:    200,
			actual:      401,
			context:     "OAuth token validation",
			minSuggestions: 4,
			mustContain: []string{
				"OAuth",
				"token",
				"authentication",
			},
		},
		{
			name:        "Expired token with token context",
			errorType:   "error_message",
			expected:    "valid token",
			actual:      "token expired",
			context:     "Token refresh logic",
			minSuggestions: 3,
			mustContain: []string{
				"expired",
				"refresh",
				"token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := GenerateComprehensiveSuggestions(tt.errorType, tt.expected, tt.actual, tt.context)

			if len(suggestions) < tt.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(suggestions))
			}

			for _, mustHave := range tt.mustContain {
				found := false
				for _, suggestion := range suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(mustHave)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', but none did. Suggestions: %v", mustHave, suggestions)
				}
			}
		})
	}
}

// TestValidationFormatter_WithEnhancedSuggestions tests the ValidationFormatter with enhanced suggestions.
func TestValidationFormatter_WithEnhancedSuggestions(t *testing.T) {
	tests := []struct {
		name               string
		validationType     string
		expected           interface{}
		actual             interface{}
		context            string
		minSuggestions     int
		hasActionableItems bool
	}{
		{
			name:            "404 status code",
			validationType:  "status_code",
			expected:        200,
			actual:          404,
			context:         "GET /api/users/123",
			minSuggestions:  3,
			hasActionableItems: true,
		},
		{
			name:            "Expired token error",
			validationType:  "error_message",
			expected:        "valid token",
			actual:          "token expired",
			context:         "OAuth validation",
			minSuggestions:  3,
			hasActionableItems: true,
		},
		{
			name:            "JSON content type error",
			validationType:  "content_type",
			expected:        "application/json",
			actual:          "text/html",
			context:         "API response",
			minSuggestions:  3,
			hasActionableItems: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the ValidationFormatter which now uses enhanced suggestions
			err := NewValidationFormatter(tt.validationType).
				WithExpected(tt.expected).
				WithActual(tt.actual).
				WithContext(tt.context).
				Format()

			// Check that suggestions were generated
			if len(err.Suggestions) < tt.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(err.Suggestions))
			}

			// Check that suggestions are actionable
			if tt.hasActionableItems {
				actionableKeywords := []string{
					"verify", "check", "ensure", "implement", "review", "consider",
					"use", "add", "update", "validate", "test", "contact",
				}
				hasActionable := false
				for _, suggestion := range err.Suggestions {
					suggestionLower := strings.ToLower(suggestion)
					for _, keyword := range actionableKeywords {
						if strings.HasPrefix(suggestionLower, keyword) ||
						   strings.Contains(suggestionLower, " "+keyword) {
							hasActionable = true
							break
						}
					}
					if hasActionable {
						break
					}
				}
				if !hasActionable {
					t.Errorf("Expected actionable suggestions, got: %v", err.Suggestions)
				}
			}

			// Check that error formatting works with enhanced suggestions
			errorOutput := err.Error()
			if !strings.Contains(errorOutput, "validation failed") {
				t.Errorf("Expected error output to contain 'validation failed', got: %s", errorOutput)
			}
		})
	}
}

// TestConvenienceFunctions_WithEnhancedSuggestions tests convenience functions with enhanced suggestions.
func TestConvenienceFunctions_WithEnhancedSuggestions(t *testing.T) {
	tests := []struct {
		name           string
		createError    func() ValidationError
		minSuggestions int
		mustContain    []string
	}{
		{
			name: "FormatStatusCodeError",
			createError: func() ValidationError {
				return FormatStatusCodeError(200, 404, "GET /api/users/123")
			},
			minSuggestions: 3,
			mustContain:    []string{"endpoint", "resource", "URL"},
		},
		{
			name: "FormatErrorMessageError",
			createError: func() ValidationError {
				return FormatErrorMessageError("invalid.*token", "token expired", "error", "OAuth validation")
			},
			minSuggestions: 2,
			mustContain:    []string{"token", "expired"},
		},
		{
			name: "FormatContentTypeError",
			createError: func() ValidationError {
				return FormatContentTypeError("application/json", "text/html", "API response")
			},
			minSuggestions: 3,
			mustContain:    []string{"Content-Type", "format"},
		},
		{
			name: "FormatStatusCodeRangeError",
			createError: func() ValidationError {
				return FormatStatusCodeRangeError("4xx", 200, "error response check", "status_code")
			},
			minSuggestions: 3,
			mustContain:    []string{"2xx", "success"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()

			if len(err.Suggestions) < tt.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(err.Suggestions))
			}

			for _, mustHave := range tt.mustContain {
				found := false
				for _, suggestion := range err.Suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(mustHave)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s', but none did. Suggestions: %v", mustHave, err.Suggestions)
				}
			}
		})
	}
}

// Benchmark_GenerateComprehensiveSuggestions benchmarks the enhanced suggestion generation.
func Benchmark_GenerateComprehensiveSuggestions(b *testing.B) {
	b.Run("404 status code", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			GenerateComprehensiveSuggestions("status_code", 200, 404, "GET /api/users/123")
		}
	})

	b.Run("Expired token", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			GenerateComprehensiveSuggestions("error_message", "valid token", "token expired", "OAuth validation")
		}
	})

	b.Run("Content type mismatch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			GenerateComprehensiveSuggestions("content_type", "application/json", "text/html", "API response")
		}
	})
}
