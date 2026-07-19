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

	// ADR-005 uniform-part-size contract: the first part pins P and every part
	// except the highest-numbered must equal P exactly. Eight full 5MiB parts
	// (P) plus a short final part keeps the ~44MB production scale from
	// bf-1v6skf (production litestream snapshot was 44,908,497 bytes and failed
	// at block 256) while staying contract-valid. (The pre-ADR-005 version used
	// a final part LARGER than P, which now correctly contradicts and poisons.)
	const block = 65536              // encryption block size
	partSize := 5 * 1024 * 1024      // 5MiB regular part (S3/B2 minimum, matches production)
	finalPartSize := 4 * 1024 * 1024 // short final part (< P), block-aligned
	sizes := []int{partSize, partSize, partSize, partSize, partSize, partSize, partSize, partSize, finalPartSize}
	var plaintext []byte
	var parts [][]byte
	for p, size := range sizes {
		part := make([]byte, size)
		for i := range part {
			switch p % 3 { // three distinguishable byte patterns, cycled across parts
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

// TestMultipartSuspectPatterns documents the ADR-005 behavior for the upload
// patterns real SDKs use by default (docs/upload-retrieval-test-matrix.md
// U6/U7/U8). ADR-005 reverses the old ADR-003 §4 interim behavior:
//
//	U6 out-of-order / parallel part upload  — now SUPPORTED (offset is a
//	    function of part number alone, not arrival order).
//	U7 part retry after network failure     — now IDEMPOTENT for same-size
//	    retries (CTR is deterministic; same N → same offset → same ciphertext).
//	U8 non-block-aligned part               — still REJECTED (the (N-1)*P/
//	    BlockSize offset formula requires every part on a block boundary).
//
// The contradiction cases (a part larger than P, two short parts, a retry with
// a different size) poison the upload — those are covered in
// multipart_out_of_order_test.go.
func TestMultipartSuspectPatterns(t *testing.T) {
	t.Run("U6_out_of_order_parts_supported", func(t *testing.T) {
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "out-of-order.dat"

		uploadID := initiateMultipart(t, h, bucket, key)

		// Upload parts 1, 3, 2 in that order. ADR-005 derives each part's CTR
		// offset from its part number, so out-of-order arrival is fine.
		mk := func(b byte) []byte {
			p := make([]byte, 5*1024*1024) // 5MiB, block-aligned
			for i := range p {
				p[i] = b
			}
			return p
		}
		uploadPart(t, h, bucket, key, uploadID, 1, mk(0xAA))
		uploadPart(t, h, bucket, key, uploadID, 3, mk(0xCC))
		uploadPart(t, h, bucket, key, uploadID, 2, mk(0xBB))

		// All three must have been accepted (uploadPart fatals on non-200).
		// Complete must succeed and decrypt byte-for-byte — proving the
		// out-of-order offsets were each correct.
		var plaintext []byte
		for _, b := range []byte{0xAA, 0xBB, 0xCC} {
			plaintext = append(plaintext, mk(b)...)
		}
		completeMultipart(t, h, bucket, key, uploadID, []string{"etag-part-1", "etag-part-2", "etag-part-3"})

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET after out-of-order complete failed: status %d: %s", w.Code, w.Body.String())
		}
		if !bytes.Equal(w.Body.Bytes(), plaintext) {
			t.Fatalf("out-of-order round-trip mismatch: got %d bytes, want %d", w.Body.Len(), len(plaintext))
		}
	})

	t.Run("U6_part_2_before_part_1_supported", func(t *testing.T) {
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "parallel-sim.dat"

		uploadID := initiateMultipart(t, h, bucket, key)

		// Part 2 arrives before part 1 (a concurrent uploader whose part 2
		// finished first). Must be accepted under ADR-005.
		part2 := make([]byte, 5*1024*1024)
		for i := range part2 {
			part2[i] = 0xBB
		}
		uploadPart(t, h, bucket, key, uploadID, 2, part2) // pins P from part 2
	})

	t.Run("U7_part_retry_idempotent", func(t *testing.T) {
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "retry-test.dat"

		uploadID := initiateMultipart(t, h, bucket, key)

		// Upload part 1 successfully.
		part1 := make([]byte, 5*1024*1024) // 5MiB, block-aligned
		for i := range part1 {
			part1[i] = 0xAA
		}
		uploadPart(t, h, bucket, key, uploadID, 1, part1)

		// Re-upload part 1 with the SAME size (retry after a network failure).
		// ADR-005 rule 5: same N → same offset → byte-identical ciphertext; the
		// retry is idempotent and must succeed (200), not 400.
		part1Retry := make([]byte, 5*1024*1024)
		for i := range part1Retry {
			part1Retry[i] = 0xAA
		}
		uploadPart(t, h, bucket, key, uploadID, 1, part1Retry) // fatals if not 200
	})

	t.Run("U8_non_block_aligned_part_rejected", func(t *testing.T) {
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "unaligned-test.dat"

		uploadID := initiateMultipart(t, h, bucket, key)

		// A part whose size is not a multiple of the block size. 10,000,000 %
		// 65536 = 16976 (not aligned) — still rejected under ADR-005 rule 1.
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

	t.Run("U8_zero_byte_first_part_rejected", func(t *testing.T) {
		// An empty first part cannot pin the uniform part size P (ADR-005 rule 1).
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "zero-byte.dat"

		uploadID := initiateMultipart(t, h, bucket, key)

		part := make([]byte, 0)
		url := fmt.Sprintf("/%s/%s?partNumber=1&uploadId=%s", bucket, key, uploadID)
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(part))
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 BadRequest for empty first part, got %d: %s", w.Code, w.Body.String())
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("empty first part")) {
			t.Errorf("Expected error about empty first part not pinning P, got: %s", w.Body.String())
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

// TestMultipartADR005Acceptance is the acceptance suite for ADR-005 (out-of-order
// multipart via the uniform-part-size contract). Every subtest exercises the
// real HTTP handler path through internal/crypto — no mock-only coverage (the
// bf-28rb standard). Each maps to an explicit acceptance criterion in the
// bf-5tol4d task description.
func TestMultipartADR005Acceptance(t *testing.T) {
	// mkPart returns a size-byte part whose content is a function of the absolute
	// byte offset base+offset — so any single-byte CTR/HMAC offset error between
	// parts diverges immediately on retrieval (a constant fill would not).
	mkPart := func(base, size int) []byte {
		b := make([]byte, size)
		for i := range b {
			b[i] = byte((base + i) % 251)
		}
		return b
	}

	// AC: "shuffled/concurrent part arrival at >=44MB byte-verified round-trip".
	// Nine parts (eight 5MiB + one 4MiB short final = 44MiB) are uploaded
	// concurrently from goroutines in a shuffled, non-sequential order. The
	// per-upload multipartLock serializes state updates, so -race must stay clean.
	// uploadPart (which calls t.Fatalf) cannot be used inside goroutines, so each
	// goroutine records its outcome into its own results slot.
	t.Run("concurrent_shuffled_44MiB_byte_verified", func(t *testing.T) {
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "concurrent-44mib.parquet"
		const regular = 5 * 1024 * 1024
		const short = 4 * 1024 * 1024

		// Eight full P parts (base offsets advance by P) + one short final.
		var parts [][]byte
		var plaintext []byte
		base := 0
		for i := 0; i < 8; i++ {
			p := mkPart(base, regular)
			parts = append(parts, p)
			plaintext = append(plaintext, p...)
			base += regular
		}
		final := mkPart(base, short)
		parts = append(parts, final)
		plaintext = append(plaintext, final...)
		if len(plaintext) < 44_000_000 { // 44MiB (46,137,344 B) >= 44MB
			t.Fatalf("plaintext %d B is below the 44MB acceptance scale", len(plaintext))
		}

		uploadID := initiateMultipart(t, h, bucket, key)

		// Shuffled arrival order: parts 5,1,7,3,9,2,8,4,6 (the short final, part
		// 9, lands in the middle of the stream, not last).
		order := []int{5, 1, 7, 3, 9, 2, 8, 4, 6}
		type result struct {
			part int
			code int
			body string
		}
		results := make([]result, len(order))
		var wg sync.WaitGroup
		for i, partNum := range order {
			i, partNum := i, partNum
			wg.Add(1)
			go func() {
				defer wg.Done()
				url := fmt.Sprintf("/%s/%s?partNumber=%d&uploadId=%s", bucket, key, partNum, uploadID)
				req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(parts[partNum-1]))
				w := httptest.NewRecorder()
				h.HandleRoot(w, req)
				results[i] = result{part: partNum, code: w.Code, body: w.Body.String()}
			}()
		}
		wg.Wait()

		for _, r := range results {
			if r.code != http.StatusOK {
				t.Errorf("concurrent UploadPart %d failed: status %d: %s", r.part, r.code, r.body)
			}
		}

		// Complete with parts in part-number order (1..9); recordingBackend etags
		// are "etag-part-<N>".
		etags := make([]string, len(parts))
		for i := range parts {
			etags[i] = fmt.Sprintf("etag-part-%d", i+1)
		}
		completeMultipart(t, h, bucket, key, uploadID, etags)

		// Full GET: byte-for-byte at 44MiB.
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET after concurrent complete failed: status %d: %s", w.Code, w.Body.String())
		}
		if !bytes.Equal(w.Body.Bytes(), plaintext) {
			t.Fatalf("concurrent 44MiB round-trip mismatch: got %d bytes, want %d; first divergence at %d",
				w.Body.Len(), len(plaintext), firstDivergence(w.Body.Bytes(), plaintext))
		}
	})

	// AC: "retried part mid-upload". Parts 1,2,3 are uploaded, then part 2 (a
	// part that is NOT the only one present) is re-uploaded with the same size.
	// ADR-005 rule 5: same N -> same offset -> byte-identical ciphertext; the
	// retry is idempotent (200) and the object round-trips byte-for-byte.
	t.Run("retried_part_mid_upload", func(t *testing.T) {
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "retry-mid.dat"
		const P = 5 * 1024 * 1024

		p1 := mkPart(0, P)
		p2 := mkPart(P, P)
		p3 := mkPart(2*P, P)
		uploadID := initiateMultipart(t, h, bucket, key)
		uploadPart(t, h, bucket, key, uploadID, 1, p1)
		uploadPart(t, h, bucket, key, uploadID, 2, p2)
		uploadPart(t, h, bucket, key, uploadID, 3, p3)
		uploadPart(t, h, bucket, key, uploadID, 2, p2) // mid-upload same-size retry — must be 200

		plaintext := append(append(append([]byte{}, p1...), p2...), p3...)
		completeMultipart(t, h, bucket, key, uploadID, []string{"etag-part-1", "etag-part-2", "etag-part-3"})

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET after mid-upload retry failed: status %d: %s", w.Code, w.Body.String())
		}
		if !bytes.Equal(w.Body.Bytes(), plaintext) {
			t.Fatalf("retry-mid-upload round-trip mismatch: got %d bytes, want %d; first divergence at %d",
				w.Body.Len(), len(plaintext), firstDivergence(w.Body.Bytes(), plaintext))
		}
	})

	// AC: "short final part arriving before some middle parts". The
	// highest-numbered part is the short final (< P) and it arrives BEFORE a
	// middle part. Upload order is 1 (P), 3 (short final), 2 (P). The offset
	// (N-1)*P/BlockSize is computable from N regardless of arrival order, so this
	// must succeed and round-trip byte-for-byte.
	t.Run("short_final_before_middle_parts", func(t *testing.T) {
		_, _, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "short-final-before-middle.dat"
		const P = 5 * 1024 * 1024
		const short = 3 * 1024 * 1024 // < P, block-aligned

		p1 := mkPart(0, P)       // base, pins P
		p2 := mkPart(P, P)       // middle part, arrives LAST
		p3 := mkPart(2*P, short) // short final (highest number), arrives before part 2
		uploadID := initiateMultipart(t, h, bucket, key)
		uploadPart(t, h, bucket, key, uploadID, 1, p1)
		uploadPart(t, h, bucket, key, uploadID, 3, p3) // short final before the middle part
		uploadPart(t, h, bucket, key, uploadID, 2, p2)

		plaintext := append(append(append([]byte{}, p1...), p2...), p3...)
		completeMultipart(t, h, bucket, key, uploadID, []string{"etag-part-1", "etag-part-2", "etag-part-3"})

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET after short-final-before-middle failed: status %d: %s", w.Code, w.Body.String())
		}
		if !bytes.Equal(w.Body.Bytes(), plaintext) {
			t.Fatalf("short-final-before-middle round-trip mismatch: got %d bytes, want %d; first divergence at %d",
				w.Body.Len(), len(plaintext), firstDivergence(w.Body.Bytes(), plaintext))
		}
	})

	// AC: "short-final-part-arriving-FIRST hard-fails poisoned with no stored
	// object". The short final part (part 3) arrives first and pins P too small;
	// the next regular part (5MiB > P) contradicts the contract. ADR-005 rule 4:
	// the offending UploadPart is rejected AND the upload id is poisoned, so
	// Complete fails and no violating object is ever stored (GET -> 404).
	t.Run("short_final_first_poisons_no_object", func(t *testing.T) {
		_, rb, h := recordingTestSetup(t)
		bucket, key := "test-bucket", "short-final-first-poison.dat"
		const regular = 5 * 1024 * 1024
		const short = 4 * 1024 * 1024 // < regular, block-aligned

		uploadID := initiateMultipart(t, h, bucket, key)

		// Part 3 (the short final) arrives FIRST and pins P = short.
		shortReq := httptest.NewRequest(http.MethodPut,
			fmt.Sprintf("/%s/%s?partNumber=3&uploadId=%s", bucket, key, uploadID),
			bytes.NewReader(mkPart(2*regular, short)))
		shortW := httptest.NewRecorder()
		h.HandleRoot(shortW, shortReq)
		if shortW.Code != http.StatusOK {
			t.Fatalf("short final part (first arrival) should be accepted (200), got %d: %s", shortW.Code, shortW.Body.String())
		}

		// Part 1 (regular, 5MiB) now contradicts P (5MiB > 4MiB) -> 400 + poison.
		contradictReq := httptest.NewRequest(http.MethodPut,
			fmt.Sprintf("/%s/%s?partNumber=1&uploadId=%s", bucket, key, uploadID),
			bytes.NewReader(mkPart(0, regular)))
		contradictW := httptest.NewRecorder()
		h.HandleRoot(contradictW, contradictReq)
		if contradictW.Code != http.StatusBadRequest {
			t.Fatalf("contradicting part should be rejected with 400, got %d: %s", contradictW.Code, contradictW.Body.String())
		}
		if !bytes.Contains(contradictW.Body.Bytes(), []byte("invalidated")) &&
			!bytes.Contains(contradictW.Body.Bytes(), []byte("larger than")) {
			t.Errorf("contradiction error should explain the invalidation, got: %s", contradictW.Body.String())
		}

		// Poison propagates: any further UploadPart on this id also fails.
		afterReq := httptest.NewRequest(http.MethodPut,
			fmt.Sprintf("/%s/%s?partNumber=2&uploadId=%s", bucket, key, uploadID),
			bytes.NewReader(mkPart(regular, regular)))
		afterW := httptest.NewRecorder()
		h.HandleRoot(afterW, afterReq)
		if afterW.Code != http.StatusBadRequest {
			t.Fatalf("UploadPart after poison should be rejected with 400, got %d: %s", afterW.Code, afterW.Body.String())
		}

		// Complete must fail (upload is poisoned) — never assemble/store.
		var xmlBody bytes.Buffer
		xmlBody.WriteString("<CompleteMultipartUpload>")
		for i := 1; i <= 3; i++ {
			fmt.Fprintf(&xmlBody, "<Part><PartNumber>%d</PartNumber><ETag>etag-part-%d</ETag></Part>", i, i)
		}
		xmlBody.WriteString("</CompleteMultipartUpload>")
		completeReq := httptest.NewRequest(http.MethodPost,
			fmt.Sprintf("/%s/%s?uploadId=%s", bucket, key, uploadID), &xmlBody)
		completeW := httptest.NewRecorder()
		h.HandleRoot(completeW, completeReq)
		if completeW.Code == http.StatusOK {
			t.Fatalf("Complete on a poisoned upload must fail, got 200: %s", completeW.Body.String())
		}
		if !bytes.Contains(completeW.Body.Bytes(), []byte("invalidated")) {
			t.Errorf("Complete failure should tell the client the upload is invalidated, got: %s", completeW.Body.String())
		}

		// No violating object was ever stored: GET returns 404 NoSuchKey.
		getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
		getW := httptest.NewRecorder()
		h.HandleRoot(getW, getReq)
		if getW.Code != http.StatusNotFound {
			t.Fatalf("GET on a poisoned/never-completed upload must be 404 (no stored object), got %d: %s", getW.Code, getW.Body.String())
		}
		if n := rb.objectPutCount(bucket, key); n != 0 {
			t.Fatalf("the object key was Put %d time(s) — a violating object must never be stored", n)
		}
	})
}
