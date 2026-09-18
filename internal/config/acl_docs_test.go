package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/acl"
)

// docExample is one credential ACL example extracted from a documentation
// file, in document order.
type docExample struct {
	file   string // base name of the file the example came from
	line   int    // 1-based line of the example (for failure messages)
	name   string // credential name the example is attributed to
	acl    string // ACL literal as written in the doc
	hasACL bool   // false when the entry deliberately shows no acl field
}

var (
	// envACL matches ARMOR_AUTH_<NAME>_ACL="..." example lines.
	envACL = regexp.MustCompile(`ARMOR_AUTH_([A-Z0-9]+)_ACL\s*=\s*"([^"]*)"`)
	// yamlName matches the "- name: X" list items of the credentials YAML example.
	yamlName = regexp.MustCompile(`^\s*-\s+name:\s*(\S+)\s*$`)
	// yamlACL matches the acl field of a YAML credentials entry.
	yamlACL = regexp.MustCompile(`^\s*acl:\s*"([^"]*)"\s*$`)
	// fence matches a markdown fence opener/closer and captures its language.
	fence = regexp.MustCompile("^(```|~~~)\\s*(\\S*)")
)

// extractACLExamples scans a documentation file for credential ACL examples:
// ARMOR_AUTH_<NAME>_ACL="..." literals anywhere in the file, and name/acl
// pairs inside yaml fences. A YAML entry that deliberately shows no acl field
// (documenting the empty-ACL case) is returned with hasACL=false. Examples
// are returned in document order so a name appearing in two sections can be
// selected by occurrence.
func extractACLExamples(t *testing.T, path string) []docExample {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	base := filepath.Base(path)
	var found []docExample
	// pending tracks the current YAML entry that has not shown an acl yet.
	pending, pendingLine, pendingDone := "", 0, false
	flushPending := func() {
		if pending != "" && !pendingDone {
			found = append(found, docExample{file: base, line: pendingLine, name: pending})
		}
		pending, pendingLine, pendingDone = "", 0, false
	}
	inFence, lang := false, ""
	for i, line := range strings.Split(string(data), "\n") {
		if m := fence.FindStringSubmatch(line); m != nil {
			if !inFence {
				inFence, lang = true, m[2]
			} else {
				flushPending()
				inFence, lang = false, ""
			}
			continue
		}
		if lang == "yaml" {
			if m := yamlName.FindStringSubmatch(line); m != nil {
				flushPending()
				pending, pendingLine = m[1], i+1
				continue
			}
			if m := yamlACL.FindStringSubmatch(line); m != nil && pending != "" {
				found = append(found, docExample{file: base, line: i + 1, name: pending, acl: m[1], hasACL: true})
				pendingDone = true
			}
			continue
		}
		// Outside yaml fences (prose, bash blocks), match env-style literals.
		if m := envACL.FindStringSubmatch(line); m != nil {
			found = append(found, docExample{file: base, line: i + 1, name: m[1], acl: m[2], hasACL: true})
		}
	}
	flushPending()
	return found
}

// docsDir returns the repository's docs directory, found by walking up from
// the working directory to the go.mod.
func docsDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "docs")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the config package working directory")
		}
		dir = parent
	}
}

// TestDocumentationACLs verifies that every ACL example in
// docs/authentication.md and docs/connection-guide.md parses to the
// bucket/prefix/verbs the surrounding prose claims. The examples are
// extracted from the documentation files themselves, so editing an example
// without keeping it parseable as documented fails this test.
func TestDocumentationACLs(t *testing.T) {
	docs := docsDir(t)
	examples := map[string][]docExample{}
	for _, f := range []string{"authentication.md", "connection-guide.md"} {
		examples[f] = extractACLExamples(t, filepath.Join(docs, f))
	}

	// lookup returns the occ-th example attributed to cred in file.
	lookup := func(t *testing.T, file, cred string, occ int) docExample {
		t.Helper()
		n := 0
		for _, ex := range examples[file] {
			if ex.name == cred {
				if n++; n == occ {
					return ex
				}
			}
		}
		t.Errorf("no example #%d for credential %s in %s (found: %s)",
			occ, cred, file, summarize(examples[file]))
		return docExample{}
	}

	tests := []struct {
		name        string
		example     docExample
		description string // What the prose claims this grants
		expectNoACL bool   // true for the entry documenting the empty-ACL case
		validator   func([]acl.ACLEntry) bool
	}{
		{
			// The named-credentials example: no action segment means all verbs.
			name:        "authentication.md READONLY (named credentials)",
			example:     lookup(t, "authentication.md", "READONLY", 1),
			description: "Grants read-only access to mybucket:readonly/ prefix (all verbs when no action segment)",
			validator: func(acls []acl.ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				return acls[0].Bucket == "mybucket" &&
					acls[0].Prefix == "readonly/" &&
					acls[0].Actions == nil // nil Actions = all verbs permitted
			},
		},
		{
			name:        "authentication.md WRITER (named credentials)",
			example:     lookup(t, "authentication.md", "WRITER", 1),
			description: "Grants full access to mybucket (all keys) and otherbucket:uploads/ prefix",
			validator: func(acls []acl.ACLEntry) bool {
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
			name:        "authentication.md LOGS",
			example:     lookup(t, "authentication.md", "LOGS", 1),
			description: "Grants all verbs on mybucket:logs/ prefix",
			validator: func(acls []acl.ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				return acls[0].Bucket == "mybucket" &&
					acls[0].Prefix == "logs/" &&
					acls[0].Actions == nil
			},
		},
		{
			name:        "authentication.md READONLY (action verbs)",
			example:     lookup(t, "authentication.md", "READONLY", 2),
			description: "Grants only GET and LIST on mybucket:readonly/ prefix",
			validator: func(acls []acl.ACLEntry) bool {
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
			// The append-only backup writer from the prose above the example.
			name:        "authentication.md BACKUP",
			example:     lookup(t, "authentication.md", "BACKUP", 1),
			description: "Grants only PUT and LIST on mybucket:backups/ (append-only backup writer)",
			validator: func(acls []acl.ACLEntry) bool {
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
			// ADR-012 amendment (2026-09-13): the writer-with-abort profile
			// from the action-verbs section.
			name:        "authentication.md RAW",
			example:     lookup(t, "authentication.md", "RAW", 1),
			description: "Grants PUT, LIST and ABORT on mybucket:raw/ (multipart-write and abort cleanup, no delete)",
			validator: func(acls []acl.ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				if acls[0].Bucket != "mybucket" || acls[0].Prefix != "raw/" {
					return false
				}
				if len(acls[0].Actions) != 3 {
					return false
				}
				return acls[0].Actions["put"] && acls[0].Actions["list"] && acls[0].Actions["abort"]
			},
		},
		{
			name:        "authentication.md CROSSBUCKET",
			example:     lookup(t, "authentication.md", "CROSSBUCKET", 1),
			description: "Grants all verbs on bucket-primary (all keys) and only GET+LIST on bucket-audit:logs/",
			validator: func(acls []acl.ACLEntry) bool {
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
			// YAML credentials-file examples: acl comes from the entry's acl
			// field rather than an environment triplet.
			name:        "authentication.md FORGEJO_BACKUP (YAML file)",
			example:     lookup(t, "authentication.md", "FORGEJO_BACKUP", 1),
			description: "Grants only PUT and LIST on iad-ci:forgejo-backup/",
			validator: func(acls []acl.ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				if acls[0].Bucket != "iad-ci" || acls[0].Prefix != "forgejo-backup/" {
					return false
				}
				if len(acls[0].Actions) != 2 {
					return false
				}
				return acls[0].Actions["put"] && acls[0].Actions["list"]
			},
		},
		{
			name:        "authentication.md READONLY_USER (YAML file)",
			example:     lookup(t, "authentication.md", "READONLY_USER", 1),
			description: "Grants only GET and LIST on mybucket:readonly/",
			validator: func(acls []acl.ACLEntry) bool {
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
			// The empty-ACL case: FULL_ACCESS deliberately shows no acl field.
			// If someone adds an acl to this entry, the documented example of
			// "no ACL means full access" is gone and this test fails.
			name:        "authentication.md FULL_ACCESS (YAML file, empty ACL)",
			example:     lookup(t, "authentication.md", "FULL_ACCESS", 1),
			description: "Documents the empty-ACL case: the entry shows no acl field",
			expectNoACL: true,
		},
		{
			name:        "connection-guide.md READONLY",
			example:     lookup(t, "connection-guide.md", "READONLY", 1),
			description: "Grants all verbs on mybucket:readonly/ prefix (no action segment = all permitted)",
			validator: func(acls []acl.ACLEntry) bool {
				if len(acls) != 1 {
					return false
				}
				return acls[0].Bucket == "mybucket" &&
					acls[0].Prefix == "readonly/" &&
					acls[0].Actions == nil
			},
		},
		{
			name:        "connection-guide.md WRITER",
			example:     lookup(t, "connection-guide.md", "WRITER", 1),
			description: "Grants all verbs on mybucket (all keys)",
			validator: func(acls []acl.ACLEntry) bool {
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
			ex := tt.example
			if ex.file == "" {
				t.Fatal("example not found in the documentation (see earlier error)")
			}
			if tt.expectNoACL {
				// The empty-ACL case must stay empty: nothing to parse, and an
				// acl field appearing here means the documented example of
				// "no ACL means full access" is gone.
				if ex.hasACL {
					t.Errorf("%s:%d: FULL_ACCESS gained an acl field (%q); the doc's empty-ACL example is no longer documented", ex.file, ex.line, ex.acl)
				} else {
					t.Logf("✓ %s:%d FULL_ACCESS still documents the empty-ACL case", ex.file, ex.line)
				}
				return
			}
			if !ex.hasACL {
				t.Fatalf("%s example at %s:%d unexpectedly shows no ACL", ex.name, ex.file, ex.line)
			}

			// Parse the ACL string exactly as written in the doc
			acls, err := parseACL(ex.acl)
			if err != nil {
				t.Errorf("parseACL(%q) from %s:%d failed: %v\nDescription: %s", ex.acl, ex.file, ex.line, err, tt.description)
				return
			}

			// Validate the parsed ACL matches expectations
			if !tt.validator(acls) {
				t.Errorf("ACL validation failed for %q (%s:%d)\nDescription: %s\nParsed: %+v", ex.acl, ex.file, ex.line, tt.description, acls)
			}

			t.Logf("✓ %s:%d %s parses as documented", ex.file, ex.line, ex.name)
		})
	}
}

// summarize renders an extraction result list for failure messages.
func summarize(examples []docExample) string {
	if len(examples) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(examples))
	for _, ex := range examples {
		parts = append(parts, ex.name)
	}
	return strings.Join(parts, ", ")
}

// TestACLPrefixNormalization verifies that prefix wildcards are normalized
// as documented in docs/authentication.md (ACL format): trailing /* becomes a
// literal / prefix, bare * becomes empty string.
func TestACLPrefixNormalization(t *testing.T) {
	tests := []struct {
		input      string
		wantBucket string
		wantPrefix string
	}{
		// Trailing /* becomes literal / prefix
		{"mybucket:logs/*", "mybucket", "logs/"},
		{"mybucket:readonly/*", "mybucket", "readonly/"},
		{"mybucket:backups/*", "mybucket", "backups/"},
		{"*:data/*", "*", "data/"},
		// Bare * becomes empty prefix
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
// space-separated and +-separated verbs, as documented in
// docs/authentication.md (ACL format).
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
			name:      "mixed separators (docs use +, but spaces work)",
			aclString: "mybucket:data/:get+put list",
			wantActions: map[string]bool{
				"get":  true,
				"put":  true,
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
				t.Fatalf("parseACL(%q).Actions is nil, want non-nil", tt.aclString)
			}
			for verb, want := range tt.wantActions {
				if got := acls[0].Actions[verb]; got != want {
					t.Errorf("parseACL(%q).Actions[%q] = %v, want %v", tt.aclString, verb, got, want)
				}
			}
		})
	}
}

// TestACLInvalidVerbs verifies that unknown action verbs are rejected
// (docs/authentication.md, Action verbs: only the five listed verbs parse).
func TestACLInvalidVerbs(t *testing.T) {
	tests := []struct {
		name      string
		aclString string
	}{
		{
			name:      "unknown verb",
			aclString: "mybucket:data/:read",
		},
		{
			name:      "mixed valid and invalid",
			aclString: "mybucket:data/:get+write",
		},
		{
			name:      "uppercase verb (must be lowercase)",
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
		name      string
		aclString string
	}{
		{
			name:      "wildcard in middle of prefix",
			aclString: "mybucket:*/data/",
		},
		{
			name:      "wildcard at start of prefix",
			aclString: "mybucket:*/logs",
		},
		{
			name:      "multiple wildcards",
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
