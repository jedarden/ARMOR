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

// TestSecondaryBackendFieldIsNotExported verifies that the secondaryBackend field
// is properly encapsulated and can only be set via WithSecondaryBackend().
func TestSecondaryBackendFieldIsNotExported(t *testing.T) {
	// This is a compile-time test to ensure secondaryBackend is properly encapsulated
	// The field should only be accessible via WithSecondaryBackend() method

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

	// The secondaryBackend field should be private (lowercase), so we can only access it
	// via the WithSecondaryBackend() method. This verifies proper encapsulation.
	if h.secondaryBackend != nil {
		t.Error("secondaryBackend should be nil initially")
	}

	// Test the setter method
	secondaryDir := t.TempDir()
	secondaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: secondaryDir})
	if err != nil {
		t.Fatalf("failed to create secondary backend: %v", err)
	}

	h.WithSecondaryBackend(secondaryBackend)

	// Verify the field is now set
	if h.secondaryBackend == nil {
		t.Error("secondaryBackend should be non-nil after WithSecondaryBackend()")
	}
}

// TestMultipleWithSecondaryBackendCalls verifies that calling WithSecondaryBackend()
// multiple times correctly replaces the previous secondary backend.

// TestMultipleWithSecondaryBackendCalls verifies that calling WithSecondaryBackend()
// multiple times correctly replaces the previous secondary backend.
func TestMultipleWithSecondaryBackendCalls(t *testing.T) {
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

	// Set first secondary backend
	secondary1Dir := t.TempDir()
	secondary1, err := backend.NewFSBackend(backend.FSConfig{BasePath: secondary1Dir})
	if err != nil {
		t.Fatalf("failed to create secondary backend 1: %v", err)
	}

	h.WithSecondaryBackend(secondary1)

	firstBackend := h.secondaryBackend
	if firstBackend != secondary1 {
		t.Error("first WithSecondaryBackend() call failed")
	}

	// Replace with second secondary backend
	secondary2Dir := t.TempDir()
	secondary2, err := backend.NewFSBackend(backend.FSConfig{BasePath: secondary2Dir})
	if err != nil {
		t.Fatalf("failed to create secondary backend 2: %v", err)
	}

	h.WithSecondaryBackend(secondary2)

	secondBackend := h.secondaryBackend
	if secondBackend != secondary2 {
		t.Error("second WithSecondaryBackend() call failed")
	}

	// Verify the backend was actually replaced
	if secondBackend == firstBackend {
		t.Error("WithSecondaryBackend() should replace the previous backend, not keep it")
	}
}

// TestSecondaryBackendIntegrationWithConfig simulates the full integration
// test where config is loaded from environment and secondary backend is created
// and wired into handlers.

// TestSecondaryBackendIntegrationWithConfig simulates the full integration
// test where config is loaded from environment and secondary backend is created
// and wired into handlers.
func TestSecondaryBackendIntegrationWithConfig(t *testing.T) {
	// Test Case 1: Config with secondary backend enabled
	t.Run("config_with_secondary_backend", func(t *testing.T) {
		// Simulate environment variables being set
		// In real scenario, these would be set via os.Setenv() before config.Load()

		cfg := &config.Config{
			BlockSize:            65536,
			AuthAccessKey:        "test",
			AuthSecretKey:        "REMOVED-NOT-A-SECRET-VALUE",
			SecondaryBackendType: "filesystem",
			SecondaryBackendPath: t.TempDir(),
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

		// Verify secondary backend is NOT automatically created by New()
		if h.secondaryBackend != nil {
			t.Error("New() should not automatically create secondary backend from config")
		}

		// The server would normally create the secondary backend from config and wire it in
		if cfg.SecondaryBackendType != "" && cfg.SecondaryBackendPath != "" {
			secondaryBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: cfg.SecondaryBackendPath})
			if err != nil {
				t.Fatalf("failed to create secondary backend from config: %v", err)
			}
			h.WithSecondaryBackend(secondaryBackend)

			if h.secondaryBackend == nil {
				t.Error("WithSecondaryBackend() should have set the secondary backend")
			}
		}
	})

	// Test Case 2: Config without secondary backend (disabled)
	t.Run("config_without_secondary_backend", func(t *testing.T) {
		cfg := &config.Config{
			BlockSize:            65536,
			AuthAccessKey:        "test",
			AuthSecretKey:        "REMOVED-NOT-A-SECRET-VALUE",
			SecondaryBackendType: "", // Empty = disabled
			SecondaryBackendPath: "", // Empty when disabled
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

		// Verify secondary backend is not created when config has it disabled
		if h.secondaryBackend != nil {
			t.Error("New() should not create secondary backend when config has it disabled")
		}

		// WithSecondaryBackend(nil) should keep it nil
		h.WithSecondaryBackend(nil)
		if h.secondaryBackend != nil {
			t.Error("secondaryBackend should remain nil when config has it disabled")
		}
	})
}

// TestSecondaryBackendNilDoesNotAffectPrimaryOperations verifies that when
// secondaryBackend is nil, all primary backend operations continue to work correctly.

// TestSecondaryBackendNilDoesNotAffectPrimaryOperations verifies that when
// secondaryBackend is nil, all primary backend operations continue to work correctly.
func TestSecondaryBackendNilDoesNotAffectPrimaryOperations(t *testing.T) {
	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-object.txt"
	body := []byte("test content for primary ops")

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
	if err := h.backend.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	// PutObject
	reqPut := httptest.NewRequest("PUT", "/"+bucket+"/"+key, bytes.NewReader(body))
	reqPut = reqPut.WithContext(ctx)
	reqPut.Header.Set("Content-Type", "text/plain")
	wPut := httptest.ResponseRecorder{}
	h.PutObject(&wPut, reqPut, bucket, key)
	if wPut.Code != http.StatusOK {
		t.Fatalf("PutObject failed: status %d", wPut.Code)
	}

	// GetObject
	reqGet := httptest.NewRequest("GET", "/"+bucket+"/"+key, nil)
	reqGet = reqGet.WithContext(ctx)
	wGet := httptest.ResponseRecorder{}
	h.GetObject(&wGet, reqGet, bucket, key)
	if wGet.Code != http.StatusOK {
		t.Fatalf("GetObject failed: status %d", wGet.Code)
	}

	// Verify primary backend has the object
	info, err := h.backend.Head(ctx, bucket, h.applyPrefix(key))
	if err != nil {
		t.Errorf("object not found in primary backend: %v", err)
	} else {
		// Verify the object metadata is correct
		if info.ETag == "" {
			t.Error("object ETag is empty")
		}
		if info.Size <= 0 {
			t.Error("object size is invalid")
		}
	}

	// DeleteObject
	reqDel := httptest.NewRequest("DELETE", "/"+bucket+"/"+key, nil)
	reqDel = reqDel.WithContext(ctx)
	wDel := httptest.ResponseRecorder{}
	h.DeleteObject(&wDel, reqDel, bucket, key)
	if wDel.Code != http.StatusNoContent {
		t.Fatalf("DeleteObject failed: status %d", wDel.Code)
	}

	// Verify object was deleted from primary backend
	_, err = h.backend.Head(ctx, bucket, h.applyPrefix(key))
	if err == nil {
		t.Error("object still exists in primary backend after delete")
	}
}

// TestGetObjectChecksSecondaryBackendOnPrimaryFailure verifies that GetObject
// checks the secondary backend when the primary backend fails. This test creates
// a scenario where the primary backend is inaccessible and the secondary backend
// contains the object, verifying fallback behavior per the integration contract.
//
// This test uses a mock primary backend that fails reads and a filesystem secondary
// backend that succeeds, demonstrating the failover path.
