// Concurrent, shuffled, unaligned multipart round-trip test
// Plan §8.11 acceptance - validates v3 multipart with real-world patterns
//
// This test MUST:
// - Pass under -race detector (validates concurrency safety)
// - Run in short mode (no testing.Short() skip) - part of go test ./... -short
// - Test through real S3 HTTP handlers
//
// Requirements:
// - ARMOR_ENDPOINT environment variable (defaults to http://localhost:9000)
// - ARMOR_TEST_BUCKET for testBucket name
// - ARMOR_ACCESS_KEY_ID and ARMOR_SECRET_ACCESS_KEY for credentials

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// readFull reads the entire reader into a byte slice
func readFull(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// getEnvOr gets an environment variable with a fallback value
func getEnvOr(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// generateTestKey creates a unique test key
func generateTestKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}
	return fmt.Sprintf("test-concurrent-mp-%x", b)
}

// createS3Client creates an S3 client for testing
func createS3Client(t *testing.T, armorEndpoint string) *s3.Client {
	t.Helper()

	accessKey := os.Getenv("ARMOR_ACCESS_KEY_ID")
	secretKey := os.Getenv("ARMOR_SECRET_ACCESS_KEY")

	// Use test credentials if not set
	if accessKey == "" {
		accessKey = "test_access_key"
	}
	if secretKey == "" {
		secretKey = "test_secret_key"
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey,
			secretKey,
			"",
		)),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		t.Fatalf("Failed to load AWS config: %v", err)
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(armorEndpoint)
		o.UsePathStyle = true
	})
}

// Get test bucket from environment or use default
var testBucket = getEnvOr("ARMOR_TEST_BUCKET", "test-bucket")

// TestConcurrentMultipartRoundtrip tests concurrent multipart uploads with multiple
// real-world patterns, then validates with full and range downloads, and verifies
// abort cleanup. This test is part of Plan §8.11 acceptance criteria.
//
// This test MUST:
// - Pass under -race detector (validates concurrency safety)
// - Run in short mode (no testing.Short() skip)
// - Test through real S3 HTTP handlers
func TestConcurrentMultipartRoundtrip(t *testing.T) {
	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Run all test patterns
	t.Run("BarmanPattern", func(t *testing.T) {
		testBarmanPattern(t, client, ctx)
	})

	t.Run("AWSClIDefaults", func(t *testing.T) {
		testAWSClIDefaults(t, client, ctx)
	})

	t.Run("SingleByteFinalPart", func(t *testing.T) {
		testSingleByteFinalPart(t, client, ctx)
	})

	t.Run("AbortRemovesState", func(t *testing.T) {
		testAbortRemovesState(t, client, ctx)
	})
}

// testBarmanPattern tests barman's backup pattern: chunk_size + n*512 part sizes
// This validates non-uniform part sizes that are multiples of 512 bytes (sector alignment)
func testBarmanPattern(t *testing.T, client *s3.Client, ctx context.Context) {
	t.Helper()

	key := generateTestKey(t)

	// Barman uses chunk_size + n*512 where chunk_size varies by backup size
	// Simulating: 10MiB base + multiples of 512 bytes
	partSizes := []int{
		10*1024*1024 + 512*0,   // 10 MiB exactly
		10*1024*1024 + 512*17,  // 10 MiB + 8704 bytes (uncompressed wal segment)
		10*1024*1024 + 512*33,  // 10 MiB + 16896 bytes
		10*1024*1024 + 512*65,  // 10 MiB + 33280 bytes
		5*1024*1024 + 512*8,    // 5 MiB + 4096 bytes (final smaller part)
	}

	// Build test data
	var testData []byte
	for _, size := range partSizes {
		partData := make([]byte, size)
		rand.Read(partData)
		testData = append(testData, partData...)
	}

	t.Logf("Barman pattern: %d parts, total %d bytes", len(partSizes), len(testData))

	// Create and upload multipart
	uploadID, partETags := uploadMultipartWithParts(t, client, ctx, testBucket, key, testData, partSizes)

	// Complete the upload
	completeMultipartUpload(t, client, ctx, testBucket, key, uploadID, partETags)

	// Verify full download and ranges
	verifyRoundtrip(t, client, ctx, testBucket, key, testData, 20)

	t.Logf("Barman pattern test completed successfully")
}

// testAWSClIDefaults tests AWS CLI default behavior:
// - 8 MiB part size (AWS CLI default for multipart)
// - Concurrency 10 (AWS CLI default)
// - Random arrival order (parts uploaded out of sequence)
func testAWSClIDefaults(t *testing.T, client *s3.Client, ctx context.Context) {
	t.Helper()

	key := generateTestKey(t)

	// AWS CLI default: 8 MiB parts
	const partSize = 8 * 1024 * 1024
	const numParts = 15 // Total 120 MiB
	const concurrency = 10

	totalSize := partSize * numParts
	testData := make([]byte, totalSize)
	rand.Read(testData)

	t.Logf("AWS CLI pattern: %d parts of %d bytes, concurrency %d, total %d bytes",
		numParts, partSize, concurrency, totalSize)

	// Create multipart upload
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := *createResp.UploadId

	// Shuffle part order for upload (test out-of-order arrival)
	partOrder := make([]int, numParts)
	for i := 0; i < numParts; i++ {
		partOrder[i] = i
	}
	rand.Shuffle(len(partOrder), func(i, j int) {
		partOrder[i], partOrder[j] = partOrder[j], partOrder[i]
	})

	// Upload with concurrency using semaphore pattern
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	partETags := make([]types.CompletedPart, numParts)
	errors := make(chan error, numParts)

	startTime := time.Now()
	for _, partIdx := range partOrder {
		wg.Add(1)
		go func(partNum int) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			partStart := partNum * partSize
			partEnd := partStart + partSize
			partData := testData[partStart:partEnd]

			uploadResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
				Bucket:     aws.String(testBucket),
				Key:        aws.String(key),
				UploadId:   aws.String(uploadID),
				PartNumber: aws.Int32(int32(partNum + 1)),
				Body:       bytes.NewReader(partData),
			})
			if err != nil {
				errors <- fmt.Errorf("UploadPart %d failed: %w", partNum+1, err)
				return
			}

			// Store part ETag safely
			mu.Lock()
			partETags[partNum] = types.CompletedPart{
				ETag:       uploadResp.ETag,
				PartNumber: aws.Int32(int32(partNum + 1)),
			}
			mu.Unlock()
		}(partIdx)
	}

	// Wait for all uploads
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Fatalf("Concurrent upload error: %v", err)
	}

	uploadDuration := time.Since(startTime)
	t.Logf("Concurrent upload completed in %v (%.2f MiB/s)",
		uploadDuration,
		float64(totalSize)/(1024*1024)/uploadDuration.Seconds())

	// Complete the upload
	completeMultipartUpload(t, client, ctx, testBucket, key, uploadID, partETags)

	// Verify full download and ranges
	verifyRoundtrip(t, client, ctx, testBucket, key, testData, 20)

	t.Logf("AWS CLI pattern test completed successfully")
}

// testSingleByteFinalPart tests the edge case of a 1-byte final part
// This validates that the system handles minimum-size final parts correctly
func testSingleByteFinalPart(t *testing.T, client *s3.Client, ctx context.Context) {
	t.Helper()

	key := generateTestKey(t)

	// Create large parts followed by a 1-byte final part
	partSizes := []int{
		5 * 1024 * 1024, // 5 MiB
		5 * 1024 * 1024, // 5 MiB
		5 * 1024 * 1024, // 5 MiB
		1,               // 1 byte final part
	}

	// Build test data
	var testData []byte
	for _, size := range partSizes {
		partData := make([]byte, size)
		rand.Read(partData)
		testData = append(testData, partData...)
	}

	t.Logf("Single-byte final part: %d parts, total %d bytes", len(partSizes), len(testData))

	// Create and upload multipart
	uploadID, partETags := uploadMultipartWithParts(t, client, ctx, testBucket, key, testData, partSizes)

	// Complete the upload
	completeMultipartUpload(t, client, ctx, testBucket, key, uploadID, partETags)

	// Verify full download and ranges (including ranges that hit the final byte)
	verifyRoundtrip(t, client, ctx, testBucket, key, testData, 20)

	// Specific test: range that includes the final byte
	t.Run("FinalByteRange", func(t *testing.T) {
		start := int64(len(testData) - 10)
		end := int64(len(testData) - 1)

		getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String(key),
			Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
		})
		if err != nil {
			t.Fatalf("GetObject with final byte range failed: %v", err)
		}
		defer getResp.Body.Close()

		downloadedData, err := readFull(getResp.Body)
		if err != nil {
			t.Fatalf("Failed to read range data: %v", err)
		}

		expectedData := testData[start : end+1]
		if !bytes.Equal(downloadedData, expectedData) {
			t.Errorf("Final byte range data mismatch")
		}
	})

	t.Logf("Single-byte final part test completed successfully")
}

// testAbortRemovesState verifies that AbortMultipartUpload actually removes
// all state associated with the upload, including parts and metadata
func testAbortRemovesState(t *testing.T, client *s3.Client, ctx context.Context) {
	t.Helper()

	key := generateTestKey(t)

	// Create multipart upload
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := *createResp.UploadId

	// Upload several parts
	partSizes := []int{5 * 1024 * 1024, 6 * 1024 * 1024, 5 * 1024 * 1024}
	for i, size := range partSizes {
		partData := make([]byte, size)
		rand.Read(partData)

		_, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(testBucket),
			Key:        aws.String(key),
			UploadId:   aws.String(uploadID),
			PartNumber: aws.Int32(int32(i + 1)),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", i+1, err)
		}
	}

	t.Logf("Uploaded %d parts before abort", len(partSizes))

	// Verify parts are listed
	listResp, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(testBucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		t.Fatalf("ListParts before abort failed: %v", err)
	}
	if len(listResp.Parts) != len(partSizes) {
		t.Errorf("Expected %d parts, got %d", len(partSizes), len(listResp.Parts))
	}

	// Verify upload appears in ListMultipartUploads
	uploadsResp, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Fatalf("ListMultipartUploads before abort failed: %v", err)
	}

	found := false
	for _, upload := range uploadsResp.Uploads {
		if *upload.Key == key && *upload.UploadId == uploadID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Upload not found in ListMultipartUploads before abort")
	}

	// Abort the upload
	_, err = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(testBucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		t.Fatalf("AbortMultipartUpload failed: %v", err)
	}

	// Verify upload no longer appears in ListMultipartUploads
	uploadsResp2, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Fatalf("ListMultipartUploads after abort failed: %v", err)
	}

	for _, upload := range uploadsResp2.Uploads {
		if *upload.Key == key && *upload.UploadId == uploadID {
			t.Error("Aborted upload still appears in ListMultipartUploads")
		}
	}

	// Verify parts are no longer accessible (or return empty/error)
	listResp2, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(testBucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	// After abort, ListParts may return error or empty list - both are acceptable
	if err == nil && len(listResp2.Parts) > 0 {
		t.Error("Parts still visible after abort")
	}

	// Verify the final object was not created
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Error("Object exists after abort - should have been cleaned up")
	}

	t.Logf("Abort cleanup verified successfully")
}

// uploadMultipartWithParts is a helper that uploads multipart data with specific part sizes
func uploadMultipartWithParts(t *testing.T, client *s3.Client, ctx context.Context, testBucket, key string, testData []byte, partSizes []int) (string, []types.CompletedPart) {
	t.Helper()

	// Create multipart upload
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := *createResp.UploadId

	// Upload parts with specified sizes
	var partETags []types.CompletedPart
	offset := 0
	for i, size := range partSizes {
		partData := testData[offset : offset+size]

		uploadResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(testBucket),
			Key:        aws.String(key),
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

	return uploadID, partETags
}

// completeMultipartUpload is a helper that completes a multipart upload
func completeMultipartUpload(t *testing.T, client *s3.Client, ctx context.Context, testBucket, key, uploadID string, partETags []types.CompletedPart) {
	t.Helper()

	_, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(testBucket),
		Key:             aws.String(key),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: partETags},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
}

// verifyRoundtrip performs full object download and random range validation
func verifyRoundtrip(t *testing.T, client *s3.Client, ctx context.Context, testBucket, key string, testData []byte, numRanges int) {
	t.Helper()

	// Test 1: Full object GET
	t.Run("FullGET", func(t *testing.T) {
		getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucket),
			Key:    aws.String(key),
		})
		if err != nil {
			t.Fatalf("GetObject failed: %v", err)
		}
		defer getResp.Body.Close()

		downloadedData, err := readFull(getResp.Body)
		if err != nil {
			t.Fatalf("Failed to read object data: %v", err)
		}

		if !bytes.Equal(downloadedData, testData) {
			t.Errorf("Full download data mismatch")
			if len(downloadedData) != len(testData) {
				t.Errorf("Size mismatch: got %d bytes, want %d bytes", len(downloadedData), len(testData))
			}
		}

		// Verify content length
		if *getResp.ContentLength != int64(len(testData)) {
			t.Errorf("ContentLength mismatch: got %d, want %d", *getResp.ContentLength, len(testData))
		}
	})

	// Test 2: Random ranges
	t.Run("RandomRanges", func(t *testing.T) {
		for i := 0; i < numRanges; i++ {
			t.Run(fmt.Sprintf("Range_%d", i), func(t *testing.T) {
				// Generate random range
				start := rand.Int63n(int64(len(testData)))
				end := start + rand.Int63n(int64(len(testData))-start)
				if end >= int64(len(testData)) {
					end = int64(len(testData)) - 1
				}

				// Suffix range test (last N bytes)
				if rand.Intn(10) == 0 && i < numRanges/2 {
					// Test suffix range: bytes=-N (last N bytes)
					suffixLength := rand.Int63n(10000) + 1
					if suffixLength > int64(len(testData)) {
						suffixLength = int64(len(testData))
					}
					start = int64(len(testData)) - suffixLength
					end = int64(len(testData)) - 1

					getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
						Bucket: aws.String(testBucket),
						Key:    aws.String(key),
						Range:  aws.String(fmt.Sprintf("bytes=-%d", suffixLength)),
					})
					if err != nil {
						t.Fatalf("GetObject with suffix range failed: %v", err)
					}
					defer getResp.Body.Close()

					downloadedData, err := readFull(getResp.Body)
					if err != nil {
						t.Fatalf("Failed to read range data: %v", err)
					}

					expectedData := testData[start : end+1]
					if !bytes.Equal(downloadedData, expectedData) {
						t.Errorf("Suffix range data mismatch for range %d-%d (suffix %d)", start, end, suffixLength)
					}
					return
				}

				// Normal range
				getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
					Bucket: aws.String(testBucket),
					Key:    aws.String(key),
					Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
				})
				if err != nil {
					t.Fatalf("GetObject with range failed: %v", err)
				}
				defer getResp.Body.Close()

				downloadedData, err := readFull(getResp.Body)
				if err != nil {
					t.Fatalf("Failed to read range data: %v", err)
				}

				expectedData := testData[start : end+1]
				if !bytes.Equal(downloadedData, expectedData) {
					t.Errorf("Range data mismatch for range %d-%d", start, end)
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
		}
	})
}
