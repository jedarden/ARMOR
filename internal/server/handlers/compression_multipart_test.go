// Package handlers tests compression behavior with multipart uploads.
package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
)

// TestCreateMultipartUpload_CompressionRejected verifies that CreateMultipartUpload
// returns InvalidArgument when compression is enabled (ADR-007).
func TestCreateMultipartUpload_CompressionRejected(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fsBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("create filesystem backend: %v", err)
	}

	// Test with compression enabled
	cfg := loadTestConfig(t, tmpDir)
	cfg.Compress = true

	server, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("create test server: %v", err)
	}

	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-key"

	// Attempt to create multipart upload with compression enabled
	req := httptest.NewRequest("POST", "/"+bucket+"/"+key+"?uploads", nil)
	recorder := httptest.NewRecorder()

	server.CreateMultipartUpload(recorder, req.WithContext(ctx), bucket, key)

	resp := recorder.Result()
	defer resp.Body.Close()

	// Verify error response
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for multipart with compression, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	bodyStr := string(body[:n])

	if !contains(bodyStr, "InvalidArgument") {
		t.Errorf("Expected InvalidArgument error, got: %s", bodyStr)
	}
	if !contains(bodyStr, "Compression is not supported for multipart uploads") {
		t.Errorf("Expected multipart rejection message, got: %s", bodyStr)
	}
}

// TestCreateMultipartUpload_CompressionDisabled verifies that CreateMultipartUpload
// works normally when compression is disabled.
func TestCreateMultipartUpload_CompressionDisabled(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	fsBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("create filesystem backend: %v", err)
	}

	// Test with compression disabled (default)
	cfg := loadTestConfig(t, tmpDir)
	cfg.Compress = false

	server, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("create test server: %v", err)
	}

	// Initialize key manager
	km := keymanager.New(cfg.MEK, cfg.NamedKeys, cfg.KeyRoutes)
	server.SetKeyManager(km)

	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-key"

	// Create multipart upload with compression disabled
	req := httptest.NewRequest("POST", "/"+bucket+"/"+key+"?uploads", nil)
	recorder := httptest.NewRecorder()

	server.CreateMultipartUpload(recorder, req.WithContext(ctx), bucket, key)

	resp := recorder.Result()
	defer resp.Body.Close()

	// Should succeed
	if resp.StatusCode != http.StatusOK {
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		t.Errorf("Expected status %d for multipart without compression, got %d: %s", http.StatusOK, resp.StatusCode, string(body[:n]))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func setupTestEnvironment(t *testing.T) (string, func()) {
	// This should be defined in the test utilities
	// For now, create a minimal implementation
	t.Helper()
	tmpDir := t.TempDir()
	return tmpDir, func() {}
}

func loadTestConfig(t *testing.T, tmpDir string) *config.Config {
	t.Helper()

	// Generate a test MEK
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	return &config.Config{
		Bucket:     "test-bucket",
		MEK:        mek,
		BlockSize:  65536,
		Compress:   false,
	}
}

func NewWithBackend(cfg *config.Config, be backend.Backend) (*Handlers, error) {
	handlers := New(cfg, be, nil, nil, nil, nil)
	return handlers, nil
}

func (h *Handlers) SetKeyManager(km *keymanager.KeyManager) {
	h.keyManager = km
}
