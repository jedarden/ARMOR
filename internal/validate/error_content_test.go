package validate

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestValidationError_Content_SingleStatusCodeMismatch verifies that single status code
// mismatch errors include expected code, actual code, and helpful context.
func TestValidationError_Content_SingleStatusCodeMismatch(t *testing.T) {
	tests := []struct {
		name           string
		expected       int
		actual         int
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:     "200 expected but got 404",
			expected: 200,
			actual:   404,
			mustContain: []string{
				"200",
				"404",
				"Expected:",
				"Received:",
			},
		},
		{
			name:     "201 expected but got 500",
			expected: 201,
			actual:   500,
			mustContain: []string{
				"201",
				"500",
			},
		},
		{
			name:     "401 expected but got 403",
			expected: 401,
			actual:   403,
			mustContain: []string{
				"401",
				"403",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a response with the actual status code
			resp := httptest.NewRecorder()
			resp.Code = tt.actual
			httpResp := resp.Result()

			// Validate with details
			result := ValidateStatusCodeWithDetails(httpResp, tt.expected)

			// Verify the result is invalid
			if result.Valid {
				t.Errorf("Expected validation to fail for status %d vs expected %d", tt.actual, tt.expected)
			}

			// Verify the mismatch details contain required information
			mismatchMsg := result.MismatchDetails
			if mismatchMsg == "" {
				t.Errorf("Expected mismatch details to be populated, got empty string")
			}

			// Check for required content
			for _, required := range tt.mustContain {
				if !strings.Contains(mismatchMsg, required) {
					t.Errorf("Expected mismatch details to contain '%s', got: %s", required, mismatchMsg)
				}
			}

			// Verify expected and actual codes are correctly stored
			if result.ActualCode != tt.actual {
				t.Errorf("Expected ActualCode to be %d, got %d", tt.actual, result.ActualCode)
			}

			if len(result.ExpectedCodes) != 1 || result.ExpectedCodes[0] != tt.expected {
				t.Errorf("Expected ExpectedCodes to be [%d], got %v", tt.expected, result.ExpectedCodes)
			}

			// Verify that no matched code is set
			if result.MatchedCode != nil {
				t.Errorf("Expected MatchedCode to be nil for failed validation, got %v", *result.MatchedCode)
			}
		})
	}
}

// TestValidationError_Content_MultipleStatusCodeMismatch verifies that multiple status code
// mismatch errors include the full list of expected codes and the actual code.
func TestValidationError_Content_MultipleStatusCodeMismatch(t *testing.T) {
	tests := []struct {
		name        string
		expected    []int
		actual      int
		mustContain []string
	}{
		{
			name:     "404 not in [200, 201, 204]",
			expected: []int{200, 201, 204},
			actual:   404,
			mustContain: []string{
				"200",
				"201",
				"204",
				"404",
				"one of",
			},
		},
		{
			name:     "500 not in [400, 401, 403, 404]",
			expected: []int{400, 401, 403, 404},
			actual:   500,
			mustContain: []string{
				"400",
				"401",
				"403",
				"404",
				"500",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a response with the actual status code
			resp := httptest.NewRecorder()
			resp.Code = tt.actual
			httpResp := resp.Result()

			// Validate with details
			result := ValidateStatusCodeWithDetails(httpResp, tt.expected)

			// Verify the result is invalid
			if result.Valid {
				t.Errorf("Expected validation to fail for status %d vs expected %v", tt.actual, tt.expected)
			}

			// Verify the mismatch details contain required information
			mismatchMsg := result.MismatchDetails
			if mismatchMsg == "" {
				t.Errorf("Expected mismatch details to be populated, got empty string")
			}

			// Check for required content
			for _, required := range tt.mustContain {
				if !strings.Contains(mismatchMsg, required) {
					t.Errorf("Expected mismatch details to contain '%s', got: %s", required, mismatchMsg)
				}
			}

			// Verify all expected codes are stored
			if len(result.ExpectedCodes) != len(tt.expected) {
				t.Errorf("Expected %d ExpectedCodes, got %d", len(tt.expected), len(result.ExpectedCodes))
			}
			for i, code := range tt.expected {
				if result.ExpectedCodes[i] != code {
					t.Errorf("Expected ExpectedCodes[%d] to be %d, got %d", i, code, result.ExpectedCodes[i])
				}
			}
		})
	}
}

// TestValidationError_Content_RangeValidation verifies that range validation errors
// include the range description, bounds, and actual value.
func TestValidationError_Content_RangeValidation(t *testing.T) {
	tests := []struct {
		name        string
		statusRange StatusCodeRange
		actual      int
		mustContain []string
	}{
		{
			name: "500 not in 4xx range",
			statusRange: StatusCodeRange{
				Min:         400,
				Max:         499,
				Description: "Client Error",
			},
			actual: 500,
			mustContain: []string{
				"500",
				"400",
				"499",
				"Client Error",
			},
		},
		{
			name: "200 not in 5xx range",
			statusRange: StatusCodeRange{
				Min:         500,
				Max:         599,
				Description: "Server Error",
			},
			actual: 200,
			mustContain: []string{
				"200",
				"500",
				"599",
				"Server Error",
			},
		},
		{
			name: "399 not in 2xx range",
			statusRange: StatusCodeRange{
				Min:         200,
				Max:         299,
				Description: "Success",
			},
			actual: 399,
			mustContain: []string{
				"399",
				"200",
				"299",
				"Success",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a response with the actual status code
			resp := httptest.NewRecorder()
			resp.Code = tt.actual
			httpResp := resp.Result()

			// Validate with details
			result := ValidateStatusCodeRangeWithDetails(httpResp, tt.statusRange)

			// Verify the result is invalid
			if result.Valid {
				t.Errorf("Expected validation to fail for status %d vs range %d-%d", tt.actual, tt.statusRange.Min, tt.statusRange.Max)
			}

			// Verify the mismatch details contain required information
			mismatchMsg := result.MismatchDetails
			if mismatchMsg == "" {
				t.Errorf("Expected mismatch details to be populated, got empty string")
			}

			// Check for required content
			for _, required := range tt.mustContain {
				if !strings.Contains(mismatchMsg, required) {
					t.Errorf("Expected mismatch details to contain '%s', got: %s", required, mismatchMsg)
				}
			}

			// Verify the category is set
			if result.Category != tt.statusRange.Description {
				t.Errorf("Expected Category to be '%s', got '%s'", tt.statusRange.Description, result.Category)
			}
		})
	}
}

// TestValidationError_Content_RangePatternInt verifies that range pattern validation
// errors include the pattern, expected range, and actual value.
func TestValidationError_Content_RangePatternInt(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		actual      int
		mustContain []string
	}{
		{
			name:    "200 not in 4xx pattern",
			pattern: "4xx",
			actual:  200,
			mustContain: []string{
				"200",
				"4xx",
				"400",
				"499",
				"Client Error",
			},
		},
		{
			name:    "404 not in 2xx pattern",
			pattern: "2xx",
			actual:  404,
			mustContain: []string{
				"404",
				"2xx",
				"200",
				"299",
				"Success",
			},
		},
		{
			name:    "500 not in 1xx pattern",
			pattern: "1xx",
			actual:  500,
			mustContain: []string{
				"500",
				"1xx",
				"100",
				"199",
				"Informational",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate with the pattern
			err := ValidateStatusCodeRangeInt(tt.pattern, tt.actual)

			// Verify an error is returned
			if err == nil {
				t.Errorf("Expected error for pattern '%s' with status %d", tt.pattern, tt.actual)
			}

			// Convert error to string for content checking
			errMsg := err.Error()

			// Check for required content
			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s', got: %s", required, errMsg)
				}
			}
		})
	}
}

// TestValidationError_Content_ErrorMessagePattern verifies that error message
// validation errors include the expected pattern and actual message content.
func TestValidationError_Content_ErrorMessagePattern(t *testing.T) {
	tests := []struct {
		name           string
		response       string
		expectedPattern string
		mustContain    []string
	}{
		{
			name:           "pattern 'not found' not in error message",
			response:       `{"error": "Access denied"}`,
			expectedPattern: "not found",
			mustContain: []string{
				"not found",
				"Access denied",
				"pattern",
			},
		},
		{
			name:           "regex pattern 'invalid.*token' not matched",
			response:       `{"message": "Authentication successful"}`,
			expectedPattern: "invalid.*token",
			mustContain: []string{
				"invalid.*token",
				"pattern",
				"Authentication",
			},
		},
		{
			name:           "long response with snippet",
			response:       `{"error": "` + strings.Repeat("very long error message ", 20) + `"}`,
			expectedPattern: "not found",
			mustContain: []string{
				"not found",
				"pattern",
				"Response",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate error message
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)

			// Verify an error is returned
			if err == nil {
				t.Errorf("Expected error for pattern '%s' with response: %s", tt.expectedPattern, tt.response)
			}

			// Convert error to string for content checking
			errMsg := err.Error()

			// Check for required content
			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s', got: %s", required, errMsg)
				}
			}
		})
	}
}

// TestValidationError_Content_FormatValidationError verifies that ValidationError
// formatting includes expected, actual, context, response snippet, and suggestions.
func TestValidationError_Content_FormatValidationError(t *testing.T) {
	tests := []struct {
		name            string
		validationType  string
		expected        interface{}
		actual          interface{}
		context         string
		responseSnippet string
		mustContain     []string
	}{
		{
			name:           "status code validation error",
			validationType: "status_code",
			expected:       200,
			actual:         404,
			context:        "GET /api/users",
			responseSnippet: `{"error": "User not found"}`,
			mustContain: []string{
				"status_code validation failed",
				"200",
				"404",
				"GET /api/users",
				"Suggestions",
			},
		},
		{
			name:           "error message validation error",
			validationType: "error_message",
			expected:       "invalid.*token",
			actual:         "access denied",
			context:        "OAuth token validation",
			responseSnippet: `{"error": "access_denied"}`,
			mustContain: []string{
				"error_message validation failed",
				"invalid.*token",
				"access denied",
				"OAuth",
				"Suggestions",
			},
		},
		{
			name:           "content type validation error",
			validationType: "content_type",
			expected:       "application/json",
			actual:         "text/html",
			context:        "API response parsing",
			responseSnippet: "<!DOCTYPE html>",
			mustContain: []string{
				"content_type validation failed",
				"application/json",
				"text/html",
				"Suggestions",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create validation error
			ve := FormatValidationError(
				tt.validationType,
				tt.expected,
				tt.actual,
				tt.context,
				tt.responseSnippet,
			)

			// Convert to string for content checking
			errMsg := ve.Error()

			// Check for required content
			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s'\nGot: %s", required, errMsg)
				}
			}

			// Verify suggestions are included
			if len(ve.Suggestions) == 0 {
				t.Errorf("Expected suggestions to be populated for validation type '%s'", tt.validationType)
			}

			// Verify error contains suggestions section
			if !strings.Contains(errMsg, "Suggestions:") {
				t.Errorf("Expected error message to contain 'Suggestions:' section, got: %s", errMsg)
			}
		})
	}
}

// TestValidationError_Content_StatusSuggestions verifies that status code
// suggestions are relevant to the actual status code category.
func TestValidationError_Content_StatusSuggestions(t *testing.T) {
	tests := []struct {
		name                string
		expected            interface{}
		actual              int
		expectedSuggestions []string
	}{
		{
			name:     "404 should include resource verification suggestions",
			expected: 200,
			actual:   404,
			expectedSuggestions: []string{
				"endpoint",
				"resource",
				"exists",
			},
		},
		{
			name:     "401 should include authentication suggestions",
			expected: 200,
			actual:   401,
			expectedSuggestions: []string{
				"authentication",
				"credentials",
				"token",
			},
		},
		{
			name:     "403 should include authorization suggestions",
			expected: 200,
			actual:   403,
			expectedSuggestions: []string{
				"permission",
				"scopes",
			},
		},
		{
			name:     "500 should include retry suggestions",
			expected: 200,
			actual:   500,
			expectedSuggestions: []string{
				"retry",
				"backoff",
				"service",
			},
		},
		{
			name:     "429 should include rate limiting suggestions",
			expected: 200,
			actual:   429,
			expectedSuggestions: []string{
				"rate limiting",
				"quota",
				"backoff",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create validation error
			ve := FormatValidationError("status_code", tt.expected, tt.actual, "", "")

			// Verify suggestions are included
			errMsg := ve.Error()

			// Check for expected suggestion keywords
			for _, keyword := range tt.expectedSuggestions {
				if !strings.Contains(errMsg, keyword) {
					t.Errorf("Expected suggestions to contain '%s' for status code %d\nGot: %s", keyword, tt.actual, errMsg)
				}
			}
		})
	}
}

// TestValidationError_Content_ErrorCategories verifies that error categories
// are correctly identified in detailed validation results.
func TestValidationError_Content_ErrorCategories(t *testing.T) {
	tests := []struct {
		name          string
		actual        int
		isClientError bool
		isServerError bool
		category      string
	}{
		{
			name:          "200 is success",
			actual:        200,
			isClientError: false,
			isServerError: false,
			category:      "success",
		},
		{
			name:          "404 is client error",
			actual:        404,
			isClientError: true,
			isServerError: false,
			category:      "client_error",
		},
		{
			name:          "500 is server error",
			actual:        500,
			isClientError: false,
			isServerError: true,
			category:      "server_error",
		},
		{
			name:          "301 is redirection",
			actual:        301,
			isClientError: false,
			isServerError: false,
			category:      "redirection",
		},
		{
			name:          "100 is informational",
			actual:        100,
			isClientError: false,
			isServerError: false,
			category:      "other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a response with the actual status code
			resp := httptest.NewRecorder()
			resp.Code = tt.actual
			httpResp := resp.Result()

			// Validate with details
			result := ValidateStatusCodeWithDetails(httpResp, 999) // Use invalid expected code to ensure mismatch

			// Verify category classification
			if result.IsClientError != tt.isClientError {
				t.Errorf("Expected IsClientError to be %v for status %d, got %v", tt.isClientError, tt.actual, result.IsClientError)
			}

			if result.IsServerError != tt.isServerError {
				t.Errorf("Expected IsServerError to be %v for status %d, got %v", tt.isServerError, tt.actual, result.IsServerError)
			}

			if result.Category != tt.category {
				t.Errorf("Expected Category to be '%s' for status %d, got '%s'", tt.category, tt.actual, result.Category)
			}
		})
	}
}

// TestValidationError_Content_ResponseSnippet verifies that response snippets
// are properly truncated and sanitized in error messages.
func TestValidationError_Content_ResponseSnippet(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		snippetContains string
		snippetOmits    string
	}{
		{
			name:            "short response included fully",
			response:        `{"error": "not found"}`,
			snippetContains: `{"error": "not found"}`,
			snippetOmits:    "",
		},
		{
			name:            "long response is truncated",
			response:        `{"error": "` + strings.Repeat("a", 300) + `"}`,
			snippetContains: "...",
			snippetOmits:    "",
		},
		{
			name:            "newlines are sanitized",
			response:        "{\n  \"error\": \"test\"\n}\n",
			snippetContains: "{",
			snippetOmits:    "",
		},
		{
			name:            "carriage returns are sanitized",
			response:        "{\r  \"error\": \"test\"\r}\r",
			snippetContains: "{",
			snippetOmits:    "\r",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate error message
			err := ValidateErrorMessage([]byte(tt.response), "xyz")

			// Verify an error is returned
			if err == nil {
				t.Errorf("Expected error for response: %s", tt.response)
			}

			// Convert error to string for content checking
			errMsg := err.Error()

			// Check for required content
			if tt.snippetContains != "" && !strings.Contains(errMsg, tt.snippetContains) {
				t.Errorf("Expected error message to contain '%s', got: %s", tt.snippetContains, errMsg)
			}

			// Check that sanitized content is omitted
			if tt.snippetOmits != "" && strings.Contains(errMsg, tt.snippetOmits) {
				t.Errorf("Expected error message to omit '%s', got: %s", tt.snippetOmits, errMsg)
			}
		})
	}
}

// TestValidationError_Content_DetailedResultFields verifies that all fields
// in detailed validation results are correctly populated.
func TestValidationError_Content_DetailedResultFields(t *testing.T) {
	// Create a response with status 404
	resp := httptest.NewRecorder()
	resp.Code = 404
	httpResp := resp.Result()

	// Validate with multiple expected codes
	expectedCodes := []int{200, 201, 204}
	result := ValidateStatusCodeWithDetails(httpResp, expectedCodes)

	// Verify all result fields are correctly populated
	if result.Valid {
		t.Error("Expected validation to fail")
	}

	if result.ActualCode != 404 {
		t.Errorf("Expected ActualCode to be 404, got %d", result.ActualCode)
	}

	if len(result.ExpectedCodes) != len(expectedCodes) {
		t.Errorf("Expected %d ExpectedCodes, got %d", len(expectedCodes), len(result.ExpectedCodes))
	}

	for i, code := range expectedCodes {
		if result.ExpectedCodes[i] != code {
			t.Errorf("Expected ExpectedCodes[%d] to be %d, got %d", i, code, result.ExpectedCodes[i])
		}
	}

	if result.MatchedCode != nil {
		t.Errorf("Expected MatchedCode to be nil for failed validation, got %v", *result.MatchedCode)
	}

	if result.MismatchDetails == "" {
		t.Error("Expected MismatchDetails to be populated")
	}

	if !result.IsClientError {
		t.Error("Expected IsClientError to be true for 404")
	}

	if result.IsServerError {
		t.Error("Expected IsServerError to be false for 404")
	}

	if result.Category != "client_error" {
		t.Errorf("Expected Category to be 'client_error', got '%s'", result.Category)
	}
}

// TestValidationError_Content_ErrorMessageValidationResult verifies that
// error message validation results include all required fields.
func TestValidationError_Content_ErrorMessageValidationResult(t *testing.T) {
	tests := []struct {
		name              string
		response          interface{}
		pattern           EnhancedErrorMessagePattern
		wantValid         bool
		wantFound         bool
		wantPatternMatched bool
		mustContainIssues []string
	}{
		{
			name:     "successful validation with pattern match",
			response: map[string]interface{}{"error": "invalid token"},
			pattern: EnhancedErrorMessagePattern{
				Pattern: "invalid.*token",
			},
			wantValid:         true,
			wantFound:         true,
			wantPatternMatched: true,
			mustContainIssues:  nil,
		},
		{
			name:     "failed validation - no pattern match",
			response: map[string]interface{}{"error": "access denied"},
			pattern: EnhancedErrorMessagePattern{
				Pattern: "invalid.*token",
			},
			wantValid:         false,
			wantFound:         true,
			wantPatternMatched: false,
			mustContainIssues: []string{
				"pattern",
				"invalid.*token",
			},
		},
		{
			name:     "failed validation - must contain missing",
			response: map[string]interface{}{"error": "something"},
			pattern: EnhancedErrorMessagePattern{
				MustContain: []string{"invalid", "token"},
			},
			wantValid:         false,
			wantFound:         true,
			wantPatternMatched: true,
			mustContainIssues: []string{
				"invalid",
				"token",
				"required",
			},
		},
		{
			name:     "failed validation - length too short",
			response: map[string]interface{}{"error": "short"},
			pattern: EnhancedErrorMessagePattern{
				MinLength: 10,
			},
			wantValid:         false,
			wantFound:         true,
			wantPatternMatched: true,
			mustContainIssues: []string{
				"length",
				"minimum",
				"10",
			},
		},
		{
			name:     "failed validation - contains forbidden string",
			response: map[string]interface{}{"error": "invalid password"},
			pattern: EnhancedErrorMessagePattern{
				MustNotContain: []string{"password", "secret"},
			},
			wantValid:         false,
			wantFound:         true,
			wantPatternMatched: true,
			mustContainIssues: []string{
				"password",
				"forbidden",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateErrorMessageWithDetails(tt.response, tt.pattern)

			// Verify basic validation results
			if result.Valid != tt.wantValid {
				t.Errorf("Expected Valid to be %v, got %v", tt.wantValid, result.Valid)
			}

			if result.Found != tt.wantFound {
				t.Errorf("Expected Found to be %v, got %v", tt.wantFound, result.Found)
			}

			if result.PatternMatched != tt.wantPatternMatched {
				t.Errorf("Expected PatternMatched to be %v, got %v", tt.wantPatternMatched, result.PatternMatched)
			}

			// Verify issues contain expected content
			if tt.mustContainIssues != nil {
				if len(result.Issues) == 0 {
					t.Error("Expected Issues to be populated for failed validation")
				}

				for _, expectedIssue := range tt.mustContainIssues {
					found := false
					for _, issue := range result.Issues {
						if strings.Contains(issue, expectedIssue) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected Issues to contain '%s', got: %v", expectedIssue, result.Issues)
					}
				}
			}
		})
	}
}

// Example demonstrating how to use detailed validation error information
func ExampleValidateStatusCodeWithDetails_errorContent() {
	// Create a response with status 404
	resp := httptest.NewRecorder()
	resp.Code = 404
	httpResp := resp.Result()

	// Validate with expected status 200 (no context)
	result := ValidateStatusCodeWithDetails(httpResp, 200)

	// Use detailed error information
	if !result.Valid {
		fmt.Printf("Validation failed:\n%s\n", result.MismatchDetails)
		fmt.Printf("Category: %s\n", result.Category)
	}

	// Output:
	// Validation failed:
	// Status code category: 4xx - Client error responses
	// Expected: 200 (OK)
	// Received: 404 (Not Found)
	// Common causes:
	//   1. Verify the endpoint URL is correct
	//   2. Check if the resource ID or identifier exists
	//   3. Ensure the resource hasn't been deleted or moved
	//   4. Review API versioning (URL path may have changed)
	// Category: client_error
}

// ExampleValidateStatusCodeWithDetails_withContext demonstrates validation with request context
func ExampleValidateStatusCodeWithDetails_withContext() {
	// Create a response with status 401
	resp := httptest.NewRecorder()
	resp.Code = 401
	httpResp := resp.Result()

	// Validate with expected status 200 and provide request context
	result := ValidateStatusCodeWithDetails(httpResp, 200, "POST /api/users")

	// Use detailed error information
	if !result.Valid {
		fmt.Printf("Validation failed:\n%s\n", result.MismatchDetails)
	}

	// Output:
	// Validation failed:
	// Request: POST /api/users
	// Status code category: 4xx - Client error responses
	// Expected: 200 (OK)
	// Received: 401 (Unauthorized)
	// Common causes:
	//   1. Verify authentication credentials are correct
	//   2. Check if API token or session has expired
	//   3. Ensure Authorization header is properly formatted (e.g., 'Bearer <token>')
	//   4. Confirm authentication method is supported
}

// ExampleValidateStatusCodeWithDetails_rateLimit demonstrates 429 rate limit error with suggestions
func ExampleValidateStatusCodeWithDetails_rateLimit() {
	// Create a response with status 429
	resp := httptest.NewRecorder()
	resp.Code = 429
	httpResp := resp.Result()

	// Validate with expected status 200
	result := ValidateStatusCodeWithDetails(httpResp, 200, "GET /api/data")

	// Use detailed error information
	if !result.Valid {
		fmt.Printf("Validation failed:\n%s\n", result.MismatchDetails)
	}

	// Output:
	// Validation failed:
	// Request: GET /api/data
	// Status code category: 4xx - Client error responses
	// Expected: 200 (OK)
	// Received: 429 (Too Many Requests)
	// Common causes:
	//   1. Implement rate limiting and exponential backoff
	//   2. Check API quota limits and current usage
	//   3. Consider caching responses to reduce request frequency
	//   4. Review rate limit headers (Retry-After, X-RateLimit-Remaining)
}

// ExampleValidateStatusCodeWithDetails_serverError demonstrates 500 server error with suggestions
func ExampleValidateStatusCodeWithDetails_serverError() {
	// Create a response with status 500
	resp := httptest.NewRecorder()
	resp.Code = 500
	httpResp := resp.Result()

	// Validate with expected status 200
	result := ValidateStatusCodeWithDetails(httpResp, 200, "DELETE /api/resource/123")

	// Use detailed error information
	if !result.Valid {
		fmt.Printf("Validation failed:\n%s\n", result.MismatchDetails)
		fmt.Printf("Is server error: %v\n", result.IsServerError)
	}

	// Output:
	// Validation failed:
	// Request: DELETE /api/resource/123
	// Status code category: 5xx - Server error responses
	// Expected: 200 (OK)
	// Received: 500 (Internal Server Error)
	// Common causes:
	//   1. Implement retry logic with exponential backoff
	//   2. Check service status page for ongoing issues
	//   3. Contact support if the issue persists
	//   4. Verify request doesn't trigger server-side bugs
	// Is server error: true
}

// TestValidationError_Content_ExtendedFields verifies that extended ValidationError
// fields are properly populated and displayed in error messages.
func TestValidationError_Content_ExtendedFields(t *testing.T) {
	tests := []struct {
		name              string
		validationType   string
		expected          interface{}
		actual            interface{}
		context           string
		responseSnippet   string
		fieldName         string
		location          string
		relatedFields     []string
		patternDetails    string
		rangeInfo         string
		validationDetails []string
		mustContain       []string
	}{
		{
			name:            "error message with field name and pattern details",
			validationType:  "error_message",
			expected:        "invalid.*token",
			actual:          "access denied",
			context:         "OAuth token validation",
			responseSnippet: `{"error": "access_denied"}`,
			fieldName:       "error",
			location:        "response body",
			relatedFields:   nil,
			patternDetails:  "regex pattern 'invalid.*token' did not match",
			rangeInfo:       "",
			validationDetails: []string{
				"No matching error field found",
				"Checked 3 error message fields",
			},
			mustContain: []string{
				"error_message validation failed",
				"invalid.*token",
				"access denied",
				"Field:",
				"error",
				"Pattern:",
				"regex pattern",
				"Details:",
				"No matching error field found",
			},
		},
		{
			name:            "status code range with range information",
			validationType:  "status_code_range",
			expected:        "4xx Client Error (400-499)",
			actual:          200,
			context:         "error response validation",
			responseSnippet: "",
			fieldName:       "",
			location:        "",
			relatedFields:   nil,
			patternDetails:  "",
			rangeInfo:       "Range: 400-499 (Client Error)",
			validationDetails: []string{
				"Status code 200 is outside range 400-499",
				"Range '4xx' represents Client Error",
			},
			mustContain: []string{
				"status_code_range validation failed",
				"4xx",
				"400-499",
				"Client Error",
				"Range:",
				"400-499",
				"Details:",
				"Status code 200 is outside range",
			},
		},
		{
			name:            "comprehensive error with all fields",
			validationType:  "error_message",
			expected:        "not found",
			actual:          "internal server error",
			context:         "resource access validation",
			responseSnippet: `{"error": "internal server error"}`,
			fieldName:       "error",
			location:        "response body",
			relatedFields:   []string{"error", "message"},
			patternDetails:  "substring 'not found' not found in message",
			rangeInfo:       "",
			validationDetails: []string{
				"Expected pattern did not match",
				"Message length: 20 characters",
			},
			mustContain: []string{
				"error_message validation failed",
				"not found",
				"internal server error",
				"Field:",
				"error",
				"Pattern:",
				"substring 'not found'",
				"Context:",
				"resource access validation",
				"Details:",
				"Expected pattern did not match",
				"Response:",
				"internal server error",
			},
		},
	}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ve := FormatValidationErrorWithDetails(
					tt.validationType,
					tt.expected,
					tt.actual,
					tt.context,
					tt.responseSnippet,
					tt.fieldName,
					tt.location,
					tt.relatedFields,
					tt.patternDetails,
					tt.rangeInfo,
					tt.validationDetails,
				)

			errMsg := ve.Error()

			// Verify all required content is present
			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s'\nGot: %s", required, errMsg)
				}
			}

			// Verify field-specific values are correctly set
			if ve.FieldName != tt.fieldName {
				t.Errorf("Expected FieldName to be '%s', got '%s'", tt.fieldName, ve.FieldName)
			}

			if ve.PatternDetails != tt.patternDetails {
				t.Errorf("Expected PatternDetails to be '%s', got '%s'", tt.patternDetails, ve.PatternDetails)
			}

			if ve.RangeInfo != tt.rangeInfo {
				t.Errorf("Expected RangeInfo to be '%s', got '%s'", tt.rangeInfo, ve.RangeInfo)
			}

			if len(ve.ValidationDetails) != len(tt.validationDetails) {
				t.Errorf("Expected %d ValidationDetails, got %d", len(tt.validationDetails), len(ve.ValidationDetails))
			}
		})
	}
}

// TestValidationError_Content_EnhancedErrorMessageValidation verifies that
// ValidateErrorMessage provides detailed context including field names and pattern details.
func TestValidationError_Content_EnhancedErrorMessageValidation(t *testing.T) {
	tests := []struct {
		name           string
		response       string
		expectedPattern string
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:           "empty response with detailed error",
			response:       "",
			expectedPattern: "not found",
			mustContain: []string{
				"error_message validation failed",
				"non-empty response",
				"Details:",
				"Response body is empty",
			},
		},
		{
			name:           "empty pattern with detailed error",
			response:       `{"error": "test"}`,
			expectedPattern: "",
			mustContain: []string{
				"error_message validation failed",
				"non-empty pattern",
				"Details:",
				"Expected pattern cannot be empty",
			},
		},
		{
			name:           "invalid JSON with detailed error",
			response:       `{invalid json}`,
			expectedPattern: "test",
			mustContain: []string{
				"error_message validation failed",
				"valid JSON",
				"parse error",
				"Details:",
				"Failed to parse response body",
			},
		},
		{
			name:           "no error field with detailed error",
			response:       `{"status": "ok"}`,
			expectedPattern: "test",
			mustContain: []string{
				"error_message validation failed",
				"error message field",
				"no error message found",
				"Details:",
				"No error message found in response body",
				"Checked fields:",
				"error, message, detail, description, error_description",
			},
		},
		{
			name:           "invalid regex pattern with detailed error",
			response:       `{"error": "test"}`,
			expectedPattern: "[invalid(",
			mustContain: []string{
				"error_message validation failed",
				"valid regex pattern",
				"regex compilation",
				"Details:",
				"Invalid regex pattern",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)

			if err == nil {
				t.Errorf("Expected error for response: %s with pattern: %s", tt.response, tt.expectedPattern)
				return
			}

			errMsg := err.Error()

			// Verify required content
			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s'\nGot: %s", required, errMsg)
				}
			}

			// Verify forbidden content is not present
			for _, forbidden := range tt.mustNotContain {
				if strings.Contains(errMsg, forbidden) {
					t.Errorf("Expected error message to NOT contain '%s'\nGot: %s", forbidden, errMsg)
				}
			}
		})
	}
}

// TestValidationError_Content_EnhancedRangeValidation verifies that
// ValidateStatusCodeRangeInt provides detailed context in error messages.
func TestValidationError_Content_EnhancedRangeValidation(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		actual      int
		mustContain []string
	}{
		{
			name:    "invalid pattern format with details",
			pattern: "4x",
			actual:  404,
			mustContain: []string{
				"status_code_range validation failed",
				"pattern in format 'Nxx'",
				"Details:",
				"Pattern '4x' has invalid length",
				"Pattern must be exactly 3 characters",
			},
		},
		{
			name:    "invalid century digit with details",
			pattern: "0xx",
			actual:  404,
			mustContain: []string{
				"status_code_range validation failed",
				"century digit 1-5",
				"Details:",
				"Pattern '0xx' has invalid century digit",
				"Valid century digits: 1 (1xx), 2 (2xx), 3 (3xx), 4 (4xx), 5 (5xx)",
			},
		},
		{
			name:    "invalid suffix with details",
			pattern: "4yy",
			actual:  404,
			mustContain: []string{
				"status_code_range validation failed",
				"pattern ending with 'xx'",
				"Details:",
				"Pattern '4yy' has invalid suffix",
				"Pattern must end with 'xx'",
			},
		},
		{
			name:    "status code outside range with details",
			pattern: "4xx",
			actual:  200,
			mustContain: []string{
				"status_code_range validation failed",
				"4xx",
				"Client Error",
				"400-499",
				"200",
				"Details:",
				"Status code 200 is outside range 400-499",
				"Range:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodeRangeInt(tt.pattern, tt.actual)

			if err == nil {
				t.Errorf("Expected error for pattern '%s' with status %d", tt.pattern, tt.actual)
				return
			}

			errMsg := err.Error()

			// Verify required content
			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s'\nGot: %s", required, errMsg)
				}
			}
		})
	}
}

// TestValidationError_Content_PatternMatchingDetails verifies that pattern
// validation errors include detailed pattern information.
func TestValidationError_Content_PatternMatchingDetails(t *testing.T) {
	tests := []struct {
		name           string
		response       string
		expectedPattern string
		isRegex        bool
		mustContain    []string
	}{
		{
			name:           "regex pattern not matched with details",
			response:       `{"error": "access denied"}`,
			expectedPattern: "invalid.*token",
			isRegex:        true,
			mustContain: []string{
				"regex pattern 'invalid.*token'",
				"Field:",
				"error",
				"Pattern:",
				"regex pattern",
				"Details:",
				"Searched for regex pattern",
				"Actual message:",
			},
		},
		{
			name:           "substring pattern not matched with details",
			response:       `{"message": "Authentication successful"}`,
			expectedPattern: "failed",
			isRegex:        false,
			mustContain: []string{
				"substring 'failed'",
				"Field:",
				"message",
				"Pattern:",
				"substring 'failed'",
				"Details:",
				"Searched for substring",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)

			if err == nil {
				t.Errorf("Expected error for pattern '%s' with response: %s", tt.expectedPattern, tt.response)
				return
			}

			errMsg := err.Error()

			// Verify required content
			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s'\nGot: %s", required, errMsg)
				}
			}
		})
	}
}

// TestValidationError_Content_MultipleMessageFields verifies that when
// multiple error message fields are present, validation includes this context.
func TestValidationError_Content_MultipleMessageFields(t *testing.T) {
	response := `{
		"error": "primary error",
		"message": "secondary message",
		"detail": "detailed description"
	}`

	err := ValidateErrorMessage([]byte(response), "not found")

	if err == nil {
		t.Error("Expected error when pattern not found in multiple fields")
		return
	}

	errMsg := err.Error()

	// Should include details about checking multiple fields
	requiredStrings := []string{
		"Checked",
		"error message fields",
		"Field:",
		"error",
	}

	for _, required := range requiredStrings {
		if !strings.Contains(errMsg, required) {
			t.Errorf("Expected error message to contain '%s'\nGot: %s", required, errMsg)
		}
	}
}

// TestValidationError_Content_DetailedSuggestions verifies that detailed
// suggestions are provided based on the specific error context.
func TestValidationError_Content_DetailedSuggestions(t *testing.T) {
	tests := []struct {
		name            string
		validationType  string
		expected        interface{}
		actual          interface{}
		expectedSuggestions []string
	}{
		{
			name:           "404 specific suggestions",
			validationType: "status_code",
			expected:       200,
			actual:         404,
			expectedSuggestions: []string{
				"endpoint",
				"resource",
				"exists",
			},
		},
		{
			name:           "401 specific suggestions",
			validationType: "status_code",
			expected:       200,
			actual:         401,
			expectedSuggestions: []string{
				"authentication",
				"credentials",
				"token",
			},
		},
		{
			name:           "token error specific suggestions",
			validationType: "error_message",
			expected:       "token.*expired",
			actual:         "token invalid",
			expectedSuggestions: []string{
				"token",
				"expired",
				"refresh",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ve := FormatValidationError(tt.validationType, tt.expected, tt.actual, "", "")
			errMsg := ve.Error()

			// Verify suggestions are present
			for _, suggestion := range tt.expectedSuggestions {
				if !strings.Contains(errMsg, suggestion) {
					t.Errorf("Expected suggestions to contain '%s' for %s=%v, actual=%v\nGot: %s",
						suggestion, tt.validationType, tt.expected, tt.actual, errMsg)
				}
			}
		})
	}
}

// TestValidateStatusCodeWithDetails_EnhancedMismatchReporting verifies that enhanced
// mismatch reporting includes comprehensive information as required by the acceptance criteria.
func TestValidateStatusCodeWithDetails_EnhancedMismatchReporting(t *testing.T) {
	tests := []struct {
		name                string
		expected            interface{}
		actual              int
		requestContext      string
		mustContain         []string
		mustNotContain      []string
		verifyCategory      string
		verifyClientError   bool
		verifyServerError   bool
		minSuggestions      int
	}{
		{
			name:           "404 with context - shows all details",
			expected:       200,
			actual:         404,
			requestContext: "GET /api/users/123",
			mustContain: []string{
				"Request: GET /api/users/123",
				"Status code category: 4xx - Client error responses",
				"Expected: 200 (OK)",
				"Received: 404 (Not Found)",
				"Common causes:",
				"endpoint URL",
				"resource",
				"exists",
			},
			verifyCategory:    "client_error",
			verifyClientError: true,
			verifyServerError: false,
			minSuggestions:   4,
		},
		{
			name:           "401 without context - shows suggestions",
			expected:       200,
			actual:         401,
			requestContext: "",
			mustContain: []string{
				"Status code category: 4xx - Client error responses",
				"Expected: 200 (OK)",
				"Received: 401 (Unauthorized)",
				"Common causes:",
				"authentication",
				"credentials",
				"token",
				"Authorization header",
			},
			mustNotContain:   []string{"Request:"},
			verifyCategory:   "client_error",
			verifyClientError: true,
			verifyServerError: false,
			minSuggestions:   4,
		},
		{
			name:           "403 - permission suggestions",
			expected:       200,
			actual:         403,
			requestContext: "DELETE /api/admin/settings",
			mustContain: []string{
				"Request: DELETE /api/admin/settings",
				"Status code category: 4xx - Client error responses",
				"Received: 403 (Forbidden)",
				"Common causes:",
				"permission",
				"scopes",
				"OAuth",
			},
			verifyCategory:    "client_error",
			verifyClientError: true,
			verifyServerError: false,
			minSuggestions:   4,
		},
		{
			name:           "429 - rate limit suggestions",
			expected:       200,
			actual:         429,
			requestContext: "GET /api/data",
			mustContain: []string{
				"Received: 429 (Too Many Requests)",
				"Common causes:",
				"rate limiting",
				"quota",
				"backoff",
				"Retry-After",
				"X-RateLimit",
			},
			verifyCategory:    "client_error",
			verifyClientError: true,
			verifyServerError: false,
			minSuggestions:   4,
		},
		{
			name:           "500 - server error suggestions",
			expected:       200,
			actual:         500,
			requestContext: "POST /api/complex-operation",
			mustContain: []string{
				"Request: POST /api/complex-operation",
				"Status code category: 5xx - Server error responses",
				"Received: 500 (Internal Server Error)",
				"Common causes:",
				"retry",
				"backoff",
				"service status",
				"support",
			},
			verifyCategory:    "server_error",
			verifyClientError: false,
			verifyServerError: true,
			minSuggestions:   4,
		},
		{
			name:           "502 - bad gateway suggestions",
			expected:       200,
			actual:         502,
			requestContext: "",
			mustContain: []string{
				"Status code category: 5xx - Server error responses",
				"Received: 502 (Bad Gateway)",
				"Common causes:",
				"upstream",
				"retry",
				"load balancer",
			},
			verifyCategory:    "server_error",
			verifyClientError: false,
			verifyServerError: true,
			minSuggestions:   4,
		},
		{
			name:           "503 - service unavailable suggestions",
			expected:       200,
			actual:         503,
			requestContext: "GET /api/service",
			mustContain: []string{
				"Received: 503 (Service Unavailable)",
				"Common causes:",
				"retry",
				"maintenance",
				"capacity",
				"circuit breaker",
			},
			verifyCategory:    "server_error",
			verifyClientError: false,
			verifyServerError: true,
			minSuggestions:   4,
		},
		{
			name:           "504 - gateway timeout suggestions",
			expected:       200,
			actual:         504,
			requestContext: "",
			mustContain: []string{
				"Received: 504 (Gateway Timeout)",
				"Common causes:",
				"timeout",
				"upstream",
				"latency",
			},
			verifyCategory:    "server_error",
			verifyClientError: false,
			verifyServerError: true,
			minSuggestions:   4,
		},
		{
			name:           "400 - bad request suggestions",
			expected:       201,
			actual:         400,
			requestContext: "POST /api/users",
			mustContain: []string{
				"Received: 400 (Bad Request)",
				"Common causes:",
				"syntax",
				"formatting",
				"required fields",
				"Content-Type",
			},
			verifyCategory:    "client_error",
			verifyClientError: true,
			verifyServerError: false,
			minSuggestions:   4,
		},
		{
			name:           "Multiple expected codes - shows all options",
			expected:       []int{200, 201, 204},
			actual:         404,
			requestContext: "GET /api/items",
			mustContain: []string{
				"Status code category: 4xx - Client error responses",
				"Expected: one of [",
				"200 (OK)",
				"201 (Created)",
				"204 (No Content)",
				"]",
				"Received: 404 (Not Found)",
				"Common causes:",
			},
			verifyCategory:    "client_error",
			verifyClientError: true,
			verifyServerError: false,
			minSuggestions:   4,
		},
		{
			name:           "Unknown 4xx code - generic suggestions",
			expected:       200,
			actual:         418, // I'm a teapot
			requestContext: "",
			mustContain: []string{
				"Status code category: 4xx - Client error responses",
				"Common causes:",
				"request parameters",
				"headers",
				"API contracts",
			},
			verifyCategory:    "client_error",
			verifyClientError: true,
			verifyServerError: false,
			minSuggestions:   4,
		},
		{
			name:           "Unknown 5xx code - generic suggestions",
			expected:       200,
			actual:         599, // Non-standard
			requestContext: "",
			mustContain: []string{
				"Status code category: 5xx - Server error responses",
				"Common causes:",
				"retry",
				"service status",
				"support",
				"server logs",
			},
			verifyCategory:    "server_error",
			verifyClientError: false,
			verifyServerError: true,
			minSuggestions:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a response with the actual status code
			resp := httptest.NewRecorder()
			resp.Code = tt.actual
			httpResp := resp.Result()

			// Validate with details
			var result StatusCodeValidationResult
			if tt.requestContext != "" {
				result = ValidateStatusCodeWithDetails(httpResp, tt.expected, tt.requestContext)
			} else {
				result = ValidateStatusCodeWithDetails(httpResp, tt.expected)
			}

			// Verify the result is invalid
			if result.Valid {
				t.Errorf("Expected validation to fail for status %d vs expected %v", tt.actual, tt.expected)
			}

			// Verify mismatch details are populated
			mismatchMsg := result.MismatchDetails
			if mismatchMsg == "" {
				t.Errorf("Expected mismatch details to be populated, got empty string")
			}

			// Check for required content
			for _, required := range tt.mustContain {
				if !strings.Contains(mismatchMsg, required) {
					t.Errorf("Expected mismatch details to contain '%s'\nGot: %s", required, mismatchMsg)
				}
			}

			// Check that forbidden content is not present
			for _, forbidden := range tt.mustNotContain {
				if strings.Contains(mismatchMsg, forbidden) {
					t.Errorf("Expected mismatch details to NOT contain '%s'\nGot: %s", forbidden, mismatchMsg)
				}
			}

			// Verify category
			if result.Category != tt.verifyCategory {
				t.Errorf("Expected Category to be '%s', got '%s'", tt.verifyCategory, result.Category)
			}

			// Verify error type flags
			if result.IsClientError != tt.verifyClientError {
				t.Errorf("Expected IsClientError to be %v, got %v", tt.verifyClientError, result.IsClientError)
			}

			if result.IsServerError != tt.verifyServerError {
				t.Errorf("Expected IsServerError to be %v, got %v", tt.verifyServerError, result.IsServerError)
			}

			// Verify minimum number of suggestions (expect "Common causes:" + numbered items)
			commonCausesCount := strings.Count(mismatchMsg, "Common causes:")
			if commonCausesCount != 1 {
				t.Errorf("Expected 'Common causes:' to appear exactly once, appeared %d times", commonCausesCount)
			}

			// Count numbered suggestion items (e.g., "  1. ", "  2. ", etc.)
			suggestionCount := 0
			for i := 1; i <= 10; i++ {
				if strings.Contains(mismatchMsg, fmt.Sprintf("  %d. ", i)) {
					suggestionCount++
				}
			}
			if suggestionCount < tt.minSuggestions {
				t.Errorf("Expected at least %d numbered suggestions, found %d\nGot: %s", tt.minSuggestions, suggestionCount, mismatchMsg)
			}
		})
	}
}

// TestValidateStatusCodeWithDetails_SpecificCodeSuggestions verifies that specific
// status codes get the correct suggestions as per acceptance criteria.
func TestValidateStatusCodeWithDetails_SpecificCodeSuggestions(t *testing.T) {
	tests := []struct {
		name              string
		actualCode        int
		expectedKeywords  []string
		notExpectedKeywords []string
	}{
		{
			name:       "400 - bad request",
			actualCode: 400,
			expectedKeywords: []string{
				"syntax",
				"formatting",
				"required fields",
				"Content-Type",
				"JSON schema",
			},
		},
		{
			name:       "401 - unauthorized",
			actualCode: 401,
			expectedKeywords: []string{
				"authentication",
				"credentials",
				"expired",
				"Authorization header",
				"Bearer",
			},
		},
		{
			name:       "403 - forbidden",
			actualCode: 403,
			expectedKeywords: []string{
				"permission",
				"scopes",
				"roles",
				"OAuth",
			},
		},
		{
			name:       "404 - not found",
			actualCode: 404,
			expectedKeywords: []string{
				"endpoint URL",
				"resource",
				"exists",
				"deleted or moved",
				"API versioning",
			},
		},
		{
			name:       "429 - too many requests",
			actualCode: 429,
			expectedKeywords: []string{
				"rate limiting",
				"backoff",
				"quota",
				"Retry-After",
				"X-RateLimit-Remaining",
			},
		},
		{
			name:       "500 - internal server error",
			actualCode: 500,
			expectedKeywords: []string{
				"retry",
				"backoff",
				"service status",
				"support",
				"server-side bugs",
			},
		},
		{
			name:       "502 - bad gateway",
			actualCode: 502,
			expectedKeywords: []string{
				"upstream",
				"retry",
				"load balancer",
				"network connectivity",
			},
		},
		{
			name:       "503 - service unavailable",
			actualCode: 503,
			expectedKeywords: []string{
				"retry",
				"maintenance",
				"capacity",
				"scaling",
				"circuit breaker",
			},
		},
		{
			name:       "504 - gateway timeout",
			actualCode: 504,
			expectedKeywords: []string{
				"timeout",
				"retry",
				"upstream",
				"request complexity",
				"network latency",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a response with the actual status code
			resp := httptest.NewRecorder()
			resp.Code = tt.actualCode
			httpResp := resp.Result()

			// Validate with expected 200
			result := ValidateStatusCodeWithDetails(httpResp, 200, "TEST REQUEST")

			// Verify result is invalid
			if result.Valid {
				t.Errorf("Expected validation to fail for status %d", tt.actualCode)
			}

			mismatchMsg := result.MismatchDetails

			// Check for expected keywords
			for _, keyword := range tt.expectedKeywords {
				if !strings.Contains(mismatchMsg, keyword) {
					t.Errorf("Expected suggestions for status %d to contain '%s'\nGot: %s", tt.actualCode, keyword, mismatchMsg)
				}
			}

			// Check that unexpected keywords are NOT present
			for _, keyword := range tt.notExpectedKeywords {
				if strings.Contains(mismatchMsg, keyword) {
					t.Errorf("Expected suggestions for status %d to NOT contain '%s'\nGot: %s", tt.actualCode, keyword, mismatchMsg)
				}
			}
		})
	}
}

// TestValidateStatusCodeWithDetails_CategoryDescriptions verifies that all status
// code categories have proper descriptions as per acceptance criteria.
func TestValidateStatusCodeWithDetails_CategoryDescriptions(t *testing.T) {
	tests := []struct {
		name         string
		actualCode   int
		expectedDesc string
	}{
		{
			name:         "200 - success",
			actualCode:   200,
			expectedDesc: "2xx - Successful responses",
		},
		{
			name:         "301 - redirection",
			actualCode:   301,
			expectedDesc: "3xx - Redirection messages",
		},
		{
			name:         "404 - client error",
			actualCode:   404,
			expectedDesc: "4xx - Client error responses",
		},
		{
			name:         "500 - server error",
			actualCode:   500,
			expectedDesc: "5xx - Server error responses",
		},
		{
			name:         "100 - other",
			actualCode:   100,
			expectedDesc: "Non-standard status codes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a response
			resp := httptest.NewRecorder()
			resp.Code = tt.actualCode
			httpResp := resp.Result()

			// Validate
			result := ValidateStatusCodeWithDetails(httpResp, 999)

			// Check mismatch details for category description
			if !strings.Contains(result.MismatchDetails, tt.expectedDesc) {
				t.Errorf("Expected mismatch details to contain category '%s'\nGot: %s", tt.expectedDesc, result.MismatchDetails)
			}
		})
	}
}

// TestValidationError_Content_Sanitization verifies that response snippets
// are properly sanitized and truncated in error messages.
func TestValidationError_Content_Sanitization(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		snippetContains string
		snippetMaxLength int
	}{
		{
			name:            "long response truncated with ellipsis",
			response:        `{"error": "` + strings.Repeat("very long message ", 50) + `"}`,
			snippetContains: "...",
			snippetMaxLength: 200,
		},
		{
			name:            "newlines are replaced with spaces",
			response:        "{\n  \"error\": \"test\"\n}\n",
			snippetContains: "{",
			snippetMaxLength: 200,
		},
		{
			name:            "carriage returns are replaced",
			response:        "{\r  \"error\": \"test\"\r}\r",
			snippetContains: "{",
			snippetMaxLength: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), "xyz")
			if err == nil {
				t.Fatal("Expected error for response")
			}

			errMsg := err.Error()

			// Verify snippet sanitization
			if tt.snippetContains != "" && !strings.Contains(errMsg, tt.snippetContains) {
				t.Errorf("Expected sanitized snippet to contain '%s'\nGot: %s", tt.snippetContains, errMsg)
			}

			// Verify no raw newlines in response section
			if strings.Contains(errMsg, "\n") {
				lines := strings.Split(errMsg, "\n")
				for i, line := range lines {
					if strings.Contains(line, "Response:") && strings.Contains(line, "\n") {
						t.Errorf("Response section should not contain raw newlines at line %d: %s", i, line)
					}
				}
			}
		})
	}
}
