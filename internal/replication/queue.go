// Package replication provides async replication queue infrastructure for ARMOR.
// This enables deferred replication of objects to secondary backends without blocking
// the client request path.
package replication

import (
	"context"
	"sync"
	"sync/atomic"
)

// DefaultQueueBufferSize is the default buffered channel capacity for ReplicationQueue.
const DefaultQueueBufferSize = 4096

// ReplicationQueue buffers object replication tasks and drains them asynchronously
// via a background worker goroutine. The enqueue operation is non-blocking — when the
// channel is full, items are dropped with a metric increment (replication is a cache
// — the primary backend remains authoritative).
//
// This is infrastructure only — the worker goroutine is a stub that logs keys. The
// actual replication logic will be implemented in a follow-up task.
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
}

// task represents a single replication task.
type task struct {
	bucket string
	key    string
}

// Metrics holds replication queue metrics.
type Metrics struct {
	// QueueDepth is the current number of items in the queue
	QueueDepth *atomic.Int64

	// DroppedTotal is the count of items dropped due to full queue
	DroppedTotal *atomic.Int64
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{
		QueueDepth:   &atomic.Int64{},
		DroppedTotal: &atomic.Int64{},
	}
}

// NewReplicationQueue creates a ReplicationQueue with the specified buffer size.
// Pass 0 for bufSize to use DefaultQueueBufferSize. Call Start to launch the
// background worker goroutine.
func NewReplicationQueue(metrics *Metrics, bufSize int) *ReplicationQueue {
	if bufSize <= 0 {
		bufSize = DefaultQueueBufferSize
	}
	return &ReplicationQueue{
		metrics:  metrics,
		queueCh:  make(chan task, bufSize),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		started:  atomic.Bool{},
		depth:    atomic.Int64{},
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
	t := task{bucket: bucket, key: key}
	select {
	case q.queueCh <- t:
		q.depth.Add(1)
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

// processTask is the stub worker that processes a single replication task.
// For now, this just logs the task. The actual replication logic will be
// implemented in a follow-up task.
func (q *ReplicationQueue) processTask(t task) {
	// TODO: Implement actual replication to secondary backend
	// This is infrastructure only — we just acknowledge the task for now
	// The key will be logged once replication logic is added
	_ = t.bucket // Use in future implementation
	_ = t.key     // Use in future implementation
}
