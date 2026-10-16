package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/agentation"
	"github.com/jedarden/armor/internal/agentation/agentationsmoke"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server"
	"github.com/jedarden/armor/internal/server/srvtest"
)

// The demo command serves the same dashboard through its admin listener, but
// it is a distinct documented entry point and must be exercised independently
// of the production dashboard handler tests.

func demoConfigForTest(t *testing.T, demoDir, listen, adminListen string) (*config.Config, demoCredentials) {
	t.Helper()

	for _, name := range []string{
		"ARMOR_BACKEND", "ARMOR_FS_PATH", "ARMOR_BUCKET", "ARMOR_MEK",
		"ARMOR_AUTH_ACCESS_KEY", "ARMOR_AUTH_SECRET_KEY", "ARMOR_ALLOW_NO_CREDENTIALS",
		"ARMOR_LISTEN", "ARMOR_ADMIN_LISTEN", "ARMOR_DASHBOARD_USER",
		"ARMOR_DASHBOARD_PASS", "ARMOR_ADMIN_TOKEN",
	} {
		t.Setenv(name, "")
	}

	mekPath := demoMEKFilePath(demoDir, true, "")
	mek, err := demoMEK(mekPath)
	if err != nil {
		t.Fatalf("load demo MEK: %v", err)
	}
	cfg, credentials, err := configureDemo(demoDir, hex.EncodeToString(mek), listen, adminListen)
	if err != nil {
		t.Fatalf("configure demo defaults: %v", err)
	}
	if cfg.DashboardUser == "" || cfg.DashboardPass == "" || cfg.AdminToken == "" {
		t.Fatalf("demo defaults omitted dashboard auth: user=%q pass-set=%t admin-token-set=%t", cfg.DashboardUser, cfg.DashboardPass != "", cfg.AdminToken != "")
	}
	if cfg.DashboardUser != credentials.dashboardUser || cfg.DashboardPass != credentials.dashboardPass || cfg.AdminToken != credentials.adminToken {
		t.Fatal("demo defaults did not pass printed dashboard credentials into the loaded config")
	}
	return cfg, credentials
}

func newDemoAgentationServer(t *testing.T) (*server.Server, *httptest.Server, string) {
	t.Helper()

	cfg, credentials := demoConfigForTest(t, t.TempDir(), "127.0.0.1:0", "127.0.0.1:0")
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
		stopDemoServer(srv)
	})

	pageURL, err := url.Parse(ts.URL + "/dashboard")
	if err != nil {
		t.Fatalf("parse demo dashboard URL: %v", err)
	}
	pageURL.User = url.UserPassword(credentials.dashboardUser, credentials.dashboardPass)
	return srv, ts, pageURL.String()
}

func stopDemoServer(srv *server.Server) {
	srv.StopAuthFileWatcher()
	srv.StopCanary()
	srv.StopReplicationQueue()
	srv.StopManifestCompactor()
	srv.StopManifestWriter()
}

func TestDemoMEKFilePersistsWithPrivatePermissions(t *testing.T) {
	demoDir := t.TempDir()
	mekPath := demoMEKFilePath(demoDir, true, "")
	first, err := demoMEK(mekPath)
	if err != nil {
		t.Fatalf("create demo MEK: %v", err)
	}
	second, err := demoMEK(mekPath)
	if err != nil {
		t.Fatalf("reuse demo MEK after restart: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("demo MEK changed between runs using the same -dir")
	}
	info, err := os.Stat(mekPath)
	if err != nil {
		t.Fatalf("stat demo MEK file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("demo MEK file mode = %04o, want 0600", got)
	}
	if got := filepath.Base(mekPath); got != ".armor-demo-mek" {
		t.Errorf("default demo MEK file = %q, want .armor-demo-mek", got)
	}
}

func TestDemoDirKeepsObjectsReadableAfterRestart(t *testing.T) {
	demoDir := t.TempDir()
	firstConfig, _ := demoConfigForTest(t, demoDir, "127.0.0.1:0", "127.0.0.1:0")
	seedBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: firstConfig.FSPath})
	if err != nil {
		t.Fatalf("create demo backend: %v", err)
	}
	if err := seedBackend.CreateBucket(context.Background(), firstConfig.Bucket); err != nil {
		t.Fatalf("create demo bucket: %v", err)
	}
	firstServer, err := server.New(firstConfig)
	if err != nil {
		t.Fatalf("create first demo server: %v", err)
	}
	firstHTTP := httptest.NewServer(firstServer.Handler())
	firstStopped := false
	t.Cleanup(func() {
		firstHTTP.Close()
		if !firstStopped {
			stopDemoServer(firstServer)
		}
	})

	want := []byte("demo object survives a restart")
	put, err := http.NewRequest(http.MethodPut, firstHTTP.URL+"/"+firstConfig.Bucket+"/restart-check.txt", bytes.NewReader(want))
	if err != nil {
		t.Fatalf("create demo PUT request: %v", err)
	}
	srvtest.SignS3Request(put, want, "armor", "armor-demo-secret", "us-east-005")
	putResp, err := firstHTTP.Client().Do(put)
	if err != nil {
		t.Fatalf("PUT demo object: %v", err)
	}
	io.Copy(io.Discard, putResp.Body)
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT demo object = %d, want 200", putResp.StatusCode)
	}

	firstHTTP.Close()
	stopDemoServer(firstServer)
	firstStopped = true

	secondConfig, _ := demoConfigForTest(t, demoDir, "127.0.0.1:0", "127.0.0.1:0")
	secondServer, err := server.New(secondConfig)
	if err != nil {
		t.Fatalf("create restarted demo server: %v", err)
	}
	secondHTTP := httptest.NewServer(secondServer.Handler())
	t.Cleanup(func() {
		secondHTTP.Close()
		stopDemoServer(secondServer)
	})

	getReq, err := http.NewRequest(http.MethodGet, secondHTTP.URL+"/"+secondConfig.Bucket+"/restart-check.txt", nil)
	if err != nil {
		t.Fatalf("create demo GET request: %v", err)
	}
	srvtest.SignS3Request(getReq, nil, "armor", "armor-demo-secret", "us-east-005")
	getResp, err := secondHTTP.Client().Do(getReq)
	if err != nil {
		t.Fatalf("GET demo object after restart: %v", err)
	}
	defer getResp.Body.Close()
	got, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("read demo object after restart: %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET demo object after restart = %d, want 200: %s", getResp.StatusCode, got)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("GET demo object after restart = %q, want %q", got, want)
	}
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
