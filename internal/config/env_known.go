package config

import (
	"fmt"
	"sort"
	"strings"
)

// knownEnvVars lists every ARMOR_* environment variable name this package
// reads, plus the runtime-built family prefixes (the trailing "_" entries: a
// family entry covers every member, so ARMOR_AUTH_ covers
// ARMOR_AUTH_<NAME>_ACCESS_KEY and ARMOR_MEK_ covers ARMOR_MEK_<NAME>_RING —
// the same matching rule docsindex.PhantomEnvVars applies). It feeds the
// startup warning about unrecognized ARMOR_* variables: the process
// environment can carry names meant for another tool, and a misspelled name
// (ARMOR_CF_DOMIAN for ARMOR_CF_DOMAIN) otherwise silently disables the very
// setting it was meant to configure, because nothing reads it.
//
// The list must stay in sync with the package sources — it is static data a
// deployed binary can consult without them, and TestKnownEnvVarsMatchesSource
// holds it to docsindex.ConfigEnvVars, the same extraction the
// configuration-reference parity check uses: an ARMOR_* literal added to any
// non-test file of this package without a matching entry here fails the test.
// Demo and test-suite names that appear only in *_test.go are deliberately
// absent — they configure test runs, not deployments.
var knownEnvVars = []string{
	"ARMOR_ADMIN_LISTEN",
	"ARMOR_ADMIN_TOKEN",
	"ARMOR_ALLOW_NO_CREDENTIALS",
	"ARMOR_AUTH_",
	"ARMOR_AUTH_ACCESS_KEY",
	"ARMOR_AUTH_FILE",
	"ARMOR_AUTH_SECRET_KEY",
	"ARMOR_B2_ACCESS_KEY_ID",
	"ARMOR_B2_ENDPOINT",
	"ARMOR_B2_REGION",
	"ARMOR_B2_SECRET_ACCESS_KEY",
	"ARMOR_BACKEND",
	"ARMOR_BLOCK_SIZE",
	"ARMOR_BUCKET",
	"ARMOR_BUCKET_ALIASES",
	"ARMOR_CACHE_MAX_BYTES",
	"ARMOR_CACHE_MAX_ENTRIES",
	"ARMOR_CACHE_TTL",
	"ARMOR_CANARY_DISABLED",
	"ARMOR_CF_DISABLE_HTTP2",
	"ARMOR_CF_DOMAIN",
	"ARMOR_CF_STALE_FALLBACK",
	"ARMOR_CF_VERSION_PINNED",
	"ARMOR_COMPRESS",
	"ARMOR_COMPRESS_RULES",
	"ARMOR_DASHBOARD_CREDENTIAL",
	"ARMOR_DASHBOARD_PASS",
	"ARMOR_DASHBOARD_TOKEN",
	"ARMOR_DASHBOARD_USER",
	"ARMOR_FORMAT_VERSION",
	"ARMOR_FS_PATH",
	"ARMOR_KEY_ROUTES",
	"ARMOR_LISTEN",
	"ARMOR_LIST_CACHE_MAX_ENTRIES",
	"ARMOR_LIST_CACHE_TTL",
	"ARMOR_LIST_HEAD_FANOUT",
	"ARMOR_LOG_LEVEL",
	"ARMOR_MANIFEST_COMPACTION_INTERVAL",
	"ARMOR_MANIFEST_COMPACTION_THRESHOLD",
	"ARMOR_MANIFEST_ENABLED",
	"ARMOR_MANIFEST_LOAD_TIMEOUT",
	"ARMOR_MANIFEST_PREFIX",
	"ARMOR_MEK",
	"ARMOR_MEK_",
	"ARMOR_MEK_RING",
	"ARMOR_PREFIX",
	"ARMOR_PRESIGN_BASE_URL",
	"ARMOR_PRESIGN_ENABLED",
	"ARMOR_PRESIGN_SECRET",
	"ARMOR_READ_BACKEND_LIMIT",
	"ARMOR_READ_BUFFER_MEMORY_BUDGET",
	"ARMOR_READ_CONCURRENCY",
	"ARMOR_READ_FETCH_SIZE",
	"ARMOR_READ_REQUEST_TIMEOUT",
	"ARMOR_READ_RETRY_ATTEMPTS",
	"ARMOR_READ_RETRY_BACKOFF",
	"ARMOR_READ_RETRY_MAX_BACKOFF",
	"ARMOR_REQUIRE_PREFIX",
	"ARMOR_SECONDARY_B2_BUCKET",
	"ARMOR_SECONDARY_B2_ENDPOINT",
	"ARMOR_SECONDARY_B2_KEY",
	"ARMOR_SECONDARY_B2_KEY_ID",
	"ARMOR_SECONDARY_BACKEND",
	"ARMOR_SECONDARY_BACKEND_PATH",
	"ARMOR_SECONDARY_BACKEND_TYPE",
	"ARMOR_SMALL_OBJECT_FETCH_BYTES",
	"ARMOR_UPLOAD_PART_MEMORY_BUDGET",
	"ARMOR_WRITER_ID",
}

// unknownEnvVars returns the sorted names of the ARMOR_* variables set to a
// non-empty value in environ that no entry of known covers: an entry matches
// exactly, and an entry ending in "_" is a family prefix matching every name
// it prefixes. An empty value does not warn — every reader in this package
// treats empty exactly like unset, and environment files legitimately keep
// disabled entries — and a name not beginning with ARMOR_ is not this
// package's concern.
func unknownEnvVars(environ []string, known []string) []string {
	var unknown []string
	for _, kv := range environ {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		name := kv[:eq]
		if kv[eq+1:] == "" || !strings.HasPrefix(name, "ARMOR_") {
			continue
		}
		covered := false
		for _, k := range known {
			if name == k || (strings.HasSuffix(k, "_") && strings.HasPrefix(name, k)) {
				covered = true
				break
			}
		}
		if !covered {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	return unknown
}

// unknownEnvVarWarnings formats one startup warning per unrecognized ARMOR_*
// variable set in environ. Warnings are not errors: an unrecognized variable
// has no effect, so the process starts, but the operator is told at startup
// (and by armor check) rather than discovering it during an incident.
func unknownEnvVarWarnings(environ []string) []string {
	unknown := unknownEnvVars(environ, knownEnvVars)
	if len(unknown) == 0 {
		return nil
	}
	warnings := make([]string, 0, len(unknown))
	for _, name := range unknown {
		warnings = append(warnings, fmt.Sprintf(
			"unknown environment variable %s is set; this process does not read it and will ignore it (the variables ARMOR reads are listed in docs/configuration.md)", name))
	}
	return warnings
}
