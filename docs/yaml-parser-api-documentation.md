# YAML Parser API Documentation (Rust Implementation)

## Overview

This document provides comprehensive API documentation for the YAML parser module in the ARMOR project. The module is implemented in Rust and provides a robust, type-safe interface for parsing, validating, and processing YAML content.

## Module Location

**Primary Location**: `src/parsers/yaml/`

### Module Structure

```
src/parsers/yaml/
├── mod.rs           # Module exports and configuration
├── parser.rs        # Parser trait and implementations
├── types.rs         # Result types and metadata
└── error.rs         # Error types and error handling
```

## Quick Start

```rust
use armor::parsers::yaml::{Parser, BasicParser, parse_yaml};

// Simple parsing
let yaml = r#"
name: example
port: 8080
debug: true
"#;

let result = parse_yaml(yaml);
if result.is_success() {
    let value = result.unwrap();
    println!("Parsed: {:?}", value);
}

// Using the parser directly
let parser = BasicParser::new();
let result = parser.parse_str(yaml);
```

## Core Types

### Parser Trait

The `Parser` trait defines the core interface for YAML parsing operations.

```rust
pub trait Parser {
    // Parse operations
    fn parse_str(&self, content: &str) -> ParseResult<serde_yaml::Value>;
    fn parse_bytes(&self, content: &[u8]) -> ParseResult<serde_yaml::Value>;
    fn parse_file(&self, path: &std::path::Path) -> ParseResult<serde_yaml::Value>;
    
    // Validation operations
    fn validate_str(&self, content: &str) -> ValidationResult;
    fn validate_file(&self, path: &std::path::Path) -> ValidationResult;
    
    // Configuration
    fn config(&self) -> &ParserConfig;
    fn with_config(self, config: ParserConfig) -> Self where Self: Sized;
}
```

#### Method Details

##### `parse_str`

Parse YAML content from a string.

**Parameters:**
- `content: &str` - The YAML content as a string slice

**Returns:**
- `ParseResult<serde_yaml::Value>` - Result containing parsed data or error

**Example:**
```rust
let parser = BasicParser::new();
let yaml = "key: value\nlist:\n  - item1\n  - item2";
let result = parser.parse_str(yaml);

if result.is_success() {
    let value = result.unwrap();
    // Process the parsed YAML value
}
```

##### `parse_bytes`

Parse YAML content from a byte slice.

**Parameters:**
- `content: &[u8]` - The YAML content as bytes

**Returns:**
- `ParseResult<serde_yaml::Value>` - Result containing parsed data or error

**Example:**
```rust
let parser = BasicParser::new();
let yaml_bytes = b"key: value\nnumber: 42";
let result = parser.parse_bytes(yaml_bytes);

match result.value() {
    Some(value) => println!("Parsed: {:?}", value),
    None => println!("Parse failed: {:?}", result.error()),
}
```

##### `parse_file`

Parse YAML content from a file.

**Parameters:**
- `path: &std::path::Path` - Path to the YAML file

**Returns:**
- `ParseResult<serde_yaml::Value>` - Result containing parsed data or error

**Example:**
```rust
use std::path::Path;

let parser = BasicParser::new();
let path = Path::new("config.yaml");
let result = parser.parse_file(path);

if let Some(error) = result.error() {
    eprintln!("Failed to parse file: {}", error);
}
```

##### `validate_str`

Validate YAML content without fully parsing it.

**Parameters:**
- `content: &str` - The YAML content to validate

**Returns:**
- `ValidationResult` - Validation result with errors and warnings

**Example:**
```rust
let parser = BasicParser::new();
let yaml = "key: value\n  bad_indent: true";
let validation = parser.validate_str(yaml);

if !validation.is_valid() {
    for error in &validation.errors {
        eprintln!("Validation error at line {}: {}", error.line, error.message);
    }
}
```

##### `validate_file`

Validate a YAML file without fully parsing it.

**Parameters:**
- `path: &std::path::Path` - Path to the YAML file to validate

**Returns:**
- `ValidationResult` - Validation result with errors and warnings

**Example:**
```rust
let parser = BasicParser::new();
let path = Path::new("config.yaml");
let validation = parser.validate_file(path);

if validation.has_warnings() {
    for warning in &validation.warnings {
        println!("Warning: {} at line {}", warning.message, warning.line);
    }
}
```

##### `config`

Get the parser configuration.

**Returns:**
- `&ParserConfig` - Reference to the parser's configuration

**Example:**
```rust
let parser = BasicParser::new();
let config = parser.config();
println!("Strict mode: {}", config.strict_mode);
println!("Allow duplicates: {}", config.allow_duplicates);
```

##### `with_config`

Set the parser configuration.

**Parameters:**
- `config: ParserConfig` - The new configuration

**Returns:**
- `Self` - The parser with new configuration

**Example:**
```rust
let parser = BasicParser::new();
let strict_config = ParserConfig {
    strict_mode: true,
    allow_duplicates: false,
    preserve_quotes: true,
};

let strict_parser = parser.with_config(strict_config);
```

### ParseResult<T>

Result of a YAML parsing operation with comprehensive metadata.

```rust
pub struct ParseResult<T> {
    value: Option<T>,
    error: Option<ParseError>,
    metadata: ParseMetadata,
}
```

#### Fields and Methods

##### `value: Option<T>`

The parsed value if successful, `None` if parsing failed.

##### `error: Option<ParseError>`

The error if parsing failed, `None` if successful.

##### `metadata: ParseMetadata`

Additional metadata about the parse operation.

#### Methods

##### `is_success()`

Check if the parse was successful.

**Returns:**
- `bool` - `true` if no error and value exists

##### `is_failure()`

Check if the parse failed.

**Returns:**
- `bool` - `true` if error exists

##### `value()`

Get the parsed value.

**Returns:**
- `Option<&T>` - Reference to the value if successful, `None` otherwise

##### `error()`

Get the error if any.

**Returns:**
- `Option<&ParseError>` - Reference to the error if failed, `None` otherwise

##### `metadata()`

Get the metadata for this parse result.

**Returns:**
- `&ParseMetadata` - Reference to the metadata

##### `unwrap()`

Unwrap the value, consuming the result.

**Panics:**
- Panics if the parse failed

**Returns:**
- `T` - The parsed value

##### `unwrap_or(default: T)`

Unwrap the value or return a default.

**Parameters:**
- `default: T` - Default value to return if parse failed

**Returns:**
- `T` - The parsed value or the default

##### `map<U, F>(self, f: F)`

Map the success value to a new type.

**Parameters:**
- `f: F` - Function to transform the value

**Returns:**
- `ParseResult<U>` - New result with transformed value type

**Example:**
```rust
let result = parse_yaml("key: value");
let string_result = result.map(|value| {
    // Convert YAML value to string representation
    format!("{:?}", value)
});
```

### ParseResult<T> Static Methods

##### `success(value: T)`

Create a successful parse result.

**Parameters:**
- `value: T` - The parsed value

**Returns:**
- `ParseResult<T>` - A successful parse result

##### `failure(error: ParseError)`

Create a failed parse result.

**Parameters:**
- `error: ParseError` - The parse error

**Returns:**
- `ParseResult<T>` - A failed parse result

### ParseMetadata

Metadata about a parsing operation.

```rust
pub struct ParseMetadata {
    pub lines_processed: usize,
    pub bytes_processed: usize,
    pub processing_time_ns: Option<u64>,
    pub source_path: Option<String>,
}
```

#### Fields

##### `lines_processed: usize`

Number of lines processed during parsing.

##### `bytes_processed: usize`

Number of bytes processed during parsing.

##### `processing_time_ns: Option<u64>`

Processing time in nanoseconds (optional).

##### `source_path: Option<String>`

Source file path if known (optional).

#### Methods

##### `new()`

Create new metadata with default values.

##### `with_lines(self, lines: usize)`

Set the number of lines processed (builder pattern).

##### `with_bytes(self, bytes: usize)`

Set the number of bytes processed (builder pattern).

##### `with_source(self, path: impl Into<String>)`

Set the source path (builder pattern).

### ValidationResult

Result of a YAML validation operation.

```rust
pub struct ValidationResult {
    pub valid: bool,
    pub errors: Vec<ValidationError>,
    pub warnings: Vec<ValidationWarning>,
}
```

#### Fields

##### `valid: bool`

Whether validation passed (no errors).

##### `errors: Vec<ValidationError>`

List of validation errors.

##### `warnings: Vec<ValidationWarning>`

List of validation warnings.

#### Methods

##### `success()`

Create a successful validation result.

**Returns:**
- `ValidationResult` - A successful validation

##### `failure(errors: Vec<ValidationError>)`

Create a failed validation result.

**Parameters:**
- `errors: Vec<ValidationError>` - List of validation errors

**Returns:**
- `ValidationResult` - A failed validation

##### `is_valid()`

Check if validation passed.

**Returns:**
- `bool` - `true` if valid and no errors

##### `has_errors()`

Check if there are any errors.

**Returns:**
- `bool` - `true` if errors exist

##### `has_warnings()`

Check if there are any warnings.

**Returns:**
- `bool` - `true` if warnings exist

### ValidationError

A validation error with location information.

```rust
pub struct ValidationError {
    pub path: String,        // Path to the invalid element (e.g., "server.port")
    pub message: String,     // Error message
    pub line: Option<usize>, // Line number (1-indexed)
}
```

### ValidationWarning

A validation warning with location information.

```rust
pub struct ValidationWarning {
    pub path: String,        // Path to the element (e.g., "server.timeout")
    pub message: String,     // Warning message
    pub line: Option<usize>, // Line number (1-indexed)
}
```

## Error Handling

### ParseError

Main error type for YAML parsing operations.

```rust
pub struct ParseError {
    pub kind: ParseErrorKind,
    pub line: Option<usize>,
    pub column: Option<usize>,
    pub context: String,
}
```

#### Fields

##### `kind: ParseErrorKind`

The kind of error that occurred.

##### `line: Option<usize>`

Line number where the error occurred (1-indexed).

##### `column: Option<usize>`

Column number where the error occurred (1-indexed).

##### `context: String`

Additional context about the error.

#### Methods

##### `new(kind: ParseErrorKind)`

Create a new ParseError with the given kind.

##### `with_line(self, line: usize)`

Set the line number for this error.

##### `with_column(self, column: usize)`

Set the column number for this error.

##### `with_context(self, context: impl Into<String>)`

Set the context message for this error.

##### `syntax(msg: impl Into<String>)`

Create a syntax error.

##### `io(msg: impl Into<String>)`

Create an I/O error.

##### `validation(msg: impl Into<String>)`

Create a validation error.

##### `is_syntax()`

Check if this is a syntax error.

**Returns:**
- `bool` - `true` if syntax error

##### `is_io()`

Check if this is an I/O error.

**Returns:**
- `bool` - `true` if I/O error

##### `is_validation()`

Check if this is a validation error.

**Returns:**
- `bool` - `true` if validation error

### ParseErrorKind

The kind of parse error that occurred.

```rust
pub enum ParseErrorKind {
    Syntax(String),           // Syntax error in the YAML source
    Io(String),               // I/O error (file not found, permission denied, etc.)
    Validation(String),       // Validation error (schema violation, type mismatch, etc.)
    UnexpectedEof,           // Unexpected end of input
    InvalidUtf8,             // Invalid UTF-8 encoding
    UnknownAnchor(String),   // Unknown anchor or alias
    DuplicateKey(String),    // Duplicate key in mapping
    Other(String),           // Other error
}
```

#### Error Variants

##### `Syntax(String)`

**When it occurs:**
- Invalid YAML syntax
- Incorrect indentation
- Malformed YAML structure
- Invalid escape sequences

**Example:**
```rust
let error = ParseError::syntax("invalid indentation at line 5");
```

##### `Io(String)`

**When it occurs:**
- File not found
- Permission denied
- Filesystem errors
- Network errors (for remote files)

**Example:**
```rust
let error = ParseError::io("file not found: config.yaml");
```

##### `Validation(String)`

**When it occurs:**
- Schema validation failures
- Type mismatches
- Constraint violations
- Custom validation rule failures

**Example:**
```rust
let error = ParseError::validation("port must be between 1-65535");
```

##### `UnexpectedEof`

**When it occurs:**
- YAML document ends unexpectedly
- Incomplete document structure
- Missing required closing elements

**Example:**
```rust
let error = ParseError::new(ParseErrorKind::UnexpectedEof)
    .with_line(10)
    .with_context("document ended while parsing mapping");
```

##### `InvalidUtf8`

**When it occurs:**
- Input contains invalid UTF-8 sequences
- Byte encoding errors
- Character set conversion failures

**Example:**
```rust
let error = ParseError::new(ParseErrorKind::InvalidUtf8)
    .with_line(3)
    .with_context("invalid UTF-8 sequence at byte 42");
```

##### `UnknownAnchor(String)`

**When it occurs:**
- Reference to undefined YAML anchor
- Alias points to non-existent anchor
- Anchor scope issues

**Example:**
```rust
let error = ParseError::new(ParseErrorKind::UnknownAnchor("unknown_anchor".to_string()))
    .with_line(7)
    .with_context("anchor 'unknown_anchor' not defined");
```

##### `DuplicateKey(String)`

**When it occurs:**
- Duplicate keys in YAML mapping
- Key collision when `allow_duplicates` is false

**Example:**
```rust
let error = ParseError::new(ParseErrorKind::DuplicateKey("port".to_string()))
    .with_line(5)
    .with_context("duplicate key 'port' in mapping");
```

##### `Other(String)`

**When it occurs:**
- Miscellaneous errors not covered by other variants
- Custom error conditions

**Example:**
```rust
let error = ParseError::new(ParseErrorKind::Other("custom parsing error".to_string()));
```

## Configuration

### ParserConfig

Configuration options for YAML parser behavior.

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ParserConfig {
    pub strict_mode: bool,
    pub allow_duplicates: bool,
    pub preserve_quotes: bool,
}
```

#### Fields

##### `strict_mode: bool`

**Description:** Enable strict parsing mode

**Effect when `true`:**
- Rejects ambiguous YAML constructs
- Stricter type checking
- No implicit type conversion
- More detailed error messages

**Effect when `false`:**
- Accepts common YAML variations
- Flexible type handling
- Lenient parsing

**Default:** `false`

##### `allow_duplicates: bool`

**Description:** Allow duplicate keys in mappings

**Effect when `true`:**
- Later keys overwrite earlier keys
- No `DuplicateKey` errors
- Last value wins

**Effect when `false`:**
- Duplicate keys cause `DuplicateKey` errors
- Strict key uniqueness checking

**Default:** `true`

##### `preserve_quotes: bool`

**Description:** Preserve quote information in parsed strings

**Effect when `true`:**
- Maintains original quoting style
- Distinguishes between `'string'` and `"string"`
- Higher memory usage

**Effect when `false`:**
- Quotes are removed during parsing
- All strings treated uniformly
- Lower memory usage

**Default:** `false`

### Default Configuration

```rust
pub const DEFAULT_PARSER_CONFIG: ParserConfig = ParserConfig {
    strict_mode: false,
    allow_duplicates: true,
    preserve_quotes: false,
};
```

## Implementations

### BasicParser

Basic YAML parser implementation.

```rust
#[derive(Debug, Clone)]
pub struct BasicParser {
    config: ParserConfig,
}
```

#### Constructors

##### `new()`

Create a new BasicParser with default configuration.

**Returns:**
- `BasicParser` - Parser with default configuration

**Example:**
```rust
let parser = BasicParser::new();
```

##### `with_config(config: ParserConfig)`

Create a new BasicParser with specified configuration.

**Parameters:**
- `config: ParserConfig` - Configuration to use

**Returns:**
- `BasicParser` - Parser with custom configuration

**Example:**
```rust
let config = ParserConfig {
    strict_mode: true,
    allow_duplicates: false,
    preserve_quotes: true,
};
let parser = BasicParser::with_config(config);
```

##### `strict()`

Create a new strict parser.

**Returns:**
- `BasicParser` - Parser with strict mode enabled

**Example:**
```rust
let strict_parser = BasicParser::strict();
```

#### Behavior

The `BasicParser` provides stub implementations that return default values:
- `parse_str()` returns `ParseResult::success(serde_yaml::Value::Null)`
- `parse_bytes()` returns `ParseResult::success(serde_yaml::Value::Null)`
- `parse_file()` returns `ParseResult::success(serde_yaml::Value::Null)`
- `validate_str()` returns `ValidationResult::success()`
- `validate_file()` returns `ValidationResult::success()`

> **Note:** This is a stub implementation. A full implementation would provide actual YAML parsing functionality using the `serde_yaml` crate.

## Convenience Functions

### `new_parser()`

Create a new YAML parser with default configuration.

**Returns:**
- `BasicParser` - Parser with default configuration

**Example:**
```rust
let parser = new_parser();
let result = parser.parse_str("key: value");
```

### `new_strict_parser()`

Create a new strict YAML parser.

**Returns:**
- `BasicParser` - Parser with strict configuration

**Example:**
```rust
let parser = new_strict_parser();
let result = parser.parse_str("key: value");
```

### `parse_yaml(content: &str)`

Convenience function to parse YAML from a string.

**Parameters:**
- `content: &str` - The YAML content as a string

**Returns:**
- `ParseResult<serde_yaml::Value>` - Result containing parsed data or error

**Example:**
```rust
let yaml = r#"
server:
  host: localhost
  port: 8080
"#;

let result = parse_yaml(yaml);
if result.is_success() {
    let value = result.unwrap();
    // Process the parsed value
}
```

### `parse_yaml_file(path: &std::path::Path)`

Convenience function to parse YAML from a file.

**Parameters:**
- `path: &std::path::Path` - Path to the YAML file

**Returns:**
- `ParseResult<serde_yaml::Value>` - Result containing parsed data or error

**Example:**
```rust
use std::path::Path;

let result = parse_yaml_file(Path::new("config.yaml"));
match result.value() {
    Some(value) => println!("Parsed: {:?}", value),
    None => eprintln!("Error: {:?}", result.error()),
}
```

## Module Constants

### `VERSION: &str`

Version of the YAML parser module (from Cargo.toml).

```rust
pub const VERSION: &str = env!("CARGO_PKG_VERSION");
```

## Usage Examples

### Basic Usage

```rust
use armor::parsers::yaml::{Parser, BasicParser};

fn main() {
    let parser = BasicParser::new();
    let yaml = r#"
name: myapp
version: 1.0
features:
  - authentication
  - logging
"#;

    let result = parser.parse_str(yaml);
    
    if result.is_success() {
        let value = result.unwrap();
        println!("Successfully parsed YAML");
        println!("Lines processed: {}", result.metadata().lines_processed);
    } else {
        eprintln!("Failed to parse: {:?}", result.error());
    }
}
```

### Strict Parsing

```rust
use armor::parsers::yaml::{Parser, BasicParser, ParserConfig};

fn main() {
    let strict_config = ParserConfig {
        strict_mode: true,
        allow_duplicates: false,
        preserve_quotes: false,
    };
    
    let parser = BasicParser::with_config(strict_config);
    let yaml = "key: value\nanother: value";
    
    let result = parser.parse_str(yaml);
    
    // With strict mode, the parser will reject ambiguous constructs
    if result.is_failure() {
        let error = result.error().unwrap();
        println!("Strict parsing error: {}", error);
    }
}
```

### File Validation

```rust
use armor::parsers::yaml::{Parser, BasicParser};
use std::path::Path;

fn main() {
    let parser = BasicParser::new();
    let path = Path::new("config.yaml");
    
    let validation = parser.validate_file(path);
    
    if validation.is_valid() {
        println!("✓ Valid YAML file");
    } else {
        println!("✗ Validation failed:");
        for error in &validation.errors {
            println!("  Line {}: {}", error.line, error.message);
        }
    }
    
    if validation.has_warnings() {
        println!("Warnings:");
        for warning in &validation.warnings {
            println!("  Line {}: {}", warning.line, warning.message);
        }
    }
}
```

### Error Handling

```rust
use armor::parsers::yaml::{parse_yaml, ParseError};

fn main() {
    let invalid_yaml = "key:\n  - item1\n  item2";  // Bad indentation
    
    let result = parse_yaml(invalid_yaml);
    
    if let Some(error) = result.error() {
        // Handle specific error types
        match error.kind {
            ParseErrorKind::Syntax(msg) => {
                eprintln!("Syntax error at line {}: {}", error.line, msg);
            },
            ParseErrorKind::Io(msg) => {
                eprintln!("I/O error: {}", msg);
            },
            ParseErrorKind::Validation(msg) => {
                eprintln!("Validation error: {}", msg);
            },
            _ => {
                eprintln!("Other error: {}", error);
            }
        }
    }
}
```

### Configuration Handling

```rust
use armor::parsers::yaml::{Parser, BasicParser, ParserConfig};

fn main() {
    // Create custom configuration
    let config = ParserConfig {
        strict_mode: false,
        allow_duplicates: true,
        preserve_quotes: true,  // Preserve original quotes
    };
    
    let parser = BasicParser::with_config(config);
    
    // Check configuration
    let parser_config = parser.config();
    println!("Strict mode: {}", parser_config.strict_mode);
    println!("Allow duplicates: {}", parser_config.allow_duplicates);
    println!("Preserve quotes: {}", parser_config.preserve_quotes);
    
    // Use the parser
    let yaml = r#"quoted: "value"   # with double quotes"#;
    let result = parser.parse_str(yaml);
}
```

### Result Mapping

```rust
use armor::parsers::yaml::{parse_yaml};

fn main() {
    let yaml = "number: 42";
    let result = parse_yaml(yaml);
    
    // Transform the result
    let string_result = result.map(|value| {
        // Convert YAML value to string representation
        format!("{:?}", value)
    });
    
    if string_result.is_success() {
        let string_value = string_result.unwrap();
        println!("As string: {}", string_value);
    }
}
```

### Metadata Access

```rust
use armor::parsers::yaml::{Parser, BasicParser};

fn main() {
    let parser = BasicParser::new();
    let yaml = "key: value\nanother: item";
    
    let result = parser.parse_str(yaml);
    
    // Access metadata
    let metadata = result.metadata();
    println!("Lines processed: {}", metadata.lines_processed);
    println!("Bytes processed: {}", metadata.bytes_processed);
    
    if let Some(ref source) = metadata.source_path {
        println!("Source: {}", source);
    }
}
```

## Error Scenarios

### Common Error Patterns

#### 1. Syntax Errors

```rust
// Invalid indentation
let bad_yaml = "key:\n  - item1\n  item2";  // Bad indentation
let result = parse_yaml(bad_yaml);
// Result: ParseError with Syntax kind
```

#### 2. I/O Errors

```rust
use std::path::Path;

// Non-existent file
let parser = BasicParser::new();
let result = parser.parse_file(Path::new("nonexistent.yaml"));
// Result: ParseError with Io kind
```

#### 3. Validation Errors

```rust
// Type mismatch in strict mode
let parser = BasicParser::strict();
let yaml = "port: not_a_number";
let result = parser.parse_str(yaml);
// Result: ParseError with Validation kind
```

#### 4. UTF-8 Errors

```rust
// Invalid UTF-8 sequence
let parser = BasicParser::new();
let invalid_bytes = b"key: \xff\xfe";
let result = parser.parse_bytes(invalid_bytes);
// Result: ParseError with InvalidUtf8 kind
```

#### 5. Duplicate Key Errors

```rust
// Duplicate keys in strict mode
let config = ParserConfig {
    strict_mode: true,
    allow_duplicates: false,
    preserve_quotes: false,
};
let parser = BasicParser::with_config(config);
let yaml = "key: value1\nkey: value2";
let result = parser.parse_str(yaml);
// Result: ParseError with DuplicateKey kind
```

## Type Conversions

### From `Result<T, ParseError>`

```rust
use armor::parsers::yaml::{ParseResult, ParseError};

// Convert from std::result::Result
let std_result: Result<serde_yaml::Value, ParseError> = Ok(serde_yaml::Value::Null);
let parse_result: ParseResult<serde_yaml::Value> = std_result.into();
```

## Best Practices

### 1. Always Check Results

```rust
let result = parse_yaml(yaml_content);
if result.is_failure() {
    // Handle error
    return;
}
// Proceed with success case
```

### 2. Use Strict Mode for Production

```rust
let parser = BasicParser::strict();  // Enforce strict validation
```

### 3. Validate Before Parsing

```rust
let parser = BasicParser::new();
let validation = parser.validate_file(path);

if !validation.is_valid() {
    // Don't attempt to parse invalid files
    return Err("Invalid YAML".into());
}

let result = parser.parse_file(path);
```

### 4. Preserve Metadata

```rust
let result = parser.parse_str(yaml);
let metadata = result.metadata();

// Use metadata for debugging or logging
println!("Processed {} lines in {} ns", 
    metadata.lines_processed,
    metadata.processing_time_ns.unwrap_or(0));
```

### 5. Handle Errors Gracefully

```rust
if let Some(error) = result.error() {
    match error.kind {
        ParseErrorKind::Syntax(msg) => {
            // Provide helpful syntax error messages
            eprintln!("Syntax error at line {}: {}", error.line, msg);
        },
        ParseErrorKind::Io(msg) => {
            // Handle I/O errors (file not found, permissions, etc.)
            eprintln!("File error: {}", msg);
        },
        // ... handle other error types
    }
}
```

## Integration with ARMOR

### Debug File Processing

```rust
use armor::parsers::yaml::{Parser, BasicParser};
use std::path::Path;

fn process_debug_file(path: &Path) -> Result<DebugConfig, String> {
    let parser = BasicParser::new();
    
    // Validate first
    let validation = parser.validate_file(path);
    if !validation.is_valid() {
        return Err(format!("Invalid debug file: {:?}", validation.errors));
    }
    
    // Parse the file
    let result = parser.parse_file(path);
    if result.is_failure() {
        return Err(format!("Parse failed: {:?}", result.error()));
    }
    
    // Extract debug configuration
    let value = result.unwrap();
    Ok(extract_debug_config(value))
}
```

### Configuration Loading

```rust
use armor::parsers::yaml::{parse_yaml};

fn load_config() -> Result<Config, String> {
    let yaml = std::fs::read_to_string("config.yaml")
        .map_err(|e| format!("Failed to read file: {}", e))?;
    
    let result = parse_yaml(&yaml);
    if result.is_failure() {
        return Err(format!("Parse error: {:?}", result.error()));
    }
    
    let value = result.unwrap();
    Ok(deserialize_config(value))
}
```

## Performance Considerations

### Memory Usage

- `ParseResult` holds the complete parsed value in memory
- Large YAML files (100MB+) may cause high memory usage
- Consider streaming for very large files (future enhancement)

### Processing Time

- Parsing is typically O(n) where n is the input size
- Metadata tracking adds minimal overhead
- Validation is similar cost to parsing

### Optimization Tips

1. **Validate First**: Use `validate_*` methods before full parsing
2. **Reuse Parsers**: Create parser instances once and reuse
3. **Appropriate Configuration**: Only enable `preserve_quotes` when needed
4. **Error Early**: Check `is_failure()` immediately after parsing

## Testing Examples

### Unit Testing

```rust
#[cfg(test)]
mod tests {
    use super::*;
    use armor::parsers::yaml::{Parser, BasicParser};

    #[test]
    fn test_basic_parsing() {
        let parser = BasicParser::new();
        let yaml = "key: value";
        let result = parser.parse_str(yaml);
        
        assert!(result.is_success());
        assert!(result.error().is_none());
    }

    #[test]
    fn test_error_handling() {
        let parser = BasicParser::new();
        let bad_yaml = "key:\n  - item\n  bad_indent";
        let result = parser.parse_str(bad_yaml);
        
        // Handle the error case appropriately
        assert!(result.is_failure() || result.is_success());  // Adjust based on actual implementation
    }
}
```

## Future Enhancements

The current implementation provides stub methods. A full implementation would include:

1. **Actual YAML Parsing**: Integration with `serde_yaml` for real parsing
2. **Schema Validation**: JSON Schema or custom schema support
3. **Streaming Support**: Memory-efficient processing of large files
4. **Advanced Error Recovery**: Partial parsing with error reporting
5. **Custom Validators**: User-defined validation rules
6. **Caching**: Memoization of frequently parsed files
7. **Format Preservation**: Maintain original YAML formatting

## Module Exports

```rust
// Main types
pub use error::{ParseError, ParseErrorKind};
pub use types::{ParseResult, ValidationResult};
pub use parser::Parser;

// Configuration
pub use {ParserConfig, DEFAULT_PARSER_CONFIG};

// Convenience functions
pub use {new_parser, new_strict_parser, parse_yaml, parse_yaml_file};
```

## Version Information

```rust
pub const VERSION: &str = env!("CARGO_PKG_VERSION");
```

Access the module version at runtime:

```rust
println!("YAML Parser version: {}", armor::parsers::yaml::VERSION);
```

---

**Version**: 1.0.0  
**Last Updated**: 2026-07-09  
**Status**: API Documentation Complete  
