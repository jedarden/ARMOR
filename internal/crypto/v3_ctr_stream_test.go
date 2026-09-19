package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file are the characterization oracle for v3 block
// crypto: they pin the HISTORICAL construction — one cipher.NewCTR per 16
// bytes with the counter IV[0:8] || uint16(part) || uint32(block) ||
// uint16(aesBlock) built by MakeV3Counter, and the ComputeV3BlockHMAC input
// layout — and require EncryptBlockV3/DecryptBlockV3 to match it byte for
// byte. The committed golden vectors under testdata/v3 are cross-checked in
// both directions against the same reference.
//
// The oracle exists for the one-CTR-stream-per-ARMOR-block optimization
// tracked by armor-95f5cd32 and its children: a replacement keystream
// implementation must reproduce this reference exactly, so counter-semantics
// drift, cross-block keystream reuse, or an HMAC input change fails the
// build instead of corrupting data silently.

// referenceV3Keystream XORs src into a fresh buffer using the historical
// construction: one cipher.NewCTR per 16 bytes, counter from MakeV3Counter.
func referenceV3Keystream(t *testing.T, dek, iv []byte, part uint16, block uint32, src []byte) []byte {
	t.Helper()
	blockCipher, err := aes.NewCipher(dek)
	require.NoError(t, err)
	dst := make([]byte, len(src))
	numAESBlocks := (len(src) + 15) / 16
	for aesBlockIdx := 0; aesBlockIdx < numAESBlocks; aesBlockIdx++ {
		stream := cipher.NewCTR(blockCipher, MakeV3Counter(iv, part, block, uint16(aesBlockIdx)))
		start := aesBlockIdx * 16
		end := start + 16
		if end > len(src) {
			end = len(src)
		}
		stream.XORKeyStream(dst[start:end], src[start:end])
	}
	return dst
}

// referenceV3BlockHMAC recomputes the block HMAC the historical way, with
// separate part/block length-prefixed writes in the order
// uint16(part) || uint32(block) || ciphertext, so any change to
// ComputeV3BlockHMAC's input layout is cross-checked too.
func referenceV3BlockHMAC(t *testing.T, hmacKey []byte, part uint16, block uint32, ciphertext []byte) []byte {
	t.Helper()
	mac := hmac.New(sha256.New, hmacKey)
	partBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(partBytes, part)
	mac.Write(partBytes)
	blockBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(blockBytes, block)
	mac.Write(blockBytes)
	mac.Write(ciphertext)
	return mac.Sum(nil)
}

// TestV3BlockCryptoMatchesHistoricalKeystreamAndHMAC cross-checks
// EncryptBlockV3 and DecryptBlockV3 against the historical per-16-byte
// keystream and HMAC construction in both directions, across partial AES
// blocks, short final blocks, parts 0/1/10000 (and the u16 extreme), high
// block indices, 64 KiB, and the V3MaxBlockSize boundary where the u16
// aesBlock field is exactly exhausted.
func TestV3BlockCryptoMatchesHistoricalKeystreamAndHMAC(t *testing.T) {
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 1)
	}
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i + 10)
	}
	hmacKey, err := DeriveHMACKey(dek)
	require.NoError(t, err)

	// lengths: partial AES blocks (1, 15, 17, 4095), exact and straddling
	// AES block multiples (16, 31, 32, 63, 4096), 64 KiB neighbours, and the
	// 1 MiB maximum where 65536 AES blocks exactly exhaust the u16 aesBlock
	// field (indices 0..0xFFFF, no wrap).
	lengths := []int{
		1, 15, 16, 17, 31, 32, 63, 4095, 4096,
		64*1024 - 1, 64 * 1024, 64*1024 + 1,
		V3MaxBlockSize - 1, V3MaxBlockSize,
	}
	parts := []uint16{0, 1, 10000, 65535}
	blocks := []uint32{0, 1, 42, 0xFFFFFFFE, 0xFFFFFFFF}

	plaintextOf := func(n int) []byte {
		pt := make([]byte, n)
		x := uint32(0x9e3779b9)
		for i := range pt {
			x = x*1664525 + 1013904223
			pt[i] = byte(x >> 24)
		}
		return pt
	}

	for _, part := range parts {
		for _, block := range blocks {
			for _, n := range lengths {
				t.Run(fmt.Sprintf("part=%d_block=%d_len=%d", part, block, n), func(t *testing.T) {
					plaintext := plaintextOf(n)

					// Encrypt: the implementation must reproduce the
					// historical keystream and HMAC byte for byte.
					ciphertext, macValue, err := EncryptBlockV3(dek, iv, part, block, plaintext, V3MaxBlockSize)
					require.NoError(t, err)
					assert.Equal(t, referenceV3Keystream(t, dek, iv, part, block, plaintext), ciphertext)
					assert.Equal(t, referenceV3BlockHMAC(t, hmacKey, part, block, ciphertext), macValue)

					// Decrypt: the reference-encrypted ciphertext (not the
					// implementation's own output) must decrypt back.
					refCiphertext := referenceV3Keystream(t, dek, iv, part, block, plaintext)
					refMAC := referenceV3BlockHMAC(t, hmacKey, part, block, refCiphertext)
					decrypted, err := DecryptBlockV3(dek, iv, part, block, refCiphertext, refMAC, V3MaxBlockSize)
					require.NoError(t, err)
					assert.Equal(t, plaintext, decrypted)
				})
			}
		}
	}
}

// TestV3CounterExhaustionBoundary pins the u16 aesBlock field's exact
// capacity: a V3MaxBlockSize payload spans 65536 AES blocks whose counter
// indices run 0..0xFFFF without wrapping, and must match the historical
// reference and round-trip. The blockSize parameter above V3MaxBlockSize
// fails closed on decrypt as well (the encrypt side is covered by
// TestV3MaxBlockSizeConstraint in v3_counter_test.go).
//
// A payload LONGER than V3MaxBlockSize is NOT rejected by the historical
// code: the loop's uint16(aesBlockIdx) wraps back to 0 at AES block 65536
// and silently reuses the aesBlock=0 keystream. That defect is deliberately
// not asserted here — the fail-closed payload check (validateV3BlockPayload)
// lands with the armor-95f5cd32 optimization children, and pinning the wrap
// in this oracle would force that bead to flip this file. The boundary
// assertions below hold both before and after the hardening.
func TestV3CounterExhaustionBoundary(t *testing.T) {
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 5)
	}
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i + 50)
	}
	hmacKey, err := DeriveHMACKey(dek)
	require.NoError(t, err)

	// At the boundary: 1 MiB = exactly 65536 AES blocks, the last of which
	// uses counter aesBlock=0xFFFF. The u16 field is exactly exhausted.
	plaintext := make([]byte, V3MaxBlockSize)
	for i := range plaintext {
		plaintext[i] = byte(i * 11)
	}

	ciphertext, macValue, err := EncryptBlockV3(dek, iv, 10000, 0xFFFFFFFF, plaintext, V3MaxBlockSize)
	require.NoError(t, err)
	assert.Equal(t, referenceV3Keystream(t, dek, iv, 10000, 0xFFFFFFFF, plaintext), ciphertext,
		"keystream must match the historical reference at the exact u16 aesBlock boundary")
	assert.Equal(t, referenceV3BlockHMAC(t, hmacKey, 10000, 0xFFFFFFFF, ciphertext), macValue)

	decrypted, err := DecryptBlockV3(dek, iv, 10000, 0xFFFFFFFF, ciphertext, macValue, V3MaxBlockSize)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// The last AES block's keystream must come from counter aesBlock=0xFFFF
	// (not a wrapped 0): XOR the final 16 bytes with a stream seeded there.
	blockCipher, err := aes.NewCipher(dek)
	require.NoError(t, err)
	tail := make([]byte, 16)
	cipher.NewCTR(blockCipher, MakeV3Counter(iv, 10000, 0xFFFFFFFF, 0xFFFF)).XORKeyStream(tail, plaintext[V3MaxBlockSize-16:])
	assert.Equal(t, tail, ciphertext[V3MaxBlockSize-16:],
		"final AES block must use aesBlock=0xFFFF, proving no wrap at the boundary")

	// blockSize parameter beyond the maximum fails closed on decrypt.
	_, err = DecryptBlockV3(dek, iv, 0, 0, ciphertext, macValue, V3MaxBlockSize+1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds Version3 maximum")
}

// TestV3TamperFailsClosed verifies the fail-closed behaviour is unchanged:
// modified ciphertext, HMAC, part number or block index must all be rejected
// before any plaintext is returned.
func TestV3TamperFailsClosed(t *testing.T) {
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	plaintext := []byte("tamper detection must remain byte-exact under the single-stream CTR implementation")

	ciphertext, macValue, err := EncryptBlockV3(dek, iv, 7, 9, plaintext, 64*1024)
	require.NoError(t, err)

	tamperedCiphertext := bytes.Clone(ciphertext)
	tamperedCiphertext[len(tamperedCiphertext)/2] ^= 0x01

	tamperedMAC := bytes.Clone(macValue)
	tamperedMAC[0] ^= 0x01

	truncatedMAC := macValue[:HMACSize-1]

	cases := []struct {
		name       string
		part       uint16
		block      uint32
		ciphertext []byte
		mac        []byte
	}{
		{"modified ciphertext", 7, 9, tamperedCiphertext, macValue},
		{"modified HMAC", 7, 9, ciphertext, tamperedMAC},
		{"wrong part number", 8, 9, ciphertext, macValue},
		{"wrong block index", 7, 10, ciphertext, macValue},
		{"short HMAC", 7, 9, ciphertext, truncatedMAC},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decrypted, err := DecryptBlockV3(dek, iv, tc.part, tc.block, tc.ciphertext, tc.mac, 64*1024)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrHMACMismatch) || len(tc.mac) != HMACSize,
				"expected HMAC failure, got: %v", err)
			assert.Nil(t, decrypted, "no plaintext may be released when authentication fails")
		})
	}
}

// TestV3ShortFinalBlockRoundTrip exercises the framing-level path
// (EncryptV3 + DecryptV3) for objects whose final ARMOR block is short and
// whose lengths are not AES-block multiples.
func TestV3ShortFinalBlockRoundTrip(t *testing.T) {
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	blockSize := 64 * 1024

	for _, n := range []int{1, 16, blockSize - 1, blockSize, blockSize + 17, 2*blockSize + 5} {
		plaintext := make([]byte, n)
		for i := range plaintext {
			plaintext[i] = byte(i*31 + 7)
		}

		encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version3)
		require.NoError(t, err)
		ciphertext, blockTable, err := encryptor.EncryptV3(plaintext, false)
		require.NoError(t, err)

		decryptor, err := NewDecryptorWithVersion(dek, iv, blockSize, Version3)
		require.NoError(t, err)
		decrypted, err := decryptor.DecryptV3(ciphertext, 0, blockTable)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted, "round trip failed for n=%d", n)
	}
}

// TestV3GoldenVectorsMatchHistoricalConstruction cross-checks the committed
// golden vectors in testdata/v3 against the historical construction in both
// directions, independently of the generator in v3_vectors_test.go: it reads
// only the recorded JSON, re-encrypts each recorded plaintext block and
// compares ciphertext and HMAC byte for byte, then decrypts each recorded
// ciphertext block and compares the plaintext.
//
// The multipart vector records part 1's ciphertext in the top-level
// Ciphertext/Blocks fields while Plaintext holds the combined parts and the
// sidecar carries every part's block table; part 2's plaintext is the
// suffix of the combined plaintext and its construction is pinned through
// the sidecar's recorded HMACs (its ciphertext bytes are not recorded).
func TestV3GoldenVectorsMatchHistoricalConstruction(t *testing.T) {
	for _, name := range []string{"1-block-single-put", "3-block-compressed", "2-part-multipart"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "v3", name+".json"))
			require.NoError(t, err)

			var vec V3TestVector
			require.NoError(t, json.Unmarshal(data, &vec))

			dek, err := base64.StdEncoding.DecodeString(vec.DEK)
			require.NoError(t, err)
			iv, err := base64.StdEncoding.DecodeString(vec.IV)
			require.NoError(t, err)
			plaintext, err := base64.StdEncoding.DecodeString(vec.Plaintext)
			require.NoError(t, err)
			ciphertext, err := base64.StdEncoding.DecodeString(vec.Ciphertext)
			require.NoError(t, err)

			crossCheckPartBlocks := func(t *testing.T, label string, part uint16, partPlaintext []byte, entries []V3BlockEntry, partCiphertext []byte) {
				t.Helper()
				require.NotEmpty(t, entries, "%s: no block entries recorded", label)

				ctOffset := 0
				ptOffset := 0
				for blockIdx := uint32(0); blockIdx < uint32(len(entries)); blockIdx++ {
					entry := entries[blockIdx]
					blockHMAC, err := base64.StdEncoding.DecodeString(entry.HMAC)
					require.NoError(t, err, "%s: bad HMAC base64", label)

					end := ptOffset + vec.BlockSize
					if end > len(partPlaintext) {
						end = len(partPlaintext)
					}
					blockPlaintext := partPlaintext[ptOffset:end]
					require.Len(t, blockPlaintext, int(entry.CLen),
						"%s block %d: recorded clen must match the plaintext slice (vectors are uncompressed)", label, blockIdx)

					// Encrypt direction: historical construction over the
					// recorded plaintext must reproduce the recorded
					// ciphertext bytes and block HMAC.
					reCT, reMAC, err := EncryptBlockV3(dek, iv, part, blockIdx, blockPlaintext, vec.BlockSize)
					require.NoError(t, err)
					if partCiphertext != nil {
						require.GreaterOrEqual(t, len(partCiphertext), ctOffset+int(entry.CLen),
							"%s: recorded ciphertext shorter than block table", label)
						assert.Equal(t, partCiphertext[ctOffset:ctOffset+int(entry.CLen)], reCT,
							"%s block %d: encrypt direction mismatch", label, blockIdx)
					}
					assert.Equal(t, blockHMAC, reMAC,
						"%s block %d: recorded HMAC is not the historical construction's", label, blockIdx)

					// Decrypt direction: recorded ciphertext (where recorded)
					// must decrypt to the recorded plaintext under the
					// recorded HMAC; for sidecar-only parts the re-encrypted
					// ciphertext round-trips through the RECORDED HMAC.
					decInput := reCT
					if partCiphertext != nil {
						decInput = partCiphertext[ctOffset : ctOffset+int(entry.CLen)]
					}
					decrypted, err := DecryptBlockV3(dek, iv, part, blockIdx, decInput, blockHMAC, vec.BlockSize)
					require.NoError(t, err, "%s block %d: decrypt direction failed", label, blockIdx)
					assert.Equal(t, blockPlaintext, decrypted,
						"%s block %d: decrypt direction mismatch", label, blockIdx)

					ctOffset += int(entry.CLen)
					ptOffset = end
				}
				if partCiphertext != nil {
					assert.Equal(t, len(partCiphertext), ctOffset,
						"%s: recorded ciphertext length must equal the block table total", label)
				}
				assert.Equal(t, len(partPlaintext), ptOffset,
					"%s: block table must cover the whole part plaintext", label)
			}

			// A multipart vector composes the recorded Plaintext from the
			// sidecar's parts, in sidecar order; its top-level
			// Ciphertext/Blocks fields carry vec.Part alone. Walk the sidecar
			// so every part's construction is pinned, handing the top-level
			// ciphertext only to the vec.Part entry.
			if vec.Sidecar != nil {
				ptOffset := 0
				for _, pe := range vec.Sidecar.Parts {
					partLen := int(pe.PlaintextLen)
					require.LessOrEqual(t, ptOffset+partLen, len(plaintext),
						"sidecar part %d overruns combined plaintext", pe.N)
					var partCiphertext []byte
					if pe.N == vec.Part {
						partCiphertext = ciphertext
						assert.Equal(t, vec.Blocks, pe.Blocks,
							"top-level Blocks must be the sidecar entry for part %d", vec.Part)
					}
					crossCheckPartBlocks(t, fmt.Sprintf("sidecar part=%d", pe.N), pe.N,
						plaintext[ptOffset:ptOffset+partLen], pe.Blocks, partCiphertext)
					ptOffset += partLen
				}
				assert.Equal(t, len(plaintext), ptOffset,
					"sidecar parts must cover the whole combined plaintext")
			} else {
				// Single-part vector: top-level fields carry the whole object.
				crossCheckPartBlocks(t, fmt.Sprintf("part=%d", vec.Part), vec.Part, plaintext, vec.Blocks, ciphertext)
			}
		})
	}
}
