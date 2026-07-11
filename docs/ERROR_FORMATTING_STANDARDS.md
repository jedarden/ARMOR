# ARMOR Error Message Formatting Standards

This document defines the standard error message formatting patterns for all error types in the ARMOR codebase, covering both Rust and Go implementations.

## Table of Contents

1. [Core Principles](#core-principles)
2. [Rust Error Formatting Standards](#rust-error-formatting-standards)
3. [Go Error Formatting Standards](#go-error-formatting-standards)
4. [Cross-Language Consistency](#cross-language-consistency)
5. [Examples](#examples)

---

## Core Principles

### 1. Human-Readability First
- Error messages must be clear and actionable
- Avoid technical jargon when possible
- Include context about what operation failed
- Provide hints for resolution when applicable

### 2. Structured Information
Error messages should include, in order:
1. **Error Type/Category** - What kind of error occurred
2. **Location** - Where the error occurred (file, line, column, field path)
3. **Message** - Human-readable description of the problem
4. **Context** - What operation was being performed (optional but recommended)
5. **Details** - Expected vs. actual values, constraints violated (optional)

### 3. Location Information Hierarchy
Location information should be formatted as follows, from most specific to least:
- `file:line:column` - Most specific (preferred when available)
- `file:line` - When column unknown
- `file` - When line/column unknown
- `line:column` - When file unknown
- `line` - When only line available
- `<unknown>` - When no location info available

### 4. Field Path Convention
Use dot-notation for field paths:
- Simple: `server.port`
- Nested: `database.connectionPool.maxConnections`
- Array access: `servers.api.responses[0].statusCode`
- Kubernetes-style: `spec.template.spec.containers[0].image`

### 5. Type Mismatch Format
Type mismatches should always show:
- What field has the error
- What type was expected
- What type was actually found

Format: `expected <type>, got <type>` or `expected <type>, found <type>`

---

## Rust Error Formatting Standards

### ParseError (src/parsers/yaml/error.rs)

#### Location Formatting
```rust
// location_string() method patterns:
"config.yaml:10:5"      // path + line + column (most specific)
"config.yaml:10"        // path + line
"config.yaml::5"         // path + column (rare)
"config.yaml"           // path only
"10:5"                  // line + column only
"10"                    // line only
"col 5"                 // column only (rare)
"<unknown>"             // no location info
```

#### Summary Format
```rust
// summary() method patterns:
"<location>: <error-kind>: <message>"              // without context
"<location>: <error-kind>: <message> - <context>"  // with context
```

#### Error Kind Display Formats
```rust
// ParseErrorKind Display implementations:
"syntax error: <message>"
"I/O error: <message>"
"validation error: <message>"
"type mismatch at '<field>': expected <expected>, got <actual>"
"unexpected end of input"
"invalid UTF-8 encoding"
"unknown anchor: <name>"
"duplicate key: <key>"
"error: <message>"  // catch-all Other variant
```

#### Detailed Report Format
```rust
// detailed_report() method structure:
"error: <summary>\n" +
"  context: <context-message>\n" +
"\n  snippet:\n" +
"    <code-line-1>\n" +
"    <code-line-2>\n" +
"    <caret-position>\n"
```

### ValidationError (src/parsers/yaml/types.rs)

#### Display Format
```rust
// With line: "<line>: validation error at '<path>': <message>"
// Without line: "validation error at '<path>': <message>"
```

#### Construction Pattern
```rust
ValidationError::new("server.port", "port must be between 1 and 65535")
    .with_line(15)
```

#### Example Outputs
```
// Full format with line:
"15: validation error at 'server.port': port must be between 1 and 65535"

// Minimal format without line:
"validation error at 'database.connectionPool.maxConnections': connection pool too small"
```

### ValidationWarning (src/parsers/yaml/types.rs)

#### Display Format
```rust
// With line: "<line>: warning at '<path>': <message>"
// Without line: "warning at '<path>': <message>"
```

### ParseWarning (src/parsers/yaml/types.rs)

#### Warning Kind Formats
```rust
// ParseWarningKind Display implementations:
"warning: field '<old_field>' is deprecated, use '<new_field>' instead"
"warning: unknown key '<key>'"
"warning: duplicate key '<key>'"
```

---

## Go Error Formatting Standards

### YAMLError Interface (internal/yamlutil/errors.go)

All Go error types implement the `YAMLError` interface with:
- `Code() ErrorCode` - Machine-readable error code
- `YAMLErrorType() ErrorType` - Error category for type switching
- `Context() string` - Additional context about the error

### ParseError (internal/yamlutil/errors.go)

#### Error Format
```go
// With line and column:
"parse error in <file> at line <line>, column <column>: <message> (expected: <expected>, actual: <actual>)"

// With line only:
"parse error in <file> at line <line>: <message> (expected: <expected>, actual: <actual>)"

// Without location:
"parse error in <file>: <message> (expected: <expected>, actual: <actual>)"
```

#### String Format (for debugging)
```go
"  Error: <message>\n" +
"  Type: <error-type>\n" +
"  Location: Line <line>, Column <column>\n" +
"  Field: <field-path>\n" +
"  Constraint: <constraint>\n" +
"  Expected Type: <expected-type>\n" +
"  Actual Type: <actual-type>\n" +
"  Context: <context>\n"
```

### ValidationError (internal/yamlutil/errors.go)

#### Error Format
```go
// With line and column:
"validation error in <file> at line <line>, column <column> at field <field-path>: <message> (constraint: <constraint>) (expected <expected-type>, got <actual-type>)"

// With line only:
"validation error in <file> at line <line> at field <field-path>: <message> (constraint: <constraint>) (expected <expected-type>, got <actual-type>)"

// Without location:
"validation error in <file> at field <field-path>: <message> (constraint: <constraint>) (expected <expected-type>, got <actual-type>)"
```

#### String Format (for debugging)
```go
"  Error: <message>\n" +
"  Type: <error-type>\n" +
"  Location: Line <line>, Column <column>\n" +
"  Field: <field-path>\n" +
"  Constraint: <constraint>\n" +
"  Expected Type: <expected-type>\n" +
"  Actual Type: <actual-type>\n" +
"  Context: <context>\n"
```

### FileError (internal/yamlutil/errors.go)

#### Error Format
```go
// With operation:
"file error during <operation> on <path>: <message>: <underlying-error>"

// Without operation:
"file error in <path>: <message>: <underlying-error>"
```

### SyntaxError (internal/yamlutil/errors.go)

#### Error Format
```go
// With location:
"syntax error in <file> at line <line>, column <column>: <message>"

// Without location:
"syntax error in <file>: <message>"
```

### TypeMismatchError (internal/yamlutil/errors.go)

#### Error Format
```go
// With line:
"type mismatch in <file> at line <line>, field <field-path>: expected <expected-type>, got <actual-type>"

// Without line:
"type mismatch in <file>, field <field-path>: expected <expected-type>, got <actual-type>"
```

### FieldNotFoundError (internal/yamlutil/errors.go)

#### Error Format
```go
// With line:
"required field missing in <file> at line <line>: <field-path>"

// Without line:
"required field missing in <file>: <field-path>"
```

### ConstraintError (internal/yamlutil/errors.go)

#### Error Format
```go
// With line:
"constraint violation in <file> at line <line>, field <field-path>: <message>"

// Without line:
"constraint violation in <file>, field <field-path>: <message>"
```

### DuplicateKeyError (internal/yamlutil/errors.go)

#### Error Format
```go
// With line numbers:
"duplicate key error in <file> at line <line2>: key \"<key>\" already defined at line <line1>"

// Without line numbers:
"duplicate key error in <file>: key \"<key>\""
```

### StructureError (internal/yamlutil/errors.go)

#### Error Format
```go
// With line:
"structure error in <file> at line <line>: <message>"

// Without line:
"structure error in <file>: <message>"
```

### SchemaLoadError (internal/yamlutil/errors.go)

#### Error Format
```go
"schema load error in <file>: <message>"
```

### SchemaValidationError (internal/yamlutil/errors.go)

#### Error Format
```go
// With line:
"schema validation error in <file> at line <line>: <message>"

// Without line:
"schema validation error in <file>: <message>"
```

---

## Cross-Language Consistency

### Consistent Elements Across Rust and Go

1. **Error Category Prefix**
   - Rust: `"syntax error:"`, `"validation error:"`, `"I/O error:"`
   - Go: `"syntax error in"`, `"validation error in"`, `"file error"`
   - **Standard**: Use lowercase category with "error:" suffix

2. **Location Format**
   - Both: `file:line:column` or `file:line` when column unavailable
   - **Standard**: Always include line when available

3. **Field Path Format**
   - Both: Dot-notation for nested fields
   - **Standard**: Use `'field.path'` format in quotes

4. **Type Mismatch**
   - Rust: `"expected <type>, got <type>"`
   - Go: `"expected <type>, got <type>"` (consistent!)
   - **Standard**: Always use "got" not "found" or "actual"

5. **Constraint Information**
   - Rust: `"(constraint: <constraint>)"`
   - Go: `"(constraint: <constraint>)"`
   - **Standard**: Always include constraint in parentheses

### Inconsistencies to Address

1. **Location Preposition**
   - Rust: `"config.yaml:10:5: syntax error:"`
   - Go: `"syntax error in config.yaml at line 10, column 5:"`
   - **Resolution**: Accept language-specific conventions (differing grammatical structure)

2. **Context Formatting**
   - Rust: `" - <context>"` suffix
   - Go: Inline in message or separate field
   - **Resolution**: Both are valid for their language idioms

---

## Examples

### Rust Examples

#### ParseError with Full Context
```rust
let error = ParseError::syntax("Missing colon")
    .with_path("config.yaml")
    .with_line(10)
    .with_column(5)
    .with_context("while parsing service definition");

// Display output:
// "config.yaml:10:5: syntax error: Missing colon - while parsing service definition"

// Detailed report:
// error: config.yaml:10:5: syntax error: Missing colon - while parsing service definition
//   context: while parsing service definition
//
//   snippet:
//     service: name: web
//          ^
```

#### ValidationError with Field Path
```rust
let error = ValidationError::new("server.port", "port must be between 1 and 65535")
    .with_line(15);

// Display output:
// "15: validation error at 'server.port': port must be between 1 and 65535"
```

#### Type Mismatch Error
```rust
let error = ParseError::type_mismatch("database.port", "integer", "string")
    .with_path("config.yaml")
    .with_line(20);

// Display output:
// "config.yaml:20: type mismatch at 'database.port': expected integer, got string"
```

### Go Examples

#### ParseError with Full Context
```go
err := NewParseError("config.yaml", "Missing colon", 10, 5, ErrCodeInvalidSyntax, "identifier", "123")

// Error() output:
// "parse error in config.yaml at line 10, column 5: Missing colon (expected: identifier, actual: 123)"

// String() output:
//   Error: Missing colon
//   Type: parse
//   Location: Line 10, Column 5
```

#### ValidationError with Field Path and Type Info
```go
err := NewValidationError("config.yaml", "port must be between 1 and 65535", "server.port", 
    "must be between 1-65535", ErrCodeInvalidValue, 15, 0, "", "")

// Error() output (with ExpectedType and ActualType set):
// "validation error in config.yaml at line 15 at field server.port: port must be between 1 and 65535 (constraint: must be between 1-65535) (expected integer, got string)"
```

#### TypeMismatchError
```go
err := NewTypeMismatchError("config.yaml", "database.port", "integer", "string", "abc", 20, ErrCodeTypeMismatch)

// Error() output:
// "type mismatch in config.yaml at line 20, field database.port: expected integer, got string"
```

---

## Testing and Verification

All error message formats should be tested with comprehensive examples. See:
- `tests/error_message_format_examples.rs` - Rust error format tests
- `tests/validation_error_format_test.rs` - Validation error format tests
- `internal/yamlutil/errors_test.go` - Go error format tests

When adding new error types or modifying existing formats:
1. Add test cases documenting the expected format
2. Include examples of all format variations (with/without location, etc.)
3. Verify output is human-readable and actionable
4. Ensure cross-language consistency where applicable
