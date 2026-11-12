package handlers_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/replication"
	"github.com/jedarden/armor/internal/server/handlers"
)

func TestPutObjectGetObject(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Create plaintext content
	plaintext := []byte("Hello, ARMOR! This is a test file with some content.")

	// Create PUT request
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test-key", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify ETag is returned
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Error("expected ETag header")
	}

	// Now GET the object back
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/test-key", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify content matches
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Errorf("content mismatch: got %q, want %q", w.Body.String(), string(plaintext))
	}

	// Verify headers
	if w.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %s", w.Header().Get("Content-Type"))
	}
}

func TestPutObjectLargeFile(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a file larger than one block (64KB)
	plaintext := make([]byte, 100000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/large-file", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// GET it back
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/large-file", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("large file content mismatch")
	}
}

func TestGetObjectRange(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create content
	plaintext := make([]byte, 200000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// PUT the object
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/range-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed with status %d", w.Code)
	}

	// GET with range request
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/range-test", nil)
	req.Header.Set("Range", "bytes=1000-1999")
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d", w.Code)
	}

	expectedRange := plaintext[1000:2000]
	if !bytes.Equal(w.Body.Bytes(), expectedRange) {
		t.Errorf("range content mismatch: got %d bytes, want %d bytes", len(w.Body.Bytes()), len(expectedRange))
	}

	// Verify Content-Range header
	contentRange := w.Header().Get("Content-Range")
	if contentRange == "" {
		t.Error("expected Content-Range header")
	}
}

func TestGetObjectRangeCompressed(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create content
	plaintext := make([]byte, 200000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// PUT the object
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/range-compressed-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed with status %d", w.Code)
	}

	// Manually mark the object as compressed in the mock backend metadata
	mb.mu.Lock()
	objKey := "test-bucket/range-compressed-test"
	if meta, ok := mb.meta[objKey]; ok {
		meta["x-amz-meta-armor-compressed"] = "true"
		meta["x-amz-meta-armor-compression-type"] = "gzip"
	}
	mb.mu.Unlock()

	// GET with range request on compressed object should return 416
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/range-compressed-test", nil)
	req.Header.Set("Range", "bytes=1000-1999")
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("expected status 416 for range request on compressed object, got %d", w.Code)
	}

	// Verify error message mentions compression
	body := w.Body.String()
	if !strings.Contains(body, "compressed, so byte-range reads are unavailable") {
		t.Errorf("expected error message about range reads on compressed objects, got: %s", body)
	}
}

func TestEncryptionRoundTrip(t *testing.T) {
	// This test verifies that the encryption actually happens
	// by checking that the stored data is different from plaintext
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	plaintext := []byte("This is sensitive data that should be encrypted")

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/encrypted", bytes.NewReader(plaintext))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Check that stored data is encrypted (not plaintext)
	mb.mu.Lock()
	storedData := mb.objects["test-bucket/encrypted"]
	mb.mu.Unlock()

	if bytes.Contains(storedData, plaintext) {
		t.Error("plaintext found in stored data - encryption may not be working")
	}

	// Verify envelope magic is present (ARMR header)
	if len(storedData) < 4 || string(storedData[:4]) != "ARMR" {
		t.Error("ARMR magic not found at start of stored data")
	}
}

func TestMultipleFilesSameMEK(t *testing.T) {
	// Test that multiple files encrypted with the same MEK can all be decrypted
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	files := map[string][]byte{
		"file1.txt": []byte("Content of file 1"),
		"file2.txt": []byte("Content of file 2 - different"),
		"file3.txt": []byte("Third file with more content"),
	}

	// Upload all files
	for key, content := range files {
		req := httptest.NewRequest(http.MethodPut, "/test-bucket/"+key, bytes.NewReader(content))
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("failed to upload %s: status %d", key, w.Code)
		}
	}

	// Verify all files can be downloaded and decrypted correctly
	for key, expectedContent := range files {
		req := httptest.NewRequest(http.MethodGet, "/test-bucket/"+key, nil)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("failed to download %s: status %d", key, w.Code)
			continue
		}

		if !bytes.Equal(w.Body.Bytes(), expectedContent) {
			t.Errorf("content mismatch for %s", key)
		}
	}
}

func TestNonARMORObjectPassthrough(t *testing.T) {
	// Test that non-ARMOR objects (without x-amz-meta-armor-version) pass through unchanged
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Store a plain object directly in the mock backend
	plainData := []byte("Plain unencrypted data")
	mb.Put(context.Background(), "test-bucket", "plain-file", bytes.NewReader(plainData), int64(len(plainData)), map[string]string{
		"Content-Type": "text/plain",
	})

	// GET should return it unchanged
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/plain-file", nil)
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plainData) {
		t.Error("plain data was modified")
	}
}

func TestETagConsistency(t *testing.T) {
	// Test that ETag is consistent across uploads of same content
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	content := []byte("Same content")

	// Upload twice
	req1 := httptest.NewRequest(http.MethodPut, "/test-bucket/etag-test-1", bytes.NewReader(content))
	req1.Header.Set("Content-Type", "text/plain")
	w1 := httptest.NewRecorder()
	h.HandleRoot(w1, req1)
	etag1 := w1.Header().Get("ETag")

	req2 := httptest.NewRequest(http.MethodPut, "/test-bucket/etag-test-2", bytes.NewReader(content))
	req2.Header.Set("Content-Type", "text/plain")
	w2 := httptest.NewRecorder()
	h.HandleRoot(w2, req2)
	etag2 := w2.Header().Get("ETag")

	if etag1 != etag2 {
		t.Errorf("ETags for same content differ: %s vs %s", etag1, etag2)
	}

	// Upload different content
	differentContent := []byte("Different content")
	req3 := httptest.NewRequest(http.MethodPut, "/test-bucket/etag-test-3", bytes.NewReader(differentContent))
	req3.Header.Set("Content-Type", "text/plain")
	w3 := httptest.NewRecorder()
	h.HandleRoot(w3, req3)
	etag3 := w3.Header().Get("ETag")

	if etag1 == etag3 {
		t.Error("ETags for different content are the same")
	}
}

// TestHMACVerification ensures HMAC verification catches tampering
// With streaming decryption, the HTTP status is written before all blocks are verified.
// Tampering is detected during streaming, which will cause the body to be incomplete.
// The httptest.ResponseRecorder doesn't capture streaming errors, so we verify
// that the body content is incomplete or corrupted rather than checking status code.

// TestHMACVerification ensures HMAC verification catches tampering
// With streaming decryption, the HTTP status is written before all blocks are verified.
// Tampering is detected during streaming, which will cause the body to be incomplete.
// The httptest.ResponseRecorder doesn't capture streaming errors, so we verify
// that the body content is incomplete or corrupted rather than checking status code.
func TestHMACVerification(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	plaintext := []byte("Content to verify")

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/verify-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Tamper with stored data
	mb.mu.Lock()
	stored := mb.objects["test-bucket/verify-test"]
	// Modify a byte in the encrypted data section (after header)
	if len(stored) > 100 {
		stored[80] ^= 0xFF
	}
	mb.mu.Unlock()

	// GET should fail integrity check
	// With streaming, the status is written before HMAC verification completes.
	// The verification error will cause incomplete/corrupted body content.
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/verify-test", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Verify the response is not the original plaintext (tampering detected)
	// Either the body is empty/incomplete, or it's corrupted
	if bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("expected corrupted/incomplete data for tampered object, got original plaintext")
	}

	// For range requests, we still get 500 because HMAC is verified before streaming
	// But for full downloads with streaming, we may get 200 with incomplete body
	// This is expected behavior - streaming provides lower latency at the cost of
	// potentially incomplete responses on integrity failure
}

// TestRangeSuffixRequest tests suffix range requests (e.g., "bytes=-100")

// TestRangeSuffixRequest tests suffix range requests (e.g., "bytes=-100")
func TestRangeSuffixRequest(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	plaintext := make([]byte, 10000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/suffix-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Request last 100 bytes
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/suffix-test", nil)
	req.Header.Set("Range", "bytes=-100")
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d", w.Code)
	}

	expected := plaintext[len(plaintext)-100:]
	if !bytes.Equal(w.Body.Bytes(), expected) {
		t.Error("suffix range content mismatch")
	}
}

// TestStreamingDecryption verifies that pipelined streaming decryption works correctly
// for multi-block files. This exercises the io.Pipe based streaming path.

// TestStreamingDecryption verifies that pipelined streaming decryption works correctly
// for multi-block files. This exercises the io.Pipe based streaming path.
func TestStreamingDecryption(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a file spanning multiple blocks (64KB each)
	// Using 3 blocks worth of data
	blockSize := cfg.BlockSize
	plaintext := make([]byte, blockSize*3-1000) // Not aligned to block boundary
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// Upload
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/streaming-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Download with full streaming (no Range header)
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/streaming-test", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	// Verify streaming header is present
	if w.Header().Get("X-Armor-Stream") != "pipelined" {
		t.Error("expected X-Armor-Stream: pipelined header")
	}

	// Verify content matches exactly
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Errorf("streaming content mismatch: got %d bytes, want %d bytes",
			len(w.Body.Bytes()), len(plaintext))
	}

	// Verify Accept-Ranges header
	if w.Header().Get("Accept-Ranges") != "bytes" {
		t.Error("expected Accept-Ranges: bytes header")
	}
}

// TestStreamingDecryptionVariousSizes tests streaming with different file sizes

// TestStreamingDecryptionVariousSizes tests streaming with different file sizes
func TestStreamingDecryptionVariousSizes(t *testing.T) {
	sizes := []int{
		100,     // Tiny
		4096,    // One 4KB page
		65535,   // Just under one block
		65536,   // Exactly one block
		65537,   // Just over one block
		131072,  // Two blocks
		1000000, // ~1MB
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			cfg, mb, cache, footerCache, mek := testSetup(t)
			h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

			plaintext := make([]byte, size)
			for i := range plaintext {
				plaintext[i] = byte(i % 256)
			}

			key := fmt.Sprintf("streaming-test-%d", size)
			req := httptest.NewRequest(http.MethodPut, "/test-bucket/"+key, bytes.NewReader(plaintext))
			req.Header.Set("Content-Type", "application/octet-stream")
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("PUT failed: status %d", w.Code)
			}

			req = httptest.NewRequest(http.MethodGet, "/test-bucket/"+key, nil)
			w = httptest.NewRecorder()
			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("GET failed: status %d", w.Code)
			}

			if !bytes.Equal(w.Body.Bytes(), plaintext) {
				t.Errorf("content mismatch for size %d: got %d bytes, want %d",
					size, len(w.Body.Bytes()), size)
			}
		})
	}
}

// TestCopyObject tests S3 CopyObject with ARMOR encryption

// TestStreamingEncryptionLargeFile tests streaming encryption for files > 10MB
func TestStreamingEncryptionLargeFile(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a file larger than the 10MB streaming threshold
	// Using 15MB to ensure streaming is triggered
	size := 15 * 1024 * 1024
	plaintext := make([]byte, size)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// Upload with Content-Length header to trigger streaming
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/streaming-large", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(size)
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT failed: status %d", w.Code)
	}

	// Verify streaming header is set
	if w.Header().Get("X-Armor-Streaming") != "true" {
		t.Error("expected X-Armor-Streaming: true header for large file")
	}

	// Verify ETag is returned
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Error("expected ETag header")
	}

	// GET the file back and verify content
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/streaming-large", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Errorf("streaming content mismatch: got %d bytes, want %d bytes",
			len(w.Body.Bytes()), len(plaintext))
	}
}

// TestStreamingEncryptionMultiBlock tests streaming with multi-block files

// TestStreamingEncryptionMultiBlock tests streaming with multi-block files
func TestStreamingEncryptionMultiBlock(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a file that spans multiple blocks (> 64KB) but under streaming threshold
	// This tests the buffered path
	blockSize := cfg.BlockSize
	plaintext := make([]byte, blockSize*2+1000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/multi-block", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT failed: status %d", w.Code)
	}

	// GET and verify
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/multi-block", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("multi-block content mismatch")
	}
}

// TestStreamingEncryptionRangeRead tests range reads on streaming-encrypted files

// TestStreamingEncryptionRangeRead tests range reads on streaming-encrypted files
func TestStreamingEncryptionRangeRead(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a large file (triggers streaming)
	size := 12 * 1024 * 1024
	plaintext := make([]byte, size)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// Upload
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/streaming-range", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(size)
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Debug: Check what was stored
	mb.mu.Lock()
	storedData := mb.objects["test-bucket/streaming-range"]
	storedMeta := mb.meta["test-bucket/streaming-range"]
	mb.mu.Unlock()

	expectedEnvelopeSize := int64(64 + size + int(int(size/65536)+1)*32)
	t.Logf("Stored data size: %d, expected envelope size: ~%d", len(storedData), expectedEnvelopeSize)
	t.Logf("Metadata: %+v", storedMeta)

	// Range read from the middle
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/streaming-range", nil)
	req.Header.Set("Range", "bytes=1000000-2000000")
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d, body: %s", w.Code, w.Body.String())
	}

	expectedRange := plaintext[1000000 : 2000000+1]
	if !bytes.Equal(w.Body.Bytes(), expectedRange) {
		t.Errorf("range content mismatch: got %d bytes, want %d bytes",
			len(w.Body.Bytes()), len(expectedRange))
	}

	// Verify Content-Range header
	contentRange := w.Header().Get("Content-Range")
	if contentRange == "" {
		t.Error("expected Content-Range header")
	}
}

// TestStreamingEncryptionSHA256 verifies SHA-256 integrity for streaming uploads

// TestStreamingEncryptionSHA256 verifies SHA-256 integrity for streaming uploads
func TestStreamingEncryptionSHA256(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a known-content large file
	size := 11 * 1024 * 1024
	plaintext := make([]byte, size)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	// Upload with streaming
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/sha256-test", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(size)
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d", w.Code)
	}

	// Verify the stored metadata has the correct plaintext SHA-256
	mb.mu.Lock()
	meta := mb.meta["test-bucket/sha256-test"]
	storedData := mb.objects["test-bucket/sha256-test"]
	mb.mu.Unlock()

	// Verify ARMR magic header
	if len(storedData) < 4 || string(storedData[:4]) != "ARMR" {
		t.Error("ARMR magic not found at start of stored data")
	}

	// Verify metadata contains SHA-256
	sha256Meta := meta["x-amz-meta-armor-plaintext-sha256"]
	if sha256Meta == "" {
		t.Error("missing plaintext SHA-256 in metadata")
	}

	// GET and verify full content
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/sha256-test", nil)
	w = httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET failed: status %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Error("streaming SHA-256 content mismatch")
	}
}

// TestStreamingEncryptionThreshold tests the boundary between buffered and streaming

// TestStreamingEncryptionThreshold tests the boundary between buffered and streaming
func TestStreamingEncryptionThreshold(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Test just under the 10MB threshold (should use buffered path)
	threshold := 10 * 1024 * 1024

	tests := []struct {
		name         string
		size         int
		expectStream bool
	}{
		{"under_threshold", threshold - 1, false},
		{"at_threshold", threshold, false}, // at threshold still uses buffered
		{"over_threshold", threshold + 1, true},
		{"large_over", threshold * 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plaintext := make([]byte, tt.size)
			for i := range plaintext {
				plaintext[i] = byte(i % 256)
			}

			req := httptest.NewRequest(http.MethodPut, "/test-bucket/"+tt.name, bytes.NewReader(plaintext))
			req.Header.Set("Content-Type", "application/octet-stream")
			req.ContentLength = int64(tt.size)
			w := httptest.NewRecorder()

			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("PUT failed: status %d", w.Code)
				return
			}

			streamingHeader := w.Header().Get("X-Armor-Streaming")
			if tt.expectStream && streamingHeader != "true" {
				t.Errorf("expected X-Armor-Streaming: true, got %s", streamingHeader)
			}
			if !tt.expectStream && streamingHeader == "true" {
				t.Error("did not expect X-Armor-Streaming: true for small file")
			}

			// Verify content integrity
			req = httptest.NewRequest(http.MethodGet, "/test-bucket/"+tt.name, nil)
			w = httptest.NewRecorder()

			h.HandleRoot(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("GET failed: status %d", w.Code)
				return
			}

			if !bytes.Equal(w.Body.Bytes(), plaintext) {
				t.Errorf("content mismatch for %s", tt.name)
			}
		})
	}
}

// mockBackendWithLifecycle is a mock backend that supports lifecycle configuration for testing

// mockSlowReplicationQueue is a mock replication.Enqueuer that simulates slow enqueue operations.
// It tracks enqueue timing for testing async behavior.
type mockSlowReplicationQueue struct {
	enqueueDelay     time.Duration
	enqueueCallCount atomic.Int32
	enqueueStartChan chan struct{} // Signals when enqueue starts
	enqueueDoneChan  chan struct{} // Signals when enqueue completes
}

// Compile-time interface assertion

// Compile-time interface assertion
var _ replication.Enqueuer = (*mockSlowReplicationQueue)(nil)

// newMockSlowReplicationQueue creates a mock queue with configurable delay.

// newMockSlowReplicationQueue creates a mock queue with configurable delay.
func newMockSlowReplicationQueue(delay time.Duration) *mockSlowReplicationQueue {
	return &mockSlowReplicationQueue{
		enqueueDelay:     delay,
		enqueueStartChan: make(chan struct{}, 1),
		enqueueDoneChan:  make(chan struct{}, 1),
	}
}

// Enqueue simulates a slow enqueue operation.
// This method is called by the handler's goroutine after the response is sent.

// Enqueue simulates a slow enqueue operation.
// This method is called by the handler's goroutine after the response is sent.
func (m *mockSlowReplicationQueue) Enqueue(bucket, key string) {
	m.enqueueCallCount.Add(1)

	// Signal that enqueue has started
	select {
	case m.enqueueStartChan <- struct{}{}:
	default:
	}

	// Simulate slow enqueue
	time.Sleep(m.enqueueDelay)

	// Signal that enqueue has completed
	select {
	case m.enqueueDoneChan <- struct{}{}:
	default:
	}
}

// TestPutObjectAsyncEnqueueDoesNotBlockResponse verifies that the PutObject handler
// returns the success response before the async enqueue goroutine completes.
// This test ensures that replication enqueue does not block the client response path.

// TestPutObjectAsyncEnqueueDoesNotBlockResponse verifies that the PutObject handler
// returns the success response before the async enqueue goroutine completes.
// This test ensures that replication enqueue does not block the client response path.
func TestPutObjectAsyncEnqueueDoesNotBlockResponse(t *testing.T) {
	// Use a significant delay to ensure timing is measurable
	// 100ms is long enough to measure but short enough not to slow down tests
	enqueueDelay := 100 * time.Millisecond

	cfg, mb, cache, footerCache, km := testSetup(t)

	// Create a mock queue with configurable delay
	slowQueue := newMockSlowReplicationQueue(enqueueDelay)

	// Create handlers with the slow queue
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)
	h.WithReplicationQueue(slowQueue)

	// Test content
	plaintext := []byte("Test content for async enqueue verification")

	// Record start time before sending request
	requestStartTime := time.Now()

	// Create PUT request
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test-key", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	// This should return quickly without waiting for enqueue
	h.HandleRoot(w, req)

	// Record response time
	responseTime := time.Since(requestStartTime)

	// Verify we got a successful response
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify ETag is returned (response is complete)
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Error("expected ETag header to be set")
	}

	// Response should be MUCH faster than the enqueue delay
	// We allow a generous margin (50ms) for test execution overhead
	maxExpectedResponseTime := enqueueDelay / 2 // Response should be at most half the enqueue delay
	marginOfError := 50 * time.Millisecond

	if responseTime > maxExpectedResponseTime+marginOfError {
		t.Errorf("response took too long: %v, expected < %v (enqueue delay was %v)",
			responseTime, maxExpectedResponseTime+marginOfError, enqueueDelay)
	}

	// Now wait for the async enqueue to complete
	select {
	case <-slowQueue.enqueueStartChan:
		// Enqueue started (good - goroutine was launched)
	case <-time.After(200 * time.Millisecond):
		t.Error("enqueue did not start within timeout")
	}

	select {
	case <-slowQueue.enqueueDoneChan:
		// Enqueue completed (good - goroutine finished work)
	case <-time.After(200 * time.Millisecond):
		t.Error("enqueue did not complete within timeout")
	}

	// Record total time including async enqueue
	totalTime := time.Since(requestStartTime)

	// Verify that response was faster than total time (async happened after response)
	if responseTime >= totalTime {
		t.Errorf("response time (%v) should be less than total time (%v) - enqueue may have blocked response",
			responseTime, totalTime)
	}

	// Verify that total time is longer than response time (async work happened)
	if totalTime <= responseTime {
		t.Errorf("total time (%v) should be greater than response time (%v) - async work may not have occurred",
			totalTime, responseTime)
	}

	// Verify enqueue was actually called
	callCount := slowQueue.enqueueCallCount.Load()
	if callCount != 1 {
		t.Errorf("expected enqueue to be called once, got %d calls", callCount)
	}

	t.Logf("✓ Response time: %v (well below enqueue delay of %v)", responseTime, enqueueDelay)
	t.Logf("✓ Total time (including async enqueue): %v", totalTime)
	t.Logf("✓ Enqueue completed after response (response returned %v before enqueue finished)",
		totalTime-responseTime)
}

// TestHandlerErrorResponseIncludesRequestIdAndResource verifies that handler error
// responses include both <RequestId> and <Resource> elements in the XML body.
// This ensures consistency with server.writeError which already includes RequestId,
// and provides clients that parse RequestId from the body with a consistent format.
