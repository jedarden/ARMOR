// Package handlers_test provides integration tests for ARMOR_PREFIX functionality.
// These tests verify that prefix normalization and key rewriting work correctly
// end-to-end with actual S3 operations when ARMOR_PREFIX is configured.
package handlers_test

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// testSetupWithPrefix creates common test dependencies with a configured prefix.
// The prefix is normalized to match config.Load() behavior (ADR-001).
func testSetupWithPrefix(t *testing.T, prefix string) (*config.Config, *mockBackend, *backend.MetadataCache, *backend.FooterCache, *keymanager.KeyManager) {
	t.Helper()

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	// Normalize prefix to match config.Load() behavior
	normalizedPrefix := normalizeTestPrefix(prefix)

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
		Prefix:        normalizedPrefix,
	}

	mb := newMockBackend()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)

	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	return cfg, mb, cache, footerCache, km
}

// TestPutObjectWithPrefix verifies that objects are stored with the prefix
// but can be retrieved using the original key.
func TestPutObjectWithPrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "kalshi-tape/")
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Upload object with client key (no prefix)
	plaintext := []byte("Test content for prefix test")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/data/file.txt", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Verify object is stored in backend WITH prefix
	mb.mu.Lock()
	_, storedWithPrefix := mb.objects["test-bucket/kalshi-tape/data/file.txt"]
	_, storedWithoutPrefix := mb.objects["test-bucket/data/file.txt"]
	mb.mu.Unlock()

	if !storedWithPrefix {
		t.Error("object should be stored with prefix in backend")
	}
	if storedWithoutPrefix {
		t.Error("object should NOT be stored without prefix in backend")
	}

	// Retrieve object using client key (no prefix)
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/data/file.txt", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("retrieved content does not match original")
	}
}

// TestListObjectsV2WithPrefix verifies that ListObjectsV2 works correctly
// when ARMOR_PREFIX is configured.
func TestListObjectsV2WithPrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "prod-env/")
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Upload multiple objects in different directories
	objects := map[string]string{
		"data/file1.txt":           "Content 1",
		"data/file2.txt":           "Content 2",
		"logs/2024/01.log":         "Log content",
		"config/settings.json":     "{}",
		"nested/deep/path/file.txt": "Deep file",
	}

	for key, content := range objects {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/test-bucket/%s", key), bytes.NewReader([]byte(content)))
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("failed to upload %s: status %d", key, w.Code)
		}
	}

	// Test 1: List all objects (no prefix filter)
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListObjectsV2 failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var result struct {
		Contents []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
	}

	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse XML: %v", err)
	}

	if len(result.Contents) != len(objects) {
		t.Errorf("expected %d objects, got %d", len(objects), len(result.Contents))
	}

	// Verify all keys are returned WITHOUT prefix
	for _, obj := range result.Contents {
		if _, exists := objects[obj.Key]; !exists {
			t.Errorf("unexpected key in listing: %s", obj.Key)
		}
		// Keys should NOT have the prefix
		if hasPrefix(obj.Key, "prod-env/") {
			t.Errorf("returned key should not have prefix: %s", obj.Key)
		}
	}

	// Test 2: List with prefix filter
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=data/", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListObjectsV2 with prefix failed: status %d", w.Code)
	}

	var prefixResult struct {
		Contents []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
	}

	if err := xml.Unmarshal(w.Body.Bytes(), &prefixResult); err != nil {
		t.Fatalf("failed to parse XML: %v", err)
	}

	// Should only return data/ files
	if len(prefixResult.Contents) != 2 {
		t.Errorf("expected 2 objects with data/ prefix, got %d", len(prefixResult.Contents))
	}

	for _, obj := range prefixResult.Contents {
		if obj.Key != "data/file1.txt" && obj.Key != "data/file2.txt" {
			t.Errorf("unexpected key in data/ listing: %s", obj.Key)
		}
	}
}

// TestListObjectsV2WithDelimiterAndPrefix tests that common prefixes (directories)
// are correctly stripped when using delimiter with prefix filtering.
func TestListObjectsV2WithDelimiterAndPrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "tenant-1/")
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Upload objects in a directory structure
	objects := map[string]string{
		"data/2024/01.parquet": "data 1",
		"data/2024/02.parquet": "data 2",
		"data/2025/01.parquet": "data 3",
		"logs/app.log":         "log",
		"config/app.json":      "config",
	}

	for key, content := range objects {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/test-bucket/%s", key), bytes.NewReader([]byte(content)))
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("failed to upload %s: status %d", key, w.Code)
		}
	}

	// List with delimiter to group by directory
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&delimiter=/&prefix=data/", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListObjectsV2 with delimiter failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var result struct {
		Contents []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
		CommonPrefixes []struct {
			Prefix string `xml:"Prefix"`
		} `xml:"CommonPrefixes"`
	}

	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse XML: %v", err)
	}

	// Should have common prefixes for subdirectories
	// Common prefixes should NOT have the ARMOR_PREFIX
	expectedCommonPrefixes := []string{"data/2024/", "data/2025/"}
	if len(result.CommonPrefixes) != len(expectedCommonPrefixes) {
		t.Errorf("expected %d common prefixes, got %d", len(expectedCommonPrefixes), len(result.CommonPrefixes))
	}

	for i, cp := range result.CommonPrefixes {
		if cp.Prefix != expectedCommonPrefixes[i] {
			t.Errorf("common prefix %d: expected %s, got %s", i, expectedCommonPrefixes[i], cp.Prefix)
		}
		// Common prefix should NOT contain tenant-1/
		if hasPrefix(cp.Prefix, "tenant-1/") {
			t.Errorf("common prefix should not contain ARMOR_PREFIX: %s", cp.Prefix)
		}
	}
}

// TestDeleteObjectWithPrefix verifies that delete operations work correctly
// with prefix configured.
func TestDeleteObjectWithPrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "logs/")
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Upload object
	content := []byte("log content to delete")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/app/2024-01-01.log", bytes.NewReader(content))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Verify object exists in backend with prefix
	mb.mu.Lock()
	_, exists := mb.objects["test-bucket/logs/app/2024-01-01.log"]
	mb.mu.Unlock()

	if !exists {
		t.Error("object should exist in backend with prefix before delete")
	}

	// Delete object
	req = httptest.NewRequest(http.MethodDelete, "/test-bucket/app/2024-01-01.log", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE failed: status %d", w.Code)
	}

	// Verify object is deleted from backend
	mb.mu.Lock()
	_, existsAfter := mb.objects["test-bucket/logs/app/2024-01-01.log"]
	mb.mu.Unlock()

	if existsAfter {
		t.Error("object should be deleted from backend")
	}

	// Verify GET fails
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/app/2024-01-01.log", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w.Code)
	}
}

// TestHeadObjectWithPrefix verifies that HEAD operations work correctly
// with prefix configured.
func TestHeadObjectWithPrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "cache/")
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	plaintext := []byte("cached content")

	// Upload object
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/key/value", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// HEAD the object
	req = httptest.NewRequest(http.MethodHead, "/test-bucket/key/value", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HEAD failed: status %d", w.Code)
	}

	// Verify Content-Length is plaintext size
	expectedLength := fmt.Sprintf("%d", len(plaintext))
	if w.Header().Get("Content-Length") != expectedLength {
		t.Errorf("expected Content-Length %s, got %s", expectedLength, w.Header().Get("Content-Length"))
	}

	// Verify Content-Type
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	// Body should be empty for HEAD
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body for HEAD, got %d bytes", w.Body.Len())
	}
}

// TestCopyObjectWithPrefix verifies that CopyObject works correctly
// when ARMOR_PREFIX is configured.
func TestCopyObjectWithPrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "copies/")
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Upload source object
	srcContent := []byte("source content for copy")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/original/source.txt", bytes.NewReader(srcContent))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT source failed: status %d", w.Code)
	}

	// Verify source is stored with prefix
	mb.mu.Lock()
	_, srcExists := mb.objects["test-bucket/copies/original/source.txt"]
	mb.mu.Unlock()

	if !srcExists {
		t.Error("source should be stored with prefix")
	}

	// Copy the object - the copy source path is also relative to the prefix
	// The handler should strip the prefix from the copy-source path before looking it up
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/copied/dest.txt", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/original/source.txt")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("COPY failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Verify destination is stored with prefix
	mb.mu.Lock()
	_, destExists := mb.objects["test-bucket/copies/copied/dest.txt"]
	mb.mu.Unlock()

	if !destExists {
		t.Error("destination should be stored with prefix")
	}

	// Get the destination file and verify content
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/copied/dest.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET destination failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), srcContent) {
		t.Error("copied content does not match source")
	}
}

// TestNestedPrefixWithMultipleLevels tests deeply nested prefixes.
func TestNestedPrefixWithMultipleLevels(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "env/prod/tenant/app/")
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Upload object with nested path
	plaintext := []byte("nested content")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/data/year=2024/month=06/file.parquet", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Verify object is stored with full prefix
	mb.mu.Lock()
	_, exists := mb.objects["test-bucket/env/prod/tenant/app/data/year=2024/month=06/file.parquet"]
	mb.mu.Unlock()

	if !exists {
		t.Error("object should be stored with full nested prefix")
	}

	// Retrieve object
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/data/year=2024/month=06/file.parquet", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("retrieved content does not match original")
	}
}

// TestRangeRequestsWithPrefix verifies that range requests work correctly
// with prefix configured.
func TestRangeRequestsWithPrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "range-test/")
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Create content larger than one block
	plaintext := make([]byte, 200000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/large-file.bin", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Request a range
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/large-file.bin", nil)
	req.Header.Set("Range", "bytes=1000-1999")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d", w.Code)
	}

	expectedRange := plaintext[1000:2000]
	if !bytes.Equal(w.Body.Bytes(), expectedRange) {
		t.Error("range content mismatch")
	}

	// Verify Content-Range header
	contentRange := w.Header().Get("Content-Range")
	if contentRange == "" {
		t.Error("expected Content-Range header")
	}
}

// TestEmptyPrefix tests that empty prefix works correctly (no-op behavior).
func TestEmptyPrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "")
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Upload object
	plaintext := []byte("no prefix content")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/data/file.txt", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Verify object is stored WITHOUT prefix (empty prefix means no prefix)
	mb.mu.Lock()
	_, withPrefix := mb.objects["test-bucket/data/file.txt"]
	_, withEmptyPrefix := mb.objects["test-bucket/data/file.txt"] // same check
	mb.mu.Unlock()

	if !withPrefix && !withEmptyPrefix {
		t.Error("object should be stored without prefix when prefix is empty")
	}

	// Retrieve object
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/data/file.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("retrieved content does not match original")
	}
}

// TestPrefixNormalizationIntegration tests that the prefix normalization
// from config.Load() works correctly in integration.
func TestPrefixNormalizationIntegration(t *testing.T) {
	// Create config with various prefix formats and verify they normalize correctly
	testCases := []struct {
		name     string
		prefix   string
		expected string
	}{
		{
			name:     "simple prefix without slash",
			prefix:   "kalshi-tape",
			expected: "kalshi-tape/",
		},
		{
			name:     "prefix with trailing slash",
			prefix:   "kalshi-tape/",
			expected: "kalshi-tape/",
		},
		{
			name:     "prefix with leading slash",
			prefix:   "/kalshi-tape",
			expected: "kalshi-tape/",
		},
		{
			name:     "nested path",
			prefix:   "env/prod/data",
			expected: "env/prod/data/",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, tc.prefix)
			h := handlers.New(cfg, mb, cache, footerCache, km, nil)

			// Upload a test object
			plaintext := []byte("test")
			req := httptest.NewRequest(http.MethodPut, "/test-bucket/file.txt", bytes.NewReader(plaintext))
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("PUT failed: status %d", w.Code)
			}

			// Verify object is stored with normalized prefix
			expectedBackendKey := fmt.Sprintf("test-bucket/%sfile.txt", tc.expected)
			mb.mu.Lock()
			_, exists := mb.objects[expectedBackendKey]
			mb.mu.Unlock()

			if !exists {
				t.Errorf("object should be stored with normalized prefix %s, backend key: %s", tc.expected, expectedBackendKey)
			}

			// Verify object can be retrieved
			req = httptest.NewRequest(http.MethodGet, "/test-bucket/file.txt", nil)
			w = httptest.NewRecorder()
			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("GET failed: status %d", w.Code)
			}
		})
	}
}

// TestSpecialCharactersInKeyWithPrefix tests keys with special characters
// work correctly with prefix.
func TestSpecialCharactersInKeyWithPrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "special/")
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	specialKeys := []struct {
		name         string
		key          string
		encodedKey   string
	}{
		{
			name:       "file with spaces",
			key:        "file with spaces.txt",
			encodedKey: "file%20with%20spaces.txt",
		},
		{
			name:       "file with dashes",
			key:        "file-with-dashes.txt",
			encodedKey: "file-with-dashes.txt",
		},
		{
			name:       "file with underscores",
			key:        "file_with_underscores.txt",
			encodedKey: "file_with_underscores.txt",
		},
		{
			name:       "file with dots",
			key:        "file.with.dots.txt",
			encodedKey: "file.with.dots.txt",
		},
		{
			name:       "path with spaces",
			key:        "path/to/file with multiple spaces.txt",
			encodedKey: "path/to/file%20with%20multiple%20spaces.txt",
		},
	}

	for _, tc := range specialKeys {
		t.Run(tc.name, func(t *testing.T) {
			plaintext := []byte("test content")
			// PUT with URL-encoded key
			req := httptest.NewRequest(http.MethodPut, "/test-bucket/"+tc.encodedKey, bytes.NewReader(plaintext))
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("PUT failed for key %s: status %d", tc.key, w.Code)
			}

			// Retrieve with URL-encoded key and verify
			req = httptest.NewRequest(http.MethodGet, "/test-bucket/"+tc.encodedKey, nil)
			w = httptest.NewRecorder()
			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("GET failed for key %s: status %d", tc.key, w.Code)
			}

			if !bytes.Equal(w.Body.Bytes(), plaintext) {
				t.Errorf("content mismatch for key %s", tc.key)
			}
		})
	}
}

// hasPrefix is a helper to check if a string has a prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// normalizeTestPrefix normalizes a prefix string to match config.Load() behavior.
// This replicates the logic from internal/config/config.go:normalizePrefix.
func normalizeTestPrefix(prefix string) string {
	if prefix == "" {
		return ""
	}

	// Remove leading slashes
	prefix = strings.TrimLeft(prefix, "/")

	// Remove all trailing slashes first
	prefix = strings.TrimRight(prefix, "/")

	// Add exactly one trailing slash if non-empty
	if prefix != "" {
		prefix += "/"
	}

	return prefix
}
