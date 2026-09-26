package srvtest

// Server-level pin of the recovery half of the ADR-006 overwrite contract
// under an active secondary outage. The existing overwrite pins
// (TestOverwriteSinglePutConvergesOnSecondary) converge an overwrite issued
// while BOTH backends are healthy; the existing outage pins
// (TestSecondaryOutageWindowAckAndFullDrain,
// TestProviderOutageRestoreAsyncWindowLoss) write NEW keys during the window.
// Neither covers the combination that makes the window dangerous for data
// already on the mirror: a key's mirror copy going stale because the object
// was overwritten mid-outage.
//
// The property pinned here comes from the queue's retry design: every attempt
// re-runs fallbackCopy, which Get-s from the primary at RETRY time
// (internal/replication processTask → fallbackCopy), so once the secondary
// recovers the mirror must converge on the NEWEST primary ciphertext — never
// the version current when the first attempt failed, and never a corrupt mix
// of the two. A stale v1 landing at failover would silently restore
// superseded data.

import (
	"bytes"
	"errors"
	"testing"
)

func TestOverwriteDuringSecondaryOutageConvergesAfterRecovery(t *testing.T) {
	h := New(t)

	const key = "outage-overwrite/data.bin"
	storedKey := h.StoredKey(key)

	v1 := RandomBytes(t, 160*1024+11)
	h.PutObject(h.Bucket, key, v1)
	h.WaitUntilEnqueued(1, WaitTimeout)
	h.WaitUntilReplicated([]string{storedKey}, WaitTimeout)
	v1Mirror := h.Snapshot(h.Secondary)[storedKey]
	if v1Mirror == nil {
		t.Fatalf("v1 never landed on the secondary at %q", storedKey)
	}

	// Outage: every secondary write fails from here until explicitly
	// recovered.
	h.Secondary.SetPutError(errors.New("injected secondary outage: overwrite-during-window drill"))
	defer h.Secondary.SetPutError(nil)

	// The overwrite is acknowledged while the mirror is unreachable — the
	// primary ack never depends on the secondary.
	v2 := RandomBytes(t, 288*1024+5)
	h.PutObject(h.Bucket, key, v2)
	h.WaitUntilEnqueued(2, WaitTimeout)

	// Still inside the window: whatever the queue has attempted, the mirror
	// must hold exactly the untouched v1 envelope — no partial v2 bytes, no
	// torn mix. And the window must be loss-with-retries, not a drop: the
	// task stays queued for recovery.
	if mirror := h.Snapshot(h.Secondary)[storedKey]; !bytes.Equal(mirror, v1Mirror) {
		t.Fatalf("mirror changed during the faulted window; want the untouched v1 envelope (got %d bytes, v1 was %d)", len(mirror), len(v1Mirror))
	}
	if got := h.QueueMetrics.DroppedTotal.Load(); got != 0 {
		t.Fatalf("DroppedTotal = %d during the faulted window, want 0 (the task must survive to recovery)", got)
	}

	// Recovery: clear the fault and wait for the retried task to converge.
	h.Secondary.SetPutError(nil)
	waitUntilConverged(t, h, []string{storedKey}, WaitTimeout)

	v2Primary := h.Snapshot(h.Primary)[storedKey]
	if bytes.Equal(v1Mirror, v2Primary) {
		t.Fatal("v2 produced ciphertext identical to v1's; the payloads must differ for the convergence assert to mean anything")
	}
	if mirror := h.Snapshot(h.Secondary)[storedKey]; !bytes.Equal(mirror, v2Primary) {
		t.Fatalf("after recovery the mirror did not converge on the v2 ciphertext — a stale v1 landing at failover would restore superseded data")
	}

	// The work was done by the RETRY path, and nothing was dropped or
	// permanently errored along the way. (AssertQueueDrained cannot be used
	// here: it pins RetriesTotal == 0, and this drill exists to make that
	// counter move.)
	if got := h.QueueMetrics.RetriesTotal.Load(); got == 0 {
		t.Error("RetriesTotal = 0 after recovery; the convergence came from somewhere other than the retry path the drill exists to exercise")
	}
	if got := h.QueueMetrics.EnqueuedTotal.Load(); got != 2 {
		t.Errorf("EnqueuedTotal = %d, want 2 (v1 + v2)", got)
	}
	if got := h.QueueMetrics.ErrorsTotal.Load(); got != 0 {
		t.Errorf("ErrorsTotal = %d, want 0 (the outage is transient, not a permanent error)", got)
	}
	if got := h.QueueMetrics.DroppedTotal.Load(); got != 0 {
		t.Errorf("DroppedTotal = %d after recovery, want 0", got)
	}
	if got := h.QueueMetrics.QueueDepth.Load(); got != 0 {
		t.Errorf("QueueDepth = %d after convergence, want 0", got)
	}

	// Landing contract on the final state: the mirror holds exactly the v2
	// envelope, byte-identical to the primary.
	h.AssertLandingSet([]string{storedKey})
}
