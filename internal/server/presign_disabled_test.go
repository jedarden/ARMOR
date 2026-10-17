package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// TestPresignDisabled_ShareRouteRequiresS3Auth verifies that an unregistered
// /share/ path falls through to the S3 root handler (197be975).
func TestPresignDisabled_ShareRouteRequiresS3Auth(t *testing.T) {
	// Create a minimal config with presign disabled
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Backend:        "filesystem",
		FSPath:         tmpDir,
		Bucket:         "test-bucket",
		MEK:            make([]byte, 32), // dummy MEK
		PresignEnabled: false,
	}

	// Create server with disabled presign
	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// The disabled share route is not registered, so the S3 auth gate rejects
	// this unauthenticated request before object routing.
	req := httptest.NewRequest("GET", "/share/some-token", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for unauthenticated /share/ when presign disabled, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "MissingAuthenticationToken") {
		t.Errorf("Expected S3 authentication error, got: %s", w.Body.String())
	}
}

// TestPresignDisabled_AdminPresignForbiddenWithoutAdminToken verifies the
// fail-closed admin gate added in c4b0f671 takes precedence over route handling.
func TestPresignDisabled_AdminPresignForbiddenWithoutAdminToken(t *testing.T) {
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
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	// Create server with disabled presign
	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// The admin token gate runs before the disabled route can return 404.
	req := httptest.NewRequest("POST", "/admin/presign", strings.NewReader(`{"key":"test.txt"}`))
	req.Header.Set("Content-Type", "application/json")
	// Add basic auth header
	req.SetBasicAuth("test-key", "test-secret")
	w := httptest.NewRecorder()
	server.AdminHandler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for /admin/presign without admin token, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "admin API disabled: ARMOR_ADMIN_TOKEN not set") {
		t.Errorf("Expected disabled admin API message, got: %s", w.Body.String())
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
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
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
