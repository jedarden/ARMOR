// Focused conformance coverage for CopyObject's metadata directive and
// same-source/destination rejection (armor-116ce8f3). The integration legs
// in tests/integration/s3_copyobject_test.go pin the same behaviors against
// a live endpoint, but they only run under credentials in the
// armor-integration WorkflowTemplate; these handler-level tests pin the
// contract on every -short run so a regression cannot wait for that gate.
package handlers_test

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/server/handlers"
)

// copyConformanceErrorCode extracts <Code> from an S3 error body.
func copyConformanceErrorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var e struct {
		Code string `xml:"Code"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("parse error body %q: %v", w.Body.String(), err)
	}
	return e.Code
}

// TestCopyObjectSelfCopyRejected pins that a copy whose source names the
// request's own destination fails with InvalidRequest before any backend
// work, for both accepted x-amz-copy-source spellings, and that the stored
// object is untouched by the rejected attempt.
func TestCopyObjectSelfCopyRejected(t *testing.T) {
	cfg, _, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, newMockBackend(), cache, footerCache, mek, nil)

	srcContent := []byte("content that must survive a rejected self-copy")
	put := httptest.NewRequest(http.MethodPut, "/test-bucket/self-copy.txt", bytes.NewReader(srcContent))
	put.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, put)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT source failed: status %d, body %s", w.Code, w.Body.String())
	}

	for _, source := range []string{"/test-bucket/self-copy.txt", "test-bucket/self-copy.txt"} {
		req := httptest.NewRequest(http.MethodPut, "/test-bucket/self-copy.txt", nil)
		req.Header.Set("x-amz-copy-source", source)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("self-copy via %q: status %d, want 400 (body %s)", source, w.Code, w.Body.String())
			continue
		}
		if code := copyConformanceErrorCode(t, w); code != "InvalidRequest" {
			t.Errorf("self-copy via %q: error code %q, want InvalidRequest", source, code)
		}
	}

	// The rejected copy must not have disturbed the stored object.
	get := httptest.NewRequest(http.MethodGet, "/test-bucket/self-copy.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, get)
	if w.Code != http.StatusOK {
		t.Fatalf("GET after rejected self-copy: status %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), srcContent) {
		t.Error("object content changed under a rejected self-copy")
	}
}

// TestCopyObjectMetadataDirectiveCopyCarriesUserMetadata pins the COPY
// directive (the default and the explicit spelling) carrying the source's
// x-amz-meta-* keys to the destination through the ARMOR re-wrap, which
// rebuilds the metadata map and would otherwise drop them.
func TestCopyObjectMetadataDirectiveCopyCarriesUserMetadata(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	srcContent := []byte("content whose user metadata must survive the copy")
	put := httptest.NewRequest(http.MethodPut, "/test-bucket/meta-src.txt", bytes.NewReader(srcContent))
	put.Header.Set("Content-Type", "text/plain")
	put.Header.Set("x-amz-meta-project", "ARMOR")
	put.Header.Set("x-amz-meta-environment", "conformance")
	w := httptest.NewRecorder()
	h.HandleRoot(w, put)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT source failed: status %d, body %s", w.Code, w.Body.String())
	}

	cases := []struct {
		name      string
		dstKey    string
		directive string // empty = header absent (the default)
	}{
		{name: "default directive", dstKey: "meta-dst-default.txt"},
		{name: "explicit COPY", dstKey: "meta-dst-copy.txt", directive: "COPY"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPut, "/test-bucket/"+tc.dstKey, nil)
		req.Header.Set("x-amz-copy-source", "/test-bucket/meta-src.txt")
		if tc.directive != "" {
			req.Header.Set("x-amz-metadata-directive", tc.directive)
		}
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("copy (%s): status %d, body %s", tc.name, w.Code, w.Body.String())
			continue
		}

		get := httptest.NewRequest(http.MethodGet, "/test-bucket/"+tc.dstKey, nil)
		wg := httptest.NewRecorder()
		h.HandleRoot(wg, get)
		if wg.Code != http.StatusOK {
			t.Errorf("GET destination (%s): status %d", tc.name, wg.Code)
			continue
		}
		if !bytes.Equal(wg.Body.Bytes(), srcContent) {
			t.Errorf("destination content (%s) does not match source", tc.name)
		}
		if got := wg.Header().Get("X-Amz-Meta-Project"); got != "ARMOR" {
			t.Errorf("destination X-Amz-Meta-Project (%s) = %q, want %q", tc.name, got, "ARMOR")
		}
		if got := wg.Header().Get("X-Amz-Meta-Environment"); got != "conformance" {
			t.Errorf("destination X-Amz-Meta-Environment (%s) = %q, want %q", tc.name, got, "conformance")
		}

		// The re-wrapped ARMOR fields must have moved with the copy even
		// while the client keys were carried.
		mb.mu.Lock()
		dstMeta := mb.meta["test-bucket/"+tc.dstKey]
		mb.mu.Unlock()
		if dstMeta["x-amz-meta-armor-wrapped-dek"] == "" {
			t.Errorf("destination (%s) missing re-wrapped ARMOR metadata", tc.name)
		}
	}
}

// TestCopyObjectMetadataDirectiveReplaceSwapsUserMetadata pins that REPLACE
// takes the request's x-amz-meta-* set and drops the source's stale keys.
func TestCopyObjectMetadataDirectiveReplaceSwapsUserMetadata(t *testing.T) {
	cfg, _, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, newMockBackend(), cache, footerCache, mek, nil)

	srcContent := []byte("content whose stale metadata must not survive REPLACE")
	put := httptest.NewRequest(http.MethodPut, "/test-bucket/replace-src.txt", bytes.NewReader(srcContent))
	put.Header.Set("Content-Type", "text/plain")
	put.Header.Set("x-amz-meta-stage", "dev")
	put.Header.Set("x-amz-meta-legacy", "stale")
	w := httptest.NewRecorder()
	h.HandleRoot(w, put)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT source failed: status %d, body %s", w.Code, w.Body.String())
	}

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/replace-dst.txt", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/replace-src.txt")
	req.Header.Set("x-amz-metadata-directive", "REPLACE")
	req.Header.Set("x-amz-meta-stage", "prod")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("copy (REPLACE): status %d, body %s", w.Code, w.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/test-bucket/replace-dst.txt", nil)
	wg := httptest.NewRecorder()
	h.HandleRoot(wg, get)
	if wg.Code != http.StatusOK {
		t.Fatalf("GET destination: status %d", wg.Code)
	}
	if got := wg.Header().Get("X-Amz-Meta-Stage"); got != "prod" {
		t.Errorf("destination X-Amz-Meta-Stage = %q, want %q (the request's value)", got, "prod")
	}
	if got := wg.Header().Get("X-Amz-Meta-Legacy"); got != "" {
		t.Errorf("destination X-Amz-Meta-Legacy = %q, want empty — REPLACE must drop source keys absent from the request", got)
	}
}
