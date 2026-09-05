package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// TestDashboardPresignRoutesToAdminPresign pins the loopback wiring for the
// dashboard share-link handler. PresignHandlerWithAuth takes the full admin
// URL and posts to it verbatim, so the route must be handed /admin/presign.
// Pointing it at /admin/key/rotate (the rotate endpoint's URL was shared with
// this route) makes presign unreachable on every deployment — rotate reads the
// body as a legacy request-body MEK and answers 400 "Invalid MEK length" —
// and turns a UI action into a POST against a destructive admin route.
func TestDashboardPresignRoutesToAdminPresign(t *testing.T) {
	var gotPaths []string
	adminTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"url":"https://armor.example.com/share/abc123","expires_in":"1h","expires_at":"2026-09-06T00:00:00Z"}`)
	}))
	defer adminTarget.Close()

	cfg := &config.Config{
		Backend:        "filesystem",
		FSPath:         t.TempDir(),
		Bucket:         "test-bucket",
		MEK:            make([]byte, 32),
		PresignEnabled: true,
		PresignSecret:  bytes.Repeat([]byte{0x07}, 32),
		PresignBaseURL: "https://armor.example.com/share",
		DashboardUser:  "dash-user",
		DashboardPass:  "dash-pass",
		AuthAccessKey:  "test-key",
		AuthSecretKey:  "test-secret",
		AdminListen:    strings.TrimPrefix(adminTarget.URL, "http://"),
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/dashboard/presign",
		strings.NewReader(`{"key":"test/file.txt","expires_in":"1h"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("dash-user", "dash-pass")
	rec := httptest.NewRecorder()

	srv.AdminHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("/dashboard/presign = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if len(gotPaths) != 1 || gotPaths[0] != "/admin/presign" {
		t.Errorf("/dashboard/presign proxied to %v, want [/admin/presign]", gotPaths)
	}
}
