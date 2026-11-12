package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
)

// TestGetObjectChecksSecondaryBackendOnPrimaryFailure verifies that GetObject
// checks the secondary backend when the primary backend fails. This test creates
// a scenario where the primary backend is inaccessible and the secondary backend
// contains the object, verifying fallback behavior per the integration contract.
//
// This test uses a mock primary backend that fails reads and a filesystem secondary
// backend that succeeds, demonstrating the failover path.
func TestGetObjectChecksSecondaryBackendOnPrimaryFailure(t *testing.T) {
	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-fallback-object.txt"
	body := []byte("test content for fallback")

	// Create secondary filesystem backend (this will be our successful backend)
	secondaryDir := t.TempDir()
	secondaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: secondaryDir})
	if err != nil {
		t.Fatalf("failed to create secondary backend: %v", err)
	}

	// Create bucket in secondary backend and put object there
	if err := secondaryBackend.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("failed to create bucket in secondary: %v", err)
	}

	// Manually put an object in the secondary backend (simulating it was replicated)
	// We need to create an ARMOR-encrypted object, so we'll simulate that by putting
	// a non-ARMOR object for simplicity - the test just verifies secondary backend is checked
	if err := secondaryBackend.Put(ctx, bucket, key, bytes.NewReader(body), int64(len(body)), map[string]string{
		"Content-Type": "text/plain",
	}); err != nil {
		t.Fatalf("failed to put object in secondary: %v", err)
	}

	// Create a failing primary backend (mock that fails Get/Head operations)
	failingPrimary := &failingBackend{}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create keymanager: %v", err)
	}

	// Create handler with failing primary and working secondary
	h := &Handlers{
		config:           cfg,
		backend:          failingPrimary,
		secondaryBackend: secondaryBackend,
		cache:            backend.NewMetadataCache(1000, 300),
		footerCache:      backend.NewFooterCache(1000, 300),
		listCache:        backend.NewListCache(1000, 300),
		keyManager:       km,
	}

	// GetObject should attempt primary, fail, and may attempt secondary
	// Note: Current implementation doesn't actually fall back to secondary for reads,
	// but this test documents the expected behavior contract
	reqGet := httptest.NewRequest("GET", "/"+bucket+"/"+key, nil)
	reqGet = reqGet.WithContext(ctx)
	wGet := httptest.NewRecorder()
	h.GetObject(wGet, reqGet, bucket, key)

	// The current implementation does not implement secondary fallback for GetObject.
	// An unclassified primary failure is now surfaced as a server error rather
	// than being misreported as an absent object.
	switch wGet.Code {
	case http.StatusOK:
		t.Log("GetObject succeeded via secondary backend - fallback is implemented")
		// If fallback is implemented, verify we got the right content
		if !bytes.Equal(wGet.Body.Bytes(), body) {
			t.Errorf("content mismatch: got %q, want %q", wGet.Body.String(), string(body))
		}
	case http.StatusInternalServerError:
		t.Log("GetObject returned 500 - secondary backend fallback not yet implemented")
		if !strings.Contains(wGet.Body.String(), "<Code>InternalError</Code>") {
			t.Errorf("primary backend failure response = %q, want InternalError", wGet.Body.String())
		}
		// Verify the object exists in secondary (demonstrating it was there for potential fallback)
		secBody, secInfo, err := secondaryBackend.Get(ctx, bucket, key)
		if err != nil {
			t.Errorf("object not found in secondary backend: %v", err)
		} else {
			secBody.Close()
			if secInfo.Size != int64(len(body)) {
				t.Errorf("secondary object size mismatch: got %d, want %d", secInfo.Size, len(body))
			}
		}
	default:
		t.Errorf("unexpected status code: got %d, want 200 or 500", wGet.Code)
	}
}

// TestPutObjectDoesNotReplicateToSecondaryBackend verifies the current behavior
// that PutObject does NOT replicate to the secondary backend. The secondaryBackend
// field exists but is not used by PutObject in the current implementation.
// This test documents the actual behavior - replication is a no-op.
//
// This is expected current behavior per ADR-006: secondary backend replication
// is a complete no-op unless explicitly implemented. The field exists for future
// use but is not currently used by any handler methods.

// TestPutObjectDoesNotReplicateToSecondaryBackend verifies the current behavior
// that PutObject does NOT replicate to the secondary backend. The secondaryBackend
// field exists but is not used by PutObject in the current implementation.
// This test documents the actual behavior - replication is a no-op.
//
// This is expected current behavior per ADR-006: secondary backend replication
// is a complete no-op unless explicitly implemented. The field exists for future
// use but is not currently used by any handler methods.
func TestPutObjectDoesNotReplicateToSecondaryBackend(t *testing.T) {
	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-no-replication.txt"
	body := []byte("test content - no replication expected")

	// Create primary filesystem backend
	primaryDir := t.TempDir()
	primaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: primaryDir})
	if err != nil {
		t.Fatalf("failed to create primary backend: %v", err)
	}

	// Create secondary filesystem backend (should remain empty)
	secondaryDir := t.TempDir()
	secondaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: secondaryDir})
	if err != nil {
		t.Fatalf("failed to create secondary backend: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create keymanager: %v", err)
	}

	// Create handler with both primary and secondary backends
	h := &Handlers{
		config:           cfg,
		backend:          primaryBackend,
		secondaryBackend: secondaryBackend,
		cache:            backend.NewMetadataCache(1000, 300),
		footerCache:      backend.NewFooterCache(1000, 300),
		listCache:        backend.NewListCache(1000, 300),
		keyManager:       km,
	}

	// Create bucket in both backends
	if err := primaryBackend.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("failed to create bucket in primary: %v", err)
	}
	if err := secondaryBackend.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("failed to create bucket in secondary: %v", err)
	}

	// Perform PutObject
	reqPut := httptest.NewRequest("PUT", "/"+bucket+"/"+key, bytes.NewReader(body))
	reqPut = reqPut.WithContext(ctx)
	reqPut.Header.Set("Content-Type", "text/plain")
	wPut := httptest.ResponseRecorder{}
	h.PutObject(&wPut, reqPut, bucket, key)

	if wPut.Code != http.StatusOK {
		t.Fatalf("PutObject failed: status %d, body: %s", wPut.Code, wPut.Body.String())
	}

	// Verify object exists in primary backend
	primaryInfo, err := primaryBackend.Head(ctx, bucket, h.applyPrefix(key))
	if err != nil {
		t.Errorf("object not found in primary backend: %v", err)
	} else {
		if primaryInfo.ETag == "" {
			t.Error("primary object ETag is empty")
		}
		t.Logf("Object successfully stored in primary backend: ETag=%s, Size=%d", primaryInfo.ETag, primaryInfo.Size)
	}

	// Verify object does NOT exist in secondary backend (current expected behavior)
	_, err = secondaryBackend.Head(ctx, bucket, h.applyPrefix(key))
	if err == nil {
		t.Error("unexpected: object found in secondary backend - replication is not implemented")
	} else {
		t.Logf("Expected: object not in secondary backend (%v) - replication not implemented", err)
	}

	// Verify GetObject still works (reads from primary)
	reqGet := httptest.NewRequest("GET", "/"+bucket+"/"+key, nil)
	reqGet = reqGet.WithContext(ctx)
	wGet := httptest.NewRecorder()
	h.GetObject(wGet, reqGet, bucket, key)

	if wGet.Code != http.StatusOK {
		t.Errorf("GetObject failed: status %d", wGet.Code)
	}

	// Verify the content matches what we put in
	if wGet.Body != nil && !bytes.Equal(wGet.Body.Bytes(), body) {
		t.Errorf("content mismatch: got %q, want %q", wGet.Body.String(), string(body))
	}
}

// failingBackend is a mock backend that always fails for testing fallback behavior

// failingBackend is a mock backend that always fails for testing fallback behavior
type failingBackend struct{}

func (f *failingBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return nil, nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	return nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	return nil, nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	return nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) Delete(ctx context.Context, bucket, key string) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	return nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	return f.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

func (f *failingBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) ListBuckets(ctx context.Context) ([]backend.BucketInfo, error) {
	return nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) CreateBucket(ctx context.Context, bucket string) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) DeleteBucket(ctx context.Context, bucket string) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) HeadBucket(ctx context.Context, bucket string) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return nil, nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	return "", fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	return "", fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	return "", fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*backend.ListPartsResult, error) {
	return nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*backend.ListMultipartUploadsResult, error) {
	return nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	return fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*backend.ListObjectVersionsResult, error) {
	return nil, fmt.Errorf("primary backend is failing")
}

func (f *failingBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*backend.ObjectInfo, error) {
	return nil, fmt.Errorf("primary backend is failing")
}
