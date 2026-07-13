# Bead bf-1c5q5: Fix verify_formatting_test.go parameters

## Status: Already Fixed

## Issue
The bead requested fixing `NewParseError` and `NewValidationError` constructor calls in `internal/yamlutil/verify_formatting_test.go` by adding missing `ErrorCode`, `ErrorType`, and `expectedType`/`actualType` parameters.

## Resolution
This issue was already fixed in commit `93d4bc81` on 2026-07-13 12:01:17.

### Changes Applied
The commit updated the following calls:

1. **Line 11** - `NewParseError`:
   - Added missing `contextStr` parameter (empty string)
   
2. **Lines 35-47** - `NewValidationError`:
   - Changed empty string code to `ErrCodeInvalidValue`
   - Added `ErrorTypeValidation`
   - Added `expectedType` and `actualType` parameters (empty strings)

3. **Line 113** - `NewParseError` (in test array):
   - Changed empty string to `ErrCodeInvalidSyntax`

4. **Line 114** - `NewValidationError` (in test array):
   - Changed empty string code to `ErrCodeInvalidValue`
   - Added `ErrorTypeValidation`
   - Added `expectedType` and `actualType` parameters (empty strings)

## Verification
All tests in `verify_formatting_test.go` now pass:

```bash
go test ./internal/yamlutil -run TestErrorFormattingExamples -v
go test ./internal/yamlutil -run TestHumanReadableFormatting -v
```

Both test functions pass successfully, confirming the error constructors have correct parameter signatures.
