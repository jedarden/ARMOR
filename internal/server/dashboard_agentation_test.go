package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/agentation"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/dashboard"
)

// The dashboard page wires the Agentation toolbar through three cooperating
// pieces: the import map in the page head, the module tag that loads
// /dashboard/agentation.js, and the route itself. The workspace rule is
// "verify by mounting, never by grepping the tag" — the browser half of that
// lives in internal/agentation/agentationsmoke and runs from
// scripts/verify-agentation-mount.sh; this file pins the production wiring
// the browser half depends on.

// TestDashboardAgentationWiring drives the production Server mux and asserts
// that the served dashboard page carries the import map, module tag and mount
// self-check in that order, and that /dashboard/agentation.js serves the real
// toolbar module under dashboard authentication. The tag without the route is
// the failure mode to guard: /dashboard/agentation.js would fall through to
// the "/dashboard/" prefix handler and answer the module request with HTML,
// which kills the module before it creates #agentation-root.
func TestDashboardAgentationWiring(t *testing.T) {
	const (
		bucket   = "agentation-wiring"
		user     = "agentation-admin"
		password = "agentation-password"
	)

	basePath := t.TempDir()
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: basePath})
	if err != nil {
		t.Fatalf("create filesystem backend: %v", err)
	}
	if err := fsBackend.CreateBucket(context.Background(), bucket); err != nil {
		t.Fatalf("create test bucket: %v", err)
	}

	cfg := &config.Config{
		Bucket:        bucket,
		B2Region:      "us-east-005",
		BlockSize:     65536,
		MEK:           bytes.Repeat([]byte{0x11}, 32),
		DashboardUser: user,
		DashboardPass: password,
	}
	armorServer, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("create ARMOR server: %v", err)
	}
	armorServer.dashboard = dashboard.NewWithAuth(
		fsBackend, bucket, armorServer.metrics, user, password, "", nil, "", false)

	ts := httptest.NewServer(armorServer.AdminHandler())
	t.Cleanup(ts.Close)

	// The module endpoint serves the vendored toolbar module, and nothing
	// else — HTML here is the fall-through-to-prefix-handler failure.
	resp := dashboardRequest(t, ts.Client(), ts.URL+"/dashboard/agentation.js", user, password)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /dashboard/agentation.js = %d, want 200: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
		t.Errorf("agentation.js Content-Type = %q, want text/javascript", ct)
	}
	if string(body) != agentation.ScriptJS() {
		t.Errorf("agentation.js body does not match the vendored module")
	}

	// Anonymous module requests fail closed like every dashboard route.
	resp = dashboardRequest(t, ts.Client(), ts.URL+"/dashboard/agentation.js", "", "")
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous GET /dashboard/agentation.js = %d, want 401", resp.StatusCode)
	}

	// The page head carries import map -> module tag -> mount check, in that
	// order; a map after the tag never applies to it.
	resp = dashboardRequest(t, ts.Client(), ts.URL+"/dashboard", user, password)
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /dashboard = %d", resp.StatusCode)
	}
	html := string(page)
	moduleTag := `<script type="module" src="/dashboard/agentation.js"></script>`
	for _, want := range []string{agentation.ImportMapHTML, moduleTag, agentation.MountCheckHTML} {
		if !strings.Contains(html, want) {
			t.Errorf("dashboard page missing Agentation wiring:\n%s", want)
		}
	}
	order := []int{
		strings.Index(html, agentation.ImportMapHTML),
		strings.Index(html, moduleTag),
		strings.Index(html, agentation.MountCheckHTML),
	}
	for i, at := range order {
		if at < 0 {
			t.Fatalf("dashboard page wiring piece %d absent; cannot pin ordering", i)
		}
	}
	if order[0] >= order[1] || order[1] >= order[2] {
		t.Errorf("Agentation wiring out of order: import map at %d, module tag at %d, mount check at %d",
			order[0], order[1], order[2])
	}
	if headEnd := strings.Index(html, "</head>"); headEnd >= 0 && order[2] > headEnd {
		t.Errorf("Agentation wiring lands after </head> (mount check at %d, </head> at %d)", order[2], headEnd)
	}
}
