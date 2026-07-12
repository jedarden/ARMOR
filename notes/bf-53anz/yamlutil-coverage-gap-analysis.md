# YAMLUtil Coverage Gap Analysis

## Overview

This analysis identifies coverage gaps in the yamlutil package based on the coverage report from `/home/coding/ARMOR/notes/bf-66x3j/coverage.html`.

## Files Below 80% Coverage

| File | Coverage | Priority |
|------|----------|----------|
| schema.go | 0.0% | HIGH |
| schema_interfaces.go | 0.0% | HIGH |
| template.go | 0.0% | MEDIUM |
| interfaces.go | 21.8% | HIGH |
| result_types.go | 26.0% | MEDIUM |
| errors.go | 66.5% | MEDIUM |
| validator.go | 75.0% | LOW |

## Detailed Analysis by File

### 1. schema.go (0.0% coverage) - HIGH PRIORITY

**Exported Functions:**
- `NewSchemaValidator(schema *Schema) *SchemaValidator`
- `NewSchemaValidatorWithConfig(schema *Schema, config *ValidatorConfig) *SchemaValidator`
- `LoadSchema(schemaPath string) (*Schema, error)`

**Key Methods (on SchemaValidator):**
- `Validate(data map[string]interface{}) SchemaValidationResult`
- `ValidateFile(filePath string) SchemaValidationResult`

**Uncovered Error Cases:**
1. **Missing or invalid schema files**
   - Non-existent schema file paths
   - Files with unsupported extensions (not .json, .yaml, .yml)
   - Malformed JSON/YAML in schema files
   - Empty schema files

2. **Invalid schema definitions**
   - Nil schema passed to NewSchemaValidator
   - Schema with nil field definitions
   - Invalid field types (not in valid types list)
   - Min constraint > Max constraint

3. **Validation failures**
   - Missing required fields in data
   - Type mismatches (expected vs actual)
   - Constraint violations (min, max, pattern, allowed values)
   - Nested schema validation failures
   - Array item validation failures
   - Unknown fields in strict mode

4. **Schema compilation errors**
   - Invalid regex patterns
   - Circular schema references
   - Invalid nested schema definitions

**Required Test Cases:**
```go
// Test schema loading
TestLoadSchema_ValidJSONSchema
TestLoadSchema_ValidYAMLSchema
TestLoadSchema_InvalidExtension
TestLoadSchema_NonExistentFile
TestLoadSchema_MalformedJSON
TestLoadSchema_MalformedYAML
TestLoadSchema_EmptyFile

// Test schema validator creation
TestNewSchemaValidator_NilSchema
TestNewSchemaValidator_InvalidFieldDefinition
TestNewSchemaValidator_InvalidFieldType
TestNewSchemaValidator_MinGreaterThanMax

// Test validation
TestSchemaValidator_Validate_MissingRequiredFields
TestSchemaValidator_Validate_TypeMismatch
TestSchemaValidator_Validate_MinConstraintViolation
TestSchemaValidator_Validate_MaxConstraintViolation
TestSchemaValidator_Validate_PatternViolation
TestSchemaValidator_Validate_AllowedValuesViolation
TestSchemaValidator_Validate_NestedSchemaFailure
TestSchemaValidator_Validate_ArrayItemFailure
TestSchemaValidator_Validate_UnknownFieldsStrictMode
TestSchemaValidator_Validate_ValidDataPasses

// Test file validation
TestSchemaValidator_ValidateFile_FileReadError
TestSchemaValidator_ValidateFile_YAMLParseError
TestSchemaValidator_ValidateFile_ValidYAML
```

### 2. schema_interfaces.go (0.0% coverage) - HIGH PRIORITY

**Exported Functions:**
- `NewStringConstraint(...)` - Creates string validation constraint
- `NewNumberConstraint(...)` - Creates number validation constraint
- `NewArrayConstraint(...)` - Creates array validation constraint
- `NewObjectConstraint(...)` - Creates object validation constraint
- `NewBooleanConstraint(...)` - Creates boolean validation constraint
- `NewTypeConstraint(...)` - Creates type validation constraint
- `GetTypeName(value interface{}) string` - Returns type name of value

**Uncovered Error Cases:**
1. **String constraint validation**
   - Value is not a string (type error)
   - String length < minLength
   - String length > maxLength
   - String doesn't match pattern
   - String not in allowed values
   - Invalid regex pattern in constructor

2. **Number constraint validation**
   - Value is not a number (type error)
   - Number < minimum
   - Number <= minimum (exclusive)
   - Number > maximum
   - Number >= maximum (exclusive)
   - Number not a multiple of specified value
   - Conversion to float64 fails

3. **Array constraint validation**
   - Value is not an array (type error)
   - Array length < minItems
   - Array length > maxItems
   - Duplicate items when uniqueItems=true

4. **Object constraint validation**
   - Value is not an object (type error)
   - Property count < minProperties
   - Property count > maxProperties
   - Missing required fields
   - Disallowed properties present

5. **Boolean constraint validation**
   - Value is not boolean (type error)
   - Value not in allowed boolean values

6. **Type constraint validation**
   - Null value when not nullable
   - Type mismatch for expected type
   - Custom checker returns false
   - Coercion failure

7. **GetTypeName edge cases**
   - Nil value
   - Unknown/custom types
   - Pointer types

**Required Test Cases:**
```go
// String constraint tests
TestStringConstraint_ValidString
TestStringConstraint_NonStringValue
TestStringConstraint_MinLengthViolation
TestStringConstraint_MaxLengthViolation
TestStringConstraint_PatternViolation
TestStringConstraint_NotInAllowedValues
TestStringConstraint_InvalidRegexPattern

// Number constraint tests
TestNumberConstraint_ValidNumber
TestNumberConstraint_NonNumericValue
TestNumberConstraint_MinViolation
TestNumberConstraint_ExclusiveMinViolation
TestNumberConstraint_MaxViolation
TestNumberConstraint_ExclusiveMaxViolation
TestNumberConstraint_NotMultipleOf

// Array constraint tests
TestArrayConstraint_ValidArray
TestArrayConstraint_NonArrayValue
TestArrayConstraint_MinItemsViolation
TestArrayConstraint_MaxItemsViolation
TestArrayConstraint_DuplicateItems

// Object constraint tests
TestObjectConstraint_ValidObject
TestObjectConstraint_NonObjectValue
TestObjectConstraint_MinPropertiesViolation
TestObjectConstraint_MaxPropertiesViolation
TestObjectConstraint_MissingRequiredField
TestObjectConstraint_DisallowedProperty

// Boolean constraint tests
TestBooleanConstraint_ValidBoolean
TestBooleanConstraint_NonBooleanValue
TestBooleanConstraint_NotInAllowedValues

// Type constraint tests
TestTypeConstraint_ValidType
TestTypeConstraint_NullWhenNotNullable
TestTypeConstraint_TypeMismatch
TestTypeConstraint_CustomCheckerFalse
TestTypeConstraint_CoercionFailure

// GetTypeName tests
TestGetTypeName_NilValue
TestGetTypeName_AllBasicTypes
TestGetTypeName_CustomType
```

### 3. template.go (0.0% coverage) - MEDIUM PRIORITY

**Exported Functions:**
- `NewTemplateProcessor() *TemplateProcessor`
- `ProcessTemplate(template string, variables map[string]string) (string, error)` - method
- `ProcessTemplateFile(templatePath string, variables map[string]string) (string, error)` - method

**Status:** The file contains stub implementations that return "not yet implemented" errors.

**Uncovered Error Cases:**
1. **Template processing errors** (when implemented)
   - Undefined variables in strict mode
   - Invalid variable syntax
   - Nested/recursive variable expansion
   - Escape character handling
   - File read errors for template files

**Required Test Cases:**
```go
// Note: These tests should fail with "not yet implemented" until feature is complete
TestTemplateProcessor_ProcessTemplate_NotImplemented
TestTemplateProcessor_ProcessTemplateFile_NotImplemented
TestTemplateProcessor_NewProcessor_CreatesDefaultConfig
```

### 4. interfaces.go (21.8% coverage) - HIGH PRIORITY

**Key Interfaces (currently have minimal coverage):**
- `FileReader` interface methods
- `YAMLParser` interface implementations
- `YAMLValidator` interface methods
- `FieldAccessor` interface methods
- `YAMLCache` interface methods
- `YAMLWatcher` interface methods
- `YAMLConverter` interface methods
- `YAMLPathNavigator` interface methods

**Uncovered Error Cases:**
1. **FileReader interface implementations**
   - File read permission errors
   - Directory passed as file path
   - Symbolic link handling
   - Large file handling

2. **YAMLParser interface implementations**
   - CachedParser: Cache eviction behavior
   - CachedParser: TTL expiration
   - CachedParser: Cache statistics accuracy
   - StreamingParser: File size limit enforcement
   - StreamingParser: Memory limits

3. **FieldAccessor interface methods**
   - Invalid path syntax (empty, malformed)
   - Non-existent intermediate fields in nested paths
   - Type coercion failures
   - Nil values in nested structures

4. **YAMLCache interface methods**
   - Cache invalidation race conditions
   - Cache size limits
   - Concurrent access

5. **YAMLWatcher interface methods**
   - File watcher initialization failures
   - Watch on non-existent file
   - Multiple watches on same file
   - Watcher cleanup on close

6. **YAMLConverter interface methods**
   - Invalid data structures for conversion
   - Circular references in data
   - Type conversion failures

7. **YAMLPathNavigator interface methods**
   - Invalid path expressions
   - Array index out of bounds
   - Wildcard pattern errors
   - Path syntax errors

**Required Test Cases:**
```go
// FileReader interface tests
TestDefaultFileReader_Read_PermissionDenied
TestDefaultFileReader_Read_DirectoryPath
TestDefaultFileReader_Read_Symlink
TestDefaultFileReader_Exists_NonExistent

// CachedParser tests
TestCachedParser_ParseFile_CacheHit
TestCachedParser_ParseFile_CacheMiss
TestCachedParser_CacheEviction_LRU
TestCachedParser_TTLExpiration
TestCachedParser_CacheStats
TestCachedParser_ConcurrentAccess

// StreamingParser tests
TestStreamingParser_FileSizeLimit
TestStreamingParser_MemoryEfficiency

// FieldAccessor interface tests
TestFieldAccessor_EmptyPath
TestFieldAccessor_MalformedPath
TestFieldAccessor_NonExistentIntermediateField
TestFieldAccessor_NilValueInPath

// YAMLCache interface tests
TestYAMLCache_InvalidationRace
TestYAMLCache_SizeLimit
TestYAMLCache_ConcurrentAccess

// YAMLWatcher interface tests
TestYAMLWatcher_NonExistentFile
TestYAMLWatcher_DuplicateWatches
TestYAMLWatcher_Cleanup

// YAMLConverter interface tests
TestYAMLConverter_InvalidStructure
TestYAMLConverter_CircularReferences
TestYAMLConverter_TypeConversionFailure

// YAMLPathNavigator interface tests
TestYAMLPathNavigator_InvalidExpression
TestYAMLPathNavigator_ArrayIndexOutOfBounds
TestYAMLPathNavigator_WildcardErrors
```

### 5. result_types.go (26.0% coverage) - MEDIUM PRIORITY

**Exported Types/Methods:**
- `SuccessParseResult[T]` methods (String, ToLegacy, etc.)
- `ValidationResult` methods (ErrorSummary, WarningSummary, FullSummary)
- `SchemaValidationResult` methods (ErrorSummary)
- `ProcessingResult` methods
- `FieldAccessResult` methods
- `BatchValidationResult` methods

**Uncovered Error Cases:**
1. **SuccessParseResult methods**
   - String method with nil metadata
   - ToLegacy conversion with complex types
   - Raw field operations when Raw is nil

2. **ValidationResult methods**
   - ErrorSummary with multiple errors
   - WarningSummary with Unicode characters
   - FullSummary edge cases (nil schema version, zero duration)

3. **SchemaValidationResult methods**
   - ErrorSummary with mixed error types
   - Empty error lists
   - Nil constraint violations

4. **ProcessingResult methods**
   - Summary with nil ValidationResult
   - StageResults with various data types

5. **FieldAccessResult methods**
   - String method with complex values
   - IsSuccess with error but exists=true

6. **BatchValidationResult methods**
   - SuccessRate with zero total files
   - Summary with no failed files
   - GetResultsByStatus with all valid/invalid

**Required Test Cases:**
```go
// SuccessParseResult tests
TestSuccessParseResult_String_NilMetadata
TestSuccessParseResult_ToLegacy_ComplexTypes
TestSuccessParseResult_RawFieldOperations

// ValidationResult tests
TestValidationResult_ErrorSummary_MultipleErrors
TestValidationResult_WarningSummary_Unicode
TestValidationResult_FullSummary_EdgeCases

// SchemaValidationResult tests
TestSchemaValidationResult_ErrorSummary_MixedTypes
TestSchemaValidationResult_ErrorSummary_EmptyLists
TestSchemaValidationResult_ErrorSummary_NilViolations

// ProcessingResult tests
TestProcessingResult_Summary_NilValidation
TestProcessingResult_StageResults_Variants

// FieldAccessResult tests
TestFieldAccessResult_String_ComplexValues
TestFieldAccessResult_IsSuccess_ErrorWithExists

// BatchValidationResult tests
TestBatchValidationResult_SuccessRate_ZeroTotal
TestBatchValidationResult_Summary_NoFailures
TestBatchValidationResult_GetResultsByStatus_AllValid
TestBatchValidationResult_GetResultsByStatus_AllInvalid
```

### 6. errors.go (66.5% coverage) - MEDIUM PRIORITY

**Uncovered Error Cases:**
1. **Error variant edge cases**
   - Unrecognized error codes
   - Empty error messages
   - Nil error wrapping

2. **Error method edge cases**
   - String() with nil context
   - Context() with empty strings
   - Code() with undefined codes

3. **Error type detection**
   - Ambiguous error scenarios
   - Multiple error conditions

**Required Test Cases:**
```go
TestParseError_EmptyMessage
TestParseError_NilContext
TestParseError_UnrecognizedCode
TestSyntaxError_UnrecognizedVariant
TestValidationError_MultipleErrors
TestYAMLError_Wrapping
TestErrorContext_EmptyStrings
```

### 7. validator.go (75.0% coverage) - LOW PRIORITY

**Uncovered Error Cases:**
1. **Validator edge cases**
   - Empty YAML content
   - YAML with only comments
   - YAML with document separators (---, ...)
   - Mixed line endings (CRLF vs LF)

2. **Strict mode differences**
   - Unknown fields rejection
   - Duplicate key detection
   - Anchor/alias validation

**Required Test Cases:**
```go
TestValidator_EmptyContent
TestValidator_OnlyComments
TestValidator_DocumentSeparators
TestValidator_MixedLineEndings
TestStrictValidator_UnknownFields
TestStrictValidator_DuplicateKeys
TestStrictValidator_AnchorsAndAliases
```

## Missing Files Error Cases Summary

The following error case categories are completely uncovered across the low-coverage files:

### File I/O Errors
- Missing files (schema.go, validator.go)
- Permission denied (interfaces.go)
- Directory instead of file (interfaces.go)
- Symbolic link issues (interfaces.go)

### Parse Errors
- Invalid YAML syntax (schema.go, validator.go)
- Invalid JSON syntax (schema.go)
- Empty files (schema.go, validator.go)
- Unsupported file extensions (schema.go)

### Type Errors
- Type mismatches (schema.go, schema_interfaces.go)
- Null/non-nullable conflicts (schema_interfaces.go)
- Invalid type definitions (schema.go)
- Coercion failures (schema_interfaces.go)

### Constraint Violations
- Min/max violations (schema.go, schema_interfaces.go)
- Pattern matching failures (schema.go, schema_interfaces.go)
- Allowed value violations (schema.go, schema_interfaces.go)
- Length violations (schema_interfaces.go)
- Required field violations (schema.go)

### Schema/Definition Errors
- Nil schemas (schema.go)
- Invalid field definitions (schema.go)
- Circular references (schema.go)
- Invalid regex patterns (schema_interfaces.go)
- Min > max constraints (schema.go)

### Validation Edge Cases
- Empty data structures (validator.go)
- Comment-only files (validator.go)
- Unicode handling (result_types.go)
- Document separators (validator.go)
- Mixed line endings (validator.go)

## Priority Implementation Order

1. **Phase 1: HIGH** - Critical functionality
   - schema.go (LoadSchema, Validate, ValidateFile)
   - interfaces.go (FileReader, YAMLParser implementations)
   - schema_interfaces.go (constraint implementations)

2. **Phase 2: MEDIUM** - Important functionality
   - result_types.go (method edge cases)
   - errors.go (error edge cases)
   - template.go (stub validation)

3. **Phase 3: LOW** - Nice to have
   - validator.go (edge cases)

## Test Strategy Recommendations

1. **Table-driven tests** for constraint validation (schema_interfaces.go)
   - Test all constraint types with various inputs
   - Include edge cases and boundary conditions

2. **Error wrapping tests** for error handling paths
   - Test that errors are properly wrapped with context
   - Verify error codes and types

3. **Integration tests** for schema validation
   - Test complete schema loading and validation workflows
   - Include complex nested structures

4. **Mock tests** for interface implementations
   - Mock file system for FileReader tests
   - Mock time for cache TTL tests

5. **Property-based tests** for result type methods
   - Test String() methods with various inputs
   - Test summary methods with different error counts
