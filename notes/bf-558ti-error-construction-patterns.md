# Error Construction Patterns in yamlutil Package

**Bead:** bf-558ti  
**Date:** 2026-07-13

## Overview

This document describes the proper patterns for constructing ValidationError and Result types in the `internal/yamlutil` package. The yamlutil package provides a comprehensive error hierarchy with dedicated constructor functions for each error type.

## Key Principle: Always Use Constructor Functions

**NEVER** use direct field access or struct literal initialization for error types. **ALWAYS** use the provided constructor functions.

## Error Type Hierarchy

```
YAMLError (base interface)
├── FileError (file I/O errors)
├── ParseError (YAML parsing errors)
│   ├── SyntaxError (YAML syntax errors)
│   ├── StructureError (YAML structure errors)
│   └── TypeMismatchError (type conversion errors)
├── ValidationError (validation errors)
│   ├── FieldNotFoundError (missing required fields)
│   ├── ConstraintError (constraint violations)
│   └── DuplicateKeyError (duplicate key errors)
└── SchemaError (schema-related errors)
    ├── SchemaLoadError (schema loading errors)
    └── SchemaValidationError (schema validation errors)
```

## Constructor Functions Reference

### 1. ValidationError

```go
func NewValidationError(
    filePath string,
    message string,
    fieldPath string,
    constraint string,
    code ErrorCode,
    line int,
    column int,
    errorType ErrorType,
    path string,
    expectedType string,
    actualType string,
) *ValidationError
```

**Example usage:**

```go
// Basic validation error
err := NewValidationError(
    "config.yaml",           // filePath
    "invalid port number",   // message
    "server.port",          // fieldPath
    "",                     // constraint (optional)
    "",                     // code (defaults to ErrCodeValidationFailed)
    0,                      // line (optional)
    0,                      // column (optional)
    "",                     // errorType (defaults to ErrorTypeValidation)
    "",                     // path (optional)
    "",                     // expectedType (optional)
    "",                     // actualType (optional)
)

// Full-featured validation error with all parameters
err := NewValidationError(
    "config.yaml",                    // filePath
    "port must be between 1-65535",   // message
    "server.port",                   // fieldPath
    "must be between 1-65535",       // constraint
    ErrCodeInvalidValue,             // code
    10,                              // line
    5,                               // column
    ErrorTypeConstraint,             // errorType
    "server.port",                   // path
    "integer",                       // expectedType
    "string",                        // actualType
)
```

### 2. SyntaxError

```go
func NewSyntaxError(
    filePath string,
    message string,
    line int,
    column int,
    expected string,
    found string,
    errorCode ErrorCode,
) *SyntaxError
```

**Example usage:**

```go
err := NewSyntaxError(
    "config.yaml",           // filePath
    "missing colon",         // message
    10,                      // line
    5,                       // column
    ":",                     // expected (optional)
    "",                      // found (optional)
    ErrCodeInvalidSyntax,   // errorCode (optional)
)
```

### 3. StructureError

```go
func NewStructureError(
    filePath string,
    message string,
    line int,
    duplicateKey string,
    location string,
    errorCode ErrorCode,
) *StructureError
```

**Example usage:**

```go
err := NewStructureError(
    "config.yaml",            // filePath
    "duplicate key detected", // message
    15,                       // line
    "server.port",           // duplicateKey (optional)
    "spec.server",           // location (optional)
    ErrCodeDuplicateKey,     // errorCode (optional)
)
```

### 4. TypeMismatchError

```go
func NewTypeMismatchError(
    filePath string,
    fieldPath string,
    expectedType string,
    actualType string,
    value string,
    line int,
    errorCode ErrorCode,
) *TypeMismatchError
```

**Example usage:**

```go
err := NewTypeMismatchError(
    "config.yaml",        // filePath
    "server.port",       // fieldPath
    "integer",           // expectedType
    "string",            // actualType
    "8080",              // value (the actual string value)
    20,                  // line
    ErrCodeTypeMismatch, // errorCode (optional)
)
```

### 5. FieldNotFoundError

```go
func NewFieldNotFoundError(
    filePath string,
    fieldPath string,
    line int,
    errorCode ErrorCode,
) *FieldNotFoundError
```

**Example usage:**

```go
err := NewFieldNotFoundError(
    "config.yaml",          // filePath
    "database.host",       // fieldPath
    8,                     // line (optional)
    ErrCodeRequiredField,  // errorCode (optional)
)
```

### 6. ConstraintError

```go
func NewConstraintError(
    filePath string,
    fieldPath string,
    constraintType string,
    constraint string,
    message string,
    value string,
    line int,
    errorCode ErrorCode,
) *ConstraintError
```

**Example usage:**

```go
err := NewConstraintError(
    "config.yaml",               // filePath
    "server.port",              // fieldPath
    "range",                   // constraintType
    "must be between 1-65535", // constraint
    "port out of range",       // message
    "70000",                   // value (the actual invalid value)
    12,                        // line
    ErrCodeConstraintViolation, // errorCode (optional)
)
```

### 7. DuplicateKeyError

```go
func NewDuplicateKeyError(
    filePath string,
    key string,
    location string,
    line1 int,
    line2 int,
    code ErrorCode,
) *DuplicateKeyError
```

**Example usage:**

```go
err := NewDuplicateKeyError(
    "config.yaml",      // filePath
    "server.port",     // key
    "spec.server",     // location (nested path, optional)
    10,                // line1 (first occurrence)
    25,                // line2 (duplicate occurrence)
    ErrCodeDuplicateKey, // code (optional)
)
```

### 8. ParseError

```go
func NewParseError(
    filePath string,
    message string,
    line int,
    column int,
    code ErrorCode,
    expected string,
    actual string,
    contextStr string,
) *ParseError
```

**Example usage:**

```go
err := NewParseError(
    "config.yaml",       // filePath
    "invalid syntax",    // message
    10,                  // line
    5,                   // column
    ErrCodeInvalidSyntax, // code (optional)
    "identifier",        // expected (optional)
    "123",              // actual (optional)
    "while parsing config", // contextStr (optional)
)
```

### 9. SchemaLoadError

```go
func NewSchemaLoadError(
    filePath string,
    message string,
    err error,
    code ErrorCode,
) *SchemaLoadError
```

**Example usage:**

```go
err := NewSchemaLoadError(
    "schema.yaml",                  // filePath
    "failed to parse schema",       // message
    underlyingErr,                 // err (optional)
    ErrCodeSchemaLoadFailed,       // code (optional)
)
```

## Best Practices

### 1. Use Empty Strings/Zero Values for Optional Parameters

For optional parameters, pass empty strings or zero values. The constructor functions will apply appropriate defaults.

```go
// Good - using defaults for optional parameters
err := NewValidationError(
    "config.yaml",
    "invalid value",
    "server.port",
    "",  // constraint (optional)
    "",  // code (defaults to ErrCodeValidationFailed)
    0,   // line (unknown)
    0,   // column (unknown)
    "",  // errorType (defaults to ErrorTypeValidation)
    "",  // path
    "",  // expectedType
    "",  // actualType
)
```

### 2. Use Specific Error Codes

When the error type is known, use the specific error code constants:

```go
// Available error codes (from errors.go):
//   - ErrCodeFileNotFound
//   - ErrCodeFileAccessDenied
//   - ErrCodeFileIOError
//   - ErrCodeFileEmpty
//   - ErrCodeInvalidSyntax
//   - ErrCodeTypeMismatch
//   - ErrCodeInvalidStructure
//   - ErrCodeDuplicateKey
//   - ErrCodeValidationFailed
//   - ErrCodeRequiredField
//   - ErrCodeConstraintViolation
//   - ErrCodeInvalidValue
//   - ErrCodeSchemaLoadFailed
//   - ErrCodeSchemaValidation
//   - ErrCodeSchemaNotFound
//   - ErrCodeSchemaInvalid
```

### 3. Provide Field Paths for Validation Errors

When creating validation errors, always provide the field path using dot notation:

```go
err := NewValidationError(
    "config.yaml",
    "invalid port",
    "spec.server.port",  // Full field path
    "must be 1-65535",
    ErrCodeInvalidValue,
    10,
    0,
    ErrorTypeConstraint,
    "spec.server.port",  // path parameter (same as fieldPath)
    "integer",
    "string",
)
```

### 4. Provide Line Numbers When Available

Line numbers significantly improve error messages. Always provide them when parsing YAML:

```go
err := NewTypeMismatchError(
    "config.yaml",
    "server.port",
    "integer",
    "string",
    "8080",
    42,  // Line number where error occurred
    ErrCodeTypeMismatch,
)
```

## What NOT To Do

### ❌ Direct Field Assignment

```go
// BAD - Never do this
ve := &ValidationError{
    FilePath:  "config.yaml",
    Message:   "error",
    FieldPath: "server.port",
}
```

### ❌ Struct Literals Without Constructor

```go
// BAD - Never do this
ve := ValidationError{
    FilePath:  "config.yaml",
    Message:   "error",
}
```

### ❌ Creating Errors Then Modifying Fields

```go
// BAD - Constructor then field modification
err := NewValidationError(...)
err.Line = 10  // Don't modify fields after creation
```

## Testing Error Construction

When writing tests for error creation, verify that the constructor properly initializes all fields:

```go
func TestNewValidationError(t *testing.T) {
    err := NewValidationError(
        "config.yaml",
        "invalid port",
        "server.port",
        "must be 1-65535",
        ErrCodeInvalidValue,
        10,
        5,
        ErrorTypeConstraint,
        "server.port",
        "integer",
        "string",
    )

    // Verify field values
    if err.FilePath != "config.yaml" {
        t.Errorf("FilePath = %q, want %q", err.FilePath, "config.yaml")
    }
    if err.FieldPath != "server.port" {
        t.Errorf("FieldPath = %q, want %q", err.FieldPath, "server.port")
    }
    if err.Line != 10 {
        t.Errorf("Line = %d, want %d", err.Line, 10)
    }
    // ... verify other fields

    // Verify YAMLError interface implementation
    var ye YAMLError = err
    if ye.Code() != ErrCodeInvalidValue {
        t.Errorf("Code() = %q, want %q", ye.Code(), ErrCodeInvalidValue)
    }
    if ye.YAMLErrorType() != ErrorTypeConstraint {
        t.Errorf("YAMLErrorType() = %q, want %q", ye.YAMLErrorType(), ErrorTypeConstraint)
    }
}
```

## Summary

The yamlutil package provides a comprehensive set of constructor functions for all error types. These constructors:

1. **Initialize all fields properly** - No uninitialized fields
2. **Handle defaults automatically** - Empty codes/types get sensible defaults
3. **Implement the YAMLError interface** - Consistent error handling
4. **Provide type-safe construction** - Compiler checks parameter types
5. **Support optional parameters** - Empty string/zero for unknown values

**Always use these constructor functions instead of direct field access or struct literals.**
