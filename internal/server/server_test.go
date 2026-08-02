package server

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
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

func TestSecondaryBackendInitialization(t *testing.T) {
	tests := []struct {
		name                string
		secondaryBackendStr string
		expectSecondary     bool
		expectError         bool
	}{
		{
			name:                "no secondary backend configured",
			secondaryBackendStr: "",
			expectSecondary:     false,
			expectError:         false,
		},
		{
			name:                "filesystem secondary backend with valid path",
			secondaryBackendStr: "", // Will be set to temp dir in test
			expectSecondary:     true,
			expectError:         false,
		},
		{
			name:                "filesystem secondary backend without path - should error",
			secondaryBackendStr: "filesystem:", // Missing path
			expectSecondary:     false,
			expectError:         true, // missing path causes config load to fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for filesystem backend testing
			tmpDir := ""
			if tt.name == "filesystem secondary backend with valid path" {
				var err error
				tmpDir, err = os.MkdirTemp("", "armor-secondary-test-*")
				if err != nil {
					t.Fatalf("failed to create temp dir: %v", err)
				}
				defer os.RemoveAll(tmpDir)
				tt.secondaryBackendStr = "filesystem:" + tmpDir
			}

			setTestEnv(t, append(minimalTestEnv(),
				"ARMOR_SECONDARY_BACKEND", tt.secondaryBackendStr,
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
		"ARMOR_SECONDARY_BACKEND", "invalid-type:/some/path",
	)...)

	_, err := config.Load()
	if err == nil {
		t.Error("expected error for invalid backend type, got nil")
	}
	// Config should fail to load with an unsupported backend type
}

func TestSecondaryBackendInvalidB2Config(t *testing.T) {
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
		"ARMOR_SECONDARY_BACKEND", "filesystem:"+tmpDir,
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
