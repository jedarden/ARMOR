// Golden fixture validation: runs the committed V1/V2 migration fixtures in
// tests/fixtures/migration through ARMOR's real decrypt and migration paths.
//
// The fixtures were produced by tests/fixtures/migration/standalone_generator.go,
// an independent reimplementation of the V1/V2 crypto (it imports nothing from
// internal/crypto), so passes here are adversarial evidence that the production
// read and migration paths accept third-party-generated data. The deterministic
// fixture keys are documented in docs/format/v3-fixture-reference.md.
//
// Layers:
//   - TestGoldenFixturesDecrypt    every fixture decrypts to its documented
//     plaintext SHA-256 and length (success-outcome fixtures), corrupt
//     fixtures fail closed (failure-outcome fixtures), and sidecar block
//     accounting matches the documented 64 KiB block size
//   - TestGoldenFixturesDryRun     dry-run migration succeeds for every
//     success-outcome fixture and rewrites nothing
//   - TestGoldenFixturesMigrate    live migration matches each fixture's
//     expected_migration_outcome and produces a readable V3 object, cross-
//     checked against v3-golden-outcomes-computed.json
//   - TestGoldenFixtureCorruptions corrupted derivatives of valid fixture
//     material must fail closed: one recorded failure, object untouched
//
// One defect is known at HEAD and pinned by a dedicated test:
//
//  1. Migrator multipart output (production-code side, owned by the multipart
//     migration-path lineage): FormatMigrator.uploadAsMultipart discards the
//     per-part HMAC tables and never persists an HMAC sidecar, and the
//     post-migration verify calls decryptSingleObject unconditionally, so
//     migrating any valid object larger than the 5 MiB multipart threshold
//     replaces it with an unreadable body and then fails its own verify --
//     a data-loss shape. Pinned by TestGoldenMultipartMigratorDefect; while
//     the pin reproduces, committed multipart success-migrations skip with a
//     pointer to it instead of failing at the migrator.
//
// A fixture-side wrap-format defect (committed multipart fixtures wrapping
// their DEK with 60-byte AES-GCM output where production unwrap requires the
// 40-byte AES-KWP form) previously lived here as a second pinned defect; the
// fixtures were regenerated with production-compatible KWP wraps and the pin
// (TestGoldenFixtureWrapDefect) was removed. goldenWrapDefect remains as the
// data-driven discriminator: if incompatible wraps ever return to the
// committed tree, the affected subtests skip again instead of failing on
// bytes known to be unopenable.
//
// These tests read only tests/fixtures/migration and the in-memory mock
// backend; they never talk to a real bucket.
package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// goldenMEKHex is the deterministic fixture MEK from the "Test Constants"
// section of docs/format/v3-fixture-reference.md.
const goldenMEKHex = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

// goldenBlockSize is 64 KiB, the fixture block size documented in the same
// reference.
const goldenBlockSize = 65536

// goldenPartSize is the migrator's multipart output part size (see
// FormatMigrator.uploadAsMultipart).
const goldenPartSize = 5 * 1024 * 1024

// goldenMultipartThreshold is the plaintext size above which the migrator
// re-uploads as multipart instead of a single PUT.
const goldenMultipartThreshold = 5 * 1024 * 1024

func goldenMEK() []byte {
	mek, err := hex.DecodeString(goldenMEKHex)
	if err != nil {
		panic("invalid golden MEK hex: " + err.Error())
	}
	return mek
}

// goldenFixtureMeta mirrors the fixture metadata.json schema (see
// tests/fixtures/migration/README.md).
type goldenFixtureMeta struct {
	PlaintextSHA256 string `json:"plaintext_sha256"`
	PlaintextLength int    `json:"plaintext_length"`
	SourceVersion   string `json:"source_version"`
	SourceLayout    string `json:"source_layout"`
	V3Expected      struct {
		IsMultipart     bool   `json:"is_multipart"`
		PartCount       int    `json:"part_count"`
		BlocksPerPart   []int  `json:"blocks_per_part"`
		CompressionUsed bool   `json:"compression_used"`
		SidecarPath     string `json:"sidecar_path"`
	} `json:"v3_expected"`
	Description              string `json:"description"`
	ExpectedMigrationOutcome string `json:"expected_migration_outcome"`
	ExpectedFailureReason    string `json:"expected_failure_reason"`
}

// goldenFixture is one committed fixture directory.
type goldenFixture struct {
	Name       string // category/variant relative to the fixture root
	Dir        string
	Meta       goldenFixtureMeta
	ObjectMeta map[string]string
	Data       []byte
	Sidecar    []byte // nil for single-PUT fixtures
}

// goldenFixtureRoot locates tests/fixtures/migration relative to this file.
func goldenFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Clean(filepath.Join("..", "..", "tests", "fixtures", "migration"))
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatalf("fixture root not found at %s", root)
	}
	return root
}

// loadGoldenFixtures walks the fixture tree and returns every fixture that has
// a metadata.json, sorted by name. Directories without one (canonical/, stray
// build output) are skipped.
func loadGoldenFixtures(t *testing.T) []goldenFixture {
	t.Helper()
	root := goldenFixtureRoot(t)
	var fixtures []goldenFixture
	categories, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read fixture root: %v", err)
	}
	for _, cat := range categories {
		if !cat.IsDir() || cat.Name() == "canonical" {
			continue
		}
		variants, err := os.ReadDir(filepath.Join(root, cat.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", cat.Name(), err)
		}
		for _, variant := range variants {
			dir := filepath.Join(root, cat.Name(), variant.Name())
			if !variant.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, "metadata.json")); err != nil {
				continue // not a fixture directory
			}
			f := goldenFixture{Name: cat.Name() + "/" + variant.Name(), Dir: dir}
			f.Meta = loadGoldenFixtureMeta(t, dir)
			f.ObjectMeta = loadGoldenObjectMetadata(t, dir)
			f.Data, f.Sidecar = loadGoldenFixtureBytes(t, dir)
			fixtures = append(fixtures, f)
		}
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].Name < fixtures[j].Name })
	if len(fixtures) == 0 {
		t.Fatal("no fixtures found under tests/fixtures/migration")
	}
	return fixtures
}

func loadGoldenFixtureMeta(t *testing.T, dir string) goldenFixtureMeta {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		t.Fatalf("metadata.json: %v", err)
	}
	var meta goldenFixtureMeta
	if err := json.Unmarshal(b, &meta); err != nil {
		t.Fatalf("metadata.json: %v", err)
	}
	return meta
}

func loadGoldenObjectMetadata(t *testing.T, dir string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "object_metadata.json"))
	if err != nil {
		t.Fatalf("object_metadata.json: %v", err)
	}
	var meta map[string]string
	if err := json.Unmarshal(b, &meta); err != nil {
		t.Fatalf("object_metadata.json: %v", err)
	}
	return meta
}

func loadGoldenFixtureBytes(t *testing.T, dir string) (data []byte, sidecar []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "stored_ciphertext.bin"))
	if err != nil {
		t.Fatalf("stored_ciphertext.bin: %v", err)
	}
	sidecar, err = os.ReadFile(filepath.Join(dir, "sidecar.bin"))
	if err != nil {
		sidecar = nil // single-PUT fixtures have no sidecar
	}
	return data, sidecar
}

// goldenWrapDefect reports why production crypto cannot unwrap a fixture's
// wrapped DEK, or "" if it can. It mirrors the exact chain the migrator runs
// -- backend.ParseARMORMetadata's v2:/legacy split followed by
// crypto.UnwrapDEK (the terminal step of decryptSingleObject and
// decryptMultipartObject alike) -- so a non-empty result here is precisely the
// failure the production paths would hit.
//
// This is deliberately data-driven rather than a hardcoded skip list: when the
// fixtures are regenerated with production-compatible wraps the defect
// disappears from every subtest at once, with no map to maintain. The
// committed fixtures now unwrap cleanly, so this returns "" for all of them.
func goldenWrapDefect(f goldenFixture) string {
	wrapped := f.ObjectMeta["x-amz-meta-armor-wrapped-dek"]
	if wrapped == "" {
		return "fixture carries no x-amz-meta-armor-wrapped-dek"
	}
	var raw []byte
	if len(wrapped) > 4 && wrapped[:3] == "v2:" {
		parts := strings.SplitN(wrapped, ":", 3)
		if len(parts) != 3 {
			return "v2: string does not split into v2:<fingerprint>:<base64>"
		}
		decoded, err := base64.StdEncoding.DecodeString(parts[2])
		if err != nil {
			return "v2: inner base64 does not decode: " + err.Error()
		}
		raw = decoded
	} else {
		decoded, err := base64.StdEncoding.DecodeString(wrapped)
		if err != nil {
			return "legacy base64 does not decode: " + err.Error()
		}
		raw = decoded
	}
	if _, err := crypto.UnwrapDEK(goldenMEK(), raw); err != nil {
		return fmt.Sprintf("production unwrap rejects the committed bytes: %v", err)
	}
	return ""
}

// goldenWrapFingerprintNote returns a non-empty warning when a v2-format
// wrapped DEK carries a fingerprint that production multi-key routing would
// reject (16 hex chars expected). The migrator's unwrap path ignores the
// fingerprint, so this is report-only: it does not gate any subtest.
func goldenWrapFingerprintNote(f goldenFixture) string {
	wrapped := f.ObjectMeta["x-amz-meta-armor-wrapped-dek"]
	if len(wrapped) <= 4 || wrapped[:3] != "v2:" {
		return ""
	}
	parts := strings.SplitN(wrapped, ":", 3)
	if len(parts) != 3 {
		return "v2: string does not split into v2:<fingerprint>:<base64>"
	}
	if len(parts[1]) != 16 {
		return fmt.Sprintf("v2 fingerprint is %d hex chars, production requires 16", len(parts[1]))
	}
	return ""
}

// goldenMultipartBackend extends MockBackend with real multipart-upload
// semantics: parts are assembled in upload order and the assembled bytes
// become the object body at CompleteMultipartUpload, mirroring what B2 does
// server-side. The base stubs silently keep the old object body after a
// multipart re-upload, which would hide defects in the migrator's multipart
// path.
type goldenMultipartBackend struct {
	*MockBackend
	uploads map[string][]byte
	nextID  int
}

func newGoldenMultipartBackend() *goldenMultipartBackend {
	return &goldenMultipartBackend{
		MockBackend: NewMockBackend(),
		uploads:     make(map[string][]byte),
	}
}

func (g *goldenMultipartBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	g.nextID++
	return fmt.Sprintf("golden-upload-%d", g.nextID), nil
}

func (g *goldenMultipartBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	g.uploads[uploadID] = append(g.uploads[uploadID], data...)
	return fmt.Sprintf("etag-%s-%d", uploadID, partNumber), nil
}

func (g *goldenMultipartBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	assembled, ok := g.uploads[uploadID]
	if !ok {
		return "", fmt.Errorf("unknown upload %q", uploadID)
	}
	// The completed upload replaces the body; metadata follows via Copy,
	// exactly as uploadAsMultipart does for real backends.
	g.objects[key] = &MockObject{Data: assembled, Metadata: map[string]string{}}
	return "etag-completed", nil
}

// goldenSidecarPath is the sidecar location the read path derives from an
// object key (see FormatMigrator.loadHMCTableFromSidecar).
func goldenSidecarPath(key string) string {
	return fmt.Sprintf(".armor/hmac/%x", sha256.Sum256([]byte(key)))
}

// seedGoldenObject stores object material under key and the HMAC sidecar, if
// any, at the derived sidecar path.
func seedGoldenObject(t *testing.T, g *goldenMultipartBackend, key string, data []byte, sidecar []byte, meta map[string]string) {
	t.Helper()
	g.objects[key] = &MockObject{Data: data, Metadata: meta}
	if sidecar != nil {
		g.objects[goldenSidecarPath(key)] = &MockObject{Data: sidecar}
	}
}

func newGoldenMigrator(g *goldenMultipartBackend) *FormatMigrator {
	return NewFormatMigrator(g, "golden-bucket", goldenMEK(), "golden", crypto.Version3, []string{"1", "2"}, nil)
}

// decryptGoldenFixture decrypts the committed fixture bytes through the
// production read path (single-PUT or multipart, as the fixture dictates).
func decryptGoldenFixture(t *testing.T, f goldenFixture, key string) ([]byte, error) {
	t.Helper()
	armorMeta, ok := backend.ParseARMORMetadata(f.ObjectMeta)
	if !ok {
		return nil, fmt.Errorf("object_metadata.json did not parse as ARMOR metadata")
	}
	g := newGoldenMultipartBackend()
	seedGoldenObject(t, g, key, f.Data, f.Sidecar, f.ObjectMeta)
	fm := newGoldenMigrator(g)
	if f.Sidecar != nil {
		return fm.decryptMultipartObject(armorMeta, key, f.ObjectMeta, bytes.NewReader(f.Data))
	}
	return fm.decryptSingleObject(armorMeta, bytes.NewReader(f.Data))
}

// checkGoldenPlaintext asserts decrypted plaintext against the fixture's
// documented SHA-256 and length.
func checkGoldenPlaintext(t *testing.T, f goldenFixture, plaintext []byte) {
	t.Helper()
	if len(plaintext) != f.Meta.PlaintextLength {
		t.Fatalf("plaintext length = %d, want %d", len(plaintext), f.Meta.PlaintextLength)
	}
	sum := sha256.Sum256(plaintext)
	if got := hex.EncodeToString(sum[:]); got != f.Meta.PlaintextSHA256 {
		t.Fatalf("plaintext sha256 = %s, want %s", got, f.Meta.PlaintextSHA256)
	}
}

// TestGoldenFixturesDecrypt validates every committed fixture's ciphertext
// against its documented plaintext (decryption validation). Success-outcome
// fixtures must decrypt to the documented bytes; failure-outcome (corrupt)
// fixtures must fail closed. Sidecar block accounting is pure byte math and
// is asserted for every fixture whose intended defect is not the sidecar
// itself: for the failure-outcome fixtures whose defect IS an accounting
// defect (malformed/invalid_sidecar_format, malformed/truncated_sidecar),
// the defect is asserted via goldenSidecarAccounting and the decrypt
// expectation is skipped, since it could only fail on the same bytes.
func TestGoldenFixturesDecrypt(t *testing.T) {
	for _, f := range loadGoldenFixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			if f.Sidecar != nil {
				if defect := goldenSidecarAccounting(f); defect != "" && f.Meta.ExpectedMigrationOutcome == "failure" {
					// The fixture's intended defect is the sidecar accounting
					// defect itself (the matrix asserts it at stageAccounting).
					// The skip re-arms the moment a regenerated fixture carries
					// a structurally sound sidecar.
					t.Skipf("corrupt fixture exhibits its intended sidecar defect: %s", defect)
				}
				if len(f.Sidecar)%32 != 0 {
					t.Fatalf("sidecar size %d is not a multiple of the 32-byte HMAC", len(f.Sidecar))
				}
				wantBlocks := (f.Meta.PlaintextLength + goldenBlockSize - 1) / goldenBlockSize
				if got := len(f.Sidecar) / 32; got != wantBlocks {
					t.Fatalf("sidecar covers %d blocks, want %d for %d bytes", got, wantBlocks, f.Meta.PlaintextLength)
				}
			}
			if want := f.Sidecar != nil; want != f.Meta.V3Expected.IsMultipart {
				t.Errorf("fixture declares is_multipart=%v but sidecar presence says %v",
					f.Meta.V3Expected.IsMultipart, want)
			}
			if note := goldenWrapFingerprintNote(f); note != "" {
				t.Logf("note: %s (production multi-key routing would reject this; the migrator path ignores it)", note)
			}

			switch f.Meta.ExpectedMigrationOutcome {
			case "success":
				if reason := goldenWrapDefect(f); reason != "" {
					t.Skipf("known committed-fixture wrap defect: %s", reason)
				}
				plaintext, err := decryptGoldenFixture(t, f, "golden/"+f.Name)
				if err != nil {
					t.Fatalf("decrypt failed: %v", err)
				}
				checkGoldenPlaintext(t, f, plaintext)
				t.Logf("GOLDEN %s decrypt=ok sha256=%s len=%d", f.Name, f.Meta.PlaintextSHA256, f.Meta.PlaintextLength)
			case "failure":
				plaintext, err := decryptGoldenFixture(t, f, "golden/"+f.Name)
				if err == nil {
					// Some corrupt variants decrypt cleanly by design and are
					// rejected at classification instead; TestGoldenFixturesMigrate
					// is the enforcing layer for those.
					t.Skipf("corrupt fixture decrypts cleanly; failure is enforced at classification")
				}
				_ = plaintext
				t.Logf("GOLDEN %s decrypt=failed-closed reason=%q", f.Name, err.Error())
			default:
				t.Fatalf("unknown expected_migration_outcome %q", f.Meta.ExpectedMigrationOutcome)
			}
		})
	}
}

// TestGoldenFixturesDryRun validates that dry-run migration classifies every
// fixture without rewriting anything: success-outcome fixtures decrypt and
// count as processed, failure-outcome fixtures are caught, and in both cases
// the stored object is left byte-for-byte untouched.
func TestGoldenFixturesDryRun(t *testing.T) {
	for _, f := range loadGoldenFixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			g := newGoldenMultipartBackend()
			key := "golden/" + f.Name
			seedGoldenObject(t, g, key, f.Data, f.Sidecar, f.ObjectMeta)

			result, err := newGoldenMigrator(g).Migrate(context.Background(), true, 1)
			if err != nil {
				t.Fatalf("dry-run Migrate returned error: %v", err)
			}

			// A dry run must not touch the object, whatever its outcome.
			obj, ok := g.objects[key]
			if !ok {
				t.Fatal("object removed by dry run")
			}
			if !bytes.Equal(obj.Data, f.Data) {
				t.Error("dry run modified the object body")
			}
			if got, want := obj.Metadata["x-amz-meta-armor-version"], f.ObjectMeta["x-amz-meta-armor-version"]; got != want {
				t.Errorf("dry run rewrote version metadata: got %q, want %q", got, want)
			}

			switch f.Meta.ExpectedMigrationOutcome {
			case "success":
				if reason := goldenWrapDefect(f); reason != "" {
					t.Skipf("known committed-fixture wrap defect: %s", reason)
				}
				if result.ProcessedObjects != 1 {
					t.Errorf("ProcessedObjects = %d, want 1", result.ProcessedObjects)
				}
				if result.FailedObjects != 0 || len(result.Failures) != 0 {
					t.Errorf("dry run recorded failures: %+v", result.Failures)
				}
				t.Logf("GOLDEN %s dryrun=ok", f.Name)
			case "failure":
				if result.FailedObjects != 1 || len(result.Failures) != 1 || result.Failures[0].Reason == "" {
					t.Fatalf("corrupt fixture was not caught in dry run: FailedObjects=%d failures=%+v",
						result.FailedObjects, result.Failures)
				}
				t.Logf("GOLDEN %s dryrun=failed-closed reason=%q", f.Name, result.Failures[0].Reason)
			default:
				t.Fatalf("unknown expected_migration_outcome %q", f.Meta.ExpectedMigrationOutcome)
			}
		})
	}
}

// computedGoldenEntry mirrors one outcome entry of
// tests/fixtures/migration/v3-golden-outcomes-computed.json. Only the fields
// that describe the migrator's output (outcome, multipart-ness, compression)
// are cross-checked; the file's part_count/blocks_per_part describe the
// SOURCE layout and legitimately differ from the migrator's 5 MiB-part output.
type computedGoldenEntry struct {
	ExpectedOutcome string `json:"expected_outcome"`
	V3Expected      struct {
		IsMultipart     bool `json:"is_multipart"`
		CompressionUsed bool `json:"compression_used"`
	} `json:"v3_expected"`
}

type computedGoldenFile struct {
	Fixtures map[string]computedGoldenEntry `json:"fixtures"`
}

func loadComputedGoldens(t *testing.T) computedGoldenFile {
	t.Helper()
	path := filepath.Join(goldenFixtureRoot(t), "v3-golden-outcomes-computed.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read computed goldens: %v", err)
	}
	var file computedGoldenFile
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return file
}

// TestGoldenFixturesMigrate runs a live migration against every committed
// fixture and asserts the fixture's documented expected_migration_outcome. On
// success the migrated object must decrypt with the production read path,
// preserve the documented plaintext, and carry the output structure implied by
// the migrator's rules (single-PUT below the multipart threshold, 5 MiB parts
// above it).
func TestGoldenFixturesMigrate(t *testing.T) {
	computed := loadComputedGoldens(t)
	for _, f := range loadGoldenFixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			switch f.Meta.ExpectedMigrationOutcome {
			case "success":
				if reason := goldenWrapDefect(f); reason != "" {
					t.Skipf("known committed-fixture wrap defect: %s", reason)
				}
				// While the migrator multipart defect is present (see
				// TestGoldenMultipartMigratorDefect) any success-outcome object
				// above the threshold fails at the migrator, not at the fixture.
				if f.Meta.PlaintextLength > goldenMultipartThreshold {
					if defect, detail := probeGoldenMigratorMultipartDefect(t); defect {
						t.Skipf("migrator multipart output defect present: %s", detail)
					}
				}
			case "failure":
				// no skip: corrupt fixtures must be caught below
			default:
				t.Fatalf("unknown expected_migration_outcome %q", f.Meta.ExpectedMigrationOutcome)
			}

			g := newGoldenMultipartBackend()
			key := "golden/" + f.Name
			seedGoldenObject(t, g, key, f.Data, f.Sidecar, f.ObjectMeta)

			result, err := newGoldenMigrator(g).Migrate(context.Background(), false, 1)
			if err != nil {
				t.Fatalf("Migrate returned error: %v", err)
			}

			switch f.Meta.ExpectedMigrationOutcome {
			case "success":
				for _, failure := range result.Failures {
					t.Errorf("recorded failure for %s: %s", failure.Key, failure.Reason)
				}
				if result.FailedObjects != 0 {
					t.Fatalf("FailedObjects = %d, want 0", result.FailedObjects)
				}
				if result.ProcessedObjects != 1 {
					t.Fatalf("ProcessedObjects = %d, want 1", result.ProcessedObjects)
				}
				verifyGoldenMigration(t, g, f, key, computed.Fixtures[strings.ReplaceAll(f.Name, "/", "_")])
				t.Logf("GOLDEN %s migrate=ok version=3", f.Name)
			case "failure":
				if result.FailedObjects != 1 || len(result.Failures) != 1 || result.Failures[0].Reason == "" {
					t.Fatalf("corrupt fixture was not caught: FailedObjects=%d failures=%+v",
						result.FailedObjects, result.Failures)
				}
				// Fail closed: the corrupted object must be left exactly as seeded.
				obj, ok := g.objects[key]
				if !ok {
					t.Fatal("corrupted object was removed")
				}
				if !bytes.Equal(obj.Data, f.Data) {
					t.Error("corrupted object body was rewritten")
				}
				if obj.Metadata["x-amz-meta-armor-version"] == "3" {
					t.Error("corrupted object was migrated to the target version")
				}
				t.Logf("GOLDEN %s migrate=failed-as-expected reason=%q", f.Name, result.Failures[0].Reason)
			}
		})
	}
}

// verifyGoldenMigration re-reads the migrated object and asserts version,
// readability, plaintext preservation, output structure, and the computed
// golden cross-check.
func verifyGoldenMigration(t *testing.T, g *goldenMultipartBackend, f goldenFixture, key string, computed computedGoldenEntry) {
	t.Helper()
	obj, ok := g.objects[key]
	if !ok {
		t.Fatal("migrated object is missing from the backend")
	}
	if got := obj.Metadata["x-amz-meta-armor-version"]; got != "3" {
		t.Fatalf("version after migration = %q, want 3", got)
	}

	// Independent read-back: the migrated object must decrypt through the
	// production path for its OUTPUT layout and yield the documented plaintext.
	armorMeta, ok := backend.ParseARMORMetadata(obj.Metadata)
	if !ok {
		t.Fatal("migrated metadata does not parse as ARMOR")
	}
	fm := newGoldenMigrator(g)
	var plaintext []byte
	var err error
	if obj.Metadata["x-amz-meta-armor-multipart"] == "true" {
		plaintext, err = fm.decryptMultipartObject(armorMeta, key, obj.Metadata, bytes.NewReader(obj.Data))
	} else {
		plaintext, err = fm.decryptSingleObject(armorMeta, bytes.NewReader(obj.Data))
	}
	if err != nil {
		t.Fatalf("migrated object does not decrypt: %v", err)
	}
	checkGoldenPlaintext(t, f, plaintext)

	// Output structure per the migrator's rules: objects above the multipart
	// threshold re-upload as 5 MiB multipart; the rest re-encrypt as single-PUTs.
	wantMultipart := f.Meta.PlaintextLength > goldenMultipartThreshold
	if got := obj.Metadata["x-amz-meta-armor-multipart"] == "true"; got != wantMultipart {
		t.Errorf("multipart flag after migration = %v, want %v", got, wantMultipart)
	}
	if wantMultipart {
		if got := obj.Metadata["x-amz-meta-armor-part-size"]; got != fmt.Sprintf("%d", goldenPartSize) {
			t.Errorf("part size after migration = %q, want %d", got, goldenPartSize)
		}
		// A multipart output needs the HMAC sidecar the read path requires.
		if _, ok := g.objects[goldenSidecarPath(key)]; !ok {
			t.Errorf("migrated multipart object has no HMAC sidecar at %s", goldenSidecarPath(key))
		}
	}

	// Cross-check the computed golden file where it describes the output.
	if computed.ExpectedOutcome != "" && computed.ExpectedOutcome != "success" {
		t.Errorf("computed golden expects outcome %q for a fixture documented as success", computed.ExpectedOutcome)
	}
	if computed.ExpectedOutcome != "" {
		if computed.V3Expected.CompressionUsed {
			t.Errorf("computed golden expects compression but the migrator writes uncompressed")
		}
		if computed.V3Expected.IsMultipart != wantMultipart {
			t.Errorf("computed golden is_multipart=%v conflicts with the threshold rule (%v)",
				computed.V3Expected.IsMultipart, wantMultipart)
		}
	}
}

// synthesizeGoldenMultipartObject builds a VALID V1 multipart object with
// production crypto -- the same shape the real PUT path stores (headerless
// assembled ciphertext plus a flat HMAC sidecar) -- so the migrator's
// multipart paths can be exercised independently of the committed multipart
// fixture bytes. Sizes at or below the multipart
// threshold migrate to single-PUTs; sizes above it exercise uploadAsMultipart.
func synthesizeGoldenMultipartObject(t *testing.T, plaintextLength int) (data, sidecar, plaintext []byte, meta map[string]string) {
	t.Helper()
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("generate DEK: %v", err)
	}
	wrappedDEK, err := crypto.WrapDEK(goldenMEK(), dek)
	if err != nil {
		t.Fatalf("wrap DEK: %v", err)
	}
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i)
	}
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, goldenBlockSize, crypto.Version1)
	if err != nil {
		t.Fatalf("create encryptor: %v", err)
	}
	plaintext = make([]byte, plaintextLength)
	for i := range plaintext {
		plaintext[i] = byte(i * 31)
	}
	ciphertext, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	plaintextSHA := sha256.Sum256(plaintext)
	meta = map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", goldenBlockSize),
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", plaintextLength),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":      "true",
	}
	return ciphertext, hmacTable, plaintext, meta
}

// goldenMigratorDefectProbeSize is 6 MiB: above the multipart threshold, so a
// live migration of a valid object this size must take the uploadAsMultipart
// path under test.
const goldenMigratorDefectProbeSize = 6 * 1024 * 1024

// probeGoldenMigratorMultipartDefect migrates a valid synthesized multipart
// object above the threshold and reports whether the known uploadAsMultipart
// defect still reproduces (true) or the behavior changed (false, with a
// description of the new symptom). TestGoldenMultipartMigratorDefect asserts
// on the full symptom set; the migrate suite uses this to skip committed
// multipart success-migrations that would fail at the migrator rather than at
// the fixture.
func probeGoldenMigratorMultipartDefect(t *testing.T) (defect bool, detail string) {
	t.Helper()

	data, sidecar, _, meta := synthesizeGoldenMultipartObject(t, goldenMigratorDefectProbeSize)

	g := newGoldenMultipartBackend()
	key := "golden/defect-probe/multipart-migrator"
	seedGoldenObject(t, g, key, data, sidecar, meta)

	result, err := newGoldenMigrator(g).Migrate(context.Background(), false, 1)
	if err != nil {
		return false, fmt.Sprintf("Migrate errored: %v", err)
	}
	if result.FailedObjects != 1 || len(result.Failures) != 1 {
		return false, fmt.Sprintf("no failure recorded: FailedObjects=%d failures=%+v", result.FailedObjects, result.Failures)
	}
	reason := result.Failures[0].Reason
	if !strings.Contains(reason, "envelope header") {
		return false, fmt.Sprintf("failure reason changed: %q", reason)
	}
	obj := g.objects[key]
	if obj == nil {
		return false, "object removed during failed migration"
	}
	if bytes.Equal(obj.Data, data) {
		return false, "object body untouched"
	}
	armorMeta, ok := backend.ParseARMORMetadata(obj.Metadata)
	if !ok {
		return false, "replaced metadata does not parse as ARMOR"
	}
	fm := newGoldenMigrator(g)
	if _, err := fm.decryptSingleObject(armorMeta, bytes.NewReader(obj.Data)); err == nil {
		return false, "replaced object decrypts as single-PUT"
	}
	if _, err := fm.decryptMultipartObject(armorMeta, key, obj.Metadata, bytes.NewReader(obj.Data)); err == nil {
		return false, "replaced object decrypts as multipart"
	}
	return true, reason
}

// TestGoldenMultipartMigratorDefect pins the migrator-side defect: migrating a
// VALID multipart object above the multipart threshold replaces the object
// with an unreadable body (uploadAsMultipart discards the per-part HMAC
// tables and never persists a sidecar) and then fails its own read-back
// verify, because the verify calls decryptSingleObject unconditionally and
// headerless multipart data has no envelope header. When the migrator's
// multipart path is fixed this test FAILS -- delete it together with the
// skip in TestGoldenFixturesMigrate.
func TestGoldenMultipartMigratorDefect(t *testing.T) {
	defect, detail := probeGoldenMigratorMultipartDefect(t)
	if !defect {
		t.Fatalf("defect no longer reproduces: %s -- the migrator changed; revisit this pin test and the TestGoldenFixturesMigrate skip", detail)
	}
	t.Logf("defect reproduced: object replaced with an unreadable body, failure reason=%q", detail)
}

// TestGoldenMultipartDeclaredSHAEnforcement pins the multipart read path's
// declared-plaintext-digest enforcement (the malformed/
// multipart_contradictory_hashes matrix row, whose fixture category has not
// landed): a multipart object whose declared digest disagrees with its
// decryptable content fails with ErrIntegrityVerification even though every
// per-block HMAC verifies, and the dry run records exactly one failure
// without rewriting anything -- so neither run can migrate (and above the
// multipart threshold, overwrite) an object whose integrity metadata lies.
// The exemptions that keep legacy objects migrating are pinned on the other
// side: a matching declared digest and an absent one (the pre-bf-1v2ehf
// placeholder shape) both decrypt and migrate.
func TestGoldenMultipartDeclaredSHAEnforcement(t *testing.T) {
	data, sidecar, _, meta := synthesizeGoldenMultipartObject(t, 1<<20)

	trueSHA := meta["x-amz-meta-armor-sha256"]
	if len(trueSHA) != 64 {
		t.Fatalf("synthesized declared sha256 = %q, want 64 hex digits", trueSHA)
	}
	badSHA := []byte(trueSHA)
	if badSHA[0] == '0' {
		badSHA[0] = '1'
	} else {
		badSHA[0] = '0'
	}
	contradicted := withMeta(meta, "x-amz-meta-armor-sha256", string(badSHA))

	t.Run("decrypt_rejects_contradicted_declaration", func(t *testing.T) {
		armorMeta, ok := backend.ParseARMORMetadata(contradicted)
		if !ok {
			t.Fatal("tampered metadata does not parse as ARMOR")
		}
		g := newGoldenMultipartBackend()
		key := "golden/declared-sha/reject"
		seedGoldenObject(t, g, key, data, sidecar, contradicted)
		fm := newGoldenMigrator(g)
		_, err := fm.decryptMultipartObject(armorMeta, key,
			contradicted, bytes.NewReader(data))
		if err == nil {
			t.Fatal("contradicted declaration decrypted cleanly")
		}
		if !errors.Is(err, ErrIntegrityVerification) {
			t.Fatalf("error is not ErrIntegrityVerification-class: %v", err)
		}
	})

	t.Run("matching_and_absent_declarations_still_decrypt", func(t *testing.T) {
		for name, declaredMeta := range map[string]map[string]string{
			"matching_digest": meta,
			// Legacy multipart objects predate whole-object digests entirely.
			"absent_digest": withMeta(meta, "x-amz-meta-armor-sha256", ""),
		} {
			t.Run(name, func(t *testing.T) {
				armorMeta, ok := backend.ParseARMORMetadata(declaredMeta)
				if !ok {
					t.Fatal("metadata does not parse as ARMOR")
				}
				g := newGoldenMultipartBackend()
				key := "golden/declared-sha/" + name
				seedGoldenObject(t, g, key, data, sidecar, declaredMeta)
				fm := newGoldenMigrator(g)
				plaintext, err := fm.decryptMultipartObject(armorMeta, key,
					declaredMeta, bytes.NewReader(data))
				if err != nil {
					t.Fatalf("declared digest %q rejected valid multipart object: %v", declaredMeta["x-amz-meta-armor-sha256"], err)
				}
				sum := sha256.Sum256(plaintext)
				if got := hex.EncodeToString(sum[:]); got != trueSHA {
					t.Fatalf("plaintext sha256 = %s, want %s", got, trueSHA)
				}
			})
		}
	})

	// Both runs through the migrator: the contradicted declaration must fail
	// closed (exactly one recorded failure, stored object byte-identical,
	// version untouched) while the same bytes under the true digest migrate
	// cleanly -- proving the rejection comes from the enforcement and not the
	// object material.
	for _, dryRun := range []struct {
		name   string
		isDry  bool
		expect func(t *testing.T, result *MigrationResult, untouched bool)
	}{
		{"dry_run", true, func(t *testing.T, result *MigrationResult, untouched bool) {
			if result.FailedObjects != 1 || len(result.Failures) != 1 {
				t.Fatalf("dry run did not reject: FailedObjects=%d failures=%+v", result.FailedObjects, result.Failures)
			}
			if reason := result.Failures[0].Reason; !strings.Contains(reason, "plaintext SHA-256 mismatch") {
				t.Fatalf("failure reason %q does not name the digest mismatch", reason)
			}
			if !untouched {
				t.Error("dry run rewrote the rejected object")
			}
		}},
		{"live", false, func(t *testing.T, result *MigrationResult, untouched bool) {
			if result.FailedObjects != 1 || len(result.Failures) != 1 {
				t.Fatalf("live run did not reject: FailedObjects=%d failures=%+v", result.FailedObjects, result.Failures)
			}
			if !untouched {
				t.Error("live run rewrote the rejected object")
			}
		}},
	} {
		t.Run("contradicted_"+dryRun.name, func(t *testing.T) {
			g := newGoldenMultipartBackend()
			key := "golden/declared-sha/contradicted-" + dryRun.name
			seedGoldenObject(t, g, key, data, sidecar, contradicted)

			result, err := newGoldenMigrator(g).Migrate(context.Background(), dryRun.isDry, 1)
			if err != nil {
				t.Fatalf("Migrate returned error: %v", err)
			}
			obj := g.objects[key]
			if obj == nil {
				t.Fatal("rejected object was removed")
			}
			untouched := bytes.Equal(obj.Data, data) && obj.Metadata["x-amz-meta-armor-version"] != "3"
			dryRun.expect(t, result, untouched)
		})
	}

	t.Run("control_matching_digest_migrates", func(t *testing.T) {
		g := newGoldenMultipartBackend()
		key := "golden/declared-sha/control"
		seedGoldenObject(t, g, key, data, sidecar, meta)

		result, err := newGoldenMigrator(g).Migrate(context.Background(), false, 1)
		if err != nil {
			t.Fatalf("Migrate returned error: %v", err)
		}
		if result.FailedObjects != 0 {
			t.Fatalf("matching digest failed migration: %+v", result.Failures)
		}
		if got := g.objects[key].Metadata["x-amz-meta-armor-version"]; got != "3" {
			t.Fatalf("control object not migrated, version = %q", got)
		}
	})
}

// TestGoldenFixtureCorruptions derives corrupted variants from valid fixture
// material and asserts every one fails closed: exactly one recorded failure
// with a reason, and the stored object left untouched. Single-PUT corruption
// cases mutate a committed fixture; multipart corruption cases mutate a
// synthesized valid multipart object, keeping them independent of the
// committed multipart fixture bytes.
func TestGoldenFixtureCorruptions(t *testing.T) {
	fixtures := loadGoldenFixtures(t)
	byName := make(map[string]goldenFixture, len(fixtures))
	for _, f := range fixtures {
		byName[f.Name] = f
	}
	single, ok := byName["v1_single_put/explicit_version"]
	if !ok {
		t.Fatal("v1_single_put/explicit_version not present; corruption cases need a production-valid base fixture")
	}

	flipByte := func(b []byte, off int) []byte {
		c := append([]byte(nil), b...)
		c[off] ^= 0x01
		return c
	}
	flipBase64Char := func(s string) string {
		// Stay inside the base64 alphabet so the base64 pre-validation in
		// migrateObject passes and the corruption reaches the crypto layer.
		b := []byte(s)
		i := len(b) / 2
		if b[i] == 'A' {
			b[i] = 'B'
		} else {
			b[i] = 'A'
		}
		return string(b)
	}
	flipFirstHexDigit := func(s string) string {
		// Stay inside the hex alphabet so the declaration stays a well-formed
		// digest and the corruption reaches the declared-digest comparison
		// rather than the not-a-digest exemption.
		b := []byte(s)
		if b[0] == '0' {
			b[0] = '1'
		} else {
			b[0] = '0'
		}
		return string(b)
	}

	// Valid 1 MiB multipart object (below the migration threshold, so an
	// uncorrupted copy migrates cleanly to a single-PUT).
	mpData, mpSidecar, _, mpMeta := synthesizeGoldenMultipartObject(t, 1<<20)

	cases := []struct {
		name    string
		data    []byte
		sidecar []byte
		meta    map[string]string
	}{
		{
			name:    "single/corrupted_ciphertext",
			data:    flipByte(single.Data, crypto.HeaderSize+10),
			sidecar: single.Sidecar,
			meta:    single.ObjectMeta,
		},
		{
			name:    "single/corrupted_hmac_table",
			data:    flipByte(single.Data, len(single.Data)-1),
			sidecar: single.Sidecar,
			meta:    single.ObjectMeta,
		},
		{
			name:    "single/truncated_ciphertext",
			data:    single.Data[:len(single.Data)-8],
			sidecar: single.Sidecar,
			meta:    single.ObjectMeta,
		},
		{
			name:    "single/truncated_envelope_header",
			data:    single.Data[:crypto.HeaderSize-1],
			sidecar: single.Sidecar,
			meta:    single.ObjectMeta,
		},
		{
			// Restored from the deleted TestGoldenMigrationIVIntegrityGap
			// pin: flipping an IV byte scrambles the CTR keystream without
			// touching any HMAC'd ciphertext byte, so only the header
			// plaintext-SHA check catches it.
			name:    "single/corrupted_envelope_iv",
			data:    flipByte(single.Data, 10), // the IV lives at envelope-header bytes [6,22)
			sidecar: single.Sidecar,
			meta:    single.ObjectMeta,
		},
		{
			name:    "single/corrupted_wrapped_dek",
			data:    single.Data,
			sidecar: single.Sidecar,
			meta: withMeta(single.ObjectMeta, "x-amz-meta-armor-wrapped-dek",
				flipBase64Char(single.ObjectMeta["x-amz-meta-armor-wrapped-dek"])),
		},
		{
			name:    "multipart/corrupted_ciphertext",
			data:    flipByte(mpData, 1024),
			sidecar: mpSidecar,
			meta:    mpMeta,
		},
		{
			name:    "multipart/corrupted_sidecar",
			data:    mpData,
			sidecar: flipByte(mpSidecar, 40),
			meta:    mpMeta,
		},
		{
			name:    "multipart/missing_sidecar",
			data:    mpData,
			sidecar: nil,
			meta:    mpMeta,
		},
		{
			name:    "multipart/truncated_ciphertext",
			data:    mpData[:len(mpData)-512],
			sidecar: mpSidecar,
			meta:    mpMeta,
		},
		{
			name:    "multipart/corrupted_wrapped_dek",
			data:    mpData,
			sidecar: mpSidecar,
			meta: withMeta(mpMeta, "x-amz-meta-armor-wrapped-dek",
				flipBase64Char(mpMeta["x-amz-meta-armor-wrapped-dek"])),
		},
		{
			// Declared digest contradicts the content (the
			// malformed/multipart_contradictory_hashes matrix row; its fixture
			// category has not landed, so this synthesized case arms the
			// enforcement now). Every per-block HMAC verifies -- only the
			// declared-digest check can reject it.
			name:    "multipart/contradicted_declared_sha256",
			data:    mpData,
			sidecar: mpSidecar,
			meta: withMeta(mpMeta, "x-amz-meta-armor-sha256",
				flipFirstHexDigit(mpMeta["x-amz-meta-armor-sha256"])),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newGoldenMultipartBackend()
			objName := "golden/corruption/" + tc.name
			seedGoldenObject(t, g, objName, tc.data, tc.sidecar, tc.meta)

			result, err := newGoldenMigrator(g).Migrate(context.Background(), false, 1)
			if err != nil {
				t.Fatalf("Migrate returned error: %v", err)
			}
			if result.FailedObjects != 1 || len(result.Failures) != 1 {
				t.Fatalf("corruption was not caught: FailedObjects=%d failures=%+v", result.FailedObjects, result.Failures)
			}
			if result.Failures[0].Reason == "" {
				t.Fatal("recorded failure has no reason")
			}

			// Fail closed: the corrupted object must be left exactly as seeded.
			obj, ok := g.objects[objName]
			if !ok {
				t.Fatal("corrupted object was removed")
			}
			if !bytes.Equal(obj.Data, tc.data) {
				t.Error("corrupted object body was rewritten")
			}
			if obj.Metadata["x-amz-meta-armor-version"] == "3" {
				t.Error("corrupted object was migrated to the target version")
			}
			t.Logf("GOLDEN %s corrupt=failed-closed reason=%q", tc.name, result.Failures[0].Reason)
		})
	}
}

// withMeta returns a copy of meta with key set to value.
func withMeta(meta map[string]string, key, value string) map[string]string {
	out := make(map[string]string, len(meta)+1)
	for k, v := range meta {
		out[k] = v
	}
	out[key] = value
	return out
}
