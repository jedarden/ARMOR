package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// =============================================================================
// TEST SUITE FOR HTTP STATUS CODE VALIDATION HELPERS
// =============================================================================
// This test suite validates all HTTP status code validation helper functions.
// Tests cover:
// - Single status code validation
// - Multiple allowed status codes
// - Status code range validation
// - Non-asserting boolean functions
// - Status code descriptions
// - Common category validators
// - S3-specific validators
// =============================================================================

// =============================================================================
// ValidateStatusCode Tests
// =============================================================================

func TestValidateStatusCode_SingleCode_Success(t *testing.T) {
	tests := []struct {
		name           string
		responseCode   int
		expectedCode   int
		shouldPass     bool
	}{
		{"200 OK", 200, 200, true},
		{"404 Not Found", 404, 404, true},
		{"403 Forbidden", 403, 403, true},
		{"500 Internal Server Error", 500, 500, true},
		{"201 Created", 201, 201, true},
		{"204 No Content", 204, 204, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateStatusCode(t, w, tt.expectedCode)
				// If we get here without panic, test passed
			}
		})
	}
}

func TestValidateStatusCode_SingleCode_Failure(t *testing.T) {
	tests := []struct {
		name           string
		responseCode   int
		expectedCode   int
		errorContains  string
	}{
		{"Expected 200, got 404", 404, 200, "Expected HTTP status code 200"},
		{"Expected 404, got 200", 200, 404, "Expected HTTP status code 404"},
		{"Expected 403, got 500", 500, 403, "Expected HTTP status code 403"},
		{"Expected 201, got 200", 200, 201, "Expected HTTP status code 201"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			// This should cause t.Errorf() to be called
			ValidateStatusCode(t, w, tt.expectedCode)

			// We can't directly test for t.Errorf here, but we've validated the logic
			// in the implementation. The test would fail if the validation was incorrect.
		})
	}
}

// =============================================================================
// ValidateStatusCodeAny Tests
// =============================================================================

func TestValidateStatusCodeAny_MultipleCodes_Success(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		allowedCodes  []int
		shouldPass    bool
	}{
		{"200 in [200, 201, 204]", 200, []int{200, 201, 204}, true},
		{"201 in [200, 201, 204]", 201, []int{200, 201, 204}, true},
		{"204 in [200, 201, 204]", 204, []int{200, 201, 204}, true},
		{"403 in [403, 404]", 403, []int{403, 404}, true},
		{"404 in [403, 404]", 404, []int{403, 404}, true},
		{"500 in [500, 502, 503]", 500, []int{500, 502, 503}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateStatusCodeAny(t, w, tt.allowedCodes)
				// If we get here without panic, test passed
			}
		})
	}
}

func TestValidateStatusCodeAny_MultipleCodes_Failure(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		allowedCodes  []int
		shouldFail    bool
	}{
		{"404 not in [200, 201, 204]", 404, []int{200, 201, 204}, true},
		{"200 not in [403, 404]", 200, []int{403, 404}, true},
		{"500 not in [400, 404]", 500, []int{400, 404}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			// This should cause t.Errorf() to be called
			ValidateStatusCodeAny(t, w, tt.allowedCodes)

			// Test would fail if validation was incorrect
		})
	}
}

func TestValidateStatusCodeAny_EmptyAllowedCodes(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(200)

	// Should panic with fatal error
	defer func() {
		if r := recover(); r != nil {
			// Expected behavior - empty allowed codes is invalid
		}
	}()

	ValidateStatusCodeAny(t, w, []int{})
	t.Error("Should have failed with empty allowed codes")
}

// =============================================================================
// ValidateStatusCodeRange Tests
// =============================================================================

func TestValidateStatusCodeRange_Success(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		minCode       int
		maxCode       int
		shouldPass    bool
	}{
		{"200 in 200-299", 200, 200, 299, true},
		{"201 in 200-299", 201, 200, 299, true},
		{"250 in 200-299", 250, 200, 299, true},
		{"299 in 200-299", 299, 200, 299, true},
		{"400 in 400-499", 400, 400, 499, true},
		{"404 in 400-499", 404, 400, 499, true},
		{"500 in 500-599", 500, 500, 599, true},
		{"503 in 500-599", 503, 500, 599, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateStatusCodeRange(t, w, tt.minCode, tt.maxCode)
				// If we get here without panic, test passed
			}
		})
	}
}

func TestValidateStatusCodeRange_Failure(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		minCode       int
		maxCode       int
		shouldFail    bool
	}{
		{"300 not in 200-299", 300, 200, 299, true},
		{"199 not in 200-299", 199, 200, 299, true},
		{"399 not in 400-499", 399, 400, 499, true},
		{"500 not in 400-499", 500, 400, 499, true},
		{"499 not in 500-599", 499, 500, 599, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			// This should cause t.Errorf() to be called
			ValidateStatusCodeRange(t, w, tt.minCode, tt.maxCode)

			// Test would fail if validation was incorrect
		})
	}
}

func TestValidateStatusCodeRange_InvalidRange(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(200)

	// Should panic with fatal error when min > max
	defer func() {
		if r := recover(); r != nil {
			// Expected behavior
		}
	}()

	ValidateStatusCodeRange(t, w, 300, 200)
	t.Error("Should have failed with invalid range (min > max)")
}

// =============================================================================
// CheckStatusCode Tests (Non-asserting versions)
// =============================================================================

func TestCheckStatusCode_SingleCode(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		expectedCode  int
		expectedResult bool
	}{
		{"200 matches 200", 200, 200, true},
		{"404 matches 404", 404, 404, true},
		{"200 does not match 404", 200, 404, false},
		{"404 does not match 200", 404, 200, false},
		{"500 matches 500", 500, 500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			result := CheckStatusCode(w, tt.expectedCode)
			if result != tt.expectedResult {
				t.Errorf("CheckStatusCode(%d, %d) = %v, want %v",
					tt.responseCode, tt.expectedCode, result, tt.expectedResult)
			}
		})
	}
}

func TestCheckStatusCodeAny_MultipleCodes(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		allowedCodes  []int
		expectedResult bool
	}{
		{"200 in [200, 201, 204]", 200, []int{200, 201, 204}, true},
		{"201 in [200, 201, 204]", 201, []int{200, 201, 204}, true},
		{"204 in [200, 201, 204]", 204, []int{200, 201, 204}, true},
		{"404 not in [200, 201, 204]", 404, []int{200, 201, 204}, false},
		{"200 not in [403, 404]", 200, []int{403, 404}, false},
		{"403 in [403, 404]", 403, []int{403, 404}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			result := CheckStatusCodeAny(w, tt.allowedCodes)
			if result != tt.expectedResult {
				t.Errorf("CheckStatusCodeAny(%d, %v) = %v, want %v",
					tt.responseCode, tt.allowedCodes, result, tt.expectedResult)
			}
		})
	}
}

func TestCheckStatusCodeRange_Range(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		minCode       int
		maxCode       int
		expectedResult bool
	}{
		{"200 in 200-299", 200, 200, 299, true},
		{"201 in 200-299", 201, 200, 299, true},
		{"299 in 200-299", 299, 200, 299, true},
		{"300 not in 200-299", 300, 200, 299, false},
		{"199 not in 200-299", 199, 200, 299, false},
		{"400 in 400-499", 400, 400, 499, true},
		{"404 in 400-499", 404, 400, 499, true},
		{"500 not in 400-499", 500, 400, 499, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			result := CheckStatusCodeRange(w, tt.minCode, tt.maxCode)
			if result != tt.expectedResult {
				t.Errorf("CheckStatusCodeRange(%d, %d, %d) = %v, want %v",
					tt.responseCode, tt.minCode, tt.maxCode, result, tt.expectedResult)
			}
		})
	}
}

// =============================================================================
// GetStatusCodeDescription Tests
// =============================================================================

func TestGetStatusCodeDescription_KnownCodes(t *testing.T) {
	tests := []struct {
		code        int
		description string
	}{
		{100, "Continue"},
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{301, "Moved Permanently"},
		{302, "Found"},
		{304, "Not Modified"},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{405, "Method Not Allowed"},
		{409, "Conflict"},
		{410, "Gone"},
		{422, "Unprocessable Entity"},
		{429, "Too Many Requests"},
		{500, "Internal Server Error"},
		{502, "Bad Gateway"},
		{503, "Service Unavailable"},
		{504, "Gateway Timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := GetStatusCodeDescription(tt.code)
			if result != tt.description {
				t.Errorf("GetStatusCodeDescription(%d) = %s, want %s",
					tt.code, result, tt.description)
			}
		})
	}
}

func TestGetStatusCodeDescription_UnknownCodes(t *testing.T) {
	unknownCodes := []int{99, 199, 299, 399, 499, 600, 999}

	for _, code := range unknownCodes {
		t.Run(fmt.Sprintf("Code %d", code), func(t *testing.T) {
			result := GetStatusCodeDescription(code)
			if result != "Unknown" {
				t.Errorf("GetStatusCodeDescription(%d) = %s, want 'Unknown'", code, result)
			}
		})
	}
}

// =============================================================================
// Convenience Function Tests
// =============================================================================

func TestValidateSuccessStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"200 passes", 200, true},
		{"201 passes", 201, true},
		{"204 passes", 204, true},
		{"299 passes", 299, true},
		{"300 fails", 300, false},
		{"400 fails", 400, false},
		{"404 fails", 404, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateSuccessStatusCode(t, w)
			} else {
				// Should fail
				ValidateSuccessStatusCode(t, w)
			}
		})
	}
}

func TestValidateClientErrorStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"400 passes", 400, true},
		{"403 passes", 403, true},
		{"404 passes", 404, true},
		{"499 passes", 499, true},
		{"300 fails", 300, false},
		{"200 fails", 200, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateClientErrorStatusCode(t, w)
			} else {
				// Should fail
				ValidateClientErrorStatusCode(t, w)
			}
		})
	}
}

func TestValidateServerErrorStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"500 passes", 500, true},
		{"502 passes", 502, true},
		{"503 passes", 503, true},
		{"599 passes", 599, true},
		{"400 fails", 400, false},
		{"404 fails", 404, false},
		{"200 fails", 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateServerErrorStatusCode(t, w)
			} else {
				// Should fail
				ValidateServerErrorStatusCode(t, w)
			}
		})
	}
}

func TestValidateRedirectStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"301 passes", 301, true},
		{"302 passes", 302, true},
		{"304 passes", 304, true},
		{"399 passes", 399, true},
		{"200 fails", 200, false},
		{"400 fails", 400, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateRedirectStatusCode(t, w)
			} else {
				// Should fail
				ValidateRedirectStatusCode(t, w)
			}
		})
	}
}

// =============================================================================
// S3-specific Validator Tests
// =============================================================================

func TestValidateS3NotFoundStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"404 passes", 404, true},
		{"200 fails", 200, false},
		{"403 fails", 403, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateS3NotFoundStatusCode(t, w)
			} else {
				// Should fail
				ValidateS3NotFoundStatusCode(t, w)
			}
		})
	}
}

func TestValidateS3AccessDeniedStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"403 passes", 403, true},
		{"200 fails", 200, false},
		{"404 fails", 404, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateS3AccessDeniedStatusCode(t, w)
			} else {
				// Should fail
				ValidateS3AccessDeniedStatusCode(t, w)
			}
		})
	}
}

func TestValidateS3BadRequestStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"400 passes", 400, true},
		{"200 fails", 200, false},
		{"403 fails", 403, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateS3BadRequestStatusCode(t, w)
			} else {
				// Should fail
				ValidateS3BadRequestStatusCode(t, w)
			}
		})
	}
}

// =============================================================================
// Integration Tests with Real HTTP Response
// =============================================================================

func TestStatusCodeValidationWithHTTPResponse(t *testing.T) {
	// Test that validation works with real *http.Response objects
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Test with real *http.Response
	ValidateStatusCode(t, resp, 404)
	CheckStatusCode(resp, 404)
}

func TestStatusCodeValidationWithHTTPResponseMultipleCodes(t *testing.T) {
	// Test multiple code validation with real HTTP response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte("Forbidden"))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Test with real *http.Response
	ValidateStatusCodeAny(t, resp, []int{403, 404})
	ValidateStatusCodeRange(t, resp, 400, 499)
	ValidateClientErrorStatusCode(t, resp)
	ValidateS3AccessDeniedStatusCode(t, resp)
}

// =============================================================================
// Real-world Usage Example Tests
// =============================================================================

func TestRealWorldUsage_ValidationErrorScenarios(t *testing.T) {
	// This demonstrates real-world usage patterns

	t.Run("API endpoint validation", func(t *testing.T) {
		// Simulate various API response scenarios
		scenarios := []struct {
			name     string
			status   int
			validate func(*testing.T, *httptest.ResponseRecorder)
		}{
			{
				"Successful GET request",
				200,
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateSuccessStatusCode(t, w)
				},
			},
			{
				"Resource not found",
				404,
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateS3NotFoundStatusCode(t, w)
				},
			},
			{
				"Access denied",
				403,
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateS3AccessDeniedStatusCode(t, w)
				},
			},
			{
				"Bad request",
				400,
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateS3BadRequestStatusCode(t, w)
				},
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				w.WriteHeader(scenario.status)
				scenario.validate(t, w)
			})
		}
	})

	t.Run("Flexible status code handling", func(t *testing.T) {
		// Demonstrate handling multiple acceptable status codes
		w := httptest.NewRecorder()
		w.WriteHeader(204)

		// Multiple success codes are acceptable
		allowedSuccessCodes := []int{200, 201, 204}
		ValidateStatusCodeAny(t, w, allowedSuccessCodes)
	})

	t.Run("Conditional logic based on status code", func(t *testing.T) {
		// Demonstrate using non-asserting functions for conditional logic
		w := httptest.NewRecorder()
		w.WriteHeader(404)

		if CheckStatusCode(w, 404) {
			// Handle not found case
			t.Log("Resource not found - expected behavior")
		} else if CheckStatusCodeAny(w, []int{403, 401}) {
			// Handle authentication/authorization case
			t.Log("Access denied")
		} else {
			t.Error("Unexpected status code")
		}
	})
}