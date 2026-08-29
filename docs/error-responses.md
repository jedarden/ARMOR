# ARMOR Error Responses

**Version:** 1.0  
**Date:** 2026-08-29  
**Status:** Active

## Overview

This document is the authoritative reference for all error responses returned by ARMOR. It consolidates and replaces the following previous documentation:

- `admin-endpoint-error-headers.md`
- `admin-endpoint-error-response-headers.md`
- `error-response-header-consistency.md`
- `error-response-headers-specification.md`
- `error-header-spec.md`
- `s3-endpoint-response-headers.md`
- `auth-rejection-headers.md`

## Scope

ARMOR has two distinct API surfaces with different error response formats:

1. **S3-Facing Endpoints** - Public S3-compatible API returning XML errors
2. **Admin Endpoints** - Management and monitoring API with mixed response formats

## Table of Contents

- [S3-Facing Endpoints](#s3-facing-endpoints)
  - [Error Response Structure](#error-response-structure)
  - [Error Codes by HTTP Status](#error-codes-by-http-status)
  - [Authentication Errors](#authentication-errors-http-403)
  - [Per-Operation Errors](#per-operation-errors)
- [Admin Endpoints](#admin-endpoints)
  - [Endpoint Summary](#endpoint-summary)
  - [Health Endpoints](#health-endpoints)
  - [Key Management](#key-management)
  - [Pre-signed URLs](#pre-signed-urls)
  - [B2 Key Management](#b2-key-management)
  - [Status Endpoints](#status-endpoints)
- [Inconsistencies and Remediation](#inconsistencies-and-remediation)
- [Implementation Reference](#implementation-reference)

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
</Error>
```

**Standard Headers:**

| Header | Value | Presence |
|--------|-------|----------|
| `Content-Type` | `application/xml` | Always |
| `Access-Control-Allow-Origin` | `*` | HTTP 403 only |
| `Access-Control-Allow-Methods` | `GET, PUT, DELETE, HEAD, POST, OPTIONS` | HTTP 403 only |
| `Access-Control-Allow-Headers` | `Authorization, Content-Type, Range, Content-Length` | HTTP 403 only |

**Key Difference from AWS S3:** ARMOR includes CORS headers on error responses, while AWS S3 only includes them when explicitly configured via bucket CORS rules.

### Error Codes by HTTP Status

#### HTTP 400 Bad Request

| Error Code | Message | When Returned |
|------------|---------|---------------|
| `InvalidRequest` | Unsupported POST operation | Unknown POST operations |
| `InvalidRequest` | Missing partNumber | UploadPart without partNumber query param |
| `InvalidRequest` | Invalid partNumber | PartNumber out of range (1-10000) |
| `InvalidRequest` | No parts specified | CompleteMultipartUpload with empty parts list |
| `InvalidRange` | Invalid range: [details] | Malformed Range header or out of bounds |
| `MalformedXML` | Failed to parse XML: [details] | Invalid XML in request body |
| `MalformedXML` | No objects specified for deletion | DeleteObjects with empty object list |
| `InvalidCopySource` | Invalid copy source format | Malformed x-amz-copy-source header |

**Headers:** `Content-Type: application/xml` (no CORS headers)

#### HTTP 403 Forbidden - Authentication Errors

| Error Code | Message | Trigger |
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
| `AccessDenied` | Access Denied | ACL-based authorization rejection |

**Headers:** `Content-Type: application/xml` plus CORS headers (see above)

**Example Response:**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>SignatureDoesNotMatch</Code>
  <Message>The request signature we calculated does not match the signature you provided</Message>
</Error>
```

#### HTTP 404 Not Found

| Error Code | Message | When Returned |
|------------|---------|---------------|
| `NoSuchKey` | Object not found | GetObject/HeadObject on non-existent object |
| `NoSuchBucket` | Bucket not found | GetBucketLocation/HeadBucket on non-existent bucket |
| `NoSuchUpload` | Multipart upload not found | UploadPart/CompleteMultipartUpload/AbortMultipartUpload on non-existent upload |
| `NoSuchUpload` | Multipart upload does not match bucket/key | Upload ID exists but for different bucket/key |

**Headers:** `Content-Type: application/xml` (no CORS headers)

#### HTTP 405 Method Not Allowed

| Error Code | Message | When Returned |
|------------|---------|---------------|
| `MethodNotAllowed` | Method X not allowed | HTTP method not supported for endpoint |

**Headers:** `Content-Type: application/xml` (no CORS headers)

#### HTTP 412 Precondition Failed

| Error Code | Message | When Returned |
|------------|---------|---------------|
| `PreconditionFailed` | Precondition failed | If-Match or If-Unmodified-Since condition not met |

**Headers:** `Content-Type: application/xml` (no CORS headers)

#### HTTP 500 Internal Server Error

| Error Code | Message | When Returned |
|------------|---------|---------------|
| `InternalError` | Failed to [operation]: [details] | Backend/cryptographic failures (key management, encryption, B2 operations) |

**Headers:** `Content-Type: application/xml` (no CORS headers)

#### HTTP 503 Service Unavailable

| Status | Response | When |
|--------|----------|------|
| 200 | `Ready` | Readiness probe healthy |
| 503 | `Not ready - canary check failed` | Canary verification failed |
| 503 | `Not ready - manifest writer has never flushed` | Manifest startup lag |
| 503 | `Not ready - manifest writer last flush X ago (threshold 60s)` | Manifest writer stall |
| 503 | `Not ready - no health signal available` | Health check unavailable |

**Note:** `/readyz` returns plain text, not XML, as it is a health endpoint.

### Authentication Errors (HTTP 403)

All authentication/authorization errors return HTTP 403 with CORS headers. The ten authentication error codes are:

| Error Code | Message | Status Code | CORS Headers |
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

**Verification:** All 10 authentication error codes have been verified to return HTTP 403 (2026-07-14).

### Per-Operation Errors

#### Object Operations

| Operation | HTTP 200 Success Headers | HTTP 4xx Error Codes | HTTP 5xx Error Codes |
|-----------|------------------------|---------------------|---------------------|
| GetObject | Content-Type, Content-Length, ETag, Last-Modified, Accept-Ranges | InvalidRange, PreconditionFailed | InternalError |
| HeadObject | Content-Type, Content-Length, ETag, Last-Modified, Accept-Ranges | (none) | InternalError |
| PutObject | ETag, X-Armor-Streaming (if >10MB) | InvalidRequest | InternalError |
| DeleteObject | (none - 204 No Content) | (none) | InternalError |
| CopyObject | Content-Type (XML body with CopyObjectResult) | InvalidCopySource | InternalError |

#### Range Requests

| HTTP Status | Error Code | Headers |
|-------------|------------|---------|
| 206 (success) | N/A | Content-Range: bytes start-end/total, X-Armor-Footer-Cache (if Parquet footer hit) |
| 400 | InvalidRange | Content-Type: application/xml |
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
| UploadPart | ETag | InvalidRequest | NoSuchUpload | InternalError |
| CompleteMultipartUpload | Content-Type: application/xml | MalformedXML, InvalidRequest | NoSuchUpload | InternalError |
| AbortMultipartUpload | (none - 204) | (none) | NoSuchUpload | InternalError |
| ListParts | Content-Type: application/xml | (none) | NoSuchUpload | InternalError |
| ListMultipartUploads | Content-Type: application/xml | (none) | (none) | InternalError |

#### Bulk Operations

| Operation | HTTP 200 Success | HTTP 400 Errors | HTTP 5xx Errors |
|-----------|------------------|-----------------|----------------|
| DeleteObjects (POST with ?delete) | Content-Type: application/xml | MalformedXML | InternalError |

#### Backend Error Propagation

ARMOR's backend (B2/Cloudflare R2) may return errors that are wrapped in `InternalError` responses:

| Backend Error | ARMOR Response |
|---------------|----------------|
| B2 auth failure | 500 InternalError |
| Network timeout | 500 InternalError |
| Storage full | 500 InternalError |
| Rate limited | 500 InternalError |

**Note:** Backend-specific error details are logged but not exposed to clients in error messages for security reasons.

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

**Code Reference:** `internal/server/server.go:359-376`

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

**Code Reference:** `internal/server/server.go:486-507`

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

**Code Reference:** `internal/server/server.go:510-573`

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

**Code Reference:** `internal/server/server.go:576-596`

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
  <Message>Invalid credentials</Message>
</Error>
```

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

**Code Reference:** `internal/server/server.go:828-913`

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

**Code Reference:** `internal/server/server.go:1246-1324`

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

**Code Reference:** `internal/server/server.go:1326-1364`

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

**Code Reference:** `internal/server/server.go:599-615`

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

**Error Response (405 Method Not Allowed):**
```
Method not allowed
```

**Code Reference:** `internal/server/server.go:617-639`

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

**Code Reference:** `internal/server/server.go:641-680`

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

**Code Reference:** `internal/server/server.go:915-1015`

---

## Inconsistencies and Remediation

### Summary of Inconsistencies

| Inconsistency | Severity | Affected Endpoints | Impact |
|---------------|----------|-------------------|--------|
| Mixed error formats (JSON/XML/Plain) | Medium | `/admin/presign` | API confusion |
| Content-Type mismatches | Medium | `/admin/b2/keys/*` | JSON in text/plain |
| 405 returns plain text | Medium | Most admin endpoints | Not S3-compatible |
| Admin endpoint format inconsistency | Low | All admin endpoints | Poor DX |

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
  <Message>Invalid credentials</Message>
</Error>
```

This is because `/admin/presign` calls `s.writeError()` which uses the S3 error format.

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

### Remediation Plan

#### Priority 1 (S3 Protocol Compliance)

| Issue | Effort | Recommendation |
|-------|--------|----------------|
| Method Not Allowed XML format | Low | Convert 405 errors to XML for S3 endpoints |
| Presigned URL endpoint format | Low | Convert `/admin/presign` errors to XML consistently |

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
| S3 HTTP status code consistency | ✅ PASS | All auth errors return 403 |
| S3 XML structure consistency | ✅ PASS | All responses follow S3 XML error format |
| S3 error code casing | ✅ PASS | All codes use PascalCase |
| S3 XML escaping | ✅ PASS | Special characters properly escaped |
| Admin endpoint consistency | ⚠️ PARTIAL | Mixed formats, see inconsistencies above |
| CORS headers on 403 | ✅ PASS | All auth errors include CORS headers |

---

## Implementation Reference

### Error Response Writers

ARMOR has two `writeError` functions that handle S3 error responses:

1. **`internal/server/server.go:writeError`** (lines 796-805) - Handles authentication/authorization errors
2. **`internal/server/handlers/handlers.go:writeError`** (lines 2695-2704) - Handles S3 operation errors

Both implementations are **identical**:

```go
func writeError(w http.ResponseWriter, code, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    var codeBuf, msgBuf bytes.Buffer
    xml.EscapeText(&codeBuf, []byte(code))
    xml.EscapeText(&msgBuf, []byte(message))
    fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<Error>\n  <Code>%s</Code>\n  <Message>%s</Message>\n</Error>",
        codeBuf.String(), msgBuf.String())
}
```

**Note:** This code duplication is a maintainability concern (both functions must be updated together), but is not currently a functional inconsistency.

### Admin Error Responses

Admin endpoints use Go's standard `http.Error()` function for most errors, which returns plain text with `Content-Type: text/plain`:

```go
http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
```

For S3-compatible auth errors, admin endpoints call the XML `writeError()` function:

```go
s.writeError(w, "AccessDenied", "Invalid credentials", 403)
```

### Performance

Error responses are consistently fast:
- No additional headers added beyond `Content-Type` and CORS (for 403)
- Response time dominated by authentication verification, not header setting
- Single `WriteHeader` call per response

**Average Response Time:** <150µs (measured 2026-07-14)

### Testing

See the following test files for error response verification:

- `internal/server/invalid_credential_test.go` - Auth rejection scenarios
- `internal/server/malformed_signature_test.go` - Malformed signature scenarios
- `internal/server/content_type_consistency_test.go` - Content-Type verification
- `internal/server/error_response_verification_test.go` - Comprehensive error verification

**Run tests:**
```bash
# Run all error response tests
go test -v -run "TestInvalidCredentialRejection|TestMalformedSignatureRejection" ./internal/server/

# Run Content-Type consistency test
go test -v -run TestContentTypeConsistencyAcrossAllRejections ./internal/server/

# Run comprehensive error verification
go test -v -run TestComprehensiveErrorVerification ./internal/server/
```

### S3 Compliance

✅ **Compliant** with [AWS S3 Error Responses](https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html):
- XML format matches S3 specification
- Error codes match S3 error codes
- HTTP status codes match S3 behavior
- Content-Type header matches S3 (`application/xml`)

⚠️ **Partial Deviations:**
- CORS headers on errors differ from AWS (present in ARMOR, absent in AWS unless configured)
- Backend errors wrapped as `InternalError` hide specific S3 error codes

✅ **Implemented (2026-08-29):**
- `x-amz-request-id` header (request tracing) - Now set from middleware context
- `x-amz-id-2` header (extended request ID) - Now set from middleware context

**Note:** These headers are now implemented in the `writeError` functions in both `server.go` (line 1562) and `handlers.go` (line 4712), an improvement from the original 2026-07-14 documentation which noted them as missing.

---

## References

### AWS S3 Documentation
- [S3 Error Responses](https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html)
- [S3 Error Code List](https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html#ErrorCodeList)

### Implementation Files
- `internal/server/server.go` - Main server, admin endpoints, auth errors
- `internal/server/handlers/handlers.go` - S3 operation error responses
- `internal/server/auth.go` - Authentication error definitions

### Verification Beads
- bf-2n6273: Comprehensive header specification
- bf-649uw6: Error response header consistency verification
- bf-4bwxtc: Content-Type header consistency verification
- bf-o7eo21: HTTP status code consistency verification
- bf-5ppsfh: Authentication rejection response headers documentation
- bf-58oib3: Invalid AWS credentials rejection testing

---

**End of Document**
