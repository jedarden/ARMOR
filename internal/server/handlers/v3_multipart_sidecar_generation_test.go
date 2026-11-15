package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
)

// The v3 multipart sidecar cache is keyed by the STORED-object generation, not
// by the composite multipart ETag: that ETag is a pure function of the
// plaintext (md5 of per-part plaintext MD5s), so re-uploading the same
// plaintext to the same key reproduces it while replacing the ciphertext and
// every block HMAC. Keying on it made the first GET after such an overwrite
// serve the previous generation's sidecar and fail HMAC verification at part 1
// block 0 (armor-a6d3dd64).
//
// These tests pin the production contract: the caller passes the generation of
// the ciphertext HEAD it just performed, generations with identical composite
// ETags get their own sidecar entries, and an empty generation never shares a
// cached entry.

const sidecarGenBlockSize = 128

// sidecarGenObject is one stored generation of the SAME plaintext.
type sidecarGenObject struct {
	dek, iv []byte
	cipher  []byte
	plain   []byte
	sidecar *backend.HMACTableSidecarV3
}

// newSidecarGenObject encrypts one generation of a fixed plaintext under a
// fresh IV. freshIV changes the ciphertext and every block HMAC while the
// plaintext (and therefore the composite multipart ETag a client sees) stays
// identical — the exact shape of a same-plaintext multipart overwrite.
func newSidecarGenObject(t *testing.T, ivSeed string, plain []byte, compositeETag string) sidecarGenObject {
	t.Helper()

	b64 := func(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
	clen := func(n int, compressed bool) string {
		v := uint32(n)
		if compressed {
			v |= 0x80000000
		}
		raw := make([]byte, 4)
		binary.BigEndian.PutUint32(raw, v)
		return base64.StdEncoding.EncodeToString(raw)
	}

	dek := sha256Seed("armor-sidecar-gen-dek")
	iv := sha256Seed("armor-sidecar-gen-iv-" + ivSeed)[:16]

	sidecar := &backend.HMACTableSidecarV3{Version: 3, BlockSize: sidecarGenBlockSize}
	sidecarPart := backend.HMACPartV3{N: 1}

	var cipher []byte
	for blockIdx := 0; blockIdx < 3; blockIdx++ {
		blockPlain := plain[blockIdx*sidecarGenBlockSize:]
		if len(blockPlain) > sidecarGenBlockSize {
			blockPlain = blockPlain[:sidecarGenBlockSize]
		}
		ciphertext, hmacValue, err := crypto.EncryptBlockV3(dek, iv, 1, uint32(blockIdx), blockPlain, sidecarGenBlockSize)
		if err != nil {
			t.Fatalf("EncryptBlockV3(block %d): %v", blockIdx, err)
		}
		sidecarPart.Blocks = append(sidecarPart.Blocks, []string{
			b64(hmacValue), clen(len(ciphertext), false),
		})
		cipher = append(cipher, ciphertext...)
	}
	sidecarPart.PlaintextLen = int64(len(plain))
	sidecarPart.CiphertextLen = int64(len(cipher))
	sidecar.Parts = append(sidecar.Parts, sidecarPart)

	return sidecarGenObject{
		dek: dek, iv: iv, cipher: cipher, plain: plain,
		sidecar: sidecar,
	}
}

func sha256Seed(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

// sidecarGenBackend serves one mutable ciphertext object plus map-stored
// objects (the sidecar) — the minimum handleV3MultipartGet and
// LoadHMACTableV3 read.
type sidecarGenBackend struct {
	backend.NilBackend
	mu         sync.Mutex
	objects    map[string][]byte
	ciphertext []byte
}

func (b *sidecarGenBackend) GetRange(_ context.Context, _ string, _ string, offset, length int64) (io.ReadCloser, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if offset < 0 || offset+length > int64(len(b.ciphertext)) {
		return nil, fmt.Errorf("range %d+%d beyond %d bytes", offset, length, len(b.ciphertext))
	}
	return io.NopCloser(bytes.NewReader(b.ciphertext[offset : offset+length])), nil
}

func (b *sidecarGenBackend) GetDirect(_ context.Context, _ string, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, ok := b.objects[key]
	if !ok {
		return nil, nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(data)), &backend.ObjectInfo{Key: key, Size: int64(len(data))}, nil
}

func (b *sidecarGenBackend) setGeneration(obj sidecarGenObject, sidecarBytes []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ciphertext = obj.cipher
	for k := range b.objects {
		delete(b.objects, k)
	}
	b.objects[backend.GetSidecarKey("", "key")] = sidecarBytes
}

func sidecarGzipBytes(t *testing.T, sidecar *backend.HMACTableSidecarV3) []byte {
	t.Helper()
	data, err := json.Marshal(sidecar)
	if err != nil {
		t.Fatalf("marshal sidecar: %v", err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		t.Fatalf("gzip sidecar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func sidecarGenMeta(t *testing.T, obj sidecarGenObject, compositeETag string) (*backend.ARMORMetadata, *crypto.Decryptor) {
	t.Helper()
	meta := &backend.ARMORMetadata{
		Version:       crypto.Version3,
		BlockSize:     sidecarGenBlockSize,
		PlaintextSize: int64(len(obj.plain)),
		IV:            obj.iv,
		ETag:          compositeETag, // the composite multipart ETag: identical across generations
		ContentType:   "application/octet-stream",
	}
	meta.CiphertextSize = int64(len(obj.cipher))
	dec, err := crypto.NewDecryptor(obj.dek, obj.iv, sidecarGenBlockSize)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	return meta, dec
}

// TestV3MultipartSidecarCacheServesFreshSidecarAfterSamePlaintextOverwrite
// reproduces armor-a6d3dd64: two multipart generations of byte-identical
// plaintext share the composite ETag, and the second GET must load the second
// generation's sidecar instead of replaying the first generation's block
// HMACs against new ciphertext.
func TestV3MultipartSidecarCacheServesFreshSidecarAfterSamePlaintextOverwrite(t *testing.T) {
	compositeETag := "7e0b0f60e6f383862c4dca7b1e0b1a11-1" // what ComputeCompositeETag would yield
	plain := make([]byte, 3*sidecarGenBlockSize-32)       // two full blocks and one tail block
	for i := range plain {
		plain[i] = byte(i * 7)
	}

	gen1 := newSidecarGenObject(t, "gen-1", plain, compositeETag)
	gen2 := newSidecarGenObject(t, "gen-2", plain, compositeETag)
	if bytes.Equal(gen1.cipher, gen2.cipher) {
		t.Fatal("fresh IVs must produce different ciphertext")
	}

	be := &sidecarGenBackend{
		objects:    map[string][]byte{backend.GetSidecarKey("", "key"): sidecarGzipBytes(t, gen1.sidecar)},
		ciphertext: gen1.cipher,
	}
	h := &Handlers{
		backend:               be,
		multipartSidecarCache: backend.NewMultipartSidecarCache(10, 60),
		config:                &config.Config{},
	}

	gen1Parts := v3TableKeyParts{ETag: "stored-etag-gen-1", StoredSize: int64(len(gen1.cipher)), LastModified: time.Unix(1000, 0)}
	gen2Parts := v3TableKeyParts{ETag: "stored-etag-gen-2", StoredSize: int64(len(gen2.cipher)), LastModified: time.Unix(2000, 0)}

	read := func(gen v3TableKeyParts, obj sidecarGenObject) []byte {
		t.Helper()
		meta, dec := sidecarGenMeta(t, obj, compositeETag)
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/bucket/key", nil)
		h.handleV3MultipartGet(w, r, "bucket", "key", "prefixed/key", dec, meta, int64(len(obj.plain)), time.Now(), gen)
		if w.Code != 200 {
			t.Fatalf("GET status %d: %s", w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}

	// Generation 1: populates the cache.
	if got := read(gen1Parts, gen1); !bytes.Equal(got, plain) {
		t.Fatalf("generation 1 GET returned %d bytes, want %d", len(got), len(plain))
	}

	// Same-plaintext overwrite: new ciphertext, new sidecar, SAME composite
	// ETag. Before the fix the cached generation-1 sidecar was served and the
	// read died on "HMAC verification failed for part 1 block 0".
	be.setGeneration(gen2, sidecarGzipBytes(t, gen2.sidecar))
	if got := read(gen2Parts, gen2); !bytes.Equal(got, plain) {
		t.Fatalf("generation 2 GET after same-plaintext overwrite returned %d bytes, want %d (stale sidecar would fail HMAC)", len(got), len(plain))
	}

	// The generations' entries coexist: re-reading generation 1 still works
	// against its own sidecar.
	be.setGeneration(gen1, sidecarGzipBytes(t, gen1.sidecar))
	if got := read(gen1Parts, gen1); !bytes.Equal(got, plain) {
		t.Fatalf("generation 1 re-read returned %d bytes, want %d", len(got), len(plain))
	}
}

// TestV3MultipartSidecarCacheEmptyGenerationBypassesCache pins the fail-safe:
// a request with no stored-generation identity must not share (or populate) a
// cached entry — every such read loads fresh.
func TestV3MultipartSidecarCacheEmptyGenerationBypassesCache(t *testing.T) {
	plain := make([]byte, 2*sidecarGenBlockSize)
	for i := range plain {
		plain[i] = byte(i + 1)
	}
	obj := newSidecarGenObject(t, "bypass", plain, "composite-1")

	be := &sidecarGenBackend{
		objects:    map[string][]byte{backend.GetSidecarKey("", "key"): sidecarGzipBytes(t, obj.sidecar)},
		ciphertext: obj.cipher,
	}
	cache := backend.NewMultipartSidecarCache(10, 60)
	h := &Handlers{backend: be, multipartSidecarCache: cache, config: &config.Config{}}

	meta, dec := sidecarGenMeta(t, obj, "composite-1")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/bucket/key", nil)
	h.handleV3MultipartGet(w, r, "bucket", "key", "prefixed/key", dec, meta, int64(len(plain)), time.Now(), v3TableKeyParts{})
	if w.Code != 200 {
		t.Fatalf("GET status %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.Bytes(); !bytes.Equal(got, plain) {
		t.Fatalf("empty-generation GET returned %d bytes, want %d", len(got), len(plain))
	}
	if cache.Size() != 0 {
		t.Fatalf("empty-generation read cached %d entries, want 0", cache.Size())
	}
}
