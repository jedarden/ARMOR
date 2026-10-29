package server

// Deterministic backend-fault coverage for the manifest cold-load fallback
// (armor-b1745d47, plan §9.6 "fix the sequence reset"). Before the fix, an
// exhausted startup load fell back to an empty index whose writer sequence
// restarted at 1, so the first post-failure flush PUT
// delta-0000000001.jsonl over the persisted delta of the same name.
//
// The fault injection wraps the filesystem backend the production wiring
// builds: Get/GetDirect fail while ListRaw keeps working (the load fails
// AFTER discovery — the exact transient that used to reset the sequence),
// and Put records every key it is handed so the suite can prove no colliding
// PUT ever happens.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/manifest"
)

// faultBackend wraps a real backend with deterministic failures. A non-nil
// fetchErr fails Get/GetDirect (delta and snapshot bodies); a non-nil listErr
// fails ListRaw (discovery). Every Put is recorded before delegation.
type faultBackend struct {
	backend.Backend
	fetchErr error
	listErr  error
	putKeys  []string
}

func (b *faultBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	if b.fetchErr != nil {
		return nil, nil, b.fetchErr
	}
	return b.Backend.Get(ctx, bucket, key)
}

func (b *faultBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	if b.fetchErr != nil {
		return nil, nil, b.fetchErr
	}
	return b.Backend.GetDirect(ctx, bucket, key)
}

func (b *faultBackend) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	if b.listErr != nil {
		return nil, b.listErr
	}
	return b.Backend.ListRaw(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

func (b *faultBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	b.putKeys = append(b.putKeys, key)
	return b.Backend.Put(ctx, bucket, key, body, size, meta)
}

// coldLoadFaultConfig is the deployment under test: filesystem backend,
// manifest enabled, writer w1 whose shard already holds persisted deltas.
func coldLoadFaultConfig(root string) *config.Config {
	return &config.Config{
		Backend:                     "filesystem",
		FSPath:                      root,
		Bucket:                      "b",
		ManifestEnabled:             true,
		ManifestPrefix:              ".armor/manifest",
		WriterID:                    "w1",
		ManifestLoadTimeout:         30,
		ReadRetryAttempts:           3,
		ReadRetryBackoff:            time.Millisecond,
		ReadRetryMaxBackoff:         time.Millisecond,
		ManifestCompactionInterval:  3600,
		ManifestCompactionThreshold: 1000,
	}
}

// seedPersistedDeltas writes delta-1..delta-N for writer w1 exactly as a
// healthy run would have left them, one op per delta with a recognisable
// size so reconstruction is observable.
func seedPersistedDeltas(t *testing.T, be backend.Backend, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		data, err := manifest.MarshalDelta([]manifest.Op{{
			Operation: "put",
			Key:       "b/obj-" + strconv.Itoa(i),
			Entry:     &manifest.Entry{PlaintextSize: int64(i) * 111, BlockSize: 65536},
		}})
		if err != nil {
			t.Fatalf("MarshalDelta: %v", err)
		}
		key := manifest.DeltaKey(".armor/manifest", "w1", uint64(i))
		if err := be.Put(context.Background(), "b", key, strings.NewReader(string(data)), int64(len(data)), nil); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
}

// storeCensus snapshots every file under root as path→content.
func storeCensus(t *testing.T, root string) map[string]string {
	t.Helper()
	census := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		census[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("census walk: %v", err)
	}
	return census
}

// TestColdLoadFaultPreservesPersistedDeltas is the bead's first acceptance
// leg: persisted current-writer deltas survive a failed cold load, a new
// write's manifest flush lands above the recovered floor, no colliding PUT
// occurs, and compaction stays paused while the index is incomplete.
func TestColdLoadFaultPreservesPersistedDeltas(t *testing.T) {
	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	seedPersistedDeltas(t, fs, 3)

	// The persisted store, byte for byte, before anything else touches it.
	// The filesystem backend keeps a .metadata sidecar per object, so three
	// seeded deltas are six files.
	before := storeCensus(t, root)
	if len(before) != 6 {
		t.Fatalf("seed produced %d files, want 6 (3 deltas + sidecars): %v", len(before), keysOf(before))
	}

	// Cold load fails AFTER discovery: listing works, every body fetch fails.
	faulty := &faultBackend{Backend: fs, fetchErr: errors.New("injected transient fetch failure")}
	st := startManifestSystem(coldLoadFaultConfig(root), logging.New("test"), faulty)

	if st.degradedReason == "" {
		t.Fatal("degraded reason empty after failed cold load")
	}
	if got := st.idx.Seq(); got != 3 {
		t.Fatalf("recovered sequence floor = %d, want 3 (max persisted delta)", got)
	}
	if st.writer == nil {
		t.Fatal("writer nil — publication above a recovered floor must stay available")
	}
	if st.compactor != nil {
		t.Fatal("compactor started on an incomplete index — compaction would overwrite the good snapshot and delete unloaded deltas")
	}

	// A new object write through the production recorder, then a flush.
	rec := &manifestRecorder{idx: st.idx, writer: st.writer}
	rec.RecordPut("b", "obj-4", 44, "sha", nil, nil, "fp", 65536, "", "etag-4", nil, 44, nil)
	st.writer.Stop()

	// Every pre-existing delta key retains its original bytes.
	after := storeCensus(t, root)
	for key, want := range before {
		got, ok := after[key]
		if !ok {
			t.Fatalf("persisted delta %s vanished after the degraded flush", key)
		}
		if got != want {
			t.Fatalf("persisted delta %s was overwritten", key)
		}
	}
	// The new delta lands above the floor with a unique key.
	newKey := "b/.armor/manifest/w1/delta-0000000004.jsonl"
	if _, ok := after[newKey]; !ok {
		t.Fatalf("post-recovery delta %s missing; store now: %v", newKey, keysOf(after))
	}
	// And no PUT ever targeted a pre-existing key.
	for _, put := range faulty.putKeys {
		if put == manifest.DeltaKey(".armor/manifest", "w1", 1) ||
			put == manifest.DeltaKey(".armor/manifest", "w1", 2) ||
			put == manifest.DeltaKey(".armor/manifest", "w1", 3) {
			t.Fatalf("colliding PUT to pre-existing delta %s", put)
		}
	}

	// The bead's second acceptance leg: the next successful load replays
	// everything — seeded and recovery-era deltas alike — and the sequence
	// continues uniquely.
	recovered := startManifestSystem(coldLoadFaultConfig(root), logging.New("test"), fs)
	if recovered.degradedReason != "" {
		t.Fatalf("reload still degraded: %s", recovered.degradedReason)
	}
	if recovered.compactor == nil {
		t.Fatal("healthy reload has no compactor")
	}
	for i := 1; i <= 4; i++ {
		if _, ok := recovered.idx.Get("b", "obj-"+strconv.Itoa(i)); !ok {
			t.Fatalf("reloaded index missing obj-%d", i)
		}
	}
	if got := recovered.idx.Seq(); got != 4 {
		t.Fatalf("reloaded sequence = %d, want 4", got)
	}
	recovered.writer.Stop()
	recovered.compactor.Stop()

	if next := recovered.idx.IncSeq(); next != 5 {
		t.Fatalf("first post-reload sequence = %d, want 5", next)
	}

	// The pre-failure bytes are still intact after the healthy reload.
	final := storeCensus(t, root)
	for key, want := range before {
		if got := final[key]; got != want {
			t.Fatalf("persisted delta %s changed across the healthy reload", key)
		}
	}
}

// TestColdLoadFaultWithoutFloorPausesPublication: when even the floor
// recovery list fails, no safe high-water mark exists and the pod must not
// publish any delta at all — reads and writes continue through the B2
// fallback until a restart loads successfully.
func TestColdLoadFaultWithoutFloorPausesPublication(t *testing.T) {
	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	seedPersistedDeltas(t, fs, 2)

	faulty := &faultBackend{Backend: fs, listErr: errors.New("injected discovery failure")}
	st := startManifestSystem(coldLoadFaultConfig(root), logging.New("test"), faulty)

	if st.writer != nil {
		t.Fatal("writer started without a recovered sequence floor — its first flush could overwrite a persisted delta")
	}
	if st.compactor != nil {
		t.Fatal("compactor started without a recovered sequence floor")
	}
	if st.degradedReason == "" {
		t.Fatal("degraded reason empty after failed cold load and failed floor recovery")
	}
	if !strings.Contains(st.degradedNote, "publication paused") {
		t.Fatalf("degraded note %q does not name the paused mode", st.degradedNote)
	}
	if len(faulty.putKeys) != 0 {
		t.Fatalf("publication attempted without a floor: PUTs %v", faulty.putKeys)
	}

	// The readiness surface carries the mode, not just the failure.
	s := &Server{
		manifestDegradedReason: st.degradedReason,
		manifestDegradedNote:   st.degradedNote,
		canaryDisabled:         true,
	}
	rec := httptest.NewRecorder()
	s.readyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	var body struct {
		Ready            bool   `json:"ready"`
		ManifestDegraded bool   `json:"manifest_degraded"`
		Reason           string `json:"reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode readyz body: %v", err)
	}
	if !body.ManifestDegraded {
		t.Fatalf("manifest_degraded not set: %s", rec.Body.String())
	}
	if !strings.Contains(body.Reason, "publication paused") {
		t.Fatalf("readyz reason %q does not carry the paused-publication note", body.Reason)
	}
}

// TestColdLoadFreshStoreStillPublishes: a store that does not exist yet is
// the first-startup case, not a fault. The filesystem backend fails the
// bucket walk with ENOENT where B2 lists an empty bucket; that must land as
// a complete (empty) index — writer and compactor running, first delta at
// sequence 1, no degradation — exactly like a B2 first boot.
func TestColdLoadFreshStoreStillPublishes(t *testing.T) {
	root := t.TempDir() // the bucket directory does not exist yet
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}

	st := startManifestSystem(coldLoadFaultConfig(root), logging.New("test"), fs)
	if st.degradedReason != "" {
		t.Fatalf("fresh store reported degraded: %s", st.degradedReason)
	}
	if st.writer == nil || st.compactor == nil {
		t.Fatal("fresh store must run both the manifest writer and compactor")
	}

	rec := &manifestRecorder{idx: st.idx, writer: st.writer}
	rec.RecordPut("b", "obj-1", 11, "sha", nil, nil, "fp", 65536, "", "etag-1", nil, 11, nil)
	st.writer.Stop()

	if _, ok := storeCensus(t, root)["b/.armor/manifest/w1/delta-0000000001.jsonl"]; !ok {
		t.Fatal("first-ever delta delta-0000000001.jsonl missing after flush")
	}
}

func keysOf(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
