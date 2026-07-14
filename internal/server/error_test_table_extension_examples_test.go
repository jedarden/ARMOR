// Package server provides comprehensive examples of test table extension patterns.
//
// # Test Table Extension Examples
//
// This file contains practical, runnable examples demonstrating how to extend
// ARMOR's test table structure for new error types. Each example shows a different
// extension pattern and can be used as a template for your own test scenarios.
//
// # Running These Examples
//
// To run these examples:
//
//	go test -v ./internal/server -run TestExtensionExample
//
// To run a specific example:
//
//	go test -v ./internal/server -run TestExtensionExampleRateLimit
//
// # Example Categories
//
// 1. Basic Extension: TestExtensionExampleBasicExtension
//    - Shows how to extend existing tables with custom cases
//
// 2. New Category: TestExtensionExampleNewCategory
//    - Shows how to create a completely new error category
//
// 3. Custom Validation: TestExtensionExampleCustomValidation
//    - Shows how to add custom validation logic
//
// 4. Multiple HTTP Methods: TestExtensionExampleMultipleMethods
//    - Shows how to test same error across different HTTP methods
//
// 5. Header Validation: TestExtensionExampleHeaderValidation
//    - Shows how to validate custom response headers
//
// 6. Performance Testing: TestExtensionExamplePerformance
//    - Shows how to add performance expectations
//
// 7. Conditional Testing: TestExtensionExampleConditional
//    - Shows how to create conditional tests that skip based on environment
//
// 8. Table Merging: TestExtensionExampleTableMerging
//    - Shows how to merge multiple test tables
//
// 9. Filtering: TestExtensionExampleFiltering
//    - Shows how to filter tests by category or tag
package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// EXAMPLE 1: BASIC TABLE EXTENSION
// =============================================================================

// TestExtensionExampleBasicExtension demonstrates extending an existing table.
//
// This is the simplest extension pattern - add custom test cases to a
// predefined table and run them together.
func TestExtensionExampleBasicExtension(t *testing.T) {
	t.Run("Extend basic error tests", func(t *testing.T) {
		// Get the base table
		base := ARMORErrorTestTables.BasicErrorTests()

		// Define custom test cases to add
		customCases := []ARMORErrorTestCase{
			{
				Name:              "Custom: Rate limit error",
				Description:       "Tests 429 response when rate limit is exceeded",
				StatusCode:        429,
				ErrorCode:         "TooManyRequests",
				Message:           "Rate limit exceeded",
				Path:              "/armor/blobs/file.dat",
				ExpectXMLStructure: true,
				MaxResponseTime:   200 * time.Millisecond,
				Category:          "RateLimit",
				Tags:              []string{"rate-limit", "429", "custom"},
				Headers: map[string]string{
					"Retry-After":           "60",
					"X-RateLimit-Remaining": "0",
				},
			},
			{
				Name:              "Custom: Quota exceeded",
				Description:       "Tests 503 response when quota is exceeded",
				StatusCode:        503,
				ErrorCode:         "SlowDown",
				Message:           "Reduce request rate",
				Path:              "/armor/blobs/file.dat",
				ExpectXMLStructure: true,
				MaxResponseTime:   300 * time.Millisecond,
				Category:          "Quota",
				Tags:              []string{"quota", "503", "custom"},
				Headers: map[string]string{
					"Retry-After": "120",
				},
			},
		}

		// Create extended table using helper
		extended := ExtendARMORTestTable(base, customCases)

		// Verify the extension worked
		expectedCount := len(base.TestCases) + len(customCases)
		if len(extended.TestCases) != expectedCount {
			t.Errorf("Expected %d test cases, got %d", expectedCount, len(extended.TestCases))
		}

		// Run all tests in the extended table
		for _, tc := range extended.TestCases {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}

		t.Logf("Successfully ran %d tests from extended table", len(extended.TestCases))
	})

	t.Run("Extend authentication errors", func(t *testing.T) {
		// Get authentication error table
		base := ARMORErrorTestTables.AuthenticationErrors()

		// Add custom IP whitelist error
		customCases := []ARMORErrorTestCase{
			{
				Name:              "Custom: IP not whitelisted",
				Description:       "Tests 403 when IP address is not whitelisted",
				StatusCode:        403,
				ErrorCode:         ErrorCodeAccessDenied,
				Message:           "IP address not whitelisted",
				Path:              "/armor/blobs/file.dat",
				ExpectXMLStructure: true,
				MaxResponseTime:   250 * time.Millisecond,
				Category:          "IPWhitelist",
				Tags:              []string{"ip", "whitelist", "403", "custom"},
				Headers: map[string]string{
					"X-Blocked-Reason": "ip_not_whitelisted",
					"X-Client-IP":      "192.168.1.100",
				},
			},
		}

		// Create extended table
		extended := ExtendARMORTestTable(base, customCases)

		// Run all tests
		for _, tc := range extended.TestCases {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}
	})
}

// =============================================================================
// EXAMPLE 2: NEW ERROR CATEGORY
// =============================================================================

// TestExtensionExampleNewCategory demonstrates creating a new error category.
//
// Use this pattern when you have a completely new type of error that doesn't
// fit into existing categories.
func TestExtensionExampleNewCategory(t *testing.T) {
	t.Run("Rate limit error category", func(t *testing.T) {
		// Create a completely new error category table
		rateLimitTable := ARMORErrorTestTable{
			Name:        "Rate Limit Errors",
			Description: "Tests for rate limiting and quota errors",
			TestCases: []ARMORErrorTestCase{
				{
					Name:              "Rate limit exceeded - default",
					Description:       "Tests 429 when default rate limit is exceeded",
					StatusCode:        429,
					ErrorCode:         "TooManyRequests",
					Message:           "Rate limit exceeded",
					Path:              "/armor/blobs/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   200 * time.Millisecond,
					Category:          "RateLimit",
					Tags:              []string{"rate-limit", "429", "default"},
					Headers: map[string]string{
						"Retry-After":           "60",
						"X-RateLimit-Remaining": "0",
						"X-RateLimit-Limit":     "1000",
					},
				},
				{
					Name:              "Rate limit exceeded - API tier",
					Description:       "Tests 429 when API rate limit is exceeded",
					StatusCode:        429,
					ErrorCode:         "TooManyRequests",
					Message:           "API rate limit exceeded",
					Path:              "/api/v1/blobs/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   200 * time.Millisecond,
					Category:          "RateLimit",
					Tags:              []string{"rate-limit", "429", "api"},
					Headers: map[string]string{
						"Retry-After":           "60",
						"X-RateLimit-Remaining": "0",
						"X-RateLimit-Limit":     "100",
						"X-RateLimit-Scope":     "api",
					},
				},
				{
					Name:              "Slow down - quota exceeded",
					Description:       "Tests 503 when request quota is exceeded",
					StatusCode:        503,
					ErrorCode:         "SlowDown",
					Message:           "Reduce request rate",
					Path:              "/armor/blobs/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   300 * time.Millisecond,
					Category:          "Quota",
					Tags:              []string{"quota", "503", "slow-down"},
					Headers: map[string]string{
						"Retry-After": "120",
					},
				},
			},
		}

		// Run all rate limit tests
		for _, tc := range rateLimitTable.TestCases {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}

		t.Logf("Successfully ran %d rate limit tests", len(rateLimitTable.TestCases))
	})

	t.Run("Geographic access control category", func(t *testing.T) {
		// Create a new geographic access control category
		geoTable := ARMORErrorTestTable{
			Name:        "Geographic Access Control",
			Description: "Tests for geographic access restrictions",
			TestCases: []ARMORErrorTestCase{
				{
					Name:              "Access denied - blocked region",
					Description:       "Tests 403 when accessing from blocked region",
					StatusCode:        403,
					ErrorCode:         ErrorCodeAccessDenied,
					Message:           "Access from this region is restricted",
					Path:              "/armor/blobs/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   250 * time.Millisecond,
					Category:          "GeoBlock",
					Tags:              []string{"geo", "region", "403"},
					Headers: map[string]string{
						"X-Geo-Blocked":      "true",
						"X-Geo-Region":       "blocked-region",
						"X-Allowed-Regions":  "us-east-1,us-west-2",
					},
				},
				{
					Name:              "Access denied - blocked country",
					Description:       "Tests 403 when accessing from blocked country",
					StatusCode:        403,
					ErrorCode:         ErrorCodeAccessDenied,
					Message:           "Access from this country is restricted",
					Path:              "/armor/blobs/file.dat",
					ExpectXMLStructure: true,
					MaxResponseTime:   250 * time.Millisecond,
					Category:          "GeoBlock",
					Tags:              []string{"geo", "country", "403"},
					Headers: map[string]string{
						"X-Geo-Blocked":       "true",
						"X-Geo-Country":       "XX",
						"X-Allowed-Countries": "US,CA,GB",
					},
				},
			},
		}

		// Run all geographic access tests
		for _, tc := range geoTable.TestCases {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}

		t.Logf("Successfully ran %d geographic access tests", len(geoTable.TestCases))
	})
}

// =============================================================================
// EXAMPLE 3: CUSTOM VALIDATION
// =============================================================================

// TestExtensionExampleCustomValidation demonstrates custom validation logic.
//
// Use this pattern when you need to validate complex scenarios beyond
// standard error response validation.
func TestExtensionExampleCustomValidation(t *testing.T) {
	t.Run("Custom header validation", func(t *testing.T) {
		// Define validation helper for rate limit headers
		validateRateLimitHeaders := func(t *testing.T, resp *http.Response) {
			t.Helper()

			// Check for required headers
			requiredHeaders := []string{
				"Retry-After",
				"X-RateLimit-Remaining",
				"X-RateLimit-Limit",
			}

			for _, header := range requiredHeaders {
				value := resp.Header.Get(header)
				if value == "" {
					t.Errorf("Missing required header: %s", header)
				}
			}

			// Validate Retry-After is numeric
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				if _, err := strconv.Atoi(retryAfter); err != nil {
					t.Errorf("Retry-After should be numeric, got '%s': %v", retryAfter, err)
				}
			}

			// Validate rate limit values are numeric
			remaining := resp.Header.Get("X-RateLimit-Remaining")
			if remaining != "" {
				if _, err := strconv.Atoi(remaining); err != nil {
					t.Errorf("X-RateLimit-Remaining should be numeric, got '%s': %v", remaining, err)
				}
			}

			limit := resp.Header.Get("X-RateLimit-Limit")
			if limit != "" {
				if _, err := strconv.Atoi(limit); err != nil {
					t.Errorf("X-RateLimit-Limit should be numeric, got '%s': %v", limit, err)
				}
			}
		}

		// Create test case with custom validation
		tc := ARMORErrorTestCase{
			Name:              "Rate limit with header validation",
			Description:       "Tests 429 with custom rate limit header validation",
			StatusCode:        429,
			ErrorCode:         "TooManyRequests",
			Message:           "Rate limit exceeded",
			Path:              "/armor/blobs/file.dat",
			ExpectXMLStructure: true,
			Category:          "RateLimit",
			Tags:              []string{"rate-limit", "429", "headers"},
			ValidateFunc: func(t *testing.T, resp *http.Response) {
				t.Helper()

				// Standard validation first
				ValidateErrorResponse(t, resp, 429, "TooManyRequests")

				// Custom header validation
				validateRateLimitHeaders(t, resp)

				t.Log("Custom validation passed")
			},
		}

		// Run test with custom validation
		TestARMORErrorScenario(t, tc)
	})

	t.Run("Custom response body validation", func(t *testing.T) {
		// Create test case with custom body validation
		tc := ARMORErrorTestCase{
			Name:              "Error message with JSON details",
			Description:       "Tests error response with embedded JSON details",
			StatusCode:        400,
			ErrorCode:         ErrorCodeInvalidRequest,
			Message:           "Invalid request - validation failed",
			Path:              "/armor/blobs/file.dat",
			ExpectXMLStructure: true,
			Category:          "Validation",
			Tags:              []string{"validation", "400", "json"},
			ValidateFunc: func(t *testing.T, resp *http.Response) {
				t.Helper()

				// Standard validation
				ValidateErrorResponse(t, resp, 400, ErrorCodeInvalidRequest)

				// Custom body validation
				s3Err := GetS3ErrorFromResponse(t, resp)

				// Validate message contains specific keywords
				expectedKeywords := []string{"validation", "failed"}
				for _, keyword := range expectedKeywords {
					if !strings.Contains(strings.ToLower(s3Err.Message), keyword) {
						t.Errorf("Expected message to contain '%s', got '%s'", keyword, s3Err.Message)
					}
				}

				t.Log("Custom body validation passed")
			},
		}

		TestARMORErrorScenario(t, tc)
	})
}

// =============================================================================
// EXAMPLE 4: MULTIPLE HTTP METHODS
// =============================================================================

// TestExtensionExampleMultipleMethods demonstrates testing errors across HTTP methods.
//
// Use this pattern when an error can occur with different HTTP methods.
func TestExtensionExampleMultipleMethods(t *testing.T) {
	// Define test cases for different HTTP methods
	methods := []struct {
		name             string
		method           string
		body             string
		additionalTags   []string
	}{
		{
			name:           "GET request",
			method:         "GET",
			body:           "",
			additionalTags: []string{"get"},
		},
		{
			name:           "PUT request",
			method:         "PUT",
			body:           "test data",
			additionalTags: []string{"put"},
		},
		{
			name:           "POST request",
			method:         "POST",
			body:           `{"key": "value"}`,
			additionalTags: []string{"post"},
		},
		{
			name:           "DELETE request",
			method:         "DELETE",
			body:           "",
			additionalTags: []string{"delete"},
		},
	}

	// Create test cases for each method
	var testCases []ARMORErrorTestCase
	for _, methodInfo := range methods {
		testCases = append(testCases, ARMORErrorTestCase{
			Name:              fmt.Sprintf("Rate limit on %s", methodInfo.name),
			Description:       fmt.Sprintf("Tests 429 when rate limit exceeded on %s", methodInfo.name),
			StatusCode:        429,
			ErrorCode:         "TooManyRequests",
			Message:           "Rate limit exceeded",
			Path:              "/armor/blobs/file.dat",
			Method:            methodInfo.method,
			Body:              methodInfo.body,
			ExpectXMLStructure: true,
			MaxResponseTime:   200 * time.Millisecond,
			Category:          "RateLimit",
			Tags:              append([]string{"rate-limit", "429"}, methodInfo.additionalTags...),
			Headers: map[string]string{
				"Retry-After":           "60",
				"X-RateLimit-Remaining": "0",
			},
		})
	}

	// Create table
	table := CreateARMORTestTable(
		"Rate Limit Across Methods",
		"Tests rate limiting across different HTTP methods",
		testCases,
	)

	// Run all tests
	for _, tc := range table.TestCases {
		t.Run(tc.Name, func(t *testing.T) {
			TestARMORErrorScenario(t, tc)
		})
	}

	t.Logf("Successfully ran %d tests across different HTTP methods", len(table.TestCases))
}

// =============================================================================
// EXAMPLE 5: HEADER VALIDATION
// =============================================================================

// TestExtensionExampleHeaderValidation demonstrates comprehensive header validation.
//
// Use this pattern when error responses include important headers that need validation.
func TestExtensionExampleHeaderValidation(t *testing.T) {
	t.Run("ARMOR-specific headers", func(t *testing.T) {
		tc := ARMORErrorTestCase{
			Name:              "404 with ARMOR headers",
			Description:       "Tests 404 response with ARMOR-specific headers",
			StatusCode:        404,
			ErrorCode:         ErrorCodeNoSuchKey,
			Message:           "The specified key does not exist",
			Path:              "/armor/blobs/file.dat",
			ExpectXMLStructure: true,
			Category:          "NotFound",
			Tags:              []string{"404", "headers", "armor"},
			Headers: map[string]string{
				"Content-Type":       "application/xml",
				"X-ARMOR-Request-ID": "req-123456789",
				"X-ARMOR-Region":     "us-east-1",
				"X-ARMOR-Version":    "1.0.0",
			},
			ValidateFunc: func(t *testing.T, resp *http.Response) {
				t.Helper()

				// Standard validation
				ValidateErrorResponse(t, resp, 404, ErrorCodeNoSuchKey)

				// Validate ARMOR-specific headers
				AssertARMORHeader(t, resp, "X-ARMOR-Request-ID", "")
				AssertARMORHeader(t, resp, "X-ARMOR-Region", "us-east-1")
				AssertARMORHeader(t, resp, "X-ARMOR-Version", "1.0.0")

				// Ensure request ID is not empty
				requestID := resp.Header.Get("X-ARMOR-Request-ID")
				if requestID == "" {
					t.Error("X-ARMOR-Request-ID should not be empty")
				}

				t.Log("ARMOR header validation passed")
			},
		}

		TestARMORErrorScenario(t, tc)
	})

	t.Run("Retry header validation", func(t *testing.T) {
		tc := ARMORErrorTestCase{
			Name:              "503 with Retry-After",
			Description:       "Tests 503 response with Retry-After header",
			StatusCode:        503,
			ErrorCode:         ErrorCodeInternalError,
			Message:           "Service temporarily unavailable",
			Path:              "/armor/blobs/file.dat",
			ExpectXMLStructure: true,
			Category:          "Server",
			Tags:              []string{"503", "retry", "headers"},
			Headers: map[string]string{
				"Retry-After": "60",
			},
			ValidateFunc: func(t *testing.T, resp *http.Response) {
				t.Helper()

				// Standard validation
				ValidateErrorResponse(t, resp, 503, ErrorCodeInternalError)

				// Validate Retry-After header
				retryAfter := resp.Header.Get("Retry-After")
				if retryAfter == "" {
					t.Fatal("Expected Retry-After header to be present")
				}

				// Validate it's a valid number
				retrySeconds, err := strconv.Atoi(retryAfter)
				if err != nil {
					t.Errorf("Retry-After should be numeric, got '%s': %v", retryAfter, err)
				}

				// Validate reasonable range
				if retrySeconds < 0 || retrySeconds > 86400 {
					t.Errorf("Retry-After should be between 0 and 86400 seconds, got %d", retrySeconds)
				}

				t.Logf("Retry-After validation passed: %d seconds", retrySeconds)
			},
		}

		TestARMORErrorScenario(t, tc)
	})
}

// =============================================================================
// EXAMPLE 6: PERFORMANCE TESTING
// =============================================================================

// TestExtensionExamplePerformance demonstrates adding performance expectations.
//
// Use this pattern when you need to validate error response performance.
func TestExtensionExamplePerformance(t *testing.T) {
	t.Run("Fast error responses", func(t *testing.T) {
		tc := ARMORErrorTestCase{
			Name:              "404 with fast response",
			Description:       "Tests that 404 errors are returned quickly",
			StatusCode:        404,
			ErrorCode:         ErrorCodeNoSuchKey,
			Message:           "The specified key does not exist",
			Path:              "/armor/blobs/file.dat",
			ExpectXMLStructure: true,
			MinResponseTime:   1 * time.Millisecond,   // Should take at least 1ms
			MaxResponseTime:   100 * time.Millisecond, // Should not exceed 100ms
			Category:          "NotFound",
			Tags:              []string{"404", "performance", "fast"},
		}

		TestARMORErrorScenario(t, tc)
	})

	t.Run("Slower authentication responses", func(t *testing.T) {
		tc := ARMORErrorTestCase{
			Name:              "403 auth check with normal response time",
			Description:       "Tests that auth errors return within expected time",
			StatusCode:        403,
			ErrorCode:         ErrorCodeAccessDenied,
			Message:           "Access Denied",
			Path:              "/armor/blobs/file.dat",
			ExpectXMLStructure: true,
			MinResponseTime:   5 * time.Millisecond,   // Auth takes some time
			MaxResponseTime:   500 * time.Millisecond, // But should complete quickly
			Category:          "Auth",
			Tags:              []string{"403", "performance", "auth"},
		}

		TestARMORErrorScenario(t, tc)
	})

	t.Run("Server error with acceptable delay", func(t *testing.T) {
		tc := ARMORErrorTestCase{
			Name:              "500 server error with slower response",
			Description:       "Tests that server errors return within acceptable time",
			StatusCode:        500,
			ErrorCode:         ErrorCodeInternalError,
			Message:           "Internal server error",
			Path:              "/armor/blobs/file.dat",
			ExpectXMLStructure: true,
			MinResponseTime:   10 * time.Millisecond,    // Server errors may take longer
			MaxResponseTime:   1000 * time.Millisecond,  // But should still be reasonable
			Category:          "Server",
			Tags:              []string{"500", "performance", "slow"},
		}

		TestARMORErrorScenario(t, tc)
	})
}

// =============================================================================
// EXAMPLE 7: CONDITIONAL TESTING
// =============================================================================

// TestExtensionExampleConditional demonstrates conditional test execution.
//
// Use this pattern when tests should only run in certain environments.
func TestExtensionExampleConditional(t *testing.T) {
	t.Run("Environment-dependent test", func(t *testing.T) {
		// Create test case that checks environment
		tc := ARMORErrorTestCase{
			Name:              "Feature-specific error",
			Description:       "Tests an error that only occurs when feature is enabled",
			StatusCode:        400,
			ErrorCode:         ErrorCodeInvalidRequest,
			Message:           "Feature not enabled",
			Path:              "/armor/blobs/file.dat",
			ExpectXMLStructure: true,
			Category:          "FeatureFlag",
			Tags:              []string{"feature", "conditional"},
			SetupFunc: func(t *testing.T, server *ConfigurableErrorServer) {
				t.Helper()

				// Check if feature is enabled
				featureEnabled := false // In real tests, check env var or config
				if !featureEnabled {
					t.Skip("Feature not enabled in this environment")
				}

				t.Log("Feature is enabled, running test")
			},
		}

		TestARMORErrorScenario(t, tc)
	})

	t.Run("Skip with reason", func(t *testing.T) {
		// Create test case that is always skipped
		tc := ARMORErrorTestCase{
			Name:              "Skipped test example",
			Description:       "This test is always skipped as an example",
			StatusCode:        418,
			ErrorCode:         "ImATeapot",
			Message:           "I'm a teapot",
			Path:              "/armor/blobs/tea",
			Category:          "Example",
			SkipReason:        "This is an example of skipping a test",
		}

		TestARMORErrorScenario(t, tc)
	})
}

// =============================================================================
// EXAMPLE 8: TABLE MERGING
// =============================================================================

// TestExtensionExampleTableMerging demonstrates merging multiple test tables.
//
// Use this pattern when you want to combine several test tables into one.
func TestExtensionExampleTableMerging(t *testing.T) {
	t.Run("Merge predefined tables", func(t *testing.T) {
		// Get multiple predefined tables
		basic := ARMORErrorTestTables.BasicErrorTests()
		auth := ARMORErrorTestTables.AuthenticationErrors()
		validation := ARMORErrorTestTables.ValidationErrors()

		// Create custom rate limit table
		customRateLimit := ARMORErrorTestTable{
			Name:        "Custom Rate Limits",
			Description: "Custom rate limit error scenarios",
			TestCases: []ARMORErrorTestCase{
				{
					Name:       "429 too many requests",
					StatusCode: 429,
					ErrorCode:  "TooManyRequests",
					Message:    "Rate limit exceeded",
					Path:       "/armor/blobs/file.dat",
					Category:   "RateLimit",
					Tags:       []string{"rate-limit", "429"},
				},
			},
		}

		// Merge all tables using helper
		merged := MergeARMORTestTables(basic, auth, validation, customRateLimit)

		t.Logf("Merged table name: %s", merged.Name)
		t.Logf("Total test cases: %d", len(merged.TestCases))

		// Verify merge worked
		expectedCount := len(basic.TestCases) + len(auth.TestCases) +
			len(validation.TestCases) + len(customRateLimit.TestCases)
		if len(merged.TestCases) != expectedCount {
			t.Errorf("Expected %d cases, got %d", expectedCount, len(merged.TestCases))
		}

		// Run a sample of tests (not all to save time)
		runCount := 0
		for _, tc := range merged.TestCases {
			if runCount >= 3 {
				break // Only run first 3 as example
			}
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
			runCount++
		}

		t.Logf("Successfully ran sample of %d tests from merged table", runCount)
	})
}

// =============================================================================
// EXAMPLE 9: FILTERING
// =============================================================================

// TestExtensionExampleFiltering demonstrates filtering tests by category and tags.
//
// Use this pattern when you want to run specific subsets of tests.
func TestExtensionExampleFiltering(t *testing.T) {
	t.Run("Filter by category", func(t *testing.T) {
		// Get all tests
		all := ARMORErrorTestTables.AllTests()

		// Filter by Auth category
		authTests := filterARMORTestsByCategory(all, "Auth")

		t.Logf("Found %d auth tests", len(authTests))

		if len(authTests) == 0 {
			t.Error("Expected to find at least one auth test")
		}

		// Run auth tests
		for _, tc := range authTests {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}

		t.Logf("Successfully ran %d auth tests", len(authTests))
	})

	t.Run("Filter by tag", func(t *testing.T) {
		// Get all tests
		all := ARMORErrorTestTables.AllTests()

		// Filter by "404" tag
		notFoundTests := filterARMORTestsByTag(all, "404")

		t.Logf("Found %d tests with tag '404'", len(notFoundTests))

		if len(notFoundTests) == 0 {
			t.Error("Expected to find at least one test with tag '404'")
		}

		// Run 404 tests
		for _, tc := range notFoundTests {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}

		t.Logf("Successfully ran %d tests with tag '404'", len(notFoundTests))
	})

	t.Run("Filter by multiple tags", func(t *testing.T) {
		// Get all tests
		all := ARMORErrorTestTables.AllTests()

		// Find tests with both "auth" and "403" tags
		var auth403Tests []ARMORErrorTestCase
		for _, tc := range all.TestCases {
			hasAuthTag := false
			has403Tag := false
			for _, tag := range tc.Tags {
				if tag == "auth" {
					hasAuthTag = true
				}
				if tag == "403" {
					has403Tag = true
				}
			}
			if hasAuthTag && has403Tag {
				auth403Tests = append(auth403Tests, tc)
			}
		}

		t.Logf("Found %d tests with both 'auth' and '403' tags", len(auth403Tests))

		if len(auth403Tests) == 0 {
			t.Error("Expected to find at least one test with both 'auth' and '403' tags")
		}

		// Run filtered tests
		for _, tc := range auth403Tests {
			t.Run(tc.Name, func(t *testing.T) {
				TestARMORErrorScenario(t, tc)
			})
		}

		t.Logf("Successfully ran %d tests with both 'auth' and '403' tags", len(auth403Tests))
	})
}

// =============================================================================
// EXAMPLE 10: COMPLETE WORKFLOW
// =============================================================================

// TestExtensionExampleCompleteWorkflow demonstrates a complete extension workflow.
//
// This example shows the entire process from planning to execution.
func TestExtensionExampleCompleteWorkflow(t *testing.T) {
	// Step 1: Define the error type
	type CustomErrorType struct {
		Name        string
		StatusCode  int
		ErrorCode   string
		Description string
	}

	customError := CustomErrorType{
		Name:        "RateLimitExceeded",
		StatusCode:  429,
		ErrorCode:   "TooManyRequests",
		Description: "Returned when rate limit is exceeded",
	}

	// Step 2: Create test cases for different scenarios
	testCases := []ARMORErrorTestCase{
		{
			Name:              fmt.Sprintf("%s - default tier", customError.Name),
			Description:       fmt.Sprintf("Tests %s on default rate limit tier", customError.Description),
			StatusCode:        customError.StatusCode,
			ErrorCode:         customError.ErrorCode,
			Message:           "Rate limit exceeded",
			Path:              "/armor/blobs/file.dat",
			ExpectXMLStructure: true,
			MaxResponseTime:   200 * time.Millisecond,
			Category:          "RateLimit",
			Tags:              []string{"rate-limit", "429", "default"},
		},
		{
			Name:              fmt.Sprintf("%s - premium tier", customError.Name),
			Description:       fmt.Sprintf("Tests %s on premium rate limit tier", customError.Description),
			StatusCode:        customError.StatusCode,
			ErrorCode:         customError.ErrorCode,
			Message:           "API rate limit exceeded",
			Path:              "/api/v1/blobs/file.dat",
			ExpectXMLStructure: true,
			MaxResponseTime:   200 * time.Millisecond,
			Category:          "RateLimit",
			Tags:              []string{"rate-limit", "429", "premium"},
		},
	}

	// Step 3: Extend with base tests if needed
	base := ARMORErrorTestTables.BasicErrorTests()
	extended := ExtendARMORTestTable(base, testCases)

	// Step 4: Run all tests
	for _, tc := range extended.TestCases {
		t.Run(tc.Name, func(t *testing.T) {
			TestARMORErrorScenario(t, tc)
		})
	}

	t.Logf("Successfully completed workflow with %d total tests", len(extended.TestCases))
}
