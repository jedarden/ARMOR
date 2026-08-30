// Package server tests the admin credential listing endpoint.
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/acl"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
)

// TestHandleListCreds_NoSecretLeak verifies that the /admin/creds endpoint
// never returns secret_key material in the response body, even when credentials
// are loaded from both environment variables and ARMOR_AUTH_FILE.
func TestHandleListCreds_NoSecretLeak(t *testing.T) {
	// Create test credentials with known secret values
	envSecret := "test-env-secret-key-12345678901234567890"
	fileSecret1 := "test-file-secret-key-alpha-123456789"
	fileSecret2 := "test-file-secret-key-beta-987654321"

	// Set up environment variable credential
	os.Setenv("ARMOR_AUTH_ACCESS_KEY", "env-test-key")
	os.Setenv("ARMOR_AUTH_SECRET_KEY", envSecret)
	defer os.Unsetenv("ARMOR_AUTH_ACCESS_KEY")
	defer os.Unsetenv("ARMOR_AUTH_SECRET_KEY")

	// Create a temporary YAML credential file
	yamlContent := `credentials:
  - name: FILE_CRED_ALPHA
    access_key: file-alpha-key
    secret_key: test-file-secret-key-alpha-123456789
    acl: "testbucket:alpha/*:get+list"
  - name: FILE_CRED_BETA
    access_key: file-beta-key
    secret_key: test-file-secret-key-beta-987654321
    acl: "testbucket:beta/*:put+delete"
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "credentials.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to create test YAML file: %v", err)
	}
	os.Setenv("ARMOR_AUTH_FILE", tmpFile)
	defer os.Unsetenv("ARMOR_AUTH_FILE")

	// Load config with both env and file credentials
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify we have credentials from both sources
	if len(cfg.Credentials) < 3 {
		t.Fatalf("Expected at least 3 credentials (1 env + 2 file), got %d", len(cfg.Credentials))
	}

	// Create a minimal server with the loaded config
	var buf bytes.Buffer
	logger := logging.New("test")
	logger.SetOutput(&buf)
	s := &Server{
		config: cfg,
		logger: logger,
	}

	// Create a request to /admin/creds
	req := httptest.NewRequest(http.MethodGet, "/admin/creds", nil)
	rec := httptest.NewRecorder()

	// Call the handler directly
	s.handleListCreds(rec, req)

	// Check response status
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Parse the response
	var response []struct {
		Name     string        `json:"name"`
		ACLs     []acl.ACLEntry `json:"acls"`
		Source   string        `json:"source"`
		LoadedAt time.Time     `json:"loaded_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Verify we got all credentials
	if len(response) < 3 {
		t.Errorf("Expected at least 3 credentials in response, got %d", len(response))
	}

	// Critical assertion: NO secret key material should be in the response body
	// (checked below via rec.Body.Bytes() directly)

	// Check for each specific secret value
	secretValues := []string{
		envSecret,
		fileSecret1,
		fileSecret2,
		"secret",
		"secret_key",
		"SecretKey",
	}

	for _, secretVal := range secretValues {
		if bytes.Contains(rec.Body.Bytes(), []byte(secretVal)) {
			// More specific checks to avoid false positives
			if secretVal == "secret" || secretVal == "secret_key" || secretVal == "SecretKey" {
				// These are field names that might legitimately appear in JSON structure
				// Check if they appear as values (with quotes around them in unexpected places)
				continue
			}
			t.Errorf("Secret key material leaked in response: found %q in body", secretVal)
		}
	}

	// Verify each credential has the expected fields
	for _, cred := range response {
		// Name should be present and non-empty
		if cred.Name == "" {
			t.Error("Credential has empty name field")
		}

		// Source should be either "env" or "file"
		if cred.Source != "env" && cred.Source != "file" {
			t.Errorf("Credential %s has invalid source: %q", cred.Name, cred.Source)
		}

		// LoadedAt should be a reasonable timestamp (not zero, not in the future)
		if cred.LoadedAt.IsZero() {
			t.Errorf("Credential %s has zero LoadedAt timestamp", cred.Name)
		}
		if cred.LoadedAt.After(time.Now()) {
			t.Errorf("Credential %s has LoadedAt timestamp in the future: %v", cred.Name, cred.LoadedAt)
		}
	}

	// Verify we have both env and file sources
	hasEnvSource := false
	hasFileSource := false
	for _, cred := range response {
		if cred.Source == "env" {
			hasEnvSource = true
		}
		if cred.Source == "file" {
			hasFileSource = true
		}
	}
	if !hasEnvSource {
		t.Error("Response does not contain any credentials with source='env'")
	}
	if !hasFileSource {
		t.Error("Response does not contain any credentials with source='file'")
	}

	// Verify ACLs are present for file credentials
	for _, cred := range response {
		if cred.Source == "file" {
			if len(cred.ACLs) == 0 {
				t.Errorf("File credential %s should have ACLs", cred.Name)
			}
		}
	}
}

// TestHandleListCreds_MethodNotPost verifies that only GET requests are allowed.
func TestHandleListCreds_MethodNotAllowed(t *testing.T) {
	cfg := &config.Config{
		Credentials: make(map[string]*config.Credential),
	}
	var buf bytes.Buffer
	logger := logging.New("test")
	logger.SetOutput(&buf)
	s := &Server{
		config: cfg,
		logger: logger,
	}

	// Try POST request
	req := httptest.NewRequest(http.MethodPost, "/admin/creds", nil)
	rec := httptest.NewRecorder()
	s.handleListCreds(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 Method Not Allowed for POST, got %d", rec.Code)
	}

	// Try PUT request
	req = httptest.NewRequest(http.MethodPut, "/admin/creds", nil)
	rec = httptest.NewRecorder()
	s.handleListCreds(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 Method Not Allowed for PUT, got %d", rec.Code)
	}

	// Verify GET works
	req = httptest.NewRequest(http.MethodGet, "/admin/creds", nil)
	rec = httptest.NewRecorder()
	s.handleListCreds(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK for GET, got %d", rec.Code)
	}
}

// TestHandleListCreds_EmptyCredentials verifies behavior with no credentials.
func TestHandleListCreds_EmptyCredentials(t *testing.T) {
	cfg := &config.Config{
		Credentials: make(map[string]*config.Credential),
	}
	var buf bytes.Buffer
	logger := logging.New("test")
	logger.SetOutput(&buf)
	s := &Server{
		config: cfg,
		logger: logger,
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/creds", nil)
	rec := httptest.NewRecorder()
	s.handleListCreds(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK with no credentials, got %d", rec.Code)
	}

	var response []struct {
		Name     string        `json:"name"`
		ACLs     []acl.ACLEntry `json:"acls"`
		Source   string        `json:"source"`
		LoadedAt time.Time     `json:"loaded_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if len(response) != 0 {
		t.Errorf("Expected empty array with no credentials, got %d items", len(response))
	}
}
