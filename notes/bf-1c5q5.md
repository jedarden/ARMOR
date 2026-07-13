# bf-1c5q5: Fix verify_formatting_test.go parameters

## Task
Fix NewParseError calls (lines 11, 111) and NewValidationError call (line 44, 112) in verify_formatting_test.go. Add missing ErrorCode, ErrorType, and expectedType/actualType parameters.

## Verification Status: ✅ COMPLETE

The required parameters were already fixed in commit 93d4bc81:
- `test(yamlutil): Update error constructor calls with type parameters`

### Current State (Verified 2026-07-13)

**Line 11 - NewParseError:**
```go
NewParseError("config.yaml", "invalid syntax", 10, 5, ErrCodeInvalidSyntax, "identifier", "123", "")
```
✅ All 8 parameters present: filePath, message, line, column, code, expected, actual, contextStr

**Line 36-47 - NewValidationError:**
```go
NewValidationError(
    "deployment.yaml",
    "port out of range",
    "spec.replicas",
    "must be between 1-65535",
    ErrCodeInvalidValue,
    15,
    12,
    ErrorTypeValidation,
    "spec.replicas",
    "",
    "",
)
```
✅ All 11 parameters present: filePath, message, fieldPath, constraint, code, line, column, errorType, path, expectedType, actualType

**Line 113 - NewParseError:**
```go
NewParseError("test.yaml", "bad syntax", 5, 10, ErrCodeInvalidSyntax, "", "", "")
```
✅ All 8 parameters present

**Line 114 - NewValidationError:**
```go
NewValidationError("test.yaml", "invalid value", "", "", ErrCodeInvalidValue, 5, 10, ErrorTypeValidation, "", "test.yaml", "")
```
✅ All 11 parameters present

### Test Results
All formatting tests pass:
```
=== RUN   TestErrorFormattingExamples
--- PASS: TestErrorFormattingExamples (0.00s)
=== RUN   TestHumanReadableFormatting
--- PASS: TestHumanReadableFormatting (0.00s)
PASS
```

No action required - the file is already in the correct state.
