//go:build integration
// +build integration

// Comprehensive v3 multipart upload tests
// Tests for S3 CompleteMultipartUpload with v3 format (gzip-compressed JSON sidecar)

package integration

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// sha256Sum computes the SHA-256 hash of the input data
func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// TestCompleteMultipartUpload_V3_SinglePart tests a 1-part upload with v3 format
func TestCompleteMultipartUpload_V3_SinglePart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create multipart upload with v3 format
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Metadata: map[string]string{
			"test-name":     "TestCompleteMultipartUpload_V3_SinglePart",
			"armor-format":  "3",
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
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        uploadID,
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
	t.Logf("Completed multipart upload, Location: %s, ETag: %s", *completeResp.Location, *completeResp.ETag)

	// Verify the object exists and has v3 metadata
	headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}

	// Check metadata contains version 3
	version := headResp.Metadata["Armor-Version"]
	if version != "3" {
		t.Errorf("Expected ARMOR version 3, got: %s", version)
	}

	// Check multipart flag is set
	multipartFlag := headResp.Metadata["Armor-Multipart"]
	if multipartFlag != "true" {
		t.Errorf("Expected ARMOR multipart flag 'true', got: %s", multipartFlag)
	}

	// Verify the v3 sidecar exists and can be parsed
	sidecarKey := fmt.Sprintf(".armor/hmac/%x", sha256Sum([]byte(key)))
	sidecarResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sidecarKey),
	})
	if err != nil {
		t.Fatalf("Failed to get v3 sidecar: %v", err)
	}
	defer sidecarResp.Body.Close()

	// Decompress and parse the gzip-compressed JSON sidecar
	gz, err := gzip.NewReader(sidecarResp.Body)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	var sidecar struct {
		Version  int `json:"version"`
		Parts    []struct {
			N              int        `json:"n"`
			PlaintextLen   int64      `json:"plaintext_len"`
			CiphertextLen  int64      `json:"ciphertext_len"`
			Blocks         [][]string `json:"blocks"`
		} `json:"parts"`
	}

	if err := json.NewDecoder(gz).Decode(&sidecar); err != nil {
		t.Fatalf("Failed to decode v3 sidecar: %v", err)
	}

	// Verify sidecar structure
	if sidecar.Version != 3 {
		t.Errorf("Expected sidecar version 3, got: %d", sidecar.Version)
	}

	if len(sidecar.Parts) != 1 {
		t.Errorf("Expected 1 part in sidecar, got: %d", len(sidecar.Parts))
	}

	part := sidecar.Parts[0]
	if part.N != 1 {
		t.Errorf("Expected part number 1, got: %d", part.N)
	}

	if part.PlaintextLen != int64(len(partData)) {
		t.Errorf("Expected plaintext_len %d, got: %d", len(partData), part.PlaintextLen)
	}

	if part.CiphertextLen != int64(len(partData)) {
		t.Errorf("Expected ciphertext_len %d, got: %d", len(partData), part.CiphertextLen)
	}

	if len(part.Blocks) == 0 {
		t.Error("Expected at least one block in part")
	}

	// Verify each block has [hmac_b64, clen] format
	for i, block := range part.Blocks {
		if len(block) != 2 {
			t.Errorf("Block %d: expected [hmac_b64, clen], got %d elements", i, len(block))
		}
		// Verify clen is a valid integer
		var clen int
		if _, err := fmt.Sscanf(block[1], "%d", &clen); err != nil {
			t.Errorf("Block %d: clen is not a valid integer: %s", i, block[1])
		}
	}

	t.Log("Successfully validated v3 sidecar structure")

	// Verify per-part objects were deleted after Complete
	multipartPrefix := fmt.Sprintf(".armor/multipart/%s/", *uploadID)
	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(multipartPrefix),
		MaxKeys: aws.Int32(100),
	})
	if err != nil {
		t.Fatalf("Failed to list per-part objects: %v", err)
	}

	if len(listResp.Contents) > 0 {
		t.Errorf("Expected per-part objects to be deleted after Complete, found %d objects", len(listResp.Contents))
		for _, obj := range listResp.Contents {
			t.Logf("Leaked object: %s", *obj.Key)
		}
	}

	// Cleanup
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Logf("Cleanup warning: failed to delete object: %v", err)
	}

	// Delete sidecar
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sidecarKey),
	})
	if err != nil {
		t.Logf("Cleanup warning: failed to delete sidecar: %v", err)
	}
}

// TestCompleteMultipartUpload_V3_TwoParts tests a 2-part upload with v3 format
func TestCompleteMultipartUpload_V3_TwoParts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create multipart upload with v3 format
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Metadata: map[string]string{
			"test-name":    "TestCompleteMultipartUpload_V3_TwoParts",
			"armor-format": "3",
		},
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId
	t.Logf("Created multipart upload: %s", *uploadID)

	// Upload two parts (each 10 MB to meet minimum part size)
	part1Data := generateTestData(10 * 1024 * 1024) // 10 MB
	uploadPartResp1, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(part1Data),
	})
	if err != nil {
		t.Fatalf("UploadPart 1 failed: %v", err)
	}
	part1ETag := uploadPartResp1.ETag
	t.Logf("Uploaded part 1, ETag: %s", *part1ETag)

	part2Data := generateTestData(10 * 1024 * 1024) // 10 MB
	uploadPartResp2, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(2),
		Body:       bytes.NewReader(part2Data),
	})
	if err != nil {
		t.Fatalf("UploadPart 2 failed: %v", err)
	}
	part2ETag := uploadPartResp2.ETag
	t.Logf("Uploaded part 2, ETag: %s", *part2ETag)

	// Complete the multipart upload
	completeResp, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{
					ETag:       part1ETag,
					PartNumber: aws.Int32(1),
				},
				{
					ETag:       part2ETag,
					PartNumber: aws.Int32(2),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("Completed multipart upload, Location: %s, ETag: %s", *completeResp.Location, *completeResp.ETag)

	// Verify the object exists and has v3 metadata
	headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}

	// Check metadata contains version 3
	version := headResp.Metadata["Armor-Version"]
	if version != "3" {
		t.Errorf("Expected ARMOR version 3, got: %s", version)
	}

	// Check multipart flag is set
	multipartFlag := headResp.Metadata["Armor-Multipart"]
	if multipartFlag != "true" {
		t.Errorf("Expected ARMOR multipart flag 'true', got: %s", multipartFlag)
	}

	// Verify the v3 sidecar exists and can be parsed
	sidecarKey := fmt.Sprintf(".armor/hmac/%x", sha256Sum([]byte(key)))
	sidecarResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sidecarKey),
	})
	if err != nil {
		t.Fatalf("Failed to get v3 sidecar: %v", err)
	}
	defer sidecarResp.Body.Close()

	// Decompress and parse the gzip-compressed JSON sidecar
	gz, err := gzip.NewReader(sidecarResp.Body)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	var sidecar struct {
		Version  int `json:"version"`
		Parts    []struct {
			N              int        `json:"n"`
			PlaintextLen   int64      `json:"plaintext_len"`
			CiphertextLen  int64      `json:"ciphertext_len"`
			Blocks         [][]string `json:"blocks"`
		} `json:"parts"`
	}

	if err := json.NewDecoder(gz).Decode(&sidecar); err != nil {
		t.Fatalf("Failed to decode v3 sidecar: %v", err)
	}

	// Verify sidecar structure
	if sidecar.Version != 3 {
		t.Errorf("Expected sidecar version 3, got: %d", sidecar.Version)
	}

	if len(sidecar.Parts) != 2 {
		t.Errorf("Expected 2 parts in sidecar, got: %d", len(sidecar.Parts))
	}

	// Verify part 1
	part1 := sidecar.Parts[0]
	if part1.N != 1 {
		t.Errorf("Expected part 1 number 1, got: %d", part1.N)
	}

	if part1.PlaintextLen != int64(len(part1Data)) {
		t.Errorf("Expected part 1 plaintext_len %d, got: %d", len(part1Data), part1.PlaintextLen)
	}

	if part1.CiphertextLen != int64(len(part1Data)) {
		t.Errorf("Expected part 1 ciphertext_len %d, got: %d", len(part1Data), part1.CiphertextLen)
	}

	if len(part1.Blocks) == 0 {
		t.Error("Expected at least one block in part 1")
	}

	// Verify part 2
	part2 := sidecar.Parts[1]
	if part2.N != 2 {
		t.Errorf("Expected part 2 number 2, got: %d", part2.N)
	}

	if part2.PlaintextLen != int64(len(part2Data)) {
		t.Errorf("Expected part 2 plaintext_len %d, got: %d", len(part2Data), part2.PlaintextLen)
	}

	if part2.CiphertextLen != int64(len(part2Data)) {
		t.Errorf("Expected part 2 ciphertext_len %d, got: %d", len(part2Data), part2.CiphertextLen)
	}

	if len(part2.Blocks) == 0 {
		t.Error("Expected at least one block in part 2")
	}

	t.Log("Successfully validated v3 sidecar structure for 2-part upload")

	// Verify per-part objects were deleted after Complete
	multipartPrefix := fmt.Sprintf(".armor/multipart/%s/", *uploadID)
	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(multipartPrefix),
		MaxKeys: aws.Int32(100),
	})
	if err != nil {
		t.Fatalf("Failed to list per-part objects: %v", err)
	}

	if len(listResp.Contents) > 0 {
		t.Errorf("Expected per-part objects to be deleted after Complete, found %d objects", len(listResp.Contents))
		for _, obj := range listResp.Contents {
			t.Logf("Leaked object: %s", *obj.Key)
		}
	}

	// Cleanup
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Logf("Cleanup warning: failed to delete object: %v", err)
	}

	// Delete sidecar
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sidecarKey),
	})
	if err != nil {
		t.Logf("Cleanup warning: failed to delete sidecar: %v", err)
	}
}

// TestAbortMultipartUpload_V3_Cleanup tests that Abort properly deletes per-part objects for v3
func TestAbortMultipartUpload_V3_Cleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create multipart upload with v3 format
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Metadata: map[string]string{
			"test-name":    "TestAbortMultipartUpload_V3_Cleanup",
			"armor-format": "3",
		},
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId
	t.Logf("Created multipart upload: %s", *uploadID)

	// Upload a part
	partData := generateTestData(10 * 1024 * 1024) // 10 MB
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

	// Verify per-part objects exist
	multipartPrefix := fmt.Sprintf(".armor/multipart/%s/", *uploadID)
	listResp1, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(multipartPrefix),
		MaxKeys: aws.Int32(100),
	})
	if err != nil {
		t.Fatalf("Failed to list per-part objects before abort: %v", err)
	}

	if len(listResp1.Contents) == 0 {
		t.Error("Expected per-part objects to exist before abort")
	} else {
		t.Logf("Found %d per-part objects before abort", len(listResp1.Contents))
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

	// Give it a moment for cleanup
	time.Sleep(1 * time.Second)

	// Verify per-part objects were deleted after Abort
	listResp2, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(multipartPrefix),
		MaxKeys: aws.Int32(100),
	})
	if err != nil {
		t.Fatalf("Failed to list per-part objects after abort: %v", err)
	}

	if len(listResp2.Contents) > 0 {
		t.Errorf("Expected per-part objects to be deleted after Abort, found %d objects", len(listResp2.Contents))
		for _, obj := range listResp2.Contents {
			t.Logf("Leaked object: %s", *obj.Key)
		}
	} else {
		t.Log("Successfully verified all per-part objects were deleted after Abort")
	}
}
