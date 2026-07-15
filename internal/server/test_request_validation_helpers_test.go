package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// TEST REQUEST HELPERS - TESTS
// =============================================================================

// TestMakeTestRequest tests the MakeTestRequest helper function.
func TestMakeTestRequest(t *testing.T) {
	// Create a simple test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	tests := []struct {
		name    string
		opts    TestRequestOptions
		wantErr bool
	}{
		{
			name: "simple GET request",
			opts: TestRequestOptions{
				Method: "GET",
				Path:   "/test",
			},
			wantErr: false,
		},
		{
			name: "GET with headers",
			opts: TestRequestOptions{
				Method: "GET",
				Path:   "/test",
				Headers: map[string]string{
					"X-Custom-Header": "value",
				},
			},
			wantErr: false,
		},
		{
			name: "POST with body",
			opts: TestRequestOptions{
				Method: "POST",
				Path:   "/test",
				Body:   strings.NewReader("test body"),
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
			},
			wantErr: false,
		},
		{
			name: "request with query parameters",
			opts: TestRequestOptions{
				Method:      "GET",
				Path:        "/test",
				QueryParams: map[string]string{"key": "value"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := MakeTestRequest(server.URL, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeTestRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && resp != nil {
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected status 200, got %d", resp.StatusCode)
				}
			}
		})
	}
}

// TestMakeGETRequest tests the MakeGETRequest helper function.
func TestMakeGETRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := MakeGETRequest(server.URL, "/test")
	if err != nil {
		t.Fatalf("MakeGETRequest() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// TestMakePOSTRequest tests the MakePOSTRequest helper function.
func TestMakePOSTRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "test body" {
			t.Errorf("Expected body 'test body', got '%s'", string(body))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	resp, err := MakePOSTRequest(server.URL, "/test", "test body", "text/plain")
	if err != nil {
		t.Fatalf("MakePOSTRequest() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}
}

// TestMakePUTRequest tests the MakePUTRequest helper function.
func TestMakePUTRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT method, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "updated body" {
			t.Errorf("Expected body 'updated body', got '%s'", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := MakePUTRequest(server.URL, "/test", "updated body", "text/plain")
	if err != nil {
		t.Fatalf("MakePUTRequest() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// TestMakeDELETERequest tests the MakeDELETERequest helper function.
func TestMakeDELETERequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	resp, err := MakeDELETERequest(server.URL, "/test")
	if err != nil {
		t.Fatalf("MakeDELETERequest() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
}

// =============================================================================
// RESPONSE VALIDATION HELPERS - TESTS
// =============================================================================

// TestValidateResponseStructure tests the ValidateResponseStructure helper function.
func TestValidateResponseStructure(t *testing.T) {
	// Create test server that returns S3 errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>The specified key does not exist</Message></Error>`))
	}))
	defer server.Close()

	tests := []struct {
		name string
		opts ResponseValidationOptions
		wantValid bool
	}{
		{
			name: "valid error response",
			opts: ResponseValidationOptions{
				ExpectedStatusCode: 404,
				ExpectedErrorCode:  "NoSuchKey",
			},
			wantValid: true,
		},
		{
			name: "invalid status code",
			opts: ResponseValidationOptions{
				ExpectedStatusCode: 403,
				ExpectedErrorCode:  "NoSuchKey",
			},
			wantValid: false,
		},
		{
			name: "invalid error code",
			opts: ResponseValidationOptions{
				ExpectedStatusCode: 404,
				ExpectedErrorCode:  "AccessDenied",
			},
			wantValid: false,
		},
		{
			name: "valid with message keywords",
			opts: ResponseValidationOptions{
				ExpectedStatusCode: 404,
				ExpectedErrorCode:  "NoSuchKey",
				ExpectedMessageKeywords: []string{"not", "exist"},
			},
			wantValid: true,
		},
		{
			name: "valid with min message length",
			opts: ResponseValidationOptions{
				ExpectedStatusCode: 404,
				ExpectedErrorCode:  "NoSuchKey",
				MinMessageLength: 10,
			},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a fresh request for each test case
			resp, err := http.Get(server.URL + "/test")
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}

			result := ValidateResponseStructure(resp, tt.opts)
			if result.IsValid != tt.wantValid {
				t.Errorf("ValidateResponseStructure() IsValid = %v, want %v (error: %s)", result.IsValid, tt.wantValid, result.ErrorMessage)
			}
		})
	}
}

// TestValidateErrorResponse tests the ValidateErrorResponse helper function.
func TestValidateErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	// This should not panic
	ValidateErrorResponse(t, resp, 403, "AccessDenied")
}

// TestValidateSuccessResponse tests the ValidateSuccessResponse helper function.
func TestValidateSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Result>Success</Result>`))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	// This should not panic
	ValidateSuccessResponse(t, resp, 200, "application/xml")
}

// =============================================================================
// COMBINED HELPERS - TESTS
// =============================================================================

// TestRequestAndValidateError tests the RequestAndValidateError helper function.
func TestRequestAndValidateError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>Not found</Message></Error>`))
	}))
	defer server.Close()

	s3Err := RequestAndValidateError(t, server.URL, TestRequestOptions{
		Method: "GET",
		Path:   "/test",
	}, 404, "NoSuchKey")

	if s3Err.Code != "NoSuchKey" {
		t.Errorf("Expected error code 'NoSuchKey', got '%s'", s3Err.Code)
	}
	if s3Err.Message == "" {
		t.Error("Expected non-empty error message")
	}
}

// TestRequestAndValidateSuccess tests the RequestAndValidateSuccess helper function.
func TestRequestAndValidateSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// This should not panic
	RequestAndValidateSuccess(t, server.URL, TestRequestOptions{
		Method: "GET",
		Path:   "/test",
	}, 200)
}

// =============================================================================
// BATCH REQUESTS - TESTS
// =============================================================================

// TestMakeBatchRequests tests the MakeBatchRequests helper function.
func TestMakeBatchRequests(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	requests := []TestRequestOptions{
		{Method: "GET", Path: "/test1"},
		{Method: "GET", Path: "/test2"},
		{Method: "GET", Path: "/test3"},
	}

	results := MakeBatchRequests(server.URL, requests)

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		if result.Error != nil {
			t.Errorf("Request %d failed: %v", i, result.Error)
		}
		if result.Response.StatusCode != http.StatusOK {
			t.Errorf("Request %d returned status %d", i, result.Response.StatusCode)
		}
		if result.Duration <= 0 {
			t.Errorf("Request %d has invalid duration: %v", i, result.Duration)
		}
		result.Response.Body.Close()
	}

	// Verify all requests were made
	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}
}

// TestMakeBatchRequestsWithTimeout tests batch requests with timeout.
func TestMakeBatchRequestsWithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	requests := []TestRequestOptions{
		{Method: "GET", Path: "/test", Timeout: 50 * time.Millisecond},
	}

	start := time.Now()
	results := MakeBatchRequests(server.URL, requests)
	duration := time.Since(start)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Error == nil {
		t.Error("Expected timeout error, got nil")
	}

	// Batch should complete quickly even with one slow request
	if duration > 1*time.Second {
		t.Errorf("Batch took too long: %v", duration)
	}
}

// =============================================================================
// INTEGRATION TESTS WITH ERROR SERVER
// =============================================================================

// TestRequestHelpersWithConfigurableErrorServer tests helpers with ConfigurableErrorServer.
func TestRequestHelpersWithConfigurableErrorServer(t *testing.T) {
	scenarios := []ErrorServerScenario{
		{
			StatusCode: 404,
			ErrorCode:  "NoSuchKey",
			Message:    "Not found",
		},
	}

	server := NewConfigurableErrorServer(scenarios)
	defer server.Close()

	t.Run("GET request with error validation", func(t *testing.T) {
		s3Err := RequestAndValidateError(t, server.URL, TestRequestOptions{
			Method: "GET",
			Path:   "/resource",
		}, 404, "NoSuchKey")

		if s3Err.Code != "NoSuchKey" {
			t.Errorf("Expected error code 'NoSuchKey', got '%s'", s3Err.Code)
		}
	})

	t.Run("POST request with error validation", func(t *testing.T) {
		s3Err := RequestAndValidateError(t, server.URL, TestRequestOptions{
			Method: "POST",
			Path:   "/resource",
			Body:   strings.NewReader("test"),
		}, 404, "NoSuchKey")

		if s3Err.Code != "NoSuchKey" {
			t.Errorf("Expected error code 'NoSuchKey', got '%s'", s3Err.Code)
		}
	})

	t.Run("detailed validation", func(t *testing.T) {
		resp, err := MakeGETRequest(server.URL, "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		result := ValidateResponseStructure(resp, ResponseValidationOptions{
			ExpectedStatusCode:     404,
			ExpectedErrorCode:     "NoSuchKey",
			ExpectedMessageKeywords: []string{"not", "found"},
			MinMessageLength:       5,
			ValidateStructure:     true,
		})

		if !result.IsValid {
			t.Errorf("Validation failed: %s", result.ErrorMessage)
		}
		if !result.StatusCodeValid {
			t.Error("Status code validation failed")
		}
		if !result.ErrorCodeValid {
			t.Error("Error code validation failed")
		}
		if !result.MessageValid {
			t.Error("Message validation failed")
		}
	})
}

// TestRequestHelpersWithPredefinedScenarios tests helpers with predefined error scenarios.
func TestRequestHelpersWithPredefinedScenarios(t *testing.T) {
	// Test with predefined NotFound scenario
	server := NewSingleErrorServer(404, "NoSuchKey", "The specified key does not exist")
	defer server.Close()

	s3Err := RequestAndValidateError(t, server.URL, TestRequestOptions{
		Method: "GET",
		Path:   "/api/blobs/test.txt",
	}, 404, "NoSuchKey")

	if s3Err.Code != "NoSuchKey" {
		t.Errorf("Expected error code 'NoSuchKey', got '%s'", s3Err.Code)
	}
	if !strings.Contains(strings.ToLower(s3Err.Message), "not") && !strings.Contains(strings.ToLower(s3Err.Message), "exist") {
		t.Errorf("Expected message to contain 'not' or 'exist', got '%s'", s3Err.Message)
	}
}

// TestBatchRequestsWithErrorServer tests batch requests with error server.
func TestBatchRequestsWithErrorServer(t *testing.T) {
	scenarios := []ErrorServerScenario{
		{
			StatusCode:    404,
			ErrorCode:     "NoSuchKey",
			Message:       "Not found",
			RequestMatcher: MatchByPath("/missing"),
		},
		{
			StatusCode:    403,
			ErrorCode:     "AccessDenied",
			Message:       "Access denied",
			RequestMatcher: MatchByPath("/forbidden"),
		},
	}

	server := NewConfigurableErrorServer(scenarios)
	defer server.Close()

	requests := []TestRequestOptions{
		{Method: "GET", Path: "/missing"},
		{Method: "GET", Path: "/forbidden"},
		{Method: "GET", Path: "/missing"},
	}

	results := MakeBatchRequests(server.URL, requests)

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// Check first request (404)
	if results[0].Response.StatusCode != 404 {
		t.Errorf("Request 0: expected status 404, got %d", results[0].Response.StatusCode)
	}
	s3Err0 := GetErrorServerS3Error(t, results[0].Response)
	if s3Err0.Code != "NoSuchKey" {
		t.Errorf("Request 0: expected error code 'NoSuchKey', got '%s'", s3Err0.Code)
	}
	results[0].Response.Body.Close()

	// Check second request (403)
	if results[1].Response.StatusCode != 403 {
		t.Errorf("Request 1: expected status 403, got %d", results[1].Response.StatusCode)
	}
	s3Err1 := GetErrorServerS3Error(t, results[1].Response)
	if s3Err1.Code != "AccessDenied" {
		t.Errorf("Request 1: expected error code 'AccessDenied', got '%s'", s3Err1.Code)
	}
	results[1].Response.Body.Close()

	// Check third request (404)
	if results[2].Response.StatusCode != 404 {
		t.Errorf("Request 2: expected status 404, got %d", results[2].Response.StatusCode)
	}
	results[2].Response.Body.Close()
}
