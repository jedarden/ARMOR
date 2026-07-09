# YAML Parser Module Data Flow

This document describes the data flow through the YAML parser module, from input files to parsed and validated data structures.

## Overview

The YAML parser module provides a comprehensive pipeline for processing YAML files through multiple stages:

```
Input File → File I/O → Parsing → Validation → Schema Validation → Field Access → Output
```

## Components and Data Flow

### 1. File Input Stage

**Entry Points:**
- `ParseFile(filePath string, data interface{})` - Parse into typed struct
- `ParseFileToMap(filePath string)` - Parse into generic map
- `ValidateFile(filePath string)` - Validate YAML syntax
- `LoadSchema(schemaPath string)` - Load schema definition

**Data Flow:**
```
File System → FileReader.ReadFile() → []byte
```

**Components:**
- `FileReader` interface - Abstracts file I/O operations
- `FileError` - Encapsulates file I/O errors with context
- Helper functions: `ReadFile()`, `FileExists()`, `IsYAMLFile()`

**Error Handling:**
- File not found → `FileError` with `os.ErrNotExist`
- Permission denied → `FileError` with `os.ErrPermission`
- Path resolution errors → `FileError` with operation context

### 2. Parsing Stage

**Input:** Raw byte array from file I/O
**Output:** Parsed data structure + `ParseResult`

**Data Flow:**
```
[]byte → yaml.Unmarshal() → interface{} → ParseResult
```

**Components:**
- `Parser` struct - Core parsing engine with configuration
- `ParserConfig` - Controls parsing behavior (strict mode, duplicate keys, etc.)
- `ParseResult` - Encapsulates parsing outcome with data/error

**Parser Types:**
- `NewParser()` - Default lenient parser
- `NewStrictParser()` - Strict mode (rejects unknown fields)
- Configurable via `ParserConfig`

**Parsing Behavior:**
```
Content Check → Empty/Whitespace? → Return Empty Map
                  ↓
                No → yaml.Unmarshal() → Syntax Error? → Return YAMLParseError
                                              ↓
                                            No → Success → Return ParseResult{Success: true}
```

**Configuration Options:**
- `Strict` - Reject unknown struct fields
- `AllowDuplicateKeys` - Permit duplicate mapping keys
- `PreserveOrder` - Maintain YAML key order
- `EmptyFileAsError` - Treat empty files as errors
- `MaxFileSize` - Limit file size
- `Encoding` - Character encoding (UTF-8, UTF-16LE, UTF-16BE)

### 3. Validation Stage

**Input:** Raw YAML content (string or bytes)
**Output:** `ValidationResult` with errors/warnings

**Data Flow:**
```
string/yaml.Content → yaml.Node → Syntax Check → ValidationResult
                                          ↓
                                    Structural Check → Duplicate Keys?
                                                        ↓
                                                    Warning Collection
```

**Components:**
- `Validator` struct - Validation engine with configuration
- `ValidatorConfig` - Controls validation strictness
- `ValidationResult` - Detailed validation outcome
- `ValidationError` - Individual error with line/column info

**Validation Types:**
- `NewValidator()` - Default balanced validation
- `NewStrictValidator()` - Production-ready strict validation
- Configurable via `ValidatorConfig`

**Validation Checks:**
1. **Syntax Validation** - YAML structure compliance
2. **Empty Content** - Detect empty files/strings
3. **Duplicate Keys** - Find duplicate mapping keys
4. **Indentation** - Check consistency (optional)
5. **Line Length** - Warn on long lines (optional)
6. **Document Structure** - Validate YAML markers (optional)

**Error Categories:**
- `ErrorTypeSyntax` - YAML syntax errors
- `ErrorTypeStructure` - Structural problems (duplicates, nesting)
- `ErrorTypeIO` - File I/O errors
- `ErrorTypeEmpty` - Empty content
- `ErrorTypeDeprecated` - Deprecated features
- `ErrorTypeUnknown` - Uncategorized errors

### 4. Schema Validation Stage

**Input:** Parsed YAML data + Schema definition
**Output:** `SchemaValidationResult` with constraint violations

**Data Flow:**
```
map[string]interface{} + Schema → Field Validation → Type Check → Constraints Check
                                                          ↓
                                                    Nested Validation
                                                          ↓
                                              SchemaValidationResult
```

**Components:**
- `Schema` - Schema definition with field constraints
- `SchemaValidator` - Schema-based validation engine
- `SchemaDefinition` interface - Schema abstraction
- `FieldDefinition` - Individual field constraints
- `SchemaValidationResult` - Comprehensive validation outcome

**Validation Process:**
```
For each field:
  1. Required Check → Missing? → Add MissingRequiredFields
  2. Type Check → Mismatch? → Add TypeMismatches
  3. Constraint Check → Violation? → Add ConstraintViolations
  4. Nested Schema → Recursive validation
  5. Array Items → Item schema validation
```

**Field Constraints:**
- `Type` - Expected data type (string, int, float, bool, array, object)
- `Required` - Field must be present
- `Min/Max` - Numeric range or string/array length
- `Pattern` - Regex pattern for strings
- `AllowedValues` - Enumeration constraint
- `DefaultValue` - Fallback value
- `NestedSchema` - Schema for nested structures
- `ArrayItemSchema` - Schema for array elements

**Schema Loading:**
```
File System → LoadSchema() → JSON/YAML → Parse → BuildSchemaFromData()
```

### 5. Field Access Stage

**Input:** Parsed YAML data (map[string]interface{})
**Output:** Typed field values or errors

**Data Flow:**
```
map[string]interface{} → Path Navigation → Type Assertion → Value
                                                               ↓
                                                         Type Mismatch Error
                                                               ↓
                                                         Field Not Found Error
```

**Components:**
- `FieldAccessor` interface - Field navigation abstraction
- `FieldNotFoundError` - Missing field error
- `TypeMismatchError` - Type assertion error
- `FieldRequirement` - Field validation specification

**Field Access Methods:**
- `GetField(data, path, defaultValue)` - Get with default fallback
- `GetString/GetInt/GetBool(data, path, defaultValue)` - Type-safe getters
- `HasField(data, path)` - Check field existence
- `GetRequiredField/String/Int/Bool(data, path)` - Required field access (errors)

**Path Navigation:**
```
Dot Notation: "server.port" → data["server"]["port"]
Nested Access: Recursive map traversal
```

**Field Requirements Validation:**
```
FieldRequirement[] → ValidateFieldRequirements() → Error List
                                                    ↓
                                          Missing Fields → FieldNotFoundError
                                                    ↓
                                          Type Mismatch → TypeMismatchError
```

## Complete Processing Pipeline

### Standard Parse Flow

```
1. Input: filePath string
   ↓
2. File I/O: FileReader.ReadFile(filePath) → []byte, error
   ↓
3. Size Check: MaxFileSize validation
   ↓
4. Empty Check: EmptyFileAsError validation
   ↓
5. Parse: yaml.Unmarshal(content, &data) → error
   ↓
6. Result: ParseResult{Success, Data, Error}
```

### Validation Flow

```
1. Input: filePath string or yamlContent string
   ↓
2. File I/O (if file): os.ReadFile(filePath) → []byte
   ↓
3. Empty Check: TrimSpace() == "" → EmptyError
   ↓
4. Syntax Parse: yaml.Unmarshal(&node) → error
   ↓
5. Error Extraction: parseYAMLError() → ValidationError with line/column
   ↓
6. Structural Check: checkStructuralIssues() → []ValidationError (warnings)
   ↓
7. Result: ValidationResult{Valid, Errors, Warnings}
```

### Schema Validation Flow

```
1. Input: data map[string]interface{} + schema *Schema
   ↓
2. Schema Compile: schema.Validate() → error (once)
   ↓
3. For each root field:
   a. Required Check: Missing? → Add MissingRequiredFields
   b. Type Validation: validateType() → TypeMismatch?
   c. Constraint Validation: validateConstraints() → Violations?
   d. Nested Schema: Recursive validateFields()
   e. Array Items: validateField() for each item
   ↓
4. Result: SchemaValidationResult{Valid, Errors, Warnings, ...}
```

### Field Access Flow

```
1. Input: data map[string]interface{} + path string
   ↓
2. Path Parse: Split on "." → components []
   ↓
3. Traversal: For each component:
   a. Current level map lookup
   b. Type assertion to map[string]interface{}
   c. Continue to next level or return value
   ↓
4. Type Assertion: Assert to expected type
   ↓
5. Result: Value or error (FieldNotFoundError/TypeMismatchError)
```

## Error Flow and Handling

### Error Type Hierarchy

```
Error (interface{})
├── FileError (file.go)
│   ├── Op: string ("read", "resolve", etc.)
│   ├── Path: string
│   └── Err: error (underlying OS error)
│
├── YAMLParseError (parser.go)
│   ├── FilePath: string
│   ├── Line: int
│   ├── Column: int
│   ├── Message: string
│   └── RawError: error
│
├── ValidationError (validator.go)
│   ├── Type: ErrorType
│   ├── Line: int
│   ├── Column: int
│   ├── Message: string
│   ├── Context: string
│   └── FilePath: string
│
├── FieldNotFoundError (debug_helpers.go)
│   └── FieldPath: string
│
├── TypeMismatchError (debug_helpers.go)
│   ├── FieldPath: string
│   ├── ExpectedType: string
│   └── ActualType: string
│
└── SchemaError (schema.go)
    ├── Message: string
    └── FilePath: string
```

### Error Propagation

```
File Error → Wrap in FileError → Propagate to caller
                ↓
            Parse Error → Wrap in YAMLParseError → ParseResult{Success: false}
                ↓
            Validation Error → Collect in ValidationResult.Errors
                ↓
            Schema Error → Collect in SchemaValidationResult.Errors
                ↓
            Field Access Error → Return FieldNotFoundError/TypeMismatchError
```

## Configuration Flow

### Parser Configuration

```
DefaultParserConfig() → ParserConfig → Apply to Parser
StrictParserConfig()   → ParserConfig → Apply to Parser
Custom ParserConfig     → ParserConfig → Apply to Parser
```

**Configuration Application:**
1. Create `ParserConfig` with desired options
2. Use `Parser{strict: config.Strict}` or similar
3. Parser respects config during parsing operations

### Validator Configuration

```
DefaultValidatorConfig() → ValidatorConfig → Apply to Validator
StrictValidatorConfig()   → ValidatorConfig → Apply to Validator
LenientValidatorConfig()   → ValidatorConfig → Apply to Validator
```

**Configuration Application:**
1. Create `ValidatorConfig` with desired options
2. Use `Validator{strict: config.Strict}` or similar
3. Validator respects config during validation operations

## Caching and Performance

### Potential Caching Points

1. **Parsed Data Cache:**
   - Cache parsed `map[string]interface{}` by file path
   - Invalidate on file modification (file watcher)
   - Reduce repeated parsing overhead

2. **Schema Cache:**
   - Cache compiled schemas by schema path
   - Reuse for multiple validations
   - Reduce schema compilation overhead

3. **Field Access Cache:**
   - Cache field navigation results
   - Invalidate on data modification
   - Optimize repeated field access

### Performance Considerations

- **File I/O:** Largest cost factor, batch file operations when possible
- **Parsing:** yaml.Unmarshal is CPU-intensive for large files
- **Validation:** Syntax check is fast, structural checks add overhead
- **Schema Validation:** Depends on schema complexity and data size
- **Field Access:** Path navigation has O(depth) complexity

## Concurrent Processing

### Thread Safety

- **File I/O:** Safe (each operation independent)
- **Parsing:** Safe (no shared state in Parser)
- **Validation:** Safe (no shared state in Validator)
- **Field Access:** Safe (read-only operations on data)
- **Caching:** Requires synchronization if implemented

### Batch Processing

```
Multiple Files → Parallel ParseFile() → []ParseResult
                              ↓
                        Parallel ValidateFile() → []ValidationResult
                              ↓
                        Parallel Validate(schema) → []SchemaValidationResult
```

## Integration Examples

### Complete Validation Pipeline

```go
// 1. Parse file
parser := NewParser()
result := parser.ParseFileToMap("config.yaml")
if !result.Success {
    return result.Error
}

// 2. Validate syntax
validator := NewValidator()
vResult := validator.ValidateFile("config.yaml")
if vResult.HasErrors() {
    return vResult.ErrorSummary()
}

// 3. Validate against schema
schema, _ := LoadSchema("schema.json")
schemaValidator := NewSchemaValidator(schema)
sResult := schemaValidator.Validate(result.Data.(map[string]interface{}))
if sResult.HasErrors() {
    return sResult.ErrorSummary()
}

// 4. Access fields
sessionId := GetString(result.Data.(map[string]interface{}), "session.id", "")
```

### Field Validation Pipeline

```go
// 1. Parse data
data, err := ParseYAML("config.yaml")
if err != nil {
    return err
}

// 2. Define requirements
requirements := []FieldRequirement{
    {Path: "server.host", Type: "string", Optional: false},
    {Path: "server.port", Type: "int", Optional: false},
    {Path: "debug.enabled", Type: "bool", Optional: true},
}

// 3. Validate requirements
errors := ValidateFieldRequirements(data, requirements)
if len(errors) > 0 {
    return errors
}

// 4. Access required fields
host, err := GetRequiredString(data, "server.host")
if err != nil {
    return err
}
```

## Data Structures

### Key Data Structures

```
ParseResult:
  - FilePath: string
  - Data: interface{}
  - Success: bool
  - Error: error

ValidationResult:
  - FilePath: string
  - Valid: bool
  - Errors: []ValidationError
  - Warnings: []ValidationError

SchemaValidationResult:
  - Valid: bool
  - Errors: []SchemaValidationError
  - Warnings: []SchemaValidationError
  - MissingRequiredFields: []string
  - TypeMismatches: []FieldTypeError
  - ConstraintViolations: []ConstraintViolation

FieldConstraints:
  - Type: string
  - Required: bool
  - Min: *int
  - Max: *int
  - Pattern: string
  - AllowedValues: []interface{}
  - DefaultValue: interface{}
```

## Summary

The YAML parser module provides a comprehensive, well-structured pipeline for processing YAML files with the following key characteristics:

1. **Modular Design:** Clear separation between file I/O, parsing, validation, and field access
2. **Configurable Behavior:** Flexible parser and validator configuration options
3. **Comprehensive Error Handling:** Detailed error types with context and location information
4. **Schema Support:** JSON Schema-style validation with type checking and constraints
5. **Type-Safe Access:** Field navigation with type assertions and error reporting
6. **Performance Considerations:** Opportunities for caching and batch processing

The data flow is designed to be both efficient and easy to understand, with clear error propagation and flexible configuration options for different use cases (development, production, legacy support, etc.).
