package docsindex

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// This file is the inverse of the consumer/config parity checks: those two
// catch a variable the code reads with no reference row (configref.go,
// consumerref.go); this one catches a doc claiming ARMOR itself requires a
// variable that nothing reads at all — the "phantom" failure mode
// docs/connection-guide.md had for ARMOR_ENDPOINT/ACCESS_KEY/SECRET_KEY/
// REGION, convention names for the connecting client that this guide's own
// example scripts read, presented under a "Required Environment Variables"
// heading as if ARMOR's server read them too.

var (
	// mdHeadingLine matches a markdown heading line, capturing its '#' level.
	mdHeadingLine = regexp.MustCompile(`^(#{1,6})[ \t]`)
	// requiredEnvHeadingText matches heading text claiming the variables
	// below it are required of ARMOR itself.
	requiredEnvHeadingText = regexp.MustCompile(`(?i)Required Environment Variables`)
	// docExportVar matches an `export ARMOR_X=` line, the convention docs use
	// to show a variable's name in a shell block.
	docExportVar = regexp.MustCompile(`export\s+(ARMOR_[A-Z0-9_]+)=`)
)

// RequiredEnvVarClaims returns, sorted and de-duplicated, every ARMOR_*
// variable a doc exports under a heading whose text matches "Required
// Environment Variables" (case-insensitive) — a claim that ARMOR itself
// needs it set. A variable exported in an unrelated example elsewhere in the
// same doc (e.g. a Kubernetes or troubleshooting snippet re-using the same
// shell variable name) makes no such claim and is not returned; only the
// text inside the claiming heading's own section — up to the next heading of
// the same or shallower level — is scanned.
func RequiredEnvVarClaims(doc []byte) []string {
	lines := strings.Split(string(doc), "\n")
	seen := make(map[string]bool)
	inSection := false
	sectionLevel := 0
	for _, line := range lines {
		if m := mdHeadingLine.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			if inSection && level <= sectionLevel {
				inSection = false
			}
			if requiredEnvHeadingText.MatchString(line) {
				inSection = true
				sectionLevel = level
			}
			continue
		}
		if !inSection {
			continue
		}
		for _, m := range docExportVar.FindAllStringSubmatch(line, -1) {
			seen[m[1]] = true
		}
	}
	claims := make([]string, 0, len(seen))
	for v := range seen {
		claims = append(claims, v)
	}
	sort.Strings(claims)
	return claims
}

// PhantomEnvVars returns, sorted, every name in claims that known does not
// cover: a plain name must appear exactly, a family prefix (known entry
// ending in "_") covers any claim it prefixes.
func PhantomEnvVars(claims, known []string) []string {
	var phantoms []string
	for _, c := range claims {
		covered := false
		for _, k := range known {
			if c == k || (strings.HasSuffix(k, "_") && strings.HasPrefix(c, k)) {
				covered = true
				break
			}
		}
		if !covered {
			phantoms = append(phantoms, c)
		}
	}
	sort.Strings(phantoms)
	return phantoms
}

// forEachDoc runs visit over every Markdown file under docsDir except
// anything under archive/, in lexical order.
func forEachDoc(docsDir string, visit func(path string, body []byte) error) error {
	var files []string
	err := filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(docsDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == archivePrefix[:len(archivePrefix)-1] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		body, readErr := os.ReadFile(f)
		if readErr != nil {
			return readErr
		}
		if visitErr := visit(f, body); visitErr != nil {
			return visitErr
		}
	}
	return nil
}
