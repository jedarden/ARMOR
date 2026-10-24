package crypto

import (
	"bytes"
	"errors"
	"testing"
)

// The per-object V3BlockCryptor must produce bytes identical to the static
// one-shot EncryptBlockV3/DecryptBlockV3 across every (part, block) slot it
// can serve, so an object encrypted through the per-object context decrypts
// through the one-shot path and vice versa — no storage-format change.

func TestV3BlockCryptorMatchesStaticBlockFunctions(t *testing.T) {
	dek, iv := benchV3KeyMaterial()

	cryptor, err := NewV3BlockCryptor(dek, iv, 64*1024)
	if err != nil {
		t.Fatalf("NewV3BlockCryptor: %v", err)
	}

	block := benchV3Plaintext(1000)
	for _, tc := range []struct {
		part  uint16
		block uint32
	}{
		{0, 0}, {0, 1}, {1, 0}, {1, 5}, {7, 1234},
	} {
		gotCiphertext, gotHMAC, err := cryptor.EncryptBlock(tc.part, tc.block, block)
		if err != nil {
			t.Fatalf("EncryptBlock(part=%d, block=%d): %v", tc.part, tc.block, err)
		}
		wantCiphertext, wantHMAC, err := EncryptBlockV3(dek, iv, tc.part, tc.block, block, 64*1024)
		if err != nil {
			t.Fatalf("EncryptBlockV3(part=%d, block=%d): %v", tc.part, tc.block, err)
		}
		if !bytes.Equal(gotCiphertext, wantCiphertext) {
			t.Fatalf("part=%d block=%d: cryptor ciphertext != static ciphertext", tc.part, tc.block)
		}
		if !bytes.Equal(gotHMAC, wantHMAC) {
			t.Fatalf("part=%d block=%d: cryptor HMAC != static HMAC", tc.part, tc.block)
		}

		gotPlaintext, err := cryptor.DecryptBlock(tc.part, tc.block, gotCiphertext, gotHMAC)
		if err != nil {
			t.Fatalf("DecryptBlock(part=%d, block=%d): %v", tc.part, tc.block, err)
		}
		if !bytes.Equal(gotPlaintext, block) {
			t.Fatalf("part=%d block=%d: roundtrip mismatch", tc.part, tc.block)
		}

		// The one-shot decryptor accepts the cryptor's ciphertext and vice
		// versa.
		oneShot, err := DecryptBlockV3(dek, iv, tc.part, tc.block, gotCiphertext, gotHMAC, 64*1024)
		if err != nil {
			t.Fatalf("DecryptBlockV3 of cryptor ciphertext: %v", err)
		}
		if !bytes.Equal(oneShot, block) {
			t.Fatalf("part=%d block=%d: one-shot decrypt of cryptor ciphertext mismatch", tc.part, tc.block)
		}
	}
}

func TestV3BlockCryptorRejectsBadInput(t *testing.T) {
	dek, iv := benchV3KeyMaterial()

	cryptor, err := NewV3BlockCryptor(dek, iv, 64*1024)
	if err != nil {
		t.Fatalf("NewV3BlockCryptor: %v", err)
	}

	plaintext := benchV3Plaintext(128)
	ciphertext, mac, err := cryptor.EncryptBlock(0, 0, plaintext)
	if err != nil {
		t.Fatalf("EncryptBlock: %v", err)
	}

	if _, err := cryptor.DecryptBlock(0, 0, ciphertext, mac[:31]); err == nil {
		t.Fatal("short expected HMAC accepted")
	}

	bad := append([]byte(nil), mac...)
	bad[0] ^= 0xff
	if _, err := cryptor.DecryptBlock(0, 0, ciphertext, bad); !errors.Is(err, ErrHMACMismatch) {
		t.Fatalf("tampered HMAC: got %v, want ErrHMACMismatch", err)
	}
	// No plaintext may be released for a failed verification.
	if _, err := cryptor.DecryptBlock(0, 1, ciphertext, mac); !errors.Is(err, ErrHMACMismatch) {
		t.Fatalf("block-number swap: got %v, want ErrHMACMismatch", err)
	}

	oversized := make([]byte, V3MaxBlockSize+1)
	if _, _, err := cryptor.EncryptBlock(0, 0, oversized); err == nil {
		t.Fatal("oversized plaintext accepted")
	}
	if _, err := cryptor.DecryptBlock(0, 0, oversized, mac); err == nil {
		t.Fatal("oversized ciphertext accepted")
	}

	if _, err := NewV3BlockCryptor(dek, iv, V3MaxBlockSize+1); err == nil {
		t.Fatal("blockSize over the v3 maximum accepted")
	}
	if _, err := NewV3BlockCryptor(dek[:31], iv, 64*1024); err == nil {
		t.Fatal("short DEK accepted")
	}
	if _, err := NewV3BlockCryptor(dek, iv[:15], 64*1024); err == nil {
		t.Fatal("short IV accepted")
	}
}

// The pooled zstd codecs must survive reuse across calls — including a call
// that errored on malformed input — without cross-call state leakage.
func TestZstdCodecPoolReuse(t *testing.T) {
	blockA := bytes.Repeat([]byte("compressible armor payload A, "), 10)
	blockB := bytes.Repeat([]byte("a second, different payload that also compresses well, "), 8)

	compressedA, wasCompressed, _, err := Compress(blockA)
	if err != nil || !wasCompressed {
		t.Fatalf("Compress(blockA): compressed=%v err=%v", wasCompressed, err)
	}
	compressedB, wasCompressed, _, err := Compress(blockB)
	if err != nil || !wasCompressed {
		t.Fatalf("Compress(blockB): compressed=%v err=%v", wasCompressed, err)
	}

	// Interleave a malformed decompression between two valid ones: the
	// decoder the pool hands out next must be unaffected.
	if _, err := DecompressBlock([]byte{0x28, 0xb5, 0x2f, 0xfd, 0xde, 0xad}, true); err == nil {
		t.Fatal("malformed zstd frame decompressed without error")
	}

	gotA, err := DecompressBlock(compressedA, true)
	if err != nil {
		t.Fatalf("DecompressBlock(compressedA) after malformed input: %v", err)
	}
	if !bytes.Equal(gotA, blockA) {
		t.Fatal("DecompressBlock(compressedA): roundtrip mismatch after pool reuse")
	}
	gotB, err := DecompressBlock(compressedB, true)
	if err != nil {
		t.Fatalf("DecompressBlock(compressedB): %v", err)
	}
	if !bytes.Equal(gotB, blockB) {
		t.Fatal("DecompressBlock(compressedB): roundtrip mismatch after pool reuse")
	}

	// Same for the Decompress wrapper over the shared decoder pool.
	if _, err := Decompress(compressedA); err != nil {
		t.Fatalf("Decompress(compressedA): %v", err)
	}
}
