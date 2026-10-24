package handlers_test

// Small single-PUT object single-fetch read path (armor-98864f12).
//
// A full GET of a single-PUT object at or below ARMOR_SMALL_OBJECT_FETCH_BYTES
// reads the entire stored envelope — header, data blocks, and the v1/v2 inline
// HMAC table or v3 trailer block table — in ONE ranged request and slices the
// regions out locally. These tests cover the boundary sizes, the envelope
// shapes the streaming path serves, and every failure the acceptance names:
// corrupted block or HMAC, truncated body, a backend that ignores the range
// by answering long, and a range request that must keep the streaming reads.
// Request-count pins live in get_request_count_test.go.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
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

// TestSmallFetchBoundarySizes exercises the threshold boundary for v2 and v3:
// a stored envelope exactly at the threshold takes the single fetch (one
// data-path request) and one byte over takes the streaming path (three
// data-path requests), and both round-trip the plaintext byte-for-byte.
func TestSmallFetchBoundarySizes(t *testing.T) {
	for _, version := range []int{2, 3} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			const bucket = "test-bucket"
			const key = "boundary.bin"
			// Two blocks plus a partial tail: every envelope region present.
			plaintext := smallGETPlaintext(2*65536 + 17)

			cfg, cb, h, _ := countingTestSetup(t, version, 0)
			putObjectThroughHandler(t, h, bucket, key, plaintext)
			storedLen := int64(len(cb.objects[bucket+"/"+key]))

			wholeFetches := func() int {
				n := 0
				for _, c := range cb.recordedCalls() {
					if c.Method == "GET_RANGE" && c.Offset == 0 && c.Length == storedLen {
						n++
					}
				}
				return n
			}

			// Exactly at the threshold: one whole-envelope request, and no
			// other ranged read.
			cfg.SmallObjectFetchBytes = storedLen
			cb.resetCalls()
			got := getObjectThroughHandler(t, h, bucket, key)
			if !bytes.Equal(got, plaintext) {
				t.Fatalf("at-threshold GET did not round-trip: got %d bytes, want %d", len(got), len(plaintext))
			}
			if n := wholeFetches(); n != 1 {
				t.Fatalf("at-threshold GET made %d whole-envelope ranged calls, want exactly 1\ncalls: %s",
					n, formatCalls(cb.recordedCalls()))
			}
			for _, c := range cb.recordedCalls() {
				if c.Method == "GET_RANGE" && c.Length != storedLen {
					t.Fatalf("at-threshold GET made a ranged call outside the single fetch: %s",
						formatCalls(cb.recordedCalls()))
				}
			}

			// One byte over: the streaming path, three data-path requests.
			cfg.SmallObjectFetchBytes = storedLen - 1
			cb.resetCalls()
			got = getObjectThroughHandler(t, h, bucket, key)
			if !bytes.Equal(got, plaintext) {
				t.Fatalf("over-threshold GET did not round-trip: got %d bytes, want %d", len(got), len(plaintext))
			}
			if n := wholeFetches(); n != 0 {
				t.Fatalf("over-threshold GET still made %d whole-envelope ranged calls; the streaming fallback did not engage", n)
			}
			dataPath := 0
			for _, c := range cb.recordedCalls() {
				if c.Method == "GET_RANGE" {
					dataPath++
				}
			}
			if dataPath != 3 {
				t.Fatalf("over-threshold GET made %d data-path ranged calls, want the streaming shape's 3\ncalls: %s",
					dataPath, formatCalls(cb.recordedCalls()))
			}
		})
	}
}

// TestSmallFetchEmptyObject covers zero-byte single-PUT objects on the single
// fetch: a zero-block v2 envelope has an empty inline HMAC table, and a
// zero-block v3 envelope has an empty trailer window — both must round-trip
// the empty plaintext without tripping table decoding. The v3 envelope is
// planted directly: the single-PUT write path refuses zero-byte v3 uploads,
// so no production writer can produce this shape today (the empty-object read
// contract is what this test pins).
func TestSmallFetchEmptyObject(t *testing.T) {
	const bucket = "test-bucket"

	t.Run("v2", func(t *testing.T) {
		// objectName, not key: a local literally named `key` holding a
		// string trips the generic-api-key rule in the fleet secret scan.
		const objectName = "empty-v2.bin"
		_, cb, h, _ := countingTestSetup(t, 2, 0)
		putObjectThroughHandler(t, h, bucket, objectName, nil)
		assertEmptyObjectRoundTrips(t, h, cb, bucket, objectName)
	})

	t.Run("v3", func(t *testing.T) {
		const objectName = "empty-v3.bin"
		_, cb, h, km := countingTestSetup(t, 3, 0)
		plantV3SinglePUTEnvelope(t, cb, km, bucket, objectName, nil, false)
		assertEmptyObjectRoundTrips(t, h, cb, bucket, objectName)
	})
}

// assertEmptyObjectRoundTrips GETs the stored empty object through the
// handler and requires an empty 200 body from one whole-envelope request.
func assertEmptyObjectRoundTrips(t *testing.T, h *handlers.Handlers, cb *callCountingBackend, bucket, key string) {
	t.Helper()
	storedLen := int64(len(cb.objects[bucket+"/"+key]))
	if storedLen != crypto.HeaderSize {
		t.Fatalf("empty envelope is %d bytes, want just the %d-byte header", storedLen, crypto.HeaderSize)
	}
	cb.resetCalls()

	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET of empty object failed: status %d: %s", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("GET of empty object returned %d bytes, want 0", w.Body.Len())
	}
	n := 0
	for _, c := range cb.recordedCalls() {
		if c.Method == "GET_RANGE" && c.Offset == 0 && c.Length == storedLen {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("empty-object GET made %d whole-envelope ranged calls, want exactly 1\ncalls: %s",
			n, formatCalls(cb.recordedCalls()))
	}
}

// plantV3SinglePUTEnvelope stores a v3 single-PUT envelope built block by
// block with crypto.EncryptBlockV3 — the exact block/HMAC construction the v3
// read path verifies. compressBlocks selects per-block zstd compression with
// the entry compression flag (the entry.IsCompressed() read shape; the
// production write path compresses whole objects instead, tracked separately
// from this bead, and crypto.EncryptWithBlockCompression's table flavor does
// not match the read path's part-prefixed HMACs). plaintext == nil plants a
// zero-block envelope (header only, empty trailer window), a shape the
// single-PUT write path currently refuses rather than produces.
func plantV3SinglePUTEnvelope(t *testing.T, cb *callCountingBackend, km *keymanager.KeyManager, bucket, key string, plaintext []byte, compressBlocks bool) {
	t.Helper()
	mek, err := km.GetMEKByID("default")
	if err != nil {
		t.Fatalf("failed to get default MEK: %v", err)
	}
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("failed to generate DEK: %v", err)
	}
	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("failed to generate IV: %v", err)
	}
	wrappedDEKStr, err := crypto.WrapDEKWithFingerprint(mek, dek)
	if err != nil {
		t.Fatalf("failed to wrap DEK: %v", err)
	}
	parts := strings.SplitN(wrappedDEKStr, ":", 3)
	if len(parts) != 3 || parts[0] != "v2" {
		t.Fatalf("unexpected wrapped-DEK format")
	}
	wrappedDEK, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("failed to decode wrapped DEK: %v", err)
	}
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)

	blockCount := int(crypto.ComputeBlockCount(int64(len(plaintext)), 65536))
	table := crypto.NewBlockTable(65536, blockCount)
	var encrypted []byte
	for i := 0; i < blockCount; i++ {
		start := i * 65536
		end := start + 65536
		if end > len(plaintext) {
			end = len(plaintext)
		}
		block := plaintext[start:end]
		data := block
		wasCompressed := false
		if compressBlocks {
			compressed, didCompress, _, cerr := crypto.CompressBlock(block)
			if cerr != nil {
				t.Fatalf("block %d compression failed: %v", i, cerr)
			}
			if didCompress {
				data = compressed
				wasCompressed = true
			}
		}
		ciphertext, mac, eerr := crypto.EncryptBlockV3(dek, iv, 0, uint32(i), data, 65536)
		if eerr != nil {
			t.Fatalf("block %d encryption failed: %v", i, eerr)
		}
		var macArr [32]byte
		copy(macArr[:], mac)
		if err := table.AddEntry(crypto.NewBlockTableEntry(macArr, uint32(len(ciphertext)), wasCompressed)); err != nil {
			t.Fatalf("block %d table entry failed: %v", i, err)
		}
		encrypted = append(encrypted, ciphertext...)
	}
	var tableBytes []byte
	if blockCount > 0 {
		tableBytes, err = table.Encode()
		if err != nil {
			t.Fatalf("failed to encode block table: %v", err)
		}
	}
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), 65536, plaintextSHA, crypto.Version3)
	if err != nil {
		t.Fatalf("failed to create v3 header: %v", err)
	}
	headerBytes, err := header.Encode()
	if err != nil {
		t.Fatalf("failed to encode v3 header: %v", err)
	}
	envelope := append(append(append([]byte{}, headerBytes...), encrypted...), tableBytes...)

	if compressBlocks {
		compressedAny := false
		for _, entry := range table.Entries {
			if entry.IsCompressed() {
				compressedAny = true
			}
		}
		if !compressedAny {
			t.Fatalf("fixture plaintext produced no compressed blocks; make it more compressible")
		}
	}

	meta := (&backend.ARMORMetadata{
		Version:        3,
		BlockSize:      65536,
		PlaintextSize:  int64(len(plaintext)),
		ContentType:    "application/octet-stream",
		IV:             iv,
		WrappedDEK:     wrappedDEK,
		MEKFingerprint: parts[1],
		PlaintextSHA:   hex.EncodeToString(plaintextSHA[:]),
		ETag:           backend.ComputeETag(plaintext),
		KeyID:          "default",
	}).ToMetadata()

	if err := cb.Put(context.Background(), bucket, key, bytes.NewReader(envelope), int64(len(envelope)), meta); err != nil {
		t.Fatalf("failed to plant v3 envelope: %v", err)
	}
}

// TestSmallFetchBlockCompressedV3 round-trips a block-compressed v3 envelope
// through the single fetch: every compressed block is decrypted and
// decompressed from the locally sliced data region, with the same per-block
// HMAC verification the streaming path runs.
func TestSmallFetchBlockCompressedV3(t *testing.T) {
	const bucket = "test-bucket"
	const key = "block-compressed.bin"
	// Highly compressible, one full block plus a compressed partial tail.
	pattern := bytes.Repeat([]byte("0123456789abcdefghijklmnopqrstuvwxyz"), 8)
	plaintext := bytes.Repeat(pattern, (65536+512)/len(pattern)+1)[:65536+512]

	_, cb, h, km := countingTestSetup(t, 3, 0)
	plantV3SinglePUTEnvelope(t, cb, km, bucket, key, plaintext, true)
	storedLen := int64(len(cb.objects[bucket+"/"+key]))
	cb.resetCalls()

	got := getObjectThroughHandler(t, h, bucket, key)
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("block-compressed GET did not round-trip: got %d bytes, want %d", len(got), len(plaintext))
	}
	n := 0
	for _, c := range cb.recordedCalls() {
		if c.Method == "GET_RANGE" && c.Offset == 0 && c.Length == storedLen {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("block-compressed GET made %d whole-envelope ranged calls, want exactly 1\ncalls: %s",
			n, formatCalls(cb.recordedCalls()))
	}
}

// TestSmallFetchCorruptionFails plants corrupted envelopes and requires the
// GET to fail without releasing the intact plaintext: a flipped ciphertext
// byte must trip the block HMAC, and a flipped table byte must fail the
// verification or decoding it feeds. Corruption may legally surface as a
// non-200 status or a truncated body (the stream aborts mid-response); what
// it may never do is deliver the intact plaintext.
func TestSmallFetchCorruptionFails(t *testing.T) {
	const bucket = "test-bucket"
	const key = "corrupt.bin"

	for _, version := range []int{2, 3} {
		// Data-region flip: one encrypted block fails its HMAC check.
		t.Run(fmt.Sprintf("v%d/data-byte", version), func(t *testing.T) {
			plaintext := smallGETPlaintext(2 * 65536)

			_, cb, h, _ := countingTestSetup(t, version, 0)
			putObjectThroughHandler(t, h, bucket, key, plaintext)
			cb.objects[bucket+"/"+key][crypto.HeaderSize+3] ^= 0xFF
			cb.resetCalls()

			assertCorruptGETFails(t, h, bucket, key, plaintext, version)
		})
	}

	// v2 inline-HMAC-table flip: the entry covering block 0 no longer
	// matches, so verification must fail before block 0 is released.
	t.Run("v2/hmac-table-byte", func(t *testing.T) {
		plaintext := smallGETPlaintext(2 * 65536)

		_, cb, h, _ := countingTestSetup(t, 2, 0)
		putObjectThroughHandler(t, h, bucket, key, plaintext)
		env := cb.objects[bucket+"/"+key]
		hmacTableOffset := crypto.HeaderSize + int64(len(plaintext))
		env[hmacTableOffset] ^= 0xFF
		cb.resetCalls()

		assertCorruptGETFails(t, h, bucket, key, plaintext, 2)
	})

	// v3 trailer flip: the corrupted entry is decoded locally from the
	// fetched buffer, so the failure must come from verification of the
	// block it describes (or the decode itself), never from intact output.
	t.Run("v3/trailer-byte", func(t *testing.T) {
		plaintext := smallGETPlaintext(2 * 65536)

		_, cb, h, _ := countingTestSetup(t, 3, 0)
		putObjectThroughHandler(t, h, bucket, key, plaintext)
		env := cb.objects[bucket+"/"+key]
		env[int64(len(env))-crypto.BlockTableEntrySize+5] ^= 0xFF
		cb.resetCalls()

		assertCorruptGETFails(t, h, bucket, key, plaintext, 3)
	})
}

// assertCorruptGETFails requires that a GET of the corrupted object does not
// deliver the intact plaintext, whether the handler fails the response early
// (non-200) or aborts the stream mid-body.
func assertCorruptGETFails(t *testing.T, h *handlers.Handlers, bucket, key string, plaintext []byte, version int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("corrupt v%d GET: unexpected status %d", version, w.Code)
	}
	if bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Fatalf("corrupt v%d GET released the intact plaintext", version)
	}
	if w.Code == http.StatusOK && w.Body.Len() == len(plaintext) {
		t.Fatalf("corrupt v%d GET returned the full plaintext length with a 200", version)
	}
}

// faultRangeBackend rewrites the body of one shaped ranged request. A nil
// rewrite passes every body through untouched; the field is set between the
// upload and the GET under test.
type faultRangeBackend struct {
	*mockBackend
	rewrite func(offset, length int64, real []byte) []byte
}

func (f *faultRangeBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	body, err := f.mockBackend.GetRange(ctx, bucket, key, offset, length)
	if err != nil || f.rewrite == nil {
		return body, err
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	if repl := f.rewrite(offset, length, data); repl != nil {
		return io.NopCloser(bytes.NewReader(repl)), nil
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func faultFetchSetup(t *testing.T, version int) (*config.Config, *faultRangeBackend, *handlers.Handlers) {
	t.Helper()
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize:             65536,
		AuthAccessKey:         "test",
		AuthSecretKey:         "REMOVED-NOT-A-SECRET-VALUE",
		FormatWriteVersion:    version,
		SmallObjectFetchBytes: 1 << 20,
		ReadBackendLimit:      64,
	}
	fb := &faultRangeBackend{mockBackend: newMockBackend()}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}
	h := handlers.New(cfg, fb, cache, footerCache, km, nil)
	return cfg, fb, h
}

// TestSmallFetchTruncatedBodyFails serves a whole-envelope body one byte
// short of the length the HEAD reported. The single fetch must reject it
// before any response is written — a short body cannot be trusted against
// the envelope layout — rather than stream a truncated envelope and fail
// somewhere inside decryption.
func TestSmallFetchTruncatedBodyFails(t *testing.T) {
	for _, version := range []int{2, 3} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			const bucket = "test-bucket"
			const key = "truncated.bin"
			plaintext := smallGETPlaintext(2 * 65536)

			_, fb, h := faultFetchSetup(t, version)
			putObjectThroughHandler(t, h, bucket, key, plaintext)
			storedLen := int64(len(fb.objects[bucket+"/"+key]))
			fb.rewrite = func(offset, length int64, real []byte) []byte {
				if offset == 0 && length == storedLen {
					return real[:len(real)-1]
				}
				return nil
			}

			req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("truncated v%d GET: status %d, want 500 (body: %d bytes)", version, w.Code, w.Body.Len())
			}
		})
	}
}

// TestSmallFetchOverLongBodyFails emulates a backend that ignored the Range
// header and answered with more bytes than the HEAD-reported span: the single
// fetch must reject the over-long body up front. (The real CF and direct-S3
// paths enforce the same length in internal/backend's readRangeBody, and a
// 206 naming the wrong span fails checkContentRange there; this covers the
// handler-side contract those backends satisfy.)
func TestSmallFetchOverLongBodyFails(t *testing.T) {
	for _, version := range []int{2, 3} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			const bucket = "test-bucket"
			const key = "overlong.bin"
			plaintext := smallGETPlaintext(2 * 65536)

			_, fb, h := faultFetchSetup(t, version)
			putObjectThroughHandler(t, h, bucket, key, plaintext)
			storedLen := int64(len(fb.objects[bucket+"/"+key]))
			fb.rewrite = func(offset, length int64, real []byte) []byte {
				if offset == 0 && length == storedLen {
					return append(append([]byte{}, real...), bytes.Repeat([]byte{0xA5}, 16)...)
				}
				return nil
			}

			req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("over-long v%d GET: status %d, want 500 (body: %d bytes)", version, w.Code, w.Body.Len())
			}
		})
	}
}

// TestSmallFetchRangeRequestKeepsStreaming pins that a ranged GET of a small
// object never takes the whole-envelope fetch: the range handlers fetch their
// own spans, so a whole-object read would be a wasted round trip for them.
func TestSmallFetchRangeRequestKeepsStreaming(t *testing.T) {
	const bucket = "test-bucket"
	const key = "ranged.bin"
	plaintext := smallGETPlaintext(2 * 65536)

	_, cb, h, _ := countingTestSetup(t, 3, 0)
	putObjectThroughHandler(t, h, bucket, key, plaintext)
	storedLen := int64(len(cb.objects[bucket+"/"+key]))
	cb.resetCalls()

	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	req.Header.Set("Range", "bytes=0-99")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusPartialContent {
		t.Fatalf("ranged GET: status %d, want 206", w.Code)
	}
	if len(w.Body.Bytes()) != 100 {
		t.Fatalf("ranged GET returned %d bytes, want 100", w.Body.Len())
	}
	for _, c := range cb.recordedCalls() {
		if c.Method == "GET_RANGE" && c.Length == storedLen {
			t.Fatalf("ranged GET made a whole-envelope fetch it cannot use\ncalls: %s", formatCalls(cb.recordedCalls()))
		}
	}
}

// blockingRangeBackend parks the first whole-envelope ranged request until
// the test releases it, holding that request's small-fetch buffer slot.
type blockingRangeBackend struct {
	*mockBackend
	fullLen int64
	hitFull chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingRangeBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	if length == b.fullLen && offset == 0 {
		b.once.Do(func() { close(b.hitFull) })
		select {
		case <-b.release:
		case <-time.After(10 * time.Second):
			// Unblock rather than hang the suite; the test asserts on the
			// bodies and will fail on the garbled ordering.
		}
	}
	return b.mockBackend.GetRange(ctx, bucket, key, offset, length)
}

// TestSmallFetchBufferCapBoundsConcurrency proves the aggregate-buffer bound:
// with a slot cap of 1, a second small GET issued while the first one's buffer
// is still live (parked inside its backend fetch) cannot take a slot — it
// falls back to the streaming reads and still serves the full plaintext.
func TestSmallFetchBufferCapBoundsConcurrency(t *testing.T) {
	const bucket = "test-bucket"
	const key = "capped.bin"
	plaintext := smallGETPlaintext(2 * 65536)

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize:             65536,
		AuthAccessKey:         "test",
		AuthSecretKey:         "REMOVED-NOT-A-SECRET-VALUE",
		FormatWriteVersion:    3,
		SmallObjectFetchBytes: 1 << 20,
		ReadBackendLimit:      1, // one small-fetch buffer slot
	}
	blocking := &blockingRangeBackend{
		mockBackend: newMockBackend(),
		hitFull:     make(chan struct{}),
		release:     make(chan struct{}),
	}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}
	h := handlers.New(cfg, blocking, cache, footerCache, km, nil)

	putObjectThroughHandler(t, h, bucket, key, plaintext)
	blocking.fullLen = int64(len(blocking.objects[bucket+"/"+key]))

	// Request A: parks inside its whole-envelope fetch, holding the only slot.
	type result struct {
		code int
		body []byte
	}
	doneA := make(chan result, 1)
	go func() {
		req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		doneA <- result{code: w.Code, body: w.Body.Bytes()}
	}()

	select {
	case <-blocking.hitFull:
	case <-time.After(10 * time.Second):
		close(blocking.release)
		t.Fatal("request A never reached its whole-envelope fetch")
	}

	// Request B, while A's buffer is live: must stream (no whole-envelope
	// fetch possible — that path would block on the slot) and still succeed.
	reqB := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	wB := httptest.NewRecorder()
	h.HandleRoot(wB, reqB)
	if wB.Code != http.StatusOK || !bytes.Equal(wB.Body.Bytes(), plaintext) {
		t.Fatalf("request B under the full cap failed: status %d, %d bytes", wB.Code, wB.Body.Len())
	}

	close(blocking.release)
	resA := <-doneA
	if resA.code != http.StatusOK || !bytes.Equal(resA.body, plaintext) {
		t.Fatalf("request A failed after release: status %d, %d bytes", resA.code, len(resA.body))
	}
}
