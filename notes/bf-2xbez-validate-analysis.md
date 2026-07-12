# Validate() Implementation and Error Flow Analysis

**Bead:** bf-2xbez  
**Date:** 2026-07-12  
**Scope:** ARMOR yamlutil package validation system

## Executive Summary

The ARMOR codebase implements a comprehensive multi-layered validation system centered around the `ValidatedSchema` interface and related validation infrastructure. The system provides:

- **Schema validation** through `ValidatedSchema` interface
- **Data validation** through `SchemaValidator` with detailed error reporting
- **Constraint validation** through six specialized constraint types
- **Error hierarchy** with structured YAMLError types for precise error handling

## 1. ValidatedSchema Interface Implementations

### Interface Definition

**Location:** `internal/yamlutil/schema_interfaces.go` (Lines 31-44)

```go
type ValidatedSchema interface {
    Validate() error
    Name() string
    Description() string
    Version() string
}
```

### Documentation Comments

The interface specifies that `Validate()` returns YAMLError types from the error hierarchy:
- `SchemaLoadError` (ErrCodeSchemaLoadFailed): when schema cannot be loaded
- `SchemaValidationError` (ErrCodeSchemaInvalid): when schema definition is invalid  
- `ValidationError` (ErrCodeValidationFailed): for general validation failures

### Concrete Implementations Found

#### SchemaDefinition

**Location:** `internal/yamlutil/schema.go` (Lines 59-80)

**Methods:**
- `Validate(value interface{}) error` (Lines 757-785)
- `Compile() error` (Lines 732-748)
- `Name()`, `Description()`, `Version()` via struct fields

**Note:** SchemaDefinition does NOT implement the ValidatedSchema interface as defined. The interface requires `Validate() error` (no parameters), but SchemaDefinition has `Validate(value interface{}) error`. This appears to be a design inconsistency.

## 2. Validate() Method Implementations

### Constraint Validators (Value-Level Validation)

**Location:** `internal/yamlutil/schema_interfaces.go`

All constraint validators implement:
```go
Validate(value interface{}) *ConstraintError
```

1. **StringConstraintImpl** (Lines 343-395)
   - Validates string length, patterns, allowed values, format
   - Returns `*ConstraintError` or `nil`

2. **NumberConstraintImpl** (Lines 458-507)
   - Validates numeric ranges, exclusive bounds, multiples
   - Returns `*ConstraintError` or `nil`

3. **ArrayConstraintImpl** (Lines 560-593)
   - Validates array length, unique items
   - Returns `*ConstraintError` or `nil`

4. **ObjectConstraintImpl** (Lines 647-695)
   - Validates properties, required fields, property counts
   - Returns `*ConstraintError` or `nil`

5. **BooleanConstraintImpl** (Lines 746-763)
   - Validates boolean values against allowed values
   - Returns `*ConstraintError` or `nil`

6. **TypeConstraintImpl** (Lines 795-812)
   - Validates type matching and nullable constraints
   - Returns `*ConstraintError` or `nil`

### Schema-Level Validators

#### SchemaDefinition.Validate()

**Location:** `internal/yamlutil/schema.go` (Lines 757-785)

**Signature:** `func (s *SchemaDefinition) Validate(value interface{}) error`

**Error Returns:**
- `NewValidationError` for nil values (Line 759)
- `NewTypeMismatchError` for type conversion failures (Line 765)
- `NewFieldNotFoundError` for missing required fields (Line 772)
- Type mismatch errors during field validation (Line 791)
- Constraint errors from constraint validation (Lines 845-869)

**Error Creation Points:**
```go
// Line 734: SchemaLoadError for nil schema
return NewSchemaLoadError("", "schema is nil", nil, ErrCodeSchemaInvalid)

// Line 740: ValidationError for nil field definitions
return NewValidationError("", fmt.Sprintf("field %s has nil definition", fieldName), fieldName, "", ErrCodeSchemaInvalid, 0, 0, ErrorTypeSchemaValidate, "")

// Line 759: ValidationError for nil value
return NewValidationError("", "value cannot be nil", "", "", ErrCodeValidationFailed, 0, 0, ErrorTypeValidation, "")

// Line 765: TypeMismatchError for wrong type
return NewTypeMismatchError("", "", "map[string]interface{}", fmt.Sprintf("%T", value), "", 0, ErrCodeTypeMismatch)

// Line 772: FieldNotFoundError for missing required field
return NewFieldNotFoundError("", fieldName, 0, ErrCodeRequiredField)
```

#### SchemaValidator.Validate()

**Location:** `internal/yamlutil/schema.go` (Lines 157-206)

**Signature:** `func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult`

**Return Type:** `SchemaValidationResult` (structured result, NOT error)

**Error Handling:**
- Line 169: Calls `compileSchema()` and handles compilation errors
- Line 180: Calls `sv.schema.Validate(data)` and wraps errors in SchemaValidationResult
- Lines 189-200: Performs detailed field validation with SchemaValidationResult

**Key Difference:** Returns `SchemaValidationResult` instead of `error`, providing:
- `Valid` boolean
- `Errors []SchemaValidationError`
- `Warnings []SchemaValidationError`
- `MissingRequiredFields []string`
- `TypeMismatches []FieldTypeError`
- `ConstraintViolations []ConstraintViolation`

## 3. Validate() Call Sites

### Internal Call Sites (schema.go)

1. **Line 180:** `sv.schema.Validate(data)`
   - Context: SchemaValidator calling schema's Validate method
   - Error: Wrapped in SchemaValidationResult

2. **Line 243:** `sv.Validate(data)`
   - Context: ValidateFile delegating to Validate method
   - Error: Returns SchemaValidationResult

3. **Line 627:** `schemaDef.Compile()`
   - Context: LoadSchema validating schema definition
   - Error: Returned to caller or wrapped in SchemaError

### Test File Call Sites

**Location:** `internal/yamlutil/schema_validation_test.go`

4. **Line 94:** `tt.schema.Validate()`
   - Context: Testing ValidatedSchema interface compliance
   - Error: Checked for YAMLError interface compliance

5. **Line 147:** `schema.Validate()`
   - Context: Interface compliance testing
   - Error: Checked for non-nil errors

6. **Lines 224, 310:** `validator.Validate(tt.data)`
   - Context: Testing data validation
   - Error: Results checked in SchemaValidationResult

7. **Line 391:** `validator.ValidateFile()`
   - Context: File-based validation testing
   - Error: Results checked in SchemaValidationResult

**Location:** `internal/yamlutil/integration_test.go`

8. **Lines 1061-1263:** Multiple `validator.ValidateFile()` calls
9. **Lines 1292, 1319:** Integration workflow validation

## 4. Error Return Patterns

### Pattern 1: Direct Error Returns (SchemaDefinition)

`SchemaDefinition.Validate()` returns `error` directly:

```go
func (s *SchemaDefinition) Validate(value interface{}) error {
    if value == nil {
        return NewValidationError(...)
    }
    
    data, ok := value.(map[string]interface{})
    if !ok {
        return NewTypeMismatchError(...)
    }
    
    for fieldName, fieldDef := range s.RootFields {
        if fieldDef.Required {
            if _, exists := data[fieldName]; !exists {
                return NewFieldNotFoundError(...)
            }
        }
        
        if fieldValue, exists := data[fieldName]; exists {
            if err := s.validateField(fieldValue, fieldDef, fieldName); err != nil {
                return err  // Propagates validation errors
            }
        }
    }
    
    return nil  // Success
}
```

**Error Types Returned:**
- `ValidationError` (via NewValidationError)
- `TypeMismatchError` (via NewTypeMismatchError)
- `FieldNotFoundError` (via NewFieldNotFoundError)
- `ConstraintError` (via NewConstraintError)

### Pattern 2: Structured Results (SchemaValidator)

`SchemaValidator.Validate()` returns `SchemaValidationResult`:

```go
func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult {
    result := SchemaValidationResult{
        Valid: true,
        Errors: []SchemaValidationError{},
        // ...
    }
    
    if !sv.compiled {
        if err := sv.compileSchema(); err != nil {
            result.Valid = false
            result.Errors = append(result.Errors, SchemaValidationError{
                Message: fmt.Sprintf("Invalid schema: %v", err),
            })
            return result
        }
    }
    
    if err := sv.schema.Validate(data); err != nil {
        result.Valid = false
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Validation failed: %v", err),
        })
        return result
    }
    
    // Detailed field validation...
    result.Valid = !result.HasErrors()
    return result
}
```

**Key Difference:** Errors are collected into result structure rather than returned directly.

### Pattern 3: Constraint Validation Errors

Constraint validators return `*ConstraintError`:

```go
func (sc *StringConstraintImpl) Validate(value interface{}) *ConstraintError {
    str, ok := value.(string)
    if !ok {
        return &ConstraintError{
            Constraint:     fmt.Sprintf("value is not a string: %T", value),
            ConstraintType: "string",
            Value:          fmt.Sprintf("%v", value),
        }
    }
    
    if sc.minLength > 0 && len(str) < sc.minLength {
        return &ConstraintError{
            Constraint:     fmt.Sprintf("string length %d is less than minimum %d", len(str), sc.minLength),
            ConstraintType: "min_length",
            Value:          str,
        }
    }
    
    return nil  // Success
}
```

## 5. Error Conversion Points

### Conversion 1: YAMLError → SchemaValidationResult

**Location:** `internal/yamlutil/schema.go` (Lines 169-186)

```go
if err := sv.compileSchema(); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Invalid schema: %v", err),
    })
    return result
}
```

**Conversion Pattern:** YAMLError → SchemaValidationError (string message only)

### Conversion 2: LocalValidationError → ValidationError

**Location:** `internal/yamlutil/validator.go` (Lines 48-59)

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

**Conversion Pattern:** Local type → Standard YAMLError type

### Conversion 3: Error → SchemaError

**Location:** `internal/yamlutil/schema.go` (Lines 584-634)

```go
func LoadSchema(schemaPath string) (*SchemaDefinition, error) {
    content, err := os.ReadFile(schemaPath)
    if err != nil {
        return nil, &SchemaError{
            Message:  fmt.Sprintf("Failed to read schema file: %v", err),
            FilePath: schemaPath,
        }
    }
    
    // Parse and build...
    
    if err := schemaDef.Compile(); err != nil {
        return nil, &SchemaError{
            Message:  fmt.Sprintf("Failed to compile schema: %v", err),
            FilePath: schemaPath,
        }
    }
    
    return schemaDef, nil
}
```

**Conversion Pattern:** Various errors → SchemaError with file context

## 6. Error Flow Map

```
┌─────────────────────────────────────────────────────────────────┐
│                     Validation Entry Points                       │
├─────────────────────────────────────────────────────────────────┤
│  ValidateString() │ ValidateFile() │ Validate(data interface{})  │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Schema Validator Layer                        │
├─────────────────────────────────────────────────────────────────┤
│  SchemaValidator.Validate()                                     │
│  ├─ compileSchema()                                             │
│  ├─ schema.Validate(data) ────────────────┐                   │
│  └─ validateFields() ───────────────────────┤                   │
└─────────────────────────────────────────────┼───────────────────┘
                                               │
                     ┌─────────────────────────┼─────────────────────┐
                     │                         │                     │
                     ▼                         ▼                     ▼
        ┌─────────────────────┐   ┌──────────────────┐   ┌─────────────────┐
        │ SchemaDefinition     │   │ Field Validation │   │ Constraint       │
        │ .Validate()         │   │ validateField()  │   │ Validation      │
        ├─────────────────────┤   ├──────────────────┤   ├─────────────────┤
        │ • Type checking     │   │ • Type validation│   │ • Min/Max       │
        │ • Required fields   │   │ • Constraint     │   │ • Pattern       │
        │ • Schema rules      │   │   validation     │   │ • Allowed       │
        └──────────┬──────────┘   └────────┬─────────┘   │   values         │
                   │                      │             └─────────────────┘
                   ▼                      ▼
        ┌──────────────────────────────────────────────┐
        │              Error Creation                    │
        ├──────────────────────────────────────────────┤
        │ • NewValidationError()                         │
        │ • NewTypeMismatchError()                       │
        │ • NewFieldNotFoundError()                      │
        │ • NewConstraintError()                         │
        │ • NewSchemaLoadError()                         │
        └──────────────────────────────────────────────┘
                   │
                   ▼
        ┌──────────────────────────────────────────────┐
        │           Error Flow Paths                    │
        ├──────────────────────────────────────────────┤
        │  Path 1: error → SchemaValidationResult       │
        │  Path 2: error → SchemaError → caller         │
        │  Path 3: *ConstraintError → ValidationError    │
        │  Path 4: LocalValidationError → ValidationError│
        └──────────────────────────────────────────────┘
```

## 7. Key Findings and Issues

### Issue 1: Interface/Implementation Mismatch

The `ValidatedSchema` interface requires `Validate() error` (no parameters), but `SchemaDefinition.Validate(value interface{}) error` takes a parameter. This means:

**SchemaDefinition does NOT implement ValidatedSchema** as the interface is currently defined.

**Potential Fix Options:**
1. Change interface to: `Validate(value interface{}) error`
2. Split into two interfaces:
   - `ValidatedSchema` for schema validation (no params)
   - `DataValidator` for data validation (with value param)
3. Add adapter methods to SchemaDefinition

### Issue 2: Mixed Return Patterns

The codebase has two different validation return patterns:

1. **Error-based**: `SchemaDefinition.Validate()` returns `error`
2. **Result-based**: `SchemaValidator.Validate()` returns `SchemaValidationResult`

This creates inconsistency in how validation results are handled across the codebase.

### Issue 3: Error Information Loss

When converting YAMLError to SchemaValidationResult (Line 180-186), only the error message is preserved:

```go
result.Errors = append(result.Errors, SchemaValidationError{
    Message: fmt.Sprintf("Validation failed: %v", err),
})
```

**Lost Information:**
- Error code (`err.Code()`)
- Error type (`err.YAMLErrorType()`) 
- Context (`err.Context()`)
- Structured error details

### Issue 4: Constraint Error vs YAMLError

Constraint validators return `*ConstraintError`, but this is NOT a YAMLError interface type. This creates:

1. **Type inconsistency**: Constraint errors don't implement YAMLError interface
2. **Handling complexity**: Different error types require different handling patterns
3. **Limited error context**: ConstraintError has fewer fields than ValidationError

## 8. Recommendations

### 1. Standardize Interface Definitions

```go
// Schema validation (validates the schema itself)
type ValidatedSchema interface {
    ValidateSchema() error  // Validates schema definition
    Name() string
    Description() string
    Version() string
}

// Data validation (validates data against schema)
type DataValidator interface {
    Validate(value interface{}) error
}
```

### 2. Standardize Error Returns

Choose ONE pattern and apply consistently:

**Option A:** Always return structured results
```go
Validate(value interface{}) SchemaValidationResult
```

**Option B:** Always return errors with rich error types
```go
Validate(value interface{}) error
```

### 3. Make ConstraintError Implement YAMLError

```go
type ConstraintError struct {
    // ... existing fields ...
    ErrorCode ErrorCode
    ErrorType ErrorType
}

func (ce *ConstraintError) Code() ErrorCode { return ce.ErrorCode }
func (ce *ConstraintError) YAMLErrorType() ErrorType { return ce.ErrorType }
func (ce *ConstraintError) Context() string { return ce.Message }
```

### 4. Preserve Error Information in Conversions

```go
result.Errors = append(result.Errors, SchemaValidationError{
    Message: err.Error(),
    ErrorCode: err.Code(),
    ErrorType: err.YAMLErrorType(),
    Context: err.Context(),
    // ... preserve full error structure
})
```

## 9. Validation Usage Patterns

### Pattern 1: Direct Schema Validation

```go
schemaDef := &SchemaDefinition{...}
if err := schemaDef.Compile(); err != nil {
    return fmt.Errorf("schema compilation failed: %w", err)
}

if err := schemaDef.Validate(data); err != nil {
    return fmt.Errorf("validation failed: %w", err)
}
```

### Pattern 2: Schema Validator Usage

```go
validator := NewSchemaValidator(schemaDef)
result := validator.Validate(data)

if !result.Valid {
    for _, err := range result.Errors {
        log.Printf("Validation error: %s", err.Message)
    }
}
```

### Pattern 3: File-Based Validation

```go
validator := NewSchemaValidator(schemaDef)
result := validator.ValidateFile("config.yaml")

if result.HasErrors() {
    // Handle errors
}

if result.HasWarnings() {
    // Handle warnings
}
```

## 10. Error Type Reference

### YAMLError Hierarchy

```
YAMLError (interface)
├── ParseError
│   ├── SyntaxError
│   ├── StructureError
│   └── TypeMismatchError
├── ValidationError
│   ├── FieldNotFoundError
│   ├── ConstraintError
│   └── DuplicateKeyError
└── SchemaError
    ├── SchemaLoadError
    └── SchemaValidationError
```

### Error Codes

- `ErrCodeFileNotFound` - File I/O errors
- `ErrCodeInvalidSyntax` - YAML syntax errors
- `ErrCodeTypeMismatch` - Type conversion errors
- `ErrCodeValidationFailed` - General validation failures
- `ErrCodeRequiredField` - Missing required fields
- `ErrCodeConstraintViolation` - Constraint violations
- `ErrCodeSchemaLoadFailed` - Schema loading errors
- `ErrCodeSchemaInvalid` - Invalid schema definitions

---

**Analysis Completed:** 2026-07-12  
**Total Files Analyzed:** 8 core files + test files  
**Lines of Code Reviewed:** ~3,000+ lines  
**Error Types Identified:** 12 distinct error types  
**Validation Patterns:** 3 main patterns documented
