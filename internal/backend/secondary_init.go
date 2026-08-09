// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"fmt"
	"os"
)

// BackendConfig is the parsed representation of a secondary backend
// configuration. It is produced by the secondary-backend config parser and
// consumed by the per-type initializers (InitFilesystemBackend and, in a
// sibling task, the B2 initializer).
//
// Only the fields relevant to the configured backend type are populated:
//   - Type="filesystem" uses Path
//
// B2 fields are intentionally omitted here; they are added by the B2
// initialization task once its config format is settled.
type BackendConfig struct {
	// Type selects the backend implementation ("filesystem", "b2").
	Type string
	// Path is the root directory for a filesystem backend.
	Path string
}

// InitFilesystemBackend initializes a filesystem backend from a parsed
// BackendConfig (Type="filesystem", Path="/path").
//
// Unlike NewFSBackend, which creates its base directory on demand, this
// initializer validates that the operator-provided path already exists and is
// an accessible directory. A secondary replication target should fail loudly
// if its backing path is missing (e.g. a volume that failed to mount) rather
// than silently creating an empty directory and masking the problem.
//
// It returns the initialized backend on success, or an error if:
//   - the path is empty or missing,
//   - the path does not exist,
//   - the path exists but is not a directory,
//   - the path is not accessible (e.g. permission denied).
func InitFilesystemBackend(cfg BackendConfig) (Backend, error) {
	// Defensive type check: a non-empty type that isn't filesystem is a
	// programming error from the dispatcher. An empty type is allowed so the
	// function can be called with only a path.
	if cfg.Type != "" && cfg.Type != "filesystem" {
		return nil, fmt.Errorf("filesystem backend requires type %q, got %q", "filesystem", cfg.Type)
	}

	if cfg.Path == "" {
		return nil, fmt.Errorf("filesystem backend path is required")
	}

	info, err := os.Stat(cfg.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("filesystem backend path does not exist: %s", cfg.Path)
		}
		return nil, fmt.Errorf("filesystem backend path is inaccessible: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("filesystem backend path is not a directory: %s", cfg.Path)
	}

	// Path is validated as an existing directory; NewFSBackend's MkdirAll is a
	// no-op here but keeps the constructor's invariants intact.
	return NewFSBackend(FSConfig{BasePath: cfg.Path})
}
