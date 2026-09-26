package srvtest

// Server-level pin of the ADR-006 read/write routing contract, the one
// routing guarantee docs/disaster-recovery.md states in prose but nothing
// verified deterministically before this file:
//
//   - "Replication does not change the read path." While the primary is
//     healthy, every read is served from the primary and ONLY from the
//     primary. A mirror holding poisoned bytes — and an actively faulted
//     secondary write path — must be invisible to GET/HEAD/LIST, and a read
//     must enqueue nothing and trigger no secondary write.
//
//   - "Failover is never automatic." With the primary destroyed and the
//     mirror holding fully valid, decryptable ciphertext of the same object
//     under the same stored key, the live deployment must NOT fall back to
//     the secondary: the read fails (404 NoSuchKey from the primary miss).
//     Serving the mirror would silently merge the mirror's async-window
//     losses and resurrected deletes into the client's view — the runbook's
//     manual promotion exists precisely to make that an operator decision.

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strconv"
	"testing"
)

func TestReadsIgnoreTheSecondary(t *testing.T) {
	h := New(t)
	ctx := context.Background()

	// Two corpus members: `live` exercises the healthy-primary routing legs,
	// `cold` is never read until after the primary is destroyed, so its 404
	// cannot be explained by a warm metadata/footer cache.
	const (
		liveKey = "routing/live.bin"
		coldKey = "routing/cold.bin"
	)
	livePlaintext := RandomBytes(t, 192*1024+7)
	coldPlaintext := RandomBytes(t, 96*1024+3)
	h.PutObject(h.Bucket, liveKey, livePlaintext)
	h.PutObject(h.Bucket, coldKey, coldPlaintext)
	storedLive := h.StoredKey(liveKey)
	storedCold := h.StoredKey(coldKey)
	h.WaitUntilEnqueued(2, WaitTimeout)
	h.WaitUntilReplicated([]string{storedLive, storedCold}, WaitTimeout)

	// Capture the genuine replicated envelope of the cold object so the
	// mirror can be restored to a fully valid state before the
	// destroyed-primary leg — the fallback prohibition is only proven if the
	// mirror copy would actually decrypt.
	body, info, err := h.Secondary.Backend.Get(ctx, h.Bucket, storedCold)
	if err != nil {
		t.Fatalf("secondary Get %q: %v", storedCold, err)
	}
	coldEnvelope, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		t.Fatalf("secondary read %q: %v", storedCold, err)
	}
	if info == nil || len(info.Metadata) == 0 {
		t.Fatalf("replicated cold object carries no metadata; envelope capture is invalid")
	}

	// Poison the mirror: garbage bytes with NO ARMOR metadata under the live
	// stored key. If any read path ever consulted the secondary, the
	// non-ARMOR passthrough would serve these bytes and the equality assert
	// below would fail. Also fault every secondary Put: a read must never
	// trigger a replication write either.
	if err := h.Secondary.Backend.Put(ctx, h.Bucket, storedLive, bytes.NewReader(RandomBytes(t, 4096)), 4096, nil); err != nil {
		t.Fatalf("poisoning the mirror: %v", err)
	}
	h.Secondary.SetPutError(errors.New("injected: no read may touch the secondary"))
	defer h.Secondary.SetPutError(nil)

	rec := h.Do(http.MethodGet, "/"+h.Bucket+"/"+liveKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET with a poisoned, faulted secondary: status %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Equal(rec.Body.Bytes(), livePlaintext) {
		t.Fatalf("GET with a poisoned secondary returned non-primary bytes (%d bytes, want the %d-byte plaintext) — the read path consulted the mirror", rec.Body.Len(), len(livePlaintext))
	}

	// HEAD must describe the primary object, not the poisoned mirror bytes.
	hrec := h.Do(http.MethodHead, "/"+h.Bucket+"/"+liveKey, nil)
	if hrec.Code != http.StatusOK {
		t.Fatalf("HEAD with a poisoned secondary: status %d: %s", hrec.Code, hrec.Body.String())
	}
	if got := hrec.Header().Get("Content-Length"); got != strconv.Itoa(len(livePlaintext)) {
		t.Errorf("HEAD Content-Length %q with a poisoned secondary, want the primary size %d", got, len(livePlaintext))
	}

	// Listing is a primary operation too: the poisoned mirror state must not
	// surface.
	lrec := h.Do(http.MethodGet, "/"+h.Bucket+"?list-type=2", nil)
	if lrec.Code != http.StatusOK {
		t.Fatalf("LIST with a poisoned secondary: status %d: %s", lrec.Code, lrec.Body.String())
	}
	var lr struct {
		Contents []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
	}
	if err := xml.Unmarshal(lrec.Body.Bytes(), &lr); err != nil {
		t.Fatalf("LIST parse: %v", err)
	}
	found := false
	for _, c := range lr.Contents {
		if c.Key == liveKey {
			found = true
		}
	}
	if !found {
		t.Errorf("LIST with a poisoned secondary is missing %q", liveKey)
	}

	// The read path must have stayed completely off the replication queue
	// and off the secondary write path: no new tasks, no new Puts (the fault
	// would have failed them loudly if any had been attempted).
	if got := h.QueueMetrics.EnqueuedTotal.Load(); got != 2 {
		t.Errorf("EnqueuedTotal = %d after the reads, want 2 — a read enqueued a replication task", got)
	}
	putCallsBeforeGet := h.Secondary.PutCalls()
	rec = h.Do(http.MethodGet, "/"+h.Bucket+"/"+liveKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("second GET with a faulted secondary: status %d: %s", rec.Code, rec.Body.String())
	}
	if got := h.Secondary.PutCalls(); got != putCallsBeforeGet {
		t.Errorf("secondary Put calls went %d → %d across a GET — the read path wrote to (or retried against) the secondary", putCallsBeforeGet, got)
	}

	// Restore the genuine mirror state and clear the fault, then destroy the
	// primary: the deployment now faces exactly the situation automatic
	// fallback would "helpfully" survive, with a fully valid decryptable copy
	// one field away.
	h.Secondary.SetPutError(nil)
	if err := h.Secondary.Backend.Put(ctx, h.Bucket, storedCold, bytes.NewReader(coldEnvelope), int64(len(coldEnvelope)), info.Metadata); err != nil {
		t.Fatalf("restoring the genuine mirror envelope: %v", err)
	}
	destroyPrimary(t, h)

	// Failover is never automatic: the read must fail rather than serve the
	// mirror. 404 (NoSuchKey from the primary miss) is the pinned current
	// behavior; any 2xx here means the deployment silently failed over.
	rec = h.Do(http.MethodGet, "/"+h.Bucket+"/"+coldKey, nil)
	if rec.Code == http.StatusOK {
		t.Fatalf("GET with the primary destroyed returned 200 — the deployment fell back to the secondary; failover must stay a manual promotion")
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET with the primary destroyed: status %d, want 404 (no automatic failover); body: %s", rec.Code, rec.Body.String())
	}
	if bytes.Equal(rec.Body.Bytes(), coldPlaintext) {
		t.Fatalf("GET with the primary destroyed served the original plaintext from the mirror")
	}

	// And the mirror copy itself must be untouched by the failed read — the
	// promotion candidate stays intact for the runbook's manual procedure.
	mbody, _, err := h.Secondary.Backend.Get(ctx, h.Bucket, storedCold)
	if err != nil {
		t.Fatalf("mirror Get after the destroyed-primary read: %v", err)
	}
	mirror, err := io.ReadAll(mbody)
	mbody.Close()
	if err != nil {
		t.Fatalf("mirror read after the destroyed-primary read: %v", err)
	}
	if !bytes.Equal(mirror, coldEnvelope) {
		t.Errorf("mirror envelope changed across the destroyed-primary read")
	}
}
