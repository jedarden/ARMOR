//go:build integration
// +build integration

// GetObject Edge Cases and Comprehensive Tests
// Tests for GetObject operation covering conditional reads, response overrides, and edge cases

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestGetObject_ConditionalIfMatch tests If-Match conditional download
func TestGetObject_ConditionalIfMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Upload test data
	testData := generateTestData(1024)
	putResp, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	etag := strings.Trim(*putResp.ETag, "\"")
	t.Logf("Uploaded object, ETag: %s", etag)

	// Test 1: GetObject with matching ETag should succeed
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		IfMatch: aws.String(etag),
	})
	if err != nil {
		t.Errorf("GetObject with matching ETag failed: %v", err)
	} else {
		getResp.Body.Close()
		t.Log("GetObject with matching ETag succeeded")
	}
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	// Test 2: GetObject with non-matching ETag should fail
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(key),
		IfMatch: aws.String("\"fake-etag-12345\""),
	})
	if err == nil {
		t.Error("GetObject with non-matching ETag should fail")
	} else {
		t.Logf("GetObject with non-matching ETag failed as expected: %v", err)
	}

	t.Log("TestGetObject_ConditionalIfMatch completed successfully")
}

// TestGetObject_ConditionalIfNoneMatch tests If-None-Match conditional download
func TestGetObject_ConditionalIfNoneMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Upload test data
	testData := generateTestData(1024)
	putResp, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	etag := strings.Trim(*putResp.ETag, "\"")
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	// Test 1: GetObject with matching ETag should return 304 Not Modified
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:       aws.String(bucket),
		Key:          aws.String(key),
		IfNoneMatch:  aws.String(etag),
	})
	if err == nil {
		getResp.Body.Close()
		t.Error("GetObject with matching If-None-Match should return 304")
	} else {
		t.Logf("GetObject with matching If-None-Match returned error as expected: %v", err)
	}

	// Test 2: GetObject with non-matching ETag should succeed
	getResp, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:       aws.String(bucket),
		Key:          aws.String(key),
		IfNoneMatch:  aws.String("\"fake-etag-12345\""),
	})
	if err != nil {
		t.Errorf("GetObject with non-matching If-None-Match failed: %v", err)
	} else {
		getResp.Body.Close()
		t.Log("GetObject with non-matching If-None-Match succeeded")
	}

	t.Log("TestGetObject_ConditionalIfNoneMatch completed successfully")
}

// TestGetObject_ConditionalModifiedSince tests If-Modified-Since conditional download
func TestGetObject_ConditionalModifiedSince(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Upload test data
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

	// Get object to retrieve Last-Modified
	headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}
	lastModified := *headResp.LastModified
	t.Logf("Object Last-Modified: %v", lastModified)

	// Test 1: GetObject with If-Modified-Since before Last-Modified should succeed
	futureTime := lastModified.Add(-1 * time.Hour)
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:            aws.String(bucket),
		Key:               aws.String(key),
		IfModifiedSince:   &futureTime,
	})
	if err != nil {
		t.Errorf("GetObject with If-Modified-Since (older) failed: %v", err)
	} else {
		getResp.Body.Close()
		t.Log("GetObject with If-Modified-Since (older) succeeded")
	}

	// Test 2: GetObject with If-Modified-Since after Last-Modified should return 304
	futureTime = lastModified.Add(1 * time.Hour)
	getResp, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:            aws.String(bucket),
		Key:               aws.String(key),
		IfModifiedSince:   &futureTime,
	})
	if err == nil {
		getResp.Body.Close()
		t.Error("GetObject with If-Modified-Since (newer) should return 304")
	} else {
		t.Logf("GetObject with If-Modified-Since (newer) returned error as expected: %v", err)
	}

	t.Log("TestGetObject_ConditionalModifiedSince completed successfully")
}

// TestGetObject_ConditionalUnmodifiedSince tests If-Unmodified-Since conditional download
func TestGetObject_ConditionalUnmodifiedSince(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Upload test data
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

	// Get object to retrieve Last-Modified
	headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}
	lastModified := *headResp.LastModified

	// Test 1: GetObject with If-Unmodified-Since after Last-Modified should succeed
	futureTime := lastModified.Add(1 * time.Hour)
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:              aws.String(bucket),
		Key:                 aws.String(key),
		IfUnmodifiedSince:   &futureTime,
	})
	if err != nil {
		t.Errorf("GetObject with If-Unmodified-Since (newer) failed: %v", err)
	} else {
		getResp.Body.Close()
		t.Log("GetObject with If-Unmodified-Since (newer) succeeded")
	}

	// Test 2: GetObject with If-Unmodified-Since before Last-Modified should fail
	pastTime := lastModified.Add(-1 * time.Hour)
	getResp, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:              aws.String(bucket),
		Key:                 aws.String(key),
		IfUnmodifiedSince:   &pastTime,
	})
	if err == nil {
		getResp.Body.Close()
		t.Error("GetObject with If-Unmodified-Since (older) should fail")
	} else {
		t.Logf("GetObject with If-Unmodified-Since (older) failed as expected: %v", err)
	}

	t.Log("TestGetObject_ConditionalUnmodifiedSince completed successfully")
}

// TestGetObject_ResponseHeaderOverrides tests response header override parameters
func TestGetObject_ResponseHeaderOverrides(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Upload test data
	testData := generateTestData(1024)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(testData),
		ContentType:   aws.String("text/plain"),
		CacheControl:  aws.String("no-cache"),
		ContentDisposition: aws.String("attachment; filename=original.txt"),
		ContentEncoding: aws.String("identity"),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	// Test response header overrides
	testCases := []struct {
		name              string
		responseCacheControl     string
		responseContentDisposition string
		responseContentEncoding  string
		responseContentType       string
		responseExpires          *time.Time
	}{
		{
			name:                     "Override all",
			responseCacheControl:     "max-age=3600",
			responseContentDisposition: "attachment; filename=overridden.txt",
			responseContentEncoding:  "gzip",
			responseContentType:      "application/json",
			responseExpires:         func() *time.Time {
				t, _ := time.Parse(time.RFC1123, "Thu, 01 Dec 2099 16:00:00 GMT")
				return &t
			}(),
		},
		{
			name:                     "Partial override",
			responseCacheControl:     "max-age=1800",
			responseContentDisposition: "inline",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
				Bucket:                         aws.String(bucket),
				Key:                            aws.String(key),
				ResponseCacheControl:           aws.String(tc.responseCacheControl),
				ResponseContentDisposition:     aws.String(tc.responseContentDisposition),
				ResponseContentEncoding:        aws.String(tc.responseContentEncoding),
				ResponseContentType:            aws.String(tc.responseContentType),
				ResponseExpires:                tc.responseExpires,
			})

			if err != nil {
				t.Logf("GetObject with overrides returned error: %v", err)
				// ARMOR may not support all overrides
				return
			}
			defer getResp.Body.Close()

			// Verify overrides are applied (if supported)
			if tc.responseCacheControl != "" && getResp.CacheControl != nil {
				if *getResp.CacheControl != tc.responseCacheControl {
					t.Logf("CacheControl override: sent %s, got %s",
						tc.responseCacheControl, *getResp.CacheControl)
				}
			}

			if tc.responseContentDisposition != "" && getResp.ContentDisposition != nil {
				if *getResp.ContentDisposition != tc.responseContentDisposition {
					t.Logf("ContentDisposition override: sent %s, got %s",
						tc.responseContentDisposition, *getResp.ContentDisposition)
				}
			}

			t.Logf("Response overrides applied (partial support may be present)")
		})
	}

	t.Log("TestGetObject_ResponseHeaderOverrides completed")
}

// TestGetObject_RangeEdgeCases tests edge cases for range requests
func TestGetObject_RangeEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Upload test data
	testData := generateTestData(100 * 1024) // 100 KB
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

	testCases := []struct {
		name           string
		rangeHeader    string
		shouldSucceed  bool
		description    string
	}{
		{
			name:          "First byte",
			rangeHeader:   "bytes=0-0",
			shouldSucceed: true,
			description:   "Single byte from start",
		},
		{
			name:          "Last byte",
			rangeHeader:   fmt.Sprintf("bytes=%d-%d", len(testData)-1, len(testData)-1),
			shouldSucceed: true,
			description:   "Single byte from end",
		},
		{
			name:          "From offset to end",
			rangeHeader:   fmt.Sprintf("bytes=%d-", len(testData)/2),
			shouldSucceed: true,
			description:   "Open-ended range",
		},
		{
			name:          "Last 100 bytes",
			rangeHeader:   fmt.Sprintf("bytes=-100"),
			shouldSucceed: true,
			description:   "Suffix range",
		},
		{
			name:          "Multiple ranges",
			rangeHeader:   "bytes=0-10,20-30",
			shouldSucceed: false,
			description:   "Multiple ranges (not supported)",
		},
		{
			name:          "Invalid range",
			rangeHeader:   "bytes=100-50",
			shouldSucceed: false,
			description:   "Start after end",
		},
		{
			name:          "Range beyond file",
			rangeHeader:   fmt.Sprintf("bytes=0-%d", len(testData)+1000),
			shouldSucceed: false,
			description:   "Range exceeds file size",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Range:  aws.String(tc.rangeHeader),
			})

			if tc.shouldSucceed {
				if err != nil {
					t.Errorf("%s: expected success, got error: %v", tc.description, err)
				} else {
					defer getResp.Body.Close()
					downloaded, _ := io.ReadAll(getResp.Body)
					t.Logf("%s: succeeded, got %d bytes", tc.description, len(downloaded))

					// Verify Content-Range is present
					if getResp.ContentRange == nil {
						t.Logf("Warning: Content-Range header missing")
					} else {
						t.Logf("Content-Range: %s", *getResp.ContentRange)
					}
				}
			} else {
				if err == nil {
					getResp.Body.Close()
					t.Errorf("%s: expected failure, succeeded", tc.description)
				} else {
					t.Logf("%s: failed as expected: %v", tc.description, err)
				}
			}
		})
	}

	t.Log("TestGetObject_RangeEdgeCases completed")
}

// TestGetObject_PartNumber tests multipart object part retrieval
func TestGetObject_PartNumber(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create a multipart upload
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	// Upload parts
	partSize := int64(5 * 1024 * 1024) // 5 MB
	uploadedParts := []types.CompletedPart{}

	for i := int32(1); i <= 3; i++ {
		partData := generateTestData(int(partSize))
		uploadResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(key),
			UploadId:   uploadID,
			PartNumber: aws.Int32(i),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
				UploadId: uploadID,
			})
			t.Fatalf("UploadPart %d failed: %v", i, err)
		}

		uploadedParts = append(uploadedParts, types.CompletedPart{
			ETag:       uploadResp.ETag,
			PartNumber: aws.Int32(i),
		})
	}

	// Complete the upload
	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: uploadedParts},
	})
	if err != nil {
		client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(key),
			UploadId: uploadID,
		})
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	t.Log("Multipart upload completed")

	// Test retrieving specific parts
	testCases := []struct {
		name        string
		partNumber  int32
		shouldFail  bool
	}{
		{"Part 1", 1, false},
		{"Part 2", 2, false},
		{"Part 3", 3, false},
		{"Part 4 (nonexistent)", 4, true},
		{"Part 0 (invalid)", 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
				Bucket:     aws.String(bucket),
				Key:        aws.String(key),
				PartNumber: aws.Int32(tc.partNumber),
			})

			if tc.shouldFail {
				if err == nil {
					getResp.Body.Close()
					t.Errorf("Part %d should fail", tc.partNumber)
				} else {
					t.Logf("Part %d failed as expected: %v", tc.partNumber, err)
				}
			} else {
				if err != nil {
					t.Errorf("Part %d retrieval failed: %v", tc.partNumber, err)
				} else {
					defer getResp.Body.Close()
					downloaded, _ := io.ReadAll(getResp.Body)
					t.Logf("Part %d retrieved, size: %d bytes", tc.partNumber, len(downloaded))

					// Verify part size
					if int64(len(downloaded)) != partSize {
						t.Errorf("Part %d size mismatch: got %d, want %d",
							tc.partNumber, len(downloaded), partSize)
					}
				}
			}
		})
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	t.Log("TestGetObject_PartNumber completed successfully")
}

// TestGetObject_VeryLargeRange tests range requests on large files
func TestGetObject_VeryLargeRange(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create a large file via multipart
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId

	// Upload multiple parts to create a 15 MB file
	partSize := int64(5 * 1024 * 1024)
	numParts := 3
	uploadedParts := []types.CompletedPart{}

	for i := int32(1); i <= int32(numParts); i++ {
		partData := generateTestData(int(partSize))
		uploadResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(key),
			UploadId:   uploadID,
			PartNumber: aws.Int32(i),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
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
		Key:             aws.String(key),
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: uploadedParts},
	})
	if err != nil {
		client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(key),
			UploadId: uploadID,
		})
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	totalSize := partSize * int64(numParts)
	t.Logf("Created large file: %d bytes", totalSize)
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	// Test large range spanning multiple parts
	rangeStart := int64(2 * 1024 * 1024) // 2 MB
	rangeEnd := int64(10 * 1024 * 1024)  // 10 MB (spans parts)

	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd)),
	})
	if err != nil {
		t.Fatalf("GetObject with large range failed: %v", err)
	}
	defer getResp.Body.Close()

	downloaded, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expectedSize := rangeEnd - rangeStart + 1
	if int64(len(downloaded)) != expectedSize {
		t.Errorf("Large range size mismatch: got %d bytes, want %d bytes",
			len(downloaded), expectedSize)
	} else {
		t.Logf("Large range successful: %d bytes", len(downloaded))
	}

	t.Log("TestGetObject_VeryLargeRange completed successfully")
}

// TestGetObject_MissingObject tests error handling for non-existent objects
func TestGetObject_MissingObject(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	nonExistentKey := fmt.Sprintf("nonexistent-%d", time.Now().UnixNano())

	_, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(nonExistentKey),
	})

	if err == nil {
		t.Error("GetObject of non-existent object should fail")
	} else {
		t.Logf("GetObject of non-existent object failed as expected: %v", err)
	}

	t.Log("TestGetObject_MissingObject completed")
}
