package docsindex

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// envVarLiteral matches an ARMOR_* environment variable name used as a Go
// string literal, e.g. os.Getenv("ARMOR_BUCKET") or getEnv("ARMOR_LISTEN", ...).
// A trailing underscore ("ARMOR_MEK_") marks a family prefix whose members
// are built at runtime (ARMOR_MEK_<NAME>).
var envVarLiteral = regexp.MustCompile(`"(ARMOR_[A-Z0-9_]*)"`)

// ConfigEnvVars returns, sorted and de-duplicated, every ARMOR_* environment
// variable name that appears as a string literal in the non-test Go sources
// of configDir. It is the set the README configuration reference must cover.
func ConfigEnvVars(configDir string) ([]string, error) {
	seen := make(map[string]bool)
	err := filepath.WalkDir(configDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != configDir {
				return filepath.SkipDir // config is a flat package
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, m := range envVarLiteral.FindAllStringSubmatch(string(src), -1) {
			if m[1] == "ARMOR_" {
				continue // a bare prefix used for string building, not a variable
			}
			seen[m[1]] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	vars := make([]string, 0, len(seen))
	for v := range seen {
		vars = append(vars, v)
	}
	sort.Strings(vars)
	return vars, nil
}

// UndocumentedEnvVars returns every name in vars that readme does not
// mention, sorted. A plain name counts as documented when it appears anywhere
// in the README. A family prefix (name ending in "_") counts as documented
// when the README shows its placeholder form, e.g. "ARMOR_MEK_<NAME>" for
// "ARMOR_MEK_".
func UndocumentedEnvVars(vars []string, readme []byte) []string {
	text := string(readme)
	var missing []string
	for _, v := range vars {
		want := v
		if strings.HasSuffix(v, "_") {
			want = v + "<NAME>"
		}
		if !strings.Contains(text, want) {
			missing = append(missing, v)
		}
	}
	sort.Strings(missing)
	return missing
}
