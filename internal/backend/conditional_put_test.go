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
	"sync"
	"sync/atomic"
	"testing"
)

func TestB2PutIfAbsentSerializesHeadThenPut(t *testing.T) {
	var requests atomic.Int32
	var stored atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.Method {
		case http.MethodHead:
			if stored.Load() {
				w.Header().Set("Content-Length", "7")
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`)
		case http.MethodPut:
			if got := r.Header.Get("If-None-Match"); got != "" {
				t.Errorf("If-None-Match = %q, want empty for B2 compatibility", got)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read request: %v", err)
			}
			if string(body) != "payload" {
				t.Errorf("body = %q, want payload", body)
			}
			stored.Store(true)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("method = %s, want HEAD or PUT", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
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
	if err := b2.PutIfAbsent(context.Background(), "bucket", "key", strings.NewReader("payload"), 7, nil); err != nil {
		t.Fatalf("first PutIfAbsent: %v", err)
	}
	err = b2.PutIfAbsent(context.Background(), "bucket", "key", strings.NewReader("replacement"), 11, nil)
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("second PutIfAbsent error = %v, want ErrPreconditionFailed", err)
	}
	if requests.Load() != 3 {
		t.Fatalf("requests = %d, want 3 (HEAD, PUT, HEAD)", requests.Load())
	}
}

func TestB2PutIfAbsentAllowsOneConcurrentWriter(t *testing.T) {
	var stored atomic.Bool
	var puts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			if stored.Load() {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`)
		case http.MethodPut:
			puts.Add(1)
			stored.Store(true)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
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

	const writers = 16
	start := make(chan struct{})
	results := make(chan error, writers)
	var wg sync.WaitGroup
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- b2.PutIfAbsent(context.Background(), "bucket", "key", strings.NewReader("payload"), 7, nil)
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var created, rejected int
	for err := range results {
		switch {
		case err == nil:
			created++
		case errors.Is(err, ErrPreconditionFailed):
			rejected++
		default:
			t.Fatalf("PutIfAbsent returned unexpected error: %v", err)
		}
	}
	if created != 1 || rejected != writers-1 {
		t.Fatalf("created=%d rejected=%d, want 1 and %d", created, rejected, writers-1)
	}
	if puts.Load() != 1 {
		t.Fatalf("PUT requests = %d, want 1", puts.Load())
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
