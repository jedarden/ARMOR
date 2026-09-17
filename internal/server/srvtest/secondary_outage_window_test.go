package srvtest

// Server-level e2e verification of ADR-006 decision #1 / rejected alternative
// 2: a secondary that is DOWN or SLOW must never delay or fail the
// client-facing write ack, and the replication queue must catch up once the
// secondary recovers. The queue-level twins
// (TestEnqueueDoesNotWaitForSecondaryWrite, TestTransientSecondaryOutageEventuallyRecovers
// in internal/replication/queue_test.go) drive q.Enqueue directly over mock
// backends; these tests write through the real authenticated S3 mux during a
// multi-object fault window — PUT *and* CompleteMultipartUpload — and watch
// the depth/lag gauges move coherently across the whole cycle: flat before,
// risen while the fault holds the single worker on the head-of-line task,
// zero again after the drain.

import (
	"errors"
	"testing"
	"time"
)

// ackBound is the ceiling for a single client write ack while the secondary
// is failing outright. Generous against a loaded box, but far below any
// synchronous-dual-write behavior a regression would reintroduce.
const ackBound = time.Second

// pollFor polls cond every PollInterval until it holds or the deadline
// expires, returning its last value. Faulted-queue state transitions asserted
// here are steady states (a head-of-line task held by the serial worker, a
// gauge between task completions), so polling keeps the assertions
// deterministic without loosening them.
func pollFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(PollInterval)
	}
	t.Fatalf("condition %q not met within %s", what, timeout)
}

// writeFaultWindow issues every client write shape (three PUTs, two complete
// multipart uploads) through the real mux while the secondary fault is
// active, timing each ack. Returns the stored (client → backend) keys in
// write order. Ack success is enforced by the harness helpers; the timings
// are checked by the caller against its own bound.
func writeFaultWindow(t *testing.T, h *Harness, multipartAckBound time.Duration) []string {
	t.Helper()

	var stored []string
	putKeys := []string{"window/put-a.bin", "window/put-b.bin", "window/put-c.txt"}
	bodies := [][]byte{
		RandomBytes(t, 64*1024+11),
		RandomBytes(t, 200*1024+37),
		[]byte("plaintext audit log line written during the secondary fault window"),
	}
	for i, key := range putKeys {
		start := time.Now()
		h.PutObject(h.Bucket, key, bodies[i])
		if ack := time.Since(start); ack > ackBound {
			t.Errorf("PUT %s ack took %v with the secondary faulted; the ack must never wait on the secondary", key, ack)
		}
		stored = append(stored, h.StoredKey(key))
	}

	for _, key := range []string{"window/mp1.dump", "window/mp2.dump"} {
		start := time.Now()
		h.MultipartUpload(h.Bucket, key, RandomBytes(t, minMultipartPart))
		// The bound covers the whole CreateMultipartUpload → UploadPart →
		// CompleteMultipartUpload flow (three HTTP round-trips plus local
		// assembly); only the complete touches the replication path, and the
		// complete alone is what a synchronous dual-write regression would
		// stall on the faulted secondary.
		if ack := time.Since(start); ack > multipartAckBound {
			t.Errorf("CompleteMultipartUpload flow for %s took %v with the secondary faulted; the ack must never wait on the secondary", key, ack)
		}
		stored = append(stored, h.StoredKey(key))
	}
	return stored
}

// assertAllMissing asserts none of the stored keys is present on the
// secondary — used mid-outage, before recovery.
func assertAllMissing(t *testing.T, h *Harness, stored []string) {
	t.Helper()
	snap := h.Snapshot(h.Secondary)
	for _, k := range stored {
		if _, ok := snap[k]; ok {
			t.Fatalf("object %q landed on the secondary while the fault was still active", k)
		}
	}
}

// TestSecondaryOutageWindowAckAndFullDrain holds the secondary in a failing
// state across a five-object write window (3 PUT + 2 CompleteMultipartUpload)
// and asserts, at the server boundary: every ack succeeded within the bound;
// the queue depth gauge ROSE during the outage (tasks backed up behind the
// head-of-line task the serial worker is stuck retrying); nothing landed
// while the fault held; and after recovery the queue fully drained with every
// faulted-window object on the secondary byte-identical and depth/lag back to
// zero.
func TestSecondaryOutageWindowAckAndFullDrain(t *testing.T) {
	h := New(t)

	if got := h.QueueMetrics.QueueDepth.Load(); got != 0 {
		t.Fatalf("QueueDepth = %d before any write, want 0", got)
	}
	if got := h.QueueMetrics.LagSeconds.Load(); got != 0 {
		t.Fatalf("LagSeconds = %d before any write, want 0", got)
	}

	h.Secondary.SetPutError(errors.New("injected secondary outage: service unavailable"))

	const wantTasks = 5
	stored := writeFaultWindow(t, h, 2*ackBound)
	if len(stored) != wantTasks {
		t.Fatalf("write window produced %d tasks, want %d", len(stored), wantTasks)
	}
	h.WaitUntilEnqueued(wantTasks, WaitTimeout)

	// Depth during the outage: the serial worker holds the head-of-line task
	// in a transient-retry loop until the fault clears, so every later task
	// stays queued — exactly N-1, deterministically, while the fault holds.
	// A synchronous dual-write regression instead shows depth 0 here (or
	// never acks at all).
	pollFor(t, WaitTimeout, "queue depth to rise to N-1 during the outage", func() bool {
		return h.QueueMetrics.QueueDepth.Load() >= wantTasks-1
	})
	if got := h.QueueMetrics.QueueDepth.Load(); got > wantTasks {
		t.Errorf("QueueDepth = %d during outage, want <= %d", got, wantTasks)
	}

	// The fault was actually exercised: the worker attempted the secondary
	// write and classified the failure as transient (retry), not permanent.
	pollFor(t, WaitTimeout, "retries to be recorded against the faulted secondary", func() bool {
		return h.QueueMetrics.RetriesTotal.Load() > 0
	})
	if h.Secondary.PutCalls() == 0 {
		t.Fatal("replication queue never attempted the secondary write")
	}
	assertAllMissing(t, h, stored)

	// Recovery: the queue must catch up on the whole faulted window.
	h.Secondary.SetPutError(nil)
	h.WaitUntilReplicated(stored, WaitTimeout)

	h.AssertLandingSet(stored)

	// Post-drain gauge coherence: depth back to zero, lag fully caught up.
	pollFor(t, WaitTimeout, "queue depth to return to 0 after the drain", func() bool {
		return h.QueueMetrics.QueueDepth.Load() == 0
	})
	if got := h.QueueMetrics.LagSeconds.Load(); got != 0 {
		t.Errorf("LagSeconds = %d after full drain, want 0", got)
	}
	if got := h.QueueMetrics.EnqueuedTotal.Load(); got != wantTasks {
		t.Errorf("EnqueuedTotal = %d, want %d", got, wantTasks)
	}
	if got := h.QueueMetrics.DroppedTotal.Load(); got != 0 {
		t.Errorf("DroppedTotal = %d, want 0 (transient failures must be retried, not dropped)", got)
	}
	if got := h.QueueMetrics.ErrorsTotal.Load(); got != 0 {
		t.Errorf("ErrorsTotal = %d, want 0 (recovered transient failures are not errors)", got)
	}
}

// TestSlowSecondaryWindowAckAndFullDrain replaces the failing secondary with
// a slow one (every replication Put sleeps far longer than a healthy ack) and
// runs the same multi-object window. It additionally pins the lag gauge's
// mid-drain behavior: once the head-of-line task lands, the age of the still-
// pending tasks behind it must be visible as armor_replication_lag_seconds >=
// 1 while the serial worker works through the delay, then return to 0 with
// depth once the queue fully drains.
func TestSlowSecondaryWindowAckAndFullDrain(t *testing.T) {
	h := New(t)

	const putDelay = 1100 * time.Millisecond
	h.Secondary.SetPutDelay(putDelay)

	// Two PUTs and one multipart upload: three replication tasks, enough to
	// observe depth backup AND the lag gauge between consecutive completions.
	stored := []string{
		h.StoredKey("slow-window/put-a.bin"),
		h.StoredKey("slow-window/put-b.bin"),
		h.StoredKey("slow-window/mp1.dump"),
	}

	// PUT acks are bounded by the injected delay itself: a healthy ack is
	// milliseconds, and the secondary write being timed takes putDelay — so
	// ack >= putDelay can only mean the ack waited on the secondary.
	start := time.Now()
	h.PutObject(h.Bucket, "slow-window/put-a.bin", RandomBytes(t, 64*1024+11))
	if ack := time.Since(start); ack >= putDelay {
		t.Errorf("PUT ack took %v, longer than the injected %v secondary delay; the ack must not wait on the secondary write", ack, putDelay)
	}
	start = time.Now()
	h.PutObject(h.Bucket, "slow-window/put-b.bin", RandomBytes(t, 200*1024+37))
	if ack := time.Since(start); ack >= putDelay {
		t.Errorf("PUT ack took %v, longer than the injected %v secondary delay; the ack must not wait on the secondary write", ack, putDelay)
	}
	start = time.Now()
	h.MultipartUpload(h.Bucket, "slow-window/mp1.dump", RandomBytes(t, minMultipartPart))
	// Same rationale as writeFaultWindow: the flow's first two legs are
	// primary-only; a dual-write regression stalls the complete leg.
	if ack := time.Since(start); ack > 2*putDelay {
		t.Errorf("CompleteMultipartUpload flow took %v, far beyond the injected %v secondary delay; the ack must not wait on the secondary write", ack, putDelay)
	}

	h.WaitUntilEnqueued(int64(len(stored)), WaitTimeout)

	// Depth rises while the worker is pinned in task 1's slow Put.
	pollFor(t, WaitTimeout, "queue depth to rise to N-1 behind the slow secondary write", func() bool {
		return h.QueueMetrics.QueueDepth.Load() >= int64(len(stored)-1)
	})

	// Lag gauge mid-drain: when task 1 completes (~putDelay in), tasks 2-3
	// are still pending with enqueue timestamps from the start of the window,
	// so armor_replication_lag_seconds must read >= 1s while task 2's slow
	// Put is still running. The window between the two completions is one
	// full putDelay wide, so this poll is deterministic.
	pollFor(t, 3*putDelay, "replication lag gauge to expose the backed-up window", func() bool {
		return h.QueueMetrics.LagSeconds.Load() >= 1
	})

	// Shrink the tail: tasks already mid-Put keep their captured delay, but
	// the remaining drain runs at normal speed.
	h.Secondary.SetPutDelay(0)

	h.WaitUntilReplicated(stored, WaitTimeout)
	h.AssertLandingSet(stored)

	// Full drain with a clean record: a slow (not failing) secondary costs
	// delay, not retries, drops, or errors.
	pollFor(t, WaitTimeout, "queue depth to return to 0 after the drain", func() bool {
		return h.QueueMetrics.QueueDepth.Load() == 0
	})
	h.AssertQueueDrained(int64(len(stored)))
	if got := h.QueueMetrics.LagSeconds.Load(); got != 0 {
		t.Errorf("LagSeconds = %d after full drain, want 0", got)
	}
}
