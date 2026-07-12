# NewValidationError Call Sites Analysis

**Task:** Identify all `NewValidationError` callers in the ARMOR codebase and verify they pass the `path` parameter.

## Summary

- **Total call sites found:** 43
- **Test files:** 42 call sites  
- **Production code:** 0 call sites (only function definition in `internal/yamlutil/errors.go`)
- **Calls passing path parameter (9 args):** 40
- **False positives:** 3 (function definition `func TestNewValidationError`, and error messages mentioning the function name)

## Key Finding

**ALL actual `NewValidationError` calls pass the `path` parameter.** The function is **NOT used in production code** - only in tests.

## Function Signature

```go
func NewValidationError(
    filePath string,      // 1
    message string,      // 2
    fieldPath string,    // 3
    constraint string,   // 4
    code ErrorCode,      // 5
    line int,            // 6
    column int,          // 7
    errorType ErrorType, // 8
    path string          // 9 - This parameter is passed by all callers
) *ValidationError
```

## Detailed Analysis by File

### Test Files (all passing path parameter)

| File | Calls | Status |
|------|-------|--------|
| `internal/yamlutil/error_message_format_examples_test.go` | 6 | ✓ All pass |
| `internal/yamlutil/error_message_quality_test.go` | 9 | ✓ All pass |
| `internal/yamlutil/error_message_quality_comprehensive_test.go` | 2 | ✓ All pass |
| `internal/yamlutil/errors_test.go` | 6 | ✓ All pass |
| `internal/yamlutil/path_test.go` | 1 | ✓ Pass |
| `internal/yamlutil/result_types_test.go` | 3 | ✓ All pass |
| `internal/yamlutil/validation_error_path_test.go` | 10 | ✓ All pass |
| `internal/yamlutil/validation_error_demo_test.go` | 3 | ✓ All pass |
| `internal/yamlutil/verify_error_formatting_test.go` | 2 | ✓ All pass |
| `internal/yamlutil/verify_formatting_test.go` | 1 | ✓ Pass |

### Production Files

| File | Usage |
|------|-------|
| `internal/yamlutil/errors.go` | Function definition only (no callers) |

## Verification Command Used

```bash
grep -rn "NewValidationError(" /home/coding/ARMOR --include="*.go" --exclude="*_test.go" | grep -v "func NewValidationError" | grep -v "//"
```

Result: **No output** (no production callers)

## Conclusion

The `NewValidationError` function is a well-tested utility that is **only used in test code**. All test callers correctly pass the `path` parameter (the 9th parameter). There are no production code paths to update or fix.

## Context

This analysis was performed for bead `bf-wjuii` as part of ongoing validation error handling improvements in the ARMOR project.
