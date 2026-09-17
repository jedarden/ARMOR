// Acceptance pin for the generated V1 single-object fixture with explicit
// version metadata (armor-53bcdd78). The fixture-matrix and golden suites
// prove the whole generated_fixtures/ set decrypts; this file additionally
// pins what that bead's acceptance criteria name directly:
//
//   - which metadata field carries the explicit format version, its value,
//     and that the production parser reads it (README "The explicit
//     format-version field" section)
//   - that the field agrees with the envelope header's version byte, so the
//     fixture is not accidentally a header-vs-metadata contradiction
//   - that the production legacy reader decrypts the committed bytes to the
//     plaintext the regen manifest records — manifest against decrypted
//     bytes, not manifest against itself
//   - the omission variant (armor-3aac661d): field absent, parser defaults
//     to V1, the derivation-driving header byte agrees with that default,
//     bytes still decrypt to the plaintext the regen manifest records
//
// Verification only. The fixture bytes themselves are emitted by
// tests/fixtures/migration/standalone_generator.go, which deliberately links
// no ARMOR code (see criterion 5 of the bead); nothing here may be used to
// produce them.
package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// regenManifestEntry mirrors the slice of generated_fixtures/manifest.json
// this test consumes. The canonical schema types live in the generator's
// package main and cannot be imported; duplicating the three checked fields
// keeps a schema drift a compile-time-free but test-visible event.
type regenManifestEntry struct {
	FixtureID       string `json:"fixture_id"`
	FormatVersion   string `json:"format_version"`
	PlaintextSHA256 string `json:"plaintext_sha256"`
	PlaintextLength int64  `json:"plaintext_length"`
	V3Expected      struct {
		IsMultipart bool `json:"is_multipart"`
	} `json:"v3_expected"`
}

type regenManifest struct {
	Entries []regenManifestEntry `json:"entries"`
}

// regenManifestEntryFor loads generated_fixtures/manifest.json and returns
// the entry for fixtureID.
func regenManifestEntryFor(t *testing.T, fixtureID string) regenManifestEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goldenFixtureRoot(t), "generated_fixtures", "manifest.json"))
	if err != nil {
		t.Fatalf("read generated_fixtures manifest: %v", err)
	}
	var manifest regenManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse generated_fixtures manifest: %v", err)
	}
	for _, entry := range manifest.Entries {
		if entry.FixtureID == fixtureID {
			return entry
		}
	}
	t.Fatalf("generated_fixtures manifest has no entry for %s", fixtureID)
	return regenManifestEntry{}
}

func TestGeneratedV1ExplicitFixtureLegacyReader(t *testing.T) {
	byName, _ := goldenMatrixFixtures(t)
	f, ok := byName["generated_fixtures/v1-single-explicit-short"]
	if !ok {
		t.Fatal("generated_fixtures/v1-single-explicit-short not on disk; run the standalone generator")
	}

	// The explicit version field: present, "1", and read by the production
	// metadata parser (not defaulted).
	if got := f.ObjectMeta["x-amz-meta-armor-version"]; got != "1" {
		t.Fatalf("x-amz-meta-armor-version = %q, want %q — the fixture must carry its format version explicitly", got, "1")
	}
	armorMeta, ok := backend.ParseARMORMetadata(f.ObjectMeta)
	if !ok {
		t.Fatal("object_metadata.json did not parse as ARMOR metadata")
	}
	if armorMeta.Version != 1 {
		t.Fatalf("ParseARMORMetadata version = %d, want 1 (read from x-amz-meta-armor-version)", armorMeta.Version)
	}
	if len(f.Data) < crypto.HeaderSize {
		t.Fatalf("stored_ciphertext.bin is %d bytes, shorter than the %d-byte envelope header", len(f.Data), crypto.HeaderSize)
	}

	// The explicit field must agree with the envelope header's version byte;
	// a disagreement would make this a contradictory-version fixture, not an
	// explicit one.
	header, err := crypto.DecodeHeader(f.Data[:crypto.HeaderSize])
	if err != nil {
		t.Fatalf("committed bytes do not decode as an envelope header: %v", err)
	}
	if header.Version != crypto.Version1 {
		t.Fatalf("envelope header version = %d, want %d", header.Version, crypto.Version1)
	}
	if uint8(armorMeta.Version) != header.Version {
		t.Fatalf("metadata version %d disagrees with envelope header version %d", armorMeta.Version, header.Version)
	}
	if int(header.PlaintextSize) != f.Meta.PlaintextLength {
		t.Fatalf("header plaintext size = %d, metadata.json records %d", header.PlaintextSize, f.Meta.PlaintextLength)
	}
	if got := hex.EncodeToString(header.PlaintextSHA[:]); got != f.Meta.PlaintextSHA256 {
		t.Fatalf("header plaintext sha256 = %s, metadata.json records %s", got, f.Meta.PlaintextSHA256)
	}

	// The production legacy (pre-V3) reader over the committed artifacts.
	plaintext, err := decryptGoldenFixture(t, f, "generated/v1-single-explicit-short")
	if err != nil {
		t.Fatalf("legacy reader failed to decrypt the committed fixture: %v", err)
	}
	checkGoldenPlaintext(t, f, plaintext)

	// The regen manifest is machine-checkable against the decrypted bytes:
	// what the reader got is what the manifest says to expect.
	entry := regenManifestEntryFor(t, "v1-single-explicit-short")
	sum := sha256.Sum256(plaintext)
	if got := hex.EncodeToString(sum[:]); got != entry.PlaintextSHA256 {
		t.Fatalf("decrypted plaintext sha256 = %s, manifest records %s", got, entry.PlaintextSHA256)
	}
	if int64(len(plaintext)) != entry.PlaintextLength {
		t.Fatalf("decrypted plaintext length = %d, manifest records %d", len(plaintext), entry.PlaintextLength)
	}
	if entry.FormatVersion != "v1" {
		t.Fatalf("manifest format_version = %q, want \"v1\"", entry.FormatVersion)
	}
	if entry.V3Expected.IsMultipart {
		t.Fatal("manifest v3_expected.is_multipart = true for a single-PUT fixture")
	}
}

// TestGeneratedV1ImplicitFixtureDefaultsToV1 pins the omission variant's
// documented reader behavior (armor-3aac661d): without the version field the
// production parser defaults to V1, the envelope header byte that actually
// drives single-PUT decryption agrees with that default, and the committed
// bytes still decrypt to exactly what the regen manifest records.
func TestGeneratedV1ImplicitFixtureDefaultsToV1(t *testing.T) {
	byName, _ := goldenMatrixFixtures(t)
	f, ok := byName["generated_fixtures/v1-single-implicit-short"]
	if !ok {
		t.Fatal("generated_fixtures/v1-single-implicit-short not on disk; run the standalone generator")
	}

	// The omission: the field is absent from the committed object metadata,
	// and the production parser's backward-compat default is V1.
	if _, has := f.ObjectMeta["x-amz-meta-armor-version"]; has {
		t.Fatal("implicit fixture carries x-amz-meta-armor-version; it must omit it to exercise the fallback")
	}
	armorMeta, ok := backend.ParseARMORMetadata(f.ObjectMeta)
	if !ok {
		t.Fatal("object_metadata.json did not parse as ARMOR metadata")
	}
	if armorMeta.Version != 1 {
		t.Fatalf("ParseARMORMetadata defaulted missing version to %d, want 1", armorMeta.Version)
	}

	// The derivation path: single-PUT decryption takes its version from the
	// envelope header's byte, never from the (defaulted) metadata — so the
	// header must be a genuine V1 header agreeing with the default, or this
	// would be a contradictory-version fixture instead of an implicit one.
	if len(f.Data) < crypto.HeaderSize {
		t.Fatalf("stored_ciphertext.bin is %d bytes, shorter than the %d-byte envelope header", len(f.Data), crypto.HeaderSize)
	}
	header, err := crypto.DecodeHeader(f.Data[:crypto.HeaderSize])
	if err != nil {
		t.Fatalf("committed bytes do not decode as an envelope header: %v", err)
	}
	if header.Version != crypto.Version1 {
		t.Fatalf("envelope header version = %d, want %d", header.Version, crypto.Version1)
	}
	if uint8(armorMeta.Version) != header.Version {
		t.Fatalf("defaulted metadata version %d disagrees with envelope header version %d", armorMeta.Version, header.Version)
	}
	if int(header.PlaintextSize) != f.Meta.PlaintextLength {
		t.Fatalf("header plaintext size = %d, metadata.json records %d", header.PlaintextSize, f.Meta.PlaintextLength)
	}
	if got := hex.EncodeToString(header.PlaintextSHA[:]); got != f.Meta.PlaintextSHA256 {
		t.Fatalf("header plaintext sha256 = %s, metadata.json records %s", got, f.Meta.PlaintextSHA256)
	}

	// The production legacy (pre-V3) reader over the committed artifacts.
	plaintext, err := decryptGoldenFixture(t, f, "generated/v1-single-implicit-short")
	if err != nil {
		t.Fatalf("legacy reader failed to decrypt the implicit-V1 fixture: %v", err)
	}
	checkGoldenPlaintext(t, f, plaintext)

	// The regen manifest is machine-checkable against the decrypted bytes,
	// exactly as for the explicit sibling: what the reader got is what the
	// manifest says to expect.
	entry := regenManifestEntryFor(t, "v1-single-implicit-short")
	sum := sha256.Sum256(plaintext)
	if got := hex.EncodeToString(sum[:]); got != entry.PlaintextSHA256 {
		t.Fatalf("decrypted plaintext sha256 = %s, manifest records %s", got, entry.PlaintextSHA256)
	}
	if int64(len(plaintext)) != entry.PlaintextLength {
		t.Fatalf("decrypted plaintext length = %d, manifest records %d", len(plaintext), entry.PlaintextLength)
	}
	if entry.FormatVersion != "v1" {
		t.Fatalf("manifest format_version = %q, want \"v1\"", entry.FormatVersion)
	}
	if entry.V3Expected.IsMultipart {
		t.Fatal("manifest v3_expected.is_multipart = true for a single-PUT fixture")
	}
}
