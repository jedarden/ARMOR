package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// recycleStubHandler is a comparable http.Handler so the wiring test can
// check handler identity (func-typed handlers cannot be compared).
type recycleStubHandler struct {
	tag int
}

func (s recycleStubHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestS3HandlerWithRecycling(t *testing.T) {
	inner := recycleStubHandler{tag: 1}

	t.Run("disabled returns the handler unchanged", func(t *testing.T) {
		got, requests, age, err := s3HandlerWithRecycling(inner, getenvFromMap(nil))
		if err != nil {
			t.Fatalf("s3HandlerWithRecycling: %v", err)
		}
		if got != inner {
			t.Fatal("recycling disabled but the handler was wrapped")
		}
		if requests != 0 || age != 0 {
			t.Fatalf("thresholds = %d/%v, want 0/0 for the disabled default", requests, age)
		}
	})

	t.Run("configured wraps the handler", func(t *testing.T) {
		got, requests, age, err := s3HandlerWithRecycling(inner, getenvFromMap(map[string]string{
			EnvMaxConnRequests: "1000",
			EnvMaxConnAge:      "90s",
		}))
		if err != nil {
			t.Fatalf("s3HandlerWithRecycling: %v", err)
		}
		if got == inner {
			t.Fatal("recycling configured but the handler was not wrapped")
		}
		if requests != 1000 {
			t.Fatalf("maxRequests = %d, want 1000", requests)
		}
		if age != 90*time.Second {
			t.Fatalf("maxAge = %v, want 90s", age)
		}
	})

	t.Run("bad values fail naming the variable", func(t *testing.T) {
		_, _, _, err := s3HandlerWithRecycling(inner, getenvFromMap(map[string]string{EnvMaxConnAge: "soon"}))
		if err == nil || !strings.Contains(err.Error(), EnvMaxConnAge) {
			t.Fatalf("err = %v, want a failure naming %q", err, EnvMaxConnAge)
		}
	})
}

// countingListener counts accepted connections so a test can prove that every
// request rode one underlying TCP connection.
type countingListener struct {
	net.Listener
	accepts atomic.Int32
}

func (l *countingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil {
		l.accepts.Add(1)
	}
	return c, err
}

// serveRecycled starts an S3 http.Server built through the production
// constructor and wiring, with the given recycling environment, on a counting
// listener, and returns its address. The returned stop func shuts it down.
func serveRecycled(t *testing.T, env map[string]string, inner http.Handler) (addr string, accepts *atomic.Int32, stop func()) {
	t.Helper()

	wrapped, _, _, err := s3HandlerWithRecycling(inner, getenvFromMap(env))
	if err != nil {
		t.Fatalf("s3HandlerWithRecycling: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	counting := &countingListener{Listener: ln}
	srv := newS3HTTPServer(ln.Addr().String(), wrapped)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(counting)
	}()
	return ln.Addr().String(), &counting.accepts, func() {
		_ = srv.Close()
		<-done
	}
}

// roundTrip sends one GET over conn, reads the full response and body, and
// returns them.
func roundTrip(t *testing.T, conn net.Conn, br *bufio.Reader, path string) (*http.Response, []byte) {
	t.Helper()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: armor\r\nUser-Agent: conn-recycle-test\r\nAccept: */*\r\n\r\n", path)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatalf("read response for %s: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body for %s: %v", path, err)
	}
	_ = resp.Body.Close()
	return resp, body
}

// TestS3ConnRecyclingClosesAfterThresholdOnOneConnection is the acceptance
// scenario: sequential requests over one connection are served normally up to
// the threshold, the response after it carries `Connection: close` (and a
// slow in-flight request still completes with its full body), and the server
// then closes the connection. Exactly one connection is ever accepted,
// proving the requests really did share one keep-alive connection.
func TestS3ConnRecyclingClosesAfterThresholdOnOneConnection(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Make the threshold-tripping response slow: the close may only
		// happen once it has fully completed.
		if r.URL.Query().Get("i") == "3" {
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Fprintf(w, "body-%s", r.URL.Query().Get("i"))
	})
	addr, accepts, stop := serveRecycled(t, map[string]string{EnvMaxConnRequests: "2"}, inner)
	defer stop()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	br := bufio.NewReader(conn)

	// Requests 1 and 2: served cleanly, connection kept alive.
	for i := 1; i <= 2; i++ {
		resp, body := roundTrip(t, conn, br, fmt.Sprintf("/?i=%d", i))
		if resp.Close {
			t.Fatalf("request %d told to close before the threshold of 2", i)
		}
		if want := fmt.Sprintf("body-%d", i); !bytes.Equal(body, []byte(want)) {
			t.Fatalf("request %d body = %q, want %q", i, body, want)
		}
	}

	// Request 3: full body, plus the close hint. http.ReadResponse
	// consumes the wire's `Connection: close` header into resp.Close,
	// which is the signal a real S3 client acts on.
	resp, body := roundTrip(t, conn, br, "/?i=3")
	if !resp.Close {
		t.Fatalf("request 3: response not flagged as close-signaled (Connection header = %q), want close", resp.Header.Get("Connection"))
	}
	if !bytes.Equal(body, []byte("body-3")) {
		t.Fatalf("request 3 body = %q, want %q: the in-flight response did not complete normally", body, "body-3")
	}

	// The server must now close its end.
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	var buf [1]byte
	if _, err := br.Read(buf[:]); err != io.EOF {
		t.Fatalf("read after the close-signaled response = %v, want EOF", err)
	}

	if got := accepts.Load(); got != 1 {
		t.Fatalf("server accepted %d connections, want 1: the requests did not share one keep-alive connection", got)
	}
}

// TestS3ConnRecyclingDisabledByDefaultKeepsConnectionOpen pins the
// conservative default: with neither variable set, connections are never
// recycled.
func TestS3ConnRecyclingDisabledByDefaultKeepsConnectionOpen(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "body-%s", r.URL.Query().Get("i"))
	})
	addr, accepts, stop := serveRecycled(t, nil, inner)
	defer stop()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	br := bufio.NewReader(conn)

	for i := 1; i <= 4; i++ {
		resp, _ := roundTrip(t, conn, br, fmt.Sprintf("/?i=%d", i))
		if resp.Close {
			t.Fatalf("request %d told to close with recycling disabled", i)
		}
	}

	// Give the server a moment to close its end if it were going to; the
	// deadline firing means the connection is still open, as wanted.
	if err := conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	var buf [1]byte
	if _, err := br.Read(buf[:]); err != io.EOF {
		if !strings.Contains(err.Error(), "timeout") {
			t.Fatalf("connection closed with recycling disabled (read = %v), want it still open", err)
		}
	} else {
		t.Fatal("connection closed by the server with recycling disabled, want it still open")
	}

	if got := accepts.Load(); got != 1 {
		t.Fatalf("server accepted %d connections, want 1", got)
	}
}
