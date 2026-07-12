# Validate() Call Sites - Error Handling Analysis

**Bead:** bf-4y58v  
**Generated:** 2026-07-12  
**Purpose:** Analyze Validate() call sites for error handling patterns and identify needed updates

---

## Executive Summary

Total Validate() call sites analyzed: **3**  
Sites requiring error handling updates: **1** (MEDIUM PRIORITY)  
Sites with adequate error handling: **2**

### Key Finding
The main call site at `internal/yamlutil/schema.go:180` has **basic error handling** but is **missing rich context extraction** from the YAMLError interface. While it correctly type-asserts and extracts basic information, it fails to capture several valuable fields available in the YAMLError hierarchy.

---

## Call Site Analysis

### Site 1: SchemaValidator.Validate() → Schema.Validate()

**Location:** `internal/yamlutil/schema.go:180`  
**Caller:** `SchemaValidator.Validate(data interface{})`  
**Callee:** `Schema.Validate(value interface{}) error` (implemented by `SchemaDefinition`)

**Current Error Handling Code:**
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    
    // Handle YAMLError with structured information
    if yamlErr, ok := err.(YAMLError); ok {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   yamlErr.Error(),
            ErrorCode: yamlErr.Code(),
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

**Error Handling Pattern:**
- ✅ Uses type assertion to check for YAMLError interface
- ✅ Extracts `Message` via `yamlErr.Error()` method
- ✅ Extracts `ErrorCode` via `yamlErr.Code()` method
- ✅ Provides fallback for generic errors
- ❌ **Missing:** FilePath extraction
- ❌ **Missing:** FieldPath extraction  
- ❌ **Missing:** Line/Column location extraction
- ❌ **Missing:** ErrorType extraction via `YAMLErrorType()`
- ❌ **Missing:** Context extraction via `Context()`
- ❌ **Missing:** Type mismatch information (ExpectedType/ActualType)

**Available Fields Not Extracted:**

From YAMLError interface:
- `YAMLErrorType()` - Returns error category (validation, type_mismatch, constraint, etc.)
- `Context()` - Returns additional context about the error state

From ValidationError struct (implements YAMLError):
- `FilePath` - Path to the file being validated
- `FieldPath` - Dot-notation path to the invalid field
- `Line` - Line number where error occurred (1-indexed)
- `Column` - Column number where error occurred (1-indexed)
- `Constraint` - Constraint that was violated
- `ExpectedType` - Expected type for type mismatch errors
- `ActualType` - Actual type found for type mismatch errors

From SchemaValidationError struct (target type):
- `FilePath` - Available but not populated
- `FieldPath` - Available but not populated
- `Line` - Available but not populated
- `Expected` - Available but not populated
- `Found` - Available but not populated

**Update Priority:** **MEDIUM**
- **Rationale:** The error handling is functional but loses valuable debugging context
- **Impact:** Users/receivers of SchemaValidationResult get incomplete error information
- **Risk:** Low - schema validation still works, just with less detailed error reporting
- **Benefit:** High - rich error context improves debugging and user experience

**Recommended Update:**
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    
    // Handle YAMLError with structured information
    if yamlErr, ok := err.(YAMLError); ok {
        svarErr := SchemaValidationError{
            Message:   yamlErr.Error(),
            ErrorCode: yamlErr.Code(),
        }
        
        // Extract additional context from ValidationError if available
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

---

### Site 2: SchemaValidator.ValidateFile() → SchemaValidator.Validate()

**Location:** `internal/yamlutil/schema.go:253`  
**Caller:** `SchemaValidator.ValidateFile(filePath string) SchemaValidationResult`  
**Callee:** `SchemaValidator.Validate(data interface{}) SchemaValidationResult`

**Current Code:**
```go
// Validate against schema
return sv.Validate(data)
```

**Error Handling:** N/A - Direct delegation, no error handling performed

**Update Priority:** **NONE**
- **Rationale:** Simple delegation to Validate() which handles all error processing
- **Impact:** None - errors are fully handled by the delegated method

---

### Site 3: Validator.ValidateStringWithPath()

**Location:** `internal/yamlutil/validator.go:110`  
**Caller:** N/A (public API)  
**Callee:** `Validator.ValidateStringWithPath(yamlContent, "<string>")`

**Current Code:**
```go
func (v *Validator) ValidateString(yamlContent string) ValidationResult {
    return v.ValidateStringWithPath(yamlContent, "<string>")
}
```

**Error Handling:** N/A - Different validation system (uses ValidationResult, not YAMLError)

**Update Priority:** **NONE**
- **Rationale:** Separate validation system that doesn't use YAMLError hierarchy
- **Impact:** None - out of scope for this analysis

---

## Error Handling Patterns Analysis

### Pattern 1: Type Assertion with Basic Extraction (Current)

**Used at:** `internal/yamlutil/schema.go:180-195`

**Characteristics:**
- Type asserts to YAMLError interface
- Extracts interface methods: `Error()`, `Code()`
- Provides fallback for non-YAMLError errors
- Returns early on error

**Strengths:**
- ✅ Type-safe error handling
- ✅ Graceful fallback for generic errors
- ✅ Maintains error flow control

**Weaknesses:**
- ❌ Underutilizes YAMLError interface (only 2 of 3 methods used)
- ❌ Misses struct-level context (FilePath, FieldPath, Line, Column)
- ❌ Loses type mismatch details (ExpectedType, ActualType)
- ❌ No error type categorization in output

### Pattern 2: Direct Delegation

**Used at:** `internal/yamlutil/schema.go:253`

**Characteristics:**
- Passes through to method with error handling
- No error processing at delegation point

**Strengths:**
- ✅ DRY - avoids duplicate error handling
- ✅ Single source of truth for error processing

**Weaknesses:**
- None - appropriate pattern for delegation

### Pattern 3: YAMLError Creation (Error Source)

**Used at:** `internal/yamlutil/schema.go:770+` (SchemaDefinition.Validate)

**Characteristics:**
- Creates YAMLError instances with rich context
- Uses structured error constructors (NewValidationError, NewTypeMismatchError, etc.)
- Populates detailed error fields

**Strengths:**
- ✅ Rich error context creation
- ✅ Consistent error type hierarchy
- ✅ Detailed error information available

**Weaknesses:**
- ❌ Information lost at call sites that don't extract it

---

## SchemaValidationError vs ValidationError Field Mapping

| ValidationError Field | SchemaValidationError Field | Currently Extracted? |
|----------------------|----------------------------|---------------------|
| FilePath              | FilePath                   | ❌ NO               |
| FieldPath             | FieldPath                  | ❌ NO               |
| Message               | Message                    | ✅ YES (via Error())|
| Line                  | Line                       | ❌ NO               |
| Column                | - (not in target)          | N/A                 |
| ErrorCode             | ErrorCode                  | ✅ YES (via Code()) |
| Type                  | ErrorType                  | ❌ NO               |
| ExpectedType          | Expected                   | ❌ NO               |
| ActualType            | Found                      | ❌ NO               |
| ContextStr            | - (not in target)          | N/A                 |

**Missed Extraction Opportunities:** 6 out of 9 available fields

---

## Criticality Assessment

### HIGH CRITICALITY
**None** - No sites with missing error handling that would cause incorrect behavior

### MEDIUM CRITICALITY
**Site:** `internal/yamlutil/schema.go:180`
- **Issue:** Incomplete error context extraction
- **Impact:** Degraded debugging experience and user-facing error messages
- **Risk:** Low - validation still functions correctly
- **Benefit:** High - significant improvement in error clarity and debuggability
- **Effort:** Low - straightforward field additions

### LOW CRITICALITY
**Site:** `internal/yamlutil/schema.go:253` (ValidateFile delegation)
- **Issue:** None - delegation is appropriate
- **Impact:** None
- **Action:** No changes needed

---

## Implementation Recommendations

### Phase 1: Enrich Error Context Extraction (MEDIUM Priority)

**File:** `internal/yamlutil/schema.go:180-195`

**Changes:**
1. Add type assertion to `*ValidationError` to extract struct fields
2. Populate `FilePath`, `FieldPath`, `Line` in SchemaValidationError
3. Populate `Expected` and `Found` from type mismatch errors
4. Add `ErrorType` field extraction from `YAMLErrorType()`
5. Consider adding `Context` field to SchemaValidationError struct

**Estimated Effort:** 30 minutes

**Testing:**
- Unit test with validation errors containing file paths
- Unit test with field path errors
- Unit test with type mismatch errors
- Verify SchemaValidationResult contains full context

### Phase 2: Consider SchemaValidationError Schema Enhancement (LOW Priority)

**File:** `internal/yamlutil/errors.go` (SchemaValidationError struct)

**Proposed Addition:**
```go
type SchemaValidationError struct {
    FilePath    string    // Path to the file being validated
    SchemaPath  string    // Path to the schema file
    FieldPath   string    // Dot-notation path to the invalid field
    Message     string    // Description of the validation failure
    Expected    string    // What was expected by the schema
    Found       string    // What was actually found
    Line        int       // Line number where validation failed
    Column      int       // Column number where validation failed
    ErrorCode   ErrorCode // Error code for programmatic handling
    ErrorType   string    // Error type category (NEW)
    Context     string    // Additional context (NEW)
}
```

**Estimated Effort:** 15 minutes (struct change) + 15 minutes (update extraction)

### Phase 3: Documentation Updates

**Updates needed:**
1. Add godoc comments showing YAMLError extraction pattern
2. Document available error fields in SchemaValidationResult
3. Add example showing full error context usage

**Estimated Effort:** 30 minutes

---

## Risk Assessment

### Current State Risks
1. **Low:** Error messages lack context for debugging
2. **Low:** Users cannot identify error location (file/line/column)
3. **Low:** Type mismatch errors don't show expected vs actual types

### Update Risks
1. **Low:** Adding field extraction is additive (no breaking changes)
2. **Low:** SchemaValidationError struct expansion is non-breaking
3. **Medium:** If SchemaValidationError is used externally, struct changes could affect consumers (check imports)

### Mitigation Strategies
1. Run `grep -r "SchemaValidationError" --include="*.go"` to find external usage
2. Add fields in non-breaking way (ensure existing code continues to work)
3. Add tests for new field extractions

---

## Dependencies

### Required Documentation
- ✅ **bf-52zl8**: Validate() call sites catalog (COMPLETE)
- ✅ **bf-cdc05**: Validate() call sites documentation (COMPLETE)

### Related Work
- Error type hierarchy implementation (existing)
- YAMLError interface definition (existing)
- SchemaValidationResult type (existing)

### Blocking Work
None - this analysis can proceed independently

---

## Test Coverage Analysis

### Current Test Coverage for Error Handling

**File:** `internal/yamlutil/schema_validation_test.go`

**Test Coverage:**
- ✅ Tests for basic validation error detection
- ✅ Tests for error code extraction
- ❌ **No tests for FilePath extraction** (not implemented)
- ❌ **No tests for FieldPath extraction** (not implemented)
- ❌ **No tests for Line/Column extraction** (not implemented)
- ❌ **No tests for ErrorType extraction** (not implemented)

### Recommended Test Additions

```go
func TestSchemaValidationErrorContextExtraction(t *testing.T) {
    schema := &SchemaDefinition{
        RootFields: map[string]*FieldDefinition{
            "name": {Type: "string", Required: true},
        },
    }
    
    validator := NewSchemaValidator(schema)
    
    // Test with file-based validation
    result := validator.ValidateFile("testdata/invalid.yaml")
    
    if !result.Valid {
        for _, err := range result.Errors {
            assert.NotEmpty(t, err.FilePath, "FilePath should be populated")
            assert.NotEmpty(t, err.ErrorCode, "ErrorCode should be populated")
            // After implementation:
            // assert.NotEmpty(t, err.FieldPath, "FieldPath should be populated")
            // assert.NotEmpty(t, err.ErrorType, "ErrorType should be populated")
        }
    }
}
```

---

## Success Criteria

### For Site 1 (schema.go:180)
- [ ] FilePath extracted from ValidationError
- [ ] FieldPath extracted from ValidationError  
- [ ] Line/Column extracted from ValidationError
- [ ] ErrorType extracted via YAMLErrorType()
- [ ] Expected/Found extracted from type mismatch errors
- [ ] Context extracted via Context() method
- [ ] Unit tests added for new extractions
- [ ] Documentation updated with extraction pattern

### For Documentation
- [ ] Error handling analysis document created
- [ ] Update priorities documented with rationale
- [ ] Implementation recommendations provided
- [ ] Risk assessment completed
- [ ] Test coverage gaps identified

---

## Conclusion

The ARMOR Go codebase has **3 Validate() call sites** with varying error handling completeness:

1. **Site 1 (schema.go:180)**: Has basic YAMLError handling but misses 6 of 9 available fields
   - **Priority:** MEDIUM - valuable improvement for debugging
   - **Effort:** Low - straightforward field additions
   - **Risk:** Low - additive changes only

2. **Site 2 (schema.go:253)**: Delegation pattern - no changes needed
   - **Priority:** NONE
   - **Effort:** N/A
   - **Risk:** N/A

3. **Site 3 (validator.go:110)**: Different validation system - out of scope
   - **Priority:** NONE
   - **Effort:** N/A
   - **Risk:** N/A

**Recommendation:** Implement MEDIUM priority updates to Site 1 to provide richer error context to users and improve debugging experience. The changes are low-risk and high-benefit.

---

**Next Steps:**
1. Review and approve this analysis
2. Implement Phase 1 (enrich error context extraction)
3. Add unit tests for new field extractions
4. Update documentation
5. Consider Phase 2 (SchemaValidationError schema enhancement)

---

**Analysis Complete:** 2026-07-12  
**Bead:** bf-4y58v  
**Status:** READY FOR IMPLEMENTATION
