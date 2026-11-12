package server

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// The "B2 key management disabled (application key is bucket-scoped)" line
// is B2-specific guidance: it tells a B2 operator why the key-management
// admin routes answer 503. On a filesystem backend (the demo) the same line
// reads like a fault in a server that never had a B2 account, so server.New
// must not emit it there.

// captureServerNewStdout runs fn with the package's os.Stdout swapped for a
// pipe and returns everything written to the captured stream. server.New
// binds its logger to os.Stdout at construction, so this is the stream its
// startup lines land on. Lines the background manifest load writes after fn
// returns may be lost to the closed pipe; the lines logged synchronously
// inside fn — the ones under test — are all captured, because the pipe
// buffers them until the write end closes.
func captureServerNewStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create capture pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = writer
	fn()
	os.Stdout = old
	writer.Close()
	captured, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	reader.Close()
	return string(captured)
}

func TestServerNewFilesystemBackendEmitsNoB2KeyManagementLine(t *testing.T) {
	setTestEnv(t,
		"ARMOR_BACKEND", "filesystem",
		"ARMOR_FS_PATH", t.TempDir(),
		"ARMOR_BUCKET", "demo-bucket",
		"ARMOR_MEK", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
		"ARMOR_AUTH_ACCESS_KEY", "test-access-key",
		"ARMOR_AUTH_SECRET_KEY", "test-secret-key",
	)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load filesystem config: %v", err)
	}

	var srv *Server
	out := captureServerNewStdout(t, func() {
		srv, err = New(cfg)
	})
	if err != nil {
		t.Fatalf("create filesystem server: %v", err)
	}
	t.Cleanup(func() {
		srv.StopManifestWriter()
		srv.StopManifestCompactor()
		srv.StopAuthFileWatcher()
	})

	if strings.Contains(out, "B2 key management") {
		t.Errorf("filesystem deployment logged a B2-only line:\n%s", out)
	}
}
