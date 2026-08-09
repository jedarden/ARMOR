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
