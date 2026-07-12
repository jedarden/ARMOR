# Bead bf-1tsan: Update ParseError in Error Message Quality Test Files

## Task
Update ParseError constructions in error message quality test files to use NewParseError().

## Files Checked
- `internal/yamlutil/error_message_quality_test.go`
- `internal/yamlutil/error_message_quality_comprehensive_test.go`
- `internal/yamlutil/verify_formatting_test.go`

## Findings
**No changes required.** All three test files already use `NewParseError()` calls exclusively.

### Verification
Used grep to search for direct ParseError struct constructions:
```bash
grep -n "ParseError{" internal/yamlutil/error_message_quality_test.go
grep -n "ParseError{" internal/yamlutil/error_message_quality_comprehensive_test.go
grep -n "ParseError{" internal/yamlutil/verify_formatting_test.go
```

All three searches returned **no results**, confirming there are no direct `ParseError{}` or `&ParseError{}` struct constructions in these files.

### Current State
All ParseError instances in these files use the proper constructor function:
- `error_message_quality_test.go`: Multiple calls to `NewParseError()`
- `error_message_quality_comprehensive_test.go`: Multiple calls to `NewParseError()`
- `verify_formatting_test.go`: Multiple calls to `NewParseError()`

### Test Results
Tests run successfully with current implementation. The test logic remains intact and all error messages are properly formatted using the NewParseError() constructor.

## Conclusion
The task acceptance criteria are already met:
- ✓ All ParseError struct constructions use NewParseError()
- ✓ Test logic remains identical
- ✓ Tests remain readable

No code changes were necessary for this bead.
