package crypto

import (
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	// HMACKeyInfo is the info string for HKDF key derivation.
	HMACKeyInfo = "armor-hmac-v1"
)

// DeriveHMACKey derives the HMAC key from a DEK using HKDF-SHA256.
// The HMAC key is used for per-block HMAC-SHA256 authentication.
func DeriveHMACKey(dek []byte) []byte {
	hkdf := hkdf.New(sha256.New, dek, nil, []byte(HMACKeyInfo))
	hmacKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdf, hmacKey); err != nil {
		panic("HKDF derivation failed: " + err.Error())
	}
	return hmacKey
}
