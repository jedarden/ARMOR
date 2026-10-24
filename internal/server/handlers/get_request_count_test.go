package handlers_test

// Small-object full-GET backend request-count baseline (armor-50a36688).
//
// For objects small enough that bandwidth is irrelevant (commitgraph's
// per-repo Parquet artifacts: ~620k objects, median ~23 KB), full-GET latency
// is set by the number of sequential backend round trips the read path makes.
// These tests pin that number per envelope shape: one full-object GET of a
// small single-PUT object written in v1/v2 and in v3, and of a small v3
// multipart object. Every backend call the handler makes — including the
// readManifest GET that misses for every non-multipart object — is recorded
// and asserted, so a change that adds a round trip fails here, and a change
// that removes one updates this pin and the published baseline in
// docs/performance/small-object-get-baseline.md in the same bead.
//
// Since armor-420c15a5 the manifest probe and the plaintext-key HEAD run
// concurrently, so the first two recorded calls are asserted as an unordered
// pair and the overlap itself is proven deterministically: the guarded
// manifest GET blocks until a HEAD is recorded, which a sequential read path
// cannot satisfy (see callCountingBackend's overlap guard).

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

// backendCall is one read-path backend request issued by a handler.
type backendCall struct {
	Method string // GET, GET_DIRECT, GET_RANGE, HEAD
	Key    string // bucket + "/" + key as addressed on the backend
	Offset int64
	Length int64
	Hit    bool // backend had the object (a manifest miss records Hit=false)
}

// callCountingBackend wraps recordingBackend and records every read-path
// backend call, optionally injecting a fixed latency per call. Recording a
// miss matters as much as recording a hit: the readManifest GET for a
// single-PUT object is a real round trip even though it 404s.
type callCountingBackend struct {
	*recordingBackend
	delay time.Duration // injected per-call RTT; 0 disables

	// Overlap guard (armor-420c15a5): the manifest-object GET blocks until a
	// HEAD has been recorded and remembers that it happened. A read path that
	// still probes the manifest and heads the plaintext key sequentially can
	// never satisfy it — no HEAD is in flight while the manifest GET runs —
	// so manifestSawHead() after a measured GET is a deterministic proof the
	// two ran concurrently, not a timing assertion.
	overlapGuard bool
	headSeen     chan struct{}
	headSeenOnce sync.Once

	mu          sync.Mutex
	calls       []backendCall
	manifestSaw bool
}

func (c *callCountingBackend) record(method, key string, offset, length int64, hit bool) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	c.mu.Lock()
	c.calls = append(c.calls, backendCall{Method: method, Key: key, Offset: offset, Length: length, Hit: hit})
	c.mu.Unlock()
}

// manifestSawHead reports whether the guarded manifest GET observed a HEAD
// while it was in flight (see the overlap guard above).
func (c *callCountingBackend) manifestSawHead() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.manifestSaw
}

func (c *callCountingBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	body, info, err := c.mockBackend.Get(ctx, bucket, key)
	c.record("GET", bucket+"/"+key, 0, 0, err == nil)
	if c.overlapGuard && strings.HasSuffix(key, ".armor-manifest") {
		select {
		case <-c.headSeen:
			c.mu.Lock()
			c.manifestSaw = true
			c.mu.Unlock()
		case <-time.After(5 * time.Second):
			// Not seen: the read path ran the probe and the HEAD
			// sequentially. The caller's manifestSawHead() assertion fails
			// with that verdict; resolve here so a regression degrades to a
			// slow failure instead of a hang.
		}
	}
	return body, info, err
}

func (c *callCountingBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	body, info, err := c.mockBackend.GetDirect(ctx, bucket, key)
	c.record("GET_DIRECT", bucket+"/"+key, 0, 0, err == nil)
	return body, info, err
}

func (c *callCountingBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	body, _, err := c.mockBackend.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	c.record("GET_RANGE", bucket+"/"+key, offset, length, err == nil)
	return body, err
}

func (c *callCountingBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	body, headers, err := c.mockBackend.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	c.record("GET_RANGE", bucket+"/"+key, offset, length, err == nil)
	return body, headers, err
}

func (c *callCountingBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	info, err := c.mockBackend.Head(ctx, bucket, key)
	c.record("HEAD", bucket+"/"+key, 0, 0, err == nil)
	c.headSeenOnce.Do(func() { close(c.headSeen) })
	return info, err
}

// recordedCalls returns a copy of the call log.
func (c *callCountingBackend) recordedCalls() []backendCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]backendCall, len(c.calls))
	copy(out, c.calls)
	return out
}

// resetCalls clears the call log (between the writes that build state and
// the GET under measurement).
func (c *callCountingBackend) resetCalls() {
	c.mu.Lock()
	c.calls = nil
	c.mu.Unlock()
}

// countingTestSetup builds a Handlers over a call-counting backend. delay is
// the per-call latency injected for the benchmark; 0 for pure counting tests.
// SmallObjectFetchBytes is set to the production default (1 MiB, the
// ARMOR_SMALL_OBJECT_FETCH_BYTES default) so the pins reflect what production
// actually does; tests that need the streaming path set cfg.
// SmallObjectFetchBytes = 0 after setup.
func countingTestSetup(t *testing.T, formatWriteVersion int, delay time.Duration) (*config.Config, *callCountingBackend, *handlers.Handlers, *keymanager.KeyManager) {
	t.Helper()
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize:             65536,
		AuthAccessKey:         "test-access-key",
		AuthSecretKey:         "REMOVED-NOT-A-SECRET-VALUE",
		FormatWriteVersion:    formatWriteVersion,
		SmallObjectFetchBytes: 1 << 20,
		// ReadBackendLimit sizes the small-fetch buffer cap inside
		// handlers.New, so it must be set before construction: the latency
		// matrix runs cells at c256, and the default-fallback cap of 64
		// would push overflow requests onto the streaming reads mid-cell.
		// Harmless for the sequential counting tests.
		ReadBackendLimit: 512,
	}
	cb := &callCountingBackend{
		recordingBackend: newRecordingBackend(),
		delay:            delay,
		overlapGuard:     true,
		headSeen:         make(chan struct{}),
	}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}
	h := handlers.New(cfg, cb, cache, footerCache, km, nil)
	return cfg, cb, h, km
}

// smallGETPlaintext returns deterministic pseudo-random plaintext of n bytes
// (deterministic so a decryption failure can't hide behind RNG variance).
func smallGETPlaintext(n int) []byte {
	data := make([]byte, n)
	seed := uint32(n) | 1
	for i := range data {
		// xorshift32: cheap, deterministic, no shared RNG state.
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		data[i] = byte(seed)
	}
	return data
}

// putObjectThroughHandler issues a single-PUT upload through the HTTP surface.
func putObjectThroughHandler(t *testing.T, h *handlers.Handlers, bucket, key string, plaintext []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT %s failed: status %d: %s", key, w.Code, w.Body.String())
	}
}

// getObjectThroughHandler issues a full-object GET through the HTTP surface
// and returns the plaintext body.
func getObjectThroughHandler(t *testing.T, h *handlers.Handlers, bucket, key string) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s failed: status %d: %s", key, w.Code, w.Body.String())
	}
	return w.Body.Bytes()
}

// plantLegacyV1Object stores a v1-envelope object directly in the backend.
// Production code cannot write v1 (pinned by TestNoVersion1ProductionPaths),
// but the read path still serves v1 objects written before that gate, so the
// request-count baseline covers the shape. The envelope and metadata are
// built exactly as PutObject builds a v2 one, with the version set to 1.
func plantLegacyV1Object(t *testing.T, cb *callCountingBackend, km *keymanager.KeyManager, bucket, key string, plaintext []byte) {
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

	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), 65536, plaintextSHA, crypto.Version1)
	if err != nil {
		t.Fatalf("failed to create v1 header: %v", err)
	}
	headerBytes, err := header.Encode()
	if err != nil {
		t.Fatalf("failed to encode v1 header: %v", err)
	}
	// NewEncryptorWithVersion with Version1 — the tagged NewEncryptorV1 helper
	// is only for the legacy test build; the untagged constructor accepts v1.
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, 65536, crypto.Version1)
	if err != nil {
		t.Fatalf("failed to create v1 encryptor: %v", err)
	}
	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt v1: %v", err)
	}
	envelope := append(append(append([]byte{}, headerBytes...), encrypted...), hmacTable...)

	meta := (&backend.ARMORMetadata{
		Version:        1,
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
		t.Fatalf("failed to plant v1 object: %v", err)
	}
}

func formatCalls(calls []backendCall) string {
	var b strings.Builder
	for i, c := range calls {
		if i > 0 {
			b.WriteString(" -> ")
		}
		if c.Method == "GET_RANGE" {
			fmt.Fprintf(&b, "%s %s[%d,+%d](hit=%v)", c.Method, c.Key, c.Offset, c.Length, c.Hit)
		} else {
			fmt.Fprintf(&b, "%s %s(hit=%v)", c.Method, c.Key, c.Hit)
		}
	}
	return b.String()
}

// assertCallsEqual requires the recorded log to match want exactly, in order.
func assertCallsEqual(t *testing.T, got, want []backendCall, context string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d backend calls recorded, baseline is %d — a round trip was added or removed; update this pin and docs/performance/small-object-get-baseline.md in the same bead\nrecorded: %s",
			context, len(got), len(want), formatCalls(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: backend call %d mismatch:\n  got  %+v\n  want %+v\nfull recorded sequence: %s",
				context, i+1, got[i], want[i], formatCalls(got))
		}
	}
}

// assertProbePair asserts that the first two recorded calls are the manifest
// GET and the overlapped plaintext-key HEAD in either order — they race by
// design since armor-420c15a5 — and that the overlap guard fired: the
// manifest GET must have observed the HEAD in flight, which a read path that
// serializes the two calls cannot satisfy.
func assertProbePair(t *testing.T, cb *callCountingBackend, manifestCall, headCall backendCall, context string) {
	t.Helper()
	calls := cb.recordedCalls()
	if len(calls) < 2 {
		t.Fatalf("%s: only %d backend calls recorded, want at least the probe/HEAD pair\nrecorded: %s",
			context, len(calls), formatCalls(calls))
	}
	first, second := calls[0], calls[1]
	if !((first == manifestCall && second == headCall) || (first == headCall && second == manifestCall)) {
		t.Fatalf("%s: first two backend calls should be the manifest probe and the overlapped HEAD in either order:\n  got  %s, %s\n  want %s / %s\nfull recorded sequence: %s",
			context, formatCalls([]backendCall{first}), formatCalls([]backendCall{second}),
			formatCalls([]backendCall{manifestCall}), formatCalls([]backendCall{headCall}), formatCalls(calls))
	}
	if !cb.manifestSawHead() {
		t.Fatalf("%s: the manifest GET did not overlap a HEAD — the probe and the legacy HEAD ran sequentially; armor-420c15a5's round-trip saving is gone", context)
	}
}

// assertProbePairThen pins a measured GET whose first two backend calls are
// the manifest GET and the overlapped plaintext-key HEAD (assertProbePair),
// with the remaining calls matching want exactly, in order, and the total
// count pinned at 2+len(want).
func assertProbePairThen(t *testing.T, cb *callCountingBackend, manifestCall, headCall backendCall, want []backendCall, context string) {
	t.Helper()
	calls := cb.recordedCalls()
	if len(calls) != 2+len(want) {
		t.Fatalf("%s: %d backend calls recorded, baseline is %d — a round trip was added or removed; update this pin and docs/performance/small-object-get-baseline.md in the same bead\nrecorded: %s",
			context, len(calls), 2+len(want), formatCalls(calls))
	}
	assertProbePair(t, cb, manifestCall, headCall, context)
	assertCallsEqual(t, calls[2:], want, context)
}

// TestSmallObjectFullGETRequestCountSinglePUTV2 pins the backend request
// count for a full GET of a small single-PUT v2-envelope object: three calls
// total, two on the critical path. The readManifest GET (a miss for every
// non-multipart object) and the legacy-path HEAD overlap (armor-420c15a5 —
// previously the HEAD waited one full sequential round trip for the probe);
// then ONE ranged request covering the whole stored envelope — header, data
// blocks and the inline HMAC table together (armor-98864f12) — which the
// handler parses locally. Before armor-98864f12 this was three further
// sequential calls: envelope header, HMAC table, data.
func TestSmallObjectFullGETRequestCountSinglePUTV2(t *testing.T) {
	const bucket = "test-bucket"
	const key = "small-v2.bin"
	// 23 KiB: the commitgraph Parquet object median size (armor-50a36688).
	plaintext := smallGETPlaintext(23 * 1024)

	_, cb, h, _ := countingTestSetup(t, 2, 0)
	putObjectThroughHandler(t, h, bucket, key, plaintext)
	cb.resetCalls()

	got := getObjectThroughHandler(t, h, bucket, key)
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("GET did not round-trip the plaintext: got %d bytes, want %d", len(got), len(plaintext))
	}

	hmacSize := int64(crypto.ComputeBlockCount(int64(len(plaintext)), 65536)) * crypto.HMACSize
	storedLen := crypto.HeaderSize + int64(len(plaintext)) + hmacSize
	assertProbePairThen(t, cb,
		backendCall{Method: "GET", Key: bucket + "/" + key + ".armor-manifest", Hit: false},
		backendCall{Method: "HEAD", Key: bucket + "/" + key, Hit: true},
		[]backendCall{
			// One whole-envelope fetch (armor-98864f12): the handler slices
			// header, data and HMAC table out of this buffer locally.
			{Method: "GET_RANGE", Key: bucket + "/" + key, Offset: 0, Length: storedLen, Hit: true},
		},
		"single-PUT v2")
}

// TestSmallObjectFullGETRequestCountSinglePUTV1 plants a legacy v1 envelope
// and pins the same call sequence as v2: v1 and v2 share the inline-HMAC
// layout, differing only in counter derivation inside the decryptor.
func TestSmallObjectFullGETRequestCountSinglePUTV1(t *testing.T) {
	const bucket = "test-bucket"
	const key = "small-v1.bin"
	plaintext := smallGETPlaintext(23 * 1024)

	_, cb, h, km := countingTestSetup(t, 2, 0)
	plantLegacyV1Object(t, cb, km, bucket, key, plaintext)
	cb.resetCalls()

	got := getObjectThroughHandler(t, h, bucket, key)
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("GET did not round-trip the v1 plaintext: got %d bytes, want %d", len(got), len(plaintext))
	}

	hmacOffset := crypto.HeaderSize + int64(len(plaintext))
	hmacSize := int64(crypto.ComputeBlockCount(int64(len(plaintext)), 65536)) * crypto.HMACSize
	assertProbePairThen(t, cb,
		backendCall{Method: "GET", Key: bucket + "/" + key + ".armor-manifest", Hit: false},
		backendCall{Method: "HEAD", Key: bucket + "/" + key, Hit: true},
		[]backendCall{
			// One whole-envelope fetch (armor-98864f12): v1 shares the v2
			// inline-HMAC layout, so the same single request covers header,
			// data and table.
			{Method: "GET_RANGE", Key: bucket + "/" + key, Offset: 0, Length: hmacOffset + hmacSize, Hit: true},
		},
		"single-PUT v1")
}

// TestSmallObjectFullGETRequestCountSinglePUTV3 pins the backend request
// count for a full GET of a small single-PUT v3-envelope object: three calls
// total, two on the critical path — the overlapped manifest-probe/HEAD pair
// (armor-420c15a5), then ONE ranged request covering the whole stored
// envelope (armor-98864f12). The handler decodes the envelope header and the
// trailer block table from that buffer locally; before armor-98864f12 this
// was three further sequential calls: envelope header, trailer block table,
// data. The generation-keyed table cache (armor-c3199f1c) is untouched but
// unused by this path — there is no table fetch to cache.
func TestSmallObjectFullGETRequestCountSinglePUTV3(t *testing.T) {
	const bucket = "test-bucket"
	const key = "small-v3.bin"
	plaintext := smallGETPlaintext(23 * 1024)

	_, cb, h, _ := countingTestSetup(t, 3, 0)
	putObjectThroughHandler(t, h, bucket, key, plaintext)
	cb.resetCalls()

	got := getObjectThroughHandler(t, h, bucket, key)
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("GET did not round-trip the plaintext: got %d bytes, want %d", len(got), len(plaintext))
	}

	// The one data-path request is the whole stored envelope, exactly as the
	// HEAD reported it: header + data blocks + trailer block table.
	storedLen := int64(len(cb.objects[bucket+"/"+key]))
	calls := cb.recordedCalls()
	if len(calls) != 3 {
		t.Fatalf("single-PUT v3: %d backend calls recorded, baseline is 3 (manifest probe ∥ head, one whole-envelope fetch): %s",
			len(calls), formatCalls(calls))
	}
	assertProbePair(t, cb,
		backendCall{Method: "GET", Key: bucket + "/" + key + ".armor-manifest", Hit: false},
		backendCall{Method: "HEAD", Key: bucket + "/" + key, Hit: true},
		"single-PUT v3")
	fetch := calls[2]
	if fetch.Method != "GET_RANGE" || fetch.Offset != 0 || fetch.Length != storedLen {
		t.Fatalf("single-PUT v3: call 3 should be the whole-envelope fetch (%d bytes), got %+v", storedLen, fetch)
	}
}

// TestOverThresholdObjectKeepsStreamingPath pins the request count for an
// object one byte over the small-fetch threshold: the per-region streaming
// reads are unchanged (armor-98864f12 only moved the small-object baseline).
// Same shape as the pre-armor-98864f12 small-object pins: overlapped
// manifest-probe/HEAD pair, envelope header, trailer block table, data.
func TestOverThresholdObjectKeepsStreamingPath(t *testing.T) {
	const bucket = "test-bucket"
	const key = "over-threshold-v3.bin"
	plaintext := smallGETPlaintext(23 * 1024)

	cfg, cb, h, _ := countingTestSetup(t, 3, 0)
	putObjectThroughHandler(t, h, bucket, key, plaintext)

	storedLen := int64(len(cb.objects[bucket+"/"+key]))
	// One byte below the stored size: the object no longer qualifies.
	cfg.SmallObjectFetchBytes = storedLen - 1
	cb.resetCalls()

	got := getObjectThroughHandler(t, h, bucket, key)
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("GET did not round-trip the plaintext: got %d bytes, want %d", len(got), len(plaintext))
	}

	calls := cb.recordedCalls()
	if len(calls) != 5 {
		t.Fatalf("over-threshold v3: %d backend calls recorded, baseline is 5 (manifest probe ∥ head, header, trailer block table, data): %s",
			len(calls), formatCalls(calls))
	}
	assertProbePair(t, cb,
		backendCall{Method: "GET", Key: bucket + "/" + key + ".armor-manifest", Hit: false},
		backendCall{Method: "HEAD", Key: bucket + "/" + key, Hit: true},
		"over-threshold v3")
	header := calls[2]
	if header.Method != "GET_RANGE" || header.Offset != 0 || header.Length != crypto.HeaderSize {
		t.Fatalf("over-threshold v3: call 3 should be the envelope header read, got %+v", header)
	}
	blockCount := crypto.ComputeBlockCount(int64(len(plaintext)), 65536)
	trailer := calls[3]
	if trailer.Method != "GET_RANGE" || trailer.Length != int64(blockCount)*crypto.BlockTableEntrySize || trailer.Offset <= crypto.HeaderSize {
		t.Fatalf("over-threshold v3: call 4 should be the trailer block table read (%d entries), got %+v", blockCount, trailer)
	}
	data := calls[4]
	if data.Method != "GET_RANGE" || data.Offset != 0 || data.Length != storedLen-int64(blockCount)*crypto.BlockTableEntrySize {
		t.Fatalf("over-threshold v3: call 5 should be the header+data stream read, got %+v", data)
	}
}

// TestSmallObjectFullGETRequestCountMultipartV3 pins the backend request
// count for a full GET of a v3 multipart object: five calls total. The
// manifest hit races the plaintext-key probe HEAD (armor-420c15a5) whose
// result is discarded on the manifest path — the "one more total request"
// the overlap trades for one fewer sequential round trip on the legacy
// path — followed by the one authoritative ciphertext HEAD (armor-c3199f1c:
// the ADR-016 freshness check and the ciphertext-size lookup are a single
// round trip), the HMAC sidecar load, and ONE range read covering the whole
// concatenated ciphertext (armor-272299ca: storage has no part boundaries,
// so the pipelined reader streams the object in a single ranged request
// instead of one per-part fetch).
// Multipart objects cannot be small by construction (ADR-015 requires a
// uniform part size of at least 5 MiB), so this uses the smallest legal
// two-part shape; what is pinned is the fixed per-GET overhead.
func TestSmallObjectFullGETRequestCountMultipartV3(t *testing.T) {
	const bucket = "test-bucket"
	const key = "small-multipart-v3.bin"
	const mib = 1024 * 1024
	// Uniform part size P = 5 MiB + 512 (ADR-015 minimum) with a smaller
	// final part, the smallest legal two-part multipart object.
	part1 := smallGETPlaintext(5*mib + 512)
	part2 := smallGETPlaintext(33 * 1024)
	plaintext := append(append([]byte{}, part1...), part2...)

	_, cb, h, _ := countingTestSetup(t, 3, 0)
	uploadID := initiateMultipart(t, h, bucket, key)
	etag1 := uploadPart(t, h, bucket, key, uploadID, 1, part1)
	etag2 := uploadPart(t, h, bucket, key, uploadID, 2, part2)
	completeMultipart(t, h, bucket, key, uploadID, []string{etag1, etag2})
	cb.resetCalls()

	got := getObjectThroughHandler(t, h, bucket, key)
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("GET did not round-trip the multipart plaintext: got %d bytes, want %d", len(got), len(plaintext))
	}

	calls := cb.recordedCalls()
	// 4 fixed calls (probe pair, combined freshness/size HEAD, sidecar)
	// plus one whole-object GET_RANGE (armor-272299ca).
	const wantCount = 4 + 1
	if len(calls) != wantCount {
		t.Fatalf("v3 multipart: %d backend calls recorded, baseline is %d (manifest hit ∥ discarded plaintext-key probe head, combined freshness/size head, sidecar, one whole-object range): %s",
			len(calls), wantCount, formatCalls(calls))
	}
	// The probe pair races; the probe HEAD is a hit (the assembled ciphertext
	// object lives under the key itself) and its result is discarded.
	assertProbePair(t, cb,
		backendCall{Method: "GET", Key: bucket + "/" + key + ".armor-manifest", Hit: true},
		backendCall{Method: "HEAD", Key: bucket + "/" + key, Hit: true},
		"v3 multipart")
	if calls[2].Method != "HEAD" || !calls[2].Hit {
		t.Fatalf("v3 multipart: call 3 should be the combined ciphertext freshness/size head, got %+v", calls[2])
	}
	if calls[3].Method != "GET_DIRECT" || !calls[3].Hit {
		t.Fatalf("v3 multipart: call 4 should be the HMAC sidecar load, got %+v", calls[3])
	}
	data := calls[4]
	if data.Method != "GET_RANGE" || data.Offset != 0 || data.Length <= 0 {
		t.Fatalf("v3 multipart: call 5 should be the whole-object range read from offset 0, got %+v", data)
	}
}
