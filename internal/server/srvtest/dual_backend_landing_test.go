package srvtest

// Server-level e2e verification of ADR-006 dual-backend replication: an
// object the primary backend acknowledged must LAND on the secondary once the
// replication queue drains. Unlike internal/replication/queue_test.go (queue
// mechanics over mock backends) and internal/server/handlers/
// replication_enqueue_test.go (enqueue counts over a mock Enqueuer), these
// tests write through the real authenticated S3 mux of the real Server and
// enumerate both real backends afterwards.

import (
	"errors"
	"testing"
	"time"
)

// minMultipartPart is the smallest part size the S3 API guarantees to accept
// for a non-first part; used here so the single-part upload also matches what
// real clients send.
const minMultipartPart = 5*1024*1024 + 256*1024

// TestSinglePutLandsOnSecondary writes two objects through the real PUT path
// (one plaintext-y text object, one incompressible object crossing several
// 64KiB encryption blocks) and asserts both land on the secondary
// byte-identical, with the secondary holding exactly that set.
func TestSinglePutLandsOnSecondary(t *testing.T) {
	h := New(t)

	bodies := map[string][]byte{
		"notes/hello.txt": []byte("hello dual-backend replication"),
		"data/blob.bin":   RandomBytes(t, 200*1024+37),
	}
	for key, body := range bodies {
		h.PutObject(h.Bucket, key, body)
	}

	h.WaitUntilEnqueued(int64(len(bodies)), WaitTimeout)
	expected := []string{h.StoredKey("notes/hello.txt"), h.StoredKey("data/blob.bin")}
	h.WaitUntilReplicated(expected, WaitTimeout)

	h.AssertLandingSet(expected)
	h.AssertQueueDrained(int64(len(bodies)))
}

// TestMultipartUploadLandsOnSecondary completes a full multipart upload
// (CreateMultipartUpload, UploadPart, CompleteMultipartUpload) through the
// real paths and asserts the assembled object lands on the secondary
// byte-identical. The multipart layout also writes internal state on the
// primary — the `.armor/hmac/...` HMAC table sidecar and `.armor/multipart/...`
// bookkeeping — which must stay primary-only, plus the ADR-016
// `.armor-manifest` sidecar that handlers do not enqueue today (see
// AssertLandingSet).
func TestMultipartUploadLandsOnSecondary(t *testing.T) {
	h := New(t)

	part := RandomBytes(t, minMultipartPart)
	h.MultipartUpload(h.Bucket, "backups/db.dump", part)

	storedKey := h.StoredKey("backups/db.dump")
	h.WaitUntilEnqueued(1, WaitTimeout)
	h.WaitUntilReplicated([]string{storedKey}, WaitTimeout)

	// Layout sanity on the primary: completion must have produced the
	// assembled ciphertext and the ADR-016 manifest sidecar, and the multipart
	// path is expected to have written at least one internal `.armor/` object
	// (the HMAC table sidecar survives DeleteState cleanup).
	primary := h.Snapshot(h.Primary)
	if _, ok := primary[storedKey]; !ok {
		t.Fatalf("primary layout changed: assembled object %q missing", storedKey)
	}
	manifestKey := storedKey + ".armor-manifest"
	if _, ok := primary[manifestKey]; !ok {
		t.Fatalf("primary layout changed: ADR-016 manifest sidecar %q missing", manifestKey)
	}
	internalCount := 0
	for k := range primary {
		if IsInternalKey(k) {
			internalCount++
			t.Logf("primary internal state: %s", k)
		}
	}
	if internalCount == 0 {
		t.Logf("no internal .armor/ keys found on primary after completion; layout may have changed")
	}

	h.AssertLandingSet([]string{storedKey})
	h.AssertQueueDrained(1)
}

// TestSecondaryOutageLandsAfterRecovery injects a transient secondary failure
// and asserts the primary ack was never blocked, the object did NOT land while
// the fault was active, and the queue's retry loop lands it once the secondary
// recovers.
func TestSecondaryOutageLandsAfterRecovery(t *testing.T) {
	h := New(t)

	h.Secondary.SetPutError(errors.New("injected secondary outage: service unavailable"))

	key := "outage/recovered.bin"
	body := RandomBytes(t, 64*1024+11)

	start := time.Now()
	h.PutObject(h.Bucket, key, body)
	ackDuration := time.Since(start)
	if ackDuration > time.Second {
		t.Errorf("primary ack took %v despite a failing secondary; replication must not block the ack", ackDuration)
	}

	storedKey := h.StoredKey(key)
	h.WaitUntilEnqueued(1, WaitTimeout)

	// Give the queue a few failed attempts (backoff starts at 100ms), then
	// prove the fault actually swallowed the writes before clearing it.
	time.Sleep(350 * time.Millisecond)
	if h.Secondary.PutCalls() == 0 {
		t.Fatal("replication queue never attempted the secondary write")
	}
	if _, ok := h.Snapshot(h.Secondary)[storedKey]; ok {
		t.Fatal("object landed on the secondary while fault injection was active")
	}
	if retries := h.QueueMetrics.RetriesTotal.Load(); retries == 0 {
		t.Fatal("queue reported no retries despite a failing secondary")
	}

	h.Secondary.SetPutError(nil)
	h.WaitUntilReplicated([]string{storedKey}, WaitTimeout)

	h.AssertLandingSet([]string{storedKey})
	if got := h.QueueMetrics.DroppedTotal.Load(); got != 0 {
		t.Errorf("DroppedTotal = %d, want 0 (transient failures must be retried, not dropped)", got)
	}
	if got := h.QueueMetrics.ErrorsTotal.Load(); got != 0 {
		t.Errorf("ErrorsTotal = %d, want 0 (recovered transient failures are not errors)", got)
	}
}

// TestSlowSecondaryDoesNotBlockPrimaryAck injects a slow secondary write and
// asserts the client ack returns long before the replication write completes,
// while the object still lands afterwards.
func TestSlowSecondaryDoesNotBlockPrimaryAck(t *testing.T) {
	h := New(t)

	const putDelay = 250 * time.Millisecond
	h.Secondary.SetPutDelay(putDelay)

	key := "slow/landed.bin"
	body := RandomBytes(t, 32*1024+5)

	start := time.Now()
	h.PutObject(h.Bucket, key, body)
	ackDuration := time.Since(start)
	if ackDuration >= putDelay {
		t.Errorf("primary ack took %v, longer than the injected %v secondary delay; the ack must not wait on the secondary write", ackDuration, putDelay)
	}

	storedKey := h.StoredKey(key)
	h.WaitUntilEnqueued(1, WaitTimeout)
	h.WaitUntilReplicated([]string{storedKey}, WaitTimeout)

	h.AssertLandingSet([]string{storedKey})
	h.AssertQueueDrained(1)
}
