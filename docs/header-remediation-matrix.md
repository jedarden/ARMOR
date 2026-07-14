# ARMOR Header Remediation Matrix

**Version:** 1.0  
**Date:** 2026-07-14  
**Related Bead:** bf-1jjcp7

## Quick Reference Matrix

| Priority | Issue ID | Issue | Component | Effort | Release | Impact |
|----------|----------|-------|-----------|--------|---------|--------|
| **P1** | H1 | Missing `x-amz-request-id` header | S3 endpoints | 2-3h | v0.2.0 | 🔴 High - AWS SDK compatibility |
| **P1** | H2 | Missing `RequestId` XML element | S3 errors | 1-2h | v0.2.0 | 🔴 High - Request tracing |
| **P2** | MH2 | Content-Type mismatch (JSON→text/plain) | B2 endpoints | 2-3h | v0.2.0 | 🟡 Med-High - HTTP compliance |
| **P2** | MH1 | Missing `x-amz-id-2` header | S3 endpoints | 1-2h | v0.2.0 | 🟡 Med-High - S3 compliance |
| **P2** | Gap 2 | Backend error propagation tests | All S3 | 1-2d | v0.2.0 | 🟡 Med-High - Production readiness |
| **P3** | M1 | Missing `Resource` XML element | S3 errors | 2-3h | v0.3.0 | 🟢 Medium - Debugging context |
| **P3** | M2 | Mixed admin endpoint formats | Admin API | 2-3d | v0.3.0 | 🟢 Medium - API consistency |
| **P3** | M3 | 405 Method Not Allowed format | All admin | 1-2d | v0.3.0 | 🟢 Medium - API consistency |
| **P3** | M4 | Duplicate writeError functions | Code quality | 3-4h | v0.3.0 | 🟢 Medium - Maintainability |
| **P3** | Gap 1,3,5,7 | Comprehensive test coverage | Test suite | 3-5d | v0.3.0 | 🟢 Medium - Production confidence |
| **P4** | L1 | CORS headers always present | S3 endpoints | 2-3d | v0.4.0 | 🔵 Low - AWS behavior match |
| **P4** | L2 | Missing `HostId` XML element | S3 errors | 1h | v0.4.0 | 🔵 Low - Edge case completeness |
| **P4** | Gap 4,6 | Header ordering tests | Test suite | 1-2d | v0.4.0 | 🔵 Low - Edge case coverage |

**Legend:**
- 🔴 **High Priority** - Blocks compatibility or critical functionality
- 🟡 **Medium-High Priority** - Important compliance or production readiness
- 🟢 **Medium Priority** - API consistency, maintainability, or code quality
- 🔵 **Low Priority** - Optional enhancements or edge cases

---

## Summary by Priority

### P1 - High Priority (2 items, 3-5 hours)
**Must fix for AWS SDK compatibility and request tracing**

### P2 - Medium-High Priority (3 items, 2-3 days)
**Important for HTTP compliance and production readiness**

### P3 - Medium Priority (6 items, 1-2 weeks)
**API consistency, code maintainability, and test coverage**

### P4 - Low Priority (3 items, 3-5 days)
**Optional enhancements and edge cases**

---

## Summary by Component

### S3-Facing Endpoints
- **P1:** Request ID header, RequestId XML element
- **P2:** Extended ID header
- **P3:** Resource XML element
- **P4:** CORS behavior, HostId element

### Admin Endpoints
- **P2:** B2 content-type fix
- **P3:** Error response standardization, 405 format

### Code Quality
- **P3:** Consolidate error handling

### Test Coverage
- **P2:** Backend error propagation
- **P3:** Comprehensive error scenarios
- **P4:** Header ordering, chunked encoding

---

## Effort Summary

| Priority | Total Effort | Quick Wins | Larger Tasks |
|----------|--------------|------------|--------------|
| P1 | 3-5 hours | ✅ All | - |
| P2 | 2-3 days | 3 fixes (6-8h) | 1 test suite (1-2d) |
| P3 | 1-2 weeks | 3 fixes (6-10h) | 3 tasks (4-9d) |
| P4 | 3-5 days | 1 fix (1h) | 2 tasks (3-5d) |
| **Total** | **3-4 weeks** | **10 fixes (13-21h)** | **6 tasks (8-16d)** |

---

## Release Timeline

```
v0.2.0 (2 weeks)  ████████████████████████████████
  ├─ P1-001: Request ID header (2-3h)
  ├─ P1-002: RequestId XML element (1-2h)
  ├─ P2-001: B2 content-type fix (2-3h)
  ├─ P2-002: Extended ID header (1-2h)
  └─ P2-003: Backend error tests (1-2d)

v0.3.0 (6-8 weeks)  ████████████████████████████████████████████████████████
  ├─ P3-001: Resource XML element (2-3h)
  ├─ P3-002: Admin endpoint standardization (2-3d)
  ├─ P3-003: 405 format consistency (1-2d)
  ├─ P3-004: Consolidate error handling (3-4h)
  └─ P3-005: Comprehensive test coverage (3-5d)

v0.4.0 (Future)  ████████████████████████████████
  ├─ P4-001: Configurable CORS (2-3d)
  ├─ P4-002: HostId element (1h)
  └─ P4-003: Edge case tests (1-2d)
```

---

## Quick Wins vs. Long-Term

### Quick Wins (13-21 hours of work)
**Can be completed in 2-3 days**

| Priority | Issue | Effort | Impact |
|----------|-------|--------|--------|
| P1 | H1: Request ID header | 2-3h | 🔴 High |
| P1 | H2: RequestId XML | 1-2h | 🔴 High |
| P2 | MH2: B2 content-type | 2-3h | 🟡 Med-High |
| P2 | MH1: Extended ID | 1-2h | 🟡 Med-High |
| P3 | M1: Resource XML | 2-3h | 🟢 Medium |
| P3 | M4: Consolidate errors | 3-4h | 🟢 Medium |
| P4 | L2: HostId XML | 1h | 🔵 Low |

### Long-Term Improvements (8-16 days of work)
**Require more planning and testing**

| Priority | Issue | Effort | Impact |
|----------|-------|--------|--------|
| P2 | Gap 2: Backend error tests | 1-2d | 🟡 Med-High |
| P3 | M2: Admin endpoint standardization | 2-3d | 🟢 Medium |
| P3 | M3: 405 format consistency | 1-2d | 🟢 Medium |
| P3 | Gap 1,3,5,7: Comprehensive tests | 3-5d | 🟢 Medium |
| P4 | L1: Configurable CORS | 2-3d | 🔵 Low |
| P4 | Gap 4,6: Edge case tests | 1-2d | 🔵 Low |

---

## Compliance Progress

**Current State: 66% Compliant**

```
███████████████████████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
```

**After v0.2.0: 86% Compliant**

```
█████████████████████████████████████████████████████████████████████░░░░░
```

**After v0.3.0: 100% Compliant**

```
███████████████████████████████████████████████████████████████████████████
```

---

## Decision Matrix

### Fix Now (v0.2.0)
- Blocks AWS SDK compatibility
- Breaks request tracing
- HTTP compliance violations
- Production readiness gaps

### Fix Soon (v0.3.0)
- API inconsistencies
- Code maintainability issues
- Test coverage gaps

### Fix Later (v0.4.0+)
- Optional enhancements
- Edge case coverage
- Nice-to-have features

---

## Dependencies

```
P1-001 (Request ID header)
  └─→ P1-002 (RequestId XML)
      └─→ P2-002 (Extended ID)
          └─→ P3-001 (Resource XML)
              └─→ P4-002 (HostId XML)

P3-002 (Admin standardization)
  └─→ P3-003 (405 format)

P1-002 (RequestId XML)
  └─→ P3-004 (Consolidate errors)
```

---

## Risk Rating

| Priority | Risk Level | Rationale |
|----------|------------|------------|
| P1 | Low | Pure additive changes, no breaking changes |
| P2 | Low-Medium | Content-type fix has low risk; tests are additive |
| P3 | Medium | Admin API changes may break existing clients |
| P4 | Low | Optional features, can be deferred |

---

## Business Value

| Priority | Business Impact | ROI |
|----------|-----------------|-----|
| P1 | 🔴 High - SDK compatibility, debugging | ⭐⭐⭐⭐⭐ |
| P2 | 🟡 Medium-High - Compliance, production readiness | ⭐⭐⭐⭐ |
| P3 | 🟢 Medium - Developer experience, maintainability | ⭐⭐⭐ |
| P4 | 🔵 Low - Edge cases, optional features | ⭐⭐ |

---

## Implementation Notes

### Phase 1 (v0.2.0) - Critical Path
1. Start with P1-001 (foundation for all other request ID work)
2. Immediately implement P1-002 (depends on P1-001)
3. Parallel: P2-001 (independent, simple fix)
4. Then P2-002 (builds on P1-001)
5. Finally P2-003 (tests for production readiness)

### Phase 2 (v0.3.0) - Consistency Path
1. P3-001 (builds on P1-002, P2-002)
2. P3-004 (consolidation after XML structure finalized)
3. P3-002 (breaking change - coordinate with users)
4. P3-003 (depends on P3-002)
5. P3-005 (comprehensive tests)

### Phase 3 (v0.4.0+) - Enhancement Path
1. P4-002 (quick win, builds on P3-001)
2. P4-001 (larger feature, can be deferred)
3. P4-003 (edge case tests)

---

**Document Status:** ✅ Complete  
**Purpose:** Quick reference for remediation planning  
**Full Plan:** See `header-remediation-plan.md`

---

**End of Document**
