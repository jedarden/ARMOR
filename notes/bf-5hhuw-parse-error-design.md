# ParseError Type Design Documentation

## Overview

The `ParseError` type represents failure cases in YAML parsing operations, providing structured error information with rich context for debugging and user feedback. This document provides a comprehensive design specification for the error type and its handling philosophy.

## Design Philosophy

The `ParseError` type follows a structured approach to error handling that emphasizes:

1. **Clear categorization** - Each error falls into a distinct category (syntax, I/O, validation, etc.)
2. **Rich context** - Errors carry location information, context messages, and code snippets
3. **Composability** - Errors propagate cleanly through the call stack using Rust's `?` operator
4. **User-friendly output** - Multiple formatting options for different use cases (logging, UI, debugging)

## Type Structure

### Core Fields

| Field | Type | Purpose | Optional |
|-------|------|---------|----------|
| `kind` | `ParseErrorKind` | The specific category of error that occurred | Required |
| `line` | `Option<usize>` | Line number where error occurred (1-indexed) | Optional |
| `column` | `Option<usize>` | Column number where error occurred (1-indexed) | Optional |
| `path` | `Option<String>` | File or source path where error occurred | Optional |
| `snippet` | `Option<String>` | Code snippet showing the problematic segment | Optional |
| `context` | `String` | Additional context about what operation was being performed | Optional |

### Field Relationships

```
Core Error Identification:
  kind + line + column + path = Error identity

Contextual Information:
  context + snippet = User-friendly debugging aids

Equality:
  kind + line + column + path only (context and snippet excluded)
```

Key invariants:
- `kind` is always required
- Location fields (`line`, `column`, `path`) are optional but recommended
- Context fields (`context`, `snippet`) provide additional debugging information
- Equality comparison only includes identification fields, not context

## ParseErrorKind Enum

The `ParseErrorKind` enum represents distinct categories of parsing errors, each serving a specific purpose:

| Variant | Description | Use Case |
|---------|-------------|----------|
| `Syntax(String)` | Syntax error in YAML source | Invalid YAML structure, malformed syntax, grammar violations |
| `Io(String)` | I/O error | File not found, permission denied, read/write failures |
| `Validation(String)` | Validation error | Constraint violations, schema validation failures |
| `TypeMismatch { field, expected, actual }` | Type mismatch error | Wrong Rust/YAML type for a field |
| `UnexpectedEof` | Unexpected end of input | Incomplete YAML documents, truncated files |
| `InvalidUtf8` | Invalid UTF-8 encoding | Encoding errors when converting bytes to strings |
| `UnknownAnchor(String)` | Unknown anchor or alias | Unresolved anchor/alias references |
| `DuplicateKey(String)` | Duplicate key in mapping | YAML mappings with repeated keys |
| `Other(String)` | Catch-all for unclassified errors | Extensibility for future error types |

### Error Kind Selection Guide

#### `Syntax` vs Other Variants

Use `ParseErrorKind::Syntax` for:
- Invalid YAML indentation
- Invalid escape sequences
- Malformed scalars or mappings
- YAML grammar violations

```rust
// ✅ Correct: Use Syntax for YAML syntax errors
ParseError::syntax("invalid indentation");
```

#### `Io` vs Other Variants

Use `ParseErrorKind::Io` for:
- File not found (`std::io::ErrorKind::NotFound`)
- Permission denied (`std::io::ErrorKind::PermissionDenied`)
- Read/write failures (`std::io::ErrorKind::Other`)
- Network I/O errors

```rust
// ✅ Correct: Use Io for file system errors
let content = std::fs::read_to_string(path)?;  // io::Error → Io
```

**Do NOT use** `Io` for:
- YAML syntax errors → use `Syntax`
- Type mismatches → use `TypeMismatch`
- Constraint violations → use `Validation`

#### `TypeMismatch` vs `Validation`

Use `ParseErrorKind::TypeMismatch` when:
- A value has the wrong Rust/YAML **type**
- Expecting an integer but finding a string
- Expecting a sequence but finding a scalar
- Expecting a boolean but finding a number

Use `ParseErrorKind::Validation` for:
- **Value** constraint violations (right type, wrong value)
- Port number > 65535 (correct type, invalid value)
- String doesn't match required pattern
- Array length constraints violated
- Business logic or schema validation failures

The key distinction: `TypeMismatch` is about **type**, `Validation` is about **value**.

```rust
// ✅ Correct: Use TypeMismatch for type errors
ParseError::type_mismatch("port", "integer", "string");

// ✅ Correct: Use Validation for value constraints
ParseError::validation("port must be between 1 and 65535");
```

#### `UnexpectedEof` vs `Syntax`

Use `ParseErrorKind::UnexpectedEof` when:
- Input ends **prematurely**
- Missing closing brackets, braces, quotes
- Truncated files or streams
- Multi-document YAML streams ending mid-document

Use `ParseErrorKind::Syntax` for:
- General YAML syntax violations
- Invalid indentation, escape sequences
- Malformed scalars or mappings not specifically EOF-related

```rust
// ✅ Correct: Use UnexpectedEof for incomplete input
if input.ends_with("key: ") {
    return Err(ParseError::new(ParseErrorKind::UnexpectedEof));
}

// ✅ Correct: Use Syntax for general YAML errors
if !is_valid_indentation(line) {
    return Err(ParseError::syntax("invalid indentation"));
}
```

## Error Creation API

### Constructor Methods

| Method | Purpose | Example |
|--------|---------|---------|
| `new(kind)` | Create error with minimal information | `ParseError::new(ParseErrorKind::Syntax("msg".into()))` |
| `syntax(msg)` | Create syntax error | `ParseError::syntax("invalid YAML")` |
| `io(msg)` | Create I/O error | `ParseError::io("file not found")` |
| `validation(msg)` | Create validation error | `ParseError::validation("port out of range")` |
| `type_mismatch(field, expected, actual)` | Create type mismatch error | `ParseError::type_mismatch("port", "integer", "string")` |

### Builder Methods

| Method | Purpose | Returns |
|--------|---------|---------|
| `with_line(usize)` | Set line number (1-indexed) | `Self` |
| `with_column(usize)` | Set column number (1-indexed) | `Self` |
| `with_path(String)` | Set file/source path | `Self` |
| `with_snippet(String)` | Set code snippet for context | `Self` |
| `with_context(String)` | Set contextual message | `Self` |
| `with_location(usize, usize)` | Set both line and column | `Self` |

### Builder Pattern Examples

```rust
// Minimal error
let error = ParseError::syntax("invalid YAML");

// Full error with all context
let error = ParseError::syntax("invalid port value")
    .with_path("config/services.yaml")
    .with_location(15, 8)
    .with_context("while parsing service configuration")
    .with_snippet("services:\n  - name: web\n    port: abc");

// Chained construction
let error = ParseError::type_mismatch("database.port", "integer", "string")
    .with_path("config/production.yaml")
    .with_location(42, 10)
    .with_context("while parsing database settings");
```

## Error Location Strategy

### Location Fields

**Line and Column Numbers:**
- **1-indexed** to match text editor conventions
- `line` refers to the line number in the source file
- `column` refers to the character position within the line
- Both are optional but highly recommended for user-friendly errors

**Path Information:**
- Can be absolute or relative paths
- Displayed in error messages to help users locate the source
- Optional, but recommended for file-based operations

**Location String Format:**

| Combination | Format | Example |
|-------------|--------|---------|
| path + line + column | `path:line:column` | `config.yaml:10:5` |
| path + line | `path:line` | `config.yaml:10` |
| path only | `path` | `config.yaml` |
| line + column | `line:column` | `42:15` |
| line only | `line` | `42` |
| column only | `col column` | `col 8` |
| none | `<unknown>` | `<unknown>` |

### Error Context Fields

**Context Message:**
- Describes what operation was being performed when the error occurred
- Helps users understand the broader scenario
- Example: "while parsing service configuration"

**Code Snippet:**
- Contains relevant lines of source code where the error occurred
- Displayed in detailed error reports with visual indicator
- Should be concise but sufficient to understand the error

## Error Message Formatting

The `ParseError` type provides three levels of formatting for different use cases:

### 1. Summary Format (Single-line, logging)

```rust
let summary = error.summary();
// Output: "config.yaml:10: syntax error: invalid token - while parsing"
```

**Use cases:**
- Logging to files
- Console output in compact mode
- Error aggregation and summary reports

**Format:**
```
{location}: {error kind} - {context}
```

### 2. Display Format (User-friendly, multi-line)

```rust
let display = format!("{}", error);
// Output:
// config.yaml:10: syntax error: invalid token - while parsing
//
//   snippet:
//     service:
//       port: abc
//             ^
```

**Use cases:**
- User-facing error messages
- Interactive command-line tools
- Development mode output

**Format:**
```
{location}: {error kind} - {context}

  snippet:
    {snippet lines}
    {visual indicator}
```

### 3. Detailed Report Format (Maximum debugging info)

```rust
let report = error.detailed_report();
// Output:
// error: config.yaml:10: syntax error: invalid token - while parsing
//   context: while parsing service configuration
//
//   snippet:
//     service:
//       port: abc
//             ^
```

**Use cases:**
- Debugging complex parsing issues
- Error report generation
- Development and troubleshooting

**Format:**
```
error: {summary}
  context: {context}

  snippet:
    {snippet lines}
    {visual indicator}
```

### 4. Structured Format (Machine-readable)

```rust
let structured = error.format_structured();
// Output: "ParseError { kind: Syntax("..."), location: config.yaml:10:5, line: Some(10), column: Some(5) }"
```

**Use cases:**
- Debugging and development
- Machine-readable logging
- Error analysis and reporting

## Error Propagation Strategy

### Automatic Conversion via `From` Trait

The `ParseError` type implements `From` for common error types:

| Source Type | Target ParseErrorKind | Conversion Strategy |
|-------------|----------------------|---------------------|
| `std::io::Error` | `Io` | Direct message conversion |
| `serde_yaml::Error` | Various | Message-based classification |
| `std::str::Utf8Error` | `InvalidUtf8` | Context added |
| `std::string::FromUtf8Error` | `InvalidUtf8` | Context added |

### Usage with `?` Operator

```rust
fn parse_config(path: &Path) -> Result<Config> {
    // io::Error is automatically converted to ParseError
    let content = std::fs::read_to_string(path)?;
    
    // serde_yaml::Error is automatically converted to ParseError
    let value: serde_yaml::Value = serde_yaml::from_str(&content)?;
    
    Ok(Config::from(value))
}
```

### Adding Context with `.context()`

```rust
fn parse_database_config(value: &serde_yaml::Value) -> Result<DatabaseConfig> {
    let port = value["port"]
        .as_i64()
        .ok_or_else(|| ParseError::type_mismatch("port", "integer", "null"))
        .context("while parsing database configuration")?;
    
    Ok(DatabaseConfig { port })
}
```

## Error Type Checking API

The `ParseError` type provides predicate methods for type checking:

| Method | Returns `true` if |
|--------|-------------------|
| `is_syntax()` | `kind == ParseErrorKind::Syntax(..)` |
| `is_io()` | `kind == ParseErrorKind::Io(..)` |
| `is_validation()` | `kind == ParseErrorKind::Validation(..)` |
| `is_type_mismatch()` | `kind == ParseErrorKind::TypeMismatch { .. }` |

```rust
match result {
    Err(error) if error.is_syntax() => {
        // Handle syntax errors specifically
    }
    Err(error) if error.is_io() => {
        // Handle I/O errors specifically
    }
    Err(error) => {
        // Handle all other errors
    }
    Ok(value) => {
        // Handle success
    }
}
```

## Trait Implementations

### Clone

```rust
#[derive(Clone)]
pub struct ParseError { ... }
```

**Purpose:** Enable error cloning for error accumulation, retry logic, and concurrent error handling.

### PartialEq

```rust
impl PartialEq for ParseError {
    fn eq(&self, other: &Self) -> bool {
        // Only compare identification fields
        self.kind == other.kind
            && self.line == other.line
            && self.column == other.column
            && self.path == other.path
    }
}
```

**Purpose:** Enable error comparison for testing and deduplication. Note that `context` and `snippet` are NOT included in equality to enable error enrichment without affecting identity.

### Display

```rust
impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.summary())?;
        
        if let Some(snippet) = &self.snippet {
            write!(f, "\n\n  snippet:")?;
            for line in snippet.lines() {
                write!(f, "\n    {}", line)?;
            }
            
            if let Some(col) = self.column {
                if col > 0 {
                    write!(f, "\n    {}", " ".repeat(col.saturating_sub(1)))?;
                    write!(f, "^")?;
                }
            }
        }
        
        Ok(())
    }
}
```

**Purpose:** Provide user-friendly error output with code snippets and visual indicators.

### Debug

```rust
impl fmt::Debug for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("ParseError")
            .field("kind", &self.kind)
            .field("location", &self.location_string())
            .field("line", &self.line)
            .field("column", &self.column)
            .field("path", &self.path)
            .field("context", &self.context)
            .field("has_snippet", &self.snippet.is_some())
            .finish()
    }
}
```

**Purpose:** Provide detailed debugging information for development.

### std::error::Error

```rust
impl std::error::Error for ParseError {}
```

**Purpose:** Enable integration with Rust's standard error handling ecosystem.

## Usage Patterns

### Basic Error Creation

```rust
// Syntax error
let error = ParseError::syntax("invalid YAML syntax");

// I/O error
let error = ParseError::io("file not found");

// Validation error
let error = ParseError::validation("port must be between 1 and 65535");

// Type mismatch error
let error = ParseError::type_mismatch("port", "integer", "string");
```

### Error with Location

```rust
let error = ParseError::syntax("invalid token")
    .with_path("config.yaml")
    .with_line(10)
    .with_column(5);
```

### Error with Context

```rust
let error = ParseError::validation("port out of range")
    .with_path("config/services.yaml")
    .with_line(15)
    .with_context("while parsing service configuration");
```

### Error with Snippet

```rust
let error = ParseError::type_mismatch("port", "integer", "string")
    .with_path("config.yaml")
    .with_line(8)
    .with_column(10)
    .with_snippet("service:\n  name: web\n  port: \"8080\"");
```

### Complete Error

```rust
let error = ParseError::syntax("invalid escape sequence")
    .with_path("config/app.yaml")
    .with_location(8, 25)
    .with_context("while parsing application name")
    .with_snippet("app:\n  name: \"My\\nApp\"");
```

## Error Display Examples

### Minimal Error

```rust
let error = ParseError::syntax("invalid YAML");
println!("{}", error);
// Output: <unknown>: syntax error: invalid YAML
```

### Error with Location

```rust
let error = ParseError::syntax("invalid token")
    .with_path("config.yaml")
    .with_line(10);
println!("{}", error);
// Output: config.yaml:10: syntax error: invalid token
```

### Error with Context

```rust
let error = ParseError::syntax("invalid token")
    .with_path("config.yaml")
    .with_line(10)
    .with_context("while parsing services");
println!("{}", error);
// Output: config.yaml:10: syntax error: invalid token - while parsing
```

### Error with Snippet

```rust
let error = ParseError::syntax("invalid port value")
    .with_path("config.yaml")
    .with_line(5)
    .with_column(10)
    .with_snippet("service:\n  port: abc");
println!("{}", error);
// Output:
// config.yaml:5:10: syntax error: invalid port value
//
//   snippet:
//     service:
//       port: abc
//             ^
```

### Type Mismatch Error

```rust
let error = ParseError::type_mismatch("port", "integer", "string")
    .with_path("service.yaml")
    .with_line(8)
    .with_column(10);
println!("{}", error);
// Output: service.yaml:8:10: type mismatch at 'port': expected integer, got string
```

## Integration with ParseResult

The `ParseError` type integrates seamlessly with the `ParseResult<T>` type:

```rust
// Creating a failed ParseResult
let result = ParseResult::<i32>::failure(
    ParseError::syntax("invalid YAML")
        .with_path("config.yaml")
        .with_line(10)
);

// Checking for error
if result.is_failure() {
    if let Some(error) = result.error() {
        println!("Error: {}", error);
    }
}

// Converting from Result<T>
let std_result: Result<i32, ParseError> = Err(ParseError::syntax("error"));
let parse_result = ParseResult::from(std_result);
```

## Best Practices

### DO ✅

1. **Always include location information when available**
   ```rust
   let error = ParseError::syntax("invalid token")
       .with_line(10)
       .with_column(5);
   ```

2. **Use specific error kinds for clarity**
   ```rust
   // ✅ Specific
   ParseError::type_mismatch("port", "integer", "string");
   
   // ❌ Generic
   ParseError::validation("port is wrong type");
   ```

3. **Add context for nested operations**
   ```rust
   let error = ParseError::syntax("invalid YAML")
       .with_context("while parsing service configuration");
   ```

4. **Include snippets for user-friendly errors**
   ```rust
   let error = ParseError::syntax("invalid port")
       .with_snippet("service:\n  port: abc");
   ```

### DON'T ❌

1. **Don't use Io for parsing errors**
   ```rust
   // ❌ Wrong
   ParseError::io("invalid YAML structure");
   
   // ✅ Correct
   ParseError::syntax("invalid YAML structure");
   ```

2. **Don't use Validation for type errors**
   ```rust
   // ❌ Wrong
   ParseError::validation("port must be integer");
   
   // ✅ Correct
   ParseError::type_mismatch("port", "integer", "string");
   ```

3. **Don't omit context in complex operations**
   ```rust
   // ❌ Less helpful
   ParseError::syntax("invalid YAML");
   
   // ✅ More helpful
   ParseError::syntax("invalid YAML")
       .with_context("while parsing database configuration");
   ```

4. **Don't use Other when a specific variant exists**
   ```rust
   // ❌ Wrong
   ParseError::new(ParseErrorKind::Other("duplicate key".to_string()));
   
   // ✅ Correct
   ParseError::new(ParseErrorKind::DuplicateKey("name".to_string()));
   ```

## Summary

The `ParseError` type provides a comprehensive error handling system for YAML parsing with:

- **Clear categorization** via `ParseErrorKind` enum
- **Rich context** through location, path, snippet, and context fields
- **Builder pattern** for flexible error construction
- **Multiple formatting options** for different use cases
- **Standard trait implementations** for ecosystem integration
- **Type-safe error propagation** via `From` traits
- **Predicate methods** for error type checking

This design enables user-friendly error messages while maintaining developer-friendly debugging capabilities, supporting both automated error handling and interactive user scenarios.
