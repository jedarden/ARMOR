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
- [Authentication](#authentication)
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
stop working. The same suite also renders the tracked
[`compose.yaml`](compose.yaml) (both profiles, no daemon needed) and, where a
daemon is available, brings its demo profile up and runs the connectivity
check through it; it fails when `compose.yaml`'s pinned image drifts from
[`VERSION`](VERSION).

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

ARMOR is configured entirely by environment variables. A new B2-backed
deployment needs the variables below; everything else has a default or is
optional.

### Required configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_B2_REGION` | With `b2` | — | B2 region (e.g., `us-east-005`) |
| `ARMOR_B2_ACCESS_KEY_ID` | With `b2` | — | B2 application key ID |
| `ARMOR_B2_SECRET_ACCESS_KEY` | With `b2` | — | B2 application key |
| `ARMOR_BUCKET` | Yes | — | Bucket name (both backends) |
| `ARMOR_CF_DOMAIN` | No | — | Cloudflare domain CNAMEd to the bucket; when set, reads go through Cloudflare (free egress, edge cache) instead of the B2 endpoint |
| `ARMOR_MEK` | Yes | — | Master encryption key for the default key (hex, 32 bytes = 64 characters) |
| `ARMOR_AUTH_ACCESS_KEY` / `ARMOR_AUTH_SECRET_KEY` | One of these | — | The default client credential; full access to `ARMOR_BUCKET` |

The complete reference — listeners, backend, encryption and keys, client
authentication, caches, the manifest index, dashboard and pre-signed URLs,
bucket aliases, multi-key routing, and the secondary backend — lives in
[docs/configuration.md](docs/configuration.md).

## Authentication

ARMOR credentials are separate from your B2 credentials: ARMOR validates
clients locally, then uses its own B2 credentials to talk to the backend.

```bash
ARMOR_AUTH_ACCESS_KEY=my-access-key
ARMOR_AUTH_SECRET_KEY=my-secret-key
```

Named credentials add an optional ACL (`bucket:prefix[:actions]`, comma-separated):

```bash
ARMOR_AUTH_READONLY_ACCESS_KEY=reader-key
ARMOR_AUTH_READONLY_SECRET_KEY=reader-secret
ARMOR_AUTH_READONLY_ACL="mybucket:readonly/*"
```

Full ACL grammar, action verbs, append-only writers, and the YAML
credentials file: [docs/authentication.md](docs/authentication.md).

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
│   ├── config/                # Environment configuration (see docs/configuration.md)
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
├── compose.yaml, .env.example # Docker Compose demo + production profiles (see docs/connection-guide.md)
├── AGENTS.md                  # Guide for contributors and agents
├── CHANGELOG.md, VERSION      # Release record and counter
└── Makefile
```

Deployment manifests live in `jedarden/declarative-config` under
`k8s/<cluster>/<namespace>/armor-deployment.y*ml`, applied by ArgoCD.

## Documentation

- **[Documentation index](docs/README.md)** — every document, organized by audience (Operate, Design, Test, Archive)
- [Configuration reference](docs/configuration.md) — every ARMOR_* environment variable, bucket aliases, multi-key routing, secondary backend
- [AGENTS.md](AGENTS.md) — how to build, test, track work and release in this repository
- [Release process](docs/release-process.md) — cutting a release, what CI publishes, fleet rollout, fix propagation
- [Disaster recovery](docs/disaster-recovery.md) — MEK backup/escrow, restore drills, offline decryption, secondary failover
- [Key rotation runbook](docs/key-rotation-runbook.md)
- [Authentication and ACLs](docs/authentication.md) — credential model and ACL grammar
- [Cloudflare setup](docs/cloudflare-setup.md) — DNS configuration for zero-egress downloads
- [Web dashboard](docs/dashboard.md)
- [Integration tests](tests/integration/README.md) — testing against real B2 + Cloudflare
- [Implementation plan](docs/plan/plan.md) — architecture, phases, decisions

## License

MIT
