# ARMOR Validate() Call Sites Catalog

Generated: 2026-07-12
Bead: bf-5wm9t
Updated: 2026-07-12

## Summary
Total Validate() occurrences found: **150+** (including definitions, implementations, call sites, and documentation)

## Search Method
- `rg -n "\.Validate\(" -C 2` - Method call patterns
- `rg -n "\bValidate\(" -C 2` - Direct function calls
- `rg -n "Validate," -C 2` - Type assertions/declarations
- `rg -n "\bValidate\b" -C 1` - All Validate occurrences

## Raw Output
Complete raw findings with context saved to: `/tmp/validate_callsites_raw.txt`

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

## 6. Additional Validation Categories Found

### YAML Validation (70+ call sites)
- `validator.ValidateString()` - 40+ occurrences
- `validator.ValidateFile()` - 20+ occurrences
- `validator.ValidateMultipleFiles()` - 5+ occurrences
- `validator.ValidateStringWithPath()` - 5+ occurrences

### Field Validation (15+ call sites)
- `ValidateRequiredFields()` - 10+ occurrences
- `ValidateFieldRequirements()` - 5+ occurrences

### Key Indentation Validation (15+ call sites)
- `ValidateMappingKeyIndent()` - 10+ occurrences
- `ValidateKeyIndentationSequence()` - 2+ occurrences
- `ValidateMappingKeyIndentLine()` - 2+ occurrences

### Syntax Validation (5+ call sites)
- `ValidateSyntax()` - 3+ occurrences
- `ValidateSyntaxInFile()` - 2+ occurrences

### Config/Field References (15+ occurrences)
- `ValidateTypes`, `ValidateRanges`, `ValidatePatterns`, `ValidateLengths` config fields
- `ValidateOnly` field in validation context
- `ValidateAfterParse` parser config field

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
| Interface Definitions | 20+ |
| Method Implementations | 20+ |
| Production Call Sites | 5+ |
| Test Call Sites | 70+ |
| Documentation/Comments | 20+ |
| Config Fields/Assertions | 15+ |
| **Total** | **150+** |

---

## Key Findings

1. **All Validate() calls are internal to yamlutil package** - No cross-package dependencies found
2. **Primary validation pattern**: SchemaValidator → SchemaDefinition.Validate()
3. **Constraint validation**: 6 constraint types with individual Validate() implementations
4. **Test coverage**: Comprehensive test coverage with 70+ call sites in test code
5. **No external API calls**: All Validate() calls are within the yamlutil subsystem
6. **Multiple validation types**: YAML syntax validation, schema validation, field validation, key indentation validation
7. **Heavily tested**: Test code accounts for ~70% of all Validate() call sites

---

## Files Containing Validate()

**Core Implementation:**
1. `internal/yamlutil/schema_interfaces.go` - Interface definitions and constraint implementations
2. `internal/yamlutil/schema.go` - Core SchemaValidator and SchemaDefinition implementations
3. `internal/yamlutil/validator.go` - YAML validation methods
4. `internal/yamlutil/syntax_validator.go` - Syntax validation
5. `internal/yamlutil/key_detection.go` - Key indentation validation
6. `internal/yamlutil/debug_helpers.go` - Field validation helpers
7. `internal/yamlutil/interfaces.go` - Field accessor interfaces
8. `internal/yamlutil/future.go` - Future/schema validation stubs
9. `internal/yamlutil/config.go` - Configuration structs

**Test Files:**
10. `internal/yamlutil/schema_validation_test.go` - Schema validation tests
11. `internal/yamlutil/validator_test.go` - Validator tests
12. `internal/yamlutil/key_indentation_validation_test.go` - Key indentation tests
13. `internal/yamlutil/debug_helpers_test.go` - Helper validation tests
14. `internal/yamlutil/interfaces_test.go` - Interface tests
15. `internal/yamlutil/integration_test.go` - Integration tests
16. `internal/yamlutil/empty_file_scenarios_test.go` - Empty file tests
17. `internal/yamlutil/invalid_yaml_fixed_test.go` - Invalid YAML tests
18. `internal/yamlutil/syntax_validator_test.go` - Syntax validator tests
19. `internal/yamlutil/config_test.go` - Config tests
20. `internal/yamlutil/examples_test.go` - Example tests
21. `internal/yamlutil/errors_test.go` - Error type tests
22. `internal/yamlutil/parse_error_examples_test.go` - Parse error examples

**Documentation:**
23. `internal/yamlutil/doc.go` - Usage examples in comments

