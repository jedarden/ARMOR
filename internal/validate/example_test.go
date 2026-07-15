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

	pattern := "(authentication|authorized?|access|permission).*(failed|denied)"
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
// with enhanced error reporting that includes range boundaries, distance from range, and suggestions.
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
	// Validation failed: status_code_range validation failed
	//   Expected: 4xx Client Error (400-499)
	//   Actual:   200 (OK)
	//   Context:  status code validation failed for pattern '4xx'
	//   Pattern:  Range pattern: 4xx, Distance: 200 codes below range
	//   Range:    4xx Client Error (400-499)
	//   Details:
	//     - Expected range: 400-499 (Client Error)
	//     - Actual status code: 200
	//     - Distance from valid range: 200 codes below range
	//     - Range pattern context: 4xx
	//     - Status code 200 is 200 codes below the minimum 400
	//   Suggestions:
	//     - The request succeeded - update test expectations if this is expected behavior
	//     - Verify the test is checking for the correct response type
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
// with detailed error information including range boundaries, distance, and suggestions.
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
	// Pattern '2xx' vs 404: ERROR - status_code_range validation failed
	//   Expected: 2xx Success (200-299)
	//   Actual:   404 (Not Found)
	//   Context:  status code validation failed for pattern '2xx'
	//   Pattern:  Range pattern: 2xx, Distance: 105 codes above range
	//   Range:    2xx Success (200-299)
	//   Details:
	//     - Expected range: 200-299 (Success)
	//     - Actual status code: 404
	//     - Distance from valid range: 105 codes above range
	//     - Range pattern context: 2xx
	//     - Status code 404 is 105 codes above the maximum 299
	//   Suggestions:
	//     - Review request parameters for errors
	//     - Check authentication credentials
	//     - Verify the resource exists and is accessible
	//
	// Pattern '4xx' vs 500: ERROR - status_code_range validation failed
	//   Expected: 4xx Client Error (400-499)
	//   Actual:   500 (Internal Server Error)
	//   Context:  status code validation failed for pattern '4xx'
	//   Pattern:  Range pattern: 4xx, Distance: 1 codes above range
	//   Range:    4xx Client Error (400-499)
	//   Details:
	//     - Expected range: 400-499 (Client Error)
	//     - Actual status code: 500
	//     - Distance from valid range: 1 codes above range
	//     - Range pattern context: 4xx
	//     - Status code 500 is 1 codes above the maximum 499
	//   Suggestions:
	//     - Check if this is a temporary server issue
	//     - Implement retry logic with backoff
	//     - Verify service status and availability
	//
	// Pattern 'invalid' vs 200: ERROR - status_code_range validation failed
	//   Expected: pattern in format 'Nxx' (3 chars)
	//   Actual:   'invalid' (7 chars)
	//   Context:  invalid pattern format: invalid
	//   Details:
	//     - Pattern 'invalid' has invalid length
	//     - Pattern must be exactly 3 characters (e.g., '4xx', '5xx')
	//     - Got 7 characters, expected 3
	//   Suggestions:
	//     - Verify the response format is correct
}

// ExampleValidateStatusCodeRangeInt_realWorld demonstrates real-world usage patterns
// with enhanced error reporting that shows range boundaries, distance, and context.
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
	// Pattern '2xx' vs 404: status_code_range validation failed
	//   Expected: 2xx Success (200-299)
	//   Actual:   404 (Not Found)
	//   Context:  status code validation failed for pattern '2xx'
	//   Pattern:  Range pattern: 2xx, Distance: 105 codes above range
	//   Range:    2xx Success (200-299)
	//   Details:
	//     - Expected range: 200-299 (Success)
	//     - Actual status code: 404
	//     - Distance from valid range: 105 codes above range
	//     - Range pattern context: 2xx
	//     - Status code 404 is 105 codes above the maximum 299
	//   Suggestions:
	//     - Review request parameters for errors
	//     - Check authentication credentials
	//     - Verify the resource exists and is accessible
	//
	// Pattern '4xx' vs 200: status_code_range validation failed
	//   Expected: 4xx Client Error (400-499)
	//   Actual:   200 (OK)
	//   Context:  status code validation failed for pattern '4xx'
	//   Pattern:  Range pattern: 4xx, Distance: 200 codes below range
	//   Range:    4xx Client Error (400-499)
	//   Details:
	//     - Expected range: 400-499 (Client Error)
	//     - Actual status code: 200
	//     - Distance from valid range: 200 codes below range
	//     - Range pattern context: 4xx
	//     - Status code 200 is 200 codes below the minimum 400
	//   Suggestions:
	//     - The request succeeded - update test expectations if this is expected behavior
	//     - Verify the test is checking for the correct response type
	//
	// Pattern '5xx' vs 301: status_code_range validation failed
	//   Expected: 5xx Server Error (500-599)
	//   Actual:   301 (Moved Permanently)
	//   Context:  status code validation failed for pattern '5xx'
	//   Pattern:  Range pattern: 5xx, Distance: 199 codes below range
	//   Range:    5xx Server Error (500-599)
	//   Details:
	//     - Expected range: 500-599 (Server Error)
	//     - Actual status code: 301
	//     - Distance from valid range: 199 codes below range
	//     - Range pattern context: 5xx
	//     - Status code 301 is 199 codes below the minimum 500
	//   Suggestions:
	//     - Review the unexpected status code
	//     - Check API documentation for valid codes
	//     - Verify the request is properly formatted
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
	// Invalid pattern '4x': status_code_range validation failed
	//   Expected: pattern in format 'Nxx' (3 chars)
	//   Actual:   '4x' (2 chars)
	//   Context:  invalid pattern format: 4x
	//   Details:
	//     - Pattern '4x' has invalid length
	//     - Pattern must be exactly 3 characters (e.g., '4xx', '5xx')
	//     - Got 2 characters, expected 3
	//   Suggestions:
	//     - Verify the response format is correct
	//
	// Invalid pattern '4xxx': status_code_range validation failed
	//   Expected: pattern in format 'Nxx' (3 chars)
	//   Actual:   '4xxx' (4 chars)
	//   Context:  invalid pattern format: 4xxx
	//   Details:
	//     - Pattern '4xxx' has invalid length
	//     - Pattern must be exactly 3 characters (e.g., '4xx', '5xx')
	//     - Got 4 characters, expected 3
	//   Suggestions:
	//     - Verify the response format is correct
	//
	// Invalid pattern '400': status_code_range validation failed
	//   Expected: pattern ending with 'xx'
	//   Actual:   '00'
	//   Context:  invalid pattern suffix in '400'
	//   Details:
	//     - Pattern '400' has invalid suffix
	//     - Pattern must end with 'xx' (e.g., '4xx', '5xx')
	//     - Got '00', expected 'xx'
	//   Suggestions:
	//     - Verify the response format is correct
	//
	// Invalid pattern '0xx': status_code_range validation failed
	//   Expected: century digit 1-5
	//   Actual:   '0' (ASCII 48)
	//   Context:  invalid pattern century in '0xx'
	//   Details:
	//     - Pattern '0xx' has invalid century digit
	//     - Valid century digits: 1 (1xx), 2 (2xx), 3 (3xx), 4 (4xx), 5 (5xx)
	//     - Got '0', expected digit between '1' and '5'
	//   Suggestions:
	//     - Verify the response format is correct
	//
	// Invalid pattern '6xx': status_code_range validation failed
	//   Expected: century digit 1-5
	//   Actual:   '6' (ASCII 54)
	//   Context:  invalid pattern century in '6xx'
	//   Details:
	//     - Pattern '6xx' has invalid century digit
	//     - Valid century digits: 1 (1xx), 2 (2xx), 3 (3xx), 4 (4xx), 5 (5xx)
	//     - Got '6', expected digit between '1' and '5'
	//   Suggestions:
	//     - Verify the response format is correct
	//
	// Invalid pattern '9xx': status_code_range validation failed
	//   Expected: century digit 1-5
	//   Actual:   '9' (ASCII 57)
	//   Context:  invalid pattern century in '9xx'
	//   Details:
	//     - Pattern '9xx' has invalid century digit
	//     - Valid century digits: 1 (1xx), 2 (2xx), 3 (3xx), 4 (4xx), 5 (5xx)
	//     - Got '9', expected digit between '1' and '5'
	//   Suggestions:
	//     - Verify the response format is correct
	//
	// Invalid pattern 'xxx': status_code_range validation failed
	//   Expected: century digit 1-5
	//   Actual:   'x' (ASCII 120)
	//   Context:  invalid pattern century in 'xxx'
	//   Details:
	//     - Pattern 'xxx' has invalid century digit
	//     - Valid century digits: 1 (1xx), 2 (2xx), 3 (3xx), 4 (4xx), 5 (5xx)
	//     - Got 'x', expected digit between '1' and '5'
	//   Suggestions:
	//     - Verify the response format is correct
	//
	// Invalid pattern '': status_code_range validation failed
	//   Expected: pattern in format 'Nxx' (3 chars)
	//   Actual:   '' (0 chars)
	//   Context:  invalid pattern format:
	//   Details:
	//     - Pattern '' has invalid length
	//     - Pattern must be exactly 3 characters (e.g., '4xx', '5xx')
	//     - Got 0 characters, expected 3
	//   Suggestions:
	//     - Verify the response format is correct
}

// ExampleValidateStatusCodeRangeWithDetails demonstrates enhanced range validation error reporting
// with comprehensive information about range boundaries, distance from range, and pattern context.
func ExampleValidateStatusCodeRangeWithDetails() {
	// Create a response with status 200
	resp := httptest.NewRecorder()
	resp.Code = 200
	httpResp := resp.Result()

	// Validate against 4xx range with detailed error information
	result := validate.ValidateStatusCodeRangeWithDetails(httpResp, validate.Range4xx)

	// The result includes comprehensive error information
	fmt.Printf("Valid: %v\n", result.Valid)
	fmt.Printf("Actual code: %d\n", result.ActualCode)
	fmt.Printf("Category: %s\n", result.Category)

	// Output:
	// Valid: false
	// Actual code: 200
	// Category: Client Error
}

// ExampleValidateStatusCodeRangeWithDetails_customRanges demonstrates range validation
// with custom status code ranges and detailed error reporting.
func ExampleValidateStatusCodeRangeWithDetails_customRanges() {
	// Define custom ranges for specific use cases
	customRange := validate.StatusCodeRange{
		Min:         200,
		Max:         204,
		Description: "Standard success responses",
	}

	// Test responses against custom range
	testCodes := []int{200, 201, 204, 206, 400, 500}

	for _, code := range testCodes {
		resp := httptest.NewRecorder()
		resp.Code = code
		httpResp := resp.Result()

		result := validate.ValidateStatusCodeRangeWithDetails(httpResp, customRange)

		if !result.Valid {
			fmt.Printf("Status %d: Invalid (distance: %s)\n", code, result.MismatchDetails)
		} else {
			fmt.Printf("Status %d: Valid within range %d-%d\n", code, customRange.Min, customRange.Max)
		}
	}

	// Output:
	// Status 200: Valid within range 200-204
	// Status 201: Valid within range 200-204
	// Status 204: Valid within range 200-204
	// Status 206: Invalid (distance: status_code_range validation failed
	//   Expected: 200-204 (Standard success responses) - 2xx range (custom) pattern
	//   Actual:   206 (Partial Content)
	//   Pattern:  Range pattern: 2xx range (custom), Distance: 2 codes above range
	//   Range:    200-204 (Standard success responses) - 2xx range (custom) pattern
	//   Details:
	//     - Expected range: 200-204 Standard success responses
	//     - Actual status code: 206
	//     - Distance from valid range: 2 codes above range
	//     - Range pattern context: 2xx range (custom)
	//     - Status code 206 is 2 codes above the maximum 204
	//   Suggestions:
	//     - The request succeeded - update test expectations if this is expected behavior
	//     - Verify the test is checking for the correct response type
	// )
	// Status 400: Invalid (distance: status_code_range validation failed
	//   Expected: 200-204 (Standard success responses) - 2xx range (custom) pattern
	//   Actual:   400 (Bad Request)
	//   Pattern:  Range pattern: 2xx range (custom), Distance: 196 codes above range
	//   Range:    200-204 (Standard success responses) - 2xx range (custom) pattern
	//   Details:
	//     - Expected range: 200-204 Standard success responses
	//     - Actual status code: 400
	//     - Distance from valid range: 196 codes above range
	//     - Range pattern context: 2xx range (custom)
	//     - Status code 400 is 196 codes above the maximum 204
	//   Suggestions:
	//     - Review request parameters for errors
	//     - Check authentication credentials
	//     - Verify the resource exists and is accessible
	// )
	// Status 500: Invalid (distance: status_code_range validation failed
	//   Expected: 200-204 (Standard success responses) - 2xx range (custom) pattern
	//   Actual:   500 (Internal Server Error)
	//   Pattern:  Range pattern: 2xx range (custom), Distance: 296 codes above range
	//   Range:    200-204 (Standard success responses) - 2xx range (custom) pattern
	//   Details:
	//     - Expected range: 200-204 Standard success responses
	//     - Actual status code: 500
	//     - Distance from valid range: 296 codes above range
	//     - Range pattern context: 2xx range (custom)
	//     - Status code 500 is 296 codes above the maximum 204
	//   Suggestions:
	//     - Check if this is a temporary server issue
	//     - Implement retry logic with backoff
	//     - Verify service status and availability
	// )
}

// ExampleValidateStatusCodeRangeWithDetails_allStandardRanges demonstrates validation
// across all standard HTTP status code ranges with detailed error information.
func ExampleValidateStatusCodeRangeWithDetails_allStandardRanges() {
	standardRanges := []validate.StatusCodeRange{
		validate.Range1xx,
		validate.Range2xx,
		validate.Range3xx,
		validate.Range4xx,
		validate.Range5xx,
	}

	testStatuses := []int{100, 200, 301, 404, 500}

	for _, status := range testStatuses {
		for _, statusRange := range standardRanges {
			resp := httptest.NewRecorder()
			resp.Code = status
			httpResp := resp.Result()

			result := validate.ValidateStatusCodeRangeWithDetails(httpResp, statusRange)

			if result.Valid {
				fmt.Printf("Status %d matches %s range (%d-%d)\n",
					status, statusRange.Description, statusRange.Min, statusRange.Max)
				break // Found the matching range
			}
		}
	}

	// Output:
	// Status 100 matches Informational range (100-199)
	// Status 200 matches Success range (200-299)
	// Status 301 matches Redirection range (300-399)
	// Status 404 matches Client Error range (400-499)
	// Status 500 matches Server Error range (500-599)
}

// =============================================================================
// COMPREHENSIVE EXAMPLES FOR ENHANCED VALIDATION ERRORS
// =============================================================================

// ExampleFormatStatusCodeError demonstrates enhanced status code error reporting
// with full detailed output including expected/actual values, context, and suggestions.
func ExampleFormatStatusCodeError() {
	// Create a status code mismatch error
	err := validate.FormatStatusCodeError(200, 404, "GET /api/users/123")
	
	fmt.Printf("Error Type: %s\n", err.ErrorType)
	fmt.Printf("Expected: %v\n", err.Expected)
	fmt.Printf("Actual: %v\n", err.Actual)
	fmt.Printf("Context: %s\n", err.Context)
	fmt.Println("\nFull Error Message:")
	fmt.Println(err.Error())
	fmt.Println("\nSuggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// Error Type: status_code
	// Expected: 200
	// Actual: 404
	// Context: GET /api/users/123
	//
	// Full Error Message:
	// status_code validation failed
	//   Expected: 200 (OK)
	//   Actual:   404 (Not Found)
	//   Context:  GET /api/users/123
	//   Suggestions:
	//     - Verify the endpoint URL is correct
	//     - Check if the resource ID or identifier exists
	//     - Ensure the resource hasn't been deleted or moved
	//
	// Suggestions:
	//   1. Verify the endpoint URL is correct
	//   2. Check if the resource ID or identifier exists
	//   3. Ensure the resource hasn't been deleted or moved
}

// ExampleFormatStatusCodeError_multipleCodes demonstrates enhanced error reporting
// when validating against multiple allowed status codes.
func ExampleFormatStatusCodeError_multipleCodes() {
	// Create error with multiple allowed codes
	err := validate.FormatStatusCodeError([]int{200, 201, 204}, 404, "POST /api/users")
	
	fmt.Printf("Error Type: %s\n", err.ErrorType)
	fmt.Printf("Expected: %v\n", err.Expected)
	fmt.Printf("Actual: %v\n", err.Actual)
	fmt.Println("\nFull Error Message:")
	fmt.Println(err.Error())
	fmt.Println("\nSuggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// Error Type: status_code
	// Expected: [200 201 204]
	// Actual: 404
	//
	// Full Error Message:
	// status_code validation failed
	//   Expected: one of [200 (OK), 201 (Created), 204 (No Content)]
	//   Actual:   404 (Not Found)
	//   Context:  POST /api/users
	//   Suggestions:
	//     - Verify the endpoint URL is correct
	//     - Check if the resource ID or identifier exists
	//     - Ensure the resource hasn't been deleted or moved
	//
	// Suggestions:
	//   1. Verify the endpoint URL is correct
	//   2. Check if the resource ID or identifier exists
	//   3. Ensure the resource hasn't been deleted or moved
}

// ExampleFormatStatusCodeRangeError demonstrates enhanced range validation error
// reporting with full details about range boundaries, distance from range, and context.
func ExampleFormatStatusCodeRangeError() {
	// Create a range validation error
	err := validate.FormatStatusCodeRangeError("4xx", 200, "Expected error response")
	
	fmt.Printf("Error Type: %s\n", err.ErrorType)
	fmt.Printf("Expected: %v\n", err.Expected)
	fmt.Printf("Actual: %v\n", err.Actual)
	fmt.Printf("Range Info: %s\n", err.RangeInfo)
	fmt.Printf("Context: %s\n", err.Context)
	fmt.Println("\nValidation Details:")
	for i, detail := range err.ValidationDetails {
		fmt.Printf("  %d. %s\n", i+1, detail)
	}
	fmt.Println("\nFull Error Message:")
	fmt.Println(err.Error())
	fmt.Println("\nSuggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// Error Type: status_code_range
	// Expected: 4xx (Client Error)
	// Actual: 200
	// Range Info: 400-499 (Client Error)
	// Context: Expected error response
	//
	// Validation Details:
	//   1. Status code 200 is outside range 400-499
	//
	// Full Error Message:
	// status_code_range validation failed
	//   Expected: 4xx (Client Error)
	//   Actual:   200 (OK)
	//   Context:  Expected error response
	//   Range:    400-499 (Client Error)
	//   Details:
	//     - Status code 200 is outside range 400-499
	//   Suggestions:
	//     - The request succeeded - update test expectations if this is expected behavior
	//     - Verify the test is checking for the correct response type
	//
	// Suggestions:
	//   1. The request succeeded - update test expectations if this is expected behavior
	//   2. Verify the test is checking for the correct response type
}

// ExampleFormatStatusCodeRangeError_serverError demonstrates 5xx range validation errors
// with comprehensive server error context and suggestions.
func ExampleFormatStatusCodeRangeError_serverError() {
	// Create a 5xx range validation error
	err := validate.FormatStatusCodeRangeError("5xx", 404, "API response validation")
	
	fmt.Printf("Error Type: %s\n", err.ErrorType)
	fmt.Printf("Range Info: %s\n", err.RangeInfo)
	fmt.Println("\nValidation Details:")
	for i, detail := range err.ValidationDetails {
		fmt.Printf("  %d. %s\n", i+1, detail)
	}
	fmt.Println("\nFull Error Message:")
	fmt.Println(err.Error())
	fmt.Println("\nSuggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// Error Type: status_code_range
	// Range Info: 500-599 (Server Error)
	//
	// Validation Details:
	//   1. Status code 404 is outside range 500-599
	//
	// Full Error Message:
	// status_code_range validation failed
	//   Expected: 5xx (Server Error)
	//   Actual:   404 (Not Found)
	//   Context:  API response validation
	//   Range:    500-599 (Server Error)
	//   Details:
	//     - Status code 404 is outside range 500-599
	//   Suggestions:
	//     - Check if this is a temporary server issue
	//     - Implement retry logic with backoff
	//     - Verify service status and availability
	//
	// Suggestions:
	//   1. Check if this is a temporary server issue
	//   2. Implement retry logic with backoff
	//   3. Verify service status and availability
}

// ExampleFormatErrorMessageError demonstrates enhanced error message pattern
// validation with full details about expected patterns, actual messages, and field context.
func ExampleFormatErrorMessageError() {
	// Create an error message pattern validation error
	err := validate.FormatErrorMessageError("invalid.*token", "access_denied", "error", "OAuth token validation")
	
	fmt.Printf("Error Type: %s\n", err.ErrorType)
	fmt.Printf("Expected Pattern: %v\n", err.Expected)
	fmt.Printf("Actual Message: %v\n", err.Actual)
	fmt.Printf("Field Name: %s\n", err.FieldName)
	fmt.Printf("Context: %s\n", err.Context)
	fmt.Println("\nFull Error Message:")
	fmt.Println(err.Error())
	fmt.Println("\nSuggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// Error Type: error_message
	// Expected Pattern: invalid.*token
	// Actual Message: access_denied
	// Field Name: error
	// Context: OAuth token validation
	//
	// Full Error Message:
	// error_message validation failed
	//   Expected: invalid.*token
	//   Actual:   access_denied
	//   Context:  OAuth token validation
	//   Field:    error
	//   Suggestions:
	//     - Review the error message for specific details
	//     - Check API documentation for this error type
	//     - Verify request parameters match requirements
	//
	// Suggestions:
	//   1. Review the error message for specific details
	//   2. Check API documentation for this error type
	//   3. Verify request parameters match requirements
}

// ExampleFormatErrorMessageError_userNotFound demonstrates user not found
// pattern validation with comprehensive error context.
func ExampleFormatErrorMessageError_userNotFound() {
	// Create a user not found pattern validation error
	err := validate.FormatErrorMessageError("User .* not found", "Invalid credentials", "message", "User lookup API")
	
	fmt.Printf("Error Type: %s\n", err.ErrorType)
	fmt.Printf("Expected Pattern: %v\n", err.Expected)
	fmt.Printf("Actual Message: %v\n", err.Actual)
	fmt.Printf("Field Name: %s\n", err.FieldName)
	fmt.Println("\nFull Error Message:")
	fmt.Println(err.Error())
	fmt.Println("\nSuggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// Error Type: error_message
	// Expected Pattern: User .* not found
	// Actual Message: Invalid credentials
	// Field Name: message
	//
	// Full Error Message:
	// error_message validation failed
	//   Expected: User .* not found
	//   Actual:   Invalid credentials
	//   Context:  User lookup API
	//   Field:    message
	//   Suggestions:
	//     - Review the error message for specific details
	//     - Check API documentation for this error type
	//     - Verify request parameters match requirements
	//
	// Suggestions:
	//   1. Review the error message for specific details
	//   2. Check API documentation for this error type
	//   3. Verify request parameters match requirements
}

// ExampleFormatValidationErrorWithDetails demonstrates the most comprehensive
// validation error with all fields populated including response snippets,
// validation details, and custom suggestions.
func ExampleFormatValidationErrorWithDetails() {
	// Create a comprehensive validation error with all fields
	err := validate.FormatValidationErrorWithDetails(
		"error_message",
		"User .* not found",
		"Internal server error",
		"User profile lookup API endpoint",
		`{"error": "Internal server error", "code": "SERVER_ERROR", "timestamp": "2026-07-15T10:30:45Z"}`,
		"error",
		"user_profile_service.go:142",
		[]string{"user_id", "profile_id", "auth_token"},
		"Expected pattern 'User .* not found' to match error message",
		"",
		[]string{
			"Pattern mismatch indicates server error instead of not found",
			"Expected user lookup error, got internal server error",
			"This suggests a backend service failure",
			"Check server logs and service health status",
		},
	)

	fmt.Printf("Error Type: %s\n", err.ErrorType)
	fmt.Printf("Expected: %v\n", err.Expected)
	fmt.Printf("Actual: %v\n", err.Actual)
	fmt.Printf("Context: %s\n", err.Context)
	fmt.Printf("Field Name: %s\n", err.FieldName)
	fmt.Printf("Location: %s\n", err.Location)
	fmt.Printf("Pattern Details: %s\n", err.PatternDetails)
	fmt.Printf("Response Snippet: %s\n", err.ResponseSnippet)
	fmt.Printf("Related Fields: %v\n", err.RelatedFields)
	fmt.Println("\nValidation Details:")
	for i, detail := range err.ValidationDetails {
		fmt.Printf("  %d. %s\n", i+1, detail)
	}
	fmt.Println("\nFull Error Message:")
	fmt.Println(err.Error())
	fmt.Println("\nCustom Suggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// Error Type: error_message
	// Expected: User .* not found
	// Actual: Internal server error
	// Context: User profile lookup API endpoint
	// Field Name: error
	// Location: user_profile_service.go:142
	// Pattern Details: Expected pattern 'User .* not found' to match error message
	// Response Snippet: {"error": "Internal server error", "code": "SERVER_ERROR", "timestamp": "2026-07-15T10:30:45Z"}
	// Related Fields: [user_id profile_id auth_token]
	//
	// Validation Details:
	//   1. Pattern mismatch indicates server error instead of not found
	//   2. Expected user lookup error, got internal server error
	//   3. This suggests a backend service failure
	//   4. Check server logs and service health status
	//
	// Full Error Message:
	// error_message validation failed
	//   Expected: User .* not found
	//   Actual:   Internal server error
	//   Context:  User profile lookup API endpoint
	//   Field:    error
	//   Location: user_profile_service.go:142
	//   Response: {"error": "Internal server error", "code": "SERVER_ERROR", "timestamp": "2026-07-15T10:30:45Z"}
	//   Related Fields: user_id, profile_id, auth_token
	//   Pattern: Expected pattern 'User .* not found' to match error message
	//   Details:
	//     - Pattern mismatch indicates server error instead of not found
	//     - Expected user lookup error, got internal server error
	//     - This suggests a backend service failure
	//     - Check server logs and service health status
	//   Suggestions:
	//     - Pattern mismatch indicates server error instead of not found
	//     - Expected user lookup error, got internal server error
	//     - This suggests a backend service failure
	//     - Check server logs and service health status
	//
	// Custom Suggestions:
	//   1. Pattern mismatch indicates server error instead of not found
	//   2. Expected user lookup error, got internal server error
	//   3. This suggests a backend service failure
	//   4. Check server logs and service health status
}

// ExampleFormatValidationErrorWithDetails_rangeValidation demonstrates enhanced
// range validation with comprehensive distance calculation and context.
func ExampleFormatValidationErrorWithDetails_rangeValidation() {
	// Create a range validation error with full details
	err := validate.FormatValidationErrorWithDetails(
		"status_code_range",
		"4xx",
		200,
		"Client error response validation",
		`{"status": "ok", "data": {"user": {"id": 123, "name": "John Doe"}}}`,
		"",
		"",
		nil,
		"",
		"400-499 (Client Error)",
		[]string{
			"Expected range: 400-499 (Client Error)",
			"Actual status code: 200",
			"Distance from valid range: 200 codes below range",
			"Range pattern context: 4xx",
			"Status code 200 is 200 codes below the minimum 400",
		},
	)

	fmt.Printf("Error Type: %s\n", err.ErrorType)
	fmt.Printf("Expected: %v\n", err.Expected)
	fmt.Printf("Actual: %v\n", err.Actual)
	fmt.Printf("Range Info: %s\n", err.RangeInfo)
	fmt.Printf("Context: %s\n", err.Context)
	fmt.Printf("Response Snippet: %s\n", err.ResponseSnippet)
	fmt.Println("\nValidation Details:")
	for i, detail := range err.ValidationDetails {
		fmt.Printf("  %d. %s\n", i+1, detail)
	}
	fmt.Println("\nFull Error Message:")
	fmt.Println(err.Error())
	fmt.Println("\nAuto-Generated Suggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// Error Type: status_code_range
	// Expected: 4xx
	// Actual: 200
	// Range Info: 400-499 (Client Error)
	// Context: Client error response validation
	// Response Snippet: {"status": "ok", "data": {"user": {"id": 123, "name": "John Doe"}}}
	//
	// Validation Details:
	//   1. Expected range: 400-499 (Client Error)
	//   2. Actual status code: 200
	//   3. Distance from valid range: 200 codes below range
	//   4. Range pattern context: 4xx
	//   5. Status code 200 is 200 codes below the minimum 400
	//
	// Full Error Message:
	// status_code_range validation failed
	//   Expected: 4xx (Client Error)
	//   Actual:   200 (OK)
	//   Context:  Client error response validation
	//   Range:    400-499 (Client Error)
	//   Response: {"status": "ok", "data": {"user": {"id": 123, "name": "John Doe"}}}
	//   Details:
	//     - Expected range: 400-499 (Client Error)
	//     - Actual status code: 200
	//     - Distance from valid range: 200 codes below range
	//     - Range pattern context: 4xx
	//     - Status code 200 is 200 codes below the minimum 400
	//   Suggestions:
	//     - The request succeeded - update test expectations if this is expected behavior
	//     - Verify the test is checking for the correct response type
	//
	// Auto-Generated Suggestions:
	//   1. The request succeeded - update test expectations if this is expected behavior
	//   2. Verify the test is checking for the correct response type
}

// ExampleValidationFormatter demonstrates the builder pattern for creating
// customized validation errors with full control over all fields.
func ExampleValidationFormatter() {
	// Use the builder pattern to create a custom validation error
	err := validate.NewValidationFormatter("custom_validation").
		WithExpected("required_field").
		WithActual(nil).
		WithContext("Form submission validation").
		WithResponseSnippet(`{"form": {"name": "", "email": "test@example.com"}}`).
		WithFieldName("name").
		WithPatternDetails("Required field validation failed").
		WithValidationDetails(
			"Field 'name' is empty",
			"Field 'name' is required",
			"Minimum length: 1 character",
		).
		WithSuggestions(
			"Provide a value for the 'name' field",
			"Ensure the name field is not empty",
			"Check form validation rules",
		).
		Format()

	fmt.Printf("Error Type: %s\n", err.ErrorType)
	fmt.Printf("Expected: %v\n", err.Expected)
	fmt.Printf("Actual: %v\n", err.Actual)
	fmt.Printf("Context: %s\n", err.Context)
	fmt.Printf("Field Name: %s\n", err.FieldName)
	fmt.Println("\nFull Error Message:")
	fmt.Println(err.Error())

	// Output:
	// Error Type: custom_validation
	// Expected: required_field
	// Actual:
	// Context: Form submission validation
	// Field Name: name
	//
	// Full Error Message:
	// custom_validation validation failed
	//   Expected: required_field
	//   Actual:
	//   Context:  Form submission validation
	//   Field:    name
	//   Pattern:  Required field validation failed
	//   Details:
	//     - Field 'name' is empty
	//     - Field 'name' is required
	//     - Minimum length: 1 character
	//   Response: {"form": {"name": "", "email": "test@example.com"}}
	//   Suggestions:
	//     - Provide a value for the 'name' field
	//     - Ensure the name field is not empty
	//     - Check form validation rules
}

// ExampleFormatValidationErrorHelper demonstrates the streamlined validation error
// formatting helper function with various usage patterns.
func ExampleFormatValidationErrorHelper() {
	// Basic usage with minimal parameters
	err := validate.FormatValidationErrorHelper("status_code", 200, 404)
	fmt.Println("Basic error:")
	fmt.Println(err.Error())
	fmt.Println()

	// With context for better debugging
	err = validate.FormatValidationErrorHelper("status_code", 200, 404,
		validate.WithValidationContext("GET /api/users/123"))
	fmt.Println("Error with context:")
	fmt.Println(err.Error())
	fmt.Println()

	// With custom suggestions
	err = validate.FormatValidationErrorHelper("custom_field", "required", "",
		validate.WithFieldName("email"),
		validate.WithSuggestions(
			"Email address is required",
			"Please provide a valid email",
		))
	fmt.Println("Error with custom suggestions:")
	fmt.Println(err.Error())
	fmt.Println()

	// Complete example with all options
	err = validate.FormatValidationErrorHelper("error_message",
		"User .* not found",
		"Internal server error",
		validate.WithValidationContext("User profile lookup API"),
		validate.WithFieldName("error"),
		validate.WithResponseSnippet(`{"error": "Internal server error", "code": "SERVER_ERROR"}`),
		validate.WithSuggestions(
			"Check if user exists in database",
			"Verify user ID is correct",
			"Review server logs for errors",
		),
	)
	fmt.Println("Complete error:")
	fmt.Println(err.Error())

	// Output:
	// Basic error:
	// status_code validation failed
	//   Expected: 200 (OK)
	//   Actual:   404 (Not Found)
	//   Suggestions:
	//     - Verify the endpoint URL is correct
	//     - Check if the resource ID or identifier exists
	//     - Ensure the resource hasn't been deleted or moved
	//
	// Error with context:
	// status_code validation failed
	//   Expected: 200 (OK)
	//   Actual:   404 (Not Found)
	//   Context:  GET /api/users/123
	//   Suggestions:
	//     - Verify the endpoint URL is correct
	//     - Check if the resource ID or identifier exists
	//     - Ensure the resource hasn't been deleted or moved
	//
	// Error with custom suggestions:
	// custom_field validation failed
	//   Expected: required
	//   Actual:
	//   Field:    email
	//   Suggestions:
	//     - Email address is required
	//     - Please provide a valid email
	//
	// Complete error:
	// error_message validation failed
	//   Expected: User .* not found
	//   Actual:   Internal server error
	//   Context:  User profile lookup API
	//   Field:    error
	//   Response: {"error": "Internal server error", "code": "SERVER_ERROR"}
	//   Suggestions:
	//     - Check if user exists in database
	//     - Verify user ID is correct
	//     - Review server logs for errors
}

// ExampleFormatValidationErrorHelper_realWorld demonstrates real-world usage patterns
// for the validation error helper in common scenarios.
func ExampleFormatValidationErrorHelper_realWorld() {
	// Scenario 1: API endpoint validation
	fmt.Println("=== API Endpoint Validation ===")
	err := validate.FormatValidationErrorHelper("status_code", 200, 404,
		validate.WithValidationContext("GET /api/users/123"),
		validate.WithSuggestions(
			"Verify the endpoint URL is correct",
			"Check if the user ID exists in the database",
			"Ensure the user hasn't been deleted",
		),
	)
	fmt.Printf("API Error: %v\n", err)
	fmt.Println()

	// Scenario 2: Error message pattern validation
	fmt.Println("=== Error Message Pattern Validation ===")
	err = validate.FormatValidationErrorHelper("error_message",
		"invalid.*token",
		"access_denied",
		validate.WithFieldName("error"),
		validate.WithValidationContext("OAuth token validation"),
		validate.WithPatternDetails("Expected pattern 'invalid.*token' to match error message"),
	)
	fmt.Printf("Pattern Error: %v\n", err)
	fmt.Println()

	// Scenario 3: Content-Type header validation
	fmt.Println("=== Content-Type Validation ===")
	err = validate.FormatValidationErrorHelper("content_type",
		"application/json",
		"text/html",
		validate.WithValidationContext("API response validation"),
		validate.WithFieldName("Content-Type"),
	)
	fmt.Printf("Content-Type Error: %v\n", err)
	fmt.Println()

	// Scenario 4: Range validation
	fmt.Println("=== Range Validation ===")
	err = validate.FormatValidationErrorHelper("status_code_range",
		"4xx",
		200,
		validate.WithValidationContext("Expected error response"),
		validate.WithRangeInfo("400-499 (Client Error)"),
		validate.WithValidationDetails(
			"Expected status code in range 400-499",
			"Got status code 200 indicating success",
			"Test expected error response but received success",
		),
	)
	fmt.Printf("Range Error: %v\n", err)
	fmt.Println()

	// Scenario 5: Custom validation with multiple details
	fmt.Println("=== Custom Field Validation ===")
	err = validate.FormatValidationErrorHelper("custom_field",
		"non_empty_string",
		"",
		validate.WithFieldName("username"),
		validate.WithValidationContext("User registration form"),
		validate.WithValidationDetails(
			"Field 'username' is empty",
			"Field 'username' is required",
			"Minimum length: 3 characters",
			"Maximum length: 20 characters",
		),
		validate.WithSuggestions(
			"Provide a username between 3-20 characters",
			"Username cannot be empty or contain only whitespace",
			"Check form validation rules for username requirements",
		),
	)
	fmt.Printf("Custom Validation Error: %v\n", err)

	// Output:
	// === API Endpoint Validation ===
	// API Error: status_code validation failed
	//   Expected: 200 (OK)
	//   Actual:   404 (Not Found)
	//   Context:  GET /api/users/123
	//   Suggestions:
	//     - Verify the endpoint URL is correct
	//     - Check if the user ID exists in the database
	//     - Ensure the user hasn't been deleted
	//
	// === Error Message Pattern Validation ===
	// Pattern Error: error_message validation failed
	//   Expected: invalid.*token
	//   Actual:   access_denied
	//   Context:  OAuth token validation
	//   Field:    error
	//   Pattern:  Expected pattern 'invalid.*token' to match error message
	//   Suggestions:
	//     - Review the error message for specific details
	//     - Check API documentation for this error type
	//     - Verify request parameters match requirements
	//
	// === Content-Type Validation ===
	// Content-Type Error: content_type validation failed
	//   Expected: application/json
	//   Actual:   text/html
	//   Context:  API response validation
	//   Field:    Content-Type
	//   Suggestions:
	//     - Verify Content-Type header matches request body format
	//     - Check if charset or boundary parameters are needed
	//     - Ensure the body is properly formatted for the content type
	//
	// === Range Validation ===
	// Range Error: status_code_range validation failed
	//   Expected: 4xx
	//   Actual:   200
	//   Context:  Expected error response
	//   Range:    400-499 (Client Error)
	//   Details:
	//     - Expected status code in range 400-499
	//     - Got status code 200 indicating success
	//     - Test expected error response but received success
	//   Suggestions:
	//     - The request succeeded - update test expectations if this is expected behavior
	//     - Verify the test is checking for the correct response type
	//
	// === Custom Field Validation ===
	// Custom Validation Error: custom_field validation failed
	//   Expected: non_empty_string
	//   Actual:
	//   Context:  User registration form
	//   Field:    username
	//   Details:
	//     - Field 'username' is empty
	//     - Field 'username' is required
	//     - Minimum length: 3 characters
	//     - Maximum length: 20 characters
	//   Suggestions:
	//     - Provide a username between 3-20 characters
	//     - Username cannot be empty or contain only whitespace
	//     - Check form validation rules for username requirements
}

// ExampleFormatValidationErrorHelper_differentValueTypes demonstrates the helper
// working with different value types for expected and actual parameters.
func ExampleFormatValidationErrorHelper_differentValueTypes() {
	// Integer values
	err := validate.FormatValidationErrorHelper("status_code", 200, 404)
	fmt.Printf("Integer values: %v\n", err.ErrorType)

	// String values
	err = validate.FormatValidationErrorHelper("error_message", "expected_pattern", "actual_message")
	fmt.Printf("String values: %v\n", err.ErrorType)

	// Slice of integers
	err = validate.FormatValidationErrorHelper("status_code", []int{200, 201, 204}, 404)
	fmt.Printf("Slice values: %v\n", err.ErrorType)

	// Nil expected value
	err = validate.FormatValidationErrorHelper("custom_validation", nil, "actual_value")
	fmt.Printf("Nil expected: %v\n", err.ErrorType)

	// Nil actual value
	err = validate.FormatValidationErrorHelper("custom_validation", "expected_value", nil)
	fmt.Printf("Nil actual: %v\n", err.ErrorType)

	// Output:
	// Integer values: status_code
	// String values: error_message
	// Slice values: status_code
	// Nil expected: custom_validation
	// Nil actual: custom_validation
}

// ExampleFormatValidationErrorHelper_autoGeneratedSuggestions demonstrates the
// auto-generated suggestions feature when custom suggestions are not provided.
func ExampleFormatValidationErrorHelper_autoGeneratedSuggestions() {
	// Status code 404 - auto-generates resource not found suggestions
	err := validate.FormatValidationErrorHelper("status_code", 200, 404,
		validate.WithValidationContext("GET /api/users/123"))
	fmt.Println("404 Error (auto suggestions):")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}
	fmt.Println()

	// Status code 401 - auto-generates authentication suggestions
	err = validate.FormatValidationErrorHelper("status_code", 200, 401,
		validate.WithValidationContext("POST /api/sensitive"))
	fmt.Println("401 Error (auto suggestions):")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}
	fmt.Println()

	// Status code 500 - auto-generates server error suggestions
	err = validate.FormatValidationErrorHelper("status_code", 200, 500,
		validate.WithValidationContext("GET /api/data"))
	fmt.Println("500 Error (auto suggestions):")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}
	fmt.Println()

	// Custom error type - auto-generates generic suggestions
	err = validate.FormatValidationErrorHelper("custom_validation", "expected", "actual")
	fmt.Println("Custom Error (auto suggestions):")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// 404 Error (auto suggestions):
	//   1. Verify the endpoint URL is correct
	//   2. Check if the resource ID or identifier exists
	//   3. Ensure the resource hasn't been deleted or moved
	//
	// 401 Error (auto suggestions):
	//   1. Verify authentication credentials are correct
	//   2. Check if API token or session has expired
	//   3. Ensure Authorization header is properly formatted (e.g., 'Bearer <token>')
	//   4. Confirm authentication method is supported
	//
	// 500 Error (auto suggestions):
	//   1. Implement retry logic with exponential backoff
	//   2. Check service status page for ongoing issues
	//   3. Contact support if the issue persists
	//   4. Verify request doesn't trigger server-side bugs
	//
	// Custom Error (auto suggestions):
	//   1. Review the request parameters and try again
}

// ExampleFormatValidationErrorHelper_vsBuilderPattern compares the streamlined
// helper with the builder pattern for different complexity scenarios.
func ExampleFormatValidationErrorHelper_vsBuilderPattern() {
	fmt.Println("=== Simple Scenario (Helper is cleaner) ===")

	// Using streamlined helper
	err := validate.FormatValidationErrorHelper("status_code", 200, 404,
		validate.WithValidationContext("GET /api/users"))
	fmt.Printf("Helper approach:\n%v\n\n", err)

	// Using builder pattern
	err = validate.NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users").
		Format()
	fmt.Printf("Builder pattern:\n%v\n\n", err)

	fmt.Println("=== Complex Scenario (Builder pattern offers more control) ===")

	// Using streamlined helper with multiple options
	err = validate.FormatValidationErrorHelper("error_message",
		"invalid.*token",
		"access_denied",
		validate.WithValidationContext("OAuth validation"),
		validate.WithFieldName("error"),
		validate.WithResponseSnippet(`{"error": "access_denied"}`),
		validate.WithPatternDetails("Regex pattern mismatch"),
		validate.WithValidationDetails("Pattern did not match", "Got different error"),
		validate.WithSuggestions("Check token format", "Verify OAuth scopes"),
	)
	fmt.Printf("Helper with options:\n%v\n\n", err)

	// Using builder pattern with all options
	err = validate.NewValidationFormatter("error_message").
		WithExpected("invalid.*token").
		WithActual("access_denied").
		WithContext("OAuth validation").
		WithFieldName("error").
		WithResponseSnippet(`{"error": "access_denied"}`).
		WithPatternDetails("Regex pattern mismatch").
		WithValidationDetails("Pattern did not match", "Got different error").
		WithSuggestions("Check token format", "Verify OAuth scopes").
		Format()
	fmt.Printf("Builder pattern:\n%v\n", err)

	// Output:
	// === Simple Scenario (Helper is cleaner) ===
	// Helper approach:
	// status_code validation failed
	//   Expected: 200 (OK)
	//   Actual:   404 (Not Found)
	//   Context:  GET /api/users
	//   Suggestions:
	//     - Verify the endpoint URL is correct
	//     - Check if the resource ID or identifier exists
	//     - Ensure the resource hasn't been deleted or moved
	//
	// Builder pattern:
	// status_code validation failed
	//   Expected: 200 (OK)
	//   Actual:   404 (Not Found)
	//   Context:  GET /api/users
	//   Suggestions:
	//     - Verify the endpoint URL is correct
	//     - Check if the resource ID or identifier exists
	//     - Ensure the resource hasn't been deleted or moved
	//
	// === Complex Scenario (Builder pattern offers more control) ===
	// Helper with options:
	// error_message validation failed
	//   Expected: invalid.*token
	//   Actual:   access_denied
	//   Context:  OAuth validation
	//   Field:    error
	//   Pattern:  Regex pattern mismatch
	//   Details:
	//     - Pattern did not match
	//     - Got different error
	//   Response: {"error": "access_denied"}
	//   Suggestions:
	//     - Check token format
	//     - Verify OAuth scopes
	//
	// Builder pattern:
	// error_message validation failed
	//   Expected: invalid.*token
	//   Actual:   access_denied
	//   Context:  OAuth validation
	//   Field:    error
	//   Pattern:  Regex pattern mismatch
	//   Details:
	//     - Pattern did not match
	//     - Got different error
	//   Response: {"error": "access_denied"}
	//   Suggestions:
	//     - Check token format
	//     - Verify OAuth scopes
}
