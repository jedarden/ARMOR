package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestRecycler(maxRequests int, maxAge time.Duration) *ConnRecycler {
	cr := NewConnRecycler(maxRequests, maxAge)
	cr.now = func() time.Time { return time.Unix(0, 0) }
	cr.lastSweep = time.Unix(0, 0)
	return cr
}

// requestThrough sends one request with the given remote address through the
// recycler and returns the Connection response header value ("" when unset).
func requestThrough(t *testing.T, h http.Handler, remoteAddr string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	h.ServeHTTP(rec, req)
	return rec.Header().Get("Connection")
}

// stubHandler is a comparable http.Handler so tests can check handler
// identity (func-typed handlers cannot be compared).
type stubHandler struct {
	tag int
}

func (s stubHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestConnRecyclerDisabledReturnsHandlerUnchanged(t *testing.T) {
	cr := newTestRecycler(0, 0)
	next := stubHandler{tag: 1}
	if got := cr.Wrap(next); got != next {
		t.Fatal("Wrap returned a new handler with both thresholds disabled; want the handler passed through unchanged")
	}
}

func TestConnRecyclerDisabledNeverSetsCloseHeader(t *testing.T) {
	cr := newTestRecycler(0, 0)
	h := cr.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for i := 1; i <= 5; i++ {
		if got := requestThrough(t, h, "10.0.0.1:1000"); got != "" {
			t.Fatalf("request %d with recycling disabled: Connection header = %q, want unset", i, got)
		}
	}
}

func TestConnRecyclerClosesAfterRequestThreshold(t *testing.T) {
	cr := newTestRecycler(2, 0)
	h := cr.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	// The first maxRequests requests are served cleanly...
	for i := 1; i <= 2; i++ {
		if got := requestThrough(t, h, "10.0.0.1:1000"); got == "close" {
			t.Fatalf("request %d of 2: Connection header = close, want the connection kept alive", i)
		}
	}
	// ...then the next request is told to close...
	if got := requestThrough(t, h, "10.0.0.1:1000"); got != "close" {
		t.Fatalf("request 3 of 2: Connection header = %q, want close", got)
	}
	// ...and stays told even if the client ignores the hint.
	if got := requestThrough(t, h, "10.0.0.1:1000"); got != "close" {
		t.Fatalf("request 4 of 2: Connection header = %q, want close", got)
	}
}

func TestConnRecyclerCountsConnectionsIndependently(t *testing.T) {
	cr := newTestRecycler(2, 0)
	h := cr.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	// Interleave two connections; each gets its own count.
	got := []string{
		requestThrough(t, h, "10.0.0.1:1000"),
		requestThrough(t, h, "10.0.0.2:2000"),
		requestThrough(t, h, "10.0.0.1:1000"),
		requestThrough(t, h, "10.0.0.1:1000"),
		requestThrough(t, h, "10.0.0.2:2000"),
		requestThrough(t, h, "10.0.0.2:2000"),
	}
	want := []string{"", "", "", "close", "", "close"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("request %d: Connection header = %q, want %q", i+1, got[i], want[i])
		}
	}
}

func TestConnRecyclerClosesAfterMaxAge(t *testing.T) {
	cr := newTestRecycler(0, 5*time.Minute)
	current := time.Unix(0, 0)
	cr.now = func() time.Time { return current }
	h := cr.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	if got := requestThrough(t, h, "10.0.0.1:1000"); got == "close" {
		t.Fatal("request on a fresh connection: Connection header = close, want keep-alive")
	}

	// Just inside the age limit the connection is still kept alive.
	current = time.Unix(0, 0).Add(4*time.Minute + 59*time.Second)
	if got := requestThrough(t, h, "10.0.0.1:1000"); got != "" {
		t.Fatalf("request at 4m59s of a 5m limit: Connection header = %q, want keep-alive", got)
	}

	// At the limit and beyond, the next request is told to close.
	current = time.Unix(0, 0).Add(5 * time.Minute)
	if got := requestThrough(t, h, "10.0.0.1:1000"); got != "close" {
		t.Fatalf("request at 5m00s of a 5m limit: Connection header = %q, want close", got)
	}
}

func TestConnRecyclerEitherThresholdTriggers(t *testing.T) {
	// A request count of 1000 with a 1m age: the age must trigger long
	// before the count does.
	cr := newTestRecycler(1000, time.Minute)
	current := time.Unix(0, 0)
	cr.now = func() time.Time { return current }
	h := cr.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	requestThrough(t, h, "10.0.0.1:1000")
	current = time.Unix(0, 0).Add(2 * time.Minute)
	if got := requestThrough(t, h, "10.0.0.1:1000"); got != "close" {
		t.Fatalf("Connection header = %q, want close from the age threshold with the count threshold far away", got)
	}

	// And the reverse: an age of 1h with a count of 1 must trigger on
	// the second request, while the connection is seconds old.
	cr2 := newTestRecycler(1, time.Hour)
	h2 := cr2.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	requestThrough(t, h2, "10.0.0.2:2000")
	if got := requestThrough(t, h2, "10.0.0.2:2000"); got != "close" {
		t.Fatalf("Connection header = %q, want close from the count threshold with the age threshold far away", got)
	}
}

// TestConnRecyclerCompletesInFlightResponses pins the boundary guarantee at
// the middleware level: the request whose response carries the close hint
// still gets its full, normally-framed response.
func TestConnRecyclerCompletesInFlightResponses(t *testing.T) {
	cr := newTestRecycler(1, 0)
	h := cr.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("0123456789abcdef")); err != nil {
			t.Errorf("write body: %v", err)
		}
	}))

	send := func(remoteAddr string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remoteAddr
		h.ServeHTTP(rec, req)
		return rec
	}

	first := send("10.0.0.1:1000")
	if got := first.Header().Get("Connection"); got == "close" {
		t.Fatal("first response told to close before the threshold of 1")
	}

	// Second request on the same connection: its response carries the
	// close hint and is otherwise completely normal.
	second := send("10.0.0.1:1000")
	if got := second.Body.String(); got != "0123456789abcdef" {
		t.Fatalf("body = %q, want the full 16 bytes", got)
	}
	if got := second.Header().Get("Connection"); got != "close" {
		t.Fatalf("Connection header = %q, want close on the response that trips the threshold", got)
	}
	if second.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", second.Code)
	}
	if ct := second.Header().Get("Content-Type"); ct != "application/octet-stream" {
		t.Fatalf("Content-Type = %q, want the inner handler's value preserved alongside the close hint", ct)
	}
}

func TestConnRecyclerSweepsStaleEntries(t *testing.T) {
	cr := newTestRecycler(1, 0)
	current := time.Unix(0, 0)
	cr.now = func() time.Time { return current }
	h := cr.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	// A connection that trips the threshold then goes quiet...
	if got := requestThrough(t, h, "10.0.0.1:1000"); got != "" {
		t.Fatalf("first request: Connection header = %q, want keep-alive", got)
	}
	if got := requestThrough(t, h, "10.0.0.1:1000"); got != "close" {
		t.Fatalf("second request: Connection header = %q, want close", got)
	}

	// ...has its entry dropped once it is stale...
	current = time.Unix(0, 0).Add(connStatsStaleAfter + time.Minute)
	requestThrough(t, h, "10.0.0.2:2000") // triggers the sweep via the interval
	cr.mu.Lock()
	_, alive := cr.conns["10.0.0.1:1000"]
	entries := len(cr.conns)
	cr.mu.Unlock()
	if alive {
		t.Fatal("stale entry survived the sweep")
	}
	if entries != 1 {
		t.Fatalf("map holds %d entries after the sweep, want 1", entries)
	}

	// ...and a reconnect that reuses the address starts from a clean count.
	if got := requestThrough(t, h, "10.0.0.1:1000"); got == "close" {
		t.Fatal("reconnected connection told to close on its first request, want a fresh count")
	}
}
