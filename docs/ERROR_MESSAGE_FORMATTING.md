# Error Message Formatting Standards

This document describes the standardized error message formatting patterns used throughout the ARMOR codebase.

## Overview

ARMOR uses a structured approach to error handling that emphasizes:

1. **Clear categorization** - Each error falls into a distinct category (syntax, I/O, validation, etc.)
2. **Rich context** - Errors carry location information, context messages, and code snippets
3. **Human-readable output** - Multiple formatting options for different use cases
4. **Consistent patterns** - All error types follow established formatting conventions

## Error Types

### 1. ParseError (YAML)

Located in `src/parsers/yaml/error.rs`

**Purpose**: Comprehensive error type for YAML parsing operations

**Format Components**:
- Location: `<file>:<line>:<column>` or variations
- Error kind: `syntax error:`, `I/O error:`, `validation error:`, etc.
- Message: The specific error description
- Context: Additional information (optional, separated by ` - `)

**Format Examples**:

```bash
# Full location with context
config.yaml:10:5: syntax error: Missing colon - while parsing service definition

# File and line only
config.yaml:10: syntax error: Invalid YAML structure

# Line and column only
10:5: syntax error: Unexpected token

# Type mismatch with field path
config.yaml:8:10: type mismatch at 'database.port': expected integer, got string
```

**Constructor Methods**:
```rust
// Syntax errors
ParseError::syntax("invalid YAML indentation")

// I/O errors  
ParseError::io("file not found")

// Validation errors
ParseError::validation("port must be between 1 and 65535")

// Type mismatch errors
ParseError::type_mismatch("port", "integer", "string")

// With builder pattern
ParseError::syntax("invalid token")
    .with_path("config.yaml")
    .with_line(10)
    .with_column(5)
    .with_context("while parsing services")
    .with_snippet("services:\n  - name: web\n    port: abc")
```

**Display Methods**:
```rust
// Single-line summary (for logging)
error.summary()  // "config.yaml:10: syntax error: Invalid token - while parsing services"

// Detailed multi-line report (for user display)
error.detailed_report()  // Includes snippet with visual indicator

// Location string
error.location_string()  // "config.yaml:10:5"

// Standard display
format!("{}", error)  // Same as summary()
```

### 2. ValidationError

Located in `src/parsers/yaml/types.rs`

**Purpose**: Validation errors with field paths

**Format Components**:
- Line number (optional): `<line>: `
- Error type: `validation error at`
- Field path: `'<field-path>'` (quoted, dot-notation)
- Message: The validation failure description

**Format Examples**:

```bash
# With line number
15: validation error at 'server.port': port must be between 1 and 65535

# Without line number
validation error at 'server.port': port must be between 1 and 65535

# Nested field paths
42: validation error at 'spec.template.spec.containers[0].image': invalid image tag

# Array field paths
10: validation error at 'services[0].port': port must be between 1 and 65535
```

**Constructor Methods**:
```rust
// Basic validation error
ValidationError::new("server.port", "port must be between 1 and 65535")

// With line number
ValidationError::new("server.port", "port must be between 1 and 65535")
    .with_line(15)
```

### 3. ParseWarning

Located in `src/parsers/yaml/types.rs`

**Purpose**: Non-fatal warnings during parsing

**Format Components**:
- Location (optional): `<line>: `
- Warning type: `warning:`
- Message: The warning description

**Format Examples**:

```bash
# Deprecated field
10: warning: field 'old_api' is deprecated, use 'new_api' instead

# Unknown key
15: warning: unknown key 'unknown_setting'

# Duplicate key
20: warning: duplicate key 'name'
```

**Constructor Methods**:
```rust
// Deprecated field warning
ParseWarning::deprecated_field("old_api", "new_api")

// Unknown key warning
ParseWarning::unknown_key("unknown_setting")

// With line number
ParseWarning::deprecated_field("old_field", "new_field")
    .with_line(10)
```

### 4. ValidationWarning

Located in `src/parsers/yaml/types.rs`

**Purpose**: Non-fatal validation warnings

**Format Components**:
- Line number (optional): `<line>: `
- Warning type: `warning at`
- Field path: `'<field-path>'` (quoted)
- Message: The warning description

**Format Examples**:

```bash
# With line number
25: warning at 'server.timeout': value is unusually high

# Without line number
warning at 'server.timeout': value is unusually high
```

## Error Format Patterns

### Pattern 1: Location Prefix

All error types start with location information when available:

```rust
// Full location: file:line:column
"config.yaml:10:5: error message"

// Partial location: file:line
"config.yaml:10: error message"

// Line only: line:column
"10:5: error message"

// No location
"error message"
```

### Pattern 2: Error Type Label

Each error includes a descriptive label:

```rust
"syntax error: <message>"
"I/O error: <message>"  
"validation error: <message>"
"validation error at '<field>': <message>"
"type mismatch at '<field>': expected <expected>, got <actual>"
"warning: <message>"
"warning at '<field>': <message>"
```

### Pattern 3: Context Separator

Additional context is separated by ` - `:

```rust
"config.yaml:10:5: syntax error: Missing colon - while parsing service definition"
                    ^^^                      ^                             ^
                  location               error kind                    context
```

### Pattern 4: Field Path Format

Field paths use quoted dot-notation:

```rust
// Simple field
"server.port"

// Nested field
"database.connectionPool.maxConnections"

// Array access
"servers[0].port"

// Kubernetes-style paths
"spec.template.spec.containers[0].image"
```

### Pattern 5: Type Mismatch Format

Type mismatches follow a specific pattern:

```rust
"type mismatch at '<field>': expected <expected>, got <actual>"

// Examples:
"type mismatch at 'port': expected integer, got string"
"type mismatch at 'enabled': expected boolean, got string"
"type mismatch at 'tags': expected array, got scalar"
```

## Creating New Error Messages

When creating new error messages, follow these guidelines:

### 1. Choose the Right Error Type

| Situation | Error Type | Constructor |
|-----------|-----------|-------------|
| Invalid YAML syntax | `ParseError::syntax()` | Syntax errors |
| File I/O failures | `ParseError::io()` | I/O errors |
| Constraint violations | `ParseError::validation()` | Validation errors |
| Wrong type for field | `ParseError::type_mismatch()` | Type errors |
| Schema validation failures | `ValidationError::new()` | Field validation |
| Non-critical issues | `ParseWarning::*()` | Warnings |

### 2. Provide Clear, Actionable Messages

```rust
// ✅ Good: Clear and actionable
ParseError::validation("port must be between 1 and 65535")
ParseError::syntax("missing colon after key name")
ParseError::type_mismatch("port", "integer", "string")

// ❌ Bad: Vague or cryptic
ParseError::validation("invalid value")
ParseError::syntax("parse error")
```

### 3. Include Context When Useful

```rust
// With context for better debugging
ParseError::syntax("invalid escape sequence")
    .with_context("while parsing field 'description'")

// Without context for simple cases
ParseError::validation("port out of range")
```

### 4. Use Field Paths for Location

```rust
// ✅ Good: Specific field path
ValidationError::new("database.port", "port must be between 1 and 65535")

// ❌ Bad: Generic location
ValidationError::new("port", "port must be between 1 and 65535")
```

## Testing Error Messages

The ARMOR test suite includes comprehensive error message format tests:

```bash
# Run error message format tests
cargo test --test error_message_format_examples

# Run validation error format tests
cargo test --test validation_error_format_test

# Run all error-related tests
cargo test error
```

## Examples

### Example 1: Complete ParseError with All Components

```rust
let error = ParseError::type_mismatch("services[0].port", "integer", "string")
    .with_path("config/services.yaml")
    .with_line(10)
    .with_column(14)
    .with_context("while parsing service configuration")
    .with_snippet("services:\n  - name: web\n    port: \"8080\"");

println!("{}", error.summary());
// Output: config/services.yaml:10:14: type mismatch at 'services[0].port': expected integer, got string - while parsing service configuration

println!("{}", error.detailed_report());
// Output:
// error: config/services.yaml:10:14: type mismatch at 'services[0].port': expected integer, got string - while parsing service configuration
//   context: while parsing service configuration
//
//   snippet:
//     services:
//       - name: web
//         port: "8080"
//              ^
```

### Example 2: ValidationError for Nested Fields

```rust
let error = ValidationError::new("spec.template.spec.containers[0].image", "invalid image tag")
    .with_line(42);

println!("{}", error);
// Output: 42: validation error at 'spec.template.spec.containers[0].image': invalid image tag
```

### Example 3: ParseWarning for Deprecated Fields

```rust
let warning = ParseWarning::deprecated_field("old_api", "new_api")
    .with_line(10);

println!("{}", warning);
// Output: 10: warning: field 'old_api' is deprecated, use 'new_api' instead
```

## Format Reference Table

| Error Type | Location Format | Error Label | Message Format | Example |
|------------|-----------------|--------------|-----------------|---------|
| ParseError (syntax) | `file:line:column` | `syntax error:` | Message | `config.yaml:10:5: syntax error: Missing colon` |
| ParseError (I/O) | `file` or `<unknown>` | `I/O error:` | Message | `config.yaml: I/O error: file not found` |
| ParseError (validation) | `file:line` | `validation error:` | Message | `config.yaml:15: validation error: port out of range` |
| ParseError (type mismatch) | `file:line:column` | `type mismatch at '<field>':` | Expected, got | `config.yaml:8:10: type mismatch at 'port': expected integer, got string` |
| ValidationError | `line:` or none | `validation error at '<path>':` | Message | `15: validation error at 'server.port': port must be between 1 and 65535` |
| ParseWarning | `line:` or none | `warning:` | Message | `10: warning: field 'old_api' is deprecated, use 'new_api' instead` |
| ValidationWarning | `line:` or none | `warning at '<path>':` | Message | `25: warning at 'server.timeout': value is unusually high` |

## Consistency Rules

1. **Location First**: Always start with location when available
2. **Error Type Label**: Include descriptive error type label
3. **Human-Readable Messages**: Use clear, non-technical language when possible
4. **Quoted Paths**: Field paths are always quoted in single quotes
5. **Context Separator**: Use ` - ` to separate context from main message
6. **Type Information**: For type mismatches, explicitly state expected and actual types

## Files Reference

- **Error Definitions**: `src/parsers/yaml/error.rs`
- **Result Types**: `src/parsers/yaml/types.rs`
- **Generic Traits**: `src/parsers/traits.rs`
- **Format Examples**: `tests/error_message_format_examples.rs`
- **Validation Tests**: `tests/validation_error_format_test.rs`
- **Additional Tests**: `tests/error_message_format_examples_test.rs`

## See Also

- [Error Handling Philosophy](../src/parsers/yaml/error.rs#error-handling-philosophy)
- [Error Type Examples](../tests/error_message_format_examples.rs)
- [Validation Error Tests](../tests/validation_error_format_test.rs)
