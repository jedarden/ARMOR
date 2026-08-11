// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"os"
	"testing"
)

func TestParseSecondaryBackendEnv(t *testing.T) {
	// Save original env value
	orig := os.Getenv("ARMOR_SECONDARY_BACKEND")
	defer func() {
		if orig == "" {
			os.Unsetenv("ARMOR_SECONDARY_BACKEND")
		} else {
			os.Setenv("ARMOR_SECONDARY_BACKEND", orig)
		}
	}()

	tests := []struct {
		name          string
		envValue      string
		expectError   bool
		errorContains string
		validate      func(BackendConfig) bool
	}{
		{
			name:        "unset env var returns zero config",
			envValue:    "",
			expectError: false,
			validate: func(cfg BackendConfig) bool {
				return cfg.Type == "" && cfg.Path == "" && cfg.Bucket == ""
			},
		},
		{
			name:        "empty env var returns zero config",
			envValue:    "",
			expectError: false,
			validate: func(cfg BackendConfig) bool {
				return cfg.Type == "" && cfg.Path == "" && cfg.Bucket == ""
			},
		},
		{
			name:          "invalid format - no colon",
			envValue:      "filesystem",
			expectError:   true,
			errorContains: "expected 'type:params'",
		},
		{
			name:          "invalid format - empty params",
			envValue:      "filesystem:",
			expectError:   true,
			errorContains: "cannot be empty",
		},
		{
			name:          "unsupported backend type",
			envValue:      "s3:/path",
			expectError:   true,
			errorContains: "unsupported secondary backend type",
		},
		{
			name:        "filesystem backend - valid path",
			envValue:    "filesystem:/tmp/armor-secondary",
			expectError: false,
			validate: func(cfg BackendConfig) bool {
				return cfg.Type == "filesystem" && cfg.Path == "/tmp/armor-secondary"
			},
		},
		{
			name:        "filesystem backend - nested path",
			envValue:    "filesystem:/backup/armor/replica",
			expectError: false,
			validate: func(cfg BackendConfig) bool {
				return cfg.Type == "filesystem" && cfg.Path == "/backup/armor/replica"
			},
		},
		{
			name:          "filesystem backend - empty path after colon",
			envValue:      "filesystem:",
			expectError:   true,
			errorContains: "cannot be empty",
		},
		{
			name:        "B2 backend - valid config with all fields",
			envValue:    "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEY004:SECRET004:testbucket",
			expectError: false,
			validate: func(cfg BackendConfig) bool {
				return cfg.Type == "b2" &&
					cfg.Region == "us-east-005" &&
					cfg.Endpoint == "https://s3.us-east-005.backblazeb2.com" &&
					cfg.AccessKeyID == "KEY004" &&
					cfg.SecretKey == "SECRET004" &&
					cfg.Bucket == "testbucket"
			},
		},
		{
			name:          "B2 backend - empty params",
			envValue:      "b2:",
			expectError:   true,
			errorContains: "cannot be empty",
		},
		{
			name:          "B2 backend - insufficient params (only 4 parts total)",
			envValue:      "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEY004",
			expectError:   true,
			errorContains: "expected at least 5",
		},
		{
			name:        "B2 backend - valid config with :// in endpoint (7 parts total)",
			envValue:    "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEY123:SECRET456:mybucket",
			expectError: false,
			validate: func(cfg BackendConfig) bool {
				return cfg.Type == "b2" &&
					cfg.Region == "us-east-005" &&
					cfg.Endpoint == "https://s3.us-east-005.backblazeb2.com" &&
					cfg.AccessKeyID == "KEY123" &&
					cfg.SecretKey == "SECRET456" &&
					cfg.Bucket == "mybucket"
			},
		},
		{
			name:          "B2 backend - empty region",
			envValue:      "b2::https://s3.us-east-005.backblazeb2.com:KEY123:SECRET456:bucket",
			expectError:   true,
			errorContains: "region cannot be empty",
		},
		{
			name:          "B2 backend - empty endpoint",
			envValue:      "b2:us-east-005::KEY123:SECRET456:bucket",
			expectError:   true,
			errorContains: "endpoint cannot be empty",
		},
		{
			name:          "B2 backend - empty access key ID",
			envValue:      "b2:us-east-005:https://s3.us-east-005.backblazeb2.com::SECRET456:bucket",
			expectError:   true,
			errorContains: "access key ID cannot be empty",
		},
		{
			name:          "B2 backend - empty secret key",
			envValue:      "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEY123::bucket",
			expectError:   true,
			errorContains: "secret key cannot be empty",
		},
		{
			name:          "B2 backend - empty bucket",
			envValue:      "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEY123:SECRET456:",
			expectError:   true,
			errorContains: "bucket cannot be empty",
		},
		{
			name:        "B2 backend - with colons in endpoint URL",
			envValue:    "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:key123:secret456:buckets",
			expectError: false,
			validate: func(cfg BackendConfig) bool {
				return cfg.Type == "b2" &&
					cfg.Region == "us-east-005" &&
					cfg.Endpoint == "https://s3.us-east-005.backblazeb2.com" &&
					cfg.AccessKeyID == "key123" &&
					cfg.SecretKey == "secret456" &&
					cfg.Bucket == "buckets"
			},
		},
		{
			name:        "B2 backend - endpoint without scheme",
			envValue:    "b2:us-east-005:s3.us-east-005.backblazeb2.com:KEYID:SECRET:bucket",
			expectError: false,
			validate: func(cfg BackendConfig) bool {
				return cfg.Type == "b2" &&
					cfg.Region == "us-east-005" &&
					cfg.Endpoint == "s3.us-east-005.backblazeb2.com" &&
					cfg.AccessKeyID == "KEYID" &&
					cfg.SecretKey == "SECRET" &&
					cfg.Bucket == "bucket"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env var
			if tt.envValue == "" {
				os.Unsetenv("ARMOR_SECONDARY_BACKEND")
			} else {
				os.Setenv("ARMOR_SECONDARY_BACKEND", tt.envValue)
			}

			// Parse config
			cfg, err := ParseSecondaryBackendEnv()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorContains)
					return
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.validate != nil && !tt.validate(cfg) {
				t.Errorf("config validation failed for %v", cfg)
			}
		})
	}
}
