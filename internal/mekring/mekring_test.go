package mekring

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/crypto"
)

// A valid 32-byte MEK and its crypto.MEKFingerprint, computed here rather
// than hardcoded so the pair can never drift from the fingerprint contract
// the grammar pins.
var (
	testMEK   = strings.Repeat("ab", 32)
	testMEK2  = strings.Repeat("cd", 32)
	testFP    = crypto.MEKFingerprint(mustHex(testMEK))
	testFP2   = crypto.MEKFingerprint(mustHex(testMEK2))
	badFP     = "0000000000000000"
	notHexKey = "zz" + strings.Repeat("ab", 31)
)

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func TestParseFingerprintKeyPairs(t *testing.T) {
	entries, notes, err := Parse(testFP + ":" + testMEK + "," + testFP2 + ":" + testMEK2)
	if err != nil {
		t.Fatalf("Parse(fingerprint:key pairs): %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("canonical pairs must not be deprecated, got notes %v", notes)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Fingerprint != testFP || string(entries[0].MEK) != string(mustHex(testMEK)) {
		t.Errorf("entry 0 = %s, want fingerprint %s with the paired key", entries[0].Fingerprint, testFP)
	}
	if entries[1].Fingerprint != testFP2 {
		t.Errorf("entry 1 fingerprint = %s, want %s", entries[1].Fingerprint, testFP2)
	}
}

func TestParseFileReference(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mek-ring")
	spec := testFP + ":" + testMEK + "\n" + testFP2 + ":" + testMEK2
	if err := os.WriteFile(path, []byte(spec), 0o600); err != nil {
		t.Fatalf("write ring file: %v", err)
	}
	entries, _, err := Parse("@" + path)
	if err != nil {
		t.Fatalf("Parse(@file): %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("@file with newline-separated pairs: got %d entries, want 2", len(entries))
	}
}

func TestParseLegacyFormsReportDeprecationNotes(t *testing.T) {
	entries, notes, err := Parse(testFP + "," + testMEK2)
	if err != nil {
		t.Fatalf("Parse(legacy forms): %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].MEK != nil {
		t.Errorf("bare fingerprint entry must carry no key material")
	}
	if entries[0].Fingerprint != testFP {
		t.Errorf("bare fingerprint entry fingerprint = %s, want %s", entries[0].Fingerprint, testFP)
	}
	if entries[1].Fingerprint != testFP2 || entries[1].MEK == nil {
		t.Errorf("bare MEK entry must keep its computed fingerprint %s and key", testFP2)
	}
	if len(notes) != 2 {
		t.Fatalf("got %d deprecation notes, want 2: %v", len(notes), notes)
	}
	for i, want := range []string{"bare fingerprint", "bare MEK"} {
		if !strings.Contains(notes[i], want) {
			t.Errorf("note %d = %q, want it to name %q", i, notes[i], want)
		}
	}
}

func TestParseRejectsBadEntriesWithoutEchoingThem(t *testing.T) {
	cases := []struct {
		name   string
		spec   string
		wantEr string
	}{
		{"mismatched pair", badFP + ":" + testMEK, "does not match the key"},
		{"short fingerprint", "abcd:" + testMEK, "fingerprint must be 16 hex chars"},
		{"short key", testFP + ":" + strings.Repeat("ab", 31), "MEK must be 64 hex chars"},
		{"non-hex key", testFP + ":" + notHexKey, "MEK must be 64 hex chars"},
		{"non-hex bare entry", "zz-not-hex-ring-4leakcheck", "Invalid MEK ring entry 0"},
		{"non-hex fingerprint", "zzzzzzzzzzzzzzzz:" + testMEK, "fingerprint must be 16 hex chars"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Parse(tc.spec)
			if err == nil {
				t.Fatalf("Parse(%d-char spec): no error, want %q", len(tc.spec), tc.wantEr)
			}
			if !strings.Contains(err.Error(), tc.wantEr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantEr)
			}
			if strings.Contains(err.Error(), tc.spec) && len(tc.spec) > 80 {
				t.Errorf("error echoes the full entry (key material must not reach diagnostics): %q", err)
			}
		})
	}
}

func TestParseEmptySpecIsNoRing(t *testing.T) {
	entries, notes, err := Parse("")
	if err != nil {
		t.Fatalf("Parse(\"\"): %v", err)
	}
	if entries != nil || notes != nil {
		t.Errorf("Parse(\"\") = %v, %v; want nil, nil", entries, notes)
	}
}
