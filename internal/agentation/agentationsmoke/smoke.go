// Package agentationsmoke drives a real headless Chromium against a served
// ARMOR page and asserts that the Agentation toolbar mounted.
//
// The workspace rule for Agentation is "verify by mounting, never by
// grepping the tag": a page with the module tag but no import map still
// renders perfectly while the module dies on the bare "react" specifier, so
// tag-shaped grep can never prove the toolbar works. This package is the
// mounting half of that rule for ARMOR's Go-template pages. It loads the
// page in a real browser and requires #agentation-root in the rendered DOM
// with the toolbar rendered inside it.
//
// The check is deliberately a skip, not a failure, when it cannot run for
// environmental reasons: no browser binary is available (set
// AGENTATION_BROWSER to point at one) or esm.sh is unreachable (the page
// imports React from there, so a mount assertion would fail for the wrong
// reason). scripts/verify-agentation-mount.sh runs it explicitly and
// surfaces the skips; anything short of a genuine loaded-page assertion
// would re-create the tag-without-map blind spot.
package agentationsmoke

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// esmShReact is the pinned React the ARMOR import maps resolve; the
// reachability probe hits the same origin the pages will.
const esmShReact = "https://esm.sh/react@18.3.1"

// Verify loads pageURL in a real headless browser and requires the
// Agentation toolbar to have mounted: #agentation-root exists in the
// rendered DOM and the toolbar rendered inside it. label names the page in
// failure messages (e.g. "proxy dashboard", "fleet console").
//
// Verify skips when no browser is available (AGENTATION_BROWSER overrides
// the search) or esm.sh is unreachable — see the package comment for why
// those are skips and not failures.
func Verify(t testing.TB, label, pageURL string) {
	t.Helper()

	browser := FindBrowser()
	if browser == "" {
		t.Skip("no chromium binary found (set AGENTATION_BROWSER to override); structural wiring tests still ran")
		return
	}
	if !esmShReachable() {
		t.Skip("esm.sh unreachable — the page cannot import react, so a mount assertion would fail for the wrong reason")
		return
	}

	dom, err := dumpDOM(t, browser, pageURL)
	if err != nil {
		t.Fatalf("%s: browser run failed: %v", label, err)
	}
	if strings.Contains(dom, `class="neterror"`) {
		t.Fatalf("%s: browser rendered a network error page, not the app", label)
	}
	if !strings.Contains(dom, `id="agentation-root"`) {
		t.Fatalf("%s: Agentation did not mount — #agentation-root missing from the rendered DOM after load", label)
	}
	if !strings.Contains(dom, "data-agentation-") {
		t.Fatalf("%s: #agentation-root exists but the toolbar never rendered inside it", label)
	}
}

// FindBrowser locates a usable Chromium-family binary: the AGENTATION_BROWSER
// override, then PATH, then the nix store, then Playwright's browser cache.
// It returns "" when none is present.
func FindBrowser() string {
	if override := os.Getenv("AGENTATION_BROWSER"); override != "" {
		if executable(override) {
			return override
		}
		return ""
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if entries, err := os.ReadDir("/nix/store"); err == nil {
		var candidates []string
		for _, entry := range entries {
			name := entry.Name()
			if !regexp.MustCompile(`-chromium-\d`).MatchString(name) || strings.HasSuffix(name, ".drv") {
				continue
			}
			path := filepath.Join("/nix/store", name, "bin", "chromium")
			if executable(path) {
				candidates = append(candidates, path)
			}
		}
		if len(candidates) > 0 {
			sort.Strings(candidates)
			return candidates[len(candidates)-1]
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		// Playwright's browser cache: headless shells and full builds, newest
		// revision wins. The headless shell is headless-only and needs no
		// display, which is exactly what this check wants.
		patterns := []string{
			filepath.Join(home, ".cache", "ms-playwright", "chromium_headless_shell-*", "chrome-linux", "headless_shell"),
			filepath.Join(home, ".cache", "ms-playwright", "chromium-*", "chrome-linux", "chrome"),
		}
		var candidates []string
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(pattern)
			candidates = append(candidates, matches...)
		}
		if len(candidates) > 0 {
			sort.Strings(candidates)
			return candidates[len(candidates)-1]
		}
	}
	return ""
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

// esmShReachable probes the React origin the pages import from, with
// retries so a transient blip does not silently degrade the check to a skip.
func esmShReachable() bool {
	client := &http.Client{Timeout: 10 * time.Second}
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
		}
		resp, err := client.Head(esmShReact)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return true
			}
		}
	}
	return false
}

// dumpDOM renders pageURL and returns the serialized DOM after the page's
// timers and network settle. --virtual-time-budget lets headless Chrome
// fast-forward through the mount check's poll loop and the async React
// import before dumping.
func dumpDOM(t testing.TB, browser, pageURL string) (string, error) {
	t.Helper()

	profile := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, browser,
		"--headless",
		"--no-sandbox",
		"--disable-gpu",
		"--no-proxy-server",
		"--dump-dom",
		"--user-data-dir="+profile,
		"--virtual-time-budget=20000",
		pageURL,
	).Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w (stderr: %s)", browser, err, bytes.TrimSpace(out))
	}
	return string(out), nil
}
