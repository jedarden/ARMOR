// Package handlers provides white-box tests for internal helper methods.
package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
)

// TestApplyPrefix tests the applyPrefix helper method.
func TestApplyPrefix(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		key      string
		expected string
	}{
		{
			name:     "empty prefix returns key unchanged",
			prefix:   "",
			key:      "test-key",
			expected: "test-key",
		},
		{
			name:     "prefix is added to key",
			prefix:   "my-prefix/",
			key:      "test-key",
			expected: "my-prefix/test-key",
		},
		{
			name:     "prefix without trailing slash",
			prefix:   "my-prefix",
			key:      "test-key",
			expected: "my-prefixtest-key", // Note: this is the current behavior - no auto slash
		},
		{
			name:     "nested path with prefix",
			prefix:   "apps/prod/",
			key:      "data/file.txt",
			expected: "apps/prod/data/file.txt",
		},
		{
			name:     "empty key with prefix",
			prefix:   "prefix/",
			key:      "",
			expected: "prefix/",
		},
		{
			name:     "key starting with slash",
			prefix:   "prefix/",
			key:      "/leading-slash",
			expected: "prefix//leading-slash",
		},
		{
			name:     "complex nested path",
			prefix:   "env/prod/tenant/",
			key:      "bucket/path/to/object.csv",
			expected: "env/prod/tenant/bucket/path/to/object.csv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Prefix:        tt.prefix,
				BlockSize:     65536,
				AuthAccessKey: "test",
				AuthSecretKey: "test",
			}
			h := &Handlers{config: cfg}

			result := h.applyPrefix(tt.key)
			if result != tt.expected {
				t.Errorf("applyPrefix(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

// TestStripPrefix tests the stripPrefix helper method.
func TestStripPrefix(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		key      string
		expected string
	}{
		{
			name:     "empty prefix returns key unchanged",
			prefix:   "",
			key:      "test-key",
			expected: "test-key",
		},
		{
			name:     "prefix is stripped from key",
			prefix:   "my-prefix/",
			key:      "my-prefix/test-key",
			expected: "test-key",
		},
		{
			name:     "key without prefix returns unchanged",
			prefix:   "my-prefix/",
			key:      "other-prefix/test-key",
			expected: "other-prefix/test-key",
		},
		{
			name:     "partial prefix match returns unchanged",
			prefix:   "my-prefix/",
			key:      "my-prefix-extra/test-key",
			expected: "my-prefix-extra/test-key",
		},
		{
			name:     "nested path with prefix stripped",
			prefix:   "apps/prod/",
			key:      "apps/prod/data/file.txt",
			expected: "data/file.txt",
		},
		{
			name:     "empty key with prefix",
			prefix:   "prefix/",
			key:      "",
			expected: "",
		},
		{
			name:     "prefix only returns empty",
			prefix:   "prefix/",
			key:      "prefix/",
			expected: "",
		},
		{
			name:     "case sensitive prefix mismatch",
			prefix:   "My-Prefix/",
			key:      "my-prefix/test-key",
			expected: "my-prefix/test-key",
		},
		{
			name:     "complex nested path with prefix",
			prefix:   "env/prod/tenant/",
			key:      "env/prod/tenant/bucket/path/object.csv",
			expected: "bucket/path/object.csv",
		},
		{
			name:     "key that is just prefix without trailing slash",
			prefix:   "prefix/",
			key:      "prefix",
			expected: "prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Prefix:        tt.prefix,
				BlockSize:     65536,
				AuthAccessKey: "test",
				AuthSecretKey: "test",
			}
			h := &Handlers{config: cfg}

			result := h.stripPrefix(tt.key)
			if result != tt.expected {
				t.Errorf("stripPrefix(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

// TestStripPrefixFromCommonPrefix tests the stripPrefixFromCommonPrefix helper method.
func TestStripPrefixFromCommonPrefix(t *testing.T) {
	tests := []struct {
		name         string
		prefix       string
		commonPrefix string
		expected     string
	}{
		{
			name:         "empty prefix returns commonPrefix unchanged",
			prefix:       "",
			commonPrefix: "test-dir/",
			expected:     "test-dir/",
		},
		{
			name:         "prefix is stripped from common prefix",
			prefix:       "my-prefix/",
			commonPrefix: "my-prefix/test-dir/",
			expected:     "test-dir/",
		},
		{
			name:         "common prefix without prefix returns unchanged",
			prefix:       "my-prefix/",
			commonPrefix: "other-dir/",
			expected:     "other-dir/",
		},
		{
			name:         "nested directory path with prefix stripped",
			prefix:       "apps/prod/",
			commonPrefix: "apps/prod/data/",
			expected:     "data/",
		},
		{
			name:         "common prefix without trailing slash",
			prefix:       "my-prefix/",
			commonPrefix: "my-prefix/test-dir",
			expected:     "test-dir",
		},
		{
			name:         "empty common prefix",
			prefix:       "my-prefix/",
			commonPrefix: "",
			expected:     "",
		},
		{
			name:         "prefix only returns empty",
			prefix:       "my-prefix/",
			commonPrefix: "my-prefix/",
			expected:     "",
		},
		{
			name:         "partial prefix match returns unchanged",
			prefix:       "my-prefix/",
			commonPrefix: "my-prefix-extra/data/",
			expected:     "my-prefix-extra/data/",
		},
		{
			name:         "complex nested directory",
			prefix:       "env/prod/tenant/",
			commonPrefix: "env/prod/tenant/bucket/path/",
			expected:     "bucket/path/",
		},
		{
			name:         "common prefix matching but no slash - returns unchanged since HasPrefix requires exact match",
			prefix:       "my-prefix/",
			commonPrefix: "my-prefix",
			expected:     "my-prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Prefix:        tt.prefix,
				BlockSize:     65536,
				AuthAccessKey: "test",
				AuthSecretKey: "test",
			}
			h := &Handlers{config: cfg}

			result := h.stripPrefixFromCommonPrefix(tt.commonPrefix)
			if result != tt.expected {
				t.Errorf("stripPrefixFromCommonPrefix(%q) = %q, want %q", tt.commonPrefix, result, tt.expected)
			}
		})
	}
}

// TestPrefixRoundTrip tests that applyPrefix and stripPrefix are inverse operations.
func TestPrefixRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		key    string
	}{
		{
			name:   "simple key with prefix",
			prefix: "my-prefix/",
			key:    "test-key",
		},
		{
			name:   "nested path with prefix",
			prefix: "apps/prod/",
			key:    "data/file.txt",
		},
		{
			name:   "key with multiple segments",
			prefix: "bucket/",
			key:    "a/b/c/d/file.txt",
		},
		{
			name:   "empty prefix",
			prefix: "",
			key:    "test-key",
		},
		{
			name:   "deeply nested path",
			prefix: "env/prod/tenant/app/",
			key:    "data/year=2024/month=06/file.parquet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Prefix:        tt.prefix,
				BlockSize:     65536,
				AuthAccessKey: "test",
				AuthSecretKey: "test",
			}
			h := &Handlers{config: cfg}

			// Apply prefix then strip it - should get original key back
			prefixed := h.applyPrefix(tt.key)
			stripped := h.stripPrefix(prefixed)

			if stripped != tt.key {
				t.Errorf("round trip failed: applyPrefix(%q) = %q, stripPrefix(%q) = %q, want %q",
					tt.key, prefixed, prefixed, stripped, tt.key)
			}
		})
	}
}

// TestPrefixMethodsEdgeCases tests edge cases specific to S3 prefix handling.
func TestPrefixMethodsEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		prefix        string
		key           string
		applyExpected string
		stripExpected string
	}{
		{
			name:          "unicode characters in key",
			prefix:        "prefix/",
			key:           "test/文件.txt",
			applyExpected: "prefix/test/文件.txt",
			stripExpected: "test/文件.txt",
		},
		{
			name:          "url-encoded characters",
			prefix:        "prefix/",
			key:           "test/file%20name.txt",
			applyExpected: "prefix/test/file%20name.txt",
			stripExpected: "test/file%20name.txt",
		},
		{
			name:          "key with spaces",
			prefix:        "prefix/",
			key:           "test/file name.txt",
			applyExpected: "prefix/test/file name.txt",
			stripExpected: "test/file name.txt",
		},
		{
			name:          "very long key",
			prefix:        "a/",
			key:           string(make([]byte, 1024)), // 1KB key
			applyExpected: "a/" + string(make([]byte, 1024)),
			stripExpected: string(make([]byte, 1024)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Prefix:        tt.prefix,
				BlockSize:     65536,
				AuthAccessKey: "test",
				AuthSecretKey: "test",
			}
			h := &Handlers{config: cfg}

			// Test applyPrefix
			applyResult := h.applyPrefix(tt.key)
			if applyResult != tt.applyExpected {
				t.Errorf("applyPrefix(%q) = %q, want %q", tt.key, applyResult, tt.applyExpected)
			}

			// Test stripPrefix on the prefixed result
			stripResult := h.stripPrefix(applyResult)
			if stripResult != tt.stripExpected {
				t.Errorf("stripPrefix(%q) = %q, want %q", applyResult, stripResult, tt.stripExpected)
			}
		})
	}
}

// freshnessHeadBackend serves Head results with a fixed LastModified so tests
// can pin the ciphertext timestamp independently of the manifest's completedAt.
// All other backend operations come from the embedded NilBackend.
type freshnessHeadBackend struct {
	*backend.NilBackend
	lastModified time.Time
	headErr      error
}

func (b *freshnessHeadBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	if b.headErr != nil {
		return nil, b.headErr
	}
	return &backend.ObjectInfo{Key: key, LastModified: b.lastModified}, nil
}

// TestVerifyCiphertextFreshness covers both timestamp orderings of the
// ciphertext freshness gate on the manifest read path.
//
// CompleteMultipartUpload assembles the ciphertext first and writes the
// manifest afterwards, so "ciphertext predates manifest completion" is the
// normal ordering and must verify cleanly (regression: this used to be
// rejected, permanently 500ing GetObject on every multipart object whose
// completion outlived its assembly second). Only a ciphertext NEWER than the
// manifest — an overwrite by a later same-key upload landing between the two
// manifest writes — is stale.
func TestVerifyCiphertextFreshness(t *testing.T) {
	manifestCompleted := time.Date(2026, 8, 31, 22, 48, 24, 0, time.UTC)

	tests := []struct {
		name          string
		ciphertext    time.Time
		completedAt   string
		headErr       error
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name:        "ciphertext predates manifest completion is served (normal multipart ordering)",
			ciphertext:  manifestCompleted.Add(-time.Minute), // production incident shape: 60s assembly gap
			completedAt: manifestCompleted.Format(time.RFC3339),
		},
		{
			name:        "ciphertext in the same second as manifest completion is served",
			ciphertext:  manifestCompleted,
			completedAt: manifestCompleted.Add(999 * time.Millisecond).Format(time.RFC3339),
		},
		{
			name:          "ciphertext newer than manifest completion is stale (overwritten)",
			ciphertext:    manifestCompleted.Add(time.Minute),
			completedAt:   manifestCompleted.Format(time.RFC3339),
			wantErr:       true,
			wantErrSubstr: "overwritten after manifest completion",
		},
		{
			name:        "unparseable completedAt skips verification",
			ciphertext:  manifestCompleted.Add(time.Hour),
			completedAt: "not-a-timestamp",
		},
		{
			name:          "ciphertext head failure propagates",
			ciphertext:    manifestCompleted.Add(-time.Minute),
			completedAt:   manifestCompleted.Format(time.RFC3339),
			headErr:       errors.New("backend unavailable"),
			wantErr:       true,
			wantErrSubstr: "failed to head ciphertext object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handlers{
				config:  &config.Config{BlockSize: 65536},
				backend: &freshnessHeadBackend{lastModified: tt.ciphertext, headErr: tt.headErr},
			}

			err := h.verifyCiphertextFreshness(context.Background(), "bucket", "bucket/key", tt.completedAt)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("verifyCiphertextFreshness() = nil, want error containing %q", tt.wantErrSubstr)
				}
				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Errorf("verifyCiphertextFreshness() error = %v, want it to contain %q", err, tt.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Errorf("verifyCiphertextFreshness() = %v, want nil", err)
			}
		})
	}
}
