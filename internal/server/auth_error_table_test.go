// Authentication error test cases demonstrating the table-driven test pattern
//
// This file demonstrates how to use the table-driven test pattern for authentication
// error scenarios, using the generic test table framework from testutil.
//
// # Pattern Overview
//
// The table-driven test pattern provides:
// 1. Clear test case definitions with descriptive names
// 2. Reusable test structures that can be extended
// 3. Type-safe error validation
// 4. Comprehensive inline documentation
//
// # Usage Pattern
//
// 1. Define input structure (AuthTestInput)
// 2. Create test table using TableTestCase
// 3. Use helper functions from testutil
// 4. Run table with RunTable function
package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/testutil"
)

// =============================================================================
// AUTHENTICATION TEST INPUT STRUCTURES
// =============================================================================

// AuthTestInput encapsulates inputs for authentication testing.
//
// This structure keeps test input data separate from test expectations.
// The expectation fields (ExpectError, ErrorContains, etc.) are set in
// the TableTestCase structure, not here.
//
// # Fields
//   - Description: Human-readable description of the test scenario
//   - SetupRequest: Function to configure the test request
//   - ValidateResponse: Function to validate the response
type AuthTestInput struct {
	Description      string
	SetupRequest     func(*http.Request)
	ValidateResponse func(*httptest.ResponseRecorder) error
}

// =============================================================================
// AUTHENTICATION ERROR TEST TABLES - MISSING AUTH HEADER
// =============================================================================

// TestAuthError_MissingAuthHeader tests requests with missing authentication headers.
//
// This test demonstrates the table-driven pattern for testing missing authentication
// scenarios using the generic test table framework.
func TestAuthError_MissingAuthHeader(t *testing.T) {
	// Define test cases using the table-driven pattern
	// Each case uses TableTestCase to define inputs and expectations clearly
	table := []testutil.TableTestCase[AuthTestInput, error]{
		{
			Name:        "request with valid auth header format succeeds",
			Description: "Tests that a properly formatted authenticated request is accepted",
			Input: AuthTestInput{
				Description: "Valid authentication header format present",
				SetupRequest: func(req *http.Request) {
					// Use TESTKEY instead of INVALID to avoid triggering invalid key error
					// IMPORTANT: Use UTC time to match mock handler's time parsing
					req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
					req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					if resp.Code != 200 {
						return fmt.Errorf("expected status 200, got %d", resp.Code)
					}
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
		{
			Name:        "request without auth header fails",
			Description: "Tests that missing Authorization header returns authentication error",
			Input: AuthTestInput{
				Description: "No Authorization header provided",
				SetupRequest: func(req *http.Request) {
					// No auth header set - date header alone is insufficient
					req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					if resp.Code != 403 {
						return fmt.Errorf("expected status 403, got %d", resp.Code)
					}
					// Validate S3 error format using testutil helpers
					if err := testutil.ValidateErrorCode(resp, "MissingAuthenticationToken"); err != nil {
						return fmt.Errorf("error code validation failed: %w", err)
					}
					// Validate error message contains expected substring (case-insensitive)
					if err := testutil.ValidateErrorMessage(resp, "Missing Authentication"); err != nil {
						return fmt.Errorf("error message validation failed: %w", err)
					}
					return nil
				},
			},
			// NOTE: This test validates the response, so we don't expect an error from the runner
			ExpectedError: nil,
			ExpectError:   false,
		},
		{
			Name:        "request with empty auth header fails",
			Description: "Tests that an empty Authorization header returns error",
			Input: AuthTestInput{
				Description: "Authorization header is present but empty",
				SetupRequest: func(req *http.Request) {
					req.Header.Set("Authorization", "")
					req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					if resp.Code != 403 {
						return fmt.Errorf("expected status 403, got %d", resp.Code)
					}
					// Validate that empty auth is treated as missing
					if err := testutil.ValidateErrorCode(resp, "MissingAuthenticationToken"); err != nil {
						return fmt.Errorf("error code validation failed: %w", err)
					}
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
	}

	// RunTable executes all test cases using the provided runner function
	// This function handles error checking, test reporting, and validation
	testutil.RunTable(t, table, func(input AuthTestInput) error {
		// Create mock handler that simulates ARMOR authentication behavior
		handler := createAuthMockHandler()

		// Build test request using testutil helpers
		req := testutil.NewTestRequest("GET", "/test-bucket/test-key", nil)

		// Apply test-specific request configuration
		if input.SetupRequest != nil {
			input.SetupRequest(req)
		}

		// Execute request and capture response
		resp := testutil.MakeRequest(handler, req)

		// Validate response using test-specific validation
		if input.ValidateResponse != nil {
			return input.ValidateResponse(resp)
		}

		return nil
	})
}

// =============================================================================
// AUTHENTICATION ERROR TEST TABLES - INVALID CREDENTIALS
// =============================================================================

// TestAuthError_InvalidCredentials tests requests with invalid credentials.
//
// This test demonstrates using the table pattern for testing various invalid
// credential scenarios including wrong keys and malformed credentials.
func TestAuthError_InvalidCredentials(t *testing.T) {
	table := []testutil.TableTestCase[AuthTestInput, error]{
		{
			Name:        "invalid access key rejected",
			Description: "Tests that an unknown access key is rejected",
			Input: AuthTestInput{
				Description: "Access key not found in credentials store",
				SetupRequest: func(req *http.Request) {
					// IMPORTANT: Must provide valid date header for credential validation to occur
					// Otherwise, the mock handler will return RequestExpired before checking credentials
					req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=INVALIDKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
					req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					if resp.Code != 403 {
						return fmt.Errorf("expected status 403, got %d", resp.Code)
					}
					// Validate error code for invalid access key
					if err := testutil.ValidateErrorCode(resp, "InvalidAccessKeyId"); err != nil {
						return fmt.Errorf("error code validation failed: %w", err)
					}
					// Validate error message
					if err := testutil.ValidateErrorMessage(resp, "does not exist"); err != nil {
						return fmt.Errorf("error message validation failed: %w", err)
					}
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
		{
			Name:        "malformed credential string rejected",
			Description: "Tests that malformed credential field is rejected",
			Input: AuthTestInput{
				Description: "Credential field does not match expected format",
				SetupRequest: func(req *http.Request) {
					// Malformed credential string - must have valid date for this to be checked
					req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=malformed, SignedHeaders=host, Signature=testsig")
					req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					if resp.Code != 403 {
						return fmt.Errorf("expected status 403, got %d", resp.Code)
					}
					// Validate error code for malformed credential
					if err := testutil.ValidateErrorCode(resp, "InvalidAccessKeyId"); err != nil {
						return fmt.Errorf("error code validation failed: %w", err)
					}
					// Validate error message contains "malformed"
					if err := testutil.ValidateErrorMessage(resp, "malformed"); err != nil {
						return fmt.Errorf("error message validation failed: %w", err)
					}
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
		{
			Name:        "signature mismatch rejected",
			Description: "Tests that incorrect signature is rejected",
			Input: AuthTestInput{
				Description: "Calculated signature does not match provided signature",
				SetupRequest: func(req *http.Request) {
					// Wrong signature - must have valid date for signature check to occur
					req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=wrongsignature")
					req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					if resp.Code != 403 {
						return fmt.Errorf("expected status 403, got %d", resp.Code)
					}
					// Validate signature mismatch error code
					if err := testutil.ValidateErrorCode(resp, "SignatureDoesNotMatch"); err != nil {
						return fmt.Errorf("error code validation failed: %w", err)
					}
					// Validate error message contains "signature"
					if err := testutil.ValidateErrorMessage(resp, "signature"); err != nil {
						return fmt.Errorf("error message validation failed: %w", err)
					}
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
	}

	testutil.RunTable(t, table, func(input AuthTestInput) error {
		handler := createAuthMockHandler()
		req := testutil.NewTestRequest("GET", "/test-bucket/test-key", nil)

		if input.SetupRequest != nil {
			input.SetupRequest(req)
		}

		resp := testutil.MakeRequest(handler, req)

		if input.ValidateResponse != nil {
			return input.ValidateResponse(resp)
		}

		return nil
	})
}

// =============================================================================
// AUTHENTICATION ERROR TEST TABLES - EXPIRED TOKEN
// =============================================================================

// TestAuthError_ExpiredToken tests requests with expired or invalid date headers.
//
// This test demonstrates testing temporal validation scenarios using the
// table-driven pattern, including expired and future-dated requests.
func TestAuthError_ExpiredToken(t *testing.T) {
	// Calculate timestamps for temporal testing
	now := time.Now().UTC()
	expiredTime := now.Add(-20 * time.Minute) // 20 minutes in the past
	futureTime := now.Add(20 * time.Minute)  // 20 minutes in the future

	table := []testutil.TableTestCase[AuthTestInput, error]{
		{
			Name:        "request with expired date header rejected",
			Description: "Tests that requests older than 15 minutes are rejected",
			Input: AuthTestInput{
				Description: "Request timestamp is outside allowed time window",
				SetupRequest: func(req *http.Request) {
					req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
					req.Header.Set("X-Amz-Date", expiredTime.Format("20060102T150405Z"))
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					if resp.Code != 403 {
						return fmt.Errorf("expected status 403, got %d", resp.Code)
					}
					// Validate expiration error code
					if err := testutil.ValidateErrorCode(resp, "RequestExpired"); err != nil {
						return fmt.Errorf("error code validation failed: %w", err)
					}
					// Validate error message contains "expired"
					if err := testutil.ValidateErrorMessage(resp, "expired"); err != nil {
						return fmt.Errorf("error message validation failed: %w", err)
					}
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
		{
			Name:        "request with future date header rejected",
			Description: "Tests that future-dated requests are rejected",
			Input: AuthTestInput{
				Description: "Request timestamp is in the future",
				SetupRequest: func(req *http.Request) {
					req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
					req.Header.Set("X-Amz-Date", futureTime.Format("20060102T150405Z"))
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					if resp.Code != 403 {
						return fmt.Errorf("expected status 403, got %d", resp.Code)
					}
					// Validate expiration error code (future dates also trigger RequestExpired)
					if err := testutil.ValidateErrorCode(resp, "RequestExpired"); err != nil {
						return fmt.Errorf("error code validation failed: %w", err)
					}
					// Validate error message contains "expired"
					if err := testutil.ValidateErrorMessage(resp, "expired"); err != nil {
						return fmt.Errorf("error message validation failed: %w", err)
					}
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
		{
			Name:        "request with missing date header rejected",
			Description: "Tests that missing X-Amz-Date header is rejected",
			Input: AuthTestInput{
				Description: "X-Amz-Date header is required for authentication",
				SetupRequest: func(req *http.Request) {
					req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
					// No date header set
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					if resp.Code != 403 {
						return fmt.Errorf("expected status 403, got %d", resp.Code)
					}
					// Validate missing date header error code
					if err := testutil.ValidateErrorCode(resp, "MissingDateHeader"); err != nil {
						return fmt.Errorf("error code validation failed: %w", err)
					}
					// Validate error message contains "date"
					if err := testutil.ValidateErrorMessage(resp, "date"); err != nil {
						return fmt.Errorf("error message validation failed: %w", err)
					}
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
		{
			// Success case: valid date header within acceptable window
			Name:        "request with valid date header succeeds",
			Description: "Tests that requests within time window are accepted",
			Input: AuthTestInput{
				Description: "X-Amz-Date header is within allowed time window",
				SetupRequest: func(req *http.Request) {
					req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
					req.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					// This should succeed with 200 since all auth parameters are valid
					if resp.Code != 200 {
						return fmt.Errorf("expected status 200, got %d", resp.Code)
					}
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
	}

	testutil.RunTable(t, table, func(input AuthTestInput) error {
		handler := createAuthMockHandler()
		req := testutil.NewTestRequest("GET", "/test-bucket/test-key", nil)

		if input.SetupRequest != nil {
			input.SetupRequest(req)
		}

		resp := testutil.MakeRequest(handler, req)

		if input.ValidateResponse != nil {
			return input.ValidateResponse(resp)
		}

		return nil
	})
}

// =============================================================================
// HTTP INTEGRATION TESTS
// =============================================================================

// TestAuthError_HTTPIntegration tests authentication errors via HTTP responses.
//
// This test demonstrates end-to-end testing of authentication errors by
// simulating HTTP request/response cycles and validating S3 error format.
func TestAuthError_HTTPIntegration(t *testing.T) {
	table := []testutil.TableTestCase[AuthTestInput, error]{
		{
			Name:        "missing auth returns 403 with S3 error format",
			Description: "Tests that missing auth returns proper S3 error response",
			Input: AuthTestInput{
				Description: "Validate complete S3 error response structure",
				SetupRequest: func(req *http.Request) {
					// No auth header
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					// Use comprehensive assertion helpers from testutil
					testutil.AssertAuthenticationError(t, resp)
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
		{
			Name:        "invalid credentials return 403 with correct error",
			Description: "Tests that invalid credentials return proper error response",
			Input: AuthTestInput{
				Description: "Validate error response for invalid credentials",
				SetupRequest: func(req *http.Request) {
					req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=INVALID/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
					req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					// Validate status code
					if err := testutil.ValidateStatusCode(resp, 403); err != nil {
						return fmt.Errorf("status validation failed: %w", err)
					}
					// Validate content type
					if err := testutil.ValidateContentType(resp, "application/xml"); err != nil {
						return fmt.Errorf("content type validation failed: %w", err)
					}
					// Validate error code
					if err := testutil.ValidateErrorCode(resp, "InvalidAccessKeyId"); err != nil {
						return fmt.Errorf("error code validation failed: %w", err)
					}
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
		{
			Name:        "valid authentication returns 200",
			Description: "Tests that valid authentication succeeds",
			Input: AuthTestInput{
				Description: "Validate successful authentication response",
				SetupRequest: func(req *http.Request) {
					req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
					req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				},
				ValidateResponse: func(resp *httptest.ResponseRecorder) error {
					// Valid authentication should return 200
					if err := testutil.ValidateStatusCode(resp, 200); err != nil {
						return fmt.Errorf("status validation failed: %w", err)
					}
					return nil
				},
			},
			ExpectedError: nil,
			ExpectError:   false,
		},
	}

	testutil.RunTable(t, table, func(input AuthTestInput) error {
		handler := createAuthMockHandler()
		req := testutil.NewTestRequest("GET", "/test-bucket/test-key", nil)

		if input.SetupRequest != nil {
			input.SetupRequest(req)
		}

		resp := testutil.MakeRequest(handler, req)

		if input.ValidateResponse != nil {
			return input.ValidateResponse(resp)
		}

		return nil
	})
}

// =============================================================================
// MOCK HANDLERS
// =============================================================================

// createAuthMockHandler creates a test HTTP handler that simulates ARMOR authentication responses.
//
// This helper function creates mock HTTP handlers that simulate the ARMOR server's
// authentication behavior for testing purposes. It returns appropriate S3-formatted
// error responses based on the authentication headers present.
//
// Returns:
//   - http.Handler: Mock HTTP handler that simulates ARMOR authentication
func createAuthMockHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		date := r.Header.Get("X-Amz-Date")

		// Check for missing authentication
		if auth == "" || auth == " " {
			sendS3Error(w, "MissingAuthenticationToken", "Missing Authentication Token")
			return
		}

		// Check for missing date header
		if date == "" {
			sendS3Error(w, "MissingDateHeader", "Missing X-Amz-Date header")
			return
		}

		// Check for expired date header
		parsedTime, err := time.Parse("20060102T150405Z", date)
		if err != nil {
			sendS3Error(w, "InvalidDateFormat", "Invalid date format in X-Amz-Date header")
			return
		}

		now := time.Now().UTC()
		// Check if date is outside 15-minute window
		if parsedTime.Before(now.Add(-15*time.Minute)) || parsedTime.After(now.Add(15*time.Minute)) {
			sendS3Error(w, "RequestExpired", "Request has expired")
			return
		}

		// Check for invalid access key
		if contains(auth, "INVALID") {
			sendS3Error(w, "InvalidAccessKeyId", "The AWS Access Key Id you provided does not exist in our records")
			return
		}

		// Check for malformed credential
		if contains(auth, "malformed") {
			sendS3Error(w, "InvalidAccessKeyId", "Malformed credential string")
			return
		}

		// Check for wrong signature
		if contains(auth, "wrongsignature") {
			sendS3Error(w, "SignatureDoesNotMatch", "The request signature we calculated does not match the signature you provided")
			return
		}

		// Success case - valid authentication
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

// sendS3Error sends a properly formatted S3 error response.
//
// This helper function constructs S3-compatible XML error responses,
// matching the format used by the real ARMOR server.
//
// Parameters:
//   - w: HTTP response writer
//   - code: S3 error code (e.g., "MissingAuthenticationToken")
//   - message: Human-readable error message
func sendS3Error(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusForbidden)
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message></Error>`, code, message)
	w.Write([]byte(xml))
}
