# Missing File Error Scenarios Test Coverage (bf-4fnxi)

## Summary

Created comprehensive test coverage for missing file and file access error scenarios in the yamlutil package.

## New Test File

Created `/home/coding/ARMOR/internal/yamlutil/missing_file_scenarios_test.go` with 8 main test functions covering:

### 1. TestReadFile_MissingFileScenarios
Tests for ReadFile with non-existent files:
- Non-existent file in non-existent directory
- Non-existent file in existing directory
- File path with unicode characters
- File path with spaces
- Relative path to non-existent file
- Empty string as file path
- Very long file path (250+ characters)

### 2. TestReadFile_PermissionDeniedScenarios
Tests for ReadFile with permission issues:
- File with no read permissions (0000)
- File with write-only permissions (0200)
- File in directory with no execute permission

### 3. TestReadFile_DirectoryScenarios
Tests for ReadFile when directory paths are provided:
- Directory path instead of file
- Directory path with trailing separator
- Current directory path (".")
- Parent directory path ("..")
- Root directory path ("/")

### 4. TestFileExists_MissingFileScenarios
Tests for FileExists with various scenarios:
- Non-existent file
- Directory instead of file
- File with no read permissions (platform-dependent)
- Broken symlink
- Relative path to non-existent file
- Empty string path

### 5. TestFileError_ErrorMessages
Verifies FileError produces appropriate error messages:
- Error with operation and path
- Error with message field
- Error with wrapped os.ErrNotExist
- Error with wrapped os.ErrPermission
- Error using Op field (backward compatibility)

### 6. TestFileError_InterfaceChecks
Verifies FileError implements all required interfaces:
- Implements error interface
- Implements YAMLError interface (with Code(), YAMLErrorType(), Context())
- Unwraps correctly
- Handles nil underlying error

### 7. TestWrapFileError
Tests the wrapFileError helper function:
- Wraps os.ErrNotExist with "file not found"
- Wraps os.ErrPermission with "permission denied"
- Returns other errors unchanged

### 8. TestIsFileNotFoundError_Verify and TestIsPermissionError_Verify
Comprehensive tests for error checking functions:
- Handles nil errors
- Recognizes raw OS errors
- Recognizes wrapped FileError
- Recognizes deeply wrapped errors
- Distinguishes between error types
- Handles generic errors

## Test Results

All tests pass:
```
PASS: TestReadFile_MissingFileScenarios (7 subtests)
PASS: TestReadFile_PermissionDeniedScenarios (3 subtests)
PASS: TestReadFile_DirectoryScenarios (5 subtests)
PASS: TestFileExists_MissingFileScenarios (6 subtests)
PASS: TestFileError_ErrorMessages (5 subtests)
PASS: TestFileError_InterfaceChecks (4 subtests)
PASS: TestWrapFileError (3 subtests)
PASS: TestIsFileNotFoundError_Verify (6 subtests)
PASS: TestIsPermissionError_Verify (6 subtests)
```

## Acceptance Criteria Met

✓ All missing file scenarios have corresponding tests
✓ Tests verify both error conditions and error messages
✓ Tests follow existing test patterns in yamlutil package
✓ All new tests pass

## Platform-Specific Notes

- FileExists behavior for files without read permissions varies by platform (test accommodates this)
- Permission tests may behave differently on Windows vs Unix-like systems
- Tests use t.TempDir() for proper cleanup
- Tests restore permissions after completion
