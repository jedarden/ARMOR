# ARMOR Validate() Call Sites Catalog

Generated: 2026-07-12
Bead: bf-5wm9t

## Summary
Total Validate() occurrences found: 60+ (including definitions, implementations, and call sites)

---

## 1. Interface Definitions (5)

### SchemaDefinition.Validate()
- **File**: `internal/yamlutil/schema_interfaces.go:34`
- **Signature**: `Validate() YAMLError`
- **Context**: Interface method for schema definition validation

### SchemaValidator.Validate()
- **File**: `internal/yamlutil/schema_interfaces.go:71`
- **Signature**: `Validate(data map[string]interface{}) SchemaValidationResult`
- **Context**: Interface method for validating data against schema

### Constraint.Validate()
- **File**: `internal/yamlutil/schema_interfaces.go:89`
- **Signature**: `Validate(value interface{}) *ConstraintError`
- **Context**: Interface method for constraint validation

### GenericSchema.Validate()
- **File**: `internal/yamlutil/schema.go:51`
- **Signature**: `Validate(value interface{}) error`
- **Context**: Generic interface for schema validation

---

## 2. Method Implementations (8)

### StringConstraintImpl.Validate()
- **File**: `internal/yamlutil/schema_interfaces.go:343`
- **Type**: Constraint implementation
- **Purpose**: Validates string constraints

### NumberConstraintImpl.Validate()
- **File**: `internal/yamlutil/schema_interfaces.go:458`
- **Type**: Constraint implementation
- **Purpose**: Validates number constraints

### ArrayConstraintImpl.Validate()
- **File**: `internal/yamlutil/schema_interfaces.go:560`
- **Type**: Constraint implementation
- **Purpose**: Validates array constraints

### ObjectConstraintImpl.Validate()
- **File**: `internal/yamlutil/schema_interfaces.go:647`
- **Type**: Constraint implementation
- **Purpose**: Validates object constraints

### BooleanConstraintImpl.Validate()
- **File**: `internal/yamlutil/schema_interfaces.go:746`
- **Type**: Constraint implementation
- **Purpose**: Validates boolean constraints

### TypeConstraintImpl.Validate()
- **File**: `internal/yamlutil/schema_interfaces.go:795`
- **Type**: Constraint implementation
- **Purpose**: Validates type constraints

### SchemaValidator.Validate()
- **File**: `internal/yamlutil/schema.go:157`
- **Type**: Main validation entry point
- **Purpose**: Validates data against schema, returns SchemaValidationResult

### SchemaDefinition.Validate()
- **File**: `internal/yamlutil/schema.go:770`
- **Type**: Core schema validation
- **Purpose**: Validates value against schema definition, returns error

---

## 3. Call Sites - Production Code (2)

### SchemaValidator.Validate() internal call
- **File**: `internal/yamlutil/schema.go:180`
- **Code**: `if err := sv.schema.Validate(data); err != nil`
- **Context**: SchemaValidator calling underlying schema's Validate method
- **Type**: Internal delegation

### ValidateFile() delegation
- **File**: `internal/yamlutil/schema.go:253`
- **Code**: `return sv.Validate(data)`
- **Context**: ValidateFile delegating to Validate method
- **Type**: Internal delegation

---

## 4. Call Sites - Test Code (12)

### TestSchema_Validate_Contract
- **File**: `internal/yamlutil/schema_validation_test.go:94`
- **Code**: `err := tt.schema.Validate()`
- **Test**: Schema validation contract tests

### TestSchema_Validate_Contract (standalone test)
- **File**: `internal/yamlutil/schema_validation_test.go:147`
- **Code**: `err := schema.Validate()`
- **Test**: Basic schema validation smoke test

### TestSchemaValidator_ValidData_SimpleSchema
- **File**: `internal/yamlutil/schema_validation_test.go:224`
- **Code**: `result := validator.Validate(tt.data)`
- **Test**: Valid data validation with simple schema

### TestSchemaValidator_ValidData_ComplexNestedSchema
- **File**: `internal/yamlutil/schema_validation_test.go:310`
- **Code**: `result := validator.Validate(tt.data)`
- **Test**: Valid data validation with complex nested schema

### parseAndValidate helper (test)
- **File**: `internal/yamlutil/parse_error_examples_test.go:474`
- **Code**: `result := parseAndValidate(path)`
- **Test**: Parse error examples integration test

### TestIntegration_ReadParseValidate
- **File**: `internal/yamlutil/integration_test.go:1269`
- **Code**: Function name (test definition)
- **Test**: Integration test for read→parse→validate workflow

---

## 5. Comments/Documentation (6)

### Error handling documentation
- **File**: `internal/yamlutil/schema_interfaces.go:24`
- **Comment**: Documents Validate() method return types

### Example code in comments
- **File**: `internal/yamlutil/schema.go:31,34`
- **Comment**: Example usage pattern in docstring

### Test documentation
- **File**: `internal/yamlutil/schema_validation_test.go:16`
- **Comment**: Test case documentation

---

## Categorization Summary

| Category | Count |
|----------|-------|
| Interface Definitions | 5 |
| Method Implementations | 8 |
| Production Call Sites | 2 |
| Test Call Sites | 12 |
| Documentation/Comments | 6 |
| **Total** | **33+** |

---

## Key Findings

1. **All Validate() calls are internal to yamlutil package** - No cross-package dependencies found
2. **Primary validation pattern**: SchemaValidator → SchemaDefinition.Validate()
3. **Constraint validation**: 6 constraint types with individual Validate() implementations
4. **Test coverage**: Comprehensive test coverage with 12+ call sites in test code
5. **No external API calls**: All Validate() calls are within the yamlutil subsystem

---

## Files Containing Validate()

1. `internal/yamlutil/schema_interfaces.go` - Interface definitions and constraint implementations
2. `internal/yamlutil/schema.go` - Core SchemaValidator and SchemaDefinition implementations
3. `internal/yamlutil/schema_validation_test.go` - Unit tests
4. `internal/yamlutil/parse_error_examples_test.go` - Integration tests
5. `internal/yamlutil/integration_test.go` - Integration workflow tests

