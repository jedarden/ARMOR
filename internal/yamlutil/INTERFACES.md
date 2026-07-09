# YAML Parser Module - Core Interfaces and Data Structures

## Overview

This document defines the core interfaces, types, and error handling strategy for the YAML parser module, providing a comprehensive foundation for YAML processing in ARMOR.

## Core Result Types

### ParseResult

Primary result type for parsing operations, providing structured success/failure reporting.

```go
type ParseResult struct {
    // FilePath is the path to the parsed file
    FilePath string

    // Data contains the parsed YAML data (usually map[string]interface{} or []interface{})
    Data interface{}

    // Success indicates whether parsing completed successfully
    Success bool

    // Error contains any error that occurred during parsing
    Error error

    // ParseDuration is the time taken to parse the file (optional, for metrics)
    ParseDuration time.Duration

    // Metrics contains additional parsing metrics (optional)
    Metrics *ParseMetrics
}
```

**Key Methods:**
- `IsFailure() bool` - Returns true if parse operation failed
- `IsSuccess() bool` - Returns true if parse operation succeeded
- `GetDetailedError() *DetailedParseError` - Extracts detailed error information

### ParseMetrics

Detailed metrics about parsing operations for performance monitoring.

```go
type ParseMetrics struct {
    ByteCount       int    // Size of parsed file in bytes
    LineCount       int    // Number of lines in YAML
    MaxNestingDepth int    // Maximum nesting depth
    KeyCount        int    // Total number of keys
    HasDocumentStart bool  // Has document start marker (---)
    UnknownFields   []string // Fields unknown during strict parsing
}
```

## Core Parser Interface

### YAMLParser

The primary interface for YAML parsing operations, supporting multiple parsing strategies.

```go
type YAMLParser interface {
    // ParseFile reads and parses a YAML file into the provided data structure
    // The data parameter must be a pointer to the target structure
    ParseFile(filePath string, data interface{}) ParseResult

    // ParseFileToMap reads and parses a YAML file into a generic map structure
    // Useful when YAML structure is unknown or dynamic
    ParseFileToMap(filePath string) ParseResult

    // ParseString parses YAML content from a string
    ParseString(yamlContent string, data interface{}) error

    // MustParseFile reads and parses a YAML file, panicking on error
    // Useful for initialization code where YAML files are critical
    MustParseFile(filePath string, data interface{})
}
```

**Implementation:**
- `Parser` struct provides the default implementation
- `NewParser()` creates a lenient parser
- `NewStrictParser()` creates a strict parser that rejects unknown fields

## Configuration Options

### ParserConfig

Comprehensive configuration for parser behavior and performance tuning.

```go
type ParserConfig struct {
    // Strict parsing mode
    StrictMode bool // Reject unknown fields and enforce strict YAML rules

    // Error handling
    VerboseErrors     bool // Include detailed context in error messages
    IncludeLineInfo   bool // Always include line/column information
    ErrorContextLines int  // Number of context lines in errors

    // Caching
    EnableCaching bool           // Cache parsed YAML documents
    CacheTTL      time.Duration // How long to keep cached documents
    MaxCacheSize  int            // Maximum documents to cache (0 = unlimited)

    // Performance
    EnableStreaming  bool   // Enable streaming for large files (experimental)
    StreamBufferSize int    // Buffer size for streaming (bytes)
    MaxFileSize     int64  // Maximum file size (bytes, 0 = unlimited)

    // Validation integration
    ValidateAfterParse bool             // Automatically validate after parsing
    ValidatorConfig   *ValidatorConfig // Validator config for auto-validation

    // Type handling
    ExplicitTypeTags  bool // Require explicit YAML type tags (!!str, !!int, etc.)
    CoerceTypes       bool // Automatically coerce between compatible types
    DefaultZeroValues bool // Use zero values for missing fields

    // Document handling
    MultiDocument     bool   // Support multi-document YAML files
    DocumentSeparator string // Separator between documents (default: "---")

    // Custom options
    CustomResolvers []TypeResolver  // Custom type resolution functions
    PostProcessors  []PostProcessor // Functions to run after parsing
}
```

**Predefined Configurations:**
- `DefaultParserConfig()` - Sensible defaults for development
- `StrictParserConfig()` - Strict parsing for production
- `PerformanceParserConfig()` - Optimized for throughput

### ValidatorConfig

Configuration for validation behavior and constraint enforcement.

```go
type ValidatorConfig struct {
    // Strict validation mode
    StrictMode       bool // Enforce strict YAML validation
    RequireAllFields bool // Require all schema fields
    RejectUnknownKeys bool // Reject keys not in schema

    // Error reporting
    VerboseErrors      bool // Detailed error context
    MaxErrors         int   // Maximum errors to collect (0 = unlimited)
    StopAtFirstError  bool  // Stop at first error
    WarningThreshold  int   // Warnings before treating as error

    // Schema validation
    EnableSchemaValidation bool   // Enable schema-based validation
    SchemaPaths            []string // Schema file paths
    SchemaValidationMode   SchemaMode // Schema strictness level

    // Constraint validation
    EnableConstraints bool          // Enable constraint checking
    ConstraintMode   ConstraintMode // Constraint enforcement level

    // Structural validation
    CheckDuplicateKeys    bool // Check for duplicate keys
    CheckCircularRefs     bool // Check for circular references
    CheckDeprecatedSyntax bool // Warn about deprecated YAML

    // Type validation
    ValidateTypes   bool // Validate field types
    ValidateRanges  bool // Validate numeric ranges
    ValidatePatterns bool // Validate string patterns
    ValidateLengths  bool // Validate string/array lengths

    // Custom validation
    CustomValidators  []FieldValidator  // Custom field validators
    SchemaValidators  []SchemaValidator  // Custom schema validators

    // Performance
    EnableValidationCache bool // Cache validation results
    CacheInvalidFiles     bool // Even cache failed validations
}
```

## Error Handling Strategy

### Result Pattern

The module uses a Result<T, Error> pattern for all operations:

```go
// Success case
result := ParseResult{
    FilePath: "config.yaml",
    Success:  true,
    Data:     parsedData,
}

// Failure case  
result := ParseResult{
    FilePath: "config.yaml",
    Success:  false,
    Error:    &ParseError{...},
}
```

### Error Hierarchy

Comprehensive error type hierarchy for precise error handling:

```
YAMLError (base interface)
├── FileError (file I/O errors)
├── ParseError (YAML parsing errors)
│   ├── SyntaxError (YAML syntax errors)
│   ├── StructureError (YAML structure errors)
│   └── TypeMismatchError (type conversion errors)
├── ValidationError (validation errors)
│   ├── FieldNotFoundError (missing required fields)
│   ├── ConstraintError (constraint violations)
│   └── DuplicateKeyError (duplicate key errors)
└── SchemaError (schema-related errors)
    ├── SchemaLoadError (schema loading errors)
    └── SchemaValidationError (schema validation errors)
```

### Detailed Error Information

Errors include comprehensive context for debugging:

```go
type ParseError struct {
    FilePath string      // Path to file being parsed
    Line     int         // Line number (1-indexed)
    Column   int         // Column number (1-indexed)
    Message  string      // Human-readable error message
    Context  string      // Additional parsing state context
    Err      error       // Underlying error for wrapping
    ErrorType ErrorType  // Specific error category
}
```

### Error Handling Patterns

**Basic error handling:**
```go
result := parser.ParseFile("config.yaml", &config)
if result.IsFailure() {
    if detailedErr := result.GetDetailedError(); detailedErr != nil {
        log.Printf("Parse error at line %d: %s", detailedErr.Line, detailedErr.Message)
    }
    return result.Error
}
```

**Type-specific error handling:**
```go
if parseErr, ok := result.Error.(*ParseError); ok {
    log.Printf("Parse error: %s at line %d", parseErr.Message, parseErr.Line)
}
```

**Error classification:**
```go
switch GetYAMLErrorType(err) {
case ErrorTypeFile:
    // Handle file I/O errors
case ErrorTypeSyntax:
    // Handle YAML syntax errors
case ErrorTypeValidation:
    // Handle validation errors
}
```

## Supporting Interfaces

### FieldAccessor

Interface for type-safe field access in parsed YAML data.

```go
type FieldAccessor interface {
    // Get field with default value
    GetField(data map[string]interface{}, path string, defaultValue interface{}) interface{}
    GetString(data map[string]interface{}, path string, defaultValue string) string
    GetInt(data map[string]interface{}, path string, defaultValue int) int
    GetBool(data map[string]interface{}, path string, defaultValue bool) bool

    // Check field existence
    HasField(data map[string]interface{}, path string) bool

    // Get required fields (error if missing)
    GetRequiredField(data map[string]interface{}, path string) (interface{}, error)
    GetRequiredString(data map[string]interface{}, path string) (string, error)
    GetRequiredInt(data map[string]interface{}, path string) (int, error)

    // Batch validation
    ValidateRequiredFields(data map[string]interface{}, requiredFields []string) []string
}
```

### YAMLValidator

Interface for YAML validation operations.

```go
type YAMLValidator interface {
    ValidateFile(filePath string) ValidationResult
    ValidateString(yamlContent string) ValidationResult
    ValidateStringWithPath(yamlContent, filePath string) ValidationResult
    ValidateMultipleFiles(filePaths []string) []ValidationResult
}
```

### YAMLProcessor

Comprehensive interface combining all YAML operations.

```go
type YAMLProcessor interface {
    FileReader      // File operations
    YAMLParser      // Parsing operations
    YAMLValidator   // Validation operations
    FieldAccessor    // Field access
    FileDiscovery   // File discovery
}
```

## Usage Examples

### Basic Parsing
```go
parser := NewParser()
result := parser.ParseFileToMap("config.yaml")
if result.IsFailure() {
    return result.Error
}
data := result.Data.(map[string]interface{})
```

### Strict Parsing with Validation
```go
parser := NewStrictParser()
var config ConfigStruct
result := parser.ParseFile("config.yaml", &config)
if result.IsFailure() {
    if detailedErr := result.GetDetailedError(); detailedErr != nil {
        log.Printf("Error: %s", detailedErr.String())
    }
    return result.Error
}
```

### Custom Configuration
```go
config := DefaultParserConfig()
config.StrictMode = true
config.VerboseErrors = true
config.EnableCaching = true

parser := &Parser{config: config}
result := parser.ParseFile("config.yaml", &data)
```

### Error Handling
```go
result := parser.ParseFile("config.yaml", &data)
if result.IsFailure() {
    switch GetYAMLErrorType(result.Error) {
    case ErrorTypeSyntax:
        if syntaxErr, ok := result.Error.(*SyntaxError); ok {
            log.Printf("Syntax error at line %d: %s", syntaxErr.Line, syntaxErr.Message)
        }
    case ErrorTypeFile:
        log.Printf("File error: %v", result.Error)
    default:
        log.Printf("Parse error: %v", result.Error)
    }
    return result.Error
}
```

## Design Principles

1. **Type Safety**: Strong typing with clear interface definitions
2. **Error Visibility**: Comprehensive error information with context
3. **Flexibility**: Multiple configuration options for different use cases
4. **Extensibility**: Interface-based design for custom implementations
5. **Performance**: Configurable caching and streaming options
6. **Testability**: Clear interfaces enable easy mocking and testing

## Conclusion

This interface and data structure definition provides a robust foundation for YAML processing in ARMOR. The combination of result types, comprehensive interfaces, flexible configuration, and detailed error handling enables safe, efficient, and maintainable YAML operations across the codebase.