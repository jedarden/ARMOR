// Package mekring defines the one MEK-ring specification grammar shared by
// the armor CLI and the companion binaries, so `-mek-ring` means the same
// thing everywhere (armor-b1e3a56b).
//
// A ring specification is either a comma-separated list of entries or an
// @file reference naming a file that holds the same list:
//
//	mekring.Parse("3fa9c1d2b4e6f801:abab...,cd10ef...")   // fingerprint:key pairs
//	mekring.Parse("@/etc/armor/mek-ring")                 // same list, from a file
//
// The canonical entry is `fingerprint:key` — a 16-hex-char fingerprint (the
// first 8 bytes of SHA-256 of the key, exactly what crypto.MEKFingerprint
// produces and what wrapped-DEK selection matches on) followed by a 64-hex-
// char (32-byte) MEK. The pair must agree: a fingerprint that does not match
// its key is rejected rather than silently mismatching at unwrap time.
//
// Two bare legacy forms are still accepted so existing scripts keep working
// during the deprecation window, and are reported in the returned notes so
// the caller can print the one-release pointer:
//
//   - a bare 16-hex-char fingerprint (armor decrypt's historical form: it
//     validates format but carries no key material, so the entry comes back
//     with a nil MEK and can never unwrap anything);
//   - a bare 64-hex-char MEK (restore-verifier's historical form: usable key
//     material on the command line, which the secrets-by-reference rule
//     retires — the fingerprint is computed at parse time).
package mekring

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/jedarden/armor/internal/crypto"
)

// Parse parses a MEK ring specification into ring entries. It returns the
// entries, one deprecation note per legacy entry (already human-readable),
// and an error whose message starts with "invalid MEK ring entry <n>" so the
// caller's diagnostics stay stable. An empty specification parses to no
// entries and no error.
func Parse(spec string) ([]crypto.RingKeyEntry, []string, error) {
	if spec == "" {
		return nil, nil, nil
	}

	if strings.HasPrefix(spec, "@") {
		data, err := os.ReadFile(strings.TrimPrefix(spec, "@"))
		if err != nil {
			return nil, nil, fmt.Errorf("read MEK ring file: %w", err)
		}
		spec = string(data)
	}

	var (
		entries []crypto.RingKeyEntry
		notes   []string
	)
	// The @file form may keep one entry per line; the inline form is
	// comma-separated. Splitting on both and dropping empties accepts
	// either shape for either source.
	for i, raw := range strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == '\n' }) {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		fp, key, paired := strings.Cut(entry, ":")
		if paired {
			ringEntry, err := parsePaired(i, fp, key)
			if err != nil {
				return nil, nil, err
			}
			entries = append(entries, ringEntry)
			continue
		}

		// Legacy bare forms — accepted for one release, reported so the
		// caller prints the pointer.
		switch {
		case isHex(entry) && len(entry) == 16:
			entries = append(entries, crypto.RingKeyEntry{MEK: nil, Fingerprint: entry})
			notes = append(notes,
				fmt.Sprintf("entry %d is a bare fingerprint (legacy armor decrypt form): it carries no key material, so it can never unwrap; pass fingerprint:key or an @file/env source", i))
		case isHex(entry) && len(entry) == 64:
			mek, err := decodeKey(i, entry)
			if err != nil {
				return nil, nil, err
			}
			entries = append(entries, crypto.RingKeyEntry{MEK: mek, Fingerprint: crypto.MEKFingerprint(mek)})
			notes = append(notes,
				fmt.Sprintf("entry %d is a bare MEK (legacy restore-verifier form): key material on the command line lands in shell history and ps; pass fingerprint:key pairs, an @file, or the environment source", i))
		default:
			return nil, nil, fmt.Errorf("invalid MEK ring entry %d: expected fingerprint:key (16 hex chars : 64 hex chars), got %d chars", i, len(entry))
		}
	}

	return entries, notes, nil
}

// parsePaired validates one fingerprint:key entry and checks the pair agree.
func parsePaired(i int, fp, key string) (crypto.RingKeyEntry, error) {
	fp = strings.TrimSpace(fp)
	key = strings.TrimSpace(key)
	if len(fp) != 16 || !isHex(fp) {
		return crypto.RingKeyEntry{}, fmt.Errorf("invalid MEK ring entry %d: fingerprint must be 16 hex chars, got %d chars", i, len(fp))
	}
	mek, err := decodeKey(i, key)
	if err != nil {
		return crypto.RingKeyEntry{}, err
	}
	if actual := crypto.MEKFingerprint(mek); actual != fp {
		return crypto.RingKeyEntry{}, fmt.Errorf("invalid MEK ring entry %d: fingerprint %s does not match the key (key fingerprints to %s)", i, fp, actual)
	}
	return crypto.RingKeyEntry{MEK: mek, Fingerprint: fp}, nil
}

// decodeKey hex-decodes one 64-char (32-byte) MEK.
func decodeKey(i int, key string) ([]byte, error) {
	if len(key) != 64 || !isHex(key) {
		return nil, fmt.Errorf("invalid MEK ring entry %d: MEK must be 64 hex chars (32 bytes), got %d chars", i, len(key))
	}
	mek, err := hex.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("invalid MEK ring entry %d: invalid hex: %w", i, err)
	}
	if len(mek) != 32 {
		return nil, fmt.Errorf("invalid MEK ring entry %d: got %d bytes, expected 32", i, len(mek))
	}
	return mek, nil
}

// isHex reports whether s is entirely hex digits.
func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return len(s) > 0
}
