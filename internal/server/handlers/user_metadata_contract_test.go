package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/server/handlers"
)

func TestUserMetadataSurvivesPutHeadGetOverwriteAndLegacyIndex(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	counting := &headCountingBackend{mockBackend: mb}
	index := newMockManifestRecorder()
	h := handlers.New(cfg, counting, cache, footerCache, km, nil)
	h.WithManifest(index)

	const path = "/test-bucket/user-metadata.bin"
	put := func(payload, revision string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(payload))
		req.Header.Set("x-amz-meta-revision", revision)
		req.Header.Set("x-amz-meta-armor-private", "must-not-echo")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("PUT returned %d: %s", w.Code, w.Body.String())
		}
	}
	check := func(method, rangeHeader, revision string, wantStatus int) {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		if rangeHeader != "" {
			req.Header.Set("Range", rangeHeader)
		}
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != wantStatus {
			t.Fatalf("%s %q returned %d, want %d: %s", method, rangeHeader, w.Code, wantStatus, w.Body.String())
		}
		if got := w.Header().Get("x-amz-meta-revision"); got != revision {
			t.Errorf("%s %q revision = %q, want %q", method, rangeHeader, got, revision)
		}
		if got := w.Header().Get("x-amz-meta-armor-private"); got != "" {
			t.Errorf("%s %q leaked reserved metadata: %q", method, rangeHeader, got)
		}
	}

	put("first-version", "1")
	counting.headCalls.Store(0)
	check(http.MethodHead, "", "1", http.StatusOK)
	if got := counting.headCalls.Load(); got != 0 {
		t.Errorf("new index entry required %d backend HEADs, want 0", got)
	}
	check(http.MethodGet, "", "1", http.StatusOK)
	check(http.MethodGet, "bytes=1-3", "1", http.StatusPartialContent)

	put("second-version", "2")
	check(http.MethodHead, "", "2", http.StatusOK)
	check(http.MethodGet, "", "2", http.StatusOK)

	// An index entry loaded from an older delta lacks the new field. Its
	// stored ciphertext metadata still has to be returned by HEAD.
	entry, ok := index.Lookup("test-bucket", "user-metadata.bin")
	if !ok {
		t.Fatal("overwrite did not update manifest index")
	}
	legacy := *entry
	legacy.UserMetadata = nil
	index.seed("test-bucket", "user-metadata.bin", &legacy)
	counting.headCalls.Store(0)
	check(http.MethodHead, "", "2", http.StatusOK)
	if got := counting.headCalls.Load(); got == 0 {
		t.Error("legacy index entry did not fall back to stored object metadata")
	}
}
