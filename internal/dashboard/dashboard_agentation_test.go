package dashboard

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/agentation"
	"github.com/jedarden/armor/internal/agentation/agentationsmoke"
	"github.com/jedarden/armor/internal/metrics"
)

// The dashboard page is one of ARMOR's two HTML entry points (the other is
// the fleet console). Per the workspace Agentation rule it must load the
// toolbar with its import map and be verified by mounting, not by grepping
// the tag. These tests pin the page assembly and the module endpoint; the
// production route registration is pinned in internal/server, and the real
// browser mount check runs via scripts/verify-agentation-mount.sh.

// TestDashboardPageAgentationWiring pins the assembled page head: import
// map, then the module tag the map resolves, then the mount self-check —
// that order is load-bearing, because a map after the tag never applies to
// it and the module dies on the bare "react" specifier while the page still
// renders.
func TestDashboardPageAgentationWiring(t *testing.T) {
	mb := newMockBackend()
	d := New(mb, "test-bucket", metrics.NewMetrics())

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	html := rec.Body.String()
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
	if !(order[0] < order[1] && order[1] < order[2]) {
		t.Errorf("Agentation wiring out of order: import map at %d, module tag at %d, mount check at %d",
			order[0], order[1], order[2])
	}
	if headEnd := strings.Index(html, "</head>"); headEnd >= 0 && order[2] > headEnd {
		t.Errorf("Agentation wiring lands after </head> (mount check at %d, </head> at %d)", order[2], headEnd)
	}

	// The tag is root-absolute so prefix navigation (/dashboard/<prefix>/)
	// loads the module from the same route rather than a nested path.
	if !strings.Contains(moduleTag, `src="/dashboard/agentation.js"`) {
		t.Errorf("module tag must use the root-absolute /dashboard/agentation.js route, got %q", moduleTag)
	}
}

// TestDashboardAgentationModuleEndpoint mirrors the production route table
// (page plus module endpoint) and asserts the endpoint serves the vendored
// toolbar module. The module request must never fall through to the page
// handler — HTML answers kill the module before it mounts.
func TestDashboardAgentationModuleEndpoint(t *testing.T) {
	mb := newMockBackend()
	d := New(mb, "test-bucket", metrics.NewMetrics())

	mux := http.NewServeMux()
	mux.HandleFunc("/dashboard", d.Handler())
	mux.HandleFunc("/dashboard/agentation.js", agentation.Handler())

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/dashboard/agentation.js")
	if err != nil {
		t.Fatalf("GET /dashboard/agentation.js: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read agentation.js body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /dashboard/agentation.js = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
		t.Errorf("agentation.js Content-Type = %q, want text/javascript", ct)
	}
	if string(body) != agentation.ScriptJS() {
		t.Error("agentation.js body does not match the vendored module")
	}
}

// TestDashboardAgentationMountsInBrowser is the mounting half of the
// verification rule: it loads the real page from a real server in a real
// browser and requires #agentation-root in the rendered DOM with the
// toolbar inside. It is skipped under -short (the fast definition-of-done
// lane runs short) and when no browser or esm.sh is available — the full
// definition of done runs it via scripts/verify-agentation-mount.sh, which
// treats an all-skip run as a failure so the mount cannot go unverified.
func TestDashboardAgentationMountsInBrowser(t *testing.T) {
	if testing.Short() {
		t.Skip("browser smoke runs without -short; see scripts/verify-agentation-mount.sh")
	}

	mb := newMockBackend()
	d := New(mb, "test-bucket", metrics.NewMetrics())

	mux := http.NewServeMux()
	mux.HandleFunc("/dashboard", d.Handler())
	mux.HandleFunc("/dashboard/agentation.js", agentation.Handler())

	ts := httptest.NewServer(mux)
	defer ts.Close()

	agentationsmoke.Verify(t, "proxy dashboard", ts.URL+"/dashboard")
}
