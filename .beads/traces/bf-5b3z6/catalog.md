# ValidationError Instantiations Catalog

## Overview

This catalog documents all direct `ValidationError{}` instantiations found in the ARMOR codebase, focusing on whether the `Path` field is present or missing.

**Total direct ValidationError instantiations found:** 9

**Summary:**
- ✅ **Has Path field:** 7 instantiations (78%)
- ❌ **Missing Path field:** 2 instantiations (22%)

**Note:** This count excludes:
- Empty slice declarations: `[]ValidationError{}`
- Constructor calls: `NewValidationError()`
- Related types: `SchemaValidationError{}`, `LocalValidationError{}`

---

## Detailed Catalog

### 1. validator.go

| Line | Context | Path Status | Notes |
|------|---------|-------------|-------|
| 50 | `ToValidationError()` function | ✅ **PRESENT** | `Path: ""` with comment "Path is context-specific and populated by caller if needed" |

**File:** `/home/coding/ARMOR/internal/yamlutil/validator.go`

```go
// Line 50-58
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

---

### 2. errors.go

| Line | Context | Path Status | Notes |
|------|---------|-------------|-------|
| 561 | `NewValidationError()` constructor | ✅ **PRESENT** | `Path: validPath` where `validPath = path` (may be empty but field is present) |

**File:** `/home/coding/ARMOR/internal/yamlutil/errors.go`

```go
// Line 561-571
return &ValidationError{
    FilePath:   filePath,
    Message:    message,
    FieldPath:  fieldPath,
    Constraint: constraint,
    ErrorCode:  errorCode,
    Line:       line,
    Column:     column,
    Type:       eType,
    Path:       validPath,  // ✅ Path field present
}
```

---

### 3. errors_test.go

| Line | Context | Path Status | Notes |
|------|---------|-------------|-------|
| 33 | `TestIsYAMLError` | ✅ **PRESENT** | Test fixture: `Path: ""` |
| 81 | `TestGetYAMLErrorType` | ✅ **PRESENT** | Test fixture: `Path: ""` |
| 163 | `TestIsParseError` | ✅ **PRESENT** | Test fixture: `Path: ""` |

**File:** `/home/coding/ARMOR/internal/yamlutil/errors_test.go`

```go
// Line 33
err: &ValidationError{FilePath: "test.yaml", Path: ""}

// Line 81
err: &ValidationError{FilePath: "test.yaml", Path: ""}

// Line 163
err: &ValidationError{FilePath: "test.yaml", Path: ""}
```

---

### 4. validator_test.go

| Lines | Context | Path Status | Notes |
|-------|---------|-------------|-------|
| 608-613 | `TestValidationResult_WarningSummary` | ✅ **PRESENT** | Array element with `Path: ""` |
| 614-619 | `TestValidationResult_WarningSummary` | ✅ **PRESENT** | Array element with `Path: ""` |
| 843 | `TestWarningSummary_SingleWarning` | ✅ **PRESENT** | Direct instantiation with `Path: ""` |
| 872 | `TestWarningSummary_MultipleWarnings` | ❌ **MISSING** | **BUG:** Only has Type, Message, FilePath |
| 873 | `TestWarningSummary_MultipleWarnings` | ❌ **MISSING** | **BUG:** Only has Type, Message, FilePath |

**File:** `/home/coding/ARMOR/internal/yamlutil/validator_test.go`

#### ✅ Has Path field (lines 608-619):
```go
Warnings: []ValidationError{
    {                           // Line 608
        Type:    ErrorTypeStructure,
        Message: "Duplicate key detected",
        Line:    5,
        Path:    "",  // ✅ Present
    },
    {                           // Line 614
        Type:    ErrorTypeValidation,
        Message: "Deprecated YAML feature",
        Line:    10,
        Path:    "",  // ✅ Present
    },
}
```

#### ✅ Has Path field (line 843):
```go
result.Warnings = append(result.Warnings, ValidationError{
    Type:     ErrorTypeStructure,
    Message:  "Test warning",
    FilePath: "test.yaml",
    Line:     1,
    Path:     "",  // ✅ Present
})
```

#### ❌ Missing Path field (lines 872-873):
```go
result.Warnings = append(result.Warnings,
    ValidationError{Type: ErrorTypeStructure, Message: "Warning 1", FilePath: "test.yaml"},  // ❌ Missing Path
    ValidationError{Type: ErrorTypeStructure, Message: "Warning 2", FilePath: "test.yaml"},  // ❌ Missing Path
)
```

---

### 5. result_types_test.go

All ValidationError references in this file are either:
- Empty slice declarations: `[]ValidationError{}`
- Constructor calls: `*NewValidationError(...)`

**No direct `ValidationError{}` instantiations found.**

---

### 6. validation_error_path_test.go

All ValidationError references use the `NewValidationError()` constructor.

**No direct `ValidationError{}` instantiations found.**

---

### 7. future.go

| Lines | Context | Path Status | Notes |
|-------|---------|-------------|-------|
| 495 | Empty slice declaration | N/A | `[]ValidationError{}` - not an instantiation |
| 496 | Empty slice declaration | N/A | `[]ValidationError{}` - not an instantiation |

**File:** `/home/coding/ARMOR/internal/yamlutil/future.go`

---

## Issues Found

### 🔴 Critical: Missing Path Field

**File:** `/home/coding/ARMOR/internal/yamlutil/validator_test.go`
**Lines:** 872-873

```go
ValidationError{Type: ErrorTypeStructure, Message: "Warning 1", FilePath: "test.yaml"},
ValidationError{Type: ErrorTypeStructure, Message: "Warning 2", FilePath: "test.yaml"},
```

**Impact:**
- These instantiations are missing the `Path` field
- This is inconsistent with the rest of the codebase
- May cause issues if the Path field is required in future validation logic

**Recommendation:**
Add `Path: ""` to both instantiations to match the pattern used elsewhere in the codebase.

---

## Files Without Issues

The following files have all ValidationError instantiations with the Path field properly set:

1. ✅ `/home/coding/ARMOR/internal/yamlutil/validator.go` - Line 50
2. ✅ `/home/coding/ARMOR/internal/yamlutil/errors.go` - Line 561
3. ✅ `/home/coding/ARMOR/internal/yamlutil/errors_test.go` - Lines 33, 81, 163
4. ✅ `/home/coding/ARMOR/internal/yamlutil/validator_test.go` - Lines 608-613, 614-619, 843-849

---

## Additional Notes

### Empty Slice Declarations

The following are **NOT** counted as ValidationError instantiations (they are slice type declarations):
- `[]ValidationError{}` - Empty slice literal (found in multiple files)
- `[]ValidationError{...}` - Slice with elements (constructor calls, not direct struct literals)

### Constructor Usage

The codebase consistently uses `NewValidationError()` constructor function for creating ValidationError instances with proper field initialization. This is the recommended pattern.

### Path Field Semantics

The `Path` field represents the YAML path to the field where the error occurred (e.g., `"spec.template.spec.containers[0].image"`). When empty, it indicates the error is at the root level or the path is unknown.

---

## Recommendations

1. **Fix the 2 missing Path fields** in `validator_test.go` lines 872-873
2. **Continue using `NewValidationError()` constructor** for new code
3. **Consider adding a lint rule** to ensure Path field is always present in ValidationError instantiations

---

## Summary Statistics

### Total ValidationError Instantiations: 9

#### By File:
- `validator.go`: 1 instantiation
- `errors.go`: 1 instantiation
- `errors_test.go`: 3 instantiations
- `validator_test.go`: 4 instantiations (2 with missing Path field)

#### By Status:
- ✅ **Path field present**: 7 (78%)
- ❌ **Path field missing**: 2 (22%)

### Breakdown of Issues:

| File | Lines | Issue Type | Count |
|------|-------|------------|-------|
| `validator_test.go` | 872-873 | Missing Path field | 2 |

---

**Generated:** 2026-07-12
**Bead ID:** bf-30p0u
**Tool:** grep, Read, Write
