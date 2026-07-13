# yaml.TypeError Type Assertions Audit

**Bead ID:** bf-693nd
**Date:** 2026-07-12
**Scope:** All `*yaml.TypeError` type assertions in `internal/yamlutil/`

## Executive Summary

This audit identifies **7 locations** where `*yaml.TypeError` type assertions are performed across the yamlutil package. All locations follow a consistent error handling pattern using the centralized `FormatYAMLErrorMessage()` function.

---

## Complete List of Type Assertion Locations

### 1. internal/yamlutil/parser.go

**Location:** Line 110-112 (ParseFile method)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    result.Error = &YAMLParseError{
        FilePath: filePath,
        Message:  FormatYAMLErrorMessage(filePath, typeErr.Errors),
        RawError: err,
    }
    return result
}
```

**Context:** Parsing YAML file into a specific data structure.
**Error Handling:** ✅ Enhanced - Uses `FormatYAMLErrorMessage()` for detailed formatting.
**Returns:** `ParseResult` with structured `YAMLParseError`.

---

**Location:** Line 167-170 (ParseFileToMap method)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    result.Error = &YAMLParseError{
        FilePath: filePath,
        Message:  FormatYAMLErrorMessage(filePath, typeErr.Errors),
        RawError: err,
    }
}
```

**Context:** Parsing YAML file into generic `map[string]interface{}`.
**Error Handling:** ✅ Enhanced - Uses `FormatYAMLErrorMessage()` for detailed formatting.
**Returns:** `ParseResult` with structured `YAMLParseError`.

---

**Location:** Line 397-400 (ParseYAML function)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    return nil, &YAMLParseError{
        FilePath: filePath,
        Message:  FormatYAMLErrorMessage(filePath, typeErr.Errors),
        RawError: err,
        Line:     extractErrorLine(err),
    }
}
```

**Context:** Standalone function to parse YAML into map.
**Error Handling:** ✅ Enhanced - Uses `FormatYAMLErrorMessage()` + extracts line number.
**Returns:** `nil` and `YAMLParseError` with line information.

**Note:** `parser.go` also contains a helper function `formatTypeErrorDetails` (line 442) that provides similar functionality but uses a different format. This could be consolidated.

---

### 2. internal/yamlutil/schema.go

**Location:** Line 292-294 (ValidateFile method)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   FormatYAMLErrorMessage(filePath, typeErr.Errors),
        ErrorCode: ErrCodeTypeMismatch,
    })
}
```

**Context:** Validating YAML file against a schema.
**Error Handling:** ✅ Enhanced - Uses `FormatYAMLErrorMessage()` with error code.
**Returns:** `SchemaValidationResult` with error added to `Errors` slice.

---

**Location:** Line 713-716 (LoadSchema function)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    return nil, &SchemaError{
        Message:  fmt.Sprintf("Failed to parse YAML schema: %v", typeErr.Errors),
        FilePath: schemaPath,
    }
}
```

**Context:** Loading a YAML schema definition file.
**Error Handling:** ⚠️ Basic - Does NOT use `FormatYAMLErrorMessage()`.
**Returns:** `nil` and `SchemaError`.

**ISSUE IDENTIFIED:** This location formats the error manually with `fmt.Sprintf` instead of using the centralized `FormatYAMLErrorMessage()` function. This should be updated for consistency.

---

### 3. internal/yamlutil/validator.go

**Location:** Line 269 (parseYAMLError method)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    ve.Type = ErrorTypeStructure
    ve.Message = FormatYAMLErrorMessage(ve.FilePath, typeErr.Errors)
    if len(typeErr.Errors) > 0 {
        ve.Context = fmt.Sprintf("Type errors: %s", strings.Join(typeErr.Errors, "; "))
    }
    return ve
}
```

**Context:** Converting yaml.v3 errors to `LocalValidationError`.
**Error Handling:** ✅ Enhanced - Uses `FormatYAMLErrorMessage()` and adds context.
**Returns:** `LocalValidationError` with type set to `ErrorTypeStructure`.

---

### 4. internal/yamlutil/syntax_validator.go

**Location:** Line 1041 (convertParseError method)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    se.Message = FormatYAMLErrorMessage(filePath, typeErr.Errors)
    se.ErrorCode = ErrCodeTypeMismatch
    return se
}
```

**Context:** Converting parse errors to `SyntaxError` for syntax validation.
**Error Handling:** ✅ Enhanced - Uses `FormatYAMLErrorMessage()` with error code.
**Returns:** `SyntaxError` with `ErrorCode` set.

---

### 5. internal/yamlutil/future.go

**Location:** Line 103 (ParseStreamToMap method)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    return nil, fmt.Errorf("%s", FormatYAMLErrorMessage("", typeErr.Errors))
}
```

**Context:** Streaming YAML parser stub (future enhancement).
**Error Handling:** ✅ Enhanced - Uses `FormatYAMLErrorMessage()` with empty filePath.
**Returns:** `nil` and formatted error.

**Note:** This is in a stub/future implementation file but still follows the pattern correctly.

---

### 6. internal/yamlutil/errors.go

**Location:** Line 1293 (FormatYAMLErrorMessage function definition)

This is the **centralized error formatting function** used by most locations:

```go
func FormatYAMLErrorMessage(filePath string, typeErrors []string) string
```

**Purpose:** Parse yaml.TypeError errors and extract structured information:
- Line numbers
- Field paths
- Expected vs actual types
- Context and values

**Output:** Multi-line formatted error message with sections for:
- Error summary (count)
- Individual error details:
  - Line number
  - Field path
  - Type mismatch
  - Value (if available)
  - Context (if available)

---

## Current Error Handling Approaches

### Approach 1: Enhanced Error Formatting (6 of 7 locations)

**Pattern:**
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    return StructuredError{
        Message: FormatYAMLErrorMessage(filePath, typeErr.Errors),
        ErrorCode: ErrCodeTypeMismatch,
        // ... other fields
    }
}
```

**Locations using this approach:**
- parser.go: Lines 110-112, 167-170, 397-400
- schema.go: Line 292-294
- validator.go: Line 269
- syntax_validator.go: Line 1041
- future.go: Line 103

**Advantages:**
- Consistent error messages across the codebase
- Structured information (line numbers, field paths, types)
- Easy to parse for tooling
- Developer-friendly output

### Approach 2: Basic Error Formatting (1 of 7 locations)

**Pattern:**
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    return Error{
        Message: fmt.Sprintf("Failed to parse YAML schema: %v", typeErr.Errors),
    }
}
```

**Location using this approach:**
- schema.go: Line 713-716 (LoadSchema function)

**Issues:**
- Inconsistent with other locations
- Loses structured error information
- Less developer-friendly
- Does not use centralized formatting function

---

## Enhancement Checklist

### High Priority

- [ ] **schema.go:713-716** - Update LoadSchema to use `FormatYAMLErrorMessage()` instead of `fmt.Sprintf`
  - Current: `fmt.Sprintf("Failed to parse YAML schema: %v", typeErr.Errors)`
  - Should be: `FormatYAMLErrorMessage(schemaPath, typeErr.Errors)`

### Medium Priority

- [ ] **parser.go** - Consolidate `formatTypeErrorDetails()` (line 442) with `FormatYAMLErrorMessage()` usage
  - Currently has two similar functions doing the same thing
  - Could unify on `FormatYAMLErrorMessage()` only

### Low Priority

- [ ] Add unit tests for all type assertion code paths
- [ ] Consider adding error codes to all return types (not just SchemaValidationResult)
- [ ] Document the pattern in a package-level comment

---

## Statistics

| File | Type Assertions | Enhanced Formatting | Basic Formatting |
|------|----------------|---------------------|------------------|
| parser.go | 3 | 3 | 0 |
| schema.go | 2 | 1 | 1 |
| validator.go | 1 | 1 | 0 |
| syntax_validator.go | 1 | 1 | 0 |
| future.go | 1 | 1 | 0 |
| errors.go | 1 (helper) | N/A | N/A |
| **Total** | **7 active + 1 helper** | **7** | **1** |

---

## Recommendations

1. **Immediate:** Fix schema.go:713-716 to use `FormatYAMLErrorMessage()`
2. **Short-term:** Consolidate duplicate formatting functions in parser.go
3. **Long-term:** Consider adding integration tests for error message formatting
4. **Documentation:** Add package-level documentation explaining the type assertion pattern

---

## Appendix: Standard Pattern Template

For future additions to the codebase, use this pattern:

```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    return &YourErrorType{
        FilePath: filePath,
        Message:  FormatYAMLErrorMessage(filePath, typeErr.Errors),
        ErrorCode: ErrCodeTypeMismatch,  // If applicable
        RawError: err,                    // If applicable
        Line:     extractErrorLine(err),  // If applicable
    }
}
```

This ensures consistency across all yaml.TypeError handling in the codebase.
