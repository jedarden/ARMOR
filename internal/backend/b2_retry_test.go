package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/metrics"
)

// newRetryTestBackend returns a backend over a fake Cloudflare origin with a
// fast retry policy and an isolated metrics registry.
func newRetryTestBackend(server *httptest.Server, m *metrics.Metrics, attempts int, backoff time.Duration) *B2Backend {
	return &B2Backend{
		cfDomain:        strings.TrimPrefix(server.URL, "https://"),
		httpClient:      server.Client(),
		readBlockSize:   4,
		readConcurrency: 3,
		readTimeout:     5 * time.Second,

		readRetryAttempts:   attempts,
		readRetryBackoff:    backoff,
		readRetryMaxBackoff: 50 * time.Millisecond,
		metrics:             m,
	}
}

// One transient Cloudflare 522 on block 2 of 4 is retried exactly once and the
// read succeeds with the full object (armor-f1791426).
func TestB2BackendGetRangeRetriesOneInjected522(t *testing.T) {
	const blockSize = 4
	object := []byte("abcdefghijklmnop")

	var mu sync.Mutex
	requests := map[string]int{}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := testRange(t, r)
		mu.Lock()
		requests[r.Header.Get("Range")]++
		seen := requests[r.Header.Get("Range")]
		mu.Unlock()
		if start == 2*blockSize && seen == 1 {
			w.WriteHeader(522)
			return
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(object[start : end+1])
	}))
	defer server.Close()

	m := metrics.NewMetrics()
	b := newRetryTestBackend(server, m, 3, time.Millisecond)

	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "object", 0, int64(len(object)))
	if err != nil {
		t.Fatalf("GetRangeWithHeaders: %v", err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil || !bytes.Equal(got, object) {
		t.Fatalf("read = %q, err %v; want the object", got, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if requests["bytes=8-11"] != 2 {
		t.Fatalf("block 2 saw %d requests, want exactly one retry (2)", requests["bytes=8-11"])
	}
	for _, rng := range []string{"bytes=0-3", "bytes=4-7", "bytes=12-15"} {
		if requests[rng] != 1 {
			t.Errorf("%s saw %d requests, want 1 (no retries elsewhere)", rng, requests[rng])
		}
	}
	if dump := m.PrometheusFormat(); !strings.Contains(dump, `armor_backend_retries_total{reason="http_5xx",outcome="success"} 1`) {
		t.Fatalf("metrics dump missing the http_5xx/success retry:\n%s", dump)
	}
}

// A persistent transient failure fails after the attempt bound with the
// original error class, and the metric records the exhaustion.
func TestB2BackendGetRangePersistentFailureFailsAfterBound(t *testing.T) {
	var count atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(522)
	}))
	defer server.Close()

	m := metrics.NewMetrics()
	b := newRetryTestBackend(server, m, 3, time.Millisecond)

	_, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "object", 0, 8)
	if err == nil || !strings.Contains(err.Error(), "cloudflare returned status 522") {
		t.Fatalf("err = %v, want the original 522 class", err)
	}
	if n := count.Load(); n != 3 {
		t.Fatalf("persistent failure took %d requests, want the 3-attempt bound", n)
	}
	if dump := m.PrometheusFormat(); !strings.Contains(dump, `armor_backend_retries_total{reason="http_5xx",outcome="exhausted"} 1`) {
		t.Fatalf("metrics dump missing the http_5xx/exhausted outcome:\n%s", dump)
	}
}

// Cancelling the caller's context during a backoff sleep returns promptly with
// the failure that triggered the retry.
func TestB2BackendGetRangeCancellationDuringBackoffReturnsPromptly(t *testing.T) {
	first := make(chan struct{})
	var once sync.Once
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(first) })
		w.WriteHeader(522)
	}))
	defer server.Close()

	m := metrics.NewMetrics()
	// The backoff is far longer than the test's patience: only cancellation
	// can end the fetch in time.
	b := newRetryTestBackend(server, m, 3, 30*time.Second)
	b.readRetryMaxBackoff = 30 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() {
		_, _, err := b.GetRangeWithHeaders(ctx, "bucket", "object", 0, 4)
		errc <- err
	}()

	select {
	case <-first:
	case <-time.After(2 * time.Second):
		t.Fatal("no request reached the fake origin")
	}
	// Cancel 100ms after the origin saw the first request: the failure has
	// long since been classified and the loop is inside the 30s backoff.
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancelled := time.Now()
	cancel()
	select {
	case err := <-errc:
		if err == nil || !strings.Contains(err.Error(), "522") {
			t.Fatalf("err = %v, want the original 522 failure", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation during backoff did not return promptly")
	}
	if time.Since(cancelled) > time.Second {
		t.Fatalf("return took %v after cancel; backoff sleep outlived the caller", time.Since(cancelled))
	}
	if dump := m.PrometheusFormat(); !strings.Contains(dump, `armor_backend_retries_total{reason="http_5xx",outcome="cancelled"} 1`) {
		t.Fatalf("metrics dump missing the http_5xx/cancelled outcome:\n%s", dump)
	}
}

// A delivered block is never re-fetched: after the failing last block surfaces
// its error through the reader, no further request is made for it.
func TestB2BackendGetRangeDoesNotRetryAfterPartialDelivery(t *testing.T) {
	const blockSize = 4
	object := []byte("abcdefghijklmnop")

	var count atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := testRange(t, r)
		if start == 3*blockSize {
			count.Add(1)
			w.WriteHeader(522)
			return
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(object[start : end+1])
	}))
	defer server.Close()

	m := metrics.NewMetrics()
	b := newRetryTestBackend(server, m, 3, time.Millisecond)

	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "object", 0, int64(len(object)))
	if err != nil {
		t.Fatalf("GetRangeWithHeaders: %v", err)
	}
	defer body.Close()

	prefix := make([]byte, blockSize)
	if _, err := io.ReadFull(body, prefix); err != nil || !bytes.Equal(prefix, object[:blockSize]) {
		t.Fatalf("delivered prefix = %q, err %v; want %q", prefix, err, object[:blockSize])
	}
	if _, err := io.ReadAll(body); err == nil || !strings.Contains(err.Error(), "522") {
		t.Fatalf("read error = %v, want the last block's 522 failure", err)
	}
	// Any post-delivery retry would surface as extra requests to the fake.
	time.Sleep(100 * time.Millisecond)
	if n := count.Load(); n != 3 {
		t.Fatalf("last block fetched %d times, want exactly the 3-attempt bound and nothing after delivery", n)
	}
}

func TestRetryClass(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"canceled", context.Canceled, ""},
		{"cf 522", &httpStatusError{status: 522}, retryReasonHTTP5xx},
		{"cf 520", &httpStatusError{status: 520}, retryReasonHTTP5xx},
		{"429", &httpStatusError{status: 429}, retryReasonHTTP429},
		{"404 not retried", &httpStatusError{status: 404}, ""},
		{"wrapped status", fmt.Errorf("fetch range block 2: %w", &httpStatusError{status: 502}), retryReasonHTTP5xx},
		{"short read", &shortReadError{got: 5, want: 8}, retryReasonTruncatedBody},
		{"unexpected eof", fmt.Errorf("read 8 bytes: %w", io.ErrUnexpectedEOF), retryReasonTruncatedBody},
		{"conn reset", fmt.Errorf("Get %q: %w", "http://cf/x", syscall.ECONNRESET), retryReasonConnectionReset},
		{"plain error", errors.New("cloudflare ignored Range header"), ""},
	}
	for _, tc := range cases {
		if got := retryClass(tc.err); got != tc.want {
			t.Errorf("%s: retryClass = %q, want %q", tc.name, got, tc.want)
		}
		if tc.err != nil && IsRetryableError(tc.err) != (tc.want != "") {
			t.Errorf("%s: IsRetryableError disagrees with retryClass", tc.name)
		}
	}
}

func TestDoWithRetryPolicy(t *testing.T) {
	ctx := context.Background()

	// Success after one transient failure.
	attempts := 0
	outcome, err := DoWithRetry(ctx, RetryPolicy{Attempts: 3, Backoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}, func() error {
		attempts++
		if attempts == 1 {
			return &httpStatusError{status: 522}
		}
		return nil
	})
	if err != nil || attempts != 2 || outcome.Retries != 1 || outcome.Reason != retryReasonHTTP5xx || outcome.Outcome != retryOutcomeSuccess {
		t.Fatalf("success case: attempts=%d outcome=%+v err=%v", attempts, outcome, err)
	}

	// A non-retryable failure returns immediately with no retry recorded.
	calls := 0
	outcome, err = DoWithRetry(ctx, RetryPolicy{Attempts: 5, Backoff: time.Millisecond}, func() error {
		calls++
		return &httpStatusError{status: 404}
	})
	if err == nil || calls != 1 || outcome.Retries != 0 || outcome.Outcome != "" {
		t.Fatalf("non-retryable case: calls=%d outcome=%+v err=%v", calls, outcome, err)
	}

	// A retryable failure whose context died before the retry decision ends
	// the sequence immediately, returns the ORIGINAL error (not the context
	// error), and never reaches the backoff.
	cctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	outcome, err = DoWithRetry(cctx, RetryPolicy{Attempts: 3, Backoff: time.Minute, MaxBackoff: time.Minute}, func() error {
		cancel()
		return errTestOriginal503
	})
	// No retry decision was taken, so the outcome stays empty — the sequence
	// neither succeeded nor exhausted its bound; it never retried at all.
	if elapsed := time.Since(start); !errors.Is(err, errTestOriginal503) || outcome.Outcome != "" || outcome.Retries != 0 || elapsed > time.Second {
		t.Fatalf("dead-context case: outcome=%+v err=%v elapsed=%v", outcome, err, elapsed)
	}

	// SleepBackoff returns the context error immediately when the context is
	// already done, and sleeps zero when the base is zero.
	sctx, scancel := context.WithCancel(context.Background())
	scancel()
	if err := SleepBackoff(sctx, 1, time.Minute, time.Minute); err == nil {
		t.Fatal("SleepBackoff on a dead context returned nil")
	}
	if err := SleepBackoff(context.Background(), 1, 0, time.Minute); err != nil {
		t.Fatalf("SleepBackoff with zero base returned %v", err)
	}
}

// errTestOriginal503 marks the injected failure so the dead-context case can
// prove the original error (not the context error) is returned.
var errTestOriginal503 = errors.New("injected 503")

// Head retries transient failures too: the fake origin 503s persistently, the
// SDK's own retryer exhausts its bound first, and the ARMOR-level retry runs
// on top of it before surfacing the failure.
func TestB2BackendHeadRetriesTransientFailure(t *testing.T) {
	var count atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	m := metrics.NewMetrics()
	b := newDirectS3TestBackend(t, server.URL, defaultReadBlockSize, defaultReadConcurrency)
	b.readRetryAttempts = 3
	b.readRetryBackoff = time.Millisecond
	b.readRetryMaxBackoff = 5 * time.Millisecond
	b.metrics = m

	_, err := b.Head(context.Background(), "bucket", "key")
	if err == nil || !strings.Contains(err.Error(), "HeadObject failed") {
		t.Fatalf("err = %v, want the HeadObject failure", err)
	}
	// The SDK's standard retryer alone stops at its own bound; more requests
	// than that prove the ARMOR-level retry ran too.
	if n := count.Load(); n <= 3 {
		t.Fatalf("HeadObject saw %d requests, want the ARMOR retry on top of the SDK bound", n)
	}
	if dump := m.PrometheusFormat(); !strings.Contains(dump, `armor_backend_retries_total{reason="http_5xx",outcome="exhausted"} 1`) {
		t.Fatalf("metrics dump missing the Head retry exhaustion:\n%s", dump)
	}
}
