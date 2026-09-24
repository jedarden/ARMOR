# Configuration Reference

ARMOR is configured entirely by environment variables. Every variable read by
`internal/config` is listed here; a test (`internal/docsindex`) fails when one
is missing. Names in `<angle brackets>` are placeholders.

The required minimum for a new deployment is the README's
[Required configuration](../README.md#required-configuration) table; everything
else on this page is optional or backend-dependent.

## Listeners and operation

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_LISTEN` | No | `0.0.0.0:9000` | S3 API listen address |
| `ARMOR_ADMIN_LISTEN` | No | `127.0.0.1:9001` | Admin API listen address (key management, migration, metrics, dashboard) |
| `ARMOR_ADMIN_TOKEN` | No | — | Bearer token that gates every `/admin/*` route and `/armor/audit`. When unset those routes are disabled and return 403 (fail-closed). Surrounding whitespace is trimmed so a provisioned value with a trailing newline still works |
| `ARMOR_ADMIN_READ_TIMEOUT` | No | disabled | Max time the admin listener spends reading a request. Go duration (`30s`, `5m`), or `0`/unset for no deadline |
| `ARMOR_ADMIN_WRITE_TIMEOUT` | No | disabled | Max time the admin listener spends writing a response. Leave unset for `POST /admin/key/rotate` and `GET /admin/key/ring?census=head`, which walk the whole bucket and can take hours |
| `ARMOR_MAX_CONN_REQUESTS` | No | disabled | Max requests one S3 keep-alive connection may serve before its next response carries `Connection: close` and the connection is closed. Non-negative integer; `0`/unset disables. See [Keep-alive connection recycling](#keep-alive-connection-recycling) |
| `ARMOR_MAX_CONN_AGE` | No | disabled | Max age of one S3 keep-alive connection before its next response carries `Connection: close` and the connection is closed. Go duration (`90s`, `5m`); `0`/unset disables. See [Keep-alive connection recycling](#keep-alive-connection-recycling) |
| `ARMOR_LOG_LEVEL` | No | `info` | `debug`, `info`, `warn` or `error`. `debug` logs request and response headers and bodies |
| `ARMOR_WRITER_ID` | No | hostname | Provenance chain writer ID |
| `ARMOR_FORMAT_VERSION` | No | `3` | Envelope format written for new objects: `3` (current) or `2` (legacy). Reported by `armor version --json` and `/version` as `format_write_version` |
| `ARMOR_CANARY_DISABLED` | No | `false` | `true` skips the canary check in `/readyz` (readiness then reports 200 without verifying the MEK) |
| `ARMOR_ALLOW_NO_CREDENTIALS` | No | `false` | Start without any client credential. Set by the `demo` subcommand only; never in production |

### Keep-alive connection recycling

A load balancer that forwards per TCP connection (kube-proxy in particular)
pins every client connection to one replica for as long as it stays open.
Long-lived client connection pools therefore skew traffic across replicas, and
scaling out does not redistribute connections that already exist.

Setting either variable makes the S3 listener recycle connections: once a
connection has served more than `ARMOR_MAX_CONN_REQUESTS` requests or lived
longer than `ARMOR_MAX_CONN_AGE`, its next response carries
`Connection: close` and the server closes the connection after that response
completes. Clients close their end cleanly, re-dial on the next request, and
the new connection is balanced afresh.

The close always lands on a response boundary: a request that has begun --
including a streamed download or a single multipart-part upload -- always
completes normally. The cost is one reconnection per recycled connection (a
TCP handshake, plus TLS if the client's path terminates it), which is why both
thresholds default to disabled and deployments opt in. A typical setting for a
deployment whose traffic skews across replicas is `ARMOR_MAX_CONN_AGE=90s`.

## Backend

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
| `ARMOR_PREFIX` | No | — | Key prefix for shared-bucket deployments (e.g., `kalshi-tape/`). Stored in B2, invisible to S3 clients ([ADR-001](adr/001-bucket-prefix.md)) |
| `ARMOR_CF_DOMAIN` | No | — | Cloudflare domain CNAMEd to the bucket. When set, reads go through Cloudflare (free egress, edge cache); when unset, reads go to the B2 S3 endpoint directly and B2 egress applies. Ignored for the filesystem backend |

## Encryption and keys

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_MEK` | Yes | — | Master encryption key for the default key (hex, 32 bytes = 64 characters) |
| `ARMOR_MEK_<NAME>` | No | — | A named master key (same format). `<NAME>` is lower-cased; `ARMOR_MEK_DEFAULT` is reserved |
| `ARMOR_MEK_RING` | No | — | Comma-separated retired MEKs (hex) that remain valid for reading objects wrapped with the default key before a rotation |
| `ARMOR_MEK_<NAME>_RING` | No | — | The same for a named key; requires `ARMOR_MEK_<NAME>` |
| `ARMOR_KEY_ROUTES` | No | — | Prefix-to-key routes, e.g. `data/pii/*=sensitive,archive/*=archive,*=default` (see [Multi-key routing](#multi-key-routing)) |
| `ARMOR_BLOCK_SIZE` | No | `65536` | Encryption block size in bytes; a power of two, at least 4096 |
| `ARMOR_COMPRESS` | No | `false` | Legacy alias for `ARMOR_COMPRESS_RULES="*=zstd"`. Multipart uploads are rejected and byte-range reads are unsupported for compressed objects ([ADR-007](adr/007-zstd-compression.md)) |
| `ARMOR_COMPRESS_RULES` | No | — | Comma-separated rules `<suffix>|<content-type>=zstd|none`, first match wins, e.g. `.jsonl=zstd,application/json=zstd,*=none`. Per-request override via `x-amz-meta-armor-compress: true|false`. Single-PUT v3 objects only |

## Client authentication

At least one credential must be configured or the server refuses to start
(unless `ARMOR_ALLOW_NO_CREDENTIALS=true`). See [Authentication](authentication.md)
for the ACL syntax.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_AUTH_ACCESS_KEY` / `ARMOR_AUTH_SECRET_KEY` | One of these | — | The default (unnamed) credential; full access to `ARMOR_BUCKET` |
| `ARMOR_AUTH_<NAME>_ACCESS_KEY`, `ARMOR_AUTH_<NAME>_SECRET_KEY`, `ARMOR_AUTH_<NAME>_ACL` | One of these | — | A named credential with an optional ACL (`bucket:prefix[:verbs]`, comma-separated) |
| `ARMOR_AUTH_FILE` | One of these | — | YAML credentials file with the same schema; merged with environment credentials (environment wins on collision) and hot-reloaded when it changes |

## Read path and caches

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_READ_CONCURRENCY` | No | `16` | Maximum concurrent ranged reads per backend read |
| `ARMOR_CACHE_MAX_ENTRIES` | No | `10000` | Object metadata cache entries |
| `ARMOR_CACHE_TTL` | No | `300` | Metadata cache TTL in seconds |
| `ARMOR_LIST_CACHE_MAX_ENTRIES` | No | `1000` | List-result cache entries |
| `ARMOR_LIST_CACHE_TTL` | No | `60` | List-result cache TTL in seconds |

## Manifest index

The manifest index caches envelope metadata (IV, wrapped DEK) so reads do not
need a HEAD per object. It is stored under `.armor/manifest/` in the bucket.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_MANIFEST_ENABLED` | No | `true` | `false` or `0` disables the index |
| `ARMOR_MANIFEST_PREFIX` | No | `.armor/manifest` | Location of the index, relative to `ARMOR_PREFIX`. Must stay inside the tenant namespace (no absolute paths, no `../`) |
| `ARMOR_MANIFEST_COMPACTION_INTERVAL` | No | `3600` | Seconds between automatic compactions |
| `ARMOR_MANIFEST_COMPACTION_THRESHOLD` | No | `1000` | Delta entry count that triggers an early compaction |
| `ARMOR_MANIFEST_LOAD_TIMEOUT` | No | `480` | Seconds allowed for the startup manifest load; `0` is unbounded. Keep it under the pod's startupProbe budget: on timeout the server starts with an empty index instead of crash-looping |

## Dashboard and pre-signed URLs

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_DASHBOARD_USER` / `ARMOR_DASHBOARD_PASS` | No | — | HTTP Basic Auth for `/dashboard` |
| `ARMOR_DASHBOARD_TOKEN` | No | — | Bearer token for `/dashboard` |
| *(none of the three above)* | — | — | The dashboard then requires `ARMOR_ADMIN_TOKEN`, and returns 403 when that is unset too |
| `ARMOR_DASHBOARD_CREDENTIAL` | No | — | Access key of a configured named credential the dashboard signs uploads, downloads and deletes with. Unset means browse-only |
| `ARMOR_PRESIGN_ENABLED` | No | `false` | Enable pre-signed share URLs (`POST /admin/presign`, `GET /share/`) |
| `ARMOR_PRESIGN_SECRET` | With presign | — | Signing key (hex, at least 32 bytes) |
| `ARMOR_PRESIGN_BASE_URL` | With presign | — | Absolute base URL for generated share links (`http://` or `https://`) |

## Secondary backend (async replication, [ADR-006](adr/006-dual-backend-replication.md))

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
[unified-bucket tenant onboarding runbook](runbooks/unified-bucket-tenant-onboarding.md).

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
default key. See the [key rotation runbook](key-rotation-runbook.md).

## Secondary backend

ARMOR supports an optional secondary backend for disaster recovery, backup, or
multi-region replication ([ADR-006](adr/006-dual-backend-replication.md)).
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
[Disaster Recovery](disaster-recovery.md).
