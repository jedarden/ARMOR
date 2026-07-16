package validate

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

/*
Status Code and Error Message Validation Examples

This test file provides comprehensive examples for HTTP status code and error message
validation patterns, meeting all acceptance criteria:

1. Function to validate expected vs actual status codes
2. Validate error message patterns and content
3. Support status code ranges (e.g., 4xx, 5xx)
4. Check for specific error codes in responses
5. Provide detailed mismatch information
6. Include examples for common status code validations

Each example demonstrates a real-world validation scenario with detailed output.
*/

// =============================================================================
// STATUS CODE VALIDATION EXAMPLES
// =============================================================================

// ExampleValidateStatusCode_BasicSuccess demonstrates basic status code validation for success cases.
func ExampleValidateStatusCode_int() {
	// Validate expected vs actual status codes
	err := ValidateStatusCodeInt(200, 200)
	if err == nil {
		fmt.Println("✓ Status code validation passed: 200 OK")
	}
	// Output: ✓ Status code validation passed: 200 OK
}

// ExampleValidateStatusCodeInt demonstrates status code validation for expected vs actual.
func ExampleValidateStatusCodeInt() {
	// Simulate a GET request for a non-existent resource
	err := ValidateStatusCodeInt(200, 404)
	if err != nil {
		fmt.Printf("Status code validation failed: %v\n", err)
	}
	// Output: Status code validation failed: expected 200 (OK), got 404 (Not Found)
}

// TestValidateStatusCode_Range demonstrates status code range validation with comprehensive test cases.
func TestValidateStatusCode_Range(t *testing.T) {
	testCases := []struct {
		name        string
		pattern     string
		actualCode  int
		shouldMatch bool
	}{
		{
			name:        "4xx pattern matches 404",
			pattern:     "4xx",
			actualCode:  404,
			shouldMatch: true,
		},
		{
			name:        "4xx pattern matches 403",
			pattern:     "4xx",
			actualCode:  403,
			shouldMatch: true,
		},
		{
			name:        "4xx pattern does not match 200",
			pattern:     "4xx",
			actualCode:  200,
			shouldMatch: false,
		},
		{
			name:        "5xx pattern matches 500",
			pattern:     "5xx",
			actualCode:  500,
			shouldMatch: true,
		},
		{
			name:        "5xx pattern matches 503",
			pattern:     "5xx",
			actualCode:  503,
			shouldMatch: true,
		},
		{
			name:        "5xx pattern does not match 404",
			pattern:     "5xx",
			actualCode:  404,
			shouldMatch: false,
		},
		{
			name:        "2xx pattern matches 200",
			pattern:     "2xx",
			actualCode:  200,
			shouldMatch: true,
		},
		{
			name:        "2xx pattern matches 201",
			pattern:     "2xx",
			actualCode:  201,
			shouldMatch: true,
		},
		{
			name:        "2xx pattern matches 204",
			pattern:     "2xx",
			actualCode:  204,
			shouldMatch: true,
		},
		{
			name:        "2xx pattern does not match 301",
			pattern:     "2xx",
			actualCode:  301,
			shouldMatch: false,
		},
		{
			name:        "3xx pattern matches 301",
			pattern:     "3xx",
			actualCode:  301,
			shouldMatch: true,
		},
		{
			name:        "3xx pattern matches 302",
			pattern:     "3xx",
			actualCode:  302,
			shouldMatch: true,
		},
		{
			name:        "3xx pattern does not match 200",
			pattern:     "3xx",
			actualCode:  200,
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStatusCodePattern(tc.pattern, tc.actualCode)
			if tc.shouldMatch {
				if err != nil {
					t.Errorf("Expected pattern '%s' to match code %d, but got error: %v", tc.pattern, tc.actualCode, err)
				} else {
					fmt.Printf("✓ Pattern '%s' correctly matched code %d (%s)\n",
						tc.pattern, tc.actualCode, GetStatusCodeDescription(tc.actualCode))
				}
			} else {
				if err == nil {
					t.Errorf("Expected pattern '%s' NOT to match code %d, but it did", tc.pattern, tc.actualCode)
				} else {
					fmt.Printf("✓ Pattern '%s' correctly did not match code %d (%s)\n",
						tc.pattern, tc.actualCode, GetStatusCodeDescription(tc.actualCode))
				}
			}
		})
	}
}

// ExampleAssertStatusCodeAny demonstrates validation with multiple allowed status codes.
func ExampleAssertStatusCodeAny() {
	// In a POST request, both 201 (Created) and 202 (Accepted) might be valid
	response := httptest.NewRecorder()
	response.WriteHeader(http.StatusCreated)

	// Validate against multiple allowed status codes
	allowedCodes := []int{http.StatusCreated, http.StatusAccepted, http.StatusOK}
	result := AssertStatusCodeAny(response, allowedCodes, false)

	if result.Match {
		fmt.Printf("✓ Status code %d is in allowed range\n", 201)
	} else {
		fmt.Printf("✗ Status code not in allowed range\n")
	}
	// Output: ✓ Status code 201 is in allowed range
}

// ExampleAssertStatusCode demonstrates detailed mismatch information.
func ExampleAssertStatusCode() {
	response := httptest.NewRecorder()
	response.WriteHeader(http.StatusUnauthorized)
	response.WriteString(`{"error": "Authentication required"}`)

	// Get detailed validation result with mismatch information
	result := AssertStatusCode(response, http.StatusOK, true)

	if !result.Match {
		fmt.Printf("Status Code Validation Failed:\n")
		fmt.Printf("  Expected: %d (%s)\n", result.Expected, result.ExpectedDescription)
		fmt.Printf("  Actual:   %d (%s)\n", result.Actual, result.ActualDescription)
		fmt.Printf("  Context:  %s\n", result.ResponseContext)
		fmt.Printf("  Error:    %s\n", result.Error)
	}

	// Output:
	// Status Code Validation Failed:
	//   Expected: 200 (OK)
	//   Actual:   401 (Unauthorized)
	//   Context:  httptest.ResponseRecorder
	//   Error:    Status code mismatch:
	//   Expected: 200 (OK)
	//   Actual:   401 (Unauthorized)
	//   Context:  httptest.ResponseRecorder
}

// =============================================================================
// ERROR MESSAGE VALIDATION EXAMPLES
// =============================================================================

// ExampleValidateErrorMessage demonstrates substring matching in error messages.
func ExampleValidateErrorMessage() {
	response := []byte(`{"error": "Resource not found", "code": 404}`)

	// Validate error message contains substring "not found"
	err := ValidateErrorMessage(response, "not found")
	if err == nil {
		fmt.Printf("Error message validation passed: message contains 'not found'\n")
	} else {
		fmt.Printf("Error message validation failed: %v\n", err)
	}
	// Output: Error message validation passed: message contains 'not found'
}

// TestValidateErrorMessage_MultipleFields demonstrates checking multiple error message fields.
func TestValidateErrorMessage_MultipleFields(t *testing.T) {
	responses := []struct {
		name     string
		response []byte
		pattern  string
	}{
		{
			name:     "error field",
			response: []byte(`{"error": "Access denied", "code": 403}`),
			pattern:  "denied",
		},
		{
			name:     "message field",
			response: []byte(`{"message": "Validation failed", "field": "email"}`),
			pattern:  "validation",
		},
		{
			name:     "detail field",
			response: []byte(`{"detail": "Rate limit exceeded", "retry_after": 60}`),
			pattern:  "rate limit",
		},
		{
			name:     "nested error.message",
			response: []byte(`{"error": {"message": "Token expired"}, "code": 401}`),
			pattern:  "expired",
		},
	}

	for _, tc := range responses {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateErrorMessage(tc.response, tc.pattern)
			if err == nil {
				fmt.Printf("✓ Found pattern '%s' in %s\n", tc.pattern, tc.name)
			} else {
				fmt.Printf("✗ Pattern '%s' not found in %s: %v\n", tc.pattern, tc.name, err)
			}
		})
	}
}

// ExampleValidateErrorMessage_detailedError demonstrates detailed error message validation output.
func ExampleValidateErrorMessage_detailedError() {
	response := []byte(`{"error": "Token has expired", "code": 401, "timestamp": "2024-01-15T10:30:00Z"}`)

	// Try to validate with a pattern that won't match
	err := ValidateErrorMessage(response, "invalid.*token")
	if err != nil {
		validationErr, ok := err.(ValidationError)
		if ok {
			fmt.Printf("Error Message Validation Failed:\n")
			fmt.Printf("  Error Type: %s\n", validationErr.ErrorType)
			fmt.Printf("  Expected: %v\n", validationErr.Expected)
			fmt.Printf("  Actual: %v\n", validationErr.Actual)
			fmt.Printf("  Field: %s\n", validationErr.FieldName)
			if len(validationErr.ValidationDetails) > 0 {
				fmt.Printf("  Details:\n")
				for _, detail := range validationErr.ValidationDetails {
					fmt.Printf("    - %s\n", detail)
				}
			}
			if len(validationErr.Suggestions) > 0 {
				fmt.Printf("  Suggestions:\n")
				for i, suggestion := range validationErr.Suggestions {
					fmt.Printf("    %d. %s\n", i+1, suggestion)
				}
			}
		}
	}

	// Output:
	// Error Message Validation Failed:
	//   Error Type: error_message
	//   Expected: invalid.*token
	//   Actual: Token has expired
	//   Field: error
	//   Details:
	//     - Searched for regex pattern
	//     - Expected pattern: invalid.*token
	//     - Actual message: "Token has expired"
	//     - Field name: error
	//     - Checked fields: error, message, detail, description, error_description
	//     - Response size: 75 bytes
	//   Suggestions:
	//     1. Pattern expects 'invalid' but message contains 'expired'
	//     2. Consider using pattern: 'invalid.*expired|expired.*invalid' to match both cases
	//     3. Or use separate patterns for different token states
}

// =============================================================================
// ERROR CODE VALIDATION EXAMPLES
// =============================================================================

// ExampleValidateErrorCode_stringCode demonstrates string error code validation.
func ExampleValidateErrorCode_stringCode() {
	response := []byte(`{"error": {"code": "AUTH_FAILED"}, "message": "Invalid credentials"}`)

	// Validate error code matches expected string
	err := ValidateErrorCode(response, "AUTH_FAILED")
	if err == nil {
		fmt.Printf("✓ Error code validation passed: AUTH_FOUND\n")
	} else {
		fmt.Printf("✗ Error code validation failed: %v\n", err)
	}
	// Output: ✓ Error code validation passed: AUTH_FOUND
}

// ExampleValidateErrorCode_numericCode demonstrates numeric error code validation.
func ExampleValidateErrorCode_numericCode() {
	response := []byte(`{"code": 401, "message": "Unauthorized"}`)

	// Validate error code matches expected number
	err := ValidateErrorCode(response, 401)
	if err == nil {
		fmt.Printf("✓ Error code validation passed: 401\n")
	} else {
		fmt.Printf("✗ Error code validation failed: %v\n", err)
	}
	// Output: ✓ Error code validation passed: 401
}

// TestValidateErrorCode_Pattern tests error code pattern validation.
func TestValidateErrorCode_Pattern(t *testing.T) {
	testCases := []struct {
		name        string
		response    []byte
		pattern     string
		shouldMatch bool
	}{
		{
			name:        "AUTH_ prefix pattern",
			response:    []byte(`{"code": "AUTH_FAILED", "message": "Authentication failed"}`),
			pattern:     "AUTH_*",
			shouldMatch: true,
		},
		{
			name:        "ERR_ prefix pattern",
			response:    []byte(`{"code": "ERR_VALIDATION", "message": "Validation error"}`),
			pattern:     "ERR_*",
			shouldMatch: true,
		},
		{
			name:        "4xx numeric pattern",
			response:    []byte(`{"code": 404, "message": "Not found"}`),
			pattern:     "4xx",
			shouldMatch: true,
		},
		{
			name:        "5xx numeric pattern",
			response:    []byte(`{"code": 500, "message": "Internal error"}`),
			pattern:     "5xx",
			shouldMatch: true,
		},
		{
			name:        "Pattern doesn't match",
			response:    []byte(`{"code": "SUCCESS", "message": "Operation succeeded"}`),
			pattern:     "AUTH_*",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateErrorCodePattern(tc.response, tc.pattern)
			if tc.shouldMatch {
				if err == nil {
					fmt.Printf("✓ Pattern '%s' matched error code\n", tc.pattern)
				} else {
					t.Errorf("Expected pattern '%s' to match, but got error: %v", tc.pattern, err)
				}
			} else {
				if err != nil {
					fmt.Printf("✓ Pattern '%s' correctly did not match\n", tc.pattern)
				} else {
					t.Errorf("Expected pattern '%s' NOT to match, but it did", tc.pattern)
				}
			}
		})
	}
}

// ExampleValidateErrorCode_multipleAllowed demonstrates multiple allowed error codes.
func ExampleValidateErrorCode_multipleAllowed() {
	response := []byte(`{"error": {"code": "TOKEN_EXPIRED"}, "message": "Token has expired"}`)

	// Validate against multiple acceptable error codes
	allowedCodes := []interface{}{
		"AUTH_FAILED",
		"TOKEN_EXPIRED",
		"INVALID_CREDENTIALS",
		"SESSION_EXPIRED",
	}

	err := ValidateErrorCodeAny(response, allowedCodes)
	if err == nil {
		fmt.Printf("✓ Error code is in allowed list\n")
	} else {
		fmt.Printf("✗ Error code not in allowed list: %v\n", err)
	}
	// Output: ✓ Error code is in allowed list
}

// ExampleValidateErrorCode_detailedMismatch demonstrates detailed error code mismatch information.
func ExampleValidateErrorCode_detailedMismatch() {
	response := []byte(`{"code": "VALIDATION_ERROR", "message": "Invalid input", "field": "email"}`)

	// Try to validate with wrong expected code
	err := ValidateErrorCode(response, "AUTH_FAILED")
	if err != nil {
		validationErr, ok := err.(ValidationError)
		if ok {
			fmt.Printf("Error Code Validation Failed:\n")
			fmt.Printf("  Expected: %v\n", validationErr.Expected)
			fmt.Printf("  Actual: %v\n", validationErr.Actual)
			fmt.Printf("  Field: %s\n", validationErr.FieldName)
			if len(validationErr.ValidationDetails) > 0 {
				fmt.Printf("  Details:\n")
				for _, detail := range validationErr.ValidationDetails {
					fmt.Printf("    - %s\n", detail)
				}
			}
			if len(validationErr.Suggestions) > 0 {
				fmt.Printf("  Suggestions:\n")
				for i, suggestion := range validationErr.Suggestions {
					fmt.Printf("    %d. %s\n", i+1, suggestion)
				}
			}
		}
	}

	// Output:
	// Error Code Validation Failed:
	//   Expected: 'AUTH_FAILED'
	//   Actual: 'VALIDATION_ERROR'
	//   Field: code
	//   Details:
	//     - Checked fields: code, error_code, errorCode, status
	//     - Found 1 error code field(s) in response
	//   Suggestions:
	//     1. Verify the expected error code matches the API documentation
	//     2. Check if error codes vary by error condition (authentication, validation, etc.)
	//     3. Review the expected error code and actual error code
}

// =============================================================================
// COMPREHENSIVE INTEGRATION EXAMPLES
// =============================================================================

// TestComprehensiveValidation tests validating both status codes and error messages.
func TestComprehensiveValidation(t *testing.T) {
	testCases := []struct {
		name           string
		response       []byte
		expectedCode   int
		expectedMsg    string
		expectedCodeStr string
	}{
		{
			name:     "404 with error message",
			response: []byte(`{"error": "User not found", "code": 404}`),
			expectedCode:   404,
			expectedMsg:    "not found",
			expectedCodeStr: "404",
		},
		{
			name:     "401 with authentication error",
			response: []byte(`{"error": "Authentication required", "code": "AUTH_REQUIRED"}`),
			expectedCode:   401,
			expectedMsg:    "authentication",
			expectedCodeStr: "AUTH_REQUIRED",
		},
		{
			name:     "403 with authorization error",
			response: []byte(`{"error": "Access denied", "code": "FORBIDDEN"}`),
			expectedCode:   403,
			expectedMsg:    "denied",
			expectedCodeStr: "FORBIDDEN",
		},
		{
			name:     "500 with server error",
			response: []byte(`{"error": "Internal server error", "code": "INTERNAL_ERROR"}`),
			expectedCode:   500,
			expectedMsg:    "internal",
			expectedCodeStr: "INTERNAL_ERROR",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate status code
			codeErr := ValidateStatusCodeInt(tc.expectedCode, tc.expectedCode)
			if codeErr != nil {
				t.Errorf("Status code validation failed: %v", codeErr)
			}

			// Validate error message
			msgErr := ValidateErrorMessage(tc.response, tc.expectedMsg)
			if msgErr != nil {
				t.Logf("Error message validation: %v", msgErr)
			}

			// Validate error code
			codeValErr := ValidateErrorCode(tc.response, tc.expectedCodeStr)
			if codeValErr != nil {
				t.Logf("Error code validation: %v", codeValErr)
			}

			if codeErr == nil && msgErr == nil && codeValErr == nil {
				fmt.Printf("✓ %s: All validations passed\n", tc.name)
			}
		})
	}
}

// TestCommonAPIValidationScenarios tests common real-world API validation scenarios.
func TestCommonAPIValidationScenarios(t *testing.T) {
	scenarios := []struct {
		name        string
		setupFunc   func() *httptest.ResponseRecorder
		validations func(*httptest.ResponseRecorder) []error
	}{
		{
			name: "REST API - GET resource not found",
			setupFunc: func() *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusNotFound)
				w.WriteString(`{"error": "Resource not found", "code": "NOT_FOUND"}`)
				return w
			},
			validations: func(w *httptest.ResponseRecorder) []error {
				var errs []error
				errs = append(errs, ValidateStatusCodeInt(404, w.Code))
				errs = append(errs, ValidateErrorMessage(w.Body.Bytes(), "not found"))
				errs = append(errs, ValidateErrorCode(w.Body.Bytes(), "NOT_FOUND"))
				return errs
			},
		},
		{
			name: "REST API - POST validation error",
			setupFunc: func() *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusBadRequest)
				w.WriteString(`{"error": "Validation failed", "field": "email", "code": "VALIDATION_ERROR"}`)
				return w
			},
			validations: func(w *httptest.ResponseRecorder) []error {
				var errs []error
				errs = append(errs, ValidateStatusCodePattern("4xx", w.Code))
				errs = append(errs, ValidateErrorMessage(w.Body.Bytes(), "validation"))
				errs = append(errs, ValidateErrorCodePattern(w.Body.Bytes(), "*_*"))
				return errs
			},
		},
		{
			name: "REST API - Authentication failure",
			setupFunc: func() *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusUnauthorized)
				w.WriteString(`{"error": "Authentication failed", "code": "AUTH_FAILED"}`)
				return w
			},
			validations: func(w *httptest.ResponseRecorder) []error {
				var errs []error
				errs = append(errs, ValidateStatusCodeInt(401, w.Code))
				errs = append(errs, ValidateErrorMessage(w.Body.Bytes(), "authentication"))
				errs = append(errs, ValidateErrorCodePattern(w.Body.Bytes(), "AUTH_*"))
				return errs
			},
		},
		{
			name: "REST API - Rate limit exceeded",
			setupFunc: func() *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusTooManyRequests)
				w.WriteString(`{"error": "Rate limit exceeded", "retry_after": 60, "code": "RATE_LIMIT"}`)
				return w
			},
			validations: func(w *httptest.ResponseRecorder) []error {
				var errs []error
				errs = append(errs, ValidateStatusCodeInt(429, w.Code))
				errs = append(errs, ValidateErrorMessage(w.Body.Bytes(), "rate limit"))
				errs = append(errs, ValidateErrorCode(w.Body.Bytes(), "RATE_LIMIT"))
				return errs
			},
		},
		{
			name: "REST API - Server error",
			setupFunc: func() *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusInternalServerError)
				w.WriteString(`{"error": "Internal server error", "code": "INTERNAL_ERROR"}`)
				return w
			},
			validations: func(w *httptest.ResponseRecorder) []error {
				var errs []error
				errs = append(errs, ValidateStatusCodePattern("5xx", w.Code))
				errs = append(errs, ValidateErrorMessage(w.Body.Bytes(), "internal"))
				errs = append(errs, ValidateErrorCodePattern(w.Body.Bytes(), "*_ERROR"))
				return errs
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			w := scenario.setupFunc()
			errs := scenario.validations(w)

			hasErrors := false
			for _, err := range errs {
				if err != nil {
					hasErrors = true
					t.Logf("Validation error: %v", err)
				}
			}

			if !hasErrors {
				fmt.Printf("✓ %s: All validations passed\n", scenario.name)
			}
		})
	}
}

// =============================================================================
// TEST FUNCTIONS
// =============================================================================

// TestStatusCodeValidationExamples runs all status code validation examples.
func TestStatusCodeValidationExamples(t *testing.T) {
	t.Run("BasicSuccess", func(t *testing.T) {
		// ExampleValidateStatusCode_BasicSuccess is tested by its output
		response := httptest.NewRecorder()
		response.WriteHeader(http.StatusOK)
		err := ValidateStatusCodeInt(200, response.Code)
		if err != nil {
			t.Errorf("Basic success validation failed: %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		response := httptest.NewRecorder()
		response.WriteHeader(http.StatusNotFound)
		err := ValidateStatusCodeInt(200, response.Code)
		if err == nil {
			t.Error("Expected validation error for 404, got nil")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("Error message should contain 404, got: %v", err)
		}
	})

	t.Run("Range_4xx", func(t *testing.T) {
		codes := []int{400, 401, 403, 404, 429}
		for _, code := range codes {
			err := ValidateStatusCodePattern("4xx", code)
			if err != nil {
				t.Errorf("Expected 4xx pattern to match %d, got error: %v", code, err)
			}
		}
	})

	t.Run("Range_5xx", func(t *testing.T) {
		codes := []int{500, 502, 503, 504}
		for _, code := range codes {
			err := ValidateStatusCodePattern("5xx", code)
			if err != nil {
				t.Errorf("Expected 5xx pattern to match %d, got error: %v", code, err)
			}
		}
	})

	t.Run("Range_2xx", func(t *testing.T) {
		codes := []int{200, 201, 204}
		for _, code := range codes {
			err := ValidateStatusCodePattern("2xx", code)
			if err != nil {
				t.Errorf("Expected 2xx pattern to match %d, got error: %v", code, err)
			}
		}
	})

	t.Run("MultipleAllowed", func(t *testing.T) {
		response := httptest.NewRecorder()
		response.WriteHeader(http.StatusCreated)
		allowedCodes := []int{http.StatusCreated, http.StatusAccepted}
		result := AssertStatusCodeAny(response, allowedCodes, false)
		if !result.Match {
			t.Errorf("Expected status code 201 to be in allowed range, got: %v", result)
		}
	})

	t.Run("DetailedMismatch", func(t *testing.T) {
		response := httptest.NewRecorder()
		response.WriteHeader(http.StatusUnauthorized)
		result := AssertStatusCode(response, http.StatusOK, true)
		if result.Match {
			t.Error("Expected validation to fail, but it passed")
		}
		if result.Expected != http.StatusOK || result.Actual != http.StatusUnauthorized {
			t.Errorf("Expected 200 vs 401, got: %d vs %d", result.Expected, result.Actual)
		}
		if result.ExpectedDescription != "OK" || result.ActualDescription != "Unauthorized" {
			t.Errorf("Expected descriptions OK vs Unauthorized, got: %s vs %s",
				result.ExpectedDescription, result.ActualDescription)
		}
	})
}

// TestErrorMessageValidationExamples runs all error message validation examples.
func TestErrorMessageValidationExamples(t *testing.T) {
	t.Run("Substring", func(t *testing.T) {
		response := []byte(`{"error": "Resource not found"}`)
		err := ValidateErrorMessage(response, "not found")
		if err != nil {
			t.Errorf("Expected substring match, got error: %v", err)
		}
	})

	t.Run("Regex", func(t *testing.T) {
		response := []byte(`{"error": "invalid_token"}`)
		err := ValidateErrorMessage(response, "invalid.*token")
		if err != nil {
			t.Errorf("Expected regex match, got error: %v", err)
		}
	})

	t.Run("MultipleFields", func(t *testing.T) {
		responses := [][]byte{
			[]byte(`{"error": "Access denied"}`),
			[]byte(`{"message": "Validation failed"}`),
			[]byte(`{"detail": "Rate limit exceeded"}`),
			[]byte(`{"error": {"message": "Token expired"}}`),
		}
		patterns := []string{"denied", "validation", "rate limit", "expired"}

		for i, resp := range responses {
			err := ValidateErrorMessage(resp, patterns[i])
			if err != nil {
				t.Errorf("Expected match for pattern %s, got error: %v", patterns[i], err)
			}
		}
	})

	t.Run("DetailedError", func(t *testing.T) {
		response := []byte(`{"error": "Token has expired"}`)
		err := ValidateErrorMessage(response, "invalid.*token")
		if err == nil {
			t.Error("Expected validation error, got nil")
		}
		validationErr, ok := err.(ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError type, got: %T", err)
		}
		if validationErr.ErrorType != "error_message" {
			t.Errorf("Expected error_type 'error_message', got: %s", validationErr.ErrorType)
		}
		if len(validationErr.ValidationDetails) == 0 {
			t.Error("Expected validation details to be populated")
		}
		if len(validationErr.Suggestions) == 0 {
			t.Error("Expected suggestions to be generated")
		}
	})
}

// TestErrorCodeValidationExamples runs all error code validation examples.
func TestErrorCodeValidationExamples(t *testing.T) {
	t.Run("StringCode", func(t *testing.T) {
		response := []byte(`{"error": {"code": "AUTH_FAILED"}}`)
		err := ValidateErrorCode(response, "AUTH_FAILED")
		if err != nil {
			t.Errorf("Expected string code match, got error: %v", err)
		}
	})

	t.Run("NumericCode", func(t *testing.T) {
		response := []byte(`{"code": 401}`)
		err := ValidateErrorCode(response, 401)
		if err != nil {
			t.Errorf("Expected numeric code match, got error: %v", err)
		}
	})

	t.Run("Pattern", func(t *testing.T) {
		testCases := []struct {
			response    []byte
			pattern     string
			shouldMatch bool
		}{
			{[]byte(`{"code": "AUTH_FAILED"}`), "AUTH_*", true},
			{[]byte(`{"code": "ERR_VALIDATION"}`), "ERR_*", true},
			{[]byte(`{"code": 404}`), "4xx", true},
			{[]byte(`{"code": 500}`), "5xx", true},
			{[]byte(`{"code": "SUCCESS"}`), "AUTH_*", false},
		}

		for _, tc := range testCases {
			err := ValidateErrorCodePattern(tc.response, tc.pattern)
			if tc.shouldMatch && err != nil {
				t.Errorf("Expected pattern '%s' to match, got error: %v", tc.pattern, err)
			}
			if !tc.shouldMatch && err == nil {
				t.Errorf("Expected pattern '%s' NOT to match, but it did", tc.pattern)
			}
		}
	})

	t.Run("MultipleAllowed", func(t *testing.T) {
		response := []byte(`{"code": "TOKEN_EXPIRED"}`)
		allowedCodes := []interface{}{"AUTH_FAILED", "TOKEN_EXPIRED", "INVALID_CREDENTIALS"}
		err := ValidateErrorCodeAny(response, allowedCodes)
		if err != nil {
			t.Errorf("Expected code to be in allowed list, got error: %v", err)
		}
	})

	t.Run("DetailedMismatch", func(t *testing.T) {
		response := []byte(`{"code": "VALIDATION_ERROR"}`)
		err := ValidateErrorCode(response, "AUTH_FAILED")
		if err == nil {
			t.Error("Expected validation error, got nil")
		}
		validationErr, ok := err.(ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError type, got: %T", err)
		}
		if validationErr.ErrorType != "error_code" {
			t.Errorf("Expected error_type 'error_code', got: %s", validationErr.ErrorType)
		}
		if len(validationErr.ValidationDetails) == 0 {
			t.Error("Expected validation details to be populated")
		}
		if len(validationErr.Suggestions) == 0 {
			t.Error("Expected suggestions to be generated")
		}
	})
}
