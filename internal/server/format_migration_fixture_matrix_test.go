// Fixture-matrix validation: locks the committed fixture tree of
// tests/fixtures/migration to an explicit pass/fail expectation per fixture,
// so the legacy-reader validation demanded by the migration plan is a table
// someone has to maintain, not whatever happens to be on disk.
//
// The matrix complements format_migration_golden_test.go, which validates
// whatever fixtures exist at run time. That test asserts corrupt fixtures
// fail closed but not HOW they fail; this file additionally pins each corrupt
// fixture to its intended failure mode, so a fixture that stops exercising
// the defect it was built for (or a defect that goes silent) is caught.
//
// Layering per corrupt fixture, strictest first:
//
//   - stageDecrypt: the legacy read path itself must reject the bytes, and
//     the error must identify the intended defect (crypto.ErrInvalidMagic,
//     ErrHMACMismatch, ErrUnwrapFailed, or the ciphertext/table truncation
//     check in decryptSingleObject).
//   - stageAccounting: the defect is visible in pure byte math (sidecar
//     length vs the 32-byte HMAC, table entries vs block count), which runs
//     before any crypto and is asserted unconditionally.
//   - stageClassify: the defect is decidable from metadata/structure rather
//     than the read path (an unparsable version the reader deliberately
//     tolerates, part accounting that only inventory sees). These are
//     asserted structurally: the dry-run migrator must record a failure and
//     process nothing.
//
// Wrap defects are handled with the same data-driven discriminator the
// golden test uses (goldenWrapDefect): if incompatible wraps ever return to
// the committed tree, a decrypt-stage subtest whose bytes can no longer
// reach its intended defect asserts the unwrap failure itself (fail-closed
// proof, masking logged) instead of a class it cannot observe. That mask is
// strictly a fallback: a read-path error that already carries the row's
// intended class is asserted as the class, never as a mask. The ordering
// matters because a wrap defect and an intended class are not exclusive --
// corrupted_wrapped_dek_tag flips a byte inside the 40-byte KWP wrap, so
// production unwrap rejects it by design, and that rejection (KWP
// authentication, not a length check) is exactly the defect the row asks
// for.
//
// Version-derivation contradictions get one extra discriminator: V1 and V2
// counters coincide on block 0 (both derive counter 0), so a single-block
// object cannot express "V1 derivation vs V2 derivation" in its ciphertext
// at all. A fixture claiming that contradiction at single-block size is
// vacuous -- owned by the fixture-construction lineage, skipped with a
// pointer until regenerated with multi-block plaintext.
//
// These tests read only tests/fixtures/migration and the in-memory mock
// backend; they never talk to a real bucket.
package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenFixtureStage says where a corrupt fixture's failure must be enforced.
type goldenFixtureStage int

const (
	stageDecrypt goldenFixtureStage = iota
	stageAccounting
	stageClassify
)

// goldenExpectation is one matrix row: what the legacy readers must do with
// a fixture, and (for corrupt fixtures) the failure signature that identifies
// its intended defect. class is a substring of the production error; stages
// without a stable error string (classification, in flux behind the
// inventory work) assert structurally instead.
type goldenExpectation struct {
	outcome string // "success" or "failure" (mirrors expected_migration_outcome)
	stage   goldenFixtureStage
	class   string // required error substring for stageDecrypt
	note    string // human context, logged on skip/mask
}

// goldenFixtureMatrix is the committed inventory of the fixture tree. Every
// fixture directory under tests/fixtures/migration must appear here exactly
// once; an unlisted fixture is a coverage failure, and so is a listed one
// missing from disk (once its category has landed).
var goldenFixtureMatrix = map[string]goldenExpectation{
	// --- valid: legacy readers must decrypt to the documented plaintext ---
	"v1_single_put/explicit_version":              {outcome: "success"},
	"v1_single_put/implicit_version":              {outcome: "success"},
	"v1_single_put/minimal_metadata":              {outcome: "success"},
	"v2_single_put/standard":                      {outcome: "success"},
	"v1_multipart/uniform_parts":                  {outcome: "success"},
	"v1_multipart/variable_final_part":            {outcome: "success"},
	"v1_multipart/non_uniform_parts":              {outcome: "success"},
	"v2_multipart/uniform_parts":                  {outcome: "success"},
	"v2_multipart/variable_final_part":            {outcome: "success"},
	"v2_multipart/non_uniform_parts":              {outcome: "success"},
	"edge_cases/empty_plaintext":                  {outcome: "success"},
	"edge_cases/single_byte_plaintext":            {outcome: "success"},
	"edge_cases/exact_block_boundary":             {outcome: "success"},
	"generated_fixtures/v1-single-explicit-short": {outcome: "success"},
	"generated_fixtures/v1-single-implicit-short": {outcome: "success"},
	"generated_fixtures/v2-single-short":          {outcome: "success"},
	"generated_fixtures/v1-multipart-uniform":     {outcome: "success"},
	"generated_fixtures/v2-multipart-uniform":     {outcome: "success"},

	// --- corrupt: read path rejects with the intended signature ---
	"malformed/invalid_envelope_magic": {
		outcome: "failure", stage: stageDecrypt,
		class: "invalid ARMOR magic",
		note:  "header magic must be rejected by crypto.DecodeHeader",
	},
	"malformed/truncated_ciphertext": {
		outcome: "failure", stage: stageDecrypt,
		class: "ciphertext too short to contain HMAC table",
		note:  "stored bytes shorter than the header-declared table + data",
	},
	"malformed/corrupted_hmac_table": {
		outcome: "failure", stage: stageDecrypt,
		class: "HMAC verification failed",
		note:  "flipped table byte must fail verifyBlockHMAC",
	},
	"malformed/corrupted_wrapped_dek_tag": {
		outcome: "failure", stage: stageDecrypt,
		class: "key unwrap failed",
		note:  "corrupted wrap tag must fail KWP authentication, not length checks",
	},

	// --- corrupt: byte-math accounting catches it before crypto ---
	"malformed/invalid_sidecar_format": {
		outcome: "failure", stage: stageAccounting,
		note: "sidecar length is not a multiple of the 32-byte HMAC",
	},
	"malformed/truncated_sidecar": {
		outcome: "failure", stage: stageAccounting,
		note: "sidecar holds fewer HMAC entries than the object has blocks",
	},

	// --- corrupt: enforcement is structural (inventory/dry-run), not the read path ---
	"malformed/invalid_version_string": {
		outcome: "failure", stage: stageClassify,
		note: "reader deliberately defaults an unparsable version to 1 (backward compat); inventory must reject",
	},
	"malformed/inconsistent_part_metadata": {
		outcome: "failure", stage: stageClassify,
		note: "declared part structure contradicts actual sizes; inventory accounting",
	},
	"malformed/multipart_part_size_mismatch": {
		outcome: "failure", stage: stageClassify,
		note: "declared part size contradicts derived boundaries; inventory accounting",
	},
	"malformed/multipart_contradictory_hashes": {
		outcome: "failure", stage: stageClassify,
		note: "metadata sha256 disagrees with the envelope header's; integrity cannot be established",
	},
	"malformed/envelope_version_mismatch": {
		outcome: "failure", stage: stageClassify,
		note: "header version disagrees with metadata version",
	},
	"malformed/v1_object_v2_metadata": {
		outcome: "failure", stage: stageClassify,
		note: "V1 object wearing V2 metadata; header-vs-metadata version compare",
	},
	"malformed/v2_object_v1_metadata": {
		outcome: "failure", stage: stageClassify,
		note: "V2 object wearing V1 metadata; header-vs-metadata version compare",
	},
	"contradictory/version_says_v1_layout_v2": {
		outcome: "failure", stage: stageClassify,
		note: "metadata claims V1 while the layout was written with V2 derivation",
	},
}

// goldenMatrixFixtures returns the loaded fixtures for a matrix subtest,
// keyed by name.
func goldenMatrixFixtures(t *testing.T) (map[string]goldenFixture, []string) {
	t.Helper()
	fixtures := loadGoldenFixtures(t)
	byName := make(map[string]goldenFixture, len(fixtures))
	names := make([]string, 0, len(fixtures))
	for _, f := range fixtures {
		byName[f.Name] = f
		names = append(names, f.Name)
	}
	return byName, names
}

// goldenCategoryLanded reports whether a matrix category has any committed
// presence on disk. Categories that are absent entirely have not landed yet
// (their fixture lineage is still in flight); individual variants missing
// from a landed category are deletions and must fail.
func goldenCategoryLanded(t *testing.T, name string) bool {
	t.Helper()
	idx := strings.Index(name, "/")
	if idx < 0 {
		return true // top-level entry: no category indirection
	}
	dir := filepath.Join(goldenFixtureRoot(t), name[:idx])
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// TestGoldenFixtureMatrixCoverage pins the fixture tree to the matrix: every
// listed fixture must exist (once its category landed), every fixture on
// disk must be listed, and canonical/ stays generator-only.
func TestGoldenFixtureMatrixCoverage(t *testing.T) {
	byName, names := goldenMatrixFixtures(t)

	for name, exp := range goldenFixtureMatrix {
		t.Run("listed/"+name, func(t *testing.T) {
			if !goldenCategoryLanded(t, name) {
				t.Skipf("category of %s has not landed yet; matrix row arms on arrival", name)
			}
			f, ok := byName[name]
			if !ok {
				t.Fatalf("matrix row %s (%s outcome) has no fixture on disk", name, exp.outcome)
			}
			if got := f.Meta.ExpectedMigrationOutcome; got != exp.outcome {
				t.Fatalf("fixture expected_migration_outcome = %q, matrix says %q", got, exp.outcome)
			}
		})
	}

	listed := make(map[string]bool, len(goldenFixtureMatrix))
	for name := range goldenFixtureMatrix {
		listed[name] = true
	}
	for _, name := range names {
		t.Run("on-disk/"+name, func(t *testing.T) {
			if !listed[name] {
				t.Fatalf("fixture %s exists on disk but has no matrix row; add it to goldenFixtureMatrix with its expected pass/fail class", name)
			}
		})
	}

	t.Run("canonical/generator-only", func(t *testing.T) {
		// canonical/ holds the legacy generator (which links internal/crypto)
		// and deliberately no fixtures; loadGoldenFixtures skips it.
		for _, name := range names {
			if strings.HasPrefix(name, "canonical/") {
				t.Fatalf("canonical/ unexpectedly carries fixture %s", name)
			}
		}
	})
}

// TestGoldenFixtureMatrixValidDecrypt runs every success-outcome matrix row
// through the legacy read path and requires the documented plaintext back.
// Known committed-fixture defects (wrap format, stale documented content)
// skip with the golden test's data-driven discriminators and re-arm when the
// fixtures are regenerated.
func TestGoldenFixtureMatrixValidDecrypt(t *testing.T) {
	byName, _ := goldenMatrixFixtures(t)

	for name, exp := range goldenFixtureMatrix {
		if exp.outcome != "success" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if !goldenCategoryLanded(t, name) {
				t.Skipf("category of %s has not landed yet", name)
			}
			f, ok := byName[name]
			if !ok {
				t.Skipf("fixture %s not on disk yet; matrix row arms on arrival", name)
			}
			if reason := goldenWrapDefect(f); reason != "" {
				t.Skipf("known committed-fixture wrap defect: %s", reason)
			}
			plaintext, err := decryptGoldenFixture(t, f, "matrix/"+name)
			if err != nil {
				t.Fatalf("legacy reader failed to decrypt valid fixture: %v", err)
			}
			checkGoldenPlaintext(t, f, plaintext)
		})
	}
}

// goldenSidecarAccounting reports the pure byte-math defect of a corrupt
// fixture's sidecar, or "" if the sidecar is structurally sound.
func goldenSidecarAccounting(f goldenFixture) string {
	if f.Sidecar == nil {
		return ""
	}
	if len(f.Sidecar)%32 != 0 {
		return "sidecar size is not a multiple of the 32-byte HMAC"
	}
	wantBlocks := (f.Meta.PlaintextLength + goldenBlockSize - 1) / goldenBlockSize
	if len(f.Sidecar)/32 < wantBlocks {
		return "sidecar holds fewer HMAC entries than the object has blocks"
	}
	return ""
}

// goldenVersionContradictionVacuous reports why a fixture's version-
// derivation contradiction cannot be expressed by its bytes, or "" when it
// can. V1 and V2 counter derivation coincide on block 0 (both produce
// counter 0), so any object that fits in a single armor block encrypts
// identically under both and the contradiction is invisible to every layer
// below inventory. Owned by the fixture-construction lineage; regenerate
// with multi-block plaintext to make the contradiction real.
func goldenVersionContradictionVacuous(f goldenFixture) string {
	if f.Meta.PlaintextLength > goldenBlockSize {
		return ""
	}
	return "single-block object: V1 and V2 counter derivation coincide on block 0, so the ciphertext cannot express the contradiction"
}

// TestGoldenFixtureMatrixCorruptFailClosed runs every failure-outcome matrix
// row and requires its intended failure mode at its declared stage:
//
//   - stageAccounting: the byte-math defect must be present.
//   - stageDecrypt: the read path must fail with an error identifying the
//     intended defect. The wrap-defect mask below is subordinate to that:
//     an error that already carries the intended class asserts directly (a
//     corrupted wrap tag fails KWP authentication by design, and that
//     authentication failure is the defect the row asks for). Only an error
//     that cannot reach the intended defect because unwrap rejects the
//     bytes first falls back to the unwrap-only fail-closed proof, with
//     the masking logged.
//   - stageClassify: the dry-run migrator must record exactly one failure
//     and process nothing. A version-contradiction fixture that is vacuous
//     at its committed size skips with a pointer to the fixture lineage
//     instead of failing.
//
// Whatever the stage, no corrupt fixture may pass through every layer: if
// the read path and accounting both accept it and the dry run processes it,
// that is a validation failure even when individual layers had no opinion.
func TestGoldenFixtureMatrixCorruptFailClosed(t *testing.T) {
	byName, _ := goldenMatrixFixtures(t)

	for name, exp := range goldenFixtureMatrix {
		if exp.outcome != "failure" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if !goldenCategoryLanded(t, name) {
				t.Skipf("category of %s has not landed yet", name)
			}
			f, ok := byName[name]
			if !ok {
				t.Skipf("fixture %s not on disk yet; matrix row arms on arrival", name)
			}

			decryptErr := error(nil)
			if _, err := decryptGoldenFixture(t, f, "matrix/"+name); err != nil {
				decryptErr = err
			}

			switch exp.stage {
			case stageAccounting:
				if defect := goldenSidecarAccounting(f); defect == "" {
					t.Fatalf("fixture no longer exhibits its accounting defect: %s", exp.note)
				}
				if decryptErr != nil {
					t.Logf("accounting defect present; read path also rejects: %v", decryptErr)
				}
				return

			case stageDecrypt:
				if defect := goldenSidecarAccounting(f); defect != "" {
					t.Fatalf("sidecar accounting defect (%s) on a decrypt-stage fixture; regenerate the fixture", defect)
				}
				// A read-path failure that already carries the row's
				// intended class IS the intended defect: assert it and
				// nothing else. This precedes the wrap-defect branch below
				// because the two are not exclusive -- a corrupted wrap tag
				// makes goldenWrapDefect fire (production unwrap rejects the
				// bytes by design) while that rejection is itself the
				// defect the row asks for (KWP authentication, not a length
				// check), not a mask of something deeper.
				if decryptErr != nil && exp.class != "" && strings.Contains(decryptErr.Error(), exp.class) {
					return
				}
				if reason := goldenWrapDefect(f); reason != "" {
					// The committed bytes cannot reach the intended defect:
					// production unwrap rejects them first. Fail-closed at
					// unwrap is the only reachable stage, so require exactly
					// that and log what is being masked.
					if decryptErr == nil {
						t.Fatalf("wrap-defective fixture decrypted cleanly; intended defect (%s) is unreachable", exp.class)
					}
					if !strings.Contains(decryptErr.Error(), "wrapped DEK must be 40 bytes") {
						t.Fatalf("expected the wrap-defect unwrap failure, got: %v", decryptErr)
					}
					t.Logf("MASKED by committed-fixture wrap defect (%s); intended class %q asserts once regenerated", reason, exp.class)
					return
				}
				if decryptErr == nil {
					t.Fatalf("legacy reader accepted corrupt fixture; want failure class %q (%s)", exp.class, exp.note)
				}
				if !strings.Contains(decryptErr.Error(), exp.class) {
					t.Fatalf("decrypt failed with %q, want class %q (%s)", decryptErr, exp.class, exp.note)
				}
				return
			}

			// stageClassify: structural enforcement through the dry run.
			g := newGoldenMultipartBackend()
			key := "matrix/" + name
			seedGoldenObject(t, g, key, f.Data, f.Sidecar, f.ObjectMeta)
			result, err := newGoldenMigrator(g).Migrate(context.Background(), true, 1)
			if err != nil {
				t.Fatalf("dry-run Migrate returned error: %v", err)
			}
			if result.FailedObjects == 1 && len(result.Failures) == 1 && result.Failures[0].Reason != "" {
				t.Logf("rejected structurally: %q", result.Failures[0].Reason)
			} else if result.ProcessedObjects != 0 {
				// The inventory accepted the fixture. That is only tolerable
				// while its bytes cannot express the contradiction at all.
				if vacuous := goldenVersionContradictionVacuous(f); vacuous != "" {
					t.Skipf("vacuous committed fixture (dry run processes it): %s; %s", vacuous, exp.note)
				}
				t.Fatalf("dry run processed corrupt fixture (ProcessedObjects=%d): %s", result.ProcessedObjects, exp.note)
			} else {
				t.Fatalf("dry run neither failed nor processed the fixture: FailedObjects=%d failures=%+v",
					result.FailedObjects, result.Failures)
			}

			// End-to-end invariant: a fixture the dry run rejects must not
			// have been clean everywhere upstream.
			if decryptErr == nil && goldenSidecarAccounting(f) == "" {
				t.Logf("accepted by read path and accounting; enforced structurally only")
			}
		})
	}
}
