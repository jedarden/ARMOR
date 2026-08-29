package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/metrics"
)

// TestErrorMetricsIncrement verifies that armor_errors_total is incremented
// from the s3_error hook with correct labels (code and operation).
func TestErrorMetricsIncrement(t *testing.T) {
	// Create a fresh metrics instance for testing
	m := metrics.NewMetrics()

	// Create a minimal handlers instance using handlers.New
	cfg := &config.Config{
		Bucket:   "test-bucket",
		B2Region: "us-east-005",
		MEK:      make([]byte, 32),
	}

	h := New(cfg, nil, nil, nil, nil, nil)
	h.WithMetrics(m)

	// Create test request
	req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
	w := httptest.NewRecorder()

	// Get initial counter value
	initialValue := m.ErrorsTotal.Get("NoSuchKey:GetObject")

	// Trigger an error response
	h.writeError(w, req, "NoSuchKey", "The specified key does not exist", 404)

	// Verify the counter was incremented
	finalValue := m.ErrorsTotal.Get("NoSuchKey:GetObject")
	if finalValue != initialValue+1 {
		t.Errorf("Expected armor_errors_total{code=\"NoSuchKey\",operation=\"GetObject\"} to increment from %d to %d, but got %d",
			initialValue, initialValue+1, finalValue)
	}

	// Verify response was written correctly
	if w.Code != 404 {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/xml" {
		t.Errorf("Expected Content-Type 'application/xml', got '%s'", contentType)
	}
}

// TestErrorMetricsInvalidPartSize verifies that InvalidPartSize errors
// are correctly tracked in armor_errors_total (acceptance criteria).
func TestErrorMetricsInvalidPartSize(t *testing.T) {
	// This is the acceptance test for ADR-008
	// It verifies that a handler test with an InvalidPartSize rejection
	// sees the counter at 1 with the right labels.

	m := metrics.NewMetrics()

	cfg := &config.Config{
		Bucket:   "test-bucket",
		B2Region: "us-east-005",
		MEK:      make([]byte, 32),
	}

	h := New(cfg, nil, nil, nil, nil, nil)
	h.WithMetrics(m)

	// Create test request for CompleteMultipartUpload (where InvalidPartSize occurs)
	req := httptest.NewRequest("POST", "/test-bucket/test-key?uploadId=test", nil)
	w := httptest.NewRecorder()

	// Trigger InvalidPartSize error response
	h.writeError(w, req, "InvalidPartSize", "One or more of the specified parts is invalid", 400)

	// Verify the counter is at 1 with correct labels
	value := m.ErrorsTotal.Get("InvalidPartSize:CompleteMultipartUpload")
	if value != 1 {
		t.Errorf("Expected armor_errors_total{code=\"InvalidPartSize\",operation=\"CompleteMultipartUpload\"} to be 1, got %d", value)
	}

	// Verify response
	if w.Code != 400 {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	body := w.Body.String()
	if !containsSubstring(body, "InvalidPartSize") {
		t.Errorf("Expected response body to contain 'InvalidPartSize', got: %s", body)
	}
}

// TestErrorMetricsMultipleErrors verifies that multiple different errors
// create separate counter entries.
func TestErrorMetricsMultipleErrors(t *testing.T) {
	m := metrics.NewMetrics()

	cfg := &config.Config{
		Bucket:   "test-bucket",
		B2Region: "us-east-005",
		MEK:      make([]byte, 32),
	}

	h := New(cfg, nil, nil, nil, nil, nil)
	h.WithMetrics(m)

	// Simulate multiple different errors
	testCases := []struct {
		code      string
		operation string
		status    int
	}{
		{"AccessDenied", "PutObject", 403},
		{"NoSuchKey", "GetObject", 404},
		{"InvalidPartSize", "CompleteMultipartUpload", 400},
		{"InternalError", "DeleteObject", 500},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		w := httptest.NewRecorder()
		h.writeError(w, req, tc.code, "test message", tc.status)

		key := tc.code + ":" + tc.operation
		value := m.ErrorsTotal.Get(key)
		if value != 1 {
			t.Errorf("Expected %s to be 1, got %d", key, value)
		}
	}

	// Verify each error has its own counter
	if m.ErrorsTotal.Get("AccessDenied:PutObject") != 1 {
		t.Error("AccessDenied:PutObject counter not at 1")
	}
	if m.ErrorsTotal.Get("NoSuchKey:GetObject") != 1 {
		t.Error("NoSuchKey:GetObject counter not at 1")
	}
	if m.ErrorsTotal.Get("InvalidPartSize:CompleteMultipartUpload") != 1 {
		t.Error("InvalidPartSize:CompleteMultipartUpload counter not at 1")
	}
	if m.ErrorsTotal.Get("InternalError:DeleteObject") != 1 {
		t.Error("InternalError:DeleteObject counter not at 1")
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
