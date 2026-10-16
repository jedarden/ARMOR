//go:build !integration
// +build !integration

// Flag-vocabulary pins for armor-b1e3a56b: one vocabulary across the armor
// CLI and the companions — verbosity is -verbose, the decrypt envelope
// format flag is -envelope-version, and -mek-ring speaks the shared
// mekring grammar. The legacy -v and -version spellings remain deprecated
// aliases that still work and print a one-release pointer.
package main

import (
	"encoding/hex"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/crypto"
)

// captureStderr runs f with os.Stderr redirected to a pipe and returns what
// was written.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	f()
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr capture: %v", err)
	}
	return string(out)
}

// vocabularyFlagSet builds a FlagSet with the decrypt vocabulary under test.
func vocabularyFlagSet() (*flag.FlagSet, *bool, *int) {
	fs := flag.NewFlagSet("decrypt", flag.ContinueOnError)
	var verbose bool
	var version int
	fs.BoolVar(&verbose, "verbose", false, "Verbose output")
	fs.BoolVar(&verbose, "v", false, "DEPRECATED alias of -verbose")
	fs.IntVar(&version, "envelope-version", 1, "Envelope version (1, 2, or 3)")
	fs.IntVar(&version, "version", 1, "DEPRECATED alias of -envelope-version")
	return fs, &verbose, &version
}

func TestDeprecatedFlagAliasesPrintPointersAndStillWork(t *testing.T) {
	fs, verbose, version := vocabularyFlagSet()
	if err := fs.Parse([]string{"-v", "-version", "3"}); err != nil {
		t.Fatalf("parse deprecated aliases: %v", err)
	}
	got := captureStderr(t, func() {
		printDeprecatedFlagPointers(fs, map[string]string{
			"v":       "-verbose",
			"version": "-envelope-version",
		})
	})
	for _, want := range []string{
		"-v is deprecated; use -verbose instead",
		"-version is deprecated; use -envelope-version instead",
		"removed after one release",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("deprecation pointers missing %q:\n%s", want, got)
		}
	}
	if !*verbose {
		t.Errorf("-v must still set verbose during the deprecation window")
	}
	if *version != 3 {
		t.Errorf("-version must still set the envelope version; got %d, want 3", *version)
	}
}

func TestDeprecatedFlagPointersStayQuietForNewNames(t *testing.T) {
	fs, _, version := vocabularyFlagSet()
	if err := fs.Parse([]string{"-verbose", "-envelope-version", "2"}); err != nil {
		t.Fatalf("parse new names: %v", err)
	}
	if got := captureStderr(t, func() {
		printDeprecatedFlagPointers(fs, map[string]string{
			"v":       "-verbose",
			"version": "-envelope-version",
		})
	}); got != "" {
		t.Errorf("new names must not print deprecation pointers, got:\n%s", got)
	}
	if *version != 2 {
		t.Errorf("-envelope-version: got %d, want 2", *version)
	}
}

func TestParseMEKRingAcceptsTheSharedGrammar(t *testing.T) {
	// A fingerprint:key pair built from a real key so the pair agrees.
	const mekHex = "abababababababababababababababababababababababababababababababab"
	mek, err := hex.DecodeString(mekHex)
	if err != nil {
		t.Fatalf("decode test MEK: %v", err)
	}
	fp := crypto.MEKFingerprint(mek)

	entries, err := parseMEKRing(fp + ":" + mekHex)
	if err != nil {
		t.Fatalf("parseMEKRing(fingerprint:key): %v", err)
	}
	if len(entries) != 1 || entries[0].Fingerprint != fp || entries[0].MEK == nil {
		t.Fatalf("fingerprint:key entry = %+v, want one entry with key material and fingerprint %s", entries, fp)
	}
}

func TestParseMEKRingLegacyFingerprintPrintsPointer(t *testing.T) {
	const fp = "0123456789abcdef"
	var entries []crypto.RingKeyEntry
	var err error
	got := captureStderr(t, func() {
		entries, err = parseMEKRing(fp)
	})
	if err != nil {
		t.Fatalf("parseMEKRing(legacy fingerprint): %v", err)
	}
	if len(entries) != 1 || entries[0].MEK != nil || entries[0].Fingerprint != fp {
		t.Fatalf("legacy fingerprint entry = %+v, want a nil-MEK placeholder", entries)
	}
	if !strings.Contains(got, "bare fingerprint") || !strings.Contains(got, "deprecated") {
		t.Errorf("legacy fingerprint form must print the deprecation pointer, got:\n%s", got)
	}
}

func TestParseMEKRingRejectsMismatchedPairs(t *testing.T) {
	const mekHex = "abababababababababababababababababababababababababababababababab"
	_, err := parseMEKRing("0000000000000000:" + mekHex)
	if err == nil {
		t.Fatalf("mismatched fingerprint:key pair parsed without error")
	}
	if !strings.Contains(err.Error(), "does not match the key") {
		t.Errorf("mismatch error = %v, want it to name the mismatch", err)
	}
}
