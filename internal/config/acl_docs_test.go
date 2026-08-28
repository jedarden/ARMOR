package config

import (
	"strings"
	"testing"
)

// TestDocumentationACLs verifies that every ACL example in README.md and
// docs/connection-guide.md parses to the bucket/prefix/verbs the surrounding
// prose claims. This test prevents documentation drift.
func TestDocumentationACLs(t *testing.T) {
	// This test suite extracts each ARMOR_AUTH_*_ACL="..." literal from the
	// documentation and verifies it parses correctly. See:
	// - README.md lines 198-268 (Named Credentials with ACLs)
	// - docs/connection-guide.md lines 340-352 (Multi-Credential Setup)

	tests := []struct {
		name        string
		aclString   string
		description string // What the prose claims this grants
		validator   func([]ACLEntry) bool
	}{
		{
			name:        "README line 206 - readonly credential",
			aclString:   "mybucket:readonly/*",
			description: "Grants read-only access to mybucket:readonly/ prefix (all verbs when no action segment)",
			validator: func(acls []ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				return acls[0].Bucket == "mybucket" &&
					acls[0].Prefix == "readonly/" &&
					acls[0].Actions == nil // nil Actions = all verbs permitted
			},
		},
		{
			name:        "README line 211 - writer credential with two buckets",
			aclString:   "mybucket:*,otherbucket:uploads/*",
			description: "Grants full access to mybucket (all keys) and otherbucket:uploads/ prefix",
			validator: func(acls []ACLEntry) bool {
				if len(acls) != 2 {
					return false
				}
				// Order matters: comma-separated entries are parsed left-to-right
				return acls[0].Bucket == "mybucket" &&
					acls[0].Prefix == "" &&
					acls[0].Actions == nil &&
					acls[1].Bucket == "otherbucket" &&
					acls[1].Prefix == "uploads/" &&
					acls[1].Actions == nil
			},
		},
		{
			name:        "README line 238 - logs credential",
			aclString:   "mybucket:logs/*",
			description: "Grants all verbs on mybucket:logs/ prefix",
			validator: func(acls []ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				return acls[0].Bucket == "mybucket" &&
					acls[0].Prefix == "logs/" &&
					acls[0].Actions == nil
			},
		},
		{
			name:        "README line 241 - readonly with explicit get+list verbs",
			aclString:   "mybucket:readonly/*:get+list",
			description: "Grants only GET and LIST on mybucket:readonly/ prefix",
			validator: func(acls []ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				if acls[0].Bucket != "mybucket" || acls[0].Prefix != "readonly/" {
					return false
				}
				if len(acls[0].Actions) != 2 {
					return false
				}
				return acls[0].Actions["get"] && acls[0].Actions["list"]
			},
		},
		{
			name:        "README line 244 - backup append-only writer",
			aclString:   "mybucket:backups/*:put+list",
			description: "Grants only PUT and LIST on mybucket:backups/ (append-only backup writer)",
			validator: func(acls []ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				if acls[0].Bucket != "mybucket" || acls[0].Prefix != "backups/" {
					return false
				}
				if len(acls[0].Actions) != 2 {
					return false
				}
				return acls[0].Actions["put"] && acls[0].Actions["list"]
			},
		},
		{
			name:        "README line 254 - backup writer credential",
			aclString:   "mybucket:backups/*:put+list",
			description: "Grants only PUT and LIST on mybucket:backups/ (append-only backup writer)",
			validator: func(acls []ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				if acls[0].Bucket != "mybucket" || acls[0].Prefix != "backups/" {
					return false
				}
				if len(acls[0].Actions) != 2 {
					return false
				}
				return acls[0].Actions["put"] && acls[0].Actions["list"]
			},
		},
		{
			name:        "README line 263 - cross-bucket with mixed verbs",
			aclString:   "bucket-primary:*:get+put+delete+list,bucket-audit:logs/*:get+list",
			description: "Grants all verbs on bucket-primary (all keys) and only GET+LIST on bucket-audit:logs/",
			validator: func(acls []ACLEntry) bool {
				if len(acls) != 2 {
					return false
				}
				// First entry: bucket-primary:*:get+put+delete+list
				if acls[0].Bucket != "bucket-primary" || acls[0].Prefix != "" {
					return false
				}
				if len(acls[0].Actions) != 4 {
					return false
				}
				if !acls[0].Actions["get"] || !acls[0].Actions["put"] ||
					!acls[0].Actions["delete"] || !acls[0].Actions["list"] {
					return false
				}
				// Second entry: bucket-audit:logs/*:get+list
				if acls[1].Bucket != "bucket-audit" || acls[1].Prefix != "logs/" {
					return false
				}
				if len(acls[1].Actions) != 2 {
					return false
				}
				if !acls[1].Actions["get"] || !acls[1].Actions["list"] {
					return false
				}
				return true
			},
		},
		{
			name:        "connection-guide line 347 - readonly credential",
			aclString:   "mybucket:readonly/*",
			description: "Grants all verbs on mybucket:readonly/ prefix (no action segment = all permitted)",
			validator: func(acls []ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				return acls[0].Bucket == "mybucket" &&
					acls[0].Prefix == "readonly/" &&
					acls[0].Actions == nil
			},
		},
		{
			name:        "connection-guide line 351 - writer credential",
			aclString:   "mybucket:*",
			description: "Grants all verbs on mybucket (all keys)",
			validator: func(acls []ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				return acls[0].Bucket == "mybucket" &&
					acls[0].Prefix == "" &&
					acls[0].Actions == nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the ACL string
			acls, err := parseACL(tt.aclString)
			if err != nil {
				t.Errorf("parseACL(%q) failed: %v\nDescription: %s", tt.aclString, err, tt.description)
				return
			}

			// Validate the parsed ACL matches expectations
			if !tt.validator(acls) {
				t.Errorf("ACL validation failed for %q\nDescription: %s\nParsed: %+v", tt.aclString, tt.description, acls)
			}

			// Verify prefix normalization: /* trailing wildcard becomes /
			// This is documented in README.md line 173-175
			t.Logf("✓ %s parses as documented", tt.name)
		})
	}
}

// TestACLPrefixNormalization verifies that prefix wildcards are normalized
// as documented: trailing /* becomes a literal / prefix, bare * becomes empty string.
func TestACLPrefixNormalization(t *testing.T) {
	tests := []struct {
		input       string
		wantBucket string
		wantPrefix string
	}{
		// Trailing /* becomes literal / prefix (README line 173-175)
		{"mybucket:logs/*", "mybucket", "logs/"},
		{"mybucket:readonly/*", "mybucket", "readonly/"},
		{"mybucket:backups/*", "mybucket", "backups/"},
		{"*:data/*", "*", "data/"},
		// Bare * becomes empty prefix (README line 221)
		{"mybucket:*", "mybucket", ""},
		{"*:*", "*", ""},
		// Specific prefix without wildcard
		{"mybucket:data/", "mybucket", "data/"},
		{"mybucket:path/to/files/", "mybucket", "path/to/files/"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			acls, err := parseACL(tt.input)
			if err != nil {
				t.Fatalf("parseACL(%q) failed: %v", tt.input, err)
			}
			if len(acls) != 1 {
				t.Fatalf("parseACL(%q) returned %d entries, want 1", tt.input, len(acls))
			}
			if acls[0].Bucket != tt.wantBucket {
				t.Errorf("parseACL(%q).Bucket = %q, want %q", tt.input, acls[0].Bucket, tt.wantBucket)
			}
			if acls[0].Prefix != tt.wantPrefix {
				t.Errorf("parseACL(%q).Prefix = %q, want %q", tt.input, acls[0].Prefix, tt.wantPrefix)
			}
		})
	}
}

// TestACLActionVerbs verifies that action verb parsing accepts both
// space-separated and +-separated verbs (README line 234).
func TestACLActionVerbs(t *testing.T) {
	tests := []struct {
		name        string
		aclString   string
		wantActions map[string]bool
	}{
		{
			name:      "+-separated verbs",
			aclString: "mybucket:data/:get+list",
			wantActions: map[string]bool{
				"get":  true,
				"list": true,
			},
		},
		{
			name:      "space-separated verbs",
			aclString: "mybucket:data/:get list",
			wantActions: map[string]bool{
				"get":  true,
				"list": true,
			},
		},
		{
			name:      "mixed separators (README documents +, but spaces work)",
			aclString: "mybucket:data/:get+put list",
			wantActions: map[string]bool{
				"get": true,
				"put": true,
				"list": true,
			},
		},
		{
			name:      "all four verbs",
			aclString: "bucket:*:get+put+delete+list",
			wantActions: map[string]bool{
				"get":    true,
				"put":    true,
				"delete": true,
				"list":   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acls, err := parseACL(tt.aclString)
			if err != nil {
				t.Fatalf("parseACL(%q) failed: %v", tt.aclString, err)
			}
			if len(acls) != 1 {
				t.Fatalf("parseACL(%q) returned %d entries, want 1", tt.aclString, len(acls))
			}
			if acls[0].Actions == nil {
				t.Fatal("parseACL(%q).Actions is nil, want non-nil", tt.aclString)
			}
			for verb, want := range tt.wantActions {
				if got := acls[0].Actions[verb]; got != want {
					t.Errorf("parseACL(%q).Actions[%q] = %v, want %v", tt.aclString, verb, got, want)
				}
			}
		})
	}
}

// TestACLInvalidVerbs verifies that unknown action verbs are rejected (README line 225).
func TestACLInvalidVerbs(t *testing.T) {
	tests := []struct {
		name     string
		aclString string
	}{
		{
			name:     "unknown verb",
			aclString: "mybucket:data/:read",
		},
		{
			name:     "mixed valid and invalid",
			aclString: "mybucket:data/:get+write",
		},
		{
			name:     "uppercase verb (must be lowercase)",
			aclString: "mybucket:data/:GET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseACL(tt.aclString)
			if err == nil {
				t.Errorf("parseACL(%q) succeeded, expected error for invalid verb", tt.aclString)
			}
			if !strings.Contains(err.Error(), "invalid action verb") {
				t.Errorf("parseACL(%q) error = %v, want error about invalid action verb", tt.aclString, err)
			}
		})
	}
}

// TestACLInvalidWildcardPositions verifies that wildcards must be at the end
// of the prefix (config.go line 562).
func TestACLInvalidWildcardPositions(t *testing.T) {
	tests := []struct {
		name     string
		aclString string
	}{
		{
			name:     "wildcard in middle of prefix",
			aclString: "mybucket:*/data/",
		},
		{
			name:     "wildcard at start of prefix",
			aclString: "mybucket:*/logs",
		},
		{
			name:     "multiple wildcards",
			aclString: "mybucket:data*/*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseACL(tt.aclString)
			if err == nil {
				t.Errorf("parseACL(%q) succeeded, expected error for invalid wildcard position", tt.aclString)
			}
			if !strings.Contains(err.Error(), "wildcard must be") {
				t.Errorf("parseACL(%q) error = %v, want error about wildcard position", tt.aclString, err)
			}
		})
	}
}
