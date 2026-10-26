package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
)

// Version3 counter construction:
// counter_block = IV[0:8] || uint16(part) || uint32(block) || uint16(aesBlock)
//
// Per-(part,block) HMAC:
// hmac_input = uint16(part) || uint32(block) || ciphertext_block
// hmac = HMAC-SHA256(hmacKey, hmac_input)
//
// All integers are big-endian.

// V3MaxBlockSize is the maximum block size for Version3 (1 MiB).
// This ensures the per-block AES counter index (j) fits in 16 bits.
const V3MaxBlockSize = 1024 * 1024

// MakeV3Counter creates a 16-byte counter for Version3 encryption.
//
// Counter layout: IV[0:8] (8 bytes) || uint16(part) (2 bytes) || uint32(block) (4 bytes) || uint16(aesBlock) (2 bytes)
//
// Parameters:
//   - iv: 16-byte initialization vector
//   - part: Part number (0 for single-PUT, 1..10000 for multipart)
//   - block: ARMOR block index within the part (0-based)
//   - aesBlock: AES block index within the ARMOR block (0-based, max blockSize/16-1)
//
// Returns a 16-byte counter block for AES-CTR mode.
func MakeV3Counter(iv []byte, part uint16, block uint32, aesBlock uint16) []byte {
	if len(iv) != 16 {
		panic(fmt.Sprintf("IV must be 16 bytes, got %d", len(iv)))
	}

	counter := make([]byte, 16)
	// IV[0:8] - first 8 bytes of IV
	copy(counter[0:8], iv[0:8])
	// uint16(part) - part number (big-endian)
	binary.BigEndian.PutUint16(counter[8:10], part)
	// uint32(block) - ARMOR block index (big-endian)
	binary.BigEndian.PutUint32(counter[10:14], block)
	// uint16(aesBlock) - AES block index (big-endian)
	binary.BigEndian.PutUint16(counter[14:16], aesBlock)

	return counter
}

// ComputeV3BlockHMAC computes HMAC-SHA256 for a Version3 encrypted block.
//
// HMAC input: uint16(part) || uint32(block) || ciphertext_block
//
// Parameters:
//   - hmacKey: 32-byte HMAC key derived from DEK
//   - part: Part number (0 for single-PUT, 1..10000 for multipart)
//   - block: ARMOR block index within the part (0-based)
//   - ciphertext: Encrypted (and possibly compressed) block bytes
//
// Returns a 32-byte HMAC-SHA256 value.
func ComputeV3BlockHMAC(hmacKey []byte, part uint16, block uint32, ciphertext []byte) []byte {
	if len(hmacKey) != 32 {
		panic(fmt.Sprintf("HMAC key must be 32 bytes, got %d", len(hmacKey)))
	}

	mac := hmac.New(sha256.New, hmacKey)

	// Write part number and block index (big-endian) from one stack buffer
	// to keep the hot path allocation-free beyond the hash state.
	var prefix [4]byte
	binary.BigEndian.PutUint16(prefix[0:2], part)
	mac.Write(prefix[0:2])
	binary.BigEndian.PutUint32(prefix[0:4], block)
	mac.Write(prefix[0:4])

	// Write ciphertext
	mac.Write(ciphertext)

	return mac.Sum(nil)
}

// VerifyV3BlockHMAC verifies the HMAC for a Version3 encrypted block.
//
// Parameters:
//   - hmacKey: 32-byte HMAC key derived from DEK
//   - part: Part number (0 for single-PUT, 1..10000 for multipart)
//   - block: ARMOR block index within the part (0-based)
//   - ciphertext: Encrypted (and possibly compressed) block bytes
//   - expected: Expected HMAC value (32 bytes)
//
// Returns nil if HMAC is valid, ErrHMACMismatch if invalid.
func VerifyV3BlockHMAC(hmacKey []byte, part uint16, block uint32, ciphertext []byte, expected []byte) error {
	if len(expected) != HMACSize {
		return fmt.Errorf("expected HMAC must be %d bytes", HMACSize)
	}

	computed := ComputeV3BlockHMAC(hmacKey, part, block, ciphertext)
	if !hmac.Equal(computed, expected) {
		return ErrHMACMismatch
	}
	return nil
}

// newV3CTR returns a single CTR stream spanning one whole ARMOR block, seeded
// at aesBlock = 0 for the v3 counter construction
// IV[0:8] || uint16(part) || uint32(block) || uint16(aesBlock).
//
// Go's CTR mode increments the full 16-byte counter big-endian as one value,
// so successive keystream blocks run the aesBlock field 0, 1, 2, ... exactly
// as the historical one-cipher.NewCTR-per-16-bytes construction did. The
// increment cannot carry from the aesBlock field into the block field while a
// block payload holds at most 65536 AES blocks (V3MaxBlockSize), which
// validateV3BlockPayload enforces — so one stream per ARMOR block is
// byte-identical to the historical keystream.
func newV3CTR(blockCipher cipher.Block, iv []byte, part uint16, block uint32) cipher.Stream {
	if len(iv) != 16 {
		panic(fmt.Sprintf("IV must be 16 bytes, got %d", len(iv)))
	}
	counter := make([]byte, 16)
	copy(counter[0:8], iv[0:8])
	binary.BigEndian.PutUint16(counter[8:10], part)
	binary.BigEndian.PutUint32(counter[10:14], block)
	// counter[14:16] stays zero: the stream starts at aesBlock 0.
	return cipher.NewCTR(blockCipher, counter)
}

// validateV3BlockParams checks the DEK/IV/blockSize invariants shared by
// EncryptBlockV3 and DecryptBlockV3.
func validateV3BlockParams(dek, iv []byte, blockSize int) error {
	if len(dek) != 32 {
		return fmt.Errorf("DEK must be 32 bytes")
	}
	if len(iv) != 16 {
		return fmt.Errorf("IV must be 16 bytes")
	}
	if blockSize > V3MaxBlockSize {
		return fmt.Errorf("block size %d exceeds Version3 maximum %d", blockSize, V3MaxBlockSize)
	}
	return nil
}

// validateV3BlockPayload rejects a payload long enough to exhaust the u16
// aesBlock counter field (65536 AES blocks = V3MaxBlockSize). Beyond that the
// counter would carry into the part/block namespace; the historical
// implementation instead wrapped uint16(aesBlockIdx) back to 0 and silently
// reused keystream.
func validateV3BlockPayload(payload []byte) error {
	if len(payload) > V3MaxBlockSize {
		return fmt.Errorf("block payload %d bytes exceeds Version3 maximum %d", len(payload), V3MaxBlockSize)
	}
	return nil
}

// V3BlockCryptor is the per-object Version3 crypto context: the AES cipher,
// the HKDF-derived HMAC key, and a reusable HMAC state, all keyed off one
// DEK/IV pair. Constructing it once per object and reusing it across the
// block loop replaces the per-block aes.NewCipher, HKDF derivation, and
// hmac.New that the one-shot EncryptBlockV3/DecryptBlockV3 pay for every
// 64 KiB block. It is NOT safe for concurrent use; the reusable HMAC state
// is unguarded by design.
type V3BlockCryptor struct {
	blockCipher cipher.Block
	hmacKey     []byte
	iv          []byte
	mac         hash.Hash
}

// NewV3BlockCryptor derives the per-object v3 key material once: the AES
// cipher from the DEK and the HMAC key via HKDF. blockSize must be
// <= V3MaxBlockSize.
func NewV3BlockCryptor(dek, iv []byte, blockSize int) (*V3BlockCryptor, error) {
	if err := validateV3BlockParams(dek, iv, blockSize); err != nil {
		return nil, err
	}
	blockCipher, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	hmacKey, err := DeriveHMACKey(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to derive HMAC key: %w", err)
	}
	return newV3BlockCryptor(blockCipher, hmacKey, iv), nil
}

// newV3BlockCryptor assembles a cryptor from already-derived state (the AES
// cipher and HMAC key a Decryptor/Encryptor holds), skipping re-derivation.
func newV3BlockCryptor(blockCipher cipher.Block, hmacKey, iv []byte) *V3BlockCryptor {
	return &V3BlockCryptor{
		blockCipher: blockCipher,
		hmacKey:     hmacKey,
		iv:          iv,
		mac:         hmac.New(sha256.New, hmacKey),
	}
}

// EncryptBlock encrypts one ARMOR block of the object with v3 counter
// construction (see newV3CTR). The DEK/IV/blockSize were fixed when the
// cryptor was built, so only part, block index, and the payload vary.
// A plaintext longer than V3MaxBlockSize is rejected: the u16 aesBlock
// counter field would wrap or carry into the part/block namespace and
// reuse keystream.
func (c *V3BlockCryptor) EncryptBlock(part uint16, block uint32, plaintext []byte) (ciphertext []byte, hmacValue []byte, err error) {
	if err := validateV3BlockPayload(plaintext); err != nil {
		return nil, nil, err
	}

	// Encrypt the whole ARMOR block with one CTR stream (see newV3CTR for
	// why this is byte-identical to the historical per-16-byte construction).
	ciphertext = make([]byte, len(plaintext))
	hmacValue = make([]byte, HMACSize)
	if err := c.encryptBlockInto(ciphertext, hmacValue, part, block, plaintext); err != nil {
		return nil, nil, err
	}

	return ciphertext, hmacValue, nil
}

// encryptBlockInto permits exact overlap between dst and plaintext. The
// caller must hash plaintext before calling it, because encryption overwrites
// the input when the slices alias. macOut is filled without a per-block slice.
func (c *V3BlockCryptor) encryptBlockInto(dst, macOut []byte, part uint16, block uint32, plaintext []byte) error {
	if err := validateV3BlockPayload(plaintext); err != nil {
		return err
	}
	if len(dst) != len(plaintext) || len(macOut) != HMACSize {
		return fmt.Errorf("v3 block output sizes do not match input")
	}
	newV3CTR(c.blockCipher, c.iv, part, block).XORKeyStream(dst, plaintext)
	c.computeBlockHMACInto(macOut, part, block, dst)
	return nil
}

// DecryptBlock decrypts one ARMOR block of the object with v3 counter
// construction. The block's HMAC is verified exactly once, before any
// plaintext is released. A ciphertext longer than V3MaxBlockSize is
// rejected before any keystream is generated: the u16 aesBlock counter
// field would wrap or carry into the part/block namespace and decrypt with
// reused keystream.
func (c *V3BlockCryptor) DecryptBlock(part uint16, block uint32, ciphertext []byte, expectedHMAC []byte) (plaintext []byte, err error) {
	if err := validateV3BlockPayload(ciphertext); err != nil {
		return nil, err
	}

	if err := c.verifyBlockHMAC(part, block, ciphertext, expectedHMAC); err != nil {
		return nil, fmt.Errorf("HMAC verification failed for part %d block %d: %w", part, block, err)
	}

	// Decrypt the whole ARMOR block with one CTR stream (see newV3CTR).
	plaintext = make([]byte, len(ciphertext))
	newV3CTR(c.blockCipher, c.iv, part, block).XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// computeBlockHMAC computes HMAC-SHA256 over the v3 (part, block, ciphertext)
// input using the cryptor's reusable HMAC state. Byte-identical to
// ComputeV3BlockHMAC on the same key: same input order, same Sum.
func (c *V3BlockCryptor) computeBlockHMAC(part uint16, block uint32, ciphertext []byte) []byte {
	return c.computeBlockHMACInto(nil, part, block, ciphertext)
}

func (c *V3BlockCryptor) computeBlockHMACInto(dst []byte, part uint16, block uint32, ciphertext []byte) []byte {
	c.mac.Reset()

	var prefix [4]byte
	binary.BigEndian.PutUint16(prefix[0:2], part)
	c.mac.Write(prefix[0:2])
	binary.BigEndian.PutUint32(prefix[0:4], block)
	c.mac.Write(prefix[0:4])

	c.mac.Write(ciphertext)

	return c.mac.Sum(dst[:0])
}

// verifyBlockHMAC verifies a block HMAC with the reusable state, comparing
// into a stack buffer so verification allocates nothing.
func (c *V3BlockCryptor) verifyBlockHMAC(part uint16, block uint32, ciphertext, expected []byte) error {
	if len(expected) != HMACSize {
		return fmt.Errorf("expected HMAC must be %d bytes", HMACSize)
	}

	computed := c.computeBlockHMAC(part, block, ciphertext)
	if !hmac.Equal(computed, expected) {
		return ErrHMACMismatch
	}
	return nil
}

// EncryptBlockV3 encrypts a single ARMOR block with Version3 semantics.
// It derives the cipher and HMAC key for this one call; callers looping over
// an object's blocks should hold a V3BlockCryptor instead.
//
// Parameters:
//   - dek: 32-byte data encryption key
//   - iv: 16-byte initialization vector
//   - part: Part number (0 for single-PUT, 1..10000 for multipart)
//   - block: ARMOR block index within the part (0-based)
//   - plaintext: Plaintext block bytes (may be smaller than full block size for last block)
//   - blockSize: ARMOR block size (must be <= V3MaxBlockSize)
//
// Returns:
//   - ciphertext: Encrypted block (same length as plaintext)
//   - hmac: 32-byte HMAC-SHA256 of the encrypted block
//   - err: Error if encryption fails. A plaintext longer than V3MaxBlockSize
//     is rejected: the u16 aesBlock counter field would wrap or carry into
//     the part/block namespace and reuse keystream.
func EncryptBlockV3(dek, iv []byte, part uint16, block uint32, plaintext []byte, blockSize int) (ciphertext []byte, hmacValue []byte, err error) {
	cryptor, err := NewV3BlockCryptor(dek, iv, blockSize)
	if err != nil {
		return nil, nil, err
	}
	return cryptor.EncryptBlock(part, block, plaintext)
}

// DecryptBlockV3 decrypts a single ARMOR block with Version3 semantics.
// It derives the cipher and HMAC key for this one call; callers looping over
// an object's blocks should hold a V3BlockCryptor instead.
//
// Parameters:
//   - dek: 32-byte data encryption key
//   - iv: 16-byte initialization vector
//   - part: Part number (0 for single-PUT, 1..10000 for multipart)
//   - block: ARMOR block index within the part (0-based)
//   - ciphertext: Encrypted block bytes
//   - expectedHMAC: Expected HMAC-SHA256 value (32 bytes)
//   - blockSize: ARMOR block size (must be <= V3MaxBlockSize)
//
// Returns:
//   - plaintext: Decrypted block (same length as ciphertext)
//   - err: Error if decryption or HMAC verification fails. A ciphertext
//     longer than V3MaxBlockSize is rejected before any keystream is
//     generated: the u16 aesBlock counter field would wrap or carry into
//     the part/block namespace and decrypt with reused keystream.
func DecryptBlockV3(dek, iv []byte, part uint16, block uint32, ciphertext []byte, expectedHMAC []byte, blockSize int) (plaintext []byte, err error) {
	cryptor, err := NewV3BlockCryptor(dek, iv, blockSize)
	if err != nil {
		return nil, err
	}
	return cryptor.DecryptBlock(part, block, ciphertext, expectedHMAC)
}
