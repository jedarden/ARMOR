package docsindex

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHandlerErrorCodes(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("handlers.go", `package handlers

func (h *Handlers) f(w http.ResponseWriter, r *http.Request) {
	h.writeError(w, r, "AccessDenied", "nope", 403)
	h.writeBackendError(w, r, "InternalError",
		"failed", 500, err)
}

func (h *Handlers) writeErrorCause(w http.ResponseWriter, r *http.Request, code, message string, statusCode int, cause error) {
	switch x {
	case y:
		code, message, statusCode = "RequestTimeout", "deadline", 503
	case z:
		code, statusCode = "ClientDisconnect", 499
	}
}
`)
	write("handlers_test.go", `package handlers

func TestX(t *testing.T) {
	h.writeError(w, r, "ShouldNotCount", "x", 400)
}
`)

	codes, err := HandlerErrorCodes(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"AccessDenied":   true,
		"InternalError":  true,
		"RequestTimeout": true,
	}
	if !reflect.DeepEqual(codes, want) {
		t.Errorf("HandlerErrorCodes() = %v, want %v", codes, want)
	}
}

func TestDocumentedErrorCodes(t *testing.T) {
	doc := []byte("# Doc\n\n| `AccessDenied` | Access Denied | ACL rejection |\n" +
		"| `InvalidRange` | Invalid range | out of bounds |\n" +
		"| not-a-code | plain text | first cell has no backticks |\n" +
		"| `/healthz` | GET | not a PascalCase code |\n")

	got := DocumentedErrorCodes(doc)
	want := map[string]bool{"AccessDenied": true, "InvalidRange": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DocumentedErrorCodes() = %v, want %v", got, want)
	}
}

func TestDocumentedErrorCodesScopedToMarkers(t *testing.T) {
	doc := []byte("# Doc\n\n" +
		"<!-- handlers-error-codes:start -->\n" +
		"| `AccessDenied` | 403 |\n" +
		"<!-- handlers-error-codes:end -->\n\n" +
		"## Auth table (not part of the handlers contract)\n" +
		"| `MissingAuthenticationToken` | Missing Authentication Token | 403 | |\n")

	got := DocumentedErrorCodes(doc)
	want := map[string]bool{"AccessDenied": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DocumentedErrorCodes() = %v, want %v (auth table outside the markers must not count)", got, want)
	}
}

func TestErrorCodeRefProblems(t *testing.T) {
	emitted := map[string]bool{"A": true, "B": true}
	documented := map[string]bool{"B": true, "C": true}
	got := ErrorCodeRefProblems(emitted, documented)
	want := []string{"documented but never emitted: C", "emitted but not documented: A"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ErrorCodeRefProblems() = %q, want %q", got, want)
	}
}

// TestHandlersErrorCodesDocumented is the live acceptance check: every S3
// error code internal/server/handlers can emit must appear in
// docs/error-responses.md, and the doc must not claim a code the package
// never emits.
func TestHandlersErrorCodesDocumented(t *testing.T) {
	root := repoRoot(t)
	handlersDir := filepath.Join(root, "internal", "server", "handlers")

	emitted, err := HandlerErrorCodes(handlersDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(emitted) == 0 {
		t.Fatal("HandlerErrorCodes found no codes — regexp likely stopped matching the package's call shape")
	}

	doc, err := os.ReadFile(filepath.Join(root, "docs", "error-responses.md"))
	if err != nil {
		t.Fatal(err)
	}
	documented := DocumentedErrorCodes(doc)

	if problems := ErrorCodeRefProblems(emitted, documented); len(problems) > 0 {
		t.Errorf("docs/error-responses.md is not in parity with internal/server/handlers:\n  %s",
			strings.Join(problems, "\n  "))
	}
}
