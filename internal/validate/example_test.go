package validate_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/jedarden/armor/internal/validate"
)

// ExampleHTTPStatusCodeIsValid demonstrates how to validate HTTP status codes
// against both single and multiple expected codes.
func ExampleHTTPStatusCodeIsValid() {
	// Create a test response with status 200 OK
	resp := httptest.NewRecorder()
	resp.Code = 200
	httpResp := resp.Result()

	// Single status code validation
	isValid := validate.HTTPStatusCodeIsValid(httpResp, 200)
	fmt.Printf("Single code check (200): %v\n", isValid)

	// Multiple valid status codes (e.g., for range requests accepting 200 or 206)
	isValid = validate.HTTPStatusCodeIsValid(httpResp, []int{200, 206})
	fmt.Printf("Multiple codes check ([200, 206]): %v\n", isValid)

	// Output:
	// Single code check (200): true
	// Multiple codes check ([200, 206]): true
}

// ExampleHTTPStatusCodeIsValid_checkingNotFound demonstrates checking for 404 Not Found
func ExampleHTTPStatusCodeIsValid_checkingNotFound() {
	// Create a test response with status 404 Not Found
	resp := httptest.NewRecorder()
	resp.Code = 404
	httpResp := resp.Result()

	// Check if response is 404
	isNotFound := validate.HTTPStatusCodeIsValid(httpResp, 404)
	fmt.Printf("Is 404 Not Found: %v\n", isNotFound)

	// Output:
	// Is 404 Not Found: true
}

// ExampleHTTPStatusCodeIsValid_acceptingMultiple demonstrates accepting multiple success codes
func ExampleHTTPStatusCodeIsValid_acceptingMultiple() {
	tests := []struct {
		name     string
		status   int
		expected []int
	}{
		{"200 OK", 200, []int{200, 201, 204}},
		{"201 Created", 201, []int{200, 201, 204}},
		{"204 No Content", 204, []int{200, 201, 204}},
		{"206 Partial Content", 206, []int{200, 206}},
		{"404 Not Found", 404, []int{200, 201, 204}},
	}

	for _, tt := range tests {
		resp := httptest.NewRecorder()
		resp.Code = tt.status
		httpResp := resp.Result()

		isValid := validate.HTTPStatusCodeIsValid(httpResp, tt.expected)
		fmt.Printf("%s: valid=%v\n", tt.name, isValid)
	}

	// Output:
	// 200 OK: valid=true
	// 201 Created: valid=true
	// 204 No Content: valid=true
	// 206 Partial Content: valid=true
	// 404 Not Found: valid=false
}

// ExampleHTTPStatusCodeIsError demonstrates error detection helpers
func ExampleHTTPStatusCodeIsError() {
	statusCodes := []int{200, 201, 204, 400, 403, 404, 500, 502, 503}

	for _, code := range statusCodes {
		resp := httptest.NewRecorder()
		resp.Code = code
		httpResp := resp.Result()

		isError := validate.HTTPStatusCodeIsError(httpResp)
		isClientError := validate.HTTPStatusCodeIsClientError(httpResp)
		isServerError := validate.HTTPStatusCodeIsServerError(httpResp)

		fmt.Printf("Status %d: error=%v client=%v server=%v\n",
			code, isError, isClientError, isServerError)
	}

	// Output:
	// Status 200: error=false client=false server=false
	// Status 201: error=false client=false server=false
	// Status 204: error=false client=false server=false
	// Status 400: error=true client=true server=false
	// Status 403: error=true client=true server=false
	// Status 404: error=true client=true server=false
	// Status 500: error=true client=false server=true
	// Status 502: error=true client=false server=true
	// Status 503: error=true client=false server=true
}

// ExampleHTTPStatusCodeIsValid_realWorld demonstrates real-world usage patterns
func ExampleHTTPStatusCodeIsValid_realWorld() {
	// Simulate making an HTTP request
	makeRequest := func(status int) *http.Response {
		resp := httptest.NewRecorder()
		resp.Code = status
		return resp.Result()
	}

	// Example 1: Simple success check
	resp := makeRequest(200)
	if validate.HTTPStatusCodeIsValid(resp, 200) {
		fmt.Println("Request succeeded")
	}

	// Example 2: Accept multiple success codes (e.g., for PUT requests)
	resp = makeRequest(201)
	if validate.HTTPStatusCodeIsValid(resp, []int{200, 201, 204}) {
		fmt.Println("Resource created successfully")
	}

	// Example 3: Check for specific error codes
	resp = makeRequest(404)
	if validate.HTTPStatusCodeIsValid(resp, []int{400, 404, 500}) {
		fmt.Println("Request failed with expected error code")
	}

	// Example 4: Error categorization
	resp = makeRequest(403)
	if validate.HTTPStatusCodeIsClientError(resp) {
		fmt.Println("Client error - check authentication/authorization")
	}

	resp = makeRequest(503)
	if validate.HTTPStatusCodeIsServerError(resp) {
		fmt.Println("Server error - retry later")
	}

	// Output:
	// Request succeeded
	// Resource created successfully
	// Request failed with expected error code
	// Client error - check authentication/authorization
	// Server error - retry later
}

// ExampleHTTPStatusCodeIsValid_nilHandling demonstrates safe handling of nil responses
func ExampleHTTPStatusCodeIsValid_nilHandling() {
	var resp *http.Response = nil

	// All helper functions safely handle nil responses
	isValid := validate.HTTPStatusCodeIsValid(resp, 200)
	isError := validate.HTTPStatusCodeIsError(resp)
	isClientError := validate.HTTPStatusCodeIsClientError(resp)
	isServerError := validate.HTTPStatusCodeIsServerError(resp)

	fmt.Printf("Nil response handling: valid=%v error=%v client=%v server=%v\n",
		isValid, isError, isClientError, isServerError)

	// Output:
	// Nil response handling: valid=false error=false client=false server=false
}

// ExampleValidateErrorMessagePattern demonstrates error message pattern validation
func ExampleValidateErrorMessagePattern() {
	// Case-insensitive pattern matching for "not found" errors
	body := []byte(`{"error": "Authentication failed"}`)
	matches, err := validate.ValidateErrorMessagePattern(body, "authentication.*failed", true)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Pattern matches: %v\n", matches)

	// Output:
	// Pattern matches: true
}

// ExampleErrorCodeInResponse demonstrates error code detection
func ExampleErrorCodeInResponse() {
	body := []byte(`{"error_code": "UNAUTHORIZED", "message": "Access denied"}`)
	found, err := validate.ErrorCodeInResponse(body, "UNAUTHORIZED")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Error code found: %v\n", found)

	// Output:
	// Error code found: true
}

// ExampleGetErrorMessage demonstrates error message extraction
func ExampleGetErrorMessage() {
	body := []byte(`{"error": "Resource not found", "code": "NOT_FOUND"}`)
	message, err := validate.GetErrorMessage(body)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Error message: %s\n", message)

	// Output:
	// Error message: Resource not found
}

// ExampleGetErrorCode demonstrates error code extraction
func ExampleGetErrorCode() {
	body := []byte(`{"error_code": "AUTH_FAILED", "message": "Invalid credentials"}`)
	code, err := validate.GetErrorCode(body)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Error code: %s\n", code)

	// Output:
	// Error code: AUTH_FAILED
}

// ExampleValidateErrorMessagePattern_auth demonstrates authentication error validation
func ExampleValidateErrorMessagePattern_auth() {
	// Check for authentication/authorization error patterns
	testCases := []struct {
		body    string
		matches bool
	}{
		{`{"error": "Authentication failed"}`, true},
		{`{"message": "User not authorized"}`, true},
		{`{"detail": "Access denied"}`, true},
		{`{"error": "Permission denied"}`, true},
		{`{"message": "Resource not found"}`, false},
	}

	pattern := "(authentication|authorization|access|permission).*failed|denied"
	for _, tc := range testCases {
		matches, _ := validate.ValidateErrorMessagePattern([]byte(tc.body), pattern, true)
		fmt.Printf("Body: %s - Matches: %v\n", tc.body, matches)
	}

	// Output:
	// Body: {"error": "Authentication failed"} - Matches: true
	// Body: {"message": "User not authorized"} - Matches: true
	// Body: {"detail": "Access denied"} - Matches: true
	// Body: {"error": "Permission denied"} - Matches: true
	// Body: {"message": "Resource not found"} - Matches: false
}

// ExampleErrorCodeInResponse_oauth2 demonstrates OAuth2 error code validation
func ExampleErrorCodeInResponse_oauth2() {
	// Common OAuth2 error codes
	oauth2Errors := []struct {
		body      string
		errorCode string
	}{
		{`{"error": "invalid_token", "error_description": "The access token expired"}`, "invalid_token"},
		{`{"error": "invalid_grant", "error_description": "Invalid authorization code"}`, "invalid_grant"},
		{`{"error": "unauthorized_client", "error_description": "Client not authorized"}`, "unauthorized_client"},
		{`{"error": "access_denied", "error_description": "Resource owner denied access"}`, "access_denied"},
	}

	for _, tc := range oauth2Errors {
		found, _ := validate.ErrorCodeInResponse([]byte(tc.body), tc.errorCode)
		fmt.Printf("Error code %s found: %v\n", tc.errorCode, found)
	}

	// Output:
	// Error code invalid_token found: true
	// Error code invalid_grant found: true
	// Error code unauthorized_client found: true
	// Error code access_denied found: true
}

// ExampleValidateStatusCodeRange demonstrates status code range validation
func ExampleValidateStatusCodeRange() {
	// Simulate API responses with different status codes
	responses := []struct {
		status int
		desc   string
	}{
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{500, "Internal Server Error"},
		{502, "Bad Gateway"},
		{503, "Service Unavailable"},
	}

	for _, resp := range responses {
		// Check for success codes (2xx)
		isSuccess := resp.status >= 200 && resp.status < 300

		// Check for client errors (4xx)
		isClientError := resp.status >= 400 && resp.status < 500

		// Check for server errors (5xx)
		isServerError := resp.status >= 500 && resp.status < 600

		fmt.Printf("%d (%s): Success=%v, ClientError=%v, ServerError=%v\n",
			resp.status, resp.desc, isSuccess, isClientError, isServerError)
	}

	// Output:
	// 200 (OK): Success=true, ClientError=false, ServerError=false
	// 201 (Created): Success=true, ClientError=false, ServerError=false
	// 204 (No Content): Success=true, ClientError=false, ServerError=false
	// 400 (Bad Request): Success=false, ClientError=true, ServerError=false
	// 401 (Unauthorized): Success=false, ClientError=true, ServerError=false
	// 403 (Forbidden): Success=false, ClientError=true, ServerError=false
	// 404 (Not Found): Success=false, ClientError=true, ServerError=false
	// 500 (Internal Server Error): Success=false, ClientError=false, ServerError=true
	// 502 (Bad Gateway): Success=false, ClientError=false, ServerError=true
	// 503 (Service Unavailable): Success=false, ClientError=false, ServerError=true
}

// Status code and error message validation examples are demonstrated in other examples above

// =============================================================================
// STATUS CODE RANGE INT VALIDATION EXAMPLES
// =============================================================================

// ExampleValidateStatusCodeRangeInt demonstrates basic status code range validation using pattern strings
func ExampleValidateStatusCodeRangeInt() {
	// Validate that a status code falls within the 4xx range
	err := validate.ValidateStatusCodeRangeInt("4xx", 404)
	if err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Printf("404 is valid in 4xx range\n")
	}

	// Validate that a status code falls within the 5xx range
	err = validate.ValidateStatusCodeRangeInt("5xx", 500)
	if err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Printf("500 is valid in 5xx range\n")
	}

	// This returns an error because 200 is not in the 4xx range
	err = validate.ValidateStatusCodeRangeInt("4xx", 200)
	if err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	}

	// Output:
	// 404 is valid in 4xx range
	// 500 is valid in 5xx range
	// Validation failed: status code 200 is not in range 4xx (expected 400-499)
}

// ExampleValidateStatusCodeRangeInt_allRanges demonstrates validation across all status code ranges
func ExampleValidateStatusCodeRangeInt_allRanges() {
	statusCodes := []int{100, 200, 301, 404, 500}
	ranges := []string{"1xx", "2xx", "3xx", "4xx", "5xx"}

	for _, code := range statusCodes {
		for _, pattern := range ranges {
			err := validate.ValidateStatusCodeRangeInt(pattern, code)
			if err == nil {
				fmt.Printf("Status %d matches pattern %s\n", code, pattern)
				break // Found the matching range
			}
		}
	}

	// Output:
	// Status 100 matches pattern 1xx
	// Status 200 matches pattern 2xx
	// Status 301 matches pattern 3xx
	// Status 404 matches pattern 4xx
	// Status 500 matches pattern 5xx
}

// ExampleValidateStatusCodeRangeInt_errorHandling demonstrates proper error handling
func ExampleValidateStatusCodeRangeInt_errorHandling() {
	testCases := []struct {
		pattern string
		actual  int
	}{
		{"2xx", 200}, // Success
		{"2xx", 201}, // Success
		{"2xx", 404}, // Error - not in range
		{"4xx", 500}, // Error - not in range
		{"invalid", 200}, // Error - invalid pattern
	}

	for _, tc := range testCases {
		err := validate.ValidateStatusCodeRangeInt(tc.pattern, tc.actual)
		if err != nil {
			fmt.Printf("Pattern '%s' vs %d: ERROR - %v\n", tc.pattern, tc.actual, err)
		} else {
			fmt.Printf("Pattern '%s' vs %d: VALID\n", tc.pattern, tc.actual)
		}
	}

	// Output:
	// Pattern '2xx' vs 200: VALID
	// Pattern '2xx' vs 201: VALID
	// Pattern '2xx' vs 404: ERROR - status code 404 is not in range 2xx (expected 200-299)
	// Pattern '4xx' vs 500: ERROR - status code 500 is not in range 4xx (expected 400-499)
	// Pattern 'invalid' vs 200: ERROR - invalid pattern format: invalid (expected format: '4xx', '5xx', etc.)
}

// ExampleValidateStatusCodeRangeInt_realWorld demonstrates real-world usage patterns
func ExampleValidateStatusCodeRangeInt_realWorld() {
	// Simulate API response validation
	validateResponse := func(statusCode int) {
		// Check for success (2xx)
		if err := validate.ValidateStatusCodeRangeInt("2xx", statusCode); err == nil {
			fmt.Printf("Request succeeded with status %d\n", statusCode)
			return
		}

		// Check for client errors (4xx)
		if err := validate.ValidateStatusCodeRangeInt("4xx", statusCode); err == nil {
			fmt.Printf("Client error with status %d - check request parameters\n", statusCode)
			return
		}

		// Check for server errors (5xx)
		if err := validate.ValidateStatusCodeRangeInt("5xx", statusCode); err == nil {
			fmt.Printf("Server error with status %d - retry later\n", statusCode)
			return
		}

		// Check for redirects (3xx)
		if err := validate.ValidateStatusCodeRangeInt("3xx", statusCode); err == nil {
			fmt.Printf("Redirect with status %d\n", statusCode)
			return
		}

		fmt.Printf("Unexpected status code: %d\n", statusCode)
	}

	// Test various response codes
	validateResponse(200) // Success
	validateResponse(404) // Client error
	validateResponse(500) // Server error
	validateResponse(301) // Redirect

	// Output:
	// Request succeeded with status 200
	// Client error with status 404 - check request parameters
	// Server error with status 500 - retry later
	// Redirect with status 301
}

// ExampleParseStatusCodeRange demonstrates parsing status code range patterns
func ExampleParseStatusCodeRange() {
	patterns := []string{"1xx", "2xx", "3xx", "4xx", "5xx"}

	for _, pattern := range patterns {
		min, max, err := validate.ParseStatusCodeRange(pattern)
		if err != nil {
			fmt.Printf("Error parsing '%s': %v\n", pattern, err)
			continue
		}
		fmt.Printf("Pattern '%s' maps to range %d-%d\n", pattern, min, max)
	}

	// Output:
	// Pattern '1xx' maps to range 100-199
	// Pattern '2xx' maps to range 200-299
	// Pattern '3xx' maps to range 300-399
	// Pattern '4xx' maps to range 400-499
	// Pattern '5xx' maps to range 500-599
}

// ExampleGetStatusCodeRangeDescription demonstrates getting descriptions for status code ranges
func ExampleGetStatusCodeRangeDescription() {
	patterns := []string{"1xx", "2xx", "3xx", "4xx", "5xx"}

	for _, pattern := range patterns {
		desc, err := validate.GetStatusCodeRangeDescription(pattern)
		if err != nil {
			fmt.Printf("Error getting description for '%s': %v\n", pattern, err)
			continue
		}
		fmt.Printf("Pattern '%s': %s\n", pattern, desc)
	}

	// Output:
	// Pattern '1xx': Informational (1xx)
	// Pattern '2xx': Success (2xx)
	// Pattern '3xx': Redirection (3xx)
	// Pattern '4xx': Client Error (4xx)
	// Pattern '5xx': Server Error (5xx)
}

// ExampleValidateStatusCodeRangeInt_restAPI demonstrates REST API validation scenarios
func ExampleValidateStatusCodeRangeInt_restAPI() {
	// Simulate REST API endpoint responses
	responses := []struct {
		endpoint string
		status   int
	}{
		{"/users", 200},           // GET - success
		{"/users", 201},           // POST - created
		{"/users/123", 404},       // GET - not found
		{"/users/123", 403},       // PUT - forbidden
		{"/users", 500},           // Any - server error
		{"/users", 503},           // Any - unavailable
	}

	for _, resp := range responses {
		switch {
		case validate.ValidateStatusCodeRangeInt("2xx", resp.status) == nil:
			fmt.Printf("%s: Success (%d)\n", resp.endpoint, resp.status)
		case validate.ValidateStatusCodeRangeInt("4xx", resp.status) == nil:
			fmt.Printf("%s: Client error (%d) - fix request\n", resp.endpoint, resp.status)
		case validate.ValidateStatusCodeRangeInt("5xx", resp.status) == nil:
			fmt.Printf("%s: Server error (%d) - retry later\n", resp.endpoint, resp.status)
		default:
			fmt.Printf("%s: Unexpected status %d\n", resp.endpoint, resp.status)
		}
	}

	// Output:
	// /users: Success (200)
	// /users: Success (201)
	// /users/123: Client error (404) - fix request
	// /users/123: Client error (403) - fix request
	// /users: Server error (500) - retry later
	// /users: Server error (503) - retry later
}

// ExampleValidateStatusCodeRangeInt_errorMessages demonstrates detailed error messages
func ExampleValidateStatusCodeRangeInt_errorMessages() {
	// Show descriptive error messages for various mismatches
	testCases := []struct {
		pattern string
		actual  int
	}{
		{"2xx", 404},
		{"4xx", 200},
		{"5xx", 301},
	}

	for _, tc := range testCases {
		err := validate.ValidateStatusCodeRangeInt(tc.pattern, tc.actual)
		if err != nil {
			fmt.Printf("Pattern '%s' vs %d: %v\n", tc.pattern, tc.actual, err)
		}
	}

	// Output:
	// Pattern '2xx' vs 404: status code 404 is not in range 2xx (expected 200-299)
	// Pattern '4xx' vs 200: status code 200 is not in range 4xx (expected 400-499)
	// Pattern '5xx' vs 301: status code 301 is not in range 5xx (expected 500-599)
}

// ExampleValidateStatusCodeRangeInt_invalidPatterns demonstrates handling invalid patterns
func ExampleValidateStatusCodeRangeInt_invalidPatterns() {
	invalidPatterns := []struct {
		pattern string
		actual  int
	}{
		{"4x", 404},      // Too short
		{"4xxx", 404},    // Too long
		{"400", 404},     // No 'xx' suffix
		{"0xx", 4},       // Invalid century (0)
		{"6xx", 600},     // Invalid century (6)
		{"9xx", 900},     // Invalid century (9)
		{"xxx", 404},     // Non-digit century
		{"", 404},        // Empty pattern
	}

	for _, tc := range invalidPatterns {
		err := validate.ValidateStatusCodeRangeInt(tc.pattern, tc.actual)
		if err != nil {
			fmt.Printf("Invalid pattern '%s': %v\n", tc.pattern, err)
		}
	}

	// Output:
	// Invalid pattern '4x': invalid pattern format: 4x (expected format: '4xx', '5xx', etc.)
	// Invalid pattern '4xxx': invalid pattern format: 4xxx (expected format: '4xx', '5xx', etc.)
	// Invalid pattern '400': invalid pattern suffix: 400 (expected 'xx')
	// Invalid pattern '0xx': invalid pattern century: 0 (must be 1-5)
	// Invalid pattern '6xx': invalid pattern century: 6 (must be 1-5)
	// Invalid pattern '9xx': invalid pattern century: 9 (must be 1-5)
	// Invalid pattern 'xxx': invalid pattern century: x (must be 1-5)
	// Invalid pattern '': invalid pattern format:  (expected format: '4xx', '5xx', etc.)
}
