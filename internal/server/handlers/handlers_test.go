// Package handlers_test tests the S3 operation handlers.
package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
)

// mockBackend implements backend.Backend for testing.
type mockBackend struct {
	mu                                 sync.Mutex
	objects                            map[string][]byte
	meta                               map[string]map[string]string
	modified                           map[string]time.Time // LastModified stamped at write time, as a real object store would
	capturedListMultipartUploadsPrefix string               // Captured prefix argument for testing
	capturedAbortMultipartKey          string
	capturedListPartsKey               string
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		objects:  make(map[string][]byte),
		meta:     make(map[string]map[string]string),
		modified: make(map[string]time.Time),
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
	m.modified[bucket+"/"+key] = time.Now()
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
		StoredSize:       int64(len(data)),
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
		StoredSize:       int64(len(data)),
		Metadata:         meta,
		IsARMOREncrypted: meta["x-amz-meta-armor-version"] != "",
		LastModified:     m.modified[k],
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
	delete(m.modified, k)
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
	var commonPrefixes []string
	prefixPath := bucket + "/" + prefix

	for k, data := range m.objects {
		if prefix != "" && (len(k) < len(prefixPath) || k[:len(prefixPath)] != prefixPath) {
			continue
		}
		key := k[len(bucket)+1:]

		// If delimiter is set, check for common prefixes
		if delimiter != "" {
			// Find the delimiter position after the prefix
			searchStart := len(prefix)
			delimiterPos := strings.Index(key[searchStart:], delimiter)
			if delimiterPos != -1 {
				// This is a common prefix (directory)
				commonPrefix := key[:searchStart+delimiterPos+len(delimiter)]
				// Avoid duplicates
				found := false
				for _, cp := range commonPrefixes {
					if cp == commonPrefix {
						found = true
						break
					}
				}
				if !found {
					commonPrefixes = append(commonPrefixes, commonPrefix)
				}
				continue // Skip this key, it's represented as a common prefix
			}
		}

		meta := m.meta[k]
		info := backend.ObjectInfo{
			Key:        key,
			Size:       int64(len(data)),
			StoredSize: int64(len(data)),
			Metadata:   meta,
		}
		if am, ok := backend.ParseARMORMetadata(meta); ok {
			info.Size = am.PlaintextSize
			info.ETag = am.ETag
		}
		objects = append(objects, info)
	}

	return &backend.ListResult{Objects: objects, CommonPrefixes: commonPrefixes}, nil
}

func (m *mockBackend) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	return m.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
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
	m.modified[dst] = time.Now()
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

// Multipart upload methods (stub implementations for testing)
func (m *mockBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	return fmt.Sprintf("upload-%d", time.Now().UnixNano()), nil
}

func (m *mockBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	// The real backends consume the whole part body, and the handler verifies
	// exactly that after the upload, so the mock must drain it too.
	if _, err := io.Copy(io.Discard, body); err != nil {
		return "", err
	}
	return fmt.Sprintf("etag-%d", partNumber), nil
}

func (m *mockBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	return "final-etag", nil
}

func (m *mockBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	m.mu.Lock()
	m.capturedAbortMultipartKey = key
	m.mu.Unlock()
	return nil
}

func (m *mockBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*backend.ListPartsResult, error) {
	m.mu.Lock()
	m.capturedListPartsKey = key
	m.mu.Unlock()
	return &backend.ListPartsResult{}, nil
}

func (m *mockBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*backend.ListMultipartUploadsResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Capture the prefix for test assertions
	m.capturedListMultipartUploadsPrefix = prefix

	// Return a result with at least one upload and a NextKeyMarker with the prefix prepended
	// This allows tests to verify prefix handling and stripPrefix logic
	result := &backend.ListMultipartUploadsResult{
		Bucket:        bucket,
		Uploads:       []backend.UploadInfo{},
		NextKeyMarker: prefix + "next-upload-marker.txt",
		IsTruncated:   true,
	}

	// Return an upload with the prefix prepended to exercise stripPrefix logic
	result.Uploads = append(result.Uploads, backend.UploadInfo{
		UploadID:  "test-upload-id",
		Key:       prefix + "test-multipart-upload.txt",
		Initiated: time.Now(),
	})

	return result, nil
}

// Lifecycle configuration methods (stub implementations for testing)

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

	bucketPrefix := bucket + "/"

	// Find objects matching the prefix
	for k := range m.objects {
		if !strings.HasPrefix(k, bucketPrefix) {
			continue
		}
		key := strings.TrimPrefix(k, bucketPrefix)

		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}

		// If delimiter is set, check for common prefixes
		if delimiter != "" {
			// Find the delimiter position after the prefix
			searchStart := len(prefix)
			if len(key) > searchStart {
				delimiterPos := strings.Index(key[searchStart:], delimiter)
				if delimiterPos != -1 {
					// This is a common prefix (directory)
					commonPrefix := key[:searchStart+delimiterPos+len(delimiter)]
					// Avoid duplicates
					found := false
					for _, cp := range result.CommonPrefixes {
						if cp == commonPrefix {
							found = true
							break
						}
					}
					if !found {
						result.CommonPrefixes = append(result.CommonPrefixes, commonPrefix)
					}
					continue // Skip this key, it's represented as a common prefix
				}
			}
		}

		// Create version info for actual objects (not common prefixes)
		meta := m.meta[k]
		info := backend.ObjectVersionInfo{
			Key:          key,
			VersionID:    "v1",
			Size:         int64(len(m.objects[k])),
			ETag:         meta["x-amz-meta-armor-etag"],
			LastModified: time.Now(),
			IsLatest:     true,
		}
		result.Versions = append(result.Versions, info)
	}

	return result, nil
}

func (m *mockBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*backend.ObjectInfo, error) {
	return m.Head(ctx, bucket, key)
}

// testSetup creates common test dependencies.

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
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
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
