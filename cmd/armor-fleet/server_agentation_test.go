package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/agentation"
	"github.com/jedarden/armor/internal/agentation/agentationsmoke"
)

// The fleet console is one of ARMOR's three UI entry points. These tests pin
// its Agentation wiring and the module endpoint on the real route table; the
// browser mount check runs via
// scripts/verify-agentation-mount.sh.

// TestFleetPageAgentationWiring pins the assembled page head: import map,
// then the module tag the map resolves, then the mount self-check. That
// order is load-bearing — a map after the tag never applies to it, and the
// module dies on the bare "react" specifier while the page still renders,
// which is exactly how the console's placeholder "wiring" shipped unnoticed.
func TestFleetPageAgentationWiring(t *testing.T) {
	s := NewServer(NewFleetMonitor(nil, "", 0), "127.0.0.1:0")
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	page, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	html := string(page)

	moduleTag := `<script type="module" src="/agentation.js"></script>`
	for _, want := range []string{agentation.ImportMapHTML, moduleTag, agentation.MountCheckHTML} {
		if !strings.Contains(html, want) {
			t.Errorf("fleet console page missing Agentation wiring:\n%s", want)
		}
	}
	order := []int{
		strings.Index(html, agentation.ImportMapHTML),
		strings.Index(html, moduleTag),
		strings.Index(html, agentation.MountCheckHTML),
	}
	for i, at := range order {
		if at < 0 {
			t.Fatalf("fleet console wiring piece %d absent; cannot pin ordering", i)
		}
	}
	if !(order[0] < order[1] && order[1] < order[2]) {
		t.Errorf("Agentation wiring out of order: import map at %d, module tag at %d, mount check at %d",
			order[0], order[1], order[2])
	}
	if headEnd := strings.Index(html, "</head>"); headEnd >= 0 && order[2] > headEnd {
		t.Errorf("Agentation wiring lands after </head> (mount check at %d, </head> at %d)", order[2], headEnd)
	}
}

// TestFleetAgentationModuleEndpoint asserts /agentation.js serves the real
// vendored toolbar module from the real route table — not the console.log
// placeholder the console shipped with originally.
func TestFleetAgentationModuleEndpoint(t *testing.T) {
	s := NewServer(NewFleetMonitor(nil, "", 0), "127.0.0.1:0")
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/agentation.js")
	if err != nil {
		t.Fatalf("GET /agentation.js: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read agentation.js body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /agentation.js = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
		t.Errorf("agentation.js Content-Type = %q, want text/javascript", ct)
	}
	if string(body) != agentation.ScriptJS() {
		t.Error("agentation.js body does not match the vendored module")
	}
}

// TestFleetAgentationMountsInBrowser is the mounting half of the
// verification rule: it loads the real page from the real route table in a
// real browser and requires #agentation-root in the rendered DOM with the
// toolbar inside. Skipped under -short and when no browser or esm.sh is
// available — the full definition of done runs it via
// scripts/verify-agentation-mount.sh, which treats an all-skip run as a
// failure so the mount cannot go unverified.
func TestFleetAgentationMountsInBrowser(t *testing.T) {
	if testing.Short() {
		t.Skip("browser smoke runs without -short; see scripts/verify-agentation-mount.sh")
	}

	s := NewServer(NewFleetMonitor(nil, "", 0), "127.0.0.1:0")
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	agentationsmoke.Verify(t, "fleet console", ts.URL+"/")
}
