package server

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server/middleware"
)

// TestS3ErrorResponsesIncludeRequestId verifies that S3 error responses
// include both the x-amz-request-id/x-amz-id-2 headers and the <RequestId>
// XML element in the response body.
func TestS3ErrorResponsesIncludeRequestId(t *testing.T) {
	// Create a minimal test server
	cfg := &config.Config{
		Credentials: map[string]*config.Credential{
			"test-key": {
				AccessKey: "test-key",
				SecretKey: "test-secret",
			},
		},
		B2Region:          "us-west-002",
		B2Endpoint:        "https://s3.us-west-002.backblazeb2.com",
		B2AccessKeyID:     "test-b2-key",
		B2SecretAccessKey: "test-b2-secret",
		Bucket:            "test-bucket",
	}

	// Mock backend - we'll just need a basic implementation
	// For this test, we'll just check the error response format
	s := &Server{
		config: cfg,
		// Other fields can be nil for this test
	}

	// Create a test HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		// Simulate an authentication error using writeError
		s.writeError(w, r, "AccessDenied", "Access Denied", 403)
	})

	// Wrap with the request ID middleware
	handler := middleware.RequestID(mux)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rec, req)

	// Verify response
	resp := rec.Result()
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != 403 {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}

	// Check for x-amz-request-id header
	requestID := resp.Header.Get("x-amz-request-id")
	if requestID == "" {
		t.Error("x-amz-request-id header not set on error response")
	}

	// Check for x-amz-id-2 header
	extendedID := resp.Header.Get("x-amz-id-2")
	if extendedID == "" {
		t.Error("x-amz-id-2 header not set on error response")
	}

	// Read and parse response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Parse XML response
	var errorResponse struct {
		XMLName  xml.Name `xml:"Error"`
		Code     string   `xml:"Code"`
		Message  string   `xml:"Message"`
		RequestID string  `xml:"RequestId"`
	}

	if err := xml.Unmarshal(body, &errorResponse); err != nil {
		t.Fatalf("Failed to parse XML response: %v\nBody: %s", err, string(body))
	}

	// Verify error code and message
	if errorResponse.Code != "AccessDenied" {
		t.Errorf("Expected error code 'AccessDenied', got '%s'", errorResponse.Code)
	}
	if errorResponse.Message != "Access Denied" {
		t.Errorf("Expected error message 'Access Denied', got '%s'", errorResponse.Message)
	}

	// Verify RequestId XML element is present
	if errorResponse.RequestID == "" {
		t.Error("<RequestId> element not found in error response XML")
		t.Errorf("Response body was: %s", string(body))
	}

	// Verify RequestId XML element matches the header value
	if errorResponse.RequestID != requestID {
		t.Errorf("RequestId XML element (%s) doesn't match x-amz-request-id header (%s)",
			errorResponse.RequestID, requestID)
	}

	t.Logf("✓ Error response includes x-amz-request-id: %s", requestID)
	t.Logf("✓ Error response includes x-amz-id-2: %s", extendedID)
	t.Logf("✓ Error response includes <RequestId>: %s", errorResponse.RequestID)
}

// TestS3SuccessResponsesIncludeRequestId verifies that successful S3 responses
// include the x-amz-request-id and x-amz-id-2 headers.
func TestS3SuccessResponsesIncludeRequestId(t *testing.T) {
	// Create a simple success handler
	successHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult></ListBucketResult>`))
	})

	// Wrap with the request ID middleware
	handler := middleware.RequestID(successHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/bucket", nil)
	rec := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rec, req)

	// Verify response
	resp := rec.Result()
	defer resp.Body.Close()

	// Check for x-amz-request-id header
	requestID := resp.Header.Get("x-amz-request-id")
	if requestID == "" {
		t.Error("x-amz-request-id header not set on success response")
	}

	// Check for x-amz-id-2 header
	extendedID := resp.Header.Get("x-amz-id-2")
	if extendedID == "" {
		t.Error("x-amz-id-2 header not set on success response")
	}

	t.Logf("✓ Success response includes x-amz-request-id: %s", requestID)
	t.Logf("✓ Success response includes x-amz-id-2: %s", extendedID)
}

// TestRequestIdMiddlewareIsWiredIntoHandler verifies that the request ID
// middleware is actually wired into the main S3 handler chain.
func TestRequestIdMiddlewareIsWiredIntoHandler(t *testing.T) {
	// Create a minimal config
	cfg := &config.Config{
		Bucket: "test-bucket",
	}

	s := &Server{
		config: cfg,
		// Other fields can be nil for this test
	}

	// Get the main handler
	handler := s.Handler()

	// Create a test request to a public endpoint (healthz)
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rec, req)

	// Verify response
	resp := rec.Result()
	defer resp.Body.Close()

	// Even public endpoints should get request IDs
	requestID := resp.Header.Get("x-amz-request-id")
	if requestID == "" {
		t.Error("x-amz-request-id header not set on /healthz endpoint")
	}

	extendedID := resp.Header.Get("x-amz-id-2")
	if extendedID == "" {
		t.Error("x-amz-id-2 header not set on /healthz endpoint")
	}

	t.Logf("✓ Handler chain includes request ID middleware")
	t.Logf("✓ x-amz-request-id: %s", requestID)
	t.Logf("✓ x-amz-id-2: %s", extendedID)
}
