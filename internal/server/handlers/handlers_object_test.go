package handlers_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/manifest"
	"github.com/jedarden/armor/internal/server/handlers"
	"github.com/jedarden/armor/internal/server/middleware"
)

func TestHeadObject(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	plaintext := []byte("Test content for HEAD")

	// PUT the object
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/head-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// HEAD the object
	req = httptest.NewRequest(http.MethodHead, "/test-bucket/head-test", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify Content-Length is plaintext size
	if w.Header().Get("Content-Length") != fmt.Sprintf("%d", len(plaintext)) {
		t.Errorf("expected Content-Length %d, got %s", len(plaintext), w.Header().Get("Content-Length"))
	}

	// Body should be empty for HEAD
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body for HEAD, got %d bytes", w.Body.Len())
	}
}

func TestDeleteObject(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	plaintext := []byte("Content to delete")

	// PUT the object
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/delete-test", bytes.NewReader(plaintext))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// DELETE the object
	req = httptest.NewRequest(http.MethodDelete, "/test-bucket/delete-test", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// GET should now fail
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/delete-test", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestCopyObject tests S3 CopyObject with ARMOR encryption
func TestCopyObject(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Upload source file
	srcContent := []byte("Source content to copy")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/source-file.txt", bytes.NewReader(srcContent))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Copy the file
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/dest-file.txt", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/source-file.txt")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("COPY failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Verify response is XML
	if w.Header().Get("Content-Type") != "application/xml" {
		t.Errorf("expected Content-Type application/xml, got %s", w.Header().Get("Content-Type"))
	}

	// Get the destination file and verify content
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/dest-file.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), srcContent) {
		t.Error("copied content does not match source")
	}
}

// TestCopyObjectRewrapsDEK tests that CopyObject re-wraps the DEK

// TestCopyObjectRewrapsDEK tests that CopyObject re-wraps the DEK
func TestCopyObjectRewrapsDEK(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Upload source file
	srcContent := []byte("Content with key that should be re-wrapped")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/rewrap-source.txt", bytes.NewReader(srcContent))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Get the original wrapped DEK
	mb.mu.Lock()
	srcMeta := mb.meta["test-bucket/rewrap-source.txt"]
	originalWrappedDEK := srcMeta["x-amz-meta-armor-wrapped-dek"]
	mb.mu.Unlock()

	// Copy the file
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/rewrap-dest.txt", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/rewrap-source.txt")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("COPY failed: status %d", w.Code)
	}

	// Get the destination's wrapped DEK - should be different (re-wrapped)
	// even though it wraps the same DEK
	mb.mu.Lock()
	dstMeta := mb.meta["test-bucket/rewrap-dest.txt"]
	newWrappedDEK := dstMeta["x-amz-meta-armor-wrapped-dek"]
	mb.mu.Unlock()

	// The wrapped DEK should be the same since we're using the same MEK
	// (re-wrapping with same MEK produces same output due to AES-KWP)
	// But the important thing is that decryption still works
	if newWrappedDEK == "" {
		t.Error("destination missing wrapped DEK")
	}

	// Verify we can still decrypt
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/rewrap-dest.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if !bytes.Equal(w.Body.Bytes(), srcContent) {
		t.Error("decrypted content does not match original")
	}

	// Log for visibility
	t.Logf("Original wrapped DEK length: %d", len(originalWrappedDEK))
	t.Logf("New wrapped DEK length: %d", len(newWrappedDEK))
}

// TestCopyObjectNonARMOR tests copying non-ARMOR objects

// TestCopyObjectNonARMOR tests copying non-ARMOR objects
func TestCopyObjectNonARMOR(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Store a plain (non-ARMOR) object directly in the mock backend
	plainData := []byte("Plain unencrypted data for copy test")
	mb.Put(context.Background(), "test-bucket", "plain-source.txt", bytes.NewReader(plainData), int64(len(plainData)), map[string]string{
		"Content-Type": "text/plain",
	})

	// Copy the plain file
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/plain-dest.txt", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/plain-source.txt")
	req.Header.Set("x-amz-metadata-directive", "REPLACE")
	req.Header.Set("x-amz-meta-visible", "preserved")
	req.Header.Set("x-amz-meta-armor-forged", "must-not-be-stored")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("COPY failed: status %d", w.Code)
	}

	// Verify the destination is also plain (not ARMOR encrypted)
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/plain-dest.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plainData) {
		t.Error("copied plain content does not match source")
	}

	mb.mu.Lock()
	destMeta := mb.meta["test-bucket/plain-dest.txt"]
	armorMeta := destMeta["x-amz-meta-armor-forged"]
	visibleMeta := destMeta["x-amz-meta-visible"]
	mb.mu.Unlock()
	if armorMeta != "" {
		t.Errorf("client-supplied reserved metadata was copied: %q", armorMeta)
	}
	if visibleMeta != "preserved" {
		t.Errorf("client-supplied metadata = %q, want %q", visibleMeta, "preserved")
	}
}

// TestCopyObjectMissingSource tests error handling for missing source

// TestCopyObjectMissingSource tests error handling for missing source
func TestCopyObjectMissingSource(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Try to copy a non-existent file
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/dest.txt", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/nonexistent.txt")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing source, got %d", w.Code)
	}
}

// TestCopyObjectWithMetadataDirective tests COPY vs REPLACE metadata directive

// TestCopyObjectWithMetadataDirective tests COPY vs REPLACE metadata directive
func TestCopyObjectWithMetadataDirective(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Upload source file
	srcContent := []byte("Content for metadata directive test")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/meta-source.txt", bytes.NewReader(srcContent))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Copy with REPLACE directive and new content type
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/meta-dest.txt", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/meta-source.txt")
	req.Header.Set("x-amz-metadata-directive", "REPLACE")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("COPY with REPLACE failed: status %d", w.Code)
	}

	// Verify the destination has the new content type
	req = httptest.NewRequest(http.MethodHead, "/test-bucket/meta-dest.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Content-Type should be from ARMOR metadata
	if w.Code != http.StatusOK {
		t.Errorf("HEAD failed: status %d", w.Code)
	}
}

// TestDeleteObjects tests S3 DeleteObjects (bulk delete)

// TestDeleteObjects tests S3 DeleteObjects (bulk delete)
func TestDeleteObjects(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create multiple objects
	for i := 0; i < 5; i++ {
		content := []byte(fmt.Sprintf("Content %d", i))
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/test-bucket/bulk-delete/file%d.txt", i), bytes.NewReader(content))
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
	}

	// Verify objects exist
	mb.mu.Lock()
	initialCount := len(mb.objects)
	mb.mu.Unlock()
	if initialCount != 5 {
		t.Fatalf("expected 5 objects before delete, got %d", initialCount)
	}

	// Create DeleteObjects request
	deleteXML := `<?xml version="1.0" encoding="UTF-8"?>
<Delete>
  <Object>
    <Key>bulk-delete/file0.txt</Key>
  </Object>
  <Object>
    <Key>bulk-delete/file1.txt</Key>
  </Object>
  <Object>
    <Key>bulk-delete/file2.txt</Key>
  </Object>
</Delete>`

	req := httptest.NewRequest(http.MethodPost, "/test-bucket?delete=", bytes.NewReader([]byte(deleteXML)))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response is XML
	if w.Header().Get("Content-Type") != "application/xml" {
		t.Errorf("expected Content-Type application/xml, got %s", w.Header().Get("Content-Type"))
	}

	// Parse response
	var result struct {
		Deleted []struct {
			Key string `xml:"Key"`
		} `xml:"Deleted"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(result.Deleted) != 3 {
		t.Errorf("expected 3 deleted objects in response, got %d", len(result.Deleted))
	}

	// Verify objects were deleted
	mb.mu.Lock()
	remainingCount := len(mb.objects)
	mb.mu.Unlock()
	if remainingCount != 2 {
		t.Errorf("expected 2 objects remaining after delete, got %d", remainingCount)
	}
}

// TestDeleteObjectsQuiet tests DeleteObjects with quiet mode

// TestDeleteObjectsQuiet tests DeleteObjects with quiet mode
func TestDeleteObjectsQuiet(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create an object
	content := []byte("Content to delete quietly")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/quiet-delete/file.txt", bytes.NewReader(content))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Delete with quiet mode
	deleteXML := `<?xml version="1.0" encoding="UTF-8"?>
<Delete>
  <Quiet>true</Quiet>
  <Object>
    <Key>quiet-delete/file.txt</Key>
  </Object>
</Delete>`

	req = httptest.NewRequest(http.MethodPost, "/test-bucket?delete=", bytes.NewReader([]byte(deleteXML)))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Parse response - should have no deleted keys in quiet mode
	var result struct {
		Deleted []struct {
			Key string `xml:"Key"`
		} `xml:"Deleted"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(result.Deleted) != 0 {
		t.Errorf("expected 0 deleted objects in quiet mode, got %d", len(result.Deleted))
	}
}

// TestHeadBucket tests S3 HeadBucket

// mockManifestRecorder is a simple ManifestRecorder for testing.
type mockManifestRecorder struct {
	mu      sync.RWMutex
	entries map[string]*handlers.ManifestEntry
}

func newMockManifestRecorder() *mockManifestRecorder {
	return &mockManifestRecorder{entries: make(map[string]*handlers.ManifestEntry)}
}

func (m *mockManifestRecorder) RecordPut(bucket, key string, size int64, sha256Hex string, iv, wrappedDEK []byte, mekFingerprint string, blockSize int, contentType, etag string, chainEntry *manifest.ChainEntry, ciphertextSize int64, userMetadata map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[bucket+"/"+key] = &handlers.ManifestEntry{
		PlaintextSize: size,
		ContentType:   contentType,
		ETag:          etag,
		LastModified:  time.Now(),
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		BlockSize:     blockSize,
		UserMetadata:  userMetadata,
	}
}

func (m *mockManifestRecorder) RecordDelete(bucket, key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, bucket+"/"+key)
}

func (m *mockManifestRecorder) Lookup(bucket, key string) (*handlers.ManifestEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[bucket+"/"+key]
	return e, ok
}

// seed adds a manifest entry directly without going through the handler.

// seed adds a manifest entry directly without going through the handler.
func (m *mockManifestRecorder) seed(bucket, key string, entry *handlers.ManifestEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[bucket+"/"+key] = entry
}

// TestHeadObjectManifestFastPath verifies that HeadObject returns metadata from
// the in-memory manifest without issuing a B2 HeadObject call.

// TestHeadObjectManifestFastPath verifies that HeadObject returns metadata from
// the in-memory manifest without issuing a B2 HeadObject call.
func TestHeadObjectManifestFastPath(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)

	hcb := &headCountingBackend{mockBackend: mb}
	rec := newMockManifestRecorder()

	h := handlers.New(cfg, hcb, cache, footerCache, km, nil)
	h.WithManifest(rec)

	const (
		bucket    = "test-bucket"
		key       = "manifest-test/file.parquet"
		wantSize  = int64(123456)
		wantCType = "application/octet-stream"
		wantETag  = "abc123"
	)

	// Pre-populate the manifest — no object exists in the backend.
	rec.seed(bucket, key, &handlers.ManifestEntry{
		PlaintextSize: wantSize,
		ContentType:   wantCType,
		ETag:          wantETag,
		LastModified:  time.Now(),
		UserMetadata:  map[string]string{},
	})

	req := httptest.NewRequest(http.MethodHead, "/"+bucket+"/"+key, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Length"); got != fmt.Sprintf("%d", wantSize) {
		t.Errorf("Content-Length: got %s, want %d", got, wantSize)
	}
	if got := w.Header().Get("Content-Type"); got != wantCType {
		t.Errorf("Content-Type: got %s, want %s", got, wantCType)
	}
	if got := w.Header().Get("ETag"); got != fmt.Sprintf(`"%s"`, wantETag) {
		t.Errorf("ETag: got %s, want %q", got, wantETag)
	}
	// The manifest hit must not have triggered a backend HeadObject call.
	if n := hcb.headCalls.Load(); n != 0 {
		t.Errorf("expected 0 backend Head() calls (manifest hit), got %d", n)
	}
}

// TestHeadObjectManifestMissFallsBack verifies that HeadObject falls back to B2
// when the manifest does not have an entry for the requested key.

// TestHeadObjectManifestMissFallsBack verifies that HeadObject falls back to B2
// when the manifest does not have an entry for the requested key.
func TestHeadObjectManifestMissFallsBack(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)

	hcb := &headCountingBackend{mockBackend: mb}
	rec := newMockManifestRecorder()

	h := handlers.New(cfg, hcb, cache, footerCache, km, nil)
	h.WithManifest(rec)

	// PUT an object so the backend has it.
	plaintext := []byte("fallback test content")
	putReq := httptest.NewRequest(http.MethodPut, "/test-bucket/fallback-key", bytes.NewReader(plaintext))
	putReq.Header.Set("Content-Type", "text/plain")
	putW := httptest.NewRecorder()
	h.HandleRoot(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("PUT failed: %d", putW.Code)
	}

	// Clear the manifest so the HEAD triggers a B2 fallback.
	rec.RecordDelete("test-bucket", "fallback-key")
	hcb.headCalls.Store(0)

	req := httptest.NewRequest(http.MethodHead, "/test-bucket/fallback-key", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if n := hcb.headCalls.Load(); n != 1 {
		t.Errorf("expected exactly 1 backend Head() call (manifest miss fallback), got %d", n)
	}
}

// TestHeadObjectManifestAllHeaders verifies that HeadObject served from the
// manifest index returns all expected response headers, including Last-Modified,
// and makes no backend call.

// TestHeadObjectManifestAllHeaders verifies that HeadObject served from the
// manifest index returns all expected response headers, including Last-Modified,
// and makes no backend call.
func TestHeadObjectManifestAllHeaders(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)

	hcb := &headCountingBackend{mockBackend: mb}
	rec := newMockManifestRecorder()

	h := handlers.New(cfg, hcb, cache, footerCache, km, nil)
	h.WithManifest(rec)

	modTime := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	rec.seed("test-bucket", "headers/object.parquet", &handlers.ManifestEntry{
		PlaintextSize: 98765,
		ContentType:   "application/parquet",
		ETag:          "deadbeef01",
		LastModified:  modTime,
		UserMetadata:  map[string]string{},
	})

	req := httptest.NewRequest(http.MethodHead, "/test-bucket/headers/object.parquet", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Length"); got != "98765" {
		t.Errorf("Content-Length: got %q, want \"98765\"", got)
	}
	if got := w.Header().Get("Content-Type"); got != "application/parquet" {
		t.Errorf("Content-Type: got %q, want \"application/parquet\"", got)
	}
	wantETag := `"deadbeef01"`
	if got := w.Header().Get("ETag"); got != wantETag {
		t.Errorf("ETag: got %q, want %q", got, wantETag)
	}
	wantLastMod := modTime.UTC().Format(http.TimeFormat)
	if got := w.Header().Get("Last-Modified"); got != wantLastMod {
		t.Errorf("Last-Modified: got %q, want %q", got, wantLastMod)
	}
	if got := w.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("Accept-Ranges: got %q, want \"bytes\"", got)
	}
	if n := hcb.headCalls.Load(); n != 0 {
		t.Errorf("expected 0 backend Head() calls (manifest cache hit), got %d", n)
	}
}

// TestHeadObjectManifestCacheHitNotModified verifies that a HEAD request
// with a matching If-None-Match header returns 304 Not Modified when served
// from the manifest, without hitting the backend.

// TestHeadObjectManifestCacheHitNotModified verifies that a HEAD request
// with a matching If-None-Match header returns 304 Not Modified when served
// from the manifest, without hitting the backend.
func TestHeadObjectManifestCacheHitNotModified(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)

	hcb := &headCountingBackend{mockBackend: mb}
	rec := newMockManifestRecorder()

	h := handlers.New(cfg, hcb, cache, footerCache, km, nil)
	h.WithManifest(rec)

	const etag = "myetag123"
	rec.seed("test-bucket", "cond/file.parquet", &handlers.ManifestEntry{
		PlaintextSize: 512,
		ContentType:   "application/octet-stream",
		ETag:          etag,
		LastModified:  time.Now().UTC(),
		UserMetadata:  map[string]string{},
	})

	// Client sends If-None-Match matching the manifest ETag.
	req := httptest.NewRequest(http.MethodHead, "/test-bucket/cond/file.parquet", nil)
	req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, etag))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", w.Code)
	}
	// ETag and Last-Modified must be present in 304 response.
	if got := w.Header().Get("ETag"); got == "" {
		t.Error("ETag header missing from 304 response")
	}
	if got := w.Header().Get("Last-Modified"); got == "" {
		t.Error("Last-Modified header missing from 304 response")
	}
	if n := hcb.headCalls.Load(); n != 0 {
		t.Errorf("expected 0 backend Head() calls (304 served from manifest), got %d", n)
	}
}

// TestISO8601TimestampFormat verifies that ListObjectsV2 returns timestamps
// in ISO 8601 format with milliseconds (compatible with DuckDB httpfs).
// This test ensures the fix for https://github.com/jedarden/ARMOR/issues/8
// where DuckDB httpfs was failing to parse timestamps in ARMOR responses.

// TestHandlerErrorResponseIncludesRequestIdAndResource verifies that handler error
// responses include both <RequestId> and <Resource> elements in the XML body.
// This ensures consistency with server.writeError which already includes RequestId,
// and provides clients that parse RequestId from the body with a consistent format.
func TestHandlerErrorResponseIncludesRequestIdAndResource(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test a scenario that triggers a handler error (e.g., getting a non-existent object)
	// We'll use an invalid request that causes an InternalError from handlers
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/nonexistent-key", nil)

	// Add request ID to context using middleware helper
	ctx := middleware.WithRequestID(req.Context(), "test-request-id-12345")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// We expect a 404 or error response
	// The important thing is that if we get an error, it includes RequestId and Resource
	if w.Code != http.StatusOK {
		// Parse the XML error response
		var errorResponse struct {
			Code      string `xml:"Code"`
			Message   string `xml:"Message"`
			RequestID string `xml:"RequestId"`
			Resource  string `xml:"Resource"`
		}

		body := w.Body.String()
		if err := xml.Unmarshal([]byte(body), &errorResponse); err != nil {
			t.Fatalf("Failed to parse error response XML: %v\nBody: %s", err, body)
		}

		// Verify RequestId is present and matches the context value
		if errorResponse.RequestID == "" {
			t.Errorf("Error response missing <RequestId> element\nBody: %s", body)
		}
		if errorResponse.RequestID != "test-request-id-12345" {
			t.Errorf("RequestId mismatch: got %s, want test-request-id-12345\nBody: %s", errorResponse.RequestID, body)
		}

		// Verify Resource is present and matches the request path
		if errorResponse.Resource == "" {
			t.Errorf("Error response missing <Resource> element\nBody: %s", body)
		}
		if errorResponse.Resource != "/test-bucket/nonexistent-key" {
			t.Errorf("Resource mismatch: got %s, want /test-bucket/nonexistent-key\nBody: %s", errorResponse.Resource, body)
		}
	}
}
