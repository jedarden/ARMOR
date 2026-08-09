// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitFilesystemBackendFromConfig(t *testing.T) {
	// A pre-created directory shared by the success cases.
	validDir := t.TempDir()

	// A regular file used to verify the not-a-directory rejection.
	regularFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(regularFile, []byte("x"), 0644); err != nil {
		t.Fatalf("setup: create regular file: %v", err)
	}

	tests := []struct {
		name        string
		cfg         BackendConfig
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty path",
			cfg:         BackendConfig{Type: "filesystem", Path: ""},
			wantErr:     true,
			errContains: "path is required",
		},
		{
			name:        "missing path field entirely",
			cfg:         BackendConfig{Type: "filesystem"},
			wantErr:     true,
			errContains: "path is required",
		},
		{
			name:        "non-existent path",
			cfg:         BackendConfig{Type: "filesystem", Path: "/nonexistent-armor-path-12345"},
			wantErr:     true,
			errContains: "does not exist",
		},
		{
			name:        "nested non-existent path",
			cfg:         BackendConfig{Type: "filesystem", Path: filepath.Join(validDir, "does-not-exist")},
			wantErr:     true,
			errContains: "does not exist",
		},
		{
			name:        "path is a regular file not a directory",
			cfg:         BackendConfig{Type: "filesystem", Path: regularFile},
			wantErr:     true,
			errContains: "not a directory",
		},
		{
			name:        "wrong backend type",
			cfg:         BackendConfig{Type: "b2", Path: validDir},
			wantErr:     true,
			errContains: "requires type",
		},
		{
			name:    "valid existing directory",
			cfg:     BackendConfig{Type: "filesystem", Path: validDir},
			wantErr: false,
		},
		{
			name:    "valid directory, empty type allowed",
			cfg:     BackendConfig{Path: validDir},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := InitFilesystemBackend(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				if b != nil {
					t.Errorf("expected nil backend on error, got %T", b)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if b == nil {
				t.Fatal("expected non-nil backend, got nil")
			}

			// Must be a concrete *FSBackend with the configured base path.
			fs, ok := b.(*FSBackend)
			if !ok {
				t.Fatalf("expected *FSBackend, got %T", b)
			}
			if fs.basePath != tt.cfg.Path {
				t.Errorf("basePath = %q, want %q", fs.basePath, tt.cfg.Path)
			}
		})
	}
}

// TestInitFilesystemBackend_Inaccessible exercises the inaccessible-path
// branch. A NUL byte in the path makes os.Stat fail with EINVAL (not
// IsNotExist), which is deterministic regardless of whether the test runs as
// root — unlike a chmod-based permission test, which root bypasses.
func TestInitFilesystemBackend_Inaccessible(t *testing.T) {
	cfg := BackendConfig{Type: "filesystem", Path: "bad\x00path"}
	b, err := InitFilesystemBackend(cfg)
	if err == nil {
		t.Fatalf("expected error for inaccessible path, got backend %T", b)
	}
	if !strings.Contains(err.Error(), "inaccessible") {
		t.Errorf("expected error containing %q, got %q", "inaccessible", err.Error())
	}
}

// TestInitFilesystemBackend_PermissionDenied covers the EACCES branch when not
// running as root (root bypasses filesystem permissions, so the test is
// skipped in that case).
func TestInitFilesystemBackend_PermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission-based test is not meaningful")
	}

	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatalf("setup: mkdir child: %v", err)
	}
	// Strip all permissions from the parent so the child cannot be statted.
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatalf("setup: chmod parent: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0755) }) // restore so TempDir cleanup works

	b, err := InitFilesystemBackend(BackendConfig{Type: "filesystem", Path: child})
	if err == nil {
		t.Fatalf("expected permission error, got backend %T", b)
	}
	// Either the "does not exist" or "inaccessible" message is acceptable
	// here; both originate from the os.Stat failure path.
	if !strings.Contains(err.Error(), "inaccessible") && !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected inaccessible/does-not-exist error, got %q", err.Error())
	}
}

// TestInitFilesystemBackend_ReturnsBackendInterface confirms the returned
// value satisfies the Backend interface and is usable for a round-trip.
func TestInitFilesystemBackend_ReturnsBackendInterface(t *testing.T) {
	dir := t.TempDir()
	b, err := InitFilesystemBackend(BackendConfig{Type: "filesystem", Path: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Smoke test: the backend is wired and can create/list buckets.
	ctx := context.Background()
	if err := b.CreateBucket(ctx, "smoke"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	buckets, err := b.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	found := false
	for _, bk := range buckets {
		if bk.Name == "smoke" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find 'smoke' bucket in ListBuckets result")
	}
}
