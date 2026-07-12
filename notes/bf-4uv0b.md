# Bead bf-4uv0b: Replace FileError constructions in permission tests

## Task
Replace FileError struct initializations with NewFileError() in permission-related test functions.

## Target
File: internal/yamlutil/file_test.go
Function: TestReadFilePermissionDenied (lines 300-410)

## Analysis
After examining the target function `TestReadFilePermissionDenied` (lines 300-410), I confirmed that:

1. **No direct FileError constructions exist**: The function does not contain any `&FileError{Path: "...", Message: "...", Err: ...}` struct initializations that need to be replaced.

2. **Function only tests scenarios**: The test function creates files with various permission settings and verifies that the `ReadFile()` function returns appropriate errors. It does not construct FileError structs directly.

3. **All tests pass**: The permission tests compile and run successfully:
   - `TestReadFilePermissionDenied/file_without_read_permissions` - PASS
   - `TestReadFilePermissionDenied/file_with_read-only_permissions_(successful_read)` - PASS
   - `TestReadFilePermissionDenied/directory_without_execute_permissions` - PASS

4. **Already using correct patterns**: The test code properly uses error checking via type assertions (`fileErr, ok := err.(*FileError)`) and helper functions like `IsPermissionError()`, which is the correct approach for testing error handling.

## Conclusion
The acceptance criteria is already met:
- All FileError constructions in target function replaced with NewFileError(): ✓ (0 out of 0, 100% complete)
- Tests compile and pass: ✓
- No test logic changed: ✓

The task appears to have been based on an older version of the code or the file has already been refactored to use the correct error construction patterns.

## Test Results
```
=== RUN   TestReadFilePermissionDenied
=== RUN   TestReadFilePermissionDenied/file_without_read_permissions
=== RUN   TestReadFilePermissionDenied/file_with_read-only_permissions_(successful_read)
=== RUN   TestReadFilePermissionDenied/directory_without_execute_permissions
--- PASS: TestReadFilePermissionDenied (0.00s)
```

All tests pass successfully.
