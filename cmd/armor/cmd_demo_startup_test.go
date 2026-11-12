package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/server"
)

// The demo's startup output has two surfaces with two different audiences:
// the structured JSON log lines a pipeline would collect, and the plain-text
// banner a human reads in their terminal. Credentials belong to the banner
// only — these tests pin both sides of that split, and the readiness fix
// that keeps an idle demo's /readyz from sitting at 503 forever.

func TestDemoBannerPlainTextCarriesCredentials(t *testing.T) {
	credentials := demoCredentials{
		dashboardUser: "demo-feedface",
		dashboardPass: "demo-pass-value",
		adminToken:    "demo-token-value",
	}
	var buf bytes.Buffer
	printDemoBanner(&buf, demoBanner{
		Dir:          "/tmp/armor-demo-123",
		Listen:       "127.0.0.1:19000",
		AdminListen:  "127.0.0.1:19001",
		DashboardURL: "http://127.0.0.1:19001/dashboard",
		Credentials:  credentials,
	})
	got := buf.String()

	for _, want := range []string{
		"demo-feedface",
		"demo-pass-value",
		"demo-token-value",
		"armor-demo-secret",
		"access key: armor",
		"http://127.0.0.1:19001/dashboard",
		"127.0.0.1:19000",
		"/tmp/armor-demo-123",
		"Press Ctrl-C to stop",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("demo banner missing %q:\n%s", want, got)
		}
	}
}

func TestDemoStartupJSONLogsCarryNoCredentials(t *testing.T) {
	credentials := demoCredentials{
		dashboardUser: "demo-cafe",
		dashboardPass: "secret-pass-value",
		adminToken:    "secret-token-value",
	}
	var logBuf, bannerBuf bytes.Buffer
	logger := logging.New("armor-demo")
	logger.SetOutput(&logBuf)
	announceDemo(&bannerBuf, logger, demoBanner{
		Dir:          "/tmp/armor-demo-456",
		Listen:       "127.0.0.1:19000",
		AdminListen:  "127.0.0.1:19001",
		DashboardURL: "http://127.0.0.1:19001/dashboard",
		Credentials:  credentials,
	})

	logged := logBuf.String()
	if logged == "" {
		t.Fatal("demo startup emitted no structured log line")
	}
	for _, line := range strings.Split(strings.TrimSuffix(logged, "\n"), "\n") {
		var entry logging.Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("demo structured output is not one JSON object per line: %v\n%s", err, logged)
		}
		if entry.Msg == "" {
			t.Errorf("demo structured output carries a blank msg line:\n%s", logged)
		}
		for _, secret := range []string{"demo-cafe", "secret-pass-value", "secret-token-value", "armor-demo-secret"} {
			if strings.Contains(line, secret) {
				t.Errorf("credential value %q reached the structured log surface:\n%s", secret, line)
			}
		}
	}

	// The banner is where those same values do belong.
	for _, want := range []string{"demo-cafe", "secret-pass-value", "secret-token-value"} {
		if !strings.Contains(bannerBuf.String(), want) {
			t.Errorf("banner missing %q:\n%s", want, bannerBuf.String())
		}
	}
}

// TestDemoReadyzBecomesReady reproduces the reported defect end to end: the
// demo S3 listener's /readyz sat at 503 ("manifest writer has never
// flushed") for an entirely idle run, because the demo runs no canary and an
// idle filesystem deployment never writes a manifest delta.
func TestDemoReadyzBecomesReady(t *testing.T) {
	cfg, _ := demoConfigForTest(t, t.TempDir(), "127.0.0.1:0", "127.0.0.1:0")
	if err := initializeDemoBucket(cfg); err != nil {
		t.Fatalf("initialize demo bucket: %v", err)
	}
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("create demo server: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.Close()
		stopDemoServer(srv)
	})

	deadline := time.Now().Add(5 * time.Second)
	var lastCode int
	var lastBody string
	for {
		resp, err := http.Get(ts.URL + "/readyz")
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				t.Fatalf("read readyz body: %v", readErr)
			}
			lastCode, lastBody = resp.StatusCode, string(body)
			if lastCode == http.StatusOK {
				return
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("demo /readyz never became ready: last status %d body %s", lastCode, lastBody)
}
