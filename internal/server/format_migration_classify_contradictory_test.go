// Package server tests for the Contradictory category of
// ClassifyMigrationObject: metadata that parses but contradicts itself.
// One fixture row per detection rule (each pinning the exact composed
// reason), the multi-rule composition order (sorted, every match
// reported), determinism across repeated invocations, purity (the input
// map is never mutated), and the agreement contract with migrateObject's
// base64 pre-validation — the DEK and IV values the migration rejects with
// an error are exactly the values classified contradictory.
package server

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/crypto"
)

// Contradiction-rule slugs, mirrored here so a slug rename breaks a test
// rather than silently reordering reports.
const (
	contradictionMultipartMissingSizes = "multipart-claims-missing-sizes"
	contradictionMultipartFieldsNoFlag = "multipart-fields-without-flag"
	contradictionV2PrefixedDEKOnV1     = "v2-prefixed-dek-on-v1"
	contradictionV2DEKLacksPrefix      = "v2-dek-lacks-fingerprint-prefix"
	contradictionNonpositiveBlockSize  = "nonpositive-block-size"
	contradictionNegativePlaintextSize = "negative-plaintext-size"
	contradictionInvalidBase64DEK      = "invalid-base64-wrapped-dek"
	contradictionInvalidBase64IV       = "invalid-base64-iv"
	contradictionNoWrappedDEK          = "no-wrapped-dek-key-material"
)

// contradictionFixture wraps one metadata map: a v1 single-PUT baseline
// with valid unprefixed key material and positive sizes, onto which each
// test row layers exactly the defect it characterizes.
func contradictionFixture() map[string]string {
	return map[string]string{
		armorMetaVersion:       "1",
		armorMetaWrappedDEK:    "AAAA",
		armorMetaBlockSize:     "4096",
		armorMetaPlaintextSize: "2048",
	}
}

// TestClassifyMigrationObjectContradictoryRules pins one fixture row per
// contradiction rule: the category is Contradictory, shouldMigrate is false
// (contradictory objects are detected and reported, never migrated), and the
// reason names the rule and its deciding fields. Every row trips exactly
// one rule; inputs tripping several are covered by
// TestClassifyMigrationObjectContradictoryMultiRule below.
func TestClassifyMigrationObjectContradictoryRules(t *testing.T) {
	cases := []struct {
		name       string
		rawMeta    map[string]string
		wantReason string
	}{
		{
			// multipart claimed, both multipart sizes absent
			name: "multipart flag true with both sizes absent",
			rawMeta: map[string]string{
				armorMetaVersion:    "1",
				armorMetaMultipart:  "true",
				armorMetaWrappedDEK: "AAAA",
				armorMetaBlockSize:  "65536",
			},
			wantReason: `contradictory metadata: multipart-claims-missing-sizes: x-amz-meta-armor-multipart="true" x-amz-meta-armor-part-size=<unset> x-amz-meta-armor-plaintext-size=<unset>`,
		},
		{
			// multipart claimed, both multipart sizes present but zero
			name: "multipart flag true with both sizes zero",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaMultipart:     "true",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaBlockSize:     "65536",
				armorMetaPartSize:      "0",
				armorMetaPlaintextSize: "0",
			},
			wantReason: `contradictory metadata: multipart-claims-missing-sizes: x-amz-meta-armor-multipart="true" x-amz-meta-armor-part-size="0" x-amz-meta-armor-plaintext-size="0"`,
		},
		{
			// multipart claimed, only the plaintext size is usable
			name: "multipart flag true with part size missing",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaMultipart:     "true",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaBlockSize:     "65536",
				armorMetaPlaintextSize: "16777216",
			},
			wantReason: `contradictory metadata: multipart-claims-missing-sizes: x-amz-meta-armor-multipart="true" x-amz-meta-armor-part-size=<unset> x-amz-meta-armor-plaintext-size="16777216"`,
		},
		{
			// part-size is multipart-only metadata; no writer emits it on a
			// single-PUT object
			name: "part size present without multipart flag",
			rawMeta: map[string]string{
				armorMetaVersion:       "2",
				armorMetaWrappedDEK:    "v2:1111111111111111:AAAA",
				armorMetaBlockSize:     "4096",
				armorMetaPartSize:      "8388608",
				armorMetaPlaintextSize: "2048",
			},
			wantReason: `contradictory metadata: multipart-fields-without-flag: x-amz-meta-armor-multipart=<unset> x-amz-meta-armor-part-size="8388608"`,
		},
		{
			// the fingerprinted wrapping is v2+ only (backend ToMetadata's
			// v2 emit rule); a v1 object cannot carry it
			name: "v2-prefixed wrapped DEK on version 1",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaWrappedDEK:    "v2:1111111111111111:AAAA",
				armorMetaBlockSize:     "4096",
				armorMetaPlaintextSize: "2048",
			},
			wantReason: `contradictory metadata: v2-prefixed-dek-on-v1: x-amz-meta-armor-version="1" x-amz-meta-armor-wrapped-dek="v2:1111111111111111:AAAA"`,
		},
		{
			// every v2+ wrapped DEK carries the fingerprint prefix
			name: "unprefixed wrapped DEK on version 2",
			rawMeta: map[string]string{
				armorMetaVersion:       "2",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaBlockSize:     "4096",
				armorMetaPlaintextSize: "2048",
			},
			wantReason: `contradictory metadata: v2-dek-lacks-fingerprint-prefix: x-amz-meta-armor-version="2" x-amz-meta-armor-wrapped-dek="AAAA"`,
		},
		{
			// block size zero: no envelope layout can use it
			name: "zero block size",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaBlockSize:     "0",
				armorMetaPlaintextSize: "2048",
			},
			wantReason: `contradictory metadata: nonpositive-block-size: x-amz-meta-armor-block-size="0" x-amz-meta-armor-version="1"`,
		},
		{
			// block size negative
			name: "negative block size",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaBlockSize:     "-4096",
				armorMetaPlaintextSize: "2048",
			},
			wantReason: `contradictory metadata: nonpositive-block-size: x-amz-meta-armor-block-size="-4096" x-amz-meta-armor-version="1"`,
		},
		{
			// block size absent: ARMOR writers always emit one, so a
			// version claim without it did not come from an ARMOR writer
			name: "absent block size",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaPlaintextSize: "2048",
			},
			wantReason: `contradictory metadata: nonpositive-block-size: x-amz-meta-armor-block-size=<unset> x-amz-meta-armor-version="1"`,
		},
		{
			// plaintext size below zero: no plaintext can have it
			name: "negative plaintext size",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaBlockSize:     "4096",
				armorMetaPlaintextSize: "-512",
			},
			wantReason: `contradictory metadata: negative-plaintext-size: x-amz-meta-armor-plaintext-size="-512" x-amz-meta-armor-version="1"`,
		},
		{
			// wrapped DEK that is not base64 at all
			name: "wrapped DEK with invalid base64",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaWrappedDEK:    "not!base64!",
				armorMetaBlockSize:     "4096",
				armorMetaPlaintextSize: "2048",
			},
			wantReason: `contradictory metadata: invalid-base64-wrapped-dek: x-amz-meta-armor-wrapped-dek="not!base64!"`,
		},
		{
			// v2-shaped DEK without the second colon: migrateObject's
			// pre-validation rejects it as an invalid v2 wrapped DEK format
			name: "v2-shaped wrapped DEK without fingerprint separator",
			rawMeta: map[string]string{
				armorMetaVersion:       "2",
				armorMetaWrappedDEK:    "v2:nosecondcolon",
				armorMetaBlockSize:     "4096",
				armorMetaPlaintextSize: "2048",
			},
			wantReason: `contradictory metadata: invalid-base64-wrapped-dek: x-amz-meta-armor-wrapped-dek="v2:nosecondcolon"`,
		},
		{
			// IV that is not base64
			name: "IV with invalid base64",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaIV:            "%%%invalid",
				armorMetaBlockSize:     "4096",
				armorMetaPlaintextSize: "2048",
			},
			wantReason: `contradictory metadata: invalid-base64-iv: x-amz-meta-armor-iv="%%%invalid"`,
		},
		{
			// an ARMOR version claimed with no key material at all:
			// ParseARMORMetadata (and therefore migration) treats the
			// object as not ARMOR-encrypted whatever the header says
			name:       "armor version claimed without wrapped DEK",
			rawMeta:    contradictionFixtureWithoutDEK(),
			wantReason: `contradictory metadata: no-wrapped-dek-key-material: x-amz-meta-armor-version="1" x-amz-meta-armor-wrapped-dek=<unset>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			category, reason, shouldMigrate := ClassifyMigrationObject(tc.rawMeta, 3)
			if category != CategoryContradictory {
				t.Errorf("category = %q, want %q", category, CategoryContradictory)
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q,\n      want %q", reason, tc.wantReason)
			}
			if shouldMigrate {
				t.Errorf("shouldMigrate = true, want false (contradictory objects are never migrated)")
			}
		})
	}
}

// contradictionFixtureWithoutDEK returns the baseline fixture stripped of
// its wrapped DEK.
func contradictionFixtureWithoutDEK() map[string]string {
	meta := contradictionFixture()
	delete(meta, armorMetaWrappedDEK)
	return meta
}

// TestClassifyMigrationObjectContradictoryMultiRule pins the composition
// contract: an input tripping several rules reports every one of them, in
// sorted order, as one deterministic reason string. The fixture trips five
// rules at once — invalid base64 in both the IV and the (v2-prefixed,
// v1-claimed) wrapped DEK, a multipart claim without either multipart size,
// and an absent block size.
func TestClassifyMigrationObjectContradictoryMultiRule(t *testing.T) {
	rawMeta := map[string]string{
		armorMetaVersion:    "1",
		armorMetaMultipart:  "true",
		armorMetaWrappedDEK: "v2:abcd:!!!",
		armorMetaIV:         "%%%",
	}

	wantReason := `contradictory metadata: ` +
		`invalid-base64-iv: x-amz-meta-armor-iv="%%%"; ` +
		`invalid-base64-wrapped-dek: x-amz-meta-armor-wrapped-dek="v2:abcd:!!!"; ` +
		`multipart-claims-missing-sizes: x-amz-meta-armor-multipart="true" x-amz-meta-armor-part-size=<unset> x-amz-meta-armor-plaintext-size=<unset>; ` +
		`nonpositive-block-size: x-amz-meta-armor-block-size=<unset> x-amz-meta-armor-version="1"; ` +
		`v2-prefixed-dek-on-v1: x-amz-meta-armor-version="1" x-amz-meta-armor-wrapped-dek="v2:abcd:!!!"`

	category, reason, shouldMigrate := ClassifyMigrationObject(rawMeta, 3)
	if category != CategoryContradictory {
		t.Fatalf("category = %q, want %q", category, CategoryContradictory)
	}
	if shouldMigrate {
		t.Fatalf("shouldMigrate = true, want false (contradictory objects are never migrated)")
	}
	if reason != wantReason {
		t.Fatalf("reason = %q,\n      want %q", reason, wantReason)
	}

	// Every matched rule is reported, and the reports appear in sorted
	// order: the index of each slug in the reason must increase.
	slugs := []string{
		contradictionInvalidBase64IV,
		contradictionInvalidBase64DEK,
		contradictionMultipartMissingSizes,
		contradictionNonpositiveBlockSize,
		contradictionV2PrefixedDEKOnV1,
	}
	last := -1
	for _, slug := range slugs {
		idx := strings.Index(reason, slug)
		if idx < 0 {
			t.Errorf("reason does not report matched rule %q", slug)
			continue
		}
		if idx < last {
			t.Errorf("rule %q reported out of sorted order (index %d after %d)", slug, idx, last)
		}
		last = idx
	}
	// Rules the input does not trip are absent from the report.
	for _, absent := range []string{
		contradictionMultipartFieldsNoFlag,
		contradictionV2DEKLacksPrefix,
		contradictionNegativePlaintextSize,
		contradictionNoWrappedDEK,
	} {
		if strings.Contains(reason, absent) {
			t.Errorf("reason reports untripped rule %q", absent)
		}
	}
}

// TestClassifyMigrationObjectContradictoryDeterministic extends the
// determinism contract to contradictory inputs: identical input yields
// byte-identical output across repeated invocations. Go randomizes map
// iteration order per range, so any rule or reason composed by ranging over
// the raw metadata map would diverge within these runs.
func TestClassifyMigrationObjectContradictoryDeterministic(t *testing.T) {
	inputs := []map[string]string{
		// the five-rule fixture from the multi-rule test
		{
			armorMetaVersion:    "1",
			armorMetaMultipart:  "true",
			armorMetaWrappedDEK: "v2:abcd:!!!",
			armorMetaIV:         "%%%",
		},
		// multipart claim with no sizes, no block size and no key
		// material: three rules at once
		{
			armorMetaVersion:   "1",
			armorMetaMultipart: "true",
		},
		// v2-prefixed DEK on v1 with negative sizes
		{
			armorMetaVersion:       "1",
			armorMetaWrappedDEK:    "v2:1111111111111111:AAAA",
			armorMetaBlockSize:     "-1",
			armorMetaPlaintextSize: "-2",
		},
		// v1 single-PUT with invalid base64 in both key fields (the DEK
		// v2-prefixed, so the prefix rule fires too) and a stray zero part
		// size
		{
			armorMetaVersion:       "1",
			armorMetaMultipart:     "false",
			armorMetaWrappedDEK:    "v2:fp:%%%",
			armorMetaIV:            "%%",
			armorMetaBlockSize:     "65536",
			armorMetaPartSize:      "0",
			armorMetaPlaintextSize: "10",
		},
	}

	for _, rawMeta := range inputs {
		for _, target := range []uint8{2, 3} {
			wantCategory, wantReason, wantMigrate := ClassifyMigrationObject(rawMeta, target)
			if wantCategory != CategoryContradictory {
				t.Fatalf("fixture %v at target v%d classified %q, want contradictory", rawMeta, target, wantCategory)
			}
			if wantReason == "" {
				t.Fatalf("classifier returned an empty reason for %v at target v%d", rawMeta, target)
			}
			for i := 0; i < 50; i++ {
				category, reason, shouldMigrate := ClassifyMigrationObject(rawMeta, target)
				if category != wantCategory || reason != wantReason || shouldMigrate != wantMigrate {
					t.Fatalf("call %d diverged for %v at target v%d:\n got (%q, %q, %t)\nwant (%q, %q, %t)",
						i, rawMeta, target, category, reason, shouldMigrate,
						wantCategory, wantReason, wantMigrate)
				}
			}
		}
	}
}

// TestClassifyMigrationObjectContradictoryPurity pins the purity contract
// from the input side: classifying a contradictory object reads the
// metadata map but never mutates it, and repeated calls on the same map
// return identical values.
func TestClassifyMigrationObjectContradictoryPurity(t *testing.T) {
	rawMeta := map[string]string{
		armorMetaVersion:    "1",
		armorMetaMultipart:  "true",
		armorMetaWrappedDEK: "v2:abcd:!!!",
		armorMetaIV:         "%%%",
		"Content-Type":      "text/plain",
	}
	snapshot := make(map[string]string, len(rawMeta))
	for k, v := range rawMeta {
		snapshot[k] = v
	}

	firstCategory, firstReason, firstMigrate := ClassifyMigrationObject(rawMeta, 3)
	if firstCategory != CategoryContradictory || firstMigrate {
		t.Fatalf("fixture classified (%q, %t), want (contradictory, false)", firstCategory, firstMigrate)
	}
	if !reflect.DeepEqual(rawMeta, snapshot) {
		t.Errorf("classification mutated the input metadata:\n got %v\nwant %v", rawMeta, snapshot)
	}

	secondCategory, secondReason, secondMigrate := ClassifyMigrationObject(rawMeta, 3)
	if secondCategory != firstCategory || secondReason != firstReason || secondMigrate != firstMigrate {
		t.Errorf("repeated classification diverged:\n got (%q, %q, %t)\nwant (%q, %q, %t)",
			secondCategory, secondReason, secondMigrate, firstCategory, firstReason, firstMigrate)
	}
	if !reflect.DeepEqual(rawMeta, snapshot) {
		t.Errorf("repeated classification mutated the input metadata:\n got %v\nwant %v", rawMeta, snapshot)
	}
}

// TestClassifyMigrationObjectAgreesWithMigrateObjectBase64Validation pins
// the agreement contract: for the same wrapped-DEK and IV values, exactly
// the inputs migrateObject's pre-validation rejects with an error are the
// inputs the classifier reports as contradictory. The classifier side and
// the migration side are exercised independently — the classifier directly,
// the migration through a one-object dry-run walk — so a future edit to
// either side that broke agreement fails here.
func TestClassifyMigrationObjectAgreesWithMigrateObjectBase64Validation(t *testing.T) {
	// wrapped-DEK values migrateObject rejects: not base64, v2-shaped
	// without the fingerprint separator, the too-short "v2:" and "v2:a"
	// (below the prefix threshold, so validated as plain base64 and
	// failing there), and a well-shaped v2 wrapping with an invalid
	// base64 payload.
	invalidDEKs := []string{
		"not!base64!",
		"v2:nosecondcolon",
		"v2:",
		"v2:a",
		"v2:1111111111111111:!!!",
	}
	for _, dek := range invalidDEKs {
		meta := contradictionFixture()
		meta[armorMetaWrappedDEK] = dek
		category, reason, shouldMigrate := ClassifyMigrationObject(meta, 3)
		if category != CategoryContradictory || shouldMigrate {
			t.Errorf("DEK %q: classified (%q, migrate=%t), want (contradictory, false)", dek, category, shouldMigrate)
		}
		if !strings.Contains(reason, contradictionInvalidBase64DEK) {
			t.Errorf("DEK %q: reason does not report invalid base64: %q", dek, reason)
		}
		failure := classifyAgreementMigrateFailure(t, meta)
		if !strings.Contains(failure, "invalid base64") && !strings.Contains(failure, "invalid v2 wrapped DEK format") {
			t.Errorf("DEK %q: migrateObject failure does not blame base64 validation: %q", dek, failure)
		}
	}

	invalidIVs := []string{"%%%", "not base64"}
	for _, iv := range invalidIVs {
		meta := contradictionFixture()
		meta[armorMetaIV] = iv
		category, reason, shouldMigrate := ClassifyMigrationObject(meta, 3)
		if category != CategoryContradictory || shouldMigrate {
			t.Errorf("IV %q: classified (%q, migrate=%t), want (contradictory, false)", iv, category, shouldMigrate)
		}
		if !strings.Contains(reason, contradictionInvalidBase64IV) {
			t.Errorf("IV %q: reason does not report invalid base64: %q", iv, reason)
		}
		failure := classifyAgreementMigrateFailure(t, meta)
		if !strings.Contains(failure, "invalid base64") {
			t.Errorf("IV %q: migrateObject failure does not blame base64 validation: %q", iv, failure)
		}
	}

	// Values that pass validation must not trip the base64 rules — even
	// when they trip other contradiction rules (the v2-prefixed DEK on a
	// v1 object is contradictory for its prefix, not its base64).
	for _, dek := range []string{"AAAA", "v2:1111111111111111:AAAA"} {
		meta := contradictionFixture()
		meta[armorMetaWrappedDEK] = dek
		_, reason, _ := ClassifyMigrationObject(meta, 3)
		if strings.Contains(reason, contradictionInvalidBase64DEK) {
			t.Errorf("valid DEK %q reported as invalid base64: %q", dek, reason)
		}
	}
}

// classifyAgreementMigrateFailure runs one dry-run migration walk over a
// single object carrying the given metadata and returns the recorded
// failure reason. migrateObject validates the base64 fields before
// anything else, so a metadata-only defect fails there rather than later
// in the walk.
func classifyAgreementMigrateFailure(t *testing.T, meta map[string]string) string {
	t.Helper()

	mockBackend := NewMockBackend()
	mockBackend.objects["agreement.dat"] = &MockObject{
		Data:     []byte("ciphertext"),
		Metadata: meta,
	}

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	result, err := migrator.Migrate(context.Background(), true, 1)
	if err != nil {
		t.Logf("migration returned error (expected for invalid metadata): %v", err)
	}
	if result.FailedObjects != 1 || len(result.Failures) != 1 {
		t.Fatalf("dry-run walk recorded %d failures (%d failed objects), want exactly 1: %v",
			len(result.Failures), result.FailedObjects, result.Failures)
	}
	return result.Failures[0].Reason
}
