# Validate() Error Handling Updates - Prioritization

**Bead:** bf-5o1o1
**Task:** Prioritize Validate() error handling updates
**Date:** 2026-07-12
**Status:** COMPLETE

---

## Executive Summary

**Total call sites analyzed:** 106 (3 production Go + 3 production Rust + 100+ test)
**Sites requiring updates:** 1 (MEDIUM priority)
**Sites with adequate error handling:** 5 production sites

### Key Finding

The ARMOR codebase demonstrates **mature error handling practices** around Validate() calls. All Rust production code has excellent error handling. Only one Go call site has an opportunity for improvement: enriching error context extraction at `internal/yamlutil/schema.go:180`.

---

## Call Site Analysis Summary

### Rust Code (src/)

#### Production Call Sites: 3/3 ✅ Excellent

| Site | Location | Pattern | Status | Priority |
|------|----------|---------|--------|----------|
| 1 | src/parsers/config.rs:662 | `self.config.validate()?` | ✅ Excellent | LOW (no change) |
| 2 | src/parsers/config.rs:1007 | `self.config.validate()?` | ✅ Excellent | LOW (no change) |
| 3 | src/parsers/yaml/parser.rs:121 | `result = validator.validate(content)` | ✅ Good | LOW (no change) |

**Rust Assessment:** All production call sites use appropriate error handling patterns:
- Builder patterns correctly use `?` operator for error propagation
- YamlParser intentionally uses ValidationResult struct directly
- Error messages are descriptive and actionable
- Test coverage is comprehensive (100+ test sites)

**Rust Priority:** 🟢 **LOW** - No changes needed

---

### Go Code (internal/yamlutil)

#### Production Call Sites: 3

| Site | Location | Pattern | Status | Priority |
|------|----------|---------|--------|----------|
| 1 | internal/yamlutil/schema.go:180 | Type assertion + basic extraction | ⚠️ Incomplete | 🟡 MEDIUM |
| 2 | internal/yamlutil/schema.go:253 | Delegation | ✅ Appropriate | 🟢 LOW (no change) |
| 3 | internal/yamlutil/validator.go:110 | Different validation system | ✅ Out of scope | 🟢 LOW (no change) |

**Go Assessment:**
- Site 1 has functional error handling but misses 6 of 9 available error fields
- Site 2 is a simple delegation pattern - appropriate
- Site 3 uses a different validation system (ValidationResult, not YAMLError)

**Go Priority:** 🟡 **MEDIUM** for Site 1 only

---

## Detailed Priority Analysis

### Priority: HIGH 🔴

**None identified.**

No sites have missing error handling that would cause incorrect behavior or data loss.

---

### Priority: MEDIUM 🟡

#### 1. Enrich Error Context Extraction (Go only)

**Location:** `internal/yamlutil/schema.go:180-195`
**Caller:** `SchemaValidator.Validate(data interface{})`
**Callee:** `Schema.Validate(value interface{}) error`

**Current Behavior:**
```go
if yamlErr, ok := err.(YAMLError); ok {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   yamlErr.Error(),      // ✅ Extracted
        ErrorCode: yamlErr.Code(),        // ✅ Extracted
    })
}
```

**What's Missing:**
- ❌ FilePath - Path to the file being validated
- ❌ FieldPath - Dot-notation path to the invalid field
- ❌ Line - Line number where error occurred
- ❌ ErrorType - Error category (validation, type_mismatch, constraint)
- ❌ Expected - Expected type for type mismatch errors
- ❌ Found - Actual type found
- ❌ Context - Additional context about error state

**Impact:**
- **Debugging:** Users cannot locate where in the file the error occurred
- **Type Mismatches:** Cannot see expected vs actual types
- **Error Categorization:** Cannot programmatically handle different error types

**Risk Assessment:**
- **Risk of Change:** LOW - Additive field extraction only
- **Risk of Inaction:** LOW - Validation still works, just with less detail
- **Benefit of Fix:** HIGH - Significantly improved debugging experience

**Effort Estimate:** ~30 minutes implementation + ~30 minutes testing

**Recommended Update:**
```go
if yamlErr, ok := err.(YAMLError); ok {
    svarErr := SchemaValidationError{
        Message:   yamlErr.Error(),
        ErrorCode: yamlErr.Code(),
    }
    
    // Extract ValidationError struct fields if available
    if verr, ok := err.(*ValidationError); ok {
        svarErr.FilePath = verr.FilePath
        svarErr.FieldPath = verr.FieldPath
        svarErr.Line = verr.Line
        if verr.ExpectedType != "" {
            svarErr.Expected = verr.ExpectedType
        }
        if verr.ActualType != "" {
            svarErr.Found = verr.ActualType
        }
    }
    
    // Extract error type
    svarErr.ErrorType = string(yamlErr.YAMLErrorType())
    
    // Extract context if available
    if ctx := yamlErr.Context(); ctx != "" {
        svarErr.Context = ctx
    }
    
    result.Errors = append(result.Errors, svarErr)
}
```

---

### Priority: LOW 🟢

#### 1. Rust Code - All Sites (No Changes Needed)

**Assessment:** ✅ Excellent error handling throughout

**Rationale:**
- Builder patterns correctly propagate errors with `?` operator
- ValidationResult usage is intentional and appropriate
- Error messages are descriptive and actionable
- Test coverage is comprehensive

**Action:** None - maintain current patterns

---

#### 2. Go Site 2 - Delegation Pattern (No Changes Needed)

**Location:** `internal/yamlutil/schema.go:253`

**Current Code:**
```go
func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult {
    data, err := os.ReadFile(filePath)
    // ... error handling ...
    return sv.Validate(data)
}
```

**Assessment:** ✅ Appropriate delegation pattern

**Rationale:**
- Simple delegation to method with full error handling
- Single source of truth for error processing
- No error processing at delegation point (by design)

**Action:** None

---

#### 3. SchemaValidationError Schema Enhancement (Optional Future Work)

**Location:** `internal/yamlutil/errors.go`

**Proposed Addition:**
```go
type SchemaValidationError struct {
    FilePath    string    // ✅ Already exists
    SchemaPath  string    // Consider adding
    FieldPath   string    // ✅ Already exists
    Message     string    // ✅ Already exists
    Expected    string    // ✅ Already exists
    Found       string    // ✅ Already exists
    Line        int       // ✅ Already exists
    Column      int       // Consider adding
    ErrorCode   ErrorCode // ✅ Already exists
    ErrorType   string    // Consider adding
    Context     string    // Consider adding
}
```

**Assessment:** Optional enhancement

**Rationale:**
- Most fields already exist in struct
- Missing fields: SchemaPath, Column, ErrorType, Context
- Would require verifying no breaking changes to external consumers

**Action:** Optional - prioritize based on usage patterns

---

## Risk vs Benefit Matrix

| Site | Risk of Inaction | Risk of Change | Benefit | Priority |
|------|------------------|----------------|---------|----------|
| Go schema.go:180 | Low (degraded debuggability) | Low (additive only) | High (rich context) | 🟡 MEDIUM |
| Rust config.rs:662 | None | N/A | None | 🟢 LOW |
| Rust config.rs:1007 | None | N/A | None | 🟢 LOW |
| Rust parser.rs:121 | None | N/A | None | 🟢 LOW |
| Go schema.go:253 | None | N/A | None | 🟢 LOW |
| Go validator.go:110 | None | N/A | None | 🟢 LOW |

---

## Implementation Priority Order

### Phase 1: MEDIUM Priority (Recommended)

1. **Enrich Go error context extraction** (`internal/yamlutil/schema.go:180`)
   - Add type assertion to `*ValidationError`
   - Extract FilePath, FieldPath, Line, Expected, Found
   - Extract ErrorType and Context
   - Add unit tests for new extractions
   - **Effort:** 1 hour total
   - **Impact:** High - significantly improves error debugging

### Phase 2: LOW Priority (Optional)

1. **Consider SchemaValidationError schema enhancement**
   - Add ErrorType and Context fields if not present
   - Verify no breaking changes to external consumers
   - **Effort:** 30 minutes
   - **Impact:** Medium - better structured error data

### Phase 3: Future Considerations

1. **Error context standardization** (both languages)
   - Document best practices for error context enrichment
   - Add helper methods for common patterns
   - **Effort:** 2-4 hours
   - **Impact:** Medium - code consistency

2. **Error type consolidation** (architectural consideration)
   - Current mix: `Result<(), String>`, `Result<(), ParseError>`, `ValidationResult`
   - Consider standardizing on fewer types
   - **Effort:** 4-8 hours (breaking changes)
   - **Impact:** Low-Medium - cognitive load reduction

---

## Recommendations

### Immediate Actions (Recommended)

✅ **Implement MEDIUM priority update to Go schema.go:180**
- Low-risk, high-benefit improvement
- Straightforward field additions
- Improves debugging experience for all validation errors

### Future Improvements (Optional)

1. **SchemaValidationError schema enhancement**
   - Add ErrorType and Context fields
   - Verify external usage first

2. **Documentation updates**
   - Document YAMLError extraction pattern
   - Add examples of proper error context propagation
   - Include best practices guide

3. **Error context standardization**
   - Create helper methods for common enrichment patterns
   - Document consistency guidelines

### NOT Recommended

1. ❌ **Error type consolidation** (breaking change)
   - Current mix of types is intentional per context
   - Migration cost outweighs benefits
   - Not a priority

---

## Testing Requirements

### For Go schema.go:180 Update

**Required Tests:**
1. Unit test with validation errors containing file paths
2. Unit test with field path errors
3. Unit test with type mismatch errors (ExpectedType/ActualType)
4. Unit test with error type extraction
5. Unit test with context extraction

**Test Template:**
```go
func TestSchemaValidationErrorContextExtraction(t *testing.T) {
    schema := &SchemaDefinition{
        RootFields: map[string]*FieldDefinition{
            "name": {Type: "string", Required: true},
        },
    }
    
    validator := NewSchemaValidator(schema)
    result := validator.ValidateFile("testdata/invalid.yaml")
    
    if !result.Valid {
        for _, err := range result.Errors {
            assert.NotEmpty(t, err.FilePath, "FilePath should be populated")
            assert.NotEmpty(t, err.ErrorCode, "ErrorCode should be populated")
            // After implementation:
            assert.NotEmpty(t, err.FieldPath, "FieldPath should be populated")
            assert.NotEmpty(t, err.ErrorType, "ErrorType should be populated")
        }
    }
}
```

---

## Conclusion

The ARMOR codebase has **excellent error handling** in production code:

### Strengths
- ✅ All Rust code uses appropriate patterns (`?` operator, ValidationResult)
- ✅ Go code has functional error handling with room for enhancement
- ✅ Comprehensive test coverage (100+ test sites)
- ✅ No critical missing error handling

### Areas for Improvement
- 🟡 One Go site could benefit from richer error context extraction
- 🟢 Optional: SchemaValidationError schema enhancement
- 🟢 Optional: Error context standardization documentation

### Overall Assessment

**Priority Summary:**
- 🔴 HIGH: 0 sites
- 🟡 MEDIUM: 1 site (Go schema.go:180)
- 🟢 LOW: 5 sites (no changes needed, optional enhancements)

**Recommendation:** Implement the MEDIUM priority update to enrich Go error context extraction. It's low-risk, high-benefit, and straightforward to implement.

---

## Dependencies

### Required Documentation (Complete)
- ✅ bf-52zl8: Validate() call site locations
- ✅ bf-iamqn: Validate() call site categorization
- ✅ bf-5agz8: Validate() call site context verification
- ✅ bf-4y58v: Validate() error handling analysis

### Code References
- `internal/yamlutil/schema.go:180-195` (Go - MEDIUM priority)
- `src/parsers/config.rs:662, 1007` (Rust - LOW priority, no changes)
- `src/parsers/yaml/parser.rs:121` (Rust - LOW priority, no changes)

---

**Status:** PRIORITIZATION COMPLETE

**Next Steps:**
1. Review this prioritization
2. Implement MEDIUM priority update if approved
3. Add tests for new field extractions
4. Update documentation with extraction pattern examples

---

**Generated:** 2026-07-12
**Bead:** bf-5o1o1
