# YAML Parser Module Architecture

## Overview

The YAML parser utility module (`internal/yamlutil`) provides comprehensive YAML handling capabilities for ARMOR debug files and configuration management. The module is designed with three-layer architecture focusing on robust file I/O, flexible parsing, and detailed error reporting.

## Design Principles

1. **Safety First**: All operations return detailed error information with context
2. **Type Safety**: Provide both generic and type-safe access patterns
3. **Debugging Support**: Comprehensive error messages with line/column information
4. **Progressive Enhancement**: Support simple use cases while enabling advanced scenarios
5. **Performance**: Efficient file I/O with minimal allocations

## Architecture Layers

### Layer 1: File I/O Foundation (file.go)

**Purpose**: Provide safe, contextualized file operations with detailed error reporting.

**Key Components**:
- `FileError`: Structured error type with operation context
- `ReadFile()`: Safe file reading with error wrapping
- `FileExists()`: Reliable file existence checking
- Error classification utilities

**Design Rationale**: 
- Wraps OS errors with operation context for better debugging
- Distinguishes between "not found", "permission denied", and other I/O errors
- Provides absolute path resolution for consistent error messages

### Layer 2: Core Parsing (parser.go)

**Purpose**: Transform YAML content into Go data structures with comprehensive error handling.

**Key Components**:
- `Parser`: Main parsing abstraction with strict/non-strict modes
- `ParseResult`: Structured result type with success/failure information
- `YAMLParseError`: Detailed syntax errors with line/column information
- `ParseYAML()`: Simplified parsing function for common use cases

**Design Patterns**:
- **Builder Pattern**: `NewParser()` and `NewStrictParser()` constructors
- **Result Pattern**: `ParseResult` struct encapsulates success/failure states
- **Error Chain**: `YAMLParseError` wraps underlying parser errors

### Layer 3: Validation & Analysis (validator.go)

**Purpose**: Validate YAML syntax and structure with detailed error reporting.

**Key Components**:
- `Validator`: Validation engine with configurable strictness
- `ValidationResult`: Comprehensive validation results with errors/warnings
- `ValidationError`: Detailed error with categorization
- Structural issue detection (duplicate keys, etc.)

**Design Rationale**:
- Separates validation from parsing for focused error reporting
- Categorizes errors by type (syntax, structure, I/O, etc.)
- Provides both errors and warnings for nuanced feedback

### Layer 4: Field Access & Debug Helpers (debug_helpers.go)

**Purpose**: Provide convenient, type-safe access to nested YAML fields.

**Key Components**:
- Field access functions: `GetField()`, `GetString()`, `GetInt()`, `GetBool()`
- Required field functions: `GetRequiredString()`, `GetRequiredInt()`, etc.
- Field validation: `ValidateRequiredFields()`, `ValidateFieldRequirements()`
- Error types: `FieldNotFoundError`, `TypeMismatchError`

**Navigation Pattern**:
```go
// Dot-notation navigation: "server.port" → data["server"]["port"]
parts := strings.Split(path, ".")
for _, part := range parts {
    currentMap, ok := current.(map[string]interface{})
    if !ok { return nil, false }
    current, ok = currentMap[part]
}
```

## Component Relationships

```
┌─────────────────────────────────────────────────────────┐
│                    Application Code                      │
│  (config loading, debug file processing, validation)    │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│              Field Access Layer (debug_helpers.go)       │
│  Type-safe field access, required field validation       │
└─────────────────────────────────────────────────────────┘
                           │
           ┌───────────────┴───────────────┐
           ▼                               ▼
┌──────────────────────┐      ┌──────────────────────┐
│ Validation Layer     │      │ Parsing Layer        │
│ (validator.go)       │      │ (parser.go)           │
│ Syntax/structure     │      │ YAML → Go structs     │
└──────────────────────┘      └──────────────────────┘
           │                               │
           └───────────────┬───────────────┘
                           ▼
┌─────────────────────────────────────────────────────────┐
│              File I/O Layer (file.go)                    │
│  Safe file operations with error context                │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│              Operating System / Filesystem               │
└─────────────────────────────────────────────────────────┘
```

## Error Handling Strategy

### Error Type Hierarchy

```
error
├── FileError (file.go)
│   ├── Operation: "read", "resolve"
│   └── Underlying: OS errors (not found, permission, etc.)
├── YAMLParseError (parser.go)
│   ├── Line/Column information
│   └── Raw parser error
├── ValidationError (validator.go)
│   ├── Type: syntax, structure, io, empty
│   └── Context information
└── Field Access Errors (debug_helpers.go)
    ├── FieldNotFoundError
    └── TypeMismatchError
```

### Error Propagation Pattern

```go
// Layer 1: File I/O wraps OS errors
return &FileError{
    Op:   "read",
    Path: absPath,
    Err:  wrapFileError(err),
}

// Layer 2: Parser wraps parse errors
return &YAMLParseError{
    FilePath: filePath,
    Message:  err.Error(),
    RawError: err,
    Line:     extractErrorLine(err),
}

// Layer 3: Validator categorizes errors
return ValidationError{
    Type:     categorizeError(errMsg),
    Line:     extractLineNumber(err),
    Message:  err.Error(),
}
```

## Memory Management

### Data Flow

1. **File Reading**: `ReadFile()` loads entire file into memory
2. **Parsing**: `yaml.Unmarshal()` parses into `map[string]interface{}`
3. **Field Access**: Navigation creates minimal temporary allocations
4. **Validation**: Uses yaml.Node for efficient structure analysis

### Optimization Considerations

- **Lazy Parsing**: Parse only when needed, not on file discovery
- **Reuse**: Parser instances can be reused for multiple files
- **Streaming**: Not currently supported (design limitation)
- **Caching**: Not implemented (future enhancement consideration)

## Thread Safety

### Current Design
- **Not Thread-Safe**: Parser and Validator instances maintain state
- **Shared Data**: Parsed `map[string]interface{}` is not safe for concurrent writes
- **Recommended Pattern**: Create separate instances per goroutine

### Future Enhancements
- Thread-safe parsers for concurrent file processing
- Synchronized field access for shared configuration structures

## Usage Patterns

### Pattern 1: Simple Configuration Loading

```go
// Parse into generic map
data, err := yamlutil.ParseYAML("config.yaml")
if err != nil {
    log.Fatal(err)
}
port := yamlutil.GetInt(data, "server.port", 8080)
```

### Pattern 2: Typed Configuration Structs

```go
parser := yamlutil.NewParser()
var config ConfigStruct
result := parser.ParseFile("config.yaml", &config)
if !result.Success {
    log.Fatal(result.Error)
}
```

### Pattern 3: Validation-First Approach

```go
validator := yamlutil.NewValidator()
result := validator.ValidateFile("config.yaml")
if result.HasErrors() {
    fmt.Println(result.ErrorSummary())
    return
}
```

### Pattern 4: Debug File Analysis

```go
data, err := yamlutil.ParseYAML("debug.yaml")
required := []string{"timestamp", "level", "message"}
missing := yamlutil.ValidateRequiredFields(data, required)
if len(missing) > 0 {
    log.Printf("Missing required fields: %v", missing)
}
```

## Extension Points

### Future Enhancements

1. **Schema Validation**: JSON Schema or custom schema support
2. **YAML Writing**: Serialize Go structs back to YAML
3. **Stream Processing**: Handle large files incrementally  
4. **Caching Layer**: Memoize frequently parsed files
5. **Field Mutation**: Safe field setting and deletion
6. **Configuration Merging**: Combine multiple YAML sources
7. **Environment Variable Expansion**: `${VAR}` substitution
8. **Include Directives**: Import other YAML files

### Plugin Architecture

```go
// Future: Extensible parser with middleware
type ParserMiddleware func(ParseContext) ParseContext

parser := NewParser()
parser.Use(ExpandEnvVars())
parser.Use(ResolveIncludes())
parser.Use(ApplyDefaults())
```

## Testing Strategy

### Coverage Areas

1. **Unit Tests**: Each exported function has dedicated tests
2. **Error Paths**: Comprehensive error condition testing
3. **Integration Tests**: End-to-end parsing and validation
4. **Edge Cases**: Empty files, malformed YAML, boundary conditions

### Test File Organization

- `parser_test.go`: Core parsing functionality
- `validator_test.go`: Validation logic
- `file_test.go`: File I/O operations
- `debug_helpers_test.go`: Field access and validation

## Performance Characteristics

### Time Complexity

- **File Reading**: O(n) where n = file size
- **Parsing**: O(n) where n = YAML content length
- **Field Access**: O(d) where d = path depth (typically 1-3)
- **Validation**: O(n) for full document analysis

### Space Complexity

- **Memory Usage**: ~2x file size (raw content + parsed structure)
- **Field Access**: O(1) additional memory per operation
- **Validation**: O(d) where d = document depth for recursive checks

## Dependencies

### External Dependencies

- `gopkg.in/yaml.v3`: YAML parsing and encoding

### Internal Dependencies

- Standard library: `fmt`, `os`, `path/filepath`, `strings`
- No ARMOR-internal dependencies (self-contained module)

## Migration Path

### For Existing Code

1. **Identify**: Find ad-hoc YAML parsing in codebase
2. **Replace**: Use `yamlutil.ParseYAML()` instead of direct `yaml.Unmarshal()`
3. **Enhance**: Add validation with `Validator`
4. **Type-Safe**: Migrate to typed field access functions

### Backward Compatibility

- Existing YAML files work without modification
- Error handling enhancement (more detailed errors)
- Performance neutral or improved

## Security Considerations

### Input Validation

- **File Size**: Consider limits for extremely large files
- **Path Traversal**: `filepath.Abs()` prevents directory traversal
- **Content Sanitization**: Parser doesn't execute code (safe by design)

### Error Information

- **Path Disclosure**: Error messages include file paths (acceptable for internal tool)
- **Memory Limits**: No DoS protection for malicious YAML (future enhancement)

## Monitoring and Observability

### Metrics to Track

- Parse success/failure rates
- Validation error patterns
- File access latency
- Memory usage for large files

### Logging Integration

```go
// Future: Structured logging support
parser.SetLogger(logger)
result := parser.ParseFile("config.yaml", &config)
// Logs: "yaml_parse", file: "config.yaml", success: true, duration_ms: 15
```

## Conclusion

The YAML parser module provides a robust, three-layer architecture for YAML handling in ARMOR. The design prioritizes safety, type safety, and debugging support while maintaining performance and extensibility for future enhancements.

The module follows Go best practices with clear error handling, comprehensive testing, and logical separation of concerns across file I/O, parsing, validation, and field access layers.