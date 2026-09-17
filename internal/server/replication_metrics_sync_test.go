package server

import (
	"context"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/metrics"
	"github.com/jedarden/armor/internal/replication"
)

// TestPublishReplicationMetricsSyncsExpvars verifies that the server publishes
// the replication queue's authoritative counters into the expvar-backed metrics
// behind /metrics and the dashboard. Without this bridge the armor_replication_*
// series are emitted but stay at zero regardless of replication activity.
func TestPublishReplicationMetricsSyncsExpvars(t *testing.T) {
	qm := replication.NewMetrics()
	m := metrics.NewMetrics()
	// Construct the queue before setting values: its constructor rewires
	// qm.QueueDepth onto the queue's own internal depth atomic.
	s := &Server{
		metrics:          m,
		replicationQueue: replication.NewReplicationQueue(qm, nil, nil, 0, nil),
	}
	qm.QueueDepth.Store(3)
	qm.LagSeconds.Store(42)
	qm.DroppedTotal.Store(1)
	qm.ErrorsTotal.Store(2)
	qm.RetriesTotal.Store(7)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.publishReplicationMetrics(ctx)

	// The first snapshot is published before the first tick, so this only waits
	// for goroutine scheduling, not for replicationMetricsSyncInterval.
	deadline := time.Now().Add(2 * time.Second)
	for m.ReplicationQueueDepth.String() != "3" || m.ReplicationLagSeconds.String() != "42" {
		if time.Now().After(deadline) {
			t.Fatalf("replication stats not published: depth=%s lag=%s dropped=%s errors=%s retries=%s",
				m.ReplicationQueueDepth.String(), m.ReplicationLagSeconds.String(),
				m.ReplicationDroppedTotal.String(), m.ReplicationErrorsTotal.String(),
				m.ReplicationRetriesTotal.String())
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got := m.ReplicationDroppedTotal.String(); got != "1" {
		t.Errorf("ReplicationDroppedTotal = %s, want 1", got)
	}
	if got := m.ReplicationErrorsTotal.String(); got != "2" {
		t.Errorf("ReplicationErrorsTotal = %s, want 2", got)
	}
	if got := m.ReplicationRetriesTotal.String(); got != "7" {
		t.Errorf("ReplicationRetriesTotal = %s, want 7", got)
	}

	// Once the publisher is parked on its ticker, cancelling the context must
	// stop it: a later queue-side change must not be republished. (The publisher
	// sets before selecting, so waiting for the snapshot above guarantees the
	// cancel races nothing.)
	cancel()
	qm.QueueDepth.Store(99)
	time.Sleep(50 * time.Millisecond)
	if got := m.ReplicationQueueDepth.String(); got != "3" {
		t.Errorf("ReplicationQueueDepth = %s after ctx cancel, want 3 (publisher should have stopped)", got)
	}
}
