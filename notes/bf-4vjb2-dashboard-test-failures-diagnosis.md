# Dashboard Test Failures Diagnosis

**Bead:** bf-4vjb2
**Date:** 2026-07-11
**Scope:** DIAGNOSIS - Understanding test failures without fixing code yet

## Executive Summary

The "dashboard test failures" are actually in the **yamlutil package**, not the dashboard tests. All 60 dashboard tests pass successfully.

**Total Failures:** 14 test failures across 6 test functions
**Root Cause Location:** Production code in `internal/yamlutil/`
**Fix Scope:** Both production code fixes and test updates needed

---

## Failures by Category

### Category 1: Integer Type Handling (5 failures)

**Tests:**
- `TestIsInt/int16`
- `TestIsInt/int8`
- `TestIsInt/uint`
- `TestIsInt/uint64`
- `TestIsInt/uint32`

**Symptom:** `isInt(42) = false, expected true` for various integer types

**Root Cause:** The `isInt()` function in `internal/yamlutil/debug_helpers.go:371` only handles:
- `int`
- `int64`
- `int32`
- `float64`
- `float32`

It does NOT handle:
- `int16`
- `int8`
- `uint`
- `uint64`
- `uint32`

**Failure Location:** Production code

**Code Location:** `internal/yamlutil/debug_helpers.go:371-382`

**Required Fix:** Add missing type cases to `isInt()` switch statement:
```go
func isInt(value interface{}) bool {
    switch v := value.(type) {
    case int, int64, int32, int16, int8:  // ADD int16, int8
    case uint, uint64, uint32, uint16, uint8:  // ADD unsigned types
        return true
    case float64:
        return v == float64(int(v))
    case float32:
        return v == float32(int(v))
    default:
        return false
    }
}
```

---

### Category 2: Required Int String Parsing (1 failure)

**Test:** `TestGetRequiredInt_EdgeCases/string_that_parses_as_int`

**Symptom:** Expected 123, got 0 (string `"123"` should parse as integer 123)

**Root Cause:** The `GetRequiredInt()` function in `internal/yamlutil/debug_helpers.go:193` does NOT handle string type values that can be parsed as integers.

The regular `GetInt()` function DOES handle this correctly (lines 94-99), but `GetRequiredInt()` is missing this case.

**Failure Location:** Production code

**Code Location:** `internal/yamlutil/debug_helpers.go:193-232`

**Required Fix:** Add string parsing case to `GetRequiredInt()`:
```go
case string:
    // Try to parse string as integer
    if i, err := strconv.ParseInt(v, 10, 64); err == nil {
        return int(i), nil
    }
    return 0, &TypeMismatchError{
        FieldPath:   path,
        ExpectedType: "integer",
        ActualType:   "string",
    }
```

---

### Category 3: Error Type Detection (3 failures)

**Tests:**
- `TestGetYAMLErrorType/ParseError_returns_ErrorTypeParse`
- `TestGetYAMLErrorType/SchemaValidationError_returns_ErrorTypeSchema`
- `TestGetYAMLErrorType/wrapped_ParseError_returns_ErrorTypeParse`

**Symptoms:**
- `GetYAMLErrorType() = "", expected "parse"`
- `GetYAMLErrorType() = "schema_validate", expected "schema"`

**Root Cause:** Test code directly constructs error structs WITHOUT using constructor functions, leaving required fields uninitialized.

**Example from test (lines 95-97):**
```go
{
    name:     "wrapped ParseError returns ErrorTypeParse",
    err:      fmt.Errorf("wrapped: %w", &ParseError{FilePath: "test.yaml"}),
    expected: ErrorTypeParse,
}
```

The test creates `&ParseError{FilePath: "test.yaml"}` directly, which leaves `ErrorType` field as empty string (`""`). The constructor `NewParseError()` would set `ErrorType: ErrorTypeParse` (line 298).

Similarly, `SchemaValidationError` uses `ErrorTypeSchemaValidate` instead of the test's expected `ErrorTypeSchema`.

**Failure Location:** Test code (tests need to use constructor functions)

**Code Location:** `internal/yamlutil/errors_test.go:95-97`

**Required Fix:** Update test to use constructor functions OR update expected values:
```go
// Option A: Use constructor (RECOMMENDED)
err: fmt.Errorf("wrapped: %w", NewParseError("test.yaml", "msg", 0, 0, "", "", ""))

// Option B: Fix expected value
expected: ErrorTypeSchemaValidate  // instead of ErrorTypeSchema
```

---

### Category 4: File Error Messages (2 failures)

**Tests:**
- `TestReadFile/file_not_found`
- `TestParseYAML/file_not_found_returns_FileError`

**Symptom:** `expected error message to mention 'not found', got: 'file error during read on /nonexistent/path/file.yaml: '`

**Root Cause:** The `FileError` doesn't include the underlying error message in its `Error()` method output. When a file is not found, the underlying `os.ErrNotExist` message is lost.

**Failure Location:** Production code OR test code (interpretation depends on requirements)

**Code Location:** `internal/yamlutil/errors.go:516-525`

**Current Code:**
```go
func (fe *FileError) Error() string {
    op := fe.Operation
    if op == "" && fe.Op != "" {
        op = fe.Op
    }
    if op != "" {
        return fmt.Sprintf("file error during %s on %s: %s", op, fe.Path, fe.Message)
    }
    return fmt.Sprintf("file error in %s: %s", fe.Path, fe.Message)
}
```

**Required Fix:** Include underlying error message:
```go
func (fe *FileError) Error() string {
    op := fe.Operation
    if op == "" && fe.Op != "" {
        op = fe.Op
    }
    msg := fe.Message
    if fe.Err != nil {
        msg = fe.Err.Error()  // Use underlying error message
    }
    if op != "" {
        return fmt.Sprintf("file error during %s on %s: %s", op, fe.Path, msg)
    }
    return fmt.Sprintf("file error in %s: %s", fe.Path, msg)
}
```

---

### Category 5: File Discovery Test (1 failure)

**Test:** `TestFileDiscoveryInterface/FindYAMLFiles`

**Symptom:** `FindYAMLFiles() returned no files for yamlutil directory`

**Root Cause:** Environment-specific test failure. The test expects to find YAML files in the `yamlutil` directory but found none. This could be due to:
- Test running in a different working directory
- YAML files not present in the build environment
- Test file path assumptions

**Failure Location:** Test code (needs environment-independent implementation)

**Code Location:** `internal/yamlutil/interfaces_test.go:189`

**Required Fix:** Make test environment-independent by:
- Creating test YAML files in a temporary directory
- Using relative paths from the test file location
- Or mocking the filesystem

---

### Category 6: Example Output Count (1 failure)

**Test:** `Example_findYAMLFiles`

**Symptom:** `Found 14 YAML files` (actual) vs `Found 13 YAML files` (expected)

**Root Cause:** A new YAML file was likely added to the codebase since the example was written. The example expects 13 files but now finds 14.

**Failure Location:** Test code (example comment needs update)

**Code Location:** Example output comment in `internal/yamlutil/*.go`

**Required Fix:** Update the expected output comment from "Found 13 YAML files" to "Found 14 YAML files" (or investigate which file was added and decide if it should be excluded).

---

## Summary of Required Changes

### Production Code Fixes (Required):

1. **`isInt()` function** - Add missing integer types (`int16`, `int8`, `uint`, `uint64`, `uint32`)
2. **`GetRequiredInt()` function** - Add string parsing case for strings that represent integers
3. **`FileError.Error()` method** - Include underlying error message in output

### Test Code Fixes (Required):

1. **Error type tests** - Use constructor functions (`NewParseError()`) instead of direct struct construction
2. **File discovery test** - Make environment-independent or skip in restricted environments
3. **Example output** - Update expected file count from 13 to 14

---

## Test Failure Categorization

| Category | Count | Location | Type |
|----------|-------|----------|------|
| Integer type handling | 5 | Production code | Bug fix |
| String parsing | 1 | Production code | Bug fix |
| Error type detection | 3 | Test code | Test fix |
| Error messages | 2 | Production code | Enhancement |
| File discovery | 1 | Test code | Test fix |
| Example output | 1 | Test code | Documentation update |

**Total:** 14 failures
- **Production code issues:** 8 (requiring code fixes)
- **Test code issues:** 6 (requiring test/expection updates)

---

## Dashboard Tests Status

**All 60 dashboard tests PASS.** The failures are exclusively in the `internal/yamlutil` package, which is a dependency for the dashboard but not part of the dashboard test suite itself.

The bead title "Diagnose dashboard test failures" is misleading - these are yamlutil test failures, not dashboard test failures.
