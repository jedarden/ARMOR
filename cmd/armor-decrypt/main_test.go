// Unit tests for armor-decrypt CLI tool.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// Test helpers

func makeMEK(t *testing.T) []byte {
	t.Helper()
	mek, err := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	return mek
}

// emptyStringSHA256Hex is the SHA-256 of the empty string — the placeholder
// digest CompleteMultipartUpload stores for multipart objects (ADR-003 gap
// bf-1v2ehf). Tests mirror reality by storing this, not the true whole-object SHA.
const emptyStringSHA256Hex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// Test encrypt-decrypt round-trip via internal/crypto, decrypt via tool code path
func TestRoundTripSinglePart(t *testing.T) {
	mek := makeMEK(t)

	// Generate DEK and encrypt using internal/crypto
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}

	plaintext := []byte("Hello, ARMOR! This is test data for round-trip encryption.")
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)

	blockSize := 65536

	// Create encryptor
	encryptor, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	// Encrypt the plaintext
	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Wrap the DEK
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	// Build envelope header
	header, err := crypto.NewEnvelopeHeader(iv, int64(len(plaintext)), blockSize, plaintextSHA)
	if err != nil {
		t.Fatalf("NewEnvelopeHeader: %v", err)
	}

	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("header.Encode: %v", err)
	}

	// Assemble full envelope file
	var envelope bytes.Buffer
	envelope.Write(headerBuf)
	envelope.Write(encrypted)
	envelope.Write(hmacTable)

	// Now decrypt via the tool's decryptLocal code path
	tmpDir := t.TempDir()
	encryptedFile := filepath.Join(tmpDir, "encrypted.bin")
	if err := os.WriteFile(encryptedFile, envelope.Bytes(), 0644); err != nil {
		t.Fatalf("write encrypted file: %v", err)
	}

	// Create inputSource for local file
	src := &inputSource{
		Type:       "local",
		Path:       encryptedFile,
		WrappedDEK: wrappedDEK,
	}

	// Use decryptLocal function (no sidecar => single-PUT envelope path)
	sidecarFlag = ""
	defer func() { sidecarFlag = "" }()

	ctx := context.Background()
	decrypted, err := decryptLocal(ctx, src, mek)
	if err != nil {
		t.Fatalf("decryptLocal: %v", err)
	}

	// Verify the plaintext matches
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data mismatch:\n got: %q\nwant: %q", decrypted, plaintext)
	}
}

// buildMultipartFixture builds a real ADR-003 multipart object: headerless raw
// ciphertext (no envelope header) plus a JSON HMAC sidecar in the exact
// HMACTableSidecar wire format the server writes at .armor/hmac/<sha256(key)>.
// It mirrors what CompleteMultipartUpload produces (per-block HMACs split out of
// the Encryptor's flattened table), so the round-trip through armor-decrypt
// matches what real stored objects require.
func buildMultipartFixture(t *testing.T, mek []byte, blockSize int, plaintext []byte) (ciphertext, sidecarJSON []byte, wrappedDEK, iv []byte) {
	t.Helper()
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}
	iv, err = crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}
	wrappedDEK, err = crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}
	enc, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	encrypted, hmacTable, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Multipart body is raw ciphertext only — no header, no trailing HMAC.
	// Split the flattened HMAC table into one entry per block, exactly as the
	// server's SaveHMACTable stores them in the JSON sidecar.
	blockHMACs := make([][]byte, 0, len(hmacTable)/crypto.HMACSize)
	for i := 0; i < len(hmacTable); i += crypto.HMACSize {
		blockHMACs = append(blockHMACs, append([]byte(nil), hmacTable[i:i+crypto.HMACSize]...))
	}
	sidecarObj := backend.HMACTableSidecar{
		Key:        "replica/db.snapshot",
		BlockHMACs: blockHMACs,
		BlockSize:  blockSize,
	}
	sidecarJSON, err = json.Marshal(sidecarObj)
	if err != nil {
		t.Fatalf("marshal sidecar: %v", err)
	}
	return encrypted, sidecarJSON, wrappedDEK, iv
}

// TestRoundTripLocalMultipart decrypts a local multipart object (headerless
// ciphertext + JSON sidecar + IV) through decryptLocal, the ADR-003 layout that
// armor-decrypt previously could not read at all (bf-5jc1j8).
func TestRoundTripLocalMultipart(t *testing.T) {
	mek := makeMEK(t)
	blockSize := 65536

	// Plaintext spanning several blocks with a partial final block, like a real
	// (non-block-aligned) backup tail.
	plaintext := make([]byte, blockSize*3+1234)
	for i := range plaintext {
		plaintext[i] = byte(i)
	}

	ciphertext, sidecarJSON, wrappedDEK, iv := buildMultipartFixture(t, mek, blockSize, plaintext)

	tmpDir := t.TempDir()
	ctFile := filepath.Join(tmpDir, "object.bin")
	sidecarFile := filepath.Join(tmpDir, "object.hmac.json")
	if err := os.WriteFile(ctFile, ciphertext, 0644); err != nil {
		t.Fatalf("write ciphertext: %v", err)
	}
	if err := os.WriteFile(sidecarFile, sidecarJSON, 0644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	// Multipart local mode: -sidecar present, -iv supplied (no header to read it from).
	sidecarFlag = sidecarFile
	ivFlag = hex.EncodeToString(iv)
	defer func() { sidecarFlag = ""; ivFlag = "" }()

	src := &inputSource{
		Type:       "local",
		Path:       ctFile,
		WrappedDEK: wrappedDEK,
	}

	decrypted, err := decryptLocal(context.Background(), src, mek)
	if err != nil {
		t.Fatalf("decryptLocal multipart: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("multipart plaintext mismatch: got %d bytes, want %d", len(decrypted), len(plaintext))
	}
}

// TestRoundTripLocalMultipartCorrupted flips a ciphertext byte and asserts the
// per-block HMAC verification catches it — the corruption-detection guarantee
// must hold on the multipart path, not just the single-PUT path.
func TestRoundTripLocalMultipartCorrupted(t *testing.T) {
	mek := makeMEK(t)
	blockSize := 65536
	plaintext := make([]byte, blockSize*2+500)
	for i := range plaintext {
		plaintext[i] = byte(i)
	}

	ciphertext, sidecarJSON, wrappedDEK, iv := buildMultipartFixture(t, mek, blockSize, plaintext)

	// Corrupt one ciphertext byte (not the HMAC sidecar).
	ciphertext[42] ^= 0xff

	tmpDir := t.TempDir()
	ctFile := filepath.Join(tmpDir, "object.bin")
	sidecarFile := filepath.Join(tmpDir, "object.hmac.json")
	if err := os.WriteFile(ctFile, ciphertext, 0644); err != nil {
		t.Fatalf("write ciphertext: %v", err)
	}
	if err := os.WriteFile(sidecarFile, sidecarJSON, 0644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	sidecarFlag = sidecarFile
	ivFlag = hex.EncodeToString(iv)
	defer func() { sidecarFlag = ""; ivFlag = "" }()

	src := &inputSource{
		Type:       "local",
		Path:       ctFile,
		WrappedDEK: wrappedDEK,
	}

	_, err := decryptLocal(context.Background(), src, mek)
	if err == nil {
		t.Fatal("expected HMAC error for corrupted multipart ciphertext, got nil")
	}
	if !strings.Contains(err.Error(), "HMAC") && !strings.Contains(err.Error(), "decrypt blocks") {
		t.Errorf("error should report HMAC/decrypt failure, got: %v", err)
	}
}

// TestDecryptLocalMultipartMissingIV verifies the tool refuses a multipart local
// object without an IV, with a clear message — a multipart object has no header
// to recover the IV from.
func TestDecryptLocalMultipartMissingIV(t *testing.T) {
	mek := makeMEK(t)
	blockSize := 65536
	plaintext := []byte("small multipart object")
	ciphertext, sidecarJSON, wrappedDEK, _ := buildMultipartFixture(t, mek, blockSize, plaintext)

	tmpDir := t.TempDir()
	ctFile := filepath.Join(tmpDir, "object.bin")
	sidecarFile := filepath.Join(tmpDir, "object.hmac.json")
	if err := os.WriteFile(ctFile, ciphertext, 0644); err != nil {
		t.Fatalf("write ciphertext: %v", err)
	}
	if err := os.WriteFile(sidecarFile, sidecarJSON, 0644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	sidecarFlag = sidecarFile
	ivFlag = "" // missing on purpose
	defer func() { sidecarFlag = ""; ivFlag = "" }()

	src := &inputSource{Type: "local", Path: ctFile, WrappedDEK: wrappedDEK}
	_, err := decryptLocal(context.Background(), src, mek)
	if err == nil {
		t.Fatal("expected error for multipart local without -iv, got nil")
	}
	if !strings.Contains(err.Error(), "-iv") {
		t.Errorf("error should mention -iv, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// B2 path: a fake backend serves fixture objects and JSON HMAC sidecars, so the
// full decryptB2 dispatch (marker -> readB2{Envelope,Multipart}Ciphertext) runs
// without B2 credentials. This is the actual DR recovery path.
// ---------------------------------------------------------------------------

// fakeB2Object is one stored object: raw bytes (envelope or headerless
// ciphertext for data objects; JSON sidecar bytes for .armor/hmac/* objects)
// plus its metadata.
type fakeB2Object struct {
	body     []byte
	metadata map[string]string
}

// fakeB2Backend serves fixture objects by key. Embedding backend.Backend
// satisfies the rest of the interface with nil stubs decryptB2 never calls.
type fakeB2Backend struct {
	backend.Backend
	objects map[string]*fakeB2Object
}

func (f *fakeB2Backend) Head(_ context.Context, _, key string) (*backend.ObjectInfo, error) {
	obj, ok := f.objects[key]
	if !ok {
		return nil, fmt.Errorf("NoSuchKey: %s", key)
	}
	return &backend.ObjectInfo{
		Key:              key,
		Size:             int64(len(obj.body)),
		Metadata:         obj.metadata,
		IsARMOREncrypted: true,
	}, nil
}

func (f *fakeB2Backend) GetRange(_ context.Context, _, key string, offset, length int64) (io.ReadCloser, error) {
	obj, ok := f.objects[key]
	if !ok {
		return nil, fmt.Errorf("NoSuchKey: %s", key)
	}
	end := offset + length
	if offset < 0 {
		offset = 0
	}
	if end > int64(len(obj.body)) {
		end = int64(len(obj.body))
	}
	if offset > int64(len(obj.body)) || offset > end {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	return io.NopCloser(bytes.NewReader(obj.body[offset:end])), nil
}

// GetDirect serves the JSON HMAC sidecar. The key is the sidecar object name
// ".armor/hmac/<hex(sha256(key))>" that MultipartStateManager.LoadHMACTable fetches.
func (f *fakeB2Backend) GetDirect(_ context.Context, _, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	obj, ok := f.objects[key]
	if !ok {
		return nil, nil, fmt.Errorf("NoSuchKey: %s", key)
	}
	return io.NopCloser(bytes.NewReader(obj.body)), &backend.ObjectInfo{Key: key, Size: int64(len(obj.body))}, nil
}

// sidecarKeyFor mirrors MultipartStateManager's sidecar object name.
func sidecarKeyFor(key string) string {
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf(".armor/hmac/%s", hex.EncodeToString(h[:]))
}

// withFakeBackend swaps b2BackendFactory for one serving the given objects, runs
// fn, and restores the real factory. This is what lets decryptB2 run in-unit-test.
func withFakeBackend(t *testing.T, objects map[string]*fakeB2Object, fn func()) {
	t.Helper()
	prev := b2BackendFactory
	b2BackendFactory = func(_ context.Context) (backend.Backend, error) {
		return &fakeB2Backend{objects: objects}, nil
	}
	defer func() { b2BackendFactory = prev }()
	fn()
}

// storeMultipartObjects registers a multipart object and its JSON sidecar in the
// fake backend under the keys decryptB2 / LoadHMACTable look them up.
func storeMultipartObjects(objects map[string]*fakeB2Object, key string, ciphertext, sidecarJSON, wrappedDEK, iv []byte, blockSize int, plaintextSize int64) {
	meta := (&backend.ARMORMetadata{
		Version:       1,
		BlockSize:     blockSize,
		PlaintextSize: plaintextSize,
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		PlaintextSHA:  emptyStringSHA256Hex, // multipart placeholder (bf-1v2ehf)
	}).ToMetadata()
	meta["x-amz-meta-armor-multipart"] = "true"
	objects[key] = &fakeB2Object{body: ciphertext, metadata: meta}
	objects[sidecarKeyFor(key)] = &fakeB2Object{body: sidecarJSON}
}

// TestDecryptB2Multipart is the core regression for bf-5jc1j8: decryptB2 must
// dispatch on the multipart marker, read headerless ciphertext from offset 0, and
// load the HMAC sidecar — not decode a 64-byte envelope header from raw
// ciphertext (which fails with "invalid ARMOR magic").
func TestDecryptB2Multipart(t *testing.T) {
	mek := makeMEK(t)
	blockSize := 65536
	// Multi-part-sized plaintext (>= 5 MiB like a real backup part), partial tail.
	plaintext := make([]byte, blockSize*90+777)
	for i := range plaintext {
		plaintext[i] = byte(i)
	}

	ciphertext, sidecarJSON, wrappedDEK, iv := buildMultipartFixture(t, mek, blockSize, plaintext)
	const key = "replica/db.snapshot"

	objects := map[string]*fakeB2Object{}
	storeMultipartObjects(objects, key, ciphertext, sidecarJSON, wrappedDEK, iv, blockSize, int64(len(plaintext)))

	src := &inputSource{Type: "b2", Bucket: "bucket", Path: key}

	var decrypted []byte
	withFakeBackend(t, objects, func() {
		var err error
		decrypted, err = decryptB2(context.Background(), src, mek, "")
		if err != nil {
			t.Fatalf("decryptB2 multipart: %v", err)
		}
	})

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("B2 multipart plaintext mismatch: got %d bytes, want %d", len(decrypted), len(plaintext))
	}
}

// TestDecryptB2MultipartCorrupted asserts the B2 multipart path still detects
// ciphertext corruption via per-block HMAC verification.
func TestDecryptB2MultipartCorrupted(t *testing.T) {
	mek := makeMEK(t)
	blockSize := 65536
	plaintext := make([]byte, blockSize*5+100)
	for i := range plaintext {
		plaintext[i] = byte(i)
	}

	ciphertext, sidecarJSON, wrappedDEK, iv := buildMultipartFixture(t, mek, blockSize, plaintext)
	// Corrupt a byte in the second block (past the first) so block-0 still passes.
	ciphertext[blockSize+10] ^= 0xff

	const key = "replica/db.snapshot"
	objects := map[string]*fakeB2Object{}
	storeMultipartObjects(objects, key, ciphertext, sidecarJSON, wrappedDEK, iv, blockSize, int64(len(plaintext)))

	src := &inputSource{Type: "b2", Bucket: "bucket", Path: key}

	withFakeBackend(t, objects, func() {
		_, err := decryptB2(context.Background(), src, mek, "")
		if err == nil {
			t.Fatal("expected HMAC error for corrupted B2 multipart ciphertext, got nil")
		}
		if !strings.Contains(err.Error(), "HMAC") && !strings.Contains(err.Error(), "decrypt blocks") {
			t.Errorf("error should report HMAC/decrypt failure, got: %v", err)
		}
	})
}

// TestDecryptB2SinglePUT is the single-PUT regression: decryptB2 must still read
// the envelope header + inline HMAC table for non-multipart objects.
func TestDecryptB2SinglePUT(t *testing.T) {
	mek := makeMEK(t)
	blockSize := 65536
	plaintext := []byte("Single-PUT object over B2 — envelope header + inline HMAC table.")

	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}
	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)
	header, err := crypto.NewEnvelopeHeader(iv, int64(len(plaintext)), blockSize, plaintextSHA)
	if err != nil {
		t.Fatalf("NewEnvelopeHeader: %v", err)
	}
	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("header.Encode: %v", err)
	}
	encryptor, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	envelope := make([]byte, 0, len(headerBuf)+len(encrypted)+len(hmacTable))
	envelope = append(envelope, headerBuf...)
	envelope = append(envelope, encrypted...)
	envelope = append(envelope, hmacTable...)

	meta := (&backend.ARMORMetadata{
		Version:       1,
		BlockSize:     blockSize,
		PlaintextSize: int64(len(plaintext)),
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		PlaintextSHA:  hex.EncodeToString(plaintextSHA[:]),
	}).ToMetadata()

	const key = "path/to/single.bin"
	objects := map[string]*fakeB2Object{
		key: {body: envelope, metadata: meta},
	}

	src := &inputSource{Type: "b2", Bucket: "bucket", Path: key}

	var decrypted []byte
	withFakeBackend(t, objects, func() {
		var err error
		decrypted, err = decryptB2(context.Background(), src, mek, "")
		if err != nil {
			t.Fatalf("decryptB2 single-PUT: %v", err)
		}
	})

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("B2 single-PUT plaintext mismatch: got %q, want %q", decrypted, plaintext)
	}
}

// Test wrong MEK produces clear error
func TestWrongMEK(t *testing.T) {
	mek := makeMEK(t)

	// Generate DEK and encrypt with correct MEK
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}

	plaintext := []byte("Secret data")
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)

	blockSize := 65536

	encryptor, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	header, err := crypto.NewEnvelopeHeader(iv, int64(len(plaintext)), blockSize, plaintextSHA)
	if err != nil {
		t.Fatalf("NewEnvelopeHeader: %v", err)
	}

	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("header.Encode: %v", err)
	}

	// Assemble envelope
	var envelope bytes.Buffer
	envelope.Write(headerBuf)
	envelope.Write(encrypted)
	envelope.Write(hmacTable)

	// Write encrypted file
	tmpDir := t.TempDir()
	encryptedFile := filepath.Join(tmpDir, "encrypted.bin")
	if err := os.WriteFile(encryptedFile, envelope.Bytes(), 0644); err != nil {
		t.Fatalf("write encrypted file: %v", err)
	}

	// Try to decrypt with WRONG MEK
	wrongMEK, _ := hex.DecodeString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

	src := &inputSource{
		Type:       "local",
		Path:       encryptedFile,
		WrappedDEK: wrappedDEK,
	}

	sidecarFlag = ""
	defer func() { sidecarFlag = "" }()

	ctx := context.Background()
	_, err = decryptLocal(ctx, src, wrongMEK)
	if err == nil {
		t.Fatal("expected error with wrong MEK, got nil")
	}

	// Check error message is clear
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unwrap DEK") {
		t.Errorf("error message should mention unwrap failure, got: %v", errMsg)
	}
}

// Test corrupted block produces clear error
func TestCorruptedBlock(t *testing.T) {
	mek := makeMEK(t)

	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}

	plaintext := []byte("Data that will be corrupted")
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)

	blockSize := 65536

	encryptor, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// CORRUPT the encrypted data
	encrypted[0] ^= 0xff

	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	header, err := crypto.NewEnvelopeHeader(iv, int64(len(plaintext)), blockSize, plaintextSHA)
	if err != nil {
		t.Fatalf("NewEnvelopeHeader: %v", err)
	}

	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("header.Encode: %v", err)
	}

	// Assemble envelope
	var envelope bytes.Buffer
	envelope.Write(headerBuf)
	envelope.Write(encrypted)
	envelope.Write(hmacTable)

	tmpDir := t.TempDir()
	encryptedFile := filepath.Join(tmpDir, "encrypted.bin")
	if err := os.WriteFile(encryptedFile, envelope.Bytes(), 0644); err != nil {
		t.Fatalf("write encrypted file: %v", err)
	}

	src := &inputSource{
		Type:       "local",
		Path:       encryptedFile,
		WrappedDEK: wrappedDEK,
	}

	sidecarFlag = ""
	defer func() { sidecarFlag = "" }()

	ctx := context.Background()
	_, err = decryptLocal(ctx, src, mek)
	if err == nil {
		t.Fatal("expected error with corrupted block, got nil")
	}

	// Check error message mentions HMAC
	errMsg := err.Error()
	if !strings.Contains(errMsg, "HMAC") {
		t.Errorf("error message should mention HMAC failure, got: %v", errMsg)
	}
}

// Test getInputSource dispatches local single-PUT vs B2.
func TestGetInputSource(t *testing.T) {
	oldInput := inputFlag
	oldWrapped := wrappedDEKFlag
	oldBucket := b2BucketFlag
	defer func() {
		inputFlag = oldInput
		wrappedDEKFlag = oldWrapped
		b2BucketFlag = oldBucket
	}()

	t.Run("B2 URL", func(t *testing.T) {
		inputFlag = "b2://my-bucket/path/to/file"
		wrappedDEKFlag = ""
		b2BucketFlag = ""

		src, err := getInputSource()
		if err != nil {
			t.Fatalf("getInputSource: %v", err)
		}

		if src.Type != "b2" {
			t.Errorf("expected type b2, got %s", src.Type)
		}
		if src.Bucket != "my-bucket" {
			t.Errorf("expected bucket my-bucket, got %s", src.Bucket)
		}
		if src.Path != "path/to/file" {
			t.Errorf("expected path path/to/file, got %s", src.Path)
		}
	})

	t.Run("local file with wrapped DEK", func(t *testing.T) {
		inputFlag = "/path/to/file.bin"
		wrappedDEKFlag = "dGVzdA==" // "test" in base64
		b2BucketFlag = ""

		src, err := getInputSource()
		if err != nil {
			t.Fatalf("getInputSource: %v", err)
		}

		if src.Type != "local" {
			t.Errorf("expected type local, got %s", src.Type)
		}
		if src.Path != "/path/to/file.bin" {
			t.Errorf("expected path /path/to/file.bin, got %s", src.Path)
		}
		if src.WrappedDEK == nil || len(src.WrappedDEK) != 4 {
			t.Errorf("wrapped DEK not decoded correctly")
		}
	})

	t.Run("local file without wrapped DEK returns error", func(t *testing.T) {
		inputFlag = "/path/to/file.bin"
		wrappedDEKFlag = ""
		b2BucketFlag = ""

		_, err := getInputSource()
		if err == nil {
			t.Error("expected error for local file without wrapped DEK")
		}
	})

	t.Run("bucket specified separately", func(t *testing.T) {
		inputFlag = "path/to/file"
		wrappedDEKFlag = ""
		b2BucketFlag = "my-bucket"

		src, err := getInputSource()
		if err != nil {
			t.Fatalf("getInputSource: %v", err)
		}

		if src.Type != "b2" {
			t.Errorf("expected type b2, got %s", src.Type)
		}
		if src.Bucket != "my-bucket" {
			t.Errorf("expected bucket my-bucket, got %s", src.Bucket)
		}
	})
}

// Test parseB2URL
func TestParseB2URL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantErr    bool
		wantBucket string
		wantKey    string
	}{
		{
			name:       "valid URL",
			url:        "b2://my-bucket/path/to/file",
			wantErr:    false,
			wantBucket: "my-bucket",
			wantKey:    "path/to/file",
		},
		{
			name:    "URL with no path",
			url:     "b2://my-bucket",
			wantErr: true,
		},
		{
			name:    "invalid URL format",
			url:     "https://example.com/file",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := parseB2URL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseB2URL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if src.Bucket != tt.wantBucket {
					t.Errorf("Bucket = %v, want %v", src.Bucket, tt.wantBucket)
				}
				if src.Path != tt.wantKey {
					t.Errorf("Path = %v, want %v", src.Path, tt.wantKey)
				}
			}
		})
	}
}

// TestMain sets up test flags
func TestMain(m *testing.M) {
	// Clear any flags set during normal init
	flag.Parse()
	os.Exit(m.Run())
}

// Test that we can write to stdout
func TestWriteOutput(t *testing.T) {
	data := []byte("test output data")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputFlag = "" // stdout
	err := writeOutput(data)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("writeOutput to stdout: %v", err)
	}

	// Read back
	var buf bytes.Buffer
	io.Copy(&buf, r)
	if !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("stdout data mismatch: got %q, want %q", buf.Bytes(), data)
	}
}

// Test write to file
func TestWriteOutputFile(t *testing.T) {
	data := []byte("test output to file")
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	outputFlag = outputFile
	err := writeOutput(data)
	if err != nil {
		t.Fatalf("writeOutput to file: %v", err)
	}

	// Read back
	readData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	if !bytes.Equal(readData, data) {
		t.Errorf("file data mismatch: got %q, want %q", readData, data)
	}
}
