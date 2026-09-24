// Package server tests for the declared-part-structure accounting the
// migration inventory applies to multipart candidates: a declared
// part-count/part-size that contradicts the structure the rest of the
// metadata derives must be reported Contradictory by the classifier and
// must fail the walk before migrateObject runs — one recorded failure, zero
// processed objects, and the stored object byte-for-byte untouched in the
// dry run and the live run alike.
//
// The corrupt objects here are synthesized from a valid multipart object
// whose part metadata is then overwritten, exactly how the committed
// malformed/inconsistent_part_metadata and malformed/multipart_part_size_
// mismatch fixtures are built (standalone_generator.go), so these tests do
// not depend on the fixture tree having landed: the matrix rows arm when it
// does, and this file is the enforcement proof meanwhile.
//
// The live-run body assertions are the data-destruction regression: above
// the multipart threshold the migrator's uploadAsMultipart path replaces
// the stored body before failing its own read-back verify (the defect
// pinned by TestGoldenMultipartMigratorDefect), so an unchecked corrupt
// fixture destroys the stored object. Gating at part accounting keeps
// migrateObject unreachable for it.
package server

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// Contradiction-rule slugs for the declared-part-structure rules, mirrored
// here so a slug rename breaks a test rather than silently unarming the
// enforcement.
const (
	contradictionPartSizeNotBlockAligned = "multipart-part-size-not-block-aligned"
	contradictionPartCountContradicts    = "part-count-contradicts-part-structure"
	partAccountingReasonPrefix           = "declared part structure contradicts the derived object structure"
	inconsistentPartMetadataPlaintextLen = 119 // one 119-byte part on disk
	partSizeMismatchDeclaredPartSize     = "307200"
	partSizeMismatchActualPartSize       = 512 * 1024 // the boundaries the bytes follow
	inconsistentPartMetadataCount        = "999"
	inconsistentPartMetadataDeclaredSize = "1"
)

// corruptPartMetadata returns a copy of a valid multipart object's metadata
// with the declared part structure overwritten the way the corrupt fixtures
// are.
func corruptPartMetadata(meta map[string]string, overrides map[string]string) map[string]string {
	out := withMeta(meta, "x-amz-meta-armor-part-size", overrides["x-amz-meta-armor-part-size"])
	if count, ok := overrides["x-amz-meta-armor-part-count"]; ok {
		out = withMeta(out, "x-amz-meta-armor-part-count", count)
	} else {
		delete(out, "x-amz-meta-armor-part-count")
	}
	return out
}

// assertPartAccountingFailure runs one Migrate pass and asserts the
// fail-closed shape shared by both corrupt rows: exactly one recorded
// failure carrying the part-accounting reason, and — in a dry run — zero
// processed objects, since the failure must be recorded before
// migrateObject can run at all.
func assertPartAccountingFailure(t *testing.T, g *goldenMultipartBackend, key string, dryRun bool, wantProcessed int) {
	t.Helper()
	result, err := newGoldenMigrator(g).Migrate(context.Background(), dryRun, 1)
	if err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	if result.FailedObjects != 1 || len(result.Failures) != 1 {
		t.Fatalf("FailedObjects=%d failures=%+v, want exactly one recorded failure", result.FailedObjects, result.Failures)
	}
	reason := result.Failures[0].Reason
	if !strings.HasPrefix(reason, partAccountingReasonPrefix) {
		t.Fatalf("failure reason = %q, want prefix %q", reason, partAccountingReasonPrefix)
	}
	if result.ProcessedObjects != wantProcessed {
		t.Fatalf("ProcessedObjects = %d, want %d", result.ProcessedObjects, wantProcessed)
	}
}

// assertObjectUntouched asserts the seeded object survived the walk
// byte-for-byte, at its original version: a corrupt object that is failed
// must not be rewritten, re-uploaded or version-bumped.
func assertObjectUntouched(t *testing.T, g *goldenMultipartBackend, key string, data, sidecar []byte, meta map[string]string) {
	t.Helper()
	obj, ok := g.objects[key]
	if !ok {
		t.Fatal("object was removed")
	}
	if !bytes.Equal(obj.Data, data) {
		t.Error("object body was rewritten")
	}
	if got, want := obj.Metadata["x-amz-meta-armor-version"], meta["x-amz-meta-armor-version"]; got != want {
		t.Errorf("version metadata changed: got %q, want %q", got, want)
	}
	if sidecar != nil {
		stored, ok := g.objects[goldenSidecarPath(key)]
		if !ok {
			t.Fatalf("HMAC sidecar was removed from %s", goldenSidecarPath(key))
		}
		if !bytes.Equal(stored.Data, sidecar) {
			t.Error("HMAC sidecar was rewritten")
		}
	}
}

// TestFormatMigrationRejectsInconsistentPartMetadata pins the
// malformed/inconsistent_part_metadata shape: a small multipart object whose
// metadata declares 999 parts of 1 byte. Below the multipart threshold this
// object decrypts and migrates cleanly today — the declared structure is
// never consulted — so without the accounting gate the dry run processes it
// and a live run rewrites it as a v3 single-PUT.
func TestFormatMigrationRejectsInconsistentPartMetadata(t *testing.T) {
	data, sidecar, _, meta := synthesizeGoldenMultipartObject(t, inconsistentPartMetadataPlaintextLen)
	corrupt := corruptPartMetadata(meta, map[string]string{
		"x-amz-meta-armor-part-count": inconsistentPartMetadataCount,
		"x-amz-meta-armor-part-size":  inconsistentPartMetadataDeclaredSize,
	})

	t.Run("dry_run", func(t *testing.T) {
		g := newGoldenMultipartBackend()
		key := "golden/part-accounting/inconsistent-part-metadata"
		seedGoldenObject(t, g, key, data, sidecar, corrupt)

		assertPartAccountingFailure(t, g, key, true, 0)
		assertObjectUntouched(t, g, key, data, sidecar, corrupt)

		// Both rules the metadata trips are reported: the declared part
		// size is off the block grid and the declared count contradicts the
		// declared sizes.
		fm := newGoldenMigrator(g)
		if err := fm.validatePartAccounting(corrupt); err == nil {
			t.Fatal("validatePartAccounting accepted the corrupt metadata")
		} else {
			for _, slug := range []string{contradictionPartSizeNotBlockAligned, contradictionPartCountContradicts} {
				if !strings.Contains(err.Error(), slug) {
					t.Errorf("reason %q does not report rule %q", err, slug)
				}
			}
		}
	})

	t.Run("live", func(t *testing.T) {
		g := newGoldenMultipartBackend()
		key := "golden/part-accounting/inconsistent-part-metadata"
		seedGoldenObject(t, g, key, data, sidecar, corrupt)

		// The gate sits before migrateObject in the walk, so the live run
		// records the failure without counting the object as processed —
		// the same shape as the envelope-version gate.
		assertPartAccountingFailure(t, g, key, false, 0)
		assertObjectUntouched(t, g, key, data, sidecar, corrupt)
	})
}

// TestFormatMigrationRejectsMultipartPartSizeMismatch pins the
// malformed/multipart_part_size_mismatch shape: a multipart object above the
// multipart threshold whose declared part size (307200) is not a whole
// number of blocks. This is the data-destruction row: today the live walk
// decrypts it, takes the uploadAsMultipart path, replaces the stored body
// and metadata, and only then fails its own read-back verify.
func TestFormatMigrationRejectsMultipartPartSizeMismatch(t *testing.T) {
	data, sidecar, _, meta := synthesizeGoldenMultipartObject(t, goldenMigratorDefectProbeSize)
	corrupt := corruptPartMetadata(meta, map[string]string{
		"x-amz-meta-armor-part-size": partSizeMismatchDeclaredPartSize,
	})
	if got := corrupt["x-amz-meta-armor-plaintext-size"]; got == "" {
		t.Fatal("synthesized metadata carries no plaintext size")
	}

	t.Run("declared_size_off_block_grid", func(t *testing.T) {
		if partSizeMismatchActualPartSize%goldenBlockSize != 0 {
			t.Fatalf("actual part size %d is not block-aligned; fixture premise broken", partSizeMismatchActualPartSize)
		}
		declared, ok := metaSize(corrupt, armorMetaPartSize)
		if !ok || declared <= 0 {
			t.Fatalf("declared part size %q did not parse", corrupt[armorMetaPartSize])
		}
		if declared%int64(goldenBlockSize) == 0 {
			t.Fatalf("declared part size %d is block-aligned; the contradiction would not exist", declared)
		}
	})

	t.Run("dry_run", func(t *testing.T) {
		g := newGoldenMultipartBackend()
		key := "golden/part-accounting/part-size-mismatch"
		seedGoldenObject(t, g, key, data, sidecar, corrupt)

		assertPartAccountingFailure(t, g, key, true, 0)
		assertObjectUntouched(t, g, key, data, sidecar, corrupt)
	})

	t.Run("live", func(t *testing.T) {
		g := newGoldenMultipartBackend()
		key := "golden/part-accounting/part-size-mismatch"
		seedGoldenObject(t, g, key, data, sidecar, corrupt)

		assertPartAccountingFailure(t, g, key, false, 0)
		// The destruction regression: the body and version must survive the
		// failed migration byte-for-byte.
		assertObjectUntouched(t, g, key, data, sidecar, corrupt)
	})
}

// TestClassifyPartAccountingRules pins the classifier side of the shared
// accounting: corrupt declared structures are Contradictory with their rule
// slugs; every valid layout the writers emit — uniform, variable-final,
// single-part, non-uniform, a foreign but consistent part-count — still
// classifies as a migration candidate.
func TestClassifyPartAccountingRules(t *testing.T) {
	base := map[string]string{
		armorMetaVersion:       "1",
		armorMetaWrappedDEK:    "AAAA",
		armorMetaIV:            "AAAAAAAAAAAAAAAAAAAAAA==",
		armorMetaBlockSize:     "65536",
		armorMetaMultipart:     "true",
		armorMetaPlaintextSize: "15728640",
	}
	apply := func(meta map[string]string, pairs ...string) map[string]string {
		out := make(map[string]string, len(meta)+len(pairs)/2)
		for k, v := range meta {
			out[k] = v
		}
		for i := 0; i+1 < len(pairs); i += 2 {
			if pairs[i+1] == "" {
				delete(out, pairs[i])
			} else {
				out[pairs[i]] = pairs[i+1]
			}
		}
		return out
	}

	cases := []struct {
		name          string
		rawMeta       map[string]string
		wantCategory  MigrationCategory
		wantShouldMig bool
		wantSlugs     []string
		absentSlugs   []string
	}{
		{
			name: "inconsistent_part_metadata trips both rules",
			rawMeta: apply(base,
				armorMetaPartSize, inconsistentPartMetadataDeclaredSize,
				armorMetaPlaintextSize, "119",
				armorMetaPartCount, inconsistentPartMetadataCount,
			),
			wantCategory: CategoryContradictory,
			wantSlugs:    []string{contradictionPartSizeNotBlockAligned, contradictionPartCountContradicts},
		},
		{
			name:         "part size off the block grid",
			rawMeta:      apply(base, armorMetaPartSize, partSizeMismatchDeclaredPartSize),
			wantCategory: CategoryContradictory,
			wantSlugs:    []string{contradictionPartSizeNotBlockAligned},
		},
		{
			name: "part count contradicting the declared sizes",
			rawMeta: apply(base,
				armorMetaPartSize, "5242880",
				armorMetaPartCount, "999",
			),
			wantCategory: CategoryContradictory,
			wantSlugs:    []string{contradictionPartCountContradicts},
		},
		{
			name:          "uniform parts stay candidates",
			rawMeta:       apply(base, armorMetaPartSize, "5242880"),
			wantCategory:  CategoryV1Multipart,
			wantShouldMig: true,
		},
		{
			name:          "variable final part stays a candidate",
			rawMeta:       apply(base, armorMetaPartSize, "3145728"),
			wantCategory:  CategoryV1Multipart,
			wantShouldMig: true,
		},
		{
			name: "single part smaller than the declared part size stays a candidate",
			rawMeta: apply(base,
				armorMetaPartSize, "5242880",
				armorMetaPlaintextSize, "119",
			),
			wantCategory:  CategoryV1Multipart,
			wantShouldMig: true,
		},
		{
			name: "consistent foreign part count stays a candidate",
			rawMeta: apply(base,
				armorMetaPartSize, "5242880",
				armorMetaPartCount, "3",
			),
			wantCategory:  CategoryV1Multipart,
			wantShouldMig: true,
		},
		{
			// A uniform multipart claim without a part size is caught by the
			// pre-existing multipart-claims-missing-sizes rule; the part
			// accounting has no opinion of its own without a declared part
			// size to compare against, and stays out of the report.
			name: "part count without part size reports only the existing rule",
			rawMeta: apply(base,
				armorMetaPartSize, "",
				armorMetaPartCount, "3",
			),
			wantCategory: CategoryContradictory,
			wantSlugs:    []string{contradictionMultipartMissingSizes},
			absentSlugs:  []string{contradictionPartCountContradicts},
		},
		{
			name: "non-uniform object with unaligned part size stays a candidate",
			rawMeta: apply(base,
				armorMetaPartSize, "1000000",
				armorMetaNonUniform, "true",
			),
			wantCategory:  CategoryV1Multipart,
			wantShouldMig: true,
		},
		{
			name: "part size without multipart flag is the existing rule only",
			rawMeta: apply(base,
				armorMetaMultipart, "",
				armorMetaPartSize, "5242880",
			),
			wantCategory: CategoryContradictory,
			wantSlugs:    []string{contradictionMultipartFieldsNoFlag},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			category, reason, shouldMigrate := ClassifyMigrationObject(tc.rawMeta, 3)
			if category != tc.wantCategory {
				t.Fatalf("category = %q, want %q (reason %q)", category, tc.wantCategory, reason)
			}
			if shouldMigrate != tc.wantShouldMig {
				t.Fatalf("shouldMigrate = %t, want %t (reason %q)", shouldMigrate, tc.wantShouldMig, reason)
			}
			for _, slug := range tc.wantSlugs {
				if !strings.Contains(reason, slug) {
					t.Errorf("reason %q does not report rule %q", reason, slug)
				}
			}
			for _, slug := range tc.absentSlugs {
				if strings.Contains(reason, slug) {
					t.Errorf("reason %q reports untripped rule %q", reason, slug)
				}
			}
			if tc.wantCategory == CategoryContradictory && len(tc.wantSlugs) > 0 {
				if !strings.HasPrefix(reason, "contradictory metadata: ") {
					t.Errorf("reason %q is not a contradiction report", reason)
				}
			}
		})
	}
}

// TestValidatePartAccountingAgreesWithClassifier pins the agreement contract
// between the walk's pre-migrate gate and the inventory classifier: for the
// same metadata, the gate rejects exactly the inputs the classifier reports
// Contradictory for part-structure reasons, and neither side has an opinion
// the other lacks.
func TestValidatePartAccountingAgreesWithClassifier(t *testing.T) {
	corruptRows := [][]string{
		{inconsistentPartMetadataDeclaredSize, inconsistentPartMetadataCount},
		{partSizeMismatchDeclaredPartSize, ""},
	}
	for _, row := range corruptRows {
		meta := map[string]string{
			armorMetaVersion:       "1",
			armorMetaWrappedDEK:    "AAAA",
			armorMetaBlockSize:     "65536",
			armorMetaMultipart:     "true",
			armorMetaPartSize:      row[0],
			armorMetaPlaintextSize: "15728640",
		}
		if row[1] != "" {
			meta[armorMetaPartCount] = row[1]
		}
		category, reason, shouldMigrate := ClassifyMigrationObject(meta, 3)
		if category != CategoryContradictory || shouldMigrate {
			t.Fatalf("classifier accepted %v (category %q, reason %q)", meta, category, reason)
		}
		fm := &FormatMigrator{}
		if err := fm.validatePartAccounting(meta); err == nil {
			t.Fatalf("walk gate accepted what the classifier rejects: %v", meta)
		}
	}

	valid := map[string]string{
		armorMetaVersion:       "1",
		armorMetaWrappedDEK:    "AAAA",
		armorMetaBlockSize:     "65536",
		armorMetaMultipart:     "true",
		armorMetaPartSize:      "5242880",
		armorMetaPlaintextSize: "15728640",
	}
	if category, reason, _ := ClassifyMigrationObject(valid, 3); category != CategoryV1Multipart {
		t.Fatalf("valid uniform multipart classified %q (reason %q), want a candidate", category, reason)
	}
	fm := &FormatMigrator{}
	if err := fm.validatePartAccounting(valid); err != nil {
		t.Fatalf("walk gate rejected a valid uniform multipart: %v", err)
	}
}
