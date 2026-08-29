// Package handlers_test tests the S3 operation handlers.
package handlers_test

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/server/handlers"
)

// TestVersion2EncryptionPutObject verifies that PutObject uses Version2 encryption.
func TestVersion2EncryptionPutObject(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Create plaintext content
	plaintext := []byte("Hello, ARMOR! This is a test file for Version2 encryption.")

	// Create PUT request
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test-key", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the stored metadata has Version 2
	mb.mu.Lock()
	meta, ok := mb.meta["test-bucket/test-key"]
	mb.mu.Unlock()

	if !ok {
		t.Fatal("object metadata not found")
	}

	version := meta["x-amz-meta-armor-version"]
	if version != "2" {
		t.Errorf("expected version 2, got %s", version)
	}

	// Verify the envelope header also has Version 2
	data := mb.objects["test-bucket/test-key"]
	if len(data) < 64 {
		t.Fatalf("encrypted data too short: %d bytes", len(data))
	}

	// Check envelope version (byte 4 in the header)
	envelopeVersion := data[4]
	if envelopeVersion != 0x02 {
		t.Errorf("expected envelope version 0x02, got 0x%02x", envelopeVersion)
	}
}

// TestVersion2EncryptionStreamingPut verifies that streaming PUT uses Version2 encryption.
func TestVersion2EncryptionStreamingPut(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Create plaintext content larger than one block to trigger streaming
	plaintext := make([]byte, 128*1024) // 128KB (2 blocks)
	if _, err := rand.Read(plaintext); err != nil {
		t.Fatalf("failed to generate random plaintext: %v", err)
	}

	// Create PUT request
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/streaming-key", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the stored metadata has Version 2
	mb.mu.Lock()
	meta, ok := mb.meta["test-bucket/streaming-key"]
	mb.mu.Unlock()

	if !ok {
		t.Fatal("object metadata not found")
	}

	version := meta["x-amz-meta-armor-version"]
	if version != "2" {
		t.Errorf("expected version 2, got %s", version)
	}

	// Verify the envelope header also has Version 2
	data := mb.objects["test-bucket/streaming-key"]
	if len(data) < 64 {
		t.Fatalf("encrypted data too short: %d bytes", len(data))
	}

	envelopeVersion := data[4]
	if envelopeVersion != 0x02 {
		t.Errorf("expected envelope version 0x02, got 0x%02x", envelopeVersion)
	}
}

// TestVersion2EncryptionMultipartUpload verifies that multipart upload uses Version2 encryption.
func TestVersion2EncryptionMultipartUpload(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	bucket := "test-bucket"
	key := "multipart-key"
	uploadID := "test-upload-id"

	// Step 1: Create multipart upload
	createReqBody := `<CreateMultipartUploadRequest xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></CreateMultipartUploadRequest>`
	createReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%s?uploads", bucket, key), strings.NewReader(createReqBody))
	createW := httptest.NewRecorder()

	h.HandleRoot(createW, createReq)

	if createW.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: status %d, body %s", createW.Code, createW.Body.String())
	}

	// Parse upload ID from response
	var createResult struct {
		UploadID string `xml:"UploadId"`
		Bucket   string `xml:"Bucket"`
		Key      string `xml:"Key"`
	}
	if err := xml.NewDecoder(createW.Body).Decode(&createResult); err != nil {
		t.Fatalf("failed to parse CreateMultipartUpload result: %v", err)
	}

	uploadID = createResult.UploadID

	// Step 2: Upload part 1
	part1Data := make([]byte, 64*1024) // 64KB (1 block)
	if _, err := rand.Read(part1Data); err != nil {
		t.Fatalf("failed to generate part 1 data: %v", err)
	}

	part1Req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/%s?partNumber=1&uploadId=%s", bucket, key, uploadID), bytes.NewReader(part1Data))
	part1W := httptest.NewRecorder()

	h.HandleRoot(part1W, part1Req)

	if part1W.Code != http.StatusOK {
		t.Fatalf("UploadPart 1 failed: status %d, body %s", part1W.Code, part1W.Body.String())
	}

	// Step 3: Upload part 2
	part2Data := make([]byte, 64*1024) // 64KB (1 block)
	if _, err := rand.Read(part2Data); err != nil {
		t.Fatalf("failed to generate part 2 data: %v", err)
	}

	part2Req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/%s?partNumber=2&uploadId=%s", bucket, key, uploadID), bytes.NewReader(part2Data))
	part2W := httptest.NewRecorder()

	h.HandleRoot(part2W, part2Req)

	if part2W.Code != http.StatusOK {
		t.Fatalf("UploadPart 2 failed: status %d, body %s", part2W.Code, part2W.Body.String())
	}

	// Step 4: Complete multipart upload
	completeReqBody := fmt.Sprintf(`<CompleteMultipartUploadRequest xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
		<Part>
			<PartNumber>1</PartNumber>
			<ETag>%s</ETag>
		</Part>
		<Part>
			<PartNumber>2</PartNumber>
			<ETag>%s</ETag>
		</Part>
	</CompleteMultipartUploadRequest>`, part1W.Header().Get("ETag"), part2W.Header().Get("ETag"))

	completeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%s?uploadId=%s", bucket, key, uploadID), strings.NewReader(completeReqBody))
	completeW := httptest.NewRecorder()

	h.HandleRoot(completeW, completeReq)

	if completeW.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload failed: status %d, body %s", completeW.Code, completeW.Body.String())
	}

	// Step 5: Verify the stored metadata has Version 2
	mb.mu.Lock()
	meta, ok := mb.meta[bucket+"/"+key]
	mb.mu.Unlock()

	if !ok {
		t.Fatal("object metadata not found after CompleteMultipartUpload")
	}

	version := meta["x-amz-meta-armor-version"]
	if version != "2" {
		t.Errorf("expected version 2, got %s", version)
	}

	// Verify multipart flag is set
	multipartFlag := meta["x-amz-meta-armor-multipart"]
	if multipartFlag != "true" {
		t.Errorf("expected multipart flag true, got %s", multipartFlag)
	}
}

// TestNoVersion1ProductionPaths verifies that no production code path can create Version1 objects.
func TestNoVersion1ProductionPaths(t *testing.T) {
	// This test verifies that we cannot produce Version1 objects from production code
	// unless we explicitly use the legacyv1test build tag (which would require a separate build)

	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	testCases := []struct {
		name     string
		setupReq func() *http.Request
	}{
		{
			name: "PutObject",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodPut, "/test-bucket/test-key", bytes.NewReader([]byte("test data")))
			},
		},
		{
			name: "StreamingPut",
			setupReq: func() *http.Request {
				largeData := make([]byte, 128*1024)
				rand.Read(largeData)
				return httptest.NewRequest(http.MethodPut, "/test-bucket/large-key", bytes.NewReader(largeData))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.setupReq()
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()

			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("request failed: status %d, body %s", w.Code, w.Body.String())
			}

			// Extract key from request path
			pathParts := strings.SplitN(req.URL.Path, "/", 3)
			if len(pathParts) < 3 {
				t.Fatal("invalid request path")
			}
			bucket := pathParts[1]
			key := pathParts[2]

			// Verify Version 2 in metadata
			mb.mu.Lock()
			meta, ok := mb.meta[bucket+"/"+key]
			mb.mu.Unlock()

			if !ok {
				t.Fatal("object metadata not found")
			}

			version := meta["x-amz-meta-armor-version"]
			if version != "2" {
				t.Errorf("production path %s produced version %s, expected version 2", tc.name, version)
			}
		})
	}
}
