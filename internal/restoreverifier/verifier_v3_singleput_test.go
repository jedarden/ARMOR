package restoreverifier

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// armorEncryptV3SinglePut builds a v3 single-PUT object exactly the way the
// server's PutObject does (handlers.go: header || EncryptV3 blocks || encoded
// trailer block table) plus the metadata it stores, so the verifier's two
// restore paths are exercised against the real on-B2 layout.
func armorEncryptV3SinglePut(t *testing.T, mek []byte, blockSize int, plaintext []byte) (ciphertext []byte, meta map[string]string) {
	t.Helper()
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}
	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}
	wrapped, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}
	sha := crypto.ComputePlaintextSHA256(plaintext)

	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, sha, crypto.Version3)
	if err != nil {
		t.Fatalf("NewEnvelopeHeaderWithVersion: %v", err)
	}
	headerBytes, err := header.Encode()
	if err != nil {
		t.Fatalf("header encode: %v", err)
	}
	enc, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, crypto.Version3)
	if err != nil {
		t.Fatalf("NewEncryptorWithVersion: %v", err)
	}
	encrypted, blockTable, err := enc.EncryptV3(plaintext, false)
	if err != nil {
		t.Fatalf("EncryptV3: %v", err)
	}
	trailer, err := blockTable.Encode()
	if err != nil {
		t.Fatalf("block table encode: %v", err)
	}

	envelope := make([]byte, 0, len(headerBytes)+len(encrypted)+len(trailer))
	envelope = append(envelope, headerBytes...)
	envelope = append(envelope, encrypted...)
	envelope = append(envelope, trailer...)

	m := (&backend.ARMORMetadata{
		Version:        3,
		BlockSize:      blockSize,
		PlaintextSize:  int64(len(plaintext)),
		IV:             iv,
		WrappedDEK:     wrapped,
		MEKFingerprint: crypto.MEKFingerprint(mek),
		PlaintextSHA:   hexEncode(sha[:]),
	}).ToMetadata()
	return envelope, m
}

// TestRestoreViaARMOR_V3SinglePut_ReadsFullTrailer is the regression test for
// armor-8d4420fc: the ARMOR path sized a v3 single-PUT object with the v1/v2
// HMAC-table entry width (32 bytes per block) although the v3 trailer entry is
// 36 bytes (HMAC + CLen). The fetch came up 4 bytes per block short, the
// trailer slice started 4*blockCount bytes early, and the CLen field was read
// out of HMAC bytes ("failed to decode v3 block table: ciphertext length N
// exceeds block size"), so every v3 single-PUT object in the fleet failed the
// dual-path check while the direct path succeeded. The sizes below cover one
// block, an exact block boundary, and several blocks with a partial tail.
func TestRestoreViaARMOR_V3SinglePut_ReadsFullTrailer(t *testing.T) {
	const blockSize = 4096
	mek := bytes.Repeat([]byte{0x5A}, 32)
	rng := rand.New(rand.NewSource(1969))

	for _, n := range []int{1112, blockSize, 3*blockSize + 17} {
		t.Run(fmt.Sprintf("%d_bytes", n), func(t *testing.T) {
			plaintext := make([]byte, n)
			rng.Read(plaintext)
			ct, meta := armorEncryptV3SinglePut(t, mek, blockSize, plaintext)

			blockCount := crypto.ComputeBlockCount(int64(n), blockSize)
			wantLen := crypto.HeaderSize + int64(n) + int64(blockCount)*crypto.BlockTableEntrySize
			if int64(len(ct)) != wantLen {
				t.Fatalf("fixture layout: got %d stored bytes, want %d (header + ciphertext + %d x 36-byte entries)", len(ct), wantLen, blockCount)
			}

			key := fmt.Sprintf("v3/%d.bin", n)
			fb := &fakeBackend{
				ciphertext: ct, plaintext: plaintext,
				info: &backend.ObjectInfo{Key: key, Size: int64(n), StoredSize: int64(len(ct)), Metadata: meta},
			}
			v := New(fb, mek, nil, blockSize, nil, Config{})

			got, err := v.restoreViaARMOR(context.Background(), "b", key)
			if err != nil {
				t.Fatalf("restoreViaARMOR: %v", err)
			}
			if !bytes.Equal(got, plaintext) {
				t.Fatalf("restoreViaARMOR returned %d bytes that differ from the %d-byte plaintext", len(got), n)
			}

			direct, err := v.restoreViaDirectDecrypt(context.Background(), "b", key)
			if err != nil {
				t.Fatalf("restoreViaDirectDecrypt: %v", err)
			}
			if !bytes.Equal(direct, plaintext) {
				t.Fatalf("direct path returned %d bytes that differ from the plaintext", len(direct))
			}

			result := v.verifyObject(context.Background(), ObjectSample{Key: key, Bucket: "b", ArtifactType: ArtifactGeneric, Metadata: meta}, ModeDual)
			if result.Status != StatusPass {
				t.Fatalf("dual-path verify: status %q path %q error %q, want %q", result.Status, result.Path, result.Error, StatusPass)
			}
		})
	}
}
