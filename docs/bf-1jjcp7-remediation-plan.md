# ARMOR Header Consistency Remediation Plan

**Version:** 1.0  
**Date:** 2026-07-14  
**Related Bead:** bf-1jjcp7  
**Status:** Complete  
**Source Document:** bf-2oq6du-header-consistency-issues.md

---

## Executive Summary

This document provides a prioritized remediation plan for all 13 identified header consistency issues and 7 test coverage gaps in ARMOR. The plan balances AWS S3 compatibility requirements, production stability, and development effort to create a clear roadmap for improvements.

### Remediation Overview

| Priority Level | Issues | Total Effort | Target Timeline | Business Impact |
|----------------|--------|--------------|-----------------|-----------------|
| **P0** | 2 | 3-5 hours | Immediate (Release 0.2.0) | Restores AWS SDK compatibility |
| **P1** | 3 | 2-3 days | Next Sprint (Release 0.2.1) | Production readiness & correctness |
| **P2** | 7 | 1-2 weeks | Next Quarter (Release 0.3.0) | API consistency & maintainability |
| **P3** | 4 | 3-5 days | Future (Release 0.4.0) | Full AWS parity & edge cases |
| **Total** | **16** | **3-4 weeks** | **Q3-Q4 2026** | **Complete compliance** |

---

## Priority Matrix (P0-P3)

### P0: Critical - AWS SDK Compatibility (DO NOW)

**Timeline:** Immediate - Must ship in Release 0.2.0  
**Risk of Delay:** Breaking AWS SDK compatibility, untraceable requests  
**Effort:** 3-5 hours total

| Issue ID | Component | Effort | Complexity | Risk | Business Impact |
|----------|-----------|--------|------------|------|-----------------|
| **H1** | x-amz-request-id header | 2-3 hours | Low | Low | **HIGH** - Enables SDK compatibility, request tracing, support debugging |
| **H2** | RequestId XML element | 1-2 hours | Low | Low | **HIGH** - Completes request tracing, SDK error parsing |

**Dependencies:** H2 depends on H1 (request ID must be generated first)

**Implementation Notes:**
```go
// Create new middleware: internal/server/middleware/request_id.go
package middleware

import (
    "github.com/google/uuid"
    "net/http"
)

// RequestID generates and adds x-amz-request-id header to all responses
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := uuid.New().String()
        w.Header().Set("x-amz-request-id", requestID)
        
        // Store in request context for error responses
        ctx := context.WithValue(r.Context(), "requestID", requestID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Updated writeError in handlers.go
func writeError(w http.ResponseWriter, r *http.Request, code, message string, statusCode int) {
    requestID := r.Context().Value("requestID").(string)
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
  <RequestId>%s</RequestId>
</Error>`, code, message, requestID)
}
```

**Testing Required:**
- Verify header present on all S3 responses (success & error)
- Verify RequestId element in all error XML
- Test request ID propagation through middleware chain
- Validate UUID format matches AWS pattern

**Quick Win:** ✅ YES - Simple middleware, immediate impact

---

### P1: High - Production Readiness (NEXT SPRINT)

**Timeline:** Next Sprint - Target Release 0.2.1  
**Risk of Delay:** Incorrect HTTP semantics, untested backend failures  
**Effort:** 2-3 days total

| Issue ID | Component | Effort | Complexity | Risk | Business Impact |
|----------|-----------|--------|------------|------|-----------------|
| **MH1** | x-amz-id-2 header | 1-2 hours | Low | Low | **MED** - Extended request tracing, S3 tool compatibility |
| **MH2** | Content-Type mismatch (B2 endpoints) | 2-3 hours | Low | Low | **MED** - Correct HTTP semantics, JSON parsing |
| **Gap 2** | Backend error propagation tests | 1-2 days | Medium | Medium | **HIGH** - Production confidence, error mapping validation |

**Dependencies:** MH1 depends on P0 completion (uses request ID infrastructure)

**Implementation Notes for MH1:**
```go
// Extend request ID middleware to include x-amz-id-2
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := uuid.New().String()
        extendedID := generateExtendedID() // Opaque string for S3 compatibility
        
        w.Header().Set("x-amz-request-id", requestID)
        w.Header().Set("x-amz-id-2", extendedID)
        
        ctx := context.WithValue(r.Context(), "requestID", requestID)
        ctx = context.WithValue(ctx, "extendedID", extendedID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func generateExtendedID() string {
    // S3 uses an opaque extended ID - we can use base64 of random bytes
    b := make([]byte, 16)
    rand.Read(b)
    return base64.StdEncoding.EncodeToString(b)
}
```

**Implementation Notes for MH2:**
```go
// internal/server/server.go:1246-1364
// Fix B2 endpoints to return correct Content-Type
func (s *Server) handleListB2Keys(w http.ResponseWriter, r *http.Request) {
    // ... existing logic ...
    
    if err != nil {
        w.Header().Set("Content-Type", "application/json") // FIX: was text/plain
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": fmt.Sprintf("Failed to list keys: %v", err),
        })
        return
    }
    // ... success case already returns application/json ...
}
```

**Implementation Notes for Gap 2 (Backend Error Tests):**
```go
// internal/server/handlers/backend_test.go
func TestBackendErrorPropagation(t *testing.T) {
    tests := []struct {
        name          string
        simulateError func(*mockBackend)
        expectCode    string
        expectStatus  int
    }{
        {
            name: "B2 rate limit maps to SlowDown",
            simulateError: func(m *mockBackend) {
                m.simulateRateLimit()
            },
            expectCode: "SlowDown",
            expectStatus: 503,
        },
        {
            name: "B2 timeout maps to RequestTimeout",
            simulateError: func(m *mockBackend) {
                m.simulateTimeout()
            },
            expectCode: "RequestTimeout",
            expectStatus: 400,
        },
        {
            name: "Storage full maps to InternalError",
            simulateError: func(m *mockBackend) {
                m.simulateStorageFull()
            },
            expectCode: "InternalError",
            expectStatus: 500,
        },
    }
    // ... test implementation ...
}
```

**Testing Required:**
- Verify x-amz-id-2 format and uniqueness
- Confirm B2 error responses return application/json
- Run backend error simulation tests
- Validate error code mapping matches AWS S3 behavior

**Quick Wins:** ✅ YES - MH1 and MH2 are simple fixes with high value

---

### P2: Medium - API Consistency & Maintainability (NEXT QUARTER)

**Timeline:** Next Quarter - Target Release 0.3.0  
**Risk of Delay:** API fragmentation, maintenance burden, incomplete debugging  
**Effort:** 1-2 weeks total

| Issue ID | Component | Effort | Complexity | Risk | Business Impact |
|----------|-----------|--------|------------|------|-----------------|
| **M1** | Resource XML element | 2-3 hours | Low | Low | **MED** - Debugging context, tool compatibility |
| **M2** | Mixed response formats (admin endpoints) | 2-3 days | Medium | Medium | **MED** - API consistency, client experience |
| **M3** | Method Not Allowed format (405) | 1-2 days | Medium | Low | **MED** - API consistency, error handling |
| **M4** | Duplicate writeError implementations | 3-4 hours | Low | Low | **LOW** - Code maintainability |
| **Gap 1** | Pre-signed URL error tests | 1 day | Medium | Low | **MED** - Edge case coverage |
| **Gap 3** | Concurrent request tests | 1 day | Medium | Medium | **MED** - Production load patterns |
| **Gap 5** | Large payload error tests | 1 day | Medium | Low | **MED** - Memory safety, edge cases |
| **Gap 7** | Complex conditional request tests | 1 day | Medium | Low | **LOW** - Edge case coverage |

**Dependencies:** None - Can be worked in parallel

**Implementation Notes for M1 (Resource element):**
```go
// Extract resource from request path
func extractResource(r *http.Request) string {
    // S3 resource format: /bucket/key or /bucket
    path := r.URL.Path
    if strings.HasPrefix(path, "/") {
        return path
    }
    return "/" + path
}

// Updated writeError with Resource
func writeError(w http.ResponseWriter, r *http.Request, code, message string, statusCode int) {
    requestID := r.Context().Value("requestID").(string)
    resource := extractResource(r)
    
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
```

**Implementation Notes for M2 & M3 (Standardized Admin Errors):**
```go
// internal/pkg/errors/admin.go
package errors

import "net/http"

// AdminError represents a structured admin API error
type AdminError struct {
    Code           string `json:"code"`
    Message        string `json:"error"`
    AllowedMethods []string `json:"allowed_methods,omitempty"`
}

// WriteAdminError writes a standardized JSON error response
func WriteAdminError(w http.ResponseWriter, err AdminError, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(err)
}

// Usage in handlers:
func (s *Server) handleKeyRotate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        errors.WriteAdminError(w, errors.AdminError{
            Code:           "MethodNotAllowed",
            Message:        "Method not allowed",
            AllowedMethods: []string{"POST"},
        }, http.StatusMethodNotAllowed)
        return
    }
    // ... existing logic ...
}
```

**Implementation Notes for M4 (Consolidate writeError):**
```go
// Create: internal/pkg/errors/s3.go
package errors

import (
    "fmt"
    "net/http"
)

// WriteS3Error writes an S3-compliant XML error response
func WriteS3Error(w http.ResponseWriter, r *http.Request, code, message string, statusCode int) {
    requestID := r.Context().Value("requestID").(string)
    extendedID := r.Context().Value("extendedID").(string)
    resource := extractResource(r)
    
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
  <Resource>%s</Resource>
  <RequestId>%s</RequestId>
  <HostId>%s</HostId>
</Error>`, code, message, resource, requestID, extendedID)
}

// Replace all writeError calls in server.go and handlers.go
```

**Testing Required:**
- Verify Resource element format matches S3 pattern
- Test all admin endpoints return JSON errors consistently
- Validate Method Not Allowed includes allowed methods
- Run concurrent request tests under load
- Test large file upload error scenarios
- Verify pre-signed URL edge cases (expired, invalid, concurrent)

**Quick Wins:** ⚠️ PARTIAL - M1, M4 are quick; M2, M3 require more effort but high value

---

### P3: Low - Full AWS Parity & Edge Cases (FUTURE)

**Timeline:** Future Enhancement - Target Release 0.4.0  
**Risk of Delay:** Minimal - Nice-to-have features  
**Effort:** 3-5 days total

| Issue ID | Component | Effort | Complexity | Risk | Business Impact |
|----------|-----------|--------|------------|------|-----------------|
| **L1** | CORS headers configuration | 2-3 days | High | Medium | **LOW** - AWS behavior match, security tuning |
| **L2** | HostId XML element | 1 hour | Low | Low | **LOW** - Edge case completeness |
| **Gap 4** | Header ordering tests | 1 day | Low | Low | **LOW** - Edge case coverage |
| **Gap 6** | Chunked encoding error tests | 1-2 days | Medium | Low | **LOW** - Streaming edge cases |

**Dependencies:** L2 depends on P0-P1 completion (uses request ID infrastructure)

**Implementation Notes for L1 (Configurable CORS):**
```go
// Add CORS configuration to bucket config
type BucketCORSConfig struct {
    AllowedOrigins     []string `json:"allowed_origins"`
    AllowedMethods     []string `json:"allowed_methods"`
    AllowedHeaders     []string `json:"allowed_headers"`
    ExposeHeaders      []string `json:"expose_headers"`
    MaxAgeSeconds      int      `json:"max_age_seconds"`
    AllowCredentials  bool     `json:"allow_credentials"`
}

// Middleware checks bucket CORS config before adding headers
func CORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        bucket := extractBucket(r)
        
        if config := getBucketCORSConfig(bucket); config != nil {
            // Apply bucket-specific CORS rules
            if originMatches(r.Header.Get("Origin"), config.AllowedOrigins) {
                w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
                w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
                w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
                if config.MaxAgeSeconds > 0 {
                    w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAgeSeconds))
                }
            }
        } else {
            // No CORS config - no headers (matches AWS behavior)
        }
        
        next.ServeHTTP(w, r)
    })
}
```

**Testing Required:**
- Verify CORS headers respect bucket configuration
- Test CORS preflight requests
- Validate HostId element format
- Run chunked encoding error scenarios
- Test header ordering consistency

**Quick Wins:** ❌ NO - Requires significant effort for low business impact

---

## Implementation Timeline

### Phase 1: Immediate Fixes (Week 1 - Release 0.2.0)

**Deliverables:**
- ✅ x-amz-request-id header middleware
- ✅ RequestId XML element in error responses
- ✅ Test coverage for request ID propagation

**Risk:** Low - Simple additions, no breaking changes

**Validation:**
```bash
# Test request ID present on all responses
curl -I http://localhost:8080/bucket/key
# Should include: x-amz-request-id: <uuid>

# Test error includes RequestId element
curl http://localhost:8080/nonexistent-bucket/key
# Should include: <RequestId><uuid></RequestId>
```

---

### Phase 2: Production Readiness (Weeks 2-3 - Release 0.2.1)

**Deliverables:**
- ✅ x-amz-id-2 extended request ID header
- ✅ B2 endpoint Content-Type fixes
- ✅ Backend error propagation test suite

**Risk:** Low-Medium - Requires mock backend infrastructure

**Validation:**
```bash
# Test extended ID present
curl -I http://localhost:8080/bucket/key
# Should include: x-amz-id-2: <base64-string>

# Test B2 error returns JSON
curl http://localhost:8080/admin/b2/keys
# Error case should return: Content-Type: application/json
```

---

### Phase 3: API Consistency (Weeks 4-6 - Release 0.3.0)

**Deliverables:**
- ✅ Resource XML element in error responses
- ✅ Standardized JSON error format for admin endpoints
- ✅ Method Not Allowed structured responses
- ✅ Consolidated error handling utilities
- ✅ Comprehensive test coverage for edge cases

**Risk:** Medium - Changes error response format for admin endpoints (non-breaking but requires client updates)

**Validation:**
```bash
# Test Resource element present
curl http://localhost:8080/bucket/nonexistent-key
# Should include: <Resource>/bucket/nonexistent-key</Resource>

# Test admin endpoint returns JSON error
curl -X GET http://localhost:8080/admin/key/rotate
# Should return: {"code":"MethodNotAllowed","error":"Method not allowed","allowed_methods":["POST"]}
```

---

### Phase 4: Full Parity (Weeks 7-9 - Release 0.4.0)

**Deliverables:**
- ✅ Configurable CORS behavior
- ✅ HostId XML element
- ✅ Edge case test coverage (headers, chunked encoding)

**Risk:** Medium-High - CORS behavior change may affect existing clients

**Validation:**
```bash
# Test CORS respects bucket config
# Before: All requests get CORS headers
# After: Only buckets with CORS config get headers

# Test HostId element
curl http://localhost:8080/bucket/nonexistent-key
# Should include: <HostId><extended-id></HostId>
```

---

## Effort vs. Impact Matrix

```
HIGH IMPACT
    │
    │  P0 │ P1 │
    │     │    │
────┼─────┼────┼───── HIGH EFFORT
    │     │ P2 │
    │     │    │
    │  P3 │    │
    │     │    │
    └─────┴────┴─────→
           LOW EFFORT
```

**Quick Wins (High Impact, Low Effort):**
- P0: Request ID infrastructure (3-5 hours)
- P1: Content-Type fixes (2-3 hours)
- P1: Extended request ID (1-2 hours)

**Strategic Investments (High Impact, High Effort):**
- P1: Backend error tests (1-2 days)
- P2: Admin endpoint consistency (2-3 days)

**Low Priority (Low Impact):**
- P3: CORS configuration (2-3 days)
- P3: Header ordering tests (1 day)

---

## Risk Assessment

### Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Breaking existing clients with error format changes | Low | Medium | Version admin API changes, document migration |
| Performance degradation from request ID generation | Low | Low | Use UUID v4 (fast), benchmark before merge |
| Test suite complexity increases | Medium | Low | Incremental test addition, maintain test clarity |
| CORS behavior change affects existing integrations | Medium | Medium | Feature flag, gradual rollout, monitor logs |

### Operational Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Insufficient testing time for backend error scenarios | Medium | Medium | Prioritize P1 tests, add to CI |
| Resource constraints for Phase 2 work | Low | Low | Plan capacity, be ready to defer P3 items |
| Documentation drift during implementation | Medium | Low | Update docs with each PR, not at end |

---

## Testing Strategy

### Unit Tests

**Coverage Targets:**
- P0: 100% of request ID middleware paths
- P1: 90% of backend error mappings
- P2: 85% of admin error handlers
- P3: 70% of CORS logic

**Test Files:**
- `internal/server/middleware/request_id_test.go` (NEW)
- `internal/server/handlers/backend_error_test.go` (NEW)
- `internal/server/admin_error_test.go` (NEW)
- `internal/pkg/errors/s3_test.go` (NEW)

### Integration Tests

**Scenarios:**
- End-to-end request ID propagation (all endpoints)
- Backend failure simulation (rate limits, timeouts, auth failures)
- Admin endpoint error format consistency
- CORS configuration behavior

### Performance Tests

**Benchmarks:**
- Request ID generation overhead (target: < 1μs per request)
- Error response size increase (target: < 100 bytes per error)
- CORS lookup overhead (target: < 10μs per request)

---

## Success Metrics

### Release 0.2.0 (P0)

| Metric | Target | How to Measure |
|--------|--------|----------------|
| Request ID presence | 100% of responses | Integration tests, manual curl |
| RequestId element present | 100% of error XML | Integration tests |
| AWS SDK compatibility | Full | Test with boto3, AWS Go SDK |

### Release 0.2.1 (P1)

| Metric | Target | How to Measure |
|--------|--------|----------------|
| Extended ID presence | 100% of responses | Integration tests |
| B2 error Content-Type | 100% JSON | Integration tests |
| Backend error coverage | > 80% scenarios | Test coverage report |

### Release 0.3.0 (P2)

| Metric | Target | How to Measure |
|--------|--------|----------------|
| Resource element present | 100% of errors | Integration tests |
| Admin error format | 100% JSON | Integration tests |
| Code duplication | < 5% duplicate error code | SonarQube, manual review |

### Release 0.4.0 (P3)

| Metric | Target | How to Measure |
|--------|--------|----------------|
| CORS configuration | Working | Integration tests, feature flag |
| HostId element | 100% of errors | Integration tests |
| Edge case coverage | > 90% scenarios | Test coverage report |

---

## Dependencies and Blocking Items

### External Dependencies

- ✅ None identified - all work is internal

### Internal Dependencies

```
P0 (Request ID)
  ├── H1: x-amz-request-id header
  └── H2: RequestId XML element
      ↓
P1 (Extended IDs & Correctness)
  ├── MH1: x-amz-id-2 header (uses P0 infrastructure)
  ├── MH2: Content-Type fixes (independent)
  └── Gap 2: Backend tests (independent)
      ↓
P2 (Consistency)
  ├── M1: Resource element (uses P0 infrastructure)
  ├── M2: Admin format standardization (independent)
  ├── M3: Method Not Allowed (independent)
  ├── M4: Code consolidation (uses P0-P1 infrastructure)
  └── Gaps 1,3,5,7: Test coverage (independent)
      ↓
P3 (Parity)
  ├── L1: CORS configuration (independent)
  ├── L2: HostId element (uses P0-P1 infrastructure)
  └── Gaps 4,6: Edge case tests (independent)
```

**Critical Path:** P0 → P1 → P2 → P3  
**Parallel Work:** Within each priority, items can be worked simultaneously

---

## Rollout Strategy

### Phase 1: P0 (Immediate)

**Rollout Plan:**
1. Implement request ID middleware in dev
2. Run full test suite
3. Deploy to staging environment
4. Validate with production-like load
5. Deploy to production (Release 0.2.0)
6. Monitor request ID logs for 48 hours

**Rollback Plan:**
- Remove middleware from chain
- No data migration needed (stateless change)

---

### Phase 2: P1 (Next Sprint)

**Rollout Plan:**
1. Implement extended ID and Content-Type fixes
2. Add backend error simulation tests
3. Deploy to staging, run backend failure scenarios
4. Deploy to production (Release 0.2.1)
5. Monitor error logs for correct mappings

**Rollback Plan:**
- Revert Content-Type changes
- Disable extended ID generation
- Tests remain in place

---

### Phase 3: P2 (Next Quarter)

**Rollout Plan:**
1. Implement admin endpoint format changes
2. Update internal tooling to use new error format
3. Deploy to staging, validate admin API compatibility
4. Coordinate with internal teams on client updates
5. Deploy to production (Release 0.3.0)
6. Monitor admin API error logs

**Rollback Plan:**
- Revert admin error handlers to plain text
- Client applications tolerate both formats
- Document deprecation timeline

---

### Phase 4: P3 (Future)

**Rollout Plan:**
1. Implement configurable CORS
2. Feature flag disabled by default
3. Enable per-bucket gradually
4. Monitor CORS header behavior
5. Deploy to production (Release 0.4.0)

**Rollback Plan:**
- Disable feature flag
- Revert to always-allow CORS behavior
- Monitor for client-side CORS errors

---

## Monitoring and Alerting

### New Metrics to Monitor

| Metric | Type | Alert Threshold | Purpose |
|--------|------|-----------------|---------|
| request_id_missing_rate | gauge | > 0.1% | Detect middleware failures |
| error_response_size | histogram | p99 > 1KB | Track error response growth |
| backend_error_mapping_failure | counter | > 0 | Detect unmapped backend errors |
| admin_error_format_inconsistency | counter | > 0 | Detect non-JSON admin errors |
| cors_header_absence_rate | gauge | > 0% (when configured) | Verify CORS configuration |

### Log Queries

```sql
-- Verify request ID presence
| count() as total, 
  count(request_id) as with_id,
  (with_id * 100.0 / total) as coverage
FROM armor.http_requests

-- Detect missing Resource elements
| count() as errors,
  count() as with_resource,
  (errors - with_resource) as missing
FROM armor.s3_error_responses
WHERE http_status >= 400

-- Admin endpoint format check
| endpoint, 
  count() as total,
  count(content_type="application/json") as json_errors
FROM armor.admin_api_errors
GROUP BY endpoint
```

---

## Documentation Updates

### Documents to Update

| Document | Updates Required | Priority |
|----------|-----------------|----------|
| `docs/error-response-headers-specification.md` | Add x-amz-request-id, x-amz-id-2 | P0 |
| `docs/admin-endpoint-error-response-headers.md` | Update error format specification | P2 |
| `README.md` | Update API compatibility notes | P0 |
| API documentation | Add request ID header documentation | P0 |
| Changelog | Document all changes per release | All |
| `docs/bf-1jjcp7-remediation-plan.md` | Mark items complete as implemented | Ongoing |

---

## Acceptance Criteria

### Overall Plan Acceptance

- ✅ All 16 issues assigned priority levels (P0-P3)
- ✅ Effort estimates provided for each fix (3 hours - 3 days)
- ✅ Target release timeline proposed (0.2.0 - 0.4.0)
- ✅ Implementation notes and considerations documented
- ✅ Quick wins identified and separated (P0-P1 items)
- ✅ Risk assessment included for all phases
- ✅ Testing strategy defined
- ✅ Success metrics established
- ✅ Rollback plans documented

### Per-Priority Acceptance

**P0 Acceptance:**
- [ ] Request ID middleware implemented
- [ ] RequestId element in all error responses
- [ ] Tests pass (unit + integration)
- [ ] AWS SDK compatibility validated
- [ ] Documentation updated

**P1 Acceptance:**
- [ ] Extended ID header added
- [ ] B2 endpoints return correct Content-Type
- [ ] Backend error tests passing (80%+ coverage)
- [ ] Error mappings validated
- [ ] Documentation updated

**P2 Acceptance:**
- [ ] Resource element in all error responses
- [ ] Admin endpoints standardized to JSON errors
- [ ] Method Not Allowed returns structured response
- [ ] Duplicate writeError code consolidated
- [ ] Test coverage gaps filled (Gaps 1,3,5,7)
- [ ] Documentation updated

**P3 Acceptance:**
- [ ] CORS behavior configurable per bucket
- [ ] HostId element in error responses
- [ ] Edge case tests passing (Gaps 4,6)
- [ ] Documentation updated

---

## Next Steps

### Immediate Actions (This Week)

1. **Create P0 implementation bead** - Track request ID middleware work
2. **Set up development branch** - `feature/request-id-infrastructure`
3. **Write initial middleware** - `internal/server/middleware/request_id.go`
4. **Add test coverage** - `middleware/request_id_test.go`

### Short-Term Actions (Next Sprint)

1. **Create P1 implementation beads** - Track extended ID and Content-Type work
2. **Set up backend error simulation** - Mock infrastructure for tests
3. **Plan admin endpoint migration** - Coordinate with internal teams

### Long-Term Actions (Next Quarter)

1. **Create P2 implementation beads** - Track consistency improvements
2. **Plan admin API versioning** - If breaking changes needed
3. **Evaluate CORS requirements** - Gather bucket configuration needs

---

## Appendix: Quick Reference

### Priority Cheat Sheet

```
P0 = CRITICAL - AWS SDK compatibility, must fix now (3-5 hours)
P1 = HIGH - Production readiness, fix next sprint (2-3 days)
P2 = MEDIUM - API consistency, fix next quarter (1-2 weeks)
P3 = LOW - Nice-to-have, future enhancement (3-5 days)
```

### Effort Estimates

```
Low    = 1-3 hours   (simple code change, limited testing)
Medium = 1-2 days    (moderate complexity, multiple test scenarios)
High   = 2-3 days    (complex logic, extensive testing, infrastructure)
```

### Risk Levels

```
Low     = Simple change, no dependencies, easy rollback
Medium  = Moderate complexity, some dependencies, planned rollback
High    = Complex logic, many dependencies, careful rollback required
```

---

**Document Status:** ✅ Complete  
**Remediation Plan Status:** Ready for Implementation  
**Next Phase:** P0 Implementation (Request ID Infrastructure)

---

**End of Document**
