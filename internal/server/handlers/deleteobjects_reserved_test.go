// Package handlers_test tests S3 operation handlers.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/acl"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server/handlers"
)

// deleteObjectsRequestXML builds a non-quiet Delete request body naming keys.
func deleteObjectsRequestXML(keys ...string) string {
	var b strings.Builder
	b.WriteString("<Delete>")
	for _, key := range keys {
		b.WriteString("<Object><Key>")
		b.WriteString(key)
		b.WriteString("</Key></Object>")
	}
	b.WriteString("</Delete>")
	return b.String()
}

// TestDeleteObjects_ReservedNamespaceRefusedPerKey pins the namespace guard
// for bulk deletes: single-key operations refuse keys under .armor/ in
// HandleRoot, but POST ?delete carries its keys in the request body where
// that guard cannot see them, so DeleteObjects must refuse each reserved
// key itself — whatever the credential's ACL grants, and even when auth is
// disabled — while still deleting the ordinary keys in the same batch.
func TestDeleteObjects_ReservedNamespaceRefusedPerKey(t *testing.T) {
	internalKey := ".armor/hmac/abc"
	ordinaryKey := "ordinary/file.txt"

	credentials := map[string]*config.Credential{
		// The defect shape: a credential whose ACL grants delete at the
		// bucket root (nil ACLs) and would pass every per-key check.
		"full access": {
			AccessKey: "full_access_key",
			SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
			ACLs:      nil,
		},
		// Auth disabled: the HandleRoot guard fires without any
		// credential, so the bulk form must refuse the key here too.
		"no auth": nil,
	}

	for name, cred := range credentials {
		t.Run(name, func(t *testing.T) {
			be := &mockDeleteBackend{
				mockBackend: newMockBackend(),
				deletedKeys: make(map[string]bool),
			}
			cfg, _, cache, footerCache, km := testSetup(t)
			h := handlers.New(cfg, be, cache, footerCache, km, nil)

			rec := newMockManifestRecorder()
			h.WithManifest(rec)

			// Seed both objects directly in the backend and the manifest:
			// the internal sidecar must survive the request, and its
			// manifest entry must not be tombstoned.
			ctx := context.Background()
			if err := be.Put(ctx, "test-bucket", internalKey, strings.NewReader("hmac-sidecar"), 11, nil); err != nil {
				t.Fatalf("seed internal object: %v", err)
			}
			if err := be.Put(ctx, "test-bucket", ordinaryKey, strings.NewReader("ordinary"), 8, nil); err != nil {
				t.Fatalf("seed ordinary object: %v", err)
			}
			for _, key := range []string{internalKey, ordinaryKey} {
				rec.seed("test-bucket", key, &handlers.ManifestEntry{PlaintextSize: 8, ETag: "abc"})
			}

			req := httptest.NewRequest("POST", "/test-bucket?delete",
				bytes.NewReader([]byte(deleteObjectsRequestXML(internalKey, ordinaryKey))))
			if cred != nil {
				req = req.WithContext(acl.WithCredential(req.Context(), cred))
			}
			w := httptest.NewRecorder()
			h.DeleteObjects(w, req, "test-bucket")

			if w.Code != http.StatusOK {
				t.Fatalf("mixed batch: expected 200, got %d: %s", w.Code, w.Body.String())
			}

			type DeleteResult struct {
				XMLName xml.Name `xml:"DeleteResult"`
				Deleted []struct {
					Key string `xml:"Key"`
				} `xml:"Deleted"`
				Error []struct {
					Key     string `xml:"Key"`
					Code    string `xml:"Code"`
					Message string `xml:"Message"`
				} `xml:"Error"`
			}
			var result DeleteResult
			if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}

			if len(result.Deleted) != 1 || result.Deleted[0].Key != ordinaryKey {
				t.Fatalf("Deleted entries = %+v, want exactly the ordinary key %q", result.Deleted, ordinaryKey)
			}
			if len(result.Error) != 1 || result.Error[0].Key != internalKey || result.Error[0].Code != "AccessDenied" {
				t.Fatalf("Error entries = %+v, want one AccessDenied for %q", result.Error, internalKey)
			}
			if !strings.Contains(result.Error[0].Message, ".armor/") {
				t.Errorf("error message %q does not name the reserved namespace", result.Error[0].Message)
			}

			// The internal object must still exist in the backend and in
			// the manifest; the ordinary object must be gone from both.
			if _, _, err := be.Get(ctx, "test-bucket", internalKey); err != nil {
				t.Errorf("internal object was deleted: %v", err)
			}
			if _, ok := rec.Lookup("test-bucket", internalKey); !ok {
				t.Errorf("internal key was tombstoned in the manifest although it was refused")
			}
			if !be.deletedKeys[ordinaryKey] {
				t.Errorf("ordinary key %q was not deleted", ordinaryKey)
			}
			if be.deletedKeys[internalKey] {
				t.Errorf("internal key %q reached the backend delete", internalKey)
			}
		})
	}
}

// TestDeleteObjects_MaxKeysAndBodyBounded pins the two request-size
// refusals: more than 1000 keys in one batch, and a body larger than the
// pre-parse bound, are both rejected with 400 MalformedXML before any key
// is evaluated or removed. The oversized body is well-formed XML naming a
// single key, so only the bound — not the parser — can reject it.
func TestDeleteObjects_MaxKeysAndBodyBounded(t *testing.T) {
	const bodyBound = 8 << 20 // mirrors maxDeleteObjectsBodyBytes in handlers.go

	tests := []struct {
		name string
		body func() string
	}{
		{
			name: "1001 keys",
			body: func() string {
				keys := make([]string, 1001)
				for i := range keys {
					keys[i] = fmt.Sprintf("over/limit/file-%04d.txt", i)
				}
				return deleteObjectsRequestXML(keys...)
			},
		},
		{
			name: "oversized body",
			body: func() string {
				return deleteObjectsRequestXML("k" + strings.Repeat("a", bodyBound))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			be := &mockDeleteBackend{
				mockBackend: newMockBackend(),
				deletedKeys: make(map[string]bool),
			}
			cfg, _, cache, footerCache, km := testSetup(t)
			h := handlers.New(cfg, be, cache, footerCache, km, nil)

			req := httptest.NewRequest("POST", "/test-bucket?delete",
				bytes.NewReader([]byte(tt.body())))
			w := httptest.NewRecorder()
			h.DeleteObjects(w, req, "test-bucket")

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "MalformedXML") {
				t.Errorf("response lacks MalformedXML code: %s", w.Body.String())
			}
			if len(be.deletedKeys) != 0 {
				t.Errorf("rejected request still deleted keys: %v", be.deletedKeys)
			}
		})
	}
}
