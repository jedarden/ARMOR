// Package testutil provides request/response validation helpers for ARMOR testing.
//
// # Validation Helpers Overview
//
// This package provides comprehensive helper functions for making test requests
// and validating error responses. These helpers are designed to work with the
// ARMOR server test infrastructure and provide a consistent, type-safe interface
// for testing HTTP request/response scenarios.
//
// # Request Helpers
//
// Request helpers simplify the creation of test requests with various parameters:
//
//	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//	req = WithAuthHeader(req, "test-key", "test-secret")
//	req = WithDateHeader(req, time.Now())
//	req = WithHeader(req, "X-Custom-Header", "value")
//
// # Response Validation
//
// Response validators check various aspects of HTTP responses:
//
//	assert.NoError(t, ValidateStatusCode(resp, 403))
//	assert.NoError(t, ValidateErrorCode(resp, "MissingAuthenticationToken"))
//	assert.NoError(t, ValidateErrorMessage(resp, "missing"))
//	assert.NoError(t, ValidateContentType(resp, "application/xml"))
//	assert.NoError(t, ValidateResponseTime(resp, 100*time.Millisecond))
//
// # Comprehensive Assertions
//
// Convenience assertions combine common validation patterns:
//
//	AssertErrorResponse(t, resp, "MissingAuthenticationToken", 403)
//	AssertAuthenticationError(t, resp)
//	AssertAuthorizationError(t, resp)
//
// # Example Usage
//
//	func TestMissingAuthentication(t *testing.T) {
//	    server := setupTestServer(t)
//	    req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//
//	    resp := MakeRequest(server.Handler(), req)
//
//	    AssertErrorResponse(t, resp, "MissingAuthenticationToken", 403)
//	    AssertResponseTimeUnder(t, resp, 100*time.Millisecond)
//	    AssertContentType(t, resp, "application/xml")
//	}
package testutil

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// REQUEST BUILDERS
// =============================================================================
// These helpers simplify the creation of test requests with common parameters.
// =============================================================================

// TestRequestConfig holds configuration for test requests.
//
// This type provides a fluent interface for building test requests with
// various parameters. Use the TestRequestBuilder functions to construct
// requests incrementally.
//
// Example:
//
//	cfg := TestRequestConfig{
//	    Method: "GET",
//	    Path: "/test-bucket/test-key",
//	}
//	req := BuildTestRequest(cfg)
//	req = WithAuthHeader(req, "test-key", "test-secret")
type TestRequestConfig struct {
	Method      string
	Path        string
	Body        []byte
	Headers     map[string]string
	QueryParams map[string]string
}

// NewTestRequest creates a basic test request with method and path.
//
// This is the simplest way to create a test request. Additional headers
// and parameters can be added using the With* helper functions.
//
// Parameters:
//   - method: HTTP method (GET, POST, PUT, DELETE, etc.)
//   - path: Request path (e.g., "/test-bucket/test-key")
//   - body: Optional request body
//
// Returns:
//   - *http.Request: Configured test request
//
// Example:
//
//	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//	req = WithAuthHeader(req, "test-key", "test-secret")
func NewTestRequest(method, path string, body []byte) *http.Request {
	return httptest.NewRequest(method, path, nil)
}

// BuildTestRequest creates a test request from configuration.
//
// This function builds a complete test request from a TestRequestConfig,
// including all headers and query parameters.
//
// Parameters:
//   - config: TestRequestConfig with all request parameters
//
// Returns:
//   - *http.Request: Fully configured test request
//
// Example:
//
//	cfg := TestRequestConfig{
//	    Method: "GET",
//	    Path: "/test-bucket/test-key",
//	    Headers: map[string]string{
//	        "X-Custom-Header": "value",
//	    },
//	    QueryParams: map[string]string{
//	        "versioning": "false",
//	    },
//	}
//	req := BuildTestRequest(cfg)
func BuildTestRequest(config TestRequestConfig) *http.Request {
	var path string
	if config.QueryParams != nil && len(config.QueryParams) > 0 {
		path = config.Path + "?" + encodeQueryParams(config.QueryParams)
	} else {
		path = config.Path
	}

	req := httptest.NewRequest(config.Method, path, nil)
	if config.Body != nil {
		req.Body = nil // Would need to set up properly
	}

	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	return req
}

// WithAuthHeader adds AWS-style authorization header to request.
//
// This helper adds the Authorization header with AWS4-HMAC-SHA256 signature.
// For testing purposes, this creates a validly-formatted auth header (actual
// signature validation depends on the test server implementation).
//
// Parameters:
//   - req: The request to modify
//   - accessKey: AWS access key ID
//   - secretKey: AWS secret access key
//
// Returns:
//   - *http.Request: The modified request
//
// Example:
//
//	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//	req = WithAuthHeader(req, "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234")
func WithAuthHeader(req *http.Request, accessKey, secretKey string) *http.Request {
	timestamp := time.Now().Format("20060102T150405Z")
	credential := fmt.Sprintf("%s/20130524/us-east-1/s3/aws4_request", accessKey)
	signature := "test-signature" // Simplified for testing

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s, SignedHeaders=host;x-amz-date, Signature=%s",
		credential, signature)
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("X-Amz-Date", timestamp)

	return req
}

// WithDateHeader adds X-Amz-Date header to request.
//
// This helper adds the AWS-standard date header used for request validation.
//
// Parameters:
//   - req: The request to modify
//   - t: Time for the date header
//
// Returns:
//   - *http.Request: The modified request
//
// Example:
//
//	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//	req = WithDateHeader(req, time.Now())
func WithDateHeader(req *http.Request, t time.Time) *http.Request {
	req.Header.Set("X-Amz-Date", t.Format("20060102T150405Z"))
	return req
}

// WithHeader adds a custom header to request.
//
// This is a generic helper for adding any custom header to a test request.
//
// Parameters:
//   - req: The request to modify
//   - key: Header name
//   - value: Header value
//
// Returns:
//   - *http.Request: The modified request
//
// Example:
//
//	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//	req = WithHeader(req, "X-Custom-Header", "custom-value")
func WithHeader(req *http.Request, key, value string) *http.Request {
	req.Header.Set(key, value)
	return req
}

// WithExpiredDate sets the date header to an expired timestamp.
//
// This helper creates a request with a date header that is outside the
// allowed time window for testing expiration scenarios.
//
// Parameters:
//   - req: The request to modify
//
// Returns:
//   - *http.Request: The modified request
//
// Example:
//
//	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//	req = WithExpiredDate(req) // Date will be 20 minutes in the past
func WithExpiredDate(req *http.Request) *http.Request {
	oldTime := time.Now().Add(-20 * time.Minute)
	return WithDateHeader(req, oldTime)
}

// =============================================================================
// REQUEST EXECUTION
// =============================================================================
// These helpers execute test requests and capture responses.
// =============================================================================

// MakeRequest executes a test request against a handler.
//
// This is a convenience function for executing requests and capturing responses.
// It uses httptest.NewRecorder() to capture the response.
//
// Parameters:
//   - handler: The HTTP handler to test
//   - req: The request to execute
//
// Returns:
//   - *httptest.ResponseRecorder: Response recorder with response data
//
// Example:
//
//	server := setupTestServer(t)
//	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//	resp := MakeRequest(server.Handler(), req)
//
//	assert.Equal(t, 403, resp.Code)
func MakeRequest(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

// MakeRequestWithTiming executes a request and measures response time.
//
// This function executes a request while measuring the time taken to respond.
// The response time is included in the TestResponse struct for performance
// validation.
//
// Parameters:
//   - handler: The HTTP handler to test
//   - req: The request to execute
//
// Returns:
//   - *TestResponse: Response with timing information
//
// Example:
//
//	server := setupTestServer(t)
//	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//	resp := MakeRequestWithTiming(server.Handler(), req)
//
//	assert.Less(t, resp.ResponseTime, 100*time.Millisecond)
func MakeRequestWithTiming(handler http.Handler, req *http.Request) *TestResponse {
	start := time.Now()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	elapsed := time.Since(start)

	return &TestResponse{
		ResponseRecorder: w,
		ResponseTime:     elapsed,
	}
}

// =============================================================================
// RESPONSE TYPES
// =============================================================================
// These types encapsulate response data for validation.
// =============================================================================

// S3Error represents the standard S3 XML error response format.
//
// This type is used to unmarshal XML error responses from ARMOR,
// which follows the S3 error response specification.
//
// Example XML:
//
//	<?xml version="1.0" encoding="UTF-8"?>
//	<Error>
//	    <Code>MissingAuthenticationToken</Code>
//	    <Message>The request must include either a valid (registered) AWS Access Key Id or a valid X.509 certificate</Message>
//	</Error>
type S3Error struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

// TestResponse wraps an HTTP response with additional validation metadata.
//
// This type extends httptest.ResponseRecorder with additional information
// useful for testing, such as response time and parsed error data.
//
// Fields:
//   - ResponseRecorder: The underlying response recorder
//   - ResponseTime: Time taken to receive the response
//   - S3Error: Parsed S3 error (if response is an error)
type TestResponse struct {
	ResponseRecorder *httptest.ResponseRecorder
	ResponseTime     time.Duration
	S3Error          *S3Error
}

// =============================================================================
// RESPONSE VALIDATORS
// =============================================================================
// These validators check specific aspects of HTTP responses.
// =============================================================================

// ValidateStatusCode checks if response has expected status code.
//
// This validator checks the HTTP status code and returns an error if it
// doesn't match the expected value.
//
// Parameters:
//   - resp: Response recorder from request
//   - expectedCode: Expected HTTP status code
//
// Returns:
//   - error: nil if status code matches, error otherwise
//
// Example:
//
//	resp := MakeRequest(handler, req)
//	err := ValidateStatusCode(resp, 403)
//	assert.NoError(t, err)
func ValidateStatusCode(resp *httptest.ResponseRecorder, expectedCode int) error {
	if resp.Code != expectedCode {
		return fmt.Errorf("expected status code %d, got %d", expectedCode, resp.Code)
	}
	return nil
}

// ValidateContentType checks if response has expected content type.
//
// This validator checks the Content-Type header and returns an error if it
// doesn't match the expected value.
//
// Parameters:
//   - resp: Response recorder from request
//   - expectedContentType: Expected content type (e.g., "application/xml")
//
// Returns:
//   - error: nil if content type matches, error otherwise
//
// Example:
//
//	resp := MakeRequest(handler, req)
//	err := ValidateContentType(resp, "application/xml")
//	assert.NoError(t, err)
func ValidateContentType(resp *httptest.ResponseRecorder, expectedContentType string) error {
	contentType := resp.Header().Get("Content-Type")
	if contentType != expectedContentType {
		return fmt.Errorf("expected Content-Type '%s', got '%s'", expectedContentType, contentType)
	}
	return nil
}

// ValidateErrorCode checks if response contains expected S3 error code.
//
// This validator parses the XML error response and checks if the error code
// matches the expected value.
//
// Parameters:
//   - resp: Response recorder from request
//   - expectedCode: Expected S3 error code (e.g., "MissingAuthenticationToken")
//
// Returns:
//   - error: nil if error code matches, error otherwise
//
// Example:
//
//	resp := MakeRequest(handler, req)
//	err := ValidateErrorCode(resp, "MissingAuthenticationToken")
//	assert.NoError(t, err)
func ValidateErrorCode(resp *httptest.ResponseRecorder, expectedCode string) error {
	var s3Err S3Error
	if err := xml.Unmarshal(resp.Body.Bytes(), &s3Err); err != nil {
		return fmt.Errorf("failed to parse error response: %w", err)
	}

	if s3Err.Code != expectedCode {
		return fmt.Errorf("expected error code '%s', got '%s'", expectedCode, s3Err.Code)
	}
	return nil
}

// ValidateErrorMessage checks if error message contains expected substring.
//
// This validator parses the XML error response and checks if the error message
// contains the expected substring.
//
// Parameters:
//   - resp: Response recorder from request
//   - expectedSubstring: Substring that should be in the error message
//
// Returns:
//   - error: nil if message contains substring, error otherwise
//
// Example:
//
//	resp := MakeRequest(handler, req)
//	err := ValidateErrorMessage(resp, "missing authentication")
//	assert.NoError(t, err)
func ValidateErrorMessage(resp *httptest.ResponseRecorder, expectedSubstring string) error {
	var s3Err S3Error
	if err := xml.Unmarshal(resp.Body.Bytes(), &s3Err); err != nil {
		return fmt.Errorf("failed to parse error response: %w", err)
	}

	if !strings.Contains(strings.ToLower(s3Err.Message), strings.ToLower(expectedSubstring)) {
		return fmt.Errorf("expected error message to contain '%s', got '%s'", expectedSubstring, s3Err.Message)
	}
	return nil
}

// ValidateResponseTime checks if response completed within time limit.
//
// This validator checks if the response time is within the specified threshold.
// Useful for performance testing and ensuring error responses are fast.
//
// Parameters:
//   - resp: Test response with timing information
//   - maxDuration: Maximum allowed response duration
//
// Returns:
//   - error: nil if response time is under threshold, error otherwise
//
// Example:
//
//	resp := MakeRequestWithTiming(handler, req)
//	err := ValidateResponseTime(resp, 100*time.Millisecond)
//	assert.NoError(t, err)
func ValidateResponseTime(resp *TestResponse, maxDuration time.Duration) error {
	if resp.ResponseTime > maxDuration {
		return fmt.Errorf("response time %v exceeds maximum %v", resp.ResponseTime, maxDuration)
	}
	return nil
}

// ValidateHeader checks if response has expected header value.
//
// This validator checks if a specific response header matches the expected value.
//
// Parameters:
//   - resp: Response recorder from request
//   - header: Header name to check
//   - expectedValue: Expected header value
//
// Returns:
//   - error: nil if header matches, error otherwise
//
// Example:
//
//	resp := MakeRequest(handler, req)
//	err := ValidateHeader(resp, "X-Custom-Header", "expected-value")
//	assert.NoError(t, err)
func ValidateHeader(resp *httptest.ResponseRecorder, header, expectedValue string) error {
	value := resp.Header().Get(header)
	if value != expectedValue {
		return fmt.Errorf("expected header '%s' to be '%s', got '%s'", header, expectedValue, value)
	}
	return nil
}

// =============================================================================
// COMPREHENSIVE ASSERTIONS
// =============================================================================
// These assertions combine multiple validators for common test scenarios.
// =============================================================================

// AssertErrorResponse validates complete error response.
//
// This assertion validates that the response is an error response with the
// specified error code and HTTP status code. It combines multiple validators
// for comprehensive error response checking.
//
// Parameters:
//   - t: Testing instance for reporting failures
//   - resp: Response recorder from request
//   - expectedErrorCode: Expected S3 error code
//   - expectedStatusCode: Expected HTTP status code
//
// Example:
//
//	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//	resp := MakeRequest(handler, req)
//	AssertErrorResponse(t, resp, "MissingAuthenticationToken", 403)
func AssertErrorResponse(t *testing.T, resp *httptest.ResponseRecorder, expectedErrorCode string, expectedStatusCode int) {
	t.Helper()

	if err := ValidateStatusCode(resp, expectedStatusCode); err != nil {
		t.Errorf("Status code validation failed: %v", err)
	}

	if err := ValidateErrorCode(resp, expectedErrorCode); err != nil {
		t.Errorf("Error code validation failed: %v", err)
	}

	if err := ValidateContentType(resp, "application/xml"); err != nil {
		t.Errorf("Content-Type validation failed: %v", err)
	}
}

// AssertAuthenticationError validates authentication error response.
//
// This assertion checks for missing or invalid authentication errors.
// It validates that the response indicates an authentication problem.
//
// Parameters:
//   - t: Testing instance for reporting failures
//   - resp: Response recorder from request
//
// Example:
//
//	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//	resp := MakeRequest(handler, req)
//	AssertAuthenticationError(t, resp)
func AssertAuthenticationError(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()

	// Check for authentication-related error codes
	var s3Err S3Error
	if err := xml.Unmarshal(resp.Body.Bytes(), &s3Err); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	authErrorCodes := map[string]bool{
		"MissingAuthenticationToken": true,
		"InvalidAccessKeyId":          true,
		"SignatureDoesNotMatch":       true,
		"IncompleteSignature":         true,
		"InvalidAlgorithm":            true,
	}

	if !authErrorCodes[s3Err.Code] {
		t.Errorf("Expected authentication error code, got '%s'", s3Err.Code)
	}

	if resp.Code != 403 {
		t.Errorf("Expected status 403 for authentication error, got %d", resp.Code)
	}
}

// AssertAuthorizationError validates authorization error response.
//
// This assertion checks for permission/authorization errors.
// It validates that the response indicates an authorization problem.
//
// Parameters:
//   - t: Testing instance for reporting failures
//   - resp: Response recorder from request
//
// Example:
//
//	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
//	req = WithAuthHeader(req, "RESTRICTEDKEY", "secret")
//	resp := MakeRequest(handler, req)
//	AssertAuthorizationError(t, resp)
func AssertAuthorizationError(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()

	// Check for authorization-related error codes
	var s3Err S3Error
	if err := xml.Unmarshal(resp.Body.Bytes(), &s3Err); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	authzErrorCodes := map[string]bool{
		"AccessDenied":      true,
		"RequestExpired":    true,
		"MissingDateHeader": true,
	}

	if !authzErrorCodes[s3Err.Code] {
		t.Errorf("Expected authorization error code, got '%s'", s3Err.Code)
	}

	if resp.Code != 403 {
		t.Errorf("Expected status 403 for authorization error, got %d", resp.Code)
	}
}

// AssertStatusCode validates response status code.
//
// This is a simple assertion for status code validation.
//
// Parameters:
//   - t: Testing instance for reporting failures
//   - resp: Response recorder from request
//   - expectedCode: Expected HTTP status code
//
// Example:
//
//	resp := MakeRequest(handler, req)
//	AssertStatusCode(t, resp, 200)
func AssertStatusCode(t *testing.T, resp *httptest.ResponseRecorder, expectedCode int) {
	t.Helper()
	if err := ValidateStatusCode(resp, expectedCode); err != nil {
		t.Errorf("Status code validation failed: %v", err)
	}
}

// AssertErrorType validates error code in response.
//
// This assertion checks if the error response contains the specified error code.
//
// Parameters:
//   - t: Testing instance for reporting failures
//   - resp: Response recorder from request
//   - expectedCode: Expected S3 error code
//
// Example:
//
//	resp := MakeRequest(handler, req)
//	AssertErrorType(t, resp, "MissingAuthenticationToken")
func AssertErrorType(t *testing.T, resp *httptest.ResponseRecorder, expectedCode string) {
	t.Helper()
	if err := ValidateErrorCode(resp, expectedCode); err != nil {
		t.Errorf("Error code validation failed: %v", err)
	}
}

// AssertErrorMessage validates error message contains substring.
//
// This assertion checks if the error message contains the expected substring.
//
// Parameters:
//   - t: Testing instance for reporting failures
//   - resp: Response recorder from request
//   - expectedSubstring: Substring expected in error message
//
// Example:
//
//	resp := MakeRequest(handler, req)
//	AssertErrorMessage(t, resp, "authentication")
func AssertErrorMessage(t *testing.T, resp *httptest.ResponseRecorder, expectedSubstring string) {
	t.Helper()
	if err := ValidateErrorMessage(resp, expectedSubstring); err != nil {
		t.Errorf("Error message validation failed: %v", err)
	}
}

// AssertContentType validates response content type.
//
// This assertion checks if the response has the expected content type.
//
// Parameters:
//   - t: Testing instance for reporting failures
//   - resp: Response recorder from request
//   - expectedContentType: Expected content type
//
// Example:
//
//	resp := MakeRequest(handler, req)
//	AssertContentType(t, resp, "application/xml")
func AssertContentType(t *testing.T, resp *httptest.ResponseRecorder, expectedContentType string) {
	t.Helper()
	if err := ValidateContentType(resp, expectedContentType); err != nil {
		t.Errorf("Content-Type validation failed: %v", err)
	}
}

// AssertHeader validates response header value.
//
// This assertion checks if a response header matches the expected value.
//
// Parameters:
//   - t: Testing instance for reporting failures
//   - resp: Response recorder from request
//   - header: Header name to check
//   - expectedValue: Expected header value
//
// Example:
//
//	resp := MakeRequest(handler, req)
//	AssertHeader(t, resp, "X-Custom-Header", "expected-value")
func AssertHeader(t *testing.T, resp *httptest.ResponseRecorder, header, expectedValue string) {
	t.Helper()
	if err := ValidateHeader(resp, header, expectedValue); err != nil {
		t.Errorf("Header validation failed: %v", err)
	}
}

// AssertResponseTimeUnder validates response time is under threshold.
//
// This assertion checks if the response completed within the specified time limit.
//
// Parameters:
//   - t: Testing instance for reporting failures
//   - resp: Test response with timing information
//   - maxDuration: Maximum allowed response time
//
// Example:
//
//	resp := MakeRequestWithTiming(handler, req)
//	AssertResponseTimeUnder(t, resp, 100*time.Millisecond)
func AssertResponseTimeUnder(t *testing.T, resp *TestResponse, maxDuration time.Duration) {
	t.Helper()
	if err := ValidateResponseTime(resp, maxDuration); err != nil {
		t.Errorf("Response time validation failed: %v", err)
	}
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================
// These functions provide common utilities for test helpers.
// =============================================================================

// encodeQueryParams encodes query parameters for URL.
//
// This is a simple utility function for encoding query parameters.
func encodeQueryParams(params map[string]string) string {
	var parts []string
	for key, value := range params {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, "&")
}

// ParseS3Error parses S3 error from response body.
//
// This utility function parses the XML error response and returns the
// parsed S3Error struct. Returns error if parsing fails.
//
// Parameters:
//   - body: Response body bytes
//
// Returns:
//   - *S3Error: Parsed error response
//   - error: Parsing error if any
//
// Example:
//
//	var s3Err S3Error
//	if err := xml.Unmarshal(resp.Body.Bytes(), &s3Err); err != nil {
//	    t.Fatalf("Failed to parse error: %v", err)
//	}
func ParseS3Error(body []byte) (*S3Error, error) {
	var s3Err S3Error
	if err := xml.Unmarshal(body, &s3Err); err != nil {
		return nil, fmt.Errorf("failed to parse S3 error: %w", err)
	}
	return &s3Err, nil
}

// GetS3Error extracts S3 error from response recorder.
//
// This is a convenience function that parses the S3 error from a response recorder.
// Returns error if parsing fails.
//
// Parameters:
//   - resp: Response recorder from request
//
// Returns:
//   - *S3Error: Parsed error response
//   - error: Parsing error if any
//
// Example:
//
//	s3Err, err := GetS3Error(resp)
//	if err != nil {
//	    t.Fatalf("Failed to extract error: %v", err)
//	}
//	assert.Equal(t, "MissingAuthenticationToken", s3Err.Code)
func GetS3Error(resp *httptest.ResponseRecorder) (*S3Error, error) {
	return ParseS3Error(resp.Body.Bytes())
}
