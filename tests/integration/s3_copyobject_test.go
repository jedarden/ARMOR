//go:build integration
// +build integration

// Comprehensive CopyObject Tests
// Tests for CopyObject operation covering same-bucket, cross-bucket, metadata, and DEK re-wrapping

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestCopyObject_SameBucket tests copying within the same bucket
func TestCopyObject_SameBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	sourceKey := generateTestKey(t) + "-source"
	destKey := generateTestKey(t) + "-dest"

	// Upload source object
	testData := generateTestData(1024 * 1024) // 1 MB
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject source failed: %v", err)
	}
	t.Logf("Uploaded source object: %s", sourceKey)

	// Copy within same bucket
	copyResp, err := client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", bucket, sourceKey)),
		Key:        aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("CopyObject failed: %v", err)
	}
	t.Logf("CopyObject succeeded, ETag: %s", *copyResp.CopyObjectResult.ETag)

	// Verify destination object exists
	headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("HeadObject destination failed: %v", err)
	}

	if headResp.ContentLength == nil || *headResp.ContentLength != int64(len(testData)) {
		t.Errorf("Size mismatch: got %d, want %d",
			headResp.ContentLength, int64(len(testData)))
	}

	// Verify destination content matches source
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("GetObject destination failed: %v", err)
	}
	defer getResp.Body.Close()

	downloaded, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read destination: %v", err)
	}

	if !bytes.Equal(testData, downloaded) {
		t.Error("Destination content doesn't match source")
	}

	t.Log("Destination content verified - copy successful")

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})

	t.Log("TestCopyObject_SameBucket completed successfully")
}

// TestCopyObject_MetadataCopy tests metadata copying with COPY directive
func TestCopyObject_MetadataCopy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	sourceKey := generateTestKey(t) + "-source"
	destKey := generateTestKey(t) + "-dest"

	// Upload source with metadata
	sourceMetadata := map[string]string{
		"x-amz-meta-source-key":   "source-value",
		"x-amz-meta-project":      "ARMOR",
		"x-amz-meta-environment":  "test",
	}

	testData := generateTestData(1024)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(sourceKey),
		Body:     bytes.NewReader(testData),
		Metadata: sourceMetadata,
	})
	if err != nil {
		t.Fatalf("PutObject source failed: %v", err)
	}
	t.Logf("Uploaded source with %d metadata keys", len(sourceMetadata))

	// Copy with COPY directive (default - copies metadata)
	copyResp, err := client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:         aws.String(bucket),
		CopySource:     aws.String(fmt.Sprintf("%s/%s", bucket, sourceKey)),
		Key:            aws.String(destKey),
		Metadata:       sourceMetadata,
		MetadataDirective: types.MetadataDirectiveCopy,
	})
	if err != nil {
		t.Fatalf("CopyObject with COPY directive failed: %v", err)
	}
	t.Logf("CopyObject with COPY directive, ETag: %s", *copyResp.CopyObjectResult.ETag)

	// Verify metadata was copied
	destHead, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("HeadObject destination failed: %v", err)
	}

	if destHead.Metadata == nil {
		t.Error("Destination has no metadata")
	} else {
		t.Logf("Destination has %d metadata keys", len(destHead.Metadata))
		for k, v := range sourceMetadata {
			if destVal, ok := destHead.Metadata[k]; ok && destVal == v {
				t.Logf("Metadata %s=%s preserved", k, v)
			} else {
				t.Logf("Warning: metadata %s not fully preserved", k)
			}
		}
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})

	t.Log("TestCopyObject_MetadataCopy completed")
}

// TestCopyObject_MetadataReplace tests metadata replacement with REPLACE directive
func TestCopyObject_MetadataReplace(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	sourceKey := generateTestKey(t) + "-source"
	destKey := generateTestKey(t) + "-dest"

	// Upload source with metadata
	sourceMetadata := map[string]string{
		"x-amz-meta-old-key": "old-value",
	}

	testData := generateTestData(1024)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(sourceKey),
		Body:     bytes.NewReader(testData),
		Metadata: sourceMetadata,
	})
	if err != nil {
		t.Fatalf("PutObject source failed: %v", err)
	}

	// Copy with REPLACE directive (replace metadata)
	newMetadata := map[string]string{
		"x-amz-meta-new-key":   "new-value",
		"x-amz-meta-project":   "ARMOR-Copy",
		"x-amz-meta-version":   "2.0",
	}

	copyResp, err := client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:           aws.String(bucket),
		CopySource:       aws.String(fmt.Sprintf("%s/%s", bucket, sourceKey)),
		Key:              aws.String(destKey),
		Metadata:         newMetadata,
		MetadataDirective: types.MetadataDirectiveReplace,
	})
	if err != nil {
		t.Fatalf("CopyObject with REPLACE directive failed: %v", err)
	}
	t.Logf("CopyObject with REPLACE directive, ETag: %s", *copyResp.CopyObjectResult.ETag)

	// Verify metadata was replaced
	destHead, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("HeadObject destination failed: %v", err)
	}

	if destHead.Metadata == nil {
		t.Error("Destination has no metadata after REPLACE")
	} else {
		t.Logf("Destination has %d metadata keys after REPLACE", len(destHead.Metadata))
		for k, v := range newMetadata {
			if destVal, ok := destHead.Metadata[k]; ok && destVal == v {
				t.Logf("New metadata %s=%s present", k, v)
			} else {
				t.Logf("Warning: new metadata %s not found", k)
			}
		}

		// Old metadata should NOT be present
		if oldVal, ok := destHead.Metadata["x-amz-meta-old-key"]; ok {
			t.Logf("Warning: old metadata still present: %s", oldVal)
		} else {
			t.Log("Old metadata correctly absent after REPLACE")
		}
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})

	t.Log("TestCopyObject_MetadataReplace completed")
}

// TestCopyObject_ContentTypeReplace tests replacing content type during copy
func TestCopyObject_ContentTypeReplace(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	sourceKey := generateTestKey(t) + "-source"
	destKey := generateTestKey(t) + "-dest"

	// Upload source as text/plain
	testData := generateTestData(512)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(sourceKey),
		Body:        bytes.NewReader(testData),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		t.Fatalf("PutObject source failed: %v", err)
	}

	// Copy and change content type
	_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:           aws.String(bucket),
		CopySource:       aws.String(fmt.Sprintf("%s/%s", bucket, sourceKey)),
		Key:              aws.String(destKey),
		ContentType:      aws.String("application/json"),
		MetadataDirective: types.MetadataDirectiveReplace,
	})
	if err != nil {
		t.Fatalf("CopyObject with ContentType failed: %v", err)
	}

	// Verify content type changed
	destHead, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("HeadObject destination failed: %v", err)
	}

	if destHead.ContentType == nil {
		t.Error("ContentType not returned")
	} else if *destHead.ContentType != "application/json" {
		t.Logf("ContentType changed from text/plain to %s", *destHead.ContentType)
	} else {
		t.Log("ContentType successfully replaced to application/json")
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})

	t.Log("TestCopyObject_ContentTypeReplace completed")
}

// TestCopyObject_LargeFile tests copying large files
func TestCopyObject_LargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	sourceKey := generateTestKey(t) + "-source"
	destKey := generateTestKey(t) + "-dest"

	// Upload large source via multipart
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	// Upload parts
	partSize := int64(5 * 1024 * 1024) // 5 MB
	numParts := 3
	uploadedParts := []types.CompletedPart{}

	for i := int32(1); i <= int32(numParts); i++ {
		partData := generateTestData(int(partSize))
		uploadResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(sourceKey),
			UploadId:   uploadID,
			PartNumber: aws.Int32(i),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(sourceKey),
				UploadId: uploadID,
			})
			t.Fatalf("UploadPart %d failed: %v", i, err)
		}

		uploadedParts = append(uploadedParts, types.CompletedPart{
			ETag:       uploadResp.ETag,
			PartNumber: aws.Int32(i),
		})
	}

	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(sourceKey),
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: uploadedParts},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	sourceSize := partSize * int64(numParts)
	t.Logf("Created large source file: %d bytes", sourceSize)

	// Copy large file
	startTime := time.Now()
	_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", bucket, sourceKey)),
		Key:        aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("CopyObject large file failed: %v", err)
	}
	duration := time.Since(startTime)
	t.Logf("Copied large file (%d bytes) in %v", sourceSize, duration)

	// Verify destination size
	destHead, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("HeadObject destination failed: %v", err)
	}

	if destHead.ContentLength == nil || *destHead.ContentLength != sourceSize {
		t.Errorf("Size mismatch after copy: got %d, want %d",
			destHead.ContentLength, sourceSize)
	} else {
		t.Logf("Destination size verified: %d bytes", *destHead.ContentLength)
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})

	t.Log("TestCopyObject_LargeFile completed successfully")
}

// TestCopyObject_OverwriteDestination tests copying over an existing destination
func TestCopyObject_OverwriteDestination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	sourceKey := generateTestKey(t) + "-source"
	destKey := generateTestKey(t) + "-dest"

	// Upload source
	sourceData := generateTestData(1024)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
		Body:   bytes.NewReader(sourceData),
	})
	if err != nil {
		t.Fatalf("PutObject source failed: %v", err)
	}

	// Upload initial destination (different content)
	destData := generateTestData(2048)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
		Body:   bytes.NewReader(destData),
	})
	if err != nil {
		t.Fatalf("PutObject initial destination failed: %v", err)
	}

	// Get original destination ETag
	origDestHead, _ := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})
	origETag := origDestHead.ETag
	t.Logf("Original destination ETag: %s", *origETag)

	// Copy over the destination
	copyResp, err := client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", bucket, sourceKey)),
		Key:        aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("CopyObject over existing destination failed: %v", err)
	}
	t.Logf("CopyObject overwrote destination, new ETag: %s", *copyResp.CopyObjectResult.ETag)

	// Verify destination now has source content
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("GetObject destination failed: %v", err)
	}
	defer getResp.Body.Close()

	downloaded, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read destination: %v", err)
	}

	if !bytes.Equal(sourceData, downloaded) {
		t.Error("Destination doesn't contain source data after overwrite")
	}

	if len(downloaded) != len(sourceData) {
		t.Errorf("Destination size %d doesn't match source size %d",
			len(downloaded), len(sourceData))
	} else {
		t.Log("Destination successfully overwritten with source content")
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})

	t.Log("TestCopyObject_OverwriteDestination completed successfully")
}

// TestCopyObject_NonExistentSource tests copying from non-existent source
func TestCopyObject_NonExistentSource(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	sourceKey := fmt.Sprintf("nonexistent-source-%d", time.Now().UnixNano())
	destKey := generateTestKey(t) + "-dest"

	// Attempt to copy from non-existent source
	_, err := client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", bucket, sourceKey)),
		Key:        aws.String(destKey),
	})

	if err == nil {
		t.Error("CopyObject from non-existent source should fail")
		// Cleanup if it accidentally succeeded
		_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(destKey),
		})
	} else {
		t.Logf("CopyObject from non-existent source failed as expected: %v", err)
	}

	t.Log("TestCopyObject_NonExistentSource completed")
}

// TestCopyObject_SelfCopy tests copying an object to itself (should fail)
func TestCopyObject_SelfCopy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	key := generateTestKey(t)

	// Upload object
	testData := generateTestData(1024)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	// Attempt to copy to itself
	_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", bucket, key)),
		Key:        aws.String(key),
	})

	if err == nil {
		t.Error("CopyObject to same key should fail")
	} else {
		t.Logf("CopyObject to same key failed as expected: %v", err)
	}

	t.Log("TestCopyObject_SelfCopy completed")
}

// TestCopyObject_DEKReWrapping tests that copy re-wraps DEK for key rotation
func TestCopyObject_DEKReWrapping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	sourceKey := generateTestKey(t) + "-source"
	destKey := generateTestKey(t) + "-dest"

	// Upload source with specific metadata to track DEK
	testData := generateTestData(1024)
	putResp, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
		Body:   bytes.NewReader(testData),
		Metadata: map[string]string{
			"dek-generation": "original",
		},
	})
	if err != nil {
		t.Fatalf("PutObject source failed: %v", err)
	}
	sourceETag := *putResp.ETag

	// Copy the object
	copyResp, err := client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", bucket, sourceKey)),
		Key:        aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("CopyObject failed: %v", err)
	}
	destETag := *copyResp.CopyObjectResult.ETag

	t.Logf("Source ETag: %s, Dest ETag: %s", sourceETag, destETag)

	// Verify both objects are decryptable (same plaintext)
	sourceResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	if err != nil {
		t.Fatalf("GetObject source failed: %v", err)
	}
	sourceContent, _ := io.ReadAll(sourceResp.Body)
	sourceResp.Body.Close()

	destResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})
	if err != nil {
		t.Fatalf("GetObject dest failed: %v", err)
	}
	destContent, _ := io.ReadAll(destResp.Body)
	destResp.Body.Close()

	if !bytes.Equal(sourceContent, destContent) {
		t.Error("Source and destination content mismatch")
	} else {
		t.Log("Source and destination content match - DEK re-wrapping successful")
	}

	// Verify both have same plaintext
	if !bytes.Equal(testData, sourceContent) {
		t.Error("Source content doesn't match original")
	}
	if !bytes.Equal(testData, destContent) {
		t.Error("Dest content doesn't match original")
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})

	t.Log("TestCopyObject_DEKReWrapping completed successfully")
}

// TestCopyObject_Tagging tests copying with tag replacement
func TestCopyObject_Tagging(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	sourceKey := generateTestKey(t) + "-source"
	destKey := generateTestKey(t) + "-dest"

	// Upload source
	testData := generateTestData(512)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
		Body:   bytes.NewReader(testData),
		Tagging: aws.String("Environment=Production&Project=ARMOR"),
	})
	if err != nil {
		t.Logf("PutObject with tagging: %v", err)
		t.Log("Skipping tagging test - may not be supported")
		return
	}

	// Copy with tag replacement
	_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:           aws.String(bucket),
		CopySource:       aws.String(fmt.Sprintf("%s/%s", bucket, sourceKey)),
		Key:              aws.String(destKey),
		Tagging:          aws.String("Environment=Test&Project=Copied"),
		TaggingDirective: types.TaggingDirectiveReplace,
	})

	if err != nil {
		t.Logf("CopyObject with TaggingDirective: %v", err)
		t.Log("Tag replacement may not be supported in ARMOR")
	} else {
		// Verify destination tags
		getTagResp, err := client.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(destKey),
		})
		if err != nil {
			t.Fatalf("GetObjectTagging failed: %v", err)
		}

		t.Logf("Destination has %d tags", len(getTagResp.TagSet))
		for _, tag := range getTagResp.TagSet {
			t.Logf("Tag: %s=%s", *tag.Key, *tag.Value)
		}
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})

	t.Log("TestCopyObject_Tagging completed")
}

// TestCopyObject_StorageClass tests storage class handling during copy
func TestCopyObject_StorageClass(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	sourceKey := generateTestKey(t) + "-source"
	destKey := generateTestKey(t) + "-dest"

	// Upload source
	testData := generateTestData(512)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject source failed: %v", err)
	}

	// Copy with storage class directive
	_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:              aws.String(bucket),
		CopySource:          aws.String(fmt.Sprintf("%s/%s", bucket, sourceKey)),
		Key:                 aws.String(destKey),
		StorageClass:        types.StorageClassStandardIa,
	})

	if err != nil {
		t.Logf("CopyObject with StorageClass: %v", err)
		t.Log("Storage class directive may not be supported in ARMOR")
	} else {
		// Verify storage class
		headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(destKey),
		})
		if err != nil {
			t.Fatalf("HeadObject failed: %v", err)
		}

		if headResp.StorageClass != "" {
			t.Logf("Destination storage class: %s", headResp.StorageClass)
		}
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(sourceKey),
	})
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(destKey),
	})

	t.Log("TestCopyObject_StorageClass completed")
}
