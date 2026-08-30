package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/keymanager"
)

func TestPerKeyRotationLeavesOtherRoutedKeysUntouched(t *testing.T) {
	oldDefault := rotationTestMEK(0x11)
	oldPII := rotationTestMEK(0x22)
	newDefault := rotationTestMEK(0x33)
	newPII := rotationTestMEK(0x44)
	mock := newMockRotationBackend()
	const bucket = "test-bucket"

	defaultMeta, err := createTestARMORObject(oldDefault, bucket, "public/report.json", []byte("public"))
	if err != nil {
		t.Fatalf("create default object: %v", err)
	}
	piiMeta, err := createTestARMORObject(oldPII, bucket, "data/pii/customer.json", []byte("private"))
	if err != nil {
		t.Fatalf("create PII object: %v", err)
	}
	piiMeta["x-amz-meta-armor-key-id"] = "pii"

	var defaultBody, piiBody bytes.Buffer
	defaultBody.WriteString("public")
	piiBody.WriteString("private")
	if err := mock.Put(context.Background(), bucket, "public/report.json", &defaultBody, int64(defaultBody.Len()), defaultMeta); err != nil {
		t.Fatalf("put default object: %v", err)
	}
	if err := mock.Put(context.Background(), bucket, "data/pii/customer.json", &piiBody, int64(piiBody.Len()), piiMeta); err != nil {
		t.Fatalf("put PII object: %v", err)
	}

	defaultBefore := wrappedDEKFromMetadata(t, mock, bucket, "public/report.json")
	piiBefore := wrappedDEKFromMetadata(t, mock, bucket, "data/pii/customer.json")

	piiRotator := NewKeyRotatorForKey(mock, bucket, "pii", oldPII, newPII, nil, nil)
	result, err := piiRotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("PII rotation: %v", err)
	}
	if result.ProcessedObjects != 1 {
		t.Fatalf("PII rotation processed %d objects, want 1", result.ProcessedObjects)
	}

	defaultAfterPIIRotation := wrappedDEKFromMetadata(t, mock, bucket, "public/report.json")
	if !bytes.Equal(defaultAfterPIIRotation, defaultBefore) {
		t.Error("default-key object changed during PII-key rotation")
	}
	piiAfter := wrappedDEKFromMetadata(t, mock, bucket, "data/pii/customer.json")
	if bytes.Equal(piiAfter, piiBefore) {
		t.Error("PII object wrapped DEK did not change during PII-key rotation")
	}
	if _, err := crypto.UnwrapDEK(newPII, piiAfter); err != nil {
		t.Fatalf("PII object does not unwrap with rotated PII key: %v", err)
	}
	if _, err := crypto.UnwrapDEK(oldDefault, defaultAfterPIIRotation); err != nil {
		t.Fatalf("default object no longer unwraps with its original key: %v", err)
	}

	defaultRotator := NewKeyRotatorForKey(mock, bucket, "default", oldDefault, newDefault, nil, nil)
	result, err = defaultRotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("default rotation: %v", err)
	}
	if result.ProcessedObjects != 1 {
		t.Fatalf("default rotation processed %d objects, want 1", result.ProcessedObjects)
	}
	defaultAfter := wrappedDEKFromMetadata(t, mock, bucket, "public/report.json")
	if _, err := crypto.UnwrapDEK(newDefault, defaultAfter); err != nil {
		t.Fatalf("default object does not unwrap with rotated default key: %v", err)
	}
	if _, err := crypto.UnwrapDEK(newPII, wrappedDEKFromMetadata(t, mock, bucket, "data/pii/customer.json")); err != nil {
		t.Fatalf("PII object was affected by default rotation: %v", err)
	}
}

func TestRotateKeyRejectsUnknownKeyID(t *testing.T) {
	defaultMEK := rotationTestMEK(0x55)
	km, err := keymanager.New(defaultMEK, nil, nil)
	if err != nil {
		t.Fatalf("New key manager: %v", err)
	}
	s := &Server{keyManager: km}
	req := httptest.NewRequest(http.MethodPost, "/admin/key/rotate?key-id=missing", bytes.NewReader(rotationTestMEK(0x66)))
	w := httptest.NewRecorder()
	s.rotateKey(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown key rotation status = %d, want 400; body %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("Unknown key ID")) {
		t.Errorf("unknown key rotation body = %q", w.Body.String())
	}
}

func wrappedDEKFromMetadata(t *testing.T, b *mockRotationBackend, bucket, key string) []byte {
	t.Helper()
	info, err := b.Head(context.Background(), bucket, key)
	if err != nil {
		t.Fatalf("head %s: %v", key, err)
	}
	value := info.Metadata["x-amz-meta-armor-wrapped-dek"]
	wrapped, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("decode wrapped DEK for %s: %v", key, err)
	}
	return wrapped
}

func rotationTestMEK(value byte) []byte {
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = value
	}
	return mek
}
