package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

func TestListObjectsV2(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create multiple objects
	for i := 0; i < 5; i++ {
		content := []byte(fmt.Sprintf("Content %d", i))
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/test-bucket/list-test/file%d.txt", i), bytes.NewReader(content))
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
	}

	// List objects
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=list-test/", nil)
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Parse XML response
	var result struct {
		Contents []struct {
			Key  string `xml:"Key"`
			Size int64  `xml:"Size"`
		} `xml:"Contents"`
	}

	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse XML: %v", err)
	}

	if len(result.Contents) != 5 {
		t.Errorf("expected 5 objects, got %d", len(result.Contents))
	}

	// Verify sizes are plaintext sizes (not encrypted sizes)
	for _, obj := range result.Contents {
		if obj.Size <= 0 || obj.Size > 20 { // Content is "Content X" which is 9-10 bytes
			t.Errorf("unexpected size for %s: %d", obj.Key, obj.Size)
		}
	}
}

func TestListObjectsV2StartAfter(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/test-bucket/start-after/file%d.txt", i), bytes.NewReader([]byte("content")))
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("PUT %d failed: status %d, body %s", i, w.Code, w.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=start-after/&start-after=start-after/file1.txt", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListObjectsV2 failed: status %d, body %s", w.Code, w.Body.String())
	}

	var result struct {
		Contents []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if got, want := len(result.Contents), 3; got != want {
		t.Fatalf("StartAfter returned %d objects, want %d", got, want)
	}
	wantKeys := map[string]bool{
		"start-after/file2.txt": false,
		"start-after/file3.txt": false,
		"start-after/file4.txt": false,
	}
	for _, object := range result.Contents {
		if _, ok := wantKeys[object.Key]; !ok {
			t.Errorf("unexpected StartAfter key %q", object.Key)
			continue
		}
		wantKeys[object.Key] = true
	}
	for key, found := range wantKeys {
		if !found {
			t.Errorf("StartAfter response omitted %q", key)
		}
	}
}

// TestEmptyBucketRendering tests that listing an empty bucket returns valid XML
// with no objects and doesn't cause any errors.
func TestEmptyBucketRendering(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Verify the backend starts empty
	mb.mu.Lock()
	initialObjectCount := len(mb.objects)
	mb.mu.Unlock()

	if initialObjectCount != 0 {
		t.Errorf("expected empty backend, got %d objects", initialObjectCount)
	}

	// List objects in the empty bucket
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2", nil)
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	// Should return 200 OK even for empty bucket
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for empty bucket, got %d: %s", w.Code, w.Body.String())
	}

	// Verify Content-Type is XML
	if w.Header().Get("Content-Type") != "application/xml" {
		t.Errorf("expected Content-Type application/xml, got %s", w.Header().Get("Content-Type"))
	}

	// Parse XML response
	var result struct {
		XMLName     xml.Name `xml:"ListBucketResult"`
		Name        string   `xml:"Name"`
		Prefix      string   `xml:"Prefix"`
		MaxKeys     int      `xml:"MaxKeys"`
		IsTruncated bool     `xml:"IsTruncated"`
		Contents    []struct {
			Key  string `xml:"Key"`
			Size int64  `xml:"Size"`
		} `xml:"Contents"`
		CommonPrefixes []struct {
			Prefix string `xml:"Prefix"`
		} `xml:"CommonPrefixes"`
	}

	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse XML response: %v\nResponse body: %s", err, w.Body.String())
	}

	// Verify basic response fields
	if result.Name != "test-bucket" {
		t.Errorf("expected bucket name 'test-bucket', got '%s'", result.Name)
	}

	if result.MaxKeys != 1000 {
		t.Errorf("expected MaxKeys 1000, got %d", result.MaxKeys)
	}

	if result.IsTruncated {
		t.Error("expected IsTruncated to be false for empty bucket")
	}

	// Empty bucket should have no objects
	if len(result.Contents) != 0 {
		t.Errorf("expected 0 objects in empty bucket, got %d", len(result.Contents))
	}

	// Empty bucket should have no common prefixes
	if len(result.CommonPrefixes) != 0 {
		t.Errorf("expected 0 common prefixes in empty bucket, got %d", len(result.CommonPrefixes))
	}

	// Verify response is well-formed XML
	if !strings.HasPrefix(w.Body.String(), `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Error("expected XML declaration in response")
	}

	// Test with prefix filter on empty bucket
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=data/", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for empty bucket with prefix, got %d", w.Code)
	}

	// Parse and verify empty response with prefix
	var prefixResult struct {
		Prefix   string `xml:"Prefix"`
		Contents []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
	}

	if err := xml.Unmarshal(w.Body.Bytes(), &prefixResult); err != nil {
		t.Fatalf("failed to parse XML with prefix: %v", err)
	}

	if prefixResult.Prefix != "data/" {
		t.Errorf("expected prefix 'data/', got '%s'", prefixResult.Prefix)
	}

	if len(prefixResult.Contents) != 0 {
		t.Errorf("expected 0 objects with prefix filter on empty bucket, got %d", len(prefixResult.Contents))
	}

	// Test with delimiter on empty bucket
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&delimiter=/", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for empty bucket with delimiter, got %d", w.Code)
	}
}

// TestConditionalRequests tests If-Match, If-None-Match, If-Modified-Since, If-Unmodified-Since

// TestListObjectVersions tests the ListObjectVersions operation
func TestListObjectVersions(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Upload a file first
	plaintext := []byte("Version test content")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/version-test.txt", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Now list versions
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?versions", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListObjectVersions failed: status %d", w.Code)
	}

	// Verify response is XML
	if w.Header().Get("Content-Type") != "application/xml" {
		t.Errorf("expected Content-Type application/xml, got %s", w.Header().Get("Content-Type"))
	}

	// Parse XML response
	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	// The response should contain ListVersionsResult
	if !strings.Contains(string(body), "ListVersionsResult") && !strings.Contains(string(body), "ListBucketResult") {
		t.Errorf("response should contain ListVersionsResult, got: %s", string(body))
	}

	// The response should contain the bucket name
	if !strings.Contains(string(body), "test-bucket") {
		t.Errorf("response should contain bucket name, got: %s", string(body))
	}

	// The response should contain the object we uploaded
	if !strings.Contains(string(body), "version-test.txt") {
		t.Errorf("response should contain version-test.txt, got: %s", string(body))
	}
}

// TestListObjectVersions_VersionKeysHavePrefixStripped tests that version keys
// in the response have the configured prefix stripped.

// TestListObjectVersions_VersionKeysHavePrefixStripped tests that version keys
// in the response have the configured prefix stripped.
func TestListObjectVersions_VersionKeysHavePrefixStripped(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		Prefix:        "my-prefix/",
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackend()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Add a version directly to the mock backend with the prefixed key
	testContent := []byte("Test content for version")
	mb.Put(context.Background(), "test-bucket", "my-prefix/test-file.txt", bytes.NewReader(testContent), int64(len(testContent)), map[string]string{
		"Content-Type": "text/plain",
	})

	// List versions
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?versions", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListObjectVersions failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Parse the XML response
	var result struct {
		Versions []struct {
			Key string `xml:"Key"`
		} `xml:"Version"`
	}

	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse XML: %v\nResponse: %s", err, string(body))
	}

	// Verify that the prefix was stripped from version keys
	if len(result.Versions) == 0 {
		t.Fatal("expected at least one version in response")
	}

	for _, v := range result.Versions {
		if strings.HasPrefix(v.Key, "my-prefix/") {
			t.Errorf("version key %q still has prefix, expected it to be stripped", v.Key)
		}
		if v.Key == "my-prefix/test-file.txt" {
			t.Errorf("version key was not stripped: got %q, want %q", v.Key, "test-file.txt")
		}
	}

	// Verify we got the expected key without prefix
	found := false
	for _, v := range result.Versions {
		if v.Key == "test-file.txt" {
			found = true
			break
		}
	}
	if !found {
		keys := make([]string, len(result.Versions))
		for i, v := range result.Versions {
			keys[i] = v.Key
		}
		t.Errorf("expected key 'test-file.txt' in response, got keys: %v", keys)
	}
}

// TestListObjectVersions_CommonPrefixesHavePrefixStripped tests that common prefixes
// in the response have the configured prefix stripped.

// TestListObjectVersions_CommonPrefixesHavePrefixStripped tests that common prefixes
// in the response have the configured prefix stripped.
func TestListObjectVersions_CommonPrefixesHavePrefixStripped(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		Prefix:        "apps/prod/",
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackend()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Add some objects with different directories
	testContent := []byte("Test content")
	mb.Put(context.Background(), "test-bucket", "apps/prod/data/file1.txt", bytes.NewReader(testContent), int64(len(testContent)), map[string]string{
		"Content-Type": "text/plain",
	})
	mb.Put(context.Background(), "test-bucket", "apps/prod/logs/file2.txt", bytes.NewReader(testContent), int64(len(testContent)), map[string]string{
		"Content-Type": "text/plain",
	})

	// List versions with delimiter to get common prefixes
	// Note: We pass prefix= (empty) because the configured prefix is hidden from clients.
	// The handler will apply the configured prefix, making it look for apps/prod/* in the backend.
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?versions&delimiter=/&prefix=", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListObjectVersions failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Parse the XML response
	var result struct {
		CommonPrefixes []string `xml:"CommonPrefixes>Prefix"`
	}

	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse XML: %v\nResponse: %s", err, string(body))
	}

	// Verify that the prefix was stripped from common prefixes
	for _, cp := range result.CommonPrefixes {
		if strings.HasPrefix(cp, "apps/prod/") {
			t.Errorf("common prefix %q still has the configured prefix, expected it to be stripped", cp)
		}
	}

	// Verify we get the expected common prefixes without the configured prefix
	expectedPrefixes := map[string]bool{
		"data/": false,
		"logs/": false,
	}

	for _, cp := range result.CommonPrefixes {
		if _, expected := expectedPrefixes[cp]; expected {
			expectedPrefixes[cp] = true
		}
	}

	for prefix, found := range expectedPrefixes {
		if !found {
			t.Errorf("expected common prefix %q not found in response", prefix)
		}
	}
}

// versionListCaptureBackend wraps mockBackend and records the arguments passed
// to ListObjectVersions, so tests can assert the handler forwards the correct
// (configured + client) prefix to the backend — the criterion the pure stripping
// tests below cannot check on their own.

// versionListCaptureBackend wraps mockBackend and records the arguments passed
// to ListObjectVersions, so tests can assert the handler forwards the correct
// (configured + client) prefix to the backend — the criterion the pure stripping
// tests below cannot check on their own.
type versionListCaptureBackend struct {
	*mockBackend
	mu                 sync.Mutex
	listVersionsCalls  int
	gotPrefix          string
	gotDelimiter       string
	gotKeyMarker       string
	gotVersionIDMarker string
}

func (c *versionListCaptureBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*backend.ListObjectVersionsResult, error) {
	c.mu.Lock()
	c.listVersionsCalls++
	c.gotPrefix = prefix
	c.gotDelimiter = delimiter
	c.gotKeyMarker = keyMarker
	c.gotVersionIDMarker = versionIDMarker
	c.mu.Unlock()
	return c.mockBackend.ListObjectVersions(ctx, bucket, prefix, delimiter, keyMarker, versionIDMarker, maxKeys)
}

// lastPrefix returns the prefix argument from the most recent ListObjectVersions
// call. Locked so the read synchronises with the write and is race-clean.

// lastPrefix returns the prefix argument from the most recent ListObjectVersions
// call. Locked so the read synchronises with the write and is race-clean.
func (c *versionListCaptureBackend) lastPrefix() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gotPrefix
}

// TestListObjectVersions_PrefixEndToEnd verifies the full prefix round-trip when
// a client supplies a prefix on top of a configured ARMOR_PREFIX:
//  1. the client prefix is composed with the configured prefix and passed to the backend,
//  2. the backend filters using that composed prefix,
//  3. the configured prefix is stripped from the returned keys,
//  4. the client sees only its own unprefixed keys and its own prefix echoed back.

// TestListObjectVersions_PrefixEndToEnd verifies the full prefix round-trip when
// a client supplies a prefix on top of a configured ARMOR_PREFIX:
//  1. the client prefix is composed with the configured prefix and passed to the backend,
//  2. the backend filters using that composed prefix,
//  3. the configured prefix is stripped from the returned keys,
//  4. the client sees only its own unprefixed keys and its own prefix echoed back.
func TestListObjectVersions_PrefixEndToEnd(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		Prefix:        "vault/",
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackend()
	cb := &versionListCaptureBackend{mockBackend: mb}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, cb, cache, footerCache, km, nil)

	// Seed objects in the backend with the configured prefix already applied
	// (as the backing store would hold them). Two sit under the client's "a/"
	// prefix; one does not, so we can prove the client prefix is honoured.
	content := []byte("end-to-end prefix content")
	for _, key := range []string{"vault/a/x.txt", "vault/a/y.txt", "vault/b/z.txt"} {
		if err := mb.Put(context.Background(), "test-bucket", key, bytes.NewReader(content), int64(len(content)), map[string]string{
			"Content-Type": "text/plain",
		}); err != nil {
			t.Fatalf("seed put %q failed: %v", key, err)
		}
	}

	// The client lists versions for prefix "a/" — it knows nothing of "vault/".
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?versions&prefix=a/", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListObjectVersions failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Criterion 1: the composed prefix (configured + client) reached the backend.
	if got, want := cb.lastPrefix(), "vault/a/"; got != want {
		t.Errorf("backend received prefix %q, want %q (configured + client)", got, want)
	}

	// Parse the XML response.
	var result struct {
		Prefix   string `xml:"Prefix"`
		Versions []struct {
			Key string `xml:"Key"`
		} `xml:"Version"`
	}
	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse XML: %v\nResponse: %s", err, string(body))
	}

	// Criterion 3: the echoed <Prefix> is the client's own (unprefixed) value.
	if got, want := result.Prefix, "a/"; got != want {
		t.Errorf("response Prefix = %q, want %q", got, want)
	}

	// Criteria 2 & 3: every returned key is unprefixed, limited to "a/", and
	// the out-of-prefix object never leaks through.
	wantKeys := map[string]bool{"a/x.txt": false, "a/y.txt": false}
	for _, v := range result.Versions {
		if strings.HasPrefix(v.Key, "vault/") {
			t.Errorf("version key %q still carries the configured prefix", v.Key)
		}
		if _, ok := wantKeys[v.Key]; ok {
			wantKeys[v.Key] = true
		} else {
			t.Errorf("unexpected version key %q in response", v.Key)
		}
	}
	for key, found := range wantKeys {
		if !found {
			keys := make([]string, len(result.Versions))
			for i, v := range result.Versions {
				keys[i] = v.Key
			}
			t.Errorf("expected unprefixed key %q in response, got keys: %v", key, keys)
		}
	}
}

// countingListBackend wraps mockBackend and counts List() calls.

// countingListBackend wraps mockBackend and counts List() calls.
type countingListBackend struct {
	*mockBackend
	mu        sync.Mutex
	listCalls int
}

func (c *countingListBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	c.mu.Lock()
	c.listCalls++
	c.mu.Unlock()
	return c.mockBackend.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

func (c *countingListBackend) listCallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.listCalls
}

func TestListObjects_CacheHit(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	cb := &countingListBackend{mockBackend: mb}
	lc := backend.NewListCache(100, 60)
	h := handlers.New(cfg, cb, cache, footerCache, km, lc)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=data/", nil)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d: %s", i, w.Code, w.Body.String())
		}
	}

	if cb.listCallCount() != 1 {
		t.Errorf("expected backend.List() called once within TTL, got %d", cb.listCallCount())
	}
}

func TestListObjects_CacheInvalidatedByPut(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	cb := &countingListBackend{mockBackend: mb}
	lc := backend.NewListCache(100, 60)
	h := handlers.New(cfg, cb, cache, footerCache, km, lc)

	// First list — populates cache
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=data/", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list 1: expected 200, got %d", w.Code)
	}

	// PutObject under the same prefix — invalidates the cache entry
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/data/file.txt", bytes.NewReader([]byte("content")))
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("put: expected 200, got %d", w.Code)
	}

	// Second list — cache entry was invalidated, backend must be called again
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=data/", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list 2: expected 200, got %d", w.Code)
	}

	if cb.listCallCount() != 2 {
		t.Errorf("expected backend.List() called twice after PutObject invalidation, got %d", cb.listCallCount())
	}
}

func TestListObjects_CacheInvalidatedByPutWithBarePrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	cb := &countingListBackend{mockBackend: mb}
	lc := backend.NewListCache(100, 60)
	h := handlers.New(cfg, cb, cache, footerCache, km, lc)

	// A boto3-style Prefix omits the trailing slash. The cached empty result
	// must not survive a write below that prefix.
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=data", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list 1: expected 200, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/test-bucket/data/file.txt", bytes.NewReader([]byte("content")))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("put: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=data", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list 2: expected 200, got %d", w.Code)
	}
	if cb.listCallCount() != 2 {
		t.Errorf("expected backend.List() called twice after bare-prefix PutObject invalidation, got %d", cb.listCallCount())
	}
}

func TestListObjects_DisabledCache(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	cb := &countingListBackend{mockBackend: mb}
	// nil listCache simulates TTL=0 disabled path
	h := handlers.New(cfg, cb, cache, footerCache, km, nil)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=data/", nil)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d: %s", i, w.Code, w.Body.String())
		}
	}

	if cb.listCallCount() != 2 {
		t.Errorf("expected backend.List() called twice with disabled cache, got %d", cb.listCallCount())
	}
}

// headCountingBackend wraps mockBackend and counts Head() calls.

// headCountingBackend wraps mockBackend and counts Head() calls.
type headCountingBackend struct {
	*mockBackend
	headCalls atomic.Int64
}

func (h *headCountingBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	h.headCalls.Add(1)
	return h.mockBackend.Head(ctx, bucket, key)
}

// mockManifestRecorder is a simple ManifestRecorder for testing.

// TestISO8601TimestampFormat verifies that ListObjectsV2 returns timestamps
// in ISO 8601 format with milliseconds (compatible with DuckDB httpfs).
// This test ensures the fix for https://github.com/jedarden/ARMOR/issues/8
// where DuckDB httpfs was failing to parse timestamps in ARMOR responses.
func TestISO8601TimestampFormat(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Create a test object
	content := []byte("test content for timestamp verification")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/ts-test/file.txt", bytes.NewReader(content))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to create test object: status %d", w.Code)
	}

	// List objects to get the XML response
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=ts-test/", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListObjectsV2 failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Parse XML and verify timestamp format
	type ListResult struct {
		XMLName  xml.Name `xml:"ListBucketResult"`
		Contents []struct {
			Key          string `xml:"Key"`
			LastModified string `xml:"LastModified"`
		} `xml:"Contents"`
	}

	var result ListResult
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse XML: %v", err)
	}

	if len(result.Contents) == 0 {
		t.Fatal("no objects returned")
	}

	// Verify timestamp format is ISO 8601 with milliseconds
	for _, obj := range result.Contents {
		if obj.LastModified == "" {
			t.Errorf("empty LastModified for key %s", obj.Key)
			continue
		}

		// Parse with RFC3339 (DuckDB uses ISO 8601 which is compatible)
		parsedTime, err := time.Parse(time.RFC3339, obj.LastModified)
		if err != nil {
			t.Errorf("timestamp %q is not valid RFC3339/ISO 8601: %v", obj.LastModified, err)
			continue
		}

		// Verify it has milliseconds (3 decimal places)
		// Expected format: 2006-01-02T15:04:05.000Z (24 chars minimum)
		if len(obj.LastModified) < 24 {
			t.Errorf("timestamp %q appears to lack milliseconds (expected format: 2006-01-02T15:04:05.000Z)", obj.LastModified)
		}

		// Verify the format string matches what we expect
		expectedFormat := parsedTime.UTC().Format("2006-01-02T15:04:05.000Z")
		if obj.LastModified != expectedFormat {
			t.Errorf("timestamp format mismatch: got %q, expected %q", obj.LastModified, expectedFormat)
		}

		t.Logf("✓ %s -> LastModified: %s (valid ISO 8601 with milliseconds, DuckDB httpfs compatible)", obj.Key, obj.LastModified)
	}
}

// TestURLDecodeHivePartitionKeys verifies that ARMOR correctly handles
// URL-encoded Hive partition keys in object paths (DuckDB httpfs encodes '=' as '%3D').
// This test ensures the fix for commit 5638212.

// TestURLDecodeHivePartitionKeys verifies that ARMOR correctly handles
// URL-encoded Hive partition keys in object paths (DuckDB httpfs encodes '=' as '%3D').
// This test ensures the fix for commit 5638212.
func TestURLDecodeHivePartitionKeys(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Hive partition key with '=' characters (typical DuckDB httpfs use case)
	// DuckDB will encode this as: year%3D2024/month%3D06/day%3D08/file.parquet
	hiveKey := "year=2024/month=06/day=08/test.parquet"
	content := []byte("test parquet data")

	// PUT the object with the unencoded key
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/"+hiveKey, bytes.NewReader(content))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to PUT object with Hive partition key: status %d, body: %s", w.Code, w.Body.String())
	}

	// GET the object with URL-encoded key (as DuckDB httpfs sends it)
	// '=' is encoded as '%3D'
	encodedKey := "year%3D2024/month%3D06/day%3D08/test.parquet"
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/"+encodedKey, nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("failed to GET object with URL-encoded Hive partition key: status %d, body: %s", w.Code, w.Body.String())
	}

	// Verify we got the correct content back
	if !bytes.Equal(w.Body.Bytes(), content) {
		t.Errorf("content mismatch: got %q, want %q", w.Body.String(), string(content))
	}

	// HEAD request with URL encoding should also work
	req = httptest.NewRequest(http.MethodHead, "/test-bucket/"+encodedKey, nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HEAD request failed with URL-encoded key: status %d", w.Code)
	}

	t.Logf("✓ URL-encoded Hive partition key (%s) correctly decoded and served", encodedKey)
}

// mockSlowReplicationQueue is a mock replication.Enqueuer that simulates slow enqueue operations.
// It tracks enqueue timing for testing async behavior.
