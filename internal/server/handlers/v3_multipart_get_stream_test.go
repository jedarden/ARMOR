package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// runV3MultipartGetHandler invokes handleV3MultipartGet against the shared
// v3 range fixture with an httptest recorder.
func runV3MultipartGetHandler(t *testing.T, fx *v3RangeFixture, beb backend.Backend) *httptest.ResponseRecorder {
	t.Helper()

	h := &Handlers{backend: beb, multipartSidecarCache: fx.cache}
	decryptor, err := crypto.NewDecryptor(fx.dek, fx.iv, fx.blockSize)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	armorMeta := &backend.ARMORMetadata{
		BlockSize:   fx.blockSize,
		IV:          fx.iv,
		ETag:        v3RangeETag,
		ContentType: "application/octet-stream",
	}

	r := httptest.NewRequest("GET", "/bucket/key", nil)
	w := httptest.NewRecorder()
	h.handleV3MultipartGet(w, r, "bucket", "key", "prefixed/key", decryptor, armorMeta, int64(len(fx.plaintext)), time.Now())
	return w
}

// TestV3MultipartGetStreamsWholeObjectInOneRange verifies the full-GET path
// addresses the whole concatenated ciphertext with a single ranged request —
// storage has no part boundaries, so the backend's pipelined reader owns the
// read-ahead — and returns the byte-exact plaintext with the sidecar-declared
// Content-Length.
func TestV3MultipartGetStreamsWholeObjectInOneRange(t *testing.T) {
	fx := newV3RangeFixture(t, []int{4, 4, 4}, 128)
	be := newRangeBackend(fx.ciphertext)

	w := runV3MultipartGetHandler(t, fx, be)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if got := w.Body.Bytes(); !bytes.Equal(got, fx.plaintext) {
		t.Fatalf("plaintext mismatch: got %d bytes, want %d; first divergence at %d",
			len(got), len(fx.plaintext), firstDivergence(got, fx.plaintext))
	}
	if got, want := w.Header().Get("Content-Length"), strconv.Itoa(len(fx.plaintext)); got != want {
		t.Errorf("Content-Length = %q, want %q", got, want)
	}
	if got := w.Header().Get("X-Armor-Stream"); got != "pipelined" {
		t.Errorf("X-Armor-Stream = %q, want pipelined", got)
	}

	calls := be.recordedCalls()
	if len(calls) != 1 {
		t.Fatalf("ranged requests = %d (%+v), want 1 — one request for the whole object", len(calls), calls)
	}
	if calls[0].offset != 0 || calls[0].length != int64(len(fx.ciphertext)) {
		t.Errorf("range call = %+v, want offset 0 length %d", calls[0], len(fx.ciphertext))
	}
}

// TestV3MultipartGetUnalignedPartsPartialFinalBlocksAndPartNamespace
// round-trips an object whose parts hold different, non-block-multiple block
// counts with a partial final block: every part's blocks must decrypt in the
// part-scoped (part, block) counter namespace — a global block index would
// fail HMAC verification for part 1+ and truncate the body.
func TestV3MultipartGetUnalignedPartsPartialFinalBlocksAndPartNamespace(t *testing.T) {
	fx := newV3RangeFixture(t, []int{3, 5, 1}, 128)
	be := newRangeBackend(fx.ciphertext)

	w := runV3MultipartGetHandler(t, fx, be)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if got := w.Body.Bytes(); !bytes.Equal(got, fx.plaintext) {
		t.Fatalf("plaintext mismatch: got %d bytes, want %d; first divergence at %d",
			len(got), len(fx.plaintext), firstDivergence(got, fx.plaintext))
	}
}

// TestV3MultipartGetCompressedBlocksRoundTrip covers per-block decompression
// in the stream: one incompressible block followed by a highly compressible
// one, the second stored with the sidecar compression flag set.
func TestV3MultipartGetCompressedBlocksRoundTrip(t *testing.T) {
	const blockSize = 65536

	dek := sha256.Sum256([]byte("armor-v3-get-compressed-dek"))
	iv := sha256.Sum256([]byte("armor-v3-get-compressed-iv"))

	plain0 := make([]byte, blockSize)
	for i := range plain0 {
		plain0[i] = byte(i*31) ^ byte(i>>7)
	}
	plain1 := bytes.Repeat([]byte{'A'}, blockSize)

	cipher0, hmac0, err := crypto.EncryptBlockV3(dek[:], iv[:16], 1, 0, plain0, blockSize)
	if err != nil {
		t.Fatalf("EncryptBlockV3 block 0: %v", err)
	}
	compressed1, wasCompressed, _, err := crypto.CompressBlock(plain1)
	if err != nil || !wasCompressed {
		t.Fatalf("CompressBlock: err=%v wasCompressed=%v, want compressed", err, wasCompressed)
	}
	cipher1, hmac1, err := crypto.EncryptBlockV3(dek[:], iv[:16], 1, 1, compressed1, blockSize)
	if err != nil {
		t.Fatalf("EncryptBlockV3 block 1: %v", err)
	}

	clen := func(n int, compressed bool) string {
		v := uint32(n)
		if compressed {
			v |= 0x80000000
		}
		raw := make([]byte, 4)
		binary.BigEndian.PutUint32(raw, v)
		return base64.StdEncoding.EncodeToString(raw)
	}
	b64 := func(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

	sidecar := &backend.HMACTableSidecarV3{Version: 3, BlockSize: blockSize}
	sidecar.Parts = append(sidecar.Parts, backend.HMACPartV3{
		N:             1,
		PlaintextLen:  int64(len(plain0) + len(plain1)),
		CiphertextLen: int64(len(cipher0) + len(cipher1)),
		Blocks: [][]string{
			{b64(hmac0), clen(len(cipher0), false)},
			{b64(hmac1), clen(len(cipher1), true)},
		},
	})

	cache := backend.NewMultipartSidecarCache(10, 60)
	if err := cache.Set("bucket", "prefixed/key", v3RangeETag, sidecar); err != nil {
		t.Fatalf("cache.Set: %v", err)
	}
	entry := cache.Get("bucket", "prefixed/key", v3RangeETag)

	var ciphertext []byte
	ciphertext = append(ciphertext, cipher0...)
	ciphertext = append(ciphertext, cipher1...)

	want := append(append([]byte(nil), plain0...), plain1...)
	fx := &v3RangeFixture{
		dek:        dek[:],
		iv:         iv[:16],
		blockSize:  blockSize,
		cache:      cache,
		entry:      entry,
		ciphertext: ciphertext,
		plaintext:  want,
	}

	w := runV3MultipartGetHandler(t, fx, newRangeBackend(ciphertext))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if got := w.Body.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("plaintext mismatch: got %d bytes, want %d", len(got), len(want))
	}
}

// chanWriter is an http.ResponseWriter that forwards every Write to a channel
// so a test can observe plaintext emission as it happens.
type chanWriter struct {
	header http.Header
	code   int
	writes chan []byte
}

func (c *chanWriter) Header() http.Header  { return c.header }
func (c *chanWriter) WriteHeader(code int) { c.code = code }
func (c *chanWriter) Write(p []byte) (int, error) {
	c.writes <- append([]byte(nil), p...)
	return len(p), nil
}

// gatedRangeBackend serves its ciphertext but parks reads that would deliver
// bytes past gateAt until the gate is released.
type gatedRangeBackend struct {
	*backend.NilBackend
	data   []byte
	gateAt int64
	gate   chan struct{}
}

func (b *gatedRangeBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 || offset+length > int64(len(b.data)) {
		return nil, fmt.Errorf("range %d+%d out of bounds (have %d bytes)", offset, length, len(b.data))
	}
	return &gatedRangeReader{data: b.data, pos: offset, end: offset + length, gateAt: b.gateAt, gate: b.gate}, nil
}

type gatedRangeReader struct {
	data   []byte
	pos    int64
	end    int64
	gateAt int64
	gate   chan struct{}
}

func (g *gatedRangeReader) Read(p []byte) (int, error) {
	if g.pos >= g.gateAt {
		<-g.gate
	}
	if g.pos >= g.end {
		return 0, io.EOF
	}
	n := copy(p, g.data[g.pos:min64(g.end, int64(len(g.data)))])
	g.pos += int64(n)
	return n, nil
}

func (g *gatedRangeReader) Close() error { return nil }

// firstDivergence returns the offset of the first byte where a and b differ,
// or the shorter length when one is a prefix of the other — for pinpointing
// where an assembled body diverged from the expected plaintext.
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

// TestV3MultipartGetYieldsVerifiedPlaintextBeforeFirstPartArrives pins the
// time-to-first-byte property: the backend parks before the first part's last
// ciphertext block has been served, and the first verified plaintext block
// must still reach the response writer. The whole-part implementation could
// not emit anything here.
func TestV3MultipartGetYieldsVerifiedPlaintextBeforeFirstPartArrives(t *testing.T) {
	const blockSize = 128
	fx := newV3RangeFixture(t, []int{4, 4, 4}, blockSize)

	gate := make(chan struct{})
	be := &gatedRangeBackend{data: fx.ciphertext, gateAt: 3 * blockSize, gate: gate}

	h := &Handlers{backend: be, multipartSidecarCache: fx.cache}
	decryptor, err := crypto.NewDecryptor(fx.dek, fx.iv, fx.blockSize)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	armorMeta := &backend.ARMORMetadata{
		BlockSize:   fx.blockSize,
		IV:          fx.iv,
		ETag:        v3RangeETag,
		ContentType: "application/octet-stream",
	}

	w := &chanWriter{header: http.Header{}, writes: make(chan []byte, 64)}
	done := make(chan struct{})
	go func() {
		defer close(done)
		r := httptest.NewRequest("GET", "/bucket/key", nil)
		h.handleV3MultipartGet(w, r, "bucket", "key", "prefixed/key", decryptor, armorMeta, int64(len(fx.plaintext)), time.Now())
	}()

	// body accumulates every emission, starting with the TTFB block: the
	// gated-out first emission is part of the object too.
	var body []byte
	select {
	case got := <-w.writes:
		if len(got) != blockSize {
			t.Fatalf("first emission = %d bytes, want one %d-byte block", len(got), blockSize)
		}
		body = append(body, got...)
	case <-time.After(5 * time.Second):
		t.Fatal("no verified plaintext emitted before the first part finished arriving")
	}

	// Release the gate and drain the rest: the parked backend finishes, and
	// the assembled body must still be byte-exact.
	close(gate)
	deadline := time.After(5 * time.Second)
	for int64(len(body)) < int64(len(fx.plaintext)) {
		select {
		case b := <-w.writes:
			body = append(body, b...)
		case <-deadline:
			t.Fatalf("stream stalled after %d of %d bytes", len(body), len(fx.plaintext))
		}
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not return after the stream completed")
	}
	if !bytes.Equal(body, fx.plaintext) {
		t.Fatalf("assembled body mismatch: got %d bytes, want %d", len(body), len(fx.plaintext))
	}
}

// truncRangeBackend serves the first serveBytes ciphertext bytes, then either
// errors or truncates, simulating a backend failure or corruption that
// surfaces mid-stream — after the response headers are already sent.
type truncRangeBackend struct {
	*backend.NilBackend
	data       []byte
	serveBytes int64
	failWith   error // non-nil: reader returns this error; nil: clean EOF
}

func (b *truncRangeBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	if offset != 0 {
		return nil, fmt.Errorf("unexpected offset %d", offset)
	}
	return &truncRangeReader{data: b.data, end: b.serveBytes, failWith: b.failWith}, nil
}

type truncRangeReader struct {
	data     []byte
	pos      int64
	end      int64
	failWith error
}

func (r *truncRangeReader) Read(p []byte) (int, error) {
	if r.pos >= r.end {
		if r.failWith != nil {
			return 0, r.failWith
		}
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:min64(r.end, r.pos+int64(len(p)))])
	r.pos += int64(n)
	return n, nil
}

func (r *truncRangeReader) Close() error { return nil }

// TestV3MultipartGetBackendErrorAfterHeadersTruncatesObservably pins the
// mid-stream failure contract: with the headers (and Content-Length) already
// sent, a backend read failure leaves a short body against the declared
// length — the correct prefix of the plaintext, never an XML error body
// appended to partial plaintext, never a clean truncated success.
func TestV3MultipartGetBackendErrorAfterHeadersTruncatesObservably(t *testing.T) {
	fx := newV3RangeFixture(t, []int{4, 4}, 128)
	const serveBytes = 200 // block 0 fully served, block 1 cut short
	be := &truncRangeBackend{data: fx.ciphertext, serveBytes: serveBytes, failWith: fmt.Errorf("backend exploded")}

	w := runV3MultipartGetHandler(t, fx, be)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d after mid-stream backend error, want 200 (headers were already sent)", w.Code)
	}
	if got, want := w.Header().Get("Content-Length"), strconv.Itoa(len(fx.plaintext)); got != want {
		t.Fatalf("Content-Length = %q, want %q", got, want)
	}
	body := w.Body.Bytes()
	if len(body) == 0 || int64(len(body)) >= int64(len(fx.plaintext)) {
		t.Fatalf("body = %d bytes, want a non-empty truncation of %d", len(body), len(fx.plaintext))
	}
	if !bytes.Equal(body, fx.plaintext[:len(body)]) {
		t.Fatal("emitted plaintext is not the correct prefix of the object")
	}
	if strings.Contains(w.Body.String(), "<Error") {
		t.Fatalf("XML error body appended to partial plaintext: %q", w.Body.String()[:min(200, w.Body.Len())])
	}
}

// TestV3MultipartGetCorruptionAfterHeadersTruncatesObservably flips ciphertext
// bytes inside a middle block: HMAC verification must stop the stream there,
// emitting only the verified prefix.
func TestV3MultipartGetCorruptionAfterHeadersTruncatesObservably(t *testing.T) {
	fx := newV3RangeFixture(t, []int{4, 4}, 128)
	corrupt := append([]byte(nil), fx.ciphertext...)
	const corruptAt = 2*128 + 10 // block 2 of part 0
	corrupt[corruptAt] ^= 0xFF
	be := newRangeBackend(corrupt)

	w := runV3MultipartGetHandler(t, fx, be)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d after mid-stream corruption, want 200 (headers were already sent)", w.Code)
	}
	body := w.Body.Bytes()
	if want := 2 * 128; len(body) != want {
		t.Fatalf("body = %d bytes, want only the %d verified bytes before the corrupt block", len(body), want)
	}
	if !bytes.Equal(body, fx.plaintext[:len(body)]) {
		t.Fatal("emitted plaintext is not the correct prefix of the object")
	}
	if strings.Contains(w.Body.String(), "<Error") {
		t.Fatal("XML error body appended to partial plaintext")
	}
}

// bodyCloseRecorder wraps a reader and signals when Close is called, so tests
// can assert the backend reader is released.
type closeSignalReader struct {
	io.ReadCloser
	once   sync.Once
	closed chan struct{}
}

func (c *closeSignalReader) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.ReadCloser.Close()
}

type closingRangeBackend struct {
	*backend.NilBackend
	data   []byte
	closed chan struct{}
}

func (b *closingRangeBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	return &closeSignalReader{
		ReadCloser: io.NopCloser(bytes.NewReader(b.data[offset : offset+length])),
		closed:     b.closed,
	}, nil
}

// TestStreamV3MultipartBlocksUnblocksWhenWriterCloses pins the goroutine and
// buffer release on the client-disconnect path: when the consumer disappears
// (the handler closes the pipe reader), a producer parked on a block write
// must exit and close the backend reader.
func TestStreamV3MultipartBlocksUnblocksWhenWriterCloses(t *testing.T) {
	fx := newV3RangeFixture(t, []int{4}, 128)

	closed := make(chan struct{})
	be := &closingRangeBackend{data: fx.ciphertext, closed: closed}
	body, err := be.GetRange(context.Background(), "bucket", "prefixed/key", 0, int64(len(fx.ciphertext)))
	if err != nil {
		t.Fatalf("GetRange: %v", err)
	}

	decryptor, err := crypto.NewDecryptor(fx.dek, fx.iv, fx.blockSize)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}

	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		defer body.Close()
		done <- streamV3MultipartBlocks(pw, body, fx.entry, decryptor, fx.iv, fx.blockSize, int64(len(fx.plaintext)))
	}()

	// Consume one block, then vanish the way the handler does on a client
	// disconnect: close the pipe reader while the producer continues.
	buf := make([]byte, 128)
	if _, err := io.ReadFull(pr, buf); err != nil {
		t.Fatalf("read first block: %v", err)
	}
	pr.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("producer goroutine still parked after the pipe reader closed")
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("backend reader not closed after the producer exited")
	}
}

// slowRangeBackend serves its ciphertext in small chunks with a delay, and its
// reads abort when the request context is cancelled — mirroring how a real
// backend's in-flight range reads die with the request.
type slowRangeBackend struct {
	*backend.NilBackend
	data      []byte
	delay     time.Duration
	releaseMu sync.Mutex
	releaseCh chan struct{}
}

// released is closed once a reader this backend served has been Closed.
func (b *slowRangeBackend) released() <-chan struct{} {
	b.releaseMu.Lock()
	defer b.releaseMu.Unlock()
	if b.releaseCh == nil {
		b.releaseCh = make(chan struct{})
	}
	return b.releaseCh
}

func (b *slowRangeBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	b.releaseMu.Lock()
	defer b.releaseMu.Unlock()
	if b.releaseCh == nil {
		b.releaseCh = make(chan struct{})
	}
	return &slowRangeReader{ctx: ctx, data: b.data, pos: offset, end: offset + length, delay: b.delay, release: b.releaseCh}, nil
}

type slowRangeReader struct {
	ctx     context.Context
	data    []byte
	pos     int64
	end     int64
	delay   time.Duration
	release chan struct{}
	once    sync.Once
}

func (r *slowRangeReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	case <-time.After(r.delay):
	}
	if r.pos >= r.end {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:min64(r.end, r.pos+int64(len(p)))])
	r.pos += int64(n)
	return n, nil
}

// Close is the release signal: the handler's streaming goroutine owns the
// reader and defers its Close, so a closed reader proves the goroutine
// stopped — on the cancellation path and the completion path alike.
func (r *slowRangeReader) Close() error {
	r.once.Do(func() { close(r.release) })
	return nil
}

// TestV3MultipartGetClientCancellationStopsStreamAndReleasesBackend runs the
// handler behind a real HTTP server and cancels the client mid-body: the
// streaming goroutine must stop and release the backend reader instead of
// draining the whole object into a dead connection.
func TestV3MultipartGetClientCancellationStopsStreamAndReleasesBackend(t *testing.T) {
	fx := newV3RangeFixture(t, []int{8, 8}, 128)
	be := &slowRangeBackend{NilBackend: backend.NewNilBackend(), data: fx.ciphertext, delay: 10 * time.Millisecond}

	h := &Handlers{backend: be, multipartSidecarCache: fx.cache}
	decryptor, err := crypto.NewDecryptor(fx.dek, fx.iv, fx.blockSize)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	armorMeta := &backend.ARMORMetadata{
		BlockSize:   fx.blockSize,
		IV:          fx.iv,
		ETag:        v3RangeETag,
		ContentType: "application/octet-stream",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleV3MultipartGet(w, r, "bucket", "key", "prefixed/key", decryptor, armorMeta, int64(len(fx.plaintext)), time.Now())
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	// Read a little, then walk away.
	head := make([]byte, 100)
	if _, err := io.ReadFull(resp.Body, head); err != nil {
		t.Fatalf("read head: %v", err)
	}
	cancel()
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// The backend reader must be released without streaming the rest.
	select {
	case <-be.released():
	case <-time.After(5 * time.Second):
		t.Fatal("backend reader still open 5s after client cancellation")
	}
}

// TestV3MultipartGetWholeObjectAddressBeyondUint32 pins full-GET addressing at
// cumulative ciphertext sizes past 2^32: the single range must carry the full
// 64-bit total (the previous repeated uint32 accumulator wrapped at 4GiB and
// fetched the wrong bytes, armor-817d9d92).
func TestV3MultipartGetWholeObjectAddressBeyondUint32(t *testing.T) {
	const partCiphertextLen = int64(1) << 30 // 1GiB per part
	const partCount = 6                      // cumulative 6GiB: past both 2^32 and B2's 5GiB part ceiling

	sidecar := &backend.HMACTableSidecarV3{Version: 3, BlockSize: 65536}
	clen := make([]byte, 4)
	binary.BigEndian.PutUint32(clen, uint32(partCiphertextLen))
	block := []string{
		base64.StdEncoding.EncodeToString(make([]byte, 32)),
		base64.StdEncoding.EncodeToString(clen),
	}
	for p := 1; p <= partCount; p++ {
		sidecar.Parts = append(sidecar.Parts, backend.HMACPartV3{
			N:             p,
			PlaintextLen:  partCiphertextLen,
			CiphertextLen: partCiphertextLen,
			Blocks:        [][]string{block},
		})
	}

	cache := backend.NewMultipartSidecarCache(10, 60)
	if err := cache.Set("bucket", "prefixed/key", v3RangeETag, sidecar); err != nil {
		t.Fatalf("cache.Set: %v", err)
	}
	entry := cache.Get("bucket", "prefixed/key", v3RangeETag)
	// The checked int64 cumulative offsets must already carry the >5GiB
	// total: the handler's single whole-object range is addressed from them,
	// so a uint32-class wrap here would fetch the wrong bytes.
	total, err := entry.TotalCiphertextSize()
	if err != nil {
		t.Fatalf("TotalCiphertextSize: %v", err)
	}
	if want := partCiphertextLen * partCount; total != want {
		t.Fatalf("TotalCiphertextSize = %d, want %d", total, want)
	}

	be := &offsetCaptureBackend{NilBackend: backend.NewNilBackend()}
	h := &Handlers{backend: be, multipartSidecarCache: cache}

	armorMeta := &backend.ARMORMetadata{
		BlockSize:   65536,
		IV:          make([]byte, 16),
		ETag:        v3RangeETag,
		ContentType: "application/octet-stream",
	}
	r := httptest.NewRequest("GET", "/bucket/key", nil)
	w := httptest.NewRecorder()
	decryptor, err := crypto.NewDecryptor(make([]byte, 32), make([]byte, 16), 65536)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	h.handleV3MultipartGet(w, r, "bucket", "key", "prefixed/key", decryptor, armorMeta, partCiphertextLen*partCount, time.Now())

	offsets, lengths := be.recorded()
	if len(offsets) != 1 {
		t.Fatalf("ranged requests = %d, want 1", len(offsets))
	}
	wantTotal := partCiphertextLen * partCount
	if offsets[0] != 0 || lengths[0] != wantTotal {
		t.Errorf("range call = offset %d length %d, want offset 0 length %d", offsets[0], lengths[0], wantTotal)
	}
	if wantTotal <= int64(1)<<32 {
		t.Fatalf("test setup: total %d does not exceed 2^32", wantTotal)
	}
}
