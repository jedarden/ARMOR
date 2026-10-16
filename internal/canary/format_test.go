package canary

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// b2CanaryBackend keeps the bytes after asynchronous cleanup and applies B2's
// non-final part floor at CompleteMultipartUpload.
type b2CanaryBackend struct {
	*mockBackend
	mu          sync.Mutex
	envelope    []byte
	metadata    map[string]string
	sidecar     []byte
	partLengths []int
}

func (b *b2CanaryBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	b.mu.Lock()
	if strings.Contains(key, "/canary/") {
		b.envelope = bytes.Clone(data)
		b.metadata = meta
	}
	if strings.Contains(key, "/hmac/") {
		b.sidecar = bytes.Clone(data)
	}
	b.mu.Unlock()
	return b.mockBackend.Put(ctx, bucket, key, bytes.NewReader(data), size, meta)
}

func (b *b2CanaryBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	b.mockBackend.mu.Lock()
	lengths := make([]int, len(parts))
	for i, part := range parts {
		lengths[i] = len(b.multipartData[uploadID][part.PartNumber])
	}
	b.mockBackend.mu.Unlock()
	for i := 0; i < len(lengths)-1; i++ {
		if lengths[i] < b2MinPartSize {
			return "", fmt.Errorf("EntityTooSmall: part %d has %d bytes", i+1, lengths[i])
		}
	}
	b.mu.Lock()
	b.partLengths = lengths
	b.mu.Unlock()
	return b.mockBackend.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts)
}

func TestCanaryV3SinglePutRoundTrip(t *testing.T) {
	b := &b2CanaryBackend{mockBackend: newMockBackend()}
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatal(err)
	}
	m := NewMonitor(Config{Backend: b, Bucket: "probe", MEK: mek, BlockSize: crypto.DefaultBlockSize, InstanceID: "v3", FormatWriteVersion: 3})
	if m.formatWriteVersion != 3 || m.canarySize != 1024 {
		t.Fatalf("canary version=%d size=%d, want v3 and 1024", m.formatWriteVersion, m.canarySize)
	}
	result, err := m.check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusHealthy || !result.HMACVerified || !result.DecryptVerified {
		t.Fatalf("v3 canary result: %+v", result)
	}
	b.mu.Lock()
	envelope := bytes.Clone(b.envelope)
	metadata := b.metadata
	b.mu.Unlock()
	header, err := crypto.DecodeHeader(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if header.Version != crypto.Version3 || header.PlaintextSize != 1024 || metadata["x-amz-meta-armor-version"] != "3" {
		t.Fatalf("canary wrote version=%d size=%d metadata=%q", header.Version, header.PlaintextSize, metadata["x-amz-meta-armor-version"])
	}
	if len(envelope) != crypto.HeaderSize+1024+crypto.BlockTableEntrySize {
		t.Fatalf("v3 envelope has %d bytes, want header + plaintext + block table", len(envelope))
	}
	// A corrupted block table must fail the same authenticated read used by check.
	envelope[len(envelope)-crypto.BlockTableEntrySize] ^= 1
	if _, err := decryptCanaryEnvelope(envelope, metadata, mek); err == nil {
		t.Fatal("corrupt v3 HMAC was accepted")
	}
}

func TestCanaryGetObjectMismatchFailsReadiness(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatal(err)
	}
	called := false
	m := NewMonitor(Config{
		Backend:   newMockBackend(),
		Bucket:    "probe",
		MEK:       mek,
		BlockSize: crypto.DefaultBlockSize,
		ReadObject: func(_ context.Context, bucket, key string) ([]byte, error) {
			called = true
			if bucket != "probe" || !strings.HasPrefix(key, ".armor/canary/") {
				t.Errorf("GetObject addressed %q/%q", bucket, key)
			}
			return []byte("wrong plaintext"), nil
		},
	})
	if _, err := m.check(context.Background()); err == nil || !strings.Contains(err.Error(), "GetObject plaintext") {
		t.Fatalf("mismatched GetObject result did not fail canary: %v", err)
	}
	if !called {
		t.Fatal("normal GetObject callback was not called")
	}
}

func TestCanaryV3MultipartRoundTripAtB2Minimum(t *testing.T) {
	b := &b2CanaryBackend{mockBackend: newMockBackend()}
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatal(err)
	}
	m := NewMonitor(Config{Backend: b, Bucket: "probe", MEK: mek, BlockSize: crypto.DefaultBlockSize, InstanceID: "v3", FormatWriteVersion: 3})
	result, err := m.checkMultipart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusHealthy || result.MultipartHealthy != StatusHealthy || !result.HMACVerified || !result.DecryptVerified {
		t.Fatalf("v3 multipart canary result: %+v", result)
	}
	b.mu.Lock()
	lengths := append([]int(nil), b.partLengths...)
	sidecar := bytes.Clone(b.sidecar)
	b.mu.Unlock()
	if len(lengths) != 3 || lengths[0] < b2MinPartSize || lengths[1] < b2MinPartSize || lengths[2] == 0 {
		t.Fatalf("v3 multipart part sizes = %v, want three parts with B2-sized non-final parts", lengths)
	}
	if len(sidecar) < 2 || sidecar[0] != 0x1f || sidecar[1] != 0x8b {
		t.Fatal("v3 multipart sidecar was not stored as gzip")
	}
}
