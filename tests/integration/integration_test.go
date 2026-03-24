//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Environment variables required for integration tests:
// ARMOR_INTEGRATION_TEST=1              - Must be set to run tests
// ARMOR_B2_ACCESS_KEY_ID                - B2 application key ID
// ARMOR_B2_SECRET_ACCESS_KEY            - B2 application key secret
// ARMOR_B2_REGION                       - B2 region (e.g., us-east-005)
// ARMOR_BUCKET                          - B2 bucket name
// ARMOR_CF_DOMAIN                       - Cloudflare domain CNAME'd to B2
// ARMOR_MEK                             - Master encryption key (hex, 32 bytes)
// ARMOR_AUTH_ACCESS_KEY                 - ARMOR access key for client auth
// ARMOR_AUTH_SECRET_KEY                 - ARMOR secret key for client auth

var (
	b2AccessKeyID     string
	b2SecretAccessKey string
	b2Region          string
	bucket            string
	cfDomain          string
	mek               string
	armorAccessKey    string
	armorSecretKey    string
)

func TestMain(m *testing.M) {
	// Skip if integration test flag not set
	if os.Getenv("ARMOR_INTEGRATION_TEST") != "1" {
		fmt.Println("Skipping integration tests: ARMOR_INTEGRATION_TEST not set")
		os.Exit(0)
	}

	// Load required environment variables
	b2AccessKeyID = os.Getenv("ARMOR_B2_ACCESS_KEY_ID")
	b2SecretAccessKey = os.Getenv("ARMOR_B2_SECRET_ACCESS_KEY")
	b2Region = os.Getenv("ARMOR_B2_REGION")
	bucket = os.Getenv("ARMOR_BUCKET")
	cfDomain = os.Getenv("ARMOR_CF_DOMAIN")
	mek = os.Getenv("ARMOR_MEK")
	armorAccessKey = os.Getenv("ARMOR_AUTH_ACCESS_KEY")
	armorSecretKey = os.Getenv("ARMOR_AUTH_SECRET_KEY")

	missing := []string{}
	if b2AccessKeyID == "" {
		missing = append(missing, "ARMOR_B2_ACCESS_KEY_ID")
	}
	if b2SecretAccessKey == "" {
		missing = append(missing, "ARMOR_B2_SECRET_ACCESS_KEY")
	}
	if b2Region == "" {
		missing = append(missing, "ARMOR_B2_REGION")
	}
	if bucket == "" {
		missing = append(missing, "ARMOR_BUCKET")
	}
	if cfDomain == "" {
		missing = append(missing, "ARMOR_CF_DOMAIN")
	}
	if mek == "" {
		missing = append(missing, "ARMOR_MEK")
	}
	if armorAccessKey == "" {
		missing = append(missing, "ARMOR_AUTH_ACCESS_KEY")
	}
	if armorSecretKey == "" {
		missing = append(missing, "ARMOR_AUTH_SECRET_KEY")
	}

	if len(missing) > 0 {
		fmt.Printf("Skipping integration tests: missing environment variables: %s\n", strings.Join(missing, ", "))
		os.Exit(0)
	}

	os.Exit(m.Run())
}

// generateTestKey creates a unique test key with a prefix
func generateTestKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}
	return fmt.Sprintf("test-%s/%x", t.Name(), b)
}

// generateTestData creates test data of a given size
func generateTestData(size int) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(i % 256)
	}
	return data
}

// computeSHA256 computes SHA-256 hash of data
func computeSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// createS3Client creates an S3 client pointing at ARMOR
func createS3Client(t *testing.T, armorEndpoint string) *s3.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			armorAccessKey,
			armorSecretKey,
			"",
		)),
		config.WithRegion("us-east-1"), // ARMOR doesn't care about region
	)
	if err != nil {
		t.Fatalf("Failed to load AWS config: %v", err)
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(armorEndpoint)
		o.UsePathStyle = true
	})
}

// TestPutGetRoundtrip tests basic upload and download through ARMOR
func TestPutGetRoundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	// This test requires ARMOR to be running
	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)
	testData := generateTestData(1024 * 1024) // 1 MB

	// Upload
	t.Logf("Uploading %d bytes to %s/%s", len(testData), bucket, key)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	t.Logf("Upload successful")

	// Download
	t.Logf("Downloading from %s/%s", bucket, key)
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer resp.Body.Close()

	downloaded, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Verify content matches
	if !bytes.Equal(testData, downloaded) {
		t.Errorf("Downloaded data doesn't match uploaded data: got %d bytes, want %d bytes",
			len(downloaded), len(testData))
	}

	// Verify size
	if resp.ContentLength != nil && *resp.ContentLength != int64(len(testData)) {
		t.Errorf("ContentLength mismatch: got %d, want %d", *resp.ContentLength, len(testData))
	}

	t.Logf("Download successful, content verified")

	// Cleanup
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Logf("Warning: failed to delete test object: %v", err)
	}
}

// TestRangeRead tests range requests through ARMOR
func TestRangeRead(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)
	testData := generateTestData(256 * 1024) // 256 KB

	// Upload
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

	// Test various range reads
	ranges := []struct {
		start, end int64
	}{
		{0, 100},            // First 100 bytes
		{1000, 2000},        // Middle chunk
		{int64(len(testData)) - 100, int64(len(testData)) - 1}, // Last 100 bytes
		{0, int64(len(testData)) - 1}, // Full file
	}

	for _, r := range ranges {
		rangeHeader := fmt.Sprintf("bytes=%d-%d", r.start, r.end)
		t.Run(rangeHeader, func(t *testing.T) {
			resp, err := client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Range:  aws.String(rangeHeader),
			})
			if err != nil {
				t.Fatalf("GetObject with range %s failed: %v", rangeHeader, err)
			}
			defer resp.Body.Close()

			downloaded, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			expected := testData[r.start : r.end+1]
			if !bytes.Equal(expected, downloaded) {
				t.Errorf("Range read mismatch: got %d bytes, want %d bytes",
					len(downloaded), len(expected))
				// Show first difference
				for i := 0; i < len(expected) && i < len(downloaded); i++ {
					if expected[i] != downloaded[i] {
						t.Errorf("First difference at offset %d: got %d, want %d",
							i, downloaded[i], expected[i])
						break
					}
				}
			}

			expectedLen := r.end - r.start + 1
			if resp.ContentLength != nil && *resp.ContentLength != expectedLen {
				t.Errorf("ContentLength mismatch: got %d, want %d",
					*resp.ContentLength, expectedLen)
			}
		})
	}
}

// TestHeadObject tests HeadObject returns correct plaintext size
func TestHeadObject(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)
	testData := generateTestData(50 * 1024) // 50 KB

	// Upload
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

	// Head
	resp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}

	// Verify plaintext size
	if resp.ContentLength != nil && *resp.ContentLength != int64(len(testData)) {
		t.Errorf("HeadObject ContentLength mismatch: got %d, want %d (plaintext size)",
			*resp.ContentLength, len(testData))
	}

	t.Logf("HeadObject returned correct plaintext size: %d", *resp.ContentLength)
}

// TestListObjectsV2 tests listing with size correction
func TestListObjectsV2(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	prefix := fmt.Sprintf("test-list-%d/", time.Now().UnixNano())

	// Upload multiple objects with known sizes
	sizes := []int{10 * 1024, 20 * 1024, 30 * 1024}
	keys := []string{}
	for i, size := range sizes {
		key := fmt.Sprintf("%sfile-%d.bin", prefix, i)
		keys = append(keys, key)
		testData := generateTestData(size)
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(testData),
		})
		if err != nil {
			t.Fatalf("PutObject failed for %s: %v", key, err)
		}
	}

	// Cleanup
	defer func() {
		for _, key := range keys {
			client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
		}
	}()

	// List
	resp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 failed: %v", err)
	}

	// Verify sizes
	if len(resp.Contents) != len(sizes) {
		t.Fatalf("ListObjectsV2 returned %d objects, want %d", len(resp.Contents), len(sizes))
	}

	for i, obj := range resp.Contents {
		expectedSize := int64(sizes[i])
		if obj.Size != nil && *obj.Size != expectedSize {
			t.Errorf("Object %s size mismatch: got %d, want %d (plaintext size)",
				*obj.Key, *obj.Size, expectedSize)
		}
	}

	t.Logf("ListObjectsV2 returned correct plaintext sizes for all objects")
}

// TestDeleteObject tests delete through ARMOR
func TestDeleteObject(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)
	testData := generateTestData(1024)

	// Upload
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}

	// Delete
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("DeleteObject failed: %v", err)
	}

	// Verify deleted - GetObject should fail
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Error("GetObject succeeded after delete, expected error")
	}
	t.Logf("Delete verified - object no longer accessible")
}

// TestCopyObject tests copying objects through ARMOR
func TestCopyObject(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	srcKey := generateTestKey(t)
	dstKey := srcKey + "-copy"
	testData := generateTestData(10 * 1024)

	// Upload source
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(srcKey),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(srcKey),
	})

	// Copy
	_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(fmt.Sprintf("%s/%s", bucket, srcKey)),
	})
	if err != nil {
		t.Fatalf("CopyObject failed: %v", err)
	}
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(dstKey),
	})

	// Download copy and verify
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(dstKey),
	})
	if err != nil {
		t.Fatalf("GetObject for copy failed: %v", err)
	}
	defer resp.Body.Close()

	copied, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read copied object: %v", err)
	}

	if !bytes.Equal(testData, copied) {
		t.Error("Copied object content doesn't match original")
	}

	t.Logf("CopyObject verified - content matches")
}

// TestMultipartUpload tests multipart upload through ARMOR
func TestMultipartUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create multipart upload
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
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

	// Upload parts (3 parts of 5 MB each = 15 MB total)
	partSize := int64(5 * 1024 * 1024)
	parts := []types.CompletedPart{}
	for i := int32(1); i <= 3; i++ {
		partData := generateTestData(int(partSize))
		partData[0] = byte(i) // Mark each part uniquely

		uploadResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(key),
			UploadId:   uploadID,
			PartNumber: &i,
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			cleanup()
			t.Fatalf("UploadPart %d failed: %v", i, err)
		}
		t.Logf("Uploaded part %d, ETag: %s", i, *uploadResp.ETag)

		parts = append(parts, types.CompletedPart{
			ETag:       uploadResp.ETag,
			PartNumber: &i,
		})
	}

	// Complete multipart upload
	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
	})
	if err != nil {
		cleanup()
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("Completed multipart upload")

	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	// Verify download
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer getResp.Body.Close()

	// Verify size
	expectedSize := partSize * 3
	if getResp.ContentLength != nil && *getResp.ContentLength != expectedSize {
		t.Errorf("ContentLength mismatch: got %d, want %d", *getResp.ContentLength, expectedSize)
	}

	t.Logf("Multipart upload verified, size: %d", *getResp.ContentLength)
}

// TestLargeFile tests uploading a file larger than the streaming threshold
func TestLargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Create 20 MB of test data (above streaming threshold of 10 MB)
	size := 20 * 1024 * 1024
	testData := generateTestData(size)
	// Add unique pattern for verification
	copy(testData[0:8], []byte("ARMOR_TEST"))

	// Upload
	t.Logf("Uploading %d bytes (above streaming threshold)", size)
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

	// Download and verify
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer resp.Body.Close()

	downloaded, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if !bytes.Equal(testData, downloaded) {
		t.Errorf("Downloaded data doesn't match: got %d bytes, want %d bytes",
			len(downloaded), len(testData))
	}

	// Test range read on large file
	rangeResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String("bytes=10485760-10486847"), // 1 KB from middle
	})
	if err != nil {
		t.Fatalf("Range GetObject failed: %v", err)
	}
	defer rangeResp.Body.Close()

	rangeData, err := io.ReadAll(rangeResp.Body)
	if err != nil {
		t.Fatalf("Failed to read range response: %v", err)
	}

	expectedRange := testData[10485760:10486848]
	if !bytes.Equal(expectedRange, rangeData) {
		t.Errorf("Range read mismatch: got %d bytes, want %d bytes",
			len(rangeData), len(expectedRange))
	}

	t.Logf("Large file test passed, verified full download and range read")
}

// TestConditionalRequests tests ETag-based conditional requests
func TestConditionalRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)
	testData := generateTestData(1024)

	// Upload
	putResp, err := client.PutObject(ctx, &s3.PutObjectInput{
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

	etag := putResp.ETag
	t.Logf("Uploaded object with ETag: %s", *etag)

	// Test If-Match with correct ETag
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(key),
		IfMatch: etag,
	})
	if err != nil {
		t.Errorf("GetObject with matching If-Match failed: %v", err)
	}

	// Test If-Match with wrong ETag (should fail)
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(key),
		IfMatch: aws.String("\"wrong-etag\""),
	})
	if err == nil {
		t.Error("GetObject with wrong If-Match should have failed")
	} else {
		t.Logf("If-Match with wrong ETag correctly failed: %v", err)
	}

	// Test If-None-Match with wrong ETag (should succeed)
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		IfNoneMatch: aws.String("\"wrong-etag\""),
	})
	if err != nil {
		t.Errorf("GetObject with wrong If-None-Match failed: %v", err)
	}

	t.Logf("Conditional request tests passed")
}

// TestPresignedURL tests the pre-signed URL functionality
func TestPresignedURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}
	adminEndpoint := os.Getenv("ARMOR_ADMIN_ENDPOINT")
	if adminEndpoint == "" {
		adminEndpoint = "http://localhost:9001"
	}

	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)
	testData := generateTestData(1024)

	// Upload
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

	// Request pre-signed URL from admin endpoint
	presignReq := fmt.Sprintf(`{"bucket":"%s","key":"%s","expires_in":300}`, bucket, key)
	resp, err := http.Post(adminEndpoint+"/admin/presign", "application/json",
		strings.NewReader(presignReq))
	if err != nil {
		t.Skipf("Presign endpoint not available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Skipf("Presign endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response to get share URL
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read presign response: %v", err)
	}

	// Response should contain a URL
	shareURL := strings.TrimSpace(string(body))
	shareURL = strings.Trim(shareURL, "\"")

	t.Logf("Got pre-signed URL: %s", shareURL)

	// Download via share URL
	shareResp, err := http.Get(shareURL)
	if err != nil {
		t.Fatalf("Failed to fetch from share URL: %v", err)
	}
	defer shareResp.Body.Close()

	if shareResp.StatusCode != http.StatusOK {
		t.Fatalf("Share URL returned %d", shareResp.StatusCode)
	}

	downloaded, err := io.ReadAll(shareResp.Body)
	if err != nil {
		t.Fatalf("Failed to read share response: %v", err)
	}

	if !bytes.Equal(testData, downloaded) {
		t.Errorf("Downloaded data from share URL doesn't match")
	}

	t.Logf("Pre-signed URL test passed")
}

// TestHealthEndpoints tests the health check endpoints
func TestHealthEndpoints(t *testing.T) {
	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	// Test /healthz
	resp, err := http.Get(armorEndpoint + "/healthz")
	if err != nil {
		t.Skipf("ARMOR not running at %s: %v", armorEndpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("/healthz returned %d, want 200", resp.StatusCode)
	}

	// Test /readyz
	resp, err = http.Get(armorEndpoint + "/readyz")
	if err != nil {
		t.Fatalf("/readyz request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("/readyz returned %d, want 200", resp.StatusCode)
	}

	t.Logf("Health endpoints verified")
}

// TestCanaryEndpoint tests the canary integrity endpoint
func TestCanaryEndpoint(t *testing.T) {
	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	resp, err := http.Get(armorEndpoint + "/armor/canary")
	if err != nil {
		t.Skipf("ARMOR not running at %s: %v", armorEndpoint, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("/armor/canary returned %d: %s", resp.StatusCode, string(body))
	}

	t.Logf("Canary response: %s", string(body))
}

// TestDirectB2Download verifies that downloading directly from B2 returns ciphertext
// This confirms the encryption is actually happening
func TestDirectB2Download(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)
	testData := generateTestData(1024)
	// Add recognizable pattern
	copy(testData[0:4], []byte("TEST"))

	// Upload through ARMOR
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

	// Download directly from Cloudflare (bypassing ARMOR)
	// This should return ciphertext, not plaintext
	cfURL := fmt.Sprintf("https://%s/file/%s/%s", cfDomain, bucket, key)
	t.Logf("Downloading directly from Cloudflare: %s", cfURL)

	resp, err := http.Get(cfURL)
	if err != nil {
		t.Skipf("Direct Cloudflare download failed: %v", err)
	}
	defer resp.Body.Close()

	ciphertext, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read direct download: %v", err)
	}

	// Verify we got ciphertext (should NOT start with "TEST")
	if bytes.HasPrefix(ciphertext, []byte("TEST")) {
		t.Error("Direct download returned plaintext - encryption not working!")
	}

	// Verify ciphertext is different from plaintext
	if bytes.Equal(testData, ciphertext) {
		t.Error("Ciphertext equals plaintext - encryption not working!")
	}

	// Verify ciphertext starts with ARMOR magic
	if len(ciphertext) >= 4 && string(ciphertext[0:4]) != "ARMR" {
		t.Logf("Warning: ciphertext doesn't start with ARMR magic, got: %q", ciphertext[0:4])
	}

	t.Logf("Direct B2/Cloudflare download confirmed encrypted (size: %d, expected: %d)",
		len(ciphertext), len(testData))
}
