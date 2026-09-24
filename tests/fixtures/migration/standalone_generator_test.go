package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
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

// deterministicPlaintext returns an n-byte plaintext filled with the fixed
// byte(i % 256) pattern, the same fill main() uses.
func deterministicPlaintext(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 256)
	}
	return b
}

// malformedMatrixRows is the malformed/* inventory of goldenFixtureMatrix
// (internal/server/format_migration_fixture_matrix_test.go), sorted. The
// generator must emit exactly this set of directory names — no more, no less.
var malformedMatrixRows = []string{
	"corrupted_hmac_table",
	"corrupted_wrapped_dek_tag",
	"envelope_version_mismatch",
	"inconsistent_part_metadata",
	"invalid_envelope_magic",
	"invalid_sidecar_format",
	"invalid_version_string",
	"multipart_contradictory_hashes",
	"multipart_part_size_mismatch",
	"truncated_ciphertext",
	"truncated_sidecar",
	"v1_object_v2_metadata",
	"v2_object_v1_metadata",
}

// generateMalformedSet emits every malformed/* fixture main() emits, into
// root, via the same generator calls. Plaintexts mirror main()'s shapes at
// unit-test scale: a single-block single-PUT plaintext, a four-block multipart
// plaintext (so sidecar defects have >= 2 HMAC entries to work with), and a
// two-part plaintext sized against the 512 KiB parts the part-size mismatch
// base is written with.
func generateMalformedSet(t *testing.T, root string) {
	t.Helper()
	gen, err := NewFixtureGenerator(root)
	if err != nil {
		t.Fatalf("NewFixtureGenerator: %v", err)
	}
	single := deterministicPlaintext(200)
	fourBlocks := deterministicPlaintext(4 * 65536)
	twoParts := deterministicPlaintext(2 * 512 * 1024)
	partSize := 5 * 1024 * 1024

	mustBundle := func(bundle *FixtureBundle, err error) *FixtureBundle {
		t.Helper()
		if err != nil {
			t.Fatalf("malformed generation failed: %v", err)
		}
		return bundle
	}

	fixtures := map[string]*FixtureBundle{
		"invalid_version_string":         mustBundle(gen.GenerateMalformedInvalidVersion(single)),
		"envelope_version_mismatch":      mustBundle(gen.GenerateMalformedEnvelopeVersionMismatch(single)),
		"corrupted_hmac_table":           mustBundle(gen.GenerateMalformedCorruptedHMAC(single)),
		"inconsistent_part_metadata":     mustBundle(gen.GenerateMalformedInconsistentPartMetadata(single)),
		"invalid_envelope_magic":         mustBundle(gen.GenerateMalformedInvalidEnvelopeMagic(single)),
		"truncated_ciphertext":           mustBundle(gen.GenerateMalformedTruncatedCiphertext(single)),
		"corrupted_wrapped_dek_tag":      mustBundle(gen.GenerateMalformedCorruptedWrappedDEKTag(single)),
		"invalid_sidecar_format":         mustBundle(gen.GenerateMalformedInvalidSidecarFormat(fourBlocks, partSize)),
		"truncated_sidecar":              mustBundle(gen.GenerateMalformedTruncatedSidecar(fourBlocks, partSize)),
		"multipart_part_size_mismatch":   mustBundle(gen.GenerateMalformedMultipartPartSizeMismatch(twoParts)),
		"multipart_contradictory_hashes": mustBundle(gen.GenerateMalformedMultipartContradictoryHashes(fourBlocks, partSize)),
		"v1_object_v2_metadata":          mustBundle(gen.GenerateMalformedV1ObjectV2Metadata(single)),
		"v2_object_v1_metadata":          mustBundle(gen.GenerateMalformedV2ObjectV1Metadata(single)),
	}
	for _, row := range malformedMatrixRows {
		bundle, ok := fixtures[row]
		if !ok {
			t.Fatalf("internal: matrix row %s has no generation call", row)
		}
		if err := gen.WriteFixture(filepath.Join("malformed", row), bundle); err != nil {
			t.Fatalf("write malformed/%s: %v", row, err)
		}
	}
}

// TestMalformedSetMatchesMatrixRows pins the acceptance property: the
// generator emits every malformed/* row of goldenFixtureMatrix — names
// exactly equal — and every emitted fixture declares a failure outcome.
func TestMalformedSetMatchesMatrixRows(t *testing.T) {
	dir := t.TempDir()
	generateMalformedSet(t, dir)

	entries, err := os.ReadDir(filepath.Join(dir, "malformed"))
	if err != nil {
		t.Fatalf("read malformed/: %v", err)
	}
	var got []string
	for _, e := range entries {
		if e.IsDir() {
			got = append(got, e.Name())
		}
	}
	sort.Strings(got)

	if len(got) != len(malformedMatrixRows) {
		t.Fatalf("malformed/ holds %d dirs (%v), want exactly the %d matrix rows (%v)",
			len(got), got, len(malformedMatrixRows), malformedMatrixRows)
	}
	for i := range got {
		if got[i] != malformedMatrixRows[i] {
			t.Fatalf("malformed/ dir %q is not a matrix row (want %q); full set: %v", got[i], malformedMatrixRows[i], got)
		}
	}

	for _, name := range got {
		raw, err := os.ReadFile(filepath.Join(dir, "malformed", name, "metadata.json"))
		if err != nil {
			t.Fatalf("read malformed/%s metadata: %v", name, err)
		}
		var meta FixtureMetadata
		if err := json.Unmarshal(raw, &meta); err != nil {
			t.Fatalf("parse malformed/%s metadata: %v", name, err)
		}
		if meta.SourceVersion != "malformed" {
			t.Errorf("malformed/%s: source_version = %q, want malformed", name, meta.SourceVersion)
		}
		if meta.ExpectedMigrationOutcome != "failure" {
			t.Errorf("malformed/%s: expected_migration_outcome = %q, want failure", name, meta.ExpectedMigrationOutcome)
		}
		if meta.ExpectedFailureReason == "" {
			t.Errorf("malformed/%s: expected_failure_reason is empty", name)
		}
	}
}

// TestMalformedSetIsDeterministic pins the second acceptance property: two
// generation runs produce byte-identical malformed/ trees.
func TestMalformedSetIsDeterministic(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	generateMalformedSet(t, first)
	generateMalformedSet(t, second)
	requireTreesIdentical(t,
		hashTree(t, filepath.Join(first, "malformed")),
		hashTree(t, filepath.Join(second, "malformed")),
		"two consecutive malformed runs")
}

// requireSameMalformedFacts asserts the facts a corruption must never touch
// (the documented plaintext) plus the malformed markers it must carry.
func requireSameMalformedFacts(t *testing.T, base, derived *FixtureBundle, label string) {
	t.Helper()
	if derived.Metadata.PlaintextSHA256 != base.Metadata.PlaintextSHA256 {
		t.Errorf("%s: plaintext_sha256 changed (%q -> %q)", label,
			base.Metadata.PlaintextSHA256, derived.Metadata.PlaintextSHA256)
	}
	if derived.Metadata.PlaintextLength != base.Metadata.PlaintextLength {
		t.Errorf("%s: plaintext_length changed (%d -> %d)", label,
			base.Metadata.PlaintextLength, derived.Metadata.PlaintextLength)
	}
	if derived.Metadata.SourceVersion != "malformed" {
		t.Errorf("%s: source_version = %q, want malformed", label, derived.Metadata.SourceVersion)
	}
	if derived.Metadata.ExpectedMigrationOutcome != "failure" {
		t.Errorf("%s: expected_migration_outcome = %q, want failure", label,
			derived.Metadata.ExpectedMigrationOutcome)
	}
	if derived.Metadata.ExpectedFailureReason == "" {
		t.Errorf("%s: expected_failure_reason is empty", label)
	}
}

// requireOnlyMetadataKeysDiffer asserts both bundles carry the same metadata
// keys and exactly the named keys differ in value (zero keys: identical).
func requireOnlyMetadataKeysDiffer(t *testing.T, base, derived *FixtureBundle, label string, keys ...string) {
	t.Helper()
	allowed := make(map[string]bool, len(keys))
	for _, k := range keys {
		allowed[k] = true
	}
	if len(base.ObjectMetadata) != len(derived.ObjectMetadata) {
		t.Fatalf("%s: metadata holds %d keys, base has %d", label,
			len(derived.ObjectMetadata), len(base.ObjectMetadata))
	}
	for k, want := range base.ObjectMetadata {
		got, ok := derived.ObjectMetadata[k]
		if !ok {
			t.Errorf("%s: metadata key %q is missing", label, k)
			continue
		}
		if got == want && allowed[k] {
			t.Errorf("%s: metadata key %q must differ from the base but does not", label, k)
		}
		if got != want && !allowed[k] {
			t.Errorf("%s: metadata key %q differs from the base unexpectedly (%q -> %q)", label, k, want, got)
		}
	}
}

// requireBytesIdentical asserts two byte payloads are equal.
func requireBytesIdentical(t *testing.T, want, got []byte, what string) {
	t.Helper()
	if !bytes.Equal(want, got) {
		t.Errorf("%s: bytes differ (len %d vs %d)", what, len(want), len(got))
	}
}

// diffPositions returns the indices where two equal-length byte slices differ.
func diffPositions(a, b []byte) []int {
	var diffs []int
	for i := range a {
		if a[i] != b[i] {
			diffs = append(diffs, i)
		}
	}
	return diffs
}

// TestMalformedCorruptionIsolated pins, for each of the nine generators added
// for the missing malformed/* matrix rows, that the derived bundle differs
// from its valid base exactly in the intended field — one targeted
// byte-level corruption, nothing else.
func TestMalformedCorruptionIsolated(t *testing.T) {
	gen, err := NewFixtureGenerator(t.TempDir())
	if err != nil {
		t.Fatalf("NewFixtureGenerator: %v", err)
	}

	single := deterministicPlaintext(200)
	fourBlocks := deterministicPlaintext(4 * 65536)
	twoParts := deterministicPlaintext(2 * 512 * 1024)
	partSize := 5 * 1024 * 1024

	cases := []struct {
		name  string
		base  func() (*FixtureBundle, error)
		gen   func() (*FixtureBundle, error)
		check func(t *testing.T, base, derived *FixtureBundle)
	}{
		{
			name: "invalid_envelope_magic",
			base: func() (*FixtureBundle, error) { return gen.GenerateV1SingleExplicit(single) },
			gen:  func() (*FixtureBundle, error) { return gen.GenerateMalformedInvalidEnvelopeMagic(single) },
			check: func(t *testing.T, base, derived *FixtureBundle) {
				requireOnlyMetadataKeysDiffer(t, base, derived, "invalid_envelope_magic")
				if len(derived.StoredCiphertext) != len(base.StoredCiphertext) {
					t.Fatalf("invalid_envelope_magic: envelope is %d bytes, base is %d",
						len(derived.StoredCiphertext), len(base.StoredCiphertext))
				}
				if !bytes.Equal(derived.StoredCiphertext[0:4], []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
					t.Fatalf("invalid_envelope_magic: magic = % x, want de ad be ef", derived.StoredCiphertext[0:4])
				}
				requireBytesIdentical(t, base.StoredCiphertext[4:], derived.StoredCiphertext[4:],
					"invalid_envelope_magic: bytes after the magic")
			},
		},
		{
			name: "truncated_ciphertext",
			base: func() (*FixtureBundle, error) { return gen.GenerateV1SingleExplicit(single) },
			gen:  func() (*FixtureBundle, error) { return gen.GenerateMalformedTruncatedCiphertext(single) },
			check: func(t *testing.T, base, derived *FixtureBundle) {
				requireOnlyMetadataKeysDiffer(t, base, derived, "truncated_ciphertext")
				if len(derived.StoredCiphertext) >= envelopeHeaderSize+32 {
					t.Fatalf("truncated_ciphertext: %d stored bytes still hold a 32-byte HMAC table; the truncation is too small to express the defect",
						len(derived.StoredCiphertext))
				}
				if !bytes.Equal(derived.StoredCiphertext, base.StoredCiphertext[:len(derived.StoredCiphertext)]) {
					t.Fatalf("truncated_ciphertext: truncated envelope is not a prefix of the base envelope")
				}
			},
		},
		{
			name: "corrupted_wrapped_dek_tag",
			base: func() (*FixtureBundle, error) { return gen.GenerateV1SingleExplicit(single) },
			gen:  func() (*FixtureBundle, error) { return gen.GenerateMalformedCorruptedWrappedDEKTag(single) },
			check: func(t *testing.T, base, derived *FixtureBundle) {
				requireBytesIdentical(t, base.StoredCiphertext, derived.StoredCiphertext,
					"corrupted_wrapped_dek_tag: stored ciphertext")
				requireOnlyMetadataKeysDiffer(t, base, derived, "corrupted_wrapped_dek_tag",
					"x-amz-meta-armor-wrapped-dek")
				baseWrap, err := base64.StdEncoding.DecodeString(base.ObjectMetadata["x-amz-meta-armor-wrapped-dek"])
				if err != nil {
					t.Fatalf("decode base wrapped DEK: %v", err)
				}
				gotWrap, err := base64.StdEncoding.DecodeString(derived.ObjectMetadata["x-amz-meta-armor-wrapped-dek"])
				if err != nil {
					t.Fatalf("decode derived wrapped DEK: %v", err)
				}
				if len(baseWrap) != 40 || len(gotWrap) != 40 {
					t.Fatalf("corrupted_wrapped_dek_tag: wrapped DEK is %d/%d bytes, want 40/40 (a length check must not fire instead of the tag check)",
						len(baseWrap), len(gotWrap))
				}
				diffs := diffPositions(baseWrap, gotWrap)
				if len(diffs) != 1 || diffs[0] != len(baseWrap)-1 {
					t.Fatalf("corrupted_wrapped_dek_tag: wrapped DEK differs at %v, want exactly [%d] (the final KWP byte)",
						diffs, len(baseWrap)-1)
				}
			},
		},
		{
			name: "invalid_sidecar_format",
			base: func() (*FixtureBundle, error) { return gen.GenerateV1Multipart(fourBlocks, partSize) },
			gen:  func() (*FixtureBundle, error) { return gen.GenerateMalformedInvalidSidecarFormat(fourBlocks, partSize) },
			check: func(t *testing.T, base, derived *FixtureBundle) {
				requireOnlyMetadataKeysDiffer(t, base, derived, "invalid_sidecar_format")
				requireBytesIdentical(t, base.StoredCiphertext, derived.StoredCiphertext,
					"invalid_sidecar_format: stored ciphertext")
				if len(derived.SidecarData) != len(base.SidecarData)-1 {
					t.Fatalf("invalid_sidecar_format: sidecar is %d bytes, want %d (base minus one)",
						len(derived.SidecarData), len(base.SidecarData)-1)
				}
				if len(derived.SidecarData)%32 == 0 {
					t.Fatalf("invalid_sidecar_format: sidecar is still a multiple of 32 bytes; the format defect is gone")
				}
				if !bytes.Equal(derived.SidecarData, base.SidecarData[:len(derived.SidecarData)]) {
					t.Fatalf("invalid_sidecar_format: sidecar is not a prefix of the base sidecar")
				}
			},
		},
		{
			name: "truncated_sidecar",
			base: func() (*FixtureBundle, error) { return gen.GenerateV1Multipart(fourBlocks, partSize) },
			gen:  func() (*FixtureBundle, error) { return gen.GenerateMalformedTruncatedSidecar(fourBlocks, partSize) },
			check: func(t *testing.T, base, derived *FixtureBundle) {
				requireOnlyMetadataKeysDiffer(t, base, derived, "truncated_sidecar")
				requireBytesIdentical(t, base.StoredCiphertext, derived.StoredCiphertext,
					"truncated_sidecar: stored ciphertext")
				if len(derived.SidecarData) != len(base.SidecarData)-32 {
					t.Fatalf("truncated_sidecar: sidecar is %d bytes, want %d (base minus one HMAC entry)",
						len(derived.SidecarData), len(base.SidecarData)-32)
				}
				if len(derived.SidecarData)%32 != 0 {
					t.Fatalf("truncated_sidecar: sidecar is %d bytes, must stay a multiple of 32 so the defect is a missing entry, not a broken format",
						len(derived.SidecarData))
				}
				wantBlocks := (int(derived.Metadata.PlaintextLength) + 65536 - 1) / 65536
				if entries := len(derived.SidecarData) / 32; entries >= wantBlocks {
					t.Fatalf("truncated_sidecar: sidecar holds %d entries, need < %d blocks", entries, wantBlocks)
				}
				if !bytes.Equal(derived.SidecarData, base.SidecarData[:len(derived.SidecarData)]) {
					t.Fatalf("truncated_sidecar: sidecar is not a prefix of the base sidecar")
				}
			},
		},
		{
			name: "multipart_part_size_mismatch",
			base: func() (*FixtureBundle, error) { return gen.GenerateV1Multipart(twoParts, 512*1024) },
			gen:  func() (*FixtureBundle, error) { return gen.GenerateMalformedMultipartPartSizeMismatch(twoParts) },
			check: func(t *testing.T, base, derived *FixtureBundle) {
				requireOnlyMetadataKeysDiffer(t, base, derived, "multipart_part_size_mismatch",
					"x-amz-meta-armor-part-size")
				if got, want := derived.ObjectMetadata["x-amz-meta-armor-part-size"], "307200"; got != want {
					t.Fatalf("multipart_part_size_mismatch: declared part size = %q, want %q", got, want)
				}
				requireBytesIdentical(t, base.StoredCiphertext, derived.StoredCiphertext,
					"multipart_part_size_mismatch: stored ciphertext")
				requireBytesIdentical(t, base.SidecarData, derived.SidecarData,
					"multipart_part_size_mismatch: sidecar")
				// The declared size must actually move a boundary: the same
				// plaintext splits differently under 300 KiB and 512 KiB parts.
				const declared = 300 * 1024
				derivedParts := (int(derived.Metadata.PlaintextLength) + declared - 1) / declared
				if actualParts := derived.Metadata.V3Expected.PartCount; derivedParts == actualParts {
					t.Fatalf("multipart_part_size_mismatch: declared %d B splits the plaintext into %d parts, same as the actual %d; the contradiction is not expressible",
						declared, derivedParts, actualParts)
				}
			},
		},
		{
			name: "multipart_contradictory_hashes",
			base: func() (*FixtureBundle, error) { return gen.GenerateV1Multipart(fourBlocks, partSize) },
			gen: func() (*FixtureBundle, error) {
				return gen.GenerateMalformedMultipartContradictoryHashes(fourBlocks, partSize)
			},
			check: func(t *testing.T, base, derived *FixtureBundle) {
				requireOnlyMetadataKeysDiffer(t, base, derived, "multipart_contradictory_hashes",
					"x-amz-meta-armor-sha256")
				got := derived.ObjectMetadata["x-amz-meta-armor-sha256"]
				if len(got) != 64 {
					t.Fatalf("multipart_contradictory_hashes: sha256 = %q, want 64 hex digits", got)
				}
				if got == base.ObjectMetadata["x-amz-meta-armor-sha256"] {
					t.Fatalf("multipart_contradictory_hashes: sha256 metadata still matches the base")
				}
				if got == derived.Metadata.PlaintextSHA256 {
					t.Fatalf("multipart_contradictory_hashes: sha256 metadata still matches the documented plaintext digest")
				}
				requireBytesIdentical(t, base.StoredCiphertext, derived.StoredCiphertext,
					"multipart_contradictory_hashes: stored ciphertext")
				requireBytesIdentical(t, base.SidecarData, derived.SidecarData,
					"multipart_contradictory_hashes: sidecar")
			},
		},
		{
			name: "v1_object_v2_metadata",
			base: func() (*FixtureBundle, error) { return gen.GenerateV1SingleExplicit(single) },
			gen:  func() (*FixtureBundle, error) { return gen.GenerateMalformedV1ObjectV2Metadata(single) },
			check: func(t *testing.T, base, derived *FixtureBundle) {
				requireOnlyMetadataKeysDiffer(t, base, derived, "v1_object_v2_metadata",
					"x-amz-meta-armor-version")
				if got := derived.ObjectMetadata["x-amz-meta-armor-version"]; got != "2" {
					t.Fatalf("v1_object_v2_metadata: version metadata = %q, want %q", got, "2")
				}
				// The object itself must stay genuine V1: bytes untouched,
				// header version byte still 1.
				requireBytesIdentical(t, base.StoredCiphertext, derived.StoredCiphertext,
					"v1_object_v2_metadata: stored ciphertext")
				if derived.StoredCiphertext[4] != 0x01 {
					t.Fatalf("v1_object_v2_metadata: header version byte = %#x, want 0x01 (a genuine V1 envelope)",
						derived.StoredCiphertext[4])
				}
			},
		},
		{
			name: "v2_object_v1_metadata",
			base: func() (*FixtureBundle, error) { return gen.GenerateV2Single(single) },
			gen:  func() (*FixtureBundle, error) { return gen.GenerateMalformedV2ObjectV1Metadata(single) },
			check: func(t *testing.T, base, derived *FixtureBundle) {
				requireOnlyMetadataKeysDiffer(t, base, derived, "v2_object_v1_metadata",
					"x-amz-meta-armor-version")
				if got := derived.ObjectMetadata["x-amz-meta-armor-version"]; got != "1" {
					t.Fatalf("v2_object_v1_metadata: version metadata = %q, want %q", got, "1")
				}
				// The object itself must stay genuine V2: bytes untouched,
				// header version byte still 2.
				requireBytesIdentical(t, base.StoredCiphertext, derived.StoredCiphertext,
					"v2_object_v1_metadata: stored ciphertext")
				if derived.StoredCiphertext[4] != 0x02 {
					t.Fatalf("v2_object_v1_metadata: header version byte = %#x, want 0x02 (a genuine V2 envelope)",
						derived.StoredCiphertext[4])
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, err := tc.base()
			if err != nil {
				t.Fatalf("generate base: %v", err)
			}
			derived, err := tc.gen()
			if err != nil {
				t.Fatalf("generate malformed bundle: %v", err)
			}
			requireSameMalformedFacts(t, base, derived, tc.name)
			tc.check(t, base, derived)
		})
	}
}
