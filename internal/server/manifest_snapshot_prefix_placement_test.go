package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/manifest"
)

// TestManifestSnapshotPlacesUnderTenantPrefix is the snapshot half of the
// ADR-001 internal-namespace placement guarantee. The delta half is covered by
// TestManifestPrefixIsolatesTenantsInOneBucket; here a prefixed tenant's
// manifest actually compacts (threshold 1) and the snapshot must physically
// land at <ARMOR_PREFIX>.armor/manifest/<writer>/snapshot.json.gz inside the
// tenant namespace — never at the bucket-root .armor/manifest — and a second
// instance of the same tenant must load its entries from that snapshot alone,
// because compaction deletes the deltas it absorbed.
func TestManifestSnapshotPlacesUnderTenantPrefix(t *testing.T) {
	root := t.TempDir()

	// Threshold 1: the first delta flush triggers an early compaction, so the
	// test needs no timer manipulation. Set before the helper's config.Load.
	t.Setenv("ARMOR_MANIFEST_COMPACTION_THRESHOLD", "1")

	tenantP := newManifestTenantServer(t, root, "p/", "writer-s", "")
	// Mirror the handler flow: RecordPut-style write to the in-memory index
	// (what the compactor will snapshot) plus EnqueuePut for persistence —
	// the writer is a persistence queue and never touches the index itself.
	entry := &manifest.Entry{PlaintextSize: 10, BlockSize: 65536}
	tenantP.manifest.Put("shared-bucket", "ledger/row-1", entry)
	tenantP.manifestWriter.EnqueuePut("shared-bucket", "ledger/row-1", entry, nil)
	flushTenantManifest(t, tenantP)

	snapshotPath := filepath.Join(root, "shared-bucket", "p", ".armor", "manifest", "writer-s", "snapshot.json.gz")

	// The compactor runs asynchronously after the writer's flush; wait for it.
	waitForFile(t, root, snapshotPath, 10*time.Second)

	// Compaction deleted the deltas it absorbed from the tenant namespace.
	deltaGlob := filepath.Join(root, "shared-bucket", "p", ".armor", "manifest", "writer-s", "delta-*.jsonl")
	waitForGlobEmpty(t, deltaGlob, 10*time.Second)

	// Nothing may sit at the bucket-root .armor/manifest — the uncomposed
	// location an unprefixed deployment uses, and where the pre-fix manifest
	// write landed (commit 2347a039 is the regression this guards).
	rootManifest := filepath.Join(root, "shared-bucket", ".armor", "manifest")
	if _, err := os.Stat(rootManifest); !os.IsNotExist(err) {
		t.Errorf("internal manifest objects exist at the bucket root %s (err %v) — store holds:%s",
			rootManifest, err, listStoreKeys(t, root))
	}

	// A second p/ instance must load the entry from the snapshot alone.
	againP := newManifestTenantServer(t, root, "p/", "writer-s-2", "")
	if got := againP.manifest.Len(); got != 1 {
		t.Errorf("second p/ instance loaded %d manifest entries, want 1 (from the snapshot); store holds:%s",
			got, listStoreKeys(t, root))
	}
}

// waitForFile polls until path exists or the deadline passes.
func waitForFile(t *testing.T, root, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s did not appear within %s — store holds:%s", path, timeout, listStoreKeys(t, root))
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// waitForGlobEmpty polls until glob matches nothing or the deadline passes.
func waitForGlobEmpty(t *testing.T, glob string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		matches, err := filepath.Glob(glob)
		if err != nil {
			t.Fatalf("glob %s: %v", glob, err)
		}
		if len(matches) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("deltas still present after %s (compaction should have deleted them): %v", timeout, matches)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
