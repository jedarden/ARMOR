package handlers_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// replayablePutBackend emulates the SDK's signing pass followed by an upload
// pass. A non-seekable body would make B2Backend.put write a second temp file.
type replayablePutBackend struct {
	*mockBackend
	putCalls          int
	conditionalCalls  int
	seekable          bool
	replayed          bool
	plaintextReads    int64
	rejectConditional bool
	cancelOnPut       func()
	earlySuccess      bool
}

func (b *replayablePutBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	b.putCalls++
	seeker, ok := body.(io.ReadSeeker)
	b.seekable = ok
	if !ok {
		return errors.New("backend received a non-seekable encrypted body")
	}
	if b.earlySuccess {
		return nil
	}
	if b.cancelOnPut != nil {
		buf := make([]byte, 64<<10)
		if n, err := seeker.Read(buf); err != nil || n <= 64 {
			return fmt.Errorf("cancellation probe read %d bytes: %v", n, err)
		}
		b.cancelOnPut()
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	first, err := io.ReadAll(seeker)
	if err != nil {
		return err
	}
	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		return err
	}
	second, err := io.ReadAll(seeker)
	if err != nil {
		return err
	}
	if int64(len(first)) != size || !bytes.Equal(first, second) {
		return fmt.Errorf("encrypted replay changed or length differed: first=%d second=%d declared=%d", len(first), len(second), size)
	}
	b.replayed = true
	if stats, ok := body.(interface{ PlaintextBytesRead() int64 }); ok {
		b.plaintextReads = stats.PlaintextBytesRead()
	}
	return b.mockBackend.Put(ctx, bucket, key, bytes.NewReader(second), size, meta)
}

func (b *replayablePutBackend) PutIfAbsent(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	b.conditionalCalls++
	if b.rejectConditional {
		return backend.ErrPreconditionFailed
	}
	return b.Put(ctx, bucket, key, body, size, meta)
}

func replayableTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	return dir
}

func assertNoReplayableTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("upload left %d temporary files in %s", len(entries), dir)
	}
}

func TestReplayableStreamingV3FullAndRangeRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name          string
		unknownLength bool
	}{
		{name: "known-length"},
		{name: "unknown-length", unknownLength: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := replayableTempDir(t)
			cfg, base, cache, footerCache, _ := testSetupWithPrefix(t, "tenant/")
			cfg.FormatWriteVersion = 3
			routes, err := keymanager.ParseKeyRoutes("data/pii/*=pii,*=default")
			if err != nil {
				t.Fatal(err)
			}
			km, err := keymanager.New(testMEK(0x11), map[string][]byte{"pii": testMEK(0x22)}, routes)
			if err != nil {
				t.Fatal(err)
			}
			storage := &replayablePutBackend{mockBackend: base}
			h := handlers.New(cfg, storage, cache, footerCache, km, nil)
			pattern := []byte("replayable-v3-block-boundary-")
			payload := bytes.Repeat(pattern, (11<<20+17)/len(pattern)+1)
			payload = payload[:11<<20+17]
			const key = "data/pii/large.bin"
			req := httptest.NewRequest(http.MethodPut, "/test-bucket/"+key, bytes.NewReader(payload))
			if tc.unknownLength {
				req.ContentLength = -1
			}
			put := httptest.NewRecorder()
			h.HandleRoot(put, req)
			if put.Code != http.StatusOK || put.Header().Get("X-Armor-Streaming") != "true" {
				t.Fatalf("PUT = %d streaming=%q: %s", put.Code, put.Header().Get("X-Armor-Streaming"), put.Body.String())
			}
			if !storage.seekable || !storage.replayed || storage.putCalls != 1 {
				t.Fatalf("backend saw seekable=%t replayed=%t putCalls=%d", storage.seekable, storage.replayed, storage.putCalls)
			}
			storedKey := "test-bucket/tenant/" + key
			base.mu.Lock()
			stored, ok := base.objects[storedKey]
			meta := base.meta[storedKey]
			base.mu.Unlock()
			if !ok || len(stored) < 64 || stored[4] != 3 {
				t.Fatalf("prefixed v3 ciphertext missing or malformed: present=%t bytes=%d", ok, len(stored))
			}
			if got := meta["x-amz-meta-armor-key-id"]; got != "pii" {
				t.Fatalf("named-key metadata = %q, want pii", got)
			}
			get := httptest.NewRecorder()
			h.HandleRoot(get, httptest.NewRequest(http.MethodGet, "/test-bucket/"+key, nil))
			if get.Code != http.StatusOK || !bytes.Equal(get.Body.Bytes(), payload) {
				t.Fatalf("full GET = %d bytes=%d, want %d", get.Code, get.Body.Len(), len(payload))
			}
			const lo, hi = 65530, 131100
			rangeReq := httptest.NewRequest(http.MethodGet, "/test-bucket/"+key, nil)
			rangeReq.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", lo, hi))
			ranged := httptest.NewRecorder()
			h.HandleRoot(ranged, rangeReq)
			if ranged.Code != http.StatusPartialContent || !bytes.Equal(ranged.Body.Bytes(), payload[lo:hi+1]) {
				t.Fatalf("range GET = %d bytes=%d, want %d", ranged.Code, ranged.Body.Len(), hi-lo+1)
			}
			assertNoReplayableTempFiles(t, tmpDir)
		})
	}
}

func TestReplayableStreamingConditionalRejectionCleansSpool(t *testing.T) {
	tmpDir := replayableTempDir(t)
	cfg, base, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3
	storage := &replayablePutBackend{mockBackend: base, rejectConditional: true}
	h := handlers.New(cfg, storage, cache, footerCache, km, nil)
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/create-only", bytes.NewReader([]byte("new body")))
	req.ContentLength = -1
	req.Header.Set("If-None-Match", "*")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusPreconditionFailed || storage.conditionalCalls != 1 || storage.putCalls != 0 {
		t.Fatalf("conditional rejection = %d, conditional=%d put=%d: %s", w.Code, storage.conditionalCalls, storage.putCalls, w.Body.String())
	}
	assertNoReplayableTempFiles(t, tmpDir)
}

func TestReplayableStreamingShortKnownLengthDoesNotAcknowledge(t *testing.T) {
	tmpDir := replayableTempDir(t)
	cfg, base, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3
	storage := &replayablePutBackend{mockBackend: base}
	h := handlers.New(cfg, storage, cache, footerCache, km, nil)
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/short", bytes.NewReader([]byte("short")))
	req.ContentLength = 11 << 20
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code == http.StatusOK || storage.putCalls != 0 {
		t.Fatalf("short body acknowledged or uploaded: status=%d puts=%d", w.Code, storage.putCalls)
	}
	assertNoReplayableTempFiles(t, tmpDir)
}

type failedUploadReader struct{ sent bool }

func (r *failedUploadReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, "partial body"), nil
	}
	return 0, io.ErrUnexpectedEOF
}

func TestReplayableStreamingBodyErrorCleansSpool(t *testing.T) {
	tmpDir := replayableTempDir(t)
	cfg, base, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3
	storage := &replayablePutBackend{mockBackend: base}
	h := handlers.New(cfg, storage, cache, footerCache, km, nil)
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/read-error", &failedUploadReader{})
	req.ContentLength = -1
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code == http.StatusOK || storage.putCalls != 0 {
		t.Fatalf("failed input acknowledged or uploaded: status=%d puts=%d", w.Code, storage.putCalls)
	}
	assertNoReplayableTempFiles(t, tmpDir)
}

// A file-size limit makes the plaintext spool write fail without exhausting
// the host's disk. Run it in a child so the process limit cannot affect other
// tests; the handler must reject the upload and unlink the partial spool.
func TestReplayableStreamingSpoolWriteFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("file-size limit probe uses Linux signals")
	}
	if os.Getenv("ARMOR_TEST_SPOOL_LIMIT_CHILD") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestReplayableStreamingSpoolWriteFailure$")
		cmd.Env = append(os.Environ(), "ARMOR_TEST_SPOOL_LIMIT_CHILD=1", "TMPDIR="+t.TempDir())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("limited child failed: %v\n%s", err, out)
		}
		return
	}
	tmpDir := replayableTempDir(t)
	signal.Ignore(syscall.SIGXFSZ)
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 64 << 10, Max: 64 << 10}); err != nil {
		t.Fatal(err)
	}
	cfg, base, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3
	storage := &replayablePutBackend{mockBackend: base}
	h := handlers.New(cfg, storage, cache, footerCache, km, nil)
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/disk-limit", &repeatByteReader{remaining: 2 << 20, value: 0x23})
	req.ContentLength = -1
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code == http.StatusOK || storage.putCalls != 0 {
		t.Fatalf("failed spool acknowledged or uploaded: status=%d puts=%d", w.Code, storage.putCalls)
	}
	assertNoReplayableTempFiles(t, tmpDir)
}

func TestReplayableStreamingCanceledBackendDoesNotAcknowledge(t *testing.T) {
	tmpDir := replayableTempDir(t)
	cfg, base, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	storage := &replayablePutBackend{mockBackend: base, cancelOnPut: cancel}
	h := handlers.New(cfg, storage, cache, footerCache, km, nil)
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/canceled", bytes.NewReader([]byte("body"))).WithContext(ctx)
	req.ContentLength = -1
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code == http.StatusOK || storage.putCalls != 1 || !storage.seekable {
		t.Fatalf("canceled backend write = %d puts=%d seekable=%t", w.Code, storage.putCalls, storage.seekable)
	}
	base.mu.Lock()
	_, stored := base.objects["test-bucket/canceled"]
	base.mu.Unlock()
	if stored {
		t.Fatal("canceled object was stored")
	}
	assertNoReplayableTempFiles(t, tmpDir)
}

func TestReplayableStreamingRejectsIncompleteBackendConsumption(t *testing.T) {
	tmpDir := replayableTempDir(t)
	cfg, base, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3
	storage := &replayablePutBackend{mockBackend: base, earlySuccess: true}
	h := handlers.New(cfg, storage, cache, footerCache, km, nil)
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/incomplete-backend", bytes.NewReader([]byte("body")))
	req.ContentLength = -1
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code == http.StatusOK || storage.putCalls != 1 {
		t.Fatalf("incomplete backend read acknowledged: status=%d puts=%d", w.Code, storage.putCalls)
	}
	assertNoReplayableTempFiles(t, tmpDir)
}

type replayableAccountingBackend struct {
	*mockBackend
	start              time.Time
	firstPayloadAt     time.Duration
	ciphertextBytes    int64
	plaintextReadBytes int64
}

func (b *replayableAccountingBackend) Put(ctx context.Context, _, _ string, body io.Reader, size int64, _ map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	seeker, ok := body.(io.ReadSeeker)
	if !ok {
		return errors.New("64 MiB backend received a non-seekable body")
	}
	buf := make([]byte, 32<<10)
	n, err := seeker.Read(buf)
	if err != nil {
		return err
	}
	if n <= 64 {
		return fmt.Errorf("first backend read returned only %d bytes", n)
	}
	b.firstPayloadAt = time.Since(b.start)
	rest, err := io.Copy(io.Discard, seeker)
	if err != nil {
		return err
	}
	b.ciphertextBytes = int64(n) + rest
	if b.ciphertextBytes != size {
		return fmt.Errorf("ciphertext bytes = %d, declared %d", b.ciphertextBytes, size)
	}
	if stats, ok := body.(interface{ PlaintextBytesRead() int64 }); ok {
		b.plaintextReadBytes = stats.PlaintextBytesRead()
	}
	return nil
}

// This local probe measures the handler and a discarding backend. It is not a
// B2 throughput measurement; it pins spool accounting and first payload time.
func TestReplayableStreaming64MiBLocalAccounting(t *testing.T) {
	tmpDir := replayableTempDir(t)
	cfg, base, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3
	storage := &replayableAccountingBackend{mockBackend: base}
	h := handlers.New(cfg, storage, cache, footerCache, km, nil)
	const size = 64 << 20
	// A generated reader avoids a 64 MiB test fixture allocation while
	// keeping a deterministic full-size request body.
	readBody := &repeatByteReader{remaining: size, value: 0x5a}
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/accounting", readBody)
	req.ContentLength = size
	w := httptest.NewRecorder()
	storage.start = time.Now()
	h.HandleRoot(w, req)
	elapsed := time.Since(storage.start)
	if w.Code != http.StatusOK {
		t.Fatalf("64 MiB PUT = %d: %s", w.Code, w.Body.String())
	}
	if storage.plaintextReadBytes != size {
		t.Fatalf("plaintext spool bytes read = %d, want %d", storage.plaintextReadBytes, size)
	}
	if readBody.remaining != 0 {
		t.Fatalf("request body has %d bytes remaining", readBody.remaining)
	}
	if storage.firstPayloadAt <= 0 || storage.ciphertextBytes <= size {
		t.Fatalf("first payload time=%s ciphertext bytes=%d", storage.firstPayloadAt, storage.ciphertextBytes)
	}
	t.Logf("local 64 MiB: plaintext spool bytes written=%d (request copy), read=%d (reader counter), ciphertext temp bytes written=0 (seekable body), first backend payload at=%s, elapsed=%s", size, storage.plaintextReadBytes, storage.firstPayloadAt, elapsed)
	assertNoReplayableTempFiles(t, tmpDir)
}

type repeatByteReader struct {
	remaining int64
	value     byte
}

func (r *repeatByteReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > r.remaining {
		n = int(r.remaining)
	}
	for i := range p[:n] {
		p[i] = r.value
	}
	r.remaining -= int64(n)
	return n, nil
}
