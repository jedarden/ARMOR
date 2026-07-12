# Task bf-qoy8u: Update ParseError in Error Formatting Test Files

## Task
Update ParseError constructions in error formatting test files to use NewParseError().

## Files Analyzed
- `internal/yamlutil/error_message_format_examples_test.go`
- `internal/yamlutil/verify_error_formatting_test.go`

## Findings
All ParseError instantiations in both files are already using `NewParseError()` constructor function.

### Verification Results
- **Direct ParseError struct constructions found**: 0
- **NewParseError() usages**: 9 total

#### NewParseError() locations:
1. `verify_error_formatting_test.go:13` - AC1 test
2. `verify_error_formatting_test.go:71` - AC4 consistency test  
3. `error_message_format_examples_test.go:38` - TestParseErrorLineColumnFullFormat
4. `error_message_format_examples_test.go:99` - TestParseErrorLineColumnVariations
5. `error_message_format_examples_test.go:115` - TestParseErrorWithExpectedActual
6. `error_message_format_examples_test.go:172` - TestParseErrorAllErrorCodes
7. `error_message_format_examples_test.go:729` - TestErrorFormatConsistency
8. `error_message_format_examples_test.go:827` - TestErrorRecognition
9. `error_message_format_examples_test.go:882` - TestErrorCodes

## Conclusion
✅ Task already complete - no changes required.
