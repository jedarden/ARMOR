package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestMEKFingerprint(t *testing.T) {
	// Test with the known test vector
	fp := MEKFingerprint(TestVectorMEK)
	if fp != TestVectorFingerprint {
		t.Errorf("MEKFingerprint(TestVectorMEK) = %s, want %s", fp, TestVectorFingerprint)
	}
}

func TestMEKFingerprintEmpty(t *testing.T) {
	// Test with empty input
	fp := MEKFingerprint([]byte{})
	if fp != "" {
		t.Errorf("MEKFingerprint([]byte{}) = %s, want empty string", fp)
	}
}

func TestMEKFingerprintDifferentKeys(t *testing.T) {
	// Test that different keys produce different fingerprints
	key1 := make([]byte, 32)
	for i := range key1 {
		key1[i] = 0x01
	}
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = 0x02
	}

	fp1 := MEKFingerprint(key1)
	fp2 := MEKFingerprint(key2)

	if fp1 == fp2 {
		t.Error("Different keys should produce different fingerprints")
	}

	// Verify key1 matches test vector
	if fp1 != TestVectorFingerprint {
		t.Errorf("Key of all 0x01 should produce test vector fingerprint, got %s", fp1)
	}
}

func TestMEKFingerprintLength(t *testing.T) {
	// Verify fingerprint is always 16 hex characters
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	fp := MEKFingerprint(key)
	if len(fp) != 16 {
		t.Errorf("Fingerprint length = %d, want 16", len(fp))
	}

	// Verify it's valid hex
	_, err := hex.DecodeString(fp)
	if err != nil {
		t.Errorf("Fingerprint should be valid hex: %v", err)
	}
}

func TestIdentifierFingerprint(t *testing.T) {
	// SHA-256("test-bucket") = 1c8c1f6e8f5c4bd8f8d3a3f0e7e6b8b2...
	// The first 8 bytes hex-encoded to 16 characters is the fingerprint.
	// Recompute it here rather than hardcoding a second test vector, so this
	// test pins stability without duplicating the constant.
	want := func(v string) string {
		sum := sha256.Sum256([]byte(v))
		return hex.EncodeToString(sum[:8])[:16]
	}

	got := IdentifierFingerprint("test-bucket")
	if got != want("test-bucket") {
		t.Errorf("IdentifierFingerprint = %s, want %s", got, want("test-bucket"))
	}

	// Same shape and length as MEKFingerprint, so both render identically in a log.
	if len(got) != len(MEKFingerprint(TestVectorMEK)) {
		t.Errorf("IdentifierFingerprint length = %d, want %d to match MEKFingerprint",
			len(got), len(MEKFingerprint(TestVectorMEK)))
	}

	// The identifier itself must not be recoverable from, or present in, the fingerprint.
	if strings.Contains(got, "test-bucket") {
		t.Error("IdentifierFingerprint output must not contain the identifier")
	}
}

func TestIdentifierFingerprintEmpty(t *testing.T) {
	// An unset identifier must be distinguishable from a set one.
	if got := IdentifierFingerprint(""); got != "" {
		t.Errorf("IdentifierFingerprint(\"\") = %q, want empty string", got)
	}
}

func TestIdentifierFingerprintDistinct(t *testing.T) {
	fp1 := IdentifierFingerprint("bucket-one")
	fp2 := IdentifierFingerprint("bucket-two")

	if fp1 == fp2 {
		t.Error("Different identifiers should produce different fingerprints")
	}

	// Deterministic across calls -- this is what lets a log be compared
	// against the expected value.
	if fp1 != IdentifierFingerprint("bucket-one") {
		t.Error("IdentifierFingerprint must be deterministic for the same input")
	}
}

// TestIdentifierFingerprintVector pins the fingerprint to a value computed
// independently of the code under test.
//
// TestIdentifierFingerprint recomputes the expected result with the same
// SHA-256/truncate/hex recipe, which confirms self-consistency but would pass
// unchanged through any alteration of that recipe. Every logged bucket and
// access-key fingerprint is only comparable against a runbook or a previous
// pod's log for as long as this recipe never moves, so the pinned constant is
// what actually catches a silent algorithm change.
func TestIdentifierFingerprintVector(t *testing.T) {
	got := IdentifierFingerprint(TestVectorIdentifier)
	if got != TestVectorIdentifierFingerprint {
		t.Errorf("IdentifierFingerprint(%q) = %s, want %s -- the fingerprint algorithm moved and every previously logged fingerprint is now unmatchable",
			TestVectorIdentifier, got, TestVectorIdentifierFingerprint)
	}
}
