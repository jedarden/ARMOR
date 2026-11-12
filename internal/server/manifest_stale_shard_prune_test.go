package server

// Stale-shard prune coverage (armor-45abc172). A successful cold load merges
// every discovered writer shard into the index, so the dead shards other pod
// replacements left behind are redundant copies the next cold load would
// fetch and replay forever — the corpus that overran iad-ci's load budget.
// These tests pin the wiring contract: a loaded pod prunes foreign stale
// objects and nothing else, and a degraded pod (empty index) prunes nothing.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/manifest"
)

// seedShardDelta writes one valid put delta for a writer shard and returns
// the store path it landed at (the filesystem backend nests under
// <FSPath>/<bucket>/, standing in for the bucket root a B2 key would see).
func seedShardDelta(t *testing.T, root string, be backend.Backend, writerID string, seq uint64, objKey string) string {
	t.Helper()
	data, err := manifest.MarshalDelta([]manifest.Op{{
		Operation: "put",
		Key:       objKey,
		Entry:     &manifest.Entry{PlaintextSize: 10, BlockSize: 65536},
	}})
	if err != nil {
		t.Fatalf("MarshalDelta for %s: %v", writerID, err)
	}
	key := manifest.DeltaKey(".armor/manifest", writerID, seq)
	if err := be.Put(context.Background(), "b", key, strings.NewReader(string(data)), int64(len(data)), nil); err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
	return filepath.Join(root, "b", filepath.FromSlash(key))
}

// ageStorePath backdates a store file so the prune's cutoff classifies it as
// pre-load state.
func ageStorePath(t *testing.T, path string, age time.Duration) {
	t.Helper()
	old := time.Now().Add(-age)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("Chtimes %s: %v", path, err)
	}
}

// waitForStoreCondition polls the store census until cond holds, bounded by
// the prune goroutine's asynchronous completion.
func waitForStoreCondition(t *testing.T, root string, cond func(census map[string]bool) bool, what string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		census := make(map[string]bool)
		for key := range storeCensus(t, root) {
			census[key] = true
		}
		if cond(census) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s; store holds:%s", what, storeCensus(t, root))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSuccessfulColdLoadPrunesDeadWriterShards(t *testing.T) {
	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}

	dead1 := seedShardDelta(t, root, fs, "writer-dead-1", 1, "b/old-1")
	dead2 := seedShardDelta(t, root, fs, "writer-dead-2", 1, "b/old-2")
	current := seedShardDelta(t, root, fs, "w1", 1, "b/current")
	fresh := seedShardDelta(t, root, fs, "writer-fresh", 1, "b/fresh")
	stray := filepath.Join(root, "b", ".armor", "manifest", "stray.jsonl")
	if err := os.WriteFile(stray, []byte("no shard segment\n"), 0o644); err != nil {
		t.Fatalf("write stray: %v", err)
	}

	// Everything but the fresh foreign shard predates any cutoff the load
	// could compute: two dead shards, the current shard's own object, and
	// the malformed stray. Only the two dead shards may be pruned — the
	// current shard survives despite its age, and a key the loader skips
	// (no writer segment) is never deleted.
	for _, p := range []string{dead1, dead2, current, stray} {
		ageStorePath(t, p, 2*time.Hour)
	}

	srv := newAsyncColdLoadServer(t, root, fs)
	if status := srv.WaitForManifestLoad(30 * time.Second); status != manifestLoaded {
		t.Fatalf("manifest load settled as %q, want %q", status, manifestLoaded)
	}
	if got := srv.manifestIndex().Len(); got != 4 {
		t.Errorf("loaded %d manifest entries, want 4 (both dead shards merge before being pruned)", got)
	}

	waitForStoreCondition(t, root, func(census map[string]bool) bool {
		return !census[filepath.ToSlash(dead1)] && !census[filepath.ToSlash(dead2)]
	}, "dead shards to be pruned")

	for key, path := range map[string]string{
		"current writer's object":      current,
		"fresh foreign shard's object": fresh,
		"shardless stray":              stray,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was pruned (%s): %v", key, path, err)
		}
	}
}

func TestDegradedColdLoadDoesNotPrune(t *testing.T) {
	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}

	// A delta the loader cannot load fails the whole load: the pod lands in
	// the empty-index fallback, which merged nothing and must prune nothing.
	corrupt := filepath.Join(root, "b", ".armor", "manifest", "writer-dead", "delta-0000000001.jsonl")
	if err := os.MkdirAll(filepath.Dir(corrupt), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(corrupt, []byte("not-json\n"), 0o644); err != nil {
		t.Fatalf("write corrupt delta: %v", err)
	}
	ageStorePath(t, corrupt, 2*time.Hour)
	stale := seedShardDelta(t, root, fs, "writer-dead-2", 1, "b/old-2")
	ageStorePath(t, stale, 2*time.Hour)

	srv := newAsyncColdLoadServer(t, root, fs)
	if status := srv.WaitForManifestLoad(30 * time.Second); status != manifestDegraded {
		t.Fatalf("manifest load settled as %q, want %q", status, manifestDegraded)
	}

	// Give an (incorrect) prune every opportunity to run before asserting
	// the store is untouched.
	time.Sleep(500 * time.Millisecond)
	for _, p := range []string{corrupt, stale} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("degraded pod pruned %s: %v", p, err)
		}
	}
}
