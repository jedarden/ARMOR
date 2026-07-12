# Validate() Error Handling Updates - Urgency Prioritization

**Bead:** bf-261aw
**Task:** Prioritize error handling updates by urgency
**Date:** 2026-07-12
**Status:** COMPLETE

---

## Executive Summary

**Total production call sites analyzed:** 5 (3 Rust + 2 Go)
**Call sites with error handling gaps:** 1 (Go only)
**Gap urgency:** CRITICAL - processes external YAML data with incomplete error context

**Key Finding:** Previous analysis classified the single gap as MEDIUM priority, but under the urgency-based framework (Critical/High/Medium/Low), this gap is **CRITICAL** because it processes external data (YAML files from disk) and provides incomplete error context to users.

---

## Urgency Framework

The prioritization framework classifies call sites by data source and usage context:

| Urgency | Criteria | Definition |
|---------|----------|------------|
| **CRITICAL 🔴** | User input or external data | validate() processes data from untrusted sources (files, network, user input) |
| **HIGH 🟠** | Core business logic | validate() in primary application logic paths |
| **MEDIUM 🟡** | Internal helpers | validate() in internal utilities and helpers |
| **LOW 🟢** | Tests or edge cases | validate() in test code or non-critical paths |

---

## Call Site Classification by Urgency

### CRITICAL 🔴 Urgency

#### 1. internal/yamlutil/schema.go:180 - SchemaValidator::Validate() [HAS GAP]

**Context:** Validates YAML data against schema
**Data Source:** External data (files read from disk via ValidateFile())
**Current Pattern:** Type assertion to YAMLError + incomplete field extraction
**Status:** ⚠️ **GAP IDENTIFIED** - Incomplete error context extraction

**Why CRITICAL:**
- Processes **external YAML data** from files on disk
- Users rely on error messages to fix YAML configuration files
- Missing fields (FilePath, FieldPath, Line, ExpectedType, ActualType) prevent effective debugging
- Cannot locate errors in multi-line YAML files without context
- Type mismatches don't show what types were involved

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

| Field | Status | Impact on External Data Processing |
|-------|--------|-----------------------------------|
| Message | ✅ Extracted | N/A |
| ErrorCode | ✅ Extracted | N/A |
| **FilePath** | ❌ **Missing** | Cannot identify which file has the error |
| **FieldPath** | ❌ **Missing** | Cannot locate invalid field in YAML structure |
| **Line** | ❌ **Missing** | Cannot jump to error location in YAML file |
| **ErrorType** | ❌ **Missing** | Cannot categorize errors programmatically |
| **Expected** | ❌ **Missing** | Cannot see expected type for mismatches |
| **Found** | ❌ **Missing** | Cannot see actual type found |
| **Context** | ❌ **Missing** | Missing additional debugging context |

**Impact on External Data Processing:**
- Users cannot determine which configuration file is invalid
- Developers cannot locate errors in 500+ line YAML files
- Type mismatch errors don't show expected vs actual types
- Cannot create clickable error links in IDEs or editors
- Cannot provide field-specific suggestions for fixes

**User Impact Scenarios:**
1. **Multi-file Projects:** User has 10 YAML config files, error says "validation failed" - which file?
2. **Large Config Files:** User has a 500-line config, error says "invalid field" - which line?
3. **Type Mismatches:** User provides `"port: abc"`, error says "validation failed" - expected what type?

---

#### 2. src/parsers/yaml/parser.rs:121 - BasicParser::validate_str() [NO GAP]

**Context:** Validates YAML content strings
**Data Source:** External data (YAML content from files or network)
**Current Pattern:** ValidationResult inspection
**Status:** ✅ **NO GAP** - Appropriate error handling

**Why CRITICAL:**
- Processes **external YAML content** (strings from files, network, user input)
- Comprehensive error handling with ValidationResult
- Properly merges errors from SyntaxDetector
- Checks `result.is_valid()` before proceeding

**Code (lines 121-133):**
```rust
// Run syntax validation
let mut result = validator.validate(content);

// If no errors from basic validation, run enhanced detection
if result.is_valid() {
    let mut detector = SyntaxDetector::new();
    let detector_result = detector.detect_to_validation_result(content);

    // Merge errors from detector
    if !detector_result.is_valid() {
        result.valid = false;
        result.errors.extend(detector_result.errors);
    }
}

result
```

**Assessment:** ✅ Excellent error handling for external data
- Uses ValidationResult for detailed error inspection
- Merges errors from multiple validation sources
- Returns rich error context to caller
- No gaps identified

---

### HIGH 🟠 Urgency

**None identified.**

No call sites in core business logic have error handling gaps. All Rust production code in this category has excellent error handling.

---

### MEDIUM 🟡 Urgency

#### 1. src/parsers/config.rs:662 - ParserConfigBuilder::build() [NO GAP]

**Context:** Builder validates internal configuration
**Data Source:** Internal (builder-constructed config)
**Current Pattern:** Question mark operator
**Status:** ✅ **NO GAP** - Excellent error handling

**Why MEDIUM:**
- Internal helper for core business logic (parser configuration)
- Validates internally constructed config objects, not external data
- Uses `?` operator for proper error propagation
- Error propagated with full context from validate()

**Code:**
```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Assessment:** ✅ Excellent error handling for internal helper

---

#### 2. src/parsers/config.rs:1007 - ValidatorConfigBuilder::build() [NO GAP]

**Context:** Builder validates internal configuration
**Data Source:** Internal (builder-constructed config)
**Current Pattern:** Question mark operator
**Status:** ✅ **NO GAP** - Excellent error handling

**Why MEDIUM:**
- Internal helper for core business logic (validator configuration)
- Validates internally constructed config objects, not external data
- Uses `?` operator for proper error propagation
- Consistent with ParserConfigBuilder pattern

**Code:**
```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Assessment:** ✅ Excellent error handling for internal helper

---

### LOW 🟢 Urgency

#### 1. internal/yamlutil/schema.go:253 - SchemaValidator::ValidateFile() [NO GAP]

**Context:** Delegates to Validate() method
**Data Source:** External (reads file from disk)
**Current Pattern:** Simple delegation
**Status:** ✅ **NO GAP** - Appropriate delegation

**Why LOW:**
- Simple delegation wrapper - no error processing by design
- Single source of truth for error processing at Validate() method
- Caller receives full SchemaValidationResult from delegate

**Code:**
```go
func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult {
    data, err := os.ReadFile(filePath)
    // ... error handling ...
    return sv.Validate(data)  // Delegates to Validate()
}
```

**Assessment:** ✅ Appropriate delegation pattern - no gap

---

## Urgency-Ranked Priority List

### Priority 1: CRITICAL 🔴 (Immediate Action Required)

**1. Enrich Go error context extraction at internal/yamlutil/schema.go:180**

- **Urgency:** CRITICAL - processes external YAML data
- **Gap:** Extracts only 2 of 9 available error fields
- **Impact:** Users cannot debug YAML file errors without file paths, line numbers, field paths, or type information
- **Risk of Inaction:** HIGH - degraded UX for external data processing, poor debugging experience
- **Risk of Fix:** LOW - additive field extraction only, no breaking changes
- **Benefit:** HIGH - significantly improves debugging for YAML file errors
- **Effort:** ~1 hour implementation + testing

**Implementation Priority:** **DO FIRST** - This is the only gap with CRITICAL urgency

---

### Priority 2: HIGH 🟠 (No Sites)

**No HIGH urgency gaps identified.** All core business logic call sites have excellent error handling.

---

### Priority 3: MEDIUM 🟡 (No Gaps)

**No MEDIUM urgency gaps identified.** All internal helper call sites have excellent error handling.

---

### Priority 4: LOW 🟢 (No Gaps)

**No LOW urgency gaps identified.** All other call sites have appropriate error handling.

---

## Comparison with Previous Analysis

### Previous Classification (bf-5o1o1)

The previous prioritization classified the single gap as **MEDIUM** priority based on a different framework:
- Risk of Inaction: LOW (validation still works, just poor messages)
- Risk of Change: LOW (additive only)
- Benefit: HIGH (improved debugging)

### New Classification (bf-261aw)

Under the urgency-based framework, this gap is reclassified as **CRITICAL**:
- **Data Source:** External YAML data from files
- **Usage Context:** User-facing YAML file validation
- **Impact:** Users cannot debug configuration errors without context

**Rationale for Reclassification:**

The previous framework assessed "risk vs benefit" but did not consider data source. The new framework prioritizes based on **what type of data is being validated**:

- External data validation errors require rich context for users to fix their files
- Internal validation errors (builders, helpers) have less stringent UX requirements
- The gap at schema.go:180 processes files from disk - this is user-facing external data
- Without file paths, line numbers, and field paths, users cannot locate errors in their YAML files

---

## Implementation Roadmap

### Phase 1: CRITICAL Priority (Immediate - Week 1)

**Task:** Enrich Go error context extraction

**Location:** `internal/yamlutil/schema.go:180-195`

**Changes Required:**
1. Add type assertion to `*ValidationError` to extract struct fields
2. Call YAMLError methods for additional context
3. Populate all available SchemaValidationError fields

**Code Changes:**
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false

    // Handle YAMLError with structured information
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
    } else {
        // Handle generic errors
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Validation failed: %v", err),
        })
    }
    return result
}
```

**Testing Required:**
1. Unit test with validation errors containing file paths
2. Unit test with field path errors
3. Unit test with type mismatch errors (ExpectedType/ActualType)
4. Unit test with error type extraction
5. Unit test with context extraction

**Effort Estimate:**
- Implementation: 30 minutes
- Testing: 30 minutes
- Documentation: 15 minutes
- **Total: ~1.25 hours**

**Impact:** Enables users to debug YAML file errors with complete context

---

### Phase 2: HIGH/MEDIUM/LOW Priority (Deferred)

**No gaps identified in HIGH, MEDIUM, or LOW priority categories.**

All other call sites have appropriate error handling for their urgency level.

---

## Testing Requirements

### For CRITICAL Priority Update (schema.go:180)

**Test Scenarios:**

1. **File Path Extraction:**
   ```go
   func TestFilePathExtraction(t *testing.T) {
       schema := &SchemaDefinition{...}
       validator := NewSchemaValidator(schema)
       result := validator.ValidateFile("testdata/invalid.yaml")

       assert.False(t, result.Valid)
       assert.NotEmpty(t, result.Errors[0].FilePath, "FilePath should be populated")
   }
   ```

2. **Field Path Extraction:**
   ```go
   func TestFieldPathExtraction(t *testing.T) {
       // Test that field paths are extracted for nested field errors
   }
   ```

3. **Type Mismatch Extraction:**
   ```go
   func TestTypeMismatchExtraction(t *testing.T) {
       // Test that ExpectedType and ActualType are populated
   }
   ```

4. **Line Number Extraction:**
   ```go
   func TestLineNumberExtraction(t *testing.T) {
       // Test that line numbers are extracted
   }
   ```

5. **Error Type Extraction:**
   ```go
   func TestErrorTypeExtraction(t *testing.T) {
       // Test that ErrorType is populated
   }
   ```

---

## Risk vs Benefit Matrix (Updated)

| Site | Urgency | Data Source | Gap? | Risk of Inaction | Risk of Change | Benefit | Priority |
|------|---------|-------------|------|------------------|----------------|---------|----------|
| Go schema.go:180 | 🔴 CRITICAL | External files | Yes | HIGH (poor UX for external data) | LOW (additive) | HIGH (rich context) | **1** |
| Rust parser.rs:121 | 🔴 CRITICAL | External data | No | None | N/A | None | N/A |
| Rust config.rs:662 | 🟡 MEDIUM | Internal helper | No | None | N/A | None | N/A |
| Rust config.rs:1007 | 🟡 MEDIUM | Internal helper | No | None | N/A | None | N/A |
| Go schema.go:253 | 🟢 LOW | Delegation | No | None | N/A | None | N/A |

---

## Recommendations

### Immediate Action (Required)

✅ **Implement CRITICAL priority update to Go schema.go:180**

**Rationale:**
- Processes external YAML data from files
- Missing error context prevents users from debugging configuration errors
- Low-risk, high-benefit improvement
- Straightforward field additions (~1 hour total effort)
- Significantly improves user experience for YAML file validation

### Future Work (Optional)

1. **SchemaValidationError schema enhancement**
   - Add ErrorType and Context fields if not present
   - Verify no breaking changes to external consumers
   - Effort: 30 minutes
   - Impact: Medium - better structured error data

2. **Error context standardization documentation**
   - Document YAMLError extraction pattern
   - Add examples of proper error context propagation
   - Effort: 2-4 hours
   - Impact: Medium - code consistency

---

## Conclusion

### Summary of Findings

**Overall Assessment:** The ARMOR codebase demonstrates **mature error handling practices** around Validate() calls. All Rust production code has excellent error handling. Only one Go call site has a gap, and it requires **CRITICAL** urgency attention under the data-source-based framework.

**Gap Count:** 1 CRITICAL priority gap (out of 5 production call sites)

**Strengths:**
- ✅ All Rust code uses appropriate error handling patterns
- ✅ Builder patterns correctly propagate errors
- ✅ ValidationResult usage is intentional and appropriate
- ✅ Comprehensive test coverage

**Area for Improvement:**
- 🔴 One Go site processes external data with incomplete error context extraction

### Urgency-Ranked Priorities

**🔴 CRITICAL (1 gap):**
- Go schema.go:180 - Extract full error context for external YAML data

**🟠 HIGH (0 gaps):**
- No gaps identified

**🟡 MEDIUM (0 gaps):**
- No gaps identified

**🟢 LOW (0 gaps):**
- No gaps identified

### Next Steps

1. ✅ Review this urgency-based prioritization
2. 🔴 Implement CRITICAL priority update to Go schema.go:180
3. Add tests for new field extractions
4. Update documentation with extraction pattern examples

---

## Dependencies

### Previous Work Referenced
- ✅ bf-52zl8: Validate() call site locations catalog
- ✅ bf-5o1o1: Validate() error handling prioritization (MEDIUM framework)
- ✅ bf-5sor5: Validate() error handling gaps analysis
- ✅ bf-5h0z6: Validate() call sites catalog verification
- ✅ bf-4y58v: Validate() error handling analysis

### Code References
- `internal/yamlutil/schema.go:180-195` (Go - 🔴 CRITICAL priority gap)
- `src/parsers/config.rs:662, 1007` (Rust - 🟡 MEDIUM urgency, no gaps)
- `src/parsers/yaml/parser.rs:121` (Rust - 🔴 CRITICAL urgency, no gap)
- `internal/yamlutil/schema.go:253` (Go - 🟢 LOW urgency, no gap)

---

**Status:** URGENCY PRIORITIZATION COMPLETE

**Next Steps:**
1. Implement CRITICAL priority fix at Go schema.go:180
2. Add comprehensive tests for all 9 error fields
3. Verify error reporting in CLI/UI
4. Document extraction pattern for future reference

---

**Generated:** 2026-07-12
**Bead:** bf-261aw
