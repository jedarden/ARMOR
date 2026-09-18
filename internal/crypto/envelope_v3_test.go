package crypto_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/jedarden/armor/internal/crypto"
)

// fixedDEK and fixedIV are fixed test keys for the counter-block and HMAC
// format tests below.
//
// This file once also held a second TestGenerateV3Vectors golden generator
// that wrote "%d-<name>.json" files alongside the plain-named set from
// v3_vectors_test.go. It was removed: its vectors were built with a
// hand-rolled "simplified" HMAC key derivation (SHA256(DEK || info)) that the
// production HKDF-SHA256 crypto.DeriveHMACKey can never reproduce, so they
// could never be normative, and no consumer read them. The authoritative
// generator is the in-package one in v3_vectors_test.go, which builds vectors
// with the production crypto functions and writes the plain <name>.json
// files that cmd/armor and docs/format/envelope-v3.md consume.
var (
	fixedDEK = []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
	}
	fixedIV = []byte{
		0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0xFA, 0xF9, 0xF8,
		0xF7, 0xF6, 0xF5, 0xF4, 0xF3, 0xF2, 0xF1, 0xF0,
	}
)

// buildCounterBlock constructs the v3 counter block
func buildCounterBlock(iv []byte, part, blockIdx, aesBlockIdx int) []byte {
	counter := make([]byte, 16)

	// IV[0:8]
	copy(counter[0:8], iv[0:8])

	// uint16(part) - big-endian
	binary.BigEndian.PutUint16(counter[8:10], uint16(part))

	// uint32(blockIdx) - big-endian
	binary.BigEndian.PutUint32(counter[10:14], uint32(blockIdx))

	// uint16(aesBlockIdx) - big-endian
	binary.BigEndian.PutUint16(counter[14:16], uint16(aesBlockIdx))

	return counter
}

// TestV3CounterBlockUniqueness verifies that counter blocks are unique
func TestV3CounterBlockUniqueness(t *testing.T) {
	part := 1
	blockSize := 65536
	seen := make(map[string]bool)

	// Test multiple blocks and parts
	for blockIdx := 0; blockIdx < 10; blockIdx++ {
		for aesBlockIdx := 0; aesBlockIdx < blockSize/16; aesBlockIdx++ {
			cb := buildCounterBlock(fixedIV, part, blockIdx, aesBlockIdx)
			cbStr := fmt.Sprintf("%x", cb)

			if seen[cbStr] {
				t.Errorf("Duplicate counter block: part=%d block=%d aes=%d", part, blockIdx, aesBlockIdx)
			}
			seen[cbStr] = true
		}
	}

	// Verify different parts don't collide
	cb1 := buildCounterBlock(fixedIV, 1, 0, 0)
	cb2 := buildCounterBlock(fixedIV, 2, 0, 0)
	if bytes.Equal(cb1, cb2) {
		t.Error("Counter blocks collide across different parts")
	}
}

// TestV3HMACInputFormat verifies HMAC input format
func TestV3HMACInputFormat(t *testing.T) {
	hmacKey, err := crypto.DeriveHMACKey(fixedDEK)
	if err != nil {
		t.Fatalf("derive HMAC key: %v", err)
	}
	part := uint16(1)
	blockIdx := uint32(5)
	ciphertext := []byte{0xAA, 0xBB, 0xCC, 0xDD}

	mac := hmac.New(sha256.New, hmacKey)

	buf := make([]byte, 8) // 2 + 4 + 2 (padding)
	binary.BigEndian.PutUint16(buf[0:2], part)
	binary.BigEndian.PutUint32(buf[2:6], blockIdx)

	mac.Write(buf[0:6])
	mac.Write(ciphertext)

	result := mac.Sum(nil)

	// Verify HMAC is deterministic
	mac2 := hmac.New(sha256.New, hmacKey)
	binary.Write(mac2, binary.BigEndian, part)
	binary.Write(mac2, binary.BigEndian, blockIdx)
	mac2.Write(ciphertext)
	result2 := mac2.Sum(nil)

	if !hmac.Equal(result, result2) {
		t.Error("HMAC computation is not deterministic")
	}
}
