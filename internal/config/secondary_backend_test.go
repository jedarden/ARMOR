package config

import (
	"os"
	"strings"
	"testing"
)

// TestSecondaryBackendConfigValid tests that when ARMOR_SECONDARY_BACKEND_TYPE=filesystem
// and ARMOR_SECONDARY_BACKEND_PATH is set, the config fields are populated correctly.
func TestSecondaryBackendConfigValid(t *testing.T) {
	setEnv(t, append(minimalEnv(),
		"ARMOR_SECONDARY_BACKEND_TYPE", "filesystem",
		"ARMOR_SECONDARY_BACKEND_PATH", "/tmp/armor/secondary",
	)...)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.SecondaryBackendType != "filesystem" {
		t.Errorf("SecondaryBackendType = %q, want 'filesystem'", cfg.SecondaryBackendType)
	}
	if cfg.SecondaryBackendPath != "/tmp/armor/secondary" {
		t.Errorf("SecondaryBackendPath = %q, want '/tmp/armor/secondary'", cfg.SecondaryBackendPath)
	}
}

// TestSecondaryBackendConfigDisabled tests that when ARMOR_SECONDARY_BACKEND_TYPE is unset,
// the secondary backend remains disabled with no error.
func TestSecondaryBackendConfigDisabled(t *testing.T) {
	setEnv(t, minimalEnv()...)
	// Explicitly unset secondary backend env vars
	os.Unsetenv("ARMOR_SECONDARY_BACKEND_TYPE")
	os.Unsetenv("ARMOR_SECONDARY_BACKEND_PATH")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.SecondaryBackendType != "" {
		t.Errorf("SecondaryBackendType = %q, want empty string (disabled)", cfg.SecondaryBackendType)
	}
	if cfg.SecondaryBackendPath != "" {
		t.Errorf("SecondaryBackendPath = %q, want empty string (disabled)", cfg.SecondaryBackendPath)
	}
}

// TestSecondaryBackendConfigInvalidType tests that non-'filesystem' values for
// ARMOR_SECONDARY_BACKEND_TYPE are rejected with a clear error.
func TestSecondaryBackendConfigInvalidType(t *testing.T) {
	invalidTypes := []string{"s3", "wasabi", "garbage", "S3", "FILESYSTEM", "FileSystem", "minio"}

	for _, invalidType := range invalidTypes {
		t.Run(invalidType, func(t *testing.T) {
			setEnv(t, append(minimalEnv(),
				"ARMOR_SECONDARY_BACKEND_TYPE", invalidType,
				"ARMOR_SECONDARY_BACKEND_PATH", "/tmp/armor/secondary",
			)...)

			_, err := Load()
			if err == nil {
				t.Error("Load() expected error for invalid ARMOR_SECONDARY_BACKEND_TYPE, got nil")
			}
			if !strings.Contains(err.Error(), "ARMOR_SECONDARY_BACKEND_TYPE must be 'filesystem'") {
				t.Errorf("Error message should mention 'filesystem' requirement, got: %v", err)
			}
			if !strings.Contains(err.Error(), invalidType) {
				t.Errorf("Error message should cite the invalid value, got: %v", err)
			}
		})
	}
}

// TestSecondaryBackendConfigMissingPath tests that when Type=filesystem but
// ARMOR_SECONDARY_BACKEND_PATH is empty/missing, an error is returned.
func TestSecondaryBackendConfigMissingPath(t *testing.T) {
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
			errorContains: "ARMOR_SECONDARY_BACKEND_PATH is required",
		},
		{
			name:          "path is empty string",
			pathValue:     "",
			shouldError:   true,
			errorContains: "ARMOR_SECONDARY_BACKEND_PATH is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, minimalEnv()...)
			os.Setenv("ARMOR_SECONDARY_BACKEND_TYPE", "filesystem")
			if tt.pathValue != "" {
				os.Setenv("ARMOR_SECONDARY_BACKEND_PATH", tt.pathValue)
			} else {
				os.Unsetenv("ARMOR_SECONDARY_BACKEND_PATH")
			}

			_, err := Load()
			if !tt.shouldError && err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if tt.shouldError {
				if err == nil {
					t.Error("Load() expected error for missing ARMOR_SECONDARY_BACKEND_PATH, got nil")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got: %v", tt.errorContains, err)
				}
			}
		})
	}
}

// TestSecondaryBackendConfigEdgeCases tests edge cases like empty strings,
// whitespace-only values, Type set but Path unset.
func TestSecondaryBackendConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		typeValue     string
		pathValue     string
		shouldError   bool
		errorContains string
		checkFunc     func(*Config, error)
	}{
		{
			name:        "empty type (no secondary backend)",
			typeValue:   "",
			pathValue:   "/tmp/armor/secondary",
			shouldError: false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				if cfg.SecondaryBackendType != "" {
					t.Errorf("SecondaryBackendType should be empty, got %q", cfg.SecondaryBackendType)
				}
				if cfg.SecondaryBackendPath != "" {
					t.Errorf("SecondaryBackendPath should be empty when Type is empty, got %q", cfg.SecondaryBackendPath)
				}
			},
		},
		{
			name:        "type set but path unset",
			typeValue:   "filesystem",
			pathValue:   "",
			shouldError: true,
			errorContains: "ARMOR_SECONDARY_BACKEND_PATH is required",
		},
		{
			name:        "whitespace-only type treated as empty (disabled)",
			typeValue:   "   ",
			pathValue:   "/tmp/armor/secondary",
			shouldError: true,
			errorContains: "must be 'filesystem'",
		},
		{
			name:        "whitespace-only path is accepted (not trimmed)",
			typeValue:   "filesystem",
			pathValue:   "   ",
			shouldError: false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				// Whitespace is not trimmed by the config loader
				if cfg.SecondaryBackendPath != "   " {
					t.Errorf("SecondaryBackendPath should preserve whitespace, got %q", cfg.SecondaryBackendPath)
				}
			},
		},
		{
			name:        "path with leading/trailing whitespace is preserved",
			typeValue:   "filesystem",
			pathValue:   "  /tmp/armor/secondary  ",
			shouldError: false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				// Path should be preserved as-is (including whitespace)
				if cfg.SecondaryBackendPath != "  /tmp/armor/secondary  " {
					t.Errorf("SecondaryBackendPath should preserve whitespace, got %q", cfg.SecondaryBackendPath)
				}
			},
		},
		{
			name:        "type filesystem with valid path",
			typeValue:   "filesystem",
			pathValue:   "/var/lib/armor/replica",
			shouldError: false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				if cfg.SecondaryBackendType != "filesystem" {
					t.Errorf("SecondaryBackendType = %q, want 'filesystem'", cfg.SecondaryBackendType)
				}
				if cfg.SecondaryBackendPath != "/var/lib/armor/replica" {
					t.Errorf("SecondaryBackendPath = %q, want '/var/lib/armor/replica'", cfg.SecondaryBackendPath)
				}
			},
		},
		{
			name:        "type lowercase filesystem",
			typeValue:   "filesystem",
			pathValue:   "/mnt/backup",
			shouldError: false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				if cfg.SecondaryBackendType != "filesystem" {
					t.Errorf("SecondaryBackendType = %q, want 'filesystem'", cfg.SecondaryBackendType)
				}
			},
		},
		{
			name:        "mixed case filesystem is rejected (case-sensitive)",
			typeValue:   "FileSystem",
			pathValue:   "/mnt/backup",
			shouldError: true,
			errorContains: "must be 'filesystem'",
		},
		{
			name:        "uppercase FILESYSTEM is rejected (case-sensitive)",
			typeValue:   "FILESYSTEM",
			pathValue:   "/mnt/backup",
			shouldError: true,
			errorContains: "must be 'filesystem'",
		},
		{
			name:        "relative path is accepted",
			typeValue:   "filesystem",
			pathValue:   "./secondary",
			shouldError: false,
			checkFunc: func(cfg *Config, err error) {
				if err != nil {
					t.Errorf("Load() unexpected error: %v", err)
				}
				if cfg.SecondaryBackendPath != "./secondary" {
					t.Errorf("SecondaryBackendPath = %q, want './secondary'", cfg.SecondaryBackendPath)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, minimalEnv()...)
			if tt.typeValue != "" {
				os.Setenv("ARMOR_SECONDARY_BACKEND_TYPE", tt.typeValue)
			} else {
				os.Unsetenv("ARMOR_SECONDARY_BACKEND_TYPE")
			}
			if tt.pathValue != "" {
				os.Setenv("ARMOR_SECONDARY_BACKEND_PATH", tt.pathValue)
			} else {
				os.Unsetenv("ARMOR_SECONDARY_BACKEND_PATH")
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

// TestSecondaryBackendConfigVariousPaths tests various valid path formats
// to ensure they are accepted and populated correctly.
func TestSecondaryBackendConfigVariousPaths(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "absolute path",
			path:     "/tmp/armor/secondary",
			expected: "/tmp/armor/secondary",
		},
		{
			name:     "absolute path with trailing slash",
			path:     "/tmp/armor/secondary/",
			expected: "/tmp/armor/secondary/",
		},
		{
			name:     "relative path",
			path:     "./secondary",
			expected: "./secondary",
		},
		{
			name:     "relative path with parent directory",
			path:     "../backup/armor",
			expected: "../backup/armor",
		},
		{
			name:     "home directory expansion",
			path:     "~/armor/secondary",
			expected: "~/armor/secondary",
		},
		{
			name:     "path with spaces",
			path:     "/tmp/armor backup/secondary",
			expected: "/tmp/armor backup/secondary",
		},
		{
			name:     "complex path",
			path:     "/var/lib/armor/replica/backups",
			expected: "/var/lib/armor/replica/backups",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, append(minimalEnv(),
				"ARMOR_SECONDARY_BACKEND_TYPE", "filesystem",
				"ARMOR_SECONDARY_BACKEND_PATH", tt.path,
			)...)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			if cfg.SecondaryBackendPath != tt.expected {
				t.Errorf("SecondaryBackendPath = %q, want %q", cfg.SecondaryBackendPath, tt.expected)
			}
		})
	}
}
