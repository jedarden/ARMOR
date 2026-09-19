package backend

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// These tests pin the b2 fetchRange behavior when the origin answers a ranged
// request with 200 and the whole object instead of 206 with the requested
// range. B2 does this occasionally under concurrency and always for a
// malformed Range header (observed live on iad-ci, armor-817d9d92). For a
// nonzero offset the full body is the wrong bytes, so fetchRange retries once
// and then fails with an explicit error instead of surfacing a confusing
// length mismatch.

func newRangeIgnoreBackend(server *httptest.Server) *B2Backend {
	return &B2Backend{
		cfDomain:        strings.TrimPrefix(server.URL, "https://"),
		httpClient:      server.Client(),
		readBlockSize:   64,
		readConcurrency: 4,
	}
}

func fullBodyHandler(t *testing.T, object []byte, calls *int, mu *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n := *calls
		*calls++
		mu.Unlock()

		if n == 0 {
			// Origin ignores Range: full object with a 200.
			w.Header().Set("Content-Length", strconv.Itoa(len(object)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(object)
			return
		}
		start, end := testRange(t, r)
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(object[start : end+1])
	}
}

func TestFetchRangeRetriesOnceWhenOriginIgnoresRange(t *testing.T) {
	object := make([]byte, 200)
	for i := range object {
		object[i] = byte(i)
	}

	var mu sync.Mutex
	calls := 0
	server := httptest.NewTLSServer(fullBodyHandler(t, object, &calls, &mu))
	defer server.Close()

	b := newRangeIgnoreBackend(server)
	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 100, 50)
	if err != nil {
		t.Fatalf("GetRangeWithHeaders: %v", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(data) != string(object[100:150]) {
		t.Fatalf("data = %q, want object[100:150]", data)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("requests = %d, want 2 (one ignored range, one retry)", calls)
	}
}

func TestFetchRangeFailsAfterRepeatFullBodyOnSubrange(t *testing.T) {
	object := make([]byte, 200)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always ignore the Range header.
		w.Header().Set("Content-Length", strconv.Itoa(len(object)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(object)
	}))
	defer server.Close()

	b := newRangeIgnoreBackend(server)
	_, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 100, 50)
	if err == nil {
		t.Fatal("expected error when origin serves a full body twice, got nil")
	}
	if !strings.Contains(err.Error(), "ignored Range") {
		t.Fatalf("error = %v, want an explicit ignored-Range message", err)
	}
}

func TestFetchRangeAcceptsFullBodyAtZeroOffset(t *testing.T) {
	// Object sized to one read block so the whole-object fetch is a single
	// ranged request: offset 0 with the full length is a whole-object fetch,
	// where a 200 is a legitimate answer and the bytes are correct either way.
	object := make([]byte, 64)
	for i := range object {
		object[i] = byte(255 - i)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(object)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(object)
	}))
	defer server.Close()

	b := newRangeIgnoreBackend(server)
	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, int64(len(object)))
	if err != nil {
		t.Fatalf("GetRangeWithHeaders: %v", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(data) != string(object) {
		t.Fatal("full-object fetch returned wrong bytes")
	}
}
