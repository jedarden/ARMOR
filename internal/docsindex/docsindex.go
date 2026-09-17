// Package docsindex validates that docs/README.md stays a complete and
// accurate index of the documentation tree.
//
// The index promises that every file under docs/ (excluding archive/ and the
// index itself) is linked exactly once and that every link resolves. This
// package turns that promise into a checkable report so the index cannot
// silently drift as documents are added.
package docsindex

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	// indexName is the index file itself, relative to docsDir. It does not
	// link itself.
	indexName = "README.md"
	// archivePrefix holds historical documents that are deliberately omitted
	// from the index; anything under it is exempt from coverage.
	archivePrefix = "archive/"
)

// Files walks docsDir and returns every file the index must link, as
// slash-separated paths relative to docsDir: all regular files except the
// index itself and anything under archive/. The result is sorted.
func Files(docsDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(docsDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == indexName || strings.HasPrefix(rel, archivePrefix) {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// Validate checks the documentation index against a candidate file list and
// returns every problem found, sorted. docsDir is the directory the index
// lives in and the base all relative links resolve against; files are the
// documents the index must cover, as slash-separated paths relative to
// docsDir (see Files); index is the raw index content.
//
// Three properties are enforced:
//
//   - every relative link in the index resolves to something on disk;
//   - every candidate file is linked from the index exactly once;
//   - nothing the index links was left out of the candidate list (a link to
//     a file the caller did not offer, e.g. an uncommitted stray, is
//     reported rather than silently ignored).
func Validate(docsDir string, files []string, index []byte) []string {
	var problems []string
	counts := make(map[string]int)
	for _, raw := range extractLinks(index) {
		if isExternal(raw) {
			continue
		}
		target, _, _ := strings.Cut(raw, "#")
		if target == "" {
			continue
		}
		resolved := filepath.Clean(filepath.Join(docsDir, filepath.FromSlash(target)))
		if _, err := os.Stat(resolved); err != nil {
			problems = append(problems, "broken link: ["+raw+"] does not resolve")
			continue
		}
		rel, relErr := filepath.Rel(docsDir, resolved)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue // outside the docs tree: resolution checked, coverage not tracked
		}
		rel = filepath.ToSlash(rel)
		if rel == indexName || isArchived(rel) {
			continue
		}
		counts[rel]++
	}

	for _, f := range files {
		n := counts[f]
		delete(counts, f)
		switch {
		case n == 0:
			problems = append(problems, "not linked from index: "+f)
		case n > 1:
			problems = append(problems, fmt.Sprintf("linked %d times from index: %s", n, f))
		}
	}
	for rel := range counts {
		problems = append(problems, "linked from index but not in candidate files: "+rel)
	}

	sort.Strings(problems)
	return problems
}

// isArchived reports whether an index-relative path is inside (or is) the
// archive directory, which the index deliberately does not cover.
func isArchived(rel string) bool {
	return rel == strings.TrimSuffix(archivePrefix, "/") || strings.HasPrefix(rel, archivePrefix)
}

// isExternal reports whether a link target does not point at a repository
// file: absolute URLs, mailto, and pure in-page fragments.
func isExternal(target string) bool {
	return strings.Contains(target, "://") ||
		strings.HasPrefix(target, "mailto:") ||
		strings.HasPrefix(target, "#")
}

// extractLinks returns every link target in a markdown document, in document
// order: inline links, reference definitions, and angle-bracket targets.
// Fenced code blocks and inline code spans are ignored — code samples that
// merely look like links must not satisfy coverage.
func extractLinks(index []byte) []string {
	var targets []string
	inFence := false
	for _, line := range strings.Split(string(index), "\n") {
		if fence := strings.TrimLeft(line, " "); strings.HasPrefix(fence, "```") || strings.HasPrefix(fence, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		line = inlineCode.ReplaceAllStringFunc(line, func(m string) string {
			return strings.Repeat(" ", len(m))
		})
		for _, m := range inlineLink.FindAllStringSubmatch(line, -1) {
			if t := normalizeTarget(m[1]); t != "" {
				targets = append(targets, t)
			}
		}
		if m := referenceDef.FindStringSubmatch(line); m != nil {
			if t := normalizeTarget(m[1]); t != "" {
				targets = append(targets, t)
			}
		}
	}
	return targets
}

// normalizeTarget parses a raw markdown link destination: angle-bracketed
// destinations may contain spaces, otherwise anything after whitespace is a
// link title.
func normalizeTarget(dest string) string {
	dest = strings.TrimSpace(dest)
	if strings.HasPrefix(dest, "<") {
		if i := strings.IndexByte(dest, '>'); i >= 0 {
			return dest[1:i]
		}
	}
	if i := strings.IndexAny(dest, " \t"); i >= 0 {
		dest = dest[:i] // drop a link title
	}
	return strings.TrimSuffix(strings.TrimPrefix(dest, "<"), ">")
}

var (
	// inlineLink matches [text](dest "title"), capturing the whole
	// destination-plus-title blob for normalizeTarget to parse.
	inlineLink = regexp.MustCompile(`\[[^\]\n]*\]\(\s*([^)\n]*?)\s*\)`)
	// referenceDef matches the definition form [id]: dest on its own line.
	referenceDef = regexp.MustCompile(`^\s*\[[^\]\n]+\]:\s*<?([^\s>]*)>?`)
	// inlineCode matches backtick spans, whose contents are not markup.
	inlineCode = regexp.MustCompile("`[^`\n]*`")
)
