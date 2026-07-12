# Task bf-2oo3p: Update test file callers with path parameter

## Task Description
Update all NewValidationError calls in test files to pass appropriate path values (typically use fieldPath).

## Investigation Results

### Findings
After thorough investigation, **all test file NewValidationError calls already pass appropriate path values**:

1. **Non-empty fieldPath cases** - path parameter is set to fieldPath:
   - `NewValidationError(..., "server.port", ..., "server.port")`
   - `NewValidationError(..., tt.fieldPath, ..., tt.fieldPath)`
   - `NewValidationError(..., "database.connectionTimeout", ..., "database.connectionTimeout")`

2. **Empty fieldPath cases** - path parameter is either empty or filename:
   - `NewValidationError("test.yaml", "validation failed", "", ..., "")`
   - `NewValidationError("test.yaml", "invalid value", "", ..., "test.yaml")`

### Verification
- All 54 NewValidationError calls in test files were reviewed
- All yamlutil tests pass: `go test ./internal/yamlutil/... -v`
- Path parameter values correctly reflect validation error location
- When fieldPath is provided, it's used as the path parameter
- When fieldPath is empty, path is either empty or contains the filename

### Files Reviewed
- `internal/yamlutil/verify_formatting_test.go` ✓
- `internal/yamlutil/errors_test.go` ✓
- `internal/yamlutil/validation_error_demo_test.go` ✓
- `internal/yamlutil/validation_error_path_test.go` ✓
- `internal/yamlutil/error_message_format_examples_test.go` ✓
- `internal/yamlutil/error_message_quality_test.go` ✓
- `internal/yamlutil/result_types_test.go` ✓
- `internal/yamlutil/path_test.go` ✓

### Conclusion
**Task already completed** - No changes needed. All test file NewValidationError calls pass appropriate path values, and all tests pass successfully.

## Test Results
```
=== RUN   TestNewValidationError
--- PASS: TestNewValidationError (0.00s)
=== RUN   TestNewValidationErrorPathHandling
--- PASS: TestNewValidationErrorPathHandling (0.00s)
PASS
ok  	github.com/jedarden/armor/internal/yamlutil	(cached)
```
