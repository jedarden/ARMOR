// Package metrics provides Prometheus metrics for ARMOR.
package metrics

import (
	"expvar"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds all ARMOR metrics.
type Metrics struct {
	// Request metrics
	RequestsTotal         *expvar.Int
	RequestsInFlight      *expvar.Int
	RequestDurationMillis *expvar.Map

	// Data transfer metrics
	BytesUploaded        *expvar.Int
	BytesDownloaded      *expvar.Int
	BytesFetchedFromB2   *expvar.Int
	RangeReadsTotal      *expvar.Int
	RangeBytesSavedTotal *expvar.Int

	// Cache metrics
	CacheHitsTotal   *expvar.Int
	CacheMissesTotal *expvar.Int

	// Encryption metrics
	EncryptionOpsTotal   *expvar.Map
	DecryptionOpsTotal   *expvar.Map
	KeyWrapOpsTotal      *expvar.Int
	KeyUnwrapOpsTotal    *expvar.Int

	// Canary metrics
	CanaryChecksTotal    *expvar.Int
	CanaryCheckFailures  *expvar.Int
	CanaryLastCheckTime  *expvar.String
	CanaryLastCheckError *expvar.String

	// Multipart canary metrics
	MultipartCanaryChecksTotal    *expvar.Int
	MultipartCanaryCheckFailures  *expvar.Int
	MultipartCanaryLastCheckTime  *expvar.String
	MultipartCanaryLastCheckError *expvar.String
	MultipartCanaryHealthy        *expvar.Int

	// Multipart histogram metrics (bucketed by operation and status)
	MultipartUploadBuckets    *expvar.Map // Histogram buckets: upload operation, keyed by latency
	MultipartVerificationBuckets *expvar.Map // Histogram buckets: verification operation, keyed by latency
	MultipartOperationTotal    *expvar.Map // Counter by operation and status: operation_status

	// Multipart metrics
	ActiveMultipartUploads *expvar.Int
	MultipartPartsUploaded *expvar.Int

	// Key rotation metrics
	KeyRotationsTotal    *expvar.Int
	KeyRotationObjects   *expvar.Int
	KeyRotationErrors    *expvar.Int
	KeyRotationStartTime *expvar.String

	// Provenance metrics
	ProvenanceEntriesTotal *expvar.Int
	ProvenanceChainLength  *expvar.Int

	// Backend metrics
	BackendRequestsTotal   *expvar.Map
	BackendRequestDuration *expvar.Map

	// Restore verifier metrics (Phase 6)
	RestoreVerifierLastCheckTime   *expvar.String
	RestoreVerifierLastCheckError  *expvar.String
	RestoreVerifierChecksTotal     *expvar.Map
	RestoreVerifierFailuresTotal   *expvar.Map
	RestoreVerifierObjectsVerified *expvar.Map
	RestoreVerifierObjectsFailed   *expvar.Map
	RestoreVerifierLatencyMillis   *expvar.Map

	// Restore verifier per-bucket gauges (Phase 6a — restorability alerting).
	// Each map is keyed by bucket name so PrometheusFormat can emit one labeled
	// series per bucket. These back the restorability PrometheusRule:
	// armor_last_verified_restore_timestamp, armor_verified_object_ratio, and
	// armor_restore_verification_failures_total.
	RestoreVerifierLastVerifiedTs *expvar.Map // bucket -> last verification time (unix seconds)
	RestoreVerifierObjectRatio    *expvar.Map // bucket -> verified/total ratio (0..1)
	RestoreVerifierFailureCount   *expvar.Map // bucket -> failed object count

	// Internal state
	startTime time.Time
}

// DefaultMetrics is the default metrics instance.
var DefaultMetrics = NewMetrics()

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	m := &Metrics{
		startTime: time.Now(),
	}

	// Request metrics
	m.RequestsTotal = new(expvar.Int)
	m.RequestsInFlight = new(expvar.Int)
	m.RequestDurationMillis = new(expvar.Map).Init()

	// Data transfer metrics
	m.BytesUploaded = new(expvar.Int)
	m.BytesDownloaded = new(expvar.Int)
	m.BytesFetchedFromB2 = new(expvar.Int)
	m.RangeReadsTotal = new(expvar.Int)
	m.RangeBytesSavedTotal = new(expvar.Int)

	// Cache metrics
	m.CacheHitsTotal = new(expvar.Int)
	m.CacheMissesTotal = new(expvar.Int)

	// Encryption metrics
	m.EncryptionOpsTotal = new(expvar.Map).Init()
	m.DecryptionOpsTotal = new(expvar.Map).Init()
	m.KeyWrapOpsTotal = new(expvar.Int)
	m.KeyUnwrapOpsTotal = new(expvar.Int)

	// Canary metrics
	m.CanaryChecksTotal = new(expvar.Int)
	m.CanaryCheckFailures = new(expvar.Int)
	m.CanaryLastCheckTime = new(expvar.String)
	m.CanaryLastCheckError = new(expvar.String)

	// Multipart canary metrics
	m.MultipartCanaryChecksTotal = new(expvar.Int)
	m.MultipartCanaryCheckFailures = new(expvar.Int)
	m.MultipartCanaryLastCheckTime = new(expvar.String)
	m.MultipartCanaryLastCheckError = new(expvar.String)
	m.MultipartCanaryHealthy = new(expvar.Int)

	// Multipart histogram metrics
	m.MultipartUploadBuckets = new(expvar.Map).Init()
	m.MultipartVerificationBuckets = new(expvar.Map).Init()
	m.MultipartOperationTotal = new(expvar.Map).Init()

	// Multipart metrics
	m.ActiveMultipartUploads = new(expvar.Int)
	m.MultipartPartsUploaded = new(expvar.Int)

	// Key rotation metrics
	m.KeyRotationsTotal = new(expvar.Int)
	m.KeyRotationObjects = new(expvar.Int)
	m.KeyRotationErrors = new(expvar.Int)
	m.KeyRotationStartTime = new(expvar.String)

	// Provenance metrics
	m.ProvenanceEntriesTotal = new(expvar.Int)
	m.ProvenanceChainLength = new(expvar.Int)

	// Backend metrics
	m.BackendRequestsTotal = new(expvar.Map).Init()
	m.BackendRequestDuration = new(expvar.Map).Init()

	// Restore verifier metrics
	m.RestoreVerifierLastCheckTime = new(expvar.String)
	m.RestoreVerifierLastCheckError = new(expvar.String)
	m.RestoreVerifierChecksTotal = new(expvar.Map).Init()
	m.RestoreVerifierFailuresTotal = new(expvar.Map).Init()
	m.RestoreVerifierObjectsVerified = new(expvar.Map).Init()
	m.RestoreVerifierObjectsFailed = new(expvar.Map).Init()
	m.RestoreVerifierLatencyMillis = new(expvar.Map).Init()

	// Restore verifier per-bucket gauges (Phase 6a)
	m.RestoreVerifierLastVerifiedTs = new(expvar.Map).Init()
	m.RestoreVerifierObjectRatio = new(expvar.Map).Init()
	m.RestoreVerifierFailureCount = new(expvar.Map).Init()

	return m
}

// IncRequestsTotal increments the request counter for an operation and status.
func (m *Metrics) IncRequestsTotal(operation string, status int) {
	key := fmt.Sprintf("%s_%dxx", operation, status/100)
	m.RequestsTotal.Add(1)
	// Track by operation and status class
	var counter expvar.Int
	counter.Add(1)
	m.RequestDurationMillis.Set(key, &counter)
}

// IncRequestsInFlight increments the in-flight request counter.
func (m *Metrics) IncRequestsInFlight() {
	m.RequestsInFlight.Add(1)
}

// DecRequestsInFlight decrements the in-flight request counter.
func (m *Metrics) DecRequestsInFlight() {
	m.RequestsInFlight.Add(-1)
}

// RecordRequestDuration records the duration of a request.
func (m *Metrics) RecordRequestDuration(operation string, duration time.Duration) {
	key := operation
	millis := duration.Milliseconds()
	// Store as a histogram bucket approximation
	bucket := fmt.Sprintf("%s_bucket_le_%d", key, millis)
	var counter expvar.Int
	counter.Add(1)
	m.RequestDurationMillis.Set(bucket, &counter)
}

// AddBytesUploaded adds to the uploaded bytes counter.
func (m *Metrics) AddBytesUploaded(n int64) {
	m.BytesUploaded.Add(n)
}

// AddBytesDownloaded adds to the downloaded bytes counter.
func (m *Metrics) AddBytesDownloaded(n int64) {
	m.BytesDownloaded.Add(n)
}

// AddBytesFetchedFromB2 adds to the bytes fetched from B2 counter.
func (m *Metrics) AddBytesFetchedFromB2(n int64) {
	m.BytesFetchedFromB2.Add(n)
}

// IncRangeReads increments the range read counter.
func (m *Metrics) IncRangeReads() {
	m.RangeReadsTotal.Add(1)
}

// AddRangeBytesSaved adds to the bytes saved by range reads counter.
func (m *Metrics) AddRangeBytesSaved(n int64) {
	m.RangeBytesSavedTotal.Add(n)
}

// IncCacheHits increments the cache hit counter.
func (m *Metrics) IncCacheHits() {
	m.CacheHitsTotal.Add(1)
}

// IncCacheMisses increments the cache miss counter.
func (m *Metrics) IncCacheMisses() {
	m.CacheMissesTotal.Add(1)
}

// IncEncryptionOps increments the encryption operations counter.
func (m *Metrics) IncEncryptionOps(opType string) {
	var counter expvar.Int
	counter.Add(1)
	m.EncryptionOpsTotal.Set(opType, &counter)
}

// IncDecryptionOps increments the decryption operations counter.
func (m *Metrics) IncDecryptionOps(opType string) {
	var counter expvar.Int
	counter.Add(1)
	m.DecryptionOpsTotal.Set(opType, &counter)
}

// IncKeyWrap increments the key wrap counter.
func (m *Metrics) IncKeyWrap() {
	m.KeyWrapOpsTotal.Add(1)
}

// IncKeyUnwrap increments the key unwrap counter.
func (m *Metrics) IncKeyUnwrap() {
	m.KeyUnwrapOpsTotal.Add(1)
}

// IncCanaryChecks increments the canary check counter.
func (m *Metrics) IncCanaryChecks() {
	m.CanaryChecksTotal.Add(1)
}

// IncCanaryFailures increments the canary failure counter.
func (m *Metrics) IncCanaryFailures() {
	m.CanaryCheckFailures.Add(1)
}

// SetCanaryLastCheck sets the last canary check time.
func (m *Metrics) SetCanaryLastCheck(t time.Time) {
	m.CanaryLastCheckTime.Set(t.UTC().Format(time.RFC3339))
}

// SetCanaryLastError sets the last canary error.
func (m *Metrics) SetCanaryLastError(err string) {
	m.CanaryLastCheckError.Set(err)
}

// IncMultipartCanaryChecks increments the multipart canary check counter.
func (m *Metrics) IncMultipartCanaryChecks() {
	m.MultipartCanaryChecksTotal.Add(1)
}

// IncMultipartCanaryFailures increments the multipart canary failure counter.
func (m *Metrics) IncMultipartCanaryFailures() {
	m.MultipartCanaryCheckFailures.Add(1)
}

// SetMultipartCanaryLastCheck sets the last multipart canary check time.
func (m *Metrics) SetMultipartCanaryLastCheck(t time.Time) {
	m.MultipartCanaryLastCheckTime.Set(t.UTC().Format(time.RFC3339))
}

// SetMultipartCanaryLastError sets the last multipart canary error.
func (m *Metrics) SetMultipartCanaryLastError(err string) {
	m.MultipartCanaryLastCheckError.Set(err)
}

// SetMultipartCanaryHealthy sets the multipart canary health status (1 = healthy, 0 = unhealthy).
func (m *Metrics) SetMultipartCanaryHealthy(healthy bool) {
	if healthy {
		m.MultipartCanaryHealthy.Set(1)
	} else {
		m.MultipartCanaryHealthy.Set(0)
	}
}

// RecordMultipartUpload records the completion time of a multipart upload operation.
// operation should be "upload" or "verify"
// status should be "success" or "failure"
func (m *Metrics) RecordMultipartUpload(operation string, status string, duration time.Duration) {
	millis := duration.Milliseconds()

	// Create a composite key for operation+status combination
	opStatusKey := fmt.Sprintf("%s_%s", operation, status)

	// Track total count for this operation/status
	var counter expvar.Int
	counter.Add(1)
	m.MultipartOperationTotal.Set(opStatusKey, &counter)

	// Track sum and count in the appropriate histogram map
	var histogramMap *expvar.Map
	switch operation {
	case "upload":
		histogramMap = m.MultipartUploadBuckets
	case "verify":
		histogramMap = m.MultipartVerificationBuckets
	default:
		return // Invalid operation
	}

	// Store sum: multipart_upload_sum_success, multipart_upload_sum_failure
	sumKey := fmt.Sprintf("%s_%s", opStatusKey, "sum")
	var currentSum expvar.Int
	if existingSum := histogramMap.Get(sumKey); existingSum != nil {
		currentSum.Set(existingSum.(*expvar.Int).Value() + int64(millis))
	} else {
		currentSum.Set(int64(millis))
	}
	histogramMap.Set(sumKey, &currentSum)

	// Store count
	countKey := fmt.Sprintf("%s_%s", opStatusKey, "count")
	var currentCount expvar.Int
	if existingCount := histogramMap.Get(countKey); existingCount != nil {
		currentCount.Set(existingCount.(*expvar.Int).Value() + 1)
	} else {
		currentCount.Set(1)
	}
	histogramMap.Set(countKey, &currentCount)

	// Store last value for monitoring
	lastKey := fmt.Sprintf("%s_%s", opStatusKey, "last_millis")
	var lastVal expvar.Int
	lastVal.Set(millis)
	histogramMap.Set(lastKey, &lastVal)
}

// IncActiveMultipartUploads increments the active multipart upload counter.
func (m *Metrics) IncActiveMultipartUploads() {
	m.ActiveMultipartUploads.Add(1)
}

// DecActiveMultipartUploads decrements the active multipart upload counter.
func (m *Metrics) DecActiveMultipartUploads() {
	m.ActiveMultipartUploads.Add(-1)
}

// IncMultipartPartsUploaded increments the multipart parts counter.
func (m *Metrics) IncMultipartPartsUploaded() {
	m.MultipartPartsUploaded.Add(1)
}

// IncKeyRotations increments the key rotation counter.
func (m *Metrics) IncKeyRotations() {
	m.KeyRotationsTotal.Add(1)
}

// AddKeyRotationObjects adds to the key rotation objects counter.
func (m *Metrics) AddKeyRotationObjects(n int64) {
	m.KeyRotationObjects.Add(n)
}

// IncKeyRotationErrors increments the key rotation error counter.
func (m *Metrics) IncKeyRotationErrors() {
	m.KeyRotationErrors.Add(1)
}

// SetKeyRotationStartTime sets the key rotation start time.
func (m *Metrics) SetKeyRotationStartTime(t time.Time) {
	m.KeyRotationStartTime.Set(t.UTC().Format(time.RFC3339))
}

// IncProvenanceEntries increments the provenance entries counter.
func (m *Metrics) IncProvenanceEntries() {
	m.ProvenanceEntriesTotal.Add(1)
}

// SetProvenanceChainLength sets the provenance chain length.
func (m *Metrics) SetProvenanceChainLength(n int64) {
	m.ProvenanceChainLength.Set(n)
}

// IncBackendRequests increments the backend request counter.
func (m *Metrics) IncBackendRequests(operation string) {
	var counter expvar.Int
	counter.Add(1)
	m.BackendRequestsTotal.Set(operation, &counter)
}

// RecordBackendRequestDuration records the duration of a backend request.
func (m *Metrics) RecordBackendRequestDuration(operation string, duration time.Duration) {
	key := operation
	millis := duration.Milliseconds()
	bucket := fmt.Sprintf("%s_duration_%d", key, millis)
	var counter expvar.Int
	counter.Add(1)
	m.BackendRequestDuration.Set(bucket, &counter)
}

// PrometheusFormat returns metrics in Prometheus text format.
func (m *Metrics) PrometheusFormat() string {
	var sb strings.Builder

	// Helper to write a metric
	writeMetric := func(name, help, metricType string, value expvar.Var) {
		fmt.Fprintf(&sb, "# HELP armor_%s %s\n", name, help)
		fmt.Fprintf(&sb, "# TYPE armor_%s %s\n", name, metricType)
		switch v := value.(type) {
		case *expvar.Int:
			fmt.Fprintf(&sb, "armor_%s %s\n", name, v.String())
		case *expvar.String:
			fmt.Fprintf(&sb, "armor_%s %q\n", name, v.String())
		}
	}

	// Request metrics
	writeMetric("requests_total", "Total number of requests", "counter", m.RequestsTotal)
	writeMetric("requests_in_flight", "Number of requests currently being processed", "gauge", m.RequestsInFlight)
	writeMetric("bytes_uploaded_total", "Total plaintext bytes uploaded by clients", "counter", m.BytesUploaded)
	writeMetric("bytes_downloaded_total", "Total plaintext bytes downloaded by clients", "counter", m.BytesDownloaded)
	writeMetric("bytes_fetched_from_b2_total", "Total ciphertext bytes fetched from B2/Cloudflare", "counter", m.BytesFetchedFromB2)
	writeMetric("range_reads_total", "Total number of range read requests", "counter", m.RangeReadsTotal)
	writeMetric("range_bytes_saved_total", "Bytes NOT transferred due to range reads", "counter", m.RangeBytesSavedTotal)

	// Cache metrics
	writeMetric("metadata_cache_hits_total", "Total number of metadata cache hits", "counter", m.CacheHitsTotal)
	writeMetric("metadata_cache_misses_total", "Total number of metadata cache misses", "counter", m.CacheMissesTotal)

	// Encryption metrics
	writeMetric("key_wrap_ops_total", "Total number of key wrap operations", "counter", m.KeyWrapOpsTotal)
	writeMetric("key_unwrap_ops_total", "Total number of key unwrap operations", "counter", m.KeyUnwrapOpsTotal)

	// Canary metrics
	writeMetric("canary_checks_total", "Total number of canary checks", "counter", m.CanaryChecksTotal)
	writeMetric("canary_check_failures_total", "Total number of canary check failures", "counter", m.CanaryCheckFailures)
	writeMetric("canary_last_check_time", "Time of last canary check", "gauge", m.CanaryLastCheckTime)
	writeMetric("canary_last_check_error", "Error from last failed canary check", "gauge", m.CanaryLastCheckError)

	// Multipart canary metrics
	writeMetric("multipart_canary_checks_total", "Total number of multipart canary checks", "counter", m.MultipartCanaryChecksTotal)
	writeMetric("multipart_canary_check_failures_total", "Total number of multipart canary check failures", "counter", m.MultipartCanaryCheckFailures)
	writeMetric("multipart_canary_last_check_time", "Time of last multipart canary check", "gauge", m.MultipartCanaryLastCheckTime)
	writeMetric("multipart_canary_last_check_error", "Error from last failed multipart canary check", "gauge", m.MultipartCanaryLastCheckError)
	writeMetric("multipart_canary_healthy", "Multipart canary health status (1=healthy, 0=unhealthy)", "gauge", m.MultipartCanaryHealthy)

	// Multipart metrics
	writeMetric("active_multipart_uploads", "Number of in-progress multipart uploads", "gauge", m.ActiveMultipartUploads)
	writeMetric("multipart_parts_uploaded_total", "Total number of multipart parts uploaded", "counter", m.MultipartPartsUploaded)

	// Key rotation metrics
	writeMetric("key_rotations_total", "Total number of key rotations", "counter", m.KeyRotationsTotal)
	writeMetric("key_rotation_objects_total", "Total number of objects processed during key rotations", "counter", m.KeyRotationObjects)
	writeMetric("key_rotation_errors_total", "Total number of key rotation errors", "counter", m.KeyRotationErrors)
	writeMetric("key_rotation_start_time", "Start time of last key rotation", "gauge", m.KeyRotationStartTime)

	// Provenance metrics
	writeMetric("provenance_entries_total", "Total number of provenance entries recorded", "counter", m.ProvenanceEntriesTotal)
	writeMetric("provenance_chain_length", "Length of the provenance chain for this writer", "gauge", m.ProvenanceChainLength)

	// Multipart canary histogram metrics
	// Export multipart upload duration histogram
	fmt.Fprintf(&sb, "\n# HELP armor_multipart_canary_upload_duration_seconds Multipart canary upload duration in seconds\n")
	fmt.Fprintf(&sb, "# TYPE armor_multipart_canary_upload_duration_seconds histogram\n")
	for _, opStatus := range []string{"upload_success", "upload_failure", "verify_success", "verify_failure"} {
		parts := strings.Split(opStatus, "_")
		operation := parts[0]
		status := parts[1]

		// Get the appropriate map
		var histogramMap *expvar.Map
		switch operation {
		case "upload":
			histogramMap = m.MultipartUploadBuckets
		case "verify":
			histogramMap = m.MultipartVerificationBuckets
		default:
			continue
		}

		sumKey := fmt.Sprintf("%s_sum", opStatus)
		countKey := fmt.Sprintf("%s_count", opStatus)
		lastKey := fmt.Sprintf("%s_last_millis", opStatus)

		sum := histogramMap.Get(sumKey)
		count := histogramMap.Get(countKey)
		last := histogramMap.Get(lastKey)

		if count != nil && count.(*expvar.Int).Value() > 0 {
			sumVal := int64(0)
			if sum != nil {
				sumVal = sum.(*expvar.Int).Value()
			}
			countVal := count.(*expvar.Int).Value()
			lastVal := int64(0)
			if last != nil {
				lastVal = last.(*expvar.Int).Value()
			}

			// Export as seconds
			fmt.Fprintf(&sb, "armor_multipart_canary_upload_duration_seconds_sum{operation=\"%s\",status=\"%s\"} %.6f\n", operation, status, float64(sumVal)/1000.0)
			fmt.Fprintf(&sb, "armor_multipart_canary_upload_duration_seconds_count{operation=\"%s\",status=\"%s\"} %d\n", operation, status, countVal)
			fmt.Fprintf(&sb, "armor_multipart_canary_upload_duration_seconds_last{operation=\"%s\",status=\"%s\"} %.6f\n", operation, status, float64(lastVal)/1000.0)
		}
	}

	// Restore verifier per-bucket gauges (Phase 6a — restorability alerting).
	// One labeled series per bucket drives the restore-age and verification-failure
	// PrometheusRules. Emitted manually (like the multipart histogram above)
	// because the writeMetric helper only handles scalar Int/String vars, not the
	// bucket-labeled maps.
	sb.WriteString("\n# HELP armor_last_verified_restore_timestamp Unix timestamp of the most recent verification attempt per bucket\n")
	sb.WriteString("# TYPE armor_last_verified_restore_timestamp gauge\n")
	m.RestoreVerifierLastVerifiedTs.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_last_verified_restore_timestamp{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

	sb.WriteString("# HELP armor_verified_object_ratio Ratio of verified objects to total objects sampled per bucket (0..1)\n")
	sb.WriteString("# TYPE armor_verified_object_ratio gauge\n")
	m.RestoreVerifierObjectRatio.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_verified_object_ratio{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

	sb.WriteString("# HELP armor_restore_verification_failures_total Number of objects that failed verification per bucket\n")
	sb.WriteString("# TYPE armor_restore_verification_failures_total counter\n")
	m.RestoreVerifierFailureCount.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_restore_verification_failures_total{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

	// Uptime
	uptime := time.Since(m.startTime).Seconds()
	sb.WriteString("# HELP armor_uptime_seconds Server uptime in seconds\n")
	sb.WriteString("# TYPE armor_uptime_seconds gauge\n")
	fmt.Fprintf(&sb, "armor_uptime_seconds %.2f\n", uptime)

	return sb.String()
}

// Handler returns an HTTP handler for Prometheus metrics.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.Write([]byte(m.PrometheusFormat()))
	}
}

// RecordRestoreVerifierCheck records a restore verifier check completion.
func (m *Metrics) RecordRestoreVerifierCheck(bucket string, duration time.Duration, success bool) {
	var counter expvar.Int
	counter.Add(1)
	m.RestoreVerifierChecksTotal.Set(bucket, &counter)

	if success {
		m.RestoreVerifierObjectsVerified.Set(bucket, &counter)
	} else {
		m.RestoreVerifierFailuresTotal.Set(bucket, &counter)
		m.RestoreVerifierObjectsFailed.Set(bucket, &counter)
	}

	latencyKey := fmt.Sprintf("%s_latency", bucket)
	var latency expvar.Int
	latency.Set(int64(duration.Milliseconds()))
	m.RestoreVerifierLatencyMillis.Set(latencyKey, &latency)
}

// SetRestoreVerifierLastCheckTime sets the last check time for restore verifier.
func (m *Metrics) SetRestoreVerifierLastCheckTime(t time.Time) {
	m.RestoreVerifierLastCheckTime.Set(t.Format(time.RFC3339))
}

// SetRestoreVerifierLastError sets the last error for restore verifier.
func (m *Metrics) SetRestoreVerifierLastError(err string) {
	m.RestoreVerifierLastCheckError.Set(err)
}

// RecordRestoreBucketState publishes the per-bucket restorability gauges that
// back the restore-age and verification-failure PrometheusRules. lastVerified is
// the time of this verification attempt (success or failure) so the
// restore-age alert advances on every run; ratio is verified/total in [0,1];
// failures is the count of objects that failed verification this run (and is
// exported as a counter so any non-zero value trips the failure alert).
func (m *Metrics) RecordRestoreBucketState(bucket string, lastVerified time.Time, ratio float64, failures int64) {
	if bucket == "" {
		return
	}

	var ts expvar.Int
	ts.Set(lastVerified.Unix())
	m.RestoreVerifierLastVerifiedTs.Set(bucket, &ts)

	var r expvar.Float
	r.Set(ratio)
	m.RestoreVerifierObjectRatio.Set(bucket, &r)

	var fc expvar.Int
	fc.Set(failures)
	m.RestoreVerifierFailureCount.Set(bucket, &fc)
}

// RequestTracker tracks in-flight requests using a WaitGroup.
type RequestTracker struct {
	wg    sync.WaitGroup
	count atomic.Int64
}

// Start begins tracking a request.
func (rt *RequestTracker) Start() {
	rt.wg.Add(1)
	rt.count.Add(1)
}

// End marks a request as complete.
func (rt *RequestTracker) End() {
	rt.wg.Done()
	rt.count.Add(-1)
}

// Wait waits for all in-flight requests to complete.
func (rt *RequestTracker) Wait() {
	rt.wg.Wait()
}

// Count returns the current number of in-flight requests.
func (rt *RequestTracker) Count() int64 {
	return rt.count.Load()
}

// DefaultRequestTracker is the default request tracker.
var DefaultRequestTracker = &RequestTracker{}

// StartTime returns when the metrics were initialized.
func (m *Metrics) StartTime() time.Time {
	return m.startTime
}
