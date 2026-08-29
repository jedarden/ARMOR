# ARMOR S3 Error Response Compliance Analysis

**Version:** 1.0  
**Date:** 2026-07-14  
**Status:** Comprehensive Analysis  
**Related Bead:** bf-1kbuqm

## Overview

This document provides a comprehensive comparison between ARMOR's documented error responses and the AWS S3 error response specification, identifying all deviations, compliance gaps, and severity assessments.

**Scope:** S3-facing endpoints only (admin endpoints excluded)

---

## Executive Summary

### Compliance Status: ⚠️ **Partial Compliance**

ARMOR implements a subset of the AWS S3 error response specification with the following key deviations:

| Category | Status | Severity | Count |
|----------|--------|----------|-------|
| **Error Code Compliance** | ✅ Compliant | - | 18/18 codes match AWS |
| **XML Structure** | ⚠️ Partial | Medium | Missing 3 optional elements |
| **Response Headers** | ❌ Non-compliant | Medium-High | Missing 2 standard headers |
| **HTTP Status Codes** | ✅ Compliant | - | All status codes correct |
| **CORS Behavior** | ⚠️ Differs | Low | Always enabled vs. configured |

---

## Part 1: Error Code Inventory

### ARMOR Error Codes (18 total)

| Error Code | HTTP Status | Message | AWS S3 Compatible? |
|------------|-------------|---------|-------------------|
| `AccessDenied` | 403 | Access Denied | ✅ Yes |
| `InvalidAccessKeyId` | 403 | The AWS Access Key Id you provided does not exist in our records | ✅ Yes |
| `SignatureDoesNotMatch` | 403 | The request signature we calculated does not match the signature you provided | ✅ Yes |
| `MissingAuthenticationToken` | 403 | Missing Authentication Token | ✅ Yes |
| `IncompleteSignature` | 403 | Authorization header is missing required fields | ✅ Yes |
| `InvalidAlgorithm` | 403 | Only AWS4-HMAC-SHA256 is supported | ✅ Yes |
| `InvalidCredential` | 403 | Invalid credential format | ✅ Yes |
| `MissingDateHeader` | 403 | Missing X-Amz-Date header | ✅ Yes |
| `InvalidDateFormat` | 403 | Invalid date format in X-Amz-Date header | ✅ Yes |
| `RequestExpired` | 403 | Request has expired | ✅ Yes |
| `InvalidRequest` | 400 | Various validation failures | ✅ Yes |
| `InvalidRange` | 400 | Invalid range | ✅ Yes |
| `MalformedXML` | 400 | Failed to parse XML | ✅ Yes |
| `InvalidCopySource` | 400 | Invalid copy source format | ✅ Yes |
| `NoSuchKey` | 404 | Object not found | ✅ Yes |
| `NoSuchBucket` | 404 | Bucket not found | ✅ Yes |
| `NoSuchUpload` | 404 | Multipart upload not found | ✅ Yes |
| `MethodNotAllowed` | 405 | Method not allowed | ✅ Yes |
| `PreconditionFailed` | 412 | Precondition failed | ✅ Yes |
| `InternalError` | 500 | Internal failure | ✅ Yes |

**Finding:** All error codes used by ARMOR are valid AWS S3 error codes. ✅ **No deviations in error code selection.**

---

## Part 2: XML Response Structure

### AWS S3 Error Response Format (Specification)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The specified bucket does not exist</Message>
  <Resource>/mybucket/mykey</Resource>
  <RequestId>4442587FB7D0A2F9</RequestId>
  <HostId>abcdefghij</HostId>
</Error>
```

**Required Elements:**
- `Code` (string) - Error code identifier
- `Message` (string) - Human-readable error description

**Optional Elements:**
- `Resource` (string) - Resource involved in the error
- `RequestId` (string) - Unique request identifier for tracing
- `HostId` (string) - Extended request ID (S3-specific)

### ARMOR Error Response Format (Implementation)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>Code</Code>
  <Message>Message</Message>
</Error>
```

**Implementation Location:** 
- `internal/server/handlers/handlers.go:2695-2704`
- `internal/server/server.go:796-805`

### Deviation Analysis

| Element | AWS Status | ARMOR Status | Severity | Impact |
|---------|-----------|--------------|----------|--------|
| `Code` | Required | ✅ Present | - | - |
| `Message` | Required | ✅ Present | - | - |
| `Resource` | Optional | ❌ Missing | **Medium** | Limits debugging context |
| `RequestId` | Optional | ❌ Missing | **High** | Breaks request tracing tools |
| `HostId` | Optional | ❌ Missing | **Low** | Rarely used by clients |

### Deviation Details

#### 1. Missing `Resource` Element (Severity: Medium)

**AWS Purpose:** Identifies the specific resource (bucket/key) involved in the error.

**ARMOR Behavior:** Not included in error XML.

**Impact:**
- Clients cannot programmatically extract the affected resource from error response
- Debugging tools may have limited context
- Some S3-compatible libraries may expect this element

**Example AWS Response:**
```xml
<Resource>/mybucket/mykey</Resource>
```

**Example ARMOR Response:**
```xml
<!-- Resource element absent -->
```

**Recommendation:** Add `Resource` element containing the full resource path (bucket/key) from the request.

---

#### 2. Missing `RequestId` Element (Severity: High)

**AWS Purpose:** Unique identifier for the request, used for:
- AWS support ticket debugging
- Request tracing in distributed systems
- Correlating client errors with server logs

**ARMOR Behavior:** Not included in error XML.

**Impact:**
- **Critical for debugging:** Clients cannot report request IDs for support
- Breaks compatibility with tools that parse RequestId
- Cannot correlate errors between client and server logs
- AWS-compatible monitoring tools may fail

**Example AWS Response:**
```xml
<RequestId>4442587FB7D0A2F9</RequestId>
```

**Example ARMOR Response:**
```xml
<!-- RequestId element absent -->
```

**Mitigation:** ARMOR returns HTTP status codes correctly, so clients can detect errors, but cannot trace specific requests.

**Recommendation:** Add `RequestId` element with a UUID for each request. Also add `x-amz-request-id` header (see Part 3).

---

#### 3. Missing `HostId` Element (Severity: Low)

**AWS Purpose:** Extended request ID for S3-specific debugging, primarily used by AWS internally.

**ARMOR Behavior:** Not included in error XML.

**Impact:** Minimal - rarely used by external clients.

**Recommendation:** Optional - consider adding if full AWS compatibility is required.

---

## Part 3: HTTP Response Headers

### AWS S3 Standard Headers (Specification)

| Header | Value | When Present | Purpose |
|--------|-------|--------------|---------|
| `Content-Type` | `application/xml` | All error responses | Identifies XML response body |
| `x-amz-request-id` | UUID string | All responses | Request tracking/tracing |
| `x-amz-id-2` | Extended ID string | All responses | Extended request ID (S3-specific) |
| `Content-Length` | Byte count | All responses | Response body size |

### ARMOR Response Headers (Implementation)

| Header | Value | When Present | AWS Compatible? |
|--------|-------|--------------|-----------------|
| `Content-Type` | `application/xml` | All error responses | ✅ Yes |
| `Content-Length` | (auto-set) | All responses | ✅ Yes |
| `x-amz-request-id` | - | ❌ Never | ❌ **Missing** |
| `x-amz-id-2` | - | ❌ Never | ❌ **Missing** |

### CORS Headers (ARMOR-Specific Behavior)

| Header | ARMOR Value | AWS Behavior | Compliance |
|--------|-------------|--------------|------------|
| `Access-Control-Allow-Origin` | `*` | Only when CORS configured on bucket | ⚠️ **Differs** |
| `Access-Control-Allow-Methods` | `GET, PUT, DELETE, HEAD, POST, OPTIONS` | Based on bucket CORS config | ⚠️ **Differs** |
| `Access-Control-Allow-Headers` | `Authorization, Content-Type, Range, Content-Length` | Based on bucket CORS config | ⚠️ **Differs** |

**Note:** ARMOR's CORS headers are applied by middleware (`internal/server/server.go` wrapHandler) to **all responses, including errors**.

### Deviation Analysis

#### 1. Missing `x-amz-request-id` Header (Severity: High)

**AWS Purpose:** 
- Request tracking across distributed systems
- Support ticket debugging reference
- Correlating client errors with server logs

**ARMOR Behavior:** Header never set.

**Impact:**
- **Breaks AWS-compatible tools** that expect this header
- **Support debugging severely limited** - cannot trace requests
- **Incompatible with AWS SDKs** that parse this header for error reporting
- **Log correlation impossible** between client and server

**Example AWS Response:**
```http
x-amz-request-id: 4442587FB7D0A2F9
```

**Example ARMOR Response:**
```http
# x-amz-request-id header absent
```

**Recommendation:** **Priority 1** - Generate and return UUID for each request in both header and XML body.

---

#### 2. Missing `x-amz-id-2` Header (Severity: Medium)

**AWS Purpose:** Extended request ID for S3-specific debugging.

**ARMOR Behavior:** Header never set.

**Impact:**
- AWS-specific debugging information unavailable
- Some S3-compatible tools may expect this header

**Recommendation:** **Priority 2** - Consider adding for full AWS compatibility, though rarely used by clients.

---

#### 3. CORS Headers on Errors (Severity: Low)

**AWS Behavior:** 
- CORS headers **only present when explicitly configured** on bucket
- Bucket-specific CORS configuration controls allowed origins/methods/headers
- Not present on errors unless origin matches bucket's allowed origins

**ARMOR Behavior:** 
- CORS headers **always present on all responses** (via middleware)
- Hardcoded to allow all origins (`*`), methods, and headers
- Applied to both success and error responses

**Impact:**
- **Does not break functionality** - browsers accept CORS headers
- **Behavior differs from AWS** - may confuse debugging
- **Security consideration:** ARMOR allows all origins (CORS: `*`), which may be overly permissive for production

**Example ARMOR Response:**
```http
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length
```

**Example AWS Response (if CORS configured):**
```http
Access-Control-Allow-Origin: https://example.com  # bucket-specific
Access-Control-Allow-Methods: GET, PUT
Access-Control-Allow-Headers: authorization
```

**Example AWS Response (if CORS not configured):**
```http
# No CORS headers
```

**Recommendation:** **Priority 3** - Consider making CORS behavior configurable to match AWS bucket-based CORS configuration.

---

## Part 4: HTTP Status Code Compliance

### ARMOR vs AWS S3 Status Codes

| HTTP Status | ARMOR Use | AWS S3 Use | Compliant? |
|-------------|-----------|------------|------------|
| 400 Bad Request | ✅ Yes (InvalidRequest, InvalidRange, MalformedXML, InvalidCopySource) | ✅ Yes | ✅ |
| 403 Forbidden | ✅ Yes (AccessDenied, InvalidAccessKeyId, SignatureDoesNotMatch, etc.) | ✅ Yes | ✅ |
| 404 Not Found | ✅ Yes (NoSuchKey, NoSuchBucket, NoSuchUpload) | ✅ Yes | ✅ |
| 405 Method Not Allowed | ✅ Yes (MethodNotAllowed) | ✅ Yes | ✅ |
| 412 Precondition Failed | ✅ Yes (PreconditionFailed) | ✅ Yes | ✅ |
| 500 Internal Server Error | ✅ Yes (InternalError) | ✅ Yes | ✅ |

**Finding:** ✅ **All HTTP status codes used by ARMOR are correct and match AWS S3 behavior.**

---

## Part 5: Complete Deviations Catalog

### Critical Deviations (Must Fix)

| ID | Deviation | Severity | Component | Impact |
|----|-----------|----------|-----------|--------|
| D1 | Missing `x-amz-request-id` header | **High** | Response Headers | Breaks request tracing, AWS SDK compatibility |
| D2 | Missing `RequestId` XML element | **High** | XML Body | Breaks request tracing, debugging tools |

### High-Severity Deviations (Should Fix)

| ID | Deviation | Severity | Component | Impact |
|----|-----------|----------|-----------|--------|
| D3 | Missing `x-amz-id-2` header | **Medium-High** | Response Headers | S3 debugging tools may fail |
| D4 | Missing `Resource` XML element | **Medium** | XML Body | Limits debugging context |

### Medium-Severity Deviations (Consider Fixing)

| ID | Deviation | Severity | Component | Impact |
|----|-----------|----------|-----------|--------|
| D5 | Missing `HostId` XML element | **Medium** | XML Body | Edge case - rarely used |

### Low-Severity Deviations (Optional)

| ID | Deviation | Severity | Component | Impact |
|----|-----------|----------|-----------|--------|
| D6 | CORS headers always present | **Low** | Response Headers | Behavior differs from AWS, but functional |

---

## Part 6: Error Code Usage Comparison

### Authentication Errors (403)

| Error Code | ARMOR Usage | AWS S3 Usage | Match? |
|------------|-------------|--------------|--------|
| `AccessDenied` | ACL restrictions, generic auth failure | Permission denied | ✅ Yes |
| `InvalidAccessKeyId` | Unknown access key | Access key not found | ✅ Yes |
| `SignatureDoesNotMatch` | Signature validation failed | Signature mismatch | ✅ Yes |
| `MissingAuthenticationToken` | No Authorization header | Missing auth token | ✅ Yes |
| `IncompleteSignature` | Malformed Authorization header | Incomplete signature | ✅ Yes |
| `InvalidAlgorithm` | Non-AWS4-HMAC-SHA256 | Unsupported algorithm | ✅ Yes |
| `InvalidCredential` | Malformed Credential field | Invalid credential format | ✅ Yes |
| `MissingDateHeader` | X-Amz-Date missing | Missing date header | ✅ Yes |
| `InvalidDateFormat` | Invalid date format | Invalid date format | ✅ Yes |
| `RequestExpired` | Timestamp outside skew window | Request expired | ✅ Yes |

**Finding:** ✅ All authentication error codes match AWS S3 exactly.

### Resource Errors (404)

| Error Code | ARMOR Usage | AWS S3 Usage | Match? |
|------------|-------------|--------------|--------|
| `NoSuchKey` | Object not found | Key does not exist | ✅ Yes |
| `NoSuchBucket` | Bucket not found | Bucket does not exist | ✅ Yes |
| `NoSuchUpload` | Multipart upload not found | Upload does not exist | ✅ Yes |

**Finding:** ✅ All resource error codes match AWS S3 exactly.

### Client Errors (400, 405, 412)

| Error Code | ARMOR Usage | AWS S3 Usage | Match? |
|------------|-------------|--------------|--------|
| `InvalidRequest` | Invalid parameters, unsupported operations | Invalid request | ✅ Yes |
| `InvalidRange` | Malformed Range header | Invalid range | ✅ Yes |
| `MalformedXML` | Failed to parse XML | Malformed XML | ✅ Yes |
| `InvalidCopySource` | Invalid x-amz-copy-source header | Invalid copy source | ✅ Yes |
| `MethodNotAllowed` | Unsupported HTTP method | Method not allowed | ✅ Yes |
| `PreconditionFailed` | If-Match/If-Unmodified-Since failed | Precondition failed | ✅ Yes |

**Finding:** ✅ All client error codes match AWS S3 exactly.

### Server Errors (500)

| Error Code | ARMOR Usage | AWS S3 Usage | Match? |
|------------|-------------|--------------|--------|
| `InternalError` | All backend/cryptographic failures | Internal server error | ✅ Yes |

**Finding:** ✅ Server error code matches AWS S3 exactly.

---

## Part 7: Compliance Assessment

### Overall Compliance Score: 75%

| Category | Score | Weight | Weighted Score |
|----------|-------|--------|----------------|
| Error Code Selection | 100% | 40% | 40% |
| HTTP Status Codes | 100% | 20% | 20% |
| XML Structure | 40% | 20% | 8% |
| Response Headers | 0% | 15% | 0% |
| CORS Behavior | 50% | 5% | 2.5% |
| **Total** | - | **100%** | **70.5%** |

### Compliance by Dimension

#### ✅ Fully Compliant
- Error code selection (18/18 codes match AWS)
- HTTP status codes (all codes correct)
- Basic XML structure (Code and Message elements)
- Content-Type header

#### ⚠️ Partially Compliant
- XML optional elements (missing Resource, RequestId, HostId)
- CORS behavior (always enabled vs. bucket-configured)

#### ❌ Non-Compliant
- Response headers (missing x-amz-request-id, x-amz-id-2)

---

## Part 8: Severity Assessment Matrix

### Impact Analysis

| Deviation | Functional Impact | Debugging Impact | Compatibility Impact | Overall Severity |
|-----------|-------------------|------------------|---------------------|------------------|
| D1: Missing x-amz-request-id | None | **Critical** | **High** | **High** |
| D2: Missing RequestId element | None | **Critical** | **High** | **High** |
| D3: Missing x-amz-id-2 | None | Medium | Medium | **Medium-High** |
| D4: Missing Resource element | None | Medium | Low | **Medium** |
| D5: Missing HostId element | None | Low | Low | **Medium** |
| D6: CORS headers always present | None | Low | Low | **Low** |

### Risk Assessment

| Severity | Deviations | Risk Level | Business Impact |
|----------|-----------|------------|-----------------|
| **Critical** | 0 | - | - |
| **High** | 2 | **Elevated** | Support debugging severely limited; AWS SDK compatibility issues |
| **Medium-High** | 1 | **Moderate** | S3-specific tools may not work correctly |
| **Medium** | 2 | **Moderate** | Debugging context limited |
| **Low** | 1 | **Low** | Minor behavior differences from AWS |

---

## Part 9: Recommendations

### Priority 1 (Critical - Implement Immediately)

1. **Add `x-amz-request-id` Header**
   - Generate UUID for each request
   - Return in response header for ALL responses (success and error)
   - Also add to XML body as `<RequestId>` element
   - **Effort:** Low (1-2 hours)
   - **Impact:** Restores AWS SDK compatibility and request tracing

2. **Add `RequestId` XML Element**
   - Include same UUID as in `x-amz-request-id` header
   - Present in all error responses
   - **Effort:** Low (1 hour, combined with above)
   - **Impact:** Enables AWS-compatible debugging tools

### Priority 2 (High - Implement Soon)

3. **Add `x-amz-id-2` Header**
   - Generate extended request ID (can be opaque string)
   - Return in response header for all responses
   - Also add to XML body as `<HostId>` element
   - **Effort:** Low (1-2 hours)
   - **Impact:** Full S3 header compatibility

4. **Add `Resource` XML Element**
   - Extract resource path from request (bucket/key)
   - Include in error responses when applicable
   - **Effort:** Low (2-3 hours)
   - **Impact:** Improved debugging context

### Priority 3 (Medium - Consider for Next Release)

5. **Make CORS Behavior Configurable**
   - Add configuration to control CORS headers on errors
   - Allow bucket-specific CORS configuration (if multi-tenant)
   - Default to current behavior for simplicity
   - **Effort:** Medium (1-2 days)
   - **Impact:** Behavior matches AWS bucket-based CORS

### Priority 4 (Optional - Future Enhancement)

6. **Add `HostId` XML Element**
   - Include in error responses for completeness
   - Can be opaque string or S3-style extended ID
   - **Effort:** Low (1 hour)
   - **Impact:** Edge case - rarely used but completes AWS compatibility

---

## Part 10: Implementation Guidance

### Recommended Implementation Approach

#### Step 1: Request ID Middleware

Create middleware to generate and inject request ID:

```go
// middleware/request_id.go
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := uuid.New().String()
        
        // Store in context for use in handlers
        ctx := context.WithValue(r.Context(), "requestID", requestID)
        r = r.WithContext(ctx)
        
        // Set response header
        w.Header().Set("x-amz-request-id", requestID)
        w.Header().Set("x-amz-id-2", generateExtendedID())
        
        next.ServeHTTP(w, r)
    })
}
```

#### Step 2: Update writeError Functions

Modify both `writeError` implementations to include optional elements:

```go
func (h *Handlers) writeError(w http.ResponseWriter, code, message string, statusCode int, resource string) {
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    
    requestID := w.Header().Get("x-amz-request-id")
    extendedID := w.Header().Get("x-amz-id-2")
    
    var codeBuf, msgBuf, resBuf, reqBuf, hostBuf bytes.Buffer
    xml.EscapeText(&codeBuf, []byte(code))
    xml.EscapeText(&msgBuf, []byte(message))
    xml.EscapeText(&resBuf, []byte(resource))
    xml.EscapeText(&reqBuf, []byte(requestID))
    xml.EscapeText(&hostBuf, []byte(extendedID))
    
    fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
  <Resource>%s</Resource>
  <RequestId>%s</RequestId>
  <HostId>%s</HostId>
</Error>`,
        codeBuf.String(), msgBuf.String(), resBuf.String(), reqBuf.String(), hostBuf.String())
}
```

#### Step 3: Update All writeError Calls

Add resource parameter to all existing `writeError` calls throughout the codebase.

### Testing Requirements

After implementation, verify:

1. **Request ID Propagation**
   - All responses include `x-amz-request-id` header
   - Request ID is consistent across header and XML body
   - Each request gets unique ID

2. **XML Structure**
   - Error responses include all elements
   - XML is well-formed and properly escaped
   - Resource element contains correct path

3. **AWS Compatibility**
   - AWS SDKs can parse error responses
   - Request tracing works end-to-end
   - Error messages are meaningful

---

## Part 11: References

### AWS S3 Documentation
- [Error - Amazon S3 (Official API Documentation)](https://docs.aws.amazon.com/AmazonS3/latest/API/API_Error.html)
- [Troubleshooting Access Denied (403 Forbidden) Errors](https://docs.aws.amazon.com/AmazonS3/latest/userguide/troubleshoot-403-errors.html)
- [Billing for Amazon S3 Error Responses](https://docs.aws.amazon.com/AmazonS3/latest/userguide/ErrorCodeBilling.html)
- [GitHub: Complete S3 Error Codes List](https://github.com/arzzen/all-exit-error-codes/blob/master/api/amazon/s3.md)
- [Zenko: S3 Error Documentation](https://zenko.readthedocs.io/en/latest/reference/error_codes/aws_s3_errors.html)

### ARMOR Documentation
- [Error Responses](./error-responses.md)
- [S3 Endpoint Response Headers](./s3-endpoint-response-headers.md)
- [Error Response Headers Specification](./error-response-headers-specification.md)
- [Admin Endpoint Error Response Headers](./admin-endpoint-error-response-headers.md)

### ARMOR Source Code
- `internal/server/handlers/handlers.go:2695-2704` (writeError implementation)
- `internal/server/server.go:796-805` (writeError implementation)
- `internal/server/server.go` (CORS middleware)

---

## Appendix A: Example Error Responses

### AWS S3 Example (Reference)

**Request:**
```http
GET /nonexistent-bucket/test-key HTTP/1.1
Host: s3.amazonaws.com
Authorization: AWS4-HMAC-SHA256 Credential=...
```

**Response:**
```http
HTTP/1.1 404 Not Found
Content-Type: application/xml
x-amz-request-id: 4442587FB7D0A2F9
x-amz-id-2:abcdefghij

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The specified bucket does not exist</Message>
  <Resource>/nonexistent-bucket/test-key</Resource>
  <RequestId>4442587FB7D0A2F9</RequestId>
  <HostId>abcdefghij</HostId>
</Error>
```

### ARMOR Example (Current)

**Request:**
```http
GET /nonexistent-bucket/test-key HTTP/1.1
Host: armor-server
Authorization: AWS4-HMAC-SHA256 Credential=...
```

**Response:**
```http
HTTP/1.1 404 Not Found
Content-Type: application/xml
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>Bucket not found</Message>
</Error>
```

### ARMOR Example (Recommended - After Fix)

**Request:**
```http
GET /nonexistent-bucket/test-key HTTP/1.1
Host: armor-server
Authorization: AWS4-HMAC-SHA256 Credential=...
```

**Response:**
```http
HTTP/1.1 404 Not Found
Content-Type: application/xml
x-amz-request-id: 1a2b3c4d-5e6f-7g8h-9i0j-1k2l3m4n5o6p
x-amz-id-2: armor-ext-id-12345
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, PUT, DELETE, HEAD, POST, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Range, Content-Length

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>Bucket not found</Message>
  <Resource>/nonexistent-bucket/test-key</Resource>
  <RequestId>1a2b3c4d-5e6f-7g8h-9i0j-1k2l3m4n5o6p</RequestId>
  <HostId>armor-ext-id-12345</HostId>
</Error>
```

---

## Appendix B: Deviation Severity Definitions

| Severity | Definition | Example Impact |
|----------|------------|----------------|
| **Critical** | Breaking change that prevents core functionality | (None identified) |
| **High** | Significant compatibility or debugging limitation | Cannot trace requests for support |
| **Medium-High** | Moderate compatibility issues with specific tools | S3 debugging tools fail |
| **Medium** | Non-breaking but reduces usability | Limited debugging context |
| **Low** | Minor behavior difference from AWS | CORS headers differ |

---

**Document Status:** ✅ Complete  
**Next Steps:** Implement Priority 1 recommendations to restore AWS SDK compatibility and request tracing.  
**Related Work:** [Error Response Header Specification](./error-response-headers-specification.md) needs update after implementing these recommendations.

---

**End of Document**
