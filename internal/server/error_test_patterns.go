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
// This type is used across error testing, fixtures, and validation.
// It matches the standard S3 error response format with Code and Message fields.
type S3Error struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

// =============================================================================
// ERROR SCENARIO TYPES (for non-test fixture usage)
// =============================================================================

// ErrorScenarioConfig represents configuration for an error test scenario.
// This type is used in both test fixtures and non-test code to define
// expected error behaviors.
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
// This type is used for logging, debugging, and performance monitoring.
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
// This helper is useful for logging and debugging error responses.
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
// This type combines the S3 error with HTTP response metadata for
// comprehensive testing scenarios.
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
type ErrorCategory string

const (
	// CategoryAuth authentication errors (401, 403)
	CategoryAuth ErrorCategory = "Auth"

	// CategoryNotFound resource not found errors (404)
	CategoryNotFound ErrorCategory = "NotFound"

	// CategoryInvalidRequest invalid request errors (400, 415)
	CategoryInvalidRequest ErrorCategory = "InvalidRequest"

	// CategoryMethodNotAllowed method not allowed errors (405)
	CategoryMethodNotAllowed ErrorCategory = "MethodNotAllowed"

	// CategoryInternal server errors (500)
	CategoryInternal ErrorCategory = "Internal"

	// CategoryCORS CORS-related errors
	CategoryCORS ErrorCategory = "CORS"

	// CategoryGeneral general uncategorized errors
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
const (
	// ErrorCodeAccessDenied - Access denied (403)
	ErrorCodeAccessDenied = "AccessDenied"

	// ErrorCodeInvalidAccessKeyId - Invalid access key (403)
	ErrorCodeInvalidAccessKeyId = "InvalidAccessKeyId"

	// ErrorCodeSignatureDoesNotMatch - Signature mismatch (403)
	ErrorCodeSignatureDoesNotMatch = "SignatureDoesNotMatch"

	// ErrorCodeMissingAuthenticationToken - Missing auth header (403)
	ErrorCodeMissingAuthenticationToken = "MissingAuthenticationToken"

	// ErrorCodeNoSuchKey - Resource not found (404)
	ErrorCodeNoSuchKey = "NoSuchKey"

	// ErrorCodeMethodNotAllowed - Method not allowed (405)
	ErrorCodeMethodNotAllowed = "MethodNotAllowed"

	// ErrorCodeUnsupportedMediaType - Unsupported media type (415)
	ErrorCodeUnsupportedMediaType = "UnsupportedMediaType"

	// ErrorCodeInvalidRequest - Invalid request (400)
	ErrorCodeInvalidRequest = "InvalidRequest"

	// ErrorCodeInternalError - Internal server error (500)
	ErrorCodeInternalError = "InternalError"

	// ErrorCodeRequestExpired - Request timestamp expired (403)
	ErrorCodeRequestExpired = "RequestExpired"
)

// =============================================================================
// ERROR CATEGORY MAPPING
// =============================================================================

// CategoryForCode returns the error category for a given error code.
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
// DOCUMENTATION
// =============================================================================

/*
File Organization:

This file (error_test_patterns.go) contains:
- Base error types (S3Error, ErrorResponseFixture)
- Error configuration types (ErrorScenarioConfig)
- Error categorization (ErrorCategory, error codes)
- Mapping functions (CategoryForCode, ExpectedStatusCodeForCode)

Related files:
- error_test_infrastructure_test.go - Test-specific helpers and validators
- error_test_patterns_base_test.go - Test pattern structures and tables
- error_test_patterns_test.go - Tests for the pattern infrastructure
- http_error_fixtures.go - HTTP error response fixtures

Usage Patterns:

1. Creating error fixtures:
   fixture := &ErrorResponseFixture{
       S3Error: S3Error{Code: "NoSuchKey", Message: "Not found"},
       HTTPStatus: 404,
   }

2. Using error categories:
   category := CategoryForCode("NoSuchKey")  // Returns CategoryNotFound

3. Getting expected status codes:
   status := ExpectedStatusCodeForCode("AccessDenied")  // Returns 403

4. Configuring error scenarios:
   config := DefaultErrorScenarioConfig()
   config.ExpectedCode = "NoSuchKey"
   config.ExpectedStatus = 404

Extension Points:

- Add new error code constants as needed
- Extend ErrorCategory for new error types
- Add new mapping functions for additional error metadata
- Create custom fixture types by extending ErrorResponseFixture
*/
