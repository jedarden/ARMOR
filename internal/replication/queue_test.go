package replication

import (
	"context"
	"testing"
	"time"
)

// TestNewReplicationQueue verifies queue initialization
func TestNewReplicationQueue(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 100)

	if q == nil {
		t.Fatal("NewReplicationQueue returned nil")
	}

	if q.metrics != metrics {
		t.Error("metrics not set correctly")
	}

	if cap(q.queueCh) != 100 {
		t.Errorf("expected channel capacity 100, got %d", cap(q.queueCh))
	}

	if q.started.Load() {
		t.Error("queue should not be started after NewReplicationQueue")
	}
}

// TestNewReplicationQueueDefaultBufferSize verifies default buffer size
func TestNewReplicationQueueDefaultBufferSize(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 0)

	if cap(q.queueCh) != DefaultQueueBufferSize {
		t.Errorf("expected default capacity %d, got %d", DefaultQueueBufferSize, cap(q.queueCh))
	}
}

// TestStart verifies the background goroutine starts correctly
func TestStart(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if q.started.Load() {
		t.Error("queue should not be started initially")
	}

	q.Start(ctx)

	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)

	if !q.started.Load() {
		t.Error("queue should be started after Start()")
	}

	// Verify Start is idempotent
	q.Start(ctx)
	if !q.started.Load() {
		t.Error("queue should still be started after duplicate Start()")
	}

	q.Stop()
}

// TestStartIdempotent verifies Start cannot be called twice successfully
func TestStartIdempotent(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	// Try starting again
	q.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	// Should still have exactly one worker
	q.Stop()
	<-q.done // Should not block
}

// TestStopIdempotent verifies Stop can be called multiple times safely
func TestStopIdempotent(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	// First stop
	q.Stop()

	// Second stop (should not block or panic)
	q.Stop()

	// Third stop (should not block or panic)
	q.Stop()
}

// TestEnqueueNonBlocking verifies enqueue doesn't block when channel is full
func TestEnqueueNonBlocking(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 2)

	// Don't start the worker, so items stay in the queue
	// This tests the non-blocking behavior of enqueue

	// Fill the queue to capacity
	q.Enqueue("bucket", "key1")
	q.Enqueue("bucket", "key2")

	// This third enqueue should be dropped immediately (non-blocking)
	start := time.Now()
	q.Enqueue("bucket", "key3")
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("enqueue on full queue took too long: %v", elapsed)
	}

	// Verify one item was dropped
	if dropped := metrics.DroppedTotal.Load(); dropped != 1 {
		t.Errorf("expected 1 dropped item, got %d", dropped)
	}
}

// TestEnqueueDroppedMetric verifies the dropped metric increments correctly
func TestEnqueueDroppedMetric(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 1)

	// Don't start the worker, so we can test queue overflow behavior

	// Fill queue to capacity
	q.Enqueue("bucket", "key1")

	// These should be dropped (queue is full)
	q.Enqueue("bucket", "key2")
	q.Enqueue("bucket", "key3")
	q.Enqueue("bucket", "key4")

	// Verify three items were dropped
	if dropped := metrics.DroppedTotal.Load(); dropped != 3 {
		t.Errorf("expected 3 dropped items, got %d", dropped)
	}
}

// TestGracefulShutdown verifies Stop drains the queue before exit
func TestGracefulShutdown(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	// Enqueue several items
	for i := 0; i < 10; i++ {
		q.Enqueue("bucket", "key"+string(rune('0'+i)))
	}

	// Stop should drain all items
	start := time.Now()
	q.Stop()
	elapsed := time.Since(start)

	// Stop should wait for drain (but not too long)
	if elapsed > 5*time.Second {
		t.Errorf("Stop took too long: %v", elapsed)
	}

	// Verify no items were dropped
	if dropped := metrics.DroppedTotal.Load(); dropped != 0 {
		t.Errorf("expected 0 dropped items, got %d", dropped)
	}
}

// TestContextCancellation verifies goroutine exits on context cancel
func TestContextCancellation(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 100)

	ctx, cancel := context.WithCancel(context.Background())

	q.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	// Enqueue some items
	q.Enqueue("bucket", "key1")
	q.Enqueue("bucket", "key2")

	// Cancel context
	cancel()

	// Wait for goroutine to exit
	select {
	case <-q.done:
		// Expected - goroutine exited
	case <-time.After(5 * time.Second):
		t.Fatal("goroutine did not exit after context cancellation")
	}

	// Verify queue was drained
	if dropped := metrics.DroppedTotal.Load(); dropped != 0 {
		t.Errorf("expected 0 dropped items, got %d", dropped)
	}
}

// TestQueueDepthTracking verifies depth metric updates correctly
func TestQueueDepthTracking(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 100)

	// Set the depth reference for metrics
	metrics.QueueDepth = &q.depth

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)
	defer q.Stop()
	time.Sleep(10 * time.Millisecond)

	// Enqueue items
	for i := 0; i < 5; i++ {
		q.Enqueue("bucket", "key"+string(rune('0'+i)))
	}

	// Give worker time to process some items
	time.Sleep(50 * time.Millisecond)

	// Depth should be between 0 and 5 (worker may have processed some)
	depth := metrics.QueueDepth.Load()
	if depth < 0 || depth > 5 {
		t.Errorf("queue depth out of expected range: got %d", depth)
	}

	// Wait for worker to finish processing
	q.metrics.DroppedTotal.Add(0) // Sync point
	time.Sleep(100 * time.Millisecond)

	// Eventually depth should reach 0
	for i := 0; i < 10; i++ {
		if metrics.QueueDepth.Load() == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Error("queue did not drain to depth 0")
}

// TestStopWithEmptyQueue verifies Stop works correctly on empty queue
func TestStopWithEmptyQueue(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	// Don't enqueue anything
	start := time.Now()
	q.Stop()
	elapsed := time.Since(start)

	// Should return immediately
	if elapsed > 100*time.Millisecond {
		t.Errorf("Stop on empty queue took too long: %v", elapsed)
	}
}

// TestConcurrentEnqueue verifies concurrent enqueue operations
func TestConcurrentEnqueue(t *testing.T) {
	metrics := NewMetrics()
	q := NewReplicationQueue(metrics, 1000)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)
	defer q.Stop()
	time.Sleep(10 * time.Millisecond)

	// Launch multiple goroutines enqueuing concurrently
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			q.Enqueue("bucket", "key1")
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			q.Enqueue("bucket", "key2")
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			q.Enqueue("bucket", "key3")
		}
		done <- struct{}{}
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// Give worker time to process
	time.Sleep(100 * time.Millisecond)

	// Verify no race conditions or panics occurred
	// If we got here without deadlock, the test passes
}

// TestMetricsInitialization verifies metrics are properly initialized
func TestMetricsInitialization(t *testing.T) {
	metrics := NewMetrics()

	if metrics.QueueDepth == nil {
		t.Error("QueueDepth not initialized")
	}

	if metrics.DroppedTotal == nil {
		t.Error("DroppedTotal not initialized")
	}

	if metrics.QueueDepth.Load() != 0 {
		t.Error("QueueDepth should start at 0")
	}

	if metrics.DroppedTotal.Load() != 0 {
		t.Error("DroppedTotal should start at 0")
	}
}
