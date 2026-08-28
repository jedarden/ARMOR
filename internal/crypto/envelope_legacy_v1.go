//go:build legacyv1test
// +build legacyv1test

package crypto

// NewEnvelopeHeaderV1 creates a Version1 envelope header with legacy (vulnerable) counter derivation.
// This function is only available with the 'legacyv1test' build tag for testing purposes.
// WARNING: Version1 has a CTR keystream reuse vulnerability - adjacent blocks share keystream.
// DO NOT use Version1 for production data.
func NewEnvelopeHeaderV1(iv []byte, plaintextSize int64, blockSize int, plaintextSHA [32]byte) (*EnvelopeHeader, error) {
	return NewEnvelopeHeaderWithVersion(iv, plaintextSize, blockSize, plaintextSHA, Version1)
}
