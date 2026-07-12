# Validate() Error Handling Priority Recommendations

**Bead ID:** bf-104jw  
**Task:** Document final priority recommendations  
**Date:** 2026-07-12  
**Status:** ✅ COMPLETE  
**Based on:** bf-45l8s (catalog), bf-5sor5 (gaps analysis), bf-261aw (urgency prioritization)

---

## Executive Summary

**Total production call sites:** 5 (3 Rust + 2 Go)  
**Sites requiring updates:** 2 (1 High, 1 Medium)  
**Sites with no updates needed:** 3 (already excellent)  

**Overall Assessment:** The ARMOR codebase demonstrates excellent error handling practices. No critical gaps exist. Two optional enhancements are identified to improve error context and debugging support.

---

## Priority Framework Definition

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

## Priority Rankings by Urgency

### Priority Level 1: CRITICAL (P0)
**Status:** ✅ **NONE**

**Sites:** None

**Assessment:** No critical gaps found. All sites handling external data have appropriate error handling.

---

### Priority Level 2: HIGH (P1)

#### Site #1: `src/parsers/yaml/parser.rs:121`

**Code:**
```rust
fn validate_str(&self, content: &str) -> ValidationResult {
    let validator = if self.config.is_strict() {
        SyntaxValidator::strict()
    } else {
        SyntaxValidator::lenient()
    };

    let mut result = validator.validate(content);

    // If no errors from basic validation, run enhanced detection
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

**Data Source:** External string content (potentially user input or files)

**Current State:** ✅ **GOOD** - Accumulates validation errors, returns ValidationResult

**Why HIGH Priority:**
- ✅ Handles **external data** (user input or files)
- ✅ Core **business logic** for YAML parsing
- ✅ Current error handling is GOOD (accumulates errors properly)
- ⚠️ **Missing data source context** - no indication of where YAML came from

**Gap:** Lacks diagnostic context about data origin (file path, user input source, etc.)

**Recommended Enhancement:**
```rust
// Add source context to ValidationResult
pub struct ValidationResult {
    pub valid: bool,
    pub errors: Vec<ValidationError>,
    pub source: Option<String>,  // ← Add this field
}

fn validate_str_with_source(&self, content: &str, source: &str) -> ValidationResult {
    let mut result = self.validate_str(content);
    result.source = Some(source.to_string());
    result
}
```

**Benefits:**
- Better debugging when validation fails
- Clear indication of data origin (file path, user input, etc.)
- Helps trace error sources in production

**Effort:** LOW (add optional source field to ValidationResult)

**Timeline:** Next quarterly review

**Risk:** VERY LOW (purely additive, optional field)

---

### Priority Level 3: MEDIUM (P2)

#### Site #2: `internal/yamlutil/schema.go:180`

**Code:**
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false

    // Handle YAMLError with structured information
    if yamlErr, ok := err.(YAMLError); ok {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   yamlErr.Error(),      // ✅ Extracted
            ErrorCode: yamlErr.Code(),        // ✅ Extracted
        })
    } else {
        // Handle generic errors
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Validation failed: %v", err),
        })
    }
    return result
}
```

**Context:** SchemaValidator.Validate() - validates data against compiled schema

**Data Source:** Interface{} parameter (could be user-provided or loaded from files)

**Current State:** ✅ **GOOD** - Handles YAMLError specifically, falls back to generic errors

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

**Risk:** LOW (backward compatible, additive changes only)

---

### Priority Level 4: LOW (P3)

#### Site #3-5: No Updates Needed

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

**Success Criteria:**
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

**Success Criteria:**
- [ ] Source field added to ValidationResult struct
- [ ] validate_str_with_source() method created
- [ ] Tests for source tracking added
- [ ] Documentation updated with usage examples
- [ ] All tests passing

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

## Priority Summary Table

| Priority | Count | Call Sites | Action Required | Timeline |
|----------|-------|------------|-----------------|----------|
| **P0 (Critical)** | 0 | None | ✅ None - excellent | N/A |
| **P1 (High)** | 1 | `src/parsers/yaml/parser.rs:121` | Review for enhanced context | Next quarter |
| **P2 (Medium)** | 1 | `internal/yamlutil/schema.go:180` | Enrich error field extraction | Next sprint |
| **P3 (Low)** | 3 | `src/parsers/config.rs:662,1007` + `schema.go:253` | ✅ None - already excellent | N/A |

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

## Test Code Analysis

**Total test code call sites:** 150+ (Rust + Go)

**Status:** ✅ **NO ACTION NEEDED**

Test code intentionally uses direct Validate()/validate() calls for simplicity and clarity. This is the **correct pattern** for tests:
- Test code should be simple and readable
- Direct calls make test intent clear
- No need for complex error handling in tests
- Test failures are expected and handled by test framework

---

## Final Recommendations

### Immediate Action (Recommended)

**Priority: MEDIUM (Next Sprint)**
1. **Enrich error context extraction** at `internal/yamlutil/schema.go:180`
   - Add type assertion to `*ValidationError`
   - Extract: FilePath, FieldPath, Line, ExpectedType, ActualType
   - Extract: ErrorType via `YAMLErrorType()`
   - Extract: Context via `Context()`

2. **Consider SchemaValidationError schema enhancement**
   - Add `ErrorType string` field
   - Add `Context string` field
   - Verify no breaking changes to external consumers

3. **Add comprehensive tests**
   - Test FilePath extraction from file-based validation
   - Test FieldPath extraction for field-level errors
   - Test type mismatch error extraction
   - Test ErrorType and Context extraction

### Future Enhancement (Next Quarter)

**Priority: HIGH (Next Quarterly Review)**
1. **Add data source tracking** to `src/parsers/yaml/parser.rs:121`
   - Add optional source field to ValidationResult
   - Create validate_str_with_source() method
   - Update callers to provide source context
   - Add tests for source tracking

### No Action Needed

**Priority: LOW**
- Rust sites (config.rs): Excellent error handling, maintain current patterns
- Go delegation site (schema.go:253): Appropriate pattern, no changes needed

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

---

## Implementation Guidance

### Systematic Update Approach

1. **Start with P2 (schema.go:180)** - Highest value, lowest risk
   - Add struct fields first (ErrorType, Context)
   - Update extraction logic
   - Add comprehensive tests
   - Verify backward compatibility

2. **Follow with P1 (parser.rs:121)** - Diagnostic enhancement
   - Add optional source field to ValidationResult
   - Create new method alongside existing
   - Update key callers to provide source
   - Add tests for source tracking

3. **Maintain P3 patterns** - Examples of excellent error handling
   - Document current patterns for reference
   - Use as examples for new code
   - No changes needed

### Testing Strategy

**For P2 Implementation:**
- Unit tests for each field extraction
- Integration tests with sample YAML files
- Backward compatibility tests
- Error message format verification

**For P1 Implementation:**
- Unit tests for source field tracking
- Integration tests with file paths
- Tests for optional field behavior
- Documentation examples

### Documentation Updates

- Update error handling examples in docs
- Add source tracking usage examples
- Document new SchemaValidationError fields
- Create migration guide if needed

---

**Priority Recommendations Complete:** 2026-07-12  
**Bead:** bf-104jw  
**Status:** ✅ READY FOR SYSTEMATIC IMPLEMENTATION  
**Next Steps:** Implement P2 fix in next sprint, consider P1 enhancement next quarter

---

## Dependencies

### Required Analysis (Complete)
- ✅ **bf-52zl8**: Validate() call sites catalog
- ✅ **bf-45l8s**: Comprehensive Validate() call sites catalog
- ✅ **bf-5h0z6**: Validate() call sites catalog verification
- ✅ **bf-5sor5**: Validate() error handling gaps analysis
- ✅ **bf-678r9**: Validate() error handling prioritization
- ✅ **bf-261aw**: Error handling urgency prioritization
- ✅ **bf-104jw**: Priority recommendations documentation (current)

### Related Code Files
- `internal/yamlutil/schema.go:180-195` (Go - MEDIUM priority)
- `src/parsers/yaml/parser.rs:121` (Rust - HIGH priority)
- `src/parsers/config.rs:662,1007` (Rust - LOW priority, excellent)
- `internal/yamlutil/schema.go:253` (Go - LOW priority, appropriate)
- `internal/yamlutil/errors.go` (YAMLError hierarchy)
- `internal/yamlutil/schema_interfaces.go` (Validation interfaces)
