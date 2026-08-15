// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"strings"
	"testing"
)

// TestParseSecondaryBackendConfigStringValid tests valid config string formats.
func TestParseSecondaryBackendConfigStringValid(t *testing.T) {
	tests := []struct {
		name      string
		configStr string
		want      BackendConfig
	}{
		{
			name:      "filesystem absolute path",
			configStr: "filesystem:/backup/armor",
			want: BackendConfig{
				Type: "filesystem",
				Path: "/backup/armor",
			},
		},
		{
			name:      "filesystem relative path",
			configStr: "filesystem:./backup",
			want: BackendConfig{
				Type: "filesystem",
				Path: "./backup",
			},
		},
		{
			name:      "filesystem with trailing slash",
			configStr: "filesystem:/backup/armor/",
			want: BackendConfig{
				Type: "filesystem",
				Path: "/backup/armor/",
			},
		},
		{
			name:      "b2 all fields",
			configStr: "b2:mybucket:appKeyId:accountId:appKey",
			want: BackendConfig{
				Type:        "b2",
				Bucket:      "mybucket",
				AccessKeyID: "appKeyId",
				SecretKey:   "appKey",
			},
		},
		{
			name:      "b2 with hyphenated bucket",
			configStr: "b2:my-bucket-123:keyABC:id123:secretXYZ",
			want: BackendConfig{
				Type:        "b2",
				Bucket:      "my-bucket-123",
				AccessKeyID: "keyABC",
				SecretKey:   "secretXYZ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSecondaryBackendConfigString(tt.configStr)
			if err != nil {
				t.Fatalf("ParseSecondaryBackendConfigString() error: %v", err)
			}

			compareBackendConfigs(t, got, tt.want)
		})
	}
}

// TestParseSecondaryBackendConfigStringEmpty tests empty string handling.
func TestParseSecondaryBackendConfigStringEmpty(t *testing.T) {
	cfg, err := ParseSecondaryBackendConfigString("")
	if err != nil {
		t.Fatalf("ParseSecondaryBackendConfigString() unexpected error: %v", err)
	}

	// Should return zero config (disabled state)
	if cfg.Type != "" {
		t.Errorf("Type should be empty for disabled backend, got %q", cfg.Type)
	}
	if cfg.Path != "" {
		t.Errorf("Path should be empty for disabled backend, got %q", cfg.Path)
	}
	if cfg.Bucket != "" {
		t.Errorf("Bucket should be empty for disabled backend, got %q", cfg.Bucket)
	}
}

// TestParseSecondaryBackendConfigStringInvalidType tests unsupported backend types.
func TestParseSecondaryBackendConfigStringInvalidType(t *testing.T) {
	tests := []struct {
		name      string
		configStr string
		wantErr   string
	}{
		{
			name:      "s3 type",
			configStr: "s3:mybucket:keyid:secret",
			wantErr:   "unsupported backend type",
		},
		{
			name:      "wasabi type",
			configStr: "wasabi:bucket:creds",
			wantErr:   "unsupported backend type",
		},
		{
			name:      "unknown type",
			configStr: "unknown:some:params",
			wantErr:   "unsupported backend type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSecondaryBackendConfigString(tt.configStr)
			if err == nil {
				t.Error("ParseSecondaryBackendConfigString() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Error should contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

// TestParseSecondaryBackendConfigStringMalformed tests malformed config strings.
func TestParseSecondaryBackendConfigStringMalformed(t *testing.T) {
	tests := []struct {
		name      string
		configStr string
		wantErr   string
	}{
		{
			name:      "no colon separator",
			configStr: "filesystem",
			wantErr:   "invalid config format",
		},
		{
			name:      "missing params after colon",
			configStr: "filesystem:",
			wantErr:   "params cannot be empty",
		},
		{
			name:      "type only with colon",
			configStr: "b2:",
			wantErr:   "params cannot be empty",
		},
		{
			name:      "empty filesystem path",
			configStr: "filesystem:  ",
			wantErr:   "filesystem path cannot be empty",
		},
		{
			name:      "b2 missing fields (only 3 fields)",
			configStr: "b2:bucket:key:id",
			wantErr:   "invalid B2 format",
		},
		{
			name:      "b2 too many fields (5 fields)",
			configStr: "b2:bucket:key:id:secret:extra",
			wantErr:   "invalid B2 format",
		},
		{
			name:      "b2 empty bucket",
			configStr: "b2::key:id:secret",
			wantErr:   "B2 bucket cannot be empty",
		},
		{
			name:      "b2 empty key",
			configStr: "b2:bucket::id:secret",
			wantErr:   "B2 key",
		},
		{
			name:      "b2 empty id",
			configStr: "b2:bucket:key::secret",
			wantErr:   "B2 id",
		},
		{
			name:      "b2 empty secret",
			configStr: "b2:bucket:key:id:",
			wantErr:   "B2 secret",
		},
		{
			name:      "b2 all empty fields",
			configStr: "b2::::",
			wantErr:   "B2 bucket cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSecondaryBackendConfigString(tt.configStr)
			if err == nil {
				t.Error("ParseSecondaryBackendConfigString() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Error should contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

// TestParseSecondaryBackendConfigStringWhitespace tests handling of whitespace.
func TestParseSecondaryBackendConfigStringWhitespace(t *testing.T) {
	tests := []struct {
		name      string
		configStr string
		want      BackendConfig
	}{
		{
			name:      "filesystem with spaces around path",
			configStr: "filesystem:  /backup/armor  ",
			want: BackendConfig{
				Type: "filesystem",
				Path: "/backup/armor",
			},
		},
		{
			name:      "b2 with spaces around all fields",
			configStr: "b2: mybucket : keyId : accountId : appKey",
			want: BackendConfig{
				Type:        "b2",
				Bucket:      "mybucket",
				AccessKeyID: "keyId",
				SecretKey:   "appKey",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSecondaryBackendConfigString(tt.configStr)
			if err != nil {
				t.Fatalf("ParseSecondaryBackendConfigString() error: %v", err)
			}

			compareBackendConfigs(t, got, tt.want)
		})
	}
}

// TestParseSecondaryBackendConfigStringCase tests case handling.
func TestParseSecondaryBackendConfigStringCase(t *testing.T) {
	tests := []struct {
		name      string
		configStr string
		wantType  string
	}{
		{
			name:      "filesystem uppercase",
			configStr: "FILESYSTEM:/path",
			wantType:  "filesystem",
		},
		{
			name:      "filesystem mixed case",
			configStr: "FileSystem:/path",
			wantType:  "filesystem",
		},
		{
			name:      "b2 uppercase",
			configStr: "B2:bucket:key:id:secret",
			wantType:  "b2",
		},
		{
			name:      "b2 mixed case",
			configStr: "B2:bucket:key:id:secret",
			wantType:  "b2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSecondaryBackendConfigString(tt.configStr)
			if err != nil {
				t.Fatalf("ParseSecondaryBackendConfigString() error: %v", err)
			}

			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
		})
	}
}

// compareBackendConfigs compares two BackendConfig values for test assertions.
func compareBackendConfigs(t *testing.T, got, want BackendConfig) {
	t.Helper()

	if got.Type != want.Type {
		t.Errorf("Type = %q, want %q", got.Type, want.Type)
	}
	if got.Path != want.Path {
		t.Errorf("Path = %q, want %q", got.Path, want.Path)
	}
	if got.Bucket != want.Bucket {
		t.Errorf("Bucket = %q, want %q", got.Bucket, want.Bucket)
	}
	if got.AccessKeyID != want.AccessKeyID {
		t.Errorf("AccessKeyID = %q, want %q", got.AccessKeyID, want.AccessKeyID)
	}
	if got.SecretKey != want.SecretKey {
		t.Errorf("SecretKey = %q, want %q", got.SecretKey, want.SecretKey)
	}
}
