# ARMOR

[![iad-ci](https://img.shields.io/github/checks-status/jedarden/ARMOR/main?label=iad-ci)](docs/release-process.md)

**Authenticated Range-readable Managed Object Repository**

ARMOR is an S3-compatible proxy server that encrypts data before storing it in
[Backblaze B2](https://www.backblaze.com/cloud-storage) and serves downloads
through Cloudflare for zero-egress cost. Any S3-compatible client (boto3, AWS
CLI, DuckDB, rclone, litestream, barman) works without modification.

- **Zero-knowledge encryption** — data is encrypted before it leaves ARMOR; B2 only ever stores ciphertext
- **Zero egress fees** — downloads route through Cloudflare via the Bandwidth Alliance
- **Seekable encryption** — AES-256-CTR with 64 KB blocks enables byte-range reads without decrypting the whole file
- **DuckDB-compatible** — query encrypted Parquet files with column pruning and predicate pushdown intact
- **Multi-key routing** — different master keys for different path prefixes; automatic key selection per object

## Install

**Container image:**

```bash
docker pull ghcr.io/jedarden/armor:<version>
```

`<version>` is a release counter such as `0.1.1969`; the current one is in the
[`VERSION`](VERSION) file. Only pinned version tags are published — there is no
`latest` or other floating tag.

**Go toolchain** (Go 1.25 or newer):

```bash
go install github.com/jedarden/armor/cmd/armor@v<version>
```

**Platform note:** published images are built for linux/amd64 only. On Apple
Silicon, add `--platform linux/amd64` to `docker pull` and `docker run`.

## Contents

- [Install](#install)
- [Quick Start](#quick-start)
- [Subcommands](#subcommands)
- [Production Docker deployment](#production-docker-deployment)
- [Client configuration](#client-configuration)
- [Cost model](#cost-model)
- [Architecture](#architecture)
- [Encryption design](#encryption-design)
- [Security model](#security-model)
- [Configuration reference](#configuration-reference)
- [Bucket aliases](#bucket-aliases)
- [Multi-key routing](#multi-key-routing)
- [Authentication](#authentication)
- [Secondary backend](#secondary-backend)
- [S3 API coverage](#s3-api-coverage)
- [Multipart upload constraints](#multipart-upload-constraints)
- [HTTP endpoints](#http-endpoints)
- [Web dashboard](#web-dashboard)
- [Disaster recovery](#disaster-recovery)
- [Releases and versioning](#releases-and-versioning)
- [Repository structure](#repository-structure)
- [Documentation](#documentation)
- [License](#license)

## Quick Start

### Images

Every release publishes the server image to the GitHub Container Registry:

| Image | Access |
|---|---|
| `ghcr.io/jedarden/armor:<version>` | Public. Pulls need no credentials. **Use this one.** |

The same tags are also pushed to a private Docker Hub namespace used by the
fleet's own deployments; anonymous pulls there return 401, so everything in
this document points at GHCR instead. The companion images (restore verifier,
fleet console) are published to that internal namespace only.

`<version>` is a release counter such as `0.1.1969`. The current one is in
[`VERSION`](VERSION); every published version has an entry in
[`CHANGELOG.md`](CHANGELOG.md) and a release on GitHub. There is no `latest`
tag; pin a version.

### Local demo (Docker only)

The demo uses a temporary filesystem backend and fixed, non-secret credentials.
It does not need Backblaze B2, Cloudflare, or an AWS account.

Start ARMOR in the background:

```bash
docker run -d --name armor-demo \
  -p 9000:9000 \
  -p 9001:9001 \
  ghcr.io/jedarden/armor:<version> demo \
  --listen 0.0.0.0:9000 \
  --admin-listen 0.0.0.0:9001
```

Run these three client commands from any shell. They use the official AWS CLI
container, so the only software required on the machine is Docker:

```bash
# 1. Connectivity check: this succeeds even when the demo bucket is empty.
docker run --rm --network container:armor-demo \
  -e AWS_ACCESS_KEY_ID=armor \
  -e AWS_SECRET_ACCESS_KEY=armor-demo-secret \
  -e AWS_DEFAULT_REGION=us-east-1 \
  amazon/aws-cli:2.29.0 \
  --endpoint-url http://127.0.0.1:9000 s3 ls

# 2. Create the bucket used by the demo.
docker run --rm --network container:armor-demo \
  -e AWS_ACCESS_KEY_ID=armor \
  -e AWS_SECRET_ACCESS_KEY=armor-demo-secret \
  -e AWS_DEFAULT_REGION=us-east-1 \
  amazon/aws-cli:2.29.0 \
  --endpoint-url http://127.0.0.1:9000 s3 mb s3://demo-bucket

# 3. List the demo bucket.
docker run --rm --network container:armor-demo \
  -e AWS_ACCESS_KEY_ID=armor \
  -e AWS_SECRET_ACCESS_KEY=armor-demo-secret \
  -e AWS_DEFAULT_REGION=us-east-1 \
  amazon/aws-cli:2.29.0 \
  --endpoint-url http://127.0.0.1:9000 s3 ls s3://demo-bucket
```

If the AWS CLI is already installed, the first check is equivalently:

```bash
AWS_ACCESS_KEY_ID=armor AWS_SECRET_ACCESS_KEY=armor-demo-secret \
  aws --endpoint-url http://localhost:9000 s3 ls
```

Stop and remove the demo when finished:

```bash
docker rm -f armor-demo
```

This workflow is guarded by an automated smoke test (`make test-docker-demo`),
which replays the commands above against the pinned image and fails if they
stop working.

### Build from source

```bash
git clone https://github.com/jedarden/ARMOR.git && cd ARMOR
make build            # every cmd/ binary into bin/, version injected from VERSION
bin/armor help
scripts/definition-of-done.sh --fast   # build + vet + script tests, the local gate
```

Go 1.25 or newer is required (`go.mod`). Contributors and agents: read
[AGENTS.md](AGENTS.md).

## Subcommands

`armor help` prints this list; `serve` is the default when no subcommand is given.

| Subcommand | What it does |
|---|---|
| `serve` | Start the S3-compatible server (default). Configured entirely by environment variables, see [Configuration reference](#configuration-reference) |
| `demo` | Start ARMOR with a temporary filesystem backend and fixed credentials (`--listen`, `--admin-listen`, `--dir`) |
| `check` | Verify a deployment: config, backend connectivity (HeadBucket), the Cloudflare path (ranged GET) and MEK correctness via the canary. Exit 0 ok, 1 config error, 2 connectivity or MEK failure |
| `client-config` | Print known-good configuration for `aws-cli`, `rclone`, `boto3`, `duckdb`, `litestream` or `barman` (`--for`, `--endpoint`, `--bucket`, `--credential`) |
| `decrypt` | Decrypt objects offline with only the MEK and B2 access (or a local ciphertext copy); see [Disaster recovery](#disaster-recovery) |
| `verify` | Verify encrypted objects for corruption: HMAC and digest checks over a prefix, a keys file, or a time window (`--prefix`, `--keys-file`, `--since`, `--quick`) |
| `migrate` | Client of `POST /admin/format/migrate`: migrate legacy-format objects to the current envelope format (`--admin-url`, `--target`, `--include`, `--dry-run`, `--watch`) |
| `version` | Print `armor <version> (<go>, <os>/<arch>)`. The same information is served as JSON at `/version` |
| `help` | Show the subcommand list |

## Production Docker deployment

For a B2-backed deployment, replace every placeholder with a value from your
environment. Keep the same MEK when restarting an instance; losing it makes
existing objects unreadable. Pin the image to a published version.

```bash
docker run -d --name armor \
  -p 9000:9000 \
  -p 127.0.0.1:9001:9001 \
  -e ARMOR_B2_REGION=us-east-005 \
  -e ARMOR_B2_ACCESS_KEY_ID=<b2-key-id> \
  -e ARMOR_B2_SECRET_ACCESS_KEY=<b2-key-secret> \
  -e ARMOR_BUCKET=<b2-bucket> \
  -e ARMOR_CF_DOMAIN=<cloudflare-domain> \
  -e ARMOR_MEK=<64-hex-character-mek> \
  -e ARMOR_AUTH_ACCESS_KEY=<armor-access-key> \
  -e ARMOR_AUTH_SECRET_KEY=<armor-secret-key> \
  -e ARMOR_ADMIN_TOKEN=<admin-bearer-token> \
  -e ARMOR_ADMIN_LISTEN=0.0.0.0:9001 \
  ghcr.io/jedarden/armor:<version>
```

Without `ARMOR_ADMIN_TOKEN` the key-management and migration endpoints are
disabled (fail-closed), which is fine for a plain proxy but leaves no way to
rotate keys. Verify a running instance with `armor check` (inside the
container) or `curl http://127.0.0.1:9001/version`.

## Client configuration

ARMOR provides a `client-config` command that generates known-good,
copy-pasteable configuration snippets for common S3-compatible tools:

```bash
armor client-config --for aws-cli --endpoint http://localhost:9000 --bucket my-bucket
armor client-config --for rclone --endpoint http://localhost:9000
armor client-config --for boto3 --endpoint http://localhost:9000 --credential backup-writer
armor client-config --for duckdb --endpoint http://localhost:9000
armor client-config --for litestream --endpoint http://localhost:9000
armor client-config --for barman --endpoint http://localhost:9000
```

The output includes the endpoint URL, path-style addressing (required), a
region placeholder (required by clients, unused by ARMOR), the credential
environment variable names (never values), and the multipart contract in force
for the configured write format together with a pointer to the tested
[multipart client-concurrency compatibility matrix](docs/multipart-client-compatibility.md).

### Quick examples

```bash
# AWS CLI
aws --endpoint-url http://localhost:9000 s3 cp file.txt s3://bucket/key
```

```python
# boto3
import boto3
s3 = boto3.client('s3',
    endpoint_url='http://localhost:9000',
    aws_access_key_id='my-access-key',
    aws_secret_access_key='my-secret-key')
s3.upload_file('local.txt', 'bucket', 'key')
```

```sql
-- DuckDB
INSTALL httpfs;
LOAD httpfs;
SET s3_endpoint='localhost:9000';
SET s3_access_key_id='my-access-key';
SET s3_secret_access_key='my-secret-key';
SELECT * FROM read_parquet('s3://bucket/data.parquet');
```

## Cost model

| Component | Cost |
|-----------|------|
| Storage | ~$6–7/TB/month |
| Compression savings (optional) | Varies by data type. Optional zstd via `ARMOR_COMPRESS_RULES` reduces storage for compressible data: manifests (2–5×), WAL (3–5×), JSON logs (2–4×). Parquet/columnar: minimal additional benefit. See [ADR-007](docs/adr/007-zstd-compression.md) |
| Egress (via Cloudflare Bandwidth Alliance) | $0 |
| B2 API calls | $0 |
| Cloudflare (free plan) | $0 |
| **Total** | **~$6–7/TB/month** (base), lower with compression for compressible workloads |

## Architecture

### Upload path (direct to B2; ingress is free)

```
┌──────────┐     ┌──────────────┐     ┌──────────┐
│  Client   │────▶│    ARMOR     │────▶│    B2    │
│           │     │  encrypt +   │     │          │
│           │     │  upload      │     │          │
└──────────┘     └──────────────┘     └──────────┘
```

### Download path (through Cloudflare; egress is free)

```
┌──────────┐     ┌────────────┐     ┌────────────┐     ┌──────────┐
│  Client   │◀───│ Cloudflare │◀───│  Cloudflare │◀───│    B2    │
│  ARMOR    │    │   Edge     │    │  PNI Link   │    │          │
│  decrypt  │    │  (cache)   │    │  (free)     │    │          │
└──────────┘     └────────────┘    └────────────┘     └──────────┘
```

### DuckDB query path (seekable decryption)

DuckDB issues byte-range GET requests for specific row groups and columns.
ARMOR decrypts only the requested 64 KB blocks, so column pruning and predicate
pushdown remain effective:

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

ARMOR is stateless: any instance with the same MEK, B2 credentials and
Cloudflare domain can serve the same bucket, and all authoritative state
(envelope metadata, key-rotation progress, provenance chain, manifest index)
lives in B2 under the reserved `.armor/` prefix.

## Encryption design

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

Key rotation re-wraps DEKs without re-uploading file data; it is a
metadata-only operation. The on-disk envelope is
[format version 3](docs/format/envelope-v3.md) (the default write format);
version 2 is still readable and can be written with `ARMOR_FORMAT_VERSION=2`.
Version 1 objects are readable but are migrated to v3 by `armor migrate`
because of the CTR counter defect described in
[ADR-005](docs/adr/005-ctr-counter-stride-fix.md).

## Security model

| Threat | Mitigation |
|--------|-----------|
| B2 data breach | All stored data is AES-256-CTR encrypted with per-file DEKs; useless without the MEK |
| Cloudflare CDN inspection | All cached content is ciphertext; the CDN sees only opaque blobs |
| Man-in-the-middle | TLS on the ARMOR listener plus server-side encryption; plaintext never leaves ARMOR |
| ARMOR server compromise | MEK exposed: rotate immediately; per-file DEKs limit blast radius |
| Network sniffing (client ↔ ARMOR) | TLS on the ARMOR listener or localhost-only binding |
| Public bucket enumeration | An attacker can list and download ciphertext, indistinguishable from random bytes without the MEK |
| Bit-flipping on ciphertext | Per-block HMAC-SHA256 detects any modification |
| Block reordering/truncation | Block index is implicit in the offset; the HMAC table length validates the block count |
| Unauthorized access | ARMOR-side SigV4 authentication plus prefix/verb ACLs (not B2 access control) |
| V1 keystream reuse | Version 1 envelopes had a CTR counter bug (keystream reuse between adjacent blocks). Migrate with `armor migrate`. See [ADR-005](docs/adr/005-ctr-counter-stride-fix.md) |

## Configuration reference

ARMOR is configured entirely by environment variables. Every variable read by
`internal/config` is listed here; a test (`internal/docsindex`) fails when one
is missing. Names in `<angle brackets>` are placeholders.

### Listeners and operation

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_LISTEN` | No | `0.0.0.0:9000` | S3 API listen address |
| `ARMOR_ADMIN_LISTEN` | No | `127.0.0.1:9001` | Admin API listen address (key management, migration, metrics, dashboard) |
| `ARMOR_ADMIN_TOKEN` | No | — | Bearer token that gates every `/admin/*` route and `/armor/audit`. When unset those routes are disabled and return 403 (fail-closed). Surrounding whitespace is trimmed so a provisioned value with a trailing newline still works |
| `ARMOR_ADMIN_READ_TIMEOUT` | No | disabled | Max time the admin listener spends reading a request. Go duration (`30s`, `5m`), or `0`/unset for no deadline |
| `ARMOR_ADMIN_WRITE_TIMEOUT` | No | disabled | Max time the admin listener spends writing a response. Leave unset for `POST /admin/key/rotate` and `GET /admin/key/ring?census=head`, which walk the whole bucket and can take hours |
| `ARMOR_LOG_LEVEL` | No | `info` | `debug`, `info`, `warn` or `error`. `debug` logs request and response headers and bodies |
| `ARMOR_WRITER_ID` | No | hostname | Provenance chain writer ID |
| `ARMOR_FORMAT_VERSION` | No | `3` | Envelope format written for new objects: `3` (current) or `2` (legacy). Reported by `armor version --json` and `/version` as `format_write_version` |
| `ARMOR_CANARY_DISABLED` | No | `false` | `true` skips the canary check in `/readyz` (readiness then reports 200 without verifying the MEK) |
| `ARMOR_ALLOW_NO_CREDENTIALS` | No | `false` | Start without any client credential. Set by the `demo` subcommand only; never in production |

### Backend

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_BACKEND` | No | `b2` | Primary backend: `b2` or `filesystem` |
| `ARMOR_FS_PATH` | With `filesystem` | — | Directory for the filesystem backend |
| `ARMOR_B2_REGION` | With `b2` | — | B2 region (e.g., `us-east-005`) |
| `ARMOR_B2_ENDPOINT` | No | `https://s3.<region>.backblazeb2.com` | B2 S3 endpoint override |
| `ARMOR_B2_ACCESS_KEY_ID` | With `b2` | — | B2 application key ID |
| `ARMOR_B2_SECRET_ACCESS_KEY` | With `b2` | — | B2 application key |
| `ARMOR_BUCKET` | Yes | — | Bucket name (both backends) |
| `ARMOR_BUCKET_ALIASES` | No | — | Comma-separated legacy bucket names served from `ARMOR_BUCKET` (see [Bucket aliases](#bucket-aliases)) |
| `ARMOR_PREFIX` | No | — | Key prefix for shared-bucket deployments (e.g., `kalshi-tape/`). Stored in B2, invisible to S3 clients ([ADR-001](docs/adr/001-bucket-prefix.md)) |
| `ARMOR_CF_DOMAIN` | No | — | Cloudflare domain CNAMEd to the bucket. When set, reads go through Cloudflare (free egress, edge cache); when unset, reads go to the B2 S3 endpoint directly and B2 egress applies. Ignored for the filesystem backend |

### Encryption and keys

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_MEK` | Yes | — | Master encryption key for the default key (hex, 32 bytes = 64 characters) |
| `ARMOR_MEK_<NAME>` | No | — | A named master key (same format). `<NAME>` is lower-cased; `DEFAULT` is reserved |
| `ARMOR_MEK_RING` | No | — | Comma-separated retired MEKs (hex) that remain valid for reading objects wrapped with the default key before a rotation |
| `ARMOR_MEK_<NAME>_RING` | No | — | The same for a named key; requires `ARMOR_MEK_<NAME>` |
| `ARMOR_KEY_ROUTES` | No | — | Prefix-to-key routes, e.g. `data/pii/*=sensitive,archive/*=archive,*=default` (see [Multi-key routing](#multi-key-routing)) |
| `ARMOR_BLOCK_SIZE` | No | `65536` | Encryption block size in bytes; a power of two, at least 4096 |
| `ARMOR_COMPRESS` | No | `false` | Legacy alias for `ARMOR_COMPRESS_RULES="*=zstd"`. Multipart uploads are rejected and byte-range reads are unsupported for compressed objects ([ADR-007](docs/adr/007-zstd-compression.md)) |
| `ARMOR_COMPRESS_RULES` | No | — | Comma-separated rules `<suffix>|<content-type>=zstd|none`, first match wins, e.g. `.jsonl=zstd,application/json=zstd,*=none`. Per-request override via `x-amz-meta-armor-compress: true|false`. Single-PUT v3 objects only |

### Client authentication

At least one credential must be configured or the server refuses to start
(unless `ARMOR_ALLOW_NO_CREDENTIALS=true`). See [Authentication](#authentication)
for the ACL syntax.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_AUTH_ACCESS_KEY` / `ARMOR_AUTH_SECRET_KEY` | One of these | — | The default (unnamed) credential; full access to `ARMOR_BUCKET` |
| `ARMOR_AUTH_<NAME>_ACCESS_KEY`, `ARMOR_AUTH_<NAME>_SECRET_KEY`, `ARMOR_AUTH_<NAME>_ACL` | One of these | — | A named credential with an optional ACL (`bucket:prefix[:verbs]`, comma-separated) |
| `ARMOR_AUTH_FILE` | One of these | — | YAML credentials file with the same schema; merged with environment credentials (environment wins on collision) and hot-reloaded when it changes |

### Read path and caches

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_READ_CONCURRENCY` | No | `16` | Maximum concurrent ranged reads per backend read |
| `ARMOR_CACHE_MAX_ENTRIES` | No | `10000` | Object metadata cache entries |
| `ARMOR_CACHE_TTL` | No | `300` | Metadata cache TTL in seconds |
| `ARMOR_LIST_CACHE_MAX_ENTRIES` | No | `1000` | List-result cache entries |
| `ARMOR_LIST_CACHE_TTL` | No | `60` | List-result cache TTL in seconds |

### Manifest index

The manifest index caches envelope metadata (IV, wrapped DEK) so reads do not
need a HEAD per object. It is stored under `.armor/manifest/` in the bucket.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_MANIFEST_ENABLED` | No | `true` | `false` or `0` disables the index |
| `ARMOR_MANIFEST_PREFIX` | No | `.armor/manifest` | Location of the index, relative to `ARMOR_PREFIX`. Must stay inside the tenant namespace (no absolute paths, no `../`) |
| `ARMOR_MANIFEST_COMPACTION_INTERVAL` | No | `3600` | Seconds between automatic compactions |
| `ARMOR_MANIFEST_COMPACTION_THRESHOLD` | No | `1000` | Delta entry count that triggers an early compaction |
| `ARMOR_MANIFEST_LOAD_TIMEOUT` | No | `480` | Seconds allowed for the startup manifest load; `0` is unbounded. Keep it under the pod's startupProbe budget: on timeout the server starts with an empty index instead of crash-looping |

### Dashboard and pre-signed URLs

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_DASHBOARD_USER` / `ARMOR_DASHBOARD_PASS` | No | — | HTTP Basic Auth for `/dashboard` |
| `ARMOR_DASHBOARD_TOKEN` | No | — | Bearer token for `/dashboard` |
| *(none of the three above)* | — | — | The dashboard then requires `ARMOR_ADMIN_TOKEN`, and returns 403 when that is unset too |
| `ARMOR_DASHBOARD_CREDENTIAL` | No | — | Access key of a configured named credential the dashboard signs uploads, downloads and deletes with. Unset means browse-only |
| `ARMOR_PRESIGN_ENABLED` | No | `false` | Enable pre-signed share URLs (`POST /admin/presign`, `GET /share/`) |
| `ARMOR_PRESIGN_SECRET` | With presign | — | Signing key (hex, at least 32 bytes) |
| `ARMOR_PRESIGN_BASE_URL` | With presign | — | Absolute base URL for generated share links (`http://` or `https://`) |

### Secondary backend (async replication, [ADR-006](docs/adr/006-dual-backend-replication.md))

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_SECONDARY_BACKEND` | No | — | Enables replication. Either a compact form (`filesystem:/backup/armor` or `b2:<region>:<endpoint>:<keyid>:<secret>:<bucket>`) or a bare kind (`filesystem` or `b2`) combined with the variables below |
| `ARMOR_SECONDARY_BACKEND_PATH` | With `filesystem` | — | Directory for a filesystem secondary |
| `ARMOR_SECONDARY_BACKEND_TYPE` | No | — | Legacy selector; only `filesystem` is accepted and it pairs with `ARMOR_SECONDARY_BACKEND_PATH` |
| `ARMOR_SECONDARY_B2_ENDPOINT`, `ARMOR_SECONDARY_B2_KEY_ID`, `ARMOR_SECONDARY_B2_KEY`, `ARMOR_SECONDARY_B2_BUCKET` | With a B2 secondary | — | B2 secondary credentials kept out of the selector string. `ARMOR_SECONDARY_BACKEND` takes precedence when both forms are set |

## Bucket aliases

`ARMOR_BUCKET_ALIASES` accepts a comma-separated list of legacy bucket names
that are served from `ARMOR_BUCKET`:

```bash
-e ARMOR_BUCKET=unified-bucket \
-e ARMOR_BUCKET_ALIASES=old-name,older-name \
```

A request whose bucket is an alias is served exactly as if it named
`ARMOR_BUCKET`: backend calls go to the configured bucket, and ACL entries
written against the configured bucket keep matching. The name the client sent
is echoed back wherever S3 semantics require it (`ListObjectsV2` /
`ListObjectVersions` `Name`; `CopyObject` sources resolve through the same
table). `ListBuckets` still reports only the configured bucket: an alias is not
a bucket. A bucket name that is neither the configured bucket nor an alias
fails in the backend as before.

Aliases exist so a tenant can be consolidated into the shared ADR-001 bucket
without every consumer changing its bucket name on cutover night. Set the old
name as an alias **before** the move, and drop it once every client has been
repointed; see the
[unified-bucket tenant onboarding runbook](docs/runbooks/unified-bucket-tenant-onboarding.md).

Aliasing never widens access: the alias is resolved to the configured bucket
before the ACL check, so a credential's bucket and prefix scoping is enforced
on the resolved path.

## Multi-key routing

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
default key. See the [key rotation runbook](docs/key-rotation-runbook.md).

## Authentication

ARMOR uses its own credential system for client authentication. **These ARMOR
credentials are separate from your B2 credentials**: ARMOR validates clients
locally, then uses its own B2 credentials to talk to the backend. B2
credentials never leave the ARMOR server, multiple clients can share one ARMOR
with different keys and permissions, and access keys can be scoped per bucket
or per prefix.

### Default credential

The simplest deployment uses a single static key pair:

```bash
ARMOR_AUTH_ACCESS_KEY=my-access-key
ARMOR_AUTH_SECRET_KEY=my-secret-key
```

### Named credentials with ACLs

For multi-user deployments, define any number of named credentials via
environment triplets: one `ACCESS_KEY`, one `SECRET_KEY`, and one optional `ACL`:

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

**ACL format**

- **Syntax:** `bucket:prefix[:actions]`
- **Multiple rules:** comma-separated (`bucket1:prefix1,bucket2:prefix2`)
- **Wildcard bucket:** `*` matches all buckets
- **Wildcard prefix:** `*` or an empty string matches all keys

**Action verbs** ([ADR-012](docs/adr/012-authorization-action-verbs-and-consumer-separation.md)).
If no actions are specified, all verbs are permitted.

| Verb | S3 operations covered |
|------|----------------------|
| `get` | GetObject, HeadObject |
| `put` | PutObject, CreateMultipartUpload, UploadPart, CompleteMultipartUpload, CopyObject (destination) |
| `delete` | DeleteObject, DeleteObjects, DeleteBucket, DeleteBucketLifecycleConfiguration |
| `list` | ListObjectsV2, ListMultipartUploads, ListObjectVersions, ListParts, ListBuckets |
| `abort` | AbortMultipartUpload |

An entry granting `delete` continues to grant `abort` (abort was part of delete
before it became its own verb); the reverse does not hold.

```bash
# All verbs on logs/ (no action segment = all permitted)
ARMOR_AUTH_LOGS_ACL="mybucket:logs/*"

# Only GET and LIST on readonly/
ARMOR_AUTH_READONLY_ACL="mybucket:readonly/*:get+list"

# Append-only backup writer: can write and list, never read, overwrite-protect or delete
ARMOR_AUTH_BACKUP_ACL="mybucket:backups/*:put+list"

# Multipart writer that can clean up its own aborted uploads but never delete objects
ARMOR_AUTH_RAW_ACL="mybucket:raw/*:put+list+abort"

# Full access to one bucket, read-only to another
ARMOR_AUTH_CROSSBUCKET_ACL="bucket-primary:*:get+put+delete+list,bucket-audit:logs/*:get+list"
```

**Overwrite-as-destruction risk:** without bucket versioning, a compromised
`put`-only credential can still overwrite existing objects. Append-only writers
mitigate but do not eliminate this; it is accepted residual risk.

**Empty ACL:** a credential with no `ACL` has full access to `ARMOR_BUCKET`.

### Credentials from a YAML file

For deployments managed by Kubernetes or an external secret system, load
credentials from a file:

```bash
ARMOR_AUTH_FILE=/etc/armor/credentials.yaml
```

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
    # No ACL means full access to the configured bucket
```

File credentials are merged with environment credentials; environment wins on
an access-key collision (logged at WARN); duplicate access keys within the file
are skipped (first wins); the file is watched and reloaded without a restart;
validation errors name the entry index and field, never the values.

## Secondary backend

ARMOR supports an optional secondary backend for disaster recovery, backup, or
multi-region replication ([ADR-006](docs/adr/006-dual-backend-replication.md)).
Writes are replicated asynchronously through a queue; reads always come from
the primary.

```bash
ARMOR_SECONDARY_BACKEND="filesystem:/backup/armor"
ARMOR_SECONDARY_BACKEND="b2:us-east-005:https://s3.us-east-005.backblazeb2.com:KEYID:SECRET:mybucket"
```

To keep credentials out of the selector string, use the individual variables:

```bash
ARMOR_SECONDARY_BACKEND=b2
ARMOR_SECONDARY_B2_ENDPOINT=https://s3.us-east-005.backblazeb2.com
ARMOR_SECONDARY_B2_KEY_ID=your-key-id
ARMOR_SECONDARY_B2_KEY=your-key-secret
ARMOR_SECONDARY_B2_BUCKET=your-bucket
```

The compact `ARMOR_SECONDARY_BACKEND` form takes precedence over the individual
variables when both are configured. The failover procedure is in
[Disaster Recovery](docs/disaster-recovery.md).

## S3 API coverage

### Transforming operations (encryption/decryption applied)

| Operation | Support |
|-----------|---------|
| PutObject | Full (streaming for large files; `If-None-Match: *` create-only honored) |
| GetObject | Full (range reads) |
| HeadObject | Full (plaintext size, conditionals) |
| CopyObject | Full (DEK re-wrapping, cross-bucket) |
| CreateMultipartUpload / UploadPart / CompleteMultipartUpload / AbortMultipartUpload | Full |
| ListParts / ListMultipartUploads | Full |

### Passthrough operations

| Operation | Support |
|-----------|---------|
| ListObjectsV2 | Full (size correction, `.armor/` filter) |
| DeleteObject / DeleteObjects | Full |
| ListBuckets | Full |
| CreateBucket / DeleteBucket / HeadBucket | Full |
| Lifecycle configuration | Full |
| Object Lock / Retention / Legal Hold | Full |

A detailed comparison against AWS S3 is in
[docs/s3-compliance-comparison.md](docs/s3-compliance-comparison.md).

**Reserved namespace: `.armor/`.** Client operations targeting keys under this
prefix return `403 AccessDenied`. It holds the provenance chain
(`.armor/chain/<writer>/*`, `.armor/chain-head/<writer>`), manifest deltas
(`.armor/manifest/<writer>/*`), multipart HMAC sidecars (`.armor/hmac/<sha256>`),
key-rotation state (`.armor/rotation-state.json`), multipart crash-recovery
state (`.armor/multipart/*.state`) and canary objects (`.armor/canary/*`).

## Multipart upload constraints

The constraints depend on the configured write format version
(`ARMOR_FORMAT_VERSION`, reported by `armor version --json` and `/version`).

### Format version 3 (default)

**No part-order or part-size contract.** Parts are encrypted in independent
counter namespaces, so any part sizes work (no block alignment), out-of-order
and concurrent part uploads are fully supported, part retries are idempotent,
and the only remaining rule is B2's own: non-final parts must be at least 5 MiB.

### Format version 2 (legacy)

[ADR-015](docs/adr/015-out-of-order-multipart-uniform-part-size.md)'s
uniform-part-size contract, as amended by
[ADR-011](docs/adr/011-barman-stays-on-armor-non-uniform-multipart.md):

- Part 1 pins the uniform part size `P` for the entire upload; part 1 itself may be any size
- Parts arriving before part 1 receive HTTP 503 SlowDown (retryable; standard clients retry transparently)
- Every part except the highest-numbered one must be exactly `P`; the final part may be any size
- Non-uniform part sizes (e.g. barman's `chunk_size + N×512` pattern) switch the upload to ADR-011 non-uniform mode instead of failing
- Genuine contract contradictions poison the upload with a 400: a loud failure, never silent corruption

Optional tuning for format version 2 (not required): AWS CLI
`multipart_chunksize = 67108864`, rclone `--s3-chunk-size 67108864`, boto3
`TransferConfig(multipart_chunksize=64*1024*1024)`.

### Migration (v1/v2 → v3)

Existing objects are migrated in place with `armor migrate --admin-url
http://127.0.0.1:9001 --target v3` (a client of `POST /admin/format/migrate`;
requires `ARMOR_ADMIN_TOKEN`). Start with `--dry-run`. Per-client behavior on
each format is documented, with the tests that back every row, in the
[multipart client-concurrency compatibility matrix](docs/multipart-client-compatibility.md).

## HTTP endpoints

### S3 listener (`ARMOR_LISTEN`, default `:9000`)

| Path | Auth | Description |
|---|---|---|
| `/healthz` | none | Liveness: the process is up |
| `/readyz` | none | Readiness: canary health (or always 200 when `ARMOR_CANARY_DISABLED=true`) |
| `/version` | none | `{"version":"0.1.1969","format_write_version":3,"go":"1.25.0"}`. Every response also carries `Server: ARMOR/<version>` |
| `/share/<token>` | token | Decrypted content for a pre-signed URL (only when `ARMOR_PRESIGN_ENABLED=true`) |
| everything else | SigV4 | The S3 API |

### Admin listener (`ARMOR_ADMIN_LISTEN`, default `127.0.0.1:9001`)

Routes marked *token* require `Authorization: Bearer <ARMOR_ADMIN_TOKEN>` and
return 403 when no token is configured. Every gated call is audit-logged.

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/healthz` | GET | none | Liveness |
| `/version` | GET | none | Same JSON as on the S3 listener |
| `/metrics` | GET | none | Prometheus metrics ([reference](docs/metrics.md)) |
| `/armor/canary` | GET | none | Canary integrity status (single-PUT and multipart canaries) |
| `/armor/audit` | GET | token | Walk the provenance chains and verify their integrity ([guide](docs/provenance-audit-walker.md)) |
| `/admin/key/verify` | GET | token | Verify the MEK can decrypt the canary object |
| `/admin/key/rotate` | POST | token | Rotate one MEK (`?key-id=<name>`, default key when omitted): re-wraps matching DEKs, no file re-upload; resumable ([runbook](docs/key-rotation-runbook.md)) |
| `/admin/key/ring` | GET | token | Key-ring census from the manifest; `?census=head` walks the bucket instead |
| `/admin/key/export` | GET | token | Export the current MEK (`?confirm=yes`) |
| `/admin/format/migrate` | POST / GET | token | Start (`?dry_run=true`, `?include=v1,v2`, `?target=3`, `?concurrency=N`) or poll an envelope-format migration |
| `/admin/manifest` | GET | token | Manifest state ([operator guide](docs/notes/manifest-repair-quarantine.md)) |
| `/admin/manifest/repair` | POST | token | Re-stamp a manifest's completion marker |
| `/admin/manifest/quarantine` | POST | token | Mark a manifest quarantined so stale entries are not served |
| `/admin/manifest/release` | POST | token | Lift a quarantine |
| `/admin/creds` | GET | token | List configured credentials (access keys and ACLs, never secrets) |
| `/admin/provenance/compact` | POST | token | Compact the provenance chain |
| `/admin/presign` | POST | token | Generate a pre-signed share URL (`ARMOR_PRESIGN_ENABLED=true`) |
| `/admin/b2/keys` | GET / POST | token | List or create scoped B2 application keys |
| `/admin/b2/keys/<id>` | DELETE | token | Delete a B2 application key |
| `/dashboard`, `/dashboard/...` | GET / POST | dashboard auth | Web dashboard and its JSON API (`/dashboard/api/list`, `/dashboard/metrics`, `/dashboard/encryption-stats`, `/dashboard/credential-activity`, upload/download/delete, presign, key rotation) |

## Web dashboard

A web dashboard for bucket browsing, encryption status, and metrics is
available on the admin port:

```bash
open http://localhost:9001/dashboard
```

Bucket browsing with prefix navigation, encryption status badges per object
(key name, ARMOR vs. unencrypted), metadata cache hit rates, and real-time
metrics (requests, bytes transferred, uptime, canary status). See
[docs/dashboard.md](docs/dashboard.md).

## Disaster recovery

The `decrypt` subcommand recovers encrypted objects without a running ARMOR
server — it needs the MEK and either B2 access or a local copy of the object,
and self-verifies every block it decrypts. Multipart objects store a
placeholder plaintext SHA-256, so recovered multipart output will not match
`sha256sum`, by design. The full runbook — decrypt commands, key requirements,
MEK escrow, restore drills, secondary failover — is in
[docs/disaster-recovery.md](docs/disaster-recovery.md).

## Releases and versioning

- Versions are `0.1.<counter>`; the counter only increases and carries no
  SemVer meaning. What changed is in [`CHANGELOG.md`](CHANGELOG.md).
- A release is one commit, `release: armor <version>`, produced by
  `scripts/cut-release.sh`, which bumps `VERSION` and writes the CHANGELOG
  entry. CI builds and publishes the images, verifies each tag exists in the
  registry, runs the compatibility suite, then creates the `v<version>` git
  tag and the Forgejo and GitHub releases. Nothing is tagged by hand.
- The badge at the top reflects the last release build on `main`.
- Full procedure, fleet rollout and the correctness-fix propagation checklist:
  [docs/release-process.md](docs/release-process.md).

## Repository structure

```
ARMOR/
├── cmd/
│   ├── armor/                 # The server and its subcommands (serve, demo, check, decrypt, verify, migrate, client-config, version)
│   ├── restore-verifier/      # Continuous restore verification harness (ADR-004)
│   ├── armor-fleet/           # Fleet console: version and health across deployments
│   └── verify-objects/        # Offline object verifier
├── internal/
│   ├── server/                # S3 handlers, admin API, auth middleware
│   ├── crypto/                # Envelope encryption, seekable CTR, HMAC tables
│   ├── backend/               # B2 S3 client, Cloudflare download routing, filesystem backend
│   ├── config/                # Environment configuration (see Configuration reference)
│   ├── acl/                   # Credential ACL evaluation (ADR-012)
│   ├── keymanager/            # Multi-key routing and key rings
│   ├── manifest/              # Manifest index (envelope metadata cache)
│   ├── canary/                # Self-healing integrity monitor
│   ├── provenance/            # Tamper-evident audit chain
│   ├── replication/           # Secondary backend queue (ADR-006)
│   ├── restoreverifier/       # restore-verifier library
│   ├── dashboard/             # Web dashboard
│   ├── presign/               # Pre-signed share URLs
│   ├── b2keys/                # B2 application key management
│   ├── metrics/, logging/     # Prometheus metrics, structured logging
│   ├── docsindex/             # Tests that keep docs/README.md and this file in sync with the tree
│   ├── version/               # Build-time version
│   └── testutil/
├── tests/
│   ├── integration/           # Real B2 + Cloudflare (build tag `integration`)
│   ├── aws-cli-compatibility/ # AWS CLI and rclone against a live endpoint
│   ├── docker-demo-smoke/     # The README demo, replayed
│   ├── fixtures/              # Migration fixtures and golden outcomes
│   └── test_drift_check.py    # Drift tooling tests (python3 -m pytest)
├── scripts/                   # definition-of-done.sh, release-gate.sh, cut-release.sh, drift check, ops tooling
├── docs/                      # ADRs, runbooks, notes, plan (index: docs/README.md)
├── config/drift-config.json   # Fleet drift-check configuration
├── Dockerfile                 # Published image (final stage = armor server)
├── AGENTS.md                  # Guide for contributors and agents
├── CHANGELOG.md, VERSION      # Release record and counter
└── Makefile
```

Deployment manifests live in `jedarden/declarative-config` under
`k8s/<cluster>/<namespace>/armor-deployment.y*ml`, applied by ArgoCD.

## Documentation

- **[Documentation index](docs/README.md)** — every document, organized by audience (Operate, Design, Test, Archive)
- [AGENTS.md](AGENTS.md) — how to build, test, track work and release in this repository
- [Release process](docs/release-process.md) — cutting a release, what CI publishes, fleet rollout, fix propagation
- [Disaster recovery](docs/disaster-recovery.md) — MEK backup/escrow, restore drills, offline decryption, secondary failover
- [Key rotation runbook](docs/key-rotation-runbook.md)
- [Cloudflare setup](docs/cloudflare-setup.md) — DNS configuration for zero-egress downloads
- [Web dashboard](docs/dashboard.md)
- [Integration tests](tests/integration/README.md) — testing against real B2 + Cloudflare
- [Implementation plan](docs/plan/plan.md) — architecture, phases, decisions

## License

MIT
