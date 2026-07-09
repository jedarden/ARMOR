# YAML Parser Module Structure

## Overview

The YAML parser module (`internal/yamlutil/`) provides comprehensive YAML parsing, validation, and field access utilities for ARMOR. The module is designed for robust YAML handling in Go applications with three main components:

1. **File I/O Operations** - Safe, error-contextualized YAML file operations
2. **YAML Parsing** - Parse YAML files into typed structures or generic maps  
3. **Field Access** - Type-safe helpers for accessing nested YAML fields
4. **Validation** - YAML syntax and structure validation with detailed error reporting

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     yamlutil Package                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   File I/O   │  │   Parser     │  │  Validator   │     │
│  │   Module     │  │   Module     │  │   Module     │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
│                                                              │
│  ┌──────────────────────────────────────────────────┐      │
│  │         Field Access / Debug Helpers              │      │
│  └──────────────────────────────────────────────────┘      │
│                                                              │
│  ┌──────────────────────────────────────────────────┐      │
│  │            Error Types & Interfaces               │      │
│  └──────────────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

## Module Components

### 1. File I/O Module (`file.go`)

**Purpose**: Provide safe file operations with comprehensive error handling

**Key Types**:
- `FileError` - File operation errors with context
- `ReadFile` - Read file contents with error context
- `FileExists` - Check file existence with permission handling
- `IsFileNotFoundError` - Error type checking
- `IsPermissionError` - Error type checking

**Design Patterns**:
- Error wrapping for context preservation
- Permission-aware file existence checking
- Absolute path resolution for better error messages

### 2. Parser Module (`parser.go`)

**Purpose**: Parse YAML files with flexible data structures

**Key Types**:
- `Parser` - YAML parser with strict mode support
- `ParseResult` - Structured parsing results with metadata
- `YAMLParseError` - Detailed syntax errors with line/column info
- `NewParser()` - Standard parser
- `NewStrictParser()` - Strict mode parser

**Core Methods**:
```go
func (p *Parser) ParseFile(filePath string, data interface{}) ParseResult
func (p *Parser) ParseFileToMap(filePath string) ParseResult
func (p *Parser) ParseString(yamlContent string, data interface{}) error
func (p *Parser) MustParseFile(filePath string, data interface{})
```

**Design Patterns**:
- Structured result objects for error handling
- Generic and typed parsing options
- Graceful handling of empty files
- Line/column error extraction

### 3. Validator Module (`validator.go`)

**Purpose**: Validate YAML syntax and structure with detailed reporting

**Key Types**:
- `Validator` - YAML validator with strict mode
- `ValidationResult` - Structured validation results
- `ValidationError` - Detailed error information
- `ErrorType` - Categorized error types (syntax, structure, I/O, etc.)

**Error Types**:
- `ErrorTypeSyntax` - YAML syntax errors
- `ErrorTypeStructure` - Structural errors (duplicate keys, nesting)
- `ErrorTypeIO` - I/O errors (file not found, permissions)
- `ErrorTypeEmpty` - Empty file or content
- `ErrorTypeDeprecated` - Deprecated YAML features

**Core Methods**:
```go
func (v *Validator) ValidateFile(filePath string) ValidationResult
func (v *Validator) ValidateString(yamlContent string) ValidationResult
func (v *Validator) ValidateMultipleFiles(filePaths []string) []ValidationResult
```

### 4. Field Access Module (`debug_helpers.go`)

**Purpose**: Type-safe field access with dot notation

**Key Functions**:
- `GetField` - Generic field access with defaults
- `GetString, GetInt, GetBool` - Type-safe field access
- `HasField` - Field existence checking
- `GetRequiredField, GetRequiredString, GetRequiredInt, GetRequiredBool` - Required field access

**Error Types**:
- `FieldNotFoundError` - Missing required fields
- `TypeMismatchError` - Type validation failures

**Design Patterns**:
- Dot notation for nested access (`server.port`)
- Type conversion with fallback
- Required vs optional field handling
- Batch field validation

## Interface Definitions

### Parser Interface

```go
type YAMLParser interface {
    ParseFile(filePath string, data interface{}) ParseResult
    ParseFileToMap(filePath string) ParseResult
    ParseString(yamlContent string, data interface{}) error
    MustParseFile(filePath string, data interface{})
}
```

### Validator Interface

```go
type YAMLValidator interface {
    ValidateFile(filePath string) ValidationResult
    ValidateString(yamlContent string) ValidationResult
    ValidateMultipleFiles(filePaths []string) []ValidationResult
}
```

### Field Accessor Interface

```go
type FieldAccessor interface {
    GetField(data map[string]interface{}, path string, defaultValue interface{}) interface{}
    GetString(data map[string]interface{}, path string, defaultValue string) string
    GetInt(data map[string]interface{}, path string, defaultValue int) int
    GetBool(data map[string]interface{}, path string, defaultValue bool) bool
    HasField(data map[string]interface{}, path string) bool
}
```

## File Organization

```
internal/yamlutil/
├── doc.go              # Package documentation and usage examples
├── file.go             # File I/O operations
├── file_test.go        # File operation tests
├── parser.go           # YAML parsing implementation
├── parser_test.go      # Parser tests
├── validator.go        # YAML validation implementation  
├── validator_test.go   # Validator tests
├── debug_helpers.go    # Field access helpers
├── debug_helpers_test.go # Field access tests
└── types.go            # Common type definitions (future)
```

## API Design Principles

### 1. Error Handling
- All errors include context (file path, operation, line numbers)
- Specific error types for different failure modes
- Error unwrapping support for inspection

### 2. Type Safety
- Both generic (map) and typed (struct) parsing
- Type-safe field access with automatic conversion
- Type validation with detailed mismatch information

### 3. Flexibility
- Support for both strict and lenient parsing/validation
- Optional vs required field handling
- Default values for missing fields

### 4. Performance
- Minimal memory allocations
- Efficient field navigation
- Batch operations for multiple files

## Usage Patterns

### Basic File Reading
```go
content, err := yamlutil.ReadFile("config.yaml")
if err != nil {
    if yamlutil.IsFileNotFoundError(err) {
        log.Fatal("Config file not found")
    }
    log.Fatal(err)
}
```

### Structured Parsing
```go
parser := yamlutil.NewParser()
var config Config
result := parser.ParseFile("config.yaml", &config)
if !result.Success {
    log.Fatal(result.Error)
}
```

### Field Access
```go
data, err := yamlutil.ParseYAML("config.yaml")
if err != nil {
    log.Fatal(err)
}

// Type-safe field access with defaults
port := yamlutil.GetInt(data, "server.port", 8080)
enabled := yamlutil.GetBool(data, "server.enabled", true)

// Required fields
timeout, err := yamlutil.GetRequiredInt(data, "server.timeout")
if err != nil {
    log.Fatal("Missing required field: server.timeout")
}
```

### Validation
```go
validator := yamlutil.NewValidator()
result := validator.ValidateFile("config.yaml")

if result.HasErrors() {
    fmt.Println("Validation errors:")
    for _, err := range result.Errors {
        fmt.Printf("  Line %d: %s\n", err.Line, err.Message)
    }
}
```

### Advanced Usage
```go
// Strict parsing for production
strictParser := yamlutil.NewStrictParser()
strictValidator := yamlutil.NewStrictValidator()

// Batch validation
validator := yamlutil.NewValidator()
results := validator.ValidateMultipleFiles([]string{
    "config.yaml",
    "database.yaml", 
    "logging.yaml",
})

// Recursive YAML discovery
allYAMLs, err := yamlutil.FindYAMLFilesRecursive("/etc/app")
if err != nil {
    log.Fatal(err)
}
```

## Extension Points

### Custom Type Converters
Field access functions support automatic type conversion. To add custom conversions, extend the respective type functions:

```go
// Custom types can be handled by extending type conversion logic
func GetCustomType(data map[string]interface{}, path string) CustomType {
    value, exists := getFieldAtPath(data, path)
    if !exists {
        return DefaultCustomType
    }
    return convertToCustomType(value)
}
```

### Custom Validation Rules
Additional validation can be layered on top of the base validation:

```go
func ValidateCustomRules(data map[string]interface{}) []error {
    var errors []error
    // Add custom validation logic
    return errors
}
```

### Schema Validation (Future)
A schema validation module could be added:

```go
// types.go - Future extension
type SchemaValidator struct {
    schema Schema
}

func (sv *SchemaValidator) Validate(data map[string]interface{}) []ValidationError {
    // Schema validation implementation
}
```

## Testing Strategy

### Unit Tests
- Each module has comprehensive unit tests
- Tests cover normal cases, error cases, and edge cases
- Mock file system for file I/O testing

### Integration Tests
- Multi-module testing (parsing + validation + field access)
- Real YAML file testing
- Error propagation testing

### Test Coverage
Current test coverage:
- `parser_test.go` - Parser functionality
- `validator_test.go` - Validation functionality  
- `debug_helpers_test.go` - Field access functionality
- `file_test.go` - File I/O functionality

## Performance Considerations

### Memory Efficiency
- Parse to specific structs to avoid generic map overhead
- Reuse parser/validator instances for multiple files
- Stream processing for large files (future enhancement)

### Error Performance
- Early returns on fatal errors
- Minimal error message construction until needed
- Efficient error type checking

### Field Access Performance
- Direct map access for first-level fields
- Efficient string splitting for dot notation
- Type assertion caching opportunities

## Dependencies

```
gopkg.in/yaml.v3    # YAML parsing library
```

## Future Enhancements

### Potential Additions
1. **Schema Validation** - JSON Schema-style YAML validation
2. **Stream Processing** - Memory-efficient large file processing  
3. **Caching** - Parsed file caching with invalidation
4. **Advanced Validation** - Cross-field validation, custom rules
5. **YAML Templates** - Variable expansion and template processing
6. **Diff/Merge** - YAML file comparison and merging utilities

### Extension Files
- `schema.go` - Schema validation implementation
- `stream.go` - Stream processing for large files
- `template.go` - Template variable expansion
- `diff.go` - YAML comparison utilities

## Best Practices

### For Users
1. Use typed structs when YAML structure is known
2. Use generic maps for dynamic or unknown structures
3. Always check validation results before using parsed data
4. Use strict mode for production, lenient for development
5. Validate required fields before processing

### For Contributors
1. Maintain error context in all operations
2. Add comprehensive tests for new features
3. Document error types and their meanings
4. Support both strict and lenient modes where applicable
5. Consider performance implications of new features

## Related Documentation

- [Package Documentation](../internal/yamlutil/doc.go) - Comprehensive usage guide
- [Examples](../examples/) - Usage examples (future)
- [Testing Guide](../tests/yamlutil/) - Test patterns and utilities

## Version History

- **v0.1.0** (2024) - Initial implementation with basic parsing, validation, and field access
- **v0.2.0** (2024) - Added comprehensive error types and detailed error reporting  
- **v0.3.0** (2024) - Enhanced field access with type conversion and required field support

## Support and Maintenance

This module is actively maintained as part of the ARMOR project. For questions or issues:

1. Check existing tests for usage patterns
2. Review comprehensive documentation in `doc.go`
3. Examine test files for detailed usage examples