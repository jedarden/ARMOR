package main

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func getenvFromMap(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestNewS3HTTPServerAllowsLongRunningRequestsAndResponses(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := newS3HTTPServer("127.0.0.1:0", handler)

	if server.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v, want 0 so long multipart completion responses remain connected", server.WriteTimeout)
	}
	if server.ReadTimeout != 0 {
		t.Fatalf("ReadTimeout = %v, want 0 so long multipart request lifecycles remain connected", server.ReadTimeout)
	}
	if server.ReadHeaderTimeout != s3ReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %v, want %v", server.ReadHeaderTimeout, s3ReadHeaderTimeout)
	}
	if server.IdleTimeout != s3IdleTimeout {
		t.Fatalf("IdleTimeout = %v, want %v", server.IdleTimeout, s3IdleTimeout)
	}
	if server.Handler == nil {
		t.Fatal("Handler is nil")
	}
}

func TestNewAdminHTTPServerDefaultsToUnboundedTimeouts(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server, err := newAdminHTTPServer("127.0.0.1:0", handler, getenvFromMap(nil))
	if err != nil {
		t.Fatalf("newAdminHTTPServer: %v", err)
	}

	if server.ReadTimeout != 0 {
		t.Fatalf("ReadTimeout = %v, want 0 so a rotation walking the whole bucket is not cut off", server.ReadTimeout)
	}
	if server.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v, want 0 because net/http arms the write deadline before the handler runs, so any non-zero value caps the handler's runtime", server.WriteTimeout)
	}
	if server.ReadHeaderTimeout != adminReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %v, want %v as the slowloris guardrail that replaces ReadTimeout", server.ReadHeaderTimeout, adminReadHeaderTimeout)
	}
	if server.IdleTimeout != adminIdleTimeout {
		t.Fatalf("IdleTimeout = %v, want %v; http.Server falls back to ReadTimeout for a zero IdleTimeout, which is also 0 now", server.IdleTimeout, adminIdleTimeout)
	}
	if server.Handler == nil {
		t.Fatal("Handler is nil")
	}
}

func TestAdminTimeoutsReadEnvOverrides(t *testing.T) {
	testCases := []struct {
		name      string
		env       map[string]string
		wantRead  time.Duration
		wantWrite time.Duration
	}{
		{name: "unset means disabled", env: nil, wantRead: 0, wantWrite: 0},
		{name: "empty means disabled", env: map[string]string{EnvAdminReadTimeout: "", EnvAdminWriteTimeout: ""}, wantRead: 0, wantWrite: 0},
		{name: "explicit zero means disabled", env: map[string]string{EnvAdminReadTimeout: "0", EnvAdminWriteTimeout: "0s"}, wantRead: 0, wantWrite: 0},
		{
			name:      "both overridden",
			env:       map[string]string{EnvAdminReadTimeout: "45s", EnvAdminWriteTimeout: "5m"},
			wantRead:  45 * time.Second,
			wantWrite: 5 * time.Minute,
		},
		{
			name:      "read only, write stays disabled",
			env:       map[string]string{EnvAdminReadTimeout: "1h30m"},
			wantRead:  90 * time.Minute,
			wantWrite: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotRead, gotWrite, err := adminTimeouts(getenvFromMap(tc.env))
			if err != nil {
				t.Fatalf("adminTimeouts: %v", err)
			}
			if gotRead != tc.wantRead {
				t.Fatalf("ReadTimeout = %v, want %v", gotRead, tc.wantRead)
			}
			if gotWrite != tc.wantWrite {
				t.Fatalf("WriteTimeout = %v, want %v", gotWrite, tc.wantWrite)
			}
		})
	}
}

func TestAdminTimeoutsRejectsBadValues(t *testing.T) {
	testCases := []struct {
		name   string
		env    map[string]string
		wantIn string
	}{
		{name: "not a duration", env: map[string]string{EnvAdminReadTimeout: "soon"}, wantIn: EnvAdminReadTimeout},
		{name: "bare number is not a duration", env: map[string]string{EnvAdminWriteTimeout: "30"}, wantIn: EnvAdminWriteTimeout},
		{name: "negative", env: map[string]string{EnvAdminWriteTimeout: "-5s"}, wantIn: EnvAdminWriteTimeout},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := adminTimeouts(getenvFromMap(tc.env))
			if err == nil {
				t.Fatalf("adminTimeouts(%v) = nil error, want a failure naming the offending variable", tc.env)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("error %q does not name %q", err, tc.wantIn)
			}
		})
	}
}

func TestNewAdminHTTPServerPropagatesBadEnv(t *testing.T) {
	server, err := newAdminHTTPServer("127.0.0.1:0", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		getenvFromMap(map[string]string{EnvAdminWriteTimeout: "nope"}))
	if err == nil {
		t.Fatal("newAdminHTTPServer returned nil error for an unparsable override, want failure before the listener starts")
	}
	if server != nil {
		t.Fatalf("server = %v, want nil so a misconfigured deployment fails at startup instead of listening", server)
	}
}

// startAdminServer runs srv on an ephemeral loopback listener and returns its
// base URL.
func startAdminServer(t *testing.T, srv *http.Server) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	return "http://" + ln.Addr().String()
}

// TestNewAdminHTTPServerAllowsSlowHandlerByDefault is the behavioural half of
// the regression: under the old hard-coded 30s WriteTimeout any handler that
// outlived the cap was killed with an empty body, because the deadline is
// armed before the handler runs. The default now leaves it unset.
func TestNewAdminHTTPServerAllowsSlowHandlerByDefault(t *testing.T) {
	const body = `{"active_fingerprint":"3d3e10bb0ba11bcb"}`

	srv, err := newAdminHTTPServer("127.0.0.1:0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}), getenvFromMap(nil))
	if err != nil {
		t.Fatalf("newAdminHTTPServer: %v", err)
	}

	resp, err := http.Get(startAdminServer(t, srv) + "/admin/key/ring")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(got) != body {
		t.Fatalf("body = %q, want %q -- the slow handler must get its whole response through", got, body)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestNewAdminHTTPServerEnforcesConfiguredWriteTimeout is the converse check:
// an operator who sets ARMOR_ADMIN_WRITE_TIMEOUT gets a deadline that actually
// binds, which is what proves the plumbing reaches the live connection rather
// than just the struct field.
func TestNewAdminHTTPServerEnforcesConfiguredWriteTimeout(t *testing.T) {
	const body = `{"ok":true}`

	srv, err := newAdminHTTPServer("127.0.0.1:0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Outlives the configured deadline by 6x.
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(body))
	}), getenvFromMap(map[string]string{EnvAdminWriteTimeout: "50ms"}))
	if err != nil {
		t.Fatalf("newAdminHTTPServer: %v", err)
	}

	resp, err := http.Get(startAdminServer(t, srv) + "/admin/key/rotate")
	if err == nil {
		defer resp.Body.Close()
		got, readErr := io.ReadAll(resp.Body)
		if readErr == nil && string(got) == body {
			t.Fatalf("body = %q, want the request to fail or truncate: the configured write deadline did not bind", got)
		}
		t.Logf("write deadline truncated the response as configured (body %q, read error %v)", got, readErr)
		return
	}
	t.Logf("write deadline killed the connection as configured: %v", err)
}
