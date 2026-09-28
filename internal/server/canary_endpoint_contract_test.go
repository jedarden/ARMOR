package server

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
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

// smallFailBackend keeps the multipart path working while making the
// ordinary single-object canary fail. This makes the two health families
// independently observable at the HTTP surface.
type smallFailBackend struct {
	*countingBackend
}

func (b *smallFailBackend) Put(context.Context, string, string, io.Reader, int64, map[string]string) error {
	return fmt.Errorf("forced small-object failure: Put rejected")
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

// TestCanaryEndpointDisabledShape pins the explicit disabled state. Disabled
// readiness is intentionally still HTTP 200, but the status body must tell an
// operator that no integrity check is being performed.
func TestCanaryEndpointDisabledShape(t *testing.T) {
	m := canary.NewMonitor(canary.Config{})
	s := &Server{canary: m, canaryDisabled: true}

	req := httptest.NewRequest(http.MethodGet, "/armor/canary", nil)
	rec := httptest.NewRecorder()
	s.canaryHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("disabled GET returned %d, want 200", rec.Code)
	}
	if got, want := rec.Body.String(), `{"status":"unknown","error":"canary monitor disabled"}`; got != want {
		t.Errorf("disabled body = %q, want %q", got, want)
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
		if got := rec.Header().Get("Allow"); got != http.MethodGet {
			t.Errorf("%s /armor/canary Allow = %q, want GET", method, got)
		}
	}
}

// TestCanaryEndpointHealthyAndFailedStates pins the ordinary endpoint's
// status codes and diagnostic fields for a successful check and an exhausted
// retry budget. Both are status responses, so the endpoint remains 200; the
// JSON status is the health signal and /readyz is the readiness gate.
func TestCanaryEndpointHealthyAndFailedStates(t *testing.T) {
	testCases := []struct {
		name       string
		monitor    *canary.Monitor
		wantStatus canary.Status
		wantError  string
	}{
		{
			name: "healthy",
			monitor: canary.NewMonitor(canary.Config{
				Backend:           newCountingBackend(),
				Bucket:            "test-bucket",
				MEK:               makeTestMEK(t),
				BlockSize:         65536,
				CanarySize:        512,
				MultipartSize:     4096,
				Interval:          time.Hour,
				MultipartInterval: time.Hour,
				MaxRetries:        1,
			}),
		},
		{
			name: "failed",
			monitor: canary.NewMonitor(canary.Config{
				Backend:           &smallFailBackend{newCountingBackend()},
				Bucket:            "test-bucket",
				MEK:               makeTestMEK(t),
				BlockSize:         65536,
				CanarySize:        512,
				MultipartSize:     4096,
				Interval:          time.Hour,
				MultipartInterval: time.Hour,
				MaxRetries:        1,
			}),
			wantStatus: canary.StatusUnhealthy,
			wantError:  "forced small-object failure",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.monitor
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			m.Start(ctx)
			defer m.Stop()

			deadline := time.Now().Add(5 * time.Second)
			for {
				status := m.GetStatus()
				if (tc.wantStatus == "" && status.Status == canary.StatusHealthy) || status.Status == tc.wantStatus {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("monitor status = %q, want %q", status.Status, tc.wantStatus)
				}
				time.Sleep(10 * time.Millisecond)
			}

			s := &Server{canary: m}
			rec := httptest.NewRecorder()
			s.canaryHandler(rec, httptest.NewRequest(http.MethodGet, "/armor/canary", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET /armor/canary returned %d, want 200", rec.Code)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			wantStatus := tc.wantStatus
			if wantStatus == "" {
				wantStatus = canary.StatusHealthy
			}
			if got := body["status"]; got != string(wantStatus) {
				t.Errorf("status = %v, want %q", got, wantStatus)
			}
			if tc.wantError != "" && !strings.Contains(body["last_error"].(string), tc.wantError) {
				t.Errorf("last_error = %v, want substring %q", body["last_error"], tc.wantError)
			}
		})
	}
}

func makeTestMEK(t *testing.T) []byte {
	t.Helper()
	mek := make([]byte, 32)
	if _, err := cryptoRand.Read(mek); err != nil {
		t.Fatalf("read MEK: %v", err)
	}
	return mek
}

// TestCanaryEndpointStaleState pins that a previously healthy but overdue
// monitor returns "stale" and remains a non-ready health signal, rather than
// silently presenting its old success as current.
func TestCanaryEndpointStaleState(t *testing.T) {
	m := canary.NewMonitor(canary.Config{
		Backend:           newCountingBackend(),
		Bucket:            "test-bucket",
		MEK:               makeTestMEK(t),
		BlockSize:         65536,
		CanarySize:        512,
		MultipartSize:     4096,
		Interval:          time.Hour,
		MultipartInterval: time.Hour,
		StaleAfter:        time.Nanosecond,
		MaxRetries:        1,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m.Start(ctx)
	defer m.Stop()

	deadline := time.Now().Add(5 * time.Second)
	for m.GetStatus().Status != canary.StatusStale {
		if time.Now().After(deadline) {
			t.Fatalf("monitor status = %q, want stale", m.GetStatus().Status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	s := &Server{canary: m, canaryStarted: true}
	rec := httptest.NewRecorder()
	s.canaryHandler(rec, httptest.NewRequest(http.MethodGet, "/armor/canary", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode stale response: %v", err)
	}
	if body["status"] != string(canary.StatusStale) {
		t.Errorf("stale endpoint status = %v, want stale", body["status"])
	}

	ready := httptest.NewRecorder()
	s.readyz(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusServiceUnavailable {
		t.Errorf("stale /readyz returned %d, want 503", ready.Code)
	}
	if !strings.Contains(ready.Body.String(), "canary check stale") {
		t.Errorf("stale /readyz body = %q, want stale reason", ready.Body.String())
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
