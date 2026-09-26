// cmd_verify_report_contract_test.go pins the 'armor verify' report and
// exit-code contract against a mixed inventory: single-PUT and multipart
// objects, legacy (v1/v2) and v3 formats, successes and both failure classes
// in one run. The earlier mixed tests (TestRunVerificationReportRows, the
// binary contract's mixed_inventory subtest) are single-PUT v2 only, and the
// multipart tests each verify a single object — nothing exercised a report
// that mixes shapes and formats, which is exactly where a dropped or
// double-counted row would hide.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/crypto"
)

// buildMixedFormatInventory puts one healthy and — in the two families with
// their own in-body failure mode — one damaged object of every format/shape
// verify handles into the mock bucket. Keys sort in the order the names
// suggest: a-legacy-single, b-v3-single, c-legacy-multipart and d-v3-multipart
// are OK; e-legacy-corrupt and f-v3-corrupt are CORRUPTED; g-plain (never
// ARMOR) is ERROR. Returns the key list to verify.
func buildMixedFormatInventory(t *testing.T, mock *MockB2Backend, mek []byte) []string {
	t.Helper()
	ctx := context.Background()
	now := time.Now()

	legacyOK := createValidARMORObject(t, mek, "a-legacy-single", []byte("legacy v2 single-PUT body"))
	mock.PutTestObject(ctx, "test-bucket", "a-legacy-single", legacyOK.data, legacyOK.metadata, true, now)

	v3OK := createValidV3SinglePUTObject(t, mek, "b-v3-single", []byte("v3 single-PUT body"))
	mock.PutTestObject(ctx, "test-bucket", "b-v3-single", v3OK.data, v3OK.metadata, true, now)

	legacyMP := createV2MultipartFixture(t, mek, "c-legacy-multipart", bytes.Repeat([]byte("legacy multipart "), 128))
	legacyMP.put(ctx, mock, "test-bucket", now)

	v3MP := createV3MultipartFixture(t, mek, "d-v3-multipart", bytes.Repeat([]byte("v3 multipart walk-"), 200), true)
	v3MP.put(ctx, mock, "test-bucket", now)

	legacyBad := createValidARMORObject(t, mek, "e-legacy-corrupt", []byte("this legacy body will be damaged"))
	envelope, err := crypto.DecodeHeader(legacyBad.data)
	if err != nil {
		t.Fatalf("decode legacy envelope: %v", err)
	}
	hmacTableSize := int(crypto.ComputeBlockCount(int64(envelope.PlaintextSize), envelope.BlockSize())) * crypto.HMACSize
	ciphertext := legacyBad.data[crypto.HeaderSize : len(legacyBad.data)-hmacTableSize]
	ciphertext[len(ciphertext)/2] ^= 0xFF
	mock.PutTestObject(ctx, "test-bucket", "e-legacy-corrupt", legacyBad.data, legacyBad.metadata, true, now)

	v3Bad := createV3MultipartFixture(t, mek, "f-v3-corrupt", bytes.Repeat([]byte("corrupt the last part-"), 180), true)
	v3Bad.corruptPartCiphertext(t, len(v3Bad.partOffsets)-1)
	v3Bad.put(ctx, mock, "test-bucket", now)

	mock.PutTestObject(ctx, "test-bucket", "g-plain", []byte("plain bytes, never armored"), map[string]string{}, false, now)

	return []string{
		"a-legacy-single", "b-v3-single", "c-legacy-multipart", "d-v3-multipart",
		"e-legacy-corrupt", "f-v3-corrupt", "g-plain",
	}
}

// TestVerifyMixedFormatInventoryReport pins the report contract against the
// full matrix: exactly one row per object (none dropped, none duplicated),
// rows sorted by key, the right verdict in every format/shape family, every
// failing row carrying a diagnosable error, counters agreeing with the rows,
// and any failure making the process exit non-zero.
func TestVerifyMixedFormatInventoryReport(t *testing.T) {
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	origBucket := bucketFlag
	bucketFlag = "test-bucket"
	t.Cleanup(func() { bucketFlag = origBucket })

	keys := buildMixedFormatInventory(t, mock, mek)

	wantStatus := map[string]string{
		"a-legacy-single":    "OK",
		"b-v3-single":        "OK",
		"c-legacy-multipart": "OK",
		"d-v3-multipart":     "OK",
		"e-legacy-corrupt":   "CORRUPTED",
		"f-v3-corrupt":       "CORRUPTED",
		"g-plain":            "ERROR",
	}

	report := runVerification(context.Background(), mock, newVerifyKeySource(mek, nil), keys, time.Time{})

	if report.TotalObjects != len(keys) || len(report.Results) != len(keys) {
		t.Fatalf("total=%d rows=%d, want %d/%d (one row per object, none dropped)",
			report.TotalObjects, len(report.Results), len(keys), len(keys))
	}

	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	seen := map[string]int{}
	for i, row := range report.Results {
		seen[row.Key]++
		if row.Key != sorted[i] {
			t.Errorf("row %d key = %q, want %q (rows must be sorted by key so two runs diff cleanly)", i, row.Key, sorted[i])
		}
	}
	for _, key := range keys {
		if seen[key] != 1 {
			t.Errorf("key %q has %d report rows, want exactly 1", key, seen[key])
		}
	}

	for _, row := range report.Results {
		if want := wantStatus[row.Key]; row.Status != want {
			t.Errorf("%s status = %s (error: %q), want %s", row.Key, row.Status, row.Error, want)
		}
		if row.Status != "OK" && row.Error == "" {
			t.Errorf("failing row %q carries an empty error field", row.Key)
		}
		if row.Bucket != bucketFlag {
			t.Errorf("%s row bucket = %q, want %q", row.Key, row.Bucket, bucketFlag)
		}
	}

	if report.OKCount != 4 || report.CorruptedCount != 2 || report.ErrorCount != 1 {
		t.Errorf("counts = OK:%d CORRUPTED:%d ERROR:%d, want 4/2/1",
			report.OKCount, report.CorruptedCount, report.ErrorCount)
	}
	if code := verificationExitCode(report); code != 1 {
		t.Errorf("exit code = %d, want 1: any CORRUPTED or ERROR object must fail the run", code)
	}
}

// TestVerifyAllOKMixedFormatsExitsZero pins the other half of the exit-code
// contract: a healthy inventory spanning all four format/shape cells — legacy
// and v3, single-PUT and multipart — produces an all-OK report and exits 0.
// A verifier that spuriously failed any one cell fails here.
func TestVerifyAllOKMixedFormatsExitsZero(t *testing.T) {
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()
	ctx := context.Background()
	now := time.Now()

	legacy := createValidARMORObject(t, mek, "a-legacy-single", []byte("legacy v2 single-PUT body"))
	mock.PutTestObject(ctx, "test-bucket", "a-legacy-single", legacy.data, legacy.metadata, true, now)

	v3 := createValidV3SinglePUTObject(t, mek, "b-v3-single", []byte("v3 single-PUT body"))
	mock.PutTestObject(ctx, "test-bucket", "b-v3-single", v3.data, v3.metadata, true, now)

	legacyMP := createV2MultipartFixture(t, mek, "c-legacy-multipart", bytes.Repeat([]byte("legacy multipart "), 128))
	legacyMP.put(ctx, mock, "test-bucket", now)

	v3MP := createV3MultipartFixture(t, mek, "d-v3-multipart", bytes.Repeat([]byte("v3 multipart walk-"), 200), true)
	v3MP.put(ctx, mock, "test-bucket", now)

	keys := []string{"a-legacy-single", "b-v3-single", "c-legacy-multipart", "d-v3-multipart"}
	report := runVerification(ctx, mock, newVerifyKeySource(mek, nil), keys, time.Time{})

	if report.OKCount != len(keys) || report.CorruptedCount != 0 || report.ErrorCount != 0 {
		t.Errorf("counts = OK:%d CORRUPTED:%d ERROR:%d, want %d/0/0 — every healthy format/shape must verify OK",
			report.OKCount, report.CorruptedCount, report.ErrorCount, len(keys))
		for _, row := range report.Results {
			if row.Status != "OK" {
				t.Errorf("  %s = %s (%s)", row.Key, row.Status, row.Error)
			}
		}
	}
	if len(report.Results) != len(keys) {
		t.Errorf("rows = %d, want %d", len(report.Results), len(keys))
	}
	if code := verificationExitCode(report); code != 0 {
		t.Errorf("exit code = %d, want 0 for an all-OK inventory", code)
	}
}

// TestVerifyReportWireFieldNames pins the JSON field names of the report and
// its rows — the interface every -output consumer parses. A renamed or
// dropped field breaks downstream tooling silently, so both key sets are
// asserted exactly: no missing field and no unexpected one, through the real
// writeReport file path rather than a test-side marshaler.
func TestVerifyReportWireFieldNames(t *testing.T) {
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	origBucket, origPrefix := bucketFlag, prefixFlag
	bucketFlag, prefixFlag = "wire-bucket", "weekly-scan/"
	t.Cleanup(func() { bucketFlag, prefixFlag = origBucket, origPrefix })

	ctx := context.Background()
	now := time.Now()

	okObj := createValidV3SinglePUTObject(t, mek, "b-ok", []byte("healthy v3 body"))
	mock.PutTestObject(ctx, bucketFlag, "b-ok", okObj.data, okObj.metadata, true, now)

	badObj := createValidARMORObject(t, mek, "a-corrupt", []byte("damaged legacy body"))
	envelope, err := crypto.DecodeHeader(badObj.data)
	if err != nil {
		t.Fatalf("decode legacy envelope: %v", err)
	}
	hmacTableSize := int(crypto.ComputeBlockCount(int64(envelope.PlaintextSize), envelope.BlockSize())) * crypto.HMACSize
	ciphertext := badObj.data[crypto.HeaderSize : len(badObj.data)-hmacTableSize]
	ciphertext[len(ciphertext)/2] ^= 0xFF
	mock.PutTestObject(ctx, bucketFlag, "a-corrupt", badObj.data, badObj.metadata, true, now)

	report := runVerification(ctx, mock, newVerifyKeySource(mek, nil), []string{"a-corrupt", "b-ok"}, time.Time{})

	outPath := filepath.Join(t.TempDir(), "report.json")
	if err := writeReport(report, outPath); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read -output report: %v", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("report is not JSON: %v\n%s", err, data)
	}
	assertExactJSONKeys(t, "report envelope", doc, []string{
		"bucket", "prefix", "total_objects", "ok_count", "corrupted_count",
		"error_count", "quick_mode", "verification_date", "duration_seconds", "results",
	})

	// Rows sort by key: the CORRUPTED legacy row first, the OK v3 row second.
	// A failing row carries error AND details; an OK row carries details and
	// omits the empty error field.
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(doc["results"], &rows); err != nil {
		t.Fatalf("results is not a JSON array of objects: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("results rows = %d, want 2", len(rows))
	}
	assertExactJSONKeys(t, "CORRUPTED row", rows[0], []string{
		"bucket", "key", "status", "error", "details", "size_bytes", "modification_time", "duration_seconds",
	})
	assertExactJSONKeys(t, "OK row", rows[1], []string{
		"bucket", "key", "status", "details", "size_bytes", "modification_time", "duration_seconds",
	})

	// The stable fields carry real values, not just the right names.
	var decoded []ObjectVerificationResult
	if err := json.Unmarshal(doc["results"], &decoded); err != nil {
		t.Fatalf("rows do not decode into ObjectVerificationResult: %v", err)
	}
	corrupted, healthy := decoded[0], decoded[1]
	if corrupted.Key != "a-corrupt" || corrupted.Status != "CORRUPTED" {
		t.Errorf("first row = %s/%s, want a-corrupt/CORRUPTED", corrupted.Key, corrupted.Status)
	}
	if healthy.Key != "b-ok" || healthy.Status != "OK" {
		t.Errorf("second row = %s/%s, want b-ok/OK", healthy.Key, healthy.Status)
	}
	if corrupted.SizeBytes == 0 || healthy.SizeBytes == 0 {
		t.Errorf("size_bytes not populated: corrupt=%d ok=%d", corrupted.SizeBytes, healthy.SizeBytes)
	}
	if corrupted.ModTime.IsZero() || healthy.ModTime.IsZero() {
		t.Errorf("modification_time not populated")
	}
}

// assertExactJSONKeys fails when got carries any key outside want or is
// missing any key from want — an exact field-name match in both directions.
func assertExactJSONKeys(t *testing.T, label string, got map[string]json.RawMessage, want []string) {
	t.Helper()
	wantSet := make(map[string]bool, len(want))
	for _, k := range want {
		wantSet[k] = true
		if _, ok := got[k]; !ok {
			t.Errorf("%s: JSON missing field %q", label, k)
		}
	}
	for k := range got {
		if !wantSet[k] {
			t.Errorf("%s: unexpected JSON field %q (field set drifted from the documented report contract)", label, k)
		}
	}
}
