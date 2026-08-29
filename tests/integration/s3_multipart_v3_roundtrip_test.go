// +build integration

package integration

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestV3MultipartRoundTrip tests the full v3 multipart upload and download cycle.
func TestV3MultipartRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup(t)
	defer teardown(t)

	// Create test data that will be split across multiple parts
	// Each part should be at least 5 MiB (B2 minimum) except the last
	partSize := 5 * 1024 * 1024 // 5 MiB
	numParts := 3
	totalSize := partSize*numParts + 1024 // Add some extra for the last part

	testData := make([]byte, totalSize)
	rand.Read(testData)

	// Calculate expected checksums
	expectedMD5 := md5.Sum(testData)
	expectedETag := hex.EncodeToString(expectedMD5[:]) + "-3" // 3 parts

	// Create multipart upload
	createResp, err := s3Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String("v3-multipart-roundtrip-test"),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := *createResp.UploadId

	// Upload parts
	var partETags []types.CompletedPart
	for i := 0; i < numParts; i++ {
		partStart := i * partSize
		partEnd := partStart + partSize
		if i == numParts-1 {
			partEnd = len(testData) // Last part may be smaller
		}

		partData := testData[partStart:partEnd]

		uploadResp, err := s3Client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(testBucket),
			Key:        aws.String("v3-multipart-roundtrip-test"),
			UploadId:   aws.String(uploadID),
			PartNumber: aws.Int32(int32(i + 1)),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", i+1, err)
		}

		partETags = append(partETags, types.CompletedPart{
			ETag:       uploadResp.ETag,
			PartNumber: aws.Int32(int32(i + 1)),
		})
	}

	// Complete multipart upload
	completeResp, err := s3Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(testBucket),
		Key:             aws.String("v3-multipart-roundtrip-test"),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: partETags},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	// Verify ETag matches expected
	if *completeResp.ETag != expectedETag {
		t.Errorf("ETag mismatch: got %s, want %s", *completeResp.ETag, expectedETag)
	}

	// Test 1: Full object GET should return byte-identical data
	t.Run("FullGET", func(t *testing.T) {
		getResp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String("v3-multipart-roundtrip-test"),
		})
		if err != nil {
			t.Fatalf("GetObject failed: %v", err)
		}
		defer getResp.Body.Close()

		downloadedData, err := io.ReadAll(getResp.Body)
		if err != nil {
			t.Fatalf("Failed to read object data: %v", err)
		}

		if !bytes.Equal(downloadedData, testData) {
			t.Errorf("Downloaded data does not match uploaded data")
			if len(downloadedData) != len(testData) {
				t.Errorf("Size mismatch: got %d bytes, want %d bytes", len(downloadedData), len(testData))
			}
		}

		// Verify metadata
		if *getResp.ContentLength != int64(len(testData)) {
			t.Errorf("ContentLength mismatch: got %d, want %d", *getResp.ContentLength, len(testData))
		}

		if getResp.ETag == nil || *getResp.ETag != expectedETag {
			t.Errorf("ETag mismatch: got %s, want %s", *getResp.ETag, expectedETag)
		}
	})

	// Test 2: Range request inside a single part
	t.Run("RangeInsideSinglePart", func(t *testing.T) {
		// Request a range that's entirely within part 1
		start := int64(partSize + 100)
		end := int64(partSize + 200)

		getResp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String("v3-multipart-roundtrip-test"),
			Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
		})
		if err != nil {
			t.Fatalf("GetObject with range failed: %v", err)
		}
		defer getResp.Body.Close()

		downloadedData, err := io.ReadAll(getResp.Body)
		if err != nil {
			t.Fatalf("Failed to read range data: %v", err)
		}

		expectedData := testData[start : end+1]
		if !bytes.Equal(downloadedData, expectedData) {
			t.Errorf("Range data mismatch")
		}

		// Verify Content-Range header
		if getResp.ContentRange == nil {
			t.Errorf("Missing Content-Range header")
		} else {
			expectedRange := fmt.Sprintf("bytes %d-%d/%d", start, end, len(testData))
			if *getResp.ContentRange != expectedRange {
				t.Errorf("Content-Range mismatch: got %s, want %s", *getResp.ContentRange, expectedRange)
			}
		}
	})

	// Test 3: Range request straddling two parts
	t.Run("RangeStraddlingTwoParts", func(t *testing.T) {
		// Request a range that starts near the end of part 1 and continues into part 2
		start := int64(partSize - 100)
		end := int64(partSize + 100)

		getResp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String("v3-multipart-roundtrip-test"),
			Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
		})
		if err != nil {
			t.Fatalf("GetObject with straddling range failed: %v", err)
		}
		defer getResp.Body.Close()

		downloadedData, err := io.ReadAll(getResp.Body)
		if err != nil {
			t.Fatalf("Failed to read straddling range data: %v", err)
		}

		expectedData := testData[start : end+1]
		if !bytes.Equal(downloadedData, expectedData) {
			t.Errorf("Straddling range data mismatch")
		}

		// Verify Content-Range header
		if getResp.ContentRange == nil {
			t.Errorf("Missing Content-Range header")
		} else {
			expectedRange := fmt.Sprintf("bytes %d-%d/%d", start, end, len(testData))
			if *getResp.ContentRange != expectedRange {
				t.Errorf("Content-Range mismatch: got %s, want %s", *getResp.ContentRange, expectedRange)
			}
		}
	})

	// Test 4: Range request covering the last (short) part
	t.Run("RangeCoveringLastPart", func(t *testing.T) {
		// Request a range that starts in part 2 and covers the last part
		start := int64(partSize*2 - 50)
		end := int64(len(testData) - 1)

		getResp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String("v3-multipart-roundtrip-test"),
			Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
		})
		if err != nil {
			t.Fatalf("GetObject with last-part range failed: %v", err)
		}
		defer getResp.Body.Close()

		downloadedData, err := io.ReadAll(getResp.Body)
		if err != nil {
			t.Fatalf("Failed to read last-part range data: %v", err)
		}

		expectedData := testData[start : end+1]
		if !bytes.Equal(downloadedData, expectedData) {
			t.Errorf("Last-part range data mismatch")
		}

		// Verify Content-Range header
		if getResp.ContentRange == nil {
			t.Errorf("Missing Content-Range header")
		} else {
			expectedRange := fmt.Sprintf("bytes %d-%d/%d", start, end, len(testData))
			if *getResp.ContentRange != expectedRange {
				t.Errorf("Content-Range mismatch: got %s, want %s", *getResp.ContentRange, expectedRange)
			}
		}
	})

	// Test 5: Sidecar cache invalidation on overwrite
	t.Run("SidecarCacheInvalidation", func(t *testing.T) {
		// Upload a new version of the same object with different data
		newTestData := make([]byte, totalSize/2) // Smaller this time
		rand.Read(newTestData)

		newMD5 := md5.Sum(newTestData)
		newETag := hex.EncodeToString(newMD5[:])

		// Simple PUT (not multipart) to overwrite
		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String("v3-multipart-roundtrip-test"),
			Body:   bytes.NewReader(newTestData),
		})
		if err != nil {
			t.Fatalf("PutObject overwrite failed: %v", err)
		}

		// Verify we get the new data
		getResp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String("v3-multipart-roundtrip-test"),
		})
		if err != nil {
			t.Fatalf("GetObject after overwrite failed: %v", err)
		}
		defer getResp.Body.Close()

		downloadedData, err := io.ReadAll(getResp.Body)
		if err != nil {
			t.Fatalf("Failed to read overwritten data: %v", err)
		}

		if !bytes.Equal(downloadedData, newTestData) {
			t.Errorf("Downloaded data does not match new uploaded data (cache not invalidated)")
		}

		if getResp.ETag == nil || *getResp.ETag != newETag {
			t.Errorf("ETag after overwrite mismatch: got %s, want %s", *getResp.ETag, newETag)
		}
	})
}

// TestV3MultipartUnalignedParts tests v3 multipart with random unaligned part sizes.
// This validates that the part-level offset mapping works correctly without
// requiring uniform part sizes (v3 doesn't have the ADR-005 constraint).
func TestV3MultipartUnalignedParts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup(t)
	defer teardown(t)

	// Create parts with random sizes (all ≥ 5 MiB except last)
	partSizes := []int{
		5*1024*1024 + 12345, // 5 MiB + random offset
		6*1024*1024 + 67890, // 6 MiB + different offset
		5*1024*1024 + 11111, // 5 MiB + another offset
		2*1024*1024,         // Last part can be smaller
	}

	// Build test data
	var testData []byte
	for _, size := range partSizes {
		partData := make([]byte, size)
		rand.Read(partData)
		testData = append(testData, partData...)
	}

	// Create multipart upload
	createResp, err := s3Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String("v3-multipart-unaligned-test"),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := *createResp.UploadId

	// Upload parts with their specific sizes
	var partETags []types.CompletedPart
	offset := 0
	for i, size := range partSizes {
		partData := testData[offset : offset+size]

		uploadResp, err := s3Client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(testBucket),
			Key:        aws.String("v3-multipart-unaligned-test"),
			UploadId:   aws.String(uploadID),
			PartNumber: aws.Int32(int32(i + 1)),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", i+1, err)
		}

		partETags = append(partETags, types.CompletedPart{
			ETag:       uploadResp.ETag,
			PartNumber: aws.Int32(int32(i + 1)),
		})

		offset += size
	}

	// Complete multipart upload
	_, err = s3Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(testBucket),
		Key:             aws.String("v3-multipart-unaligned-test"),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: partETags},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	// Verify full object GET
	t.Run("FullGET_Unaligned", func(t *testing.T) {
		getResp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String("v3-multipart-unaligned-test"),
		})
		if err != nil {
			t.Fatalf("GetObject failed: %v", err)
		}
		defer getResp.Body.Close()

		downloadedData, err := io.ReadAll(getResp.Body)
		if err != nil {
			t.Fatalf("Failed to read object data: %v", err)
		}

		if !bytes.Equal(downloadedData, testData) {
			t.Errorf("Unaligned parts: downloaded data does not match uploaded data")
		}
	})

	// Test range crossing part boundaries with unaligned sizes
	t.Run("RangeUnalignedCrossings", func(t *testing.T) {
		// Find the boundary between part 1 and part 2
		boundaryOffset := partSizes[0]

		// Request range that crosses this boundary
		start := boundaryOffset - 1000
		end := boundaryOffset + 1000

		getResp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String("v3-multipart-unaligned-test"),
			Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
		})
		if err != nil {
			t.Fatalf("GetObject with range failed: %v", err)
		}
		defer getResp.Body.Close()

		downloadedData, err := io.ReadAll(getResp.Body)
		if err != nil {
			t.Fatalf("Failed to read range data: %v", err)
		}

		expectedData := testData[start : end+1]
		if !bytes.Equal(downloadedData, expectedData) {
			t.Errorf("Unaligned range data mismatch")
		}
	})
}

// TestV3MultipartSmallRanges tests very small ranges to validate offset precision.
func TestV3MultipartSmallRanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup(t)
	defer teardown(t)

	// Create a simple multipart object
	partSize := 5 * 1024 * 1024
	testData := make([]byte, partSize*2)
	rand.Read(testData)

	// Upload as multipart
	createResp, err := s3Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String("v3-small-ranges-test"),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := *createResp.UploadId

	// Upload two parts
	var partETags []types.CompletedPart
	for i := 0; i < 2; i++ {
		partData := testData[i*partSize : (i+1)*partSize]

		uploadResp, err := s3Client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(testBucket),
			Key:        aws.String("v3-small-ranges-test"),
			UploadId:   aws.String(uploadID),
			PartNumber: aws.Int32(int32(i + 1)),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", i+1, err)
		}

		partETags = append(partETags, types.CompletedPart{
			ETag:       uploadResp.ETag,
			PartNumber: aws.Int32(int32(i + 1)),
		})
	}

	// Complete
	_, err = s3Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(testBucket),
		Key:             aws.String("v3-small-ranges-test"),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: partETags},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	// Test various small ranges
	testRanges := []struct {
		start, end int64
	}{
		{0, 0},           // First byte
		{100, 100},       // Arbitrary byte
		{partSize - 1, partSize - 1}, // Last byte of first part
		{partSize, partSize},         // First byte of second part
		{partSize + 1000, partSize + 1000}, // Middle of second part
		{int64(len(testData) - 1), int64(len(testData) - 1)}, // Last byte
	}

	for _, r := range testRanges {
		t.Run(fmt.Sprintf("Range_%d_%d", r.start, r.end), func(t *testing.T) {
			getResp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(testBucket),
				Key:    aws.String("v3-small-ranges-test"),
				Range:  aws.String(fmt.Sprintf("bytes=%d-%d", r.start, r.end)),
			})
			if err != nil {
				t.Fatalf("GetObject with range failed: %v", err)
			}
			defer getResp.Body.Close()

			downloadedData, err := io.ReadAll(getResp.Body)
			if err != nil {
				t.Fatalf("Failed to read range data: %v", err)
			}

			expectedData := testData[r.start : r.end+1]
			if !bytes.Equal(downloadedData, expectedData) {
				t.Errorf("Small range data mismatch for range %d-%d", r.start, r.end)
			}
		})
	}
}
