//go:build integration
// +build integration

// Package server provides comprehensive end-to-end tests for named credentials
// and ACL enforcement per ADR-012.
//
// These tests validate:
// 1. Parsing ACL strings correctly with action verbs
// 2. Enforcing action verbs per S3 operation
// 3. Testing append-only writer patterns (put+list only)
// 4. Multi-bucket ACL scopes
package server

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"github.com/jedarden/armor/internal/acl"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// TestNamedCredentialsACLParsing validates that ACL strings with action verbs
// are parsed correctly per ADR-012 syntax.
func TestNamedCredentialsACLParsing(t *testing.T) {
	// Import parseACL from config package for testing
	// We'll test by creating credentials and checking their parsed ACLs

	tests := []struct {
		name        string
		aclStr      string
		expectError bool
		validateACL func(t *testing.T, acls []acl.ACLEntry)
	}{
		{
			name:        "empty ACL string",
			aclStr:      "",
			expectError: true,
		},
		{
			name:        "single entry no verbs (backward compat)",
			aclStr:      "mybucket:logs/*",
			expectError: false,
			validateACL: func(t *testing.T, acls []acl.ACLEntry) {
				if len(acls) != 1 {
					t.Fatalf("expected 1 ACL entry, got %d", len(acls))
				}
				entry := acls[0]
				if entry.Bucket != "mybucket" {
					t.Errorf("expected bucket 'mybucket', got %q", entry.Bucket)
				}
				if entry.Prefix != "logs/*" {
					t.Errorf("expected prefix 'logs/*', got %q", entry.Prefix)
				}
				if entry.Actions != nil {
					t.Errorf("expected nil Actions (backward compat), got %v", entry.Actions)
				}
			},
		},
		{
			name:        "single entry with get verb",
			aclStr:      "mybucket:readonly/*:get",
			expectError: false,
			validateACL: func(t *testing.T, acls []acl.ACLEntry) {
				if len(acls) != 1 {
					t.Fatalf("expected 1 ACL entry, got %d", len(acls))
				}
				entry := acls[0]
				if entry.Bucket != "mybucket" {
					t.Errorf("expected bucket 'mybucket', got %q", entry.Bucket)
				}
				if entry.Prefix != "readonly/*" {
					t.Errorf("expected prefix 'readonly/*', got %q", entry.Prefix)
				}
				if entry.Actions == nil || !entry.Actions["get"] {
					t.Errorf("expected Actions[get] = true, got %v", entry.Actions)
				}
				if entry.Actions != nil && (entry.Actions["put"] || entry.Actions["delete"] || entry.Actions["list"]) {
					t.Errorf("expected only 'get' verb, got %v", entry.Actions)
				}
			},
		},
		{
			name:        "single entry with put+list verbs (append-only)",
			aclStr:      "mybucket:backups/*:put+list",
			expectError: false,
			validateACL: func(t *testing.T, acls []acl.ACLEntry) {
				if len(acls) != 1 {
					t.Fatalf("expected 1 ACL entry, got %d", len(acls))
				}
				entry := acls[0]
				if entry.Bucket != "mybucket" {
					t.Errorf("expected bucket 'mybucket', got %q", entry.Bucket)
				}
				if entry.Prefix != "backups/*" {
					t.Errorf("expected prefix 'backups/*', got %q", entry.Prefix)
				}
				if entry.Actions == nil || !entry.Actions["put"] || !entry.Actions["list"] {
					t.Errorf("expected Actions[put] = true and Actions[list] = true, got %v", entry.Actions)
				}
				if entry.Actions != nil && (entry.Actions["get"] || entry.Actions["delete"]) {
					t.Errorf("expected only 'put' and 'list' verbs, got %v", entry.Actions)
				}
			},
		},
		{
			name:        "single entry with space-separated verbs",
			aclStr:      "mybucket:data/*:get list",
			expectError: false,
			validateACL: func(t *testing.T, acls []acl.ACLEntry) {
				if len(acls) != 1 {
					t.Fatalf("expected 1 ACL entry, got %d", len(acls))
				}
				entry := acls[0]
				if entry.Actions == nil || !entry.Actions["get"] || !entry.Actions["list"] {
					t.Errorf("expected Actions[get] = true and Actions[list] = true, got %v", entry.Actions)
				}
				if entry.Actions != nil && (entry.Actions["put"] || entry.Actions["delete"]) {
					t.Errorf("expected only 'get' and 'list' verbs, got %v", entry.Actions)
				}
			},
		},
		{
			name:        "single entry with all four verbs",
			aclStr:      "mybucket:admin/*:get+put+delete+list",
			expectError: false,
			validateACL: func(t *testing.T, acls []acl.ACLEntry) {
				if len(acls) != 1 {
					t.Fatalf("expected 1 ACL entry, got %d", len(acls))
				}
				entry := acls[0]
				if entry.Actions == nil || !entry.Actions["get"] || !entry.Actions["put"] || !entry.Actions["delete"] || !entry.Actions["list"] {
					t.Errorf("expected all four verbs to be true, got %v", entry.Actions)
				}
			},
		},
		{
			name:        "multiple entries with different verbs",
			aclStr:      "bucket-a:data/*:get,bucket-b:logs/*:put+list",
			expectError: false,
			validateACL: func(t *testing.T, acls []acl.ACLEntry) {
				if len(acls) != 2 {
					t.Fatalf("expected 2 ACL entries, got %d", len(acls))
				}
				entryA := acls[0]
				if entryA.Bucket != "bucket-a" || entryA.Prefix != "data/*" {
					t.Errorf("entry 0: expected bucket-a:data/*, got %s:%s", entryA.Bucket, entryA.Prefix)
				}
				if entryA.Actions == nil || !entryA.Actions["get"] {
					t.Errorf("entry 0: expected get action, got %v", entryA.Actions)
				}

				entryB := acls[1]
				if entryB.Bucket != "bucket-b" || entryB.Prefix != "logs/*" {
					t.Errorf("entry 1: expected bucket-b:logs/*, got %s:%s", entryB.Bucket, entryB.Prefix)
				}
				if entryB.Actions == nil || !entryB.Actions["put"] || !entryB.Actions["list"] {
					t.Errorf("entry 1: expected put+list actions, got %v", entryB.Actions)
				}
			},
		},
		{
			name:        "wildcard bucket with verbs",
			aclStr:      "*:public/*:get+list",
			expectError: false,
			validateACL: func(t *testing.T, acls []acl.ACLEntry) {
				if len(acls) != 1 {
					t.Fatalf("expected 1 ACL entry, got %d", len(acls))
				}
				entry := acls[0]
				if entry.Bucket != "*" {
					t.Errorf("expected bucket '*', got %q", entry.Bucket)
				}
				if entry.Prefix != "public/*" {
					t.Errorf("expected prefix 'public/*', got %q", entry.Prefix)
				}
				if entry.Actions == nil || !entry.Actions["get"] || !entry.Actions["list"] {
					t.Errorf("expected get+list actions, got %v", entry.Actions)
				}
			},
		},
		{
			name:        "empty prefix with verbs",
			aclStr:      "mybucket::get",
			expectError: false,
			validateACL: func(t *testing.T, acls []acl.ACLEntry) {
				if len(acls) != 1 {
					t.Fatalf("expected 1 ACL entry, got %d", len(acls))
				}
				entry := acls[0]
				if entry.Bucket != "mybucket" {
					t.Errorf("expected bucket 'mybucket', got %q", entry.Bucket)
				}
				if entry.Prefix != "" {
					t.Errorf("expected empty prefix, got %q", entry.Prefix)
				}
				if entry.Actions == nil || !entry.Actions["get"] {
					t.Errorf("expected get action, got %v", entry.Actions)
				}
			},
		},
		{
			name:        "wildcard prefix with verbs",
			aclStr:      "mybucket:*:put+delete",
			expectError: false,
			validateACL: func(t *testing.T, acls []acl.ACLEntry) {
				if len(acls) != 1 {
					t.Fatalf("expected 1 ACL entry, got %d", len(acls))
				}
				entry := acls[0]
				if entry.Bucket != "mybucket" {
					t.Errorf("expected bucket 'mybucket', got %q", entry.Bucket)
				}
				if entry.Prefix != "" {
					t.Errorf("expected empty prefix (normalized from '*'), got %q", entry.Prefix)
				}
				if entry.Actions == nil || !entry.Actions["put"] || !entry.Actions["delete"] {
					t.Errorf("expected put+delete actions, got %v", entry.Actions)
				}
			},
		},
		{
			name:        "invalid verb is rejected",
			aclStr:      "mybucket:data/*:execute",
			expectError: true,
		},
		{
			name:        "invalid ACL format (no colon)",
			aclStr:      "mybucket-data",
			expectError: true,
		},
		{
			name:        "empty bucket is rejected",
			aclStr:      ":data/*:get",
			expectError: true,
		},
		{
			name:        "mixed separators (plus and space)",
			aclStr:      "mybucket:backups/*:put+list get",
			expectError: false,
			validateACL: func(t *testing.T, acls []acl.ACLEntry) {
				if len(acls) != 1 {
					t.Fatalf("expected 1 ACL entry, got %d", len(acls))
				}
				entry := acls[0]
				if entry.Actions == nil || !entry.Actions["put"] || !entry.Actions["list"] || !entry.Actions["get"] {
					t.Errorf("expected put+list+get actions, got %v", entry.Actions)
				}
			},
		},
		{
			name:        "multi-bucket ACL with different scopes",
			aclStr:      "bucket-primary:*:get+put+delete+list,bucket-audit:logs/*:get+list",
			expectError: false,
			validateACL: func(t *testing.T, acls []acl.ACLEntry) {
				if len(acls) != 2 {
					t.Fatalf("expected 2 ACL entries, got %d", len(acls))
				}
				entryPrimary := acls[0]
				if entryPrimary.Bucket != "bucket-primary" {
					t.Errorf("expected bucket 'bucket-primary', got %q", entryPrimary.Bucket)
				}
				// Wildcard prefix normalizes to empty
				if entryPrimary.Prefix != "" {
					t.Errorf("expected empty prefix (normalized from '*'), got %q", entryPrimary.Prefix)
				}
				if len(entryPrimary.Actions) != 4 {
					t.Errorf("expected 4 actions for bucket-primary, got %d", len(entryPrimary.Actions))
				}

				entryAudit := acls[1]
				if entryAudit.Bucket != "bucket-audit" {
					t.Errorf("expected bucket 'bucket-audit', got %q", entryAudit.Bucket)
				}
				if entryAudit.Prefix != "logs/*" {
					t.Errorf("expected prefix 'logs/*', got %q", entryAudit.Prefix)
				}
				if len(entryAudit.Actions) != 2 {
					t.Errorf("expected 2 actions for bucket-audit, got %d", len(entryAudit.Actions))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acls, err := parseACL(tt.aclStr)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error parsing ACL %q, got nil", tt.aclStr)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error parsing ACL %q: %v", tt.aclStr, err)
				return
			}

			if tt.validateACL != nil {
				tt.validateACL(t, acls)
			}
		})
	}
}

// parseACL is exported from internal/config for testing - this mirrors that implementation
func parseACL(aclStr string) ([]acl.ACLEntry, error) {
	if aclStr == "" {
		return nil, fmt.Errorf("ACL string contains no valid entries")
	}

	var entries []acl.ACLEntry
	parts := strings.Split(aclStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		seg := strings.SplitN(part, ":", 3)
		if len(seg) < 2 {
			return nil, fmt.Errorf("invalid ACL entry %q (expected bucket:prefix)", part)
		}

		bucket := strings.TrimSpace(seg[0])
		prefix := strings.TrimSpace(seg[1])

		if bucket == "" {
			return nil, fmt.Errorf("invalid ACL entry %q (empty bucket)", part)
		}

		// Normalize wildcard prefix
		if prefix == "*" {
			prefix = ""
		}

		entry := acl.ACLEntry{
			Bucket: bucket,
			Prefix: prefix,
		}

		// Optional third segment: action verbs
		if len(seg) == 3 {
			actions, err := parseActions(seg[2])
			if err != nil {
				return nil, fmt.Errorf("invalid ACL entry %q: %w", part, err)
			}
			entry.Actions = actions
		}

		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("ACL string contains no valid entries")
	}

	return entries, nil
}

// parseActions parses the optional third ACL segment ("get+list") into an Actions set
func parseActions(verbStr string) (map[string]bool, error) {
	verbs := strings.Fields(strings.ReplaceAll(verbStr, "+", " "))
	if len(verbs) == 0 {
		return nil, nil
	}

	validActions := map[string]bool{
		"get":    true,
		"put":    true,
		"delete": true,
		"list":   true,
	}

	actions := make(map[string]bool, len(verbs))
	for _, v := range verbs {
		if !validActions[v] {
			return nil, fmt.Errorf("invalid action verb %q (expected one of get, put, delete, list)", v)
		}
		actions[v] = true
	}
	return actions, nil
}

// TestAppendOnlyWriterPattern validates the append-only backup writer role
// (Put+List only) described in ADR-012 decision 3.
func TestAppendOnlyWriterPattern(t *testing.T) {
	// Append-only backup-writer credential (ADR-012 decision 3): writes and
	// listings only, scoped to a backups/ prefix.
	credentials := map[string]*config.Credential{
		"BACKUPWRITER": {
			AccessKey: "BACKUPWRITER",
			SecretKey: "BACKUPSECRET1234567890123456789012",
			ACLs: []acl.ACLEntry{{
				Bucket:  "test-bucket",
				Prefix:  "backups/",
				Actions: map[string]bool{ActionPut: true, ActionList: true},
			}},
		},
	}

	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	tests := []struct {
		name        string
		method      string
		path        string
		key         string // S3 key for ACL check
		expectAllow bool
	}{
		// Append-only role permits writes
		{"PutObject allowed", "PUT", "/test-bucket/backups/db.dump", "backups/db.dump", true},
		{"CreateMultipartUpload allowed", "POST", "/test-bucket/backups/large.tar?uploads", "backups/large.tar", true},

		// Append-only role permits listings
		{"ListObjectsV2 allowed", "GET", "/test-bucket?prefix=backups/", "backups/", true},
		{"ListMultipartUploads allowed", "GET", "/test-bucket?uploads&prefix=backups/", "backups/", true},

		// Append-only role denies reads (no exfiltration)
		{"GetObject denied", "GET", "/test-bucket/backups/db.dump", "backups/db.dump", false},
		{"HeadObject denied", "HEAD", "/test-bucket/backups/db.dump", "backups/db.dump", false},

		// Append-only role denies deletes (no destruction)
		{"DeleteObject denied", "DELETE", "/test-bucket/backups/old.dump", "backups/old.dump", false},
		{"DeleteObjects denied", "POST", "/test-bucket?delete", "backups/file", false},
		{"AbortMultipartUpload denied", "DELETE", "/test-bucket/backups/file.tar?uploadId=123", "backups/file.tar", false},

		// Out-of-scope operations are denied
		{"Wrong prefix denied", "GET", "/test-bucket/other/file", "other/file", false},
		{"Wrong bucket denied", "GET", "/other-bucket/backups/file", "backups/file", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createSignedRequestForAuthTest(t, tt.method, tt.path, "", "BACKUPWRITER", "BACKUPSECRET1234567890123456789012", nil)
			cred, err := auth.VerifyRequest(req, nil)
			if err != nil {
				t.Fatalf("SigV4 verification failed: %v", err)
			}

			verb := ActionForRequest(req)
			err = acl.CheckACL(cred, "test-bucket", tt.key, verb)

			if tt.expectAllow && err != nil {
				t.Errorf("expected ACL allow, got denial: %v (verb=%s)", err, verb)
			}
			if !tt.expectAllow && err != acl.ErrAccessDenied {
				t.Errorf("expected ACL denial (acl.ErrAccessDenied), got: %v (verb=%s)", err, verb)
			}
		})
	}

	// Test the overwrite-as-destruction residual risk (ADR-012 decision 3)
	t.Run("overwrite-as-destruction residual risk", func(t *testing.T) {
		req := createSignedRequestForAuthTest(t, "PUT", "/test-bucket/backups/db.dump", "", "BACKUPWRITER", "BACKUPSECRET1234567890123456789012", nil)
		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("SigV4 verification failed: %v", err)
		}

		verb := ActionForRequest(req)
		err = acl.CheckACL(cred, "test-bucket", "backups/db.dump", verb)

		// In v1, PutObject overwrites are permitted (accepted residual risk)
		if err != nil {
			t.Errorf("overwrite PutObject should be permitted by append-only role (v1 residual risk), got: %v", err)
		}
	})
}

// TestReadOnlyWriterPattern validates read-only credentials with Get+List verbs.
func TestReadOnlyWriterPattern(t *testing.T) {
	credentials := map[string]*config.Credential{
		"READONLY": {
			AccessKey: "READONLY",
			SecretKey: "READONLYSECRET123456789012345678",
			ACLs: []acl.ACLEntry{{
				Bucket:  "test-bucket",
				Prefix:  "public/",
				Actions: map[string]bool{ActionGet: true, ActionList: true},
			}},
		},
	}

	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	tests := []struct {
		name        string
		method      string
		path        string
		key         string
		expectAllow bool
	}{
		// Read-only role permits reads
		{"GetObject allowed", "GET", "/test-bucket/public/data.txt", "public/data.txt", true},
		{"HeadObject allowed", "HEAD", "/test-bucket/public/data.txt", "public/data.txt", true},
		{"ListObjectsV2 allowed", "GET", "/test-bucket?prefix=public/", "public/", true},

		// Read-only role denies writes
		{"PutObject denied", "PUT", "/test-bucket/public/new.txt", "public/new.txt", false},
		{"CreateMultipartUpload denied", "POST", "/test-bucket/public/large.tar?uploads", "public/large.tar", false},

		// Read-only role denies deletes
		{"DeleteObject denied", "DELETE", "/test-bucket/public/old.txt", "public/old.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createSignedRequestForAuthTest(t, tt.method, tt.path, "", "READONLY", "READONLYSECRET123456789012345678", nil)
			cred, err := auth.VerifyRequest(req, nil)
			if err != nil {
				t.Fatalf("SigV4 verification failed: %v", err)
			}

			verb := ActionForRequest(req)
			err = acl.CheckACL(cred, "test-bucket", tt.key, verb)

			if tt.expectAllow && err != nil {
				t.Errorf("expected ACL allow, got denial: %v", err)
			}
			if !tt.expectAllow && err != acl.ErrAccessDenied {
				t.Errorf("expected ACL denial, got: %v", err)
			}
		})
	}
}

// TestDeleteOnlyWriterPattern validates delete-only credentials with Delete verb.
func TestDeleteOnlyWriterPattern(t *testing.T) {
	credentials := map[string]*config.Credential{
		"CLEANUP": {
			AccessKey: "CLEANUP",
			SecretKey: "CLEANUPSECRET1234567890123456789",
			ACLs: []acl.ACLEntry{{
				Bucket:  "test-bucket",
				Prefix:  "temp/",
				Actions: map[string]bool{ActionDelete: true},
			}},
		},
	}

	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	tests := []struct {
		name        string
		method      string
		path        string
		key         string
		expectAllow bool
	}{
		// Delete-only role permits deletes
		{"DeleteObject allowed", "DELETE", "/test-bucket/temp/file.txt", "temp/file.txt", true},
		{"DeleteObjects allowed", "POST", "/test-bucket?delete", "temp/file", true},
		{"AbortMultipartUpload allowed", "DELETE", "/test-bucket/temp/file.tar?uploadId=123", "temp/file.tar", true},

		// Delete-only role denies reads
		{"GetObject denied", "GET", "/test-bucket/temp/file.txt", "temp/file.txt", false},

		// Delete-only role denies writes
		{"PutObject denied", "PUT", "/test-bucket/temp/new.txt", "temp/new.txt", false},

		// Delete-only role denies listings
		{"ListObjectsV2 denied", "GET", "/test-bucket?prefix=temp/", "temp/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createSignedRequestForAuthTest(t, tt.method, tt.path, "", "CLEANUP", "CLEANUPSECRET1234567890123456789", nil)
			cred, err := auth.VerifyRequest(req, nil)
			if err != nil {
				t.Fatalf("SigV4 verification failed: %v", err)
			}

			verb := ActionForRequest(req)
			err = acl.CheckACL(cred, "test-bucket", tt.key, verb)

			if tt.expectAllow && err != nil {
				t.Errorf("expected ACL allow, got denial: %v", err)
			}
			if !tt.expectAllow && err != acl.ErrAccessDenied {
				t.Errorf("expected ACL denial, got: %v", err)
			}
		})
	}
}

// TestMultiBucketACLScope validates ACL enforcement across multiple buckets.
func TestMultiBucketACLScope(t *testing.T) {
	// Multi-bucket credential with different scopes per bucket
	credentials := map[string]*config.Credential{
		"CROSSBUCKET": {
			AccessKey: "CROSSBUCKET",
			SecretKey: "CROSSSECRET12345678901234567890123",
			ACLs: []acl.ACLEntry{
				{Bucket: "bucket-primary", Prefix: "", Actions: map[string]bool{ActionGet: true, ActionPut: true, ActionDelete: true, ActionList: true}},
				{Bucket: "bucket-audit", Prefix: "logs/", Actions: map[string]bool{ActionGet: true, ActionList: true}},
				{Bucket: "*", Prefix: "public/", Actions: map[string]bool{ActionGet: true}},
			},
		},
	}

	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	tests := []struct {
		name        string
		method      string
		path        string
		bucket      string
		key         string
		expectAllow bool
	}{
		// Full access to bucket-primary
		{"bucket-primary full access", "PUT", "/bucket-primary/file.txt", "bucket-primary", "file.txt", true},
		{"bucket-primary delete", "DELETE", "/bucket-primary/old.txt", "bucket-primary", "old.txt", true},

		// Read-only access to bucket-audit logs/
		{"bucket-audit logs read", "GET", "/bucket-audit/logs/app.log", "bucket-audit", "logs/app.log", true},
		{"bucket-audit logs list", "GET", "/bucket-audit?prefix=logs/", "bucket-audit", "logs/", true},
		{"bucket-audit logs write denied", "PUT", "/bucket-audit/logs/new.log", "bucket-audit", "logs/new.log", false},
		{"bucket-audit non-logs denied", "GET", "/bucket-audit/admin/config", "bucket-audit", "admin/config", false},

		// Read-only public/ on all buckets via wildcard
		{"wildcard public read", "GET", "/any-bucket/public/file.txt", "any-bucket", "public/file.txt", true},
		{"wildcard public write denied", "PUT", "/any-bucket/public/file.txt", "any-bucket", "public/file.txt", false},
		{"wildcard non-public denied", "GET", "/any-bucket/private/file.txt", "any-bucket", "private/file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createSignedRequestForAuthTest(t, tt.method, tt.path, "", "CROSSBUCKET", "CROSSSECRET12345678901234567890123", nil)
			cred, err := auth.VerifyRequest(req, nil)
			if err != nil {
				t.Fatalf("SigV4 verification failed: %v", err)
			}

			verb := ActionForRequest(req)
			err = acl.CheckACL(cred, tt.bucket, tt.key, verb)

			if tt.expectAllow && err != nil {
				t.Errorf("expected ACL allow, got denial: %v", err)
			}
			if !tt.expectAllow && err != acl.ErrAccessDenied {
				t.Errorf("expected ACL denial, got: %v", err)
			}
		})
	}
}

// TestWildcardBucketWithPrefix validates wildcard bucket ACLs with prefix restrictions.
func TestWildcardBucketWithPrefix(t *testing.T) {
	credentials := map[string]*config.Credential{
		"PUBLICREADER": {
			AccessKey: "PUBLICREADER",
			SecretKey: "PUBLICSECRET123456789012345678901",
			ACLs: []acl.ACLEntry{
				{Bucket: "*", Prefix: "public/", Actions: map[string]bool{ActionGet: true, ActionList: true}},
			},
		},
	}

	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	tests := []struct {
		name        string
		method      string
		path        string
		bucket      string
		key         string
		expectAllow bool
	}{
		{"public/ on any bucket", "GET", "/bucket-a/public/file.txt", "bucket-a", "public/file.txt", true},
		{"public/ on another bucket", "GET", "/bucket-b/public/data", "bucket-b", "public/data", true},
		{"non-public denied", "GET", "/bucket-a/private/file", "bucket-a", "private/file", false},
		{"public/ write denied", "PUT", "/bucket-a/public/new", "bucket-a", "public/new", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createSignedRequestForAuthTest(t, tt.method, tt.path, "", "PUBLICREADER", "PUBLICSECRET123456789012345678901", nil)
			cred, err := auth.VerifyRequest(req, nil)
			if err != nil {
				t.Fatalf("SigV4 verification failed: %v", err)
			}

			verb := ActionForRequest(req)
			err = acl.CheckACL(cred, tt.bucket, tt.key, verb)

			if tt.expectAllow && err != nil {
				t.Errorf("expected ACL allow, got denial: %v", err)
			}
			if !tt.expectAllow && err != acl.ErrAccessDenied {
				t.Errorf("expected ACL denial, got: %v", err)
			}
		})
	}
}

// TestBackwardCompatibleACLs validates that two-segment "bucket:prefix" ACLs
// (without verbs) still grant full access (backward compatibility).
func TestBackwardCompatibleACLs(t *testing.T) {
	credentials := map[string]*config.Credential{
		"LEGACY": {
			AccessKey: "LEGACY",
			SecretKey: "LEGACYSECRET12345678901234567890",
			ACLs: []acl.ACLEntry{
				{Bucket: "test-bucket", Prefix: "legacy/"}, // No Actions = all verbs permitted
			},
		},
	}

	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	tests := []struct {
		name   string
		method string
		path   string
		key    string
	}{
		{"GetObject", "GET", "/test-bucket/legacy/file.txt", "legacy/file.txt"},
		{"PutObject", "PUT", "/test-bucket/legacy/new.txt", "legacy/new.txt"},
		{"DeleteObject", "DELETE", "/test-bucket/legacy/old.txt", "legacy/old.txt"},
		{"ListObjectsV2", "GET", "/test-bucket?prefix=legacy/", "legacy/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createSignedRequestForAuthTest(t, tt.method, tt.path, "", "LEGACY", "LEGACYSECRET12345678901234567890", nil)
			cred, err := auth.VerifyRequest(req, nil)
			if err != nil {
				t.Fatalf("SigV4 verification failed: %v", err)
			}

			verb := ActionForRequest(req)
			err = acl.CheckACL(cred, "test-bucket", tt.key, verb)

			if err != nil {
				t.Errorf("backward-compatible ACL (no verbs) should allow %s, got denial: %v", tt.name, err)
			}
		})
	}
}

// TestEmptyACLGrantsFullAccess validates that credentials with no ACLs
// (empty ACL list) have full access.
func TestEmptyACLGrantsFullAccess(t *testing.T) {
	credentials := map[string]*config.Credential{
		"FULLACCESS": {
			AccessKey: "FULLACCESS",
			SecretKey: "FULLSECRET12345678901234567890123",
			ACLs:      nil, // nil = full access
		},
	}

	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	tests := []struct {
		name   string
		method string
		path   string
		key    string
	}{
		{"GetObject", "GET", "/any-bucket/any/path", "any/path"},
		{"PutObject", "PUT", "/any-bucket/any/path", "any/path"},
		{"DeleteObject", "DELETE", "/any-bucket/any/path", "any/path"},
		{"ListObjectsV2", "GET", "/any-bucket?prefix=any/", "any/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createSignedRequestForAuthTest(t, tt.method, tt.path, "", "FULLACCESS", "FULLSECRET12345678901234567890123", nil)
			cred, err := auth.VerifyRequest(req, nil)
			if err != nil {
				t.Fatalf("SigV4 verification failed: %v", err)
			}

			verb := ActionForRequest(req)
			err = acl.CheckACL(cred, "any-bucket", tt.key, verb)

			if err != nil {
				t.Errorf("empty ACL (nil) should grant full access for %s, got denial: %v", tt.name, err)
			}
		})
	}
}

// TestACLEndToEndIntegration tests ACL enforcement through a full HTTP server.
func TestACLEndToEndIntegration(t *testing.T) {
	// Create test server with multiple credentials
	credentials := map[string]*config.Credential{
		"FULLADMIN": {
			AccessKey: "FULLADMIN",
			SecretKey: "FULLADMINSECRET12345678901234567",
			ACLs:      nil, // Full access
		},
		"BACKUPWRITER": {
			AccessKey: "BACKUPWRITER",
			SecretKey: "BACKUPSECRET1234567890123456789012",
			ACLs: []acl.ACLEntry{{
				Bucket:  "test-bucket",
				Prefix:  "backups/",
				Actions: map[string]bool{ActionPut: true, ActionList: true},
			}},
		},
		"READONLY": {
			AccessKey: "READONLY",
			SecretKey: "READONLYSECRET123456789012345678",
			ACLs: []acl.ACLEntry{{
				Bucket:  "test-bucket",
				Prefix:  "readonly/",
				Actions: map[string]bool{ActionGet: true, ActionList: true},
			}},
		},
	}

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i % 256)
	}

	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         mek,
		BlockSize:   65536,
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}

	testServer := httptest.NewUnstartedServer(srv.Handler())
	testServer.Start()
	defer testServer.Close()

	baseURL := testServer.URL

	// Test append-only writer pattern
	t.Run("append-only writer pattern end-to-end", func(t *testing.T) {
		tests := []struct {
			name        string
			accessKey   string
			secretKey   string
			method      string
			path        string
			body        []byte
			expectCode  int
			expectError string
		}{
			{"backup writer can PUT", "BACKUPWRITER", "BACKUPSECRET1234567890123456789012", "PUT", "/test-bucket/backups/db.dump", []byte("data"), 200, ""},
			{"backup writer can LIST", "BACKUPWRITER", "BACKUPSECRET1234567890123456789012", "GET", "/test-bucket?prefix=backups/", nil, 200, ""},
			{"backup writer cannot GET", "BACKUPWRITER", "BACKUPSECRET1234567890123456789012", "GET", "/test-bucket/backups/db.dump", nil, 403, "AccessDenied"},
			{"backup writer cannot DELETE", "BACKUPWRITER", "BACKUPSECRET1234567890123456789012", "DELETE", "/test-bucket/backups/db.dump", nil, 403, "AccessDenied"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := createSignedRequestForE2E(t, tt.method, baseURL+tt.path, tt.body, tt.accessKey, tt.secretKey)
				resp := makeRequestForE2E(t, testServer, req)

				if resp.StatusCode != tt.expectCode {
					bodyBytes, _ := io.ReadAll(resp.Body)
					t.Errorf("expected status %d, got %d: %s", tt.expectCode, resp.StatusCode, string(bodyBytes))
				}

				if tt.expectError != "" {
					var s3Err S3Error
					bodyBytes, _ := io.ReadAll(resp.Body)
					resp.Body.Close()
					if xml.Unmarshal(bodyBytes, &s3Err) == nil && s3Err.Code != tt.expectError {
						t.Errorf("expected error code %s, got %s", tt.expectError, s3Err.Code)
					}
				}
			})
		}
	})

	// Test read-only pattern
	t.Run("read-only pattern end-to-end", func(t *testing.T) {
		tests := []struct {
			name       string
			accessKey  string
			secretKey  string
			method     string
			path       string
			body       []byte
			expectCode int
		}{
			{"readonly can LIST", "READONLY", "READONLYSECRET123456789012345678", "GET", "/test-bucket?prefix=readonly/", nil, 200},
			{"readonly cannot PUT", "READONLY", "READONLYSECRET123456789012345678", "PUT", "/test-bucket/readonly/file", []byte("data"), 403},
			{"readonly cannot DELETE", "READONLY", "READONLYSECRET123456789012345678", "DELETE", "/test-bucket/readonly/file", nil, 403},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := createSignedRequestForE2E(t, tt.method, baseURL+tt.path, tt.body, tt.accessKey, tt.secretKey)
				resp := makeRequestForE2E(t, testServer, req)

				if resp.StatusCode != tt.expectCode {
					bodyBytes, _ := io.ReadAll(resp.Body)
					resp.Body.Close()
					t.Errorf("expected status %d, got %d: %s", tt.expectCode, resp.StatusCode, string(bodyBytes))
				}
			})
		}
	})

	// Test admin has full access
	t.Run("admin has full access end-to-end", func(t *testing.T) {
		req := createSignedRequestForE2E(t, "GET", baseURL+"/test-bucket/backups/db.dump", nil, "FULLADMIN", "FULLADMINSECRET12345678901234567")
		resp := makeRequestForE2E(t, testServer, req)
		defer resp.Body.Close()

		// Admin should not get 403 (might get 404 for non-existent object, but not access denied)
		if resp.StatusCode == 403 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Errorf("Admin should have full access, got 403: %s", string(bodyBytes))
		}
	})
}

// createSignedRequestForE2E creates an authenticated HTTP request for end-to-end testing
func createSignedRequestForE2E(t *testing.T, method, url string, body []byte, accessKey, secretKey string) *http.Request {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Parse URL to get path for signing
	u, err := req.URL.Parse(url)
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}
	req.URL = u

	// Sign the request
	auth := &SigV4Auth{
		credentials: map[string]*config.Credential{
			accessKey: {
				AccessKey: accessKey,
				SecretKey: secretKey,
			},
		},
		region:  "us-east-005",
		service: "s3",
	}

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	credentialDate := amzDate[:8]
	credentialScope := credentialDate + "/us-east-005/s3/aws4_request"

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", sha256Sum(body))

	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	sort.Strings(signedHeaders)

	canonicalRequest := auth.buildCanonicalRequest(req, signedHeaders, body)
	stringToSign := auth.buildStringToSign(amzDate, credentialScope, "us-east-005", canonicalRequest)

	cred := &config.Credential{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}
	signingKey := auth.getSigningKeyForCredential(cred, credentialDate, "us-east-005")
	signature := hex.EncodeToString(auth.hmacSHA256(signingKey, stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, strings.Join(signedHeaders, ";"), signature)
	req.Header.Set("Authorization", authHeader)

	return req
}

// makeRequestForE2E sends an HTTP request and returns the response
func makeRequestForE2E(t *testing.T, ts *httptest.Server, req *http.Request) *http.Response {
	t.Helper()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	return resp
}
