package docsindex

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRequiredEnvVarClaims(t *testing.T) {
	doc := []byte(`# Guide

## Environment Setup

### Required Environment Variables

` + "```bash" + `
export ARMOR_ENDPOINT=http://localhost:9000
export ARMOR_ACCESS_KEY=key
` + "```" + `

### Next Section

` + "```bash" + `
export ARMOR_OUTSIDE=not-a-claim
` + "```" + `
`)
	got := RequiredEnvVarClaims(doc)
	want := []string{"ARMOR_ACCESS_KEY", "ARMOR_ENDPOINT"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RequiredEnvVarClaims() = %q, want %q", got, want)
	}
}

func TestRequiredEnvVarClaimsSectionEndsAtShallowerHeading(t *testing.T) {
	doc := []byte(`## Required Environment Variables

export ARMOR_A=1

## Sibling

export ARMOR_B=1
`)
	got := RequiredEnvVarClaims(doc)
	want := []string{"ARMOR_A"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RequiredEnvVarClaims() = %q, want %q", got, want)
	}
}

func TestPhantomEnvVars(t *testing.T) {
	claims := []string{"ARMOR_BUCKET", "ARMOR_ENDPOINT", "ARMOR_MEK_PROD"}
	known := []string{"ARMOR_BUCKET", "ARMOR_MEK_"}
	got := PhantomEnvVars(claims, known)
	want := []string{"ARMOR_ENDPOINT"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PhantomEnvVars() = %q, want %q", got, want)
	}
}

func TestForEachDoc(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.md", "a")
	write("sub/b.md", "b")
	write("archive/c.md", "c")
	write("notes.txt", "not markdown")

	var seen []string
	err := forEachDoc(dir, func(path string, body []byte) error {
		rel, _ := filepath.Rel(dir, path)
		seen = append(seen, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.md", "sub/b.md"}
	if !reflect.DeepEqual(seen, want) {
		t.Errorf("forEachDoc() visited %q, want %q", seen, want)
	}
}

// checkNoPhantomRequiredEnvVars extends TestConsumerReferenceParity with the
// reverse check: a doc claiming, under a "Required Environment
// Variables" heading, that ARMOR itself needs a variable nothing reads at
// all (docs/connection-guide.md made this claim for ARMOR_ENDPOINT,
// ARMOR_ACCESS_KEY, ARMOR_SECRET_KEY and ARMOR_REGION, which are only the
// connecting client's own convention names, read by that guide's own
// example scripts, not by ARMOR).
func checkNoPhantomRequiredEnvVars(t *testing.T, root string, consumerVars []string) {
	t.Helper()
	configVars, err := ConfigEnvVars(filepath.Join(root, "internal", "config"))
	if err != nil {
		t.Fatal(err)
	}
	known := append(append([]string{}, configVars...), consumerVars...)

	var phantoms []string
	err = forEachDoc(filepath.Join(root, "docs"), func(path string, body []byte) error {
		claims := RequiredEnvVarClaims(body)
		for _, p := range PhantomEnvVars(claims, known) {
			phantoms = append(phantoms, path+": "+p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(phantoms) != 0 {
		t.Errorf("docs claim ARMOR requires these variables under a \"Required Environment Variables\" heading, but no code reads them (reword the doc to say they are the connecting client's own convention, or wire the variable into ARMOR if it should read it): %q", phantoms)
	}
}
