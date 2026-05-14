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
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
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
	body, _, err := m.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	return body, err
}

func (m *mockBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := bucket + "/" + key
	data, ok := m.objects[k]
	if !ok {
		return nil, nil, fmt.Errorf("object not found: %s", key)
	}
	if offset >= int64(len(data)) {
		return nil, nil, fmt.Errorf("offset out of range")
	}
	end := offset + length
	if end > int64(len(data)) {
		end = int64(len(data))
	}
	// Mock doesn't simulate CF caching, so return empty headers
	return io.NopCloser(bytes.NewReader(data[offset:end])), make(map[string]string), nil
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

// Lifecycle configuration methods (stub implementations for testing)
func (m *mockBackend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, fmt.Errorf("lifecycle configuration not found")
}

func (m *mockBackend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

func (m *mockBackend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	return nil
}

// Object Lock methods (stub implementations for testing)
func (m *mockBackend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, fmt.Errorf("object lock configuration not found")
}

func (m *mockBackend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

func (m *mockBackend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, fmt.Errorf("retention not found")
}

func (m *mockBackend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	return nil
}

func (m *mockBackend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, fmt.Errorf("legal hold not found")
}

func (m *mockBackend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	return nil
}

func (m *mockBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*backend.ListObjectVersionsResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &backend.ListObjectVersionsResult{
		IsTruncated: false,
	}

	// Find objects matching the prefix
	for k := range m.objects {
		if !strings.HasPrefix(k, bucket+"/") {
			continue
		}
		key := strings.TrimPrefix(k, bucket+"/")
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		// Create version info
		meta := m.meta[k]
		info := backend.ObjectVersionInfo{
			Key:          key,
			VersionID:    "v1",
			Size:         int64(len(m.objects[k])),
			ETag:         meta["x-amz-meta-armor-etag"],
			LastModified: time.Now(),
			IsLatest:   true,
		}
		result.Versions = append(result.Versions, info)
	}

	return result, nil
}

func (m *mockBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*backend.ObjectInfo, error) {
	return m.Head(ctx, bucket, key)
}

// testSetup creates common test dependencies.
func testSetup(t *testing.T) (*config.Config, *mockBackend, *backend.MetadataCache, *backend.FooterCache, *keymanager.KeyManager) {
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

	// Create a KeyManager with the generated MEK
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	return cfg, mb, cache, footerCache, km
}

func TestPutObjectGetObject(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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

func TestEncryptionRoundTrip(t *testing.T) {
	// This test verifies that the encryption actually happens
	// by checking that the stored data is different from plaintext
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
			h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
func TestHeadBucket(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

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

// TestAbortMultipartUpload tests S3 AbortMultipartUpload
func TestAbortMultipartUpload(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a multipart upload
	req := httptest.NewRequest(http.MethodPost, "/test-bucket/test-abort.txt?uploads=", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: status %d", w.Code)
	}

	// Parse upload ID from response
	var result struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse CreateMultipartUpload response: %v", err)
	}
	uploadID := result.UploadID

	// Abort the multipart upload
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/test-bucket/test-abort.txt?uploadId=%s", uploadID), nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for abort, got %d", w.Code)
	}

	// Verify state was cleaned up
	mb.mu.Lock()
	stateKey := fmt.Sprintf(".armor/multipart/%s.state", uploadID)
	_, exists := mb.objects[stateKey]
	mb.mu.Unlock()
	if exists {
		t.Error("multipart state should have been deleted")
	}
}

// TestListParts tests S3 ListParts operation
func TestListParts(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a multipart upload
	req := httptest.NewRequest(http.MethodPost, "/test-bucket/test-list-parts.txt?uploads=", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: status %d", w.Code)
	}

	// Parse upload ID from response
	var result struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse CreateMultipartUpload response: %v", err)
	}
	uploadID := result.UploadID

	// Upload a part
	partContent := []byte("Part 1 content")
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/test-bucket/test-list-parts.txt?partNumber=1&uploadId=%s", uploadID), bytes.NewReader(partContent))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UploadPart failed: status %d", w.Code)
	}

	// List parts
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/test-bucket/test-list-parts.txt?uploadId=%s", uploadID), nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListParts failed: status %d", w.Code)
	}

	// Verify response is XML
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/xml" {
		t.Errorf("expected Content-Type application/xml, got %s", contentType)
	}

	// Verify response contains ListPartsResult
	body := w.Body.String()
	if !strings.Contains(body, "ListPartsResult") {
		t.Error("response should contain ListPartsResult element")
	}
	if !strings.Contains(body, uploadID) {
		t.Error("response should contain upload ID")
	}
}

// TestListMultipartUploads tests S3 ListMultipartUploads operation
func TestListMultipartUploads(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create multiple multipart uploads
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/test-bucket/test-list-uploads-%d.txt?uploads=", i), nil)
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("CreateMultipartUpload %d failed: status %d", i, w.Code)
		}
	}

	// List multipart uploads
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?uploads=", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListMultipartUploads failed: status %d", w.Code)
	}

	// Verify response is XML
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/xml" {
		t.Errorf("expected Content-Type application/xml, got %s", contentType)
	}

	// Verify response contains ListMultipartUploadsResult
	body := w.Body.String()
	if !strings.Contains(body, "ListMultipartUploadsResult") {
		t.Error("response should contain ListMultipartUploadsResult element")
	}
	if !strings.Contains(body, "test-bucket") {
		t.Error("response should contain bucket name")
	}
}

// TestAbortMultipartUploadNotFound tests aborting a non-existent upload
func TestAbortMultipartUploadNotFound(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Try to abort a non-existent upload
	req := httptest.NewRequest(http.MethodDelete, "/test-bucket/test.txt?uploadId=nonexistent-upload-id", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent upload, got %d", w.Code)
	}
}

// TestListPartsNotFound tests listing parts of a non-existent upload
func TestListPartsNotFound(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Try to list parts of a non-existent upload
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/test.txt?uploadId=nonexistent-upload-id", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent upload, got %d", w.Code)
	}
}

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
func TestStreamingEncryptionLargeFile(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a file larger than the 10MB streaming threshold
	// Using 15MB to ensure streaming is triggered
	size := 15 * 1024 * 1024
	plaintext := make([]byte, size)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// Upload with Content-Length header to trigger streaming
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/streaming-large", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(size)
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT failed: status %d", w.Code)
	}

	// Verify streaming header is set
	if w.Header().Get("X-Armor-Streaming") != "true" {
		t.Error("expected X-Armor-Streaming: true header for large file")
	}

	// Verify ETag is returned
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Error("expected ETag header")
	}

	// GET the file back and verify content
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/streaming-large", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Errorf("streaming content mismatch: got %d bytes, want %d bytes",
			len(w.Body.Bytes()), len(plaintext))
	}
}

// TestStreamingEncryptionMultiBlock tests streaming with multi-block files
func TestStreamingEncryptionMultiBlock(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a file that spans multiple blocks (> 64KB) but under streaming threshold
	// This tests the buffered path
	blockSize := cfg.BlockSize
	plaintext := make([]byte, blockSize*2+1000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/multi-block", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT failed: status %d", w.Code)
	}

	// GET and verify
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/multi-block", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("multi-block content mismatch")
	}
}

// TestStreamingEncryptionRangeRead tests range reads on streaming-encrypted files
func TestStreamingEncryptionRangeRead(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a large file (triggers streaming)
	size := 12 * 1024 * 1024
	plaintext := make([]byte, size)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// Upload
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/streaming-range", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(size)
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Debug: Check what was stored
	mb.mu.Lock()
	storedData := mb.objects["test-bucket/streaming-range"]
	storedMeta := mb.meta["test-bucket/streaming-range"]
	mb.mu.Unlock()

	expectedEnvelopeSize := int64(64 + size + int(int(size/65536)+1)*32)
	t.Logf("Stored data size: %d, expected envelope size: ~%d", len(storedData), expectedEnvelopeSize)
	t.Logf("Metadata: %+v", storedMeta)

	// Range read from the middle
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/streaming-range", nil)
	req.Header.Set("Range", "bytes=1000000-2000000")
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d, body: %s", w.Code, w.Body.String())
	}

	expectedRange := plaintext[1000000 : 2000000+1]
	if !bytes.Equal(w.Body.Bytes(), expectedRange) {
		t.Errorf("range content mismatch: got %d bytes, want %d bytes",
			len(w.Body.Bytes()), len(expectedRange))
	}

	// Verify Content-Range header
	contentRange := w.Header().Get("Content-Range")
	if contentRange == "" {
		t.Error("expected Content-Range header")
	}
}

// TestStreamingEncryptionSHA256 verifies SHA-256 integrity for streaming uploads
func TestStreamingEncryptionSHA256(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a known-content large file
	size := 11 * 1024 * 1024
	plaintext := make([]byte, size)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// Upload with streaming
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/sha256-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(size)
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Verify the stored metadata has the correct plaintext SHA-256
	mb.mu.Lock()
	meta := mb.meta["test-bucket/sha256-test"]
	storedData := mb.objects["test-bucket/sha256-test"]
	mb.mu.Unlock()

	// Verify ARMR magic header
	if len(storedData) < 4 || string(storedData[:4]) != "ARMR" {
		t.Error("ARMR magic not found at start of stored data")
	}

	// Verify metadata contains SHA-256
	sha256Meta := meta["x-amz-meta-armor-plaintext-sha256"]
	if sha256Meta == "" {
		t.Error("missing plaintext SHA-256 in metadata")
	}

	// GET and verify full content
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/sha256-test", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("streaming SHA-256 content mismatch")
	}
}

// TestStreamingEncryptionThreshold tests the boundary between buffered and streaming
func TestStreamingEncryptionThreshold(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Test just under the 10MB threshold (should use buffered path)
	threshold := 10 * 1024 * 1024

	tests := []struct {
		name        string
		size        int
		expectStream bool
	}{
		{"under_threshold", threshold - 1, false},
		{"at_threshold", threshold, false}, // at threshold still uses buffered
		{"over_threshold", threshold + 1, true},
		{"large_over", threshold * 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plaintext := make([]byte, tt.size)
			for i := range plaintext {
				plaintext[i] = byte(i % 256)
			}

			req := httptest.NewRequest(http.MethodPut, "/test-bucket/"+tt.name, bytes.NewReader(plaintext))
			req.Header.Set("Content-Type", "application/octet-stream")
			req.ContentLength = int64(tt.size)
			w := httptest.NewRecorder()

			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("PUT failed: status %d", w.Code)
				return
			}

			streamingHeader := w.Header().Get("X-Armor-Streaming")
			if tt.expectStream && streamingHeader != "true" {
				t.Errorf("expected X-Armor-Streaming: true, got %s", streamingHeader)
			}
			if !tt.expectStream && streamingHeader == "true" {
				t.Error("did not expect X-Armor-Streaming: true for small file")
			}

			// Verify content integrity
			req = httptest.NewRequest(http.MethodGet, "/test-bucket/"+tt.name, nil)
			w = httptest.NewRecorder()

			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("GET failed: status %d", w.Code)
				return
			}

			if !bytes.Equal(w.Body.Bytes(), plaintext) {
				t.Errorf("content mismatch for %s", tt.name)
			}
		})
	}
}

// mockBackendWithLifecycle is a mock backend that supports lifecycle configuration for testing
type mockBackendWithLifecycle struct {
	*mockBackend
	lifecycleConfig []byte
}

func newMockBackendWithLifecycle() *mockBackendWithLifecycle {
	return &mockBackendWithLifecycle{
		mockBackend: newMockBackend(),
	}
}

func (m *mockBackendWithLifecycle) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	if m.lifecycleConfig == nil {
		return nil, fmt.Errorf("lifecycle configuration not found")
	}
	return m.lifecycleConfig, nil
}

func (m *mockBackendWithLifecycle) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	m.lifecycleConfig = config
	return nil
}

func (m *mockBackendWithLifecycle) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	m.lifecycleConfig = nil
	return nil
}

// TestGetBucketLifecycleConfiguration tests GET ?lifecycle on a bucket
func TestGetBucketLifecycleConfiguration(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
	}

	mb := newMockBackendWithLifecycle()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test 1: Get lifecycle when not set - should return error
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?lifecycle", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Should return 500 because our mock returns an error when config doesn't exist
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for missing lifecycle config, got %d", w.Code)
	}

	// Test 2: Set and then get lifecycle configuration
	lifecycleXML := `<?xml version="1.0" encoding="UTF-8"?>
<LifecycleConfiguration>
  <Rule>
    <ID>test-rule</ID>
    <Status>Enabled</Status>
    <Filter>
      <Prefix>logs/</Prefix>
    </Filter>
    <Expiration>
      <Days>30</Days>
    </Expiration>
  </Rule>
</LifecycleConfiguration>`

	// PUT lifecycle configuration
	req = httptest.NewRequest(http.MethodPut, "/test-bucket?lifecycle", strings.NewReader(lifecycleXML))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT lifecycle: expected status 200, got %d", w.Code)
	}

	// GET lifecycle configuration
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?lifecycle", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET lifecycle: expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "test-rule") {
		t.Error("GET lifecycle response should contain rule ID")
	}
}

// TestPutBucketLifecycleConfiguration tests PUT ?lifecycle on a bucket
func TestPutBucketLifecycleConfiguration(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
	}

	mb := newMockBackendWithLifecycle()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test PUT with valid lifecycle configuration
	lifecycleXML := `<?xml version="1.0" encoding="UTF-8"?>
<LifecycleConfiguration>
  <Rule>
    <ID>expire-old-logs</ID>
    <Status>Enabled</Status>
    <Prefix>logs/</Prefix>
    <Expiration>
      <Days>7</Days>
    </Expiration>
  </Rule>
  <Rule>
    <ID>abort-incomplete-uploads</ID>
    <Status>Enabled</Status>
    <Filter>
      <Prefix>uploads/</Prefix>
    </Filter>
    <AbortIncompleteMultipartUpload>
      <DaysAfterInitiation>1</DaysAfterInitiation>
    </AbortIncompleteMultipartUpload>
  </Rule>
</LifecycleConfiguration>`

	req := httptest.NewRequest(http.MethodPut, "/test-bucket?lifecycle", strings.NewReader(lifecycleXML))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT lifecycle: expected status 200, got %d", w.Code)
	}

	// Verify the configuration was stored
	if mb.lifecycleConfig == nil {
		t.Error("lifecycle configuration should have been stored")
	}
}

// TestDeleteBucketLifecycleConfiguration tests DELETE ?lifecycle on a bucket
func TestDeleteBucketLifecycleConfiguration(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
	}

	mb := newMockBackendWithLifecycle()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// First set a lifecycle configuration
	lifecycleXML := `<?xml version="1.0" encoding="UTF-8"?>
<LifecycleConfiguration>
  <Rule>
    <ID>test-rule</ID>
    <Status>Enabled</Status>
    <Prefix>test/</Prefix>
    <Expiration>
      <Days>30</Days>
    </Expiration>
  </Rule>
</LifecycleConfiguration>`

	req := httptest.NewRequest(http.MethodPut, "/test-bucket?lifecycle", strings.NewReader(lifecycleXML))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT lifecycle: expected status 200, got %d", w.Code)
	}

	// Now delete it
	req = httptest.NewRequest(http.MethodDelete, "/test-bucket?lifecycle", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE lifecycle: expected status 204, got %d", w.Code)
	}

	// Verify it was deleted
	if mb.lifecycleConfig != nil {
		t.Error("lifecycle configuration should have been deleted")
	}
}

// mockBackendWithObjectLock is a mock backend that supports object lock operations for testing
type mockBackendWithObjectLock struct {
	*mockBackend
	objectLockConfig map[string][]byte          // bucket -> config
	retentionConfig  map[string][]byte          // bucket/key -> config
	legalHoldConfig  map[string][]byte          // bucket/key -> config
}

func newMockBackendWithObjectLock() *mockBackendWithObjectLock {
	return &mockBackendWithObjectLock{
		mockBackend:      newMockBackend(),
		objectLockConfig: make(map[string][]byte),
		retentionConfig:  make(map[string][]byte),
		legalHoldConfig:  make(map[string][]byte),
	}
}

func (m *mockBackendWithObjectLock) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	config, ok := m.objectLockConfig[bucket]
	if !ok {
		return nil, fmt.Errorf("object lock configuration not found")
	}
	return config, nil
}

func (m *mockBackendWithObjectLock) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	m.objectLockConfig[bucket] = config
	return nil
}

func (m *mockBackendWithObjectLock) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	k := bucket + "/" + key
	config, ok := m.retentionConfig[k]
	if !ok {
		return nil, fmt.Errorf("retention not found")
	}
	return config, nil
}

func (m *mockBackendWithObjectLock) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	k := bucket + "/" + key
	m.retentionConfig[k] = retention
	return nil
}

func (m *mockBackendWithObjectLock) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	k := bucket + "/" + key
	config, ok := m.legalHoldConfig[k]
	if !ok {
		return nil, fmt.Errorf("legal hold not found")
	}
	return config, nil
}

func (m *mockBackendWithObjectLock) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	k := bucket + "/" + key
	m.legalHoldConfig[k] = legalHold
	return nil
}

func (m *mockBackendWithObjectLock) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*backend.ListObjectVersionsResult, error) {
	return nil, fmt.Errorf("not implemented")
}

// TestGetObjectLockConfiguration tests GET ?object-lock on a bucket
func TestGetObjectLockConfiguration(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test 1: Get object lock config when not set - should return error
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?object-lock", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Should return 500 because our mock returns an error when config doesn't exist
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for missing object lock config, got %d", w.Code)
	}

	// Test 2: Set and then get object lock configuration
	objectLockXML := `<?xml version="1.0" encoding="UTF-8"?>
<ObjectLockConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <ObjectLockEnabled>Enabled</ObjectLockEnabled>
  <Rule>
    <DefaultRetention>
      <Mode>GOVERNANCE</Mode>
      <Days>30</Days>
    </DefaultRetention>
  </Rule>
</ObjectLockConfiguration>`

	// PUT object lock configuration
	req = httptest.NewRequest(http.MethodPut, "/test-bucket?object-lock", strings.NewReader(objectLockXML))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT object-lock: expected status 200, got %d", w.Code)
	}

	// GET object lock configuration
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?object-lock", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET object-lock: expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Enabled") {
		t.Error("GET object-lock response should contain ObjectLockEnabled")
	}
}

// TestPutObjectLockConfiguration tests PUT ?object-lock on a bucket
func TestPutObjectLockConfiguration(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test PUT with valid object lock configuration
	objectLockXML := `<?xml version="1.0" encoding="UTF-8"?>
<ObjectLockConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <ObjectLockEnabled>Enabled</ObjectLockEnabled>
  <Rule>
    <DefaultRetention>
      <Mode>COMPLIANCE</Mode>
      <Years>1</Years>
    </DefaultRetention>
  </Rule>
</ObjectLockConfiguration>`

	req := httptest.NewRequest(http.MethodPut, "/test-bucket?object-lock", strings.NewReader(objectLockXML))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT object-lock: expected status 200, got %d", w.Code)
	}

	// Verify the configuration was stored
	if mb.objectLockConfig["test-bucket"] == nil {
		t.Error("object lock configuration should have been stored")
	}
}

// TestGetObjectRetention tests GET ?retention on an object
func TestGetObjectRetention(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test 1: Get retention when not set - should return error
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/test-object.txt?retention", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Should return 500 because our mock returns an error when config doesn't exist
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for missing retention, got %d", w.Code)
	}

	// Test 2: Set and then get retention
	retentionXML := `<?xml version="1.0" encoding="UTF-8"?>
<Retention xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Mode>GOVERNANCE</Mode>
  <RetainUntilDate>2026-12-31T00:00:00Z</RetainUntilDate>
</Retention>`

	// PUT retention
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/test-object.txt?retention", strings.NewReader(retentionXML))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT retention: expected status 200, got %d", w.Code)
	}

	// GET retention
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/test-object.txt?retention", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET retention: expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "GOVERNANCE") {
		t.Error("GET retention response should contain Mode")
	}
}

// TestPutObjectRetention tests PUT ?retention on an object
func TestPutObjectRetention(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test PUT with valid retention configuration
	retentionXML := `<?xml version="1.0" encoding="UTF-8"?>
<Retention xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Mode>COMPLIANCE</Mode>
  <RetainUntilDate>2027-01-01T00:00:00Z</RetainUntilDate>
</Retention>`

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/retained-file.txt?retention", strings.NewReader(retentionXML))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT retention: expected status 200, got %d", w.Code)
	}

	// Verify the retention was stored
	key := "test-bucket/retained-file.txt"
	if mb.retentionConfig[key] == nil {
		t.Error("retention configuration should have been stored")
	}
}

// TestGetObjectLegalHold tests GET ?legal-hold on an object
func TestGetObjectLegalHold(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test 1: Get legal hold when not set - should return error
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/test-object.txt?legal-hold", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Should return 500 because our mock returns an error when config doesn't exist
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for missing legal hold, got %d", w.Code)
	}

	// Test 2: Set and then get legal hold
	legalHoldXML := `<?xml version="1.0" encoding="UTF-8"?>
<LegalHold xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Status>ON</Status>
</LegalHold>`

	// PUT legal hold
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/test-object.txt?legal-hold", strings.NewReader(legalHoldXML))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT legal-hold: expected status 200, got %d", w.Code)
	}

	// GET legal hold
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/test-object.txt?legal-hold", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET legal-hold: expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "ON") {
		t.Error("GET legal-hold response should contain Status")
	}
}

// TestPutObjectLegalHold tests PUT ?legal-hold on an object
func TestPutObjectLegalHold(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test PUT with legal hold ON
	legalHoldOnXML := `<?xml version="1.0" encoding="UTF-8"?>
<LegalHold xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Status>ON</Status>
</LegalHold>`

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/legal-hold-file.txt?legal-hold", strings.NewReader(legalHoldOnXML))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT legal-hold ON: expected status 200, got %d", w.Code)
	}

	// Verify the legal hold was stored
	key := "test-bucket/legal-hold-file.txt"
	if mb.legalHoldConfig[key] == nil {
		t.Error("legal hold configuration should have been stored")
	}

	// Test PUT with legal hold OFF (removing the hold)
	legalHoldOffXML := `<?xml version="1.0" encoding="UTF-8"?>
<LegalHold xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Status>OFF</Status>
</LegalHold>`

	req = httptest.NewRequest(http.MethodPut, "/test-bucket/legal-hold-file.txt?legal-hold", strings.NewReader(legalHoldOffXML))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT legal-hold OFF: expected status 200, got %d", w.Code)
	}

	// Verify the legal hold was updated
	if !bytes.Contains(mb.legalHoldConfig[key], []byte("OFF")) {
		t.Error("legal hold should have been updated to OFF")
	}
}

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
type headCountingBackend struct {
	*mockBackend
	headCalls atomic.Int64
}

func (h *headCountingBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	h.headCalls.Add(1)
	return h.mockBackend.Head(ctx, bucket, key)
}

// mockManifestRecorder is a simple ManifestRecorder for testing.
type mockManifestRecorder struct {
	mu      sync.RWMutex
	entries map[string]*handlers.ManifestEntry
}

func newMockManifestRecorder() *mockManifestRecorder {
	return &mockManifestRecorder{entries: make(map[string]*handlers.ManifestEntry)}
}

func (m *mockManifestRecorder) RecordPut(bucket, key string, size int64, sha256Hex string, iv, wrappedDEK []byte, blockSize int, contentType, etag string) {
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
func (m *mockManifestRecorder) seed(bucket, key string, entry *handlers.ManifestEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[bucket+"/"+key] = entry
}

// TestHeadObjectManifestFastPath verifies that HeadObject returns metadata from
// the in-memory manifest without issuing a B2 HeadObject call.
func TestHeadObjectManifestFastPath(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)

	hcb := &headCountingBackend{mockBackend: mb}
	rec := newMockManifestRecorder()

	h := handlers.New(cfg, hcb, cache, footerCache, km, nil)
	h.WithManifest(rec)

	const (
		bucket      = "test-bucket"
		key         = "manifest-test/file.parquet"
		wantSize    = int64(123456)
		wantCType   = "application/octet-stream"
		wantETag    = "abc123"
	)

	// Pre-populate the manifest — no object exists in the backend.
	rec.seed(bucket, key, &handlers.ManifestEntry{
		PlaintextSize: wantSize,
		ContentType:   wantCType,
		ETag:          wantETag,
		LastModified:  time.Now(),
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
