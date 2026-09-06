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

// IdentifierFingerprint computes the fingerprint of a non-secret identifier,
// such as a bucket name or a client access-key ID. It is the first 8 bytes of
// SHA-256(value), encoded as 16 hex characters -- the same shape and length as
// MEKFingerprint, so both kinds of fingerprint render identically in a log.
//
// Identifiers of this kind are not secrets, but ARMOR deliberately does not
// publish them: a bucket name in a pod log is enough to let an unauthenticated
// caller target the bucket, and an access-key ID is the lookup half of a SigV4
// credential. Logging the fingerprint preserves the two things a startup log
// needs -- "is this the value I think it is?" (compare against
// IdentifierFingerprint of the expected name) and "did this change across
// restarts?" -- without publishing the identifier itself.
//
// An empty value yields an empty fingerprint, so an unset bucket is
// distinguishable from a set one.
func IdentifierFingerprint(value string) string {
	if value == "" {
		return ""
	}
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:8])[:16]
}

// TestVectorFingerprint is a known test vector for MEKFingerprint.
// For a 32-byte key of all 0x01 bytes, the SHA-256 hash is:
// 72cd6e8422c407fb6d098690f1130b7ded7ec2f7f5e1d30bd9d521f015363793
// The first 8 bytes (72cd6e8422c407fb) encode to this 16-hex-char fingerprint.
// This test vector ensures the fingerprint function remains stable across
// implementations and changes.
const TestVectorFingerprint = "72cd6e8422c407fb"

// TestVectorMEK is the 32-byte key that produces TestVectorFingerprint.
// MEK: 0x01 repeated 32 times.
var TestVectorMEK = make([]byte, 32)

func init() {
	for i := range TestVectorMEK {
		TestVectorMEK[i] = 0x01
	}
}

// TestVectorIdentifierFingerprint is the known test vector for
// IdentifierFingerprint("armor-example-bucket").
//
// A logged fingerprint is only useful for as long as the function that produced
// it is stable: runbooks and dashboards record the 16 hex characters, and a
// silent change to the digest, its truncation or its encoding would make every
// previously logged fingerprint unmatchable with no error to point at. This
// constant pins the algorithm the same way TestVectorFingerprint pins
// MEKFingerprint. It is SHA-256("armor-example-bucket")[0:8], hex-encoded.
const TestVectorIdentifierFingerprint = "6c9105eaeff5825c"

// TestVectorIdentifier is the input that produces TestVectorIdentifierFingerprint.
const TestVectorIdentifier = "armor-example-bucket"
