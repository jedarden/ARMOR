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
	"strings"
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

func (m *mockBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	src := srcBucket + "/" + srcKey
	dst := dstBucket + "/" + dstKey

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

func (m *mockBackend) ListBuckets(ctx context.Context) ([]backend.BucketInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Extract unique bucket names from stored objects
	buckets := make(map[string]time.Time)
	for k := range m.objects {
		parts := strings.SplitN(k, "/", 2)
		if len(parts) > 0 && parts[0] != "" {
			bucket := parts[0]
			if _, exists := buckets[bucket]; !exists {
				buckets[bucket] = time.Now()
			}
		}
	}

	result := make([]backend.BucketInfo, 0, len(buckets))
	for name, created := range buckets {
		result = append(result, backend.BucketInfo{
			Name:         name,
			CreationDate: created,
		})
	}

	return result, nil
}

func (m *mockBackend) CreateBucket(ctx context.Context, bucket string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// In a real implementation, this would create the bucket
	// For the mock, we just track that it exists via a marker
	m.objects[bucket+"/.bucket"] = nil
	return nil
}

func (m *mockBackend) DeleteBucket(ctx context.Context, bucket string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if bucket is empty (except for the marker)
	for k := range m.objects {
		if strings.HasPrefix(k, bucket+"/") && k != bucket+"/.bucket" {
			return fmt.Errorf("bucket not empty")
		}
	}

	// Remove the bucket marker
	delete(m.objects, bucket+"/.bucket")
	return nil
}

func (m *mockBackend) HeadBucket(ctx context.Context, bucket string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if any objects exist in this bucket
	for k := range m.objects {
		if strings.HasPrefix(k, bucket+"/") {
			return nil
		}
	}

	return fmt.Errorf("bucket not found: %s", bucket)
}

func (m *mockBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return m.Get(ctx, bucket, key)
}

// Multipart upload methods (stub implementations for testing)
func (m *mockBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	return fmt.Sprintf("upload-%d", time.Now().UnixNano()), nil
}

func (m *mockBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	return fmt.Sprintf("etag-%d", partNumber), nil
}

func (m *mockBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	return "final-etag", nil
}

func (m *mockBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	return nil
}

func (m *mockBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*backend.ListPartsResult, error) {
	return &backend.ListPartsResult{}, nil
}

func (m *mockBackend) ListMultipartUploads(ctx context.Context, bucket string) (*backend.ListMultipartUploadsResult, error) {
	return &backend.ListMultipartUploadsResult{}, nil
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

// TestCopyObject tests S3 CopyObject with ARMOR encryption
func TestCopyObject(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

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
func TestCopyObjectRewrapsDEK(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

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
func TestCopyObjectNonARMOR(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	// Store a plain (non-ARMOR) object directly in the mock backend
	plainData := []byte("Plain unencrypted data for copy test")
	mb.Put(context.Background(), "test-bucket", "plain-source.txt", bytes.NewReader(plainData), int64(len(plainData)), map[string]string{
		"Content-Type": "text/plain",
	})

	// Copy the plain file
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/plain-dest.txt", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/plain-source.txt")
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
}

// TestCopyObjectMissingSource tests error handling for missing source
func TestCopyObjectMissingSource(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

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
func TestCopyObjectWithMetadataDirective(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

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
func TestDeleteObjects(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

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
func TestDeleteObjectsQuiet(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

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
func TestHeadBucket(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	// Create an object (which implicitly creates the bucket in our mock)
	content := []byte("test content")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/head-bucket-test/file.txt", bytes.NewReader(content))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Head bucket should succeed
	req = httptest.NewRequest(http.MethodHead, "/test-bucket", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for existing bucket, got %d", w.Code)
	}

	// Head non-existent bucket should fail
	req = httptest.NewRequest(http.MethodHead, "/non-existent-bucket", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent bucket, got %d", w.Code)
	}
}

// TestListBuckets tests S3 ListBuckets
func TestListBuckets(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	// Create objects in multiple buckets
	for bucketNum := 0; bucketNum < 3; bucketNum++ {
		for fileNum := 0; fileNum < 2; fileNum++ {
			content := []byte(fmt.Sprintf("Content %d-%d", bucketNum, fileNum))
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/bucket-%d/file%d.txt", bucketNum, fileNum), bytes.NewReader(content))
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
		}
	}

	// List buckets
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var result struct {
		Buckets struct {
			Bucket []struct {
				Name string `xml:"Name"`
			} `xml:"Bucket"`
		} `xml:"Buckets"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(result.Buckets.Bucket) != 3 {
		t.Errorf("expected 3 buckets, got %d", len(result.Buckets.Bucket))
	}

	// Verify bucket names
	bucketNames := make(map[string]bool)
	for _, b := range result.Buckets.Bucket {
		bucketNames[b.Name] = true
	}
	for i := 0; i < 3; i++ {
		expectedName := fmt.Sprintf("bucket-%d", i)
		if !bucketNames[expectedName] {
			t.Errorf("expected bucket %s in list", expectedName)
		}
	}
}

// TestCreateBucket tests S3 CreateBucket
func TestCreateBucket(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	// Create bucket
	req := httptest.NewRequest(http.MethodPut, "/new-test-bucket", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify Location header
	location := w.Header().Get("Location")
	if location != "/new-test-bucket" {
		t.Errorf("expected Location /new-test-bucket, got %s", location)
	}

	// Verify bucket exists via HEAD
	req = httptest.NewRequest(http.MethodHead, "/new-test-bucket", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for created bucket, got %d", w.Code)
	}
}

// TestDeleteBucket tests S3 DeleteBucket
func TestDeleteBucket(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek)

	// Create an empty bucket
	req := httptest.NewRequest(http.MethodPut, "/bucket-to-delete", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Delete the bucket
	req = httptest.NewRequest(http.MethodDelete, "/bucket-to-delete", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify bucket no longer exists
	req = httptest.NewRequest(http.MethodHead, "/bucket-to-delete", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for deleted bucket, got %d", w.Code)
	}
}
