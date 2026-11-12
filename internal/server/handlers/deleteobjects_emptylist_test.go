// Package handlers_test tests S3 operation handlers.
package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/server/handlers"
)

// TestDeleteObjects_EmptyBatchAccepted pins the empty-list conformance of
// DeleteObjects: a well-formed request naming zero keys is accepted and
// answered 200 with an empty <DeleteResult>, both in normal and quiet mode,
// and touches nothing in the backend or the manifest. A body that is not
// XML at all stays MalformedXML — the acceptance is an empty *list*, never
// an unparseable body.
func TestDeleteObjects_EmptyBatchAccepted(t *testing.T) {
	accepted := []struct {
		name string
		body string
	}{
		{name: "empty list", body: "<Delete></Delete>"},
		{name: "empty list quiet", body: "<Delete><Quiet>true</Quiet></Delete>"},
	}

	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			be := &mockDeleteBackend{
				mockBackend: newMockBackend(),
				deletedKeys: make(map[string]bool),
			}
			cfg, _, cache, footerCache, km := testSetup(t)
			h := handlers.New(cfg, be, cache, footerCache, km, nil)
			rec := newMockManifestRecorder()
			h.WithManifest(rec)

			// Seed one ordinary object so "an empty batch touches nothing"
			// is asserted against a populated bucket, not an empty one.
			ctx := context.Background()
			if err := be.Put(ctx, "test-bucket", "ordinary/file.txt", strings.NewReader("ordinary"), 8, nil); err != nil {
				t.Fatalf("seed ordinary object: %v", err)
			}
			rec.seed("test-bucket", "ordinary/file.txt", &handlers.ManifestEntry{PlaintextSize: 8, ETag: "abc"})

			req := httptest.NewRequest(http.MethodPost, "/test-bucket?delete",
				bytes.NewReader([]byte(tt.body)))
			w := httptest.NewRecorder()
			h.DeleteObjects(w, req, "test-bucket")

			if w.Code != http.StatusOK {
				t.Fatalf("empty batch: expected 200, got %d: %s", w.Code, w.Body.String())
			}
			body := w.Body.String()
			if !strings.Contains(body, "<DeleteResult") {
				t.Fatalf("response is not a DeleteResult document: %s", body)
			}
			if strings.Contains(body, "<Deleted>") || strings.Contains(body, "<Error>") {
				t.Fatalf("empty batch produced entries: %s", body)
			}
			if len(be.deletedKeys) != 0 {
				t.Errorf("empty batch deleted keys: %v", be.deletedKeys)
			}
			if _, _, err := be.Get(ctx, "test-bucket", "ordinary/file.txt"); err != nil {
				t.Errorf("seeded object did not survive an empty batch: %v", err)
			}
			if _, ok := rec.Lookup("test-bucket", "ordinary/file.txt"); !ok {
				t.Errorf("manifest entry was tombstoned by an empty batch")
			}
		})
	}

	t.Run("empty body is malformed", func(t *testing.T) {
		be := &mockDeleteBackend{
			mockBackend: newMockBackend(),
			deletedKeys: make(map[string]bool),
		}
		cfg, _, cache, footerCache, km := testSetup(t)
		h := handlers.New(cfg, be, cache, footerCache, km, nil)

		req := httptest.NewRequest(http.MethodPost, "/test-bucket?delete",
			bytes.NewReader(nil))
		w := httptest.NewRecorder()
		h.DeleteObjects(w, req, "test-bucket")

		if w.Code != http.StatusBadRequest {
			t.Fatalf("non-XML body: expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "MalformedXML") {
			t.Errorf("non-XML body lacks MalformedXML code: %s", w.Body.String())
		}
	})
}
