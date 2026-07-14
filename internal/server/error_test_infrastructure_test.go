package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestErrorTestInfrastructure verifies the test infrastructure itself works correctly.
// These are meta-tests that validate the testing helpers, not the ARMOR server.

func TestNewTestServer(t *testing.T) {
	fixture := NewTestServer(t)

	if fixture == nil {
		t.Fatal("NewTestServer returned nil fixture")
	}

	if fixture.Config == nil {
		t.Error("Fixture Config is nil")
	}

	if fixture.Handler == nil {
		t.Error("Fixture Handler is nil")
	}

	if fixture.Config.Bucket != "test-bucket" {
		t.Errorf("Expected bucket 'test-bucket', got '%s'", fixture.Config.Bucket)
	}
}

func TestDefaultTestCredentials(t *testing.T) {
	creds := DefaultTestCredentials()

	if creds == nil {
		t.Fatal("DefaultTestCredentials returned nil")
	}

	if creds.ValidAccessKey == "" {
		t.Error("ValidAccessKey is empty")
	}

	if creds.ValidSecretKey == "" {
		t.Error("ValidSecretKey is empty")
	}

	if creds.InvalidAccessKey == "" {
		t.Error("InvalidAccessKey is empty")
	}

	if creds.InvalidSecretKey == "" {
		t.Error("InvalidSecretKey is empty")
	}
}

func TestVerifyErrorResponse_BasicValidation(t *testing.T) {
	fixture := NewTestServer(t)

	// Create a request that will produce an error
	req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/nonexistent-key")
	w := httptest.NewRecorder()
	fixture.Handler.ServeHTTP(w, req)

	// Use the fluent validator
	VerifyErrorResponse(t, w).
		HTTPStatusCode(404).
		ContentType("application/xml").
		BodyNotEmpty().
		HasCode("NoSuchKey").
		MessageMinLength(15).
		HasXMLDeclaration().
		Assert()
}

func TestVerifyErrorResponse_MessageValidation(t *testing.T) {
	fixture := NewTestServer(t)

	// Test NoSuchKey scenario
	req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/missing-object")
	w := httptest.NewRecorder()
	fixture.Handler.ServeHTTP(w, req)

	// Use message validation
	VerifyErrorResponse(t, w).
		HasCode("NoSuchKey").
		MessageContainsAny("not found", "object", "key").
		Assert()
}

func TestVerifyErrorResponse_WithTiming(t *testing.T) {
	fixture := NewTestServer(t)

	req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/test-key")

	duration, w := MeasureRequestTime(fixture.Handler, req)

	// Verify with timing
	VerifyErrorResponseWithTiming(t, w, duration).
		HTTPStatusCode(404).
		ResponseTime(100 * time.Millisecond).
		Assert()
}

func TestVerifyStandardErrorResponse(t *testing.T) {
	fixture := NewTestServer(t)

	req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/missing")
	w := httptest.NewRecorder()
	fixture.Handler.ServeHTTP(w, req)

	// Use the convenience function
	VerifyStandardErrorResponse(t, w, "NoSuchKey", 404)
}

func TestParseErrorResponse(t *testing.T) {
	fixture := NewTestServer(t)

	req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/missing")
	w := httptest.NewRecorder()
	fixture.Handler.ServeHTTP(w, req)

	s3Err := ParseErrorResponse(t, w.Body.Bytes())

	if s3Err.Code != "NoSuchKey" {
		t.Errorf("Expected error code 'NoSuchKey', got '%s'", s3Err.Code)
	}

	if s3Err.Message == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestCreateTestRequest(t *testing.T) {
	t.Run("Basic request creation", func(t *testing.T) {
		req := CreateTestRequest(t, "GET", "/test-bucket/key", nil, nil)

		if req.Method != "GET" {
			t.Errorf("Expected method GET, got %s", req.Method)
		}

		if req.URL.Path != "/test-bucket/key" {
			t.Errorf("Expected path /test-bucket/key, got %s", req.URL.Path)
		}
	})

	t.Run("Request with body", func(t *testing.T) {
		body := strings.NewReader("test body")
		req := CreateTestRequest(t, "PUT", "/test-bucket/key", body, nil)

		if req.Method != "PUT" {
			t.Errorf("Expected method PUT, got %s", req.Method)
		}
	})

	t.Run("Request with custom headers", func(t *testing.T) {
		headers := map[string]string{
			"X-Custom-Header": "custom-value",
			"X-Another":      "another-value",
		}
		req := CreateTestRequest(t, "GET", "/test-bucket/key", nil, headers)

		if req.Header.Get("X-Custom-Header") != "custom-value" {
			t.Error("Custom header not set")
		}

		if req.Header.Get("X-Another") != "another-value" {
			t.Error("Another custom header not set")
		}
	})
}

func TestCreateAuthenticatedTestRequest(t *testing.T) {
	req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/key")

	if req == nil {
		t.Fatal("CreateAuthenticatedTestRequest returned nil")
	}

	if req.Method != "GET" {
		t.Errorf("Expected method GET, got %s", req.Method)
	}

	auth := req.Header.Get("Authorization")
	if auth == "" {
		t.Error("Authorization header not set")
	}

	if !strings.Contains(auth, "AWS4-HMAC-SHA256") {
		t.Error("Authorization header missing AWS4-HMAC-SHA256")
	}
}

func TestRunErrorScenario(t *testing.T) {
	fixture := NewTestServer(t)

	scenario := ErrorScenario{
		Name: "Test scenario",
		SetupRequest: func(t *testing.T) *http.Request {
			return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/nonexistent")
		},
		ExpectedCode:     "NoSuchKey",
		ExpectedStatus:   404,
		MinMessageLength:  15,
		MaxResponseTime:  100 * time.Millisecond,
		ExpectedKeywords: []string{"not found"},
	}

	result := RunErrorScenario(t, fixture, scenario)

	if !result.Passed {
		t.Errorf("Scenario should have passed, got: %+v", result)
	}

	if result.ScenarioName != "Test scenario" {
		t.Errorf("Expected scenario name 'Test scenario', got '%s'", result.ScenarioName)
	}

	if result.ErrorCode != "NoSuchKey" {
		t.Errorf("Expected error code 'NoSuchKey', got '%s'", result.ErrorCode)
	}

	if result.StatusCode != 404 {
		t.Errorf("Expected status 404, got %d", result.StatusCode)
	}

	if result.Duration > 100*time.Millisecond {
		t.Errorf("Response time %v exceeds threshold", result.Duration)
	}
}

func TestErrorTestSuite(t *testing.T) {
	t.Run("Create and run suite", func(t *testing.T) {
		suite := NewErrorTestSuite(t, "Test Suite")

		if suite == nil {
			t.Fatal("NewErrorTestSuite returned nil")
		}

		if suite.name != "Test Suite" {
			t.Errorf("Expected suite name 'Test Suite', got '%s'", suite.name)
		}

		if suite.fixture == nil {
			t.Error("Suite fixture is nil")
		}

		// Add scenarios
		suite.AddScenario(ErrorScenario{
			Name: "Scenario 1",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/key1")
			},
			ExpectedCode:    "NoSuchKey",
			ExpectedStatus:  404,
			MinMessageLength: 15,
		}).AddScenario(ErrorScenario{
			Name: "Scenario 2",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/key2")
			},
			ExpectedCode:    "NoSuchKey",
			ExpectedStatus:  404,
			MinMessageLength: 15,
		})

		if len(suite.scenarios) != 2 {
			t.Errorf("Expected 2 scenarios, got %d", len(suite.scenarios))
		}

		suite.Run()
		suite.ReportStatistics()

		// Verify results were collected
		if len(suite.results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(suite.results))
		}
	})

	t.Run("Fluent API for adding scenarios", func(t *testing.T) {
		suite := NewErrorTestSuite(t, "Fluent Suite")

		suite.AddScenario(ErrorScenario{
			Name: "First",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/key")
			},
			ExpectedCode:    "NoSuchKey",
			ExpectedStatus:  404,
			MinMessageLength: 15,
		})

		if len(suite.scenarios) != 1 {
			t.Errorf("Expected 1 scenario, got %d", len(suite.scenarios))
		}

		// Verify chaining returns the same suite
		newSuite := suite.AddScenario(ErrorScenario{
			Name: "Second",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "POST", "/test-bucket/key")
			},
			ExpectedCode:    "InvalidRequest",
			ExpectedStatus:  400,
			MinMessageLength: 20,
		})

		if newSuite != suite {
			t.Error("Fluent API should return the same suite instance")
		}

		if len(suite.scenarios) != 2 {
			t.Errorf("Expected 2 scenarios after fluent add, got %d", len(suite.scenarios))
		}
	})
}

func TestMeasureRequestTime(t *testing.T) {
	fixture := NewTestServer(t)

	req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/key")

	duration, w := MeasureRequestTime(fixture.Handler, req)

	if duration < 0 {
		t.Errorf("Duration should be positive, got %v", duration)
	}

	if w == nil {
		t.Error("ResponseRecorder should not be nil")
	}

	if w.Code == 0 {
		t.Error("Expected HTTP status code, got 0")
	}
}

// TestErrorTestInfrastructure_Examples demonstrates real usage patterns
func TestErrorTestInfrastructure_Examples(t *testing.T) {
	t.Run("Example: Testing NoSuchKey scenario", func(t *testing.T) {
		fixture := NewTestServer(t)

		// Create request
		req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/nonexistent-file.txt")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		// Validate response
		VerifyErrorResponse(t, w).
			HTTPStatusCode(404).
			HasCode("NoSuchKey").
			MessageContainsAny("not found", "object", "key").
			Assert()
	})

	t.Run("Example: Testing with performance constraints", func(t *testing.T) {
		fixture := NewTestServer(t)

		req := CreateAuthenticatedTestRequest(t, "POST", "/test-bucket/test-key")

		duration, w := MeasureRequestTime(fixture.Handler, req)

		VerifyErrorResponseWithTiming(t, w, duration).
			HTTPStatusCode(400).
			HasCode("InvalidRequest").
			ResponseTime(100 * time.Millisecond).
			Assert()
	})

	t.Run("Example: Using test suite for multiple scenarios", func(t *testing.T) {
		suite := NewErrorTestSuite(t, "Multiple Error Scenarios")

		// Add multiple scenarios
		suite.AddScenario(ErrorScenario{
			Name: "Non-existent object",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/missing")
			},
			ExpectedCode:     "NoSuchKey",
			ExpectedStatus:   404,
			ExpectedKeywords: []string{"not found"},
			MinMessageLength: 15,
		}).AddScenario(ErrorScenario{
			Name: "Unsupported operation",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "POST", "/test-bucket/key")
			},
			ExpectedCode:     "InvalidRequest",
			ExpectedStatus:   400,
			ExpectedKeywords: []string{"post", "unsupported"},
			MinMessageLength: 20,
		}).AddScenario(ErrorScenario{
			Name: "Unsupported method",
			SetupRequest: func(t *testing.T) *http.Request {
				return CreateAuthenticatedTestRequest(t, "PATCH", "/test-bucket/key")
			},
			ExpectedCode:     "MethodNotAllowed",
			ExpectedStatus:   405,
			ExpectedKeywords: []string{"method", "patch"},
			MinMessageLength: 15,
		})

		suite.Run()
		suite.ReportStatistics()
	})
}


// BenchmarkErrorTestInfrastructure benchmarks the infrastructure helpers
func BenchmarkErrorTestInfrastructure(b *testing.B) {
	fixture := NewTestServer(&testing.T{})

	req := CreateTestRequest(&testing.T{}, "GET", "/test-bucket/key", nil, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)
		VerifyErrorResponse(&testing.T{}, w).
			HTTPStatusCode(404).
			HasCode("NoSuchKey").
			Assert()
	}
}

func BenchmarkErrorTestSuite(b *testing.B) {
	b.ReportAllocs()

	suite := NewErrorTestSuite(&testing.T{}, "Benchmark Suite")

	suite.AddScenario(ErrorScenario{
		Name: "Scenario 1",
		SetupRequest: func(t *testing.T) *http.Request {
			return CreateTestRequest(t, "GET", "/test-bucket/key1", nil, nil)
		},
		ExpectedCode:    "NoSuchKey",
		ExpectedStatus:  404,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suite.Run()
	}
}

// =============================================================================
// CORS VALIDATION HELPER TESTS
// =============================================================================

// TestCORSValidationHelpers tests the new CORS validation helper functions.
func TestCORSValidationHelpers(t *testing.T) {
	fixture := NewTestServer(t)

	t.Run("HasCORSHeaders validates all CORS headers present", func(t *testing.T) {
		req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/test-key")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		VerifyErrorResponse(t, w).
			HasCORSHeaders().
			Assert()
	})

	t.Run("CORSOrigin validates Access-Control-Allow-Origin header", func(t *testing.T) {
		req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/test-key")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		VerifyErrorResponse(t, w).
			CORSOrigin("*").
			Assert()
	})

	t.Run("CORSMethods validates Access-Control-Allow-Methods header", func(t *testing.T) {
		req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/test-key")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		VerifyErrorResponse(t, w).
			CORSMethods("GET, PUT, DELETE, HEAD, POST, OPTIONS").
			Assert()
	})

	t.Run("CORSHeaders validates Access-Control-Allow-Headers header", func(t *testing.T) {
		req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/test-key")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		VerifyErrorResponse(t, w).
			CORSHeaders("Authorization, Content-Type, Range, Content-Length").
			Assert()
	})

	t.Run("Combined CORS validation with other checks", func(t *testing.T) {
		req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/nonexistent")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		// Verify all CORS headers along with standard error response checks
		VerifyErrorResponse(t, w).
			HTTPStatusCode(404).
			ContentType("application/xml").
			HasCORSHeaders().
			CORSOrigin("*").
			CORSMethods("GET, PUT, DELETE, HEAD, POST, OPTIONS").
			CORSHeaders("Authorization, Content-Type, Range, Content-Length").
			HasCode("NoSuchKey").
			BodyNotEmpty().
			Assert()
	})
}

// TestCORSHeadersOnAllErrors verifies CORS headers are present on all error responses.
func TestCORSHeadersOnAllErrors(t *testing.T) {
	fixture := NewTestServer(t)

	errorScenarios := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedStatus int
	}{
		{
			name: "404 NoSuchKey has CORS headers",
			setupRequest: func() *http.Request {
				return CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/nonexistent")
			},
			expectedStatus: 404,
		},
		{
			name: "400 InvalidRequest has CORS headers",
			setupRequest: func() *http.Request {
				return CreateAuthenticatedTestRequest(t, "POST", "/test-bucket/test-key")
			},
			expectedStatus: 400,
		},
		{
			name: "405 MethodNotAllowed has CORS headers",
			setupRequest: func() *http.Request {
				return CreateAuthenticatedTestRequest(t, "PATCH", "/test-bucket/test-key")
			},
			expectedStatus: 405,
		},
	}

	for _, scenario := range errorScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			req := scenario.setupRequest()
			w := httptest.NewRecorder()
			fixture.Handler.ServeHTTP(w, req)

			// Verify CORS headers are present
			origin := w.Header().Get("Access-Control-Allow-Origin")
			if origin == "" {
				t.Error("Expected Access-Control-Allow-Origin header to be set")
			}

			methods := w.Header().Get("Access-Control-Allow-Methods")
			if methods == "" {
				t.Error("Expected Access-Control-Allow-Methods header to be set")
			}

			headers := w.Header().Get("Access-Control-Allow-Headers")
			if headers == "" {
				t.Error("Expected Access-Control-Allow-Headers header to be set")
			}

			// Verify using the helper
			VerifyErrorResponse(t, w).
				HTTPStatusCode(scenario.expectedStatus).
				HasCORSHeaders().
				Assert()
		})
	}
}

// TestStatusCodeValidation validates HTTP status code helper.
func TestStatusCodeValidation(t *testing.T) {
	fixture := NewTestServer(t)

	t.Run("Validates correct 404 status", func(t *testing.T) {
		req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/nonexistent")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		VerifyErrorResponse(t, w).
			HTTPStatusCode(404).
			Assert()
	})

	t.Run("Validates correct 400 status", func(t *testing.T) {
		req := CreateAuthenticatedTestRequest(t, "POST", "/test-bucket/test-key")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		VerifyErrorResponse(t, w).
			HTTPStatusCode(400).
			Assert()
	})
}

// TestContentTypeValidation validates Content-Type header helper.
func TestContentTypeValidation(t *testing.T) {
	fixture := NewTestServer(t)

	t.Run("Validates application/xml content type", func(t *testing.T) {
		req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/nonexistent")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		VerifyErrorResponse(t, w).
			ContentType("application/xml").
			Assert()
	})
}

// TestErrorStructureValidation validates error response structure helpers.
func TestErrorStructureValidation(t *testing.T) {
	fixture := NewTestServer(t)

	t.Run("Validates error response has code and message", func(t *testing.T) {
		req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/nonexistent")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		VerifyErrorResponse(t, w).
			HasCode("NoSuchKey").
			BodyNotEmpty().
			Assert()
	})

	t.Run("Validates error message content", func(t *testing.T) {
		req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/nonexistent")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		VerifyErrorResponse(t, w).
			HasCode("NoSuchKey").
			MessageContainsAny("not found", "object", "key").
			MessageMinLength(15).
			Assert()
	})
}

// TestAllValidationHelpersCombined tests all helpers working together.
func TestAllValidationHelpersCombined(t *testing.T) {
	fixture := NewTestServer(t)

	t.Run("Comprehensive error response validation", func(t *testing.T) {
		req := CreateAuthenticatedTestRequest(t, "GET", "/test-bucket/nonexistent-key")
		w := httptest.NewRecorder()
		fixture.Handler.ServeHTTP(w, req)

		// Use all helper functions together
		VerifyErrorResponse(t, w).
			HTTPStatusCode(404).
			ContentType("application/xml").
			HasCORSHeaders().
			CORSOrigin("*").
			CORSMethods("GET, PUT, DELETE, HEAD, POST, OPTIONS").
			CORSHeaders("Authorization, Content-Type, Range, Content-Length").
			HasCode("NoSuchKey").
			BodyNotEmpty().
			HasXMLDeclaration().
			MessageMinLength(15).
			Assert()
	})
}

