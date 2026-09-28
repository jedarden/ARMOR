// Package agentation wires the Agentation visual-feedback toolbar into
// ARMOR's web surfaces (the proxy dashboard, demo dashboard, and fleet
// console).
//
// Agentation is the workspace-standard UI feedback tool: it adds a toolbar
// that turns clicks, text selections, and drag-selects into structured
// markdown with CSS selectors, so feedback carries precise DOM pointers
// instead of prose descriptions.
//
// The wiring has a failure mode that makes naive integration worthless:
// agentation.js imports the bare specifiers "react" and "react-dom/client",
// which a browser can only resolve through an import map, and the import
// map must appear in the document before the module tag that needs it.
// Without the map the module dies on 'Failed to resolve module specifier
// "react"', the toolbar never mounts, and the page still renders perfectly
// — so the script tag reads as proof of something that is not happening.
// Pages must therefore be verified by mounting (#agentation-root exists in
// the DOM after load), never by grepping for the tag.
//
// This package is the single source of truth for that wiring:
//
//   - agentation.js — the bootstrapping module, vendored verbatim (see the
//     provenance note beside it); it creates #agentation-root and renders
//     the toolbar into it. React and the agentation module itself load from
//     esm.sh at runtime, deliberately not vendored: a blocked or unreachable
//     esm.sh fails this one non-critical module independently and neither
//     hangs nor breaks the page.
//   - ImportMapHTML — the import map resolving the bare specifiers above.
//   - MountCheckHTML — the per-page self-check that logs a mount verdict
//     the browser smoke test (scripts/verify-agentation-mount.sh) reads.
//
// ARMOR's UI pages are Go string templates, so each page assembles its head
// as ImportMapHTML + its own module tag + MountCheckHTML, in that order; the
// tests in this package and on each page pin the ordering and the mount
// contract.
package agentation

import (
	_ "embed"
	"net/http"
)

// agentation.js is vendored verbatim from dashboard-site's
// public/agentation.js, which is itself the agentation npm package's
// documented bootstrapping snippet (agentation@3.0.2 via esm.sh, react
// external). It is byte-identical to the copy proven to mount on
// dashboard.ardenone.com. Update it by copying the new snippet over the
// file; agentation_test.go pins the contract the file must keep.
//
//go:embed agentation.js
var scriptJS string

// ScriptJS returns the Agentation bootstrapping module served at an
// /agentation.js endpoint.
func ScriptJS() string { return scriptJS }

// ImportMapHTML resolves the bare specifiers agentation.js imports. It must
// appear in the document before any module tag that needs it. It ends with a
// newline so pages can concatenate their module tag directly after it.
const ImportMapHTML = `<script type="importmap">
{
    "imports": {
        "react": "https://esm.sh/react@18.3.1",
        "react-dom": "https://esm.sh/react-dom@18.3.1",
        "react-dom/client": "https://esm.sh/react-dom@18.3.1/client",
        "react/jsx-runtime": "https://esm.sh/react@18.3.1/jsx-runtime"
    }
}
</script>
`

// MountCheckHTML is the per-page mount self-check. The toolbar mounts
// asynchronously (its imports resolve over the network), so the check polls
// for #agentation-root and logs one of the verdicts the browser smoke test
// (scripts/verify-agentation-mount.sh) reads: "✓ Agentation mounted
// successfully" on success, "Agentation did not mount" on failure. Both
// branches are structural — a page that can only warn cannot prove a mount,
// which is exactly how the tag-without-map failure shipped unnoticed.
const MountCheckHTML = `<script type="module">
(() => {
  const deadline = Date.now() + 10000;
  const poll = () => {
    if (document.getElementById('agentation-root')) {
      console.log('✓ Agentation mounted successfully');
      return;
    }
    if (Date.now() >= deadline) {
      console.warn('Agentation did not mount: #agentation-root absent');
      return;
    }
    setTimeout(poll, 100);
  };
  poll();
})();
</script>
`

// Handler serves the Agentation bootstrapping module.
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write([]byte(scriptJS))
	}
}
