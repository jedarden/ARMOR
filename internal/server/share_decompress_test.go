package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/presign"
	"github.com/klauspost/compress/zstd"
)

// TestShareGET_CompressedObject tests that compressed objects are properly decompressed
func TestShareGET_CompressedObject(t *testing.T) {
	// Setup test environment with filesystem backend
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a filesystem backend directly
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem backend: %v", err)
	}

	// Decode presign secret from hex
	presignSecret, err := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	if err != nil {
		t.Fatalf("Failed to decode presign secret: %v", err)
	}

	// Decode MEK from hex
	mek, err := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	if err != nil {
		t.Fatalf("Failed to decode MEK: %v", err)
	}

	// Create minimal test config
	cfg := &config.Config{
		BlockSize:          65536,
		B2Region:           "us-east-005",
		B2Endpoint:         "https://s3.us-east-005.backblazeb2.com",
		B2AccessKeyID:      "testkey",
		B2SecretAccessKey:  "testsecret",
		PresignSecret:      presignSecret,
		MEK:                mek,
	}

	// Create test server with filesystem backend only
	srv, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize presigner for token generation (NewWithBackend skips this)
	srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

	// Create test data
	originalData := []byte("Hello, ARMOR! This is compressed test data.")

	// Compress the data using zstd
	compressedData := compressData(originalData)

	// Encrypt the compressed data
	encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, compressedData, true)

	// Store the encrypted object in the backend
	ctx := context.Background()
	storeTestObject(t, srv.backend, ctx, "test-bucket", "test-key-compressed", encryptedData, hmacTable, armorMeta)

	// Generate a share token
	token := generateTestToken(t, srv, "test-bucket", "test-key-compressed", time.Hour)

	// Make GET request to share endpoint
	req := httptest.NewRequest("GET", "/share/"+token, nil)
	w := httptest.NewRecorder()
	srv.handleShare(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Read response body
	retrievedData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Verify decompression - retrieved data should match original (uncompressed)
	if !bytes.Equal(retrievedData, originalData) {
		t.Errorf("Decompressed data mismatch.\nGot: %q (%d bytes)\nWant: %q (%d bytes)",
			string(retrievedData), len(retrievedData), string(originalData), len(originalData))
	}

	// Verify that the compressed data is different from original (to ensure compression actually happened)
	if bytes.Equal(compressedData, originalData) {
		t.Error("Compressed data should differ from original data")
	}
}

// TestShareGET_UncompressedObject tests that uncompressed objects are served as-is
func TestShareGET_UncompressedObject(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create filesystem backend
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem backend: %v", err)
	}

	cfg := loadTestConfig(t, tmpDir)
	srv, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize presigner for token generation
	srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

	// Create test data (not compressed)
	originalData := []byte("Hello, ARMOR! This is uncompressed test data.")

	// Encrypt the uncompressed data
	encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, originalData, false)

	// Store the encrypted object
	ctx := context.Background()
	storeTestObject(t, srv.backend, ctx, "test-bucket", "test-key-uncompressed", encryptedData, hmacTable, armorMeta)

	// Generate share token
	token := generateTestToken(t, srv, "test-bucket", "test-key-uncompressed", time.Hour)

	// Make GET request
	req := httptest.NewRequest("GET", "/share/"+token, nil)
	w := httptest.NewRecorder()
	srv.handleShare(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Read response body
	retrievedData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Verify no decompression happened - retrieved data should match original
	if !bytes.Equal(retrievedData, originalData) {
		t.Errorf("Uncompressed data mismatch.\nGot: %q (%d bytes)\nWant: %q (%d bytes)",
			string(retrievedData), len(retrievedData), string(originalData), len(originalData))
	}
}

// TestShareGET_CompressionBehavior is a table-driven test that verifies decompression behavior
// for all object types: compressed, uncompressed, and legacy.
func TestShareGET_CompressionBehavior(t *testing.T) {
	testCases := []struct {
		name              string
		data              []byte
		compressed        bool
		wantDecompression bool // whether decompression should occur
		description       string
	}{
		{
			name:              "uncompressed_small_text",
			data:              []byte("Hello, ARMOR! This is uncompressed test data."),
			compressed:        false,
			wantDecompression: false,
			description:       "uncompressed small text object - should bypass decompression",
		},
		{
			name:              "uncompressed_binary_data",
			data:              generateRandomData(50 * 1024), // 50KB
			compressed:        false,
			wantDecompression: false,
			description:       "uncompressed binary object - should bypass decompression",
		},
		{
			name:              "compressed_repeating_text",
			data:              []byte(strings.Repeat("Repeat pattern for compression. ", 100)),
			compressed:        true,
			wantDecompression: true,
			description:       "compressed repeating text - should decompress",
		},
		{
			name:              "compressed_binary_data",
			data:              generateRandomData(100 * 1024), // 100KB
			compressed:        true,
			wantDecompression: true,
			description:       "compressed binary object - should decompress",
		},
		{
			name:              "compressed_highly_repetitive_large",
			data:              bytes.Repeat([]byte("ARMOR compresses well. "), 10*1024), // ~250KB highly repetitive, compresses to a tiny fraction
			compressed:        true,
			wantDecompression: true,
			description:       "compressed large highly-repetitive object (excellent compression ratio) - should decompress",
		},
		{
			name: "compressed_structured_json",
			data: bytes.Repeat([]byte(`{"id":12345,"name":"armor-test","payload":"compressed structured data compresses well"}`), 256), // ~21KB of repeated JSON records
			compressed:        true,
			wantDecompression: true,
			description:       "compressed structured JSON object (repeated records compress efficiently) - should decompress",
		},
		{
			name: "compressed_repeated_log_lines",
			data: bytes.Repeat([]byte("2026-08-09T12:00:00Z INFO armor request handled status=200 bytes=1024 duration=5ms\n"), 3*1024), // ~250KB of repeated timestamped log entries, compresses extremely well
			compressed:        true,
			wantDecompression: true,
			description:       "compressed repeated log entries (timestamped log data compresses efficiently) - should decompress",
		},
		{
			// Multilingual UTF-8 content — emoji (4-byte sequences), CJK, accented
			// Latin, Cyrillic and Greek. No existing case exercises non-ASCII byte
			// patterns, so this case verifies byte-exact decompression of content
			// whose original bytes span the full range of UTF-8 multibyte encodings.
			name:              "compressed_unicode_multilingual",
			data:              bytes.Repeat([]byte("Hello 世界! 🌍 Café — 日本語 тест ελληνικά \n"), 2*1024), // ~150KB multilingual UTF-8, compresses well
			compressed:        true,
			wantDecompression: true,
			description:       "compressed multilingual UTF-8 text (emoji/CJK/Cyrillic/Greek diverse byte patterns) - should decompress",
		},
		{
			name:              "legacy_object_no_compression_flag",
			data:              []byte("Legacy object without compression metadata flag."),
			compressed:        false, // legacy objects don't have compression flag set
			wantDecompression: false,
			description:       "legacy object - should bypass decompression",
		},
		{
			// A genuine legacy object (written before compression existed) whose
			// stored metadata carries NO x-amz-meta-armor-compressed key, so the
			// Compressed flag stays at its zero value (false) and decompression is
			// bypassed. The plaintext here begins with the zstd magic number
			// (0x28 0xB5 0x2F 0xFD) — the real backward-compat hazard: a
			// decompression path that sniffed content rather than trusting the
			// metadata flag would try to decompress (and corrupt) this object.
			// Serving the bytes through unchanged proves the legacy path honors the
			// absent flag and triggers no decompression and no error.
			name:              "legacy_object_zstd_magic_prefix_no_flag",
			data:              append([]byte{0x28, 0xb5, 0x2f, 0xfd}, []byte(" legacy plaintext that coincidentally begins with zstd magic bytes")...),
			compressed:        false,
			wantDecompression: false,
			description:       "legacy object with zstd magic-byte prefix and no compression flag - must serve bytes unchanged without decompression",
		},
		{
			// A legacy object larger than a single encryption block
			// (BlockSize=65536), so its ciphertext spans multiple blocks that
			// the GET path must decrypt and reassemble across block boundaries.
			// Like every legacy object it carries NO x-amz-meta-armor-compressed
			// key, so the Compressed flag stays false and decompression is
			// bypassed. This is distinct from the other legacy cases (all well
			// under one block): it proves the multi-block reassembly path honors
			// the absent flag end-to-end and serves the object byte-exact with
			// no error, not just single-block objects.
			name:              "legacy_object_multiblock_no_flag",
			data:              bytes.Repeat([]byte("legacy multi-block payload segment. "), 4*1024), // ~136KB, spans multiple 64KB blocks
			compressed:        false,
			wantDecompression: false,
			description:       "legacy multi-block object (spans >1 encryption block) with no compression flag - must reassemble and serve unchanged without decompression",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, cleanup := setupTestEnvironment(t)
			defer cleanup()

			// Create filesystem backend
			fsBackend, err := backend.NewFSBackend(backend.FSConfig{
				BasePath: tmpDir,
			})
			if err != nil {
				t.Fatalf("Failed to create filesystem backend: %v", err)
			}

			cfg := loadTestConfig(t, tmpDir)
			srv, err := NewWithBackend(cfg, fsBackend)
			if err != nil {
				t.Fatalf("Failed to create server: %v", err)
			}

			// Initialize presigner for token generation
			srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

			// Prepare data based on compression setting
			var dataToEncrypt []byte
			if tc.compressed {
				// Compress the data before encryption
				dataToEncrypt = compressData(tc.data)
			} else {
				// Use original data uncompressed
				dataToEncrypt = tc.data
			}

			// Encrypt the data
			encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, dataToEncrypt, tc.compressed)

			// Store the object
			objectKey := "test-key-" + tc.name
			ctx := context.Background()
			storeTestObject(t, srv.backend, ctx, "test-bucket", objectKey, encryptedData, hmacTable, armorMeta)

			// Generate share token
			token := generateTestToken(t, srv, "test-bucket", objectKey, time.Hour)

			// Make GET request
			req := httptest.NewRequest("GET", "/share/"+token, nil)
			w := httptest.NewRecorder()
			srv.handleShare(w, req)

			// Check response
			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			// Read response body
			retrievedData, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			// Verify response is non-empty — an empty body means the GET
			// produced nothing (object retrieval failed silently or
			// decompression yielded no output). The equality checks below
			// only catch a wrong body, not a missing one, so guard it here.
			if len(retrievedData) == 0 {
				t.Fatalf("%s: expected non-empty response body after decompression, got 0 bytes",
					tc.description)
			}

			// Verify retrieved data matches original (before compression)
			if !bytes.Equal(retrievedData, tc.data) {
				t.Errorf("%s: data mismatch.\nGot: %q (%d bytes)\nWant: %q (%d bytes)",
					tc.description,
					string(retrievedData), len(retrievedData),
					string(tc.data), len(tc.data))
			}

			// Explicitly verify decompression behavior
			if tc.wantDecompression {
				// For compressed objects, verify the retrieved data is NOT the same as encrypted data
				// (which would indicate decompression failed to run)
				if bytes.Equal(retrievedData, dataToEncrypt) {
					t.Errorf("%s: expected decompression to occur, but data appears unchanged (decompression may not have run)",
						tc.description)
				}
			} else {
				// For uncompressed objects, verify the retrieved data IS the same as input to encryption
				// (which confirms decompression was bypassed)
				if !bytes.Equal(retrievedData, dataToEncrypt) {
					t.Errorf("%s: expected no decompression, but data was modified (decompression may have run)",
						tc.description)
				}
			}

			// Verify metadata compression flag matches test case
			if armorMeta.Compressed != tc.compressed {
				t.Errorf("Metadata Compressed flag mismatch: got %v, want %v",
					armorMeta.Compressed, tc.compressed)
			}
		})
	}
}

// TestShareGET_LegacyObject tests legacy objects without Compressed flag
func TestShareGET_LegacyObject(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create filesystem backend
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem backend: %v", err)
	}

	cfg := loadTestConfig(t, tmpDir)
	srv, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize presigner for token generation
	srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

	// Create test data
	originalData := []byte("Hello, ARMOR! This is legacy test data.")

	// Encrypt without setting Compressed flag (legacy behavior)
	encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, originalData, false)
	armorMeta.Compressed = false // Explicitly set to false for legacy objects

	// Store the legacy object
	ctx := context.Background()
	storeTestObject(t, srv.backend, ctx, "test-bucket", "test-key-legacy", encryptedData, hmacTable, armorMeta)

	// Generate share token
	token := generateTestToken(t, srv, "test-bucket", "test-key-legacy", time.Hour)

	// Make GET request
	req := httptest.NewRequest("GET", "/share/"+token, nil)
	w := httptest.NewRecorder()
	srv.handleShare(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Read response body
	retrievedData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Verify legacy objects work unchanged
	if !bytes.Equal(retrievedData, originalData) {
		t.Errorf("Legacy object data mismatch.\nGot: %q (%d bytes)\nWant: %q (%d bytes)",
			string(retrievedData), len(retrievedData), string(originalData), len(originalData))
	}
}

// TestShareGET_RoundTrip tests complete round-trip: compress → encrypt → store → GET → decompress
func TestShareGET_RoundTrip(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create filesystem backend
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem backend: %v", err)
	}

	cfg := loadTestConfig(t, tmpDir)
	srv, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize presigner for token generation
	srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

	// Test various data sizes and types
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "small_text",
			data: []byte("Small text data for round-trip testing."),
		},
		{
			name: "medium_binary",
			data: generateRandomData(100 * 1024), // 100KB
		},
		{
			name: "large_binary",
			data: generateRandomData(512 * 1024), // 512KB (reduced from 1MB for speed)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: Compress the data
			compressedData := compressData(tc.data)

			// Step 2: Encrypt the compressed data
			encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, compressedData, true)

			// Step 3: Store in backend
			objectKey := "test-key-roundtrip-" + tc.name
			ctx := context.Background()
			storeTestObject(t, srv.backend, ctx, "test-bucket", objectKey, encryptedData, hmacTable, armorMeta)

			// Step 4: Generate share token
			token := generateTestToken(t, srv, "test-bucket", objectKey, time.Hour)

			// Step 5: GET from share endpoint
			req := httptest.NewRequest("GET", "/share/"+token, nil)
			w := httptest.NewRecorder()
			srv.handleShare(w, req)

			// Check response
			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			// Step 6: Read and verify round-trip
			retrievedData, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			// Verify complete round-trip: original → compress → encrypt → store → GET → decompress → original
			if !bytes.Equal(retrievedData, tc.data) {
				t.Errorf("Round-trip failed for %s.\nOriginal size: %d bytes\nRetrieved size: %d bytes",
					tc.name, len(tc.data), len(retrievedData))
			}

			// Verify compression ratio (compressed should be smaller for most data)
			if len(compressedData) >= len(tc.data) && tc.name != "small_text" {
				t.Logf("Note: Compression didn't reduce size for %s (compressed: %d, original: %d)",
					tc.name, len(compressedData), len(tc.data))
			}
		})
	}
}

// TestShareGET_CompressionDetectionFromFirstBlock tests that compression is detected from the first decrypted block
func TestShareGET_CompressionDetectionFromFirstBlock(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create filesystem backend
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem backend: %v", err)
	}

	cfg := loadTestConfig(t, tmpDir)
	srv, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize presigner for token generation
	srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

	// Create data that will be compressed
	originalData := []byte("Repeat data pattern for compression. " +
		"Repeat data pattern for compression. " +
		"Repeat data pattern for compression.")

	compressedData := compressData(originalData)

	// Verify zstd magic bytes are present
	if !crypto.IsCompressed(compressedData) {
		t.Error("Compressed data should have zstd magic bytes")
	}

	// Encrypt and store
	encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, compressedData, true)
	ctx := context.Background()
	storeTestObject(t, srv.backend, ctx, "test-bucket", "test-key-detect", encryptedData, hmacTable, armorMeta)

	// Generate token and make request
	token := generateTestToken(t, srv, "test-bucket", "test-key-detect", time.Hour)
	req := httptest.NewRequest("GET", "/share/"+token, nil)
	w := httptest.NewRecorder()
	srv.handleShare(w, req)

	// Verify successful decompression
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	retrievedData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !bytes.Equal(retrievedData, originalData) {
		t.Errorf("Data mismatch after compression detection and decompression")
	}
}

// TestShareGET_RangeRequestWithCompression tests that range requests work correctly with compressed data
func TestShareGET_RangeRequestWithCompression(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create filesystem backend
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem backend: %v", err)
	}

	cfg := loadTestConfig(t, tmpDir)
	srv, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize presigner for token generation
	srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

	// Create and compress test data
	originalData := generateRandomData(200 * 1024) // 200KB
	compressedData := compressData(originalData)

	// Encrypt and store
	encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, compressedData, true)
	ctx := context.Background()
	storeTestObject(t, srv.backend, ctx, "test-bucket", "test-key-range", encryptedData, hmacTable, armorMeta)

	// Generate token with range
	token := generateTestToken(t, srv, "test-bucket", "test-key-range", time.Hour)

	// Make range request
	req := httptest.NewRequest("GET", "/share/"+token, nil)
	req.Header.Set("Range", "bytes=0-9999") // Request first 10KB
	w := httptest.NewRecorder()
	srv.handleShare(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	// Range requests over compressed objects should return 416 (Range Not Satisfiable)
	// Compression destroys fixed-offset seeking (zstd is variable-length encoding)
	if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("Expected status 416 (Range Not Satisfiable) for compressed object, got %d", resp.StatusCode)
	}

	// Verify error message explains the limitation
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	errorMsg := string(body)
	if !strings.Contains(errorMsg, "Range reads unsupported on compressed objects") {
		t.Errorf("Expected error message about range reads on compressed objects, got: %s", errorMsg)
	}
}

// TestShareGET_RangeRequestUncompressed tests that range requests work correctly for uncompressed objects
func TestShareGET_RangeRequestUncompressed(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create filesystem backend
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem backend: %v", err)
	}

	cfg := loadTestConfig(t, tmpDir)
	srv, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize presigner for token generation
	srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

	// Create test data (NOT compressed)
	originalData := generateRandomData(200 * 1024) // 200KB

	// Encrypt and store WITHOUT compression
	encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, originalData, false)
	ctx := context.Background()
	storeTestObject(t, srv.backend, ctx, "test-bucket", "test-key-uncompressed-range", encryptedData, hmacTable, armorMeta)

	// Generate token
	token := generateTestToken(t, srv, "test-bucket", "test-key-uncompressed-range", time.Hour)

	// Make range request
	req := httptest.NewRequest("GET", "/share/"+token, nil)
	req.Header.Set("Range", "bytes=0-9999") // Request first 10KB
	w := httptest.NewRecorder()
	srv.handleShare(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	// Should return 206 Partial Content for successful range request
	if resp.StatusCode != http.StatusPartialContent {
		t.Errorf("Expected status 206 (Partial Content) for uncompressed object, got %d", resp.StatusCode)
	}

	// Verify we got the expected range of data
	retrievedData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Should have received 10KB (bytes 0-9999)
	expectedLen := 10000
	if len(retrievedData) != expectedLen {
		t.Errorf("Expected %d bytes from range request, got %d", expectedLen, len(retrievedData))
	}

	// Verify the data matches the original range
	if !bytes.Equal(retrievedData, originalData[:expectedLen]) {
		t.Errorf("Range data mismatch: retrieved data doesn't match original data range")
	}
}

// Helper functions

// setupTestEnvironment creates a temporary directory for testing
func setupTestEnvironment(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "armor-share-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// loadTestConfig loads test configuration
func loadTestConfig(t *testing.T, tmpDir string) *config.Config {
	t.Helper()

	// Set required environment variables
	os.Setenv("ARMOR_B2_REGION", "us-east-005")
	os.Setenv("ARMOR_B2_ENDPOINT", "https://s3.us-east-005.backblazeb2.com")
	os.Setenv("ARMOR_B2_ACCESS_KEY_ID", "testkey")
	os.Setenv("ARMOR_B2_SECRET_ACCESS_KEY", "testsecret")
	os.Setenv("ARMOR_BUCKET", "testbucket")
	os.Setenv("ARMOR_MEK", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	os.Setenv("ARMOR_PRESIGN_SECRET", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	os.Setenv("ARMOR_SECONDARY_BACKEND_TYPE", "filesystem")
	os.Setenv("ARMOR_SECONDARY_BACKEND_PATH", tmpDir)

	t.Cleanup(func() {
		// Unset environment variables
		os.Unsetenv("ARMOR_B2_REGION")
		os.Unsetenv("ARMOR_B2_ENDPOINT")
		os.Unsetenv("ARMOR_B2_ACCESS_KEY_ID")
		os.Unsetenv("ARMOR_B2_SECRET_ACCESS_KEY")
		os.Unsetenv("ARMOR_BUCKET")
		os.Unsetenv("ARMOR_MEK")
		os.Unsetenv("ARMOR_PRESIGN_SECRET")
		os.Unsetenv("ARMOR_SECONDARY_BACKEND_TYPE")
		os.Unsetenv("ARMOR_SECONDARY_BACKEND_PATH")
	})

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	return cfg
}

// encryptTestData encrypts test data with ARMOR encryption
func encryptTestData(t *testing.T, srv *Server, data []byte, compressed bool) ([]byte, []byte, *backend.ARMORMetadata) {
	t.Helper()

	// Get MEK from key manager
	mek, keyID, err := srv.keyManager.GetMEK("test-key")
	if err != nil {
		t.Fatalf("Failed to get MEK: %v", err)
	}

	// Generate DEK and IV
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("Failed to generate IV: %v", err)
	}

	// Create encryptor
	blockSize := 65536
	encryptor, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Encrypt data
	encrypted, hmacTable, err := encryptor.Encrypt(data)
	if err != nil {
		t.Fatalf("Failed to encrypt data: %v", err)
	}

	// Wrap DEK
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK: %v", err)
	}

	// Create ARMOR metadata
	armorMeta := &backend.ARMORMetadata{
		Version:       1,
		BlockSize:     blockSize,
		PlaintextSize: int64(len(data)),
		ContentType:   "application/octet-stream",
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		PlaintextSHA:  "test-sha256",
		ETag:          "test-etag",
		KeyID:         keyID,
		Compressed:    compressed,
	}

	return encrypted, hmacTable, armorMeta
}

// storeTestObject stores a test object in the backend
func storeTestObject(t *testing.T, backend backend.Backend, ctx context.Context, bucket, key string, encryptedData, hmacTable []byte, armorMeta *backend.ARMORMetadata) {
	t.Helper()

	// Build the ARMOR envelope: header + encrypted data + HMAC table
	// This matches the format expected by handleShareFullObject
	header, err := crypto.NewEnvelopeHeader(
		armorMeta.IV,
		armorMeta.PlaintextSize,
		armorMeta.BlockSize,
		[32]byte{}, // PlaintextSHA - placeholder for test data
	)
	if err != nil {
		t.Fatalf("Failed to create envelope header: %v", err)
	}
	headerBytes, err := header.Encode()
	if err != nil {
		t.Fatalf("Failed to encode envelope header: %v", err)
	}

	// Build full envelope: header + encrypted data + HMAC table
	objectSize := int64(len(headerBytes) + len(encryptedData) + len(hmacTable))
	fullObject := make([]byte, 0, objectSize)
	fullObject = append(fullObject, headerBytes...)
	fullObject = append(fullObject, encryptedData...)
	fullObject = append(fullObject, hmacTable...)

	// Convert ARMORMetadata to S3 metadata format using the existing ToMetadata method
	meta := armorMeta.ToMetadata()

	// Store the object
	err = backend.Put(ctx, bucket, key, bytes.NewReader(fullObject), objectSize, meta)
	if err != nil {
		t.Fatalf("Failed to store object: %v", err)
	}
}

// generateTestToken generates a test share token
func generateTestToken(t *testing.T, srv *Server, bucket, key string, expiration time.Duration) string {
	t.Helper()

	tokenStr, err := srv.presigner.GenerateToken(bucket, key, expiration)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	return tokenStr
}

// compressData is a helper function that compresses data using zstd
func compressData(data []byte) []byte {
	var buf bytes.Buffer
	encoder, err := zstd.NewWriter(&buf)
	if err != nil {
		panic(err)
	}
	encoder.Write(data)
	encoder.Close()
	return buf.Bytes()
}

// generateRandomData generates random data for testing
func generateRandomData(size int) []byte {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return data
}

// TestShareGET_EmptyObject tests that empty objects (0-byte plaintext) are handled correctly
func TestShareGET_EmptyObject(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create filesystem backend
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem backend: %v", err)
	}

	cfg := loadTestConfig(t, tmpDir)
	srv, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize presigner for token generation
	srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

	// Create empty data (0 bytes)
	originalData := []byte("")

	// Encrypt the empty data without compression
	encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, originalData, false)

	// Verify plaintext size is 0
	if armorMeta.PlaintextSize != 0 {
		t.Errorf("Expected PlaintextSize 0 for empty object, got %d", armorMeta.PlaintextSize)
	}

	// Store the empty object
	ctx := context.Background()
	storeTestObject(t, srv.backend, ctx, "test-bucket", "empty-object", encryptedData, hmacTable, armorMeta)

	// Generate share token
	token := generateTestToken(t, srv, "test-bucket", "empty-object", time.Hour)

	// Make GET request
	req := httptest.NewRequest("GET", "/share/"+token, nil)
	w := httptest.NewRecorder()
	srv.handleShare(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for empty object, got %d", resp.StatusCode)
	}

	// Read response body
	retrievedData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Verify empty object is returned correctly (0 bytes)
	if len(retrievedData) != 0 {
		t.Errorf("Expected empty body (0 bytes), got %d bytes: %q", len(retrievedData), string(retrievedData))
	}

	// Verify the data is empty
	if !bytes.Equal(retrievedData, originalData) {
		t.Errorf("Empty object data mismatch.\nGot: %q (%d bytes)\nWant: %q (%d bytes)",
			string(retrievedData), len(retrievedData), string(originalData), len(originalData))
	}
}

// TestShareGET_SingleByteObject tests that single-byte objects are handled correctly
func TestShareGET_SingleByteObject(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create filesystem backend
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem backend: %v", err)
	}

	cfg := loadTestConfig(t, tmpDir)
	srv, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize presigner for token generation
	srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

	// Test each possible single-byte value
	testCases := []struct {
		name  string
		data  []byte
		desc  string
	}{
		{
			name: "single_byte_zero",
			data: []byte{0x00},
			desc: "single byte with value 0x00",
		},
		{
			name: "single_byte_a",
			data: []byte{'a'},
			desc: "single ASCII character 'a'",
		},
		{
			name: "single_byte_ff",
			data: []byte{0xFF},
			desc: "single byte with value 0xFF",
		},
		{
			name: "single_byte_space",
			data: []byte{' '},
			desc: "single space character",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt the single-byte data without compression
			encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, tc.data, false)

			// Verify plaintext size is 1
			if armorMeta.PlaintextSize != 1 {
				t.Errorf("Expected PlaintextSize 1 for single-byte object, got %d", armorMeta.PlaintextSize)
			}

			// Store the single-byte object
			ctx := context.Background()
			objectKey := "single-byte-" + tc.name
			storeTestObject(t, srv.backend, ctx, "test-bucket", objectKey, encryptedData, hmacTable, armorMeta)

			// Generate share token
			token := generateTestToken(t, srv, "test-bucket", objectKey, time.Hour)

			// Make GET request
			req := httptest.NewRequest("GET", "/share/"+token, nil)
			w := httptest.NewRecorder()
			srv.handleShare(w, req)

			// Check response
			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 for single-byte object, got %d", resp.StatusCode)
			}

			// Read response body
			retrievedData, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			// Verify single byte is returned correctly
			if len(retrievedData) != 1 {
				t.Errorf("Expected 1 byte, got %d bytes: %q", len(retrievedData), retrievedData)
			}

			// Verify the exact byte value
			if !bytes.Equal(retrievedData, tc.data) {
				t.Errorf("%s: data mismatch.\nGot: %v (%d bytes)\nWant: %v (%d bytes)",
					tc.desc, retrievedData, len(retrievedData), tc.data, len(tc.data))
			}

			// Verify the exact byte value
			if retrievedData[0] != tc.data[0] {
				t.Errorf("%s: byte value mismatch.\nGot: 0x%02X\nWant: 0x%02X",
					tc.desc, retrievedData[0], tc.data[0])
			}
		})
	}
}

// TestShareGET_SmallObjectsCompressedFlag tests small objects (<4 bytes) with Compressed=true flag
// This exercises the Decompress len<4 early-return path where data is returned unchanged
func TestShareGET_SmallObjectsCompressedFlag(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create filesystem backend
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem backend: %v", err)
	}

	cfg := loadTestConfig(t, tmpDir)
	srv, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize presigner for token generation
	srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

	// Test small objects (< 4 bytes) with Compressed=true flag set
	// This exercises the Decompress early-return: len(compressed) < 4 returns data unchanged
	testCases := []struct {
		name       string
		data       []byte
		compressed bool
		desc       string
	}{
		{
			name:       "empty_compressed_flag",
			data:       []byte(""),
			compressed: true,
			desc:       "empty object (0 bytes) with Compressed=true - should return unchanged",
		},
		{
			name:       "one_byte_compressed_flag",
			data:       []byte{'X'},
			compressed: true,
			desc:       "single-byte object with Compressed=true - should return unchanged",
		},
		{
			name:       "two_bytes_compressed_flag",
			data:       []byte{'A', 'B'},
			compressed: true,
			desc:       "two-byte object with Compressed=true - should return unchanged",
		},
		{
			name:       "three_bytes_compressed_flag",
			data:       []byte{'1', '2', '3'},
			compressed: true,
			desc:       "three-byte object with Compressed=true - should return unchanged",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt the data WITH Compressed=true flag set
			// Note: We don't actually compress since it's < 4 bytes, we just set the flag
			encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, tc.data, tc.compressed)

			// Verify Compressed flag is set
			if !armorMeta.Compressed {
				t.Errorf("Expected Compressed flag to be true for test case %s", tc.name)
			}

			// Verify plaintext size matches
			if armorMeta.PlaintextSize != int64(len(tc.data)) {
				t.Errorf("PlaintextSize mismatch: got %d, want %d", armorMeta.PlaintextSize, len(tc.data))
			}

			// Store the object
			ctx := context.Background()
			objectKey := "small-" + tc.name
			storeTestObject(t, srv.backend, ctx, "test-bucket", objectKey, encryptedData, hmacTable, armorMeta)

			// Generate share token
			token := generateTestToken(t, srv, "test-bucket", objectKey, time.Hour)

			// Make GET request
			req := httptest.NewRequest("GET", "/share/"+token, nil)
			w := httptest.NewRecorder()
			srv.handleShare(w, req)

			// Check response - should return 200 with no panic
			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			// Read response body
			retrievedData, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			// Verify data is returned unchanged (exercising Decompress len<4 early-return)
			if !bytes.Equal(retrievedData, tc.data) {
				t.Errorf("%s: data mismatch.\nGot: %v (%d bytes)\nWant: %v (%d bytes)",
					tc.desc, retrievedData, len(retrievedData), tc.data, len(tc.data))
			}

			// For objects < 4 bytes with Compressed=true, the Decompress function
			// should return data unchanged (early return at len < 4)
			// Verify we got the original bytes, not an error or corruption
			if len(retrievedData) != len(tc.data) {
				t.Errorf("%s: length mismatch after Decompress early-return.\nGot: %d bytes\nWant: %d bytes",
					tc.desc, len(retrievedData), len(tc.data))
			}
		})
	}
}

// TestSmallCompressedObjectGet tests that objects <4 bytes with Compressed=true
// exercise the Decompress early-return path (crypto/decryptor.go:310-312)
// and return the bytes unchanged without error.
func TestSmallCompressedObjectGet(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create filesystem backend
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create filesystem backend: %v", err)
	}

	cfg := loadTestConfig(t, tmpDir)
	srv, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize presigner for token generation
	srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

	// Test case: 3-byte object with Compressed=true
	// This exercises the Decompress len<4 early-return at crypto/decryptor.go:310-312
	smallData := []byte("ABC") // 3 bytes (< 4 bytes threshold)

	// Encrypt the data WITH Compressed=true flag set
	// Note: We don't actually compress since it's < 4 bytes, we just set the flag
	encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, smallData, true)

	// Verify Compressed flag is set
	if !armorMeta.Compressed {
		t.Errorf("Expected Compressed flag to be true for small object test")
	}

	// Verify plaintext size matches (3 bytes)
	if armorMeta.PlaintextSize != 3 {
		t.Errorf("PlaintextSize mismatch: got %d, want %d", armorMeta.PlaintextSize, 3)
	}

	// Store the object
	ctx := context.Background()
	objectKey := "small-compressed-test"
	storeTestObject(t, srv.backend, ctx, "test-bucket", objectKey, encryptedData, hmacTable, armorMeta)

	// Generate share token
	token := generateTestToken(t, srv, "test-bucket", objectKey, time.Hour)

	// Make GET request
	req := httptest.NewRequest("GET", "/share/"+token, nil)
	w := httptest.NewRecorder()
	srv.handleShare(w, req)

	// Check response - should return HTTP 200 with no panic
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected HTTP 200 status for small compressed object, got %d", resp.StatusCode)
	}

	// Read response body
	retrievedData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Verify data is returned unchanged (exercising Decompress len<4 early-return)
	// The Decompress function at crypto/decryptor.go:310-312 should return data unchanged
	if !bytes.Equal(retrievedData, smallData) {
		t.Errorf("Small compressed object data mismatch.\nGot: %v (%d bytes)\nWant: %v (%d bytes)",
			retrievedData, len(retrievedData), smallData, len(smallData))
	}

	// Verify length is preserved (3 bytes)
	if len(retrievedData) != 3 {
		t.Errorf("Expected 3 bytes after Decompress early-return, got %d bytes", len(retrievedData))
	}

	// Verify the exact byte values
	if retrievedData[0] != smallData[0] || retrievedData[1] != smallData[1] || retrievedData[2] != smallData[2] {
		t.Errorf("Byte values corrupted after Decompress early-return.\nGot: %v\nWant: %v",
			retrievedData, smallData)
	}

	// Test confirms that the Decompress guard at crypto/decryptor.go:310-312 works correctly:
	// - For len(compressed) < 4, Decompress returns data unchanged with nil error
	// - No panic or error occurs from the early-return path
	// - The <4-byte compressed object is served correctly via GET
	t.Logf("✓ TestSmallCompressedObjectGet passed: Decompress early-return works correctly for <4-byte objects")
	t.Logf("  - Input: %d bytes with Compressed=true", len(smallData))
	t.Logf("  - Output: %d bytes unchanged", len(retrievedData))
	t.Logf("  - HTTP Status: %d", resp.StatusCode)
}
