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

// The first fetch must not be a gate for starting read-ahead. This pins the
// handler/backend boundary that lets a large GET use the first 1 MiB request
// and later requests in the same round trip (armor-43e2062b).
func TestB2BackendStartsReadAheadBeforeFirstFetchCompletes(t *testing.T) {
	const blockSize = 8
	object := bytes.Repeat([]byte("x"), blockSize*2)
	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var firstOnce, secondOnce sync.Once

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := testRange(t, r)
		switch start {
		case 0:
			firstOnce.Do(func() { close(firstStarted) })
			select {
			case <-releaseFirst:
			case <-r.Context().Done():
				return
			}
		case blockSize:
			secondOnce.Do(func() { close(secondStarted) })
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(object)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(object[start : end+1])
	}))
	defer server.Close()

	b := &B2Backend{
		cfDomain:        strings.TrimPrefix(server.URL, "https://"),
		httpClient:      server.Client(),
		readBlockSize:   blockSize,
		readConcurrency: 2,
	}
	result := make(chan struct {
		body io.ReadCloser
		err  error
	}, 1)
	go func() {
		body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, int64(len(object)))
		result <- struct {
			body io.ReadCloser
			err  error
		}{body: body, err: err}
	}()

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first range request did not start")
	}
	select {
	case <-secondStarted:
		// Expected: read-ahead began while block 0 was still held.
	case <-time.After(time.Second):
		t.Fatal("second range request waited for the first fetch to complete")
	}
	close(releaseFirst)

	select {
	case res := <-result:
		if res.err != nil {
			t.Fatalf("GetRangeWithHeaders: %v", res.err)
		}
		got, err := io.ReadAll(res.body)
		res.body.Close()
		if err != nil || !bytes.Equal(got, object) {
			t.Fatalf("read = %d bytes, err %v; want the object", len(got), err)
		}
	case <-time.After(time.Second):
		t.Fatal("pipelined range read did not finish after releasing block 0")
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

// Slow readers must not let the reorder window turn into an unbounded byte
// reservation. The request budget above bounds HTTP calls; this test pins the
// separate global backing-buffer budget across independent clients.
func TestB2BackendReadBufferBudgetBoundsConcurrentSlowReaders(t *testing.T) {
	const (
		blockSize = 8
		clients   = 4
		blocks    = 32
	)
	object := bytes.Repeat([]byte("01234567"), blocks)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := testRange(t, r)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(object)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(object[start : end+1])
	}))
	defer server.Close()

	budgetBytes := int64(5 * (blockSize + 1))
	budget := newReadBufferBudget(budgetBytes)
	b := &B2Backend{
		cfDomain:         strings.TrimPrefix(server.URL, "https://"),
		httpClient:       server.Client(),
		readBlockSize:    blockSize,
		readConcurrency:  2,
		readBufferBudget: budget,
	}

	bodies := make(chan io.ReadCloser, clients)
	errs := make(chan error, clients)
	for i := 0; i < clients; i++ {
		go func() {
			body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, int64(len(object)))
			if err != nil {
				errs <- err
				return
			}
			bodies <- body
		}()
	}

	var readers []io.ReadCloser
	for i := 0; i < clients; i++ {
		select {
		case err := <-errs:
			t.Fatalf("GetRangeWithHeaders: %v", err)
		case body := <-bodies:
			readers = append(readers, body)
		case <-time.After(3 * time.Second):
			t.Fatal("slow readers did not all acquire their first buffers")
		}
	}
	// Leave all clients unread long enough for every pipeline to fill as far as
	// the global budget permits.
	time.Sleep(100 * time.Millisecond)
	if peak := budget.peakBytes(); peak > budgetBytes {
		t.Fatalf("peak live fetch-buffer bytes = %d, want <= %d", peak, budgetBytes)
	}
	if used := budget.inUseBytes(); used > budgetBytes {
		t.Fatalf("live fetch-buffer bytes = %d, want <= %d", used, budgetBytes)
	}

	var wg sync.WaitGroup
	for _, body := range readers {
		wg.Add(1)
		go func(body io.ReadCloser) {
			defer wg.Done()
			if _, err := io.Copy(io.Discard, body); err != nil {
				t.Errorf("slow-reader drain: %v", err)
			}
			_ = body.Close()
		}(body)
	}
	wg.Wait()
	if used := budget.inUseBytes(); used != 0 {
		t.Fatalf("live fetch-buffer bytes after Close = %d, want 0", used)
	}
}

func TestB2BackendReadBufferPoolReusesFixedBuffer(t *testing.T) {
	b := &B2Backend{
		readBlockSize:    8,
		readBufferBudget: newReadBufferBudget(64),
	}
	first, err := b.acquireReadBuffer(context.Background(), 8)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	capacity := cap(first.buf)
	first.release()
	second, err := b.acquireReadBuffer(context.Background(), 8)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	defer second.release()
	// sync.Pool is an opportunistic cache and may discard an entry at any
	// time, so pointer identity is not a valid contract to assert here. The
	// deterministic contract is that every fetch receives the fixed-size
	// buffer shape that the pool stores, and that releasing it returns the
	// budget reservation even when the runtime drops the pooled entry.
	if got := cap(second.buf); got != capacity {
		t.Fatalf("fetch buffer capacity = %d, want %d", got, capacity)
	}
	if got := b.readBufferBudget.inUseBytes(); got != int64(capacity) {
		t.Fatalf("live buffer budget = %d, want %d", got, capacity)
	}
	second.release()
	if got := b.readBufferBudget.inUseBytes(); got != 0 {
		t.Fatalf("live buffer budget after release = %d, want 0", got)
	}
}
