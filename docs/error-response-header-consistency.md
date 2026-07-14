# ARMOR Error Response Header Consistency Documentation

## Overview

This document verifies and documents the consistency of HTTP response headers across all rejection/error scenarios in ARMOR.

## Analysis Date

2026-07-14

## Error Response Implementation

### Response Writers

ARMOR has two `writeError` functions that handle error responses:

1. **`internal/server/server.go:writeError`** - Handles authentication/authorization errors
2. **`internal/server/handlers/handlers.go:writeError`** - Handles S3 operation errors

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

### Response Headers Set

**All error responses set exactly two headers:**

1. **`Content-Type: application/xml`** - Consistently set for all error types
2. **HTTP Status Code** - Varies by error category (see below)

### Response Format

All error responses follow the S3 XML error format:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>ERROR_CODE</Code>
  <Message>Error message description</Message>
</Error>
```

## Error Categories and HTTP Status Codes

### 1. Authentication/Authorization Errors (403 Forbidden)

| Error Code | Trigger | Message |
|------------|---------|---------|
| `MissingAuthenticationToken` | Authorization header is missing | "Missing Authentication Token" |
| `InvalidAccessKeyId` | Invalid access key provided | "The AWS Access Key Id you provided does not exist" |
| `SignatureDoesNotMatch` | Calculated signature doesn't match | "The request signature we calculated does not match the signature you provided" |
| `RequestExpired` | Request timestamp outside 15-minute window | "Request has expired" |
| `InvalidAlgorithm` | Non-AWS4-HMAC-SHA256 algorithm | "Only AWS4-HMAC-SHA256 is supported" |
| `IncompleteSignature` | Authorization header missing fields | "Authorization header is missing required fields" |
| `MissingDateHeader` | X-Amz-Date header missing | "Missing X-Amz-Date header" |
| `InvalidDateFormat` | X-Amz-Date format invalid | "Invalid date format in X-Amz-Date header" |
| `AccessDenied` | ACL-based access control rejection | "Access Denied" |

**Headers:** `Content-Type: application/xml`, `HTTP/1.1 403`

### 2. Client Input Errors (400 Bad Request / 405 Method Not Allowed)

| Error Code | HTTP Status | Trigger | Message |
|------------|-------------|---------|---------|
| `InvalidRequest` | 400 | Unsupported POST operation, missing/invalid parameters | Varies |
| `MethodNotAllowed` | 405 | HTTP method not supported | "Method {method} not allowed" |
| `InvalidRange` | 400 | Invalid Range header format | "Invalid range: {details}" |
| `MalformedXML` | 400 | Failed to parse request XML | "Failed to parse XML: {details}" |

**Headers:** `Content-Type: application/xml`, `HTTP/1.1 400` or `HTTP/1.1 405`

### 3. Resource Not Found Errors (404 Not Found)

| Error Code | Trigger | Message |
|------------|---------|---------|
| `NoSuchKey` | Object does not exist | "Object not found" |
| `NoSuchBucket` | Bucket does not exist | "Bucket not found" |
| `NoSuchUpload` | Multipart upload ID not found | "Multipart upload not found" |

**Headers:** `Content-Type: application/xml`, `HTTP/1.1 404`

### 4. Conditional Request Errors (412 Precondition Failed)

| Error Code | Trigger | Message |
|------------|---------|---------|
| `PreconditionFailed` | If-Match/If-Unmodified-Since condition failed | "Precondition failed" |

**Headers:** `Content-Type: application/xml`, `HTTP/1.1 412`

### 5. Internal Server Errors (500 Internal Server Error)

| Error Code | Trigger | Message |
|------------|---------|---------|
| `InternalError` | Backend failures, encryption/decryption errors | Varies: "Failed to {operation}: {error}" |

**Headers:** `Content-Type: application/xml`, `HTTP/1.1 500`

### 6. Service Unavailable (503 Service Unavailable)

| Error Code | Trigger | Message |
|------------|---------|---------|
| `ServiceUnavailable` | Health check failures | Varies |

**Headers:** `Content-Type: application/xml`, `HTTP/1.1 503`

## Consistency Verification

### ✅ Consistent Headers

All error responses across **all rejection types** consistently set:
- **`Content-Type: application/xml`** - Always set, no exceptions
- **HTTP status code** - Appropriate for error category

### ✅ Consistent Response Format

All error responses follow the same XML structure:
- XML declaration: `<?xml version="1.0" encoding="UTF-8"?>`
- Root element: `<Error>`
- Code element: `<Code>`
- Message element: `<Message>`

### ✅ XML Escaping

Both `writeError` implementations properly escape XML special characters in code and message fields, preventing injection attacks.

## Potential Minor Inconsistencies

### 1. Message Format Variation

**Status:** ⚠️ Minor inconsistency in message verbosity

Some error messages include detailed error context (e.g., `"Failed to encrypt: {error}"`), while others are static strings (e.g., `"Access Denied"`). This is **intentional and appropriate** - internal errors need diagnostics, while auth errors should be generic for security.

**Recommendation:** No change needed - this variation is security best practice.

### 2. Error Code Casing

**Status:** ✅ Consistent

All error codes use PascalCase (e.g., `SignatureDoesNotMatch`, `InternalError`), matching AWS S3 conventions.

### 3. Duplicate writeError Implementations

**Status:** ⚠️ Code duplication

Both `server.go` and `handlers.go` contain identical `writeError` functions. This creates maintenance risk if one is updated without the other.

**Recommendation:** Consider extracting to a shared utility function.

## Header Consistency Summary Table

| Error Category | HTTP Status | Content-Type | Other Headers |
|----------------|-------------|--------------|---------------|
| Authentication failures | 403 | application/xml | None |
| Authorization failures | 403 | application/xml | None |
| Input validation errors | 400 | application/xml | None |
| Method not allowed | 405 | application/xml | None |
| Resource not found | 404 | application/xml | None |
| Precondition failed | 412 | application/xml | None |
| Internal errors | 500 | application/xml | None |
| Service unavailable | 503 | application/xml | None |

## Verification Results

### Acceptance Criteria Status

| Criterion | Status | Notes |
|-----------|--------|-------|
| Headers documented for each rejection scenario | ✅ Complete | All error types documented with headers |
| Inconsistent headers identified and documented | ✅ Complete | No functional inconsistencies found; only minor code duplication noted |
| Header consistency verified | ✅ Verified | All responses use `Content-Type: application/xml` consistently |

### Performance

Error responses are consistently fast:
- No additional headers added beyond `Content-Type`
- Response time dominated by authentication verification, not header setting
- Single `WriteHeader` call per response (no multiple writes)

## Conclusion

ARMOR error response headers are **fully consistent** across all rejection types:

1. ✅ All error responses set exactly the same headers (`Content-Type: application/xml` plus HTTP status)
2. ✅ Header values are consistent and appropriate for each error category
3. ✅ Response format is standardized S3 XML error structure
4. ✅ No functional inconsistencies found

The only minor issue is code duplication between two identical `writeError` functions, which is a maintainability concern but not a functional inconsistency.

**Overall Assessment: PASS** - Error response header consistency is properly implemented across all rejection scenarios.
