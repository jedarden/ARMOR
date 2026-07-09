# YAML Parser API Documentation

## Overview

The YAML parser module provides comprehensive YAML parsing functionality for the ARMOR project. It includes:

- **Core Parser trait** - Abstract interface for parsing YAML content
- **ParseResult type** - Structured result handling with metadata
- **ParseError types** - Detailed error reporting with location information
- **Validation types** - YAML validation without full parsing
- **Configuration options** - Flexible parser behavior control

## Version

Current version: `0.1.422` (from `CARGO_PKG_VERSION`)

## Table of Contents

1. [Parser Trait](#parser-trait)
2. [ParseResult](#parseresult)
3. [ParseError](#parseerror)
4. [ValidationResult](#validationresult)
5. [Configuration](#configuration)
6. [Usage Examples](#usage-examples)
7. [Error Scenarios](#error-scenarios)

---

## Parser Trait

The `Parser` trait defines the core interface for YAML parsers in ARMOR.

### Trait Methods

#### `parse_str(content: &str) -> ParseResult<serde_yaml::Value>`

Parse YAML content from a string.

**Parameters:**
- `content` - The YAML content as a string slice

**Returns:**
- `ParseResult<serde_yaml::Value>` - The parsed value or error

**Example:**
```rust
use armor::parsers::yaml::Parser;

let parser = BasicParser::new();
let yaml = "name: test\nvalue: 42";
let result = parser.parse_str(yaml);
```

---

#### `parse_bytes(content: &[u8]) -> ParseResult<serde_yaml::Value>`

Parse YAML content from a byte slice.

**Parameters:**
- `content` - The YAML content as bytes

**Returns:**
- `ParseResult<serde_yaml::Value>` - The parsed value or error

**Use case:** Parsing YAML from non-UTF8 sources that need validation

**Example:**
```rust
let yaml_bytes = b"name: test\nvalue: 42";
let result = parser.parse_bytes(yaml_bytes);
```

---

#### `parse_file(path: &std::path::Path) -> ParseResult<serde_yaml::Value>`

Parse YAML content from a file.

**Parameters:**
- `path` - Path to the YAML file

**Returns:**
- `ParseResult<serde_yaml::Value>` - The parsed value or error

**Errors:**
- Returns `ParseErrorKind::Io` if the file doesn't exist or can't be read

**Example:**
```rust
use std::path::Path;

let path = Path::new("config.yaml");
let result = parser.parse_file(path);
```

---

#### `validate_str(content: &str) -> ValidationResult`

Validate YAML content without fully parsing it.

**Parameters:**
- `content` - The YAML content as a string slice

**Returns:**
- `ValidationResult` - Validation status with any errors/warnings

**Use case:** Quick syntax checking without allocating parse structures

**Example:**
```rust
let yaml = "name: test\nvalue: 42";
let validation = parser.validate_str(yaml);
if validation.is_valid() {
    println!("YAML is valid!");
}
```

---

#### `validate_file(path: &std::path::Path) -> ValidationResult`

Validate a YAML file without fully parsing it.

**Parameters:**
- `path` - Path to the YAML file

**Returns:**
- `ValidationResult` - Validation status with any errors/warnings

**Example:**
```rust
let path = Path::new("config.yaml");
let validation = parser.validate_file(path);
```

---

#### `config() -> &ParserConfig`

Get the parser configuration.

**Returns:**
- Reference to the parser's configuration

---

#### `with_config(config: ParserConfig) -> Self`

Create a new parser with the specified configuration.

**Parameters:**
- `config` - The new configuration

**Returns:**
- A new parser instance with the new configuration

**Example:**
```rust
let strict_config = ParserConfig {
    strict_mode: true,
    allow_duplicates: false,
    preserve_quotes: true,
};

let strict_parser = parser.with_config(strict_config);
```

---

## ParseResult

The `ParseResult<T>` type wraps the result of a parsing operation, providing structured success/failure handling with metadata.

### Fields

#### `value: Option<T>`

The parsed value, present only when parsing succeeds.

#### `error: Option<ParseError>`

The error that occurred, present only when parsing fails.

#### `metadata: ParseMetadata`

Additional metadata about the parsing operation (lines processed, bytes, timing, etc.)

### Methods

#### `success(value: T) -> ParseResult<T>`

Create a successful parse result.

**Example:**
```rust
let result = ParseResult::success(serde_yaml::Value::String("hello".into()));
```

---

#### `failure(error: ParseError) -> ParseResult<T>`

Create a failed parse result.

**Example:**
```rust
let error = ParseError::syntax("Unexpected token");
let result = ParseResult::<serde_yaml::Value>::failure(error);
```

---

#### `is_success() -> bool`

Check if the parse was successful.

**Returns:** `true` if parsing succeeded, `false` otherwise

---

#### `is_failure() -> bool`

Check if the parse failed.

**Returns:** `true` if parsing failed, `false` otherwise

---

#### `value() -> Option<&T>`

Get a reference to the parsed value.

**Returns:** `Some(&T)` if successful, `None` if failed

---

#### `error() -> Option<&ParseError>`

Get a reference to the error.

**Returns:** `Some(&ParseError)` if failed, `None` if successful

---

#### `metadata() -> &ParseMetadata`

Get a reference to the parse metadata.

**Returns:** Reference to metadata about the parsing operation

---

#### `unwrap() -> T`

Unwrap the value, consuming the result.

**Panics:** If the parse failed

**Example:**
```rust
let result = parser.parse_str("name: test");
let value = result.unwrap(); // panics if parse failed
```

---

#### `unwrap_or(default: T) -> T`

Unwrap the value or return a default.

**Example:**
```rust
let result = parser.parse_str("invalid: yaml:");
let value = result.unwrap_or(serde_yaml::Value::Null);
```

---

#### `map<U, F>(self, f: F) -> ParseResult<U>`

Transform the success value using a function.

**Type parameters:**
- `U` - The new value type
- `F` - Function type: `FnOnce(T) -> U`

**Example:**
```rust
let result = parser.parse_str("name: test");
let string_result = result.map(|value| value.as_str().unwrap_or("").to_string());
```

---

## ParseError

The `ParseError` type represents errors that occur during YAML parsing, with detailed location and context information.

### Fields

#### `kind: ParseErrorKind`

The specific kind of error that occurred.

#### `line: Option<usize>`

Line number where the error occurred (1-indexed), if available.

#### `column: Option<usize>`

Column number where the error occurred (1-indexed), if available.

#### `context: String`

Additional context about the error.

### Methods

#### `new(kind: ParseErrorKind) -> ParseError`

Create a new ParseError with the given kind.

---

#### `with_line(self, line: usize) -> ParseError`

Set the line number for this error (builder pattern).

**Example:**
```rust
let error = ParseError::syntax("Unexpected token").with_line(42);
```

---

#### `with_column(self, column: usize) -> ParseError`

Set the column number for this error (builder pattern).

---

#### `with_context(self, context: impl Into<String>) -> ParseError`

Set the context message for this error.

**Example:**
```rust
let error = ParseError::syntax("Unexpected token")
    .with_line(42)
    .with_context("While parsing mapping");
```

---

#### `syntax(msg: impl Into<String>) -> ParseError`

Create a syntax error.

**Example:**
```rust
let error = ParseError::syntax("Unexpected colon");
```

---

#### `io(msg: impl Into<String>) -> ParseError`

Create an I/O error.

**Example:**
```rust
let error = ParseError::io("File not found: config.yaml");
```

---

#### `validation(msg: impl Into<String>) -> ParseError`

Create a validation error.

**Example:**
```rust
let error = ParseError::validation("Port must be positive");
```

---

#### `is_syntax() -> bool`

Check if this is a syntax error.

#### `is_io() -> bool`

Check if this is an I/O error.

#### `is_validation() -> bool`

Check if this is a validation error.

---

## ParseErrorKind

Enum representing all possible error categories.

### Variants

#### `Syntax(String)`

**Description:** Syntax error in the YAML source

**When it occurs:**
- Invalid YAML syntax
- Unexpected tokens
- Malformed structures

**Example:**
```rust
ParseErrorKind::Syntax("Unexpected token at line 5".into())
```

---

#### `Io(String)`

**Description:** I/O error during file operations

**When it occurs:**
- File not found
- Permission denied
- Disk read error

**Example:**
```rust
ParseErrorKind::Io("Failed to open config.yaml".into())
```

---

#### `Validation(String)`

**Description:** Validation error (schema or type mismatch)

**When it occurs:**
- Schema validation failures
- Type mismatches
- Constraint violations

**Example:**
```rust
ParseErrorKind::Validation("Port must be between 1-65535".into())
```

---

#### `UnexpectedEof`

**Description:** Unexpected end of input

**When it occurs:**
- YAML document ends prematurely
- Incomplete structures

**Example:**
```rust
ParseErrorKind::UnexpectedEof
```

---

#### `InvalidUtf8`

**Description:** Invalid UTF-8 encoding

**When it occurs:**
- Byte content contains invalid UTF-8 sequences

**Example:**
```rust
ParseErrorKind::InvalidUtf8
```

---

#### `UnknownAnchor(String)`

**Description:** Reference to undefined anchor

**When it occurs:**
- YAML alias references an anchor that doesn't exist

**Example:**
```rust
ParseErrorKind::UnknownAnchor("my_anchor".into())
```

---

#### `DuplicateKey(String)`

**Description:** Duplicate key in mapping

**When it occurs:**
- Same key appears multiple times in a mapping (when `allow_duplicates` is false)

**Example:**
```rust
ParseErrorKind::DuplicateKey("name".into())
```

---

#### `Other(String)`

**Description:** Uncategorized error

**When it occurs:**
- Errors that don't fit other categories

---

## ValidationResult

Result of a YAML validation operation, providing detailed error and warning information.

### Fields

#### `valid: bool`

Whether validation passed (no errors).

#### `errors: Vec<ValidationError>`

List of validation errors found.

#### `warnings: Vec<ValidationWarning>`

List of validation warnings (non-fatal issues).

### Methods

#### `success() -> ValidationResult`

Create a successful validation result.

**Example:**
```rust
let result = ValidationResult::success();
assert!(result.is_valid());
```

---

#### `failure(errors: Vec<ValidationError>) -> ValidationResult`

Create a failed validation result.

**Example:**
```rust
let errors = vec![
    ValidationError { path: "port".into(), message: "Must be positive".into(), line: Some(5) }
];
let result = ValidationResult::failure(errors);
```

---

#### `is_valid() -> bool`

Check if validation passed (no errors).

**Returns:** `true` if valid, `false` if errors exist

---

#### `has_errors() -> bool`

Check if there are any errors.

---

#### `has_warnings() -> bool`

Check if there are any warnings.

---

### ValidationError

Represents a single validation error.

**Fields:**
- `path: String` - Path to the invalid element (e.g., "server.port")
- `message: String` - Error message
- `line: Option<usize>` - Line number where error occurred

---

### ValidationWarning

Represents a single validation warning (non-fatal).

**Fields:**
- `path: String` - Path to the element
- `message: String` - Warning message
- `line: Option<usize>` - Line number

---

## Status

Enum representing success/error states (used by Result types).

### Variants

- `SUCCESS` - Operation completed successfully
- `ERROR` - Operation encountered an error

### Methods

#### `is_success() -> bool`

Check if status is SUCCESS.

#### `is_error() -> bool`

Check if status is ERROR.

#### `from_bool(success: bool) -> Status`

Convert from boolean (true → SUCCESS, false → ERROR).

#### `as_bool() -> bool`

Convert to boolean.

---

## Configuration

### `ParserConfig`

Configuration options for YAML parser behavior.

**Default values:**
```rust
pub const DEFAULT_PARSER_CONFIG: ParserConfig = ParserConfig {
    strict_mode: false,
    allow_duplicates: true,
    preserve_quotes: false,
};
```

### Fields

#### `strict_mode: bool`

**Description:** Enable strict parsing mode

**When enabled:**
- Disallows certain YAML features that may be ambiguous
- Enforces stricter syntax rules
- May reject valid YAML that uses edge cases

**Default:** `false`

---

#### `allow_duplicates: bool`

**Description:** Allow duplicate keys in mappings

**When disabled:**
- Parser returns `ParseErrorKind::DuplicateKey` when duplicate keys are detected
- Only the last occurrence is typically used

**Default:** `true`

---

#### `preserve_quotes: bool`

**Description:** Preserve quote information in parsed strings

**When enabled:**
- Distinguishes between quoted and unquoted strings
- Maintains original quote style in metadata

**Default:** `false`

---

### Configuration Examples

**Default configuration:**
```rust
let parser = BasicParser::new();
// or
let parser = BasicParser::default();
```

**Strict parser:**
```rust
let strict_parser = BasicParser::strict();
// Equivalent to:
// strict_mode: true
// allow_duplicates: false
// preserve_quotes: false
```

**Custom configuration:**
```rust
let config = ParserConfig {
    strict_mode: false,
    allow_duplicates: false,
    preserve_quotes: true,
};
let parser = BasicParser::with_config(config);
```

---

## Usage Examples

### Basic Parsing

```rust
use armor::parsers::yaml::{Parser, BasicParser};

let parser = BasicParser::new();
let yaml = r#"
name: example
port: 8080
debug: true
"#;

match parser.parse_str(yaml) {
    Ok(result) if result.is_success() => {
        let value = result.value().unwrap();
        println!("Parsed successfully: {:?}", value);
    }
    Ok(result) if result.is_failure() => {
        if let Some(error) = result.error() {
            eprintln!("Parse error: {}", error);
        }
    }
    Err(e) => eprintln!("Unexpected error: {:?}", e),
    _ => {}
}
```

### File Parsing

```rust
use std::path::Path;
use armor::parsers::yaml::BasicParser;

let parser = BasicParser::new();
let path = Path::new("config.yaml");

let result = parser.parse_file(path);
if let Some(error) = result.error() {
    eprintln!("Failed to parse {}: {}", path.display(), error);
} else {
    println!("Successfully parsed {}", path.display());
}
```

### Validation

```rust
use armor::parsers::yaml::Parser;

let yaml = r#"
server:
  host: localhost
  port: 8080
"#;

let validation = parser.validate_str(yaml);
if validation.is_valid() {
    println!("YAML is valid!");
} else {
    for error in &validation.errors {
        eprintln!("Error at {}: {}", error.path, error.message);
    }
}

for warning in &validation.warnings {
    println!("Warning: {} - {}", warning.path, warning.message);
}
```

### Error Handling

```rust
use armor::parsers::yaml::{ParseError, ParseErrorKind};

let result = parser.parse_str(invalid_yaml);

if let Some(error) = result.error() {
    match &error.kind {
        ParseErrorKind::Syntax(msg) => {
            eprintln!("Syntax error: {}", msg);
            if let Some(line) = error.line {
                eprintln!("  at line {}", line);
            }
        }
        ParseErrorKind::Io(msg) => {
            eprintln!("I/O error: {}", msg);
        }
        ParseErrorKind::Validation(msg) => {
            eprintln!("Validation error: {}", msg);
        }
        _ => {
            eprintln!("Other error: {}", error);
        }
    }
}
```

### Configuration Management

```rust
use armor::parsers::yaml::{Parser, ParserConfig, BasicParser};

// Start with default parser
let parser = BasicParser::new();

// Check current configuration
let config = parser.config();
println!("Strict mode: {}", config.strict_mode);

// Update configuration for strict parsing
let strict_config = ParserConfig {
    strict_mode: true,
    allow_duplicates: false,
    preserve_quotes: false,
};

let strict_parser = parser.with_config(strict_config);
```

### Convenience Functions

```rust
use armor::parsers::yaml::{parse_yaml, parse_yaml_file};

// Parse from string
let result = parse_yaml("name: test\nvalue: 42");

// Parse from file
let result = parse_yaml_file(Path::new("config.yaml"));
```

---

## Error Scenarios

### 1. Invalid YAML Syntax

**Scenario:** Malformed YAML syntax

**Example:**
```yaml
name: test
  invalid indentation
value: 42
```

**Error:** `ParseErrorKind::Syntax("Unexpected indentation")`

**Location:** Line 2, Column 3

---

### 2. File Not Found

**Scenario:** Attempting to parse a non-existent file

**Example:**
```rust
let result = parser.parse_file(Path::new("missing.yaml"));
```

**Error:** `ParseErrorKind::Io("File not found: missing.yaml")`

---

### 3. Duplicate Keys (when disabled)

**Scenario:** Mapping with duplicate keys

**Example:**
```yaml
ports:
  - 8080
  - 9090
ports:
  - 3000
```

**Error:** `ParseErrorKind::DuplicateKey("ports")`

**Configuration:** Requires `allow_duplicates: false`

---

### 4. Unknown Anchor

**Scenario:** Reference to undefined anchor

**Example:**
```yaml
defaults: &default
  timeout: 30
server:
  timeout: *missing_anchor
```

**Error:** `ParseErrorKind::UnknownAnchor("missing_anchor")`

---

### 5. Invalid UTF-8

**Scenario:** Byte content with invalid UTF-8 sequences

**Example:**
```rust
let invalid_bytes = b"\xFF\xFE invalid";
let result = parser.parse_bytes(invalid_bytes);
```

**Error:** `ParseErrorKind::InvalidUtf8`

---

### 6. Unexpected End of File

**Scenario:** Incomplete YAML document

**Example:**
```yaml
server:
  host: localhost
  port:
```

**Error:** `ParseErrorKind::UnexpectedEof`

---

### 7. Validation Errors

**Scenario:** Schema violations (e.g., type mismatch)

**Example:**
```yaml
server:
  port: "not_a_number"
```

**Error:** `ParseErrorKind::Validation("Port must be an integer")`

---

## API Reference Summary

| Component | Type | Description |
|-----------|------|-------------|
| `Parser` | Trait | Core parser interface |
| `BasicParser` | Struct | Default parser implementation |
| `ParseResult<T>` | Struct | Parse operation result |
| `ParseError` | Struct | Detailed error information |
| `ParseErrorKind` | Enum | Error category |
| `ValidationResult` | Struct | Validation result |
| `ParserConfig` | Struct | Parser configuration |
| `Status` | Enum | Success/error state |

---

## Type Aliases

```rust
pub type Result<T> = std::result::Result<T, ParseError>;
```

This alias simplifies error handling in parser implementations.

---

## Related Modules

- `crate::parsers::yaml::error` - Error types and definitions
- `crate::parsers::yaml::types` - Result and validation types
- `crate::parsers::yaml::parser` - Parser implementations

---

## Notes

- All line and column numbers are 1-indexed for consistency with common text editors
- The parser uses `serde_yaml::Value` as the default parsed type
- Metadata tracking (lines/bytes processed) is available via `ParseMetadata`
- Validation operations are lightweight and don't allocate full parse structures
