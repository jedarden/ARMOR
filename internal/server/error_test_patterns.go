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
// =============================================================================

// AuthErrorPatterns provides predefined patterns for authentication error scenarios.
var AuthErrorPatterns = struct {
	// MissingAuthHeader covers requests without Authorization header
	MissingAuthHeader ErrorScenarioConfig

	// InvalidAccessKeyId covers requests with invalid access key
	InvalidAccessKeyId ErrorScenarioConfig

	// SignatureDoesNotMatch covers requests with incorrect signature
	SignatureDoesNotMatch ErrorScenarioConfig

	// MissingDateHeader covers requests without X-Amz-Date header
	MissingDateHeader ErrorScenarioConfig

	// RequestExpired covers requests with expired timestamp
	RequestExpired ErrorScenarioConfig

	// MalformedAuthHeader covers requests with invalid Authorization header format
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
// =============================================================================

// ClientErrorPatterns provides predefined patterns for client error scenarios (4xx).
var ClientErrorPatterns = struct {
	// BadRequest covers generic 400 Bad Request errors
	BadRequest ErrorScenarioConfig

	// NotFound covers 404 Not Found errors
	NotFound ErrorScenarioConfig

	// MethodNotAllowed covers 405 Method Not Allowed errors
	MethodNotAllowed ErrorScenarioConfig

	// UnsupportedMediaType covers 415 Unsupported Media Type errors
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
// =============================================================================

// ServerErrorPatterns provides predefined patterns for server error scenarios (5xx).
var ServerErrorPatterns = struct {
	// InternalError covers 500 Internal Server Error
	InternalError ErrorScenarioConfig

	// ServiceUnavailable covers 503 Service Unavailable
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
// =============================================================================

// PatternForCode returns a predefined pattern for the given error code,
// or a default pattern if no specific pattern exists.
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
