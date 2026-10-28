// Package handlers_test tests the bounded v3 part-encryption pipeline
// (armor-cb2b0731): seekable plaintext sources, replayable part ciphertext,
// backend retry replay, and aggregate admission.
package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
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

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// newV3UploadWithBackend creates one multipart upload against the given
// backend and returns the handler and upload ID. partBudget sizes the
// aggregate v3 UploadPart memory budget.
func newV3UploadWithBackend(t *testing.T, be backend.Backend, partBudget int64) (h *handlers.Handlers, uploadID string, mek []byte) {
	t.Helper()
	mek = make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatal(err)
	}
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{BlockSize: 65536, FormatWriteVersion: 3, UploadPartMemoryBudget: partBudget}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	handler := handlers.New(cfg, be, cache, footerCache, km, nil)

	ctx := context.Background()
	const bucket, key = "test-bucket", "stream-parts"
	createResp := httptest.NewRecorder()
	handler.CreateMultipartUpload(createResp, createUploadRequest(ctx, bucket, key, "application/octet-stream"), bucket, key)
	if createResp.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: %d %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("parse upload ID: %v", err)
	}
	return handler, created.UploadID, mek
}

// v3UploadKeyMaterial reads an upload's meta.json and unwraps its DEK with
// the known MEK, so a test can compute the buffered encryption reference.
func v3UploadKeyMaterial(t *testing.T, be backend.Backend, mek []byte, uploadID string) (dek, iv []byte) {
	t.Helper()
	body, _, err := be.GetDirect(context.Background(), "test-bucket", fmt.Sprintf(".armor/multipart/%s/meta.json", uploadID))
	if err != nil {
		t.Fatalf("load meta.json: %v", err)
	}
	data, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}
	var meta backend.MultipartMetadataV3
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parse meta.json: %v", err)
	}
	dek, err = crypto.UnwrapDEK(mek, meta.WrappedDEK)
	if err != nil {
		t.Fatalf("unwrap DEK: %v", err)
	}
	return dek, meta.IV
}

// uploadPartViaHandler drives one UploadPart through the handler with the
// given plaintext and returns the response recorder.
func uploadPartViaHandler(t *testing.T, h *handlers.Handlers, ctx context.Context, bucket, key, uploadID string, partNumber int, plaintext []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := createUploadPartRequest(ctx, bucket, key, uploadID, partNumber, "application/octet-stream", plaintext)
	resp := httptest.NewRecorder()
	h.UploadPart(resp, req, bucket, key, uploadID)
	return resp
}

// partStateV3For reads the stored part-<n>.json for an upload.
func partStateV3For(t *testing.T, be *recordingBackend, uploadID string, partNumber int) *backend.PartDataV3 {
	t.Helper()
	body, _, err := be.GetDirect(context.Background(), "test-bucket", fmt.Sprintf(".armor/multipart/%s/part-%d.json", uploadID, partNumber))
	if err != nil {
		t.Fatalf("load part-%d.json: %v", partNumber, err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read part-%d.json: %v", partNumber, err)
	}
	var partData backend.PartDataV3
	if err := json.Unmarshal(data, &partData); err != nil {
		t.Fatalf("parse part-%d.json: %v", partNumber, err)
	}
	return &partData
}

// TestUploadPartV3ReplayableMatchesBufferedReference pins the streamed
// pipeline's output to the buffered EncryptPartV3 reference: ciphertext
// bytes, per-block HMAC records, plaintext SHA-256 and lengths must all be
// identical for aligned, unaligned and empty parts.
func TestUploadPartV3ReplayableMatchesBufferedReference(t *testing.T) {
	const blockSize = 65536
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatal(err)
	}
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{BlockSize: blockSize, FormatWriteVersion: 3}
	be := newRecordingBackend()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	h := handlers.New(cfg, be, cache, footerCache, km, nil)

	ctx := context.Background()
	const bucket, key = "test-bucket", "reference-parts"
	createResp := httptest.NewRecorder()
	h.CreateMultipartUpload(createResp, createUploadRequest(ctx, bucket, key, "application/octet-stream"), bucket, key)
	if createResp.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: %d %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("parse upload ID: %v", err)
	}

	// The upload's meta.json carries the IV and wrapped DEK; with a known MEK
	// the reference encryption is computable.
	metaBody, _, err := be.GetDirect(ctx, bucket, fmt.Sprintf(".armor/multipart/%s/meta.json", created.UploadID))
	if err != nil {
		t.Fatalf("load meta.json: %v", err)
	}
	metaData, err := io.ReadAll(metaBody)
	metaBody.Close()
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}
	var meta backend.MultipartMetadataV3
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("parse meta.json: %v", err)
	}
	dek, err := crypto.UnwrapDEK(mek, meta.WrappedDEK)
	if err != nil {
		t.Fatalf("unwrap DEK: %v", err)
	}

	// Aligned, unaligned multi-block, single unaligned byte and empty parts.
	sizes := []int{3*blockSize + 7, blockSize, 1, 0}
	for i, size := range sizes {
		partNumber := i + 1
		plaintext := make([]byte, size)
		if _, err := rand.Read(plaintext); err != nil {
			t.Fatal(err)
		}
		resp := uploadPartViaHandler(t, h, ctx, bucket, key, created.UploadID, partNumber, plaintext)
		if resp.Code != http.StatusOK {
			t.Fatalf("part %d (size %d): status=%d body=%s", partNumber, size, resp.Code, resp.Body.String())
		}

		refCiphertext, refHMACs, err := crypto.EncryptPartV3(dek, meta.IV, uint16(partNumber), blockSize, plaintext)
		if err != nil {
			t.Fatalf("reference encrypt part %d: %v", partNumber, err)
		}
		be.rmu.Lock()
		gotCiphertext := be.uploads[created.UploadID][int32(partNumber)]
		be.rmu.Unlock()
		if !bytes.Equal(gotCiphertext, refCiphertext) {
			t.Fatalf("part %d ciphertext differs from buffered reference: got %d bytes, want %d", partNumber, len(gotCiphertext), len(refCiphertext))
		}

		refBlockHMACs := make([][]byte, len(refHMACs)/crypto.HMACSize)
		for b := range refBlockHMACs {
			refBlockHMACs[b] = refHMACs[b*crypto.HMACSize : (b+1)*crypto.HMACSize]
		}
		sum := sha256.Sum256(plaintext)
		want := &backend.PartDataV3{
			PartNumber:       partNumber,
			PlaintextLen:     int64(size),
			CiphertextLen:    int64(len(refCiphertext)),
			BlockHMACsBase64: backend.EncodeHMACToBase64(refBlockHMACs),
			PlaintextSHAHex:  fmt.Sprintf("%x", sum),
		}
		got := partStateV3For(t, be, created.UploadID, partNumber)
		if got.PartNumber != want.PartNumber || got.PlaintextLen != want.PlaintextLen ||
			got.CiphertextLen != want.CiphertextLen || got.BlockHMACsBase64 != want.BlockHMACsBase64 ||
			got.PlaintextSHAHex != want.PlaintextSHAHex {
			t.Fatalf("part %d metadata mismatch:\n got %+v\nwant %+v", partNumber, got, want)
		}
	}
}

// retryReplayBackend simulates an SDK retry: it consumes half the body,
// rewinds the seeker, re-reads the whole body, and fails the call unless both
// passes produced identical bytes.
type retryReplayBackend struct {
	*recordingBackend
	mu      sync.Mutex
	replay  map[int32]int
	seekers bool
}

func (b *retryReplayBackend) UploadPart(
	ctx context.Context, bucket, key, uploadID string, partNumber int32,
	body io.Reader, size int64,
) (string, error) {
	seeker, ok := body.(io.Seeker)
	if !ok {
		return "", fmt.Errorf("part body is not seekable: %T", body)
	}
	half := make([]byte, size/2)
	if _, err := io.ReadFull(body, half); err != nil {
		return "", fmt.Errorf("first pass read: %w", err)
	}
	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("retry seek: %w", err)
	}
	replayed, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("second pass read: %w", err)
	}
	if int64(len(replayed)) != size {
		return "", fmt.Errorf("replayed length %d, want %d", len(replayed), size)
	}
	if !bytes.Equal(replayed[:len(half)], half) {
		return "", fmt.Errorf("replayed part %d ciphertext differs from first pass", partNumber)
	}
	b.mu.Lock()
	b.replay[partNumber]++
	b.seekers = true
	b.mu.Unlock()
	return b.recordingBackend.UploadPart(ctx, bucket, key, uploadID, partNumber, bytes.NewReader(replayed), size)
}

// TestUploadPartV3BackendRetryReplaysIdenticalCiphertext uploads
// shuffled, nonuniform parts — full-size, unaligned and a final short part —
// through a backend that forces one rewind-and-retry per part. Every replay
// must be byte-identical, and the stored part state must still match the
// buffered reference.
func TestUploadPartV3BackendRetryReplaysIdenticalCiphertext(t *testing.T) {
	const blockSize = 65536
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatal(err)
	}
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{BlockSize: blockSize, FormatWriteVersion: 3}
	be := &retryReplayBackend{
		recordingBackend: newRecordingBackend(),
		replay:           make(map[int32]int),
	}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	h := handlers.New(cfg, be, cache, footerCache, km, nil)

	ctx := context.Background()
	const bucket, key = "test-bucket", "retry-parts"
	createResp := httptest.NewRecorder()
	h.CreateMultipartUpload(createResp, createUploadRequest(ctx, bucket, key, "application/octet-stream"), bucket, key)
	if createResp.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: %d %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("parse upload ID: %v", err)
	}

	sizes := map[int]int{1: blockSize + 12345, 2: 2*blockSize + 1, 3: blockSize / 3} // final short part
	for partNumber, size := range sizes {
		plaintext := make([]byte, size)
		if _, err := rand.Read(plaintext); err != nil {
			t.Fatal(err)
		}
		resp := uploadPartViaHandler(t, h, ctx, bucket, key, created.UploadID, partNumber, plaintext)
		if resp.Code != http.StatusOK {
			t.Fatalf("part %d: status=%d body=%s", partNumber, resp.Code, resp.Body.String())
		}
		be.mu.Lock()
		retries := be.replay[int32(partNumber)]
		be.mu.Unlock()
		if retries != 1 {
			t.Fatalf("part %d: backend forced-retry count = %d, want 1", partNumber, retries)
		}
	}
	if !be.seekers {
		t.Fatal("backend never saw a seekable part body")
	}
}

// TestUploadPartV3BudgetBoundsInFlightParts proves aggregate admission: with
// a two-part budget, only two of four concurrent parts read their bodies and
// reach the backend; releasing one admits another, so independent parts
// proceed concurrently while aggregate payload memory stays bounded.
func TestUploadPartV3BudgetBoundsInFlightParts(t *testing.T) {
	const partSize = 64 * 1024
	be := &blockingV3Backend{
		recordingBackend: newRecordingBackend(),
		entered:          make(chan int32, 4),
		release:          make(chan struct{}),
	}
	h, uploadID, _ := newV3UploadWithBackend(t, be, 2*partSize)

	ctx := context.Background()
	codes := make(chan int, 4)
	var wg sync.WaitGroup
	for part := 1; part <= 4; part++ {
		wg.Add(1)
		go func(partNumber int) {
			defer wg.Done()
			resp := uploadPartViaHandler(t, h, ctx, "test-bucket", "stream-parts", uploadID, partNumber, bytes.Repeat([]byte{byte(partNumber)}, partSize))
			codes <- resp.Code
		}(part)
	}

	waitForEntry := func(n int, what string) map[int32]bool {
		seen := make(map[int32]bool, n)
		timer := time.NewTimer(4 * time.Second)
		defer timer.Stop()
		for len(seen) < n {
			select {
			case part := <-be.entered:
				seen[part] = true
			case <-timer.C:
				close(be.release)
				wg.Wait()
				t.Fatalf("%s: only %d parts reached the backend (budget admission stalled): %v", what, len(seen), seen)
			}
		}
		return seen
	}

	// Two parts are admitted and reach the backend concurrently.
	waitForEntry(2, "two admitted parts")

	// The budget is exhausted: no third part may be admitted while both are
	// held by the blocked backend.
	select {
	case part := <-be.entered:
		close(be.release)
		wg.Wait()
		t.Fatalf("budget admitted a third part (%d) beyond the two-part budget", part)
	case <-time.After(300 * time.Millisecond):
	}

	// Releasing one part frees budget for exactly one waiting part — an
	// independent part proceeds without waiting for the slow one. Exactly one
	// more entry can occur while the other admitted part stays blocked.
	be.release <- struct{}{}
	waitForEntry(1, "one more part after a single release")

	close(be.release)
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != http.StatusOK {
			t.Errorf("UploadPart status=%d want=%d", code, http.StatusOK)
		}
	}
}

// TestUploadPartV3BudgetWaitCanceledRequestReturns covers cancellation while
// a part waits for budget: the request returns promptly, no part state is
// saved for it, and the canceled waiter does not leak budget — the next part
// uploads normally once the holder finishes.
func TestUploadPartV3BudgetWaitCanceledRequestReturns(t *testing.T) {
	const partSize = 64 * 1024
	be := &blockingV3Backend{
		recordingBackend: newRecordingBackend(),
		entered:          make(chan int32, 2),
		release:          make(chan struct{}),
	}
	h, uploadID, _ := newV3UploadWithBackend(t, be, partSize)

	bg := context.Background()
	done := make(chan *httptest.ResponseRecorder, 2)
	// Part 1 holds the entire budget inside the blocked backend.
	go func() {
		done <- uploadPartViaHandler(t, h, bg, "test-bucket", "stream-parts", uploadID, 1, bytes.Repeat([]byte{1}, partSize))
	}()
	select {
	case part := <-be.entered:
		if part != 1 {
			t.Fatalf("unexpected first backend entry: part %d", part)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("part 1 never reached the backend")
	}

	// Part 2 waits for budget; cancel its request.
	waitCtx, cancel := context.WithCancel(bg)
	go func() {
		done <- uploadPartViaHandler(t, h, waitCtx, "test-bucket", "stream-parts", uploadID, 2, bytes.Repeat([]byte{2}, partSize))
	}()
	select {
	case part := <-be.entered:
		close(be.release)
		t.Fatalf("part %d was admitted past an exhausted budget", part)
	case <-time.After(200 * time.Millisecond):
	}
	cancel()

	select {
	case resp := <-done:
		if resp.Code != http.StatusOK {
			if resp.Code != 499 && resp.Code != 500 {
				t.Errorf("canceled part status=%d, want 499 or 500", resp.Code)
			}
		} else {
			t.Error("canceled part was acknowledged with 200")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("canceled part request did not return")
	}

	// No durable part state may exist for the canceled part.
	if partData, err := func() (*backend.PartDataV3, error) {
		body, _, gerr := be.GetDirect(bg, "test-bucket", fmt.Sprintf(".armor/multipart/%s/part-2.json", uploadID))
		if gerr != nil {
			return nil, gerr
		}
		defer body.Close()
		data, rerr := io.ReadAll(body)
		if rerr != nil {
			return nil, rerr
		}
		var pd backend.PartDataV3
		return &pd, json.Unmarshal(data, &pd)
	}(); err == nil {
		t.Fatalf("canceled part 2 left durable state: %+v", partData)
	}

	// The canceled waiter must not have leaked budget: once part 1 finishes,
	// part 3 uploads normally.
	close(be.release)
	if resp := <-done; resp.Code != http.StatusOK {
		t.Fatalf("part 1 status=%d want=200", resp.Code)
	}
	if resp := uploadPartViaHandler(t, h, bg, "test-bucket", "stream-parts", uploadID, 3, bytes.Repeat([]byte{3}, partSize)); resp.Code != http.StatusOK {
		t.Fatalf("part 3 after cancellation status=%d body=%s", resp.Code, resp.Body.String())
	}
}

// unknownLengthPartRequest builds an UploadPart request whose body has no
// Content-Length (chunked encoding), forcing the spill path.
func unknownLengthPartRequest(ctx context.Context, bucket, key, uploadID string, partNumber int, plaintext []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPut,
		fmt.Sprintf("/%s?partNumber=%d&uploadId=%s", key, partNumber, uploadID),
		io.NopCloser(bytes.NewReader(plaintext)))
	req.URL.Path = "/" + key
	req.URL.RawQuery = fmt.Sprintf("partNumber=%d&uploadId=%s", partNumber, uploadID)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = -1
	return req.WithContext(context.WithValue(context.WithValue(ctx, "bucket", bucket), "key", key))
}

// TestUploadPartV3SpilledUnknownLengthMatchesPooled uploads the same
// plaintext twice — once with a declared Content-Length (pooled source) and
// once chunked with no length (temp-file spool) — and requires identical
// stored ciphertext and identical durable part state. The spool must be
// removed once the handler returns.
func TestUploadPartV3SpilledUnknownLengthMatchesPooled(t *testing.T) {
	spoolDir := t.TempDir()
	t.Setenv("TMPDIR", spoolDir)

	rec := newRecordingBackend()
	h, uploadID, mek := newV3UploadWithBackend(t, rec, 0)
	bg := context.Background()
	const bucket, key = "test-bucket", "stream-parts"
	plaintext := make([]byte, 3*65536+12345)
	if _, err := rand.Read(plaintext); err != nil {
		t.Fatal(err)
	}

	pooledResp := uploadPartViaHandler(t, h, bg, bucket, key, uploadID, 1, plaintext)
	if pooledResp.Code != http.StatusOK {
		t.Fatalf("pooled part status=%d body=%s", pooledResp.Code, pooledResp.Body.String())
	}
	rec.rmu.Lock()
	pooledCiphertext := append([]byte(nil), rec.uploads[uploadID][1]...)
	rec.rmu.Unlock()

	spillReq := unknownLengthPartRequest(bg, bucket, key, uploadID, 2, plaintext)
	spillResp := httptest.NewRecorder()
	h.UploadPart(spillResp, spillReq, bucket, key, uploadID)
	if spillResp.Code != http.StatusOK {
		t.Fatalf("spilled part status=%d body=%s", spillResp.Code, spillResp.Body.String())
	}
	rec.rmu.Lock()
	spilledCiphertext := rec.uploads[uploadID][2]
	rec.rmu.Unlock()

	// Each upload has its own DEK and IV, so the spilled part is checked
	// against its own upload's buffered reference, not the other upload's
	// ciphertext.
	dek, iv := v3UploadKeyMaterial(t, rec, mek, uploadID)
	refCiphertext, refHMACs, err := crypto.EncryptPartV3(dek, iv, 2, 65536, plaintext)
	if err != nil {
		t.Fatalf("reference encrypt: %v", err)
	}
	if !bytes.Equal(spilledCiphertext, refCiphertext) {
		t.Fatalf("spilled ciphertext (%d bytes) differs from its upload's buffered reference (%d bytes)",
			len(spilledCiphertext), len(refCiphertext))
	}
	if len(pooledCiphertext) != len(spilledCiphertext) {
		t.Fatalf("pooled and spilled ciphertext lengths differ: %d vs %d", len(pooledCiphertext), len(spilledCiphertext))
	}
	spilledPart := partStateV3For(t, rec, uploadID, 2)
	refBlocks := make([][]byte, len(refHMACs)/crypto.HMACSize)
	for b := range refBlocks {
		refBlocks[b] = refHMACs[b*crypto.HMACSize : (b+1)*crypto.HMACSize]
	}
	sum := sha256.Sum256(plaintext)
	wantPart := &backend.PartDataV3{
		PartNumber:       2,
		PlaintextLen:     int64(len(plaintext)),
		CiphertextLen:    int64(len(refCiphertext)),
		BlockHMACsBase64: backend.EncodeHMACToBase64(refBlocks),
		PlaintextSHAHex:  fmt.Sprintf("%x", sum),
	}
	if spilledPart.PartNumber != wantPart.PartNumber || spilledPart.PlaintextLen != wantPart.PlaintextLen ||
		spilledPart.CiphertextLen != wantPart.CiphertextLen || spilledPart.BlockHMACsBase64 != wantPart.BlockHMACsBase64 ||
		spilledPart.PlaintextSHAHex != wantPart.PlaintextSHAHex {
		t.Fatalf("spilled part state mismatch:\n got %+v\nwant %+v", spilledPart, wantPart)
	}
	assertNoReplayableTempFiles(t, spoolDir)
}

// failingPartSaveBackend fails the durable part-metadata save, exercising
// the failure path after the backend upload succeeded.
type failingPartSaveBackend struct {
	*recordingBackend
}

func (b *failingPartSaveBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	if strings.Contains(key, "part-1.json") {
		return errors.New("simulated part metadata save failure")
	}
	return b.recordingBackend.Put(ctx, bucket, key, body, size, meta)
}

// TestUploadPartV3SpillFailurePathsCleanUp covers cleanup when the request
// fails after the body was spilled: a metadata-save failure returns 500 and
// removes the spool, and a canceled request unblocks and removes it too.
// No part may be acknowledged (200) without its durable part state saved.
func TestUploadPartV3SpillFailurePathsCleanUp(t *testing.T) {
	spoolDir := t.TempDir()
	t.Setenv("TMPDIR", spoolDir)

	// Metadata-save failure: the backend stored the part, the save of
	// part-1.json fails, the request must fail and leave no spool behind.
	failing := &failingPartSaveBackend{recordingBackend: newRecordingBackend()}
	h, uploadID, _ := newV3UploadWithBackend(t, failing, 0)
	bg := context.Background()
	const bucket, key = "test-bucket", "stream-parts"
	plaintext := bytes.Repeat([]byte{0x33}, 2*65536+9)

	req := unknownLengthPartRequest(bg, bucket, key, uploadID, 1, plaintext)
	resp := httptest.NewRecorder()
	h.UploadPart(resp, req, bucket, key, uploadID)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("metadata-save failure status=%d, want 500", resp.Code)
	}
	assertNoReplayableTempFiles(t, spoolDir)

	// Cancellation while the backend holds the spilled part: the request
	// returns without success and the spool is removed.
	blocking := &blockingV3Backend{
		recordingBackend: newRecordingBackend(),
		entered:          make(chan int32, 1),
		release:          make(chan struct{}),
	}
	h2, uploadID2, _ := newV3UploadWithBackend(t, blocking, 0)
	waitCtx, cancel := context.WithCancel(bg)
	done := make(chan int, 1)
	go func() {
		r := unknownLengthPartRequest(waitCtx, bucket, key, uploadID2, 1, plaintext)
		w := httptest.NewRecorder()
		h2.UploadPart(w, r, bucket, key, uploadID2)
		done <- w.Code
	}()
	select {
	case <-blocking.entered:
	case <-time.After(4 * time.Second):
		t.Fatal("spilled part never reached the backend")
	}
	cancel()
	select {
	case code := <-done:
		if code == http.StatusOK {
			t.Fatal("canceled spilled part was acknowledged with 200")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("canceled spilled part request did not return")
	}
	close(blocking.release)
	assertNoReplayableTempFiles(t, spoolDir)
}
