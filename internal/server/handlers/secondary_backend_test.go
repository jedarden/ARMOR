package handlers

import (
	"testing"

	"github.com/jedarden/armor/internal/backend"
)

// TestSecondaryBackendWiring verifies the ADR-006 secondary backend is exposed
// on the Handlers struct and is populated only when configured.
//
// This is the Handlers-level counterpart of server.TestSecondaryBackendInitialization:
// that test drives the config → Server construction path and asserts on
// srv.secondaryBackend; this one asserts the field reaches Handlers and is nil
// by default. It is a white-box test (package handlers) so it can read the
// unexported secondaryBackend field directly, exactly as the server test reads
// the unexported Server field.
func TestSecondaryBackendWiring(t *testing.T) {
	// Default: no secondary backend wired — replication must be a no-op (nil).
	h := &Handlers{}
	if h.secondaryBackend != nil {
		t.Fatalf("expected secondaryBackend to be nil by default, got %T", h.secondaryBackend)
	}

	// Wire a real filesystem secondary backend (the only implemented ADR-006
	// type) and assert it is stored on the struct with the concrete type intact.
	dir := t.TempDir()
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: dir})
	if err != nil {
		t.Fatalf("failed to create filesystem secondary backend: %v", err)
	}

	h.WithSecondaryBackend(fsBackend)
	if h.secondaryBackend == nil {
		t.Fatal("expected secondaryBackend to be set after WithSecondaryBackend, got nil")
	}
	if _, ok := h.secondaryBackend.(*backend.FSBackend); !ok {
		t.Errorf("expected secondary backend to be *backend.FSBackend, got %T", h.secondaryBackend)
	}

	// Wiring nil must keep replication disabled (no-op), matching the
	// ARMOR_SECONDARY_BACKEND_TYPE-unset case the server passes through.
	h.WithSecondaryBackend(nil)
	if h.secondaryBackend != nil {
		t.Errorf("expected secondaryBackend to be nil after WithSecondaryBackend(nil), got %T", h.secondaryBackend)
	}
}

// TestSecondaryBackendIdempotency verifies that calling WithSecondaryBackend
// multiple times replaces the backend rather than appending or accumulating.
// This is the idempotency contract: the last call wins, and there is never more
// than one secondary backend wired in.
func TestSecondaryBackendIdempotency(t *testing.T) {
	h := &Handlers{}

	// Wire first backend
	dir1 := t.TempDir()
	backend1, err := backend.NewFSBackend(backend.FSConfig{BasePath: dir1})
	if err != nil {
		t.Fatalf("failed to create first backend: %v", err)
	}

	h.WithSecondaryBackend(backend1)
	if h.secondaryBackend != backend1 {
		t.Error("first backend was not set correctly")
	}

	// Wire second backend - should replace, not append
	dir2 := t.TempDir()
	backend2, err := backend.NewFSBackend(backend.FSConfig{BasePath: dir2})
	if err != nil {
		t.Fatalf("failed to create second backend: %v", err)
	}

	h.WithSecondaryBackend(backend2)
	if h.secondaryBackend != backend2 {
		t.Errorf("second backend did not replace first; got %T, want *backend.FSBackend", h.secondaryBackend)
	}
	if h.secondaryBackend == backend1 {
		t.Error("WithSecondaryBackend did not replace backend1 with backend2")
	}

	// Wire nil - should replace with nil (disable replication)
	h.WithSecondaryBackend(nil)
	if h.secondaryBackend != nil {
		t.Errorf("expected nil after WithSecondaryBackend(nil), got %T", h.secondaryBackend)
	}

	// Wire again - should work after nil too
	dir3 := t.TempDir()
	backend3, err := backend.NewFSBackend(backend.FSConfig{BasePath: dir3})
	if err != nil {
		t.Fatalf("failed to create third backend: %v", err)
	}

	h.WithSecondaryBackend(backend3)
	if h.secondaryBackend != backend3 {
		t.Error("third backend was not set after nil")
	}
}
