package handlers_test

// Behavioral coverage for armor-420c15a5: GetObject issues the ADR-016
// manifest probe and the legacy plaintext-key HEAD concurrently, then decides
// exactly as the sequential path did. These tests pin the decision semantics
// the overlap must not change: an existing manifest wins and the concurrent
// HEAD result is discarded no matter what it says (hit, 404, or anything
// else); only the latency changed. The round-trip-count half of the overlap
// is pinned by the TestSmallObjectFullGETRequestCount* tests with a
// deterministic overlap guard in get_request_count_test.go; the failure-class
// behaviors that must be unchanged by the overlap (stale manifest, quarantined
// manifest, non-404 manifest-read failure) are held by
// TestGetObjectManifestCiphertextFreshnessOrdering,
// TestQuarantineMakesReadsFailNonRetryably, and
// TestGetObjectClassifiesManifestLoadFailure respectively, which run this same
// overlapped path.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// probeTestSetup builds the config, caches and key manager testSetup uses,
// over a caller-supplied backend so the overlap tests can wrap or block the
// probe calls.
func probeTestSetup(t *testing.T, be backend.Backend) (*config.Config, *backend.MetadataCache, *backend.FooterCache, *keymanager.KeyManager) {
	t.Helper()

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)

	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	return cfg, cache, footerCache, km
}

// writeProbeManifest plants a manifest for key over a single-PUT ciphertext
// (the shape the freshness tests use: a real PUT's ciphertext with the ADR-016
// manifest fields added and completedAt in the near future, the normal
// ordering). metaOverride patches selected metadata entries so a test can make
// the manifest observably disagree with the stored object's own metadata.
func writeProbeManifest(t *testing.T, mb *mockBackend, bucket, key string, metaOverride map[string]string, completedAt time.Time) {
	t.Helper()

	fullKey := bucket + "/" + key
	mb.mu.Lock()
	armorMeta := make(map[string]string, len(mb.meta[fullKey])+2)
	for k, v := range mb.meta[fullKey] {
		armorMeta[k] = v
	}
	mb.mu.Unlock()
	if armorMeta["x-amz-meta-armor-version"] == "" {
		t.Fatal("stored ciphertext object carries no ARMOR metadata")
	}
	for k, v := range metaOverride {
		armorMeta[k] = v
	}
	armorMeta["x-amz-meta-armor-completed-at"] = completedAt.UTC().Format(time.RFC3339)
	armorMeta["x-amz-meta-armor-ciphertext-ref"] = key
	armorMeta["Content-Type"] = "application/x-armor-manifest+json"

	manifestBody, err := json.Marshal(backend.ManifestBody{
		CiphertextObject: key,
		UploadID:         "upload-probe-overlap-test",
		CompletedAt:      completedAt.UTC().Format(time.RFC3339),
		Metadata:         armorMeta,
	})
	if err != nil {
		t.Fatalf("failed to marshal manifest body: %v", err)
	}
	if err := mb.Put(context.Background(), bucket, key+".armor-manifest", bytes.NewReader(manifestBody), int64(len(manifestBody)), armorMeta); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
}

// headCountingSpyBackend counts HEAD outcomes per key without changing any
// behavior, so a test can prove the overlapped probe HEAD actually fired and
// was then discarded.
type headCountingSpyBackend struct {
	*mockBackend
	mu   sync.Mutex
	hits map[string]int
	miss map[string]int
}

func newHeadCountingSpyBackend(mb *mockBackend) *headCountingSpyBackend {
	return &headCountingSpyBackend{mockBackend: mb, hits: make(map[string]int), miss: make(map[string]int)}
}

func (b *headCountingSpyBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	info, err := b.mockBackend.Head(ctx, bucket, key)
	b.mu.Lock()
	if err != nil {
		b.miss[bucket+"/"+key]++
	} else {
		b.hits[bucket+"/"+key]++
	}
	b.mu.Unlock()
	return info, err
}

func (b *headCountingSpyBackend) headCounts(key string) (hits, miss int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.hits[key], b.miss[key]
}

// TestGetObjectOverlapManifestWinsOverLegacyObject covers the "legacy object
// plus a newer manifest for the same key" acceptance case: both objects exist,
// the manifest is newer, and its metadata observably disagrees with the stored
// object's own HEAD. The read must serve the manifest's verdict — the
// concurrent legacy HEAD (which did run, and did hit) must be discarded.
func TestGetObjectOverlapManifestWinsOverLegacyObject(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	spy := newHeadCountingSpyBackend(mb)
	h := handlers.New(cfg, spy, cache, footerCache, km, nil)

	bucket, key := "test-bucket", "overlap-legacy-key"
	plaintext := []byte("probe overlap: manifest must win over the legacy object")

	putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, bytes.NewReader(plaintext))
	putReq.Header.Set("Content-Type", "text/plain")
	putW := httptest.NewRecorder()
	h.HandleRoot(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d, body %s", putW.Code, putW.Body.String())
	}

	// The manifest is newer than the legacy object and claims a different
	// content type than the stored object's own metadata — the value the
	// legacy HEAD would have served if it won.
	writeProbeManifest(t, mb, bucket, key, map[string]string{
		"x-amz-meta-armor-content-type": "application/x-manifest-wins",
	}, time.Now().Add(2*time.Minute))

	getReq := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	getW := httptest.NewRecorder()
	h.HandleRoot(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET returned %d: %s", getW.Code, getW.Body.String())
	}
	if !bytes.Equal(getW.Body.Bytes(), plaintext) {
		t.Fatalf("GET content mismatch: got %q, want %q", string(getW.Body.Bytes()), string(plaintext))
	}
	if gotCT := getW.Header().Get("Content-Type"); gotCT != "application/x-manifest-wins" {
		t.Fatalf("GET served Content-Type %q — the legacy HEAD's metadata won instead of the manifest's", gotCT)
	}

	// The probe HEAD did run concurrently and hit the legacy object; the
	// decision above was made despite its result, not because it never fired.
	// (The ADR-016 freshness HEAD later targets the same ciphertext name, so
	// hits counts both — the probe is proved to have fired by hits >= 1 and
	// its result was discarded by the Content-Type assertion above.)
	if hits, miss := spy.headCounts(bucket + "/" + key); hits < 1 || miss != 0 {
		t.Fatalf("overlapped legacy HEAD recorded hits=%d miss=%d, want at least one hit and no miss", hits, miss)
	}
}

// probeMissBackend makes exactly the overlapped probe HEAD — the one issued
// while the manifest GET is still in flight — miss definitively, and counts
// the misses. The window is deterministic, not a timing race: the GET holds
// it open until the Head running inside it arrives (or a grace timeout lapses
// and the test's miss-count assertion fails with that verdict), so a
// late-scheduled probe goroutine cannot fall outside it. The production
// manifest writer stores the ciphertext under the key itself, so the live
// fleet cannot produce this ordering today; the pin exists because the
// decision rule must not depend on that: with a manifest present, a
// definitive 404 on the discarded probe must neither fail the request nor
// downgrade it to the legacy path.
type probeMissBackend struct {
	*mockBackend
	mu           sync.Mutex
	probePending bool
	probeArrived chan struct{}
	probeMisses  int64
}

func newProbeMissBackend(mb *mockBackend) *probeMissBackend {
	return &probeMissBackend{mockBackend: mb, probeArrived: make(chan struct{})}
}

func (b *probeMissBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	if strings.HasSuffix(key, ".armor-manifest") {
		b.mu.Lock()
		b.probePending = true
		b.mu.Unlock()
		defer func() {
			// Hold the probe window open for the overlapped HEAD.
			select {
			case <-b.probeArrived:
			case <-time.After(5 * time.Second):
			}
			b.mu.Lock()
			b.probePending = false
			b.mu.Unlock()
		}()
	}
	return b.mockBackend.Get(ctx, bucket, key)
}

func (b *probeMissBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	b.mu.Lock()
	if b.probePending {
		b.probePending = false
		b.probeMisses++
		b.mu.Unlock()
		close(b.probeArrived)
		return nil, backend.ErrNotFound
	}
	b.mu.Unlock()
	return b.mockBackend.Head(ctx, bucket, key)
}

// TestGetObjectOverlapIgnoresProbeHead404 pins the "HEAD 404 with the manifest
// present" acceptance case: the overlapped probe HEAD misses definitively and
// the GET still serves the object from the manifest.
func TestGetObjectOverlapIgnoresProbeHead404(t *testing.T) {
	mb := newMockBackend()
	pb := newProbeMissBackend(mb)
	cfg, cache, footerCache, km := probeTestSetup(t, pb)
	h := handlers.New(cfg, pb, cache, footerCache, km, nil)

	bucket, key := "test-bucket", "overlap-head404-key"
	plaintext := []byte("probe overlap: a discarded 404 must not fail the read")

	putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, bytes.NewReader(plaintext))
	putReq.Header.Set("Content-Type", "text/plain")
	putW := httptest.NewRecorder()
	h.HandleRoot(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d, body %s", putW.Code, putW.Body.String())
	}

	writeProbeManifest(t, mb, bucket, key, nil, time.Now().Add(2*time.Minute))

	getReq := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	getW := httptest.NewRecorder()
	h.HandleRoot(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET returned %d with the manifest present despite the probe HEAD's 404: %s", getW.Code, getW.Body.String())
	}
	if !bytes.Equal(getW.Body.Bytes(), plaintext) {
		t.Fatalf("GET content mismatch: got %q, want %q", string(getW.Body.Bytes()), string(plaintext))
	}
	if n := pb.probeMisses; n != 1 {
		t.Fatalf("probe HEAD misses = %d, want exactly 1 (the overlapped probe, not the freshness HEAD)", n)
	}
}

// blockingProbeBackend blocks both probe calls until the request context is
// cancelled and records that each observed the cancellation, for the client
// cancellation acceptance case.
type blockingProbeBackend struct {
	*mockBackend
	getStarted   chan struct{}
	headStarted  chan struct{}
	getOnce      sync.Once
	headOnce     sync.Once
	getCanceled  atomic.Bool
	headCanceled atomic.Bool
}

func (b *blockingProbeBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	b.getOnce.Do(func() { close(b.getStarted) })
	<-ctx.Done()
	b.getCanceled.Store(true)
	return nil, nil, ctx.Err()
}

func (b *blockingProbeBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	b.headOnce.Do(func() { close(b.headStarted) })
	<-ctx.Done()
	b.headCanceled.Store(true)
	return nil, ctx.Err()
}

// TestGetObjectOverlapClientCancellation pins that a client disconnect while
// both probe requests are in flight cancels BOTH of them and leaves no
// goroutine behind: the manifest GET and the overlapped probe HEAD share the
// request context, and the probe goroutine always delivers into its buffered
// channel and exits.
func TestGetObjectOverlapClientCancellation(t *testing.T) {
	mb := newMockBackend()
	bb := &blockingProbeBackend{
		mockBackend: mb,
		getStarted:  make(chan struct{}),
		headStarted: make(chan struct{}),
	}
	cfg, cache, footerCache, km := probeTestSetup(t, bb)
	h := handlers.New(cfg, bb, cache, footerCache, km, nil)

	bucket, key := "test-bucket", "overlap-cancel-key"
	// The manifest only needs to exist so that readManifest's GET is one of
	// the two probe requests; the read never completes here, so minimal
	// metadata suffices.
	manifestBody, err := json.Marshal(backend.ManifestBody{
		CiphertextObject: key,
		UploadID:         "upload-overlap-cancel",
		CompletedAt:      time.Now().UTC().Format(time.RFC3339),
		Metadata:         map[string]string{"x-amz-meta-armor-version": "3"},
	})
	if err != nil {
		t.Fatalf("failed to marshal manifest body: %v", err)
	}
	if err := mb.Put(context.Background(), bucket, key+".armor-manifest", bytes.NewReader(manifestBody), int64(len(manifestBody)), nil); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// The handler blocks on the backend, so run it under a cancellable
	// context the way net/http would for a disconnecting client.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
	}()

	// Both probes in flight: the manifest GET and the overlapped HEAD.
	for _, started := range []chan struct{}{bb.getStarted, bb.headStarted} {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("probe request never started; the manifest GET and the HEAD are not both in flight")
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not return after client cancellation")
	}

	// Both goroutines are unblocked by the cancellation; wait for the
	// scheduler to actually run their post-cancel statements rather than
	// racing them from the test goroutine.
	bothObserved := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if bb.getCanceled.Load() && bb.headCanceled.Load() {
			bothObserved = true
			break
		}
	}
	if !bothObserved {
		t.Fatalf("probe cancellation not observed by both requests: manifest GET cancelled=%v, probe HEAD cancelled=%v",
			bb.getCanceled.Load(), bb.headCanceled.Load())
	}

	// No goroutine leak: both the handler goroutine and the probe goroutine
	// must be gone shortly after the cancellation. Count stacks still inside
	// GetObject (the handler frame or its probe closure) rather than the
	// global goroutine count — full-suite runs keep unrelated goroutines
	// alive, and the global count is noise against them.
	goroutinesInGetObject := func() int {
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		count := 0
		for _, line := range strings.Split(string(buf[:n]), "\n") {
			if strings.Contains(line, "handlers.(*Handlers) GetObject") {
				count++
			}
		}
		return count
	}
	leaked := -1
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if now := goroutinesInGetObject(); now == 0 {
			leaked = -1
			break
		} else {
			leaked = now
		}
	}
	if leaked != -1 {
		t.Fatalf("%d goroutine(s) still inside GetObject after cancellation drained — the probe goroutine leaks", leaked)
	}
}
