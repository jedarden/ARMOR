// Package server provides comprehensive error testing infrastructure for ARMOR's S3-compatible API.
//
// # Error Testing Framework Overview
//
// This package implements a complete framework for testing HTTP error responses, including:
//
// 1. Error Pattern Definitions (error_test_patterns.go):
//   - Predefined error scenarios for common S3 errors (404, 403, 500, etc.)
//   - Type-safe error structures (S3Error, ErrorScenarioConfig)
//   - Error categorization by type (Auth, NotFound, Internal, etc.)
//   - Mapping functions for error codes to patterns
//
// 2. HTTP Error Fixtures (http_error_fixtures.go):
//   - Reusable HTTP error response fixtures
//   - Configurable factory functions for common error codes
//   - XML serialization for S3-compatible error responses
//
// 3. Test Infrastructure (error_test_infrastructure_test.go):
//   - Test server and credential fixtures
//   - Reusable test helpers for error validation
//   - Setup/teardown utilities
//
// 4. Status Code Validation (error_status_validation.go):
//   - Comprehensive status code validation helpers
//   - Flexible assertion modes (boolean vs detailed error)
//   - Multi-code validation support
//
// 5. Test Pattern Tables (error_test_patterns_base_test.go):
//   - Table-driven test structures for consistency
//   - Organized test cases by error category
//   - Extensible patterns for custom scenarios
//
// # Quick Start
//
// Use a predefined pattern:
//
//	pattern := CommonErrorPatterns.ResourceNotFound
//	fmt.Printf("Expected status: %d\n", pattern.ExpectedStatus) // 404
//
// Get a pattern by error code:
//
//	pattern := PatternForCode("NoSuchKey")
//	fmt.Printf("Pattern: %s\n", pattern.Name)
//
// Get all patterns for a category:
//
//	authPatterns := PatternsForCategory(CategoryAuth)
//	for _, p := range authPatterns {
//	    fmt.Printf("- %s\n", p.Name)
//	}
//
// Create a test fixture:
//
//	fixture := NotFoundFixture("/api/blobs/missing-file.txt")
//	response := fixture.ToXMLResponse()
//
// # Error Pattern Categories
//
//   - CommonErrorPatterns: 8 patterns covering the most common S3 error scenarios
//   - AuthErrorPatterns: 6 patterns specifically for authentication failures
//   - ClientErrorPatterns: 4 patterns for 4xx client errors
//   - ServerErrorPatterns: 2 patterns for 5xx server errors
//   - ErrorPatternByCategory: Map-based access to patterns by category
//
// # Type Safety
//
// All error structures use consistent, well-documented types:
//   - S3Error: Core S3 error response structure
//   - ErrorScenarioConfig: Test scenario configuration
//   - ErrorResponseFixture: Complete response fixture with metadata
//   - HTTPErrorFixture: HTTP-specific error fixture
//   - ErrorResponseMetadata: Response metadata for logging/debugging
//
// # Testing Best Practices
//
// 1. Use predefined patterns when possible for consistency
// 2. Extend patterns with custom configurations rather than creating from scratch
// 3. Use category-based pattern access for organized testing
// 4. Leverage table-driven tests for comprehensive coverage
// 5. Document custom patterns with clear descriptions
//
// # Related Files
//
//   - error_pattern_usage_example.go: Comprehensive usage examples
//   - error_test_patterns_test.go: Tests for the pattern infrastructure
//   - error_structure_validation_test.go: Error structure validation tests
//   - error_status_validation_test.go: Status code validation tests
//   - error_response_verification_test.go: Response verification tests
package server

import (
	"encoding/xml"
	"io"
	"net/http"
	"time"
)

// =============================================================================
// ERROR TEST PATTERNS - BASE TYPES
// =============================================================================
// This file provides foundational types and structures for error response testing.
// It defines the common error structures that are used across both test and
// non-test code, ensuring type consistency and reusability.
//
// Design Philosophy:
// - Separation of concerns: base types in this file, test-specific helpers in test files
// - Type consistency: shared types across test and non-test code
// - Reusability: fixtures and patterns can be used anywhere
// - Self-documenting: clear field names and comprehensive documentation
//
// Usage:
//   - Test files import this package for base error structures
//   - Non-test files (fixtures) use S3Error for type-safe error responses
//   - Test patterns build on these base types
// =============================================================================

// =============================================================================
// CORE ERROR TYPES
// =============================================================================

// S3Error represents an S3 XML error response.
//
// This is the core type used across error testing, fixtures, and validation.
// It matches the standard S3 error response format with Code and Message fields.
//
// Use this type when:
//   - Parsing S3 XML error responses
//   - Creating test fixtures that simulate S3 errors
//   - Validating error response structure
//   - Serializing errors to XML format
//
// Example (parsing):
//
//	var s3Err S3Error
//	err := xml.Unmarshal(responseBody, &s3Err)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Error code: %s, message: %s\n", s3Err.Code, s3Err.Message)
//
// Example (creating):
//
//	s3Err := S3Error{
//	    Code: "NoSuchKey",
//	    Message: "The specified key does not exist",
//	}
//	xmlBytes, _ := xml.Marshal(s3Err)
//
// Fields:
//   - XMLName: XML element name (set to "Error" automatically)
//   - Code: S3 error code (e.g., "NoSuchKey", "AccessDenied")
//   - Message: Human-readable error message
type S3Error struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

// =============================================================================
// ERROR SCENARIO TYPES (for non-test fixture usage)
// =============================================================================

// ErrorScenarioConfig represents configuration for an error test scenario.
//
// This type is the primary configuration structure for defining expected error
// behaviors in tests. It is used across test fixtures, validation helpers, and
// predefined error patterns.
//
// Use this type when:
//   - Defining expected error behaviors for test scenarios
//   - Configuring validation rules for error responses
//   - Creating custom error patterns based on predefined ones
//   - Extending existing patterns with custom expectations
//
// Example (using a predefined pattern):
//
//	pattern := CommonErrorPatterns.ResourceNotFound
//	fmt.Printf("Expected status: %d\n", pattern.ExpectedStatus) // 404
//	fmt.Printf("Expected code: %s\n", pattern.ExpectedCode) // "NoSuchKey"
//
// Example (creating a custom pattern):
//
//	customPattern := ErrorScenarioConfig{
//	    Name: "Custom Not Found",
//	    ExpectedCode: "CustomNotFound",
//	    ExpectedStatus: 404,
//	    ExpectedMessage: "Custom not found message",
//	    ExpectedKeywords: []string{"custom", "not", "found"},
//	    MinMessageLength: 15,
//	    MaxResponseTime: 500 * time.Millisecond,
//	    Description: "Custom not found scenario",
//	    Category: string(CategoryNotFound),
//	}
//
// Example (extending a predefined pattern):
//
//	basePattern := CommonErrorPatterns.ResourceNotFound
//	customPattern := basePattern
//	customPattern.ExpectedMessage = "Custom message"
//	customPattern.MaxResponseTime = 1000 * time.Millisecond
//
// Fields:
//   - Name: Scenario name for identification and test reporting
//   - ExpectedCode: S3 error code (e.g., "NoSuchKey", "AccessDenied")
//   - ExpectedStatus: HTTP status code (e.g., 404, 403, 500)
//   - ExpectedMessage: Text that should appear in the error message
//   - ExpectedKeywords: Alternative keywords (any match is acceptable)
//   - MinMessageLength: Minimum acceptable message length
//   - MaxResponseTime: Maximum acceptable response duration
//   - Description: Explains what this scenario tests
//   - Category: Error type category (e.g., "NotFound", "Auth", "CORS")
//
// Default values:
//   - MinMessageLength: 15 (via DefaultErrorScenarioConfig())
//   - Category: "General" (via DefaultErrorScenarioConfig())
//   - MaxResponseTime: Varies by pattern (typically 200-1000ms)
type ErrorScenarioConfig struct {
	// Name is the scenario name for identification
	Name string

	// ExpectedCode is the expected S3 error code (e.g., "NoSuchKey", "AccessDenied")
	ExpectedCode string

	// ExpectedStatus is the expected HTTP status code
	ExpectedStatus int

	// ExpectedMessage contains text that should appear in the error message
	ExpectedMessage string

	// ExpectedKeywords are alternative keywords (any match is acceptable)
	ExpectedKeywords []string

	// MinMessageLength is the minimum acceptable message length
	MinMessageLength int

	// MaxResponseTime is the maximum acceptable response duration
	MaxResponseTime time.Duration

	// Description explains what this scenario tests
	Description string

	// Category categorizes the error type (e.g., "NotFound", "Auth", "CORS")
	Category string
}

// DefaultErrorScenarioConfig returns a default scenario configuration.
func DefaultErrorScenarioConfig() ErrorScenarioConfig {
	return ErrorScenarioConfig{
		MinMessageLength: 15,
		Category:         "General",
	}
}

// =============================================================================
// ERROR RESPONSE VALIDATION TYPES
// =============================================================================

// ErrorResponseMetadata contains metadata about an error response.
//
// This type is used for logging, debugging, and performance monitoring. It captures
// all relevant information about an error response without storing the full body.
//
// Use this type when:
//   - Logging error responses for debugging
//   - Monitoring response times and performance
//   - Tracking error patterns over time
//   - Collecting metrics for error analysis
//
// Example (using ExtractMetadata):
//
//	start := time.Now()
//	resp, err := http.Get("http://example.com/api/resource")
//	duration := time.Since(start)
//
//	metadata := ExtractMetadata(resp, duration)
//	log.Printf("Error: %s (%d) in %v\n", metadata.ErrorCode, metadata.StatusCode, metadata.ResponseTime)
//
// Fields:
//   - StatusCode: HTTP status code received
//   - ErrorCode: S3 error code parsed from the response
//   - Message: Error message text
//   - ResponseTime: How long the request took
//   - ContentLength: Size of the response body
//   - ContentType: Response content type header
//   - Timestamp: When the response was received
type ErrorResponseMetadata struct {
	// StatusCode is the HTTP status code received
	StatusCode int

	// ErrorCode is the S3 error code parsed from the response
	ErrorCode string

	// Message is the error message text
	Message string

	// ResponseTime is how long the request took
	ResponseTime time.Duration

	// ContentLength is the size of the response body
	ContentLength int64

	// ContentType is the response content type header
	ContentType string

	// Timestamp when the response was received
	Timestamp time.Time
}

// ExtractMetadata extracts metadata from an HTTP response.
//
// This helper is useful for logging and debugging error responses. It parses
// the S3 error from the response body and returns comprehensive metadata.
//
// Parameters:
//   - resp: HTTP response to extract metadata from
//   - duration: Time taken for the request
//
// Returns:
//   - ErrorResponseMetadata containing all extracted information
//
// Use this helper when:
//   - Logging error responses for later analysis
//   - Debugging unexpected error responses
//   - Collecting performance metrics
//   - Tracking error patterns in production
//
// Note: This function consumes the response body.
func ExtractMetadata(resp *http.Response, duration time.Duration) ErrorResponseMetadata {
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	var s3Err S3Error
	xml.Unmarshal(body, &s3Err)

	return ErrorResponseMetadata{
		StatusCode:   resp.StatusCode,
		ErrorCode:    s3Err.Code,
		Message:      s3Err.Message,
		ResponseTime: duration,
		ContentLength: resp.ContentLength,
		ContentType:  resp.Header.Get("Content-Type"),
		Timestamp:    time.Now(),
	}
}

// =============================================================================
// ERROR FIXTURE TYPES
// =============================================================================

// ErrorResponseFixture represents a complete error response fixture.
//
// This type combines the S3 error with HTTP response metadata for comprehensive
// testing scenarios. It is a complete representation of an error response that
// can be used for validation, comparison, and re-creation in tests.
//
// Use this type when:
//   - Creating complete error response fixtures for testing
//   - Comparing actual responses against expected fixtures
//   - Serializing and re-creating error responses in tests
//   - Capturing full response state for debugging
//
// Example (using ToFixture):
//
//	start := time.Now()
//	resp, err := http.Get("http://example.com/api/resource")
//	duration := time.Since(start)
//
//	fixture, err := ToFixture(resp, duration)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Fixture: %s (%d)\n", fixture.S3Error.Code, fixture.HTTPStatus)
//
// Example (manual creation):
//
//	fixture := &ErrorResponseFixture{
//	    S3Error: S3Error{Code: "NoSuchKey", Message: "Not found"},
//	    HTTPStatus: 404,
//	    Headers: http.Header{"Content-Type": []string{"application/xml"}},
//	    Body: []byte("<?xml version=\"1.0\"?><Error><Code>NoSuchKey</Code></Error>"),
//	    Duration: 150 * time.Millisecond,
//	}
//
// Fields:
//   - S3Error: Parsed S3 error structure
//   - HTTPStatus: HTTP status code
//   - Headers: HTTP response headers
//   - Body: Raw response body bytes
//   - Duration: Response time
type ErrorResponseFixture struct {
	// S3Error is the parsed S3 error structure
	S3Error S3Error

	// HTTPStatus is the HTTP status code
	HTTPStatus int

	// Headers contains the HTTP response headers
	Headers http.Header

	// Body is the raw response body
	Body []byte

	// Duration is how long the response took
	Duration time.Duration
}

// ToFixture converts an HTTP response to a fixture.
//
// This helper parses an HTTP response and creates a complete fixture that can
// be used for testing, validation, and debugging. It reads and parses the
// response body, extracts headers, and captures timing information.
//
// Parameters:
//   - resp: HTTP response to convert
//   - duration: Time taken for the request
//
// Returns:
//   - *ErrorResponseFixture containing all response information
//   - error if response body parsing fails
//
// Use this helper when:
//   - Capturing actual responses for fixture creation
//   - Creating test fixtures from real server responses
//   - Debugging unexpected error responses
//   - Recording responses for regression testing
//
// Note: This function consumes the response body.
func ToFixture(resp *http.Response, duration time.Duration) (*ErrorResponseFixture, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var s3Err S3Error
	if err := xml.Unmarshal(body, &s3Err); err != nil {
		return nil, err
	}

	return &ErrorResponseFixture{
		S3Error:   s3Err,
		HTTPStatus: resp.StatusCode,
		Headers:   resp.Header.Clone(),
		Body:      body,
		Duration:  duration,
	}, nil
}

// =============================================================================
// ERROR CATEGORY ENUMERATION
// =============================================================================

// ErrorCategory represents different categories of errors.
//
// Error categories are used to organize and group error patterns by type.
// They enable efficient testing and validation of specific error classes.
//
// Use error categories when:
//   - Getting all patterns for a specific error type via PatternsForCategory()
//   - Organizing tests by error category
//   - Filtering errors in monitoring and logging
//   - Understanding error patterns at a high level
//
// Example (using a category):
//
//	authPatterns := PatternsForCategory(CategoryAuth)
//	for _, pattern := range authPatterns {
//	    fmt.Printf("Auth pattern: %s\n", pattern.Name)
//	}
//
// Example (checking a category):
//
//	category := CategoryForCode("NoSuchKey")
//	fmt.Printf("Category: %s\n", category) // "NotFound"
type ErrorCategory string

const (
	// CategoryAuth authentication errors (401, 403)
	//
	// Covers:
	//   - Missing authentication tokens
	//   - Invalid access keys
	//   - Signature mismatches
	//   - Expired requests
	CategoryAuth ErrorCategory = "Auth"

	// CategoryNotFound resource not found errors (404)
	//
	// Covers:
	//   - Missing objects (NoSuchKey)
	//   - Missing buckets
	//   - Missing resources
	CategoryNotFound ErrorCategory = "NotFound"

	// CategoryInvalidRequest invalid request errors (400, 415)
	//
	// Covers:
	//   - Malformed requests
	//   - Invalid parameters
	//   - Unsupported media types
	CategoryInvalidRequest ErrorCategory = "InvalidRequest"

	// CategoryMethodNotAllowed method not allowed errors (405)
	//
	// Covers:
	//   - Unsupported HTTP methods
	//   - Method restrictions
	CategoryMethodNotAllowed ErrorCategory = "MethodNotAllowed"

	// CategoryInternal server errors (500)
	//
	// Covers:
	//   - Internal server errors
	//   - Service unavailable
	//   - Infrastructure failures
	CategoryInternal ErrorCategory = "Internal"

	// CategoryCORS CORS-related errors
	//
	// Covers:
	//   - CORS validation failures
	//   - Origin restrictions
	CategoryCORS ErrorCategory = "CORS"

	// CategoryGeneral general uncategorized errors
	//
	// Default category for errors that don't fit other categories
	CategoryGeneral ErrorCategory = "General"
)

// String returns the string representation of the error category.
func (e ErrorCategory) String() string {
	return string(e)
}

// =============================================================================
// ERROR CODE CONSTANTS
// =============================================================================

// Common S3 error codes used across testing.
//
// These constants represent standard S3 error codes. Using constants instead of
// string literals prevents typos and enables compile-time checking.
//
// Use these constants when:
//   - Creating custom error patterns
//   - Parsing error responses
//   - Writing validation logic
//   - Referencing S3 error codes in tests
//
// Example:
//
//	pattern := ErrorScenarioConfig{
//	    ExpectedCode: ErrorCodeNoSuchKey, // Use constant instead of "NoSuchKey"
//	    ExpectedStatus: 404,
//	}
//
// S3 Error Code Reference:
// https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html
const (
	// ErrorCodeAccessDenied - Access denied (403)
	//
	// Client does not have access to the requested resource.
	// Common causes:
	//   - Invalid or expired credentials
	//   - Insufficient permissions
	//   - ACL restrictions
	ErrorCodeAccessDenied = "AccessDenied"

	// ErrorCodeInvalidAccessKeyId - Invalid access key (403)
	//
	// The AWS access key ID provided does not exist in our records.
	// Common causes:
	//   - Mistyped access key
	//   - Access key that doesn't belong to any account
	ErrorCodeInvalidAccessKeyId = "InvalidAccessKeyId"

	// ErrorCodeSignatureDoesNotMatch - Signature mismatch (403)
	//
	// The request signature calculated by the server does not match the signature provided.
	// Common causes:
	//   - Incorrect secret key
	//   - Tampered request
	//   - Signing algorithm mismatch
	ErrorCodeSignatureDoesNotMatch = "SignatureDoesNotMatch"

	// ErrorCodeMissingAuthenticationToken - Missing auth header (403)
	//
	// Authorization header is missing or required date header is missing.
	// Common causes:
	//   - No Authorization header
	//   - Missing X-Amz-Date header
	ErrorCodeMissingAuthenticationToken = "MissingAuthenticationToken"

	// ErrorCodeNoSuchKey - Resource not found (404)
	//
	// The specified key does not exist.
	// Common causes:
	//   - Object was deleted
	//   - Wrong bucket/key name
	ErrorCodeNoSuchKey = "NoSuchKey"

	// ErrorCodeMethodNotAllowed - Method not allowed (405)
	//
	// The specified method is not allowed against this resource.
	// Common causes:
	//   - Wrong HTTP method for endpoint
	//   - Method restrictions
	ErrorCodeMethodNotAllowed = "MethodNotAllowed"

	// ErrorCodeUnsupportedMediaType - Unsupported media type (415)
	//
	// The content type is not supported.
	// Common causes:
	//   - Wrong Content-Type header
	//   - Unsupported format
	ErrorCodeUnsupportedMediaType = "UnsupportedMediaType"

	// ErrorCodeInvalidRequest - Invalid request (400)
	//
	// The request is invalid or malformed.
	// Common causes:
	//   - Missing required parameters
	//   - Invalid query string
	//   - Malformed request body
	ErrorCodeInvalidRequest = "InvalidRequest"

	// ErrorCodeInternalError - Internal server error (500)
	//
	// Internal server error occurred.
	// Common causes:
	//   - Server-side bug
	//   - Infrastructure failure
	//   - Unexpected error condition
	ErrorCodeInternalError = "InternalError"

	// ErrorCodeRequestExpired - Request timestamp expired (403)
	//
	// Request has expired (typically >15 minutes old).
	// Common causes:
	//   - Stale request
	//   - Clock skew
	ErrorCodeRequestExpired = "RequestExpired"
)

// =============================================================================
// ERROR CATEGORY MAPPING
// =============================================================================

// CategoryForCode returns the error category for a given error code.
//
// This helper maps S3 error codes to their corresponding error categories,
// enabling organized testing and analysis by error type.
//
// Parameters:
//   - code: S3 error code (e.g., "NoSuchKey", "AccessDenied")
//
// Returns:
//   - ErrorCategory for the given error code
//
// Use this helper when:
//   - Categorizing errors for logging/monitoring
//   - Organizing tests by error category
//   - Understanding error types at runtime
//   - Filtering errors by category
//
// Example:
//
//	category := CategoryForCode("NoSuchKey")
//	if category == CategoryNotFound {
//	    fmt.Println("This is a not-found error")
//	}
//
// Category mappings:
//   - Auth: AccessDenied, InvalidAccessKeyId, SignatureDoesNotMatch,
//     MissingAuthenticationToken, RequestExpired
//   - NotFound: NoSuchKey
//   - MethodNotAllowed: MethodNotAllowed
//   - InvalidRequest: UnsupportedMediaType, InvalidRequest
//   - Internal: InternalError
//   - General: Any other error code
func CategoryForCode(code string) ErrorCategory {
	switch code {
	case ErrorCodeAccessDenied, ErrorCodeInvalidAccessKeyId,
	     ErrorCodeSignatureDoesNotMatch, ErrorCodeMissingAuthenticationToken,
	     ErrorCodeRequestExpired:
		return CategoryAuth
	case ErrorCodeNoSuchKey:
		return CategoryNotFound
	case ErrorCodeMethodNotAllowed:
		return CategoryMethodNotAllowed
	case ErrorCodeUnsupportedMediaType, ErrorCodeInvalidRequest:
		return CategoryInvalidRequest
	case ErrorCodeInternalError:
		return CategoryInternal
	default:
		return CategoryGeneral
	}
}

// =============================================================================
// HTTP STATUS CODE MAPPING
// =============================================================================

// ExpectedStatusCodeForCode returns the expected HTTP status code for an S3 error code.
//
// This helper maps S3 error codes to their corresponding HTTP status codes,
// enabling validation of error responses without hardcoding status values.
//
// Parameters:
//   - code: S3 error code (e.g., "NoSuchKey", "AccessDenied")
//
// Returns:
//   - Expected HTTP status code for the given error code
//
// Use this helper when:
//   - Validating error responses
//   - Creating test expectations
//   - Understanding error code to status code mappings
//   - Writing generic error validation logic
//
// Example:
//
//	expectedStatus := ExpectedStatusCodeForCode("NoSuchKey")
//	if resp.StatusCode == expectedStatus {
//	    fmt.Println("Status code is correct")
//	}
//
// Status code mappings:
//   - 403: AccessDenied, InvalidAccessKeyId, SignatureDoesNotMatch,
//     MissingAuthenticationToken, RequestExpired
//   - 404: NoSuchKey
//   - 405: MethodNotAllowed
//   - 415: UnsupportedMediaType
//   - 400: InvalidRequest
//   - 500: InternalError
//   - 500 (default): Any other error code
func ExpectedStatusCodeForCode(code string) int {
	switch code {
	case ErrorCodeAccessDenied, ErrorCodeInvalidAccessKeyId,
	     ErrorCodeSignatureDoesNotMatch, ErrorCodeMissingAuthenticationToken,
	     ErrorCodeRequestExpired:
		return 403
	case ErrorCodeNoSuchKey:
		return 404
	case ErrorCodeMethodNotAllowed:
		return 405
	case ErrorCodeUnsupportedMediaType:
		return 415
	case ErrorCodeInvalidRequest:
		return 400
	case ErrorCodeInternalError:
		return 500
	default:
		return 500 // Default to internal error for unknown codes
	}
}

// =============================================================================
// PREDEFINED COMMON ERROR PATTERNS
// =============================================================================
// These predefined patterns represent common error scenarios that can be reused
// across different test files. Each pattern is fully documented and provides
// a complete ErrorScenarioConfig ready for use.
//
// Usage Examples:
//   // Use a predefined pattern directly
//   pattern := CommonErrorPatterns.ResourceNotFound
//   config := pattern.Config()
//
//   // Extend a predefined pattern
//   customPattern := CommonErrorPatterns.ResourceNotFound
//   customPattern.Config.ExpectedMessage = "Custom not found message"
//
//   // Create a new pattern based on a predefined one
//   myPattern := ErrorScenarioConfig{
//       Name: "My custom pattern",
//       ExpectedCode: CommonErrorPatterns.ResourceNotFound.Config.ExpectedCode,
//       ExpectedStatus: CommonErrorPatterns.ResourceNotFound.Config.ExpectedStatus,
//   }
// =============================================================================

// CommonErrorPatterns provides a collection of predefined error patterns
// for common S3 error scenarios. These patterns are ready-to-use and cover
// the most frequent error cases encountered in testing.
var CommonErrorPatterns = struct {
	// ResourceNotFound covers the classic 404 error when a requested object does not exist
	ResourceNotFound ErrorScenarioConfig

	// AccessDenied covers authentication failures where credentials are invalid
	AccessDenied ErrorScenarioConfig

	// InvalidRequest covers malformed requests with missing or invalid parameters
	InvalidRequest ErrorScenarioConfig

	// UnsupportedMediaType covers content-type validation errors
	UnsupportedMediaType ErrorScenarioConfig

	// MethodNotAllowed covers HTTP method validation errors
	MethodNotAllowed ErrorScenarioConfig

	// InternalServerError covers server-side errors
	InternalServerError ErrorScenarioConfig

	// SignatureMismatch covers AWS signature validation failures
	SignatureMismatch ErrorScenarioConfig

	// RequestExpired covers timestamp validation errors
	RequestExpired ErrorScenarioConfig
}{
	// ResourceNotFound: Standard 404 error for missing objects
	ResourceNotFound: ErrorScenarioConfig{
		Name:              "Resource Not Found",
		ExpectedCode:      ErrorCodeNoSuchKey,
		ExpectedStatus:    404,
		ExpectedMessage:   "The specified key does not exist",
		ExpectedKeywords:  []string{"not", "found", "exist", "no such key"},
		MinMessageLength:   15,
		MaxResponseTime:    500 * time.Millisecond,
		Description:       "Tests for requests attempting to access non-existent resources. Covers GET, HEAD, and other operations on missing objects.",
		Category:          string(CategoryNotFound),
	},

	// AccessDenied: Authentication failure with invalid credentials
	AccessDenied: ErrorScenarioConfig{
		Name:              "Access Denied",
		ExpectedCode:      ErrorCodeAccessDenied,
		ExpectedStatus:    403,
		ExpectedMessage:   "Access Denied",
		ExpectedKeywords:  []string{"access", "denied", "permission", "authorized"},
		MinMessageLength:   12,
		MaxResponseTime:    300 * time.Millisecond,
		Description:       "Tests for requests with invalid or missing credentials. Covers both missing Authorization headers and invalid access keys.",
		Category:          string(CategoryAuth),
	},

	// InvalidRequest: Malformed request parameters
	InvalidRequest: ErrorScenarioConfig{
		Name:              "Invalid Request",
		ExpectedCode:      ErrorCodeInvalidRequest,
		ExpectedStatus:    400,
		ExpectedMessage:   "The request is invalid",
		ExpectedKeywords:  []string{"invalid", "request", "malformed"},
		MinMessageLength:   15,
		MaxResponseTime:    200 * time.Millisecond,
		Description:       "Tests for requests with missing required parameters, invalid query strings, or malformed request bodies.",
		Category:          string(CategoryInvalidRequest),
	},

	// UnsupportedMediaType: Content-type validation error
	UnsupportedMediaType: ErrorScenarioConfig{
		Name:              "Unsupported Media Type",
		ExpectedCode:      ErrorCodeUnsupportedMediaType,
		ExpectedStatus:    415,
		ExpectedMessage:   "The content type is not supported",
		ExpectedKeywords:  []string{"content", "type", "supported", "media"},
		MinMessageLength:   20,
		MaxResponseTime:    200 * time.Millisecond,
		Description:       "Tests for requests with unsupported Content-Type headers. Used when the server requires specific content types (e.g., application/xml).",
		Category:          string(CategoryInvalidRequest),
	},

	// MethodNotAllowed: HTTP method validation error
	MethodNotAllowed: ErrorScenarioConfig{
		Name:              "Method Not Allowed",
		ExpectedCode:      ErrorCodeMethodNotAllowed,
		ExpectedStatus:    405,
		ExpectedMessage:   "The specified method is not allowed",
		ExpectedKeywords:  []string{"method", "allowed", "supported"},
		MinMessageLength:   20,
		MaxResponseTime:    200 * time.Millisecond,
		Description:       "Tests for requests using HTTP methods that are not supported for the given resource. Commonly tested with POST on object endpoints.",
		Category:          string(CategoryMethodNotAllowed),
	},

	// InternalServerError: Server-side errors
	InternalServerError: ErrorScenarioConfig{
		Name:              "Internal Server Error",
		ExpectedCode:      ErrorCodeInternalError,
		ExpectedStatus:    500,
		ExpectedMessage:   "Internal server error",
		ExpectedKeywords:  []string{"internal", "error", "server"},
		MinMessageLength:   15,
		MaxResponseTime:    1000 * time.Millisecond,
		Description:       "Tests for unexpected server-side errors. These typically indicate a bug or infrastructure issue that needs investigation.",
		Category:          string(CategoryInternal),
	},

	// SignatureMismatch: AWS signature validation failure
	SignatureMismatch: ErrorScenarioConfig{
		Name:              "Signature Does Not Match",
		ExpectedCode:      ErrorCodeSignatureDoesNotMatch,
		ExpectedStatus:    403,
		ExpectedMessage:   "The request signature we calculated does not match",
		ExpectedKeywords:  []string{"signature", "match", "calculated"},
		MinMessageLength:   25,
		MaxResponseTime:    300 * time.Millisecond,
		Description:       "Tests for requests with incorrect AWS signature calculations. Covers malformed signatures, tampered requests, and incorrect secret keys.",
		Category:          string(CategoryAuth),
	},

	// RequestExpired: Timestamp validation error
	RequestExpired: ErrorScenarioConfig{
		Name:              "Request Expired",
		ExpectedCode:      ErrorCodeRequestExpired,
		ExpectedStatus:    403,
		ExpectedMessage:   "Request has expired",
		ExpectedKeywords:  []string{"expired", "timestamp", "date"},
		MinMessageLength:   15,
		MaxResponseTime:    300 * time.Millisecond,
		Description:       "Tests for requests with expired timestamps. AWS S3 requires requests to be made within a specific time window (typically 15 minutes).",
		Category:          string(CategoryAuth),
	},
}

// =============================================================================
// AUTHENTICATION ERROR PATTERNS
// =============================================================================
// These patterns are specifically designed for authentication-related testing.
// They cover the spectrum of authentication failures from missing credentials
// to expired timestamps.
//
// Design Philosophy:
//   - Comprehensive coverage of all auth failure scenarios
//   - Clear distinction between different auth error types
//   - Realistic expectations for response times
//   - Actionable error messages
//
// Use these patterns when:
//   - Testing authentication and authorization logic
//   - Verifying AWS signature validation
//   - Testing credential validation
//   - Writing security-focused tests
// =============================================================================

// AuthErrorPatterns provides predefined patterns for authentication error scenarios.
//
// Use this collection when:
//   - Testing authentication failures
//   - Validating AWS signature behavior
//   - Testing credential validation logic
//   - Writing security test suites
//
// Example:
//
//	pattern := AuthErrorPatterns.MissingAuthHeader
//	expectedStatus := pattern.ExpectedStatus    // 403
//	expectedCode := pattern.ExpectedCode        // "MissingAuthenticationToken"
//
// Available patterns:
//   - MissingAuthHeader: No Authorization header
//   - InvalidAccessKeyId: Invalid access key
//   - SignatureDoesNotMatch: Incorrect signature
//   - MissingDateHeader: No X-Amz-Date header
//   - RequestExpired: Expired timestamp
//   - MalformedAuthHeader: Invalid Authorization header format
var AuthErrorPatterns = struct {
	// MissingAuthHeader covers requests without Authorization header
	//
	// Use this pattern when:
	//   - Testing requests without any authentication
	//   - Verifying auth header is required
	//   - Testing basic auth enforcement
	//
	// Expected behavior:
	//   - HTTP Status: 403
	//   - S3 Code: "MissingAuthenticationToken"
	//   - Message: "Missing Authentication Token"
	//   - Response Time: < 250ms
	MissingAuthHeader ErrorScenarioConfig

	// InvalidAccessKeyId covers requests with invalid access key
	//
	// Use this pattern when:
	//   - Testing with non-existent access keys
	//   - Verifying key validation logic
	//   - Testing typo scenarios
	//
	// Expected behavior:
	//   - HTTP Status: 403
	//   - S3 Code: "InvalidAccessKeyId"
	//   - Message: "The AWS Access Key Id you provided does not exist"
	//   - Response Time: < 300ms
	InvalidAccessKeyId ErrorScenarioConfig

	// SignatureDoesNotMatch covers requests with incorrect signature
	//
	// Use this pattern when:
	//   - Testing with incorrect secret keys
	//   - Verifying signature calculation
	//   - Testing signature validation logic
	//
	// Expected behavior:
	//   - HTTP Status: 403
	//   - S3 Code: "SignatureDoesNotMatch"
	//   - Message: "The request signature we calculated does not match"
	//   - Response Time: < 350ms
	SignatureDoesNotMatch ErrorScenarioConfig

	// MissingDateHeader covers requests without X-Amz-Date header
	//
	// Use this pattern when:
	//   - Testing signature requirements
	//   - Verifying date header validation
	//   - Testing timestamp enforcement
	//
	// Expected behavior:
	//   - HTTP Status: 403
	//   - S3 Code: "MissingAuthenticationToken"
	//   - Message: "Missing required date header"
	//   - Response Time: < 250ms
	MissingDateHeader ErrorScenarioConfig

	// RequestExpired covers requests with expired timestamp
	//
	// Use this pattern when:
	//   - Testing stale requests
	//   - Verifying timestamp validation
	//   - Testing time window enforcement
	//
	// Expected behavior:
	//   - HTTP Status: 403
	//   - S3 Code: "RequestExpired"
	//   - Message: "Request has expired"
	//   - Response Time: < 300ms
	RequestExpired ErrorScenarioConfig

	// MalformedAuthHeader covers requests with invalid Authorization header format
	//
	// Use this pattern when:
	//   - Testing malformed Authorization headers
	//   - Verifying header format validation
	//   - Testing parser robustness
	//
	// Expected behavior:
	//   - HTTP Status: 403
	//   - S3 Code: "MissingAuthenticationToken"
	//   - Message: "Invalid authorization header format"
	//   - Response Time: < 250ms
	MalformedAuthHeader ErrorScenarioConfig
}{
	// MissingAuthHeader: No Authorization header present
	MissingAuthHeader: ErrorScenarioConfig{
		Name:              "Missing Authentication Token",
		ExpectedCode:      ErrorCodeMissingAuthenticationToken,
		ExpectedStatus:    403,
		ExpectedMessage:   "Missing Authentication Token",
		ExpectedKeywords:  []string{"missing", "authentication", "token", "authorization"},
		MinMessageLength:   15,
		MaxResponseTime:    250 * time.Millisecond,
		Description:       "Tests for requests completely missing the Authorization header. This is the most basic authentication failure scenario.",
		Category:          string(CategoryAuth),
	},

	// InvalidAccessKeyId: Access key not found or invalid
	InvalidAccessKeyId: ErrorScenarioConfig{
		Name:              "Invalid Access Key ID",
		ExpectedCode:      ErrorCodeInvalidAccessKeyId,
		ExpectedStatus:    403,
		ExpectedMessage:   "The AWS Access Key Id you provided does not exist in our records",
		ExpectedKeywords:  []string{"access", "key", "invalid", "exist"},
		MinMessageLength:   20,
		MaxResponseTime:    300 * time.Millisecond,
		Description:       "Tests for requests with an invalid or non-existent access key ID. This typically means the key was mistyped or does not belong to any account.",
		Category:          string(CategoryAuth),
	},

	// SignatureDoesNotMatch: Calculated signature doesn't match
	SignatureDoesNotMatch: ErrorScenarioConfig{
		Name:              "Signature Does Not Match",
		ExpectedCode:      ErrorCodeSignatureDoesNotMatch,
		ExpectedStatus:    403,
		ExpectedMessage:   "The request signature we calculated does not match the signature you provided",
		ExpectedKeywords:  []string{"signature", "match", "calculated", "provided"},
		MinMessageLength:   30,
		MaxResponseTime:    350 * time.Millisecond,
		Description:       "Tests for requests where the signature calculation failed. This can be due to incorrect secret key, tampered request, or signing algorithm mismatch.",
		Category:          string(CategoryAuth),
	},

	// MissingDateHeader: Required date header is missing
	MissingDateHeader: ErrorScenarioConfig{
		Name:              "Missing Date Header",
		ExpectedCode:      ErrorCodeMissingAuthenticationToken,
		ExpectedStatus:    403,
		ExpectedMessage:   "Missing required date header",
		ExpectedKeywords:  []string{"missing", "date", "header", "timestamp"},
		MinMessageLength:   15,
		MaxResponseTime:    250 * time.Millisecond,
		Description:       "Tests for requests missing the X-Amz-Date header. The date header is required for signature calculation and request timestamp validation.",
		Category:          string(CategoryAuth),
	},

	// RequestExpired: Request timestamp is too old
	RequestExpired: ErrorScenarioConfig{
		Name:              "Request Expired",
		ExpectedCode:      ErrorCodeRequestExpired,
		ExpectedStatus:    403,
		ExpectedMessage:   "Request has expired",
		ExpectedKeywords:  []string{"expired", "timestamp", "date", "old"},
		MinMessageLength:   15,
		MaxResponseTime:    300 * time.Millisecond,
		Description:       "Tests for requests with timestamps outside the valid window (typically more than 15 minutes old). AWS S3 rejects stale requests.",
		Category:          string(CategoryAuth),
	},

	// MalformedAuthHeader: Authorization header format is invalid
	MalformedAuthHeader: ErrorScenarioConfig{
		Name:              "Malformed Authorization Header",
		ExpectedCode:      ErrorCodeMissingAuthenticationToken,
		ExpectedStatus:    403,
		ExpectedMessage:   "Invalid authorization header format",
		ExpectedKeywords:  []string{"authorization", "header", "malformed", "format", "invalid"},
		MinMessageLength:   20,
		MaxResponseTime:    250 * time.Millisecond,
		Description:       "Tests for requests with Authorization headers that don't match the expected AWS signature format. Covers missing fields or incorrect structure.",
		Category:          string(CategoryAuth),
	},
}

// =============================================================================
// CLIENT ERROR PATTERNS (4xx)
// =============================================================================
// These patterns represent client-side errors (4xx status codes). These are
// errors where the client made an invalid request and should correct their
// request before retrying.
//
// Design Philosophy:
//   - Clear indication of client-side issues
//   - Specific error codes for different client failures
//   - Actionable error messages
//   - Fast response times for client errors
//
// Use these patterns when:
//   - Testing client error handling
//   - Verifying request validation logic
//   - Testing API contract compliance
//   - Writing integration tests for error scenarios
// =============================================================================

// ClientErrorPatterns provides predefined patterns for client error scenarios (4xx).
//
// Use this collection when:
//   - Testing 4xx client error responses
//   - Validating request validation
//   - Testing API behavior for malformed requests
//   - Writing comprehensive client error tests
//
// Example:
//
//	pattern := ClientErrorPatterns.NotFound
//	expectedStatus := pattern.ExpectedStatus    // 404
//	expectedCode := pattern.ExpectedCode        // "NoSuchKey"
//
// Available patterns:
//   - BadRequest: Generic 400 Bad Request errors
//   - NotFound: 404 Not Found errors
//   - MethodNotAllowed: 405 Method Not Allowed errors
//   - UnsupportedMediaType: 415 Unsupported Media Type errors
var ClientErrorPatterns = struct {
	// BadRequest covers generic 400 Bad Request errors
	//
	// Use this pattern when:
	//   - Testing malformed requests
	//   - Verifying request validation
	//   - Testing catch-all client error handling
	//
	// Expected behavior:
	//   - HTTP Status: 400
	//   - S3 Code: "InvalidRequest"
	//   - Message: "Bad Request"
	//   - Response Time: < 200ms
	BadRequest ErrorScenarioConfig

	// NotFound covers 404 Not Found errors
	//
	// Use this pattern when:
	//   - Testing missing resource scenarios
	//   - Verifying 404 error responses
	//   - Testing resource existence validation
	//
	// Expected behavior:
	//   - HTTP Status: 404
	//   - S3 Code: "NoSuchKey"
	//   - Response Time: < 500ms
	NotFound ErrorScenarioConfig

	// MethodNotAllowed covers 405 Method Not Allowed errors
	//
	// Use this pattern when:
	//   - Testing HTTP method restrictions
	//   - Verifying method validation
	//   - Testing API endpoint contract
	//
	// Expected behavior:
	//   - HTTP Status: 405
	//   - S3 Code: "MethodNotAllowed"
	//   - Response Time: < 200ms
	MethodNotAllowed ErrorScenarioConfig

	// UnsupportedMediaType covers 415 Unsupported Media Type errors
	//
	// Use this pattern when:
	//   - Testing content-type validation
	//   - Verifying media type restrictions
	//   - Testing format enforcement
	//
	// Expected behavior:
	//   - HTTP Status: 415
	//   - S3 Code: "UnsupportedMediaType"
	//   - Response Time: < 200ms
	UnsupportedMediaType ErrorScenarioConfig
}{
	// BadRequest: Generic bad request error
	BadRequest: ErrorScenarioConfig{
		Name:              "Bad Request",
		ExpectedCode:      ErrorCodeInvalidRequest,
		ExpectedStatus:    400,
		ExpectedMessage:   "Bad Request",
		ExpectedKeywords:  []string{"bad", "request", "invalid"},
		MinMessageLength:   10,
		MaxResponseTime:    200 * time.Millisecond,
		Description:       "Tests for generic bad request errors. This is a catch-all for malformed requests that don't fit more specific error categories.",
		Category:          string(CategoryInvalidRequest),
	},

	// NotFound: Resource not found
	NotFound: CommonErrorPatterns.ResourceNotFound,

	// MethodNotAllowed: HTTP method not supported
	MethodNotAllowed: CommonErrorPatterns.MethodNotAllowed,

	// UnsupportedMediaType: Content type not supported
	UnsupportedMediaType: CommonErrorPatterns.UnsupportedMediaType,
}

// =============================================================================
// SERVER ERROR PATTERNS (5xx)
// =============================================================================
// These patterns represent server-side errors (5xx status codes). These are
// errors where the server failed to fulfill a valid request.
//
// Design Philosophy:
//   - Identify server-side issues requiring investigation
//   - Provide actionable error information
//   - Set appropriate response time expectations
//   - Distinguish between transient and permanent errors
//
// Use these patterns when:
//   - Testing server error handling
//   - Verifying error response structure for server failures
//   - Testing circuit breaker logic
//   - Writing resilience tests
// =============================================================================

// ServerErrorPatterns provides predefined patterns for server error scenarios (5xx).
//
// Use this collection when:
//   - Testing 5xx server error responses
//   - Validating server failure handling
//   - Testing error recovery logic
//   - Writing server error scenarios
//
// Example:
//
//	pattern := ServerErrorPatterns.InternalError
//	expectedStatus := pattern.ExpectedStatus    // 500
//	expectedCode := pattern.ExpectedCode        // "InternalError"
//
// Available patterns:
//   - InternalError: 500 Internal Server Error
//   - ServiceUnavailable: 503 Service Unavailable
var ServerErrorPatterns = struct {
	// InternalError covers 500 Internal Server Error
	//
	// Use this pattern when:
	//   - Testing unexpected server failures
	//   - Verifying 500 error responses
	//   - Testing server error handling
	//
	// Expected behavior:
	//   - HTTP Status: 500
	//   - S3 Code: "InternalError"
	//   - Message: "Internal server error"
	//   - Response Time: < 1000ms
	InternalError ErrorScenarioConfig

	// ServiceUnavailable covers 503 Service Unavailable
	//
	// Use this pattern when:
	//   - Testing service unavailability scenarios
	//   - Verifying 503 error responses
	//   - Testing maintenance mode
	//   - Testing circuit breaker behavior
	//
	// Expected behavior:
	//   - HTTP Status: 503
	//   - S3 Code: "InternalError"
	//   - Message: "Service Unavailable"
	//   - Response Time: < 2000ms
	ServiceUnavailable ErrorScenarioConfig
}{
	// InternalError: Generic internal server error
	InternalError: CommonErrorPatterns.InternalServerError,

	// ServiceUnavailable: Service temporarily unavailable
	ServiceUnavailable: ErrorScenarioConfig{
		Name:              "Service Unavailable",
		ExpectedCode:      ErrorCodeInternalError,
		ExpectedStatus:    503,
		ExpectedMessage:   "Service Unavailable",
		ExpectedKeywords:  []string{"unavailable", "service", "temporarily"},
		MinMessageLength:   15,
		MaxResponseTime:    2000 * time.Millisecond,
		Description:       "Tests for service unavailability errors. This typically occurs during maintenance, overload, or infrastructure issues.",
		Category:          string(CategoryInternal),
	},
}

// =============================================================================
// ERROR PATTERN COLLECTIONS BY CATEGORY
// =============================================================================
// These collections organize patterns by error category for easy access
// when testing specific types of errors.
// =============================================================================

// ErrorPatternByCategory maps error categories to their predefined patterns.
// This allows quick access to all patterns for a specific error type.
var ErrorPatternByCategory = map[ErrorCategory][]ErrorScenarioConfig{
	CategoryAuth: {
		AuthErrorPatterns.MissingAuthHeader,
		AuthErrorPatterns.InvalidAccessKeyId,
		AuthErrorPatterns.SignatureDoesNotMatch,
		AuthErrorPatterns.MissingDateHeader,
		AuthErrorPatterns.RequestExpired,
		AuthErrorPatterns.MalformedAuthHeader,
		CommonErrorPatterns.SignatureMismatch,
	},
	CategoryNotFound: {
		ClientErrorPatterns.NotFound,
	},
	CategoryInvalidRequest: {
		ClientErrorPatterns.BadRequest,
		ClientErrorPatterns.UnsupportedMediaType,
		CommonErrorPatterns.InvalidRequest,
	},
	CategoryMethodNotAllowed: {
		ClientErrorPatterns.MethodNotAllowed,
	},
	CategoryInternal: {
		ServerErrorPatterns.InternalError,
		ServerErrorPatterns.ServiceUnavailable,
	},
}

// =============================================================================
// ERROR PATTERN BUILDERS
// =============================================================================
// These builder functions help create custom error patterns based on
// predefined patterns or from scratch.
//
// Use these functions when:
//   - Getting patterns by error code dynamically
//   - Retrieving all patterns for a specific category
//   - Getting all common patterns for iteration
//   - Building custom validation logic
// =============================================================================

// PatternForCode returns a predefined pattern for the given error code,
// or a default pattern if no specific pattern exists.
//
// This function provides dynamic access to predefined patterns based on their
// error code. It's useful when working with error codes from responses or
// external sources.
//
// Parameters:
//   - code: S3 error code (e.g., "NoSuchKey", "AccessDenied")
//
// Returns:
//   - ErrorScenarioConfig for the given error code, or a default pattern
//
// Use this function when:
//   - Looking up patterns dynamically from error codes
//   - Handling errors from external sources
//   - Building flexible validation logic
//   - Converting error codes to test patterns
//
// Example:
//
//	pattern := PatternForCode("NoSuchKey")
//	fmt.Printf("Pattern: %s, Status: %d\n", pattern.Name, pattern.ExpectedStatus)
//
// Example (unknown code):
//
//	pattern := PatternForCode("UnknownError")
//	// Returns default pattern with ExpectedStatus: 500
//
// Supported error codes:
//   - "AccessDenied" → CommonErrorPatterns.AccessDenied
//   - "InvalidAccessKeyId" → AuthErrorPatterns.InvalidAccessKeyId
//   - "SignatureDoesNotMatch" → AuthErrorPatterns.SignatureDoesNotMatch
//   - "MissingAuthenticationToken" → AuthErrorPatterns.MissingAuthHeader
//   - "NoSuchKey" → CommonErrorPatterns.ResourceNotFound
//   - "MethodNotAllowed" → CommonErrorPatterns.MethodNotAllowed
//   - "UnsupportedMediaType" → CommonErrorPatterns.UnsupportedMediaType
//   - "InvalidRequest" → CommonErrorPatterns.InvalidRequest
//   - "InternalError" → CommonErrorPatterns.InternalServerError
//   - "RequestExpired" → CommonErrorPatterns.RequestExpired
//   - Any other code → Default pattern (500, "Unknown Error Pattern")
func PatternForCode(code string) ErrorScenarioConfig {
	switch code {
	case ErrorCodeAccessDenied:
		return CommonErrorPatterns.AccessDenied
	case ErrorCodeInvalidAccessKeyId:
		return AuthErrorPatterns.InvalidAccessKeyId
	case ErrorCodeSignatureDoesNotMatch:
		return AuthErrorPatterns.SignatureDoesNotMatch
	case ErrorCodeMissingAuthenticationToken:
		return AuthErrorPatterns.MissingAuthHeader
	case ErrorCodeNoSuchKey:
		return CommonErrorPatterns.ResourceNotFound
	case ErrorCodeMethodNotAllowed:
		return CommonErrorPatterns.MethodNotAllowed
	case ErrorCodeUnsupportedMediaType:
		return CommonErrorPatterns.UnsupportedMediaType
	case ErrorCodeInvalidRequest:
		return CommonErrorPatterns.InvalidRequest
	case ErrorCodeInternalError:
		return CommonErrorPatterns.InternalServerError
	case ErrorCodeRequestExpired:
		return CommonErrorPatterns.RequestExpired
	default:
		// Return a default pattern for unknown codes
		return ErrorScenarioConfig{
			Name:             "Unknown Error Pattern",
			ExpectedCode:     code,
			ExpectedStatus:   500,
			ExpectedMessage:  "Unknown error",
			MinMessageLength: 10,
			MaxResponseTime:  1000 * time.Millisecond,
			Description:      "Default pattern for unknown error codes",
			Category:         string(CategoryGeneral),
		}
	}
}

// PatternsForCategory returns all predefined patterns for a given error category.
//
// This function provides access to all patterns belonging to a specific category,
// useful for organized testing and validation by error type.
//
// Parameters:
//   - category: Error category (e.g., CategoryAuth, CategoryNotFound)
//
// Returns:
//   - Slice of ErrorScenarioConfig for the category, or empty slice if not found
//
// Use this function when:
//   - Testing all errors of a specific type
//   - Building category-specific test suites
//   - Organizing tests by error category
//   - Validating category-based error handling
//
// Example:
//
//	authPatterns := PatternsForCategory(CategoryAuth)
//	for _, pattern := range authPatterns {
//	    fmt.Printf("Auth pattern: %s\n", pattern.Name)
//	}
//
// Example (unknown category):
//
//	patterns := PatternsForCategory("UnknownCategory")
//	// Returns empty slice
//
// Available categories:
//   - CategoryAuth: 7 patterns (authentication errors)
//   - CategoryNotFound: 1 pattern (resource not found)
//   - CategoryInvalidRequest: 3 patterns (invalid requests)
//   - CategoryMethodNotAllowed: 1 pattern (method restrictions)
//   - CategoryInternal: 2 patterns (server errors)
func PatternsForCategory(category ErrorCategory) []ErrorScenarioConfig {
	patterns, exists := ErrorPatternByCategory[category]
	if !exists {
		return []ErrorScenarioConfig{}
	}
	result := make([]ErrorScenarioConfig, len(patterns))
	copy(result, patterns)
	return result
}

// AllCommonPatterns returns all predefined common error patterns.
//
// This function provides access to all common error patterns in a single slice,
// useful for comprehensive testing and iteration over all common scenarios.
//
// Returns:
//   - Slice of all 8 common error patterns
//
// Use this function when:
//   - Testing all common error scenarios
//   - Building comprehensive test suites
//   - Iterating over all common patterns
//   - Validating common error handling
//
// Example:
//
//	allPatterns := AllCommonPatterns()
//	for _, pattern := range allPatterns {
//	    fmt.Printf("Pattern: %s (Status: %d)\n", pattern.Name, pattern.ExpectedStatus)
//	}
//
// Example (count):
//
//	totalPatterns := len(AllCommonPatterns()) // 8
//
// Returned patterns (in order):
//   1. ResourceNotFound (404)
//   2. AccessDenied (403)
//   3. InvalidRequest (400)
//   4. UnsupportedMediaType (415)
//   5. MethodNotAllowed (405)
//   6. InternalServerError (500)
//   7. SignatureMismatch (403)
//   8. RequestExpired (403)
func AllCommonPatterns() []ErrorScenarioConfig {
	return []ErrorScenarioConfig{
		CommonErrorPatterns.ResourceNotFound,
		CommonErrorPatterns.AccessDenied,
		CommonErrorPatterns.InvalidRequest,
		CommonErrorPatterns.UnsupportedMediaType,
		CommonErrorPatterns.MethodNotAllowed,
		CommonErrorPatterns.InternalServerError,
		CommonErrorPatterns.SignatureMismatch,
		CommonErrorPatterns.RequestExpired,
	}
}

// =============================================================================
// DOCUMENTATION
// =============================================================================

/*
File Organization:

This file (error_test_patterns.go) contains:
- Base error types (S3Error, ErrorResponseFixture)
- Error configuration types (ErrorScenarioConfig)
- Error categorization (ErrorCategory, error codes)
- Mapping functions (CategoryForCode, ExpectedStatusCodeForCode)
- Predefined error patterns (CommonErrorPatterns, AuthErrorPatterns, etc.)

Related files:
- error_test_infrastructure_test.go - Test-specific helpers and validators
- error_test_patterns_base_test.go - Test pattern structures and tables
- error_test_patterns_test.go - Tests for the pattern infrastructure
- http_error_fixtures.go - HTTP error response fixtures

Usage Patterns:

1. Using predefined patterns:
   pattern := CommonErrorPatterns.ResourceNotFound
   config := pattern.Config()

2. Getting a pattern by error code:
   pattern := PatternForCode("NoSuchKey")

3. Getting all patterns for a category:
   authPatterns := PatternsForCategory(CategoryAuth)

4. Getting all common patterns:
   allPatterns := AllCommonPatterns()

5. Using auth-specific patterns:
   pattern := AuthErrorPatterns.MissingAuthHeader
   expectedStatus := pattern.ExpectedStatus

Pattern Categories:

- CommonErrorPatterns: 8 patterns covering the most common S3 error scenarios
- AuthErrorPatterns: 6 patterns specifically for authentication failures
- ClientErrorPatterns: 4 patterns for 4xx client errors
- ServerErrorPatterns: 2 patterns for 5xx server errors
- ErrorPatternByCategory: Map-based access to patterns by category

Extension Points:

- Add new predefined patterns to the pattern structs
- Extend ErrorCategory for new error types
- Add new mapping functions for additional error metadata
- Create custom fixture types by extending ErrorResponseFixture
- Add builder functions for specific pattern types

Best Practices:

1. Use predefined patterns when possible for consistency
2. Extend patterns with custom configurations rather than creating from scratch
3. Document custom patterns with clear descriptions
4. Use category-based pattern access for organized testing
5. Keep response time expectations realistic for your environment
*/
