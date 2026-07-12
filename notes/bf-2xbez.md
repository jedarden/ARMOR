# Validate() Implementations and Error Flow Analysis

**Bead ID**: bf-2xbez
**Date**: 2026-07-12
**Scope**: ARMOR yamlutil package validation system

---

## Executive Summary

The ARMOR codebase contains a comprehensive validation system with **two different validation interfaces** and a hierarchical error type system. The `ValidatedSchema` interface is defined but **has no current implementations**, while the codebase actively uses a different `Schema` interface for validation.

---

## 1. All Validate() Implementations

### 1.1 ValidatedSchema Interface (Defined but Not Implemented)

**Location**: `internal/yamlutil/schema_interfaces.go:31-44`

```go
type ValidatedSchema interface {
    Validate() error           // Validates schema definition itself
    Name() string
    Description() string
    Version() string
}
```

**Status**: ✅ Interface defined, ❌ NO implementations found

This interface appears to be part of a planned validation framework that hasn't been fully implemented yet.

---

### 1.2 Schema Interface (Active Usage)

**Location**: `internal/yamlutil/schema.go:38-52`

```go
type Schema interface {
    Validate(value interface{}) error
}
```

**Implementations**: `SchemaDefinition` (line 757)

#### SchemaDefinition.Validate()
**Signature**: `func (s *SchemaDefinition) Validate(value interface{}) error`

**Location**: `internal/yamlutil/schema.go:757-785`

**Purpose**: Validates YAML data against schema definition

**Validation Flow**:
1. Checks for nil values
2. Converts value to `map[string]interface{}`
3. Validates all required fields exist
4. Validates field types and constraints for each existing field

**Returns** (YAMLError types):
- `ValidationError` - General validation failures
- `TypeMismatchError` - Type conversion errors
- `FieldNotFoundError` - Missing required fields
- `ConstraintError` - Constraint violations

---

### 1.3 SchemaValidator.Validate()

**Signature**: `func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult`

**Location**: `internal/yamlutil/schema.go:157-206`

**Purpose**: Comprehensive schema validation with detailed results

**Validation Flow**:
1. Compiles schema if not already compiled (calls `compileSchema()`)
2. Calls underlying schema's `Validate(data)` method
3. For `SchemaDefinition`, performs detailed field validation
4. Populates `SchemaValidationResult` with all errors/warnings

**Returns**: `SchemaValidationResult` containing:
- `Valid` (bool)
- `Errors []SchemaValidationError`
- `Warnings []SchemaValidationError`
- `MissingRequiredFields []string`
- `TypeMismatches []FieldTypeError`
- `ConstraintViolations []ConstraintViolation`

---

### 1.4 SchemaValidator.ValidateFile()

**Signature**: `func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult`

**Location**: `internal/yamlutil/schema.go:212-244`

**Purpose**: File-based validation

**Flow**:
1. Reads file content
2. Parses YAML
3. Delegates to `Validate(data)`

---

### 1.5 SchemaDefinition.Compile()

**Signature**: `func (s *SchemaDefinition) Compile() error`

**Location**: `internal/yamlutil/schema.go:732-748`

**Purpose**: Validates the schema definition itself

**Validates**:
- Schema is not nil
- All field definitions are not nil
- Field types are valid
- Field constraints (min/max) are consistent

**Returns**: `ValidationError` or `SchemaLoadError` (YAMLError types)

---

### 1.6 Constraint Implementations

All constraint types implement `Validate(value interface{}) *ConstraintError`:

| Constraint Type | Location | Purpose |
|----------------|----------|---------|
| `StringConstraintImpl` | schema_interfaces.go:343 | String length, pattern, enum validation |
| `NumberConstraintImpl` | schema_interfaces.go:458 | Numeric range, multipleOf validation |
| `ArrayConstraintImpl` | schema_interfaces.go:560 | Array item count, uniqueness validation |
| `ObjectConstraintImpl` | schema_interfaces.go:647 | Object property count, required fields validation |
| `BooleanConstraintImpl` | schema_interfaces.go:746 | Boolean value validation |
| `TypeConstraintImpl` | schema_interfaces.go:795 | Runtime type checking |

---

## 2. All Validate() Call Sites

### 2.1 Non-Test Callers

#### In SchemaValidator (schema.go)

```go
// Line 180: Validates data against underlying schema
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Validation failed: %v", err),
    })
    return result
}

// Line 243: ValidateFile delegates to Validate
return sv.Validate(data)
```

#### In compileSchema()

```go
// Line 249: Validates schema during compilation
if err := schemaDef.Compile(); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Invalid schema: %v", err),
    })
    return result
}
```

### 2.2 Test-Only Callers

**Location**: `internal/yamlutil/schema_validation_test.go`

The test file references a `Schema` type with `Validate()` method that doesn't match the current implementation. Tests appear to be written for:
- A schema with `Validate()` method (no parameters)
- Metadata methods like `Name()`, `Version()`, `Description()`

**Observation**: Tests reference the `ValidatedSchema` interface pattern, but no such implementation exists in the codebase.

---

## 3. Error Return Patterns

### 3.1 SchemaDefinition.Validate() Error Types

**Returns**: Single `error` implementing `YAMLError` interface

| Error Type | When Returned | Constructor |
|------------|---------------|-------------|
| `ValidationError` | Value is nil | `NewValidationError(..., ErrCodeValidationFailed, ...)` |
| `TypeMismatchError` | Wrong type for field | `NewTypeMismatchError(..., ErrCodeTypeMismatch)` |
| `FieldNotFoundError` | Missing required field | `NewFieldNotFoundError(..., ErrCodeRequiredField)` |
| `ConstraintError` | Constraint violated | `NewConstraintError(..., ErrCodeConstraintViolation)` |

**Example from code** (schema.go:757-785):
```go
func (s *SchemaDefinition) Validate(value interface{}) error {
    if value == nil {
        return NewValidationError("", "value cannot be nil", "", "", 
            ErrCodeValidationFailed, 0, 0, ErrorTypeValidation, "")
    }

    data, ok := value.(map[string]interface{})
    if !ok {
        return NewTypeMismatchError("", "", "map[string]interface{}", 
            fmt.Sprintf("%T", value), "", 0, ErrCodeTypeMismatch)
    }

    for fieldName, fieldDef := range s.RootFields {
        if fieldDef.Required {
            if _, exists := data[fieldName]; !exists {
                return NewFieldNotFoundError("", fieldName, 0, ErrCodeRequiredField)
            }
        }

        if fieldValue, exists := data[fieldName]; exists {
            if err := s.validateField(fieldValue, fieldDef, fieldName); err != nil {
                return err
            }
        }
    }

    return nil
}
```

---

### 3.2 SchemaValidator.Validate() Error Pattern

**Returns**: `SchemaValidationResult` (struct with multiple error slices)

**Error Fields**:
- `Errors []SchemaValidationError` - General validation errors
- `Warnings []SchemaValidationError` - Validation warnings
- `MissingRequiredFields []string` - Paths to missing required fields
- `TypeMismatches []FieldTypeError` - Type mismatch details
- `ConstraintViolations []ConstraintViolation` - Constraint violation details

**Error Conversion** (schema.go:180-185):
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Validation failed: %v", err),
    })
    return result
}
```

**Pattern**: Single YAMLError from `SchemaDefinition.Validate()` is wrapped in a `SchemaValidationError` slice within `SchemaValidationResult`.

---

### 3.3 SchemaDefinition.Compile() Error Pattern

**Returns**: Single `error` implementing `YAMLError` interface

| Error Type | When Returned | Constructor |
|------------|---------------|-------------|
| `SchemaLoadError` | Schema is nil | `NewSchemaLoadError(..., ErrCodeSchemaInvalid)` |
| `ValidationError` | Invalid field definition | `NewValidationError(..., ErrCodeSchemaInvalid)` |

---

## 4. Error Conversion Points

### 4.1 Primary Conversion Point

**Location**: `internal/yamlutil/schema.go:180-185`

**Conversion**: Single YAMLError → SchemaValidationError slice

```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Validation failed: %v", err),
    })
    return result
}
```

**Flow**:
```
SchemaDefinition.Validate()
  ↓ (returns YAMLError)
SchemaValidator.Validate()
  ↓ (wraps in SchemaValidationResult)
Caller receives SchemaValidationResult
```

---

### 4.2 Schema Compilation Error Flow

**Location**: `internal/yamlutil/schema.go:168-177`

**Conversion**: Compile error → SchemaValidationError slice

```go
if !sv.compiled {
    if err := sv.compileSchema(); err != nil {
        result.Valid = false
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Invalid schema: %v", err),
        })
        return result
    }
    sv.compiled = true
}
```

---

### 4.3 Field Validation Error Collection

**Location**: `internal/yamlutil/schema.go:254-288`

**Pattern**: Direct field validation without conversion

```go
func (sv *SchemaValidator) validateFields(
    data map[string]interface{},
    fields map[string]*FieldDefinition,
    pathPrefix string,
    result *SchemaValidationResult,
) {
    // Check required fields
    for fieldName, fieldDef := range fields {
        if fieldDef.Required {
            fullPath := sv.joinPath(pathPrefix, fieldName)
            if _, exists := data[fieldName]; !exists {
                result.Valid = false
                result.MissingRequiredFields = append(result.MissingRequiredFields, fullPath)
            }
        }
    }

    // Validate existing fields
    for fieldName, value := range data {
        fieldDef, exists := fields[fieldName]
        if !exists {
            if sv.config.StrictMode {
                result.Warnings = append(result.Warnings, SchemaValidationError{
                    FieldPath: sv.joinPath(pathPrefix, fieldName),
                    Message:   "Unknown field in strict mode",
                })
            }
            continue
        }

        fullPath := sv.joinPath(pathPrefix, fieldName)
        sv.validateField(value, fieldDef, fullPath, result)
    }
}
```

---

## 5. Error Type Hierarchy

### 5.1 YAMLError Interface

**Base Interface**: `internal/yamlutil/errors.go:31-42`

```go
type YAMLError interface {
    error
    Code() ErrorCode
    YAMLErrorType() ErrorType
    Context() string
}
```

### 5.2 Error Hierarchy

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

### 5.3 Error Codes

**File Errors**:
- `ErrCodeFileNotFound`
- `ErrCodeFileAccessDenied`
- `ErrCodeFileIOError`
- `ErrCodeFileEmpty`

**Parse Errors**:
- `ErrCodeInvalidSyntax`
- `ErrCodeTypeMismatch`
- `ErrCodeInvalidStructure`
- `ErrCodeDuplicateKey`
- `ErrCodeParseError`

**Validation Errors**:
- `ErrCodeValidationFailed`
- `ErrCodeRequiredField`
- `ErrCodeConstraintViolation`
- `ErrCodeInvalidValue`

**Schema Errors**:
- `ErrCodeSchemaLoadFailed`
- `ErrCodeSchemaValidation`
- `ErrCodeSchemaNotFound`
- `ErrCodeSchemaInvalid`

---

## 6. Key Findings

### 6.1 Interface Mismatch

1. **ValidatedSchema Interface**: Defined with `Validate() error` - **NO implementations**
2. **Schema Interface**: Active with `Validate(value interface{}) error` - **HAS implementations**
3. **Tests**: Reference `ValidatedSchema` pattern that doesn't exist in implementation

### 6.2 Current Architecture

The codebase uses a **two-layer validation approach**:

1. **Schema Layer**: `SchemaDefinition` implements `Schema` interface
   - Validates data against schema rules
   - Returns single YAMLError

2. **Validator Layer**: `SchemaValidator` wraps schemas
   - Provides comprehensive validation results
   - Converts single errors to structured result
   - Supports file-based validation

### 6.3 Error Flow Patterns

**Single Error Pattern** (SchemaDefinition):
```
Validate() → single YAMLError
```

**Structured Result Pattern** (SchemaValidator):
```
Validate() → SchemaValidationResult {
    Errors:         []SchemaValidationError
    Warnings:       []SchemaValidationError
    MissingFields:  []string
    TypeMismatches: []FieldTypeError
    ConstraintVios: []ConstraintViolation
}
```

---

## 7. Recommendations

### 7.1 For Consistency

1. **Decide on primary interface**: Either implement `ValidatedSchema` or remove it
2. **Update tests**: Align tests with actual implementation
3. **Document architecture**: Clarify two-layer validation approach

### 7.2 For Error Handling

1. **Maintain YAMLError hierarchy**: Current structure is comprehensive
2. **Document conversion points**: Clearly label where error types are converted
3. **Consider error context**: Add more field path tracking in conversions

---

## 8. File Inventory

### Schema Interfaces
- `internal/yamlutil/schema_interfaces.go` - ValidatedSchema interface definition
- `internal/yamlutil/schema.go` - Schema interface, SchemaDefinition, SchemaValidator

### Error Types
- `internal/yamlutil/errors.go` - YAMLError interface and all error implementations

### Result Types
- `internal/yamlutil/result_types.go` - SchemaValidationResult, ValidationResult

### Tests
- `internal/yamlutil/schema_validation_test.go` - Tests (reference non-existent Schema type)

---

## Summary

The ARMOR validation system has a **well-defined error hierarchy** but **incomplete interface implementation**. The `ValidatedSchema` interface exists without implementations, while the codebase actively uses a different `Schema` interface. The error flow converts single YAMLErrors into structured `SchemaValidationResult` objects, providing comprehensive validation feedback while maintaining type safety through the YAMLError interface hierarchy.
