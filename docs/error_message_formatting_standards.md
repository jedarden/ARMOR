# ARMOR Error Message Formatting Standards

## Overview

This document defines the standard error message formatting patterns used throughout the ARMOR codebase. All error types MUST follow these patterns to ensure consistency, readability, and maintainability.

## Core Principles

1. **Human-Readable First**: Error messages should be immediately understandable by users without consulting documentation
2. **Context-Rich**: Include location, field paths, and relevant constraints
3. **Actionable**: When possible, suggest what the user should do to fix the error
4. **Consistent**: All errors follow the same structural patterns
5. **Machine-Parseable**: Structured formats for logging and programmatic analysis

## Standard Format Patterns

### 1. ParseError Summary Format

**Pattern**: `<location>: <error-kind>: <message> - <context>`

**Components**:
- `location`: File path and line/column information (e.g., `config.yaml:10:5`)
- `error-kind`: Error category (e.g., `syntax error`, `validation error`)
- `message`: Brief description of the specific error
- `context`: Additional context (optional, separated by ` - `)

**Examples**:
```
config.yaml:10:5: syntax error: Missing colon - while parsing service definition
<unknown>: validation error: port out of range
database.yaml:15:12: type mismatch at 'server.port': expected integer, got string
```

### 2. Location String Patterns

The location string adapts based on available information:

| Available Information | Format | Example |
|----------------------|--------|---------|
| Path + Line + Column | `path:line:column` | `config.yaml:10:5` |
| Path + Line | `path:line` | `config.yaml:10` |
| Path only | `path` | `config.yaml` |
| Line + Column only | `line:column` | `10:5` |
| Line only | `line` | `10` |
| Column only | `col N` | `col 5` |
| None | `<unknown>` | `<unknown>` |

### 3. Error Kind Formats

Each error kind has a specific display format:

| Error Kind | Format | Example |
|------------|--------|---------|
| Syntax | `syntax error: <message>` | `syntax error: invalid YAML indentation` |
| I/O | `I/O error: <message>` | `I/O error: file not found` |
| Validation | `validation error: <message>` | `validation error: port out of range` |
| Type Mismatch | `type mismatch at '<field>': expected <expected>, got <actual>` | `type mismatch at 'port': expected integer, got string` |
| Unexpected EOF | `unexpected end of input` | `unexpected end of input` |
| Invalid UTF-8 | `invalid UTF-8 encoding` | `invalid UTF-8 encoding` |
| Unknown Anchor | `unknown anchor: <name>` | `unknown anchor: ref` |
| Duplicate Key | `duplicate key: <key>` | `duplicate key: name` |
| Other | `error: <message>` | `error: unclassified error` |

### 4. ValidationError Format

**Pattern**: `<line>: validation error at '<path>': <message>`

**Components**:
- `line`: Line number (optional)
- `path`: Field path in dot-notation or bracket-notation
- `message`: Validation failure description

**Examples**:
```
42: validation error at 'server.port': port must be between 1 and 65535
15: validation error at 'database.connectionPool.maxConnections': pool size must be positive
```

### 5. Field Path Patterns

Field paths use consistent notation:

| Path Type | Pattern | Example |
|-----------|---------|---------|
| Simple | `fieldname` | `port` |
| Nested | `parent.child` | `server.port` |
| Deep nested | `grandparent.parent.child` | `database.connectionPool.maxConnections` |
| Array access | `array[index]` | `servers[0]` |
| Array field | `array[index].field` | `servers[0].port` |
| Kubernetes-style | `spec.template.spec.containers[0].image` | `spec.template.spec.containers[0].image` |

### 6. Constraint Information Pattern

**Pattern**: `(constraint: <details>)`

**Examples**:
```
validation error: value out of range (constraint: must be between 1-65535)
validation error: invalid format (constraint: must match ^[a-z]+$ pattern)
```

### 7. Detailed Report Format

For user-facing errors with code snippets:

```
error: <location>: <error-kind>: <message> - <context>
  context: <context-message>

  snippet:
    <code line 1>
    <code line 2>
    <pointer with ^>
```

**Example**:
```
error: config.yaml:5:10: syntax error: Invalid escape sequence
  context: while parsing field 'description'

  snippet:
    description: "Product \x name"
           ^
```

## Error Message Writing Guidelines

### DO ✓

- **Use clear, non-technical language**: "port must be between 1 and 65535" not "port constraint violation"
- **Include the actual value**: "expected integer, got string" not "type mismatch"
- **Specify valid ranges**: "must be between 1 and 65535" not "out of range"
- **Reference the field path**: "at field 'server.port'" not "validation error"
- **Provide context**: "while parsing service configuration" not just "error"

### DON'T ✗

- **Use cryptic abbreviations**: "IO err" not "I/O error: file not found"
- **Omit field information**: "validation failed" not "validation error: port must be between 1 and 65535"
- **Use vague language**: "something went wrong" - always specify what
- **Forget location**: Always include file/line information when available
- **Skip constraint details**: If there's a constraint, document it

## Standardized Error Messages

### Common Validation Errors

| Scenario | Standard Message |
|----------|------------------|
| Port out of range | `port must be between 1 and 65535` |
| Missing required field | `missing required field: '<field>'` |
| Invalid type | `expected <expected>, got <actual>` |
| Empty value | `field cannot be empty` |
| Invalid format | `value does not match required format: <pattern>` |
| Out of range | `value must be between <min> and <max>` |
| Too short | `value must be at least <n> characters` |
| Too long | `value must be at most <n> characters` |

### Common Type Mismatches

| Expected | Actual | Standard Message |
|----------|--------|------------------|
| integer | string | `expected integer, got string` |
| string | integer | `expected string, got integer` |
| boolean | string | `expected boolean, got string` |
| array | scalar | `expected array, got scalar value` |
| object | string | `expected object, got string` |

### Common Syntax Errors

| Scenario | Standard Message |
|----------|------------------|
| Missing colon | `missing colon after key` |
| Invalid indentation | `invalid YAML indentation` |
| Invalid escape sequence | `invalid escape sequence in string` |
| Unclosed quote | `unclosed quote` |
| Invalid character | `invalid character in YAML stream` |

### Common I/O Errors

| Scenario | Standard Message |
|----------|------------------|
| File not found | `file not found: <path>` |
| Permission denied | `permission denied: <operation> on <path>` |
| Read failure | `failed to read file: <reason>` |
| Write failure | `failed to write file: <reason>` |

## Error Creation Patterns

### Using Convenience Constructors

```rust
// Syntax errors
ParseError::syntax("invalid YAML indentation")

// Validation errors
ParseError::validation("port must be between 1 and 65535")

// Type mismatches
ParseError::type_mismatch("port", "integer", "string")

// I/O errors
ParseError::io("file not found: config.yaml")
```

### Using Builder Pattern

```rust
ParseError::validation("port must be between 1 and 65535")
    .with_path("config.yaml")
    .with_line(15)
    .with_column(10)
    .with_context("at field server.port (constraint: must be between 1-65535)")
    .with_snippet("server:\n  port: 70000")
```

## Testing and Verification

### Test Coverage Requirements

All error formats must have test coverage in `tests/error_message_format_examples.rs`:

1. **Location variations**: Test all location string patterns
2. **Error kind formats**: Test all error kind display formats
3. **Field path patterns**: Test simple, nested, and array paths
4. **Constraint formatting**: Test constraint information display
5. **Real-world scenarios**: Test realistic error cases
6. **Consistency checks**: Verify format consistency across error types

### Format Verification

Run the error format tests to verify compliance:

```bash
cargo test error_message_format
cargo test validation_error_format
cargo test parse_error_
```

## Structured Output Formats

### Summary Format (Single-line)

For logging and compact output:
```rust
let summary = error.summary();
// Output: "config.yaml:10:5: syntax error: invalid token - while parsing"
```

### Detailed Report Format (Multi-line)

For user-facing display:
```rust
let report = error.detailed_report();
// Output: Multi-line report with snippet and visual indicator
```

### Structured Format (Machine-parseable)

For programmatic analysis:
```rust
let structured = error.format_structured();
// Output: "ParseError { kind: ..., location: ..., line: ..., column: ... }"
```

## When to Use Each Error Kind

### ParseErrorKind Selection Guide

| Situation | Use This Kind |
|-----------|---------------|
| File system errors | `Io` |
| YAML grammar violations | `Syntax` |
| Value constraint violations | `Validation` |
| Wrong type for value | `TypeMismatch` |
| Incomplete input | `UnexpectedEof` |
| Encoding errors | `InvalidUtf8` |
| Unknown anchor reference | `UnknownAnchor` |
| Duplicate mapping keys | `DuplicateKey` |
| Unclassifiable errors | `Other` |

## Migration Guide

### Converting Old Error Messages

If you find error messages that don't conform to these standards:

1. **Check the standard patterns**: Find the matching pattern in this document
2. **Use convenience constructors**: Prefer `ParseError::syntax()`, `ParseError::validation()`, etc.
3. **Add context**: Use `.with_context()` to provide additional information
4. **Include location**: Use `.with_path()`, `.with_line()`, `.with_column()`
5. **Add constraints**: Document constraints in the message or context
6. **Test updates**: Add test cases to verify the new format

## References

- **Implementation**: `src/parsers/yaml/error.rs` (937 lines)
- **Test Documentation**: `tests/error_message_format_examples.rs` (574 lines)
- **Error Types**: `src/parsers/yaml/types.rs` (754 lines)

## Version History

- **v1.0** (2026-07-11): Initial standardization based on existing ARMOR error handling patterns

---

*This document is a living standard. Update it when new error patterns are introduced or when existing patterns are improved.*