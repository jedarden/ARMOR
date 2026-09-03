package handlers_test

// Tests for the operator repair paths in manifest_repair.go: re-stamping a
// stale manifest (RepairManifest), quarantining one so readers get a
// definitive non-retryable error instead of a retryable 500
// (QuarantineManifest), and lifting a quarantine (ReleaseManifest).
//
// The stale-manifest condition and its retryable 500 come from the ADR-016
// ciphertext freshness gate exercised in
// multipart_manifest_freshness_test.go; these tests treat that gate as the
// starting point and verify the operator's way out of it.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	handlers "github.com/jedarden/armor/internal/server/handlers"
)

// repairHarness stores one ARMOR-encrypted object via a real PUT and hands
// back helpers to write manifests for it and read it through the full
// GetObject path.
type repairHarness struct {
	t         *testing.T
	h         *handlers.Handlers
	mb        *mockBackend
	bucket    string
	key       string
	plaintext []byte
	armorMeta map[string]string
}

func newRepairHarness(t *testing.T) *repairHarness {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	bucket, key := "repair-bucket", "repair-key"
	plaintext := []byte("operator repair path regression plaintext")

	putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, bytes.NewReader(plaintext))
	putReq.Header.Set("Content-Type", "text/plain")
	putW := httptest.NewRecorder()
	h.HandleRoot(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d, body %s", putW.Code, putW.Body.String())
	}

	mb.mu.Lock()
	armorMeta := make(map[string]string, len(mb.meta[bucket+"/"+key]))
	for k, v := range mb.meta[bucket+"/"+key] {
		armorMeta[k] = v
	}
	mb.mu.Unlock()

	return &repairHarness{t: t, h: h, mb: mb, bucket: bucket, key: key, plaintext: plaintext, armorMeta: armorMeta}
}

// writeStaleManifest installs a manifest whose completedAt precedes the
// ciphertext's LastModified — the overwrite ordering the freshness gate
// rejects with a retryable 500.
func (rh *repairHarness) writeStaleManifest() {
	rh.t.Helper()
	// The mock stamps ciphertext LastModified at write time (now); a
	// completedAt two minutes back is therefore strictly older.
	rh.writeManifest(time.Now().Add(-2 * time.Minute))
}

// writeFreshManifest installs a manifest in the normal completion ordering:
// the ciphertext was assembled before the manifest was completed.
func (rh *repairHarness) writeFreshManifest() {
	rh.t.Helper()
	rh.writeManifest(time.Now().Add(2 * time.Minute))
}

func (rh *repairHarness) writeManifest(completedAt time.Time) {
	rh.t.Helper()
	meta := make(map[string]string, len(rh.armorMeta)+2)
	for k, v := range rh.armorMeta {
		meta[k] = v
	}
	meta["x-amz-meta-armor-completed-at"] = completedAt.UTC().Format(time.RFC3339)
	meta["x-amz-meta-armor-ciphertext-ref"] = rh.key
	meta["Content-Type"] = "application/x-armor-manifest+json"

	manifestJSON, err := json.Marshal(backend.ManifestBody{
		CiphertextObject: rh.key,
		UploadID:         "upload-repair-test",
		CompletedAt:      completedAt.UTC().Format(time.RFC3339),
		Metadata:         meta,
	})
	if err != nil {
		rh.t.Fatalf("failed to marshal manifest body: %v", err)
	}
	if err := rh.mb.Put(context.Background(), rh.bucket, rh.key+".armor-manifest", bytes.NewReader(manifestJSON), int64(len(manifestJSON)), meta); err != nil {
		rh.t.Fatalf("failed to write manifest: %v", err)
	}
}

// manifestMeta returns the metadata currently stored on the manifest object.
func (rh *repairHarness) manifestMeta() map[string]string {
	rh.t.Helper()
	rh.mb.mu.Lock()
	defer rh.mb.mu.Unlock()
	meta := rh.mb.meta[rh.bucket+"/"+rh.key+".armor-manifest"]
	if meta == nil {
		rh.t.Fatal("manifest object not found in backend")
	}
	out := make(map[string]string, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

// get reads the object through the full GetObject path and returns the status
// and response body.
func (rh *repairHarness) get() (int, string) {
	req := httptest.NewRequest(http.MethodGet, "/"+rh.bucket+"/"+rh.key, nil)
	w := httptest.NewRecorder()
	rh.h.HandleRoot(w, req)
	return w.Code, w.Body.String()
}

func (rh *repairHarness) requireGETStatus(wantCode int, wantInBody string) {
	rh.t.Helper()
	code, body := rh.get()
	if code != wantCode {
		rh.t.Fatalf("GetObject returned %d, want %d: %s", code, wantCode, body)
	}
	if wantInBody != "" && !strings.Contains(body, wantInBody) {
		rh.t.Fatalf("GetObject body %q does not contain %q", body, wantInBody)
	}
}

// TestRepairManifestRestampsAndRestoresReads covers the repair happy path: a
// manifest the freshness gate rejects with a retryable 500 is re-stamped to
// the ciphertext's LastModified, after which the same gate passes and the
// object reads again with its original plaintext.
func TestRepairManifestRestampsAndRestoresReads(t *testing.T) {
	rh := newRepairHarness(t)
	rh.writeStaleManifest()
	rh.requireGETStatus(http.StatusInternalServerError, "Stale manifest")

	inspected, err := rh.h.InspectManifest(context.Background(), rh.bucket, rh.key)
	if err != nil {
		t.Fatalf("InspectManifest: %v", err)
	}
	if inspected.FreshnessChecked && inspected.Fresh {
		t.Fatal("inspect reports a stale manifest as fresh")
	}

	status, err := rh.h.RepairManifest(context.Background(), rh.bucket, rh.key)
	if err != nil {
		t.Fatalf("RepairManifest: %v", err)
	}
	if !status.Fresh {
		t.Errorf("repaired manifest reports Fresh=false (verify error: %q)", status.VerifyError)
	}
	if status.OriginalCompletedAt == "" {
		t.Error("repair did not preserve the original completion timestamp")
	}
	if status.RepairedAt == "" {
		t.Error("repair did not stamp a repaired-at timestamp")
	}
	if status.CompletedAt != status.CiphertextModified {
		t.Errorf("re-stamped completedAt %q does not match ciphertext LastModified %q", status.CompletedAt, status.CiphertextModified)
	}

	// Both channels a reader or debugging tool can look at must carry the
	// re-stamped timestamp: the header map the freshness gate reads and the
	// JSON body.
	meta := rh.manifestMeta()
	if meta["x-amz-meta-armor-completed-at"] != status.CompletedAt {
		t.Errorf("manifest header completedAt %q does not match repaired value %q", meta["x-amz-meta-armor-completed-at"], status.CompletedAt)
	}
	var body backend.ManifestBody
	manifestBytes := func() []byte {
		rh.mb.mu.Lock()
		defer rh.mb.mu.Unlock()
		return rh.mb.objects[rh.bucket+"/"+rh.key+".armor-manifest"]
	}()
	if err := json.Unmarshal(manifestBytes, &body); err != nil {
		t.Fatalf("decode rewritten manifest body: %v", err)
	}
	if body.CompletedAt != status.CompletedAt {
		t.Errorf("manifest JSON completedAt %q does not match repaired value %q", body.CompletedAt, status.CompletedAt)
	}

	// The gate that produced the retryable 500 now passes and the plaintext
	// survives the round trip untouched.
	code, got := rh.get()
	if code != http.StatusOK {
		t.Fatalf("GetObject after repair returned %d: %s", code, got)
	}
	if !bytes.Equal([]byte(got), rh.plaintext) {
		// The GET response is the decrypted plaintext; the XML error path
		// already failed the status check above.
		t.Errorf("GetObject content mismatch after repair: got %q, want %q", got, string(rh.plaintext))
	}
}

// TestQuarantineMakesReadsFailNonRetryably verifies the second operator path:
// a quarantined object returns 403 AccessDenied — a status clients like
// litestream do not retry — rather than the retryable 500 a stale manifest
// produces. Quarantine is checked before freshness, so it is definitive even
// for a manifest the gate would reject.
func TestQuarantineMakesReadsFailNonRetryably(t *testing.T) {
	t.Run("stale manifest gets 403 instead of 500", func(t *testing.T) {
		rh := newRepairHarness(t)
		rh.writeStaleManifest()
		rh.requireGETStatus(http.StatusInternalServerError, "Stale manifest")

		if _, err := rh.h.QuarantineManifest(context.Background(), rh.bucket, rh.key, "stale manifest, ciphertext suspect"); err != nil {
			t.Fatalf("QuarantineManifest: %v", err)
		}
		rh.requireGETStatus(http.StatusForbidden, "quarantined by the operator")
		rh.requireGETStatus(http.StatusForbidden, "stale manifest, ciphertext suspect")
	})

	t.Run("fresh manifest is quarantined too", func(t *testing.T) {
		rh := newRepairHarness(t)
		rh.writeFreshManifest()
		rh.requireGETStatus(http.StatusOK, "")

		if _, err := rh.h.QuarantineManifest(context.Background(), rh.bucket, rh.key, "pending investigation"); err != nil {
			t.Fatalf("QuarantineManifest: %v", err)
		}
		rh.requireGETStatus(http.StatusForbidden, "pending investigation")

		inspected, err := rh.h.InspectManifest(context.Background(), rh.bucket, rh.key)
		if err != nil {
			t.Fatalf("InspectManifest: %v", err)
		}
		if !inspected.Quarantined || inspected.QuarantineReason != "pending investigation" {
			t.Errorf("inspect reports quarantined=%v reason=%q, want true/\"pending investigation\"", inspected.Quarantined, inspected.QuarantineReason)
		}
	})
}

// TestReleaseManifestLiftsQuarantine verifies that releasing restores
// readability and that release is idempotent, so an operator script can
// release unconditionally after a repair.
func TestReleaseManifestLiftsQuarantine(t *testing.T) {
	rh := newRepairHarness(t)
	rh.writeFreshManifest()

	if _, err := rh.h.QuarantineManifest(context.Background(), rh.bucket, rh.key, "investigating"); err != nil {
		t.Fatalf("QuarantineManifest: %v", err)
	}
	rh.requireGETStatus(http.StatusForbidden, "quarantined by the operator")

	status, err := rh.h.ReleaseManifest(context.Background(), rh.bucket, rh.key)
	if err != nil {
		t.Fatalf("ReleaseManifest: %v", err)
	}
	if status.Quarantined {
		t.Error("released manifest still reports quarantined")
	}
	rh.requireGETStatus(http.StatusOK, "")

	// Idempotent: releasing an already-released manifest succeeds.
	if _, err := rh.h.ReleaseManifest(context.Background(), rh.bucket, rh.key); err != nil {
		t.Fatalf("second ReleaseManifest: %v", err)
	}
}

// TestRepairLiftsQuarantine verifies that repairing a stale-and-quarantined
// manifest lifts the quarantine — a repair that left the object unreadable
// could never be the intent, and the operator can re-quarantine afterwards.
func TestRepairLiftsQuarantine(t *testing.T) {
	rh := newRepairHarness(t)
	rh.writeStaleManifest()

	if _, err := rh.h.QuarantineManifest(context.Background(), rh.bucket, rh.key, "stale"); err != nil {
		t.Fatalf("QuarantineManifest: %v", err)
	}
	rh.requireGETStatus(http.StatusForbidden, "quarantined by the operator")

	if _, err := rh.h.RepairManifest(context.Background(), rh.bucket, rh.key); err != nil {
		t.Fatalf("RepairManifest: %v", err)
	}

	meta := rh.manifestMeta()
	if meta["x-amz-meta-armor-quarantined"] != "" {
		t.Error("repair left the quarantine marker in place")
	}
	rh.requireGETStatus(http.StatusOK, "")
}

// TestRepairPreservesOriginalCompletedAt verifies the provenance chain across
// repeated repairs: the first repair records the timestamp the manifest was
// written with, and later repairs leave that record intact.
func TestRepairPreservesOriginalCompletedAt(t *testing.T) {
	rh := newRepairHarness(t)
	rh.writeStaleManifest()

	first, err := rh.h.RepairManifest(context.Background(), rh.bucket, rh.key)
	if err != nil {
		t.Fatalf("first RepairManifest: %v", err)
	}
	original := first.OriginalCompletedAt
	if original == "" {
		t.Fatal("first repair recorded no original completion timestamp")
	}

	// A second repair (e.g. the ciphertext changed again) must not overwrite
	// the original record with the first repair's stamp.
	second, err := rh.h.RepairManifest(context.Background(), rh.bucket, rh.key)
	if err != nil {
		t.Fatalf("second RepairManifest: %v", err)
	}
	if second.OriginalCompletedAt != original {
		t.Errorf("second repair changed original completedAt from %q to %q", original, second.OriginalCompletedAt)
	}
}

// TestRepairRefusals covers the conditions a repair refuses rather than
// inventing state for.
func TestRepairRefusals(t *testing.T) {
	t.Run("no manifest", func(t *testing.T) {
		rh := newRepairHarness(t)
		if _, err := rh.h.RepairManifest(context.Background(), rh.bucket, rh.key); err != handlers.ErrManifestNotFound {
			t.Errorf("repair without a manifest: err = %v, want ErrManifestNotFound", err)
		}
		if _, err := rh.h.InspectManifest(context.Background(), rh.bucket, rh.key); err != handlers.ErrManifestNotFound {
			t.Errorf("inspect without a manifest: err = %v, want ErrManifestNotFound", err)
		}
	})

	t.Run("manifest without ciphertext ref", func(t *testing.T) {
		rh := newRepairHarness(t)
		rh.writeManifestAt(time.Now(), "", true)
		if _, err := rh.h.RepairManifest(context.Background(), rh.bucket, rh.key); err != handlers.ErrNoCiphertextRef {
			t.Errorf("repair without ciphertext ref: err = %v, want ErrNoCiphertextRef", err)
		}
	})

	t.Run("manifest without completion timestamp", func(t *testing.T) {
		rh := newRepairHarness(t)
		rh.writeManifestAt(time.Now(), rh.key, false)
		if _, err := rh.h.RepairManifest(context.Background(), rh.bucket, rh.key); err != handlers.ErrNoCompletedAt {
			t.Errorf("repair without completedAt: err = %v, want ErrNoCompletedAt", err)
		}
	})

	t.Run("dangling ciphertext ref", func(t *testing.T) {
		rh := newRepairHarness(t)
		rh.writeManifestAt(time.Now(), "missing-ciphertext-object", true)
		_, err := rh.h.RepairManifest(context.Background(), rh.bucket, rh.key)
		if err == nil {
			t.Fatal("repair with a dangling ciphertext ref succeeded")
		}
		if !strings.Contains(err.Error(), "missing-ciphertext-object") {
			t.Errorf("dangling-ref error %q does not name the missing object", err)
		}
		if strings.Contains(err.Error(), "no manifest found") {
			t.Errorf("dangling ciphertext ref misreported as missing manifest: %v", err)
		}
	})
}

// TestQuarantineReasonValidation verifies the operator-supplied reason is
// bounded and header-safe: backend metadata travels as HTTP header values.
func TestQuarantineReasonValidation(t *testing.T) {
	rh := newRepairHarness(t)
	rh.writeFreshManifest()

	if _, err := rh.h.QuarantineManifest(context.Background(), rh.bucket, rh.key, "not ascii \x80"); err == nil {
		t.Error("non-printable reason accepted")
	}
	if _, err := rh.h.QuarantineManifest(context.Background(), rh.bucket, rh.key, strings.Repeat("x", 257)); err == nil {
		t.Error("oversized reason accepted")
	}

	// An empty reason is defaulted rather than rejected, so the minimal
	// quarantine call still records something.
	status, err := rh.h.QuarantineManifest(context.Background(), rh.bucket, rh.key, "")
	if err != nil {
		t.Fatalf("quarantine with empty reason: %v", err)
	}
	if status.QuarantineReason == "" {
		t.Error("empty reason was not defaulted")
	}
	rh.requireGETStatus(http.StatusForbidden, "quarantined by the operator")
}

// writeManifestAt writes a manifest with explicit control over the ciphertext
// ref and whether the completion timestamp is present at all, for the refusal
// cases.
func (rh *repairHarness) writeManifestAt(completedAt time.Time, ciphertextRef string, withCompletedAt bool) {
	rh.t.Helper()
	meta := make(map[string]string, len(rh.armorMeta)+2)
	for k, v := range rh.armorMeta {
		meta[k] = v
	}
	if withCompletedAt {
		meta["x-amz-meta-armor-completed-at"] = completedAt.UTC().Format(time.RFC3339)
	}
	if ciphertextRef != "" {
		meta["x-amz-meta-armor-ciphertext-ref"] = ciphertextRef
	}
	meta["Content-Type"] = "application/x-armor-manifest+json"

	manifestJSON, err := json.Marshal(backend.ManifestBody{
		CiphertextObject: ciphertextRef,
		UploadID:         "upload-repair-test",
		CompletedAt:      meta["x-amz-meta-armor-completed-at"],
		Metadata:         meta,
	})
	if err != nil {
		rh.t.Fatalf("failed to marshal manifest body: %v", err)
	}
	if err := rh.mb.Put(context.Background(), rh.bucket, rh.key+".armor-manifest", bytes.NewReader(manifestJSON), int64(len(manifestJSON)), meta); err != nil {
		rh.t.Fatalf("failed to write manifest: %v", err)
	}
}

// TestManifestRepairRespectsKeyPrefix pins that the repair operations address
// the same prefixed manifest object the read path reads. The manifest key is
// derived once, in manifestKeyFor, and shared by readManifest and every
// rewrite, so the two cannot drift — this test fails if they ever do.
func TestManifestRepairRespectsKeyPrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "kalshi-tape/")
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	bucket, key := "repair-bucket", "prefixed-repair-key"
	plaintext := []byte("prefixed operator repair plaintext")

	putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, bytes.NewReader(plaintext))
	putReq.Header.Set("Content-Type", "text/plain")
	putW := httptest.NewRecorder()
	h.HandleRoot(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d, body %s", putW.Code, putW.Body.String())
	}

	// The PUT landed under the configured prefix; the manifest fixture is
	// written against that stored key, exactly as CompleteMultipartUpload
	// would in a prefixed deployment.
	mb.mu.Lock()
	var storedKey string
	for backendKey := range mb.objects {
		if strings.HasSuffix(backendKey, "/"+key) && !strings.HasSuffix(backendKey, ".armor-manifest") {
			storedKey = strings.TrimPrefix(backendKey, bucket+"/")
		}
	}
	armorMeta := make(map[string]string, len(mb.meta[bucket+"/"+storedKey]))
	for k, v := range mb.meta[bucket+"/"+storedKey] {
		armorMeta[k] = v
	}
	mb.mu.Unlock()
	if storedKey == key {
		t.Fatalf("PUT ignored the configured prefix: object stored at %q", storedKey)
	}

	meta := make(map[string]string, len(armorMeta)+2)
	for k, v := range armorMeta {
		meta[k] = v
	}
	// completedAt in the past makes the manifest stale: the mock stamps the
	// ciphertext's LastModified at write time.
	stamped := time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	meta["x-amz-meta-armor-completed-at"] = stamped
	meta["x-amz-meta-armor-ciphertext-ref"] = storedKey
	meta["Content-Type"] = "application/x-armor-manifest+json"
	manifestJSON, err := json.Marshal(backend.ManifestBody{
		CiphertextObject: storedKey,
		UploadID:         "upload-repair-test",
		CompletedAt:      stamped,
		Metadata:         meta,
	})
	if err != nil {
		t.Fatalf("marshal manifest body: %v", err)
	}
	if err := mb.Put(context.Background(), bucket, storedKey+".armor-manifest", bytes.NewReader(manifestJSON), int64(len(manifestJSON)), meta); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// The repair is addressed with the client key, not the stored key — the
	// operator names the object the way readers do.
	if _, err := h.RepairManifest(context.Background(), bucket, key); err != nil {
		t.Fatalf("RepairManifest on prefixed object: %v", err)
	}
	get := func() (int, string) {
		req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		return w.Code, w.Body.String()
	}
	if code, body := get(); code != http.StatusOK {
		t.Fatalf("GetObject after prefixed repair returned %d, want 200: %s", code, body)
	}

	if _, err := h.QuarantineManifest(context.Background(), bucket, key, "prefixed quarantine"); err != nil {
		t.Fatalf("QuarantineManifest on prefixed object: %v", err)
	}
	if code, body := get(); code != http.StatusForbidden {
		t.Fatalf("GetObject on prefixed quarantined object returned %d, want 403: %s", code, body)
	}
}
