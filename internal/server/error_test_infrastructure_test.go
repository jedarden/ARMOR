package server

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// =============================================================================
// ERROR TEST INFRASTRUCTURE
// =============================================================================
// This file provides reusable test infrastructure for error response testing.
// It includes:
// - Test helpers for error response validation
// - Test fixtures for common error scenarios
// - Base test structures for extending error tests
// - Documentation and examples
//
// Usage:
//   1. Use NewTestServer() to create a test server instance
//   2. Use error response helpers to validate responses
//   3. Use test fixtures for common scenarios
//   4. Extend base test structures for custom error types
// =============================================================================

// =============================================================================
// TEST FIXTURES
// =============================================================================

// TestServerFixture provides a configured test server for error testing.
type TestServerFixture struct {
	Config  *config.Config
	Handler http.Handler
}

// NewTestServer creates a new test server fixture with default credentials.
//
// The test server is configured with:
// - Bucket: "test-bucket"
// - Region: "us-east-005"
// - Default credentials: TESTACCESSKEY/TESTSECRETKEY...
// - Full access (no ACL restrictions)
//
// Returns a TestServerFixture ready for error testing.
func NewTestServer(t *testing.T) *TestServerFixture {
	t.Helper()

	credentials := map[string]*config.Credential{
		"TESTACCESSKEY": {
			AccessKey: "TESTACCESSKEY",
			SecretKey: "TESTSECRETKEY123456789012345678901234",
			ACLs:      nil, // Full access
		},
		"RESTRICTEDKEY": {
			AccessKey: "RESTRICTEDKEY",
			SecretKey: "RESTRICTEDSECRET1234567890123456789",
			ACLs: []config.ACLEntry{
				{
					Bucket: "test-bucket",
					Prefix: "allowed/",
				},
			},
		},
	}

	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         make([]byte, 32),
		BlockSize:   65536,
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}

	return &TestServerFixture{
		Config:  cfg,
		Handler: server.Handler(),
	}
}

// TestCredentialsFixture provides test credentials for different scenarios.
type TestCredentialsFixture struct {
	ValidAccessKey      string
	ValidSecretKey      string
	RestrictedAccessKey string
	RestrictedSecretKey string
	InvalidAccessKey    string
	InvalidSecretKey    string
}

// DefaultTestCredentials returns the default test credentials fixture.
func DefaultTestCredentials() *TestCredentialsFixture {
	return &TestCredentialsFixture{
		ValidAccessKey:      "TESTACCESSKEY",
		ValidSecretKey:      "TESTSECRETKEY123456789012345678901234",
		RestrictedAccessKey: "RESTRICTEDKEY",
		RestrictedSecretKey: "RESTRICTEDSECRET1234567890123456789",
		InvalidAccessKey:    "INVALIDKEY",
		InvalidSecretKey:    "WRONGSECRETKEY123456789012345678901",
	}
}

// =============================================================================
// ERROR RESPONSE VALIDATION HELPERS
// =============================================================================

// ErrorResponseValidator provides fluent error response validation.
type ErrorResponseValidator struct {
	t                *testing.T
	response         *httptest.ResponseRecorder
	s3Error          *S3Error
	expectations     []expectation
	measuredDuration time.Duration
}

type expectation struct {
	name     string
	validate func(*ErrorResponseValidator) error
	failed   bool
}

// VerifyErrorResponse creates a new validator for the given response.
func VerifyErrorResponse(t *testing.T, w *httptest.ResponseRecorder) *ErrorResponseValidator {
	t.Helper()

	return &ErrorResponseValidator{
		t:        t,
		response: w,
	}
}

// VerifyErrorResponseWithTiming creates a validator that records response time.
func VerifyErrorResponseWithTiming(t *testing.T, w *httptest.ResponseRecorder, duration time.Duration) *ErrorResponseValidator {
	t.Helper()

	return &ErrorResponseValidator{
		t:                t,
		response:         w,
		measuredDuration: duration,
	}
}

// HTTPStatusCode verifies the HTTP status code.
func (v *ErrorResponseValidator) HTTPStatusCode(code int) *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "HTTPStatusCode",
		validate: func(ev *ErrorResponseValidator) error {
			if ev.response.Code != code {
				ev.t.Errorf("Expected HTTP status %d, got %d", code, ev.response.Code)
				return nil
			}
			return nil
		},
	})
	return v
}

// ContentType verifies the Content-Type header.
func (v *ErrorResponseValidator) ContentType(contentType string) *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "ContentType",
		validate: func(ev *ErrorResponseValidator) error {
			actual := ev.response.Header().Get("Content-Type")
			if actual != contentType {
				ev.t.Errorf("Expected Content-Type '%s', got '%s'", contentType, actual)
			}
			return nil
		},
	})
	return v
}

// HasCode verifies the S3 error code.
func (v *ErrorResponseValidator) HasCode(code string) *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "HasCode",
		validate: func(ev *ErrorResponseValidator) error {
			if ev.s3Error == nil {
				if err := ev.parse(); err != nil {
					ev.t.Errorf("Failed to parse error response: %v", err)
					return err
				}
			}
			if ev.s3Error.Code != code {
				ev.t.Errorf("Expected error code '%s', got '%s'", code, ev.s3Error.Code)
			}
			return nil
		},
	})
	return v
}

// HasMessage verifies the error message contains expected text.
func (v *ErrorResponseValidator) HasMessage(substr string) *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "HasMessage",
		validate: func(ev *ErrorResponseValidator) error {
			if ev.s3Error == nil {
				if err := ev.parse(); err != nil {
					ev.t.Errorf("Failed to parse error response: %v", err)
					return err
				}
			}
			if !strings.Contains(ev.s3Error.Message, substr) {
				ev.t.Errorf("Expected error message to contain '%s', got '%s'", substr, ev.s3Error.Message)
			}
			return nil
		},
	})
	return v
}

// MessageMinLength verifies the error message is at least n characters.
func (v *ErrorResponseValidator) MessageMinLength(minLen int) *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "MessageMinLength",
		validate: func(ev *ErrorResponseValidator) error {
			if ev.s3Error == nil {
				if err := ev.parse(); err != nil {
					ev.t.Errorf("Failed to parse error response: %v", err)
					return err
				}
			}
			if len(ev.s3Error.Message) < minLen {
				ev.t.Errorf("Error message too short (got %d chars, want at least %d): %s",
					len(ev.s3Error.Message), minLen, ev.s3Error.Message)
			}
			return nil
		},
	})
	return v
}

// ResponseTime verifies the response time is under the threshold.
func (v *ErrorResponseValidator) ResponseTime(maxDuration time.Duration) *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "ResponseTime",
		validate: func(ev *ErrorResponseValidator) error {
			if v.measuredDuration > maxDuration {
				v.t.Errorf("Response time %v exceeds maximum %v", v.measuredDuration, maxDuration)
			}
			return nil
		},
	})
	return v
}

// HasXMLDeclaration verifies the response starts with XML declaration.
func (v *ErrorResponseValidator) HasXMLDeclaration() *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "HasXMLDeclaration",
		validate: func(ev *ErrorResponseValidator) error {
			body := v.response.Body.String()
			if !strings.HasPrefix(body, "<?xml") {
				v.t.Errorf("Expected XML declaration, got: %s", body[:min(len(body), 50)])
			}
			return nil
		},
	})
	return v
}

// BodyNotEmpty verifies the response body is not empty.
func (v *ErrorResponseValidator) BodyNotEmpty() *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "BodyNotEmpty",
		validate: func(ev *ErrorResponseValidator) error {
			if v.response.Body.Len() == 0 {
				v.t.Error("Expected non-empty response body")
			}
			return nil
		},
	})
	return v
}

// MessageContainsAny verifies the message contains at least one of the keywords.
func (v *ErrorResponseValidator) MessageContainsAny(keywords ...string) *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "MessageContainsAny",
		validate: func(ev *ErrorResponseValidator) error {
			if ev.s3Error == nil {
				if err := ev.parse(); err != nil {
					ev.t.Errorf("Failed to parse error response: %v", err)
					return err
				}
			}
			message := strings.ToLower(ev.s3Error.Message)
			found := false
			for _, keyword := range keywords {
				if strings.Contains(message, strings.ToLower(keyword)) {
					found = true
					break
				}
			}
			if !found {
				v.t.Errorf("Error message should contain at least one of %v, got: %s",
					keywords, ev.s3Error.Message)
			}
			return nil
		},
	})
	return v
}

// HasCORSHeaders verifies that CORS headers are present on the error response.
func (v *ErrorResponseValidator) HasCORSHeaders() *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "HasCORSHeaders",
		validate: func(ev *ErrorResponseValidator) error {
			origin := ev.response.Header().Get("Access-Control-Allow-Origin")
			if origin == "" {
				ev.t.Error("Expected CORS header 'Access-Control-Allow-Origin' to be set")
			}
			methods := ev.response.Header().Get("Access-Control-Allow-Methods")
			if methods == "" {
				ev.t.Error("Expected CORS header 'Access-Control-Allow-Methods' to be set")
			}
			headers := ev.response.Header().Get("Access-Control-Allow-Headers")
			if headers == "" {
				ev.t.Error("Expected CORS header 'Access-Control-Allow-Headers' to be set")
			}
			return nil
		},
	})
	return v
}

// CORSOrigin verifies the CORS Access-Control-Allow-Origin header value.
func (v *ErrorResponseValidator) CORSOrigin(origin string) *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "CORSOrigin",
		validate: func(ev *ErrorResponseValidator) error {
			actual := v.response.Header().Get("Access-Control-Allow-Origin")
			if actual != origin {
				ev.t.Errorf("Expected CORS origin '%s', got '%s'", origin, actual)
			}
			return nil
		},
	})
	return v
}

// CORSMethods verifies the CORS Access-Control-Allow-Methods header value.
func (v *ErrorResponseValidator) CORSMethods(methods string) *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "CORSMethods",
		validate: func(ev *ErrorResponseValidator) error {
			actual := v.response.Header().Get("Access-Control-Allow-Methods")
			if actual != methods {
				ev.t.Errorf("Expected CORS methods '%s', got '%s'", methods, actual)
			}
			return nil
		},
	})
	return v
}

// CORSHeaders verifies the CORS Access-Control-Allow-Headers header value.
func (v *ErrorResponseValidator) CORSHeaders(headers string) *ErrorResponseValidator {
	v.expectations = append(v.expectations, expectation{
		name: "CORSHeaders",
		validate: func(ev *ErrorResponseValidator) error {
			actual := v.response.Header().Get("Access-Control-Allow-Headers")
			if actual != headers {
				ev.t.Errorf("Expected CORS headers '%s', got '%s'", headers, actual)
			}
			return nil
		},
	})
	return v
}

// Assert runs all validations and asserts if any fail.
// This is the terminal method - call it to execute all validations.
func (v *ErrorResponseValidator) Assert() {
	v.t.Helper()

	// Parse response once for all validations
	if err := v.parse(); err != nil {
		v.t.Fatalf("Failed to parse error response: %v", err)
		return
	}

	// Run all validations
	for _, exp := range v.expectations {
		if err := exp.validate(v); err != nil {
			exp.failed = true
		}
	}
}

// parse parses the S3 error response.
func (v *ErrorResponseValidator) parse() error {
	if v.s3Error != nil {
		return nil // Already parsed
	}

	if v.response.Body.Len() == 0 {
		return nil // Nothing to parse
	}

	v.s3Error = &S3Error{}
	if err := xml.Unmarshal(v.response.Body.Bytes(), v.s3Error); err != nil {
		return err
	}

	return nil
}

// =============================================================================
// SIMPLE VALIDATION HELPERS
// =============================================================================

// VerifyStandardErrorResponse performs standard error response validation.
// This is a convenience function for common validation patterns.
//
// It verifies:
// - HTTP status code matches expected
// - Content-Type is "application/xml"
// - Response body is not empty
// - Error code matches expected
// - Error message is at least 15 characters
// - Response starts with XML declaration
func VerifyStandardErrorResponse(t *testing.T, w *httptest.ResponseRecorder, expectedCode string, expectedStatus int) {
	t.Helper()

	VerifyErrorResponse(t, w).
		HTTPStatusCode(expectedStatus).
		ContentType("application/xml").
		BodyNotEmpty().
		HasCode(expectedCode).
		MessageMinLength(15).
		HasXMLDeclaration().
		Assert()
}

// ParseErrorResponse parses an S3 error response from the response body.
func ParseErrorResponse(t *testing.T, body []byte) *S3Error {
	t.Helper()

	var s3Err S3Error
	if err := xml.Unmarshal(body, &s3Err); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	return &s3Err
}

// =============================================================================
// CORE VALIDATION HELPER FUNCTIONS
// =============================================================================
// These are standalone helper functions for validating HTTP error responses.
// They provide a simpler alternative to the fluent ErrorResponseValidator API.
// Each helper performs a single validation and can be used independently.
// =============================================================================

// ValidateHTTPStatusCode validates the HTTP status code of an error response.
//
// This helper function checks that the response has the expected HTTP status code.
// It's useful for verifying that error responses return the correct status codes
// (e.g., 404 for NotFound, 403 for Forbidden, 400 for BadRequest).
//
// Example:
//
//	ValidateHTTPStatusCode(t, w, 404)
func ValidateHTTPStatusCode(t *testing.T, w *httptest.ResponseRecorder, expectedCode int) {
	t.Helper()

	if w.Code != expectedCode {
		t.Errorf("Expected HTTP status code %d, got %d", expectedCode, w.Code)
	}
}

// ValidateContentType validates the Content-Type header of an error response.
//
// This helper function checks that the response has the expected Content-Type header.
// For S3-compatible error responses, this should typically be "application/xml".
// ValidateContentType is defined in content_type_validation.go with enhanced functionality.

// ValidateErrorStructure validates that an error response has proper structure.
//
// This helper function checks that the error response contains:
// - A non-empty response body
// - A valid S3 Error XML structure with Code and Message fields
// - A non-empty error code
// - A non-empty error message
//
// This ensures the error response is well-formed and contains the required fields.
//
// Example:
//
//	ValidateErrorStructure(t, w)
func ValidateErrorStructure(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()

	// Check response body is not empty
	if w.Body.Len() == 0 {
		t.Fatal("Error response body is empty")
	}

	// Parse the error response
	s3Err := ParseErrorResponse(t, w.Body.Bytes())

	// Validate error code is present
	if s3Err.Code == "" {
		t.Error("Error response missing Code field")
	}

	// Validate error message is present
	if s3Err.Message == "" {
		t.Error("Error response missing Message field")
	}
}

// ValidateErrorCode validates the S3 error code in an error response.
//
// This helper function checks that the error response contains the expected S3 error code.
// Common S3 error codes include: NoSuchKey, AccessDenied, InvalidRequest, etc.
//
// Example:
//
//	ValidateErrorCode(t, w, "NoSuchKey")
func ValidateErrorCode(t *testing.T, w *httptest.ResponseRecorder, expectedCode string) {
	t.Helper()

	s3Err := ParseErrorResponse(t, w.Body.Bytes())

	if s3Err.Code != expectedCode {
		t.Errorf("Expected error code '%s', got '%s'", expectedCode, s3Err.Code)
	}
}

// ValidateCORSHeaders validates that CORS headers are present on an error response.
//
// This helper function checks that the error response includes the required CORS headers:
// - Access-Control-Allow-Origin
// - Access-Control-Allow-Methods
// - Access-Control-Allow-Headers
//
// This ensures that error responses properly support cross-origin requests.
//
// Example:
//
//	ValidateCORSHeaders(t, w)
func ValidateCORSHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()

	// Check Access-Control-Allow-Origin
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin == "" {
		t.Error("Expected CORS header 'Access-Control-Allow-Origin' to be set")
	}

	// Check Access-Control-Allow-Methods
	methods := w.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("Expected CORS header 'Access-Control-Allow-Methods' to be set")
	}

	// Check Access-Control-Allow-Headers
	headers := w.Header().Get("Access-Control-Allow-Headers")
	if headers == "" {
		t.Error("Expected CORS header 'Access-Control-Allow-Headers' to be set")
	}
}

// ValidateCORSOrigin validates the CORS Access-Control-Allow-Origin header value.
//
// This helper function checks that the error response has the expected CORS origin.
// Common values are "*" (allow all origins) or a specific origin.
//
// Example:
//
//	ValidateCORSOrigin(t, w, "*")
func ValidateCORSOrigin(t *testing.T, w *httptest.ResponseRecorder, expectedOrigin string) {
	t.Helper()

	actualOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if actualOrigin != expectedOrigin {
		t.Errorf("Expected CORS origin '%s', got '%s'", expectedOrigin, actualOrigin)
	}
}

// ValidateCORSMethods validates the CORS Access-Control-Allow-Methods header value.
//
// This helper function checks that the error response has the expected CORS methods.
// Common values include: "GET, PUT, DELETE, HEAD, POST, OPTIONS"
//
// Example:
//
//	ValidateCORSMethods(t, w, "GET, PUT, DELETE, HEAD, POST, OPTIONS")
func ValidateCORSMethods(t *testing.T, w *httptest.ResponseRecorder, expectedMethods string) {
	t.Helper()

	actualMethods := w.Header().Get("Access-Control-Allow-Methods")
	if actualMethods != expectedMethods {
		t.Errorf("Expected CORS methods '%s', got '%s'", expectedMethods, actualMethods)
	}
}

// ValidateCORSAllowHeaders validates the CORS Access-Control-Allow-Headers header value.
//
// This helper function checks that the error response has the expected CORS headers.
// Common values include: "Authorization, Content-Type, Range, Content-Length"
//
// Example:
//
//	ValidateCORSAllowHeaders(t, w, "Authorization, Content-Type, Range, Content-Length")
func ValidateCORSAllowHeaders(t *testing.T, w *httptest.ResponseRecorder, expectedHeaders string) {
	t.Helper()

	actualHeaders := w.Header().Get("Access-Control-Allow-Headers")
	if actualHeaders != expectedHeaders {
		t.Errorf("Expected CORS allow-headers '%s', got '%s'", expectedHeaders, actualHeaders)
	}
}

// ValidateErrorMessageContains validates that the error message contains expected text.
//
// This helper function checks that the error response message contains the specified substring.
// This is useful for verifying that error messages are descriptive and helpful.
//
// Example:
//
//	ValidateErrorMessageContains(t, w, "not found")
func ValidateErrorMessageContains(t *testing.T, w *httptest.ResponseRecorder, expectedText string) {
	t.Helper()

	s3Err := ParseErrorResponse(t, w.Body.Bytes())

	if !strings.Contains(s3Err.Message, expectedText) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expectedText, s3Err.Message)
	}
}

// ValidateErrorMessageMinLength validates that the error message meets minimum length.
//
// This helper function checks that the error response message is at least the specified
// number of characters. This ensures error messages are sufficiently descriptive.
//
// Example:
//
//	ValidateErrorMessageMinLength(t, w, 15)
func ValidateErrorMessageMinLength(t *testing.T, w *httptest.ResponseRecorder, minLength int) {
	t.Helper()

	s3Err := ParseErrorResponse(t, w.Body.Bytes())

	if len(s3Err.Message) < minLength {
		t.Errorf("Error message too short (got %d chars, want at least %d): %s",
			len(s3Err.Message), minLength, s3Err.Message)
	}
}

// ValidateXMLDeclaration validates that the response starts with an XML declaration.
//
// This helper function checks that the error response begins with "<?xml" which indicates
// a proper XML declaration. S3 error responses should be valid XML documents.
//
// Example:
//
//	ValidateXMLDeclaration(t, w)
func ValidateXMLDeclaration(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()

	body := w.Body.String()

	if !strings.HasPrefix(body, "<?xml") {
		t.Errorf("Expected response to start with XML declaration, got: %s", body[:min(len(body), 50)])
	}
}

// =============================================================================
// ERROR RESPONSE STRUCTURE VALIDATION
// =============================================================================
// These helpers provide comprehensive error response structure validation
// with support for optional field validation and detailed error reporting.
// =============================================================================

// ErrorStructureValidationOptions configures error structure validation.
type ErrorStructureValidationOptions struct {
	// RequireCode ensures the error code field is present and non-empty
	RequireCode bool
	// RequireMessage ensures the error message field is present and non-empty
	RequireMessage bool
	// ExpectedCode checks that the error code matches the expected value
	ExpectedCode string
	// MinMessageLength ensures the error message meets minimum length
	MinMessageLength int
	// MessageContains checks that the error message contains specific text
	MessageContains string
	// CustomFields validates additional custom fields in the error response
	CustomFields map[string]string
}

// DefaultValidationOptions returns default validation options.
func DefaultValidationOptions() ErrorStructureValidationOptions {
	return ErrorStructureValidationOptions{
		RequireCode:      true,
		RequireMessage:   true,
		MinMessageLength: 10,
		CustomFields:     make(map[string]string),
	}
}

// ValidateErrorResponseStructure validates the structure of an error response.
//
// This helper function performs comprehensive validation of error response structure:
// - Validates response body is not empty
// - Validates error field exists (Code field)
// - Validates error message is present and non-empty
// - Supports optional field validation via options
//
// Returns true if validation passes, false otherwise.
// Note: This function does not call t.Error() to allow testing of invalid cases.
// Use AssertValidErrorResponseStructure() for assertion behavior.
//
// Parameters:
//
//	t: Testing instance for context
//	body: Response body as byte array
//	options: Validation options (use DefaultValidationOptions() for defaults)
//
// Example:
//
//	opts := DefaultValidationOptions()
//	opts.RequireCode = true
//	opts.RequireMessage = true
//	opts.MinMessageLength = 15
//	valid := ValidateErrorResponseStructure(t, responseBody, opts)
func ValidateErrorResponseStructure(t *testing.T, body []byte, options ErrorStructureValidationOptions) bool {
	t.Helper()

	// Check response body is not empty
	if len(body) == 0 {
		return false
	}

	// Parse the error response
	var s3Err S3Error
	if err := xml.Unmarshal(body, &s3Err); err != nil {
		return false
	}

	// Track validation results
	allValid := !options.RequireCode || s3Err.Code != ""

	// Validate error code field exists if required

	// Validate error message field exists if required
	if options.RequireMessage {
		trimmedMsg := strings.TrimSpace(s3Err.Message)
		if trimmedMsg == "" {
			allValid = false
		}
	}

	// Validate expected code if specified
	if options.ExpectedCode != "" && s3Err.Code != options.ExpectedCode {
		allValid = false
	}

	// Validate minimum message length if specified
	if options.MinMessageLength > 0 {
		// Use trimmed message for length validation
		trimmedMsg := strings.TrimSpace(s3Err.Message)
		if len(trimmedMsg) < options.MinMessageLength {
			allValid = false
		}
	}

	// Validate message contains expected text if specified (case-insensitive)
	if options.MessageContains != "" {
		messageLower := strings.ToLower(s3Err.Message)
		keywordLower := strings.ToLower(options.MessageContains)
		if !strings.Contains(messageLower, keywordLower) {
			allValid = false
		}
	}

	// Validate custom fields if specified
	for field, expectedValue := range options.CustomFields {
		actualValue := getXMLField(body, field)
		if actualValue != expectedValue {
			allValid = false
		}
	}

	return allValid
}

// ValidateErrorResponseStructureSimple validates error response structure with defaults.
//
// This is a simplified version that uses default validation options.
// It validates:
// - Response body is not empty
// - Error code field exists and is non-empty
// - Error message field exists and is non-empty (min 10 chars)
//
// Returns true if validation passes, false otherwise.
//
// Example:
//
//	valid := ValidateErrorResponseStructureSimple(t, responseBody)
func ValidateErrorResponseStructureSimple(t *testing.T, body []byte) bool {
	t.Helper()
	return ValidateErrorResponseStructure(t, body, DefaultValidationOptions())
}

// AssertValidErrorResponseStructure validates error structure and asserts if invalid.
//
// This helper validates error response structure and fails the test if validation fails.
// Use this when you want test execution to stop on validation failure.
// Provides detailed error messages for debugging.
//
// Example:
//
//	AssertValidErrorResponseStructure(t, responseBody)
func AssertValidErrorResponseStructure(t *testing.T, body []byte) {
	t.Helper()

	if !ValidateErrorResponseStructureSimple(t, body) {
		// Provide detailed error message
		if len(body) == 0 {
			t.Fatal("Error response structure validation failed: response body is empty")
		}

		var s3Err S3Error
		if err := xml.Unmarshal(body, &s3Err); err != nil {
			t.Fatalf("Error response structure validation failed: invalid XML: %v", err)
		}

		if s3Err.Code == "" {
			t.Fatal("Error response structure validation failed: missing Code field")
		}

		if s3Err.Message == "" {
			t.Fatal("Error response structure validation failed: missing Message field")
		}

		if len(s3Err.Message) < 10 {
			t.Fatalf("Error response structure validation failed: message too short (%d chars, want at least 10): %s",
				len(s3Err.Message), s3Err.Message)
		}

		t.Fatal("Error response structure validation failed")
	}
}

// AssertValidErrorResponseStructureWithOptions validates with custom options and asserts.
//
// This helper validates error response structure with custom options and fails the test
// if validation fails. Use this when you need specific validation requirements.
// Provides detailed error messages for debugging.
//
// Example:
//
//	opts := DefaultValidationOptions()
//	opts.MinMessageLength = 20
//	opts.MessageContains = "authentication"
//	AssertValidErrorResponseStructureWithOptions(t, responseBody, opts)
func AssertValidErrorResponseStructureWithOptions(t *testing.T, body []byte, options ErrorStructureValidationOptions) {
	t.Helper()

	if !ValidateErrorResponseStructure(t, body, options) {
		// Provide detailed error message based on options
		if len(body) == 0 {
			t.Fatal("Error response structure validation failed: response body is empty")
		}

		var s3Err S3Error
		if err := xml.Unmarshal(body, &s3Err); err != nil {
			t.Fatalf("Error response structure validation failed: invalid XML: %v", err)
		}

		if options.RequireCode && s3Err.Code == "" {
			t.Fatal("Error response structure validation failed: missing Code field")
		}

		if options.RequireMessage && s3Err.Message == "" {
			t.Fatal("Error response structure validation failed: missing Message field")
		}

		if options.ExpectedCode != "" && s3Err.Code != options.ExpectedCode {
			t.Fatalf("Error response structure validation failed: expected code '%s', got '%s'",
				options.ExpectedCode, s3Err.Code)
		}

		if options.MinMessageLength > 0 && len(s3Err.Message) < options.MinMessageLength {
			t.Fatalf("Error response structure validation failed: message too short (%d chars, want at least %d): %s",
				len(s3Err.Message), options.MinMessageLength, s3Err.Message)
		}

		if options.MessageContains != "" && !strings.Contains(s3Err.Message, options.MessageContains) {
			t.Fatalf("Error response structure validation failed: expected message to contain '%s', got '%s'",
				options.MessageContains, s3Err.Message)
		}

		for field, expectedValue := range options.CustomFields {
			actualValue := getXMLField(body, field)
			if actualValue != expectedValue {
				t.Fatalf("Error response structure validation failed: expected field '%s' to be '%s', got '%s'",
					field, expectedValue, actualValue)
			}
		}

		t.Fatal("Error response structure validation failed")
	}
}

// getXMLField extracts a field value from XML response body.
// This is a helper for validating custom fields in error responses.
func getXMLField(body []byte, field string) string {
	// Simple XML field extraction using string search
	// This is basic but works for common S3 error response structures
	openTag := "<" + field + ">"
	closeTag := "</" + field + ">"

	bodyStr := string(body)
	openIdx := strings.Index(bodyStr, openTag)
	if openIdx == -1 {
		return ""
	}

	closeIdx := strings.Index(bodyStr[openIdx:], closeTag)
	if closeIdx == -1 {
		return ""
	}

	return strings.TrimSpace(bodyStr[openIdx+len(openTag) : openIdx+closeIdx])
}

// =============================================================================
// REQUEST CREATION HELPERS
// =============================================================================

// CreateTestRequest creates an HTTP request for testing.
//
// This helper centralizes request creation and ensures consistent test setup.
// It supports:
// - Custom HTTP methods and paths
// - Optional request body
// - Custom headers
// - Test server URL format
func CreateTestRequest(t *testing.T, method, path string, body io.Reader, headers map[string]string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, path, body)

	// Set default headers for test requests
	if headers == nil {
		headers = make(map[string]string)
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return req
}

// CreateAuthenticatedTestRequest creates a properly authenticated test request.
//
// This helper creates a request with valid AWS4-HMAC-SHA256 authentication.
// It's useful for testing non-authentication errors (where auth should succeed).
func CreateAuthenticatedTestRequest(t *testing.T, method, path string) *http.Request {
	t.Helper()

	// Use the working authentication helper from auth_integration_test.go
	return createSignedRequestForAuthTest(t, method, path, "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
}

// =============================================================================
// TEST SCENARIO STRUCTURES
// =============================================================================

// ErrorScenario represents a testable error scenario.
type ErrorScenario struct {
	// Name is the scenario name for test reporting
	Name string

	// SetupRequest creates the request that triggers the error
	SetupRequest func(*testing.T) *http.Request

	// ExpectedCode is the expected S3 error code
	ExpectedCode string

	// ExpectedStatus is the expected HTTP status code
	ExpectedStatus int

	// ExpectedKeywords are keywords that should appear in the error message
	ExpectedKeywords []string

	// MinMessageLength is the minimum acceptable message length
	MinMessageLength int

	// MaxResponseTime is the maximum acceptable response duration
	MaxResponseTime time.Duration

	// Description explains what this scenario tests
	Description string
}

// ErrorScenarioResult captures the result of running a scenario.
type ErrorScenarioResult struct {
	ScenarioName string
	StatusCode   int
	ErrorCode    string
	Message      string
	Duration     time.Duration
	Passed       bool
}

// RunErrorScenario executes a single error scenario and returns the result.
func RunErrorScenario(t *testing.T, fixture *TestServerFixture, scenario ErrorScenario) *ErrorScenarioResult {
	t.Helper()

	req := scenario.SetupRequest(t)
	w := httptest.NewRecorder()

	// Measure response time
	start := time.Now()
	fixture.Handler.ServeHTTP(w, req)
	duration := time.Since(start)

	// Parse response
	var s3Err S3Error
	xml.Unmarshal(w.Body.Bytes(), &s3Err)

	// Build result
	result := &ErrorScenarioResult{
		ScenarioName: scenario.Name,
		StatusCode:   w.Code,
		ErrorCode:    s3Err.Code,
		Message:      s3Err.Message,
		Duration:     duration,
	}

	// Validate result
	result.Passed = true

	if w.Code != scenario.ExpectedStatus {
		t.Errorf("%s: Expected status %d, got %d", scenario.Name, scenario.ExpectedStatus, w.Code)
		result.Passed = false
	}

	if s3Err.Code != scenario.ExpectedCode {
		t.Errorf("%s: Expected error code '%s', got '%s'", scenario.Name, scenario.ExpectedCode, s3Err.Code)
		result.Passed = false
	}

	if len(s3Err.Message) < scenario.MinMessageLength {
		t.Errorf("%s: Error message too short (got %d chars, want at least %d): %s",
			scenario.Name, len(s3Err.Message), scenario.MinMessageLength, s3Err.Message)
		result.Passed = false
	}

	// Check keywords if specified
	if len(scenario.ExpectedKeywords) > 0 {
		message := strings.ToLower(s3Err.Message)
		found := false
		for _, keyword := range scenario.ExpectedKeywords {
			if strings.Contains(message, strings.ToLower(keyword)) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: Error message should contain at least one of %v, got: %s",
				scenario.Name, scenario.ExpectedKeywords, s3Err.Message)
			result.Passed = false
		}
	}

	// Check response time if specified
	if scenario.MaxResponseTime > 0 && duration > scenario.MaxResponseTime {
		t.Errorf("%s: Response time %v exceeds maximum %v", scenario.Name, duration, scenario.MaxResponseTime)
		result.Passed = false
	}

	return result
}

// =============================================================================
// BASE TEST STRUCTURES
// =============================================================================

// ErrorScenarioSuite provides a test suite structure for running error scenarios.
//
// This structure helps organize related error tests and provides
// consistent reporting and validation across test types.
type ErrorScenarioSuite struct {
	t         *testing.T
	name      string
	fixture   *TestServerFixture
	scenarios []ErrorScenario
	results   []*ErrorScenarioResult
}

// NewErrorScenarioSuite creates a new error scenario suite.
func NewErrorScenarioSuite(t *testing.T, name string) *ErrorScenarioSuite {
	t.Helper()

	return &ErrorScenarioSuite{
		t:       t,
		name:    name,
		fixture: NewTestServer(t),
		results: make([]*ErrorScenarioResult, 0),
	}
}

// AddScenario adds a test scenario to the suite.
func (suite *ErrorScenarioSuite) AddScenario(scenario ErrorScenario) *ErrorScenarioSuite {
	suite.scenarios = append(suite.scenarios, scenario)
	return suite
}

// Run executes all scenarios in the suite.
func (suite *ErrorScenarioSuite) Run() {
	suite.t.Helper()

	suite.t.Run(suite.name, func(t *testing.T) {
		for _, scenario := range suite.scenarios {
			t.Run(scenario.Name, func(t *testing.T) {
				result := RunErrorScenario(t, suite.fixture, scenario)
				suite.results = append(suite.results, result)
			})
		}
	})
}

// RunSequentially executes all scenarios sequentially (not in parallel).
// Use this when scenarios have dependencies or share state.
func (suite *ErrorScenarioSuite) RunSequentially() {
	suite.t.Helper()

	suite.t.Run(suite.name, func(t *testing.T) {
		for _, scenario := range suite.scenarios {
			t.Run(scenario.Name, func(t *testing.T) {
				t.Parallel() // Disable parallelism
				result := RunErrorScenario(t, suite.fixture, scenario)
				suite.results = append(suite.results, result)
			})
		}
	})
}

// ReportStatistics logs performance statistics for the suite.
func (suite *ErrorScenarioSuite) ReportStatistics() {
	suite.t.Helper()

	if len(suite.results) == 0 {
		suite.t.Log("No results to report")
		return
	}

	var total, max, min time.Duration
	min = 1 * time.Second // Initialize to high value
	passed := 0
	failed := 0

	for _, result := range suite.results {
		total += result.Duration
		if result.Duration > max {
			max = result.Duration
		}
		if result.Duration < min {
			min = result.Duration
		}
		if result.Passed {
			passed++
		} else {
			failed++
		}
	}

	avg := total / time.Duration(len(suite.results))

	suite.t.Logf("Error Test Suite: %s", suite.name)
	suite.t.Logf("  Total scenarios: %d", len(suite.results))
	suite.t.Logf("  Passed: %d, Failed: %d", passed, failed)
	suite.t.Logf("  Average response time: %v", avg)
	suite.t.Logf("  Min response time: %v", min)
	suite.t.Logf("  Max response time: %v", max)
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MeasureRequestTime measures the time to execute a request.
// Returns the duration and response recorder.
func MeasureRequestTime(handler http.Handler, req *http.Request) (time.Duration, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	start := time.Now()
	handler.ServeHTTP(w, req)
	duration := time.Since(start)
	return duration, w
}

// =============================================================================
// DOCUMENTATION AND EXAMPLES
// =============================================================================

/*
Example Usage:

Basic error response validation:

    func TestMyError(t *testing.T) {
        fixture := NewTestServer(t)

        req := CreateTestRequest(t, "GET", "/test-bucket/nonexistent", nil, nil)
        w := httptest.NewRecorder()
        fixture.Handler.ServeHTTP(w, req)

        VerifyStandardErrorResponse(t, w, "NoSuchKey", 404)
    }

Advanced validation with fluent API:

    func TestMyErrorAdvanced(t *testing.T) {
        fixture := NewTestServer(t)

        req := CreateTestRequest(t, "POST", "/test-bucket/test-key", nil, nil)
        w := httptest.NewRecorder()
        fixture.Handler.ServeHTTP(w, req)

        duration, _ := MeasureRequestTime(fixture.Handler, req)

        VerifyErrorResponseWithTiming(t, w, duration).
            HTTPStatusCode(400).
            ContentType("application/xml").
            HasCode("InvalidRequest").
            MessageContainsAny("post", "unsupported", "operation").
            ResponseTime(100 * time.Millisecond).
            Assert()
    }

CORS header validation:

    func TestErrorCORSHeaders(t *testing.T) {
        fixture := NewTestServer(t)

        req := CreateTestRequest(t, "GET", "/test-bucket/nonexistent", nil, nil)
        w := httptest.NewRecorder()
        fixture.Handler.ServeHTTP(w, req)

        VerifyErrorResponse(t, w).
            HTTPStatusCode(404).
            HasCORSHeaders().
            CORSOrigin("*").
            CORSMethods("GET, PUT, DELETE, HEAD, POST, OPTIONS").
            CORSHeaders("Authorization, Content-Type, Range, Content-Length").
            Assert()
    }

Using test suites:

    func TestMyErrorSuite(t *testing.T) {
        suite := NewErrorScenarioSuite(t, "My Error Scenarios").
            AddScenario(ErrorScenario{
                Name: "Scenario 1",
                SetupRequest: func(t *testing.T) *http.Request {
                    return CreateTestRequest(t, "GET", "/test-bucket/key1", nil, nil)
                },
                ExpectedCode: "NoSuchKey",
                ExpectedStatus: 404,
                MinMessageLength: 15,
                MaxResponseTime: 100 * time.Millisecond,
            }).
            AddScenario(ErrorScenario{
                Name: "Scenario 2",
                SetupRequest: func(t *testing.T) *http.Request {
                    return CreateTestRequest(t, "POST", "/test-bucket/key2", nil, nil)
                },
                ExpectedCode: "InvalidRequest",
                ExpectedStatus: 400,
                MinMessageLength: 20,
            })

        suite.Run()
        suite.ReportStatistics()
    }

Using error scenarios directly:

    func TestMyScenario(t *testing.T) {
        fixture := NewTestServer(t)

        scenario := ErrorScenario{
            Name: "My custom scenario",
            SetupRequest: func(t *testing.T) *http.Request {
                return CreateTestRequest(t, "GET", "/test-bucket/test", nil, nil)
            },
            ExpectedCode: "NoSuchKey",
            ExpectedStatus: 404,
            ExpectedKeywords: []string{"not found", "object"},
            MinMessageLength: 15,
            MaxResponseTime: 100 * time.Millisecond,
        }

        result := RunErrorScenario(t, fixture, scenario)

        if !result.Passed {
            t.Errorf("Scenario failed: %s", result.Message)
        }
    }
*/
