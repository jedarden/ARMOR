// Package server provides base test infrastructure for ARMOR error response testing.
//
// # ARMOR Error Testing Framework
//
// This file provides the foundational test infrastructure for ARMOR's S3-compatible
// error responses. It integrates the existing error patterns, test helpers, and
// server setup into a cohesive, extensible testing framework.
//
// # Architecture Overview
//
// The error testing framework consists of three layers:
//
// 1. **Base Infrastructure Layer** (this file and related files):
//   - Error test patterns and types (error_test_patterns.go)
//   - Test server setup (test_server_setup_test.go)
//   - Request/response helpers (test_request_validation_helpers.go)
//   - Base test tables (test_tables.go in validate package)
//
// 2. **ARMOR-Specific Layer**:
//   - ARMOR error test tables (ARMORErrorTestTables in this file)
//   - S3-specific error patterns (CommonErrorPatterns, AuthErrorPatterns)
//   - ARMOR response validation helpers
//
// 3. **Extension Layer**:
//   - Custom error type tests (auth, validation, etc.)
//   - Domain-specific error scenarios
//   - Integration test patterns
//
// # Quick Start
//
// Run basic error tests:
//
//	func TestARMORErrorResponses(t *testing.T) {
//	    for _, tc := range ARMORErrorTestTables.BasicErrorTests() {
//	        t.Run(tc.Name, func(t *testing.T) {
//	            server := NewSingleErrorServer(tc.StatusCode, tc.ErrorCode, tc.Message)
//	            defer server.Close()
//
//	            resp, err := MakeGETRequest(server.URL, tc.Path)
//	            if err != nil {
//	                t.Fatalf("Request failed: %v", err)
//	            }
//	            ValidateErrorResponse(t, resp, tc.StatusCode, tc.ErrorCode)
//	        })
//	    }
//	}
//
// Test authentication errors:
//
//	func TestARMORAuthErrors(t *testing.T) {
//	    for _, tc := range ARMORErrorTestTables.AuthenticationErrors() {
//	        t.Run(tc.Name, func(t *testing.T) {
//	            // Test authentication scenarios
//	        })
//	    }
//	}
//
// # Available Test Tables
//
//   - ARMORErrorTestTables.BasicErrorTests() - Common S3 error scenarios
//   - ARMORErrorTestTables.AuthenticationErrors() - Auth-specific errors
//   - ARMORErrorTestTables.ValidationErrors() - Input validation errors
//   - ARMORErrorTestTables.ServerErrors() - 5xx server errors
//   - ARMORErrorTestTables.S3ProtocolErrors() - S3 protocol compliance
//
// # Extension Pattern
//
// To create custom error tests:
//
// 1. Define test cases using ARMORErrorTestCase
// 2. Use ARMORTestServer for setup
// 3. Use ARMOR request helpers for making requests
// 4. Use ARMOR validation helpers for assertions
//
// Example:
//
//	customTests := []ARMORErrorTestCase{
//	    {Name: "Custom 404", StatusCode: 404, ErrorCode: "CustomNotFound"},
//	    {Name: "Custom 403", StatusCode: 403, ErrorCode: "CustomDenied"},
//	}
//	for _, tc := range customTests {
//	    t.Run(tc.Name, testARMORErrorScenario(tc))
//	}
package server

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// ARMOR ERROR TEST CASE STRUCTURE
// =============================================================================
// This is the primary test case structure for ARMOR error testing.
// It integrates S3 error patterns with ARMOR-specific testing needs.
//
// Design Philosophy:
// - Single source of truth for test case data
// - Clear separation between test data and test logic
// - Easy to create and extend
// - Type-safe and self-documenting
// =============================================================================

// ARMORErrorTestCase represents a single ARMOR error test case.
//
// This type encapsulates all information needed to test an ARMOR error scenario.
// It's used throughout the test framework for consistency and reusability.
//
// Use this type when:
//   - Creating ARMOR-specific error tests
//   - Building custom test tables
//   - Extending existing test tables
//   - Documenting expected error behaviors
//
// Example (basic usage):
//
//	tc := ARMORErrorTestCase{
//	    Name:       "Object not found",
//	    StatusCode: 404,
//	    ErrorCode:  "NoSuchKey",
//	    Message:    "The specified key does not exist",
//	    Path:       "/armor/blobs/missing-object.dat",
//	}
//
// Example (with validation):
//
//	tc := ARMORErrorTestCase{
//	    Name:             "Access denied without credentials",
//	    StatusCode:       403,
//	    ErrorCode:        "AccessDenied",
//	    Message:          "Access Denied",
//	    Path:             "/armor/blobs/protected.dat",
//	    ExpectXMLStructure: true,
//	    MinResponseTime:  100 * time.Millisecond,
//	    MaxResponseTime:  500 * time.Millisecond,
//	    Headers:          map[string]string{"WWW-Authenticate": "AWS4-HMAC-SHA256"},
//	}
//
// Example (from predefined pattern):
//
//	pattern := CommonErrorPatterns.ResourceNotFound
//	tc := ARMORErrorTestCase{
//	    Name:       pattern.Name,
//	    StatusCode: pattern.ExpectedStatus,
//	    ErrorCode:  pattern.ExpectedCode,
//	    Message:    pattern.ExpectedMessage,
//	    Path:       "/armor/blobs/test.dat",
//	}
type ARMORErrorTestCase struct {
	// Name is the test case name for identification and test reporting
	Name string

	// Description is an optional detailed explanation of what is being tested
	Description string

	// StatusCode is the expected HTTP status code (e.g., 404, 403, 500)
	StatusCode int

	// ErrorCode is the expected S3 error code (e.g., "NoSuchKey", "AccessDenied")
	ErrorCode string

	// Message is the expected error message text or keywords
	Message string

	// Path is the request path that triggers this error (e.g., "/armor/blobs/file.dat")
	Path string

	// Method is the HTTP method (default: "GET")
	Method string

	// Headers are expected response headers
	Headers map[string]string

	// Body is the optional request body for POST/PUT requests
	Body string

	// QueryParams are optional query parameters
	QueryParams map[string]string

	// ExpectXMLStructure indicates whether the response should be valid S3 XML
	ExpectXMLStructure bool

	// MinResponseTime is the minimum expected response duration
	MinResponseTime time.Duration

	// MaxResponseTime is the maximum expected response duration
	MaxResponseTime time.Duration

	// Category is an optional category for organizing tests
	Category string

	// Tags are optional tags for filtering and selecting tests
	Tags []string

	// SkipReason can be set to skip this test with a reason
	SkipReason string

	// SetupFunc is an optional custom setup function
	SetupFunc func(*testing.T, *ConfigurableErrorServer)

	// ValidateFunc is an optional custom validation function
	ValidateFunc func(*testing.T, *http.Response)
}

// ARMORErrorTestTable represents a collection of ARMOR error test cases.
//
// This type groups related test cases together for organized testing.
// Tables can be predefined, custom, or extended from existing tables.
//
// Use this type when:
//   - Creating reusable ARMOR error test tables
//   - Organizing tests by error type or scenario
//   - Sharing test tables across multiple test files
//
// Example (creating a table):
//
//	table := ARMORErrorTestTable{
//	    Name:        "Authentication Errors",
//	    Description: "Tests for ARMOR authentication error scenarios",
//	    TestCases: []ARMORErrorTestCase{
//	        {Name: "No credentials", StatusCode: 403, ErrorCode: "AccessDenied"},
//	        {Name: "Invalid credentials", StatusCode: 403, ErrorCode: "InvalidAccessKeyId"},
//	    },
//	}
//
// Example (extending a table):
//
//	base := ARMORErrorTestTables.BasicErrorTests()
//	custom := ARMORErrorTestCase{Name: "Custom error", StatusCode: 418, ErrorCode: "ImATeapot"}
//	extended := append(base.TestCases, custom)
type ARMORErrorTestTable struct {
	// Name is the table name for identification
	Name string

	// Description explains the purpose and scope of this test table
	Description string

	// TestCases is the collection of error test cases
	TestCases []ARMORErrorTestCase

	// SetupFunc is an optional setup function for all tests in the table
	SetupFunc func(*testing.T)

	// TeardownFunc is an optional teardown function for all tests in the table
	TeardownFunc func(*testing.T)
}

// =============================================================================
// ARMOR ERROR TEST TABLES
// =============================================================================
// These are predefined test tables for common ARMOR error scenarios.
// They are ready to use and can be extended with custom test cases.
//
// Design Philosophy:
//   - Coverage: Cover the most common ARMOR error scenarios
//   - Consistency: Use consistent patterns across all tables
//   - Extensibility: Easy to extend with custom cases
//   - Documentation: Clear descriptions for each test
// =============================================================================

// ARMORErrorTestTables provides predefined test tables for ARMOR error testing.
//
// This is the primary entry point for ARMOR error testing. It provides
// comprehensive test tables covering all common error scenarios.
//
// Use this collection when:
//   - Running comprehensive ARMOR error tests
//   - Validating ARMOR's S3-compatible error responses
//   - Testing error handling in ARMOR clients
//   - Debugging error response issues
//
// Example:
//
//	for _, tc := range ARMORErrorTestTables.BasicErrorTests() {
//	    t.Run(tc.Name, testARMORErrorScenario(tc))
//	}
//
// Example (filtering by category):
//
//	allTests := ARMORErrorTestTables.AllTests()
//	authTests := filterARMORTestsByCategory(allTests, "Auth")
//	for _, tc := range authTests {
//	    t.Run(tc.Name, testARMORErrorScenario(tc))
//	}
var ARMORErrorTestTables = struct {
	// BasicErrorTests returns test cases for common S3 error scenarios
	BasicErrorTests func() ARMORErrorTestTable

	// AuthenticationErrors returns test cases for authentication error scenarios
	AuthenticationErrors func() ARMORErrorTestTable

	// ValidationErrors returns test cases for input validation error scenarios
	ValidationErrors func() ARMORErrorTestTable

	// ServerErrors returns test cases for server error scenarios
	ServerErrors func() ARMORErrorTestTable

	// S3ProtocolErrors returns test cases for S3 protocol compliance
	S3ProtocolErrors func() ARMORErrorTestTable

	// CORSHeaders returns test cases for CORS header validation
	CORSHeaders func() ARMORErrorTestTable

	// AllTests returns all ARMOR error test cases combined
	AllTests func() ARMORErrorTestTable
}{
	// BasicErrorTests provides test cases for common S3 error scenarios
	BasicErrorTests: func() ARMORErrorTestTable {
		return ARMORErrorTestTable{
			Name:        "Basic S3 Error Responses",
			Description: "Tests for common S3-compatible error responses in ARMOR",
			TestCases: []ARMORErrorTestCase{
				{
					Name:              "NoSuchKey - object not found",
					Description:       "Tests 404 response when requesting non-existent object",
					StatusCode:        404,
					ErrorCode:         ErrorCodeNoSuchKey,
					Message:           "The specified key does not exist",
					Path:              "/armor/blobs/non-existent-file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   500 * time.Millisecond,
					Category:          "NotFound",
					Tags:              []string{"s3", "404", "basic"},
				},
				{
					Name:              "AccessDenied - no permissions",
					Description:       "Tests 403 response when user lacks permission",
					StatusCode:        403,
					ErrorCode:         ErrorCodeAccessDenied,
					Message:           "Access Denied",
					Path:              "/armor/blobs/protected-file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   300 * time.Millisecond,
					Category:          "Auth",
					Tags:              []string{"s3", "403", "auth"},
					Headers: map[string]string{
						"Content-Type": "application/xml",
					},
				},
				{
					Name:              "InvalidRequest - malformed request",
					Description:       "Tests 400 response for malformed requests",
					StatusCode:        400,
					ErrorCode:         ErrorCodeInvalidRequest,
					Message:           "The request is invalid or malformed",
					Path:              "/armor/blobs/file.dat?invalid=param",
					ExpectXMLStructure: true,
					MaxResponseTime:   200 * time.Millisecond,
					Category:          "Validation",
					Tags:              []string{"s3", "400", "validation"},
				},
				{
					Name:              "InternalError - server error",
					Description:       "Tests 500 response for internal server errors",
					StatusCode:        500,
					ErrorCode:         ErrorCodeInternalError,
					Message:           "Internal server error",
					Path:              "/armor/blobs/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   1000 * time.Millisecond,
					Category:          "Server",
					Tags:              []string{"s3", "500", "internal"},
				},
			},
		}
	},

	// AuthenticationErrors provides test cases for authentication scenarios
	AuthenticationErrors: func() ARMORErrorTestTable {
		return ARMORErrorTestTable{
			Name:        "Authentication Error Responses",
			Description: "Tests for ARMOR authentication and authorization error scenarios",
			TestCases: []ARMORErrorTestCase{
				{
					Name:              "Missing authentication token",
					Description:       "Tests error when Authorization header is missing",
					StatusCode:        403,
					ErrorCode:         ErrorCodeMissingAuthenticationToken,
					Message:           "Missing Authentication Token",
					Path:              "/armor/blobs/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   250 * time.Millisecond,
					Category:          "Auth",
					Tags:              []string{"auth", "missing", "token"},
				},
				{
					Name:              "Invalid access key ID",
					Description:       "Tests error when access key ID is invalid",
					StatusCode:        403,
					ErrorCode:         ErrorCodeInvalidAccessKeyId,
					Message:           "The AWS Access Key Id you provided does not exist in our records",
					Path:              "/armor/blobs/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   300 * time.Millisecond,
					Category:          "Auth",
					Tags:              []string{"auth", "invalid", "key"},
				},
				{
					Name:              "Signature does not match",
					Description:       "Tests error when request signature is incorrect",
					StatusCode:        403,
					ErrorCode:         ErrorCodeSignatureDoesNotMatch,
					Message:           "The request signature we calculated does not match the signature you provided",
					Path:              "/armor/blobs/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   350 * time.Millisecond,
					Category:          "Auth",
					Tags:              []string{"auth", "signature", "mismatch"},
				},
				{
					Name:              "Request expired",
					Description:       "Tests error when request timestamp is too old",
					StatusCode:        403,
					ErrorCode:         ErrorCodeRequestExpired,
					Message:           "Request has expired",
					Path:              "/armor/blobs/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   300 * time.Millisecond,
					Category:          "Auth",
					Tags:              []string{"auth", "expired", "timestamp"},
				},
			},
		}
	},

	// ValidationErrors provides test cases for input validation scenarios
	ValidationErrors: func() ARMORErrorTestTable {
		return ARMORErrorTestTable{
			Name:        "Validation Error Responses",
			Description: "Tests for ARMOR input validation error scenarios",
			TestCases: []ARMORErrorTestCase{
				{
					Name:              "Unsupported media type",
					Description:       "Tests 415 error for unsupported content types",
					StatusCode:        415,
					ErrorCode:         ErrorCodeUnsupportedMediaType,
					Message:           "The content type is not supported",
					Path:              "/armor/blobs/file.dat",
					Method:            "PUT",
					Body:              "test data",
					ExpectXMLStructure: true,
					MaxResponseTime:   200 * time.Millisecond,
					Category:          "Validation",
					Tags:              []string{"validation", "content-type", "415"},
				},
				{
					Name:              "Method not allowed",
					Description:       "Tests 405 error for unsupported HTTP methods",
					StatusCode:        405,
					ErrorCode:         ErrorCodeMethodNotAllowed,
					Message:           "The specified method is not allowed against this resource",
					Path:              "/armor/blobs/file.dat",
					Method:            "POST",
					ExpectXMLStructure: true,
					MaxResponseTime:   200 * time.Millisecond,
					Category:          "Validation",
					Tags:              []string{"validation", "method", "405"},
				},
			},
		}
	},

	// ServerErrors provides test cases for server error scenarios
	ServerErrors: func() ARMORErrorTestTable {
		return ARMORErrorTestTable{
			Name:        "Server Error Responses",
			Description: "Tests for ARMOR server error scenarios (5xx)",
			TestCases: []ARMORErrorTestCase{
				{
					Name:              "Service unavailable",
					Description:       "Tests 503 response when service is temporarily unavailable",
					StatusCode:        503,
					ErrorCode:         ErrorCodeInternalError,
					Message:           "Service Unavailable",
					Path:              "/armor/blobs/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   2000 * time.Millisecond,
					Category:          "Server",
					Tags:              []string{"server", "503", "unavailable"},
					Headers: map[string]string{
						"Retry-After": "60",
					},
				},
			},
		}
	},

	// S3ProtocolErrors provides test cases for S3 protocol compliance
	S3ProtocolErrors: func() ARMORErrorTestTable {
		return ARMORErrorTestTable{
			Name:        "S3 Protocol Compliance",
			Description: "Tests for S3 protocol compliance in error responses",
			TestCases: []ARMORErrorTestCase{
				{
					Name:              "Bucket not found (S3 protocol)",
					Description:       "Tests S3-compliant bucket not found error",
					StatusCode:        404,
					ErrorCode:         "NoSuchBucket",
					Message:           "The specified bucket does not exist",
					Path:              "/non-existent-bucket/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   500 * time.Millisecond,
					Category:          "S3Protocol",
					Tags:              []string{"s3", "protocol", "bucket"},
				},
			},
		}
	},

	// CORSHeaders provides test cases for CORS header validation
	CORSHeaders: func() ARMORErrorTestTable {
		return ARMORErrorTestTable{
			Name:        "CORS Header Validation",
			Description: "Tests for CORS headers in error responses",
			TestCases: []ARMORErrorTestCase{
				{
					Name:        "CORS headers on 404 error",
					Description: "Tests that CORS headers are present on error responses",
					StatusCode:   404,
					ErrorCode:    ErrorCodeNoSuchKey,
					Message:      "Not found",
					Path:         "/armor/blobs/file.dat",
					Category:     "CORS",
					Tags:         []string{"cors", "headers"},
					Headers: map[string]string{
						"Access-Control-Allow-Origin":  "*",
						"Access-Control-Allow-Methods": "GET, HEAD, PUT",
					},
				},
			},
		}
	},

		// AllTests returns all ARMOR error test cases combined
		AllTests: func() ARMORErrorTestTable {
			// Declare local functions to avoid initialization cycle
			getBasic := func() ARMORErrorTestTable {
				return ARMORErrorTestTable{
					Name:        "Basic S3 Error Responses",
					Description: "Tests for common S3-compatible error responses in ARMOR",
					TestCases: []ARMORErrorTestCase{
						{
							Name:              "NoSuchKey - object not found",
							Description:       "Tests 404 response when requesting non-existent object",
							StatusCode:        404,
							ErrorCode:         ErrorCodeNoSuchKey,
							Message:           "The specified key does not exist",
							Path:              "/armor/blobs/non-existent-file.dat",
							ExpectXMLStructure: true,
							MaxResponseTime:   500 * time.Millisecond,
							Category:          "NotFound",
							Tags:              []string{"s3", "404", "basic"},
						},
						{
							Name:              "AccessDenied - no permissions",
							Description:       "Tests 403 response when user lacks permission",
							StatusCode:        403,
							ErrorCode:         ErrorCodeAccessDenied,
							Message:           "Access Denied",
							Path:              "/armor/blobs/protected-file.dat",
							ExpectXMLStructure: true,
							MaxResponseTime:   300 * time.Millisecond,
							Category:          "Auth",
							Tags:              []string{"s3", "403", "auth"},
							Headers: map[string]string{
								"Content-Type": "application/xml",
							},
						},
						{
							Name:              "InvalidRequest - malformed request",
							Description:       "Tests 400 response for malformed requests",
							StatusCode:        400,
							ErrorCode:         ErrorCodeInvalidRequest,
							Message:           "The request is invalid or malformed",
							Path:              "/armor/blobs/file.dat?invalid=param",
							ExpectXMLStructure: true,
							MaxResponseTime:   200 * time.Millisecond,
							Category:          "Validation",
							Tags:              []string{"s3", "400", "validation"},
						},
						{
							Name:              "InternalError - server error",
							Description:       "Tests 500 response for internal server errors",
							StatusCode:        500,
							ErrorCode:         ErrorCodeInternalError,
							Message:           "Internal server error",
							Path:              "/armor/blobs/file.dat",
							ExpectXMLStructure: true,
							MaxResponseTime:   1000 * time.Millisecond,
							Category:          "Server",
							Tags:              []string{"s3", "500", "internal"},
						},
					},
				}
			}
			getAuth := func() ARMORErrorTestTable {
				return ARMORErrorTestTable{
					Name:        "Authentication Error Responses",
					Description: "Tests for ARMOR authentication and authorization error scenarios",
					TestCases: []ARMORErrorTestCase{
						{
							Name:              "Missing authentication token",
							Description:       "Tests error when Authorization header is missing",
							StatusCode:        403,
							ErrorCode:         ErrorCodeMissingAuthenticationToken,
							Message:           "Missing Authentication Token",
							Path:              "/armor/blobs/file.dat",
							ExpectXMLStructure: true,
							MaxResponseTime:   250 * time.Millisecond,
							Category:          "Auth",
							Tags:              []string{"auth", "missing", "token"},
						},
						{
							Name:              "Invalid access key ID",
							Description:       "Tests error when access key ID is invalid",
							StatusCode:        403,
							ErrorCode:         ErrorCodeInvalidAccessKeyId,
							Message:           "The AWS Access Key Id you provided does not exist in our records",
							Path:              "/armor/blobs/file.dat",
							ExpectXMLStructure: true,
							MaxResponseTime:   300 * time.Millisecond,
							Category:          "Auth",
							Tags:              []string{"auth", "invalid", "key"},
						},
						{
							Name:              "Signature does not match",
							Description:       "Tests error when request signature is incorrect",
							StatusCode:        403,
							ErrorCode:         ErrorCodeSignatureDoesNotMatch,
							Message:           "The request signature we calculated does not match the signature you provided",
							Path:              "/armor/blobs/file.dat",
							ExpectXMLStructure: true,
							MaxResponseTime:   350 * time.Millisecond,
							Category:          "Auth",
							Tags:              []string{"auth", "signature", "mismatch"},
						},
						{
							Name:              "Request expired",
							Description:       "Tests error when request timestamp is too old",
							StatusCode:        403,
							ErrorCode:         ErrorCodeRequestExpired,
							Message:           "Request has expired",
							Path:              "/armor/blobs/file.dat",
							ExpectXMLStructure: true,
							MaxResponseTime:   300 * time.Millisecond,
							Category:          "Auth",
							Tags:              []string{"auth", "expired", "timestamp"},
						},
					},
				}
			}
			getValidation := func() ARMORErrorTestTable {
				return ARMORErrorTestTable{
					Name:        "Validation Error Responses",
					Description: "Tests for ARMOR input validation error scenarios",
					TestCases: []ARMORErrorTestCase{
						{
							Name:              "Unsupported media type",
							Description:       "Tests 415 error for unsupported content types",
							StatusCode:        415,
							ErrorCode:         ErrorCodeUnsupportedMediaType,
							Message:           "The content type is not supported",
							Path:              "/armor/blobs/file.dat",
							Method:            "PUT",
							Body:              "test data",
							ExpectXMLStructure: true,
							MaxResponseTime:   200 * time.Millisecond,
							Category:          "Validation",
							Tags:              []string{"validation", "content-type", "415"},
						},
						{
							Name:              "Method not allowed",
							Description:       "Tests 405 error for unsupported HTTP methods",
							StatusCode:        405,
							ErrorCode:         ErrorCodeMethodNotAllowed,
							Message:           "The specified method is not allowed against this resource",
							Path:              "/armor/blobs/file.dat",
							Method:            "POST",
							ExpectXMLStructure: true,
							MaxResponseTime:   200 * time.Millisecond,
							Category:          "Validation",
							Tags:              []string{"validation", "method", "405"},
						},
					},
				}
			}
			getServer := func() ARMORErrorTestTable {
				return ARMORErrorTestTable{
					Name:        "Server Error Responses",
					Description: "Tests for ARMOR server error scenarios (5xx)",
					TestCases: []ARMORErrorTestCase{
						{
							Name:              "Service unavailable",
							Description:       "Tests 503 response when service is temporarily unavailable",
							StatusCode:        503,
							ErrorCode:         ErrorCodeInternalError,
							Message:           "Service Unavailable",
							Path:              "/armor/blobs/file.dat",
							ExpectXMLStructure: true,
							MaxResponseTime:   2000 * time.Millisecond,
							Category:          "Server",
							Tags:              []string{"server", "503", "unavailable"},
							Headers: map[string]string{
								"Retry-After": "60",
							},
						},
					},
				}
			}
			getProtocol := func() ARMORErrorTestTable {
				return ARMORErrorTestTable{
					Name:        "S3 Protocol Compliance",
					Description: "Tests for S3 protocol compliance in error responses",
					TestCases: []ARMORErrorTestCase{
						{
							Name:              "Bucket not found (S3 protocol)",
							Description:       "Tests S3-compliant bucket not found error",
							StatusCode:        404,
							ErrorCode:         "NoSuchBucket",
							Message:           "The specified bucket does not exist",
							Path:              "/non-existent-bucket/file.dat",
							ExpectXMLStructure: true,
							MaxResponseTime:   500 * time.Millisecond,
							Category:          "S3Protocol",
							Tags:              []string{"s3", "protocol", "bucket"},
						},
					},
				}
			}
			getCORS := func() ARMORErrorTestTable {
				return ARMORErrorTestTable{
					Name:        "CORS Header Validation",
					Description: "Tests for CORS headers in error responses",
					TestCases: []ARMORErrorTestCase{
						{
							Name:        "CORS headers on 404 error",
							Description: "Tests that CORS headers are present on error responses",
							StatusCode:   404,
							ErrorCode:    ErrorCodeNoSuchKey,
							Message:      "Not found",
							Path:         "/armor/blobs/file.dat",
							Category:     "CORS",
							Tags:         []string{"cors", "headers"},
							Headers: map[string]string{
								"Access-Control-Allow-Origin":  "*",
								"Access-Control-Allow-Methods": "GET, HEAD, PUT",
							},
						},
					},
				}
			}

			basic := getBasic()
			auth := getAuth()
			validation := getValidation()
			server := getServer()
			protocol := getProtocol()
			cors := getCORS()

			// Merge all test cases
			allCases := append([]ARMORErrorTestCase{}, basic.TestCases...)
			allCases = append(allCases, auth.TestCases...)
			allCases = append(allCases, validation.TestCases...)
			allCases = append(allCases, server.TestCases...)
			allCases = append(allCases, protocol.TestCases...)
			allCases = append(allCases, cors.TestCases...)

			return ARMORErrorTestTable{
				Name:        "All ARMOR Error Tests",
				Description: "Comprehensive test suite covering all ARMOR error scenarios",
				TestCases:   allCases,
			}
		},
}

// =============================================================================
// ARMOR TEST HELPERS
// =============================================================================
// These helper functions simplify writing ARMOR error tests.
// They integrate the existing server setup and validation helpers.
// =============================================================================

// TestARMORErrorScenario tests a single ARMOR error scenario.
//
// This helper function encapsulates the complete testing pattern for ARMOR
// error scenarios. It sets up a test server, makes a request, and validates
// the response against expectations.
//
// Use this helper when:
//   - Testing individual error scenarios
//   - Building table-driven tests
//   - Creating integration tests
//
// Example (in a test):
//
//	func TestMyErrorScenarios(t *testing.T) {
//	    for _, tc := range myTestCases {
//	        t.Run(tc.Name, func(t *testing.T) {
//	            TestARMORErrorScenario(t, tc)
//	        })
//	    }
//	}
//
// Parameters:
//   - t: Testing instance
//   - tc: ARMOR error test case to execute
func TestARMORErrorScenario(t *testing.T, tc ARMORErrorTestCase) {
	t.Helper()

	// Skip if reason provided
	if tc.SkipReason != "" {
		t.Skip(tc.SkipReason)
	}

	// Setup test server
	scenario := ErrorServerScenario{
		StatusCode: tc.StatusCode,
		ErrorCode:  tc.ErrorCode,
		Message:    tc.Message,
		Resource:   tc.Path,
		Headers:    tc.Headers,
	}

	// Add request matcher if path is specified
	if tc.Path != "" {
		scenario.RequestMatcher = MatchByPath(tc.Path)
	}

	server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
	defer server.Close()

	// Custom setup function
	if tc.SetupFunc != nil {
		tc.SetupFunc(t, server)
	}

	// Prepare request options
	method := tc.Method
	if method == "" {
		method = "GET"
	}

	opts := TestRequestOptions{
		Method:      method,
		Path:        tc.Path,
		QueryParams: tc.QueryParams,
		Headers:     map[string]string{}, // Request headers (not response)
	}

	// Add body if specified
	if tc.Body != "" {
		opts.Body = strings.NewReader(tc.Body)
	}

	// Make request
	start := time.Now()
	resp, err := MakeTestRequest(server.URL, opts)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Validate response time if specified
	if tc.MinResponseTime > 0 && duration < tc.MinResponseTime {
		t.Errorf("Response time %v is less than minimum %v", duration, tc.MinResponseTime)
	}
	if tc.MaxResponseTime > 0 && duration > tc.MaxResponseTime {
		t.Errorf("Response time %v exceeds maximum %v", duration, tc.MaxResponseTime)
	}

	// Custom validation function
	if tc.ValidateFunc != nil {
		tc.ValidateFunc(t, resp)
	} else {
		// Default validation
		ValidateErrorResponse(t, resp, tc.StatusCode, tc.ErrorCode)
	}
}

// filterARMORTestsByCategory filters ARMOR error test cases by category.
//
// This helper allows selective test execution based on category.
// Useful for running specific subsets of tests.
//
// Parameters:
//   - table: ARMOR error test table to filter
//   - category: Category to filter by
//
// Returns filtered test cases
//
// Example:
//
//	all := ARMORErrorTestTables.AllTests()
//	authTests := filterARMORTestsByCategory(all, "Auth")
func filterARMORTestsByCategory(table ARMORErrorTestTable, category string) []ARMORErrorTestCase {
	var filtered []ARMORErrorTestCase
	for _, tc := range table.TestCases {
		if tc.Category == category {
			filtered = append(filtered, tc)
		}
	}
	return filtered
}

// filterARMORTestsByTag filters ARMOR error test cases by tag.
//
// This helper allows selective test execution based on tags.
// Useful for running specific test scenarios.
//
// Parameters:
//   - table: ARMOR error test table to filter
//   - tag: Tag to filter by
//
// Returns filtered test cases
//
// Example:
//
//	all := ARMORErrorTestTables.AllTests()
//	notFoundTests := filterARMORTestsByTag(all, "404")
func filterARMORTestsByTag(table ARMORErrorTestTable, tag string) []ARMORErrorTestCase {
	var filtered []ARMORErrorTestCase
	for _, tc := range table.TestCases {
		for _, t := range tc.Tags {
			if t == tag {
				filtered = append(filtered, tc)
				break
			}
		}
	}
	return filtered
}

// =============================================================================
// DOCUMENTATION
// =============================================================================

/*

# ARMOR Error Testing Guide

## Overview

The ARMOR error testing framework provides a comprehensive, extensible infrastructure
for testing S3-compatible error responses. It integrates error patterns, test servers,
request helpers, and validation functions into a cohesive testing experience.

## Core Components

### 1. Error Test Patterns (error_test_patterns.go)

Defines base types and predefined error patterns:
- S3Error: Core S3 error response structure
- ErrorScenarioConfig: Test scenario configuration
- CommonErrorPatterns: 8 common error scenarios
- AuthErrorPatterns: 6 authentication error scenarios
- ClientErrorPatterns: 4xx client errors
- ServerErrorPatterns: 5xx server errors

### 2. Test Server Setup (test_server_setup_test.go)

Provides configurable test servers:
- ConfigurableErrorServer: Server with configurable error responses
- PredefinedErrorScenarios: Common error scenarios
- Request matchers: Path, method, header matching
- Request/response logging

### 3. Request/Response Helpers (test_request_validation_helpers.go)

Simplifies making requests and validating responses:
- MakeTestRequest: Full-featured request builder
- ValidateErrorResponse: Error response validation
- ValidateResponseStructure: Comprehensive structure validation
- RequestAndValidateError: Combined helper

### 4. Base Test Tables (test_tables.go in validate package)

Provides table-driven test structures:
- ValidationTestCase: Core test case structure
- ValidationTestTable: Test table collection
- CommonValidationTests: Prebuilt test tables
- Extension helpers: ExtendTable, FilterTable, MergeTables

## Usage Patterns

### Pattern 1: Use Predefined Tables

Run tests from predefined ARMOR tables:

    func TestARMORErrorResponses(t *testing.T) {
        table := ARMORErrorTestTables.BasicErrorTests()

        for _, tc := range table.TestCases {
            t.Run(tc.Name, func(t *testing.T) {
                TestARMORErrorScenario(t, tc)
            })
        }
    }

### Pattern 2: Extend Existing Tables

Add custom tests to existing tables:

    func TestExtendedErrorTests(t *testing.T) {
        base := ARMORErrorTestTables.BasicErrorTests()

        custom := []ARMORErrorTestCase{
            {
                Name:       "Custom 418 error",
                StatusCode: 418,
                ErrorCode:  "ImATeapot",
                Message:    "I'm a teapot",
                Path:       "/armor/blobs/tea",
            },
        }

        allCases := append(base.TestCases, custom...)
        for _, tc := range allCases {
            t.Run(tc.Name, func(t *testing.T) {
                TestARMORErrorScenario(t, tc)
            })
        }
    }

### Pattern 3: Create Custom Table

Create a custom test table for specific scenarios:

    func TestCustomScenarios(t *testing.T) {
        customTable := ARMORErrorTestTable{
            Name:        "Custom Scenarios",
            Description: "Custom ARMOR error scenarios",
            TestCases: []ARMORErrorTestCase{
                {Name: "Scenario 1", StatusCode: 404, ErrorCode: "CustomNotFound"},
                {Name: "Scenario 2", StatusCode: 403, ErrorCode: "CustomDenied"},
            },
        }

        for _, tc := range customTable.TestCases {
            t.Run(tc.Name, func(t *testing.T) {
                TestARMORErrorScenario(t, tc)
            })
        }
    }

### Pattern 4: Filter Tests by Category

Run specific test categories:

    func TestAuthErrorsOnly(t *testing.T) {
        all := ARMORErrorTestTables.AllTests()
        authTests := filterARMORTestsByCategory(all, "Auth")

        for _, tc := range authTests {
            t.Run(tc.Name, func(t *testing.T) {
                TestARMORErrorScenario(t, tc)
            })
        }
    }

### Pattern 5: Custom Validation

Use custom validation logic:

    tc := ARMORErrorTestCase{
        Name:       "Custom validation",
        StatusCode: 404,
        ErrorCode:  "NoSuchKey",
        Path:       "/armor/blobs/file.dat",
        ValidateFunc: func(t *testing.T, resp *http.Response) {
            t.Helper()
            // Custom validation logic
            if resp.Header.Get("X-Custom-Header") == "" {
                t.Error("Missing custom header")
            }
        },
    }
    TestARMORErrorScenario(t, tc)

### Pattern 6: Test with Real ARMOR Server

Test against a real ARMOR server:

    func TestRealARMORServer(t *testing.T) {
        serverURL := os.Getenv("ARMOR_TEST_SERVER_URL")
        if serverURL == "" {
            t.Skip("ARMOR_TEST_SERVER_URL not set")
        }

        opts := TestRequestOptions{
            Method: "GET",
            Path:   "/armor/blobs/nonexistent.dat",
        }

        resp, err := MakeTestRequest(serverURL, opts)
        if err != nil {
            t.Fatalf("Request failed: %v", err)
        }

        ValidateErrorResponse(t, resp, 404, "NoSuchKey")
    }

## Integration Points

### With ARMOR Server

The test framework integrates seamlessly with ARMOR's server:
- Uses ARMOR's error response structures
- Validates S3 XML format compliance
- Tests ARMOR-specific headers and behavior

### With CI/CD

Tests are designed for CI/CD environments:
- No external dependencies required for mock tests
- Can test against real ARMOR instances
- Fast execution for rapid feedback

### With Other Packages

The framework coordinates with other ARMOR packages:
- validate: Test table structures and patterns
- backend: Backend error scenarios
- config: Configuration-driven error testing

## Best Practices

1. **Use predefined patterns when possible**: Leverage CommonErrorPatterns and AuthErrorPatterns
2. **Organize tests by category**: Use Category field for test organization
3. **Add descriptive names**: Test names should clearly describe what is being tested
4. **Use tags for filtering**: Enable selective test execution with Tags
5. **Set realistic timing expectations**: Use MinResponseTime and MaxResponseTime appropriately
6. **Document complex tests**: Use Description field for non-obvious test scenarios
7. **Extend rather than replace**: Extend existing tables rather than creating from scratch

## Common Test Scenarios

### Testing Missing Objects

    tc := ARMORErrorTestCase{
        Name:       "Blob not found",
        StatusCode: 404,
        ErrorCode:  "NoSuchKey",
        Message:    "The specified key does not exist",
        Path:       "/armor/blobs/missing-file.dat",
    }

### Testing Access Denied

    tc := ARMORErrorTestCase{
        Name:       "Access denied to protected blob",
        StatusCode: 403,
        ErrorCode:  "AccessDenied",
        Message:    "Access Denied",
        Path:       "/armor/blobs/protected.dat",
    }

### Testing Invalid Input

    tc := ARMORErrorTestCase{
        Name:       "Invalid content type",
        StatusCode: 415,
        ErrorCode:  "UnsupportedMediaType",
        Message:    "The content type is not supported",
        Path:       "/armor/blobs/file.dat",
        Method:     "PUT",
        Body:       "test data",
    }

## Extending the Framework

### Adding New Error Types

To add a new error type:

1. Define error code constant in error_test_patterns.go
2. Add pattern to appropriate pattern struct
3. Create test cases in ARMORErrorTestTables
4. Document the new error type

Example:

    // In error_test_patterns.go
    const ErrorCodeCustomError = "CustomError"

    // In ARMORErrorTestTables
    CustomErrors: func() ARMORErrorTestTable {
        return ARMORErrorTestTable{
            Name: "Custom Errors",
            TestCases: []ARMORErrorTestCase{
                {
                    Name:       "Custom error",
                    StatusCode: 418,
                    ErrorCode:  ErrorCodeCustomError,
                    Message:    "Custom error message",
                    Path:       "/armor/blobs/file.dat",
                },
            },
        }
    }

### Adding New Validation Rules

To add new validation logic:

1. Create validation function
2. Add field to ARMORErrorTestCase if needed
3. Integrate with TestARMORErrorScenario

Example:

    func validateCustomHeader(t *testing.T, resp *http.Response) {
        t.Helper()
        header := resp.Header.Get("X-Custom-Header")
        if header == "" {
            t.Error("Missing X-Custom-Header")
        }
    }

    tc := ARMORErrorTestCase{
        Name:         "Custom header test",
        StatusCode:   200,
        ValidateFunc: validateCustomHeader,
    }

## Troubleshooting

### Test Failures

If tests fail:
1. Check error code matches expected
2. Verify status code is correct
3. Ensure XML structure is valid
4. Validate response time expectations
5. Check for required headers

### Common Issues

**Issue**: Test server not responding
**Solution**: Ensure server is created and deferred Close() is called

**Issue**: Validation fails for valid error
**Solution**: Check that ErrorCode matches S3 specification exactly

**Issue**: Response time exceeds expectations
**Solution**: Adjust MaxResponseTime or investigate performance issues

## Related Files

- error_test_patterns.go: Base types and error patterns
- test_server_setup_test.go: Test server configuration
- test_request_validation_helpers.go: Request/response helpers
- error_patterns_verification_test.go: Pattern verification tests
- internal/validate/test_tables.go: Base test table structures

*/
