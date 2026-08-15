package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// TestRequestIDMiddleware verifies that the RequestID middleware:
// 1. Sets x-amz-request-id header
// 2. Sets x-amz-id-2 header
// 3. Stores request IDs in context
func TestRequestIDMiddleware(t *testing.T) {
	// Create a test handler that checks the context
	handlerCalled := false
	var capturedRequestID, capturedExtendedID string

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		capturedRequestID = GetRequestID(r.Context())
		capturedExtendedID = GetExtendedID(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with middleware
	middleware := RequestID(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Serve the request
	middleware.ServeHTTP(rec, req)

	// Verify handler was called
	if !handlerCalled {
		t.Error("Handler was not called")
	}

	// Verify response headers
	resp := rec.Result()
	defer resp.Body.Close()

	requestIDHeader := resp.Header.Get("x-amz-request-id")
	if requestIDHeader == "" {
		t.Error("x-amz-request-id header not set")
	}

	extendedIDHeader := resp.Header.Get("x-amz-id-2")
	if extendedIDHeader == "" {
		t.Error("x-amz-id-2 header not set")
	}

	// Verify context values were captured
	if capturedRequestID == "" {
		t.Error("RequestID not stored in context")
	}
	if capturedExtendedID == "" {
		t.Error("ExtendedID not stored in context")
	}

	// Verify header values match context values
	if requestIDHeader != capturedRequestID {
		t.Errorf("Header request ID (%s) doesn't match context value (%s)", requestIDHeader, capturedRequestID)
	}
	if extendedIDHeader != capturedExtendedID {
		t.Errorf("Header extended ID (%s) doesn't match context value (%s)", extendedIDHeader, capturedExtendedID)
	}

	// Verify request ID is a valid UUID
	if _, err := uuid.Parse(requestIDHeader); err != nil {
		t.Errorf("x-amz-request-id is not a valid UUID: %v", err)
	}

	// Verify extended ID is non-empty and reasonable length (base64 of 16 bytes = 24 chars)
	if len(extendedIDHeader) != 24 {
		t.Errorf("x-amz-id-2 has unexpected length: got %d, expected 24", len(extendedIDHeader))
	}
}

// TestRequestIDUniqueness verifies that each request gets a unique ID.
func TestRequestIDUniqueness(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestID(testHandler)

	// Make multiple requests and collect IDs
	requestIDs := make(map[string]bool)
	extendedIDs := make(map[string]bool)

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		resp := rec.Result()
		requestID := resp.Header.Get("x-amz-request-id")
		extendedID := resp.Header.Get("x-amz-id-2")

		// Check for duplicates
		if requestIDs[requestID] {
			t.Errorf("Duplicate request ID found: %s", requestID)
		}
		if extendedIDs[extendedID] {
			t.Errorf("Duplicate extended ID found: %s", extendedID)
		}

		requestIDs[requestID] = true
		extendedIDs[extendedID] = true
	}

	// Verify we got 100 unique IDs
	if len(requestIDs) != 100 {
		t.Errorf("Expected 100 unique request IDs, got %d", len(requestIDs))
	}
	if len(extendedIDs) != 100 {
		t.Errorf("Expected 100 unique extended IDs, got %d", len(extendedIDs))
	}
}

// TestGetRequestIDEmptyContext verifies that GetRequestID returns empty string
// when the context doesn't contain a request ID.
func TestGetRequestIDEmptyContext(t *testing.T) {
	// Create a context without request ID
	ctx := context.Background()

	requestID := GetRequestID(ctx)
	if requestID != "" {
		t.Errorf("Expected empty string, got %s", requestID)
	}

	extendedID := GetExtendedID(ctx)
	if extendedID != "" {
		t.Errorf("Expected empty string, got %s", extendedID)
	}
}
