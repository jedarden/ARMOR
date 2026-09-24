package server

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/canary"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/metrics"
)

// canary_endpoint_contract_test.go pins the HTTP behavior of GET /armor/canary
// for the multipart family (ADR-002): the endpoint serves the documented
// not-configured shape, rejects non-GET with 405, and — the contract the
// ADR exists for — reports a multipart failure independently of the
// small-object status, so a multipart-only regression is visible while the
// small-object canary stays green. Field-set pinning lives in
// internal/canary/contract_test.go; the human-readable contract is
// docs/observability-contract.md.

// multipartFailBackend forces the multipart leg of every canary check to fail
// at CreateMultipartUpload while leaving the small-object path (Put /
// GetRangeWithHeaders) fully working.
type multipartFailBackend struct {
	*countingBackend
}

func (b *multipartFailBackend) CreateMultipartUpload(_ context.Context, _, _ string, _ map[string]string) (string, error) {
	return "", fmt.Errorf("forced multipart failure: CreateMultipartUpload rejected")
}

// TestCanaryEndpointNotConfiguredShape pins the body a disabled monitor
// serves: HTTP 200 with the documented literal, so a missing canary is
// "unknown" rather than a scrape error.
func TestCanaryEndpointNotConfiguredShape(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/armor/canary", nil)
	rec := httptest.NewRecorder()
	s.canaryHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("not-configured GET returned %d, want 200", rec.Code)
	}
	if got, want := rec.Body.String(), `{"status":"unknown","error":"canary monitor not configured"}`; got != want {
		t.Errorf("not-configured body = %q, want %q", got, want)
	}
}

// TestCanaryEndpointRejectsNonGET pins the 405 contract for methods other
// than GET — the endpoint is a read-only health signal.
func TestCanaryEndpointRejectsNonGET(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := cryptoRand.Read(mek); err != nil {
		t.Fatalf("read MEK: %v", err)
	}
	m := canary.NewMonitor(canary.Config{
		Backend:    newCountingBackend(),
		Bucket:     "test-bucket",
		MEK:        mek,
		BlockSize:  65536,
		CanarySize: 512,
	})
	s := &Server{canary: m}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/armor/canary", nil)
		rec := httptest.NewRecorder()
		s.canaryHandler(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /armor/canary returned %d, want 405", method, rec.Code)
		}
	}
}

// TestCanaryEndpointMultipartFailureIndependence drives a multipart-only
// failure through a live monitor and pins what /armor/canary then reports:
// multipart unhealthy with the error and failure streak populated, while the
// small-object status stays healthy — ADR-002's independence contract, at
// the endpoint a human or alert responder actually reads.
func TestCanaryEndpointMultipartFailureIndependence(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := cryptoRand.Read(mek); err != nil {
		t.Fatalf("read MEK: %v", err)
	}
	mb := &multipartFailBackend{newCountingBackend()}
	m := canary.NewMonitor(canary.Config{
		Backend:    mb,
		Bucket:     "test-bucket",
		MEK:        mek,
		BlockSize:  65536,
		InstanceID: "multipart-fail",
		CanarySize: 512,
		// Size is irrelevant — the forced failure fires at
		// CreateMultipartUpload, before any bytes move.
		MultipartSize: 4096,
		// Start runs one small check and one multipart check immediately;
		// hourly tickers keep the test to exactly that one pair.
		Interval:          time.Hour,
		MultipartInterval: time.Hour,
		MaxRetries:        1, // one attempt, no retry sleep
		RetryDelay:        time.Millisecond,
	})

	s := &Server{
		config:  &config.Config{Bucket: "test-bucket"},
		backend: mb,
		canary:  m,
		logger:  logging.New("test"),
		metrics: metrics.DefaultMetrics,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m.Start(ctx)
	defer m.Stop()

	// The startup goroutine runs the small check first, then the multipart
	// check — once the multipart state has flipped, the small-object result
	// is already settled and the assertions below race nothing.
	deadline := time.Now().Add(5 * time.Second)
	for m.GetStatus().MultipartHealthy != canary.StatusUnhealthy {
		if time.Now().After(deadline) {
			t.Fatalf("multipart status never became unhealthy; got %v", m.GetStatus().MultipartHealthy)
		}
		time.Sleep(10 * time.Millisecond)
	}

	req := httptest.NewRequest(http.MethodGet, "/armor/canary", nil)
	rec := httptest.NewRecorder()
	s.canaryHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /armor/canary returned %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var status map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// The independence contract: multipart visibly broken, small object green.
	if got := status["multipart_healthy_status"]; got != string(canary.StatusUnhealthy) {
		t.Errorf("multipart_healthy_status = %v, want %q", got, canary.StatusUnhealthy)
	}
	if got, ok := status["multipart_healthy"].(bool); !ok || got {
		t.Errorf("multipart_healthy = %v (bool %t), want false", status["multipart_healthy"], ok)
	}
	if got := status["status"]; got != string(canary.StatusHealthy) {
		t.Errorf("status = %v, want %q — a multipart failure must not mask the small-object canary", got, canary.StatusHealthy)
	}

	errStr, ok := status["multipart_last_error"].(string)
	if !ok || errStr == "" {
		t.Fatalf("multipart_last_error missing or empty: %v", status["multipart_last_error"])
	}
	if !strings.Contains(errStr, "forced multipart failure") {
		t.Errorf("multipart_last_error = %q, want it to carry the backend failure", errStr)
	}

	fails, ok := status["multipart_consecutive_fails"].(float64)
	if !ok || fails < 1 {
		t.Errorf("multipart_consecutive_fails = %v, want >= 1", status["multipart_consecutive_fails"])
	}

	if got := status["multipart_last_check"]; got == "" {
		t.Error("multipart_last_check missing; the endpoint must carry the last attempt time")
	}
}
