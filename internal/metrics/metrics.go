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

	// Internal state
	startTime time.Time
	mu        sync.Mutex
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
			sb.WriteString(fmt.Sprintf("armor_%s %q\n", name, v.String()))
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

	// Uptime
	uptime := time.Since(m.startTime).Seconds()
	sb.WriteString("# HELP armor_uptime_seconds Server uptime in seconds\n")
	sb.WriteString("# TYPE armor_uptime_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("armor_uptime_seconds %.2f\n", uptime))

	return sb.String()
}

// Handler returns an HTTP handler for Prometheus metrics.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.Write([]byte(m.PrometheusFormat()))
	}
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
