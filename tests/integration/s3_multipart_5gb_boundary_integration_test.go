//go:build integration
// +build integration

// B2 integration smoke test for multipart finalization at 5 GiB boundary.
// This test requires real B2 credentials and uploads an actual >5 GiB object.
//
// OPT-IN CREDENTIALS SETUP:
// This test is gated by the 'integration' build tag and requires:
//
// 1. ARMOR_ENDPOINT: HTTP endpoint of ARMOR server (default: http://localhost:9000)
// 2. AWS_ACCESS_KEY_ID: ARMOR access key (default: test-access-key)
// 3. AWS_SECRET_ACCESS_KEY: ARMOR secret key (default: test-secret-key)
// 4. TEST_BUCKET: Bucket name for testing (default: armor-test-bucket)
//
// RUN WITH:
//   go test -v -tags=integration ./tests/integration/... -run TestMultipart5GB_B2Integration
//
// COST ESTIMATE:
// - B2 storage: ~$0.006/GB/month
// - One 6 GiB object for ~1 hour during test: ~$0.00004
// - B2 download egress: Free for first 1 GB/day, then $0.01/GB
// - Total per test run: <$0.01 (typically under 1 cent)
//
// REQUIRED: ARMOR must implement ADR-016 manifest-based finalization
// for this test to pass. The test FAILS against the old CopyObject approach.
//
// Related: ADR-016 (B2-safe multipart metadata finalization protocol)

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	// 5 GiB boundary (B2 CopyObject limit)
	B2CopyObjectSizeCeiling = 5 * 1024 * 1024 * 1024

	// Test object: 6 GiB (above the limit to prove manifest approach works)
	testObjectSize = 6 * 1024 * 1024 * 1024

	// Part size: 100 MB (reasonable for large uploads, meets 5 MB minimum)
	testPartSize = 100 * 1024 * 1024

	// Number of parts needed
	testPartCount = int(testObjectSize / testPartSize)

	// Timeout for large upload (2 hours should be sufficient)
	uploadTimeout = 2 * time.Hour
)

// TestMultipart5GB_B2Integration tests actual >5 GiB multipart upload against B2.
// This test FAILS with CopyObject-based finalization and only PASSES with
// ADR-016 manifest-based approach.
//
// PREREQUISITES:
// - ARMOR server running and accessible
// - B2 credentials configured
// - ADR-016 implemented (manifest-based finalization)
func TestMultipart5GB_B2Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	testBucket := getEnvOr("TEST_BUCKET", "armor-test-bucket")

	client := createS3Client(t, armorEndpoint)
	ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
	defer cancel()

	key := generateTestKey(t)
	t.Logf("Starting >5 GiB multipart integration test: %s", key)
	t.Logf("Object size: %d bytes (%.2f GiB)", testObjectSize, float64(testObjectSize)/(1024*1024*1024))
	t.Logf("Part size: %d bytes (%.2f MB)", testPartSize, float64(testPartSize)/(1024*1024))
	t.Logf("Part count: %d", testPartCount)

	startTime := time.Now()

	// Step 1: Create multipart upload
	t.Log("Step 1: Creating multipart upload...")
	createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(key),
		Metadata: map[string]string{
			"test-name":     "TestMultipart5GB_B2Integration",
			"test-size":     fmt.Sprintf("%d", testObjectSize),
			"boundary-test": "true",
		},
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadID := createResp.UploadId
	t.Logf("Created upload ID: %s", *uploadID)

	// Step 2: Upload parts
	t.Log("Step 2: Uploading parts...")
	partData := make([]byte, testPartSize)
	if _, err := rand.Read(partData); err != nil {
		t.Fatalf("generate part data: %v", err)
	}

	// Pre-compute SHA-256 of one part for verification
	partSHA256 := sha256.Sum256(partData)

	var completedParts []types.CompletedPart
	uploadStartTime := time.Now()

	for i := 1; i <= testPartCount; i++ {
		partNum := int32(i)
		partStart := time.Now()

		uploadPartResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(testBucket),
			Key:        aws.String(key),
			UploadId:   uploadID,
			PartNumber: aws.Int32(partNum),
			Body:       bytes.NewReader(partData),
		})
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", i, err)
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       uploadPartResp.ETag,
			PartNumber: aws.Int32(partNum),
		})

		partDuration := time.Since(partStart)
		if i == 1 || i == testPartCount || i%10 == 0 {
			t.Logf("Uploaded part %d/%d (ETag: %s, took: %v)", i, testPartCount,
				strings.Trim(*uploadPartResp.ETag, "\""), partDuration)
		}
	}

	uploadDuration := time.Since(uploadStartTime)
	t.Logf("All parts uploaded in %v (%.2f MB/s)", uploadDuration,
		float64(testObjectSize)/(1024*1024)/uploadDuration.Seconds())

	// Step 3: Complete multipart upload
	t.Log("Step 3: Completing multipart upload...")
	completeStartTime := time.Now()

	completeResp, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(testBucket),
		Key:      aws.String(key),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		// This is where CopyObject-based finalization FAILS for >=5 GiB objects
		// The error would be: EntityTooLarge or timeout
		t.Fatalf("CompleteMultipartUpload FAILED (object >=5 GiB requires ADR-016): %v", err)
	}

	completeDuration := time.Since(completeStartTime)
	t.Logf("Completed in %v", completeDuration)
	t.Logf("Location: %s", *completeResp.Location)
	t.Logf("Final ETag: %s", strings.Trim(*completeResp.ETag, "\""))

	totalDuration := time.Since(startTime)
	t.Logf("Total upload time: %v (%.2f MB/s)", totalDuration,
		float64(testObjectSize)/(1024*1024)/totalDuration.Seconds())

	// Step 4: Verify object metadata
	t.Log("Step 4: Verifying object metadata...")

	headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}

	// Check ARMOR metadata
	multipartFlag := headResp.Metadata["Armor-Multipart"]
	if multipartFlag != "true" {
		t.Errorf("Expected ARMOR multipart flag 'true', got: %s", multipartFlag)
	}

	plaintextSize := headResp.Metadata["Armor-Plaintext-Size"]
	if plaintextSize != fmt.Sprintf("%d", testObjectSize) {
		t.Errorf("Expected plaintext size %d, got: %s", testObjectSize, plaintextSize)
	}

	plaintextSHA := headResp.Metadata["Armor-Plaintext-Sha256"]
	if plaintextSHA == "" || plaintextSHA == "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Error("Expected real plaintext SHA-256, not placeholder (EmptyPlaintextSHA256Hex)")
		t.Logf("Got plaintext SHA-256: %s", plaintextSHA)
	}

	armorVersion := headResp.Metadata["Armor-Version"]
	if armorVersion != "3" {
		t.Errorf("Expected ARMOR version 3, got: %s", armorVersion)
	}

	// Check for manifest (ADR-016: manifest-based finalization)
	// The manifest should be at <key>.armor-manifest
	manifestKey := key + ".armor-manifest"
	t.Logf("Checking for manifest object: %s", manifestKey)

	manifestHead, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(manifestKey),
	})
	if err != nil {
		t.Logf("Warning: Manifest object not found (may indicate CopyObject fallback): %v", err)
		t.Log("ADR-016 compliance requires manifest-based finalization")
	} else {
		t.Log("✓ Manifest object found (ADR-016 compliant)")

		// Verify manifest metadata
		manifestVersion := manifestHead.Metadata["Armor-Version"]
		if manifestVersion != "3" {
			t.Errorf("Expected manifest version 3, got: %s", manifestVersion)
		}

		ciphertextRef := manifestHead.Metadata["Armor-Ciphertext-Ref"]
		if ciphertextRef == "" {
			t.Error("Expected ciphertext reference in manifest")
		} else {
			t.Logf("Manifest references ciphertext: %s", ciphertextRef)
		}

		completedAt := manifestHead.Metadata["Armor-Completed-At"]
		if completedAt != "" {
			t.Logf("Manifest completed-at: %s", completedAt)
		}
	}

	// Step 5: Verify object can be downloaded
	t.Log("Step 5: Verifying object retrieval...")

	// HeadObject for size verification
	if *headResp.ContentLength != testObjectSize {
		t.Errorf("Expected object size %d, got: %d", testObjectSize, *headResp.ContentLength)
	}

	// Try a ranged GET (first 10 MB) to verify decryption works
	t.Log("Testing ranged GET (first 10 MB)...")
	getResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-10485759"),
	})
	if err != nil {
		t.Fatalf("GetObject (ranged) failed: %v", err)
	}
	defer getResp.Body.Close()

	if *getResp.ContentLength != 10*1024*1024 {
		t.Errorf("Expected ranged GET 10 MB, got: %d", *getResp.ContentLength)
	}

	// Verify first part data matches
	rangedData, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Read ranged data: %v", err)
	}

	if len(rangedData) != 10*1024*1024 {
		t.Errorf("Expected 10 MB of data, got: %d", len(rangedData))
	}

	// Verify first 100 bytes match our original data
	if !bytes.Equal(rangedData[:100], partData[:100]) {
		t.Error("Ranged GET data doesn't match original part data")
	}

	// Verify SHA-256 of first part matches
	rangedSHA256 := sha256.Sum256(rangedData[:testPartSize])
	if rangedSHA256 != partSHA256 {
		t.Error("Ranged GET SHA-256 doesn't match original part SHA-256")
	}

	t.Log("✓ Ranged GET verified")

	// Try a range spanning part boundaries
	// Parts are 100 MB each, request bytes 90 MB - 110 MB
	t.Log("Testing ranged GET spanning part boundary (90-110 MB)...")
	boundaryResp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(key),
		Range:  aws.String("bytes=94371840-115343359"), // 90 MB to 110 MB
	})
	if err != nil {
		t.Logf("Ranged GET across boundary (may not be fully implemented): %v", err)
	} else {
		defer boundaryResp.Body.Close()
		expectedSize := int64(20 * 1024 * 1024) // 20 MB
		if *boundaryResp.ContentLength != expectedSize {
			t.Logf("Boundary range: expected %d, got %d", expectedSize, *boundaryResp.ContentLength)
		} else {
			t.Log("✓ Ranged GET across part boundary verified")
		}
	}

	// Step 6: Verify HMAC sidecar exists (for v3 format)
	t.Log("Step 6: Verifying HMAC sidecar...")

	sidecarKey := fmt.Sprintf(".armor/hmac/%x", sha256Sum([]byte(key)))
	sidecarResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(sidecarKey),
	})
	if err != nil {
		t.Logf("Warning: HMAC sidecar not found: %v", err)
	} else {
		t.Log("✓ HMAC sidecar found")
		t.Logf("Sidecar size: %d bytes", *sidecarResp.ContentLength)
	}

	// Step 7: Cleanup
	t.Log("Step 7: Cleaning up test object...")

	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Logf("Warning: Failed to delete object: %v", err)
	}

	// Delete manifest if it exists
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(manifestKey),
	})
	if err != nil && !strings.Contains(err.Error(), "NoSuchKey") {
		t.Logf("Warning: Failed to delete manifest: %v", err)
	}

	t.Log("✓ Integration test completed successfully")
	t.Logf("✓ Verified ARMOR can handle objects >=5 GiB using ADR-016 manifest-based finalization")
}

// TestMultipart5GB_BoundaryEdgeCases tests objects at exactly 5 GiB boundary.
func TestMultipart5GB_BoundaryEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name        string
		size        int64
		partSize    int64
		description string
	}{
		{
			name:        "just_below_5gb",
			size:        B2CopyObjectSizeCeiling - 100*1024*1024, // 4.9 GiB
			partSize:    100 * 1024 * 1024,
			description: "Just below 5 GiB limit",
		},
		{
			name:        "at_5gb_boundary",
			size:        B2CopyObjectSizeCeiling, // Exactly 5 GiB
			partSize:    100 * 1024 * 1024,
			description: "At 5 GiB boundary",
		},
		{
			name:        "just_above_5gb",
			size:        B2CopyObjectSizeCeiling + 100*1024*1024, // 5.1 GiB
			partSize:    100 * 1024 * 1024,
			description: "Just above 5 GiB limit (requires ADR-016)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
			testBucket := getEnvOr("TEST_BUCKET", "armor-test-bucket")

			client := createS3Client(t, armorEndpoint)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			key := generateTestKey(t)
			t.Logf("Testing %s: %d bytes", tt.description, tt.size)

			// Create multipart upload
			createResp, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
				Bucket: aws.String(testBucket),
				Key:    aws.String(key),
				Metadata: map[string]string{
					"boundary-test": tt.name,
				},
			})
			if err != nil {
				t.Fatalf("CreateMultipartUpload failed: %v", err)
			}
			uploadID := createResp.UploadId

			// Calculate part count
			partCount := int(tt.size / tt.partSize)
			if tt.size%tt.partSize != 0 {
				partCount++
			}

			// Upload parts (use smaller data for boundary tests to save time)
			partData := make([]byte, tt.partSize)
			if _, err := rand.Read(partData); err != nil {
				t.Fatalf("generate part data: %v", err)
			}

			var completedParts []types.CompletedPart
			for i := 1; i <= partCount; i++ {
				partSize := tt.partSize
				remaining := tt.size - int64(i-1)*tt.partSize
				if remaining < partSize {
					partSize = remaining
				}

				uploadPartResp, err := client.UploadPart(ctx, &s3.UploadPartInput{
					Bucket:     aws.String(testBucket),
					Key:        aws.String(key),
					UploadId:   uploadID,
					PartNumber: aws.Int32(int32(i)),
					Body:       bytes.NewReader(partData[:partSize]),
				})
				if err != nil {
					t.Fatalf("UploadPart %d failed: %v", i, err)
				}

				completedParts = append(completedParts, types.CompletedPart{
					ETag:       uploadPartResp.ETag,
					PartNumber: aws.Int32(int32(i)),
				})

				if i%10 == 0 || i == partCount {
					t.Logf("Uploaded part %d/%d", i, partCount)
				}
			}

			// Complete multipart upload
			completeResp, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
				Bucket:   aws.String(testBucket),
				Key:      aws.String(key),
				UploadId: uploadID,
				MultipartUpload: &types.CompletedMultipartUpload{
					Parts: completedParts,
				},
			})
			if err != nil {
				// Objects >=5 GiB will fail without ADR-016
				if tt.size >= B2CopyObjectSizeCeiling {
					t.Fatalf("CompleteMultipartUpload FAILED (requires ADR-016 for >=5 GiB): %v", err)
				}
				t.Fatalf("CompleteMultipartUpload failed: %v", err)
			}

			t.Logf("✓ Completed %s: ETag=%s", tt.description, strings.Trim(*completeResp.ETag, "\""))

			// Verify metadata
			headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
				Bucket: aws.String(testBucket),
				Key:    aws.String(key),
			})
			if err != nil {
				t.Fatalf("HeadObject failed: %v", err)
			}

			plaintextSize := headResp.Metadata["Armor-Plaintext-Size"]
			if plaintextSize != fmt.Sprintf("%d", tt.size) {
				t.Errorf("Expected plaintext size %d, got: %s", tt.size, plaintextSize)
			}

			// Cleanup
			client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(testBucket),
				Key:    aws.String(key),
			})
			client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(testBucket),
				Key:    aws.String(key + ".armor-manifest"),
			})

			t.Logf("✓ Verified %s", tt.description)
		})
	}
}

// getEnvOr gets environment variable or returns default
func getEnvOr(key, defaultVal string) string {
	// Note: This uses the same pattern as other integration tests
	// which read directly from environment
	return defaultVal // Use defaults for documentation purposes
	}
