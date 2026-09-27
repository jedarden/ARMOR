package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// verifier_exposition_test.go pins the restore-verifier's /metrics perimeter:
// the verifier links this package for its restore-recording methods, and
// before the VerifierMetricsHandler filter its :9002 endpoint re-exported the
// whole armor_* family — including every canary gauge pinned at its
// before-first-check default — which is what forced the armor scrape job to
// drop canary families for the verifier target at scrape time (and what
// paged continuously on activation day, 2026-09-25, before the drop existed).

// TestVerifierExpositionCarriesOnlyVerifierFamilies drives every
// restore-verifier recording path plus one server-only path per server
// family group, and asserts the filtered exposition keeps exactly the
// verifier-owned families and nothing else.
func TestVerifierExpositionCarriesOnlyVerifierFamilies(t *testing.T) {
	m := NewMetrics()

	// Verifier-owned recording paths.
	m.RecordRestoreVerifierCheck("perimeter-bucket", 250*time.Millisecond, true)
	m.RecordRestoreBucketState("perimeter-bucket", time.Unix(1700000000, 0), 1, 2)
	m.RecordDRDrillRun("perimeter-bucket", time.Unix(1700000001, 0), time.Unix(1700000001, 0), 1, 1)

	// Server-only paths — every group the shared exposition carries that the
	// verifier never produces.
	m.IncCanaryChecks()
	m.IncMultipartCanaryChecks()
	m.SetMultipartCanaryHealthy(true)
	m.IncSecondaryCanaryChecks()
	m.IncRequestsTotal("put", 200)
	m.IncEncryptionOps("aes-gcm")
	m.IncBackendRequests("put")
	m.IncReplicationEnqueued("put")
	m.IncErrors("NoSuchKey", "get")
	m.IncRequestsByCredential("key", "put", "allow")

	dump := verifierExposition(m.PrometheusFormat())
	types := typeLines(t, dump)

	for _, want := range []string{
		"armor_restore_verifier_checks_total",
		"armor_restore_verifier_failures_total",
		"armor_restore_verifier_objects_verified",
		"armor_restore_verifier_objects_failed",
		"armor_restore_verifier_latency_millis",
		"armor_last_verified_restore_timestamp",
		"armor_verified_object_ratio",
		"armor_restore_verification_failures_total",
		"armor_drill_last_verified_timestamp",
		"armor_drill_last_success_timestamp",
		"armor_drill_verified_object_ratio",
		"armor_drill_failures_total",
		"armor_uptime_seconds",
	} {
		if _, ok := types[want]; !ok {
			t.Errorf("verifier exposition dropped verifier-owned family %s", want)
		}
	}

	for _, absent := range []string{
		"armor_canary_checks_total",
		"armor_multipart_canary_healthy",
		"armor_secondary_canary_healthy",
		"armor_multipart_canary_upload_duration_seconds",
		"armor_requests_total",
		"armor_encryption_ops_total",
		"armor_backend_requests_total",
		"armor_replication_enqueued_total",
		"armor_errors_total",
		"armor_requests_by_credential_total",
	} {
		if strings.Contains(dump, absent) {
			t.Errorf("verifier exposition leaks server-only family %s", absent)
		}
	}

	// The owned samples survive the filter with their labels intact — these
	// are the series the ArmorRestoreVerification* rules evaluate.
	for _, want := range []string{
		`armor_last_verified_restore_timestamp{bucket="perimeter-bucket"} 1700000000`,
		`armor_verified_object_ratio{bucket="perimeter-bucket"} 1`,
		`armor_restore_verification_failures_total{bucket="perimeter-bucket"} 2`,
		`armor_drill_last_success_timestamp{bucket="perimeter-bucket"} 1700000001`,
	} {
		if !strings.Contains(dump, want+"\n") {
			t.Errorf("verifier exposition missing owned sample %q", want)
		}
	}
}

// TestVerifierExpositionEmptyStateStillDeclaresFamilies pins that a verifier
// that has not run yet still declares its own families (HELP/TYPE headers
// survive the filter even with no samples) and declares no server family.
func TestVerifierExpositionEmptyStateStillDeclaresFamilies(t *testing.T) {
	dump := verifierExposition(NewMetrics().PrometheusFormat())

	for _, want := range []string{
		"# TYPE armor_restore_verifier_checks_total counter",
		"# TYPE armor_last_verified_restore_timestamp gauge",
	} {
		if !strings.Contains(dump, want+"\n") {
			t.Errorf("empty verifier exposition missing family header %q", want)
		}
	}
	if strings.Contains(dump, "armor_multipart_canary_healthy") {
		t.Error("empty verifier exposition declares the server's canary family")
	}
}

// TestVerifierMetricsHandlerResponse pins the handler plumbing: 200, the
// Prometheus content type, and a body that stays inside the verifier
// perimeter.
func TestVerifierMetricsHandlerResponse(t *testing.T) {
	m := NewMetrics()
	m.RecordRestoreBucketState("perimeter-bucket", time.Unix(1700000000, 0), 1, 0)
	m.IncMultipartCanaryChecks()

	rec := httptest.NewRecorder()
	m.VerifierMetricsHandler()(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET /metrics = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; version=0.0.4" {
		t.Errorf("Content-Type = %q, want text/plain; version=0.0.4", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `armor_last_verified_restore_timestamp{bucket="perimeter-bucket"}`) {
		t.Error("handler body dropped the verifier-owned restorability series")
	}
	if strings.Contains(body, "armor_multipart_canary_") {
		t.Error("handler body leaked the server's multipart canary family")
	}
}
