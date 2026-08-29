// Package crypto provides cryptographic primitives for ARMOR.
package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

// MEKFingerprint computes the fingerprint of a master encryption key.
// The fingerprint is the first 8 bytes of SHA-256(MEK), encoded as 16 hex characters.
// This provides a unique identifier for each key without exposing the key material.
func MEKFingerprint(mek []byte) string {
	if len(mek) == 0 {
		return ""
	}
	h := sha256.Sum256(mek)
	return hex.EncodeToString(h[:8])[:16]
}

// TestVectorFingerprint is a known test vector for MEKFingerprint.
// For a 32-byte key of all 0x01 bytes, the SHA-256 hash begins with
// the bytes shown here, resulting in this 16-hex-char fingerprint.
// This test vector ensures the fingerprint function remains stable across
// implementations and changes.
const TestVectorFingerprint = "588161913cc0c9f5"

// TestVectorMEK is the 32-byte key that produces TestVectorFingerprint.
// MEK: 0x01 repeated 32 times.
var TestVectorMEK = make([]byte, 32)

func init() {
	for i := range TestVectorMEK {
		TestVectorMEK[i] = 0x01
	}
}
