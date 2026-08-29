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

// labelledCounter manages per-label counters with proper increment semantics.
// Unlike expvar.Map's Set() which replaces values, this ensures counters accumulate.
type labelledCounter struct {
	mu sync.Mutex
	m  *expvar.Map
}

func newLabelledCounter() *labelledCounter {
	return &labelledCounter{
		m: new(expvar.Map).Init(),
	}
}

// Add increments the counter for a given label set by delta.
func (lc *labelledCounter) Add(key string, delta int64) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Get existing counter or create new one
	if v := lc.m.Get(key); v != nil {
		if iv, ok := v.(*expvar.Int); ok {
			iv.Add(delta)
			return
		}
	}

	// First time seeing this key - create new counter
	var iv expvar.Int
	iv.Add(delta)
	lc.m.Set(key, &iv)
}

// Set sets the counter for a given label set to a specific value.
func (lc *labelledCounter) Set(key string, value int64) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	var iv expvar.Int
	iv.Set(value)
	lc.m.Set(key, &iv)
}

// Get returns the counter value for a given label set.
func (lc *labelledCounter) Get(key string) int64 {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if v := lc.m.Get(key); v != nil {
		if iv, ok := v.(*expvar.Int); ok {
			return iv.Value()
		}
	}
	return 0
}

// Do iterates over all key-value pairs in the counter.
func (lc *labelledCounter) Do(f func(expvar.KeyValue)) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.m.Do(f)
}

// Metrics holds all ARMOR metrics.
type Metrics struct {
	// Request metrics
	RequestsTotal         *expvar.Int
	RequestsInFlight      *expvar.Int
	requestsByLabel       *labelledCounter // per-operation, status-class counters

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
	EncryptionOpsTotal *labelledCounter
	DecryptionOpsTotal *labelledCounter
	KeyWrapOpsTotal    *expvar.Int
	KeyUnwrapOpsTotal  *expvar.Int

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

	// Secondary backend canary metrics (ADR-006)
	SecondaryCanaryChecksTotal    *expvar.Int
	SecondaryCanaryCheckFailures  *expvar.Int
	SecondaryCanaryLastCheckTime  *expvar.String
	SecondaryCanaryLastCheckError *expvar.String
	SecondaryCanaryHealthy        *expvar.Int

	// Multipart histogram metrics (bucketed by operation and status)
	MultipartUploadBuckets       *expvar.Map // Histogram buckets: upload operation, keyed by latency
	MultipartVerificationBuckets *expvar.Map // Histogram buckets: verification operation, keyed by latency
	MultipartOperationTotal      *expvar.Map // Counter by operation and status: operation_status

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
	BackendRequestsTotal   *labelledCounter
	BackendRequestDuration *labelledCounter

	// Request duration histogram (fixed buckets)
	requestDurationBuckets []int64
	requestDurationSum      *expvar.Map // operation -> sum of durations
	requestDurationCount   *expvar.Map // operation -> count of observations
	requestDurationBucketsMap *expvar.Map // operation_bucket_le -> cumulative count

	// Restore verifier metrics (Phase 6)
	RestoreVerifierLastCheckTime   *expvar.String
	RestoreVerifierLastCheckError  *expvar.String
	restoreVerifierChecksTotal     *labelledCounter
	restoreVerifierFailuresTotal   *labelledCounter
	restoreVerifierObjectsVerified *labelledCounter
	restoreVerifierObjectsFailed   *labelledCounter
	restoreVerifierLatencyMillis   *labelledCounter

	// Restore verifier per-bucket gauges (Phase 6a — restorability alerting).
	// Each map is keyed by bucket name so PrometheusFormat can emit one labeled
	// series per bucket. These back the restorability PrometheusRule:
	// armor_last_verified_restore_timestamp, armor_verified_object_ratio, and
	// armor_restore_verification_failures_total.
	RestoreVerifierLastVerifiedTs *expvar.Map // bucket -> last verification time (unix seconds)
	RestoreVerifierObjectRatio    *expvar.Map // bucket -> verified/total ratio (0..1)
	RestoreVerifierFailureCount   *expvar.Map // bucket -> failed object count

	// DR-drill (direct-only) per-bucket gauges — the direct-path analogue of the
	// three above, kept distinct so a drill run never bumps the dual-path
	// restorability series (and vice versa). They back the drill_restore_age and
	// drill-failure signals and expose the drill's own last-success timestamp
	// (armor_drill_last_success_timestamp) for "when did we last prove recovery
	// with the ARMOR server deliberately excluded?".
	RestoreVerifierDrillLastVerifiedTs *expvar.Map // bucket -> last drill attempt time (unix seconds)
	RestoreVerifierDrillLastSuccessTs  *expvar.Map // bucket -> last successful direct-only recovery (unix seconds)
	RestoreVerifierDrillObjectRatio    *expvar.Map // bucket -> recovered/total ratio (0..1)
	RestoreVerifierDrillFailureCount   *expvar.Map // bucket -> cumulative failed direct-only recoveries

	// Replication queue metrics for async secondary backend replication
	ReplicationQueueDepth   *expvar.Int // Current number of items in replication queue
	ReplicationDroppedTotal *expvar.Int // Total number of items dropped due to full queue
	ReplicationErrorsTotal  *expvar.Int // Total number of replication copy failures after all retries
	ReplicationRetriesTotal *expvar.Int // Total number of replication retry attempts
	// ReplicationEnqueuedTotal tracks total enqueued replication operations by operation type.
	// Uses atomic operations for thread-safe increments without mutex contention.
	// Operations are: "put" for standard uploads, "put-streaming" for streaming uploads.
	ReplicationEnqueuedTotal *expvar.Map // Deprecated: kept for Prometheus format compatibility
	// Atomic counters for thread-safe increments
	replicationEnqueuedPut               atomic.Int64 // Counter for "put" operations
	replicationEnqueuedPutStreaming     atomic.Int64 // Counter for "put-streaming" operations
	replicationEnqueuedCompleteMultipart atomic.Int64 // Counter for "completemultipart" operations

	// Manifest compaction metrics
	CompactionErrorsTotal *expvar.Int // Total number of manifest compaction errors

	// S3 error metrics (ADR-008)
	ErrorsTotal *labelledCounter // Total S3 errors by error code and operation

	// Requests by credential metrics (Plan §8.9)
	// Tracks requests by access_key_id, verb, and authorization result (allow, deny-auth, deny-acl)
	// access_key_id is an identifier (not a secret) - cardinality bounded by configured credentials + "unknown"
	requestsByCredentialTotal *labelledCounter

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
	m.requestsByLabel = newLabelledCounter()

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
	m.EncryptionOpsTotal = newLabelledCounter()
	m.DecryptionOpsTotal = newLabelledCounter()
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

	// Secondary backend canary metrics
	m.SecondaryCanaryChecksTotal = new(expvar.Int)
	m.SecondaryCanaryCheckFailures = new(expvar.Int)
	m.SecondaryCanaryLastCheckTime = new(expvar.String)
	m.SecondaryCanaryLastCheckError = new(expvar.String)
	m.SecondaryCanaryHealthy = new(expvar.Int)

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
	m.BackendRequestsTotal = newLabelledCounter()
	m.BackendRequestDuration = newLabelledCounter()

	// Request duration histogram (fixed buckets in milliseconds)
	m.requestDurationBuckets = []int64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}
	m.requestDurationSum = new(expvar.Map).Init()
	m.requestDurationCount = new(expvar.Map).Init()
	m.requestDurationBucketsMap = new(expvar.Map).Init()

	// Restore verifier metrics
	m.RestoreVerifierLastCheckTime = new(expvar.String)
	m.RestoreVerifierLastCheckError = new(expvar.String)
	m.restoreVerifierChecksTotal = newLabelledCounter()
	m.restoreVerifierFailuresTotal = newLabelledCounter()
	m.restoreVerifierObjectsVerified = newLabelledCounter()
	m.restoreVerifierObjectsFailed = newLabelledCounter()
	m.restoreVerifierLatencyMillis = newLabelledCounter()

	// Restore verifier per-bucket gauges (Phase 6a)
	m.RestoreVerifierLastVerifiedTs = new(expvar.Map).Init()
	m.RestoreVerifierObjectRatio = new(expvar.Map).Init()
	m.RestoreVerifierFailureCount = new(expvar.Map).Init()

	// DR-drill (direct-only) per-bucket gauges — distinct from the dual-path
	// gauges above so a drill never perturbs the continuous-verification series.
	m.RestoreVerifierDrillLastVerifiedTs = new(expvar.Map).Init()
	m.RestoreVerifierDrillLastSuccessTs = new(expvar.Map).Init()
	m.RestoreVerifierDrillObjectRatio = new(expvar.Map).Init()
	m.RestoreVerifierDrillFailureCount = new(expvar.Map).Init()

	// Replication queue metrics
	m.ReplicationQueueDepth = new(expvar.Int)
	m.ReplicationDroppedTotal = new(expvar.Int)
	m.ReplicationErrorsTotal = new(expvar.Int)
	m.ReplicationRetriesTotal = new(expvar.Int)
	m.ReplicationEnqueuedTotal = new(expvar.Map).Init()

	// Manifest compaction metrics
	m.CompactionErrorsTotal = new(expvar.Int)

	// S3 error metrics
	m.ErrorsTotal = newLabelledCounter()

	// Requests by credential metrics
	m.requestsByCredentialTotal = newLabelledCounter()

	return m
}

// IncRequestsTotal increments the request counter for an operation and status.
func (m *Metrics) IncRequestsTotal(operation string, status int) {
	m.RequestsTotal.Add(1)
	// Track by operation and status class
	key := fmt.Sprintf("%s_%dxx", operation, status/100)
	m.requestsByLabel.Add(key, 1)
}

// IncRequestsInFlight increments the in-flight request counter.
func (m *Metrics) IncRequestsInFlight() {
	m.RequestsInFlight.Add(1)
}

// DecRequestsInFlight decrements the in-flight request counter.
func (m *Metrics) DecRequestsInFlight() {
	m.RequestsInFlight.Add(-1)
}

// RecordRequestDuration records the duration of a request as a histogram with fixed buckets.
func (m *Metrics) RecordRequestDuration(operation string, duration time.Duration) {
	millis := duration.Milliseconds()

	// Update sum for this operation
	sumKey := operation
	var currentSum expvar.Int
	if existingSum := m.requestDurationSum.Get(sumKey); existingSum != nil {
		currentSum.Set(existingSum.(*expvar.Int).Value() + millis)
	} else {
		currentSum.Set(millis)
	}
	m.requestDurationSum.Set(sumKey, &currentSum)

	// Update count for this operation
	countKey := operation
	var currentCount expvar.Int
	if existingCount := m.requestDurationCount.Get(countKey); existingCount != nil {
		currentCount.Set(existingCount.(*expvar.Int).Value() + 1)
	} else {
		currentCount.Set(1)
	}
	m.requestDurationCount.Set(countKey, &currentCount)

	// Update bucket counters (cumulative histogram)
	for _, bucketLe := range m.requestDurationBuckets {
		if millis <= bucketLe {
			bucketKey := fmt.Sprintf("%s_bucket_le_%d", operation, bucketLe)
			var bucketVal expvar.Int
			if existingBucket := m.requestDurationBucketsMap.Get(bucketKey); existingBucket != nil {
				bucketVal.Set(existingBucket.(*expvar.Int).Value() + 1)
			} else {
				bucketVal.Set(1)
			}
			m.requestDurationBucketsMap.Set(bucketKey, &bucketVal)
		}
	}

	// Also track in +Inf bucket (all observations)
	infBucketKey := fmt.Sprintf("%s_bucket_le_Inf", operation)
	var infBucketVal expvar.Int
	if existingInf := m.requestDurationBucketsMap.Get(infBucketKey); existingInf != nil {
		infBucketVal.Set(existingInf.(*expvar.Int).Value() + 1)
	} else {
		infBucketVal.Set(1)
	}
	m.requestDurationBucketsMap.Set(infBucketKey, &infBucketVal)
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
	m.EncryptionOpsTotal.Add(opType, 1)
}

// IncDecryptionOps increments the decryption operations counter.
func (m *Metrics) IncDecryptionOps(opType string) {
	m.DecryptionOpsTotal.Add(opType, 1)
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

// IncSecondaryCanaryChecks increments the secondary canary check counter.
func (m *Metrics) IncSecondaryCanaryChecks() {
	m.SecondaryCanaryChecksTotal.Add(1)
}

// IncSecondaryCanaryFailures increments the secondary canary failure counter.
func (m *Metrics) IncSecondaryCanaryFailures() {
	m.SecondaryCanaryCheckFailures.Add(1)
}

// SetSecondaryCanaryLastCheck sets the last secondary canary check time.
func (m *Metrics) SetSecondaryCanaryLastCheck(t time.Time) {
	m.SecondaryCanaryLastCheckTime.Set(t.UTC().Format(time.RFC3339))
}

// SetSecondaryCanaryLastError sets the last secondary canary error.
func (m *Metrics) SetSecondaryCanaryLastError(err string) {
	m.SecondaryCanaryLastCheckError.Set(err)
}

// SetSecondaryCanaryHealthy sets the secondary canary health status (1 = healthy, 0 = unhealthy).
func (m *Metrics) SetSecondaryCanaryHealthy(healthy bool) {
	if healthy {
		m.SecondaryCanaryHealthy.Set(1)
	} else {
		m.SecondaryCanaryHealthy.Set(0)
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
	m.BackendRequestsTotal.Add(operation, 1)
}

// RecordBackendRequestDuration records the duration of a backend request.
func (m *Metrics) RecordBackendRequestDuration(operation string, duration time.Duration) {
	key := operation
	millis := duration.Milliseconds()
	bucket := fmt.Sprintf("%s_duration_%d", key, millis)
	m.BackendRequestDuration.Add(bucket, 1)
}

// IncReplicationEnqueued increments the replication enqueued counter for an operation.
// Uses atomic operations for thread-safe increments without mutex contention.
// Multiple goroutines can call this method simultaneously without data races.
//
// Example usage (concurrent):
//
//	// In upload handler goroutine 1:
//	go func() {
//	    metrics.IncReplicationEnqueued("put")
//	}()
//
//	// In upload handler goroutine 2 (simultaneous):
//	go func() {
//	    metrics.IncReplicationEnqueued("put-streaming")
//	}()
//
// Supported operations:
//   - "put" — standard object uploads (PutObject)
//   - "put-streaming" — streaming uploads (PutObject with streaming)
//   - "completemultipart" — multipart upload completions (CompleteMultipartUpload)
func (m *Metrics) IncReplicationEnqueued(operation string) {
	switch operation {
	case "put":
		m.replicationEnqueuedPut.Add(1)
	case "put-streaming":
		m.replicationEnqueuedPutStreaming.Add(1)
	case "completemultipart":
		m.replicationEnqueuedCompleteMultipart.Add(1)
	}
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

	// Labelled counter: encryption operations by type
	sb.WriteString("\n# HELP armor_encryption_ops_total Total number of encryption operations by type\n")
	sb.WriteString("# TYPE armor_encryption_ops_total counter\n")
	m.EncryptionOpsTotal.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_encryption_ops_total{operation=%q} %s\n", kv.Key, kv.Value.String())
	})

	// Labelled counter: decryption operations by type
	sb.WriteString("# HELP armor_decryption_ops_total Total number of decryption operations by type\n")
	sb.WriteString("# TYPE armor_decryption_ops_total counter\n")
	m.DecryptionOpsTotal.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_decryption_ops_total{operation=%q} %s\n", kv.Key, kv.Value.String())
	})

	// Labelled counter: requests by operation and status class
	sb.WriteString("# HELP armor_requests_by_label Total number of requests by operation and status class\n")
	sb.WriteString("# TYPE armor_requests_by_label counter\n")
	m.requestsByLabel.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_requests_by_label{key=%q} %s\n", kv.Key, kv.Value.String())
	})

	// Request duration histogram (fixed buckets)
	sb.WriteString("\n# HELP armor_request_duration_ms Request duration in milliseconds\n")
	sb.WriteString("# TYPE armor_request_duration_ms histogram\n")

	// Export histogram for each operation we've seen
	m.requestDurationCount.Do(func(kv expvar.KeyValue) {
		operation := kv.Key
		countVal := kv.Value.(*expvar.Int).Value()

		// Get sum
		var sumVal int64
		if sum := m.requestDurationSum.Get(operation); sum != nil {
			sumVal = sum.(*expvar.Int).Value()
		}

		// Export _sum and _count
		fmt.Fprintf(&sb, "armor_request_duration_ms_sum{operation=%q} %d\n", operation, sumVal)
		fmt.Fprintf(&sb, "armor_request_duration_ms_count{operation=%q} %d\n", operation, countVal)

		// Export bucket counters (cumulative)
		for _, bucketLe := range m.requestDurationBuckets {
			bucketKey := fmt.Sprintf("%s_bucket_le_%d", operation, bucketLe)
			if bucket := m.requestDurationBucketsMap.Get(bucketKey); bucket != nil {
				fmt.Fprintf(&sb, "armor_request_duration_ms_bucket{operation=%q,le=%q} %s\n", operation, fmt.Sprintf("%d", bucketLe), bucket.(*expvar.Int).String())
			} else {
				// Bucket exists but has no observations yet (shouldn't happen with cumulative, but be defensive)
				fmt.Fprintf(&sb, "armor_request_duration_ms_bucket{operation=%q,le=%q} 0\n", operation, fmt.Sprintf("%d", bucketLe))
			}
		}
		// Export +Inf bucket
		infBucketKey := fmt.Sprintf("%s_bucket_le_Inf", operation)
		if infBucket := m.requestDurationBucketsMap.Get(infBucketKey); infBucket != nil {
			fmt.Fprintf(&sb, "armor_request_duration_ms_bucket{operation=%q,le=\"+Inf\"} %s\n", operation, infBucket.(*expvar.Int).String())
		}
	})

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

	// Secondary backend canary metrics (ADR-006)
	writeMetric("secondary_canary_checks_total", "Total number of secondary backend canary checks", "counter", m.SecondaryCanaryChecksTotal)
	writeMetric("secondary_canary_check_failures_total", "Total number of secondary backend canary check failures", "counter", m.SecondaryCanaryCheckFailures)
	writeMetric("secondary_canary_last_check_time", "Time of last secondary backend canary check", "gauge", m.SecondaryCanaryLastCheckTime)
	writeMetric("secondary_canary_last_check_error", "Error from last failed secondary backend canary check", "gauge", m.SecondaryCanaryLastCheckError)
	writeMetric("secondary_canary_healthy", "Secondary backend canary health status (1=healthy, 0=unhealthy)", "gauge", m.SecondaryCanaryHealthy)

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

	// Backend request metrics
	sb.WriteString("\n# HELP armor_backend_requests_total Total number of backend requests by operation\n")
	sb.WriteString("# TYPE armor_backend_requests_total counter\n")
	m.BackendRequestsTotal.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_backend_requests_total{operation=%q} %s\n", kv.Key, kv.Value.String())
	})

	// Backend request duration metrics
	sb.WriteString("# HELP armor_backend_request_duration_total Backend request duration in milliseconds\n")
	sb.WriteString("# TYPE armor_backend_request_duration_total counter\n")
	m.BackendRequestDuration.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_backend_request_duration_total{key=%q} %s\n", kv.Key, kv.Value.String())
	})

	// Restore verifier metrics
	sb.WriteString("\n# HELP armor_restore_verifier_checks_total Total number of restore verifier checks per bucket\n")
	sb.WriteString("# TYPE armor_restore_verifier_checks_total counter\n")
	m.restoreVerifierChecksTotal.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_restore_verifier_checks_total{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

	sb.WriteString("# HELP armor_restore_verifier_failures_total Total number of restore verifier failures per bucket\n")
	sb.WriteString("# TYPE armor_restore_verifier_failures_total counter\n")
	m.restoreVerifierFailuresTotal.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_restore_verifier_failures_total{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

	sb.WriteString("# HELP armor_restore_verifier_objects_verified Total number of objects verified per bucket\n")
	sb.WriteString("# TYPE armor_restore_verifier_objects_verified counter\n")
	m.restoreVerifierObjectsVerified.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_restore_verifier_objects_verified{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

	sb.WriteString("# HELP armor_restore_verifier_objects_failed Total number of objects that failed verification per bucket\n")
	sb.WriteString("# TYPE armor_restore_verifier_objects_failed counter\n")
	m.restoreVerifierObjectsFailed.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_restore_verifier_objects_failed{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

	sb.WriteString("# HELP armor_restore_verifier_latency_millis Restore verifier latency in milliseconds per bucket\n")
	sb.WriteString("# TYPE armor_restore_verifier_latency_millis gauge\n")
	m.restoreVerifierLatencyMillis.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_restore_verifier_latency_millis{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

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

	// DR-drill (direct-only) per-bucket gauges — the direct-path analogue of the
	// three above. Distinct series so a drill run never perturbs the dual-path
	// restorability alerting; armor_drill_last_success_timestamp is the
	// "drill_last_success" status field the task asked for.
	sb.WriteString("\n# HELP armor_drill_last_verified_timestamp Unix timestamp of the most recent direct-only DR-drill attempt per bucket\n")
	sb.WriteString("# TYPE armor_drill_last_verified_timestamp gauge\n")
	m.RestoreVerifierDrillLastVerifiedTs.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_drill_last_verified_timestamp{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

	sb.WriteString("# HELP armor_drill_last_success_timestamp Unix timestamp of the most recent successful direct-only recovery per bucket (drill_last_success)\n")
	sb.WriteString("# TYPE armor_drill_last_success_timestamp gauge\n")
	m.RestoreVerifierDrillLastSuccessTs.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_drill_last_success_timestamp{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

	sb.WriteString("# HELP armor_drill_verified_object_ratio Ratio of objects recovered direct-only to total sampled per bucket (0..1)\n")
	sb.WriteString("# TYPE armor_drill_verified_object_ratio gauge\n")
	m.RestoreVerifierDrillObjectRatio.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_drill_verified_object_ratio{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

	sb.WriteString("# HELP armor_drill_failures_total Cumulative objects that failed direct-only recovery per bucket\n")
	sb.WriteString("# TYPE armor_drill_failures_total counter\n")
	m.RestoreVerifierDrillFailureCount.Do(func(kv expvar.KeyValue) {
		fmt.Fprintf(&sb, "armor_drill_failures_total{bucket=%q} %s\n", kv.Key, kv.Value.String())
	})

	// Replication queue metrics
	writeMetric("replication_queue_depth", "Current number of items in the replication queue", "gauge", m.ReplicationQueueDepth)
	writeMetric("replication_dropped_total", "Total number of items dropped due to full replication queue", "counter", m.ReplicationDroppedTotal)
	writeMetric("replication_errors_total", "Total number of replication copy failures after all retries", "counter", m.ReplicationErrorsTotal)
	writeMetric("replication_retries_total", "Total number of replication retry attempts", "counter", m.ReplicationRetriesTotal)

	// Replication lag and duration metrics will be exported from the replication package's Metrics struct
	// These are accessed through the replication queue's metrics instance

	// Manifest compaction metrics
	writeMetric("manifest_compaction_errors_total", "Total number of manifest compaction errors", "counter", m.CompactionErrorsTotal)

	// S3 error metrics (ADR-008)
	sb.WriteString("\n# HELP armor_errors_total Total number of S3 errors by error code and operation\n")
	sb.WriteString("# TYPE armor_errors_total counter\n")
	m.ErrorsTotal.Do(func(kv expvar.KeyValue) {
		// Parse the "code:operation" key format
		parts := strings.SplitN(kv.Key, ":", 2)
		if len(parts) == 2 {
			code := parts[0]
			operation := parts[1]
			fmt.Fprintf(&sb, "armor_errors_total{code=%q,operation=%q} %s\n", code, operation, kv.Value.String())
		} else {
			// Fallback for malformed keys (shouldn't happen)
			fmt.Fprintf(&sb, "armor_errors_total{code=%q,operation=\"\"} %s\n", kv.Key, kv.Value.String())
		}
	})

	// Requests by credential metrics (Plan §8.9)
	sb.WriteString("\n# HELP armor_requests_by_credential_total Total number of requests by access key ID, verb, and authorization result\n")
	sb.WriteString("# TYPE armor_requests_by_credential_total counter\n")
	m.requestsByCredentialTotal.Do(func(kv expvar.KeyValue) {
		// Parse the "access_key_id:verb:result" key format
		parts := strings.SplitN(kv.Key, ":", 3)
		if len(parts) == 3 {
			accessKeyID := parts[0]
			verb := parts[1]
			result := parts[2]
			fmt.Fprintf(&sb, "armor_requests_by_credential_total{access_key_id=%q,verb=%q,result=%q} %s\n", accessKeyID, verb, result, kv.Value.String())
		} else {
			// Fallback for malformed keys (shouldn't happen)
			fmt.Fprintf(&sb, "armor_requests_by_credential_total{access_key_id=%q,verb=\"\",result=\"\"} %s\n", kv.Key, kv.Value.String())
		}
	})

	sb.WriteString("\n# HELP armor_replication_enqueued_total Total number of items enqueued for replication by operation\n")
	sb.WriteString("# TYPE armor_replication_enqueued_total counter\n")
	// Read from atomic counters
	fmt.Fprintf(&sb, "armor_replication_enqueued_total{operation=%q} %d\n", "put", m.replicationEnqueuedPut.Load())
	fmt.Fprintf(&sb, "armor_replication_enqueued_total{operation=%q} %d\n", "put-streaming", m.replicationEnqueuedPutStreaming.Load())
	fmt.Fprintf(&sb, "armor_replication_enqueued_total{operation=%q} %d\n", "completemultipart", m.replicationEnqueuedCompleteMultipart.Load())


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
	m.restoreVerifierChecksTotal.Add(bucket, 1)

	if success {
		m.restoreVerifierObjectsVerified.Add(bucket, 1)
	} else {
		m.restoreVerifierFailuresTotal.Add(bucket, 1)
		m.restoreVerifierObjectsFailed.Add(bucket, 1)
	}

	latencyKey := fmt.Sprintf("%s_latency", bucket)
	m.restoreVerifierLatencyMillis.Set(latencyKey, int64(duration.Milliseconds()))
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

// RecordDRDrillRun publishes the per-bucket direct-only DR-drill gauges — the
// drill analogue of RecordRestoreBucketState, plus the drill's own last-success
// timestamp. lastVerified is this attempt's time (success or failure) so the
// drill_restore_age signal advances every run; lastSuccess is the most recent
// *successful* direct-only recovery (zero when none has succeeded yet, so the
// drill_last_success gauge stays at 0 until recovery is actually proven);
// ratio is recovered/total in [0,1]; failures is the cumulative drill failure
// count (a counter so any increase trips the drill-failure signal).
func (m *Metrics) RecordDRDrillRun(bucket string, lastVerified, lastSuccess time.Time, ratio float64, failures int64) {
	if bucket == "" {
		return
	}

	var ts expvar.Int
	ts.Set(lastVerified.Unix())
	m.RestoreVerifierDrillLastVerifiedTs.Set(bucket, &ts)

	var s expvar.Int
	if !lastSuccess.IsZero() {
		s.Set(lastSuccess.Unix())
	}
	m.RestoreVerifierDrillLastSuccessTs.Set(bucket, &s)

	var r expvar.Float
	r.Set(ratio)
	m.RestoreVerifierDrillObjectRatio.Set(bucket, &r)

	var fc expvar.Int
	fc.Set(failures)
	m.RestoreVerifierDrillFailureCount.Set(bucket, &fc)
}

// SetReplicationQueueDepth sets the current replication queue depth.
func (m *Metrics) SetReplicationQueueDepth(depth int64) {
	m.ReplicationQueueDepth.Set(depth)
}

// AddReplicationQueueDepth adds to the replication queue depth (can be negative).
func (m *Metrics) AddReplicationQueueDepth(delta int64) {
	m.ReplicationQueueDepth.Add(delta)
}

// IncReplicationDropped increments the replication dropped counter.
func (m *Metrics) IncReplicationDropped() {
	m.ReplicationDroppedTotal.Add(1)
}

// IncReplicationErrors increments the replication errors counter.
func (m *Metrics) IncReplicationErrors() {
	m.ReplicationErrorsTotal.Add(1)
}

// IncReplicationRetries increments the replication retries counter.
func (m *Metrics) IncReplicationRetries() {
	m.ReplicationRetriesTotal.Add(1)
}

// IncCompactionErrors increments the manifest compaction error counter.
func (m *Metrics) IncCompactionErrors() {
	m.CompactionErrorsTotal.Add(1)
}

// IncErrors increments the S3 error counter for a given error code and operation.
// The label key combines error code and operation as "code:operation".
func (m *Metrics) IncErrors(code, operation string) {
	key := fmt.Sprintf("%s:%s", code, operation)
	m.ErrorsTotal.Add(key, 1)
}

// IncRequestsByCredential increments the requests by credential counter.
// Labels: access_key_id, verb, result (allow, deny-auth, deny-acl)
// The label key combines all three as "access_key_id:verb:result"
func (m *Metrics) IncRequestsByCredential(accessKeyID, verb, result string) {
	key := fmt.Sprintf("%s:%s:%s", accessKeyID, verb, result)
	m.requestsByCredentialTotal.Add(key, 1)
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
