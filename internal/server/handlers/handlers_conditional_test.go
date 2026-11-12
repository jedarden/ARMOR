package handlers_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/server/handlers"
)

// TestConditionalRequests tests If-Match, If-None-Match, If-Modified-Since, If-Unmodified-Since
func TestConditionalRequests(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create and upload an object
	plaintext := []byte("Hello, ARMOR! This is a test file for conditional requests.")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/conditional-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed with status %d", w.Code)
	}

	// Get the ETag from the PUT response
	etag := strings.Trim(w.Header().Get("ETag"), `"`)

	// Test 1: If-Match with matching ETag - should succeed
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/conditional-test", nil)
	req.Header.Set("If-Match", fmt.Sprintf(`"%s"`, etag))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("If-Match with matching ETag: expected status 200, got %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("If-Match with matching ETag: content mismatch")
	}

	// Test 2: If-Match with non-matching ETag - should fail with 412
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/conditional-test", nil)
	req.Header.Set("If-Match", `"wrong-etag"`)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusPreconditionFailed {
		t.Errorf("If-Match with non-matching ETag: expected status 412, got %d", w.Code)
	}

	// Test 3: If-Match with * (match any) - should succeed
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/conditional-test", nil)
	req.Header.Set("If-Match", "*")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("If-Match *: expected status 200, got %d", w.Code)
	}

	// Test 4: If-None-Match with matching ETag - should return 304
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/conditional-test", nil)
	req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, etag))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("If-None-Match with matching ETag: expected status 304, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Error("If-None-Match with matching ETag: expected empty body")
	}

	// Test 5: If-None-Match with non-matching ETag - should succeed
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/conditional-test", nil)
	req.Header.Set("If-None-Match", `"wrong-etag"`)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("If-None-Match with non-matching ETag: expected status 200, got %d", w.Code)
	}

	// Test 6: If-Modified-Since with future date - should return 304
	futureTime := time.Now().Add(24 * time.Hour).UTC().Format(http.TimeFormat)
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/conditional-test", nil)
	req.Header.Set("If-Modified-Since", futureTime)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("If-Modified-Since with future date: expected status 304, got %d", w.Code)
	}

	// Test 7: If-Modified-Since with past date - should succeed
	pastTime := time.Now().Add(-24 * time.Hour).UTC().Format(http.TimeFormat)
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/conditional-test", nil)
	req.Header.Set("If-Modified-Since", pastTime)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("If-Modified-Since with past date: expected status 200, got %d", w.Code)
	}

	// Test 8: If-Unmodified-Since with future date - should succeed
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/conditional-test", nil)
	req.Header.Set("If-Unmodified-Since", futureTime)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("If-Unmodified-Since with future date: expected status 200, got %d", w.Code)
	}

	// Test 9: If-Unmodified-Since with past date - should fail with 412
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/conditional-test", nil)
	req.Header.Set("If-Unmodified-Since", pastTime)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusPreconditionFailed {
		t.Errorf("If-Unmodified-Since with past date: expected status 412, got %d", w.Code)
	}
}

// TestHeadConditionalRequests tests conditional headers with HEAD requests

// TestHeadConditionalRequests tests conditional headers with HEAD requests
func TestHeadConditionalRequests(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create and upload an object
	plaintext := []byte("Hello, ARMOR! This is a test file for HEAD conditional requests.")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/head-conditional-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed with status %d", w.Code)
	}

	etag := strings.Trim(w.Header().Get("ETag"), `"`)

	// Test HEAD with If-None-Match matching - should return 304
	req = httptest.NewRequest(http.MethodHead, "/test-bucket/head-conditional-test", nil)
	req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, etag))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("HEAD If-None-Match: expected status 304, got %d", w.Code)
	}

	// Test HEAD with If-Match non-matching - should return 412
	req = httptest.NewRequest(http.MethodHead, "/test-bucket/head-conditional-test", nil)
	req.Header.Set("If-Match", `"wrong-etag"`)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusPreconditionFailed {
		t.Errorf("HEAD If-Match non-matching: expected status 412, got %d", w.Code)
	}

	// Test HEAD with If-Match matching - should return 200
	req = httptest.NewRequest(http.MethodHead, "/test-bucket/head-conditional-test", nil)
	req.Header.Set("If-Match", fmt.Sprintf(`"%s"`, etag))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HEAD If-Match matching: expected status 200, got %d", w.Code)
	}
}

// TestConditionalRequestsWithRange tests conditional headers with range requests

// TestConditionalRequestsWithRange tests conditional headers with range requests
func TestConditionalRequestsWithRange(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create content larger than one block
	plaintext := make([]byte, 200000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/range-conditional-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed with status %d", w.Code)
	}

	etag := strings.Trim(w.Header().Get("ETag"), `"`)

	// Range request with If-Match matching - should succeed
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/range-conditional-test", nil)
	req.Header.Set("Range", "bytes=0-999")
	req.Header.Set("If-Match", fmt.Sprintf(`"%s"`, etag))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("Range If-Match: expected status 206, got %d", w.Code)
	}

	// Range request with If-None-Match matching - should return 304 (not range)
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/range-conditional-test", nil)
	req.Header.Set("Range", "bytes=0-999")
	req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, etag))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("Range If-None-Match: expected status 304, got %d", w.Code)
	}

	// Range request with If-Match non-matching - should fail with 412
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/range-conditional-test", nil)
	req.Header.Set("Range", "bytes=0-999")
	req.Header.Set("If-Match", `"wrong-etag"`)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusPreconditionFailed {
		t.Errorf("Range If-Match non-matching: expected status 412, got %d", w.Code)
	}
}

// TestMultipleETagsInIfMatch tests If-Match with multiple ETags

// TestMultipleETagsInIfMatch tests If-Match with multiple ETags
func TestMultipleETagsInIfMatch(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create and upload an object
	plaintext := []byte("Test content for multiple ETags")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/multi-etag-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed with status %d", w.Code)
	}

	etag := strings.Trim(w.Header().Get("ETag"), `"`)

	// Test If-Match with multiple ETags where one matches
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/multi-etag-test", nil)
	req.Header.Set("If-Match", fmt.Sprintf(`"wrong1", "%s", "wrong2"`, etag))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("If-Match with multiple ETags (one matching): expected status 200, got %d", w.Code)
	}

	// Test If-None-Match with multiple ETags where one matches
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/multi-etag-test", nil)
	req.Header.Set("If-None-Match", fmt.Sprintf(`"wrong1", "%s", "wrong2"`, etag))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("If-None-Match with multiple ETags (one matching): expected status 304, got %d", w.Code)
	}
}

// TestStreamingEncryptionLargeFile tests streaming encryption for files > 10MB
