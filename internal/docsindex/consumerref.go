package docsindex

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// This file is the consumer half of the configuration-reference parity check
// (configref.go is the internal/config half). Companion binaries (cmd/armor,
// cmd/restore-verifier, cmd/armor-fleet) and packages outside internal/config
// also read ARMOR_* settings directly — through os.Getenv rather than the
// config struct — and those reads are held to the same standard: every
// variable needs a reference row in docs/configuration.md whose Required,
// Default and Description cells are stated, whose Default agrees with any
// literal default the consumer declares, and whose text carries the anchors of
// any bound or accepted value the consumer enforces.
//
// The scan walks the shipped trees only (cmd/ and internal/). The harnesses
// under tests/ read suite-local variables (ARMOR_B2_TEST_*, ARMOR_PERF_COMMIT)
// that configure a test run, not a deployment, so they are out of scope.

var (
	// consumerRoots are the top-level trees the consumer scan walks, relative
	// to the repository root.
	consumerRoots = []string{"cmd", "internal"}
	// consumerSkipDirs are the direct children of internal/ the scan does not
	// walk: internal/config is covered by the config-side scan (configref.go),
	// and internal/docsindex is this checker itself.
	consumerSkipDirs = []string{"config", "docsindex"}
)

// forEachConsumerSource runs visit over every non-test .go file under the
// consumer roots of root, in lexical order. A missing root is skipped so unit
// tests can build fixture trees that only exercise part of the layout.
func forEachConsumerSource(root string, visit func(src []byte) error) error {
	var files []string
	for _, top := range consumerRoots {
		dir := filepath.Join(root, top)
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				// Only the packages internal/config and internal/docsindex
				// are exempt; a "config" directory nested deeper is a
				// different package and stays in scope.
				if filepath.Dir(path) == dir && top == "internal" && containsString(consumerSkipDirs, d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if d.Type().IsRegular() && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	sort.Strings(files)
	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if err := visit(src); err != nil {
			return err
		}
	}
	return nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ConsumerEnvVars returns, sorted and de-duplicated, every ARMOR_* environment
// variable name that appears as a string literal in the non-test Go sources
// the consumer scan covers — everything under cmd/ and internal/ except
// internal/config (whose variables the config-side scan owns) and this
// package. A variable both config and a consumer read appears here too; the
// two checks agree on the row it shares.
func ConsumerEnvVars(root string) ([]string, error) {
	seen := make(map[string]bool)
	err := forEachConsumerSource(root, func(src []byte) error {
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

// ConsumerDefaults extracts, sorted by variable, every default the consumer
// sources declare in a mechanically recognizable way — the same idioms
// CodeDefaults recognizes, including the parseInt(os.Getenv("ARMOR_X"), 16)
// fallback form the companion binaries use for flag defaults. Reads without a
// literal default (a bare os.Getenv) are absent; their rows are still held to
// the non-empty-cell standard.
func ConsumerDefaults(root string) ([]CodeDefault, error) {
	seen := make(map[string]CodeDefault)
	err := forEachConsumerSource(root, func(src []byte) error {
		extractDefaults(src, seen)
		return nil
	})
	if err != nil {
		return nil, err
	}
	defaults := make([]CodeDefault, 0, len(seen))
	for _, d := range seen {
		defaults = append(defaults, d)
	}
	sort.Slice(defaults, func(i, j int) bool { return defaults[i].Var < defaults[j].Var })
	return defaults, nil
}

// ConsumerValidationRules extracts, sorted by variable, the checkable
// constraints the consumer sources state in their validation errors — the same
// extraction ValidationRules applies to internal/config.
func ConsumerValidationRules(root string) ([]ValidationRule, error) {
	tokensByKey := make(map[string]map[string]bool)
	err := forEachConsumerSource(root, func(src []byte) error {
		extractRules(src, tokensByKey)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rulesFrom(tokensByKey), nil
}

// ConsumerRefProblems holds the configuration reference to the same standard
// ConfigRefProblems enforces, for the variables the consumer scan reads: a row
// per variable, non-empty cells, defaults that agree with the literals the
// consumers declare, and the anchors of the bounds and accepted values they
// enforce. Messages attribute findings to the consumer scan rather than to
// internal/config.
func ConsumerRefProblems(vars []string, defaults []CodeDefault, rules []ValidationRule, rows []ConfigRefRow) []string {
	return refProblems("the consumer scan", vars, defaults, rules, rows)
}
