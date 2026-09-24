package restoreverifier

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/metrics"
)

// contract_test.go pins the published HTTP contract of the restore-verifier
// status surface: the BucketState and VerificationResult JSON field sets, the
// healthz/readyz verdict rules and their transitions, and the trigger
// endpoint's mode validation. The human-readable contract is
// docs/observability-contract.md; a change here that breaks one of these
// assertions is a breaking change to that document and must land together.
// The escalation/dedupe mechanism's behavior is specified by
// escalation_test.go and is not duplicated here.

const contractBucket = "contract-bucket"

// newContractVerifier builds a verifier with one enabled bucket and no
// backend: enough to serve status JSON, never enough to reach a backend.
func newContractVerifier(t *testing.T) *Verifier {
	t.Helper()
	return New(nil, nil, nil, 65536, nil, Config{
		Buckets: []BucketConfig{{Bucket: contractBucket, Enabled: true}},
	})
}

// state returns the live (unexported-map) BucketState for the contract bucket.
func contractState(t *testing.T, v *Verifier) *BucketState {
	t.Helper()
	s, ok := v.buckets[contractBucket]
	if !ok {
		t.Fatalf("bucket %s not in verifier state", contractBucket)
	}
	return s
}

func doGet(h http.HandlerFunc, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// TestStatusHandlerBucketStateJSONContract pins the exact BucketState JSON
// key set served by GET /status — the fields docs/observability-contract.md
// documents, drill fields included.
func TestStatusHandlerBucketStateJSONContract(t *testing.T) {
	v := newContractVerifier(t)

	rec := doGet(v.StatusHandler(nil), "/status")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /status = %d, want 200", rec.Code)
	}

	var status map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode /status body: %v", err)
	}
	bs, ok := status[contractBucket]
	if !ok {
		t.Fatalf("/status has no entry for %s", contractBucket)
	}

	want := []string{
		"bucket", "last_verification", "last_success", "verified_object_ratio",
		"total_objects", "verified_objects", "failed_objects", "recent_results",
		"historical_sample_size",
		"drill_last_verification", "drill_last_success",
		"drill_total_objects", "drill_verified_objects", "drill_failed_objects",
	}
	for _, field := range want {
		if _, ok := bs[field]; !ok {
			t.Errorf("BucketState JSON missing contract field %q", field)
		}
	}
	for field := range bs {
		var known bool
		for _, w := range want {
			if w == field {
				known = true
			}
		}
		if !known {
			t.Errorf("undocumented BucketState field %q; update docs/observability-contract.md", field)
		}
	}

	// Fresh state: never verified, never succeeded — the values /readyz and
	// /healthz read before the first run.
	if bs["bucket"] != contractBucket {
		t.Errorf("bucket = %v, want %s", bs["bucket"], contractBucket)
	}
	if v, err := bs["verified_object_ratio"].(float64); !err || v != 0 {
		t.Errorf("fresh verified_object_ratio = %v, want 0", bs["verified_object_ratio"])
	}
}

// TestStatusHandlerVerificationResultJSONContract pins the VerificationResult
// field set (a member of recent_results), with every omitempty field
// populated so the full contract shape is exercised.
func TestStatusHandlerVerificationResultJSONContract(t *testing.T) {
	v := newContractVerifier(t)

	now := time.Now()
	state := contractState(t, v)
	state.mu.Lock()
	state.LastVerification = now
	state.LastSuccess = now
	state.TotalObjects = 2
	state.VerifiedObjects = 1
	state.FailedObjects = 1
	state.RecentResults = []VerificationResult{{
		Key:               "backups/app.db",
		Bucket:            contractBucket,
		Status:            StatusConflict,
		Path:              PathARMOR,
		Timestamp:         now,
		ArtifactType:      ArtifactSQLite,
		ExpectedSHA256:    "expected",
		ARMORSHA256:       "armor-digest",
		DirectSHA256:      "direct-digest",
		ARMORPathLatency:  1500,
		DirectPathLatency: 2500,
		Error:             "paths disagree",
		AssertionPassed:   false,
		AssertionError:    "integrity_check failed",
	}}
	state.mu.Unlock()

	rec := doGet(v.StatusHandler(nil), "/status")
	var status map[string]struct {
		RecentResults []map[string]any `json:"recent_results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode /status body: %v", err)
	}
	results := status[contractBucket].RecentResults
	if len(results) != 1 {
		t.Fatalf("recent_results = %d entries, want 1", len(results))
	}

	want := []string{
		"key", "bucket", "status", "path", "timestamp", "artifact_type",
		"expected_sha256", "armor_sha256", "direct_sha256",
		"armor_path_latency_ms", "direct_path_latency_ms",
		"error", "assertion_passed", "assertion_error",
	}
	for _, field := range want {
		if _, ok := results[0][field]; !ok {
			t.Errorf("VerificationResult JSON missing contract field %q", field)
		}
	}
	if results[0]["status"] != string(StatusConflict) {
		t.Errorf("status = %v, want %q", results[0]["status"], StatusConflict)
	}
	if results[0]["path"] != string(PathARMOR) {
		t.Errorf("path = %v, want %q", results[0]["path"], PathARMOR)
	}
}

// TestBucketStatusHandlerParamContract pins /bucket's parameter contract:
// 400 without the parameter, 404 for an unconfigured bucket, 200 with the
// same BucketState shape otherwise.
func TestBucketStatusHandlerParamContract(t *testing.T) {
	v := newContractVerifier(t)
	h := v.BucketStatusHandler(nil)

	if rec := doGet(h, "/bucket"); rec.Code != http.StatusBadRequest {
		t.Errorf("GET /bucket (no param) = %d, want 400", rec.Code)
	}
	if rec := doGet(h, "/bucket?bucket=nope"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /bucket?bucket=nope = %d, want 404", rec.Code)
	}
	if rec := doGet(h, "/bucket?bucket="+contractBucket); rec.Code != http.StatusOK {
		t.Errorf("GET /bucket?bucket=%s = %d, want 200", contractBucket, rec.Code)
	}
}

// TestTriggerHandlerModeContract pins POST /trigger: 202 with the
// mode-specific confirmation for dual (the default) and dr-drill, 400 naming
// the mode for anything else, 405 for non-POST. With zero configured buckets
// the spawned run is a no-op, so the background goroutine is safe in tests.
func TestTriggerHandlerModeContract(t *testing.T) {
	v := New(nil, nil, nil, 65536, nil, Config{})
	h := v.TriggerHandler(metrics.NewMetrics())

	post := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}

	rec := post("/trigger")
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), "Verification triggered") {
		t.Errorf("POST /trigger (default mode) = %d %q, want 202 dual confirmation", rec.Code, rec.Body.String())
	}
	rec = post("/trigger?mode=dual")
	if rec.Code != http.StatusAccepted {
		t.Errorf("POST /trigger?mode=dual = %d, want 202", rec.Code)
	}
	rec = post("/trigger?mode=dr-drill")
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), "DR-drill") {
		t.Errorf("POST /trigger?mode=dr-drill = %d %q, want 202 drill confirmation", rec.Code, rec.Body.String())
	}

	rec = post("/trigger?mode=bogus")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "bogus") {
		t.Errorf("unknown mode = %d %q, want 400 naming the mode", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/trigger", nil)
	rec = httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /trigger = %d, want 405", rec.Code)
	}
}

// TestHealthAndReadyTransitions drives the /healthz and /readyz verdict
// rules through their transitions: fresh (never verified) is 503 on both;
// a completed successful run turns both 200; a failed object turns only
// /healthz; staleness beyond 24h turns only /healthz.
func TestHealthAndReadyTransitions(t *testing.T) {
	v := newContractVerifier(t)
	healthz := v.HealthHandler(metrics.NewMetrics())
	readyz := v.ReadyHandler(metrics.NewMetrics())
	state := contractState(t, v)

	if rec := doGet(healthz, "/healthz"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("fresh verifier /healthz = %d, want 503 (never verified)", rec.Code)
	}
	if rec := doGet(readyz, "/readyz"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("fresh verifier /readyz = %d, want 503 (no success yet)", rec.Code)
	}

	// A completed run with everything verified: both endpoints turn 200.
	now := time.Now()
	state.mu.Lock()
	state.LastVerification = now
	state.LastSuccess = now
	state.mu.Unlock()
	if rec := doGet(healthz, "/healthz"); rec.Code != http.StatusOK {
		t.Errorf("/healthz after successful run = %d, want 200", rec.Code)
	}
	if rec := doGet(readyz, "/readyz"); rec.Code != http.StatusOK {
		t.Errorf("/readyz after successful run = %d, want 200", rec.Code)
	}

	// A failed object in the latest sample: /healthz flips to 503 while
	// /readyz stays 200 — a past success still proves readiness.
	state.mu.Lock()
	state.FailedObjects = 1
	state.mu.Unlock()
	if rec := doGet(healthz, "/healthz"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/healthz with failed objects = %d, want 503", rec.Code)
	}
	if rec := doGet(readyz, "/readyz"); rec.Code != http.StatusOK {
		t.Errorf("/readyz with failed objects = %d, want 200 (last_success unchanged)", rec.Code)
	}

	// Staleness: last_verification beyond 24h is unhealthy even with no
	// failed objects.
	state.mu.Lock()
	state.FailedObjects = 0
	state.LastVerification = now.Add(-25 * time.Hour)
	state.mu.Unlock()
	if rec := doGet(healthz, "/healthz"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/healthz with stale last_verification = %d, want 503", rec.Code)
	}

	// Method guard: the GET endpoints reject POST.
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()
	healthz(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /healthz = %d, want 405", rec.Code)
	}
}

// TestRunStatusEnumContract pins the verification status/path/mode vocabularies
// the /status JSON and the trigger API speak.
func TestRunStatusEnumContract(t *testing.T) {
	wantStatuses := map[VerificationStatus]bool{
		StatusPass: true, StatusFail: true, StatusPending: true,
		StatusUnknown: true, StatusConflict: true, StatusRestoreError: true,
		StatusChecksumError: true, StatusAssertionError: true,
	}
	if len(wantStatuses) != 8 {
		t.Fatalf("status vocabulary changed: %d statuses", len(wantStatuses))
	}
	for s, spelled := range map[VerificationStatus]string{
		StatusPass: "pass", StatusConflict: "conflict",
		StatusRestoreError: "restore_error", StatusChecksumError: "checksum_error",
		StatusAssertionError: "assertion_error",
	} {
		if string(s) != spelled {
			t.Errorf("status %q renamed to %q; update docs/observability-contract.md", spelled, s)
		}
	}

	if PathARMOR != "armor" || PathDirect != "direct" || PathDualMatch != "dual_match" {
		t.Errorf("path vocabulary changed: %q %q %q", PathARMOR, PathDirect, PathDualMatch)
	}
	if ModeDual != "dual" || ModeDRDrill != "dr-drill" {
		t.Errorf("mode vocabulary changed: %q %q", ModeDual, ModeDRDrill)
	}
	if string(BeadStaleness) != "staleness" || string(BeadFailure) != "failure" {
		t.Errorf("bead kinds changed: %q %q", BeadFailure, BeadStaleness)
	}
}

// TestFailureClassVocabularyContract pins the escalation failure classes —
// the fourth component of the dedupe key and a documented vocabulary.
func TestFailureClassVocabularyContract(t *testing.T) {
	for status, want := range map[VerificationStatus]FailureClass{
		StatusRestoreError:   FailureRestoreError,
		StatusChecksumError:  FailureChecksumError,
		StatusAssertionError: FailureAssertionError,
		StatusConflict:       FailureConflict,
		StatusFail:           FailureRestoreError, // unrecognized non-pass falls back to restore_error
	} {
		if got := classFor(status); got != want {
			t.Errorf("classFor(%q) = %q, want %q", status, got, want)
		}
	}
	if FailureRestoreError != "restore_error" || FailureChecksumError != "checksum_error" ||
		FailureAssertionError != "assertion_error" || FailureConflict != "dual_path_conflict" {
		t.Errorf("failure-class strings changed: %q %q %q %q",
			FailureRestoreError, FailureChecksumError, FailureAssertionError, FailureConflict)
	}
}

// TestFreshnessWindowConstants pins the two configured freshness numbers the
// contract doc publishes: the default staleness window (escalation) and the
// per-run deadline.
func TestFreshnessWindowConstants(t *testing.T) {
	if DefaultRunTimeout != 2*time.Hour {
		t.Errorf("DefaultRunTimeout = %s, want 2h", DefaultRunTimeout)
	}
	// The verifier's own zero-value deadline fallback must be the default.
	v := newContractVerifier(t)
	if v.runTimeout != DefaultRunTimeout {
		t.Errorf("zero Config.RunTimeout fell back to %s, want DefaultRunTimeout", v.runTimeout)
	}
}
