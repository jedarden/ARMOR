package config

import (
	"encoding/hex"
	"fmt"
	"github.com/jedarden/armor/internal/acl"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseACL(t *testing.T) {
	tests := []struct {
		name        string
		aclStr      string
		expectCount int
		expectError bool
		checkFunc   func([]acl.ACLEntry) bool
	}{
		{
			name:        "empty string - returns nil",
			aclStr:      "",
			expectCount: 0, // parseACL returns nil for empty string
		},
		{
			name:        "single bucket with wildcard prefix",
			aclStr:      "my-bucket:*",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == ""
			},
		},
		{
			name:        "single bucket with specific prefix",
			aclStr:      "my-bucket:data/",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == "data/"
			},
		},
		{
			name:        "multiple entries",
			aclStr:      "bucket-a:prefix-a/,bucket-b:prefix-b/",
			expectCount: 2,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "bucket-a" && acls[0].Prefix == "prefix-a/" &&
					acls[1].Bucket == "bucket-b" && acls[1].Prefix == "prefix-b/"
			},
		},
		{
			name:        "wildcard bucket",
			aclStr:      "*:public/",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "*" && acls[0].Prefix == "public/"
			},
		},
		{
			// Backward compatibility: a two-segment entry specifies no verbs,
			// so Actions must be nil (which reads as "all verbs permitted").
			name:        "two-segment defaults to all actions (nil map)",
			aclStr:      "my-bucket:data/",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == "data/" && acls[0].Actions == nil
			},
		},
		{
			name:        "three-segment single verb",
			aclStr:      "mybucket:foo/:get",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "mybucket" && acls[0].Prefix == "foo/" &&
					actionsEqual(acls[0].Actions, "get")
			},
		},
		{
			name:        "three-segment multiple verbs plus-separated",
			aclStr:      "mybucket:foo/:get+list",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "mybucket" && acls[0].Prefix == "foo/" &&
					actionsEqual(acls[0].Actions, "get", "list")
			},
		},
		{
			name:        "three-segment multiple verbs space-separated",
			aclStr:      "mybucket:foo/:put delete",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "mybucket" && acls[0].Prefix == "foo/" &&
					actionsEqual(acls[0].Actions, "put", "delete")
			},
		},
		{
			name:        "three-segment mixed separators and whitespace",
			aclStr:      "mybucket:foo/:get + list",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "mybucket" && acls[0].Prefix == "foo/" &&
					actionsEqual(acls[0].Actions, "get", "list")
			},
		},
		{
			name:        "three-segment all four verbs",
			aclStr:      "bucket:/:get+put+delete+list",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "bucket" && acls[0].Prefix == "/" &&
					actionsEqual(acls[0].Actions, "get", "put", "delete", "list")
			},
		},
		{
			// A present-but-empty third segment is treated like an absent one:
			// no verbs specified → all permitted (nil map).
			name:        "trailing empty segment defaults to all actions",
			aclStr:      "bucket:prefix/:",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "bucket" && acls[0].Prefix == "prefix/" && acls[0].Actions == nil
			},
		},
		{
			// Verb scoping is per-entry: only the scoped entry is restricted.
			name:        "mixed scoped and unscoped entries",
			aclStr:      "bucket-a:data/:get+list,bucket-b:other/",
			expectCount: 2,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "bucket-a" && acls[0].Prefix == "data/" &&
					actionsEqual(acls[0].Actions, "get", "list") &&
					acls[1].Bucket == "bucket-b" && acls[1].Prefix == "other/" && acls[1].Actions == nil
			},
		},
		{
			name:        "three-segment invalid verb",
			aclStr:      "bucket:foo/:read",
			expectError: true,
		},
		{
			name:        "three-segment unknown verb among valid ones",
			aclStr:      "bucket:foo/:get+write",
			expectError: true,
		},
		{
			// Verbs are case-sensitive: capitalized forms are rejected.
			name:        "three-segment uppercase verb rejected (case-sensitive)",
			aclStr:      "bucket:foo/:Get",
			expectError: true,
		},
		{
			name:        "invalid format - missing colon",
			aclStr:      "bucket-only",
			expectError: true,
		},
		{
			name:        "invalid format - empty bucket",
			aclStr:      ":prefix/",
			expectError: true,
		},
		{
			name:        "trailing wildcard normalized to prefix",
			aclStr:      "my-bucket:data/*",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == "data/"
			},
		},
		{
			name:        "trailing wildcard with actions",
			aclStr:      "my-bucket:data/*:get+list",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == "data/" &&
					actionsEqual(acls[0].Actions, "get", "list")
			},
		},
		{
			name:        "explicit prefix with trailing slash unchanged",
			aclStr:      "my-bucket:data/",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == "data/"
			},
		},
		{
			name:        "bare wildcard normalized to empty prefix",
			aclStr:      "my-bucket:*",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == ""
			},
		},
		{
			name:        "interior wildcard rejected",
			aclStr:      "my-bucket:data*",
			expectError: true,
		},
		{
			name:        "interior wildcard with suffix rejected",
			aclStr:      "my-bucket:data*x",
			expectError: true,
		},
		{
			name:        "multiple wildcards rejected",
			aclStr:      "my-bucket:data*test*",
			expectError: true,
		},
		{
			name:        "wildcard in middle of prefix rejected",
			aclStr:      "my-bucket:da*ta/",
			expectError: true,
		},
		{
			name:        "wildcard only in bucket (valid)",
			aclStr:      "*:data/",
			expectCount: 1,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "*" && acls[0].Prefix == "data/"
			},
		},
		{
			name:        "mixed wildcard and explicit prefix entries",
			aclStr:      "bucket:*:get,bucket:data/*:list,bucket:specific/",
			expectCount: 3,
			checkFunc: func(acls []acl.ACLEntry) bool {
				return acls[0].Bucket == "bucket" && acls[0].Prefix == "" &&
					actionsEqual(acls[0].Actions, "get") &&
					acls[1].Bucket == "bucket" && acls[1].Prefix == "data/" &&
					actionsEqual(acls[1].Actions, "list") &&
					acls[2].Bucket == "bucket" && acls[2].Prefix == "specific/" &&
					acls[2].Actions == nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acls, err := parseACL(tt.aclStr)

			if tt.name == "empty string - returns nil" {
				// parseACL returns nil for empty string (meaning no ACLs)
				if acls != nil {
					t.Errorf("expected nil for empty string, got %v", acls)
				}
				return
			}

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(acls) != tt.expectCount {
				t.Errorf("expected %d ACLs, got %d", tt.expectCount, len(acls))
			}

			if tt.checkFunc != nil && !tt.checkFunc(acls) {
				t.Error("ACL check function failed")
			}
		})
	}
}

func TestParseKeyRoutes(t *testing.T) {
	tests := []struct {
		name        string
		routesStr   string
		expectCount int
		expectError bool
		checkFunc   func([]KeyRoute) bool
	}{
		{
			name:        "empty string",
			routesStr:   "",
			expectCount: 0,
		},
		{
			name:        "single route",
			routesStr:   "data/=sensitive",
			expectCount: 1,
			checkFunc: func(routes []KeyRoute) bool {
				return routes[0].Prefix == "data/" && routes[0].KeyName == "sensitive"
			},
		},
		{
			name:        "multiple routes",
			routesStr:   "data/pii/*=sensitive,archive/*=archive,*=default",
			expectCount: 3,
			checkFunc: func(routes []KeyRoute) bool {
				return routes[0].Prefix == "data/pii/" && routes[0].KeyName == "sensitive" &&
					routes[1].Prefix == "archive/" && routes[1].KeyName == "archive" &&
					routes[2].Prefix == "" && routes[2].KeyName == "default"
			},
		},
		{
			name:        "invalid format",
			routesStr:   "invalid-format",
			expectError: true,
		},
		{
			name:        "empty prefix",
			routesStr:   "=keyname",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes, err := parseKeyRoutes(tt.routesStr)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(routes) != tt.expectCount {
				t.Errorf("expected %d routes, got %d", tt.expectCount, len(routes))
			}

			if tt.checkFunc != nil && !tt.checkFunc(routes) {
				t.Error("route check function failed")
			}
		})
	}
}

func TestLoadMultiKeyConfiguration(t *testing.T) {
	env := append(minimalEnv(),
		"ARMOR_MEK_SENSITIVE", "1111111111111111111111111111111111111111111111111111111111111111",
		"ARMOR_MEK_ARCHIVE", "2222222222222222222222222222222222222222222222222222222222222222",
		"ARMOR_KEY_ROUTES", "data/pii/*=SENSITIVE,archive/*=archive,*=default",
	)
	setEnv(t, env...)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.NamedKeys) != 2 {
		t.Fatalf("NamedKeys count = %d, want 2", len(cfg.NamedKeys))
	}
	if _, ok := cfg.NamedKeys["sensitive"]; !ok {
		t.Error("NamedKeys missing sensitive key")
	}
	if _, ok := cfg.NamedKeys["archive"]; !ok {
		t.Error("NamedKeys missing archive key")
	}
	if len(cfg.KeyRoutes) != 3 {
		t.Fatalf("KeyRoutes count = %d, want 3", len(cfg.KeyRoutes))
	}
	want := []KeyRoute{
		{Prefix: "data/pii/", KeyName: "sensitive"},
		{Prefix: "archive/", KeyName: "archive"},
		{Prefix: "", KeyName: "default"},
	}
	for i, route := range cfg.KeyRoutes {
		if route != want[i] {
			t.Errorf("KeyRoutes[%d] = %+v, want %+v", i, route, want[i])
		}
	}
}

// setEnv sets multiple env vars for the duration of a test and restores them in
// cleanup. Returns a teardown function (also registered via t.Cleanup).
func setEnv(t *testing.T, pairs ...string) {
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

// minimalEnv returns the set of required env var pairs needed for Load() to succeed.
func minimalEnv() []string {
	return []string{
		"ARMOR_B2_REGION", "us-east-005",
		"ARMOR_B2_ACCESS_KEY_ID", "testkey",
		"ARMOR_B2_SECRET_ACCESS_KEY", "testsecret",
		"ARMOR_BUCKET", "testbucket",
		"ARMOR_CF_DOMAIN", "test.example.com",
		"ARMOR_MEK", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
	}
}

func TestManifestConfigDefaults(t *testing.T) {
	setEnv(t, minimalEnv()...)
	// Unset manifest vars so defaults apply.
	for _, k := range []string{"ARMOR_MANIFEST_ENABLED", "ARMOR_MANIFEST_PREFIX", "ARMOR_MANIFEST_COMPACTION_INTERVAL", "ARMOR_MANIFEST_COMPACTION_THRESHOLD"} {
		os.Unsetenv(k)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !cfg.ManifestEnabled {
		t.Error("ManifestEnabled should default to true")
	}
	if cfg.ManifestPrefix != ".armor/manifest" {
		t.Errorf("ManifestPrefix default = %q, want .armor/manifest", cfg.ManifestPrefix)
	}
	if cfg.ManifestCompactionInterval != 3600 {
		t.Errorf("ManifestCompactionInterval default = %d, want 3600", cfg.ManifestCompactionInterval)
	}
	if cfg.ManifestCompactionThreshold != 1000 {
		t.Errorf("ManifestCompactionThreshold default = %d, want 1000", cfg.ManifestCompactionThreshold)
	}
}

func TestReadConcurrencyConfig(t *testing.T) {
	env := append(minimalEnv(), "ARMOR_READ_CONCURRENCY", "16")
	setEnv(t, env...)
	os.Unsetenv("ARMOR_READ_CONCURRENCY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ReadConcurrency != 16 {
		t.Fatalf("ReadConcurrency default = %d, want 16", cfg.ReadConcurrency)
	}

	os.Setenv("ARMOR_READ_CONCURRENCY", "4")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() with ARMOR_READ_CONCURRENCY=4: %v", err)
	}
	if cfg.ReadConcurrency != 4 {
		t.Fatalf("ReadConcurrency = %d, want 4", cfg.ReadConcurrency)
	}

	os.Setenv("ARMOR_READ_CONCURRENCY", "0")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted ARMOR_READ_CONCURRENCY=0")
	}
}

func TestManifestEnabledFalse(t *testing.T) {
	setEnv(t, minimalEnv()...)
	for _, v := range []string{"false", "0"} {
		os.Setenv("ARMOR_MANIFEST_ENABLED", v)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error with ARMOR_MANIFEST_ENABLED=%s: %v", v, err)
		}
		if cfg.ManifestEnabled {
			t.Errorf("ManifestEnabled should be false when env var = %q", v)
		}
	}
}

func TestManifestEnabledTrue(t *testing.T) {
	setEnv(t, minimalEnv()...)
	for _, v := range []string{"true", "1", "yes", ""} {
		os.Setenv("ARMOR_MANIFEST_ENABLED", v)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error with ARMOR_MANIFEST_ENABLED=%s: %v", v, err)
		}
		if !cfg.ManifestEnabled {
			t.Errorf("ManifestEnabled should be true when env var = %q", v)
		}
	}
}

func TestManifestPrefix(t *testing.T) {
	setEnv(t, minimalEnv()...)
	os.Setenv("ARMOR_MANIFEST_PREFIX", ".custom/manifest/prefix")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ManifestPrefix != ".custom/manifest/prefix" {
		t.Errorf("ManifestPrefix = %q, want .custom/manifest/prefix", cfg.ManifestPrefix)
	}
}

func TestManifestCompactionInterval(t *testing.T) {
	setEnv(t, minimalEnv()...)
	os.Setenv("ARMOR_MANIFEST_COMPACTION_INTERVAL", "7200")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ManifestCompactionInterval != 7200 {
		t.Errorf("ManifestCompactionInterval = %d, want 7200", cfg.ManifestCompactionInterval)
	}
}

func TestNormalizePrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no trailing slash",
			input:    "kalshi-tape",
			expected: "kalshi-tape/",
		},
		{
			name:     "with trailing slash",
			input:    "kalshi-tape/",
			expected: "kalshi-tape/",
		},
		{
			name:     "leading slash without trailing",
			input:    "/kalshi-tape",
			expected: "kalshi-tape/",
		},
		{
			name:     "both leading and trailing slash",
			input:    "/kalshi-tape/",
			expected: "kalshi-tape/",
		},
		{
			name:     "multiple trailing slashes",
			input:    "kalshi-tape//",
			expected: "kalshi-tape/",
		},
		{
			name:     "multiple leading slashes",
			input:    "//kalshi-tape",
			expected: "kalshi-tape/",
		},
		{
			name:     "nested path without trailing slash",
			input:    "env/prod",
			expected: "env/prod/",
		},
		{
			name:     "nested path with trailing slash",
			input:    "env/prod/",
			expected: "env/prod/",
		},
		{
			name:     "only slashes",
			input:    "///",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePrefix(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePrefix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestArmorPrefix(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "no prefix set",
			envValue: "",
			expected: "",
		},
		{
			name:     "simple prefix",
			envValue: "kalshi-tape",
			expected: "kalshi-tape/",
		},
		{
			name:     "prefix with trailing slash",
			envValue: "kalshi-tape/",
			expected: "kalshi-tape/",
		},
		{
			name:     "nested path",
			envValue: "prod/data",
			expected: "prod/data/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, minimalEnv()...)
			if tt.envValue != "" {
				os.Setenv("ARMOR_PREFIX", tt.envValue)
			} else {
				os.Unsetenv("ARMOR_PREFIX")
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			if cfg.Prefix != tt.expected {
				t.Errorf("Prefix = %q, want %q", cfg.Prefix, tt.expected)
			}
		})
	}
}

func TestLoadReportsMultipleErrors(t *testing.T) {
	// Unset all required env vars
	for _, k := range []string{
		"ARMOR_B2_REGION",
		"ARMOR_B2_ACCESS_KEY_ID",
		"ARMOR_B2_SECRET_ACCESS_KEY",
		"ARMOR_BUCKET",
		"ARMOR_MEK",
	} {
		os.Unsetenv(k)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error when required vars are missing")
	}

	// Verify all missing variables are reported in the error message
	errMsg := err.Error()
	for _, k := range []string{
		"ARMOR_B2_REGION",
		"ARMOR_B2_ACCESS_KEY_ID",
		"ARMOR_B2_SECRET_ACCESS_KEY",
		"ARMOR_BUCKET",
		"ARMOR_MEK",
	} {
		if !strings.Contains(errMsg, k+" is required") && !strings.Contains(errMsg, k) {
			t.Errorf("Error message should mention missing %s, got: %v", k, errMsg)
		}
	}
}

func TestLoadWithThreeMissingRequiredVars(t *testing.T) {
	// Set only B2_REGION and SECRET_KEY, missing ACCESS_KEY_ID, BUCKET, and MEK
	os.Unsetenv("ARMOR_B2_REGION")
	os.Unsetenv("ARMOR_B2_ACCESS_KEY_ID")
	os.Unsetenv("ARMOR_B2_SECRET_ACCESS_KEY")
	os.Unsetenv("ARMOR_BUCKET")
	os.Unsetenv("ARMOR_MEK")

	os.Setenv("ARMOR_B2_REGION", "us-west-001")
	os.Setenv("ARMOR_B2_SECRET_ACCESS_KEY", "testsecret")
	defer func() {
		os.Unsetenv("ARMOR_B2_REGION")
		os.Unsetenv("ARMOR_B2_SECRET_ACCESS_KEY")
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error when required vars are missing")
	}

	// Verify all three missing variables are reported in the error
	errMsg := err.Error()
	for _, k := range []string{
		"ARMOR_B2_ACCESS_KEY_ID",
		"ARMOR_BUCKET",
		"ARMOR_MEK",
	} {
		if !strings.Contains(errMsg, k) {
			t.Errorf("Error message should mention missing %s, got: %v", k, errMsg)
		}
	}

	// Verify the ones that ARE set are NOT in the error
	if strings.Contains(errMsg, "ARMOR_B2_REGION is required") {
		t.Error("ARMOR_B2_REGION should not be reported as missing (it was set)")
	}
	if strings.Contains(errMsg, "ARMOR_B2_SECRET_ACCESS_KEY is required") {
		t.Error("ARMOR_B2_SECRET_ACCESS_KEY should not be reported as missing (it was set)")
	}
}

// actionsEqual reports whether got matches the expected set of verbs (a nil
// got is treated as the empty set). A nil expected asserts the set is empty.
func TestLoadWithMultipleCredentialErrors(t *testing.T) {
	// Set minimal required env vars
	setEnv(t, minimalEnv()...)

	// Set up multiple named credentials with errors:
	// - "CRED1": missing secret key
	// - "CRED2": has both keys but invalid ACL
	// - "CRED3": duplicate access key (same as CRED2)
	os.Setenv("ARMOR_AUTH_CRED1_ACCESS_KEY", "cred1key")
	// Missing: ARMOR_AUTH_CRED1_SECRET_KEY

	os.Setenv("ARMOR_AUTH_CRED2_ACCESS_KEY", "cred2key")
	os.Setenv("ARMOR_AUTH_CRED2_SECRET_KEY", "cred2secret")
	os.Setenv("ARMOR_AUTH_CRED2_ACL", "invalid acl format") // Invalid ACL

	os.Setenv("ARMOR_AUTH_CRED3_ACCESS_KEY", "cred2key") // Duplicate of CRED2
	os.Setenv("ARMOR_AUTH_CRED3_SECRET_KEY", "cred3secret")

	defer func() {
		os.Unsetenv("ARMOR_AUTH_CRED1_ACCESS_KEY")
		os.Unsetenv("ARMOR_AUTH_CRED2_ACCESS_KEY")
		os.Unsetenv("ARMOR_AUTH_CRED2_SECRET_KEY")
		os.Unsetenv("ARMOR_AUTH_CRED2_ACL")
		os.Unsetenv("ARMOR_AUTH_CRED3_ACCESS_KEY")
		os.Unsetenv("ARMOR_AUTH_CRED3_SECRET_KEY")
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error when credentials have validation failures")
	}

	// Verify all three credential errors are reported in the error
	errMsg := err.Error()
	expectedErrors := []string{
		"ARMOR_AUTH_CRED1",
		"ARMOR_AUTH_CRED2",
		"ARMOR_AUTH_CRED3",
	}

	for _, expected := range expectedErrors {
		if !strings.Contains(errMsg, expected) {
			t.Errorf("Error message should mention %s, got: %v", expected, errMsg)
		}
	}
}

func actionsEqual(got map[string]bool, expected ...string) bool {
	want := make(map[string]bool, len(expected))
	for _, v := range expected {
		want[v] = true
	}
	return reflect.DeepEqual(got, want)
}

func TestLoadFailsWithoutCredentials(t *testing.T) {
	// Set minimal required env vars but NO auth credentials
	setEnv(t, minimalEnv()...)

	// Explicitly unset auth credential env vars
	os.Unsetenv("ARMOR_AUTH_ACCESS_KEY")
	os.Unsetenv("ARMOR_AUTH_SECRET_KEY")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should fail when no credentials are configured")
	}

	// Verify the error message mentions credential requirement
	errMsg := err.Error()
	if !strings.Contains(errMsg, "no client credential configured") {
		t.Errorf("Error message should mention credential requirement, got: %v", errMsg)
	}
	// Verify it lists all credential sources
	if !strings.Contains(errMsg, "ARMOR_AUTH_ACCESS_KEY") {
		t.Errorf("Error message should mention ARMOR_AUTH_ACCESS_KEY, got: %v", errMsg)
	}
	if !strings.Contains(errMsg, "ARMOR_AUTH_SECRET_KEY") {
		t.Errorf("Error message should mention ARMOR_AUTH_SECRET_KEY, got: %v", errMsg)
	}
	if !strings.Contains(errMsg, "ARMOR_AUTH_FILE") {
		t.Errorf("Error message should mention ARMOR_AUTH_FILE, got: %v", errMsg)
	}
}

func TestLoadSucceedsWithAllowNoCredentials(t *testing.T) {
	// Set minimal required env vars but NO auth credentials
	env := append(minimalEnv(), "ARMOR_ALLOW_NO_CREDENTIALS", "true")
	setEnv(t, env...)

	// Explicitly unset auth credential env vars
	os.Unsetenv("ARMOR_AUTH_ACCESS_KEY")
	os.Unsetenv("ARMOR_AUTH_SECRET_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should succeed with ARMOR_ALLOW_NO_CREDENTIALS=true: %v", err)
	}

	// Verify AllowNoCredentials is set
	if !cfg.AllowNoCredentials {
		t.Error("AllowNoCredentials should be true when ARMOR_ALLOW_NO_CREDENTIALS=true")
	}

	// Verify no credentials were loaded
	if len(cfg.Credentials) != 0 {
		t.Errorf("Credentials map should be empty, got %d credentials", len(cfg.Credentials))
	}
}

func TestLoadSucceedsWithDefaultCredentials(t *testing.T) {
	// Set minimal required env vars including default credentials
	env := append(minimalEnv(),
		"ARMOR_AUTH_ACCESS_KEY", "test-access-key",
		"ARMOR_AUTH_SECRET_KEY", "test-secret-key",
	)
	setEnv(t, env...)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify default credential was loaded
	if len(cfg.Credentials) != 1 {
		t.Fatalf("Expected 1 credential, got %d", len(cfg.Credentials))
	}

	cred, exists := cfg.Credentials["test-access-key"]
	if !exists {
		t.Fatal("Default credential not found in credentials map")
	}

	if cred.AccessKey != "test-access-key" {
		t.Errorf("AccessKey = %q, want test-access-key", cred.AccessKey)
	}

	if cred.SecretKey != "test-secret-key" {
		t.Errorf("SecretKey = %q, want test-secret-key", cred.SecretKey)
	}

	// Verify credential source is env
	if cred.Source != CredentialSourceEnv {
		t.Errorf("Source = %q, want env", cred.Source)
	}
}

func TestLoadSucceedsWithNamedCredentialsOnly(t *testing.T) {
	// Set minimal required env vars but NO default credentials
	// Only set named credentials
	env := append(minimalEnv(),
		"ARMOR_AUTH_READONLY_ACCESS_KEY", "readonly-key",
		"ARMOR_AUTH_READONLY_SECRET_KEY", "readonly-secret",
	)
	setEnv(t, env...)

	// Explicitly unset default credential env vars
	os.Unsetenv("ARMOR_AUTH_ACCESS_KEY")
	os.Unsetenv("ARMOR_AUTH_SECRET_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should succeed with named credentials: %v", err)
	}

	// Verify named credential was loaded
	if len(cfg.Credentials) != 1 {
		t.Fatalf("Expected 1 credential, got %d", len(cfg.Credentials))
	}

	cred, exists := cfg.Credentials["readonly-key"]
	if !exists {
		t.Fatal("Named credential not found in credentials map")
	}

	if cred.AccessKey != "readonly-key" {
		t.Errorf("AccessKey = %q, want readonly-key", cred.AccessKey)
	}
}

func TestRedacted(t *testing.T) {
	// Create a config with all secrets set
	testMEK := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	testB2Secret := "b2secretkey1234567890abcdefghijklmn"
	testAuthSecret := "authsecretkey1234567890abcdefghijklmn"
	testPresignSecret := "7072657369676e736563726574313233343536373839306162636465666768696a6b6c6d6e" // hex of "presignsecret1234567890abcdefghijklmn"
	testDashboardPass := "dashboardpass123"
	testDashboardToken := "admintoken1234567890abcdefghijklmn"
	testAdminToken := "admintoken1234567890abcdefghijklmn"
	testNamedKey1 := "1111111111111111111111111111111111111111111111111111111111111111"
	testNamedKey2 := "2222222222222222222222222222222222222222222222222222222222222222"
	testCredSecret := "credentialsecret1234567890abcdefghijklmn"

	setEnv(t, append(minimalEnv(),
		"ARMOR_B2_SECRET_ACCESS_KEY", testB2Secret,
		"ARMOR_MEK", testMEK,
		"ARMOR_AUTH_ACCESS_KEY", "defaultkey",
		"ARMOR_AUTH_SECRET_KEY", testAuthSecret,
		"ARMOR_PRESIGN_SECRET", testPresignSecret,
		"ARMOR_DASHBOARD_PASS", testDashboardPass,
		"ARMOR_DASHBOARD_TOKEN", testDashboardToken,
		"ARMOR_ADMIN_TOKEN", testAdminToken,
		"ARMOR_MEK_SENSITIVE", testNamedKey1,
		"ARMOR_MEK_ARCHIVE", testNamedKey2,
		"ARMOR_AUTH_APP_ACCESS_KEY", "appkey",
		"ARMOR_AUTH_APP_SECRET_KEY", testCredSecret,
		"ARMOR_AUTH_APP_ACL", "app-bucket:app-prefix/",
	)...)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Get redacted config
	rc := cfg.Redacted()

	// Verify no secret material appears in the redacted output
	output := fmt.Sprintf("%+v", rc)

	// Check that none of the secret values appear in the output
	secrets := []string{
		testMEK,
		testB2Secret,
		testAuthSecret,
		// Note: testPresignSecret is hex-encoded, so the original plaintext won't appear
		// The hex-encoded version is what's actually stored
		"presignsecret1234567890abcdefghijklmn", // Check plaintext doesn't appear
		testDashboardPass,
		testDashboardToken,
		testAdminToken,
		testNamedKey1,
		testNamedKey2,
		testCredSecret,
	}

	for _, secret := range secrets {
		if strings.Contains(output, secret) {
			t.Errorf("Secret material appears in redacted output: %s", secret)
		}
	}

	// Verify all secret fields show "<set>"
	if rc.B2SecretAccessKey != "<set>" {
		t.Errorf("B2SecretAccessKey = %q, want <set>", rc.B2SecretAccessKey)
	}
	if rc.MEK != "<set>" {
		t.Errorf("MEK = %q, want <set>", rc.MEK)
	}
	if rc.AuthSecretKey != "<set>" {
		t.Errorf("AuthSecretKey = %q, want <set>", rc.AuthSecretKey)
	}
	if rc.PresignSecret != "<set>" {
		t.Errorf("PresignSecret = %q, want <set>", rc.PresignSecret)
	}
	if rc.DashboardPass != "<set>" {
		t.Errorf("DashboardPass = %q, want <set>", rc.DashboardPass)
	}
	if rc.DashboardToken != "<set>" {
		t.Errorf("DashboardToken = %q, want <set>", rc.DashboardToken)
	}
	if rc.AdminToken != "<set>" {
		t.Errorf("AdminToken = %q, want <set>", rc.AdminToken)
	}

	// Verify named keys are redacted
	if len(rc.NamedKeys) != 2 {
		t.Errorf("NamedKeys count = %d, want 2", len(rc.NamedKeys))
	}
	for name, status := range rc.NamedKeys {
		if status != "<set>" {
			t.Errorf("NamedKeys[%s] = %q, want <set>", name, status)
		}
	}

	// Verify credentials are redacted
	if len(rc.Credentials) != 2 { // default + APP
		t.Errorf("Credentials count = %d, want 2", len(rc.Credentials))
	}
	for accessKey, redactedCred := range rc.Credentials {
		if redactedCred.SecretKey != "<set>" {
			t.Errorf("Credentials[%s].SecretKey = %q, want <set>", accessKey, redactedCred.SecretKey)
		}
		// Verify ACLs are preserved (not redacted)
		if accessKey == "appkey" && len(redactedCred.ACLs) == 0 {
			t.Error("Credentials[appkey].ACLs should be preserved")
		}
	}
}

func TestKeyRingParsing(t *testing.T) {
	tests := []struct {
		name        string
		ringStr     string
		activeMEK   string
		expectCount int
		expectError bool
	}{
		{
			name:        "empty ring",
			ringStr:     "",
			activeMEK:   "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			expectCount: 0,
		},
		{
			name:    "single retired key",
			ringStr: "1111111111111111111111111111111111111111111111111111111111111111",
			activeMEK: "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			expectCount: 1,
		},
		{
			name: "two-key ring",
			ringStr: "1111111111111111111111111111111111111111111111111111111111111111," +
				"2222222222222222222222222222222222222222222222222222222222222222",
			activeMEK:   "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			expectCount: 2,
		},
		{
			name:        "invalid hex",
			ringStr:     "invalidhex1234,2222222222222222222222222222222222222222222222222222222222222222",
			activeMEK:   "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			expectError: true,
		},
		{
			name:        "wrong length - too short",
			ringStr:     "1111,2222222222222222222222222222222222222222222222222222222222222222",
			activeMEK:   "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			expectError: true,
		},
		{
			name:        "duplicate of active key",
			ringStr:     "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			activeMEK:   "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			expectError: true,
		},
		{
			name:    "empty entry in ring",
			ringStr: "1111111111111111111111111111111111111111111111111111111111111111,,",
			activeMEK: "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			expectError: true,
		},
		{
			name: "mixed valid and invalid",
			ringStr: "1111111111111111111111111111111111111111111111111111111111111111," +
				"invalid," +
				"2222222222222222222222222222222222222222222222222222222222222222",
			activeMEK:   "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activeMEK, _ := hex.DecodeString(tt.activeMEK)
			ringMEKs, errs := parseKeyRing(tt.ringStr, "TEST_RING", activeMEK)

			if tt.expectError {
				if len(errs) == 0 {
					t.Error("Expected error, got none")
				}
				return
			}

			if len(errs) > 0 {
				t.Errorf("Unexpected errors: %v", errs)
			}

			expectBytes := tt.expectCount * 32
			if len(ringMEKs) != expectBytes {
				t.Errorf("Ring MEKs length = %d, want %d", len(ringMEKs), expectBytes)
			}
		})
	}
}

func TestLoadWithKeyRing(t *testing.T) {
	activeMEK := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	retired1 := "1111111111111111111111111111111111111111111111111111111111111111"
	retired2 := "2222222222222222222222222222222222222222222222222222222222222222"

	env := append(minimalEnv(),
		"ARMOR_MEK_RING", retired1+","+retired2,
	)
	setEnv(t, env...)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify ring was loaded
	if len(cfg.KeyRings) != 1 {
		t.Fatalf("KeyRings count = %d, want 1", len(cfg.KeyRings))
	}

	ringMEKs, exists := cfg.KeyRings["default"]
	if !exists {
		t.Fatal("Default key ring not found")
	}

	// Should have 2 retired keys (64 bytes total)
	if len(ringMEKs) != 64 {
		t.Errorf("Ring MEKs length = %d, want 64", len(ringMEKs))
	}
}

func TestLoadWithNamedKeyRing(t *testing.T) {
	activeMEK := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	namedMEK := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	retired1 := "1111111111111111111111111111111111111111111111111111111111111111"
	retired2 := "2222222222222222222222222222222222222222222222222222222222222222"

	env := append(minimalEnv(),
		"ARMOR_MEK_ARCHIVE", namedMEK,
		"ARMOR_MEK_ARCHIVE_RING", retired1+","+retired2,
	)
	setEnv(t, env...)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify named key was loaded
	if len(cfg.NamedKeys) != 1 {
		t.Fatalf("NamedKeys count = %d, want 1", len(cfg.NamedKeys))
	}

	// Verify ring was loaded
	if len(cfg.KeyRings) != 1 {
		t.Fatalf("KeyRings count = %d, want 1", len(cfg.KeyRings))
	}

	ringMEKs, exists := cfg.KeyRings["archive"]
	if !exists {
		t.Fatal("Archive key ring not found")
	}

	// Should have 2 retired keys (64 bytes total)
	if len(ringMEKs) != 64 {
		t.Errorf("Ring MEKs length = %d, want 64", len(ringMEKs))
	}
}

func TestLoadWithRingValidationErrors(t *testing.T) {
	activeMEK := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

	tests := []struct {
		name        string
		envVars     []string
		expectError bool
		errorMsg    string
	}{
		{
			name: "ring without named key",
			envVars: []string{
				"ARMOR_MEK_ARCHIVE_RING", "1111111111111111111111111111111111111111111111111111111111111111",
			},
			expectError: true,
			errorMsg:    "ARMOR_MEK_ARCHIVE_RING specified without ARMOR_MEK_ARCHIVE",
		},
		{
			name: "invalid hex in ring",
			envVars: []string{
				"ARMOR_MEK_RING", "invalidhex,2222222222222222222222222222222222222222222222222222222222222222",
			},
			expectError: true,
			errorMsg:    "invalid hex",
		},
		{
			name: "duplicate active key in ring",
			envVars: []string{
				"ARMOR_MEK_RING", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
			},
			expectError: true,
			errorMsg:    "duplicates the active key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := append(minimalEnv(), tt.envVars...)
			setEnv(t, env...)

			_, err := Load()
			if !tt.expectError {
				if err != nil {
					t.Fatalf("Load() unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("Load() should have returned error")
			}

			if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Error message should contain %q, got: %v", tt.errorMsg, err)
			}
		})
	}
}

func TestRedactedWithKeyRings(t *testing.T) {
	activeMEK := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	retired1 := "1111111111111111111111111111111111111111111111111111111111111111"
	retired2 := "2222222222222222222222222222222222222222222222222222222222222222"

	env := append(minimalEnv(),
		"ARMOR_MEK_RING", retired1+","+retired2,
	)
	setEnv(t, env...)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	rc := cfg.Redacted()

	// Verify fingerprints are present
	if len(rc.KeyRingFingerprints) != 1 {
		t.Fatalf("KeyRingFingerprints count = %d, want 1", len(rc.KeyRingFingerprints))
	}

	fingerprints, exists := rc.KeyRingFingerprints["default"]
	if !exists {
		t.Fatal("Default key ring fingerprints not found")
	}

	// Should have 2 fingerprints
	if len(fingerprints) != 2 {
		t.Errorf("Fingerprints count = %d, want 2", len(fingerprints))
	}

	// Verify each fingerprint is 16 hex characters
	for _, fp := range fingerprints {
		if len(fp) != 16 {
			t.Errorf("Fingerprint length = %d, want 16", len(fp))
		}
		// Verify it's valid hex
		_, err := hex.DecodeString(fp)
		if err != nil {
			t.Errorf("Fingerprint should be valid hex: %v", err)
		}
	}

	// Verify actual MEKs don't appear in redacted output
	output := fmt.Sprintf("%+v", rc)
	if strings.Contains(output, retired1) || strings.Contains(output, retired2) {
		t.Error("Ring MEKs should not appear in redacted output")
	}
}

func TestFormatWriteVersion(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectVersion int
		expectError   bool
		errorContains string
	}{
		{
			name:          "unset defaults to 2",
			envValue:      "",
			expectVersion: 2,
			expectError:   false,
		},
		{
			name:          "explicit 2",
			envValue:      "2",
			expectVersion: 2,
			expectError:   false,
		},
		{
			name:          "explicit 3",
			envValue:      "3",
			expectVersion: 3,
			expectError:   false,
		},
		{
			name:          "invalid value 1",
			envValue:      "1",
			expectError:   true,
			errorContains: "ARMOR_FORMAT_VERSION must be 2 or 3, got 1",
		},
		{
			name:          "invalid value 4",
			envValue:      "4",
			expectError:   true,
			errorContains: "ARMOR_FORMAT_VERSION must be 2 or 3, got 4",
		},
		{
			name:          "invalid value 0",
			envValue:      "0",
			expectError:   true,
			errorContains: "ARMOR_FORMAT_VERSION must be 2 or 3, got 0",
		},
		{
			name:          "invalid non-numeric",
			envValue:      "abc",
			expectError:   true,
			errorContains: "ARMOR_FORMAT_VERSION must be an integer (2 or 3), got \"abc\"",
		},
		{
			name:          "invalid negative",
			envValue:      "-1",
			expectError:   true,
			errorContains: "ARMOR_FORMAT_VERSION must be 2 or 3, got -1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			env := minimalEnv()
			if tt.envValue != "" {
				env = append(env, "ARMOR_FORMAT_VERSION", tt.envValue)
			}
			setEnv(t, env...)

			cfg, err := Load()

			if tt.expectError {
				if err == nil {
					t.Errorf("Load() should return error for ARMOR_FORMAT_VERSION=%q", tt.envValue)
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Load() unexpected error: %v", err)
				}
				if cfg.FormatWriteVersion != tt.expectVersion {
					t.Errorf("FormatWriteVersion = %d, want %d", cfg.FormatWriteVersion, tt.expectVersion)
				}
			}
		})
	}
}

func TestFormatWriteVersionInRedacted(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectVersion int
	}{
		{
			name:          "default version 2",
			envValue:      "",
			expectVersion: 2,
		},
		{
			name:          "explicit version 2",
			envValue:      "2",
			expectVersion: 2,
		},
		{
			name:          "explicit version 3",
			envValue:      "3",
			expectVersion: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			env := minimalEnv()
			if tt.envValue != "" {
				env = append(env, "ARMOR_FORMAT_VERSION", tt.envValue)
			}
			setEnv(t, env...)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}

			rc := cfg.Redacted()
			if rc.FormatWriteVersion != tt.expectVersion {
				t.Errorf("Redacted FormatWriteVersion = %d, want %d", rc.FormatWriteVersion, tt.expectVersion)
			}
		})
	}
}

func TestPresignDisabledByDefault(t *testing.T) {
	// Set minimal required environment variables
	setenv(t, "ARMOR_BACKEND", "filesystem")
	setenv(t, "ARMOR_FS_PATH", t.TempDir())
	setenv(t, "ARMOR_BUCKET", "test-bucket")
	setenv(t, "ARMOR_MEK", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	setenv(t, "ARMOR_AUTH_ACCESS_KEY", "test-key")
	setenv(t, "ARMOR_AUTH_SECRET_KEY", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.PresignEnabled {
		t.Error("PresignEnabled should be false by default")
	}
	if len(cfg.PresignSecret) > 0 {
		t.Error("PresignSecret should be empty when disabled")
	}
	if cfg.PresignBaseURL != "" {
		t.Errorf("PresignBaseURL should be empty when disabled, got '%s'", cfg.PresignBaseURL)
	}
}

func TestPresignEnabledWithValidConfig(t *testing.T) {
	// Set minimal required environment variables
	setenv(t, "ARMOR_BACKEND", "filesystem")
	setenv(t, "ARMOR_FS_PATH", t.TempDir())
	setenv(t, "ARMOR_BUCKET", "test-bucket")
	setenv(t, "ARMOR_MEK", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	setenv(t, "ARMOR_AUTH_ACCESS_KEY", "test-key")
	setenv(t, "ARMOR_AUTH_SECRET_KEY", "test-secret")

	// Enable presign with valid configuration
	secretHex := hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef")) // 32 bytes
	setenv(t, "ARMOR_PRESIGN_ENABLED", "true")
	setenv(t, "ARMOR_PRESIGN_SECRET", secretHex)
	setenv(t, "ARMOR_PRESIGN_BASE_URL", "https://armor.example.com/share")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if !cfg.PresignEnabled {
		t.Error("PresignEnabled should be true when ARMOR_PRESIGN_ENABLED=true")
	}
	if len(cfg.PresignSecret) != 32 {
		t.Errorf("PresignSecret should be 32 bytes, got %d", len(cfg.PresignSecret))
	}
	if cfg.PresignBaseURL != "https://armor.example.com/share" {
		t.Errorf("PresignBaseURL = '%s', want 'https://armor.example.com/share'", cfg.PresignBaseURL)
	}
}

func TestPresignEnabledWithoutSecret(t *testing.T) {
	// Set minimal required environment variables
	setenv(t, "ARMOR_BACKEND", "filesystem")
	setenv(t, "ARMOR_FS_PATH", t.TempDir())
	setenv(t, "ARMOR_BUCKET", "test-bucket")
	setenv(t, "ARMOR_MEK", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	setenv(t, "ARMOR_AUTH_ACCESS_KEY", "test-key")
	setenv(t, "ARMOR_AUTH_SECRET_KEY", "test-secret")

	// Enable presign without secret
	setenv(t, "ARMOR_PRESIGN_ENABLED", "true")
	setenv(t, "ARMOR_PRESIGN_BASE_URL", "https://armor.example.com/share")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should error when ARMOR_PRESIGN_SECRET is missing")
	}
	if !strings.Contains(err.Error(), "ARMOR_PRESIGN_SECRET is required") {
		t.Errorf("Error should mention ARMOR_PRESIGN_SECRET is required, got: %v", err)
	}
}

func TestPresignEnabledWithoutBaseURL(t *testing.T) {
	// Set minimal required environment variables
	setenv(t, "ARMOR_BACKEND", "filesystem")
	setenv(t, "ARMOR_FS_PATH", t.TempDir())
	setenv(t, "ARMOR_BUCKET", "test-bucket")
	setenv(t, "ARMOR_MEK", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	setenv(t, "ARMOR_AUTH_ACCESS_KEY", "test-key")
	setenv(t, "ARMOR_AUTH_SECRET_KEY", "test-secret")

	// Enable presign without base URL
	secretHex := hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef")) // 32 bytes
	setenv(t, "ARMOR_PRESIGN_ENABLED", "true")
	setenv(t, "ARMOR_PRESIGN_SECRET", secretHex)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should error when ARMOR_PRESIGN_BASE_URL is missing")
	}
	if !strings.Contains(err.Error(), "ARMOR_PRESIGN_BASE_URL is required") {
		t.Errorf("Error should mention ARMOR_PRESIGN_BASE_URL is required, got: %v", err)
	}
}

func TestPresignEnabledWithRelativeBaseURL(t *testing.T) {
	// Set minimal required environment variables
	setenv(t, "ARMOR_BACKEND", "filesystem")
	setenv(t, "ARMOR_FS_PATH", t.TempDir())
	setenv(t, "ARMOR_BUCKET", "test-bucket")
	setenv(t, "ARMOR_MEK", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	setenv(t, "ARMOR_AUTH_ACCESS_KEY", "test-key")
	setenv(t, "ARMOR_AUTH_SECRET_KEY", "test-secret")

	// Enable presign with relative base URL
	secretHex := hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef")) // 32 bytes
	setenv(t, "ARMOR_PRESIGN_ENABLED", "true")
	setenv(t, "ARMOR_PRESIGN_SECRET", secretHex)
	setenv(t, "ARMOR_PRESIGN_BASE_URL", "/share")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should error when ARMOR_PRESIGN_BASE_URL is relative")
	}
	if !strings.Contains(err.Error(), "must be an absolute URL") {
		t.Errorf("Error should mention absolute URL requirement, got: %v", err)
	}
}

func TestPresignDisabledFieldsAreEmpty(t *testing.T) {
	// Set minimal required environment variables
	setenv(t, "ARMOR_BACKEND", "filesystem")
	setenv(t, "ARMOR_FS_PATH", t.TempDir())
	setenv(t, "ARMOR_BUCKET", "test-bucket")
	setenv(t, "ARMOR_MEK", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	setenv(t, "ARMOR_AUTH_ACCESS_KEY", "test-key")
	setenv(t, "ARMOR_AUTH_SECRET_KEY", "test-secret")

	// Explicitly disable presign
	setenv(t, "ARMOR_PRESIGN_ENABLED", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.PresignEnabled {
		t.Error("PresignEnabled should be false when ARMOR_PRESIGN_ENABLED=false")
	}
	if len(cfg.PresignSecret) > 0 {
		t.Error("PresignSecret should be empty when disabled")
	}
	if cfg.PresignBaseURL != "" {
		t.Errorf("PresignBaseURL should be empty when disabled, got '%s'", cfg.PresignBaseURL)
	}
}

func TestPresignRedactedConfig(t *testing.T) {
	// Set minimal required environment variables
	setenv(t, "ARMOR_BACKEND", "filesystem")
	setenv(t, "ARMOR_FS_PATH", t.TempDir())
	setenv(t, "ARMOR_BUCKET", "test-bucket")
	setenv(t, "ARMOR_MEK", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	setenv(t, "ARMOR_AUTH_ACCESS_KEY", "test-key")
	setenv(t, "ARMOR_AUTH_SECRET_KEY", "test-secret")

	// Test disabled state
	setenv(t, "ARMOR_PRESIGN_ENABLED", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	rc := cfg.Redacted()
	if rc.PresignEnabled {
		t.Error("Redacted PresignEnabled should be false")
	}

	// Test enabled state
	secretHex := hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	setenv(t, "ARMOR_PRESIGN_ENABLED", "true")
	setenv(t, "ARMOR_PRESIGN_SECRET", secretHex)
	setenv(t, "ARMOR_PRESIGN_BASE_URL", "https://armor.example.com/share")

	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	rc = cfg.Redacted()
	if !rc.PresignEnabled {
		t.Error("Redacted PresignEnabled should be true")
	}
	if rc.PresignSecret != "<set>" {
		t.Errorf("Redacted PresignSecret = '%s', want '<set>'", rc.PresignSecret)
	}
	if rc.PresignBaseURL != "https://armor.example.com/share" {
		t.Errorf("Redacted PresignBaseURL = '%s', want 'https://armor.example.com/share'", rc.PresignBaseURL)
	}
}
