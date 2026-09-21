// Package server tests for the migration classification counts: the
// per-dimension inventory report (source/layout, size bucket, key
// fingerprint, outcome). The inventory pass (countObjects) owns and
// populates the static dimensions; the migration walk owns the outcome
// counters, and a full Migrate() run reports both combined.
package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// clsFingerprintA is a synthetic 16-hex fingerprint used for hand-built
// v2-format wrapped DEKs whose key material is never unwrapped (the objects
// carrying them are skipped before any decryption happens).
const clsFingerprintA = "1111111111111111"

// clsTestMEK returns the deterministic 32-byte test MEK used across the
// classification fixtures.
func clsTestMEK() []byte {
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}
	return mek
}

// clsTestDEK returns a deterministic 32-byte test DEK.
func clsTestDEK() []byte {
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 1)
	}
	return dek
}

// putRealV1SinglePUT stores a genuinely migratable v1 single-PUT object,
// built exactly the way the round-trip tests build theirs: real KWP-wrapped
// DEK, real envelope header, real ciphertext and HMAC table.
func putRealV1SinglePUT(t *testing.T, mb *MockBackend, mek []byte, key string) {
	t.Helper()

	dek := clsTestDEK()
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("failed to wrap DEK: %v", err)
	}

	iv := make([]byte, 16)
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, 4096, crypto.Version1)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	plaintext := []byte("test data for migration")
	ciphertext, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), 4096, plaintextSHA, crypto.Version1)
	if err != nil {
		t.Fatalf("failed to build envelope header: %v", err)
	}
	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("failed to encode envelope header: %v", err)
	}

	body := append(append([]byte{}, ciphertext...), hmacTable...)
	mb.objects[key] = &MockObject{
		Data: append(headerBuf, body...),
		Metadata: map[string]string{
			armorMetaVersion:       "1",
			armorMetaWrappedDEK:    base64.StdEncoding.EncodeToString(wrappedDEK),
			armorMetaIV:            base64.StdEncoding.EncodeToString(iv),
			armorMetaBlockSize:     "4096",
			armorMetaPlaintextSize: "25",
			armorMetaPlaintextSHA:  "test-sha256",
		},
	}
}

// putIntegrityFailureV2 stores a v2 single-PUT object that decrypts to
// nothing: the envelope header is valid but the ciphertext body is random
// bytes, so HMAC verification fails and the walk records an
// integrity-failed outcome rather than a plain failure.
func putIntegrityFailureV2(t *testing.T, mb *MockBackend, mek []byte, key string) {
	t.Helper()

	wrappedDEK, err := crypto.WrapDEKWithFingerprint(mek, clsTestDEK())
	if err != nil {
		t.Fatalf("failed to wrap DEK with fingerprint: %v", err)
	}

	iv := make([]byte, 16)
	plaintextSHA := crypto.ComputePlaintextSHA256([]byte("corrupted payload"))
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, 25, 4096, plaintextSHA, crypto.Version2)
	if err != nil {
		t.Fatalf("failed to build envelope header: %v", err)
	}
	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("failed to encode envelope header: %v", err)
	}

	// 64 bytes of deterministic garbage: header.PlaintextSize 25 with block
	// size 4096 means one block, so decryptSingleObject splits off a 32-byte
	// HMAC table and fails verifying it.
	garbage := make([]byte, 64)
	for i := range garbage {
		garbage[i] = byte(0xA5)
	}

	mb.objects[key] = &MockObject{
		Data: append(headerBuf, garbage...),
		Metadata: map[string]string{
			armorMetaVersion:       "2",
			armorMetaWrappedDEK:    wrappedDEK,
			armorMetaIV:            base64.StdEncoding.EncodeToString(iv),
			armorMetaBlockSize:     "4096",
			armorMetaPlaintextSize: "2147483648", // 2 GiB: classification size is metadata-driven
		},
	}
}

// putSidecarlessMultipart stores a multipart-flagged object whose DEK
// unwraps fine but whose HMAC sidecar does not exist, so the migration
// attempt fails before any content verification (a plain failure, not an
// integrity failure). v2Style selects the DEK encoding. Both size headers a
// real multipart writer emits are present, so the object classifies as a
// genuine multipart layout rather than contradictory metadata (a multipart
// claim without part/plaintext sizes trips ClassifyMigrationObject's
// multipart-claims-missing-sizes rule).
func putSidecarlessMultipart(t *testing.T, mb *MockBackend, mek []byte, key string, version uint8, v2Style bool) {
	t.Helper()

	var dek string
	if v2Style {
		d, err := crypto.WrapDEKWithFingerprint(mek, clsTestDEK())
		if err != nil {
			t.Fatalf("failed to wrap DEK with fingerprint: %v", err)
		}
		dek = d
	} else {
		d, err := crypto.WrapDEK(mek, clsTestDEK())
		if err != nil {
			t.Fatalf("failed to wrap DEK: %v", err)
		}
		dek = base64.StdEncoding.EncodeToString(d)
	}

	iv := make([]byte, 16)
	mb.objects[key] = &MockObject{
		Data: []byte("multipart ciphertext without sidecar"),
		Metadata: map[string]string{
			armorMetaVersion:       fmt.Sprintf("%d", version),
			armorMetaWrappedDEK:    dek,
			armorMetaIV:            base64.StdEncoding.EncodeToString(iv),
			armorMetaBlockSize:     "4096",
			armorMetaMultipart:     "true",
			armorMetaPartSize:      "4096",
			armorMetaPlaintextSize: "37",
		},
	}
}

// buildClassificationInventory populates mb with a ten-object inventory
// covering every source bucket, every skip and failure path, and several
// size buckets, then returns the classification a correct walk must report
// for it plus the real MEK fingerprint the v2-wrapped objects carry.
//
// The walk runs with target version 3 and include list {1,2}: v3+ objects
// are skipped (not-in-include and at-target), v1 and v2 objects are
// candidates.
func buildClassificationInventory(t *testing.T, mb *MockBackend) (ObjectClassification, string) {
	t.Helper()

	mek := clsTestMEK()
	fpMEK := crypto.MEKFingerprint(mek)

	// a: not ARMOR-encrypted at all -> non-armor, skipped, <1MB.
	mb.objects["a-plain.bin"] = &MockObject{
		Data:     make([]byte, 100),
		Metadata: map[string]string{"Content-Type": "text/plain"},
	}

	// b: v3 single-PUT at target version -> v3 bucket (layout collapses for
	// v3), skipped as at-target, 1-10MB (metadata plaintext size),
	// fingerprint A.
	mb.objects["b-v3-at-target.bin"] = &MockObject{
		Data: []byte("at target"),
		Metadata: map[string]string{
			armorMetaVersion:       "3",
			armorMetaWrappedDEK:    "v2:" + clsFingerprintA + ":AAAA",
			armorMetaPlaintextSize: "5242880", // 5 MiB
		},
	}

	// c: v3 object, newer than every include entry -> v3, skipped,
	// 100MB-1GB, fingerprint A.
	mb.objects["c-v3-newer.bin"] = &MockObject{
		Data: []byte("newer format"),
		Metadata: map[string]string{
			armorMetaVersion:       "3",
			armorMetaWrappedDEK:    "v2:" + clsFingerprintA + ":AAAA",
			armorMetaPlaintextSize: "157286400", // 150 MiB
		},
	}

	// d: a real v1 single-PUT object that migrates successfully ->
	// v1_single_put, processed, <1MB, legacy DEK wrapping.
	putRealV1SinglePUT(t, mb, mek, "d-v1-single-live.bin")

	// e: v2 multipart, DEK unwraps but sidecar is missing -> v2_multipart,
	// failed, <1MB, real MEK fingerprint.
	putSidecarlessMultipart(t, mb, mek, "e-v2-multipart.bin", 2, true)

	// f: v1 multipart, same missing-sidecar failure with legacy wrapping ->
	// v1_multipart, failed, <1MB, legacy.
	putSidecarlessMultipart(t, mb, mek, "f-v1-multipart.bin", 1, false)

	// g: claims v2 but carries no wrapped DEK -> contradictory, failed,
	// <1MB, no key material.
	mb.objects["g-contradictory.bin"] = &MockObject{
		Data: []byte("version without key material"),
		Metadata: map[string]string{
			armorMetaVersion:   "2",
			armorMetaBlockSize: "4096",
		},
	}

	// h: armor-version header present but unparseable -> malformed, skipped.
	mb.objects["h-malformed-version.bin"] = &MockObject{
		Data: []byte("unparseable version"),
		Metadata: map[string]string{
			armorMetaVersion: "not-a-number",
		},
	}

	// i: v2 single-PUT with valid header and corrupt body ->
	// v2_single_put, integrity-failed, 1-10GB (metadata), real fingerprint.
	putIntegrityFailureV2(t, mb, mek, "i-v2-integrity.bin")

	// j: another v3 object, >10GB by metadata -> v3, skipped, fingerprint A.
	mb.objects["j-v3-huge.bin"] = &MockObject{
		Data: []byte("huge by metadata"),
		Metadata: map[string]string{
			armorMetaVersion:       "3",
			armorMetaWrappedDEK:    "v2:" + clsFingerprintA + ":AAAA",
			armorMetaPlaintextSize: "21474836480", // 20 GiB
		},
	}

	want := ObjectClassification{
		V1SinglePut:   1, // d
		V1Multipart:   1, // f
		V2SinglePut:   1, // i (integrity failure)
		V2Multipart:   1, // e
		V3:            3, // b (at target), c, j
		NonARMOR:      1, // a
		Malformed:     1, // h
		Contradictory: 1, // g
	}

	want.SizeLessThan1MB = 6 // a, d, e, f, g, h
	want.Size1MBTo10MB = 1   // b
	want.Size100MBTo1GB = 1  // c
	want.Size1GBTo10GB = 1   // i
	want.SizeGreater10GB = 1 // j
	want.Size10MBTo100MB = 0 // not covered by this inventory

	want.ByKeyFingerprint = map[string]int{
		clsFingerprintA: 3, // b, c, j
		fpMEK:           2, // e, i
		"legacy":        2, // d, f
	}

	want.OutcomeProcessed = 1       // d
	want.OutcomeSkipped = 5         // a, b, c, h, j
	want.OutcomeFailed = 3          // e, f, g
	want.OutcomeIntegrityFailed = 1 // i

	return want, fpMEK
}

// TestObjectClassificationRecordBucketsEveryDimension drives record() across
// every source, outcome and fingerprint combination and checks each
// dimension landed in exactly one bucket, that the dimensions agree, and
// that the fingerprint dimension may total less than Total().
func TestObjectClassificationRecordBucketsEveryDimension(t *testing.T) {
	var c ObjectClassification

	sources := []sourceKind{
		srcV1SinglePut, srcV1Multipart, srcV2SinglePut, srcV2Multipart,
		srcV3Plus, srcNonARMOR, srcMalformed, srcContradictory,
	}
	outcomes := []outcomeKind{
		outcomeProcessed, outcomeSkipped, outcomeFailed, outcomeIntegrityFailed,
	}

	// 8 sources x 4 outcomes = 32 objects, sizes all in the first bucket,
	// fingerprint only for half of them.
	for i, src := range sources {
		for j, outcome := range outcomes {
			fp := ""
			if (i+j)%2 == 0 {
				fp = "aaabbbccccddddee"
			}
			c.record(src, 100, fp, outcome)
		}
	}

	if c.V1SinglePut != 4 || c.V1Multipart != 4 || c.V2SinglePut != 4 || c.V2Multipart != 4 {
		t.Errorf("source layout buckets wrong: v1s=%d v1m=%d v2s=%d v2m=%d", c.V1SinglePut, c.V1Multipart, c.V2SinglePut, c.V2Multipart)
	}
	if c.V3 != 4 || c.NonARMOR != 4 || c.Malformed != 4 || c.Contradictory != 4 {
		t.Errorf("source version buckets wrong: v3=%d non-armor=%d malformed=%d contradictory=%d", c.V3, c.NonARMOR, c.Malformed, c.Contradictory)
	}
	if c.SizeLessThan1MB != 32 {
		t.Errorf("size bucket wrong: <1MB=%d, want 32", c.SizeLessThan1MB)
	}
	if c.OutcomeProcessed != 8 || c.OutcomeSkipped != 8 || c.OutcomeFailed != 8 || c.OutcomeIntegrityFailed != 8 {
		t.Errorf("outcome buckets wrong: processed=%d skipped=%d failed=%d integrity=%d",
			c.OutcomeProcessed, c.OutcomeSkipped, c.OutcomeFailed, c.OutcomeIntegrityFailed)
	}
	if got := c.ByKeyFingerprint["aaabbbccccddddee"]; got != 16 {
		t.Errorf("fingerprint bucket wrong: got %d, want 16", got)
	}

	if total := c.Total(); total != 32 {
		t.Errorf("Total() = %d, want 32", total)
	}
	if !c.Balanced() {
		t.Error("Balanced() = false for a classification where every dimension got exactly one increment per object")
	}
}

// TestObjectClassificationSizeBoundaries pins the size-bucket edges: every
// boundary value lands in the upper bucket (comparisons are strict <).
func TestObjectClassificationSizeBoundaries(t *testing.T) {
	const (
		mb1   = 1 << 20
		mb10  = 10 << 20
		mb100 = 100 << 20
		gb1   = 1 << 30
		gb10  = 10 << 30
	)

	cases := []struct {
		size int64
		want int
	}{
		{0, 0},
		{100, 0},
		{mb1 - 1, 0},
		{mb1, 1},
		{mb10 - 1, 1},
		{mb10, 2},
		{mb100 - 1, 2},
		{mb100, 3},
		{gb1 - 1, 3},
		{gb1, 4},
		{gb10 - 1, 4},
		{gb10, 5},
		{gb10 * 3, 5},
	}

	var c ObjectClassification
	for _, tc := range cases {
		c = ObjectClassification{}
		c.record(srcV1SinglePut, tc.size, "", outcomeSkipped)
		switch tc.want {
		case 0:
			if c.SizeLessThan1MB != 1 {
				t.Errorf("size %d should land in <1MB", tc.size)
			}
		case 1:
			if c.Size1MBTo10MB != 1 {
				t.Errorf("size %d should land in 1MB-10MB", tc.size)
			}
		case 2:
			if c.Size10MBTo100MB != 1 {
				t.Errorf("size %d should land in 10MB-100MB", tc.size)
			}
		case 3:
			if c.Size100MBTo1GB != 1 {
				t.Errorf("size %d should land in 100MB-1GB", tc.size)
			}
		case 4:
			if c.Size1GBTo10GB != 1 {
				t.Errorf("size %d should land in 1GB-10GB", tc.size)
			}
		case 5:
			if c.SizeGreater10GB != 1 {
				t.Errorf("size %d should land in >10GB", tc.size)
			}
		}
	}
}

// TestObjectClassificationBalancedDetectsImbalance checks that Balanced()
// rejects classifications whose dimensions disagree.
func TestObjectClassificationBalancedDetectsImbalance(t *testing.T) {
	base := func() ObjectClassification {
		var c ObjectClassification
		c.record(srcV1SinglePut, 100, "fp", outcomeProcessed)
		c.record(srcV2Multipart, 100, "", outcomeSkipped)
		return c
	}
	balancedBase := base()
	if !balancedBase.Balanced() {
		t.Fatal("baseline classification should balance")
	}

	sizeOff := base()
	sizeOff.SizeLessThan1MB++ // size dimension now sums high
	if sizeOff.Balanced() {
		t.Error("Balanced() = true with an inflated size total")
	}

	outcomeOff := base()
	outcomeOff.OutcomeFailed++ // outcome dimension now sums high
	if outcomeOff.Balanced() {
		t.Error("Balanced() = true with an inflated outcome total")
	}

	sourceOff := base()
	sourceOff.V3++ // source dimension now sums high
	if sourceOff.Balanced() {
		t.Error("Balanced() = true with an inflated source total")
	}
}

// TestObjectClassificationSummary checks the human-readable rendering: one
// line per dimension, per-dimension totals, sorted fingerprints, and the
// empty-fingerprint case.
func TestObjectClassificationSummary(t *testing.T) {
	var c ObjectClassification
	c.record(srcV2SinglePut, 100, "bbbbbbbbbbbbbbbb", outcomeProcessed)
	c.record(srcV2SinglePut, 100, "aaaaaaaaaaaaaaaa", outcomeProcessed)
	c.record(srcNonARMOR, 100, "", outcomeSkipped)

	s := c.Summary()
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("Summary() rendered %d lines, want 4 (source, size, keys, outcome):\n%s", len(lines), s)
	}
	for i, prefix := range []string{"source:", "size:", "keys:", "outcome:"} {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Errorf("line %d = %q, want prefix %q", i, lines[i], prefix)
		}
	}
	if !strings.Contains(lines[0], "v2-single=2") || !strings.Contains(lines[0], "non-armor=1") {
		t.Errorf("source line missing buckets: %q", lines[0])
	}
	if !strings.Contains(lines[0], "(total 3)") {
		t.Errorf("source line missing total: %q", lines[0])
	}
	if !strings.Contains(lines[2], "aaaaaaaaaaaaaaaa=1 bbbbbbbbbbbbbbbb=1") {
		t.Errorf("keys line not sorted or missing buckets: %q", lines[2])
	}
	if !strings.Contains(lines[2], "(total 2)") {
		t.Errorf("keys line missing total: %q", lines[2])
	}
	if !strings.Contains(lines[3], "processed=2 skipped=1") || !strings.Contains(lines[3], "(total 3)") {
		t.Errorf("outcome line wrong: %q", lines[3])
	}

	// No key material anywhere: the keys dimension renders an explicit none.
	var none ObjectClassification
	none.record(srcNonARMOR, 100, "", outcomeSkipped)
	if noneLine := none.Summary(); !strings.Contains(noneLine, "(none)") {
		t.Errorf("Summary() without fingerprints should render (none):\n%s", noneLine)
	}
}

// TestObjectClassificationCopyIsDeep checks the snapshot handed out on
// MigrationResult shares no state with the live counters.
func TestObjectClassificationCopyIsDeep(t *testing.T) {
	var c ObjectClassification
	c.record(srcV1SinglePut, 100, "aaaaaaaaaaaaaaaa", outcomeProcessed)

	snap := c.copy()
	snap.ByKeyFingerprint["aaaaaaaaaaaaaaaa"] = 99
	if got := c.ByKeyFingerprint["aaaaaaaaaaaaaaaa"]; got != 1 {
		t.Errorf("copy shares the fingerprint map: live counter mutated to %d", got)
	}
}

// TestMigrationResultClassificationJSON checks the machine-readable shape:
// the classification travels on the result JSON with every dimension
// encoded, and decodes back identically.
func TestMigrationResultClassificationJSON(t *testing.T) {
	var c ObjectClassification
	c.record(srcV2Multipart, 5<<20, "aaaaaaaaaaaaaaaa", outcomeFailed)
	c.record(srcV1SinglePut, 100, "", outcomeProcessed)

	result := MigrationResult{
		TotalObjects:     2,
		ProcessedObjects: 1,
		FailedObjects:    1,
		Classification:   c,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	cls, ok := raw["classification"].(map[string]interface{})
	if !ok {
		t.Fatalf("result JSON has no classification object: %s", data)
	}
	for _, key := range []string{
		"v1_single_put", "v1_multipart", "v2_single_put", "v2_multipart",
		"v3", "non_armor", "malformed", "contradictory",
		"size_lt_1mb", "size_1mb_to_10mb", "size_10mb_to_100mb",
		"size_100mb_to_1gb", "size_1gb_to_10gb", "size_gt_10gb",
		"by_key_fingerprint",
		"outcome_processed", "outcome_skipped", "outcome_failed", "outcome_integrity_failed",
	} {
		if _, ok := cls[key]; !ok {
			t.Errorf("classification JSON missing key %q: %s", key, data)
		}
	}

	var back MigrationResult
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(back.Classification, result.Classification) {
		t.Errorf("classification did not round-trip:\n got %+v\nwant %+v", back.Classification, result.Classification)
	}
}

// TestInventoryDerivation pins the per-object derivation rules of the
// inventory pass: the source bucket from ClassifyMigrationObject mapped
// through sourceKindForCategory, the fingerprint from the parsed metadata's
// MEKFingerprint (legacy label for v1-style wrapping, empty without key
// material), and the size from the recorded plaintext size with
// listing-size fallback.
func TestInventoryDerivation(t *testing.T) {
	legacyDEK := base64.StdEncoding.EncodeToString([]byte("raw-wrapped-dek-bytes"))
	v2DEK := "v2:aaaaaaaaaaaaaaaa:AAAA"

	cases := []struct {
		name     string
		rawMeta  map[string]string
		listSize int64
		wantSrc  sourceKind
		wantSize int64
		wantFP   string
	}{
		{
			name:     "v1 single put legacy dek",
			rawMeta:  map[string]string{armorMetaVersion: "1", armorMetaWrappedDEK: legacyDEK, armorMetaBlockSize: "4096", armorMetaPlaintextSize: "4096"},
			listSize: 9999,
			wantSrc:  srcV1SinglePut, wantSize: 4096, wantFP: "legacy",
		},
		{
			name:     "v1 multipart with sizes",
			rawMeta:  map[string]string{armorMetaVersion: "1", armorMetaWrappedDEK: legacyDEK, armorMetaBlockSize: "4096", armorMetaMultipart: "true", armorMetaPartSize: "4096", armorMetaPlaintextSize: "8192"},
			listSize: 100,
			wantSrc:  srcV1Multipart, wantSize: 8192, wantFP: "legacy",
		},
		{
			name:     "v2 single put fingerprinted dek",
			rawMeta:  map[string]string{armorMetaVersion: "2", armorMetaWrappedDEK: v2DEK, armorMetaBlockSize: "4096"},
			listSize: 100,
			wantSrc:  srcV2SinglePut, wantSize: 100, wantFP: "aaaaaaaaaaaaaaaa",
		},
		{
			name:     "v2 multipart with sizes",
			rawMeta:  map[string]string{armorMetaVersion: "2", armorMetaWrappedDEK: v2DEK, armorMetaBlockSize: "4096", armorMetaMultipart: "true", armorMetaPartSize: "4096", armorMetaPlaintextSize: "8192"},
			listSize: 100,
			wantSrc:  srcV2Multipart, wantSize: 8192, wantFP: "aaaaaaaaaaaaaaaa",
		},
		{
			name:     "v3 at target collapses layout",
			rawMeta:  map[string]string{armorMetaVersion: "3", armorMetaWrappedDEK: v2DEK},
			listSize: 100,
			wantSrc:  srcV3Plus, wantSize: 100, wantFP: "aaaaaaaaaaaaaaaa",
		},
		{
			name:     "version beyond target",
			rawMeta:  map[string]string{armorMetaVersion: "4", armorMetaWrappedDEK: v2DEK},
			listSize: 100,
			wantSrc:  srcV3Plus, wantSize: 100, wantFP: "aaaaaaaaaaaaaaaa",
		},
		{
			name:     "non armor object",
			rawMeta:  map[string]string{"Content-Type": "text/plain"},
			listSize: 100,
			wantSrc:  srcNonARMOR, wantSize: 100, wantFP: "",
		},
		{
			name:     "unparseable version header",
			rawMeta:  map[string]string{armorMetaVersion: "not-a-number"},
			listSize: 100,
			wantSrc:  srcMalformed, wantSize: 100, wantFP: "",
		},
		{
			name:     "contradictory claims version without dek",
			rawMeta:  map[string]string{armorMetaVersion: "2", armorMetaBlockSize: "4096"},
			listSize: 100,
			wantSrc:  srcContradictory, wantSize: 100, wantFP: "",
		},
		{
			name:     "contradictory multipart claim without sizes",
			rawMeta:  map[string]string{armorMetaVersion: "2", armorMetaWrappedDEK: v2DEK, armorMetaBlockSize: "4096", armorMetaMultipart: "true"},
			listSize: 100,
			wantSrc:  srcContradictory, wantSize: 100, wantFP: "aaaaaaaaaaaaaaaa",
		},
		{
			name:     "no plaintext size falls back to listing size",
			rawMeta:  map[string]string{armorMetaVersion: "1", armorMetaWrappedDEK: legacyDEK, armorMetaBlockSize: "4096"},
			listSize: 4096,
			wantSrc:  srcV1SinglePut, wantSize: 4096, wantFP: "legacy",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			armorMeta, _ := backend.ParseARMORMetadata(tc.rawMeta)
			src := sourceKindForCategory(mustClassify(t, tc.rawMeta, crypto.Version3))
			size := inventorySizeBytes(armorMeta, tc.listSize)
			fp := inventoryFingerprint(armorMeta)
			if src != tc.wantSrc {
				t.Errorf("source = %v, want %v", src, tc.wantSrc)
			}
			if size != tc.wantSize {
				t.Errorf("size = %d, want %d", size, tc.wantSize)
			}
			if fp != tc.wantFP {
				t.Errorf("fingerprint = %q, want %q", fp, tc.wantFP)
			}
		})
	}
}

// mustClassify asserts ClassifyMigrationObject returns one of the known
// categories and returns it.
func mustClassify(t *testing.T, rawMeta map[string]string, target uint8) MigrationCategory {
	t.Helper()
	category, _, _ := ClassifyMigrationObject(rawMeta, target)
	switch category {
	case CategoryV1SinglePut, CategoryV1Multipart, CategoryV2SinglePut,
		CategoryV2Multipart, CategoryAlreadyAtTarget, CategoryNonARMOR,
		CategoryMalformed, CategoryContradictory:
		return category
	default:
		t.Fatalf("unknown category %q", category)
		return ""
	}
}

// TestCountObjectsClassificationInventory pins the ownership split at the
// inventory seam: countObjects alone populates the static dimensions
// (source, size, fingerprint) and TotalObjects, leaves every outcome counter
// at zero, and re-derives the static dimensions from the listing on each
// run — replacing whatever static counts the loaded state carried while
// preserving its outcome counts.
func TestCountObjectsClassificationInventory(t *testing.T) {
	ctx := context.Background()
	mb := NewMockBackend()
	want, _ := buildClassificationInventory(t, mb)

	static := want
	static.OutcomeProcessed, static.OutcomeSkipped = 0, 0
	static.OutcomeFailed, static.OutcomeIntegrityFailed = 0, 0

	migrator := NewFormatMigrator(mb, "test-bucket", clsTestMEK(), "default", crypto.Version3, []string{"1", "2"}, nil)
	if err := migrator.initOrLoadState(ctx, false, 1); err != nil {
		t.Fatalf("failed to init state: %v", err)
	}
	if err := migrator.countObjects(ctx); err != nil {
		t.Fatalf("countObjects failed: %v", err)
	}

	state := migrator.GetState()
	if !reflect.DeepEqual(state.Classification, static) {
		t.Errorf("inventory classification wrong (outcomes must stay zero before any walk):\n got %+v\nwant %+v", state.Classification, static)
	}
	if state.TotalObjects != 5 {
		t.Errorf("TotalObjects = %d, want 5 migration candidates", state.TotalObjects)
	}

	// A second inventory pass replaces stale static counts — the listing is
	// the single source of truth for those dimensions — while outcome counts
	// loaded with the state are carried over untouched for the walk to keep
	// accumulating on.
	fm := migrator
	fm.stateMu.Lock()
	fm.state.Classification.V1SinglePut = 99
	fm.state.Classification.ByKeyFingerprint = map[string]int{"stale": 1}
	fm.state.Classification.OutcomeProcessed = 2
	fm.state.Classification.OutcomeSkipped = 1
	fm.stateMu.Unlock()

	if err := fm.countObjects(ctx); err != nil {
		t.Fatalf("second countObjects failed: %v", err)
	}
	state = fm.GetState()
	wantSecond := static
	wantSecond.OutcomeProcessed, wantSecond.OutcomeSkipped = 2, 1
	if !reflect.DeepEqual(state.Classification, wantSecond) {
		t.Errorf("second inventory did not reset static dims / preserve outcomes:\n got %+v\nwant %+v", state.Classification, wantSecond)
	}
}

// TestMigrateClassificationMatchesInventory is the accuracy acceptance: a
// walk over a mixed inventory must classify every walked object into the
// buckets its metadata dictates, the dimensions must balance, and the
// classification totals must agree with the legacy processed/skipped
// counters over the same walk.
func TestMigrateClassificationMatchesInventory(t *testing.T) {
	ctx := context.Background()
	mb := NewMockBackend()
	want, _ := buildClassificationInventory(t, mb)
	mek := clsTestMEK()

	migrator := NewFormatMigrator(mb, "test-bucket", mek, "default", crypto.Version3, []string{"1", "2"}, nil)
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	if !reflect.DeepEqual(result.Classification, want) {
		t.Errorf("classification does not match inventory:\n got %+v\nwant %+v", result.Classification, want)
	}

	if total := result.Classification.Total(); total != 10 {
		t.Errorf("Total() = %d, want 10 (every walked object classified exactly once)", total)
	}
	if !result.Classification.Balanced() {
		t.Errorf("classification dimensions do not balance: %+v", result.Classification)
	}

	// Cross-check against the legacy counters: attempted candidates are the
	// processed/failed/integrity outcomes, skipped objects are the skipped
	// outcome, and together they cover the walk.
	attempted := result.Classification.OutcomeProcessed + result.Classification.OutcomeFailed + result.Classification.OutcomeIntegrityFailed
	if attempted != result.ProcessedObjects {
		t.Errorf("attempted outcomes %d != ProcessedObjects %d", attempted, result.ProcessedObjects)
	}
	if result.Classification.OutcomeSkipped != result.SkippedObjects {
		t.Errorf("skipped outcome %d != SkippedObjects %d", result.Classification.OutcomeSkipped, result.SkippedObjects)
	}
	if result.Classification.Total() != result.ProcessedObjects+result.SkippedObjects {
		t.Errorf("Total() %d != ProcessedObjects+SkippedObjects %d", result.Classification.Total(), result.ProcessedObjects+result.SkippedObjects)
	}
	if result.FailedObjects != 4 {
		t.Errorf("FailedObjects = %d, want 4 (two sidecarless multiparts, one contradictory, one integrity)", result.FailedObjects)
	}
	if result.TotalObjects != 5 {
		t.Errorf("TotalObjects = %d, want 5 migration candidates", result.TotalObjects)
	}

	// The live state and the persisted state JSON carry the same
	// machine-readable report.
	state := migrator.GetState()
	if !reflect.DeepEqual(state.Classification, want) {
		t.Errorf("GetState().Classification does not match inventory: %+v", state.Classification)
	}
	stateObj, ok := mb.objects[".armor/migration-state.json"]
	if !ok {
		t.Fatal("migration state was not persisted")
	}
	var persisted MigrationState
	if err := json.Unmarshal(stateObj.Data, &persisted); err != nil {
		t.Fatalf("persisted state does not parse: %v", err)
	}
	if !reflect.DeepEqual(persisted.Classification, want) {
		t.Errorf("persisted classification does not match inventory:\n got %+v\nwant %+v", persisted.Classification, want)
	}
}

// TestMigrateClassificationSummaryLoggedAndReadable exercises the
// human-readable rendering over a real walk: the report includes every
// dimension with the inventory's counts.
func TestMigrateClassificationSummaryLoggedAndReadable(t *testing.T) {
	ctx := context.Background()
	mb := NewMockBackend()
	// The inventory is built for the walk; the expected counts are asserted
	// literally below so the assertions double as documentation.
	buildClassificationInventory(t, mb)

	migrator := NewFormatMigrator(mb, "test-bucket", clsTestMEK(), "default", crypto.Version3, []string{"1", "2"}, nil)
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	summary := result.Classification.Summary()
	for _, want := range []string{
		"v1-single=1", "v1-multipart=1", "v2-single=1", "v2-multipart=1",
		"v3=3", "non-armor=1", "malformed=1", "contradictory=1",
		"(total 10)",
		"<1MB=6", "1MB-10MB=1", "100MB-1GB=1", "1GB-10GB=1", ">10GB=1",
		clsFingerprintA + "=3", "legacy=2",
		"processed=1", "skipped=5", "failed=3", "integrity-failed=1",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("Summary() missing %q:\n%s", want, summary)
		}
	}
}

// TestMigrateClassificationDryRunDoesNotLeak checks that the classification
// a dry run persists never leaks into a subsequent live run: the live report
// counts every object exactly once, not doubled.
func TestMigrateClassificationDryRunDoesNotLeak(t *testing.T) {
	ctx := context.Background()
	mb := NewMockBackend()
	want, _ := buildClassificationInventory(t, mb)

	migrator := NewFormatMigrator(mb, "test-bucket", clsTestMEK(), "default", crypto.Version3, []string{"1", "2"}, nil)

	// Dry run first: classifies the whole inventory and completes.
	dry, err := migrator.Migrate(ctx, true, 1)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if !reflect.DeepEqual(dry.Classification, want) {
		t.Errorf("dry-run classification wrong:\n got %+v\nwant %+v", dry.Classification, want)
	}

	// Live run over the same bucket: must report the same classification,
	// not the dry run's counts plus its own.
	live, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("live migration failed: %v", err)
	}
	if !reflect.DeepEqual(live.Classification, want) {
		t.Errorf("live classification polluted by dry run:\n got %+v\nwant %+v", live.Classification, want)
	}
}

// TestMigrateClassificationDryRunPersistsStateJSON is the dry-run acceptance
// for the persisted report: a dry run over the mixed eight-category fixture
// writes a migration state whose classification member carries every source
// category under its JSON tag — the state file an operator or the CLI reads
// back shows the whole inventory, not just the walk's outcome counts.
func TestMigrateClassificationDryRunPersistsStateJSON(t *testing.T) {
	ctx := context.Background()
	mb := NewMockBackend()
	want, _ := buildClassificationInventory(t, mb)

	migrator := NewFormatMigrator(mb, "test-bucket", clsTestMEK(), "default", crypto.Version3, []string{"1", "2"}, nil)
	dry, err := migrator.Migrate(ctx, true, 1)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	stateObj, ok := mb.objects[".armor/migration-state.json"]
	if !ok {
		t.Fatal("dry run did not persist migration state")
	}
	var raw struct {
		DryRun         bool                   `json:"dry_run"`
		Classification map[string]interface{} `json:"classification"`
	}
	if err := json.Unmarshal(stateObj.Data, &raw); err != nil {
		t.Fatalf("persisted state does not parse: %v", err)
	}
	if !raw.DryRun {
		t.Error("persisted state does not record dry_run=true")
	}

	// Every one of the eight source categories, by its persisted JSON tag.
	for key, wantCount := range map[string]float64{
		"v1_single_put": float64(want.V1SinglePut),
		"v1_multipart":  float64(want.V1Multipart),
		"v2_single_put": float64(want.V2SinglePut),
		"v2_multipart":  float64(want.V2Multipart),
		"v3":            float64(want.V3),
		"non_armor":     float64(want.NonARMOR),
		"malformed":     float64(want.Malformed),
		"contradictory": float64(want.Contradictory),
	} {
		got, ok := raw.Classification[key]
		if !ok {
			t.Errorf("persisted classification missing category key %q:\n%s", key, stateObj.Data)
			continue
		}
		if got != wantCount {
			t.Errorf("persisted classification %s = %v, want %v", key, got, wantCount)
		}
	}

	// The same document decodes back into the state struct unchanged: the
	// state file, GetState() and the returned result all carry the same
	// report the run produced.
	var persisted MigrationState
	if err := json.Unmarshal(stateObj.Data, &persisted); err != nil {
		t.Fatalf("persisted state does not parse into MigrationState: %v", err)
	}
	if !reflect.DeepEqual(persisted.Classification, want) {
		t.Errorf("persisted dry-run classification does not match inventory:\n got %+v\nwant %+v", persisted.Classification, want)
	}
	if state := migrator.GetState(); !reflect.DeepEqual(state.Classification, want) {
		t.Errorf("GetState() dry-run classification does not match inventory:\n got %+v\nwant %+v", state.Classification, want)
	}
	if !reflect.DeepEqual(dry.Classification, want) {
		t.Errorf("dry-run result classification does not match inventory:\n got %+v\nwant %+v", dry.Classification, want)
	}
}

// TestMigrateClassificationResumeAccumulates checks the resume semantics:
// the inventory pass re-derives the static dimensions from the full listing
// (replacing the partial counts the loaded state carried), while the walk
// records outcomes only for objects past the cursor, accumulating on top of
// the outcome counts loaded with the state — so the final report still
// covers the whole inventory exactly once.
func TestMigrateClassificationResumeAccumulates(t *testing.T) {
	ctx := context.Background()
	mb := NewMockBackend()
	want, _ := buildClassificationInventory(t, mb)
	mek := clsTestMEK()

	// Seed the state a first (interrupted) live run would have left behind:
	// cursor parked after e (objects a..e already walked and classified),
	// counters matching what that partial walk produced.
	seeded := ObjectClassification{
		V1SinglePut: 1, V2Multipart: 1, V3: 2, NonARMOR: 1, // d, e, b+c, a
		SizeLessThan1MB: 3, Size1MBTo10MB: 1, Size100MBTo1GB: 1, // a,d,e / b / c
		ByKeyFingerprint: map[string]int{
			clsFingerprintA:            2, // b, c
			crypto.MEKFingerprint(mek): 1, // e
			"legacy":                   1, // d
		},
		OutcomeProcessed: 1, OutcomeSkipped: 3, OutcomeFailed: 1, // d / a,b,c / e
	}
	state := MigrationState{
		ID:                  "format-migration-interrupted",
		StartTime:           time.Now().Add(-time.Minute),
		LastUpdated:         time.Now().Add(-time.Second),
		Status:              "in_progress",
		TotalObjects:        5,
		ProcessedObjects:    2,
		SkippedObjects:      3,
		FailedObjects:       1,
		LastKey:             "e-v2-multipart.bin",
		IncludeVersions:     []string{"1", "2"},
		CurrentWriteVersion: crypto.Version3,
		DryRun:              false,
		Concurrency:         1,
		Classification:      seeded,
	}
	stateData, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal seeded state: %v", err)
	}
	mb.objects[".armor/migration-state.json"] = &MockObject{
		Data:     stateData,
		Metadata: map[string]string{"Content-Type": "application/json"},
	}

	migrator := NewFormatMigrator(mb, "test-bucket", mek, "default", crypto.Version3, []string{"1", "2"}, nil)
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("resumed migration failed: %v", err)
	}

	if !reflect.DeepEqual(result.Classification, want) {
		t.Errorf("resumed classification does not cover the inventory exactly once:\n got %+v\nwant %+v", result.Classification, want)
	}
	if total := result.Classification.Total(); total != 10 {
		t.Errorf("Total() = %d, want 10 (resume must not double-count the cursor-passed objects)", total)
	}
}
