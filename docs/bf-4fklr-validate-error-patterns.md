# Validate() Error Return Patterns - ARMOR Rust Codebase

## Overview

This document catalogs all error types returned by `validate()` methods and their callers in the ARMOR Rust codebase, along with error handling patterns.

**Note:** This analysis is based on the actual Rust implementation in ARMOR, not a Go codebase.

---

## Validate() Method Implementations

### 1. `Parser<Input, Output>::validate(&self, source: Input) -> Result<(), ParseError>`
**Location:** `src/parsers/traits.rs:323`

**Error Returns:**
- `Err(ParseError)` - Wraps any parsing error that occurs during validation
  - `ParseError::Yaml(YamlParseError)` - YAML-specific parsing errors
  - `ParseError::Io(String)` - I/O errors during file reading
  - `ParseError::Validation(String)` - Validation failures
  - `ParseError::TypeMismatch {field, expected, actual}` - Type mismatches
  - `ParseError::Syntax(String)` - Syntax errors

**Implementation Pattern:**
```rust
fn validate(&self, source: Input) -> Result<(), ParseError> {
    self.parse(source)?;  // Attempt parsing and discard result
    Ok(())
}
```

**Error Wrapping:** Delegates to `parse()` method, propagates any `ParseError` directly

---

### 2. `SyntaxValidator::validate(&self, content: &str) -> ValidationResult`
**Location:** `src/parsers/yaml/syntax_validator.rs:65`

**Return Type:** `ValidationResult` struct (not `Result`)

**Return Structure:**
```rust
pub struct ValidationResult {
    pub valid: bool,
    pub errors: Vec<ValidationError>,
    pub warnings: Vec<ValidationWarning>,
}
```

**Error Returns:**
- `ValidationResult` with `valid: false` and populated `errors` vector
- Each error is a `ValidationError` containing:
  - `path: String` - Path to the invalid element
  - `message: String` - Human-readable error description
  - `code: ErrorCode` - Machine-readable error code from bf-68hqo hierarchy
  - `line: Option<usize>` - Line number (1-indexed)
  - `column: Option<usize>` - Column number (1-indexed)

**Implementation Pattern:**
```rust
pub fn validate(&self, content: &str) -> ValidationResult {
    let mut errors = Vec::new();
    let mut warnings = Vec::new();
    
    // Run validation passes
    if let Err(mut line_errors) = self.validate_indentation(line, line_num, &context) {
        errors.append(&mut line_errors);
    }
    
    ValidationResult {
        valid: errors.is_empty(),
        errors,
        warnings,
    }
}
```

**Error Wrapping:** None - returns structured `ValidationResult` directly

---

### 3. `YamlParser::validate_str(&self, content: &str) -> ValidationResult`
**Location:** `src/parsers/yaml/parser.rs` (trait implementation)

**Return Type:** `ValidationResult` struct

**Error Returns:** Delegates to `SyntaxValidator::validate()`, returns `ValidationResult`

**Implementation Pattern:**
```rust
fn validate_str(&self, content: &str) -> ValidationResult {
    let validator = if self.config.is_strict() {
        SyntaxValidator::strict()
    } else {
        SyntaxValidator::lenient()
    };
    validator.validate(content)
}
```

---

### 4. `YamlParser::validate_file(&self, path: &Path) -> ValidationResult`
**Location:** `src/parsers/yaml/parser.rs` (trait implementation)

**Return Type:** `ValidationResult` struct

**Error Returns:**
- File read errors → `ValidationResult` with I/O error in errors vector
- YAML syntax errors → Delegates to `validate_str()`

**Implementation Pattern:**
```rust
fn validate_file(&self, path: &Path) -> ValidationResult {
    let content = match std::fs::read_to_string(path) {
        Ok(content) => content,
        Err(err) => {
            return ValidationResult {
                valid: false,
                errors: vec![ValidationError::new(
                    format!("file: {}", path.display()),
                    format!("Failed to read file: {}", err)
                ).with_code(ErrorCode::IO_READ_FAILED)],
                warnings: Vec::new(),
            };
        }
    };
    self.validate_str(&content)
}
```

---

### 5. `ParserConfig::validate(&self) -> Result<(), String>`
**Location:** `src/parsers/config.rs:537`

**Return Type:** `Result<(), String>`

**Error Returns:**
- `Err(String)` - Configuration inconsistency error messages
  - "warnings_as_errors requires emit_warnings to be true"
  - "Strict mode with allow_duplicates=true is inconsistent"
  - "strict_types=true with lenient mode is inconsistent"

**Implementation Pattern:**
```rust
pub fn validate(&self) -> Result<(), String> {
    if self.warnings_as_errors && !self.emit_warnings {
        return Err("warnings_as_errors requires emit_warnings to be true".to_string());
    }
    // ... other consistency checks
    Ok(())
}
```

---

### 6. `ValidatorConfig::validate(&self) -> Result<(), String>`
**Location:** `src/parsers/config.rs:908`

**Return Type:** `Result<(), String>`

**Error Returns:**
- `Err(String)` - Configuration validation error messages
  - "Strict mode requires require_all_fields to be true"
  - "Strict mode requires disallow_unknown_fields to be true"
  - "warnings_as_errors requires emit_warnings to be true"

**Implementation Pattern:**
```rust
pub fn validate(&self) -> Result<(), String> {
    if self.mode.is_strict() && !self.require_all_fields {
        return Err("Strict mode requires require_all_fields to be true".to_string());
    }
    // ... other consistency checks
    Ok(())
}
```

---

### 7. `ValidationHook::validate(&self, field: &str, value: &Value) -> Result<(), String>`
**Location:** `src/parsers/config.rs:255`

**Return Type:** `Result<(), String>`

**Error Returns:**
- `Err(String)` - Custom validation error messages from user-provided validation functions

**Implementation Pattern:**
```rust
pub fn validate(&self, field: &str, value: &Value) -> Result<(), String> {
    (self.validator)(field, value)  // Calls user-provided validation function
}
```

---

## Error Type Hierarchy

### Primary Error Types

```
ParseError (enum)
├── Yaml(YamlParseError)        // YAML-specific errors
├── Io(String)                   // I/O errors
├── Validation(String)           // Validation errors
├── TypeMismatch {field, expected, actual}  // Type mismatches
├── Syntax(String)               // Syntax errors
└── Other(String)                 // Other errors

ValidationResult (struct)
├── valid: bool
├── errors: Vec<ValidationError>
└── warnings: Vec<ValidationWarning>

ValidationError (struct)
├── path: String
├── message: String
├── code: ErrorCode              // Machine-readable error codes
├── line: Option<usize>
├── column: Option<usize>
└── indentation_error_type: Option<IndentationErrorType>
└── delimiter_error_type: Option<DelimiterErrorType>
```

### Error Code Types (ErrorCode enum)

**Syntax Errors:**
- `YAML_INVALID_SYNTAX`
- `YAML_INVALID_INDENTATION`
- `YAML_INVALID_DELIMITER`
- `YAML_INVALID_ESCAPE_SEQUENCE`
- `YAML_INVALID_SCALAR`

**Type Mismatches:**
- `TYPE_EXPECTED_INTEGER`
- `TYPE_EXPECTED_STRING`
- `TYPE_EXPECTED_BOOLEAN`
- `TYPE_EXPECTED_ARRAY`
- `TYPE_EXPECTED_OBJECT`
- `TYPE_EXPECTED_NUMBER`
- `TYPE_UNEXPECTED_NULL`

**Validation Errors:**
- `VALIDATION_REQUIRED_FIELD_MISSING`
- `VALIDATION_VALUE_OUT_OF_RANGE`
- `VALIDATION_STRING_TOO_SHORT`
- `VALIDATION_STRING_TOO_LONG`
- `VALIDATION_PATTERN_MISMATCH`
- `VALIDATION_INVALID_VALUE`
- `VALIDATION_ARRAY_TOO_FEW_ITEMS`
- `VALIDATION_ARRAY_TOO_MANY_ITEMS`
- `VALIDATION_ARRAY_NOT_UNIQUE`
- `VALIDATION_OBJECT_TOO_FEW_PROPERTIES`
- `VALIDATION_OBJECT_TOO_MANY_PROPERTIES`

**I/O Errors:**
- `IO_FILE_NOT_FOUND`
- `IO_PERMISSION_DENIED`
- `IO_READ_FAILED`
- `IO_WRITE_FAILED`

**Other Errors:**
- `ENCODING_INVALID_UTF8`
- `ANCHOR_UNKNOWN`
- `KEY_DUPLICATE`
- `EOF_UNEXPECTED`

---

## Error Handling Patterns in Callers

### Pattern 1: Direct Error Propagation

```rust
// In parser trait implementations
fn validate(&self, source: Input) -> Result<(), ParseError> {
    self.parse(source)?;  // Use ? operator for propagation
    Ok(())
}
```

**Used by:** Generic parser trait implementations

---

### Pattern 2: Structured Result Conversion

```rust
// Converting file read errors to ValidationResult
let content = match std::fs::read_to_string(path) {
    Ok(content) => content,
    Err(err) => {
        return ValidationResult {
            valid: false,
            errors: vec![ValidationError::new(
                format!("file: {}", path.display()),
                format!("Failed to read file: {}", err)
            ).with_code(ErrorCode::IO_READ_FAILED)],
            warnings: Vec::new(),
        };
    }
};
```

**Used by:** `YamlParser::validate_file()`

---

### Pattern 3: Error Collection with Context

```rust
// Collecting multiple validation errors
let mut errors = Vec::new();

if let Err(mut line_errors) = self.validate_indentation(line, line_num, &context) {
    errors.append(&mut line_errors);
}

if let Err(mut line_errors) = self.validate_delimiters(line, line_num) {
    errors.append(&mut line_errors);
}

ValidationResult {
    valid: errors.is_empty(),
    errors,
    warnings,
}
```

**Used by:** `SyntaxValidator::validate()`

---

### Pattern 4: Conditional Error Creation

```rust
// String-based error returns for configuration validation
pub fn validate(&self) -> Result<(), String> {
    if self.warnings_as_errors && !self.emit_warnings {
        return Err("warnings_as_errors requires emit_warnings to be true".to_string());
    }
    Ok(())
}
```

**Used by:** `ParserConfig::validate()`, `ValidatorConfig::validate()`

---

### Pattern 5: User-Provided Validation Functions

```rust
// Delegating to custom validation logic
pub fn validate(&self, field: &str, value: &Value) -> Result<(), String> {
    (self.validator)(field, value)  // Calls user function
}

// Example user function
fn validate_port(field: &str, value: &Value) -> Result<(), String> {
    let port = value.as_i64().ok_or("port must be an integer")?;
    if !(1..=65535).contains(&port) {
        return Err(format!("port {} out of valid range (1-65535)", port));
    }
    Ok(())
}
```

**Used by:** `ValidationHook::validate()`, `TypeConstructor::construct()`

---

## Error Construction Patterns

### Pattern 1: Enum Constructor Functions

```rust
// ParseError constructors
ParseError::syntax("invalid YAML syntax")
ParseError::io("file not found")
ParseError::validation("value out of range")
ParseError::type_mismatch("port", "integer", "string")
```

**Location:** `src/parsers/traits.rs` (via `ParseError` enum methods)

---

### Pattern 2: Struct Builder Pattern

```rust
// ValidationError construction with builder methods
ValidationError::new("server.port", "port out of range")
    .with_code(ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE)
    .with_line(42)
    .with_column(10)
```

**Location:** `src/parsers/yaml/types.rs`

---

### Pattern 3: Result Struct Direct Construction

```rust
// ValidationResult construction
ValidationResult {
    valid: false,
    errors: vec![error1, error2],
    warnings: vec![warning1],
}

// Convenience constructors
ValidationResult::success()
ValidationResult::failure(errors)
```

**Location:** `src/parsers/yaml/types.rs`

---

### Pattern 4: Error Type Conversion

```rust
// Converting std::io::Error to ParseError
impl From<std::io::Error> for ParseError {
    fn from(err: std::io::Error) -> Self {
        Self::Io(err.to_string())
    }
}

// Converting YamlParseError to ParseError
impl From<YamlParseError> for ParseError {
    fn from(err: YamlParseError) -> Self {
        Self::Yaml(err)
    }
}
```

**Location:** `src/parsers/traits.rs`

---

## Summary Table

| Validate Method | Return Type | Error Types | Wrapping | Construction Pattern |
|----------------|-------------|-------------|----------|---------------------|
| `Parser::validate` | `Result<(), ParseError>` | `ParseError` enum variants | Delegates to `parse()` | Enum constructors |
| `SyntaxValidator::validate` | `ValidationResult` | `ValidationError` struct | None - returns struct | Struct builder pattern |
| `YamlParser::validate_str` | `ValidationResult` | `ValidationError` struct | Delegates to `SyntaxValidator` | Inherited |
| `YamlParser::validate_file` | `ValidationResult` | `ValidationError` struct | Converts I/O errors | Struct construction |
| `ParserConfig::validate` | `Result<(), String>` | String error messages | None - direct returns | String formatting |
| `ValidatorConfig::validate` | `Result<(), String>` | String error messages | None - direct returns | String formatting |
| `ValidationHook::validate` | `Result<(), String>` | User-provided errors | Delegates to user function | User-defined |

---

## Key Observations

### 1. Dual Return Pattern Philosophy

ARMOR uses two distinct error handling patterns:

- **`Result<T, E>` pattern**: Used for parsing operations where failure is exceptional and stops execution
- **Structured result pattern**: Used for validation operations where multiple errors should be collected and reported together

This allows both simple error checking (via `?` operator) and comprehensive error reporting.

### 2. No Error Wrapping in Validation Results

Validation methods that return `ValidationResult` do NOT use Rust's standard error wrapping (`?` operator, `From` conversions). Instead, they construct result structs directly with error lists.

### 3. Error Code Hierarchy Integration

The `ErrorCode` enum provides machine-readable error codes that map to the `ErrorType` categories, enabling programmatic error handling and classification.

### 4. Rich Location Information

`ValidationError` includes optional line/column numbers and specialized error type fields (indentation_error_type, delimiter_error_type) for detailed error reporting.

### 5. User-Extensible Validation

The `ValidationHook` and `TypeConstructor` systems allow users to provide custom validation logic that integrates with the standard error handling patterns.

### 6. Consistent Error Display

All error types implement `Display` for user-friendly error messages and `Error` trait for compatibility with Rust error handling.

---

## Generated: 2026-07-12
## ARMOR Rust Codebase Analysis
