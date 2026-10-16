package handlers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var goIdentifierInClientText = regexp.MustCompile(`\b(?:WrapDEKWithFingerprint|PutObject|partNumber|[a-z][a-z0-9_]*\.[A-Z][A-Za-z0-9_]*)\b`)

// TestWriteErrorCallSitesDoNotExposeImplementationDetails keeps the source
// catalogue boundary honest. Dynamic diagnostics are allowed only as the
// argument to a resolver call; string literals passed to a writeError method
// must not contain Go-style identifiers or operator-only admin URLs.
func TestWriteErrorCallSitesDoNotExposeImplementationDetails(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(thisFile)
	files, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, filepath.Join(root, "..", "server.go"))

	for _, filename := range files {
		if err := checkWriteErrorCallSites(filename); err != nil {
			t.Error(err)
		}
	}
}

func checkWriteErrorCallSites(filename string) error {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, contents, 0)
	if err != nil {
		return err
	}
	var firstErr error
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 4 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (selector.Sel.Name != "writeError" && selector.Sel.Name != "writeErrorCause" && selector.Sel.Name != "writeBackendError") {
			return true
		}

		ast.Inspect(call.Args[3], func(child ast.Node) bool {
			literal, ok := child.(*ast.BasicLit)
			if !ok || literal.Kind.String() != "STRING" {
				return true
			}
			text, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr != nil {
				return true
			}
			if strings.Contains(text, "/admin/") {
				firstErr = errorAt(fset, literal, "client error text contains an operator-only /admin/ URL")
				return false
			}
			if goIdentifierInClientText.MatchString(text) {
				firstErr = errorAt(fset, literal, "client error text contains a Go-style identifier")
				return false
			}
			return true
		})
		return firstErr == nil
	})
	return firstErr
}

func errorAt(fset *token.FileSet, node ast.Node, message string) error {
	return &callSiteError{position: fset.Position(node.Pos()), message: message}
}

type callSiteError struct {
	position token.Position
	message  string
}

func (e *callSiteError) Error() string {
	return e.position.String() + ": " + e.message
}
