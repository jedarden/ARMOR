package server

import (
	"net/http"
	"testing"
	"time"
)

// =============================================================================
// TABLE-DRIVEN TEST EXAMPLES USING ERROR SCENARIO FIXTURES
// =============================================================================
// This file demonstrates how to use the error scenario fixtures for rapid test authoring.
// Each test shows a different pattern for using the fixtures.
// =============================================================================

// TestBasicErrorScenarios demonstrates basic usage of predefined fixtures.
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

// TestBadRequestVariations demonstrates different 400 Bad Request scenarios.
func TestBadRequestVariations(t *testing.T) {
	fixture := NewTestServer(t)

	scenarios := []ErrorScenario{
		BadRequestScenario(),
		BadRequestWithMissingContentTypeScenario(),
		BadRequestWithInvalidQueryScenario(),
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			result := runScenario(t, fixture, scenario)
			if !result.Passed {
				t.Errorf("Bad request scenario failed: %s", result.Message)
			}
		})
	}
}

// TestNotFoundVariations demonstrates different 404 Not Found scenarios.
func TestNotFoundVariations(t *testing.T) {
	fixture := NewTestServer(t)

	scenarios := []ErrorScenario{
		NotFoundScenario(),
		NotFoundWithBucketScenario("nonexistent-bucket-1"),
		NotFoundWithBucketScenario("another-missing-bucket"),
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			result := runScenario(t, fixture, scenario)
			if !result.Passed {
				t.Errorf("Not found scenario failed: %s", result.Message)
			}
		})
	}
}

// TestCustomErrorScenarios demonstrates how to create custom scenarios based on fixtures.
func TestCustomErrorScenarios(t *testing.T) {
	fixture := NewTestServer(t)

	scenarios := []ErrorScenario{
		{
			Name:        "Custom - Empty body returns 400",
			Description: "PUT request with empty body should return 400",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "PUT", "/test-bucket/key")
			},
			ExpectedStatus:   400,
			ExpectedCode:     ErrorCodeInvalidRequest,
			ExpectedKeywords: []string{"body", "required"},
			MinMessageLength: 15,
		},
		{
			Name:        "Custom - Invalid character in key",
			Description: "Key with invalid characters should return 400",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/key/with\\invalid")
			},
			ExpectedStatus:   400,
			ExpectedCode:     ErrorCodeInvalidRequest,
			ExpectedKeywords: []string{"key", "invalid"},
			MinMessageLength: 15,
		},
		{
			Name:        "Custom - Unsupported method",
			Description: "POST request should return 405",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "POST", "/test-bucket/key")
			},
			ExpectedStatus:   405,
			ExpectedCode:     ErrorCodeMethodNotAllowed,
			ExpectedKeywords: []string{"method", "allowed"},
			MinMessageLength: 15,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			result := runScenario(t, fixture, scenario)
			if !result.Passed {
				t.Errorf("Custom scenario failed: %s", result.Message)
			}
		})
	}
}

// TestExtendedScenariosWithPerformanceChecks demonstrates scenarios with strict response time requirements.
func TestExtendedScenariosWithPerformanceChecks(t *testing.T) {
	fixture := NewTestServer(t)

	scenarios := []ErrorScenario{
		func() ErrorScenario {
			scenario := NotFoundScenario()
			scenario.Name = "Extended - NotFound with performance check"
			scenario.MaxResponseTime = 50 * time.Millisecond
			return scenario
		}(),
		func() ErrorScenario {
			scenario := BadRequestScenario()
			scenario.Name = "Extended - BadRequest with response time check"
			scenario.MaxResponseTime = 50 * time.Millisecond
			return scenario
		}(),
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			result := runScenario(t, fixture, scenario)
			if !result.Passed {
				t.Errorf("Extended scenario failed: %s", result.Message)
			}
		})
	}
}

// TestComprehensiveErrorCoverage demonstrates coverage across all error types.
func TestComprehensiveErrorCoverage(t *testing.T) {
	fixture := NewTestServer(t)

	scenarios := []ErrorScenario{
		// 400 Bad Request scenarios
		BadRequestScenario(),
		BadRequestWithMissingContentTypeScenario(),
		BadRequestWithInvalidQueryScenario(),

		// 404 Not Found scenarios
		NotFoundScenario(),
		NotFoundWithBucketScenario("test-bucket-1"),
		NotFoundWithBucketScenario("test-bucket-2"),

		// 500 Internal Server Error scenarios
		InternalServerErrorScenario(),

		// Custom scenarios for additional error types
		{
			Name:        "Method Not Allowed",
			Description: "Unsupported HTTP method should return 405",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "POST", "/test-bucket/key")
			},
			ExpectedStatus:   405,
			ExpectedCode:     ErrorCodeMethodNotAllowed,
			ExpectedKeywords: []string{"method", "allowed"},
			MinMessageLength: 15,
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

// TestRapidErrorTestAuthoring demonstrates how quickly new error tests can be added.
func TestRapidErrorTestAuthoring(t *testing.T) {
	fixture := NewTestServer(t)

	// Example: Adding a new error type is as simple as defining a fixture
	customNotFoundError := func() ErrorScenario {
		return ErrorScenario{
			Name:        "Custom Not Found",
			Description: "Custom not found scenario",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/custom-missing")
			},
			ExpectedStatus:   404,
			ExpectedCode:     ErrorCodeNoSuchKey,
			ExpectedKeywords: []string{"not", "found"},
			MinMessageLength: 15,
		}
	}

	scenarios := []ErrorScenario{
		customNotFoundError(),
		BadRequestScenario(),
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			result := runScenario(t, fixture, scenario)
			if !result.Passed {
				t.Errorf("Rapid test scenario failed: %s", result.Message)
			}
		})
	}
}

// TestFixtureCustomization demonstrates how to customize predefined fixtures.
func TestFixtureCustomization(t *testing.T) {
	fixture := NewTestServer(t)

	// Customize the base BadRequestScenario
	customBadRequest := BadRequestScenario()
	customBadRequest.Name = "Customized - Special bad request case"
	customBadRequest.Description = "A customized version of the bad request scenario"
	customBadRequest.ExpectedKeywords = []string{"special", "validation"}

	scenarios := []ErrorScenario{
		customBadRequest,
		NotFoundScenario(),
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			result := runScenario(t, fixture, scenario)
			if !result.Passed {
				t.Errorf("Customized fixture failed: %s", result.Message)
			}
		})
	}
}
