# YAML Parser API Reference

This document provides comprehensive API documentation for the ARMOR YAML parser module, located in `src/parsers/yaml/`.

## Table of Contents

1. [Overview](#overview)
2. [Parser Trait](#parser-trait)
3. [ParseResult](#parseresult)
4. [ParseError](#parseerror)
5. [ParserConfig](#parserconfig)
6. [Usage Examples](#usage-examples)

---

## Overview

The YAML parser module provides a flexible, trait-based interface for parsing YAML content from various sources. It supports parsing from strings, bytes, and files, with both strict and lenient validation modes.

**Module Location:** `src/parsers/yaml/`

**Module Version:** Available via `parsers::yaml::VERSION`

**Key Components:**
- `Parser` trait - Core parsing interface
- `BasicParser` - Default implementation
- `ParseResult<T>` - Result wrapper with metadata
- `ParseError` - Comprehensive error handling
- `ValidationResult` - Validation output
- `ParserConfig` - Configuration options

---

## Parser Trait

The `Parser` trait defines the core interface for YAML parsing operations.

### Trait Definition

```rust
pub trait Parser {
    fn parse_str(&self, content: &str) -> ParseResult<serde_yaml::Value>;
    fn parse_bytes(&self, content: &[u8]) -> ParseResult<serde_yaml::Value>;
    fn parse_file(&self, path: &std::path::Path) -> ParseResult<serde_yaml::Value>;
    fn validate_str(&self, content: &str) -> ValidationResult;
    fn validate_file(&self, path: &std::path::Path) -> ValidationResult;
    fn config(&self) -> &ParserConfig;
    fn with_config(self, config: ParserConfig) -> Self where Self: Sized;
}
```

### Methods

#### `parse_str`

Parse YAML content from a string.

**Signature:**
```rust
fn parse_str(&self, content: &str) -> ParseResult<serde_yaml::Value>
```

**Parameters:**
- `content: &str` - The YAML content as a string

**Returns:**
- `ParseResult<serde_yaml::Value>` - The parsed data or error

**Example:**
```rust
use armor::parsers::yaml::Parser;

let parser = BasicParser::new();
let yaml = "name: test\nvalue: 42";
let result = parser.parse_str(yaml);
```

#### `parse_bytes`

Parse YAML content from a byte slice.

**Signature:**
```rust
fn parse_bytes(&self, content: &[u8]) -> ParseResult<serde_yaml::Value>
```

**Parameters:**
- `content: &[u8]` - The YAML content as bytes

**Returns:**
- `ParseResult<serde_yaml::Value>` - The parsed data or error

**When to Use:**
- Reading from binary sources
- When you have `Vec<u8>` data
- Working with non-UTF8 sources that need validation

#### `parse_file`

Parse YAML content from a file.

**Signature:**
```rust
fn parse_file(&self, path: &std::path::Path) -> ParseResult<serde_yaml::Value>
```

**Parameters:**
- `path: &std::path::Path` - Path to the YAML file

**Returns:**
- `ParseResult<serde_yaml::Value>` - The parsed data or error

**Error Conditions:**
- File not found
- Permission denied
- Invalid UTF-8 encoding
- YAML syntax errors

#### `validate_str`

Validate YAML content without fully parsing it.

**Signature:**
```rust
fn validate_str(&self, content: &str) -> ValidationResult
```

**Parameters:**
- `content: &str` - The YAML content as a string

**Returns:**
- `ValidationResult` - Validation status with errors and warnings

**Use Cases:**
- Quick syntax checking
- Configuration validation before loading
- Pre-commit hooks

#### `validate_file`

Validate a YAML file without fully parsing it.

**Signature:**
```rust
fn validate_file(&self, path: &std::path::Path) -> ValidationResult
```

**Parameters:**
- `path: &std::path::Path` - Path to the YAML file

**Returns:**
- `ValidationResult` - Validation status with errors and warnings

#### `config`

Get the parser configuration.

**Signature:**
```rust
fn config(&self) -> &ParserConfig
```

**Returns:**
- `&ParserConfig` - Reference to the parser's configuration

#### `with_config`

Set the parser configuration.

**Signature:**
```rust
fn with_config(self, config: ParserConfig) -> Self where Self: Sized
```

**Parameters:**
- `config: ParserConfig` - The new configuration

**Returns:**
- `Self` - The parser with the new configuration

---

## ParseResult

The `ParseResult<T>` type wraps parsing results with metadata and error information.

### Structure

```rust
pub struct ParseResult<T> {
    value: Option<T>,
    error: Option<ParseError>,
    metadata: ParseMetadata,
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `value` | `Option<T>` | The parsed value, if successful |
| `error` | `Option<ParseError>` | The error, if parsing failed |
| `metadata` | `ParseMetadata` | Additional metadata about the parse operation |

### Methods

#### Constructor Methods

```rust
// Create a successful parse result
ParseResult::success(value: T) -> Self

// Create a failed parse result
ParseResult::failure(error: ParseError) -> Self
```

#### Query Methods

```rust
// Check if the parse was successful
pub fn is_success(&self) -> bool

// Check if the parse failed
pub fn is_failure(&self) -> bool

// Get the parsed value
pub fn value(&self) -> Option<&T>

// Get the error, if any
pub fn error(&self) -> Option<&ParseError>

// Get the metadata
pub fn metadata(&self) -> ParseMetadata
```

#### Unwrap Methods

```rust
// Unwrap the value, consuming the result (panics on failure)
pub fn unwrap(self) -> T

// Unwrap the value or return a default
pub fn unwrap_or(self, default: T) -> T
```

#### Transformation Methods

```rust
// Map the success value to a new type
pub fn map<U, F>(self, f: F) -> ParseResult<U>
where
    F: FnOnce(T) -> U
```

#### From Implementation

```rust
impl<T> From<Result<T>> for ParseResult<T>
```

Converts from `std::result::Result<T, ParseError>` to `ParseResult<T>`.

### Usage Example

```rust
let result = parser.parse_str(yaml_content);

if result.is_success() {
    if let Some(value) = result.value() {
        println!("Parsed: {:?}", value);
    }
} else {
    if let Some(error) = result.error() {
        eprintln!("Error: {}", error);
    }
}

// Or using unwrap_or
let value = result.unwrap_or(serde_yaml::Value::Null);
```

---

## ParseResult Metadata

### ParseMetadata Structure

```rust
pub struct ParseMetadata {
    pub lines_processed: usize,
    pub bytes_processed: usize,
    pub processing_time_ns: Option<u64>,
    pub source_path: Option<String>,
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `lines_processed` | `usize` | Number of lines processed |
| `bytes_processed` | `usize` | Number of bytes processed |
| `processing_time_ns` | `Option<u64>` | Processing time in nanoseconds |
| `source_path` | `Option<String>` | Source file path, if known |

### Builder Methods

```rust
// Create new metadata
ParseMetadata::new() -> Self

// Set the number of lines processed
pub fn with_lines(self, lines: usize) -> Self

// Set the number of bytes processed
pub fn with_bytes(self, bytes: usize) -> Self

// Set the source path
pub fn with_source(self, path: impl Into<String>) -> Self
```

---

## ParseError

The `ParseError` type provides detailed error information for parsing failures.

### Structure

```rust
pub struct ParseError {
    pub kind: ParseErrorKind,
    pub line: Option<usize>,
    pub column: Option<usize>,
    pub context: String,
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `kind` | `ParseErrorKind` | The kind of error that occurred |
| `line` | `Option<usize>` | Line number (1-indexed) |
| `column` | `Option<usize>` | Column number (1-indexed) |
| `context` | `String` | Additional context about the error |

### Constructor Methods

```rust
// Create a new ParseError with the given kind
ParseError::new(kind: ParseErrorKind) -> Self

// Create a syntax error
ParseError::syntax(msg: impl Into<String>) -> Self

// Create an I/O error
ParseError::io(msg: impl Into<String>) -> Self

// Create a validation error
ParseError::validation(msg: impl Into<String>) -> Self
```

### Builder Methods

```rust
// Set the line number
pub fn with_line(self, line: usize) -> Self

// Set the column number
pub fn with_column(self, column: usize) -> Self

// Set the context message
pub fn with_context(self, context: impl Into<String>) -> Self
```

### Query Methods

```rust
// Check if this is a syntax error
pub fn is_syntax(&self) -> bool

// Check if this is an I/O error
pub fn is_io(&self) -> bool

// Check if this is a validation error
pub fn is_validation(&self) -> bool
```

---

## ParseErrorKind

The `ParseErrorKind` enum categorizes different types of parsing errors.

### Variants

| Variant | Description | When It Occurs |
|---------|-------------|----------------|
| `Syntax(String)` | Syntax error in the YAML source | Invalid YAML syntax, malformed structure |
| `Io(String)` | I/O error | File not found, permission denied, read errors |
| `Validation(String)` | Validation error | Schema violation, type mismatch |
| `UnexpectedEof` | Unexpected end of input | Incomplete YAML document |
| `InvalidUtf8` | Invalid UTF-8 encoding | Non-UTF8 byte sequences in input |
| `UnknownAnchor(String)` | Unknown anchor or alias | Reference to non-existent anchor |
| `DuplicateKey(String)` | Duplicate key in mapping | Same key appears twice in a mapping |
| `Other(String)` | Other error | Catch-all for unclassified errors |

### Error Handling Example

```rust
match result {
    Ok(value) => println!("Success: {:?}", value),
    Err(error) => {
        eprintln!("Error: {}", error);
        if let Some(line) = error.line {
            eprintln!("  at line {}", line);
        }
        match error.kind {
            ParseErrorKind::Syntax(msg) => eprintln!("  Syntax: {}", msg),
            ParseErrorKind::Io(msg) => eprintln!("  I/O: {}", msg),
            ParseErrorKind::UnknownAnchor(name) => eprintln!("  Unknown anchor: {}", name),
            _ => eprintln!("  Other error"),
        }
    }
}
```

---

## ValidationResult

The `ValidationResult` type represents the outcome of YAML validation operations.

### Structure

```rust
pub struct ValidationResult {
    pub valid: bool,
    pub errors: Vec<ValidationError>,
    pub warnings: Vec<ValidationWarning>,
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `valid` | `bool` | Whether validation passed |
| `errors` | `Vec<ValidationError>` | List of validation errors |
| `warnings` | `Vec<ValidationWarning>` | List of validation warnings |

### Methods

```rust
// Create a successful validation result
ValidationResult::success() -> Self

// Create a failed validation result
ValidationResult::failure(errors: Vec<ValidationError>) -> Self

// Check if validation passed
pub fn is_valid(&self) -> bool

// Check if there are any errors
pub fn has_errors(&self) -> bool

// Check if there are any warnings
pub fn has_warnings(&self) -> bool
```

### Usage Example

```rust
let result = parser.validate_str(yaml_content);

if result.is_valid() {
    println!("YAML is valid");
} else {
    eprintln!("Validation failed:");
    for error in &result.errors {
        eprintln!("  - {} at line {:?}: {}", error.path, error.line, error.message);
    }
}

if result.has_warnings() {
    eprintln!("Warnings:");
    for warning in &result.warnings {
        eprintln!("  - {} at line {:?}: {}", warning.path, warning.line, warning.message);
    }
}
```

### ValidationError / ValidationWarning

Both structures share the same layout:

```rust
pub struct ValidationError {
    pub path: String,        // Path to the invalid element (e.g., "server.port")
    pub message: String,     // Error message
    pub line: Option<usize>, // Line number (1-indexed)
}

pub struct ValidationWarning {
    pub path: String,
    pub message: String,
    pub line: Option<usize>,
}
```

---

## ParserConfig

The `ParserConfig` struct defines configuration options for the YAML parser.

### Structure

```rust
pub struct ParserConfig {
    pub strict_mode: bool,
    pub allow_duplicates: bool,
    pub preserve_quotes: bool,
}
```

### Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `strict_mode` | `bool` | `false` | Enable strict parsing mode |
| `allow_duplicates` | `bool` | `true` | Allow duplicate keys in mappings |
| `preserve_quotes` | `bool` | `false` | Preserve quote information in parsed strings |

### Configuration Options Explained

#### `strict_mode`

When `true`, enables strict YAML parsing:
- Rejects ambiguous YAML constructs
- Enforces stricter type checking
- May reject valid but unusual YAML

**Default:** `false`

**When to Enable:**
- Configuration files requiring strict validation
- Security-sensitive parsing
- When you need to guarantee YAML compliance

#### `allow_duplicates`

When `true`, allows duplicate keys in mappings (last occurrence wins).

When `false`, duplicate keys result in `ParseErrorKind::DuplicateKey`.

**Default:** `true`

**When to Disable:**
- When duplicate keys indicate data errors
- Schema validation requiring unique keys
- Debugging configuration issues

#### `preserve_quotes`

When `true`, preserves quote style information in parsed strings.

When `false`, quoted and unquoted strings are treated identically.

**Default:** `false`

**When to Enable:**
- Round-trip YAML parsing (parse → modify → serialize)
- When quote style is semantically important
- Preserving source formatting

### Default Configuration

```rust
pub const DEFAULT_PARSER_CONFIG: ParserConfig = ParserConfig {
    strict_mode: false,
    allow_duplicates: true,
    preserve_quotes: false,
};
```

### Usage Example

```rust
use armor::parsers::yaml::{Parser, ParserConfig};

// Create custom configuration
let config = ParserConfig {
    strict_mode: true,
    allow_duplicates: false,
    preserve_quotes: true,
};

let parser = BasicParser::new().with_config(config);
```

---

## Usage Examples

### Basic Parsing

```rust
use armor::parsers::yaml::{Parser, BasicParser};

// Create a parser
let parser = BasicParser::new();

// Parse from string
let yaml = r#"
name: example
port: 8080
features:
  - auth
  - logging
"#;

let result = parser.parse_str(yaml);

if result.is_success() {
    println!("Parsed successfully");
    if let Some(value) = result.value() {
        println!("Value: {:?}", value);
    }
}
```

### Strict Parsing

```rust
use armor::parsers::yaml::{Parser, ParserConfig, BasicParser};

// Create a strict parser
let config = ParserConfig {
    strict_mode: true,
    allow_duplicates: false,
    preserve_quotes: false,
};

let parser = BasicParser::new().with_config(config);
let result = parser.parse_str(yaml_content);
```

### Validation Only

```rust
use armor::parsers::yaml::Parser;

let parser = BasicParser::new();
let result = parser.validate_str(yaml_content);

if result.is_valid() {
    println!("YAML is valid!");
} else {
    eprintln!("Validation errors:");
    for error in &result.errors {
        eprintln!("  {} at line {:?}: {}", error.path, error.line, error.message);
    }
}
```

### Error Handling

```rust
use armor::parsers::yaml::{ParseErrorKind, Parser};

let parser = BasicParser::new();
match parser.parse_str(invalid_yaml) {
    Ok(result) => {
        if let Some(value) = result.value() {
            println!("Parsed: {:?}", value);
        }
    }
    Err(error) => {
        eprintln!("Parse error at line {:?}", error.line);
        match error.kind {
            ParseErrorKind::Syntax(msg) => eprintln!("Syntax error: {}", msg),
            ParseErrorKind::Io(msg) => eprintln!("I/O error: {}", msg),
            ParseErrorKind::DuplicateKey(key) => eprintln!("Duplicate key: {}", key),
            _ => eprintln!("Other error: {}", error),
        }
    }
}
```

### File Parsing with Metadata

```rust
use armor::parsers::yaml::Parser;
use std::path::Path;

let parser = BasicParser::new();
let path = Path::new("config.yaml");

let result = parser.parse_file(path);

// Access metadata
if let Some(metadata) = result.value() {
    let meta = result.metadata();
    println!("Processed {} lines, {} bytes",
             meta.lines_processed, meta.bytes_processed);
}
```

### Using Convenience Functions

```rust
use armor::parsers::yaml::{parse_yaml, parse_yaml_file};

// Parse from string
let result = parse_yaml("key: value");

// Parse from file
let result = parse_yaml_file(Path::new("config.yaml"));
```

### Using Builder Pattern

```rust
use armor::parsers::yaml::{BasicParser, ParserConfig};

// Default parser
let parser = BasicParser::new();

// Strict parser (built-in)
let parser = BasicParser::strict();

// Custom parser
let parser = BasicParser::with_config(ParserConfig {
    strict_mode: true,
    allow_duplicates: false,
    preserve_quotes: false,
});
```

---

## Type Aliases

The module provides type aliases for convenience:

```rust
pub type Result<T> = std::result::Result<T, ParseError>;
```

---

## Constants

```rust
/// Module version
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

/// Default parser configuration
pub const DEFAULT_PARSER_CONFIG: ParserConfig = ParserConfig {
    strict_mode: false,
    allow_duplicates: true,
    preserve_quotes: false,
};
```

---

## Related Documentation

- [YAML Module Design](bf-4lqn4-yaml-module-design-summary.md) - Overall module architecture
- [Parser Design Completion](bf-4lqn4-yaml-parser-design-completion.md) - Implementation status
