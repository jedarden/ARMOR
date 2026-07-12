# Task bf-2bh1q: Update ParseError in formatting test files

## Task Summary
Update ParseError constructions in error_message_format_examples_test.go, verify_error_formatting_test.go, and verify_formatting_test.go to use NewParseError().

## Findings
All three test files are **already using `NewParseError()`** instead of direct struct construction.

### Files Verified:

1. **error_message_format_examples_test.go**
   - Uses `NewParseError()` on lines: 13, 38, 71, 99, 115, 172
   - No direct `&ParseError{}` or `ParseError{}` constructions found

2. **verify_error_formatting_test.go**
   - Uses `NewParseError()` on line: 11
   - No direct `&ParseError{}` or `ParseError{}` constructions found

3. **verify_formatting_test.go**
   - Uses `NewParseError()` on lines: 38, 99, 115, 172, 195, 258, 283, 307, 336, 686, 729, 827
   - No direct `&ParseError{}` or `ParseError{}` constructions found

## Acceptance Status
✅ All ParseError struct constructions already use NewParseError()
✅ Test logic remains identical (no changes needed)
✅ Tests remain readable (already readable)

## Conclusion
This task was already completed in a previous session. No code changes were required.
