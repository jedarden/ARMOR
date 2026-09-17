package docsindex

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestExtractLinks(t *testing.T) {
	doc := []byte(`# Index

- [A](a.md) and [B](sub/b.md "the title")
- [angled](<weird name.md>)
- [fragment only](#section) and [external](https://example.com) and [mail](mailto:x@y.z)
- [with anchor](a.md#notes)
- [ref][reflink]

[reflink]: ref.md
` + "```markdown\n" + `[fake](fake.md)` + "```\n" +
		"Inline `[fake2](fake2.md)` is not a link.\n")

	got := extractLinks(doc)
	want := []string{"a.md", "sub/b.md", "weird name.md", "#section", "https://example.com", "mailto:x@y.z", "a.md#notes", "ref.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractLinks() = %q, want %q", got, want)
	}
}

func TestFilesWalk(t *testing.T) {
	docs := t.TempDir()
	write := func(rel string) {
		path := filepath.Join(docs, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md")
	write("guide.md")
	write("archive/old.md")
	write("sub/deep.md")

	files, err := Files(docs)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"guide.md", "sub/deep.md"}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("Files() = %q, want %q (index and archive/ must be excluded)", files, want)
	}
}

func TestValidate(t *testing.T) {
	build := func(t *testing.T, index string) string {
		t.Helper()
		docs := t.TempDir()
		for _, f := range []string{"README.md", "a.md", "b.md", "archive/old.md"} {
			path := filepath.Join(docs, f)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(docs, indexName), []byte(index), 0o644); err != nil {
			t.Fatal(err)
		}
		// A sibling tree the index may link into, outside docs/.
		if err := os.MkdirAll(filepath.Join(docs, "..", "tests"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(docs, "..", "tests", "README.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return docs
	}

	t.Run("clean index", func(t *testing.T) {
		docs := build(t, "# Index\n\n- [A](a.md)\n- [B](b.md#sec)\n- [Archive](archive/)\n- [Tests](../tests/README.md)\n")
		files, err := Files(docs)
		if err != nil {
			t.Fatal(err)
		}
		index, err := os.ReadFile(filepath.Join(docs, "README.md"))
		if err != nil {
			t.Fatal(err)
		}
		if problems := Validate(docs, files, index); len(problems) != 0 {
			t.Errorf("Validate() = %q, want none", problems)
		}
	})

	t.Run("missing and duplicate coverage", func(t *testing.T) {
		docs := build(t, "# Index\n\n- [A](a.md)\n- [A again](a.md)\n")
		files, err := Files(docs)
		if err != nil {
			t.Fatal(err)
		}
		index, err := os.ReadFile(filepath.Join(docs, "README.md"))
		if err != nil {
			t.Fatal(err)
		}
		problems := Validate(docs, files, index)
		want := []string{
			"linked 2 times from index: a.md",
			"not linked from index: b.md",
		}
		if !reflect.DeepEqual(problems, want) {
			t.Errorf("Validate() = %q, want %q", problems, want)
		}
	})

	t.Run("broken and outside links", func(t *testing.T) {
		docs := build(t, "# Index\n\n- [A](a.md)\n- [B](b.md)\n- [gone](missing.md)\n- [outside](../nowhere.md)\n")
		files, err := Files(docs)
		if err != nil {
			t.Fatal(err)
		}
		index, err := os.ReadFile(filepath.Join(docs, "README.md"))
		if err != nil {
			t.Fatal(err)
		}
		problems := Validate(docs, files, index)
		if len(problems) != 2 ||
			!strings.Contains(strings.Join(problems, "\n"), "missing.md") ||
			!strings.Contains(strings.Join(problems, "\n"), "nowhere.md") {
			t.Errorf("Validate() = %q, want exactly the two broken links", problems)
		}
	})

	t.Run("linked stray reported", func(t *testing.T) {
		docs := build(t, "# Index\n\n- [A](a.md)\n- [B](b.md)\n")
		// Present on disk but deliberately not offered as a candidate —
		// the shape of an uncommitted stray in a shared working tree.
		stray := filepath.Join(docs, "stray.md")
		if err := os.WriteFile(stray, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		index := []byte("# Index\n\n- [A](a.md)\n- [B](b.md)\n- [stray](stray.md)\n")
		files := []string{"a.md", "b.md"}
		problems := Validate(docs, files, index)
		want := []string{"linked from index but not in candidate files: stray.md"}
		if !reflect.DeepEqual(problems, want) {
			t.Errorf("Validate() = %q, want %q", problems, want)
		}
	})
}

func TestDocsTreeIsIndexed(t *testing.T) {
	root := repoRoot(t)
	docsDir := filepath.Join(root, "docs")

	files, err := Files(docsDir)
	if err != nil {
		t.Fatal(err)
	}
	// On a shared working tree, skip files that are not committed yet: another
	// worker's in-flight document is not part of the durable set this index
	// covers. On a clean extraction (which only contains committed files) the
	// filter either matches exactly or is unavailable and unnecessary.
	if tracked, ok := trackedFiles(root); ok {
		filtered := files[:0]
		for _, f := range files {
			if tracked["docs/"+f] {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	index, err := os.ReadFile(filepath.Join(docsDir, indexName))
	if err != nil {
		t.Fatal(err)
	}
	if problems := Validate(docsDir, files, index); len(problems) > 0 {
		t.Errorf("docs/README.md is not a consistent index of docs/:\n  %s",
			strings.Join(problems, "\n  "))
	}
}

// repoRoot walks up from this file to the directory containing go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", filepath.Dir(thisFile))
		}
		dir = parent
	}
}

// trackedFiles returns the files git tracks in this checkout. The bool result
// is false when git cannot answer (no binary, or a clean extraction without a
// repository), in which case the caller must fall back to the raw walk.
func trackedFiles(root string) (map[string]bool, bool) {
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--", "docs").Output()
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
