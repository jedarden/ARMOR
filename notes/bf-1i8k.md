# Manifest Test Files Search Results (bf-1i8k)

## Task Summary
Search for all test files related to manifests in the ARMOR codebase.

## Files with 'manifest' in their name
- `internal/manifest/manifest_test.go`

## Test files that import/reference manifest modules

### Core manifest tests (internal/manifest/)
- `internal/manifest/manifest_test.go`
- `internal/manifest/loader_test.go`
- `internal/manifest/writer_test.go`
- `internal/manifest/compaction_test.go`
- `internal/manifest/roundtrip_test.go`

### Config tests
- `internal/config/config_test.go` - Contains `TestManifestConfigDefaults`, `TestManifestEnabledFalse`, `TestManifestEnabledTrue`, `TestManifestPrefix`, `TestManifestCompactionInterval`, `TestManifestCompactionThreshold`

### Server tests
- `internal/server/handlers/handlers_test.go`
- `internal/server/readyz_test.go`
- `internal/server/key_rotation_test.go`

## Complete list of all _test.go files in codebase (for reference)
```
internal/backend/backend_test.go
internal/backend/cache_test.go
internal/b2keys/b2keys_test.go
internal/canary/canary_test.go
internal/config/config_test.go
internal/crypto/crypto_test.go
internal/dashboard/dashboard_test.go
internal/keymanager/keymanager_test.go
internal/logging/logging_test.go
internal/manifest/compaction_test.go
internal/manifest/loader_test.go
internal/manifest/manifest_test.go
internal/manifest/roundtrip_test.go
internal/manifest/writer_test.go
internal/metrics/metrics_test.go
internal/presign/presign_test.go
internal/provenance/provenance_test.go
internal/server/auth_test.go
internal/server/b2keys_test.go
internal/server/handlers/handlers_test.go
internal/server/key_rotation_test.go
internal/server/readyz_test.go
tests/integration/awscli_test.go
tests/integration/integration_test.go
```

## Summary
- **Total test files**: 24
- **Manifest-related test files**: 9
- **Core manifest package tests**: 5 (all in `internal/manifest/`)
