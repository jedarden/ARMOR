# Fix Function Call Errors in verify_formatting_test.go

## Task
Fix NewValidationError and NewParseError function calls in verify_formatting_test.go with incorrect argument counts.

## Work Completed
The function call errors were already fixed in a previous commit (1422a62d). The current state:

1. **Line 11 - NewParseError**: Fixed to use proper ErrorCode parameter
   - Before: `NewParseError("config.yaml", "invalid syntax", 10, 5, "", "identifier", "123", "")`
   - After: `NewParseError("config.yaml", "invalid syntax", 10, 5, ErrCodeInvalidSyntax, "identifier", "123", "")`

2. **Line 113 - NewParseError**: Fixed to use proper ErrorCode parameter
   - Already correctly using `ErrCodeInvalidSyntax`

3. **Line 44 & 114 - NewValidationError**: Using correct parameter order
   - All 11 parameters in correct order
   - Using proper types (ErrorCode, ErrorType)

## Verification
- File compiles without errors ✓
- All tests in verify_formatting_test.go pass ✓
- Test output confirms proper error formatting ✓

## Status
The task has been completed successfully. All function call signatures match their definitions in errors.go.
