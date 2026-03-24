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
	return meta
}
