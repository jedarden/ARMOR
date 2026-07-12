# Validate() Call Sites in ARMOR Codebase

## Overview
This document identifies all locations in the ARMOR codebase that call `Validate()` methods, including the validated type, calling function, and context.

## Summary
Total Validate() call sites found: **8**

---

## Call Sites by File

### 1. internal/yamlutil/schema.go

#### Call Site 1: Documentation Example (Line 34)
- **Location**: `internal/yamlutil/schema.go:34`
- **Type**: Documentation comment (not executable code)
- **Calling Context**: Schema interface documentation
- **Validated Type**: Generic `interface{}` value
- **Code**:
  ```go
  //	err := schema.Validate(data)
  ```
- **Purpose**: Example usage in godoc comment showing how to call the Schema interface's Validate method

#### Call Site 2: SchemaValidator.Validate() Method (Line 180)
- **Location**: `internal/yamlutil/schema.go:180`
- **Calling Function**: `SchemaValidator.Validate(data interface{}) SchemaValidationResult`
- **Validated Type**: `interface{}` - the input data parameter
- **Code**:
  ```go
  if err := sv.schema.Validate(data); err != nil {
      result.Valid = false
      result.Errors = append(result.Errors, SchemaValidationError{
          Message: fmt.Sprintf("Validation failed: %v", err),
      })
      return result
  }
  ```
- **Purpose**: Validates the input data against the schema defined in `sv.schema`. If validation fails, records the error in the result and returns early.
- **Context**: This is the main validation entry point for schema-based validation of YAML/JSON data

#### Call Site 3: SchemaValidator.ValidateFile() Method (Line 243)
- **Location**: `internal/yamlutil/schema.go:243`
- **Calling Function**: `SchemaValidator.ValidateFile(filePath string) SchemaValidationResult`
- **Validated Type**: `interface{}` - parsed YAML data from file
- **Code**:
  ```go
  // Validate against schema
  return sv.Validate(data)
  ```
- **Purpose**: After reading and parsing a YAML file, delegates to `Validate()` to validate the parsed data
- **Context**: File-based validation that chains into the main Validate() method
- **Flow**: Read file → Parse YAML → Call Validate() → Return result

---

### 2. internal/yamlutil/schema_validation_test.go

#### Call Site 4: TestSchema_Validate_Contract (Line 94)
- **Location**: `internal/yamlutil/schema_validation_test.go:94`
- **Calling Function**: `TestSchema_Validate_Contract` - test function
- **Validated Type**: `*Schema` - schema definition being validated for correctness
- **Code**:
  ```go
  err := tt.schema.Validate()
  ```
- **Purpose**: Tests that Schema implementations properly validate themselves (schema definition validation, not data validation)
- **Context**: Contract verification test ensuring Schema interface is correctly implemented
- **Test Coverage**: Tests both valid schemas and invalid schemas (nil, nil field definition, invalid field type, min > max constraints)

#### Call Site 5: TestSchemaDefinition_Interface (Line 147)
- **Location**: `internal/yamlutil/schema_validation_test.go:147`
- **Calling Function**: `TestSchemaDefinition_Interface` - test function
- **Validated Type**: `*Schema` - a properly formed test schema
- **Code**:
  ```go
  err := schema.Validate()
  if err != nil {
      t.Errorf("Schema.Validate() unexpected error: %v", err)
  }
  ```
- **Purpose**: Verifies that a well-formed schema validates successfully
- **Context**: Interface validation test confirming Schema implements SchemaDefinition interface correctly

#### Call Site 6: TestSchema_Validate_GenericValues (Line 224)
- **Location**: `internal/yamlutil/schema_validation_test.go:224`
- **Calling Function**: `TestSchema_Validate_GenericValues` - test function
- **Validated Type**: `map[string]interface{}` - test data containing various primitive types
- **Code**:
  ```go
  validator := NewSchemaValidator(schema)
  result := validator.Validate(tt.data)
  ```
- **Purpose**: Tests validation of generic values (strings, integers, booleans) against schema type definitions
- **Context**: Data validation test ensuring the validator correctly checks type compliance
- **Test Coverage**:
  - Valid data with all types
  - Missing required fields
  - Integer out of range
  - Wrong type for field

#### Call Site 7: TestSchema_Validate_NestedStructures (Line 310)
- **Location**: `internal/yamlutil/schema_validation_test.go:310`
- **Calling Function**: `TestSchema_Validate_NestedStructures` - test function
- **Validated Type**: `map[string]interface{}` - test data with nested objects and arrays
- **Code**:
  ```go
  validator := NewSchemaValidator(schema)
  result := validator.Validate(tt.data)
  ```
- **Purpose**: Tests validation of nested object structures and array items
- **Context**: Nested validation test ensuring recursive schema validation works correctly
- **Test Coverage**:
  - Valid nested structures
  - Missing required nested fields
  - Array item constraint violations

---

## Validation Types and Patterns

### 1. Schema Self-Validation
- **Pattern**: `schema.Validate()` (no arguments)
- **Purpose**: Validates the schema definition itself for correctness
- **Found in**: Test code only (schema_validation_test.go)
- **Validates**: Schema structure, field definitions, constraint validity

### 2. Data Validation
- **Pattern**: `schema.Validate(data)` or `validator.Validate(data)`
- **Purpose**: Validates data against a schema definition
- **Found in**: Production code (schema.go) and test code
- **Validates**: Data structure, field types, constraint compliance

### 3. Interface-Based Validation
- **Pattern**: `sv.schema.Validate(data)` where schema is Schema interface
- **Purpose**: Polymorphic validation through interface
- **Found in**: SchemaValidator.Validate()
- **Validates**: Any value against the encapsulated schema

---

## Key Implementation Types

### Schema Interface
```go
type Schema interface {
    Validate(value interface{}) error
}
```

### SchemaDefinition (implements Schema)
```go
type SchemaDefinition struct {
    Type       SchemaType
    Name       string
    RootFields map[string]*FieldDefinition
    // ... other fields
}

func (s *SchemaDefinition) Validate(value interface{}) error
```

### SchemaValidator
```go
type SchemaValidator struct {
    schema Schema
    config *ValidatorConfig
}

func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult
func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult
```

---

## Call Graph

```
SchemaValidator.ValidateFile() [schema.go:212]
    └─> SchemaValidator.Validate() [schema.go:243]
        └─> sv.schema.Validate() [schema.go:180]
            └─> SchemaDefinition.Validate() [schema.go:757]

Test Functions [schema_validation_test.go]
    ├─> TestSchema_Validate_Contract [line 94]
    │   └─> tt.schema.Validate()
    ├─> TestSchemaDefinition_Interface [line 147]
    │   └─> schema.Validate()
    ├─> TestSchema_Validate_GenericValues [line 224]
    │   └─> validator.Validate(tt.data)
    └─> TestSchema_Validate_NestedStructures [line 310]
        └─> validator.Validate(tt.data)
```

---

## Related Code

### Schema Definition Validation
- **File**: `internal/yamlutil/schema.go`
- **Method**: `SchemaDefinition.Validate(value interface{}) error` [line 757]
- **Validates**: Data against schema (type checking, required fields, constraints)

### Schema Compile-Time Validation
- **File**: `internal/yamlutil/schema.go`
- **Method**: `SchemaDefinition.Compile() error` [line 732]
- **Validates**: Schema definition correctness (before use)

### Field Validation
- **File**: `internal/yamlutil/schema.go`
- **Method**: `SchemaDefinition.validateField()` [line 788]
- **Validates**: Individual field values against field definitions

---

## Notes

1. **All Validate() calls are in Go code** - No Rust, JavaScript, or TypeScript implementations found
2. **No external Validate() callers** - All calls are within the yamlutil package or its tests
3. **Two validation modes**:
   - Schema self-validation (test code only)
   - Data validation against schema (production code)
4. **Polymorphic design** - Schema interface allows different schema implementations to be used interchangeably
5. **Recursive validation** - Nested structures and arrays are validated recursively through the same Validate() interface

---

## Generated Information

- **Search Date**: 2026-07-12
- **Search Scope**: /home/coding/ARMOR
- **File Types Searched**: *.rs, *.go, *.js, *.ts
- **Total Files Scanned**: All Go files in ARMOR codebase
- **Total Call Sites**: 8
