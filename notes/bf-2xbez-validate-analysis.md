# Validate() Implementations and Error Flow Analysis

**Bead**: bf-2xbez  
**Task**: Analyze Validate() implementations and error flow  
**Date**: 2026-07-12

## Summary

This analysis identified critical gaps in the ARMOR codebase regarding schema validation interfaces and error handling patterns. The `ValidatedSchema` interface is defined but **has zero implementations**, while validation is handled through the separate `Schema` interface with inconsistent error return patterns.

---

## 1. ValidatedSchema Interface

**Location**: `internal/yamlutil/schema_interfaces.go:31-44`

```go
type ValidatedSchema interface {
    // Validate checks if the schema definition itself is valid.
    // Returns a YAMLError if the schema has invalid configuration.
    Validate() YAMLError
    
    // Name returns the schema name identifier.
    Name() string
    
    // Description returns a human-readable description of the schema.
    Description() string
    
    // Version returns the schema version for compatibility tracking.
    Version() string
}
```

### Status: **NO IMPLEMENTATIONS FOUND**

No types in the codebase implement the `ValidatedSchema` interface. The interface is defined but unused.

---

## 2. Schema Interface (Different from ValidatedSchema)

**Location**: `internal/yamlutil/schema.go:38-52`

```go
type Schema interface {
    // Validate validates the given value against the schema rules.
    Validate(value interface{}) error
}
```

### Implementation: SchemaDefinition

**Location**: `internal/yamlutil/schema.go:59-80`

```go
type SchemaDefinition struct {
    Type          SchemaType
    Name          string
    Description   string
    Version       string
    RootFields    map[string]*FieldDefinition
    NestedSchemas map[string]*Schema
    definitions   map[string]*FieldDefinition
}
```

**Methods**:
- `Compile() error` - Validates the schema definition itself (line 732)
- `Validate(value interface{}) error` - Validates data against schema (line 757)

---

## 3. Current Validate() Call Patterns

### 3.1 SchemaValidator.Validate() Call Site

**Location**: `internal/yamlutil/schema.go:180-186`

```go
// Compile schema if not already compiled
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

// Validate data against schema
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Validation failed: %v", err),
    })
    return result
}
```

**Error Pattern**: YAMLError → string conversion → SchemaValidationError

---

## 4. Error Return Patterns in SchemaDefinition

### 4.1 Compile() Method (schema validation)

**Location**: `internal/yamlutil/schema.go:732-748`

```go
func (s *SchemaDefinition) Compile() error {
    if s == nil {
        return NewSchemaLoadError("", "schema is nil", nil, ErrCodeSchemaInvalid)
    }
    
    for fieldName, fieldDef := range s.RootFields {
        if fieldDef == nil {
            return NewValidationError("", fmt.Sprintf("field %s has nil definition", fieldName), 
                fieldName, "", ErrCodeSchemaInvalid, 0, 0, ErrorTypeSchemaValidate, "")
        }
        if err := s.validateFieldDefinition(fieldDef, fieldName); err != nil {
            return err
        }
    }
    return nil
}
```

**Error Types Returned**:
- `SchemaLoadError` via `NewSchemaLoadError()`
- `ValidationError` via `NewValidationError()`
- Returns as `error` interface, not `YAMLError`

### 4.2 Validate() Method (data validation)

**Location**: `internal/yamlutil/schema.go:757-785`

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

**Error Types Returned**:
- `ValidationError` via `NewValidationError()`
- `TypeMismatchError` via `NewTypeMismatchError()`
- `FieldNotFoundError` via `NewFieldNotFoundError()`
- `ConstraintError` via `NewConstraintError()` (from validateField)
- All returned as `error` interface

---

## 5. Error Conversion Points

### 5.1 Primary Conversion Locations

| Location | Method | From | To | Pattern |
|----------|--------|------|-----|---------|
| schema.go:734 | Compile() | NewSchemaLoadError() | error | YAMLError → error |
| schema.go:740 | Compile() | NewValidationError() | error | YAMLError → error |
| schema.go:759 | Validate() | NewValidationError() | error | YAMLError → error |
| schema.go:765 | Validate() | NewTypeMismatchError() | error | YAMLError → error |
| schema.go:772 | Validate() | NewFieldNotFoundError() | error | YAMLError → error |
| schema.go:845 | validateConstraints() | NewConstraintError() | error | YAMLError → error |

### 5.2 Secondary Conversion (SchemaValidator)

**Location**: `schema.go:180-186`

```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Validation failed: %v", err),
    })
    return result
}
```

**Pattern**: `error` → string → `SchemaValidationError`

**Loss of Information**:
- Original YAMLError type information is lost
- Error code is discarded
- Field path and line information is lost in string conversion
- Structured error context is lost

---

## 6. YAMLError Hierarchy

**Location**: `internal/yamlutil/errors.go:27-42`

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

**All YAMLError types provide**:
- `Code() ErrorCode` - Machine-readable error code
- `YAMLErrorType() ErrorType` - Category for type switching
- `Context() string` - Additional error context

---

## 7. Current Error Handling Issues

### Issue 1: Interface Mismatch
- `ValidatedSchema` interface requires `Validate() YAMLError`
- No implementations exist
- Actual validation uses `Schema` interface with `Validate(value interface{}) error`

### Issue 2: Type Erasure
- YAMLError types are created but returned as plain `error`
- Type information is lost at interface boundary
- Callers cannot use type assertions to access YAMLError methods

### Issue 3: Double Wrapping
- YAMLError → error → string → SchemaValidationError
- Each conversion loses information
- Structured error data is discarded

### Issue 4: Inconsistent Patterns
- Compile() returns error (should return YAMLError per ValidatedSchema contract)
- Validate() returns error (different signature than ValidatedSchema requires)
- Error handling varies between methods

---

## 8. SchemaValidationHandler Interface

**Location**: `internal/yamlutil/schema_interfaces.go:60-76`

```go
type SchemaValidationHandler interface {
    // ValidateSchema validates the schema definition itself.
    ValidateSchema(schema ValidatedSchema) YAMLError
    
    // ValidateValue validates a single value against a field definition.
    ValidateValue(fieldPath string, value interface{}, fieldDef *FieldDefinition) YAMLError
    
    // Validate validates YAML data against the schema.
    Validate(data map[string]interface{}) SchemaValidationResult
    
    // ValidateFile validates a YAML file against the schema.
    ValidateFile(filePath string) SchemaValidationResult
}
```

**Status**: **NO IMPLEMENTATIONS FOUND**

This interface is also defined but has no implementations.

---

## 9. Constraint Interface Implementations

**Location**: `internal/yamlutil/schema_interfaces.go:86-96`

```go
type Constraint interface {
    Validate(value interface{}) *ConstraintError
    Description() string
    ConstraintType() string
}
```

**Implementations Found**:
- `StringConstraintImpl` (line 316)
- `NumberConstraintImpl` (line 426)
- `ArrayConstraintImpl` (line 538)
- `ObjectConstraintImpl` (line 624)
- `BooleanConstraintImpl` (line 730)
- `TypeConstraintImpl` (line 778)

**Pattern**: All return `*ConstraintError` (which is a YAMLError type)

---

## 10. Recommendations

### 10.1 Align Interface Definitions
**Option A**: Implement ValidatedSchema
- Add `Validate() YAMLError` to SchemaDefinition (schema validation)
- Add `Name()`, `Description()`, `Version()` methods
- Keep existing `Validate(value interface{}) error` as data validation

**Option B**: Remove Unused Interfaces
- Remove ValidatedSchema interface (unused)
- Remove SchemaValidationHandler interface (unused)
- Document Schema as the primary validation interface

### 10.2 Fix Type Erasure
- Change return types from `error` to `YAMLError`
- Update method signatures:
  - `Compile() YAMLError` instead of `Compile() error`
  - `Validate(value interface{}) YAMLError` instead of `Validate(value interface{}) error`

### 10.3 Preserve Error Information
- Avoid string conversions in error handling
- Use type assertions to access YAMLError methods
- Propagate structured error context through call stack

### 10.4 Error Conversion Strategy
- Implement proper error wrapping without losing type information
- Use `%w` directive for error wrapping
- Avoid error message stringification

---

## 11. Next Steps

1. **Decision Point**: Choose Option A (implement ValidatedSchema) or Option B (remove unused interfaces)

2. **If Option A**:
   - Implement Validate() YAMLError on SchemaDefinition
   - Add Name(), Description(), Version() methods
   - Update all callers to handle YAMLError returns

3. **If Option B**:
   - Remove ValidatedSchema interface
   - Remove SchemaValidationHandler interface  
   - Document Schema as primary interface
   - Fix return types to preserve YAMLError information

4. **Update Error Handling**:
   - Change Compile() return type to YAMLError
   - Change Validate() return type to YAMLError
   - Update all error handling code

5. **Add Tests**:
   - Test ValidatedSchema implementations (if Option A)
   - Test error type preservation
   - Test error conversion points

---

## 12. Related Files

- `internal/yamlutil/schema_interfaces.go` - Interface definitions
- `internal/yamlutil/schema.go` - Schema implementation
- `internal/yamlutil/errors.go` - YAMLError hierarchy
- `internal/yamlutil/validator.go` - YAML validation
- `internal/yamlutil/config.go` - Configuration types

---

## 13. Acceptance Criteria Status

- ✅ **List of all Validate() implementations found**:
  - SchemaDefinition.Validate(value interface{}) error
  - StringConstraintImpl.Validate(value interface{}) *ConstraintError
  - NumberConstraintImpl.Validate(value interface{}) *ConstraintError
  - ArrayConstraintImpl.Validate(value interface{}) *ConstraintError
  - ObjectConstraintImpl.Validate(value interface{}) *ConstraintError
  - BooleanConstraintImpl.Validate(value interface{}) *ConstraintError
  - TypeConstraintImpl.Validate(value interface{}) *ConstraintError

- ✅ **List of all Validate() call sites documented**:
  - SchemaValidator.Validate() → Schema.Validate() (line 180)
  - SchemaValidator.ValidateFile() → Validate() (line 243)

- ✅ **Current error patterns documented**:
  - YAMLError types created but returned as plain error
  - Double wrapping: YAMLError → error → string → SchemaValidationError
  - Type information loss at interface boundaries

- ✅ **Conversion points identified**:
  - Compile() method: 2 conversion points
  - Validate() method: 4 conversion points
  - SchemaValidator wrapper: 1 major conversion point

---

**Analysis Complete**
