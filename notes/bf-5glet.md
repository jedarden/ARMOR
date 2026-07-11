# Task bf-5glet: Define YAMLError base interface

## Status: Already Complete

The YAMLError base interface was already defined in `internal/yamlutil/errors.go` (lines 27-42).

## Existing Implementation

The YAMLError interface provides:

### Interface Definition
```go
type YAMLError interface {
	error
	Code() ErrorCode
	YAMLErrorType() ErrorType
	Context() string
}
```

### Methods
- `Code() ErrorCode` - Returns the error code for programmatic error handling
- `Error() string` - From stdlib errors interface (embedded)
- `YAMLErrorType() ErrorType` - Returns the category of error for type switching
- `Context() string` - Returns additional context about the error

### Acceptance Criteria Met
- ✓ YAMLError interface exists in internal/yamlutil/errors.go
- ✓ Interface has Code() method (returns ErrorCode, a string-based type)
- ✓ Interface has Error() method (from stdlib errors)
- ✓ Interface is documented with godoc comments

### Error Hierarchy
The interface serves as the foundation for a comprehensive error hierarchy:
- FileError (file I/O errors)
- ParseError (YAML parsing errors)
  - SyntaxError (YAML syntax errors)
  - StructureError (YAML structure errors)
  - TypeMismatchError (type conversion errors)
- ValidationError (validation errors)
  - FieldNotFoundError (missing required fields)
  - ConstraintError (constraint violations)
  - DuplicateKeyError (duplicate key errors)
- SchemaError (schema-related errors)
  - SchemaLoadError (schema loading errors)
  - SchemaValidationError (schema validation errors)

All error types implement the YAMLError interface, providing a common foundation for error handling throughout the yamlutil package.
