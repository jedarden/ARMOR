package srvtest

// Server-level e2e pinning of the ADR-006 overwrite and delete propagation
// semantics. ADR-006, "Dual-Backend Async Replication for Provider-Outage
// Resilience" (docs/adr/006-dual-backend-replication.md; the file numbering
// was repaired and cross-references fixed by commit 23dc39e9, which is the
// authoritative ADR-006 location), Decision #1 names PutObject and
// CompleteMultipartUpload as the enqueue sites and is silent on overwrites
// and deletes. These tests pin what the implementation actually does so it
// cannot drift into an implicit contract:
//
//   - Overwrites REPLICATE. Every acknowledged write re-enqueues the key
//     (handlers enqueue on each PutObject — buffered and streaming — and each
//     CompleteMultipartUpload), and the queue's sequential Get+Put worker
//     converges the secondary on the newest primary ciphertext. Pinned
//     byte-exact for both the single-PUT and the multipart layout.
//
//   - Deletes DO NOT replicate. DeleteObject writes only to the primary and
//     never enqueues; no delete task exists in the queue protocol. The
//     secondary retains the last replicated ciphertext after a primary
//     delete. This is asserted explicitly here and recorded as a DR caveat
//     in docs/disaster-recovery.md (operator-runbook bead armor-b2235418).

import (
	"bytes"
	"net/http"
	"testing"
	"time"
)

// waitUntilConverged polls both backends until every given stored key is
// present on the secondary and byte-identical to the primary copy. Unlike
// WaitUntilReplicated (presence only), this is the overwrite signal: the
// secondary already holds the key, and what must be waited for is the new
// ciphertext replacing the old.
func waitUntilConverged(t *testing.T, h *Harness, keys []string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		primary := h.Snapshot(h.Primary)
		secondary := h.Snapshot(h.Secondary)
		converged := true
		for _, k := range keys {
			pBytes, pok := primary[k]
			sBytes, sok := secondary[k]
			if !pok || !sok || !bytes.Equal(pBytes, sBytes) {
				converged = false
				break
			}
		}
		if converged {
			return
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("secondary did not converge with the primary for %v within %s", keys, timeout)
		}
		time.Sleep(PollInterval)
	}
}

// TestOverwriteSinglePutConvergesOnSecondary puts an object, lets it land,
// then puts a DIFFERENT body (different length and content) to the same key
// and asserts the secondary converges byte-exact on the new ciphertext once
// the queue drains — no stale bytes, no duplicate or partial objects.
func TestOverwriteSinglePutConvergesOnSecondary(t *testing.T) {
	h := New(t)

	key := "overwrite/single.bin"
	storedKey := h.StoredKey(key)

	v1 := RandomBytes(t, 200*1024+37)
	h.PutObject(h.Bucket, key, v1)
	h.WaitUntilEnqueued(1, WaitTimeout)
	h.WaitUntilReplicated([]string{storedKey}, WaitTimeout)

	primary := h.Snapshot(h.Primary)
	v1Primary, ok := primary[storedKey]
	if !ok {
		t.Fatalf("v1 never landed on the primary at %q", storedKey)
	}
	v1Secondary := h.Snapshot(h.Secondary)[storedKey]
	if v1Secondary == nil {
		t.Fatalf("v1 never landed on the secondary at %q", storedKey)
	}
	if !bytes.Equal(v1Primary, v1Secondary) {
		t.Fatal("v1 landing was not byte-identical; overwrite test is invalid")
	}

	v2 := RandomBytes(t, 320*1024+91)
	h.PutObject(h.Bucket, key, v2)
	h.WaitUntilEnqueued(2, WaitTimeout)
	waitUntilConverged(t, h, []string{storedKey}, WaitTimeout)

	// Sanity: the overwrite must actually have changed the ciphertext, or
	// every equality assertion below is vacuous.
	v2Primary := h.Snapshot(h.Primary)[storedKey]
	if bytes.Equal(v1Primary, v2Primary) {
		t.Fatal("second PUT produced identical primary ciphertext; test payload must differ")
	}

	h.AssertLandingSet([]string{storedKey})
	h.AssertQueueDrained(2)

	secondary := h.Snapshot(h.Secondary)
	if !bytes.Equal(secondary[storedKey], v2Primary) {
		t.Fatal("secondary did not converge on the v2 ciphertext after drain")
	}
	if bytes.Equal(secondary[storedKey], v1Secondary) {
		t.Fatal("secondary still serves the v1 ciphertext after the overwrite drained")
	}
}

// TestOverwriteMultipartConvergesOnSecondary completes a multipart upload,
// lets it land, then completes a SECOND multipart upload (different part
// count, sizes, and content) to the same key and asserts the secondary
// converges byte-exact on the new assembled envelope, still holding exactly
// that one object — the superseded envelope is replaced, and the ADR-016
// `.armor-manifest` sidecar of either generation stays primary-only.
func TestOverwriteMultipartConvergesOnSecondary(t *testing.T) {
	h := New(t)

	key := "overwrite/db.dump"
	storedKey := h.StoredKey(key)

	// Payload shapes respect the ADR-015 uniform-part-size contract: part 1
	// pins P, every non-final part must be exactly P, and the final part must
	// be SMALLER than P (any part larger than P invalidates the upload). The
	// two uploads still differ in part count, layout, and bytes.
	h.MultipartUpload(h.Bucket, key, RandomBytes(t, minMultipartPart), RandomBytes(t, 128*1024+9))
	h.WaitUntilEnqueued(1, WaitTimeout)
	h.WaitUntilReplicated([]string{storedKey}, WaitTimeout)

	primary := h.Snapshot(h.Primary)
	v1Primary, ok := primary[storedKey]
	if !ok {
		t.Fatalf("v1 multipart envelope never landed on the primary at %q", storedKey)
	}
	v1Secondary := h.Snapshot(h.Secondary)[storedKey]
	if v1Secondary == nil {
		t.Fatalf("v1 multipart envelope never landed on the secondary at %q", storedKey)
	}
	if !bytes.Equal(v1Primary, v1Secondary) {
		t.Fatal("v1 multipart landing was not byte-identical; overwrite test is invalid")
	}

	h.MultipartUpload(h.Bucket, key,
		RandomBytes(t, minMultipartPart),
		RandomBytes(t, minMultipartPart),
		RandomBytes(t, 64*1024+3))
	h.WaitUntilEnqueued(2, WaitTimeout)
	waitUntilConverged(t, h, []string{storedKey}, WaitTimeout)

	// Sanity: a two-part and a three-part upload of fresh random bytes cannot
	// produce the same assembled envelope.
	v2Primary := h.Snapshot(h.Primary)[storedKey]
	if bytes.Equal(v1Primary, v2Primary) {
		t.Fatal("second multipart completion produced an identical envelope; test payload must differ")
	}

	h.AssertLandingSet([]string{storedKey})
	h.AssertQueueDrained(2)

	secondary := h.Snapshot(h.Secondary)
	if !bytes.Equal(secondary[storedKey], v2Primary) {
		t.Fatal("secondary did not converge on the v2 multipart envelope after drain")
	}
}

// TestDeleteDoesNotPropagateToSecondary pins the delete half of the ADR-006
// contract: DeleteObject removes from the primary only and never enqueues, so
// the secondary RETAINS the last replicated ciphertext after a primary
// delete. A failover promotion therefore resurrects primary-deleted keys —
// documented in docs/disaster-recovery.md, not a bug to fix here. The delete
// path is layout-independent (single DeleteObject handler for both single-PUT
// and multipart objects), so one layout is asserted.
func TestDeleteDoesNotPropagateToSecondary(t *testing.T) {
	h := New(t)

	key := "delete/retained.bin"
	storedKey := h.StoredKey(key)

	body := RandomBytes(t, 128*1024+13)
	h.PutObject(h.Bucket, key, body)
	h.WaitUntilEnqueued(1, WaitTimeout)
	h.WaitUntilReplicated([]string{storedKey}, WaitTimeout)

	v1Secondary := h.Snapshot(h.Secondary)[storedKey]
	if v1Secondary == nil {
		t.Fatalf("object never landed on the secondary at %q", storedKey)
	}

	rec := h.Do(http.MethodDelete, "/"+h.Bucket+"/"+key, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DeleteObject %s/%s: status %d: %s", h.Bucket, key, rec.Code, rec.Body.String())
	}

	// The implementation-level proof of non-propagation: the delete enqueued
	// no replication task, so the queue accepted nothing beyond the original
	// PUT and holds no pending work.
	if got := h.QueueMetrics.EnqueuedTotal.Load(); got != 1 {
		t.Errorf("EnqueuedTotal = %d after delete, want 1 (a delete must not enqueue a replication task)", got)
	}

	if _, ok := h.Snapshot(h.Primary)[storedKey]; ok {
		t.Fatal("primary still holds the deleted key; delete test is invalid")
	}

	secondary := h.Snapshot(h.Secondary)
	retained, ok := secondary[storedKey]
	if !ok {
		t.Fatal("secondary dropped the object after a primary delete — deletes must not propagate")
	}
	if !bytes.Equal(retained, v1Secondary) {
		t.Fatal("secondary object changed across a primary delete; expected the retained ciphertext to be untouched")
	}

	h.AssertQueueDrained(1)
}
