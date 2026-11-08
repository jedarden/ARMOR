package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/server/handlers"
)

func TestObjectLockRequestsReturnNotImplementedBeforeBackend(t *testing.T) {
	cfg, backend, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, backend, cache, footerCache, km, nil)

	for _, tc := range []struct {
		name   string
		header string
		value  string
	}{
		{name: "mode", header: "x-amz-object-lock-mode", value: "COMPLIANCE"},
		{name: "retain until date", header: "x-amz-object-lock-retain-until-date", value: "2026-12-31T00:00:00Z"},
		{name: "legal hold", header: "x-amz-object-lock-legal-hold", value: "ON"},
	} {
		t.Run("header-"+tc.name, func(t *testing.T) {
			key := strings.ReplaceAll(tc.name, " ", "-")
			req := httptest.NewRequest(http.MethodPut, "/test-bucket/header-"+key, strings.NewReader("must not be stored"))
			req.Header.Set(tc.header, tc.value)
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
			assertObjectLockNotImplemented(t, w)
		})
	}

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "get bucket configuration", method: http.MethodGet, path: "/test-bucket?object-lock"},
		{name: "put bucket configuration", method: http.MethodPut, path: "/test-bucket?object-lock"},
		{name: "get retention", method: http.MethodGet, path: "/test-bucket/object?retention"},
		{name: "put retention", method: http.MethodPut, path: "/test-bucket/object?retention"},
		{name: "get legal hold", method: http.MethodGet, path: "/test-bucket/object?legal-hold"},
		{name: "put legal hold", method: http.MethodPut, path: "/test-bucket/object?legal-hold"},
	} {
		t.Run("subresource-"+tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("must not be forwarded"))
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
			assertObjectLockNotImplemented(t, w)
		})
	}

	if len(backend.objects) != 0 {
		t.Fatalf("unsupported Object Lock requests stored objects: %v", backend.objects)
	}
}

func assertObjectLockNotImplemented(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501; body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "<Code>NotImplemented</Code>") {
		t.Fatalf("body = %s, want NotImplemented error code", w.Body.String())
	}
}
