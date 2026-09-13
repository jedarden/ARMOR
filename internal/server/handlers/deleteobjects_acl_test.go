// Package handlers_test tests S3 operation handlers.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/acl"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server/handlers"
)

// TestDeleteObjects_PerKeyACLAEnforcement verifies that DeleteObjects enforces
// ACLs on a per-key basis rather than only at the bucket level.
func TestDeleteObjects_PerKeyACLAEnforcement(t *testing.T) {
	// Create test backend
	be := &mockDeleteBackend{
		mockBackend: newMockBackend(),
		deletedKeys: make(map[string]bool),
	}

	// Create handlers through the exported constructor, reusing the shared
	// test setup for caches and key manager.
	cfg, _, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, be, cache, footerCache, km, nil)

	// Test credentials with different ACL scopes
	credFullAccess := &config.Credential{
		AccessKey: "full_access_key",
		SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
		ACLs:      nil, // nil means full access
	}

	credPrefixOnly := &config.Credential{
		AccessKey: "prefix_key",
		SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
		ACLs: []acl.ACLEntry{
			{
				Bucket: "test-bucket",
				Prefix: "allowed/",
			},
		},
	}

	credNoAccess := &config.Credential{
		AccessKey: "no_access_key",
		SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
		ACLs: []acl.ACLEntry{
			{
				Bucket: "other-bucket",
				Prefix: "",
			},
		},
	}

	tests := []struct {
		name            string
		credential      *config.Credential
		keysToDelete    []string
		expectedAllowed []string
		expectedDenied  []string
	}{
		{
			name:       "Full access allows all keys",
			credential: credFullAccess,
			keysToDelete: []string{
				"allowed/file1.txt",
				"allowed/file2.txt",
				"other/file3.txt",
			},
			expectedAllowed: []string{
				"allowed/file1.txt",
				"allowed/file2.txt",
				"other/file3.txt",
			},
			expectedDenied: nil,
		},
		{
			name:       "Prefix ACL allows only matching keys",
			credential: credPrefixOnly,
			keysToDelete: []string{
				"allowed/file1.txt",
				"allowed/file2.txt",
				"other/file3.txt",
				"restricted/file4.txt",
			},
			expectedAllowed: []string{
				"allowed/file1.txt",
				"allowed/file2.txt",
			},
			expectedDenied: []string{
				"other/file3.txt",
				"restricted/file4.txt",
			},
		},
		{
			name:       "No access denies all keys",
			credential: credNoAccess,
			keysToDelete: []string{
				"any/file.txt",
				"another/file.txt",
			},
			expectedAllowed: nil,
			expectedDenied: []string{
				"any/file.txt",
				"another/file.txt",
			},
		},
		{
			name:       "Nil credential (no auth) allows all",
			credential: nil,
			keysToDelete: []string{
				"any/file.txt",
				"another/file.txt",
			},
			expectedAllowed: []string{
				"any/file.txt",
				"another/file.txt",
			},
			expectedDenied: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock backend
			be.deletedKeys = make(map[string]bool)

			// Build DeleteObjects request XML
			type Object struct {
				Key string `xml:"Key"`
			}
			type DeleteRequest struct {
				XMLName xml.Name `xml:"Delete"`
				Objects []Object `xml:"Object"`
				Quiet   bool     `xml:"Quiet"`
			}

			reqXML, err := xml.Marshal(DeleteRequest{
				Objects: func() []Object {
					objs := make([]Object, len(tt.keysToDelete))
					for i, key := range tt.keysToDelete {
						objs[i] = Object{Key: key}
					}
					return objs
				}(),
				Quiet: false,
			})
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			// Create HTTP request with credential in context
			req := httptest.NewRequest("POST", "/test-bucket?delete", bytes.NewReader(reqXML))
			req.Header.Set("Content-Type", "application/xml")

			// Store credential in context
			if tt.credential != nil {
				req = req.WithContext(acl.WithCredential(req.Context(), tt.credential))
			}

			w := httptest.NewRecorder()

			// Call DeleteObjects handler
			h.DeleteObjects(w, req, "test-bucket")

			// Check response status
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			// Parse response
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
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			// Verify deleted keys
			deletedKeys := make([]string, len(result.Deleted))
			for i, d := range result.Deleted {
				deletedKeys[i] = d.Key
			}

			// Verify denied keys
			deniedKeys := make([]string, len(result.Error))
			for i, e := range result.Error {
				deniedKeys[i] = e.Key
				if e.Code != "AccessDenied" {
					t.Errorf("Expected error code AccessDenied, got %s", e.Code)
				}
			}

			// Check expected allowed keys
			if !stringSlicesEqual(deletedKeys, tt.expectedAllowed) {
				t.Errorf("Expected allowed keys %v, got %v", tt.expectedAllowed, deletedKeys)
			}

			// Check expected denied keys
			if !stringSlicesEqual(deniedKeys, tt.expectedDenied) {
				t.Errorf("Expected denied keys %v, got %v", tt.expectedDenied, deniedKeys)
			}

			// Verify backend only deleted allowed keys
			for _, key := range tt.expectedAllowed {
				if !be.deletedKeys[key] {
					t.Errorf("Expected backend to delete key %s, but it was not deleted", key)
				}
			}
			for _, key := range tt.keysToDelete {
				expected := false
				for _, allowed := range tt.expectedAllowed {
					if key == allowed {
						expected = true
						break
					}
				}
				actual := be.deletedKeys[key]
				if actual != expected {
					t.Errorf("Key %s: expected backend delete=%v, got=%v", key, expected, actual)
				}
			}
		})
	}
}

// TestDeleteObjects_DeniedKeysNotTombstoned pins the manifest contract of a
// mixed batch: only the allowed keys may be removed from the manifest index.
// A denied key still exists in the backend, so dropping its manifest entry
// (and persisting a "del" delta for it) would hide a live object from
// manifest lookups even though nothing was deleted.
func TestDeleteObjects_DeniedKeysNotTombstoned(t *testing.T) {
	be := &mockDeleteBackend{
		mockBackend: newMockBackend(),
		deletedKeys: make(map[string]bool),
	}

	cfg, _, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, be, cache, footerCache, km, nil)

	rec := newMockManifestRecorder()
	h.WithManifest(rec)

	cred := &config.Credential{
		AccessKey: "prefix_key",
		SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
		ACLs: []acl.ACLEntry{
			{
				Bucket: "test-bucket",
				Prefix: "allowed/",
			},
		},
	}

	for _, key := range []string{"allowed/in-scope.txt", "denied/out-of-scope.txt"} {
		rec.seed("test-bucket", key, &handlers.ManifestEntry{PlaintextSize: 10, ETag: "abc"})
	}

	body := `<Delete>` +
		`<Object><Key>allowed/in-scope.txt</Key></Object>` +
		`<Object><Key>denied/out-of-scope.txt</Key></Object>` +
		`</Delete>`

	req := httptest.NewRequest("POST", "/test-bucket?delete", bytes.NewReader([]byte(body)))
	req = req.WithContext(acl.WithCredential(req.Context(), cred))
	w := httptest.NewRecorder()
	h.DeleteObjects(w, req, "test-bucket")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if _, ok := rec.Lookup("test-bucket", "allowed/in-scope.txt"); ok {
		t.Errorf("allowed key should have been removed from the manifest")
	}
	if _, ok := rec.Lookup("test-bucket", "denied/out-of-scope.txt"); !ok {
		t.Errorf("denied key was tombstoned in the manifest although the ACL refused to delete it")
	}
}

// TestDeleteObjects_QuietModeStillReportsDeniedKeys pins the S3 quiet-mode
// contract: Quiet omits the successful Deleted entries but per-key errors
// must still be reported, otherwise a restricted credential's failed batch
// looks fully successful.
func TestDeleteObjects_QuietModeStillReportsDeniedKeys(t *testing.T) {
	be := &mockDeleteBackend{
		mockBackend: newMockBackend(),
		deletedKeys: make(map[string]bool),
	}

	cfg, _, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, be, cache, footerCache, km, nil)

	cred := &config.Credential{
		AccessKey: "prefix_key",
		SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
		ACLs: []acl.ACLEntry{
			{
				Bucket: "test-bucket",
				Prefix: "allowed/",
			},
		},
	}

	body := `<Delete>` +
		`<Quiet>true</Quiet>` +
		`<Object><Key>allowed/in-scope.txt</Key></Object>` +
		`<Object><Key>denied/out-of-scope.txt</Key></Object>` +
		`</Delete>`

	req := httptest.NewRequest("POST", "/test-bucket?delete", bytes.NewReader([]byte(body)))
	req = req.WithContext(acl.WithCredential(req.Context(), cred))
	w := httptest.NewRecorder()
	h.DeleteObjects(w, req, "test-bucket")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	type DeleteResult struct {
		XMLName xml.Name `xml:"DeleteResult"`
		Deleted []struct {
			Key string `xml:"Key"`
		} `xml:"Deleted"`
		Error []struct {
			Key  string `xml:"Key"`
			Code string `xml:"Code"`
		} `xml:"Error"`
	}

	var result DeleteResult
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result.Deleted) != 0 {
		t.Errorf("quiet mode must omit successful Deleted entries, got %+v", result.Deleted)
	}
	if len(result.Error) != 1 || result.Error[0].Key != "denied/out-of-scope.txt" || result.Error[0].Code != "AccessDenied" {
		t.Errorf("quiet mode must still report per-key AccessDenied, got %+v", result.Error)
	}
}

// mockDeleteBackend embeds the package's full mock backend and records
// which keys DeleteObjects actually removed, so the test can assert per-key
// ACL enforcement without re-implementing the whole Backend interface.
type mockDeleteBackend struct {
	*mockBackend
	deletedKeys map[string]bool
}

func (m *mockDeleteBackend) Delete(ctx context.Context, bucket, key string) error {
	m.deletedKeys[key] = true
	return m.mockBackend.Delete(ctx, bucket, key)
}

func (m *mockDeleteBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		m.deletedKeys[key] = true
	}
	return m.mockBackend.DeleteObjects(ctx, bucket, keys)
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
