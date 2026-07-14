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
