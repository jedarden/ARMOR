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

// TestPutObjectOverrideTrueStreamsLargeObjectWhenConfigOff verifies that a
// compress=true override can NO LONGER pull an over-threshold body into the
// buffered path when it cannot honor it safely. A compression decision (here:
// the client-supplied x-amz-meta-armor-compress header with config
// compression off) never influences path choice: whole-object compression
// needs the entire body in memory, so honoring the override on a large body
// was a memory-DoS route — any authenticated client could stream a multi-GiB
// PUT through io.ReadAll with one header. The request takes the streaming
// path (which spools to a temp file, bounding memory) and stores the object
// UNCOMPRESSED; the override keeps its small-object semantics (see
// TestSmallObjectOverridePrecedence). (ADR-007 per-request override, bounded.)
func TestPutObjectOverrideTrueStreamsLargeObjectWhenConfigOff(t *testing.T) {
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

	// The bounded streaming path must have been taken, not the buffered
	// whole-body path — the routing assertion is the bounded-memory check:
	// the streaming path spools to a temp file, and io.ReadAll is unreachable
	// for any body whose declared length exceeds the streaming threshold.
	if w.Header().Get("X-Armor-Streaming") != "true" {
		t.Error("compress=true override must not force a large body onto the buffered path")
	}

	mb.mu.Lock()
	meta := mb.meta["test-bucket/override-large"]
	data := mb.objects["test-bucket/override-large"]
	mb.mu.Unlock()

	if meta["x-amz-meta-armor-compressed"] != "" {
		t.Errorf("expected the streamed object to be stored uncompressed, got compressed=%q", meta["x-amz-meta-armor-compressed"])
	}
	if len(data) < 64 {
		t.Fatalf("stored envelope too short: %d bytes", len(data))
	}
	if got := data[62]; got != crypto.CompressionFlagNone {
		t.Errorf("expected Reserved[0]=0x%02x (uncompressed), got 0x%02x", crypto.CompressionFlagNone, got)
	}
	if envelopeVersion := data[4]; envelopeVersion != 0x03 {
		t.Errorf("expected envelope version 0x03, got 0x%02x", envelopeVersion)
	}

	// The streamed object must still round-trip byte-identically.
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/override-large", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET failed: status %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Errorf("streaming round-trip mismatch: got %d bytes, want %d", w.Body.Len(), len(plaintext))
	}
}

// TestPutObjectEnabledCompressionStreamsAboveThreshold verifies the same
// bound for server-side compression: with compression ENABLED (a matching
// rule, or the ARMOR_COMPRESS=true global alias), an over-threshold body
// still takes the streaming path and stores uncompressed. Before the bound a
// matching rule forced every over-threshold matching object through io.ReadAll
// — the same unbounded-buffering defect as the override header, reached via
// configuration instead of a request header.
func TestPutObjectEnabledCompressionStreamsAboveThreshold(t *testing.T) {
	plaintext := compressibleBody(10*1024*1024 + 1) // over the streaming threshold

	t.Run("matching_rule", func(t *testing.T) {
		cfg, mb, cache, footerCache, km := testSetup(t)
		cfg.FormatWriteVersion = 3
		rules, err := config.ParseCompressRules(".jsonl=zstd")
		if err != nil {
			t.Fatalf("parse rules: %v", err)
		}
		cfg.CompressRules = rules
		h := handlers.New(cfg, mb, cache, footerCache, km, nil)

		req := httptest.NewRequest(http.MethodPut, "/test-bucket/metrics.jsonl", bytes.NewReader(plaintext))
		req.Header.Set("Content-Type", "application/jsonl")
		w := httptest.NewRecorder()

		h.HandleRoot(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("X-Armor-Streaming") != "true" {
			t.Error("a matching compression rule must not force a large body onto the buffered path")
		}

		mb.mu.Lock()
		meta := mb.meta["test-bucket/metrics.jsonl"]
		mb.mu.Unlock()

		if meta["x-amz-meta-armor-compressed"] != "" {
			t.Errorf("expected the streamed object to be stored uncompressed, got compressed=%q", meta["x-amz-meta-armor-compressed"])
		}

		req = httptest.NewRequest(http.MethodGet, "/test-bucket/metrics.jsonl", nil)
		w = httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET failed: status %d", w.Code)
		}
		if !bytes.Equal(w.Body.Bytes(), plaintext) {
			t.Errorf("streaming round-trip mismatch: got %d bytes, want %d", w.Body.Len(), len(plaintext))
		}
	})

	t.Run("global_flag", func(t *testing.T) {
		cfg, mb, cache, footerCache, km := testSetup(t)
		cfg.FormatWriteVersion = 3
		cfg.Compress = true
		h := handlers.New(cfg, mb, cache, footerCache, km, nil)

		req := httptest.NewRequest(http.MethodPut, "/test-bucket/flag-large", bytes.NewReader(plaintext))
		req.Header.Set("Content-Type", "application/octet-stream")
		w := httptest.NewRecorder()

		h.HandleRoot(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("X-Armor-Streaming") != "true" {
			t.Error("the global compress flag must not force a large body onto the buffered path")
		}

		mb.mu.Lock()
		meta := mb.meta["test-bucket/flag-large"]
		mb.mu.Unlock()

		if meta["x-amz-meta-armor-compressed"] != "" {
			t.Errorf("expected the streamed object to be stored uncompressed, got compressed=%q", meta["x-amz-meta-armor-compressed"])
		}
	})
}

// TestPutObjectUnknownLengthOverrideStreams extends the bound to the
// unknown-length (chunked) shape: a compress=true override cannot force a
// body with no Content-Length onto the buffered path either — without a
// length there is no proof the body fits the buffer, so it streams.
func TestPutObjectUnknownLengthOverrideStreams(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	plaintext := compressibleBody(1024)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/chunked-override.parquet", unknownLengthBody{bytes.NewReader(plaintext)})
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("x-amz-meta-armor-compress", "true")
	if req.ContentLength != -1 {
		t.Fatalf("test premise: expected unknown ContentLength, got %d", req.ContentLength)
	}
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Armor-Streaming") != "true" {
		t.Error("an unknown-length body must take the streaming path even with compress=true")
	}

	mb.mu.Lock()
	meta := mb.meta["test-bucket/chunked-override.parquet"]
	mb.mu.Unlock()

	if meta["x-amz-meta-armor-compressed"] != "" {
		t.Errorf("expected the streamed object to be stored uncompressed, got compressed=%q", meta["x-amz-meta-armor-compressed"])
	}
}

// TestPutObjectOverrideFalseStillStreamsLargeObjectWhenConfigOff verifies that
// a compress=false override does NOT force the buffered path — no override
// value pulls large bodies into memory — and that the streamed
// object still round-trips byte-identically.
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
