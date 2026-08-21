package replication

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// helper function to create a test queue with mock backends
func newTestQueue(bufSize int) (*Metrics, *ReplicationQueue) {
	metrics := NewMetrics()
	primary := NewMockBackend()
	secondary := NewMockBackend()
	q := NewReplicationQueue(metrics, primary, secondary, bufSize, nil)
	return metrics, q
}

// TestNewReplicationQueue verifies queue initialization
func TestNewReplicationQueue(t *testing.T) {
	metrics := NewMetrics()
	primary := NewMockBackend()
	secondary := NewMockBackend()
	q := NewReplicationQueue(metrics, primary, secondary, 100, nil)

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
	primary := NewMockBackend()
	secondary := NewMockBackend()
	q := NewReplicationQueue(metrics, primary, secondary, 0, nil)

	if cap(q.queueCh) != DefaultQueueBufferSize {
		t.Errorf("expected default capacity %d, got %d", DefaultQueueBufferSize, cap(q.queueCh))
	}
}

// TestStart verifies the background goroutine starts correctly
func TestStart(t *testing.T) {
	metrics := NewMetrics()
	primary := NewMockBackend()
	secondary := NewMockBackend()
	q := NewReplicationQueue(metrics, primary, secondary, 10, nil)

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
	_, q := newTestQueue(10)

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
	_, q := newTestQueue(10)

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
	metrics, q := newTestQueue(2)

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
	metrics, q := newTestQueue(1)

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
	primary := NewMockBackend()
	secondary := NewMockBackend()
	q := NewReplicationQueue(metrics, primary, secondary, 100, nil)

	// Setup: create bucket and objects in primary backend
	ctx := context.Background()
	primary.CreateBucket(ctx, "test-bucket")

	// Put a test object in primary
	testData := "test-object-data"
	primary.Put(ctx, "test-bucket", "test-key", strings.NewReader(testData), int64(len(testData)), map[string]string{"Content-Type": "text/plain"})

	// Create bucket in secondary
	secondary.CreateBucket(ctx, "test-bucket")

	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(startCtx)
	time.Sleep(10 * time.Millisecond)

	// Enqueue one item (this one should succeed)
	q.Enqueue("test-bucket", "test-key")

	// Give worker time to process
	time.Sleep(100 * time.Millisecond)

	// Stop should drain all items quickly
	start := time.Now()
	q.Stop()
	elapsed := time.Since(start)

	// Stop should wait for drain (but not too long)
	if elapsed > 1*time.Second {
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
	primary := NewMockBackend()
	secondary := NewMockBackend()
	q := NewReplicationQueue(metrics, primary, secondary, 100, nil)

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
	primary := NewMockBackend()
	secondary := NewMockBackend()
	q := NewReplicationQueue(metrics, primary, secondary, 100, nil)

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
	primary := NewMockBackend()
	secondary := NewMockBackend()
	q := NewReplicationQueue(metrics, primary, secondary, 10, nil)

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
	primary := NewMockBackend()
	secondary := NewMockBackend()
	q := NewReplicationQueue(metrics, primary, secondary, 1000, nil)

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

// TestIsTransientError verifies the error classification logic.
func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		// Transient errors (should return true)
		{"nil error", nil, false},
		{"timeout", errors.New("context timeout exceeded"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"rate limit", errors.New("rate limit exceeded"), true},
		{"too many requests", errors.New("too many requests"), true},
		{"service unavailable", errors.New("service unavailable"), true},
		{"gateway timeout", errors.New("gateway timeout"), true},
		{"bad gateway", errors.New("bad gateway"), true},
		{"network unreachable", errors.New("network unreachable"), true},
		{"context deadline exceeded", context.DeadlineExceeded, true},
		{"context canceled", context.Canceled, true},
		{"tcp read error", errors.New("read tcp 192.168.1.1:80"), true},
		{"tcp write error", errors.New("write tcp 192.168.1.1:80"), true},

		// Permanent errors (should return false)
		{"not found", errors.New("object not found"), false},
		{"no such bucket", errors.New("no such bucket"), false},
		{"does not exist", errors.New("bucket does not exist"), false},
		{"access denied", errors.New("access denied"), false},
		{"forbidden", errors.New("forbidden"), false},
		{"unauthorized", errors.New("unauthorized"), false},
		{"invalid bucket", errors.New("invalid bucket name"), false},
		{"bucket not found", errors.New("bucket not found"), false},
		{"authentication failed", errors.New("authentication failed"), false},
		{"permission denied", errors.New("permission denied"), false},

		// Unknown errors - default to transient (true)
		{"unknown error", errors.New("some unknown error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTransientError(tt.err)
			if result != tt.expected {
				t.Errorf("isTransientError(%q) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestRetryWithTransientError verifies that transient errors trigger retries.
func TestRetryWithTransientError(t *testing.T) {
	metrics := NewMetrics()
	primary := NewMockBackend()

	// Create a secondary that fails with transient errors initially
	attempts := 0
	failBackend := &failingBackend{
		MockBackend: NewMockBackend(),
		shouldFail: func() bool {
			attempts++
			return attempts <= 2 // Fail first 2 attempts, succeed on 3rd
		},
		failErr: errors.New("connection timeout"),
	}

	q := NewReplicationQueue(metrics, primary, failBackend, 100, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup: create bucket and objects
	ctxSetup := context.Background()
	primary.CreateBucket(ctxSetup, "test-bucket")
	testData := "test-object-data"
	primary.Put(ctxSetup, "test-bucket", "test-key", strings.NewReader(testData), int64(len(testData)), map[string]string{"Content-Type": "text/plain"})
	failBackend.CreateBucket(ctxSetup, "test-bucket")

	q.Start(ctx)
	defer q.Stop()
	time.Sleep(10 * time.Millisecond)

	// Enqueue item that will fail initially
	q.Enqueue("test-bucket", "test-key")

	// Wait for retries to complete
	time.Sleep(500 * time.Millisecond)

	// Verify retries occurred
	retries := metrics.RetriesTotal.Load()
	if retries < 1 {
		t.Errorf("expected at least 1 retry for transient errors, got %d", retries)
	}

	// Verify no permanent error recorded (should have succeeded on retry)
	errors := metrics.ErrorsTotal.Load()
	if errors != 0 {
		t.Errorf("expected no errors after successful retry, got %d", errors)
	}
}

// TestNoRetryForPermanentError verifies that permanent errors skip retries.
func TestNoRetryForPermanentError(t *testing.T) {
	metrics := NewMetrics()
	primary := NewMockBackend()

	// Create a secondary that fails with permanent errors
	permanentBackend := &failingBackend{
		MockBackend: NewMockBackend(),
		shouldFail: func() bool {
			return true // Always fail
		},
		failErr: errors.New("bucket not found"),
	}

	q := NewReplicationQueue(metrics, primary, permanentBackend, 100, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup: create bucket and objects
	ctxSetup := context.Background()
	primary.CreateBucket(ctxSetup, "test-bucket")
	testData := "test-object-data"
	primary.Put(ctxSetup, "test-bucket", "test-key", strings.NewReader(testData), int64(len(testData)), map[string]string{"Content-Type": "text/plain"})

	q.Start(ctx)
	defer q.Stop()
	time.Sleep(10 * time.Millisecond)

	// Enqueue item that will fail with permanent error
	q.Enqueue("test-bucket", "test-key")

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify NO retries occurred for permanent error
	retries := metrics.RetriesTotal.Load()
	if retries != 0 {
		t.Errorf("expected no retries for permanent errors, got %d", retries)
	}

	// Verify error was recorded
	errors := metrics.ErrorsTotal.Load()
	if errors != 1 {
		t.Errorf("expected 1 error for permanent failure, got %d", errors)
	}
}

// failingBackend wraps a backend and simulates failures for testing.
type failingBackend struct {
	*MockBackend
	shouldFail func() bool
	failErr    error
}

func (f *failingBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	if f.shouldFail() {
		return f.failErr
	}
	return f.MockBackend.Copy(ctx, srcBucket, srcKey, dstBucket, dstKey, meta, replaceMetadata)
}

func (f *failingBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	if f.shouldFail() {
		return f.failErr
	}
	return f.MockBackend.Put(ctx, bucket, key, body, size, meta)
}

// TestReplicationPipelineEndToEnd verifies the complete replication pipeline:
// 1. Enqueue objects for replication
// 2. Worker drains and processes them
// 3. Successful copy to secondary backend
// 4. Metrics emitted (lag and duration)
// 5. Worker exits gracefully
func TestReplicationPipelineEndToEnd(t *testing.T) {
	metrics := NewMetrics()
	primary := NewMockBackend()
	secondary := NewMockBackend()
	q := NewReplicationQueue(metrics, primary, secondary, 100, nil)

	ctx := context.Background()

	// Setup: create bucket and populate with test objects in primary backend
	testBucket := "test-bucket"
	primary.CreateBucket(ctx, testBucket)
	secondary.CreateBucket(ctx, testBucket)

	// Create test objects with different sizes to test copy duration
	testCases := []struct {
		key         string
		data        string
		contentType string
	}{
		{"small-object.txt", "small", "text/plain"},
		{"medium-object.txt", strings.Repeat("medium", 100), "text/plain"},
		{"large-object.txt", strings.Repeat("large", 1000), "text/plain"},
	}

	// Put test objects in primary backend
	for _, tc := range testCases {
		err := primary.Put(ctx, testBucket, tc.key, strings.NewReader(tc.data), int64(len(tc.data)), map[string]string{
			"Content-Type": tc.contentType,
		})
		if err != nil {
			t.Fatalf("failed to put test object %s: %v", tc.key, err)
		}
	}

	// Start the worker
	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(startCtx)
	time.Sleep(10 * time.Millisecond)

	// Enqueue all test objects for replication
	for _, tc := range testCases {
		q.Enqueue(testBucket, tc.key)
	}

	// Wait a moment for processing to begin and lag to be calculated
	// The lag metric is updated when tasks are processed, so we need to wait
	// for at least one task to start processing
	time.Sleep(100 * time.Millisecond)

	// Verify lag metric is updated (should be >= 0, could be 0 if processing is fast)
	lag := metrics.LagSeconds.Load()
	// Lag can be 0 if processing completed before our check, which is fine
	// The important thing is that it doesn't stay non-zero after completion
	if lag < 0 {
		t.Errorf("expected lag >= 0, got %d", lag)
	}

	// Stop the queue - this should drain all remaining items
	q.Stop()

	// Verify all objects were replicated to secondary backend
	for _, tc := range testCases {
		// Verify object exists in secondary
		body, info, err := secondary.Get(ctx, testBucket, tc.key)
		if err != nil {
			t.Errorf("object %s not found in secondary backend: %v", tc.key, err)
			continue
		}
		defer body.Close()

		// Verify content
		data, err := io.ReadAll(body)
		if err != nil {
			t.Errorf("failed to read object %s from secondary: %v", tc.key, err)
			continue
		}

		if string(data) != tc.data {
			t.Errorf("content mismatch for %s: expected %q, got %q", tc.key, tc.data, string(data))
		}

		// Verify metadata
		if info.ContentType != tc.contentType {
			t.Errorf("content-type mismatch for %s: expected %s, got %s", tc.key, tc.contentType, info.ContentType)
		}
	}

	// Verify metrics were emitted
	if metrics.ErrorsTotal.Load() != 0 {
		t.Errorf("expected no errors, got %d", metrics.ErrorsTotal.Load())
	}

	copyCount := metrics.CopyDurationSeconds.GetCount()
	if copyCount != int64(len(testCases)) {
		t.Errorf("expected %d copy duration observations, got %d", len(testCases), copyCount)
	}

	// Verify queue depth is 0 after stop
	if depth := metrics.QueueDepth.Load(); depth != 0 {
		t.Errorf("expected queue depth 0 after stop, got %d", depth)
	}

	// Verify lag metric is 0 after all items processed
	if lag := metrics.LagSeconds.Load(); lag != 0 {
		t.Errorf("expected lag 0 after all items processed, got %d", lag)
	}

	// Verify no items were dropped
	if dropped := metrics.DroppedTotal.Load(); dropped != 0 {
		t.Errorf("expected 0 dropped items, got %d", dropped)
	}

	// Verify Prometheus format includes our metrics
	prometheus := metrics.PrometheusFormat()
	if !strings.Contains(prometheus, "armor_replication_lag_seconds") {
		t.Error("Prometheus format missing replication_lag_seconds metric")
	}
	if !strings.Contains(prometheus, "armor_replication_copy_duration_seconds") {
		t.Error("Prometheus format missing replication_copy_duration_seconds metric")
	}
	if !strings.Contains(prometheus, "armor_replication_queue_depth") {
		t.Error("Prometheus format missing replication_queue_depth metric")
	}
}

// TestReplicationLagMetricTracking verifies that the lag metric accurately
// tracks the age of the oldest unreplicated object.
func TestReplicationLagMetricTracking(t *testing.T) {
	metrics := NewMetrics()
	primary := NewMockBackend()
	secondary := NewMockBackend()
	q := NewReplicationQueue(metrics, primary, secondary, 100, nil)

	ctx := context.Background()

	// Setup: create bucket and test object
	testBucket := "test-bucket"
	primary.CreateBucket(ctx, testBucket)
	secondary.CreateBucket(ctx, testBucket)

	testData := "test-data-for-lag-tracking"
	primary.Put(ctx, testBucket, "test-key", strings.NewReader(testData), int64(len(testData)), nil)

	// Start the worker
	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(startCtx)
	time.Sleep(10 * time.Millisecond)

	// Enqueue an item
	q.Enqueue(testBucket, "test-key")

	// Wait briefly for lag to be calculated
	time.Sleep(100 * time.Millisecond)

	// Verify lag metric is non-negative (could be 0 if processed quickly)
	lag := metrics.LagSeconds.Load()
	if lag < 0 {
		t.Errorf("expected lag >= 0, got %d", lag)
	}
	// The lag can be 0 if the task was processed before we checked, which is fine
	// What matters is that it returns to 0 after all items are processed

	// Stop and wait for drain
	q.Stop()

	// Verify lag metric returns to 0 after all items processed
	lag = metrics.LagSeconds.Load()
	if lag != 0 {
		t.Errorf("expected lag 0 after all items processed, got %d", lag)
	}
}

// TestCopyDurationHistogram verifies that the copy duration histogram
// correctly records operation durations.
func TestCopyDurationHistogram(t *testing.T) {
	h := newCopyDurationHistogram()

	// Record some durations
	durations := []float64{0.05, 0.2, 0.8, 1.5, 5.0, 15.0, 45.0, 120.0}
	for _, d := range durations {
		h.Observe(d)
	}

	totalCount := int64(len(durations))
	if h.GetCount() != totalCount {
		t.Errorf("expected count %d, got %d", totalCount, h.GetCount())
	}

	// Verify sum is non-zero
	if h.GetSum() == 0 {
		t.Error("expected non-zero sum")
	}

	// Verify Prometheus format
	prometheus := h.PrometheusFormat("test_copy_duration")
	if !strings.Contains(prometheus, "test_copy_duration_bucket") {
		t.Error("Prometheus format missing bucket lines")
	}
	if !strings.Contains(prometheus, "test_copy_duration_sum") {
		t.Error("Prometheus format missing sum")
	}
	if !strings.Contains(prometheus, "test_copy_duration_count") {
		t.Error("Prometheus format missing count")
	}

	// Verify we have the +Inf bucket
	if !strings.Contains(prometheus, "+Inf") {
		t.Error("Prometheus format missing +Inf bucket")
	}
}
