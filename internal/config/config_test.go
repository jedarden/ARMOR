package config

import (
	"os"
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
            name:        "invalid format - missing colon",
            aclStr:      "bucket-only",
            expectError: true,
        },
        {
            name:        "invalid format - empty bucket",
            aclStr:      ":prefix/",
            expectError: true,
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
                return routes[0].Prefix == "data/pii/*" && routes[0].KeyName == "sensitive" &&
                    routes[1].Prefix == "archive/*" && routes[1].KeyName == "archive" &&
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
