package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
)

// newAdminAuthServer builds a minimal Server wired with the admin auth
// middleware. The stub next handler returns 200 OK so tests can assert purely
// on the gate's behaviour (allow/deny) without standing up a real backend or
// touching the MEK. The MEK is intentionally never set or read here.
func newAdminAuthServer(t *testing.T, token string, logBuf *bytes.Buffer) *Server {
	t.Helper()
	logger := logging.New("test")
	if logBuf != nil {
		logger.SetOutput(logBuf)
	}
	return &Server{
		config: &config.Config{AdminToken: token},
		logger: logger,
	}
}

// stubOK is the protected handler the middleware wraps in tests.
func stubOK() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

// TestAdminPublicPathsBypassToken confirms probes, metrics, and dashboard paths
// are reachable without a bearer token (kubelet/Prometheus/dashboard auth).
func TestAdminPublicPathsBypassToken(t *testing.T) {
	s := newAdminAuthServer(t, "sekrit", nil)
	h := s.adminAuthMiddleware(stubOK())

	public := []string{
		"/healthz",
		"/readyz",
		"/armor/canary",
		"/metrics",
		"/dashboard",
		"/dashboard/object",
		"/dashboard/api/list?prefix=x",
	}
	for _, path := range public {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("public path %s: expected 200 (no token), got %d", path, rec.Code)
		}
	}
}

// TestAdminRoutesRequireToken confirms every gated route is 401 without a token
// and 200 with the valid bearer token.
func TestAdminRoutesRequireToken(t *testing.T) {
	const token = "sekrit"
	s := newAdminAuthServer(t, token, nil)
	h := s.adminAuthMiddleware(stubOK())

	gated := []string{
		"/admin/key/verify",
		"/admin/key/rotate",
		"/admin/key/export",
		"/armor/audit",
		"/admin/presign",
		"/admin/b2/keys",
		"/admin/b2/keys/key123",
	}

	for _, path := range gated {
		// Without token -> 401.
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("gated path %s without token: expected 401, got %d", path, rec.Code)
		}

		// With valid token -> 200.
		req = httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("gated path %s with token: expected 200, got %d", path, rec.Code)
		}
	}
}

// TestAdminExportRequiresTokenEvenWithConfirm is the highest-stakes assertion:
// the MEK export route must be blocked by the token gate even when ?confirm=yes
// is present, so the MEK can never be exfiltrated by a single unauthenticated
// GET. The response must be 401 and must NOT contain a "mek" JSON field.
func TestAdminExportRequiresTokenEvenWithConfirm(t *testing.T) {
	const token = "sekrit"
	s := newAdminAuthServer(t, token, nil)
	h := s.adminAuthMiddleware(stubOK())

	req := httptest.NewRequest(http.MethodGet, "/admin/key/export?confirm=yes", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("export?confirm=yes without token: expected 401, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, `"mek"`) {
		t.Errorf("MEK leaked in denied export response: %q", body)
	}
}

// TestAdminRejectsInvalidTokens covers wrong value, prefix-match, different
// length, and malformed Authorization headers.
func TestAdminRejectsInvalidTokens(t *testing.T) {
	const token = "sekrit"
	s := newAdminAuthServer(t, token, nil)
	h := s.adminAuthMiddleware(stubOK())

	cases := []struct {
		name   string
		header string
	}{
		{"wrong value", "Bearer wrong"},
		{"prefix match only", "Bearer sekri"},
		{"longer than token", "Bearer sekrit-extra"},
		{"different scheme", "Basic sekrit"},
		{"no bearer prefix", "sekrit"},
		{"empty bearer", "Bearer "},
		{"missing header", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/key/verify", nil)
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s: expected 401, got %d", c.name, rec.Code)
			}
		})
	}
}

// TestAdminFailClosedWhenTokenUnset confirms that when ARMOR_ADMIN_TOKEN is not
// configured, gated routes return 403 (disabled) rather than open access. Public
// probe paths remain reachable so kubelet does not kill the pod.
func TestAdminFailClosedWhenTokenUnset(t *testing.T) {
	s := newAdminAuthServer(t, "", nil) // no token configured
	h := s.adminAuthMiddleware(stubOK())

	// Gated route -> 403 fail-closed.
	req := httptest.NewRequest(http.MethodGet, "/admin/key/export?confirm=yes", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("unset token: expected gated route 403, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"mek"`) {
		t.Errorf("MEK leaked in fail-closed response: %q", rec.Body.String())
	}

	// Public probe path still reachable.
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("unset token: expected /healthz 200, got %d", rec.Code)
	}
}

// TestAdminAuditLogging confirms every gated admin call is audit-logged with
// remote address, method, path, and outcome — and that the bearer token value
// and the MEK never appear in the log output.
func TestAdminAuditLogging(t *testing.T) {
	const token = "sekrit-value-not-for-logs"
	var buf bytes.Buffer
	s := newAdminAuthServer(t, token, &buf)
	h := s.adminAuthMiddleware(stubOK())

	// Allowed call.
	req := httptest.NewRequest(http.MethodPost, "/admin/key/rotate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = "10.0.0.5:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// Denied call.
	req = httptest.NewRequest(http.MethodGet, "/admin/key/export?confirm=yes", nil)
	req.RemoteAddr = "10.0.0.9:5678"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	out := buf.String()
	for _, want := range []string{"admin API access", "10.0.0.5:1234", "/admin/key/rotate", "allowed", "denied", "/admin/key/export"} {
		if !strings.Contains(out, want) {
			t.Errorf("audit log missing %q; got:\n%s", want, out)
		}
	}
	// The token value and the literal "mek" must never be logged.
	if strings.Contains(out, token) {
		t.Errorf("audit log leaked bearer token value:\n%s", out)
	}
	if strings.Contains(strings.ToLower(out), `"mek"`) {
		t.Errorf("audit log mentions MEK:\n%s", out)
	}
}

// TestIsAdminPathPublic is a table test for the path classifier used by the gate.
func TestIsAdminPathPublic(t *testing.T) {
	cases := []struct {
		path    string
		public  bool
	}{
		{"/healthz", true},
		{"/readyz", true},
		{"/armor/canary", true},
		{"/metrics", true},
		{"/dashboard", true},
		{"/dashboard/object", true},
		{"/dashboard/api/list", true},
		{"/admin/key/verify", false},
		{"/admin/key/rotate", false},
		{"/admin/key/export", false},
		{"/armor/audit", false},
		{"/admin/presign", false},
		{"/admin/b2/keys", false},
		{"/admin/b2/keys/id", false},
		{"/admin", false},
		{"/", false},
	}
	for _, c := range cases {
		if got := isAdminPathPublic(c.path); got != c.public {
			t.Errorf("isAdminPathPublic(%q) = %v, want %v", c.path, got, c.public)
		}
	}
}
