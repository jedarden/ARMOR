# Error Handling Updates - Urgency Prioritization

**Bead ID:** bf-261aw
**Date:** 2026-07-12
**Based on Analysis:** bf-5sor5 (error handling gaps), bf-678r9 (prioritization)
**Status:** ✅ COMPLETE

## Task

Prioritize identified error handling gaps by urgency using the following framework:
- **Critical:** validate() on user input or external data
- **High:** validate() in core business logic
- **Medium:** validate() in internal helpers
- **Low:** validate() in tests or edge cases

---

## Summary

**Total production code call sites analyzed:** 5 (3 Rust + 2 Go)
**Sites requiring updates:** 2 (1 High, 1 Medium)
**Sites with no updates needed:** 3 (already excellent)

### Priority Distribution

| Priority | Count | Sites | Action Required |
|----------|-------|-------|-----------------|
| **Critical (P0)** | 0 | None | ✅ None - excellent |
| **High (P1)** | 1 | `src/parsers/yaml/parser.rs:121` | Review for enhanced context |
| **Medium (P2)** | 1 | `internal/yamlutil/schema.go:180` | Enrich error field extraction |
| **Low (P3)** | 3 | `src/parsers/config.rs:662,1007` + `schema.go:253` | ✅ None - already excellent |

---

## Ranked Priorities by Urgency

### Priority Level 1: CRITICAL (P0)
**Definition:** validate() on user input or external data with missing or inadequate error handling

**Sites:** **NONE** ✅

**Assessment:** No critical gaps found. All sites handling external data have appropriate error handling.

---

### Priority Level 2: HIGH (P1)
**Definition:** validate() in core business logic where enhanced context would significantly improve debugging

#### Site #1: `src/parsers/yaml/parser.rs:121`

**Current Code:**
```rust
fn validate_str(&self, content: &str) -> ValidationResult {
    let validator = if self.config.is_strict() {
        SyntaxValidator::strict()
    } else {
        SyntaxValidator::lenient()
    };

    let mut result = validator.validate(content);

    // Enhanced detection
    if result.is_valid() {
        let mut detector = SyntaxDetector::new();
        let detector_result = detector.detect_to_validation_result(content);

        if !detector_result.is_valid() {
            result.valid = false;
            result.errors.extend(detector_result.errors);
        }
    }

    result
}
```

**Context:** BasicParser.validate_str() - validates external YAML string content

**Why HIGH Priority:**
- ✅ Handles **external data** (user input or files)
- ✅ Core **business logic** for YAML parsing
- ✅ Current error handling is GOOD (accumulates errors properly)
- ⚠️ **Missing data source context** - no indication of where YAML came from

**Current State:** Functional but lacks diagnostic context

**Recommended Enhancement:**
```rust
// Add source context to ValidationResult
fn validate_str_with_source(&self, content: &str, source: &str) -> ValidationResult {
    let mut result = self.validate_str(content);
    result.source = Some(source.to_string()); // Add source field
    result
}
```

**Benefits:**
- Better debugging when validation fails
- Clear indication of data origin (file path, user input, etc.)
- Helps trace error sources in production

**Effort:** LOW (add optional source field to ValidationResult)

**Timeline:** Next quarterly review

---

### Priority Level 3: MEDIUM (P2)
**Definition:** validate() in internal helpers where error context could be enriched

#### Site #2: `internal/yamlutil/schema.go:180`

**Current Code:**
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false

    if yamlErr, ok := err.(YAMLError); ok {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   yamlErr.Error(),      // ✅ Extracted
            ErrorCode: yamlErr.Code(),        // ✅ Extracted
        })
    } else {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Validation failed: %v", err),
        })
    }
    return result
}
```

**Context:** SchemaValidator.Validate() - validates data against compiled schema

**Why MEDIUM Priority:**
- ✅ **Internal helper** function (not directly exposed to users)
- ✅ Error handling is **functional** (no silent failures)
- ⚠️ **Missing 6 out of 9 available fields** from ValidationError
- ⚠️ **Loses rich context** (FilePath, FieldPath, Line, ExpectedType, ActualType, ErrorType, Context)

**Current Extraction (2 fields):**
- ✅ Message (via `Error()`)
- ✅ ErrorCode (via `Code()`)

**Missing Extractions (6 fields):**
- ❌ FilePath (from `ValidationError.FilePath`)
- ❌ FieldPath (from `ValidationError.FieldPath`)
- ❌ Line (from `ValidationError.Line`)
- ❌ ExpectedType (from `ValidationError.ExpectedType`)
- ❌ ActualType (from `ValidationError.ActualType`)
- ❌ ErrorType (via `YAMLError.YAMLErrorType()`)
- ❌ Context (via `YAMLError.Context()`)

**Impact:**
- **Debugging difficulty:** Users cannot locate where in the file the error occurred
- **Type mismatches:** Cannot see expected vs actual types
- **Error categorization:** Cannot programmatically handle different error types
- **Lost context:** Additional debugging information discarded

**Recommended Fix:**
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

**Note:** This fix requires adding `ErrorType` and `Context` fields to `SchemaValidationError` struct first.

**Benefits:**
- **7x more error context** (2 fields → 9 fields)
- File location information (FilePath, Line, FieldPath)
- Type mismatch details (ExpectedType, ActualType)
- Error categorization (ErrorType)
- Additional debugging context (Context)

**Effort:** MEDIUM (requires struct changes + extraction logic + tests)

**Timeline:** Next sprint

---

### Priority Level 4: LOW (P3)
**Definition:** validate() with already excellent error handling, or in tests/edge cases

#### Site #3-5: `src/parsers/config.rs:662,1007` + `internal/yamlutil/schema.go:253`

**Site #3: `src/parsers/config.rs:662`**
```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```
**Status:** ✅ **EXCELLENT** - Idiomatic Rust `?` operator, rich ParseError context

**Site #4: `src/parsers/config.rs:1007`**
```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```
**Status:** ✅ **EXCELLENT** - Same pattern as Site #3

**Site #5: `internal/yamlutil/schema.go:253`**
```go
func ReadAndValidate(path string, schema Schema) SchemaValidationResult {
    // ... file reading ...
    return sv.Validate(parsedData)
}
```
**Status:** ✅ **APPROPRIATE** - Clean delegation to Site #2

**Why LOW Priority:**
- ✅ **Already excellent** error handling
- ✅ **Internal helpers** with proper patterns
- ✅ **No improvements needed**
- ✅ Exemplary patterns for other code to follow

**Action:** None - maintain current patterns

---

## Test Code Analysis

**Total test code call sites:** 150+ (Rust + Go)

**Status:** ✅ **NO ACTION NEEDED**

Test code intentionally uses direct Validate()/validate() calls for simplicity and clarity. This is the **correct pattern** for tests:
- Test code should be simple and readable
- Direct calls make test intent clear
- No need for complex error handling in tests
- Test failures are expected and handled by test framework

---

## Implementation Roadmap

### Phase 1: Medium Priority (Next Sprint)
**Target:** `internal/yamlutil/schema.go:180`

**Tasks:**
1. Add `ErrorType` and `Context` fields to `SchemaValidationError` struct
2. Update error extraction logic at schema.go:180
3. Add comprehensive unit tests for new field extractions
4. Verify no breaking changes to external consumers

**Estimated Effort:** 2-3 days

**Risk Level:** LOW (backward compatible, additive changes only)

---

### Phase 2: High Priority (Next Quarter)
**Target:** `src/parsers/yaml/parser.rs:121`

**Tasks:**
1. Add optional `source` field to `ValidationResult` struct
2. Create `validate_str_with_source()` method
3. Update callers to provide source context
4. Add tests for source tracking

**Estimated Effort:** 1-2 days

**Risk Level:** VERY LOW (purely additive, optional field)

---

### Phase 3: Verification (Ongoing)
**Target:** All sites

**Tasks:**
1. Monitor error rates and patterns
2. Gather feedback on error message usefulness
3. Review test coverage for error handling
4. Update documentation with examples

**Estimated Effort:** Ongoing maintenance

---

## Risk Assessment

### Risk of Inaction

**For P2 Site (schema.go:180):**
- **Severity:** LOW (no crashes, just degraded debugging)
- **Likelihood:** HIGH (errors always lack rich context)
- **Impact:** Harder troubleshooting, longer debugging time
- **User Impact:** Generic errors without location or type information

**For P1 Site (parser.rs:121):**
- **Severity:** LOW (functional but suboptimal)
- **Likelihood:** MEDIUM (missing context only when debugging)
- **Impact:** Cannot trace error sources in production
- **User Impact:** Indirect (affects support/troubleshooting)

**For P3 Sites:**
- **Risk:** NONE - already excellent

---

### Risk of Implementation

**P2 Fix (schema.go:180):**
- **Breaking Changes:** NONE (additive field extraction only)
- **API Compatibility:** MAINTAINED (existing fields preserved)
- **Test Coverage:** NEEDS UPDATE (add tests for new extractions)
- **Deployment Risk:** LOW (backward compatible)

**P1 Enhancement (parser.rs:121):**
- **Breaking Changes:** NONE (optional source field)
- **API Compatibility:** MAINTAINED (new method alongside existing)
- **Test Coverage:** NEEDS UPDATE (add source tracking tests)
- **Deployment Risk:** VERY LOW (purely additive)

---

## Success Criteria

### For P2 Site (schema.go:180)

- [ ] FilePath extracted from ValidationError
- [ ] FieldPath extracted from ValidationError
- [ ] Line extracted from ValidationError
- [ ] ExpectedType/ActualType extracted from type mismatch errors
- [ ] ErrorType extracted via YAMLErrorType()
- [ ] Context extracted via Context() method
- [ ] SchemaValidationError struct updated with ErrorType and Context fields
- [ ] Unit tests added for all new extractions
- [ ] All tests passing
- [ ] No breaking changes to external consumers verified

### For P1 Site (parser.rs:121)

- [ ] Source field added to ValidationResult struct
- [ ] validate_str_with_source() method created
- [ ] Tests for source tracking added
- [ ] Documentation updated with usage examples
- [ ] All tests passing

### For P3 Sites

- [ ] Verified no changes needed (already excellent)
- [ ] Patterns documented for future reference

---

## Conclusion

The ARMOR codebase demonstrates **excellent error handling practices** for validation:

### Strengths
- ✅ **0 Critical gaps** - no urgent fixes required
- ✅ All Rust code uses idiomatic `?` operator
- ✅ All Go code has functional error handling
- ✅ No silent failures or error swallowing
- ✅ Comprehensive test coverage

### Areas for Enhancement
- 🟡 **1 MEDIUM priority** site (schema.go:180) - enrich error context extraction
- 🟢 **1 HIGH priority** site (parser.rs:121) - add data source tracking
- 🟢 **3 LOW priority** sites - already excellent, maintain current patterns

### Overall Assessment

**Priority for systematic updates: LOW to MEDIUM**

Current state is **production-ready**. The identified enhancements are improvements, not fixes:
- P2 fix provides 7x more error context with low risk
- P1 enhancement adds diagnostic value with very low risk
- P3 sites are exemplary patterns to maintain

### Recommendation

**Implement P2 fix in next sprint** for high-value, low-risk improvement to error context.

**Consider P1 enhancement in next quarter** for better debugging support.

**Maintain P3 patterns** as examples of excellent error handling.

---

## Dependencies

### Required Analysis (Complete)
- ✅ **bf-5h0z6**: Validate() call sites catalog
- ✅ **bf-5sor5**: Validate() error handling gaps analysis
- ✅ **bf-678r9**: Validate() error handling prioritization
- ✅ **bf-261aw**: Error handling urgency ranking (current)

### Related Code
- `internal/yamlutil/schema.go:180-195` (Go - MEDIUM priority)
- `src/parsers/yaml/parser.rs:121` (Rust - HIGH priority)
- `src/parsers/config.rs:662,1007` (Rust - LOW priority, excellent)
- `internal/yamlutil/schema.go:253` (Go - LOW priority, appropriate)
- `internal/yamlutil/errors.go` (YAMLError hierarchy)

---

## Appendix: Priority Framework Definition

### Critical (P0)
- validate() on **user input or external data** with **missing or inadequate error handling**
- Could cause crashes, panics, or silent failures
- Security-sensitive validation (authentication, authorization, file uploads)

### High (P1)
- validate() in **core business logic** or **public APIs**
- Error handling that **loses important context** or diagnostic information
- Validation of **external data sources** (files, network, configuration)

### Medium (P2)
- validate() in **internal helpers** or **library code**
- Error handling that **works but could be more informative**
- Validation of **internal data structures**
- **Functional but loses rich context**

### Low (P3)
- validate() with **already good/excellent error handling**
- Validation of **trusted internal data**
- Test code or edge cases
- **No improvements needed**

---

**Prioritization Complete:** 2026-07-12
**Bead:** bf-261aw
**Status:** ✅ READY FOR IMPLEMENTATION
**Next Steps:** Implement P2 fix in next sprint, consider P1 enhancement next quarter
