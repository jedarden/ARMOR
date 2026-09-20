package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The v3 counter layout IV[0:8] || uint16(part) || uint32(block) ||
// uint16(aesBlock) gives the aesBlock field exactly 65536 slots, so a block
// payload may span at most V3MaxBlockSize = 1 MiB. The historical
// per-16-byte loop wrapped uint16(aesBlockIdx) back to 0 beyond that and
// silently reused keystream; validateV3BlockPayload fails closed instead.
// The decrypt-side rejection is pinned by armor-7b40cb59 alongside the
// one-CTR-stream-per-ARMOR-block decrypt paths, the encrypt-side rejection
// by armor-2164a738 alongside the one-CTR-stream encrypt path.
func TestV3DecryptRejectsOversizedBlockPayload(t *testing.T) {
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 3)
	}
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i + 90)
	}
	hmacKey, err := DeriveHMACKey(dek)
	require.NoError(t, err)

	// One byte past the u16 aesBlock capacity, carrying a valid HMAC so the
	// payload guard — not authentication — is what must reject it.
	oversized := make([]byte, V3MaxBlockSize+1)
	for i := range oversized {
		oversized[i] = byte(i * 7)
	}
	oversizedMAC := ComputeV3BlockHMAC(hmacKey, 1, 2, oversized)

	t.Run("DecryptBlockV3 fails closed", func(t *testing.T) {
		plaintext, err := DecryptBlockV3(dek, iv, 1, 2, oversized, oversizedMAC, V3MaxBlockSize)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds Version3 maximum")
		assert.Nil(t, plaintext, "no plaintext may be released for an oversized block payload")
	})

	t.Run("Decryptor block path fails closed", func(t *testing.T) {
		decryptor, err := NewDecryptorWithVersion(dek, iv, V3MaxBlockSize, Version3)
		require.NoError(t, err)
		plaintext, err := decryptor.decryptBlockV3(oversized, 1, 2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds Version3 maximum")
		assert.Nil(t, plaintext)
	})

	t.Run("boundary payload still decrypts", func(t *testing.T) {
		// Exactly V3MaxBlockSize = 65536 AES blocks exhausts the u16 aesBlock
		// field (indices 0..0xFFFF) without wrapping and must stay decryptable
		// on the direct path...
		plaintext := make([]byte, V3MaxBlockSize)
		for i := range plaintext {
			plaintext[i] = byte(i * 13)
		}
		ciphertext, macValue, err := EncryptBlockV3(dek, iv, 1, 2, plaintext, V3MaxBlockSize)
		require.NoError(t, err)

		decrypted, err := DecryptBlockV3(dek, iv, 1, 2, ciphertext, macValue, V3MaxBlockSize)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)

		// ...and on the framing path, where DecryptV3 slices per-block
		// payloads out of the concatenated ciphertext.
		encryptor, err := NewEncryptorWithVersion(dek, iv, V3MaxBlockSize, Version3)
		require.NoError(t, err)
		framedCiphertext, blockTable, err := encryptor.EncryptV3(plaintext, false)
		require.NoError(t, err)

		framingDecryptor, err := NewDecryptorWithVersion(dek, iv, V3MaxBlockSize, Version3)
		require.NoError(t, err)
		framed, err := framingDecryptor.DecryptV3(framedCiphertext, 0, blockTable)
		require.NoError(t, err)
		assert.Equal(t, plaintext, framed)
	})
}

// TestV3EncryptRejectsOversizedBlockPayload pins the same fail-closed bound
// on the encrypt side: a plaintext one byte past the u16 aesBlock capacity
// must be rejected before any keystream is generated — the historical
// EncryptBlockV3 wrapped uint16(aesBlockIdx) back to 0 there and silently
// reused keystream — and must release neither ciphertext nor HMAC.
func TestV3EncryptRejectsOversizedBlockPayload(t *testing.T) {
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 3)
	}
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i + 90)
	}
	hmacKey, err := DeriveHMACKey(dek)
	require.NoError(t, err)

	// Valid blockSize parameter, oversized plaintext: the payload guard —
	// not the blockSize check — is what must reject it.
	plaintext := make([]byte, V3MaxBlockSize+1)
	for i := range plaintext {
		plaintext[i] = byte(i * 7)
	}

	ciphertext, macValue, err := EncryptBlockV3(dek, iv, 1, 2, plaintext, V3MaxBlockSize)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds Version3 maximum")
	assert.Nil(t, ciphertext, "no ciphertext may be released for an oversized block payload")
	assert.Nil(t, macValue, "no HMAC may be released for an oversized block payload")

	// Exactly V3MaxBlockSize stays encryptable and matches the historical
	// keystream at the boundary where the u16 aesBlock field is exhausted.
	boundary := plaintext[:V3MaxBlockSize]
	ciphertext, macValue, err = EncryptBlockV3(dek, iv, 1, 2, boundary, V3MaxBlockSize)
	require.NoError(t, err)
	assert.Equal(t, referenceV3Keystream(t, dek, iv, 1, 2, boundary), ciphertext)
	assert.Equal(t, referenceV3BlockHMAC(t, hmacKey, 1, 2, ciphertext), macValue)
}
