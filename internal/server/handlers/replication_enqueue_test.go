// Package handlers provides integration tests for replication enqueue behavior.
package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/metrics"
)

// getReplicationEnqueuedPut parses the Prometheus format output to extract
// the replication_enqueued_total metric for "put" operations.
func getReplicationEnqueuedPut(m *metrics.Metrics) int64 {
	output := m.PrometheusFormat()
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "armor_replication_enqueued_total{operation=\"put\"}") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				value, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
				if err == nil {
					return value
				}
			}
		}
	}
	return 0
}

// mockReplicationQueue is a mock implementation of replication.Enqueuer for testing.
// It tracks whether Enqueue was called and with what parameters.
type mockReplicationQueue struct {
	enqueueCalled   *atomic.Bool
	enqueueCount    *atomic.Int64
	lastBucket      atomic.Value // string
	lastKey         atomic.Value // string
	shouldFail      *atomic.Bool  // if true, Enqueue will panic (simulating failure)
	enqueueDone     chan struct{} // signaled when Enqueue completes
}

func (m *mockReplicationQueue) Enqueue(bucket, key string) {
	defer func() {
		// Signal that Enqueue has completed
		select {
		case m.enqueueDone <- struct{}{}:
		default:
		}
	}()

	m.enqueueCalled.Store(true)
	m.enqueueCount.Add(1)
	m.lastBucket.Store(bucket)
	m.lastKey.Store(key)

	if m.shouldFail.Load() {
		// Simulate a failure during enqueue
		panic("replication enqueue failed")
	}
}

// newMockReplicationQueue creates a new properly initialized mock queue.
func newMockReplicationQueue() *mockReplicationQueue {
	return &mockReplicationQueue{
		enqueueCalled: &atomic.Bool{},
		enqueueCount:  &atomic.Int64{},
		shouldFail:    &atomic.Bool{},
		enqueueDone:   make(chan struct{}, 10),
	}
}

// waitForEnqueue waits for the next Enqueue call to complete with a timeout.
// Returns true if Enqueue was called, false if timeout is reached.
func (m *mockReplicationQueue) waitForEnqueue() bool {
	select {
	case <-m.enqueueDone:
		return true
	case <-time.After(100 * time.Millisecond):
		return false
	}
}

// reset clears the mock state for reuse.
func (m *mockReplicationQueue) reset() {
	m.enqueueCalled.Store(false)
	m.enqueueCount.Store(0)
	m.lastBucket.Store("")
	m.lastKey.Store("")
	m.shouldFail.Store(false)
}

// wasCalled returns true if Enqueue was called at least once.
func (m *mockReplicationQueue) wasCalled() bool {
	return m.enqueueCalled.Load()
}

// callCount returns the number of times Enqueue was called.
func (m *mockReplicationQueue) callCount() int64 {
	return m.enqueueCount.Load()
}

// getLastCall returns the bucket and key from the most recent Enqueue call.
func (m *mockReplicationQueue) getLastCall() (bucket, key string) {
	bucket, _ = m.lastBucket.Load().(string)
	key, _ = m.lastKey.Load().(string)
	return
}

// setShouldFail configures whether Enqueue should panic (simulate failure).
func (m *mockReplicationQueue) setShouldFail(fail bool) {
	m.shouldFail.Store(fail)
}

// setupTestHandler creates a Handlers instance configured for testing.
// Returns the handler, primary backend, and cleanup function.
func setupTestHandler(t *testing.T, withReplicationQueue bool) (*Handlers, *backend.FSBackend, func()) {
	t.Helper()

	// Create primary filesystem backend
	primaryDir := t.TempDir()
	primaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: primaryDir})
	if err != nil {
		t.Fatalf("failed to create primary backend: %v", err)
	}

	// Common configuration
	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test",
		AuthSecretKey: "test",
	}

	// Generate a MEK for the key manager
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	// Create a key manager
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create keymanager: %v", err)
	}

	// Create metrics
	mets := metrics.NewMetrics()

	// Create handler
	h := &Handlers{
		config:      cfg,
		backend:     primaryBackend,
		cache:       backend.NewMetadataCache(1000, 300),
		footerCache: backend.NewFooterCache(1000, 300),
		listCache:   backend.NewListCache(1000, 300),
		keyManager:  km,
		metrics:     mets,
	}

	// Optionally wire in replication queue
	if withReplicationQueue {
		mockQueue := newMockReplicationQueue()
		h.WithReplicationQueue(mockQueue)
	}

	cleanup := func() {
		// Cleanup is handled by t.TempDir()
	}

	return h, primaryBackend, cleanup
}

// TestReplicationEnqueueAfterPutObject verifies that objects are enqueued
// after successful PutObject to the primary backend.
func TestReplicationEnqueueAfterPutObject(t *testing.T) {
	h, primaryBackend, cleanup := setupTestHandler(t, true)
	defer cleanup()

	// Get the mock queue (it's wired in via WithReplicationQueue)
	mockQueue, ok := h.replicationQueue.(*mockReplicationQueue)
	if !ok {
		t.Fatal("replication queue is not mockReplicationQueue")
	}

	// Create test data
	testData := []byte("hello, world")
	bucket := "test-bucket"
	key := "test-object.txt"

	// Create HTTP request
	req := httptest.NewRequest("PUT", "/"+bucket+"/"+key, bytes.NewReader(testData))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	// Call PutObject
	h.PutObject(w, req, bucket, key)

	// Wait for the enqueue goroutine to complete
	if !mockQueue.waitForEnqueue() {
		t.Error("timeout waiting for enqueue to complete")
	}

	// Verify the response was successful
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	// Verify the object was stored in primary backend
	ctx := context.Background()
	body, info, err := primaryBackend.Get(ctx, bucket, key)
	if err != nil {
		t.Fatalf("failed to get object from primary backend: %v", err)
	}
	defer body.Close()

	storedData, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read stored object: %v", err)
	}

	// Note: stored data is encrypted, so it won't match test data exactly
	if len(storedData) == 0 {
		t.Error("stored data is empty")
	}

	if info == nil {
		t.Fatal("object info is nil")
	}

	// Verify replication queue Enqueue was called exactly once
	if !mockQueue.wasCalled() {
		t.Error("replication queue Enqueue was not called after PutObject")
	}

	if mockQueue.callCount() != 1 {
		t.Errorf("expected Enqueue to be called once, got %d calls", mockQueue.callCount())
	}

	// Verify the bucket and key passed to Enqueue match the PutObject parameters
	enqueuedBucket, enqueuedKey := mockQueue.getLastCall()
	if enqueuedBucket != bucket {
		t.Errorf("expected bucket %q, got %q", bucket, enqueuedBucket)
	}

	if enqueuedKey != key {
		t.Errorf("expected key %q, got %q", key, enqueuedKey)
	}

	// Verify the replication_enqueued_total metric was incremented
	initialCount := getReplicationEnqueuedPut(h.metrics)
	if initialCount != 1 {
		t.Errorf("expected replication_enqueued_total to be 1, got %d", initialCount)
	}
}

// TestReplicationEnqueueNilQueue verifies that no enqueue happens when
// ARMOR_SECONDARY_BACKEND is not configured (nil queue).
func TestReplicationEnqueueNilQueue(t *testing.T) {
	h, primaryBackend, cleanup := setupTestHandler(t, false)
	defer cleanup()

	// Verify replication queue is nil
	if h.replicationQueue != nil {
		t.Error("expected replication queue to be nil when not configured")
	}

	// Create test data
	testData := []byte("hello, world")
	bucket := "test-bucket"
	key := "test-object.txt"

	// Create HTTP request
	req := httptest.NewRequest("PUT", "/"+bucket+"/"+key, bytes.NewReader(testData))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	// Call PutObject
	h.PutObject(w, req, bucket, key)

	// Verify the response was successful (put should succeed even without replication)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	// Verify the object was stored in primary backend
	ctx := context.Background()
	body, info, err := primaryBackend.Get(ctx, bucket, key)
	if err != nil {
		t.Fatalf("failed to get object from primary backend: %v", err)
	}
	defer body.Close()

	if info == nil {
		t.Fatal("object info is nil")
	}

	// Verify no replication metric was incremented
	metricCount := getReplicationEnqueuedPut(h.metrics)
	if metricCount != 0 {
		t.Errorf("expected replication_enqueued_total to be 0, got %d", metricCount)
	}
}

// TestReplicationEnqueueFailureDoesNotAffectPutObject verifies that enqueue
// failures are logged but don't affect PutObject response.
func TestReplicationEnqueueFailureDoesNotAffectPutObject(t *testing.T) {
	h, primaryBackend, cleanup := setupTestHandler(t, true)
	defer cleanup()

	// Get the mock queue and configure it to fail
	mockQueue, ok := h.replicationQueue.(*mockReplicationQueue)
	if !ok {
		t.Fatal("replication queue is not mockReplicationQueue")
	}

	// Configure the mock to panic on Enqueue (simulate failure)
	mockQueue.setShouldFail(true)

	// Create test data
	testData := []byte("hello, world")
	bucket := "test-bucket"
	key := "test-object.txt"

	// Create HTTP request
	req := httptest.NewRequest("PUT", "/"+bucket+"/"+key, bytes.NewReader(testData))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	// Call PutObject - this should still succeed even though enqueue will panic
	// The panic is caught by the defer in the goroutine
	h.PutObject(w, req, bucket, key)

	// Wait for the enqueue goroutine to complete (or panic)
	mockQueue.waitForEnqueue()

	// Verify the response was successful (enqueue failure should not affect response)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK despite enqueue failure, got %d", resp.StatusCode)
	}

	// Verify the object was stored in primary backend (put succeeded)
	ctx := context.Background()
	body, info, err := primaryBackend.Get(ctx, bucket, key)
	if err != nil {
		t.Fatalf("failed to get object from primary backend: %v", err)
	}
	defer body.Close()

	if info == nil {
		t.Fatal("object info is nil")
	}

	// Verify that the enqueue was attempted (called before panic)
	if !mockQueue.wasCalled() {
		t.Error("enqueue was not called before failure")
	}
}

// TestReplicationEnqueueMultiplePutObjects verifies that each PutObject
// results in a separate Enqueue call.
func TestReplicationEnqueueMultiplePutObjects(t *testing.T) {
	h, _, cleanup := setupTestHandler(t, true)
	defer cleanup()

	// Get the mock queue
	mockQueue, ok := h.replicationQueue.(*mockReplicationQueue)
	if !ok {
		t.Fatal("replication queue is not mockReplicationQueue")
	}

	bucket := "test-bucket"
	numUploads := 5

	// Perform multiple PutObject operations
	for i := 0; i < numUploads; i++ {
		testData := []byte(fmt.Sprintf("test data %d", i))
		key := fmt.Sprintf("test-object-%d.txt", i)

		req := httptest.NewRequest("PUT", "/"+bucket+"/"+key, bytes.NewReader(testData))
		req.Header.Set("Content-Type", "application/octet-stream")
		w := httptest.NewRecorder()

		h.PutObject(w, req, bucket, key)

		// Wait for enqueue to complete
		mockQueue.waitForEnqueue()

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("upload %d: expected status 200 OK, got %d", i, resp.StatusCode)
		}
	}

	// Verify Enqueue was called for each upload
	if mockQueue.callCount() != int64(numUploads) {
		t.Errorf("expected %d Enqueue calls, got %d", numUploads, mockQueue.callCount())
	}

	// Verify metric was incremented for each upload
	metricCount := getReplicationEnqueuedPut(h.metrics)
	if metricCount != int64(numUploads) {
		t.Errorf("expected replication_enqueued_total to be %d, got %d", numUploads, metricCount)
	}
}

// TestReplicationEnqueueWithDifferentBucketsAndKeys verifies that Enqueue
// is called with the correct bucket and key combinations.
func TestReplicationEnqueueWithDifferentBucketsAndKeys(t *testing.T) {
	h, _, cleanup := setupTestHandler(t, true)
	defer cleanup()

	// Get the mock queue
	mockQueue, ok := h.replicationQueue.(*mockReplicationQueue)
	if !ok {
		t.Fatal("replication queue is not mockReplicationQueue")
	}

	testCases := []struct {
		bucket string
		key    string
	}{
		{"bucket-1", "path/to/object1.txt"},
		{"bucket-2", "nested/path/object2.bin"},
		{"bucket-1", "another-object.json"},
	}

	// Perform uploads and verify enqueue parameters
	for _, tc := range testCases {
		mockQueue.reset()

		testData := []byte("test data")
		req := httptest.NewRequest("PUT", "/"+tc.bucket+"/"+tc.key, bytes.NewReader(testData))
		req.Header.Set("Content-Type", "application/octet-stream")
		w := httptest.NewRecorder()

		h.PutObject(w, req, tc.bucket, tc.key)

		// Wait for the enqueue goroutine to complete
		if !mockQueue.waitForEnqueue() {
			t.Errorf("put %s/%s: timeout waiting for enqueue to complete", tc.bucket, tc.key)
		}

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("put %s/%s: expected status 200 OK, got %d", tc.bucket, tc.key, resp.StatusCode)
		}

		// Verify enqueue was called with correct parameters
		if !mockQueue.wasCalled() {
			t.Errorf("put %s/%s: Enqueue was not called", tc.bucket, tc.key)
		}

		enqueuedBucket, enqueuedKey := mockQueue.getLastCall()
		if enqueuedBucket != tc.bucket {
			t.Errorf("put %s/%s: expected bucket %q, got %q", tc.bucket, tc.key, tc.bucket, enqueuedBucket)
		}

		if enqueuedKey != tc.key {
			t.Errorf("put %s/%s: expected key %q, got %q", tc.bucket, tc.key, tc.key, enqueuedKey)
		}
	}
}
