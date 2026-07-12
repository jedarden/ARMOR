# Validate() Error Handling - Prioritized Actions

**Bead:** bf-4y58v  
**Generated:** 2026-07-12  
**Status:** Analysis Complete

---

## Priority Matrix

| Priority | Site | File | Line | Action | Effort | Impact |
|----------|------|------|------|--------|--------|--------|
| **MEDIUM** | SchemaValidator.Validate() | internal/yamlutil/schema.go | 180-195 | Enrich error context extraction | 30 min | High |
| **NONE** | ValidateFile() | internal/yamlutil/schema.go | 253 | No changes needed | N/A | N/A |
| **NONE** | Validator.ValidateStringWithPath() | internal/yamlutil/validator.go | 110 | Out of scope | N/A | N/A |

---

## MEDIUM Priority: SchemaValidator.Validate() Error Context Enrichment

### Location
**File:** `internal/yamlutil/schema.go`  
**Lines:** 180-195

### Current State
```go
if yamlErr, ok := err.(YAMLError); ok {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   yamlErr.Error(),
        ErrorCode: yamlErr.Code(),
    })
}
```

**Extracts:** 2 of 9 available fields (22%)

### Missing Fields
1. **FilePath** - File being validated
2. **FieldPath** - Path to invalid field  
3. **Line** - Line number of error
4. **ErrorType** - Error category (validation, type_mismatch, etc.)
5. **Expected** - Expected type/value
6. **Found** - Actual type/value found

### Recommended Changes

```go
if yamlErr, ok := err.(YAMLError); ok {
    svarErr := SchemaValidationError{
        Message:   yamlErr.Error(),
        ErrorCode: yamlErr.Code(),
    }
    
    // Extract struct-level context if available
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
    
    result.Errors = append(result.Errors, svarErr)
}
```

**Post-update extraction:** 8 of 9 available fields (89%)

### Implementation Steps
1. ✅ Analysis complete
2. ⏳ Update error extraction code
3. ⏳ Add unit tests for new field extractions
4. ⏳ Test with invalid YAML files
5. ⏳ Update documentation

### Safety Assessment
- ✅ No external imports of yamlutil package
- ✅ SchemaValidationError only used internally
- ✅ Changes are additive (no breaking changes)
- ✅ Low risk, high benefit

---

## NONE Priority Sites

### Site 1: ValidateFile() Delegation
**Status:** ✅ CORRECT - Appropriate delegation pattern  
**Action:** None

### Site 2: Validator.ValidateStringWithPath()
**Status:** ✅ OUT OF SCOPE - Different validation system  
**Action:** None

---

## Impact Analysis

### User Impact
**Before:** Users see basic error messages:
```
"validation error: field 'replicas' failed validation"
```

**After:** Users see rich error context:
```
"validation error in deployment.yaml at line 15: 
 field 'spec.replicas' failed validation (expected: int, found: string)"
```

### Debugging Impact
**Before:** Limited error context for troubleshooting  
**After:** Full error context including file, line, field path, and type information

### API Impact
**Before:** SchemaValidationResult with minimal error fields populated  
**After:** SchemaValidationResult with comprehensive error context

---

## Test Coverage Requirements

### Existing Tests
- ✅ Basic validation error detection
- ✅ Error code extraction

### Required New Tests
1. **FilePath extraction test**
2. **FieldPath extraction test**
3. **Line number extraction test**
4. **ErrorType extraction test**
5. **Type mismatch (Expected/Found) extraction test**

### Test Template
```go
func TestSchemaValidationErrorContextExtraction(t *testing.T) {
    schema := &SchemaDefinition{
        RootFields: map[string]*FieldDefinition{
            "replicas": {Type: "int", Required: true},
        },
    }
    
    validator := NewSchemaValidator(schema)
    result := validator.ValidateFile("testdata/invalid-replicas.yaml")
    
    assert.False(t, result.Valid)
    assert.Len(t, result.Errors, 1)
    
    err := result.Errors[0]
    assert.NotEmpty(t, err.FilePath, "FilePath should be populated")
    assert.NotEmpty(t, err.FieldPath, "FieldPath should be populated")  
    assert.Greater(t, err.Line, 0, "Line should be populated")
    assert.NotEmpty(t, err.ErrorCode, "ErrorCode should be populated")
    assert.NotEmpty(t, err.ErrorType, "ErrorType should be populated")
}
```

---

## Success Metrics

### Completion Criteria
- [ ] All 6 missing fields extracted from ValidationError
- [ ] Unit tests added for each field extraction
- [ ] Documentation updated
- [ ] All tests passing
- [ ] No breaking changes to external code

### Quality Metrics
- **Field extraction coverage:** Target 89% (8 of 9 fields)
- **Test coverage:** 100% of new extraction paths
- **Breaking changes:** 0
- **Documentation:** Complete with examples

---

## Timeline Estimate

| Task | Time |
|------|------|
| Code implementation | 30 min |
| Unit test creation | 30 min |
| Testing and verification | 15 min |
| Documentation updates | 15 min |
| **Total** | **90 min** |

---

## Next Steps

1. ✅ **Analysis phase** - COMPLETE
2. ⏳ **Implementation** - READY TO START
3. ⏳ **Testing** - PENDING
4. ⏳ **Documentation** - PENDING
5. ⏳ **Code review** - PENDING

---

## Risk Summary

### Implementation Risks
- **Low:** Field extraction is straightforward
- **Low:** No external dependencies on SchemaValidationError
- **Low:** Additive changes only (no removals)

### Mitigation
- ✅ Confirmed no external imports of yamlutil
- ✅ Confirmed SchemaValidationError is internal-only
- ✅ Comprehensive test coverage planned
- ✅ Documentation updates planned

---

## Conclusion

**Status:** ✅ Analysis complete, ready for implementation

**Key Finding:** One MEDIUM priority site requires error context enrichment. Low-risk, high-benefit improvement that will significantly enhance error reporting and debugging capabilities.

**Recommendation:** Proceed with MEDIUM priority implementation at `internal/yamlutil/schema.go:180-195`

---

**Bead:** bf-4y58v  
**Analysis Date:** 2026-07-12  
**Next Action:** Implementation (if approved)
