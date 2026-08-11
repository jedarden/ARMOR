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
//   - B2_ENDPOINT: B2 S3 API endpoint (e.g., "https://s3.us-east-005.backblazeb2.com")
//   - B2_KEY_ID: B2 application key ID
//   - B2_KEY: B2 application key secret
//   - B2_BUCKET: Target bucket name
//
// It returns a BackendConfig populated with these values, or an error if
// required fields are missing or the endpoint URL is malformed.
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
	endpoint := os.Getenv("B2_ENDPOINT")
	keyID := os.Getenv("B2_KEY_ID")
	key := os.Getenv("B2_KEY")
	bucket := os.Getenv("B2_BUCKET")

	// If all are unset, return zero config (disabled state)
	if endpoint == "" && keyID == "" && key == "" && bucket == "" {
		return BackendConfig{}, nil
	}

	// Parse and validate endpoint
	if endpoint == "" {
		return cfg, fmt.Errorf("B2_ENDPOINT is required for secondary backend")
	}

	// Validate endpoint URL format
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return cfg, fmt.Errorf("B2_ENDPOINT must be a valid URL: %w", err)
	}

	// Ensure endpoint has a scheme
	if parsedURL.Scheme == "" {
		return cfg, fmt.Errorf("B2_ENDPOINT must include a scheme (e.g., https://)")
	}

	// Scheme must be http or https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return cfg, fmt.Errorf("B2_ENDPOINT scheme must be http or https, got %q", parsedURL.Scheme)
	}

	// Extract region from endpoint hostname
	// Expected format: s3.<region>.backblazeb2.com
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return cfg, fmt.Errorf("B2_ENDPOINT hostname is empty")
	}

	region, err := extractRegionFromEndpoint(hostname)
	if err != nil {
		return cfg, err
	}

	// Validate required fields
	if keyID == "" {
		return cfg, fmt.Errorf("B2_KEY_ID is required for secondary backend")
	}
	if key == "" {
		return cfg, fmt.Errorf("B2_KEY is required for secondary backend")
	}
	if bucket == "" {
		return cfg, fmt.Errorf("B2_BUCKET is required for secondary backend")
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
