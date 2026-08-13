package metrics

import (
	"sync"
	"testing"
)

// TestIncReplicationEnqueuedConcurrent verifies that IncReplicationEnqueued
// is thread-safe and increments correctly under concurrent access.
func TestIncReplicationEnqueuedConcurrent(t *testing.T) {
	m := NewMetrics()

	operations := []string{"put", "put-streaming"}
	numGoroutines := 100
	incrementsPerGoroutine := 100

	var wg sync.WaitGroup
	wg.Add(len(operations) * numGoroutines)

	// Launch concurrent goroutines that all increment the same metrics
	for _, op := range operations {
		for i := 0; i < numGoroutines; i++ {
			go func(operation string) {
				defer wg.Done()
				for j := 0; j < incrementsPerGoroutine; j++ {
					m.IncReplicationEnqueued(operation)
				}
			}(op)
		}
	}

	wg.Wait()

	// Verify final counts
	expectedCount := int64(numGoroutines * incrementsPerGoroutine)

	// Check "put" counter
	if m.replicationEnqueuedPut.Load() != expectedCount {
		t.Errorf("Expected put count to be %d, got %d", expectedCount, m.replicationEnqueuedPut.Load())
	}

	// Check "put-streaming" counter
	if m.replicationEnqueuedPutStreaming.Load() != expectedCount {
		t.Errorf("Expected put-streaming count to be %d, got %d", expectedCount, m.replicationEnqueuedPutStreaming.Load())
	}
}

// TestIncReplicationEnqueuedRaceDetector is a focused test for the race detector.
func TestIncReplicationEnqueuedRaceDetector(t *testing.T) {
	m := NewMetrics()

	// This test is designed to trigger race detector warnings if the
	// implementation is not thread-safe
	done := make(chan bool)

	go func() {
		for i := 0; i < 1000; i++ {
			m.IncReplicationEnqueued("put")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			m.IncReplicationEnqueued("put")
		}
		done <- true
	}()

	<-done
	<-done

	if m.replicationEnqueuedPut.Load() != 2000 {
		t.Errorf("Expected count to be 2000, got %d", m.replicationEnqueuedPut.Load())
	}
}
