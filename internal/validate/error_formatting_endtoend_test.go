package validate

import (
	"strings"
	"testing"
)

/*
End-to-End Error Formatting Tests

This test file provides comprehensive end-to-end testing for the complete error
formatting system including:
1. Error message generation with suggestions
2. Error type formatting and categorization
3. Severity level handling
4. Context and field information
5. Expected/actual value display
6. Error serialization (JSON, map)
7. All error types with their enhanced suggestions

These tests ensure that the complete error reporting system works correctly
and produces clear, actionable error messages.
*/

// TestErrorFormatting_EndToEnd_StatusCodes tests complete error formatting for status code errors.
func TestErrorFormatting_EndToEnd_StatusCodes(t *testing.T) {
	tests := []struct {
		name            string
		createError     func() ValidationError
		mustContain     []string
		mustNotContain  []string
		hasSuggestions  bool
		minSuggestions  int
	}{
		{
			name: "404 Not Found with context",
			createError: func() ValidationError {
				return NewValidationFormatter("status_code").
					WithExpected(200).
					WithActual(404).
					WithContext("GET /api/users/123").
					Format()
			},
			mustContain: []string{
				"status_code",
				"validation failed",
				"404",
				"200",
				"GET /api/users/123",
				"Suggestions",
			},
			hasSuggestions: true,
			minSuggestions: 3,
		},
		{
			name: "401 Unauthorized with OAuth context",
			createError: func() ValidationError {
				return FormatStatusCodeError(200, 401, "OAuth token validation")
			},
			mustContain: []string{
				"401",
				"200",
				"OAuth",
			},
			hasSuggestions: true,
			minSuggestions: 3,
		},
		{
			name: "500 Internal Server Error",
			createError: func() ValidationError {
				return FormatStatusCodeError(200, 500, "GET /api/data")
			},
			mustContain: []string{
				"500",
				"200",
				"retry",
			},
			hasSuggestions: true,
			minSuggestions: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errorOutput := err.Error()

			// Check for must-contain strings
			for _, mustHave := range tt.mustContain {
				if !strings.Contains(errorOutput, mustHave) {
					t.Errorf("Expected error output to contain '%s', but it didn't. Output: %s", mustHave, errorOutput)
				}
			}

			// Check for must-not-contain strings
			for _, mustNotHave := range tt.mustNotContain {
				if strings.Contains(errorOutput, mustNotHave) {
					t.Errorf("Expected error output NOT to contain '%s', but it did. Output: %s", mustNotHave, errorOutput)
				}
			}

			// Check suggestions
			if tt.hasSuggestions {
				if len(err.Suggestions) < tt.minSuggestions {
					t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(err.Suggestions))
				}

				// Verify suggestions appear in error output
				if !strings.Contains(errorOutput, "Suggestions") {
					t.Errorf("Expected error output to contain 'Suggestions', but it didn't. Output: %s", errorOutput)
				}

				// Verify suggestions are bulleted
				hasBulletPoints := false
				for _, suggestion := range err.Suggestions {
					if strings.Contains(errorOutput, "- "+suggestion) {
						hasBulletPoints = true
						break
					}
				}
				if !hasBulletPoints {
					t.Errorf("Expected suggestions to be bulleted in error output. Output: %s", errorOutput)
				}
			}

			// Validate the error struct
			if err.ErrorType == "" {
				t.Errorf("Expected ErrorType to be set, but it was empty")
			}
			if err.Message == "" {
				t.Errorf("Expected Message to be set, but it was empty")
			}
		})
	}
}

// TestErrorFormatting_EndToEnd_ErrorMessages tests complete error formatting for error message validations.
func TestErrorFormatting_EndToEnd_ErrorMessages(t *testing.T) {
	tests := []struct {
		name            string
		createError     func() ValidationError
		mustContain     []string
		hasSuggestions  bool
		minSuggestions  int
	}{
		{
			name: "Expired token error",
			createError: func() ValidationError {
				return FormatErrorMessageError("valid.*token", "token expired", "error", "OAuth validation")
			},
			mustContain: []string{
				"error_message",
				"token",
				"expired",
			},
			hasSuggestions: true,
			minSuggestions: 2,
		},
		{
			name: "Invalid credentials error",
			createError: func() ValidationError {
				return NewValidationFormatter("error_message").
					WithExpected("success").
					WithActual("invalid credentials").
					WithFieldName("error").
					WithContext("User authentication").
					Format()
			},
			mustContain: []string{
				"error_message",
				"credentials",
				"error",
			},
			hasSuggestions: true,
			minSuggestions: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errorOutput := err.Error()

			for _, mustHave := range tt.mustContain {
				if !strings.Contains(errorOutput, mustHave) {
					t.Errorf("Expected error output to contain '%s', but it didn't. Output: %s", mustHave, errorOutput)
				}
			}

			if tt.hasSuggestions && len(err.Suggestions) < tt.minSuggestions {
				t.Errorf("Expected at least %d suggestions, got %d", tt.minSuggestions, len(err.Suggestions))
			}
		})
	}
}

// TestErrorFormatting_EndToEnd_ContentType tests complete error formatting for content-type validations.
func TestErrorFormatting_EndToEnd_ContentType(t *testing.T) {
	tests := []struct {
		name            string
		createError     func() ValidationError
		mustContain     []string
		hasSuggestions  bool
	}{
		{
			name: "JSON vs HTML mismatch",
			createError: func() ValidationError {
				return FormatContentTypeError("application/json", "text/html", "API response")
			},
			mustContain: []string{
				"content_type",
				"application/json",
				"text/html",
			},
			hasSuggestions: true,
		},
		{
			name: "JSON vs XML mismatch",
			createError: func() ValidationError {
				return NewValidationFormatter("content_type").
					WithExpected("application/json").
					WithActual("application/xml").
					WithContext("API response").
					Format()
			},
			mustContain: []string{
				"content_type",
				"json",
				"xml",
			},
			hasSuggestions: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errorOutput := err.Error()

			for _, mustHave := range tt.mustContain {
				if !strings.Contains(errorOutput, mustHave) {
					t.Errorf("Expected error output to contain '%s', but it didn't. Output: %s", mustHave, errorOutput)
				}
			}

			if tt.hasSuggestions && len(err.Suggestions) == 0 {
				t.Errorf("Expected suggestions to be generated, but got none")
			}
		})
	}
}

// TestErrorFormatting_EndToEnd_StatusCodeRange tests complete error formatting for status code range validations.
func TestErrorFormatting_EndToEnd_StatusCodeRange(t *testing.T) {
	tests := []struct {
		name            string
		createError     func() ValidationError
		mustContain     []string
		hasSuggestions  bool
	}{
		{
			name: "Expected 4xx got 2xx",
			createError: func() ValidationError {
				return FormatStatusCodeRangeError("4xx", 200, "error response check", "status_code")
			},
			mustContain: []string{
				"status_code_range",
				"4xx",
				"200",
			},
			hasSuggestions: true,
		},
		{
			name: "Expected 2xx got 5xx",
			createError: func() ValidationError {
				return FormatStatusCodeMinRangeError(200, 299, 500, "success response check", "status_code")
			},
			mustContain: []string{
				"status_code_range",
				"200-299",
				"500",
			},
			hasSuggestions: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errorOutput := err.Error()

			for _, mustHave := range tt.mustContain {
				if !strings.Contains(errorOutput, mustHave) {
					t.Errorf("Expected error output to contain '%s', but it didn't. Output: %s", mustHave, errorOutput)
				}
			}

			if tt.hasSuggestions && len(err.Suggestions) == 0 {
				t.Errorf("Expected suggestions to be generated, but got none")
			}
		})
	}
}

// TestErrorFormatting_EndToEnd_WithDetails tests complete error formatting with detailed information.
func TestErrorFormatting_EndToEnd_WithDetails(t *testing.T) {
	tests := []struct {
		name            string
		createError     func() ValidationError
		mustContain     []string
		hasDetails      bool
		hasSuggestions  bool
	}{
		{
			name: "Error with validation details",
			createError: func() ValidationError {
				return NewValidationFormatter("status_code").
					WithExpected(200).
					WithActual(404).
					WithContext("GET /api/users/123").
					WithValidationDetails(
						"Expected success response",
						"Received 404 Not Found",
						"Resource does not exist",
					).
					Format()
			},
			mustContain: []string{
				"status_code",
				"404",
				"200",
				"Expected success response",
			},
			hasDetails:     true,
			hasSuggestions: true,
		},
		{
			name: "Error with pattern details",
			createError: func() ValidationError {
				return NewValidationFormatter("error_message").
					WithExpected("invalid.*token").
					WithActual("access_denied").
					WithFieldName("error").
					WithPatternDetails("regex pattern 'invalid.*token' did not match").
					WithContext("OAuth validation").
					Format()
			},
			mustContain: []string{
				"error_message",
				"pattern",
				"access_denied",
			},
			hasDetails:     true,
			hasSuggestions: true,
		},
		{
			name: "Error with response snippet",
			createError: func() ValidationError {
				return NewValidationFormatter("error_message").
					WithExpected("error").
					WithActual("").
					WithResponseSnippet(`{"status": "ok", "data": {"id": 123}}`).
					WithContext("Error field validation").
					Format()
			},
			mustContain: []string{
				"error_message",
				"Response",
				"status",
			},
			hasDetails:     true,
			hasSuggestions: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errorOutput := err.Error()

			for _, mustHave := range tt.mustContain {
				if !strings.Contains(errorOutput, mustHave) {
					t.Errorf("Expected error output to contain '%s', but it didn't. Output: %s", mustHave, errorOutput)
				}
			}

			if tt.hasDetails {
				hasAnyDetails := len(err.ValidationDetails) > 0 || err.PatternDetails != "" || err.ResponseSnippet != ""
				if !hasAnyDetails {
					t.Errorf("Expected some details to be set (ValidationDetails, PatternDetails, or ResponseSnippet), but got none")
				}
			}

			if tt.hasSuggestions && len(err.Suggestions) == 0 {
				t.Errorf("Expected suggestions to be generated, but got none")
			}
		})
	}
}

// TestErrorFormatting_EndToEnd_CustomSuggestions tests error formatting with custom suggestions.
func TestErrorFormatting_EndToEnd_CustomSuggestions(t *testing.T) {
	tests := []struct {
		name            string
		createError     func() ValidationError
		customSuggestions []string
	}{
		{
			name: "Status code with custom suggestions",
			createError: func() ValidationError {
				return NewValidationFormatter("status_code").
					WithExpected(200).
					WithActual(404).
					WithContext("GET /api/users/123").
					WithSuggestions(
						"Check the endpoint documentation",
						"Verify the API version",
						"Test with curl to confirm",
					).
					Format()
			},
			customSuggestions: []string{
				"endpoint documentation",
				"API version",
				"curl",
			},
		},
		{
			name: "Error message with custom suggestions",
			createError: func() ValidationError {
				return NewValidationFormatter("error_message").
					WithExpected("valid token").
					WithActual("token expired").
					WithFieldName("error").
					WithSuggestions(
						"Implement automatic token refresh",
						"Check token expiration time",
						"Use refresh token to get new access token",
					).
					Format()
			},
			customSuggestions: []string{
				"automatic token refresh",
				"expiration time",
				"refresh token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()

			// Check that custom suggestions are used
			if len(err.Suggestions) == 0 {
				t.Errorf("Expected custom suggestions to be set, but got none")
			}

			// Check for custom suggestion content
			for _, expectedSuggestion := range tt.customSuggestions {
				found := false
				for _, suggestion := range err.Suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(expectedSuggestion)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected custom suggestions to contain '%s', but none did. Suggestions: %v", expectedSuggestion, err.Suggestions)
				}
			}

			// Verify error output contains suggestions
			errorOutput := err.Error()
			if !strings.Contains(errorOutput, "Suggestions") {
				t.Errorf("Expected error output to contain 'Suggestions', but it didn't. Output: %s", errorOutput)
			}
		})
	}
}

// TestErrorFormatting_EndToEnd_Serialization tests error serialization with enhanced suggestions.
func TestErrorFormatting_EndToEnd_Serialization(t *testing.T) {
	tests := []struct {
		name       string
		createError func() ValidationError
	}{
		{
			name: "Serialize status code error",
			createError: func() ValidationError {
				return FormatStatusCodeError(200, 404, "GET /api/users/123")
			},
		},
		{
			name: "Serialize error message with details",
			createError: func() ValidationError {
				return NewValidationFormatter("error_message").
					WithExpected("valid").
					WithActual("invalid").
					WithFieldName("error").
					WithPatternDetails("Pattern mismatch").
					WithSuggestions("Check pattern", "Verify input").
					Format()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()

			// Test ToMap() serialization
			errMap := err.ToMap()
			if errMap == nil {
				t.Errorf("Expected ToMap() to return a map, but got nil")
			}

			// Check required fields
			if _, ok := errMap["error_type"]; !ok {
				t.Errorf("Expected ToMap() to contain 'error_type', but it didn't")
			}
			if _, ok := errMap["message"]; !ok {
				t.Errorf("Expected ToMap() to contain 'message', but it didn't")
			}

			// Check suggestions if present
			if len(err.Suggestions) > 0 {
				if _, ok := errMap["suggestions"]; !ok {
					t.Errorf("Expected ToMap() to contain 'suggestions', but it didn't")
				}
			}

			// Test ValidationErrorData conversion
			data := ToValidationErrorData(err)
			if data.MessageType != "validation_failed" {
				t.Errorf("Expected MessageType to be 'validation_failed', got %s", data.MessageType)
			}
			if data.ErrorType != err.ErrorType {
				t.Errorf("Expected ErrorType %s, got %s", err.ErrorType, data.ErrorType)
			}
		})
	}
}

// TestErrorFormatting_EndToEnd_AllErrorTypes tests complete error formatting for all error types.
func TestErrorFormatting_EndToEnd_AllErrorTypes(t *testing.T) {
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
		"field_validation",
		"type_validation",
		"timeout",
		"rate_limit",
		"retry_exceeded",
	}

	for _, errorType := range errorTypes {
		t.Run(errorType, func(t *testing.T) {
			// Create error for this type
			err := NewValidationFormatter(errorType).
				WithExpected("expected_value").
				WithActual("actual_value").
				WithContext("Test context").
				Format()

			// Validate the error structure
			if err.ErrorType == "" {
				t.Errorf("Expected ErrorType to be set for %s, but it was empty", errorType)
			}
			if err.Message == "" {
				t.Errorf("Expected Message to be set for %s, but it was empty", errorType)
			}

			// Check that suggestions were generated
			if len(err.Suggestions) == 0 {
				t.Errorf("Expected suggestions to be generated for %s, but got none", errorType)
			}

			// Verify error output contains key elements
			errorOutput := err.Error()
			if !strings.Contains(errorOutput, errorType) {
				t.Errorf("Expected error output to contain error type '%s', but it didn't. Output: %s", errorType, errorOutput)
			}

			// Check for suggestions in output
			if !strings.Contains(errorOutput, "Suggestions") {
				t.Errorf("Expected error output to contain 'Suggestions' for %s, but it didn't. Output: %s", errorType, errorOutput)
			}
		})
	}
}

// TestErrorFormatting_EndToEnd_HelperFunction tests the helper function with enhanced suggestions.
func TestErrorFormatting_EndToEnd_HelperFunction(t *testing.T) {
	tests := []struct {
		name            string
		createError     func() ValidationError
		hasSuggestions  bool
	}{
		{
			name: "FormatValidationErrorHelper with status code",
			createError: func() ValidationError {
				return FormatValidationErrorHelper("status_code", 200, 404,
					WithValidationContext("GET /api/users/123"),
				)
			},
			hasSuggestions: true,
		},
		{
			name: "FormatValidationErrorHelper with error message",
			createError: func() ValidationError {
				return FormatValidationErrorHelper("error_message", "valid token", "expired token",
					WithFieldName("error"),
					WithValidationContext("OAuth validation"),
				)
			},
			hasSuggestions: true,
		},
		{
			name: "FormatValidationErrorHelper with custom suggestions",
			createError: func() ValidationError {
				return FormatValidationErrorHelper("custom_type", "expected", "actual",
					WithSuggestions("Custom suggestion 1", "Custom suggestion 2"),
				)
			},
			hasSuggestions: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()

			// Check suggestions
			if tt.hasSuggestions && len(err.Suggestions) == 0 {
				t.Errorf("Expected suggestions to be generated, but got none")
			}

			// Check error output
			errorOutput := err.Error()
			if !strings.Contains(errorOutput, "validation failed") {
				t.Errorf("Expected error output to contain 'validation failed', but it didn't. Output: %s", errorOutput)
			}

			if tt.hasSuggestions && !strings.Contains(errorOutput, "Suggestions") {
				t.Errorf("Expected error output to contain 'Suggestions', but it didn't. Output: %s", errorOutput)
			}
		})
	}
}

// TestErrorFormatting_EndToEnd_FormattingFunctions tests all formatting functions produce valid output.
func TestErrorFormatting_EndToEnd_FormattingFunctions(t *testing.T) {
	tests := []struct {
		name       string
		formatFunc func() string
		mustContain string
	}{
		{
			name: "FormatError",
			formatFunc: func() string {
				return FormatError(ErrTypeRequired, "Field is required", "email")
			},
			mustContain: "required",
		},
		{
			name: "FormatErrorWithSeverity",
			formatFunc: func() string {
				return FormatErrorWithSeverity(ErrTypeRequired, SeverityHigh, "Field is required", "email")
			},
			mustContain: "HIGH",
		},
		{
			name: "FormatHTTPError",
			formatFunc: func() string {
				return FormatHTTPError("status_code", 404, "Resource not found", "endpoint")
			},
			mustContain: "404",
		},
		{
			name: "FormatValidationErrorWithSeverity",
			formatFunc: func() string {
				return FormatValidationErrorWithSeverity(ErrTypeRequired, "Field is required", "email", nil, nil)
			},
			mustContain: "required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.formatFunc()
			if output == "" {
				t.Errorf("Expected formatting function to produce output, but got empty string")
			}
			if !strings.Contains(output, tt.mustContain) {
				t.Errorf("Expected output to contain '%s', but it didn't. Output: %s", tt.mustContain, output)
			}
		})
	}
}
