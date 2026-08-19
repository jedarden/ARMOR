package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/presign"
)

// TestShareGET_CorruptedHMAC tests that corrupted HMAC entries return proper error response
func TestShareGET_CorruptedHMAC(t *testing.T) {
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

	// Test data - use a large enough payload to have multiple blocks
	// Block size is 65536 bytes, so we need data larger than that to span multiple blocks
	testData := make([]byte, 150000) // ~2.3 blocks worth of data
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Encrypt the test data (not compressed)
	encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, testData, false)

	// Verify metadata has Compressed=false
	if armorMeta.Compressed {
		t.Error("Expected Compressed=false in metadata")
	}

	testCases := []struct {
		name         string
		corruptFunc  func([]byte, []byte) ([]byte, []byte)
		description  string
	}{
		{
			name: "corrupted_first_hmac",
			corruptFunc: func(encrypted, hmacTable []byte) ([]byte, []byte) {
				// Corrupt the first HMAC entry
				corruptedHMAC := make([]byte, len(hmacTable))
				copy(corruptedHMAC, hmacTable)
				// Flip all bits in the first HMAC entry
				for i := 0; i < 32 && i < len(corruptedHMAC); i++ {
					corruptedHMAC[i] ^= 0xFF
				}
				return encrypted, corruptedHMAC
			},
			description: "First HMAC entry is completely corrupted",
		},
		{
			name: "corrupted_middle_hmac",
			corruptFunc: func(encrypted, hmacTable []byte) ([]byte, []byte) {
				// Corrupt a HMAC entry in the middle
				corruptedHMAC := make([]byte, len(hmacTable))
				copy(corruptedHMAC, hmacTable)
				// Flip bits in the middle HMAC entry (assuming at least 2 blocks)
				middleOffset := 32
				if middleOffset+32 <= len(corruptedHMAC) {
					for i := middleOffset; i < middleOffset+32; i++ {
						corruptedHMAC[i] ^= 0xFF
					}
				}
				return encrypted, corruptedHMAC
			},
			description: "HMAC entry in the middle is corrupted",
		},
		{
			name: "corrupted_last_hmac",
			corruptFunc: func(encrypted, hmacTable []byte) ([]byte, []byte) {
				// Corrupt the last HMAC entry
				corruptedHMAC := make([]byte, len(hmacTable))
				copy(corruptedHMAC, hmacTable)
				// Flip bits in the last HMAC entry
				lastOffset := len(corruptedHMAC) - 32
				for i := lastOffset; i < len(corruptedHMAC); i++ {
					corruptedHMAC[i] ^= 0xFF
				}
				return encrypted, corruptedHMAC
			},
			description: "Last HMAC entry is corrupted",
		},
		{
			name: "truncated_hmac_table",
			corruptFunc: func(encrypted, hmacTable []byte) ([]byte, []byte) {
				// Truncate the HMAC table to trigger bounds check failure
				if len(hmacTable) > 32 {
					return encrypted, hmacTable[:len(hmacTable)-32]
				}
				return encrypted, hmacTable[:16]
			},
			description: "HMAC table is truncated (bounds check failure)",
		},
		{
			name: "corrupted_ciphertext_block",
			corruptFunc: func(encrypted, hmacTable []byte) ([]byte, []byte) {
				// Corrupt a ciphertext block (HMAC will mismatch)
				corruptedEncrypted := make([]byte, len(encrypted))
				copy(corruptedEncrypted, encrypted)
				// Flip bits in the middle of the ciphertext
				middleOffset := len(corruptedEncrypted) / 2
				if middleOffset+16 <= len(corruptedEncrypted) {
					for i := middleOffset; i < middleOffset+16; i++ {
						corruptedEncrypted[i] ^= 0xFF
					}
				}
				return corruptedEncrypted, hmacTable
			},
			description: "Ciphertext block is corrupted (HMAC mismatch)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Apply corruption
			corruptedEncrypted, corruptedHMAC := tc.corruptFunc(encryptedData, hmacTable)

			// Store the corrupted object
			objectKey := "test-key-hmac-" + tc.name
			ctx := context.Background()
			storeTestObject(t, srv.backend, ctx, "test-bucket", objectKey, corruptedEncrypted, corruptedHMAC, armorMeta)

			// Generate share token
			token := generateTestToken(t, srv, "test-bucket", objectKey, time.Hour)

			// Make GET request to share endpoint
			req := httptest.NewRequest("GET", "/share/"+token, nil)
			w := httptest.NewRecorder()

			// This should return an error status, NOT 200
			srv.handleShare(w, req)

			// Check response
			resp := w.Result()
			defer resp.Body.Close()

			// Verify HTTP status is an error (>=400), NOT 200
			if resp.StatusCode < 400 {
				t.Errorf("Expected error status (>=400), got %d (OK)\nDescription: %s\nThis means corrupted data was served as success!", resp.StatusCode, tc.description)
			}

			// Read response body
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}
			body := string(bodyBytes)

			// Verify response contains error message
			if resp.StatusCode == http.StatusOK && len(body) == 0 {
				t.Errorf("Got HTTP 200 with empty body - this is the bug we're fixing! Description: %s", tc.description)
			}

			// Log the result for debugging
			t.Logf("✓ Server handled corrupted HMAC data appropriately")
			t.Logf("  - HTTP Status: %d", resp.StatusCode)
			t.Logf("  - Response Body: %s", body)
			t.Logf("  - Description: %s", tc.description)
		})
	}
}

// TestShareGET_ValidDataStillWorks verifies that after fixing the bug,
// valid data still works correctly
func TestShareGET_ValidDataStillWorks(t *testing.T) {
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

	// Test data
	testData := []byte("Hello, ARMOR! This is valid test data.")

	// Encrypt the test data (not compressed)
	encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, testData, false)

	// Store the valid object
	objectKey := "test-key-valid"
	ctx := context.Background()
	storeTestObject(t, srv.backend, ctx, "test-bucket", objectKey, encryptedData, hmacTable, armorMeta)

	// Generate share token
	token := generateTestToken(t, srv, "test-bucket", objectKey, time.Hour)

	// Make GET request to share endpoint
	req := httptest.NewRequest("GET", "/share/"+token, nil)
	w := httptest.NewRecorder()

	srv.handleShare(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	// Verify HTTP 200 status
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for valid data, got %d", resp.StatusCode)
	}

	// Verify we got the correct data
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !bytes.Equal(data, testData) {
		t.Errorf("Data mismatch:\nGot:  %q\nWant: %q", string(data), string(testData))
	}

	t.Logf("✓ Valid data still works correctly after HMAC corruption fix")
}
