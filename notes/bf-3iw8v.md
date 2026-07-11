# Task bf-3iw8v: Define error code constants

## Status: Already Complete

The error code constants required by this task were already fully implemented in `/home/coding/ARMOR/internal/yamlutil/errors.go` (lines 71-96).

## Implementation Details

### ErrorCode Type Definition
```go
type ErrorCode string
```
Defined at line 69 - ErrorCode is a string type providing machine-readable error identifiers.

### Error Code Constants (16 total)

#### File Error Codes (4)
- `ErrCodeFileNotFound` - "FILE_NOT_FOUND" - File does not exist
- `ErrCodeFileAccessDenied` - "FILE_ACCESS_DENIED" - Permission denied
- `ErrCodeFileIOError` - "FILE_IO_ERROR" - Generic I/O error
- `ErrCodeFileEmpty` - "FILE_EMPTY" - File is empty

#### Parse Error Codes (5)
- `ErrCodeInvalidSyntax` - "INVALID_SYNTAX" - YAML syntax error ✅ (required)
- `ErrCodeTypeMismatch` - "TYPE_MISMATCH" - Type conversion error ✅ (required)
- `ErrCodeInvalidStructure` - "INVALID_STRUCTURE" - YAML structure error
- `ErrCodeDuplicateKey` - "DUPLICATE_KEY" - Duplicate mapping key
- `ErrCodeParseError` - "PARSE_ERROR" - Generic parse error

#### Validation Error Codes (4)
- `ErrCodeValidationFailed` - "VALIDATION_FAILED" - Validation failed
- `ErrCodeRequiredField` - "REQUIRED_FIELD" - Missing required field
- `ErrCodeConstraintViolation` - "CONSTRAINT_VIOLATION" - Constraint violated
- `ErrCodeInvalidValue` - "INVALID_VALUE" - Invalid value

#### Schema Error Codes (4)
- `ErrCodeSchemaLoadFailed` - "SCHEMA_LOAD_FAILED" - Schema loading failed
- `ErrCodeSchemaValidation` - "SCHEMA_VALIDATION" - Schema validation failed
- `ErrCodeSchemaNotFound` - "SCHEMA_NOT_FOUND" - Schema not found
- `ErrCodeSchemaInvalid` - "SCHEMA_INVALID" - Invalid schema definition

## Acceptance Criteria Met

- ✅ Error code constants defined in errors.go
- ✅ At minimum: ErrCodeInvalidSyntax, ErrCodeTypeMismatch (both present)
- ✅ Constants are string type (via ErrorCode type)
- ✅ Each constant has documentation comment explaining its meaning

## Usage Example

```go
// Creating a ParseError with error code
err := &ParseError{
    FilePath:  "config.yaml",
    Line:      42,
    Message:   "Unexpected character",
    ErrorCode: ErrCodeInvalidSyntax,
}

// Checking error codes programmatically
if ye.Code() == ErrCodeInvalidSyntax {
    // Handle syntax errors
}
```

## Notes

The error code constants are used by all YAMLError implementations (ParseError, ValidationError, FileError, SyntaxError, StructureError, TypeMismatchError, FieldNotFoundError, ConstraintError, DuplicateKeyError, SchemaLoadError, SchemaValidationError) for consistent error categorization and programmatic error handling.
