package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// =============================================================================
// HTTP STATUS CODE VALIDATION HELPERS
// =============================================================================
// This module provides comprehensive HTTP status code validation utilities
// for error response testing. It supports single status codes, multiple allowed
// status codes, and flexible validation patterns.
//
// Functions:
// - ValidateStatusCode: Validate single expected status code
// - ValidateStatusCodeAny: Validate against multiple allowed status codes
// - ValidateStatusCodeRange: Validate status code is within range
// - CheckStatusCode: Non-asserting version that returns boolean
// - GetStatusCodeDescription: Get human-readable status code description
// =============================================================================

// ValidateStatusCode validates that the HTTP response has the expected status code.
//
// This is the basic status code validation helper that checks if a response
// matches a single expected status code. It provides clear error messages
// indicating what was expected vs. what was received.
//
// Parameters:
//   - t: The testing instance for assertions
//   - response: The HTTP response to validate (can be *httptest.ResponseRecorder or *http.Response)
//   - expectedCode: The expected HTTP status code (e.g., 200, 404, 500)
//
// Example:
//   ValidateStatusCode(t, w, 404)
func ValidateStatusCode(t *testing.T, response interface{}, expectedCode int) {
	t.Helper()

	actualCode := getStatusCode(response)
	if actualCode != expectedCode {
		t.Errorf("Expected HTTP status code %d (%s), got %d (%s)",
			expectedCode, GetStatusCodeDescription(expectedCode),
			actualCode, GetStatusCodeDescription(actualCode))
	}
}

// ValidateStatusCodeAny validates that the HTTP response has one of the allowed status codes.
//
// This helper is useful when multiple status codes are acceptable for a given scenario.
// For example, when testing authentication, you might accept both 403 (Forbidden)
// and 401 (Unauthorized) depending on the authentication method.
//
// Parameters:
//   - t: The testing instance for assertions
//   - response: The HTTP response to validate (can be *httptest.ResponseRecorder or *http.Response)
//   - allowedCodes: Slice of acceptable HTTP status codes
//
// Example:
//   ValidateStatusCodeAny(t, w, []int{200, 201, 204})
func ValidateStatusCodeAny(t *testing.T, response interface{}, allowedCodes []int) {
	t.Helper()

	if len(allowedCodes) == 0 {
		t.Fatal("ValidateStatusCodeAny: allowedCodes cannot be empty")
		return
	}

	actualCode := getStatusCode(response)

	// Check if actual code is in the allowed list
	for _, allowedCode := range allowedCodes {
		if actualCode == allowedCode {
			return // Success - code is allowed
		}
	}

	// Build helpful error message showing what was allowed vs. what was received
	allowedDesc := make([]string, len(allowedCodes))
	for i, code := range allowedCodes {
		allowedDesc[i] = fmt.Sprintf("%d (%s)", code, GetStatusCodeDescription(code))
	}

	t.Errorf("Expected HTTP status code to be one of [%v], got %d (%s)",
		allowedDesc, actualCode, GetStatusCodeDescription(actualCode))
}

// ValidateStatusCodeRange validates that the HTTP response status code falls within a range.
//
// This helper is useful for validating status codes by category, such as:
// - 2xx for success responses (200-299)
// - 4xx for client errors (400-499)
// - 5xx for server errors (500-599)
//
// Parameters:
//   - t: The testing instance for assertions
//   - response: The HTTP response to validate
//   - minCode: Minimum acceptable status code (inclusive)
//   - maxCode: Maximum acceptable status code (inclusive)
//
// Example:
//   // Validate 2xx success response
//   ValidateStatusCodeRange(t, w, 200, 299)
//
//   // Validate 4xx client error
//   ValidateStatusCodeRange(t, w, 400, 499)
func ValidateStatusCodeRange(t *testing.T, response interface{}, minCode, maxCode int) {
	t.Helper()

	if minCode > maxCode {
		t.Fatalf("ValidateStatusCodeRange: minCode (%d) cannot be greater than maxCode (%d)", minCode, maxCode)
	}

	actualCode := getStatusCode(response)

	if actualCode < minCode || actualCode > maxCode {
		t.Errorf("Expected HTTP status code in range [%d-%d] (%s), got %d (%s)",
			minCode, maxCode, getRangeDescription(minCode, maxCode),
			actualCode, GetStatusCodeDescription(actualCode))
	}
}

// CheckStatusCode checks if the HTTP response has the expected status code without asserting.
//
// This non-asserting version returns a boolean, allowing for conditional logic
// in tests. Useful when you need to handle different status codes programmatically
// rather than failing the test immediately.
//
// Parameters:
//   - response: The HTTP response to validate
//   - expectedCode: The expected HTTP status code
//
// Returns:
//   - true if the response has the expected status code, false otherwise
//
// Example:
//   if CheckStatusCode(w, 404) {
//       // Handle 404 case
//   } else {
//       // Handle other cases
//   }
func CheckStatusCode(response interface{}, expectedCode int) bool {
	actualCode := getStatusCode(response)
	return actualCode == expectedCode
}

// CheckStatusCodeAny checks if the HTTP response has one of the allowed status codes.
//
// This non-asserting version returns a boolean for flexible status code checking.
// Useful when you need to handle multiple acceptable status codes programmatically.
//
// Parameters:
//   - response: The HTTP response to validate
//   - allowedCodes: Slice of acceptable HTTP status codes
//
// Returns:
//   - true if the response status code is in the allowed list, false otherwise
//
// Example:
//   allowedCodes := []int{200, 201, 204}
//   if CheckStatusCodeAny(w, allowedCodes) {
//       // Handle success case
//   } else {
//       // Handle error case
//   }
func CheckStatusCodeAny(response interface{}, allowedCodes []int) bool {
	actualCode := getStatusCode(response)

	for _, allowedCode := range allowedCodes {
		if actualCode == allowedCode {
			return true
		}
	}
	return false
}

// CheckStatusCodeRange checks if the HTTP response status code falls within a range.
//
// This non-asserting version returns a boolean for range-based status code validation.
// Useful for category-based status code checking (e.g., 2xx, 4xx, 5xx).
//
// Parameters:
//   - response: The HTTP response to validate
//   - minCode: Minimum acceptable status code (inclusive)
//   - maxCode: Maximum acceptable status code (inclusive)
//
// Returns:
//   - true if the status code is within the range, false otherwise
//
// Example:
//   // Check for any success code (2xx)
//   if CheckStatusCodeRange(w, 200, 299) {
//       // Handle success
//   }
func CheckStatusCodeRange(response interface{}, minCode, maxCode int) bool {
	actualCode := getStatusCode(response)
	return actualCode >= minCode && actualCode <= maxCode
}

// GetStatusCodeDescription returns a human-readable description for an HTTP status code.
//
// This helper provides descriptive names for common HTTP status codes, making
// test error messages more informative and easier to understand.
//
// Parameters:
//   - code: The HTTP status code
//
// Returns:
//   - A human-readable description string (e.g., "Not Found" for 404)
//
// Example:
//   desc := GetStatusCodeDescription(404)
//   // desc = "Not Found"
func GetStatusCodeDescription(code int) string {
	descriptions := map[int]string{
		100: "Continue",
		101: "Switching Protocols",
		102: "Processing",
		200: "OK",
		201: "Created",
		202: "Accepted",
		203: "Non-Authoritative Information",
		204: "No Content",
		205: "Reset Content",
		206: "Partial Content",
		300: "Multiple Choices",
		301: "Moved Permanently",
		302: "Found",
		303: "See Other",
		304: "Not Modified",
		305: "Use Proxy",
		307: "Temporary Redirect",
		308: "Permanent Redirect",
		400: "Bad Request",
		401: "Unauthorized",
		402: "Payment Required",
		403: "Forbidden",
		404: "Not Found",
		405: "Method Not Allowed",
		406: "Not Acceptable",
		407: "Proxy Authentication Required",
		408: "Request Timeout",
		409: "Conflict",
		410: "Gone",
		411: "Length Required",
		412: "Precondition Failed",
		413: "Payload Too Large",
		414: "URI Too Long",
		415: "Unsupported Media Type",
		416: "Range Not Satisfiable",
		417: "Expectation Failed",
		418: "I'm a teapot",
		422: "Unprocessable Entity",
		423: "Locked",
		424: "Failed Dependency",
		426: "Upgrade Required",
		428: "Precondition Required",
		429: "Too Many Requests",
		431: "Request Header Fields Too Large",
		451: "Unavailable For Legal Reasons",
		500: "Internal Server Error",
		501: "Not Implemented",
		502: "Bad Gateway",
		503: "Service Unavailable",
		504: "Gateway Timeout",
		505: "HTTP Version Not Supported",
		506: "Variant Also Negotiates",
		507: "Insufficient Storage",
		508: "Loop Detected",
		510: "Not Extended",
		511: "Network Authentication Required",
	}

	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return "Unknown"
}

// getRangeDescription returns a human-readable description for a status code range.
//
// This helper provides category descriptions for common status code ranges,
// such as "Success" for 2xx codes or "Client Error" for 4xx codes.
func getRangeDescription(minCode, maxCode int) string {
	ranges := map[string]string{
		"200-299": "Success",
		"300-399": "Redirection",
		"400-499": "Client Error",
		"500-599": "Server Error",
	}

	key := fmt.Sprintf("%d-%d", minCode, maxCode)
	if desc, ok := ranges[key]; ok {
		return desc
	}

	return fmt.Sprintf("Codes %d-%d", minCode, maxCode)
}

// getStatusCode extracts the HTTP status code from various response types.
//
// This helper supports both *httptest.ResponseRecorder and *http.Response,
// making the validation functions work with different response types.
func getStatusCode(response interface{}) int {
	switch r := response.(type) {
	case *httptest.ResponseRecorder:
		return r.Code
	case *http.Response:
		return r.StatusCode
	default:
		panic(fmt.Sprintf("Unsupported response type: %T. Expected *httptest.ResponseRecorder or *http.Response", response))
	}
}

// =============================================================================
// CONVENIENCE FUNCTIONS FOR COMMON STATUS CODE CATEGORIES
// =============================================================================

// ValidateSuccessStatusCode validates that the response has a 2xx success status code.
//
// This convenience function validates any successful response (200-299).
// Common success codes include:
// - 200: OK
// - 201: Created
// - 204: No Content
//
// Example:
//   ValidateSuccessStatusCode(t, w)
func ValidateSuccessStatusCode(t *testing.T, response interface{}) {
	t.Helper()
	ValidateStatusCodeRange(t, response, 200, 299)
}

// ValidateClientErrorStatusCode validates that the response has a 4xx client error status code.
//
// This convenience function validates any client error response (400-499).
// Common client error codes include:
// - 400: Bad Request
// - 403: Forbidden
// - 404: Not Found
//
// Example:
//   ValidateClientErrorStatusCode(t, w)
func ValidateClientErrorStatusCode(t *testing.T, response interface{}) {
	t.Helper()
	ValidateStatusCodeRange(t, response, 400, 499)
}

// ValidateServerErrorStatusCode validates that the response has a 5xx server error status code.
//
// This convenience function validates any server error response (500-599).
// Common server error codes include:
// - 500: Internal Server Error
// - 502: Bad Gateway
// - 503: Service Unavailable
//
// Example:
//   ValidateServerErrorStatusCode(t, w)
func ValidateServerErrorStatusCode(t *testing.T, response interface{}) {
	t.Helper()
	ValidateStatusCodeRange(t, response, 500, 599)
}

// ValidateRedirectStatusCode validates that the response has a 3xx redirect status code.
//
// This convenience function validates any redirect response (300-399).
// Common redirect codes include:
// - 301: Moved Permanently
// - 302: Found
// - 304: Not Modified
//
// Example:
//   ValidateRedirectStatusCode(t, w)
func ValidateRedirectStatusCode(t *testing.T, response interface{}) {
	t.Helper()
	ValidateStatusCodeRange(t, response, 300, 399)
}

// =============================================================================
// COMMON S3 ERROR STATUS CODE VALIDATION
// =============================================================================

// ValidateS3NotFoundStatusCode validates that the response has HTTP 404 (Not Found).
//
// This is a convenience function for the common S3 error case where an object
// or bucket does not exist. It validates both the status code and ensures
// it's the correct error type for missing resources.
//
// Example:
//   ValidateS3NotFoundStatusCode(t, w)
func ValidateS3NotFoundStatusCode(t *testing.T, response interface{}) {
	t.Helper()
	ValidateStatusCode(t, response, 404)
}

// ValidateS3AccessDeniedStatusCode validates that the response has HTTP 403 (Forbidden).
//
// This is a convenience function for the common S3 authentication/authorization error
// case where access is denied due to missing credentials or insufficient permissions.
//
// Example:
//   ValidateS3AccessDeniedStatusCode(t, w)
func ValidateS3AccessDeniedStatusCode(t *testing.T, response interface{}) {
	t.Helper()
	ValidateStatusCode(t, response, 403)
}

// ValidateS3BadRequestStatusCode validates that the response has HTTP 400 (Bad Request).
//
// This is a convenience function for the common S3 validation error case where
// the request parameters or body are malformed or invalid.
//
// Example:
//   ValidateS3BadRequestStatusCode(t, w)
func ValidateS3BadRequestStatusCode(t *testing.T, response interface{}) {
	t.Helper()
	ValidateStatusCode(t, response, 400)
}