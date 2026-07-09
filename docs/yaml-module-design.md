# YAML Parser Module Design

## Overview

The YAML Parser Module (`internal/yamlutil`) provides comprehensive YAML parsing, validation, and field access utilities for the ARMOR project. The module is designed to handle ARMOR debug files and configuration data with robust error handling and type-safe operations.

## Architecture

### Design Principles

1. **Separation of Concerns**: Each file has a specific responsibility - parsing, validation, file I/O, or field access
2. **Error Context**: All operations provide detailed error context with line/column information
3. **Type Safety**: Type-safe field access with clear error messages for type mismatches
4. **Flexibility**: Support for both struct-based and generic map-based parsing
5. **Validation First**: Separate validation layer to catch syntax errors before processing

### Module Organization

```
internal/yamlutil/
├── doc.go              # Package documentation and usage examples
├── file.go             # File I/O operations with contextual error handling
├── file_test.go        # File operations tests
├── parser.go           # YAML parsing functionality
├── parser_test.go      # Parser tests
├── validator.go        # YAML validation with detailed error reporting
├── validator_test.go   # Validator tests
├── debug_helpers.go    # Field access utilities and type-safe helpers
├── debug_helpers_test.go # Field access tests
└── (Python equivalents for cross-platform compatibility)
```

## Core Components

### 1. File Operations Layer (`file.go`)

**Purpose**: Provides safe file I/O operations with enhanced error context.

**Key Types**:
```go
type FileError struct {
    Op   string // Operation that failed (e.g., "read", "exists")
    Path string // File path that caused the error
    Err  error  // Underlying OS error
}
```

**Key Functions**:
- `ReadFile(filePath string) ([]byte, error)` - Context-aware file reading
- `FileExists(filePath string) bool` - Safe file existence checking
- `IsFileNotFoundError(err error) bool` - Error type checking
- `IsPermissionError(err error) bool` - Permission error detection

**Design Features**:
- Absolute path resolution for better error messages
- Distinguishes between file not found and permission errors
- Wraps OS errors with operation context

### 2. Parser Layer (`parser.go`)

**Purpose**: Core YAML parsing functionality with multiple access patterns.

**Key Types**:
```go
type ParseResult struct {
    FilePath string       // Path to the parsed file
    Data     interface{}  // Parsed YAML data
    Success  bool         // Whether parsing succeeded
    Error    error        // Error if parsing failed
}

type Parser struct {
    strict bool // Strict mode for unknown field rejection
}

type YAMLParseError struct {
    FilePath string
    Line     int
    Column   int
    Message  string
    RawError error
}
```

**Key Functions**:
- `NewParser() *Parser` - Create standard parser
- `NewStrictParser() *Parser` - Create strict mode parser
- `ParseFile(filePath string, data interface{}) ParseResult` - Parse into struct
- `ParseFileToMap(filePath string) ParseResult` - Parse into generic map
- `ParseYAML(filePath string) (map[string]interface{}, error)` - Simplified parsing
- `ParseString(yamlContent string, data interface{}) error` - Parse string content

**Design Features**:
- Handles empty files gracefully (returns empty map, not error)
- Provides line/column information for syntax errors
- Supports both typed structs and generic maps
- Includes file discovery utilities

### 3. Validation Layer (`validator.go`)

**Purpose**: Pre-processing validation to catch syntax and structural errors.

**Key Types**:
```go
type ErrorType string

const (
    ErrorTypeSyntax     ErrorType = "syntax"
    ErrorTypeStructure  ErrorType = "structure"
    ErrorTypeIO         ErrorType = "io"
    ErrorTypeEmpty      ErrorType = "empty"
    ErrorTypeDeprecated ErrorType = "deprecated"
    ErrorTypeUnknown    ErrorType = "unknown"
)

type ValidationError struct {
    Type     ErrorType
    Line     int
    Column   int
    Message  string
    Context  string
    FilePath string
}

type ValidationResult struct {
    FilePath string
    Valid    bool
    Errors   []ValidationError
    Warnings []ValidationError
}

type Validator struct {
    strict bool
}
```

**Key Functions**:
- `NewValidator() *Validator` - Create standard validator
- `NewStrictValidator() *Validator` - Create strict validator
- `ValidateFile(filePath string) ValidationResult` - Validate file
- `ValidateString(yamlContent string) ValidationResult` - Validate string content
- `ValidateMultipleFiles(filePaths []string) []ValidationResult` - Batch validation

**Design Features**:
- Categorizes errors by type (syntax, structure, I/O)
- Provides line/column context for errors
- Separates errors from warnings
- Supports both file and string validation

### 4. Field Access Layer (`debug_helpers.go`)

**Purpose**: Type-safe field access with dot notation navigation.

**Key Types**:
```go
type FieldNotFoundError struct {
    FieldPath string // Dot-separated path to missing field
}

type TypeMismatchError struct {
    FieldPath    string
    ExpectedType string
    ActualType   string
}

type FieldRequirement struct {
    Path         string
    ExpectedType string
    Optional     bool
}
```

**Key Functions**:
- `GetField(data, path, defaultValue)` - Generic field access
- `GetString(data, path, defaultValue) string` - String field access
- `GetInt(data, path, defaultValue) int` - Integer field access  
- `GetBool(data, path, defaultValue) bool` - Boolean field access
- `HasField(data, path) bool` - Field existence check
- `GetRequiredString(data, path) (string, error)` - Required field access
- `GetRequiredInt(data, path) (int, error)` - Required field access
- `GetRequiredBool(data, path) (bool, error)` - Required field access
- `ValidateRequiredFields(data, requiredFields) []string` - Batch validation
- `ValidateFieldRequirements(data, requirements) []error` - Typed validation

**Design Features**:
- Dot notation for nested access (e.g., "server.port")
- Type conversion support (string→int, string→bool)
- Clear error messages for missing/mistyped fields
- Support for optional vs required fields

## API Design Patterns

### Pattern 1: Struct-Based Parsing

```go
// Define configuration structure
type ServerConfig struct {
    Host     string `yaml:"host"`
    Port     int    `yaml:"port"`
    Enabled  bool   `yaml:"enabled"`
}

// Parse into struct
parser := NewParser()
var config ServerConfig
result := parser.ParseFile("config.yaml", &config)

if !result.Success {
    log.Fatal(result.Error)
}

// Use typed configuration
fmt.Println("Server:", config.Host, config.Port)
```

### Pattern 2: Generic Map-Based Parsing

```go
// Parse into generic map
data, err := ParseYAML("config.yaml")
if err != nil {
    log.Fatal(err)
}

// Access fields with dot notation
host := GetString(data, "server.host", "localhost")
port := GetInt(data, "server.port", 8080)
enabled := GetBool(data, "server.enabled", true)
```

### Pattern 3: Validation-First Processing

```go
// Validate first
validator := NewValidator()
result := validator.ValidateFile("config.yaml")

if result.HasErrors() {
    fmt.Println("Validation failed:")
    for _, err := range result.Errors {
        fmt.Printf("  Line %d: %s\n", err.Line, err.Message)
    }
    return
}

// Then process if valid
if result.HasWarnings() {
    fmt.Println("Warnings:")
    for _, warn := range result.Warnings {
        fmt.Printf("  Line %d: %s\n", warn.Line, warn.Message)
    }
}

data, _ := ParseYAML("config.yaml")
```

### Pattern 4: Required Field Validation

```go
// Parse configuration
data, err := ParseYAML("config.yaml")
if err != nil {
    log.Fatal(err)
}

// Define requirements
requirements := []FieldRequirement{
    {Path: "server.host", ExpectedType: "string"},
    {Path: "server.port", ExpectedType: "int"},
    {Path: "database.name", ExpectedType: "string"},
    {Path: "debug.enabled", ExpectedType: "bool", Optional: true},
}

// Validate requirements
errors := ValidateFieldRequirements(data, requirements)
if len(errors) > 0 {
    fmt.Println("Configuration validation failed:")
    for _, err := range errors {
        fmt.Println("  ", err.Error())
    }
    return
}
```

## Error Handling Strategy

### Error Categories

1. **File I/O Errors** (`FileError`)
   - File not found
   - Permission denied
   - Path resolution failures

2. **Parse Errors** (`YAMLParseError`)
   - Syntax errors
   - Structure errors
   - Type conversion failures

3. **Validation Errors** (`ValidationError`)
   - Categorized by type (syntax, structure, I/O)
   - Include line/column context
   - Provide contextual information

4. **Field Access Errors**
   - `FieldNotFoundError` - Missing required fields
   - `TypeMismatchError` - Type conversion failures

### Error Handling Best Practices

```go
// Always check error types for specific handling
data, err := ParseYAML("config.yaml")
if err != nil {
    if parseErr, ok := err.(*YAMLParseError); ok {
        fmt.Printf("Parse error at line %d: %s\n", parseErr.Line, parseErr.Message)
    } else if fileErr, ok := err.(*FileError); ok {
        if IsFileNotFoundError(err) {
            fmt.Println("Config file not found")
        } else if IsPermissionError(err) {
            fmt.Println("Permission denied")
        }
    }
}

// Use validation results for detailed error reporting
result := validator.ValidateFile("config.yaml")
if result.HasErrors() {
    for _, err := range result.Errors {
        switch err.Type {
        case ErrorTypeSyntax:
            fmt.Printf("Syntax error: %s\n", err.Message)
        case ErrorTypeStructure:
            fmt.Printf("Structure error: %s\n", err.Message)
        }
    }
}
```

## Integration Points

### 1. ARMOR Debug File Processing

The module is designed to handle ARMOR debug files with the following characteristics:
- Complex nested structures
- Mixed data types
- Optional and required fields
- Configuration validation requirements

### 2. Configuration Management

```go
// Load and validate application configuration
func LoadConfig(path string) (*Config, error) {
    // Validate first
    validator := NewValidator()
    result := validator.ValidateFile(path)
    if result.HasErrors() {
        return nil, fmt.Errorf("config validation failed: %v", result.Errors)
    }
    
    // Parse into typed structure
    parser := NewParser()
    var config Config
    parseResult := parser.ParseFile(path, &config)
    if !parseResult.Success {
        return nil, parseResult.Error
    }
    
    return &config, nil
}
```

### 3. Tool Integration

The module provides utilities for:
- Batch validation of configuration files
- File discovery in configuration directories
- Debug field access for troubleshooting

## Performance Considerations

1. **Lazy Parsing**: Use `ParseFileToMap` for large files when only partial access is needed
2. **Validation Caching**: Validation results can be cached for unchanged files
3. **Batch Operations**: Use `ValidateMultipleFiles` for processing multiple files
4. **Memory Efficiency**: Generic maps use more memory than structs for complex data

## Testing Strategy

### Unit Tests

Each component has comprehensive test coverage:
- `file_test.go` - File I/O operations
- `parser_test.go` - Parsing functionality
- `validator_test.go` - Validation logic
- `debug_helpers_test.go` - Field access operations

### Test Categories

1. **Happy Path Tests**: Standard usage patterns
2. **Error Cases**: Invalid inputs, missing files, malformed YAML
3. **Edge Cases**: Empty files, special characters, large files
4. **Type Conversion**: Various data type combinations

## Future Enhancements

### Potential Improvements

1. **Streaming Support**: Add support for streaming large YAML files
2. **Schema Validation**: Integrate JSON Schema validation
3. **YAML 1.2 Support**: Upgrade parser for full YAML 1.2 compatibility
4. **Merge Operations**: Add utilities for merging YAML configurations
5. **Path Expressions**: Support more complex path expressions (arrays, wildcards)
6. **Caching Layer**: Add caching for frequently accessed configuration files
7. **Watch Support**: Add file watching for hot-reload configurations
8. **Custom Type Converters**: Allow registration of custom type conversion functions

### Extension Points

1. **Custom Validators**: Plugin system for domain-specific validation
2. **Type Unmarshalers**: Custom YAML unmarshalers for complex types
3. **Error Handlers**: Configurable error handling strategies
4. **File Resolvers**: Custom file path resolution logic

## Cross-Platform Support

The module includes Python equivalents for cross-platform compatibility:

```
internal/yamlutil/
├── __init__.py
├── validator.py
├── error_types.py
```

This ensures that YAML processing can be performed consistently across different environments and tools in the ARMOR ecosystem.

## Documentation Standards

All exported functions and types should include:
- Clear purpose description
- Parameter descriptions
- Return value explanations
- Usage examples
- Error condition descriptions

Package-level documentation should include:
- Module overview
- Design philosophy
- Usage patterns
- Integration examples
- Error handling guidelines

## Conclusion

The YAML Parser Module provides a robust, type-safe foundation for YAML processing in ARMOR. Its layered architecture, comprehensive error handling, and flexible API design make it suitable for both simple configuration parsing and complex debug file processing tasks.

The module's design prioritizes:
- **Safety**: Type checking and validation prevent runtime errors
- **Clarity**: Clear error messages and context aid debugging
- **Flexibility**: Multiple access patterns for different use cases
- **Maintainability**: Clear separation of concerns and comprehensive testing

This design document serves as both architectural documentation and a guide for future enhancements and integration work.