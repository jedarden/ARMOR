// Package server implements the ARMOR S3-compatible HTTP server.
package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// adminPublicExact is the set of admin-mux paths that stay token-free because
// they are hit by kubelet probes or Prometheus scrapers.
var adminPublicExact = map[string]bool{
	"/healthz":      true, // liveness probe
	"/readyz":       true, // readiness probe
	"/armor/canary": true, // canary status (read-only health signal)
	"/metrics":      true, // Prometheus scrape
}

// adminPublicPrefixes are admin-mux path prefixes that stay token-free because
// they carry their own authentication. /dashboard enforces HTTP Basic or its
// own DashboardToken via HandlerWithAuth, so it must not be double-gated here —
// but only while a dashboard credential is actually configured. See
// dashboardAuthConfigured.
var adminPublicPrefixes = []string{"/dashboard"}

// dashboardAuthConfigured reports whether the dashboard refuses anonymous
// requests on its own. This is the pivot of the /dashboard exemption in
// isAdminPathPublic: the exemption exists only while the dashboard enforces
// something, and must disappear the moment it does not.
//
// The predicate is the negation of AuthMiddleware's own allow-all condition in
// internal/dashboard (user == "" && token == "") and has to stay in step with
// it — the two layers disagreeing is precisely what left the dashboard wide
// open when ARMOR_DASHBOARD_* was unset (bead armor-cfd49e41). Note that a
// user with no password counts as configured: Authenticate can never accept
// such a request, so it rejects everything, which is fail-closed either way.
func (s *Server) dashboardAuthConfigured() bool {
	if s.config == nil {
		return false
	}
	return s.config.DashboardUser != "" || s.config.DashboardToken != ""
}

// isAdminPathPublic reports whether a request bypasses the admin token gate.
// Everything else on the admin mux — notably all /admin/* routes and
// /armor/audit — requires the ARMOR_ADMIN_TOKEN bearer token.
//
// A prefix here is token-free only while the authentication it relies on is
// actually configured. With ARMOR_DASHBOARD_USER/PASS/TOKEN all unset the
// dashboard would admit anyone, so /dashboard falls through to the admin gate
// below, which itself fails closed when ARMOR_ADMIN_TOKEN is also unset. Unset
// env can therefore never mean "no auth at all".
func (s *Server) isAdminPathPublic(path string) bool {
	if adminPublicExact[path] {
		return true
	}
	for _, p := range adminPublicPrefixes {
		if path != p && !strings.HasPrefix(path, p+"/") {
			continue
		}
		if p == "/dashboard" && !s.dashboardAuthConfigured() {
			continue
		}
		return true
	}
	return false
}

// adminTokenValid performs a constant-time comparison of the request's bearer
// token against the expected admin token. Returns false if either side is empty
// or the header is missing/malformed.
//
// The expected token is trimmed before comparing. Load() already trims
// ARMOR_ADMIN_TOKEN, but this gate is also fed config built directly (tests,
// programmatic construction), and an HTTP header cannot carry a raw \n or \r —
// so an untrimmed provisioned newline would make every comparison fail and
// silently disable the admin API. See bead armor-dbcf1139.
func adminTokenValid(r *http.Request, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	provided := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// adminAuthMiddleware gates the admin mux. All non-public routes require a
// bearer token matching s.config.AdminToken (constant-time compare). When the
// token is unset, gated routes are disabled (fail-closed → 403) so an
// unconfigured deployment cannot leak or rotate the MEK. Every gated admin
// call is audit-logged with remote address, route, and outcome. The MEK and
// the token value are never logged.
func (s *Server) adminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public paths (probes, metrics, and the dashboard while it carries its
		// own credentials) bypass the token gate. An unconfigured dashboard does
		// not: it falls through and is gated like any other privileged route.
		if s.isAdminPathPublic(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Capture the real status code for the audit log.
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		token := strings.TrimSpace(s.config.AdminToken)
		if token == "" {
			// Fail-closed: no token configured means the admin API is disabled.
			http.Error(rw, "admin API disabled: ARMOR_ADMIN_TOKEN not set", http.StatusForbidden)
			s.auditAdmin(r, rw.statusCode, "denied", "admin token not configured")
			return
		}

		if !adminTokenValid(r, token) {
			http.Error(rw, "Unauthorized", http.StatusUnauthorized)
			s.auditAdmin(r, rw.statusCode, "denied", "missing or invalid bearer token")
			return
		}

		next.ServeHTTP(rw, r)
		s.auditAdmin(r, rw.statusCode, "allowed", "")
	})
}

// auditAdmin records an admin API access attempt. It logs only metadata —
// remote address, method, path, status, and outcome — and never the MEK or the
// bearer token value.
func (s *Server) auditAdmin(r *http.Request, status int, outcome, reason string) {
	if s.logger == nil {
		return
	}
	fields := map[string]interface{}{
		"event":   "admin_access",
		"remote":  r.RemoteAddr,
		"method":  r.Method,
		"path":    r.URL.Path,
		"status":  status,
		"outcome": outcome,
	}
	if reason != "" {
		fields["reason"] = reason
	}
	s.logger.WithFields(fields).Info("admin API access")
}
