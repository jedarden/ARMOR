// Package server provides the pure per-object classifier the migration
// inventory is built from: source format version and layout, decided from
// raw metadata alone.
package server

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// MigrationCategory is the source-format bucket ClassifyMigrationObject
// assigns to one object from its raw metadata alone. It is the
// version/layout dimension of ObjectClassification, decided without any
// backend I/O.
type MigrationCategory string

// The categories ClassifyMigrationObject can return. The four layout buckets
// are the migrate-from candidates; every other category is a skip, with the
// reason recorded alongside the inventory counts.
const (
	// CategoryNonARMOR: no x-amz-meta-armor-version header at all.
	CategoryNonARMOR MigrationCategory = "non_armor"
	// CategoryMalformed: version header present but unparseable, or naming a
	// major the migration does not know (strictly below target, neither v1
	// nor v2).
	CategoryMalformed MigrationCategory = "malformed"
	// CategoryV1SinglePut: version 1, no multipart flag.
	CategoryV1SinglePut MigrationCategory = "v1_single_put"
	// CategoryV1Multipart: version 1, x-amz-meta-armor-multipart == "true".
	CategoryV1Multipart MigrationCategory = "v1_multipart"
	// CategoryV2SinglePut: version 2, no multipart flag.
	CategoryV2SinglePut MigrationCategory = "v2_single_put"
	// CategoryV2Multipart: version 2, x-amz-meta-armor-multipart == "true".
	CategoryV2Multipart MigrationCategory = "v2_multipart"
	// CategoryAlreadyAtTarget: version at or beyond the configured target.
	// Such objects are never re-encrypted, so the layout dimension collapses
	// for them (the V3 bucket of ObjectClassification).
	CategoryAlreadyAtTarget MigrationCategory = "already_at_target"
)

// ClassifyMigrationObject classifies one object's raw S3 metadata for the
// migration inventory: it returns the source-format category, a
// human-readable reason, and whether the object is a migration candidate.
//
// The function is pure — it reads nothing but rawMeta and target and performs
// no backend I/O — and deterministic: identical input yields byte-identical
// output, because reasons are composed from a fixed, explicitly named set of
// metadata keys rendered in sorted key order (see classifyReasonFields),
// never from a range over the raw map, whose iteration order Go randomizes.
//
// target is the run's configured migration target as normalized by
// parseMigrationTarget — the server's write version — never a hard-coded
// format number. Categories, in decision order:
//
//   - CategoryNonARMOR: x-amz-meta-armor-version absent or empty, matching
//     Migrate's non-ARMOR skip branch.
//   - CategoryMalformed: the version header is present but unparseable
//     (strconv.Atoi; trailing garbage fails here even where the walk's
//     lenient Sscanf would scan a leading integer), or names an unknown
//     major — a version strictly below target that is neither 1 nor 2.
//   - CategoryAlreadyAtTarget: version at or beyond target. Objects at or
//     beyond the target are never re-encrypted (the migration only ever
//     re-encrypts known older majors), so their layout is not separately
//     reported — the same at-or-beyond rule the walk's classifyListedObject
//     applies for its V3 bucket.
//   - The four layout buckets: known majors strictly below target, split by
//     the exact x-amz-meta-armor-multipart == "true" comparison the walk
//     uses.
//
// shouldMigrate is true exactly for the four layout buckets, which by
// construction are known majors strictly below the target. The
// contradictory-metadata category (an ARMOR version claimed without key
// material) is deliberately absent here; it is owned by the classifier's
// metadata-contradiction extension, not the version/layout decision.
func ClassifyMigrationObject(rawMeta map[string]string, target uint8) (category MigrationCategory, reason string, shouldMigrate bool) {
	versionStr := rawMeta[armorMetaVersion]
	if versionStr == "" {
		return CategoryNonARMOR,
			"no armor version header: " + classifyReasonFields(rawMeta, armorMetaVersion),
			false
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return CategoryMalformed,
			"unparseable armor version: " + classifyReasonFields(rawMeta, armorMetaVersion),
			false
	}

	if version >= int(target) {
		return CategoryAlreadyAtTarget,
			fmt.Sprintf("version %d at or beyond target v%d: %s", version, target,
				classifyReasonFields(rawMeta, armorMetaVersion)),
			false
	}

	if version != 1 && version != 2 {
		return CategoryMalformed,
			fmt.Sprintf("unknown armor version major %d below target v%d: %s", version, target,
				classifyReasonFields(rawMeta, armorMetaVersion)),
			false
	}

	multipart := rawMeta[armorMetaMultipart] == "true"
	fields := classifyReasonFields(rawMeta, armorMetaMultipart, armorMetaVersion)
	switch {
	case version == 1 && multipart:
		return CategoryV1Multipart, "armor v1 multipart: " + fields, true
	case version == 1:
		return CategoryV1SinglePut, "armor v1 single-PUT: " + fields, true
	case multipart:
		return CategoryV2Multipart, "armor v2 multipart: " + fields, true
	default:
		return CategoryV2SinglePut, "armor v2 single-PUT: " + fields, true
	}
}

// classifyReasonFields renders the metadata fields a classification reason
// cites, as key=value pairs in sorted key order. Composing reasons from a
// sorted, explicitly named key list — never from a range over the raw map —
// is what makes ClassifyMigrationObject deterministic: Go randomizes map
// iteration order per range, so any map-range rendering could differ between
// two invocations on the same map. A key absent from the map renders as
// key=<unset> so the reason also records which deciding fields were missing.
func classifyReasonFields(rawMeta map[string]string, keys ...string) string {
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	parts := make([]string, 0, len(sorted))
	for _, k := range sorted {
		if v, ok := rawMeta[k]; ok {
			parts = append(parts, k+"="+strconv.Quote(v))
		} else {
			parts = append(parts, k+"=<unset>")
		}
	}
	return strings.Join(parts, " ")
}
