// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"fmt"
	"os"
)

// BackendConfig is the parsed representation of a secondary backend
// configuration. It is produced by the secondary-backend config parser and
// consumed by the per-type initializers (InitFilesystemBackend and
// InitB2Backend).
//
// Only the fields relevant to the configured backend type are populated:
//   - Type="filesystem" uses Path
//   - Type="b2" uses Bucket, Region, Endpoint, AccessKeyID, and SecretKey
//
// Region and Endpoint are carried here rather than derived or defaulted by
// InitB2Backend: both are operator-provided and account-specific (B2 exposes
// multiple regions, and the S3 endpoint embeds the region), so neither has a
// sensible universal default. They map directly onto B2Config for
// NewB2Backend. The Cloudflare egress domain (B2Config.CFDomain) is
// intentionally absent: a secondary replication target downloads objects for
// verification rather than via the free-egress CDN, so InitB2Backend leaves
// CFDomain empty.
type BackendConfig struct {
	// Type selects the backend implementation ("filesystem", "b2").
	Type string
	// Path is the root directory for a filesystem backend.
	Path string

	// Bucket is the target bucket for a B2 backend. Unlike B2Config (where the
	// bucket is a per-operation parameter), a secondary backend replicates to a
	// single fixed bucket, so it lives in the config rather than each call.
	Bucket string
	// Region is the B2 region of the target bucket (e.g. "us-east-005").
	Region string
	// Endpoint is the B2 S3 API endpoint (e.g.
	// "https://s3.us-east-005.backblazeb2.com").
	Endpoint string
	// AccessKeyID is the B2 application key ID.
	AccessKeyID string
	// SecretKey is the B2 application key secret.
	SecretKey string
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

// validateB2Config validates that a BackendConfig carries every field a B2
// backend needs. It is pure validation — no network or SDK calls — so it can
// run before NewB2Backend's credential load and endpoint resolution, failing
// fast on a misconfigured secondary target instead of surfacing an opaque AWS
// SDK error on the first operation.
//
// Each check returns an error that names the offending field so an operator
// can pinpoint and fix the missing config value. It returns nil only when
// Bucket, Region, Endpoint, AccessKeyID, and SecretKey are all non-empty.
//
// The Type field is intentionally not validated here: type dispatch is the
// caller's responsibility (InitB2Backend, added in a follow-up), mirroring how
// InitFilesystemBackend handles its own type check. This function validates
// only the B2-specific parameters.
func validateB2Config(cfg BackendConfig) error {
	if cfg.Bucket == "" {
		return fmt.Errorf("B2 backend bucket is required")
	}
	if cfg.Region == "" {
		return fmt.Errorf("B2 backend region is required")
	}
	if cfg.Endpoint == "" {
		return fmt.Errorf("B2 backend endpoint is required")
	}
	if cfg.AccessKeyID == "" {
		return fmt.Errorf("B2 backend access key ID is required")
	}
	if cfg.SecretKey == "" {
		return fmt.Errorf("B2 backend secret key is required")
	}
	return nil
}
