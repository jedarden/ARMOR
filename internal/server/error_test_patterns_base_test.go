package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// =============================================================================
// ERROR TEST PATTERNS
// =============================================================================
// This file provides base test patterns and reusable test table structures
// for error response testing. It extends the infrastructure in error_test_infrastructure.go
// with organized patterns for common error scenarios.
//
// Design Philosophy:
// - Table-driven tests for consistency
// - Composable patterns for flexibility
// - Clear extension points for custom scenarios
// - Self-documenting test structures
//
// Usage:
//   1. Choose a base pattern that matches your error type
//   2. Extend the test table with your specific scenarios
//   3. Use the provided helpers to execute tests
//   4. Leverage predefined validators for common assertions
// =============================================================================

// =============================================================================
// BASE TEST TABLE STRUCTURES
// =============================================================================

// CommonErrorTestCase is the base structure for all error test cases.
// This structure captures the common elements across all error types.
type CommonErrorTestCase struct {
	// Name is the test case name for reporting
	Name string

	// Description explains what this test case validates
	Description string

	// SetupRequest creates the request that triggers the error
	SetupRequest func(*testing.T) *http.Request

	// ExpectedStatus is the expected HTTP status code
	ExpectedStatus int

	// ExpectedCode is the expected S3 error code
	ExpectedCode string

	// ExpectedMessageSubstring is text that should appear in the error message
	ExpectedMessageSubstring string

	// ExpectedMessageKeywords are alternative keywords (any match is acceptable)
	ExpectedMessageKeywords []string

	// MinMessageLength is the minimum acceptable message length (default: 15)
	MinMessageLength int

	// ValidateResponse provides custom validation beyond the standard checks
	ValidateResponse func(*testing.T, *httptest.ResponseRecorder)

	// MaxResponseTime is the maximum acceptable response duration
	MaxResponseTime time.Duration

	// SkipCORSValidation disables CORS header checks (for non-CORS scenarios)
	SkipCORSValidation bool
}

// AuthenticationErrorTestCase extends CommonErrorTestCase for auth-specific errors.
type AuthenticationErrorTestCase struct {
	CommonErrorTestCase

	// AccessKey used in the request (for documentation)
	AccessKey string

	// AuthErrorType categorizes the authentication failure
	AuthErrorType string

	// ExpectedAuthError is the specific error expected from auth verification
	ExpectedAuthError error
}

// NonAuthenticationErrorTestCase extends CommonErrorTestCase for non-auth errors.
type NonAuthenticationErrorTestCase struct {
	CommonErrorTestCase

	// ErrorCategory categorizes the non-auth error (e.g., "NotFound", "InvalidRequest")
	ErrorCategory string

	// RequiresAuth indicates whether the request needs valid authentication
	RequiresAuth bool

	// ResourcePath is the path being accessed (for documentation)
	ResourcePath string
}

// CORSErrorTestCase extends CommonErrorTestCase for CORS-specific testing.
type CORSErrorTestCase struct {
	CommonErrorTestCase

	// Origin is the CORS origin for the request
	Origin string

	// ExpectedCORSOrigin is the expected CORS Allow-Origin header value
	ExpectedCORSOrigin string

	// ExpectedCORSMethods is the expected CORS Allow-Methods header value
	ExpectedCORSMethods string

	// ExpectedCORSHeaders is the expected CORS Allow-Headers header value
	ExpectedCORSHeaders string

	// IsPreflight indicates this is a CORS preflight request
	IsPreflight bool
}

// ContentTypeErrorTestCase extends CommonErrorTestCase for content-type errors.
type ContentTypeErrorTestCase struct {
	CommonErrorTestCase

	// RequestContentType is the Content-Type header in the request
	RequestContentType string

	// ExpectedContentType is the expected error message content type reference
	ExpectedContentType string

	// SupportedContentTypes are the types that should be mentioned in the error
	SupportedContentTypes []string
}

// =============================================================================
// PREDEFINED ERROR TEST TABLES
// =============================================================================
// These tables provide common test scenarios that can be used directly
// or extended for specific testing needs.
// =============================================================================

// StandardAuthenticationErrorTests returns the base authentication error test cases.
// These tests cover the most common authentication failure scenarios.
//
// Scenarios covered:
// - Missing authorization header
// - Invalid access key ID
// - Invalid secret key
// - Malformed authorization header
// - Missing date header
// - Expired request
// - Invalid signature
//
// Usage:
//   tests := StandardAuthenticationErrorTests()
//   for _, tt := range tests {
//       t.Run(tt.Name, func(t *testing.T) {
//           RunAuthenticationTestCase(t, fixture, tt)
//       })
//   }
func StandardAuthenticationErrorTests() []AuthenticationErrorTestCase {
	return []AuthenticationErrorTestCase{
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Missing authorization header",
				Description:              "Request without Authorization header should return 403",
				SetupRequest:             createMissingAuthHeaderRequest,
				ExpectedStatus:           403,
				ExpectedCode:             "AccessDenied",
				ExpectedMessageKeywords:  []string{"access", "denied", "authorization"},
				MinMessageLength:          15,
				SkipCORSValidation:        false,
			},
			AccessKey:        "",
			AuthErrorType:    "MissingAuthHeader",
			ExpectedAuthError: ErrMissingAuthHeader,
		},
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Invalid access key",
				Description:              "Request with invalid access key should return 403",
				SetupRequest:             createInvalidKeyRequest,
				ExpectedStatus:           403,
				ExpectedCode:             "InvalidAccessKeyId",
				ExpectedMessageKeywords:  []string{"invalid", "access", "key"},
				MinMessageLength:          15,
			},
			AccessKey:        "INVALIDKEY",
			AuthErrorType:    "InvalidAccessKey",
			ExpectedAuthError: ErrInvalidAccessKey,
		},
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Invalid signature",
				Description:              "Request with incorrect signature should return 403",
				SetupRequest:             createInvalidSignatureRequest,
				ExpectedStatus:           403,
				ExpectedCode:             "SignatureDoesNotMatch",
				ExpectedMessageKeywords:  []string{"signature", "match"},
				MinMessageLength:          15,
			},
			AccessKey:        "TESTACCESSKEY",
			AuthErrorType:    "SignatureMismatch",
			ExpectedAuthError: ErrSignatureMismatch,
		},
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Malformed authorization header",
				Description:              "Request with malformed Authorization header should return 403",
				SetupRequest:             createMalformedAuthRequest,
				ExpectedStatus:           403,
				ExpectedCode:             "AccessDenied",
				ExpectedMessageKeywords:  []string{"authorization", "header"},
				MinMessageLength:          15,
			},
			AccessKey:        "",
			AuthErrorType:    "MalformedAuthHeader",
			ExpectedAuthError: ErrInvalidCredential,
		},
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Missing date header",
				Description:              "Request without X-Amz-Date header should return 403",
				SetupRequest:             createMissingDateRequest,
				ExpectedStatus:           403,
				ExpectedCode:             "AccessDenied",
				ExpectedMessageKeywords:  []string{"date", "header"},
				MinMessageLength:          15,
			},
			AccessKey:        "TESTACCESSKEY",
			AuthErrorType:    "MissingDateHeader",
			ExpectedAuthError: ErrMissingDateHeader,
		},
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Expired request",
				Description:              "Request with expired timestamp should return 403",
				SetupRequest:             createExpiredRequest,
				ExpectedStatus:           403,
				ExpectedCode:             "AccessDenied",
				ExpectedMessageKeywords:  []string{"expired", "request"},
				MinMessageLength:          15,
			},
			AccessKey:        "TESTACCESSKEY",
			AuthErrorType:    "RequestExpired",
			ExpectedAuthError: ErrRequestExpired,
		},
	}
}

// StandardNonAuthenticationErrorTests returns the base non-authentication error test cases.
// These tests cover common non-authentication error scenarios.
//
// Scenarios covered:
// - Resource not found (404)
// - Method not supported (405)
// - Unsupported media type (415)
// - Internal server error (500)
//
// Usage:
//   tests := StandardNonAuthenticationErrorTests()
//   for _, tt := range tests {
//       t.Run(tt.Name, func(t *testing.T) {
//           RunNonAuthenticationTestCase(t, fixture, tt)
//       })
//   }
func StandardNonAuthenticationErrorTests() []NonAuthenticationErrorTestCase {
	return []NonAuthenticationErrorTestCase{
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Non-existent resource returns 404",
				Description:              "Request for non-existent resource should return 404 NoSuchKey",
				SetupRequest:             createNotFoundRequest,
				ExpectedStatus:           404,
				ExpectedCode:             "NoSuchKey",
				ExpectedMessageKeywords:  []string{"not", "found", "exist"},
				MinMessageLength:          15,
			},
			ErrorCategory:  "NotFound",
			RequiresAuth:   true,
			ResourcePath:   "/test-bucket/nonexistent",
		},
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Unsupported method returns 405",
				Description:              "Request with unsupported HTTP method should return 405",
				SetupRequest:             createMethodNotAllowedRequest,
				ExpectedStatus:           405,
				ExpectedCode:             "MethodNotAllowed",
				ExpectedMessageKeywords:  []string{"method", "allowed"},
				MinMessageLength:          15,
			},
			ErrorCategory:  "MethodNotAllowed",
			RequiresAuth:   true,
			ResourcePath:   "/test-bucket/key",
		},
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Unsupported media type returns 415",
				Description:              "Request with unsupported content type should return 415",
				SetupRequest:             createUnsupportedMediaTypeRequest,
				ExpectedStatus:           415,
				ExpectedCode:             "UnsupportedMediaType",
				ExpectedMessageKeywords:  []string{"content", "type", "supported"},
				MinMessageLength:          15,
			},
			ErrorCategory:  "UnsupportedMediaType",
			RequiresAuth:   true,
			ResourcePath:   "/test-bucket/key",
		},
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Internal server error returns 500",
				Description:              "Server error should return 500 with error details",
				SetupRequest:             createInternalErrorRequest,
				ExpectedStatus:           500,
				ExpectedCode:             "InternalError",
				ExpectedMessageKeywords:  []string{"internal", "error"},
				MinMessageLength:          15,
			},
			ErrorCategory:  "InternalError",
			RequiresAuth:   true,
			ResourcePath:   "/test-bucket/key",
		},
	}
}

// StandardCORSErrorTests returns the base CORS error test cases.
// These tests verify that CORS headers are present on error responses.
//
// Usage:
//   tests := StandardCORSErrorTests()
//   for _, tt := range tests {
//       t.Run(tt.Name, func(t *testing.T) {
//           RunCORSTestCase(t, fixture, tt)
//       })
//   }
func StandardCORSErrorTests() []CORSErrorTestCase {
	return []CORSErrorTestCase{
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "404 error includes CORS headers",
				Description:              "NotFound error should include CORS headers",
				SetupRequest:             createNotFoundRequest,
				ExpectedStatus:           404,
				ExpectedCode:             "NoSuchKey",
				MinMessageLength:          15,
			},
			Origin:             "*",
			ExpectedCORSOrigin: "*",
			ExpectedCORSMethods: "GET, PUT, DELETE, HEAD, POST, OPTIONS",
			ExpectedCORSHeaders: "Authorization, Content-Type, Range, Content-Length",
			IsPreflight:         false,
		},
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "403 error includes CORS headers",
				Description:              "AccessDenied error should include CORS headers",
				SetupRequest:             createInvalidKeyRequest,
				ExpectedStatus:           403,
				ExpectedCode:             "InvalidAccessKeyId",
				MinMessageLength:          15,
			},
			Origin:             "*",
			ExpectedCORSOrigin: "*",
			ExpectedCORSMethods: "GET, PUT, DELETE, HEAD, POST, OPTIONS",
			ExpectedCORSHeaders: "Authorization, Content-Type, Range, Content-Length",
			IsPreflight:         false,
		},
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "CORS preflight on invalid resource",
				Description:              "OPTIONS request should return CORS headers",
				SetupRequest:             createPreflightRequest,
				ExpectedStatus:           404,
				ExpectedCode:             "NoSuchKey",
				MinMessageLength:          15,
			},
			Origin:             "https://example.com",
			ExpectedCORSOrigin: "https://example.com",
			ExpectedCORSMethods: "GET, PUT, DELETE, HEAD, POST, OPTIONS",
			ExpectedCORSHeaders: "Authorization, Content-Type, Range, Content-Length",
			IsPreflight:         true,
		},
	}
}

// StandardContentTypeErrorTests returns the base content-type error test cases.
// These tests verify content-type validation errors.
//
// Usage:
//   tests := StandardContentTypeErrorTests()
//   for _, tt := range tests {
//       t.Run(tt.Name, func(t *testing.T) {
//           RunContentTypeTestCase(t, fixture, tt)
//       })
//   }
func StandardContentTypeErrorTests() []ContentTypeErrorTestCase {
	return []ContentTypeErrorTestCase{
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Unsupported content type rejected",
				Description:              "Request with unsupported content type should return 415",
				SetupRequest:             createUnsupportedMediaTypeRequest,
				ExpectedStatus:           415,
				ExpectedCode:             "UnsupportedMediaType",
				ExpectedMessageKeywords:  []string{"content", "type"},
				MinMessageLength:          15,
			},
			RequestContentType:    "application/json",
			ExpectedContentType:  "application/json",
			SupportedContentTypes: []string{"application/xml", "text/plain"},
		},
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:                     "Missing content type rejected",
				Description:              "Request without content type should return 400",
				SetupRequest:             createMissingContentTypeRequest,
				ExpectedStatus:           400,
				ExpectedCode:             "InvalidRequest",
				ExpectedMessageKeywords:  []string{"content", "type", "required"},
				MinMessageLength:          15,
			},
			RequestContentType:    "",
			ExpectedContentType:  "",
			SupportedContentTypes: []string{"application/xml"},
		},
	}
}

// =============================================================================
// TEST EXECUTION HELPERS
// =============================================================================
// These helpers execute test cases with standardized validation.
// =============================================================================

// RunAuthenticationTestCase executes a single authentication error test case.
// It performs standard validation plus authentication-specific checks.
func RunAuthenticationTestCase(t *testing.T, fixture *TestServerFixture, tt AuthenticationErrorTestCase) {
	t.Helper()

	// Create request and measure response time
	req := tt.SetupRequest(t)
	duration, w := MeasureRequestTime(fixture.Handler, req)

	// Standard validation
	RunCommonErrorValidations(t, w, tt.CommonErrorTestCase, duration)

	// Authentication-specific validation
	if tt.ExpectedAuthError != nil {
		// The error response should indicate the auth failure type
		VerifyErrorResponse(t, w).
			MessageContainsAny(tt.AuthErrorType).
			Assert()
	}
}

// RunNonAuthenticationTestCase executes a single non-authentication error test case.
// It performs standard validation for non-auth errors.
func RunNonAuthenticationTestCase(t *testing.T, fixture *TestServerFixture, tt NonAuthenticationErrorTestCase) {
	t.Helper()

	// Create request and measure response time
	req := tt.SetupRequest(t)
	duration, w := MeasureRequestTime(fixture.Handler, req)

	// Standard validation
	RunCommonErrorValidations(t, w, tt.CommonErrorTestCase, duration)
}

// RunCORSTestCase executes a single CORS error test case.
// It performs standard validation plus CORS-specific checks.
func RunCORSTestCase(t *testing.T, fixture *TestServerFixture, tt CORSErrorTestCase) {
	t.Helper()

	// Create request and measure response time
	req := tt.SetupRequest(t)
	duration, w := MeasureRequestTime(fixture.Handler, req)

	// Standard validation
	RunCommonErrorValidations(t, w, tt.CommonErrorTestCase, duration)

	// CORS-specific validation
	VerifyErrorResponse(t, w).
		CORSOrigin(tt.ExpectedCORSOrigin).
		CORSMethods(tt.ExpectedCORSMethods).
		CORSHeaders(tt.ExpectedCORSHeaders).
		Assert()
}

// RunContentTypeTestCase executes a single content-type error test case.
// It performs standard validation plus content-type-specific checks.
func RunContentTypeTestCase(t *testing.T, fixture *TestServerFixture, tt ContentTypeErrorTestCase) {
	t.Helper()

	// Create request and measure response time
	req := tt.SetupRequest(t)
	duration, w := MeasureRequestTime(fixture.Handler, req)

	// Standard validation
	RunCommonErrorValidations(t, w, tt.CommonErrorTestCase, duration)

	// Content-type-specific validation
	VerifyErrorResponse(t, w).
		MessageContainsAny(tt.ExpectedContentType).
		Assert()
}

// RunCommonErrorValidations performs standard validation for all error types.
// This is called by the specific test case runners.
func RunCommonErrorValidations(t *testing.T, w *httptest.ResponseRecorder, tt CommonErrorTestCase, duration time.Duration) {
	t.Helper()

	// Use the fluent validator for comprehensive checks
	validator := VerifyErrorResponseWithTiming(t, w, duration)

	// Always validate HTTP status
	validator.HTTPStatusCode(tt.ExpectedStatus)

	// Always validate Content-Type
	validator.ContentType("application/xml")

	// Always validate error code
	if tt.ExpectedCode != "" {
		validator.HasCode(tt.ExpectedCode)
	}

	// Always validate message is present
	validator.BodyNotEmpty()

	// Always validate XML declaration
	validator.HasXMLDeclaration()

	// Validate message substring or keywords
	if tt.ExpectedMessageSubstring != "" {
		validator.HasMessage(tt.ExpectedMessageSubstring)
	} else if len(tt.ExpectedMessageKeywords) > 0 {
		validator.MessageContainsAny(tt.ExpectedMessageKeywords...)
	}

	// Validate minimum message length
	if tt.MinMessageLength > 0 {
		validator.MessageMinLength(tt.MinMessageLength)
	} else {
		validator.MessageMinLength(15) // Default minimum
	}

	// Validate response time if specified
	if tt.MaxResponseTime > 0 {
		validator.ResponseTime(tt.MaxResponseTime)
	}

	// Validate CORS headers unless skipped
	if !tt.SkipCORSValidation {
		validator.HasCORSHeaders()
	}

	// Execute all validations
	validator.Assert()

	// Run custom validation if provided
	if tt.ValidateResponse != nil {
		tt.ValidateResponse(t, w)
	}
}

// =============================================================================
// TABLE-DRIVEN TEST RUNNERS
// =============================================================================
// These helpers run entire test tables with organized reporting.
// =============================================================================

// RunAuthenticationErrorTable executes a table of authentication error tests.
// Use this for running authentication test suites with consistent reporting.
func RunAuthenticationErrorTable(t *testing.T, fixture *TestServerFixture, tests []AuthenticationErrorTestCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Description != "" {
				t.Logf("Test: %s", tt.Description)
			}
			RunAuthenticationTestCase(t, fixture, tt)
		})
	}
}

// RunNonAuthenticationErrorTable executes a table of non-authentication error tests.
// Use this for running non-authentication test suites with consistent reporting.
func RunNonAuthenticationErrorTable(t *testing.T, fixture *TestServerFixture, tests []NonAuthenticationErrorTestCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Description != "" {
				t.Logf("Test: %s", tt.Description)
			}
			RunNonAuthenticationTestCase(t, fixture, tt)
		})
	}
}

// RunCORSErrorTable executes a table of CORS error tests.
// Use this for running CORS test suites with consistent reporting.
func RunCORSErrorTable(t *testing.T, fixture *TestServerFixture, tests []CORSErrorTestCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Description != "" {
				t.Logf("Test: %s", tt.Description)
			}
			RunCORSTestCase(t, fixture, tt)
		})
	}
}

// =============================================================================
// REQUEST CREATION HELPERS
// =============================================================================
// These helpers create requests for common error scenarios.
// =============================================================================

// createMissingAuthHeaderRequest creates a request without auth header.
func createMissingAuthHeaderRequest(t *testing.T) *http.Request {
	t.Helper()
	return CreateTestRequest(t, "GET", "/test-bucket/key", nil, nil)
}

// createInvalidKeyRequest creates a request with an invalid access key.
func createInvalidKeyRequest(t *testing.T) *http.Request {
	t.Helper()
	return createSignedRequestForAuthTest(t, "GET", "/test-bucket/key", "", "INVALIDKEY", "TESTSECRETKEY123456789012345678901234", nil)
}

// createInvalidSignatureRequest creates a request with an invalid signature.
func createInvalidSignatureRequest(t *testing.T) *http.Request {
	t.Helper()
	req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
	// Tamper with signature
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-005/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=BADSIGNATURE1234567890abcdef")
	return req
}

// createMalformedAuthRequest creates a request with a malformed auth header.
func createMalformedAuthRequest(t *testing.T) *http.Request {
	t.Helper()
	req := CreateTestRequest(t, "GET", "/test-bucket/key", nil, nil)
	req.Header.Set("Authorization", "InvalidAuthHeaderFormat")
	return req
}

// createMissingDateRequest creates a request without a date header.
func createMissingDateRequest(t *testing.T) *http.Request {
	t.Helper()
	req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
	req.Header.Del("X-Amz-Date")
	return req
}

// createExpiredRequest creates a request with an expired timestamp.
func createExpiredRequest(t *testing.T) *http.Request {
	t.Helper()
	oldTime := time.Now().Add(-20 * time.Minute)
	return createSignedRequestForAuthTest(t, "GET", "/test-bucket/key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", &oldTime)
}

// createNotFoundRequest creates a request for a non-existent resource.
func createNotFoundRequest(t *testing.T) *http.Request {
	t.Helper()
	return createSignedRequestForAuthTest(t, "GET", "/test-bucket/nonexistent", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
}

// createMethodNotAllowedRequest creates a request with an unsupported method.
func createMethodNotAllowedRequest(t *testing.T) *http.Request {
	t.Helper()
	return createSignedRequestForAuthTest(t, "POST", "/test-bucket/key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
}

// createUnsupportedMediaTypeRequest creates a request with unsupported content type.
func createUnsupportedMediaTypeRequest(t *testing.T) *http.Request {
	t.Helper()
	body := `{"data": "test"}`
	return createSignedRequestForAuthTest(t, "PUT", "/test-bucket/key", body, "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
}

// createInternalErrorRequest creates a request that triggers an internal error.
// Note: This may need adjustment based on your server's implementation.
func createInternalErrorRequest(t *testing.T) *http.Request {
	t.Helper()
	// This creates a request that might trigger an internal error
	// Adjust based on your server's implementation
	return createSignedRequestForAuthTest(t, "GET", "/test-bucket/.armor/internal-trigger", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
}

// createMissingContentTypeRequest creates a request without content type.
func createMissingContentTypeRequest(t *testing.T) *http.Request {
	t.Helper()
	body := `{"data": "test"}`
	return createSignedRequestForAuthTest(t, "PUT", "/test-bucket/key", body, "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
}

// createPreflightRequest creates a CORS preflight request.
func createPreflightRequest(t *testing.T) *http.Request {
	t.Helper()
	req := CreateTestRequest(t, "OPTIONS", "/test-bucket/nonexistent", nil, map[string]string{
		"Origin":                        "https://example.com",
		"Access-Control-Request-Method": "GET",
		"Access-Control-Request-Headers": "authorization",
	})
	return req
}

// =============================================================================
// EXTENSION HELPERS
// =============================================================================
// These helpers make it easy to extend the base patterns for custom scenarios.
// =============================================================================

// ExtendAuthenticationTests creates a new test table by extending the base authentication tests.
// Use this to add custom authentication scenarios to the standard suite.
//
// Example:
//   customTests := []AuthenticationErrorTestCase{
//       {
//           CommonErrorTestCase: CommonErrorTestCase{
//               Name: "My custom auth test",
//               SetupRequest: func(t *testing.T) *http.Request {
//                   return createMyCustomRequest(t)
//               },
//               ExpectedStatus: 403,
//               ExpectedCode: "AccessDenied",
//           },
//       },
//   }
//   tests := ExtendAuthenticationTests(customTests)
func ExtendAuthenticationTests(customTests []AuthenticationErrorTestCase) []AuthenticationErrorTestCase {
	baseTests := StandardAuthenticationErrorTests()
	extended := make([]AuthenticationErrorTestCase, 0, len(baseTests)+len(customTests))
	extended = append(extended, baseTests...)
	extended = append(extended, customTests...)
	return extended
}

// ExtendNonAuthenticationTests creates a new test table by extending the base non-auth tests.
// Use this to add custom non-authentication scenarios to the standard suite.
//
// Example:
//   customTests := []NonAuthenticationErrorTestCase{
//       {
//           CommonErrorTestCase: CommonErrorTestCase{
//               Name: "My custom non-auth test",
//               SetupRequest: func(t *testing.T) *http.Request {
//                   return createMyCustomRequest(t)
//               },
//               ExpectedStatus: 404,
//               ExpectedCode: "NoSuchKey",
//           },
//           ErrorCategory: "CustomNotFound",
//           RequiresAuth: true,
//       },
//   }
//   tests := ExtendNonAuthenticationTests(customTests)
func ExtendNonAuthenticationTests(customTests []NonAuthenticationErrorTestCase) []NonAuthenticationErrorTestCase {
	baseTests := StandardNonAuthenticationErrorTests()
	extended := make([]NonAuthenticationErrorTestCase, 0, len(baseTests)+len(customTests))
	extended = append(extended, baseTests...)
	extended = append(extended, customTests...)
	return extended
}

// ExtendCORSTests creates a new test table by extending the base CORS tests.
// Use this to add custom CORS scenarios to the standard suite.
//
// Example:
//   customTests := []CORSErrorTestCase{
//       {
//           CommonErrorTestCase: CommonErrorTestCase{
//               Name: "My custom CORS test",
//               SetupRequest: func(t *testing.T) *http.Request {
//                   return createMyCustomRequest(t)
//               },
//               ExpectedStatus: 404,
//               ExpectedCode: "NoSuchKey",
//           },
//           Origin: "*",
//           ExpectedCORSOrigin: "*",
//       },
//   }
//   tests := ExtendCORSTests(customTests)
func ExtendCORSTests(customTests []CORSErrorTestCase) []CORSErrorTestCase {
	baseTests := StandardCORSErrorTests()
	extended := make([]CORSErrorTestCase, 0, len(baseTests)+len(customTests))
	extended = append(extended, baseTests...)
	extended = append(extended, customTests...)
	return extended
}

// =============================================================================
// BUILDER PATTERNS
// =============================================================================
// These builders help construct test cases programmatically.
// =============================================================================

// AuthenticationTestCaseBuilder builds authentication error test cases.
type AuthenticationTestCaseBuilder struct {
	testCase AuthenticationErrorTestCase
}

// NewAuthenticationTestCaseBuilder creates a new builder for authentication tests.
func NewAuthenticationTestCaseBuilder(name string) *AuthenticationTestCaseBuilder {
	return &AuthenticationTestCaseBuilder{
		testCase: AuthenticationErrorTestCase{
			CommonErrorTestCase: CommonErrorTestCase{
				Name:            name,
				MinMessageLength: 15,
			},
		},
	}
}

// WithDescription sets the test description.
func (b *AuthenticationTestCaseBuilder) WithDescription(desc string) *AuthenticationTestCaseBuilder {
	b.testCase.Description = desc
	return b
}

// WithRequest sets the request setup function.
func (b *AuthenticationTestCaseBuilder) WithRequest(setup func(*testing.T) *http.Request) *AuthenticationTestCaseBuilder {
	b.testCase.SetupRequest = setup
	return b
}

// WithExpectedStatus sets the expected HTTP status code.
func (b *AuthenticationTestCaseBuilder) WithExpectedStatus(status int) *AuthenticationTestCaseBuilder {
	b.testCase.ExpectedStatus = status
	return b
}

// WithExpectedCode sets the expected S3 error code.
func (b *AuthenticationTestCaseBuilder) WithExpectedCode(code string) *AuthenticationTestCaseBuilder {
	b.testCase.ExpectedCode = code
	return b
}

// WithMessageKeywords sets the expected message keywords.
func (b *AuthenticationTestCaseBuilder) WithMessageKeywords(keywords ...string) *AuthenticationTestCaseBuilder {
	b.testCase.ExpectedMessageKeywords = keywords
	return b
}

// WithAuthErrorType sets the authentication error type.
func (b *AuthenticationTestCaseBuilder) WithAuthErrorType(errorType string) *AuthenticationTestCaseBuilder {
	b.testCase.AuthErrorType = errorType
	return b
}

// WithAccessKey sets the access key for documentation.
func (b *AuthenticationTestCaseBuilder) WithAccessKey(key string) *AuthenticationTestCaseBuilder {
	b.testCase.AccessKey = key
	return b
}

// Build creates the test case.
func (b *AuthenticationTestCaseBuilder) Build() AuthenticationErrorTestCase {
	return b.testCase
}

// =============================================================================
// REQUEST CREATION HELPERS
// =============================================================================

// createInvalidCredTestRequest creates a signed request for invalid credential testing.
// This is a simple wrapper that calls createSignedRequestForAuthTest.
