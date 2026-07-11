# YAML Parser Module - Interface and Data Structure Definitions

## Overview

This document defines the core interfaces, data structures, and error handling approach for the YAML parser module in the ARMOR project.

## Table of Contents

1. [Type Definitions](#type-definitions)
2. [Error Handling Strategy](#error-handling-strategy)
3. [Parser Interfaces](#parser-interfaces)
4. [Configuration Options](#configuration-options)
5. [Usage Examples](#usage-examples)

---

## Type Definitions

### ParseResult<T>

The `ParseResult<T>` type represents the outcome of a YAML parsing operation.

**Location:** `src/parsers/yaml/types.rs`

```rust
pub struct ParseResult<T> {
    /// The parsed value, if successful
    value: Option<T>,
    /// The error, if parsing failed
    error: Option<ParseError>,
    /// Additional metadata about the parse operation
    metadata: ParseMetadata,
}
```

**Key Methods:**
- `success(value: T) -> Self` - Create a successful parse result
- `failure(error: ParseError) -> Self` - Create a failed parse result
- `is_success(&self) -> bool` - Check if the parse was successful
- `is_failure(&self) -> bool` - Check if the parse failed
- `value(&self) -> Option<&T>` - Get the parsed value
- `error(&self) -> Option<&ParseError>` - Get the error
- `metadata(&self) -> &ParseMetadata` - Get parsing metadata

**Usage Example:**
```rust
use armor::parsers::yaml::types::ParseResult;

// Create a successful result
let success = ParseResult::success(serde_yaml::Value::Number(42.into()));
assert!(success.is_success());

// Create a failed result
let error = ParseError::syntax("invalid YAML");
let failure = ParseResult::<serde_yaml::Value>::failure(error);
assert!(failure.is_failure());
```

### OperationResult<T>

A dataclass-style result type with explicit status, data, and error fields.

```rust
pub struct OperationResult<T> {
    pub status: Status,
    pub data: Option<T>,
    pub error: Option<String>,
}
```

**Status Enum:**
```rust
pub enum Status {
    SUCCESS,
    ERROR,
}
```

### ValidationResult

Result of a YAML validation operation.

```rust
pub struct ValidationResult {
    pub valid: bool,
    pub errors: Vec<ValidationError>,
    pub warnings: Vec<ValidationWarning>,
}
```

---

## Error Handling Strategy

### Core Error Type: ParseError

The `ParseError` type provides structured, contextual error information for all parsing failures.

**Location:** `src/parsers/yaml/error.rs`

### Error Categories (ParseErrorKind)

| Variant | Use Case | Example |
|---------|----------|---------|
| `Syntax(String)` | YAML syntax violations | Invalid indentation, malformed scalars |
| `Io(String)` | File system errors | File not found, permission denied |
| `Validation(String)` | Constraint violations | Port out of range, missing required field |
| `TypeMismatch { field, expected, actual }` | Type coercion failures | Expected integer, got string |
| `UnexpectedEof` | Incomplete input | Missing closing brace, truncated file |
| `InvalidUtf8` | Encoding errors | Invalid UTF-8 byte sequences |
| `UnknownAnchor(String)` | Unresolved aliases | Reference to non-existent anchor |
| `DuplicateKey(String)` | YAML spec violations | Repeated key in mapping |
| `Other(String)` | Catch-all for unclassified errors | External library errors |

### Error Structure

```rust
pub struct ParseError {
    pub kind: ParseErrorKind,           // Error category
    pub line: Option<usize>,             // 1-indexed line number
    pub column: Option<usize>,           // 1-indexed column number
    pub path: Option<String>,           // File/source path
    pub snippet: Option<String>,        // Code snippet showing error
    pub context: String,                 // Additional context
}
```

### Result Type

```rust
pub type Result<T> = std::result::Result<T, ParseError>;
```

### Error Creation Patterns

**Convenience Constructors:**
```rust
// Syntax error
let error = ParseError::syntax("invalid YAML indentation");

// I/O error
let error = ParseError::io("file not found: config.yaml");

// Validation error
let error = ParseError::validation("port must be between 1 and 65535");

// Type mismatch error
let error = ParseError::type_mismatch("port", "integer", "string");
```

**Builder Pattern for Context:**
```rust
let error = ParseError::type_mismatch("service.port", "integer", "string")
    .with_path("config/services.yaml")
    .with_line(10)
    .with_column(8)
    .with_context("while parsing service configuration")
    .with_snippet("services:\n  - name: web\n    port: abc");
```

### Error Display Options

```rust
// Single-line summary (for logging)
let summary = error.summary();
// Output: "config/services.yaml:10:8: type mismatch at 'service.port': expected integer, got string"

// Detailed multi-line report (for users)
let report = error.detailed_report();
// Output: 
// error: config/services.yaml:10:8: type mismatch at 'service.port': expected integer, got string
//   context: while parsing service configuration
//   
//   snippet:
//     services:
//       - name: web
//       port: abc
//          ^

// Structured format (for debugging)
let structured = error.format_structured();
// Output: "ParseError { kind: TypeMismatch { ... }, location: config/services.yaml:10:8, ... }"
```

### Error Propagation

```rust
use std::fs;

fn parse_config(path: &str) -> Result<Config> {
    // Automatic io::Error → ParseError conversion via From trait
    let content = fs::read_to_string(path)?;
    
    // Automatic serde_yaml::Error → ParseError conversion via From trait
    let yaml: serde_yaml::Value = serde_yaml::from_str(&content)?;
    
    Ok(Config::from_yaml(yaml)?)
}
```

---

## Parser Interfaces

### Generic Parser Trait

**Location:** `src/parsers/traits.rs`

The generic `Parser<Input, Output>` trait works with any input/output type combination.

```rust
pub trait Parser<Input, Output> {
    /// Core parsing method
    fn parse(&self, source: Input) -> Result<Output, ParseError>;
    
    /// Parse with extended options
    fn parse_with_options(&self, source: Input, options: ParseOptions) -> Result<Output, ParseError>;
    
    /// Parse from file
    fn parse_file(&self, path: &Path) -> Result<Output, ParseError>
    where
        Self: Sized,
        Input: From<String>;
    
    /// Validate without full parsing
    fn validate(&self, source: Input) -> Result<(), ParseError>;
    
    /// Get parser metadata
    fn metadata(&self) -> ParseMetadata;
}
```

### YAML-Specific Parser Trait

**Location:** `src/parsers/yaml/parser.rs`

```rust
pub trait Parser {
    /// Parse YAML from string
    fn parse_str(&self, content: &str) -> ParseResult<serde_yaml::Value>;
    
    /// Parse YAML from bytes
    fn parse_bytes(&self, content: &[u8]) -> ParseResult<serde_yaml::Value>;
    
    /// Parse YAML from file
    fn parse_file(&self, path: &Path) -> ParseResult<serde_yaml::Value>;
    
    /// Validate YAML without full parsing
    fn validate_str(&self, content: &str) -> ValidationResult;
    fn validate_file(&self, path: &Path) -> ValidationResult;
    
    /// Get/set configuration
    fn config(&self) -> &ParserConfig;
    fn with_config(self, config: ParserConfig) -> Self where Self: Sized;
}
```

### StreamingParser Trait

For processing multiple sources in sequence or parallel.

```rust
pub trait StreamingParser<Input, Output>: Parser<Input, Output> {
    fn parse_stream<'a, I>(&self, sources: I) -> Result<Vec<Output>, ParseError>
    where
        Input: 'a,
        I: IntoIterator<Item = Input>;
    
    fn parse_parallel<'a, I>(&self, sources: I, parallelism: usize) -> Result<Vec<Output>, ParseError>
    where
        Input: 'a,
        I: IntoIterator<Item = Input>;
}
```

### IncrementalParser Trait

For parsing large inputs in chunks without loading entire source into memory.

```rust
pub trait IncrementalParser<Output>: Parser<Vec<u8>, Output> {
    fn init_parse(&mut self) -> Result<(), ParseError>;
    fn feed_chunk(&mut self, chunk: Vec<u8>) -> Result<(), ParseError>;
    fn finalize_parse(&mut self) -> Result<Output, ParseError>;
}
```

---

## Configuration Options

### ParserMode

**Location:** `src/parsers/config.rs`

```rust
pub enum ParserMode {
    Strict,   // Reject malformed input, unknown fields are errors
    Lenient,  // Recover from errors, ignore unknown fields
}
```

**Strict Mode Behavior:**
- Unknown fields cause parsing failures
- Type mismatches are errors (no coercion)
- Duplicate keys are rejected
- All syntax rules enforced
- Missing required fields cause errors

**Lenient Mode Behavior:**
- Unknown fields are silently ignored
- Type mismatches are coerced when possible
- Last duplicate key wins
- Some syntax variations accepted
- Missing optional fields use defaults

### TypeConstructor

Custom hooks for constructing specific types during parsing.

```rust
pub type TypeConstructorFn = fn(&str, &serde_yaml::Value) -> Result<serde_yaml::Value, String>;

pub struct TypeConstructor {
    pub type_name: String,
    pub constructor: TypeConstructorFn,
}
```

**Example: Custom Log Level Parser**
```rust
fn log_level_constructor(field: &str, value: &serde_yaml::Value) -> Result<serde_yaml::Value, String> {
    let s = value.as_str().ok_or("expected string")?.to_lowercase();
    
    let level = match s.as_str() {
        "debug" => 0,
        "info" => 1,
        "warn" | "warning" => 2,
        "error" => 3,
        _ => return Err(format!("invalid log level: {}", s)),
    };
    
    Ok(serde_yaml::Value::Number(level.into()))
}

let constructor = TypeConstructor::new("LogLevel", log_level_constructor);
config.register_constructor("log_level", constructor);
```

### ValidationHook

Custom validation logic for specific fields or types.

```rust
pub type ValidationFn = fn(&str, &serde_yaml::Value) -> Result<(), String>;

pub struct ValidationHook {
    pub field_pattern: String,    // Supports "*" wildcard
    pub validator: ValidationFn,
}
```

**Example: Port Number Validation**
```rust
fn validate_port(field: &str, value: &serde_yaml::Value) -> Result<(), String> {
    let port = value.as_i64().ok_or("port must be an integer")?;
    
    if !(1..=65535).contains(&port) {
        return Err(format!("port {} out of valid range (1-65535)", port));
    }
    
    Ok(())
}

let hook = ValidationHook::new("port", validate_port);
config.register_validation(hook);
```

### ParserConfig Structure

```rust
pub struct ParserConfig {
    /// Parsing mode (strict vs lenient)
    pub mode: ParserMode,
    
    /// Allow duplicate keys in mappings
    pub allow_duplicates: bool,
    
    /// Preserve comments in output
    pub preserve_comments: bool,
    
    /// Preserve quote information
    pub preserve_quotes: bool,
    
    /// Maximum nesting depth (0 = unlimited)
    pub max_depth: usize,
    
    /// Enforce strict type checking
    pub strict_types: bool,
    
    /// Custom type constructors
    pub type_constructors: HashMap<String, TypeConstructor>,
    
    /// Custom validation hooks
    pub validation_hooks: Vec<ValidationHook>,
    
    /// Emit warnings for recoverable errors
    pub emit_warnings: bool,
    
    /// Treat warnings as errors
    pub warnings_as_errors: bool,
}
```

### Configuration Presets

**Default (Lenient):**
```rust
let config = ParserConfig::default();
// mode: Lenient
// allow_duplicates: true
// strict_types: false
```

**Strict Mode:**
```rust
let config = ParserConfig::strict();
// mode: Strict
// allow_duplicates: false
// strict_types: true
```

**Lenient Mode:**
```rust
let config = ParserConfig::lenient();
// mode: Lenient
// allow_duplicates: true
// strict_types: false
```

### Builder Pattern

```rust
let config = ParserConfig::builder()
    .mode(ParserMode::Strict)
    .allow_duplicates(false)
    .max_depth(10)
    .preserve_comments(true)
    .with_constructor("timeout", timeout_constructor)
    .with_validation(port_validator)
    .build();
```

---

## Usage Examples

### Basic Parsing

```rust
use armor::parsers::yaml::{parse_yaml, ParseError, Result};

fn main() -> Result<()> {
    let yaml_content = r#"
        server:
          host: localhost
          port: 8080
    "#;
    
    let result = parse_yaml(yaml_content);
    
    match result {
        Ok(parsed) => {
            println!("Parsed successfully: {:?}", parsed);
        }
        Err(ParseError::Syntax(msg)) => {
            eprintln!("Syntax error: {}", msg);
        }
        Err(ParseError::Validation(msg)) => {
            eprintln!("Validation error: {}", msg);
        }
        Err(e) => {
            eprintln!("Error: {}", e);
        }
    }
    
    Ok(())
}
```

### Error Handling with Context

```rust
use armor::parsers::yaml::ParseError;

fn parse_service_config(value: &serde_yaml::Value) -> Result<ServiceConfig> {
    let port = value["port"]
        .as_i64()
        .ok_or_else(|| ParseError::type_mismatch("port", "integer", "null"))
        .context("while parsing service configuration")?;
    
    if port < 1 || port > 65535 {
        return Err(ParseError::validation("port must be between 1 and 65535"));
    }
    
    Ok(ServiceConfig { port: port as u16 })
}
```

### Custom Type Constructors

```rust
use armor::parsers::config::{ParserConfig, TypeConstructor};
use serde_yaml::Value;

fn duration_constructor(field: &str, value: &Value) -> Result<Value, String> {
    let s = value.as_str().ok_or("duration must be a string")?;
    
    // Parse "5s", "100ms", etc.
    let (num_str, unit) = if let Some(suffix) = s.strip_suffix('s') {
        (suffix, "seconds")
    } else if let Some(suffix) = s.strip_suffix("ms") {
        (suffix, "milliseconds")
    } else {
        return Err(format!("invalid duration format: {}", s));
    };
    
    let num: u64 = num_str.parse()
        .map_err(|_| format!("invalid number: {}", num_str))?;
    
    let millis = match unit {
        "seconds" => num * 1000,
        "milliseconds" => num,
        _ => unreachable!(),
    };
    
    Ok(Value::Number(millis.into()))
}

let mut config = ParserConfig::default();
config.register_constructor("timeout", TypeConstructor::new("Duration", duration_constructor));
```

### Validation Hooks

```rust
use armor::parsers::config::{ParserConfig, ValidationHook};

fn validate_port(field: &str, value: &Value) -> Result<(), String> {
    let port = value.as_i64().ok_or("port must be an integer")?;
    if !(1..=65535).contains(&port) {
        return Err(format!("port {} out of valid range (1-65535)", port));
    }
    Ok(())
}

fn validate_log_level(field: &str, value: &Value) -> Result<(), String> {
    let level = value.as_str().ok_or("log_level must be a string")?;
    match level.to_lowercase().as_str() {
        "debug" | "info" | "warn" | "warning" | "error" => Ok(()),
        _ => Err(format!("invalid log level: {}", level)),
    }
}

let mut config = ParserConfig::builder()
    .mode(ParserMode::Strict)
    .with_validation(ValidationHook::new("port", validate_port))
    .with_validation(ValidationHook::new("log_level", validate_log_level))
    .build();
```

### File Parsing

```rust
use armor::parsers::yaml::Parser;
use std::path::Path;

fn main() -> Result<()> {
    let parser = armor::parsers::yaml::BasicParser::new();
    let path = Path::new("config/service.yaml");
    
    let result = parser.parse_file(path)?;
    
    if result.is_success() {
        if let Some(value) = result.value() {
            println!("Parsed: {:?}", value);
        }
    }
    
    Ok(())
}
```

---

## Design Decisions

### Why Two Error Types?

1. **`ParseResult<T>`** - High-level wrapper that carries metadata alongside the value/error
2. **`Result<T>`** - Standard Rust `Result<T, ParseError>` for idiomatic error propagation

### Why Two Parser Traits?

1. **`Parser<Input, Output>`** - Generic interface for any parser implementation
2. **`Parser`** (YAML-specific) - Specialized interface for YAML parsing with YAML-specific methods

### Why Comprehensive Error Context?

- **Line/column information** - Pinpoint exact error location
- **Code snippets** - Show users what went wrong
- **Context chains** - Track error propagation through call stack
- **Multiple display formats** - Support logging, UI, and debugging use cases

### Configuration Philosophy

- **Default to lenient** - Better user experience, fewer errors
- **Strict mode available** - For validation-critical applications
- **Extensible hooks** - Custom type constructors and validation
- **Builder pattern** - Fluent configuration API

---

## Module Organization

```
src/parsers/
├── mod.rs                  # Parser module exports
├── config.rs               # Configuration types (ParserConfig, TypeConstructor, ValidationHook)
├── traits.rs               # Generic parser traits (Parser, StreamingParser, IncrementalParser)
└── yaml/
    ├── mod.rs              # YAML module exports
    ├── error.rs           # Error types (ParseError, ParseErrorKind)
    ├── types.rs           # Result types (ParseResult, OperationResult, ValidationResult)
    └── parser.rs          # YAML-specific parser trait and implementations
```

---

## Best Practices

### Error Creation

✅ **DO:** Use specific error variants
```rust
ParseError::type_mismatch("port", "integer", "string")
```

❌ **DON'T:** Use generic `Other` unless necessary
```rust
ParseError::new(ParseErrorKind::Other("port is wrong type".to_string()))
```

### Error Propagation

✅ **DO:** Use `?` operator for automatic conversion
```rust
let content = fs::read_to_string(path)?;
let yaml: serde_yaml::Value = serde_yaml::from_str(&content)?;
```

❌ **DON'T:** Manual error conversion (unless adding context)
```rust
let content = fs::read_to_string(path).map_err(|e| ParseError::io(e.to_string()))?;
```

### Context Addition

✅ **DO:** Add context at appropriate abstraction levels
```rust
ParseError::type_mismatch("port", "integer", "string")
    .with_context("while parsing service configuration")
```

❌ **DON'T:** Over-embed context in error messages
```rust
ParseError::syntax("while parsing service configuration, port is wrong type")
```

---

## Testing Strategy

### Unit Tests

- Test each error variant creation
- Test error display formats
- Test configuration builders
- Test type constructors
- Test validation hooks

### Integration Tests

- Test full parsing workflows
- Test error propagation through multiple layers
- Test configuration behavior
- Test custom constructors and validators

### Property Tests

- Error round-trip (create → display → parse)
- Configuration equivalence (builder ≡ direct construction)
- Error classification (is_* methods accurate)

---

## Future Extensions

### Potential Additions

1. **Schema validation** - JSON Schema/YAML Schema support
2. **Anchor/alias resolution** - Full YAML 1.2 spec compliance
3. **Comment preservation** - Maintain comments in parsed output
4. **Location tracking** - Source locations for all parsed values
5. **Merge keys** - Support for YAML merge keys (`<<:`)
6. **Tags and custom types** - YAML tag resolution and custom types

### Extensibility Points

- Custom parser strategies via trait implementation
- Custom error types via `ParseErrorKind::Other`
- Custom type constructors via `ParserConfig::type_constructors`
- Custom validation via `ParserConfig::validation_hooks`

---

## References

- [YAML 1.2 Specification](https://yaml.org/spec/1.2/spec.html)
- [serde_yaml Documentation](https://docs.rs/serde_yaml/)
- [Rust Error Handling Best Practices](https://doc.rust-lang.org/book/ch09-00-error-handling.html)

---

*Version: 0.1.0*  
*Last Updated: 2025-01-11*