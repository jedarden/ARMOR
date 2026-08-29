package config

import (
	"os"
	"strings"
	"testing"
)

// minimalFilesystemEnv returns the set of required env var pairs needed for Load()
// to succeed with ARMOR_BACKEND=filesystem.
func minimalFilesystemEnv() []string {
	return []string{
		"ARMOR_BACKEND", "filesystem",
		"ARMOR_FS_PATH", "/tmp/armor",
		"ARMOR_BUCKET", "testbucket",
		"ARMOR_MEK", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
	}
}

// TestFilesystemBackendConfigValid tests that when ARMOR_BACKEND=filesystem
// and ARMOR_FS_PATH is set, the config fields are populated correctly.
func TestFilesystemBackendConfigValid(t *testing.T) {
	setEnv(t, minimalFilesystemEnv()...)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Backend != "filesystem" {
		t.Errorf("Backend = %q, want 'filesystem'", cfg.Backend)
	}
	if cfg.FSPath != "/tmp/armor" {
		t.Errorf("FSPath = %q, want '/tmp/armor'", cfg.FSPath)
	}
	if cfg.Bucket != "testbucket" {
		t.Errorf("Bucket = %q, want 'testbucket'", cfg.Bucket)
	}
	// Verify B2 fields are not required when using filesystem backend
	if cfg.B2Region != "" {
		t.Errorf("B2Region should be empty for filesystem backend, got %q", cfg.B2Region)
	}
	if cfg.B2AccessKeyID != "" {
		t.Errorf("B2AccessKeyID should be empty for filesystem backend, got %q", cfg.B2AccessKeyID)
	}
	if cfg.B2SecretAccessKey != "" {
		t.Errorf("B2SecretAccessKey should be empty for filesystem backend, got %q", cfg.B2SecretAccessKey)
	}
}

// TestFilesystemBackendConfigMissingPath tests that when ARMOR_BACKEND=filesystem
// but ARMOR_FS_PATH is empty/missing, an error is returned.
func TestFilesystemBackendConfigMissingPath(t *testing.T) {
	tests := []struct {
		name          string
		pathValue     string
		shouldError   bool
		errorContains string
	}{
		{
			name:          "path completely unset",
			pathValue:     "",
			shouldError:   true,
			errorContains: "ARMOR_FS_PATH is required",
		},
		{
			name:          "path is empty string",
			pathValue:     "",
			shouldError:   true,
			errorContains: "ARMOR_FS_PATH is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, minimalFilesystemEnv()...)
			os.Setenv("ARMOR_BACKEND", "filesystem")
			if tt.pathValue != "" {
				os.Setenv("ARMOR_FS_PATH", tt.pathValue)
			} else {
				os.Unsetenv("ARMOR_FS_PATH")
			}

			_, err := Load()
			if !tt.shouldError && err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if tt.shouldError {
				if err == nil {
					t.Error("Load() expected error for missing ARMOR_FS_PATH, got nil")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got: %v", tt.errorContains, err)
				}
			}
		})
	}
}

// TestFilesystemBackendConfigVariousPaths tests various valid path formats
// to ensure they are accepted and populated correctly.
func TestFilesystemBackendConfigVariousPaths(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "absolute path",
			path:     "/tmp/armor",
			expected: "/tmp/armor",
		},
		{
			name:     "absolute path with trailing slash",
			path:     "/tmp/armor/",
			expected: "/tmp/armor/",
		},
		{
			name:     "relative path",
			path:     "./armor",
			expected: "./armor",
		},
		{
			name:     "relative path with parent directory",
			path:     "../backup/armor",
			expected: "../backup/armor",
		},
		{
			name:     "home directory expansion",
			path:     "~/armor",
			expected: "~/armor",
		},
		{
			name:     "path with spaces",
			path:     "/tmp/armor storage",
			expected: "/tmp/armor storage",
		},
		{
			name:     "complex path",
			path:     "/var/lib/armor/storage",
			expected: "/var/lib/armor/storage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, minimalFilesystemEnv()...)
			os.Setenv("ARMOR_FS_PATH", tt.path)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			if cfg.FSPath != tt.expected {
				t.Errorf("FSPath = %q, want %q", cfg.FSPath, tt.expected)
			}
		})
	}
}

// TestFilesystemBackendConfigInvalidType tests that non-'b2' or 'filesystem' values for
// ARMOR_BACKEND are rejected with a clear error.
func TestFilesystemBackendConfigInvalidType(t *testing.T) {
	invalidTypes := []string{"s3", "wasabi", "garbage", "S3", "FILESYSTEM", "FileSystem", "minio"}

	for _, invalidType := range invalidTypes {
		t.Run(invalidType, func(t *testing.T) {
			setEnv(t, minimalFilesystemEnv()...)
			os.Setenv("ARMOR_BACKEND", invalidType)
			os.Setenv("ARMOR_FS_PATH", "/tmp/armor")

			_, err := Load()
			if err == nil {
				t.Error("Load() expected error for invalid ARMOR_BACKEND, got nil")
			}
			if !strings.Contains(err.Error(), "ARMOR_BACKEND must be 'b2' or 'filesystem'") {
				t.Errorf("Error message should mention valid backend types, got: %v", err)
			}
			if !strings.Contains(err.Error(), invalidType) {
				t.Errorf("Error message should cite the invalid value, got: %v", err)
			}
		})
	}
}

// TestB2BackendConfigDefaults tests that when ARMOR_BACKEND is unset or "b2",
// B2 configuration is required and defaults apply.
func TestB2BackendConfigDefaults(t *testing.T) {
	tests := []struct {
		name          string
		backendValue  string
		shouldError   bool
		errorContains string
		checkFunc     func(*Config, error)
	}{
		{
			name:         "backend unset defaults to b2",
			backendValue: "",
			shouldError:  false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				if cfg.Backend != "b2" {
					t.Errorf("Backend should default to 'b2', got %q", cfg.Backend)
				}
			},
		},
		{
			name:         "backend explicitly set to b2",
			backendValue: "b2",
			shouldError:  false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				if cfg.Backend != "b2" {
					t.Errorf("Backend = %q, want 'b2'", cfg.Backend)
				}
			},
		},
		{
			name:          "b2 backend missing B2_REGION",
			backendValue:  "b2",
			shouldError:   true,
			errorContains: "ARMOR_B2_REGION is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use minimal B2 env for most tests
			if tt.name != "b2 backend missing B2_REGION" {
				setEnv(t, append(minimalEnv(),
					"ARMOR_BACKEND", tt.backendValue,
				)...)
			} else {
				// Test missing B2_REGION
				setEnv(t,
					"ARMOR_BACKEND", "b2",
					"ARMOR_BUCKET", "testbucket",
					"ARMOR_MEK", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
				)
			}

			cfg, err := Load()
			if tt.shouldError {
				if err == nil {
					t.Error("Load() expected error, got nil")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
			}
			if tt.checkFunc != nil {
				tt.checkFunc(cfg, err)
			}
		})
	}
}

// TestFilesystemBackendWithSecondary tests that primary filesystem backend
// can coexist with secondary backend configuration.
func TestFilesystemBackendWithSecondary(t *testing.T) {
	setEnv(t, append(minimalFilesystemEnv(),
		"ARMOR_SECONDARY_BACKEND_TYPE", "filesystem",
		"ARMOR_SECONDARY_BACKEND_PATH", "/tmp/armor/secondary",
	)...)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Backend != "filesystem" {
		t.Errorf("Backend = %q, want 'filesystem'", cfg.Backend)
	}
	if cfg.FSPath != "/tmp/armor" {
		t.Errorf("FSPath = %q, want '/tmp/armor'", cfg.FSPath)
	}
	if cfg.SecondaryBackendType != "filesystem" {
		t.Errorf("SecondaryBackendType = %q, want 'filesystem'", cfg.SecondaryBackendType)
	}
	if cfg.SecondaryBackendPath != "/tmp/armor/secondary" {
		t.Errorf("SecondaryBackendPath = %q, want '/tmp/armor/secondary'", cfg.SecondaryBackendPath)
	}
}

// TestFilesystemBackendCFDomainIgnored tests that ARMOR_CF_DOMAIN is ignored
// when using filesystem backend (not required, not used).
func TestFilesystemBackendCFDomainIgnored(t *testing.T) {
	setEnv(t, append(minimalFilesystemEnv(),
		"ARMOR_CF_DOMAIN", "cdn.example.com",
	)...)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// CFDomain should be empty when using filesystem backend
	if cfg.CFDomain != "" {
		t.Errorf("CFDomain should be empty for filesystem backend, got %q", cfg.CFDomain)
	}
}

// TestFilesystemBackendConfigEdgeCases tests edge cases like empty strings,
// whitespace-only values, etc.
func TestFilesystemBackendConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		backendValue  string
		pathValue     string
		shouldError   bool
		errorContains string
		checkFunc     func(*Config, error)
	}{
		{
			name:         "filesystem with valid path",
			backendValue: "filesystem",
			pathValue:    "/var/lib/armor",
			shouldError:  false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				if cfg.Backend != "filesystem" {
					t.Errorf("Backend = %q, want 'filesystem'", cfg.Backend)
				}
				if cfg.FSPath != "/var/lib/armor" {
					t.Errorf("FSPath = %q, want '/var/lib/armor'", cfg.FSPath)
				}
			},
		},
		{
			name:         "whitespace-only backend rejected",
			backendValue: "   ",
			pathValue:    "/tmp/armor",
			shouldError:  true,
			errorContains: "ARMOR_BACKEND must be 'b2' or 'filesystem'",
		},
		{
			name:         "whitespace-only path is accepted (not trimmed)",
			backendValue: "filesystem",
			pathValue:    "   ",
			shouldError:  false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				// Whitespace is not trimmed by the config loader
				if cfg.FSPath != "   " {
					t.Errorf("FSPath should preserve whitespace, got %q", cfg.FSPath)
				}
			},
		},
		{
			name:         "path with leading/trailing whitespace is preserved",
			backendValue: "filesystem",
			pathValue:    "  /tmp/armor  ",
			shouldError:  false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				// Path should be preserved as-is (including whitespace)
				if cfg.FSPath != "  /tmp/armor  " {
					t.Errorf("FSPath should preserve whitespace, got %q", cfg.FSPath)
				}
			},
		},
		{
			name:         "lowercase filesystem accepted",
			backendValue: "filesystem",
			pathValue:    "/mnt/backup",
			shouldError:  false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				if cfg.Backend != "filesystem" {
					t.Errorf("Backend = %q, want 'filesystem'", cfg.Backend)
				}
			},
		},
		{
			name:         "mixed case FileSystem rejected (case-sensitive)",
			backendValue: "FileSystem",
			pathValue:    "/mnt/backup",
			shouldError:  true,
			errorContains: "ARMOR_BACKEND must be 'b2' or 'filesystem'",
		},
		{
			name:         "uppercase FILESYSTEM rejected (case-sensitive)",
			backendValue: "FILESYSTEM",
			pathValue:    "/mnt/backup",
			shouldError:  true,
			errorContains: "ARMOR_BACKEND must be 'b2' or 'filesystem'",
		},
		{
			name:         "lowercase b2 accepted",
			backendValue: "b2",
			shouldError:  false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				if cfg.Backend != "b2" {
					t.Errorf("Backend = %q, want 'b2'", cfg.Backend)
				}
			},
		},
		{
			name:         "uppercase B2 rejected (case-sensitive)",
			backendValue: "B2",
			shouldError:  true,
			errorContains: "ARMOR_BACKEND must be 'b2' or 'filesystem'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup base environment
			if tt.backendValue == "b2" || tt.backendValue == "" {
				setEnv(t, minimalEnv()...)
			} else {
				setEnv(t, minimalFilesystemEnv()...)
			}

			if tt.backendValue != "" {
				os.Setenv("ARMOR_BACKEND", tt.backendValue)
			} else {
				os.Unsetenv("ARMOR_BACKEND")
			}

			if tt.pathValue != "" {
				os.Setenv("ARMOR_FS_PATH", tt.pathValue)
			} else {
				os.Unsetenv("ARMOR_FS_PATH")
			}

			cfg, err := Load()
			if tt.shouldError {
				if err == nil {
					t.Error("Load() expected error, got nil")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
			}
			if tt.checkFunc != nil {
				tt.checkFunc(cfg, err)
			}
		})
	}
}
