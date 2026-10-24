// Package handlers_test tests the S3 operation handlers.
package handlers_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/server/handlers"
)

// putCompressible large-object helper: builds an over-threshold (10MB) PUT body
// that zstd can actually shrink (opportunistic pass-through would otherwise
// skip compression and mask the behavior under test).
func compressibleBody(size int) []byte {
	pattern := bytes.Repeat([]byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"), 5)
	body := bytes.Repeat(pattern, size/len(pattern)+1)
	return body[:size]
}

// unknownLengthBody hides the wrapped reader's Len so httptest.NewRequest
// leaves r.ContentLength at -1, the chunked/unknown-length upload shape.
type unknownLengthBody struct{ io.Reader }

// TestPutObjectOverrideCompressesLargeObjectWhenConfigOff verifies that a
// compress=true override is honored even when config compression is off and
// the body is over the streaming threshold. Before the fix the request took
// the streaming path, which cannot compress, and the header was silently
// ignored. (ADR-007 per-request override.)
//
// The read side is deliberately not exercised here: whole-object compressed
// GET round-trips are currently broken independently of the routing decision
// (the reader derives block layout from the original plaintext size, so a
// shrunken envelope fails verification). That defect is tracked separately;
// this test pins the write-path routing only.
func TestPutObjectOverrideCompressesLargeObjectWhenConfigOff(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3
	// Config compression stays OFF: no cfg.Compress, no CompressRules.
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	plaintext := compressibleBody(10*1024*1024 + 1) // over the streaming threshold

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/override-large", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("x-amz-meta-armor-compress", "true")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// The buffered (compressible) path must have been taken, not streaming.
	if w.Header().Get("X-Armor-Streaming") == "true" {
		t.Error("compress=true override must bypass the streaming path")
	}

	mb.mu.Lock()
	meta := mb.meta["test-bucket/override-large"]
	data := mb.objects["test-bucket/override-large"]
	mb.mu.Unlock()

	if meta["x-amz-meta-armor-compressed"] != "true" {
		t.Errorf("expected compressed=true, got %q", meta["x-amz-meta-armor-compressed"])
	}
	if meta["x-amz-meta-armor-compression-type"] != "zstd" {
		t.Errorf("expected zstd compression type, got %q", meta["x-amz-meta-armor-compression-type"])
	}
	if len(data) < 64 {
		t.Fatalf("stored envelope too short: %d bytes", len(data))
	}
	if got := data[62]; got != crypto.CompressionFlagZstd {
		t.Errorf("expected Reserved[0]=0x%02x, got 0x%02x", crypto.CompressionFlagZstd, got)
	}
	if envelopeVersion := data[4]; envelopeVersion != 0x03 {
		t.Errorf("expected envelope version 0x03, got 0x%02x", envelopeVersion)
	}
}

// TestPutObjectOverrideFalseStillStreamsLargeObjectWhenConfigOff verifies that
// a compress=false override does NOT force the buffered path — only
// compress=true pulls large bodies into memory — and that the rerouted
// streaming object still round-trips byte-identically.
func TestPutObjectOverrideFalseStillStreamsLargeObjectWhenConfigOff(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	plaintext := compressibleBody(10*1024*1024 + 1)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/override-false-large", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("x-amz-meta-armor-compress", "false")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Armor-Streaming") != "true" {
		t.Error("compress=false override must not force the buffered path")
	}

	mb.mu.Lock()
	meta := mb.meta["test-bucket/override-false-large"]
	mb.mu.Unlock()

	if meta["x-amz-meta-armor-compressed"] != "" {
		t.Errorf("expected no compression with override=false, got %q", meta["x-amz-meta-armor-compressed"])
	}

	req = httptest.NewRequest(http.MethodGet, "/test-bucket/override-false-large", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET failed: status %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Errorf("streaming round-trip mismatch: got %d bytes, want %d", w.Body.Len(), len(plaintext))
	}
}

// TestPutObjectInvalidOverrideRejectedOnLargeObject verifies that an invalid
// override value 400s even when the body would otherwise take the streaming
// path (which cannot validate the header). ADR-007: invalid values return 400,
// identically in the small and large paths.
func TestPutObjectInvalidOverrideRejectedOnLargeObject(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	plaintext := compressibleBody(10*1024*1024 + 1)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/override-bogus", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("x-amz-meta-armor-compress", "bogus")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("InvalidArgument")) {
		t.Errorf("expected InvalidArgument error code, got: %s", w.Body.String())
	}

	mb.mu.Lock()
	_, stored := mb.objects["test-bucket/override-bogus"]
	mb.mu.Unlock()
	if stored {
		t.Error("rejected PUT must not store an object")
	}
}

// TestLargeUncompressedObjectStreamsDespiteUnrelatedRules verifies that a
// configured compression rule that does not match the object leaves large
// uploads on the bounded streaming path: previously the mere presence of any
// rule forced every over-threshold body through io.ReadAll. A .parquet key
// (already-compressed workload shape) must stream while .jsonl keys still
// compress. (ADR-007 rules; streaming threshold 10MB.)
func TestLargeUncompressedObjectStreamsDespiteUnrelatedRules(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	cfg.FormatWriteVersion = 3
	rules, err := config.ParseCompressRules(".jsonl=zstd")
	if err != nil {
		t.Fatalf("parse rules: %v", err)
	}
	cfg.CompressRules = rules
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	plaintext := compressibleBody(10*1024*1024 + 1)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/metrics-2026.parquet", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Armor-Streaming") != "true" {
		t.Error("unrelated compression rules must not force a large uncompressed object into the buffered path")
	}

	mb.mu.Lock()
	meta := mb.meta["test-bucket/metrics-2026.parquet"]
	mb.mu.Unlock()

	if meta["x-amz-meta-armor-compressed"] != "" {
		t.Errorf("expected no compression for unmatched rule, got %q", meta["x-amz-meta-armor-compressed"])
	}

	req = httptest.NewRequest(http.MethodGet, "/test-bucket/metrics-2026.parquet", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET failed: status %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Errorf("round-trip mismatch: got %d bytes, want %d", w.Body.Len(), len(plaintext))
	}
}

// TestUnknownLengthUncompressedStreamsDespiteRules verifies the unknown
// Content-Length shape: without a length the old decision buffered whenever
// any rule was configured. The decision-first routing must stream it.
func TestUnknownLengthUncompressedStreamsDespiteRules(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	rules, err := config.ParseCompressRules(".jsonl=zstd")
	if err != nil {
		t.Fatalf("parse rules: %v", err)
	}
	cfg.CompressRules = rules
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	plaintext := compressibleBody(1024)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/chunked.parquet", unknownLengthBody{bytes.NewReader(plaintext)})
	req.Header.Set("Content-Type", "application/octet-stream")
	if req.ContentLength != -1 {
		t.Fatalf("test premise: expected unknown ContentLength, got %d", req.ContentLength)
	}
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Armor-Streaming") != "true" {
		t.Error("unknown-length uncompressed objects must take the streaming path")
	}
}

// TestSmallObjectOverridePrecedence pins the small (always-buffered) path to
// the same precedence the large path now applies: the per-request override
// beats configured rules, and an invalid override is rejected with 400.
func TestSmallObjectOverridePrecedence(t *testing.T) {
	rules, err := config.ParseCompressRules(".jsonl=zstd")
	if err != nil {
		t.Fatalf("parse rules: %v", err)
	}

	t.Run("override_false_beats_matching_rule", func(t *testing.T) {
		cfg, mb, cache, footerCache, km := testSetup(t)
		cfg.CompressRules = rules
		h := handlers.New(cfg, mb, cache, footerCache, km, nil)

		body := compressibleBody(4096)
		req := httptest.NewRequest(http.MethodPut, "/test-bucket/small.jsonl", bytes.NewReader(body))
		req.Header.Set("x-amz-meta-armor-compress", "false")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		mb.mu.Lock()
		meta := mb.meta["test-bucket/small.jsonl"]
		mb.mu.Unlock()
		if meta["x-amz-meta-armor-compressed"] != "" {
			t.Errorf("override=false must beat a matching rule, got compressed=%q", meta["x-amz-meta-armor-compressed"])
		}
	})

	t.Run("override_true_compresses_when_config_off", func(t *testing.T) {
		cfg, mb, cache, footerCache, km := testSetup(t)
		h := handlers.New(cfg, mb, cache, footerCache, km, nil)

		body := compressibleBody(4096)
		req := httptest.NewRequest(http.MethodPut, "/test-bucket/small.txt", bytes.NewReader(body))
		req.Header.Set("x-amz-meta-armor-compress", "true")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		mb.mu.Lock()
		meta := mb.meta["test-bucket/small.txt"]
		mb.mu.Unlock()
		if meta["x-amz-meta-armor-compressed"] != "true" {
			t.Errorf("override=true must compress a small object with config off, got %q", meta["x-amz-meta-armor-compressed"])
		}
	})

	t.Run("invalid_override_rejected", func(t *testing.T) {
		cfg, mb, cache, footerCache, km := testSetup(t)
		h := handlers.New(cfg, mb, cache, footerCache, km, nil)

		req := httptest.NewRequest(http.MethodPut, "/test-bucket/small-bogus.txt", bytes.NewReader(compressibleBody(512)))
		req.Header.Set("x-amz-meta-armor-compress", "bogus")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}
