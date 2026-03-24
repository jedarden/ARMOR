package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrInvalidKeyLength = errors.New("invalid key length")
	ErrWrapFailed       = errors.New("key wrap failed")
	ErrUnwrapFailed     = errors.New("key unwrap failed")
)

// AES-KWP constants (RFC 5649)
const (
	kwpAIV = 0xA65959A6 // RFC 5649 Alternative Initial Value
)

// GenerateDEK creates a new random 256-bit data encryption key.
func GenerateDEK() ([]byte, error) {
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("failed to generate DEK: %w", err)
	}
	return dek, nil
}

// GenerateIV creates a new random 16-byte IV/nonce.
func GenerateIV() ([]byte, error) {
	iv := make([]byte, 16)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}
	return iv, nil
}

// WrapDEK wraps a DEK using the MEK with AES-KWP (RFC 5649).
// Returns the wrapped key (40 bytes for a 32-byte DEK).
func WrapDEK(mek, dek []byte) ([]byte, error) {
	if len(mek) != 32 {
		return nil, fmt.Errorf("%w: MEK must be 32 bytes", ErrInvalidKeyLength)
	}
	if len(dek) != 32 {
		return nil, fmt.Errorf("%w: DEK must be 32 bytes", ErrInvalidKeyLength)
	}

	block, err := aes.NewCipher(mek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// AES-KWP: RFC 5649 Section 4
	// For 32-byte keys, we use the simplified approach:
	// 1. Prepend the AIV (4 bytes) + length (4 bytes)
	// 2. Apply AES key wrap with 6 rounds

	// Construct the AIV + MLI (Message Length Indicator)
	aiv := make([]byte, 8)
	binary.BigEndian.PutUint32(aiv[0:4], kwpAIV)
	binary.BigEndian.PutUint32(aiv[4:8], uint32(len(dek)*8)) // MLI in bits

	// Pad DEK to multiple of 8 bytes (already 32, so no padding needed)
	// Concatenate AIV || DEK
	plaintext := make([]byte, 8+len(dek))
	copy(plaintext[0:8], aiv)
	copy(plaintext[8:], dek)

	// AES Key Wrap: 6 rounds for 32-bit semiblock
	return aesKeyWrap(block, plaintext)
}

// UnwrapDEK unwraps a wrapped DEK using the MEK with AES-KWP (RFC 5649).
func UnwrapDEK(mek, wrappedDEK []byte) ([]byte, error) {
	if len(mek) != 32 {
		return nil, fmt.Errorf("%w: MEK must be 32 bytes", ErrInvalidKeyLength)
	}
	if len(wrappedDEK) != 40 {
		return nil, fmt.Errorf("%w: wrapped DEK must be 40 bytes", ErrInvalidKeyLength)
	}

	block, err := aes.NewCipher(mek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// AES Key Unwrap
	plaintext, err := aesKeyUnwrap(block, wrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnwrapFailed, err)
	}

	// Verify AIV and extract DEK
	if len(plaintext) < 8 {
		return nil, fmt.Errorf("%w: plaintext too short", ErrUnwrapFailed)
	}

	// Check AIV
	aiv := binary.BigEndian.Uint32(plaintext[0:4])
	if aiv != kwpAIV {
		return nil, fmt.Errorf("%w: invalid AIV", ErrUnwrapFailed)
	}

	// Check MLI
	mli := binary.BigEndian.Uint32(plaintext[4:8])
	if mli != 256 { // 32 bytes * 8 bits
		return nil, fmt.Errorf("%w: invalid MLI", ErrUnwrapFailed)
	}

	// Extract DEK
	dek := make([]byte, 32)
	copy(dek, plaintext[8:40])

	return dek, nil
}

// aesKeyWrap implements AES Key Wrap (RFC 3394).
func aesKeyWrap(block cipher.Block, plaintext []byte) ([]byte, error) {
	n := (len(plaintext) - 8) / 8 // number of semiblocks
	if n < 1 {
		return nil, errors.New("plaintext too short")
	}

	// Initialize
	a := make([]byte, 8)
	copy(a, plaintext[0:8])

	r := make([][]byte, n)
	for i := 0; i < n; i++ {
		r[i] = make([]byte, 8)
		copy(r[i], plaintext[8+i*8:8+(i+1)*8])
	}

	// 6n rounds
	for j := 0; j < 6; j++ {
		for i := 0; i < n; i++ {
			// B = AES(A || R[i])
			b := make([]byte, 16)
			copy(b[0:8], a)
			copy(b[8:16], r[i])
			block.Encrypt(b, b)

			// A = MSB(B) XOR t
			copy(a, b[0:8])
			t := uint64(j*n + i + 1)
			xorUint64(a, t)

			// R[i] = LSB(B)
			copy(r[i], b[8:16])
		}
	}

	// Output: C0 || C1 || ... || Cn
	ciphertext := make([]byte, 8*(n+1))
	copy(ciphertext[0:8], a)
	for i := 0; i < n; i++ {
		copy(ciphertext[8+i*8:8+(i+1)*8], r[i])
	}

	return ciphertext, nil
}

// aesKeyUnwrap implements AES Key Unwrap (RFC 3394).
func aesKeyUnwrap(block cipher.Block, ciphertext []byte) ([]byte, error) {
	n := (len(ciphertext) - 8) / 8
	if n < 1 {
		return nil, errors.New("ciphertext too short")
	}

	// Initialize
	a := make([]byte, 8)
	copy(a, ciphertext[0:8])

	r := make([][]byte, n)
	for i := 0; i < n; i++ {
		r[i] = make([]byte, 8)
		copy(r[i], ciphertext[8+i*8:8+(i+1)*8])
	}

	// Reverse 6n rounds
	for j := 5; j >= 0; j-- {
		for i := n - 1; i >= 0; i-- {
			// A = MSB(C) XOR t
			t := uint64(j*n + i + 1)
			xorUint64(a, t)

			// B = AES-1(A || R[i])
			b := make([]byte, 16)
			copy(b[0:8], a)
			copy(b[8:16], r[i])
			block.Decrypt(b, b)

			// A = MSB(B)
			copy(a, b[0:8])

			// R[i] = LSB(B)
			copy(r[i], b[8:16])
		}
	}

	// Output: A || R[0] || ... || R[n-1]
	plaintext := make([]byte, 8*(n+1))
	copy(plaintext[0:8], a)
	for i := 0; i < n; i++ {
		copy(plaintext[8+i*8:8+(i+1)*8], r[i])
	}

	return plaintext, nil
}

// xorUint64 XORs a uint64 value into an 8-byte slice (big-endian).
func xorUint64(a []byte, t uint64) {
	a[0] ^= byte(t >> 56)
	a[1] ^= byte(t >> 48)
	a[2] ^= byte(t >> 40)
	a[3] ^= byte(t >> 32)
	a[4] ^= byte(t >> 24)
	a[5] ^= byte(t >> 16)
	a[6] ^= byte(t >> 8)
	a[7] ^= byte(t)
}
