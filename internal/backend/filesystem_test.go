// Package backend provides tests for the filesystem backend implementation.
package backend

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// TestFSBackend_PutGet tests basic Put and Get operations.
func TestFSBackend_PutGet(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "armor-fs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create filesystem backend
	fs, err := NewFSBackend(FSConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create FS backend: %v", err)
	}

	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-object.txt"
	content := "Hello, ARMOR filesystem backend!"
	meta := map[string]string{
		"Content-Type":                    "text/plain",
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-plaintext-size": "35",
	}

	// Create bucket
	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Put object
	body := strings.NewReader(content)
	if err := fs.Put(ctx, bucket, key, body, int64(len(content)), meta); err != nil {
		t.Fatalf("Failed to put object: %v", err)
	}

	// Get object
	rc, info, err := fs.Get(ctx, bucket, key)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}
	defer rc.Close()

	// Verify content
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Failed to read object: %v", err)
	}
	if string(data) != content {
		t.Errorf("Content mismatch: got %q, want %q", string(data), content)
	}

	// Verify metadata
	if info.Key != key {
		t.Errorf("Key mismatch: got %q, want %q", info.Key, key)
	}
	if info.Size != 35 { // Plaintext size
		t.Errorf("Size mismatch: got %d, want %d", info.Size, 35)
	}
	if !info.IsARMOREncrypted {
		t.Error("Expected ARMOR encrypted flag to be true")
	}

	// Head object
	headInfo, err := fs.Head(ctx, bucket, key)
	if err != nil {
		t.Fatalf("Failed to head object: %v", err)
	}
	if headInfo.Key != key {
		t.Errorf("Head key mismatch: got %q, want %q", headInfo.Key, key)
	}

	// Delete object
	if err := fs.Delete(ctx, bucket, key); err != nil {
		t.Fatalf("Failed to delete object: %v", err)
	}

	// Verify deletion
	_, _, err = fs.Get(ctx, bucket, key)
	if err == nil {
		t.Error("Expected error getting deleted object, got nil")
	}
}

// TestFSBackend_List tests List operations with prefix and delimiter.
func TestFSBackend_List(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "armor-fs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fs, err := NewFSBackend(FSConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create FS backend: %v", err)
	}

	ctx := context.Background()
	bucket := "test-bucket"

	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Create test objects with hierarchical structure
	objects := []struct {
		key     string
		content string
	}{
		{"dir1/file1.txt", "content1"},
		{"dir1/file2.txt", "content2"},
		{"dir2/file3.txt", "content3"},
		{"root.txt", "root content"},
	}

	for _, obj := range objects {
		body := strings.NewReader(obj.content)
		meta := map[string]string{"Content-Type": "text/plain"}
		if err := fs.Put(ctx, bucket, obj.key, body, int64(len(obj.content)), meta); err != nil {
			t.Fatalf("Failed to put object %s: %v", obj.key, err)
		}
	}

	// Test list with prefix
	result, err := fs.List(ctx, bucket, "dir1/", "", "", 0)
	if err != nil {
		t.Fatalf("Failed to list with prefix: %v", err)
	}
	if len(result.Objects) != 2 {
		t.Errorf("Expected 2 objects with prefix 'dir1/', got %d", len(result.Objects))
	}

	// Test list with delimiter
	result, err = fs.List(ctx, bucket, "", "/", "", 0)
	if err != nil {
		t.Fatalf("Failed to list with delimiter: %v", err)
	}
	if len(result.CommonPrefixes) != 2 {
		t.Errorf("Expected 2 common prefixes, got %d: %v", len(result.CommonPrefixes), result.CommonPrefixes)
	}
}

// TestFSBackend_MultipartUpload tests multipart upload operations.
func TestFSBackend_MultipartUpload(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "armor-fs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fs, err := NewFSBackend(FSConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create FS backend: %v", err)
	}

	ctx := context.Background()
	bucket := "test-bucket"
	key := "multipart-test.bin"

	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	meta := map[string]string{"Content-Type": "application/octet-stream"}

	// Create multipart upload
	uploadID, err := fs.CreateMultipartUpload(ctx, bucket, key, meta)
	if err != nil {
		t.Fatalf("Failed to create multipart upload: %v", err)
	}
	if uploadID == "" {
		t.Fatal("Got empty upload ID")
	}

	// Upload parts
	partSize := int64(1024 * 1024) // 1MB parts
	var parts []CompletedPart
	for i := int32(1); i <= 3; i++ {
		content := make([]byte, partSize)
		for j := range content {
			content[j] = byte(i)
		}
		body := strings.NewReader(string(content))

		etag, err := fs.UploadPart(ctx, bucket, key, uploadID, i, body, partSize)
		if err != nil {
			t.Fatalf("Failed to upload part %d: %v", i, err)
		}
		if etag == "" {
			t.Errorf("Got empty ETag for part %d", i)
		}

		parts = append(parts, CompletedPart{PartNumber: i, ETag: etag})
	}

	// List parts
	listResult, err := fs.ListParts(ctx, bucket, key, uploadID)
	if err != nil {
		t.Fatalf("Failed to list parts: %v", err)
	}
	if len(listResult.Parts) != 3 {
		t.Errorf("Expected 3 parts, got %d", len(listResult.Parts))
	}

	// Complete multipart upload
	finalETag, err := fs.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts)
	if err != nil {
		t.Fatalf("Failed to complete multipart upload: %v", err)
	}
	if finalETag == "" {
		t.Error("Got empty final ETag")
	}

	// Verify the final object
	info, err := fs.Head(ctx, bucket, key)
	if err != nil {
		t.Fatalf("Failed to head completed object: %v", err)
	}
	expectedSize := partSize * 3
	if info.Size != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, info.Size)
	}

	// Clean up
	if err := fs.Delete(ctx, bucket, key); err != nil {
		t.Fatalf("Failed to delete object: %v", err)
	}
}

// TestFSBackend_Copy tests copy operations including cross-bucket.
func TestFSBackend_Copy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "armor-fs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fs, err := NewFSBackend(FSConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create FS backend: %v", err)
	}

	ctx := context.Background()
	bucket1 := "bucket1"
	bucket2 := "bucket2"

	for _, bucket := range []string{bucket1, bucket2} {
		if err := fs.CreateBucket(ctx, bucket); err != nil {
			t.Fatalf("Failed to create bucket %s: %v", bucket, err)
		}
	}

	srcKey := "source.txt"
	dstKey := "destination.txt"
	content := "copy test content"
	meta := map[string]string{"Content-Type": "text/plain"}

	// Put source object
	body := strings.NewReader(content)
	if err := fs.Put(ctx, bucket1, srcKey, body, int64(len(content)), meta); err != nil {
		t.Fatalf("Failed to put source object: %v", err)
	}

	// Test same-bucket copy
	if err := fs.Copy(ctx, bucket1, srcKey, bucket1, dstKey, nil, false); err != nil {
		t.Fatalf("Failed to copy in same bucket: %v", err)
	}

	// Verify copy
	rc, info, err := fs.Get(ctx, bucket1, dstKey)
	if err != nil {
		t.Fatalf("Failed to get copied object: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Failed to read copied object: %v", err)
	}
	if string(data) != content {
		t.Errorf("Copy content mismatch: got %q, want %q", string(data), content)
	}
	if info.ContentType != "text/plain" {
		t.Errorf("ContentType not preserved: got %q", info.ContentType)
	}

	// Test cross-bucket copy
	crossBucketKey := "cross-bucket.txt"
	if err := fs.Copy(ctx, bucket1, srcKey, bucket2, crossBucketKey, nil, false); err != nil {
		t.Fatalf("Failed to copy across buckets: %v", err)
	}

	// Verify cross-bucket copy
	rc, _, err = fs.Get(ctx, bucket2, crossBucketKey)
	if err != nil {
		t.Fatalf("Failed to get cross-bucket copy: %v", err)
	}
	defer rc.Close()

	data, err = io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Failed to read cross-bucket copy: %v", err)
	}
	if string(data) != content {
		t.Errorf("Cross-bucket copy content mismatch: got %q, want %q", string(data), content)
	}
}

// TestFSBackend_BucketOperations tests bucket management operations.
func TestFSBackend_BucketOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "armor-fs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fs, err := NewFSBackend(FSConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create FS backend: %v", err)
	}

	ctx := context.Background()

	// List buckets initially (should be empty)
	buckets, err := fs.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("Failed to list buckets: %v", err)
	}
	if len(buckets) != 0 {
		t.Errorf("Expected 0 buckets initially, got %d", len(buckets))
	}

	// Create bucket
	bucketName := "test-bucket"
	if err := fs.CreateBucket(ctx, bucketName); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Head bucket (should exist)
	if err := fs.HeadBucket(ctx, bucketName); err != nil {
		t.Errorf("Failed to head bucket: %v", err)
	}

	// List buckets (should have 1)
	buckets, err = fs.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("Failed to list buckets after creation: %v", err)
	}
	if len(buckets) != 1 {
		t.Errorf("Expected 1 bucket after creation, got %d", len(buckets))
	}
	if buckets[0].Name != bucketName {
		t.Errorf("Bucket name mismatch: got %q, want %q", buckets[0].Name, bucketName)
	}

	// Try to delete non-empty bucket (should fail)
	if err := fs.Put(ctx, bucketName, "test.txt", strings.NewReader("test"), 4, nil); err != nil {
		t.Fatalf("Failed to put test object: %v", err)
	}
	err = fs.DeleteBucket(ctx, bucketName)
	if err == nil {
		t.Error("Expected error deleting non-empty bucket, got nil")
	}

	// Delete object and then bucket
	if err := fs.Delete(ctx, bucketName, "test.txt"); err != nil {
		t.Fatalf("Failed to delete test object: %v", err)
	}
	if err := fs.DeleteBucket(ctx, bucketName); err != nil {
		t.Errorf("Failed to delete empty bucket: %v", err)
	}

	// Verify bucket is gone
	err = fs.HeadBucket(ctx, bucketName)
	if err == nil {
		t.Error("Expected error heading deleted bucket, got nil")
	}
}

// TestFSBackend_GetRange tests range read operations.
func TestFSBackend_GetRange(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "armor-fs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fs, err := NewFSBackend(FSConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create FS backend: %v", err)
	}

	ctx := context.Background()
	bucket := "test-bucket"
	key := "range-test.txt"
	content := "0123456789"

	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	body := strings.NewReader(content)
	if err := fs.Put(ctx, bucket, key, body, int64(len(content)), nil); err != nil {
		t.Fatalf("Failed to put object: %v", err)
	}

	// Test range read: bytes 2-6 (should return "23456")
	rc, err := fs.GetRange(ctx, bucket, key, 2, 5)
	if err != nil {
		t.Fatalf("Failed to get range: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Failed to read range: %v", err)
	}
	if string(data) != "23456" {
		t.Errorf("Range content mismatch: got %q, want %q", string(data), "23456")
	}
}

// TestFSBackend_DeleteObjects tests batch delete operations.
func TestFSBackend_DeleteObjects(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "armor-fs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fs, err := NewFSBackend(FSConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create FS backend: %v", err)
	}

	ctx := context.Background()
	bucket := "test-bucket"

	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Create multiple objects
	keys := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, key := range keys {
		body := strings.NewReader("content")
		if err := fs.Put(ctx, bucket, key, body, 7, nil); err != nil {
			t.Fatalf("Failed to put object %s: %v", key, err)
		}
	}

	// Delete multiple objects
	if err := fs.DeleteObjects(ctx, bucket, keys); err != nil {
		t.Fatalf("Failed to delete objects: %v", err)
	}

	// Verify all objects are deleted
	for _, key := range keys {
		_, _, err := fs.Get(ctx, bucket, key)
		if err == nil {
			t.Errorf("Expected error getting deleted object %s, got nil", key)
		}
	}
}

// TestFSBackend_ListMultipartUploads tests listing active multipart uploads.
func TestFSBackend_ListMultipartUploads(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "armor-fs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fs, err := NewFSBackend(FSConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create FS backend: %v", err)
	}

	ctx := context.Background()
	bucket := "test-bucket"

	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Create multiple multipart uploads
	uploadIDs := []string{}
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("multipart-%d.bin", i)
		uploadID, err := fs.CreateMultipartUpload(ctx, bucket, key, nil)
		if err != nil {
			t.Fatalf("Failed to create multipart upload %d: %v", i, err)
		}
		uploadIDs = append(uploadIDs, uploadID)
	}

	// List multipart uploads
	result, err := fs.ListMultipartUploads(ctx, bucket)
	if err != nil {
		t.Fatalf("Failed to list multipart uploads: %v", err)
	}

	if len(result.Uploads) != 3 {
		t.Errorf("Expected 3 multipart uploads, got %d", len(result.Uploads))
	}

	// Clean up
	for _, uploadID := range uploadIDs {
		// Find the key for this upload ID (from our naming pattern)
		for _, upload := range result.Uploads {
			if upload.UploadID == uploadID {
				if err := fs.AbortMultipartUpload(ctx, bucket, upload.Key, uploadID); err != nil {
					t.Fatalf("Failed to abort upload %s: %v", uploadID, err)
				}
				break
			}
		}
	}
}

// TestFSBackend_ARMORMetadata tests ARMOR-specific metadata handling.
func TestFSBackend_ARMORMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "armor-fs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fs, err := NewFSBackend(FSConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create FS backend: %v", err)
	}

	ctx := context.Background()
	bucket := "armor-bucket"
	key := "encrypted.bin"

	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Create ARMOR metadata
	armorMeta := &ARMORMetadata{
		Version:       1,
		BlockSize:     65536,
		PlaintextSize: 1024,
		ContentType:   "application/octet-stream",
		IV:            []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		WrappedDEK:    []byte{21, 22, 23, 24, 25, 26, 27, 28},
		PlaintextSHA:  "abc123",
		ETag:          "etag-value",
	}

	meta := armorMeta.ToMetadata()

	// Write encrypted object (ciphertext is larger than plaintext in real scenario)
	ciphertext := make([]byte, 2048) // Simulate ciphertext
	body := strings.NewReader(string(ciphertext))

	if err := fs.Put(ctx, bucket, key, body, int64(len(ciphertext)), meta); err != nil {
		t.Fatalf("Failed to put ARMOR object: %v", err)
	}

	// Verify metadata is preserved
	info, err := fs.Head(ctx, bucket, key)
	if err != nil {
		t.Fatalf("Failed to head ARMOR object: %v", err)
	}

	if !info.IsARMOREncrypted {
		t.Error("Expected ARMOR encrypted flag to be true")
	}

	if info.Size != 1024 { // Should report plaintext size
		t.Errorf("Expected plaintext size 1024, got %d", info.Size)
	}

	// Verify ARMOR metadata can be parsed back
	parsed, ok := ParseARMORMetadata(info.Metadata)
	if !ok {
		t.Fatal("Failed to parse ARMOR metadata")
	}

	if parsed.Version != 1 {
		t.Errorf("Version mismatch: got %d, want %d", parsed.Version, 1)
	}
	if parsed.BlockSize != 65536 {
		t.Errorf("BlockSize mismatch: got %d, want %d", parsed.BlockSize, 65536)
	}
	if parsed.PlaintextSize != 1024 {
		t.Errorf("PlaintextSize mismatch: got %d, want %d", parsed.PlaintextSize, 1024)
	}
}
