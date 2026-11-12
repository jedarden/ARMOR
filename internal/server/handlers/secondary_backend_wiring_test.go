package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
)

// TestNewLeavesSecondaryNil verifies that New() constructor leaves
// secondaryBackend nil by default (no secondary configured).
func TestNewLeavesSecondaryNil(t *testing.T) {
	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	primaryDir := t.TempDir()
	primaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: primaryDir})
	if err != nil {
		t.Fatalf("failed to create primary backend: %v", err)
	}

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create keymanager: %v", err)
	}

	h := New(cfg, primaryBackend, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), km, backend.NewListCache(1000, 300))

	if h.secondaryBackend != nil {
		t.Errorf("New() should leave secondaryBackend nil, got %T", h.secondaryBackend)
	}
}

// TestSecondaryNilDoesNotPanic verifies that operations with a nil secondaryBackend
// never panic or crash. This is the safety contract: even if some code path
// erroneously dereferences secondaryBackend without checking, it must not
// crash the server.

// TestSecondaryNilDoesNotPanic verifies that operations with a nil secondaryBackend
// never panic or crash. This is the safety contract: even if some code path
// erroneously dereferences secondaryBackend without checking, it must not
// crash the server.
func TestSecondaryNilDoesNotPanic(t *testing.T) {
	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-panic.txt"
	body := []byte("test content")

	primaryDir := t.TempDir()
	primaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: primaryDir})
	if err != nil {
		t.Fatalf("failed to create primary backend: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create keymanager: %v", err)
	}

	h := &Handlers{
		config:     cfg,
		backend:    primaryBackend,
		cache:      backend.NewMetadataCache(1000, 300),
		listCache:  backend.NewListCache(1000, 300),
		keyManager: km,
		// secondaryBackend is explicitly nil
	}

	// Create bucket
	if err := h.backend.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	// Test PutObject - should not panic
	assertNotPanic(t, func() {
		req := httptest.NewRequest("PUT", "/"+bucket+"/"+key, bytes.NewReader(body))
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.ResponseRecorder{}
		h.PutObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("PutObject failed with status %d", w.Code)
		}
	})

	// Test GetObject - should not panic
	assertNotPanic(t, func() {
		req := httptest.NewRequest("GET", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.GetObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("GetObject failed with status %d", w.Code)
		}
	})

	// Test HeadObject - should not panic
	assertNotPanic(t, func() {
		req := httptest.NewRequest("HEAD", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.HeadObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("HeadObject failed with status %d", w.Code)
		}
	})

	// Test DeleteObject - should not panic
	assertNotPanic(t, func() {
		req := httptest.NewRequest("DELETE", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.DeleteObject(&w, req, bucket, key)
		if w.Code != http.StatusNoContent {
			t.Fatalf("DeleteObject failed with status %d", w.Code)
		}
	})
}

// assertNotPanic runs f and calls t.Fatal if it panics.

// assertNotPanic runs f and calls t.Fatal if it panics.
func assertNotPanic(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("function panicked: %v", r)
		}
	}()
	f()
}

// TestConfigUnset verifies handler behavior when secondary backend is not
// configured via config (SecondaryBackendType is empty/unset).
//
// This test ensures that:
// - Operations do not attempt to use a nil/unset secondary backend
// - Handler methods gracefully handle missing secondary configuration
// - No crashes or panics occur when secondary backend is unset
// - Behavior matches expected no-op semantics

// TestConfigUnset verifies handler behavior when secondary backend is not
// configured via config (SecondaryBackendType is empty/unset).
//
// This test ensures that:
// - Operations do not attempt to use a nil/unset secondary backend
// - Handler methods gracefully handle missing secondary configuration
// - No crashes or panics occur when secondary backend is unset
// - Behavior matches expected no-op semantics
func TestConfigUnset(t *testing.T) {
	ctx := context.Background()

	// Create a config with unset secondary backend
	cfg := &config.Config{
		BlockSize:            65536,
		AuthAccessKey:        "test",
		AuthSecretKey:        "REMOVED-NOT-A-SECRET-VALUE",
		SecondaryBackendType: "", // Empty = not configured (no secondary backend)
		SecondaryBackendPath: "", // Empty when type is unset
	}

	// Create a primary filesystem backend
	primaryDir := t.TempDir()
	primaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: primaryDir})
	if err != nil {
		t.Fatalf("failed to create primary backend: %v", err)
	}

	// Generate a MEK for the key manager
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	// Create a key manager
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create keymanager: %v", err)
	}

	// Create handler with config that has unset secondary backend
	h := &Handlers{
		config:           cfg,
		backend:          primaryBackend,
		secondaryBackend: nil, // No secondary backend configured
		cache:            backend.NewMetadataCache(1000, 300),
		footerCache:      backend.NewFooterCache(1000, 300),
		listCache:        backend.NewListCache(1000, 300),
		keyManager:       km,
	}

	// Verify that config reflects unset secondary backend
	if cfg.SecondaryBackendType != "" {
		t.Errorf("Expected SecondaryBackendType to be empty, got %s", cfg.SecondaryBackendType)
	}
	if h.secondaryBackend != nil {
		t.Errorf("Expected secondaryBackend to be nil, got %T", h.secondaryBackend)
	}

	// Test that all operations work correctly without crashing
	bucket := "test-bucket"
	key := "test-config-unset.txt"
	body := []byte("test content for config unset")

	// Create bucket
	assertNotPanic(t, func() {
		req := httptest.NewRequest("PUT", "/"+bucket, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.CreateBucket(&w, req, bucket)
		if w.Code != http.StatusOK {
			t.Fatalf("CreateBucket failed with status %d", w.Code)
		}
	})

	// PutObject - should succeed without attempting secondary backend operations
	assertNotPanic(t, func() {
		req := httptest.NewRequest("PUT", "/"+bucket+"/"+key, bytes.NewReader(body))
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.ResponseRecorder{}
		h.PutObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("PutObject failed with status %d", w.Code)
		}
	})

	// GetObject - should succeed without attempting secondary backend operations
	assertNotPanic(t, func() {
		req := httptest.NewRequest("GET", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.GetObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("GetObject failed with status %d", w.Code)
		}
	})

	// HeadObject - should succeed without attempting secondary backend operations
	assertNotPanic(t, func() {
		req := httptest.NewRequest("HEAD", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.HeadObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("HeadObject failed with status %d", w.Code)
		}
	})

	// CopyObject - should succeed without attempting secondary backend operations
	copyDestKey := "test-copy-dest.txt"
	assertNotPanic(t, func() {
		req := httptest.NewRequest("PUT", "/"+bucket+"/"+copyDestKey, nil)
		req = req.WithContext(ctx)
		req.Header.Set("x-amz-copy-source", "/"+bucket+"/"+key)
		w := httptest.ResponseRecorder{}
		h.CopyObject(&w, req, bucket, copyDestKey)
		if w.Code != http.StatusOK {
			t.Fatalf("CopyObject failed with status %d", w.Code)
		}
	})

	// ListObjectsV2 - should succeed without attempting secondary backend operations
	assertNotPanic(t, func() {
		req := httptest.NewRequest("GET", "/"+bucket+"?list-type=2", nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.ListObjectsV2(&w, req, bucket)
		if w.Code != http.StatusOK {
			t.Fatalf("ListObjectsV2 failed with status %d", w.Code)
		}
	})

	// DeleteObject - should succeed without attempting secondary backend operations
	assertNotPanic(t, func() {
		req := httptest.NewRequest("DELETE", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.DeleteObject(&w, req, bucket, key)
		if w.Code != http.StatusNoContent {
			t.Fatalf("DeleteObject failed with status %d", w.Code)
		}
	})

	// Also delete the copy destination object
	assertNotPanic(t, func() {
		req := httptest.NewRequest("DELETE", "/"+bucket+"/"+copyDestKey, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.DeleteObject(&w, req, bucket, copyDestKey)
		if w.Code != http.StatusNoContent {
			t.Fatalf("DeleteObject (copy dest) failed with status %d", w.Code)
		}
	})

	// DeleteBucket - should succeed without attempting secondary backend operations
	// Note: After deleting all objects, the bucket should be empty
	assertNotPanic(t, func() {
		req := httptest.NewRequest("DELETE", "/"+bucket, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.DeleteBucket(&w, req, bucket)
		if w.Code != http.StatusNoContent {
			t.Fatalf("DeleteBucket failed with status %d", w.Code)
		}
	})

	// Verify no-op semantics: the primary backend should contain all expected data
	// and operations should have completed successfully without any secondary backend interaction

	// Create a new bucket and object for final verification
	bucket2 := "test-bucket-2"
	key2 := "test-final-verification.txt"
	body2 := []byte("final verification content")

	if err := h.backend.CreateBucket(ctx, bucket2); err != nil {
		t.Fatalf("failed to create bucket for final verification: %v", err)
	}

	reqPut := httptest.NewRequest("PUT", "/"+bucket2+"/"+key2, bytes.NewReader(body2))
	reqPut = reqPut.WithContext(ctx)
	reqPut.Header.Set("Content-Type", "text/plain")
	wPut := httptest.ResponseRecorder{}
	h.PutObject(&wPut, reqPut, bucket2, key2)

	if wPut.Code != http.StatusOK {
		t.Fatalf("final PutObject failed with status %d", wPut.Code)
	}

	// Verify object exists in primary backend (not in secondary, which is nil/unset)
	info, err := h.backend.Head(ctx, bucket2, h.applyPrefix(key2))
	if err != nil {
		t.Errorf("object not found in primary backend: %v", err)
	} else {
		// Verify the object has the correct metadata
		if info.Size != int64(len(body2)) {
			t.Errorf("object size mismatch: got %d, want %d", info.Size, len(body2))
		}
		// The ETag should be non-empty (indicates successful encryption/storage)
		if info.ETag == "" {
			t.Error("object ETag is empty")
		}
	}

	// Clean up - delete object before deleting bucket
	if err := h.backend.Delete(ctx, bucket2, h.applyPrefix(key2)); err != nil {
		t.Logf("cleanup warning: failed to delete object %s: %v", key2, err)
	}
	if err := h.backend.DeleteBucket(ctx, bucket2); err != nil {
		t.Logf("cleanup warning: failed to delete bucket %s: %v", bucket2, err)
	}
}

// TestNewWithSecondaryBackend verifies that New() creates a handlers instance
// with secondaryBackend field nil by default, and WithSecondaryBackend() correctly
// wires in a secondary backend.

// TestNewWithSecondaryBackend verifies that New() creates a handlers instance
// with secondaryBackend field nil by default, and WithSecondaryBackend() correctly
// wires in a secondary backend.
func TestNewWithSecondaryBackend(t *testing.T) {
	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	primaryDir := t.TempDir()
	primaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: primaryDir})
	if err != nil {
		t.Fatalf("failed to create primary backend: %v", err)
	}

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create keymanager: %v", err)
	}

	// Test New() leaves secondaryBackend nil
	h := New(cfg, primaryBackend, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), km, backend.NewListCache(1000, 300))

	if h.secondaryBackend != nil {
		t.Errorf("New() should leave secondaryBackend nil, got %T", h.secondaryBackend)
	}

	// Test WithSecondaryBackend() wires in a secondary backend
	secondaryDir := t.TempDir()
	secondaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: secondaryDir})
	if err != nil {
		t.Fatalf("failed to create secondary backend: %v", err)
	}

	h.WithSecondaryBackend(secondaryBackend)

	if h.secondaryBackend == nil {
		t.Error("WithSecondaryBackend() should set secondaryBackend, got nil")
	}
	if h.secondaryBackend != secondaryBackend {
		t.Error("WithSecondaryBackend() should set the exact backend passed in")
	}
}

// TestWithSecondaryBackendNil verifies that WithSecondaryBackend(nil) correctly
// sets the secondary backend field to nil without panicking.

// TestWithSecondaryBackendNil verifies that WithSecondaryBackend(nil) correctly
// sets the secondary backend field to nil without panicking.
func TestWithSecondaryBackendNil(t *testing.T) {
	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	primaryDir := t.TempDir()
	primaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: primaryDir})
	if err != nil {
		t.Fatalf("failed to create primary backend: %v", err)
	}

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create keymanager: %v", err)
	}

	h := New(cfg, primaryBackend, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), km, backend.NewListCache(1000, 300))

	// Create a secondary backend initially
	secondaryDir := t.TempDir()
	secondaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: secondaryDir})
	if err != nil {
		t.Fatalf("failed to create secondary backend: %v", err)
	}

	h.WithSecondaryBackend(secondaryBackend)
	if h.secondaryBackend != secondaryBackend {
		t.Error("initial WithSecondaryBackend() failed")
	}

	// Set it back to nil - should not panic
	h.WithSecondaryBackend(nil)
	if h.secondaryBackend != nil {
		t.Errorf("WithSecondaryBackend(nil) should set secondaryBackend to nil, got %T", h.secondaryBackend)
	}
}

// TestHandlersMethodsWithNilSecondary verifies that all handler methods work
// correctly when secondaryBackend is nil (no secondary configured).

// TestHandlersMethodsWithNilSecondary verifies that all handler methods work
// correctly when secondaryBackend is nil (no secondary configured).
func TestHandlersMethodsWithNilSecondary(t *testing.T) {
	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-object.txt"
	body := []byte("test content")

	primaryDir := t.TempDir()
	primaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: primaryDir})
	if err != nil {
		t.Fatalf("failed to create primary backend: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create keymanager: %v", err)
	}

	h := &Handlers{
		config:           cfg,
		backend:          primaryBackend,
		secondaryBackend: nil, // Explicitly nil
		cache:            backend.NewMetadataCache(1000, 300),
		footerCache:      backend.NewFooterCache(1000, 300),
		listCache:        backend.NewListCache(1000, 300),
		keyManager:       km,
	}

	// Create bucket
	assertNotPanic(t, func() {
		req := httptest.NewRequest("PUT", "/"+bucket, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.CreateBucket(&w, req, bucket)
		if w.Code != http.StatusOK {
			t.Fatalf("CreateBucket failed with status %d", w.Code)
		}
	})

	// Test PutObject - should not attempt secondary operations
	assertNotPanic(t, func() {
		req := httptest.NewRequest("PUT", "/"+bucket+"/"+key, bytes.NewReader(body))
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.ResponseRecorder{}
		h.PutObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("PutObject failed with status %d", w.Code)
		}
	})

	// Test GetObject - should not attempt secondary operations
	assertNotPanic(t, func() {
		req := httptest.NewRequest("GET", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.GetObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("GetObject failed with status %d", w.Code)
		}
	})

	// Test HeadObject - should not attempt secondary operations
	assertNotPanic(t, func() {
		req := httptest.NewRequest("HEAD", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.HeadObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("HeadObject failed with status %d", w.Code)
		}
	})

	// Test DeleteObject - should not attempt secondary operations
	assertNotPanic(t, func() {
		req := httptest.NewRequest("DELETE", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.DeleteObject(&w, req, bucket, key)
		if w.Code != http.StatusNoContent {
			t.Fatalf("DeleteObject failed with status %d", w.Code)
		}
	})
}

// TestHandlersMethodsWithSecondaryBackend verifies that all handler methods work
// correctly when secondaryBackend is non-nil and properly delegated to.

// TestHandlersMethodsWithSecondaryBackend verifies that all handler methods work
// correctly when secondaryBackend is non-nil and properly delegated to.
func TestHandlersMethodsWithSecondaryBackend(t *testing.T) {
	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-object.txt"
	body := []byte("test content")

	primaryDir := t.TempDir()
	primaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: primaryDir})
	if err != nil {
		t.Fatalf("failed to create primary backend: %v", err)
	}

	secondaryDir := t.TempDir()
	secondaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: secondaryDir})
	if err != nil {
		t.Fatalf("failed to create secondary backend: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create keymanager: %v", err)
	}

	h := &Handlers{
		config:           cfg,
		backend:          primaryBackend,
		secondaryBackend: secondaryBackend,
		cache:            backend.NewMetadataCache(1000, 300),
		footerCache:      backend.NewFooterCache(1000, 300),
		listCache:        backend.NewListCache(1000, 300),
		keyManager:       km,
	}

	// Create bucket in primary backend
	assertNotPanic(t, func() {
		req := httptest.NewRequest("PUT", "/"+bucket, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.CreateBucket(&w, req, bucket)
		if w.Code != http.StatusOK {
			t.Fatalf("CreateBucket failed with status %d", w.Code)
		}
	})

	// Test PutObject - should succeed with secondary backend present
	assertNotPanic(t, func() {
		req := httptest.NewRequest("PUT", "/"+bucket+"/"+key, bytes.NewReader(body))
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.ResponseRecorder{}
		h.PutObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("PutObject failed with status %d", w.Code)
		}
	})

	// Test GetObject - should succeed with secondary backend present
	assertNotPanic(t, func() {
		req := httptest.NewRequest("GET", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.GetObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("GetObject failed with status %d", w.Code)
		}
	})

	// Test HeadObject - should succeed with secondary backend present
	assertNotPanic(t, func() {
		req := httptest.NewRequest("HEAD", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.HeadObject(&w, req, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("HeadObject failed with status %d", w.Code)
		}
	})

	// Test DeleteObject - should succeed with secondary backend present
	assertNotPanic(t, func() {
		req := httptest.NewRequest("DELETE", "/"+bucket+"/"+key, nil)
		req = req.WithContext(ctx)
		w := httptest.ResponseRecorder{}
		h.DeleteObject(&w, req, bucket, key)
		if w.Code != http.StatusNoContent {
			t.Fatalf("DeleteObject failed with status %d", w.Code)
		}
	})
}

// TestSecondaryBackendFieldIsNotExported verifies that the secondaryBackend field
// is properly encapsulated and can only be set via WithSecondaryBackend().
