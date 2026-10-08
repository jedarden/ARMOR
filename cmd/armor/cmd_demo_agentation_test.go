package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/agentation"
	"github.com/jedarden/armor/internal/agentation/agentationsmoke"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server"
)

// The demo command serves the same dashboard through its admin listener, but
// it is a distinct documented entry point and must be exercised independently
// of the production dashboard handler tests.

func newDemoAgentationServer(t *testing.T) (*server.Server, *httptest.Server, string) {
	t.Helper()

	cfg := &config.Config{
		Listen:             "127.0.0.1:0",
		AdminListen:        "127.0.0.1:0",
		Backend:            "filesystem",
		FSPath:             t.TempDir(),
		Bucket:             "demo-bucket",
		MEK:                bytes.Repeat([]byte{0x42}, 32),
		BlockSize:          64 * 1024,
		DashboardUser:      "agentation-demo",
		DashboardPass:      t.Name(),
		AllowNoCredentials: true,
		CanaryDisabled:     true,
	}
	seedBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: cfg.FSPath})
	if err != nil {
		t.Fatalf("create demo backend: %v", err)
	}
	if err := seedBackend.CreateBucket(context.Background(), cfg.Bucket); err != nil {
		t.Fatalf("create demo bucket: %v", err)
	}
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("create demo server: %v", err)
	}

	ts := httptest.NewServer(srv.AdminHandler())
	t.Cleanup(func() {
		ts.Close()
		srv.StopAuthFileWatcher()
		srv.StopCanary()
		srv.StopReplicationQueue()
		srv.StopManifestCompactor()
		srv.StopManifestWriter()
	})

	pageURL, err := url.Parse(ts.URL + "/dashboard")
	if err != nil {
		t.Fatalf("parse demo dashboard URL: %v", err)
	}
	pageURL.User = url.UserPassword(cfg.DashboardUser, cfg.DashboardPass)
	return srv, ts, pageURL.String()
}

func TestDemoDashboardAgentationWiring(t *testing.T) {
	_, _, pageURL := newDemoAgentationServer(t)

	resp, err := http.Get(pageURL)
	if err != nil {
		t.Fatalf("GET demo dashboard: %v", err)
	}
	defer resp.Body.Close()
	page, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read demo dashboard: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET demo dashboard = %d, want 200", resp.StatusCode)
	}

	html := string(page)
	moduleTag := `<script type="module" src="/dashboard/agentation.js"></script>`
	for _, want := range []string{agentation.ImportMapHTML, moduleTag, agentation.MountCheckHTML} {
		if !strings.Contains(html, want) {
			t.Errorf("demo dashboard page missing Agentation wiring:\n%s", want)
		}
	}
	order := []int{
		strings.Index(html, agentation.ImportMapHTML),
		strings.Index(html, moduleTag),
		strings.Index(html, agentation.MountCheckHTML),
	}
	for i, at := range order {
		if at < 0 {
			t.Fatalf("demo dashboard wiring piece %d absent; cannot pin ordering", i)
		}
	}
	if order[0] >= order[1] || order[1] >= order[2] {
		t.Errorf("demo dashboard Agentation wiring out of order: import map at %d, module tag at %d, mount check at %d", order[0], order[1], order[2])
	}
	if headEnd := strings.Index(html, "</head>"); headEnd >= 0 && order[2] > headEnd {
		t.Errorf("demo dashboard Agentation wiring lands after </head>")
	}
}

func TestDemoAgentationModuleEndpoint(t *testing.T) {
	_, ts, pageURL := newDemoAgentationServer(t)
	parsed, err := url.Parse(pageURL)
	if err != nil {
		t.Fatalf("parse demo dashboard URL: %v", err)
	}
	parsed.Path = "/dashboard/agentation.js"

	resp, err := ts.Client().Get(parsed.String())
	if err != nil {
		t.Fatalf("GET demo Agentation module: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read demo Agentation module: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET demo Agentation module = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
		t.Errorf("demo Agentation module Content-Type = %q, want JavaScript", ct)
	}
	if string(body) != agentation.ScriptJS() {
		t.Error("demo Agentation module does not match the vendored module")
	}
}

func TestDemoAgentationMountsInBrowser(t *testing.T) {
	if testing.Short() {
		t.Skip("browser smoke runs without -short; see scripts/verify-agentation-mount.sh")
	}

	_, _, pageURL := newDemoAgentationServer(t)
	agentationsmoke.Verify(t, "demo dashboard", pageURL)
}
