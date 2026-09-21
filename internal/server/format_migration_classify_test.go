// Package server tests for ClassifyMigrationObject, the pure version/layout
// classifier for the migration inventory: every category row, exact reason
// strings, and determinism across repeated invocations. Purity is by
// construction — the classifier takes a metadata map and a target and touches
// no backend — which is why these tests need nothing but the function itself.
package server

import (
	"testing"
)

// TestClassifyMigrationObjectCategories pins every category decision and the
// exact reason string for each: non-ARMOR, malformed (unparseable and
// unknown major), the four version x layout candidate buckets, and
// already-at-target against both a v2 and a v3 target. Reasons cite only the
// deciding metadata keys, rendered in sorted key order, so unrelated keys
// neither change the category nor leak into the reason. Candidate rows carry
// self-consistent metadata (valid wrapped DEK, positive sizes) so they
// characterize the layout decision; the metadata that trips the
// contradiction rules is characterized in
// format_migration_classify_contradictory_test.go.
func TestClassifyMigrationObjectCategories(t *testing.T) {
	cases := []struct {
		name         string
		rawMeta      map[string]string
		target       uint8
		wantCategory MigrationCategory
		wantReason   string
		wantMigrate  bool
	}{
		{
			name:         "no armor version key is non-armor",
			rawMeta:      map[string]string{"Content-Type": "text/plain"},
			target:       3,
			wantCategory: CategoryNonARMOR,
			wantReason:   "no armor version header: x-amz-meta-armor-version=<unset>",
			wantMigrate:  false,
		},
		{
			name:         "nil metadata is non-armor",
			rawMeta:      nil,
			target:       3,
			wantCategory: CategoryNonARMOR,
			wantReason:   "no armor version header: x-amz-meta-armor-version=<unset>",
			wantMigrate:  false,
		},
		{
			// Present but empty matches Migrate's own non-ARMOR branch,
			// which tests the header value for emptiness, not presence.
			name:         "empty version value is non-armor like the walk",
			rawMeta:      map[string]string{armorMetaVersion: ""},
			target:       3,
			wantCategory: CategoryNonARMOR,
			wantReason:   `no armor version header: x-amz-meta-armor-version=""`,
			wantMigrate:  false,
		},
		{
			name:         "unparseable version is malformed",
			rawMeta:      map[string]string{armorMetaVersion: "not-a-number"},
			target:       3,
			wantCategory: CategoryMalformed,
			wantReason:   `unparseable armor version: x-amz-meta-armor-version="not-a-number"`,
			wantMigrate:  false,
		},
		{
			// strconv.Atoi rejects trailing garbage the walk's lenient
			// Sscanf would have accepted as a leading integer.
			name:         "trailing-garbage version is malformed",
			rawMeta:      map[string]string{armorMetaVersion: "2x"},
			target:       3,
			wantCategory: CategoryMalformed,
			wantReason:   `unparseable armor version: x-amz-meta-armor-version="2x"`,
			wantMigrate:  false,
		},
		{
			name:         "version zero below target is an unknown major",
			rawMeta:      map[string]string{armorMetaVersion: "0"},
			target:       3,
			wantCategory: CategoryMalformed,
			wantReason:   `unknown armor version major 0 below target v3: x-amz-meta-armor-version="0"`,
			wantMigrate:  false,
		},
		{
			name:         "negative version below target is an unknown major",
			rawMeta:      map[string]string{armorMetaVersion: "-1"},
			target:       3,
			wantCategory: CategoryMalformed,
			wantReason:   `unknown armor version major -1 below target v3: x-amz-meta-armor-version="-1"`,
			wantMigrate:  false,
		},
		{
			name: "v1 without multipart flag is a single-put candidate",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaBlockSize:     "4096",
				armorMetaPlaintextSize: "2048",
			},
			target:       3,
			wantCategory: CategoryV1SinglePut,
			wantReason:   `armor v1 single-PUT: x-amz-meta-armor-multipart=<unset> x-amz-meta-armor-version="1"`,
			wantMigrate:  true,
		},
		{
			name: "v1 with multipart true is a multipart candidate",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaMultipart:     "true",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaBlockSize:     "65536",
				armorMetaPartSize:      "8388608",
				armorMetaPlaintextSize: "16777216",
			},
			target:       3,
			wantCategory: CategoryV1Multipart,
			wantReason:   `armor v1 multipart: x-amz-meta-armor-multipart="true" x-amz-meta-armor-version="1"`,
			wantMigrate:  true,
		},
		{
			name: "v2 with multipart false is a single-put candidate",
			rawMeta: map[string]string{
				armorMetaVersion:       "2",
				armorMetaMultipart:     "false",
				armorMetaWrappedDEK:    "v2:1111111111111111:AAAA",
				armorMetaBlockSize:     "4096",
				armorMetaPlaintextSize: "2048",
			},
			target:       3,
			wantCategory: CategoryV2SinglePut,
			wantReason:   `armor v2 single-PUT: x-amz-meta-armor-multipart="false" x-amz-meta-armor-version="2"`,
			wantMigrate:  true,
		},
		{
			name: "v2 with multipart true is a multipart candidate",
			rawMeta: map[string]string{
				armorMetaVersion:       "2",
				armorMetaMultipart:     "true",
				armorMetaWrappedDEK:    "v2:1111111111111111:AAAA",
				armorMetaBlockSize:     "65536",
				armorMetaPartSize:      "8388608",
				armorMetaPlaintextSize: "16777216",
			},
			target:       3,
			wantCategory: CategoryV2Multipart,
			wantReason:   `armor v2 multipart: x-amz-meta-armor-multipart="true" x-amz-meta-armor-version="2"`,
			wantMigrate:  true,
		},
		{
			// The layout flag is compared exactly, like the walk's
			// isMultipart; any other value means single-PUT.
			name: "multipart flag comparison is exact",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaMultipart:     "TRUE",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaBlockSize:     "4096",
				armorMetaPlaintextSize: "2048",
			},
			target:       3,
			wantCategory: CategoryV1SinglePut,
			wantReason:   `armor v1 single-PUT: x-amz-meta-armor-multipart="TRUE" x-amz-meta-armor-version="1"`,
			wantMigrate:  true,
		},
		{
			name:         "v3 against v3 target is already at target",
			rawMeta:      map[string]string{armorMetaVersion: "3"},
			target:       3,
			wantCategory: CategoryAlreadyAtTarget,
			wantReason:   `version 3 at or beyond target v3: x-amz-meta-armor-version="3"`,
			wantMigrate:  false,
		},
		{
			name:         "v2 against v2 target is already at target",
			rawMeta:      map[string]string{armorMetaVersion: "2"},
			target:       2,
			wantCategory: CategoryAlreadyAtTarget,
			wantReason:   `version 2 at or beyond target v2: x-amz-meta-armor-version="2"`,
			wantMigrate:  false,
		},
		{
			// Beyond-target versions mirror the walk's V3 bucket, which
			// collapses every version at or beyond the target: they are
			// never re-encrypted whatever they claim to be.
			name:         "version beyond target is already at target",
			rawMeta:      map[string]string{armorMetaVersion: "4"},
			target:       3,
			wantCategory: CategoryAlreadyAtTarget,
			wantReason:   `version 4 at or beyond target v3: x-amz-meta-armor-version="4"`,
			wantMigrate:  false,
		},
		{
			name:         "v3 against v2 target is beyond target",
			rawMeta:      map[string]string{armorMetaVersion: "3"},
			target:       2,
			wantCategory: CategoryAlreadyAtTarget,
			wantReason:   `version 3 at or beyond target v2: x-amz-meta-armor-version="3"`,
			wantMigrate:  false,
		},
		{
			name: "v1 against v2 target stays a candidate",
			rawMeta: map[string]string{
				armorMetaVersion:       "1",
				armorMetaMultipart:     "true",
				armorMetaWrappedDEK:    "AAAA",
				armorMetaBlockSize:     "65536",
				armorMetaPartSize:      "8388608",
				armorMetaPlaintextSize: "16777216",
			},
			target:       2,
			wantCategory: CategoryV1Multipart,
			wantReason:   `armor v1 multipart: x-amz-meta-armor-multipart="true" x-amz-meta-armor-version="1"`,
			wantMigrate:  true,
		},
		{
			name: "unrelated metadata keys do not change the category or leak into the reason",
			rawMeta: map[string]string{
				armorMetaVersion:        "2",
				armorMetaWrappedDEK:     "v2:1111111111111111:AAAA",
				armorMetaBlockSize:      "4096",
				"Content-Type":          "application/octet-stream",
				"x-amz-meta-armor-etag": "deadbeef",
				armorMetaPlaintextSize:  "4096",
				armorMetaPlaintextSHA:   "0123abcd",
			},
			target:       3,
			wantCategory: CategoryV2SinglePut,
			wantReason:   `armor v2 single-PUT: x-amz-meta-armor-multipart=<unset> x-amz-meta-armor-version="2"`,
			wantMigrate:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			category, reason, shouldMigrate := ClassifyMigrationObject(tc.rawMeta, tc.target)
			if category != tc.wantCategory {
				t.Errorf("category = %q, want %q", category, tc.wantCategory)
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
			if shouldMigrate != tc.wantMigrate {
				t.Errorf("shouldMigrate = %t, want %t", shouldMigrate, tc.wantMigrate)
			}
		})
	}
}

// TestClassifyMigrationObjectDeterministic checks the determinism contract:
// identical input yields byte-identical output across repeated invocations.
// Each input is classified 50 times against both a v2 and a v3 target; Go
// randomizes map iteration order per range, so an implementation that
// composed reasons by ranging over the raw metadata map would almost
// certainly diverge within those runs.
func TestClassifyMigrationObjectDeterministic(t *testing.T) {
	inputs := []map[string]string{
		{"Content-Type": "text/plain"},
		{armorMetaVersion: "not-a-number", "Content-Type": "text/plain"},
		{armorMetaVersion: "0"},
		{
			armorMetaVersion:       "1",
			armorMetaMultipart:     "true",
			armorMetaWrappedDEK:    "AAAA",
			armorMetaBlockSize:     "65536",
			armorMetaPartSize:      "8388608",
			armorMetaPlaintextSize: "16777216",
		},
		{
			armorMetaVersion:    "2",
			armorMetaWrappedDEK: "v2:1111111111111111:AAAA",
			armorMetaBlockSize:  "4096",
			"Content-Type":      "text/plain",
		},
		{armorMetaVersion: "3", armorMetaMultipart: "true"},
		{armorMetaVersion: "4"},
	}

	for _, rawMeta := range inputs {
		for _, target := range []uint8{2, 3} {
			wantCategory, wantReason, wantMigrate := ClassifyMigrationObject(rawMeta, target)
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
