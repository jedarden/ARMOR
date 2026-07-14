# ARMOR S3 Endpoint Response Headers

**Version:** 1.0  
**Date:** 2026-07-14  
**Status:** Active

## Overview

This document provides a focused inventory of all response headers returned by ARMOR's S3-facing endpoints, including both success and error responses. It serves as a quick reference for understanding ARMOR's S3 API compatibility.

**Related Documentation:**
- [Error Response Header Specification](./error-header-spec.md) - Comprehensive error response documentation
- [Error Responses](./error-responses.md) - Detailed error response documentation

## Response Header Categories

### 1. Standard HTTP Headers (All Responses)

| Header | Presence | Description |
|--------|----------|-------------|
| `Content-Type` | All responses | `application/xml` for errors; varies for success (object content type) |
| `Content-Length` | Success responses | Size of response body (not set for error responses) |
| `Content-Range` | Partial GET | Range response format: `bytes {start}-{end}/{total}` |
| `Content-Disposition` | Conditional | Set when object has `Content-Disposition` metadata |
| `Accept-Ranges` | HEAD/GET | Always `bytes` (indicates range request support) |

### 2. S3-Specific Headers (Success Responses)

| Header | Format | When Returned | Example |
|--------|--------|---------------|---------|
| `ETag` | `"{md5hash}"` | PUT success, GET/HEAD success, ListObjects entries | `"5d41402abc4b2a76b9719d911017c592"` |
| `Last-Modified` | HTTP TimeFormat | GET/HEAD object, ListObjects entries | `Mon, 14 Jul 2026 05:00:00 GMT` |

### 3. CORS Headers (HTTP 403 Only)

| Header | Value | When Returned |
|--------|-------|---------------|
| `Access-Control-Allow-Origin` | `*` | HTTP 403 authentication/authorization errors only |
| `Access-Control-Allow-Methods` | `GET, PUT, DELETE, HEAD, POST, OPTIONS` | HTTP 403 authentication/authorization errors only |
| `Access-Control-Allow-Headers` | `Authorization, Content-Type, Range, Content-Length` | HTTP 403 authentication/authorization errors only |

### 4. ARMOR-Specific Headers

| Header | Value | When Returned |
|--------|-------|---------------|
| `X-Armor-Streaming` | `true` | PUT object with streaming encryption (>10MB objects) |

## Error Response Headers by HTTP Status

### HTTP 400 Bad Request

**Headers:**
```
Content-Type: application/xml
```

**Error Codes:**
- `InvalidRequest` - Unsupported operations, invalid parameters
- `InvalidRange` - Invalid Range header format
- `MalformedXML` - Failed to parse request XML

**No CORS headers**

### HTTP 403 Forbidden

**Headers:**
```
Content-Type: application/xml
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
```

**Error Codes:**
- `MissingAuthenticationToken` - Authorization header missing
- `InvalidAccessKeyId` - Invalid access key
- `SignatureDoesNotMatch` - Signature validation failed
- `RequestExpired` - Request timestamp outside 15-minute window
- `InvalidAlgorithm` - Non-AWS4-HMAC-SHA256 algorithm
- `IncompleteSignature` - Authorization header missing fields
- `MissingDateHeader` - X-Amz-Date header missing
- `InvalidDateFormat` - Invalid date format
- `InvalidCredential` - Invalid credential format
- `AccessDenied` - ACL-based authorization rejection

**CORS headers present** (only error type with CORS)

### HTTP 404 Not Found

**Headers:**
```
Content-Type: application/xml
```

**Error Codes:**
- `NoSuchKey` - Object not found
- `NoSuchBucket` - Bucket not found
- `NoSuchUpload` - Multipart upload not found

**No CORS headers**

### HTTP 405 Method Not Allowed

**Headers:**
```
Content-Type: application/xml
```

**Error Codes:**
- `MethodNotAllowed` - HTTP method not supported

**No CORS headers**

### HTTP 412 Precondition Failed

**Headers:**
```
Content-Type: application/xml
```

**Error Codes:**
- `PreconditionFailed` - If-Match/If-Unmodified-Since condition failed

**No CORS headers**

### HTTP 500 Internal Server Error

**Headers:**
```
Content-Type: application/xml
```

**Error Codes:**
- `InternalError` - Backend failures, encryption/decryption errors

**No CORS headers**

### HTTP 503 Service Unavailable

**Headers:**
```
Content-Type: application/xml
```

**Error Codes:**
- `ServiceUnavailable` - Health check failures

**No CORS headers**

## Success Response Headers by Operation

### GET Object (200)

**Headers:**
```
Content-Type: {object-content-type}
Content-Length: {object-size}
ETag: "{etag-md5}"
Last-Modified: {http-timeformat}
Accept-Ranges: bytes
Content-Disposition: {disposition}  (if present in metadata)
Content-Range: bytes {start}-{end}/{total}  (for Range requests)
```

**Conditional Request Responses (304):**
- Returns `304 Not Modified` with no body when `If-Match`/`If-None-Match`/`If-Modified-Since`/`If-Unmodified-Since` conditions are met

### HEAD Object (200)

**Headers:**
```
Content-Type: {object-content-type}
Content-Length: {object-size}
ETag: "{etag-md5}"
Last-Modified: {http-timeformat}
Accept-Ranges: bytes
```

**No response body**

### PUT Object (200)

**Headers:**
```
ETag: "{etag-md5}"
X-Armor-Streaming: true  (for >10MB objects using streaming encryption)
```

**No response body**

### DELETE Object (204)

**Headers:**
```
(no headers)
```

**No response body** (HTTP 204 No Content)

### ListObjectsV2 (200)

**Headers:**
```
Content-Type: application/xml
```

**Response Body:** XML with `Contents` entries including `ETag` and `Last-Modified` for each object

### CopyObject (200)

**Headers:**
```
Content-Type: application/xml
```

**Response Body:** XML with destination object's `ETag` and `Last-Modified`

## Deviations from AWS S3

### Missing Standard S3 Response Headers

ARMOR does **not** return the following standard AWS S3 response headers:

| Missing Header | AWS Purpose | Impact |
|----------------|-------------|--------|
| `x-amz-request-id` | AWS request tracking ID | Medium - prevents request tracing in AWS-compatible tools |
| `x-amz-id-2` | AWS extended request ID (S3-specific) | Low - rarely used by clients |

**Recommendation:** Consider adding `x-amz-request-id` header with a UUID for each request to improve AWS compatibility and request tracing.

### Non-Standard Behaviors

1. **ETag for Streaming Encryption:**
   - **AWS S3:** ETag is always MD5 hash of object content (multi-part objects have different ETag format)
   - **ARMOR:** For >10MB objects using streaming encryption, ETag is SHA-256 truncated to 16 bytes
   - **Impact:** Low - ETag format is opaque to most clients, but may affect conditional requests
   - **Mitigation:** ARMOR sets `X-Armor-Streaming: true` header to indicate this case

2. **405 Method Not Allowed (Non-S3 Endpoints):**
   - **AWS S3:** Would return XML error response
   - **ARMOR:** Admin endpoints (e.g., `/admin/key/*`) return plain text "Method not allowed"
   - **Impact:** Low - admin endpoints are not S3-facing
   - **Status:** Documented in error-header-spec.md as Priority 2 remediation

## Quick Reference Tables

### Error Response Summary

| HTTP Status | Content-Type | CORS | Error Codes |
|-------------|--------------|------|-------------|
| 400 | application/xml | No | InvalidRequest, InvalidRange, MalformedXML |
| 403 | application/xml | Yes | 10 auth error codes (MissingAuthenticationToken, etc.) |
| 404 | application/xml | No | NoSuchKey, NoSuchBucket, NoSuchUpload |
| 405 | application/xml | No | MethodNotAllowed |
| 412 | application/xml | No | PreconditionFailed |
| 500 | application/xml | No | InternalError |
| 503 | application/xml | No | ServiceUnavailable |

### Success Response Header Summary

| Operation | ETag | Last-Modified | Accept-Ranges | Other |
|-----------|------|---------------|----------------|-------|
| GET Object | ✓ | ✓ | ✓ | Content-Type, Content-Length, Content-Range (if Range) |
| HEAD Object | ✓ | ✓ | ✓ | Content-Type, Content-Length |
| PUT Object | ✓ | ✗ | ✗ | X-Armor-Streaming (if >10MB) |
| DELETE Object | ✗ | ✗ | ✗ | (no headers, 204 response) |
| ListObjectsV2 | ✓ (in body) | ✓ (in body) | ✗ | Content-Type: application/xml |

## Implementation Files

- **Error Responses:** `internal/server/handlers/handlers.go:2695-2704` (`writeError`)
- **Success Responses:** 
  - GET Object: `internal/server/handlers/handlers.go:602` (`GetObject`)
  - HEAD Object: `internal/server/handlers/handlers.go:1133` (`HeadObject`)
  - PUT Object: `internal/server/handlers/handlers.go:267` (`PutObject`)
  - DELETE Object: `internal/server/handlers/handlers.go:1210` (`DeleteObject`)
  - ListObjectsV2: `internal/server/handlers/handlers.go:1470`
  - CopyObject: `internal/server/handlers/handlers.go:1238`
  - CORS: `internal/server/server.go:656-660`

## Testing

See [Error Response Header Specification](./error-header-spec.md#testing-requirements) for comprehensive testing documentation.

## References

### AWS S3 Documentation
- [S3 Response Headers](https://docs.aws.amazon.com/AmazonS3/latest/API/RESTCommonResponseHeaders.html)
- [S3 Error Responses](https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html)

### Internal Documentation
- [Error Response Header Specification](./error-header-spec.md)
- [Error Responses](./error-responses.md)
- [Authentication Rejection Headers](./auth-rejection-headers.md)

---

**End of Document**
