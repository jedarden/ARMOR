# Validate() Error Return Patterns

## Overview

This document catalogs all error types returned by Validate() methods and their callers in the ARMOR codebase, along with error handling patterns.

## Validate() Method Implementations

### 1. SchemaDefinition.Validate(value interface{}) error
**Location:** `internal/yamlutil/schema.go:757`

**Error Returns:**
- `*ValidationError` (via `NewValidationError`)
  - `ErrCodeValidationFailed` - when value is nil
  - `ErrCodeSchemaInvalid` - when field has nil definition
- `*TypeMismatchError` (via `NewTypeMismatchError`)
  - `ErrCodeTypeMismatch` - when value is not `map[string]interface{}` or field type mismatch
- `*FieldNotFoundError` (via `NewFieldNotFoundError`)
  - `ErrCodeRequiredField` - when required field is missing
- `*ConstraintError` (via `NewConstraintError`)
  - `ErrCodeConstraintViolation` - min/max/pattern constraint violations
  - `ErrCodeInvalidValue` - when value not in allowed values list

**Error Wrapping:** None - returns direct error types

---

### 2. SchemaValidator.Validate(data interface{}) SchemaValidationResult
**Location:** `internal/yamlutil/schema.go:157`

**Return Type:** `SchemaValidationResult` struct (not an error)

**Error Handling Pattern:**
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Validation failed: %v", err),
    })
    return result
}
```

**Key Characteristic:** Converts `error` to `SchemaValidationResult` with error list

---

### 3. SchemaValidator.ValidateFile(filePath string) SchemaValidationResult
**Location:** `internal/yamlutil/schema.go:212`

**Return Type:** `SchemaValidationResult` struct

**Error Handling Pattern:**
- File read errors → wrapped in `SchemaValidationResult.Errors`
- YAML parse errors → wrapped in `SchemaValidationResult.Errors`
- Schema validation → delegates to `Validate()` method above

---

### 4. StringConstraintImpl.Validate(value interface{}) *ConstraintError
**Location:** `internal/yamlutil/schema_interfaces.go:343`

**Error Returns:**
- `*ConstraintError` (direct struct initialization)
  - Type mismatch errors
  - Min/max length violations
  - Pattern mismatch
  - Value not in allowed list

**Error Wrapping:** None - returns nil on success, `*ConstraintError` on failure

---

### 5. NumberConstraintImpl.Validate(value interface{}) *ConstraintError
**Location:** `internal/yamlutil/schema_interfaces.go:458`

**Error Returns:**
- `*ConstraintError` (direct struct initialization)
  - Non-numeric values
  - Min/max violations (inclusive/exclusive)
  - Multiple of violations

---

### 6. ArrayConstraintImpl.Validate(value interface{}) *ConstraintError
**Location:** `internal/yamlutil/schema_interfaces.go:560`

**Error Returns:**
- `*ConstraintError` (direct struct initialization)
  - Non-array values
  - Min/max items violations
  - Unique items violations

---

### 7. ObjectConstraintImpl.Validate(value interface{}) *ConstraintError
**Location:** `internal/yamlutil/schema_interfaces.go:647`

**Error Returns:**
- `*ConstraintError` (direct struct initialization)
  - Non-object values
  - Required property violations
  - Additional property violations (in strict mode)

---

### 8. BooleanConstraintImpl.Validate(value interface{}) *ConstraintError
**Location:** `internal/yamlutil/schema_interfaces.go:746`

**Error Returns:**
- `*ConstraintError` (direct struct initialization)
  - Non-boolean values

---

### 9. TypeConstraintImpl.Validate(value interface{}) *ConstraintError
**Location:** `internal/yamlutil/schema_interfaces.go:795`

**Error Returns:**
- `*ConstraintError` (direct struct initialization)
  - Type expectation violations

---

### 10. Validator.ValidateString(yamlContent string) ValidationResult
**Location:** `internal/yamlutil/validator.go:109`

**Return Type:** `ValidationResult` struct (not an error)

**Error Handling Pattern:**
```go
result := ValidationResult{
    FilePath: filePath,
    Valid:    true,
    Errors:   []ValidationError{},
    Warnings: []ValidationError{},
}
// ... validation logic ...
return result
```

**Key Characteristic:** Never returns error - always returns `ValidationResult` struct

---

### 11. Validator.ValidateFile(filePath string) ValidationResult
**Location:** `internal/yamlutil/validator.go:152`

**Return Type:** `ValidationResult` struct

**Error Handling Pattern:**
```go
if err != nil {
    result.Valid = false
    ve := LocalValidationError{
        FilePath: filePath,
        Message:  fmt.Sprintf("Failed to read file: %v", err),
        Type:     ErrorTypeIO,
    }
    result.Errors = append(result.Errors, ve.ToValidationError())
    return result
}
```

**Key Characteristic:** Converts I/O errors to `ValidationError` instances

---

## Error Type Hierarchy

### Base Error Types

```
ValidationError (implements YAMLError interface)
├── TypeMismatchError
├── FieldNotFoundError
├── ConstraintError
├── DuplicateKeyError
└── SchemaLoadError

LocalValidationError (convertible to ValidationError)
```

### Error Types by Function

| Method | Returns | Error Type |
|--------|---------|------------|
| `SchemaDefinition.Validate` | `error` | `ValidationError`, `TypeMismatchError`, `FieldNotFoundError`, `ConstraintError` |
| `SchemaValidator.Validate` | `SchemaValidationResult` | Struct containing error list |
| `SchemaValidator.ValidateFile` | `SchemaValidationResult` | Struct containing error list |
| `Validator.ValidateString` | `ValidationResult` | Struct containing error list |
| `Validator.ValidateFile` | `ValidationResult` | Struct containing error list |
| `*ConstraintImpl.Validate` | `*ConstraintError` | Direct struct or nil |

---

## Error Construction Patterns

### 1. NewValidationError Constructor

**Signature:**
```go
func NewValidationError(filePath string, message string, fieldPath string, constraint string, code ErrorCode, line int, column int, errorType ErrorType, path string) *ValidationError
```

**Usage Examples:**
```go
// Nil value check
return NewValidationError("", "value cannot be nil", "", "", ErrCodeValidationFailed, 0, 0, ErrorTypeValidation, "")

// Schema validation error
return NewValidationError("", fmt.Sprintf("field %s has nil definition", fieldName), fieldName, "", ErrCodeSchemaInvalid, 0, 0, ErrorTypeSchemaValidate, "")
```

---

### 2. NewTypeMismatchError Constructor

**Signature:**
```go
func NewTypeMismatchError(filePath string, fieldPath string, expectedType string, actualType string, value string, line int, errorCode ErrorCode) *TypeMismatchError
```

**Usage Examples:**
```go
// Root type mismatch
return NewTypeMismatchError("", "", "map[string]interface{}", fmt.Sprintf("%T", value), "", 0, ErrCodeTypeMismatch)

// Field type mismatch
return NewTypeMismatchError("", fieldPath, fieldDef.Type, s.getTypeName(value), fmt.Sprintf("%v", value), 0, ErrCodeTypeMismatch)
```

---

### 3. NewFieldNotFoundError Constructor

**Signature:**
```go
func NewFieldNotFoundError(filePath string, fieldPath string, line int, errorCode ErrorCode) *FieldNotFoundError
```

**Usage Example:**
```go
return NewFieldNotFoundError("", fieldName, 0, ErrCodeRequiredField)
```

---

### 4. NewConstraintError Constructor

**Signature:**
```go
func NewConstraintError(filePath string, fieldPath string, constraintType string, constraint string, message string, value string, line int, errorCode ErrorCode) *ConstraintError
```

**Usage Examples:**
```go
// Min constraint
return NewConstraintError("", fieldPath, "min", fmt.Sprintf("must be >= %d", *fieldDef.Min), fmt.Sprintf("value violates minimum constraint %d", *fieldDef.Min), fmt.Sprintf("%v", value), 0, ErrCodeConstraintViolation)

// Pattern constraint
return NewConstraintError("", fieldPath, "pattern", fieldDef.Pattern, fmt.Sprintf("value does not match pattern '%s'", fieldDef.Pattern), strVal, 0, ErrCodeConstraintViolation)

// Enum constraint
return NewConstraintError("", fieldPath, "enum", fmt.Sprintf("must be one of: %v", fieldDef.AllowedValues), fmt.Sprintf("value not in allowed list"), fmt.Sprintf("%v", value), 0, ErrCodeInvalidValue)
```

---

### 5. Direct ConstraintError Construction (Constraint Implementations)

**Pattern:**
```go
return &ConstraintError{
    Constraint:     fmt.Sprintf("description here"),
    ConstraintType: "type_of_constraint",
    Value:          fmt.Sprintf("%v", value),
}
```

**Usage Examples:**
```go
// String constraint - type check
return &ConstraintError{
    Constraint:     fmt.Sprintf("value is not a string: %T", value),
    ConstraintType: "string",
    Value:          fmt.Sprintf("%v", value),
}

// String constraint - min length
return &ConstraintError{
    Constraint:     fmt.Sprintf("string length %d is less than minimum %d", len(str), sc.minLength),
    ConstraintType: "min_length",
    Value:          str,
}

// Number constraint - non-numeric
return &ConstraintError{
    Constraint: fmt.Sprintf("value is not a number: %v", err),
}
```

---

## Error Handling Patterns in Callers

### Pattern 1: Direct Error Return

```go
if err := sv.schema.Validate(data); err != nil {
    return err  // Direct propagation
}
```

**Used by:** Low-level validation methods

---

### Pattern 2: Error to Result Struct Conversion

```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Validation failed: %v", err),
    })
    return result
}
```

**Used by:** `SchemaValidator.Validate()`, `SchemaValidator.ValidateFile()`

---

### Pattern 3: Error to ValidationError List Conversion

```go
if err != nil {
    result.Valid = false
    ve := LocalValidationError{
        FilePath: filePath,
        Message:  fmt.Sprintf("Failed to read file: %v", err),
        Type:     ErrorTypeIO,
    }
    result.Errors = append(result.Errors, ve.ToValidationError())
    return result
}
```

**Used by:** `Validator.ValidateFile()`, `Validator.ValidateStringWithPath()`

---

## Error Code Constants

| ErrorCode | Description |
|-----------|-------------|
| `ErrCodeValidationFailed` | General validation failure |
| `ErrCodeTypeMismatch` | Type expectation not met |
| `ErrCodeRequiredField` | Required field missing |
| `ErrCodeConstraintViolation` | Constraint rule violated |
| `ErrCodeInvalidValue` | Value invalid for any reason |
| `ErrCodeSchemaInvalid` | Schema definition error |
| `ErrCodeDuplicateKey` | Duplicate mapping key |

---

## Error Type Constants (ErrorType)

| ErrorType | Description |
|-----------|-------------|
| `ErrorTypeValidation` | General validation error |
| `ErrorTypeTypeMismatch` | Type mismatch |
| `ErrorTypeFieldNotFound` | Required field missing |
| `ErrorTypeConstraint` | Constraint violation |
| `ErrorTypeSyntax` | YAML syntax error |
| `ErrorTypeStructure` | YAML structure error |
| `ErrorTypeIO` | I/O error |
| `ErrorTypeEmpty` | Empty content |
| `ErrorTypeSchemaValidate` | Schema validation error |

---

## Key Observations

### 1. Dual Return Patterns

ARMOR uses two distinct patterns:

- **Error return:** `SchemaDefinition.Validate()` returns `error` directly
- **Result struct:** `SchemaValidator.Validate()` and `Validator.Validate*()` return result structs

This allows both simple error checking and detailed error collection.

### 2. No Error Wrapping

Validate() methods do NOT wrap errors with `fmt.Errorf()` or `errors.Wrap()`. They return clean error types directly.

### 3. Constructor Pattern

All error types use dedicated constructor functions (`New*Error()`) for consistent initialization.

### 4. Struct Conversion

When converting from error return to result struct, errors are converted via string formatting:
```go
Message: fmt.Sprintf("Validation failed: %v", err)
```

### 5. Constraint Error Variations

Two patterns for constraint errors:
- **SchemaDefinition:** Uses `NewConstraintError()` constructor
- **Constraint implementations:** Use direct `&ConstraintError{}` struct initialization

---

## Summary Table

| Validate Method | Return Type | Error Types | Wrapping | Constructor |
|----------------|-------------|-------------|----------|-------------|
| `SchemaDefinition.Validate` | `error` | ValidationError, TypeMismatchError, FieldNotFoundError, ConstraintError | None | New*Error() |
| `SchemaValidator.Validate` | `SchemaValidationResult` | SchemaValidationError (list) | String format | N/A |
| `SchemaValidator.ValidateFile` | `SchemaValidationResult` | SchemaValidationError (list) | String format | N/A |
| `Validator.ValidateString` | `ValidationResult` | ValidationError (list) | LocalValidationError.ToValidationError() | N/A |
| `Validator.ValidateFile` | `ValidationResult` | ValidationError (list) | LocalValidationError.ToValidationError() | N/A |
| `*ConstraintImpl.Validate` | `*ConstraintError` or `nil` | ConstraintError | None | Direct struct init |

---

## Generated: 2026-07-12
