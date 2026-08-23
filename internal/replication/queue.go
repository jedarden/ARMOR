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
// The worker reads from the primary backend and writes to the secondary backend,
// using backend.Copy() when available (B2-to-B2) or falling back to Get+Put.
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

	// started indicates whether Start has been called
	started atomic.Bool

	// depth tracks the current queue depth for metrics
	depth atomic.Int64

	// primary is the primary backend (source for replication)
	primary backend.Backend

	// secondary is the secondary backend (target for replication)
	secondary backend.Backend

	// oldestTaskEnqueued tracks when the oldest task in the queue was enqueued
	// for the replication_lag_seconds metric (Unix nanoseconds)
	oldestTaskEnqueued atomic.Int64

	// mu protects concurrent updates to oldestTaskEnqueued
	mu sync.Mutex

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
	if bufSize <= 0 {
		bufSize = DefaultQueueBufferSize
	}
	if logger == nil {
		logger = log.New(log.Writer(), "[replication] ", log.LstdFlags|log.Lmsgprefix)
	}
	return &ReplicationQueue{
		metrics:   metrics,
		queueCh:   make(chan task, bufSize),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		started:   atomic.Bool{},
		depth:     atomic.Int64{},
		primary:   primary,
		secondary: secondary,
		logger:    logger,
	}
}

// Start launches the background worker goroutine. Call it once after NewReplicationQueue.
// ctx cancellation stops the goroutine (same effect as Stop).
func (q *ReplicationQueue) Start(ctx context.Context) {
	if !q.started.CompareAndSwap(false, true) {
		// Already started
		return
	}
	go q.run(ctx)
}

// Stop signals the worker goroutine to stop and waits for it to exit.
// It waits up to shutdownTimeout for the queue to drain before forcing
// a stop. Safe to call multiple times (idempotent).
func (q *ReplicationQueue) Stop() {
	q.once.Do(func() {
		close(q.stop)
	})
	<-q.done
}

// Enqueue adds a replication task for the given bucket and key.
// Non-blocking: when the channel is full, the item is silently dropped
// and the dropped metric is incremented.
func (q *ReplicationQueue) Enqueue(bucket, key string) {
	t := task{
		bucket:     bucket,
		key:        key,
		enqueuedAt: time.Now().UnixNano(),
	}
	select {
	case q.queueCh <- t:
		q.depth.Add(1)
		// Update oldest task timestamp if this is now the oldest
		q.updateOldestTaskTimestamp(t.enqueuedAt)
	default:
		// Queue full — drop and increment metric
		q.metrics.DroppedTotal.Add(1)
	}
}

// run is the background worker goroutine. It drains replication tasks
// from the queue and processes them. For now, this is a stub that logs
// the keys — the actual replication logic will be added later.
func (q *ReplicationQueue) run(ctx context.Context) {
	defer close(q.done)

	for {
		select {
		case t := <-q.queueCh:
			q.depth.Add(-1)
			q.processTask(t)
		case <-q.stop:
			// Drain remaining tasks before exit
			q.drain()
			return
		case <-ctx.Done():
			// Context cancelled — drain and exit
			q.drain()
			return
		}
	}
}

// drain processes all remaining tasks in the queue before shutdown.
func (q *ReplicationQueue) drain() {
	for {
		select {
		case t := <-q.queueCh:
			q.depth.Add(-1)
			q.processTask(t)
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
// and updates metrics. The worker never blocks on errors — it logs, increments
// the error metric, and continues to the next task.
func (q *ReplicationQueue) processTask(t task) {
	defer q.updateOldestAfterTaskRemoval()

	// Update lag metric before processing
	q.updateLagMetric()

	// Attempt replication with retries
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Check if error is permanent - if so, skip immediately
			if !isTransientError(lastErr) {
				q.logger.Printf("replication failed for %s/%s with permanent error (skipping retries): %v", t.bucket, t.key, lastErr)
				q.metrics.ErrorsTotal.Add(1)
				return
			}

			// Exponential backoff: 100ms, 200ms, 400ms
			backoffMs := int64(100 * (1 << (attempt - 1)))
			time.Sleep(time.Duration(backoffMs) * time.Millisecond)
			q.metrics.RetriesTotal.Add(1)
		}

		// Time the copy operation for the duration histogram
		start := time.Now()

		// Try backend.Copy() first (most efficient for B2-to-B2)
		err := q.secondary.Copy(context.Background(), t.bucket, t.key, t.bucket, t.key, nil, false)
		if err == nil {
			// Success! Record duration and return
			duration := time.Since(start).Seconds()
			q.metrics.CopyDurationSeconds.Observe(duration)
			q.logger.Printf("replicated %s/%s (attempt %d, %.2fs)", t.bucket, t.key, attempt+1, duration)
			return
		}

		// Copy failed, fall back to Get+Put pattern
		// This handles cases where Copy() is not available or fails
		lastErr = q.fallbackCopy(t.bucket, t.key)
		if lastErr == nil {
			duration := time.Since(start).Seconds()
			q.metrics.CopyDurationSeconds.Observe(duration)
			q.logger.Printf("replicated %s/%s via Get+Put (attempt %d, %.2fs)", t.bucket, t.key, attempt+1, duration)
			return
		}

		// Log retry attempt
		if attempt < maxRetries {
			q.logger.Printf("replication attempt %d failed for %s/%s: %v (will retry)", attempt+1, t.bucket, t.key, lastErr)
		}
	}

	// All retries exhausted
	q.metrics.ErrorsTotal.Add(1)
	q.logger.Printf("replication failed after %d attempts for %s/%s: %v", maxRetries+1, t.bucket, t.key, lastErr)
}

// fallbackCopy implements Get+Put pattern for backends that don't support Copy.
func (q *ReplicationQueue) fallbackCopy(bucket, key string) error {
	ctx := context.Background()

	// Get object from primary backend
	body, info, err := q.primary.Get(ctx, bucket, key)
	if err != nil {
		return err
	}
	defer body.Close()

	// Put to secondary backend with same metadata
	err = q.secondary.Put(ctx, bucket, key, body, info.Size, info.Metadata)
	if err != nil {
		return err
	}

	return nil
}

// updateOldestTaskTimestamp updates the oldest task enqueue timestamp if the given timestamp is older.
func (q *ReplicationQueue) updateOldestTaskTimestamp(timestamp int64) {
	q.mu.Lock()
	defer q.mu.Unlock()

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

// updateOldestAfterTaskRemoval updates the oldest task timestamp after a task is removed.
// This is a placeholder for a more sophisticated implementation that would track
// the second-oldest task. For now, we conservatively set to 0 when we can't determine the actual oldest.
func (q *ReplicationQueue) updateOldestAfterTaskRemoval() {
	// In a production implementation, we would maintain a priority queue of enqueue times.
	// For this implementation, we'll conservatively set to 0 when we can't track accurately.
	// The lag will spike briefly when processing the oldest task, then settle to the next task's age.
	q.mu.Lock()
	defer q.mu.Unlock()

	// If we processed the oldest task, we need to find the next oldest
	// This is complex without a priority queue, so we use a heuristic:
	// If queue depth is 0, reset to 0. Otherwise, we'll update on next enqueue.
	if q.depth.Load() == 0 {
		q.oldestTaskEnqueued.Store(0)
	}
	// If queue still has items, the next enqueue will update if it's older,
	// or we'll continue with a conservative lag estimate.
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
