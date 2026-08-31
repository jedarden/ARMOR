//go:build integration
// +build integration

// Tests for ADR-016: B2-Safe Multipart Metadata Finalization Protocol
// Tests for manifest object pattern in multipart uploads

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestMultipartManifestCreation tests that a manifest object is created during multipart completion
func TestMultipartManifestCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)
	manifestKey := key + ".armor-manifest"

	// Create multipart upload with v3 format
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Metadata: map[string]string{
			"test-name":    "TestMultipartManifestCreation",
			"armor-format": "3",
		},
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId
	t.Logf("Created multipart upload: %s", *uploadID)

	// Upload a single part (10 MB to meet minimum part size)
	partData := generateTestData(10 * 1024 * 1024) // 10 MB
	uploadPartResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(partData),
	})
	if err != nil {
		t.Fatalf("UploadPart failed: %v", err)
	}
	part1ETag := uploadPartResp.ETag
	t.Logf("Uploaded part 1, ETag: %s", *part1ETag)

	// Complete the multipart upload
	completeResp, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{
					ETag:       part1ETag,
					PartNumber: aws.Int32(1),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("Completed multipart upload, ETag: %s", *completeResp.ETag)

	// Verify manifest object exists
	headManifestResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(manifestKey),
	})
	if err != nil {
		t.Fatalf("Manifest object not found: %v", err)
	}
	t.Logf("Manifest object exists, Content-Type: %s", *headManifestResp.ContentType)

	// Verify manifest content type
	if *headManifestResp.ContentType != "application/x-armor-manifest+json" {
		t.Errorf("Manifest has wrong Content-Type: %s, expected application/x-armor-manifest+json", *headManifestResp.ContentType)
	}

	// Verify manifest metadata contains ARMOR fields
	if headManifestResp.Metadata["Armor-Version"] == "" {
		t.Error("Manifest missing x-amz-meta-armor-version")
	}
	if headManifestResp.Metadata["Armor-Ciphertext-Ref"] == "" {
		t.Error("Manifest missing x-amz-meta-armor-ciphertext-ref")
	}
	if headManifestResp.Metadata["Armor-Completed-At"] == "" {
		t.Error("Manifest missing x-amz-meta-armor-completed-at")
	}
	if headManifestResp.Metadata["Armor-Multipart"] != "true" {
		t.Error("Manifest missing x-amz-meta-armor-multipart flag")
	}

	// Verify ciphertext reference points to the correct object
	ciphertextRef := headManifestResp.Metadata["Armor-Ciphertext-Ref"]
	t.Logf("Manifest ciphertext-ref: %s", ciphertextRef)

	// Read manifest body to verify JSON structure
	getManifestResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(manifestKey),
	})
	if err != nil {
		t.Fatalf("Failed to get manifest body: %v", err)
	}
	defer getManifestResp.Body.Close()

	var manifestBody struct {
		CiphertextObject string            `json:"ciphertext_object"`
		UploadID         string            `json:"upload_id"`
		CompletedAt      string            `json:"completed_at"`
		Metadata         map[string]string `json:"metadata"`
	}

	manifestBytes, err := io.ReadAll(getManifestResp.Body)
	if err != nil {
		t.Fatalf("Failed to read manifest body: %v", err)
	}

	if err := json.Unmarshal(manifestBytes, &manifestBody); err != nil {
		t.Fatalf("Failed to parse manifest JSON: %v", err)
	}

	t.Logf("Manifest body: ciphertext_object=%s, upload_id=%s, completed_at=%s",
		manifestBody.CiphertextObject, manifestBody.UploadID, manifestBody.CompletedAt)

	if manifestBody.CiphertextObject == "" {
		t.Error("Manifest body missing ciphertext_object")
	}
	if manifestBody.UploadID == "" {
		t.Error("Manifest body missing upload_id")
	}
	if manifestBody.CompletedAt == "" {
		t.Error("Manifest body missing completed_at")
	}

	// Verify ciphertext object exists and can be read
	getCipherResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("Failed to get ciphertext object: %v", err)
	}
	defer getCipherResp.Body.Close()

	t.Logf("Ciphertext object size: %d", getCipherResp.ContentLength)

	// Verify ciphertext object has NO ARMOR metadata (it should be opaque encrypted data)
	if getCipherResp.Metadata["Armor-Version"] != "" {
		t.Error("Ciphertext object should not have x-amz-meta-armor-version (metadata should be in manifest only)")
	}
}

// TestMultipartManifestReadPath tests reading an object via manifest
func TestMultipartManifestReadPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Upload and complete multipart upload
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Metadata: map[string]string{
			"test-name":    "TestMultipartManifestReadPath",
			"armor-format": "3",
		},
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	// Upload a single part
	partData := generateTestData(10 * 1024 * 1024)
	uploadPartResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(partData),
	})
	if err != nil {
		t.Fatalf("UploadPart failed: %v", err)
	}

	// Complete the upload
	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{
					ETag:       uploadPartResp.ETag,
					PartNumber: aws.Int32(1),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	// Read the object back via ARMOR (should use manifest)
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer getResp.Body.Close()

	t.Logf("Object size: %d, Content-Type: %s", getResp.ContentLength, *getResp.ContentType)

	// Verify we got the decrypted plaintext
	downloadedData, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read downloaded data: %v", err)
	}

	if len(downloadedData) != len(partData) {
		t.Errorf("Downloaded data size mismatch: got %d, want %d", len(downloadedData), len(partData))
	}

	// Verify data integrity
	if !bytes.Equal(downloadedData, partData) {
		t.Error("Downloaded data does not match uploaded data")
	}

	// Compute SHA-256 of downloaded data
	downloadedSHA := sha256.Sum256(downloadedData)
	partDataSHA := sha256.Sum256(partData)

	if !bytes.Equal(downloadedSHA[:], partDataSHA[:]) {
		t.Error("Downloaded data SHA-256 does not match uploaded data SHA-256")
	}

	t.Logf("Successfully downloaded and verified %d bytes via manifest", len(downloadedData))
}

// TestMultipartManifestLargeObject tests manifest creation for a large object (> 5GB)
// This is the key test for ADR-016 - verifying that large objects work without CopyObject
func TestMultipartManifestLargeObject(t *testing.T) {
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
			"test-name":    "TestMultipartManifestLargeObject",
			"armor-format": "3",
		},
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	// Upload 3 parts of 10 MB each (30 MB total - not actually >5GB but tests the path)
	// Real >5GB tests would be too slow for CI
	const partSize = 10 * 1024 * 1024
	const numParts = 3

	var completedParts []types.CompletedPart
	plaintextData := make([]byte, 0, partSize*numParts)

	for i := 1; i <= numParts; i++ {
		partData := generateTestData(partSize)
		plaintextData = append(plaintextData, partData...)

		uploadPartResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(key),
			UploadId:   uploadID,
			PartNumber: aws.Int32(int32(i)),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", i, err)
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       uploadPartResp.ETag,
			PartNumber: aws.Int32(int32(i)),
		})
		t.Logf("Uploaded part %d/%d, ETag: %s", i, numParts, *uploadPartResp.ETag)
	}

	// Complete the multipart upload
	completeResp, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completedParts},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("Completed multipart upload, ETag: %s", *completeResp.ETag)

	// Verify manifest exists
	manifestKey := key + ".armor-manifest"
	headManifestResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(manifestKey),
	})
	if err != nil {
		t.Fatalf("Manifest object not found: %v", err)
	}

	t.Logf("Manifest exists for %s-byte object", headManifestResp.Metadata["Armor-Plaintext-Size"])

	// Verify the object can be read back
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer getResp.Body.Close()

	downloadedData, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read downloaded data: %v", err)
	}

	if len(downloadedData) != len(plaintextData) {
		t.Errorf("Downloaded data size mismatch: got %d, want %d", len(downloadedData), len(plaintextData))
	}

	// Verify SHA-256
	downloadedSHA := sha256.Sum256(downloadedData)
	plaintextSHA := sha256.Sum256(plaintextData)

	if !bytes.Equal(downloadedSHA[:], plaintextSHA[:]) {
		t.Error("Downloaded data SHA-256 does not match uploaded data SHA-256")
	}

	t.Logf("Successfully verified %d-byte multipart object with manifest", len(downloadedData))
}

// TestMultipartManifestStaleDetection tests detection of stale manifests
func TestMultipartManifestStaleDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// First upload: create and complete
	createResp1, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Metadata: map[string]string{
			"test-name":    "TestMultipartManifestStaleDetection-1",
			"armor-format": "3",
		},
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}

	partData := generateTestData(10 * 1024 * 1024)
	uploadPartResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   createResp1.UploadId,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(partData),
	})
	if err != nil {
		t.Fatalf("UploadPart failed: %v", err)
	}

	completeResp, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: createResp1.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{ETag: uploadPartResp.ETag, PartNumber: aws.Int32(1)},
			},
		},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("First upload completed, ETag: %s", *completeResp.ETag)

	// Wait a moment to ensure timestamp difference
	time.Sleep(2 * time.Second)

	// Second upload: overwrite with new data
	createResp2, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Metadata: map[string]string{
			"test-name":    "TestMultipartManifestStaleDetection-2",
			"armor-format": "3",
		},
	})
	if err != nil {
		t.Fatalf("Second CreateMultipartUpload failed: %v", err)
	}

	newPartData := generateTestData(10 * 1024 * 1024)
	uploadPartResp2, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   createResp2.UploadId,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(newPartData),
	})
	if err != nil {
		t.Fatalf("Second UploadPart failed: %v", err)
	}

	completeResp2, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: createResp2.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{ETag: uploadPartResp2.ETag, PartNumber: aws.Int32(1)},
			},
		},
	})
	if err != nil {
		t.Fatalf("Second CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("Second upload completed, ETag: %s", *completeResp2.ETag)

	// Read the object - should get the new data, not stale old data
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer getResp.Body.Close()

	downloadedData, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read downloaded data: %v", err)
	}

	// Should match the NEW data, not the old data
	if !bytes.Equal(downloadedData, newPartData) {
		t.Error("Downloaded data does not match the NEW uploaded data (may have returned stale data)")
	}

	t.Logf("Successfully verified fresh data returned, not stale manifest")
}

// TestMultipartManifestIdempotentCompletion tests that retrying completion doesn't create duplicates
func TestMultipartManifestIdempotentCompletion(t *testing.T) {
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
			"test-name":    "TestMultipartManifestIdempotentCompletion",
			"armor-format": "3",
		},
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	// Upload a single part
	partData := generateTestData(10 * 1024 * 1024)
	uploadPartResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(partData),
	})
	if err != nil {
		t.Fatalf("UploadPart failed: %v", err)
	}

	// Complete the multipart upload
	completeResp, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{ETag: uploadPartResp.ETag, PartNumber: aws.Int32(1)},
			},
		},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	etag1 := *completeResp.ETag
	t.Logf("First completion, ETag: %s", etag1)

	// Try to complete again (idempotent - should succeed or return same result)
	completeResp2, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{ETag: uploadPartResp.ETag, PartNumber: aws.Int32(1)},
			},
		},
	})
	// This might fail with NoSuchUpload (expected) or succeed (also OK)
	if err != nil {
		t.Logf("Second completion failed (expected): %v", err)
		// Verify object still exists and is readable
	} else {
		etag2 := *completeResp2.ETag
		t.Logf("Second completion succeeded, ETag: %s", etag2)
		// If it succeeds, ETags should match
		if etag1 != etag2 {
			t.Errorf("ETag mismatch on idempotent completion: %s != %s", etag1, etag2)
		}
	}

	// Verify object is still readable and correct
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer getResp.Body.Close()

	downloadedData, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read downloaded data: %v", err)
	}

	if !bytes.Equal(downloadedData, partData) {
		t.Error("Downloaded data does not match uploaded data after idempotent completion")
	}

	t.Logf("Idempotent completion test passed - object is readable and correct")
}
