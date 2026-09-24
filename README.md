# ARMOR

[![release](https://img.shields.io/github/v/release/jedarden/ARMOR)](https://github.com/jedarden/ARMOR/releases/latest)

**Authenticated Range-readable Managed Object Repository**

ARMOR is an S3-compatible proxy server that encrypts data before storing it in
[Backblaze B2](https://www.backblaze.com/cloud-storage) and serves downloads
through Cloudflare for zero-egress cost. Any S3-compatible client (boto3, AWS
CLI, DuckDB, rclone, litestream, barman) works without modification: reads,
range reads and single-PUT writes are plain S3, and multipart writers run with
their default concurrency on the default write format (v3), whose only
multipart rule is B2's own ≥ 5 MiB non-final-part minimum. The legacy v2
format keeps a uniform-part-size contract that stock client retry behavior
already covers. Per-client behavior, tested configuration examples for AWS
CLI, litestream and barman, and the behaviors each format rejects:
[docs/multipart-client-compatibility.md](docs/multipart-client-compatibility.md).

- **Zero-knowledge encryption** — data is encrypted before it leaves ARMOR; B2 only ever stores ciphertext (guaranteed for envelope v2/v3 objects; legacy v1 objects must be migrated first — see [Security model](#security-model))
- **Zero egress fees** — downloads route through Cloudflare via the Bandwidth Alliance
- **Seekable encryption** — AES-256-CTR with 64 KB blocks enables byte-range reads without decrypting the whole file
- **DuckDB-compatible** — query encrypted Parquet files with column pruning and predicate pushdown intact
- **Multi-key routing** — different master keys for different path prefixes; automatic key selection per object

## Project status and contributing

ARMOR is private infrastructure in production use. The source of truth is the
private Forgejo instance at `git.ardenone.com/jedarden/ARMOR`; the GitHub
repository `jedarden/ARMOR` is a read-only mirror updated on every commit.
File issues on GitHub (the public surface); code changes land through Forgejo.
Build, test, work-tracking and release conventions: [AGENTS.md](AGENTS.md).

## Install

**Container image:**

```bash
docker pull ghcr.io/jedarden/armor:<version>
```

`<version>` is a release counter such as `0.1.1970`; the current one is in the
[`VERSION`](VERSION) file. Only pinned version tags are published — there is no
`latest` or other floating tag.

**From source** (Go 1.25+ per `go.mod`; contributors read [AGENTS.md](AGENTS.md)):

```bash
git clone https://github.com/jedarden/ARMOR.git && cd ARMOR
make build                             # every cmd/ binary into bin/, version injected from VERSION
scripts/definition-of-done.sh --fast   # build + vet + script tests, the local gate
```

**Platform note:** published images are built for linux/amd64 only. On Apple
Silicon, add `--platform linux/amd64` to `docker pull` and `docker run`.

## Quick Start

Every release publishes `ghcr.io/jedarden/armor:<version>` — public, no
credentials needed, and the only registry this document uses. (The same tags
also go to a private Docker Hub namespace for fleet deployments, where
anonymous pulls return 401; companion images are internal-only.)

### The 60-second demo (Docker Compose)

The tracked [`compose.yaml`](compose.yaml) runs the demo — ARMOR against a
temporary filesystem backend with fixed, non-secret credentials; no B2,
Cloudflare or AWS account needed:

```bash
docker compose --profile demo up -d
docker compose --profile demo logs -f armor-demo   # watch it start
docker compose --profile demo down                 # stop and remove
```

Compose publishes the demo's S3 port on the host (`9000`); the connectivity
check below also works from the host with a local AWS CLI. For the container
form, swap `container:armor-demo` for the compose container's ID:
`container:$(docker compose --profile demo ps -q armor-demo)`.

### Local demo (plain Docker)

Start ARMOR in the background:

```bash
docker run -d --name armor-demo -p 9000:9000 -p 9001:9001 \
  ghcr.io/jedarden/armor:<version> demo --listen 0.0.0.0:9000 --admin-listen 0.0.0.0:9001
```

Then verify connectivity with the official AWS CLI container (succeeds even
when the bucket is empty) — and create and list the demo bucket the same way
(`s3 mb s3://demo-bucket`, then `s3 ls s3://demo-bucket`):

```bash
docker run --rm --network container:armor-demo -e AWS_ACCESS_KEY_ID=armor \
  -e AWS_SECRET_ACCESS_KEY=armor-demo-secret -e AWS_DEFAULT_REGION=us-east-1 \
  amazon/aws-cli:2.29.0 --endpoint-url http://127.0.0.1:9000 s3 ls
```

With the AWS CLI on the host, the check is equivalently
`AWS_ACCESS_KEY_ID=armor AWS_SECRET_ACCESS_KEY=armor-demo-secret aws
--endpoint-url http://localhost:9000 s3 ls`; tear down with
`docker rm -f armor-demo`.

Both demo paths are guarded by an automated smoke test (`make
test-docker-demo`), which replays these commands against the image pinned by
[`VERSION`](VERSION) and fails when `compose.yaml`'s pinned tag drifts from it.

## Subcommands

`armor help` prints the full list; `serve` is the default. The others: `demo`
(temporary filesystem-backed instance), `check` (verify a deployment: config,
backend connectivity, the Cloudflare path, MEK correctness via the canary),
`client-config` (print known-good configuration for common S3 clients, see
below), `decrypt` (offline recovery with only the MEK), `verify` (check
objects for corruption), `migrate` (legacy envelopes to the current format)
and `version`. The per-command reference — flags, inputs and outputs, exit
codes, credential requirements, and safe-use notes — is
[docs/cli-reference.md](docs/cli-reference.md).

## Production Docker deployment

For a B2-backed deployment, replace every placeholder with a value from your
environment. Keep the same MEK when restarting an instance; losing it makes
existing objects unreadable. Pin the image to a published version.

```bash
docker run -d --name armor \
  -p 9000:9000 -p 127.0.0.1:9001:9001 \
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
disabled (fail-closed) — fine for a plain proxy, but there is no way to rotate
keys. Verify a running instance with `armor check` (inside the container) or
`curl http://127.0.0.1:9001/version`. The `production` profile of
[`compose.yaml`](compose.yaml) runs the same configuration from a root `.env`
(copy [`.env.example`](.env.example)) — see [docs/connection-guide.md](docs/connection-guide.md).

## Client configuration

`client-config` prints known-good, copy-pasteable configuration for common
S3-compatible tools: endpoint URL, path-style addressing (required), a region
placeholder (required by clients, unused by ARMOR), credential
environment-variable names (never values), and the multipart contract in force.

```bash
armor client-config --for aws-cli --endpoint http://localhost:9000 --bucket my-bucket
armor client-config --for rclone --endpoint http://localhost:9000
# --for also accepts: boto3, duckdb, litestream, barman
```

Per-client walkthroughs: [docs/connection-guide.md](docs/connection-guide.md).
DuckDB encrypted-Parquet queries: [docs/research/duckdb-encrypted-parquet.md](docs/research/duckdb-encrypted-parquet.md).

## Architecture

Uploads go client → ARMOR → B2: ARMOR encrypts, and B2 ingress is free.
Downloads come back B2 → Cloudflare → ARMOR → client over the Cloudflare PNI
link, so egress is free and the CDN only ever caches ciphertext. Storage is
~$6–7/TB/month on B2; egress, B2 API calls and the Cloudflare free plan are
$0, and optional zstd compression ([ADR-007](docs/adr/007-zstd-compression.md))
shrinks compressible data a further 2–5×.

Reads are seekable — AES-256-CTR in 64 KB blocks with per-block HMACs — so a
range request decrypts only the blocks it touches; DuckDB's row-group and
column-chunk reads keep column pruning and predicate pushdown effective.
ARMOR is stateless: any instance with the same MEK, B2 credentials and
Cloudflare domain can serve the same bucket, and all authoritative state
(envelope metadata, key-rotation progress, provenance chain, manifest index)
lives in B2 under the reserved `.armor/` prefix. The full design record —
plan, phases, ADRs — starts at [docs/plan/plan.md](docs/plan/plan.md).

## Encryption design

A master key (MEK), stored locally and never uploaded, wraps a random per-file
data encryption key (DEK); the wrapped DEK is stored alongside the object's
ciphertext in B2. File data is AES-256-CTR in 64 KB blocks, each block
authenticated by an HMAC-SHA256 recorded in the envelope — that is what makes
reads seekable and tamper-evident.

Key rotation re-wraps DEKs without re-uploading file data — metadata only.

The zero-knowledge claim depends on the object's envelope version:

- **v1 (legacy; no current release writes it)** — a CTR counter defect
  ([ADR-005](docs/adr/005-ctr-counter-stride-fix.md)) reused keystream
  between adjacent 64 KB blocks, so ciphertext alone reveals the XOR of the
  plaintexts of any two adjacent blocks. v1 objects remain readable, but the
  zero-knowledge claim does **not** apply to them until they are migrated.
- **v2 (the 2024-08 fix)** — the counter advances by the full AES-block count
  of each 64 KB block, so keystreams never overlap and the zero-knowledge
  claim holds. Still readable, and selectable for new writes with
  `ARMOR_FORMAT_VERSION=2`.
- **v3 (the default write format)** —
  [spec](docs/format/envelope-v3.md); keeps v2's non-overlapping counters,
  gives each multipart part its own counter namespace, and adds
  self-describing parts and optional per-block zstd compression.

Migrating legacy objects re-encrypts them to v3 under fresh per-object keys:
`armor migrate --admin-url http://127.0.0.1:9001 --target v3` (requires
`ARMOR_ADMIN_TOKEN`; start with `--dry-run`). The operator procedure —
safety gates, monitoring, failure recovery, and the per-bucket evidence
record — is the [Format Migration
Runbook](docs/runbooks/format-migration.md); per-format outcomes and known
issues: [V3 Migration Reference](docs/research/migration/V3_Migration_Reference.md).
Verify
migrated objects with `armor verify` (offline audit; exits non-zero when any
object is corrupted) or the continuous restore verifier
([deployment guide](docs/restore-verifier-deployment-guide.md),
[ADR-004](docs/adr/004-continuous-restore-verification.md)).

## Security model

| Threat | Mitigation |
|--------|-----------|
| B2 data breach | v2/v3 objects are AES-256-CTR encrypted with per-file DEKs — useless without the MEK. Legacy v1 objects are the exception: their keystream reuse ([ADR-005](docs/adr/005-ctr-counter-stride-fix.md)) lets ciphertext alone reveal plaintext XOR between adjacent blocks; migrate them (see [Encryption design](#encryption-design)) |
| CDN or on-path inspection | Cached and transmitted content is ciphertext — the CDN sees only opaque blobs; TLS on the ARMOR listener |
| ARMOR server compromise | MEK exposed: rotate immediately; per-file DEKs limit blast radius |
| Ciphertext tampering (bit-flip, reorder, truncate) | Per-block HMAC-SHA256 detects modification; the block index is implicit in the offset and the HMAC table length validates the block count |
| Unauthorized access | ARMOR-side SigV4 authentication plus prefix/verb ACLs (not B2 access control) |
| V1 keystream reuse | Version 1 envelopes had a CTR counter bug (keystream reuse between adjacent blocks), so the zero-knowledge claim holds only after migration. Migrate with `armor migrate --target v3` ([runbook](docs/runbooks/format-migration.md)), then verify with `armor verify`. See [ADR-005](docs/adr/005-ctr-counter-stride-fix.md) |

## Configuration reference

ARMOR is configured entirely by environment variables. A new B2-backed
deployment needs the variables below; everything else has a default or is
optional. The complete reference — listeners, backend, encryption and keys,
client authentication, caches, the manifest index, dashboard and pre-signed
URLs, bucket aliases, multi-key routing, and the secondary backend — lives in
[docs/configuration.md](docs/configuration.md).

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_B2_REGION` | With `b2` | — | B2 region (e.g., `us-east-005`) |
| `ARMOR_B2_ACCESS_KEY_ID` | With `b2` | — | B2 application key ID |
| `ARMOR_B2_SECRET_ACCESS_KEY` | With `b2` | — | B2 application key |
| `ARMOR_BUCKET` | Yes | — | Bucket name (both backends) |
| `ARMOR_CF_DOMAIN` | No | — | Cloudflare domain CNAMEd to the bucket; when set, reads go through Cloudflare (free egress, edge cache) instead of the B2 endpoint |
| `ARMOR_MEK` | Yes | — | Master encryption key for the default key (hex, 32 bytes = 64 characters) |
| `ARMOR_AUTH_ACCESS_KEY` / `ARMOR_AUTH_SECRET_KEY` | One of these | — | The default client credential; full access to `ARMOR_BUCKET` |

## Authentication

ARMOR credentials are separate from your B2 credentials: ARMOR validates
clients locally and uses its own B2 credentials for the backend. The default
client credential is `ARMOR_AUTH_ACCESS_KEY` / `ARMOR_AUTH_SECRET_KEY` (full
access to `ARMOR_BUCKET`). Named credentials (`ARMOR_AUTH_<NAME>_ACCESS_KEY` /
`ARMOR_AUTH_<NAME>_SECRET_KEY`) add an optional ACL, `bucket:prefix[:actions]`,
comma-separated for multiple entries. Full ACL grammar, action verbs,
append-only writers and the YAML credentials file:
[docs/authentication.md](docs/authentication.md).

## S3 API coverage

Transforming operations (encryption/decryption applied): PutObject (streaming;
`If-None-Match: *` create-only honored), GetObject (range reads), HeadObject,
CopyObject (DEK re-wrapping, cross-bucket) and the full multipart set.
Passthrough: ListObjectsV2 (size correction, `.armor/` filter), DeleteObject /
DeleteObjects, ListBuckets, CreateBucket / DeleteBucket / HeadBucket, lifecycle
configuration, Object Lock / Retention / Legal Hold. Per-operation comparison
against AWS S3: [docs/s3-compliance-comparison.md](docs/s3-compliance-comparison.md).

**Reserved namespace: `.armor/`.** Client operations targeting keys under this
prefix return `403 AccessDenied`. It holds the provenance chain and chain
heads, manifest deltas, multipart HMAC sidecars, key-rotation state, multipart
crash-recovery state and canary objects (`.armor/chain/`, `.armor/manifest/`,
`.armor/hmac/`, `.armor/rotation-state.json`, `.armor/multipart/`, `.armor/canary/`).

## Multipart upload constraints

With format version 3 (the default write format) there is no part-order or
part-size contract: parts are encrypted in independent counter namespaces, so
any part sizes work (no block alignment), out-of-order and concurrent part
uploads are fully supported, retries are idempotent, and the only remaining
rule is B2's own — non-final parts must be at least 5 MiB.

Format version 2 (legacy, `ARMOR_FORMAT_VERSION=2`) keeps
[ADR-015](docs/adr/015-out-of-order-multipart-uniform-part-size.md)'s
uniform-part-size contract, as amended by
[ADR-011](docs/adr/011-barman-stays-on-armor-non-uniform-multipart.md):
non-uniform part sizes switch the upload to non-uniform mode instead of
failing, and a genuine contract contradiction poisons the upload with a 400 —
loud, never silent. Per-format client behavior:
[docs/multipart-client-compatibility.md](docs/multipart-client-compatibility.md).
Existing v1/v2 objects migrate in place with `armor migrate --admin-url
http://127.0.0.1:9001 --target v3` (requires `ARMOR_ADMIN_TOKEN`; start with
`--dry-run`).

## HTTP endpoints

The S3 listener (`ARMOR_LISTEN`, default `:9000`) serves `/healthz` (liveness),
`/readyz` (readiness — canary health, or always 200 when
`ARMOR_CANARY_DISABLED=true`), `/version` (version JSON; every response carries
`Server: ARMOR/<version>`) and `/share/<token>` (decrypted content for a
pre-signed URL, `ARMOR_PRESIGN_ENABLED=true`); everything else is the
SigV4-authenticated S3 API.

### Admin API

`ARMOR_ADMIN_LISTEN` (default `127.0.0.1:9001`). Routes marked *token* require
`Authorization: Bearer <ARMOR_ADMIN_TOKEN>` and return 403 when none is
configured; every gated call is audit-logged.

- Open: `/healthz`, `/version`, `/metrics` (Prometheus,
  [reference](docs/metrics.md)), `/armor/canary` (integrity status,
  single-PUT and multipart)
- *Token*: `/armor/audit` — provenance-chain walk
  ([guide](docs/provenance-audit-walker.md)); `/admin/key/verify|rotate|ring|export`
  — MEK verification, rotation ([runbook](docs/key-rotation-runbook.md)),
  census, export (`?confirm=yes`); `/admin/format/migrate` — start or poll an
  envelope-format migration; `/admin/manifest[/repair|/quarantine|/release]` —
  manifest state and repair
  ([operator guide](docs/notes/manifest-repair-quarantine.md)); `/admin/creds`
  — configured credentials (access keys and ACLs, never secrets);
  `/admin/provenance/compact`; `/admin/presign` — pre-signed share URL;
  `/admin/b2/keys[/<id>]` — list, create, delete scoped B2 application keys
- Dashboard auth: `/dashboard`, `/dashboard/...` — web dashboard and JSON API:
  bucket browsing, encryption status, live metrics
  ([docs/dashboard.md](docs/dashboard.md))

## Disaster recovery

The `decrypt` subcommand recovers encrypted objects without a running ARMOR
server — it needs the MEK and either B2 access or a local copy of the object,
and self-verifies every block it decrypts (recovered multipart output carries a
placeholder plaintext SHA-256, so it will not match `sha256sum`, by design).
Full runbook: [docs/disaster-recovery.md](docs/disaster-recovery.md).

The `verify` subcommand audits objects offline the same way: it unwraps
fingerprinted wrapped DEKs with the active MEK or a `-escrow` ring (an object
naming a fingerprint neither carries is an ERROR, not corruption), verifies
multipart objects through their manifest and HMAC sidecar, writes one
`-output` JSON row per object with the failure reason, and exits non-zero
when any object is CORRUPTED or ends in ERROR — safe to gate scripts on.

## Releases and versioning

- Versions are `0.1.<counter>`; the counter only increases and carries no
  SemVer meaning. What changed is in [`CHANGELOG.md`](CHANGELOG.md). A release
  is one commit, `release: armor <version>`, produced by `scripts/cut-release.sh`;
  CI builds and publishes the images, verifies each tag exists in the registry,
  runs the compatibility suite, then creates the `v<version>` git tag and the
  Forgejo and GitHub releases. Nothing is tagged by hand.
- Full procedure, fleet rollout and the fix-propagation checklist:
  [docs/release-process.md](docs/release-process.md).

## Repository structure

| Path | What it is |
|------|------------|
| `cmd/` | The three binaries: `armor` (the server and its `serve`, `demo`, `check`, `decrypt`, `verify`, `migrate`, `client-config`, `version`, `help` subcommands), `restore-verifier`, `armor-fleet` |
| `internal/` | All packages: `server` (S3 + admin handlers), `crypto`, `backend`, `config`, `keymanager`, `manifest`, `acl`, `canary`, `dashboard`, `presign`, `provenance`, `replication`, `restoreverifier`, `metrics`, `logging`, `b2keys`, `docsindex`, `version`, `testutil` |
| `tests/` | Go suites outside the package tree — `integration/` (real B2, build-tagged), `aws-cli-compatibility/`, `docker-demo-smoke/`, `rbac/`, `performance/` — plus the pytest suites (`tests/test_*.py`) and migration `fixtures/` |
| `scripts/` | Operator tooling: `definition-of-done.sh`, `release-gate.sh`, `cut-release.sh`, drift check, starvation watch ([scripts/README.md](scripts/README.md)) |
| `docs/` | ADRs, runbooks, notes, plan — every file indexed by [docs/README.md](docs/README.md) |
| `config/drift-config.json` | Fleet drift-check configuration |
| `Dockerfile`, `Dockerfile.test`, `compose.yaml`, `.env.example` | The published image (final stage stays the armor server), the test image, and the Compose demo + production profiles |
| `AGENTS.md` | Guide for contributors and agents: layout, gates, beads, commits, releases |
| `VERSION`, `CHANGELOG.md` | The release counter and its notes; only `scripts/cut-release.sh` changes them |
| `Makefile` | Build, test and release targets (`make help`) |

Deployment manifests do **not** live here. They live in
`jedarden/declarative-config` under
`k8s/<cluster>/<namespace>/armor-deployment.y*ml` and `restore-verifier*.y*ml`,
applied by ArgoCD.

## Documentation

- **[Documentation index](docs/README.md)** — every document, organized by audience (Operate, Design, Test, Archive)
- [Configuration reference](docs/configuration.md) — every ARMOR_* environment variable, bucket aliases, multi-key routing, secondary backend
- [AGENTS.md](AGENTS.md) — how to build, test, track work and release in this repository
- [Release process](docs/release-process.md) — cutting a release, what CI publishes, fleet rollout, fix propagation
- [Disaster recovery](docs/disaster-recovery.md) — MEK backup/escrow, restore drills, offline decryption, secondary failover
- [Integration tests](tests/integration/README.md) — testing against real B2 + Cloudflare

## License

MIT
