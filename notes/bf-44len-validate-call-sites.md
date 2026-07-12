# Validate() Call Sites Analysis

## Overview
This document catalogs all `Validate()` method implementations and call sites in the ARMOR codebase.

## Validate() Method Implementations

### 1. Schema Interface (schema.go:38-51)
```go
type Schema interface {
    Validate(value interface{}) error
}
```
**Purpose:** Generic interface for validating values against schema rules.

---

### 2. SchemaDefinition.Validate() (schema.go:757)
```go
func (s *SchemaDefinition) Validate(value interface{}) error
```
**Type:** SchemaDefinition struct  
**Purpose:** Validates data against schema definition rules.  
**Returns:** YAMLError types (SchemaLoadError, SchemaValidationError, ValidationError)

---

### 3. SchemaValidator.Validate() (schema.go:157)
```go
func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult
```
**Type:** SchemaValidator struct  
**Purpose:** Validates YAML data against schema with comprehensive error reporting.  
**Returns:** SchemaValidationResult with all errors and warnings

---

### 4. SchemaValidator.ValidateFile() (schema.go:212)
```go
func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult
```
**Type:** SchemaValidator struct  
**Purpose:** Validates a YAML file against the schema.  
**Returns:** SchemaValidationResult with detailed error information

---

## Validate() Call Sites

### Production Code (Non-Test)

#### 1. schema.go:180 - SchemaValidator calling Schema.Validate()
```go
if err := sv.schema.Validate(data); err != nil {
```
**Calling Context:** SchemaValidator.Validate() method  
**Validated Type:** Schema interface (stored in sv.schema field)  
**Validated Data:** Generic interface{} data  
**Purpose:** Delegates validation to the underlying schema implementation

---

#### 2. schema.go:243 - SchemaValidator.ValidateFile calling Validate()
```go
return sv.Validate(data)
```
**Calling Context:** SchemaValidator.ValidateFile() method  
**Validated Type:** Calls SchemaValidator.Validate() method on same instance  
**Validated Data:** map[string]interface{} parsed from YAML file  
**Purpose:** Reuses validation logic after file parsing

---

### Test Code

#### 3. schema_validation_test.go:94 - Schema interface contract test
```go
err := tt.schema.Validate()
```
**Calling Context:** TestSchema_Validate_Contract test function  
**Validated Type:** Schema interface (Schema struct)  
**Validated Data:** Schema definition itself (schema self-validation)  
**Purpose:** Tests that Schema implementations properly validate their own definitions

---

#### 4. schema_validation_test.go:147 - ValidatedSchema interface test
```go
err := schema.Validate()
```
**Calling Context:** TestValidatedSchema_Interface test function  
**Validated Type:** ValidatedSchema interface  
**Validated Data:** Schema definition itself  
**Purpose:** Tests ValidatedSchema interface compliance

---

#### 5. schema_validation_test.go:224 - SchemaValidator generic value validation
```go
result := validator.Validate(tt.data)
```
**Calling Context:** TestSchema_Validate_GenericValues test function  
**Validated Type:** SchemaValidator struct  
**Validated Data:** Generic interface{} test data  
**Purpose:** Tests SchemaValidator with various data types

---

#### 6. schema_validation_test.go:310 - SchemaValidator constraint validation
```go
result := validator.Validate(tt.data)
```
**Calling Context:** TestSchema_Validate_Constraints test function  
**Validated Type:** SchemaValidator struct  
**Validated Data:** Data with constraint violations  
**Purpose:** Tests constraint validation behavior

---

## Related Validate Methods (Not Direct Calls)

These are Validate methods that don't call `.Validate()` on another object but are part of the validation API:

### Validator.ValidateString() (validator.go:109)
```go
func (v *Validator) ValidateString(yamlContent string) ValidationResult
```
**Type:** Validator struct  
**Purpose:** Validates YAML content from a string

---

### Validator.ValidateStringWithPath() (validator.go:114)
```go
func (v *Validator) ValidateStringWithPath(yamlContent, filePath string) ValidationResult
```
**Type:** Validator struct  
**Purpose:** Validates YAML content with file path context for error reporting

---

### Validator.ValidateFile() (validator.go:152)
```go
func (v *Validator) ValidateFile(filePath string) ValidationResult
```
**Type:** Validator struct  
**Purpose:** Validates a YAML file (syntax and structure only, not schema-based)

---

### Validator.ValidateMultipleFiles() (validator.go:312)
```go
func (v *Validator) ValidateMultipleFiles(filePaths []string) []ValidationResult
```
**Type:** Validator struct  
**Purpose:** Validates multiple YAML files

---

## Summary

### Total Validate() Call Sites: 6
- **Production Code:** 2 call sites
- **Test Code:** 4 call sites

### Key Implementation Types
1. **Schema interface** - Generic validation interface
2. **SchemaDefinition** - Concrete schema implementation
3. **SchemaValidator** - Comprehensive validator with detailed error reporting
4. **ValidatedSchema** - Interface for schema self-validation

### Validation Patterns
1. **Schema Self-Validation:** `schema.Validate()` - validates schema definition itself
2. **Data Validation:** `schema.Validate(data)` - validates data against schema
3. **File Validation:** `validator.ValidateFile(path)` - validates YAML files
4. **Comprehensive Validation:** `schemaValidator.Validate(data)` - returns detailed validation results

### Error Handling
All Validate() methods in production code return structured error types:
- **YAMLError hierarchy** for ValidatedSchema interface methods
- **SchemaValidationResult** for SchemaValidator methods
- **error** for Schema interface methods (which can be YAMLError types)
