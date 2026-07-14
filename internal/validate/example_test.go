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
