// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"time"
)

// ObjectInfo contains metadata about an object.
type ObjectInfo struct {
	Key           string
	Size          int64 // Plaintext size
	ContentType   string
	ETag          string
	LastModified  time.Time
	Metadata      map[string]string
	IsARMOREncrypted bool
}

// ListResult contains the result of a ListObjects operation.
type ListResult struct {
	Objects      []ObjectInfo
	IsTruncated  bool
	NextToken    string
	CommonPrefixes []string
}

// BucketInfo contains metadata about a bucket.
type BucketInfo struct {
	Name         string
	CreationDate time.Time
}

// Backend defines the interface for storage backends.
// Implementations include B2 S3, mock backends for testing, etc.
type Backend interface {
	// Put stores an object with the given metadata.
	Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error

	// Get retrieves an object's full content and metadata.
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error)

	// GetRange retrieves a byte range from an object.
	GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error)

	// GetRangeWithHeaders retrieves a byte range from an object along with response headers.
	// The headers map contains relevant HTTP response headers (e.g., CF-Cache-Status).
	GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error)

	// Head retrieves object metadata without the body.
	Head(ctx context.Context, bucket, key string) (*ObjectInfo, error)

	// Delete removes an object.
	Delete(ctx context.Context, bucket, key string) error

	// DeleteObjects removes multiple objects.
	DeleteObjects(ctx context.Context, bucket string, keys []string) error

	// List objects in a bucket with optional prefix.
	List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*ListResult, error)

	// Copy copies an object, optionally replacing metadata.
	// Supports cross-bucket copy (srcBucket and dstBucket can be different).
	Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error

	// ListBuckets lists all buckets.
	ListBuckets(ctx context.Context) ([]BucketInfo, error)

	// CreateBucket creates a new bucket.
	CreateBucket(ctx context.Context, bucket string) error

	// DeleteBucket deletes an empty bucket.
	DeleteBucket(ctx context.Context, bucket string) error

	// HeadBucket checks if a bucket exists.
	HeadBucket(ctx context.Context, bucket string) error

	// GetDirect retrieves an object directly from B2 (not via Cloudflare).
	// Used for internal .armor/ objects.
	GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error)

	// Multipart upload operations

	// CreateMultipartUpload initiates a multipart upload.
	CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (uploadID string, err error)

	// UploadPart uploads a part to a multipart upload.
	UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (etag string, err error)

	// CompleteMultipartUpload completes a multipart upload.
	CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) (etag string, err error)

	// AbortMultipartUpload aborts a multipart upload.
	AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error

	// ListParts lists the parts of a multipart upload.
	ListParts(ctx context.Context, bucket, key, uploadID string) (*ListPartsResult, error)

	// ListMultipartUploads lists active multipart uploads.
	ListMultipartUploads(ctx context.Context, bucket string) (*ListMultipartUploadsResult, error)

	// Lifecycle configuration operations (passthrough)

	// GetBucketLifecycleConfiguration gets the lifecycle configuration for a bucket.
	GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error)

	// PutBucketLifecycleConfiguration sets the lifecycle configuration for a bucket.
	PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error

	// DeleteBucketLifecycleConfiguration deletes the lifecycle configuration for a bucket.
	DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error

	// Object Lock operations (passthrough)

	// GetObjectLockConfiguration gets the object lock configuration for a bucket.
	GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error)

	// PutObjectLockConfiguration sets the object lock configuration for a bucket.
	PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error

	// GetObjectRetention gets the retention settings for an object.
	GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error)

	// PutObjectRetention sets the retention settings for an object.
	PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error

	// GetObjectLegalHold gets the legal hold status for an object.
	GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error)

	// PutObjectLegalHold sets the legal hold status for an object.
	PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error

	// Versioning operations

	// ListObjectVersions lists all versions of objects in a bucket.
	ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*ListObjectVersionsResult, error)

	// HeadVersion retrieves object metadata for a specific version.
	HeadVersion(ctx context.Context, bucket, key, versionID string) (*ObjectInfo, error)
}

// CompletedPart represents a completed part in a multipart upload.
type CompletedPart struct {
	PartNumber int32
	ETag       string
}

// PartInfo contains information about an uploaded part.
type PartInfo struct {
	PartNumber   int32
	ETag         string
	Size         int64
	LastModified time.Time
}

// ListPartsResult contains the result of a ListParts operation.
type ListPartsResult struct {
	Bucket           string
	Key              string
	UploadID         string
	Parts            []PartInfo
	NextPartNumberMarker int
	IsTruncated      bool
}

// UploadInfo contains information about an active multipart upload.
type UploadInfo struct {
	UploadID   string
	Key        string
	Initiated  time.Time
}

// ListMultipartUploadsResult contains the result of a ListMultipartUploads operation.
type ListMultipartUploadsResult struct {
	Bucket       string
	Uploads      []UploadInfo
	NextKeyMarker string
	NextUploadIDMarker string
	IsTruncated  bool
}

// ObjectVersionInfo contains metadata about an object version.
// Note: ContentType, Metadata, and IsARMOREncrypted are not populated by ListObjectVersions
// because the S3 API doesn't return them. Use HeadObject with VersionId to get these.
type ObjectVersionInfo struct {
	Key            string
	VersionID      string
	Size           int64 // Ciphertext size from API; for ARMOR objects, use HeadObject for plaintext size
	ETag           string
	LastModified   time.Time
	IsLatest       bool
	IsDeleteMarker bool
}

// ListObjectVersionsResult contains the result of a ListObjectVersions operation.
type ListObjectVersionsResult struct {
	Versions           []ObjectVersionInfo
	IsTruncated        bool
	NextKeyMarker      string
	NextVersionIDMarker string
	CommonPrefixes     []string
}

// ARMORMetadata extracts ARMOR-specific metadata from object headers.
type ARMORMetadata struct {
	Version       int
	BlockSize     int
	PlaintextSize int64
	ContentType   string
	IV            []byte
	WrappedDEK    []byte
	PlaintextSHA  string
	ETag          string
	KeyID         string // Key identifier for multi-key support (empty = default)
}

// ParseARMORMetadata extracts ARMOR metadata from S3 headers.
func ParseARMORMetadata(meta map[string]string) (*ARMORMetadata, bool) {
	version := meta["x-amz-meta-armor-version"]
	if version == "" {
		return nil, false
	}

	am := &ARMORMetadata{
		ContentType: meta["x-amz-meta-armor-content-type"],
		PlaintextSHA: meta["x-amz-meta-armor-plaintext-sha256"],
		ETag:        meta["x-amz-meta-armor-etag"],
	}

	// Parse version (expecting "1")
	if version == "1" {
		am.Version = 1
	}

	// Parse block size
	if bs := meta["x-amz-meta-armor-block-size"]; bs != "" {
		var blockSize int
		if _, err := fmt.Sscanf(bs, "%d", &blockSize); err == nil {
			am.BlockSize = blockSize
		}
	}

	// Parse plaintext size
	if ps := meta["x-amz-meta-armor-plaintext-size"]; ps != "" {
		var size int64
		if _, err := fmt.Sscanf(ps, "%d", &size); err == nil {
			am.PlaintextSize = size
		}
	}

	// Parse IV (base64)
	if iv := meta["x-amz-meta-armor-iv"]; iv != "" {
		if decoded, err := base64.StdEncoding.DecodeString(iv); err == nil {
			am.IV = decoded
		}
	}

	// Parse wrapped DEK (base64)
	if dek := meta["x-amz-meta-armor-wrapped-dek"]; dek != "" {
		if decoded, err := base64.StdEncoding.DecodeString(dek); err == nil {
			am.WrappedDEK = decoded
		}
	}

	// Parse key ID (for multi-key support)
	am.KeyID = meta["x-amz-meta-armor-key-id"]

	return am, true
}

// ToMetadata converts ARMORMetadata to S3 metadata headers.
func (am *ARMORMetadata) ToMetadata() map[string]string {
	meta := make(map[string]string)
	meta["x-amz-meta-armor-version"] = "1"
	meta["x-amz-meta-armor-block-size"] = fmt.Sprintf("%d", am.BlockSize)
	meta["x-amz-meta-armor-plaintext-size"] = fmt.Sprintf("%d", am.PlaintextSize)
	meta["x-amz-meta-armor-content-type"] = am.ContentType
	meta["x-amz-meta-armor-iv"] = base64.StdEncoding.EncodeToString(am.IV)
	meta["x-amz-meta-armor-wrapped-dek"] = base64.StdEncoding.EncodeToString(am.WrappedDEK)
	meta["x-amz-meta-armor-plaintext-sha256"] = am.PlaintextSHA
	meta["x-amz-meta-armor-etag"] = am.ETag
	// Only include key-id if set (non-default key)
	if am.KeyID != "" && am.KeyID != "default" {
		meta["x-amz-meta-armor-key-id"] = am.KeyID
	}
	return meta
}
