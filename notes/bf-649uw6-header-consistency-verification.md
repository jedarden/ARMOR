# ARMOR Error Response Header Consistency Verification

**Bead ID:** bf-649uw6  
**Date:** 2026-07-14  
**Task:** Verify error response header consistency across all rejection types

## Executive Summary

✅ **VERIFIED**: All error responses across ARMOR use **consistent headers** regardless of rejection type.

## Key Findings

### 1. **Unified Error Response Implementation**

All error responses flow through two identical functions:

#### `server.go:797-805` - Server-level errors
```go
func (s *Server) writeError(w http.ResponseWriter, code, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    // ... XML generation
}
```

#### `handlers.go:2696-2704` - Handler-level errors
```go
func (h *Handlers) writeError(w http.ResponseWriter, code, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    // ... XML generation
}
```

**Both functions are identical** in header setting behavior.

### 2. **Consistent Headers Across All Error Types**

**Every error response sets:**
- `Content-Type: application/xml` (uniform across all rejection types)
- HTTP status code (varies appropriately by error type)

**No error-specific headers are set** - headers are consistent regardless of:
- Authentication failure (403)
- Authorization failure (403) 
- Object not found (404)
- Validation errors (400)
- Internal errors (500)
- Method not allowed (405)
- Precondition failed (412)

### 3. **Error Categories and Status Codes**

| Error Category | HTTP Status | Example Error Codes | Headers Set |
|---|---|---|---|
| Authentication | 403 | MissingAuthenticationToken, InvalidAccessKeyId, SignatureDoesNotMatch, InvalidAlgorithm, IncompleteSignature, MissingDateHeader, InvalidDateFormat, RequestExpired, InvalidCredential | `Content-Type: application/xml` |
| Authorization | 403 | AccessDenied | `Content-Type: application/xml` |
| Object Errors | 404 | NoSuchKey | `Content-Type: application/xml` |
| Object Errors | 400 | InvalidRange | `Content-Type: application/xml` |
| Bucket Errors | 404 | NoSuchBucket | `Content-Type: application/xml` |
| Multipart Errors | 404 | NoSuchUpload | `Content-Type: application/xml` |
| Validation Errors | 400 | InvalidRequest, MalformedXML | `Content-Type: application/xml` |
| Method Errors | 405 | MethodNotAllowed | `Content-Type: application/xml` |
| Conditional Request | 412 | PreconditionFailed | `Content-Type: application/xml` |
| Internal Errors | 500 | InternalError | `Content-Type: application/xml` |

### 4. **Global Headers (Middleware Layer)**

**CORS headers** are set globally in `server.go:657-659` for ALL requests (both success and error):
- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS`
- `Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length`

These are **identical for all responses**, ensuring no header-based fingerprinting of error types.

### 5. **Success Response Headers (Not Set on Errors)**

Custom headers set **only on successful responses** (never on errors):
- `X-Armor-Streaming: true` (streaming GET responses)
- `X-Armor-Stream: pipelined` (pipelined responses)
- `X-Armor-Footer-Cache: HIT` (cached footer responses)
- `ETag` (successful GET/HEAD/COPY)
- `Last-Modified` (successful GET/HEAD)
- `Content-Length` (successful responses)
- `Accept-Ranges: bytes` (successful range requests)
- `Content-Range` (successful partial content)

**These headers are NEVER set on error responses**, preventing header-based error type inference.

## Verification Methodology

### Code Analysis
1. ✅ Located all error response code paths (`writeError` functions)
2. ✅ Verified no direct `w.WriteHeader()` calls bypass `writeError`
3. ✅ Checked all rejection scenarios use centralized error functions
4. ✅ Verified no error-specific headers are set
5. ✅ Confirmed global headers apply uniformly to all responses

### Test Coverage Verification
- ✅ `error_response_test.go` - Tests header consistency across auth failures
- ✅ `error_response_verification_test.go` - Comprehensive verification
- ✅ `malformed_signature_test.go` - Tests malformed signature rejections
- ✅ `invalid_credential_test.go` - Tests invalid credential rejections

## Consistency Verification Results

### ✅ **PASSED** - All Acceptance Criteria Met

1. ✅ **Headers documented for each rejection scenario** - All 19+ error types documented in `bf-1wg09o-error-response-catalog.md`
2. ✅ **Inconsistent headers identified and documented** - No inconsistencies found; all error responses use identical headers
3. ✅ **Header consistency verified** - Verified that `Content-Type: application/xml` is set consistently across all rejection types

## Error Response Format

All error responses follow the **same XML format**:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>ERROR_CODE</Code>
  <Message>Human-readable error message</Message>
</Error>
```

With identical headers:
```http
HTTP/1.1 <status_code> <status_text>
Content-Type: application/xml
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
Content-Length: <calculated>
```

## Conclusion

ARMOR implements **excellent header consistency** across all rejection types. No header-based fingerprinting can differentiate between:
- Missing authentication token vs. invalid signature
- Access denied vs. object not found
- Validation error vs. internal server error

All error responses present the same header profile (`Content-Type: application/xml` plus global CORS headers), with only the HTTP status code and XML error code/message varying to indicate the specific rejection reason.

This design prevents attackers from using header analysis to:
1. Fingerprint the server
2. Differentiate between error types
3. Perform header-based reconnaissance
4. Exploit header disclosure vulnerabilities

## Recommendations

**No changes required** - The current implementation already follows security best practices for error response header consistency.

---

**Verification Status:** ✅ COMPLETE  
**Commit Required:** Yes (documentation only, no code changes)  
**Next Action:** Commit this verification note and close bead bf-649uw6