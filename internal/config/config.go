// Package config handles ARMOR configuration via environment variables.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// KeyRoute represents a prefix-to-key mapping for multi-key support.
type KeyRoute struct {
	Prefix  string
	KeyName string
}

// ACLEntry represents a single ACL rule for a credential.
type ACLEntry struct {
	Bucket string // Bucket name, "*" for all buckets
	Prefix string // Key prefix, "*" or "" for any prefix
}

// Credential represents an ARMOR client credential with optional ACLs.
type Credential struct {
	AccessKey string
	SecretKey string
	ACLs      []ACLEntry // Empty means full access to configured bucket
}

// Config holds all ARMOR configuration.
type Config struct {
	// Server configuration
	Listen      string
	AdminListen string

	// B2 backend configuration
	B2Region          string
	B2Endpoint        string
	B2AccessKeyID     string
	B2SecretAccessKey string
	Bucket            string

	// Cloudflare download configuration
	CFDomain string

	// Encryption configuration
	MEK       []byte
	BlockSize int

	// Multi-key configuration
	NamedKeys map[string][]byte // Named MEKs (key name -> MEK)
	KeyRoutes []KeyRoute        // Prefix to key name mappings

	// Authentication credentials for ARMOR clients
	AuthAccessKey string
	AuthSecretKey string

	// Multi-credential support
	Credentials map[string]*Credential // Access key -> Credential

	// Writer ID for provenance chain
	WriterID string

	// Cache configuration
	CacheMaxEntries int
	CacheTTL        int

	// List cache configuration
	ListCacheMaxEntries int
	ListCacheTTL        int

	// Pre-signed URL configuration
	PresignSecret  []byte // Secret key for signing pre-signed URLs
	PresignBaseURL string // Base URL for pre-signed URLs (e.g., "https://armor.example.com/share")

	// Readiness probe configuration
	ReadyzCacheTTL int // Seconds to cache backend connectivity check (default 30)

	// Manifest index configuration (Phase 4)
	ManifestEnabled             bool
	ManifestPrefix              string
	ManifestCompactionInterval  int // seconds between automatic compactions
	ManifestCompactionThreshold int // delta entry count triggering early compaction
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Listen:      getEnv("ARMOR_LISTEN", "0.0.0.0:9000"),
		AdminListen: getEnv("ARMOR_ADMIN_LISTEN", "127.0.0.1:9001"),
	}

	// Required B2 configuration
	cfg.B2Region = os.Getenv("ARMOR_B2_REGION")
	if cfg.B2Region == "" {
		return nil, fmt.Errorf("ARMOR_B2_REGION is required")
	}

	cfg.B2Endpoint = os.Getenv("ARMOR_B2_ENDPOINT")
	if cfg.B2Endpoint == "" {
		cfg.B2Endpoint = fmt.Sprintf("https://s3.%s.backblazeb2.com", cfg.B2Region)
	}

	cfg.B2AccessKeyID = os.Getenv("ARMOR_B2_ACCESS_KEY_ID")
	if cfg.B2AccessKeyID == "" {
		return nil, fmt.Errorf("ARMOR_B2_ACCESS_KEY_ID is required")
	}

	cfg.B2SecretAccessKey = os.Getenv("ARMOR_B2_SECRET_ACCESS_KEY")
	if cfg.B2SecretAccessKey == "" {
		return nil, fmt.Errorf("ARMOR_B2_SECRET_ACCESS_KEY is required")
	}

	cfg.Bucket = os.Getenv("ARMOR_BUCKET")
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("ARMOR_BUCKET is required")
	}

	// Cloudflare domain (required)
	cfg.CFDomain = os.Getenv("ARMOR_CF_DOMAIN")
	if cfg.CFDomain == "" {
		return nil, fmt.Errorf("ARMOR_CF_DOMAIN is required")
	}

	// Master encryption key (required)
	mekHex := os.Getenv("ARMOR_MEK")
	if mekHex == "" {
		return nil, fmt.Errorf("ARMOR_MEK is required")
	}
	var err error
	cfg.MEK, err = hex.DecodeString(mekHex)
	if err != nil {
		return nil, fmt.Errorf("ARMOR_MEK must be hex-encoded: %w", err)
	}
	if len(cfg.MEK) != 32 {
		return nil, fmt.Errorf("ARMOR_MEK must be 32 bytes (64 hex chars), got %d bytes", len(cfg.MEK))
	}

	// Block size (default 64KB)
	cfg.BlockSize = getEnvInt("ARMOR_BLOCK_SIZE", 65536)
	if cfg.BlockSize < 4096 || (cfg.BlockSize&(cfg.BlockSize-1)) != 0 {
		return nil, fmt.Errorf("ARMOR_BLOCK_SIZE must be a power of 2 >= 4096")
	}

	// Auth credentials (generate random if not provided)
	cfg.AuthAccessKey = os.Getenv("ARMOR_AUTH_ACCESS_KEY")
	if cfg.AuthAccessKey == "" {
		cfg.AuthAccessKey = generateRandomKey(16)
	}
	cfg.AuthSecretKey = os.Getenv("ARMOR_AUTH_SECRET_KEY")
	if cfg.AuthSecretKey == "" {
		cfg.AuthSecretKey = generateRandomKey(32)
	}

	// Initialize credentials map with default credential
	cfg.Credentials = make(map[string]*Credential)
	cfg.Credentials[cfg.AuthAccessKey] = &Credential{
		AccessKey: cfg.AuthAccessKey,
		SecretKey: cfg.AuthSecretKey,
		ACLs:      nil, // nil means full access to configured bucket
	}

	// Load additional named credentials (ARMOR_AUTH_<NAME>_ACCESS_KEY, _SECRET_KEY, _ACL)
	if err := loadNamedCredentials(cfg); err != nil {
		return nil, err
	}

	// Writer ID (default to hostname)
	cfg.WriterID = os.Getenv("ARMOR_WRITER_ID")
	if cfg.WriterID == "" {
		cfg.WriterID, _ = os.Hostname()
		if cfg.WriterID == "" {
			cfg.WriterID = "armor-unknown"
		}
	}

	// Cache configuration
	cfg.CacheMaxEntries = getEnvInt("ARMOR_CACHE_MAX_ENTRIES", 10000)
	cfg.CacheTTL = getEnvInt("ARMOR_CACHE_TTL", 300)

	// List cache configuration
	cfg.ListCacheMaxEntries = getEnvInt("ARMOR_LIST_CACHE_MAX_ENTRIES", 1000)
	cfg.ListCacheTTL = getEnvInt("ARMOR_LIST_CACHE_TTL", 60)

	// Readiness probe configuration
	cfg.ReadyzCacheTTL = getEnvInt("ARMOR_READYZ_CACHE_TTL", 30)

	// Manifest index configuration
	manifestEnabledStr := os.Getenv("ARMOR_MANIFEST_ENABLED")
	cfg.ManifestEnabled = manifestEnabledStr != "false" && manifestEnabledStr != "0"
	cfg.ManifestPrefix = getEnv("ARMOR_MANIFEST_PREFIX", ".armor/manifest")
	cfg.ManifestCompactionInterval = getEnvInt("ARMOR_MANIFEST_COMPACTION_INTERVAL", 3600)
	cfg.ManifestCompactionThreshold = getEnvInt("ARMOR_MANIFEST_COMPACTION_THRESHOLD", 1000)

	// Pre-signed URL configuration
	presignSecretHex := os.Getenv("ARMOR_PRESIGN_SECRET")
	if presignSecretHex != "" {
		cfg.PresignSecret, err = hex.DecodeString(presignSecretHex)
		if err != nil {
			return nil, fmt.Errorf("ARMOR_PRESIGN_SECRET must be hex-encoded: %w", err)
		}
		if len(cfg.PresignSecret) < 32 {
			return nil, fmt.Errorf("ARMOR_PRESIGN_SECRET must be at least 32 bytes (64 hex chars)")
		}
	} else {
		// Use the auth secret key as the presign secret if not specified
		cfg.PresignSecret = []byte(cfg.AuthSecretKey)
	}
	cfg.PresignBaseURL = os.Getenv("ARMOR_PRESIGN_BASE_URL")
	if cfg.PresignBaseURL == "" {
		// Default to /share path on the main listener
		cfg.PresignBaseURL = "/share"
	}

	// Load named keys (ARMOR_MEK_<NAME>)
	cfg.NamedKeys = make(map[string][]byte)
	for _, env := range os.Environ() {
		// Look for ARMOR_MEK_<NAME> pattern
		if strings.HasPrefix(env, "ARMOR_MEK_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}
			name := strings.TrimPrefix(parts[0], "ARMOR_MEK_")
			name = strings.ToLower(name)
			if name == "" {
				continue
			}
			// Decode hex MEK
			mek, err := hex.DecodeString(parts[1])
			if err != nil {
				return nil, fmt.Errorf("ARMOR_MEK_%s must be hex-encoded: %w", name, err)
			}
			if len(mek) != 32 {
				return nil, fmt.Errorf("ARMOR_MEK_%s must be 32 bytes (64 hex chars), got %d bytes", name, len(mek))
			}
			cfg.NamedKeys[name] = mek
		}
	}

	// Load key routes (ARMOR_KEY_ROUTES)
	if routesStr := os.Getenv("ARMOR_KEY_ROUTES"); routesStr != "" {
		routes, err := parseKeyRoutes(routesStr)
		if err != nil {
			return nil, fmt.Errorf("ARMOR_KEY_ROUTES: %w", err)
		}
		cfg.KeyRoutes = routes
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func generateRandomKey(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random key: %v", err))
	}
	return hex.EncodeToString(b)
}

// parseKeyRoutes parses a key routes string.
// Format: "prefix1=key1,prefix2=key2,*=default"
// The * prefix is a catch-all that maps to the default key.
func parseKeyRoutes(routesStr string) ([]KeyRoute, error) {
	if routesStr == "" {
		return nil, nil
	}

	var routes []KeyRoute
	parts := strings.Split(routesStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid route format %q (expected prefix=keyname)", part)
		}

		prefix := strings.TrimSpace(kv[0])
		keyName := strings.TrimSpace(kv[1])

		if prefix == "" || keyName == "" {
			return nil, fmt.Errorf("invalid route %q (empty prefix or key name)", part)
		}

		// Handle wildcard - it maps to default key
		if prefix == "*" {
			prefix = ""
		}

		routes = append(routes, KeyRoute{
			Prefix:  prefix,
			KeyName: keyName,
		})
	}

	return routes, nil
}

// loadNamedCredentials loads additional named credentials from environment variables.
// Format: ARMOR_AUTH_<NAME>_ACCESS_KEY, ARMOR_AUTH_<NAME>_SECRET_KEY, ARMOR_AUTH_<NAME>_ACL
// Named credentials must have a non-empty NAME that doesn't conflict with default credential names.
func loadNamedCredentials(cfg *Config) error {
	// Collect all credential names
	credNames := make(map[string]bool)
	for _, env := range os.Environ() {
		// Look for ARMOR_AUTH_<NAME>_ACCESS_KEY pattern where NAME is not empty
		// and not one of the default credential env vars
		if strings.HasPrefix(env, "ARMOR_AUTH_") && strings.Contains(env, "_ACCESS_KEY=") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}
			// Extract name: ARMOR_AUTH_<NAME>_ACCESS_KEY -> <NAME>
			envKey := parts[0]
			// Skip the default credential env var
			if envKey == "ARMOR_AUTH_ACCESS_KEY" {
				continue
			}
			namePart := strings.TrimPrefix(envKey, "ARMOR_AUTH_")
			namePart = strings.TrimSuffix(namePart, "_ACCESS_KEY")
			if namePart == "" {
				continue
			}
			credNames[namePart] = true
		}
	}

	// Load each credential
	for name := range credNames {
		accessKey := os.Getenv("ARMOR_AUTH_" + name + "_ACCESS_KEY")
		secretKey := os.Getenv("ARMOR_AUTH_" + name + "_SECRET_KEY")
		aclStr := os.Getenv("ARMOR_AUTH_" + name + "_ACL")

		if accessKey == "" || secretKey == "" {
			return fmt.Errorf("ARMOR_AUTH_%s_ACCESS_KEY and ARMOR_AUTH_%s_SECRET_KEY are both required", name, name)
		}

		cred := &Credential{
			AccessKey: accessKey,
			SecretKey: secretKey,
		}

		// Parse ACL if provided
		if aclStr != "" {
			acls, err := parseACL(aclStr)
			if err != nil {
				return fmt.Errorf("ARMOR_AUTH_%s_ACL: %w", name, err)
			}
			cred.ACLs = acls
		}

		// Check for duplicate access key
		if _, exists := cfg.Credentials[accessKey]; exists {
			return fmt.Errorf("duplicate access key in ARMOR_AUTH_%s", name)
		}

		cfg.Credentials[accessKey] = cred
	}

	return nil
}

// parseACL parses an ACL string into ACL entries.
// Format: "bucket1:prefix1,bucket2:prefix2,bucket3:*"
// A bucket of "*" means all buckets.
// A prefix of "*" or "" means any prefix within the bucket.
func parseACL(aclStr string) ([]ACLEntry, error) {
	if aclStr == "" {
		return nil, nil
	}

	var entries []ACLEntry
	parts := strings.Split(aclStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split bucket:prefix
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid ACL entry %q (expected bucket:prefix)", part)
		}

		bucket := strings.TrimSpace(kv[0])
		prefix := strings.TrimSpace(kv[1])

		if bucket == "" {
			return nil, fmt.Errorf("invalid ACL entry %q (empty bucket)", part)
		}

		// Normalize wildcard prefix
		if prefix == "*" {
			prefix = ""
		}

		entries = append(entries, ACLEntry{
			Bucket: bucket,
			Prefix: prefix,
		})
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("ACL string contains no valid entries")
	}

	return entries, nil
}
