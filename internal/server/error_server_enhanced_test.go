// Unit tests for enhanced error server functionality
package server

import (
	"io"
	"net/http"
	"testing"
	"time"
)

// =============================================================================
// TESTS FOR CONVENIENCE FUNCTIONS
// =============================================================================

func TestNewRateLimitScenario(t *testing.T) {
	t.Run("creates rate limit scenario with correct defaults", func(t *testing.T) {
		frequency := 0.5
		retryAfter := 10 * time.Second

		scenario := NewRateLimitScenario(frequency, retryAfter)

		if scenario.Type != ErrorTypeRateLimit {
			t.Errorf("Expected type ErrorTypeRateLimit, got %s", scenario.Type)
		}

		if scenario.Severity != ErrorSeverityMedium {
			t.Errorf("Expected severity ErrorSeverityMedium, got %s", scenario.Severity)
		}

		if scenario.Frequency != frequency {
			t.Errorf("Expected frequency %f, got %f", frequency, scenario.Frequency)
		}

		if scenario.RetryAfter != retryAfter {
			t.Errorf("Expected retry after %v, got %v", retryAfter, scenario.RetryAfter)
		}

		if scenario.StatusCode != 429 {
			t.Errorf("Expected status code 429, got %d", scenario.StatusCode)
		}

		if scenario.ErrorCode != "SlowDown" {
			t.Errorf("Expected error code 'SlowDown', got '%s'", scenario.ErrorCode)
		}

		// Check for rate limit headers
		if scenario.Headers == nil {
			t.Fatal("Headers should not be nil")
		}

		if scenario.Headers["Retry-After"] == "" {
			t.Error("Expected Retry-After header to be set")
		}

		if scenario.Headers["X-RateLimit-Limit"] == "" {
			t.Error("Expected X-RateLimit-Limit header to be set")
		}
	})
}

func TestNewTimeoutScenario(t *testing.T) {
	t.Run("creates timeout scenario with correct defaults", func(t *testing.T) {
		frequency := 0.3
		delay := 5 * time.Second

		scenario := NewTimeoutScenario(frequency, delay)

		if scenario.Type != ErrorTypeTimeout {
			t.Errorf("Expected type ErrorTypeTimeout, got %s", scenario.Type)
		}

		if scenario.Severity != ErrorSeverityHigh {
			t.Errorf("Expected severity ErrorSeverityHigh, got %s", scenario.Severity)
		}

		if scenario.Frequency != frequency {
			t.Errorf("Expected frequency %f, got %f", frequency, scenario.Frequency)
		}

		if scenario.Delay != delay {
			t.Errorf("Expected delay %v, got %v", delay, scenario.Delay)
		}

		if scenario.StatusCode != 408 {
			t.Errorf("Expected status code 408, got %d", scenario.StatusCode)
		}

		if scenario.ErrorCode != "RequestTimeout" {
			t.Errorf("Expected error code 'RequestTimeout', got '%s'", scenario.ErrorCode)
		}
	})
}

func TestNewConnectionRefusedScenario(t *testing.T) {
	t.Run("creates connection refused scenario with correct defaults", func(t *testing.T) {
		frequency := 0.1

		scenario := NewConnectionRefusedScenario(frequency)

		if scenario.Type != ErrorTypeConnectionRefused {
			t.Errorf("Expected type ErrorTypeConnectionRefused, got %s", scenario.Type)
		}

		if scenario.Severity != ErrorSeverityCritical {
			t.Errorf("Expected severity ErrorSeverityCritical, got %s", scenario.Severity)
		}

		if scenario.Frequency != frequency {
			t.Errorf("Expected frequency %f, got %f", frequency, scenario.Frequency)
		}

		if scenario.StatusCode != 503 {
			t.Errorf("Expected status code 503, got %d", scenario.StatusCode)
		}

		if scenario.ErrorCode != "ServiceUnavailable" {
			t.Errorf("Expected error code 'ServiceUnavailable', got '%s'", scenario.ErrorCode)
		}

		if scenario.Message == "" {
			t.Error("Expected non-empty message")
		}
	})
}

func TestNewSlowResponseScenario(t *testing.T) {
	t.Run("creates slow response scenario with correct defaults", func(t *testing.T) {
		frequency := 0.2
		delay := 2 * time.Second

		scenario := NewSlowResponseScenario(frequency, delay)

		if scenario.Type != ErrorTypeSlowResponse {
			t.Errorf("Expected type ErrorTypeSlowResponse, got %s", scenario.Type)
		}

		if scenario.Severity != ErrorSeverityLow {
			t.Errorf("Expected severity ErrorSeverityLow, got %s", scenario.Severity)
		}

		if scenario.Frequency != frequency {
			t.Errorf("Expected frequency %f, got %f", frequency, scenario.Frequency)
		}

		if scenario.Delay != delay {
			t.Errorf("Expected delay %v, got %v", delay, scenario.Delay)
		}

		if scenario.StatusCode != 200 {
			t.Errorf("Expected status code 200, got %d", scenario.StatusCode)
		}
	})
}

func TestNewServiceUnavailableScenario(t *testing.T) {
	t.Run("creates service unavailable scenario with correct defaults", func(t *testing.T) {
		frequency := 0.25
		retryAfter := 15 * time.Second

		scenario := NewServiceUnavailableScenario(frequency, retryAfter)

		if scenario.Type != ErrorTypeServiceUnavailable {
			t.Errorf("Expected type ErrorTypeServiceUnavailable, got %s", scenario.Type)
		}

		if scenario.Severity != ErrorSeverityHigh {
			t.Errorf("Expected severity ErrorSeverityHigh, got %s", scenario.Severity)
		}

		if scenario.Frequency != frequency {
			t.Errorf("Expected frequency %f, got %f", frequency, scenario.Frequency)
		}

		if scenario.RetryAfter != retryAfter {
			t.Errorf("Expected retry after %v, got %v", retryAfter, scenario.RetryAfter)
		}

		if scenario.StatusCode != 503 {
			t.Errorf("Expected status code 503, got %d", scenario.StatusCode)
		}

		if scenario.Headers["Retry-After"] == "" {
			t.Error("Expected Retry-After header to be set")
		}
	})
}

// =============================================================================
// TESTS FOR SEVERITY-BASED SCENARIOS
// =============================================================================

func TestNewLowSeverityScenario(t *testing.T) {
	t.Run("creates low severity scenario", func(t *testing.T) {
		scenario := NewLowSeverityScenario(ErrorTypeInternalError, 0.1)

		if scenario.Severity != ErrorSeverityLow {
			t.Errorf("Expected severity ErrorSeverityLow, got %s", scenario.Severity)
		}

		if scenario.StatusCode != 500 {
			t.Errorf("Expected status code 500, got %d", scenario.StatusCode)
		}

		if scenario.Message == "" {
			t.Error("Expected non-empty message")
		}
	})
}

func TestNewMediumSeverityScenario(t *testing.T) {
	t.Run("creates medium severity scenario", func(t *testing.T) {
		scenario := NewMediumSeverityScenario(ErrorTypeRateLimit, 0.3)

		if scenario.Severity != ErrorSeverityMedium {
			t.Errorf("Expected severity ErrorSeverityMedium, got %s", scenario.Severity)
		}

		if scenario.StatusCode != 500 {
			t.Errorf("Expected status code 500, got %d", scenario.StatusCode)
		}
	})
}

func TestNewHighSeverityScenario(t *testing.T) {
	t.Run("creates high severity scenario", func(t *testing.T) {
		scenario := NewHighSeverityScenario(ErrorTypeServiceUnavailable, 0.5)

		if scenario.Severity != ErrorSeverityHigh {
			t.Errorf("Expected severity ErrorSeverityHigh, got %s", scenario.Severity)
		}

		if scenario.StatusCode != 500 {
			t.Errorf("Expected status code 500, got %d", scenario.StatusCode)
		}

		if scenario.Message == "" {
			t.Error("Expected non-empty message")
		}
	})
}

func TestNewCriticalSeverityScenario(t *testing.T) {
	t.Run("creates critical severity scenario", func(t *testing.T) {
		scenario := NewCriticalSeverityScenario(ErrorTypeConnectionRefused, 0.05)

		if scenario.Severity != ErrorSeverityCritical {
			t.Errorf("Expected severity ErrorSeverityCritical, got %s", scenario.Severity)
		}

		if scenario.StatusCode != 503 {
			t.Errorf("Expected status code 503, got %d", scenario.StatusCode)
		}

		if scenario.Headers["Retry-After"] != "60" {
			t.Errorf("Expected Retry-After header '60', got '%s'", scenario.Headers["Retry-After"])
		}
	})
}

// =============================================================================
// TESTS FOR DEFAULT VALUES
// =============================================================================

func TestEnhancedErrorScenario_GetDefaultStatusCode(t *testing.T) {
	tests := []struct {
		name           string
		errorType      ErrorType
		expectedStatus int
	}{
		{"rate limit", ErrorTypeRateLimit, 429},
		{"service unavailable", ErrorTypeServiceUnavailable, 503},
		{"internal error", ErrorTypeInternalError, 500},
		{"bad request", ErrorTypeBadRequest, 400},
		{"unauthorized", ErrorTypeUnauthorized, 401},
		{"forbidden", ErrorTypeForbidden, 403},
		{"not found", ErrorTypeNotFound, 404},
		{"timeout", ErrorTypeTimeout, 408},
		{"connection refused", ErrorTypeConnectionRefused, 503},
		{"slow response", ErrorTypeSlowResponse, 200},
		{"unknown type", ErrorType("unknown"), 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenario := EnhancedErrorScenario{Type: tt.errorType}
			if got := scenario.getDefaultStatusCode(); got != tt.expectedStatus {
				t.Errorf("getDefaultStatusCode() = %d, want %d", got, tt.expectedStatus)
			}
		})
	}
}

func TestEnhancedErrorScenario_GetDefaultErrorCode(t *testing.T) {
	tests := []struct {
		name         string
		errorType    ErrorType
		expectedCode string
		overrideCode string
	}{
		{"rate limit", ErrorTypeRateLimit, "SlowDown", ""},
		{"service unavailable", ErrorTypeServiceUnavailable, "ServiceUnavailable", ""},
		{"internal error", ErrorTypeInternalError, ErrorCodeInternalError, ""},
		{"bad request", ErrorTypeBadRequest, ErrorCodeInvalidRequest, ""},
		{"unauthorized", ErrorTypeUnauthorized, "Unauthorized", ""},
		{"forbidden", ErrorTypeForbidden, ErrorCodeAccessDenied, ""},
		{"not found", ErrorTypeNotFound, ErrorCodeNoSuchKey, ""},
		{"timeout", ErrorTypeTimeout, "RequestTimeout", ""},
		{"connection refused", ErrorTypeConnectionRefused, "ServiceUnavailable", ""},
		{"slow response", ErrorTypeSlowResponse, "", ""},
		{"with override", ErrorTypeRateLimit, "CustomCode", "CustomCode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenario := EnhancedErrorScenario{
				Type:              tt.errorType,
				ErrorCodeOverride: tt.overrideCode,
			}
			if got := scenario.getDefaultErrorCode(); got != tt.expectedCode {
				t.Errorf("getDefaultErrorCode() = %s, want %s", got, tt.expectedCode)
			}
		})
	}
}

func TestEnhancedErrorScenario_GetDefaultMessage(t *testing.T) {
	tests := []struct {
		name           string
		errorType      ErrorType
		expectedSubstr string
		overrideMsg    string
	}{
		{"rate limit", ErrorTypeRateLimit, "Rate limit exceeded", ""},
		{"service unavailable", ErrorTypeServiceUnavailable, "temporarily unavailable", ""},
		{"internal error", ErrorTypeInternalError, "internal error", ""},
		{"bad request", ErrorTypeBadRequest, "invalid or malformed", ""},
		{"unauthorized", ErrorTypeUnauthorized, "Authentication is required", ""},
		{"forbidden", ErrorTypeForbidden, "Access to this resource is denied", ""},
		{"not found", ErrorTypeNotFound, "does not exist", ""},
		{"timeout", ErrorTypeTimeout, "timed out", ""},
		{"connection refused", ErrorTypeConnectionRefused, "Connection", ""},
		{"slow response", ErrorTypeSlowResponse, "successfully", ""},
		{"with override", ErrorTypeRateLimit, "Custom message", "Custom message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenario := EnhancedErrorScenario{
				Type:            tt.errorType,
				MessageOverride: tt.overrideMsg,
			}
			got := scenario.getDefaultMessage()
			if !contains(got, tt.expectedSubstr) {
				t.Errorf("getDefaultMessage() = %s, want substring %s", got, tt.expectedSubstr)
			}
		})
	}
}

// =============================================================================
// TESTS FOR ENHANCED ERROR SERVER
// =============================================================================

func TestNewEnhancedErrorServer(t *testing.T) {
	t.Run("creates enhanced server successfully", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewRateLimitScenario(0.5, 5*time.Second),
		}

		server := NewEnhancedErrorServer(scenarios)
		if server == nil {
			t.Fatal("Expected non-nil server")
		}
		defer server.Close()

		if server.ConfigurableErrorServer == nil {
			t.Error("Expected base server to be initialized")
		}

		if len(server.EnhancedScenarios) != 1 {
			t.Errorf("Expected 1 scenario, got %d", len(server.EnhancedScenarios))
		}

		if server.RandomSource == nil {
			t.Error("Expected random source to be initialized")
		}
	})

	t.Run("creates server with empty scenarios", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{}

		server := NewEnhancedErrorServer(scenarios)
		if server == nil {
			t.Fatal("Expected non-nil server")
		}
		defer server.Close()

		if len(server.EnhancedScenarios) != 0 {
			t.Errorf("Expected 0 scenarios, got %d", len(server.EnhancedScenarios))
		}
	})
}

func TestNewEnhancedErrorServerWithSeed(t *testing.T) {
	t.Run("creates server with deterministic randomness", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewTimeoutScenario(1.0, 100*time.Millisecond),
		}

		seed := int64(12345)
		server := NewEnhancedErrorServerWithSeed(scenarios, seed)
		defer server.Close()

		if server.RandomSource == nil {
			t.Fatal("Expected random source to be initialized")
		}

		// Test that randomness is deterministic
		firstValue := server.RandomSource.Float64()
		secondValue := server.RandomSource.Float64()

		if firstValue == secondValue {
			t.Error("Random source should produce different values")
		}

		// Create another server with same seed and verify first value matches
		server2 := NewEnhancedErrorServerWithSeed(scenarios, seed)
		defer server2.Close()

		firstValue2 := server2.RandomSource.Float64()
		if firstValue != firstValue2 {
			t.Errorf("Expected same first value with same seed: %f != %f", firstValue, firstValue2)
		}
	})
}

func TestEnhancedErrorServer_ShouldInjectError(t *testing.T) {
	t.Run("respects frequency setting", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewRateLimitScenario(1.0, 5*time.Second), // 100% frequency
		}

		server := NewEnhancedErrorServerWithSeed(scenarios, 999)
		defer server.Close()

		req, _ := http.NewRequest("GET", "/test", nil)

		// With 100% frequency, should always inject
		for i := 0; i < 10; i++ {
			if !server.ShouldInjectError(scenarios[0], req) {
				t.Error("Expected error to be injected with 100% frequency")
			}
		}

		total, errors, rate := server.GetStats()
		if errors != 10 {
			t.Errorf("Expected 10 errors, got %d", errors)
		}
		if rate != 1.0 {
			t.Errorf("Expected 100%% error rate, got %f", rate)
		}
		if total != 10 {
			t.Errorf("Expected 10 total requests, got %d", total)
		}
	})

	t.Run("respects zero frequency", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewRateLimitScenario(0.0, 5*time.Second), // 0% frequency
		}

		server := NewEnhancedErrorServerWithSeed(scenarios, 999)
		defer server.Close()

		req, _ := http.NewRequest("GET", "/test", nil)

		// With 0% frequency, should never inject
		for i := 0; i < 10; i++ {
			if server.ShouldInjectError(scenarios[0], req) {
				t.Error("Expected no error injection with 0% frequency")
			}
		}

		_, errors, rate := server.GetStats()
		if errors != 0 {
			t.Errorf("Expected 0 errors, got %d", errors)
		}
		if rate != 0.0 {
			t.Errorf("Expected 0%% error rate, got %f", rate)
		}
	})

	t.Run("respects condition function", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewRateLimitScenario(1.0, 5*time.Second),
		}
		scenarios[0].Condition = func(r *http.Request) bool {
			return r.URL.Path == "/trigger"
		}

		server := NewEnhancedErrorServerWithSeed(scenarios, 999)
		defer server.Close()

		req1, _ := http.NewRequest("GET", "/trigger", nil)
		req2, _ := http.NewRequest("GET", "/other", nil)

		if !server.ShouldInjectError(scenarios[0], req1) {
			t.Error("Expected error injection for matching path")
		}

		if server.ShouldInjectError(scenarios[0], req2) {
			t.Error("Expected no error injection for non-matching path")
		}
	})

	t.Run("respects intercept flag", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewRateLimitScenario(0.0, 5*time.Second), // 0% frequency
		}
		scenarios[0].Intercept = true

		server := NewEnhancedErrorServer(scenarios)
		defer server.Close()

		req, _ := http.NewRequest("GET", "/test", nil)

		// With intercept=true, should inject regardless of frequency
		if !server.ShouldInjectError(scenarios[0], req) {
			t.Error("Expected error injection with intercept flag")
		}
	})

	t.Run("combines frequency and probability", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewRateLimitScenario(0.5, 5*time.Second),
		}
		scenarios[0].Probability = 0.5 // Both frequency and probability

		server := NewEnhancedErrorServer(scenarios)
		defer server.Close()

		req, _ := http.NewRequest("GET", "/test", nil)

		// With both set at 0.5, actual injection should be around 0.25 (0.5 * 0.5)
		// We'll test 1000 times and check that it's reasonable
		injections := 0
		for i := 0; i < 1000; i++ {
			if server.ShouldInjectError(scenarios[0], req) {
				injections++
			}
		}

		rate := float64(injections) / 1000.0
		// Should be around 0.25, give it some tolerance (0.20 to 0.30)
		if rate < 0.20 || rate > 0.30 {
			t.Errorf("Expected injection rate around 0.25, got %f", rate)
		}
	})
}

func TestEnhancedErrorServer_GetStats(t *testing.T) {
	t.Run("tracks statistics correctly", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewRateLimitScenario(0.5, 5*time.Second),
		}

		server := NewEnhancedErrorServer(scenarios)
		defer server.Close()

		req, _ := http.NewRequest("GET", "/test", nil)

		// Make 10 requests
		for i := 0; i < 10; i++ {
			server.ShouldInjectError(scenarios[0], req)
		}

		total, errors, rate := server.GetStats()

		if total != 10 {
			t.Errorf("Expected 10 total requests, got %d", total)
		}

		// With 50% frequency, errors should be around 5
		if errors < 3 || errors > 7 {
			t.Logf("Warning: Expected around 5 errors, got %d (acceptable variance)", errors)
		}

		// Rate should be around 0.5
		if rate < 0.3 || rate > 0.7 {
			t.Logf("Warning: Expected rate around 0.5, got %f (acceptable variance)", rate)
		}
	})

	t.Run("returns zero stats initially", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewRateLimitScenario(0.5, 5*time.Second),
		}

		server := NewEnhancedErrorServer(scenarios)
		defer server.Close()

		total, errors, rate := server.GetStats()

		if total != 0 {
			t.Errorf("Expected 0 total requests, got %d", total)
		}

		if errors != 0 {
			t.Errorf("Expected 0 errors, got %d", errors)
		}

		if rate != 0.0 {
			t.Errorf("Expected 0.0 rate, got %f", rate)
		}
	})
}

func TestEnhancedErrorServer_ResetCounters(t *testing.T) {
	t.Run("resets counters correctly", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewRateLimitScenario(1.0, 5*time.Second),
		}

		server := NewEnhancedErrorServer(scenarios)
		defer server.Close()

		req, _ := http.NewRequest("GET", "/test", nil)

		// Make some requests
		for i := 0; i < 5; i++ {
			server.ShouldInjectError(scenarios[0], req)
		}

		// Verify stats
		total, errors, _ := server.GetStats()
		if total != 5 || errors != 5 {
			t.Errorf("Expected 5/5 before reset, got %d/%d", total, errors)
		}

		// Reset
		server.ResetCounters()

		// Verify reset
		total, errors, rate := server.GetStats()
		if total != 0 || errors != 0 || rate != 0.0 {
			t.Errorf("Expected 0/0/0.0 after reset, got %d/%d/%f", total, errors, rate)
		}

		// Make more requests and verify counters work again
		for i := 0; i < 3; i++ {
			server.ShouldInjectError(scenarios[0], req)
		}

		total, errors, _ = server.GetStats()
		if total != 3 || errors != 3 {
			t.Errorf("Expected 3/3 after reset, got %d/%d", total, errors)
		}
	})
}

// =============================================================================
// TESTS FOR SERVER INTEGRATION
// =============================================================================

func TestEnhancedErrorServer_Integration(t *testing.T) {
	t.Run("server handles requests with error injection", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewRateLimitScenario(1.0, 5*time.Second), // Always inject
		}

		server := NewEnhancedErrorServer(scenarios)
		defer server.Close()

		// Make a request
		resp, err := http.Get(server.URL + "/test")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should get rate limit response
		if resp.StatusCode != 429 {
			t.Errorf("Expected status 429, got %d", resp.StatusCode)
		}

		// Check for rate limit headers
		if resp.Header.Get("Retry-After") == "" {
			t.Error("Expected Retry-After header")
		}
	})

	t.Run("server respects probability over multiple requests", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewRateLimitScenario(0.0, 5*time.Second), // Never inject via frequency
		}
		scenarios[0].Probability = 1.0 // Always inject via probability

		server := NewEnhancedErrorServer(scenarios)
		defer server.Close()

		// Make multiple requests
		rateLimitCount := 0
		for i := 0; i < 10; i++ {
			resp, err := http.Get(server.URL + "/test")
			if err != nil {
				t.Fatalf("Request %d failed: %v", i, err)
			}
			if resp.StatusCode == 429 {
				rateLimitCount++
			}
			resp.Body.Close()
		}

		// Should get rate limit every time due to probability=1.0
		if rateLimitCount != 10 {
			t.Errorf("Expected 10 rate limit responses, got %d", rateLimitCount)
		}
	})
}

func TestEnhancedErrorServer_SlowResponse(t *testing.T) {
	t.Run("slow response scenario works", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewSlowResponseScenario(1.0, 100*time.Millisecond), // 100ms delay
		}

		server := NewEnhancedErrorServer(scenarios)
		defer server.Close()

		start := time.Now()
		resp, err := http.Get(server.URL + "/test")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		duration := time.Since(start)

		// Should take at least 100ms
		if duration < 100*time.Millisecond {
			t.Errorf("Expected delay of at least 100ms, got %v", duration)
		}

		// Should be successful response
		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}

func TestEnhancedErrorServer_TimeoutScenario(t *testing.T) {
	t.Run("timeout scenario with delay works", func(t *testing.T) {
		scenarios := []EnhancedErrorScenario{
			NewTimeoutScenario(1.0, 50*time.Millisecond), // 50ms delay
		}

		server := NewEnhancedErrorServer(scenarios)
		defer server.Close()

		start := time.Now()
		resp, err := http.Get(server.URL + "/test")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		duration := time.Since(start)

		// Should take at least 50ms
		if duration < 50*time.Millisecond {
			t.Errorf("Expected delay of at least 50ms, got %v", duration)
		}

		// Should return timeout status
		if resp.StatusCode != 408 {
			t.Errorf("Expected status 408, got %d", resp.StatusCode)
		}

		// Read body to verify error message
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		// Check for "timed out" (the actual message text)
		if !contains(string(body), "timed out") {
			t.Errorf("Expected timeout message in body, got: %s", string(body))
		}
	})
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
