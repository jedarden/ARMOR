# Bead bf-qoy8u: Update ParseError in Error Formatting Test Files

## Task
Update ParseError constructions in error formatting test files to use NewParseError().

## Files Checked
- `internal/yamlutil/error_message_format_examples_test.go`
- `internal/yamlutil/verify_error_formatting_test.go`

## Findings
Both files already use `NewParseError()` throughout. No direct `ParseError{}` struct constructions were found.

### Verification
```bash
grep -n "ParseError{" internal/yamlutil/error_message_format_examples_test.go internal/yamlutil/verify_error_formatting_test.go
# No matches found
```

### Current State
- `error_message_format_examples_test.go`: All ParseError instances use `NewParseError()`
- `verify_error_formatting_test.go`: All ParseError instances use `NewParseError()`

## Conclusion
The task requirements have already been met. Both test files properly use the `NewParseError()` constructor function instead of direct struct construction.

## Files
- error_message_format_examples_test.go (verified compliant)
- verify_error_formatting_test.go (verified compliant)
