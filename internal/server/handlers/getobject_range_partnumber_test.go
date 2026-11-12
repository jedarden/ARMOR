package handlers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/server/handlers"
)

func TestGetObjectInvalidRangeReturns416(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	plaintext := bytes.Repeat([]byte("range"), 256)
	put := httptest.NewRecorder()
	h.HandleRoot(put, httptest.NewRequest(http.MethodPut, "/test-bucket/range-invalid", bytes.NewReader(plaintext)))
	if put.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", put.Code, put.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/test-bucket/range-invalid", nil)
	get.Header.Set("Range", "bytes=1280-2048")
	resp := httptest.NewRecorder()
	h.HandleRoot(resp, get)

	if resp.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("invalid range status = %d, want %d; body = %s", resp.Code, http.StatusRequestedRangeNotSatisfiable, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "<Code>InvalidRange</Code>") {
		t.Fatalf("invalid range body = %s, want InvalidRange", resp.Body.String())
	}
}
