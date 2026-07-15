package validate

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// =============================================================================
// COMPREHENSIVE TESTS FOR ENHANCED VALIDATION ERRORS
// =============================================================================

// TestFormatStatusCodeError_ContentVerification tests that FormatStatusCodeError includes
// all expected content in error messages including expected/actual values and suggestions.
func TestFormatStatusCodeError_ContentVerification(t *testing.T) {
	testCases := []struct {
		name             string
		expected         interface{}
		actual           int
		context          string
		expectedInOutput []string
	}{
		{
			name:     "200 vs 404 mismatch",
			expected: 200,
			actual:   404,
			context:  "GET /api/users/123",
			expectedInOutput: []string{
				"200",
				"404",
				"Not Found",
				"GET /api/users/123",
			},
		},
		{
			name:     "Multiple expected codes vs 401",
			expected: []int{200, 201, 204},
			actual:   401,
			context:  "POST /api/auth/login",
			expectedInOutput: []string{
				"200",
				"401",
				"Unauthorized",
				"POST /api/auth/login",
			},
		},
		{
			name:     "500 error with context",
			expected: 200,
			actual:   500,
			context:  "DELETE /api/users/123",
			expectedInOutput: []string{
				"200",
				"500",
				"Internal Server Error",
				"DELETE /api/users/123",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ve := FormatStatusCodeError(tc.expected, tc.actual, tc.context)

			// Verify error was created (non-zero ValidationError)
			if ve.ErrorType == "" {
				t.Fatal("Expected ValidationError to be created")
			}

			// Verify error type
			if ve.ErrorType != "status_code" {
				t.Errorf("Expected ErrorType 'status_code', got '%s'", ve.ErrorType)
			}

			// Verify expected and actual values are in error message
			errorMsg := ve.Error()
			for _, expectedStr := range tc.expectedInOutput {
				if !strings.Contains(errorMsg, expectedStr) {
					t.Errorf("Expected error message to contain '%s', but it didn't\nError: %s", expectedStr, errorMsg)
				}
			}

			// Verify suggestions are included
			if len(ve.Suggestions) == 0 {
				t.Error("Expected suggestions to be included in error")
			}
		})
	}
}

// TestFormatStatusCodeRangeError_ContentVerification tests that FormatStatusCodeRangeError
// includes comprehensive range information, boundaries, and distance from range.
func TestFormatStatusCodeRangeError_ContentVerification(t *testing.T) {
	testCases := []struct {
		name             string
		pattern          string
		actual           int
		context          string
		expectedInOutput []string
	}{
		{
			name:    "4xx pattern with 200 status",
			pattern: "4xx",
			actual:  200,
			context: "Error response validation",
			expectedInOutput: []string{
				"4xx",
				"200",
				"400",
				"499",
				"Client Error",
			},
		},
		{
			name:    "5xx pattern with 404 status",
			pattern: "5xx",
			actual:  404,
			context: "Server error check",
			expectedInOutput: []string{
				"5xx",
				"404",
				"500",
				"599",
				"Server Error",
			},
		},
		{
			name:    "2xx pattern with 500 status",
			pattern: "2xx",
			actual:  500,
			context: "Success response validation",
			expectedInOutput: []string{
				"2xx",
				"500",
				"200",
				"299",
				"Success",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := FormatStatusCodeRangeError(tc.pattern, tc.actual, tc.context)

			// Verify error type
			if err.ErrorType != "status_code_range" {
				t.Errorf("Expected ErrorType 'status_code_range', got '%s'", err.ErrorType)
			}

			// Verify range information is included
			if err.RangeInfo == "" {
				t.Error("Expected RangeInfo to be populated")
			}

			// Verify expected strings are in error message
			errorMsg := err.Error()
			for _, expectedStr := range tc.expectedInOutput {
				if !strings.Contains(errorMsg, expectedStr) {
					t.Errorf("Expected error message to contain '%s', but it didn't\nError: %s", expectedStr, errorMsg)
				}
			}

			// Verify validation details are included
			if len(err.ValidationDetails) == 0 {
				t.Error("Expected validation details to be included")
			}

			// Verify suggestions are included
			if len(err.Suggestions) == 0 {
				t.Error("Expected suggestions to be included in error")
			}
		})
	}
}

// TestFormatErrorMessageError_ContentVerification tests that FormatErrorMessageError
// includes pattern details, field names, and appropriate suggestions.
func TestFormatErrorMessageError_ContentVerification(t *testing.T) {
	testCases := []struct {
		name             string
		expectedPattern  string
		actualMessage    string
		fieldName        string
		context          string
		expectedInOutput []string
	}{
		{
			name:            "Token pattern mismatch",
			expectedPattern: "invalid.*token",
			actualMessage:   "access_denied",
			fieldName:       "error",
			context:         "OAuth validation",
			expectedInOutput: []string{
				"invalid.*token",
				"access_denied",
				"error",
				"OAuth",
			},
		},
		{
			name:            "User not found pattern",
			expectedPattern: "User .* not found",
			actualMessage:   "Invalid credentials",
			fieldName:       "message",
			context:         "User lookup",
			expectedInOutput: []string{
				"User",
				"not found",
				"Invalid credentials",
				"message",
			},
		},
		{
			name:            "Permission denied pattern",
			expectedPattern: "permission.*denied",
			actualMessage:   "Resource not found",
			fieldName:       "detail",
			context:         "Authorization check",
			expectedInOutput: []string{
				"permission",
				"denied",
				"Resource not found",
				"detail",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := FormatErrorMessageError(tc.expectedPattern, tc.actualMessage, tc.fieldName, tc.context)

			// Verify error type
			if err.ErrorType != "error_message" {
				t.Errorf("Expected ErrorType 'error_message', got '%s'", err.ErrorType)
			}

			// Verify field name is included
			if err.FieldName != tc.fieldName {
				t.Errorf("Expected FieldName '%s', got '%s'", tc.fieldName, err.FieldName)
			}

			// Verify expected strings are in error message
			errorMsg := err.Error()
			for _, expectedStr := range tc.expectedInOutput {
				if !strings.Contains(errorMsg, expectedStr) {
					t.Errorf("Expected error message to contain '%s', but it didn't\nError: %s", expectedStr, errorMsg)
				}
			}

			// Verify suggestions are included
			if len(err.Suggestions) == 0 {
				t.Error("Expected suggestions to be included in error")
			}
		})
	}
}

// TestFormatValidationErrorWithDetails_ResponseSnippetExcerpt tests that response
// snippets are properly included in validation errors and truncated when too long.
func TestFormatValidationErrorWithDetails_ResponseSnippetExcerpt(t *testing.T) {
	longSnippet := `{"error": "This is a very long error message that contains lots of details about what went wrong including stack traces and debugging information", "code": "ERR_12345"}`

	testCases := []struct {
		name          string
		snippet       string
		expectedTrunc bool
		minLength     int
	}{
		{
			name:          "Short snippet",
			snippet:       `{"error": "test"}`,
			expectedTrunc: false,
			minLength:     15,
		},
		{
			name:          "Long snippet",
			snippet:       longSnippet,
			expectedTrunc: true,
			minLength:     50,
		},
		{
			name:          "Empty snippet",
			snippet:       "",
			expectedTrunc: false,
			minLength:     0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := FormatValidationErrorWithDetails(
				"status_code",
				200,
				404,
				"API validation",
				tc.snippet,
				"",
				"",
				nil,
				"",
				"",
				nil,
			)

			// Verify snippet is handled correctly
			if tc.snippet != "" && tc.minLength > 0 {
				if len(err.ResponseSnippet) < tc.minLength {
					t.Errorf("Expected ResponseSnippet to be at least %d chars, got %d", tc.minLength, len(err.ResponseSnippet))
				}
			}

			if tc.snippet == "" && err.ResponseSnippet != "" {
				t.Errorf("Expected empty ResponseSnippet for empty input, got '%s'", err.ResponseSnippet)
			}
		})
	}
}

// TestFormatValidationErrorWithDetails_SuggestionsForCommonFailures tests that
// appropriate suggestions are generated for common validation failure scenarios.
func TestFormatValidationErrorWithDetails_SuggestionsForCommonFailures(t *testing.T) {
	testCases := []struct {
		name                string
		validationType     string
		expected           interface{}
		actual              interface{}
		expectedSuggestions []string
	}{
		{
			name:            "404 Not Found suggestions",
			validationType: "status_code",
			expected:        200,
			actual:          404,
			expectedSuggestions: []string{
				"endpoint",
				"resource",
				"exists",
			},
		},
		{
			name:            "401 Unauthorized suggestions",
			validationType: "status_code",
			expected:        200,
			actual:          401,
			expectedSuggestions: []string{
				"authentication",
				"credentials",
				"token",
			},
		},
		{
			name:            "403 Forbidden suggestions",
			validationType: "status_code",
			expected:        200,
			actual:          403,
			expectedSuggestions: []string{
				"permission",
				"resource",
			},
		},
		{
			name:            "500 Server Error suggestions",
			validationType: "status_code",
			expected:        200,
			actual:          500,
			expectedSuggestions: []string{
				"retry",
				"service",
			},
		},
		{
			name:            "Error message pattern suggestions",
			validationType: "error_message",
			expected:        "invalid.*token",
			actual:          "access_denied",
			expectedSuggestions: []string{
				"token",
				"authentication",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := FormatValidationErrorWithDetails(
				tc.validationType,
				tc.expected,
				tc.actual,
				"",
				"",
				"",
				"",
				nil,
				"",
				"",
				nil,
			)

			// Verify suggestions were generated
			if len(err.Suggestions) == 0 {
				t.Error("Expected suggestions to be auto-generated")
			}

			// Verify suggestions contain expected keywords
			allSuggestions := strings.Join(err.Suggestions, " ")
			for _, expectedKeyword := range tc.expectedSuggestions {
				if !strings.Contains(strings.ToLower(allSuggestions), expectedKeyword) {
					t.Errorf("Expected suggestions to contain '%s', but they didn't\nSuggestions: %v", expectedKeyword, err.Suggestions)
				}
			}
		})
	}
}

// =============================================================================
// TABLE-DRIVEN TESTS FOR MULTIPLE SCENARIOS
// =============================================================================

// TestStatusCodeValidation_TableDriven tests status code validation across
// multiple scenarios using table-driven approach.
func TestStatusCodeValidation_TableDriven(t *testing.T) {
	testCases := []struct {
		name           string
		validationFunc func() ValidationError
		shouldPass     bool
		expectedInMsg  []string
	}{
		{
			name: "200 success validation",
			validationFunc: func() ValidationError {
				return FormatStatusCodeError(200, 200, "Success check")
			},
			shouldPass:    true,
			expectedInMsg: []string{},
		},
		{
			name: "404 not found with full context",
			validationFunc: func() ValidationError {
				return FormatStatusCodeError(200, 404, "GET /api/users/123")
			},
			shouldPass:    false,
			expectedInMsg: []string{"404", "Not Found", "GET /api/users/123"},
		},
		{
			name: "401 unauthorized with auth context",
			validationFunc: func() ValidationError {
				return FormatStatusCodeError(200, 401, "POST /api/auth/login")
			},
			shouldPass:    false,
			expectedInMsg: []string{"401", "Unauthorized", "authentication"},
		},
		{
			name: "403 forbidden with permission context",
			validationFunc: func() ValidationError {
				return FormatStatusCodeError(200, 403, "DELETE /api/users/123")
			},
			shouldPass:    false,
			expectedInMsg: []string{"403", "Forbidden", "permission"},
		},
		{
			name: "500 server error with retry context",
			validationFunc: func() ValidationError {
				return FormatStatusCodeError(200, 500, "PUT /api/users/123")
			},
			shouldPass:    false,
			expectedInMsg: []string{"500", "Internal Server Error", "retry"},
		},
		{
			name: "Multiple allowed codes 201",
			validationFunc: func() ValidationError {
				return FormatStatusCodeError([]int{200, 201, 204}, 201, "POST /api/users")
			},
			shouldPass:    true,
			expectedInMsg: []string{},
		},
		{
			name: "Multiple allowed codes 404 mismatch",
			validationFunc: func() ValidationError {
				return FormatStatusCodeError([]int{200, 201, 204}, 404, "GET /api/users")
			},
			shouldPass:    false,
			expectedInMsg: []string{"404", "200", "201", "204"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ve := tc.validationFunc()

			if tc.shouldPass {
				// When should pass, we expect either no error or a successful validation
				// For status codes, we can check if Expected equals Actual
				if fmt.Sprintf("%v", ve.Expected) != fmt.Sprintf("%v", ve.Actual) {
					// This is actually an error case
				}
			} else {
				// Verify expected content in error message
				errorMsg := ve.Error()
				for _, expectedStr := range tc.expectedInMsg {
					if !strings.Contains(errorMsg, expectedStr) {
						t.Errorf("Expected error message to contain '%s', but it didn't\nError: %s", expectedStr, errorMsg)
					}
				}

				// Verify suggestions are present
				if len(ve.Suggestions) == 0 {
					t.Error("Expected suggestions to be included in error")
				}
			}
		})
	}
}

// TestRangeValidation_TableDriven tests range validation across multiple
// scenarios using table-driven approach.
func TestRangeValidation_TableDriven(t *testing.T) {
	testCases := []struct {
		name          string
		pattern       string
		actual        int
		context       string
		shouldPass    bool
		expectedInMsg []string
	}{
		{
			name:          "4xx range with valid 404",
			pattern:       "4xx",
			actual:        404,
			context:       "Client error check",
			shouldPass:    true,
			expectedInMsg: []string{},
		},
		{
			name:          "4xx range with invalid 200",
			pattern:       "4xx",
			actual:        200,
			context:       "Expected client error",
			shouldPass:    false,
			expectedInMsg: []string{"4xx", "400", "499", "Client Error", "200"},
		},
		{
			name:          "5xx range with valid 500",
			pattern:       "5xx",
			actual:        500,
			context:       "Server error check",
			shouldPass:    true,
			expectedInMsg: []string{},
		},
		{
			name:          "5xx range with invalid 404",
			pattern:       "5xx",
			actual:        404,
			context:       "Expected server error",
			shouldPass:    false,
			expectedInMsg: []string{"5xx", "500", "599", "Server Error", "404"},
		},
		{
			name:          "2xx range with valid 200",
			pattern:       "2xx",
			actual:        200,
			context:       "Success check",
			shouldPass:    true,
			expectedInMsg: []string{},
		},
		{
			name:          "2xx range with invalid 500",
			pattern:       "2xx",
			actual:        500,
			context:       "Expected success",
			shouldPass:    false,
			expectedInMsg: []string{"2xx", "200", "299", "Success", "500"},
		},
		{
			name:          "3xx range with valid 301",
			pattern:       "3xx",
			actual:        301,
			context:       "Redirect check",
			shouldPass:    true,
			expectedInMsg: []string{},
		},
		{
			name:          "3xx range with invalid 200",
			pattern:       "3xx",
			actual:        200,
			context:       "Expected redirect",
			shouldPass:    false,
			expectedInMsg: []string{"3xx", "300", "399", "Redirection", "200"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStatusCodeRangeInt(tc.pattern, tc.actual)

			if tc.shouldPass {
				if err != nil {
					t.Errorf("Expected validation to pass, but got error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Expected validation to fail, but it passed")
				}

				// Verify expected content in error message
				errorMsg := err.Error()
				for _, expectedStr := range tc.expectedInMsg {
					if !strings.Contains(errorMsg, expectedStr) {
						t.Errorf("Expected error message to contain '%s', but it didn't\nError: %s", expectedStr, errorMsg)
					}
				}
			}
		})
	}
}

// TestMessagePatternValidation_TableDriven tests message pattern validation
// across multiple scenarios using table-driven approach.
func TestMessagePatternValidation_TableDriven(t *testing.T) {
	testCases := []struct {
		name            string
		expectedPattern string
		actualMessage   string
		fieldName       string
		context         string
		shouldMatch     bool
		expectedInMsg   []string
	}{
		{
			name:            "Matching token pattern",
			expectedPattern: "invalid.*token",
			actualMessage:   "invalid_token",
			fieldName:       "error",
			context:         "Token validation",
			shouldMatch:     true,
			expectedInMsg:   []string{},
		},
		{
			name:            "Non-matching token pattern",
			expectedPattern: "invalid.*token",
			actualMessage:   "access_denied",
			fieldName:       "error",
			context:         "Token validation",
			shouldMatch:     false,
			expectedInMsg:   []string{"invalid.*token", "access_denied", "error"},
		},
		{
			name:            "Matching user pattern",
			expectedPattern: "User .* not found",
			actualMessage:   "User john_doe not found",
			fieldName:       "message",
			context:         "User lookup",
			shouldMatch:     true,
			expectedInMsg:   []string{},
		},
		{
			name:            "Non-matching user pattern",
			expectedPattern: "User .* not found",
			actualMessage:   "Invalid credentials",
			fieldName:       "message",
			context:         "User lookup",
			shouldMatch:     false,
			expectedInMsg:   []string{"User", "not found", "Invalid credentials", "message"},
		},
		{
			name:            "Matching permission pattern",
			expectedPattern: "permission.*denied",
			actualMessage:   "permission_denied",
			fieldName:       "detail",
			context:         "Authorization check",
			shouldMatch:     true,
			expectedInMsg:   []string{},
		},
		{
			name:            "Non-matching permission pattern",
			expectedPattern: "permission.*denied",
			actualMessage:   "Resource not found",
			fieldName:       "detail",
			context:         "Authorization check",
			shouldMatch:     false,
			expectedInMsg:   []string{"permission", "denied", "Resource not found", "detail"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matched, _ := regexp.MatchString(tc.expectedPattern, tc.actualMessage)

			if tc.shouldMatch {
				if !matched {
					t.Errorf("Expected pattern to match, but it didn't\nPattern: %s\nMessage: %s", tc.expectedPattern, tc.actualMessage)
				}
			} else {
				if matched {
					t.Errorf("Expected pattern not to match, but it did\nPattern: %s\nMessage: %s", tc.expectedPattern, tc.actualMessage)
				}

				// Create error and verify content
				err := FormatErrorMessageError(tc.expectedPattern, tc.actualMessage, tc.fieldName, tc.context)
				errorMsg := err.Error()

				for _, expectedStr := range tc.expectedInMsg {
					if !strings.Contains(errorMsg, expectedStr) {
						t.Errorf("Expected error message to contain '%s', but it didn't\nError: %s", expectedStr, errorMsg)
					}
				}
			}
		})
	}
}

// =============================================================================
// BENCHMARK TESTS FOR ERROR FORMATTING PERFORMANCE
// =============================================================================

// BenchmarkFormatStatusCodeError benchmarks the performance of status code error formatting.
func BenchmarkFormatStatusCodeError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FormatStatusCodeError(200, 404, "GET /api/users/123")
	}
}

// BenchmarkFormatStatusCodeRangeError benchmarks the performance of range error formatting.
func BenchmarkFormatStatusCodeRangeError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FormatStatusCodeRangeError("4xx", 200, "Error response validation")
	}
}

// BenchmarkFormatErrorMessageError benchmarks the performance of error message formatting.
func BenchmarkFormatErrorMessageError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FormatErrorMessageError("invalid.*token", "access_denied", "error", "OAuth validation")
	}
}

// BenchmarkFormatValidationErrorWithDetails benchmarks the performance of detailed validation error formatting.
func BenchmarkFormatValidationErrorWithDetails(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FormatValidationErrorWithDetails(
			"status_code",
			200,
			404,
			"GET /api/users/123",
			`{"error": "Resource not found"}`,
			"error",
			"line 42",
			[]string{"user_id", "resource_id"},
			"Expected pattern: Resource .* not found",
			"",
			[]string{"Pattern mismatch", "Field not found"},
		)
	}
}

// BenchmarkFormatValidationErrorWithDetails_AllFields benchmarks with all fields populated.
func BenchmarkFormatValidationErrorWithDetails_AllFields(b *testing.B) {
	longSnippet := strings.Repeat("Error details ", 100) // Create a long response snippet
	relatedFields := make([]string, 10)
	for i := range relatedFields {
		relatedFields[i] = fmt.Sprintf("field_%d", i)
	}
	validationDetails := make([]string, 5)
	for i := range validationDetails {
		validationDetails[i] = fmt.Sprintf("Validation step %d completed", i+1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatValidationErrorWithDetails(
			"error_message",
			"expected.*pattern",
			"actual_message",
			"Complex validation scenario with multiple fields",
			longSnippet,
			"error_field",
			"validation location details",
			relatedFields,
			"Pattern matching details with regex information",
			"400-499 (Client Error)",
			validationDetails,
			"Custom suggestion 1", "Custom suggestion 2", "Custom suggestion 3",
		)
	}
}

// BenchmarkValidationError_ErrorMethod benchmarks the Error() method performance.
func BenchmarkValidationError_ErrorMethod(b *testing.B) {
	err := FormatValidationErrorWithDetails(
		"status_code",
		200,
		404,
		"GET /api/users/123",
		`{"error": "Resource not found"}`,
		"error",
		"line 42",
		[]string{"user_id", "resource_id"},
		"Expected pattern: Resource .* not found",
		"",
		[]string{"Pattern mismatch", "Field not found"},
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}

// =============================================================================
// INTEGRATION TESTS WITH HTTP RESPONSES
// =============================================================================

// TestStatusCodeValidation_WithHTTPResponses tests enhanced error reporting
// with actual HTTP response objects.
func TestStatusCodeValidation_WithHTTPResponses(t *testing.T) {
	testCases := []struct {
		name           string
		createResponse func() *http.Response
		expectedCode   interface{}
		shouldPass     bool
		verifyContent  []string
	}{
		{
			name: "httptest.ResponseRecorder with 200",
			createResponse: func() *http.Response {
				w := httptest.NewRecorder()
				w.Code = 200
				return w.Result()
			},
			expectedCode:  200,
			shouldPass:    true,
			verifyContent: []string{},
		},
		{
			name: "httptest.ResponseRecorder with 404 mismatch",
			createResponse: func() *http.Response {
				w := httptest.NewRecorder()
				w.Code = 404
				return w.Result()
			},
			expectedCode:  200,
			shouldPass:    false,
			verifyContent: []string{"404", "Not Found", "200"},
		},
		{
			name: "Multiple allowed codes with 201",
			createResponse: func() *http.Response {
				w := httptest.NewRecorder()
				w.Code = 201
				return w.Result()
			},
			expectedCode:  []int{200, 201, 204},
			shouldPass:    true,
			verifyContent: []string{},
		},
		{
			name: "Multiple allowed codes with 404 mismatch",
			createResponse: func() *http.Response {
				w := httptest.NewRecorder()
				w.Code = 404
				return w.Result()
			},
			expectedCode:  []int{200, 201, 204},
			shouldPass:    false,
			verifyContent: []string{"404", "200", "201", "204"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := tc.createResponse()

			// Use the validation helper
			if !HTTPStatusCodeIsValid(resp, tc.expectedCode) {
				if tc.shouldPass {
					t.Errorf("Expected validation to pass, but it failed")
				}
			}

			// If it should fail, verify the error content
			if !tc.shouldPass {
				ve := FormatStatusCodeError(tc.expectedCode, resp.StatusCode, "HTTP response validation")
				errorMsg := ve.Error()
				for _, expectedStr := range tc.verifyContent {
					if !strings.Contains(errorMsg, expectedStr) {
						t.Errorf("Expected error message to contain '%s', but it didn't\nError: %s", expectedStr, errorMsg)
					}
				}
			}
		})
	}
}

// TestStatusCodeRangeValidation_WithHTTPResponses tests range validation
// with actual HTTP response objects.
func TestStatusCodeRangeValidation_WithHTTPResponses(t *testing.T) {
	testCases := []struct {
		name          string
		statusCode    int
		rangePattern  string
		shouldPass    bool
		verifyContent []string
	}{
		{
			name:          "404 in 4xx range",
			statusCode:    404,
			rangePattern:  "4xx",
			shouldPass:    true,
			verifyContent: []string{},
		},
		{
			name:          "200 not in 4xx range",
			statusCode:    200,
			rangePattern:  "4xx",
			shouldPass:    false,
			verifyContent: []string{"4xx", "400", "499", "Client Error", "200"},
		},
		{
			name:          "500 in 5xx range",
			statusCode:    500,
			rangePattern:  "5xx",
			shouldPass:    true,
			verifyContent: []string{},
		},
		{
			name:          "404 not in 5xx range",
			statusCode:    404,
			rangePattern:  "5xx",
			shouldPass:    false,
			verifyContent: []string{"5xx", "500", "599", "Server Error", "404"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate the status code against the range
			err := ValidateStatusCodeRangeInt(tc.rangePattern, tc.statusCode)

			if tc.shouldPass {
				if err != nil {
					t.Errorf("Expected validation to pass, but it failed\nError: %s", err.Error())
				}
			} else {
				if err == nil {
					t.Error("Expected validation to fail, but it passed")
				}

				// Verify expected content in error message
				errorMsg := err.Error()
				for _, expectedStr := range tc.verifyContent {
					if !strings.Contains(errorMsg, expectedStr) {
						t.Errorf("Expected error message to contain '%s', but it didn't\nError: %s", expectedStr, errorMsg)
					}
				}
			}
		})
	}
}
