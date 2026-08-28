// Package config handles ARMOR configuration via environment variables.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jedarden/armor/internal/acl"
)

// KeyRoute represents a prefix-to-key mapping for multi-key support.
type KeyRoute struct {
	Prefix  string
	KeyName string
}

// Credential represents an ARMOR client credential with optional ACLs.
type Credential struct {
	AccessKey string
	SecretKey string
	ACLs      []acl.ACLEntry // Empty means full access to configured bucket
}

// GetACLs returns the ACLs for this credential, implementing the interface
// expected by acl.CheckACL to avoid import cycles.
func (c *Credential) GetACLs() []acl.ACLEntry {
	return c.ACLs
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

	// Prefix for all keys (shared bucket support via ADR-001)
	// When set, all S3 keys are prefixed with this value before B2 operations
	// Normalized to exactly one trailing slash, no leading slash (e.g., "kalshi-tape/")
	// Empty/unset means no prefix is applied
	Prefix string

	// Cloudflare download configuration
	CFDomain string

	// CanaryDisabled skips the canary check in /readyz when true.
	// Set ARMOR_CANARY_DISABLED=true to disable the canary readiness gating.
	CanaryDisabled bool

	// Encryption configuration
	MEK       []byte
	BlockSize int

	// Compress enables zstd compression for single-PUT uploads.
	// When enabled, multipart uploads are rejected and range reads are unsupported.
	// See ADR-007.
	Compress bool

	// Read path configuration
	ReadConcurrency int // Maximum concurrent ranged GETs (default 16)

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

	// Dashboard authentication configuration
	// If DashboardUser and DashboardPass are set, HTTP Basic Auth is required
	// If DashboardToken is set, Bearer token authentication is required
	// If neither are set, dashboard is open (not recommended for production)
	DashboardUser  string
	DashboardPass  string
	DashboardToken string

	// AdminToken gates the admin API (all /admin/* routes and /armor/audit).
	// When set, every gated request must carry "Authorization: Bearer <token>"
	// and is compared in constant time. When unset, gated admin routes are
	// disabled (fail-closed) so the MEK cannot be exported or rotated without
	// an explicitly configured secret. Probes (/healthz, /readyz), /armor/canary,
	// /metrics, and /dashboard* (which carry their own auth) are unaffected.
	// See bead bf-5m9nde.
	AdminToken string

	// LogLevel controls the verbosity of application logging.
	// Valid values: "debug", "info", "warn", "error". Default: "info"
	// When set to "debug", HTTP request/response headers and bodies are logged.
	LogLevel string

	// Secondary backend configuration (ADR-006)
	// When set, enables async replication to a secondary backend
	SecondaryBackend     string // Backend identifier (e.g., "filesystem", "s3", "wasabi")
	SecondaryBackendType string // Type: "filesystem" (future: "s3", "wasabi")
	SecondaryBackendPath string // Path for filesystem backend (required when Type=filesystem)
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

	// Cloudflare domain (optional — empty string enables direct S3 fallback for downloads)
	cfg.CFDomain = os.Getenv("ARMOR_CF_DOMAIN")

	// Prefix for shared bucket support (ADR-001)
	// Normalize to exactly one trailing slash, no leading slash
	cfg.Prefix = normalizePrefix(os.Getenv("ARMOR_PREFIX"))

	// Canary disabled flag
	cfg.CanaryDisabled = os.Getenv("ARMOR_CANARY_DISABLED") == "true"

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

	// Compression (default disabled per ADR-007)
	cfg.Compress = os.Getenv("ARMOR_COMPRESS") == "true"

	// Number of ranged reads allowed in flight for a backend read.
	cfg.ReadConcurrency = getEnvInt("ARMOR_READ_CONCURRENCY", 16)
	if cfg.ReadConcurrency < 1 {
		return nil, fmt.Errorf("ARMOR_READ_CONCURRENCY must be at least 1")
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
			name := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(parts[0], "ARMOR_MEK_")))
			if name == "" {
				continue
			}
			if name == "default" {
				return nil, fmt.Errorf("ARMOR_MEK_DEFAULT is reserved; use ARMOR_MEK for the default key")
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

	// Dashboard authentication configuration
	cfg.DashboardUser = os.Getenv("ARMOR_DASHBOARD_USER")
	cfg.DashboardPass = os.Getenv("ARMOR_DASHBOARD_PASS")
	cfg.DashboardToken = os.Getenv("ARMOR_DASHBOARD_TOKEN")

	// Admin API bearer token. When set, all /admin/* routes (and /armor/audit)
	// require it; when unset, gated admin routes are disabled (fail-closed).
	cfg.AdminToken = os.Getenv("ARMOR_ADMIN_TOKEN")

	// Log level configuration (default: info)
	cfg.LogLevel = getEnv("ARMOR_LOG_LEVEL", "info")
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	// Secondary backend configuration (ADR-006)
	// Only enabled if ARMOR_SECONDARY_BACKEND_TYPE is set
	cfg.SecondaryBackendType = os.Getenv("ARMOR_SECONDARY_BACKEND_TYPE")
	if cfg.SecondaryBackendType != "" {
		// Validate backend type
		if cfg.SecondaryBackendType != "filesystem" {
			return nil, fmt.Errorf("ARMOR_SECONDARY_BACKEND_TYPE must be 'filesystem', got '%s'", cfg.SecondaryBackendType)
		}

		// For filesystem backend, path is required
		cfg.SecondaryBackendPath = os.Getenv("ARMOR_SECONDARY_BACKEND_PATH")
		if cfg.SecondaryBackendPath == "" {
			return nil, fmt.Errorf("ARMOR_SECONDARY_BACKEND_PATH is required when ARMOR_SECONDARY_BACKEND_TYPE=filesystem")
		}
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

// normalizePrefix normalizes a prefix string according to ADR-001.
// - Removes leading slashes
// - Ensures exactly one trailing slash
// - Empty string stays empty (no prefix)
// Examples:
//
//	"" → ""
//	"kalshi-tape" → "kalshi-tape/"
//	"kalshi-tape/" → "kalshi-tape/"
//	"/kalshi-tape" → "kalshi-tape/"
//	"/kalshi-tape/" → "kalshi-tape/"
//	"kalshi-tape//" → "kalshi-tape/"
func normalizePrefix(prefix string) string {
	if prefix == "" {
		return ""
	}

	// Remove leading slashes
	prefix = strings.TrimLeft(prefix, "/")

	// Remove all trailing slashes first
	prefix = strings.TrimRight(prefix, "/")

	// Add exactly one trailing slash if non-empty
	if prefix != "" {
		prefix += "/"
	}

	return prefix
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
		keyName := strings.ToLower(strings.TrimSpace(kv[1]))

		if prefix == "" || keyName == "" {
			return nil, fmt.Errorf("invalid route %q (empty prefix or key name)", part)
		}

		// A bare * is the catch-all route. A trailing /* is the documented
		// path-prefix notation, so normalize it to the prefix before the '*'.
		if prefix == "*" {
			prefix = ""
		} else if strings.HasSuffix(prefix, "/*") {
			prefix = strings.TrimSuffix(prefix, "*")
		} else if strings.Contains(prefix, "*") {
			return nil, fmt.Errorf("invalid route %q (wildcard must be at the end of the prefix)", part)
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
//
// An optional third segment restricts an entry to specific action verbs per
// ADR-012, e.g. "bucket:prefix:get+list". Verbs are space- or '+'-separated,
// matched case-sensitively against {get, put, delete, list}, and an unknown
// verb is a parse error. When the segment is absent (or empty) the entry
// permits all verbs — its Actions map stays nil — so existing two-segment
// "bucket:prefix" ACL strings keep their meaning.
func parseACL(aclStr string) ([]acl.ACLEntry, error) {
	if aclStr == "" {
		return nil, nil
	}

	var entries []acl.ACLEntry
	parts := strings.Split(aclStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split bucket:prefix[:actions]. The optional third segment carries
		// the action verbs (e.g. "get+list") per ADR-012.
		seg := strings.SplitN(part, ":", 3)
		if len(seg) < 2 {
			return nil, fmt.Errorf("invalid ACL entry %q (expected bucket:prefix)", part)
		}

		bucket := strings.TrimSpace(seg[0])
		prefix := strings.TrimSpace(seg[1])

		if bucket == "" {
			return nil, fmt.Errorf("invalid ACL entry %q (empty bucket)", part)
		}

		// Normalize wildcard prefix: a bare * is the catch-all (empty prefix),
		// a trailing /* is the documented path-prefix notation, and any other *
		// position is invalid (wildcard must be at the end).
		if prefix == "*" {
			prefix = ""
		} else if strings.HasSuffix(prefix, "/*") {
			prefix = strings.TrimSuffix(prefix, "/*") + "/"
		} else if strings.Contains(prefix, "*") {
			return nil, fmt.Errorf("invalid ACL entry %q (wildcard must be a bare * or trailing /*)", part)
		}

		entry := acl.ACLEntry{
			Bucket: bucket,
			Prefix: prefix,
		}

		// Optional third segment: action verbs. A present-but-empty segment is
		// treated like an absent one (no verbs → all permitted).
		if len(seg) == 3 {
			actions, err := parseActions(seg[2])
			if err != nil {
				return nil, fmt.Errorf("invalid ACL entry %q: %w", part, err)
			}
			entry.Actions = actions
		}

		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("ACL string contains no valid entries")
	}

	return entries, nil
}

// validActions is the closed set of action verbs an ACL entry may grant, per
// ADR-012. Membership is matched case-sensitively (lowercase canonical forms
// only); see the ACLEntry.Actions doc for the S3-operation each verb covers.
var validActions = map[string]bool{
	"get":    true,
	"put":    true,
	"delete": true,
	"list":   true,
}

// parseActions parses the optional third ACL segment ("get+list") into an
// Actions set. It accepts space- and/or '+'-separated verb names and validates
// each case-sensitively against validActions. It returns nil (meaning "all
// verbs permitted") when the segment carries no verbs, keeping "bucket:prefix"
// ACL strings backward compatible. An unknown verb yields an error.
func parseActions(verbStr string) (map[string]bool, error) {
	// Normalize '+' to a space so a single strings.Fields collapses both
	// separators and any surrounding/interleaved whitespace.
	verbs := strings.Fields(strings.ReplaceAll(verbStr, "+", " "))
	if len(verbs) == 0 {
		return nil, nil
	}

	actions := make(map[string]bool, len(verbs))
	for _, v := range verbs {
		if !validActions[v] {
			return nil, fmt.Errorf("invalid action verb %q (expected one of get, put, delete, list)", v)
		}
		actions[v] = true
	}
	return actions, nil
}
