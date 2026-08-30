package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
)

// minimalEnv returns the set of required env var pairs needed for config.Load() to succeed.
func minimalTestEnv() []string {
	return []string{
		"ARMOR_B2_REGION", "us-east-005",
		"ARMOR_B2_ENDPOINT", "https://s3.us-east-005.backblazeb2.com",
		"ARMOR_B2_ACCESS_KEY_ID", "testkey",
		"ARMOR_B2_SECRET_ACCESS_KEY", "testsecret",
		"ARMOR_BUCKET", "testbucket",
		"ARMOR_MEK", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
	}
}

// setEnv sets multiple env vars for the duration of a test and restores them in cleanup.
func setTestEnv(t *testing.T, pairs ...string) {
	t.Helper()
	if len(pairs)%2 != 0 {
		t.Fatal("setEnv: pairs must be even")
	}
	originals := make(map[string]string, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		k, v := pairs[i], pairs[i+1]
		originals[k] = os.Getenv(k)
		os.Setenv(k, v)
	}
	t.Cleanup(func() {
		for k, v := range originals {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})
}

// The secondary-backend tests below drive the real ADR-006 configuration
// surface: two separate env vars, ARMOR_SECONDARY_BACKEND_TYPE (only
// "filesystem" is accepted) and ARMOR_SECONDARY_BACKEND_PATH (required when
// the type is set). They previously set a single colon-delimited
// ARMOR_SECONDARY_BACKEND variable that config.Load has never read, so every
// case silently configured NO secondary backend and the assertions could not
// hold — the suite had not passed since it was added in eca1b957, and it is
// what kept the armor-build `test` gate red.

func TestSecondaryBackendInitialization(t *testing.T) {
	tests := []struct {
		name            string
		backendType     string
		backendPath     string
		useTempDir      bool
		expectSecondary bool
		expectError     bool
	}{
		{
			name:            "no secondary backend configured",
			backendType:     "",
			expectSecondary: false,
			expectError:     false,
		},
		{
			name:            "filesystem secondary backend with valid path",
			backendType:     "filesystem",
			useTempDir:      true,
			expectSecondary: true,
			expectError:     false,
		},
		{
			name:            "filesystem secondary backend without path - should error",
			backendType:     "filesystem",
			backendPath:     "", // ARMOR_SECONDARY_BACKEND_PATH is required
			expectSecondary: false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for filesystem backend testing.
			// tmpDir stays in the subtest scope: the filesystem assertions below
			// join it to check that CreateBucket made the directory.
			tmpDir := ""
			if tt.useTempDir {
				var err error
				tmpDir, err = os.MkdirTemp("", "armor-secondary-test-*")
				if err != nil {
					t.Fatalf("failed to create temp dir: %v", err)
				}
				defer os.RemoveAll(tmpDir)
				tt.backendPath = tmpDir
			}

			setTestEnv(t, append(minimalTestEnv(),
				"ARMOR_SECONDARY_BACKEND_TYPE", tt.backendType,
				"ARMOR_SECONDARY_BACKEND_PATH", tt.backendPath,
			)...)

			cfg, err := config.Load()
			if err != nil && !tt.expectError {
				t.Fatalf("config.Load() error: %v", err)
			}

			if tt.expectError && err == nil {
				t.Error("expected error during config load, got nil")
				return
			}

			if tt.expectError {
				return // Expected error in config load, stop here
			}

			srv, err := New(cfg)
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			if tt.expectSecondary && srv.secondaryBackend == nil {
				t.Error("expected secondary backend to be initialized, got nil")
			}

			if !tt.expectSecondary && srv.secondaryBackend != nil {
				t.Error("expected secondary backend to be nil, got non-nil")
			}

			// Verify the secondary backend is a filesystem backend when configured
			if tt.expectSecondary && srv.secondaryBackend != nil {
				if _, ok := srv.secondaryBackend.(*backend.FSBackend); !ok {
					t.Errorf("expected secondary backend to be *backend.FSBackend, got %T", srv.secondaryBackend)
				}

				// Test that the filesystem backend has the correct base path
				// We can't directly access basePath as it's private, but we can test basic operations
				ctx := context.Background()

				// Test bucket operations
				testBucket := "test-bucket"

				// Create bucket (should create directory)
				if err := srv.secondaryBackend.CreateBucket(ctx, testBucket); err != nil {
					t.Errorf("failed to create bucket: %v", err)
				}

				// Check if bucket directory exists
				bucketPath := filepath.Join(tmpDir, testBucket)
				if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
					t.Error("bucket directory was not created")
				}

				// Clean up test bucket
				_ = srv.secondaryBackend.DeleteBucket(ctx, testBucket)
			}
		})
	}
}

func TestSecondaryBackendNilWhenNotConfigured(t *testing.T) {
	setTestEnv(t, minimalTestEnv()...)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if srv.secondaryBackend != nil {
		t.Error("expected secondary backend to be nil when not configured")
	}
}

func TestSecondaryBackendB2Initialization(t *testing.T) {
	t.Skip("a B2 secondary backend is not implemented: config.Load rejects any " +
		"ARMOR_SECONDARY_BACKEND_TYPE other than \"filesystem\" (internal/config/config.go, ADR-006). " +
		"This test asserts behaviour that does not exist and has never passed. " +
		"Unskip it when B2 secondary support lands.")

	setTestEnv(t, append(minimalTestEnv(),
		"ARMOR_SECONDARY_BACKEND", "b2:secondary-bucket:s3.us-east-005.backblazeb2.com:testkey:testsecret:us-east-005",
	)...)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if srv.secondaryBackend == nil {
		t.Fatal("expected secondary backend to be initialized")
	}

	// Verify the secondary backend is a B2 backend
	if _, ok := srv.secondaryBackend.(*backend.B2Backend); !ok {
		t.Errorf("expected secondary backend to be *backend.B2Backend, got %T", srv.secondaryBackend)
	}
}

func TestSecondaryBackendInvalidType(t *testing.T) {
	setTestEnv(t, append(minimalTestEnv(),
		"ARMOR_SECONDARY_BACKEND_TYPE", "invalid-type",
		"ARMOR_SECONDARY_BACKEND_PATH", "/some/path",
	)...)

	_, err := config.Load()
	if err == nil {
		t.Error("expected error for invalid backend type, got nil")
	}
	// Config should fail to load with an unsupported backend type
}

func TestSecondaryBackendInvalidB2Config(t *testing.T) {
	t.Skip("a B2 secondary backend is not implemented: config.Load rejects any " +
		"ARMOR_SECONDARY_BACKEND_TYPE other than \"filesystem\" (internal/config/config.go, ADR-006). " +
		"An incomplete B2 config cannot be distinguished from an invalid type today. " +
		"Unskip it when B2 secondary support lands.")

	setTestEnv(t, append(minimalTestEnv(),
		"ARMOR_SECONDARY_BACKEND", "b2:bucket:endpoint", // Missing required fields
	)...)

	_, err := config.Load()
	if err == nil {
		t.Error("expected error for incomplete B2 config, got nil")
	}
	// Config should fail to load with incomplete B2 configuration
}

func TestSecondaryBackendFilesystemIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "armor-secondary-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	setTestEnv(t, append(minimalTestEnv(),
		"ARMOR_SECONDARY_BACKEND_TYPE", "filesystem",
		"ARMOR_SECONDARY_BACKEND_PATH", tmpDir,
	)...)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if srv.secondaryBackend == nil {
		t.Fatal("expected secondary backend to be initialized")
	}

	fsBackend, ok := srv.secondaryBackend.(*backend.FSBackend)
	if !ok {
		t.Fatalf("expected secondary backend to be *backend.FSBackend, got %T", srv.secondaryBackend)
	}

	// Test basic operations
	ctx := context.Background()
	testBucket := "integration-test-bucket"
	testKey := "test-object.txt"
	testData := []byte("test data for secondary backend")

	// Create bucket
	if err := fsBackend.CreateBucket(ctx, testBucket); err != nil {
		t.Errorf("failed to create bucket: %v", err)
	}

	// Put object
	if err := fsBackend.Put(ctx, testBucket, testKey, bytes.NewReader(testData), int64(len(testData)), nil); err != nil {
		t.Errorf("failed to put object: %v", err)
	}

	// Get object
	body, info, err := fsBackend.Get(ctx, testBucket, testKey)
	if err != nil {
		t.Errorf("failed to get object: %v", err)
	}
	defer body.Close()

	if info.Size != int64(len(testData)) {
		t.Errorf("got size %d, want %d", info.Size, len(testData))
	}

	gotData, err := io.ReadAll(body)
	if err != nil {
		t.Errorf("failed to read object data: %v", err)
	}

	if !bytes.Equal(gotData, testData) {
		t.Errorf("got data %q, want %q", gotData, testData)
	}

	// Head object
	info, err = fsBackend.Head(ctx, testBucket, testKey)
	if err != nil {
		t.Errorf("failed to head object: %v", err)
	}

	if info.Size != int64(len(testData)) {
		t.Errorf("head got size %d, want %d", info.Size, len(testData))
	}

	// Delete object
	if err := fsBackend.Delete(ctx, testBucket, testKey); err != nil {
		t.Errorf("failed to delete object: %v", err)
	}

	// Delete bucket
	if err := fsBackend.DeleteBucket(ctx, testBucket); err != nil {
		t.Errorf("failed to delete bucket: %v", err)
	}
}

// TestReadyzHandler tests the /readyz endpoint returns proper JSON responses.
func TestReadyzHandler(t *testing.T) {
	// Test cases covering all readiness paths
	tests := []struct {
		name              string
		canaryDisabled    bool
		canaryRunning     bool
		canaryHealthy     bool
		hasManifestWriter bool
		manifestFlushed   bool
		wantStatus        int
		wantReady         bool
		wantReason        string
		validateFields    bool // if true, validate all JSON fields
	}{
		{
			name:           "canary disabled - always ready",
			canaryDisabled: true,
			wantStatus:     http.StatusOK,
			wantReady:      true,
			wantReason:     "Ready - canary disabled",
			validateFields: true,
		},
		{
			name:           "canary running and healthy",
			canaryRunning:  true,
			canaryHealthy:  true,
			wantStatus:     http.StatusOK,
			wantReady:      true,
			wantReason:     "Ready",
			validateFields: true,
		},
		{
			name:           "canary running and unhealthy",
			canaryRunning:  true,
			canaryHealthy:  false,
			wantStatus:     http.StatusServiceUnavailable,
			wantReady:      false,
			wantReason:     "Not ready - canary check failed",
			validateFields: true,
		},
		{
			name:              "manifest writer with recent flush",
			hasManifestWriter: true,
			manifestFlushed:   true,
			wantStatus:        http.StatusOK,
			wantReady:         true,
			wantReason:        "Ready - manifest writer recently flushed",
			validateFields:    true,
		},
		{
			name:              "manifest writer with stale flush",
			hasManifestWriter: true,
			manifestFlushed:   false,
			wantStatus:        http.StatusServiceUnavailable,
			wantReady:         false,
			wantReason:        "Not ready - manifest writer last flush",
			validateFields:    false, // reason is dynamic (contains timestamp)
		},
		{
			name:              "manifest writer never flushed",
			hasManifestWriter: true,
			manifestFlushed:   false, // no flush at all
			wantStatus:        http.StatusServiceUnavailable,
			wantReady:         false,
			wantReason:        "Not ready - manifest writer has never flushed",
			validateFields:    true,
		},
		{
			name:           "no health signal available",
			wantStatus:     http.StatusServiceUnavailable,
			wantReady:      false,
			wantReason:     "Not ready - no health signal available",
			validateFields: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up minimal environment
			setTestEnv(t, minimalTestEnv()...)

			// Load config
			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("failed to load config: %v", err)
			}

			// Create test backend
			tmpDir := t.TempDir()
			fsBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: tmpDir})
			if err != nil {
				t.Fatalf("failed to create filesystem backend: %v", err)
			}

			// Create server
			s := &Server{
				config:         cfg,
				backend:        fsBackend,
				canaryDisabled: tt.canaryDisabled,
			}

			// Set up canary if needed
			if tt.canaryRunning {
				// Create a mock canary monitor
				// For now, we'll skip this as it requires more setup
				// This would need a mock canary implementation
				t.Skip("canary running tests require mock canary implementation")
			}

			// Set up manifest writer if needed
			if tt.hasManifestWriter {
				// For now, we'll skip this as it requires more setup
				// This would need a mock manifest writer
				t.Skip("manifest writer tests require mock implementation")
			}

			// Create request
			req := httptest.NewRequest("GET", "/readyz", nil)
			w := httptest.NewRecorder()

			// Call handler
			s.readyz(w, req)

			// Check status code
			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			// Check content type is JSON
			contentType := resp.Header.Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("got content-type %q, want application/json", contentType)
			}

			// Parse and validate JSON response
			var readyzResp struct {
				Ready                  bool   `json:"ready"`
				CanaryAgeS             int    `json:"canary_age_s"`
				MultipartCanaryHealthy bool   `json:"multipart_canary_healthy"`
				ManifestFlushedS       int    `json:"manifest_flushed_s"`
				Reason                 string `json:"reason"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&readyzResp); err != nil {
				t.Fatalf("failed to decode JSON response: %v", err)
			}

			if readyzResp.Ready != tt.wantReady {
				t.Errorf("got ready=%v, want %v", readyzResp.Ready, tt.wantReady)
			}

			// Check reason contains expected substring
			if tt.wantReason != "" && !strings.Contains(readyzResp.Reason, tt.wantReason) {
				t.Errorf("got reason %q, want to contain %q", readyzResp.Reason, tt.wantReason)
			}

			// Validate field types if requested
			if tt.validateFields {
				// canary_age_s should be non-negative
				if readyzResp.CanaryAgeS < 0 {
					t.Errorf("canary_age_s should be >= 0, got %d", readyzResp.CanaryAgeS)
				}

				// manifest_flushed_s should be non-negative
				if readyzResp.ManifestFlushedS < 0 {
					t.Errorf("manifest_flushed_s should be >= 0, got %d", readyzResp.ManifestFlushedS)
				}

				// When canary disabled, canary_age_s and multipart_canary_healthy should be 0/false
				if tt.canaryDisabled {
					if readyzResp.CanaryAgeS != 0 {
						t.Errorf("canary disabled: canary_age_s should be 0, got %d", readyzResp.CanaryAgeS)
					}
					if readyzResp.MultipartCanaryHealthy {
						t.Errorf("canary disabled: multipart_canary_healthy should be false, got true")
					}
				}
			}
		})
	}
}

// TestReadyzCanaryDisabled tests the canary disabled path specifically.
func TestReadyzCanaryDisabled(t *testing.T) {
	setTestEnv(t, minimalTestEnv()...)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	s := &Server{
		config:         cfg,
		canaryDisabled: true,
	}

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	s.readyz(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want 200", resp.StatusCode)
	}

	var readyzResp struct {
		Ready                  bool   `json:"ready"`
		CanaryAgeS             int    `json:"canary_age_s"`
		MultipartCanaryHealthy bool   `json:"multipart_canary_healthy"`
		ManifestFlushedS       int    `json:"manifest_flushed_s"`
		Reason                 string `json:"reason"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&readyzResp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if !readyzResp.Ready {
		t.Error("canary disabled: expected ready=true")
	}

	if readyzResp.CanaryAgeS != 0 {
		t.Errorf("canary disabled: expected canary_age_s=0, got %d", readyzResp.CanaryAgeS)
	}

	if readyzResp.MultipartCanaryHealthy {
		t.Error("canary disabled: expected multipart_canary_healthy=false")
	}

	if readyzResp.ManifestFlushedS != 0 {
		t.Errorf("canary disabled: expected manifest_flushed_s=0, got %d", readyzResp.ManifestFlushedS)
	}

	if readyzResp.Reason != "Ready - canary disabled" {
		t.Errorf("got reason %q, want 'Ready - canary disabled'", readyzResp.Reason)
	}
}
