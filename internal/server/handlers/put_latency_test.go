package handlers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/provenance"
	"github.com/jedarden/armor/internal/server/handlers"
	"github.com/jedarden/armor/internal/server/middleware"
)

// putLatencyHarness builds a Handlers wired with a real provenance manager
// (optionally) over the shared mock backend.
func putLatencyHarness(t *testing.T, withProvenance bool) (*handlers.Handlers, *mockBackend) {
	t.Helper()
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)
	if withProvenance {
		h.WithProvenance(provenance.NewManager(mb, "test-bucket", "test-writer"))
	}
	return h, mb
}

// TestPutObjectLatencySplitPublished is the armor-69dd394b acceptance test:
// a PUT on the provenance-recording path publishes the backend PUT /
// provenance lock-wait / provenance write split to the request context that
// the request-completed log reads.
func TestPutObjectLatencySplitPublished(t *testing.T) {
	h, _ := putLatencyHarness(t, true)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test-key", bytes.NewReader([]byte("latency split payload")))
	w := httptest.NewRecorder()

	h.PutObject(w, req, "test-bucket", "test-key")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	pl := middleware.GetPutLatency(req.Context())
	if pl == nil {
		t.Fatal("expected PUT latency split in the request context after a provenance-recorded PUT")
	}
	if pl.BackendPutMs < 0 || pl.ProvenanceLockWaitMs < 0 || pl.ProvenanceWriteMs < 0 {
		t.Errorf("latency split fields must be non-negative, got %+v", pl)
	}
}

// TestPutObjectLatencySplitAbsentWhenShouldRecordFalse is the other half of
// the armor-69dd394b acceptance criteria: a PUT whose key the provenance
// manager skips publishes no split, so the log fields stay absent. PutObject
// is called directly because the .armor/ namespace guard in HandleRoot would
// 403 the request before the handler's ShouldRecord branch is reached.
func TestPutObjectLatencySplitAbsentWhenShouldRecordFalse(t *testing.T) {
	h, _ := putLatencyHarness(t, true)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/.armor/put-latency-probe", bytes.NewReader([]byte("internal object")))
	w := httptest.NewRecorder()

	h.PutObject(w, req, "test-bucket", ".armor/put-latency-probe")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if pl := middleware.GetPutLatency(req.Context()); pl != nil {
		t.Errorf("expected no PUT latency split for a ShouldRecord=false key, got %+v", pl)
	}
}

// TestPutObjectLatencySplitAbsentWithoutProvenance: with no provenance
// recorder wired there is nothing to attribute, so no split is published.
func TestPutObjectLatencySplitAbsentWithoutProvenance(t *testing.T) {
	h, _ := putLatencyHarness(t, false)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test-key", bytes.NewReader([]byte("no provenance payload")))
	w := httptest.NewRecorder()

	h.PutObject(w, req, "test-bucket", "test-key")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if pl := middleware.GetPutLatency(req.Context()); pl != nil {
		t.Errorf("expected no PUT latency split without a provenance recorder, got %+v", pl)
	}
}

// TestPutObjectStreamingLatencySplitPublished covers the streaming
// instrumentation site: an unknown-length body routes to putObjectStreaming,
// which must publish the same split.
func TestPutObjectStreamingLatencySplitPublished(t *testing.T) {
	h, _ := putLatencyHarness(t, true)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/streaming-key", bytes.NewReader([]byte("streaming latency split payload")))
	req.ContentLength = -1 // unknown size → streaming path
	w := httptest.NewRecorder()

	h.PutObject(w, req, "test-bucket", "streaming-key")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	pl := middleware.GetPutLatency(req.Context())
	if pl == nil {
		t.Fatal("expected PUT latency split in the request context after a streaming PUT")
	}
	if pl.BackendPutMs < 0 || pl.ProvenanceLockWaitMs < 0 || pl.ProvenanceWriteMs < 0 {
		t.Errorf("latency split fields must be non-negative, got %+v", pl)
	}
}
