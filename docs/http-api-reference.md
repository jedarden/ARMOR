# ARMOR HTTP API Reference

This is the operator and client reference for ARMOR's HTTP surfaces. It
describes the routes registered by `internal/server/server.go`; the route
inventory test in `internal/server/http_api_reference_test.go` fails when a
registered path is not represented here.

## Listeners and common behavior

The S3 listener is `ARMOR_LISTEN` (default `:9000`). It is path-style S3:
`/bucket/key`, not virtual-hosted style. The admin listener is
`ARMOR_ADMIN_LISTEN` (default `127.0.0.1:9001`). Keep the admin listener on a
private network or loopback address; it contains key-management and recovery
operations.

Every response from the server carries `Server: ARMOR/<version>`. Responses
from the S3 handler also carry `x-amz-request-id` and `x-amz-id-2`. Use those
IDs when correlating a client failure with logs.

<!-- endpoint: / -->
<!-- endpoint: /healthz -->
<!-- endpoint: /readyz -->
<!-- endpoint: /version -->
<!-- endpoint: /share/ -->
<!-- endpoint: /admin/key/verify -->
<!-- endpoint: /admin/key/rotate -->
<!-- endpoint: /admin/key/ring -->
<!-- endpoint: /admin/key/export -->
<!-- endpoint: /admin/format/migrate -->
<!-- endpoint: /admin/manifest -->
<!-- endpoint: /admin/manifest/repair -->
<!-- endpoint: /admin/manifest/quarantine -->
<!-- endpoint: /admin/manifest/release -->
<!-- endpoint: /admin/creds -->
<!-- endpoint: /admin/provenance/compact -->
<!-- endpoint: /armor/canary -->
<!-- endpoint: /armor/audit -->
<!-- endpoint: /admin/presign -->
<!-- endpoint: /admin/b2/keys -->
<!-- endpoint: /admin/b2/keys/ -->
<!-- endpoint: /metrics -->
<!-- endpoint: /dashboard -->
<!-- endpoint: /dashboard/ -->
<!-- endpoint: /dashboard/agentation.js -->
<!-- endpoint: /dashboard/object -->
<!-- endpoint: /dashboard/metrics -->
<!-- endpoint: /dashboard/encryption-stats -->
<!-- endpoint: /dashboard/api/list -->
<!-- endpoint: /dashboard/credential-activity -->
<!-- endpoint: /dashboard/upload -->
<!-- endpoint: /dashboard/download -->
<!-- endpoint: /dashboard/delete -->
<!-- endpoint: /dashboard/presign -->
<!-- endpoint: /dashboard/admin/key/rotate -->
<!-- endpoint: /dashboard/admin/key/status -->

## Authentication

| Surface | Authentication | Failure behavior |
|---|---|---|
| S3 object and bucket API (`/`) | AWS SigV4 using an ARMOR access-key/secret-key pair. Header authentication uses `Authorization` and `X-Amz-Date`; query authentication is accepted for AWS-style S3 presigned requests. ACLs are evaluated after signature verification. | Authentication or ACL failures are HTTP 403 with an S3 XML error. The error code identifies cases such as `MissingAuthenticationToken`, `InvalidAccessKeyId`, `SignatureDoesNotMatch`, or `AccessDenied`. |
| S3 health, readiness, and version | None | These probes are public. |
| Share URL (`/share/<token>`) | The HMAC-signed, expiring token is the credential. No SigV4 or admin bearer token is expected. | Invalid, expired, or malformed tokens are rejected before object access. |
| Admin API | `Authorization: Bearer <ARMOR_ADMIN_TOKEN>` for every route except `/healthz`, `/version`, `/armor/canary`, and `/metrics`. | Missing/invalid bearer token is 401. If `ARMOR_ADMIN_TOKEN` is unset, gated routes are disabled and return 403. Gated calls are audit-logged without logging the token or MEK. |
| `/admin/presign` | Both admin bearer authentication and a valid S3 SigV4 credential with `get` permission for the requested object. | The admin gate runs first; the endpoint then returns the normal S3-style authorization error for an invalid signer or denied object. |
| Dashboard (`/dashboard...`) | HTTP Basic (`ARMOR_DASHBOARD_USER` + `ARMOR_DASHBOARD_PASS`) or a dashboard bearer token. If no dashboard credential is configured, the dashboard is still protected by the admin gate rather than becoming anonymous. | Dashboard authentication failures are 401 with `WWW-Authenticate: Basic realm="ARMOR Dashboard"`. |

The ACL action mapping is documented in [Authentication](authentication.md):
`get` covers reads, `put` covers writes including multipart, `delete` covers
committed-data deletion, `abort` covers only aborting an incomplete multipart
upload, and `list` covers listings. A credential with no ACL action segment
retains full access to its allowed bucket/prefix.

## Errors and status conventions

S3 handler errors are XML and have this shape (fields can be omitted when no
request context is available):

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Access Denied</Message>
  <RequestId>...</RequestId>
  <Resource>/bucket/key</Resource>
</Error>
```

Treat `Code` and HTTP status as the stable contract; messages may include a
backend diagnostic. The common S3 statuses are:

| Status | Meaning and examples |
|---|---|
| 200 | Successful read, write, list, copy, configuration operation, or bulk-delete response. |
| 204 | Successful `DeleteObject`, `DeleteBucket`, lifecycle deletion, or multipart abort. |
| 206 | Successful uncompressed byte-range `GetObject` or share download. |
| 304 | Conditional object request was not modified. |
| 400 | Malformed XML, invalid query/body, invalid range/part, or a multipart contract violation. |
| 403 | Authentication/ACL denial, access to the reserved `.armor/` namespace, or a quarantined object. |
| 404 | Missing object, bucket, or multipart upload. |
| 405 | Unsupported method on the S3 router or an endpoint that explicitly checks methods. |
| 412 | `If-Match`/`If-Unmodified-Since` precondition failed. |
| 416 | Invalid/out-of-bounds object range, or a range requested for a compressed object. |
| 500 | Backend, encryption, decryption, manifest, or serialization failure. |
| 503 | Readiness failure, or a temporary multipart ordering/back-end condition (`SlowDown`). |

Admin JSON endpoints generally return `Content-Type: application/json`; the
few B2-key and method-gate failures use `http.Error`, so callers should use
the status and treat the body as diagnostic rather than requiring one error
schema. Metrics is Prometheus text. Dashboard failures are plain text unless
the handler returns JSON for a successful response.

## Prefixes, bucket aliases, and reserved state

With `ARMOR_PREFIX=tenant/`, a client still addresses `bucket/key`. ARMOR
stores the object at `tenant/key`, prepends the prefix to list filters,
markers, multipart state, HMAC sidecars, manifests, provenance records, and
canary state, and strips it from keys and common prefixes in client responses.
The prefix is not a second client-visible path component. This behavior also
applies to `ListObjectVersions`, `ListMultipartUploads`, and the manifest and
migration admin operations.

The configured `ARMOR_BUCKET` is canonical. Names in
`ARMOR_BUCKET_ALIASES` resolve to that bucket for S3 requests and presign
requests, so aliases do not create another storage or ACL scope. The
`/.armor/` namespace is reserved for ARMOR's internal state: client S3 access
to keys beginning `.armor/` is denied with 403. Do not repair or delete that
state through S3; use the dedicated admin routes and the runbooks.

The S3 handler is the prefix-transparent data path. The direct backend paths
used by `/share/<token>` and the dashboard object operations do not prepend
`ARMOR_PREFIX` to the requested object key. Consequently, do not use presigned
share/download/upload/delete dashboard paths as a prefix-transparent API in a
shared-bucket deployment; use the SigV4 S3 routes, or explicitly validate the
physical key behavior before enabling those convenience routes.

## S3-compatible API

All requests in this section use the S3 listener and SigV4 unless explicitly
marked public. The bucket and object forms below are path-style. XML response
operations use the S3 namespace; encrypted object sizes and ranges are
plaintext sizes.

### Buckets, objects, and configuration

| Operation | Request | Success | Common errors and safe-use notes |
|---|---|---|---|
| `ListBuckets` | `GET /` | 200 XML | 403/500. Lists the backend's visible buckets; it is not a substitute for testing one configured bucket. |
| `ListObjectsV2` | `GET /<bucket>` (optionally `?list-type=2`, `prefix`, `delimiter`, `continuation-token`, `max-keys`) | 200 XML with client-visible keys, plaintext sizes, and continuation token | 403/500. Prefix and continuation markers are translated through `ARMOR_PREFIX`; never include `.armor/` in a client prefix. |
| `HeadBucket` | `HEAD /<bucket>` | 200, no body | 403/404. It checks bucket reachability, not object encryption health. |
| `CreateBucket` | `PUT /<bucket>` | 200 with `Location: /<bucket>` | 403/500. Bucket creation is passed to the backend; use the configured bucket in production. |
| `DeleteBucket` | `DELETE /<bucket>` | 204 | 403/500. This is destructive and backend-dependent; verify the target bucket before issuing it. |
| `GetObject` | `GET /<bucket>/<key>`; supports `Range`, conditional headers, and S3 metadata response headers | 200 plaintext; 206 for a valid uncompressed range; 304 for a satisfied not-modified condition | 403/404/412/416/500. Compressed objects do not support byte ranges; ARMOR verifies block HMACs while decrypting. |
| `HeadObject` | `HEAD /<bucket>/<key>`; supports conditional headers | 200 headers only; 304 for a satisfied not-modified condition | 403/404/412/500. `Content-Length`, ETag, and metadata describe the plaintext object where ARMOR has encryption metadata. |
| `PutObject` | `PUT /<bucket>/<key>` with plaintext body; optional `If-None-Match: *` | 200 | 400 for invalid compression/arguments, 403, 412 for create-only collision, 500, or 501 when the backend cannot provide atomic conditional writes. Overwrite is replacement, not version-safe archival. |
| `DeleteObject` | `DELETE /<bucket>/<key>` | 204 | 403/500. It removes the logical object and updates ARMOR bookkeeping; it is not an abort operation. |
| `CopyObject` | `PUT /<bucket>/<destination>` with `x-amz-copy-source: /<source-bucket>/<source-key>` | 200 XML | 400 invalid/missing source, 403 source or destination ACL denial, 404 source missing, 500. ARMOR re-wraps encrypted object key material for the destination key; authorize both source read and destination write. |
| `DeleteObjects` | `POST /<bucket>?delete` with S3 `Delete` XML | 200 XML; per-key `AccessDenied` entries can be returned inside a successful batch response | 400 malformed/empty XML, 403/500. ACLs are checked per key. Treat the response's `<Error>` entries as failures even when HTTP is 200. |
| `GetBucketLocation` | `GET /<bucket>?location` | 200 XML | 403/404/500. Returns the static location contract needed by S3 clients. |
| `GetBucketVersioning` | `GET /<bucket>?versioning` | 200 XML | 403/404/500. ARMOR returns its supported versioning configuration; do not infer that overwrites are recoverable. |
| `GetBucketLifecycleConfiguration` | `GET /<bucket>?lifecycle` | 200 XML | 403/500. Lifecycle is passed through and is not encrypted by ARMOR. |
| `PutBucketLifecycleConfiguration` | `PUT /<bucket>?lifecycle` with lifecycle XML | 200 | 403/500. Validate retention policy before applying it; it can delete ciphertext needed for recovery. |
| `DeleteBucketLifecycleConfiguration` | `DELETE /<bucket>?lifecycle` | 204 | 403/500. Destructive configuration change. |
| `GetObjectLockConfiguration` | `GET /<bucket>?object-lock` | 200 XML | 403/404/500. Backend configuration is passed through. |
| `PutObjectLockConfiguration` | `PUT /<bucket>?object-lock` with XML | 200 | 403/500. Use the backend's object-lock semantics; this does not encrypt or version data itself. |
| `GetObjectRetention` | `GET /<bucket>/<key>?retention` | 200 XML | 403/500. Retention is backend metadata, not an ARMOR envelope field. |
| `PutObjectRetention` | `PUT /<bucket>/<key>?retention` with XML | 200 | 403/500. Treat changes as a compliance operation. |
| `GetObjectLegalHold` | `GET /<bucket>/<key>?legal-hold` | 200 XML | 403/500. Backend metadata read. |
| `PutObjectLegalHold` | `PUT /<bucket>/<key>?legal-hold` with XML | 200 | 403/500. Backend metadata write. |

`OPTIONS` on the S3 catch-all is handled as a CORS preflight and returns 200
without SigV4 authentication. It advertises `GET, PUT, DELETE, HEAD, POST,
OPTIONS` and the request headers ARMOR accepts. Do not mistake a preflight
success for authorization to perform the following S3 operation.

### Multipart uploads

Multipart uses the standard S3 query dispatch. An upload has one logical key,
one upload ID, and ARMOR-managed encryption state; only the completed object
is visible to readers.

| Operation | Request | Success | Common errors and safe-use notes |
|---|---|---|---|
| `CreateMultipartUpload` | `POST /<bucket>/<key>?uploads` | 200 XML containing upload ID | 400 when compression is enabled, 403, 500. Multipart is intentionally rejected when `ARMOR_COMPRESS` or compression rules are active; use a single PUT for compressed objects. |
| `UploadPart` | `PUT /<bucket>/<key>?uploadId=<id>&partNumber=<n>`; ARMOR also accepts the equivalent POST form | 200 with part ETag | 400 `InvalidRequest`, `InvalidPart`, or `InvalidPartSize`; 404 `NoSuchUpload`; 503 `SlowDown` while required earlier state is unavailable; 500. Part numbers are 1–10,000. |
| `ListParts` | `GET /<bucket>/<key>?uploadId=<id>` | 200 XML | 403/404 `NoSuchUpload`/500. Returned sizes are plaintext part sizes. |
| `CompleteMultipartUpload` | `POST /<bucket>/<key>?uploadId=<id>` with `CompleteMultipartUpload` XML | 200 XML with location, key, bucket, and ETag | 400 `MalformedXML`, `InvalidRequest`, `InvalidPart`, or `InvalidPartSize`; 404 `NoSuchUpload`; 500. ARMOR validates the submitted part set and writes the manifest/sidecar before exposing the completed object. |
| `AbortMultipartUpload` | `DELETE /<bucket>/<key>?uploadId=<id>` | 204 | 403/404 `NoSuchUpload`/500. Abort abandoned uploads; an upload ID is not a delete credential for a completed object. |
| `ListMultipartUploads` | `GET /<bucket>?uploads` (optionally `prefix`, markers) | 200 XML | 403/500. Client-visible keys and markers have `ARMOR_PREFIX` stripped. |

Safe multipart use:

- For current v3 writes, parts may be uploaded out of order and part sizes are
  not pinned by the v3 layout. For v2-compatible writes, upload part 1 first,
  use one uniform block-aligned size for non-final parts, use at least 5 MiB
  for a multi-part object, and allow only the final part to be short.
- Retry the same part with the same part number and size. A size-changing
  retry or a violated uniform-size contract can poison a v2 upload; abort it
  and start a new upload. Do not complete a poisoned upload.
- Preserve the ETag returned for every part and submit the complete XML with
  the exact part numbers. Keep the upload ID private until completion or
  abort. See [Multipart Client Compatibility](multipart-client-compatibility.md)
  and [Multipart Layout](multipart-layout-and-read-path.md) for the format
  details.

## Health, canary, and metrics

<!-- endpoint: /readyz -->

| Endpoint | Method | Auth | Responses |
|---|---|---|---|
| `/healthz` on S3 or admin listener | `GET` (the liveness handler does not inspect the method) | Public | 200 with `OK`; it only proves the process is alive. |
| `/readyz` on S3 listener | `GET` (the readiness handler does not inspect the method) | Public | 200 JSON when the small-object canary is healthy, canary is explicitly disabled, or the manifest writer flushed recently; 503 JSON otherwise. The body includes `ready`, `canary_age_s`, `multipart_canary_healthy`, `manifest_flushed_s`, and `reason`. The multipart field is diagnostic and does not gate readiness. |
| `/version` on S3 or admin listener | `GET` | Public | 200 JSON containing `version`, `format_write_version`, and `go`; other methods return 405. |
| `/armor/canary` on admin listener | `GET` | Public | 200 JSON with the canary result. If no monitor is configured, 200 with `{"status":"unknown","error":"canary monitor not configured"}`. Other methods return 405. |
| `/metrics` on admin listener | `GET` for Prometheus scrapers | Public | 200 Prometheus text (`text/plain; version=0.0.4`). The exposition includes request, encryption, backend, replication, key-rotation, provenance, and canary families; see [Metrics](metrics.md). |

The canary performs real encrypted round trips. A healthy small-object canary
does not imply a healthy multipart canary; inspect both the canary JSON and
the multipart metric families. The canary writes internal objects beneath the
configured prefix and they must not be manipulated as user objects.

## Presigned downloads and sharing

<!-- endpoint: /share/ -->

### `POST /admin/presign`

The request body is JSON:

```json
{
  "bucket": "optional-bucket-or-alias",
  "key": "reports/report.parquet",
  "expires_in": "1h",
  "content_disposition": "attachment; filename=report.parquet",
  "range": "bytes=0-1023"
}
```

`key` is required. `expires_in` defaults to one hour and must use a duration
accepted by ARMOR. The endpoint returns 200 JSON with `url`, `expires_in`, and
UTC `expires_at`. The generated URL is a GET-only share link. It is allowed
only when `ARMOR_PRESIGN_ENABLED=true`; otherwise it returns 404. Invalid JSON,
missing key, or invalid expiration returns 400; signer/ACL failures return
403; URL-generation failures return 500.

The request's `key` is used as the token's backend key; `/admin/presign` does
not add `ARMOR_PREFIX`. For a prefixed shared bucket, use the normal S3 GET
operation instead of minting a share URL for a logical client key.

### `GET /share/<token>`

The share handler verifies the token, HEADs the object, and returns decrypted
plaintext for ARMOR objects or passthrough bytes for non-ARMOR objects. A full
download returns 200; an uncompressed valid range returns 206 with
`Content-Range`. The token can carry a fixed range and content-disposition;
otherwise the request's `Range` header is used.

| Condition | Status |
|---|---:|
| Missing/malformed token or invalid range | 400 |
| Invalid HMAC signature | 403 |
| Expired token | 410 |
| Object absent | 404 |
| Range over a compressed object | 416 |
| Backend/decryption failure | 500 |
| Non-GET | 405 |

Share URLs are bearer credentials. Use HTTPS, choose the shortest practical
expiry, do not put them in logs or tickets, and assume anyone who receives one
can download the object until it expires. A range link cannot be used for
compressed objects because compression destroys fixed plaintext offsets. In an
`ARMOR_PREFIX` deployment, the share handler reads the token's key directly
from the backend; it does not add the client namespace prefix, so use the S3
GET path for prefixed objects.

## Admin API

All routes in this section are on the admin listener and require the admin
bearer token unless the table says `public`. Responses are JSON unless noted.
Long-running or destructive routes should be run from a private operator
network and observed to completion.

| Endpoint | Method | Success and behavior | Errors / safe-use constraints |
|---|---|---|---|
| `/admin/key/verify` | `GET` | 200 `verified` when canary decrypt and HMAC checks pass; 200 `unknown` when no canary monitor exists | 503 `unverified`; 405 otherwise. This is a diagnostic, not a key-rotation action. |
| `/admin/key/rotate` | `POST` | 200 JSON rotation result. `key-id` selects a named key. An empty body uses fingerprint-based rotation against the configured active key; a body is legacy replacement-MEK mode. | 400 invalid key/MEK; 500 rotation failure. Prefer empty-body fingerprint mode, take a backup/escrow first, expect a long-running bucket walk, and verify the canary afterward. Never put a MEK in a command line, log, or ticket. |
| `/admin/key/ring` | `GET` with optional `census=head` | 200 JSON active/ring fingerprints and object histogram. Default census uses the manifest; `census=head` verifies live metadata. | 500 key/backend failure. `census=head` can take hours on a production bucket; run it detached and reconcile `head_failures` and unattributed fingerprints. |
| `/admin/key/export?confirm=yes` | `GET` | 200 JSON break-glass escrow containing the MEK and B2 connection credentials | 400 without exact `confirm=yes`; 405 otherwise. Treat the response as a root credential: use only for recovery, transmit over HTTPS, store in an approved secret escrow, and delete local copies after use. |
| `/admin/format/migrate` | `GET` | 200 JSON current migration state or `no_migration` | 500 when no encryption key is available. |
| `/admin/format/migrate` | `POST` with `dry_run`, `target`, `include`, `concurrency` query parameters | 200 JSON migration result | 400 invalid target/include/concurrency; 500 migration failure. Start with `dry_run=true`, ensure target matches the configured write version, and do not run overlapping migrations. |
| `/admin/manifest?key=<key>[&bucket=<bucket>]` | `GET` | 200 JSON manifest freshness/quarantine state | 400 missing key, 404 missing manifest, 409 incomplete freshness state, 500. `key` is the logical unprefixed object key. |
| `/admin/manifest/repair?key=<key>[&bucket=<bucket>]` | `POST` | 200 JSON repaired state; re-stamps completion to the ciphertext timestamp | 400/404/409/500. Use only after verifying the ciphertext is canonical; this changes the read gate. |
| `/admin/manifest/quarantine?key=<key>&reason=<reason>[&bucket=<bucket>]` | `POST` | 200 JSON quarantined state | 400/404/409/500. This deliberately makes reads return 403 rather than retryable 500; record the reason in the incident evidence. |
| `/admin/manifest/release?key=<key>[&bucket=<bucket>]` | `POST` | 200 JSON released state; idempotent | 400/404/409/500. Release only after the object has been repaired or otherwise proven readable. |
| `/admin/creds` | `GET` | 200 JSON credential names, ACLs, source, and load times; secrets are omitted | 405 otherwise. Still sensitive operational metadata; restrict access and do not publish the response. |
| `/admin/provenance/compact?writer=<id>` | `POST` | 200 JSON legacy-chain compaction result | 400 missing writer, 500 compaction failure. Maintenance operation; use the provenance runbook and preserve the audit evidence. |
| `/armor/audit` | `GET` | 200 JSON provenance-chain audit result | 500 audit failure, 405 otherwise. Read-only but can walk a large chain; `ARMOR_PREFIX` is applied to the chain namespace. |
| `/admin/b2/keys` | `GET` | 200 JSON B2 application-key list (`count` and `cursor` supported) | 503 when B2 key management is unavailable, 500 backend/API failure. |
| `/admin/b2/keys` | `POST` | 201 JSON newly-created B2 application key | 400 malformed body or missing `name`/`capabilities`; 503 unavailable; 500 B2 failure. The response contains a secret key: capture it only into an approved secret destination. |
| `/admin/b2/keys/<id>` | `DELETE` | 204 | 400 missing ID, 404 unknown key, 503 unavailable, 500 B2 failure. Revocation is destructive and can immediately break clients using that key. |

`/admin/key/export` and B2-key creation are the two admin responses that can
contain credential material. Do not paste either response into logs, shell
history, bead notes, or documentation.

## Dashboard routes

The dashboard is registered only when dashboard support is configured. All
dashboard routes use the dashboard authentication described above; they do not
use S3 SigV4 from the browser. The dashboard's configured credential performs
the underlying object operations and is subject to its own ACL.

| Endpoint | Method | Success | Common errors |
|---|---|---|---|
| `/dashboard` or `/dashboard/` | `GET` | 200 HTML bucket view; `prefix` and `continuation_token` select the view | 401 dashboard auth, 500 listing/template failure. |
| `/dashboard/agentation.js` | `GET` | 200 JavaScript module for the visual-feedback toolbar | 401 dashboard auth. |
| `/dashboard/object?key=<key>` | `GET` | 200 JSON object details, including ARMOR metadata when available | 400 missing key, 404 missing object, 500 backend failure. |
| `/dashboard/metrics` | `GET` | 200 JSON live request/cache/canary/replication metrics | 401 dashboard auth. |
| `/dashboard/encryption-stats?prefix=<prefix>` | `GET` | 200 JSON encrypted/plaintext counts and key usage | 401/500. |
| `/dashboard/api/list?prefix=<prefix>&continuation_token=<token>` | `GET` | 200 JSON listing with objects, common prefixes, and pagination | 400/401/500. |
| `/dashboard/credential-activity` | `GET` | 200 JSON per-credential allow/deny activity | 401 or 502 when the loopback admin credential query fails; 500 decode failure. |
| `/dashboard/upload` | `POST` multipart form (`file`, optional `key`, `content_type`) | 200 JSON upload result | 400 malformed/missing file, 403 dashboard credential not configured or backend ACL denial, 500. Maximum parsed form size is 100 MiB. |
| `/dashboard/download?key=<key>` | `GET` | 200 object bytes with attachment headers | 400 missing key, 403 disabled/denied, 404 missing object, 500. |
| `/dashboard/delete?key=<key>` | `DELETE` or `POST` | 200 JSON delete result | 400 missing key, 403 disabled/denied, 500. This is destructive. |
| `/dashboard/presign` | `POST` JSON (`key`, optional `expires_in`) | Proxies the 200 JSON result from `/admin/presign` | 400 invalid body/key, 401 dashboard auth, 404 disabled, 502 loopback/admin failure. |
| `/dashboard/admin/key/rotate` | `POST` | Proxies the admin rotation response after generating a new MEK | 401, 502 loopback failure, or the admin API's status. Treat as a destructive long-running operation. |
| `/dashboard/admin/key/status` | `GET` | 200 JSON rotation state, including `none` when idle | 401/405/500. |

The dashboard's `prefix` query is passed to the backend as a listing prefix.
Dashboard object operations are direct backend calls and do not apply the S3
handler's `ARMOR_PREFIX` translation. In a shared-bucket prefixed deployment,
use the S3 API for object reads and mutations and treat dashboard object
actions as unsafe unless their physical-key behavior has been explicitly
validated. Callers should never send the internal `.armor/` prefix.

## Related contracts

- [Authentication and ACLs](authentication.md)
- [Metrics](metrics.md)
- [Canary and observability contract](observability-contract.md)
- [Error responses](error-responses.md)
- [ARMOR HTTP status codes](armor-http-status-codes.md)
- [Multipart client compatibility](multipart-client-compatibility.md)
- [Manifest repair/quarantine runbook](notes/manifest-repair-quarantine.md)
