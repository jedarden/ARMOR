package docsindex

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeTree writes body to the fixture file named rel under root, creating
// parent directories as needed.
func writeTree(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConsumerEnvVars(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "cmd/armor/main.go", `package main
var _ = os.Getenv("ARMOR_A")
var _ = "ARMOR_" + "BUILT" // a bare prefix, not a variable`)
	writeTree(t, root, "internal/server/deep/s.go", `package deep
var _ = os.Getenv("ARMOR_B_") + os.Getenv("ARMOR_A")`)
	writeTree(t, root, "internal/config/config.go", `package config
var _ = os.Getenv("ARMOR_CONFIG_ONLY")`)
	writeTree(t, root, "internal/docsindex/self.go", `package docsindex
var _ = os.Getenv("ARMOR_SELF_ONLY")`)
	writeTree(t, root, "internal/server/s_test.go", `package deep
var _ = os.Getenv("ARMOR_TEST_ONLY")`)
	writeTree(t, root, "internal/nested/config/x.go", `package config
var _ = os.Getenv("ARMOR_NESTED")`)

	got, err := ConsumerEnvVars(root)
	if err != nil {
		t.Fatal(err)
	}
	// config/ and docsindex/ are skipped, test files are skipped, the bare
	// prefix is skipped; a "config" directory nested deeper than internal/'s
	// direct children is a different package and stays in scope.
	want := []string{"ARMOR_A", "ARMOR_B_", "ARMOR_NESTED"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ConsumerEnvVars() = %q, want %q", got, want)
	}
}

func TestConsumerDefaults(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "cmd/restore-verifier/main.go", `package main
var readConcurrency = flag.Int("read-concurrency", parseInt(os.Getenv("ARMOR_READ_CONCURRENCY"), 16), "doc")
var mekHex = flag.String("mek", os.Getenv("ARMOR_MEK"), "doc")`)

	got, err := ConsumerDefaults(root)
	if err != nil {
		t.Fatal(err)
	}
	// The parseInt fallback form carries its default; a bare os.Getenv read
	// with no literal default is absent.
	want := []CodeDefault{{Var: "ARMOR_READ_CONCURRENCY", Kind: "int", Value: "16"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ConsumerDefaults() = %+v, want %+v", got, want)
	}
}

func TestConsumerValidationRules(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "cmd/armor/main.go", `package main
var _ = fmt.Fprintf(os.Stderr, "Error: ARMOR_FORMAT_VERSION must be an integer (2 or 3), got %q\n", s)
var _ = fmt.Errorf("unrelated message without rule words")`)

	got, err := ConsumerValidationRules(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []ValidationRule{{Var: "ARMOR_FORMAT_VERSION", Tokens: []string{"2", "3"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ConsumerValidationRules() = %+v, want %+v", got, want)
	}
}

func TestConsumerRefProblems(t *testing.T) {
	vars := []string{"ARMOR_A"}
	rows := []ConfigRefRow{
		{Line: 10, Vars: []string{"ARMOR_A"}, Required: "No", Default: "16", Desc: "effect"},
	}
	// A rule naming a variable the consumer scan does not read is attributed
	// to the consumer scan, not to internal/config.
	got := ConsumerRefProblems(vars, nil, []ValidationRule{{Var: "ARMOR_D", Tokens: []string{"7"}}}, rows)
	want := "validation rule names ARMOR_D, which the consumer scan does not read"
	if len(got) != 1 || got[0] != want {
		t.Errorf("ConsumerRefProblems() = %q, want %q", got, want)
	}
	// A drifted default is attributed the same way.
	got = ConsumerRefProblems(vars, []CodeDefault{{Var: "ARMOR_A", Kind: "int", Value: "32"}}, nil, rows)
	if len(got) != 1 || !strings.Contains(got[0], "the consumer scan declares int default") {
		t.Errorf("ConsumerRefProblems() = %q, want the consumer-attributed default mismatch", got)
	}
}

// TestConsumerReferenceParity is the repository-level guard for the consumers
// the config-side test does not see: the companion binaries (cmd/) and the
// packages outside internal/config that read ARMOR_* settings directly. Every
// variable they read needs a reference row held to the same standard as the
// config-side check — non-empty Required, Default and Description cells, a
// Default that agrees with any literal the consumer declares, and the anchors
// of any bound or accepted value the consumer enforces.
func TestConsumerReferenceParity(t *testing.T) {
	root := repoRoot(t)

	vars, err := ConsumerEnvVars(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(vars) < 20 {
		t.Fatalf("only %d consumer ARMOR_* variables found under cmd/ and internal/; the scanner is broken", len(vars))
	}
	defaults, err := ConsumerDefaults(root)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := ConsumerValidationRules(root)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := os.ReadFile(filepath.Join(root, "docs", "configuration.md"))
	if err != nil {
		t.Fatal(err)
	}
	problems := ConsumerRefProblems(vars, defaults, rules, ParseConfigRefRows(ref))
	for _, p := range problems {
		t.Error("docs/configuration.md consumer parity: " + p)
	}
}
