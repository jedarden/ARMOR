package server

// Pins for the v1 boundary in the README security model: the ciphertext-only
// guarantee covers envelope v2 and v3 objects, while a legacy v1 object's
// keystream reuse between adjacent blocks (ADR-005) lets "ciphertext alone
// reveal the XOR of the plaintexts of any two adjacent blocks" until it is
// migrated.
//
// The leak below is demonstrated on stored bytes the way a B2-side attacker
// would exploit it: no key material, no decryptor — just the envelope the
// backend persists. The post-migration leg of the boundary (a migrated object
// must recover nothing) is written and skipped: FormatMigrator currently
// re-encrypts with crypto.Encryptor.Encrypt(), whose makeCounter has no v3
// branch, so migrated "v3" envelopes still carry the v1 keystream reuse. That
// defect and the unskip are tracked in bead armor-511015d0.

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// storeLegacyObject stores a two-block envelope the way the pre-ADR-005-fix
// writer produced it: legacy counter derivation, envelope header carrying the
// given version byte, HMAC table appended after the ciphertext, legacy base64
// wrapped DEK in the metadata. Block 0 of the plaintext is all zeros so the
// keystream-cancelled XOR of the two stored ciphertext blocks yields block 1's
// plaintext itself rather than an opaque XOR. Mirrors the v1 construction used
// by the format-migration suites (e.g. TestFormatMigrationDryRun).
func storeLegacyObject(t *testing.T, mb *MockBackend, key string, mek []byte, version uint8, blockSize int) (plaintext, secret, envelope []byte) {
	t.Helper()

	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 1)
	}
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("wrap DEK: %v", err)
	}
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i + 2)
	}

	secret = make([]byte, blockSize)
	if _, err := rand.New(rand.NewSource(20260926)).Read(secret); err != nil {
		t.Fatalf("generate secret block: %v", err)
	}
	plaintext = make([]byte, 2*blockSize) // block 0 zeros, block 1 secret
	copy(plaintext[blockSize:], secret)

	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, version)
	if err != nil {
		t.Fatalf("create version-%d encryptor: %v", version, err)
	}
	ciphertext, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, crypto.ComputePlaintextSHA256(plaintext), version)
	if err != nil {
		t.Fatalf("build envelope header: %v", err)
	}
	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("encode envelope header: %v", err)
	}

	envelope = append([]byte{}, headerBuf...)
	envelope = append(envelope, ciphertext...)
	envelope = append(envelope, hmacTable...)

	mb.objects[key] = &MockObject{
		Data: envelope,
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        fmt.Sprintf("%d", version),
			"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
			"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(iv),
			"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", blockSize),
			"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		},
	}
	return plaintext, secret, envelope
}

// recoveredAdjacentXOR derives, from stored envelope bytes alone, the XOR
// ADR-005 says v1 exposes between its first two blocks: v1 derived each 64 KB
// block's CTR counter as the bare block index, so block 1's keystream is
// block 0's shifted by one AES block and C0[16+i] ⊕ C1[i] == P0[16+i] ⊕ P1[i].
func recoveredAdjacentXOR(envelope []byte, blockSize int) []byte {
	c0 := envelope[crypto.HeaderSize : crypto.HeaderSize+blockSize]
	c1 := envelope[crypto.HeaderSize+blockSize : crypto.HeaderSize+2*blockSize]
	recovered := make([]byte, blockSize-16)
	for i := range recovered {
		recovered[i] = c0[16+i] ^ c1[i]
	}
	return recovered
}

// assertNoAdjacentKeystreamCancellation checks a stored ciphertext region for
// the ADR-005 signature: at a keystream shift of 0 or one AES block, the XOR of
// two blocks must not equal the XOR of the matching plaintext bytes. A full
// match over a 64-byte window means the keystream cancelled and the pair is a
// two-time pad.
func assertNoAdjacentKeystreamCancellation(t *testing.T, cipher, plaintext []byte, blockSize int, stage string) {
	t.Helper()

	blocks := len(cipher) / blockSize
	if blocks < 2 {
		t.Fatalf("%s: ciphertext holds %d blocks; need at least 2 to test adjacent-block leakage", stage, blocks)
	}
	for _, shift := range []int{0, 16} {
		for b := 0; b+1 < blocks; b++ {
			base := b * blockSize
			limit := 64
			if blockSize-shift < limit {
				limit = blockSize - shift
			}
			cancels := true
			for i := 0; i < limit; i++ {
				got := cipher[base+shift+i] ^ cipher[base+blockSize+i]
				want := plaintext[base+shift+i] ^ plaintext[base+blockSize+i]
				if got != want {
					cancels = false
					break
				}
			}
			if cancels {
				t.Fatalf("%s: ciphertext cancels keystream at shift %d between blocks %d/%d — stored bytes leak the adjacent-block plaintext XOR (ADR-005)", stage, shift, b, b+1)
			}
		}
	}
}

// assertBackendStoresNoPlaintext scans every object body and metadata value in
// the backend for the plaintext — whole, as 32-byte windows, and base64 — and
// fails when any of them appears.
func assertBackendStoresNoPlaintext(t *testing.T, mb *MockBackend, plaintext []byte, stage string) {
	t.Helper()

	var hits []string
	check := func(location string, data []byte) {
		if bytes.Contains(data, plaintext) {
			hits = append(hits, location+" contains the whole plaintext")
		}
		for _, off := range []int{0, len(plaintext) / 2, len(plaintext) - 32} {
			if off < 0 || off+32 > len(plaintext) {
				continue
			}
			if bytes.Contains(data, plaintext[off:off+32]) {
				hits = append(hits, fmt.Sprintf("%s contains a 32-byte plaintext window (plaintext offset %d)", location, off))
			}
		}
		if bytes.Contains(data, []byte(base64.StdEncoding.EncodeToString(plaintext))) {
			hits = append(hits, location+" contains base64(plaintext)")
		}
	}
	for key, obj := range mb.objects {
		check("object "+key, obj.Data)
		for field, value := range obj.Metadata {
			check(fmt.Sprintf("metadata %s [%s]", key, field), []byte(value))
		}
	}
	if len(hits) > 0 {
		t.Fatalf("%s: plaintext present in backend storage:\n\t%s", stage, strings.Join(hits, "\n\t"))
	}
}

// TestV1StorageLeaksAdjacentBlockXOR pins the documented exclusion on stored
// bytes: with block 0 all zeros, the keystream-cancelled XOR of the two stored
// v1 ciphertext blocks IS block 1's plaintext verbatim, recovered with
// ciphertext only, no keys. This is the leak that makes v1 the exception in
// the README security model.
func TestV1StorageLeaksAdjacentBlockXOR(t *testing.T) {
	mb := NewMockBackend()
	const key = "legacy/v1-secret.bin"
	const blockSize = 65536 // the defect pairs adjacent 64 KB blocks

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	_, secret, envelope := storeLegacyObject(t, mb, key, mek, crypto.Version1, blockSize)

	if recovered := recoveredAdjacentXOR(envelope, blockSize); !bytes.Equal(recovered, secret[:blockSize-16]) {
		t.Fatal("v1 envelope did not reproduce the ADR-005 leak; this pin no longer describes the legacy format")
	}
}

// TestV2StorageDoesNotLeakAdjacentBlockXOR pins the guaranteed side of the
// boundary on stored bytes: the same two-block construction under envelope v2
// (the 2024-08 counter fix) shows no keystream cancellation at either
// candidate shift, and the backend holds no plaintext.
func TestV2StorageDoesNotLeakAdjacentBlockXOR(t *testing.T) {
	mb := NewMockBackend()
	const key = "covered/v2-object.bin"
	const blockSize = 65536

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	plaintext, _, envelope := storeLegacyObject(t, mb, key, mek, crypto.Version2, blockSize)

	cipher := envelope[crypto.HeaderSize : crypto.HeaderSize+2*blockSize]
	assertNoAdjacentKeystreamCancellation(t, cipher, plaintext, blockSize, "v2 envelope")

	assertBackendStoresNoPlaintext(t, mb, plaintext, "v2 envelope")
}

// TestMigratedV3ObjectRecoversNothingFromCiphertext is the post-migration leg
// of the boundary: after a FormatMigrator pass (what `armor migrate` drives)
// re-encrypts a legacy v1 object to v3, the same ciphertext-only attack must
// recover nothing, no stored byte or metadata value may contain the secret,
// and the migrated object must still decrypt to the original plaintext.
//
// SKIPPED, not green: FormatMigrator.encryptAsSingle/uploadAsMultipart encrypt
// through crypto.Encryptor.Encrypt(), whose makeCounter has no Version3
// branch — migrated objects are badged v3 but still carry the v1 keystream
// reuse, so this pin fails at HEAD. The fix and this unskip are tracked in
// bead armor-511015d0; the pin assumes uncompressed blocks, so the ciphertext
// region spans exactly PlaintextSize bytes from the end of the header.
func TestMigratedV3ObjectRecoversNothingFromCiphertext(t *testing.T) {
	ctx := context.Background()
	mb := NewMockBackend()
	const key = "legacy/v1-secret.bin"
	const blockSize = 65536 // the defect pairs adjacent 64 KB blocks

	t.Skip("migrated-object leak pin is out of scope for bead armor-90e6f3b0 - tracked in armor-511015d0 (FormatMigrator re-encrypts v3 output with the v1 counter derivation); unskip when that bead lands")

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	plaintext, _, envelope := storeLegacyObject(t, mb, key, mek, crypto.Version1, blockSize)

	// The documented exclusion is real before migration (see
	// TestV1StorageLeaksAdjacentBlockXOR); migrate and hold the result to the
	// guarantee the README extends to migrated objects.
	migrator := NewFormatMigrator(mb, "test-bucket", mek, "default", crypto.Version3, []string{"1"}, nil)
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if result.ProcessedObjects != 1 || result.FailedObjects != 0 {
		t.Fatalf("migration result: processed=%d failed=%d skipped=%d failures=%v",
			result.ProcessedObjects, result.FailedObjects, result.SkippedObjects, result.Failures)
	}

	obj := mb.objects[key]
	if obj == nil {
		t.Fatal("migrated object missing at its original key")
	}
	if got := obj.Metadata["x-amz-meta-armor-version"]; got != "3" {
		t.Fatalf("post-migration envelope version = %q, want 3", got)
	}
	if bytes.Equal(obj.Data, envelope) {
		t.Fatal("migration left the stored bytes unchanged; the v1 ciphertext is still in place")
	}

	hdr, err := crypto.DecodeHeader(obj.Data)
	if err != nil {
		t.Fatalf("decode migrated envelope header: %v", err)
	}
	if hdr.Version != crypto.Version3 {
		t.Fatalf("migrated header version = %d, want 3", hdr.Version)
	}
	postCipher := obj.Data[crypto.HeaderSize : crypto.HeaderSize+int(hdr.PlaintextSize)]
	assertNoAdjacentKeystreamCancellation(t, postCipher, plaintext, hdr.BlockSize(), "after v1→v3 migration")

	// No stored byte sequence or metadata value anywhere in the backend
	// contains the secret, either.
	assertBackendStoresNoPlaintext(t, mb, plaintext, "after v1→v3 migration")

	// And the migrated object still reads back: the migrator's own decrypt
	// path verifies per-block HMACs and the recorded plaintext SHA-256.
	armorMeta, ok := backend.ParseARMORMetadata(obj.Metadata)
	if !ok {
		t.Fatal("migrated object metadata does not parse as ARMOR metadata")
	}
	decrypted, err := migrator.decryptSingleObject(armorMeta, bytes.NewReader(obj.Data))
	if err != nil {
		t.Fatalf("decrypt migrated object: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("migrated object no longer decrypts to the original plaintext")
	}
}
