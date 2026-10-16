package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/keymanager"
)

func TestRotateKeyRejectsUnknownKeyID(t *testing.T) {
	defaultMEK := rotationTestMEK(0x55)
	km, err := keymanager.New(defaultMEK, nil, nil)
	if err != nil {
		t.Fatalf("New key manager: %v", err)
	}
	s := &Server{keyManager: km}
	req := httptest.NewRequest(http.MethodPost, "/admin/key/rotate?key-id=missing", nil)
	w := httptest.NewRecorder()
	s.rotateKey(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown key rotation status = %d, want 400; body %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("Unknown key ID")) {
		t.Errorf("unknown key rotation body = %q", w.Body.String())
	}
}

// noBackendCallsAllowed counts the rotation backend's read/write entry points.
// Its nil delegate makes any other backend call fail the test as well.
type noBackendCallsAllowed struct {
	backend.Backend
	calls int
}

func (b *noBackendCallsAllowed) List(context.Context, string, string, string, string, int) (*backend.ListResult, error) {
	b.calls++
	return nil, errors.New("unexpected backend List")
}

func (b *noBackendCallsAllowed) Put(context.Context, string, string, io.Reader, int64, map[string]string) error {
	b.calls++
	return errors.New("unexpected backend Put")
}

func (b *noBackendCallsAllowed) Copy(context.Context, string, string, string, string, map[string]string, bool) error {
	b.calls++
	return errors.New("unexpected backend Copy")
}

func TestRotateKeyRejectsBodyBeforeBackendAccess(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{name: "hex MEK", body: []byte(strings.Repeat("a", 64))},
		{name: "raw MEK", body: bytes.Repeat([]byte{0x42}, 32)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backendSpy := &noBackendCallsAllowed{}
			s := &Server{backend: backendSpy}
			req := httptest.NewRequest(http.MethodPost, "/admin/key/rotate", bytes.NewReader(tc.body))
			w := httptest.NewRecorder()
			s.rotateKey(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "Fingerprint mode") || !strings.Contains(w.Body.String(), "docs/key-rotation-runbook.md") {
				t.Errorf("response omits fingerprint mode or runbook: %q", w.Body.String())
			}
			if backendSpy.calls != 0 {
				t.Errorf("rejected body made %d backend calls, want zero", backendSpy.calls)
			}
		})
	}
}

func rotationTestMEK(value byte) []byte {
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = value
	}
	return mek
}
