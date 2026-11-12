// Package handlers_test tests the S3 operation handlers.
package handlers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server/handlers"
)

// TestReproCompressedV3TinyObjectGet reproduces armor-fe208b09: a 1MiB
// highly-compressible payload stored via the buffered v3 compressed path
// collapses to a ciphertext smaller than the trailer window the reader
// derives from the ORIGINAL plaintext size, so GET 500s with
// "invalid v3 trailer offset".
func TestReproCompressedV3TinyObjectGet(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3
	rules, err := config.ParseCompressRules(".bin=zstd")
	if err != nil {
		t.Fatalf("parse rules: %v", err)
	}
	cfg.CompressRules = rules
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	plaintext := compressibleBody(1024 * 1024) // 1MiB, compresses to ~hundreds of bytes

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/tiny.bin", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed with status %d: %s", w.Code, w.Body.String())
	}

	mb.mu.Lock()
	stored := len(mb.objects["test-bucket/tiny.bin"])
	compressed := mb.meta["test-bucket/tiny.bin"]["x-amz-meta-armor-compressed"]
	mb.mu.Unlock()
	t.Logf("stored ciphertext=%d bytes, compressed=%q", stored, compressed)
	if compressed != "true" {
		t.Fatalf("expected the object to be stored compressed, got %q", compressed)
	}

	req = httptest.NewRequest(http.MethodGet, "/test-bucket/tiny.bin", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET failed with status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d", w.Body.Len(), len(plaintext))
	}
}
