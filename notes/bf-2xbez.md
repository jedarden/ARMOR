# Validate() Implementation and Error Flow Analysis

**Bead ID**: bf-2xbez
**Date**: 2026-07-12
**Scope**: ARMOR yamlutil package

## Executive Summary

This analysis identifies all Validate() method implementations in the ARMOR codebase, documents their call sites, error return patterns, and error conversion points. The codebase has a well-structured error hierarchy but shows inconsistencies between interface definitions and concrete implementations.

## 1. Validate() Method Implementations

### 1.1 ValidatedSchema Interface (`schema_interfaces.go:31-44`)

```go
type ValidatedSchema interface {
    Validate() YAMLError
    Name() string
    Description() string
    Version() string
}
```

**Purpose**: Validates the schema definition itself (not data against the schema)
**Return Type**: `YAMLError`
**Implementations Found**: **ZERO** - No concrete implementations exist

### 1.2 Schema Interface (`schema.go:38-52`)

```go
type Schema interface {
    Validate(value interface{}) error
}
```

**Purpose**: Validates data against schema rules
**Return Type**: `error`
**Implementations**: `SchemaDefinition` struct

### 1.3 SchemaDefinition.Validate() (`schema.go:757-785`)

```go
func (s *SchemaDefinition) Validate(value interface{}) error
```

**Purpose**: Validates data against schema definition
**Return Type**: `error` (but returns YAMLError concrete types)

**Error Returns**:
- `NewValidationError()` - for nil values or validation failures
- `NewTypeMismatchError()` - for type mismatches  
- `NewFieldNotFoundError()` - for missing required fields
- `NewConstraintError()` - for constraint violations

### 1.4 SchemaDefinition.Compile() (`schema.go:732-748`)

```go
func (s *SchemaDefinition) Compile() error
```

**Purpose**: Validates schema definition itself
**Return Type**: `error` (but returns YAMLError concrete types)

**Error Returns**:
- `NewSchemaLoadError()` - for nil schema
- `NewValidationError()` - for invalid field definitions

### 1.5 SchemaValidator.Validate() (`schema.go:157-206`)

```go
func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult
```

**Purpose**: Validates data and returns comprehensive result
**Return Type**: `SchemaValidationResult` (struct containing error collections)

**Error Handling**: Converts errors to SchemaValidationError entries in result

### 1.6 SchemaValidator.ValidateFile() (`schema.go:212-244`)

```go
func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult
```

**Purpose**: Reads YAML file and validates its content
**Return Type**: `SchemaValidationResult`

### 1.7 Validator Methods (`validator.go`)

```go
func (v *Validator) ValidateString(yamlContent string) ValidationResult
func (v *Validator) ValidateStringWithPath(yamlContent, filePath string) ValidationResult
func (v *Validator) ValidateFile(filePath string) ValidationResult
func (v *Validator) ValidateMultipleFiles(filePaths []string) []ValidationResult
```

**Return Type**: `ValidationResult` or `[]ValidationResult`
**Purpose**: High-level YAML validation with detailed error reporting

### 1.8 DefaultSyntaxValidator Methods (`syntax_validator.go`)

```go
func (sv *DefaultSyntaxValidator) ValidateSyntax(yamlContent string) SyntaxValidationResult
func (sv *DefaultSyntaxValidator) ValidateSyntaxInFile(filePath string) SyntaxValidationResult
```

**Return Type**: `SyntaxValidationResult`
**Purpose**: Syntax-specific validation

## 2. Validate() Method Call Sites

### 2.1 SchemaValidator.Validate() (`schema.go:180`)

```go
if err := sv.schema.Validate(data); err != nil {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Validation failed: %v", err),
    })
    return result
}
```

**Pattern**: Error wrapped in result struct
**Conversion**: `error` → `SchemaValidationError` (lossy - message only)

### 2.2 SchemaValidator.compileSchema() (`schema.go:168-177`)

```go
if err := sv.compileSchema(); err != nil {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Invalid schema: %v", err),
    })
    return result
}
```

**Pattern**: Compile error wrapped in result struct
**Conversion**: `error` → `SchemaValidationError` (lossy - message only)

### 2.3 Test Files (`schema_validation_test.go`)

**Line 94**: `err := tt.schema.Validate()` - direct call with error type checking
```go
if !isYAMLError(err) {
    t.Errorf("%s: Validate() should return YAMLError-compatible error, got %T", tt.name, err)
}
```

**Line 147**: `err := schema.Validate()` - interface method testing
**Pattern**: Tests verify YAMLError interface implementation via `isYAMLError(err)` helper

### 2.4 Integration Tests (`integration_test.go`)

**Pattern**: High-level validation via `validator.ValidateFile()` and `validator.ValidateStringWithPath()`
**Flow**: Tests check ValidationResult.Valid and error arrays

### 2.5 LoadSchema() (`schema.go:627`)

```go
if err := schemaDef.Compile(); err != nil {
    return nil, &SchemaError{
        Message: fmt.Sprintf("Failed to compile schema: %v", err),
        FilePath: schemaPath,
    }
}
```

**Pattern**: Compile error wrapped in SchemaError
**Conversion**: `error` → `SchemaError` (lossy - message only)

## 3. Error Return Patterns

### 3.1 SchemaDefinition.Compile() Error Returns

```go
// Nil schema
return NewSchemaLoadError("", "schema is nil", nil, ErrCodeSchemaInvalid)

// Nil field definition
return NewValidationError("", fmt.Sprintf("field %s has nil definition", fieldName), fieldName, "", ErrCodeSchemaInvalid, 0, 0, ErrorTypeSchemaValidate, "")

// Invalid field type
return NewValidationError("", fmt.Sprintf("field %s has invalid type: %s", fieldName, fieldDef.Type), fieldName, "valid type", ErrCodeInvalidValue, 0, 0, ErrorTypeSchemaValidate, "")

// Min > max constraint
return NewValidationError("", fmt.Sprintf("field %s has min > max", fieldName), fieldName, "min <= max", ErrCodeConstraintViolation, 0, 0, ErrorTypeSchemaValidate, "")
```

**Pattern**: Direct YAMLError return
**Error Hierarchy**: All implement YAMLError interface with Code(), YAMLErrorType(), Context()
**Issue**: Method signature returns `error` (primitive type) not `YAMLError`

### 3.2 SchemaDefinition.Validate() Error Returns

```go
// Nil value check
return NewValidationError("", "value cannot be nil", "", "", ErrCodeValidationFailed, 0, 0, ErrorTypeValidation, "")

// Type conversion failure
return NewTypeMismatchError("", "", "map[string]interface{}", fmt.Sprintf("%T", value), "", 0, ErrCodeTypeMismatch)

// Missing required field
return NewFieldNotFoundError("", fieldName, 0, ErrCodeRequiredField)

// Field validation error
return NewTypeMismatchError("", fieldPath, fieldDef.Type, s.getTypeName(value), fmt.Sprintf("%v", value), 0, ErrCodeTypeMismatch)

// Constraint violations
return NewConstraintError("", fieldPath, "min", fmt.Sprintf("must be >= %d", *fieldDef.Min), fmt.Sprintf("value violates minimum constraint %d", *fieldDef.Min), fmt.Sprintf("%v", value), 0, ErrCodeConstraintViolation)

return NewConstraintError("", fieldPath, "max", fmt.Sprintf("must be <= %d", *fieldDef.Max), fmt.Sprintf("value violates maximum constraint %d", *fieldDef.Max), fmt.Sprintf("%v", value), 0, ErrCodeConstraintViolation)

return NewConstraintError("", fieldPath, "pattern", fieldDef.Pattern, fmt.Sprintf("value does not match pattern '%s'", fieldDef.Pattern), strVal, 0, ErrCodeConstraintViolation)

return NewConstraintError("", fieldPath, "enum", fmt.Sprintf("must be one of: %v", fieldDef.AllowedValues), fmt.Sprintf("value not in allowed list"), fmt.Sprintf("%v", value), 0, ErrCodeInvalidValue)
```

**Pattern**: Specific error types for different failure modes
**Type Safety**: All errors implement YAMLError interface
**Issue**: Method signature returns `error` (primitive type) not `YAMLError`

### 3.3 SchemaValidator Error Conversion Pattern

```go
func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult {
    result := SchemaValidationResult{
        Valid: true,
        Errors: []SchemaValidationError{},
        // ...
    }
    
    // Compile schema error → SchemaValidationError
    if err := sv.compileSchema(); err != nil {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Invalid schema: %v", err),
        })
        return result
    }
    
    // Validation error → SchemaValidationError
    if err := sv.schema.Validate(data); err != nil {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Validation failed: %v", err),
        })
        return result
    }
    
    return result
}
```

**Conversion 1**: `error` → `SchemaValidationError.Message` (string conversion)
**Conversion 2**: Error info lost in conversion (only message preserved)

## 4. Error Conversion Points

### 4.1 Type Assertion: error → YAMLError

**Location**: `schema_validation_test.go:104-107`, `errors.go:1211-1219`

```go
func IsYAMLError(err error) bool {
    if err == nil {
        return false
    }
    _, ok := err.(YAMLError)
    return ok
}
```

**Purpose**: Runtime type checking for interface compliance
**Usage**: Test validation, error type switching

### 4.2 Error Wrapping: error → SchemaValidationError

**Location**: `schema.go:180-184`

```go
if err := sv.schema.Validate(data); err != nil {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Validation failed: %v", err),
    })
    return result
}
```

**Lossy Conversion**: Only error message preserved
**Lost Information**:
- Error code (ErrCodeRequiredField, ErrCodeTypeMismatch, etc.)
- Error type (ErrorTypeValidation, ErrorTypeConstraint, etc.)
- Field path details
- Constraint details
- Context information

### 4.3 Error Unwrapping for Type Detection

**Location**: `errors.go:1228-1232`

```go
func GetYAMLErrorType(err error) ErrorType {
    for unwrapped := errors.Unwrap(err); unwrapped != nil; unwrapped = errors.Unwrap(unwrapped) {
        if ye, ok := unwrapped.(YAMLError); ok {
            return ye.YAMLErrorType()
        }
    }
    return ""
}
```

**Purpose**: Extract error type from wrapped error chains
**Pattern**: Iterative unwrapping to find YAMLError

### 4.4 LocalValidationError → ValidationError Conversion

**Location**: `interfaces.go:48-59`

```go
func (ve LocalValidationError) ToValidationError() ValidationError {
    return ValidationError{
        FilePath:   ve.FilePath,
        Message:    ve.Message,
        ContextStr: ve.Context,
        Line:       ve.Line,
        Column:     ve.Column,
        Type:       ve.Type,
        Path:       "",
    }
}
```

**Purpose**: Convert internal validation error to standard error type
**Pattern**: Direct field mapping with path initialization

## 5. Error Hierarchy

**Location**: `errors.go:27-42`

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

**Interface Methods**:
- `Code() ErrorCode` - Error code for programmatic handling
- `YAMLErrorType() ErrorType` - Error category for type switching
- `Context() string` - Additional context about the error
- `Error() string` - Human-readable error message

## 6. Key Findings

### 6.1 Interface Mismatch

- **ValidatedSchema.Validate()** returns `YAMLError`
- **Schema.Validate()** returns `error`
- **SchemaDefinition** implements `Schema` interface, returning `error` (not `YAMLError`)
- **ValidatedSchema** has **ZERO** implementations

### 6.2 Error Information Loss

The SchemaValidator.Validate() method converts YAMLError to SchemaValidationError, preserving only the message string:

```go
result.Errors = append(result.Errors, SchemaValidationError{
    Message: fmt.Sprintf("Validation failed: %v", err),
})
```

**Lost Information**:
- Error code (ErrCodeRequiredField, ErrCodeTypeMismatch, etc.)
- Error type (ErrorTypeValidation, ErrorTypeConstraint, etc.)
- Field path details
- Constraint details
- Context information

### 6.3 Type Safety Through Interface

All error types implement YAMLError interface, enabling:
- Runtime type checking via `IsYAMLError(err)`
- Type switching via `err.(YAMLError)`
- Error code-based handling

### 6.4 Conversion Patterns

1. **Upcast**: YAMLError → error (implicit, always safe)
2. **Type Assertion**: error → YAMLError (requires `IsYAMLError()` check)
3. **Lossy Conversion**: YAMLError → SchemaValidationError (message only)
4. **Unwrapping**: error chain → YAMLError (via `errors.Unwrap()`)

### 6.5 Method Semantics

- **ValidatedSchema.Validate()**: Validates the **schema definition itself**
- **Schema.Validate()**: Validates **data against the schema**
- **SchemaDefinition.Compile()**: Validates the schema definition (similar to ValidatedSchema.Validate())

## 7. Constraint Implementations

**Location**: `schema_interfaces.go:315-900`

| Type | Method | Returns |
|------|--------|---------|
| `StringConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |
| `NumberConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |
| `ArrayConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |
| `ObjectConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |
| `BooleanConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |
| `TypeConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |

**Key Difference**: Constraints return `*ConstraintError` (concrete type) not wrapped in interface.

## 8. Identified Issues

### Issue 1: Interface-Implementation Mismatch
**Severity**: High

`ValidatedSchema` interface defines `Validate() YAMLError` but `SchemaDefinition` has:
- `Compile() error` - different method name
- Different return type (`error` vs `YAMLError`)
- No implementation of ValidatedSchema interface

### Issue 2: Error Type Loss
**Severity**: Medium

When SchemaValidator wraps errors in SchemaValidationResult, YAMLError details are lost.

### Issue 3: Inconsistent Return Types
**Severity**: Medium

`Compile()` and `Validate()` return YAMLError instances but typed as `error`:
- Prevents compile-time type checking
- Requires runtime type assertions
- Inconsistent with YAMLError-first design intent

### Issue 4: Unused Interface
**Severity**: Low

`ValidatedSchema` interface has zero implementations - may be incomplete design or future work.

## 9. Recommendations

### Recommendation 1: Align SchemaDefinition with ValidatedSchema

Either:
A. Add `Validate() YAMLError` to SchemaDefinition that validates the schema itself
B. Rename ValidatedSchema to clarify its purpose (e.g., `SelfValidatingSchema`)

### Recommendation 2: Fix Return Type Signatures

Change method signatures to return YAMLError directly:

```go
// Before
func (s *SchemaDefinition) Compile() error
func (s *SchemaDefinition) Validate(value interface{}) error

// After
func (s *SchemaDefinition) Compile() YAMLError
func (s *SchemaDefinition) Validate(value interface{}) YAMLError
```

### Recommendation 3: Preserve YAMLError Details

When wrapping errors, preserve full YAMLError information:

```go
if err := sv.schema.Validate(data); err != nil {
    if yamlErr, ok := err.(YAMLError); ok {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   yamlErr.Error(),
            ErrorCode: yamlErr.Code(),
            ErrorType: yamlErr.YAMLErrorType(),
            FieldPath: yamlErr.Context(),
        })
    } else {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Validation failed: %v", err),
        })
    }
}
```

### Recommendation 4: Type Assertion Helpers

Add helper functions for safe YAMLError handling:

```go
func MustBeYAMLError(err error) YAMLError {
    if ye, ok := err.(YAMLError); ok {
        return ye
    }
    panic(fmt.Sprintf("error is not YAMLError: %T", err))
}
```

## 10. Summary Statistics

| Category | Count |
|----------|-------|
| ValidatedSchema implementations | 0 |
| Schema interface implementations | 1 (SchemaDefinition) |
| Validate() methods found | 20+ (different types) |
| YAMLError types defined | 11 |
| Error constructors | 8 |
| Direct YAMLError returns (constraints) | 6 |
| Wrapped error returns (schemas) | 2 |
| Error conversion points identified | 4 |

## 11. Files Analyzed

| File | Lines | Purpose |
|------|-------|---------|
| `internal/yamlutil/schema_interfaces.go` | 964 | ValidatedSchema interface, constraints |
| `internal/yamlutil/errors.go` | 1280+ | Error hierarchy definitions |
| `internal/yamlutil/schema.go` | 1122 | SchemaDefinition, SchemaValidator |
| `internal/yamlutil/validator.go` | 400+ | Validator implementations |
| `internal/yamlutil/syntax_validator.go` | 500+ | Syntax validation |
| `internal/yamlutil/schema_validation_test.go` | 460 | Schema validation tests |

---

**Analysis Completed**: 2026-07-12
**Next Steps**: Implement ValidatedSchema interface or reconcile with existing SchemaDefinition
