package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A 1 MiB span at the default fetch size is one data request, not 16
// (armor-73564f97).
func TestB2BackendGetRangeDefaultFetchSizeIsOneRequestPerMiB(t *testing.T) {
	object := bytes.Repeat([]byte("0123456789abcdef"), 1<<20/16)
	var requests atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		start, end := testRange(t, r)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(object)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(object[start : end+1])
	}))
	defer server.Close()

	b := &B2Backend{
		cfDomain:        strings.TrimPrefix(server.URL, "https://"),
		httpClient:      server.Client(),
		readBlockSize:   defaultReadBlockSize,
		readConcurrency: defaultReadConcurrency,
	}
	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, int64(len(object)))
	if err != nil {
		t.Fatalf("GetRangeWithHeaders: %v", err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil || !bytes.Equal(got, object) {
		t.Fatalf("read = %d bytes, err %v; want the object", len(got), err)
	}
	if n := requests.Load(); n != 1 {
		t.Fatalf("1 MiB took %d data requests, want 1", n)
	}
}

// With block 1 held forever and everything later fast, the number of requests
// the pipeline starts is bounded by the window, not by object length.
func TestB2BackendGetRangeStalledBlockKeepsWindowBounded(t *testing.T) {
	const (
		blockSize   = 4
		concurrency = 3
		blocks      = 500
	)
	object := bytes.Repeat([]byte("x"), blockSize*blocks)
	var (
		mu      sync.Mutex
		started = map[int]bool{}
		hold    = make(chan struct{})
	)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := testRange(t, r)
		mu.Lock()
		started[start/blockSize] = true
		mu.Unlock()
		if start == blockSize {
			select {
			case <-hold:
			case <-r.Context().Done():
				return
			}
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(object[start : end+1])
	}))
	defer server.Close()

	b := &B2Backend{
		cfDomain:        strings.TrimPrefix(server.URL, "https://"),
		httpClient:      server.Client(),
		readBlockSize:   blockSize,
		readConcurrency: concurrency,
	}
	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, int64(len(object)))
	if err != nil {
		t.Fatalf("GetRangeWithHeaders: %v", err)
	}
	time.Sleep(500 * time.Millisecond) // let the pipeline run as far as it is allowed to

	mu.Lock()
	n := len(started)
	mu.Unlock()
	if limit := concurrency*2 + 1; n > limit {
		t.Fatalf("pipeline started %d blocks behind a stalled block, want <= %d (object has %d)", n, limit, blocks)
	}
	close(hold)
	got, err := io.ReadAll(body)
	body.Close()
	if err != nil || !bytes.Equal(got, object) {
		t.Fatalf("after release read %d bytes, err %v", len(got), err)
	}
}

func TestB2BackendGetRangeRejectsWrongContentRange(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := testRange(t, r)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/100", start+1, end+1))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(bytes.Repeat([]byte("x"), int(end-start+1)))
	}))
	defer server.Close()

	b := &B2Backend{cfDomain: strings.TrimPrefix(server.URL, "https://"), httpClient: server.Client(), readBlockSize: 8, readConcurrency: 2}
	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, 8)
	if err == nil {
		body.Close()
		t.Fatal("wrong Content-Range was accepted")
	}
	if !strings.Contains(err.Error(), "Content-Range") {
		t.Fatalf("error = %v, want a Content-Range mismatch", err)
	}
}

// A wedged request fails the read at the per-request deadline instead of
// holding it open indefinitely.
func TestB2BackendGetRangeRequestTimeoutFailsStalledRead(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	b := &B2Backend{
		cfDomain:      strings.TrimPrefix(server.URL, "https://"),
		httpClient:    server.Client(),
		readBlockSize: 8, readConcurrency: 2,
		readTimeout: 150 * time.Millisecond,
	}
	done := make(chan error, 1)
	go func() {
		body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, 8)
		if err == nil {
			body.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("stalled request succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stalled request was not bounded by the per-request timeout")
	}
}

// The backend-wide budget caps simultaneous requests across independent readers.
func TestB2BackendReadBudgetCapsSimultaneousRequests(t *testing.T) {
	var cur, peak atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := cur.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		cur.Add(-1)
		start, end := testRange(t, r)
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(bytes.Repeat([]byte("x"), int(end-start+1)))
	}))
	defer server.Close()

	b := &B2Backend{
		cfDomain:      strings.TrimPrefix(server.URL, "https://"),
		httpClient:    server.Client(),
		readBlockSize: 4, readConcurrency: 8,
		readBudget: make(chan struct{}, 3),
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, 64)
			if err != nil {
				t.Errorf("read: %v", err)
				return
			}
			defer body.Close()
			_, _ = io.Copy(io.Discard, body)
		}()
	}
	wg.Wait()
	if p := peak.Load(); p > 3 {
		t.Fatalf("peak simultaneous requests = %d, want <= 3", p)
	}
}
