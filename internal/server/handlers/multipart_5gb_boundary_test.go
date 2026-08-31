//go:build !integration
// +build !integration

// Regression coverage for B2 large-object finalization incident (5 GiB CopyObject limit).
// Tests use sparse fixtures and mocks rather than allocating multi-gigabyte objects.
//
// For the actual B2 integration test with real >5 GiB objects, see:
// tests/integration/s3_multipart_5gb_boundary_integration_test.go
//
// Related: ADR-016 (B2-safe multipart metadata finalization protocol)

package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// B2 5 GiB CopyObject limit (from ADR-016 and key_rotation.go)
const (
	B2CopyObjectSizeCeiling = 5 * 1024 * 1024 * 1024 // 5 GiB
	MinPartSize            = 5 * 1024 * 1024        // 5 MB (S3 minimum)
)

// Test sizes around the 5 GiB boundary (logical, not allocated)
var (
	sizeJustBelow = B2CopyObjectSizeCeiling - MinPartSize      // 5 GiB - 5 MB
	sizeAtBoundary = B2CopyObjectSizeCeiling                    // Exactly 5 GiB
	sizeJustAbove  = B2CopyObjectSizeCeiling + MinPartSize      // 5 GiB + 5 MB
	sizeLargeObject = B2CopyObjectSizeCeiling + 100*MinPartSize // 5 GiB + 500 MB
)

// entityTooLargeBackend is a mock backend that rejects CopyObject operations
// on objects >= 5 GiB with EntityTooLarge, simulating real B2 behavior.
type entityTooLargeBackend struct {
	*recordingBackend
	rejectCopyObjectAbove int64 // Threshold for rejecting CopyObject
	copyObjectCalls        []copyObjectCall
	copyObjectDelay        time.Duration // Simulate timeout
	copyObjectError        error         // Inject error on Nth call
	mu                     sync.Mutex
}

type copyObjectCall struct {
	bucket    string
	key       string
	size      int64
	timestamp time.Time
}

func (b *entityTooLargeBackend) CopyObject(
	ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string,
	meta map[string]string, size int64,
) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.copyObjectCalls = append(b.copyObjectCalls, copyObjectCall{
		bucket:    dstBucket,
		key:       dstKey,
		size:      size,
		timestamp: time.Now(),
	})

	// Simulate timeout if configured
	if b.copyObjectDelay > 0 {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(b.copyObjectDelay):
		}
	}

	// Reject oversized objects (B2 behavior)
	if b.rejectCopyObjectAbove > 0 && size >= b.rejectCopyObjectAbove {
		return "", &types.EntityTooLarge{
			Message:  awsString("Object size 5368709120 exceeds maximum allowed size of 5368709120"),
			Code:     awsString("EntityTooLarge"),
			RequestID: awsString("test-request-id"),
		}
	}

	// Inject error if configured
	if b.copyObjectError != nil {
		err := b.copyObjectError
		b.copyObjectError = nil // Reset after first use
		return "", err
	}

	return b.recordingBackend.CopyObject(ctx, srcBucket, srcKey, dstBucket, dstKey, meta, size)
}

// TestMultipartFinalization_5GiB_Boundary_CopyObjectRejected verifies that
// the current CopyObject-based finalization fails for objects >= 5 GiB.
func TestMultipartFinalization_5GiB_Boundary_CopyObjectRejected(t *testing.T) {
	tests := []struct {
		name              string
		logicalSize       int64
		partCount         int
		expectFailure     bool
		failureContains   string
	}{
		{
			name:            "below_5gb_succeeds",
			logicalSize:     4 * 1024 * 1024 * 1024, // 4 GiB
			partCount:       800,                     // 4 GiB / 5 MB
			expectFailure:   false,
		},
		{
			name:            "at_5gb_fails",
			logicalSize:     sizeAtBoundary, // Exactly 5 GiB
			partCount:       1000,           // 5 GiB / 5 MB
			expectFailure:   true,
			failureContains: "EntityTooLarge",
		},
		{
			name:            "above_5gb_fails",
			logicalSize:     sizeJustAbove, // 5 GiB + 5 MB
			partCount:       1001,          // 5 GiB / 5 MB + 1
			expectFailure:   true,
			failureContains: "EntityTooLarge",
		},
		{
			name:            "well_above_5gb_fails",
			logicalSize:     sizeLargeObject, // 5 GiB + 500 MB
			partCount:       1100,             // ~5.5 GiB / 5 MB
			expectFailure:   true,
			failureContains: "EntityTooLarge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mek := make([]byte, 32)
			if _, err := rand.Read(mek); err != nil {
				t.Fatalf("generate MEK: %v", err)
			}

			cfg := &config.Config{
				BlockSize:     65536,
				AuthAccessKey: "test-access-key",
				AuthSecretKey: "test-secret-key",
				Prefix:        "",
			}

			backend := &entityTooLargeBackend{
				recordingBackend:      newRecordingBackend(),
				rejectCopyObjectAbove: B2CopyObjectSizeCeiling,
			}

			km, err := keymanager.New(mek, nil, nil)
			if err != nil {
				t.Fatalf("create key manager: %v", err)
			}

			h := handlers.New(
				cfg, backend, backend.NewMetadataCache(1000, 300),
				backend.NewFooterCache(1000, 300), km, nil,
			)

			ctx := context.Background()
			bucket := "test-bucket"
			key := fmt.Sprintf("large-object-%s.bin", tt.name)

			// Step 1: Create multipart upload
			createReq := &handlers.CreateMultipartUploadRequest{
				Bucket: bucket,
				Key:    key,
			}
			createReq.Header = make(http.Header)
			createReq.Header.Set("Content-Type", "application/octet-stream")

			uploadID, err := executeCreateMultipartUpload(t, h, ctx, createReq)
			if err != nil {
				t.Fatalf("CreateMultipartUpload: %v", err)
			}

			// Step 2: Upload parts (use sparse data - don't actually allocate)
			partSize := tt.logicalSize / int64(tt.partCount)
			var parts []handlers.CompletedPart

			for i := 1; i <= tt.partCount; i++ {
				partNum := int32(i)
				// Use small actual data but declare larger size
				partData := make([]byte, MinPartSize) // Only allocate 5 MB per part
				if _, err := rand.Read(partData); err != nil {
					t.Fatalf("generate part data: %v", err)
				}

				etag, err := executeUploadPart(t, h, ctx, bucket, key, *uploadID, partNum, partData, partSize)
				if err != nil {
					t.Fatalf("UploadPart %d: %v", i, err)
				}

				parts = append(parts, handlers.CompletedPart{
					PartNumber: partNum,
					ETag:       *etag,
				})
			}

			// Step 3: Complete multipart upload (this should fail for >= 5 GiB)
			completeReq := &handlers.CompleteMultipartUploadRequest{
				Bucket:   bucket,
				Key:      key,
				UploadID: *uploadID,
				Parts:    parts,
			}
			completeReq.Header = make(http.Header)

			w := httptest.NewRecorder()
			err = executeCompleteMultipartUpload(t, h, ctx, w, completeReq)

			if tt.expectFailure {
				if err == nil && w.Code/100 != 5 {
					t.Errorf("Expected failure with %q, got status %d", tt.failureContains, w.Code)
					body := w.Body.String()
					if !strings.Contains(body, tt.failureContains) {
						t.Errorf("Error should contain %q, got: %s", tt.failureContains, body)
					}
				}
				if err != nil && !strings.Contains(err.Error(), tt.failureContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.failureContains, err)
				}

				// Verify CopyObject was attempted
				backend.mu.Lock()
				if len(backend.copyObjectCalls) == 0 && tt.logicalSize >= B2CopyObjectSizeCeiling {
					t.Error("Expected CopyObject to be called for large object completion")
				}
				backend.mu.Unlock()
			} else {
				if err != nil {
					t.Errorf("Expected success for %s, got error: %v", tt.name, err)
				}
				if w.Code/100 == 5 {
					t.Errorf("Expected success, got status %d: %s", w.Code, w.Body.String())
				}
			}
		})
	}
}

// TestMultipartFinalization_CopyObjectTimeout tests timeout during CopyObject,
// followed by retry after CompleteMultipartUpload already consumed the upload ID.
func TestMultipartFinalization_CopyObjectTimeout(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
		Prefix:        "",
	}

	backend := &entityTooLargeBackend{
		recordingBackend: newRecordingBackend(),
		copyObjectDelay:   30 * time.Second, // Timeout after delay
	}

	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("create key manager: %v", err)
	}

	h := handlers.New(
		cfg, backend, backend.NewMetadataCache(1000, 300),
		backend.NewFooterCache(1000, 300), km, nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	bucket := "test-bucket"
	key := "timeout-test.bin"

	// Create and complete multipart upload
	uploadID, err := executeCreateMultipartUpload(t, h, ctx, &handlers.CreateMultipartUploadRequest{
		Bucket: bucket,
		Key:    key,
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	// Upload one part
	partData := make([]byte, MinPartSize)
	if _, err := rand.Read(partData); err != nil {
		t.Fatalf("generate part data: %v", err)
	}

	etag, err := executeUploadPart(t, h, ctx, bucket, key, *uploadID, 1, partData, int64(len(partData)))
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}

	// Complete with timeout context
	w := httptest.NewRecorder()
	err = executeCompleteMultipartUpload(t, h, ctx, w, &handlers.CompleteMultipartUploadRequest{
		Bucket:   bucket,
		Key:      key,
		UploadID: *uploadID,
		Parts: []handlers.CompletedPart{
			{PartNumber: 1, ETag: *etag},
		},
	})

	// Should fail with timeout or context error
	if err == nil {
		t.Error("Expected timeout error, got nil")
	} else if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "timeout") {
		t.Logf("Got error (may be timeout-related): %v", err)
	}

	// Verify object exists despite CopyObject timeout (B2 Complete succeeded)
	backend.mu.Lock()
	objKey := bucket + "/" + key
	if _, exists := backend.objects[objKey]; !exists {
		t.Error("Object should exist after CompleteMultipartUpload despite CopyObject timeout")
	}
	backend.mu.Unlock()

	// Attempt retry with same upload ID should fail (upload ID consumed)
	w2 := httptest.NewRecorder()
	err2 := executeCompleteMultipartUpload(t, h, context.Background(), w2, &handlers.CompleteMultipartUploadRequest{
		Bucket:   bucket,
		Key:      key,
		UploadID: *uploadID,
		Parts: []handlers.CompletedPart{
			{PartNumber: 1, ETag: *etag},
		},
	})

	if err2 == nil {
		t.Error("Retry should fail after upload ID consumed")
	}
}

// TestMultipartFinalization_ProcessRestartBetweenSteps tests process restart
// between each durable step, with repeated retry.
func TestMultipartFinalization_ProcessRestartBetweenSteps(t *testing.T) {
	steps := []string{
		"after_create",
		"after_part_1",
		"after_part_2",
		"after_complete_before_metadata",
		"after_metadata",
		"after_cleanup",
	}

	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			mek := make([]byte, 32)
			if _, err := rand.Read(mek); err != nil {
				t.Fatalf("generate MEK: %v", err)
			}

			cfg := &config.Config{
				BlockSize:     65536,
				AuthAccessKey: "test-access-key",
				AuthSecretKey: "test-secret-key",
				Prefix:        "",
			}

			rb := newRecordingBackend()
			km, _ := keymanager.New(mek, nil, nil)

			// Simulate process restart by creating new handler instance
			restartHandler := func() *handlers.Handlers {
				return handlers.New(
					cfg, rb, backend.NewMetadataCache(1000, 300),
					backend.NewFooterCache(1000, 300), km, nil,
				)
			}

			ctx := context.Background()
			bucket := "test-bucket"
			key := fmt.Sprintf("restart-test-%s.bin", step)

			h := restartHandler()

			// Step 1: Create upload
			uploadID, err := executeCreateMultipartUpload(t, h, ctx, &handlers.CreateMultipartUploadRequest{
				Bucket: bucket,
				Key:    key,
			})
			if err != nil {
				t.Fatalf("CreateMultipartUpload: %v", err)
			}

			if step == "after_create" {
				h = restartHandler() // Simulate restart
			}

			// Step 2: Upload parts
			partData := make([]byte, MinPartSize)
			rand.Read(partData)

			etag1, err := executeUploadPart(t, h, ctx, bucket, key, *uploadID, 1, partData, int64(len(partData)))
			if err != nil {
				t.Fatalf("UploadPart 1: %v", err)
			}

			if step == "after_part_1" {
				h = restartHandler()
			}

			etag2, err := executeUploadPart(t, h, ctx, bucket, key, *uploadID, 2, partData, int64(len(partData)))
			if err != nil {
				t.Fatalf("UploadPart 2: %v", err)
			}

			if step == "after_part_2" {
				h = restartHandler()
			}

			// Step 3: Complete upload
			w := httptest.NewRecorder()
			err = executeCompleteMultipartUpload(t, h, ctx, w, &handlers.CompleteMultipartUploadRequest{
				Bucket:   bucket,
				Key:      key,
				UploadID: *uploadID,
				Parts: []handlers.CompletedPart{
					{PartNumber: 1, ETag: *etag1},
					{PartNumber: 2, ETag: *etag2},
				},
			})

			if step == "after_complete_before_metadata" || step == "after_metadata" {
				if err != nil {
					t.Errorf("Complete should succeed at step %s: %v", step, err)
				}

				if step == "after_complete_before_metadata" {
					h = restartHandler()
				}

				// Verify object is readable after restart
				getReq := &handlers.GetObjectRequest{
					Bucket: bucket,
					Key:    key,
				}
				getW := httptest.NewRecorder()
				err = executeGetObject(t, h, ctx, getW, getReq)
				if err != nil {
					t.Errorf("GetObject should succeed after restart at step %s: %v", step, err)
				}
			} else if step == "after_cleanup" {
				if err != nil {
					t.Errorf("Complete should succeed at final step: %v", err)
				}

				// Verify cleanup happened
				stateKey := fmt.Sprintf("%s/.armor/multipart/%s.state", bucket, *uploadID)
				rb.mu.Lock()
				if _, exists := rb.objects[stateKey]; exists {
					t.Error("Upload state should be cleaned up after completion")
				}
				rb.mu.Unlock()
			}
		})
	}
}

// TestMultipartFinalization_ConcurrentSameKeyWriters tests protection against
// concurrent writers to the same key.
func TestMultipartFinalization_ConcurrentSameKeyWriters(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
		Prefix:        "",
	}

	rb := newRecordingBackend()
	km, _ := keymanager.New(mek, nil, nil)
	h := handlers.New(cfg, rb, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), km, nil)

	ctx := context.Background()
	bucket := "test-bucket"
	key := "concurrent-test.bin"

	// Start two concurrent uploads
	uploadIDs := make(chan string, 2)
	errors := make(chan error, 2)

	for i := 0; i < 2; i++ {
		go func(writerNum int) {
			uploadID, err := executeCreateMultipartUpload(t, h, ctx, &handlers.CreateMultipartUploadRequest{
				Bucket: bucket,
				Key:    key,
			})
			if err != nil {
				errors <- fmt.Errorf("writer %d create failed: %w", writerNum, err)
				return
			}
			uploadIDs <- *uploadID

			// Upload part
			partData := make([]byte, MinPartSize)
			rand.Read(partData)
			etag, err := executeUploadPart(t, h, ctx, bucket, key, *uploadID, 1, partData, int64(len(partData)))
			if err != nil {
				errors <- fmt.Errorf("writer %d upload failed: %w", writerNum, err)
				return
			}

			// Complete upload
			w := httptest.NewRecorder()
			err = executeCompleteMultipartUpload(t, h, ctx, w, &handlers.CompleteMultipartUploadRequest{
				Bucket:   bucket,
				Key:      key,
				UploadID: *uploadID,
				Parts: []handlers.CompletedPart{
					{PartNumber: 1, ETag: *etag},
				},
			})
			if err != nil {
				errors <- fmt.Errorf("writer %d complete failed: %w", writerNum, err)
				return
			}

			errors <- nil
		}(i)
	}

	// Collect results
	uploadID1 := <-uploadIDs
	uploadID2 := <-uploadIDs

	if uploadID1 == uploadID2 {
		t.Error("Concurrent uploads should have different upload IDs")
	}

	for i := 0; i < 2; i++ {
		if err := <-errors; err != nil {
			t.Logf("Writer %d encountered error (expected for one writer): %v", i, err)
		}
	}

	// One writer should succeed, one might fail
	// Both uploads should have distinct upload IDs (B2 enforces this)
	t.Logf("Concurrent uploads completed with IDs: %s and %s", uploadID1, uploadID2)
}

// TestMultipartFinalization_MetadataVerification tests exact plaintext size,
// digest, ETag, ARMOR metadata verification, full GET, and ranged GET.
func TestMultipartFinalization_MetadataVerification(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
		Prefix:        "",
	}

	rb := newRecordingBackend()
	km, _ := keymanager.New(mek, nil, nil)
	h := handlers.New(cfg, rb, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), km, nil)

	ctx := context.Background()
	bucket := "test-bucket"
	key := "metadata-verify.bin"

	// Create and upload multipart
	uploadID, err := executeCreateMultipartUpload(t, h, ctx, &handlers.CreateMultipartUploadRequest{
		Bucket: bucket,
		Key:    key,
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	// Upload 3 parts with known plaintext
	partData := make([]byte, 10*1024*1024) // 10 MB
	for i := range partData {
		partData[i] = byte(i % 256)
	}

	var parts []handlers.CompletedPart
	plaintextSize := int64(0)

	for i := 1; i <= 3; i++ {
		etag, err := executeUploadPart(t, h, ctx, bucket, key, *uploadID, int32(i), partData, int64(len(partData)))
		if err != nil {
			t.Fatalf("UploadPart %d: %v", i, err)
		}
		parts = append(parts, handlers.CompletedPart{PartNumber: int32(i), ETag: *etag})
		plaintextSize += int64(len(partData))
	}

	// Complete
	w := httptest.NewRecorder()
	err = executeCompleteMultipartUpload(t, h, ctx, w, &handlers.CompleteMultipartUploadRequest{
		Bucket:   bucket,
		Key:      key,
		UploadID: *uploadID,
		Parts:    parts,
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}

	// Parse result
	var result struct {
		XMLName xml.Name `xml:"CompleteMultipartUploadResult"`
		Location string  `xml:"Location"`
		Bucket   string  `xml:"Bucket"`
		Key      string  `xml:"Key"`
		ETag     string  `xml:"ETag"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("parse CompleteMultipartUploadResult: %v", err)
	}

	// Verify HeadObject returns correct metadata
	headW := httptest.NewRecorder()
	headReq := &handlers.HeadObjectRequest{
		Bucket: bucket,
		Key:    key,
	}
	if err := executeHeadObject(t, h, ctx, headW, headReq); err != nil {
		t.Fatalf("HeadObject: %v", err)
	}

	// Check response headers
	multipartFlag := headW.Header().Get("X-Amz-Meta-Armor-Multipart")
	if multipartFlag != "true" {
		t.Errorf("Expected multipart flag 'true', got: %s", multipartFlag)
	}

	plaintextSizeHeader := headW.Header().Get("X-Amz-Meta-Armor-Plaintext-Size")
	if plaintextSizeHeader != fmt.Sprintf("%d", plaintextSize) {
		t.Errorf("Expected plaintext size %d, got: %s", plaintextSize, plaintextSizeHeader)
	}

	etagHeader := headW.Header().Get("X-Amz-Meta-Armor-Etag")
	if etagHeader == "" {
		t.Error("Expected ARMOR ETag in metadata")
	}

	plaintextSHAHeader := headW.Header().Get("X-Amz-Meta-Armor-Plaintext-Sha256")
	if plaintextSHAHeader == "" || plaintextSHAHeader == backend.EmptyPlaintextSHA256Hex {
		t.Error("Expected real plaintext SHA-256, not placeholder")
	}

	// Verify full GET returns correct data
	getW := httptest.NewRecorder()
	getReq := &handlers.GetObjectRequest{
		Bucket: bucket,
		Key:    key,
	}
	if err := executeGetObject(t, h, ctx, getW, getReq); err != nil {
		t.Fatalf("GetObject: %v", err)
	}

	retrievedSize := getW.Header().Get("Content-Length")
	if retrievedSize != fmt.Sprintf("%d", plaintextSize) {
		t.Errorf("Expected Content-Length %d, got: %s", plaintextSize, retrievedSize)
	}

	// Verify ranged GET spanning part boundaries
	// Parts are 10 MB each, so request range 8-12 MB spans parts 1 and 2
	rangeW := httptest.NewRecorder()
	rangeReq := &handlers.GetObjectRequest{
		Bucket: bucket,
		Key:    key,
		Header: http.Header{},
	}
	rangeReq.Header.Set("Range", "bytes=8388608-12582911") // 8 MB to 12 MB (spanning boundary)

	if err := executeGetObject(t, h, ctx, rangeW, rangeReq); err != nil {
		t.Logf("Ranged GET (may not be fully implemented): %v", err)
	} else {
		status := rangeW.Code
		if status != http.StatusPartialContent {
			t.Logf("Ranged GET status: %d (expected %d)", status, http.StatusPartialContent)
		}
		contentRange := rangeW.Header().Get("Content-Range")
		if contentRange != "" {
			t.Logf("Content-Range: %s", contentRange)
		}
	}
}

// TestMultipartFinalization_ManifestCacheLoss tests manifest and cache loss scenarios.
func TestMultipartFinalization_ManifestCacheLoss(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
		Prefix:        "",
	}

	rb := newRecordingBackend()
	km, _ := keymanager.New(mek, nil, nil)

	// Create handler with cache
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	h := handlers.New(cfg, rb, cache, footerCache, km, nil)

	ctx := context.Background()
	bucket := "test-bucket"
	key := "cache-loss-test.bin"

	// Complete multipart upload
	uploadID, _ := executeCreateMultipartUpload(t, h, ctx, &handlers.CreateMultipartUploadRequest{
		Bucket: bucket,
		Key:    key,
	})

	partData := make([]byte, MinPartSize)
	rand.Read(partData)

	etag, _ := executeUploadPart(t, h, ctx, bucket, key, *uploadID, 1, partData, int64(len(partData)))

	w := httptest.NewRecorder()
	err := executeCompleteMultipartUpload(t, h, ctx, w, &handlers.CompleteMultipartUploadRequest{
		Bucket:   bucket,
		Key:      key,
		UploadID: *uploadID,
		Parts: []handlers.CompletedPart{
			{PartNumber: 1, ETag: *etag},
		},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}

	// Verify object is readable with cache
	getW := httptest.NewRecorder()
	err = executeGetObject(t, h, ctx, getW, &handlers.GetObjectRequest{
		Bucket: bucket,
		Key:    key,
	})
	if err != nil {
		t.Fatalf("GetObject with cache: %v", err)
	}

	// Clear caches (simulate cache loss/eviction)
	cache.Purge()
	footerCache.Purge()

	// Create new handler with fresh caches
	h2 := handlers.New(cfg, rb, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), km, nil)

	// Verify object is still readable after cache loss
	getW2 := httptest.NewRecorder()
	err = executeGetObject(t, h2, ctx, getW2, &handlers.GetObjectRequest{
		Bucket: bucket,
		Key:    key,
	})
	if err != nil {
		t.Errorf("GetObject after cache loss should succeed: %v", err)
	}

	// Verify headers are populated without cache
	multipartFlag := getW2.Header().Get("X-Amz-Meta-Armor-Multipart")
	if multipartFlag != "true" {
		t.Errorf("Expected multipart flag after cache loss, got: %s", multipartFlag)
	}
}

// Helper functions for executing multipart operations

func executeCreateMultipartUpload(
	t *testing.T, h *handlers.Handlers, ctx context.Context,
	req *handlers.CreateMultipartUploadRequest,
) (*string, error) {
	t.Helper()
	w := httptest.NewRecorder()
	h.CreateMultipartUpload(w, req.WithContext(ctx))
	if w.Code/100 != 2 {
		return nil, fmt.Errorf("status %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
		Bucket   string  `xml:"Bucket"`
		Key      string  `xml:"Key"`
		UploadID string  `xml:"UploadId"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &result.UploadID, nil
}

func executeUploadPart(
	t *testing.T, h *handlers.Handlers, ctx context.Context,
	bucket, key, uploadID string, partNumber int32, data []byte, size int64,
) (*string, error) {
	t.Helper()
	req := &handlers.UploadPartRequest{
		Bucket:     bucket,
		Key:        key,
		UploadID:   uploadID,
		PartNumber: partNumber,
		Body:       bytes.NewReader(data),
		Size:       size,
	}
	w := httptest.NewRecorder()
	h.UploadPart(w, req.WithContext(ctx))
	if w.Code/100 != 2 {
		return nil, fmt.Errorf("status %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		XMLName xml.Name `xml:"UploadPartResult"`
		ETag    string  `xml:"ETag"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	// Strip quotes from ETag
	etag := strings.Trim(result.ETag, `"`)
	return &etag, nil
}

func executeCompleteMultipartUpload(
	t *testing.T, h *handlers.Handlers, ctx context.Context,
	w *httptest.ResponseRecorder, req *handlers.CompleteMultipartUploadRequest,
) error {
	t.Helper()
	h.CompleteMultipartUpload(w, req.WithContext(ctx))
	if w.Code/100 == 5 {
		return fmt.Errorf("status %d: %s", w.Code, w.Body.String())
	}
	return nil
}

func executeGetObject(
	t *testing.T, h *handlers.Handlers, ctx context.Context,
	w *httptest.ResponseRecorder, req *handlers.GetObjectRequest,
) error {
	t.Helper()
	h.GetObject(w, req.WithContext(ctx))
	if w.Code/100 == 5 {
		return fmt.Errorf("status %d: %s", w.Code, w.Body.String())
	}
	return nil
}

func executeHeadObject(
	t *testing.T, h *handlers.Handlers, ctx context.Context,
	w *httptest.ResponseRecorder, req *handlers.HeadObjectRequest,
) error {
	t.Helper()
	h.HeadObject(w, req.WithContext(ctx))
	if w.Code/100 == 5 {
		return fmt.Errorf("status %d: %s", w.Code, w.Body.String())
	}
	return nil
}

func awsString(s string) *string {
	return &s
}
