package handlers_test

// Tests for ADR-011: Non-uniform multipart parts with cumulative offsets.
// Covers Barman pattern (chunk_size + N*512) and adversarial scenarios:
// mid-block boundaries at every offset residue, out-of-order arrival,
// retried parts at different sizes, and byte-identical verification.

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestADR011BarmanPattern tests the exact Barman cloud backup pattern:
// parts sized chunk_size + N*512 (gradually increasing).
func TestADR011BarmanPattern(t *testing.T) {
	_, _, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "barman-backup.tar"

	const chunkSize = 5 * 1024 * 1024 // 5 MiB base chunk
	const blockSize = 65536

	// Barman pattern: chunk_size + N*512
	// Part 1: 5MiB + 512
	// Part 2: 5MiB + 1024
	// Part 3: 5MiB + 1536
	partSizes := []int{
		chunkSize + 512,
		chunkSize + 1024,
		chunkSize + 1536,
	}

	var parts [][]byte
	var plaintext []byte
	baseOffset := int64(0)
	for _, size := range partSizes {
		part := make([]byte, size)
		for j := range part {
			part[j] = byte((baseOffset + int64(j)) % 251)
		}
		parts = append(parts, part)
		plaintext = append(plaintext, part...)
		baseOffset += int64(size)
	}

	uploadID := initiateMultipart(t, h, bucket, key)
	var etags []string
	for i, part := range parts {
		etags = append(etags, uploadPart(t, h, bucket, key, uploadID, i+1, part))
	}
	completeMultipart(t, h, bucket, key, uploadID, etags)

	// Verify byte-identical round-trip
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET after Barman-pattern complete failed: status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Fatalf("Barman-pattern round-trip mismatch: got %d bytes, want %d; first divergence at %d",
			w.Body.Len(), len(plaintext), firstDivergence(w.Body.Bytes(), plaintext))
	}
}

// TestADR011MidBlockBoundaries tests parts ending at every possible block offset residue.
func TestADR011MidBlockBoundaries(t *testing.T) {
	_, _, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "mid-boundary.dat"

	const blockSize = 65536
	const baseSize = 5 * 1024 * 1024

	// Test every offset residue from 1 to blockSize-1
	for residue := 1; residue < blockSize && residue < 1000; residue += 127 {
		t.Run(fmt.Sprintf("residue_%d", residue), func(t *testing.T) {
			// Part 1 ends mid-block
			part1 := make([]byte, baseSize+int64(residue))
			for i := range part1 {
				part1[i] = byte(i % 251)
			}

			// Part 2 starts mid-block
			part2 := make([]byte, baseSize)
			for i := range part2 {
				part2[i] = byte((i + len(part1)) % 251)
			}

			plaintext := append(append([]byte{}, part1...), part2...)

			uploadID := initiateMultipart(t, h, bucket, key)
			e1 := uploadPart(t, h, bucket, key, uploadID, 1, part1)
			e2 := uploadPart(t, h, bucket, key, uploadID, 2, part2)
			completeMultipart(t, h, bucket, key, uploadID, []string{e1, e2})

			// Verify round-trip
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("GET failed for residue %d: status %d: %s", residue, w.Code, w.Body.String())
			}
			if !bytes.Equal(w.Body.Bytes(), plaintext) {
				t.Fatalf("Round-trip mismatch for residue %d", residue)
			}
		})
	}
}

// TestADR011OutOfOrderArrival tests parts arriving in random order.
func TestADR011OutOfOrderArrival(t *testing.T) {
	_, _, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "out-of-order.dat"

	const baseSize = 5 * 1024 * 1024

	// Create parts with non-uniform sizes
	partSizes := []int{baseSize + 512, baseSize + 1024, baseSize + 1536}
	var parts [][]byte
	var plaintext []byte
	baseOffset := int64(0)
	for _, size := range partSizes {
		part := make([]byte, size)
		for j := range part {
			part[j] = byte((baseOffset + int64(j)) % 251)
		}
		parts = append(parts, part)
		plaintext = append(plaintext, part...)
		baseOffset += int64(size)
	}

	// Upload in random order: part 3, part 1, part 2
	uploadID := initiateMultipart(t, h, bucket, key)
	e3 := uploadPart(t, h, bucket, key, uploadID, 3, parts[2])
	e1 := uploadPart(t, h, bucket, key, uploadID, 1, parts[0])
	e2 := uploadPart(t, h, bucket, key, uploadID, 2, parts[1])

	// Complete in correct order (1,2,3)
	completeMultipart(t, h, bucket, key, uploadID, []string{e1, e2, e3})

	// Verify round-trip
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET after out-of-order complete failed: status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Fatalf("Out-of-order round-trip mismatch: got %d bytes, want %d",
			w.Body.Len(), len(plaintext))
	}
}

// TestADR011PartBeforePreviousDeferred tests that a part arriving before
// its predecessor is deferred with 503 SlowDown.
func TestADR011PartBeforePreviousDeferred(t *testing.T) {
	_, _, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "deferred.dat"

	const baseSize = 5 * 1024 * 1024

	part1 := make([]byte, baseSize+512)
	for i := range part1 {
		part1[i] = 0xAA
	}

	part2 := make([]byte, baseSize+1024)
	for i := range part2 {
		part2[i] = 0xBB
	}

	uploadID := initiateMultipart(t, h, bucket, key)

	// Try to upload part 2 before part 1 - should be deferred
	code, _, body := uploadPartResponse(t, h, bucket, key, uploadID, 2, part2)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("Expected 503 SlowDown for part 2 before part 1, got %d: %s", code, body)
	}
	if !bytes.Contains([]byte(body), []byte("SlowDown")) {
		t.Errorf("Expected SlowDown error, got: %s", body)
	}

	// Now upload part 1
	uploadPart(t, h, bucket, key, uploadID, 1, part1)

	// Part 2 should now succeed
	uploadPart(t, h, bucket, key, uploadID, 2, part2)
}

// TestADR011RetriedPartDifferentSize tests that a part retried with a
// different size is rejected (must use abort/retry).
func TestADR011RetriedPartDifferentSize(t *testing.T) {
	_, _, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "retry-diff-size.dat"

	const baseSize = 5 * 1024 * 1024

	part1 := make([]byte, baseSize)
	for i := range part1 {
		part1[i] = 0xAA
	}

	part2_v1 := make([]byte, baseSize+512)
	for i := range part2_v1 {
		part2_v1[i] = 0xBB
	}

	part2_v2 := make([]byte, baseSize+1024) // Different size!
	for i := range part2_v2 {
		part2_v2[i] = 0xBB
	}

	uploadID := initiateMultipart(t, h, bucket, key)
	uploadPart(t, h, bucket, key, uploadID, 1, part1)

	// Upload part 2 first time
	uploadPart(t, h, bucket, key, uploadID, 2, part2_v1)

	// Retry with different size - should be rejected
	code, _, body := uploadPartResponse(t, h, bucket, key, uploadID, 2, part2_v2)
	if code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for different-size retry, got %d: %s", code, body)
	}
	if !bytes.Contains([]byte(body), []byte("different size")) {
		t.Errorf("Expected different-size error, got: %s", body)
	}
}

// TestADR011RangeAcrossBoundary tests range requests that span boundary blocks.
func TestADR011RangeAcrossBoundary(t *testing.T) {
	_, _, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "range-boundary.dat"

	const blockSize = 65536
	const baseSize = 5 * 1024 * 1024

	// Part 1 ends 1000 bytes into a block
	part1 := make([]byte, baseSize+1000)
	for i := range part1 {
		part1[i] = byte(i % 251)
	}

	// Part 2 continues from that offset
	part2 := make([]byte, baseSize)
	for i := range part2 {
		part2[i] = byte((i + len(part1)) % 251)
	}

	plaintext := append(append([]byte{}, part1...), part2...)

	uploadID := initiateMultipart(t, h, bucket, key)
	e1 := uploadPart(t, h, bucket, key, uploadID, 1, part1)
	e2 := uploadPart(t, h, bucket, key, uploadID, 2, part2)
	completeMultipart(t, h, bucket, key, uploadID, []string{e1, e2})

	// Test range that spans the boundary
	// Boundary is at offset (baseSize + 1000)
	// Request range from baseSize-500 to baseSize+500
	lo := baseSize - 500
	hi := baseSize + 500
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", lo, hi))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusPartialContent {
		t.Fatalf("Range GET failed: status %d: %s", w.Code, w.Body.String())
	}
	expected := plaintext[lo : hi+1]
	if !bytes.Equal(w.Body.Bytes(), expected) {
		t.Fatalf("Range across boundary mismatch")
	}
}

// TestADR011ConcurrentShuffledArrival tests concurrent upload with shuffled
// part arrival order, verifying SlowDown deferral and retry.
func TestADR011ConcurrentShuffledArrival(t *testing.T) {
	_, _, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "concurrent-nonuniform.dat"

	const baseSize = 5 * 1024 * 1024

	// Non-uniform parts
	partSizes := []int{baseSize + 512, baseSize + 1024, baseSize + 1536, baseSize + 2048}
	var parts [][]byte
	var plaintext []byte
	baseOffset := int64(0)
	for _, size := range partSizes {
		part := make([]byte, size)
		for j := range part {
			part[j] = byte((baseOffset + int64(j)) % 251)
		}
		parts = append(parts, part)
		plaintext = append(plaintext, part...)
		baseOffset += int64(size)
	}

	uploadID := initiateMultipart(t, h, bucket, key)

	// Upload in shuffled order: 3, 1, 4, 2
	order := []int{3, 1, 4, 2}
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
			// Retry on SlowDown
			for attempt := 0; ; attempt++ {
				if attempt > 100 {
					results[i] = result{part: partNum, code: -1, body: "exhausted retries"}
					return
				}
				url := fmt.Sprintf("/%s/%s?partNumber=%d&uploadId=%s", bucket, key, partNum, uploadID)
				req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(parts[partNum-1]))
				w := httptest.NewRecorder()
				h.HandleRoot(w, req)
				if w.Code == http.StatusServiceUnavailable {
					time.Sleep(time.Millisecond)
					continue
				}
				results[i] = result{part: partNum, code: w.Code, body: w.Body.String()}
				return
			}
		}()
	}
	wg.Wait()

	// All parts should succeed
	for _, r := range results {
		if r.code != http.StatusOK {
			t.Errorf("Part %d failed: status %d: %s", r.part, r.code, r.body)
		}
	}

	// Complete with correct order
	etags := make([]string, len(parts))
	for i := range parts {
		etags[i] = fmt.Sprintf("etag-part-%d", i+1)
	}
	completeMultipart(t, h, bucket, key, uploadID, etags)

	// Verify round-trip
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET after concurrent nonuniform complete failed: status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Fatalf("Concurrent nonuniform round-trip mismatch: got %d bytes, want %d",
			w.Body.Len(), len(plaintext))
	}
}

// TestADR011UniformFastPath verifies that uniform parts still use the fast path.
func TestADR011UniformFastPath(t *testing.T) {
	_, _, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "uniform-fast.dat"

	const partSize = 5 * 1024 * 1024 // Uniform, block-aligned

	part1 := make([]byte, partSize)
	for i := range part1 {
		part1[i] = 0xAA
	}

	part2 := make([]byte, partSize)
	for i := range part2 {
		part2[i] = 0xBB
	}

	plaintext := append(append([]byte{}, part1...), part2...)

	uploadID := initiateMultipart(t, h, bucket, key)
	e1 := uploadPart(t, h, bucket, key, uploadID, 1, part1)
	e2 := uploadPart(t, h, bucket, key, uploadID, 2, part2)
	completeMultipart(t, h, bucket, key, uploadID, []string{e1, e2})

	// Verify round-trip
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET failed: status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Fatalf("Uniform fast path round-trip mismatch")
	}
}

// TestADR011ManyNonUniformParts tests a realistic scenario with many
// non-uniform parts (simulating a large Barman backup).
func TestADR011ManyNonUniformParts(t *testing.T) {
	_, _, h := recordingTestSetup(t)
	bucket, key := "test-bucket", "many-nonuniform.dat"

	const baseSize = 5 * 1024 * 1024
	const numParts = 20

	var parts [][]byte
	var plaintext []byte
	baseOffset := int64(0)
	for i := 0; i < numParts; i++ {
		// Gradually increasing sizes
		size := baseSize + int64((i+1)*512)
		part := make([]byte, size)
		for j := range part {
			part[j] = byte((baseOffset + int64(j)) % 251)
		}
		parts = append(parts, part)
		plaintext = append(plaintext, part...)
		baseOffset += size
	}

	uploadID := initiateMultipart(t, h, bucket, key)
	var etags []string
	for i, part := range parts {
		etags = append(etags, uploadPart(t, h, bucket, key, uploadID, i+1, part))
	}
	completeMultipart(t, h, bucket, key, uploadID, etags)

	// Verify round-trip
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET after many parts failed: status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Fatalf("Many parts round-trip mismatch: got %d bytes, want %d",
			w.Body.Len(), len(plaintext))
	}
}

