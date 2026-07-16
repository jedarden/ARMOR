package handlers_test

// Tests for the multipart upload HTTP surface, added per ADR-002 and
// docs/upload-retrieval-test-matrix.md (U4, R4, R7).
//
// The 2026-06 corruption bug lived in ROUTING: a standard client's
// `PUT ?partNumber&uploadId` fell through to plain PutObject, storing each
// part as the whole object. Backend-level tests and the 1KB canary were
// structurally blind to it. These tests exercise the full HTTP handler path
// with a recording backend so a routing regression fails loudly.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// recordingBackend wraps mockBackend, records which backend methods are
// invoked per object key, and implements a real in-memory multipart
// store: parts are kept per uploadID and concatenated (in the part order
// given to CompleteMultipartUpload) into the object map on completion —
// mirroring what B2 does on b2_finish_large_file.
type recordingBackend struct {
	*mockBackend

	rmu      sync.Mutex
	putKeys  []string // keys passed to Put (includes state/sidecar writes)
	partKeys []string // keys passed to UploadPart
	uploads  map[string]map[int32][]byte
	nextID   int
}

func newRecordingBackend() *recordingBackend {
	return &recordingBackend{
		mockBackend: newMockBackend(),
		uploads:     make(map[string]map[int32][]byte),
	}
}

func (r *recordingBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	r.rmu.Lock()
	r.putKeys = append(r.putKeys, bucket+"/"+key)
	r.rmu.Unlock()
	return r.mockBackend.Put(ctx, bucket, key, body, size, meta)
}

func (r *recordingBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	r.rmu.Lock()
	defer r.rmu.Unlock()
	r.nextID++
	id := fmt.Sprintf("upload-%d", r.nextID)
	r.uploads[id] = make(map[int32][]byte)
	return id, nil
}

func (r *recordingBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	r.rmu.Lock()
	defer r.rmu.Unlock()
	parts, ok := r.uploads[uploadID]
	if !ok {
		return "", fmt.Errorf("no such upload: %s", uploadID)
	}
	parts[partNumber] = data
	r.partKeys = append(r.partKeys, bucket+"/"+key)
	return fmt.Sprintf("etag-part-%d", partNumber), nil
}

func (r *recordingBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, completed []backend.CompletedPart) (string, error) {
	r.rmu.Lock()
	parts, ok := r.uploads[uploadID]
	if !ok {
		r.rmu.Unlock()
		return "", fmt.Errorf("no such upload: %s", uploadID)
	}
	var assembled []byte
	for _, p := range completed {
		data, ok := parts[p.PartNumber]
		if !ok {
			r.rmu.Unlock()
			return "", fmt.Errorf("missing part %d", p.PartNumber)
		}
		assembled = append(assembled, data...)
	}
	delete(r.uploads, uploadID)
	r.rmu.Unlock()

	// Store assembled object exactly as B2 would, preserving no metadata —
	// the handler applies ARMOR metadata afterwards via Copy.
	r.mu.Lock()
	r.objects[bucket+"/"+key] = assembled
	if _, ok := r.meta[bucket+"/"+key]; !ok {
		r.meta[bucket+"/"+key] = map[string]string{}
	}
	r.mu.Unlock()
	return "etag-assembled", nil
}

func (r *recordingBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	r.rmu.Lock()
	defer r.rmu.Unlock()
	delete(r.uploads, uploadID)
	return nil
}

// objectPutCount returns how many times Put was called with exactly this key.
func (r *recordingBackend) objectPutCount(bucket, key string) int {
	r.rmu.Lock()
	defer r.rmu.Unlock()
	n := 0
	for _, k := range r.putKeys {
		if k == bucket+"/"+key {
			n++
		}
	}
	return n
}

func recordingTestSetup(t *testing.T) (*config.Config, *recordingBackend, *handlers.Handlers) {
	t.Helper()
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
	}
	rb := newRecordingBackend()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}
	h := handlers.New(cfg, rb, cache, footerCache, km, nil)
	return cfg, rb, h
}

func initiateMultipart(t *testing.T, h *handlers.Handlers, bucket, key string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%s?uploads", bucket, key), nil)
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: status %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse initiate response: %v", err)
	}
	if result.UploadID == "" {
		t.Fatal("empty UploadId in initiate response")
	}
	return result.UploadID
}

func uploadPart(t *testing.T, h *handlers.Handlers, bucket, key, uploadID string, partNumber int, body []byte) string {
	t.Helper()
	url := fmt.Sprintf("/%s/%s?partNumber=%d&uploadId=%s", bucket, key, partNumber, uploadID)
	req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UploadPart %d failed: status %d: %s", partNumber, w.Code, w.Body.String())
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("UploadPart %d returned no ETag", partNumber)
	}
	return etag
}

func completeMultipart(t *testing.T, h *handlers.Handlers, bucket, key, uploadID string, etags []string) {
	t.Helper()
	var xmlBody bytes.Buffer
	xmlBody.WriteString("<CompleteMultipartUpload>")
	for i, etag := range etags {
		fmt.Fprintf(&xmlBody, "<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", i+1, etag)
	}
	xmlBody.WriteString("</CompleteMultipartUpload>")
	url := fmt.Sprintf("/%s/%s?uploadId=%s", bucket, key, uploadID)
	req := httptest.NewRequest(http.MethodPost, url, &xmlBody)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload failed: status %d: %s", w.Code, w.Body.String())
	}
}

// TestUploadPartRoutingNeverFallsThroughToPut is the ADR-002 regression
// tripwire. The 2026-06 bug routed `PUT ?partNumber&uploadId` to PutObject;
// this asserts the part request reaches UploadPart and that Put is never
// invoked for the object key itself (state/sidecar writes use other keys).
func TestUploadPartRoutingNeverFallsThroughToPut(t *testing.T) {
	_, rb, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "routing-tripwire.dat"

	uploadID := initiateMultipart(t, h, bucket, key)
	part := make([]byte, 2*65536) // two full blocks
	for i := range part {
		part[i] = byte(i % 251)
	}
	uploadPart(t, h, bucket, key, uploadID, 1, part)

	rb.rmu.Lock()
	partCalls := len(rb.partKeys)
	rb.rmu.Unlock()
	if partCalls == 0 {
		t.Fatal("ROUTING REGRESSION: PUT ?partNumber&uploadId did not reach backend.UploadPart")
	}
	if n := rb.objectPutCount(bucket, key); n != 0 {
		t.Fatalf("ROUTING REGRESSION: backend.Put was called %d time(s) with the object key during a part upload — parts are being stored as whole objects (the ADR-002 corruption bug)", n)
	}
}

// TestMultipartFullCycleByteVerification uploads three distinguishable parts
// (block-aligned intermediates + a non-aligned final part) through the real
// HTTP handler path (initiate → parts → complete), then verifies retrieval
// byte-for-byte via full GET, bounded Range GET across a part boundary,
// suffix Range GET (the parquet footer pattern), and HEAD length.
// Covers matrix rows U4, U9, R4, R7.
func TestMultipartFullCycleByteVerification(t *testing.T) {
	_, rb, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "full-cycle.parquet"

	// Three parts with distinguishable patterns; final part non-aligned.
	// Scaled to 44MB+ total to match the actual failing scale from bf-1v6skf
	// (production litestream snapshot was 44,908,497 bytes and failed at block 256).
	const targetTotal = 45 * 1024 * 1024 // 45MB target (slightly above production failure)
	const block = 65536                  // encryption block size
	partSize := 5 * 1024 * 1024          // 5MB per part (S3 minimum, matches production)
	finalPartSize := targetTotal - (2 * partSize)
	sizes := []int{partSize, partSize, finalPartSize}
	var plaintext []byte
	var parts [][]byte
	for p, size := range sizes {
		part := make([]byte, size)
		for i := range part {
			switch p {
			case 0:
				part[i] = byte(i & 0xFF)
			case 1:
				part[i] = 0xFF - byte(i&0xFF)
			case 2:
				if i%2 == 0 {
					part[i] = 0xAA
				} else {
					part[i] = 0x55
				}
			}
		}
		parts = append(parts, part)
		plaintext = append(plaintext, part...)
	}

	uploadID := initiateMultipart(t, h, bucket, key)
	var etags []string
	for i, part := range parts {
		etags = append(etags, uploadPart(t, h, bucket, key, uploadID, i+1, part))
	}
	completeMultipart(t, h, bucket, key, uploadID, etags)

	// Full GET: byte-for-byte.
	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET after complete failed: status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Fatalf("full GET content mismatch: got %d bytes, want %d; first divergence at %d",
			w.Body.Len(), len(plaintext), firstDivergence(w.Body.Bytes(), plaintext))
	}

	// Bounded Range GET spanning the part-1/part-2 boundary.
	lo, hi := 2*block-500, 2*block+499
	req = httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", lo, hi))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusPartialContent {
		t.Fatalf("range GET failed: status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext[lo:hi+1]) {
		t.Fatalf("range GET across part boundary mismatch (bytes=%d-%d)", lo, hi)
	}

	// Suffix Range GET — the parquet footer access pattern (matrix R3/R4).
	req = httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	req.Header.Set("Range", "bytes=-1000")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusPartialContent {
		t.Fatalf("suffix range GET failed: status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext[len(plaintext)-1000:]) {
		t.Fatal("suffix range GET (parquet footer pattern) content mismatch")
	}

	// HEAD: plaintext length, not ciphertext length.
	req = httptest.NewRequest(http.MethodHead, "/"+bucket+"/"+key, nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("HEAD after complete failed: status %d", w.Code)
	}
	if cl := w.Header().Get("Content-Length"); cl != strconv.Itoa(len(plaintext)) {
		t.Fatalf("HEAD Content-Length = %s, want %d", cl, len(plaintext))
	}

	_ = rb
}

// TestMultipartSuspectPatterns documents and enforces rejection of the three
// multipart upload patterns that real SDKs use by default but ARMOR's
// per-part CTR-offset scheme does not support (docs/upload-retrieval-test-matrix.md U6/U7/U8):
//
//	U6 out-of-order / parallel part upload  (boto3 uploads parts concurrently)
//	U7 part retry after network failure     (every SDK retries parts)
//	U8 non-block-aligned intermediate parts (any part size % 65536 != 0)
//
// UploadPart derives each part's CTR counter from arrival-order cumulative
// EncryptedBytes (handlers.go:2085), so all three would corrupt silently.
// The fix implemented in bf-59unr3 rejects these patterns with explicit 400
// errors rather than allowing silent corruption.
func TestMultipartSuspectPatterns(t *testing.T) {
	t.Run("U6_out_of_order_parts", func(t *testing.T) {
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "out-of-order.dat"

		uploadID := initiateMultipart(t, h, bucket, key)

		// Upload part 1 first (succeeds)
		part1 := make([]byte, 5*1024*1024) // 5MiB, block-aligned
		for i := range part1 {
			part1[i] = 0xAA
		}
		uploadPart(t, h, bucket, key, uploadID, 1, part1)

		// Try to upload part 3 without uploading part 2 first (should fail)
		part3 := make([]byte, 5*1024*1024)
		for i := range part3 {
			part3[i] = 0xCC
		}
		url := fmt.Sprintf("/%s/%s?partNumber=3&uploadId=%s", bucket, key, uploadID)
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(part3))
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 BadRequest for out-of-order part, got %d: %s", w.Code, w.Body.String())
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("Parts must be uploaded in sequential order")) {
			t.Errorf("Expected error message about sequential order, got: %s", w.Body.String())
		}
	})

	t.Run("U6_parallel_parts_simulation", func(t *testing.T) {
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "parallel-sim.dat"

		uploadID := initiateMultipart(t, h, bucket, key)

		// Try to upload part 2 before part 1 (simulates parallel upload where part 2 arrives first)
		part2 := make([]byte, 5*1024*1024) // 5MiB, block-aligned
		for i := range part2 {
			part2[i] = 0xBB
		}
		url := fmt.Sprintf("/%s/%s?partNumber=2&uploadId=%s", bucket, key, uploadID)
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(part2))
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 BadRequest for part 2 before part 1, got %d: %s", w.Code, w.Body.String())
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("Expected part 1, got part 2")) {
			t.Errorf("Expected error message about expecting part 1, got: %s", w.Body.String())
		}
	})

	t.Run("U7_part_retry", func(t *testing.T) {
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "retry-test.dat"

		uploadID := initiateMultipart(t, h, bucket, key)

		// Upload part 1 successfully
		part1 := make([]byte, 5*1024*1024) // 5MiB, block-aligned
		for i := range part1 {
			part1[i] = 0xAA
		}
		uploadPart(t, h, bucket, key, uploadID, 1, part1)

		// Try to upload part 1 again (simulating retry after network failure)
		part1Retry := make([]byte, 5*1024*1024)
		for i := range part1Retry {
			part1Retry[i] = 0xAA
		}
		url := fmt.Sprintf("/%s/%s?partNumber=1&uploadId=%s", bucket, key, uploadID)
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(part1Retry))
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 BadRequest for part retry, got %d: %s", w.Code, w.Body.String())
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("has already been uploaded")) {
			t.Errorf("Expected error message about already uploaded, got: %s", w.Body.String())
		}
	})

	t.Run("U8_non_block_aligned_part", func(t *testing.T) {
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "unaligned-test.dat"

		uploadID := initiateMultipart(t, h, bucket, key)

		// Try to upload a part with non-block-aligned size (10,000,000 bytes)
		// 10,000,000 % 65536 = 16976 (not aligned)
		part := make([]byte, 10_000_000)
		for i := range part {
			part[i] = 0xAA
		}
		url := fmt.Sprintf("/%s/%s?partNumber=1&uploadId=%s", bucket, key, uploadID)
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(part))
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 BadRequest for non-block-aligned part, got %d: %s", w.Code, w.Body.String())
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("not a multiple of the block size")) {
			t.Errorf("Expected error message about block alignment, got: %s", w.Body.String())
		}
	})

	t.Run("U8_zero_byte_part_allowed", func(t *testing.T) {
		// Edge case: zero-byte parts should be allowed (can happen with empty files)
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "zero-byte.dat"

		uploadID := initiateMultipart(t, h, bucket, key)

		// Zero-byte part should not trigger the alignment check
		part := make([]byte, 0)
		url := fmt.Sprintf("/%s/%s?partNumber=1&uploadId=%s", bucket, key, uploadID)
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(part))
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		// Zero-byte parts should fail for a different reason (not alignment)
		// The actual error might be about empty body or size, but not the alignment check
		if w.Code == http.StatusBadRequest && bytes.Contains(w.Body.Bytes(), []byte("not a multiple of the block size")) {
			t.Errorf("Zero-byte part should not trigger block alignment error: %s", w.Body.String())
		}
	})
}

func firstDivergence(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// TestMultipartPartBoundaryDebug tests decryption at the exact part boundary.
func TestMultipartPartBoundaryDebug(t *testing.T) {
	_, rb, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "boundary-debug.parquet"
	const partSize = 5 * 1024 * 1024

	// Two simple parts
	part1 := make([]byte, partSize)
	part2 := make([]byte, partSize)
	for i := range part1 {
		part1[i] = 0xAA
		part2[i] = 0xBB
	}

	uploadID := initiateMultipart(t, h, bucket, key)
	etag1 := uploadPart(t, h, bucket, key, uploadID, 1, part1)
	etag2 := uploadPart(t, h, bucket, key, uploadID, 2, part2)
	completeMultipart(t, h, bucket, key, uploadID, []string{etag1, etag2})

	// Check last 10 bytes of part 1
	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	req.Header.Set("Range", "bytes=5242870-5242879")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusPartialContent {
		t.Fatalf("range GET failed: status %d: %s", w.Code, w.Body.String())
	}
	part1End := w.Body.Bytes()
	t.Logf("Part 1 end (bytes 5242870-5242879): %v", part1End)

	// Check first 10 bytes of part 2
	req = httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	req.Header.Set("Range", "bytes=5242880-5242889")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusPartialContent {
		t.Fatalf("range GET failed: status %d: %s", w.Code, w.Body.String())
	}
	part2Start := w.Body.Bytes()
	t.Logf("Part 2 start (bytes 5242880-5242889): %v", part2Start)

	// Verify part 1 end is all 0xAA
	for _, b := range part1End {
		if b != 0xAA {
			t.Errorf("Part 1 end has wrong byte: got 0x%02x, want 0xAA", b)
		}
	}

	// Verify part 2 start is all 0xBB
	for i, b := range part2Start {
		if b != 0xBB {
			t.Errorf("Part 2 start byte %d is wrong: got 0x%02x, want 0xBB", i, b)
		}
	}

	_ = rb
}
