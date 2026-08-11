// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"context"
	"io"
)

// NilBackend is a no-op backend that implements the Backend interface with
// safe default behavior when no secondary backend is configured. All methods
// return appropriate zero values and nil errors, making replication a complete
// no-op without requiring nil checks at every call site.
type NilBackend struct{}

// NewNilBackend creates a new no-op backend.
func NewNilBackend() *NilBackend {
	return &NilBackend{}
}

// Put is a no-op that returns nil (success).
func (b *NilBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	return nil
}

// Get returns (nil, nil) indicating no object.
func (b *NilBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	return nil, nil, nil
}

// GetRange returns (nil, nil) indicating no content.
func (b *NilBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	return nil, nil
}

// GetRangeWithHeaders returns (nil, nil, nil) indicating no content.
func (b *NilBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	return nil, nil, nil
}

// Head returns (nil, nil) indicating no object metadata.
func (b *NilBackend) Head(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	return nil, nil
}

// Delete is a no-op that returns nil (success).
func (b *NilBackend) Delete(ctx context.Context, bucket, key string) error {
	return nil
}

// DeleteObjects is a no-op that returns nil (success).
func (b *NilBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	return nil
}

// List returns an empty result with no truncation.
func (b *NilBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*ListResult, error) {
	return &ListResult{
		Objects:        []ObjectInfo{},
		IsTruncated:    false,
		NextToken:      "",
		CommonPrefixes: []string{},
	}, nil
}

// Copy is a no-op that returns nil (success).
func (b *NilBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	return nil
}

// ListBuckets returns an empty slice with nil error.
func (b *NilBackend) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	return []BucketInfo{}, nil
}

// CreateBucket is a no-op that returns nil (success).
func (b *NilBackend) CreateBucket(ctx context.Context, bucket string) error {
	return nil
}

// DeleteBucket is a no-op that returns nil (success).
func (b *NilBackend) DeleteBucket(ctx context.Context, bucket string) error {
	return nil
}

// HeadBucket is a no-op that returns nil (success).
func (b *NilBackend) HeadBucket(ctx context.Context, bucket string) error {
	return nil
}

// GetDirect returns (nil, nil, nil) indicating no object.
func (b *NilBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	return nil, nil, nil
}

// CreateMultipartUpload returns ("", nil) indicating no upload.
func (b *NilBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	return "", nil
}

// UploadPart returns ("", nil) indicating no part uploaded.
func (b *NilBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	return "", nil
}

// CompleteMultipartUpload returns ("", nil) indicating no completion.
func (b *NilBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) (string, error) {
	return "", nil
}

// AbortMultipartUpload is a no-op that returns nil (success).
func (b *NilBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	return nil
}

// ListParts returns an empty result with nil error.
func (b *NilBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*ListPartsResult, error) {
	return &ListPartsResult{
		Bucket:               bucket,
		Key:                  key,
		UploadID:             uploadID,
		Parts:                []PartInfo{},
		NextPartNumberMarker: 0,
		IsTruncated:          false,
	}, nil
}

// ListMultipartUploads returns an empty result with nil error.
func (b *NilBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*ListMultipartUploadsResult, error) {
	return &ListMultipartUploadsResult{
		Bucket:             bucket,
		Uploads:            []UploadInfo{},
		NextKeyMarker:      "",
		NextUploadIDMarker: "",
		IsTruncated:        false,
	}, nil
}

// GetBucketLifecycleConfiguration returns (nil, nil) indicating no lifecycle config.
func (b *NilBackend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, nil
}

// PutBucketLifecycleConfiguration is a no-op that returns nil (success).
func (b *NilBackend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

// DeleteBucketLifecycleConfiguration is a no-op that returns nil (success).
func (b *NilBackend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	return nil
}

// GetObjectLockConfiguration returns (nil, nil) indicating no lock config.
func (b *NilBackend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, nil
}

// PutObjectLockConfiguration is a no-op that returns nil (success).
func (b *NilBackend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

// GetObjectRetention returns (nil, nil) indicating no retention.
func (b *NilBackend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, nil
}

// PutObjectRetention is a no-op that returns nil (success).
func (b *NilBackend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	return nil
}

// GetObjectLegalHold returns (nil, nil) indicating no legal hold.
func (b *NilBackend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, nil
}

// PutObjectLegalHold is a no-op that returns nil (success).
func (b *NilBackend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	return nil
}

// ListObjectVersions returns an empty result with nil error.
func (b *NilBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*ListObjectVersionsResult, error) {
	return &ListObjectVersionsResult{
		Versions:            []ObjectVersionInfo{},
		IsTruncated:         false,
		NextKeyMarker:       "",
		NextVersionIDMarker: "",
		CommonPrefixes:      []string{},
	}, nil
}

// HeadVersion returns (nil, nil) indicating no version metadata.
func (b *NilBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*ObjectInfo, error) {
	return nil, nil
}
