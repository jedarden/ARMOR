// Package replication provides async replication queue infrastructure for ARMOR.
// This enables deferred replication of objects to secondary backends without blocking
// the client request path.
package replication

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

// Enqueuer is the interface for enqueuing replication tasks.
// This allows handlers to use either the real queue or a mock for testing.
type Enqueuer interface {
	// Enqueue adds a replication task for the given bucket and key.
	// Implementations must be non-blocking.
	Enqueue(bucket, key string)
}

// DefaultQueueBufferSize is the default buffered channel capacity for ReplicationQueue.
const DefaultQueueBufferSize = 4096

// ReplicationQueue buffers object replication tasks and drains them asynchronously
// via a background worker goroutine. The enqueue operation is non-blocking — when the
// channel is full, items are dropped with a metric increment (replication is a cache
// — the primary backend remains authoritative).
//
// The worker reads from the primary backend and writes to the secondary backend.
// It deliberately uses Get+Put rather than Backend.Copy: Copy is an operation
// within one Backend instance and cannot copy from the primary into a separate
// secondary implementation.
type ReplicationQueue struct {
	// metrics holds the Prometheus metrics for this queue
	metrics *Metrics

	// queueCh is the buffered channel holding replication tasks
	queueCh chan task

	// stop signals the worker goroutine to stop
	stop chan struct{}

	// done is closed when the worker goroutine exits
	done chan struct{}

	// once ensures Stop is idempotent
	once sync.Once

	// enqueueMu closes the small stop/enqueue race without closing queueCh.
	// Stop waits for an in-progress enqueue before signaling the worker, so a
	// successfully sent task is always visible to the drain path.
	enqueueMu sync.Mutex

	// stateMu serializes Start and Stop so Stop-before-Start cannot race into a
	// worker that was created after the stop decision.
	stateMu sync.Mutex

	// started indicates whether Start has been called
	started atomic.Bool

	// stopped prevents handler goroutines that finish during server shutdown
	// from adding work after the worker has exited. The channel stays open so
	// Enqueue remains race-safe and never panics.
	stopped atomic.Bool

	// depth tracks the current queue depth for metrics
	depth atomic.Int64

	// primary is the primary backend (source for replication)
	primary backend.Backend

	// secondary is the secondary backend (target for replication)
	secondary backend.Backend

	// targetBucket is the secondary bucket. Filesystem targets are bucket
	// agnostic and leave this empty, which means the source bucket is reused.
	targetBucket string

	// keyPrefix identifies the shared-bucket namespace so internal ARMOR state
	// is never copied if an internal key is accidentally enqueued.
	keyPrefix string

	// oldestTaskEnqueued tracks when the oldest task in the queue was enqueued
	// for the replication_lag_seconds metric (Unix nanoseconds)
	oldestTaskEnqueued atomic.Int64

	// pendingMu protects the pending enqueue timestamps. Tasks remain pending
	// while a worker is processing them, so lag measures work not yet durable
	// on the secondary rather than only items waiting in the channel.
	pendingMu sync.Mutex
	pending   map[int64]int

	// logger is used for replication status logging
	logger *log.Logger
}

// task represents a single replication task.
type task struct {
	bucket     string
	key        string
	enqueuedAt int64 // Unix nanoseconds timestamp when enqueued
}

// Metrics holds replication queue metrics.
type Metrics struct {
	// QueueDepth is the current number of items in the queue
	QueueDepth *atomic.Int64

	// DroppedTotal is the count of items dropped due to full queue
	DroppedTotal *atomic.Int64

	// ErrorsTotal is the count of replication errors
	ErrorsTotal *atomic.Int64

	// CopyDurationSeconds tracks histogram of copy operation durations
	CopyDurationSeconds *copyDurationHistogram

	// LagSeconds is the age of the oldest unreplicated object in seconds
	LagSeconds *atomic.Int64

	// EnqueuedTotal is the count of items successfully enqueued
	EnqueuedTotal *atomic.Int64

	// RetriesTotal is the count of retry attempts
	RetriesTotal *atomic.Int64
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{
		QueueDepth:          &atomic.Int64{},
		DroppedTotal:        &atomic.Int64{},
		ErrorsTotal:         &atomic.Int64{},
		CopyDurationSeconds: newCopyDurationHistogram(),
		LagSeconds:          &atomic.Int64{},
		EnqueuedTotal:       &atomic.Int64{},
		RetriesTotal:        &atomic.Int64{},
	}
}

// NewReplicationQueue creates a ReplicationQueue with the specified buffer size.
// Pass 0 for bufSize to use DefaultQueueBufferSize. Call Start to launch the
// background worker goroutine.
//
// Parameters:
//   - metrics: Metrics instance for tracking replication operations
//   - primary: Primary backend (source for replication)
//   - secondary: Secondary backend (target for replication)
//   - bufSize: Buffer size for the replication queue (0 uses DefaultQueueBufferSize)
//   - logger: Logger for replication status (nil uses default stdout logger)
func NewReplicationQueue(metrics *Metrics, primary, secondary backend.Backend, bufSize int, logger *log.Logger) *ReplicationQueue {
	return NewReplicationQueueWithTargetBucketAndPrefix(metrics, primary, secondary, "", "", bufSize, logger)
}

// NewReplicationQueueWithTargetBucket creates a queue that writes to
// targetBucket on the secondary backend. An empty target bucket reuses the
// source bucket, which is the correct behavior for the filesystem backend.
func NewReplicationQueueWithTargetBucket(metrics *Metrics, primary, secondary backend.Backend, targetBucket string, bufSize int, logger *log.Logger) *ReplicationQueue {
	return NewReplicationQueueWithTargetBucketAndPrefix(metrics, primary, secondary, targetBucket, "", bufSize, logger)
}

// NewReplicationQueueWithTargetBucketAndPrefix is the fully-configured queue
// constructor used by the server. keyPrefix is the already-normalized
// ARMOR_PREFIX value.
func NewReplicationQueueWithTargetBucketAndPrefix(metrics *Metrics, primary, secondary backend.Backend, targetBucket, keyPrefix string, bufSize int, logger *log.Logger) *ReplicationQueue {
	if bufSize <= 0 {
		bufSize = DefaultQueueBufferSize
	}
	if logger == nil {
		logger = log.New(log.Writer(), "[replication] ", log.LstdFlags|log.Lmsgprefix)
	}
	if metrics == nil {
		metrics = NewMetrics()
	}
	q := &ReplicationQueue{
		metrics:      metrics,
		queueCh:      make(chan task, bufSize),
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
		started:      atomic.Bool{},
		stopped:      atomic.Bool{},
		depth:        atomic.Int64{},
		primary:      primary,
		secondary:    secondary,
		targetBucket: targetBucket,
		keyPrefix:    keyPrefix,
		logger:       logger,
		pending:      make(map[int64]int),
	}
	// Expose the queue's authoritative depth counter through the metrics
	// object as well; callers should not need to wire this relationship by hand.
	metrics.QueueDepth = &q.depth
	return q
}

// Start launches the background worker goroutine. Call it once after NewReplicationQueue.
// ctx cancellation stops the goroutine (same effect as Stop).
func (q *ReplicationQueue) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	q.stateMu.Lock()
	defer q.stateMu.Unlock()
	if q.stopped.Load() {
		return
	}
	if !q.started.CompareAndSwap(false, true) {
		// Already started
		return
	}
	go q.run(ctx)
}

// Stop signals the worker goroutine to stop and waits for it to exit. Pending
// tasks are drained using the worker context; retry backoff is interrupted by
// shutdown. Safe to call multiple times (idempotent).
func (q *ReplicationQueue) Stop() {
	q.stateMu.Lock()
	q.stopped.Store(true)
	started := q.started.Load()
	q.stateMu.Unlock()
	if !started {
		return
	}
	q.enqueueMu.Lock()
	q.once.Do(func() {
		close(q.stop)
	})
	q.enqueueMu.Unlock()
	<-q.done
}

// Enqueue adds a replication task for the given bucket and key.
// Non-blocking: when the channel is full, the item is silently dropped
// and the dropped metric is incremented.
func (q *ReplicationQueue) Enqueue(bucket, key string) {
	q.enqueueMu.Lock()
	defer q.enqueueMu.Unlock()

	if q.stopped.Load() || q.isInternalKey(key) {
		if q.stopped.Load() {
			q.metrics.DroppedTotal.Add(1)
		}
		return
	}
	t := task{
		bucket:     bucket,
		key:        key,
		enqueuedAt: time.Now().UnixNano(),
	}
	// Check stop before the send. The channel is intentionally never closed,
	// because handler goroutines may still be finishing while the server shuts
	// down.
	select {
	case <-q.stop:
		q.metrics.DroppedTotal.Add(1)
		return
	default:
	}
	// Track the task before publishing it to the channel. Otherwise a very fast
	// worker can finish the task before the enqueue path records its timestamp.
	q.updateOldestTaskTimestamp(t.enqueuedAt)
	// Reserve depth before publishing the task. A worker may receive from the
	// channel immediately; incrementing only after the send would let it
	// decrement first and briefly expose a negative queue depth.
	q.depth.Add(1)
	select {
	case q.queueCh <- t:
		q.metrics.EnqueuedTotal.Add(1)
	case <-q.stop:
		q.depth.Add(-1)
		q.updateOldestAfterTaskRemoval(t.enqueuedAt)
		q.metrics.DroppedTotal.Add(1)
	default:
		// Queue full — drop and increment metric
		q.depth.Add(-1)
		q.updateOldestAfterTaskRemoval(t.enqueuedAt)
		q.metrics.DroppedTotal.Add(1)
	}
}

// run is the background worker goroutine. It drains replication tasks from the
// queue and processes them until Stop or the caller's context is canceled.
func (q *ReplicationQueue) run(ctx context.Context) {
	defer func() {
		q.stopped.Store(true)
		close(q.done)
	}()

	for {
		select {
		case t := <-q.queueCh:
			q.depth.Add(-1)
			q.processTask(ctx, t)
		case <-q.stop:
			// Drain remaining tasks before exit
			q.drain(ctx)
			return
		case <-ctx.Done():
			// Context cancelled — drain and exit
			q.drain(ctx)
			return
		}
	}
}

// drain processes all remaining tasks in the queue before shutdown.
func (q *ReplicationQueue) drain(ctx context.Context) {
	for {
		select {
		case t := <-q.queueCh:
			q.depth.Add(-1)
			q.processTask(ctx, t)
		default:
			// Queue empty
			return
		}
	}
}

// isTransientError determines if an error is transient (retryable) or permanent.
// Transient errors include network issues, timeouts, rate limits, and temporary service unavailability.
// Permanent errors include not found, permission denied, and invalid bucket names.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	// Network and timeout errors (transient)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	// Check for network-related error strings
	transientPatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"rate limit",
		"too many requests",
		"service unavailable",
		"gateway timeout",
		"bad gateway",
		"network unreachable",
		"connection timed out",
		"read tcp",
		"write tcp",
	}

	for _, pattern := range transientPatterns {
		if strings.Contains(strings.ToLower(errMsg), strings.ToLower(pattern)) {
			return true
		}
	}

	// Check for specific error types
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}

	// Permanent errors - these should not be retried
	permanentPatterns := []string{
		"not found",
		"no such",
		"does not exist",
		"access denied",
		"forbidden",
		"unauthorized",
		"invalid bucket",
		"bucket not found",
		"authentication failed",
		"permission denied",
	}

	for _, pattern := range permanentPatterns {
		if strings.Contains(strings.ToLower(errMsg), strings.ToLower(pattern)) {
			return false
		}
	}

	// Default: treat unknown errors as transient (better to retry than to skip)
	return true
}

// processTask performs the actual replication from primary to secondary backend.
// It handles errors gracefully, implements retry logic with exponential backoff,
// and updates metrics. Transient failures stay with the task until recovery or
// worker shutdown; permanent failures are recorded and skipped.
func (q *ReplicationQueue) processTask(ctx context.Context, t task) {
	defer q.updateOldestAfterTaskRemoval(t.enqueuedAt)

	// Update lag metric before processing
	q.updateLagMetric()

	// Retry transient secondary failures for the lifetime of the worker. A
	// provider outage must not turn into permanent data loss merely because it
	// lasted longer than a small fixed retry budget. Permanent errors are still
	// dropped immediately, and shutdown/context cancellation stops retries.
	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return
		}

		// Time the copy operation for the duration histogram
		start := time.Now()

		err := q.fallbackCopy(ctx, t.bucket, t.key)
		if err == nil {
			duration := time.Since(start).Seconds()
			q.metrics.CopyDurationSeconds.Observe(duration)
			q.logger.Printf("replicated %s/%s via Get+Put (attempt %d, %.2fs)", t.bucket, t.key, attempt+1, duration)
			return
		}

		if !isTransientError(err) {
			q.logger.Printf("replication failed for %s/%s with permanent error (skipping retries): %v", t.bucket, t.key, err)
			q.metrics.ErrorsTotal.Add(1)
			return
		}

		q.metrics.RetriesTotal.Add(1)
		q.logger.Printf("replication attempt %d failed for %s/%s: %v (will retry)", attempt+1, t.bucket, t.key, err)
		backoff := replicationBackoff(attempt)
		timer := time.NewTimer(backoff)
		select {
		case <-timer.C:
		case <-ctx.Done():
			stopTimer(timer)
			return
		case <-q.stop:
			stopTimer(timer)
			return
		}
	}
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

// fallbackCopy implements Get+Put pattern for backends that don't support Copy.
func (q *ReplicationQueue) fallbackCopy(ctx context.Context, bucket, key string) error {
	// Get object from primary backend
	body, info, err := q.primary.Get(ctx, bucket, key)
	if err != nil {
		return err
	}
	if body == nil || info == nil {
		if body != nil {
			body.Close()
		}
		return errors.New("primary returned incomplete object")
	}
	defer body.Close()

	targetBucket := q.targetBucket
	if targetBucket == "" {
		targetBucket = bucket
	}

	// Put the physical ciphertext size, not the plaintext size reported by an
	// ARMOR-aware primary backend. The latter would make B2 reject the upload or
	// truncate the filesystem copy.
	storedSize := info.StoredSize
	if storedSize <= 0 {
		storedSize = info.Size
	}
	err = q.secondary.Put(ctx, targetBucket, key, body, storedSize, cloneMetadata(info.Metadata))
	if err != nil {
		return err
	}

	return nil
}

func (q *ReplicationQueue) isInternalKey(key string) bool {
	if strings.HasPrefix(key, ".armor/") {
		return true
	}
	return q.keyPrefix != "" && strings.HasPrefix(key, q.keyPrefix+".armor/")
}

func replicationBackoff(attempt int) time.Duration {
	const (
		initial = 100 * time.Millisecond
		maximum = 30 * time.Second
	)
	if attempt >= 9 {
		return maximum
	}
	delay := initial << attempt
	if delay > maximum {
		return maximum
	}
	return delay
}

func cloneMetadata(meta map[string]string) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	clone := make(map[string]string, len(meta))
	for key, value := range meta {
		clone[key] = value
	}
	return clone
}

// updateOldestTaskTimestamp updates the oldest task enqueue timestamp if the given timestamp is older.
func (q *ReplicationQueue) updateOldestTaskTimestamp(timestamp int64) {
	q.pendingMu.Lock()
	defer q.pendingMu.Unlock()
	q.pending[timestamp]++

	currentOldest := q.oldestTaskEnqueued.Load()
	if currentOldest == 0 || timestamp < currentOldest {
		q.oldestTaskEnqueued.Store(timestamp)
	}
}

// updateLagMetric updates the replication_lag_seconds metric based on the oldest task.
func (q *ReplicationQueue) updateLagMetric() {
	// Calculate age of oldest task in the queue
	oldest := q.oldestTaskEnqueued.Load()
	if oldest == 0 {
		q.metrics.LagSeconds.Store(0)
		return
	}

	// Calculate lag in seconds
	now := time.Now().UnixNano()
	lagNanos := now - oldest
	lagSeconds := lagNanos / int64(time.Second)
	q.metrics.LagSeconds.Store(lagSeconds)
}

// updateOldestAfterTaskRemoval updates the oldest task timestamp after a task
// finishes. A timestamp count handles the unlikely case of two tasks enqueued
// during the same nanosecond without letting one remove the other.
func (q *ReplicationQueue) updateOldestAfterTaskRemoval(timestamp int64) {
	q.pendingMu.Lock()
	defer q.pendingMu.Unlock()

	if count := q.pending[timestamp]; count <= 1 {
		delete(q.pending, timestamp)
	} else {
		q.pending[timestamp] = count - 1
	}

	oldest := int64(0)
	for pendingTimestamp := range q.pending {
		if oldest == 0 || pendingTimestamp < oldest {
			oldest = pendingTimestamp
		}
	}
	q.oldestTaskEnqueued.Store(oldest)
	if oldest == 0 {
		q.metrics.LagSeconds.Store(0)
	} else {
		q.updateLagMetric()
	}
}

// copyDurationHistogram tracks copy operation durations.
// Thread-safe for concurrent observations from the worker goroutine.
type copyDurationHistogram struct {
	// Simple histogram with buckets: 0.1s, 0.5s, 1s, 5s, 10s, 30s, 60s, 300s
	counts [8]atomic.Int64
	sum    atomic.Int64 // Total duration in milliseconds
}

// newCopyDurationHistogram creates a new copy duration histogram.
func newCopyDurationHistogram() *copyDurationHistogram {
	h := &copyDurationHistogram{}
	for i := range h.counts {
		h.counts[i] = atomic.Int64{}
	}
	h.sum = atomic.Int64{}
	return h
}

// Observe records a duration in seconds.
func (h *copyDurationHistogram) Observe(durationSeconds float64) {
	// Convert to milliseconds for sum tracking
	durationMs := int64(durationSeconds * 1000)
	h.sum.Add(durationMs)

	// Find appropriate bucket
	bucketIdx := h.bucketIndex(durationSeconds)
	h.counts[bucketIdx].Add(1)
}

// bucketIndex maps a duration in seconds to a bucket index.
func (h *copyDurationHistogram) bucketIndex(durationSeconds float64) int {
	// Buckets: 0.1s, 0.5s, 1s, 5s, 10s, 30s, 60s, 300s
	buckets := []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0, 300.0}

	for i, bucket := range buckets {
		if durationSeconds <= bucket {
			return i
		}
	}
	// Exceeds max bucket, put in last bucket
	return len(buckets) - 1
}

// GetBucketCounts returns the histogram bucket counts.
func (h *copyDurationHistogram) GetBucketCounts() []int64 {
	counts := make([]int64, len(h.counts))
	for i := range h.counts {
		counts[i] = h.counts[i].Load()
	}
	return counts
}

// GetSum returns the sum of all observed durations in milliseconds.
func (h *copyDurationHistogram) GetSum() int64 {
	return h.sum.Load()
}

// GetCount returns the total number of observations.
func (h *copyDurationHistogram) GetCount() int64 {
	var total int64
	for i := range h.counts {
		total += h.counts[i].Load()
	}
	return total
}

// PrometheusFormat returns the histogram in Prometheus text format.
func (h *copyDurationHistogram) PrometheusFormat(metricName string) string {
	buckets := []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0, 300.0}
	var sb strings.Builder

	fmt.Fprintf(&sb, "# HELP %s Replication copy operation duration in seconds\n", metricName)
	fmt.Fprintf(&sb, "# TYPE %s histogram\n", metricName)

	// Write bucket counts
	cumCount := int64(0)
	for i, bucket := range buckets {
		count := h.counts[i].Load()
		cumCount += count
		fmt.Fprintf(&sb, "%s_bucket{le=\"%.1f\"} %d\n", metricName, bucket, cumCount)
	}
	// Add +Inf bucket with total count
	totalCount := h.GetCount()
	fmt.Fprintf(&sb, "%s_bucket{le=\"+Inf\"} %d\n", metricName, totalCount)

	// Write sum and count
	sumMs := h.GetSum()
	fmt.Fprintf(&sb, "%s_sum %.6f\n", metricName, float64(sumMs)/1000.0)
	fmt.Fprintf(&sb, "%s_count %d\n", metricName, totalCount)

	return sb.String()
}

// PrometheusFormat returns the replication metrics in Prometheus text format.
func (m *Metrics) PrometheusFormat() string {
	var sb strings.Builder

	// Replication lag gauge
	fmt.Fprintf(&sb, "# HELP armor_replication_lag_seconds Age of the oldest unreplicated object in the queue\n")
	fmt.Fprintf(&sb, "# TYPE armor_replication_lag_seconds gauge\n")
	fmt.Fprintf(&sb, "armor_replication_lag_seconds %d\n", m.LagSeconds.Load())

	// Replication copy duration histogram
	sb.WriteString(m.CopyDurationSeconds.PrometheusFormat("armor_replication_copy_duration_seconds"))

	// Other replication metrics
	fmt.Fprintf(&sb, "# HELP armor_replication_queue_depth Current number of items in the replication queue\n")
	fmt.Fprintf(&sb, "# TYPE armor_replication_queue_depth gauge\n")
	fmt.Fprintf(&sb, "armor_replication_queue_depth %d\n", m.QueueDepth.Load())

	fmt.Fprintf(&sb, "# HELP armor_replication_dropped_total Total number of items dropped due to full replication queue\n")
	fmt.Fprintf(&sb, "# TYPE armor_replication_dropped_total counter\n")
	fmt.Fprintf(&sb, "armor_replication_dropped_total %d\n", m.DroppedTotal.Load())

	fmt.Fprintf(&sb, "# HELP armor_replication_errors_total Total number of replication copy failures after all retries\n")
	fmt.Fprintf(&sb, "# TYPE armor_replication_errors_total counter\n")
	fmt.Fprintf(&sb, "armor_replication_errors_total %d\n", m.ErrorsTotal.Load())

	fmt.Fprintf(&sb, "# HELP armor_replication_retries_total Total number of replication retry attempts\n")
	fmt.Fprintf(&sb, "# TYPE armor_replication_retries_total counter\n")
	fmt.Fprintf(&sb, "armor_replication_retries_total %d\n", m.RetriesTotal.Load())

	return sb.String()
}

// GetMetrics returns the metrics for the replication queue.
// This allows the canary to access lag metrics without importing the replication package.
func (q *ReplicationQueue) GetMetrics() *Metrics {
	return q.metrics
}
