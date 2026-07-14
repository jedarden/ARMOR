package server

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// =============================================================================
// HTTP ERROR RESPONSE FIXTURES
// =============================================================================
// This file provides reusable test fixtures for common HTTP error scenarios.
// These fixtures are designed to be easily configurable and loadable for different
// endpoints and error contexts, complementing the existing test infrastructure.
//
// Fixtures vs Error Patterns:
// - Error Patterns (error_test_patterns.go): Define expected behaviors for validation
// - HTTP Fixtures (this file): Create mock responses for testing
// Use fixtures when you need to simulate server responses, use patterns when validating them.
//
// When to use HTTP fixtures:
// - Mocking server error responses in integration tests
// - Creating expected responses for validation tests
// - Testing client error handling behavior
// - Building response samples for documentation
// - Writing tests that need complete HTTP response objects
//
// When to use Error Patterns instead:
// - Defining expected test outcomes
// - Validating actual server responses
// - Table-driven test scenarios
// - Reusable validation expectations
//
// Available fixtures:
// - 404 Not Found (with customizable path/message)
// - 405 Method Not Allowed (with customizable allowed methods)
// - 415 Unsupported Media Type
// - 500 Internal Server Error (generic)
// - 400 Bad Request
// - 401 Unauthorized
// - 403 Forbidden
// - 409 Conflict
//
// Usage:
//   // Create a fixture for a missing resource
//   fixture := NotFoundFixture("/api/blobs/missing-file.txt", "", "")
//   response := fixture.ToXMLResponse()
//   assert.Equal(t, 404, response.StatusCode)
//
//   // Use with error patterns for validation
//   pattern := CommonErrorPatterns.ResourceNotFound
//   assert.Equal(t, pattern.ExpectedStatus, response.StatusCode)
//
// Bead: bf-7d2vgf
// Created: 2026-07-14
// =============================================================================

// HTTPErrorFixture represents a configurable HTTP error response fixture.
//
// Provides a complete error response structure with HTTP status, content type,
// and error body that can be customized per endpoint.
type HTTPErrorFixture struct {
	// StatusCode is the HTTP status code for this error
	StatusCode int

	// ErrorCode is the S3 error code (e.g., "NoSuchKey", "MethodNotAllowed")
	ErrorCode string

	// Message is the human-readable error message
	Message string

	// Resource is the optional resource path that caused the error
	Resource string

	// ContentType is the response content type (default: "application/xml")
	ContentType string

	// AdditionalFields contains optional additional XML fields
	AdditionalFields map[string]string

	// Headers contains additional HTTP headers
	Headers map[string]string
}

// S3ErrorResponse represents a complete S3 XML error response structure.
type S3ErrorResponse struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`

	// Optional fields
	Resource       string `xml:"Resource,omitempty"`
	RequestId      string `xml:"RequestId,omitempty"`
	AllowedMethods string `xml:"AllowedMethods,omitempty"`
	Method         string `xml:"Method,omitempty"`
	ContentType    string `xml:"ContentType,omitempty"`
}

// ToXML converts the fixture to S3 XML format.
//
// Use this method when:
//   - Serializing fixtures for XML-based tests
//   - Creating XML response bodies for mocking
//   - Generating XML samples for documentation
//   - Validating fixture XML structure
func (f *HTTPErrorFixture) ToXML() string {
	response := S3ErrorResponse{
		Code:    f.ErrorCode,
		Message: f.Message,
	}

	if f.Resource != "" {
		response.Resource = f.Resource
	}

	if allowedMethods, ok := f.AdditionalFields["AllowedMethods"]; ok {
		response.AllowedMethods = allowedMethods
	}

	if method, ok := f.AdditionalFields["Method"]; ok {
		response.Method = method
	}

	if contentType, ok := f.AdditionalFields["ContentType"]; ok {
		response.ContentType = contentType
	}

	if requestId, ok := f.AdditionalFields["RequestId"]; ok {
		response.RequestId = requestId
	}

	output, err := xml.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Error>\n  <Code>%s</Code>\n  <Message>%s</Message>\n</Error>", f.ErrorCode, f.Message)
	}

	return fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n%s", string(output))
}

// ToXMLResponse creates an HTTP response with the fixture as XML body.
//
// Use this method when:
//   - Creating complete HTTP responses for testing
//   - Building mock responses for HTTP clients
//   - Testing response validation logic
//   - Creating response samples for integration tests
func (f *HTTPErrorFixture) ToXMLResponse() *http.Response {
	body := f.ToXML()

	headers := make(map[string]string)
	headers["Content-Type"] = f.ContentType
	for k, v := range f.Headers {
		headers[k] = v
	}

	return &http.Response{
		StatusCode: f.StatusCode,
		Header:     toHTTPHeader(headers),
		Body:       toResponseBody(body),
	}
}

// ToS3Error converts the fixture to an S3Error struct.
//
// Use this method when:
//   - Converting fixtures to S3Error for validation
//   - Comparing fixtures against actual error responses
//   - Using fixtures with pattern-based validation
//   - Creating S3Error objects from fixtures
func (f *HTTPErrorFixture) ToS3Error() *S3Error {
	return &S3Error{
		Code:    f.ErrorCode,
		Message: f.Message,
	}
}

// WithResource returns a new fixture with the resource path set.
//
// Use this method when:
//   - Customizing a predefined fixture with a specific path
//   - Creating fixtures for multiple resources programmatically
//   - Building fixtures from configuration data
//   - Modifying fixture resource without changing other fields
func (f *HTTPErrorFixture) WithResource(resource string) *HTTPErrorFixture {
	newFixture := *f
	newFixture.Resource = resource
	return &newFixture
}

// WithMessage returns a new fixture with the message set.
//
// Use this method when:
//   - Customizing error messages for specific scenarios
//   - Localizing error messages for different languages
//   - Adding context-specific information to messages
//   - Creating variations of base fixtures
func (f *HTTPErrorFixture) WithMessage(message string) *HTTPErrorFixture {
	newFixture := *f
	newFixture.Message = message
	return &newFixture
}

// WithRequestId returns a new fixture with a request ID set.
//
// Use this method when:
//   - Adding request tracing to error responses
//   - Testing request ID propagation in error scenarios
//   - Simulating production error responses with tracking
//   - Debugging error handling flow through systems
func (f *HTTPErrorFixture) WithRequestId(requestId string) *HTTPErrorFixture {
	newFixture := *f
	// Deep copy AdditionalFields to avoid modifying the original
	newFixture.AdditionalFields = make(map[string]string)
	for k, v := range f.AdditionalFields {
		newFixture.AdditionalFields[k] = v
	}
	newFixture.AdditionalFields["RequestId"] = requestId
	return &newFixture
}

// =============================================================================
// FACTORY FUNCTIONS FOR COMMON ERRORS
// =============================================================================

// NotFoundFixture creates a 404 Not Found error fixture.
//
// Use this fixture when:
//   - Testing client behavior for missing resources
//   - Simulating 404 responses in integration tests
//   - Verifying error handling for non-existent objects
//   - Creating mock responses for GET/HEAD requests on missing resources
//
// Args:
//   path: The resource path that was not found
//   message: Optional custom error message (default: auto-generated)
//   errorCode: Optional error code (default: "NoSuchKey")
//
// Returns:
//   *HTTPErrorFixture configured for 404 Not Found
//
// Example:
//   fixture := NotFoundFixture("/api/blobs/missing-file.txt", "", "")
//   response := fixture.ToXMLResponse()
//   assert.Equal(t, 404, response.StatusCode)
//
//   // Custom message
//   fixture := NotFoundFixture("/custom/path", "Resource does not exist", "")
func NotFoundFixture(path string, message string, errorCode string) *HTTPErrorFixture {
	if message == "" {
		message = fmt.Sprintf("The specified resource does not exist: %s", path)
	}
	if errorCode == "" {
		errorCode = "NoSuchKey"
	}

	return &HTTPErrorFixture{
		StatusCode:  404,
		ErrorCode:   errorCode,
		Message:     message,
		Resource:    path,
		ContentType: "application/xml",
	}
}

// MethodNotAllowedFixture creates a 405 Method Not Allowed error fixture.
//
// Use this fixture when:
//   - Testing HTTP method restrictions on endpoints
//   - Simulating 405 responses for unsupported methods (e.g., POST on blob endpoints)
//   - Verifying Allow header is correctly set
//   - Testing client behavior when using wrong HTTP method
//
// Args:
//   path: The resource path that was accessed
//   allowedMethods: List of allowed HTTP methods (e.g., "GET, HEAD")
//   method: The method that was attempted (default: "DELETE")
//   message: Optional custom error message (default: auto-generated)
//
// Returns:
//   *HTTPErrorFixture configured for 405 Method Not Allowed
//
// Example:
//   fixture := MethodNotAllowedFixture(
//       "/api/blobs/file.txt",
//       "GET, HEAD",
//       "DELETE",
//       ""
//   )
func MethodNotAllowedFixture(path string, allowedMethods string, method string, message string) *HTTPErrorFixture {
	if method == "" {
		method = "DELETE"
	}
	if message == "" {
		message = fmt.Sprintf("The %s method is not allowed for this resource. Allowed methods: %s", method, allowedMethods)
	}

	return &HTTPErrorFixture{
		StatusCode:  405,
		ErrorCode:   "MethodNotAllowed",
		Message:     message,
		Resource:    path,
		ContentType: "application/xml",
		AdditionalFields: map[string]string{
			"AllowedMethods": allowedMethods,
			"Method":         method,
		},
		Headers: map[string]string{
			"Allow": allowedMethods,
		},
	}
}

// UnsupportedMediaTypeFixture creates a 415 Unsupported Media Type error fixture.
//
// Use this fixture when:
//   - Testing content-type validation on endpoints
//   - Simulating 415 responses for unsupported Content-Type headers
//   - Verifying clients send correct content types
//   - Testing API contract compliance for media types
//
// Args:
//   path: The resource path that was accessed
//   contentType: The unsupported content type that was sent
//   supportedTypes: Optional list of supported content types
//   message: Optional custom error message (default: auto-generated)
//
// Returns:
//   *HTTPErrorFixture configured for 415 Unsupported Media Type
//
// Example:
//   fixture := UnsupportedMediaTypeFixture(
//       "/api/blobs/file.txt",
//       "application/json",
//       "application/xml, text/plain",
//       ""
//   )
func UnsupportedMediaTypeFixture(path string, contentType string, supportedTypes string, message string) *HTTPErrorFixture {
	if message == "" {
		if supportedTypes != "" {
			message = fmt.Sprintf("Unsupported media type '%s'. Supported types: %s", contentType, supportedTypes)
		} else {
			message = fmt.Sprintf("Unsupported media type '%s'", contentType)
		}
	}

	additionalFields := map[string]string{
		"ContentType": contentType,
	}
	if supportedTypes != "" {
		additionalFields["SupportedTypes"] = supportedTypes
	}

	return &HTTPErrorFixture{
		StatusCode:  415,
		ErrorCode:   "UnsupportedMediaType",
		Message:     message,
		Resource:    path,
		ContentType: "application/xml",
		AdditionalFields: additionalFields,
	}
}

// InternalServerErrorFixture creates a 500 Internal Server Error fixture.
//
// Use this fixture when:
//   - Testing client behavior for server failures
//   - Simulating 500 responses for unexpected server errors
//   - Testing retry logic and circuit breakers
//   - Verifying error handling for infrastructure failures
//
// Args:
//   path: Optional resource path that caused the error
//   message: Error message (default: generic internal error message)
//   errorCode: Error code type (default: "InternalError")
//   requestId: Optional request ID for tracking
//
// Returns:
//   *HTTPErrorFixture configured for 500 Internal Server Error
//
// Example:
//   fixture := InternalServerErrorFixture("", "", "", "")
//
//   // With path and request ID
//   fixture := InternalServerErrorFixture(
//       "/api/blobs/file.txt",
//       "",
//       "",
//       "req-12345"
//   )
func InternalServerErrorFixture(path string, message string, errorCode string, requestId string) *HTTPErrorFixture {
	if message == "" {
		message = "An internal server error occurred. Please try again later."
	}
	if errorCode == "" {
		errorCode = "InternalError"
	}

	additionalFields := make(map[string]string)
	if requestId != "" {
		additionalFields["RequestId"] = requestId
	}

	return &HTTPErrorFixture{
		StatusCode:  500,
		ErrorCode:   errorCode,
		Message:     message,
		Resource:    path,
		ContentType: "application/xml",
		AdditionalFields: additionalFields,
	}
}

// BadRequestFixture creates a 400 Bad Request error fixture.
//
// Use this fixture when:
//   - Testing client behavior for malformed requests
//   - Simulating 400 responses for invalid input
//   - Verifying request validation logic
//   - Testing error handling for missing/invalid parameters
//
// Args:
//   path: The resource path that was accessed
//   message: Error message (default: generic bad request message)
//   errorCode: Error code type (default: "BadRequest")
//
// Returns:
//   *HTTPErrorFixture configured for 400 Bad Request
func BadRequestFixture(path string, message string, errorCode string) *HTTPErrorFixture {
	if message == "" {
		message = "The request could not be understood or was missing required parameters."
	}
	if errorCode == "" {
		errorCode = "BadRequest"
	}

	return &HTTPErrorFixture{
		StatusCode:  400,
		ErrorCode:   errorCode,
		Message:     message,
		Resource:    path,
		ContentType: "application/xml",
	}
}

// UnauthorizedFixture creates a 401 Unauthorized error fixture.
//
// Use this fixture when:
//   - Testing authentication requirements
//   - Simulating 401 responses for missing credentials
//   - Verifying auth enforcement on protected endpoints
//   - Testing client authentication flow
//
// Args:
//   path: The resource path that was accessed
//   message: Error message (default: generic unauthorized message)
//   errorCode: Error code type (default: "Unauthorized")
//
// Returns:
//   *HTTPErrorFixture configured for 401 Unauthorized
func UnauthorizedFixture(path string, message string, errorCode string) *HTTPErrorFixture {
	if message == "" {
		message = "Authentication is required to access this resource."
	}
	if errorCode == "" {
		errorCode = "Unauthorized"
	}

	return &HTTPErrorFixture{
		StatusCode:  401,
		ErrorCode:   errorCode,
		Message:     message,
		Resource:    path,
		ContentType: "application/xml",
	}
}

// ForbiddenFixture creates a 403 Forbidden error fixture.
//
// Use this fixture when:
//   - Testing authorization and permission checks
//   - Simulating 403 responses for access denied scenarios
//   - Verifying ACL/rule enforcement on resources
//   - Testing client behavior for insufficient permissions
//
// Args:
//   path: The resource path that was accessed
//   message: Error message (default: generic forbidden message)
//   errorCode: Error code type (default: "AccessDenied")
//
// Returns:
//   *HTTPErrorFixture configured for 403 Forbidden
func ForbiddenFixture(path string, message string, errorCode string) *HTTPErrorFixture {
	if message == "" {
		message = "You do not have permission to access this resource."
	}
	if errorCode == "" {
		errorCode = "AccessDenied"
	}

	return &HTTPErrorFixture{
		StatusCode:  403,
		ErrorCode:   errorCode,
		Message:     message,
		Resource:    path,
		ContentType: "application/xml",
	}
}

// ConflictFixture creates a 409 Conflict error fixture.
//
// Use this fixture when:
//   - Testing concurrent modification scenarios
//   - Simulating 409 responses for state conflicts
//   - Verifying optimistic locking behavior
//   - Testing client retry logic for conflicts
//
// Args:
//   path: The resource path that was accessed
//   message: Error message (default: generic conflict message)
//   errorCode: Error code type (default: "Conflict")
//
// Returns:
//   *HTTPErrorFixture configured for 409 Conflict
func ConflictFixture(path string, message string, errorCode string) *HTTPErrorFixture {
	if message == "" {
		message = "The request could not be completed due to a conflict."
	}
	if errorCode == "" {
		errorCode = "Conflict"
	}

	return &HTTPErrorFixture{
		StatusCode:  409,
		ErrorCode:   errorCode,
		Message:     message,
		Resource:    path,
		ContentType: "application/xml",
	}
}

// =============================================================================
// PREDEFINED FIXTURE INSTANCES
// =============================================================================

// Common 404 scenarios
var (
	BlobNotFound = NotFoundFixture(
		"/api/blobs/missing-blob.txt",
		"",
		"NoSuchKey",
	)

	ManifestNotFound = NotFoundFixture(
		"/armor/manifest/current/manifest.json",
		"",
		"NoSuchKey",
	)
)

// Common 405 scenarios
var (
	ReadOnlyResource = MethodNotAllowedFixture(
		"/api/blobs/readonly-file.txt",
		"GET, HEAD, OPTIONS",
		"PUT",
		"",
	)

	GetOnlyResource = MethodNotAllowedFixture(
		"/api/blobs/get-only.txt",
		"GET",
		"DELETE",
		"",
	)
)

// Common 415 scenarios
var (
	JSONNotSupported = UnsupportedMediaTypeFixture(
		"/api/blobs/file.txt",
		"application/json",
		"application/xml, text/plain",
		"",
	)

	BinaryNotSupported = UnsupportedMediaTypeFixture(
		"/api/blobs/file.txt",
		"application/octet-stream",
		"text/plain",
		"",
	)
)

// Common 500 scenarios
var (
	GenericServerError = InternalServerErrorFixture(
		"",
		"",
		"",
		"",
	)

	DatabaseError = InternalServerErrorFixture(
		"",
		"Database connection failed",
		"DatabaseError",
		"",
	)

	ServerErrorWithRequestId = InternalServerErrorFixture(
		"/api/blobs/file.txt",
		"",
		"",
		fmt.Sprintf("req-%d", time.Now().Unix()),
	)
)

// Common 403 scenarios
var (
	AccessDenied = ForbiddenFixture(
		".armor/internal/resource.json",
		"Access to the .armor/ namespace is denied",
		"AccessDenied",
	)

	AuthenticationRequired = UnauthorizedFixture(
		"/api/protected/resource",
		"Valid authentication credentials required",
		"",
	)
)

// =============================================================================
// FIXTURE RETRIEVAL FUNCTIONS
// =============================================================================

// GetFixtureByStatusCode returns a fixture for the given HTTP status code.
//
// Use this helper when:
//   - Dynamically creating fixtures based on status codes
//   - Writing tests that iterate over multiple error codes
//   - Building generic error handling tests
//   - Creating fixtures without knowing the specific error type
//
// Args:
//   statusCode: HTTP status code
//   path: Optional resource path
//
// Returns:
//   *HTTPErrorFixture for the given status code
//
// Raises:
//   panic if status code is not supported
//
// Example:
//   fixture := GetFixtureByStatusCode(404, "/api/file.txt")
//   fixture := GetFixtureByStatusCode(405, "/api/file.txt")
func GetFixtureByStatusCode(statusCode int, path string) *HTTPErrorFixture {
	switch statusCode {
	case 400:
		return BadRequestFixture(path, "", "")
	case 401:
		return UnauthorizedFixture(path, "", "")
	case 403:
		return ForbiddenFixture(path, "", "")
	case 404:
		return NotFoundFixture(path, "", "")
	case 405:
		return MethodNotAllowedFixture(path, "GET, HEAD", "", "")
	case 415:
		return UnsupportedMediaTypeFixture(path, "application/json", "", "")
	case 500:
		return InternalServerErrorFixture(path, "", "", "")
	case 409:
		return ConflictFixture(path, "", "")
	default:
		panic(fmt.Sprintf("no fixture factory for status code %d", statusCode))
	}
}

// =============================================================================
// BATCH FIXTURE CREATION
// =============================================================================

// CreateFixtureBatch creates multiple fixtures for different status codes.
//
// Use this helper when:
//   - Testing multiple error scenarios in batch
//   - Building comprehensive error test suites
//   - Creating fixtures for table-driven tests
//   - Generating fixtures for multiple endpoints at once
//
// Args:
//   statusCodes: List of HTTP status codes to create fixtures for
//   basePath: Base path for all fixtures
//
// Returns:
//   []*HTTPErrorFixture for the given status codes
//
// Example:
//   fixtures := CreateFixtureBatch([]int{400, 404, 500}, "/api/blobs/file.txt")
func CreateFixtureBatch(statusCodes []int, basePath string) []*HTTPErrorFixture {
	fixtures := make([]*HTTPErrorFixture, 0, len(statusCodes))

	for _, code := range statusCodes {
		switch code {
		case 400, 401, 403, 404, 405, 415, 500, 409:
			fixture := GetFixtureByStatusCode(code, basePath)
			fixtures = append(fixtures, fixture)
		default:
			// Skip unsupported status codes
			continue
		}
	}

	return fixtures
}

// =============================================================================
// VALIDATION HELPERS FOR FIXTURES
// =============================================================================

// ValidateFixture validates that a fixture is properly configured.
//
// Use this helper when:
//   - Verifying custom fixture configurations
//   - Pre-validating fixtures before use in tests
//   - Debugging fixture configuration issues
//   - Ensuring fixture integrity in test setup
//
// Checks that:
// - Status code is valid HTTP status (100-599)
// - Error code is non-empty
// - Message is non-empty and meets minimum length (10 chars)
// - Content type is set
//
// Returns:
//   error if validation fails, nil otherwise
//
// Example:
//   fixture := NotFoundFixture("/path", "", "")
//   if err := ValidateFixture(fixture); err != nil {
//       t.Fatalf("Invalid fixture: %v", err)
//   }
func ValidateFixture(fixture *HTTPErrorFixture) error {
	if fixture.StatusCode < 100 || fixture.StatusCode >= 600 {
		return fmt.Errorf("invalid status code: %d", fixture.StatusCode)
	}

	if fixture.ErrorCode == "" {
		return fmt.Errorf("error code cannot be empty")
	}

	if strings.TrimSpace(fixture.Message) == "" {
		return fmt.Errorf("message cannot be empty")
	}

	if len(fixture.Message) < 10 {
		return fmt.Errorf("message too short: %d chars (want at least 10)", len(fixture.Message))
	}

	if fixture.ContentType == "" {
		return fmt.Errorf("content type cannot be empty")
	}

	return nil
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// toHTTPHeader converts a map[string]string to http.Header.
func toHTTPHeader(headers map[string]string) http.Header {
	h := make(http.Header)
	for k, v := range headers {
		h.Set(k, v)
	}
	return h
}

// toResponseBody converts a string to an io.ReadCloser.
func toResponseBody(body string) io.ReadCloser {
	return &stringReadCloser{strings.NewReader(body)}
}

// stringReadCloser wraps a strings.Reader to provide Close().
type stringReadCloser struct {
	*strings.Reader
}

// Close closes the string reader (no-op for strings).
func (s *stringReadCloser) Close() error {
	return nil
}
