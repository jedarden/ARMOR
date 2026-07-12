# YAMLUtil Error Test Pattern Analysis

**Task ID:** bf-61yu1
**Date:** 2026-07-11
**Scope:** READ-ONLY analysis of test code

## Summary

This analysis identifies all test files in `internal/yamlutil` that construct error structs directly instead of using constructor functions.

## Available Constructor Functions

The yamlutil package provides these constructor functions:

| Constructor | Error Type | Purpose |
|------------|-----------|---------|
| `NewParseError()` | ParseError | Generic parsing errors |
| `NewValidationError()` | ValidationError | Validation failures |
| `NewFileError()` | FileError | File I/O errors |
| `NewSchemaValidationError()` | SchemaValidationError | Schema validation errors |
| `NewTypeMismatchError()` | TypeMismatchError | Type conversion errors |
| `NewFieldNotFoundError()` | FieldNotFoundError | Missing required fields |
| `NewConstraintError()` | ConstraintError | Constraint violations |
| `NewSyntaxError()` | SyntaxError | YAML syntax errors |
| `NewStructureError()` | StructureError | YAML structure errors |
| `NewDuplicateKeyError()` | DuplicateKeyError | Duplicate key errors |
| `NewSchemaLoadError()` | SchemaLoadError | Schema loading errors |

## Direct Struct Construction Patterns Summary

| Error Type | Direct Construction Count | Constructor Available |
|------------|--------------------------|----------------------|
| ParseError | 42 | ✓ `NewParseError()` |
| ValidationError | 5 | ✓ `NewValidationError()` |
| FileError | 34 | ✓ `NewFileError()` |
| TypeMismatchError | 31 | ✓ `NewTypeMismatchError()` |
| ConstraintError | 19 | ✓ `NewConstraintError()` |
| SyntaxError | 7 | ✓ `NewSyntaxError()` |
| StructureError | 5 | ✓ `NewStructureError()` |
| SchemaValidationError | 3 | ✓ `NewSchemaValidationError()` |
| FieldNotFoundError | 1 | ✓ `NewFieldNotFoundError()` |
| DuplicateKeyError | 1 | ✓ `NewDuplicateKeyError()` |
| SchemaLoadError | 1 | ✓ `NewSchemaLoadError()` |
| **Total** | **149** | **100%** |

## Files by Pattern Count

| File | Patterns Found | Primary Error Types |
|------|---------------|-------------------|
| result_test.go | 32+ | ParseError, SyntaxError |
| error_message_quality_test.go | 30+ | All error types |
| error_message_quality_comprehensive_test.go | 15+ | All error types |
| error_message_format_examples_test.go | 13+ | TypeMismatchError, ConstraintError |
| errors_test.go | 12+ | All error types |
| file_test.go | 14+ | FileError |
| missing_file_scenarios_test.go | 13+ | FileError |
| verify_formatting_test.go | 7+ | All error types |
| debug_helpers_test.go | 2+ | TypeMismatchError, FieldNotFoundError |

## Key Files with Direct Construction

### 1. errors_test.go (12+ instances)
- **Purpose:** Tests error helper functions (`IsYAMLError`, `GetYAMLErrorType`, `IsParseError`)
- **Pattern:** Minimal initialization for interface compliance testing
- **Lines:** 28, 33, 38, 43, 76, 81, 86, 91, 96, 153, 158, 163, 573, 586, 599, 654, 667, 680

### 2. result_test.go (32+ instances)
- **Purpose:** Tests Result[T, E] type
- **Pattern:** Creating errors as values in Result types
- **Lines:** 27, 49, 74, 89, 105, 127, 150, 171, 181, 208, 240, 260, 286, 299, 301, 376, 386, 392, 458, 478, 488, 504, 515, 520, 542, 563

### 3. file_test.go (14+ instances)
- **Purpose:** Tests file operations
- **Pattern:** FileError construction for file operation scenarios
- **Lines:** 186, 202, 216, 228, 254, 266, 278, 304, 316, 328, 750, 761, 772, 783

### 4. error_message_quality_test.go (30+ instances)
- **Purpose:** Tests error message quality
- **Pattern:** Fully populated errors for message formatting verification
- **Lines:** 48, 63, 86, 97, 110, 259, 274, 298, 412, 426, 448, 459, 471, 531, 544, 593, 608, 676, 691, 845, 875, 900, 973, 988, 1021, 1022, 1024, 1025, 1026

### 5. missing_file_scenarios_test.go (13+ instances)
- **Purpose:** Tests missing file scenarios
- **Pattern:** FileError construction for missing file tests
- **Lines:** 540, 552, 564, 576, 588, 618, 630, 651, 664, 741, 746, 793, 798

### 6. error_message_quality_comprehensive_test.go (15+ instances)
- **Purpose:** Comprehensive error message quality tests
- **Pattern:** All error types with full population
- **Lines:** 343, 356, 378, 394, 406, 421, 432, 486, 520, 521, 560, 561

### 7. error_message_format_examples_test.go (13+ instances)
- **Purpose:** Examples of error message formatting
- **Pattern:** TypeMismatchError and ConstraintError examples
- **Lines:** 370, 468, 508, 553, 597, 635, 747, 762, 779, 800, 845, 854, 894, 900

### 8. verify_formatting_test.go (7+ instances)
- **Purpose:** Verifies error formatting
- **Pattern:** Specific formatting attributes
- **Lines:** 11, 42, 70, 114, 115, 116, 117

### 9. debug_helpers_test.go (2+ instances)
- **Purpose:** Tests debug helper functions
- **Pattern:** TypeMismatchError and FieldNotFoundError
- **Lines:** 837, 846

## Construction Pattern Examples

### ParseError Pattern
```go
// Current (direct construction)
err := &ParseError{FilePath: "test.yaml"}
err := &ParseError{FilePath: "test.yaml", ErrorType: ErrorTypeParse}

// Should use
err := NewParseError("test.yaml", "", 0, 0, "", "", "")
```

### TypeMismatchError Pattern
```go
// Current (direct construction)
err := &TypeMismatchError{
    FilePath:     "values.yaml",
    FieldPath:    "server.port",
    ExpectedType: "integer",
    ActualType:   "string",
    Value:        "8080",
    Line:         20,
}

// Should use
err := NewTypeMismatchError("values.yaml", "server.port", "integer", "string", "8080", 20, ErrCodeTypeMismatch)
```

### FileError Pattern
```go
// Current (direct construction)
err := &FileError{
    Op:   "read",
    Path: "/test/file.yaml",
    Err:  os.ErrNotExist,
}

// Should use
err := NewFileError("/test/file.yaml", "read", os.ErrNotExist.Error(), ErrCodeFileNotFound)
```

## Notes

1. **This is a READ-ONLY analysis** - No code changes were made.
2. **All direct construction patterns have corresponding constructor functions** - No missing constructors.
3. **Some tests intentionally use partial initialization** - For testing nil/default value handling.
4. **Result type tests (result_test.go)** - May warrant keeping direct struct initialization for brevity.
5. **All constructors support optional parameters** - Empty strings can be used for optional fields.

## Conclusion

**Total direct struct constructions identified: 149 instances**

All identified patterns have corresponding constructor functions available. The patterns are concentrated in test files that verify:
- Error type interface compliance
- Result type functionality  
- Error message formatting quality
- File operation scenarios
- Debug helper functions

This inventory provides a complete map for potential refactoring work if desired.
