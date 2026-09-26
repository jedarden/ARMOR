package metrics

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// observability_contract_test.go pins the published Prometheus contract for
// the canary and restore-verifier metric families: the exact set of TYPE
// lines each family emits, the label sets, and the gauge/counter transition
// rules that drive alerting (ADR-002 multipart visibility, ADR-004
// restorability + drill isolation). The human-readable contract is
// docs/observability-contract.md; a change here that breaks one of these
// assertions is a breaking change to that document and must land together.

var typeLineRe = regexp.MustCompile(`^# TYPE (armor_\S+) (\w+)$`)

// typeLines parses every "# TYPE armor_..." line from a PrometheusFormat dump.
func typeLines(t *testing.T, dump string) map[string]string {
	t.Helper()
	got := make(map[string]string)
	for _, line := range strings.Split(dump, "\n") {
		m := typeLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if _, dup := got[m[1]]; dup {
			t.Errorf("metric %s has more than one TYPE line", m[1])
		}
		got[m[1]] = m[2]
	}
	return got
}

// gaugeValue extracts the unlabeled numeric value of a series from a dump.
func gaugeValue(t *testing.T, dump, series string) (string, bool) {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(series) + ` (\S+)$`)
	m := re.FindStringSubmatch(dump)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// labeledValue extracts the numeric value of one labeled instance of a series.
func labeledValue(t *testing.T, dump, series, labelMatch string) (string, bool) {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(series) + `\{` + labelMatch + `\} (\S+)$`)
	m := re.FindStringSubmatch(dump)
	if m == nil {
		return "", false
	}
	return m[1], true
}

var canaryFamily = map[string]string{
	"armor_canary_checks_total":                      "counter",
	"armor_canary_check_failures_total":              "counter",
	"armor_multipart_canary_checks_total":            "counter",
	"armor_multipart_canary_check_failures_total":    "counter",
	"armor_multipart_canary_healthy":                 "gauge",
	"armor_secondary_canary_checks_total":            "counter",
	"armor_secondary_canary_check_failures_total":    "counter",
	"armor_secondary_canary_healthy":                 "gauge",
	"armor_multipart_canary_upload_duration_seconds": "histogram",
}

// TestCanaryMetricFamilyContract pins the canary family's exact series set
// and types — including the absence of any armor_canary_healthy series,
// which earlier revisions of docs/metrics.md documented in error. The
// small-object canary's health travels via /armor/canary and /readyz, not
// via a gauge.
func TestCanaryMetricFamilyContract(t *testing.T) {
	m := NewMetrics()

	// Drive every canary recording path so value-format assertions below
	// see populated series.
	m.IncCanaryChecks()
	m.IncCanaryFailures()
	m.SetCanaryLastCheck(time.Unix(1700000000, 0))
	m.SetCanaryLastError("synthetic contract failure")
	m.IncMultipartCanaryChecks()
	m.IncMultipartCanaryFailures()
	m.SetMultipartCanaryLastCheck(time.Unix(1700000000, 0))
	m.SetMultipartCanaryLastError("")
	m.SetMultipartCanaryHealthy(true)
	m.IncSecondaryCanaryChecks()
	m.IncSecondaryCanaryFailures()
	m.SetSecondaryCanaryLastCheck(time.Unix(1700000000, 0))
	m.SetSecondaryCanaryHealthy(false)

	dump := m.PrometheusFormat()
	got := typeLines(t, dump)

	// No undocumented canary series: any armor_*canary* series outside the
	// documented family is a contract break.
	for name := range got {
		if strings.Contains(name, "canary") {
			if _, ok := canaryFamily[name]; !ok {
				t.Errorf("undocumented canary series %s; update docs/observability-contract.md", name)
			}
		}
	}
	for name, typ := range canaryFamily {
		if got[name] != typ {
			t.Errorf("canary series %s TYPE = %q, want %q", name, got[name], typ)
		}
	}
	if _, exists := got["armor_canary_healthy"]; exists {
		t.Error("armor_canary_healthy must not be exported; small-object canary health is carried by /armor/canary and /readyz")
	}

	// String-valued diagnostics stay available through the status endpoints,
	// but must not appear as invalid numeric Prometheus samples. One malformed
	// sample makes Prometheus reject the whole target scrape.
	for _, name := range []string{
		"armor_canary_last_check_time", "armor_canary_last_check_error",
		"armor_multipart_canary_last_check_time", "armor_multipart_canary_last_check_error",
		"armor_secondary_canary_last_check_time", "armor_secondary_canary_last_check_error",
		"armor_key_rotation_start_time",
	} {
		if strings.Contains(dump, name) {
			t.Errorf("string diagnostic %s must not be emitted in Prometheus exposition", name)
		}
	}
}

// TestMultipartCanaryHealthyGaugeTransitions pins the alert-facing gauge
// contract of armor_multipart_canary_healthy: 1 after a passing check, 0
// after a failed one, 0 before the first completed check (ArmorMultipartCanaryUnhealthy
// fires on == 0).
func TestMultipartCanaryHealthyGaugeTransitions(t *testing.T) {
	m := NewMetrics()

	dump := m.PrometheusFormat()
	if v, ok := gaugeValue(t, dump, "armor_multipart_canary_healthy"); !ok || v != "0" {
		t.Errorf("fresh metrics: armor_multipart_canary_healthy = (%q, %v), want (\"0\", true)", v, ok)
	}

	m.SetMultipartCanaryHealthy(true)
	if v, _ := gaugeValue(t, m.PrometheusFormat(), "armor_multipart_canary_healthy"); v != "1" {
		t.Errorf("after passing check: armor_multipart_canary_healthy = %s, want 1", v)
	}

	m.SetMultipartCanaryHealthy(false)
	if v, _ := gaugeValue(t, m.PrometheusFormat(), "armor_multipart_canary_healthy"); v != "0" {
		t.Errorf("after failed check: armor_multipart_canary_healthy = %s, want 0", v)
	}
}

var restoreVerifierFamily = map[string]string{
	"armor_restore_verifier_checks_total":       "counter",
	"armor_restore_verifier_failures_total":     "counter",
	"armor_restore_verifier_objects_verified":   "counter",
	"armor_restore_verifier_objects_failed":     "counter",
	"armor_restore_verifier_latency_millis":     "gauge",
	"armor_last_verified_restore_timestamp":     "gauge",
	"armor_verified_object_ratio":               "gauge",
	"armor_restore_verification_failures_total": "counter",
	"armor_drill_last_verified_timestamp":       "gauge",
	"armor_drill_last_success_timestamp":        "gauge",
	"armor_drill_verified_object_ratio":         "gauge",
	"armor_drill_failures_total":                "counter",
}

// TestRestoreVerifierMetricFamilyContract pins the restore-verifier family's
// exact series set and types, all labeled by bucket.
func TestRestoreVerifierMetricFamilyContract(t *testing.T) {
	m := NewMetrics()

	m.RecordRestoreVerifierCheck("bucket-contract", 250*time.Millisecond, true)
	m.SetRestoreVerifierLastCheckTime(time.Unix(1700000000, 0))
	m.SetRestoreVerifierLastError("none")
	m.RecordRestoreBucketState("bucket-contract", time.Unix(1700000000, 0), 1, 0)
	m.RecordDRDrillRun("bucket-contract", time.Unix(1700000001, 0), time.Time{}, 0.5, 1)

	got := typeLines(t, m.PrometheusFormat())
	for name, typ := range restoreVerifierFamily {
		if got[name] != typ {
			t.Errorf("restore-verifier series %s TYPE = %q, want %q", name, got[name], typ)
		}
	}

	// Every restorability/drill series is bucket-labeled.
	for _, series := range []string{
		"armor_last_verified_restore_timestamp", "armor_verified_object_ratio",
		"armor_restore_verification_failures_total",
		"armor_drill_last_verified_timestamp", "armor_drill_last_success_timestamp",
		"armor_drill_verified_object_ratio", "armor_drill_failures_total",
	} {
		if _, ok := labeledValue(t, m.PrometheusFormat(), series, `bucket="bucket-contract"`); !ok {
			t.Errorf("%s has no bucket=%q instance", series, "bucket-contract")
		}
	}
}

// TestRestoreBucketStateTransitions pins the restorability signal set's
// transition rules (ADR-004 decision 6): a bucket with no successful restore
// exports timestamp 0 (immediately eligible for the stale alert), a success
// advances the timestamp, the ratio is clamped to [0,1], and the failure
// counter is monotone — an older snapshot never decreases it.
func TestRestoreBucketStateTransitions(t *testing.T) {
	m := NewMetrics()
	const b = `bucket="transition-contract"`

	// Stale: never succeeded -> 0 timestamp, 0 ratio.
	m.RecordRestoreBucketState("transition-contract", time.Time{}, 0, 0)
	dump := m.PrometheusFormat()
	if v, _ := labeledValue(t, dump, "armor_last_verified_restore_timestamp", b); v != "0" {
		t.Errorf("never-succeeded bucket: armor_last_verified_restore_timestamp = %s, want 0", v)
	}
	if v, _ := labeledValue(t, dump, "armor_verified_object_ratio", b); v != "0" {
		t.Errorf("never-succeeded bucket: armor_verified_object_ratio = %s, want 0", v)
	}

	// Fresh success: timestamp advances, ratio published, failure counter set.
	success := time.Unix(1700000100, 0)
	m.RecordRestoreBucketState("transition-contract", success, 0.8, 2)
	dump = m.PrometheusFormat()
	if v, _ := labeledValue(t, dump, "armor_last_verified_restore_timestamp", b); v != "1700000100" {
		t.Errorf("after success: armor_last_verified_restore_timestamp = %s, want 1700000100", v)
	}
	if v, _ := labeledValue(t, dump, "armor_verified_object_ratio", b); v != "0.8" {
		t.Errorf("after success: armor_verified_object_ratio = %s, want 0.8", v)
	}
	if v, _ := labeledValue(t, dump, "armor_restore_verification_failures_total", b); v != "2" {
		t.Errorf("after failures: armor_restore_verification_failures_total = %s, want 2", v)
	}

	// Monotonicity: a caller reporting an older snapshot (1 failure) must not
	// decrease the published counter.
	m.RecordRestoreBucketState("transition-contract", success, 1, 1)
	if v, _ := labeledValue(t, m.PrometheusFormat(), "armor_restore_verification_failures_total", b); v != "2" {
		t.Errorf("older snapshot decreased the counter: armor_restore_verification_failures_total = %s, want 2 (monotone)", v)
	}

	// Ratio clamping: out-of-range and NaN inputs collapse into [0,1].
	m.RecordRestoreBucketState("transition-contract", success, 1.5, 2)
	if v, _ := labeledValue(t, m.PrometheusFormat(), "armor_verified_object_ratio", b); v != "1" {
		t.Errorf("ratio > 1 not clamped: got %s, want 1", v)
	}
}

// TestDRDrillGaugeTransitionsAndIsolation pins the drill gauges'
// transition rules and their isolation from the dual-path series: a drill
// attempt advances drill_last_verified even when it recovers nothing,
// drill_last_success stays 0 until a drill actually proves recovery, and
// neither touches armor_last_verified_restore_timestamp.
func TestDRDrillGaugeTransitionsAndIsolation(t *testing.T) {
	m := NewMetrics()
	const b = `bucket="drill-contract"`

	// Isolation precondition: publishing a dual-path state must not create
	// drill series, and vice versa.
	m.RecordRestoreBucketState("drill-contract", time.Unix(1700000000, 0), 1, 0)
	dump := m.PrometheusFormat()
	if _, ok := labeledValue(t, dump, "armor_drill_last_verified_timestamp", b); ok {
		t.Error("dual-path state publication must not create drill series")
	}

	// Failed drill attempt: last_verified advances, last_success stays 0.
	m.RecordDRDrillRun("drill-contract", time.Unix(1700000060, 0), time.Time{}, 0, 3)
	dump = m.PrometheusFormat()
	if v, _ := labeledValue(t, dump, "armor_drill_last_verified_timestamp", b); v != "1700000060" {
		t.Errorf("failed drill must advance drill_last_verified: got %s", v)
	}
	if v, _ := labeledValue(t, dump, "armor_drill_last_success_timestamp", b); v != "0" {
		t.Errorf("drill_last_success must stay 0 until recovery is proven, got %s", v)
	}
	if v, _ := labeledValue(t, dump, "armor_drill_failures_total", b); v != "3" {
		t.Errorf("drill failure counter = %s, want 3", v)
	}

	// Successful drill: last_success advances.
	m.RecordDRDrillRun("drill-contract", time.Unix(1700000120, 0), time.Unix(1700000120, 0), 1, 3)
	dump = m.PrometheusFormat()
	if v, _ := labeledValue(t, dump, "armor_drill_last_success_timestamp", b); v != "1700000120" {
		t.Errorf("after successful drill: drill_last_success = %s, want 1700000120", v)
	}
	if v, _ := labeledValue(t, dump, "armor_drill_verified_object_ratio", b); v != "1" {
		t.Errorf("after successful drill: drill ratio = %s, want 1", v)
	}

	// The drill published nothing into the dual-path series.
	if v, _ := labeledValue(t, dump, "armor_last_verified_restore_timestamp", b); v != "1700000000" {
		t.Errorf("drill run perturbed the dual-path timestamp: %s, want 1700000000", v)
	}

	// Zero-value lastSuccess (never succeeded) must export 0, not a negative
	// or garbage timestamp — the stale-alert eligibility contract.
	m2 := NewMetrics()
	m2.RecordDRDrillRun("drill-contract", time.Unix(1700000000, 0), time.Time{}, 0, 0)
	v, _ := labeledValue(t, m2.PrometheusFormat(), "armor_drill_last_success_timestamp", b)
	if v != "0" {
		t.Errorf("zero lastSuccess exported as %s, want 0", v)
	}
}

// TestRecordBucketStateEmptyBucketIgnored pins the guard that an empty bucket
// label never creates a series — an unlabeled armor_last_verified_restore_timestamp
// would silently aggregate buckets in sum-by alert expressions.
func TestRecordBucketStateEmptyBucketIgnored(t *testing.T) {
	m := NewMetrics()
	m.RecordRestoreBucketState("", time.Now(), 1, 0)
	if strings.Contains(m.PrometheusFormat(), `armor_last_verified_restore_timestamp{bucket=""}`) {
		t.Error("empty bucket label must not publish a series")
	}
}

// TestMultipartCanaryHistogramLabels pins the multipart canary histogram's
// label contract: operation x status pairs, with _sum/_count/_last exported
// only for pairs with at least one observation.
func TestMultipartCanaryHistogramLabels(t *testing.T) {
	m := NewMetrics()
	m.RecordMultipartUpload("upload", "success", 1500*time.Millisecond)

	dump := m.PrometheusFormat()
	for _, want := range []string{
		`armor_multipart_canary_upload_duration_seconds_count{operation="upload",status="success"} 1`,
		`armor_multipart_canary_upload_duration_seconds_sum{operation="upload",status="success"} 1.500000`,
	} {
		if !strings.Contains(dump, want+"\n") {
			t.Errorf("multipart histogram missing series %q", want)
		}
	}
	// A pair with no observations must not emit a zero-count series.
	if strings.Contains(dump, `status="failure"`) {
		t.Error("multipart histogram exported series for an operation/status pair with no observations")
	}
}

// TestEveryExportedTypeLineIsPrefixedArmor pins the global namespace
// contract: every exported metric is armor_-prefixed, so cross-component
// dashboards never collide with foreign exporters.
func TestEveryExportedTypeLineIsPrefixedArmor(t *testing.T) {
	m := NewMetrics()
	for name := range typeLines(t, m.PrometheusFormat()) {
		if !strings.HasPrefix(name, "armor_") {
			t.Errorf("metric %s violates the armor_ prefix contract", name)
		}
	}
}
