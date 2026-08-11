// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"os"
	"testing"
)

// TestParseSecondaryBackendConfigValid tests that when all required environment
// variables are set correctly, a valid BackendConfig is returned.
func TestParseSecondaryBackendConfigValid(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		keyID    string
		key      string
		bucket   string
		wantType    string
		wantRegion string
		wantEndpoint string
		wantKeyID  string
		wantBucket string
	}{
		{
			name:     "standard us-east-005 config",
			endpoint: "https://s3.us-east-005.backblazeb2.com",
			keyID:    "keyId123",
			key:      "secretKey456",
			bucket:   "test-bucket",
			wantType:    "b2",
			wantRegion:  "us-east-005",
			wantEndpoint: "https://s3.us-east-005.backblazeb2.com",
			wantKeyID:   "keyId123",
			wantBucket:  "test-bucket",
		},
		{
			name:     "standard us-west-002 config",
			endpoint: "https://s3.us-west-002.backblazeb2.com",
			keyID:    "keyId789",
			key:      "secretKey012",
			bucket:   "my-bucket",
			wantType:    "b2",
			wantRegion:  "us-west-002",
			wantEndpoint: "https://s3.us-west-002.backblazeb2.com",
			wantKeyID:   "keyId789",
			wantBucket:  "my-bucket",
		},
		{
			name:     "http scheme",
			endpoint: "http://s3.eu-central-003.backblazeb2.com",
			keyID:    "keyIdABC",
			key:      "secretKeyDEF",
			bucket:   "http-bucket",
			wantType:    "b2",
			wantRegion:  "eu-central-003",
			wantEndpoint: "http://s3.eu-central-003.backblazeb2.com",
			wantKeyID:   "keyIdABC",
			wantBucket:  "http-bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			setEnv(t, "B2_ENDPOINT", tt.endpoint)
			setEnv(t, "B2_KEY_ID", tt.keyID)
			setEnv(t, "B2_KEY", tt.key)
			setEnv(t, "B2_BUCKET", tt.bucket)

			cfg, err := ParseSecondaryBackendConfig()

			if err != nil {
				t.Fatalf("ParseSecondaryBackendConfig() error: %v", err)
			}

			if cfg.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", cfg.Type, tt.wantType)
			}
			if cfg.Region != tt.wantRegion {
				t.Errorf("Region = %q, want %q", cfg.Region, tt.wantRegion)
			}
			if cfg.Endpoint != tt.wantEndpoint {
				t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, tt.wantEndpoint)
			}
			if cfg.AccessKeyID != tt.wantKeyID {
				t.Errorf("AccessKeyID = %q, want %q", cfg.AccessKeyID, tt.wantKeyID)
			}
			if cfg.SecretKey != tt.key {
				t.Errorf("SecretKey mismatch (value should be %q)", tt.key)
			}
			if cfg.Bucket != tt.wantBucket {
				t.Errorf("Bucket = %q, want %q", cfg.Bucket, tt.wantBucket)
			}
		})
	}
}

// TestParseSecondaryBackendConfigDisabled tests that when all environment
// variables are unset, a zero BackendConfig is returned (no error).
func TestParseSecondaryBackendConfigDisabled(t *testing.T) {
	// Ensure all env vars are unset
	unsetEnv(t, "B2_ENDPOINT", "B2_KEY_ID", "B2_KEY", "B2_BUCKET")

	cfg, err := ParseSecondaryBackendConfig()

	if err != nil {
		t.Fatalf("ParseSecondaryBackendConfig() unexpected error: %v", err)
	}

	// Verify zero config
	if cfg.Type != "" {
		t.Errorf("Type should be empty (disabled), got %q", cfg.Type)
	}
	if cfg.Region != "" {
		t.Errorf("Region should be empty (disabled), got %q", cfg.Region)
	}
	if cfg.Endpoint != "" {
		t.Errorf("Endpoint should be empty (disabled), got %q", cfg.Endpoint)
	}
	if cfg.AccessKeyID != "" {
		t.Errorf("AccessKeyID should be empty (disabled), got %q", cfg.AccessKeyID)
	}
	if cfg.SecretKey != "" {
		t.Errorf("SecretKey should be empty (disabled), got %q", cfg.SecretKey)
	}
	if cfg.Bucket != "" {
		t.Errorf("Bucket should be empty (disabled), got %q", cfg.Bucket)
	}
}

// TestParseSecondaryBackendConfigPartialConfig tests that partial configuration
// (only some env vars set) returns an appropriate error.
func TestParseSecondaryBackendConfigPartialConfig(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      string
		keyID         string
		key           string
		bucket        string
		errorContains string
	}{
		{
			name:          "only endpoint set",
			endpoint:      "https://s3.us-east-005.backblazeb2.com",
			errorContains: "B2_KEY_ID is required",
		},
		{
			name:          "endpoint and keyID set",
			endpoint:      "https://s3.us-east-005.backblazeb2.com",
			keyID:         "keyId123",
			errorContains: "B2_KEY is required",
		},
		{
			name:          "endpoint, keyID, key set",
			endpoint:      "https://s3.us-east-005.backblazeb2.com",
			keyID:         "keyId123",
			key:           "secretKey456",
			errorContains: "B2_BUCKET is required",
		},
		{
			name:          "only keyID set",
			keyID:         "keyId123",
			errorContains: "B2_ENDPOINT is required",
		},
		{
			name:          "keyID and key set",
			keyID:         "keyId123",
			key:           "secretKey456",
			errorContains: "B2_ENDPOINT is required",
		},
		{
			name:          "keyID, key, bucket set",
			keyID:         "keyId123",
			key:           "secretKey456",
			bucket:        "test-bucket",
			errorContains: "B2_ENDPOINT is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars first
			unsetEnv(t, "B2_ENDPOINT", "B2_KEY_ID", "B2_KEY", "B2_BUCKET")

			// Set only the specified vars
			if tt.endpoint != "" {
				setEnv(t, "B2_ENDPOINT", tt.endpoint)
			}
			if tt.keyID != "" {
				setEnv(t, "B2_KEY_ID", tt.keyID)
			}
			if tt.key != "" {
				setEnv(t, "B2_KEY", tt.key)
			}
			if tt.bucket != "" {
				setEnv(t, "B2_BUCKET", tt.bucket)
			}

			_, err := ParseSecondaryBackendConfig()

			if err == nil {
				t.Error("ParseSecondaryBackendConfig() expected error, got nil")
			}
			if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
				t.Errorf("Error should contain %q, got: %v", tt.errorContains, err)
			}
		})
	}
}

// TestParseSecondaryBackendConfigEmptyStrings tests that empty strings for
// required fields are treated as unset and return appropriate errors.
func TestParseSecondaryBackendConfigEmptyStrings(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      string
		keyID         string
		key           string
		bucket        string
		errorContains string
	}{
		{
			name:          "empty endpoint string",
			endpoint:      "",
			keyID:         "keyId123",
			key:           "secretKey456",
			bucket:        "test-bucket",
			errorContains: "B2_ENDPOINT is required",
		},
		{
			name:          "empty keyID string",
			endpoint:      "https://s3.us-east-005.backblazeb2.com",
			keyID:         "",
			key:           "secretKey456",
			bucket:        "test-bucket",
			errorContains: "B2_KEY_ID is required",
		},
		{
			name:          "empty key string",
			endpoint:      "https://s3.us-east-005.backblazeb2.com",
			keyID:         "keyId123",
			key:           "",
			bucket:        "test-bucket",
			errorContains: "B2_KEY is required",
		},
		{
			name:          "empty bucket string",
			endpoint:      "https://s3.us-east-005.backblazeb2.com",
			keyID:         "keyId123",
			key:           "secretKey456",
			bucket:        "",
			errorContains: "B2_BUCKET is required",
		},
		{
			name:          "all empty strings",
			endpoint:      "",
			keyID:         "",
			key:           "",
			bucket:        "",
			errorContains: "", // Should return zero config, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars first
			unsetEnv(t, "B2_ENDPOINT", "B2_KEY_ID", "B2_KEY", "B2_BUCKET")

			// Set the vars (including empty strings)
			setEnv(t, "B2_ENDPOINT", tt.endpoint)
			setEnv(t, "B2_KEY_ID", tt.keyID)
			setEnv(t, "B2_KEY", tt.key)
			setEnv(t, "B2_BUCKET", tt.bucket)

			cfg, err := ParseSecondaryBackendConfig()

			// Special case: if all are empty, should return zero config
			if tt.endpoint == "" && tt.keyID == "" && tt.key == "" && tt.bucket == "" {
				if err != nil {
					t.Errorf("ParseSecondaryBackendConfig() should return zero config, got error: %v", err)
				}
				if cfg.Type != "" || cfg.Endpoint != "" || cfg.AccessKeyID != "" {
					t.Errorf("ParseSecondaryBackendConfig() should return zero config, got: %+v", cfg)
				}
				return
			}

			if err == nil {
				t.Error("ParseSecondaryBackendConfig() expected error, got nil")
			}
			if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
				t.Errorf("Error should contain %q, got: %v", tt.errorContains, err)
			}
		})
	}
}

// TestParseSecondaryBackendConfigMalformedEndpoint tests various malformed
// endpoint URL formats.
func TestParseSecondaryBackendConfigMalformedEndpoint(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      string
		errorContains string
	}{
		{
			name:          "no scheme",
			endpoint:      "s3.us-east-005.backblazeb2.com",
			errorContains: "must include a scheme",
		},
		{
			name:          "invalid URL characters",
			endpoint:      "https://s3 .us-east-005.backblazeb2.com",
			errorContains: "valid URL",
		},
		{
			name:          "invalid scheme ftp",
			endpoint:      "ftp://s3.us-east-005.backblazeb2.com",
			errorContains: "scheme must be http or https",
		},
		{
			name:          "invalid scheme s3",
			endpoint:      "s3://s3.us-east-005.backblazeb2.com",
			errorContains: "scheme must be http or https",
		},
		{
			name:          "empty hostname",
			endpoint:      "https://",
			errorContains: "hostname is empty",
		},
		{
			name:          "wrong hostname format - missing s3 prefix",
			endpoint:      "https://us-east-005.backblazeb2.com",
			errorContains: "match format",
		},
		{
			name:          "wrong hostname format - missing backblazeb2",
			endpoint:      "https://s3.us-east-005.example.com",
			errorContains: "must end with '.backblazeb2.com'",
		},
		{
			name:          "wrong hostname format - missing region",
			endpoint:      "https://s3.backblazeb2.com",
			errorContains: "match format",
		},
		{
			name:          "hostname with IP address as region",
			endpoint:      "https://s3.192.168.1.1.backblazeb2.com",
			errorContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, "B2_ENDPOINT", tt.endpoint)
			setEnv(t, "B2_KEY_ID", "keyId123")
			setEnv(t, "B2_KEY", "secretKey456")
			setEnv(t, "B2_BUCKET", "test-bucket")

			_, err := ParseSecondaryBackendConfig()

			// IP address as region is considered valid by current implementation
			if tt.name == "hostname with IP address as region" {
				if err != nil {
					t.Errorf("ParseSecondaryBackendConfig() with IP address as region should be valid, got error: %v", err)
				}
				return
			}

			if err == nil {
				t.Error("ParseSecondaryBackendConfig() expected error, got nil")
			}
			if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
				t.Errorf("Error should contain %q, got: %v", tt.errorContains, err)
			}
		})
	}
}

// TestParseSecondaryBackendConfigEdgeCases tests edge cases and unusual
// but valid configurations.
func TestParseSecondaryBackendConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      string
		keyID         string
		key           string
		bucket        string
		shouldError   bool
		errorContains string
		wantRegion    string
	}{
		{
			name:        "endpoint with port",
			endpoint:    "https://s3.us-east-005.backblazeb2.com:443",
			keyID:       "keyId123",
			key:         "secretKey456",
			bucket:      "test-bucket",
			shouldError: false,
			wantRegion:  "us-east-005",
		},
		{
			name:        "endpoint with custom port",
			endpoint:    "https://s3.us-east-005.backblazeb2.com:8443",
			keyID:       "keyId123",
			key:         "secretKey456",
			bucket:      "test-bucket",
			shouldError: false,
			wantRegion:  "us-east-005",
		},
		{
			name:        "endpoint with path",
			endpoint:    "https://s3.us-east-005.backblazeb2.com/path",
			keyID:       "keyId123",
			key:         "secretKey456",
			bucket:      "test-bucket",
			shouldError: false,
			wantRegion:  "us-east-005",
		},
		{
			name:        "endpoint with query params",
			endpoint:    "https://s3.us-east-005.backblazeb2.com?param=value",
			keyID:       "keyId123",
			key:         "secretKey456",
			bucket:      "test-bucket",
			shouldError: false,
			wantRegion:  "us-east-005",
		},
		{
			name:        "region with hyphens",
			endpoint:    "https://s3.eu-central-003.backblazeb2.com",
			keyID:       "keyId123",
			key:         "secretKey456",
			bucket:      "test-bucket",
			shouldError: false,
			wantRegion:  "eu-central-003",
		},
		{
			name:        "whitespace in values",
			endpoint:    "https://s3.us-east-005.backblazeb2.com",
			keyID:       " keyId123 ",
			key:         " secretKey456 ",
			bucket:      " test-bucket ",
			shouldError: false,
			wantRegion:  "us-east-005",
		},
		{
			name:          "whitespace in endpoint URL",
			endpoint:      " https://s3.us-east-005.backblazeb2.com ",
			keyID:         "keyId123",
			key:           "secretKey456",
			bucket:        "test-bucket",
			shouldError:   true,
			errorContains: "valid URL",
		},
		{
			name:        "special characters in bucket name",
			endpoint:    "https://s3.us-east-005.backblazeb2.com",
			keyID:       "keyId123",
			key:         "secretKey456",
			bucket:      "test-bucket-123",
			shouldError: false,
			wantRegion:  "us-east-005",
		},
		{
			name:        "uppercase endpoint scheme",
			endpoint:    "HTTPS://s3.us-east-005.backblazeb2.com",
			keyID:       "keyId123",
			key:         "secretKey456",
			bucket:      "test-bucket",
			shouldError: false,
			wantRegion:  "us-east-005",
		},
		{
			name:        "mixed case endpoint scheme",
			endpoint:    "HtTpS://s3.us-east-005.backblazeb2.com",
			keyID:       "keyId123",
			key:         "secretKey456",
			bucket:      "test-bucket",
			shouldError: false,
			wantRegion:  "us-east-005",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, "B2_ENDPOINT", tt.endpoint)
			setEnv(t, "B2_KEY_ID", tt.keyID)
			setEnv(t, "B2_KEY", tt.key)
			setEnv(t, "B2_BUCKET", tt.bucket)

			cfg, err := ParseSecondaryBackendConfig()

			if tt.shouldError {
				if err == nil {
					t.Error("ParseSecondaryBackendConfig() expected error, got nil")
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got: %v", tt.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseSecondaryBackendConfig() unexpected error: %v", err)
			}

			if cfg.Region != tt.wantRegion {
				t.Errorf("Region = %q, want %q", cfg.Region, tt.wantRegion)
			}
			if cfg.Endpoint != tt.endpoint {
				t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, tt.endpoint)
			}
		})
	}
}

// TestExtractRegionFromEndpoint tests the region extraction logic directly.
func TestExtractRegionFromEndpoint(t *testing.T) {
	tests := []struct {
		hostname string
		want     string
		wantErr  bool
		errContains string
	}{
		{
			hostname: "s3.us-east-005.backblazeb2.com",
			want:     "us-east-005",
			wantErr:  false,
		},
		{
			hostname: "s3.eu-west-002.backblazeb2.com",
			want:     "eu-west-002",
			wantErr:  false,
		},
		{
			hostname: "s3.us-east-005.backblazeb2.com:443",
			want:     "us-east-005",
			wantErr:  false,
		},
		{
			hostname: "s3.backblazeb2.com",
			want:     "",
			wantErr:  true,
			errContains: "match format",
		},
		{
			hostname: "api.backblazeb2.com",
			want:     "",
			wantErr:  true,
			errContains: "match format",
		},
		{
			hostname: "s3.us-east-005.example.com",
			want:     "",
			wantErr:  true,
			errContains: "must end with '.backblazeb2.com'",
		},
		{
			hostname: "s3..backblazeb2.com",
			want:     "",
			wantErr:  true,
			errContains: "region is empty",
		},
		{
			hostname: "s3.us-east-005.backblazeb2.net",
			want:     "",
			wantErr:  true,
			errContains: "must end with '.backblazeb2.com'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			got, err := extractRegionFromEndpoint(tt.hostname)

			if tt.wantErr {
				if err == nil {
					t.Errorf("extractRegionFromEndpoint() expected error, got nil")
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("Error should contain %q, got: %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("extractRegionFromEndpoint() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("extractRegionFromEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Helper functions

// setEnv sets an environment variable and registers a cleanup function to
// unset it when the test completes.
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Failed to set env var %q: %v", key, err)
	}
	t.Cleanup(func() {
		os.Unsetenv(key)
	})
}

// unsetEnv unsets multiple environment variables and registers cleanup
// functions to ensure they remain unset after the test.
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		os.Unsetenv(key)
		t.Cleanup(func() {
			os.Unsetenv(key)
		})
	}
}

// containsString checks if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

// containsSubstring is a helper for containsString.
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
