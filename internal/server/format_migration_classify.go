// Package server provides the pure per-object classifier the migration
// inventory is built from: source format version and layout, decided from
// raw metadata alone, plus the metadata self-contradiction checks that keep
// un-migratable objects out of the candidate set.
package server

import (
	"encoding/base64"
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
	// CategoryContradictory: a v1/v2 candidate whose metadata parses but
	// contradicts itself — a multipart claim without the sizes that layout
	// requires, multipart-only fields on a single-PUT object, a wrapped-DEK
	// prefix that disagrees with the claimed version, unusable size values,
	// base64 migrateObject's pre-validation would reject, or an ARMOR
	// version claimed without any key material. Detected and reported,
	// never migrated.
	CategoryContradictory MigrationCategory = "contradictory"
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
//   - CategoryContradictory: a known-major candidate whose metadata parses
//     but contradicts itself (see classifyContradictions for the full rule
//     list). Every rule the metadata trips is reported, in sorted order, so
//     an object matching two rules says so. Contradictory objects are
//     detected and reported, never migrated.
//
// shouldMigrate is true exactly for the four layout buckets, which by
// construction are known majors strictly below the target whose metadata
// is self-consistent.
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

	if reasons := classifyContradictions(rawMeta, version); len(reasons) > 0 {
		return CategoryContradictory,
			"contradictory metadata: " + strings.Join(reasons, "; "),
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

// classifyContradictions evaluates the metadata self-contradiction rules
// for one migration candidate (armor version 1 or 2, strictly below
// target). Like ClassifyMigrationObject it is pure and metadata-only — no
// backend reads, no decryption — and it returns one reason per rule the
// metadata trips, sorted so a multi-rule report is deterministic. A nil
// return means the candidate's metadata is self-consistent.
//
// The rules, each naming its own slug in the reason it emits:
//
//   - multipart-claims-missing-sizes: x-amz-meta-armor-multipart == "true"
//     but part-size or plaintext-size is absent or zero. The multipart
//     completion path writes both, so a claimed multipart layout without
//     them cannot be read back.
//   - multipart-fields-without-flag: the multipart flag is not "true" but
//     part-size is present — multipart-only metadata no ARMOR writer would
//     emit on a single-PUT object.
//   - v2-prefixed-dek-on-v1 / v2-dek-lacks-fingerprint-prefix: the v2 emit
//     rule (backend ToMetadata and the migration's buildNewMetadata) writes
//     the fingerprinted "v2:<fingerprint>:" wrapping for v2+ objects only,
//     and every v2+ wrapped DEK carries it; prefix and claimed version must
//     agree in both directions.
//   - nonpositive-block-size / negative-plaintext-size: block-size absent,
//     zero or negative (real writers always emit a positive block size),
//     or a plaintext-size below zero, which no plaintext can have.
//   - invalid-base64-wrapped-dek / invalid-base64-iv: base64 fields
//     migrateObject's pre-validation would reject. Classification must
//     agree with those validations, not diverge: same inputs, Contradictory
//     here rather than a separate later error. The wrapped-DEK decision is
//     the shared wrappedDEKBase64Error, so the two cannot drift apart.
//   - no-wrapped-dek-key-material: an ARMOR version claimed with no wrapped
//     DEK at all — ParseARMORMetadata treats the object as not
//     ARMOR-encrypted whatever the version header says, exactly the
//     contradiction the walk's srcContradictory bucket records.
func classifyContradictions(rawMeta map[string]string, version int) []string {
	var reasons []string
	rule := func(slug string, keys ...string) {
		reasons = append(reasons, slug+": "+classifyReasonFields(rawMeta, keys...))
	}

	partSize, partPresent := metaSize(rawMeta, armorMetaPartSize)
	plainSize, plainPresent := metaSize(rawMeta, armorMetaPlaintextSize)
	blockSize, blockPresent := metaSize(rawMeta, armorMetaBlockSize)
	dek := rawMeta[armorMetaWrappedDEK]
	iv := rawMeta[armorMetaIV]
	multipart := rawMeta[armorMetaMultipart] == "true"

	if multipart && (!(partPresent && partSize > 0) || !(plainPresent && plainSize > 0)) {
		rule("multipart-claims-missing-sizes", armorMetaMultipart, armorMetaPartSize, armorMetaPlaintextSize)
	}
	if !multipart && partPresent {
		rule("multipart-fields-without-flag", armorMetaMultipart, armorMetaPartSize)
	}
	if version == 1 && dekHasV2FingerprintPrefix(dek) {
		rule("v2-prefixed-dek-on-v1", armorMetaVersion, armorMetaWrappedDEK)
	}
	if version >= 2 && dek != "" && !dekHasV2FingerprintPrefix(dek) {
		rule("v2-dek-lacks-fingerprint-prefix", armorMetaVersion, armorMetaWrappedDEK)
	}
	if !(blockPresent && blockSize > 0) {
		rule("nonpositive-block-size", armorMetaVersion, armorMetaBlockSize)
	}
	if plainPresent && plainSize < 0 {
		rule("negative-plaintext-size", armorMetaVersion, armorMetaPlaintextSize)
	}
	if dek != "" && wrappedDEKBase64Error(dek) != nil {
		rule("invalid-base64-wrapped-dek", armorMetaWrappedDEK)
	}
	if iv != "" {
		if _, err := base64.StdEncoding.DecodeString(iv); err != nil {
			rule("invalid-base64-iv", armorMetaIV)
		}
	}
	if dek == "" {
		rule("no-wrapped-dek-key-material", armorMetaVersion, armorMetaWrappedDEK)
	}

	sort.Strings(reasons)
	return reasons
}

// metaSize reads one numeric metadata field the way the contradiction rules
// need it: present reports whether the key carries a non-empty value, and
// value is the parsed integer, 0 when the value is absent, empty or
// unparseable. Real ARMOR writers always emit parseable positive sizes, so
// a size that is absent, empty, unparseable or non-positive contradicts the
// object's own claims rather than being a value to migrate with.
func metaSize(rawMeta map[string]string, key string) (value int64, present bool) {
	raw, ok := rawMeta[key]
	if !ok || raw == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, true
	}
	return v, true
}

// dekHasV2FingerprintPrefix reports whether a wrapped-DEK value carries the
// "v2:<fingerprint>:" prefix the v2 emit rule writes: "v2:" plus at least
// two more characters. The threshold matches migrateObject's own prefix
// test exactly, so prefix detection and prefix validation can never
// disagree about the same value.
func dekHasV2FingerprintPrefix(dek string) bool {
	return len(dek) > 4 && dek[:3] == "v2:"
}

// wrappedDEKBase64Error reports how a wrapped-DEK metadata value fails the
// base64 pre-validation migrateObject applies before parsing metadata, or
// nil when the value passes. It is the single decision shared by the
// migration path and ClassifyMigrationObject, so the two can never disagree
// about which DEK values are unusable: the values migrateObject would
// reject with an error are exactly the values the classifier reports as
// contradictory. An empty value passes here (there are no bytes to
// reject); the absence of key material is classifyContradictions' own
// rule.
func wrappedDEKBase64Error(dek string) error {
	base64DEK := dek
	if dekHasV2FingerprintPrefix(dek) {
		parts := strings.SplitN(dek, ":", 3)
		if len(parts) == 3 && parts[0] == "v2" {
			base64DEK = parts[2]
		} else {
			return fmt.Errorf("invalid v2 wrapped DEK format: %s", dek)
		}
	}
	if _, err := base64.StdEncoding.DecodeString(base64DEK); err != nil {
		return fmt.Errorf("invalid base64 in wrapped DEK: %w", err)
	}
	return nil
}
