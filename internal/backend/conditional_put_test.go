package backend

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestB2PutIfAbsentForwardsAtomicCondition(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if got := r.Header.Get("If-None-Match"); got != "*" {
			t.Errorf("If-None-Match = %q, want *", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}
		if string(body) != "payload" {
			t.Errorf("body = %q, want payload", body)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusPreconditionFailed)
		_, _ = io.WriteString(w, `<Error><Code>PreconditionFailed</Code><Message>exists</Message></Error>`)
	}))
	defer server.Close()

	b2, err := NewB2Backend(context.Background(), B2Config{
		Region:      "us-west-004",
		Endpoint:    server.URL,
		AccessKeyID: "test-access",
		SecretKey:   "test-secret",
	})
	if err != nil {
		t.Fatalf("NewB2Backend: %v", err)
	}
	err = b2.PutIfAbsent(context.Background(), "bucket", "key", strings.NewReader("payload"), 7, nil)
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("PutIfAbsent error = %v, want ErrPreconditionFailed", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want 1", requests.Load())
	}
}

func TestFSBackendPutIfAbsentDoesNotOverwrite(t *testing.T) {
	fs, err := NewFSBackend(FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	ctx := context.Background()
	if err := fs.PutIfAbsent(ctx, "bucket", "key", strings.NewReader("first"), 5, nil); err != nil {
		t.Fatalf("first PutIfAbsent: %v", err)
	}
	if err := fs.PutIfAbsent(ctx, "bucket", "key", strings.NewReader("second"), 6, nil); !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("second PutIfAbsent error = %v, want ErrPreconditionFailed", err)
	}
	stored, err := os.ReadFile(filepath.Join(fs.basePath, "bucket", "key"))
	if err != nil {
		t.Fatalf("read stored object: %v", err)
	}
	if string(stored) != "first" {
		t.Fatalf("stored body = %q, want first", stored)
	}
}
