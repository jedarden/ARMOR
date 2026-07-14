# S3 Endpoint Error Response Headers Specification

## Overview

This document catalogs all error response headers returned by ARMOR's S3-facing endpoints, including CORS behavior and compliance with AWS S3 specifications.

## Error Response Structure

All error responses follow a consistent XML format:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>ErrorCode</Code>
  <Message>Error message</Message>
</Error>
```

### Standard Headers

| Header | Value | Presence |
|--------|-------|----------|
| `Content-Type` | `application/xml` | **Always present** |
| `Access-Control-Allow-Origin` | `*` | **Always present** (CORS) |
| `Access-Control-Allow-Methods` | `GET, PUT, DELETE, HEAD, POST, OPTIONS` | **Always present** (CORS) |
| `Access-Control-Allow-Headers` | `Authorization, Content-Type, Range, Content-Length` | **Always present** (CORS) |

**Key Finding:** CORS headers are set by the `wrapHandler` middleware and are preserved for all error responses. This differs from AWS S3, which does not include CORS headers on error responses by default.

## HTTP Status Codes and Error Types

### 400 Bad Request

| Error Code | Message | Scenarios |
|------------|---------|-----------|
| `InvalidRequest` | Unsupported POST operation | Unknown POST operations |
| `InvalidRequest` | Missing partNumber | UploadPart without partNumber query param |
| `InvalidRequest` | Invalid partNumber | PartNumber out of range (1-10000) |
| `InvalidRequest` | No parts specified | CompleteMultipartUpload with empty parts list |
| `InvalidRange` | Invalid range: [details] | Malformed Range header or out of bounds |
| `MalformedXML` | Failed to parse XML: [details] | Invalid XML in request body |
| `MalformedXML` | No objects specified for deletion | DeleteObjects with empty object list |
| `InvalidCopySource` | Invalid copy source format | Malformed x-amz-copy-source header |

### 403 Forbidden

| Error Code | Message | Scenarios |
|------------|---------|-----------|
| `AccessDenied` | Access Denied | ACL restrictions (bucket/key level) |
| `AccessDenied` | Invalid credentials | Authentication failure (generic) |
| `InvalidAccessKeyId` | The AWS Access Key Id you provided does not exist | Unknown access key |
| `SignatureDoesNotMatch` | The request signature we calculated does not match the signature you provided | SigV4 signature mismatch |
| `MissingAuthenticationToken` | Missing Authentication Token | No Authorization header or query auth |
| `IncompleteSignature` | Authorization header is missing required fields | Malformed Authorization header |
| `InvalidAlgorithm` | Only AWS4-HMAC-SHA256 is supported | Non-SigV4 algorithm |
| `InvalidCredential` | Invalid credential format | Malformed Credential field |
| `MissingDateHeader` | Missing X-Amz-Date header | Required header absent |
| `InvalidDateFormat` | Invalid date format in X-Amz-Date header | Date not in ISO8601 basic format |
| `RequestExpired` | Request has expired | Request timestamp outside skew window |

### 404 Not Found

| Error Code | Message | Scenarios |
|------------|---------|-----------|
| `NoSuchKey` | Object not found: [details] | GetObject/HeadObject/DeleteObject on non-existent object |
| `NoSuchBucket` | Bucket not found: [details] | GetBucketLocation/GetBucketVersioning/HeadBucket on non-existent bucket |
| `NoSuchUpload` | Multipart upload not found: [details] | UploadPart/CompleteMultipartUpload/AbortMultipartUpload/ListParts on non-existent upload |
| `NoSuchUpload` | Multipart upload does not match bucket/key | Upload ID exists but for different bucket/key |

### 405 Method Not Allowed

| Error Code | Message | Scenarios |
|------------|---------|-----------|
| `MethodNotAllowed` | Method X not allowed | HTTP method not supported for endpoint |

### 412 Precondition Failed

| Error Code | Message | Scenarios |
|------------|---------|-----------|
| `PreconditionFailed` | Precondition failed | If-Match or If-Unmodified-Since condition not met |

### 500 Internal Server Error

| Error Code | Message | Scenarios |
|------------|---------|-----------|
| `InternalError` | Failed to [operation]: [details] | All backend/cryptographic failures (key management, encryption, B2 operations) |

### 503 Service Unavailable

| Error Code | Message | Scenarios |
|------------|---------|-----------|
| N/A (plain text) | Not ready - canary check failed | Readiness probe when backend is unhealthy |
| N/A (plain text) | Not ready - manifest writer has never flushed | Manifest startup lag |
| N/A (plain text) | Not ready - manifest writer last flush X ago | Manifest writer stall |

**Note:** 503 responses from `/readyz` are plain text, not XML, as this is a health endpoint, not an S3 operation.

## CORS Header Behavior

### Current Behavior (ARMOR)

CORS headers are set by the `wrapHandler` middleware **before** handler execution and are preserved on error responses:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
```

This applies to **all** responses, including errors.

### AWS S3 Behavior

AWS S3 does **not** include CORS headers on error responses by default. CORS headers are only added:
- When CORS is explicitly configured on the bucket
- For successful responses (200, 206, 304)
- For some 4xx errors when origin matches allowed origins

**Key Difference:** ARMOR always adds CORS headers to error responses, while AWS S3 only adds them when explicitly configured and the origin is permitted.

## Per-Operation Error Catalog

### Object Operations

#### GetObject / HeadObject

| Status | Error Code | When |
|--------|------------|------|
| 304 | N/A (not an error) | If-None-Match or If-Modified-Since condition met |
| 412 | PreconditionFailed | If-Match or If-Unmodified-Since condition not met |
| 404 | NoSuchKey | Object does not exist |
| 500 | InternalError | Backend, decryption, or metadata parsing failure |

**Headers on success:**
- `Content-Length`, `Content-Type`, `ETag`, `Last-Modified`, `Accept-Ranges`
- `X-Armor-Stream: pipelined` (streaming GET only)

**Headers on 304:**
- `ETag`, `Last-Modified`

#### PutObject

| Status | Error Code | When |
|--------|------------|------|
| 400 | InvalidRequest | Various validation failures |
| 500 | InternalError | Key generation, encryption, or backend failure |

**Headers on success:**
- `ETag`
- `X-Armor-Streaming: true` (streaming upload only)

#### DeleteObject

| Status | Error Code | When |
|--------|------------|------|
| 500 | InternalError | Backend deletion failure |

**Headers on success:**
- None (204 No Content)

#### CopyObject

| Status | Error Code | When |
|--------|------------|------|
| 400 | InvalidCopySource | Missing or malformed x-amz-copy-source |
| 404 | NoSuchKey | Source object not found |
| 500 | InternalError | Decryption, re-encryption, or backend failure |

**Headers on success:**
- `Content-Type: application/xml` (CopyObjectResult XML body)

### Range Requests

| Status | Error Code | When |
|--------|------------|------|
| 400 | InvalidRange | Malformed Range header syntax |
| 412 | PreconditionFailed | If-Match or If-Unmodified-Since condition not met |

**Headers on success (206):**
- `Content-Length`, `Content-Type`, `ETag`, `Accept-Ranges`, `Last-Modified`
- `Content-Range: bytes start-end/total`
- `X-Armor-Footer-Cache: HIT` (Parquet footer cache hit only)

### Bucket Operations

#### ListObjectsV2

| Status | Error Code | When |
|--------|------------|------|
| 500 | InternalError | Backend listing failure |

#### HeadBucket / GetBucketLocation / GetBucketVersioning

| Status | Error Code | When |
|--------|------------|------|
| 404 | NoSuchBucket | Bucket does not exist |
| 500 | InternalError | Backend failure |

**Headers on success:**
- None (200 OK or XML body for location/versioning)

#### CreateBucket / DeleteBucket

| Status | Error Code | When |
|--------|------------|------|
| 500 | InternalError | Backend failure |

**Headers on success:**
- `Location: /bucket-name` (CreateBucket only)

### Multipart Upload Operations

#### CreateMultipartUpload

| Status | Error Code | When |
|--------|------------|------|
| 500 | InternalError | Key generation or backend failure |

#### UploadPart

| Status | Error Code | When |
|--------|------------|------|
| 400 | InvalidRequest | Missing or invalid partNumber |
| 404 | NoSuchUpload | Upload ID does not exist or bucket/key mismatch |
| 500 | InternalError | Decryption, encryption, or backend failure |

**Headers on success:**
- `ETag`

#### CompleteMultipartUpload

| Status | Error Code | When |
|--------|------------|------|
| 400 | MalformedXML | Invalid parts XML |
| 400 | InvalidRequest | No parts specified |
| 404 | NoSuchUpload | Upload ID does not exist or bucket/key mismatch |
| 500 | InternalError | HMAC storage, metadata update, or backend failure |

#### AbortMultipartUpload

| Status | Error Code | When |
|--------|------------|------|
| 404 | NoSuchUpload | Upload ID does not exist or bucket/key mismatch |
| 500 | InternalError | Backend failure |

#### ListParts

| Status | Error Code | When |
|--------|------------|------|
| 404 | NoSuchUpload | Upload ID does not exist or bucket/key mismatch |
| 500 | InternalError | Backend failure |

#### ListMultipartUploads

| Status | Error Code | When |
|--------|------------|------|
| 500 | InternalError | Backend failure |

### Bulk Operations

#### DeleteObjects (POST with ?delete)

| Status | Error Code | When |
|--------|------------|------|
| 400 | MalformedXML | Invalid delete request XML |
| 400 | MalformedXML | No objects specified for deletion |
| 500 | InternalError | Backend deletion failure |

## Backend Error Propagation

ARMOR's backend (B2/Cloudflare R2) may return errors that are wrapped in `InternalError` responses. Common backend errors:

| Backend Error | ARMOR Response |
|---------------|----------------|
| B2 auth failure | 500 InternalError |
| Network timeout | 500 InternalError |
| Storage full | 500 InternalError |
| Rate limited | 500 InternalError |

**Note:** Backend-specific error details are logged but not exposed to clients in error messages for security reasons.

## Pre-signed URL Errors

The `/share/*` endpoint (pre-signed URLs) uses plain text error responses, not XML:

| Status | Response | When |
|--------|----------|------|
| 400 | `Missing token` | No token in path |
| 400 | `Invalid token` | Token malformed or signature invalid |
| 403 | `Invalid link` | Cryptographic signature verification failed |
| 404 | `Object not found: [details]` | Object does not exist in storage |
| 410 | `Link expired` | Token expiration time passed |
| 500 | `Failed to [operation]` | Decryption or backend failure |

**Headers:** None specific (standard HTTP headers only)

## Health Endpoints

### /healthz (Liveness)

| Status | Response |
|--------|----------|
| 200 | `OK` |

### /readyz (Readiness)

| Status | Response |
|--------|----------|
| 200 | `Ready` |
| 503 | `Not ready - canary check failed` |
| 503 | `Not ready - manifest writer has never flushed` |
| 503 | `Not ready - manifest writer last flush X ago (threshold 60s)` |
| 503 | `Not ready - no health signal available` |

**Note:** Health endpoints return plain text, not XML, and do not include CORS headers.

## Compliance Notes

### S3 Compliance

✅ **Compliant:**
- XML error format matches S3 specification
- Standard S3 error codes used (NoSuchKey, AccessDenied, etc.)
- Proper HTTP status codes for error types

⚠️ **Partial Compliance:**
- CORS headers on errors differ from AWS (present in ARMOR, absent in AWS unless configured)
- Some error messages may be more verbose than AWS S3
- Backend errors wrapped as InternalError hide specific S3 error codes

❌ **Not Implemented:**
- `x-amz-request-id` header (request tracing)
- `x-amz-id-2` header (extended request ID)
- Specific S3 error codes for certain edge cases (e.g., BucketAlreadyExists, InvalidPartOrder)

### Recommendations

1. **CORS Headers:** Consider adding configuration to make CORS headers on errors match AWS behavior (only when explicitly configured)

2. **Request Tracing:** Add `x-amz-request-id` to all responses for debugging support

3. **Backend Errors:** Map specific backend errors to appropriate S3 error codes (e.g., 503 for rate limiting)

4. **Auth Errors:** Ensure all authentication error codes match AWS S3 exactly

## Testing

To verify error responses:

```bash
# 404 NoSuchKey
curl -i https://your-armor-server/bucket/nonexistent-key

# 403 AccessDenied (with bad credentials)
curl -i -H "Authorization: AWS4-HMAC-SHA256 ..." https://your-armor-server/bucket/key

# 400 InvalidRange
curl -i -H "Range: bytes=invalid" https://your-armor-server/bucket/key

# 405 MethodNotAllowed
curl -i -X PATCH https://your-armor-server/bucket/key

# 412 PreconditionFailed
curl -i -H "If-Match: nonexistent-etag" https://your-armor-server/bucket/key
```

## References

- AWS S3 Error Responses: https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html
- AWS S3 Error Codes: https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html#ErrorCodeList
- ARMOR Source Code: `/home/coding/ARMOR/internal/server/handlers/handlers.go` (writeError function)
- ARMOR Source Code: `/home/coding/ARMOR/internal/server/server.go` (wrapHandler middleware)
