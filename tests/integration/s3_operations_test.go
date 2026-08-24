//go:build integration
// +build integration

// Comprehensive S3 API operation tests
// Tests for S3 operations that were not covered in the original test suite

package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestListParts tests listing parts of a multipart upload
func TestListParts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create multipart upload with multiple parts
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Metadata: map[string]string{
			"test-name": "TestListParts",
		},
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId
	t.Logf("Created multipart upload: %s", *uploadID)

	// Cleanup function
	cleanup := func() {
		_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(key),
			UploadId: uploadID,
		})
	}

	// Upload multiple parts for comprehensive testing
	partSize := int64(5 * 1024 * 1024) // 5 MB per part
	numParts := 5
	uploadedParts := []types.CompletedPart{}

	for i := int32(1); i <= int32(numParts); i++ {
		partData := generateTestData(int(partSize))
		partData[0] = byte(i) // Mark each part uniquely

		uploadResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(key),
			UploadId:   uploadID,
			PartNumber: aws.Int32(i),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			cleanup()
			t.Fatalf("UploadPart %d failed: %v", i, err)
		}

		uploadedParts = append(uploadedParts, types.CompletedPart{
			ETag:       uploadResp.ETag,
			PartNumber: aws.Int32(i),
		})
		t.Logf("Uploaded part %d, ETag: %s", i, *uploadResp.ETag)
	}

	// Test 1: List all parts
	listResp, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		cleanup()
		t.Fatalf("ListParts failed: %v", err)
	}

	if len(listResp.Parts) != numParts {
		cleanup()
		t.Errorf("ListParts returned %d parts, want %d", len(listResp.Parts), numParts)
	}

	// Verify part numbers are sequential
	for i, part := range listResp.Parts {
		expectedPartNumber := int32(i + 1)
		if *part.PartNumber != expectedPartNumber {
			cleanup()
			t.Errorf("Part number mismatch at index %d: got %d, want %d",
				i, *part.PartNumber, expectedPartNumber)
		}
		// Verify part size
		if *part.Size != partSize {
			cleanup()
			t.Errorf("Part %d size mismatch: got %d, want %d",
				*part.PartNumber, *part.Size, partSize)
		}
	}

	t.Logf("ListParts returned all %d parts correctly", numParts)

	// Test 2: List parts with max-parts parameter
	maxParts := int32(2)
	listResp2, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
		MaxParts: aws.Int32(maxParts),
	})
	if err != nil {
		cleanup()
		t.Fatalf("ListParts with MaxParts failed: %v", err)
	}

	if len(listResp2.Parts) > int(maxParts) {
		cleanup()
		t.Errorf("ListParts with MaxParts=%d returned %d parts", maxParts, len(listResp2.Parts))
	}

	if listResp2.NextPartNumberMarker == nil {
		cleanup()
		t.Error("ListParts with MaxParts should return NextPartNumberMarker")
	}

	t.Logf("ListParts with MaxParts=%d returned %d parts, NextPartNumberMarker: %s",
		maxParts, len(listResp2.Parts), *listResp2.NextPartNumberMarker)

	// Test 3: List parts with part-number-marker
	marker := int32(3)
	listResp3, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:           aws.String(bucket),
		Key:              aws.String(key),
		UploadId:         uploadID,
		PartNumberMarker: aws.String(strconv.Itoa(int(marker))),
	})
	if err != nil {
		cleanup()
		t.Fatalf("ListParts with PartNumberMarker failed: %v", err)
	}

	// Should only return parts after part 3
	for _, part := range listResp3.Parts {
		if *part.PartNumber <= marker {
			cleanup()
			t.Errorf("ListParts with PartNumberMarker=%d returned part %d", marker, *part.PartNumber)
		}
	}

	t.Logf("ListParts with PartNumberMarker=%d returned %d parts (skipping first %d)",
		marker, len(listResp3.Parts), marker)

	// Cleanup
	cleanup()
	t.Log("TestListParts completed successfully")
}

// TestListParts_EmptyUpload tests listing parts of a newly created upload with no parts
func TestListParts_EmptyUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create multipart upload but don't upload any parts
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	defer func() {
		_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(key),
			UploadId: uploadID,
		})
	}()

	// List parts of empty upload
	listResp, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("ListParts on empty upload failed: %v", err)
	}

	if len(listResp.Parts) != 0 {
		t.Errorf("ListParts on empty upload returned %d parts, want 0", len(listResp.Parts))
	}

	t.Log("ListParts on empty upload returned empty list correctly")
}

// TestListParts_NonExistentUpload tests listing parts of a non-existent upload
func TestListParts_NonExistentUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)
	fakeUploadID := "non-existent-upload-id"

	_, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(fakeUploadID),
	})

	if err == nil {
		t.Error("ListParts with non-existent upload ID should fail")
	}

	t.Logf("ListParts with non-existent upload ID failed as expected: %v", err)
}

// TestListMultipartUploads tests listing in-progress multipart uploads
func TestListMultipartUploads(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Create multiple multipart uploads
	numUploads := 3
	uploads := []struct {
		key      string
		uploadID *string
	}{
		{key: generateTestKey(t) + "-upload1"},
		{key: generateTestKey(t) + "-upload2"},
		{key: generateTestKey(t) + "-upload3"},
	}

	cleanup := func() {
		for _, u := range uploads {
			if u.uploadID != nil {
				_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
					Bucket:   aws.String(bucket),
					Key:      aws.String(u.key),
					UploadId: u.uploadID,
				})
			}
		}
	}

	// Create uploads
	for i := range uploads {
		createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(uploads[i].key),
			Metadata: map[string]string{
				"test-name": fmt.Sprintf("TestListMultipartUploads-%d", i),
			},
		})
		if err != nil {
			cleanup()
			t.Fatalf("CreateMultipartUpload %d failed: %v", i, err)
		}
		uploads[i].uploadID = createResp.UploadId
		t.Logf("Created upload %d: key=%s, uploadID=%s", i, uploads[i].key, *createResp.UploadId)
	}

	// Test 1: List all multipart uploads
	listResp, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		cleanup()
		t.Fatalf("ListMultipartUploads failed: %v", err)
	}

	// Filter for our test uploads (may have other uploads from concurrent tests)
	ourUploads := 0
	for _, upload := range listResp.Uploads {
		for _, u := range uploads {
			if *upload.Key == u.key {
				ourUploads++
				if *upload.UploadId != *u.uploadID {
					cleanup()
					t.Errorf("UploadID mismatch for key %s: got %s, want %s",
						*upload.Key, *upload.UploadId, *u.uploadID)
				}
			}
		}
	}

	if ourUploads != numUploads {
		cleanup()
		t.Errorf("ListMultipartUploads returned %d of our uploads, want %d", ourUploads, numUploads)
	}

	t.Logf("ListMultipartUploads returned all %d test uploads", numUploads)

	// Test 2: List uploads with max-uploads parameter
	maxUploads := int32(2)
	listResp2, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket:    aws.String(bucket),
		MaxUploads: aws.Int32(maxUploads),
	})
	if err != nil {
		cleanup()
		t.Fatalf("ListMultipartUploads with MaxUploads failed: %v", err)
	}

	if len(listResp2.Uploads) > int(maxUploads) {
		cleanup()
		t.Errorf("ListMultipartUploads with MaxUploads=%d returned %d uploads",
			maxUploads, len(listResp2.Uploads))
	}

	if listResp2.NextUploadIdMarker == nil && len(listResp2.Uploads) > 0 {
		cleanup()
		t.Error("ListMultipartUploads with MaxUploads should return NextUploadIdMarker when results are truncated")
	}

	t.Logf("ListMultipartUploads with MaxUploads=%d returned %d uploads",
		maxUploads, len(listResp2.Uploads))

	// Test 3: List uploads with prefix filter
	prefix := uploads[0].key[:len(uploads[0].key)-2] // Remove last 2 chars to get prefix
	listResp3, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		cleanup()
		t.Fatalf("ListMultipartUploads with Prefix failed: %v", err)
	}

	// Verify all returned uploads have the prefix
	for _, upload := range listResp3.Uploads {
		if !strings.HasPrefix(*upload.Key, prefix) {
			cleanup()
			t.Errorf("ListMultipartUploads with Prefix=%s returned upload with key %s",
				prefix, *upload.Key)
		}
	}

	t.Logf("ListMultipartUploads with Prefix=%s returned %d uploads", prefix, len(listResp3.Uploads))

	// Cleanup
	cleanup()
	t.Log("TestListMultipartUploads completed successfully")
}

// TestListMultipartUploads_EmptyBucket tests listing uploads when no uploads exist
func TestListMultipartUploads_EmptyBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Use a unique prefix that's unlikely to exist
	uniquePrefix := fmt.Sprintf("nonexistent-prefix-%d-", time.Now().UnixNano())

	listResp, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(uniquePrefix),
	})
	if err != nil {
		t.Fatalf("ListMultipartUploads failed: %v", err)
	}

	if len(listResp.Uploads) != 0 {
		t.Errorf("ListMultipartUploads with unique prefix returned %d uploads, want 0",
			len(listResp.Uploads))
	}

	t.Log("ListMultipartUploads with unique prefix returned empty list correctly")
}

// TestListMultipartUploads_WithKeyMarker tests pagination with key marker
func TestListMultipartUploads_WithKeyMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Create uploads with predictable key ordering
	uploads := []struct {
		key      string
		uploadID *string
	}{
		{key: fmt.Sprintf("test-marker/upload-a-%d", time.Now().UnixNano())},
		{key: fmt.Sprintf("test-marker/upload-b-%d", time.Now().UnixNano())},
		{key: fmt.Sprintf("test-marker/upload-c-%d", time.Now().UnixNano())},
	}

	cleanup := func() {
		for _, u := range uploads {
			if u.uploadID != nil {
				_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
					Bucket:   aws.String(bucket),
					Key:      aws.String(u.key),
					UploadId: u.uploadID,
				})
			}
		}
	}

	// Create uploads
	for i := range uploads {
		createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(uploads[i].key),
		})
		if err != nil {
			cleanup()
			t.Fatalf("CreateMultipartUpload %d failed: %v", i, err)
		}
		uploads[i].uploadID = createResp.UploadId
	}

	// List with key marker set to second upload
	marker := uploads[1].key
	listResp, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String("test-marker/"),
		KeyMarker: aws.String(marker),
	})
	if err != nil {
		cleanup()
		t.Fatalf("ListMultipartUploads with KeyMarker failed: %v", err)
	}

	// Should not return the marked upload or any before it
	for _, upload := range listResp.Uploads {
		if *upload.Key <= marker {
			cleanup()
			t.Errorf("ListMultipartUploads with KeyMarker=%s returned upload %s", marker, *upload.Key)
		}
	}

	t.Logf("ListMultipartUploads with KeyMarker returned %d uploads (after marker)", len(listResp.Uploads))

	cleanup()
	t.Log("TestListMultipartUploads_WithKeyMarker completed successfully")
}

// TestDeleteObjects_BulkDelete tests deleting multiple objects in a single request
func TestDeleteObjects_BulkDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Upload multiple objects
	numObjects := 5
	keys := []string{}
	for i := 0; i < numObjects; i++ {
		key := fmt.Sprintf("bulk-delete-test/object-%d-%d", i, time.Now().UnixNano())
		keys = append(keys, key)

		testData := generateTestData(1024)
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(testData),
		})
		if err != nil {
			t.Fatalf("PutObject for %s failed: %v", key, err)
		}
	}

	t.Logf("Uploaded %d objects for bulk delete test", numObjects)

	// Verify objects exist before deletion
	for _, key := range keys {
		_, err := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			t.Errorf("HeadObject for %s failed (object should exist): %v", key, err)
		}
	}

	// Delete all objects in one request
	deleteObjects := []types.ObjectIdentifier{}
	for _, key := range keys {
		deleteObjects = append(deleteObjects, types.ObjectIdentifier{
			Key: aws.String(key),
		})
	}

	deleteResp, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: deleteObjects,
		},
	})
	if err != nil {
		t.Fatalf("DeleteObjects failed: %v", err)
	}

	// Verify all objects were deleted
	if len(deleteResp.Deleted) != numObjects {
		t.Errorf("DeleteObjects deleted %d objects, want %d", len(deleteResp.Deleted), numObjects)
	}

	if len(deleteResp.Errors) > 0 {
		t.Errorf("DeleteObjects returned %d errors", len(deleteResp.Errors))
		for _, e := range deleteResp.Errors {
			t.Logf("Error: Key=%s, Code=%s, Message=%s", *e.Key, *e.Code, *e.Message)
		}
	}

	// Verify objects are gone
	for _, key := range keys {
		_, err := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err == nil {
			t.Errorf("Object %s still exists after DeleteObjects", key)
		}
	}

	t.Logf("DeleteObjects successfully deleted all %d objects", numObjects)
}

// TestDeleteObjects_QuietMode tests DeleteObjects with quiet mode (minimal response)
func TestDeleteObjects_QuietMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Upload test objects
	keys := []string{
		fmt.Sprintf("quiet-mode-test/object-1-%d", time.Now().UnixNano()),
		fmt.Sprintf("quiet-mode-test/object-2-%d", time.Now().UnixNano()),
	}

	for _, key := range keys {
		testData := generateTestData(512)
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(testData),
		})
		if err != nil {
			t.Fatalf("PutObject failed: %v", err)
		}
	}

	// Delete with quiet mode
	deleteObjects := []types.ObjectIdentifier{}
	for _, key := range keys {
		deleteObjects = append(deleteObjects, types.ObjectIdentifier{
			Key: aws.String(key),
		})
	}

	deleteResp, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: deleteObjects,
			Quiet:   aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatalf("DeleteObjects with quiet mode failed: %v", err)
	}

	// In quiet mode, Deleted should be empty or minimal
	// S3 specification says quiet mode omits successful deletions from response
	if len(deleteResp.Deleted) > 0 {
		t.Logf("Warning: Quiet mode returned %d deleted entries (S3 behavior varies)", len(deleteResp.Deleted))
	}

	t.Log("DeleteObjects quiet mode completed")
}

// TestDeleteObjects_NonExistentObjects tests deleting objects that don't exist
func TestDeleteObjects_NonExistentObjects(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Mix of existing and non-existent objects
	existingKey := fmt.Sprintf("mixed-delete-test/existing-%d", time.Now().UnixNano())
	nonExistentKey := fmt.Sprintf("mixed-delete-test/nonexistent-%d", time.Now().UnixNano())

	// Upload one object
	testData := generateTestData(512)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(existingKey),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}

	// Delete both
	deleteResp, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: []types.ObjectIdentifier{
				{Key: aws.String(existingKey)},
				{Key: aws.String(nonExistentKey)},
			},
		},
	})
	if err != nil {
		t.Fatalf("DeleteObjects failed: %v", err)
	}

	// Should succeed even if one object doesn't exist
	// Behavior varies: S3 may report non-existent as successful or as error
	t.Logf("DeleteObjects with mixed keys completed. Deleted: %d, Errors: %d",
		len(deleteResp.Deleted), len(deleteResp.Errors))

	if len(deleteResp.Errors) > 0 {
		t.Logf("Errors reported:")
		for _, e := range deleteResp.Errors {
			t.Logf("  Key=%s, Code=%s, Message=%s", *e.Key, *e.Code, *e.Message)
		}
	}
}

// TestDeleteObjects_EmptyList tests DeleteObjects with empty object list
func TestDeleteObjects_EmptyList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Delete with empty list
	deleteResp, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: []types.ObjectIdentifier{},
		},
	})

	if err != nil {
		t.Fatalf("DeleteObjects with empty list failed: %v", err)
	}

	if len(deleteResp.Deleted) != 0 {
		t.Errorf("DeleteObjects with empty list returned %d deleted entries", len(deleteResp.Deleted))
	}

	t.Log("DeleteObjects with empty list succeeded")
}

// Helper function to get environment variable with fallback
func getEnvOr(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
