package manifest

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/metrics"
)

// ADR-001 "Internal Namespaces": with ARMOR_PREFIX in force, config composes
// the manifest prefix into "<ARMOR_PREFIX>.armor/manifest" before the writer
// and compactor see it, so a tenant's snapshots and deltas are internal
// bookkeeping inside that tenant's namespace. The isolation and confinement
// tests in internal/server cover the delta half end to end; this pins the
// snapshot half at the compactor level: given the COMPOSED prefix a real
// prefixed deployment passes in, compaction must upload snapshot.json.gz
// beneath it, enumerate and delete deltas only beneath it, and leave nothing
// at the bucket-root .armor/manifest an unprefixed deployment uses.

const composedTenantPrefix = "tenant-a/.armor/manifest" // what config.Load yields for ARMOR_PREFIX=tenant-a/
const composedTenantWriter = "writer-p"

func putComposedPrefixDelta(s *compactionStore, seq uint64) {
	key := DeltaKey(composedTenantPrefix, composedTenantWriter, seq)
	s.put(key, []byte(`{"op":"put","key":"b/k","ts":"2026-01-01T00:00:00Z"}`+"\n"))
}

func TestCompact_PlacesSnapshotUnderComposedTenantPrefix(t *testing.T) {
	idx := New()
	idx.Put("bucket", "ledger/row-1", &Entry{
		PlaintextSize: 10,
		BlockSize:     65536,
		ETag:          "etag-1",
		LastModified:  time.Now().UTC(),
	})
	idx.SetSeq(3) // pretend 3 deltas have been written

	store := newCompactionStore()
	for seq := uint64(1); seq <= 3; seq++ {
		putComposedPrefixDelta(store, seq)
	}
	var buf bytes.Buffer
	logger := log.New(&buf, "[test-compact] ", log.LstdFlags|log.Lmsgprefix)

	c := NewCompactor(idx, composedTenantPrefix, composedTenantWriter,
		store.uploader(), store.lister(), store.deleter(),
		time.Hour, 0, logger, metrics.NewMetrics())

	if err := c.doCompact(context.Background()); err != nil {
		t.Fatalf("doCompact: %v", err)
	}

	// The snapshot must sit inside the tenant namespace, at
	// <ARMOR_PREFIX>.armor/manifest/<writer>/snapshot.json.gz.
	snapKey := SnapshotKey(composedTenantPrefix, composedTenantWriter)
	if snapKey != "tenant-a/.armor/manifest/writer-p/snapshot.json.gz" {
		t.Fatalf("SnapshotKey built %q, want the composed tenant-namespace path", snapKey)
	}
	snapData, ok := store.get(snapKey)
	if !ok || len(snapData) == 0 {
		t.Fatalf("snapshot was not uploaded under the composed prefix; store holds: %v", storeKeyList(store))
	}

	// The snapshot must round-trip and carry the tenant's entry.
	restored := New()
	if err := restored.UnmarshalSnapshot(snapData); err != nil {
		t.Fatalf("snapshot round-trip: %v", err)
	}
	if restored.Len() != 1 {
		t.Fatalf("expected 1 entry in snapshot, got %d", restored.Len())
	}
	if _, ok := restored.Get("bucket", "ledger/row-1"); !ok {
		t.Error("ledger/row-1 missing from snapshot")
	}

	// The compacted deltas must be gone from the composed location...
	for seq := uint64(1); seq <= 3; seq++ {
		if _, ok := store.get(DeltaKey(composedTenantPrefix, composedTenantWriter, seq)); ok {
			t.Errorf("delta-%d should have been deleted from the tenant namespace", seq)
		}
	}
	// ...and nothing may exist at the bucket-root .armor/manifest, which is
	// where an unprefixed deployment (or an uncomposed writer) would put them.
	for _, k := range storeKeyList(store) {
		if strings.HasPrefix(k, ".armor/manifest/") {
			t.Errorf("internal object landed at the bucket root despite the tenant prefix: %s", k)
		}
	}
}

// storeKeyList renders the store's keys for failure messages.
func storeKeyList(s *compactionStore) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.objects))
	for k := range s.objects {
		keys = append(keys, k)
	}
	return keys
}
