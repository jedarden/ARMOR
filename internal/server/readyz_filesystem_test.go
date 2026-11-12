package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/manifest"
	"github.com/jedarden/armor/internal/metrics"
)

// Filesystem-backend readiness (the demo profile). A filesystem deployment
// runs no canary and an idle one never flushes a manifest delta, so /readyz
// must not wait for a flush: "never flushed" held the demo's readiness at
// 503 for the process's whole lifetime. B2 deployments — production — keep
// the flush-recency contract unchanged.

// newReadyzBackendServer builds a Server whose readiness inputs are exactly
// one manifest state, the way the loaded demo looks after its cold load.
func newReadyzBackendServer(backendType string, state manifestState) *Server {
	return &Server{
		config:      &config.Config{Bucket: "test-bucket", Backend: backendType},
		manifestSys: resolvedManifestSystem(state),
		logger:      logging.New("test"),
		metrics:     metrics.DefaultMetrics,
	}
}

func TestReadyzFilesystemBackendReadyWithoutFlush(t *testing.T) {
	cb := newCountingBackend()
	idx := manifest.New()
	uploader := func(ctx context.Context, key string, data []byte) error {
		return cb.Put(ctx, "test-bucket", key, bytes.NewReader(data), int64(len(data)), nil)
	}
	writer := manifest.NewWriter(idx, "test-prefix", "test-writer", uploader, 0)
	writer.Start(context.Background())
	defer writer.Stop()
	// No enqueues: the writer exists but never flushes — an idle demo.

	s := newReadyzBackendServer("filesystem", manifestState{status: manifestLoaded, idx: idx, writer: writer})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	s.readyz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("filesystem backend with never-flushed writer = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var ready struct {
		Ready  bool   `json:"ready"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ready); err != nil {
		t.Fatalf("decode readyz response: %v", err)
	}
	if !ready.Ready || !bytes.Contains([]byte(ready.Reason), []byte("filesystem backend")) {
		t.Errorf("unexpected readyz response for filesystem backend: %+v", ready)
	}
}

func TestReadyzFilesystemBackendReadyWithoutWriter(t *testing.T) {
	// Manifest disabled (ARMOR_MANIFEST_ENABLED=false shape): no writer at
	// all, and still a perfectly servable filesystem deployment.
	s := newReadyzBackendServer("filesystem", manifestState{status: manifestDisabled})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	s.readyz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("filesystem backend without manifest writer = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestReadyzFilesystemBackendWaitsOutColdLoad(t *testing.T) {
	// The filesystem carve-out sits after the cold-load branch: a deployment
	// still replaying its startup load reports not ready, exactly as before.
	s := newReadyzBackendServer("filesystem", manifestState{status: manifestLoading, idx: manifest.New()})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	s.readyz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("filesystem backend during cold load = %d, want 503: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("manifest cold load in progress")) {
		t.Errorf("expected cold-load reason, got: %s", rec.Body.String())
	}
}

func TestReadyzB2BackendStillRequiresFlush(t *testing.T) {
	// Production semantics unchanged: a B2 deployment whose writer has
	// never flushed reports 503 with the same reason as before.
	cb := newCountingBackend()
	idx := manifest.New()
	uploader := func(ctx context.Context, key string, data []byte) error {
		return cb.Put(ctx, "test-bucket", key, bytes.NewReader(data), int64(len(data)), nil)
	}
	writer := manifest.NewWriter(idx, "test-prefix", "test-writer", uploader, 0)
	writer.Start(context.Background())
	defer writer.Stop()

	s := newReadyzBackendServer("b2", manifestState{status: manifestLoaded, idx: idx, writer: writer})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	s.readyz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("b2 backend with never-flushed writer = %d, want 503: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("never flushed")) {
		t.Errorf("expected 'never flushed' message for b2 backend, got: %s", rec.Body.String())
	}
}
