package server

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// TEST REQUEST HELPERS
// =============================================================================
// This file provides helper functions for making HTTP requests to test servers
// and validating error responses. These helpers simplify test setup and provide
// consistent validation patterns across tests.
//
// Design Philosophy:
// - Simple, composable helpers for common test scenarios
// - Type-safe request building and response validation
// - Consistent error reporting and assertion patterns
// - Support for both success and error response validation
// =============================================================================

// =============================================================================
// REQUEST BUILDERS
// =============================================================================

// TestRequestOptions holds options for making test requests.
//
// This type provides a flexible way to configure test requests without
// proliferating function parameters. Use it with the MakeTestRequest
// helper for consistent request building.
//
// Example:
//
//	opts := TestRequestOptions{
//	    Method: "GET",
//	    Path: "/api/resource",
//	    Headers: map[string]string{"Authorization": "Bearer token"},
//	    Body: strings.NewReader("request body"),
//	}
//	resp, err := MakeTestRequest(server.URL, opts)
type TestRequestOptions struct {
	// Method is the HTTP method (GET, POST, PUT, DELETE, etc.)
	Method string

	// Path is the request path (will be appended to server URL)
	Path string

	// Query parameters to add to the request URL
	QueryParams map[string]string

	// Headers are the HTTP headers to include
	Headers map[string]string

	// Body is the request body (optional)
	Body io.Reader

	// Timeout is the request timeout (default: 10 seconds)
	Timeout time.Duration

	// ExpectError is true when the request is expected to fail
	ExpectError bool
}

// MakeTestRequest makes an HTTP request to a test server.
//
// This helper simplifies making HTTP requests in tests by providing a clean
// interface for request configuration. It handles URL construction, header
// setting, and timeout configuration.
//
// Parameters:
//   - serverURL: Base URL of the test server
//   - opts: Request options (method, path, headers, body, etc.)
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred during the request
//
// Example:
//
//	resp, err := MakeTestRequest(server.URL, TestRequestOptions{
//	    Method: "GET",
//	    Path: "/api/resource",
//	    Headers: map[string]string{"Authorization": "Bearer token"},
//	})
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//
// Example with query parameters:
//
//	resp, err := MakeTestRequest(server.URL, TestRequestOptions{
//	    Method: "GET",
//	    Path: "/api/resource",
//	    QueryParams: map[string]string{"key": "value"},
//	})
//
// Example with body:
//
//	body := strings.NewReader(`{"data": "test"}`)
//	resp, err := MakeTestRequest(server.URL, TestRequestOptions{
//	    Method: "POST",
//	    Path: "/api/resource",
//	    Body: body,
//	    Headers: map[string]string{"Content-Type": "application/json"},
//	})
func MakeTestRequest(serverURL string, opts TestRequestOptions) (*http.Response, error) {
	// Set default timeout
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}

	// Set default method
	if opts.Method == "" {
		opts.Method = "GET"
	}

	// Construct URL with query parameters
	requestURL := serverURL + opts.Path
	if len(opts.QueryParams) > 0 {
		query := url.Values{}
		for key, value := range opts.QueryParams {
			query.Add(key, value)
		}
		requestURL += "?" + query.Encode()
	}

	// Create request
	var req *http.Request
	var err error

	if opts.Body != nil {
		req, err = http.NewRequest(opts.Method, requestURL, opts.Body)
	} else {
		req, err = http.NewRequest(opts.Method, requestURL, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	// Create client with timeout
	client := &http.Client{
		Timeout: opts.Timeout,
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		if opts.ExpectError {
			return nil, err // Expected error, return it
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// MakeGETRequest makes a GET request to a test server.
//
// This is a convenience helper for simple GET requests. Use it for basic
// read operations without complex configuration.
//
// Parameters:
//   - serverURL: Base URL of the test server
//   - path: Request path
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := MakeGETRequest(server.URL, "/api/resource")
//	if err != nil {
//	    t.Fatalf("GET request failed: %v", err)
//	}
func MakeGETRequest(serverURL string, path string) (*http.Response, error) {
	return MakeTestRequest(serverURL, TestRequestOptions{
		Method: "GET",
		Path:   path,
	})
}

// MakePOSTRequest makes a POST request to a test server.
//
// This helper simplifies making POST requests with a body. It automatically
// sets the Content-Type header to "application/xml" if not specified.
//
// Parameters:
//   - serverURL: Base URL of the test server
//   - path: Request path
//   - body: Request body as string
//   - contentType: Content-Type header (default: "application/xml")
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := MakePOSTRequest(server.URL, "/api/resource", "<data>test</data>", "application/xml")
//	if err != nil {
//	    t.Fatalf("POST request failed: %v", err)
//	}
func MakePOSTRequest(serverURL string, path string, body string, contentType string) (*http.Response, error) {
	if contentType == "" {
		contentType = "application/xml"
	}

	return MakeTestRequest(serverURL, TestRequestOptions{
		Method: "POST",
		Path:   path,
		Body:   strings.NewReader(body),
		Headers: map[string]string{
			"Content-Type": contentType,
		},
	})
}

// MakePUTRequest makes a PUT request to a test server.
//
// This helper simplifies making PUT requests with a body. It automatically
// sets the Content-Type header to "application/xml" if not specified.
//
// Parameters:
//   - serverURL: Base URL of the test server
//   - path: Request path
//   - body: Request body as string
//   - contentType: Content-Type header (default: "application/xml")
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := MakePUTRequest(server.URL, "/api/resource", "<data>updated</data>", "application/xml")
//	if err != nil {
//	    t.Fatalf("PUT request failed: %v", err)
//	}
func MakePUTRequest(serverURL string, path string, body string, contentType string) (*http.Response, error) {
	if contentType == "" {
		contentType = "application/xml"
	}

	return MakeTestRequest(serverURL, TestRequestOptions{
		Method: "PUT",
		Path:   path,
		Body:   strings.NewReader(body),
		Headers: map[string]string{
			"Content-Type": contentType,
		},
	})
}

// MakeDELETERequest makes a DELETE request to a test server.
//
// This is a convenience helper for simple DELETE requests. Use it for
// basic delete operations.
//
// Parameters:
//   - serverURL: Base URL of the test server
//   - path: Request path
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := MakeDELETERequest(server.URL, "/api/resource")
//	if err != nil {
//	    t.Fatalf("DELETE request failed: %v", err)
//	}
func MakeDELETERequest(serverURL string, path string) (*http.Response, error) {
	return MakeTestRequest(serverURL, TestRequestOptions{
		Method: "DELETE",
		Path:   path,
	})
}

// =============================================================================
// RESPONSE VALIDATION HELPERS
// =============================================================================

// ValidationResult holds the results of response validation.
//
// This type provides detailed validation results, including whether validation
// passed, specific field validations, and any error messages.
//
// Example:
//
//	result := ValidateResponseStructure(resp, ResponseValidationOptions{
//	    ExpectedStatusCode: 404,
//	    ExpectedErrorCode: "NoSuchKey",
//	})
//	if !result.IsValid {
//	    t.Errorf("Validation failed: %s", result.ErrorMessage)
//	}
type ValidationResult struct {
	// IsValid is true if all validation checks passed
	IsValid bool

	// ErrorMessage contains validation error details (empty if IsValid is true)
	ErrorMessage string

	// StatusCodeValid indicates if the status code matched expectations
	StatusCodeValid bool

	// ContentTypeValid indicates if the content type matched expectations
	ContentTypeValid bool

	// ErrorCodeValid indicates if the error code matched expectations
	ErrorCodeValid bool

	// MessageValid indicates if the error message was present and valid
	MessageValid bool

	// ResponseStructureValid indicates if the XML structure was valid
	ResponseStructureValid bool

	// ActualStatusCode contains the actual status code received
	ActualStatusCode int

	// ActualErrorCode contains the actual error code received
	ActualErrorCode string

	// ActualMessage contains the actual error message received
	ActualMessage string
}

// ResponseValidationOptions holds validation options for error responses.
//
// This type provides flexible validation configuration for error responses.
// All fields are optional - only validate what you specify.
//
// Example:
//
//	opts := ResponseValidationOptions{
//	    ExpectedStatusCode: 404,
//	    ExpectedErrorCode: "NoSuchKey",
//	    ExpectedMessageKeywords: []string{"not", "found"},
//	    MinMessageLength: 10,
//	}
type ResponseValidationOptions struct {
	// ExpectedStatusCode is the expected HTTP status code
	ExpectedStatusCode int

	// ExpectedErrorCode is the expected S3 error code
	ExpectedErrorCode string

	// ExpectedMessage contains text that should appear in the error message
	ExpectedMessage string

	// ExpectedMessageKeywords are alternative keywords (any match is acceptable)
	ExpectedMessageKeywords []string

	// MinMessageLength is the minimum acceptable message length
	MinMessageLength int

	// ExpectedContentType is the expected content type (default: "application/xml")
	ExpectedContentType string

	// RequireRequestId is true if the response must include a RequestId field
	RequireRequestId bool

	// ValidateStructure is true to check XML structure validity
	ValidateStructure bool
}

// ValidateResponseStructure validates an error response structure.
//
// This helper performs comprehensive validation of S3 error responses,
// checking status code, content type, error code, message, and structure.
//
// Parameters:
//   - resp: HTTP response to validate
//   - opts: Validation options
//
// Returns:
//   - ValidationResult: Detailed validation results
//
// Example:
//
//	result := ValidateResponseStructure(resp, ResponseValidationOptions{
//	    ExpectedStatusCode: 404,
//	    ExpectedErrorCode: "NoSuchKey",
//	    ExpectedMessageKeywords: []string{"not", "found"},
//	})
//	if !result.IsValid {
//	    t.Errorf("Validation failed: %s", result.ErrorMessage)
//	}
//
// Example with full validation:
//
//	result := ValidateResponseStructure(resp, ResponseValidationOptions{
//	    ExpectedStatusCode: 403,
//	    ExpectedErrorCode: "AccessDenied",
//	    ExpectedContentType: "application/xml",
//	    MinMessageLength: 15,
//	    ValidateStructure: true,
//	})
func ValidateResponseStructure(resp *http.Response, opts ResponseValidationOptions) ValidationResult {
	result := ValidationResult{
		IsValid:      true,
		ActualStatusCode: resp.StatusCode,
	}

	// Validate status code
	if opts.ExpectedStatusCode != 0 && resp.StatusCode != opts.ExpectedStatusCode {
		result.IsValid = false
		result.StatusCodeValid = false
		result.ErrorMessage = fmt.Sprintf("Expected status code %d, got %d", opts.ExpectedStatusCode, resp.StatusCode)
	} else {
		result.StatusCodeValid = true
	}

	// Validate content type
	expectedContentType := opts.ExpectedContentType
	if expectedContentType == "" {
		expectedContentType = "application/xml"
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, expectedContentType) {
		result.IsValid = false
		result.ContentTypeValid = false
		result.ErrorMessage = fmt.Sprintf("%s; Expected Content-Type '%s', got '%s'", result.ErrorMessage, expectedContentType, contentType)
	} else {
		result.ContentTypeValid = true
	}

	// Read and parse response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.IsValid = false
		result.ResponseStructureValid = false
		result.ErrorMessage = fmt.Sprintf("%s; Failed to read response body: %v", result.ErrorMessage, err)
		return result
	}
	defer resp.Body.Close()

	// Validate XML structure
	var s3Err S3Error
	if err := xml.Unmarshal(body, &s3Err); err != nil {
		result.IsValid = false
		result.ResponseStructureValid = false
		result.ErrorMessage = fmt.Sprintf("%s; Failed to parse S3 error response: %v", result.ErrorMessage, err)
		return result
	}
	result.ResponseStructureValid = true
	result.ActualErrorCode = s3Err.Code
	result.ActualMessage = s3Err.Message

	// Validate error code
	if opts.ExpectedErrorCode != "" && s3Err.Code != opts.ExpectedErrorCode {
		result.IsValid = false
		result.ErrorCodeValid = false
		result.ErrorMessage = fmt.Sprintf("%s; Expected error code '%s', got '%s'", result.ErrorMessage, opts.ExpectedErrorCode, s3Err.Code)
	} else {
		result.ErrorCodeValid = true
	}

	// Validate message
	if !validateErrorMessage(s3Err.Message, opts.ExpectedMessage, opts.ExpectedMessageKeywords, opts.MinMessageLength) {
		result.IsValid = false
		result.MessageValid = false
		result.ErrorMessage = fmt.Sprintf("%s; Error message validation failed", result.ErrorMessage)
	} else {
		result.MessageValid = true
	}

	return result
}

// validateErrorMessage validates the error message.
func validateErrorMessage(message string, expectedMessage string, expectedKeywords []string, minLength int) bool {
	// Check minimum length
	if minLength > 0 && len(message) < minLength {
		return false
	}

	// Check exact message match
	if expectedMessage != "" && message == expectedMessage {
		return true
	}

	// Check for keyword matches
	if len(expectedKeywords) > 0 {
		messageLower := strings.ToLower(message)
		for _, keyword := range expectedKeywords {
			if strings.Contains(messageLower, strings.ToLower(keyword)) {
				return true
			}
		}
		return false
	}

	// If no specific message validation is required, just check it's not empty
	return message != ""
}

// ValidateErrorResponse validates an error response and fails the test if invalid.
//
// This is a testing helper that validates an error response and calls t.Fatal()
// or t.Errorf() if validation fails. It provides detailed error messages for
// debugging failed tests.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to validate
//   - expectedStatusCode: Expected HTTP status code
//   - expectedErrorCode: Expected S3 error code
//
// Example:
//
//	resp, err := MakeGETRequest(server.URL, "/api/resource")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateErrorResponse(t, resp, 404, "NoSuchKey")
func ValidateErrorResponse(t *testing.T, resp *http.Response, expectedStatusCode int, expectedErrorCode string) {
	t.Helper()

	result := ValidateResponseStructure(resp, ResponseValidationOptions{
		ExpectedStatusCode: expectedStatusCode,
		ExpectedErrorCode:  expectedErrorCode,
		ValidateStructure:  true,
	})

	if !result.IsValid {
		t.Errorf("Error response validation failed: %s", result.ErrorMessage)
		t.Errorf("  Status code: %d (valid: %v)", resp.StatusCode, result.StatusCodeValid)
		t.Errorf("  Error code: %s (valid: %v)", result.ActualErrorCode, result.ErrorCodeValid)
		t.Errorf("  Message: %s (valid: %v)", result.ActualMessage, result.MessageValid)
	}
}

// ValidateSuccessResponse validates a successful HTTP response.
//
// This helper validates that a response indicates success (2xx status code)
// and has the expected content type. Use it for testing successful operations.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to validate
//   - expectedStatusCode: Expected status code (default: 200)
//   - expectedContentType: Expected content type (default: "application/xml")
//
// Example:
//
//	resp, err := MakeGETRequest(server.URL, "/api/resource")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateSuccessResponse(t, resp, 200, "application/xml")
func ValidateSuccessResponse(t *testing.T, resp *http.Response, expectedStatusCode int, expectedContentType string) {
	t.Helper()

	// Set defaults
	if expectedStatusCode == 0 {
		expectedStatusCode = 200
	}
	if expectedContentType == "" {
		expectedContentType = "application/xml"
	}

	// Validate status code is success
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Errorf("Expected success status code (2xx), got %d", resp.StatusCode)
		return
	}

	// Validate specific status code
	if resp.StatusCode != expectedStatusCode {
		t.Errorf("Expected status code %d, got %d", expectedStatusCode, resp.StatusCode)
	}

	// Validate content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, expectedContentType) {
		t.Errorf("Expected Content-Type '%s', got '%s'", expectedContentType, contentType)
	}
}

// GetErrorServerS3Error extracts an S3 error from an HTTP response.
//
// This helper reads the response body, parses the S3 error XML, and returns
// the parsed error structure. It's used internally by other validation helpers.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to extract error from
//
// Returns:
//   - *S3Error: Parsed S3 error structure
//
// Example:
//
//	resp, err := MakeGETRequest(server.URL, "/api/resource")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	s3Err := GetErrorServerS3Error(t, resp)
//	if s3Err.Code != "NoSuchKey" {
//	    t.Errorf("Expected NoSuchKey, got %s", s3Err.Code)
//	}
func GetErrorServerS3Error(t *testing.T, resp *http.Response) *S3Error {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	defer resp.Body.Close()

	var s3Err S3Error
	if err := xml.Unmarshal(body, &s3Err); err != nil {
		t.Fatalf("Failed to parse S3 error response: %v", err)
	}

	return &s3Err
}

// ExtractAndValidateError extracts and validates an S3 error from a response.
//
// This helper reads the response body, parses the S3 error, and validates it
// against expectations. It returns the parsed error for further assertions.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to extract error from
//   - expectedStatusCode: Expected HTTP status code
//   - expectedErrorCode: Expected S3 error code
//
// Returns:
//   - *S3Error: Parsed S3 error structure
//
// Example:
//
//	resp, err := MakeGETRequest(server.URL, "/api/resource")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	s3Err := ExtractAndValidateError(t, resp, 404, "NoSuchKey")
//	if s3Err.Message == "" {
//	    t.Error("Expected non-empty error message")
//	}
func ExtractAndValidateError(t *testing.T, resp *http.Response, expectedStatusCode int, expectedErrorCode string) *S3Error {
	t.Helper()

	// Validate status code
	if resp.StatusCode != expectedStatusCode {
		t.Errorf("Expected status code %d, got %d", expectedStatusCode, resp.StatusCode)
	}

	// Extract error
	s3Err := GetErrorServerS3Error(t, resp)

	// Validate error code
	if s3Err.Code != expectedErrorCode {
		t.Errorf("Expected error code '%s', got '%s'", expectedErrorCode, s3Err.Code)
	}

	return s3Err
}

// =============================================================================
// REQUEST/RESPONSE PAIR HELPERS
// =============================================================================

// RequestAndValidateError makes a request and validates the error response.
//
// This is a convenience helper that combines request making and error validation
// in a single call. Use it for streamlined error testing.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - opts: Request options
//   - expectedStatusCode: Expected HTTP status code
//   - expectedErrorCode: Expected S3 error code
//
// Returns:
//   - *S3Error: Parsed S3 error structure
//
// Example:
//
//	s3Err := RequestAndValidateError(t, server.URL, TestRequestOptions{
//	    Method: "GET",
//	    Path: "/api/resource",
//	}, 404, "NoSuchKey")
//	if s3Err.Message == "" {
//	    t.Error("Expected non-empty error message")
//	}
func RequestAndValidateError(t *testing.T, serverURL string, opts TestRequestOptions, expectedStatusCode int, expectedErrorCode string) *S3Error {
	t.Helper()

	resp, err := MakeTestRequest(serverURL, opts)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	return ExtractAndValidateError(t, resp, expectedStatusCode, expectedErrorCode)
}

// RequestAndValidateSuccess makes a request and validates success response.
//
// This is a convenience helper that combines request making and success validation
// in a single call. Use it for streamlined success testing.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - opts: Request options
//   - expectedStatusCode: Expected status code (default: 200)
//
// Example:
//
//	RequestAndValidateSuccess(t, server.URL, TestRequestOptions{
//	    Method: "GET",
//	    Path: "/api/resource",
//	}, 200)
func RequestAndValidateSuccess(t *testing.T, serverURL string, opts TestRequestOptions, expectedStatusCode int) {
	t.Helper()

	resp, err := MakeTestRequest(serverURL, opts)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	ValidateSuccessResponse(t, resp, expectedStatusCode, "")
}

// =============================================================================
// BATCH REQUEST HELPERS
// =============================================================================

// BatchRequestResult holds the result of a batch request.
//
// This type represents the result of a single request in a batch operation.
type BatchRequestResult struct {
	// Index is the request index in the batch
	Index int

	// Response is the HTTP response (nil if request failed)
	Response *http.Response

	// Error is any error that occurred during the request
	Error error

	// Duration is how long the request took
	Duration time.Duration
}

// MakeBatchRequests makes multiple requests concurrently.
//
// This helper executes multiple requests concurrently and returns all results.
// Use it for testing concurrent access and load scenarios.
//
// Parameters:
//   - serverURL: Base URL of the test server
//   - requests: List of request options
//
// Returns:
//   - []BatchRequestResult: Results for all requests
//
// Example:
//
//	requests := []TestRequestOptions{
//	    {Method: "GET", Path: "/resource1"},
//	    {Method: "GET", Path: "/resource2"},
//	    {Method: "GET", Path: "/resource3"},
//	}
//	results := MakeBatchRequests(server.URL, requests)
//	for _, result := range results {
//	    if result.Error != nil {
//	        t.Errorf("Request %d failed: %v", result.Index, result.Error)
//	    }
//	}
func MakeBatchRequests(serverURL string, requests []TestRequestOptions) []BatchRequestResult {
	results := make([]BatchRequestResult, len(requests))

	// Create channel for results
	resultChan := make(chan BatchRequestResult, len(requests))

	// Make requests concurrently
	for i, opts := range requests {
		go func(index int, options TestRequestOptions) {
			start := time.Now()
			resp, err := MakeTestRequest(serverURL, options)
			duration := time.Since(start)

			resultChan <- BatchRequestResult{
				Index:    index,
				Response: resp,
				Error:    err,
				Duration: duration,
			}
		}(i, opts)
	}

	// Collect results
	for i := 0; i < len(requests); i++ {
		result := <-resultChan
		results[result.Index] = result
	}

	return results
}

// =============================================================================
// DOCUMENTATION
// =============================================================================

/*
Usage Examples:

1. Simple GET request with error validation:

    resp, err := MakeGETRequest(server.URL, "/api/resource")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
    ValidateErrorResponse(t, resp, 404, "NoSuchKey")

2. POST request with body:

    body := `{"data": "test"}`
    resp, err := MakePOSTRequest(server.URL, "/api/resource", body, "application/json")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
    ValidateErrorResponse(t, resp, 400, "InvalidRequest")

3. Request with custom headers:

    resp, err := MakeTestRequest(server.URL, TestRequestOptions{
        Method: "GET",
        Path: "/api/resource",
        Headers: map[string]string{
            "Authorization": "Bearer token",
            "X-Custom-Header": "value",
        },
    })
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
    ValidateErrorResponse(t, resp, 403, "AccessDenied")

4. Detailed response validation:

    resp, _ := MakeGETRequest(server.URL, "/api/resource")
    result := ValidateResponseStructure(resp, ResponseValidationOptions{
        ExpectedStatusCode: 404,
        ExpectedErrorCode: "NoSuchKey",
        ExpectedMessageKeywords: []string{"not", "found"},
        MinMessageLength: 10,
        ValidateStructure: true,
    })
    if !result.IsValid {
        t.Errorf("Validation failed: %s", result.ErrorMessage)
    }

5. Combined request and validation:

    s3Err := RequestAndValidateError(t, server.URL, TestRequestOptions{
        Method: "GET",
        Path: "/api/resource",
    }, 404, "NoSuchKey")

    if s3Err.Message == "" {
        t.Error("Expected non-empty message")
    }

6. Success response validation:

    resp, err := MakeGETRequest(server.URL, "/api/resource")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
    ValidateSuccessResponse(t, resp, 200, "application/xml")

7. Batch requests for concurrent testing:

    requests := []TestRequestOptions{
        {Method: "GET", Path: "/resource1"},
        {Method: "GET", Path: "/resource2"},
        {Method: "GET", Path: "/resource3"},
    }
    results := MakeBatchRequests(server.URL, requests)
    for _, result := range results {
        if result.Error != nil {
            t.Errorf("Request %d failed: %v", result.Index, result.Error)
        }
        if result.Response.StatusCode != 200 {
            t.Errorf("Request %d returned status %d", result.Index, result.Response.StatusCode)
        }
    }

8. Request with query parameters:

    resp, err := MakeTestRequest(server.URL, TestRequestOptions{
        Method: "GET",
        Path: "/api/resource",
        QueryParams: map[string]string{
            "key1": "value1",
            "key2": "value2",
        },
    })
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

Available Helpers:

Request Builders:
- MakeTestRequest: Full-featured request builder
- MakeGETRequest: Simple GET requests
- MakePOSTRequest: POST requests with body
- MakePUTRequest: PUT requests with body
- MakeDELETERequest: Simple DELETE requests
- MakeBatchRequests: Concurrent batch requests

Response Validators:
- ValidateResponseStructure: Comprehensive structure validation
- ValidateErrorResponse: Quick error response validation
- ValidateSuccessResponse: Success response validation
- ExtractAndValidateError: Extract and validate error

Combined Helpers:
- RequestAndValidateError: Request + error validation
- RequestAndValidateSuccess: Request + success validation
*/
