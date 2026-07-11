# YAMLUtil Test Suite Results - bf-3jyba

## Execution Summary

**Date**: 2026-07-11  
**Command**: `go test ./internal/yamlutil/... -v`  
**Exit Code**: 0 (success but with test failures)  
**Total Test Runtime**: 0.014s

## Test Results Overview

### Status: FAILED (with partial failures)

The test suite ran with **multiple test failures**. While the overall exit code is 0, several individual tests and examples failed.

## Detailed Failures

### 1. TestGetYAMLErrorType (Line 531)
**Failed Subtests**:
- `ParseError_returns_ErrorTypeParse` (Line 534)
  - Expected: `parse`
  - Got: empty string
- `SchemaValidationError_returns_ErrorTypeSchema` (Line 537)
  - Expected: `schema`
  - Got: `schema_validate`
- `wrapped_ParseError_returns_ErrorTypeParse` (Line 538)
  - Expected: `parse`
  - Got: empty string

**Issue**: The `GetYAMLErrorType()` function is returning incorrect/empty values for ParseError and SchemaValidationError types.

### 2. TestFileDiscoveryInterface/FindYAMLFiles (Line 860)
**Issue**: `FindYAMLFiles()` returned no files for the yamlutil directory when it should have found YAML files.

### 3. TestValidator_ErrorFormatting (Line 1247)
**Issue**: 
- Expected Error() to contain type - Got: "validation error in <string> at line 3: yaml: line 3: mapping values are not allowed in this context"
- Expected Error() to contain line number - Got: same message without clear line number formatting

### 4. Example Test Failures

#### Example_findYAMLFiles (Line 1341)
- **Expected**: Found 13 YAML files
- **Got**: Found 14 YAML files

#### Example_fileDiscoveryPatterns (Line 1381)
- **Expected**: Found 13 YAML files in testdata
- **Got**: Found 14 YAML files in testdata

#### ExampleEnhancedParseError_syntax (Line 1407)
- **Issue**: Extra newline character in output
- **Expected**: Single line between error marker and "This is a syntax error"
- **Got**: Double newline (blank line included)

#### ExampleEnhancedParseError_typeMismatch (Line 1426)
- **Issue**: Extra newline in output
- **Expected**: No newline after error message
- **Got**: Newline before "Expected: integer"

#### ExampleEnhancedParseError_transformFromYAML (Line 1450)
- **Issue**: Context display shows extra surrounding lines
- **Expected**: Only showing 3 lines of context
- **Got**: Showing 4 lines of context

#### ExampleEnhancedParseError_richContext (Line 1469)
- **Issue**: Arrow marker position and context display differ from expected
- **Expected**: Arrow at column 5, showing 3 lines
- **Got**: Arrow at column 5, showing 4 lines with extra spacing

#### ExampleResult_comprehensiveErrorHandling (Line 1499)
- **Issue**: Missing closing parenthesis in error message
- **Expected**: `(field: server.port, expected: integer, got: string)`
- **Got**: `(field: server.port, expected: integer, got: string`

## Passing Tests

The majority of tests passed successfully, including:
- Configuration tests (Performance, Default, Strict, Lenient validator configs)
- Field accessor tests (GetField, GetString, GetInt, GetBool, HasField)
- Required field tests (GetRequiredField, GetRequiredString, GetRequiredInt, GetRequiredBool)
- Validation tests (ValidateRequiredFields, ValidateFieldRequirements)
- Error handling tests (NewParseError, NewValidationError, WrapError)
- File operation tests (ReadFile, FileExists, FileError)
- Parser tests (ParseFile, ParseFileToMap, ParseString)
- Integration tests
- Result type tests (Ok, Err, Unwrap, Map, AndThen, OrElse)

## Acceptance Criteria Status

- [x] Test suite executed successfully
- [x] Full test output captured and saved to `/tmp/yamlutil-test-output.txt` and `notes/bf-3jyba-test-output.txt`
- [x] Exit code recorded (0)
- [x] Failures clearly identified in output

## Recommendations

1. **Fix GetYAMLErrorType()**: This function needs to properly extract error types from ParseError and SchemaValidationError
2. **Fix FindYAMLFiles()**: The function should be discovering YAML files in the test directory
3. **Fix error formatting**: Ensure error messages include proper type and line number information
4. **Fix example output expectations**: Update example tests to match actual output, or fix output formatting to match expectations
5. **Context display**: Ensure consistent context line display in error messages

## Files Generated

- `/tmp/yamlutil-test-output.txt` - Raw test output
- `notes/bf-3jyba-test-output.txt` - Archived test output
- `notes/bf-3jyba.md` - This summary document
