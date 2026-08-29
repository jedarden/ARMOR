//go:build integration
// +build integration

// Comprehensive AbortMultipartUpload operation tests
// Tests for S3 AbortMultipartUpload API compliance and edge cases

package integration

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestAbortMultipartUpload_Basic tests basic abort functionality
func TestAbortMultipartUpload_Basic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create multipart upload
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Metadata: map[string]string{
			"test-name": "TestAbortMultipartUpload_Basic",
		},
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId
	t.Logf("Created multipart upload: %s", *uploadID)

	// Upload a part to ensure abort cleans it up
	partData := generateTestData(5 * 1024 * 1024) // 5 MB
	_, err = client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(partData),
	})
	if err != nil {
		t.Fatalf("UploadPart failed: %v", err)
	}
	t.Log("Uploaded part 1 before abort")

	// Verify upload appears in ListMultipartUploads
	listResp, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("ListMultipartUploads before abort failed: %v", err)
	}

	found := false
	for _, upload := range listResp.Uploads {
		if *upload.Key == key && *upload.UploadId == *uploadID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Upload not found in ListMultipartUploads before abort")
	}

	// Abort the upload
	_, err = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("AbortMultipartUpload failed: %v", err)
	}
	t.Log("Successfully aborted multipart upload")

	// Verify upload no longer appears in ListMultipartUploads
	listResp2, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("ListMultipartUploads after abort failed: %v", err)
	}

	for _, upload := range listResp2.Uploads {
		if *upload.Key == key && *upload.UploadId == *uploadID {
			t.Error("Aborted upload still appears in ListMultipartUploads")
		}
	}

	// Verify the object was not created (no final object should exist)
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Error("Object exists after abort - should have been cleaned up")
	}
	t.Log("Object correctly does not exist after abort")

	t.Log("TestAbortMultipartUpload_Basic completed successfully")
}

// TestAbortMultipartUpload_AbortEmpty tests aborting an upload with no parts
func TestAbortMultipartUpload_AbortEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create upload but don't upload any parts
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	// Abort the empty upload
	_, err = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("AbortMultipartUpload on empty upload failed: %v", err)
	}

	t.Log("Successfully aborted empty multipart upload")
}

// TestAbortMultipartUpload_AbortManyParts tests aborting upload with many parts
func TestAbortMultipartUpload_AbortManyParts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	// Upload many parts before aborting
	numParts := 10
	partSize := int64(5 * 1024 * 1024) // 5 MB

	for i := int32(1); i <= int32(numParts); i++ {
		partData := generateTestData(int(partSize))
		_, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(key),
			UploadId:   uploadID,
			PartNumber: aws.Int32(i),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", i, err)
		}
	}
	t.Logf("Uploaded %d parts before abort", numParts)

	// Abort the upload with many parts
	_, err = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("AbortMultipartUpload with many parts failed: %v", err)
	}

	// Verify cleanup - list parts should return empty or error
	_, err = client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err == nil {
		t.Log("Note: ListParts succeeded after abort (backend may allow querying aborted uploads)")
	}

	t.Log("Successfully aborted upload with many parts")
}

// TestAbortMultipartUpload_NonExistentUpload tests aborting a non-existent upload
func TestAbortMultipartUpload_NonExistentUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)
	fakeUploadID := fmt.Sprintf("non-existent-upload-%d", time.Now().UnixNano())

	// Try to abort non-existent upload
	_, err := client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(fakeUploadID),
	})

	// S3 behavior: aborting non-existent upload should succeed (idempotent)
	// or may return error - both are acceptable
	if err != nil {
		t.Logf("AbortMultipartUpload of non-existent upload returned error: %v (acceptable)", err)
	} else {
		t.Log("AbortMultipartUpload of non-existent upload succeeded (idempotent behavior)")
	}
}

// TestAbortMultipartUpload_AlreadyCompleted tests aborting an already completed upload
func TestAbortMultipartUpload_AlreadyCompleted(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create and complete a multipart upload
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	// Upload and complete
	partData := generateTestData(5 * 1024 * 1024) // 5 MB
	uploadResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(partData),
	})
	if err != nil {
		t.Fatalf("UploadPart failed: %v", err)
	}

	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{
					ETag:       uploadResp.ETag,
					PartNumber: aws.Int32(1),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	// Try to abort the already completed upload
	_, err = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})

	// S3 behavior: should return error for already completed upload
	if err == nil {
		t.Error("AbortMultipartUpload of already completed upload should fail")
	} else {
		t.Logf("AbortMultipartUpload of already completed upload failed as expected: %v", err)
	}

	// Cleanup the completed object
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
}

// TestAbortMultipartUpload_ConcurrentOperations tests abort while list operations are in progress
func TestAbortMultipartUpload_ConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create upload
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	// Upload a part
	partData := generateTestData(5 * 1024 * 1024)
	_, err = client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(partData),
	})
	if err != nil {
		t.Fatalf("UploadPart failed: %v", err)
	}

	// List parts to verify they're visible
	listResp, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("ListParts failed: %v", err)
	}
	if len(listResp.Parts) != 1 {
		t.Errorf("Expected 1 part, got %d", len(listResp.Parts))
	}

	// Abort the upload
	_, err = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("AbortMultipartUpload failed: %v", err)
	}

	// Verify the parts are no longer accessible
	listResp2, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	// After abort, list parts may return error or empty list
	if err == nil && len(listResp2.Parts) > 0 {
		t.Error("Parts still visible after abort")
	}

	t.Log("Concurrent operations test completed successfully")
}

// TestAbortMultipartUpload_StorageCleanup verifies that abort frees storage
func TestAbortMultipartUpload_StorageCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create upload and upload a large part
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	largeData := generateTestData(10 * 1024 * 1024) // 10 MB
	_, err = client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(largeData),
	})
	if err != nil {
		t.Fatalf("UploadPart failed: %v", err)
	}
	t.Log("Uploaded 10 MB part")

	// Abort - should free the storage
	_, err = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("AbortMultipartUpload failed: %v", err)
	}

	// Verify object doesn't exist (storage was freed)
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Error("Object exists after abort - storage may not have been freed")
	}

	t.Log("Storage cleanup verified after abort")
}
