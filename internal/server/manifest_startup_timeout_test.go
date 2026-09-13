package server

import (
	"os"
	"testing"

	"github.com/jedarden/armor/internal/manifest"
)

// TestManifestStartupLoadUnderTimeoutBound proves the startup-load deadline is
// seconds-scaled and does not break the happy path: a second instance running
// with ARMOR_MANIFEST_LOAD_TIMEOUT=1 (one second) still loads the delta a
// previous instance flushed. A deadline scaled in milliseconds or nanoseconds
// would already be expired by the time New() reaches the load, the load would
// land in the empty-index fallback, and the assertion below would see 0
// entries instead of 1.
func TestManifestStartupLoadUnderTimeoutBound(t *testing.T) {
	root := t.TempDir()

	// Seed the store with one flushed delta from a default-configured writer.
	seeder := newManifestTenantServer(t, root, "p/", "writer-p")
	seeder.manifestWriter.EnqueuePut("shared-bucket", "ledger/row-1", &manifest.Entry{
		PlaintextSize: 10,
		BlockSize:     65536,
	}, nil)
	flushTenantManifest(t, seeder)

	// newManifestTenantServer restores only the env vars it set, so bound the
	// load here with our own save/restore around the second Load().
	prev, had := os.LookupEnv("ARMOR_MANIFEST_LOAD_TIMEOUT")
	os.Setenv("ARMOR_MANIFEST_LOAD_TIMEOUT", "1")
	t.Cleanup(func() {
		if had {
			os.Setenv("ARMOR_MANIFEST_LOAD_TIMEOUT", prev)
		} else {
			os.Unsetenv("ARMOR_MANIFEST_LOAD_TIMEOUT")
		}
	})

	bounded := newManifestTenantServer(t, root, "p/", "writer-p-2")
	if got := bounded.manifest.Len(); got != 1 {
		t.Errorf("instance with ARMOR_MANIFEST_LOAD_TIMEOUT=1 loaded %d manifest entries, want 1; store holds:%s",
			got, listStoreKeys(t, root))
	}
}
