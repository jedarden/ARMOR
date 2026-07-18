// Package testutil provides comprehensive tests for validation helpers.
//
// These tests demonstrate the usage of request/response validation helpers
// and ensure they work correctly with the ARMOR test infrastructure.
package testutil

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// TEST REQUEST BUILDER TESTS
// =============================================================================

// TestNewTestRequest verifies basic request creation.
func TestNewTestRequest(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{
			name:   "GET request",
			method: "GET",
			path:   "/test-bucket/test-key",
		},
		{
			name:   "POST request",
			method: "POST",
			path:   "/test-bucket/test-key",
		},
		{
			name:   "PUT request",
			method: "PUT",
			path:   "/test-bucket/test-key",
		},
		{
			name:   "DELETE request",
			method: "DELETE",
			path:   "/test-bucket/test-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewTestRequest(tt.method, tt.path, tt.body)

			if req.Method != tt.method {
				t.Errorf("Expected method %s, got %s", tt.method, req.Method)
			}

			if req.URL.Path != tt.path {
				t.Errorf("Expected path %s, got %s", tt.path, req.URL.Path)
			}
		})
	}
}

// TestBuildTestRequest verifies comprehensive request building.
func TestBuildTestRequest(t *testing.T) {
	config := TestRequestConfig{
		Method: "GET",
		Path:   "/test-bucket/test-key",
		Headers: map[string]string{
			"X-Custom-Header":  "custom-value",
			"X-Another-Header": "another-value",
		},
		QueryParams: map[string]string{
			"versioning": "false",
		},
	}

	req := BuildTestRequest(config)

	if req.Method != "GET" {
		t.Errorf("Expected method GET, got %s", req.Method)
	}

	if req.Header.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("Expected custom header value")
	}

	if req.Header.Get("X-Another-Header") != "another-value" {
		t.Errorf("Expected another header value")
	}

	if !strings.Contains(req.URL.RawQuery, "versioning=false") {
		t.Errorf("Expected query parameter in URL")
	}
}

// TestWithAuthHeader verifies authorization header addition.
func TestWithAuthHeader(t *testing.T) {
	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
	req = WithAuthHeader(req, "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234")

	authHeader := req.Header.Get("Authorization")
	if !strings.Contains(authHeader, "AWS4-HMAC-SHA256") {
		t.Errorf("Expected AWS4-HMAC-SHA256 in auth header")
	}

	if !strings.Contains(authHeader, "TESTACCESSKEY") {
		t.Errorf("Expected access key in auth header")
	}

	dateHeader := req.Header.Get("X-Amz-Date")
	if dateHeader == "" {
		t.Errorf("Expected X-Amz-Date header to be set")
	}
}

// TestWithDateHeader verifies date header addition.
func TestWithDateHeader(t *testing.T) {
	now := time.Now()
	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
	req = WithDateHeader(req, now)

	dateHeader := req.Header.Get("X-Amz-Date")
	expectedDate := now.Format("20060102T150405Z")

	if dateHeader != expectedDate {
		t.Errorf("Expected date %s, got %s", expectedDate, dateHeader)
	}
}

// TestWithExpiredDate verifies expired date header.
func TestWithExpiredDate(t *testing.T) {
	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
	req = WithExpiredDate(req)

	dateHeader := req.Header.Get("X-Amz-Date")
	if dateHeader == "" {
		t.Errorf("Expected X-Amz-Date header to be set")
	}

	// Parse the date to verify it's in the past
	parsedTime, err := time.Parse("20060102T150405Z", dateHeader)
	if err != nil {
		t.Fatalf("Failed to parse date header: %v", err)
	}

	if parsedTime.After(time.Now().Add(-15 * time.Minute)) {
		t.Errorf("Expected expired date to be at least 15 minutes in the past")
	}
}

// TestWithHeader verifies custom header addition.
func TestWithHeader(t *testing.T) {
	req := NewTestRequest("GET", "/test-bucket/test-key", nil)
	req = WithHeader(req, "X-Custom-Header", "custom-value")

	value := req.Header.Get("X-Custom-Header")
	if value != "custom-value" {
		t.Errorf("Expected header value 'custom-value', got '%s'", value)
	}
}

// =============================================================================
// REQUEST EXECUTION TESTS
// =============================================================================

// TestMakeRequest verifies basic request execution.
func TestMakeRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	req := NewTestRequest("GET", "/test", nil)
	resp := MakeRequest(handler, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	body := resp.Body.String()
	if body != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", body)
	}
}

// TestMakeRequestWithTiming verifies request execution with timing.
func TestMakeRequestWithTiming(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	req := NewTestRequest("GET", "/test", nil)
	resp := MakeRequestWithTiming(handler, req)

	if resp.ResponseTime < 10*time.Millisecond {
		t.Errorf("Expected response time >= 10ms, got %v", resp.ResponseTime)
	}

	if resp.ResponseRecorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.ResponseRecorder.Code)
	}
}

// =============================================================================
// RESPONSE VALIDATOR TESTS
// =============================================================================

// TestValidateStatusCode verifies status code validation.
func TestValidateStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		expectedCode int
		expectError  bool
	}{
		{
			name:         "matching status code",
			statusCode:   200,
			expectedCode: 200,
			expectError:  false,
		},
		{
			name:         "non-matching status code",
			statusCode:   404,
			expectedCode: 200,
			expectError:  true,
		},
		{
			name:         "403 status code",
			statusCode:   403,
			expectedCode: 403,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			req := NewTestRequest("GET", "/test", nil)
			resp := MakeRequest(handler, req)

			err := ValidateStatusCode(resp, tt.expectedCode)

			if tt.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestValidateContentType verifies content type validation.
func TestValidateContentType(t *testing.T) {
	tests := []struct {
		name                string
		responseContentType string
		expectedContentType string
		expectError         bool
	}{
		{
			name:                "matching content type",
			responseContentType: "application/xml",
			expectedContentType: "application/xml",
			expectError:         false,
		},
		{
			name:                "non-matching content type",
			responseContentType: "application/json",
			expectedContentType: "application/xml",
			expectError:         true,
		},
		{
			name:                "text/xml content type",
			responseContentType: "text/xml",
			expectedContentType: "text/xml",
			expectError:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.responseContentType)
				w.WriteHeader(http.StatusOK)
			})

			req := NewTestRequest("GET", "/test", nil)
			resp := MakeRequest(handler, req)

			err := ValidateContentType(resp, tt.expectedContentType)

			if tt.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestValidateErrorCode verifies error code validation.
func TestValidateErrorCode(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		expectedCode string
		expectError  bool
	}{
		{
			name:         "matching error code",
			responseBody: `<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingAuthenticationToken</Code><Message>Test message</Message></Error>`,
			expectedCode: "MissingAuthenticationToken",
			expectError:  false,
		},
		{
			name:         "non-matching error code",
			responseBody: `<?xml version="1.0" encoding="UTF-8"?><Error><Code>InvalidAccessKeyId</Code><Message>Test message</Message></Error>`,
			expectedCode: "MissingAuthenticationToken",
			expectError:  true,
		},
		{
			name:         "invalid XML",
			responseBody: `invalid xml`,
			expectedCode: "MissingAuthenticationToken",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(tt.responseBody))
			})

			req := NewTestRequest("GET", "/test", nil)
			resp := MakeRequest(handler, req)

			err := ValidateErrorCode(resp, tt.expectedCode)

			if tt.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestValidateErrorMessage verifies error message validation.
func TestValidateErrorMessage(t *testing.T) {
	tests := []struct {
		name              string
		responseBody      string
		expectedSubstring string
		expectError       bool
	}{
		{
			name:              "message contains substring",
			responseBody:      `<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingAuthenticationToken</Code><Message>The request must include authentication token</Message></Error>`,
			expectedSubstring: "authentication",
			expectError:       false,
		},
		{
			name:              "message does not contain substring",
			responseBody:      `<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingAuthenticationToken</Code><Message>The request must include authentication token</Message></Error>`,
			expectedSubstring: "authorization",
			expectError:       true,
		},
		{
			name:              "case-insensitive matching",
			responseBody:      `<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingAuthenticationToken</Code><Message>AUTHENTICATION REQUIRED</Message></Error>`,
			expectedSubstring: "authentication",
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(tt.responseBody))
			})

			req := NewTestRequest("GET", "/test", nil)
			resp := MakeRequest(handler, req)

			err := ValidateErrorMessage(resp, tt.expectedSubstring)

			if tt.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestValidateResponseTime verifies response time validation.
func TestValidateResponseTime(t *testing.T) {
	tests := []struct {
		name        string
		delay       time.Duration
		maxDuration time.Duration
		expectError bool
	}{
		{
			name:        "response within limit",
			delay:       10 * time.Millisecond,
			maxDuration: 100 * time.Millisecond,
			expectError: false,
		},
		{
			name:        "response exceeds limit",
			delay:       150 * time.Millisecond,
			maxDuration: 100 * time.Millisecond,
			expectError: true,
		},
		{
			name:        "response within tolerance",
			delay:       90 * time.Millisecond,
			maxDuration: 100 * time.Millisecond,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tt.delay)
				w.WriteHeader(http.StatusOK)
			})

			req := NewTestRequest("GET", "/test", nil)
			resp := MakeRequestWithTiming(handler, req)

			err := ValidateResponseTime(resp, tt.maxDuration)

			if tt.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestValidateHeader verifies header validation.
func TestValidateHeader(t *testing.T) {
	tests := []struct {
		name          string
		headerName    string
		headerValue   string
		expectedValue string
		expectError   bool
	}{
		{
			name:          "matching header value",
			headerName:    "X-Custom-Header",
			headerValue:   "expected-value",
			expectedValue: "expected-value",
			expectError:   false,
		},
		{
			name:          "non-matching header value",
			headerName:    "X-Custom-Header",
			headerValue:   "actual-value",
			expectedValue: "expected-value",
			expectError:   true,
		},
		{
			name:          "missing header",
			headerName:    "X-Missing-Header",
			headerValue:   "",
			expectedValue: "expected-value",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.headerValue != "" {
					w.Header().Set(tt.headerName, tt.headerValue)
				}
				w.WriteHeader(http.StatusOK)
			})

			req := NewTestRequest("GET", "/test", nil)
			resp := MakeRequest(handler, req)

			err := ValidateHeader(resp, tt.headerName, tt.expectedValue)

			if tt.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// =============================================================================
// COMPREHENSIVE ASSERTION TESTS
// =============================================================================

// TestAssertErrorResponse verifies comprehensive error response assertion.
func TestAssertErrorResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingAuthenticationToken</Code><Message>Test error message</Message></Error>`))
	})

	req := NewTestRequest("GET", "/test", nil)
	resp := MakeRequest(handler, req)

	// This should not panic or fail
	AssertErrorResponse(t, resp, "MissingAuthenticationToken", http.StatusForbidden)
}

// TestAssertAuthenticationError verifies authentication error assertion.
func TestAssertAuthenticationError(t *testing.T) {
	authErrorCodes := []string{
		"MissingAuthenticationToken",
		"InvalidAccessKeyId",
		"SignatureDoesNotMatch",
		"IncompleteSignature",
		"InvalidAlgorithm",
	}

	for _, errorCode := range authErrorCodes {
		t.Run(errorCode, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>` + errorCode + `</Code><Message>Test error message</Message></Error>`))
			})

			req := NewTestRequest("GET", "/test", nil)
			resp := MakeRequest(handler, req)

			// This should not panic or fail
			AssertAuthenticationError(t, resp)
		})
	}
}

// TestAssertAuthorizationError verifies authorization error assertion.
func TestAssertAuthorizationError(t *testing.T) {
	authzErrorCodes := []string{
		"AccessDenied",
		"RequestExpired",
		"MissingDateHeader",
	}

	for _, errorCode := range authzErrorCodes {
		t.Run(errorCode, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>` + errorCode + `</Code><Message>Test error message</Message></Error>`))
			})

			req := NewTestRequest("GET", "/test", nil)
			resp := MakeRequest(handler, req)

			// This should not panic or fail
			AssertAuthorizationError(t, resp)
		})
	}
}

// =============================================================================
// END-TO-END INTEGRATION TESTS
// =============================================================================

// TestValidationHelpersIntegration verifies helpers work together correctly.
func TestValidationHelpersIntegration(t *testing.T) {
	t.Run("complete error response validation", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			w.Header().Set("X-Custom-Header", "custom-value")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingAuthenticationToken</Code><Message>The request must include authentication token</Message></Error>`))
		})

		req := NewTestRequest("GET", "/test-bucket/test-key", nil)
		resp := MakeRequest(handler, req)

		// Validate all aspects of the error response
		AssertStatusCode(t, resp, http.StatusForbidden)
		AssertContentType(t, resp, "application/xml")
		AssertErrorType(t, resp, "MissingAuthenticationToken")
		AssertErrorMessage(t, resp, "authentication")
		AssertHeader(t, resp, "X-Custom-Header", "custom-value")
	})

	t.Run("request builder and execution flow", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingAuthenticationToken</Code><Message>Authentication required</Message></Error>`))
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		// Test without authentication
		req := NewTestRequest("GET", "/test-bucket/test-key", nil)
		resp := MakeRequest(handler, req)
		AssertAuthenticationError(t, resp)

		// Test with authentication
		req = NewTestRequest("GET", "/test-bucket/test-key", nil)
		req = WithAuthHeader(req, "TESTKEY", "TESTSECRET")
		req = WithDateHeader(req, time.Now())
		resp = MakeRequest(handler, req)
		AssertStatusCode(t, resp, http.StatusOK)
	})

	t.Run("performance validation", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingAuthenticationToken</Code><Message>Authentication required</Message></Error>`))
		})

		req := NewTestRequest("GET", "/test-bucket/test-key", nil)
		resp := MakeRequestWithTiming(handler, req)

		// Validate performance requirements
		AssertResponseTimeUnder(t, resp, 100*time.Millisecond)
		AssertErrorResponse(t, resp.ResponseRecorder, "MissingAuthenticationToken", http.StatusForbidden)
	})
}
