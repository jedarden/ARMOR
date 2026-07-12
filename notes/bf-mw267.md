# FileError Test Construction Verification (bf-mw267)

## Summary

Verified that all FileError test constructions in `internal/yamlutil` test files are already using the `NewFileError()` constructor. No direct struct initializations found.

## Verification Results

### Direct Struct Constructions
- **Count**: 0
- **Searched pattern**: `FileError{...}` and `&FileError{...}`
- **Result**: No direct struct initializations found

### Constructor Usage
- **Count**: 33
- **Pattern**: `NewFileError(path, operation, message, err)`
- **Result**: All FileError instances use the constructor

### Test Files Verified

#### Files Using NewFileError()
1. `error_message_quality_comprehensive_test.go` - 1 usage
2. `error_message_quality_test.go` - 3 usages
3. `errors_test.go` - 2 usages
4. `file_test.go` - 14 usages
5. `missing_file_scenarios_test.go` - 13 usages

#### Files with Type Checks Only
These files check error types using `IsFileError()` or type assertions, but don't construct FileError:
- `error_cases_test.go` - Uses `IsFileError()` for type checking
- `integration_test.go` - Uses type assertion `err.(*FileError)`
- `parser_test.go` - Uses type assertion for verification

## Test Execution

All FileError-related tests pass:
```
go test -v ./internal/yamlutil/... -run "FileError"
PASS
ok      github.com/jedarden/armor/internal/yamlutil    0.002s
```

## Conclusion

**Task already complete.** All FileError test constructions in `internal/yamlutil` test files already use the `NewFileError()` constructor. No changes needed to test files.

## Related Work

This verification follows the same pattern as bead bf-1gc35 (ParseError constructor verification), which confirmed that ParseError tests were also already using the `NewParseError()` constructor.
