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

// uploader extends mockStore (defined in loader_test.go) with an Uploader
// so the same in-memory store can be used as both the write target for the
// Writer and the read source for Load.
func (m *mockStore) uploader() manifest.Uploader {
	return func(_ context.Context, key string, data []byte) error {
		cp := make([]byte, len(data))
		copy(cp, data)
		m.put(key, cp)
		return nil
	}
}

// TestWriterLoadRoundtrip verifies the full write → persist → restore cycle:
// Writer enqueues Put/Delete operations → flushes delta files to the store →
// Load reads those delta files and reconstructs an equivalent index.
func TestWriterLoadRoundtrip(t *testing.T) {
	const (
		roundtripPrefix = ".armor/manifest"
		roundtripWriter = "roundtrip-writer"
		roundtripBucket = "bucket"
	)

	store := newMockStore()
	idx := manifest.New()

	w := manifest.NewWriter(idx, roundtripPrefix, roundtripWriter, store.uploader(), 64)
	w.Start(context.Background())

	now := time.Now().UTC().Truncate(time.Millisecond)
	e1 := &manifest.Entry{
		PlaintextSize:   1000,
		PlaintextSHA256: "sha256-e1",
		IV:              []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		WrappedDEK:      []byte{20, 21, 22, 23},
		BlockSize:       65536,
		ContentType:     "application/octet-stream",
		ETag:            "etag-e1",
		LastModified:    now,
	}
	e2 := &manifest.Entry{
		PlaintextSize:   2000,
		PlaintextSHA256: "sha256-e2",
		IV:              []byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
		WrappedDEK:      []byte{30, 31, 32, 33},
		BlockSize:       65536,
		ContentType:     "application/json",
		ETag:            "etag-e2",
		LastModified:    now,
	}

	// Enqueue two puts and a put-then-delete for a third key.
	w.EnqueuePut(roundtripBucket, "data/file1.parquet", e1)
	w.EnqueuePut(roundtripBucket, "data/file2.json", e2)
	w.EnqueuePut(roundtripBucket, "data/ephemeral.txt", sampleEntry(5))
	w.EnqueueDelete(roundtripBucket, "data/ephemeral.txt")

	w.Stop() // flush all pending ops before Load

	// Restore a brand-new index from the delta files the writer produced.
	restored := manifest.New()
	if err := manifest.Load(context.Background(), restored, roundtripPrefix, roundtripWriter, store.lister(), store.fetcher()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Ephemeral key was deleted — only two entries should remain.
	if restored.Len() != 2 {
		t.Fatalf("expected 2 entries after roundtrip, got %d", restored.Len())
	}

	got1, ok := restored.Get(roundtripBucket, "data/file1.parquet")
	if !ok {
		t.Fatal("data/file1.parquet missing from restored index")
	}
	if got1.PlaintextSize != e1.PlaintextSize {
		t.Errorf("file1 PlaintextSize: got %d, want %d", got1.PlaintextSize, e1.PlaintextSize)
	}
	if got1.ETag != e1.ETag {
		t.Errorf("file1 ETag: got %q, want %q", got1.ETag, e1.ETag)
	}
	if got1.ContentType != e1.ContentType {
		t.Errorf("file1 ContentType: got %q, want %q", got1.ContentType, e1.ContentType)
	}
	if got1.BlockSize != e1.BlockSize {
		t.Errorf("file1 BlockSize: got %d, want %d", got1.BlockSize, e1.BlockSize)
	}
	if fmt.Sprintf("%x", got1.IV) != fmt.Sprintf("%x", e1.IV) {
		t.Error("file1 IV does not match")
	}
	if fmt.Sprintf("%x", got1.WrappedDEK) != fmt.Sprintf("%x", e1.WrappedDEK) {
		t.Error("file1 WrappedDEK does not match")
	}

	got2, ok := restored.Get(roundtripBucket, "data/file2.json")
	if !ok {
		t.Fatal("data/file2.json missing from restored index")
	}
	if got2.PlaintextSize != e2.PlaintextSize {
		t.Errorf("file2 PlaintextSize: got %d, want %d", got2.PlaintextSize, e2.PlaintextSize)
	}

	if _, ok := restored.Get(roundtripBucket, "data/ephemeral.txt"); ok {
		t.Error("data/ephemeral.txt should be absent (deleted before flush)")
	}

	// The restored sequence counter must reflect the writer's output.
	if restored.Seq() == 0 {
		t.Error("restored.Seq() should be > 0 (writer produced at least one delta)")
	}
}

// TestWriterLoadRoundtripSnapshotThenDeltas verifies that when a snapshot
// is present alongside delta files, Load correctly applies deltas on top of
// the snapshot — the full persist→load cycle including compaction output.
func TestWriterLoadRoundtripSnapshotThenDeltas(t *testing.T) {
	const (
		rtPrefix = ".armor/manifest"
		rtWriter = "rt-writer-2"
		rtBucket = "bucket"
	)
	earlier := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	now := time.Now().UTC().Truncate(time.Millisecond)

	// Build a snapshot representing the compacted state after the first batch.
	snapIdx := manifest.New()
	snapIdx.Put(rtBucket, "snap/a.parquet", sampleEntryAt(100, earlier))
	snapIdx.Put(rtBucket, "snap/b.parquet", sampleEntryAt(200, earlier))
	snapData, err := snapIdx.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}

	store := newMockStore()
	store.put(manifest.SnapshotKey(rtPrefix, rtWriter), snapData)

	// Simulate one new delta written after compaction: update b and add c.
	ops := []manifest.Op{
		{Operation: "put", Key: rtBucket + "/snap/b.parquet", Entry: sampleEntryAt(999, now), Ts: now},
		{Operation: "put", Key: rtBucket + "/snap/c.parquet", Entry: sampleEntryAt(300, now), Ts: now},
	}
	delta, err := manifest.MarshalDelta(ops)
	if err != nil {
		t.Fatalf("MarshalDelta: %v", err)
	}
	store.put(manifest.DeltaKey(rtPrefix, rtWriter, 1), delta)

	restored := manifest.New()
	if err := manifest.Load(context.Background(), restored, rtPrefix, rtWriter, store.lister(), store.fetcher()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// a: from snapshot (unchanged), b: updated by delta, c: added by delta.
	if restored.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", restored.Len())
	}

	aEntry, _ := restored.Get(rtBucket, "snap/a.parquet")
	if aEntry == nil || aEntry.PlaintextSize != 100 {
		t.Errorf("snap/a: want size 100, got %v", aEntry)
	}

	bEntry, _ := restored.Get(rtBucket, "snap/b.parquet")
	if bEntry == nil || bEntry.PlaintextSize != 999 {
		t.Errorf("snap/b: delta should override snapshot, want 999, got %v", bEntry)
	}

	if _, ok := restored.Get(rtBucket, "snap/c.parquet"); !ok {
		t.Error("snap/c.parquet (from delta) should be present")
	}

	if restored.Seq() != 1 {
		t.Errorf("seq: want 1, got %d", restored.Seq())
	}
}

// TestDeltaReplaySequenceOrder verifies that delta files are always applied
// in ascending sequence-number order, even when the object store lists them
// in a different order (e.g., reverse alphabetical or arbitrary map iteration).
//
// If this property were violated, a later delta that overwrites a key would
// be eclipsed by an earlier delta applied after it, producing stale data.
func TestDeltaReplaySequenceOrder(t *testing.T) {
	const (
		orderPrefix = ".armor/manifest"
		orderWriter = "order-test"
		orderBucket = "bucket"
	)
	t1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 12, 0, 1, 0, time.UTC)
	t3 := time.Date(2026, 1, 1, 12, 0, 2, 0, time.UTC)

	// Delta 1: put fileA with size=1.
	d1, _ := manifest.MarshalDelta([]manifest.Op{
		{Operation: "put", Key: orderBucket + "/fileA", Entry: sampleEntryAt(1, t1), Ts: t1},
	})
	// Delta 2: overwrite fileA with size=2 (this must win over delta 1).
	d2, _ := manifest.MarshalDelta([]manifest.Op{
		{Operation: "put", Key: orderBucket + "/fileA", Entry: sampleEntryAt(2, t2), Ts: t2},
	})
	// Delta 3: add fileB and delete fileA (fileA must be absent in the end).
	d3, _ := manifest.MarshalDelta([]manifest.Op{
		{Operation: "del", Key: orderBucket + "/fileA", Ts: t3},
		{Operation: "put", Key: orderBucket + "/fileB", Entry: sampleEntryAt(3, t3), Ts: t3},
	})

	k1 := manifest.DeltaKey(orderPrefix, orderWriter, 1)
	k2 := manifest.DeltaKey(orderPrefix, orderWriter, 2)
	k3 := manifest.DeltaKey(orderPrefix, orderWriter, 3)
	objStore := map[string][]byte{k1: d1, k2: d2, k3: d3}

	// Use a reverse-order lister to confirm the loader sorts keys before replay.
	reverseLister := func(_ context.Context, p, _ string) ([]string, string, error) {
		var keys []string
		for k := range objStore {
			if strings.HasPrefix(k, p) {
				keys = append(keys, k)
			}
		}
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
		return keys, "", nil
	}
	fetcher := func(_ context.Context, key string) ([]byte, error) {
		d, ok := objStore[key]
		if !ok {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return d, nil
	}

	idx := manifest.New()
	if err := manifest.Load(context.Background(), idx, orderPrefix, orderWriter, reverseLister, fetcher); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Delta 3 deletes fileA — it must be absent regardless of listing order.
	if _, ok := idx.Get(orderBucket, "fileA"); ok {
		t.Error("fileA should be absent: delta-3 (del) must be applied after delta-2 (put)")
	}

	// fileB was added by delta 3.
	if _, ok := idx.Get(orderBucket, "fileB"); !ok {
		t.Error("fileB should be present (added by delta-3)")
	}

	if idx.Len() != 1 {
		t.Fatalf("expected exactly 1 entry (fileB), got %d", idx.Len())
	}

	if idx.Seq() != 3 {
		t.Errorf("seq: want 3, got %d", idx.Seq())
	}
}

// TestDeltaReplayDeleteThenPut verifies that a delete followed by a put
// for the same key (across separate deltas) leaves the key present.
// This is the inverse of TestDeltaReplaySequenceOrder and exercises the
// tombstone-then-resurrect pattern.
func TestDeltaReplayDeleteThenPut(t *testing.T) {
	const (
		dtpPrefix = ".armor/manifest"
		dtpWriter = "dtp-writer"
		dtpBucket = "bucket"
	)
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Second)

	// Delta 1: delete key (key may not exist yet, but this covers the case
	// where a prior snapshot had it and the new delta removes it).
	d1, _ := manifest.MarshalDelta([]manifest.Op{
		{Operation: "del", Key: dtpBucket + "/key", Ts: t1},
	})
	// Delta 2: re-add the same key.
	d2, _ := manifest.MarshalDelta([]manifest.Op{
		{Operation: "put", Key: dtpBucket + "/key", Entry: sampleEntryAt(42, t2), Ts: t2},
	})

	store := newMockStore()
	store.put(manifest.DeltaKey(dtpPrefix, dtpWriter, 1), d1)
	store.put(manifest.DeltaKey(dtpPrefix, dtpWriter, 2), d2)

	idx := manifest.New()
	if err := manifest.Load(context.Background(), idx, dtpPrefix, dtpWriter, store.lister(), store.fetcher()); err != nil {
		t.Fatalf("Load: %v", err)
	}

	e, ok := idx.Get(dtpBucket, "key")
	if !ok {
		t.Fatal("key should be present after del-then-put in sequence order")
	}
	if e.PlaintextSize != 42 {
		t.Errorf("PlaintextSize: got %d, want 42", e.PlaintextSize)
	}
	if idx.Seq() != 2 {
		t.Errorf("seq: want 2, got %d", idx.Seq())
	}
}
