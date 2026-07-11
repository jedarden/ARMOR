# Audit: NewValidationError Calls

## Summary
- **Total NewValidationError calls found**: 21 (excluding comments, function definitions, and string literals)
- **Calls with path parameter (9 params)**: 21
- **Calls missing path parameter (8 params)**: 0
- **Calls needing updates**: 0

## Function Signature
```go
func NewValidationError(filePath string, message string, fieldPath string, constraint string, code ErrorCode, line int, column int, errorType ErrorType, path string) *ValidationError
```

## Detailed Call Analysis by File

### 1. internal/yamlutil/errors_test.go (5 calls)
All calls already include path parameter (9 params):

| Line | Path Parameter | Status |
|------|----------------|--------|
| 457 | `tt.fieldPath` | ✅ Complete |
| 512 | `"server.port"` | ✅ Complete |
| 522 | `""` | ✅ Complete |
| 530 | `""` | ✅ Complete |
| 539 | `"database.connectionTimeout"` | ✅ Complete |

**Note**: Lines 493 and 498 contain `NewValidationError()` in string literals within error messages, not actual function calls.

### 2. internal/yamlutil/error_message_format_examples_test.go (8 calls)
All calls already include path parameter (9 params):

| Line | Path Parameter | Status |
|------|----------------|--------|
| 195 | `"server.port"` | ✅ Complete |
| 258 | `tt.fieldPath` | ✅ Complete |
| 283 | `"server.port"` | ✅ Complete |
| 307 | `""` | ✅ Complete |
| 336 | `"spec.template.spec.containers[0].image"` | ✅ Complete |
| 738 | `"server.port"` | ✅ Complete |
| 836 | `"field"` | ✅ Complete |
| 888 | `"field"` | ✅ Complete |

### 3. internal/yamlutil/result_types_test.go (3 calls)
All calls already include path parameter (9 params):

| Line | Path Parameter | Status |
|------|----------------|--------|
| 424 | `"server.name"` | ✅ Complete |
| 463 | `""` | ✅ Complete |
| 548 | `"server.port"` | ✅ Complete |

### 4. internal/yamlutil/verify_error_formatting_test.go (2 calls)
All calls already include path parameter (9 params):

| Line | Path Parameter | Status |
|------|----------------|--------|
| 28 | `"spec.replicas"` | ✅ Complete |
| 72 | `"spec.replicas"` | ✅ Complete |

### 5. internal/yamlutil/validation_error_demo_test.go (3 calls)
All calls already include path parameter (9 params):

| Line | Path Parameter | Status |
|------|----------------|--------|
| 15-25 | `"server.port"` | ✅ Complete |
| 31-41 | `"spec.template.spec.containers[0].image"` | ✅ Complete |
| 47-57 | `"spec.replicas"` | ✅ Complete |

## Production Code Analysis
**Critical Finding**: There are **NO calls to `NewValidationError` in production code** (non-test files).

The only file containing the function definition is `internal/yamlutil/errors.go`, which contains:
- The function definition (line 520)
- Documentation comments (lines 499, 519-521)
- No actual calls to the function

All 21 calls are in test files, which is expected for a constructor function that's typically called by other validation code.

## Conclusion
✅ **All 21 `NewValidationError` calls in the codebase already include the `path` parameter.**

The migration to the 9-parameter signature is **complete**. No calls require updating.

## Verification Method
Used grep to find all occurrences of `NewValidationError(` in .go files, then manually excluded:
- Function definitions (`func NewValidationError`)
- Comments containing the function name
- Test function definitions (`func TestNewValidationError`)
- String literals containing the function name (e.g., in `t.Error()` messages)

**Final verified count**: 21 actual function calls, all using the correct 9-parameter signature.
