package manifest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// compactionStore is a simple in-memory object store for compaction tests.
type compactionStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	deleted []string
}

func newCompactionStore() *compactionStore {
	return &compactionStore{objects: make(map[string][]byte)}
}

func (s *compactionStore) put(key string, data []byte) {
	s.mu.Lock()
	s.objects[key] = data
	s.mu.Unlock()
}

func (s *compactionStore) get(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.objects[key]
	return d, ok
}

func (s *compactionStore) keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.objects))
	for k := range s.objects {
		out = append(out, k)
	}
	return out
}

func (s *compactionStore) lister() Lister {
	return func(ctx context.Context, prefix, token string) ([]string, string, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		var keys []string
		for k := range s.objects {
			if strings.HasPrefix(k, prefix) {
				keys = append(keys, k)
			}
		}
		return keys, "", nil
	}
}

func (s *compactionStore) uploader() Uploader {
	return func(ctx context.Context, key string, data []byte) error {
		cp := make([]byte, len(data))
		copy(cp, data)
		s.mu.Lock()
		s.objects[key] = cp
		s.mu.Unlock()
		return nil
	}
}

func (s *compactionStore) deleter() Deleter {
	return func(ctx context.Context, keys []string) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, k := range keys {
			delete(s.objects, k)
			s.deleted = append(s.deleted, k)
		}
		return nil
	}
}

func (s *compactionStore) deletedKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.deleted))
	copy(out, s.deleted)
	return out
}

const compTestPrefix = ".armor/manifest"
const compTestWriter = "writer-c"

// putDelta inserts a fake delta file into the store at the given seq.
func putDelta(s *compactionStore, seq uint64) {
	key := DeltaKey(compTestPrefix, compTestWriter, seq)
	s.put(key, []byte(`{"op":"put","key":"b/k","ts":"2026-01-01T00:00:00Z"}`+"\n"))
}

// TestCompact_NoOpWhenSeqZero verifies that compaction is a no-op when nothing
// has been written (seq == 0).
func TestCompact_NoOpWhenSeqZero(t *testing.T) {
	idx := New()
	store := newCompactionStore()

	c := NewCompactor(idx, compTestPrefix, compTestWriter,
		store.uploader(), store.lister(), store.deleter(),
		time.Hour, 0)

	if err := c.doCompact(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No snapshot should have been uploaded.
	if _, ok := store.get(SnapshotKey(compTestPrefix, compTestWriter)); ok {
		t.Error("snapshot should not exist when seq=0")
	}
}

// TestCompact_SnapshotUploadedAndDeltasDeleted verifies the happy path:
// snapshot is uploaded and all delta files with seq <= compaction point are
// deleted, while deltas with seq > compaction point survive.
func TestCompact_SnapshotUploadedAndDeltasDeleted(t *testing.T) {
	idx := New()
	idx.Put("bucket", "file.parquet", &Entry{
		PlaintextSize: 1234,
		ETag:          "etag-1",
		LastModified:  time.Now().UTC(),
	})
	idx.SetSeq(5) // pretend 5 deltas have been written

	store := newCompactionStore()
	// Populate delta files 1-5 in B2.
	for seq := uint64(1); seq <= 5; seq++ {
		putDelta(store, seq)
	}
	// Delta 6 was written concurrently during compaction and must survive.
	putDelta(store, 6)

	c := NewCompactor(idx, compTestPrefix, compTestWriter,
		store.uploader(), store.lister(), store.deleter(),
		time.Hour, 0)

	if err := c.doCompact(context.Background()); err != nil {
		t.Fatalf("doCompact: %v", err)
	}

	// Snapshot must exist.
	snapData, ok := store.get(SnapshotKey(compTestPrefix, compTestWriter))
	if !ok || len(snapData) == 0 {
		t.Fatal("snapshot.json.gz was not uploaded")
	}

	// Verify the snapshot round-trips.
	restored := New()
	if err := restored.UnmarshalSnapshot(snapData); err != nil {
		t.Fatalf("snapshot round-trip: %v", err)
	}
	if restored.Len() != 1 {
		t.Fatalf("expected 1 entry in snapshot, got %d", restored.Len())
	}
	if _, ok := restored.Get("bucket", "file.parquet"); !ok {
		t.Error("file.parquet missing from snapshot")
	}

	// Deltas 1-5 must be gone; delta 6 must survive.
	for seq := uint64(1); seq <= 5; seq++ {
		key := DeltaKey(compTestPrefix, compTestWriter, seq)
		if _, ok := store.get(key); ok {
			t.Errorf("delta-%d should have been deleted", seq)
		}
	}
	key6 := DeltaKey(compTestPrefix, compTestWriter, 6)
	if _, ok := store.get(key6); !ok {
		t.Error("delta-6 should survive (seq > compaction point)")
	}
}

// TestCompact_NoDeltasToDelete verifies that compaction succeeds even when
// there are no delta files to remove (e.g., first compaction ever).
func TestCompact_NoDeltasToDelete(t *testing.T) {
	idx := New()
	idx.Put("bucket", "f.parquet", &Entry{ETag: "x", LastModified: time.Now().UTC()})
	idx.SetSeq(3) // seq=3 but no files in store

	store := newCompactionStore()

	c := NewCompactor(idx, compTestPrefix, compTestWriter,
		store.uploader(), store.lister(), store.deleter(),
		time.Hour, 0)

	if err := c.doCompact(context.Background()); err != nil {
		t.Fatalf("doCompact: %v", err)
	}
	// Snapshot still uploaded.
	if _, ok := store.get(SnapshotKey(compTestPrefix, compTestWriter)); !ok {
		t.Error("snapshot should be uploaded even if no deltas to delete")
	}
	if len(store.deletedKeys()) != 0 {
		t.Errorf("no deltas should be deleted, got %v", store.deletedKeys())
	}
}

// TestCompact_SnapshotOverwritesPrevious verifies that a second compaction
// overwrites the previous snapshot.
func TestCompact_SnapshotOverwritesPrevious(t *testing.T) {
	idx := New()
	idx.Put("b", "k1", &Entry{PlaintextSize: 10, LastModified: time.Now().UTC()})
	idx.SetSeq(2)

	store := newCompactionStore()
	putDelta(store, 1)
	putDelta(store, 2)

	c := NewCompactor(idx, compTestPrefix, compTestWriter,
		store.uploader(), store.lister(), store.deleter(),
		time.Hour, 0)

	// First compaction.
	if err := c.doCompact(context.Background()); err != nil {
		t.Fatalf("first compact: %v", err)
	}

	// Add more entries and advance seq.
	idx.Put("b", "k2", &Entry{PlaintextSize: 20, LastModified: time.Now().UTC()})
	idx.SetSeq(4)
	putDelta(store, 3)
	putDelta(store, 4)

	// Second compaction — snapshot must reflect both k1 and k2.
	if err := c.doCompact(context.Background()); err != nil {
		t.Fatalf("second compact: %v", err)
	}

	snapData, _ := store.get(SnapshotKey(compTestPrefix, compTestWriter))
	restored := New()
	if err := restored.UnmarshalSnapshot(snapData); err != nil {
		t.Fatalf("snapshot round-trip: %v", err)
	}
	if restored.Len() != 2 {
		t.Fatalf("expected 2 entries after second compaction, got %d", restored.Len())
	}
}

// TestCompact_UploadError propagates errors from the uploader.
func TestCompact_UploadError(t *testing.T) {
	idx := New()
	idx.SetSeq(1)

	failUpload := func(ctx context.Context, key string, data []byte) error {
		return fmt.Errorf("simulated upload failure")
	}
	store := newCompactionStore()

	c := NewCompactor(idx, compTestPrefix, compTestWriter,
		failUpload, store.lister(), store.deleter(),
		time.Hour, 0)

	err := c.doCompact(context.Background())
	if err == nil {
		t.Fatal("expected error from upload failure")
	}
}

// TestCompact_DeleteError propagates errors from the deleter.
func TestCompact_DeleteError(t *testing.T) {
	idx := New()
	idx.SetSeq(1)

	store := newCompactionStore()
	putDelta(store, 1)

	failDelete := func(ctx context.Context, keys []string) error {
		return fmt.Errorf("simulated delete failure")
	}

	c := NewCompactor(idx, compTestPrefix, compTestWriter,
		store.uploader(), store.lister(), failDelete,
		time.Hour, 0)

	err := c.doCompact(context.Background())
	if err == nil {
		t.Fatal("expected error from delete failure")
	}
}

// TestCompactor_ThresholdTrigger verifies that NotifyDelta triggers early
// compaction once the threshold is reached.
func TestCompactor_ThresholdTrigger(t *testing.T) {
	idx := New()
	idx.Put("b", "k", &Entry{ETag: "x", LastModified: time.Now().UTC()})
	idx.SetSeq(3)

	store := newCompactionStore()
	for seq := uint64(1); seq <= 3; seq++ {
		putDelta(store, seq)
	}

	c := NewCompactor(idx, compTestPrefix, compTestWriter,
		store.uploader(), store.lister(), store.deleter(),
		time.Hour, 3) // threshold = 3

	c.Start(context.Background())
	defer c.Stop()

	// Signal 3 deltas — should trigger compaction.
	c.NotifyDelta()
	c.NotifyDelta()
	c.NotifyDelta()

	// Poll until snapshot appears or timeout.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := store.get(SnapshotKey(compTestPrefix, compTestWriter)); ok {
			return // compaction ran
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("compaction did not run within 2s after threshold reached")
}

// TestCompactor_StopIsIdempotent verifies that Stop can be called multiple times.
func TestCompactor_StopIsIdempotent(t *testing.T) {
	idx := New()
	store := newCompactionStore()
	c := NewCompactor(idx, compTestPrefix, compTestWriter,
		store.uploader(), store.lister(), store.deleter(),
		time.Hour, 0)
	c.Start(context.Background())
	c.Stop()
	c.Stop() // must not panic or deadlock
}

// TestCompactor_ContextCancellationStops verifies that context cancellation
// stops the background goroutine.
func TestCompactor_ContextCancellationStops(t *testing.T) {
	idx := New()
	store := newCompactionStore()
	ctx, cancel := context.WithCancel(context.Background())
	c := NewCompactor(idx, compTestPrefix, compTestWriter,
		store.uploader(), store.lister(), store.deleter(),
		time.Hour, 0)
	c.Start(ctx)
	cancel()
	c.Stop() // must return without deadlock
}

// TestCompactor_DeltaCountResetAfterCompaction verifies that the internal
// delta counter is reset so that the next threshold trigger requires a full
// threshold count of new deltas.
func TestCompactor_DeltaCountResetAfterCompaction(t *testing.T) {
	idx := New()
	idx.Put("b", "k", &Entry{ETag: "y", LastModified: time.Now().UTC()})
	idx.SetSeq(2)

	store := newCompactionStore()
	putDelta(store, 1)
	putDelta(store, 2)

	c := NewCompactor(idx, compTestPrefix, compTestWriter,
		store.uploader(), store.lister(), store.deleter(),
		time.Hour, 2) // threshold = 2

	c.Start(context.Background())
	defer c.Stop()

	// First threshold.
	c.NotifyDelta()
	c.NotifyDelta()

	// Wait for first compaction.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := store.get(SnapshotKey(compTestPrefix, compTestWriter)); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// After compaction, deltasSince should be 0. One more NotifyDelta should
	// not immediately trigger again (needs threshold=2 more calls).
	c.mu.Lock()
	count := c.deltasSince
	c.mu.Unlock()
	if count != 0 {
		t.Errorf("deltasSince should be 0 after compaction, got %d", count)
	}
}
