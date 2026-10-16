package docsindex

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// This file keeps docs/error-responses.md's S3 error-code tables honest
// against internal/server/handlers: HandlerErrorCodes extracts every S3
// <Code> value the package can write to a client, DocumentedErrorCodes
// extracts every code the doc's tables claim, and ErrorCodeRefProblems
// reports any mismatch in either direction.

var (
	// handlerErrorCode matches a literal S3 error code passed as the code
	// argument of writeError, writeErrorCause or writeBackendError (all
	// share the (w, r, code, message, statusCode[, cause]) shape), including
	// calls whose arguments wrap onto multiple lines.
	handlerErrorCode = regexp.MustCompile(`write(?:Error|ErrorCause|BackendError)\(\s*w,\s*r,\s*"([A-Z][A-Za-z]+)"`)
	// handlerReassignedCode matches writeErrorCause's inline
	// `code, message, statusCode = "RequestTimeout", ...` reassignment: the
	// one case where a literal code reaches the client without appearing as
	// a call argument. Its sibling ClientDisconnect reassignment is
	// deliberately not matched here: that one fires only after the response
	// write has already failed, so the code never actually reaches a client
	// and documenting it as a wire-level code would be wrong.
	handlerReassignedCode = regexp.MustCompile(`code,\s*message,\s*statusCode\s*=\s*"([A-Z][A-Za-z]+)"`)
	// docErrorCode matches a table row in error-responses.md whose first
	// cell is a backtick-quoted error code, e.g. "| `InvalidRange` | ... |".
	docErrorCode = regexp.MustCompile("(?m)^\\| `([A-Z][A-Za-z]+)` \\|")
	// markedSection extracts the content between the doc's
	// handlers-error-codes HTML-comment markers, which bound the one table
	// that enumerates every code internal/server/handlers emits. Other
	// tables in the doc (e.g. the auth-error codes internal/server/auth.go
	// defines) legitimately reuse the same `` `Code` `` row shape without
	// being part of this package's parity contract.
	markedSection = regexp.MustCompile(`(?s)<!-- handlers-error-codes:start -->(.*?)<!-- handlers-error-codes:end -->`)
)

// HandlerErrorCodes returns the set of S3 <Code> values internal/server/handlers
// can write to a client, scanned from its non-test Go source. handlersDir is
// that package's directory.
func HandlerErrorCodes(handlersDir string) (map[string]bool, error) {
	codes := make(map[string]bool)
	entries, err := os.ReadDir(handlersDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(handlersDir, name))
		if err != nil {
			return nil, err
		}
		src := string(data)
		for _, m := range handlerErrorCode.FindAllStringSubmatch(src, -1) {
			codes[m[1]] = true
		}
		for _, m := range handlerReassignedCode.FindAllStringSubmatch(src, -1) {
			codes[m[1]] = true
		}
	}
	return codes, nil
}

// DocumentedErrorCodes returns the set of S3 error codes error-responses.md's
// handlers-error-codes table claims, parsed from its raw content. When the
// handlers-error-codes markers are present, only the text between them is
// scanned; otherwise (e.g. a fixture doc in a test) the whole input is.
func DocumentedErrorCodes(doc []byte) map[string]bool {
	content := doc
	if m := markedSection.FindSubmatch(doc); m != nil {
		content = m[1]
	}
	codes := make(map[string]bool)
	for _, m := range docErrorCode.FindAllStringSubmatch(string(content), -1) {
		codes[m[1]] = true
	}
	return codes
}

// ErrorCodeRefProblems reports every code the handlers emit that the
// documented set omits, and every documented code the handlers never emit,
// sorted.
func ErrorCodeRefProblems(emitted, documented map[string]bool) []string {
	var problems []string
	for c := range emitted {
		if !documented[c] {
			problems = append(problems, "emitted but not documented: "+c)
		}
	}
	for c := range documented {
		if !emitted[c] {
			problems = append(problems, "documented but never emitted: "+c)
		}
	}
	sort.Strings(problems)
	return problems
}
