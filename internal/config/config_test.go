package config

import (
	"os"
	"reflect"
	"testing"
)

func TestParseACL(t *testing.T) {
	tests := []struct {
		name        string
		aclStr      string
		expectCount int
		expectError bool
		checkFunc   func([]ACLEntry) bool
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
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == ""
			},
		},
		{
			name:        "single bucket with specific prefix",
			aclStr:      "my-bucket:data/",
			expectCount: 1,
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == "data/"
			},
		},
		{
			name:        "multiple entries",
			aclStr:      "bucket-a:prefix-a/,bucket-b:prefix-b/",
			expectCount: 2,
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "bucket-a" && acls[0].Prefix == "prefix-a/" &&
					acls[1].Bucket == "bucket-b" && acls[1].Prefix == "prefix-b/"
			},
		},
		{
			name:        "wildcard bucket",
			aclStr:      "*:public/",
			expectCount: 1,
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "*" && acls[0].Prefix == "public/"
			},
		},
		{
			// Backward compatibility: a two-segment entry specifies no verbs,
			// so Actions must be nil (which reads as "all verbs permitted").
			name:        "two-segment defaults to all actions (nil map)",
			aclStr:      "my-bucket:data/",
			expectCount: 1,
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == "data/" && acls[0].Actions == nil
			},
		},
		{
			name:        "three-segment single verb",
			aclStr:      "mybucket:foo/:get",
			expectCount: 1,
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "mybucket" && acls[0].Prefix == "foo/" &&
					actionsEqual(acls[0].Actions, "get")
			},
		},
		{
			name:        "three-segment multiple verbs plus-separated",
			aclStr:      "mybucket:foo/:get+list",
			expectCount: 1,
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "mybucket" && acls[0].Prefix == "foo/" &&
					actionsEqual(acls[0].Actions, "get", "list")
			},
		},
		{
			name:        "three-segment multiple verbs space-separated",
			aclStr:      "mybucket:foo/:put delete",
			expectCount: 1,
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "mybucket" && acls[0].Prefix == "foo/" &&
					actionsEqual(acls[0].Actions, "put", "delete")
			},
		},
		{
			name:        "three-segment mixed separators and whitespace",
			aclStr:      "mybucket:foo/:get + list",
			expectCount: 1,
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "mybucket" && acls[0].Prefix == "foo/" &&
					actionsEqual(acls[0].Actions, "get", "list")
			},
		},
		{
			name:        "three-segment all four verbs",
			aclStr:      "bucket:/:get+put+delete+list",
			expectCount: 1,
			checkFunc: func(acls []ACLEntry) bool {
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
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "bucket" && acls[0].Prefix == "prefix/" && acls[0].Actions == nil
			},
		},
		{
			// Verb scoping is per-entry: only the scoped entry is restricted.
			name:        "mixed scoped and unscoped entries",
			aclStr:      "bucket-a:data/:get+list,bucket-b:other/",
			expectCount: 2,
			checkFunc: func(acls []ACLEntry) bool {
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
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == "data/"
			},
		},
		{
			name:        "trailing wildcard with actions",
			aclStr:      "my-bucket:data/*:get+list",
			expectCount: 1,
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == "data/" &&
					actionsEqual(acls[0].Actions, "get", "list")
			},
		},
		{
			name:        "explicit prefix with trailing slash unchanged",
			aclStr:      "my-bucket:data/",
			expectCount: 1,
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "my-bucket" && acls[0].Prefix == "data/"
			},
		},
		{
			name:        "bare wildcard normalized to empty prefix",
			aclStr:      "my-bucket:*",
			expectCount: 1,
			checkFunc: func(acls []ACLEntry) bool {
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
			checkFunc: func(acls []ACLEntry) bool {
				return acls[0].Bucket == "*" && acls[0].Prefix == "data/"
			},
		},
		{
			name:        "mixed wildcard and explicit prefix entries",
			aclStr:      "bucket:*:get,bucket:data/*:list,bucket:specific/",
			expectCount: 3,
			checkFunc: func(acls []ACLEntry) bool {
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

// actionsEqual reports whether got matches the expected set of verbs (a nil
// got is treated as the empty set). A nil expected asserts the set is empty.
func actionsEqual(got map[string]bool, expected ...string) bool {
	want := make(map[string]bool, len(expected))
	for _, v := range expected {
		want[v] = true
	}
	return reflect.DeepEqual(got, want)
}
