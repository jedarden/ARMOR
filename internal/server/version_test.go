// Package server tests the version endpoint and Server header.
package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server"
	"github.com/jedarden/armor/internal/version"
)

// TestVersionEndpoint tests that GET /version returns the expected JSON response.
func TestVersionEndpoint(t *testing.T) {
	cfg := &config.Config{
		FormatWriteVersion: 2,
	}

	s, err := server.New(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	handler := s.Handler()

	// Test S3 API listener
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	// Parse JSON response
	var result struct {
		Version             string `json:"version"`
		FormatWriteVersion  int    `json:"format_write_version"`
		Go                  string `json:"go"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	// Verify version
	if result.Version != version.Version {
		t.Errorf("expected version %q, got %q", version.Version, result.Version)
	}

	// Verify format_write_version
	if result.FormatWriteVersion != 2 {
		t.Errorf("expected format_write_version 2, got %d", result.FormatWriteVersion)
	}

	// Verify Go version (should have "go" prefix stripped)
	expectedGoVersion := strings.TrimPrefix(runtime.Version(), "go")
	if result.Go != expectedGoVersion {
		t.Errorf("expected go version %q, got %q", expectedGoVersion, result.Go)
	}
}

// TestVersionEndpointOnAdminListener tests that GET /version works on the admin listener.
func TestVersionEndpointOnAdminListener(t *testing.T) {
	cfg := &config.Config{
		FormatWriteVersion: 2,
	}

	s, err := server.New(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	handler := s.AdminHandler()

	// Test admin API listener
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	// Parse JSON response
	var result struct {
		Version             string `json:"version"`
		FormatWriteVersion  int    `json:"format_write_version"`
		Go                  string `json:"go"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	// Verify version
	if result.Version != version.Version {
		t.Errorf("expected version %q, got %q", version.Version, result.Version)
	}
}

// TestVersionEndpointOnlyAcceptsGet tests that /version rejects non-GET requests.
func TestVersionEndpointOnlyAcceptsGet(t *testing.T) {
	cfg := &config.Config{
		FormatWriteVersion: 2,
	}

	s, err := server.New(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	handler := s.Handler()

	// Test POST request (should fail)
	req := httptest.NewRequest(http.MethodPost, "/version", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405 for POST, got %d", w.Code)
	}
}

// TestServerHeaderOnS3Listener tests that all S3 API responses include Server: ARMOR/<version>.
func TestServerHeaderOnS3Listener(t *testing.T) {
	cfg := &config.Config{
		FormatWriteVersion: 2,
	}

	s, err := server.New(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	handler := s.Handler()

	tests := []struct {
		name           string
		method         string
		path           string
		expectServer   bool
		expectedServer string
	}{
		{
			name:         "healthz endpoint",
			method:       http.MethodGet,
			path:         "/healthz",
			expectServer: true,
		},
		{
			name:         "readyz endpoint",
			method:       http.MethodGet,
			path:         "/readyz",
			expectServer: true,
		},
		{
			name:         "version endpoint",
			method:       http.MethodGet,
			path:         "/version",
			expectServer: true,
		},
		{
			name:         "404 Not Found",
			method:       http.MethodGet,
			path:         "/nonexistent",
			expectServer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			serverHeader := w.Header().Get("Server")
			if tt.expectServer && serverHeader == "" {
				t.Error("expected Server header to be set, but it was empty")
			}
			if tt.expectServer && serverHeader != "" {
				expectedServer := "ARMOR/" + version.Version
				if serverHeader != expectedServer {
					t.Errorf("expected Server header %q, got %q", expectedServer, serverHeader)
				}
			}
		})
	}
}

// TestServerHeaderOnAdminListener tests that all admin API responses include Server: ARMOR/<version>.
func TestServerHeaderOnAdminListener(t *testing.T) {
	cfg := &config.Config{
		FormatWriteVersion: 2,
	}

	s, err := server.New(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	handler := s.AdminHandler()

	tests := []struct {
		name           string
		method         string
		path           string
		expectServer   bool
		expectedServer string
	}{
		{
			name:         "healthz endpoint",
			method:       http.MethodGet,
			path:         "/healthz",
			expectServer: true,
		},
		{
			name:         "version endpoint",
			method:       http.MethodGet,
			path:         "/version",
			expectServer: true,
		},
		{
			name:         "404 Not Found",
			method:       http.MethodGet,
			path:         "/nonexistent",
			expectServer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			serverHeader := w.Header().Get("Server")
			if tt.expectServer && serverHeader == "" {
				t.Error("expected Server header to be set, but it was empty")
			}
			if tt.expectServer && serverHeader != "" {
				expectedServer := "ARMOR/" + version.Version
				if serverHeader != expectedServer {
					t.Errorf("expected Server header %q, got %q", expectedServer, serverHeader)
				}
			}
		})
	}
}
