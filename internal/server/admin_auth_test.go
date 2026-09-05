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

// newAdminAuthServerWithDashboard is newAdminAuthServer with dashboard
// credentials configured, for the tests that pin /dashboard's exemption from
// the token gate.
func newAdminAuthServerWithDashboard(t *testing.T, token, user, pass, dashToken string, logBuf *bytes.Buffer) *Server {
	t.Helper()
	s := newAdminAuthServer(t, token, logBuf)
	s.config.DashboardUser = user
	s.config.DashboardPass = pass
	s.config.DashboardToken = dashToken
	return s
}

// stubOK is the protected handler the middleware wraps in tests.
func stubOK() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

// TestAdminPublicPathsBypassToken confirms the probe and scrape paths are
// reachable without a bearer token (kubelet/Prometheus). The dashboard paths
// are covered separately, because their exemption is conditional — see
// TestDashboardBypassesTokenGateOnlyWhenConfigured.
func TestAdminPublicPathsBypassToken(t *testing.T) {
	s := newAdminAuthServer(t, "sekrit", nil)
	h := s.adminAuthMiddleware(stubOK())

	public := []string{
		"/healthz",
		"/readyz",
		"/armor/canary",
		"/metrics",
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

// TestDashboardBypassesTokenGateOnlyWhenConfigured pins the condition behind
// the /dashboard exemption. The mount is exempt from the admin bearer-token
// gate because it authenticates requests itself — so the exemption must hold
// only while a dashboard credential is actually configured. With
// ARMOR_DASHBOARD_USER/PASS/TOKEN all unset, "no auth configured" would
// otherwise mean "no auth at all" (bead armor-cfd49e41, confirmed live on
// iad-ci), so those requests fall through to the admin gate and are audited
// like any other privileged call.
func TestDashboardBypassesTokenGateOnlyWhenConfigured(t *testing.T) {
	const token = "sekrit"

	t.Run("dashboard auth configured is token-free", func(t *testing.T) {
		for _, name := range []string{"basic", "token"} {
			var s *Server
			switch name {
			case "basic":
				s = newAdminAuthServerWithDashboard(t, token, "admin", "pw", "", nil)
			case "token":
				s = newAdminAuthServerWithDashboard(t, token, "", "", "dash-tok", nil)
			}
			h := s.adminAuthMiddleware(stubOK())

			for _, path := range []string{"/dashboard", "/dashboard/object", "/dashboard/api/list?prefix=x", "/dashboard/admin/key/status"} {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Errorf("[%s] %s: expected 200 with no bearer token, got %d", name, path, rec.Code)
				}
			}
		}
	})

	t.Run("dashboard auth unset falls through to the token gate", func(t *testing.T) {
		s := newAdminAuthServer(t, token, nil) // no ARMOR_DASHBOARD_*
		h := s.adminAuthMiddleware(stubOK())

		for _, path := range []string{"/dashboard", "/dashboard/object", "/dashboard/api/list", "/dashboard/admin/key/status", "/dashboard/admin/key/rotate"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s without dashboard auth and without bearer: expected 401, got %d", path, rec.Code)
			}

			req = httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec = httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("%s without dashboard auth, with valid bearer: expected 200, got %d", path, rec.Code)
			}
		}
	})

	t.Run("dashboard and admin auth both unset fails closed", func(t *testing.T) {
		s := newAdminAuthServer(t, "", nil)
		h := s.adminAuthMiddleware(stubOK())

		req := httptest.NewRequest(http.MethodGet, "/dashboard/admin/key/status", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("nothing configured: expected fail-closed 403, got %d", rec.Code)
		}
	})
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

// TestAdminTokenWithProvisionedNewlineStillAuthenticates is the regression
// test for the iad-kalshi incident: the provisioned admin token arrived as 45
// bytes (`openssl rand -base64 32 | bao kv put ... token=-` keeps the newline
// openssl prints), an HTTP Authorization header cannot carry a raw \n, and the
// constant-time compare therefore never matched — every /admin/* request
// returned 401 with no hint the stored value was malformed. The gate must
// tolerate the newline on the configured side and match the newline-free
// bearer value a client can actually send.
func TestAdminTokenWithProvisionedNewlineStillAuthenticates(t *testing.T) {
	const token = "sekrit"

	cases := []struct {
		name   string
		config string // what the pod was actually configured with
		bearer string // what a client can put in an Authorization header
		want   int
	}{
		{"trailing newline matches bare bearer", token + "\n", token, http.StatusOK},
		{"CRLF matches bare bearer", token + "\r\n", token, http.StatusOK},
		{"surrounding spaces match bare bearer", " " + token + " ", token, http.StatusOK},
		{"clean value still works", token, token, http.StatusOK},
		{"trimmed value does not match other bearer", token + "\n", token + "x", http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newAdminAuthServer(t, c.config, nil)
			h := s.adminAuthMiddleware(stubOK())

			req := httptest.NewRequest(http.MethodGet, "/admin/creds", nil)
			req.Header.Set("Authorization", "Bearer "+c.bearer)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != c.want {
				t.Errorf("config %q + bearer %q: expected %d, got %d", c.config, c.bearer, c.want, rec.Code)
			}
		})
	}
}

// TestAdminWhitespaceOnlyTokenFailsClosed confirms a token provisioned as
// nothing but whitespace is treated as "not set" (403, admin API disabled) —
// the truthful diagnosis — rather than 401, which reads as a client mistake.
func TestAdminWhitespaceOnlyTokenFailsClosed(t *testing.T) {
	s := newAdminAuthServer(t, "  \n\t", nil)
	h := s.adminAuthMiddleware(stubOK())

	req := httptest.NewRequest(http.MethodGet, "/admin/creds", nil)
	req.Header.Set("Authorization", "Bearer sekrit")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("whitespace-only token: expected fail-closed 403, got %d", rec.Code)
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

// TestIsAdminPathPublic is a table test for the path classifier used by the
// gate. The dashboard rows are split on whether a dashboard credential is
// configured, which is what decides the /dashboard exemption.
func TestIsAdminPathPublic(t *testing.T) {
	cases := []struct {
		name           string
		dashConfigured bool
		path           string
		public         bool
	}{
		// Probe/scrape paths are public regardless of dashboard configuration.
		{name: "healthz", dashConfigured: false, path: "/healthz", public: true},
		{name: "readyz", dashConfigured: false, path: "/readyz", public: true},
		{name: "canary", dashConfigured: true, path: "/armor/canary", public: true},
		{name: "metrics", dashConfigured: true, path: "/metrics", public: true},

		// Dashboard paths: public only while the dashboard can authenticate.
		{name: "dashboard root, unconfigured", dashConfigured: false, path: "/dashboard", public: false},
		{name: "dashboard subpath, unconfigured", dashConfigured: false, path: "/dashboard/object", public: false},
		{name: "dashboard api, unconfigured", dashConfigured: false, path: "/dashboard/api/list", public: false},
		{name: "dashboard rotation status, unconfigured", dashConfigured: false, path: "/dashboard/admin/key/status", public: false},
		{name: "dashboard root, configured", dashConfigured: true, path: "/dashboard", public: true},
		{name: "dashboard subpath, configured", dashConfigured: true, path: "/dashboard/object", public: true},
		{name: "dashboard api, configured", dashConfigured: true, path: "/dashboard/api/list", public: true},

		// Always gated.
		{name: "key verify", dashConfigured: true, path: "/admin/key/verify", public: false},
		{name: "key rotate", dashConfigured: false, path: "/admin/key/rotate", public: false},
		{name: "key export", dashConfigured: false, path: "/admin/key/export", public: false},
		{name: "audit", dashConfigured: false, path: "/armor/audit", public: false},
		{name: "presign", dashConfigured: false, path: "/admin/presign", public: false},
		{name: "b2 keys", dashConfigured: false, path: "/admin/b2/keys", public: false},
		{name: "b2 key delete", dashConfigured: false, path: "/admin/b2/keys/id", public: false},
		{name: "admin root", dashConfigured: true, path: "/admin", public: false},
		{name: "root", dashConfigured: true, path: "/", public: false},
		// Not the dashboard mount: a sibling prefix must not inherit the exemption.
		{name: "dashboard-like sibling", dashConfigured: true, path: "/dashboardx", public: false},
	}
	for _, c := range cases {
		s := newAdminAuthServer(t, "sekrit", nil)
		if c.dashConfigured {
			s = newAdminAuthServerWithDashboard(t, "sekrit", "admin", "pw", "", nil)
		}
		if got := s.isAdminPathPublic(c.path); got != c.public {
			t.Errorf("isAdminPathPublic(%q) [dashboard configured=%v] = %v, want %v", c.path, c.dashConfigured, got, c.public)
		}
	}
}

// TestDashboardAuthConfigured pins the predicate itself, including the
// degenerate configs. "Configured" means "the dashboard refuses anonymous
// requests": a user with no password can never accept a request, so it counts
// as configured (the mount stays fail-closed), while a password with no user
// leaves the dashboard's allow-all branch armed and therefore does not.
func TestDashboardAuthConfigured(t *testing.T) {
	cases := []struct {
		name              string
		user, pass, token string
		want              bool
	}{
		{"nothing set", "", "", "", false},
		{"basic only", "admin", "pw", "", true},
		{"token only", "", "", "tok", true},
		{"both", "admin", "pw", "tok", true},
		{"user without password rejects everything", "admin", "", "", true},
		{"password without user", "", "pw", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newAdminAuthServer(t, "sekrit", nil)
			s.config.DashboardUser = c.user
			s.config.DashboardPass = c.pass
			s.config.DashboardToken = c.token
			if got := s.dashboardAuthConfigured(); got != c.want {
				t.Errorf("dashboardAuthConfigured(user=%q, pass=%q, token=%q) = %v, want %v", c.user, c.pass, c.token, got, c.want)
			}
		})
	}
}
