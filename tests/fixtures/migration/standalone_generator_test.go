package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// hashTree maps every regular file under root (slash-separated relative path)
// to its SHA-256, so whole trees can be compared byte-for-byte.
func hashTree(t *testing.T, root string) map[string]string {
	t.Helper()
	sums := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sums[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return sums
}

func requireTreesIdentical(t *testing.T, got, want map[string]string, label string) {
	t.Helper()
	var gotKeys, wantKeys []string
	for k := range got {
		gotKeys = append(gotKeys, k)
	}
	for k := range want {
		wantKeys = append(wantKeys, k)
	}
	sort.Strings(gotKeys)
	sort.Strings(wantKeys)
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("%s: file sets differ (got %d files, want %d)", label, len(gotKeys), len(wantKeys))
	}
	for i, k := range gotKeys {
		if k != wantKeys[i] {
			t.Fatalf("%s: file sets differ: %q vs %q", label, k, wantKeys[i])
		}
		if got[k] != want[k] {
			t.Errorf("%s: %s content differs (sha256 %s, want %s)", label, k, got[k], want[k])
		}
	}
}

// generateRegenerationInto runs the exact main() regeneration-set code path
// into a fresh temp directory and returns its hashed generated_fixtures tree.
func generateRegenerationInto(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	gen, err := NewFixtureGenerator(dir)
	if err != nil {
		t.Fatalf("NewFixtureGenerator: %v", err)
	}
	if err := writeRegenerationSet(gen); err != nil {
		t.Fatalf("writeRegenerationSet: %v", err)
	}
	return hashTree(t, filepath.Join(dir, "generated_fixtures"))
}

// TestRegenerationSetIsDeterministic pins acceptance criterion 2: two
// consecutive runs produce byte-identical generated_fixtures trees.
func TestRegenerationSetIsDeterministic(t *testing.T) {
	first := generateRegenerationInto(t)
	second := generateRegenerationInto(t)
	requireTreesIdentical(t, second, first, "two consecutive runs")
}

// TestCommittedRegenerationSetMatchesGenerator fails when the committed
// generated_fixtures/ tree drifts from what the current generator produces —
// regenerate (see README.md) and commit instead of updating either side by hand.
func TestCommittedRegenerationSetMatchesGenerator(t *testing.T) {
	fresh := generateRegenerationInto(t)
	committed := hashTree(t, "generated_fixtures")
	requireTreesIdentical(t, fresh, committed, "committed generated_fixtures/ tree")
}

// TestRegenManifestCoversEveryFixture pins the manifest schema (acceptance
// criterion 3) and cross-checks each entry against the per-fixture
// metadata.json and the artifact bytes on disk.
func TestRegenManifestCoversEveryFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("generated_fixtures", "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest RegenManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if manifest.ManifestVersion != 1 {
		t.Errorf("manifest_version = %d, want 1", manifest.ManifestVersion)
	}
	if !manifest.Deterministic {
		t.Errorf("deterministic = false, want true")
	}

	fixtures, err := os.ReadDir("generated_fixtures")
	if err != nil {
		t.Fatalf("list generated_fixtures: %v", err)
	}
	var wantIDs []string
	for _, f := range fixtures {
		if f.IsDir() {
			wantIDs = append(wantIDs, f.Name())
		}
	}
	sort.Strings(wantIDs)

	if len(manifest.Entries) != len(wantIDs) {
		t.Fatalf("manifest has %d entries, want %d (one per fixture dir)", len(manifest.Entries), len(wantIDs))
	}
	for i, entry := range manifest.Entries {
		id := wantIDs[i]
		if entry.FixtureID != id {
			t.Errorf("entry[%d].fixture_id = %q, want %q (entries must be sorted and complete)", i, entry.FixtureID, id)
			continue
		}
		if entry.FormatVersion != "v1" && entry.FormatVersion != "v2" {
			t.Errorf("%s: format_version = %q, want v1 or v2", id, entry.FormatVersion)
		}
		if len(entry.PlaintextSHA256) != 64 {
			t.Errorf("%s: plaintext_sha256 = %q, want 64 hex chars", id, entry.PlaintextSHA256)
		}
		if entry.PlaintextLength <= 0 {
			t.Errorf("%s: plaintext_length = %d, want > 0", id, entry.PlaintextLength)
		}
		if entry.V3Expected.SidecarPath == "" && entry.V3Expected.IsMultipart {
			t.Errorf("%s: v3_expected marks multipart without a sidecar_path", id)
		}

		// Cross-check against the per-fixture metadata.json.
		metaRaw, err := os.ReadFile(filepath.Join("generated_fixtures", id, "metadata.json"))
		if err != nil {
			t.Fatalf("read %s metadata: %v", id, err)
		}
		var meta FixtureMetadata
		if err := json.Unmarshal(metaRaw, &meta); err != nil {
			t.Fatalf("parse %s metadata: %v", id, err)
		}
		if entry.PlaintextSHA256 != meta.PlaintextSHA256 || entry.PlaintextLength != meta.PlaintextLength ||
			entry.FormatVersion != meta.SourceVersion {
			t.Errorf("%s: manifest entry disagrees with metadata.json", id)
		}

		// Every artifact must exist with the recorded size and hash.
		if len(entry.Artifacts) == 0 {
			t.Errorf("%s: no artifacts recorded", id)
			continue
		}
		for _, art := range entry.Artifacts {
			path := filepath.Join("generated_fixtures", id, art.Name)
			sum, size, err := hashFile(path)
			if err != nil {
				t.Errorf("%s/%s: %v", id, art.Name, err)
				continue
			}
			if size != art.Bytes || sum != art.SHA256 {
				t.Errorf("%s/%s: sha256/size mismatch (manifest %s/%d, disk %s/%d)",
					id, art.Name, art.SHA256, art.Bytes, sum, size)
			}
		}
	}
}
