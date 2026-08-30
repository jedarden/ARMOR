// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ParseSecondaryBackendConfig parses environment variables to construct a
// BackendConfig for a secondary B2 backend. It reads the following environment
// variables:
//   - ARMOR_SECONDARY_B2_ENDPOINT: B2 S3 API endpoint (e.g., "https://s3.us-east-005.backblazeb2.com")
//   - ARMOR_SECONDARY_B2_KEY_ID: B2 application key ID
//   - ARMOR_SECONDARY_B2_KEY: B2 application key secret
//   - ARMOR_SECONDARY_B2_BUCKET: Target bucket name
//
// When all environment variables are unset, it returns a zero BackendConfig
// struct (not an error) — this represents the disabled state.
//
// The Region field is derived from the endpoint by extracting the region
// identifier from the hostname (e.g., "us-east-005" from
// "https://s3.us-east-005.backblazeb2.com").
func ParseSecondaryBackendConfig() (BackendConfig, error) {
	cfg := BackendConfig{
		Type: "b2",
	}

	// Read environment variables
	endpoint := os.Getenv("ARMOR_SECONDARY_B2_ENDPOINT")
	keyID := os.Getenv("ARMOR_SECONDARY_B2_KEY_ID")
	key := os.Getenv("ARMOR_SECONDARY_B2_KEY")
	bucket := os.Getenv("ARMOR_SECONDARY_B2_BUCKET")

	// If all are unset, return zero config (disabled state)
	if endpoint == "" && keyID == "" && key == "" && bucket == "" {
		return BackendConfig{}, nil
	}

	// Parse and validate endpoint
	if endpoint == "" {
		return cfg, fmt.Errorf("ARMOR_SECONDARY_B2_ENDPOINT is required for secondary backend")
	}

	// Validate endpoint URL format
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return cfg, fmt.Errorf("ARMOR_SECONDARY_B2_ENDPOINT must be a valid URL: %w", err)
	}

	// Ensure endpoint has a scheme
	if parsedURL.Scheme == "" {
		return cfg, fmt.Errorf("ARMOR_SECONDARY_B2_ENDPOINT must include a scheme (e.g., https://)")
	}

	// Scheme must be http or https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return cfg, fmt.Errorf("ARMOR_SECONDARY_B2_ENDPOINT scheme must be http or https, got %q", parsedURL.Scheme)
	}

	// Extract region from endpoint hostname
	// Expected format: s3.<region>.backblazeb2.com
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return cfg, fmt.Errorf("ARMOR_SECONDARY_B2_ENDPOINT hostname is empty")
	}

	region, err := extractRegionFromEndpoint(hostname)
	if err != nil {
		return cfg, err
	}

	// Validate required fields
	if keyID == "" {
		return cfg, fmt.Errorf("ARMOR_SECONDARY_B2_KEY_ID is required for secondary backend")
	}
	if key == "" {
		return cfg, fmt.Errorf("ARMOR_SECONDARY_B2_KEY is required for secondary backend")
	}
	if bucket == "" {
		return cfg, fmt.Errorf("ARMOR_SECONDARY_B2_BUCKET is required for secondary backend")
	}

	cfg.Endpoint = endpoint
	cfg.Region = region
	cfg.AccessKeyID = keyID
	cfg.SecretKey = key
	cfg.Bucket = bucket

	return cfg, nil
}

// extractRegionFromEndpoint extracts a B2 region identifier from a hostname.
// Expected format: s3.<region>.backblazeb2.com (e.g., "s3.us-east-005.backblazeb2.com")
// Returns the region string (e.g., "us-east-005") or an error if the hostname
// doesn't match the expected pattern.
func extractRegionFromEndpoint(hostname string) (string, error) {
	// Remove port if present
	if idx := strings.Index(hostname, ":"); idx != -1 {
		hostname = hostname[:idx]
	}

	// Expected format: s3.<region>.backblazeb2.com
	// Split by dots and validate
	parts := strings.Split(hostname, ".")
	if len(parts) < 4 {
		return "", fmt.Errorf("B2_ENDPOINT hostname must match format s3.<region>.backblazeb2.com, got %q", hostname)
	}

	// Check for expected domain pattern
	if parts[0] != "s3" {
		return "", fmt.Errorf("B2_ENDPOINT hostname must start with 's3.', got %q", hostname)
	}

	if parts[len(parts)-1] != "com" || parts[len(parts)-2] != "backblazeb2" {
		return "", fmt.Errorf("B2_ENDPOINT hostname must end with '.backblazeb2.com', got %q", hostname)
	}

	// Region is the second part (index 1)
	region := parts[1]
	if region == "" {
		return "", fmt.Errorf("B2_ENDPOINT region is empty in hostname %q", hostname)
	}

	return region, nil
}

// ParseSecondaryBackendConfigString parses a secondary backend configuration
// from a colon-separated string format. Supported formats:
//
//   - "filesystem:/path" - Filesystem backend at given path
//   - "b2:bucket:key:id:secret" - B2 backend with credentials
//
// The filesystem format requires a non-empty path. The B2 format requires
// exactly 5 colon-separated fields after the type prefix (bucket, key, id,
// secret). Empty strings return (BackendConfig{}, nil) to represent the
// disabled state. Unrecognized or malformed formats return an error.
//
// Examples:
//
//	"filesystem:/backup/armor" → BackendConfig{Type:"filesystem", Path:"/backup/armor"}
//	"b2:mybucket:appKeyId:accountId:appKey" → BackendConfig{Type:"b2", Bucket:"mybucket", KeyID:"appKeyId", ID:"accountId", Key:"appKey"}
//	"" → BackendConfig{}, nil (disabled)
func ParseSecondaryBackendConfigString(configStr string) (BackendConfig, error) {
	// Handle empty string gracefully (disabled backend)
	if configStr == "" {
		return BackendConfig{}, nil
	}

	// Split into type and params
	parts := strings.SplitN(configStr, ":", 2)
	if len(parts) != 2 {
		return BackendConfig{}, fmt.Errorf("invalid config format: expected 'type:params', got %q", configStr)
	}

	backendType := strings.ToLower(strings.TrimSpace(parts[0]))
	params := parts[1]

	if params == "" {
		return BackendConfig{}, fmt.Errorf("params cannot be empty for backend type %q", backendType)
	}

	switch backendType {
	case "filesystem":
		return parseFilesystemConfig(params)
	case "b2":
		return parseB2ConfigString(params)
	default:
		return BackendConfig{}, fmt.Errorf("unsupported backend type: %q (supported: filesystem, b2)", backendType)
	}
}

// parseFilesystemConfig parses a filesystem backend path.
// Format: "/path" - a non-empty filesystem path.
func parseFilesystemConfig(path string) (BackendConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return BackendConfig{}, fmt.Errorf("filesystem path cannot be empty")
	}

	return BackendConfig{
		Type: "filesystem",
		Path: path,
	}, nil
}

// parseB2ConfigString parses a B2 backend configuration from a colon-separated string.
// Format: "bucket:key:id:secret"
//   - bucket: B2 bucket name
//   - key: B2 application key ID
//   - id: B2 account ID (not used by BackendConfig, accepted for format compatibility)
//   - secret: B2 application key secret
//
// All four fields must be non-empty. Note that the 'id' field is accepted for format
// compatibility but is not stored in BackendConfig (the B2 backend derives account
// information from the credentials).
func parseB2ConfigString(params string) (BackendConfig, error) {
	// Split into exactly 4 parts: bucket:key:id:secret
	parts := strings.Split(params, ":")
	if len(parts) != 4 {
		return BackendConfig{}, fmt.Errorf("invalid B2 format: expected 'bucket:key:id:secret' (4 fields), got %d fields: %q", len(parts), params)
	}

	bucket := strings.TrimSpace(parts[0])
	keyID := strings.TrimSpace(parts[1])
	id := strings.TrimSpace(parts[2])
	secret := strings.TrimSpace(parts[3])

	// Validate all fields are non-empty
	if bucket == "" {
		return BackendConfig{}, fmt.Errorf("B2 bucket cannot be empty")
	}
	if keyID == "" {
		return BackendConfig{}, fmt.Errorf("B2 key (key ID) cannot be empty")
	}
	if id == "" {
		return BackendConfig{}, fmt.Errorf("B2 id (account ID) cannot be empty")
	}
	if secret == "" {
		return BackendConfig{}, fmt.Errorf("B2 secret (application key) cannot be empty")
	}

	return BackendConfig{
		Type:        "b2",
		Bucket:      bucket,
		AccessKeyID: keyID,
		SecretKey:   secret,
	}, nil
}
