// Package server provides configurable test server infrastructure for error testing.
//
// # Error Server Setup
//
// This file provides types and functions for setting up test servers with
// configurable error responses. These are used across multiple test files.
//
// Usage:
//   scenarios := []ErrorServerScenario{
//       {StatusCode: 404, ErrorCode: "NoSuchKey", Message: "Not found"},
//   }
//   server := NewConfigurableErrorServer(scenarios)
//   defer server.Close()
package server

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// ERROR SCENARIO CONFIGURATION
// =============================================================================

// ErrorServerScenario defines the configuration for an error response scenario.
type ErrorServerScenario struct {
	// StatusCode is the HTTP status code to return
	StatusCode int

	// ErrorCode is the S3 error code to include in the response
	ErrorCode string

	// Message is the error message
	Message string

	// Resource is the resource path that caused the error (optional)
	Resource string

	// Headers are additional HTTP headers to include (optional)
	Headers map[string]string

	// Delay is the artificial delay before responding (useful for testing timeouts)
	Delay time.Duration

	// BodyOverride is a custom response body (optional, overrides automatic XML generation)
	BodyOverride string

	// RequestMatcher is a function to match incoming requests (optional)
	// If nil, all requests will match this scenario
	RequestMatcher func(*http.Request) bool
}

// =============================================================================
// TEST SERVER SETUP
// =============================================================================

// ConfigurableErrorServer is a test server that returns configured error responses.
type ConfigurableErrorServer struct {
	// Server is the underlying httptest server
	Server *httptest.Server

	// URL is the base URL of the test server
	URL string

	// Scenarios is the list of error scenarios in priority order
	Scenarios []ErrorServerScenario

	// DefaultScenario is used when no other scenario matches
	DefaultScenario *ErrorServerScenario

	// RequestCount tracks the number of requests handled
	RequestCount int

	// RequestLog contains a log of all requests received
	RequestLog []LoggedRequest

	// ResponseLog contains a log of all responses sent
	ResponseLog []LoggedResponse
}

// LoggedRequest represents a logged incoming request.
type LoggedRequest struct {
	Timestamp time.Time
	Method    string
	Path      string
	Headers   http.Header
	Body      string
}

// LoggedResponse represents a logged outgoing response.
type LoggedResponse struct {
	Timestamp   time.Time
	StatusCode  int
	Headers     http.Header
	Body        string
	ScenarioIdx int // Index of the scenario that generated this response
}

// NewConfigurableErrorServer creates a new test server with configurable error responses.
//
// This helper function creates an HTTP test server that returns configured error responses
// based on the provided scenarios. It's useful for testing client error handling.
//
// Parameters:
//   - scenarios: List of error scenarios (evaluated in order, first match wins)
//
// Returns:
//   - *ConfigurableErrorServer: Configured test server
//
// Example:
//
//	scenarios := []ErrorServerScenario{
//	    {StatusCode: 404, ErrorCode: "NoSuchKey", Message: "Not found"},
//	    {StatusCode: 403, ErrorCode: "AccessDenied", Message: "Access denied"},
//	}
//	server := NewConfigurableErrorServer(scenarios)
//	defer server.Close()
//
//	// Make requests to server.URL
func NewConfigurableErrorServer(scenarios []ErrorServerScenario) *ConfigurableErrorServer {
	s := &ConfigurableErrorServer{
		Scenarios:  scenarios,
		RequestLog: make([]LoggedRequest, 0),
		ResponseLog: make([]LoggedResponse, 0),
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handleRequest(w, r)
	})

	s.Server = httptest.NewServer(handler)
	s.URL = s.Server.URL
	return s
}

// NewConfigurableErrorServerWithDefault creates a server with a default scenario.
//
// This variant creates a server that uses a default scenario when no other scenarios match.
// Useful for testing with a primary error case and fallback behavior.
//
// Parameters:
//   - scenarios: List of error scenarios (evaluated in order)
//   - defaultScenario: Fallback scenario when no other matches
//
// Returns:
//   - *ConfigurableErrorServer: Configured test server
//
// Example:
//
//	defaultScenario := ErrorServerScenario{
//	    StatusCode: 500,
//	    ErrorCode: "InternalError",
//	    Message: "Internal server error",
//	}
//	server := NewConfigurableErrorServerWithDefault(scenarios, &defaultScenario)
//	defer server.Close()
func NewConfigurableErrorServerWithDefault(scenarios []ErrorServerScenario, defaultScenario *ErrorServerScenario) *ConfigurableErrorServer {
	s := NewConfigurableErrorServer(scenarios)
	s.DefaultScenario = defaultScenario
	return s
}

// handleRequest processes an incoming request and responds with the appropriate error.
func (s *ConfigurableErrorServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	s.RequestCount++

	// Log the request
	s.logRequest(r)

	// Find matching scenario
	scenario, scenarioIdx := s.findMatchingScenario(r)

	// Apply delay if configured
	if scenario.Delay > 0 {
		time.Sleep(scenario.Delay)
	}

	// Set custom headers FIRST (before WriteHeader)
	for key, value := range scenario.Headers {
		w.Header().Set(key, value)
	}

	// Set default Content-Type if not set
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/xml")
	}

	// Set status code LAST (after headers)
	w.WriteHeader(scenario.StatusCode)

	// Write response body
	body := s.getResponseBody(scenario)
	w.Write([]byte(body))

	// Log the response
	s.logResponse(scenario, scenarioIdx, body)
}

// findMatchingScenario finds the first scenario that matches the request.
func (s *ConfigurableErrorServer) findMatchingScenario(r *http.Request) (ErrorServerScenario, int) {
	for i, scenario := range s.Scenarios {
		if scenario.RequestMatcher == nil || scenario.RequestMatcher(r) {
			return scenario, i
		}
	}

	// Use default if no match
	if s.DefaultScenario != nil {
		return *s.DefaultScenario, -1
	}

	// Fallback to 500 error
	return ErrorServerScenario{
		StatusCode: 500,
		ErrorCode:  "InternalError",
		Message:    "No matching scenario found",
	}, -1
}

// getResponseBody generates the response body for a scenario.
func (s *ConfigurableErrorServer) getResponseBody(scenario ErrorServerScenario) string {
	// Use custom body if provided
	if scenario.BodyOverride != "" {
		return scenario.BodyOverride
	}

	// Generate S3 error XML
	s3Err := S3ErrorResponse{
		Code:     scenario.ErrorCode,
		Message:  scenario.Message,
		Resource: scenario.Resource,
		RequestId: fmt.Sprintf("req-%d", s.RequestCount),
	}

	// Add additional fields from headers if they exist
	if method, ok := scenario.Headers["X-Error-Method"]; ok {
		s3Err.Method = method
	}
	if allowedMethods, ok := scenario.Headers["X-Error-Allowed-Methods"]; ok {
		s3Err.AllowedMethods = allowedMethods
	}
	if contentType, ok := scenario.Headers["X-Error-Content-Type"]; ok {
		s3Err.ContentType = contentType
	}

	xml, err := ToXMLWithError(s3Err)
	if err != nil {
		return fmt.Sprintf("<?xml version=\"1.0\"?><Error><Code>InternalError</Code><Message>Failed to generate error response: %v</Message></Error>", err)
	}

	return xml
}

// ToXMLWithError converts the S3 error response to XML format with error handling.
// This is a helper function for generating S3 XML responses.
func ToXMLWithError(e S3ErrorResponse) (string, error) {
	e.XMLName = xml.Name{Local: "Error"}
	bytes, err := xml.Marshal(e)
	if err != nil {
		return "", err
	}
	return xml.Header + string(bytes), nil
}

// logRequest logs an incoming request.
func (s *ConfigurableErrorServer) logRequest(r *http.Request) {
	bodyBytes, _ := io.ReadAll(r.Body)
	r.Body.Close()

	loggedReq := LoggedRequest{
		Timestamp: time.Now(),
		Method:    r.Method,
		Path:      r.URL.Path,
		Headers:   r.Header.Clone(),
		Body:      string(bodyBytes),
	}

	s.RequestLog = append(s.RequestLog, loggedReq)
}

// logResponse logs an outgoing response.
func (s *ConfigurableErrorServer) logResponse(scenario ErrorServerScenario, scenarioIdx int, body string) {
	loggedResp := LoggedResponse{
		Timestamp:   time.Now(),
		StatusCode:  scenario.StatusCode,
		Headers:     make(http.Header),
		Body:        body,
		ScenarioIdx: scenarioIdx,
	}

	// Copy headers
	for key, value := range scenario.Headers {
		loggedResp.Headers.Set(key, value)
	}
	loggedResp.Headers.Set("Content-Type", "application/xml")

	s.ResponseLog = append(s.ResponseLog, loggedResp)
}

// Close closes the test server.
func (s *ConfigurableErrorServer) Close() {
	s.Server.Close()
}

// Reset resets the request count and logs.
func (s *ConfigurableErrorServer) Reset() {
	s.RequestCount = 0
	s.RequestLog = make([]LoggedRequest, 0)
	s.ResponseLog = make([]LoggedResponse, 0)
}

// GetLastRequest returns the most recent request log.
func (s *ConfigurableErrorServer) GetLastRequest() *LoggedRequest {
	if len(s.RequestLog) == 0 {
		return nil
	}
	return &s.RequestLog[len(s.RequestLog)-1]
}

// GetLastResponse returns the most recent response log.
func (s *ConfigurableErrorServer) GetLastResponse() *LoggedResponse {
	if len(s.ResponseLog) == 0 {
		return nil
	}
	return &s.ResponseLog[len(s.ResponseLog)-1]
}

// =============================================================================
// REQUEST MATCHERS
// =============================================================================

// MatchByPath creates a request matcher that matches by URL path.
func MatchByPath(path string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		return r.URL.Path == path
	}
}

// MatchByPathPrefix creates a request matcher that matches by path prefix.
func MatchByPathPrefix(prefix string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		return strings.HasPrefix(r.URL.Path, prefix)
	}
}

// MatchByMethod creates a request matcher that matches by HTTP method.
func MatchByMethod(method string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		return r.Method == method
	}
}

// MatchByMethodAndPath creates a request matcher that matches both method and path.
func MatchByMethodAndPath(method, path string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		return r.Method == method && r.URL.Path == path
	}
}

// MatchByHeader creates a request matcher that matches by header value.
func MatchByHeader(header, value string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		return r.Header.Get(header) == value
	}
}

// MatchAlways creates a request matcher that always matches.
func MatchAlways() func(*http.Request) bool {
	return func(r *http.Request) bool {
		return true
	}
}

// =============================================================================
// CONVENIENCE FUNCTIONS
// =============================================================================

// NewSingleErrorServer creates a server with a single error scenario.
//
// This convenience function creates a server that always returns the same error.
// Useful for simple error testing scenarios.
//
// Parameters:
//   - statusCode: HTTP status code to return
//   - errorCode: S3 error code
//   - message: Error message
//
// Returns:
//   - *ConfigurableErrorServer: Configured test server
//
// Example:
//
//	server := NewSingleErrorServer(404, "NoSuchKey", "Resource not found")
//	defer server.Close()
//
//	resp, err := http.Get(server.URL + "/resource")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateErrorServerResponse(t, resp, 404, "NoSuchKey")
func NewSingleErrorServer(statusCode int, errorCode string, message string) *ConfigurableErrorServer {
	scenario := ErrorServerScenario{
		StatusCode: statusCode,
		ErrorCode:  errorCode,
		Message:    message,
		Resource:   "/api/resource",
		Headers: map[string]string{
			"Content-Type": "application/xml",
		},
	}

	return NewConfigurableErrorServer([]ErrorServerScenario{scenario})
}

// NewMultipleErrorServer creates a server with multiple error scenarios.
//
// This convenience function creates a server that responds with different errors
// based on request matching. Each scenario is evaluated in order.
//
// Parameters:
//   - scenarios: List of error scenarios with request matchers
//
// Returns:
//   - *ConfigurableErrorServer: Configured test server
//
// Example:
//
//	scenarios := []ErrorServerScenario{
//	    {
//	        StatusCode: 404,
//	        ErrorCode: "NoSuchKey",
//	        Message: "Not found",
//	        RequestMatcher: func(r *http.Request) bool {
//	            return strings.HasSuffix(r.URL.Path, "/missing")
//	        },
//	    },
//	    {
//	        StatusCode: 403,
//	        ErrorCode: "AccessDenied",
//	        Message: "Access denied",
//	        RequestMatcher: func(r *http.Request) bool {
//	            return strings.HasSuffix(r.URL.Path, "/forbidden")
//	        },
//	    },
//	}
//	server := NewMultipleErrorServer(scenarios)
//	defer server.Close()
func NewMultipleErrorServer(scenarios []ErrorServerScenario) *ConfigurableErrorServer {
	return NewConfigurableErrorServer(scenarios)
}

// NewDelayErrorServer creates a server that adds delay to responses.
//
// This is useful for testing timeout handling and slow response scenarios.
//
// Parameters:
//   - statusCode: HTTP status code to return
//   - errorCode: S3 error code
//   - message: Error message
//   - delay: Delay before responding
//
// Returns:
//   - *ConfigurableErrorServer: Configured test server
//
// Example:
//
//	server := NewDelayErrorServer(500, "InternalError", "Server error", 2*time.Second)
//	defer server.Close()
//
//	client := &http.Client{Timeout: 1 * time.Second}
//	resp, err := client.Get(server.URL + "/resource") // Will timeout
func NewDelayErrorServer(statusCode int, errorCode string, message string, delay time.Duration) *ConfigurableErrorServer {
	scenario := ErrorServerScenario{
		StatusCode: statusCode,
		ErrorCode:  errorCode,
		Message:    message,
		Resource:   "/api/resource",
		Delay:      delay,
		Headers: map[string]string{
			"Content-Type": "application/xml",
		},
	}

	return NewConfigurableErrorServer([]ErrorServerScenario{scenario})
}

// ValidateErrorServerResponse validates an error server response.
//
// This helper validates that a response matches expected error code and status.
// Useful for assertions in tests.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to validate
//   - expectedStatusCode: Expected HTTP status code
//   - expectedErrorCode: Expected S3 error code
//
// Example:
//
//	resp, err := http.Get(server.URL + "/resource")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateErrorServerResponse(t, resp, 404, "NoSuchKey")
func ValidateErrorServerResponse(t *testing.T, resp *http.Response, expectedStatusCode int, expectedErrorCode string) {
	t.Helper()

	// Validate status code
	if resp.StatusCode != expectedStatusCode {
		t.Errorf("Expected status code %d, got %d", expectedStatusCode, resp.StatusCode)
	}

	// Validate content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/xml" {
		t.Errorf("Expected Content-Type 'application/xml', got '%s'", contentType)
	}

	// Parse and validate S3 error
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	defer resp.Body.Close()

	var s3Err S3Error
	if err := xml.Unmarshal(body, &s3Err); err != nil {
		t.Fatalf("Failed to parse S3 error response: %v", err)
	}

	if s3Err.Code != expectedErrorCode {
		t.Errorf("Expected error code '%s', got '%s'", expectedErrorCode, s3Err.Code)
	}

	if s3Err.Message == "" {
		t.Error("Expected non-empty error message")
	}
}

// =============================================================================
// PREDEFINED ERROR SCENARIOS
// =============================================================================

// PredefinedErrorScenarios provides ready-to-use error scenarios.
//
// This collection contains common error scenarios that can be used directly
// in tests without needing to configure individual scenarios.
//
// Example:
//
//	scenario := PredefinedErrorScenarios.NotFound
//	server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
//	defer server.Close()
var PredefinedErrorScenarios = struct {
	// BadRequest returns a 400 InvalidRequest error scenario
	BadRequest ErrorServerScenario

	// Unauthorized returns a 401 Unauthorized error scenario
	Unauthorized ErrorServerScenario

	// Forbidden returns a 403 AccessDenied error scenario
	Forbidden ErrorServerScenario

	// NotFound returns a 404 NoSuchKey error scenario
	NotFound ErrorServerScenario

	// MethodNotAllowed returns a 405 MethodNotAllowed error scenario
	MethodNotAllowed ErrorServerScenario

	// UnsupportedMediaType returns a 415 UnsupportedMediaType error scenario
	UnsupportedMediaType ErrorServerScenario

	// InternalServerError returns a 500 InternalError error scenario
	InternalServerError ErrorServerScenario

	// ServiceUnavailable returns a 503 ServiceUnavailable error scenario
	ServiceUnavailable ErrorServerScenario
}{
	BadRequest: ErrorServerScenario{
		StatusCode: 400,
		ErrorCode:  ErrorCodeInvalidRequest,
		Message:    "The request is invalid or malformed",
		Resource:   "/api/resource",
		Headers: map[string]string{
			"Content-Type": "application/xml",
		},
	},
	Unauthorized: ErrorServerScenario{
		StatusCode: 401,
		ErrorCode:  "Unauthorized",
		Message:    "Authentication required",
		Resource:   "/api/resource",
		Headers: map[string]string{
			"Content-Type":    "application/xml",
			"WWW-Authenticate": "AWS4-HMAC-SHA256",
		},
	},
	Forbidden: ErrorServerScenario{
		StatusCode: 403,
		ErrorCode:  ErrorCodeAccessDenied,
		Message:    "Access Denied",
		Resource:   "/api/resource",
		Headers: map[string]string{
			"Content-Type": "application/xml",
		},
	},
	NotFound: ErrorServerScenario{
		StatusCode: 404,
		ErrorCode:  ErrorCodeNoSuchKey,
		Message:    "The specified key does not exist",
		Resource:   "/api/resource",
		Headers: map[string]string{
			"Content-Type": "application/xml",
		},
	},
	MethodNotAllowed: ErrorServerScenario{
		StatusCode: 405,
		ErrorCode:  ErrorCodeMethodNotAllowed,
		Message:    "The specified method is not allowed against this resource",
		Resource:   "/api/resource",
		Headers: map[string]string{
			"Content-Type":     "application/xml",
			"Allow":            "GET, HEAD, PUT",
			"X-Error-Method":   "POST",
			"X-Error-Allowed-Methods": "GET, HEAD, PUT",
		},
	},
	UnsupportedMediaType: ErrorServerScenario{
		StatusCode: 415,
		ErrorCode:  ErrorCodeUnsupportedMediaType,
		Message:    "The content type is not supported",
		Resource:   "/api/resource",
		Headers: map[string]string{
			"Content-Type":        "application/xml",
			"X-Error-Content-Type": "application/json",
		},
	},
	InternalServerError: ErrorServerScenario{
		StatusCode: 500,
		ErrorCode:  ErrorCodeInternalError,
		Message:    "Internal server error",
		Resource:   "/api/resource",
		Headers: map[string]string{
			"Content-Type": "application/xml",
		},
	},
	ServiceUnavailable: ErrorServerScenario{
		StatusCode: 503,
		ErrorCode:  ErrorCodeInternalError,
		Message:    "Service Unavailable",
		Resource:   "/api/resource",
		Headers: map[string]string{
			"Content-Type": "application/xml",
			"Retry-After":  "60",
		},
	},
}

