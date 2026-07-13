# Error Construction Patterns in yamlutil Package

## Overview

This document documents the proper patterns for constructing error types in the `internal/yamlutil` package. The package provides constructor functions that should be used instead of direct struct initialization.

## Constructor Functions

### ValidationError Construction

**Constructor:** `NewValidationError(filePath, message, fieldPath, constraint, code, line, column, errorType, path, expectedType, actualType string) *ValidationError`

**Example Usage:**
```go
// Basic validation error
err := NewValidationError(
    "config.yaml",           // filePath
    "invalid port number",   // message
    "server.port",          // fieldPath
    "must be 1-65535",      // constraint
    ErrCodeInvalidValue,    // code
    10,                     // line (1-indexed, use 0 if unknown)
    5,                      // column (1-indexed, use 0 if unknown)
    "",                     // errorType (empty for default ErrorTypeValidation)
    "",                     // path (empty defaults to fieldPath)
    "integer",              // expectedType
    "string",               // actualType
)
```

**Field Documentation:**
- `filePath`: Path to the file being validated
- `message`: Human-readable error message
- `fieldPath`: Dot-notation path to the invalid field (e.g., "server.port")
- `constraint`: Constraint that was violated (optional)
- `code`: `ErrorCode` for programmatic handling (use empty string `""` for default `ErrCodeValidationFailed`)
- `line`: Line number where error occurred (1-indexed, use 0 if unknown)
- `column`: Column number where error occurred (1-indexed, use 0 if unknown)
- `errorType`: `ErrorType` category (use empty string `""` for default `ErrorTypeValidation`)
- `path`: Dot-notation field path (optional, for backward compatibility defaults to empty string)
- `expectedType`: Expected type for type mismatch errors (optional)
- `actualType`: Actual type found for type mismatch errors (optional)

### ParseError Construction

**Constructor:** `NewParseError(filePath, message, line, column, code, expected, actual, contextStr string) *ParseError`

**Example Usage:**
```go
err := NewParseError(
    "config.yaml",           // filePath
    "invalid syntax",        // message
    10,                     // line (1-indexed, use 0 if unknown)
    5,                      // column (1-indexed, use 0 if unknown)
    ErrCodeInvalidSyntax,   // code (use "" for default ErrCodeParseError)
    "identifier",           // expected (what was expected)
    "123",                  // actual (what was actually found)
    "while parsing config",  // contextStr (additional context)
)
```

### SyntaxError Construction

**Constructor:** `NewSyntaxError(filePath, message, line, column, expected, found, errorCode string) *SyntaxError`

**Example Usage:**
```go
err := NewSyntaxError(
    "config.yaml",           // filePath
    "missing colon",         // message
    10,                     // line
    5,                      // column
    ":",                    // expected
    "",                     // found
    ErrCodeInvalidSyntax,   // errorCode (use "" for default)
)
```

### StructureError Construction

**Constructor:** `NewStructureError(filePath, message, line, duplicateKey, location, errorCode string) *StructureError`

**Example Usage:**
```go
err := NewStructureError(
    "config.yaml",           // filePath
    "duplicate key detected", // message
    10,                     // line
    "port",                 // duplicateKey (optional)
    "in server section",    // location (optional)
    ErrCodeDuplicateKey,    // errorCode (use "" for default)
)
```

### TypeMismatchError Construction

**Constructor:** `NewTypeMismatchError(filePath, fieldPath, expectedType, actualType, value string, line int, errorCode ErrorCode) *TypeMismatchError`

**Example Usage:**
```go
err := NewTypeMismatchError(
    "config.yaml",           // filePath
    "server.port",          // fieldPath
    "integer",              // expectedType
    "string",               // actualType
    "8080",                 // value (the actual value that caused error)
    8,                      // line
    ErrCodeTypeMismatch,    // errorCode (use "" for default)
)
```

### FieldNotFoundError Construction

**Constructor:** `NewFieldNotFoundError(filePath, fieldPath string, line int, errorCode ErrorCode) *FieldNotFoundError`

**Example Usage:**
```go
err := NewFieldNotFoundError(
    "config.yaml",           // filePath
    "database.host",        // fieldPath
    5,                      // line (use 0 if unknown)
    ErrCodeRequiredField,   // errorCode (use "" for default)
)
```

### ConstraintError Construction

**Constructor:** `NewConstraintError(filePath, fieldPath, constraintType, constraint, message, value string, line int, errorCode ErrorCode) *ConstraintError`

**Example Usage:**
```go
err := NewConstraintError(
    "config.yaml",           // filePath
    "server.port",          // fieldPath
    "range",                // constraintType (range, length, pattern, enum)
    "must be 1-65535",     // constraint
    "value violates minimum constraint", // message
    "70000",               // value
    10,                    // line
    ErrCodeConstraintViolation, // errorCode (use "" for default)
)
```

### DuplicateKeyError Construction

**Constructor:** `NewDuplicateKeyError(filePath, key, location string, line1, line2 int, code ErrorCode) *DuplicateKeyError`

**Example Usage:**
```go
err := NewDuplicateKeyError(
    "config.yaml",           // filePath
    "port",                 // key
    "server section",       // location (nested path to duplicate key)
    5,                      // line1 (first occurrence)
    10,                     // line2 (duplicate occurrence)
    ErrCodeDuplicateKey,    // code (use "" for default ErrCodeDuplicateKey)
)
```

### SchemaLoadError Construction

**Constructor:** `NewSchemaLoadError(filePath, message string, err error, code ErrorCode) *SchemaLoadError`

**Example Usage:**
```go
err := NewSchemaLoadError(
    "schema.yaml",          // filePath
    "failed to parse schema", // message
    underlyingErr,         // err (underlying error, use nil if not applicable)
    ErrCodeSchemaLoadFailed, // code (use "" for default)
)
```

### SchemaValidationError Construction

**Constructor:** `NewSchemaValidationError(filePath, schemaPath, fieldPath, message, expected, found string, line int, errorCode ErrorCode) *SchemaValidationError`

**Example Usage:**
```go
err := NewSchemaValidationError(
    "config.yaml",         // filePath
    "schema.yaml",         // schemaPath
    "server.port",        // fieldPath
    "type mismatch",      // message
    "integer",            // expected
    "string",             // found
    10,                   // line
    ErrCodeSchemaValidation, // errorCode (use "" for default)
)
```

### FileError Construction

**Constructor:** `NewFileError(path, operation, message string, err error) *FileError`

**Example Usage:**
```go
err := NewFileError(
    "config.yaml",         // path
    "read",               // operation (read, write, etc.)
    "file not found",     // message
    os.ErrNotExist,       // err (underlying error, use nil if not applicable)
)
```

## Helper Methods for Setting Fields

### WithContext (for Result types)

The `result.go` file provides helper methods for adding context to errors in Result chains:

```go
// WithContext adds context information to a ParseError in a Result
func WithContext[T any](r Result[T, *ParseError], context string) Result[T, *ParseError]

// WithLineNumber adds line number information to a ParseError in a Result
func WithLineNumber[T any](r Result[T, *ParseError], line int) Result[T, *ParseError]
```

**Example:**
```go
result := ParseConfig(data)
result = WithContext(result, "while parsing main config")
result = WithLineNumber(result, 42)
```

### MapErr (for Result transformation)

The `MapErr` method on Result allows transforming errors while preserving the error state:

```go
func (r Result[T, E]) MapErr(f func(E) E) Result[T, E]
```

**Example:**
```go
result := ParseConfig(data)
annotated := result.MapErr(func(e ParseError) ParseError {
    e.ContextStr = "while parsing main config"
    return e
})
```

## Proper Initialization Patterns

### DO: Use Constructor Functions

```go
// ✅ GOOD: Using constructor
err := NewValidationError("config.yaml", "invalid value", "server.port", "", "", 0, 0, "", "", "", "")

// ✅ GOOD: Using constructor with all parameters
err := NewTypeMismatchError("config.yaml", "server.port", "integer", "string", "8080", 10, ErrCodeTypeMismatch)
```

### DON'T: Direct Struct Initialization

```go
// ❌ BAD: Direct struct initialization
err := ValidationError{
    FilePath:  "config.yaml",
    Message:   "invalid value",
    FieldPath: "server.port",
    // Missing: ErrorCode, ErrorType, and other fields
}

// ❌ BAD: Direct struct initialization with partial fields
syntaxErr := SyntaxError{
    Message:   "syntax error",
    ErrorCode: ErrCodeInvalidSyntax,
    // Missing: FilePath, proper initialization
}
```

## Why Constructors Are Required

1. **Proper Field Initialization**: Constructors initialize all required fields including `ErrorCode`, `ErrorType`, and other metadata
2. **Default Value Handling**: Constructors provide sensible defaults for optional fields
3. **Interface Compliance**: Constructors ensure the error properly implements the `YAMLError` interface
4. **Future Compatibility**: Using constructors protects against field additions/reordering in struct definitions
5. **Consistent Error Messages**: Properly constructed errors produce consistent, formatted error messages

## Common Patterns

### Pattern 1: Validation Error with Type Information

```go
return NewValidationError(
    filePath,
    "value must be integer",
    fieldPath,
    "",
    ErrCodeTypeMismatch,
    line,
    0,
    ErrorTypeValidation,
    "",
    "integer",
    "string",
)
```

### Pattern 2: Simple Field Not Found

```go
return NewFieldNotFoundError(
    filePath,
    "database.host",
    5,
    ErrCodeRequiredField,
)
```

### Pattern 3: Constraint Violation

```go
return NewConstraintError(
    filePath,
    "server.port",
    "range",
    "must be 1-65535",
    "port number out of range",
    "70000",
    10,
    ErrCodeConstraintViolation,
)
```

### Pattern 4: Syntax Error with Expected/Actual

```go
return NewSyntaxError(
    filePath,
    "invalid YAML syntax",
    line,
    column,
    "indentation",
    "tab",
    ErrCodeInvalidSyntax,
)
```

### Pattern 5: Duplicate Key Detection

```go
return NewDuplicateKeyError(
    filePath,
    key,
    "in server section",
    firstLine,
    duplicateLine,
    ErrCodeDuplicateKey,
)
```

## Error Code Constants

Use the predefined error code constants from `errors.go`:

```go
// File Error Codes
ErrCodeFileNotFound
ErrCodeFileAccessDenied
ErrCodeFileIOError
ErrCodeFileEmpty

// Parse Error Codes
ErrCodeInvalidSyntax
ErrCodeTypeMismatch
ErrCodeInvalidStructure
ErrCodeDuplicateKey
ErrCodeParseError

// Validation Error Codes
ErrCodeValidationFailed
ErrCodeRequiredField
ErrCodeConstraintViolation
ErrCodeInvalidValue

// Schema Error Codes
ErrCodeSchemaLoadFailed
ErrCodeSchemaValidation
ErrCodeSchemaNotFound
ErrCodeSchemaInvalid
```

## Error Type Constants

Use the predefined error type constants:

```go
ErrorTypeFile
ErrorTypeParse
ErrorTypeSyntax
ErrorTypeStructure
ErrorTypeTypeMismatch
ErrorTypeValidation
ErrorTypeFieldNotFound
ErrorTypeConstraint
ErrorTypeDuplicateKey
ErrorTypeSchema
ErrorTypeSchemaLoad
ErrorTypeSchemaValidate
ErrorTypeUnknown
ErrorTypeEmpty
ErrorTypeIO
```

## Testing Patterns

When writing tests, use constructors to ensure errors are properly initialized:

```go
func TestSomething(t *testing.T) {
    // Use constructor for test errors
    err := NewValidationError(
        "test.yaml",
        "test error",
        "field.path",
        "",
        "",
        0,
        0,
        "",
        "",
        "",
        "",
    )
    
    // Verify error properties
    if err.Code() != ErrCodeValidationFailed {
        t.Errorf("Expected error code %v, got %v", ErrCodeValidationFailed, err.Code())
    }
}
```

## Migration from Direct Initialization

If you find code using direct struct initialization, migrate it to use constructors:

### Before (❌ Bad)
```go
err := ValidationError{
    FilePath:  "config.yaml",
    Message:   "invalid value",
    FieldPath: "server.port",
    Line:      10,
}
```

### After (✅ Good)
```go
err := NewValidationError(
    "config.yaml",
    "invalid value",
    "server.port",
    "",         // constraint
    "",         // code (uses default)
    10,         // line
    0,          // column
    "",         // errorType (uses default)
    "",         // path
    "",         // expectedType
    "",         // actualType
)
```

## Summary

1. **Always use constructor functions** - `NewValidationError`, `NewSyntaxError`, etc.
2. **Never use direct struct initialization** - e.g., `ValidationError{...}`
3. **Use empty string `""` for optional parameters** - constructors will provide defaults
4. **Use predefined constants** - `ErrCode*`, `ErrorType*` constants for codes and types
5. **Provide line numbers when available** - use 0 when unknown (1-indexed when known)
6. **Include field paths** - use dot notation (e.g., "server.port") for nested fields

## References

- Error type definitions: `internal/yamlutil/errors.go`
- Result type utilities: `internal/yamlutil/result.go`
- Example usage: Test files in `internal/yamlutil/*_test.go`
