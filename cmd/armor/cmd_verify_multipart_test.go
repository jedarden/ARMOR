// cmd_verify_multipart_test.go tests 'armor verify' against the formats the
// folded command had fallen behind on (armor-da67956d): fingerprinted wrapped
// DEKs with ring fallback (armor-28965aa0), v3 multipart objects through the
// real pipeline (headerless parts + gzip v3 HMAC sidecar + ADR-016 manifest,
// armor-86a90341), report rows, and the non-zero-exit-on-failure contract.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// multipartFixture is a multipart-completed object exactly as it sits in B2
// after CompleteMultipartUpload: headerless concatenated part ciphertext, the
// per-block HMAC sidecar at .armor/hmac/<sha256(client key)>, and the ARMOR
// parameters in metadata (carried by the object itself or only by the
// ADR-016 manifest beside it — B2 drops multipart metadata from the finished
// large file).
type multipartFixture struct {
	clientKey   string
	prefix      string // ADR-001 storage prefix ("" = bucket root)
	blockSize   int
	partSize    int64
	plaintext   []byte
	ciphertext  []byte
	sidecarJSON []byte // v3: gzipped below; v1/v2: stored as-is
	meta        map[string]string
	isV3        bool
	partOffsets []int64 // ciphertext start offset of each part
}

// storedKey is the (prefixed) key the ciphertext is addressed by.
func (f *multipartFixture) storedKey() string {
	return f.prefix + f.clientKey
}

// sidecarObjectKey is where the manager saves the HMAC sidecar:
// .armor/hmac/<sha256(client key)> at the bucket root, named by the UNprefixed
// client key. Computed inline (not via backend.GetSidecarKey) so the fixture
// pins the on-B2 location the manager's Load reads, whatever that helper's
// signature evolves into.
func (f *multipartFixture) sidecarObjectKey() string {
	keyHash := sha256.Sum256([]byte(f.clientKey))
	return fmt.Sprintf(".armor/hmac/%x", keyHash)
}

// createV3MultipartFixture builds a v3 multipart object whose parts are
// encrypted block-by-block with crypto.EncryptBlockV3 — the same wire format
// the server's UploadPart produces (uncompressed blocks: clen is the
// ciphertext length as base64 of a 4-byte big-endian uint32, no compression
// flag). Parts are split at uniform block-aligned partSize boundaries
// (ADR-015); with plaintext larger than partSize this yields multiple parts
// so the per-part HMAC walk is genuinely exercised past part 1.
// declareDigest selects between the real combined per-part digest and the
// legacy empty-string placeholder (bf-1v2ehf, "no digest declared").
func createV3MultipartFixture(t *testing.T, wrapMEK []byte, clientKey string, plaintext []byte, declareDigest bool) *multipartFixture {
	t.Helper()

	const blockSize = 1024
	partSize := int64(blockSize)

	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i ^ 0x55)
	}
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i * 3)
	}
	wrappedDEKStr, err := crypto.WrapDEKWithFingerprint(wrapMEK, dek)
	if err != nil {
		t.Fatalf("WrapDEKWithFingerprint: %v", err)
	}

	f := &multipartFixture{
		clientKey: clientKey,
		blockSize: blockSize,
		partSize:  partSize,
		plaintext: plaintext,
		isV3:      true,
	}

	var parts []backend.HMACPartV3
	for partStart := int64(0); partStart < int64(len(plaintext)); partStart += partSize {
		partEnd := partStart + partSize
		if partEnd > int64(len(plaintext)) {
			partEnd = int64(len(plaintext))
		}
		partPlaintext := plaintext[partStart:partEnd]
		partNum := len(parts) + 1
		f.partOffsets = append(f.partOffsets, int64(len(f.ciphertext)))

		blockCount := crypto.ComputeBlockCount(int64(len(partPlaintext)), blockSize)
		blocks := make([][]string, 0, blockCount)
		for blockIdx := uint32(0); blockIdx < blockCount; blockIdx++ {
			start := int64(blockIdx) * partSize
			end := start + partSize
			if end > int64(len(partPlaintext)) {
				end = int64(len(partPlaintext))
			}
			blockCT, blockHMAC, err := crypto.EncryptBlockV3(dek, iv, uint16(partNum), blockIdx, partPlaintext[start:end], blockSize)
			if err != nil {
				t.Fatalf("EncryptBlockV3 part %d block %d: %v", partNum, blockIdx, err)
			}
			f.ciphertext = append(f.ciphertext, blockCT...)
			lengthBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(lengthBytes, uint32(len(blockCT)))
			blocks = append(blocks, []string{
				base64.StdEncoding.EncodeToString(blockHMAC),
				base64.StdEncoding.EncodeToString(lengthBytes),
			})
		}

		parts = append(parts, backend.HMACPartV3{
			N:             partNum,
			PlaintextLen:  int64(len(partPlaintext)),
			CiphertextLen: int64(len(f.ciphertext)) - f.partOffsets[len(f.partOffsets)-1],
			Blocks:        blocks,
		})
	}

	sidecar := backend.HMACTableSidecarV3{Version: 3, BlockSize: blockSize, Parts: parts}
	f.sidecarJSON, err = json.Marshal(sidecar)
	if err != nil {
		t.Fatalf("marshal v3 sidecar: %v", err)
	}

	declaredSHA := emptyPlaintextSHA256Hex
	if declareDigest {
		declaredSHA = backend.ComputeMultipartDigest(plaintext, partSize)
	}
	f.meta = map[string]string{
		"x-amz-meta-armor-version":          "3",
		"x-amz-meta-armor-block-size":       fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":   fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-iv":               base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-wrapped-dek":      wrappedDEKStr,
		"x-amz-meta-armor-plaintext-sha256": declaredSHA,
		"x-amz-meta-armor-multipart":        "true",
		"x-amz-meta-armor-part-size":        fmt.Sprintf("%d", partSize),
	}
	return f
}

// createV2MultipartFixture builds a v1/v2 multipart object: flat CTR
// ciphertext (Encrypt over the whole plaintext — with block-aligned uniform
// parts that is byte-identical to the per-part EncryptWithStartingCounter
// concatenation) and the flat per-block HMAC table in a plain JSON sidecar.
func createV2MultipartFixture(t *testing.T, wrapMEK []byte, clientKey string, plaintext []byte) *multipartFixture {
	t.Helper()

	const blockSize = 1024
	partSize := int64(blockSize)

	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i ^ 0x5A)
	}
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i * 7)
	}
	wrappedDEKStr, err := crypto.WrapDEKWithFingerprint(wrapMEK, dek)
	if err != nil {
		t.Fatalf("WrapDEKWithFingerprint: %v", err)
	}

	enc, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	encrypted, hmacTable, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	blockHMACs := make([][]byte, 0, len(hmacTable)/crypto.HMACSize)
	for i := 0; i < len(hmacTable); i += crypto.HMACSize {
		blockHMACs = append(blockHMACs, append([]byte(nil), hmacTable[i:i+crypto.HMACSize]...))
	}
	sidecar := backend.HMACTableSidecar{Key: clientKey, BlockHMACs: blockHMACs, BlockSize: blockSize, Version: 2}
	sidecarJSON, err := json.Marshal(sidecar)
	if err != nil {
		t.Fatalf("marshal v2 sidecar: %v", err)
	}

	return &multipartFixture{
		clientKey:   clientKey,
		blockSize:   blockSize,
		partSize:    partSize,
		plaintext:   plaintext,
		ciphertext:  encrypted,
		sidecarJSON: sidecarJSON,
		isV3:        false,
		partOffsets: []int64{0},
		meta: map[string]string{
			"x-amz-meta-armor-version":          "2",
			"x-amz-meta-armor-block-size":       fmt.Sprintf("%d", blockSize),
			"x-amz-meta-armor-plaintext-size":   fmt.Sprintf("%d", len(plaintext)),
			"x-amz-meta-armor-iv":               base64.StdEncoding.EncodeToString(iv),
			"x-amz-meta-armor-wrapped-dek":      wrappedDEKStr,
			"x-amz-meta-armor-plaintext-sha256": backend.ComputeMultipartDigest(plaintext, partSize),
			"x-amz-meta-armor-multipart":        "true",
			"x-amz-meta-armor-part-size":        fmt.Sprintf("%d", partSize),
		},
	}
}

// gzipSidecar compresses the fixture's sidecar JSON the way
// SaveHMACTableV3 stores v3 sidecars.
func (f *multipartFixture) gzipSidecar() []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(f.sidecarJSON); err != nil {
		panic(fmt.Sprintf("gzip sidecar: %v", err))
	}
	if err := gz.Close(); err != nil {
		panic(fmt.Sprintf("gzip close: %v", err))
	}
	return buf.Bytes()
}

// put stores the fixture the way B2 holds it after a successful complete:
// ciphertext under the stored key with the ARMOR parameters in the object's
// own metadata, and the HMAC sidecar at its composed location.
func (f *multipartFixture) put(ctx context.Context, mock *MockB2Backend, bucket string, now time.Time) {
	sidecarBytes := f.sidecarJSON
	if f.isV3 {
		sidecarBytes = f.gzipSidecar()
	}
	mock.PutTestObject(ctx, bucket, f.storedKey(), f.ciphertext, f.meta, true, now)
	mock.PutTestObject(ctx, bucket, f.sidecarObjectKey(), sidecarBytes, nil, false, now)
}

// putHeadEmpty stores the fixture the way B2 ACTUALLY heads a finished
// multipart large file: no x-amz-meta-* keys on the object at all, with the
// ARMOR parameters surviving only in the ADR-016 manifest beside it. When
// manifestHeaders is true the manifest object's own metadata carries the map
// (the server's normal write); otherwise only the manifest JSON body does
// (readManifest's fallback).
func (f *multipartFixture) putHeadEmpty(ctx context.Context, mock *MockB2Backend, bucket string, now time.Time, manifestHeaders bool) {
	sidecarBytes := f.sidecarJSON
	if f.isV3 {
		sidecarBytes = f.gzipSidecar()
	}
	manifestJSON, err := json.Marshal(backend.ManifestBody{
		CiphertextObject: f.storedKey(),
		UploadID:         "upload-1",
		CompletedAt:      now.UTC().Format(time.RFC3339),
		Metadata:         f.meta,
	})
	if err != nil {
		panic(fmt.Sprintf("marshal manifest body: %v", err))
	}
	manifestMeta := map[string]string(nil)
	if manifestHeaders {
		manifestMeta = f.meta
	}

	mock.PutTestObject(ctx, bucket, f.storedKey(), f.ciphertext, map[string]string{}, false, now)
	mock.PutTestObject(ctx, bucket, f.sidecarObjectKey(), sidecarBytes, nil, false, now)
	mock.PutTestObject(ctx, bucket, f.storedKey()+verifyManifestSuffix, manifestJSON, manifestMeta, false, now)
}

// corruptSidecarHMAC rewrites one per-block HMAC inside the v3 sidecar so the
// sidecar loads and parses but the (part, block)-bound HMAC check fails.
func (f *multipartFixture) corruptSidecarHMAC(t *testing.T) {
	t.Helper()
	if !f.isV3 {
		t.Fatalf("corruptSidecarHMAC only implements the v3 sidecar")
	}
	var sidecar backend.HMACTableSidecarV3
	if err := json.Unmarshal(f.sidecarJSON, &sidecar); err != nil {
		t.Fatalf("unmarshal sidecar: %v", err)
	}
	hmacBytes, err := base64.StdEncoding.DecodeString(sidecar.Parts[0].Blocks[0][0])
	if err != nil {
		t.Fatalf("decode block hmac: %v", err)
	}
	hmacBytes[0] ^= 0xFF
	sidecar.Parts[0].Blocks[0][0] = base64.StdEncoding.EncodeToString(hmacBytes)
	f.sidecarJSON, err = json.Marshal(sidecar)
	if err != nil {
		t.Fatalf("remarshal sidecar: %v", err)
	}
}

// corruptPartCiphertext flips a byte inside the given (0-based) part's
// ciphertext, past the header-less part boundary — the corruption the per-part
// walk must catch even when it sits beyond part 1.
func (f *multipartFixture) corruptPartCiphertext(t *testing.T, partIdx int) {
	t.Helper()
	if partIdx >= len(f.partOffsets) {
		t.Fatalf("fixture only has %d parts", len(f.partOffsets))
	}
	off := f.partOffsets[partIdx]
	if off >= int64(len(f.ciphertext)) {
		t.Fatalf("part %d has no ciphertext bytes", partIdx)
	}
	f.ciphertext[off] ^= 0xFF
}

// withPrefix pins the fixture to an ADR-001 shared-bucket prefix: the
// ciphertext and manifest move under <prefix>, while the sidecar stays at the
// bucket root, named by the UNprefixed client key.
func (f *multipartFixture) withPrefix(prefix string) *multipartFixture {
	f.prefix = prefix
	return f
}

// setVerifyPrefixForTest points the verify code's prefix globals at prefix
// for the duration of one test and restores them after.
func setVerifyPrefixForTest(t *testing.T, prefix string) {
	t.Helper()
	orig := b2PrefixFlag
	b2PrefixFlag = prefix
	t.Cleanup(func() { b2PrefixFlag = orig })
}

// === Fingerprinted wrapped DEKs (armor-28965aa0 format) ===

// TestVerifyFingerprintedDEK proves the headline regression fixed by this
// bead: a healthy single-PUT object whose wrapped DEK is stored in the
// v2:<fp16>:<base64> format verifies OK. Before the fix, verify fed the whole
// metadata value to base64-decode + UnwrapDEK and reported every such object
// CORRUPTED.
func TestVerifyFingerprintedDEK(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	plaintext := []byte("plaintext under a fingerprinted DEK")
	data, meta := createFingerprintedSinglePUTObject(t, mek, plaintext)
	mock.PutTestObject(ctx, "test-bucket", "fp-object", data, meta, true, time.Now())

	result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "fp-object", time.Time{})
	if result.Status != "OK" {
		t.Errorf("Expected OK for fingerprint-DEK object, got %s: %s (details: %s)", result.Status, result.Error, result.Details)
	}
}

// createFingerprintedSinglePUTObject builds a healthy v3 single-PUT object
// whose wrapped DEK is stored in the armor-28965aa0 fingerprint format under
// wrapMEK — the on-disk shape of every object the server has written since
// that change landed.
func createFingerprintedSinglePUTObject(t *testing.T, wrapMEK, plaintext []byte) ([]byte, map[string]string) {
	t.Helper()

	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i ^ 0x55)
	}
	wrapped, err := crypto.WrapDEKWithFingerprint(wrapMEK, dek)
	if err != nil {
		t.Fatalf("WrapDEKWithFingerprint: %v", err)
	}
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i * 3)
	}

	const blockSize = 65536
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)

	envelope, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, plaintextSHA, crypto.Version3)
	if err != nil {
		t.Fatalf("NewEnvelopeHeaderWithVersion: %v", err)
	}
	header, err := envelope.Encode()
	if err != nil {
		t.Fatalf("encode header: %v", err)
	}
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, crypto.Version3)
	if err != nil {
		t.Fatalf("NewEncryptorWithVersion: %v", err)
	}
	encrypted, blockTable, err := encryptor.EncryptV3(plaintext, false)
	if err != nil {
		t.Fatalf("EncryptV3: %v", err)
	}
	tableData, err := blockTable.Encode()
	if err != nil {
		t.Fatalf("encode block table: %v", err)
	}
	data := make([]byte, 0, len(header)+len(encrypted)+len(tableData))
	data = append(data, header...)
	data = append(data, encrypted...)
	data = append(data, tableData...)

	meta := map[string]string{
		"x-amz-meta-armor-version":          "3",
		"x-amz-meta-armor-block-size":       fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":   fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-iv":               base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-wrapped-dek":      wrapped,
		"x-amz-meta-armor-plaintext-sha256": hex.EncodeToString(plaintextSHA[:]),
	}
	return data, meta
}

// TestVerifyFingerprintedDEKRingKey proves the ring fallback works exactly
// like the server read path: an object wrapped under a RETIRED MEK that only
// the escrow ring carries verifies OK when the ring is supplied, and reports
// a keying ERROR (not CORRUPTED) when it is not.
func TestVerifyFingerprintedDEKRingKey(t *testing.T) {
	ctx := context.Background()
	activeMEK := generateTestMEK(t)
	retiredMEK := make([]byte, 32)
	for i := range retiredMEK {
		retiredMEK[i] = byte(i ^ 0xAA)
	}
	mock := NewMockB2Backend()

	data, meta := createFingerprintedSinglePUTObject(t, retiredMEK, []byte("wrapped under a retired ring MEK"))
	mock.PutTestObject(ctx, "test-bucket", "ring-object", data, meta, true, time.Now())

	ring := []crypto.RingKeyEntry{{
		MEK:         retiredMEK,
		Fingerprint: crypto.MEKFingerprint(retiredMEK),
	}}

	// With the ring supplied, the object verifies.
	result := verifyObject(ctx, mock, newVerifyKeySource(activeMEK, ring), "test-bucket", "ring-object", time.Time{})
	if result.Status != "OK" {
		t.Errorf("Expected OK for ring-wrapped object with ring supplied, got %s: %s", result.Status, result.Error)
	}

	// Without it, the fingerprint is simply absent from the key set: an ERROR
	// naming the missing fingerprint, not a CORRUPTED verdict on the object.
	result = verifyObject(ctx, mock, newVerifyKeySource(activeMEK, nil), "test-bucket", "ring-object", time.Time{})
	if result.Status != "ERROR" {
		t.Errorf("Expected ERROR for ring-wrapped object without the ring, got %s: %s", result.Status, result.Error)
	}
	if !strings.Contains(result.Error, "fingerprint") {
		t.Errorf("Expected the error to name the missing fingerprint, got: %s", result.Error)
	}
}

// TestVerifyLegacyWrappedDEKStillVerifies pins the legacy path: plain base64
// wrapped DEKs (pre-fingerprint objects) keep verifying with the active MEK.
func TestVerifyLegacyWrappedDEKStillVerifies(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	validData := createValidARMORObject(t, mek, "legacy-object", []byte("legacy plaintext"))
	if strings.HasPrefix(validData.metadata["x-amz-meta-armor-wrapped-dek"], "v2:") {
		t.Fatalf("fixture should produce a legacy (unfingerprinted) wrapped DEK")
	}
	mock.PutTestObject(ctx, "test-bucket", "legacy-object", validData.data, validData.metadata, true, time.Now())

	result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "legacy-object", time.Time{})
	if result.Status != "OK" {
		t.Errorf("Expected OK for legacy wrapped DEK, got %s: %s", result.Status, result.Error)
	}
}

// === v3 multipart objects (armor-86a90341 coordination) ===

// TestVerifyV3MultipartMultiPart walks a genuinely multi-part object: several
// parts, per-part block counters, combined per-part digest enforced.
func TestVerifyV3MultipartMultiPart(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	plaintext := bytes.Repeat([]byte("v3-multipart-walk-"), 200) // 3600 bytes -> 4 parts at 1024
	fix := createV3MultipartFixture(t, mek, "v3-multi", plaintext, true)
	fix.put(ctx, mock, "test-bucket", time.Now())

	result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "v3-multi", time.Time{})
	if result.Status != "OK" {
		t.Errorf("Expected OK for multi-part v3 object, got %s: %s (details: %s)", result.Status, result.Error, result.Details)
	}
}

// TestVerifyV3MultipartManifestFallback is the armor-86a90341 scenario: B2
// heads the finished large file with NO metadata; the ADR-016 manifest beside
// the object is the only surviving source of the ARMOR parameters. Both
// manifest read modes (headers, JSON body) must resolve.
func TestVerifyV3MultipartManifestFallback(t *testing.T) {
	for _, manifestHeaders := range []bool{true, false} {
		label := "body"
		if manifestHeaders {
			label = "headers"
		}
		t.Run(label, func(t *testing.T) {
			ctx := context.Background()
			mek := generateTestMEK(t)
			mock := NewMockB2Backend()

			plaintext := bytes.Repeat([]byte("manifest-fallback-"), 150)
			fix := createV3MultipartFixture(t, mek, "manifest-object", plaintext, true)
			fix.putHeadEmpty(ctx, mock, "test-bucket", time.Now(), manifestHeaders)

			result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "manifest-object", time.Time{})
			if result.Status != "OK" {
				t.Errorf("Expected OK via manifest fallback (headers=%v), got %s: %s (details: %s)",
					manifestHeaders, result.Status, result.Error, result.Details)
			}
		})
	}
}

// TestVerifyV3MultipartCorruptedPart catches corruption in a part OTHER than
// the first — the case a flat single DecryptV3 call cannot see.
func TestVerifyV3MultipartCorruptedPart(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	plaintext := bytes.Repeat([]byte("corrupt-a-later-part-"), 180)
	fix := createV3MultipartFixture(t, mek, "v3-later-part", plaintext, true)
	fix.corruptPartCiphertext(t, len(fix.partOffsets)-1)
	fix.put(ctx, mock, "test-bucket", time.Now())

	result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "v3-later-part", time.Time{})
	if result.Status != "CORRUPTED" {
		t.Errorf("Expected CORRUPTED for later-part corruption, got %s: %s (details: %s)", result.Status, result.Error, result.Details)
	}
}

// TestVerifyV3MultipartDigestMismatch flips the declared digest so the
// combined per-part comparison fails even though every HMAC verified.
func TestVerifyV3MultipartDigestMismatch(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	plaintext := bytes.Repeat([]byte("digest-mismatch-"), 170)
	fix := createV3MultipartFixture(t, mek, "v3-digest", plaintext, true)
	fix.meta["x-amz-meta-armor-plaintext-sha256"] = strings.Repeat("0", 64)
	fix.put(ctx, mock, "test-bucket", time.Now())

	result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "v3-digest", time.Time{})
	if result.Status != "CORRUPTED" {
		t.Errorf("Expected CORRUPTED for digest mismatch, got %s: %s", result.Status, result.Error)
	}
	if !strings.Contains(result.Error, "SHA-256 mismatch") {
		t.Errorf("Expected SHA-256 mismatch error, got: %s", result.Error)
	}
}

// TestVerifyV3MultipartPlaceholderDigestSkipped pins the legacy placeholder
// handling: a declared empty-string SHA means "no digest declared" and must
// not fail the object (bf-1v2ehf).
func TestVerifyV3MultipartPlaceholderDigestSkipped(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	plaintext := bytes.Repeat([]byte("placeholder-digest-"), 160)
	fix := createV3MultipartFixture(t, mek, "v3-placeholder", plaintext, false)
	fix.put(ctx, mock, "test-bucket", time.Now())

	result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "v3-placeholder", time.Time{})
	if result.Status != "OK" {
		t.Errorf("Expected OK with placeholder digest, got %s: %s", result.Status, result.Error)
	}
}

// TestVerifyV3MultipartQuickMode checks quick mode on a multipart object:
// DEK unwrap plus sidecar load/parse/size-accounting, no body decrypt.
func TestVerifyV3MultipartQuickMode(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	plaintext := bytes.Repeat([]byte("quick-multipart-"), 150)
	fix := createV3MultipartFixture(t, mek, "v3-quick", plaintext, true)

	origQuick := quickModeFlag
	quickModeFlag = true
	t.Cleanup(func() { quickModeFlag = origQuick })

	fix.put(ctx, mock, "test-bucket", time.Now())

	result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "v3-quick", time.Time{})
	if result.Status != "OK" {
		t.Errorf("Expected OK for quick-mode multipart, got %s: %s", result.Status, result.Error)
	}
	if !strings.Contains(result.Details, "sidecar") {
		t.Errorf("Expected sidecar detail on quick-mode result, got: %s", result.Details)
	}
}

// TestVerifyV2Multipart covers the v1/v2 multipart layout: flat ciphertext,
// plain JSON sidecar, combined digest.
func TestVerifyV2Multipart(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	plaintext := bytes.Repeat([]byte("v2-multipart-body-"), 150)
	fix := createV2MultipartFixture(t, mek, "v2-multi", plaintext)
	fix.put(ctx, mock, "test-bucket", time.Now())

	result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "v2-multi", time.Time{})
	if result.Status != "OK" {
		t.Errorf("Expected OK for v2 multipart, got %s: %s (details: %s)", result.Status, result.Error, result.Details)
	}
}

// TestVerifyV2MultipartQuickModeSizeMismatch covers the only quick-mode
// structural check a v1/v2 multipart object gets: CTR ciphertext length must
// equal the declared plaintext size byte for byte, so a mismatch is CORRUPTED
// even though quick mode never decrypts or HMAC-checks the body.
func TestVerifyV2MultipartQuickModeSizeMismatch(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	plaintext := bytes.Repeat([]byte("v2-quick-size-"), 150)
	fix := createV2MultipartFixture(t, mek, "v2-quick-size", plaintext)
	// Declare one more plaintext byte than the ciphertext holds.
	fix.meta["x-amz-meta-armor-plaintext-size"] = fmt.Sprintf("%d", len(plaintext)+1)

	origQuick := quickModeFlag
	quickModeFlag = true
	t.Cleanup(func() { quickModeFlag = origQuick })

	fix.put(ctx, mock, "test-bucket", time.Now())

	result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "v2-quick-size", time.Time{})
	if result.Status != "CORRUPTED" {
		t.Errorf("Expected CORRUPTED for quick-mode v2 size mismatch, got %s: %s", result.Status, result.Error)
	}
	if !strings.Contains(result.Error, "plaintext size") {
		t.Errorf("Expected a plaintext-size error, got: %s", result.Error)
	}
}

// TestVerifyMultipartPrefixedBucket proves the ADR-001 prefix handling on
// every lookup: ciphertext under the prefixed stored key, sidecar at the
// bucket-root .armor/hmac/ named by the UNprefixed client key, manifest
// beside the prefixed key, and a head-empty object that resolves through the
// manifest (armor-1b272971 class).
func TestVerifyMultipartPrefixedBucket(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	setVerifyPrefixForTest(t, "tenant-a/")

	plaintext := bytes.Repeat([]byte("prefixed-multipart-"), 160)
	fix := createV3MultipartFixture(t, mek, "backups/data.tar.gz", plaintext, true)
	fix.withPrefix("tenant-a/")
	fix.putHeadEmpty(ctx, mock, "test-bucket", time.Now(), true)

	result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "tenant-a/backups/data.tar.gz", time.Time{})
	if result.Status != "OK" {
		t.Errorf("Expected OK for prefixed multipart object, got %s: %s (details: %s)", result.Status, result.Error, result.Details)
	}
}

// TestVerifyMultipartMissingSidecar pins the failure shape when the sidecar
// is absent: an ERROR (the object cannot be verified), never a false OK.
func TestVerifyMultipartMissingSidecar(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	plaintext := bytes.Repeat([]byte("missing-sidecar-"), 150)
	fix := createV3MultipartFixture(t, mek, "v3-nosidecar", plaintext, true)

	// Store only the object and its metadata — no sidecar, no manifest.
	mock.PutTestObject(ctx, "test-bucket", "v3-nosidecar", fix.ciphertext, fix.meta, true, time.Now())

	result := verifyObject(ctx, mock, newVerifyKeySource(mek, nil), "test-bucket", "v3-nosidecar", time.Time{})
	if result.Status != "ERROR" {
		t.Errorf("Expected ERROR for missing sidecar, got %s: %s", result.Status, result.Error)
	}
	if result.Error == "" {
		t.Errorf("Expected a non-empty Error on the missing-sidecar failure")
	}
}

// === Report rows and exit code ===

// TestRunVerificationReportRows pins the report contract: one row per key,
// rows sorted by key, non-empty Error on every failing row, counters in
// agreement, and a non-zero exit code for the same report.
func TestRunVerificationReportRows(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// a-ok: healthy v2 object.
	okData := createValidARMORObject(t, mek, "a-ok", []byte("fine"))
	mock.PutTestObject(ctx, "test-bucket", "a-ok", okData.data, okData.metadata, true, time.Now())

	// b-corrupted: valid envelope with a flipped ciphertext byte.
	badData := createValidARMORObject(t, mek, "b-corrupted", []byte("broken"))
	envelope, err := crypto.DecodeHeader(badData.data)
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	blockCount := int(crypto.ComputeBlockCount(int64(envelope.PlaintextSize), envelope.BlockSize()))
	hmacTableSize := blockCount * crypto.HMACSize
	badData.data[crypto.HeaderSize+((len(badData.data)-crypto.HeaderSize-hmacTableSize)/2)] ^= 0xFF
	mock.PutTestObject(ctx, "test-bucket", "b-corrupted", badData.data, badData.metadata, true, time.Now())

	// c-missing: object that is not ARMOR-encrypted at all.
	mock.PutTestObject(ctx, "test-bucket", "c-plain", []byte("not armor"), map[string]string{}, false, time.Now())

	report := runVerification(ctx, mock, newVerifyKeySource(mek, nil), []string{"a-ok", "b-corrupted", "c-plain"}, time.Time{})

	if len(report.Results) != 3 {
		t.Fatalf("Expected 3 result rows, got %d", len(report.Results))
	}
	if report.Results[0].Key != "a-ok" || report.Results[1].Key != "b-corrupted" || report.Results[2].Key != "c-plain" {
		t.Errorf("Expected rows sorted by key, got %s, %s, %s",
			report.Results[0].Key, report.Results[1].Key, report.Results[2].Key)
	}
	for _, row := range report.Results {
		if row.Status == "OK" {
			continue
		}
		if row.Error == "" {
			t.Errorf("Failing row %s carries an empty Error", row.Key)
		}
	}
	if report.Results[1].Status != "CORRUPTED" {
		t.Errorf("Expected b-corrupted row CORRUPTED, got %s (error: %s)", report.Results[1].Status, report.Results[1].Error)
	}
	if report.Results[2].Status != "ERROR" {
		t.Errorf("Expected c-plain row ERROR, got %s (error: %s)", report.Results[2].Status, report.Results[2].Error)
	}
	if report.OKCount != 1 || report.CorruptedCount != 1 || report.ErrorCount != 1 {
		t.Errorf("Expected counts 1/1/1, got %d/%d/%d", report.OKCount, report.CorruptedCount, report.ErrorCount)
	}
	if code := verificationExitCode(report); code != 1 {
		t.Errorf("Expected exit code 1 for a report with failures, got %d", code)
	}
}

// TestVerificationExitCode pins the exit-code contract directly: any
// CORRUPTED or ERROR verdict makes the process exit non-zero; only an
// all-OK (or empty) report exits 0.
func TestVerificationExitCode(t *testing.T) {
	cases := []struct {
		name   string
		report *VerificationReport
		want   int
	}{
		{"all ok", &VerificationReport{OKCount: 5}, 0},
		{"empty", &VerificationReport{}, 0},
		{"corrupted", &VerificationReport{OKCount: 4, CorruptedCount: 1}, 1},
		{"error", &VerificationReport{OKCount: 4, ErrorCount: 1}, 1},
		{"both", &VerificationReport{CorruptedCount: 2, ErrorCount: 3}, 1},
	}
	for _, tc := range cases {
		if got := verificationExitCode(tc.report); got != tc.want {
			t.Errorf("%s: expected exit code %d, got %d", tc.name, tc.want, got)
		}
	}
}
