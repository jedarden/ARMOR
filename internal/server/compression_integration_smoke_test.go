package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	armorcrypto "github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/presign"
)

const compressionSmokeBucket = "compression-smoke-bucket"

// compressionSmokeStorage adapts CompressionTestUtilities to the real share
// handler. The utility creates the compressed or uncompressed payload, and
// this adapter runs that payload through ARMOR encryption before storing it in
// the filesystem backend used by the test server.
type compressionSmokeStorage struct {
	t      *testing.T
	server *Server
}

func (s *compressionSmokeStorage) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	s.t.Helper()

	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read test object: %w", err)
	}
	if int64(len(data)) != size {
		return fmt.Errorf("test object size mismatch: read %d bytes, want %d", len(data), size)
	}

	compressed := meta["test-compressed"] == "true"
	encrypted, hmacTable, armorMeta := encryptTestData(s.t, s.server, data, compressed)
	if contentType := meta["content-type"]; contentType != "" {
		armorMeta.ContentType = contentType
	}
	storeTestObject(s.t, s.server.backend, ctx, bucket, key, encrypted, hmacTable, armorMeta)
	return nil
}

func (s *compressionSmokeStorage) Delete(ctx context.Context, bucket, key string) error {
	return s.server.backend.Delete(ctx, bucket, key)
}

type compressionSmokeHarness struct {
	server  *Server
	objects *armorcrypto.CompressionTestUtilities
	ctx     context.Context
	bucket  string
}

func newCompressionSmokeHarness(t *testing.T) *compressionSmokeHarness {
	t.Helper()

	tmpDir, cleanup := setupTestEnvironment(t)
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: tmpDir})
	if err != nil {
		cleanup()
		t.Fatalf("create filesystem backend: %v", err)
	}

	cfg := loadTestConfig(t, tmpDir)
	server, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		cleanup()
		t.Fatalf("create test server: %v", err)
	}
	server.presigner = presign.NewSigner(cfg.PresignSecret, "")

	storage := &compressionSmokeStorage{t: t, server: server}
	objects := armorcrypto.NewCompressionTestUtilities(storage)
	harness := &compressionSmokeHarness{
		server:  server,
		objects: objects,
		ctx:     context.Background(),
		bucket:  compressionSmokeBucket,
	}

	t.Cleanup(func() {
		if err := objects.TeardownAllTestObjects(harness.ctx, harness.bucket); err != nil {
			t.Errorf("tear down compression smoke objects: %v", err)
		}
		cleanup()
	})

	return harness
}

func (h *compressionSmokeHarness) shareGET(t *testing.T, key, rangeHeader string) *http.Response {
	t.Helper()

	token := generateTestToken(t, h.server, h.bucket, key, time.Hour)
	req := httptest.NewRequest(http.MethodGet, "/share/"+token, nil)
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	recorder := httptest.NewRecorder()
	h.server.handleShare(recorder, req)
	return recorder.Result()
}

func compressionSmokeContent() []byte {
	return armorcrypto.GeneratePatternContent(
		"ARMOR compression smoke content: verify the complete share pipeline. ",
		512,
	)
}

func verifyShareBody(t *testing.T, response *http.Response, expected []byte) []byte {
	t.Helper()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("share GET returned status %d, want %d: %s", response.StatusCode, http.StatusOK, body)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read share GET response: %v", err)
	}
	result := armorcrypto.VerifyDecompression(body, expected)
	if !result.Pass {
		t.Fatalf("share GET verification failed: %s", result.Diagnostic)
	}
	return body
}

func verifySmokeRange(t *testing.T, data []byte, compressed bool, rangeHeader string) *armorcrypto.RangeResult {
	t.Helper()

	simulator := armorcrypto.NewRangeSimulator(data, compressed, 65536)
	result, err := simulator.SimulateRangeRequest(rangeHeader)
	if err != nil {
		t.Fatalf("simulate %s range: %v", rangeHeader, err)
	}

	start, end := result.Spec.ResolveRange(int64(len(data)))
	expected := data[start : end+1]
	if err := result.Verify(expected); err != nil {
		t.Fatalf("range helper verification failed for %s: %v", rangeHeader, err)
	}
	verification := armorcrypto.VerifyRangeDecompressionWithBounds(
		data,
		result.Data,
		start,
		result.ContentLength,
	)
	if !verification.Pass {
		t.Fatalf("range decompression verification failed for %s: %s", rangeHeader, verification.Diagnostic)
	}
	return result
}

func TestCompressionSmoke_CompressedObjectGET(t *testing.T) {
	harness := newCompressionSmokeHarness(t)
	content := compressionSmokeContent()
	object, err := harness.objects.CreateCompressedTestObject(
		harness.ctx,
		harness.bucket,
		"compressed-get/object",
		content,
		"application/octet-stream",
	)
	if err != nil {
		t.Fatalf("create compressed test object: %v", err)
	}

	if err := armorcrypto.VerifyCompressedData(object.StoredContent, object.OriginalContent); err != nil {
		t.Fatalf("verify compressed setup object: %v", err)
	}
	if tracked, ok := harness.objects.GetTestObject(object.Key); !ok || tracked != object {
		t.Fatalf("compressed object was not tracked by setup helpers")
	}

	response := harness.shareGET(t, object.Key, "")
	body := verifyShareBody(t, response, object.OriginalContent)
	if !bytes.Equal(body, content) {
		t.Fatal("compressed share GET returned bytes different from original content")
	}
}

func TestCompressionSmoke_UncompressedObjectGET(t *testing.T) {
	harness := newCompressionSmokeHarness(t)
	content := compressionSmokeContent()
	object, err := harness.objects.CreateUncompressedTestObject(
		harness.ctx,
		harness.bucket,
		"uncompressed-get/object",
		content,
		"application/octet-stream",
	)
	if err != nil {
		t.Fatalf("create uncompressed test object: %v", err)
	}

	response := harness.shareGET(t, object.Key, "")
	body := verifyShareBody(t, response, object.OriginalContent)
	if !bytes.Equal(object.StoredContent, object.OriginalContent) {
		t.Fatal("uncompressed setup helper changed the stored content")
	}
	if !bytes.Equal(body, content) {
		t.Fatal("uncompressed share GET returned bytes different from original content")
	}
}

func TestCompressionSmoke_CompressedObjectRange(t *testing.T) {
	harness := newCompressionSmokeHarness(t)
	object, err := harness.objects.CreateCompressedTestObject(
		harness.ctx,
		harness.bucket,
		"compressed-range/object",
		compressionSmokeContent(),
		"application/octet-stream",
	)
	if err != nil {
		t.Fatalf("create compressed range object: %v", err)
	}

	// A full share GET proves the object can be decrypted and decompressed.
	verifyShareBody(t, harness.shareGET(t, object.Key, ""), object.OriginalContent)

	// The range helpers can still validate a plaintext slice of a compressed
	// object. The HTTP endpoint must reject the corresponding request because
	// zstd's variable-length encoding cannot safely seek by plaintext offset.
	verifySmokeRange(t, object.OriginalContent, object.Compressed, "bytes=256-1023")
	response := harness.shareGET(t, object.Key, "bytes=0-0")
	defer response.Body.Close()
	if response.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("compressed share range returned status %d, want %d: %s", response.StatusCode, http.StatusRequestedRangeNotSatisfiable, body)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read compressed range rejection: %v", err)
	}
	if !bytes.Contains(body, []byte("Range reads unsupported on compressed objects")) {
		t.Fatalf("compressed range rejection did not explain unsupported seeking: %s", body)
	}
}

func TestCompressionSmoke_UncompressedObjectRange(t *testing.T) {
	harness := newCompressionSmokeHarness(t)
	object, err := harness.objects.CreateUncompressedTestObject(
		harness.ctx,
		harness.bucket,
		"uncompressed-range/object",
		compressionSmokeContent(),
		"application/octet-stream",
	)
	if err != nil {
		t.Fatalf("create uncompressed range object: %v", err)
	}

	const rangeHeader = "bytes=256-1023"
	expectedRange := verifySmokeRange(t, object.OriginalContent, object.Compressed, rangeHeader)
	response := harness.shareGET(t, object.Key, rangeHeader)
	defer response.Body.Close()
	if response.StatusCode != http.StatusPartialContent {
		t.Fatalf("uncompressed share range returned status %d, want %d", response.StatusCode, http.StatusPartialContent)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read uncompressed share range: %v", err)
	}
	verification := armorcrypto.VerifyRangeDecompressionWithBounds(
		object.OriginalContent,
		body,
		expectedRange.Spec.Start,
		expectedRange.ContentLength,
	)
	if !verification.Pass {
		t.Fatalf("uncompressed share range verification failed: %s", verification.Diagnostic)
	}
	if err := expectedRange.Verify(body); err != nil {
		t.Fatalf("uncompressed share range differs from range helper result: %v", err)
	}

	start, end, total, err := armorcrypto.ParseContentRange(response.Header.Get("Content-Range"))
	if err != nil {
		t.Fatalf("parse share Content-Range: %v", err)
	}
	if got, want := fmt.Sprintf("bytes %d-%d/%d", start, end, total), expectedRange.ContentRange; got != want {
		t.Fatalf("share Content-Range = %q, want %q", got, want)
	}
}
