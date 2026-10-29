package server

// Bind-first cold-load coverage (armor-3b7595b6, plan §9.6): the manifest
// cold load runs in the background while both listeners serve, object
// operations fall back to B2, and the load's error, cancellation, and
// success outcomes never expose a partial index or publish a colliding
// delta.
//
// The gate backend holds only the cold load's store fetches (snapshot and
// delta bodies under the manifest store prefix) on a channel — exactly the
// "slow backend" the plan describes. Listings, object reads, per-key
// manifests, and writes pass through untouched, so the tests observe what a
// pod under a slow backend actually serves: bound listeners, health and
// readiness answers, and correct object data via the fallback paths.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/manifest"
)

// gatingBackend wraps a real backend and holds every fetch of a manifest
// store object on a channel until released. releaseErr, when set, makes held
// fetches fail the moment they are released instead of delegating — the
// transient that lands the load in the sequence-floor fallback. Every Put is
// recorded so tests can prove no colliding delta PUT ever happens.
type gatingBackend struct {
	backend.Backend
	manifestPrefix string
	hold           chan struct{}
	releaseErr     error
	releaseOnce    sync.Once
	fetches        atomic.Int64
	putMu          sync.Mutex
	putKeys        []string
}

func newGatingBackend(be backend.Backend, manifestPrefix string, releaseErr error) *gatingBackend {
	return &gatingBackend{Backend: be, manifestPrefix: manifestPrefix, hold: make(chan struct{}), releaseErr: releaseErr}
}

// gate blocks manifest-store fetches until release; everything else passes.
func (b *gatingBackend) gate(ctx context.Context, key string) error {
	if !strings.HasPrefix(key, b.manifestPrefix+"/") {
		return nil
	}
	b.fetches.Add(1)
	select {
	case <-b.hold:
	case <-ctx.Done():
		return ctx.Err()
	}
	return b.releaseErr
}

func (b *gatingBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	if err := b.gate(ctx, key); err != nil {
		return nil, nil, err
	}
	return b.Backend.Get(ctx, bucket, key)
}

func (b *gatingBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	if err := b.gate(ctx, key); err != nil {
		return nil, nil, err
	}
	return b.Backend.GetDirect(ctx, bucket, key)
}

func (b *gatingBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	b.putMu.Lock()
	b.putKeys = append(b.putKeys, key)
	b.putMu.Unlock()
	return b.Backend.Put(ctx, bucket, key, body, size, meta)
}

// release opens the gate. Idempotent, so cleanups can call it freely.
func (b *gatingBackend) release() {
	b.releaseOnce.Do(func() { close(b.hold) })
}

func (b *gatingBackend) recordedPuts() []string {
	b.putMu.Lock()
	defer b.putMu.Unlock()
	return append([]string(nil), b.putKeys...)
}

// waitHeldFetches blocks until the cold load has reached (and is stuck on) at
// least one store fetch, proving the load is genuinely in flight.
func (b *gatingBackend) waitHeldFetches(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for b.fetches.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("cold load never reached a manifest-store fetch")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

const asyncLoadTestMEK = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
const asyncLoadTestAccessKey = "test-access-key"
const asyncLoadTestSecretKey = "test-secret-key"

// newAsyncColdLoadServer builds the production manifest wiring — the same
// manifestSystem.start call server.New makes — around an injectable backend,
// so a test can gate the cold load while the real S3 and admin muxes serve.
func newAsyncColdLoadServer(t *testing.T, root string, be backend.Backend) *Server {
	t.Helper()
	pairs := []string{
		"ARMOR_BACKEND", "filesystem",
		"ARMOR_FS_PATH", root,
		"ARMOR_BUCKET", "b",
		"ARMOR_MEK", asyncLoadTestMEK,
		"ARMOR_AUTH_ACCESS_KEY", asyncLoadTestAccessKey,
		"ARMOR_AUTH_SECRET_KEY", asyncLoadTestSecretKey,
		"ARMOR_WRITER_ID", "w1",
		"ARMOR_CANARY_DISABLED", "true",
		"ARMOR_MANIFEST_LOAD_TIMEOUT", "60",
		// Keep the load's retry policy fast: the error-leg tests fail every
		// attempt and must not spend the default backoff between them.
		"ARMOR_READ_RETRY_ATTEMPTS", "3",
		"ARMOR_READ_RETRY_BACKOFF", "1ms",
		"ARMOR_READ_RETRY_MAX_BACKOFF", "1ms",
	}
	originals := make(map[string]string, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		k, v := pairs[i], pairs[i+1]
		originals[k] = os.Getenv(k)
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("Setenv(%s): %v", k, err)
		}
	}
	t.Cleanup(func() {
		for k, v := range originals {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load(): %v", err)
	}
	if cfg.ManifestPrefix != ".armor/manifest" {
		t.Fatalf("got ManifestPrefix %q, want %q", cfg.ManifestPrefix, ".armor/manifest")
	}

	srv, err := NewWithBackend(cfg, be)
	if err != nil {
		t.Fatalf("NewWithBackend(): %v", err)
	}
	// The one wiring NewWithBackend omits by design: install the manifest
	// subsystem exactly as New does. This is the production start path under
	// test — it must return before the load resolves.
	sys := newManifestSystem()
	sys.start(cfg, logging.New("test"), be)
	srv.manifestSys = sys
	return srv
}

// signedS3Request drives one SigV4-authenticated request through a listener.
func signedS3Request(t *testing.T, method, url string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest(%s %s): %v", method, url, err)
	}
	signRequest(t, req, body, asyncLoadTestAccessKey, asyncLoadTestSecretKey, time.Now().UTC())
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// waitForStatus blocks until the manifest subsystem publishes want or the
// bound passes.
func waitForStatus(t *testing.T, srv *Server, want manifestStatus) {
	t.Helper()
	if got := srv.WaitForManifestLoad(30 * time.Second); got != want {
		t.Fatalf("manifest load settled as %q, want %q", got, want)
	}
}

// manifestStoreKeys returns the census entries under the manifest store.
func manifestStoreKeys(census map[string]string) []string {
	var keys []string
	for k := range census {
		if strings.HasPrefix(k, "b/.armor/manifest/") {
			keys = append(keys, k)
		}
	}
	return keys
}

// assertSeededDeltasIntact checks the persisted deltas survived byte for byte.
func assertSeededDeltasIntact(t *testing.T, before, after map[string]string, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		key := "b/" + manifest.DeltaKey(".armor/manifest", "w1", uint64(i))
		want, ok := before[key]
		if !ok {
			continue
		}
		got, ok := after[key]
		if !ok {
			t.Fatalf("persisted delta %s vanished", key)
		}
		if got != want {
			t.Fatalf("persisted delta %s was overwritten", key)
		}
	}
}

// assertNoCollidingDeltaPUTs fails when any recorded PUT targeted a seeded
// delta rather than a key above the floor.
func assertNoCollidingDeltaPUTs(t *testing.T, puts []string, seeded int) {
	t.Helper()
	for _, k := range puts {
		for i := 1; i <= seeded; i++ {
			if k == manifest.DeltaKey(".armor/manifest", "w1", uint64(i)) {
				t.Fatalf("colliding manifest PUT to pre-existing delta %s", k)
			}
		}
	}
}

// TestColdLoadAsyncListenersServeWhileLoadHeld is the bead's hermetic
// slow-fetch leg: with the cold load stuck on manifest-store fetches, both
// listeners bind, health and readiness answer, object PUT/GET/HEAD serve
// correct data through the B2 fallback paths, no partial index is exposed,
// and — once released — the load swaps in the full index while the write
// recorded mid-load replays and publishes above the loaded sequence.
func TestColdLoadAsyncListenersServeWhileLoadHeld(t *testing.T) {
	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	seedPersistedDeltas(t, fs, 3)
	before := storeCensus(t, root)

	gating := newGatingBackend(fs, ".armor/manifest", nil)
	srv := newAsyncColdLoadServer(t, root, gating)

	// The production start path returned while the load was still running —
	// in cmd_serve the listeners bind immediately after; here the muxes are
	// the same ones ListenAndServe would serve.
	if got := srv.manifestSys.status(); got != manifestLoading {
		t.Fatalf("status right after start = %q, want %q", got, manifestLoading)
	}
	gating.waitHeldFetches(t)

	// Both listeners bind while the load is held.
	s3Listener := httptest.NewServer(srv.Handler())
	t.Cleanup(s3Listener.Close)
	adminListener := httptest.NewServer(srv.AdminHandler())
	t.Cleanup(adminListener.Close)

	// Un-gate and stop everything at teardown, in that order.
	t.Cleanup(func() {
		gating.release()
		srv.StopManifestWriter()
		srv.StopManifestCompactor()
	})

	// No partial index exposure: the store holds three deltas, the load has
	// fetched (and is stuck mid-fetch), yet the live index is empty and the
	// recorder answers lookups as misses.
	if got := srv.manifestIndex().Len(); got != 0 {
		t.Fatalf("live index holds %d entries while loading, want 0", got)
	}
	rec := &manifestRecorder{sys: srv.manifestSys}
	if _, ok := rec.Lookup("b", "obj-1"); ok {
		t.Fatal("lookup answered from a partially loaded index")
	}

	// Health and readiness answer on both listeners while loading.
	for name, base := range map[string]string{"s3": s3Listener.URL, "admin": adminListener.URL} {
		resp, err := http.Get(base + "/healthz")
		if err != nil {
			t.Fatalf("%s /healthz: %v", name, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || string(body) != "OK" {
			t.Fatalf("%s /healthz = %d %q, want 200 OK", name, resp.StatusCode, body)
		}
	}

	const objKey = "async-obj"
	body := []byte("armor async cold load payload")

	// An object write mid-load: it must complete (its manifest record is
	// buffered, not published — there is no safe sequence yet) ...
	put := signedS3Request(t, http.MethodPut, s3Listener.URL+"/b/"+objKey, body)
	put.Body.Close()
	if put.StatusCode != http.StatusOK {
		t.Fatalf("PUT while loading = %d", put.StatusCode)
	}

	// ... and object GET/HEAD still return correct data via the B2
	// fallback: GET reads the backend directly, HEAD's index fast path
	// misses and falls back to the stored metadata.
	get := signedS3Request(t, http.MethodGet, s3Listener.URL+"/b/"+objKey, nil)
	getBody, _ := io.ReadAll(get.Body)
	get.Body.Close()
	if get.StatusCode != http.StatusOK || !bytes.Equal(getBody, body) {
		t.Fatalf("GET while loading = %d (%d bytes), want 200 with %d-byte body", get.StatusCode, len(getBody), len(body))
	}
	head := signedS3Request(t, http.MethodHead, s3Listener.URL+"/b/"+objKey, nil)
	head.Body.Close()
	if head.StatusCode != http.StatusOK {
		t.Fatalf("HEAD while loading = %d", head.StatusCode)
	}
	if got := head.Header.Get("Content-Length"); got != strconv.Itoa(len(body)) {
		t.Fatalf("HEAD Content-Length = %q, want %d", got, len(body))
	}

	// Readiness reports the loading phase.
	ready := signedS3Request(t, http.MethodGet, s3Listener.URL+"/readyz", nil)
	readyBody, _ := io.ReadAll(ready.Body)
	ready.Body.Close()
	var readyz struct {
		Ready            bool   `json:"ready"`
		ManifestStatus   string `json:"manifest_status"`
		ManifestDegraded bool   `json:"manifest_degraded"`
	}
	if err := json.Unmarshal(readyBody, &readyz); err != nil {
		t.Fatalf("decode readyz: %v: %s", err, readyBody)
	}
	if !readyz.Ready || readyz.ManifestStatus != "loading" || readyz.ManifestDegraded {
		t.Fatalf("readyz while loading = %+v (%s)", readyz, readyBody)
	}

	// Nothing published while loading: the store still holds exactly the
	// seeded manifest files.
	afterPuts := gating.recordedPuts()
	for _, k := range afterPuts {
		if strings.HasPrefix(k, ".armor/manifest/") {
			t.Fatalf("manifest PUT %q happened while the load was still running", k)
		}
	}

	// Release: the load completes, swaps in the full index, and replays the
	// buffered write above it.
	gating.release()
	waitForStatus(t, srv, manifestLoaded)

	if got := srv.manifestIndex().Len(); got != 4 {
		t.Fatalf("loaded index holds %d entries, want 4 (3 seeded + 1 mid-load write)", got)
	}
	for i := 1; i <= 3; i++ {
		if _, ok := srv.manifestIndex().Get("b", "obj-"+strconv.Itoa(i)); !ok {
			t.Fatalf("loaded index missing seeded obj-%d", i)
		}
	}
	entry, ok := srv.manifestIndex().Get("b", objKey)
	if !ok || entry.PlaintextSize != int64(len(body)) {
		t.Fatalf("mid-load write missing from loaded index: ok=%v entry=%+v", ok, entry)
	}

	// The replayed write publishes above the loaded sequence (3), never
	// colliding with the persisted deltas.
	srv.StopManifestWriter()
	after := storeCensus(t, root)
	assertSeededDeltasIntact(t, before, after, 3)
	if _, ok := after["b/.armor/manifest/w1/delta-0000000004.jsonl"]; !ok {
		t.Fatalf("mid-load write never published; manifest store holds %v", manifestStoreKeys(after))
	}
	if got := srv.manifestIndex().Seq(); got != 4 {
		t.Fatalf("post-flush sequence = %d, want 4", got)
	}
	assertNoCollidingDeltaPUTs(t, gating.recordedPuts(), 3)

	// And the loaded index now serves the HEAD fast path.
	if _, ok := rec.Lookup("b", objKey); !ok {
		t.Fatal("lookup still misses after the load resolved")
	}
}

// TestColdLoadAsyncErrorDegradedWithFloor: a load that fails after the hold
// releases lands in the degraded mode with the sequence floor recovered —
// compaction paused, publication resumed above the floor, the write recorded
// mid-load replayed and published without colliding, and /readyz naming the
// mode.
func TestColdLoadAsyncErrorDegradedWithFloor(t *testing.T) {
	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	seedPersistedDeltas(t, fs, 3)
	before := storeCensus(t, root)

	gating := newGatingBackend(fs, ".armor/manifest", errors.New("injected transient fetch failure"))
	srv := newAsyncColdLoadServer(t, root, gating)
	t.Cleanup(func() {
		gating.release()
		srv.StopManifestWriter()
		srv.StopManifestCompactor()
	})
	gating.waitHeldFetches(t)

	// A write recorded mid-load; its publication must wait for the floor.
	const objKey = "held-write"
	rec := &manifestRecorder{sys: srv.manifestSys}
	rec.RecordPut("b", objKey, 42, "sha", nil, nil, "fp", 65536, "", "etag-h", nil, 42, nil)

	gating.release()
	waitForStatus(t, srv, manifestDegraded)

	if w := srv.manifestWriter(); w == nil {
		t.Fatal("degraded-with-floor mode has no writer — publication must resume above the floor")
	}
	if c := srv.manifestCompactor(); c != nil {
		t.Fatal("compactor started on an incomplete index")
	}
	if got := srv.manifestIndex().Seq(); got != 3 {
		t.Fatalf("recovered sequence floor = %d, want 3", got)
	}
	if entry, ok := srv.manifestIndex().Get("b", objKey); !ok || entry.PlaintextSize != 42 {
		t.Fatalf("mid-load write not replayed into the degraded index: ok=%v", ok)
	}

	// /readyz names the degraded mode (canary disabled keeps readiness true;
	// degradation is diagnostic).
	s3Listener := httptest.NewServer(srv.Handler())
	t.Cleanup(s3Listener.Close)
	ready := signedS3Request(t, http.MethodGet, s3Listener.URL+"/readyz", nil)
	readyBody, _ := io.ReadAll(ready.Body)
	ready.Body.Close()
	var readyz struct {
		Ready            bool   `json:"ready"`
		ManifestStatus   string `json:"manifest_status"`
		ManifestDegraded bool   `json:"manifest_degraded"`
		Reason           string `json:"reason"`
	}
	if err := json.Unmarshal(readyBody, &readyz); err != nil {
		t.Fatalf("decode readyz: %v: %s", err, readyBody)
	}
	if readyz.ManifestStatus != "degraded" || !readyz.ManifestDegraded {
		t.Fatalf("readyz degraded fields = %+v (%s)", readyz, readyBody)
	}
	if !strings.Contains(readyz.Reason, "sequence recovered at 3") {
		t.Fatalf("readyz reason %q does not name the recovered floor", readyz.Reason)
	}

	srv.StopManifestWriter()
	after := storeCensus(t, root)
	assertSeededDeltasIntact(t, before, after, 3)
	if _, ok := after["b/.armor/manifest/w1/delta-0000000004.jsonl"]; !ok {
		t.Fatalf("mid-load write never published above the floor; store holds %v", manifestStoreKeys(after))
	}
	assertNoCollidingDeltaPUTs(t, gating.recordedPuts(), 3)
}

// TestColdLoadAsyncShutdownCancelsLoad: shutdown during a held load cancels
// it — no writer or compactor starts, buffered records are dropped rather
// than published without a floor, and the manifest store is untouched.
func TestColdLoadAsyncShutdownCancelsLoad(t *testing.T) {
	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	seedPersistedDeltas(t, fs, 3)
	before := storeCensus(t, root)

	gating := newGatingBackend(fs, ".armor/manifest", nil)
	srv := newAsyncColdLoadServer(t, root, gating)
	t.Cleanup(func() {
		gating.release()
		srv.StopManifestWriter()
		srv.StopManifestCompactor()
	})
	gating.waitHeldFetches(t)

	// A write buffered mid-load; canceling the load must drop it —
	// publishing it would mean a delta with no established sequence.
	rec := &manifestRecorder{sys: srv.manifestSys}
	rec.RecordPut("b", "doomed-write", 7, "sha", nil, nil, "fp", 65536, "", "etag-d", nil, 7, nil)

	srv.StopManifestWriter() // settles: cancels the load, then stops the (nil) writer

	if got := srv.WaitForManifestLoad(10 * time.Second); got != manifestCanceled {
		t.Fatalf("manifest status after shutdown = %q, want %q", got, manifestCanceled)
	}
	if w := srv.manifestWriter(); w != nil {
		t.Fatal("writer started (or left running) after shutdown canceled the load")
	}
	if c := srv.manifestCompactor(); c != nil {
		t.Fatal("compactor started after shutdown canceled the load")
	}

	// The manifest store is byte-identical: no publication of any kind.
	after := storeCensus(t, root)
	for key, want := range before {
		if got := after[key]; got != want {
			t.Fatalf("manifest store changed across a canceled load: %s", key)
		}
	}
	for _, k := range gating.recordedPuts() {
		if strings.HasPrefix(k, ".armor/manifest/") {
			t.Fatalf("manifest PUT %q happened after cancellation", k)
		}
	}
}

// listGatingBackend holds manifest-prefix ListRaw calls on a channel until
// released, then fails them — the discovery outage that lands a load in the
// publication-paused mode while writes are still buffered. Non-manifest
// listings and every Put pass through; Puts are recorded.
type listGatingBackend struct {
	backend.Backend
	manifestPrefix string
	hold           chan struct{}
	listErr        error
	releaseOnce    sync.Once
	listCalls      atomic.Int64
	putMu          sync.Mutex
	putKeys        []string
}

func (b *listGatingBackend) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	if !strings.HasPrefix(prefix, b.manifestPrefix) {
		return b.Backend.ListRaw(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
	}
	b.listCalls.Add(1)
	select {
	case <-b.hold:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return nil, b.listErr
}

func (b *listGatingBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	b.putMu.Lock()
	b.putKeys = append(b.putKeys, key)
	b.putMu.Unlock()
	return b.Backend.Put(ctx, bucket, key, body, size, meta)
}

func (b *listGatingBackend) release() {
	b.releaseOnce.Do(func() { close(b.hold) })
}

func (b *listGatingBackend) recordedPuts() []string {
	b.putMu.Lock()
	defer b.putMu.Unlock()
	return append([]string(nil), b.putKeys...)
}

// TestColdLoadAsyncNoFloorDropsWritesBufferedMidLoad pins the resolve path
// for the worst failure shape: writes buffered while the load was held, then
// the load AND the floor recovery both fail. Resolution must drop the buffer
// (the objects' B2 headers stay authoritative) — replaying onto a nil writer
// panicked the loader goroutine, and a goroutine panic kills the process.
func TestColdLoadAsyncNoFloorDropsWritesBufferedMidLoad(t *testing.T) {
	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	before := storeCensus(t, root)

	gating := &listGatingBackend{
		Backend:        fs,
		manifestPrefix: ".armor/manifest",
		hold:           make(chan struct{}),
		listErr:        errors.New("injected discovery outage"),
	}
	srv := newAsyncColdLoadServer(t, root, gating)
	t.Cleanup(func() {
		gating.release()
		srv.StopManifestWriter()
		srv.StopManifestCompactor()
	})

	// Hold the load on its first manifest list, then record a write —
	// buffered, since there is no writer and no safe sequence yet.
	deadline := time.Now().Add(5 * time.Second)
	for gating.listCalls.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("cold load never reached a manifest list")
		}
		time.Sleep(5 * time.Millisecond)
	}
	rec := &manifestRecorder{sys: srv.manifestSys}
	rec.RecordPut("b", "orphaned-mid-load-write", 5, "sha", nil, nil, "fp", 65536, "", "etag-o", nil, 5, nil)

	gating.release()
	waitForStatus(t, srv, manifestDegraded)

	if w := srv.manifestWriter(); w != nil {
		t.Fatal("writer started without a recovered sequence floor")
	}
	// The buffered write was dropped, not served from the index and not
	// published: the fallback rule is that B2 headers stay authoritative.
	if _, ok := rec.Lookup("b", "orphaned-mid-load-write"); ok {
		t.Fatal("lookup served a record whose publication never became safe")
	}
	for _, k := range gating.recordedPuts() {
		if strings.HasPrefix(k, ".armor/manifest/") {
			t.Fatalf("manifest PUT %q attempted without a floor", k)
		}
	}
	after := storeCensus(t, root)
	for key, want := range before {
		if got := after[key]; got != want {
			t.Fatalf("manifest store changed across a publication-paused load: %s", key)
		}
	}

	s3Listener := httptest.NewServer(srv.Handler())
	t.Cleanup(s3Listener.Close)
	ready := signedS3Request(t, http.MethodGet, s3Listener.URL+"/readyz", nil)
	readyBody, _ := io.ReadAll(ready.Body)
	ready.Body.Close()
	if !strings.Contains(string(readyBody), "publication paused") {
		t.Fatalf("readyz does not name the paused mode: %s", readyBody)
	}
}

// TestColdLoadAsyncNoFloorPausesPublication: when even the floor-recovery
// list fails, publication stays off — no writer, records dropped, lookups
// missing, /readyz naming the paused mode.
func TestColdLoadAsyncNoFloorPausesPublication(t *testing.T) {
	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	seedPersistedDeltas(t, fs, 2)

	faulty := &faultBackend{Backend: fs, listErr: errors.New("injected discovery failure")}
	srv := newAsyncColdLoadServer(t, root, faulty)
	t.Cleanup(func() {
		srv.StopManifestWriter()
		srv.StopManifestCompactor()
	})

	waitForStatus(t, srv, manifestDegraded)
	if w := srv.manifestWriter(); w != nil {
		t.Fatal("writer started without a recovered sequence floor")
	}

	// Records are dropped, not buffered forever or published.
	rec := &manifestRecorder{sys: srv.manifestSys}
	rec.RecordPut("b", "untracked", 1, "sha", nil, nil, "fp", 65536, "", "etag-u", nil, 1, nil)
	if _, ok := rec.Lookup("b", "untracked"); ok {
		t.Fatal("lookup served from a publication-paused index")
	}
	for _, k := range faulty.putKeys {
		if strings.HasPrefix(k, ".armor/manifest/") {
			t.Fatalf("manifest PUT %q attempted without a floor", k)
		}
	}

	s3Listener := httptest.NewServer(srv.Handler())
	t.Cleanup(s3Listener.Close)
	ready := signedS3Request(t, http.MethodGet, s3Listener.URL+"/readyz", nil)
	readyBody, _ := io.ReadAll(ready.Body)
	ready.Body.Close()
	if !strings.Contains(string(readyBody), "publication paused") {
		t.Fatalf("readyz does not name the paused mode: %s", readyBody)
	}
}
