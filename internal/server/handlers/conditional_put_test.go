package handlers_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/server/handlers"
)

type conditionalMockBackend struct {
	*mockBackend
	conditionalCalls atomic.Int32
}

func (m *conditionalMockBackend) PutIfAbsent(_ context.Context, bucket, key string, body io.Reader, _ int64, meta map[string]string) error {
	m.conditionalCalls.Add(1)
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	objectKey := bucket + "/" + key
	if _, exists := m.objects[objectKey]; exists {
		return backend.ErrPreconditionFailed
	}
	m.objects[objectKey] = data
	m.meta[objectKey] = meta
	return nil
}

func TestPutObjectIfNoneMatchCreateOnlySmallAndStreaming(t *testing.T) {
	for _, test := range []struct {
		name      string
		streaming bool
	}{
		{name: "small"},
		{name: "streaming", streaming: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, base, cache, footerCache, km := testSetup(t)
			storage := &conditionalMockBackend{mockBackend: base}
			h := handlers.New(cfg, storage, cache, footerCache, km, nil)

			put := func(body string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPut, "/test-bucket/immutable", strings.NewReader(body))
				req.Header.Set("If-None-Match", "*")
				if test.streaming {
					req.ContentLength = -1
				}
				w := httptest.NewRecorder()
				h.HandleRoot(w, req)
				return w
			}

			if w := put("first"); w.Code != http.StatusOK {
				t.Fatalf("first PUT status = %d, body = %s", w.Code, w.Body.String())
			}
			w := put("second")
			if w.Code != http.StatusPreconditionFailed {
				t.Fatalf("second PUT status = %d, want 412; body = %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "<Code>PreconditionFailed</Code>") {
				t.Fatalf("second PUT did not return S3 PreconditionFailed: %s", w.Body.String())
			}
			if storage.conditionalCalls.Load() != 2 {
				t.Fatalf("conditional calls = %d, want 2", storage.conditionalCalls.Load())
			}

			get := httptest.NewRequest(http.MethodGet, "/test-bucket/immutable", nil)
			getW := httptest.NewRecorder()
			h.HandleRoot(getW, get)
			if getW.Code != http.StatusOK || getW.Body.String() != "first" {
				t.Fatalf("stored object changed: status=%d body=%q", getW.Code, getW.Body.String())
			}
		})
	}
}

func TestPutObjectRejectsUnsupportedIfNoneMatchValue(t *testing.T) {
	cfg, base, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, base, cache, footerCache, km, nil)
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/key", strings.NewReader("body"))
	req.Header.Set("If-None-Match", `"etag"`)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}

func TestPutObjectDoesNotEmulateConditionalWrite(t *testing.T) {
	cfg, base, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, base, cache, footerCache, km, nil)
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/key", strings.NewReader("body"))
	req.Header.Set("If-None-Match", "*")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501; body = %s", w.Code, w.Body.String())
	}
	if _, exists := base.objects["test-bucket/key"]; exists {
		t.Fatal("conditional write fell back to a racy ordinary Put")
	}
}

var _ backend.ConditionalPutBackend = (*conditionalMockBackend)(nil)
