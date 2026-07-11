# ParseError Type Design for YAML Parsing Failures

## Overview

This document describes the design of the `ParseError` type for handling YAML parsing failures in the ARMOR project. The design emphasizes clear error categorization, rich context information, and type-safe error propagation through Rust's type system.

## Design Goals

1. **Clear Error Categorization**: Distinct error variants for different failure modes
2. **Rich Context Information**: Line numbers, column positions, source paths, code snippets
3. **Composability**: Seamless error propagation using Rust's `?` operator
4. **User-Friendly Output**: Multiple formatting options for different use cases
5. **Type Safety**: Compile-time guarantees through Rust's type system

## Core Type Structure

### ParseError Struct

```rust
pub struct ParseError {
    /// The kind of error that occurred
    pub kind: ParseErrorKind,
    
    /// The line number where the error occurred (1-indexed)
    pub line: Option<usize>,
    
    /// The column number where the error occurred (1-indexed)
    pub column: Option<usize>,
    
    /// The file or source path where the error occurred
    pub path: Option<String>,
    
    /// A code snippet showing the problematic segment
    pub snippet: Option<String>,
    
    /// Additional context about the error
    pub context: String,
}
```

**Design Decisions:**

- **Option types for location fields**: Not all errors have location information (e.g., I/O errors), so `Option<usize>` allows for optional location data
- **1-indexed line/column**: Matches typical text editor conventions for user-friendly error messages
- **String for context**: Allows unlimited contextual information without heap allocation overhead
- **Clone trait**: Enables error reuse and comparison without lifetime complications

### ParseErrorKind Enum

```rust
pub enum ParseErrorKind {
    /// Syntax error in the YAML source
    Syntax(String),
    
    /// I/O error (file not found, permission denied, etc.)
    Io(String),
    
    /// Validation error (constraint violations)
    Validation(String),
    
    /// Type mismatch error (unexpected type for a field)
    TypeMismatch {
        field: String,
        expected: String,
        actual: String,
    },
    
    /// Unexpected end of input
    UnexpectedEof,
    
    /// Invalid UTF-8 encoding
    InvalidUtf8,
    
    /// Unknown anchor or alias
    UnknownAnchor(String),
    
    /// Duplicate key in mapping
    DuplicateKey(String),
    
    /// Other error (catch-all)
    Other(String),
}
```

**Error Variant Usage Guidelines:**

| Variant | Use When | Examples |
|---------|----------|----------|
| `Syntax` | YAML grammar violations | Invalid indentation, malformed scalars, invalid escape sequences |
| `Io` | File system/I/O failures | File not found, permission denied, read/write failures |
| `Validation` | Semantic constraint violations | Port out of range, invalid email format, missing required fields |
| `TypeMismatch` | Wrong Rust/YAML type | Expecting integer but finding string, expecting sequence but finding scalar |
| `UnexpectedEof` | Input ends prematurely | Incomplete YAML documents, truncated files, missing closing brackets |
| `InvalidUtf8` | Encoding errors | Invalid UTF-8 byte sequences in input |
| `UnknownAnchor` | Unresolvable anchor/alias | Anchor reference that doesn't exist in the document |
| `DuplicateKey` | Duplicate mapping keys | YAML mappings with repeated keys (spec violation) |
| `Other` | Catch-all for unclassified errors | Temporary cases during refactoring, unclassifiable external errors |

**Key Distinctions:**

- **TypeMismatch vs Validation**: `TypeMismatch` is about *type* (wrong data type), `Validation` is about *value* (right type but invalid value)
- **UnexpectedEof vs Syntax**: `UnexpectedEof` specifically for premature EOF, `Syntax` for general YAML grammar violations
- **DuplicateKey vs Validation**: `DuplicateKey` specifically for duplicate keys, `Validation` for general constraint violations

## Error Context Fields

### Location Information

- **line: Option<usize>**: 1-indexed line number where error occurred
- **column: Option<usize>**: 1-indexed column position within the line
- **path: Option<String>**: File or source path (absolute or relative)

**Rationale:** Location information is optional because not all errors can be traced to a specific source location (e.g., I/O errors, UTF-8 decoding errors).

### Source Context

- **snippet: Option<String>**: Code snippet showing the problematic lines
- **context: String**: Additional contextual information about what operation was being performed

**Rationale:** Snippets provide visual context for debugging, while the context field explains the broader scenario (e.g., "while parsing database configuration").

## Error Message Formatting

### Display Strategy

The design provides multiple formatting options for different use cases:

#### 1. Single-Line Summary (`summary()`)

```
config.yaml:10: syntax error: invalid token - while parsing services
```

**Use case:** Logging, compact error display

**Components:** Location + error kind + context (if present)

#### 2. Detailed Multi-Line Report (`detailed_report()`)

```
error: config.yaml:10: syntax error: invalid token
  context: while parsing services

  snippet:
    services:
      - name: web
        port: abc
            ^
```

**Use case:** User-facing error messages, debugging

**Components:** Error summary, context section, source snippet with visual indicator

#### 3. Structured Output (`format_structured()`)

```
ParseError { kind: Syntax("invalid token"), location: config.yaml:10:10, line: Some(10), column: Some(10) }
```

**Use case:** Programmatic analysis, structured logging systems

**Components:** Machine-readable field representation

#### 4. Display Trait Implementation

The `Display` trait provides a user-friendly default format that combines the summary with the snippet and visual indicator:

```rust
impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.summary())?;
        
        if let Some(snippet) = &self.snippet {
            // Display snippet with visual indicator
        }
        
        Ok(())
    }
}
```

### Location String Formatting

The `location_string()` method adapts the format based on available information:

| Available Fields | Output Format |
|------------------|---------------|
| path + line + column | `config.yaml:10:5` |
| path + line | `config.yaml:10` |
| path only | `config.yaml` |
| line + column | `10:5` |
| line only | `10` |
| column only | `col 5` |
| none | `<unknown>` |

## Error Propagation Strategy

### From Implementations

The design implements `From<T>` for common error types:

```rust
impl From<std::io::Error> for ParseError        // → ParseErrorKind::Io
impl From<serde_yaml::Error> for ParseError     // → Appropriate kind based on classification
impl From<std::str::Utf8Error> for ParseError    // → ParseErrorKind::InvalidUtf8
impl From<std::string::FromUtf8Error> for ParseError  // → ParseErrorKind::InvalidUtf8
```

**Strategy:** Automatic conversion enables idiomatic Rust error propagation with the `?` operator:

```rust
fn parse_config(path: &Path) -> Result<Config> {
    let content = std::fs::read_to_string(path)?;  // io::Error → ParseError
    parse_yaml(&content)
}
```

### Builder Pattern for Context

The builder-style methods allow adding context during error propagation:

```rust
fn parse_database_config(value: &serde_yaml::Value) -> Result<DatabaseConfig> {
    let port = value["port"]
        .as_i64()
        .ok_or_else(|| ParseError::type_mismatch("port", "integer", "null"))
        .context("while parsing database configuration")?;
    
    Ok(DatabaseConfig { port })
}
```

### Convenience Constructors

The design provides type-safe constructors for common error patterns:

```rust
ParseError::syntax("invalid YAML indentation")
ParseError::io("file not found")
ParseError::validation("port must be between 1 and 65535")
ParseError::type_mismatch("port", "integer", "string")
```

## Type Safety Features

### Result Type Alias

```rust
pub type Result<T> = std::result::Result<T, ParseError>;
```

**Rationale:** Provides a consistent result type throughout the parsing module.

### Type Checker Methods

```rust
pub fn is_syntax(&self) -> bool
pub fn is_io(&self) -> bool
pub fn is_validation(&self) -> bool
pub fn is_type_mismatch(&self) -> bool
```

**Rationale:** Enables pattern matching and error filtering without exposing the internal `kind` field directly.

### Equality Comparison

The `PartialEq` implementation compares only the core identification fields:

```rust
impl PartialEq for ParseError {
    fn eq(&self, other: &Self) -> bool {
        self.kind == other.kind
            && self.line == other.line
            && self.column == other.column
            && self.path == other.path
        // Note: context and snippet are NOT included
    }
}
```

**Rationale:** Two errors are considered equal if they represent the same error at the same location, regardless of additional context.

## Error Classification Strategy

### serde_yaml Error Classification

The `From<serde_yaml::Error>` implementation classifies errors based on message content:

```rust
let err_msg = err.to_string().to_lowercase();

let kind = if err_msg.contains("syntax") || err_msg.contains("unexpected") {
    ParseErrorKind::Syntax(err.to_string())
} else if err_msg.contains("duplicate") {
    ParseErrorKind::DuplicateKey(err.to_string())
} else if err_msg.contains("io") || err_msg.contains("failed to read") {
    ParseErrorKind::Io(err.to_string())
} else {
    ParseErrorKind::Other(err.to_string())
};
```

**Rationale:** serde_yaml errors are mapped to the most specific appropriate kind, with `Other` as a fallback for unclassified errors.

## Error Lifecycle

### Creation

```rust
// Basic creation
let error = ParseError::syntax("invalid token");

// With location
let error = ParseError::syntax("invalid token")
    .with_line(10)
    .with_column(5)
    .with_path("config.yaml");

// With full context
let error = ParseError::syntax("invalid token")
    .with_path("config.yaml")
    .with_line(10)
    .with_column(5)
    .with_context("while parsing services")
    .with_snippet("services:\n  port: abc");
```

### Propagation

```rust
fn parse_port(value: &serde_yaml::Value) -> Result<u16> {
    let port = value["port"]
        .as_i64()
        .ok_or_else(|| ParseError::type_mismatch("port", "integer", "null"))?;
    
    if port < 1 || port > 65535 {
        return Err(ParseError::validation("port must be between 1 and 65535"));
    }
    
    Ok(port as u16)
}
```

### Display

```rust
// For logging
println!("{}", error.summary());

// For user display
println!("{}", error.detailed_report());

// For debugging
println!("{:?}", error);
```

## Integration with ARMOR Ecosystem

### Module Structure

The `ParseError` type is located at:

```
src/parsers/yaml/error.rs  // ParseError definition and implementation
```

### Related Types

```rust
pub type Result<T> = std::result::Result<T, ParseError>;
```

### Error Flow

```
File I/O        → std::io::Error        → ParseError::Io
YAML Parsing    → serde_yaml::Error     → ParseError (classified)
UTF-8 Decoding  → Utf8Error/FromUtf8Error → ParseError::InvalidUtf8
Custom Logic    → direct creation       → ParseError (appropriate kind)
```

## Testing Strategy

The design is supported by comprehensive test coverage:

- **Unit tests**: Individual error kind creation and formatting
- **Integration tests**: Error propagation through parsing pipelines
- **Display tests**: Verify all formatting options produce expected output
- **Propagation tests**: Ensure From implementations work correctly

## Future Considerations

### Extensibility

The `Other` variant allows for temporary error cases during refactoring. If new error patterns emerge, specific variants can be added without breaking existing code.

### Error Recovery

The current design focuses on error reporting rather than recovery. Future enhancements could include:

- Suggested fixes for common errors
- Error severity levels (warning vs error)
- Partial parsing with error collection

### Structured Suggestions

The error context could be extended to include structured suggestions:

```rust
pub struct ParseError {
    // ... existing fields
    pub suggestions: Vec<String>,
}
```

## Summary

The `ParseError` type provides a comprehensive solution for YAML parsing error handling with:

- **9 error variants** covering all common failure modes
- **6 context fields** for rich error information
- **4 formatting options** for different use cases
- **Builder pattern** for ergonomic error construction
- **Type-safe propagation** through Rust's `From` trait
- **Clear documentation** with usage guidelines for each variant

The design balances usability for developers with clarity for end-users, making errors both debuggable and understandable.
