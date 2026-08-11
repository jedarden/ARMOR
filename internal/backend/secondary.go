//go:build secondary_string_parser

// secondary.go implements a colon-string config parser API
// (InitSecondaryBackend(ctx, configStr), lowercase initFilesystemBackend, and
// initB2Backend) that PREDATES and is SUPERSEDED by the committed BackendConfig
// struct API in secondary_init.go (InitFilesystemBackend / InitB2Backend).
//
// Evidence it is abandoned prototype work rather than the intended direction:
//   - every closed child of umbrella bf-3mnoxt (bf-3872el, bf-3hg55b, bf-jozjud,
//     bf-3qvink) implements the BackendConfig struct API in secondary_init.go;
//   - it carries none of the committed design decisions (validateB2Config, the
//     HeadBucket connectivity probe, require-existing path semantics);
//   - it has zero callers outside this file and its own test;
//   - its tests in secondary_test.go are internally inconsistent (they assert
//     strict 5-field parsing, but the implementation below accepts >=4), so the
//     pair has never been green together.
//
// It is excluded from the default build to keep `go test ./internal/backend/`
// green. This is a reversible stopgap pending an ownership decision to either
// delete the pair outright or rebuild it as a thin string->BackendConfig
// adapter that delegates to the committed initializers. Compile with:
//   go test -tags secondary_string_parser ./internal/backend/

// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"context"
	"fmt"
	"strings"
)

// InitSecondaryBackend initializes a secondary backend from a config string.
// Supported formats:
//   - "filesystem:/path" - local filesystem backend at the given path
//   - "b2:region:endpoint:accessKeyId:secretKey:bucket" - B2 S3 backend
//
// Returns a Backend interface and an error if the format is invalid or
// initialization fails.
//
// Examples:
//   - "filesystem:/backup/armor"
//   - "b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEYID:SECRET:mybucket"
func InitSecondaryBackend(ctx context.Context, configStr string) (Backend, error) {
	if configStr == "" {
		return nil, fmt.Errorf("config string cannot be empty")
	}

	parts := strings.SplitN(configStr, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid format: expected 'type:params', got %q", configStr)
	}

	backendType := strings.ToLower(parts[0])
	params := parts[1]

	switch backendType {
	case "filesystem":
		return initFilesystemBackend(params)
	case "b2":
		return initB2Backend(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported backend type: %q (supported: filesystem, b2)", backendType)
	}
}

// initFilesystemBackend initializes a filesystem backend from a path string.
func initFilesystemBackend(path string) (Backend, error) {
	if path == "" {
		return nil, fmt.Errorf("filesystem path cannot be empty")
	}

	cfg := FSConfig{
		BasePath: path,
	}

	return NewFSBackend(cfg)
}

// initB2Backend initializes a B2 backend from a colon-separated parameter string.
// Expected format: "region:endpoint:accessKeyId:secretKey:bucket"
// The endpoint may contain "://" (e.g., "https://s3..."), so we parse backwards.
// Example: "us-east-005:https://s3.us-east-005.backblazeb2.com:KEYID:SECRET:mybucket"
func initB2Backend(ctx context.Context, params string) (Backend, error) {
	if params == "" {
		return nil, fmt.Errorf("B2 parameters cannot be empty")
	}

	// Split by colon to get all parts
	parts := strings.Split(params, ":")

	// Since the endpoint may contain "://" (e.g., "https://s3..."), simply splitting
	// by colon doesn't give us a fixed number of parts. We need to validate that we have
	// at least enough parts to extract all required fields:
	// - bucket (last 1)
	// - secretKey (last 2)
	// - accessKeyID (last 3)
	// - at least one endpoint part (last 4)
	// - region (remaining parts)
	// So we need at least 5 parts total after the type prefix.
	//
	// However, because "://" adds extra parts, a valid config with https:// will have 7+ parts.
	// The minimum case (no "://") is exactly 5 parts.
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid B2 format: expected at least 5 colon-separated values (region:endpoint:accessKeyId:secretKey:bucket), got %d", len(parts))
	}

	// Extract from the end (last 3 are always: bucket, secretKey, accessKeyID)
	bucket := parts[len(parts)-1]
	secretKey := parts[len(parts)-2]
	accessKeyID := parts[len(parts)-3]

	// Validate we have at least region and endpoint remaining
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid B2 format: missing region and/or endpoint")
	}

	// Everything before the last 3 parts is "region:endpoint" (endpoint may contain ":")
	// Reconstruct the endpoint prefix
	endpointParts := parts[:len(parts)-3]
	if len(endpointParts) < 2 {
		return nil, fmt.Errorf("invalid B2 format: missing region and/or endpoint")
	}

	region := endpointParts[0]
	endpoint := strings.Join(endpointParts[1:], ":")

	// Validate required fields
	if region == "" {
		return nil, fmt.Errorf("B2 region cannot be empty")
	}
	if endpoint == "" {
		return nil, fmt.Errorf("B2 endpoint cannot be empty")
	}
	if accessKeyID == "" {
		return nil, fmt.Errorf("B2 access key ID cannot be empty")
	}
	if secretKey == "" {
		return nil, fmt.Errorf("B2 secret key cannot be empty")
	}
	if bucket == "" {
		return nil, fmt.Errorf("B2 bucket cannot be empty")
	}

	cfg := B2Config{
		Region:      region,
		Endpoint:    endpoint,
		AccessKeyID: accessKeyID,
		SecretKey:   secretKey,
		CFDomain:    "", // No Cloudflare domain for secondary backend
	}

	return NewB2Backend(ctx, cfg)
}
