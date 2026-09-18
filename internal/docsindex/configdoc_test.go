package docsindex

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigEnvVars(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("config.go", `package config
func Load() {
	_ = os.Getenv("ARMOR_BUCKET")
	_ = getEnv("ARMOR_LISTEN", "0.0.0.0:9000")
	_ = strings.HasPrefix(k, "ARMOR_MEK_")
	_ = "ARMOR_" + name + "_ACL"
}`)
	write("config_test.go", `package config
func TestX() { _ = os.Getenv("ARMOR_ONLY_IN_TESTS") }`)
	write("notes.txt", `"ARMOR_NOT_GO"`)

	got, err := ConfigEnvVars(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ARMOR_BUCKET", "ARMOR_LISTEN", "ARMOR_MEK_"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ConfigEnvVars() = %q, want %q", got, want)
	}
}

func TestUndocumentedEnvVars(t *testing.T) {
	readme := []byte("| `ARMOR_BUCKET` | Yes |\nNamed keys use `ARMOR_MEK_<NAME>`.\n")
	vars := []string{"ARMOR_BUCKET", "ARMOR_LISTEN", "ARMOR_MEK_", "ARMOR_AUTH_"}
	got := UndocumentedEnvVars(vars, readme)
	want := []string{"ARMOR_AUTH_", "ARMOR_LISTEN"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UndocumentedEnvVars() = %q, want %q", got, want)
	}
}

// TestReadmeDocumentsEveryConfigVariable is the repository-level guard: every
// ARMOR_* variable that internal/config reads must appear in README.md, so
// the configuration reference cannot silently fall behind the code.
func TestReadmeDocumentsEveryConfigVariable(t *testing.T) {
	root := repoRoot(t)
	vars, err := ConfigEnvVars(filepath.Join(root, "internal", "config"))
	if err != nil {
		t.Fatal(err)
	}
	if len(vars) == 0 {
		t.Fatal("no ARMOR_* variables found under internal/config; the scanner is broken")
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if missing := UndocumentedEnvVars(vars, readme); len(missing) != 0 {
		t.Errorf("README.md does not document these variables read by internal/config (add them to the Configuration reference): %q", missing)
	}
}
