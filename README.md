# ARMOR

**Authenticated Range-readable Managed Object Repository**

ARMOR is an S3-compatible proxy server that transparently encrypts data before storing it in [Backblaze B2](https://www.backblaze.com/cloud-storage) and serves downloads through Cloudflare for zero-egress cost. Any S3-compatible client — boto3, AWS CLI, DuckDB, rclone — works without modification.

- **Zero-knowledge encryption** — data is encrypted before it leaves ARMOR; B2 only ever stores ciphertext
- **Zero egress fees** — downloads route through Cloudflare via the Bandwidth Alliance
- **Seekable encryption** — AES-256-CTR with 64KB blocks enables byte-range reads without decrypting the whole file
- **DuckDB-compatible** — query encrypted Parquet files with column pruning and predicate pushdown intact
- **Multi-key routing** — different master keys for different path prefixes; automatic key selection per object

## Quick Start

### Docker

```bash
docker run -d \
  -p 9000:9000 \
  -p 9001:9001 \
  -e ARMOR_B2_REGION=us-east-005 \
  -e ARMOR_B2_ACCESS_KEY_ID=your-key-id \
  -e ARMOR_B2_SECRET_ACCESS_KEY=your-key-secret \
  -e ARMOR_BUCKET=your-bucket \
  -e ARMOR_CF_DOMAIN=your-cf-domain.example.com \
  -e ARMOR_MEK=$(openssl rand -hex 32) \
  -e ARMOR_AUTH_ACCESS_KEY=my-access-key \
  -e ARMOR_AUTH_SECRET_KEY=my-secret-key \
  ronaldraygun/armor:0.1.1911
```

> **Note:** The ARMOR CI pipeline auto-bumps the VERSION file on every build and publishes the container image as `ronaldraygun/armor:<version>` to Docker Hub. Always pin to a specific version tag in production deployments. Use the latest published tag from [Docker Hub](https://hub.docker.com/r/ronaldraygun/armor/tags).

### Client Configuration

ARMOR provides a `client-config` command that generates known-good, copy-pasteable configuration snippets for common S3-compatible tools:

```bash
armor client-config --for aws-cli --endpoint http://localhost:9000 --bucket my-bucket
armor client-config --for rclone --endpoint http://localhost:9000
armor client-config --for boto3 --endpoint http://localhost:9000 --credential backup-writer
armor client-config --for duckdb --endpoint http://localhost:9000
armor client-config --for litestream --endpoint http://localhost:9000
armor client-config --for barman --endpoint http://localhost:9000
```

Supported tools: `aws-cli`, `rclone`, `boto3`, `or `barman`. The command includes:

- Endpoint URL configuration
- Path-style addressing (required for B2/ARMOR)
- Region placeholder (required by clients but unused by ARMOR)
- Credential environment variable names (never values)
- **Format version 2:** Multipart upload constraints (block-aligned chunk sizes, minimum part sizes)
- **Format version 3:** No multipart constraints (any part size, any order, any concurrency)

See the section on [Multipart Upload Constraints](#multipart-upload-constraints) for details on format version differences.

#### Quick Examples

```bash
# AWS CLI
aws --endpoint-url http://localhost:9000 s3 cp file.txt s3://bucket/key

# boto3 (Python)
import boto3
s3 = boto3.client('s3',
    endpoint_url='http://localhost:9000',
    aws_access_key_id='my-access-key',
    aws_secret_access_key='my-secret-key')
s3.upload_file('local.txt', 'bucket', 'key')

# DuckDB
INSTALL httpfs;
LOAD httpfs;
SET s3_endpoint='localhost:9000';
SET s3_access_key_id='my-access-key';
SET s3_secret_access_key='my-secret-key';
SELECT * FROM read_parquet('s3://bucket/data.parquet');
```

## Cost Model

| Component | Cost |
|-----------|------|
| Storage | ~$6–7/TB/month |
| Compression savings (optional) | Varies by data type — Optional zstd via `ARMOR_COMPRESS=true` reduces storage for compressible data: manifests (2–5×), WAL (3–5×), JSON logs (2–4×). Parquet/columnar: minimal additional benefit (already compressed internally). See ADR-007. |
| Egress (via Cloudflare Bandwidth Alliance) | $0 |
| B2 API calls | $0 |
| Cloudflare (free plan) | $0 |
| **Total** | **~$6–7/TB/month** (base), lower with compression for compressible workloads |

## Architecture

### Upload Path (direct to B2 — ingress is free)

```
┌──────────┐     ┌──────────────┐     ┌──────────┐
│  Client   │────▶│    ARMOR     │────▶│    B2    │
│           │     │  encrypt +   │     │          │
│           │     │  upload      │     │          │
└──────────┘     └──────────────┘     └──────────┘
```

### Download Path (through Cloudflare — egress is free)

```
┌──────────┐     ┌────────────┐     ┌────────────┐     ┌──────────┐
│  Client   │◀───│ Cloudflare │◀───│  Cloudflare │◀───│    B2    │
│  ARMOR    │    │   Edge     │    │  PNI Link   │    │          │
│  decrypt  │    │  (cache)   │    │  (free)     │    │          │
└──────────┘     └────────────┘     └────────────┘     └──────────┘
```

### DuckDB Query Path (seekable decryption)

DuckDB issues byte-range GET requests for specific row groups and columns. ARMOR decrypts only the requested 64KB blocks, so column pruning and predicate pushdown remain effective:

```
DuckDB                          ARMOR                       Cloudflare → B2
  │                                │                              │
  ├─ read footer (last 8 bytes) ──▶├─ Range GET (encrypted) ────▶│
  │◀── decrypted footer ──────────┤◀── ciphertext ──────────────┤
  │                                │                              │
  ├─ read col_a, row group 3 ────▶├─ Range GET (3 blocks) ─────▶│
  │◀── decrypted column chunk ────┤◀── ciphertext ──────────────┤
  │                                │                              │
  └─ result set                    └                              └
```

## Encryption Design

```
Master Key (MEK)
 │  stored locally, never uploaded
 │
 └─▶ wraps ──▶ Data Encryption Key (DEK)
                │  random per-file, wrapped copy in B2 metadata
                │
                └─▶ encrypts ──▶ File Data
                                   AES-256-CTR, 64KB blocks
                                   per-block HMAC-SHA256
                                   seekable random access
```

Key rotation re-wraps DEKs without re-uploading file data — a metadata-only operation.

## Security Model

| Threat | Mitigation |
|--------|-----------|
| B2 data breach | All stored data is AES-256-CTR encrypted with per-file DEKs — useless without MEK |
| Cloudflare CDN inspection | All cached content is ciphertext — CDN sees only opaque blobs |
| Man-in-the-middle | TLS on ARMOR listener + client-side encryption — plaintext never leaves ARMOR |
| ARMOR server compromise | MEK exposed — rotate immediately; per-file DEKs limit blast radius |
| Network sniffing (client ↔ ARMOR) | TLS on ARMOR listener or localhost-only binding |
| Public bucket enumeration | Attacker can list/download ciphertext — indistinguishable from random bytes without MEK |
| Bit-flipping on ciphertext | Per-block HMAC-SHA256 detects any modification |
| Block reordering/truncation | Block index implicit in offset; HMAC table length validates block count |
| Unauthorized access | ARMOR-side SigV4 authentication + prefix/verb ACLs (not B2 access control) |
| V1 keystream reuse | Version 1 envelopes had CTR counter bug (keystream reuse between adjacent blocks) — migration to Version 2 required. See [ADR-005](docs/adr/005-ctr-counter-stride-fix.md) and plan.md Phase 8.1 |

## Configuration

ARMOR is configured via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_LISTEN` | No | `0.0.0.0:9000` | S3 API listen address |
| `ARMOR_ADMIN_LISTEN` | No | `127.0.0.1:9001` | Admin API (key rotation, canary, audit) |
| `ARMOR_B2_REGION` | Yes | — | B2 region (e.g., `us-east-005`) |
| `ARMOR_B2_ACCESS_KEY_ID` | Yes | — | B2 application key ID |
| `ARMOR_B2_SECRET_ACCESS_KEY` | Yes | — | B2 application key |
| `ARMOR_BUCKET` | Yes | — | B2 bucket name |
| `ARMOR_PREFIX` | No | — | Key prefix for shared bucket deployments (e.g., `kalshi-tape/`). All keys are stored with this prefix in B2 but are transparent to S3 clients (see ADR-001) |
| `ARMOR_CF_DOMAIN` | Yes | — | Cloudflare domain CNAME'd to B2 |
| `ARMOR_MEK` | Yes | — | Master encryption key (hex, 32 bytes) |
| `ARMOR_AUTH_ACCESS_KEY` | Yes* | — | Client access key |
| `ARMOR_AUTH_SECRET_KEY` | Yes* | — | Client secret key |
| `ARMOR_BLOCK_SIZE` | No | `65536` | Encryption block size (bytes) |
| `ARMOR_COMPRESS` | No | `false` | Legacy alias for `ARMOR_COMPRESS_RULES="*=zstd"` (all files compressed). Prefer `ARMOR_COMPRESS_RULES` for fine-grained control. Multipart uploads are rejected when compression is enabled. Compressed objects do not support byte-range reads. See ADR-007. |
| `ARMOR_COMPRESS_RULES` | No | — | Comma-separated compression rules: `<suffix>|<content-type>=zstd|none`. First match wins. Examples: `.jsonl=zstd,.wal=zstd,application/json=zstd,*=none`. Per-request override via `x-amz-meta-armor-compress: true|false` header. Only applies to v3 single-PUT format. See ADR-007. |
| `ARMOR_READ_CONCURRENCY` | No | `16` | Maximum concurrent ranged reads |
| `ARMOR_WRITER_ID` | No | (hostname) | Provenance chain writer ID |
| `ARMOR_DASHBOARD_USER` | No | — | Dashboard HTTP Basic Auth username |
| `ARMOR_DASHBOARD_PASS` | No | — | Dashboard HTTP Basic Auth password |
| `ARMOR_DASHBOARD_TOKEN` | No | — | Dashboard Bearer token |

### Multi-Key Routing

Route different path prefixes to different master keys:

```bash
ARMOR_MEK=<hex>                           # default key
ARMOR_MEK_SENSITIVE=<hex>                 # named key
ARMOR_MEK_ARCHIVE=<hex>                   # named key
ARMOR_KEY_ROUTES="data/pii/*=sensitive,archive/*=archive,*=default"
```

Routes use longest-prefix matching; the trailing `/*` is shorthand for the
path prefix (`data/pii/` and `archive/` above). Objects without a matching
route use the default key. Rotate one key at a time with
`POST /admin/key/rotate?key-id=sensitive`; omitting `key-id` rotates only the
default key.

### Authentication

ARMOR uses its own credential system for client authentication. **These ARMOR credentials are separate from your B2 credentials** — ARMOR validates clients locally, then uses its own B2 credentials to talk to the backend. This means:

- B2 credentials never leave the ARMOR server
- Multiple clients can share ARMOR with different access keys and permissions
- Access keys can be scoped per-bucket or per-prefix

#### Default Credential

The simplest deployment uses a single static key pair:

```bash
ARMOR_AUTH_ACCESS_KEY=my-access-key
ARMOR_AUTH_SECRET_KEY=my-secret-key
```

At least one credential must be configured (either `ARMOR_AUTH_ACCESS_KEY`/`ARMOR_AUTH_SECRET_KEY`, named credentials, or `ARMOR_AUTH_FILE`). If no credentials are configured, ARMOR will fail to start with a clear error message.

#### Named Credentials with ACLs

For multi-user deployments, define any number of named credentials via environment triplets — one `ACCESS_KEY`, one `SECRET_KEY`, and one optional `ACL`:

```bash
# Credential named "READONLY" (the name is for your bookkeeping)
ARMOR_AUTH_READONLY_ACCESS_KEY=reader-key
ARMOR_AUTH_READONLY_SECRET_KEY=reader-secret
ARMOR_AUTH_READONLY_ACL="mybucket:readonly/*"

# Credential named "WRITER"
ARMOR_AUTH_WRITER_ACCESS_KEY=writer-key
ARMOR_AUTH_WRITER_SECRET_KEY=writer-secret
ARMOR_AUTH_WRITER_ACL="mybucket:*,otherbucket:uploads/*"
```

**ACL Format**

An ACL string grants scoped access to specific bucket and prefix combinations:

- **Syntax:** `bucket:prefix[:actions]`
- **Multiple rules:** Comma-separated (`bucket1:prefix1,bucket2:prefix2`)
- **Wildcard bucket:** Use `*` to match all buckets
- **Wildcard prefix:** Use `*` or empty string to match all keys

**Action Verbs**

ACLs support fine-grained action verbs per [ADR-012](docs/adr/012-authorization-action-verbs-and-consumer-separation.md). If no actions are specified, all verbs are permitted (backward compatible).

| Verb | S3 Operations Covered |
|------|----------------------|
| `get` | GetObject, HeadObject |
| `put` | PutObject, CreateMultipartUpload, UploadPart, CompleteMultipartUpload, CopyObject (destination) |
| `delete` | DeleteObject, DeleteObjects, AbortMultipartUpload |
| `list` | ListObjectsV2, ListMultipartUploads |

Specify actions as the optional third segment, separated by `:` and using `+` or spaces to combine verbs:

```bash
# All verbs on logs/ prefix (no action segment = all permitted)
ARMOR_AUTH_LOGS_ACL="mybucket:logs/*"

# Only GET and LIST on readonly/ prefix
ARMOR_AUTH_READONLY_ACL="mybucket:readonly/*:get+list"

# Only PUT and LIST on backups/ prefix (append-only backup writer)
ARMOR_AUTH_BACKUP_ACL="mybucket:backups/*:put+list"
```

**Append-Only Backup Writers**

The standard pattern for backup systems is `put+list` — the client can write new backups and list what it wrote, but cannot read, overwrite, or delete existing data:

```bash
ARMOR_AUTH_BACKUP_WRITER_ACCESS_KEY=backup-writer
ARMOR_AUTH_BACKUP_WRITER_SECRET_KEY=backup-secret
ARMOR_AUTH_BACKUP_WRITER_ACL="mybucket:backups/*:put+list"
```

**Overwrite-as-Destruction Risk:** Without bucket versioning enabled, a compromised `put`-only credential can still overwrite existing objects by re-uploading poisoned data (S3 `PutObject` overwrites by default). Append-only writers mitigate but do not eliminate this risk in v1 — the credential cannot delete, but it can still destroy data by overwriting. This is accepted residual risk; revisit if B2 versioning is enabled.

**Multi-Bucket Example**

```bash
# Full access to one bucket, read-only to another
ARMOR_AUTH_CROSSBUCKET_ACL="bucket-primary:*:get+put+delete+list,bucket-audit:logs/*:get+list"
```

**Empty ACL**

If a credential has no `ACL` defined, it has full access to the configured `ARMOR_BUCKET`. This is the default for the unnamed `ARMOR_AUTH_*` pair.

#### Credentials from a YAML File

For deployments managed by Kubernetes or external secret systems, credentials can be loaded from a YAML file:

```bash
ARMOR_AUTH_FILE=/etc/armor/credentials.yaml
```

The YAML file uses the same schema and ACL parser as environment triplets:

```yaml
credentials:
  - name: FORGEJO_BACKUP
    access_key: "forgejo-backup-key"
    secret_key: "forgejo-backup-secret"
    acl: "iad-ci:forgejo-backup/*:put+list"

  - name: READONLY_USER
    access_key: "readonly-key"
    secret_key: "readonly-secret"
    acl: "mybucket:readonly/*:get+list"

  - name: FULL_ACCESS
    access_key: "full-key"
    secret_key: "full-secret"
    # No ACL means full access to configured bucket
```

**File Loading Behavior**

- File credentials are **merged** with environment-defined credentials
- **Environment credentials win** on access key collision (logged at WARN)
- Duplicate access keys within the file are skipped (first wins)
- File permissions are not checked (Kubernetes mounts manage this)
- Validation errors name the entry index and field, never the values

**Why Use a File?**

- Kubernetes deployments: mount a single Secret/ConfigMap instead of many env vars
- External secret systems: sync credentials from a central source
- Hot reloading: change credentials without pod restart (future feature)

### Secondary Backend

ARMOR supports an optional secondary backend for disaster recovery, backup, or multi-region replication. Secondary backends are configured via environment variables in one of two formats:

#### Colon-Separated Format (ARMOR_SECONDARY_BACKEND)

The secondary backend can be configured via a single colon-separated environment variable:

```bash
ARMOR_SECONDARY_BACKEND="filesystem:/backup/armor"
ARMOR_SECONDARY_BACKEND="b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEYID:SECRET:mybucket"
```

- **Filesystem format:** `filesystem:/path` - local filesystem backend at the given path
- **B2 format:** `b2:region:endpoint:accessKeyId:secretKey:bucket` - B2 S3 backend

When `ARMOR_SECONDARY_BACKEND` is unset or empty, the secondary backend is disabled.

#### Individual Variable Format (ARMOR_SECONDARY_B2_*)

For B2 secondary backends, individual environment variables can be used instead of the colon-separated format:

```bash
ARMOR_SECONDARY_B2_ENDPOINT=https://s3.us-east-005.backblazeb2.com
ARMOR_SECONDARY_B2_KEY_ID=your-key-id
ARMOR_SECONDARY_B2_KEY=your-key-secret
ARMOR_SECONDARY_B2_BUCKET=your-bucket
```

**Deprecated variable names:** The following old variable names are still supported for backward compatibility but will be removed in a future release:

- `B2_ENDPOINT` → Use `ARMOR_SECONDARY_B2_ENDPOINT` instead
- `B2_KEY_ID` → Use `ARMOR_SECONDARY_B2_KEY_ID` instead
- `B2_KEY` → Use `ARMOR_SECONDARY_B2_KEY` instead
- `B2_BUCKET` → Use `ARMOR_SECONDARY_B2_BUCKET` instead

When old variable names are used, ARMOR logs a deprecation warning at startup. New deployments should use the `ARMOR_SECONDARY_B2_*` names.

**Precedence:** If both new and old variable names are set, the new names take precedence. The colon-separated `ARMOR_SECONDARY_BACKEND` format takes precedence over individual variables when both are configured.

## S3 API Coverage

### Transforming Operations (encryption/decryption applied)

| Operation | Support |
|-----------|---------|
| PutObject | Full (streaming for large files) |
| GetObject | Full (range reads) |
| HeadObject | Full (plaintext size, conditionals) |
| CopyObject | Full (DEK re-wrapping, cross-bucket) |
| CreateMultipartUpload | Full |
| UploadPart | Full |
| CompleteMultipartUpload | Full |
| AbortMultipartUpload | Full |
| ListParts | Full |
| ListMultipartUploads | Full |

### Passthrough Operations

| Operation | Support |
|-----------|---------|
| ListObjectsV2 | Full (size correction, `.armor/` filter) |
| DeleteObject | Full |
| DeleteObjects | Full |
| ListBuckets | Full |
| CreateBucket / DeleteBucket / HeadBucket | Full |
| Lifecycle configuration | Full |
| Object Lock / Retention / Legal Hold | Full |

**Reserved Namespace: `.armor/`**

The `.armor/` prefix is reserved for ARMOR internal use. Client operations targeting keys with this prefix return `403 AccessDenied`. This protects:

- `.armor/chain/<writer>/*` — Tamper-evident provenance chain entries
- `.armor/chain-head/<writer>` — Provenance chain head pointers
- `.armor/manifest/<writer>/*` — Manifest delta files (IV + wrapped DEK entries)
- `.armor/hmac/<sha256>` — Multipart upload HMAC sidecars
- `.armor/rotation-state.json` — In-progress key rotation state
- `.armor/multipart/*.state` — Crash recovery state for multipart uploads
- `.armor/canary/*` — Health check canary objects

Internal ARMOR components (provenance recorder, manifest persistence, canary, key rotation, multipart state manager) access these keys directly through the backend layer, bypassing the S3 handler guard.

## Multipart Upload Constraints

ARMOR's encryption scheme requires part sizes to be block-aligned for correct counter offset calculation. The constraints depend on the configured write format version:

### Format Version 2 (Default)

**Constraint:** Uniform part sizes that are multiples of the ARMOR block size (64 KiB)

- **Minimum part size:** 5 MiB (S3 requirement, except final part)
- **Part size must be:** A multiple of 64 KiB (67108864 bytes = 64 MiB recommended)
- **Part 1** pins the uniform part size for the entire upload
- **Parts arriving before part 1** receive HTTP 503 SlowDown (retryable)
- **Block alignment** is required for all parts except the final short part and part 1 itself

**Impact:** Clients must use block-aligned chunk sizes. Tools that emit non-uniform part sizes (e.g., Barman's `chunk_size + 512` pattern) fail with `InvalidPartSize` when backups exceed the single-part threshold.

**Workarounds for format version 2:**
- AWS CLI: Set `multipart_chunksize` to 67108864 (64 MiB)
- rclone: Use `--s3-chunk-size 67108864` (64 MiB)
- boto3: Configure `TransferConfig(multipart_chunksize=64*1024*1024)`
- Barman: Use `--chunk-size=1024` (1 GiB) to stay in single-part mode for most backups
- Litestream: Set snapshot size to 64 MiB minimum

### Format Version 3 (Future)

**No constraints:** Any part size ≥ 5 MiB, any order, any concurrency

- Part sizes can vary (non-uniform multipart uploads supported)
- No block alignment requirement
- Out-of-order and concurrent part uploads fully supported
- Per-part cumulative offset tracking

**Migration:** Format version 3 is not yet released. When available, existing format version 2 objects can be optionally migrated via `armor migrate` (see [Format Migration](#format-migration)).

### Checking Your Format Version

```bash
# Check which format version your ARMOR instance writes
armor version
# Output includes: format_write_version: 2 or 3

# Generate tool-specific config with appropriate constraints
armor client-config --for aws-cli --endpoint http://localhost:9000
# Output includes multipart settings only when format_write_version=2
```

## Web Dashboard

A web dashboard for bucket browsing, encryption status, and metrics is available on the admin port (default `127.0.0.1:9001`):

```bash
open http://localhost:9001/dashboard
```

Features:
- Bucket browsing with prefix-based navigation
- Encryption status badges per object (key name, ARMOR vs. unencrypted)
- Metadata cache hit rates
- Real-time metrics: requests, bytes transferred, uptime, canary status

See [docs/dashboard.md](docs/dashboard.md) for full documentation.

## Admin API

Key management and monitoring endpoints on the admin listener (`127.0.0.1:9001`):

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/healthz` | GET | Liveness check |
| `/readyz` | GET | Readiness check (verifies B2 connectivity) |
| `/metrics` | GET | Prometheus metrics |
| `/admin/key/verify` | GET | Verify MEK can decrypt the canary object |
| `/admin/key/rotate` | POST | Rotate one MEK (`?key-id=name`; default key when omitted) — re-wraps matching DEKs, no file re-upload |
| `/admin/key/export` | GET | Export current MEK (`?confirm=yes`) |
| `/armor/audit` | GET | Walk provenance chains, verify integrity |
| `/admin/presign` | POST | Generate pre-signed share URL |
| `/armor/canary` | GET | Canary integrity status |
| `/dashboard` | GET | Web dashboard |

## Repository Structure

```
ARMOR/
├── cmd/armor/main.go          # Entrypoint
├── internal/
│   ├── server/                # S3 API handlers, auth
│   ├── crypto/                # Encryption, decryption, envelope key management
│   ├── backend/               # B2 S3 client, Cloudflare download routing
│   ├── canary/                # Self-healing integrity monitor
│   ├── config/                # Configuration loading (env vars)
│   ├── keymanager/            # Multi-key routing
│   ├── dashboard/             # Web dashboard UI and metrics
│   ├── presign/               # Pre-signed URL generation
│   ├── provenance/            # Cryptographic audit chain
│   ├── logging/               # Structured JSON logging
│   └── metrics/               # Prometheus metrics
├── deploy/kubernetes/         # Kubernetes manifests
├── tests/integration/         # Integration tests (requires real B2 + Cloudflare)
└── docs/
    ├── dashboard.md
    ├── cloudflare-setup.md    # DNS configuration for zero-egress downloads
    └── research/
```

## Documentation

- [Disaster Recovery](docs/disaster-recovery.md) — MEK backup/escrow, restore drills, rotation failure recovery
- [Web Dashboard](docs/dashboard.md) — Bucket browsing, encryption status, cache statistics
- [Cloudflare Setup](docs/cloudflare-setup.md) — DNS configuration for zero-egress B2 downloads
- [Integration Tests](tests/integration/README.md) — Testing against real B2 + Cloudflare

## Disaster Recovery / Offline Decryption

ARMOR includes a `decrypt` subcommand for recovering encrypted objects without a running ARMOR server. This enables disaster recovery scenarios where you have:

- The Master Encryption Key (MEK)
- Access to B2 (or a local copy of an encrypted object)

### Usage

#### Decrypt from B2

```bash
# Decrypt directly from B2 (requires B2 credentials)
armor decrypt \
  -mek 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef \
  -input b2://my-bucket/path/to/file.encrypted \
  -output recovered-file.txt

# Using MEK from environment
export ARMOR_MEK=0123456789abcdef...
armor decrypt -input b2://my-bucket/file -output recovered.txt

# With verbose output
armor decrypt -mek HEX -input b2://bucket/file -v -output recovered.txt
```

Multipart objects (the usual shape for large backups) need no special flags
here: the tool detects the `x-amz-meta-armor-multipart` marker in object
metadata and switches to the headerless layout automatically.

#### Decrypt from Local File

For local files, you need the wrapped DEK (from `x-amz-meta-armor-wrapped-dek` metadata):

```bash
armor decrypt \
  -mek 0123456789abcdef... \
  -input /path/to/encrypted.bin \
  -wrapped-dek WWF...base64... \
  -output plaintext.bin
```

For a local copy of a **multipart** object (headerless ciphertext — no envelope
header), two extra inputs are required, since the multipart layout has no
header to read them from:

```bash
armor decrypt \
  -mek 0123456789abcdef... \
  -input /path/to/multipart-object.bin \
  -wrapped-dek WWF...base64... \
  -iv aabbccdd...00112233 \
  -sidecar /path/to/object.hmac.json \
  -output plaintext.bin
```

- `-iv` — the object IV, from the `x-amz-meta-armor-iv` metadata field (hex).
- `-sidecar` — the JSON HMAC sidecar the server stores alongside every
  multipart object at `.armor/hmac/<sha256-of-object-key>` (download it with
  any S3 client; the hex key is `sha256sum` of the object key string).

### Key Requirements

- **MEK (Master Encryption Key)**: 32-byte hex string
- **For B2**: `ARMOR_B2_REGION`, `ARMOR_B2_ENDPOINT`, `ARMOR_B2_ACCESS_KEY_ID`, `ARMOR_B2_SECRET_ACCESS_KEY`
- **For local files**: Wrapped DEK (base64, from object metadata)

### Multi-Key Support

If your ARMOR deployment uses named keys (via `ARMOR_KEY_ROUTES`), specify the key ID:

```bash
armor decrypt \
  -mek <hex-for-specific-key> \
  -input b2://bucket/file \
  -key-id sensitive \
  -output recovered.txt
```

The key ID comes from the `x-amz-meta-armor-key-id` metadata header.

### Verification

The decrypt tool automatically:

- Verifies per-block HMAC-SHA256 integrity on every object
- Validates the plaintext SHA-256 checksum for single-PUT objects
- Detects corrupted blocks or wrong MEK

**Multipart caveat:** multipart objects store a placeholder plaintext SHA-256
(the digest of the empty string) rather than the true whole-object digest, so
for those the tool verifies integrity via per-block HMACs only and skips the
SHA check. Do not compare `sha256sum` of a recovered multipart object against
`x-amz-meta-armor-plaintext-sha256` — it will not match, by design.

Exit codes:
- `0`: Success
- `1`: Decryption failed (wrong MEK, corrupted data, HMAC mismatch)

### Example Workflow

```bash
# 1. List objects to find the target
aws s3 ls --endpoint-url http://localhost:9000 s3://bucket/

# 2. Get metadata to see key requirements
aws s3api head-object --endpoint-url http://localhost:9000 \
  --bucket bucket --key file

# 3. Decrypt with the correct MEK
armor decrypt -mek $ARMOR_MEK -input b2://bucket/file -output recovered

# 4. Verify the recovered file
#    Single-PUT objects only: should match x-amz-meta-armor-plaintext-sha256.
#    Multipart objects carry a placeholder SHA there — the decrypt tool's
#    per-block HMAC verification (a non-zero exit on failure) is the check.
sha256sum recovered
```

### Backward Compatibility

For backward compatibility during the transition period, the standalone `armor-decrypt` binary remains available. It delegates to `armor decrypt` internally and can be used interchangeably. New deployments should prefer `armor decrypt` directly.

## License

MIT
