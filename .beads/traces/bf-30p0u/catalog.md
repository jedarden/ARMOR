# ValidationError Instantiations Catalog

## Overview
This catalog documents all direct `ValidationError{}` struct instantiations found in the ARMOR codebase.

**Total instantiations found:** 11

**Summary:**
- **Path field PRESENT:** 6 (4 empty, 2 with values)
- **Path field MISSING:** 5
- **Missing Path field rate:** 45%

---

## Detailed Catalog

### 1. internal/yamlutil/validator.go:50
**Status:** ✅ Path field present (empty string)
```go
return ValidationError{
    FilePath:   ve.FilePath,
    Message:    ve.Message,
    ContextStr: ve.Context,
    Line:       ve.Line,
    Column:     ve.Column,
    Type:       ve.Type,
    Path:       "", // Path is context-specific and populated by caller if needed
}
```
**Context:** `ToValidationError()` method in `LocalValidationError` struct  
**Function:** Converts `LocalValidationError` to the standard `ValidationError`  
**Notes:** Path is explicitly set to empty string with comment explaining it's populated by caller

---

### 2. internal/yamlutil/errors.go:561
**Status:** ✅ Path field present (with value)
```go
return &ValidationError{
    FilePath:   filePath,
    Message:    message,
    FieldPath:  fieldPath,
    Constraint: constraint,
    ErrorCode:  errorCode,
    Line:       line,
    Column:     column,
    Type:       eType,
    Path:       validPath,
}
```
**Context:** `NewValidationError()` constructor function  
**Function:** Creates new ValidationError with proper initialization  
**Notes:** Path field is properly populated using `validPath` variable (derived from `path` or `fieldPath`)

---

### 3. internal/yamlutil/errors_test.go:33
**Status:** ❌ Path field MISSING
```go
err: &ValidationError{FilePath: "test.yaml"},
```
**Context:** Test case in `TestIsYAMLError`  
**Function:** Test data for testing YAML error type detection  
**Notes:** Minimal instantiation for test purposes, missing Path field

---

### 4. internal/yamlutil/errors_test.go:81
**Status:** ❌ Path field MISSING
```go
err: &ValidationError{FilePath: "test.yaml"},
```
**Context:** Test case in `TestGetYAMLErrorType`  
**Function:** Test data for testing error type extraction  
**Notes:** Minimal instantiation for test purposes, missing Path field

---

### 5. internal/yamlutil/errors_test.go:163
**Status:** ❌ Path field MISSING
```go
err: &ValidationError{FilePath: "test.yaml"},
```
**Context:** Test case in `TestIsParseError`  
**Function:** Test data for testing ParseError detection  
**Notes:** Minimal instantiation for test purposes, missing Path field

---

### 6. internal/yamlutil/validator_test.go:608-613
**Status:** ✅ Path field present (empty string)
```go
{
    Type:    ErrorTypeStructure,
    Message: "Duplicate key detected",
    Line:    5,
    Path:    "",
},
```
**Context:** Test data in `TestWarningSummary`  
**Function:** Test case with warnings  
**Notes:** Path explicitly set to empty string in test data

---

### 7. internal/yamlutil/validator_test.go:614-619
**Status:** ✅ Path field present (empty string)
```go
{
    Type:    ErrorTypeValidation,
    Message: "Deprecated YAML feature",
    Line:    10,
    Path:    "",
},
```
**Context:** Test data in `TestWarningSummary`  
**Function:** Second warning in test case  
**Notes:** Path explicitly set to empty string in test data

---

### 8. internal/yamlutil/validator_test.go:843-849
**Status:** ✅ Path field present (empty string)
```go
result.Warnings = append(result.Warnings, ValidationError{
    Type:     ErrorTypeStructure,
    Message:  "Test warning",
    FilePath: "test.yaml",
    Line:     1,
    Path:     "",
})
```
**Context:** Test code in `TestWarningSummary_SingleWarning`  
**Function:** Manually adding a warning to test WarningSummary  
**Notes:** Path explicitly set to empty string

---

### 9. internal/yamlutil/validator_test.go:872
**Status:** ❌ Path field MISSING
```go
ValidationError{Type: ErrorTypeStructure, Message: "Warning 1", FilePath: "test.yaml"},
```
**Context:** Test code in `TestWarningSummary_MultipleWarnings`  
**Function:** Appending first warning for testing  
**Notes:** Missing Path field in instantiation

---

### 10. internal/yamlutil/validator_test.go:873
**Status:** ❌ Path field MISSING
```go
ValidationError{Type: ErrorTypeStructure, Message: "Warning 2", FilePath: "test.yaml"},
```
**Context:** Test code in `TestWarningSummary_MultipleWarnings`  
**Function:** Appending second warning for testing  
**Notes:** Missing Path field in instantiation

---

### 11. internal/yamlutil/result.go:71
**Status:** ❌ Path field MISSING (Commented example)
```go
//	result := Err[Data](ValidationError{Message: "required field missing"})
```
**Context:** Documentation comment in `Err()` function  
**Function:** Example usage of Result error creation  
**Notes:** Commented example code demonstrating ValidationError usage, missing Path field

---

## Analysis Summary

### By File:
- **internal/yamlutil/validator.go**: 1 instantiation (Path present but empty)
- **internal/yamlutil/errors.go**: 1 instantiation (Path present with value)
- **internal/yamlutil/errors_test.go**: 3 instantiations (all missing Path)
- **internal/yamlutil/validator_test.go**: 5 instantiations (3 with Path, 2 missing)
- **internal/yamlutil/result.go**: 1 instantiation (commented example, missing Path)

### By Context:
- **Production code**: 2 instantiations, both have Path field present
- **Test code**: 8 instantiations, 5 missing Path field
- **Documentation**: 1 instantiation (commented), missing Path field

### Recommendations:
1. ✅ **Production code is compliant**: Both production instantiations properly include the Path field
2. ⚠️ **Test code inconsistency**: 5 of 8 test instantiations (62.5%) are missing the Path field
3. 📝 **Documentation example**: Should be updated to include Path field for completeness

### Notes:
- Empty slice declarations (`[]ValidationError{}`) and type declarations were excluded from this catalog
- Only direct struct instantiations with field assignments were counted
- Test code with minimal instantiations may intentionally omit optional fields for brevity
- The ToValidationError() method explicitly documents that Path is "populated by caller if needed"

---

## Generated Information
- **Date generated:** 2026-07-12
- **Bead ID:** bf-30p0u
- **Workspace:** /home/coding/ARMOR
- **Search pattern:** `ValidationError{`
- **Files searched:** All `*.go` files in the ARMOR codebase
