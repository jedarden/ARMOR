package server

// ADR-001 "Internal Namespaces" coexistence, on the manifest index itself.
// A bucket that gained its ARMOR_PREFIX after ARMOR had already been writing
// to it keeps pre-prefix internal objects at the bucket root, and the ADR
// keeps root `.armor/` alive precisely for the deployments that never set a
// prefix. So the composed manifest location and the legacy root location must
// both work, without ever mixing: a prefixed instance loads only
// <prefix>.armor/manifest/ (a root delta left by a pre-prefix era must never
// enter its index — keys are client-visible, so ingesting it is the
// cross-tenant contamination ADR-001 describes), and an unprefixed instance
// still writes and loads the root.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/manifest"
)

// seedRootLegacyDelta writes a genuine manifest delta object at the bucket
// root, as a pre-prefix era instance would have left it. The delta carries a
// put of legacyKey distinguishable by plaintext size from anything a prefixed
// tenant writes, so a load that ingests it is observable both as an extra
// entry and as a wrong ciphertext ref for a client-visible key.
func seedRootLegacyDelta(t *testing.T, root, bucket, legacyKey string) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Millisecond)
	ops := []manifest.Op{
		{
			Operation: "put",
			Key:       bucket + "/" + legacyKey,
			Entry: &manifest.Entry{
				PlaintextSize: 999,
				BlockSize:     65536,
				LastModified:  now,
			},
			Ts: now,
		},
	}
	delta, err := manifest.MarshalDelta(ops)
	if err != nil {
		t.Fatalf("MarshalDelta: %v", err)
	}
	// manifest.DeltaKey(".armor/manifest", ...) reproduces the key the
	// unprefixed writer would have produced, so the seeded object is exactly
	// what a pre-prefix era leaves behind.
	legacyKeyPath := filepath.Join(root, bucket, filepath.FromSlash(manifest.DeltaKey(".armor/manifest", "legacy-writer", 1)))
	if err := os.MkdirAll(filepath.Dir(legacyKeyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy manifest dir: %v", err)
	}
	if err := os.WriteFile(legacyKeyPath, delta, 0o600); err != nil {
		t.Fatalf("seed legacy manifest delta: %v", err)
	}
}

// TestPrefixedLoadIgnoresPrePrefixRootManifest starts a prefixed instance on a
// store that still holds a pre-prefix root delta and verifies the load brings
// in none of it, while the instance's own prefixed deltas keep loading
// normally alongside it — coexistence without mixing.
func TestPrefixedLoadIgnoresPrePrefixRootManifest(t *testing.T) {
	const (
		bucket     = "shared-bucket"
		ownKey     = "ledger/row-1"
		legacyKey  = "ledger/legacy-echo"
		ownSize    = int64(111)
		legacySize = int64(999)
	)

	root := t.TempDir()
	seedRootLegacyDelta(t, root, bucket, legacyKey)

	// A p/ instance starting cold: the root delta exists, the composed
	// location does not, so the index must come up empty.
	tenantP := newManifestTenantServer(t, root, "p/", "writer-p", "")
	if got := tenantP.manifest.Len(); got != 0 {
		t.Errorf("prefixed instance loaded %d entries from a store holding only a root legacy delta, want 0; store holds:%s",
			got, listStoreKeys(t, root))
	}
	if _, ok := tenantP.manifest.Get(bucket, legacyKey); ok {
		t.Errorf("pre-prefix root delta leaked into the prefixed index: %s is resolvable", legacyKey)
	}

	// The same instance records an ordinary upload and flushes it as a delta
	// under p/.armor/manifest/. A second p/ instance must load exactly that —
	// the legacy root object sitting next to it changes nothing.
	tenantP.manifestWriter.EnqueuePut(bucket, ownKey, &manifest.Entry{
		PlaintextSize: ownSize,
		BlockSize:     65536,
	}, nil)
	flushTenantManifest(t, tenantP)

	againP := newManifestTenantServer(t, root, "p/", "writer-p-2", "")
	if got := againP.manifest.Len(); got != 1 {
		t.Errorf("second p/ instance loaded %d entries, want exactly its own; store holds:%s",
			got, listStoreKeys(t, root))
	}
	e, ok := againP.manifest.Get(bucket, ownKey)
	if !ok {
		t.Fatalf("second p/ instance did not load its own delta for %s", ownKey)
	}
	if e.PlaintextSize != ownSize {
		t.Errorf("loaded %s with plaintext size %d, want %d — a root delta resolved this key", ownKey, e.PlaintextSize, ownSize)
	}
	if _, ok := againP.manifest.Get(bucket, legacyKey); ok {
		t.Errorf("pre-prefix root delta leaked into the second prefixed instance: %s is resolvable", legacyKey)
	}
}

// TestUnprefixedManifestStaysAtBucketRoot is the no-prefix baseline ADR-001
// preserves: an instance without ARMOR_PREFIX writes its deltas at the bucket
// root and a restart loads them from there. The root `.armor/` location is not
// dead code once prefixes exist — it is the live namespace for deployments
// that never set one.
func TestUnprefixedManifestStaysAtBucketRoot(t *testing.T) {
	const (
		bucket  = "shared-bucket"
		ownKey  = "ledger/row-1"
		ownSize = int64(111)
	)

	root := t.TempDir()

	legacy := newManifestTenantServer(t, root, "", "legacy-writer", "")
	legacy.manifestWriter.EnqueuePut(bucket, ownKey, &manifest.Entry{
		PlaintextSize: ownSize,
		BlockSize:     65536,
	}, nil)
	flushTenantManifest(t, legacy)

	// The delta must physically sit at the bucket root — the unprefixed
	// reserved namespace — and nowhere else.
	matches, err := filepath.Glob(filepath.Join(root, bucket, ".armor", "manifest", "*", "*.jsonl"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no delta object under %s — store holds:%s",
			filepath.Join(root, bucket, ".armor", "manifest"), listStoreKeys(t, root))
	}

	restarted := newManifestTenantServer(t, root, "", "legacy-writer-2", "")
	if got := restarted.manifest.Len(); got != 1 {
		t.Errorf("restarted unprefixed instance loaded %d entries, want 1; store holds:%s",
			got, listStoreKeys(t, root))
	}
	if _, ok := restarted.manifest.Get(bucket, ownKey); !ok {
		t.Errorf("restarted unprefixed instance did not load %s from the root manifest", ownKey)
	}
}
