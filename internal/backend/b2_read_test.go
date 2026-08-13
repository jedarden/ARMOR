package backend

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestB2BackendGetRangePipelinesAndReassemblesInOrder(t *testing.T) {
	const blockSize = 4
	object := []byte("abcdefghijklmnop")

	var (
		mu          sync.Mutex
		ranges      []string
		workerStart = make(chan struct{}, 2)
		release     = make(chan struct{})
	)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := testRange(t, r)

		mu.Lock()
		ranges = append(ranges, r.Header.Get("Range"))
		mu.Unlock()
		w.Header().Set("CF-Cache-Status", "HIT")

		// Hold block 1 so block 2 completes first. This also makes the
		// bounded concurrency observable without making the first request
		// asynchronous (the first block supplies the response headers).
		if start == blockSize {
			workerStart <- struct{}{}
			<-release
		} else if start == 2*blockSize {
			workerStart <- struct{}{}
		}

		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(object[start : end+1])
	}))
	defer server.Close()

	b := &B2Backend{
		cfDomain:        strings.TrimPrefix(server.URL, "https://"),
		httpClient:      server.Client(),
		readBlockSize:   blockSize,
		readConcurrency: 3,
	}

	body, headers, err := b.GetRangeWithHeaders(context.Background(), "bucket", "path/to/object", 0, int64(len(object)))
	if err != nil {
		t.Fatalf("GetRangeWithHeaders: %v", err)
	}
	defer body.Close()
	if headers["CF-Cache-Status"] != "HIT" {
		t.Fatalf("headers = %#v, want CF-Cache-Status HIT", headers)
	}

	select {
	case <-workerStart:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first pipelined worker")
	}
	select {
	case <-workerStart:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second pipelined worker")
	}
	close(release)

	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read pipelined range: %v", err)
	}
	if !bytes.Equal(got, object) {
		t.Fatalf("read bytes = %q, want %q", got, object)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(ranges) != len(object)/blockSize {
		t.Fatalf("got %d ranged requests, want %d (%v)", len(ranges), len(object)/blockSize, ranges)
	}
	wantRanges := []string{"bytes=0-3", "bytes=4-7", "bytes=8-11", "bytes=12-15"}
	seen := make(map[string]int, len(ranges))
	for _, gotRange := range ranges {
		seen[gotRange]++
	}
	for _, want := range wantRanges {
		if seen[want] != 1 {
			t.Errorf("range %q count = %d, want 1 (all ranges: %v)", want, seen[want], ranges)
		}
	}
}

func TestB2BackendGetRangeFailsOnPipelinedBlockError(t *testing.T) {
	const blockSize = 4
	object := []byte("abcdefghijkl")

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := testRange(t, r)
		if start == blockSize {
			http.Error(w, "backend failure", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(object[start : end+1])
	}))
	defer server.Close()

	b := &B2Backend{
		cfDomain:        strings.TrimPrefix(server.URL, "https://"),
		httpClient:      server.Client(),
		readBlockSize:   blockSize,
		readConcurrency: 3,
	}

	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "object", 0, int64(len(object)))
	if err != nil {
		t.Fatalf("GetRangeWithHeaders: %v", err)
	}
	defer body.Close()

	got, err := io.ReadAll(body)
	if err == nil {
		t.Fatalf("read succeeded with %d bytes after a block failure; want an error", len(got))
	}
	if !strings.Contains(err.Error(), "cloudflare returned status 502") {
		t.Fatalf("read error = %v, want the ranged request failure", err)
	}
}

func TestB2BackendGetRangeFailsOnShortBlock(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, "short")
	}))
	defer server.Close()

	b := &B2Backend{
		cfDomain:        strings.TrimPrefix(server.URL, "https://"),
		httpClient:      server.Client(),
		readBlockSize:   8,
		readConcurrency: 2,
	}

	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "object", 0, 8)
	if err == nil {
		body.Close()
		t.Fatal("GetRangeWithHeaders succeeded for a short ranged response")
	}
	if !strings.Contains(err.Error(), "received 5 bytes, want 8") {
		t.Fatalf("error = %v, want short-response error", err)
	}
}

func testRange(t *testing.T, r *http.Request) (start, end int) {
	t.Helper()
	value := strings.TrimPrefix(r.Header.Get("Range"), "bytes=")
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		t.Fatalf("invalid Range header %q", r.Header.Get("Range"))
	}
	var err error
	start, err = strconv.Atoi(parts[0])
	if err != nil {
		t.Fatalf("invalid range start %q: %v", parts[0], err)
	}
	end, err = strconv.Atoi(parts[1])
	if err != nil {
		t.Fatalf("invalid range end %q: %v", parts[1], err)
	}
	if start < 0 || end < start {
		t.Fatalf("invalid range %q", value)
	}
	return start, end
}
