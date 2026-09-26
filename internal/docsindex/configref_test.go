package docsindex

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSplitCells(t *testing.T) {
	got := splitCells("| `ARMOR_COMPRESS_RULES` | No | — | Rules `<a>|<b>=zstd`, first match wins |")
	want := []string{"`ARMOR_COMPRESS_RULES`", "No", "—", "Rules `<a>|<b>=zstd`, first match wins"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitCells() = %q, want %q", got, want)
	}
	// An edge separator adjacent to a real cell is not the edge itself.
	got = splitCells("| x |  |")
	want = []string{"x", ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitCells() = %q, want %q", got, want)
	}
}

func TestParseConfigRefRows(t *testing.T) {
	doc := []byte(`
Intro prose mentioning ` + "`ARMOR_NOT_A_ROW`" + ` in text.

| Name | Default |
|------|---------|
| ` + "`ARMOR_OTHER_TABLE`" + ` | 1 |

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| ` + "`ARMOR_A`" + ` | No | ` + "`on`" + ` | Does A |
| ` + "`ARMOR_B_<NAME>`" + `, ` + "`ARMOR_B_<NAME>_RING`" + ` | No | — | Family of B |

| Variable | Description |
|----------|-------------|
| ` + "`ARMOR_NO_DEFAULT_COL`" + ` | effect |

| Variable | Default | Description |
|----------|---------|-------------|
| ` + "`ARMOR_REORDERED`" + ` | ` + "`7`" + ` | effect |
`)
	rows := ParseConfigRefRows(doc)
	// The Name/Default table and the no-Default-column table are skipped; the
	// main table and the reordered-header table are both reference-shaped.
	if len(rows) != 3 {
		t.Fatalf("ParseConfigRefRows() = %d rows, want 3: %+v", len(rows), rows)
	}
	if rows[0].Vars[0] != "ARMOR_A" || rows[0].Default != "`on`" || rows[0].Desc != "Does A" || rows[0].Required != "No" {
		t.Errorf("rows[0] = %+v", rows[0])
	}
	// Both placeholder forms carry the same family token, de-duplicated.
	if len(rows[1].Vars) != 1 || rows[1].Vars[0] != "ARMOR_B_" {
		t.Errorf("rows[1].Vars = %q, want the single family token", rows[1].Vars)
	}
	if rows[1].Default != "—" {
		t.Errorf("rows[1].Default = %q", rows[1].Default)
	}
	// The reordered table must honor its own header, not a fixed position.
	if rows[2].Vars[0] != "ARMOR_REORDERED" || rows[2].Default != "`7`" || rows[2].Desc != "effect" {
		t.Errorf("rows[2] = %+v, want the reordered header honored", rows[2])
	}
}

func TestCodeDefaults(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("config.go", `package config
var _ = getEnv("ARMOR_LISTEN", "0.0.0.0:9000")
var _ = getEnvInt("ARMOR_CACHE_TTL", 300)
var _ = os.Getenv("ARMOR_CANARY_DISABLED") == "true"
var _ = os.Getenv("ARMOR_COMPUTED")
var _ = getEnv("ARMOR_DYNAMIC", someVar)
presignStr := os.Getenv("ARMOR_PRESIGN_ENABLED")
var _ = presignStr == "true" || presignStr == "1"
unusedStr := os.Getenv("ARMOR_ASSIGNED_NOT_COMPARED")
var _ = unusedStr == "yes"
`)
	// Tests must not contribute defaults.
	write("config_test.go", `package config
var _ = getEnv("ARMOR_ONLY_IN_TESTS", "x")
`)
	got, err := CodeDefaults(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []CodeDefault{
		{Var: "ARMOR_CACHE_TTL", Kind: "int", Value: "300"},
		{Var: "ARMOR_CANARY_DISABLED", Kind: "bool", Value: "false"},
		{Var: "ARMOR_LISTEN", Kind: "string", Value: "0.0.0.0:9000"},
		{Var: "ARMOR_PRESIGN_ENABLED", Kind: "bool", Value: "false"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CodeDefaults() = %+v, want %+v", got, want)
	}
}

func TestRuleTokens(t *testing.T) {
	cases := []struct {
		rule string
		want []string
	}{
		{"ARMOR_BLOCK_SIZE must be a power of 2 >= 4096", []string{"4096"}},
		{"ARMOR_BACKEND must be 'b2' or 'filesystem'", []string{"b2", "filesystem"}},
		{"ARMOR_MEK must be 32 bytes (64 hex chars)", []string{"32", "64"}},
		{"ARMOR_FORMAT_VERSION must be an integer (2 or 3)", []string{"2", "3"}},
		{"ARMOR_READ_CONCURRENCY must be at least 1", nil},
		{"ARMOR_MANIFEST_PREFIX must be a relative path", nil},
	}
	for _, c := range cases {
		if got := ruleTokens(c.rule); !reflect.DeepEqual(got, c.want) {
			t.Errorf("ruleTokens(%q) = %q, want %q", c.rule, got, c.want)
		}
	}
}

func TestValidationRules(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("config.go", `package config
var _ = fmt.Errorf("ARMOR_BLOCK_SIZE must be a power of 2 >= 4096")
var _ = fmt.Errorf("ARMOR_BACKEND must be 'b2' or 'filesystem', got '%s'", v)
var _ = fmt.Errorf("ARMOR_MEK must be hex-encoded: %w", err)             // no anchor: dropped
var _ = fmt.Errorf("ARMOR_MEK_%s must be 32 bytes (64 hex chars), got %d bytes", n, n)
var _ = fmt.Errorf("unrelated message without rule words")
`)
	got, err := ValidationRules(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []ValidationRule{
		{Var: "ARMOR_BACKEND", Tokens: []string{"b2", "filesystem"}},
		{Var: "ARMOR_BLOCK_SIZE", Tokens: []string{"4096"}},
		{Var: "ARMOR_MEK_", Tokens: []string{"32", "64"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ValidationRules() = %+v, want %+v", got, want)
	}
}

func TestConfigRefProblems(t *testing.T) {
	vars := []string{"ARMOR_A", "ARMOR_B_", "ARMOR_C"}
	defaults := []CodeDefault{{Var: "ARMOR_A", Kind: "int", Value: "16"}}
	rules := []ValidationRule{
		{Var: "ARMOR_A", Tokens: []string{"16"}}, // satisfied by the line-10 row
	}
	rows := []ConfigRefRow{
		{Line: 10, Vars: []string{"ARMOR_A"}, Required: "No", Default: "16", Desc: "effect, at least 16"},
		{Line: 11, Vars: []string{"ARMOR_B_"}, Required: "No", Default: "—", Desc: "family"},
		{Line: 12, Vars: []string{"ARMOR_UNSCANNED"}, Required: "No", Default: "—", Desc: "read elsewhere"}, // exempt
		{Line: 13, Vars: []string{"ARMOR_C"}, Required: "", Default: "—", Desc: "effect"},                   // empty Required
	}
	got := ConfigRefProblems(vars, defaults, rules, rows)
	// ARMOR_C has a row (line 13), so the only problem is its empty Required
	// cell; the row claiming ARMOR_UNSCANNED is exempt (read elsewhere).
	if len(got) != 1 {
		t.Fatalf("ConfigRefProblems() = %q, want exactly the empty-Required-cell problem", got)
	}
	if !strings.Contains(got[0], "line 13") || !strings.Contains(got[0], "Required") {
		t.Errorf("ConfigRefProblems() = %q, want the line-13 empty Required cell", got)
	}

	// Every problem class fires when nothing is documented.
	unreadRule := ValidationRule{Var: "ARMOR_D", Tokens: []string{"7"}} // not read by the code
	none := ConfigRefProblems(vars, defaults, []ValidationRule{rules[0], unreadRule}, nil)
	for _, wantProblem := range []string{
		"configuration reference has no table row for ARMOR_A",
		"configuration reference has no table row for ARMOR_B_",
		"configuration reference has no table row for ARMOR_C",
		"validation rule names ARMOR_D, which internal/config does not read",
	} {
		found := false
		for _, p := range none {
			if p == wantProblem {
				found = true
			}
		}
		if !found {
			t.Errorf("ConfigRefProblems() = %q, want %q among them", none, wantProblem)
		}
	}

	// A default that drifted, and a rule the row does not state.
	drift := ConfigRefProblems([]string{"ARMOR_A"},
		[]CodeDefault{{Var: "ARMOR_A", Kind: "int", Value: "32"}},
		[]ValidationRule{{Var: "ARMOR_A", Tokens: []string{"32"}}},
		[]ConfigRefRow{{Line: 10, Vars: []string{"ARMOR_A"}, Required: "No", Default: "16", Desc: "effect"}},
	)
	if len(drift) != 2 {
		t.Fatalf("ConfigRefProblems() = %q, want the default mismatch and the missing rule anchor", drift)
	}
	if !strings.Contains(drift[0], "does not state the validation rule") || !strings.Contains(drift[0], "missing 32") {
		t.Errorf("drift[0] = %q, want the missing rule anchor 32", drift[0])
	}
	if !strings.Contains(drift[1], `"32"`) {
		t.Errorf("drift[1] = %q, want the default mismatch on line 10", drift[1])
	}

	// A non-family variable documented in two rows is reported.
	twice := ConfigRefProblems([]string{"ARMOR_A"}, nil, nil, []ConfigRefRow{
		{Line: 10, Vars: []string{"ARMOR_A"}, Required: "No", Default: "—", Desc: "one"},
		{Line: 11, Vars: []string{"ARMOR_A"}, Required: "No", Default: "—", Desc: "two"},
	})
	if len(twice) != 1 || !strings.Contains(twice[0], "in 2 rows (lines 10, 11)") {
		t.Errorf("ConfigRefProblems() = %q, want the duplicate-row problem", twice)
	}

	// A family variable spread over placeholder rows is expected, not a problem.
	family := ConfigRefProblems([]string{"ARMOR_B_"}, nil, nil, []ConfigRefRow{
		{Line: 10, Vars: []string{"ARMOR_B_"}, Required: "No", Default: "—", Desc: "family"},
		{Line: 11, Vars: []string{"ARMOR_B_"}, Required: "No", Default: "—", Desc: "ring"},
	})
	if len(family) != 0 {
		t.Errorf("ConfigRefProblems() = %q, want none: a family may span rows", family)
	}
}

// TestConfigReferenceParity is the repository-level guard: the configuration
// reference documents every ARMOR_* variable internal/config reads as a table
// row, each row's default equals the default the code declares, and each
// variable's row states the anchors of the validation rules the code
// enforces (bounds, accepted values). Presence alone is not enough.
func TestConfigReferenceParity(t *testing.T) {
	root := repoRoot(t)
	configDir := filepath.Join(root, "internal", "config")

	vars, err := ConfigEnvVars(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(vars) == 0 {
		t.Fatal("no ARMOR_* variables found under internal/config; the scanner is broken")
	}
	defaults, err := CodeDefaults(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(defaults) < 10 {
		t.Fatalf("CodeDefaults found only %d defaults; the extractor is broken", len(defaults))
	}
	rules, err := ValidationRules(configDir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := os.ReadFile(filepath.Join(root, "docs", "configuration.md"))
	if err != nil {
		t.Fatal(err)
	}
	problems := ConfigRefProblems(vars, defaults, rules, ParseConfigRefRows(ref))
	for _, p := range problems {
		t.Error("docs/configuration.md parity: " + p)
	}
}
