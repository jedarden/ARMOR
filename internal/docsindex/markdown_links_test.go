package docsindex

import (
	"bytes"
	"fmt"
	"html"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// TestMarkdownLinksResolve guards links in the README and documentation tree,
// including fragments in another document. The index test checks file coverage,
// but a link to an existing file can still point at a heading that was renamed.
func TestMarkdownLinksResolve(t *testing.T) {
	root := repoRoot(t)
	sources := []string{"README.md"}
	err := filepath.WalkDir(filepath.Join(root, "docs"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() || filepath.Ext(path) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		// Historical archive pages are frozen and intentionally omitted from
		// the active documentation index as well.
		if strings.HasPrefix(rel, "docs/archive/") {
			return nil
		}
		sources = append(sources, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Ignore another worker's untracked draft in the shared checkout.
	if tracked, ok := trackedMarkdownFiles(root); ok {
		filtered := sources[:0]
		for _, source := range sources {
			if tracked[source] {
				filtered = append(filtered, source)
			}
		}
		sources = filtered
	}
	if problems := markdownLinkProblems(root, sources); len(problems) > 0 {
		t.Errorf("local Markdown links do not resolve:\n  %s", strings.Join(problems, "\n  "))
	}
}

func trackedMarkdownFiles(root string) (map[string]bool, bool) {
	// A nested archive may discover the parent checkout; only its own git
	// index can describe the files being tested.
	top, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil || filepath.Clean(strings.TrimSpace(string(top))) != filepath.Clean(root) {
		return nil, false
	}
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--", "README.md", "docs").Output()
	if err != nil {
		return nil, false
	}
	tracked := make(map[string]bool)
	for _, name := range bytes.Split(out, []byte{0}) {
		if len(name) > 0 {
			tracked[string(name)] = true
		}
	}
	return tracked, true
}

type markdownLink struct {
	target string
	line   int
}

func markdownLinksWithLines(content []byte) []markdownLink {
	var links []markdownLink
	inFence := false
	var fenceChar byte
	var fenceWidth int
	for i, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		if indent <= 3 && len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			width := 0
			for width < len(trimmed) && trimmed[width] == trimmed[0] {
				width++
			}
			if !inFence && width >= 3 {
				inFence, fenceChar, fenceWidth = true, trimmed[0], width
				continue
			}
			if inFence && trimmed[0] == fenceChar && width >= fenceWidth && strings.TrimSpace(trimmed[width:]) == "" {
				inFence = false
				continue
			}
		}
		if inFence || indent >= 4 {
			continue
		}
		for _, target := range extractLinks([]byte(line)) {
			links = append(links, markdownLink{target: target, line: i + 1})
		}
	}
	return links
}

func markdownLinkProblems(root string, sources []string) []string {
	var problems []string
	anchors := make(map[string]map[string]bool)
	for _, source := range sources {
		path := filepath.Join(root, filepath.FromSlash(source))
		raw, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", source, err))
			continue
		}
		for _, link := range markdownLinksWithLines(raw) {
			target := link.target
			location := fmt.Sprintf("%s:%d", source, link.line)
			if externalMarkdownLink(target) {
				continue
			}
			file, fragment, _ := strings.Cut(target, "#")
			file, err = url.PathUnescape(file)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: invalid link %q: %v", location, target, err))
				continue
			}
			resolved := path
			if file != "" {
				if strings.HasPrefix(file, "/") {
					// Markdown root paths are repository-relative on the forge.
					resolved = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(file, "/")))
				} else {
					resolved = filepath.Join(filepath.Dir(path), filepath.FromSlash(file))
				}
				resolved = filepath.Clean(resolved)
			}
			rel, err := filepath.Rel(root, resolved)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				problems = append(problems, fmt.Sprintf("%s: link %q escapes the repository", location, target))
				continue
			}
			info, err := os.Stat(resolved)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: link %q does not resolve: %v", location, target, err))
				continue
			}
			if fragment == "" || info.IsDir() || !strings.EqualFold(filepath.Ext(resolved), ".md") {
				continue
			}
			fragment, err = url.PathUnescape(fragment)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: invalid fragment in %q: %v", location, target, err))
				continue
			}
			if _, ok := anchors[resolved]; !ok {
				content, readErr := os.ReadFile(resolved)
				if readErr != nil {
					problems = append(problems, fmt.Sprintf("%s: reading %q: %v", location, target, readErr))
					continue
				}
				anchors[resolved] = markdownAnchors(content)
			}
			if !anchors[resolved][fragment] {
				problems = append(problems, fmt.Sprintf("%s: link %q has no matching anchor", location, target))
			}
		}
	}
	sort.Strings(problems)
	return problems
}

func externalMarkdownLink(target string) bool {
	if strings.HasPrefix(target, "//") {
		return true
	}
	// URI schemes include http:, mailto:, and data:. A colon elsewhere in a
	// relative path does not make it external.
	return uriScheme.MatchString(target)
}

var (
	uriScheme         = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
	heading           = regexp.MustCompile(`^ {0,3}#{1,6}(?:[ \t]+|$)(.*)$`)
	setext            = regexp.MustCompile(`^ {0,3}(?:=+|-+)[ \t]*$`)
	inlineHeadingLink = regexp.MustCompile(`\[([^]]+)\]\([^)]*\)`)
	inlineHeadingHTML = regexp.MustCompile(`<[^>]*>`)
	explicitAnchor    = regexp.MustCompile(`(?i)<a\s+[^>]*(?:id|name)=["']([^"']+)["'][^>]*>`)
)

// markdownAnchors approximates GitHub's heading IDs: punctuation is removed,
// whitespace becomes hyphens, and repeated headings receive -1, -2, etc.
func markdownAnchors(content []byte) map[string]bool {
	ids := make(map[string]bool)
	seen := make(map[string]int)
	var previous string
	inFence := false
	var fenceChar byte
	var fenceWidth int
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		if indent <= 3 && len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			width := 0
			for width < len(trimmed) && trimmed[width] == trimmed[0] {
				width++
			}
			if !inFence && width >= 3 {
				inFence, fenceChar, fenceWidth = true, trimmed[0], width
				previous = ""
				continue
			}
			if inFence && trimmed[0] == fenceChar && width >= fenceWidth && strings.TrimSpace(trimmed[width:]) == "" {
				inFence = false
				continue
			}
		}
		if inFence || indent >= 4 {
			previous = ""
			continue
		}
		for _, match := range explicitAnchor.FindAllStringSubmatch(line, -1) {
			ids[html.UnescapeString(match[1])] = true
		}
		var title string
		if match := heading.FindStringSubmatch(line); match != nil {
			title = strings.TrimSpace(match[1])
			title = strings.TrimSpace(strings.TrimRight(title, "# "))
		} else if previous != "" && setext.MatchString(line) {
			title = previous
		}
		if title != "" {
			base := githubHeadingSlug(title)
			id := base
			if n := seen[base]; n > 0 {
				id = fmt.Sprintf("%s-%d", base, n)
			}
			ids[id] = true
			seen[base]++
		}
		previous = strings.TrimSpace(line)
	}
	return ids
}

func githubHeadingSlug(title string) string {
	title = html.UnescapeString(title)
	title = inlineHeadingLink.ReplaceAllString(title, "$1")
	title = inlineHeadingHTML.ReplaceAllString(title, "")
	var slug strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsSpace(r):
			slug.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			slug.WriteRune(r)
		}
	}
	return slug.String()
}

func TestMarkdownAnchors(t *testing.T) {
	doc := []byte("# Required configuration\n## Repeat\n## Repeat\n## `armor serve` + escrow\n### Backend truth — the manifest\n<a id=\"manual-id\"></a>\n```md\n# Fake\n```\n")
	ids := markdownAnchors(doc)
	for _, id := range []string{"required-configuration", "repeat", "repeat-1", "armor-serve--escrow", "backend-truth--the-manifest", "manual-id"} {
		if !ids[id] {
			t.Errorf("missing anchor %q", id)
		}
	}
	if ids["fake"] {
		t.Error("fenced code created an anchor")
	}
}

func TestMarkdownLinkProblems(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"README.md":     "# Required configuration\n",
		"docs/guide.md": "[good](../README.md#required-configuration) [bad](../README.md#missing) [local](#guide) [gone](absent.md) [web](https://example.org/#missing)\n# Guide\n```md\n[example](also-absent.md)\n```\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	problems := markdownLinkProblems(root, []string{"docs/guide.md"})
	if len(problems) != 2 ||
		!strings.Contains(strings.Join(problems, "\n"), "docs/guide.md:1: link \"../README.md#missing\"") ||
		!strings.Contains(strings.Join(problems, "\n"), "docs/guide.md:1: link \"absent.md\"") {
		t.Fatalf("markdownLinkProblems() = %q, want the missing file and anchor with source lines", problems)
	}
}
