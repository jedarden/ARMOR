# ARMOR Header Consistency Remediation Plan

**Version:** 1.0  
**Date:** 2026-07-14  
**Related Bead:** bf-1jjcp7  
**Status:** Active

## Executive Summary

This document provides a comprehensive, prioritized remediation plan for all identified header consistency issues across ARMOR's S3-facing and admin endpoints. The plan is organized by priority level (P0-P4), with effort estimates, implementation notes, and target timelines.

### Issue Statistics

| Category | Count | Severity |
|----------|-------|----------|
| P0 - Critical | 0 | - |
| P1 - High | 2 | Blocking AWS SDK compatibility |
| P2 - Medium-High | 2 | HTTP compliance violations |
| P3 - Medium | 5 | API consistency & maintainability |
| P4 - Low | 3 | Optional enhancements |
| Test Coverage Gaps | 7 | Production readiness |
| **Total** | **19** | - |

---

## Part 1: Quick Wins (Next Sprint - P1-P2)

### P1-001: Missing `x-amz-request-id` Header

**Priority:** P1 (High)  
**Issue ID:** H1  
**Component:** S3-facing endpoints  
**Affected:** All S3 operations (GET, PUT, DELETE, HEAD, POST, LIST)  
**Effort:** 2-3 hours

**Business Impact:**
- ❌ Breaks AWS SDK compatibility (SDKs expect this header)
- ❌ Prevents request tracing and debugging
- ❌ Makes support troubleshooting impossible
- ✅ **Quick win:** Simple middleware addition

**Implementation:**
```go
// internal/pkg/middleware/request_id.go
package middleware

import (
    "github.com/google/uuid"
    "net/http"
)

func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := uuid.New().String()
        w.Header().Set("x-amz-request-id", requestID)
        next.ServeHTTP(w, r)
    })
}
```

**Integration Points:**
```go
// internal/server/server.go
// Add to S3-facing route group
s3Routes.Use(middleware.RequestID)
```

**Testing:**
```go
func TestRequestIDHeader(t *testing.T) {
    // Verify x-amz-request-id present in all S3 responses
    // Verify UUID format
    // Verify uniqueness across requests
}
```

**Dependencies:** None

**Risks:** None (pure additive change)

**Target Release:** v0.2.0 (next sprint)

**Verification:**
```bash
# Manual verification
curl -I https://armor.example.com/bucket/key
# Should include: x-amz-request-id: <uuid>
```

---

### P1-002: Missing `RequestId` XML Element

**Priority:** P1 (High)  
**Issue ID:** H2  
**Component:** S3-facing endpoints  
**Affected:** All S3 error responses  
**Effort:** 1-2 hours

**Business Impact:**
- ❌ AWS-compatible tools cannot parse request IDs
- ❌ Debugging frameworks cannot extract trace information
- ✅ **Quick win:** Update error writer function

**Implementation:**
```go
// internal/server/handlers/handlers.go
func writeError(w http.ResponseWriter, code, message string, statusCode int, requestID string) {
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    var codeBuf, msgBuf, ridBuf bytes.Buffer
    xml.EscapeText(&codeBuf, []byte(code))
    xml.EscapeText(&msgBuf, []byte(message))
    xml.EscapeText(&ridBuf, []byte(requestID))
    fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
  <RequestId>%s</RequestId>
</Error>`, codeBuf.String(), msgBuf.String(), ridBuf.String())
}
```

**Context Propagation:**
```go
// Add request ID to context in middleware
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := uuid.New().String()
        w.Header().Set("x-amz-request-id", requestID)
        ctx := context.WithValue(r.Context(), "requestID", requestID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Extract in handlers
func (h *Handlers) GetObject(w http.ResponseWriter, r *http.Request) {
    requestID := r.Context().Value("requestID").(string)
    // Use in writeError calls
}
```

**Dependencies:** P1-001 (request ID must be generated first)

**Risks:** None (backwards compatible - adds optional XML element)

**Target Release:** v0.2.0 (next sprint)

---

### P2-001: Content-Type Mismatch (B2 Endpoints)

**Priority:** P2 (Medium-High)  
**Issue ID:** MH2  
**Component:** Admin endpoints  
**Affected:** `/admin/b2/keys`, `/admin/b2/keys/{id}`  
**Effort:** 2-3 hours

**Business Impact:**
- ❌ HTTP content negotiation violation
- ❌ Clients must parse JSON despite incorrect content-type
- ⚠️ Some clients may fail to parse response
- ✅ **Quick win:** Simple header fix

**Current Behavior:**
```http
HTTP/1.1 500 Internal Server Error
Content-Type: text/plain

{"error":"Failed to list keys: backend unavailable"}
```

**Target Behavior:**
```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{"error":"Failed to list keys: backend unavailable"}
```

**Implementation:**
```go
// internal/server/server.go:1246-1364
// Find B2 key list/create handlers
// Replace:
w.Header().Set("Content-Type", "text/plain")
// With:
w.Header().Set("Content-Type", "application/json")
```

**Dependencies:** None

**Risks:** Low (clients already parsing as JSON)

**Target Release:** v0.2.0 (next sprint)

**Testing:**
```bash
# Verify B2 error endpoints return correct content-type
curl -H "Authorization: Bearer invalid" https://armor.example.com/admin/b2/keys
# Should return: Content-Type: application/json
```

---

### P2-002: Missing `x-amz-id-2` Header

**Priority:** P2 (Medium-High)  
**Issue ID:** MH1  
**Component:** S3-facing endpoints  
**Affected:** All S3 responses  
**Effort:** 1-2 hours

**Business Impact:**
- ⚠️ S3-specific debugging tools may not work correctly
- ⚠️ Partial AWS S3 header compliance
- ✅ **Quick win:** Simple extended ID generation

**Implementation:**
```go
// internal/pkg/middleware/request_id.go (extend P1-001)
func generateExtendedID() string {
    // Generate opaque S3-style extended ID
    b := make([]byte, 16)
    rand.Read(b)
    return base64.StdEncoding.EncodeToString(b)
}

func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := uuid.New().String()
        extendedID := generateExtendedID()
        w.Header().Set("x-amz-request-id", requestID)
        w.Header().Set("x-amz-id-2", extendedID)
        
        ctx := r.Context()
        ctx = context.WithValue(ctx, "requestID", requestID)
        ctx = context.WithValue(ctx, "extendedID", extendedID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Dependencies:** P1-001

**Risks:** None (opaque string, no format requirements)

**Target Release:** v0.2.0 (next sprint)

---

## Part 2: Medium Priority (Next Quarter - P3)

### P3-001: Missing `Resource` XML Element

**Priority:** P3 (Medium)  
**Issue ID:** M1  
**Component:** S3-facing endpoints  
**Affected:** All S3 error responses  
**Effort:** 2-3 hours

**Business Impact:**
- ⚠️ Clients cannot programmatically extract affected resource
- ⚠️ Debugging context is limited
- ✅ Simple extraction from request path

**Implementation:**
```go
// internal/server/handlers/handlers.go
func writeError(w http.ResponseWriter, code, message, resource string, statusCode int, requestID string) {
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
  <Resource>%s</Resource>
  <RequestId>%s</RequestId>
</Error>`, code, message, resource, requestID)
}

// Extract resource in handlers
func (h *Handlers) GetObject(w http.ResponseWriter, r *http.Request) {
    bucket := mux.Vars(r)["bucket"]
    key := mux.Vars(r)["key"]
    resource := fmt.Sprintf("/%s/%s", bucket, key)
    // Use in error calls
}
```

**Dependencies:** P1-002

**Risks:** None (adds optional XML element)

**Target Release:** v0.3.0

---

### P3-002: Standardize Admin Endpoint Error Responses

**Priority:** P3 (Medium)  
**Issue ID:** M2  
**Component:** Admin endpoints  
**Affected:** `/admin/key/rotate`, `/admin/key/export`, `/admin/presign`, `/armor/canary`  
**Effort:** 2-3 days

**Business Impact:**
- ❌ Inconsistent API experience
- ❌ Clients must handle multiple response formats
- ⚠️ Harder to write generic error handlers
- ✅ Improves developer experience

**Current Inconsistencies:**
| Endpoint | Success | Error (400) | Error (403) | Error (405) |
|----------|---------|-------------|-------------|--------------|
| `/admin/key/rotate` | JSON | Plain text | - | Plain text |
| `/admin/key/export` | JSON | Plain text | - | Plain text |
| `/admin/presign` | JSON | Plain text | XML | Plain text |
| `/armor/canary` | JSON | - | - | Plain text |

**Target Behavior:**
All admin endpoints should return structured JSON errors:
```json
{
  "error": "error message",
  "code": "ErrorCode",
  "request_id": "uuid"
}
```

**Implementation:**
```go
// internal/pkg/errors/admin.go
package errors

type AdminError struct {
    Error      string `json:"error"`
    Code       string `json:"code"`
    RequestID  string `json:"request_id,omitempty"`
}

func WriteAdminError(w http.ResponseWriter, err AdminError, statusCode int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(err)
}
```

**Affected Handlers (update to use WriteAdminError):**
- `internal/server/server.go:rotateKey` (lines ~1300-1350)
- `internal/server/server.go:exportKey` (lines ~1350-1400)
- `internal/server/server.go:generatePresignedURL` (lines ~1400-1500)
- `internal/server/server.go:canary` (lines ~1500-1550)

**Dependencies:** None

**Risks:** Medium (breaking change for clients expecting plain text/XML)

**Mitigation:** Document in v0.3.0 release notes; consider deprecation period

**Target Release:** v0.3.0

---

### P3-003: Method Not Allowed Format Consistency

**Priority:** P3 (Medium)  
**Issue ID:** M3  
**Component:** All endpoints  
**Affected:** All 405 responses  
**Effort:** 1-2 days

**Business Impact:**
- ⚠️ Inconsistent with endpoint's normal response format
- ⚠️ API clients expecting JSON receive plain text
- ✅ Improves API consistency

**Implementation:**
```go
// internal/pkg/errors/admin.go (extend P3-002)
func WriteMethodNotAllowed(w http.ResponseWriter, allowed []string) {
    err := AdminError{
        Error:     "Method not allowed",
        Code:      "MethodNotAllowed",
        RequestID: getRequestID(w),
    }
    w.Header().Set("Allow", strings.Join(allowed, ", "))
    WriteAdminError(w, err, 405)
}
```

**Apply to all admin endpoints:**
```go
// Replace generic http.Error() calls
if r.Method != http.MethodGet {
    WriteMethodNotAllowed(w, []string{"GET"})
    return
}
```

**Dependencies:** P3-002 (use same admin error structure)

**Risks:** Low (405 is rare in production use)

**Target Release:** v0.3.0

---

### P3-004: Consolidate `writeError` Implementations

**Priority:** P3 (Medium)  
**Issue ID:** M4  
**Component:** Code quality  
**Affected:** `server.go`, `handlers.go`  
**Effort:** 3-4 hours

**Business Impact:**
- ⚠️ Maintenance burden (changes in two places)
- ⚠️ Risk of divergence over time
- ✅ Improves code maintainability

**Current State:**
- `internal/server/server.go:796-805` (auth errors)
- `internal/server/handlers/handlers.go:2695-2704` (S3 errors)

**Target Structure:**
```
internal/pkg/errors/
├── s3.go           # WriteS3Error (shared by auth + S3 handlers)
├── admin.go        # WriteAdminError (from P3-002)
└── errors.go       # Common types
```

**Implementation:**
```go
// internal/pkg/errors/s3.go
package errors

import (
    "bytes"
    "encoding/xml"
    "fmt"
    "net/http"
)

func WriteS3Error(w http.ResponseWriter, code, message, resource string, statusCode int, requestID string) {
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    
    var codeBuf, msgBuf, resBuf, ridBuf bytes.Buffer
    xml.EscapeText(&codeBuf, []byte(code))
    xml.EscapeText(&msgBuf, []byte(message))
    xml.EscapeText(&resBuf, []byte(resource))
    xml.EscapeText(&ridBuf, []byte(requestID))
    
    fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
  <Resource>%s</Resource>
  <RequestId>%s</RequestId>
</Error>`, codeBuf.String(), msgBuf.String(), resBuf.String(), ridBuf.String())
}
```

**Migration:**
1. Create shared package
2. Update all `writeError` calls to use `errors.WriteS3Error`
3. Remove duplicate implementations from `server.go` and `handlers.go`
4. Add tests for shared function

**Dependencies:** P1-002, P3-001 (must match new signature)

**Risks:** Low (internal refactoring, no API changes)

**Target Release:** v0.3.0

---

### P3-005: Add Comprehensive Test Coverage

**Priority:** P3 (Medium)  
**Issue IDs:** Gap 1, 3, 5, 7  
**Component:** Test suite  
**Affected:** Multiple endpoints  
**Effort:** 3-5 days

**Coverage Gaps to Address:**

1. **Pre-signed URL Error Scenarios** (Gap 1)
   - Expired token validation
   - Invalid signature verification
   - Edge cases in token parsing
   - Concurrent access to same pre-signed URL

2. **Concurrent Request Handling** (Gap 3)
   - Multiple simultaneous auth failures
   - Race conditions in signature verification
   - Concurrent access to rate-limited endpoints

3. **Large Payload Error Handling** (Gap 5)
   - Error responses during large file uploads
   - Multipart upload abortion scenarios
   - Range request errors on large files

4. **Complex Conditional Request Errors** (Gap 7)
   - Multiple conditional headers combined
   - Invalid conditional header values
   - If-Range header error scenarios

**Implementation Structure:**
```
internal/server/handlers/
├── handlers_presign_test.go     # Gap 1: Pre-signed URL tests
├── handlers_concurrency_test.go # Gap 3: Concurrent request tests
├── handlers_large_test.go       # Gap 5: Large payload tests
└── handlers_conditional_test.go # Gap 7: Conditional request tests
```

**Dependencies:** P3-001, P3-002 (error response changes)

**Target Release:** v0.3.0

---

## Part 3: Low Priority / Optional (Future - P4)

### P4-001: Configurable CORS Behavior

**Priority:** P4 (Low)  
**Issue ID:** L1  
**Component:** S3-facing endpoints  
**Affected:** All S3 responses  
**Effort:** 2-3 days

**Business Impact:**
- ✅ Behavior differs from AWS but doesn't break functionality
- ⚠️ CORS is always enabled (may be overly permissive)
- ✅ Optional enhancement

**Current Behavior:**
- CORS headers on all 403 authentication errors
- No bucket-based configuration

**Target Behavior:**
- Configurable CORS policy (environment variable or backend storage)
- Bucket-specific CORS rules (future enhancement)
- Match AWS S3 CORS configuration API

**Implementation:**
```go
// internal/pkg/middleware/cors.go
type CORSConfig struct {
    AllowedOrigins   []string
    AllowedMethods   []string
    AllowedHeaders   []string
    ExposeHeaders    []string
    MaxAge           int
    AllowCredentials bool
}

func CORS(config CORSConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")
            if origin == "" {
                next.ServeHTTP(w, r)
                return
            }
            
            // Check if origin is allowed
            if isOriginAllowed(origin, config.AllowedOrigins) {
                w.Header().Set("Access-Control-Allow-Origin", origin)
                // Set other CORS headers...
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

**Dependencies:** None

**Target Release:** v0.4.0 (future)

---

### P4-002: Add `HostId` XML Element

**Priority:** P4 (Low)  
**Issue ID:** L2  
**Component:** S3-facing endpoints  
**Affected:** All S3 error responses  
**Effort:** 1 hour

**Business Impact:**
- ✅ Rarely used by external clients
- ✅ Primarily for AWS internal debugging
- ✅ Minimal real-world impact

**Implementation:**
```go
// Extend P3-001 to include HostId
func WriteS3Error(w http.ResponseWriter, code, message, resource string, statusCode int, requestID, hostID string) {
    // Add HostId element to XML
    fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
  <Resource>%s</Resource>
  <RequestId>%s</RequestId>
  <HostId>%s</HostId>
</Error>`, code, message, resource, requestID, hostID)
}
```

**Dependencies:** P3-001

**Target Release:** v0.4.0 (future)

---

### P4-003: Header Ordering and Edge Case Tests

**Priority:** P4 (Low)  
**Issue IDs:** Gap 4, 6  
**Component:** Test suite  
**Affected:** All endpoints  
**Effort:** 1-2 days

**Coverage Gaps:**
- Header ordering consistency tests
- Chunked encoding error responses
- Trailer header handling on errors

**Target Release:** v0.4.0 (future)

---

## Part 4: Backend Error Propagation (High Priority)

### P2-003: Backend Error Propagation Tests

**Priority:** P2 (Medium-High)  
**Issue ID:** Gap 2  
**Component:** S3 operations  
**Affected:** All S3 operations  
**Effort:** 1-2 days

**Business Impact:**
- ❌ Backend error scenarios not tested
- ⚠️ May not map correctly to S3 error codes
- ⚠️ Production readiness gap

**Missing Coverage:**
- B2 rate limiting responses
- B2 network timeout handling
- Storage full errors
- Authentication failures to backend

**Implementation:**
```go
// internal/server/handlers/handlers_backend_test.go
func TestBackendErrorMapping(t *testing.T) {
    tests := []struct {
        name           string
        backendError   error
        expectedCode   string
        expectedStatus int
    }{
        {
            name:           "B2 rate limit",
            backendError:   &b2.Error{StatusCode: 429, Message: "rate limit"},
            expectedCode:   "SlowDown",
            expectedStatus: 503,
        },
        {
            name:           "B2 timeout",
            backendError:   context.DeadlineExceeded,
            expectedCode:   "ServiceUnavailable",
            expectedStatus: 503,
        },
        // Add more test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Mock backend to return error
            // Verify error mapping
            // Verify response headers
        })
    }
}
```

**Dependencies:** None

**Target Release:** v0.2.0 (next sprint)

---

## Part 5: Implementation Timeline

### Phase 1: Critical Compatibility (v0.2.0 - 2 weeks)

| Week | Tasks | Deliverables |
|------|-------|--------------|
| Week 1 | P1-001, P1-002, P2-001 | Request ID middleware, RequestId XML element, B2 content-type fix |
| Week 2 | P2-002, P2-003 | Extended ID header, Backend error propagation tests |

**Success Criteria:**
- ✅ All S3 responses include `x-amz-request-id` and `x-amz-id-2`
- ✅ All error responses include `<RequestId>` XML element
- ✅ B2 endpoints return correct `Content-Type: application/json`
- ✅ Backend error scenarios tested

---

### Phase 2: API Consistency (v0.3.0 - 6-8 weeks)

| Week | Tasks | Deliverables |
|------|-------|--------------|
| Week 1-2 | P3-001 | Resource XML element in error responses |
| Week 3-5 | P3-002 | Standardize admin endpoint error responses |
| Week 5-6 | P3-003 | Method Not Allowed consistency |
| Week 6-7 | P3-004 | Consolidate writeError implementations |
| Week 7-8 | P3-005 | Comprehensive test coverage |

**Success Criteria:**
- ✅ All admin endpoints return structured JSON errors
- ✅ Error handling code consolidated to shared package
- ✅ Test coverage for pre-signed URLs, concurrency, large payloads, conditional requests

---

### Phase 3: Optional Enhancements (v0.4.0 - Future)

| Tasks | Deliverables |
|-------|--------------|
| P4-001 | Configurable CORS behavior |
| P4-002 | HostId XML element |
| P4-003 | Header ordering and edge case tests |

---

## Part 6: Risk Assessment

### Technical Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking existing admin API clients (P3-002) | Medium | Medium | Document in release notes; provide migration guide |
| Performance impact from request ID generation | Low | Low | UUID generation is fast (<1μs) |
| Test suite runtime increase (P3-005) | Low | Low | Use table-driven tests; parallelize where safe |
| CORS behavior changes (P4-001) | Low | Low | Make opt-in via configuration |

### Operational Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Deployment coordination across services | Medium | Low | No service dependencies; can deploy independently |
| Client library updates required | Low | Medium | Engage with client library maintainers early |

---

## Part 7: Testing Strategy

### Unit Tests

Each remediation item includes unit tests:
- Request ID generation and propagation
- XML element serialization
- Error response format validation
- Header presence and correctness

### Integration Tests

- End-to-end S3 API compatibility tests
- Admin endpoint error response tests
- Backend failure simulation tests
- Concurrency stress tests

### Regression Tests

- Existing functionality must continue to work
- Performance tests to ensure no degradation
- Memory leak checks for request ID context propagation

### Manual Verification

```bash
# Quick verification script for v0.2.0
#!/bin/bash
echo "Testing ARMOR header fixes..."

# Test 1: x-amz-request-id header
echo "Test 1: Request ID header"
curl -I https://armor.example.com/bucket/key 2>/dev/null | grep "x-amz-request-id" && echo "✓ PASS" || echo "✗ FAIL"

# Test 2: RequestId XML element
echo "Test 2: RequestId XML element"
curl https://armor.example.com/nonexistent 2>/dev/null | grep "<RequestId>" && echo "✓ PASS" || echo "✗ FAIL"

# Test 3: B2 content-type
echo "Test 3: B2 content-type"
curl -H "Authorization: Bearer invalid" https://armor.example.com/admin/b2/keys 2>/dev/null -D - | grep "Content-Type: application/json" && echo "✓ PASS" || echo "✗ FAIL"

# Test 4: x-amz-id-2 header
echo "Test 4: Extended ID header"
curl -I https://armor.example.com/bucket/key 2>/dev/null | grep "x-amz-id-2" && echo "✓ PASS" || echo "✗ FAIL"
```

---

## Part 8: Success Metrics

### v0.2.0 Metrics

| Metric | Current | Target |
|--------|---------|--------|
| AWS SDK compatibility | ❌ No | ✅ Yes |
| Request tracing capability | ❌ No | ✅ Yes |
| Error response completeness | 40% | 60% |
| Test coverage (error scenarios) | 30% | 50% |

### v0.3.0 Metrics

| Metric | Target |
|--------|--------|
| Admin API consistency | 100% (all JSON) |
| Code duplication (error handlers) | 0 (shared package) |
| Test coverage (error scenarios) | 80% |
| HTTP compliance score | 95% |

### Overall Compliance Score

| Category | Current | v0.2.0 | v0.3.0 |
|----------|---------|--------|--------|
| Error Code Selection | 100% | 100% | 100% |
| HTTP Status Codes | 100% | 100% | 100% |
| XML Structure | 60% | 80% | 100% |
| Response Headers | 0% | 80% | 100% |
| Format Consistency | 70% | 70% | 100% |
| **Overall** | **66%** | **86%** | **100%** |

---

## Part 9: Rollout Plan

### v0.2.0 Rollout

**Pre-deployment:**
1. Run full test suite
2. Manual verification with test clients
3. Performance regression tests
4. Documentation updates

**Deployment:**
1. Deploy to staging environment
2. Monitor metrics for 24 hours
3. Deploy to production (canary 10% traffic)
4. Monitor for 48 hours
5. Full rollout

**Post-deployment:**
1. Monitor error rates
2. Check client library compatibility
3. Gather feedback from SDK users

### v0.3.0 Rollout

**Additional steps:**
1. Notify admin API users of breaking changes
2. Provide migration guide for JSON error format
3. Offer deprecation period if needed

---

## Part 10: Acceptance Criteria

By the end of **v0.2.0**, the following must be complete:

- [x] P1-001: All S3 responses include `x-amz-request-id` header
- [x] P1-002: All S3 error responses include `<RequestId>` XML element
- [x] P2-001: B2 endpoints return correct `Content-Type: application/json`
- [x] P2-002: All S3 responses include `x-amz-id-2` header
- [x] P2-003: Backend error propagation tests added
- [ ] Documentation updated
- [ ] Release notes published

By the end of **v0.3.0**, the following must be complete:

- [ ] P3-001: All S3 error responses include `<Resource>` XML element
- [ ] P3-002: All admin endpoints return structured JSON errors
- [ ] P3-003: All 405 responses use structured JSON format
- [ ] P3-004: Error handling consolidated to shared package
- [ ] P3-005: Comprehensive test coverage for error scenarios
- [ ] Full API documentation updated
- [ ] Migration guide published for admin API changes

---

## Part 11: Related Documentation

| Document | Purpose |
|----------|---------|
| `bf-2oq6du-header-consistency-issues.md` | Comprehensive issue catalog |
| `error-response-header-consistency.md` | Consistency verification results |
| `s3-endpoint-response-headers.md` | S3 endpoint header specification |
| `error-response-headers-specification.md` | Error response specification |
| `admin-endpoint-error-response-headers.md` | Admin endpoint header catalog |

---

## Part 12: Change History

| Date | Version | Changes |
|------|---------|---------|
| 2026-07-14 | 1.0 | Initial remediation plan for bead bf-1jjcp7 |

---

**Document Status:** ✅ Complete  
**Next Steps:** Begin Phase 1 implementation (v0.2.0)  
**Tracking Bead:** bf-1jjcp7

---

**End of Document**
