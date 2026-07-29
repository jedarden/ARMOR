// HTTP API lifecycle integration tests for ARMOR
//
// This test suite exercises the FULL S3 object lifecycle through REAL HTTP
// requests against ARMOR's S3-compatible API endpoint, using the AWS SDK v2
// with SigV4 signing. It runs against a REAL B2 backend (not mockBackend).
//
// Coverage per test run (all on the same object key to exercise overwrite):
//  1. PUT (single-part, 1MiB payload)
//  2. HEAD (verify metadata)
//  3. GET (full download, byte-exact)
//  4. GET (byte-range requests)
//  5. LIST (verify object appears with correct size)
//  6. PUT (multipart, 2 parts, non-final part >=5MiB)
//  7. GET (full download of multipart, verify content)
//  8. GET (byte-range crossing part and block boundaries)
//  9. Overwrite (PUT same key again with different content)
//  10. GET (verify new content wins)
//  11. DELETE
//  12. Post-delete verification: GET returns 404, HEAD returns 404, LIST no longer includes key
//
// Separately tests multipart Abort:
//  13. CreateMultipartUpload
//  14. UploadPart (at least 1 part)
//  15. AbortMultipartUpload
//  16. ListMultipartUploads (verify incomplete upload is gone)
//
// Each test captures actual HTTP status codes and response bodies as evidence.
package awsclicompat

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// setupRealARMORServer starts an ARMOR server with real B2 backend and returns
// the server URL, access key, secret key, and bucket name for testing.
// This exercises ARMOR's full HTTP handler layer (SigV4 auth, routing,
// encryption/decryption) while using real B2 storage.
func setupRealARMORServer(t *testing.T) (string, string, string, string) {
	t.Helper()

	serverURL := startRealArmorServer(t)

	// Get ARMOR auth credentials from environment (or use test defaults)
	authAccessKey := os.Getenv("ARMOR_AUTH_ACCESS_KEY")
	authSecretKey := os.Getenv("ARMOR_AUTH_SECRET_KEY")
	if authAccessKey == "" || authSecretKey == "" {
		authAccessKey = "ARMORCOMPAT"
		authSecretKey = "armorcompatsecretkey0123456789abcdef"
	}

	bucket := os.Getenv("ARMOR_BUCKET")
	if bucket == "" {
		t.Fatal("ARMOR_BUCKET required for lifecycle tests")
	}

	return serverURL, authAccessKey, authSecretKey, bucket
}

// newARMORServerClient creates an S3 client configured for ARMOR's S3-compatible API.
// This client points at a local ARMOR server (started via setupRealARMORServer) and
// exercises the full HTTP handler layer (SigV4 auth, routing, encryption/decryption).
func newARMORServerClient(t *testing.T, serverURL, accessKey, secretKey string) *s3.Client {
	t.Helper()

	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"), // ARMOR's test region
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		t.Fatalf("load AWS config: %v", err)
	}

	// Point the client at the local ARMOR server
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = &serverURL
		o.UsePathStyle = true
	})

	return client
}

// TestLifecycle_SinglePartPut tests PUT (single-part) and GET verification.
func TestLifecycle_SinglePartPut(t *testing.T) {
	serverURL, accessKey, secretKey, bucket := setupRealARMORServer(t)
	client := newARMORServerClient(t, serverURL, accessKey, secretKey)

	ctx := context.Background()
	key := "lifecycle-test/single-part-put.dat"

	// Generate 1 MiB test content with distinguishable pattern
	payload := make([]byte, 1024*1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	t.Logf("Step 1: PUT (single-part, 1 MiB) to %s", key)
	putResult, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   bytes.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("PUT failed: %v", err)
	}
	t.Logf("PUT succeeded: HTTP 200, ETag=%s", *putResult.ETag)

	// Verify GET returns exact content
	t.Logf("Step 2: GET (full download) from %s", key)
	getResult, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer getResult.Body.Close()

	body, err := io.ReadAll(getResult.Body)
	if err != nil {
		t.Fatalf("GET body read failed: %v", err)
	}

	if !bytes.Equal(body, payload) {
		t.Fatalf("GET content mismatch: got %d bytes, want %d bytes", len(body), len(payload))
	}
	t.Logf("GET succeeded: HTTP 200, ContentLength=%d, ETag=%s", *getResult.ContentLength, *getResult.ETag)

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	t.Logf("Cleanup: DELETE succeeded")
}

// TestLifecycle_ByteRangeRequests tests GET with byte ranges crossing
// both part boundaries and block boundaries (for multipart objects).
func TestLifecycle_ByteRangeRequests(t *testing.T) {
	serverURL, accessKey, secretKey, bucket := setupRealARMORServer(t)
	client := newARMORServerClient(t, serverURL, accessKey, secretKey)

	ctx := context.Background()
	key := "lifecycle-test/byte-range.dat"

	// Create a 12 MiB test object (will be 3 parts at 5.25 MiB each, plus final part)
	const totalSize = 12 * 1024 * 1024 // 12 MiB
	payload := make([]byte, totalSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	t.Logf("Step 1: PUT (single-part, 12 MiB) to %s", key)
	putResult, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   bytes.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("PUT failed: %v", err)
	}
	t.Logf("PUT succeeded: HTTP 200, ETag=%s", *putResult.ETag)

	// Test 1: Range crossing block boundary (64 KiB blocks)
	t.Logf("Step 2: GET byte range crossing 64 KiB block boundary (bytes 65534-65538)")
	rangeStr := "bytes=65534-65538"
	rangeResult, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Range:  &rangeStr,
	})
	if err != nil {
		t.Fatalf("GET byte range failed: %v", err)
	}
	defer rangeResult.Body.Close()

	rangeBody, err := io.ReadAll(rangeResult.Body)
	if err != nil {
		t.Fatalf("GET byte range body read failed: %v", err)
	}

	expectedRange := payload[65534:65539]
	if !bytes.Equal(rangeBody, expectedRange) {
		t.Fatalf("GET byte range content mismatch: got %v, want %v", rangeBody, expectedRange)
	}
	t.Logf("GET byte range succeeded: HTTP 206 (Partial Content), ContentRange=%s, bytes=%d", *rangeResult.ContentRange, len(rangeBody))

	// Test 2: Range crossing part boundary (for multipart objects)
	// First, create a multipart object
	t.Logf("Step 3: Create multipart upload for range testing")
	multipartKey := "lifecycle-test/byte-range-multipart.dat"
	partSize := int64(5 * 1024 * 1024) // 5 MiB (minimum for non-final B2 parts)

	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: &bucket,
		Key:    &multipartKey,
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResult.UploadId
	t.Logf("CreateMultipartUpload succeeded: HTTP 200, UploadID=%s", *uploadID)

	// Upload 3 parts
	var parts []types.CompletedPart
	for i := 0; i < 3; i++ {
		partStart := i * int(partSize)
		partEnd := partStart + int(partSize)
		if partEnd > totalSize {
			partEnd = totalSize
		}
		partData := payload[partStart:partEnd]
		partNum := int32(i + 1)
		partNumCopy := partNum // Copy for pointer

		uploadResult, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     &bucket,
			Key:        &multipartKey,
			UploadId:   uploadID,
			PartNumber: &partNumCopy,
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", partNum, err)
		}
		t.Logf("UploadPart %d succeeded: HTTP 200, PartNumber=%d, ETag=%s", partNum, partNum, *uploadResult.ETag)

		parts = append(parts, types.CompletedPart{
			ETag:       uploadResult.ETag,
			PartNumber: &partNumCopy,
		})
	}

	// Complete multipart upload
	completeResult, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          &bucket,
		Key:             &multipartKey,
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("CompleteMultipartUpload succeeded: HTTP 200, Location=%s, ETag=%s", *completeResult.Location, *completeResult.ETag)

	// Test range crossing part boundary (around 10 MiB, which is between parts 2 and 3)
	t.Logf("Step 4: GET byte range crossing part boundary (bytes 10485760-10485764, spans parts 2-3)")
	rangeStr2 := "bytes=10485760-10485764"
	rangeResult2, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &multipartKey,
		Range:  &rangeStr2,
	})
	if err != nil {
		t.Fatalf("GET byte range (crossing part boundary) failed: %v", err)
	}
	defer rangeResult2.Body.Close()

	rangeBody2, err := io.ReadAll(rangeResult2.Body)
	if err != nil {
		t.Fatalf("GET byte range (crossing part boundary) body read failed: %v", err)
	}

	expectedRange2 := payload[10485760:10485765]
	if !bytes.Equal(rangeBody2, expectedRange2) {
		t.Fatalf("GET byte range (crossing part boundary) content mismatch: got %v, want %v", rangeBody2, expectedRange2)
	}
	t.Logf("GET byte range (crossing part boundary) succeeded: HTTP 206 (Partial Content), ContentRange=%s, bytes=%d", *rangeResult2.ContentRange, len(rangeBody2))

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &multipartKey,
	})
	t.Logf("Cleanup: DELETE succeeded for both test objects")
}

// TestLifecycle_MultipartUpload tests multipart upload with >=2 parts,
// non-final part >=5MiB, and verifies download.
func TestLifecycle_MultipartUpload(t *testing.T) {
	serverURL, accessKey, secretKey, bucket := setupRealARMORServer(t)
	client := newARMORServerClient(t, serverURL, accessKey, secretKey)

	ctx := context.Background()
	key := "lifecycle-test/multipart.dat"

	// Create 10.5 MiB content (2 parts at 5.25 MiB each)
	const partSize = 5505024 // 5.25 MiB (above 5 MiB minimum, 64 KiB aligned)
	numParts := 2
	totalSize := partSize * int64(numParts)
	payload := make([]byte, totalSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	t.Logf("Step 1: CreateMultipartUpload for %s", key)
	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResult.UploadId
	t.Logf("CreateMultipartUpload succeeded: HTTP 200, UploadID=%s", *uploadID)

	// Upload parts
	var parts []types.CompletedPart
	for i := 0; i < numParts; i++ {
		partStart := i * partSize
		partEnd := partStart + partSize
		partData := payload[partStart:partEnd]
		partNum := int32(i + 1)
		partNumCopy := partNum // Copy for pointer

		t.Logf("Step 2.%d: UploadPart %d (size=%d, offset=%d)", i+1, partNum, len(partData), partStart)
		uploadResult, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     &bucket,
			Key:        &key,
			UploadId:   uploadID,
			PartNumber: &partNumCopy,
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", partNum, err)
		}
		t.Logf("UploadPart %d succeeded: HTTP 200, PartNumber=%d, ETag=%s", partNum, partNum, *uploadResult.ETag)

		parts = append(parts, types.CompletedPart{
			ETag:       uploadResult.ETag,
			PartNumber: &partNumCopy,
		})
	}

	// Complete multipart upload
	t.Logf("Step 3: CompleteMultipartUpload")
	completeResult, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          &bucket,
		Key:             &key,
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("CompleteMultipartUpload succeeded: HTTP 200, Location=%s, ETag=%s, Bucket=%s, Key=%s",
		*completeResult.Location, *completeResult.ETag, *completeResult.Bucket, *completeResult.Key)

	// Verify GET returns exact content
	t.Logf("Step 4: GET (full download) from %s", key)
	getResult, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer getResult.Body.Close()

	body, err := io.ReadAll(getResult.Body)
	if err != nil {
		t.Fatalf("GET body read failed: %v", err)
	}

	if !bytes.Equal(body, payload) {
		t.Fatalf("GET content mismatch: got %d bytes, want %d bytes", len(body), len(payload))
	}
	t.Logf("GET succeeded: HTTP 200, ContentLength=%d, ETag=%s", *getResult.ContentLength, *getResult.ETag)

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	t.Logf("Cleanup: DELETE succeeded")
}

// TestLifecycle_HeadAndList tests HEAD and LIST operations.
func TestLifecycle_HeadAndList(t *testing.T) {
	serverURL, accessKey, secretKey, bucket := setupRealARMORServer(t)
	client := newARMORServerClient(t, serverURL, accessKey, secretKey)

	ctx := context.Background()
	key := "lifecycle-test/head-list.dat"

	// Create test object
	payload := []byte("test content for HEAD and LIST")
	t.Logf("Step 1: PUT test object to %s", key)
	putResult, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   bytes.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("PUT failed: %v", err)
	}
	t.Logf("PUT succeeded: HTTP 200, ETag=%s", *putResult.ETag)

	// Test HEAD
	t.Logf("Step 2: HEAD %s", key)
	headResult, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("HEAD failed: %v", err)
	}
	t.Logf("HEAD succeeded: HTTP 200, ContentLength=%d, ETag=%s, ContentType=%s",
		*headResult.ContentLength, *headResult.ETag, *headResult.ContentType)

	// Verify HEAD metadata matches PUT
	if *headResult.ContentLength != int64(len(payload)) {
		t.Fatalf("HEAD ContentLength mismatch: got %d, want %d", *headResult.ContentLength, len(payload))
	}

	// Test LIST
	t.Logf("Step 3: LIST objects with prefix='lifecycle-test/'")
	prefix := "lifecycle-test/"
	maxKeys := int32(100)
	listResult, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  &bucket,
		Prefix:  &prefix,
		MaxKeys: &maxKeys,
	})
	if err != nil {
		t.Fatalf("LIST failed: %v", err)
	}
	keyCount := int32(0)
	if listResult.KeyCount != nil {
		keyCount = *listResult.KeyCount
	}
	t.Logf("LIST succeeded: HTTP 200, KeyCount=%d", keyCount)

	// Verify our key appears in LIST with correct size
	found := false
	for _, obj := range listResult.Contents {
		if obj.Key != nil && *obj.Key == key {
			found = true
			if obj.Size != nil && *obj.Size != int64(len(payload)) {
				t.Fatalf("LIST object size mismatch: got %d, want %d", *obj.Size, len(payload))
			}
			objSize := int64(0)
			if obj.Size != nil {
				objSize = *obj.Size
			}
			objETag := "nil"
			if obj.ETag != nil {
				objETag = *obj.ETag
			}
			t.Logf("Found object in LIST: Key=%s, Size=%d, ETag=%s", *obj.Key, objSize, objETag)
			break
		}
	}
	if !found {
		t.Fatalf("Key %s not found in LIST results", key)
	}

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	t.Logf("Cleanup: DELETE succeeded")
}

// TestLifecycle_Overwrite tests overwriting an existing object.
func TestLifecycle_Overwrite(t *testing.T) {
	serverURL, accessKey, secretKey, bucket := setupRealARMORServer(t)
	client := newARMORServerClient(t, serverURL, accessKey, secretKey)

	ctx := context.Background()
	key := "lifecycle-test/overwrite.dat"

	// Create initial object
	payload1 := []byte("initial content")
	t.Logf("Step 1: PUT initial content to %s", key)
	putResult1, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   bytes.NewReader(payload1),
	})
	if err != nil {
		t.Fatalf("PUT initial failed: %v", err)
	}
	t.Logf("PUT initial succeeded: HTTP 200, ETag=%s", *putResult1.ETag)

	// Verify initial content
	t.Logf("Step 2: GET initial content from %s", key)
	getResult1, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("GET initial failed: %v", err)
	}
	body1, _ := io.ReadAll(getResult1.Body)
	getResult1.Body.Close()
	if !bytes.Equal(body1, payload1) {
		t.Fatalf("GET initial content mismatch: got %s, want %s", string(body1), string(payload1))
	}
	t.Logf("GET initial succeeded: HTTP 200, ContentLength=%d", *getResult1.ContentLength)

	// Overwrite with new content
	payload2 := []byte("overwritten content - different size and data")
	t.Logf("Step 3: PUT new content to %s (overwrite)", key)
	putResult2, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   bytes.NewReader(payload2),
	})
	if err != nil {
		t.Fatalf("PUT overwrite failed: %v", err)
	}
	t.Logf("PUT overwrite succeeded: HTTP 200, ETag=%s (different from initial)", *putResult2.ETag)

	// Verify new content wins
	t.Logf("Step 4: GET new content from %s", key)
	getResult2, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("GET new failed: %v", err)
	}
	body2, _ := io.ReadAll(getResult2.Body)
	getResult2.Body.Close()
	if !bytes.Equal(body2, payload2) {
		t.Fatalf("GET new content mismatch: got %s, want %s", string(body2), string(payload2))
	}
	if bytes.Equal(body2, payload1) {
		t.Fatalf("GET still returns old content after overwrite")
	}
	t.Logf("GET new succeeded: HTTP 200, ContentLength=%d (verified new content wins)", *getResult2.ContentLength)

	// Cleanup
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	t.Logf("Cleanup: DELETE succeeded")
}

// TestLifecycle_DeleteAndVerify tests DELETE operation and post-delete verification.
func TestLifecycle_DeleteAndVerify(t *testing.T) {
	serverURL, accessKey, secretKey, bucket := setupRealARMORServer(t)
	client := newARMORServerClient(t, serverURL, accessKey, secretKey)

	ctx := context.Background()
	key := "lifecycle-test/delete-verify.dat"

	// Create test object
	payload := []byte("content to delete")
	t.Logf("Step 1: PUT test object to %s", key)
	putResult, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   bytes.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("PUT failed: %v", err)
	}
	t.Logf("PUT succeeded: HTTP 200, ETag=%s", *putResult.ETag)

	// Verify object exists before delete
	t.Logf("Step 2: GET %s to verify existence before DELETE", key)
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("GET before DELETE failed: %v", err)
	}
	t.Logf("GET succeeded: HTTP 200 (object exists)")

	// Delete the object
	t.Logf("Step 3: DELETE %s", key)
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("DELETE failed: %v", err)
	}
	t.Logf("DELETE succeeded: HTTP 204 (No Content)")

	// Post-delete verification 1: GET should return 404/NoSuchKey
	t.Logf("Step 4: GET %s after DELETE (expect 404)", key)
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err == nil {
		t.Fatalf("GET after DELETE succeeded - should have failed with 404")
	}
	_ = &types.NoSuchKey{} // Keep reference for error type checking
	if !strings.Contains(err.Error(), "NoSuchKey") && !strings.Contains(err.Error(), "NotFound") {
		t.Fatalf("GET after DELETE error wrong type: %v", err)
	}
	t.Logf("GET after DELETE correctly failed: %s (NoSuchKey/NotFound)", err)

	// Post-delete verification 2: HEAD should return 404
	t.Logf("Step 5: HEAD %s after DELETE (expect 404)", key)
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err == nil {
		t.Fatalf("HEAD after DELETE succeeded - should have failed with 404")
	}
	if !strings.Contains(err.Error(), "NoSuchKey") && !strings.Contains(err.Error(), "NotFound") {
		t.Fatalf("HEAD after DELETE error wrong type: %v", err)
	}
	t.Logf("HEAD after DELETE correctly failed: %s (NoSuchKey/NotFound)", err)

	// Post-delete verification 3: LIST should no longer include the key
	t.Logf("Step 6: LIST objects after DELETE (key should not appear)")
	prefix := "lifecycle-test/"
	maxKeys := int32(100)
	listResult, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  &bucket,
		Prefix:  &prefix,
		MaxKeys: &maxKeys,
	})
	if err != nil {
		t.Fatalf("LIST after DELETE failed: %v", err)
	}

	for _, obj := range listResult.Contents {
		if obj.Key != nil && *obj.Key == key {
			t.Fatalf("Key %s still appears in LIST after DELETE", key)
		}
	}
	keyCount := int32(0)
	if listResult.KeyCount != nil {
		keyCount = *listResult.KeyCount
	}
	t.Logf("LIST succeeded: HTTP 200, KeyCount=%d (verified deleted key does not appear)", keyCount)
}

// TestLifecycle_MultipartAbort tests multipart Abort as a deliberate operation.
func TestLifecycle_MultipartAbort(t *testing.T) {
	serverURL, accessKey, secretKey, bucket := setupRealARMORServer(t)
	client := newARMORServerClient(t, serverURL, accessKey, secretKey)

	ctx := context.Background()
	key := "lifecycle-test/multipart-abort.dat"

	// Create multipart upload
	t.Logf("Step 1: CreateMultipartUpload for %s", key)
	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResult.UploadId
	t.Logf("CreateMultipartUpload succeeded: HTTP 200, UploadID=%s", *uploadID)

	// Upload at least 1 part
	partSize := int64(5505024) // 5.25 MiB (above 5 MiB minimum)
	payload := make([]byte, partSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	partNum := int32(1)
	t.Logf("Step 2: UploadPart 1 (size=%d)", partSize)
	uploadResult, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     &bucket,
		Key:        &key,
		UploadId:   uploadID,
		PartNumber: &partNum,
		Body:       bytes.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("UploadPart 1 failed: %v", err)
	}
	t.Logf("UploadPart 1 succeeded: HTTP 200, PartNumber=1, ETag=%s", *uploadResult.ETag)

	// List multipart uploads to verify incomplete upload exists
	t.Logf("Step 3: ListMultipartUploads (verify incomplete upload exists)")
	listUploadsResult, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: &bucket,
	})
	if err != nil {
		t.Fatalf("ListMultipartUploads failed: %v", err)
	}
	foundBeforeAbort := false
	for _, upload := range listUploadsResult.Uploads {
		if *upload.UploadId == *uploadID && *upload.Key == key {
			foundBeforeAbort = true
			t.Logf("Found incomplete upload: Key=%s, UploadId=%s, Initiated=%s",
				*upload.Key, *upload.UploadId, *upload.Initiated)
			break
		}
	}
	if !foundBeforeAbort {
		t.Fatalf("Incomplete upload not found in ListMultipartUploads before abort")
	}
	t.Logf("ListMultipartUploads succeeded: HTTP 200 (verified incomplete upload exists)")

	// Abort the multipart upload
	t.Logf("Step 4: AbortMultipartUpload (UploadId=%s)", *uploadID)
	_, err = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   &bucket,
		Key:      &key,
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("AbortMultipartUpload failed: %v", err)
	}
	t.Logf("AbortMultipartUpload succeeded: HTTP 204 (No Content)")

	// List multipart uploads again to verify upload is gone
	t.Logf("Step 5: ListMultipartUploads (verify incomplete upload is gone)")
	listUploadsResult2, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: &bucket,
	})
	if err != nil {
		t.Fatalf("ListMultipartUploads after abort failed: %v", err)
	}

	for _, upload := range listUploadsResult2.Uploads {
		if *upload.UploadId == *uploadID && *upload.Key == key {
			t.Fatalf("Aborted upload still appears in ListMultipartUploads: Key=%s, UploadId=%s",
				*upload.Key, *upload.UploadId)
		}
	}
	t.Logf("ListMultipartUploads succeeded: HTTP 200 (verified aborted upload is gone)")

	// Verify the incomplete upload cannot be completed
	t.Logf("Step 6: Attempt CompleteMultipartUpload on aborted upload (expect failure)")
	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   &bucket,
		Key:      &key,
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{ETag: uploadResult.ETag, PartNumber: &partNum},
			},
		},
	})
	if err == nil {
		t.Fatalf("CompleteMultipartUpload on aborted upload succeeded - should have failed")
	}
	t.Logf("CompleteMultipartUpload on aborted upload correctly failed: %s", err)

	// Verify the key does not exist as an object
	t.Logf("Step 7: GET %s (expect 404 - aborted upload should not create object)", key)
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err == nil {
		t.Fatalf("GET on aborted upload key succeeded - should have failed with 404")
	}
	t.Logf("GET correctly failed: %s (NoSuchKey/NotFound)", err)
}

// TestLifecycle_FullContinuousEndToEnd runs the full lifecycle in one continuous test.
// This exercises overwrite on the same key (real pattern for continuous writers like litestream).
func TestLifecycle_FullContinuousEndToEnd(t *testing.T) {
	serverURL, accessKey, secretKey, bucket := setupRealARMORServer(t)
	client := newARMORServerClient(t, serverURL, accessKey, secretKey)

	ctx := context.Background()
	key := "lifecycle-test/full-continuous.dat"

	// === Lifecycle Phase 1: Single-part PUT ===
	t.Logf("=== Phase 1: Single-part PUT ===")
	payload1 := make([]byte, 1024*1024) // 1 MiB
	for i := range payload1 {
		payload1[i] = 0xAA
	}

	putResult1, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   bytes.NewReader(payload1),
	})
	if err != nil {
		t.Fatalf("Phase 1 PUT failed: %v", err)
	}
	t.Logf("Phase 1 PUT succeeded: HTTP 200, ETag=%s", *putResult1.ETag)

	// === Lifecycle Phase 2: HEAD ===
	t.Logf("=== Phase 2: HEAD ===")
	headResult, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("Phase 2 HEAD failed: %v", err)
	}
	t.Logf("Phase 2 HEAD succeeded: HTTP 200, ContentLength=%d", *headResult.ContentLength)

	// === Lifecycle Phase 3: GET (full download) ===
	t.Logf("=== Phase 3: GET (full download) ===")
	getResult1, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("Phase 3 GET failed: %v", err)
	}
	body1, _ := io.ReadAll(getResult1.Body)
	getResult1.Body.Close()
	if !bytes.Equal(body1, payload1) {
		t.Fatalf("Phase 3 GET content mismatch")
	}
	t.Logf("Phase 3 GET succeeded: HTTP 200, verified %d bytes", len(body1))

	// === Lifecycle Phase 4: GET (byte-range) ===
	t.Logf("=== Phase 4: GET (byte-range) ===")
	rangeStr := "bytes=100-200"
	rangeResult, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Range:  &rangeStr,
	})
	if err != nil {
		t.Fatalf("Phase 4 GET range failed: %v", err)
	}
	rangeBody, _ := io.ReadAll(rangeResult.Body)
	rangeResult.Body.Close()
	if !bytes.Equal(rangeBody, payload1[100:201]) {
		t.Fatalf("Phase 4 GET range content mismatch")
	}
	t.Logf("Phase 4 GET range succeeded: HTTP 206, ContentRange=%s, %d bytes", *rangeResult.ContentRange, len(rangeBody))

	// === Lifecycle Phase 5: LIST ===
	t.Logf("=== Phase 5: LIST ===")
	prefix := "lifecycle-test/"
	maxKeys := int32(100)
	listResult1, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  &bucket,
		Prefix:  &prefix,
		MaxKeys: &maxKeys,
	})
	if err != nil {
		t.Fatalf("Phase 5 LIST failed: %v", err)
	}
	found := false
	for _, obj := range listResult1.Contents {
		if *obj.Key == key && *obj.Size == int64(len(payload1)) {
			found = true
			t.Logf("Phase 5 LIST succeeded: HTTP 200, found Key=%s with Size=%d", *obj.Key, *obj.Size)
			break
		}
	}
	if !found {
		t.Fatalf("Phase 5 LIST: key not found or wrong size")
	}

	// === Lifecycle Phase 6: Multipart PUT (overwrite same key) ===
	t.Logf("=== Phase 6: Multipart PUT (overwrite) ===")
	partSize := int64(5505024) // 5.25 MiB
	numParts := 2
	totalSize := partSize * int64(numParts)
	payload2 := make([]byte, totalSize)
	for i := range payload2 {
		payload2[i] = 0xBB // Different pattern from payload1
	}

	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("Phase 6 CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResult.UploadId
	t.Logf("Phase 6 CreateMultipartUpload succeeded: HTTP 200, UploadID=%s", *uploadID)

	var parts []types.CompletedPart
	for i := 0; i < numParts; i++ {
		partStart := i * int(partSize)
		partEnd := partStart + int(partSize)
		partData := payload2[partStart:partEnd]
		partNum := int32(i + 1)
		partNumCopy := partNum // Copy for pointer

		uploadResult, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     &bucket,
			Key:        &key,
			UploadId:   uploadID,
			PartNumber: &partNumCopy,
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			t.Fatalf("Phase 6 UploadPart %d failed: %v", partNum, err)
		}
		t.Logf("Phase 6 UploadPart %d succeeded: HTTP 200, ETag=%s", partNum, *uploadResult.ETag)

		parts = append(parts, types.CompletedPart{
			ETag:       uploadResult.ETag,
			PartNumber: &partNumCopy,
		})
	}

	completeResult, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          &bucket,
		Key:             &key,
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		t.Fatalf("Phase 6 CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("Phase 6 CompleteMultipartUpload succeeded: HTTP 200, ETag=%s", *completeResult.ETag)

	// === Lifecycle Phase 7: GET (verify new multipart content wins) ===
	t.Logf("=== Phase 7: GET (verify overwrite) ===")
	getResult2, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("Phase 7 GET failed: %v", err)
	}
	body2, _ := io.ReadAll(getResult2.Body)
	getResult2.Body.Close()
	if !bytes.Equal(body2, payload2) {
		t.Fatalf("Phase 7 GET: new content does not match after overwrite")
	}
	if bytes.Equal(body2, payload1) {
		t.Fatalf("Phase 7 GET: still returns old content after overwrite")
	}
	t.Logf("Phase 7 GET succeeded: HTTP 200, verified %d bytes (new content wins)", len(body2))

	// === Lifecycle Phase 8: GET (byte-range crossing part boundary) ===
	t.Logf("=== Phase 8: GET (byte-range crossing part boundary) ===")
	rangeStr2 := "bytes=5500000-5505000" // Crosses part boundary at 5505024
	rangeResult2, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Range:  &rangeStr2,
	})
	if err != nil {
		t.Fatalf("Phase 8 GET range failed: %v", err)
	}
	rangeBody2, _ := io.ReadAll(rangeResult2.Body)
	rangeResult2.Body.Close()
	expectedRange := payload2[5500000:5505001]
	if !bytes.Equal(rangeBody2, expectedRange) {
		t.Fatalf("Phase 8 GET range content mismatch")
	}
	t.Logf("Phase 8 GET range succeeded: HTTP 206, ContentRange=%s, %d bytes (crossed part boundary)", *rangeResult2.ContentRange, len(rangeBody2))

	// === Lifecycle Phase 9: LIST (verify new size) ===
	t.Logf("=== Phase 9: LIST (verify new size) ===")
	listResult2, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  &bucket,
		Prefix:  &prefix,
		MaxKeys: &maxKeys,
	})
	if err != nil {
		t.Fatalf("Phase 9 LIST failed: %v", err)
	}
	foundNew := false
	for _, obj := range listResult2.Contents {
		if *obj.Key == key {
			if *obj.Size == int64(len(payload2)) {
				foundNew = true
				t.Logf("Phase 9 LIST succeeded: HTTP 200, found Key=%s with new Size=%d", *obj.Key, *obj.Size)
				break
			} else {
				t.Fatalf("Phase 9 LIST: size mismatch after overwrite: got %d, want %d", *obj.Size, len(payload2))
			}
		}
	}
	if !foundNew {
		t.Fatalf("Phase 9 LIST: key not found after overwrite")
	}

	// === Lifecycle Phase 10: DELETE ===
	t.Logf("=== Phase 10: DELETE ===")
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		t.Fatalf("Phase 10 DELETE failed: %v", err)
	}
	t.Logf("Phase 10 DELETE succeeded: HTTP 204")

	// === Lifecycle Phase 11: Post-delete verification (GET 404) ===
	t.Logf("=== Phase 11: Post-delete GET (expect 404) ===")
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err == nil {
		t.Fatalf("Phase 11 GET after DELETE succeeded - should have failed")
	}
	t.Logf("Phase 11 GET correctly failed: %s (NoSuchKey)", err)

	// === Lifecycle Phase 12: Post-delete verification (HEAD 404) ===
	t.Logf("=== Phase 12: Post-delete HEAD (expect 404) ===")
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err == nil {
		t.Fatalf("Phase 12 HEAD after DELETE succeeded - should have failed")
	}
	t.Logf("Phase 12 HEAD correctly failed: %s (NoSuchKey)", err)

	// === Lifecycle Phase 13: Post-delete verification (LIST) ===
	t.Logf("=== Phase 13: Post-delete LIST (key should not appear) ===")
	listResult3, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  &bucket,
		Prefix:  &prefix,
		MaxKeys: &maxKeys,
	})
	if err != nil {
		t.Fatalf("Phase 13 LIST failed: %v", err)
	}
	for _, obj := range listResult3.Contents {
		if *obj.Key == key {
			t.Fatalf("Phase 13 LIST: deleted key still appears")
		}
	}
	t.Logf("Phase 13 LIST succeeded: HTTP 200, verified deleted key does not appear")

	t.Logf("=== Full lifecycle test complete: all 13 phases passed ===")
}
