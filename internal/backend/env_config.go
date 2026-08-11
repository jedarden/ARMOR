// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"fmt"
	"os"
	"strings"
)

// ParseSecondaryBackendEnv parses the ARMOR_SECONDARY_BACKEND environment variable
// and returns a BackendConfig for initializing a secondary backend.
//
// Supported formats:
//   - "filesystem:/path" - local filesystem backend at the given path
//   - "b2:region:endpoint:accessKeyId:secretKey:bucket" - B2 S3 backend
//
// When ARMOR_SECONDARY_BACKEND is unset or empty, it returns a zero BackendConfig
// (disabled state). When set, it parses the colon-separated format and validates
// that all required fields for the backend type are present.
//
// The endpoint may contain "://" (e.g., "https://s3.us-east-005.backblazeb2.com"),
// so B2 parsing extracts fields from the end (bucket is last, secret is second-to-last,
// keyId is third-to-last) and treats everything before that as region:endpoint.
//
// Returns a BackendConfig populated with parsed values, or an error if:
//   - the format is invalid (missing colon, insufficient parts)
//   - an unsupported backend type is specified
//   - required fields for the backend type are missing
func ParseSecondaryBackendEnv() (BackendConfig, error) {
	configStr := os.Getenv("ARMOR_SECONDARY_BACKEND")
	if configStr == "" {
		// Unset = disabled state
		return BackendConfig{}, nil
	}

	// Split into type and params
	parts := strings.SplitN(configStr, ":", 2)
	if len(parts) != 2 {
		return BackendConfig{}, fmt.Errorf("invalid ARMOR_SECONDARY_BACKEND format: expected 'type:params', got %q", configStr)
	}

	backendType := strings.ToLower(parts[0])
	params := parts[1]

	if params == "" {
		return BackendConfig{}, fmt.Errorf("ARMOR_SECONDARY_BACKEND params cannot be empty for type %q", backendType)
	}

	switch backendType {
	case "filesystem":
		return parseFilesystemBackend(params)
	case "b2":
		return parseB2Backend(params)
	default:
		return BackendConfig{}, fmt.Errorf("unsupported secondary backend type: %q (supported: filesystem, b2)", backendType)
	}
}

// parseFilesystemBackend parses a filesystem backend config from a path string.
// Example: "/backup/armor"
func parseFilesystemBackend(path string) (BackendConfig, error) {
	if path == "" {
		return BackendConfig{}, fmt.Errorf("filesystem path cannot be empty")
	}

	return BackendConfig{
		Type: "filesystem",
		Path: path,
	}, nil
}

// parseB2Backend parses a B2 backend config from a colon-separated parameter string.
// Expected format: "region:endpoint:accessKeyId:secretKey:bucket"
// The endpoint may contain "://" (e.g., "https://s3.us-east-005.backblazeb2.com"),
// so we parse backwards from the bucket.
//
// Example: "us-east-005:https://s3.us-east-005.backblazeb2.com:KEYID:SECRET:mybucket"
func parseB2Backend(params string) (BackendConfig, error) {
	if params == "" {
		return BackendConfig{}, fmt.Errorf("B2 parameters cannot be empty")
	}

	// Split by colon to get all parts
	parts := strings.Split(params, ":")

	// Since the endpoint may contain "://" (e.g., "https://s3..."), simply splitting
	// by colon doesn't give us a fixed number of parts. We need at least 5 parts total
	// after the type prefix:
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
		return BackendConfig{}, fmt.Errorf("invalid B2 format: expected at least 5 colon-separated values (region:endpoint:accessKeyId:secretKey:bucket), got %d", len(parts))
	}

	// Extract from the end (last 3 are always: bucket, secretKey, accessKeyID)
	bucket := parts[len(parts)-1]
	secretKey := parts[len(parts)-2]
	accessKeyID := parts[len(parts)-3]

	// Everything before the last 3 parts is "region:endpoint" (endpoint may contain ":")
	endpointParts := parts[:len(parts)-3]
	if len(endpointParts) < 2 {
		return BackendConfig{}, fmt.Errorf("invalid B2 format: missing region and/or endpoint")
	}

	region := endpointParts[0]
	endpoint := strings.Join(endpointParts[1:], ":")

	// Validate required fields
	if region == "" {
		return BackendConfig{}, fmt.Errorf("B2 region cannot be empty")
	}
	if endpoint == "" {
		return BackendConfig{}, fmt.Errorf("B2 endpoint cannot be empty")
	}
	if accessKeyID == "" {
		return BackendConfig{}, fmt.Errorf("B2 access key ID cannot be empty")
	}
	if secretKey == "" {
		return BackendConfig{}, fmt.Errorf("B2 secret key cannot be empty")
	}
	if bucket == "" {
		return BackendConfig{}, fmt.Errorf("B2 bucket cannot be empty")
	}

	return BackendConfig{
		Type:        "b2",
		Bucket:      bucket,
		Region:      region,
		Endpoint:    endpoint,
		AccessKeyID: accessKeyID,
		SecretKey:   secretKey,
	}, nil
}
