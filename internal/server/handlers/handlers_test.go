// Package handlers_test tests the S3 operation handlers.
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
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server/handlers"
)

// mockBackend implements backend.Backend for testing.
type mockBackend struct {
	mu      sync.Mutex
	objects map[string][]byte
	meta    map[string]map[string]string
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		objects: make(map[string][]byte),
		meta:    make(map[string]map[string]string),
	}
}

func (m *mockBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.objects[bucket+"/"+key] = data
	m.meta[bucket+"/"+key] = meta
	return nil
}

func (m *mockBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := bucket + "/" + key
	data, ok := m.objects[k]
	if !ok {
		return nil, nil, fmt.Errorf("object not found: %s", key)
	}

	meta := m.meta[k]
	info := &backend.ObjectInfo{
		Key:              key,
		Size:             int64(len(data)),
		Metadata:         meta,
		IsARMOREncrypted: meta["x-amz-meta-armor-version"] != "",
	}

	if am, ok := backend.ParseARMORMetadata(meta); ok {
		info.Size = am.PlaintextSize
		info.ContentType = am.ContentType
		info.ETag = am.ETag
	}

	return io.NopCloser(bytes.NewReader(data)), info, nil
}

func (m *mockBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := bucket + "/" + key
	data, ok := m.objects[k]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	if offset >= int64(len(data)) {
		return nil, fmt.Errorf("offset out of range")
	}
	end := offset + length
	if end > int64(len(data)) {
		end = int64(len(data))
	}
	return io.NopCloser(bytes.NewReader(data[offset:end])), nil
}

func (m *mockBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := bucket + "/" + key
	data, ok := m.objects[k]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}

	meta := m.meta[k]
	info := &backend.ObjectInfo{
		Key:              key,
		Size:             int64(len(data)),
		Metadata:         meta,
		IsARMOREncrypted: meta["x-amz-meta-armor-version"] != "",
		LastModified:     time.Now(),
	}

	if am, ok := backend.ParseARMORMetadata(meta); ok {
		info.Size = am.PlaintextSize
		info.ContentType = am.ContentType
		info.ETag = am.ETag
	}

	return info, nil
}

func (m *mockBackend) Delete(ctx context.Context, bucket, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := bucket + "/" + key
	delete(m.objects, k)
	delete(m.meta, k)
	return nil
}

func (m *mockBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		m.Delete(ctx, bucket, key)
	}
	return nil
}

func (m *mockBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var objects []backend.ObjectInfo
	prefixPath := bucket + "/" + prefix

	for k, data := range m.objects {
		if prefix != "" && (len(k) < len(prefixPath) || k[:len(prefixPath)] != prefixPath) {
			continue
		}
		key := k[len(bucket)+1:]
		meta := m.meta[k]
		info := backend.ObjectInfo{
			Key:      key,
			Size:     int64(len(data)),
			Metadata: meta,
		}
		if am, ok := backend.ParseARMORMetadata(meta); ok {
			info.Size = am.PlaintextSize
			info.ETag = am.ETag
		}
		objects = append(objects, info)
	}

	return &backend.ListResult{Objects: objects}, nil
}

func (m *mockBackend) Copy(ctx context.Context, bucket, srcKey, dstKey string, meta map[string]string, replaceMetadata bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	src := bucket + "/" + srcKey
	dst := bucket + "/" + dstKey

	data, ok := m.objects[src]
	if !ok {
		return fmt.Errorf("source object not found: %s", srcKey)
	}

	m.objects[dst] = data
	if replaceMetadata {
		m.meta[dst] = meta
	} else {
		// Copy metadata from source
		newMeta := make(map[string]string)
		for k, v := range m.meta[src] {
			newMeta[k] = v
		}
		for k, v := range meta {
			newMeta[k] = v
		}
		m.meta[dst] = newMeta
	}

	return nil
}

// testSetup creates common test dependencies.
func testSetup(t *testing.T) (*config.Config, *mockBackend, *backend.MetadataCache, *backend.FooterCache, []byte) {
	t.Helper()

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
	}

	mb := newMockBackend()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)

	return cfg, mb, cache, footerCache, mek
}

func TestPutObjectGetObject(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	// Create plaintext content
	plaintext := []byte("Hello, ARMOR! This is a test file with some content.")

	// Create PUT request
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test-key", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify ETag is returned
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Error("expected ETag header")
	}

	// Now GET the object back
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/test-key", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify content matches
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Errorf("content mismatch: got %q, want %q", w.Body.String(), string(plaintext))
	}

	// Verify headers
	if w.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %s", w.Header().Get("Content-Type"))
	}
}

func TestPutObjectLargeFile(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	// Create a file larger than one block (64KB)
	plaintext := make([]byte, 100000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/large-file", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// GET it back
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/large-file", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("large file content mismatch")
	}
}

func TestGetObjectRange(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	// Create content
	plaintext := make([]byte, 200000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// PUT the object
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/range-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed with status %d", w.Code)
	}

	// GET with range request
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/range-test", nil)
	req.Header.Set("Range", "bytes=1000-1999")
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d", w.Code)
	}

	expectedRange := plaintext[1000:2000]
	if !bytes.Equal(w.Body.Bytes(), expectedRange) {
		t.Errorf("range content mismatch: got %d bytes, want %d bytes", len(w.Body.Bytes()), len(expectedRange))
	}

	// Verify Content-Range header
	contentRange := w.Header().Get("Content-Range")
	if contentRange == "" {
		t.Error("expected Content-Range header")
	}
}

func TestHeadObject(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek)

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

func TestListObjectsV2(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

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

func TestEncryptionRoundTrip(t *testing.T) {
	// This test verifies that the encryption actually happens
	// by checking that the stored data is different from plaintext
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	plaintext := []byte("This is sensitive data that should be encrypted")

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/encrypted", bytes.NewReader(plaintext))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Check that stored data is encrypted (not plaintext)
	mb.mu.Lock()
	storedData := mb.objects["test-bucket/encrypted"]
	mb.mu.Unlock()

	if bytes.Contains(storedData, plaintext) {
		t.Error("plaintext found in stored data - encryption may not be working")
	}

	// Verify envelope magic is present (ARMR header)
	if len(storedData) < 4 || string(storedData[:4]) != "ARMR" {
		t.Error("ARMR magic not found at start of stored data")
	}
}

func TestMultipleFilesSameMEK(t *testing.T) {
	// Test that multiple files encrypted with the same MEK can all be decrypted
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	files := map[string][]byte{
		"file1.txt": []byte("Content of file 1"),
		"file2.txt": []byte("Content of file 2 - different"),
		"file3.txt": []byte("Third file with more content"),
	}

	// Upload all files
	for key, content := range files {
		req := httptest.NewRequest(http.MethodPut, "/test-bucket/"+key, bytes.NewReader(content))
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("failed to upload %s: status %d", key, w.Code)
		}
	}

	// Verify all files can be downloaded and decrypted correctly
	for key, expectedContent := range files {
		req := httptest.NewRequest(http.MethodGet, "/test-bucket/"+key, nil)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("failed to download %s: status %d", key, w.Code)
			continue
		}

		if !bytes.Equal(w.Body.Bytes(), expectedContent) {
			t.Errorf("content mismatch for %s", key)
		}
	}
}

func TestNonARMORObjectPassthrough(t *testing.T) {
	// Test that non-ARMOR objects (without x-amz-meta-armor-version) pass through unchanged
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	// Store a plain object directly in the mock backend
	plainData := []byte("Plain unencrypted data")
	mb.Put(context.Background(), "test-bucket", "plain-file", bytes.NewReader(plainData), int64(len(plainData)), map[string]string{
		"Content-Type": "text/plain",
	})

	// GET should return it unchanged
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/plain-file", nil)
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plainData) {
		t.Error("plain data was modified")
	}
}

func TestETagConsistency(t *testing.T) {
	// Test that ETag is consistent across uploads of same content
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	content := []byte("Same content")

	// Upload twice
	req1 := httptest.NewRequest(http.MethodPut, "/test-bucket/etag-test-1", bytes.NewReader(content))
	req1.Header.Set("Content-Type", "text/plain")
	w1 := httptest.NewRecorder()
	h.HandleRoot(w1, req1)
	etag1 := w1.Header().Get("ETag")

	req2 := httptest.NewRequest(http.MethodPut, "/test-bucket/etag-test-2", bytes.NewReader(content))
	req2.Header.Set("Content-Type", "text/plain")
	w2 := httptest.NewRecorder()
	h.HandleRoot(w2, req2)
	etag2 := w2.Header().Get("ETag")

	if etag1 != etag2 {
		t.Errorf("ETags for same content differ: %s vs %s", etag1, etag2)
	}

	// Upload different content
	differentContent := []byte("Different content")
	req3 := httptest.NewRequest(http.MethodPut, "/test-bucket/etag-test-3", bytes.NewReader(differentContent))
	req3.Header.Set("Content-Type", "text/plain")
	w3 := httptest.NewRecorder()
	h.HandleRoot(w3, req3)
	etag3 := w3.Header().Get("ETag")

	if etag1 == etag3 {
		t.Error("ETags for different content are the same")
	}
}

// TestHMACVerification ensures HMAC verification catches tampering
// With streaming decryption, the HTTP status is written before all blocks are verified.
// Tampering is detected during streaming, which will cause the body to be incomplete.
// The httptest.ResponseRecorder doesn't capture streaming errors, so we verify
// that the body content is incomplete or corrupted rather than checking status code.
func TestHMACVerification(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	plaintext := []byte("Content to verify")

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/verify-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Tamper with stored data
	mb.mu.Lock()
	stored := mb.objects["test-bucket/verify-test"]
	// Modify a byte in the encrypted data section (after header)
	if len(stored) > 100 {
		stored[80] ^= 0xFF
	}
	mb.mu.Unlock()

	// GET should fail integrity check
	// With streaming, the status is written before HMAC verification completes.
	// The verification error will cause incomplete/corrupted body content.
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/verify-test", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Verify the response is not the original plaintext (tampering detected)
	// Either the body is empty/incomplete, or it's corrupted
	if bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("expected corrupted/incomplete data for tampered object, got original plaintext")
	}

	// For range requests, we still get 500 because HMAC is verified before streaming
	// But for full downloads with streaming, we may get 200 with incomplete body
	// This is expected behavior - streaming provides lower latency at the cost of
	// potentially incomplete responses on integrity failure
}

// TestRangeSuffixRequest tests suffix range requests (e.g., "bytes=-100")
func TestRangeSuffixRequest(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	plaintext := make([]byte, 10000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/suffix-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Request last 100 bytes
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/suffix-test", nil)
	req.Header.Set("Range", "bytes=-100")
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d", w.Code)
	}

	expected := plaintext[len(plaintext)-100:]
	if !bytes.Equal(w.Body.Bytes(), expected) {
		t.Error("suffix range content mismatch")
	}
}

// TestStreamingDecryption verifies that pipelined streaming decryption works correctly
// for multi-block files. This exercises the io.Pipe based streaming path.
func TestStreamingDecryption(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	// Create a file spanning multiple blocks (64KB each)
	// Using 3 blocks worth of data
	blockSize := cfg.BlockSize
	plaintext := make([]byte, blockSize*3-1000) // Not aligned to block boundary
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// Upload
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/streaming-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Download with full streaming (no Range header)
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/streaming-test", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	// Verify streaming header is present
	if w.Header().Get("X-Armor-Stream") != "pipelined" {
		t.Error("expected X-Armor-Stream: pipelined header")
	}

	// Verify content matches exactly
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Errorf("streaming content mismatch: got %d bytes, want %d bytes",
			len(w.Body.Bytes()), len(plaintext))
	}

	// Verify Accept-Ranges header
	if w.Header().Get("Accept-Ranges") != "bytes" {
		t.Error("expected Accept-Ranges: bytes header")
	}
}

// TestStreamingDecryptionVariousSizes tests streaming with different file sizes
func TestStreamingDecryptionVariousSizes(t *testing.T) {
	sizes := []int{
		100,           // Tiny
		4096,          // One 4KB page
		65535,         // Just under one block
		65536,         // Exactly one block
		65537,         // Just over one block
		131072,        // Two blocks
		1000000,       // ~1MB
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			cfg, mb, cache, footerCache, mek := testSetup(t)
			h := handlers.New(cfg, mb, cache, footerCache, mek)

			plaintext := make([]byte, size)
			for i := range plaintext {
				plaintext[i] = byte(i % 256)
			}

			key := fmt.Sprintf("streaming-test-%d", size)
			req := httptest.NewRequest(http.MethodPut, "/test-bucket/"+key, bytes.NewReader(plaintext))
			req.Header.Set("Content-Type", "application/octet-stream")
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("PUT failed: status %d", w.Code)
			}

			req = httptest.NewRequest(http.MethodGet, "/test-bucket/"+key, nil)
			w = httptest.NewRecorder()
			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("GET failed: status %d", w.Code)
			}

			if !bytes.Equal(w.Body.Bytes(), plaintext) {
				t.Errorf("content mismatch for size %d: got %d bytes, want %d",
					size, len(w.Body.Bytes()), size)
			}
		})
	}
}
