package crypto

import (
	"encoding/hex"
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
