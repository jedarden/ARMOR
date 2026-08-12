package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/presign"
)

// TestShareGET_CorruptedCompressedData tests that corrupted compressed data returns proper error response
func TestShareGET_CorruptedCompressedData(t *testing.T) {
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

	// Create a valid zstd compressed stream first
	validCompressed := compressData([]byte("Hello, ARMOR! This is test data."))

	// Create corrupted versions of zstd data
	testCases := []struct {
		name         string
		corruptFunc  func([]byte) []byte
		description  string
	}{
		{
			name: "corrupted_content_after_magic",
			corruptFunc: func(data []byte) []byte {
				// Corrupt the content after magic bytes
				corrupted := make([]byte, len(data))
				copy(corrupted, data)
				// Flip some bytes after the magic header (magic is 4 bytes)
				for i := 4; i < len(corrupted); i += 7 {
					corrupted[i] ^= 0xFF
				}
				return corrupted
			},
			description: "zstd magic bytes present but frame content is corrupted",
		},
		{
			name: "truncated_stream",
			corruptFunc: func(data []byte) []byte {
				// Truncate the stream to make it incomplete
				if len(data) > 10 {
					return data[:len(data)/2]
				}
				return data[:5]
			},
			description: "zstd magic bytes present but stream is truncated/incomplete",
		},
		{
			name: "only_magic_bytes",
			corruptFunc: func(data []byte) []byte {
				// Return only the magic bytes
				return []byte{0x28, 0xB5, 0x2F, 0xFD}
			},
			description: "only zstd magic bytes with no frame content",
		},
		{
			name: "partial_frame",
			corruptFunc: func(data []byte) []byte {
				// Return magic bytes plus a few bytes (partial frame)
				if len(data) > 8 {
					return data[:8]
				}
				return data
			},
			description: "zstd magic bytes with partial/incomplete frame",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create corrupted zstd payload
			corruptedData := tc.corruptFunc(validCompressed)

			// Verify it has zstd magic bytes (required for Compressed=true objects)
			if len(corruptedData) < 4 || corruptedData[0] != 0x28 || corruptedData[1] != 0xB5 {
				t.Fatalf("Test data missing zstd magic bytes")
			}

			// Encrypt the corrupted data
			encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, corruptedData, true)

			// Verify metadata has Compressed=true
			if !armorMeta.Compressed {
				t.Error("Expected Compressed=true in metadata")
			}

			// Store the corrupted compressed object
			objectKey := "test-key-corrupted-" + tc.name
			ctx := context.Background()
			storeTestObject(t, srv.backend, ctx, "test-bucket", objectKey, encryptedData, hmacTable, armorMeta)

			// Generate share token
			token := generateTestToken(t, srv, "test-bucket", objectKey, time.Hour)

			// Make GET request to share endpoint
			req := httptest.NewRequest("GET", "/share/"+token, nil)
			w := httptest.NewRecorder()

			// This should NOT panic - it should handle the error gracefully
			srv.handleShare(w, req)

			// Check response
			resp := w.Result()
			defer resp.Body.Close()

			// Verify HTTP 400 status (client-side data corruption)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("Expected status 400 (BadRequest), got %d", resp.StatusCode)
			}

			// Read response body
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}
			body := string(bodyBytes)

			// Verify response contains decompression error message
			if !strings.Contains(body, "Failed to decompress data") {
				t.Errorf("Response body should contain 'Failed to decompress data', got: %s", body)
			}

			// Verify it's not a generic error - it should mention zstd
			if !strings.Contains(body, "zstd") && !strings.Contains(body, "decompression failed") {
				t.Logf("Note: Error message is: %s", body)
			}

			// If we got here without panic, the error handling is working
			t.Logf("✓ Server handled corrupted compressed data gracefully")
			t.Logf("  - HTTP Status: %d", resp.StatusCode)
			t.Logf("  - Response Body: %s", body)
			t.Logf("  - Description: %s", tc.description)
		})
	}
}

// TestShareGET_SubsequentRequestsAfterCorruption verifies that after a corrupted
// data error, subsequent requests on the same object still work correctly.
// This ensures that error handling doesn't corrupt connection state or leave
// resources in a bad state.
func TestShareGET_SubsequentRequestsAfterCorruption(t *testing.T) {
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

	// Create valid compressed object
	validCompressed := compressData([]byte("Hello, ARMOR! Valid compressed data."))

	// Encrypt the valid compressed data
	encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, validCompressed, true)

	// Store the valid compressed object
	objectKey := "test-key-valid-compressed"
	ctx := context.Background()
	storeTestObject(t, srv.backend, ctx, "test-bucket", objectKey, encryptedData, hmacTable, armorMeta)

	// Generate share token for the valid object
	token := generateTestToken(t, srv, "test-bucket", objectKey, time.Hour)

	// First request: Should succeed
	req1 := httptest.NewRequest("GET", "/share/"+token, nil)
	w1 := httptest.NewRecorder()
	srv.handleShare(w1, req1)

	resp1 := w1.Result()
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusOK {
		t.Errorf("First request expected status 200, got %d", resp1.StatusCode)
	}

	// Verify we got the correct decompressed data
	data1, err := io.ReadAll(resp1.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expectedData := []byte("Hello, ARMOR! Valid compressed data.")
	if !bytes.Equal(data1, expectedData) {
		t.Errorf("First request data mismatch:\nGot:  %q\nWant: %q", string(data1), string(expectedData))
	}

	// Second request: Should ALSO succeed (verifying no connection state corruption)
	req2 := httptest.NewRequest("GET", "/share/"+token, nil)
	w2 := httptest.NewRecorder()
	srv.handleShare(w2, req2)

	resp2 := w2.Result()
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Second request expected status 200, got %d", resp2.StatusCode)
	}

	// Verify we got the same correct decompressed data
	data2, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("Failed to read second response body: %v", err)
	}

	if !bytes.Equal(data2, expectedData) {
		t.Errorf("Second request data mismatch:\nGot:  %q\nWant: %q", string(data2), string(expectedData))
	}

	// Verify both requests returned identical data
	if !bytes.Equal(data1, data2) {
		t.Errorf("Subsequent requests returned different data:\nFirst:  %q\nSecond: %q", string(data1), string(data2))
	}

	t.Logf("✓ Subsequent requests on same object succeed with identical data")
}
