package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// TestPresignDisabled_ShareRoute404s tests that /share/ route returns 404 when presign is disabled
func TestPresignDisabled_ShareRoute404s(t *testing.T) {
	// Create a minimal config with presign disabled
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Backend:       "filesystem",
		FSPath:        tmpDir,
		Bucket:        "test-bucket",
		MEK:           make([]byte, 32), // dummy MEK
		PresignEnabled: false,
	}

	// Create server with disabled presign
	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test that /share/ route is not registered
	req := httptest.NewRequest("GET", "/share/some-token", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for /share/ when presign disabled, got %d", w.Code)
	}
}

// TestPresignDisabled_AdminPresign404s tests that /admin/presign returns 404 when presign is disabled
func TestPresignDisabled_AdminPresign404s(t *testing.T) {
	// Create a minimal config with presign disabled
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Backend:        "filesystem",
		FSPath:         tmpDir,
		Bucket:         "test-bucket",
		MEK:            make([]byte, 32), // dummy MEK
		PresignEnabled: false,
		// Add auth credentials for admin API
		AuthAccessKey: "test-key",
		AuthSecretKey: "test-secret",
	}

	// Create server with disabled presign
	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test that /admin/presign returns 404
	req := httptest.NewRequest("POST", "/admin/presign", strings.NewReader(`{"key":"test.txt"}`))
	req.Header.Set("Content-Type", "application/json")
	// Add basic auth header
	req.SetBasicAuth("test-key", "test-secret")
	w := httptest.NewRecorder()
	server.AdminHandler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for /admin/presign when presign disabled, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Pre-signing is not enabled") {
		t.Errorf("Expected error message about pre-signing not enabled, got: %s", w.Body.String())
	}
}

// TestPresignEnabled_RoutesRegistered tests that routes are properly registered when presign is enabled
func TestPresignEnabled_RoutesRegistered(t *testing.T) {
	// Create a minimal config with presign enabled
	tmpDir := t.TempDir()
	
	// Generate valid presign secret (32 bytes)
	presignSecret := make([]byte, 32)
	for i := range presignSecret {
		presignSecret[i] = byte(i % 256)
	}
	
	cfg := &config.Config{
		Backend:        "filesystem",
		FSPath:         tmpDir,
		Bucket:         "test-bucket",
		MEK:            make([]byte, 32), // dummy MEK
		PresignEnabled: true,
		PresignSecret:  presignSecret,
		PresignBaseURL: "https://armor.example.com/share",
		// Add auth credentials for admin API
		AuthAccessKey: "test-key",
		AuthSecretKey: "test-secret",
	}

	// Create server with enabled presign
	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test that /share/ route is registered (will return bad request for invalid token, not 404)
	req := httptest.NewRequest("GET", "/share/invalid-token", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Errorf("Expected non-404 response for /share/ when presign enabled, got %d", w.Code)
	}

	// Test that /admin/presign is registered (will return unauthorized for missing auth, not 404)
	req2 := httptest.NewRequest("POST", "/admin/presign", strings.NewReader(`{"key":"test.txt"}`))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.AdminHandler().ServeHTTP(w2, req2)

	if w2.Code == http.StatusNotFound {
		t.Errorf("Expected non-404 response for /admin/presign when presign enabled, got %d", w2.Code)
	}
}
