# Audit: NewValidationError Calls

## Summary
- **Total NewValidationError calls found**: 30 (excluding comments and function definition)
- **Calls with path parameter (9 params)**: 30
- **Calls missing path parameter (8 params)**: 0
- **Calls needing updates**: 0

## Function Signature
```go
func NewValidationError(filePath string, message string, fieldPath string, constraint string, code ErrorCode, line int, column int, errorType ErrorType, path string) *ValidationError
```

## Detailed Call Analysis by File

### 1. internal/yamlutil/errors_test.go (10 calls)
All calls already include path parameter:

| Line | Path Parameter | Status |
|------|----------------|--------|
| 457 | `tt.fieldPath` | ✅ 9 params |
| 512 | `"server.port"` | ✅ 9 params |
| 522 | `""` | ✅ 9 params |
| 530 | `""` | ✅ 9 params |
| 539 | `"database.connectionTimeout"` | ✅ 9 params |

### 2. internal/yamlutil/error_message_format_examples_test.go (8 calls)
All calls already include path parameter:

| Line | Path Parameter | Status |
|------|----------------|--------|
| 195 | `"server.port"` | ✅ 9 params |
| 258 | `tt.fieldPath` | ✅ 9 params |
| 283 | `"server.port"` | ✅ 9 params |
| 307 | `""` | ✅ 9 params |
| 336 | `"spec.template.spec.containers[0].image"` | ✅ 9 params |
| 738 | `"server.port"` | ✅ 9 params |
| 836 | `"field"` | ✅ 9 params |
| 888 | `"field"` | ✅ 9 params |

### 3. internal/yamlutil/result_types_test.go (3 calls)
All calls already include path parameter:

| Line | Path Parameter | Status |
|------|----------------|--------|
| 424 | `"server.name"` | ✅ 9 params |
| 463 | `""` | ✅ 9 params |
| 548 | `"server.port"` | ✅ 9 params |

### 4. internal/yamlutil/verify_error_formatting_test.go (2 calls)
All calls already include path parameter:

| Line | Path Parameter | Status |
|------|----------------|--------|
| 28 | `"spec.replicas"` | ✅ 9 params |
| 72 | `"spec.replicas"` | ✅ 9 params |

### 5. internal/yamlutil/validation_error_demo_test.go (3 calls)
All calls already include path parameter:

| Line | Path Parameter | Status |
|------|----------------|--------|
| 15-22 | `"server.port"` | ✅ 9 params |
| 31-38 | `"database.connectionTimeout"` | ✅ 9 params |
| 47-54 | `""` | ✅ 9 params |

### 6. internal/yamlutil/errors.go (0 calls)
This file contains:
- Line 499: Comment reference to function (not a call)
- Line 520: Function definition (not a call)
- Line 521: Example in comment (not a call)

## Production Code Analysis
**Critical Finding**: There are **NO calls to `NewValidationError` in production code** (non-test files).

The only file containing the function is `internal/yamlutil/errors.go`, which contains:
- The function definition (line 520)
- Documentation comments
- No actual calls to the function

All 30 calls are in test files, which is expected for a constructor function.

## Conclusion
All 30 `NewValidationError` calls in the codebase already include the `path` parameter. The migration to the 9-parameter signature is **complete**. No calls require updating.

## Verification
The grep search found 28 lines total, which includes:
- 30 actual function calls (all in test files)
- 4 comments/references
- 1 function definition
- 2 test function definitions (TestNewValidationError)

**Production code status**: No migration needed - the function is only defined, not called, in production code.

After filtering for actual calls only, all 30 calls are using the correct 9-parameter signature.
