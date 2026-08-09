//go:build secondary_string_parser

// secondary_test.go exercises the superseded colon-string parser API in
// secondary.go (InitSecondaryBackend / initFilesystemBackend / initB2Backend).
// It is gated behind the secondary_string_parser build tag — along with the
// implementation it tests — so it does not run in the default `go test
// ./internal/backend/` run. See the header comment in secondary.go for why the
// pair is excluded and the pending ownership decision. The committed-API tests
// (secondary_init_test.go, secondary_init_b2_test.go) remain the default-run
// coverage for the BackendConfig struct initializers.

// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"context"
	"testing"
)

func TestInitSecondaryBackend(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		configStr     string
		expectError   bool
		errorContains string
		checkFunc     func(Backend) bool
	}{
		{
			name:          "empty config string",
			configStr:     "",
			expectError:   true,
			errorContains: "cannot be empty",
		},
		{
			name:          "invalid format - no colon",
			configStr:     "filesystem",
			expectError:   true,
			errorContains: "invalid format",
		},
		{
			name:          "unsupported backend type",
			configStr:     "s3:/path",
			expectError:   true,
			errorContains: "unsupported backend type",
		},
		{
			name:        "filesystem backend - valid path",
			configStr:   "filesystem:/tmp/test-backend",
			expectError: false,
			checkFunc: func(b Backend) bool {
				_, ok := b.(*FSBackend)
				return ok
			},
		},
		{
			name:          "filesystem backend - empty path",
			configStr:     "filesystem:",
			expectError:   true,
			errorContains: "path cannot be empty",
		},
		{
			name:        "filesystem backend - nested path",
			configStr:   "filesystem:/tmp/armor/backup",
			expectError: false,
			checkFunc: func(b Backend) bool {
				_, ok := b.(*FSBackend)
				return ok
			},
		},
		{
			name:        "B2 backend - valid config with all fields",
			configStr:   "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEY004:SECRET004:testbucket",
			expectError: false,
			checkFunc: func(b Backend) bool {
				_, ok := b.(*B2Backend)
				return ok
			},
		},
		{
			name:          "B2 backend - empty params",
			configStr:     "b2:",
			expectError:   true,
			errorContains: "cannot be empty",
		},
		{
			name:          "B2 backend - insufficient params (4 instead of 5)",
			configStr:     "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEY004:SECRET004",
			expectError:   true,
			errorContains: "expected 5 colon-separated values",
		},
		{
			name:          "B2 backend - too many params (6 instead of 5)",
			configStr:     "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEY004:SECRET004:bucket:extra",
			expectError:   true,
			errorContains: "expected 5 colon-separated values",
		},
		{
			name:          "B2 backend - empty region",
			configStr:     "b2::https://s3.us-east-005.backblazeb2.com:KEY004:SECRET004:testbucket",
			expectError:   true,
			errorContains: "region cannot be empty",
		},
		{
			name:          "B2 backend - empty endpoint",
			configStr:     "b2:us-east-005::KEY004:SECRET004:testbucket",
			expectError:   true,
			errorContains: "endpoint cannot be empty",
		},
		{
			name:          "B2 backend - empty access key ID",
			configStr:     "b2:us-east-005:https://s3.us-east-005.backblazeb2.com::SECRET004:testbucket",
			expectError:   true,
			errorContains: "access key ID cannot be empty",
		},
		{
			name:          "B2 backend - empty secret key",
			configStr:     "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEY004::testbucket",
			expectError:   true,
			errorContains: "secret key cannot be empty",
		},
		{
			name:          "B2 backend - empty bucket",
			configStr:     "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEY004:SECRET004:",
			expectError:   true,
			errorContains: "bucket cannot be empty",
		},
		{
			name:        "B2 backend - with colons in values (endpoint URL)",
			configStr:   "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:key123:secret456:mybucket",
			expectError: false,
			checkFunc: func(b Backend) bool {
				_, ok := b.(*B2Backend)
				return ok
			},
		},
		{
			name:        "filesystem - creates directory if needed",
			configStr:   "filesystem:/tmp/armor-secondary-test-12345",
			expectError: false,
			checkFunc: func(b Backend) bool {
				_, ok := b.(*FSBackend)
				return ok
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := InitSecondaryBackend(ctx, tt.configStr)

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

			if backend == nil {
				t.Error("expected non-nil backend")
				return
			}

			if tt.checkFunc != nil && !tt.checkFunc(backend) {
				t.Error("backend check function failed")
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestInitFilesystemBackend tests the filesystem-specific initialization
func TestInitFilesystemBackend(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid path",
			path:        "/tmp/armor-test",
			expectError: false,
		},
		{
			name:          "empty path",
			path:          "",
			expectError:   true,
			errorContains: "cannot be empty",
		},
		{
			name:        "nested path",
			path:        "/var/lib/armor/backup",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := initFilesystemBackend(tt.path)

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

			if backend == nil {
				t.Error("expected non-nil backend")
				return
			}

			_, ok := backend.(*FSBackend)
			if !ok {
				t.Error("expected FSBackend")
			}
		})
	}
}

// TestInitB2Backend tests the B2-specific initialization
func TestInitB2Backend(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		params        string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid B2 config",
			params:      "us-east-005:https://s3.us-east-005.backblazeb2.com:KEY123:SECRET456:testbucket",
			expectError: false,
		},
		{
			name:          "empty params",
			params:        "",
			expectError:   true,
			errorContains: "cannot be empty",
		},
		{
			name:          "wrong number of params - 4",
			params:        "us-east-005:https://s3.us-east-005.backblazeb2.com:KEY123:SECRET456",
			expectError:   true,
			errorContains: "expected 5 colon-separated values",
		},
		{
			name:          "wrong number of params - 6",
			params:        "us-east-005:https://s3.us-east-005.backblazeb2.com:KEY123:SECRET456:bucket:extra",
			expectError:   true,
			errorContains: "expected 5 colon-separated values",
		},
		{
			name:          "empty region",
			params:        ":https://s3.us-east-005.backblazeb2.com:KEY123:SECRET456:testbucket",
			expectError:   true,
			errorContains: "region cannot be empty",
		},
		{
			name:          "empty endpoint",
			params:        "us-east-005::KEY123:SECRET456:testbucket",
			expectError:   true,
			errorContains: "endpoint cannot be empty",
		},
		{
			name:          "empty access key ID",
			params:        "us-east-005:https://s3.us-east-005.backblazeb2.com::SECRET456:testbucket",
			expectError:   true,
			errorContains: "access key ID cannot be empty",
		},
		{
			name:          "empty secret key",
			params:        "us-east-005:https://s3.us-east-005.backblazeb2.com:KEY123::testbucket",
			expectError:   true,
			errorContains: "secret key cannot be empty",
		},
		{
			name:          "empty bucket",
			params:        "us-east-005:https://s3.us-east-005.backblazeb2.com:KEY123:SECRET456:",
			expectError:   true,
			errorContains: "bucket cannot be empty",
		},
		{
			name:          "all empty fields",
			params:        "::::",
			expectError:   true,
			errorContains: "region cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := initB2Backend(ctx, tt.params)

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

			if backend == nil {
				t.Error("expected non-nil backend")
				return
			}

			_, ok := backend.(*B2Backend)
			if !ok {
				t.Error("expected B2Backend")
			}
		})
	}
}
