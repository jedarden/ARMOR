// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"strings"
	"testing"
)

// TestValidateB2Config exercises every empty-field branch of validateB2Config.
// Each case starts from a fully-populated, valid BackendConfig and blanks
// exactly one B2 field, then asserts the returned error names that field. This
// confirms the validation rejects each missing/empty parameter individually
// with an operator-actionable message.
func TestValidateB2Config(t *testing.T) {
	// A fully-populated config that every case starts from.
	valid := BackendConfig{
		Type:        "b2",
		Bucket:      "mybucket",
		Region:      "us-east-005",
		Endpoint:    "https://s3.us-east-005.backblazeb2.com",
		AccessKeyID: "KEYID",
		SecretKey:   "SECRET",
	}

	tests := []struct {
		name        string
		mutate      func(*BackendConfig)
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid config returns nil",
			mutate:  func(*BackendConfig) {},
			wantErr: false,
		},
		{
			name:        "empty bucket",
			mutate:      func(c *BackendConfig) { c.Bucket = "" },
			wantErr:     true,
			errContains: "bucket",
		},
		{
			name:        "empty region",
			mutate:      func(c *BackendConfig) { c.Region = "" },
			wantErr:     true,
			errContains: "region",
		},
		{
			name:        "empty endpoint",
			mutate:      func(c *BackendConfig) { c.Endpoint = "" },
			wantErr:     true,
			errContains: "endpoint",
		},
		{
			name:        "empty access key id",
			mutate:      func(c *BackendConfig) { c.AccessKeyID = "" },
			wantErr:     true,
			errContains: "access key ID",
		},
		{
			name:        "empty secret key",
			mutate:      func(c *BackendConfig) { c.SecretKey = "" },
			wantErr:     true,
			errContains: "secret key",
		},
		{
			// All fields blank: validateB2Config returns the first missing
			// field it checks (bucket), not a generic "everything is wrong".
			name:        "all fields empty reports first missing field",
			mutate:      func(c *BackendConfig) { *c = BackendConfig{Type: "b2"} },
			wantErr:     true,
			errContains: "bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)

			err := validateB2Config(cfg)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateB2Config_ErrorMessages confirms each error message names the
// offending field verbatim, so an operator reading a log line can locate the
// config key to fix. This guards against silent rewording that would weaken
// the operator-actionability requirement.
func TestValidateB2Config_ErrorMessages(t *testing.T) {
	cases := []struct {
		emptyField string
		mutate     func(*BackendConfig)
		wantSubstr string
	}{
		{"bucket", func(c *BackendConfig) { c.Bucket = "" }, "bucket is required"},
		{"region", func(c *BackendConfig) { c.Region = "" }, "region is required"},
		{"endpoint", func(c *BackendConfig) { c.Endpoint = "" }, "endpoint is required"},
		{"access key id", func(c *BackendConfig) { c.AccessKeyID = "" }, "access key ID is required"},
		{"secret key", func(c *BackendConfig) { c.SecretKey = "" }, "secret key is required"},
	}

	base := BackendConfig{
		Bucket:      "mybucket",
		Region:      "us-east-005",
		Endpoint:    "https://s3.us-east-005.backblazeb2.com",
		AccessKeyID: "KEYID",
		SecretKey:   "SECRET",
	}

	for _, tc := range cases {
		t.Run(tc.emptyField, func(t *testing.T) {
			cfg := base
			tc.mutate(&cfg)

			err := validateB2Config(cfg)
			if err == nil {
				t.Fatalf("expected error for empty %s, got nil", tc.emptyField)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not name the field; want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}
