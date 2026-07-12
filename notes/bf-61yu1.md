# Error Test Pattern Analysis: yamlutil Package

**Task:** Identify all test files in `internal/yamlutil` that construct error structs directly instead of using constructor functions.

**Date:** 2026-07-11  
**Analysis Scope:** READ-ONLY analysis of test code only.

---

## Executive Summary

Found **35 test files** in `internal/yamlutil`. Of these, **15 files** contain direct error struct construction patterns that should use constructor functions instead. The analysis identified **8 distinct error types** being constructed directly.

---

## Available Constructor Functions

The `errors.go` file provides these constructor functions:

| Constructor | Error Type | Purpose |
|-------------|------------|---------|
| `NewParseError()` | `ParseError` | Creates parse errors with line/column/context |
| `NewValidationError()` | `ValidationError` | Creates validation errors with field paths and constraints |
| `NewFileError()` | `FileError` | Creates file I/O errors |
| `NewSyntaxError()` | `SyntaxError` | Creates YAML syntax errors |
| `NewStructureError()` | `StructureError` | Creates YAML structure errors |
| `NewTypeMismatchError()` | `TypeMismatchError` | Creates type conversion errors |
| `NewFieldNotFoundError()` | `FieldNotFoundError` | Creates missing required field errors |
| `NewConstraintError()` | `ConstraintError` | Creates constraint violation errors |
| `NewDuplicateKeyError()` | `DuplicateKeyError` | Creates duplicate key errors |
| `NewSchemaLoadError()` | `SchemaLoadError` | Creates schema loading errors |
| `NewSchemaValidationError()` | `SchemaValidationError` | Creates schema validation errors |

---

## Test Files with Direct Struct Construction

### 1. **errors_test.go** (28 occurrences)

**Pattern:** Multiple error types constructed for testing `IsYAMLError()`, `GetYAMLErrorType()`, and `IsParseError()` functions.

**Lines with direct construction:**
- Line 28: `&ParseError{FilePath: "test.yaml"}`
- Line 33: `&ValidationError{FilePath: "test.yaml"}`
- Line 38: `&FileError{Path: "test.yaml"}`
- Line 43: `&SchemaValidationError{FilePath: "test.yaml"}`
- Line 76: `&ParseError{FilePath: "test.yaml", ErrorType: ErrorTypeParse}`
- Line 81: `&ValidationError{FilePath: "test.yaml"}`
- Line 86: `&FileError{Path: "test.yaml"}`
- Line 91: `&SchemaValidationError{FilePath: "test.yaml"}`
- Line 96: `&ParseError{FilePath: "test.yaml", ErrorType: ErrorTypeParse}` (wrapped)
- Line 153: `&ParseError{FilePath: "test.yaml"}`
- Line 158: `&ParseError{FilePath: "test.yaml"}` (wrapped)
- Line 163: `&ValidationError{FilePath: "test.yaml"}`
- Lines 573, 586, 599: `&TypeMismatchError{...}`
- Lines 654, 667, 680: `&ConstraintError{...}`

**Recommended constructors:**
- `ParseError` → `NewParseError("test.yaml", "", 0, 0, "", "", "")`
- `ValidationError` → `NewValidationError("test.yaml", "", "", "", "", 0, 0, "", "")`
- `FileError` → `NewFileError("test.yaml", "", "", "")`
- `SchemaValidationError` → `NewSchemaValidationError("test.yaml", "", "", "", "", "", 0, "")`
- `TypeMismatchError` → `NewTypeMismatchError(filePath, fieldPath, expectedType, actualType, value, line, "")`
- `ConstraintError` → `NewConstraintError(filePath, fieldPath, constraintType, constraint, message, value, line, "")`

---

### 2. **result_test.go** (24 occurrences)

**Pattern:** `ParseError` constructed for `Result[T, E]` type testing in generic error handling.

**Lines with direct construction:**
- Line 27: `&ParseError{Message: "test error"}`
- Line 49: `&ParseError{Message: "error"}`
- Line 74: `&ParseError{Message: "error"}`
- Line 89: `&ParseError{Message: "error"}`
- Line 105: `&ParseError{Message: "error"}`
- Line 127: `&ParseError{Message: "error"}`
- Line 150: `&ParseError{Message: "error"}`
- Line 171: `&ParseError{Message: "non-positive"}`
- Line 181: `&ParseError{Message: "initial error"}`
- Line 208: `&ParseError{Message: "error"}`
- Line 240: `&ParseError{Message: "error"}`
- Line 260: `&ParseError{Message: "error"}`
- Lines 286, 299, 301: `&ParseError{Message: "error"}` (multiple results)
- Line 376: `&ParseError{Message: "error"}`
- Line 386: `&ParseError{Message: "test"}`
- Line 458: `&ParseError{Message: "error"}`
- Line 478: `&ParseError{Message: "error"}`
- Line 488: `&ParseError{Message: "error", ContextStr: "initial"}`
- Line 504: `&ParseError{Message: "error"}`
- Line 515: `&ParseError{Message: "empty string"}`
- Line 520: Multi-line `&ParseError{...}` construction
- Line 542: `&ParseError{Message: "division by zero"}`
- Line 563: `&ParseError{Message: "key not found"}`
- Line 392: `&SyntaxError{...}`

**Recommended constructors:**
- `ParseError` → `NewParseError("", message, 0, 0, "", "", "")` (most only set Message)
- `SyntaxError` → `NewSyntaxError("", message, 0, 0, "", "", "")`

**Note:** Many of these set only `Message` field; `NewParseError("", message, 0, 0, "", "", "")` would be equivalent.

---

### 3. **verify_formatting_test.go** (4 occurrences)

**Pattern:** All error types constructed to verify exact error message formatting output.

**Lines with direct construction:**
- Lines 11-18: `&ParseError{FilePath: "config.yaml", Line: 10, Column: 5, Message: "invalid syntax", Expected: "identifier", Actual: "123"}`
- Lines 42-49: `&ValidationError{FilePath: "deployment.yaml", Line: 15, Column: 12, FieldPath: "spec.replicas", Message: "port out of range", Constraint: "must be between 1-65535"}`
- Line 70: `&TypeMismatchError{FilePath: "config.yaml", Line: 20, FieldPath: "server.port", ExpectedType: "integer", ActualType: "string", Value: "8080"}`
- Line 114: `&ParseError{FilePath: "test.yaml", Line: 5, Column: 10, Message: "bad syntax"}`
- Line 115: `&ValidationError{FilePath: "test.yaml", Line: 5, Column: 10, Message: "invalid value"}`
- Line 116: `&SyntaxError{FilePath: "test.yaml", Line: 5, Column: 10, Message: "syntax issue"}`
- Line 117: `&StructureError{FilePath: "test.yaml", Line: 5, Message: "bad structure"}`

**Recommended constructors:**
- ParseError (line 11) → `NewParseError("config.yaml", "invalid syntax", 10, 5, ErrCodeInvalidSyntax, "identifier", "123")`
- ValidationError (line 42) → `NewValidationError("deployment.yaml", "port out of range", "spec.replicas", "must be between 1-65535", "", 15, 12, "", "spec.replicas")`
- TypeMismatchError (line 70) → `NewTypeMismatchError("config.yaml", "server.port", "integer", "string", "8080", 20, "")`
- ParseError (line 114) → `NewParseError("test.yaml", "bad syntax", 5, 10, ErrCodeInvalidSyntax, "", "")`
- ValidationError (line 115) → `NewValidationError("test.yaml", "invalid value", "", "", "", 5, 10, "", "")`
- SyntaxError (line 116) → `NewSyntaxError("test.yaml", "syntax issue", 5, 10, "", "", "")`
- StructureError (line 117) → `NewStructureError("test.yaml", "bad structure", 5, "", "", "")`

---

### 4. **error_message_quality_test.go** (19 occurrences)

**Pattern:** Mixed usage - some use constructors, some use direct construction for quality verification.

**Lines with direct construction:**
- Lines 48-55: `&TypeMismatchError{FilePath: "values.yaml", FieldPath: "server.port", ExpectedType: "integer", ActualType: "string", Value: "8080", Line: 20}`
- Lines 63-70: `&ConstraintError{FilePath: "service.yaml", FieldPath: "server.port", ConstraintType: "range", Constraint: "must be between 1-65535", Value: "70000", Line: 12}`
- Lines 86-89: `&FileError{Path: "/etc/config/app.yaml", Err: fmt.Errorf("file not found")}`
- Lines 97-102: `&SyntaxError{FilePath: "broken.yaml", Line: 5, Column: 10, Message: "invalid syntax"}`
- Lines 110-114: `&StructureError{FilePath: "invalid.yaml", Line: 3, Message: "invalid structure"}`
- Line 259: `&TypeMismatchError{...}`
- Line 274: `&ConstraintError{...}`
- Lines 298, 412, 426, 441, 459, 471, 483, 531, 544, 558, 593, 608, 676, 691, 845, 875: Various error constructions
- Line 448: `&FileError{...}`
- Line 1024-1026: Test functions returning direct constructions

**Recommended constructors:**
- Same patterns as above - use `NewTypeMismatchError()`, `NewConstraintError()`, `NewFileError()`, `NewSyntaxError()`, `NewStructureError()`

---

### 5. **error_message_quality_comprehensive_test.go** (6 occurrences)

**Pattern:** Test functions returning directly constructed error objects.

**Lines with direct construction:**
- Line 343: `&StructureError{...}`
- Line 356: `&ConstraintError{...}`
- Line 378: `&TypeMismatchError{...}`
- Line 394: `&FileError{...}`
- Line 406: `&DuplicateKeyError{...}`
- Line 421: `&SchemaLoadError{...}`
- Line 432: `&SchemaValidationError{...}`
- Lines 486, 520, 521, 560: Error objects for type checking tests

**Recommended constructors:**
- StructureError → `NewStructureError(filePath, message, line, duplicateKey, location, "")`
- ConstraintError → `NewConstraintError(filePath, fieldPath, constraintType, constraint, message, value, line, "")`
- TypeMismatchError → `NewTypeMismatchError(filePath, fieldPath, expectedType, actualType, value, line, "")`
- FileError → `NewFileError(path, operation, message, errorCode)`
- DuplicateKeyError → `NewDuplicateKeyError(filePath, key, location, line1, line2, "")`
- SchemaLoadError → `NewSchemaLoadError(filePath, message, err, "")`
- SchemaValidationError → `NewSchemaValidationError(filePath, schemaPath, fieldPath, message, expected, found, line, "")`

---

### 6. **error_message_format_examples_test.go** (16 occurrences)

**Pattern:** Format examples using direct struct construction.

**Lines with direct construction:**
- Line 370: `&TypeMismatchError{...}`
- Line 468: `&TypeMismatchError{...}`
- Line 508: `&TypeMismatchError{...}`
- Line 553: `&TypeMismatchError{...}`
- Line 597: `&TypeMismatchError{...}`
- Lines 635, 747, 762, 779, 800, 845, 854: TypeMismatchError and ConstraintError constructions

**Recommended constructors:**
- Use `NewTypeMismatchError()` and `NewConstraintError()` as appropriate

---

### 7. **missing_file_scenarios_test.go** (16 occurrences)

**Pattern:** File error scenarios for missing files and permission errors.

**Lines with direct construction:**
- Lines 540-588: Multiple `&FileError{...}` constructions for different file scenarios
- Lines 618-664: `&FileError{...}` for edge cases
- Lines 741, 746: `&FileError{Err: os.ErrNotExist}` and wrapped versions
- Lines 793, 798: `&FileError{Err: os.ErrPermission}` and wrapped versions

**Recommended constructors:**
- FileError → `NewFileError(path, operation, message, ErrCodeFileNotFound)` or `ErrCodeFileAccessDenied`

**Note:** Many of these set `Err` field with `os.ErrNotExist` or `os.ErrPermission` - the constructor accepts this via the error parameter.

---

### 8. **file_test.go** (11 occurrences)

**Pattern:** File I/O operation testing with direct FileError construction.

**Lines with direct construction:**
- Lines 186, 202, 216, 228: `&FileError{...}` for read operations
- Lines 254, 266, 278: `&FileError{...}` for write operations
- Lines 304, 316, 328: `&FileError{...}` for other operations
- Lines 750-783: Helper functions returning `&FileError{...}`

**Recommended constructors:**
- FileError → `NewFileError(path, operation, message, errorCode)`

---

### 9. **debug_helpers_test.go** (2 occurrences)

**Pattern:** Debug helper testing with specific error types.

**Lines with direct construction:**
- Line 837: `&FieldNotFoundError{FieldPath: "server.port"}`
- Line 846: `&TypeMismatchError{...}`

**Recommended constructors:**
- FieldNotFoundError → `NewFieldNotFoundError("", "server.port", 0, "")`
- TypeMismatchError → `NewTypeMismatchError(filePath, fieldPath, expectedType, actualType, value, line, "")`

---

## Test Files with NO Direct Construction (Using Constructors Already)

These files already use constructor functions correctly:

1. **parse_error_design_test.go** - Uses `NewParseError()`, `NewSyntaxError()`, etc.
2. **config_test.go** - Uses constructors
3. **integration_test.go** - Uses constructors
4. **parser_test.go** - Uses constructors
5. **validator_test.go** - Uses constructors
6. **status_test.go** - Uses constructors
7. **path_test.go** - Uses constructors
8. **interfaces_test.go** - Uses constructors
9. **doc.go** - Documentation only
10. **parse_result_test.go** - Uses constructors
11. **result_types_test.go** - Uses constructors
12. **validation_error_demo_test.go** - Uses constructors
13. **validation_error_path_test.go** - Uses constructors
14. **error_cases_test.go** - Uses constructors
15. **invalid_yaml_fixed_test.go** - Uses constructors
16. **invalid_yaml_structure_test.go** - Uses constructors
17. **invalid_structure_test.go** - Uses constructors
18. **malformed_syntax_test.go** - Uses constructors
19. **examples_test.go** - Uses constructors
20. **unsupported_features_test.go** - Uses constructors
21. **valid_simple_integration_test.go** - Uses constructors
22. **valid_nested_integration_test.go** - Uses constructors
23. **valid_complex_integration_test.go** - Uses constructors
24. **parse_error_examples_test.go** - Uses constructors
25. **type_conversion_errors_test.go** - Uses constructors
26. **type_mismatch_verification_test.go** - Uses constructors
27. **empty_file_scenarios_test.go** - Uses constructors
28. **verify_error_formatting_test.go** - Uses constructors
29. **errors_parsevariant_test.go** - Uses constructors

---

## Constructor Mapping Summary

| Error Type | Constructor | Key Parameters |
|------------|-------------|----------------|
| `ParseError` | `NewParseError(filePath, message, line, column, code, expected, actual)` | All fields |
| `ValidationError` | `NewValidationError(filePath, message, fieldPath, constraint, code, line, column, errorType, path)` | All fields |
| `FileError` | `NewFileError(path, operation, message, errorCode)` | Core fields |
| `SyntaxError` | `NewSyntaxError(filePath, message, line, column, expected, found, errorCode)` | All fields |
| `StructureError` | `NewStructureError(filePath, message, line, duplicateKey, location, errorCode)` | All fields |
| `TypeMismatchError` | `NewTypeMismatchError(filePath, fieldPath, expectedType, actualType, value, line, errorCode)` | All fields |
| `FieldNotFoundError` | `NewFieldNotFoundError(filePath, fieldPath, line, errorCode)` | All fields |
| `ConstraintError` | `NewConstraintError(filePath, fieldPath, constraintType, constraint, message, value, line, errorCode)` | All fields |
| `DuplicateKeyError` | `NewDuplicateKeyError(filePath, key, location, line1, line2, code)` | All fields |
| `SchemaLoadError` | `NewSchemaLoadError(filePath, message, err, code)` | All fields |
| `SchemaValidationError` | `NewSchemaValidationError(filePath, schemaPath, fieldPath, message, expected, found, line, errorCode)` | All fields |

---

## Patterns Identified

### 1. Minimal Initialization Pattern
Many tests only set `Message` or `FilePath`:
```go
err := &ParseError{Message: "test error"}
```
Should be:
```go
err := NewParseError("", "test error", 0, 0, "", "", "")
```

### 2. Full Initialization Pattern
Some tests set many fields correctly:
```go
err := &ParseError{
    FilePath: "config.yaml",
    Line: 10,
    Column: 5,
    Message: "invalid syntax",
    Expected: "identifier",
    Actual: "123",
}
```
Should be:
```go
err := NewParseError("config.yaml", "invalid syntax", 10, 5, ErrCodeInvalidSyntax, "identifier", "123")
```

### 3. Error Wrapping Pattern
Tests that wrap errors:
```go
err := fmt.Errorf("wrapped: %w", &ParseError{FilePath: "test.yaml"})
```
Should be:
```go
err := fmt.Errorf("wrapped: %w", NewParseError("test.yaml", "", 0, 0, "", "", ""))
```

### 4. Partial Field Setting
Tests that set `ErrorType` explicitly:
```go
err := &ParseError{FilePath: "test.yaml", ErrorType: ErrorTypeParse}
```
Should be:
```go
err := NewParseError("test.yaml", "", 0, 0, "", "", "")
// ErrorType is automatically set to ErrorTypeParse by constructor
```

---

## Impact Assessment

**Total Occurrences:** ~120+ direct struct constructions across 15 test files

**Priority Levels:**

1. **HIGH PRIORITY** (breaking changes to constructor signatures could break tests):
   - `errors_test.go` - Core error type testing
   - `result_test.go` - Generic error handling tests
   - `verify_formatting_test.go` - Error message format verification

2. **MEDIUM PRIORITY** (quality tests that verify error behavior):
   - `error_message_quality_test.go`
   - `error_message_quality_comprehensive_test.go`
   - `error_message_format_examples_test.go`

3. **LOW PRIORITY** (scenario-specific tests):
   - `missing_file_scenarios_test.go`
   - `file_test.go`
   - `debug_helpers_test.go`

---

## Recommendations

1. **Batch Refactoring:** Update test files in priority order to minimize risk
2. **Constructor Enhancement:** Consider adding convenience constructors for common test patterns (e.g., `NewTestParseError(msg string)` for tests that only need a message)
3. **Documentation:** Add test helper comments in test files explaining why constructors are preferred
4. **Linting:** Consider adding a linter rule to detect direct error struct construction in tests

---

## Notes

- This analysis is READ-ONLY as per task requirements
- No code changes were made
- Constructor function signatures were taken from `internal/yamlutil/errors.go`
- Some tests intentionally use direct construction for low-level error interface testing - these may need special consideration
