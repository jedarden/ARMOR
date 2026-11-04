// settings.go builds the resolved-setting table behind `armor config show`.
package config

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/crypto"
)

// Setting-source labels reported by SettingRow.Source.
const (
	// SettingSourceEnv marks a setting the environment set explicitly.
	SettingSourceEnv = "env"
	// SettingSourceDefault marks a setting left unset, holding its documented
	// default (rendered as the empty string for optional settings that have
	// none).
	SettingSourceDefault = "default"
	// SettingSourceDerived marks a value computed from other settings rather
	// than read from any one variable: the B2 endpoint URL composed from the
	// region, the writer ID taken from the hostname, the secondary-backend
	// selector assembled from the ARMOR_SECONDARY_B2_* quartet.
	SettingSourceDerived = "derived"
	// SettingSourceSkipped marks a variable this deployment's backend choice
	// never reads (the ARMOR_B2_* set under ARMOR_BACKEND=filesystem,
	// ARMOR_FS_PATH under b2, ARMOR_CF_DOMAIN under filesystem). The row
	// carries an empty value: naming the variable with a "set" source would
	// claim an input that has no effect.
	SettingSourceSkipped = "skipped"
)

// SettingRow is one resolved configuration setting as `armor config show`
// prints it: the backing ARMOR_* variable (or a dynamic row name for the
// per-key and per-credential settings that have no fixed name), a log-safe
// rendering of the effective value, and how the value was resolved.
//
// Values follow the same redaction policy as RedactedConfig: secret material
// (the MEK and every other key, the admin token, client secret keys) renders
// as "<set>"/"<unset>" or, for key material with a stable fingerprint, that
// fingerprint; deliberately unpublished identifiers (bucket names, access-key
// IDs) render as fingerprints; everything else renders in the clear. A row
// value must never carry credential material.
type SettingRow struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

// Settings returns every resolved setting in a fixed, deterministic order
// (mirroring the configuration reference's grouping), redacted per the
// SettingRow policy. Static rows are always present — including the ones a
// deployment's backend choice leaves unread, which report
// SettingSourceSkipped — so the table shape is stable across deployments;
// per-named-key and per-credential rows appear only when configured.
func (c *Config) Settings() []SettingRow {
	var rows []SettingRow
	add := func(name, value, source string) {
		rows = append(rows, SettingRow{Name: name, Value: value, Source: source})
	}
	addInt := func(name string, value int, source string) {
		add(name, strconv.Itoa(value), source)
	}
	addInt64 := func(name string, value int64, source string) {
		add(name, strconv.FormatInt(value, 10), source)
	}
	addBool := func(name string, value bool, source string) {
		add(name, boolStr(value), source)
	}
	addDuration := func(name string, value time.Duration, source string) {
		add(name, value.String(), source)
	}
	addSecret := func(name string, set bool, source string) {
		add(name, boolToSetUnset(set), source)
	}
	// idValue renders an unpublished identifier as its fingerprint. An unset
	// identifier renders as the empty string, matching the JSON shape of
	// optional settings.
	idValue := func(value string) string {
		if value == "" {
			return ""
		}
		return crypto.IdentifierFingerprint(value)
	}
	// keyRingValue renders one ring's retired MEKs as their fingerprints.
	keyRingValue := func(ring []byte) string {
		if len(ring) == 0 {
			return ""
		}
		var fps []string
		for i := 0; i+32 <= len(ring); i += 32 {
			fps = append(fps, crypto.MEKFingerprint(ring[i:i+32]))
		}
		return strings.Join(fps, ",")
	}

	b2 := c.Backend == "b2"
	fs := c.Backend == "filesystem"

	// Server configuration
	add("ARMOR_LISTEN", c.Listen, envSource("ARMOR_LISTEN"))
	add("ARMOR_ADMIN_LISTEN", c.AdminListen, envSource("ARMOR_ADMIN_LISTEN"))
	add("ARMOR_LOG_LEVEL", c.LogLevel, envSource("ARMOR_LOG_LEVEL"))
	if os.Getenv("ARMOR_WRITER_ID") != "" {
		add("ARMOR_WRITER_ID", c.WriterID, SettingSourceEnv)
	} else {
		// Load derives the writer ID from the hostname when the variable is
		// unset, so the row reports the computed provenance rather than a
		// default that exists nowhere.
		add("ARMOR_WRITER_ID", c.WriterID, SettingSourceDerived)
	}
	addInt("ARMOR_FORMAT_VERSION", c.FormatWriteVersion, envSource("ARMOR_FORMAT_VERSION"))
	addBool("ARMOR_CANARY_DISABLED", c.CanaryDisabled, envSource("ARMOR_CANARY_DISABLED"))
	addBool("ARMOR_ALLOW_NO_CREDENTIALS", c.AllowNoCredentials, envSource("ARMOR_ALLOW_NO_CREDENTIALS"))

	// Backend selection and backend-specific configuration
	add("ARMOR_BACKEND", c.Backend, envSource("ARMOR_BACKEND"))
	if fs {
		add("ARMOR_FS_PATH", c.FSPath, envSource("ARMOR_FS_PATH"))
	} else {
		add("ARMOR_FS_PATH", "", SettingSourceSkipped)
	}
	if b2 {
		add("ARMOR_B2_REGION", c.B2Region, envSource("ARMOR_B2_REGION"))
		switch {
		case os.Getenv("ARMOR_B2_ENDPOINT") != "":
			add("ARMOR_B2_ENDPOINT", c.B2Endpoint, SettingSourceEnv)
		case c.B2Endpoint != "":
			// Composed from the region when the override is unset.
			add("ARMOR_B2_ENDPOINT", c.B2Endpoint, SettingSourceDerived)
		default:
			add("ARMOR_B2_ENDPOINT", "", SettingSourceDefault)
		}
		add("ARMOR_B2_ACCESS_KEY_ID", idValue(c.B2AccessKeyID), SettingSourceEnv)
		addSecret("ARMOR_B2_SECRET_ACCESS_KEY", c.B2SecretAccessKey != "", SettingSourceEnv)
	} else {
		for _, name := range []string{"ARMOR_B2_REGION", "ARMOR_B2_ENDPOINT", "ARMOR_B2_ACCESS_KEY_ID", "ARMOR_B2_SECRET_ACCESS_KEY"} {
			add(name, "", SettingSourceSkipped)
		}
	}
	add("ARMOR_BUCKET", idValue(c.Bucket), envSource("ARMOR_BUCKET"))
	if len(c.BucketAliases) == 0 {
		add("ARMOR_BUCKET_ALIASES", "", envSource("ARMOR_BUCKET_ALIASES"))
	} else {
		fps := make([]string, 0, len(c.BucketAliases))
		for _, alias := range c.BucketAliases {
			fps = append(fps, crypto.IdentifierFingerprint(alias))
		}
		add("ARMOR_BUCKET_ALIASES", strings.Join(fps, ","), SettingSourceEnv)
	}
	add("ARMOR_PREFIX", c.Prefix, envSource("ARMOR_PREFIX"))
	addBool("ARMOR_REQUIRE_PREFIX", c.RequirePrefix, envSource("ARMOR_REQUIRE_PREFIX"))
	if b2 {
		add("ARMOR_CF_DOMAIN", c.CFDomain, envSource("ARMOR_CF_DOMAIN"))
	} else {
		add("ARMOR_CF_DOMAIN", "", SettingSourceSkipped)
	}

	// Encryption and keys
	if len(c.MEK) > 0 {
		add("ARMOR_MEK", crypto.MEKFingerprint(c.MEK), SettingSourceEnv)
	} else {
		addSecret("ARMOR_MEK", false, SettingSourceDefault)
	}
	for _, name := range sortedKeyNames(c.NamedKeys) {
		// Load lowercases key names; the row names them the way the
		// environment does so an operator can match row to variable.
		add("ARMOR_MEK_"+strings.ToUpper(name), crypto.MEKFingerprint(c.NamedKeys[name]), SettingSourceEnv)
	}
	add("ARMOR_MEK_RING", keyRingValue(c.KeyRings["default"]), envSource("ARMOR_MEK_RING"))
	for _, name := range sortedKeyNames(c.NamedKeys) {
		if ring, ok := c.KeyRings[name]; ok {
			add("ARMOR_MEK_"+strings.ToUpper(name)+"_RING", keyRingValue(ring), SettingSourceEnv)
		}
	}
	if len(c.KeyRoutes) == 0 {
		add("ARMOR_KEY_ROUTES", "", envSource("ARMOR_KEY_ROUTES"))
	} else {
		parts := make([]string, 0, len(c.KeyRoutes))
		for _, route := range c.KeyRoutes {
			parts = append(parts, route.Prefix+"="+route.KeyName)
		}
		add("ARMOR_KEY_ROUTES", strings.Join(parts, ","), SettingSourceEnv)
	}
	addInt("ARMOR_BLOCK_SIZE", c.BlockSize, envSource("ARMOR_BLOCK_SIZE"))
	addBool("ARMOR_COMPRESS", c.Compress, envSource("ARMOR_COMPRESS"))
	add("ARMOR_COMPRESS_RULES", c.CompressRules.String(), envSource("ARMOR_COMPRESS_RULES"))

	// Authentication. The per-credential rows are keyed by access-key
	// fingerprint, matching RedactedConfig.Credentials; the source reports
	// where the credential came from (environment triplet or auth file).
	add("ARMOR_AUTH_ACCESS_KEY", idValue(c.AuthAccessKey), envSource("ARMOR_AUTH_ACCESS_KEY"))
	addSecret("ARMOR_AUTH_SECRET_KEY", c.AuthSecretKey != "", envSource("ARMOR_AUTH_SECRET_KEY"))
	add("ARMOR_AUTH_FILE", c.AuthFilePath, envSource("ARMOR_AUTH_FILE"))
	for _, fp := range sortedCredentialFingerprints(c.Credentials) {
		cred := c.Credentials[fp.accessKey]
		value := "secret " + boolToSetUnset(cred.SecretKey != "")
		if n := len(cred.ACLs); n > 0 {
			value += ", " + strconv.Itoa(n) + " ACLs"
		} else {
			value += ", full access"
		}
		add("credential/"+fp.fingerprint, value, string(cred.Source))
	}

	// Read path
	addInt("ARMOR_READ_CONCURRENCY", c.ReadConcurrency, envSource("ARMOR_READ_CONCURRENCY"))
	addInt64("ARMOR_READ_FETCH_SIZE", c.ReadFetchSize, envSource("ARMOR_READ_FETCH_SIZE"))
	addDuration("ARMOR_READ_REQUEST_TIMEOUT", c.ReadRequestTimeout, envSource("ARMOR_READ_REQUEST_TIMEOUT"))
	addInt("ARMOR_READ_BACKEND_LIMIT", c.ReadBackendLimit, envSource("ARMOR_READ_BACKEND_LIMIT"))
	addInt64("ARMOR_SMALL_OBJECT_FETCH_BYTES", c.SmallObjectFetchBytes, envSource("ARMOR_SMALL_OBJECT_FETCH_BYTES"))
	addInt64("ARMOR_UPLOAD_PART_MEMORY_BUDGET", c.UploadPartMemoryBudget, envSource("ARMOR_UPLOAD_PART_MEMORY_BUDGET"))
	addInt("ARMOR_READ_RETRY_ATTEMPTS", c.ReadRetryAttempts, envSource("ARMOR_READ_RETRY_ATTEMPTS"))
	addDuration("ARMOR_READ_RETRY_BACKOFF", c.ReadRetryBackoff, envSource("ARMOR_READ_RETRY_BACKOFF"))
	addDuration("ARMOR_READ_RETRY_MAX_BACKOFF", c.ReadRetryMaxBackoff, envSource("ARMOR_READ_RETRY_MAX_BACKOFF"))
	addBool("ARMOR_CF_STALE_FALLBACK", c.CFStaleFallback, envSource("ARMOR_CF_STALE_FALLBACK"))
	addBool("ARMOR_CF_VERSION_PINNED", c.CFVersionPinned, envSource("ARMOR_CF_VERSION_PINNED"))
	addBool("ARMOR_CF_DISABLE_HTTP2", c.CFDisableHTTP2, envSource("ARMOR_CF_DISABLE_HTTP2"))

	// Cache and list path
	addInt("ARMOR_CACHE_MAX_ENTRIES", c.CacheMaxEntries, envSource("ARMOR_CACHE_MAX_ENTRIES"))
	addInt("ARMOR_CACHE_TTL", c.CacheTTL, envSource("ARMOR_CACHE_TTL"))
	addInt64("ARMOR_CACHE_MAX_BYTES", c.CacheMaxBytes, envSource("ARMOR_CACHE_MAX_BYTES"))
	addInt("ARMOR_LIST_CACHE_MAX_ENTRIES", c.ListCacheMaxEntries, envSource("ARMOR_LIST_CACHE_MAX_ENTRIES"))
	addInt("ARMOR_LIST_CACHE_TTL", c.ListCacheTTL, envSource("ARMOR_LIST_CACHE_TTL"))
	addInt("ARMOR_LIST_HEAD_FANOUT", c.ListHeadFanout, envSource("ARMOR_LIST_HEAD_FANOUT"))

	// Manifest index
	addBool("ARMOR_MANIFEST_ENABLED", c.ManifestEnabled, envSource("ARMOR_MANIFEST_ENABLED"))
	add("ARMOR_MANIFEST_PREFIX", c.ManifestPrefix, envSource("ARMOR_MANIFEST_PREFIX"))
	addInt("ARMOR_MANIFEST_COMPACTION_INTERVAL", c.ManifestCompactionInterval, envSource("ARMOR_MANIFEST_COMPACTION_INTERVAL"))
	addInt("ARMOR_MANIFEST_COMPACTION_THRESHOLD", c.ManifestCompactionThreshold, envSource("ARMOR_MANIFEST_COMPACTION_THRESHOLD"))
	addInt("ARMOR_MANIFEST_LOAD_TIMEOUT", c.ManifestLoadTimeout, envSource("ARMOR_MANIFEST_LOAD_TIMEOUT"))

	// Dashboard and admin
	add("ARMOR_DASHBOARD_USER", c.DashboardUser, envSource("ARMOR_DASHBOARD_USER"))
	addSecret("ARMOR_DASHBOARD_PASS", c.DashboardPass != "", envSource("ARMOR_DASHBOARD_PASS"))
	addSecret("ARMOR_DASHBOARD_TOKEN", c.DashboardToken != "", envSource("ARMOR_DASHBOARD_TOKEN"))
	add("ARMOR_DASHBOARD_CREDENTIAL", idValue(c.DashboardCredential), envSource("ARMOR_DASHBOARD_CREDENTIAL"))
	addSecret("ARMOR_ADMIN_TOKEN", c.AdminToken != "", envSource("ARMOR_ADMIN_TOKEN"))

	// Pre-signed URLs
	addBool("ARMOR_PRESIGN_ENABLED", c.PresignEnabled, envSource("ARMOR_PRESIGN_ENABLED"))
	addSecret("ARMOR_PRESIGN_SECRET", len(c.PresignSecret) > 0, envSource("ARMOR_PRESIGN_SECRET"))
	add("ARMOR_PRESIGN_BASE_URL", c.PresignBaseURL, envSource("ARMOR_PRESIGN_BASE_URL"))

	// Secondary backend (ADR-006). SecondaryBackend carries only the backend
	// kind: the canonical selector may embed credentials and must never be
	// echoed verbatim (see Redacted).
	switch {
	case os.Getenv("ARMOR_SECONDARY_BACKEND") != "":
		add("ARMOR_SECONDARY_BACKEND", c.SecondaryBackend, SettingSourceEnv)
	case c.SecondaryBackend != "":
		add("ARMOR_SECONDARY_BACKEND", c.SecondaryBackend, SettingSourceDerived)
	default:
		add("ARMOR_SECONDARY_BACKEND", "", SettingSourceDefault)
	}
	add("ARMOR_SECONDARY_BACKEND_PATH", c.SecondaryBackendPath, envSource("ARMOR_SECONDARY_BACKEND_PATH"))
	add("ARMOR_SECONDARY_BACKEND_TYPE", c.SecondaryBackendType, envSource("ARMOR_SECONDARY_BACKEND_TYPE"))
	secondaryB2 := c.SecondaryBackendConfig.Type == "b2"
	if secondaryB2 {
		add("ARMOR_SECONDARY_B2_ENDPOINT", c.SecondaryBackendConfig.Endpoint, envSource("ARMOR_SECONDARY_B2_ENDPOINT"))
		add("ARMOR_SECONDARY_B2_KEY_ID", idValue(c.SecondaryBackendConfig.AccessKeyID), SettingSourceEnv)
		addSecret("ARMOR_SECONDARY_B2_KEY", c.SecondaryBackendConfig.SecretKey != "", SettingSourceEnv)
		add("ARMOR_SECONDARY_B2_BUCKET", idValue(c.SecondaryBackendConfig.Bucket), SettingSourceEnv)
	} else {
		for _, name := range []string{"ARMOR_SECONDARY_B2_ENDPOINT", "ARMOR_SECONDARY_B2_KEY_ID", "ARMOR_SECONDARY_B2_KEY", "ARMOR_SECONDARY_B2_BUCKET"} {
			add(name, "", SettingSourceSkipped)
		}
	}

	return rows
}

// envSource reports whether name is set to a non-empty value in the process
// environment — the exact condition every reader in Load treats as "set" —
// so a row's source says env only when the environment actually supplied it.
func envSource(name string) string {
	if os.Getenv(name) != "" {
		return SettingSourceEnv
	}
	return SettingSourceDefault
}

// boolStr renders a boolean the way the configuration reference documents
// its boolean defaults.
func boolStr(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// sortedKeyNames returns the named-key map's names in sorted order so the
// per-key rows (and only those — the rest of the table is built in a fixed
// order) stay deterministic across runs.
func sortedKeyNames(keys map[string][]byte) []string {
	names := make([]string, 0, len(keys))
	for name := range keys {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// credentialFingerprint pairs a credential's access key with the fingerprint
// that identifies it in show output.
type credentialFingerprint struct {
	accessKey   string
	fingerprint string
}

// sortedCredentialFingerprints returns the credentials in fingerprint order,
// so the per-credential rows are deterministic.
func sortedCredentialFingerprints(credentials map[string]*Credential) []credentialFingerprint {
	fps := make([]credentialFingerprint, 0, len(credentials))
	for accessKey := range credentials {
		fps = append(fps, credentialFingerprint{accessKey: accessKey, fingerprint: crypto.IdentifierFingerprint(accessKey)})
	}
	sort.Slice(fps, func(i, j int) bool { return fps[i].fingerprint < fps[j].fingerprint })
	return fps
}
