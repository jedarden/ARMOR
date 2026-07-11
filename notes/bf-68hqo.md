# Bead bf-68hqo: Define Error Types for YAML Parsing Operations

## Task Summary
Define core error types for the YAML parser utility module.

## Verification
All acceptance criteria have been verified and implemented:

### ✅ AC1: YAMLError Interface
- **Location:** `/home/coding/ARMOR/internal/yamlutil/errors.go` lines 27-42
- **Implementation:** YAMLError interface defined with:
  - `Code() ErrorCode` - Returns error code for programmatic handling
  - `YAMLErrorType() ErrorType` - Returns error category for type switching
  - `Context() string` - Returns additional context about the error
  - Standard `error` interface via `Error() string`

### ✅ AC2: ParseError with Position Info
- **Location:** `/home/coding/ARMOR/internal/yamlutil/errors.go` lines 271-388
- **Implementation:** ParseError struct implements YAMLError with:
  - `Line int` - Line number where error occurred (1-indexed)
  - `Column int` - Column number where error occurred (1-indexed)
  - `FilePath string` - Path to the file being parsed
  - `Expected string` - What was expected (for syntax/type errors)
  - `Actual string` - What was actually found (for syntax/type errors)
  - `ErrorCode ErrorCode` - Error code for programmatic handling
  - Constructor function: `NewParseError()`

### ✅ AC3: ValidationError with Path and Constraint Info
- **Location:** `/home/coding/ARMOR/internal/yamlutil/errors.go` lines 390-545
- **Implementation:** ValidationError struct implements YAMLError with:
  - `FieldPath string` - Dot-notation path to the invalid field
  - `Constraint string` - Constraint that was violated
  - `Line int` - Line number where error occurred (1-indexed)
  - `Column int` - Column number where error occurred (1-indexed)
  - `FilePath string` - Path to the file being validated
  - `ErrorCode ErrorCode` - Error code for programmatic handling
  - Constructor function: `NewValidationError()`

### ✅ AC4: Error Codes Defined as Constants
- **Location:** `/home/coding/ARMOR/internal/yamlutil/errors.go` lines 158-268
- **Implementation:** ErrorCode constants defined for:
  - File errors: `ErrCodeFileNotFound`, `ErrCodeFileAccessDenied`, `ErrCodeFileIOError`, `ErrCodeFileEmpty`
  - Parse errors: `ErrCodeInvalidSyntax`, `ErrCodeTypeMismatch`, `ErrCodeInvalidStructure`, `ErrCodeDuplicateKey`, `ErrCodeParseError`
  - Validation errors: `ErrCodeValidationFailed`, `ErrCodeRequiredField`, `ErrCodeConstraintViolation`, `ErrCodeInvalidValue`
  - Schema errors: `ErrCodeSchemaLoadFailed`, `ErrCodeSchemaValidation`, `ErrCodeSchemaNotFound`, `ErrCodeSchemaInvalid`

### ✅ AC5: Error Messages Include Context
- **Implementation:** Error messages include:
  - **Position context:** Line and column numbers in format "at line X, column Y"
  - **Path context:** Field path in format "at field server.port"
  - **Expected vs actual:** In format "(expected: X, actual: Y)"
  - **Constraint info:** In format "(constraint: X)"
  - Examples from tests:
    - `parse error in config.yaml at line 10, column 5: invalid syntax (expected: identifier, actual: 123)`
    - `validation error in deployment.yaml at line 15, column 12 at field spec.replicas: port out of range (constraint: must be between 1-65535)`

## Test Results
All error-related tests pass successfully:
- ✅ TestIsYAMLError - Tests YAMLError interface detection
- ✅ TestGetYAMLErrorType - Tests error type classification
- ✅ TestNewParseError - Tests ParseError construction with all parameters
- ✅ TestNewValidationError - Tests ValidationError construction with all parameters
- ✅ TestValidationErrorString - Tests ValidationError formatting
- ✅ TestTypeMismatchErrorFormatting - Tests type mismatch error context
- ✅ TestConstraintErrorFieldPathFormatting - Tests constraint error field paths
- ✅ TestFieldNotFoundErrorFormatting - Tests field not found errors

## Additional Error Types
The implementation also includes specialized error types:
- `FileError` - File I/O errors
- `SyntaxError` - YAML syntax errors
- `StructureError` - YAML structure errors
- `TypeMismatchError` - Type conversion errors
- `FieldNotFoundError` - Missing required fields
- `ConstraintError` - Constraint violations
- `DuplicateKeyError` - Duplicate key errors
- `SchemaLoadError` - Schema loading errors
- `SchemaValidationError` - Schema validation errors

## Conclusion
All acceptance criteria have been met. The error type system provides comprehensive coverage for YAML parsing operations with rich context information for debugging and programmatic error handling.
