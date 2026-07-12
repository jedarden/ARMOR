# Validate() Error Handling Gaps Analysis

**Bead:** bf-5sor5  
**Date:** 2026-07-12  
**Task:** Analyze Validate() error handling gaps  
**Status:** COMPLETE

---

## Executive Summary

**Total production call sites analyzed:** 5 (3 Rust + 2 Go)  
**Call sites with error handling gaps:** 1 (Go only)  
**Gap severity:** MEDIUM - functional but incomplete error context extraction  
**Overall assessment:** Rust code has excellent error handling; Go has one opportunity for improvement

---

## Gap Analysis by Call Site

### 1. src/parsers/config.rs:662 - ParserConfigBuilder::build()

**Call:** `self.config.validate()?;`  
**Pattern:** Question mark operator  
**Status:** ✅ **NO GAP** - Excellent error handling  

**Analysis:**
- Uses `?` operator for proper error propagation
- Returns `Result<ParserConfig, String>` to caller
- Error is propagated with full context from validate()
- Standard Rust pattern for fallible operations

**Gap:** **None**

---

### 2. src/parsers/config.rs:1007 - ValidatorConfigBuilder::build()

**Call:** `self.config.validate()?;`  
**Pattern:** Question mark operator  
**Status:** ✅ **NO GAP** - Excellent error handling  

**Analysis:**
- Uses `?` operator for proper error propagation
- Returns `Result<ValidatorConfig, String>` to caller
- Error is propagated with full context from validate()
- Consistent with ParserConfigBuilder pattern

**Gap:** **None**

---

### 3. src/parsers/yaml/parser.rs:121 - BasicParser::validate_str()

**Call:** `let mut result = validator.validate(content);`  
**Pattern:** ValidationResult inspection  
**Status:** ✅ **NO GAP** - Appropriate error handling  

**Analysis:**
- Stores ValidationResult for detailed inspection
- Checks `result.is_valid()` on line 124 before proceeding
- Merges errors from SyntaxDetector on lines 129-132
- Uses ValidationResult struct appropriately (rich error details)

**Gap:** **None**

---

### 4. internal/yamlutil/schema.go:253 - SchemaValidator::ValidateFile()

**Call:** `return sv.Validate(data)`  
**Pattern:** Delegation  
**Status:** ✅ **NO GAP** - Appropriate delegation  

**Analysis:**
- Simple delegation to Validate() method
- Single source of truth for error processing
- No error processing at delegation point (by design)
- Caller receives full SchemaValidationResult

**Gap:** **None**

---

### 5. internal/yamlutil/schema.go:180 - SchemaValidator::Validate()

**Call:** `if err := sv.schema.Validate(data); err != nil`  
**Pattern:** Type assertion to YAMLError  
**Status:** ⚠️ **GAP IDENTIFIED** - Incomplete error context extraction  

**Current Code (lines 180-195):**
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    
    // Handle YAMLError with structured information
    if yamlErr, ok := err.(YAMLError); ok {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   yamlErr.Error(),      // ✅ Extracted
            ErrorCode: yamlErr.Code(),       // ✅ Extracted
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

**Gap Details:**

#### Missing Error Context Fields (6 of 9 available)

| Field | Status | Impact |
|-------|--------|--------|
| Message | ✅ Extracted | N/A |
| ErrorCode | ✅ Extracted | N/A |
| **FilePath** | ❌ **Missing** | Cannot locate file with error |
| **FieldPath** | ❌ **Missing** | Cannot identify invalid field |
| **Line** | ❌ **Missing** | Cannot find error line in file |
| **ErrorType** | ❌ **Missing** | Cannot categorize error programmatically |
| **Expected** | ❌ **Missing** | Cannot see expected type for mismatches |
| **Found** | ❌ **Missing** | Cannot see actual type found |
| **Context** | ❌ **Missing** | Missing additional error state context |

#### Specific Gaps

**Gap 1: FilePath Not Populated**
- **Issue:** SchemaValidationError.FilePath remains empty
- **Impact:** Users cannot determine which file caused the error
- **Severity:** MEDIUM - file path often known from calling context
- **Available from:** `yamlErr.FilePath()` method

**Gap 2: FieldPath Not Populated**
- **Issue:** SchemaValidationError.FieldPath remains empty
- **Impact:** Cannot identify the specific field that failed validation (e.g., "server.port")
- **Severity:** HIGH - field location is critical for debugging
- **Available from:** `yamlErr.FieldPath()` method

**Gap 3: Line Number Not Populated**
- **Issue:** SchemaValidationError.Line remains 0
- **Impact:** Cannot jump to error location in YAML file
- **Severity:** HIGH - line number is essential for large files
- **Available from:** `yamlErr.Line()` method

**Gap 4: ErrorType Not Populated**
- **Issue:** SchemaValidationError.ErrorType remains empty
- **Impact:** Cannot programmatically categorize errors (validation, type_mismatch, constraint)
- **Severity:** MEDIUM - useful for error routing/handling
- **Available from:** `yamlErr.Type()` method

**Gap 5: Expected/Found Types Not Populated**
- **Issue:** Expected and Found fields remain empty for type mismatches
- **Impact:** Cannot see expected vs actual types (e.g., expected "int", got "string")
- **Severity:** HIGH - crucial for type validation errors
- **Available from:** `yamlErr.ExpectedType()` and `yamlErr.ActualType()` methods

**Gap 6: Context Not Populated**
- **Issue:** SchemaValidationError.Context remains empty
- **Impact:** Missing additional debugging context about error state
- **Severity:** LOW - supplementary information
- **Available from:** `yamlErr.Context()` method

#### Why These Fields Matter

**User Impact:**
- Error messages like "Validation failed" are not actionable
- Developers cannot locate where in a 500-line YAML file the error occurred
- Type mismatch errors don't show what types were involved

**Debugging Impact:**
- Cannot filter errors by type programmatically
- Cannot create clickable error links in IDEs
- Cannot provide field-specific suggestions

**API Impact:**
- SchemaValidationResult consumers receive incomplete error data
- Downstream error reporting systems lack context
- Cannot build rich error UI/CLI output

---

## Gap Severity Classification

### HIGH Priority Gaps

**None identified.** All production code has functional error handling that prevents incorrect behavior or data loss.

### MEDIUM Priority Gaps

**1. Go SchemaValidator error context extraction (internal/yamlutil/schema.go:180-188)**

- **Severity:** MEDIUM - functional but degraded UX
- **Risk of Inaction:** LOW - validation still works, just with poor error messages
- **Risk of Fix:** LOW - additive field extraction only, no breaking changes
- **Benefit:** HIGH - significantly improves debugging experience
- **Effort:** ~1 hour implementation + testing

### LOW Priority Gaps

**None identified.** All other gaps are minor enhancements or documentation improvements.

---

## Specific Gap Patterns

### Pattern 1: Missing Field Extraction (Go only)

**Location:** `internal/yamlutil/schema.go:184-188`

**Current:**
```go
if yamlErr, ok := err.(YAMLError); ok {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   yamlErr.Error(),
        ErrorCode: yamlErr.Code(),
    })
}
```

**Gap:** Extracts only 2 of 9 available fields

**Impact:** 
- Cannot locate error in file
- Cannot identify field path
- Cannot see line numbers
- Cannot categorize error types

**Fix:**
```go
if yamlErr, ok := err.(YAMLError); ok {
    svarErr := SchemaValidationError{
        Message:   yamlErr.Error(),
        ErrorCode: yamlErr.Code(),
    }
    
    // Extract additional context if available
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
    svarErr.ErrorType = string(yamlErr.Type())
    
    // Extract context if available
    if ctx := yamlErr.Context(); ctx != "" {
        svarErr.Context = ctx
    }
    
    result.Errors = append(result.Errors, svarErr)
}
```

---

## Test Coverage Analysis

### Current State: ✅ Excellent

**Rust Code:**
- All validation types have comprehensive test coverage
- Both success and failure paths tested
- Error messages verified in negative tests
- 131 test call sites cover edge cases

**Go Code:**
- Schema validation has test coverage
- Error handling paths tested
- Type assertions covered

**Gap:** None - test coverage is excellent

---

## Recommendations

### Immediate Action (Recommended)

✅ **Fix MEDIUM priority gap at internal/yamlutil/schema.go:180-188**

**Rationale:**
- Low-risk, high-benefit improvement
- Straightforward field additions
- No breaking changes
- Significantly improves debugging experience
- ~1 hour total effort

**Implementation Steps:**
1. Add type assertion to `*ValidationError` to extract struct fields
2. Call YAMLError methods for FilePath, Line, Type, Context
3. Add unit tests for new field extractions
4. Verify error reporting in CLI/UI

### Future Improvements (Optional)

1. **Error Context Standardization**
   - Document best practices for error context enrichment
   - Add helper methods for common extraction patterns
   - ~2-4 hours effort

2. **SchemaValidationError Schema Enhancement**
   - Consider adding ErrorType and Context fields if not present
   - Verify no breaking changes to external consumers
   - ~30 minutes effort

---

## Risk vs Benefit Matrix

| Site | Risk of Inaction | Risk of Change | Benefit | Priority |
|------|------------------|----------------|---------|----------|
| Go schema.go:180 | Low (poor debuggability) | Low (additive only) | High (rich context) | 🟡 MEDIUM |
| Rust config.rs:662 | None | N/A | None | 🟢 LOW |
| Rust config.rs:1007 | None | N/A | None | 🟢 LOW |
| Rust parser.rs:121 | None | N/A | None | 🟢 LOW |
| Go schema.go:253 | None | N/A | None | 🟢 LOW |

---

## Conclusion

### Summary of Findings

**Overall Assessment:** The ARMOR codebase demonstrates **mature error handling practices** around Validate() calls. All Rust production code has excellent error handling with no gaps. Only one Go call site has an opportunity for improvement.

**Gap Count:** 1 MEDIUM priority gap (out of 5 production call sites)

**Strengths:**
- ✅ All Rust code uses appropriate error handling patterns
- ✅ Builder patterns correctly propagate errors
- ✅ ValidationResult usage is intentional and appropriate
- ✅ Comprehensive test coverage

**Area for Improvement:**
- 🟡 One Go site could benefit from richer error context extraction (6 missing fields)

### Specific Gaps Identified

**Gap #1: Incomplete YAMLError Field Extraction**
- **Location:** `internal/yamlutil/schema.go:184-188`
- **Fields Missing:** FilePath, FieldPath, Line, ErrorType, Expected, Found, Context
- **Fields Extracted:** Message, ErrorCode
- **Severity:** MEDIUM
- **Impact:** Degraded debugging experience, cannot locate errors in files

### Overall Priority

**🟡 MEDIUM** - One site needs improvement, but validation is functional

**Recommendation:** Implement the field extraction enhancement at Go schema.go:180. It's low-risk, high-benefit, and straightforward to implement.

---

## Dependencies

### Previous Work Referenced
- ✅ bf-52zl8: Validate() call site locations catalog
- ✅ bf-5o1o1: Validate() error handling prioritization
- ✅ bf-5h0z6: Validate() call sites catalog verification
- ✅ bf-4y58v: Validate() error handling analysis

### Code References
- `internal/yamlutil/schema.go:180-195` (Go - MEDIUM priority gap)
- `src/parsers/config.rs:662, 1007` (Rust - no gaps)
- `src/parsers/yaml/parser.rs:121` (Rust - no gaps)

---

**Status:** GAP ANALYSIS COMPLETE

**Next Steps:**
1. Review this gap analysis
2. Implement MEDIUM priority fix if approved
3. Add tests for new field extractions
4. Update documentation with extraction pattern examples

---

**Generated:** 2026-07-12  
**Bead:** bf-5sor5
