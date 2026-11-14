package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/provenance"
)

func TestRotateKeyReportsUnwrapFailuresAsNonSuccess(t *testing.T) {
	const (
		bucket = "rotation-failure-bucket"
		key    = "cannot-unwrap.txt"
	)
	activeMEK := rotationTestMEK(0x55)
	unavailableMEK := rotationTestMEK(0x33)
	mock := newMockRotationBackend()
	putARMORObject(t, mock, bucket, key, []byte("ciphertext"), unavailableMEK, nil)

	beforeInfo, err := mock.Head(context.Background(), bucket, key)
	if err != nil {
		t.Fatalf("Head before rotation: %v", err)
	}
	before, ok := backend.ParseARMORMetadata(beforeInfo.Metadata)
	if !ok {
		t.Fatal("object metadata is not ARMOR metadata before rotation")
	}

	km, err := keymanager.New(activeMEK, nil, nil)
	if err != nil {
		t.Fatalf("New key manager: %v", err)
	}
	s := &Server{
		backend:    mock,
		config:     &config.Config{Bucket: bucket},
		keyManager: km,
		provenance: provenance.NewManager(mock, bucket, "rotation-failure-test"),
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/key/rotate", nil)
	w := httptest.NewRecorder()
	s.rotateKey(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("rotation HTTP status = %d, want 500; body %s", w.Code, w.Body.String())
	}
	var result RotationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode rotation result: %v; body %s", err, w.Body.String())
	}
	if result.FailedObjects != 1 || result.Status != "completed_with_failures" {
		t.Fatalf("rotation result status=%q failed_objects=%d; want completed_with_failures and 1", result.Status, result.FailedObjects)
	}
	if result.ProcessedObjects != 0 {
		t.Errorf("processed_objects = %d, want 0", result.ProcessedObjects)
	}
	if len(result.FailedKeys) != 1 || result.FailedKeys[0] != key {
		t.Errorf("failed_keys = %v, want [%s]", result.FailedKeys, key)
	}

	afterInfo, err := mock.Head(context.Background(), bucket, key)
	if err != nil {
		t.Fatalf("Head after rotation: %v", err)
	}
	after, ok := backend.ParseARMORMetadata(afterInfo.Metadata)
	if !ok {
		t.Fatal("object metadata is not ARMOR metadata after rotation")
	}
	if after.MEKFingerprint != before.MEKFingerprint {
		t.Errorf("stored fingerprint changed from %q to %q after failed rotation", before.MEKFingerprint, after.MEKFingerprint)
	}
	if !bytes.Equal(after.WrappedDEK, before.WrappedDEK) {
		t.Error("stored wrapped DEK changed after failed rotation")
	}
}

type headFailureRotationBackend struct {
	*mockRotationBackend
	failKey string
}

func (b *headFailureRotationBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	result, err := b.mockRotationBackend.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
	if err != nil {
		return nil, err
	}
	for i := range result.Objects {
		if result.Objects[i].Key == b.failKey {
			result.Objects[i].Metadata = nil
		}
	}
	return result, nil
}

func (b *headFailureRotationBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	if key == b.failKey {
		return nil, errors.New("injected Head failure")
	}
	return b.mockRotationBackend.Head(ctx, bucket, key)
}

func TestRotateKeyCountsMetadataHeadFailureAsFailure(t *testing.T) {
	const (
		bucket = "rotation-head-failure-bucket"
		key    = "head-fails.txt"
	)
	activeMEK := rotationTestMEK(0x55)
	oldMEK := rotationTestMEK(0x33)
	mock := newMockRotationBackend()
	putARMORObject(t, mock, bucket, key, []byte("ciphertext"), oldMEK, nil)
	backendWithHeadFailure := &headFailureRotationBackend{mockRotationBackend: mock, failKey: key}
	km, err := keymanager.New(activeMEK, nil, nil, map[string][]byte{"default": oldMEK})
	if err != nil {
		t.Fatalf("New key manager: %v", err)
	}
	s := &Server{
		backend:    backendWithHeadFailure,
		config:     &config.Config{Bucket: bucket},
		keyManager: km,
		provenance: provenance.NewManager(mock, bucket, "rotation-head-failure-test"),
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/key/rotate", nil)
	w := httptest.NewRecorder()
	s.rotateKey(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("rotation HTTP status = %d, want 500; body %s", w.Code, w.Body.String())
	}
	var result RotationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode rotation result: %v; body %s", err, w.Body.String())
	}
	if result.FailedObjects != 1 || result.Status != "completed_with_failures" {
		t.Fatalf("rotation result status=%q failed_objects=%d; want completed_with_failures and 1", result.Status, result.FailedObjects)
	}
	if result.ProcessedObjects != 0 || len(result.FailedKeys) != 1 || result.FailedKeys[0] != key {
		t.Errorf("rotation result processed=%d failed_keys=%v, want 0 and [%s]", result.ProcessedObjects, result.FailedKeys, key)
	}
}

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
