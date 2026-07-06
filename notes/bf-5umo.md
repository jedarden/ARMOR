# Bead bf-5umo: Locate Pluck Configuration File

## Task Summary
Find the Pluck configuration file in the ARMOR codebase.

## Investigation Results

### Search Methods Used
1. Searched for common Pluck configuration file names:
   - `.pluck.yml`, `.pluck.yaml`, `pluck.yml`, `pluck.yaml`
   - `.pluck.json`, `pluck.json`
   - `.pluck.toml`, `pluck.toml`

2. Searched for "pluck" and "exclude_labels" references in:
   - All Go source files (`*.go`)
   - All configuration files (`*.yaml`, `*.yml`, `*.json`, `*.toml`)
   - All documentation files (`*.md`)

3. Reviewed ARMOR's configuration structure in `internal/config/config.go`

### Findings
**No Pluck configuration file exists in the ARMOR codebase.**

### ARMOR Configuration Architecture

ARMOR uses **environment variables only** for configuration - there are no configuration files:

- **Source**: `internal/config/config.go`
- **Mechanism**: Environment variables exclusively
- **Storage**: Kubernetes ExternalSecrets (managed via ArgoCD in `jedarden/declarative-config`)

### Documented Environment Variables
(From `internal/config/config.go`)

Server:
- `ARMOR_LISTEN` (default: 0.0.0.0:9000)
- `ARMOR_ADMIN_LISTEN` (default: 127.0.0.1:9001)

B2 Backend:
- `ARMOR_B2_REGION` (required)
- `ARMOR_B2_ENDPOINT` (optional, defaults to B2 S3 endpoint)
- `ARMOR_B2_ACCESS_KEY_ID` (required)
- `ARMOR_B2_SECRET_ACCESS_KEY` (required)
- `ARMOR_BUCKET` (required)

Cloudflare:
- `ARMOR_CF_DOMAIN` (optional)

Prefix (ADR-001 shared bucket support):
- `ARMOR_PREFIX` (optional)

Encryption:
- `ARMOR_MEK` (required - 32-byte hex-encoded master encryption key)
- `ARMOR_BLOCK_SIZE` (default: 65536)
- `ARMOR_MEK_<NAME>` (for named keys)

Multi-key routing:
- `ARMOR_KEY_ROUTES` (format: "prefix1=key1,prefix2=key2,*=default")

Authentication:
- `ARMOR_AUTH_ACCESS_KEY` (auto-generated if not set)
- `ARMOR_AUTH_SECRET_KEY` (auto-generated if not set)
- `ARMOR_AUTH_<NAME>_ACCESS_KEY` (named credentials)
- `ARMOR_AUTH_<NAME>_SECRET_KEY`
- `ARMOR_AUTH_<NAME>_ACL` (format: "bucket:prefix,...")

Cache:
- `ARMOR_CACHE_MAX_ENTRIES` (default: 10000)
- `ARMOR_CACHE_TTL` (default: 300)
- `ARMOR_LIST_CACHE_MAX_ENTRIES` (default: 1000)
- `ARMOR_LIST_CACHE_TTL` (default: 60)

Readiness:
- `ARMOR_READYZ_CACHE_TTL` (default: 30)
- `ARMOR_CANARY_DISABLED` (default: false)

Manifest (Phase 4):
- `ARMOR_MANIFEST_ENABLED` (default: true)
- `ARMOR_MANIFEST_PREFIX` (default: .armor/manifest)
- `ARMOR_MANIFEST_COMPACTION_INTERVAL` (default: 3600)
- `ARMOR_MANIFEST_COMPACTION_THRESHOLD` (default: 1000)

Pre-signed URLs:
- `ARMOR_PRESIGN_SECRET` (optional, uses auth secret if not set)
- `ARMOR_PRESIGN_BASE_URL` (default: /share)

Dashboard:
- `ARMOR_DASHBOARD_USER`
- `ARMOR_DASHBOARD_PASS`
- `ARMOR_DASHBOARD_TOKEN`

Other:
- `ARMOR_WRITER_ID` (default: hostname)

### Related Investigation
This confirms the findings from bead **bf-83o2**, which previously investigated Pluck's `exclude_labels` configuration with the same results.

### Conclusion
**ARMOR does not use Pluck or have any Pluck configuration.** If Pluck configuration is expected:
1. It may be in a different repository (e.g., `declarative-config`)
2. It may be part of an external monitoring/observability system
3. The task may have been intended for a different codebase
