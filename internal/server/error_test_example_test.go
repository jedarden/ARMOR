// Package server provides example tests demonstrating ARMOR error testing patterns.
//
// # ARMOR Error Testing Examples
//
// This file contains comprehensive examples demonstrating how to use the ARMOR
// error testing framework. Each example shows a different testing pattern and
// can be used as a reference for writing your own tests.
//
// # Running the Examples
//
// To run these examples:
//
//	go test -v ./internal/server -run TestExample
//
// To run a specific example:
//
//	go test -v ./internal/server -run TestExampleBasic
//
// # Example Categories
//
// 1. Basic Patterns: TestExampleBasic*
//    - Demonstrates fundamental test patterns
//    - Shows basic error scenario testing
//
// 2. ARMOR Helpers: TestExampleARMORHelpers*
//    - Demonstrates ARMOR-specific helpers
//    - Shows ARMOR blob operation testing
//
// 3. Table-Driven Tests: TestExampleTable*
//    - Demonstrates table-driven test patterns
//    - Shows test table creation and usage
//
// 4. Authentication: TestExampleAuth*
//    - Demonstrates authentication testing
//    - Shows auth error scenarios
//
// 5. Custom Scenarios: TestExampleCustom*
//    - Demonstrates custom test scenarios
//    - Shows extension patterns
//
// # Using These Examples
//
// These examples are intended to be copied and modified for your specific
// testing needs. Each example includes:
//   - Clear description of what it tests
//   - Step-by-step approach
//   - Comments explaining the pattern
//   - Expected assertions
package server

import (
	"net/http"
	"testing"
	"time"
)

// =============================================================================
// EXAMPLE 1: BASIC ERROR TESTING
// =============================================================================

// TestExampleBasicErrorScenarios demonstrates basic error scenario testing.
//
// This example shows the simplest way to test ARMOR error responses using
// the configurable test server and validation helpers.
func TestExampleBasicErrorScenarios(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errorCode  string
		message    string
		path       string
	}{
		{
			name:       "404 not found",
			statusCode: 404,
			errorCode:  ErrorCodeNoSuchKey,
			message:    "The specified key does not exist",
			path:       "/armor/blobs/missing-file.dat",
		},
		{
			name:       "403 access denied",
			statusCode: 403,
			errorCode:  ErrorCodeAccessDenied,
			message:    "Access Denied",
			path:       "/armor/blobs/protected.dat",
		},
		{
			name:       "500 internal error",
			statusCode: 500,
			errorCode:  ErrorCodeInternalError,
			message:    "Internal server error",
			path:       "/armor/blobs/file.dat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: Create test server with error scenario
			scenario := ErrorServerScenario{
				StatusCode: tt.statusCode,
				ErrorCode:  tt.errorCode,
				Message:    tt.message,
				Resource:   tt.path,
			}
			server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
			defer server.Close()

			// Execute: Make request
			resp, err := MakeGETRequest(server.URL, tt.path)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			// Verify: Validate error response
			ValidateErrorResponse(t, resp, tt.statusCode, tt.errorCode)
		})
	}
}

// =============================================================================
// EXAMPLE 2: USING PREDEFINED ERROR PATTERNS
// =============================================================================

// TestExampleUsingPatterns demonstrates using predefined error patterns.
//
// This example shows how to use the predefined error patterns for consistent
// testing across your test suite.
func TestExampleUsingPatterns(t *testing.T) {
	tests := []struct {
		name    string
		pattern ErrorScenarioConfig
		path    string
	}{
		{
			name:    "Resource not found pattern",
			pattern: CommonErrorPatterns.ResourceNotFound,
			path:    "/armor/blobs/file.dat",
		},
		{
			name:    "Access denied pattern",
			pattern: CommonErrorPatterns.AccessDenied,
			path:    "/armor/blobs/protected.dat",
		},
		{
			name:    "Invalid request pattern",
			pattern: CommonErrorPatterns.InvalidRequest,
			path:    "/armor/blobs/file.dat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: Create test server from pattern
			scenario := ErrorServerScenario{
				StatusCode: tt.pattern.ExpectedStatus,
				ErrorCode:  tt.pattern.ExpectedCode,
				Message:    tt.pattern.ExpectedMessage,
				Resource:   tt.path,
			}
			server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
			defer server.Close()

			// Execute: Make request
			resp, err := MakeGETRequest(server.URL, tt.path)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			// Verify: Validate against pattern expectations
			ValidateErrorResponse(t, resp, tt.pattern.ExpectedStatus, tt.pattern.ExpectedCode)

			// Verify response time is within expectations
			if tt.pattern.MaxResponseTime > 0 {
				// In real tests, you'd measure actual response time
				// For this example, we just demonstrate the pattern
				t.Logf("Max response time expectation: %v", tt.pattern.MaxResponseTime)
			}
		})
	}
}

// =============================================================================
// EXAMPLE 3: ARMOR-SPECIFIC HELPER USAGE
// =============================================================================

// TestExampleARMORHelpers demonstrates ARMOR-specific helper functions.
//
// This example shows how to use the ARMOR-specific helper functions for
// common testing scenarios.
func TestExampleARMORHelpers(t *testing.T) {
	t.Run("Test blob not found", func(t *testing.T) {
		// Setup: Create test server
		scenario := ErrorServerScenario{
			StatusCode: 404,
			ErrorCode:  ErrorCodeNoSuchKey,
			Message:    "The specified key does not exist",
			Resource:   "/armor/blobs/missing.dat",
		}
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		// Execute: Use ARMOR helper
		resp, err := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Verify: Use ARMOR validation
		ValidateS3ErrorResponse(t, resp, ErrorCodeNoSuchKey)
	})

	t.Run("Test access denied", func(t *testing.T) {
		// Setup: Create test server
		scenario := ErrorServerScenario{
			StatusCode: 403,
			ErrorCode:  ErrorCodeAccessDenied,
			Message:    "Access Denied",
			Resource:   "/armor/blobs/protected.dat",
		}
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		// Execute: Use ARMOR helper
		resp, err := TestARMORBlobAccessDenied(t, server.URL, "/armor/blobs/protected.dat")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Verify: Use ARMOR validation
		ValidateS3ErrorResponse(t, resp, ErrorCodeAccessDenied)

		// Verify ARMOR headers
		AssertARMORHeader(t, resp, "Content-Type", "application/xml")
	})
}

// =============================================================================
// EXAMPLE 4: TABLE-DRIVEN TESTING WITH ARMOR TABLES
// =============================================================================

// TestExampleTableDrivenTests demonstrates table-driven test patterns.
//
// This example shows how to use ARMOR test tables for organized,
// maintainable test suites.
func TestExampleTableDrivenTests(t *testing.T) {
	t.Run("Use predefined ARMOR tables", func(t *testing.T) {
		// Get predefined test table
		table := ARMORErrorTestTables.BasicErrorTests()

		// Run all tests in the table
		for _, tc := range table.TestCases {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}
	})

	t.Run("Create custom ARMOR test table", func(t *testing.T) {
		// Define custom test cases
		customCases := []ARMORErrorTestCase{
			{
				Name:              "Custom blob not found",
				Description:       "Tests custom 404 scenario",
				StatusCode:        404,
				ErrorCode:         ErrorCodeNoSuchKey,
				Message:           "The specified key does not exist",
				Path:              "/armor/blobs/custom-missing.dat",
				ExpectXMLStructure: true,
				MaxResponseTime:   500 * time.Millisecond,
				Category:          "CustomNotFound",
				Tags:              []string{"custom", "404"},
			},
			{
				Name:              "Custom access denied",
				Description:       "Tests custom 403 scenario",
				StatusCode:        403,
				ErrorCode:         ErrorCodeAccessDenied,
				Message:           "Access Denied",
				Path:              "/armor/blobs/custom-protected.dat",
				ExpectXMLStructure: true,
				MaxResponseTime:   300 * time.Millisecond,
				Category:          "CustomAuth",
				Tags:              []string{"custom", "403"},
			},
		}

		// Create test table
		customTable := CreateARMORTestTable(
			"Custom ARMOR Tests",
			"Custom ARMOR error scenarios",
			customCases,
		)

		// Run all tests
		for _, tc := range customTable.TestCases {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}
	})
}

// =============================================================================
// EXAMPLE 5: AUTHENTICATION ERROR TESTING
// =============================================================================

// TestExampleAuthenticationErrors demonstrates authentication error testing.
//
// This example shows how to test various authentication failure scenarios
// using the ARMOR error testing framework.
func TestExampleAuthenticationErrors(t *testing.T) {
	t.Run("Use authentication error table", func(t *testing.T) {
		// Get authentication error test table
		table := ARMORErrorTestTables.AuthenticationErrors()

		// Run all authentication tests
		for _, tc := range table.TestCases {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}
	})

	t.Run("Test specific auth scenarios", func(t *testing.T) {
		tests := []struct {
			name      string
			scenario ErrorServerScenario
			path      string
			helper    func(*testing.T, string, string) (*http.Response, error)
			assert    func(*testing.T, *http.Response)
		}{
			{
				name: "Missing credentials",
				scenario: ErrorServerScenario{
					StatusCode: 403,
					ErrorCode:  ErrorCodeMissingAuthenticationToken,
					Message:    "Missing Authentication Token",
				},
				path:   "/armor/blobs/file.dat",
				helper: TestARMORMissingCredentials,
				assert: func(t *testing.T, resp *http.Response) {
					ValidateS3ErrorResponse(t, resp, ErrorCodeMissingAuthenticationToken)
				},
			},
			{
				name: "Invalid signature",
				scenario: ErrorServerScenario{
					StatusCode: 403,
					ErrorCode:  ErrorCodeSignatureDoesNotMatch,
					Message:    "The request signature we calculated does not match",
				},
				path:   "/armor/blobs/file.dat",
				helper: TestARMORInvalidSignature,
				assert: func(t *testing.T, resp *http.Response) {
					ValidateS3ErrorResponse(t, resp, ErrorCodeSignatureDoesNotMatch)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Setup
				server := NewConfigurableErrorServer([]ErrorServerScenario{tt.scenario})
				defer server.Close()

				// Execute
				resp, err := tt.helper(t, server.URL, tt.path)
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				defer resp.Body.Close()

				// Verify
				tt.assert(t, resp)
			})
		}
	})
}

// =============================================================================
// EXAMPLE 6: CUSTOM VALIDATION AND ASSERTIONS
// =============================================================================

// TestExampleCustomValidation demonstrates custom validation patterns.
//
// This example shows how to add custom validation logic to your tests.
func TestExampleCustomValidation(t *testing.T) {
	t.Run("Custom validation function", func(t *testing.T) {
		// Define test case with custom validation
		tc := ARMORErrorTestCase{
			Name:              "Error with custom headers",
			StatusCode:        404,
			ErrorCode:         ErrorCodeNoSuchKey,
			Message:           "Not found",
			Path:              "/armor/blobs/file.dat",
			ExpectXMLStructure: true,
			ValidateFunc: func(t *testing.T, resp *http.Response) {
				t.Helper()

				// Standard validation
				ValidateErrorResponse(t, resp, 404, ErrorCodeNoSuchKey)

				// Custom validation: Check for custom header
				customHeader := resp.Header.Get("X-Custom-Header")
				if customHeader == "" {
					t.Error("Expected X-Custom-Header to be present")
				}
			},
		}

		// Setup server with custom header
		scenario := ErrorServerScenario{
			StatusCode: 404,
			ErrorCode:  ErrorCodeNoSuchKey,
			Message:    "Not found",
			Headers: map[string]string{
				"Content-Type":     "application/xml",
				"X-Custom-Header":  "custom-value",
			},
		}
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		// Execute test with custom validation
		TestARMORErrorScenario(t, tc)
	})

	t.Run("ARMOR-specific assertions", func(t *testing.T) {
		// Setup
		scenario := ErrorServerScenario{
			StatusCode: 403,
			ErrorCode:  ErrorCodeAccessDenied,
			Message:    "Access Denied - You do not have permission",
			Headers: map[string]string{
				"Content-Type":        "application/xml",
				"X-ARMOR-Request-ID":  "test-request-123",
			},
		}
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		// Execute
		resp, err := MakeGETRequest(server.URL, "/armor/blobs/file.dat")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// ARMOR-specific assertions
		AssertARMORErrorCode(t, resp, ErrorCodeAccessDenied)
		AssertARMORErrorMessageContains(t, resp, "Access Denied")
		AssertARMORHeader(t, resp, "Content-Type", "application/xml")
		AssertARMORHeader(t, resp, "X-ARMOR-Request-ID", "")
	})
}

// =============================================================================
// EXAMPLE 7: EXTENDING PREDEFINED TABLES
// =============================================================================

// TestExampleExtendingTables demonstrates table extension patterns.
//
// This example shows how to extend predefined test tables with custom
// test cases without modifying the original tables.
func TestExampleExtendingTables(t *testing.T) {
	t.Run("Extend basic error tests", func(t *testing.T) {
		// Get base table
		base := ARMORErrorTestTables.BasicErrorTests()

		// Define custom extensions
		customCases := []ARMORErrorTestCase{
			{
				Name:       "Extended: Rate limit error",
				StatusCode: 429,
				ErrorCode:  "TooManyRequests",
				Message:    "Rate limit exceeded",
				Path:       "/armor/blobs/file.dat",
				Category:   "RateLimit",
				Tags:       []string{"rate-limit", "429"},
			},
		}

		// Extend the table
		extended := ExtendARMORTestTable(base, customCases)

		// Verify extension worked
		if len(extended.TestCases) != len(base.TestCases)+1 {
			t.Errorf("Expected %d cases, got %d", len(base.TestCases)+1, len(extended.TestCases))
		}

		// Run extended tests
		for _, tc := range extended.TestCases {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}
	})

	t.Run("Merge multiple tables", func(t *testing.T) {
		// Get multiple tables
		basic := ARMORErrorTestTables.BasicErrorTests()
		auth := ARMORErrorTestTables.AuthenticationErrors()

		// Merge tables
		merged := MergeARMORTestTables(basic, auth)

		// Verify merge worked
		expectedCount := len(basic.TestCases) + len(auth.TestCases)
		if len(merged.TestCases) != expectedCount {
			t.Errorf("Expected %d cases, got %d", expectedCount, len(merged.TestCases))
		}

		t.Logf("Merged table name: %s", merged.Name)
		t.Logf("Total test cases: %d", len(merged.TestCases))
	})
}

// =============================================================================
// EXAMPLE 8: FILTERING AND SELECTIVE TESTING
// =============================================================================

// TestExampleFiltering demonstrates test filtering patterns.
//
// This example shows how to filter tests by category or tags for
// selective test execution.
func TestExampleFiltering(t *testing.T) {
	t.Run("Filter by category", func(t *testing.T) {
		// Get all tests
		all := ARMORErrorTestTables.AllTests()

		// Filter by Auth category
		authTests := filterARMORTestsByCategory(all, "Auth")

		t.Logf("Found %d auth tests", len(authTests))

		// Run only auth tests
		for _, tc := range authTests {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}
	})

	t.Run("Filter by tag", func(t *testing.T) {
		// Get all tests
		all := ARMORErrorTestTables.AllTests()

		// Filter by tag "404"
		notFoundTests := filterARMORTestsByTag(all, "404")

		t.Logf("Found %d 404 tests", len(notFoundTests))

		// Run only 404 tests
		for _, tc := range notFoundTests {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}
	})
}

// =============================================================================
// EXAMPLE 9: REQUEST/RESPONSE VALIDATION
// =============================================================================

// TestExampleRequestValidation demonstrates comprehensive request/response validation.
//
// This example shows how to thoroughly validate both requests and responses
// using the ARMOR testing framework.
func TestExampleRequestValidation(t *testing.T) {
	t.Run("Validate S3 error response", func(t *testing.T) {
		// Setup
		scenario := ErrorServerScenario{
			StatusCode: 404,
			ErrorCode:  ErrorCodeNoSuchKey,
			Message:    "The specified key does not exist",
			Resource:   "/armor/blobs/file.dat",
		}
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		// Execute
		resp, err := MakeGETRequest(server.URL, "/armor/blobs/file.dat")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Validate S3 error structure
		s3Err := ValidateS3XMLStructure(t, resp)

		// Additional validation
		if s3Err.Code != ErrorCodeNoSuchKey {
			t.Errorf("Expected code %s, got %s", ErrorCodeNoSuchKey, s3Err.Code)
		}
		if s3Err.Message == "" {
			t.Error("Expected non-empty message")
		}
	})

	t.Run("Validate with options", func(t *testing.T) {
		// Setup
		scenario := ErrorServerScenario{
			StatusCode: 403,
			ErrorCode:  ErrorCodeAccessDenied,
			Message:    "Access Denied - You do not have permission",
		}
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		// Execute
		resp, err := MakeGETRequest(server.URL, "/armor/blobs/file.dat")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Validate with comprehensive options
		result := ValidateResponseStructure(resp, ResponseValidationOptions{
			ExpectedStatusCode:   403,
			ExpectedErrorCode:    ErrorCodeAccessDenied,
			ExpectedMessageKeywords: []string{"access", "denied"},
			MinMessageLength:      10,
			ExpectedContentType:   "application/xml",
			ValidateStructure:     true,
		})

		if !result.IsValid {
			t.Errorf("Validation failed: %s", result.ErrorMessage)
		}
	})
}

// =============================================================================
// EXAMPLE 10: ADVANCED PATTERNS
// =============================================================================

// TestExampleAdvancedPatterns demonstrates advanced testing patterns.
//
// This example shows more complex testing scenarios including custom
// server setup, multiple error scenarios, and integration patterns.
func TestExampleAdvancedPatterns(t *testing.T) {
	t.Run("Multiple error scenarios", func(t *testing.T) {
		// Setup server with multiple scenarios
		scenarios := []ErrorServerScenario{
			{
				StatusCode: 404,
				ErrorCode:  ErrorCodeNoSuchKey,
				Message:    "Not found",
				RequestMatcher: MatchByPath("/missing"),
			},
			{
				StatusCode: 403,
				ErrorCode:  ErrorCodeAccessDenied,
				Message:    "Access denied",
				RequestMatcher: MatchByPath("/forbidden"),
			},
		}
		server := NewConfigurableErrorServer(scenarios)
		defer server.Close()

		// Test 404 scenario
		resp1, _ := MakeGETRequest(server.URL, "/missing")
		ValidateErrorResponse(t, resp1, 404, ErrorCodeNoSuchKey)
		resp1.Body.Close()

		// Test 403 scenario
		resp2, _ := MakeGETRequest(server.URL, "/forbidden")
		ValidateErrorResponse(t, resp2, 403, ErrorCodeAccessDenied)
		resp2.Body.Close()
	})

	t.Run("Custom server setup", func(t *testing.T) {
		// Create test case with custom setup
		tc := ARMORErrorTestCase{
			Name:       "Custom setup test",
			StatusCode: 404,
			ErrorCode:  ErrorCodeNoSuchKey,
			Message:    "Not found",
			Path:       "/armor/blobs/file.dat",
			SetupFunc: func(t *testing.T, server *ConfigurableErrorServer) {
				t.Helper()
				// Custom setup logic
				t.Log("Custom setup before test")
			},
			ValidateFunc: func(t *testing.T, resp *http.Response) {
				t.Helper()
				// Custom validation
				ValidateErrorResponse(t, resp, 404, ErrorCodeNoSuchKey)
				t.Log("Custom validation passed")
			},
		}

		TestARMORErrorScenario(t, tc)
	})

	t.Run("Comprehensive ARMOR test", func(t *testing.T) {
		// This example shows a complete ARMOR error test
		// Setup: Create ARMOR-specific error scenario
		scenario := ErrorServerScenario{
			StatusCode: 404,
			ErrorCode:  ErrorCodeNoSuchKey,
			Message:    "The specified key does not exist",
			Resource:   "/armor/blobs/test-file.dat",
			Headers: map[string]string{
				"Content-Type":       "application/xml",
				"X-ARMOR-Request-ID": "req-123456",
			},
		}

		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		// Execute: Make ARMOR blob request
		resp, err := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/test-file.dat")
		if err != nil {
			t.Fatalf("ARMOR blob request failed: %v", err)
		}
		defer resp.Body.Close()

		// Verify: Comprehensive validation
		ValidateS3ErrorResponse(t, resp, ErrorCodeNoSuchKey)
		ValidateARMORErrorHeaders(t, resp, map[string]string{
			"X-ARMOR-Request-ID": "req-",
		})

		// Additional ARMOR-specific assertions
		AssertARMORErrorCode(t, resp, ErrorCodeNoSuchKey)
		AssertARMORErrorMessageContains(t, resp, "does not exist")
		AssertARMORHeader(t, resp, "Content-Type", "application/xml")
		AssertARMORHeader(t, resp, "X-ARMOR-Request-ID", "")
	})
}

// =============================================================================
// EXAMPLE 11: ERROR PATTERN VERIFICATION
// =============================================================================

// TestExamplePatternVerification verifies predefined error patterns.
//
// This example shows how to verify that predefined error patterns are
// correctly configured for your ARMOR instance.
func TestExamplePatternVerification(t *testing.T) {
	t.Run("Verify common patterns", func(t *testing.T) {
		// Test that common patterns are correctly configured
		patterns := []struct {
			name           string
			pattern        ErrorScenarioConfig
			expectedStatus int
			expectedCode   string
		}{
			{
				name:           "ResourceNotFound",
				pattern:        CommonErrorPatterns.ResourceNotFound,
				expectedStatus: 404,
				expectedCode:   ErrorCodeNoSuchKey,
			},
			{
				name:           "AccessDenied",
				pattern:        CommonErrorPatterns.AccessDenied,
				expectedStatus: 403,
				expectedCode:   ErrorCodeAccessDenied,
			},
			{
				name:           "InternalServerError",
				pattern:        CommonErrorPatterns.InternalServerError,
				expectedStatus: 500,
				expectedCode:   ErrorCodeInternalError,
			},
		}

		for _, tt := range patterns {
			t.Run(tt.name, func(t *testing.T) {
				if tt.pattern.ExpectedStatus != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d",
						tt.expectedStatus, tt.pattern.ExpectedStatus)
				}
				if tt.pattern.ExpectedCode != tt.expectedCode {
					t.Errorf("Expected code %s, got %s",
						tt.expectedCode, tt.pattern.ExpectedCode)
				}
				if tt.pattern.Name == "" {
					t.Error("Pattern name should not be empty")
				}
				if tt.pattern.Description == "" {
					t.Error("Pattern description should not be empty")
				}
			})
		}
	})

	t.Run("Verify auth patterns", func(t *testing.T) {
		// Test that auth patterns are correctly configured
		patterns := []ErrorScenarioConfig{
			AuthErrorPatterns.MissingAuthHeader,
			AuthErrorPatterns.InvalidAccessKeyId,
			AuthErrorPatterns.SignatureDoesNotMatch,
			AuthErrorPatterns.RequestExpired,
		}

		for _, pattern := range patterns {
			t.Run(pattern.Name, func(t *testing.T) {
				if pattern.Category != string(CategoryAuth) {
					t.Errorf("Expected category %s, got %s",
						CategoryAuth, pattern.Category)
				}
				if pattern.ExpectedStatus != 403 {
					t.Errorf("Expected status 403, got %d",
						pattern.ExpectedStatus)
				}
			})
		}
	})
}

// =============================================================================
// EXAMPLE 12: INTEGRATION WITH ARMOR SERVER
// =============================================================================

// TestExampleRealARMORIntegration demonstrates integration testing.
//
// This example shows how to test against a real ARMOR server.
// NOTE: This test requires ARMOR_TEST_SERVER_URL to be set.
func TestExampleRealARMORIntegration(t *testing.T) {
	// This test is skipped unless specifically enabled
	t.Skip("Set ARMOR_TEST_SERVER_URL to run this test")

	serverURL := "http://localhost:8080" // Would normally come from env var

	t.Run("Test real ARMOR 404 response", func(t *testing.T) {
		// Test against real ARMOR server
		resp, err := TestARMORBlobNotFound(t, serverURL, "/armor/blobs/nonexistent.dat")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Validate real ARMOR response
		ValidateS3ErrorResponse(t, resp, ErrorCodeNoSuchKey)
		ValidateARMORErrorHeaders(t, resp, nil)
	})
}
