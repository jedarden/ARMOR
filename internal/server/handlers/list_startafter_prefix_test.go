// Package handlers_test verifies the ListObjectsV2 start-after cursor
// contract under ARMOR_PREFIX (armor-56ecc6b7).
//
// The real filesystem and B2 backends implement backend.StartAfterLister, so
// the handler forwards the cursor to the backend instead of filtering the
// page itself. Composing ARMOR_PREFIX onto an absent client cursor fabricated
// start-after=<ARMOR_PREFIX> on every first-page list. B2 honors that as a
// real cursor, which emptied the first page of aws s3 ls listings (exit 1
// with no output; integration-gate diagnostic on 69624a57). These tests pin
// the wire contract: absent client cursor in, empty backend cursor out; a
// cursor the client actually sent is composed onto the prefix.
package handlers_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/server/handlers"
)

// startAfterCaptureBackend records the cursor arguments each list call
// receives. It embeds mockBackend so every other Backend method behaves as
// before, and adds ListWithStartAfter, which moves the handler onto the
// StartAfterLister path the real filesystem and B2 backends take — the path
// mockBackend alone never exercises.
type startAfterCaptureBackend struct {
	*mockBackend

	mu                      sync.Mutex
	capturedPrefix          string
	capturedDelimiter       string
	capturedContToken       string
	capturedStartAfter      string
	listWithStartAfterCalls int
}

func (s *startAfterCaptureBackend) ListWithStartAfter(ctx context.Context, bucket, prefix, delimiter, continuationToken, startAfter string, maxKeys int) (*backend.ListResult, error) {
	s.mu.Lock()
	s.capturedPrefix = prefix
	s.capturedDelimiter = delimiter
	s.capturedContToken = continuationToken
	s.capturedStartAfter = startAfter
	s.listWithStartAfterCalls++
	s.mu.Unlock()
	return s.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

func (s *startAfterCaptureBackend) capture() (prefix, delimiter, contToken, startAfter string, calls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.capturedPrefix, s.capturedDelimiter, s.capturedContToken, s.capturedStartAfter, s.listWithStartAfterCalls
}

// TestListObjectsV2StartAfterUnderPrefix uploads through the handler with
// ARMOR_PREFIX configured and lists the way aws s3 ls does: prefix +
// delimiter, no start-after on the first page. The backend must receive an
// empty cursor for that page, and must receive the composed cursor when the
// client sends one.
func TestListObjectsV2StartAfterUnderPrefix(t *testing.T) {
	const (
		prefix      = "integration-tests/"
		listDir     = "awscli-test-1/"
		clientKey   = listDir + "file.txt"
		backendPath = prefix + listDir
	)
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, prefix)
	capture := &startAfterCaptureBackend{mockBackend: mb}
	h := handlers.New(cfg, capture, cache, footerCache, km, nil)

	plaintext := []byte("prefix cursor regression\n")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/"+clientKey, bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d, body: %s", w.Code, w.Body.String())
	}
	mb.mu.Lock()
	_, stored := mb.objects["test-bucket/"+backendPath+"file.txt"]
	mb.mu.Unlock()
	if !stored {
		t.Fatalf("object was not stored under the configured prefix")
	}

	// First page of `aws s3 ls s3://bucket/awscli-test-1/`: the client sends
	// no start-after, so the backend cursor must be empty — composing the
	// prefix onto the empty string fabricated start-after=<ARMOR_PREFIX>.
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix="+listDir+"&delimiter=/", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first-page list failed: status %d, body: %s", w.Code, w.Body.String())
	}
	gotPrefix, gotDelimiter, _, gotStartAfter, calls := capture.capture()
	if calls != 1 {
		t.Fatalf("expected the lister path to be taken once, got %d calls", calls)
	}
	if gotPrefix != backendPath {
		t.Errorf("backend prefix = %q, want %q", gotPrefix, backendPath)
	}
	if gotDelimiter != "/" {
		t.Errorf("backend delimiter = %q, want %q", gotDelimiter, "/")
	}
	if gotStartAfter != "" {
		t.Errorf("backend received fabricated start-after %q; client sent none, backend must receive an empty cursor", gotStartAfter)
	}
	if !strings.Contains(w.Body.String(), "<Key>"+clientKey+"</Key>") {
		t.Errorf("listing response missing the object key, got: %s", w.Body.String())
	}

	// A cursor the client actually sent is composed onto the prefix so the
	// backend cursor stays in stored-key space.
	req = httptest.NewRequest(http.MethodGet,
		"/test-bucket?list-type=2&prefix="+listDir+"&delimiter=/&start-after="+listDir+"aaa.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("client-cursor list failed: status %d, body: %s", w.Code, w.Body.String())
	}
	_, _, _, gotStartAfter, calls = capture.capture()
	if calls != 2 {
		t.Fatalf("expected the lister path to be taken twice, got %d calls", calls)
	}
	if want := backendPath + "aaa.txt"; gotStartAfter != want {
		t.Errorf("backend start-after = %q, want %q", gotStartAfter, want)
	}
}

// TestListObjectsV2StartAfterWithoutPrefix pins the same cursor contract for
// deployments without ARMOR_PREFIX: an absent client cursor stays empty and a
// client cursor passes through unchanged.
func TestListObjectsV2StartAfterWithoutPrefix(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetupWithPrefix(t, "")
	capture := &startAfterCaptureBackend{mockBackend: mb}
	h := handlers.New(cfg, capture, cache, footerCache, km, nil)

	plaintext := []byte("no-prefix cursor regression\n")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/plain/file.txt", bytes.NewReader(plaintext))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d, body: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=plain/&delimiter=/", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first-page list failed: status %d, body: %s", w.Code, w.Body.String())
	}
	_, _, _, gotStartAfter, calls := capture.capture()
	if calls != 1 {
		t.Fatalf("expected the lister path to be taken once, got %d calls", calls)
	}
	if gotStartAfter != "" {
		t.Errorf("backend start-after = %q, want empty", gotStartAfter)
	}

	req = httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2&prefix=plain/&delimiter=/&start-after=plain/aaa.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("client-cursor list failed: status %d, body: %s", w.Code, w.Body.String())
	}
	_, _, _, gotStartAfter, calls = capture.capture()
	if calls != 2 {
		t.Fatalf("expected the lister path to be taken twice, got %d calls", calls)
	}
	if want := "plain/aaa.txt"; gotStartAfter != want {
		t.Errorf("backend start-after = %q, want %q", gotStartAfter, want)
	}
}
