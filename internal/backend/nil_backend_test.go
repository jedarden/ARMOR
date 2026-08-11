// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestNilBackendGetObject verifies GetObject returns nil, nil when backend is nil.
func TestNilBackendGetObject(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	body, info, err := backend.Get(ctx, "test-bucket", "test-key")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if body != nil {
		t.Error("expected nil body, got non-nil")
		_ = body.Close() // Ensure cleanup if not nil
	}
	if info != nil {
		t.Errorf("expected nil ObjectInfo, got %+v", info)
	}
}

// TestNilBackendPutObject verifies PutObject returns nil when backend is nil.
func TestNilBackendPutObject(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()
	body := strings.NewReader("test content")

	err := backend.Put(ctx, "test-bucket", "test-key", body, 12, map[string]string{"Content-Type": "text/plain"})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendDeleteObject verifies DeleteObject returns nil when backend is nil.
func TestNilBackendDeleteObject(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	err := backend.Delete(ctx, "test-bucket", "test-key")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendListBuckets verifies ListBuckets returns empty slice and nil when backend is nil.
func TestNilBackendListBuckets(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	buckets, err := backend.ListBuckets(ctx)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if buckets == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(buckets) != 0 {
		t.Errorf("expected empty slice (length 0), got length %d", len(buckets))
	}
}

// TestNilBackendListObjects verifies ListObjects returns empty slice and nil when backend is nil.
func TestNilBackendListObjects(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	result, err := backend.List(ctx, "test-bucket", "prefix/", "/", "", 1000)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ListResult")
	}
	if len(result.Objects) != 0 {
		t.Errorf("expected 0 objects, got %d", len(result.Objects))
	}
	if result.IsTruncated {
		t.Error("expected IsTruncated to be false")
	}
	if result.NextToken != "" {
		t.Errorf("expected empty NextToken, got %q", result.NextToken)
	}
	if len(result.CommonPrefixes) != 0 {
		t.Errorf("expected 0 common prefixes, got %d", len(result.CommonPrefixes))
	}
}

// TestNilBackendGetRange verifies GetRange returns nil, nil when backend is nil.
func TestNilBackendGetRange(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	body, err := backend.GetRange(ctx, "test-bucket", "test-key", 0, 1024)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if body != nil {
		t.Error("expected nil body, got non-nil")
		_ = body.Close() // Ensure cleanup if not nil
	}
}

// TestNilBackendGetRangeWithHeaders verifies GetRangeWithHeaders returns nil, nil, nil when backend is nil.
func TestNilBackendGetRangeWithHeaders(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	body, headers, err := backend.GetRangeWithHeaders(ctx, "test-bucket", "test-key", 0, 1024)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if body != nil {
		t.Error("expected nil body, got non-nil")
		_ = body.Close() // Ensure cleanup if not nil
	}
	if headers != nil {
		t.Errorf("expected nil headers, got %v", headers)
	}
}

// TestNilBackendHead verifies Head returns nil, nil when backend is nil.
func TestNilBackendHead(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	info, err := backend.Head(ctx, "test-bucket", "test-key")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if info != nil {
		t.Errorf("expected nil ObjectInfo, got %+v", info)
	}
}

// TestNilBackendDeleteObjects verifies DeleteObjects returns nil when backend is nil.
func TestNilBackendDeleteObjects(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()
	keys := []string{"key1", "key2", "key3"}

	err := backend.DeleteObjects(ctx, "test-bucket", keys)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendCopy verifies Copy returns nil when backend is nil.
func TestNilBackendCopy(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()
	meta := map[string]string{"Content-Type": "text/plain"}

	err := backend.Copy(ctx, "src-bucket", "src-key", "dst-bucket", "dst-key", meta, true)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendCreateBucket verifies CreateBucket returns nil when backend is nil.
func TestNilBackendCreateBucket(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	err := backend.CreateBucket(ctx, "test-bucket")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendDeleteBucket verifies DeleteBucket returns nil when backend is nil.
func TestNilBackendDeleteBucket(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	err := backend.DeleteBucket(ctx, "test-bucket")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendHeadBucket verifies HeadBucket returns nil when backend is nil.
func TestNilBackendHeadBucket(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	err := backend.HeadBucket(ctx, "test-bucket")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendGetDirect verifies GetDirect returns nil, nil, nil when backend is nil.
func TestNilBackendGetDirect(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	body, info, err := backend.GetDirect(ctx, "test-bucket", ".armor/test-key")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if body != nil {
		t.Error("expected nil body, got non-nil")
		_ = body.Close() // Ensure cleanup if not nil
	}
	if info != nil {
		t.Errorf("expected nil ObjectInfo, got %+v", info)
	}
}

// TestNilBackendCreateMultipartUpload verifies CreateMultipartUpload returns "", nil when backend is nil.
func TestNilBackendCreateMultipartUpload(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()
	meta := map[string]string{"Content-Type": "text/plain"}

	uploadID, err := backend.CreateMultipartUpload(ctx, "test-bucket", "test-key", meta)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if uploadID != "" {
		t.Errorf("expected empty upload ID, got %q", uploadID)
	}
}

// TestNilBackendUploadPart verifies UploadPart returns "", nil when backend is nil.
func TestNilBackendUploadPart(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()
	body := strings.NewReader("test part content")

	etag, err := backend.UploadPart(ctx, "test-bucket", "test-key", "upload-id", 1, body, 18)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if etag != "" {
		t.Errorf("expected empty ETag, got %q", etag)
	}
}

// TestNilBackendCompleteMultipartUpload verifies CompleteMultipartUpload returns "", nil when backend is nil.
func TestNilBackendCompleteMultipartUpload(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()
	parts := []CompletedPart{
		{PartNumber: 1, ETag: "etag1"},
		{PartNumber: 2, ETag: "etag2"},
	}

	etag, err := backend.CompleteMultipartUpload(ctx, "test-bucket", "test-key", "upload-id", parts)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if etag != "" {
		t.Errorf("expected empty ETag, got %q", etag)
	}
}

// TestNilBackendAbortMultipartUpload verifies AbortMultipartUpload returns nil when backend is nil.
func TestNilBackendAbortMultipartUpload(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	err := backend.AbortMultipartUpload(ctx, "test-bucket", "test-key", "upload-id")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendListParts verifies ListParts returns empty result with nil error when backend is nil.
func TestNilBackendListParts(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	result, err := backend.ListParts(ctx, "test-bucket", "test-key", "upload-id")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ListPartsResult")
	}
	if result.Bucket != "test-bucket" {
		t.Errorf("expected bucket %q, got %q", "test-bucket", result.Bucket)
	}
	if result.Key != "test-key" {
		t.Errorf("expected key %q, got %q", "test-key", result.Key)
	}
	if result.UploadID != "upload-id" {
		t.Errorf("expected upload ID %q, got %q", "upload-id", result.UploadID)
	}
	if len(result.Parts) != 0 {
		t.Errorf("expected 0 parts, got %d", len(result.Parts))
	}
	if result.NextPartNumberMarker != 0 {
		t.Errorf("expected NextPartNumberMarker 0, got %d", result.NextPartNumberMarker)
	}
	if result.IsTruncated {
		t.Error("expected IsTruncated to be false")
	}
}

// TestNilBackendListMultipartUploads verifies ListMultipartUploads returns empty result with nil error when backend is nil.
func TestNilBackendListMultipartUploads(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	result, err := backend.ListMultipartUploads(ctx, "test-bucket", "prefix/")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ListMultipartUploadsResult")
	}
	if result.Bucket != "test-bucket" {
		t.Errorf("expected bucket %q, got %q", "test-bucket", result.Bucket)
	}
	if len(result.Uploads) != 0 {
		t.Errorf("expected 0 uploads, got %d", len(result.Uploads))
	}
	if result.NextKeyMarker != "" {
		t.Errorf("expected empty NextKeyMarker, got %q", result.NextKeyMarker)
	}
	if result.NextUploadIDMarker != "" {
		t.Errorf("expected empty NextUploadIDMarker, got %q", result.NextUploadIDMarker)
	}
	if result.IsTruncated {
		t.Error("expected IsTruncated to be false")
	}
}

// TestNilBackendGetBucketLifecycleConfiguration verifies GetBucketLifecycleConfiguration returns nil, nil when backend is nil.
func TestNilBackendGetBucketLifecycleConfiguration(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	config, err := backend.GetBucketLifecycleConfiguration(ctx, "test-bucket")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if config != nil {
		t.Errorf("expected nil config, got %v", config)
	}
}

// TestNilBackendPutBucketLifecycleConfiguration verifies PutBucketLifecycleConfiguration returns nil when backend is nil.
func TestNilBackendPutBucketLifecycleConfiguration(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()
	config := []byte(`{"Rules":[]}`)

	err := backend.PutBucketLifecycleConfiguration(ctx, "test-bucket", config)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendDeleteBucketLifecycleConfiguration verifies DeleteBucketLifecycleConfiguration returns nil when backend is nil.
func TestNilBackendDeleteBucketLifecycleConfiguration(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	err := backend.DeleteBucketLifecycleConfiguration(ctx, "test-bucket")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendGetObjectLockConfiguration verifies GetObjectLockConfiguration returns nil, nil when backend is nil.
func TestNilBackendGetObjectLockConfiguration(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	config, err := backend.GetObjectLockConfiguration(ctx, "test-bucket")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if config != nil {
		t.Errorf("expected nil config, got %v", config)
	}
}

// TestNilBackendPutObjectLockConfiguration verifies PutObjectLockConfiguration returns nil when backend is nil.
func TestNilBackendPutObjectLockConfiguration(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()
	config := []byte(`{"ObjectLockEnabled":"Enabled"}`)

	err := backend.PutObjectLockConfiguration(ctx, "test-bucket", config)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendGetObjectRetention verifies GetObjectRetention returns nil, nil when backend is nil.
func TestNilBackendGetObjectRetention(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	retention, err := backend.GetObjectRetention(ctx, "test-bucket", "test-key")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if retention != nil {
		t.Errorf("expected nil retention, got %v", retention)
	}
}

// TestNilBackendPutObjectRetention verifies PutObjectRetention returns nil when backend is nil.
func TestNilBackendPutObjectRetention(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()
	retention := []byte(`{"Mode":"GOVERNANCE"}`)

	err := backend.PutObjectRetention(ctx, "test-bucket", "test-key", retention)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendGetObjectLegalHold verifies GetObjectLegalHold returns nil, nil when backend is nil.
func TestNilBackendGetObjectLegalHold(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	legalHold, err := backend.GetObjectLegalHold(ctx, "test-bucket", "test-key")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if legalHold != nil {
		t.Errorf("expected nil legal hold, got %v", legalHold)
	}
}

// TestNilBackendPutObjectLegalHold verifies PutObjectLegalHold returns nil when backend is nil.
func TestNilBackendPutObjectLegalHold(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()
	legalHold := []byte(`{"Status":"ON"}`)

	err := backend.PutObjectLegalHold(ctx, "test-bucket", "test-key", legalHold)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// TestNilBackendListObjectVersions verifies ListObjectVersions returns empty result with nil error when backend is nil.
func TestNilBackendListObjectVersions(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	result, err := backend.ListObjectVersions(ctx, "test-bucket", "prefix/", "/", "key-marker", "version-marker", 1000)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil ListObjectVersionsResult")
	}
	if len(result.Versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(result.Versions))
	}
	if result.IsTruncated {
		t.Error("expected IsTruncated to be false")
	}
	if result.NextKeyMarker != "" {
		t.Errorf("expected empty NextKeyMarker, got %q", result.NextKeyMarker)
	}
	if result.NextVersionIDMarker != "" {
		t.Errorf("expected empty NextVersionIDMarker, got %q", result.NextVersionIDMarker)
	}
	if len(result.CommonPrefixes) != 0 {
		t.Errorf("expected 0 common prefixes, got %d", len(result.CommonPrefixes))
	}
}

// TestNilBackendHeadVersion verifies HeadVersion returns nil, nil when backend is nil.
func TestNilBackendHeadVersion(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	info, err := backend.HeadVersion(ctx, "test-bucket", "test-key", "version-id")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if info != nil {
		t.Errorf("expected nil ObjectInfo, got %+v", info)
	}
}

// TestNilBackendImplementsInterface verifies NilBackend implements the Backend interface.
func TestNilBackendImplementsInterface(t *testing.T) {
	var _ Backend = NewNilBackend()
}

// TestNilBackendConcurrentOperations verifies NilBackend handles concurrent operations safely.
func TestNilBackendConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()
	concurrency := 100

	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			// Mix of different operations
			switch id % 5 {
			case 0:
				_, _, _ = backend.Get(ctx, "bucket", "key")
			case 1:
				_ = backend.Put(ctx, "bucket", "key", nil, 0, nil)
			case 2:
				_ = backend.Delete(ctx, "bucket", "key")
			case 3:
				_, _ = backend.ListBuckets(ctx)
			case 4:
				_, _ = backend.List(ctx, "bucket", "", "", "", 100)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < concurrency; i++ {
		select {
		case <-done:
			// OK
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent operations did not complete in time")
		}
	}
}

// TestNilBackendListObjectsParameters verifies ListObjects handles all parameter combinations.
func TestNilBackendListObjectsParameters(t *testing.T) {
	ctx := context.Background()
	backend := NewNilBackend()

	tests := []struct {
		name              string
		bucket            string
		prefix            string
		delimiter         string
		continuationToken string
		maxKeys           int
	}{
		{"empty parameters", "", "", "", "", 0},
		{"with prefix", "bucket", "prefix/", "", "", 100},
		{"with delimiter", "bucket", "", "/", "", 100},
		{"with continuation", "bucket", "", "", "token", 100},
		{"with max keys", "bucket", "", "", "", 1000},
		{"all parameters", "bucket", "prefix/", "/", "token", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := backend.List(ctx, tt.bucket, tt.prefix, tt.delimiter, tt.continuationToken, tt.maxKeys)

			if err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
			if result == nil {
				t.Fatal("expected non-nil ListResult")
			}
			if len(result.Objects) != 0 {
				t.Errorf("expected 0 objects, got %d", len(result.Objects))
			}
			if result.IsTruncated {
				t.Error("expected IsTruncated to be false")
			}
		})
	}
}
