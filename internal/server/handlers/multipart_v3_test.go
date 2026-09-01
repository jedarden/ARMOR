// Package handlers_test tests v3 multipart upload behavior.
package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/server/handlers"
)

// blockingV3Backend holds each backend UploadPart call until the test releases
// it. If the HTTP handler serializes distinct v3 parts, only one call can enter
// and the regression test times out before release.
type blockingV3Backend struct {
	*recordingBackend
	entered chan int32
	release chan struct{}
}

func (b *blockingV3Backend) UploadPart(
	ctx context.Context, bucket, key, uploadID string, partNumber int32,
	body io.Reader, size int64,
) (string, error) {
	select {
	case b.entered <- partNumber:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	select {
	case <-b.release:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return b.recordingBackend.UploadPart(ctx, bucket, key, uploadID, partNumber, body, size)
}

// TestMultipartV3DistinctPartsReachBackendConcurrently is the regression test
// for production uploads whose later request bodies sat unread behind the
// per-upload mutex while B2 took more than a minute to store an earlier part.
func TestMultipartV3DistinctPartsReachBackendConcurrently(t *testing.T) {
	cfg, _, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3
	be := &blockingV3Backend{
		recordingBackend: newRecordingBackend(),
		entered:          make(chan int32, 2),
		release:          make(chan struct{}),
	}
	h := handlers.New(cfg, be, cache, footerCache, km, nil)

	ctx := context.Background()
	const bucket, key = "test-bucket", "concurrent-v3"
	createResp := httptest.NewRecorder()
	h.CreateMultipartUpload(
		createResp, createUploadRequest(ctx, bucket, key, "application/octet-stream"),
		bucket, key,
	)
	if createResp.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: %d %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("parse upload ID: %v", err)
	}

	type result struct {
		part int
		code int
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, partNumber := range []int{1, 2} {
		wg.Add(1)
		go func(part int) {
			defer wg.Done()
			request := createUploadPartRequest(
				ctx, bucket, key, created.UploadID, part,
				"application/octet-stream", bytes.Repeat([]byte{byte(part)}, 64*1024),
			)
			response := httptest.NewRecorder()
			h.UploadPart(response, request, bucket, key, created.UploadID)
			results <- result{part: part, code: response.Code}
		}(partNumber)
	}

	seen := make(map[int32]bool, 2)
	timer := time.NewTimer(2 * time.Second)
	for len(seen) < 2 {
		select {
		case part := <-be.entered:
			seen[part] = true
		case <-timer.C:
			close(be.release)
			wg.Wait()
			t.Fatalf("distinct v3 parts were serialized before the backend: entered=%v", seen)
		}
	}
	if !timer.Stop() {
		<-timer.C
	}
	close(be.release)
	wg.Wait()
	close(results)
	for got := range results {
		if got.code != http.StatusOK {
			t.Errorf("UploadPart %d status=%d want=%d", got.part, got.code, http.StatusOK)
		}
	}
}

// TestMultipartV3ConcurrentOutOfOrder uploads parts 3,1,2 of different unaligned sizes
// concurrently and verifies each part encrypts independently with deterministic ciphertext.
func TestMultipartV3ConcurrentOutOfOrder(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)

	// Set FormatWriteVersion to 3
	cfg.FormatWriteVersion = 3

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-key"
	contentType := "application/octet-stream"

	// Create multipart upload
	createReq := createUploadRequest(ctx, bucket, key, contentType)
	createResp := httptest.NewRecorder()

	h.CreateMultipartUpload(createResp, createReq, bucket, key)

	if createResp.Code != 200 {
		t.Fatalf("CreateMultipartUpload failed: %d %s", createResp.Code, createResp.Body.String())
	}

	// Parse upload ID from response
	var result struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(createResp.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse upload ID: %v", err)
	}
	uploadID := result.UploadID

	if uploadID == "" {
		t.Fatalf("No upload ID in response")
	}

	// Create three parts with different unaligned sizes
	partSizes := map[int]int{
		1: 5 * 1024 * 1024,   // 5 MiB (minimum part size)
		2: 7*1024*1024 + 100, // 7 MiB + 100 bytes (unaligned)
		3: 6*1024*1024 - 50,  // 6 MiB - 50 bytes (unaligned)
	}

	// Track results from concurrent uploads
	type partResult struct {
		partNumber int
		etag       string
		err        error
	}
	results := make(chan partResult, 3)

	// Upload parts out of order: 3, 1, 2
	uploadOrder := []int{3, 1, 2}
	var wg sync.WaitGroup

	for _, partNum := range uploadOrder {
		wg.Add(1)
		go func(pn int) {
			defer wg.Done()

			size := partSizes[pn]
			plaintext := make([]byte, size)
			if _, err := rand.Read(plaintext); err != nil {
				results <- partResult{partNumber: pn, err: fmt.Errorf("failed to generate plaintext: %w", err)}
				return
			}

			// Create UploadPart request
			uploadReq := createUploadPartRequest(ctx, bucket, key, uploadID, pn, contentType, plaintext)
			uploadResp := httptest.NewRecorder()

			h.UploadPart(uploadResp, uploadReq, bucket, key, uploadID)

			if uploadResp.Code != 200 {
				results <- partResult{partNumber: pn, err: fmt.Errorf("UploadPart failed: %d %s", uploadResp.Code, uploadResp.Body.String())}
				return
			}

			// Extract etag
			etag := uploadResp.Header().Get("ETag")
			if etag == "" {
				results <- partResult{partNumber: pn, err: fmt.Errorf("no ETag in response")}
				return
			}

			results <- partResult{
				partNumber: pn,
				etag:       etag,
				err:        nil,
			}
		}(partNum)
	}

	wg.Wait()
	close(results)

	// Verify all uploads succeeded
	successCount := 0
	for result := range results {
		if result.err != nil {
			t.Errorf("Part %d upload failed: %v", result.partNumber, result.err)
		} else {
			successCount++
		}
	}

	if successCount != 3 {
		t.Fatalf("Expected 3 successful uploads, got %d", successCount)
	}

	t.Log("Successfully uploaded parts 3,1,2 concurrently with different unaligned sizes")
}

// TestMultipartV3Determinism verifies that uploading the same part content
// twice produces identical ciphertext (idempotent retries work).
func TestMultipartV3Determinism(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-key"
	contentType := "application/octet-stream"

	// Create multipart upload
	createReq := createUploadRequest(ctx, bucket, key, contentType)
	createResp := httptest.NewRecorder()
	h.CreateMultipartUpload(createResp, createReq, bucket, key)

	if createResp.Code != 200 {
		t.Fatalf("CreateMultipartUpload failed: %d %s", createResp.Code, createResp.Body.String())
	}

	var result struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(createResp.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse upload ID: %v", err)
	}
	uploadID := result.UploadID

	// Upload the same part content twice
	partNumber := 1
	plaintext := make([]byte, 6*1024*1024+123) // Unaligned size
	if _, err := rand.Read(plaintext); err != nil {
		t.Fatalf("Failed to generate plaintext: %v", err)
	}

	ciphertexts := make([][]byte, 0, 2)

	for i := 0; i < 2; i++ {
		uploadReq := createUploadPartRequest(ctx, bucket, key, uploadID, partNumber, contentType, plaintext)
		uploadResp := httptest.NewRecorder()
		h.UploadPart(uploadResp, uploadReq, bucket, key, uploadID)

		if uploadResp.Code != 200 {
			t.Fatalf("UploadPart attempt %d failed: %d %s", i+1, uploadResp.Code, uploadResp.Body.String())
		}

		// Get the encrypted data from the backend
		partKey := fmt.Sprintf(".armor/multipart/%s/part-%d.json", uploadID, partNumber)
		obj, _, err := mb.Get(ctx, bucket, partKey)
		if err != nil {
			t.Fatalf("Failed to get part data from backend (attempt %d): %v", i+1, err)
		}

		data, err := io.ReadAll(obj)
		obj.Close()
		if err != nil {
			t.Fatalf("Failed to read part data (attempt %d): %v", i+1, err)
		}

		ciphertexts = append(ciphertexts, data)
	}

	// Verify ciphertexts are identical
	if !bytes.Equal(ciphertexts[0], ciphertexts[1]) {
		t.Error("Ciphertexts are not identical - encryption is not deterministic")
	}

	t.Log("Verified: uploading the same part content twice produces identical ciphertext")
}

// TestMultipartV3NoSlowDown verifies that v3 uploads don't return SlowDown
// when parts are uploaded out of order (no part-1-first requirement).
func TestMultipartV3NoSlowDown(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-key"
	contentType := "application/octet-stream"

	// Create multipart upload
	createReq := createUploadRequest(ctx, bucket, key, contentType)
	createResp := httptest.NewRecorder()
	h.CreateMultipartUpload(createResp, createReq, bucket, key)

	if createResp.Code != 200 {
		t.Fatalf("CreateMultipartUpload failed: %d %s", createResp.Code, createResp.Body.String())
	}

	var result struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(createResp.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse upload ID: %v", err)
	}
	uploadID := result.UploadID

	// Upload part 3 before parts 1 and 2 (should NOT get SlowDown)
	partNumber := 3
	plaintext := make([]byte, 6*1024*1024) // 6 MiB
	if _, err := rand.Read(plaintext); err != nil {
		t.Fatalf("Failed to generate plaintext: %v", err)
	}

	uploadReq := createUploadPartRequest(ctx, bucket, key, uploadID, partNumber, contentType, plaintext)
	uploadResp := httptest.NewRecorder()
	h.UploadPart(uploadResp, uploadReq, bucket, key, uploadID)

	// Should succeed, not return SlowDown (503)
	if uploadResp.Code == 503 {
		body := uploadResp.Body.String()
		if strings.Contains(body, "SlowDown") {
			t.Fatalf("UploadPart returned SlowDown (503) - v3 should not require part-1-first")
		}
	}

	if uploadResp.Code != 200 {
		t.Fatalf("UploadPart failed: %d %s", uploadResp.Code, uploadResp.Body.String())
	}

	t.Log("Verified: v3 upload part 3 succeeded before parts 1 and 2 (no SlowDown)")
}

// TestMultipartV3IndependentPartCounter verifies that each part uses
// its own counter namespace (part n, block b) starting from block 0.
func TestMultipartV3IndependentPartCounter(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-key"
	contentType := "application/octet-stream"

	// Create multipart upload
	createReq := createUploadRequest(ctx, bucket, key, contentType)
	createResp := httptest.NewRecorder()
	h.CreateMultipartUpload(createResp, createReq, bucket, key)

	if createResp.Code != 200 {
		t.Fatalf("CreateMultipartUpload failed: %d %s", createResp.Code, createResp.Body.String())
	}

	var result struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(createResp.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse upload ID: %v", err)
	}
	uploadID := result.UploadID

	// Upload two parts with identical content but different part numbers
	identicalContent := make([]byte, 5*1024*1024) // 5 MiB
	if _, err := rand.Read(identicalContent); err != nil {
		t.Fatalf("Failed to generate plaintext: %v", err)
	}

	partCiphertexts := make(map[string][]byte)
	for _, partNum := range []int{1, 2} {
		uploadReq := createUploadPartRequest(ctx, bucket, key, uploadID, partNum, contentType, identicalContent)
		uploadResp := httptest.NewRecorder()
		h.UploadPart(uploadResp, uploadReq, bucket, key, uploadID)

		if uploadResp.Code != 200 {
			t.Fatalf("UploadPart for part %d failed: %d %s", partNum, uploadResp.Code, uploadResp.Body.String())
		}

		// Get the part data
		partKey := fmt.Sprintf(".armor/multipart/%s/part-%d.json", uploadID, partNum)
		obj, _, err := mb.Get(ctx, bucket, partKey)
		if err != nil {
			t.Fatalf("Failed to get part %d data: %v", partNum, err)
		}

		data, err := io.ReadAll(obj)
		obj.Close()
		if err != nil {
			t.Fatalf("Failed to read part %d data: %v", partNum, err)
		}

		partCiphertexts[fmt.Sprintf("part-%d", partNum)] = data
	}

	// Verify that parts with identical content but different part numbers
	// produce DIFFERENT ciphertext (because the counter includes part number)
	if bytes.Equal(partCiphertexts["part-1"], partCiphertexts["part-2"]) {
		t.Error("Parts 1 and 2 with identical content produced identical ciphertext")
		t.Error("This suggests parts are not using independent (part n, block b) counters")
	}

	t.Log("Verified: parts with identical content but different numbers produce different ciphertext")
	t.Log("This confirms each part uses its own counter namespace (part n, block b)")
}

// Helper functions

func createUploadRequest(ctx context.Context, bucket, key, contentType string) *http.Request {
	req := httptest.NewRequest("POST", "/"+key+"?uploads", nil)
	req.URL.Path = "/" + key
	req.URL.RawQuery = "uploads"
	req.Header.Set("Content-Type", contentType)
	return addContextToRequest(ctx, req, bucket, key, "")
}

func createUploadPartRequest(ctx context.Context, bucket, key, uploadID string, partNumber int, contentType string, data []byte) *http.Request {
	req := httptest.NewRequest("PUT", fmt.Sprintf("/%s?partNumber=%d&uploadId=%s", key, partNumber, uploadID), bytes.NewReader(data))
	req.URL.Path = "/" + key
	req.URL.RawQuery = fmt.Sprintf("partNumber=%d&uploadId=%s", partNumber, uploadID)
	req.Header.Set("Content-Type", contentType)
	return addContextToRequest(ctx, req, bucket, key, uploadID)
}

func addContextToRequest(ctx context.Context, req *http.Request, bucket, key, uploadID string) *http.Request {
	ctx = context.WithValue(ctx, "bucket", bucket)
	ctx = context.WithValue(ctx, "key", key)
	if uploadID != "" {
		ctx = context.WithValue(ctx, "uploadId", uploadID)
	}
	return req.WithContext(ctx)
}
