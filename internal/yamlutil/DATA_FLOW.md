# YAML Parser Data Flow

This document describes the complete data flow through the YAML parser module, from file input to processed output.

## Overview

The YAML parser module processes YAML files through a multi-stage pipeline:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         YAML Processing Pipeline                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  File Input → File Read → Parse → Validate → Field Access → Output        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Processing Stages

### Stage 1: File Read (`StageFileRead`)

**Input:** File path (string)

**Process:**
1. Resolve absolute path
2. Check file existence
3. Read file contents into memory
4. Handle file I/O errors

**Output:** Raw bytes ([]byte) or `FileError`

**Error Handling:**
- Returns `FileError` with operation context ("read", "resolve")
- Distinguishes between "not found" and "permission denied"
- Provides absolute path in error messages

**Example:**
```go
content, err := yamlutil.ReadFile("config.yaml")
if err != nil {
    if yamlutil.IsFileNotFoundError(err) {
        // Handle missing file
    }
    if yamlutil.IsPermissionError(err) {
        // Handle permission issues
    }
}
```

**Metrics Collected:**
- Byte count
- Read duration
- File size validation (against `MaxFileSize` config)

### Stage 2: Parse (`StageParse`)

**Input:** Raw bytes ([]byte)

**Process:**
1. Check for empty/whitespace-only content
2. Parse YAML using `gopkg.in/yaml.v3`
3. Unmarshal into target structure (generic map or typed struct)
4. Handle strict mode (reject unknown fields if enabled)
5. Extract line/column information from parse errors

**Output:** `ParseResult` or `ParseError`

**Configuration Impact:**
- `ParserConfig.Strict`: Reject unknown fields in struct unmarshaling
- `ParserConfig.AllowDuplicateKeys`: Fail on duplicate mapping keys
- `ParserConfig.EmptyFileAsError`: Treat empty files as errors
- `ParserConfig.MaxFileSize`: Reject files exceeding size limit
- `ParserConfig.Encoding`: Specify character encoding

**Error Handling:**
- Returns `ParseError` with line/column information
- Categorizes errors by type (syntax, structure, duplicate keys)
- Provides contextual information (problematic line content)

**Example:**
```go
parser := yamlutil.NewParser()
result := parser.ParseFile("config.yaml", &config)
if result.IsFailure() {
    if parseErr := result.GetDetailedError(); parseErr != nil {
        fmt.Printf("Parse error at line %d: %s\n", parseErr.Line, parseErr.Message)
    }
}
```

**Metrics Collected:**
- Parse duration
- Line count
- Maximum nesting depth
- Key count
- Unknown fields (in strict mode)
- Document start marker presence

### Stage 3: Validate (`StageValidate`)

**Input:** Parsed YAML data (map[string]interface{} or typed struct)

**Process:**
1. Syntax validation (already done in Parse stage)
2. Structural validation (duplicate keys, nesting consistency)
3. Schema validation (if schema provided)
4. Constraint validation (field requirements, type checks)

**Output:** `ValidationResult` or `ValidationError`

**Configuration Impact:**
- `ValidatorConfig.Strict`: Treat warnings as errors
- `ValidatorConfig.CheckDuplicateKeys`: Detect duplicate mapping keys
- `ValidatorConfig.CheckIndentation`: Validate indentation consistency
- `ValidatorConfig.MaxLineLength`: Warn on long lines
- `ValidatorConfig.RequireDocumentStart`: Require `---` marker
- `ValidatorConfig.IgnoreComments`: Strip comments before validation

**Validation Checks:**
- **Syntax:** YAML syntax correctness (done in Parse stage)
- **Structure:** Duplicate keys, invalid nesting, mapping issues
- **Schema:** Field presence, type constraints, value constraints
- **Custom:** User-defined validation rules

**Error Handling:**
- Returns `ValidationResult` with separate errors and warnings
- Each error includes field path, constraint type, actual vs expected values
- Supports multiple validation failures (returns all errors)

**Example:**
```go
validator := yamlutil.NewValidator()
result := validator.ValidateFile("config.yaml")

if result.HasErrors() {
    for _, err := range result.Errors {
        fmt.Printf("Error at line %d: field %s - %s\n", 
            err.Line, err.FieldPath, err.Message)
    }
}

if result.HasWarnings() {
    for _, warn := range result.Warnings {
        fmt.Printf("Warning: %s\n", warn.Message)
    }
}
```

**Metrics Collected:**
- Validation duration
- Number of checks performed
- Error/warning counts
- Schema version (if applicable)

### Stage 4: Field Access (`StageFieldAccess`)

**Input:** Validated YAML data (map[string]interface{})

**Process:**
1. Navigate nested fields using dot notation
2. Type-safe field extraction with defaults
3. Required field validation
4. Type checking and conversion
5. Custom constraint validation

**Output:** Typed field values or `FieldNotFoundError`/`TypeMismatchError`

**Field Access Methods:**
- `GetField(data, path, defaultValue)` - Get any field with default
- `GetString(data, path, defaultValue)` - Get string field with default
- `GetInt(data, path, defaultValue)` - Get integer field with default
- `GetBool(data, path, defaultValue)` - Get boolean field with default
- `HasField(data, path)` - Check field existence
- `GetRequiredField(data, path)` - Get required field or error
- `GetRequiredString(data, path)` - Get required string or error
- `GetRequiredInt(data, path)` - Get required integer or error
- `GetRequiredBool(data, path)` - Get required boolean or error

**Dot Notation Examples:**
```go
// Simple field
GetString(data, "server.host", "localhost")

// Nested field
GetInt(data, "server.port", 8080)

// Deeply nested
GetBool(data, "database.connection.ssl.enabled", false)
```

**Error Handling:**
- `FieldNotFoundError`: Required field is missing
- `TypeMismatchError`: Field has wrong type
- Returns default value for optional missing fields

**Example:**
```go
// Get fields with defaults
host := yamlutil.GetString(data, "server.host", "localhost")
port := yamlutil.GetInt(data, "server.port", 8080)

// Get required fields (error if missing)
dbUrl, err := yamlutil.GetRequiredString(data, "database.url")
if err != nil {
    if fieldNotFound, ok := err.(*yamlutil.FieldNotFoundError); ok {
        log.Fatalf("Missing required field: %s", fieldNotFound.FieldPath)
    }
}

// Validate multiple required fields
required := []string{"server.host", "server.port", "database.url"}
missing := yamlutil.ValidateRequiredFields(data, required)
if len(missing) > 0 {
    log.Fatalf("Missing required fields: %v", missing)
}
```

### Stage 5: Complete (`StageComplete`)

**Input:** Processed and validated data

**Process:**
1. Aggregate all pipeline metrics
2. Calculate total processing duration
3. Return final processed data
4. Provide pipeline summary

**Output:** `ProcessingPipeline` with final results

**Metrics Summary:**
```go
pipeline := ProcessingPipeline{
    Success: true,
    InputFile: "config.yaml",
    OutputData: processedData,
    TotalDuration: 150 * time.Millisecond,
    Metrics: []PipelineMetric{
        {Stage: StageFileRead, Duration: 10*time.Millisecond, Success: true},
        {Stage: StageParse, Duration: 100*time.Millisecond, Success: true},
        {Stage: StageValidate, Duration: 30*time.Millisecond, Success: true},
        {Stage: StageFieldAccess, Duration: 10*time.Millisecond, Success: true},
    },
}
```

## Error Propagation

Errors propagate through the pipeline with context preservation:

```
FileError (Stage 1) → Stop processing, return file error
        ↓
ParseError (Stage 2) → Skip validation, return parse error
        ↓
ValidationError (Stage 3) → Skip field access, return validation errors
        ↓
FieldAccessError (Stage 4) → Return field-specific errors
        ↓
Complete (Stage 5) → Return success with all data
```

**Error Wrapping:**
Each error type wraps the underlying error while adding context:
- `FileError` wraps OS errors with file path and operation
- `ParseError` wraps YAML parser errors with line/column
- `ValidationError` wraps structural issues with field paths
- `FieldNotFoundError` and `TypeMismatchError` provide field-specific context

**Error Classification:**
```go
errType := ClassifyError(err)
switch errType {
case ErrorTypeIO:
    // File system error
case ErrorTypeSyntax:
    // YAML syntax error
case ErrorTypeStructure:
    // YAML structural error
case ErrorTypeEmpty:
    // Empty file/content
}
```

## Configuration Impact on Data Flow

### Parser Configuration (`ParserConfig`)

| Config Option | Stage Affected | Impact |
|----------------|----------------|--------|
| `Strict` | Parse | Reject unknown fields in struct unmarshaling |
| `AllowDuplicateKeys` | Parse | Allow/fail on duplicate mapping keys |
| `PreserveOrder` | Parse | Maintain key order (performance impact) |
| `EmptyFileAsError` | Parse | Treat empty files as error vs. return empty map |
| `MaxFileSize` | File Read | Reject files exceeding size limit |
| `Encoding` | Parse | Specify character encoding (UTF-8, UTF-16LE, UTF-16BE) |

### Validator Configuration (`ValidatorConfig`)

| Config Option | Stage Affected | Impact |
|----------------|----------------|--------|
| `Strict` | Validate | Treat warnings as errors |
| `CheckDuplicateKeys` | Validate | Report duplicate keys as errors |
| `CheckIndentation` | Validate | Validate indentation consistency |
| `MaxLineLength` | Validate | Warn on long lines |
| `AllowDeprecated` | Validate | Allow/warn on deprecated features |
| `RequireDocumentStart` | Validate | Require `---` document marker |
| `IgnoreComments` | Validate | Strip comments before validation |

## Performance Considerations

### File Reading
- Use `FileExists` before `ReadFile` to handle missing files gracefully
- Set appropriate `MaxFileSize` to prevent memory exhaustion
- Consider streaming for very large files (future: `StreamYAMLParser`)

### Parsing
- Non-strict mode is faster (no unknown field checking)
- Key order preservation has performance penalty
- Generic maps are slower than typed structs

### Validation
- Strict mode is more thorough but slower
- Duplicate key checking adds overhead
- Comment stripping can improve performance

### Field Access
- Cache field values if accessed multiple times
- Use `HasField` before `GetRequired*` to avoid errors
- Prefer `Get*` with defaults over `GetRequired*` when possible

## Common Usage Patterns

### Basic Parse and Access
```go
// Parse file into map
data, err := yamlutil.ParseYAML("config.yaml")
if err != nil {
    log.Fatal(err)
}

// Access fields with defaults
host := yamlutil.GetString(data, "server.host", "localhost")
port := yamlutil.GetInt(data, "server.port", 8080)
```

### Parse with Validation
```go
// Parse
parser := yamlutil.NewParser()
result := parser.ParseFile("config.yaml", &config)
if result.IsFailure() {
    log.Fatal(result.Error)
}

// Validate
validator := yamlutil.NewValidator()
vResult := validator.ValidateFile("config.yaml")
if vResult.HasErrors() {
    log.Fatal(vResult.ErrorSummary())
}
```

### Strict Mode for Production
```go
// Use strict parser and validator for production
parser := yamlutil.NewStrictParser()
validator := yamlutil.NewStrictValidator()

result := parser.ParseFile("production.yaml", &config)
if result.IsFailure() {
    log.Fatal("Production config validation failed")
}

vResult := validator.ValidateFile("production.yaml")
if vResult.HasErrors() {
    log.Fatal("Production config has validation errors")
}
```

### Field Requirement Validation
```go
// Define field requirements
requirements := []yamlutil.EnhancedFieldRequirement{
    {
        Path:           "server.host",
        Required:       true,
        TypeConstraint: "string",
    },
    {
        Path:           "server.port",
        Required:       true,
        TypeConstraint: "int",
        MinValue:       ptr(float64(1)),
        MaxValue:       ptr(float64(65535)),
    },
}

// Validate each requirement
for _, req := range requirements {
    value := getYAMLValue(data, req.Path)
    errs := req.Validate(value, data)
    if len(errs) > 0 {
        log.Printf("Field %s validation failed: %v", req.Path, errs)
    }
}
```

## Debugging Data Flow Issues

### Enable Metrics Collection
```go
pipeline := &ProcessingPipeline{
    InputFile: "config.yaml",
    Metrics:   make([]PipelineMetric, 0),
}

// Track each stage
start := time.Now()
content, err := ReadFile("config.yaml")
duration := time.Since(start)
pipeline.AddMetric(StageFileRead, duration, err == nil, err)
```

### Identify Failed Stage
```go
if stage, err := pipeline.GetFailedStage(); stage != "" {
    log.Printf("Pipeline failed at stage: %s, error: %v", stage, err)
}
```

### Check Stage Durations
```go
for _, metric := range pipeline.Metrics {
    log.Printf("Stage %s: %v (success: %v)", 
        metric.Stage, metric.Duration, metric.Success)
}
```

## Data Flow Diagram

```
┌──────────────┐
│  File Path   │
└──────┬───────┘
       │
       ▼
┌───────────────────────────────────────────┐
│         Stage 1: File Read                │
│  - Resolve absolute path                  │
│  - Check file existence                    │
│  - Read bytes                             │
│  - Validate file size                     │
└──────┬────────────────────────────────────┘
       │ FileError or []byte
       ▼
┌───────────────────────────────────────────┐
│         Stage 2: Parse                    │
│  - Check empty content                    │
│  - Parse YAML syntax                      │
│  - Unmarshal to target                    │
│  - Extract error locations                │
└──────┬────────────────────────────────────┘
       │ ParseError or ParseResult
       ▼
┌───────────────────────────────────────────┐
│         Stage 3: Validate                 │
│  - Syntax validation                      │
│  - Structural validation                  │
│  - Schema validation                      │
│  - Constraint validation                  │
└──────┬────────────────────────────────────┘
       │ ValidationError or ValidationResult
       ▼
┌───────────────────────────────────────────┐
│         Stage 4: Field Access             │
│  - Navigate field paths                   │
│  - Type-safe extraction                   │
│  - Default value handling                 │
│  - Required field validation              │
└──────┬────────────────────────────────────┘
       │ FieldError or typed values
       ▼
┌───────────────────────────────────────────┐
│         Stage 5: Complete                 │
│  - Aggregate metrics                      │
│  - Calculate total duration                │
│  - Return processed data                  │
└──────┬────────────────────────────────────┘
       │
       ▼
┌──────────────┐
│ Final Output │
└──────────────┘
```

## Error Recovery Strategies

### File Read Errors
```go
content, err := yamlutil.ReadFile("config.yaml")
if err != nil {
    if yamlutil.IsFileNotFoundError(err) {
        // Create default config
        return createDefaultConfig()
    }
    if yamlutil.IsPermissionError(err) {
        // Try alternative location
        return yamlutil.ReadFile("/etc/app/config.yaml")
    }
    return err
}
```

### Parse Errors
```go
result := parser.ParseFile("config.yaml", &config)
if result.IsFailure() {
    if parseErr := result.GetDetailedError(); parseErr != nil {
        // Log detailed error with line number
        log.Printf("Parse error at line %d: %s", parseErr.Line, parseErr.Message)
        
        // Try to recover with partial data
        if partialData := extractPartialData(parseErr); partialData != nil {
            return partialData
        }
    }
}
```

### Validation Errors
```go
result := validator.ValidateFile("config.yaml")
if result.HasErrors() {
    // Log all errors
    for _, err := range result.Errors {
        log.Printf("Validation error: %s", err.Error())
    }
    
    // Check if errors are blocking
    if hasBlockingErrors(result.Errors) {
        return fmt.Errorf("blocking validation errors")
    }
    
    // Continue with warnings
    log.Printf("Proceeding with validation warnings")
}
```

### Field Access Errors
```go
// Use defaults for optional fields
port := yamlutil.GetInt(data, "server.port", 8080)

// Check required fields
dbUrl, err := yamlutil.GetRequiredString(data, "database.url")
if err != nil {
    if fieldNotFound, ok := err.(*yamlutil.FieldNotFoundError); ok {
        // Prompt user or use default
        dbUrl = promptUser("Enter database URL:")
    }
}
```

## Summary

The YAML parser module provides a robust, multi-stage pipeline for processing YAML files:

1. **File Read Stage:** Safe file I/O with detailed error context
2. **Parse Stage:** YAML syntax parsing with configurable strictness
3. **Validate Stage:** Comprehensive validation with schema support
4. **Field Access Stage:** Type-safe field access with dot notation
5. **Complete Stage:** Metrics aggregation and result delivery

Each stage provides detailed error information, metrics collection, and configurable behavior through `ParserConfig` and `ValidatorConfig`. The pipeline supports both lenient (development) and strict (production) modes, with appropriate error handling and recovery strategies.
