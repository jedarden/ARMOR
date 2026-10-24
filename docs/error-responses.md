# ARMOR Error Responses

**Version:** 2.1
**Date:** 2026-10-16
**Status:** Active

## Overview

This document is the authoritative reference for all error responses returned by ARMOR, including the verification-error structure used by the restore-verification pipeline. It consolidates and replaces the following previous documentation:

- `admin-endpoint-error-headers.md`
- `admin-endpoint-error-response-headers.md`
- `error-response-header-consistency.md`
- `error-response-headers-specification.md`
- `error-header-spec.md`
- `s3-endpoint-response-headers.md`
- `auth-rejection-headers.md`
- `error-format.md` (verification-error JSON structure)
- `error-response-inventory.md` (error-code-to-test-coverage catalog)
- `armor-http-status-codes.md` (HTTP status code usage)

## Scope

ARMOR has three distinct error surfaces with different formats:

1. **S3-Facing Endpoints** - Public S3-compatible API returning XML errors
2. **Admin Endpoints** - Management and monitoring API with mixed response formats
3. **Restore Verification** - A structured Go error type (not an HTTP response) describing byte-level decompression mismatches

## Table of Contents

- [S3-Facing Endpoints](#s3-facing-endpoints)
  - [Error Response Structure](#error-response-structure)
  - [Client Messages and Log Detail](#client-messages-and-log-detail)
  - [Error Codes by HTTP Status](#error-codes-by-http-status)
  - [Client Disconnects and Request Deadlines](#client-disconnects-and-request-deadlines)
  - [Authentication Errors](#authentication-errors-http-403)
  - [Per-Operation Errors](#per-operation-errors)
- [Admin Endpoints](#admin-endpoints)
  - [Endpoint Summary](#endpoint-summary)
  - [Health Endpoints](#health-endpoints)
  - [Key Management](#key-management)
  - [Pre-signed URLs](#pre-signed-urls)
  - [B2 Key Management](#b2-key-management)
  - [Status Endpoints](#status-endpoints)
- [Verification Error Format](#verification-error-format)
- [Inconsistencies and Remediation](#inconsistencies-and-remediation)
- [Implementation Reference](#implementation-reference)
- [Test Coverage](#test-coverage)

---

## S3-Facing Endpoints

S3-facing endpoints return AWS S3-compatible XML error responses.

### Error Response Structure

All S3 error responses follow this XML format:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>ErrorCode</Code>
  <Message>Error message</Message>
  <RequestId>request-id</RequestId>
  <Resource>/bucket/key</Resource>
</Error>
```

`RequestId` and `Resource` are included when a request ID is available from
middleware context; `Resource` is always the request path.

**Standard Headers:**

| Header | Value | Presence |
|--------|-------|----------|
| `Content-Type` | `application/xml` | Always |
| `Access-Control-Allow-Origin` | `*` | HTTP 403 only |
| `Access-Control-Allow-Methods` | `GET, PUT, DELETE, HEAD, POST, OPTIONS` | HTTP 403 only |
| `Access-Control-Allow-Headers` | `Authorization, Content-Type, Range, Content-Length` | HTTP 403 only |

**Key Difference from AWS S3:** ARMOR includes CORS headers on error responses, while AWS S3 only includes them when explicitly configured via bucket CORS rules.

### Client Messages and Log Detail

The `<Message>` a client receives is never the string a handler passes to
`writeError`. Both writers (`internal/server/handlers`'s `writeErrorCause` and
`internal/server`'s `writeError`) resolve every error through the
client-facing catalogue in `internal/server/s3errors`: the error code maps to
a fixed client message, and the handler-supplied string becomes **log detail
only**, recorded in the structured `s3_error` event alongside the request ID.

The split is the contract:

- **Client messages** state what the client did and what to change, and cite
  the request ID (`Request ID: <id>.`, or where to find it when none was
  assigned). They never contain Go identifiers, backend error text, or
  operator-only remediation such as `/admin/` URLs.
- **Log detail** carries the internal diagnostic (backend error, function
  context, repair path) to the structured log, keyed by the same request ID
  the client was told to quote.
- **`SlowDown` is the deliberate exception:** its ADR-008 part-size guidance
  is already a client contract, so non-empty detail passes through verbatim.
- **`ClientDisconnect` (499) writes no body** — the client is gone.

Two catalogue entries are dynamic: `AccessDenied` names the HTTP verb and the
request path (`Access denied: GET access to /bucket/prefix. Check the
credential permissions for this operation. Request ID: <id>.`), and
`InvalidRange` explains the compressed-object alternative when the detail
indicates compression. Every other code resolves statically; the authoritative
message text lives in `internal/server/s3errors/catalog.go`, and
`internal/server/s3errors/catalog_test.go` plus
`internal/server/handlers/error_catalogue_callsite_test.go` hold it to the
contract above (the latter walks every `writeError` call site and rejects
client strings containing Go identifiers or `/admin/` paths).

The status tables below therefore quote **log detail, not wire text**: the
`Log Detail` column is what the handler recorded in the structured log; the
client saw the catalogue message for that row's error code.

### Error Codes by HTTP Status

Every S3 `<Code>` value `internal/server/handlers` can write to a client, in
one place. `internal/docsindex` checks this table against the package's
source on every run, so an added or removed code fails CI here rather than
going stale silently.

<!-- handlers-error-codes:start -->
| Error Code | HTTP Status |
|------------|-------------|
| `AccessDenied` | 403 |
| `InternalError` | 500 |
| `InvalidArgument` | 400 |
| `InvalidCopySource` | 400 |
| `InvalidPart` | 400 |
| `InvalidPartSize` | 400 |
| `InvalidRange` | 416 (one code path returns 400 — see [Inconsistencies](#inconsistencies-and-remediation)) |
| `InvalidRequest` | 400 |
| `MalformedXML` | 400 |
| `MethodNotAllowed` | 405 |
| `NoSuchBucket` | 404 |
| `NoSuchKey` | 404 |
| `NoSuchUpload` | 404 |
| `NotImplemented` | 501 |
| `PreconditionFailed` | 412 |
| `RequestTimeout` | 503 |
| `SlowDown` | 503 |
<!-- handlers-error-codes:end -->

#### HTTP 400 Bad Request

| Error Code | Log Detail | When Returned |
|------------|---------|---------------|
| `InvalidRequest` | Unsupported POST operation | Unknown POST operations |
| `InvalidRequest` | Missing partNumber | UploadPart without partNumber query param |
| `InvalidRequest` | Invalid partNumber | PartNumber out of range (1-10000) |
| `InvalidRequest` | No parts specified | CompleteMultipartUpload with empty parts list |
| `InvalidArgument` | PutObject supports only If-None-Match: * | A conditional PUT with an `If-None-Match` value other than `*` |
| `InvalidPart` | Part retry not supported (CTR counter derivation) | UploadPart re-sends a part number already received |
| `InvalidPartSize` | Part size %d is not a multiple of the block size | A non-final multipart part is not block-aligned (ADR-015) |
| `MalformedXML` | Failed to parse XML: [details] | Invalid XML in request body |
| `MalformedXML` | No objects specified for deletion | DeleteObjects with empty object list |
| `InvalidCopySource` | Invalid copy source format | Malformed x-amz-copy-source header |

**Headers:** `Content-Type: application/xml` (no CORS headers)

#### HTTP 403 Forbidden - Authentication Errors

| Error Code | Log Detail | Trigger |
|------------|---------|---------|
| `MissingAuthenticationToken` | Missing Authentication Token | Authorization header is missing |
| `InvalidAccessKeyId` | The AWS Access Key Id you provided does not exist | Unknown access key |
| `SignatureDoesNotMatch` | The request signature we calculated does not match the signature you provided | SigV4 signature mismatch |
| `RequestExpired` | Request has expired | Request timestamp outside ±15 minute window |
| `InvalidAlgorithm` | Only AWS4-HMAC-SHA256 is supported | Non-SigV4 algorithm |
| `IncompleteSignature` | Authorization header is missing required fields | Malformed Authorization header |
| `InvalidCredential` | Invalid credential format | Malformed Credential field |
| `MissingDateHeader` | Missing X-Amz-Date header | Required header absent |
| `InvalidDateFormat` | Invalid date format in X-Amz-Date header | Date not in ISO8601 basic format |
| `AccessDenied` | Access Denied | ACL-based authorization rejection, access to the `.armor/` reserved namespace, or an append-only credential's unconditional write to an existing key — its explicit `If-None-Match: *` conflict is a 412 `PreconditionFailed` instead (ADR-017) |

**Headers:** `Content-Type: application/xml` plus CORS headers (see above)

**Example Response** (client message from the catalogue; the log detail row
above is what the structured `s3_error` event recorded):
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>SignatureDoesNotMatch</Code>
  <Message>The request signature does not match. Check the signing key, region, and signed headers, then retry. Request ID: 0123456789ABCDEF.</Message>
</Error>
```

#### HTTP 404 Not Found

| Error Code | Log Detail | When Returned |
|------------|---------|---------------|
| `NoSuchKey` | Object not found | GetObject/HeadObject on non-existent object |
| `NoSuchBucket` | Bucket not found | GetBucketLocation/HeadBucket on non-existent bucket |
| `NoSuchUpload` | Multipart upload not found | UploadPart/CompleteMultipartUpload/AbortMultipartUpload on non-existent upload |
| `NoSuchUpload` | Multipart upload does not match bucket/key | Upload ID exists but for different bucket/key |

**Headers:** `Content-Type: application/xml` (no CORS headers)

#### HTTP 405 Method Not Allowed

| Error Code | Log Detail | When Returned |
|------------|---------|---------------|
| `MethodNotAllowed` | Method X not allowed | HTTP method not supported for endpoint |

**Headers:** `Content-Type: application/xml` (no CORS headers)

#### HTTP 412 Precondition Failed

| Error Code | Log Detail | When Returned |
|------------|---------|---------------|
| `PreconditionFailed` | Precondition failed | If-Match or If-Unmodified-Since condition not met, a conditional PUT's `PutIfAbsent` losing the create-only race, or an append-only credential's own explicit `If-None-Match: *` PUT hitting an existing key (ADR-017) |

**Headers:** `Content-Type: application/xml` (no CORS headers)

#### HTTP 416 Range Not Satisfiable

| Error Code | Log Detail | When Returned |
|------------|---------|---------------|
| `InvalidRange` | Invalid range: [details] | Malformed Range header or out of bounds (GetObject, V3 multipart range reads) |
| `InvalidRange` | Range reads unsupported on compressed objects | Byte-range request against a zstd/gzip/zlib-compressed object |

**Headers:** `Content-Type: application/xml` (no CORS headers)

**Note:** one GetObject code path (the shared non-multipart range handler) returns
400 instead of 416 for the identical malformed-Range-header case; see
[Inconsistencies and Remediation](#inconsistencies-and-remediation).

#### HTTP 500 Internal Server Error

| Error Code | Log Detail | When Returned |
|------------|---------|---------------|
| `InternalError` | Failed to [operation]: [details] | Backend/cryptographic failures (key management, encryption, B2 operations) |

**Headers:** `Content-Type: application/xml` (no CORS headers)

#### HTTP 501 Not Implemented

| Error Code | Log Detail | When Returned |
|------------|---------|---------------|
| `NotImplemented` | The storage backend does not support atomic conditional/append-only writes | A conditional PUT (`If-None-Match: *`) or append-only write targets a backend without atomic create-only support |

**Headers:** `Content-Type: application/xml` (no CORS headers)

#### HTTP 503 Service Unavailable

Two independent 503 contracts exist: S3 XML errors from request processing, and
the plain-text readiness probe.

**S3 XML errors:**

| Error Code | Log Detail | When Returned |
|------------|---------|---------------|
| `RequestTimeout` | ARMOR request deadline exceeded | ARMOR's own request deadline expires while a response can still be written. Counted in `armor_errors_total`. |
| `SlowDown` | [part-1-pinning deferral message] | UploadPart defers an out-of-order part until part 1 arrives and pins the block size |

**Readiness probe (plain text, not XML):**

| Status | Response | When |
|--------|----------|------|
| 200 | `Ready` | Readiness probe healthy |
| 503 | `Not ready - canary check failed` | Canary verification failed |
| 503 | `Not ready - manifest writer has never flushed` | Manifest startup lag |
| 503 | `Not ready - manifest writer last flush X ago (threshold 60s)` | Manifest writer stall |
| 503 | `Not ready - no health signal available` | Health check unavailable |

**Note:** `/readyz` returns plain text, not XML, as it is a health endpoint.

### Client Disconnects and Request Deadlines

When a client cancels while ARMOR reads the request body or processes the
request, or the response writer detects a disconnected client, ARMOR records
the request as a client disconnect. The access log uses
`outcome=client_disconnect`, synthetic `status=499`, and WARN level;
`armor_client_disconnects_total{operation}` increments once. The 499 value is
for the access log: the disconnected client cannot receive an S3 XML error
response, and the event does not increment `armor_errors_total`.

When ARMOR's own request deadline expires, the request is a server failure:
ARMOR returns `RequestTimeout` with HTTP 503 where a response can be written,
and counts the S3 error in `armor_errors_total`. Other backend network
failures remain `InternalError` responses as described below.

### Authentication Errors (HTTP 403)

All authentication/authorization errors return HTTP 403 with CORS headers. The ten authentication error codes are:

| Error Code | Log Detail | Status Code | CORS Headers |
|------------|---------|-------------|--------------|
| `MissingAuthenticationToken` | Missing Authentication Token | 403 | ✓ |
| `InvalidAccessKeyId` | The AWS Access Key Id you provided does not exist | 403 | ✓ |
| `SignatureDoesNotMatch` | The request signature we calculated does not match the signature you provided | 403 | ✓ |
| `RequestExpired` | Request has expired | 403 | ✓ |
| `InvalidAlgorithm` | Only AWS4-HMAC-SHA256 is supported | 403 | ✓ |
| `IncompleteSignature` | Authorization header is missing required fields | 403 | ✓ |
| `InvalidCredential` | Invalid credential format | 403 | ✓ |
| `MissingDateHeader` | Missing X-Amz-Date header | 403 | ✓ |
| `InvalidDateFormat` | Invalid date format in X-Amz-Date header | 403 | ✓ |
| `AccessDenied` | Access Denied | 403 | ✓ |

### Per-Operation Errors

#### Object Operations

| Operation | HTTP 200 Success Headers | HTTP 4xx Error Codes | HTTP 5xx Error Codes |
|-----------|------------------------|---------------------|---------------------|
| GetObject | Content-Type, Content-Length, ETag, Last-Modified, Accept-Ranges | InvalidRange, PreconditionFailed | InternalError |
| HeadObject | Content-Type, Content-Length, ETag, Last-Modified, Accept-Ranges | (none) | InternalError |
| PutObject | ETag, X-Armor-Streaming (if >10MB) | InvalidRequest, InvalidArgument, PreconditionFailed, NotImplemented | InternalError |
| DeleteObject | (none - 204 No Content) | (none) | InternalError |
| CopyObject | Content-Type (XML body with CopyObjectResult) | InvalidCopySource | InternalError |

#### Range Requests

| HTTP Status | Error Code | Headers |
|-------------|------------|---------|
| 206 (success) | N/A | Content-Range: bytes start-end/total, X-Armor-Footer-Cache (if Parquet footer hit) |
| 416 (or 400, see Inconsistencies) | InvalidRange | Content-Type: application/xml |
| 412 | PreconditionFailed | Content-Type: application/xml |
| 404 | NoSuchKey | Content-Type: application/xml |

#### Bucket Operations

| Operation | HTTP 200 Success Headers | HTTP 404 Error | HTTP 5xx Error |
|-----------|------------------------|---------------|----------------|
| ListObjectsV2 | Content-Type: application/xml | (none) | InternalError |
| HeadBucket | (none) | NoSuchBucket | InternalError |
| GetBucketLocation | Content-Type: application/xml | NoSuchBucket | InternalError |
| CreateBucket | Location: /bucket-name | (none) | InternalError |
| DeleteBucket | (none) | (none) | InternalError |

#### Multipart Upload Operations

| Operation | HTTP 200 Success Headers | HTTP 400 Errors | HTTP 404 Errors | HTTP 5xx Errors |
|-----------|------------------------|-----------------|-----------------|----------------|
| CreateMultipartUpload | (none) | (none) | (none) | InternalError |
| UploadPart | ETag | InvalidRequest, InvalidPart, InvalidPartSize | NoSuchUpload | InternalError, SlowDown (503) |
| CompleteMultipartUpload | Content-Type: application/xml | MalformedXML, InvalidRequest | NoSuchUpload | InternalError |
| AbortMultipartUpload | (none - 204) | (none) | NoSuchUpload | InternalError |
| ListParts | Content-Type: application/xml | (none) | NoSuchUpload | InternalError |
| ListMultipartUploads | Content-Type: application/xml | (none) | (none) | InternalError |

#### Bulk Operations

| Operation | HTTP 200 Success | HTTP 400 Errors | HTTP 5xx Errors |
|-----------|------------------|-----------------|-----------------|
| DeleteObjects (POST with ?delete) | Content-Type: application/xml | MalformedXML | InternalError |

#### Backend Error Propagation

ARMOR's backend (B2/Cloudflare R2) may return errors that are wrapped in `InternalError` responses:

| Backend Error | ARMOR Response |
|---------------|----------------|
| B2 auth failure | 500 InternalError |
| Backend network failure without a deadline expiry | 500 InternalError |
| Storage full | 500 InternalError |
| Rate limited | 500 InternalError |

**Note:** Backend-specific error details are logged but not exposed to clients in error messages for security reasons — the client-message catalogue (see [Client Messages and Log Detail](#client-messages-and-log-detail)) enforces this for every code, not just backend failures.

---

## Admin Endpoints

Admin endpoints use mixed response formats (JSON, plain text, and some XML for S3-compatible endpoints like `/admin/presign`).

### Endpoint Summary

| Endpoint | Methods | Success Format | Error Format | Purpose |
|----------|---------|----------------|--------------|---------|
| `/healthz` | GET | Plain Text | (none - always 200) | Liveness probe |
| `/readyz` | GET | Plain Text | Plain Text | Readiness probe |
| `/admin/key/verify` | GET | JSON | JSON | Verify MEK correctness |
| `/admin/key/rotate` | POST | JSON | Plain text / JSON | Rotate master encryption key |
| `/admin/key/export` | GET | JSON | Plain Text | Export current MEK |
| `/admin/presign` | POST | JSON | XML (auth) / Plain text (validation) | Generate pre-signed URL |
| `/admin/b2/keys` | GET, POST | JSON | JSON (in text/plain) | List/create B2 keys |
| `/admin/b2/keys/{id}` | DELETE | (none - 204) | JSON (in text/plain) | Delete B2 key |
| `/armor/canary` | GET | JSON | Plain Text | Canary status |
| `/armor/audit` | GET | JSON | JSON / Plain Text | Audit status |
| `/metrics` | GET | Plain Text | (none - always 200) | Prometheus metrics |
| `/share/{token}` | GET | Binary | Plain Text | Access pre-signed URL |

### Health Endpoints

#### `/healthz` - Liveness Probe

**Method:** GET

**Success Response (200 OK):**
```
Status: 200 OK
Content-Type: text/plain

OK
```

**Error Responses:** None (always returns 200)

---

#### `/readyz` - Readiness Probe

**Method:** GET

**Success Response (200 OK):**
```
Status: 200 OK
Content-Type: text/plain

Ready
```

**Error Responses (503 Service Unavailable):**
```
Status: 503 Service Unavailable
Content-Type: text/plain

Not ready - canary check failed
```

```
Status: 503 Service Unavailable
Content-Type: text/plain

Not ready - manifest writer has never flushed
```

```
Status: 503 Service Unavailable
Content-Type: text/plain

Not ready - manifest writer last flush 120s ago (threshold 60s)
```

### Key Management

#### `/admin/key/verify` - Verify MEK

**Method:** GET

**Success Responses (200 OK):**

MEK Verified:
```json
{
  "status": "verified",
  "message": "MEK is correct"
}
```

Canary Not Configured:
```json
{
  "status": "unknown",
  "error": "canary monitor not configured"
}
```

**Error Response (503 Service Unavailable):**
```json
{
  "status": "unverified",
  "error": "canary check failed - MEK may be incorrect"
}
```

**Error Response (405 Method Not Allowed):**
```
Status: 405 Method Not Allowed
Content-Type: text/plain

Method not allowed
```

---

#### `/admin/key/rotate` - Rotate Master Encryption Key

**Method:** POST

**Success Response (200 OK):**
```json
{
  "status": "completed",
  "rotated_objects": 123,
  "failed_objects": 0,
  "duration_ms": 45678,
  "started_at": "2024-01-01T00:00:00Z",
  "completed_at": "2024-01-01T00:00:45Z"
}
```

**Error Responses (400 Bad Request):**
```
Failed to read request body: <error details>
```
```
Invalid hex-encoded MEK
```
```
Invalid MEK length: expected 32 bytes or 64 hex chars, got <actual length>
```

**Error Response (405 Method Not Allowed):**
```
Method not allowed
```

**Error Response (500 Internal Server Error):**
```json
{
  "status": "failed",
  "error": "<error message>",
  "result": {
    "rotated_objects": 10,
    "failed_objects": 1
  }
}
```

---

#### `/admin/key/export` - Export Current MEK

**Method:** GET

**Query Parameter:** `confirm=yes` (required)

**Success Response (200 OK):**
```json
{
  "mek": "64-char-hex-encoded-key",
  "format": "hex",
  "warning": "This key provides access to all encrypted data. Store securely."
}
```

**Error Response (400 Bad Request):**
```
Must include ?confirm=yes to export key
```

**Error Response (405 Method Not Allowed):**
```
Method not allowed
```

### Pre-signed URLs

#### `/admin/presign` - Generate Pre-signed URL

**Method:** POST

**Request Body:**
```json
{
  "bucket": "my-bucket",           // Optional, defaults to configured bucket
  "key": "path/to/file.parquet",   // Required
  "expires_in": "1h",              // Optional, defaults to 1h
  "content_disposition": "...",    // Optional
  "range": "bytes=0-1023"          // Optional
}
```

**Success Response (200 OK):**
```json
{
  "url": "https://...",
  "expires_in": "1h",
  "expires_at": "2024-01-01T01:00:00Z"
}
```

**Error Response (403 Forbidden - Auth Errors):**

⚠️ **INCONSISTENT:** Returns XML format (S3-compatible), unlike other admin endpoints.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Access denied: POST access to /admin/presign. Check the credential permissions for this operation. Request ID: 0123456789ABCDEF.</Message>
  <RequestId>0123456789ABCDEF</RequestId>
</Error>
```

(`Invalid credentials` is the log detail the handler supplied; the client
message comes from the catalogue — see
[Client Messages and Log Detail](#client-messages-and-log-detail).)

**Error Responses (400 Bad Request):**
```
Invalid request body: <error details>
```
```
key is required
```
```
Invalid expires_in: <error details>
```

**Error Response (500 Internal Server Error):**
```
Failed to generate URL: <error details>
```

### B2 Key Management

#### `/admin/b2/keys` - List/Create B2 Keys

**GET - List Keys:**

**Success Response (200 OK):**
```json
{
  "keys": [
    {
      "id": "keyId1",
      "name": "key-name",
      "capabilities": ["readFiles", "writeFiles"],
      "key_id": "appId_keyId",
      "secret_key": "appKey...",
      "expires_at": "2024-01-01T01:00:00Z"
    }
  ],
  "next_cursor": "cursor-for-next-page"
}
```

**Error Responses:**

503 Service Unavailable:
```
{"error":"B2 key management not available - check B2 credentials"}
```

500 Internal Server Error:
```
{"error":"Failed to list keys: <error details>"}
```

---

**POST - Create Key:**

**Request Body:**
```json
{
  "name": "key-name",
  "capabilities": ["readFiles", "writeFiles"],
  "valid_duration_seconds": 3600
}
```

**Success Response (201 Created):**
```json
{
  "id": "keyId1",
  "name": "key-name",
  "capabilities": ["readFiles", "writeFiles"],
  "key_id": "appId_keyId",
  "secret_key": "appKey...",
  "expires_at": "2024-01-01T01:00:00Z"
}
```

**Error Responses (400 Bad Request):**
```
{"error":"Invalid request body: <error details>"}
```
```
{"error":"name is required"}
```
```
{"error":"capabilities is required"}
```

---

#### `/admin/b2/keys/{id}` - Delete B2 Key

**Method:** DELETE

**Success Response (204 No Content):**
```
(no body)
```

**Error Responses:**

503 Service Unavailable:
```
{"error":"B2 key management not available - check B2 credentials"}
```

404 Not Found:
```
{"error":"key not found"}
```

400 Bad Request:
```
{"error":"key ID is required"}
```

### Status Endpoints

#### `/armor/canary` - Canary Status

**Method:** GET

**Success Response (200 OK):**
```json
{
  "decrypt_verified": true,
  "hmac_verified": true,
  "last_check": "2024-01-01T00:00:00Z",
  "error": ""
}
```

**Alternative Success (200 OK - without canary):**
```json
{
  "status": "unknown",
  "error": "canary monitor not configured"
}
```

**Error Response (405 Method Not Allowed):**
```
Method not allowed
```

---

#### `/armor/audit` - Audit Status

**Method:** GET

**Success Response (200 OK):**
```json
{
  "total_objects": 1234,
  "verified_objects": 1230,
  "failed_objects": 4,
  "errors": ["error1", "error2"]
}
```

**Error Response (500 Internal Server Error):**
```json
{
  "status": "error",
  "error": "<error details>"
}
```

**Error Response (400 Bad Request):** malformed `writer`, `untracked`,
`max_entries`, or `timeout` query parameter (see
[provenance-audit-walker.md](provenance-audit-walker.md) for the bounded
audit parameters).
```json
{
  "status": "error",
  "error": "invalid max_entries parameter \"three\": expected a positive integer"
}
```

**Error Response (405 Method Not Allowed):**
```
Method not allowed
```

---

#### `/metrics` - Prometheus Metrics

**Method:** GET

**Success Response (200 OK):**
```
# HELP armor_requests_total Total number of requests
# TYPE armor_requests_total counter
armor_requests_total 1234
...
```

**Key Replication Metrics:**
```
# HELP armor_replication_enqueued_total Total number of items enqueued for replication by operation
# TYPE armor_replication_enqueued_total counter
armor_replication_enqueued_total{operation="put"} 1234
armor_replication_enqueued_total{operation="put-streaming"} 567

# HELP armor_replication_queue_depth Current number of items in the replication queue
# TYPE armor_replication_queue_depth gauge
armor_replication_queue_depth 42

# HELP armor_replication_dropped_total Total number of items dropped due to full replication queue
# TYPE armor_replication_dropped_total counter
armor_replication_dropped_total 0
```

**Headers:** `Content-Type: text/plain; version=0.0.4` (Prometheus text format)

#### `/share/{token}` - Access Pre-signed URL

**Method:** GET

**Success Response (200 OK or 206 Partial Content):**
- Binary data (decrypted object content)
- Headers: `Content-Length`, `Content-Type`, `Accept-Ranges: bytes`, `Content-Disposition` (if specified), `Content-Range` (for 206)

**Error Responses:**

| Status | Response | When |
|--------|----------|------|
| 400 | `Missing token` | No token in path |
| 400 | `Invalid token` | Token malformed |
| 403 | `Invalid link` | Cryptographic signature verification failed |
| 404 | `Object not found: {details}` | Object does not exist in storage |
| 410 | `Link expired` | Token expiration time passed |
| 500 | `Failed to [operation]` | Decryption or backend failure |

---

## Verification Error Format

The restore-verification pipeline (ADR-004) uses a separate, non-HTTP error
type to report byte-level decompression mismatches: `VerificationError` in
`internal/crypto/verify_decompress.go`. It is a Go struct, not a wire format,
but its fields follow a fixed, documented structure so callers can build
consistent diagnostics without parsing the `Error()` string.

```go
type VerificationError struct {
    // Offset is the byte position where the first difference occurs.
    // Special values: -1 (no error, unused sentinel), -2 (length mismatch)
    Offset int64

    // Expected is the byte(s) that should be present at the error offset.
    // Single-byte errors: 1 byte. With context: [ContextBefore + 1 + ContextAfter] bytes.
    Expected []byte

    // Actual is the byte(s) found in the decompressed content at the error offset.
    // Mirrors Expected's structure.
    Actual []byte

    // ContextBytes, ContextBefore, ContextAfter control how much surrounding
    // data Expected/Actual carry around the error offset.
    ContextBytes, ContextBefore, ContextAfter int

    // ExpectedLength and ActualLength are the total expected/actual data
    // lengths, used to diagnose length-mismatch errors (Offset == -2).
    ExpectedLength, ActualLength int
}
```

**Offset calculation:** start byte-by-byte comparison from position 0; the
first index `i` where `decompressed[i] != expected[i]` is the offset.

**Error message format:**

```
verification failed: byte mismatch at offset {offset} (expected 0x{Expected}, got 0x{Actual})
```

**Error classification examples:**

- Byte mismatch: `Offset: 512, Expected: 0x03, Actual: 0x00` — single byte corrupted.
- Length mismatch: `Offset: -2, ExpectedLength: 1024, ActualLength: 997` — decompressed output 27 bytes short.

---

## Inconsistencies and Remediation

### Summary of Inconsistencies

| Inconsistency | Severity | Affected Endpoints | Impact |
|---------------|----------|-------------------|--------|
| Mixed error formats (JSON/XML/Plain) | Medium | `/admin/presign` | API confusion |
| Content-Type mismatches | Medium | `/admin/b2/keys/*` | JSON in text/plain |
| 405 returns plain text | Medium | Most admin endpoints | Not S3-compatible |
| Admin endpoint format inconsistency | Low | All admin endpoints | Poor DX |
| `InvalidRange` status code varies by code path | Low | GetObject (non-multipart range handler) | AWS uses 416; one path returns 400 for the identical failure |

### Known Inconsistencies

#### 1. Content-Type Header Mismatches

**Issue:** Many admin endpoints return JSON error responses but declare `Content-Type: text/plain`.

| Endpoint | Problem |
|----------|---------|
| `/admin/b2/keys` | Error responses return JSON but declare `text/plain` |
| `/admin/b2/keys/{id}` | Error responses return JSON but declare `text/plain` |

#### 2. Method Not Allowed Returns Plain Text

**Issue:** HTTP 405 Method Not Allowed responses use `text/plain` format across all admin endpoints, even when the endpoint normally returns JSON.

**Current Behavior:**
```
HTTP/1.1 405 Method Not Allowed
Content-Type: text/plain

Method not allowed
```

This is handled by Go's `http.Error()` function.

#### 3. Admin Endpoint Auth Uses S3 XML Format

**Issue:** `/admin/presign` uses S3 XML error format for authentication failures, while other admin endpoints use plain text or JSON.

**Admin endpoint auth errors:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Access denied: POST access to /admin/presign. Check the credential permissions for this operation. Request ID: 0123456789ABCDEF.</Message>
  <RequestId>0123456789ABCDEF</RequestId>
</Error>
```

This is because `/admin/presign` calls the server's `writeError` helper, which uses the S3 error format. The `<Message>` text comes from the client-message catalogue; the handler's detail string (`Invalid credentials`) goes only to the structured log.

#### 4. Response Format Variation Within Endpoints

| Endpoint | Success Format | Error Format | Issue |
|----------|---------------|--------------|-------|
| `/admin/key/verify` | JSON | JSON | Consistent ✅ |
| `/admin/key/rotate` | JSON | Plain text / JSON | ⚠️ Mixed |
| `/admin/key/export` | JSON | Plain text | ⚠️ Inconsistent |
| `/admin/presign` | JSON | XML / Plain text | ⚠️ Multiple formats |
| `/admin/b2/keys` | JSON | JSON in Plain | ⚠️ Type mismatch |
| `/armor/canary` | JSON | Plain text | ⚠️ Inconsistent |
| `/armor/audit` | JSON | Mixed | ⚠️ Inconsistent |

#### 5. `InvalidRange` Status Code Varies by Code Path

**Issue:** GetObject's inline range-parsing (compressed-object rejection and the
V3 multipart range path) returns 416, matching AWS S3 semantics for
`InvalidRange`. The shared non-multipart `handleRangeRequest` path returns 400
for the identical malformed-Range-header failure. Both paths use the same
error code and message shape; only the HTTP status differs.

### Remediation Plan

#### Priority 1 (S3 Protocol Compliance)

| Issue | Effort | Recommendation |
|-------|--------|----------------|
| Method Not Allowed XML format | Low | Convert 405 errors to XML for S3 endpoints |
| Presigned URL endpoint format | Low | Convert `/admin/presign` errors to XML consistently |
| `InvalidRange` status code | Low | Make `handleRangeRequest` return 416, matching the other range-parsing paths |

#### Priority 2 (Admin Interface Consistency)

| Issue | Effort | Recommendation |
|-------|--------|----------------|
| Admin API JSON format | Medium | Standardize admin endpoints on JSON format |
| Content-Type header fixes | Low | Fix `Content-Type` to match response body |

#### Recommendation: Standardized Admin Error Format

```json
{
  "error": "ErrorCode",
  "message": "Detailed error message",
  "details": {
    "field": "value"
  }
}
```

### Consistency Verification Summary

| Aspect | Status | Details |
|--------|--------|---------|
| S3 Content-Type consistency | ✅ PASS | All S3 error responses return `application/xml` |
| S3 HTTP status code consistency | ⚠️ PARTIAL | All auth errors return 403; `InvalidRange` varies by code path (see above) |
| S3 XML structure consistency | ✅ PASS | All responses follow S3 XML error format |
| S3 error code casing | ✅ PASS | All codes use PascalCase |
| S3 XML escaping | ✅ PASS | Special characters properly escaped |
| Admin endpoint consistency | ⚠️ PARTIAL | Mixed formats, see inconsistencies above |
| CORS headers on 403 | ✅ PASS | All auth errors include CORS headers |

---

## Implementation Reference

### Error Response Writers

ARMOR has two `writeError` functions that handle S3 error responses:

1. **`internal/server/server.go`'s `writeError`** - Handles authentication/authorization errors and admin-endpoint auth failures
2. **`internal/server/handlers/handlers.go`'s `writeError`** - Handles S3 operation errors; delegates to `writeErrorCause`, which also reclassifies the response as `RequestTimeout` or a client disconnect, logs a structured `s3_error` event, and increments `armor_errors_total{code,operation}` (or `armor_client_disconnects_total{operation}` for disconnects)

Both implementations resolve the error through the client-message catalogue
before writing anything (see
[Client Messages and Log Detail](#client-messages-and-log-detail)):

```go
errorEntry := s3errors.Resolve(code, s3errors.Request{
    Method: r.Method, Path: r.URL.Path,
    RequestID: requestID, Status: statusCode,
}, message) // message becomes LogDetail, never the wire text

w.Header().Set("Content-Type", "application/xml")
w.WriteHeader(statusCode)
var codeBuf, msgBuf bytes.Buffer
xml.EscapeText(&codeBuf, []byte(code))
xml.EscapeText(&msgBuf, []byte(errorEntry.ClientMessage))
fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<Error>\n  <Code>%s</Code>\n  <Message>%s</Message>\n</Error>",
    codeBuf.String(), msgBuf.String())
```

**Note:** This code duplication is a maintainability concern (both functions must be updated together), but is not currently a functional inconsistency. The catalogue itself is shared, so the two writers cannot drift on client message text.

**Observability (ADR-008):** every call through `writeError`/`writeErrorCause`
logs a structured `s3_error` event (code, operation, bucket, key, request ID,
status, log detail) and increments the `armor_errors_total{code,operation}`
counter, giving operators a code-granular axis beyond the generic
request/status counter.

### Admin Error Responses

Admin endpoints use Go's standard `http.Error()` function for most errors, which returns plain text with `Content-Type: text/plain`:

```go
http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
```

For S3-compatible auth errors, admin endpoints call the XML `writeError()` function:

```go
s.writeError(w, "AccessDenied", "Invalid credentials", 403)
```

Here `"Invalid credentials"` is the log-detail argument; the client receives
the catalogue's `AccessDenied` message (naming the verb and path).

### Performance

Error responses are consistently fast:
- No additional headers added beyond `Content-Type` and CORS (for 403)
- Response time dominated by authentication verification, not header setting
- Single `WriteHeader` call per response

### S3 Compliance

✅ **Compliant** with [AWS S3 Error Responses](https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html):
- XML format matches S3 specification
- Error codes match S3 error codes
- HTTP status codes match S3 behavior (except the `InvalidRange` path noted above)
- Content-Type header matches S3 (`application/xml`)

⚠️ **Partial Deviations:**
- CORS headers on errors differ from AWS (present in ARMOR, absent in AWS unless configured)
- Backend errors wrapped as `InternalError` hide specific S3 error codes

✅ **Implemented:**
- `x-amz-request-id` header (request tracing) - set from middleware context
- `x-amz-id-2` header (extended request ID) - set from middleware context

---

## Test Coverage

Error-response behavior is exercised by:

- `internal/server/invalid_credential_test.go` and `internal/server/invalid_credential_integration_test.go` - Auth rejection scenarios
- `internal/server/malformed_signature_test.go` - Malformed signature scenarios
- `internal/server/content_type_consistency_test.go` - Content-Type verification
- `internal/server/error_response_verification_test.go` - Comprehensive error verification
- `internal/server/auth_test.go` - Authentication unit tests
- `internal/server/s3errors/catalog_test.go` - Client-message catalogue: message/log-detail separation, `AccessDenied` verb+path, clock-skew and compressed-range guidance
- `internal/server/handlers/error_catalogue_callsite_test.go` - AST guard: walks every `writeError`/`writeErrorCause`/`writeBackendError` call site and rejects client strings containing Go identifiers or `/admin/` paths
- `internal/docsindex` - Verifies every S3 error code `internal/server/handlers` can emit appears in this document, and no documented code goes unemitted

**Run tests:**
```bash
# Run all error response tests
go test -v -run "TestInvalidCredentialRejection|TestMalformedSignatureRejection" ./internal/server/

# Run Content-Type consistency test
go test -v -run TestContentTypeConsistencyAcrossAllRejections ./internal/server/

# Run comprehensive error verification
go test -v -run TestComprehensiveErrorVerification ./internal/server/

# Run the client-message catalogue tests
go test ./internal/server/s3errors/

# Run the doc/code error-code parity check
go test ./internal/docsindex/
```

**Coverage gaps:** validation errors (`InvalidArgument`, `InvalidPart`,
`InvalidPartSize`, `MalformedXML`) and backend-triggered `InternalError`
responses have limited or no dedicated unit tests; most are exercised only
indirectly through integration tests against a real backend.

---

## References

### AWS S3 Documentation
- [S3 Error Responses](https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html)
- [S3 Error Code List](https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html#ErrorCodeList)

### Implementation Files
- `internal/server/server.go` - Main server, admin endpoints, auth errors
- `internal/server/handlers/handlers.go` - S3 operation error responses
- `internal/server/handlers/conditional_put.go` - Conditional/append-only PUT error translation
- `internal/server/auth.go` - Authentication error definitions
- `internal/crypto/verify_decompress.go` - Restore-verification error structure

### Related ADRs
- [ADR-004](adr/004-continuous-restore-verification.md) - Continuous dual-path restore verification (defines the verification error's role)
- [ADR-008](adr/008-multipart-part-size-error-clarity.md) - Server-side observability for error responses (`armor_errors_total`, structured `s3_error` logging)
- [ADR-015](adr/015-out-of-order-multipart-uniform-part-size.md) - Uniform-part-size contract (`InvalidPartSize`, `InvalidPart`, `SlowDown`)

---

**End of Document**
