# Validate() Error Handling Gaps Analysis

**Bead:** bf-5sor5
**Task:** Analyze Validate() error handling gaps
**Date:** 2026-07-12
**Status:** COMPLETE

---

## Executive Summary

**Total production call sites analyzed:** 6 (3 Rust + 3 Go)
**Sites with error handling gaps:** 1 (MEDIUM PRIORITY)
**Sites with adequate/excellent error handling:** 5

### Key Finding

The ARMOR codebase demonstrates **mature error handling practices**. All Rust production code has excellent error handling using the `?` operator for propagation. Only one Go call site at `internal/yamlutil/schema.go:180` has an opportunity for improvement: it extracts basic error information but misses rich context fields available in the YAMLError hierarchy.

---

## Detailed Analysis by Call Site

### Rust Code Analysis

#### Site 1: ParserConfigBuilder::build() → ParserConfig::validate()
**Location:** `src/parsers/config.rs:662`
**Code:** `self.config.validate()?`

**Error Handling Pattern:** ✅ **EXCELLENT**
```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Analysis:**
- ✅ Uses `?` operator for proper error propagation
- ✅ Returns Result type with error context
- ✅ No silent failures or error swallowing
- ✅ Idiomatic Rust error handling
- ✅ Callers must handle validation failures

**Gap Assessment:** **NONE** - Excellent error handling

---

#### Site 2: ValidatorConfigBuilder::build() → ValidatorConfig::validate()
**Location:** `src/parsers/config.rs:1007`
**Code:** `self.config.validate()?`

**Error Handling Pattern:** ✅ **EXCELLENT**
```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Analysis:**
- ✅ Uses `?` operator for proper error propagation
- ✅ Returns Result type with error context
- ✅ Consistent pattern with Site 1
- ✅ Builders validate before returning completed config
- ✅ Prevents invalid configurations from being used

**Gap Assessment:** **NONE** - Excellent error handling

---

#### Site 3: BasicParser::validate_str() → SyntaxValidator::validate()
**Location:** `src/parsers/yaml/parser.rs:121`
**Code:** `let mut result = validator.validate(content);`

**Error Handling Pattern:** ✅ **GOOD**
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

**Analysis:**
- ✅ Uses ValidationResult struct directly (not an error type)
- ✅ Intentional pattern - ValidationResult is the return type, not an error
- ✅ Handles validation results structurally (not via error propagation)
- ✅ Combines multiple validation sources
- ✅ Returns comprehensive validation result

**Gap Assessment:** **NONE** - Appropriate pattern for validation-focused API

---

### Go Code Analysis

#### Site 1: SchemaValidator::Validate() → Schema::Validate()
**Location:** `internal/yamlutil/schema.go:180`
**Code:** `if err := sv.schema.Validate(data); err != nil`

**Error Handling Pattern:** ⚠️ **INCOMPLETE - MEDIUM PRIORITY**
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

**Available Fields NOT Extracted:**

From ValidationError struct (available via type assertion to `*ValidationError`):
- ❌ `FilePath` - Path to the file being validated
- ❌ `FieldPath` - Dot-notation path to the invalid field
- ❌ `Line` - Line number where error occurred (1-indexed)
- ❌ `ExpectedType` - Expected type for type mismatch errors
- ❌ `ActualType` - Actual type found for type mismatch errors

From YAMLError interface methods:
- ❌ `YAMLErrorType()` - Returns error category (validation, type_mismatch, constraint)
- ❌ `Context()` - Returns additional context about the error state

**Extraction Gap:** **6 out of 9 available fields missing**

**Impact:**
- Debugging difficulty: Users cannot locate where in the file the error occurred
- Type mismatches: Cannot see expected vs actual types
- Error categorization: Cannot programmatically handle different error types
- Lost context: Additional debugging information discarded

**Gap Assessment:** **MEDIUM PRIORITY** - Functional but loses valuable context

---

#### Site 2: SchemaValidator::ValidateFile() → SchemaValidator::Validate()
**Location:** `internal/yamlutil/schema.go:253`
**Code:** `return sv.Validate(data)`

**Error Handling Pattern:** ✅ **APPROPRIATE**
```go
func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult {
    data, err := os.ReadFile(filePath)
    if err != nil {
        return SchemaValidationResult{
            Valid: false,
            Errors: []SchemaValidationError{
                {Message: fmt.Sprintf("Failed to read file: %v", err)},
            },
        }
    }

    return sv.Validate(data)
}
```

**Analysis:**
- ✅ Simple delegation pattern - appropriate for this context
- ✅ Single source of truth for error processing (in Validate())
- ✅ No error processing needed at delegation point
- ✅ Clean separation of concerns

**Gap Assessment:** **NONE** - Appropriate delegation pattern

---

#### Site 3: Validator::ValidateString() → Validator::ValidateStringWithPath()
**Location:** `internal/yamlutil/validator.go:110`
**Code:** `return v.ValidateStringWithPath(yamlContent, "<string>")`

**Error Handling Pattern:** ✅ **OUT OF SCOPE**
```go
func (v *Validator) ValidateString(yamlContent string) ValidationResult {
    return v.ValidateStringWithPath(yamlContent, "<string>")
}
```

**Analysis:**
- ✅ Different validation system (uses ValidationResult, not YAMLError)
- ✅ Simple delegation with placeholder path
- ✅ Not related to Schema validation error handling
- ✅ Out of scope for this analysis

**Gap Assessment:** **NONE** - Different validation system

---

## Summary of Error Handling Gaps

### MEDIUM Priority Gap

**Location:** `internal/yamlutil/schema.go:180-195`

**Current Extraction (2 fields):**
- ✅ Message (via `Error()`)
- ✅ ErrorCode (via `Code()`)

**Missing Extractions (6 fields):**
1. ❌ FilePath (from `ValidationError.FilePath`)
2. ❌ FieldPath (from `ValidationError.FieldPath`)
3. ❌ Line (from `ValidationError.Line`)
4. ❌ ExpectedType (from `ValidationError.ExpectedType`)
5. ❌ ActualType (from `ValidationError.ActualType`)
6. ❌ ErrorType (via `YAMLError.YAMLErrorType()`)
7. ❌ Context (via `YAMLError.Context()`)

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

---

## Error Handling Pattern Analysis

### Pattern 1: Rust `?` Operator (Excellent)
**Usage:** Rust production code
**Characteristics:**
- Propagates errors immediately to caller
- Preserves full error context
- Forces caller to handle errors
- Zero-cost abstraction at compile time

**Assessment:** ✅ **EXCELLENT** - Idiomatic Rust, no gaps

---

### Pattern 2: Type Assertion with Partial Extraction (Incomplete)
**Usage:** `internal/yamlutil/schema.go:180`
**Characteristics:**
- Type-asserts to YAMLError interface
- Extracts only interface methods (Error, Code)
- Misses struct-level fields (FilePath, Line, etc.)
- Provides fallback for generic errors

**Strengths:**
- ✅ Type-safe error handling
- ✅ Graceful fallback for non-YAMLError errors
- ✅ Maintains error flow control
- ✅ Extracts basic information

**Weaknesses:**
- ❌ Underutilizes available error information (2 of 9 fields)
- ❌ Loses file location context
- ❌ Loses type mismatch details
- ❌ Loses error categorization

**Assessment:** ⚠️ **INCOMPLETE** - Functional but loses context

---

### Pattern 3: Direct Delegation (Appropriate)
**Usage:** `internal/yamlutil/schema.go:253`
**Characteristics:**
- Passes through to method with error handling
- No error processing at delegation point
- Single source of truth for error processing

**Assessment:** ✅ **APPROPRIATE** - Clean separation of concerns

---

### Pattern 4: ValidationResult Return Type (Good)
**Usage:** `src/parsers/yaml/parser.rs:121`
**Characteristics:**
- Returns ValidationResult struct directly
- Not an error propagation pattern
- Validation-focused API design
- Combines multiple validation sources

**Assessment:** ✅ **GOOD** - Intentional pattern for validation APIs

---

## Testing Coverage Analysis

### Current Test Coverage

**Rust Code:** ✅ **EXCELLENT**
- Comprehensive test coverage in `src/parsers/config.rs`
- Tests for both success and failure cases
- Tests for strict vs lenient validation
- Tests for error propagation

**Go Code:** ⚠️ **PARTIAL**
- Basic validation error detection tested
- ErrorCode extraction tested
- **Gaps:**
  - ❌ No tests for FilePath extraction (not implemented)
  - ❌ No tests for FieldPath extraction (not implemented)
  - ❌ No tests for Line extraction (not implemented)
  - ❌ No tests for ErrorType extraction (not implemented)

**Test Coverage Gap:** Tests reflect current implementation (only extracts 2 fields)

---

## Risk Assessment

### Risk of Inaction

**For Go Site 1 (schema.go:180):**
- **Severity:** LOW
- **Likelihood:** HIGH (errors always lack rich context)
- **Impact:** Degraded debugging experience, harder troubleshooting
- **User Impact:** Users see generic errors without location or type information

**For All Other Sites:**
- **Severity:** NONE
- **Risk:** No action needed

### Risk of Fixing Go Site 1

- **Breaking Changes:** NONE (additive field extraction only)
- **API Compatibility:** MAINTAINED (existing fields preserved)
- **Test Coverage:** NEEDS UPDATE (add tests for new extractions)
- **Deployment Risk:** LOW (backward compatible)

---

## Recommendations

### Immediate Action (Recommended)

**Priority: MEDIUM**
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

### No Action Needed

**Rust Sites 1-3:** Excellent error handling, maintain current patterns

**Go Site 2 (schema.go:253):** Appropriate delegation, no changes needed

**Go Site 3 (validator.go:110):** Different validation system, out of scope

---

## Success Criteria

### For Go Site 1 (schema.go:180)

- [ ] FilePath extracted from ValidationError
- [ ] FieldPath extracted from ValidationError
- [ ] Line extracted from ValidationError
- [ ] ExpectedType/ActualType extracted from type mismatch errors
- [ ] ErrorType extracted via YAMLErrorType()
- [ ] Context extracted via Context() method
- [ ] SchemaValidationError struct updated (if needed)
- [ ] Unit tests added for new extractions
- [ ] All tests passing

### For Other Sites

- [ ] Verified no changes needed (Rust sites)
- [ ] Verified appropriate patterns (Go delegation sites)

---

## Conclusion

The ARMOR codebase has **excellent error handling** in production code:

### Strengths
- ✅ All Rust code uses idiomatic `?` operator for error propagation
- ✅ Go code has functional error handling with clear improvement path
- ✅ No critical missing error handling that would cause incorrect behavior
- ✅ Comprehensive test coverage for implemented patterns

### Areas for Improvement
- 🟡 One Go site (`internal/yamlutil/schema.go:180`) could extract richer error context
- 🟢 Optional: SchemaValidationError schema enhancement for ErrorType/Context fields
- 🟢 Optional: Enhanced test coverage for new extractions

### Overall Assessment

**Summary:**
- 🔴 HIGH priority gaps: **0 sites**
- 🟡 MEDIUM priority gaps: **1 site** (Go schema.go:180)
- 🟢 LOW/NO gaps: **5 sites**

**Recommendation:** Implement the MEDIUM priority update to enrich Go error context extraction. It's low-risk, high-benefit, and straightforward to implement. The change is purely additive (no breaking changes) and significantly improves debugging experience.

---

## Dependencies

### Required Documentation (Complete)
- ✅ **bf-52zl8**: Validate() call sites catalog
- ✅ **bf-4y58v**: Validate() error handling analysis
- ✅ **bf-5o1o1**: Validate() error handling prioritization
- ✅ **bf-5agz8**: Validate() call site context verification

### Related Code
- `internal/yamlutil/schema.go:180-195` (Go - MEDIUM priority)
- `src/parsers/config.rs:662, 1007` (Rust - EXCELLENT)
- `src/parsers/yaml/parser.rs:121` (Rust - GOOD)
- `internal/yamlutil/errors.go` (YAMLError hierarchy)

---

**Analysis Complete:** 2026-07-12
**Bead:** bf-5sor5
**Status:** READY FOR IMPLEMENTATION
