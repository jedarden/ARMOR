package metrics

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"expvar"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}

	// Verify all metric counters are initialized
	if m.RequestsTotal == nil {
		t.Error("RequestsTotal not initialized")
	}
	if m.BytesUploaded == nil {
		t.Error("BytesUploaded not initialized")
	}
	if m.CacheHitsTotal == nil {
		t.Error("CacheHitsTotal not initialized")
	}
}

func TestMetricsIncRequestsTotal(t *testing.T) {
	m := NewMetrics()

	// Initial value should be 0
	if m.RequestsTotal.String() != "0" {
		t.Errorf("expected initial value 0, got %s", m.RequestsTotal.String())
	}

	m.IncRequestsTotal("GET", 200)
	m.IncRequestsTotal("PUT", 201)
	m.IncRequestsTotal("GET", 200)

	if m.RequestsTotal.String() != "3" {
		t.Errorf("expected 3 requests, got %s", m.RequestsTotal.String())
	}
}

func TestMetricsInFlightRequests(t *testing.T) {
	m := NewMetrics()

	m.IncRequestsInFlight()
	if m.RequestsInFlight.String() != "1" {
		t.Errorf("expected 1 in-flight, got %s", m.RequestsInFlight.String())
	}

	m.IncRequestsInFlight()
	m.IncRequestsInFlight()
	if m.RequestsInFlight.String() != "3" {
		t.Errorf("expected 3 in-flight, got %s", m.RequestsInFlight.String())
	}

	m.DecRequestsInFlight()
	if m.RequestsInFlight.String() != "2" {
		t.Errorf("expected 2 in-flight, got %s", m.RequestsInFlight.String())
	}
}

func TestMetricsBytesTracking(t *testing.T) {
	m := NewMetrics()

	m.AddBytesUploaded(1024)
	m.AddBytesUploaded(2048)
	if m.BytesUploaded.String() != "3072" {
		t.Errorf("expected 3072 bytes uploaded, got %s", m.BytesUploaded.String())
	}

	m.AddBytesDownloaded(512)
	if m.BytesDownloaded.String() != "512" {
		t.Errorf("expected 512 bytes downloaded, got %s", m.BytesDownloaded.String())
	}

	m.AddBytesFetchedFromB2(4096)
	if m.BytesFetchedFromB2.String() != "4096" {
		t.Errorf("expected 4096 bytes fetched, got %s", m.BytesFetchedFromB2.String())
	}
}

func TestMetricsCacheTracking(t *testing.T) {
	m := NewMetrics()

	m.IncCacheHits()
	m.IncCacheHits()
	m.IncCacheMisses()

	if m.CacheHitsTotal.String() != "2" {
		t.Errorf("expected 2 cache hits, got %s", m.CacheHitsTotal.String())
	}
	if m.CacheMissesTotal.String() != "1" {
		t.Errorf("expected 1 cache miss, got %s", m.CacheMissesTotal.String())
	}
}

func TestMetricsRangeReadTracking(t *testing.T) {
	m := NewMetrics()

	m.IncRangeReads()
	m.AddRangeBytesSaved(10240)

	if m.RangeReadsTotal.String() != "1" {
		t.Errorf("expected 1 range read, got %s", m.RangeReadsTotal.String())
	}
	if m.RangeBytesSavedTotal.String() != "10240" {
		t.Errorf("expected 10240 bytes saved, got %s", m.RangeBytesSavedTotal.String())
	}
}

func TestMetricsCanaryTracking(t *testing.T) {
	m := NewMetrics()

	m.IncCanaryChecks()
	m.IncCanaryChecks()
	m.IncCanaryFailures()

	if m.CanaryChecksTotal.String() != "2" {
		t.Errorf("expected 2 canary checks, got %s", m.CanaryChecksTotal.String())
	}
	if m.CanaryCheckFailures.String() != "1" {
		t.Errorf("expected 1 canary failure, got %s", m.CanaryCheckFailures.String())
	}

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	m.SetCanaryLastCheck(testTime)
	// expvar.String.String() returns JSON-encoded string with quotes
	expectedTime := `"2024-01-15T10:30:00Z"`
	if m.CanaryLastCheckTime.String() != expectedTime {
		t.Errorf("unexpected last check time: got %s, want %s", m.CanaryLastCheckTime.String(), expectedTime)
	}

	m.SetCanaryLastError("test error")
	// expvar.String returns JSON-encoded strings
	if m.CanaryLastCheckError.String() != `"test error"` {
		t.Errorf("expected error '\"test error\"', got %s", m.CanaryLastCheckError.String())
	}
}

func TestMetricsMultipartTracking(t *testing.T) {
	m := NewMetrics()

	m.IncActiveMultipartUploads()
	m.IncActiveMultipartUploads()
	m.IncMultipartPartsUploaded()
	m.DecActiveMultipartUploads()

	if m.ActiveMultipartUploads.String() != "1" {
		t.Errorf("expected 1 active multipart upload, got %s", m.ActiveMultipartUploads.String())
	}
	if m.MultipartPartsUploaded.String() != "1" {
		t.Errorf("expected 1 part uploaded, got %s", m.MultipartPartsUploaded.String())
	}
}

func TestMetricsKeyRotationTracking(t *testing.T) {
	m := NewMetrics()

	m.IncKeyRotations()
	m.AddKeyRotationObjects(100)
	m.IncKeyRotationErrors()
	m.SetKeyRotationStartTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))

	if m.KeyRotationsTotal.String() != "1" {
		t.Errorf("expected 1 key rotation, got %s", m.KeyRotationsTotal.String())
	}
	if m.KeyRotationObjects.String() != "100" {
		t.Errorf("expected 100 objects rotated, got %s", m.KeyRotationObjects.String())
	}
	if m.KeyRotationErrors.String() != "1" {
		t.Errorf("expected 1 key rotation error, got %s", m.KeyRotationErrors.String())
	}
}

func TestMetricsProvenanceTracking(t *testing.T) {
	m := NewMetrics()

	m.IncProvenanceEntries()
	m.IncProvenanceEntries()
	m.SetProvenanceChainLength(42)

	if m.ProvenanceEntriesTotal.String() != "2" {
		t.Errorf("expected 2 provenance entries, got %s", m.ProvenanceEntriesTotal.String())
	}
	if m.ProvenanceChainLength.String() != "42" {
		t.Errorf("expected chain length 42, got %s", m.ProvenanceChainLength.String())
	}
}

func TestMetricsPrometheusFormat(t *testing.T) {
	m := NewMetrics()

	// Add some data
	m.IncRequestsTotal("GET", 200)
	m.AddBytesUploaded(1024)
	m.IncCacheHits()

	output := m.PrometheusFormat()

	// Check for expected metric names
	expectedMetrics := []string{
		"armor_requests_total",
		"armor_bytes_uploaded_total",
		"armor_metadata_cache_hits_total",
		"armor_uptime_seconds",
	}

	for _, name := range expectedMetrics {
		if !strings.Contains(output, name) {
			t.Errorf("expected metric %q in Prometheus output", name)
		}
	}

	// Check for HELP and TYPE comments
	if !strings.Contains(output, "# HELP") {
		t.Error("expected HELP comments in Prometheus output")
	}
	if !strings.Contains(output, "# TYPE") {
		t.Error("expected TYPE comments in Prometheus output")
	}
}

// TestMetricsRestoreBucketGauges verifies the Phase 6a per-bucket restorability
// gauges that back the restore-age and verification-failure PrometheusRules are
// emitted with a bucket label. If these series go missing, the restorability
// alerts silently stop firing.
func TestMetricsRestoreBucketGauges(t *testing.T) {
	m := NewMetrics()

	// Empty bucket is a no-op (defensive: never emit a series with bucket="").
	m.RecordRestoreBucketState("", time.Now(), 1.0, 0)

	// Two buckets with distinct state — one healthy, one with failures.
	m.RecordRestoreBucketState("armor-apexalgo", time.Unix(1_750_000_000, 0), 1.0, 0)
	m.RecordRestoreBucketState("iad-kalshi", time.Unix(1_750_003_600, 0), 0.5, 3)

	output := m.PrometheusFormat()

	cases := []struct {
		name     string
		metric   string
		bucket   string
		contains string
	}{
		{"last_verified_ts healthy", "armor_last_verified_restore_timestamp", "armor-apexalgo", `armor_last_verified_restore_timestamp{bucket="armor-apexalgo"} 1750000000`},
		{"last_verified_ts failing", "armor_last_verified_restore_timestamp", "iad-kalshi", `armor_last_verified_restore_timestamp{bucket="iad-kalshi"} 1750003600`},
		{"object_ratio healthy", "armor_verified_object_ratio", "armor-apexalgo", `armor_verified_object_ratio{bucket="armor-apexalgo"} 1`},
		{"object_ratio failing", "armor_verified_object_ratio", "iad-kalshi", `armor_verified_object_ratio{bucket="iad-kalshi"} 0.5`},
		{"failure_count healthy", "armor_restore_verification_failures_total", "armor-apexalgo", `armor_restore_verification_failures_total{bucket="armor-apexalgo"} 0`},
		{"failure_count failing", "armor_restore_verification_failures_total", "iad-kalshi", `armor_restore_verification_failures_total{bucket="iad-kalshi"} 3`},
	}
	for _, c := range cases {
		if !strings.Contains(output, c.contains) {
			t.Errorf("%s: expected %q in Prometheus output", c.name, c.contains)
		}
	}

	// Each metric must declare HELP and TYPE so Prometheus accepts the scrape.
	for _, metric := range []string{
		"armor_last_verified_restore_timestamp",
		"armor_verified_object_ratio",
		"armor_restore_verification_failures_total",
	} {
		if !strings.Contains(output, "# HELP "+metric) {
			t.Errorf("metric %q missing HELP comment", metric)
		}
		if !strings.Contains(output, "# TYPE "+metric) {
			t.Errorf("metric %q missing TYPE comment", metric)
		}
	}
}

func TestMetricsHandler(t *testing.T) {
	m := NewMetrics()
	m.IncRequestsTotal("GET", 200)

	handler := m.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected text/plain content type, got %s", contentType)
	}

	if !strings.Contains(rec.Body.String(), "armor_requests_total") {
		t.Error("expected armor_requests_total in response body")
	}
}

func TestRequestTracker(t *testing.T) {
	rt := &RequestTracker{}

	if rt.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", rt.Count())
	}

	rt.Start()
	rt.Start()
	if rt.Count() != 2 {
		t.Errorf("expected count 2, got %d", rt.Count())
	}

	rt.End()
	if rt.Count() != 1 {
		t.Errorf("expected count 1, got %d", rt.Count())
	}

	rt.End()
	if rt.Count() != 0 {
		t.Errorf("expected count 0, got %d", rt.Count())
	}
}

func TestRequestTrackerWait(t *testing.T) {
	rt := &RequestTracker{}

	done := make(chan bool)
	started := make(chan struct{})

	// Start must complete before Wait is called to avoid race with wg.Add
	go func() {
		rt.Start()
		close(started)
		time.Sleep(50 * time.Millisecond)
		rt.End()
	}()

	go func() {
		<-started // Wait for Start to complete before calling Wait
		rt.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(200 * time.Millisecond):
		t.Error("Wait did not complete in time")
	}
}

func TestDefaultMetrics(t *testing.T) {
	if DefaultMetrics == nil {
		t.Fatal("DefaultMetrics is nil")
	}

	// Test that DefaultMetrics can be used
	DefaultMetrics.IncCacheHits()
	if DefaultMetrics.CacheHitsTotal.String() != "1" {
		t.Errorf("expected 1 cache hit, got %s", DefaultMetrics.CacheHitsTotal.String())
	}
}

func TestDefaultRequestTracker(t *testing.T) {
	if DefaultRequestTracker == nil {
		t.Fatal("DefaultRequestTracker is nil")
	}

	// Reset count
	for DefaultRequestTracker.Count() > 0 {
		DefaultRequestTracker.End()
	}

	DefaultRequestTracker.Start()
	if DefaultRequestTracker.Count() != 1 {
		t.Errorf("expected count 1, got %d", DefaultRequestTracker.Count())
	}
	DefaultRequestTracker.End()
}

func TestMultipartCanaryHistogramMetrics(t *testing.T) {
	m := NewMetrics()

	// Record some multipart upload operations
	uploadDuration := 1500 * time.Millisecond
	verifyDuration := 800 * time.Millisecond

	// Record successful operations
	m.RecordMultipartUpload("upload", "success", uploadDuration)
	m.RecordMultipartUpload("verify", "success", verifyDuration)

	// Record a failure
	m.RecordMultipartUpload("upload", "failure", 500*time.Millisecond)

	// Verify operation totals
	if m.MultipartOperationTotal == nil {
		t.Fatal("MultipartOperationTotal not initialized")
	}

	// Check upload_success count
	uploadSuccessKey := "upload_success"
	if val := m.MultipartOperationTotal.Get(uploadSuccessKey); val == nil {
		t.Error("upload_success metric not recorded")
	} else if val.(*expvar.Int).Value() != 1 {
		t.Errorf("expected upload_success count 1, got %d", val.(*expvar.Int).Value())
	}

	// Check verify_success count
	verifySuccessKey := "verify_success"
	if val := m.MultipartOperationTotal.Get(verifySuccessKey); val == nil {
		t.Error("verify_success metric not recorded")
	} else if val.(*expvar.Int).Value() != 1 {
		t.Errorf("expected verify_success count 1, got %d", val.(*expvar.Int).Value())
	}

	// Check upload_failure count
	uploadFailureKey := "upload_failure"
	if val := m.MultipartOperationTotal.Get(uploadFailureKey); val == nil {
		t.Error("upload_failure metric not recorded")
	} else if val.(*expvar.Int).Value() != 1 {
		t.Errorf("expected upload_failure count 1, got %d", val.(*expvar.Int).Value())
	}

	// Verify histogram data for upload
	if m.MultipartUploadBuckets == nil {
		t.Fatal("MultipartUploadBuckets not initialized")
	}

	// Check upload_success sum
	uploadSuccessSumKey := "upload_success_sum"
	if val := m.MultipartUploadBuckets.Get(uploadSuccessSumKey); val == nil {
		t.Error("upload_success_sum not recorded")
	} else if val.(*expvar.Int).Value() != uploadDuration.Milliseconds() {
		t.Errorf("expected upload_success_sum %d, got %d", uploadDuration.Milliseconds(), val.(*expvar.Int).Value())
	}

	// Check upload_success count
	uploadSuccessCountKey := "upload_success_count"
	if val := m.MultipartUploadBuckets.Get(uploadSuccessCountKey); val == nil {
		t.Error("upload_success_count not recorded")
	} else if val.(*expvar.Int).Value() != 1 {
		t.Errorf("expected upload_success_count 1, got %d", val.(*expvar.Int).Value())
	}

	// Verify histogram data for verify
	if m.MultipartVerificationBuckets == nil {
		t.Fatal("MultipartVerificationBuckets not initialized")
	}

	// Check verify_success sum
	verifySuccessSumKey := "verify_success_sum"
	if val := m.MultipartVerificationBuckets.Get(verifySuccessSumKey); val == nil {
		t.Error("verify_success_sum not recorded")
	} else if val.(*expvar.Int).Value() != verifyDuration.Milliseconds() {
		t.Errorf("expected verify_success_sum %d, got %d", verifyDuration.Milliseconds(), val.(*expvar.Int).Value())
	}

	// Check verify_success count
	verifySuccessCountKey := "verify_success_count"
	if val := m.MultipartVerificationBuckets.Get(verifySuccessCountKey); val == nil {
		t.Error("verify_success_count not recorded")
	} else if val.(*expvar.Int).Value() != 1 {
		t.Errorf("expected verify_success_count 1, got %d", val.(*expvar.Int).Value())
	}
}

func TestMultipartCanaryMetricsPrometheusExport(t *testing.T) {
	m := NewMetrics()

	// Record some operations
	m.RecordMultipartUpload("upload", "success", 1200*time.Millisecond)
	m.RecordMultipartUpload("verify", "success", 600*time.Millisecond)
	m.SetMultipartCanaryHealthy(true)

	// Get Prometheus format
	output := m.PrometheusFormat()

	// Check for multipart histogram metrics in Prometheus output
	expectedMetrics := []string{
		"# HELP armor_multipart_canary_upload_duration_seconds Multipart canary upload duration in seconds",
		"# TYPE armor_multipart_canary_upload_duration_seconds histogram",
		`armor_multipart_canary_upload_duration_seconds_sum{operation="upload",status="success"}`,
		`armor_multipart_canary_upload_duration_seconds_count{operation="upload",status="success"}`,
		`armor_multipart_canary_upload_duration_seconds_sum{operation="verify",status="success"}`,
		`armor_multipart_canary_upload_duration_seconds_count{operation="verify",status="success"}`,
		"armor_multipart_canary_healthy",
	}

	for _, expected := range expectedMetrics {
		if !strings.Contains(output, expected) {
			t.Errorf("expected metric %q in Prometheus output", expected)
		}
	}

	// Verify the values are approximately correct (in seconds)
	// Upload: 1200ms = 1.2 seconds
	if !strings.Contains(output, `armor_multipart_canary_upload_duration_seconds_sum{operation="upload",status="success"} 1.`) {
		t.Error("expected upload duration sum to be ~1.2 seconds in Prometheus output")
	}

	// Verify: 600ms = 0.6 seconds
	if !strings.Contains(output, `armor_multipart_canary_upload_duration_seconds_sum{operation="verify",status="success"} 0.6`) {
		t.Error("expected verify duration sum to be ~0.6 seconds in Prometheus output")
	}
}

func TestMultipartCanaryHealthStatusMetric(t *testing.T) {
	m := NewMetrics()

	// Initially should be 0 (unhealthy/unknown)
	if m.MultipartCanaryHealthy.String() != "0" {
		t.Errorf("expected initial multipart_canary_healthy to be 0, got %s", m.MultipartCanaryHealthy.String())
	}

	// Set to healthy
	m.SetMultipartCanaryHealthy(true)
	if m.MultipartCanaryHealthy.String() != "1" {
		t.Errorf("expected multipart_canary_healthy to be 1 when healthy, got %s", m.MultipartCanaryHealthy.String())
	}

	// Set to unhealthy
	m.SetMultipartCanaryHealthy(false)
	if m.MultipartCanaryHealthy.String() != "0" {
		t.Errorf("expected multipart_canary_healthy to be 0 when unhealthy, got %s", m.MultipartCanaryHealthy.String())
	}

	// Verify in Prometheus export
	output := m.PrometheusFormat()
	if !strings.Contains(output, "armor_multipart_canary_healthy") {
		t.Error("expected armor_multipart_canary_healthy in Prometheus output")
	}
}

func TestMultipartCanaryMetricsDistinctFromSmallObject(t *testing.T) {
	m := NewMetrics()

	// Record small object canary metrics
	m.IncCanaryChecks()
	m.IncCanaryFailures()
	m.SetCanaryLastCheck(time.Now())
	m.SetCanaryLastError("test error")

	// Record multipart canary metrics
	m.IncMultipartCanaryChecks()
	m.IncMultipartCanaryFailures()
	m.SetMultipartCanaryLastCheck(time.Now())
	m.SetMultipartCanaryLastError("multipart error")
	m.SetMultipartCanaryHealthy(true)
	m.RecordMultipartUpload("upload", "success", 1000*time.Millisecond)

	// Verify they are tracked separately: both counters were incremented once
	if m.CanaryChecksTotal.String() != "1" || m.MultipartCanaryChecksTotal.String() != "1" {
		t.Errorf("expected both canary counters to be 1, got small=%s multipart=%s",
			m.CanaryChecksTotal.String(), m.MultipartCanaryChecksTotal.String())
	}

	// Verify Prometheus output has both sets of metrics
	output := m.PrometheusFormat()

	// Small object metrics
	if !strings.Contains(output, "armor_canary_checks_total") {
		t.Error("expected small object canary metric in output")
	}
	if !strings.Contains(output, "armor_canary_check_failures_total") {
		t.Error("expected small object canary failures in output")
	}

	// Multipart metrics (distinct names)
	if !strings.Contains(output, "armor_multipart_canary_checks_total") {
		t.Error("expected multipart canary metric in output")
	}
	if !strings.Contains(output, "armor_multipart_canary_check_failures_total") {
		t.Error("expected multipart canary failures in output")
	}
	if !strings.Contains(output, "armor_multipart_canary_healthy") {
		t.Error("expected multipart canary healthy metric in output")
	}
	if !strings.Contains(output, "armor_multipart_canary_upload_duration_seconds") {
		t.Error("expected multipart canary duration histogram in output")
	}
}

// TestLabelledCounterAccumulation verifies that labelled counters accumulate
// properly instead of being stuck at 1 (the "Set-a-new-counter" bug).
func TestLabelledCounterAccumulation(t *testing.T) {
	m := NewMetrics()

	// Test requests by label - increment 5 times for GET_2xx
	for i := 0; i < 5; i++ {
		m.IncRequestsTotal("GET", 200)
	}

	// Increment 3 times for PUT_2xx
	for i := 0; i < 3; i++ {
		m.IncRequestsTotal("PUT", 201)
	}

	// Verify the labelled counters accumulated properly
	output := m.PrometheusFormat()

	// Should have GET_2xx with value 5, not 1
	if !strings.Contains(output, `armor_requests_by_label{key="GET_2xx"} 5`) {
		t.Errorf("expected GET_2xx counter to be 5, got stuck at 1")
	}

	// Should have PUT_2xx with value 3, not 1
	if !strings.Contains(output, `armor_requests_by_label{key="PUT_2xx"} 3`) {
		t.Errorf("expected PUT_2xx counter to be 3, got stuck at 1")
	}

	// Test encryption ops - increment 4 times for encrypt
	for i := 0; i < 4; i++ {
		m.IncEncryptionOps("encrypt")
	}

	// Test decryption ops - increment 2 times for decrypt
	for i := 0; i < 2; i++ {
		m.IncDecryptionOps("decrypt")
	}

	// Verify in Prometheus output
	output = m.PrometheusFormat()

	if !strings.Contains(output, `armor_encryption_ops_total{operation="encrypt"} 4`) {
		t.Errorf("expected encrypt counter to be 4, got stuck at 1")
	}

	if !strings.Contains(output, `armor_decryption_ops_total{operation="decrypt"} 2`) {
		t.Errorf("expected decrypt counter to be 2, got stuck at 1")
	}

	// Test backend requests - increment 3 times
	for i := 0; i < 3; i++ {
		m.IncBackendRequests("get_object")
	}

	output = m.PrometheusFormat()

	if !strings.Contains(output, `armor_backend_requests_total{operation="get_object"} 3`) {
		t.Errorf("expected get_object counter to be 3, got stuck at 1")
	}

	// Test restore verifier checks - increment 6 times for bucket1, 4 times for bucket2
	for i := 0; i < 6; i++ {
		m.RecordRestoreVerifierCheck("bucket1", 100*time.Millisecond, true)
	}
	for i := 0; i < 4; i++ {
		m.RecordRestoreVerifierCheck("bucket2", 150*time.Millisecond, true)
	}

	output = m.PrometheusFormat()

	if !strings.Contains(output, `armor_restore_verifier_checks_total{bucket="bucket1"} 6`) {
		t.Errorf("expected bucket1 checks counter to be 6, got stuck at 1")
	}

	if !strings.Contains(output, `armor_restore_verifier_checks_total{bucket="bucket2"} 4`) {
		t.Errorf("expected bucket2 checks counter to be 4, got stuck at 1")
	}

	if !strings.Contains(output, `armor_restore_verifier_objects_verified{bucket="bucket1"} 6`) {
		t.Errorf("expected bucket1 verified counter to be 6, got stuck at 1")
	}

	if !strings.Contains(output, `armor_restore_verifier_objects_verified{bucket="bucket2"} 4`) {
		t.Errorf("expected bucket2 verified counter to be 4, got stuck at 1")
	}
}

// TestRequestDurationHistogram verifies the fixed-bucket histogram implementation
func TestRequestDurationHistogram(t *testing.T) {
	m := NewMetrics()

	// Record some durations across different operations
	durations := []struct {
		operation string
		duration  time.Duration
	}{
		{"PutObject", 7 * time.Millisecond},   // falls in 10ms bucket
		{"PutObject", 45 * time.Millisecond},  // falls in 50ms bucket
		{"PutObject", 150 * time.Millisecond}, // falls in 250ms bucket
		{"PutObject", 3000 * time.Millisecond}, // falls in 5000ms bucket
		{"GetObject", 20 * time.Millisecond},   // falls in 25ms bucket
		{"GetObject", 800 * time.Millisecond},   // falls in 1000ms bucket
		{"GetObject", 1500 * time.Millisecond}, // falls in 2500ms bucket
	}

	for _, d := range durations {
		m.RecordRequestDuration(d.operation, d.duration)
	}

	output := m.PrometheusFormat()

	// Verify histogram structure is present
	expectedMetricBases := []string{
		"armor_request_duration_ms_bucket{operation=\"PutObject\",le=",
		"armor_request_duration_ms_bucket{operation=\"GetObject\",le=",
		"armor_request_duration_ms_sum{operation=",
		"armor_request_duration_ms_count{operation=",
	}

	for _, base := range expectedMetricBases {
		if !strings.Contains(output, base) {
			t.Errorf("expected histogram metric base %q in Prometheus output", base)
		}
	}

	// Verify HELP and TYPE comments
	if !strings.Contains(output, "# HELP armor_request_duration_ms") {
		t.Error("expected HELP comment for request duration histogram")
	}
	if !strings.Contains(output, "# TYPE armor_request_duration_ms histogram") {
		t.Error("expected TYPE histogram for request duration")
	}
}

// TestRequestDurationHistogramBucketMonotonicity verifies that histogram bucket
// values are monotonically increasing (cumulative histogram property).
// This is critical for Prometheus histograms to work correctly.
func TestRequestDurationHistogramBucketMonotonicity(t *testing.T) {
	m := NewMetrics()

	// Record multiple observations for the same operation
	observations := []time.Duration{
		5 * time.Millisecond,
		15 * time.Millisecond,
		30 * time.Millisecond,
		75 * time.Millisecond,
		200 * time.Millisecond,
		600 * time.Millisecond,
		1500 * time.Millisecond,
		3500 * time.Millisecond,
		8000 * time.Millisecond,
	}

	for _, obs := range observations {
		m.RecordRequestDuration("PutObject", obs)
	}

	output := m.PrometheusFormat()

	// Extract bucket values for PutObject operation using string parsing
	// instead of complex regex
	lines := strings.Split(output, "\n")

	bucketValues := make(map[string]int64)
	for _, line := range lines {
		if strings.HasPrefix(line, "armor_request_duration_ms_bucket{operation=\"PutObject\"") {
			// Extract the le value and count
			// Format: armor_request_duration_ms_bucket{operation="PutObject",le="5"} 0
			parts := strings.Split(line, `le="`)
			if len(parts) == 2 {
				lePart := strings.Split(parts[1], `"} `)
				if len(lePart) == 2 {
					le := lePart[0]
					countStr := lePart[1]
					count, err := strconv.ParseInt(countStr, 10, 64)
					if err == nil {
						bucketValues[le] = count
					}
				}
			}
		}
	}

	// Expected buckets in order
	orderedBuckets := []string{"5", "10", "25", "50", "100", "250", "500", "1000", "2500", "5000", "10000", "+Inf"}
	var values []int64
	for _, le := range orderedBuckets {
		val, ok := bucketValues[le]
		if !ok {
			t.Errorf("missing bucket value for le=%s", le)
			continue
		}
		values = append(values, val)
	}

	// Verify monotonicity: each bucket should be >= the previous
	for i := 1; i < len(values); i++ {
		if values[i] < values[i-1] {
			t.Errorf("bucket monotonicity violated: bucket[%d]=%d < bucket[%d]=%d",
				i, values[i], i-1, values[i-1])
		}
	}

	// Verify +Inf bucket equals total count
	countPattern := `armor_request_duration_ms_count{operation="PutObject"}`
	for _, line := range lines {
		if strings.Contains(line, countPattern) {
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				expectedCount, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
				if err == nil && len(values) > 0 {
					infCount := values[len(values)-1]
					if infCount != expectedCount {
						t.Errorf("+Inf bucket count (%d) should equal total count (%d)", infCount, expectedCount)
					}
				}
			}
			break
		}
	}
}

// TestRequestDurationHistogramNoUnboundedKeys verifies that the old
// unbounded bucket pattern (operation_bucket_le_<exact_millis>) is not
// present in the exported metrics.
func TestRequestDurationHistogramNoUnboundedKeys(t *testing.T) {
	m := NewMetrics()

	// Record some durations with specific millis values that would have
	// created unbounded keys in the old implementation
	m.RecordRequestDuration("PutObject", 123*time.Millisecond)  // would create "PutObject_bucket_le_123"
	m.RecordRequestDuration("GetObject", 456*time.Millisecond) // would create "GetObject_bucket_le_456"

	output := m.PrometheusFormat()

	// Verify that requestsByLabel does NOT contain unbounded bucket keys
	hasUnboundedKeys := false
	m.requestsByLabel.Do(func(kv expvar.KeyValue) {
		key := kv.Key
		// Check if key matches the old pattern: <operation>_bucket_le_<number>
		if strings.Contains(key, "_bucket_le_") {
			// Extract the part after _bucket_le_
			parts := strings.Split(key, "_bucket_le_")
			if len(parts) == 2 {
				// Try to parse as number - if it succeeds, it's an unbounded key
				if _, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					hasUnboundedKeys = true
				}
			}
		}
	})

	if hasUnboundedKeys {
		t.Error("found unbounded bucket keys in requestsByLabel - old pattern should be removed")
	}

	// Verify the new histogram format is used instead
	if !strings.Contains(output, "armor_request_duration_ms_bucket") {
		t.Error("expected new histogram bucket format in Prometheus output")
	}
	if !strings.Contains(output, "armor_request_duration_ms_sum") {
		t.Error("expected new histogram sum metric in Prometheus output")
	}
	if !strings.Contains(output, "armor_request_duration_ms_count") {
		t.Error("expected new histogram count metric in Prometheus output")
	}
}

// TestRequestDurationHistogramAccuracy verifies that sum and count are accurate
func TestRequestDurationHistogramAccuracy(t *testing.T) {
	m := NewMetrics()

	// Record specific durations
	durations := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
	}

	expectedSum := int64(0)
	for _, d := range durations {
		m.RecordRequestDuration("TestOp", d)
		expectedSum += d.Milliseconds()
	}

	output := m.PrometheusFormat()

	// Verify count
	countPattern := `armor_request_duration_ms_count{operation="TestOp"} (\d+)`
	countRe := regexp.MustCompile(countPattern)
	countMatches := countRe.FindStringSubmatch(output)
	if len(countMatches) < 2 {
		t.Fatal("could not find count metric for TestOp")
	}
	actualCount, _ := strconv.ParseInt(countMatches[1], 10, 64)
	expectedCount := int64(len(durations))
	if actualCount != expectedCount {
		t.Errorf("count mismatch: got %d, want %d", actualCount, expectedCount)
	}

	// Verify sum
	sumPattern := `armor_request_duration_ms_sum{operation="TestOp"} (\d+)`
	sumRe := regexp.MustCompile(sumPattern)
	sumMatches := sumRe.FindStringSubmatch(output)
	if len(countMatches) < 2 {
		t.Fatal("could not find sum metric for TestOp")
	}
	actualSum, _ := strconv.ParseInt(sumMatches[1], 10, 64)
	if actualSum != expectedSum {
		t.Errorf("sum mismatch: got %d, want %d", actualSum, expectedSum)
	}
}

// TestRequestDurationHistogramMultipleOperations verifies that multiple
// operations are tracked independently
func TestRequestDurationHistogramMultipleOperations(t *testing.T) {
	m := NewMetrics()

	// Record durations for different operations
	operations := []string{"PutObject", "GetObject", "DeleteObject", "ListObjectsV2"}
	for _, op := range operations {
		m.RecordRequestDuration(op, 100*time.Millisecond)
	}

	output := m.PrometheusFormat()

	// Verify each operation has its own histogram
	for _, op := range operations {
		// Check for sum metric
		sumPattern := fmt.Sprintf(`armor_request_duration_ms_sum{operation=%q}`, op)
		if !strings.Contains(output, sumPattern) {
			t.Errorf("missing sum metric for operation %s", op)
		}

		// Check for count metric
		countPattern := fmt.Sprintf(`armor_request_duration_ms_count{operation=%q}`, op)
		if !strings.Contains(output, countPattern) {
			t.Errorf("missing count metric for operation %s", op)
		}

		// Check for bucket metrics
		bucketPattern := fmt.Sprintf(`armor_request_duration_ms_bucket{operation=%q,le=`, op)
		if !strings.Contains(output, bucketPattern) {
			t.Errorf("missing bucket metrics for operation %s", op)
		}
	}
}
