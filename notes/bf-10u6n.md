# Direct Field Access Audit: errors_test.go

## Date: 2026-07-13

## Summary

Examination of `/home/coding/ARMOR/internal/yamlutil/errors_test.go` revealed multiple instances of direct field access that violate the encapsulation pattern established in `errors.go`.

## Issues Found

### 1. TestNewParseError (Lines 290-307)

**Problem:** Direct field access to `*ParseError` internal fields

```go
if err.FilePath != tt.filePath {
    t.Errorf("FilePath = %q, want %q", err.FilePath, tt.filePath)
}
if err.Message != tt.message {
    t.Errorf("Message = %q, want %q", err.Message, tt.message)
}
if err.Line != tt.line {
    t.Errorf("Line = %d, want %d", err.Line, tt.line)
}
if err.Column != tt.column {
    t.Errorf("Column = %d, want %d", err.Column, tt.column)
}
if err.Expected != tt.expected {
    t.Errorf("Expected = %q, want %q", err.Expected, tt.expected)
}
if err.Actual != tt.actual {
    t.Errorf("Actual = %q, want %q", err.Actual, tt.actual)
}
```

All of these are direct struct field accesses to:
- `err.FilePath`
- `err.Message`
- `err.Line`
- `err.Column`
- `err.Expected`
- `err.Actual`

### 2. TestNewValidationError (Lines 460-477)

**Problem:** Direct field access to `*ValidationError` internal fields

```go
if err.FilePath != tt.filePath {
    t.Errorf("FilePath = %q, want %q", err.FilePath, tt.filePath)
}
if err.Message != tt.message {
    t.Errorf("Message = %q, want %q", err.Message, tt.message)
}
if err.FieldPath != tt.fieldPath {
    t.Errorf("FieldPath = %q, want %q", err.FieldPath, tt.fieldPath)
}
if err.Constraint != tt.constraint {
    t.Errorf("Constraint = %q, want %q", err.Constraint, tt.constraint)
}
if err.Line != tt.line {
    t.Errorf("Line = %d, want %d", err.Line, tt.line)
}
if err.Column != tt.column {
    t.Errorf("Column = %d, want %d", err.Column, tt.column)
}
```

Direct struct field accesses to:
- `err.FilePath`
- `err.Message`
- `err.FieldPath`
- `err.Constraint`
- `err.Line`
- `err.Column`

### 3. TestConstraintErrorFieldPathFormatting (Lines 633-669)

**Problem:** Direct struct initialization bypassing constructor

```go
err: &ConstraintError{
    FilePath:       "config.yaml",
    FieldPath:      "server.port",
    ConstraintType: "range",
    Constraint:     "must be between 1-65535",
    Value:          "70000",
    Line:           12,
},
```

This occurs in three test cases (lines 633-640, 646-652, 659-665).

**Why this is a problem:**
- The `errors.go` file explicitly states (lines 19-23):
  ```
  // CONSTRUCTOR USAGE PATTERNS:
  //
  // IMPORTANT: Always use the provided constructor functions to create error instances.
  // Direct struct initialization (e.g., ValidationError{...}) is strongly discouraged
  // as it may result in improperly initialized errors.
  ```
- The constructor `NewConstraintError` exists (line 651 in errors.go) and should be used

## Fix Plan

### Approach 1: Test Through Public Interface (Recommended)

**For ParseError tests (lines 290-307):**
- Remove direct field access assertions
- Test behavior through public interface methods:
  - `err.Error()` - Verify formatted message contains expected values
  - `err.Code()` - Verify error code
  - `err.YAMLErrorType()` - Verify error type
  - `err.Context()` - Verify context string
- These methods already correctly test the error behavior (lines 308-313 are correct)

**For ValidationError tests (lines 460-477):**
- Remove direct field access assertions
- Test behavior through public interface methods:
  - `err.Error()` - Verify formatted message contains expected values
  - `err.Code()` - Verify error code (line 478 already does this correctly)
  - `err.YAMLErrorType()` - Verify error type (line 481 already does this correctly)
- The String() method (line 486) also provides formatted output that can be tested

### Approach 2: Use Constructor Instead of Direct Struct Initialization

**For ConstraintError tests (lines 633-669):**

Replace:
```go
err: &ConstraintError{
    FilePath:       "config.yaml",
    FieldPath:      "server.port",
    ConstraintType: "range",
    Constraint:     "must be between 1-65535",
    Value:          "70000",
    Line:           12,
},
```

With:
```go
err: NewConstraintError(
    "config.yaml",
    "server.port",
    "range",
    "must be between 1-65535",
    "",  // message parameter (optional)
    "70000",
    12,
    ErrCodeConstraintViolation,
),
```

## Implementation Steps

1. **Remove direct field access from TestNewParseError:**
   - Delete lines 290-307
   - The remaining tests (lines 308-329) already correctly test through the public interface

2. **Remove direct field access from TestNewValidationError:**
   - Delete lines 460-477
   - The remaining tests (lines 478-500) already correctly test through the public interface

3. **Replace direct ConstraintError initialization with constructor calls:**
   - Replace struct literals with `NewConstraintError()` calls at lines 633-640, 646-652, 659-665

4. **Verify tests still pass:**
   - Run `go test ./internal/yamlutil/...`
   - Ensure all tests still verify the intended behavior

## Note on Test Philosophy

The current pattern of directly asserting field values in tests is actually an anti-pattern:
- It couples tests to internal implementation details
- It makes refactoring harder (any field change breaks tests)
- It tests "how" rather than "what"

The better approach is to test through the public interface (Error(), Code(), YAMLErrorType(), Context(), String()), which:
- Tests behavior rather than implementation
- Allows internal refactoring without breaking tests
- Follows Go idioms for testing exported types

## Files to Modify

- `/home/coding/ARMOR/internal/yamlutil/errors_test.go`

## Estimated Lines Changed

- Remove: ~50 lines (direct field assertions)
- Modify: ~30 lines (replace struct literals with constructor calls)
- Net change: -20 lines
