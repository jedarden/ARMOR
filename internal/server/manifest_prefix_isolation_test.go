package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/manifest"
)

// newManifestTenantServer builds a server over the shared store at root with the
// given ADR-001 tenant prefix, exactly as two tenants in one bucket would be
// configured. The caller is responsible for flushing via flushTenantManifest
// before another instance is expected to see what it enqueued.
func newManifestTenantServer(t *testing.T, root, tenantPrefix, writerID string) *Server {
	t.Helper()

	const mek = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	pairs := []string{
		"ARMOR_BACKEND", "filesystem",
		"ARMOR_FS_PATH", root,
		"ARMOR_BUCKET", "shared-bucket",
		"ARMOR_MEK", mek,
		"ARMOR_AUTH_ACCESS_KEY", "test-access-key",
		"ARMOR_AUTH_SECRET_KEY", "test-secret-key",
		"ARMOR_WRITER_ID", writerID,
		"ARMOR_PREFIX", tenantPrefix,
	}
	originals := make(map[string]string, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		k, v := pairs[i], pairs[i+1]
		originals[k] = os.Getenv(k)
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("Setenv(%s): %v", k, err)
		}
	}
	// ARMOR_MANIFEST_PREFIX must not leak in from another test: the default is
	// what a shared-bucket tenant actually runs with.
	prevManifestPrefix, hadManifestPrefix := os.LookupEnv("ARMOR_MANIFEST_PREFIX")
	os.Unsetenv("ARMOR_MANIFEST_PREFIX")

	t.Cleanup(func() {
		for k, v := range originals {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
		if hadManifestPrefix {
			os.Setenv("ARMOR_MANIFEST_PREFIX", prevManifestPrefix)
		} else {
			os.Unsetenv("ARMOR_MANIFEST_PREFIX")
		}
	})

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() for tenant %q: %v", tenantPrefix, err)
	}
	if want := filepath.ToSlash(filepath.Join(tenantPrefix, ".armor/manifest")); cfg.ManifestPrefix != want {
		t.Fatalf("tenant %q got ManifestPrefix %q, want %q", tenantPrefix, cfg.ManifestPrefix, want)
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() for tenant %q: %v", tenantPrefix, err)
	}
	if srv.manifestWriter == nil {
		t.Fatalf("tenant %q has no manifest writer", tenantPrefix)
	}

	// Stop drains and flushes whatever is still enqueued.
	t.Cleanup(srv.manifestWriter.Stop)
	return srv
}

// flushTenantManifest stops the writer, which drains any pending op and uploads
// it as a delta object before returning.
func flushTenantManifest(t *testing.T, srv *Server) {
	t.Helper()
	srv.manifestWriter.Stop()
}

// TestManifestPrefixIsolatesTenantsInOneBucket is the ADR-001 shared-bucket
// scenario: two instances, one store, different ARMOR_PREFIX. Before the
// manifest prefix was composed with the tenant prefix both wrote and loaded
// <bucket>/.armor/manifest/, so an instance started after another had written a
// delta silently indexed that tenant's keys — and keys are client-visible, so a
// cross-tenant collision resolved to the wrong ciphertext ref.
func TestManifestPrefixIsolatesTenantsInOneBucket(t *testing.T) {
	root := t.TempDir()

	// Tenant p records an upload and flushes it as a delta object.
	tenantP := newManifestTenantServer(t, root, "p/", "writer-p")
	tenantP.manifestWriter.EnqueuePut("shared-bucket", "ledger/row-1", &manifest.Entry{
		PlaintextSize: 10,
		BlockSize:     65536,
	}, nil)
	flushTenantManifest(t, tenantP)

	// A second p/ instance must find that delta under p/.armor/manifest/.
	againP := newManifestTenantServer(t, root, "p/", "writer-p-2")
	if got := againP.manifest.Len(); got != 1 {
		t.Errorf("second p/ instance loaded %d manifest entries, want 1; store holds:%s",
			got, listStoreKeys(t, root))
	}

	// A q/ instance in the same bucket must load none of them.
	tenantQ := newManifestTenantServer(t, root, "q/", "writer-q")
	if got := tenantQ.manifest.Len(); got != 0 {
		t.Errorf("q/ instance loaded %d manifest entries from p/'s deltas, want 0", got)
	}

	// And the delta object must physically sit inside the tenant namespace. The
	// filesystem backend nests under <FSPath>/<bucket>/, standing in for the
	// bucket root a B2 key would see.
	matches, err := filepath.Glob(filepath.Join(root, "shared-bucket", "p", ".armor", "manifest", "*", "*.jsonl"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no delta object under %s — store holds:%s",
			filepath.Join(root, "p", ".armor", "manifest"), listStoreKeys(t, root))
	}
}

// listStoreKeys renders everything the store holds, for the failure message.
func listStoreKeys(t *testing.T, root string) string {
	t.Helper()
	var sb strings.Builder
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		sb.WriteString("\n  " + filepath.ToSlash(rel))
		return nil
	})
	if walkErr != nil {
		return "<walk failed: " + walkErr.Error() + ">"
	}
	if sb.Len() == 0 {
		return "<store is empty>"
	}
	return sb.String()
}
