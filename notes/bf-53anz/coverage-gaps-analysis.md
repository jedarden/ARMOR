# yamlutil Coverage Gaps Analysis

## Executive Summary

Based on coverage report analysis, the yamlutil package has **7 files below 80% coverage** and **3 files with 0% coverage**. This document identifies specific gaps and required test cases.

---

## Coverage Summary by File

| File | Coverage | Status | Priority |
|------|----------|--------|----------|
| config.go | 100.0% | ✅ Above threshold | - |
| debug_helpers.go | 87.9% | ✅ Above threshold | - |
| **errors.go** | **66.5%** | ❌ Below threshold | **HIGH** |
| file.go | 83.3% | ✅ Above threshold | - |
| **interfaces.go** | **21.8%** | ❌ Below threshold | **HIGH** |
| parse_error_design.go | 94.6% | ✅ Above threshold | - |
| parse_result.go | 98.9% | ✅ Above threshold | - |
| parser.go | 89.7% | ✅ Above threshold | - |
| result.go | 82.8% | ✅ Above threshold | - |
| **result_types.go** | **26.0%** | ❌ Below threshold | **HIGH** |
| **schema.go** | **0.0%** | ❌ No coverage | **CRITICAL** |
| **schema_interfaces.go** | **0.0%** | ❌ No coverage | **CRITICAL** |
| **template.go** | **0.0%** | ❌ No coverage | **MEDIUM** |
| **validator.go** | **75.0%** | ❌ Below threshold | **HIGH** |

---

## 1. schema.go (0.0% Coverage) - CRITICAL PRIORITY

### Missing Functions
- `NewSchemaValidator()` - constructor
- `NewSchemaValidatorWithConfig()` - constructor with custom config
- `Validate()` - schema validation entry point
- `ValidateFile()` - file-based validation
- `validateFields()` - field validation logic
- `validateField()` - single field validation
- `validateType()` - type checking
- `validateConstraints()` - constraint validation
- `checkMinConstraint()` - minimum value validation
- `checkMaxConstraint()` - maximum value validation
- `isAllowedValue()` - enum validation
- `valuesEqual()` - value comparison
- `getTypeName()` - type detection
- `joinPath()` - path construction
- `LoadSchema()` - schema loading from file
- `buildSchemaFromData()` - schema construction
- `buildFieldDefinition()` - field definition builder
- `Schema.Validate()` - schema self-validation
- `validateFieldDefinition()` - field definition validation

### Required Error Case Tests
1. **Missing Files**
   - Non-existent schema file → `SchemaError` with `ErrCodeSchemaNotFound`
   - Schema file without read permissions → `SchemaError` with `ErrCodeFileAccessDenied`

2. **Invalid YAML**
   - Malformed YAML schema → `SchemaError` with parse failure
   - Empty schema file → `SchemaError` with empty content message
   - Invalid JSON/YAML structure → `SchemaError` with structural error

3. **Type Errors**
   - Type mismatch: expected string, got integer → `FieldTypeError` in result
   - Type mismatch: expected object, got scalar → `FieldTypeError` in result
   - Type mismatch: expected array, got object → `FieldTypeError` in result
   - Unknown/invalid type in schema → returns error from `Schema.Validate()`

4. **Constraint Violations**
   - Value below minimum → `ConstraintViolation` with Min constraint
   - Value above maximum → `ConstraintViolation` with Max constraint
   - String shorter than min length → `ConstraintViolation` for length
   - String longer than max length → `ConstraintViolation` for length
   - Pattern mismatch (regex) → `ConstraintViolation` for pattern
   - Value not in allowed enum → `ConstraintViolation` for enum

5. **Schema Definition Errors**
   - Min > Max constraint → error from `Schema.Validate()`
   - Invalid type name → error from `Schema.Validate()`
   - Nil field definition → error from `Schema.Validate()`
   - Unsupported file format → `SchemaError` with format error

6. **Nested Structures**
   - Missing required nested field → `MissingRequiredFields` list
   - Invalid nested object type → `FieldTypeError` with nested path
   - Nested array item validation → `FieldTypeError` with array index
   - Deep nesting path construction → verify dot notation path

### Specific Test Cases Needed

#### Test Case: Schema-001 - Load Valid Schema
```yaml
# schema.yaml
name: TestSchema
type: json
properties:
  port:
    type: integer
    minimum: 1
    maximum: 65535
  host:
    type: string
    pattern: "^[a-z]+\\.com$"
required: ["port"]
```
**Expected:** `Schema` loaded successfully, RootFields contains "port" and "host"

#### Test Case: Schema-002 - Validate Against Schema
```yaml
# config.yaml
port: 8080
host: example.com
```
**Expected:** `SchemaValidationResult.Valid = true`, empty errors list

#### Test Case: Schema-003 - Missing Required Field
```yaml
# config.yaml
host: example.com
```
**Expected:** `Valid = false`, `MissingRequiredFields = ["port"]`

#### Test Case: Schema-004 - Type Mismatch
```yaml
# config.yaml
port: "not-a-number"
host: example.com
```
**Expected:** `TypeMismatches` contains error for "port" (expected integer, got string)

#### Test Case: Schema-005 - Constraint Violation (Min)
```yaml
# config.yaml
port: 0
host: example.com
```
**Expected:** `ConstraintViolations` contains error for "port" below minimum 1

#### Test Case: Schema-006 - Constraint Violation (Max)
```yaml
# config.yaml
port: 70000
host: example.com
```
**Expected:** `ConstraintViolations` contains error for "port" above maximum 65535

#### Test Case: Schema-007 - Pattern Mismatch
```yaml
# config.yaml
port: 8080
host: INVALID-HOST
```
**Expected:** `ConstraintViolations` contains pattern violation error for "host"

#### Test Case: Schema-008 - Non-Existent Schema File
**Input:** LoadSchema("/nonexistent/schema.yaml")
**Expected:** `SchemaError` with FilePath set, ErrCode = `ErrCodeSchemaNotFound`

#### Test Case: Schema-009 - Invalid Schema YAML
```yaml
# invalid.yaml
port: : broken : yaml
```
**Expected:** `SchemaError` with parse failure message

#### Test Case: Schema-010 - Empty Schema File
**Input:** LoadSchema("empty.yaml") where file is empty
**Expected:** `SchemaError` with empty/invalid schema message

#### Test Case: Schema-011 - Schema Validation Error (Invalid Type)
```yaml
# schema with invalid type
properties:
  field:
    type: invalid_type_name
```
**Expected:** Error from `Schema.Validate()`: "field field has invalid type: invalid_type_name"

#### Test Case: Schema-012 - Schema Validation Error (Min > Max)
```yaml
# schema with min > max
properties:
  field:
    type: integer
    minimum: 100
    maximum: 50
```
**Expected:** Error from `Schema.Validate()`: "field field has min > max"

#### Test Case: Schema-013 - Nested Object Validation
```yaml
# schema
properties:
  config:
    type: object
    nested:
      properties:
        enabled:
          type: boolean
```
**Input:** Config with nested enabled = "true" (string not bool)
**Expected:** `FieldTypeError` at path "config.enabled"

#### Test Case: Schema-014 - Array Item Validation
```yaml
# schema
properties:
  tags:
    type: array
    items:
      type: string
```
**Input:** tags: [1, 2, 3] (numbers not strings)
**Expected:** `FieldTypeError` for each array item

---

## 2. schema_interfaces.go (0.0% Coverage) - CRITICAL PRIORITY

**Note:** This file likely contains interface definitions. Interfaces don't require unit tests, but:
- Verify this file contains only interface definitions
- If it contains struct implementations, those need tests
- Document the interfaces and their intended usage patterns

---

## 3. template.go (0.0% Coverage) - MEDIUM PRIORITY

**Action Required:**
1. Read template.go to understand what functionality it provides
2. If it contains template processing logic:
   - Test template variable substitution
   - Test template syntax errors
   - Test missing template variables
   - Test invalid template expressions
3. If it's only template definitions, document usage patterns

---

## 4. interfaces.go (21.8% Coverage) - HIGH PRIORITY

### Missing Implementation Coverage

#### defaultFileReader (lines 365-381)
- `Read()` method - delegation to ReadFile()
- `Exists()` method - delegation to FileExists()

#### defaultFieldAccessor (lines 367-368, 383-436)
- `GetField()` - delegation test
- `GetString()` - delegation test
- `GetInt()` - delegation test
- `GetBool()` - delegation test
- `HasField()` - delegation test
- `GetRequiredField()` - delegation test
- `GetRequiredString()` - delegation test
- `GetRequiredInt()` - delegation test
- `GetRequiredBool()` - delegation test
- `ValidateRequiredFields()` - delegation test
- `ValidateFieldRequirements()` - delegation test

#### defaultFileDiscovery (lines 370-371, 438-451)
- `FindYAMLFiles()` - delegation test
- `FindYAMLFilesRecursive()` - delegation test
- `IsYAMLFile()` - delegation test

#### StandardParser Implementation (lines 457-511)
- `NewStandardParser()` - constructor
- `NewStandardParserWithConfig()` - constructor with config
- `ParseFile()` - delegation to underlying Parser
- `ParseFileToMap()` - delegation test
- `ParseString()` - delegation test
- `MustParseFile()` - delegation test (including panic case)
- `Config()` - returns config

#### CachedParser Implementation (lines 513-708)
- `NewCachedParser()` - constructor
- `NewCachedParserWithConfig()` - constructor with config
- `ParseFile()` - **FULL CACHE LOGIC NEEDED**
  - Cache hit scenario
  - Cache miss scenario
  - Cache disabled (EnableCaching = false)
  - Cache entry copying to target parameter
  - TTL expiration
- `ParseFileToMap()` - **FULL CACHE LOGIC NEEDED**
- `ParseString()` - delegation (no caching)
- `MustParseFile()` - delegation with panic
- `ClearCache()` - clears cache map
- `CacheSize()` - returns len(cache)
- `CacheStats()` - returns statistics
- `getFromCache()` - private method test via integration
  - Entry found and not expired
  - Entry found but expired (TTL)
  - Entry not found
- `addToCache()` - private method test via integration
  - Normal addition
  - LRU eviction when cache full
- `evictLRU()` - private method test via integration
  - Finds and evicts least recently used entry
  - Handles empty cache

#### StreamingParser Implementation (lines 710-803)
- `NewStreamingParser()` - constructor
- `NewStreamingParserWithConfig()` - constructor
- `ParseFile()` - **NEEDS TESTS**
  - File size check (MaxFileSize)
  - Error when file too large
  - Delegation to standard parser
- `ParseFileToMap()` - delegation test
- `ParseString()` - delegation test
- `MustParseFile()` - delegation test
- `Config()` - returns config
- `SetBufferSize()` - sets buffer size in struct and config

#### DefaultParserFactory Implementation (lines 808-845)
- `NewParserFactory()` - constructor
- `CreateParser()` - **FULL FACTORY LOGIC NEEDED**
  - Returns CachedParser when EnableCaching = true
  - Returns StreamingParser when EnableStreaming = true
  - Returns StandardParser as default
- `CreateDefaultParser()` - returns StandardParser
- `CreateStrictParser()` - returns parser with StrictParserConfig()

#### DefaultProcessor (lines 321-360)
- `NewDefaultProcessor()` - constructor
- `NewStrictProcessor()` - constructor
- Integration with all interfaces

### Required Test Cases

#### Test Case: Interfaces-001 - CachedParser Cache Hit
**Setup:** Parse same file twice with caching enabled
**Expected:** Second call returns cached result, cacheStats.Hits = 1

#### Test Case: Interfaces-002 - CachedParser Cache Miss
**Setup:** Parse different files with caching enabled
**Expected:** Each parse caches result, cacheStats.Misses = N

#### Test Case: Interfaces-003 - CachedParser TTL Expiration
**Setup:** Parse file, wait for TTL to expire, parse again
**Expected:** Second parse is cache miss, entry evicted

#### Test Case: Interfaces-004 - CachedParser LRU Eviction
**Setup:** Fill cache to MaxCacheSize, add one more entry
**Expected:** Least recently used entry evicted, Evictions = 1

#### Test Case: Interfaces-005 - CachedParser Disabled
**Setup:** Parse with EnableCaching = false
**Expected:** No caching occurs, always cache miss behavior

#### Test Case: Interfaces-006 - StreamingParser File Size Limit
**Setup:** Parse file larger than MaxFileSize
**Expected:** Returns error with "file size X exceeds maximum Y"

#### Test Case: Interfaces-007 - StandardParser MustParseFile Panic
**Setup:** MustParseFile with invalid YAML file
**Expected:** Function panics with descriptive error message

#### Test Case: Interfaces-008 - Factory CreateParser Selection
**Setup:**
1. Config with EnableCaching = true → expect CachedParser
2. Config with EnableStreaming = true → expect StreamingParser
3. Default config → expect StandardParser
**Expected:** Correct parser type returned for each config

#### Test Case: Interfaces-009 - DefaultProcessor Integration
**Setup:** Create DefaultProcessor, use all interface methods
**Expected:** All methods delegate to correct underlying implementations

---

## 5. result_types.go (26.0% Coverage) - HIGH PRIORITY

### Missing Method Coverage

#### SuccessParseResult[T] Methods (lines 188-278)
- `FilePath()` - returns path when source is file
- `IsFile()` - checks if source type is SourceFile
- `IsMultiDocument()` - checks DocumentCount > 1
- `Size()` - returns source size
- `LineCount()` - returns metadata line count
- `String()` - formatted summary
- `ToLegacy()` - converts to ParseResult
- `GetRawBytes()` - returns raw YAML bytes
- `GetRawString()` - returns raw YAML string
- `HasRaw()` - checks if Raw != nil
- `RawSize()` - returns len(Raw)

#### ParseResult Methods (lines 343-405)
- `IsFailure()` - returns !Success
- `IsSuccess()` - returns Success
- `GetDetailedError()` - **COMPREHENSIVE ERROR EXTRACTION NEEDED**
  - Extracts from ParseError
  - Extracts from SyntaxError
  - Extracts from StructureError
  - Extracts from YAMLParseError
  - Handles unknown error types

#### ValidationResult Methods (lines 471-561)
- `HasErrors()` - checks len(Errors) > 0
- `IsValid()` - returns Valid field
- `HasWarnings()` - checks len(Warnings) > 0
- `ErrorCount()` - returns len(Errors)
- `WarningCount()` - returns len(Warnings)
- `ErrorSummary()` - formatted error summary
- `WarningSummary()` - formatted warning summary
- `FullSummary()` - complete formatted summary

#### SchemaValidationResult Methods (lines 614-666)
- `HasErrors()` - checks errors or !Valid
- `HasWarnings()` - checks len(Warnings) > 0
- `ErrorSummary()` - **COMPREHENSIVE SUMMARY NEEDED**
  - Missing required fields section
  - Type mismatches section
  - Constraint violations section
  - General errors section

#### ProcessingResult Methods (lines 695-732)
- `HasParseErrors()` - checks ParseResult.Success
- `HasValidationErrors()` - checks ValidationResult
- `Summary()` - formatted summary

#### FieldAccessResult Methods (lines 758-785)
- `IsSuccess()` - checks Error == nil && Exists && !IsNil
- `IsMissing()` - checks !Exists || IsNil
- `String()` - formatted representation

#### BatchValidationResult Methods (lines 814-878)
- `HasErrors()` - checks TotalErrors > 0
- `HasWarnings()` - checks TotalWarnings > 0
- `SuccessRate()` - calculates percentage
- `Summary()` - formatted batch summary
- `GetFailedFiles()` - returns list of failed file paths
- `GetResultsByStatus()` - splits into valid/invalid slices

#### Helper Types Methods
- `ParseSource.String()` - string representation
- `ParseSource.IsFile()` - type check
- `ParseMetadata.String()` - string representation
- `ParseTiming.String()` - string representation
- `ParseTiming.IsZero()` - checks if all times are zero
- `DetailedParseError.String()` - formatted error with all fields

### Required Test Cases

#### Test Case: ResultTypes-001 - GetDetailedError from ParseError
**Setup:** ParseResult with ParseError
**Expected:** DetailedParseError with FilePath, Line, Column, Message, ErrorType

#### Test Case: ResultTypes-002 - GetDetailedError from SyntaxError
**Setup:** ParseResult with SyntaxError
**Expected:** DetailedParseError includes Expected and Found fields

#### Test Case: ResultTypes-003 - GetDetailedError from StructureError
**Setup:** ParseResult with StructureError
**Expected:** DetailedParseError includes Context (Location)

#### Test Case: ResultTypes-004 - GetDetailedError Unknown Error
**Setup:** ParseResult with generic error
**Expected:** DetailedParseError with ErrorType = "unknown"

#### Test Case: ResultTypes-005 - ValidationResult ErrorSummary
**Setup:** ValidationResult with 3 errors
**Expected:** Formatted summary listing all 3 errors with file path

#### Test Case: ResultTypes-006 - SchemaValidationResult ErrorSummary
**Setup:** SchemaValidationResult with missing fields, type errors, constraint violations
**Expected:** Summary with 3 sections: Missing Required Fields, Type Mismatches, Constraint Violations

#### Test Case: ResultTypes-007 - BatchValidationResult SuccessRate
**Setup:** 10 files, 7 valid, 3 invalid
**Expected:** SuccessRate() returns 70.0

#### Test Case: ResultTypes-008 - BatchValidationResult GetFailedFiles
**Setup:** BatchValidationResult with mixed results
**Expected:** Returns slice of 3 file paths that failed

#### Test Case: ResultTypes-009 - SuccessParseResult ToLegacy
**Setup:** SuccessParseResult with all fields populated
**Expected:** ParseResult with Success=true, Data populated, Metrics populated

#### Test Case: ResultTypes-010 - FieldAccessResult IsSuccess
**Setup:** FieldAccessResult with value, exists=true, isnil=false, error=nil
**Expected:** IsSuccess() returns true

---

## 6. validator.go (75.0% Coverage) - HIGH PRIORITY

### Missing Coverage Areas

#### LocalValidationError Methods
- `Error()` - formatted error message
- `String()` - formatted with context
- `ToValidationError()` - conversion

#### categorizeError() Function (lines 230-261)
- **ALL ERROR PATTERNS NEEDED**
  - "could not find expected" → ErrorTypeSyntax
  - "did not find expected key" → ErrorTypeSyntax
  - "found character that cannot start any key" → ErrorTypeSyntax
  - "invalid indentation" → ErrorTypeSyntax
  - "duplicate key" → ErrorTypeStructure
  - "mapping values are not allowed" → ErrorTypeSyntax
  - "unexpected end" → ErrorTypeSyntax
  - "unacceptable character" → ErrorTypeSyntax
  - "scanner error" → ErrorTypeSyntax
  - "unmarshal errors" → ErrorTypeStructure
  - "cannot unmarshal" → ErrorTypeStructure
  - unknown → ErrorTypeUnknown

#### checkNode() for Structural Issues (lines 273-308)
- **DUPLICATE KEY DETECTION NEEDED**
  - Detect duplicate keys in mappings
  - Generate warning with line/column
  - Recursive checking of nested structures

#### parseYAMLError() Function (lines 176-228)
- **LINE/COLUMN EXTRACTION NEEDED**
  - Parse "line X" from error message
  - Parse "column Y" from error message
  - Extract context from YAML content
  - Generate pointer (^) for column position
  - Handle missing line info

### Required Test Cases

#### Test Case: Validator-001 - Empty YAML Content
**Input:** ValidateString("")
**Expected:** Valid=false, Error="YAML content is empty", ErrorType=ErrorTypeEmpty

#### Test Case: Validator-002 - categorizeError Patterns
**Inputs:** Error messages for each pattern
- "could not find expected ':'"
- "did not find expected key"
- "found character that cannot start any key"
- "invalid indentation"
- "duplicate key 'foo'"
- "mapping values are not allowed in this context"
- "unexpected end of file"
- "unacceptable character"
- "scanner error"
- "unmarshal errors"
- "cannot unmarshal"
**Expected:** Correct ErrorType for each pattern

#### Test Case: Validator-003 - Duplicate Key Detection
**Input:** YAML with duplicate key in mapping
```yaml
config:
  value: 1
  value: 2
```
**Expected:** Warnings contains DuplicateKeyError or structural warning

#### Test Case: Validator-004 - Line/Column Extraction
**Input:** YAML error at specific line/column
**Expected:** LocalValidationError with correct Line and Column set

#### Test Case: Validator-005 - Context Extraction
**Input:** YAML error with line number
**Expected:** Context contains "Line content: '...'" with problematic line

#### Test Case: Validator-006 - Column Pointer
**Input:** YAML error with line and column
**Expected:** Context includes pointer spaces + "^" at correct position

#### Test Case: Validator-007 - Unknown Error Pattern
**Input:** Error message not matching any pattern
**Expected:** ErrorType = ErrorTypeUnknown

#### Test Case: Validator-008 - ValidateFile Read Error
**Input:** Non-existent file path
**Expected:** Valid=false, Error contains "Failed to read file", ErrorType=ErrorTypeIO

#### Test Case: Validator-009 - ValidateStringWithPath
**Input:** ValidateStringWithPath(validYAML, "test.yaml")
**Expected:** ValidationResult with FilePath = "test.yaml"

#### Test Case: Validator-010 - ValidateMultipleFiles
**Input:** Array of file paths (mix of valid and invalid)
**Expected:** []ValidationResult with one result per file

---

## 7. errors.go (66.5% Coverage) - HIGH PRIORITY

### Missing Coverage Areas

#### Helper Functions (lines 724-746)
- `containsRequiredFieldKeywords()` - checks message for keywords
  - Keywords: "required", "missing", "not found", "must be provided"
- `containsConstraintKeywords()` - checks message for keywords
  - Keywords: "constraint", "range", "length", "pattern", "invalid value"

#### Error Type Check Functions (lines 1194-1279)
- `isYAMLErrorOfType()` - unwraps and checks type
  - Handles nil error
  - Unwraps error chain
  - Checks YAMLError interface
- `IsYAMLError()` - checks if error implements interface
- `GetYAMLErrorType()` - extracts ErrorType from error
  - Handles nil error
  - Unwraps error chain
  - Returns empty string for non-YAMLError
- `IsFileNotFoundError()` - checks for file not found
  - Uses os.IsNotExist()
  - Checks wrapped errors
- `IsPermissionError()` - checks for permission denied
  - Uses os.IsPermission()
  - Checks wrapped errors

#### Error Methods
- `ParseError.Unwrap()` - returns underlying error
- `ValidationError.Unwrap()` - returns underlying error
- `FileError.Unwrap()` - returns underlying error
- `SyntaxError.Unwrap()` - returns underlying error
- `StructureError.Unwrap()` - returns underlying error
- `SchemaLoadError.Unwrap()` - returns underlying error

#### ParseErrorVariant Methods (lines 133-156)
- `String()` - returns string representation
- `Description()` - returns human-readable description

### Required Test Cases

#### Test Case: Errors-001 - containsRequiredFieldKeywords
**Inputs:**
- "field is required" → true
- "missing value" → true
- "not found in config" → true
- "must be provided" → true
- "some other error" → false
**Expected:** Correct boolean for each input

#### Test Case: Errors-002 - containsConstraintKeywords
**Inputs:**
- "constraint violation" → true
- "out of range" → true
- "length too long" → true
- "pattern mismatch" → true
- "invalid value detected" → true
- "some other error" → false
**Expected:** Correct boolean for each input

#### Test Case: Errors-003 - isYAMLErrorOfType
**Setup:** Wrapped error chain
**Input:** fmt.Errorf("wrapper: %w", &ParseError{ErrorType: ErrorTypeParse})
**Expected:** Returns true for ErrorTypeParse

#### Test Case: Errors-004 - IsYAMLError
**Inputs:**
- nil → false
- errors.New("standard") → false
- &ParseError{} → true
- &ValidationError{} → true
**Expected:** Correct boolean for each input

#### Test Case: Errors-005 - GetYAMLErrorType
**Inputs:**
- nil → ""
- errors.New("standard") → ""
- &ParseError{ErrorType: ErrorTypeParse} → ErrorTypeParse
- Wrapped ValidationError → ErrorTypeValidation
**Expected:** Correct ErrorType for each input

#### Test Case: Errors-006 - IsFileNotFoundError
**Inputs:**
- nil → false
- os.ErrNotExist → true
- &FileError{Err: os.ErrNotExist} → true
- Wrapped os.ErrNotExist → true
- Other error → false
**Expected:** Correct boolean for each input

#### Test Case: Errors-007 - IsPermissionError
**Inputs:**
- nil → false
- os.ErrPermission → true
- &FileError{Err: os.ErrPermission} → true
- Wrapped os.ErrPermission → true
- Other error → false
**Expected:** Correct boolean for each input

#### Test Case: Errors-008 - ParseErrorVariant Description
**Inputs:** All variants (Syntax, TypeMismatch, Validation, IO, Structure, Custom)
**Expected:** Human-readable description for each variant

#### Test Case: Errors-009 - Error Unwrap Methods
**Setup:** Each error type with underlying error
**Expected:** Unwrap() returns the underlying error

---

## Error Case Coverage Matrix

### File I/O Errors
| Error Type | Function | Test Coverage Needed |
|------------|----------|---------------------|
| File not found | LoadSchema, ValidateFile | ✅ Documented |
| Permission denied | LoadSchema, ValidateFile | ✅ Documented |
| Empty file | ValidateFile, LoadSchema | ✅ Documented |
| Read failure | LoadSchema, ValidateFile | ✅ Documented |
| Directory instead of file | LoadSchema | ❌ Missing |

### YAML Syntax Errors
| Error Type | Pattern | Test Coverage |
|------------|---------|---------------|
| Invalid indentation | "invalid indentation" | ✅ Documented |
| Unexpected character | "found character that cannot" | ✅ Documented |
| Missing colon | "could not find expected ':'" | ✅ Documented |
| Duplicate key | "duplicate key" | ✅ Documented |
| Invalid escape | Various patterns | ❌ Missing |
| Invalid quote | Various patterns | ❌ Missing |

### Type Errors
| Type | Expected | Actual | Test Coverage |
|------|----------|--------|---------------|
| String | Integer | Number | ✅ Documented |
| Integer | String | Text | ✅ Documented |
| Boolean | String | "true"/"false" | ❌ Missing |
| Array | Object | Map | ✅ Documented |
| Object | Scalar | String/Number | ❌ Missing |
| Null | Required field | nil | ❌ Missing |

### Constraint Violations
| Constraint | Invalid Value | Test Coverage |
|------------|---------------|---------------|
| Min | Below minimum | ✅ Documented |
| Max | Above maximum | ✅ Documented |
| Pattern | Regex mismatch | ✅ Documented |
| Enum | Not in list | ✅ Documented |
| Min length | String too short | ❌ Missing |
| Max length | String too long | ❌ Missing |
| Required | Missing field | ✅ Documented |

---

## Summary of Test Cases Required

### Critical Priority (0% coverage files)
- **schema.go**: 14 test cases
- **schema_interfaces.go**: Verify interface-only content
- **template.go**: Investigate and test

### High Priority (< 80% coverage files)
- **interfaces.go**: 9 test cases
- **result_types.go**: 10 test cases
- **validator.go**: 10 test cases
- **errors.go**: 9 test cases

### Total Estimated Test Cases
**~52 new test cases** needed to achieve comprehensive coverage

---

## Implementation Recommendations

### Phase 1: Critical Coverage (schema.go)
1. Implement Schema-001 through Schema-014 test cases
2. Create test fixture files for schemas and configs
3. Test all error paths in LoadSchema()
4. Test all validation paths in Validate()

### Phase 2: High Priority Coverage
1. Implement interfaces.go test cases (focus on caching logic)
2. Implement result_types.go test cases (focus on summary methods)
3. Implement validator.go test cases (focus on error categorization)
4. Implement errors.go test cases (focus on helper functions)

### Phase 3: Medium Priority
1. Investigate template.go and implement appropriate tests
2. Verify schema_interfaces.go is interface-only
3. Add any missing edge case tests

---

## Testing Strategy

### Test Organization
```
internal/yamlutil/
├── schema_test.go           (new - schema.go tests)
├── schema_validation_test.go (new - comprehensive validation)
├── interfaces_cache_test.go (new - CachedParser tests)
├── interfaces_factory_test.go (new - Factory tests)
├── result_types_test.go      (expand existing)
├── validator_errors_test.go (expand existing)
└── errors_helpers_test.go   (expand existing)
```

### Test Fixtures Required
```
internal/yamlutil/testdata/
├── schemas/
│   ├── valid.yaml
│   ├── invalid.yaml
│   ├── empty.yaml
│   └── complex.yaml
├── configs/
│   ├── valid.yaml
│   ├── missing_required.yaml
│   ├── type_mismatch.yaml
│   └── constraint_violations.yaml
└── invalid/
    ├── malformed.yaml
    ├── empty.yaml
    └── syntax_errors.yaml
```

---

## Conclusion

The yamlutil package requires substantial test coverage improvements, particularly in:
1. **Schema validation** (0% coverage - entire subsystem untested)
2. **Interface implementations** (21.8% - critical caching logic untested)
3. **Result type methods** (26% - summary and formatting untested)
4. **Error handling helpers** (66.5% - categorization and detection untested)

Implementing the documented **52 test cases** will bring coverage above 80% across all files and ensure robust error handling throughout the package.
