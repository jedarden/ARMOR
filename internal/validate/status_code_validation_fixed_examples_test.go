package validate

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

/*
Fixed Status Code and Error Message Validation Examples

This test file provides corrected examples with proper case sensitivity
for the validation functions.
*/

// TestFixedCommonAPIValidationScenarios tests common real-world API validation scenarios
// with corrected case-sensitive patterns.
func TestFixedCommonAPIValidationScenarios(t *testing.T) {
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
				// Fixed: Use case-insensitive regex pattern
				errs = append(errs, ValidateErrorMessage(w.Body.Bytes(), "(?i)validation"))
				errs = append(errs, ValidateErrorCodePattern(w.Body.Bytes(), "VALIDATION_*"))
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
				// Fixed: Use case-insensitive regex pattern
				errs = append(errs, ValidateErrorMessage(w.Body.Bytes(), "(?i)authentication"))
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
				// Fixed: Use case-insensitive regex pattern
				errs = append(errs, ValidateErrorMessage(w.Body.Bytes(), "(?i)rate limit"))
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
				// Fixed: Use case-insensitive regex pattern
				errs = append(errs, ValidateErrorMessage(w.Body.Bytes(), "(?i)internal"))
				errs = append(errs, ValidateErrorCodePattern(w.Body.Bytes(), "*_ERROR"))
				return errs
			},
		},
		{
			name: "REST API - Success case with 201",
			setupFunc: func() *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusCreated)
				w.WriteString(`{"id": 123, "message": "Resource created"}`)
				return w
			},
			validations: func(w *httptest.ResponseRecorder) []error {
				var errs []error
				errs = append(errs, ValidateStatusCodePattern("2xx", w.Code))
				return errs
			},
		},
		{
			name: "REST API - Forbidden with permission error",
			setupFunc: func() *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusForbidden)
				w.WriteString(`{"error": "Access denied", "code": "FORBIDDEN"}`)
				return w
			},
			validations: func(w *httptest.ResponseRecorder) []error {
				var errs []error
				errs = append(errs, ValidateStatusCodeInt(403, w.Code))
				errs = append(errs, ValidateErrorMessage(w.Body.Bytes(), "denied"))
				errs = append(errs, ValidateErrorCode(w.Body.Bytes(), "FORBIDDEN"))
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

// TestComprehensiveErrorCodeValidation demonstrates various error code validation patterns.
func TestComprehensiveErrorCodeValidation(t *testing.T) {
	testCases := []struct {
		name        string
		response    []byte
		validation  func([]byte) error
		shouldPass  bool
		description string
	}{
		{
			name:        "String error code match",
			response:    []byte(`{"error": {"code": "AUTH_FAILED"}, "message": "Invalid credentials"}`),
			validation:  func(body []byte) error { return ValidateErrorCode(body, "AUTH_FAILED") },
			shouldPass:  true,
			description: "Validates exact string error code match",
		},
		{
			name:        "Numeric error code match",
			response:    []byte(`{"code": 401, "message": "Unauthorized"}`),
			validation:  func(body []byte) error { return ValidateErrorCode(body, 401) },
			shouldPass:  true,
			description: "Validates numeric error code match",
		},
		{
			name:        "AUTH_ prefix pattern",
			response:    []byte(`{"code": "AUTH_FAILED", "message": "Authentication failed"}`),
			validation:  func(body []byte) error { return ValidateErrorCodePattern(body, "AUTH_*") },
			shouldPass:  true,
			description: "Validates wildcard pattern for authentication errors",
		},
		{
			name:        "4xx numeric pattern",
			response:    []byte(`{"code": 404, "message": "Not found"}`),
			validation:  func(body []byte) error { return ValidateErrorCodePattern(body, "4xx") },
			shouldPass:  true,
			description: "Validates 4xx status code range pattern",
		},
		{
			name:        "Multiple allowed codes",
			response:    []byte(`{"code": "TOKEN_EXPIRED", "message": "Token expired"}`),
			validation: func(body []byte) error {
				allowedCodes := []interface{}{"AUTH_FAILED", "TOKEN_EXPIRED", "INVALID_CREDENTIALS"}
				return ValidateErrorCodeAny(body, allowedCodes)
			},
			shouldPass:  true,
			description: "Validates against multiple acceptable error codes",
		},
		{
			name:        "Pattern mismatch",
			response:    []byte(`{"code": "SUCCESS", "message": "Operation succeeded"}`),
			validation:  func(body []byte) error { return ValidateErrorCodePattern(body, "AUTH_*") },
			shouldPass:  false,
			description: "Correctly fails when pattern doesn't match",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.validation(tc.response)
			passed := (err == nil) == tc.shouldPass

			if passed {
				fmt.Printf("✓ %s: %s\n", tc.name, tc.description)
			} else {
				t.Errorf("✗ %s: validation result doesn't match expectation (error: %v)", tc.name, err)
			}
		})
	}
}

// TestStatusCodeRangeValidations demonstrates comprehensive status code range testing.
func TestStatusCodeRangeValidations(t *testing.T) {
	testCases := []struct {
		name        string
		pattern     string
		testCodes   []int
		shouldMatch []bool
	}{
		{
			name:        "4xx range",
			pattern:     "4xx",
			testCodes:   []int{400, 401, 403, 404, 429, 200, 301, 500},
			shouldMatch: []bool{true, true, true, true, true, false, false, false},
		},
		{
			name:        "5xx range",
			pattern:     "5xx",
			testCodes:   []int{500, 502, 503, 504, 404, 200, 301},
			shouldMatch: []bool{true, true, true, true, false, false, false},
		},
		{
			name:        "2xx range",
			pattern:     "2xx",
			testCodes:   []int{200, 201, 204, 206, 404, 301, 500},
			shouldMatch: []bool{true, true, true, true, false, false, false},
		},
		{
			name:        "3xx range",
			pattern:     "3xx",
			testCodes:   []int{301, 302, 304, 307, 200, 404, 500},
			shouldMatch: []bool{true, true, true, true, false, false, false},
		},
		{
			name:        "1xx range",
			pattern:     "1xx",
			testCodes:   []int{100, 101, 102, 200, 404},
			shouldMatch: []bool{true, true, true, false, false},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for i, code := range tc.testCodes {
				err := ValidateStatusCodePattern(tc.pattern, code)
				expectedMatch := tc.shouldMatch[i]
				actualMatch := (err == nil)

				if actualMatch == expectedMatch {
					fmt.Printf("✓ %s: code %d matched as expected\n", tc.name, code)
				} else {
					t.Errorf("✗ %s: code %d - expected match=%v, got match=%v (error: %v)",
						tc.name, code, expectedMatch, actualMatch, err)
				}
			}
		})
	}
}

// TestDetailedMismatchInformation demonstrates the detailed error reporting capabilities.
func TestDetailedMismatchInformation(t *testing.T) {
	testCases := []struct {
		name        string
		response    []byte
		validation  func([]byte) error
		checkFields []string // Fields that should be present in ValidationError
	}{
		{
			name:     "Status code mismatch details",
			response: []byte(`{"error": "Not found"}`),
			validation: func(body []byte) error {
				return ValidateStatusCodeInt(200, 404)
			},
			checkFields: []string{"ErrorType", "Expected", "Actual", "Message"},
		},
		{
			name:     "Error message pattern mismatch",
			response: []byte(`{"error": "Token has expired"}`),
			validation: func(body []byte) error {
				return ValidateErrorMessage(body, "invalid.*token")
			},
			checkFields: []string{"ErrorType", "Expected", "Actual", "ValidationDetails", "Suggestions"},
		},
		{
			name:     "Error code mismatch",
			response: []byte(`{"code": "VALIDATION_ERROR", "message": "Invalid input"}`),
			validation: func(body []byte) error {
				return ValidateErrorCode(body, "AUTH_FAILED")
			},
			checkFields: []string{"ErrorType", "Expected", "Actual", "FieldName", "ValidationDetails", "Suggestions"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.validation(tc.response)
			if err == nil {
				t.Errorf("Expected validation error, got nil")
				return
			}

			validationErr, ok := err.(ValidationError)
			if !ok {
				t.Errorf("Expected ValidationError type, got: %T", err)
				return
			}

			// Check that expected fields are populated
			for _, field := range tc.checkFields {
				var isEmpty bool
				switch field {
				case "ErrorType":
					isEmpty = validationErr.ErrorType == ""
				case "Expected":
					isEmpty = validationErr.Expected == nil
				case "Actual":
					isEmpty = validationErr.Actual == nil
				case "Message":
					isEmpty = validationErr.Message == ""
				case "FieldName":
					isEmpty = validationErr.FieldName == ""
				case "ValidationDetails":
					isEmpty = len(validationErr.ValidationDetails) == 0
				case "Suggestions":
					isEmpty = len(validationErr.Suggestions) == 0
				}

				if isEmpty {
					t.Errorf("Field '%s' should be populated but is empty", field)
				} else {
					fmt.Printf("✓ %s: field '%s' is populated\n", tc.name, field)
				}
			}

			// Print the detailed error for inspection
			fmt.Printf("Detailed error output:\n%v\n", err)
		})
	}
}

// TestStatusCodeAndErrorCodeCombined validates both status code and error code together.
func TestStatusCodeAndErrorCodeCombined(t *testing.T) {
	testCases := []struct {
		name          string
		response      []byte
		statusCode    int
		expectedCode  interface{}
		shouldMatch   bool
	}{
		{
			name:         "401 with AUTH_FAILED",
			response:     []byte(`{"error": "Authentication failed", "code": "AUTH_FAILED"}`),
			statusCode:   401,
			expectedCode: "AUTH_FAILED",
			shouldMatch:  true,
		},
		{
			name:         "404 with NOT_FOUND",
			response:     []byte(`{"error": "Resource not found", "code": "NOT_FOUND"}`),
			statusCode:   404,
			expectedCode: "NOT_FOUND",
			shouldMatch:  true,
		},
		{
			name:         "403 with FORBIDDEN",
			response:     []byte(`{"error": "Access denied", "code": "FORBIDDEN"}`),
			statusCode:   403,
			expectedCode: "FORBIDDEN",
			shouldMatch:  true,
		},
		{
			name:         "500 with INTERNAL_ERROR",
			response:     []byte(`{"error": "Internal server error", "code": "INTERNAL_ERROR"}`),
			statusCode:   500,
			expectedCode: "INTERNAL_ERROR",
			shouldMatch:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock HTTP response
			resp := &http.Response{
				StatusCode: tc.statusCode,
				Body:       &closingReadCloser{bytes: tc.response},
			}

			// Validate both status code and error code
			valid, err := ValidateStatusCodeAndErrorCode(resp, tc.statusCode, tc.expectedCode.(string))
			if err != nil {
				t.Errorf("Validation function returned error: %v", err)
				return
			}

			if valid == tc.shouldMatch {
				fmt.Printf("✓ %s: combined validation passed\n", tc.name)
			} else {
				t.Errorf("✗ %s: expected match=%v, got match=%v", tc.name, tc.shouldMatch, valid)
			}
		})
	}
}

// closingReadCloser is a helper that implements io.ReadCloser
type closingReadCloser struct {
	bytes []byte
}

func (c *closingReadCloser) Read(p []byte) (n int, err error) {
	if len(c.bytes) == 0 {
		return 0, nil
	}
	n = copy(p, c.bytes)
	c.bytes = c.bytes[n:]
	return n, nil
}

func (c *closingReadCloser) Close() error {
	return nil
}
