// Package handlers_test tests the S3 operation handlers.
package handlers_test

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/manifest"
	"github.com/jedarden/armor/internal/server/handlers"
)

// TestVersion3EncryptionPutObject verifies that PutObject uses Version3 encryption when FormatWriteVersion is 3.
func TestVersion3EncryptionPutObject(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)

	// Set FormatWriteVersion to 3
	cfg.FormatWriteVersion = 3

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Create plaintext content
	plaintext := []byte("Hello, ARMOR! This is a test file for Version3 encryption.")

	// Create PUT request
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test-key", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the stored metadata has Version 3
	mb.mu.Lock()
	meta, ok := mb.meta["test-bucket/test-key"]
	mb.mu.Unlock()

	if !ok {
		t.Fatal("object metadata not found")
	}

	version := meta["x-amz-meta-armor-version"]
	if version != "3" {
		t.Errorf("expected version 3, got %s", version)
	}

	// Verify the envelope header also has Version 3
	data := mb.objects["test-bucket/test-key"]
	if len(data) < 64 {
		t.Fatalf("encrypted data too short: %d bytes", len(data))
	}

	// Check envelope version (byte 4 in the header)
	envelopeVersion := data[4]
	if envelopeVersion != 0x03 {
		t.Errorf("expected envelope version 0x03, got 0x%02x", envelopeVersion)
	}

	// Verify v3 trailer block table format
	// v3 format: header(64) || blocks || trailer block table
	// For small plaintext that fits in one block, we should have:
	// - 64 byte header
	// - 1 block of encrypted data
	// - 36 byte trailer block table (HMAC + clen)
	blockCount := crypto.ComputeBlockCount(int64(len(plaintext)), 65536) // Default block size
	expectedTrailerSize := int64(blockCount) * crypto.BlockTableEntrySize
	minExpectedSize := 64 + int64(len(plaintext)) + expectedTrailerSize

	if int64(len(data)) < minExpectedSize {
		t.Errorf("encrypted data too short for v3 format: got %d bytes, expected at least %d bytes (header %d + plaintext %d + trailer %d)",
			len(data), minExpectedSize, 64, len(plaintext), expectedTrailerSize)
	}

	// Verify trailer block table is present at the end
	trailerOffset := 64 + int64(len(plaintext))
	if int64(len(data)) < trailerOffset + crypto.BlockTableEntrySize {
		t.Errorf("cannot read trailer block table: data too short")
	}
}

// TestVersion3EncryptionStreamingPut verifies that streaming PUT uses Version3 encryption when FormatWriteVersion is 3.
func TestVersion3EncryptionStreamingPut(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)

	// Set FormatWriteVersion to 3
	cfg.FormatWriteVersion = 3

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Create plaintext content larger than one block to trigger streaming
	plaintext := make([]byte, 128*1024) // 128KB (2 blocks)
	if _, err := rand.Read(plaintext); err != nil {
		t.Fatalf("failed to generate random plaintext: %v", err)
	}

	// Create PUT request
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/streaming-key", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the stored metadata has Version 3
	mb.mu.Lock()
	meta, ok := mb.meta["test-bucket/streaming-key"]
	mb.mu.Unlock()

	if !ok {
		t.Fatal("object metadata not found")
	}

	version := meta["x-amz-meta-armor-version"]
	if version != "3" {
		t.Errorf("expected version 3, got %s", version)
	}

	// Verify the envelope header also has Version 3
	data := mb.objects["test-bucket/streaming-key"]
	if len(data) < 64 {
		t.Fatalf("encrypted data too short: %d bytes", len(data))
	}

	envelopeVersion := data[4]
	if envelopeVersion != 0x03 {
		t.Errorf("expected envelope version 0x03, got 0x%02x", envelopeVersion)
	}

	// Verify v3 trailer block table format for 2 blocks
	blockCount := uint32(2) // 128KB with 64KB blocks = 2 blocks
	expectedTrailerSize := int64(blockCount) * crypto.BlockTableEntrySize
	minExpectedSize := 64 + int64(len(plaintext)) + expectedTrailerSize

	if int64(len(data)) < minExpectedSize {
		t.Errorf("encrypted data too short for v3 format: got %d bytes, expected at least %d bytes",
			len(data), minExpectedSize)
	}
}

// TestVersion2UnchangedWhenFormatVersionIs2 verifies that v2 path is unchanged when FormatWriteVersion is 2.
func TestVersion2UnchangedWhenFormatVersionIs2(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)

	// Explicitly set FormatWriteVersion to 2 (default)
	cfg.FormatWriteVersion = 2

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Create plaintext content
	plaintext := []byte("Hello, ARMOR! Testing that v2 is unchanged.")

	// Create PUT request
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test-key", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the stored metadata has Version 2
	mb.mu.Lock()
	meta, ok := mb.meta["test-bucket/test-key"]
	mb.mu.Unlock()

	if !ok {
		t.Fatal("object metadata not found")
	}

	version := meta["x-amz-meta-armor-version"]
	if version != "2" {
		t.Errorf("expected version 2, got %s", version)
	}

	// Verify the envelope header has Version 2
	data := mb.objects["test-bucket/test-key"]
	if len(data) < 64 {
		t.Fatalf("encrypted data too short: %d bytes", len(data))
	}

	envelopeVersion := data[4]
	if envelopeVersion != 0x02 {
		t.Errorf("expected envelope version 0x02 (v2 unchanged), got 0x%02x", envelopeVersion)
	}

	// Verify v2 inline HMAC table format (not trailer block table)
	// v2 format: header(64) || blocks || inline HMAC table
	blockCount := crypto.ComputeBlockCount(int64(len(plaintext)), 65536)
	expectedHMACTableSize := int64(blockCount) * crypto.HMACSize
	expectedSize := 64 + int64(len(plaintext)) + expectedHMACTableSize

	if int64(len(data)) != expectedSize {
		t.Errorf("v2 format size changed: got %d bytes, expected %d bytes (v2 should be unchanged)",
			len(data), expectedSize)
	}
}

// TestVersion3CompressionFlagInHeader verifies that v3 header correctly sets compression flag.
func TestVersion3CompressionFlagInHeader(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)

	// Set FormatWriteVersion to 3
	cfg.FormatWriteVersion = 3

	// Enable compression (per ADR-007, though compression is off pending compress-rules bead)
	cfg.Compress = true

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Create plaintext content
	plaintext := []byte("Hello, ARMOR! Testing compression flag in v3 header.")

	// Create PUT request
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test-key", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the stored metadata indicates compression
	mb.mu.Lock()
	meta, ok := mb.meta["test-bucket/test-key"]
	mb.mu.Unlock()

	if !ok {
		t.Fatal("object metadata not found")
	}

	compressed := meta["x-amz-meta-armor-compressed"]
	if compressed != "true" {
		t.Errorf("expected compressed=true, got %s", compressed)
	}

	// Verify the envelope header Reserved[0] byte has compression flag set
	data := mb.objects["test-bucket/test-key"]
	if len(data) < 64 {
		t.Fatalf("encrypted data too short: %d bytes", len(data))
	}

	// Reserved field is at offset 62 (2 bytes before end of 64-byte header)
	reservedByte := data[62]
	if reservedByte == 0 {
		t.Errorf("expected compression flag set in Reserved[0], got 0x%02x", reservedByte)
	}

	// Verify compression type in metadata
	compressionType := meta["x-amz-meta-armor-compression-type"]
	if compressionType != "zstd" && compressionType != "" {
		t.Errorf("expected zstd compression type, got %s", compressionType)
	}
}

// TestVersion3WithManifestCiphertextSize verifies that manifest entries include CiphertextSize for v3 objects.
func TestVersion3WithManifestCiphertextSize(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)

	// Set FormatWriteVersion to 3
	cfg.FormatWriteVersion = 3

	// Enable manifest
	cfg.ManifestEnabled = true

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Create a simple manifest recorder
	manifestRecorder := &testManifestRecorder{
		entries: make(map[string]*testManifestEntry),
	}
	h.WithManifest(manifestRecorder)

	// Create plaintext content
	plaintext := []byte("Hello, ARMOR! Testing manifest CiphertextSize.")

	// Create PUT request
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test-key", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify manifest entry was created with CiphertextSize
	entry, ok := manifestRecorder.entries["test-bucket/test-key"]
	if !ok {
		t.Fatal("manifest entry not found")
	}

	// For v3 objects, CiphertextSize should be set (non-zero)
	if entry.CiphertextSize == 0 {
		t.Errorf("expected non-zero CiphertextSize for v3 object, got %d", entry.CiphertextSize)
	}

	// Verify CiphertextSize matches the actual stored object size
	mb.mu.Lock()
	dataLen := int64(len(mb.objects["test-bucket/test-key"]))
	mb.mu.Unlock()

	if entry.CiphertextSize != dataLen {
		t.Errorf("CiphertextSize mismatch: manifest has %d, actual object is %d",
			entry.CiphertextSize, dataLen)
	}
}

// testManifestRecorder is a simple manifest recorder for testing.
type testManifestRecorder struct {
	entries map[string]*testManifestEntry
	mu      sync.Mutex
}

type testManifestEntry struct {
	Bucket          string
	Key             string
	PlaintextSize   int64
	CiphertextSize  int64
	ContentType     string
	ETag            string
}

func (m *testManifestRecorder) RecordPut(bucket, key string, size int64, sha256Hex string, iv, wrappedDEK []byte, mekFingerprint string, blockSize int, contentType, etag string, chainEntry *manifest.ChainEntry, ciphertextSize int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries[bucket+"/"+key] = &testManifestEntry{
		Bucket:         bucket,
		Key:            key,
		PlaintextSize:  size,
		CiphertextSize: ciphertextSize,
		ContentType:    contentType,
		ETag:           etag,
	}
}

func (m *testManifestRecorder) RecordDelete(bucket, key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, bucket+"/"+key)
}

func (m *testManifestRecorder) Lookup(bucket, key string) (*handlers.ManifestEntry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[bucket+"/"+key]
	if !ok {
		return nil, false
	}

	return &handlers.ManifestEntry{
		PlaintextSize:  entry.PlaintextSize,
		CiphertextSize: entry.CiphertextSize,
		ContentType:    entry.ContentType,
		ETag:           entry.ETag,
	}, true
}
