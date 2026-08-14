// Package replication provides mock backend for testing replication workers.
package replication

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

// MockBackend is a simple in-memory backend for testing replication workers.
// It implements the backend.Backend interface with basic functionality.
type MockBackend struct {
	mu      sync.RWMutex
	objects map[string]map[string]*mockObject // bucket -> key -> object
	copyErr bool                               // Force Copy() to fail for testing fallback
}

// mockObject represents an object stored in the mock backend.
type mockObject struct {
	data     []byte
	meta     map[string]string
	size     int64
	modified time.Time
}

// NewMockBackend creates a new MockBackend.
func NewMockBackend() *MockBackend {
	return &MockBackend{
		objects: make(map[string]map[string]*mockObject),
	}
}

// SetCopyError forces Copy() to return an error for testing fallback logic.
func (m *MockBackend) SetCopyError(copyErr bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.copyErr = copyErr
}

// Put stores an object in the mock backend.
func (m *MockBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.objects[bucket] == nil {
		m.objects[bucket] = make(map[string]*mockObject)
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	m.objects[bucket][key] = &mockObject{
		data:     data,
		meta:     meta,
		size:     int64(len(data)),
		modified: time.Now(),
	}

	return nil
}

// Get retrieves an object from the mock backend.
func (m *MockBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.objects[bucket] == nil {
		return nil, nil, errors.New("bucket not found")
	}

	obj, ok := m.objects[bucket][key]
	if !ok {
		return nil, nil, errors.New("object not found")
	}

	info := &backend.ObjectInfo{
		Key:          key,
		Size:         obj.size,
		ContentType:  obj.meta["Content-Type"],
		ETag:         obj.meta["ETag"],
		LastModified: obj.modified,
		Metadata:     obj.meta,
	}

	return io.NopCloser(strings.NewReader(string(obj.data))), info, nil
}

// Copy copies an object within or between buckets.
func (m *MockBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate Copy() failure for testing fallback logic
	if m.copyErr {
		return errors.New("mock Copy() failure for testing fallback")
	}

	if m.objects[srcBucket] == nil {
		return errors.New("source bucket not found")
	}

	obj, ok := m.objects[srcBucket][srcKey]
	if !ok {
		return errors.New("source object not found")
	}

	if m.objects[dstBucket] == nil {
		m.objects[dstBucket] = make(map[string]*mockObject)
	}

	// Copy the object
	copiedObj := &mockObject{
		data:     make([]byte, len(obj.data)),
		size:     obj.size,
		modified: time.Now(),
	}
	copy(copiedObj.data, obj.data)

	// Handle metadata
	if replaceMetadata {
		copiedObj.meta = meta
	} else {
		copiedObj.meta = make(map[string]string)
		for k, v := range obj.meta {
			copiedObj.meta[k] = v
		}
		// Apply any overrides
		for k, v := range meta {
			copiedObj.meta[k] = v
		}
	}

	m.objects[dstBucket][dstKey] = copiedObj
	return nil
}

// GetRange retrieves a byte range from an object.
func (m *MockBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.objects[bucket] == nil {
		return nil, errors.New("bucket not found")
	}

	obj, ok := m.objects[bucket][key]
	if !ok {
		return nil, errors.New("object not found")
	}

	if offset < 0 || offset >= int64(len(obj.data)) {
		return nil, errors.New("invalid offset")
	}

	end := offset + length
	if end > int64(len(obj.data)) {
		end = int64(len(obj.data))
	}

	return io.NopCloser(strings.NewReader(string(obj.data[offset:end]))), nil
}

// GetRangeWithHeaders retrieves a byte range from an object along with response headers.
func (m *MockBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	reader, err := m.GetRange(ctx, bucket, key, offset, length)
	if err != nil {
		return nil, nil, err
	}

	// Return empty headers for mock backend
	headers := make(map[string]string)
	return reader, headers, nil
}

// Head retrieves object metadata without the body.
func (m *MockBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.objects[bucket] == nil {
		return nil, errors.New("bucket not found")
	}

	obj, ok := m.objects[bucket][key]
	if !ok {
		return nil, errors.New("object not found")
	}

	return &backend.ObjectInfo{
		Key:          key,
		Size:         obj.size,
		ContentType:  obj.meta["Content-Type"],
		ETag:         obj.meta["ETag"],
		LastModified: obj.modified,
		Metadata:     obj.meta,
	}, nil
}

// Delete removes an object.
func (m *MockBackend) Delete(ctx context.Context, bucket, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.objects[bucket] == nil {
		return errors.New("bucket not found")
	}

	delete(m.objects[bucket], key)
	return nil
}

// DeleteObjects removes multiple objects.
func (m *MockBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		if err := m.Delete(ctx, bucket, key); err != nil {
			return err
		}
	}
	return nil
}

// List objects in a bucket.
func (m *MockBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.objects[bucket] == nil {
		return nil, errors.New("bucket not found")
	}

	var objects []backend.ObjectInfo
	for key, obj := range m.objects[bucket] {
		// Apply prefix filter
		if prefix != "" && len(key) < len(prefix) {
			continue
		}
		if prefix != "" && key[:len(prefix)] != prefix {
			continue
		}

		objects = append(objects, backend.ObjectInfo{
			Key:          key,
			Size:         obj.size,
			ContentType:  obj.meta["Content-Type"],
			ETag:         obj.meta["ETag"],
			LastModified: obj.modified,
			Metadata:     obj.meta,
		})

		// Respect maxKeys limit
		if maxKeys > 0 && len(objects) >= maxKeys {
			break
		}
	}

	return &backend.ListResult{
		Objects:     objects,
		IsTruncated: false,
	}, nil
}

// ListBuckets lists all buckets.
func (m *MockBackend) ListBuckets(ctx context.Context) ([]backend.BucketInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var buckets []backend.BucketInfo
	for bucket := range m.objects {
		buckets = append(buckets, backend.BucketInfo{
			Name:         bucket,
			CreationDate: time.Now(),
		})
	}

	return buckets, nil
}

// CreateBucket creates a new bucket.
func (m *MockBackend) CreateBucket(ctx context.Context, bucket string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.objects[bucket] == nil {
		m.objects[bucket] = make(map[string]*mockObject)
	}
	return nil
}

// DeleteBucket deletes an empty bucket.
func (m *MockBackend) DeleteBucket(ctx context.Context, bucket string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.objects[bucket] == nil {
		return errors.New("bucket not found")
	}

	if len(m.objects[bucket]) > 0 {
		return errors.New("bucket not empty")
	}

	delete(m.objects, bucket)
	return nil
}

// HeadBucket checks if a bucket exists.
func (m *MockBackend) HeadBucket(ctx context.Context, bucket string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.objects[bucket] == nil {
		return errors.New("bucket not found")
	}
	return nil
}

// GetDirect retrieves an object directly (same as Get for mock backend).
func (m *MockBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return m.Get(ctx, bucket, key)
}

// Multipart upload operations (stubs for mock backend)

// CreateMultipartUpload initiates a multipart upload.
func (m *MockBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	return "mock-upload-id", nil
}

// UploadPart uploads a part to a multipart upload.
func (m *MockBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	return "mock-etag", nil
}

// CompleteMultipartUpload completes a multipart upload.
func (m *MockBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	return "mock-etag", nil
}

// AbortMultipartUpload aborts a multipart upload.
func (m *MockBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	return nil
}

// ListParts lists the parts of a multipart upload.
func (m *MockBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*backend.ListPartsResult, error) {
	return &backend.ListPartsResult{}, nil
}

// ListMultipartUploads lists active multipart uploads.
func (m *MockBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*backend.ListMultipartUploadsResult, error) {
	return &backend.ListMultipartUploadsResult{}, nil
}

// Lifecycle configuration operations (stubs for mock backend)

// GetBucketLifecycleConfiguration gets the lifecycle configuration for a bucket.
func (m *MockBackend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, nil
}

// PutBucketLifecycleConfiguration sets the lifecycle configuration for a bucket.
func (m *MockBackend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

// DeleteBucketLifecycleConfiguration deletes the lifecycle configuration for a bucket.
func (m *MockBackend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	return nil
}

// Object Lock operations (stubs for mock backend)

// GetObjectLockConfiguration gets the object lock configuration for a bucket.
func (m *MockBackend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, nil
}

// PutObjectLockConfiguration sets the object lock configuration for a bucket.
func (m *MockBackend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

// GetObjectRetention gets the retention settings for an object.
func (m *MockBackend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, nil
}

// PutObjectRetention sets the retention settings for an object.
func (m *MockBackend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	return nil
}

// GetObjectLegalHold gets the legal hold status for an object.
func (m *MockBackend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, nil
}

// PutObjectLegalHold sets the legal hold status for an object.
func (m *MockBackend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	return nil
}

// Versioning operations (stubs for mock backend)

// ListObjectVersions lists all versions of objects in a bucket.
func (m *MockBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*backend.ListObjectVersionsResult, error) {
	return &backend.ListObjectVersionsResult{}, nil
}

// HeadVersion retrieves object metadata for a specific version.
func (m *MockBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*backend.ObjectInfo, error) {
	return nil, errors.New("versions not supported in mock backend")
}
