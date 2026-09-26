package agentation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// The workspace rule is "verify by mounting, never by grepping the tag" —
// these tests pin the wiring contract that makes a mount possible; the live
// half is scripts/verify-agentation-mount.sh.

// TestScriptJSCreatesAgentationRoot pins the contract the mount check and
// the browser smoke test both depend on: running the module creates the
// #agentation-root element and renders the toolbar into it.
func TestScriptJSCreatesAgentationRoot(t *testing.T) {
	script := ScriptJS()
	for _, want := range []string{
		"createElement('div')",
		"'agentation-root'",
		"appendChild(mount)",
		"createRoot(mount)",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("agentation.js lost %q; the toolbar can no longer mount", want)
		}
	}
}

// TestScriptJSImportsResolveThroughImportMap extracts every import
// specifier agentation.js uses and requires each to be either an absolute
// URL or a key of the import map. This is the exact failure that shipped on
// dashboard.ardenone.com: a specifier the map does not cover kills the
// module before it creates #agentation-root, with the page rendering fine.
func TestScriptJSImportsResolveThroughImportMap(t *testing.T) {
	var imports struct {
		Imports map[string]string `json:"imports"`
	}
	// ImportMapHTML is the full script block; the import map itself is the
	// JSON between the tags.
	const mapOpen = `<script type="importmap">`
	open := strings.Index(ImportMapHTML, mapOpen)
	if open < 0 {
		t.Fatal("ImportMapHTML has no importmap script tag")
	}
	rest := ImportMapHTML[open+len(mapOpen):]
	close := strings.Index(rest, "</script>")
	if close < 0 {
		t.Fatal("ImportMapHTML script tag is unterminated")
	}
	if err := json.Unmarshal([]byte(rest[:close]), &imports); err != nil {
		t.Fatalf("ImportMapHTML is not a valid import map: %v", err)
	}
	if len(imports.Imports) == 0 {
		t.Fatal("ImportMapHTML maps no specifiers")
	}

	specifierRe := regexp.MustCompile(`(?:import|from)\s+'([^']+)'`)
	seen := map[string]bool{}
	for _, m := range specifierRe.FindAllStringSubmatch(ScriptJS(), -1) {
		spec := m[1]
		seen[spec] = true
		if strings.HasPrefix(spec, "https://") || strings.HasPrefix(spec, "http://") {
			continue
		}
		if _, mapped := imports.Imports[spec]; !mapped {
			t.Errorf("agentation.js imports %q, which ImportMapHTML does not map; the module dies on 'Failed to resolve module specifier'", spec)
		}
	}
	for _, want := range []string{"react", "react-dom/client"} {
		if !seen[want] {
			t.Errorf("expected agentation.js to import %q; if the vendored script changed, re-prove it mounts", want)
		}
	}
}

// TestImportMapPinsReactThroughEsmSh keeps the map on the same pinned,
// same-origin-consistent CDN set every ARMOR page and dashboard-site use.
func TestImportMapPinsReactThroughEsmSh(t *testing.T) {
	for specifier, url := range map[string]string{
		"react":             "https://esm.sh/react@18.3.1",
		"react-dom":         "https://esm.sh/react-dom@18.3.1",
		"react-dom/client":  "https://esm.sh/react-dom@18.3.1/client",
		"react/jsx-runtime": "https://esm.sh/react@18.3.1/jsx-runtime",
	} {
		if !strings.Contains(ImportMapHTML, `"`+specifier+`": "https://esm.sh/`) {
			t.Errorf("import map is missing the pinned esm.sh entry for %q", specifier)
		}
		if url != "" && !strings.Contains(ImportMapHTML, url) {
			t.Errorf("import map drifted from the pinned URL for %q: %s", specifier, url)
		}
	}
}

// TestMountCheckLogsBothVerdicts pins the exact console lines the browser
// smoke test greps. A self-check with only a failure branch can never prove
// a mount; one with only a success branch cannot report the failure mode.
func TestMountCheckLogsBothVerdicts(t *testing.T) {
	for _, want := range []string{
		"✓ Agentation mounted successfully",
		"Agentation did not mount",
		"getElementById('agentation-root')",
	} {
		if !strings.Contains(MountCheckHTML, want) {
			t.Errorf("MountCheckHTML lost %q", want)
		}
	}
}

// TestScriptJSIsNotThePlaceholder guards the regression this wiring exists
// to fix: the fleet console served a script tag whose body was a
// console.log placeholder, which reads as wiring in every grep and mounts
// nothing in any browser.
func TestScriptJSIsNotThePlaceholder(t *testing.T) {
	if strings.Contains(ScriptJS(), "placeholder") {
		t.Error("agentation.js is a placeholder; it cannot mount the toolbar")
	}
}

func TestHandler(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/agentation.js")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
		t.Errorf("Expected Content-Type text/javascript, got %q", ct)
	}
}
