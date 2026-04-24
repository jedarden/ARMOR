package manifest_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/manifest"
)

// mockStore is a simple in-memory key→bytes store used by loader tests.
type mockStore struct {
	objects map[string][]byte
}

func newMockStore() *mockStore {
	return &mockStore{objects: make(map[string][]byte)}
}

func (m *mockStore) put(key string, data []byte) {
	m.objects[key] = data
}

// lister returns a Lister that pages through all keys with the given prefix.
// It returns all matching keys in a single page (token ignored) for simplicity.
func (m *mockStore) lister() manifest.Lister {
	return func(ctx context.Context, prefix, token string) ([]string, string, error) {
		var keys []string
		for k := range m.objects {
			if strings.HasPrefix(k, prefix) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		return keys, "", nil
	}
}

// fetcher returns a Fetcher backed by the mock store.
func (m *mockStore) fetcher() manifest.Fetcher {
	return func(ctx context.Context, key string) ([]byte, error) {
		d, ok := m.objects[key]
		if !ok {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return d, nil
	}
}

const prefix = ".armor/manifest"
const writerA = "writer-a"
const writerB = "writer-b"

func TestLoadEmpty(t *testing.T) {
	store := newMockStore()
	idx := manifest.New()

	err := manifest.Load(context.Background(), idx, prefix, writerA, store.lister(), store.fetcher())
	if err != nil {
		t.Fatalf("Load on empty store: %v", err)
	}
	if idx.Len() != 0 {
		t.Fatalf("expected empty index, got %d entries", idx.Len())
	}
	if idx.Seq() != 0 {
		t.Fatalf("expected seq=0, got %d", idx.Seq())
	}
}

func TestLoadSnapshotOnly(t *testing.T) {
	store := newMockStore()

	// Build a snapshot with two entries for writerA.
	src := manifest.New()
	src.Put("bucket", "file1.parquet", sampleEntry(100))
	src.Put("bucket", "file2.parquet", sampleEntry(200))

	snap, err := src.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}
	store.put(manifest.SnapshotKey(prefix, writerA), snap)

	idx := manifest.New()
	if err := manifest.Load(context.Background(), idx, prefix, writerA, store.lister(), store.fetcher()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if idx.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", idx.Len())
	}
	e, ok := idx.Get("bucket", "file1.parquet")
	if !ok {
		t.Fatal("file1.parquet missing")
	}
	if e.PlaintextSize != 100 {
		t.Fatalf("file1 size: want 100, got %d", e.PlaintextSize)
	}
	// No deltas → seq stays 0.
	if idx.Seq() != 0 {
		t.Fatalf("seq should be 0 (no deltas), got %d", idx.Seq())
	}
}

func TestLoadDeltasOnly(t *testing.T) {
	store := newMockStore()
	now := time.Now().UTC().Truncate(time.Millisecond)

	ops := []manifest.Op{
		{Operation: "put", Key: "bucket/alpha.parquet", Entry: sampleEntryAt(300, now), Ts: now},
		{Operation: "put", Key: "bucket/beta.parquet", Entry: sampleEntryAt(400, now), Ts: now},
	}
	delta, _ := manifest.MarshalDelta(ops)
	store.put(manifest.DeltaKey(prefix, writerA, 1), delta)

	idx := manifest.New()
	if err := manifest.Load(context.Background(), idx, prefix, writerA, store.lister(), store.fetcher()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if idx.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", idx.Len())
	}
	if idx.Seq() != 1 {
		t.Fatalf("seq: want 1, got %d", idx.Seq())
	}
}

func TestLoadSnapshotAndDeltas(t *testing.T) {
	store := newMockStore()
	earlier := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	now := time.Now().UTC().Truncate(time.Millisecond)

	// Snapshot has file-a and file-b.
	src := manifest.New()
	src.Put("bucket", "file-a.parquet", sampleEntryAt(10, earlier))
	src.Put("bucket", "file-b.parquet", sampleEntryAt(20, earlier))
	snap, _ := src.MarshalSnapshot()
	store.put(manifest.SnapshotKey(prefix, writerA), snap)

	// Delta 1: add file-c, delete file-a.
	ops1 := []manifest.Op{
		{Operation: "put", Key: "bucket/file-c.parquet", Entry: sampleEntryAt(30, now), Ts: now},
		{Operation: "del", Key: "bucket/file-a.parquet", Ts: now},
	}
	d1, _ := manifest.MarshalDelta(ops1)
	store.put(manifest.DeltaKey(prefix, writerA, 1), d1)

	// Delta 2: update file-b.
	ops2 := []manifest.Op{
		{Operation: "put", Key: "bucket/file-b.parquet", Entry: sampleEntryAt(99, now), Ts: now},
	}
	d2, _ := manifest.MarshalDelta(ops2)
	store.put(manifest.DeltaKey(prefix, writerA, 2), d2)

	idx := manifest.New()
	if err := manifest.Load(context.Background(), idx, prefix, writerA, store.lister(), store.fetcher()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Expect: file-b (updated), file-c (added); file-a deleted.
	if idx.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", idx.Len())
	}
	if _, ok := idx.Get("bucket", "file-a.parquet"); ok {
		t.Fatal("file-a should be deleted")
	}
	if e, ok := idx.Get("bucket", "file-b.parquet"); !ok || e.PlaintextSize != 99 {
		t.Fatalf("file-b: want size 99, got %v (ok=%v)", e, ok)
	}
	if _, ok := idx.Get("bucket", "file-c.parquet"); !ok {
		t.Fatal("file-c should exist")
	}
	if idx.Seq() != 2 {
		t.Fatalf("seq: want 2, got %d", idx.Seq())
	}
}

func TestLoadMultipleWriters(t *testing.T) {
	store := newMockStore()
	earlier := time.Now().UTC().Add(-time.Minute).Truncate(time.Millisecond)
	now := time.Now().UTC().Truncate(time.Millisecond)

	// WriterA writes file-shared (earlier) and file-only-a.
	srcA := manifest.New()
	srcA.Put("bucket", "shared.parquet", sampleEntryAt(111, earlier))
	srcA.Put("bucket", "only-a.parquet", sampleEntryAt(222, earlier))
	snapA, _ := srcA.MarshalSnapshot()
	store.put(manifest.SnapshotKey(prefix, writerA), snapA)

	// WriterB writes file-shared (newer, wins) and file-only-b.
	opsB := []manifest.Op{
		{Operation: "put", Key: "bucket/shared.parquet", Entry: sampleEntryAt(999, now), Ts: now},
		{Operation: "put", Key: "bucket/only-b.parquet", Entry: sampleEntryAt(333, now), Ts: now},
	}
	dB, _ := manifest.MarshalDelta(opsB)
	store.put(manifest.DeltaKey(prefix, writerB, 5), dB)

	idx := manifest.New()
	// currentWriter is writerA — seq should reflect writerA's deltas (0, no deltas for A).
	if err := manifest.Load(context.Background(), idx, prefix, writerA, store.lister(), store.fetcher()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if idx.Len() != 3 {
		t.Fatalf("expected 3 entries (shared, only-a, only-b), got %d", idx.Len())
	}

	// WriterB's newer value for shared wins.
	e, ok := idx.Get("bucket", "shared.parquet")
	if !ok {
		t.Fatal("shared.parquet missing")
	}
	if e.PlaintextSize != 999 {
		t.Fatalf("shared.parquet: want 999, got %d", e.PlaintextSize)
	}

	if _, ok := idx.Get("bucket", "only-a.parquet"); !ok {
		t.Fatal("only-a.parquet missing")
	}
	if _, ok := idx.Get("bucket", "only-b.parquet"); !ok {
		t.Fatal("only-b.parquet missing")
	}

	// currentWriter is writerA which has no deltas → seq = 0.
	if idx.Seq() != 0 {
		t.Fatalf("seq should be 0 for writerA (no deltas), got %d", idx.Seq())
	}
}

func TestLoadSeqSetForCurrentWriter(t *testing.T) {
	store := newMockStore()
	now := time.Now().UTC().Truncate(time.Millisecond)

	// currentWriter (writerA) has deltas up to seq 7.
	for _, seq := range []uint64{3, 7, 5} {
		ops := []manifest.Op{{Operation: "put", Key: "bucket/x.parquet", Entry: sampleEntryAt(int64(seq), now), Ts: now}}
		d, _ := manifest.MarshalDelta(ops)
		store.put(manifest.DeltaKey(prefix, writerA, seq), d)
	}

	// writerB has higher delta seq (10) but is not the current writer.
	opsB := []manifest.Op{{Operation: "put", Key: "bucket/y.parquet", Entry: sampleEntryAt(1, now), Ts: now}}
	dB, _ := manifest.MarshalDelta(opsB)
	store.put(manifest.DeltaKey(prefix, writerB, 10), dB)

	idx := manifest.New()
	if err := manifest.Load(context.Background(), idx, prefix, writerA, store.lister(), store.fetcher()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// seq must be 7 (max delta for writerA), not 10 (writerB).
	if idx.Seq() != 7 {
		t.Fatalf("seq: want 7, got %d", idx.Seq())
	}
}

func TestLoadFetchErrorPropagates(t *testing.T) {
	store := newMockStore()
	now := time.Now().UTC()

	// Put a delta key in the listing but NOT in the fetch store, so fetch fails.
	store.put(manifest.DeltaKey(prefix, writerA, 1), nil) // ensures listing returns the key
	brokenFetch := func(ctx context.Context, key string) ([]byte, error) {
		return nil, fmt.Errorf("simulated fetch error")
	}

	idx := manifest.New()
	err := manifest.Load(context.Background(), idx, prefix, writerA, store.lister(), brokenFetch)
	if err == nil {
		t.Fatal("expected error from broken fetcher")
	}
	_ = now
}

func TestLoadListErrorPropagates(t *testing.T) {
	brokenList := func(ctx context.Context, prefix, token string) ([]string, string, error) {
		return nil, "", fmt.Errorf("simulated list error")
	}
	idx := manifest.New()
	err := manifest.Load(context.Background(), idx, prefix, writerA, brokenList, func(_ context.Context, _ string) ([]byte, error) {
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error from broken lister")
	}
}

func TestLoadPaginatedList(t *testing.T) {
	// Lister returns keys across two pages to exercise pagination.
	now := time.Now().UTC().Truncate(time.Millisecond)

	ops1 := []manifest.Op{{Operation: "put", Key: "bucket/pg1.parquet", Entry: sampleEntryAt(1, now), Ts: now}}
	d1, _ := manifest.MarshalDelta(ops1)
	ops2 := []manifest.Op{{Operation: "put", Key: "bucket/pg2.parquet", Entry: sampleEntryAt(2, now), Ts: now}}
	d2, _ := manifest.MarshalDelta(ops2)

	key1 := manifest.DeltaKey(prefix, writerA, 1)
	key2 := manifest.DeltaKey(prefix, writerA, 2)

	pageStore := map[string][]byte{key1: d1, key2: d2}

	// Page 1 returns key1 + token; page 2 returns key2 + no token.
	callCount := 0
	pagedList := func(ctx context.Context, listPrefix, token string) ([]string, string, error) {
		callCount++
		switch callCount {
		case 1:
			return []string{key1}, "page2token", nil
		case 2:
			return []string{key2}, "", nil
		default:
			return nil, "", fmt.Errorf("unexpected list call %d", callCount)
		}
	}

	fetcher := func(ctx context.Context, key string) ([]byte, error) {
		d, ok := pageStore[key]
		if !ok {
			return nil, fmt.Errorf("not found: %s", key)
		}
		return d, nil
	}

	idx := manifest.New()
	if err := manifest.Load(context.Background(), idx, prefix, writerA, pagedList, fetcher); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if idx.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", idx.Len())
	}
	if callCount != 2 {
		t.Fatalf("expected 2 list calls, got %d", callCount)
	}
	if idx.Seq() != 2 {
		t.Fatalf("seq: want 2, got %d", idx.Seq())
	}
}

// sampleEntryAt creates an Entry with a given size and LastModified.
func sampleEntryAt(size int64, ts time.Time) *manifest.Entry {
	return &manifest.Entry{
		PlaintextSize:   size,
		PlaintextSHA256: "abc",
		IV:              []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		WrappedDEK:      []byte{17, 18, 19, 20},
		BlockSize:       65536,
		ContentType:     "application/octet-stream",
		ETag:            "etag",
		LastModified:    ts,
	}
}
