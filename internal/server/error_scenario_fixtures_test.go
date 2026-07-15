package server

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// ERROR SCENARIO FIXTURES
// =============================================================================
// This file provides reusable test fixtures for common error response scenarios.
// These fixtures enable rapid test authoring for new error types.
//
// Usage:
//   1. Use ErrorScenario struct to define test scenarios
//   2. Use predefined fixtures (400, 404, 500) for common cases
//   3. Use runScenario() helper to execute and validate
//   4. See table-driven examples at the bottom of this file
// =============================================================================

// =============================================================================
// ERROR SCENARIO STRUCT
// =============================================================================
// The ErrorScenario struct is defined in error_test_infrastructure_test.go
// This file provides fixture functions that return ErrorScenario instances
// =============================================================================

// =============================================================================
// COMMON ERROR SCENARIO FIXTURES
// =============================================================================
// These fixtures represent the most common error scenarios and can be used
// directly in tests or as templates for custom scenarios.
// =============================================================================

// BadRequestScenario returns a fixture for 400 Bad Request with validation errors.
//
// This scenario tests requests that are malformed or missing required parameters.
// Common causes:
//   - Missing required query parameters
//   - Invalid query string values
//   - Malformed request body
//   - Invalid header values
//
// Usage:
//
//	scenario := BadRequestScenario()
//	scenario.Name = "My custom bad request test"
//	scenario.SetupRequest = func(t *testing.T) *http.Request {
//	    return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/key?invalid=param")
//	}
//	result := runScenario(t, fixture, scenario)
func BadRequestScenario() ErrorScenario {
	return ErrorScenario{
		Name:        "Bad Request - Invalid Request",
		Description: "Request with invalid parameters should return 400 Bad Request",
		SetupRequest: func(t *testing.T) *http.Request {
			// Create an authenticated request with invalid query parameters
			return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/key?invalid=parameter&malformed=value")
		},
		ExpectedStatus:   400,
		ExpectedCode:     ErrorCodeInvalidRequest,
		ExpectedKeywords: []string{"invalid", "request", "parameter"},
		MinMessageLength: 15,
		MaxResponseTime:  100 * time.Millisecond,
	}
}

// BadRequestWithMissingContentTypeScenario returns a fixture for 400 Bad Request with missing Content-Type.
//
// This scenario tests requests that should have a Content-Type header but don't.
// Common causes:
//   - PUT/POST requests without Content-Type
//   - Missing required content negotiation headers
//
// Usage:
//
//	scenario := BadRequestWithMissingContentTypeScenario()
//	result := runScenario(t, fixture, scenario)
func BadRequestWithMissingContentTypeScenario() ErrorScenario {
	return ErrorScenario{
		Name:        "Bad Request - Missing Content-Type",
		Description: "Request without Content-Type header should return 400",
		SetupRequest: func(t *testing.T) *http.Request {
			body := `{"data": "test"}`
			return createSignedRequestForAuthTest(t, "PUT", "/test-bucket/key", body, "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
		},
		ExpectedStatus:   400,
		ExpectedCode:     ErrorCodeInvalidRequest,
		ExpectedKeywords: []string{"content", "type", "required"},
		MinMessageLength: 15,
		MaxResponseTime:  100 * time.Millisecond,
	}
}

// BadRequestWithInvalidQueryScenario returns a fixture for 400 Bad Request with invalid query string.
//
// This scenario tests requests with malformed or unsupported query parameters.
// Common causes:
//   - Invalid query parameter names
//   - Invalid query parameter values
//   - Conflicting query parameters
//
// Usage:
//
//	scenario := BadRequestWithInvalidQueryScenario()
//	result := runScenario(t, fixture, scenario)
func BadRequestWithInvalidQueryScenario() ErrorScenario {
	return ErrorScenario{
		Name:        "Bad Request - Invalid Query Parameters",
		Description: "Request with invalid query parameters should return 400",
		SetupRequest: func(t *testing.T) *http.Request {
			return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/key?unsupported-parameter=value&conflicting=param")
		},
		ExpectedStatus:   400,
		ExpectedCode:     ErrorCodeInvalidRequest,
		ExpectedKeywords: []string{"invalid", "query", "parameter"},
		MinMessageLength: 15,
		MaxResponseTime:  100 * time.Millisecond,
	}
}

// NotFoundScenario returns a fixture for 404 Not Found with resource error.
//
// This scenario tests requests for non-existent resources.
// Common causes:
//   - Object was deleted
//   - Wrong bucket name
//   - Wrong key name
//   - Typo in resource path
//
// Usage:
//
//	scenario := NotFoundScenario()
//	result := runScenario(t, fixture, scenario)
func NotFoundScenario() ErrorScenario {
	return ErrorScenario{
		Name:        "Not Found - Non-existent Resource",
		Description: "Request for non-existent resource should return 404",
		SetupRequest: func(t *testing.T) *http.Request {
			return createSignedRequestForAuthTest(t, "GET", "/test-bucket/nonexistent-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
		},
		ExpectedStatus:   404,
		ExpectedCode:     ErrorCodeNoSuchKey,
		ExpectedKeywords: []string{"not", "found", "exist"},
		MinMessageLength: 15,
		MaxResponseTime:  100 * time.Millisecond,
	}
}

// NotFoundWithBucketScenario returns a fixture for 404 with non-existent bucket.
//
// This scenario tests requests for a bucket that doesn't exist.
//
// Usage:
//
//	scenario := NotFoundWithBucketScenario("nonexistent-bucket")
//	result := runScenario(t, fixture, scenario)
func NotFoundWithBucketScenario(bucket string) ErrorScenario {
	return ErrorScenario{
		Name:        "Not Found - Non-existent Bucket",
		Description: "Request for non-existent bucket should return 404",
		SetupRequest: func(t *testing.T) *http.Request {
			return createSignedRequestForAuthTest(t, "GET", "/"+bucket+"/key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
		},
		ExpectedStatus:   404,
		ExpectedCode:     ErrorCodeNoSuchKey,
		ExpectedKeywords: []string{"bucket", "not", "found"},
		MinMessageLength: 15,
		MaxResponseTime:  100 * time.Millisecond,
	}
}

// InternalServerErrorScenario returns a fixture for 500 Internal Server Error.
//
// This scenario tests unexpected server-side errors.
// Common causes:
//   - Server-side bug
//   - Infrastructure failure
//   - Database connection failure
//   - Unexpected error condition
//
// NOTE: This scenario requires special server setup or mocking to trigger a genuine
// 500 error. In production, ARMOR is designed to handle errors gracefully and return
// appropriate 4xx errors. To test 500 errors, you may need to:
// 1. Modify the server to inject errors for testing
// 2. Use a mock server that returns 500 errors
// 3. Test against a server with induced failures (e.g., disable B2 backend)
//
// Usage:
//
//	scenario := InternalServerErrorScenario()
//	result := runScenario(t, fixture, scenario)
//	// If result.Passed is false, check if the server is configured to return 500
func InternalServerErrorScenario() ErrorScenario {
	return ErrorScenario{
		Name:        "Internal Server Error",
		Description: "Server error should return 500 with error details (requires special setup)",
		SetupRequest: func(t *testing.T) *http.Request {
			// This creates a request that might trigger an internal error
			// NOTE: In the current ARMOR implementation, this path returns 403
			// To properly test 500 errors, you need to:
			// 1. Use a mock server, or
			// 2. Disable the B2 backend to trigger connection errors, or
			// 3. Add server-side error injection for testing
			return createSignedRequestForAuthTest(t, "GET", "/test-bucket/key-that-does-not-exist", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
		},
		ExpectedStatus:   500,
		ExpectedCode:     ErrorCodeInternalError,
		ExpectedKeywords: []string{"internal", "error"},
		MinMessageLength: 15,
		MaxResponseTime:  100 * time.Millisecond,
	}
}

// =============================================================================
// SCENARIO EXECUTION HELPERS
// =============================================================================

// runScenario executes a single error scenario and validates the outcome.
//
// This helper:
// 1. Creates the request using the scenario's SetupRequest function
// 2. Executes the request against the test server fixture
// 3. Measures response time
// 4. Validates the response against the scenario's expectations
// 5. Returns a result object with the outcome
//
// Parameters:
//   - t: Testing instance for context and assertions
//   - fixture: Test server fixture to execute the request against
//   - scenario: Error scenario to execute and validate
//
// Returns: ErrorScenarioResult with the execution details and validation outcome
//
// Example:
//
//	result := runScenario(t, fixture, scenario)
//	if !result.Passed {
//	    t.Logf("Scenario failed: %s - %s", result.Name, result.ErrorMessage)
//	}
func runScenario(t *testing.T, fixture *TestServerFixture, scenario ErrorScenario) *ErrorScenarioResult {
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

	// Validate HTTP status
	if w.Code != scenario.ExpectedStatus {
		t.Errorf("%s: Expected status %d, got %d", scenario.Name, scenario.ExpectedStatus, w.Code)
		result.Passed = false
	}

	// Validate error code
	if s3Err.Code != scenario.ExpectedCode {
		t.Errorf("%s: Expected error code '%s', got '%s'", scenario.Name, scenario.ExpectedCode, s3Err.Code)
		result.Passed = false
	}

	// Validate message length
	minLength := scenario.MinMessageLength
	if minLength == 0 {
		minLength = 15 // Default minimum
	}
	if len(s3Err.Message) < minLength {
		t.Errorf("%s: Error message too short (got %d chars, want at least %d): %s",
			scenario.Name, len(s3Err.Message), minLength, s3Err.Message)
		result.Passed = false
	}

	// Validate keywords if specified
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

	// Validate response time if specified
	if scenario.MaxResponseTime > 0 && duration > scenario.MaxResponseTime {
		t.Errorf("%s: Response time %v exceeds maximum %v", scenario.Name, duration, scenario.MaxResponseTime)
		result.Passed = false
	}

	// Validate Content-Type header
	expectedContentType := "application/xml"
	actualContentType := w.Header().Get("Content-Type")
	if actualContentType != expectedContentType {
		t.Errorf("%s: Expected Content-Type '%s', got '%s'", scenario.Name, expectedContentType, actualContentType)
		result.Passed = false
	}

	return result
}

// =============================================================================
// TABLE-DRIVEN TEST EXAMPLES
// =============================================================================
// The following examples demonstrate how to use the fixtures in table-driven tests.
// =============================================================================

/*
Example 1: Basic error scenario table

	func TestBasicErrorScenarios(t *testing.T) {
	    fixture := NewTestServer(t)

	    scenarios := []ErrorScenario{
	        BadRequestScenario(),
	        NotFoundScenario(),
	        InternalServerErrorScenario(),
	    }

	    for _, scenario := range scenarios {
	        t.Run(scenario.Name, func(t *testing.T) {
	            result := runScenario(t, fixture, scenario)
	            if !result.Passed {
	                t.Errorf("Scenario failed: %s", result.Message)
	            }
	        })
	    }
	}
*/

/*
Example 2: Customized scenarios based on fixtures

	func TestCustomBadRequestScenarios(t *testing.T) {
	    fixture := NewTestServer(t)

	    scenarios := []ErrorScenario{
	        {
	            Name:        "Empty body returns 400",
	        Description: "PUT request with empty body should return 400",
	            SetupRequest: func(t *testing.T) *http.Request {
	                return CreateAuthenticatedTestRequest(t, "PUT", "/test-bucket/key")
	            },
	            ExpectedStatus:    400,
	            ExpectedCode:      ErrorCodeInvalidRequest,
	            ExpectedKeywords:  []string{"body", "required"},
	            MinMessageLength:  15,
	        },
	        {
	            Name:        "Invalid character in key",
	        Description: "Key with invalid characters should return 400",
	            SetupRequest: func(t *testing.T) *http.Request {
	                return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/key/with/invalid\\chars")
	            },
	            ExpectedStatus:    400,
	            ExpectedCode:      ErrorCodeInvalidRequest,
	            ExpectedKeywords:  []string{"key", "invalid"},
	            MinMessageLength:  15,
	        },
	    }

	    for _, scenario := range scenarios {
	        t.Run(scenario.Name, func(t *testing.T) {
	            result := runScenario(t, fixture, scenario)
	            if !result.Passed {
	                t.Errorf("Scenario failed: %s", result.Message)
	            }
	        })
	    }
	}
*/

/*
Example 3: Extended scenarios with performance checks

	func TestExtendedNotFoundScenarios(t *testing.T) {
	    fixture := NewTestServer(t)

	    scenarios := []ErrorScenario{
	        func() ErrorScenario {
	            scenario := NotFoundScenario()
	            scenario.Name = "Non-existent key with performance check"
	            scenario.MaxResponseTime = 50 * time.Millisecond // Stricter timeout
	            return scenario
	        }(),
	        func() ErrorScenario {
	            scenario := NotFoundWithBucketScenario("my-nonexistent-bucket")
	            scenario.Name = "Non-existent bucket with performance check"
	            scenario.MaxResponseTime = 50 * time.Millisecond
	            return scenario
	        }(),
	    }

	    for _, scenario := range scenarios {
	        t.Run(scenario.Name, func(t *testing.T) {
	            result := runScenario(t, fixture, scenario)
	            if !result.Passed {
	                t.Errorf("Scenario failed: %s", result.Message)
	            }
	        })
	    }
	}
*/

/*
Example 4: Performance-focused scenarios

	func TestErrorResponsePerformance(t *testing.T) {
	    fixture := NewTestServer(t)

	    scenarios := []ErrorScenario{
	        func() ErrorScenario {
	            scenario := BadRequestScenario()
	            scenario.Name = "Bad Request - Performance Test"
	            scenario.MaxResponseTime = 50 * time.Millisecond // Stricter timeout
	            return scenario
	        }(),
	        func() ErrorScenario {
	            scenario := NotFoundScenario()
	            scenario.Name = "Not Found - Performance Test"
	            scenario.MaxResponseTime = 50 * time.Millisecond
	            return scenario
	        }(),
	    }

	    for _, scenario := range scenarios {
	        t.Run(scenario.Name, func(t *testing.T) {
	            result := runScenario(t, fixture, scenario)
	            if !result.Passed {
	                t.Errorf("Scenario failed: %s (duration: %v)", result.Message, result.Duration)
	            } else {
	                t.Logf("Performance OK: %v", result.Duration)
	            }
	        })
	    }
	}
*/

/*
Example 5: Comprehensive error type coverage

	func TestComprehensiveErrorCoverage(t *testing.T) {
	    fixture := NewTestServer(t)

	    scenarios := []ErrorScenario{
	        // 400 Bad Request scenarios
	        BadRequestScenario(),
	        BadRequestWithMissingContentTypeScenario(),
	        BadRequestWithInvalidQueryScenario(),

	        // 404 Not Found scenarios
	        NotFoundScenario(),
	        NotFoundWithBucketScenario("nonexistent-bucket-1"),
	        NotFoundWithBucketScenario("nonexistent-bucket-2"),

	        // 500 Internal Server Error scenarios
	        InternalServerErrorScenario(),

	        // Custom scenarios for additional error types
	        {
	            Name:        "Method Not Allowed",
	        Description: "Unsupported HTTP method should return 405",
	            SetupRequest: func(t *testing.T) *http.Request {
	                return CreateAuthenticatedTestRequest(t, "POST", "/test-bucket/key")
	            },
	            ExpectedStatus:    405,
	            ExpectedCode:      ErrorCodeMethodNotAllowed,
	            ExpectedKeywords:  []string{"method", "allowed"},
	            MinMessageLength:  15,
	        },
	    }

	    passed := 0
	    failed := 0

	    for _, scenario := range scenarios {
	        t.Run(scenario.Name, func(t *testing.T) {
	            result := runScenario(t, fixture, scenario)
	            if result.Passed {
	                passed++
	            } else {
	                failed++
	                t.Errorf("Scenario validation failed")
	            }
	        })
	    }

	    t.Logf("Error coverage: %d passed, %d failed", passed, failed)
	}
*/

/*
Example 6: Rapid test authoring for new error types

	// When adding a new error type, simply define a fixture function:

	func MyNewErrorScenario() ErrorScenario {
	    return ErrorScenario{
	        Name:        "My New Error Type",
	    Description: "Description of what this error tests",
	        SetupRequest: func(t *testing.T) *http.Request {
	            return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/special-case")
	        },
	        ExpectedStatus:    418, // I'm a teapot
	        ExpectedCode:      "ImATeapot",
	        ExpectedKeywords:  []string{"teapot", "coffee"},
	        MinMessageLength:  15,
	    }
	}

	// Then use it in your test:

	func TestMyNewErrorType(t *testing.T) {
	    fixture := NewTestServer(t)
	    result := runScenario(t, fixture, MyNewErrorScenario())
	    if !result.Passed {
	        t.Errorf("New error type failed: %s", result.Message)
	    }
	}
*/
