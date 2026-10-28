package backend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	armorcrypto "github.com/jedarden/armor/internal/crypto"
)

// A real SDK retry must rewind and resend the encrypted envelope, including
// the trailer. The test server reads both attempts and rejects the first.
func TestB2PutReplayableEnvelopeRetry(t *testing.T) {
	const blockSize = 64 << 10
	plaintext := make([]byte, blockSize*3+17)
	for i := range plaintext {
		plaintext[i] = byte(i*37 + i/1024)
	}
	path := filepath.Join(t.TempDir(), "plaintext")
	if err := os.WriteFile(path, plaintext, 0o600); err != nil {
		t.Fatal(err)
	}
	spool, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer spool.Close()
	dek, iv := make([]byte, 32), make([]byte, 16)
	for i := range dek {
		dek[i] = byte(i + 1)
	}
	for i := range iv {
		iv[i] = byte(i + 3)
	}
	hash := sha256.Sum256(plaintext)
	header, err := armorcrypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, hash, armorcrypto.Version3)
	if err != nil {
		t.Fatal(err)
	}
	headerBytes, err := header.Encode()
	if err != nil {
		t.Fatal(err)
	}
	source, err := armorcrypto.NewReplayableEnvelopeReader(spool, headerBytes, dek, iv, blockSize, int64(len(plaintext)), armorcrypto.Version3)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var attempts [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.ContentLength != source.Len() || len(r.TransferEncoding) != 0 {
			t.Errorf("length=%d encoding=%v; want length=%d without chunking", r.ContentLength, r.TransferEncoding, source.Len())
		}
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("read PUT: %v", readErr)
		}
		mu.Lock()
		attempts = append(attempts, body)
		n := len(attempts)
		mu.Unlock()
		if n == 1 {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `<Error><Code>ServiceUnavailable</Code><Message>retry</Message></Error>`)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	b2, err := NewB2Backend(context.Background(), B2Config{
		Region: "us-west-004", Endpoint: server.URL,
		AccessKeyID: "test-access", SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b2.Put(context.Background(), "bucket", "key", source, source.Len(), nil); err != nil {
		t.Fatalf("Put with retry: %v", err)
	}
	if pos, err := source.Seek(0, io.SeekCurrent); err != nil || pos != source.Len() {
		t.Fatalf("successful SDK PUT left source at %d/%d: %v", pos, source.Len(), err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(attempts) != 2 || !bytes.Equal(attempts[0], attempts[1]) {
		t.Fatalf("got %d attempts; retry did not replay identical bytes", len(attempts))
	}
	if len(attempts[0]) != int(source.Len()) {
		t.Fatalf("received %d bytes, want %d", len(attempts[0]), source.Len())
	}
	// Independent reference writer checks the v3 HMAC trailer as well as
	// ciphertext and header, rather than merely comparing two bad retries.
	enc, err := armorcrypto.NewEncryptorWithVersion(dek, iv, blockSize, armorcrypto.Version3)
	if err != nil {
		t.Fatal(err)
	}
	var expected bytes.Buffer
	expected.Write(headerBytes)
	table, err := enc.EncryptV3Stream(bytes.NewReader(plaintext), &expected, int64(len(plaintext)), false)
	if err != nil {
		t.Fatal(err)
	}
	trailer, err := table.Encode()
	if err != nil {
		t.Fatal(err)
	}
	expected.Write(trailer)
	if !bytes.Equal(attempts[0], expected.Bytes()) {
		t.Fatal("SDK PUT bytes differ from reference v3 envelope")
	}
}
