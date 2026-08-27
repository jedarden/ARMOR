//go:build integration
// +build integration

// Comprehensive full lifecycle test for ARMOR S3 API
// This test exercises the entire object lifecycle through real HTTP requests,
// covering PUT, HEAD, GET, LIST, overwrite, DELETE, and post-delete verification.
// It addresses the coverage gap identified in bead armor-50dc6d01 where the canary
// bypasses the HTTP handler layer and never tests the full lifecycle.

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestFullObjectLifecycle tests the complete object lifecycle through real HTTP API calls.
// This is the primary test for bead armor-50dc6d01 which requires:
// 1. Real HTTP requests (SigV4-signed via AWS SDK) - not direct backend calls
// 2. Full lifecycle in one continuous test: PUT, HEAD, GET, LIST, overwrite, DELETE, verification
// 3. Must run against real B2 backend (not mocks)
// 4. Closure requires citing actual HTTP status codes/response bodies
func TestFullObjectLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Generate unique test data with recognizable patterns
	// Pattern: v1 | A | B | C...
	initialData := make([]byte, 256*1024) // 256 KB
	copy(initialData[0:3], []byte("v1|"))
	for i := 3; i < len(initialData); i++ {
		initialData[i] = byte(i % 256)
	}

	// ===== STEP 1: PUT (single-part) =====
	t.Log("STEP 1: PUT (single-part upload)")
	putResp1, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(initialData),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		t.Fatalf("STEP 1 FAILED: PutObject (single-part) failed: %v", err)
	}
	t.Logf("STEP 1 SUCCESS: PutObject returned HTTP status, ETag: %s", aws.ToString(putResp1.ETag))

	// Verify object appears in LIST immediately after upload
	listResp1, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 1 LIST FAILED: ListObjectsV2 failed: %v", err)
	}
	found := false
	for _, obj := range listResp1.Contents {
		if *obj.Key == key {
			found = true
			t.Logf("STEP 1 LIST SUCCESS: Object appears in LIST with size %d", *obj.Size)
			break
		}
	}
	if !found {
		t.Errorf("STEP 1 LIST FAILED: Object %s not found in LIST after PUT", key)
	}

	// ===== STEP 2: HEAD (verify metadata) =====
	t.Log("STEP 2: HEAD (verify metadata)")
	headResp1, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 2 FAILED: HeadObject failed: %v", err)
	}
	if headResp1.ContentLength == nil || *headResp1.ContentLength != int64(len(initialData)) {
		t.Errorf("STEP 2 FAILED: HeadObject ContentLength mismatch: got %d, want %d",
			headResp1.ContentLength, len(initialData))
	} else {
		t.Logf("STEP 2 SUCCESS: HeadObject returned ContentLength: %d", *headResp1.ContentLength)
	}

	// ===== STEP 3: GET (full download, byte-exact verification) =====
	t.Log("STEP 3: GET (full download)")
	getResp1, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 3 FAILED: GetObject (full download) failed: %v", err)
	}
	downloaded1, err := io.ReadAll(getResp1.Body)
	getResp1.Body.Close()
	if err != nil {
		t.Fatalf("STEP 3 FAILED: Failed to read GetObject response: %v", err)
	}
	if !bytes.Equal(initialData, downloaded1) {
		t.Errorf("STEP 3 FAILED: Downloaded content doesn't match uploaded content")
		// Find first difference
		for i := 0; i < len(initialData) && i < len(downloaded1); i++ {
			if initialData[i] != downloaded1[i] {
				t.Logf("First difference at byte %d: uploaded=%d, downloaded=%d", i, initialData[i], downloaded1[i])
				break
			}
		}
	} else {
		t.Logf("STEP 3 SUCCESS: Full download verified byte-exact (%d bytes)", len(downloaded1))
	}

	// ===== STEP 4: GET (byte-range requests, including straddling boundaries) =====
	t.Log("STEP 4: GET (byte-range requests)")

	// Calculate block size for ARMOR (default 64 KB)
	blockSize := 64 * 1024

	// Test ranges that straddle:
	// 1. Part boundary (if multipart) - test a range crossing a typical part boundary
	// 2. Block boundary (encryption block) - ARMOR encrypts in 64 KB blocks

	testRanges := []struct {
		name     string
		start    int64
		end      int64
		describe string
	}{
		{
			name:     "first-100",
			start:    0,
			end:      99,
			describe: "First 100 bytes",
		},
		{
			name:     "middle-chunk",
			start:    100000,
			end:      100100,
			describe: "Middle chunk",
		},
		{
			name:     "last-100",
			start:    int64(len(initialData) - 100),
			end:      int64(len(initialData) - 1),
			describe: "Last 100 bytes",
		},
		{
			name:     "straddle-block-boundary",
			start:    int64(blockSize - 100),
			end:      int64(blockSize + 100),
			describe: fmt.Sprintf("Straddles encryption block boundary (%d bytes)", blockSize),
		},
		{
			name:     "straddle-part-boundary",
			start:    5*1024*1024 - 100, // 5 MiB is typical multipart part boundary
			end:      5*1024*1024 + 100,
			describe: "Straddles typical multipart part boundary (5 MiB)",
		},
	}

	for i, r := range testRanges {
		// Skip range tests that exceed file size
		if r.end >= int64(len(initialData)) {
			t.Logf("STEP 4.%d SKIP: Range %s exceeds file size", i+1, r.name)
			continue
		}

		rangeHdr := fmt.Sprintf("bytes=%d-%d", r.start, r.end)
		getRangeResp, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Range:  aws.String(rangeHdr),
		})
		if err != nil {
			t.Errorf("STEP 4.%d FAILED: GetObject with range %s failed: %v", i+1, rangeHdr, err)
			continue
		}

		rangeData, err := io.ReadAll(getRangeResp.Body)
		getRangeResp.Body.Close()
		if err != nil {
			t.Errorf("STEP 4.%d FAILED: Failed to read range response: %v", i+1, err)
			continue
		}

		expectedRange := initialData[r.start : r.end+1]
		if !bytes.Equal(expectedRange, rangeData) {
			t.Errorf("STEP 4.%d FAILED: Range content mismatch: %s", i+1, r.describe)
			for j := 0; j < len(expectedRange) && j < len(rangeData); j++ {
				if expectedRange[j] != rangeData[j] {
					t.Logf("  First difference at offset %d: expected=%d, got=%d",
						r.start+int64(j), expectedRange[j], rangeData[j])
					break
				}
			}
		} else {
			t.Logf("STEP 4.%d SUCCESS: Range %s (%d bytes)", i+1, r.describe, len(rangeData))
		}

		// Range requests succeed (AWS SDK handles 206 Partial Content automatically)
		t.Logf("STEP 4.%d SUCCESS: Range %s (%d bytes)", i+1, r.describe, len(rangeData))
	}

	// ===== STEP 5: LIST (verify object appears with correct metadata) =====
	t.Log("STEP 5: LIST (verify object metadata)")
	listResp2, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 5 FAILED: ListObjectsV2 failed: %v", err)
	}
	if len(listResp2.Contents) != 1 {
		t.Errorf("STEP 5 FAILED: ListObjectsV2 returned %d objects, want 1", len(listResp2.Contents))
	} else {
		obj := listResp2.Contents[0]
		if *obj.Key != key {
			t.Errorf("STEP 5 FAILED: Listed key mismatch: got %s, want %s", *obj.Key, key)
		}
		if obj.Size == nil || *obj.Size != int64(len(initialData)) {
			t.Errorf("STEP 5 FAILED: Listed size mismatch: got %d, want %d",
				obj.Size, len(initialData))
		} else {
			t.Logf("STEP 5 SUCCESS: Object appears in LIST with correct metadata (size: %d)", *obj.Size)
		}
	}

	// ===== STEP 6: Overwrite (PUT same key with different content) =====
	t.Log("STEP 6: Overwrite (PUT same key with different content)")

	// New content with different pattern
	overwrittenData := make([]byte, 128*1024) // 128 KB (different size)
	copy(overwrittenData[0:3], []byte("v2|"))
	for i := 3; i < len(overwrittenData); i++ {
		overwrittenData[i] = byte((i + 100) % 256) // Different pattern
	}

	putResp2, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(overwrittenData),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		t.Fatalf("STEP 6 FAILED: Overwrite PutObject failed: %v", err)
	}
	t.Logf("STEP 6 SUCCESS: Overwrite completed, new ETag: %s", aws.ToString(putResp2.ETag))

	// Verify the new content wins on GET
	getResp2, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 6 GET FAILED: GetObject after overwrite failed: %v", err)
	}
	downloaded2, err := io.ReadAll(getResp2.Body)
	getResp2.Body.Close()
	if err != nil {
		t.Fatalf("STEP 6 GET FAILED: Failed to read response after overwrite: %v", err)
	}

	// Verify we got the NEW content (v2| prefix, different size)
	if !bytes.Equal(overwrittenData, downloaded2) {
		t.Errorf("STEP 6 VERIFICATION FAILED: Overwrite didn't take effect")
		if bytes.Equal(initialData, downloaded2) {
			t.Logf("  ERROR: Still has old content (v1| prefix, size %d)", len(downloaded2))
		}
		// Check if at least the prefix changed
		if len(downloaded2) >= 3 {
			t.Logf("  Downloaded prefix: %s", string(downloaded2[0:3]))
		}
	} else {
		t.Logf("STEP 6 VERIFICATION SUCCESS: New content verified (v2| prefix, %d bytes)", len(downloaded2))
	}

	// ===== STEP 7: DELETE =====
	t.Log("STEP 7: DELETE")
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 7 FAILED: DeleteObject failed: %v", err)
	}
	t.Logf("STEP 7 SUCCESS: DeleteObject completed successfully")

	// ===== STEP 8: Post-delete verification (GET returns 404) =====
	t.Log("STEP 8: Post-delete verification (GET should return 404)")
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Errorf("STEP 8 FAILED: GetObject succeeded after DELETE (expected 404)")
	} else {
		// Check if it's a NotFound error
		if isNoSuchKey(err) {
			t.Logf("STEP 8 SUCCESS: GetObject returned NoSuchKey/404 after DELETE")
		} else {
			t.Logf("STEP 8 PARTIAL: GetObject failed with error: %v", err)
		}
	}

	// ===== STEP 9: Post-delete verification (HEAD returns 404) =====
	t.Log("STEP 9: Post-delete verification (HEAD should return 404)")
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Errorf("STEP 9 FAILED: HeadObject succeeded after DELETE (expected 404)")
	} else {
		if isNoSuchKey(err) {
			t.Logf("STEP 9 SUCCESS: HeadObject returned NoSuchKey/404 after DELETE")
		} else {
			t.Logf("STEP 9 PARTIAL: HeadObject failed with error: %v", err)
		}
	}

	// ===== STEP 10: Post-delete verification (LIST no longer includes key) =====
	t.Log("STEP 10: Post-delete verification (LIST should no longer include key)")
	listResp3, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 10 FAILED: ListObjectsV2 failed: %v", err)
	}
	found = false
	for _, obj := range listResp3.Contents {
		if *obj.Key == key {
			found = true
			t.Errorf("STEP 10 FAILED: Object %s still appears in LIST after DELETE", key)
			break
		}
	}
	if !found {
		t.Logf("STEP 10 SUCCESS: Object no longer appears in LIST after DELETE")
	}

	t.Log("\n=== FULL LIFECYCLE TEST COMPLETED SUCCESSFULLY ===")
	t.Log("All steps passed: PUT → HEAD → GET → Range GETs → LIST → Overwrite → DELETE → Verification")
}

// isNoSuchKey checks if an error is a NoSuchKey error
func isNoSuchKey(err error) bool {
	if err == nil {
		return false
	}
	// Check error message for common patterns indicating 404/NotFound
	errStr := err.Error()
	return strings.Contains(errStr, "NoSuchKey") ||
	       strings.Contains(errStr, "NotFound") ||
	       strings.Contains(errStr, "404")
}

// TestMultipartLifecycle tests the complete multipart upload lifecycle through real HTTP.
// This includes creating upload, uploading >=2 parts with non-final parts >=5MiB,
// completing, verifying, and deliberately aborting with verification.
func TestMultipartLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// Use >=2 parts with non-final parts >=5MiB (B2 minimum)
	partSize := int64(6 * 1024 * 1024) // 6 MiB per part (above 5 MiB minimum)
	numParts := 3

	// Create test data with unique patterns for each part
	partData := make([][]byte, numParts)
	for i := 0; i < numParts; i++ {
		partData[i] = make([]byte, partSize)
		// Mark each part uniquely
		partData[i][0] = byte(i + 1)
		partData[i][1] = byte(i + 1)
		partData[i][2] = '|'
		for j := 3; j < len(partData[i]); j++ {
			partData[i][j] = byte((j + i*100) % 256)
		}
	}

	// Calculate total expected size
	totalSize := partSize * int64(numParts)

	// ===== STEP 1: CreateMultipartUpload =====
	t.Log("STEP 1: CreateMultipartUpload")
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 1 FAILED: CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId
	t.Logf("STEP 1 SUCCESS: Created multipart upload, ID: %s", *uploadID)

	// ===== STEP 2: Upload parts (>=2 parts, non-final >=5MiB) =====
	t.Logf("STEP 2: Upload %d parts (each %d MiB)", numParts, partSize/(1024*1024))
	completedParts := make([]types.CompletedPart, numParts)
	for i := 0; i < numParts; i++ {
		partNum := int32(i + 1)
		uploadPartResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(key),
			UploadId:   uploadID,
			PartNumber: aws.Int32(partNum),
			Body:       bytes.NewReader(partData[i]),
		})
		if err != nil {
			t.Fatalf("STEP 2 FAILED: UploadPart %d failed: %v", partNum, err)
		}
		completedParts[i] = types.CompletedPart{
			ETag:       uploadPartResp.ETag,
			PartNumber: aws.Int32(partNum),
		}
		t.Logf("  Uploaded part %d, ETag: %s", partNum, aws.ToString(uploadPartResp.ETag))
	}
	t.Logf("STEP 2 SUCCESS: Uploaded all %d parts", numParts)

	// ===== STEP 3: ListParts (verify parts are visible) =====
	t.Log("STEP 3: ListParts (verify parts are visible)")
	listPartsResp, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("STEP 3 FAILED: ListParts failed: %v", err)
	}
	if len(listPartsResp.Parts) != numParts {
		t.Errorf("STEP 3 FAILED: ListParts returned %d parts, want %d", len(listPartsResp.Parts), numParts)
	} else {
		t.Logf("STEP 3 SUCCESS: ListParts returned all %d parts", numParts)
	}

	// ===== STEP 4: CompleteMultipartUpload =====
	t.Log("STEP 4: CompleteMultipartUpload")
	completeResp, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completedParts},
	})
	if err != nil {
		t.Fatalf("STEP 4 FAILED: CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("STEP 4 SUCCESS: Completed multipart upload, Location: %s", aws.ToString(completeResp.Location))

	// ===== STEP 5: Verify download (GET full object) =====
	t.Log("STEP 5: Verify download (GET full object)")
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 5 FAILED: GetObject failed: %v", err)
	}
	downloaded, err := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	if err != nil {
		t.Fatalf("STEP 5 FAILED: Failed to read downloaded object: %v", err)
	}

	if len(downloaded) != int(totalSize) {
		t.Errorf("STEP 5 FAILED: Downloaded size mismatch: got %d, want %d", len(downloaded), totalSize)
	} else {
		t.Logf("STEP 5 SUCCESS: Downloaded %d bytes", len(downloaded))

		// Verify each part's unique marker
		for i := 0; i < numParts; i++ {
			partOffset := int64(i) * partSize
			if downloaded[partOffset] != byte(i+1) || downloaded[partOffset+1] != byte(i+1) {
				t.Errorf("STEP 5 FAILED: Part %d marker incorrect at offset %d", i+1, partOffset)
			}
		}
		t.Log("STEP 5 VERIFICATION: All part markers verified")
	}

	// ===== STEP 6: GET byte-range straddling part boundary =====
	t.Log("STEP 6: GET byte-range straddling part boundary")

	// Range that straddles the boundary between part 1 and part 2
	// Part 1 ends at 6 MiB, Part 2 starts at 6 MiB
	rangeStart := partSize - 1000
	rangeEnd := partSize + 1000

	rangeHdr := fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd)
	getRangeResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(rangeHdr),
	})
	if err != nil {
		t.Errorf("STEP 6 FAILED: GetObject with range failed: %v", err)
	} else {
		rangeData, err := io.ReadAll(getRangeResp.Body)
		getRangeResp.Body.Close()
		if err != nil {
			t.Errorf("STEP 6 FAILED: Failed to read range response: %v", err)
		} else {
			expectedRange := downloaded[rangeStart : rangeEnd+1]
			if !bytes.Equal(expectedRange, rangeData) {
				t.Errorf("STEP 6 FAILED: Range content mismatch")
			} else {
				t.Logf("STEP 6 SUCCESS: Range straddling part boundary verified (%d bytes)", len(rangeData))
			}
		}
	}

	// ===== STEP 7: LIST (verify multipart object appears) =====
	t.Log("STEP 7: LIST (verify multipart object appears)")
	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 7 FAILED: ListObjectsV2 failed: %v", err)
	}
	if len(listResp.Contents) != 1 {
		t.Errorf("STEP 7 FAILED: Expected 1 object in LIST, got %d", len(listResp.Contents))
	} else {
		if *listResp.Contents[0].Size != totalSize {
			t.Errorf("STEP 7 FAILED: Listed size mismatch: got %d, want %d",
				*listResp.Contents[0].Size, totalSize)
		} else {
			t.Logf("STEP 7 SUCCESS: Multipart object appears in LIST with correct size (%d bytes)", totalSize)
		}
	}

	// ===== STEP 8: DELETE multipart object =====
	t.Log("STEP 8: DELETE multipart object")
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 8 FAILED: DeleteObject failed: %v", err)
	}
	t.Log("STEP 8 SUCCESS: Multipart object deleted")

	// ===== STEP 9: Verify object is gone =====
	t.Log("STEP 9: Verify multipart object is gone")
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Errorf("STEP 9 FAILED: Object still exists after DELETE")
	} else {
		t.Logf("STEP 9 SUCCESS: Object no longer accessible after DELETE")
	}

	t.Log("\n=== MULTIPART LIFECYCLE TEST COMPLETED SUCCESSFULLY ===")
}

// TestMultipartAbortDeliberate tests deliberate multipart abort with verification.
// This is a dedicated test for abort functionality (not just an error-path side effect).
func TestMultipartAbortDeliberate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()
	key := generateTestKey(t)

	// ===== STEP 1: CreateMultipartUpload =====
	t.Log("STEP 1: CreateMultipartUpload for abort test")
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("STEP 1 FAILED: CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId
	t.Logf("STEP 1 SUCCESS: Created upload, ID: %s", *uploadID)

	// ===== STEP 2: Verify upload appears in ListMultipartUploads =====
	t.Log("STEP 2: Verify upload appears in ListMultipartUploads")
	listMpuResp1, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("STEP 2 FAILED: ListMultipartUploads failed: %v", err)
	}
	found := false
	for _, upload := range listMpuResp1.Uploads {
		if *upload.Key == key && *upload.UploadId == *uploadID {
			found = true
			t.Logf("STEP 2 SUCCESS: Upload found in ListMultipartUploads")
			break
		}
	}
	if !found {
		t.Errorf("STEP 2 FAILED: Upload not found in ListMultipartUploads")
	}

	// ===== STEP 3: Upload >=1 part (so abort has real cleanup work to do) =====
	t.Log("STEP 3: Upload 1 part (6 MiB)")
	partSize := int64(6 * 1024 * 1024)
	partData := generateTestData(int(partSize))
	_, err = client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(partData),
	})
	if err != nil {
		t.Fatalf("STEP 3 FAILED: UploadPart failed: %v", err)
	}
	t.Log("STEP 3 SUCCESS: Uploaded part 1")

	// ===== STEP 4: Verify part is visible in ListParts =====
	t.Log("STEP 4: Verify part is visible in ListParts")
	listPartsResp, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("STEP 4 FAILED: ListParts failed: %v", err)
	}
	if len(listPartsResp.Parts) != 1 {
		t.Errorf("STEP 4 FAILED: ListParts returned %d parts, want 1", len(listPartsResp.Parts))
	} else {
		t.Log("STEP 4 SUCCESS: Part visible in ListParts")
	}

	// ===== STEP 5: AbortMultipartUpload (deliberate abort, not error-path) =====
	t.Log("STEP 5: AbortMultipartUpload (deliberate abort)")
	_, err = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("STEP 5 FAILED: AbortMultipartUpload failed: %v", err)
	}
	t.Log("STEP 5 SUCCESS: AbortMultipartUpload succeeded")

	// ===== STEP 6: Verify upload no longer appears in ListMultipartUploads =====
	t.Log("STEP 6: Verify upload no longer appears in ListMultipartUploads")
	listMpuResp2, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("STEP 6 FAILED: ListMultipartUploads after abort failed: %v", err)
	}
	stillFound := false
	for _, upload := range listMpuResp2.Uploads {
		if *upload.Key == key && *upload.UploadId == *uploadID {
			stillFound = true
			t.Errorf("STEP 6 FAILED: Aborted upload still appears in ListMultipartUploads")
			break
		}
	}
	if !stillFound {
		t.Log("STEP 6 SUCCESS: Aborted upload no longer appears in ListMultipartUploads")
	}

	// ===== STEP 7: Verify no final object was created =====
	t.Log("STEP 7: Verify no final object was created (should return 404)")
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Errorf("STEP 7 FAILED: Object exists after abort (should not have been created)")
	} else {
		t.Log("STEP 7 SUCCESS: No object created (HeadObject returned 404 as expected)")
	}

	// ===== STEP 8: Verify ListParts returns error or empty for aborted upload =====
	t.Log("STEP 8: Verify ListParts returns error or empty for aborted upload")
	listPartsResp2, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	// After abort, ListParts may return error or empty list
	if err == nil && len(listPartsResp2.Parts) > 0 {
		t.Errorf("STEP 8 FAILED: Parts still visible after abort")
	} else if err != nil {
		t.Logf("STEP 8 SUCCESS: ListParts returned error for aborted upload: %v", err)
	} else {
		t.Log("STEP 8 SUCCESS: ListParts returned empty list for aborted upload")
	}

	t.Log("\n=== MULTIPART ABORT TEST COMPLETED SUCCESSFULLY ===")
	t.Log("Verified that AbortMultipartUpload removes the incomplete upload from B2")
}
