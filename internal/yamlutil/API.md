# YAML Parser API Documentation

## Table of Contents

1. [Overview](#overview)
2. [Core Interfaces](#core-interfaces)
3. [Parser Methods](#parser-methods)
4. [Result Types](#result-types)
5. [Error Types](#error-types)
6. [Configuration](#configuration)
7. [Usage Examples](#usage-examples)
8. [Best Practices](#best-practices)

---

## Overview

The YAML parser module provides comprehensive YAML processing capabilities with support for parsing, validation, field access, and error handling. The API is available in both Go and Python implementations with consistent interfaces and behavior.

### Design Principles

- **Type Safety**: Strong typing with clear interface definitions
- **Error Visibility**: Comprehensive error information with context
- **Flexibility**: Multiple configuration options for different use cases
- **Extensibility**: Interface-based design for custom implementations
- **Performance**: Configurable caching and streaming options
- **Testability**: Clear interfaces enable easy mocking and testing

### Supported Operations

- File I/O operations (read, write, discover)
- YAML parsing (files, strings, streams)
- YAML validation (syntax, structure, schema)
- Field access (get, check existence, validate)
- Error handling (typed errors, detailed context)

---

## Core Interfaces

### YAMLParser

Primary interface for YAML parsing operations.

```go
type YAMLParser interface {
    // ParseFile reads and parses a YAML file into a data structure
    ParseFile(filePath string, data interface{}) ParseResult
    
    // ParseFileToMap reads and parses a YAML file into a generic map
    ParseFileToMap(filePath string) ParseResult
    
    // ParseString parses YAML content from a string
    ParseString(yamlContent string, data interface{}) error
    
    // MustParseFile panics on error (for critical initialization)
    MustParseFile(filePath string, data interface{})
    
    // Config returns the parser's configuration
    Config() *ParserConfig
}
```

### YAMLValidator

Interface for YAML validation operations.

```go
type YAMLValidator interface {
    // ValidateFile validates a YAML file at the given path
    ValidateFile(filePath string) ValidationResult
    
    // ValidateString validates YAML content from a string
    ValidateString(yamlContent string) ValidationResult
    
    // ValidateStringWithPath validates content with a path for error reporting
    ValidateStringWithPath(yamlContent, filePath string) ValidationResult
    
    // ValidateMultipleFiles validates multiple files in batch
    ValidateMultipleFiles(filePaths []string) []ValidationResult
}
```

### FieldAccessor

Interface for accessing fields in parsed YAML data.

```go
type FieldAccessor interface {
    // Get field with default value
    GetField(data map[string]interface{}, path string, defaultValue interface{}) interface{}
    GetString(data map[string]interface{}, path string, defaultValue string) string
    GetInt(data map[string]interface{}, path string, defaultValue int) int
    GetBool(data map[string]interface{}, path string, defaultValue bool) bool
    
    // Check field existence
    HasField(data map[string]interface{}, path string) bool
    
    // Get required fields (returns error if missing)
    GetRequiredField(data map[string]interface{}, path string) (interface{}, error)
    GetRequiredString(data map[string]interface{}, path string) (string, error)
    GetRequiredInt(data map[string]interface{}, path string) (int, error)
    GetRequiredBool(data map[string]interface{}, path string) (bool, error)
    
    // Batch validation
    ValidateRequiredFields(data map[string]interface{}, requiredFields []string) []string
    ValidateFieldRequirements(data map[string]interface{}, requirements []FieldRequirement) []error
}
```

### FileReader

Interface for reading file contents.

```go
type FileReader interface {
    // Read reads the entire contents of a file
    Read(path string) ([]byte, error)
    
    // Exists checks if a file exists
    Exists(path string) bool
}
```

### FileDiscovery

Interface for discovering YAML files in filesystems.

```go
type FileDiscovery interface {
    // FindYAMLFiles finds YAML files in a directory (non-recursive)
    FindYAMLFiles(dirPath string) ([]string, error)
    
    // FindYAMLFilesRecursive finds YAML files recursively
    FindYAMLFilesRecursive(dirPath string) ([]string, error)
    
    // IsYAMLFile checks if a file has a YAML extension
    IsYAMLFile(filePath string) bool
}
```

### YAMLProcessor

Comprehensive interface combining all YAML operations.

```go
type YAMLProcessor interface {
    FileReader      // File operations
    YAMLParser      // Parsing operations
    YAMLValidator   // Validation operations
    FieldAccessor   // Field access
    FileDiscovery   // File discovery
}
```

---

## Parser Methods

### ParseFile

Parses a YAML file into a provided data structure.

```go
func (p *Parser) ParseFile(filePath string, data interface{}) ParseResult
```

**Parameters:**
- `filePath`: Path to the YAML file to parse
- `data`: Pointer to the target structure (must be a pointer)

**Returns:**
- `ParseResult`: Result containing success status, parsed data, and error information

**Usage:**
```go
parser := NewParser()
var config ConfigStruct
result := parser.ParseFile("config.yaml", &config)
if !result.Success {
    log.Fatal(result.Error)
}
```

**Error Conditions:**
- File not found (`ErrorTypeFile`)
- Permission denied (`ErrorTypeIO`)
- Invalid YAML syntax (`ErrorTypeSyntax`)
- Type mismatch (`ErrorTypeTypeMismatch`)
- Structure error (`ErrorTypeStructure`)

### ParseFileToMap

Parses a YAML file into a generic map structure.

```go
func (p *Parser) ParseFileToMap(filePath string) ParseResult
```

**Parameters:**
- `filePath`: Path to the YAML file to parse

**Returns:**
- `ParseResult`: Result with `Data` as `map[string]interface{}` on success

**Usage:**
```go
parser := NewParser()
result := parser.ParseFileToMap("config.yaml")
if result.Success {
    data := result.Data.(map[string]interface{})
    value := data["key"]
}
```

**Use Cases:**
- When YAML structure is unknown or dynamic
- For quick file inspection without defining types
- For generic YAML processing tools

### ParseString

Parses YAML content from a string.

```go
func (p *Parser) ParseString(yamlContent string, data interface{}) error
```

**Parameters:**
- `yamlContent`: YAML content as a string
- `data`: Pointer to the target structure (must be a pointer)

**Returns:**
- `error`: Error if parsing fails, nil on success

**Usage:**
```go
yamlContent := "name: test\nport: 8080"
var data map[string]interface{}
if err := parser.ParseString(yamlContent, &data); err != nil {
    log.Fatal(err)
}
```

### MustParseFile

Parses a YAML file, panicking on any error.

```go
func (p *Parser) MustParseFile(filePath string, data interface{})
```

**Parameters:**
- `filePath`: Path to the YAML file to parse
- `data`: Pointer to the target structure (must be a pointer)

**Panics:**
- With a descriptive message if parsing fails

**Usage:**
```go
var config Config
parser.MustParseFile("critical-config.yaml", &config)
// If we get here, parsing succeeded
```

**Use Cases:**
- Critical initialization code where failure should stop execution
- Test fixtures
- Command-line tools that cannot proceed without valid config

### Config

Returns the parser's configuration.

```go
func (p *Parser) Config() *ParserConfig
```

**Returns:**
- `*ParserConfig`: Pointer to the parser's configuration (read-only)

**Usage:**
```go
config := parser.Config()
fmt.Printf("Strict mode: %v\n", config.StrictMode)
```

---

## Result Types

### ParseResult

Primary result type for parsing operations.

```go
type ParseResult struct {
    FilePath      string         // Path to the parsed file
    Data          interface{}    // Parsed YAML data
    Success       bool           // Whether parsing completed successfully
    Error         error          // Error that occurred (if any)
    ParseDuration time.Duration // Time taken to parse (optional)
    Metrics       *ParseMetrics  // Additional metrics (optional)
}
```

**Methods:**

- `IsFailure() bool` - Returns true if parse operation failed
- `IsSuccess() bool` - Returns true if parse operation succeeded
- `GetDetailedError() *DetailedParseError` - Extracts detailed error information

**Usage:**
```go
result := parser.ParseFileToMap("config.yaml")
if result.IsFailure() {
    if detailedErr := result.GetDetailedError(); detailedErr != nil {
        log.Printf("Error at line %d: %s", detailedErr.Line, detailedErr.Message)
    }
}
```

### ParseMetrics

Detailed metrics about parsing operations.

```go
type ParseMetrics struct {
    ByteCount        int       // Size of file in bytes
    LineCount        int       // Number of lines
    MaxNestingDepth  int       // Maximum nesting depth found
    KeyCount         int       // Total number of keys
    HasDocumentStart bool      // Has document start marker (---)
    UnknownFields    []string  // Fields unknown during strict parsing
}
```

### ValidationResult

Result of YAML validation operations.

```go
type ValidationResult struct {
    FilePath             string            // Path to validated file
    Valid                bool              // Whether validation passed
    Errors               []ValidationError // Validation errors found
    Warnings             []ValidationError // Validation warnings found
    ValidationDuration   time.Duration     // Time taken to validate
    SchemaVersion        string            // Schema version used
    ValidationMode       string            // Mode used for validation
}
```

**Methods:**

- `HasErrors() bool` - Returns true if there are any validation errors
- `HasWarnings() bool` - Returns true if there are any warnings
- `ErrorCount() int` - Returns the number of validation errors
- `WarningCount() int` - Returns the number of warnings
- `ErrorSummary() string` - Returns formatted error summary
- `WarningSummary() string` - Returns formatted warning summary
- `FullSummary() string` - Returns complete summary

### SchemaValidationResult

Result of schema-based validation.

```go
type SchemaValidationResult struct {
    FilePath               string                // Path to validated file
    Valid                  bool                  // Whether validation passed
    Errors                 []SchemaValidationError // General errors
    Warnings               []SchemaValidationError // Warnings
    MissingRequiredFields  []string              // Missing required fields
    TypeMismatches         []FieldTypeError      // Type mismatch errors
    ConstraintViolations   []ConstraintViolation // Constraint violations
    SchemaInfo             *SchemaInfo           // Schema metadata
}
```

### ProcessingResult

Result of a complete YAML processing pipeline.

```go
type ProcessingResult struct {
    FilePath          string             // Path to processed file
    Success           bool               // Whether all stages completed
    ParseResult       ParseResult        // Parse stage result
    ValidationResult  *ValidationResult  // Validation stage result
    ProcessedData     interface{}        // Final processed data
    TotalDuration     time.Duration      // Total time for all stages
    StageResults      map[string]interface{} // Individual stage results
}
```

### FieldAccessResult

Result of field access operations.

```go
type FieldAccessResult struct {
    FieldPath string      // Dot-notation path to field
    Value     interface{} // Retrieved field value
    Exists    bool        // Whether field exists
    Type      string      // Type of field value
    Error     error       // Error that occurred
    IsNil     bool        // Whether field value is nil
}
```

**Methods:**

- `IsSuccess() bool` - Returns true if field access succeeded
- `IsMissing() bool` - Returns true if field does not exist

### BatchValidationResult

Result of validating multiple YAML files.

```go
type BatchValidationResult struct {
    Results        []ValidationResult // Individual results
    TotalFiles     int                // Total files validated
    ValidFiles     int                // Files that passed
    InvalidFiles   int                // Files that failed
    TotalErrors    int                // Total errors across all files
    TotalWarnings  int                // Total warnings across all files
    TotalDuration  time.Duration      // Total validation time
}
```

**Methods:**

- `HasErrors() bool` - Returns true if any file had errors
- `HasWarnings() bool` - Returns true if any file had warnings
- `SuccessRate() float64` - Returns percentage of files that passed
- `GetFailedFiles() []string` - Returns list of failed file paths
- `GetResultsByStatus() ([]ValidationResult, []ValidationResult)` - Groups results by status

---

## Error Types

### Error Hierarchy

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

### Error Types

#### ParseError

Base error type for YAML parsing errors.

```go
type ParseError struct {
    FilePath   string    // Path to file being parsed
    Line       int       // Line number (1-indexed)
    Column     int       // Column number (1-indexed)
    Message    string    // Human-readable error message
    ContextStr string    // Additional parsing context
    Err        error     // Underlying error
    ErrorType  ErrorType // Specific error category
}
```

**When it occurs:**
- General parsing failures
- YAML syntax violations
- Invalid structure

**Example:**
```
parse error in config.yaml at line 15: unexpected token
```

#### SyntaxError

YAML syntax errors during parsing.

```go
type SyntaxError struct {
    FilePath string // Path to file with syntax error
    Line     int    // Line number where error occurred
    Column   int    // Column number where error occurred
    Message  string // Description of syntax error
    Expected string // What was expected (if known)
    Found    string // What was actually found (if known)
    Err      error  // Underlying yaml parser error
}
```

**When it occurs:**
- Incorrect indentation
- Malformed key-value pairs
- Unclosed quotes
- Invalid flow-style collections

**Example:**
```
syntax error in config.yaml at line 8, column 5: indentation error
expected: consistent indentation, found: inconsistent spaces
```

#### StructureError

YAML structure errors.

```go
type StructureError struct {
    FilePath     string // Path to file with structure error
    Line         int    // Line number where error occurred
    Message      string // Description of structure error
    DuplicateKey string // Name of duplicate key (if applicable)
    Location     string // Nested path to error location
    Err          error  // Underlying error
}
```

**When it occurs:**
- Duplicate keys in mappings
- Invalid nesting
- Anchor/alias reference errors
- Document structure violations

**Example:**
```
structure error in config.yaml at line 12: duplicate key
location: server.database.config
duplicate key: host
```

#### TypeMismatchError

Type conversion errors during parsing.

```go
type TypeMismatchError struct {
    FilePath     string // Path to file with type error
    FieldPath    string // Dot-notation path to field
    ExpectedType string // Expected type description
    ActualType   string // Actual type found
    Value        string // Actual value that caused error
    Line         int    // Line number where error occurred
}
```

**When it occurs:**
- String cannot be parsed as integer
- Boolean field has non-boolean value
- Array field contains scalar value
- In strict mode, value doesn't match expected type

**Example:**
```
type mismatch in config.yaml at line 20, field server.port: expected int, got string
field: server.port, expected type: int, actual type: string, value: "8080"
```

#### FieldNotFoundError

Missing required field error.

```go
type FieldNotFoundError struct {
    FilePath  string // Path to file missing the field
    FieldPath string // Dot-notation path to missing field
    Line      int    // Line number where field should be (if known)
}
```

**When it occurs:**
- Accessing a required field that doesn't exist
- Field exists but is nil
- Path navigation fails

**Example:**
```
required field missing in config.yaml: database.connection_string
required field not found: database.connection_string
```

#### ConstraintError

Constraint validation failures.

```go
type ConstraintError struct {
    FilePath       string // Path to file with constraint error
    FieldPath      string // Dot-notation path to field
    ConstraintType string // Type of constraint (range, length, pattern, etc.)
    Constraint     string // Description of constraint
    Value          string // Actual value that violated constraint
    Line           int    // Line number where violation occurred
}
```

**When it occurs:**
- Numeric value out of range
- String too long or too short
- String doesn't match pattern
- Array length constraint violation

**Example:**
```
constraint violation in config.yaml at line 25, field server.port: value out of range
field: server.port, constraint: range 1-65535, value: 70000
```

#### DuplicateKeyError

Duplicate key errors in YAML mappings.

```go
type DuplicateKeyError struct {
    FilePath string // Path to file with duplicate keys
    Key      string // The duplicate key name
    Location string // Nested path to duplicate key
    Line1    int    // Line number of first occurrence
    Line2    int    // Line number of duplicate occurrence
}
```

**When it occurs:**
- Same key appears multiple times in a mapping
- Case-insensitive key collision
- Duplicate keys in nested structures

**Example:**
```
duplicate key error in config.yaml at line 18: key "name" already defined at line 12
duplicate key: name at server.config (first at line 12, duplicate at line 18)
```

#### SchemaLoadError

Schema loading errors.

```go
type SchemaLoadError struct {
    FilePath string // Path to schema file
    Message  string // Description of load error
    Err      error  // Underlying error
}
```

**When it occurs:**
- Schema file not found
- Invalid schema file format
- Schema parsing errors

#### SchemaValidationError

Schema validation failures.

```go
type SchemaValidationError struct {
    FilePath   string // Path to file being validated
    SchemaPath string // Path to schema file
    FieldPath  string // Dot-notation path to invalid field
    Message    string // Description of validation failure
    Expected   string // What was expected by schema
    Found      string // What was actually found
    Line       int    // Line number where validation failed
}
```

**When it occurs:**
- Data violates schema constraints
- Required field missing according to schema
- Type mismatch with schema definition
- Unknown field (in strict mode)

### Error Type Constants

```go
const (
    ErrorTypeFile           ErrorType = "file"
    ErrorTypeParse          ErrorType = "parse"
    ErrorTypeSyntax         ErrorType = "syntax"
    ErrorTypeStructure      ErrorType = "structure"
    ErrorTypeTypeMismatch   ErrorType = "type_mismatch"
    ErrorTypeValidation     ErrorType = "validation"
    ErrorTypeFieldNotFound  ErrorType = "field_not_found"
    ErrorTypeConstraint     ErrorType = "constraint"
    ErrorTypeDuplicateKey   ErrorType = "duplicate_key"
    ErrorTypeSchema         ErrorType = "schema"
    ErrorTypeSchemaLoad     ErrorType = "schema_load"
    ErrorTypeSchemaValidate ErrorType = "schema_validate"
    ErrorTypeUnknown        ErrorType = "unknown"
    ErrorTypeEmpty          ErrorType = "empty"
    ErrorTypeIO             ErrorType = "io"
)
```

### Error Helper Functions

```go
// IsYAMLError checks if an error is any type of YAMLError
func IsYAMLError(err error) bool

// GetYAMLErrorType returns the ErrorType of a YAMLError
func GetYAMLErrorType(err error) ErrorType

// IsFileNotFoundError checks if error indicates file not found
func IsFileNotFoundError(err error) bool

// IsPermissionError checks if error indicates permission issue
func IsPermissionError(err error) bool
```

---

## Configuration

### ParserConfig

Comprehensive configuration for parser behavior.

```go
type ParserConfig struct {
    // Strict parsing mode
    StrictMode bool // Reject unknown fields and enforce strict YAML rules

    // Error handling
    VerboseErrors     bool // Include detailed context in error messages
    IncludeLineInfo   bool // Always include line/column information in errors
    ErrorContextLines int  // Number of context lines to include in errors (0 = none)

    // Caching
    EnableCaching  bool           // Cache parsed YAML documents to avoid re-parsing
    CacheTTL       time.Duration // How long to keep cached documents
    MaxCacheSize   int            // Maximum number of documents to cache (0 = unlimited)

    // Performance
    EnableStreaming  bool   // Enable streaming for large files (experimental)
    StreamBufferSize int    // Buffer size for streaming (bytes)
    MaxFileSize     int64  // Maximum file size to accept (bytes, 0 = unlimited)

    // Validation integration
    ValidateAfterParse bool            // Automatically validate after parsing
    ValidatorConfig   *ValidatorConfig // Validator config to use for auto-validation

    // Type handling
    ExplicitTypeTags  bool // Require explicit YAML type tags (!!str, !!int, etc.)
    CoerceTypes       bool // Automatically coerce between compatible types
    DefaultZeroValues bool // Use zero values for missing fields instead of errors

    // Document handling
    MultiDocument     bool   // Support multi-document YAML files
    DocumentSeparator string // Separator between documents (default: "---")

    // Custom options
    CustomResolvers  []TypeResolver  // Custom type resolution functions
    PostProcessors   []PostProcessor // Functions to run after parsing
}
```

### Predefined Parser Configurations

#### DefaultParserConfig

Sensible defaults for development environments.

```go
func DefaultParserConfig() *ParserConfig
```

**Settings:**
- StrictMode: false
- VerboseErrors: true
- IncludeLineInfo: true
- ErrorContextLines: 2
- EnableCaching: false
- CacheTTL: 5 minutes
- MaxCacheSize: 100
- EnableStreaming: false
- MaxFileSize: 10MB
- ValidateAfterParse: false
- CoerceTypes: true
- DefaultZeroValues: true
- MultiDocument: false

**Use Cases:**
- Development environments
- Configuration file parsing
- General YAML processing

#### StrictParserConfig

Strict parsing for production environments.

```go
func StrictParserConfig() *ParserConfig
```

**Settings:**
- StrictMode: true
- VerboseErrors: true
- IncludeLineInfo: true
- ErrorContextLines: 3
- EnableCaching: true
- CacheTTL: 10 minutes
- MaxCacheSize: 500
- EnableStreaming: false
- MaxFileSize: 50MB
- ValidateAfterParse: true
- CoerceTypes: false
- DefaultZeroValues: false
- MultiDocument: true

**Use Cases:**
- Production configuration validation
- CI/CD pipelines
- Security-sensitive applications

#### PerformanceParserConfig

Optimized for high-throughput scenarios.

```go
func PerformanceParserConfig() *ParserConfig
```

**Settings:**
- StrictMode: false
- VerboseErrors: false
- IncludeLineInfo: false
- ErrorContextLines: 0
- EnableCaching: true
- CacheTTL: 30 minutes
- MaxCacheSize: 1000
- EnableStreaming: true
- MaxFileSize: 100MB
- ValidateAfterParse: false
- CoerceTypes: true
- DefaultZeroValues: true
- MultiDocument: true

**Use Cases:**
- High-volume processing
- Batch operations
- Large file handling

### ValidatorConfig

Configuration for validation behavior.

```go
type ValidatorConfig struct {
    // Strict validation mode
    StrictMode       bool // Enforce strict YAML validation rules
    RequireAllFields bool // Require all fields in schema to be present
    RejectUnknownKeys bool // Reject keys not defined in schema

    // Error reporting
    VerboseErrors     bool   // Include detailed context in validation errors
    MaxErrors         int    // Maximum number of errors to collect (0 = unlimited)
    StopAtFirstError  bool   // Stop validation at first error
    WarningThreshold  int    // Number of warnings before treating as error (0 = ignore)

    // Schema validation
    EnableSchemaValidation bool         // Enable schema-based validation
    SchemaPaths            []string      // Paths to schema definition files
    SchemaValidationMode   SchemaMode   // How strictly to apply schema rules

    // Constraint validation
    EnableConstraints bool          // Enable constraint checking (ranges, patterns, etc.)
    ConstraintMode   ConstraintMode // How strictly to enforce constraints

    // Structural validation
    CheckDuplicateKeys    bool // Check for duplicate keys in mappings
    CheckCircularRefs     bool // Check for circular references
    CheckDeprecatedSyntax bool // Warn about deprecated YAML features

    // Type validation
    ValidateTypes   bool // Validate field types against schema
    ValidateRanges  bool // Validate numeric ranges
    ValidatePatterns bool // Validate string patterns (regex)
    ValidateLengths  bool // Validate string/array lengths

    // Custom validation
    CustomValidators   []FieldValidator    // Custom field validation functions
    SchemaValidators   []SchemaValidatorFunc // Custom schema validation functions

    // Performance
    EnableValidationCache bool // Cache validation results
    CacheInvalidFiles     bool // Even cache files that fail validation
}
```

### SchemaMode

How strictly schema rules are applied.

```go
type SchemaMode int

const (
    SchemaModeDisabled SchemaMode = iota  // Disable schema validation
    SchemaModeLenient                     // Apply schema rules but allow unknown fields
    SchemaModeStrict                      // Apply schema rules and reject unknown fields
    SchemaModeRequired                    // Require all schema fields to be present
)
```

### ConstraintMode

How strictly constraints are enforced.

```go
type ConstraintMode int

const (
    ConstraintModeDisabled ConstraintMode = iota  // Disable constraint checking
    ConstraintModeWarn                           // Issue warnings for violations
    ConstraintModeError                          // Treat violations as errors
    ConstraintModeFatal                          // Treat violations as fatal errors
)
```

### Predefined Validator Configurations

#### DefaultValidatorConfig

Basic validation with detailed error reporting.

```go
func DefaultValidatorConfig() *ValidatorConfig
```

**Settings:**
- StrictMode: false
- VerboseErrors: true
- MaxErrors: 50
- StopAtFirstError: false
- WarningThreshold: 10
- EnableSchemaValidation: false
- SchemaValidationMode: SchemaModeLenient
- EnableConstraints: true
- ConstraintMode: ConstraintModeWarn
- CheckDuplicateKeys: true
- ValidateTypes: true

**Use Cases:**
- Development validation
- Basic syntax checking
- Configuration file validation

#### StrictValidatorConfig

Strict validation for production environments.

```go
func StrictValidatorConfig() *ValidatorConfig
```

**Settings:**
- StrictMode: true
- RequireAllFields: true
- RejectUnknownKeys: true
- VerboseErrors: true
- MaxErrors: 100
- StopAtFirstError: false
- WarningThreshold: 0
- EnableSchemaValidation: true
- SchemaValidationMode: SchemaModeStrict
- EnableConstraints: true
- ConstraintMode: ConstraintModeError
- CheckDuplicateKeys: true
- CheckCircularRefs: true
- CheckDeprecatedSyntax: true
- ValidateTypes: true
- ValidateRanges: true
- ValidatePatterns: true
- ValidateLengths: true

**Use Cases:**
- Production configuration validation
- CI/CD pipeline validation
- Security-critical applications

#### LenientValidatorConfig

Lenient validation for development.

```go
func LenientValidatorConfig() *ValidatorConfig
```

**Settings:**
- StrictMode: false
- RequireAllFields: false
- RejectUnknownKeys: false
- VerboseErrors: true
- MaxErrors: 25
- StopAtFirstError: false
- WarningThreshold: 20
- EnableSchemaValidation: false
- SchemaValidationMode: SchemaModeLenient
- EnableConstraints: true
- ConstraintMode: ConstraintModeWarn
- CheckDuplicateKeys: true

**Use Cases:**
- Development environments
- Testing
- Quick validation

---

## Usage Examples

### Basic Parsing

#### Parse into a Struct

```go
type Config struct {
    Name    string `yaml:"name"`
    Port    int    `yaml:"port"`
    Enabled bool   `yaml:"enabled"`
}

parser := NewParser()
var config Config
result := parser.ParseFile("config.yaml", &config)
if !result.Success {
    log.Fatal(result.Error)
}
fmt.Printf("Server: %s on port %d\n", config.Name, config.Port)
```

#### Parse into a Map

```go
parser := NewParser()
result := parser.ParseFileToMap("config.yaml")
if result.Success {
    data := result.Data.(map[string]interface{})
    fmt.Printf("Keys: %v\n", reflect.ValueOf(data).MapKeys())
}
```

#### Parse from String

```go
yamlContent := `
name: myapp
port: 8080
enabled: true
`

var data map[string]interface{}
if err := parser.ParseString(yamlContent, &data); err != nil {
    log.Fatal(err)
}
```

### Field Access

#### Get with Default Values

```go
data := parseYAML("config.yaml")

// Get string with default
name := GetString(data, "server.name", "localhost")

// Get int with default
port := GetInt(data, "server.port", 8080)

// Get bool with default
enabled := GetBool(data, "server.enabled", true)
```

#### Get Required Fields

```go
// Get required string (error if missing)
host, err := GetRequiredString(data, "server.host")
if err != nil {
    if fieldNotFound, ok := err.(*FieldNotFoundError); ok {
        log.Printf("Missing required field: %s", fieldNotFound.FieldPath)
    }
}

// Get required int
port, err := GetRequiredInt(data, "server.port")
if err != nil {
    log.Fatal("Port is required")
}
```

#### Validate Required Fields

```go
required := []string{
    "server.host",
    "server.port",
    "database.name",
    "database.connection_string",
}

missing := ValidateRequiredFields(data, required)
if len(missing) > 0 {
    log.Fatalf("Missing required fields: %v", missing)
}
```

### Error Handling

#### Comprehensive Error Handling

```go
result := parser.ParseFile("config.yaml", &config)
if !result.Success {
    switch GetYAMLErrorType(result.Error) {
    case ErrorTypeFile:
        if IsFileNotFoundError(result.Error) {
            log.Println("Config file not found, using defaults")
            return defaultConfig()
        }
        if IsPermissionError(result.Error) {
            log.Fatal("Permission denied reading config file")
        }
        
    case ErrorTypeSyntax:
        if syntaxErr, ok := result.Error.(*SyntaxError); ok {
            log.Printf("Syntax error at line %d, column %d: %s",
                syntaxErr.Line, syntaxErr.Column, syntaxErr.Message)
            log.Printf("Expected: %s, Found: %s", syntaxErr.Expected, syntaxErr.Found)
        }
        
    case ErrorTypeStructure:
        if structErr, ok := result.Error.(*StructureError); ok {
            log.Printf("Structure error at line %d: %s", structErr.Line, structErr.Message)
            if structErr.DuplicateKey != "" {
                log.Printf("Duplicate key: %s at %s", structErr.DuplicateKey, structErr.Location)
            }
        }
        
    case ErrorTypeTypeMismatch:
        if typeErr, ok := result.Error.(*TypeMismatchError); ok {
            log.Printf("Type mismatch at field %s: expected %s, got %s",
                typeErr.FieldPath, typeErr.ExpectedType, typeErr.ActualType)
        }
        
    default:
        log.Printf("Parse error: %v", result.Error)
    }
}
```

#### Safe Config Reading Function

```go
func readConfigSafely(path string) (map[string]interface{}, error) {
    if !FileExists(path) {
        return nil, fmt.Errorf("config file not found: %s", path)
    }

    data, err := ParseYAML(path)
    if err != nil {
        if yamlErr, ok := err.(*YAMLParseError); ok {
            return nil, fmt.Errorf("syntax error at line %d: %s",
                yamlErr.Line, yamlErr.Message)
        }
        return nil, err
    }

    // Validate critical fields
    required := []string{"server.host", "server.port"}
    missing := ValidateRequiredFields(data, required)
    if len(missing) > 0 {
        return nil, fmt.Errorf("missing required fields: %v", missing)
    }

    return data, nil
}
```

### Validation

#### Basic Validation

```go
validator := NewValidator()
result := validator.ValidateFile("config.yaml")

if result.HasErrors() {
    fmt.Println("Validation errors:")
    for _, err := range result.Errors {
        fmt.Printf("  Line %d: %s\n", err.Line, err.Message)
    }
}

if result.HasWarnings() {
    fmt.Println("Warnings:")
    for _, warn := range result.Warnings {
        fmt.Printf("  Line %d: %s\n", warn.Line, warn.Message)
    }
}
```

#### Batch Validation

```go
validator := NewValidator()
results := validator.ValidateMultipleFiles([]string{
    "config.yaml",
    "database.yaml",
    "logging.yaml",
})

for _, result := range results {
    if result.HasErrors() {
        fmt.Printf("%s: %d errors\n", result.FilePath, len(result.Errors))
    } else {
        fmt.Printf("%s: valid\n", result.FilePath)
    }
}
```

#### Strict Validation for Production

```go
strictValidator := NewStrictValidator()
result := strictValidator.ValidateFile("production-config.yaml")

if result.HasErrors() {
    log.Fatal("Production config validation failed")
    // Handle errors
}
fmt.Println("Production config is valid")
```

### Configuration

#### Custom Parser Configuration

```go
config := DefaultParserConfig()
config.StrictMode = true
config.VerboseErrors = true
config.EnableCaching = true
config.CacheTTL = 10 * time.Minute

parser := &Parser{config: config}
result := parser.ParseFile("config.yaml", &data)
```

#### Custom Validator Configuration

```go
config := DefaultValidatorConfig()
config.EnableSchemaValidation = true
config.SchemaPaths = []string{"schemas/config-schema.json"}
config.SchemaValidationMode = SchemaModeStrict
config.RejectUnknownKeys = true

validator := &Validator{config: config}
result := validator.ValidateFile("config.yaml")
```

### File Discovery

#### Find YAML Files in Directory

```go
// Non-recursive search
files, err := FindYAMLFiles("/etc/app")
if err != nil {
    log.Fatal(err)
}
for _, file := range files {
    fmt.Println("Found YAML:", file)
}

// Recursive search
allYAMLs, err := FindYAMLFilesRecursive("/etc/app")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Found %d YAML files\n", len(allYAMLs))
```

### ARMOR Debug File Processing

#### Parse ARMOR Debug File

```go
debugData, err := ParseYAML("armor-debug.yaml")
if err != nil {
    log.Fatal("Failed to parse debug file:", err)
}

sessionId := GetString(debugData, "session.id", "unknown")
timestamp := GetString(debugData, "session.timestamp", "")
fmt.Printf("Debug session: %s at %s\n", sessionId, timestamp)
```

#### Process Nested Debug Configuration

```go
debugLevel := GetString(debugData, "debug.level", "info")
logPath := GetString(debugData, "debug.log_path", "/var/log/armor.log")
maxSize := GetInt(debugData, "debug.max_size_mb", 100)

if GetBool(debugData, "debug.enabled", false) {
    setupDebugLogging(logPath, debugLevel, maxSize)
}
```

#### Process Debug Components

```go
required := []string{
    "session.id",
    "session.timestamp",
    "debug.level",
}
missing := ValidateRequiredFields(debugData, required)
if len(missing) > 0 {
    log.Fatal("Invalid debug file - missing required fields:", missing)
}

components := []string{"database", "network", "parser"}
for _, component := range components {
    enabled := GetBool(debugData,
        fmt.Sprintf("components.%s.enabled", component), false)
    level := GetString(debugData,
        fmt.Sprintf("components.%s.level", component), "info")
    fmt.Printf("%s debug: enabled=%v, level=%s\n", component, enabled, level)
}
```

#### Process Multiple Debug Files

```go
debugFiles, err := FindYAMLFilesRecursive("/var/log/armor/debug")
if err != nil {
    log.Fatal("Failed to find debug files:", err)
}

for _, debugFile := range debugFiles {
    data, err := ParseYAML(debugFile)
    if err != nil {
        log.Printf("Warning: failed to parse %s: %v", debugFile, err)
        continue
    }

    sessionId := GetString(data, "session.id", "unknown")
    fmt.Printf("Processing debug session: %s from %s\n", sessionId, debugFile)
}
```

---

## Best Practices

### Error Handling

1. **Always check result status**
   ```go
   result := parser.ParseFile("config.yaml", &data)
   if !result.Success {
       // Handle error
   }
   ```

2. **Use type-specific error handling**
   ```go
   if syntaxErr, ok := err.(*SyntaxError); ok {
       // Handle syntax error with line information
   }
   ```

3. **Provide meaningful defaults**
   ```go
   port := GetInt(data, "server.port", 8080)
   ```

4. **Validate required fields early**
   ```go
   missing := ValidateRequiredFields(data, required)
   if len(missing) > 0 {
       return fmt.Errorf("missing required fields: %v", missing)
   }
   ```

### Configuration

1. **Use appropriate configurations for environment**
   - Development: `DefaultParserConfig()`
   - Production: `StrictParserConfig()`
   - High-throughput: `PerformanceParserConfig()`

2. **Enable caching for frequently accessed files**
   ```go
   config.EnableCaching = true
   config.CacheTTL = 10 * time.Minute
   ```

3. **Use strict mode in production**
   ```go
   config.StrictMode = true
   config.RejectUnknownKeys = true
   ```

### Performance

1. **Use ParseFileToMap for unknown structures**
   ```go
   result := parser.ParseFileToMap("config.yaml")
   ```

2. **Enable streaming for large files**
   ```go
   config.EnableStreaming = true
   config.MaxFileSize = 100 * 1024 * 1024 // 100MB
   ```

3. **Batch validate multiple files**
   ```go
   results := validator.ValidateMultipleFiles(filePaths)
   ```

### Security

1. **Always validate untrusted YAML**
   ```go
   validator := NewStrictValidator()
   result := validator.ValidateFile("untrusted.yaml")
   ```

2. **Use strict mode for security-sensitive applications**
   ```go
   parser := NewStrictParser()
   config.StrictMode = true
   config.RejectUnknownKeys = true
   ```

3. **Check file permissions before reading**
   ```go
   if !FileExists(path) {
       return fmt.Errorf("file not found: %s", path)
   }
   ```

### Testing

1. **Use MustParseFile for test fixtures**
   ```go
   parser.MustParseFile("testdata/valid-config.yaml", &config)
   ```

2. **Test error handling with different error types**
   ```go
   // Test syntax error handling
   _, err := ParseYAML("testdata/syntax-error.yaml")
   if syntaxErr, ok := err.(*SyntaxError); ok {
       assert.Equal(t, 5, syntaxErr.Line)
   }
   ```

3. **Validate with strict configuration in tests**
   ```go
   validator := NewStrictValidator()
   result := validator.ValidateFile("config.yaml")
   assert.False(t, result.HasErrors())
   ```
