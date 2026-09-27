package agentation

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The per-page tests in internal/dashboard and cmd/armor-fleet pin the pages
// that exist today. They cannot see a page that does not exist yet: a new
// HTML page assembled without the Agentation wiring passes every one of them
// and ships exactly the tag-without-map blind spot this package exists to
// prevent. This file is the enumeration regression check — it sweeps every
// production Go file in the repository, treats each page head ("</head>")
// it finds as an HTML entry point, and fails unless that head is assembled
// with the wiring. A new page therefore cannot omit the toolbar without
// failing the test run, and a legitimately wired new page passes with no
// test edits.
//
// Like every check here it runs on source, not on rendered output; the
// rendered-output half stays with the per-page wiring tests and the browser
// smoke (scripts/verify-agentation-mount.sh).

// wiringIdentifiers are the references that may sit immediately before a
// page's </head>: the dashboard's combined head constant, or the individual
// pieces the fleet console assembles. Anything else is a page head this
// package cannot tell is wired.
var wiringIdentifiers = []string{
	"agentationHeadWiring",
	"agentation.ImportMapHTML",
	"agentation.MountCheckHTML",
}

// knownPages pins the enumeration itself: the scan must keep finding these
// files with wired heads in them. A floor, not a ceiling — new pages are
// covered by rule 2, not by being listed here. If one of these stops
// contributing a wired head the scan has gone blind (wrong root, renamed
// file, broken heuristic) and must fail loudly rather than pass vacuously.
var knownPages = []string{
	"internal/dashboard/dashboard.go",
	"cmd/armor-fleet/server.go",
}

// wiringWindow is how far back from "</head>" the wiring reference may sit,
// so a page that assembles its head in a separate expression still passes:
//
//	head := agentation.ImportMapHTML + tag + agentation.MountCheckHTML
//	page := head + "</head></body>"
const wiringWindow = 200

func TestEveryPageHeadCarriesAgentationWiring(t *testing.T) {
	root := "../.."
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("scan root %s is not the repository root (no go.mod): %v", root, err)
	}

	type page struct {
		heads     int      // non-comment "</head>" occurrences
		wired     int      // of those, with wiring in the window before them
		markers   int      // non-comment "<html"/"<!DOCTYPE" page starts
		unwiredAt []string // line numbers of heads with no wiring in the window
	}
	pages := map[string]*page{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "vendor", "testdata", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		text := string(src)
		p := &page{}
		p.heads, p.wired, p.unwiredAt = scanHeads(text)
		p.markers = countOutsideComments(text, "<html") + countOutsideComments(text, "<!DOCTYPE")
		if p.heads > 0 || p.markers > 0 {
			pages[rel] = p
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repository source: %v", err)
	}

	// Rule 1: a page must have a head. HTML assembled with <html but no
	// </head> has no injection point and cannot carry the toolbar.
	for rel, p := range pages {
		if p.markers > 0 && p.heads == 0 {
			t.Errorf("%s looks like an HTML page (has <html or <!DOCTYPE) but has no </head>; "+
				"give it a head and assemble the Agentation wiring into it before the head closes", rel)
		}
	}

	// Rule 2: every page head must be assembled with the wiring.
	for rel, p := range pages {
		for _, at := range p.unwiredAt {
			t.Errorf("%s: page head at %s has no Agentation wiring before it — "+
				"that page ships a toolbar that never mounts (the workspace rule is "+
				"verify-by-mounting, never by grepping the tag); assemble "+
				"agentationHeadWiring, or agentation.ImportMapHTML + the module tag + "+
				"agentation.MountCheckHTML, immediately before </head>", rel, at)
		}
	}

	// Rule 3: the scan must stay anchored to the pages it exists to cover.
	for _, rel := range knownPages {
		p, ok := pages[rel]
		if !ok {
			t.Errorf("enumeration scan found no page head in %s; the scan has gone blind "+
				"(file moved? heuristic broken?) and can no longer protect new pages", rel)
			continue
		}
		if p.wired == 0 {
			t.Errorf("enumeration scan found no wired head in %s", rel)
		}
	}
	if t.Failed() {
		return
	}

	for _, rel := range knownPages {
		t.Logf("page: %s (%d head(s), wired)", rel, pages[rel].heads)
	}
	for rel, p := range pages {
		if isKnownPage(rel) {
			continue
		}
		t.Logf("page: %s (%d head(s), %d wired)", rel, p.heads, p.wired)
	}
}

func isKnownPage(rel string) bool {
	for _, k := range knownPages {
		if rel == k {
			return true
		}
	}
	return false
}

// scanHeads finds every "</head>" outside a // comment and reports, for
// each, whether an Agentation wiring reference sits within the window
// before it.
func scanHeads(text string) (heads, wired int, unwiredAt []string) {
	for at := 0; ; {
		idx := strings.Index(text[at:], "</head>")
		if idx < 0 {
			return heads, wired, unwiredAt
		}
		idx += at
		at = idx + 1

		lineStart := strings.LastIndex(text[:idx], "\n") + 1
		if strings.Contains(text[lineStart:idx], "//") {
			continue // a doc comment mentioning </head>, not a page head
		}
		heads++
		from := idx - wiringWindow
		if from < 0 {
			from = 0
		}
		if wiredInWindow(text[from:idx]) {
			wired++
			continue
		}
		unwiredAt = append(unwiredAt, "line "+strconv.Itoa(strings.Count(text[:idx], "\n")+1))
	}
}

func wiredInWindow(window string) bool {
	for _, id := range wiringIdentifiers {
		if strings.Contains(window, id) {
			return true
		}
	}
	return false
}

// countOutsideComments counts occurrences of substr whose line does not
// carry a // before them, so a comment naming a construct does not make the
// file look like a page.
func countOutsideComments(text, substr string) int {
	n := 0
	for at := 0; ; {
		idx := strings.Index(text[at:], substr)
		if idx < 0 {
			return n
		}
		idx += at
		at = idx + 1
		lineStart := strings.LastIndex(text[:idx], "\n") + 1
		if strings.Contains(text[lineStart:idx], "//") {
			continue
		}
		n++
	}
}
