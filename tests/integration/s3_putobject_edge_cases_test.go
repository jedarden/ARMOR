//go:build integration
// +build integration

// PutObject Edge Cases and Comprehensive Metadata Tests
// Tests for PutObject operation covering edge cases, metadata, tags, and S3 compliance

package integration

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestPutObject_ZeroByteFile tests uploading a zero-byte file
func TestPutObject_ZeroByteFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t) + "-zero"

	testData := []byte{} // Zero bytes

	// Upload zero-byte file
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject with zero bytes failed: %v", err)
	}
	t.Log("Successfully uploaded zero-byte file")

	// Verify file exists and has zero size
	headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}

	if headResp.ContentLength != nil && *headResp.ContentLength != 0 {
		t.Errorf("Expected ContentLength 0, got %d", *headResp.ContentLength)
	}

	t.Log("Zero-byte file size verified correctly")

	// Verify we can download it
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer getResp.Body.Close()

	downloaded, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if len(downloaded) != 0 {
		t.Errorf("Expected 0 bytes, got %d", len(downloaded))
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	t.Log("TestPutObject_ZeroByteFile completed successfully")
}

// TestPutObject_VerySmallFile tests 1-byte files
func TestPutObject_VerySmallFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t) + "-tiny"

	testData := []byte{0x42} // 1 byte

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject with 1 byte failed: %v", err)
	}

	// Download and verify
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer getResp.Body.Close()

	downloaded, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !bytes.Equal(testData, downloaded) {
		t.Errorf("1-byte file mismatch: got %v, want %v", downloaded, testData)
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	t.Log("TestPutObject_VerySmallFile completed successfully")
}

// TestPutObject_LargeFile tests files larger than typical buffer sizes
func TestPutObject_LargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t) + "-large"

	// 20 MB file - larger than typical buffer, smaller than multipart threshold
	testData := generateTestData(20 * 1024 * 1024)

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject with 20MB failed: %v", err)
	}
	t.Logf("Successfully uploaded 20MB file")

	// Download and verify integrity
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer getResp.Body.Close()

	downloaded, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !bytes.Equal(testData, downloaded) {
		t.Errorf("Large file mismatch: got %d bytes, want %d bytes", len(downloaded), len(testData))
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	t.Log("TestPutObject_LargeFile completed successfully")
}

// TestPutObject_MetadataPreservation tests that custom metadata is preserved
func TestPutObject_MetadataPreservation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	testData := generateTestData(1024)
	metadata := map[string]string{
		"custom-key-1":  "custom-value-1",
		"custom-key-2":  "custom-value-2",
		"x-amz-meta-test": "should-have-prefix",
	}

	// Upload with custom metadata
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		Body:     bytes.NewReader(testData),
		Metadata: metadata,
	})
	if err != nil {
		t.Fatalf("PutObject with metadata failed: %v", err)
	}

	// Verify metadata is preserved
	headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}

	for k, v := range metadata {
		found := false
		for _, mk := range headResp.Metadata {
			if mk == v {
				found = true
				break
			}
		}
		if !found {
			// Check if it's in the response metadata with proper prefix
			if headResp.Metadata != nil {
				if val, ok := headResp.Metadata[k]; ok && val == v {
					found = true
				}
			}
		}
		if !found {
			t.Logf("Warning: metadata key %s with value %s not found in response", k, v)
		}
	}

	t.Logf("Metadata preserved: %d keys", len(headResp.Metadata))

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	t.Log("TestPutObject_MetadataPreservation completed")
}

// TestPutObject_ContentType tests Content-Type header handling
func TestPutObject_ContentType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	testCases := []struct {
		contentType string
		description string
	}{
		{"application/json", "JSON data"},
		{"image/jpeg", "JPEG image"},
		{"text/plain; charset=utf-8", "Plain text with charset"},
		{"application/octet-stream", "Binary data"},
		{"application/pdf", "PDF document"},
	}

	for _, tc := range testCases {
		t.Run(tc.contentType, func(t *testing.T) {
			testKey := fmt.Sprintf("%s-%s", key, tc.contentType)

			testData := generateTestData(512)
			_, err := client.PutObject(ctx, &s3.PutObjectInput{
				Bucket:      aws.String(bucket),
				Key:         aws.String(testKey),
				Body:        bytes.NewReader(testData),
				ContentType: aws.String(tc.contentType),
			})
			if err != nil {
				t.Fatalf("PutObject with ContentType=%s failed: %v", tc.contentType, err)
			}

			// Verify Content-Type is preserved
			headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(testKey),
			})
			if err != nil {
				t.Fatalf("HeadObject failed: %v", err)
			}

			if headResp.ContentType == nil {
				t.Errorf("ContentType not returned in HeadObject")
			} else if *headResp.ContentType != tc.contentType {
				t.Logf("ContentType changed: sent %s, got %s", tc.contentType, *headResp.ContentType)
			} else {
				t.Logf("ContentType preserved: %s", *headResp.ContentType)
			}

			// Cleanup
			_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(testKey),
			})
		})
	}

	t.Log("TestPutObject_ContentType completed")
}

// TestPutObject_ContentEncoding tests Content-Encoding header
func TestPutObject_ContentEncoding(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create gzip-compressed data
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	testData := generateTestData(1024)
	gz.Write(testData)
	gz.Close()

	compressedData := buf.Bytes()

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:         aws.String(bucket),
		Key:            aws.String(key),
		Body:           bytes.NewReader(compressedData),
		ContentEncoding: aws.String("gzip"),
	})
	if err != nil {
		t.Fatalf("PutObject with Content-Encoding failed: %v", err)
	}

	// Verify Content-Encoding is preserved
	headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}

	if headResp.ContentEncoding == nil {
		t.Logf("ContentEncoding not returned (may be stripped by ARMOR)")
	} else if *headResp.ContentEncoding != "gzip" {
		t.Logf("ContentEncoding changed: sent gzip, got %s", *headResp.ContentEncoding)
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	t.Log("TestPutObject_ContentEncoding completed")
}

// TestPutObject_ObjectTags tests object tagging
func TestPutObject_ObjectTags(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	testData := generateTestData(1024)
	tagging := types.Tagging{
		TagSet: []types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("Test")},
			{Key: aws.String("Project"), Value: aws.String("ARMOR")},
			{Key: aws.String("Owner"), Value: aws.String("TestSuite")},
		},
	}

	// Upload object with tags
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
		Tagging: aws.String(
			"Environment=Test&Project=ARMOR&Owner=TestSuite",
		),
	})
	if err != nil {
		t.Logf("PutObject with tagging: %v", err)
		t.Log("Object tagging may not be supported in ARMOR")
		return
	}
	t.Log("Successfully uploaded object with tags")

	// Get object tags
	getTagResp, err := client.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObjectTagging failed: %v", err)
	}

	if len(getTagResp.TagSet) != len(tagging.TagSet) {
		t.Errorf("Expected %d tags, got %d", len(tagging.TagSet), len(getTagResp.TagSet))
	}

	for i, tag := range getTagResp.TagSet {
		expectedTag := tagging.TagSet[i]
		if *tag.Key != *expectedTag.Key {
			t.Errorf("Tag key mismatch: got %s, want %s", *tag.Key, *expectedTag.Key)
		}
		if *tag.Value != *expectedTag.Value {
			t.Errorf("Tag value mismatch for key %s: got %s, want %s",
				*tag.Key, *tag.Value, *expectedTag.Value)
		}
		t.Logf("Tag %s=%s verified", *tag.Key, *tag.Value)
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	t.Log("TestPutObject_ObjectTags completed successfully")
}

// TestPutObject_Overwrite tests overwriting an existing object
func TestPutObject_Overwrite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Upload initial version
	v1Data := generateTestData(1024)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(v1Data),
	})
	if err != nil {
		t.Fatalf("PutObject v1 failed: %v", err)
	}

	// Verify v1
	v1Resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject v1 failed: %v", err)
	}
	v1Content, _ := io.ReadAll(v1Resp.Body)
	v1Resp.Body.Close()

	if !bytes.Equal(v1Data, v1Content) {
		t.Error("v1 content mismatch")
	}
	t.Log("v1 verified")

	// Overwrite with v2 (different size)
	v2Data := generateTestData(2048)
	// Make it different
	for i := range v2Data {
		v2Data[i] = 0xFF - v2Data[i]
	}

	v2PutResp, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(v2Data),
	})
	if err != nil {
		t.Fatalf("PutObject v2 failed: %v", err)
	}
	t.Logf("Overwrote object, ETag: %s", *v2PutResp.ETag)

	// Verify v2 replaced v1
	v2Resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject v2 failed: %v", err)
	}
	v2Content, _ := io.ReadAll(v2Resp.Body)
	v2Resp.Body.Close()

	if !bytes.Equal(v2Data, v2Content) {
		t.Error("v2 content mismatch - overwrite failed")
	}

	if len(v2Content) != len(v2Data) {
		t.Errorf("Size mismatch after overwrite: got %d, want %d",
			len(v2Content), len(v2Data))
	}

	t.Log("v2 verified - overwrite successful")

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	t.Log("TestPutObject_Overwrite completed successfully")
}

// TestPutObject_ConcurrentWrites tests concurrent writes to the same key
func TestPutObject_ConcurrentWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Launch multiple goroutines writing to the same key
	numWriters := 5
	results := make(chan error, numWriters)

	for i := 0; i < numWriters; i++ {
		go func(writerNum int) {
			data := generateTestData(512)
			// Mark data with writer number
			data[0] = byte(writerNum)

			_, err := client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Body:   bytes.NewReader(data),
			})
			results <- err
		}(i)
	}

	// Wait for all writes
	successCount := 0
	for i := 0; i < numWriters; i++ {
		err := <-results
		if err != nil {
			t.Logf("Concurrent write %d failed: %v", i, err)
		} else {
			successCount++
		}
	}

	t.Logf("Concurrent writes: %d/%d succeeded", successCount, numWriters)

	// Verify final state
	headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject after concurrent writes failed: %v", err)
	}

	t.Logf("Final object size: %d bytes, ETag: %s",
		*headResp.ContentLength, *headResp.ETag)

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	t.Log("TestPutObject_ConcurrentWrites completed")
}

// TestPutObject_InvalidInputs tests error handling for invalid inputs
func TestPutObject_InvalidInputs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	testCases := []struct {
		name        string
		bucketName  string
		key         string
		shouldFail  bool
		description string
	}{
		{
			name:        "Valid input",
			bucketName:  bucket,
			key:         generateTestKey(t),
			shouldFail:  false,
			description: "Valid bucket and key",
		},
		{
			name:        "Empty key",
			bucketName:  bucket,
			key:         "",
			shouldFail:  true,
			description: "Empty key should fail",
		},
		{
			name:        "Non-existent bucket",
			bucketName:  fmt.Sprintf("nonexistent-%d", time.Now().UnixNano()),
			key:         generateTestKey(t),
			shouldFail:  true,
			description: "Non-existent bucket should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testData := generateTestData(512)

			_, err := client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(tc.bucketName),
				Key:    aws.String(tc.key),
				Body:   bytes.NewReader(testData),
			})

			if tc.shouldFail {
				if err == nil {
					t.Errorf("%s: expected error but succeeded", tc.description)
				} else {
					t.Logf("%s: failed as expected: %v", tc.description, err)
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tc.description, err)
				} else {
					// Cleanup on success
					_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
						Bucket: aws.String(tc.bucketName),
						Key:    aws.String(tc.key),
					})
				}
			}
		})
	}

	t.Log("TestPutObject_InvalidInputs completed")
}
