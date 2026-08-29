// Package config handles ARMOR configuration via environment variables.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/acl"
)

// KeyRoute represents a prefix-to-key mapping for multi-key support.
type KeyRoute struct {
	Prefix  string
	KeyName string
}

// CredentialSource represents where a credential was loaded from.
type CredentialSource string

const (
	// CredentialSourceEnv indicates the credential was loaded from environment variables.
	CredentialSourceEnv CredentialSource = "env"
	// CredentialSourceFile indicates the credential was loaded from ARMOR_AUTH_FILE.
	CredentialSourceFile CredentialSource = "file"
)

// Credential represents an ARMOR client credential with optional ACLs.
type Credential struct {
	AccessKey string
	SecretKey string
	ACLs      []acl.ACLEntry // Empty means full access to configured bucket
	Source    CredentialSource
	LoadedAt  time.Time // When the credential was loaded
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

	// Primary backend configuration
	// ARMOR_BACKEND selects the primary backend: "b2" (default) or "filesystem"
	Backend     string // Backend type: "b2" or "filesystem"
	FSPath      string // Path for filesystem backend (required when Backend=filesystem)

	// Secondary backend configuration (ADR-006)
	// When set, enables async replication to a secondary backend
	SecondaryBackend     string // Backend identifier (e.g., "filesystem", "s3", "wasabi")
	SecondaryBackendType string // Type: "filesystem" (future: "s3", "wasabi")
	SecondaryBackendPath string // Path for filesystem backend (required when Type=filesystem)

	// AllowNoCredentials disables the credential requirement check.
	// When true, the server starts without client credentials.
	// This is an escape hatch for the demo subcommand only.
	// Production deployments should always configure credentials.
	AllowNoCredentials bool
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Listen:      getEnv("ARMOR_LISTEN", "0.0.0.0:9000"),
		AdminListen: getEnv("ARMOR_ADMIN_LISTEN", "127.0.0.1:9001"),
	}

	var errs []error

	// Backend selection (default: b2)
	cfg.Backend = getEnv("ARMOR_BACKEND", "b2")
	if cfg.Backend != "b2" && cfg.Backend != "filesystem" {
		errs = append(errs, fmt.Errorf("ARMOR_BACKEND must be 'b2' or 'filesystem', got '%s'", cfg.Backend))
	}

	// Filesystem backend configuration
	if cfg.Backend == "filesystem" {
		cfg.FSPath = os.Getenv("ARMOR_FS_PATH")
		if cfg.FSPath == "" {
			errs = append(errs, fmt.Errorf("ARMOR_FS_PATH is required when ARMOR_BACKEND=filesystem"))
		}
	}

	// B2 backend configuration (required only when Backend=b2)
	if cfg.Backend == "b2" {
		cfg.B2Region = os.Getenv("ARMOR_B2_REGION")
		if cfg.B2Region == "" {
			errs = append(errs, fmt.Errorf("ARMOR_B2_REGION is required when ARMOR_BACKEND=b2"))
		}

		cfg.B2Endpoint = os.Getenv("ARMOR_B2_ENDPOINT")
		if cfg.B2Endpoint == "" && cfg.B2Region != "" {
			cfg.B2Endpoint = fmt.Sprintf("https://s3.%s.backblazeb2.com", cfg.B2Region)
		}

		cfg.B2AccessKeyID = os.Getenv("ARMOR_B2_ACCESS_KEY_ID")
		if cfg.B2AccessKeyID == "" {
			errs = append(errs, fmt.Errorf("ARMOR_B2_ACCESS_KEY_ID is required when ARMOR_BACKEND=b2"))
		}

		cfg.B2SecretAccessKey = os.Getenv("ARMOR_B2_SECRET_ACCESS_KEY")
		if cfg.B2SecretAccessKey == "" {
			errs = append(errs, fmt.Errorf("ARMOR_B2_SECRET_ACCESS_KEY is required when ARMOR_BACKEND=b2"))
		}
	}

	// Bucket is required for both backends
	cfg.Bucket = os.Getenv("ARMOR_BUCKET")
	if cfg.Bucket == "" {
		errs = append(errs, fmt.Errorf("ARMOR_BUCKET is required"))
	}

	// Cloudflare domain (optional for b2, ignored for filesystem)
	if cfg.Backend == "b2" {
		cfg.CFDomain = os.Getenv("ARMOR_CF_DOMAIN")
	}

	// Prefix for shared bucket support (ADR-001)
	// Normalize to exactly one trailing slash, no leading slash
	cfg.Prefix = normalizePrefix(os.Getenv("ARMOR_PREFIX"))

	// Canary disabled flag
	cfg.CanaryDisabled = os.Getenv("ARMOR_CANARY_DISABLED") == "true"

	// Master encryption key (required)
	mekHex := os.Getenv("ARMOR_MEK")
	if mekHex == "" {
		errs = append(errs, fmt.Errorf("ARMOR_MEK is required"))
	} else {
		var err error
		cfg.MEK, err = hex.DecodeString(mekHex)
		if err != nil {
			errs = append(errs, fmt.Errorf("ARMOR_MEK must be hex-encoded: %w", err))
		} else if len(cfg.MEK) != 32 {
			errs = append(errs, fmt.Errorf("ARMOR_MEK must be 32 bytes (64 hex chars), got %d bytes", len(cfg.MEK)))
		}
	}

	// Block size (default 64KB)
	cfg.BlockSize = getEnvInt("ARMOR_BLOCK_SIZE", 65536)
	if cfg.BlockSize < 4096 || (cfg.BlockSize&(cfg.BlockSize-1)) != 0 {
		errs = append(errs, fmt.Errorf("ARMOR_BLOCK_SIZE must be a power of 2 >= 4096"))
	}

	// Compression (default disabled per ADR-007)
	cfg.Compress = os.Getenv("ARMOR_COMPRESS") == "true"

	// Number of ranged reads allowed in flight for a backend read.
	cfg.ReadConcurrency = getEnvInt("ARMOR_READ_CONCURRENCY", 16)
	if cfg.ReadConcurrency < 1 {
		errs = append(errs, fmt.Errorf("ARMOR_READ_CONCURRENCY must be at least 1"))
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
	now := time.Now()
	cfg.Credentials = make(map[string]*Credential)
	cfg.Credentials[cfg.AuthAccessKey] = &Credential{
		AccessKey: cfg.AuthAccessKey,
		SecretKey: cfg.AuthSecretKey,
		ACLs:      nil, // nil means full access to configured bucket
		Source:    CredentialSourceEnv,
		LoadedAt:  now,
	}

	// Load additional named credentials (ARMOR_AUTH_<NAME>_ACCESS_KEY, _SECRET_KEY, _ACL)
	if err := loadNamedCredentials(cfg); err != nil {
		errs = append(errs, err)
	}

	// Load credentials from ARMOR_AUTH_FILE (YAML) and merge with env credentials
	// Env credentials win on name collision (logged at WARN)
	authFile, err := LoadAuthFile()
	if err != nil {
		errs = append(errs, err)
	} else if authFile != nil {
		if err := MergeFileCredentials(cfg, authFile); err != nil {
			errs = append(errs, err)
		}
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
		var err error
		cfg.PresignSecret, err = hex.DecodeString(presignSecretHex)
		if err != nil {
			errs = append(errs, fmt.Errorf("ARMOR_PRESIGN_SECRET must be hex-encoded: %w", err))
		} else if len(cfg.PresignSecret) < 32 {
			errs = append(errs, fmt.Errorf("ARMOR_PRESIGN_SECRET must be at least 32 bytes (64 hex chars)"))
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
				errs = append(errs, fmt.Errorf("ARMOR_MEK_DEFAULT is reserved; use ARMOR_MEK for the default key"))
				continue
			}
			// Decode hex MEK
			mek, err := hex.DecodeString(parts[1])
			if err != nil {
				errs = append(errs, fmt.Errorf("ARMOR_MEK_%s must be hex-encoded: %w", name, err))
				continue
			}
			if len(mek) != 32 {
				errs = append(errs, fmt.Errorf("ARMOR_MEK_%s must be 32 bytes (64 hex chars), got %d bytes", name, len(mek)))
				continue
			}
			cfg.NamedKeys[name] = mek
		}
	}

	// Load key routes (ARMOR_KEY_ROUTES)
	if routesStr := os.Getenv("ARMOR_KEY_ROUTES"); routesStr != "" {
		routes, err := parseKeyRoutes(routesStr)
		if err != nil {
			errs = append(errs, fmt.Errorf("ARMOR_KEY_ROUTES: %w", err))
		} else {
			cfg.KeyRoutes = routes
		}
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
			errs = append(errs, fmt.Errorf("ARMOR_SECONDARY_BACKEND_TYPE must be 'filesystem', got '%s'", cfg.SecondaryBackendType))
		} else {
			// For filesystem backend, path is required
			cfg.SecondaryBackendPath = os.Getenv("ARMOR_SECONDARY_BACKEND_PATH")
			if cfg.SecondaryBackendPath == "" {
				errs = append(errs, fmt.Errorf("ARMOR_SECONDARY_BACKEND_PATH is required when ARMOR_SECONDARY_BACKEND_TYPE=filesystem"))
			}
		}
	}

	// Return all errors collected during validation
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
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
// Returns an error joining all validation failures; nil on success.
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

	var errs []error
	now := time.Now()

	// Load each credential
	for name := range credNames {
		accessKey := os.Getenv("ARMOR_AUTH_" + name + "_ACCESS_KEY")
		secretKey := os.Getenv("ARMOR_AUTH_" + name + "_SECRET_KEY")
		aclStr := os.Getenv("ARMOR_AUTH_" + name + "_ACL")

		if accessKey == "" || secretKey == "" {
			errs = append(errs, fmt.Errorf("ARMOR_AUTH_%s_ACCESS_KEY and ARMOR_AUTH_%s_SECRET_KEY are both required", name, name))
			continue
		}

		// Check for duplicate access key
		if _, exists := cfg.Credentials[accessKey]; exists {
			errs = append(errs, fmt.Errorf("duplicate access key in ARMOR_AUTH_%s", name))
			continue
		}

		cred := &Credential{
			AccessKey: accessKey,
			SecretKey: secretKey,
			Source:    CredentialSourceEnv,
			LoadedAt:  now,
		}

		// Parse ACL if provided
		if aclStr != "" {
			acls, err := parseACL(aclStr)
			if err != nil {
				errs = append(errs, fmt.Errorf("ARMOR_AUTH_%s_ACL: %w", name, err))
				continue
			}
			cred.ACLs = acls
		}

		cfg.Credentials[accessKey] = cred
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
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
		}
		if strings.Contains(prefix, "*") {
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

// RedactedConfig represents a configuration with all secret values redacted.
// Secret fields are replaced with "<set>" or "<unset>" indicators.
type RedactedConfig struct {
	// Server configuration
	Listen      string `json:"listen"`
	AdminListen string `json:"admin_listen"`

	// B2 backend configuration
	B2Region          string `json:"b2_region"`
	B2Endpoint        string `json:"b2_endpoint"`
	B2AccessKeyID     string `json:"b2_access_key_id"`
	B2SecretAccessKey string `json:"b2_secret_access_key"`
	Bucket            string `json:"bucket"`

	// Prefix for all keys (shared bucket support via ADR-001)
	Prefix string `json:"prefix"`

	// Cloudflare download configuration
	CFDomain string `json:"cf_domain"`

	// Canary configuration
	CanaryDisabled bool `json:"canary_disabled"`

	// Encryption configuration
	MEK       string `json:"mek"` // "<set>" or "<unset>"
	BlockSize int    `json:"block_size"`

	// Compress configuration
	Compress bool `json:"compress"`

	// Read path configuration
	ReadConcurrency int `json:"read_concurrency"`

	// Multi-key configuration
	NamedKeys map[string]string `json:"named_keys"` // key name -> "<set>" or "<unset>"
	KeyRoutes []KeyRoute        `json:"key_routes"`

	// Authentication credentials for ARMOR clients
	AuthAccessKey string `json:"auth_access_key"`
	AuthSecretKey string `json:"auth_secret_key"` // "<set>" or "<unset>"

	// Multi-credential support
	Credentials map[string]RedactedCredential `json:"credentials"`

	// Writer ID for provenance chain
	WriterID string `json:"writer_id"`

	// Cache configuration
	CacheMaxEntries int `json:"cache_max_entries"`
	CacheTTL        int `json:"cache_ttl"`

	// List cache configuration
	ListCacheMaxEntries int `json:"list_cache_max_entries"`
	ListCacheTTL        int `json:"list_cache_ttl"`

	// Pre-signed URL configuration
	PresignSecret  string `json:"presign_secret"`  // "<set>" or "<unset>"
	PresignBaseURL string `json:"presign_base_url"`

	// Readiness probe configuration
	ReadyzCacheTTL int `json:"readyz_cache_ttl"`

	// Manifest index configuration (Phase 4)
	ManifestEnabled             bool   `json:"manifest_enabled"`
	ManifestPrefix              string `json:"manifest_prefix"`
	ManifestCompactionInterval  int    `json:"manifest_compaction_interval"`
	ManifestCompactionThreshold int    `json:"manifest_compaction_threshold"`

	// Dashboard authentication configuration
	DashboardUser  string `json:"dashboard_user"`
	DashboardPass  string `json:"dashboard_pass"`  // "<set>" or "<unset>"
	DashboardToken string `json:"dashboard_token"` // "<set>" or "<unset>"

	// Admin API bearer token
	AdminToken string `json:"admin_token"` // "<set>" or "<unset>"

	// Log level configuration
	LogLevel string `json:"log_level"`

	// Primary backend configuration
	Backend string `json:"backend"` // "b2" or "filesystem"
	FSPath  string `json:"fs_path"`

	// Secondary backend configuration (ADR-006)
	SecondaryBackend     string `json:"secondary_backend"`
	SecondaryBackendType  string `json:"secondary_backend_type"`
	SecondaryBackendPath string `json:"secondary_backend_path"`
}

// RedactedCredential represents a credential with secret key redacted.
type RedactedCredential struct {
	AccessKey string         `json:"access_key"`
	SecretKey string         `json:"secret_key"` // "<set>" or "<unset>"
	ACLs      []acl.ACLEntry `json:"acls"`
}

// Redacted returns a configuration with all secret values replaced with
// "<set>" or "<unset>" indicators. This is safe to log without exposing
// sensitive material.
func (c *Config) Redacted() *RedactedConfig {
	rc := &RedactedConfig{
		Listen:       c.Listen,
		AdminListen:  c.AdminListen,
		B2Region:     c.B2Region,
		B2Endpoint:   c.B2Endpoint,
		B2AccessKeyID: c.B2AccessKeyID,
		Bucket:       c.Bucket,
		Prefix:       c.Prefix,
		CFDomain:     c.CFDomain,
		CanaryDisabled: c.CanaryDisabled,
		BlockSize:    c.BlockSize,
		Compress:     c.Compress,
		ReadConcurrency: c.ReadConcurrency,
		AuthAccessKey: c.AuthAccessKey,
		WriterID:     c.WriterID,
		CacheMaxEntries: c.CacheMaxEntries,
		CacheTTL:        c.CacheTTL,
		ListCacheMaxEntries: c.ListCacheMaxEntries,
		ListCacheTTL:        c.ListCacheTTL,
		PresignBaseURL: c.PresignBaseURL,
		ReadyzCacheTTL: c.ReadyzCacheTTL,
		ManifestEnabled:             c.ManifestEnabled,
		ManifestPrefix:              c.ManifestPrefix,
		ManifestCompactionInterval:  c.ManifestCompactionInterval,
		ManifestCompactionThreshold: c.ManifestCompactionThreshold,
		DashboardUser:  c.DashboardUser,
		LogLevel:       c.LogLevel,
		Backend:        c.Backend,
		FSPath:         c.FSPath,
		SecondaryBackend:     c.SecondaryBackend,
		SecondaryBackendType:  c.SecondaryBackendType,
		SecondaryBackendPath:  c.SecondaryBackendPath,
	}

	// Redact B2 secret key
	if c.B2SecretAccessKey != "" {
		rc.B2SecretAccessKey = "<set>"
	} else {
		rc.B2SecretAccessKey = "<unset>"
	}

	// Redact MEK
	if len(c.MEK) > 0 {
		rc.MEK = "<set>"
	} else {
		rc.MEK = "<unset>"
	}

	// Redact auth secret key
	if c.AuthSecretKey != "" {
		rc.AuthSecretKey = "<set>"
	} else {
		rc.AuthSecretKey = "<unset>"
	}

	// Redact presign secret
	if len(c.PresignSecret) > 0 {
		rc.PresignSecret = "<set>"
	} else {
		rc.PresignSecret = "<unset>"
	}

	// Redact dashboard credentials
	if c.DashboardPass != "" {
		rc.DashboardPass = "<set>"
	} else {
		rc.DashboardPass = "<unset>"
	}

	if c.DashboardToken != "" {
		rc.DashboardToken = "<set>"
	} else {
		rc.DashboardToken = "<unset>"
	}

	// Redact admin token
	if c.AdminToken != "" {
		rc.AdminToken = "<set>"
	} else {
		rc.AdminToken = "<unset>"
	}

	// Redact named keys
	rc.NamedKeys = make(map[string]string)
	for name, mek := range c.NamedKeys {
		if len(mek) > 0 {
			rc.NamedKeys[name] = "<set>"
		} else {
			rc.NamedKeys[name] = "<unset>"
		}
	}

	// Copy key routes (no secrets in routes)
	rc.KeyRoutes = make([]KeyRoute, len(c.KeyRoutes))
	copy(rc.KeyRoutes, c.KeyRoutes)

	// Redact credentials
	rc.Credentials = make(map[string]RedactedCredential)
	for accessKey, cred := range c.Credentials {
		rc.Credentials[accessKey] = RedactedCredential{
			AccessKey: cred.AccessKey,
			SecretKey: boolToSetUnset(cred.SecretKey != ""),
			ACLs:      cred.ACLs,
		}
	}

	return rc
}

// boolToSetUnset converts a boolean to "<set>" or "<unset>".
func boolToSetUnset(set bool) string {
	if set {
		return "<set>"
	}
	return "<unset>"
}
