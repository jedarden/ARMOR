//go:build legacyv1test
// +build legacyv1test

package crypto

// NewEncryptorV1 creates a Version1 encryptor with the legacy (vulnerable) counter derivation.
// This function is only available with the 'legacyv1test' build tag for testing purposes.
// WARNING: Version1 has a CTR keystream reuse vulnerability - adjacent blocks share keystream.
// DO NOT use Version1 for production data.
func NewEncryptorV1(dek, iv []byte, blockSize int) (*Encryptor, error) {
	return NewEncryptorWithVersion(dek, iv, blockSize, Version1)
}

// NewEncryptorWithCounterV1 creates a Version1 encryptor with a specific starting counter.
// This function is only available with the 'legacyv1test' build tag for testing purposes.
// WARNING: Version1 has a CTR keystream reuse vulnerability - adjacent blocks share keystream.
// DO NOT use Version1 for production data.
func NewEncryptorWithCounterV1(dek, iv []byte, blockSize int, startBlockIndex uint32) (*Encryptor, error) {
	return NewEncryptorWithCounterAndVersion(dek, iv, blockSize, startBlockIndex, Version1)
}
