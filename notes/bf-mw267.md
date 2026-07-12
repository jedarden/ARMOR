# Task bf-mw267: Update FileError tests to use NewFileError

## Summary

Verified that all FileError test constructions in `internal/yamlutil` test files are **already using** the `NewFileError()` constructor. No changes were needed.

## Investigation Results

### Comprehensive Search Performed

1. **Searched all test files** in `internal/yamlutil` for direct FileError struct constructions
2. **Found 155 total references** to FileError in test files
3. **Found 33 calls to NewFileError()** - all FileError creations use the constructor
4. **Zero direct struct initializations** found (no `&FileError{...}` or `FileError{...}` patterns)

### Files Verified

All test files in `internal/yamlutil` were checked, including:
- `errors_test.go`
- `error_message_quality_test.go`
- `error_cases_test.go`
- `file_test.go`
- `integration_test.go`
- `missing_file_scenarios_test.go`
- `parser_test.go`
- And 40+ other test files

### Test Execution

All FileError-related tests pass successfully:
- `TestFileError`
- `TestFileErrorMessageContent`
- `TestFileError_ErrorMessages`
- `TestFileError_InterfaceChecks`
- `TestIsFileNotFoundError`
- `TestIsPermissionError`
- `TestReadFile_MissingFileError`
- And more

## Conclusion

The task requirements are already met:
- ✅ All FileError struct constructions use NewFileError() calls
- ✅ Test logic remains identical
- ✅ No new functionality was needed
- ✅ Tests remain readable and maintainable

This work was likely completed as part of the dependency task "Update ParseError tests to use NewParseError" (bf-1gc35), which updated error constructor patterns across the codebase.
