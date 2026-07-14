# Error Response Header Consistency Analysis

**Date:** 2026-07-14  
**Bead:** bf-649uw6  
**Task:** Verify error response header consistency  
**Status:** ✅ **VERIFIED - Consistent**

---

## Executive Summary

ARMOR has **consistent error response headers** across all S3 API rejection types. All authentication and authorization errors return properly formatted S3 XML error responses with consistent headers.

### Key Findings

- ✅ **All S3 API errors use `writeError()` function** - ensures consistent header formatting
- ✅ **Content-Type: application/xml** for all S3 error responses
- ✅ **XML declaration included** in all error responses  
- ✅ **Error codes follow S3 standard** (e.g., `InvalidAccessKeyId`, `SignatureDoesNotMatch`)
- ✅ **Performance within SLA** - all error responses complete in <100µs locally

---

## Error Response Implementation

### Two Entry Points for Consistency

#### 1. Server-level Authentication Errors (`server.go:797-805`)

```go
func (s *Server) writeError(w http.ResponseWriter, code, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    var codeBuf, msgBuf bytes.Buffer
    xml.EscapeText(&codeBuf, []byte(code))
    xml.EscapeText(&msgBuf, []byte(message))
    fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<Error>\n  <Code>%s</Code>\n  <Message>%s</Message>\n</Error>",
        codeBuf.String(), msgBuf.String())
}
```

**Headers Set:**
- `Content-Type: application/xml`
- `Status: <statusCode>` (varies by error type)

#### 2. Handler-level Internal Errors (`handlers.go:2696-2704`)

```go
func (h *Handlers) writeError(w http.ResponseWriter, code, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    var codeBuf, msgBuf bytes.Buffer
    xml.EscapeText(&codeBuf, []byte(code))
    xml.EscapeText(&msgBuf, []byte(message))
    fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<Error>\n  <Code>%s</Code>\n  <Message>%s</Message>\n</Error>",
        codeBuf.String(), msgBuf.String())
}
```

**Headers Set:**
- `Content-Type: application/xml`
- `Status: <statusCode>` (varies by error type)

**Result:** Both functions are **identical** in implementation, ensuring consistency.

---

## Error Response Scenarios

### Authentication Failures (403 Forbidden)

All authentication errors are returned from `server.go` via the `wrapHandler` middleware:

| Scenario | Error Code | Status | Headers | Location |
|----------|------------|--------|---------|----------|
| Missing auth header | `MissingAuthenticationToken` | 403 | `Content-Type: application/xml` | server.go:672 |
| Invalid access key | `InvalidAccessKeyId` | 403 | `Content-Type: application/xml` | server.go:672 |
| Signature mismatch | `SignatureDoesNotMatch` | 403 | `Content-Type: application/xml` | server.go:672 |
| Malformed auth header | `InvalidAlgorithm` | 403 | `Content-Type: application/xml` | server.go:672 |
| Missing date header | `MissingDateHeader` | 403 | `Content-Type: application/xml` | server.go:672 |
| Expired request | `RequestExpired` | 403 | `Content-Type: application/xml` | server.go:672 |
| Empty signature | `IncompleteSignature` | 403 | `Content-Type: application/xml` | server.go:672 |
| Invalid signature characters | `SignatureDoesNotMatch` | 403 | `Content-Type: application/xml` | server.go:672 |

### Authorization Failures (ACL Denials)

| Scenario | Error Code | Status | Headers | Location |
|----------|------------|--------|---------|----------|
| ACL prefix mismatch | `AccessDenied` | 403 | `Content-Type: application/xml` | server.go:687, 876 |

### Internal Server Errors (500)

| Scenario | Error Code | Status | Headers | Location |
|----------|------------|--------|---------|----------|
| Encryption key failure | `InternalError` | 500 | `Content-Type: application/xml` | handlers.go:294 |
| DEK generation failure | `InternalError` | 500 | `Content-Type: application/xml` | handlers.go:301 |
| IV generation failure | `InternalError` | 500 | `Content-Type: application/xml` | handlers.go:307 |
| DEK wrapping failure | `InternalError` | 500 | `Content-Type: application/xml` | handlers.go:314 |
| Header creation failure | `InternalError` | 500 | `Content-Type: application/xml` | handlers.go:324 |
| Encryptor creation failure | `InternalError` | 500 | `Content-Type: application/xml` | handlers.go:337 |
| Upload failure | `InternalError` | 500 | `Content-Type: application/xml` | handlers.go:378 |

### Client Errors (400, 404, 405)

| Scenario | Error Code | Status | Headers | Location |
|----------|------------|--------|---------|----------|
| Unsupported POST operation | `InvalidRequest` | 400 | `Content-Type: application/xml` | handlers.go:256 |
| Method not allowed | `MethodNotAllowed` | 405 | `Content-Type: application/xml` | handlers.go:259 |
| Missing object | `NoSuchKey` | 404 | `Content-Type: application/xml` | handlers.go:various |

---

## Response Format

All S3 error responses follow this XML format:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MissingAuthenticationToken</Code>
  <Message>Missing Authentication Token</Message>
</Error>
```

**Response Characteristics:**
- XML declaration with encoding specification
- `<Error>` root element containing `<Code>` and `<Message>` child elements
- XML-escaped content to prevent injection attacks
- Consistent indentation (2-space for child elements)

---

## Performance Characteristics

From `error_response_verification_test.go`:

```
Error response performance statistics:
  Total scenarios: 8
  Average response time: 7.831µs
  Min response time: 3.345µs
  Max response time: 14.809µs
  All responses under 100ms: true
```

**Performance is well within SLA:**
- Maximum observed: **14.809µs** (0.0148ms)
- SLA threshold: **100ms**
- **~6,750x headroom** below threshold

---

## Error Code Reference

| Error Code | Scenario | HTTP Status | Source |
|------------|----------|-------------|--------|
| `MissingAuthenticationToken` | Authorization header missing | 403 | auth.go:340 |
| `InvalidAccessKeyId` | Access key not found | 403 | auth.go:344 |
| `SignatureDoesNotMatch` | Calculated signature ≠ provided | 403 | auth.go:348 |
| `RequestExpired` | Request timestamp outside ±15min | 403 | auth.go:347 |
| `AccessDenied` | ACL prefix mismatch | 403 | auth.go:349 |
| `InvalidAlgorithm` | Non-AWS4-HMAC-SHA256 algorithm | 403 | auth.go:341 |
| `IncompleteSignature` | Missing required auth fields | 403 | auth.go:343 |
| `MissingDateHeader` | X-Amz-Date header missing | 403 | auth.go:345 |
| `InvalidDateFormat` | X-Amz-Date not ISO8601 | 403 | auth.go:346 |
| `InvalidCredential` | Malformed credential string | 403 | auth.go:342 |
| `InternalError` | Server-side encryption failure | 500 | handlers.go:various |
| `InvalidRequest` | Unsupported operation | 400 | handlers.go:256 |
| `MethodNotAllowed` | HTTP method not supported | 405 | handlers.go:259 |
| `NoSuchKey` | Object not found | 404 | handlers.go:various |

---

## Consistency Verification

### Test Coverage

Two comprehensive tests verify consistency:

1. **`TestErrorResponseHeadersConsistency`** (`error_response_test.go`)
   - Verifies `Content-Type: application/xml` for all rejections
   - Validates non-empty response body
   - Checks XML declaration presence

2. **`TestComprehensiveErrorVerification`** (`error_response_verification_test.go`)
   - Tests 8 distinct rejection scenarios
   - Measures response time performance
   - Verifies meaningful error messages
   - Confirms consistent headers across all scenarios

### Test Results

```bash
=== RUN   TestErrorResponseHeadersConsistency
--- PASS: TestErrorResponseHeadersConsistency (0.00s)
    --- PASS: TestErrorResponseHeadersConsistency/Missing_auth_header (0.00s)
    --- PASS: TestErrorResponseHeadersConsistency/Invalid_access_key (0.00s)
    --- PASS: TestErrorResponseHeadersConsistency/Malformed_auth_header (0.00s)
    --- PASS: TestErrorResponseHeadersConsistency/Missing_date_header (0.00s)

=== RUN   TestComprehensiveErrorVerification
--- PASS: TestComprehensiveErrorVerification (0.00s)
    --- PASS: TestComprehensiveErrorVerification/All_responses_have_consistent_Content-Type_headers
    --- PASS: TestComprehensiveErrorVerification/All_responses_complete_within_performance_threshold
```

---

## Non-S3 Endpoint Headers

Non-S3 endpoints (health checks, admin API, dashboard) use `http.Error()` instead of `writeError()` and therefore have **different headers**:

### Health Check Endpoints

| Endpoint | Error Response | Headers |
|----------|---------------|---------|
| `/healthz` | `OK` (plain text) | `Status: 200` |
| `/readyz` (unhealthy) | Error message (plain text) | `Status: 503` |

### Admin API Errors

| Endpoint | Error Response | Headers |
|----------|---------------|---------|
| `/admin/key/verify` (invalid method) | `Method not allowed` (plain text) | `Status: 405` |
| `/admin/key/rotate` (bad request) | JSON error | `Status: 400` |
| `/admin/presign` (auth failure) | XML S3 error | `Content-Type: application/xml` |

**Note:** Admin endpoints that require S3 authentication (like `/admin/presign`) use `writeError()` and return S3 XML errors.

### Dashboard Errors

Dashboard endpoints use `http.Error()` for most failures, returning plain text or JSON responses depending on the endpoint.

**This is intentional:** These are administrative/debugging endpoints, not S3 API operations.

---

## Consistency Issues Found

### ✅ None - All S3 API Errors Are Consistent

After comprehensive analysis:

1. ✅ **All S3 API errors** use `writeError()` function (identical implementation in both `server.go` and `handlers.go`)
2. ✅ **All S3 errors** return `Content-Type: application/xml`
3. ✅ **All S3 errors** include XML declaration
4. ✅ **All S3 errors** follow S3 standard error codes
5. ✅ **Performance is consistent** across all rejection types
6. ✅ **Response format is uniform** (escaped XML, consistent structure)

### Design Decision: Non-S3 Endpoints

Non-S3 endpoints (health, admin, dashboard) intentionally use different response formats:
- **Health checks:** Plain text (for curl/monitoring simplicity)
- **Admin API:** Plain text or JSON (depending on endpoint)
- **Dashboard:** HTML/JSON (web UI focus)

This is **not an inconsistency** - it's a **design choice** to match HTTP/REST conventions for non-S3 operations.

---

## Recommendations

### Current State: ✅ No Changes Required

The ARMOR error response system is **well-designed and consistent**:

1. **Centralized error formatting:** Both `writeError()` implementations are identical
2. **Comprehensive testing:** Test coverage verifies consistency and performance
3. **Clear separation:** S3 API errors use XML; non-S3 endpoints use appropriate formats
4. **Performance excellence:** All error responses complete in microseconds

### Future Considerations

If extending the error response system:

1. **Maintain single source of truth:** Consider extracting `writeError()` to a shared package
2. **Extend testing:** Add new rejection scenarios to `error_response_verification_test.go`
3. **Document new error codes:** Update the error code reference table when adding codes
4. **Preserve consistency:** Any new S3 error must use `writeError()` and return XML

---

## Conclusion

**✅ VERIFIED:** Error response headers are **consistent across all S3 API rejection types**. 

- All S3 authentication/authorization errors return `Content-Type: application/xml`
- All include proper XML declaration and formatting
- Error codes follow S3 standards
- Performance is excellent (<15µs local, <100ms SLA)

**No inconsistencies found.** The system is production-ready.

---

**Verification Date:** 2026-07-14  
**Bead Completed:** bf-649uw6  
**Test Files:** `error_response_test.go`, `error_response_verification_test.go`
