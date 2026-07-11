# YAML Parser Configuration Guide

This guide provides comprehensive documentation for the YAML parser configuration options in ARMOR. The configuration system allows fine-grained control over parsing behavior, validation, and error handling.

## Overview

The YAML parser configuration is managed through the [`ParserConfig`] structure, which provides:

- **Parsing modes**: Strict vs lenient parsing strategies
- **Validation hooks**: Custom validation logic for specific fields
- **Type constructors**: Custom type conversion logic
- **Error handling**: Warning emission and error treatment
- **Formatting options**: Comment and quote preservation

## Core Configuration Structures

### ParserConfig

The main configuration structure that consolidates all parsing options:

```rust
pub struct ParserConfig {
    /// Parsing mode (strict vs lenient)
    pub mode: ParserMode,
    
    /// Allow duplicate keys in mappings
    pub allow_duplicates: bool,
    
    /// Preserve comments in the output (if format supports it)
    pub preserve_comments: bool,
    
    /// Preserve quote information in parsed strings
    pub preserve_quotes: bool,
    
    /// Maximum nesting depth (0 = unlimited)
    pub max_depth: usize,
    
    /// Enforce strict type checking (no implicit coercion)
    pub strict_types: bool,
    
    /// Custom type constructors registered by field name
    pub type_constructors: HashMap<String, TypeConstructor>,
    
    /// Custom validation hooks
    pub validation_hooks: Vec<ValidationHook>,
    
    /// Emit warnings for recoverable errors
    pub emit_warnings: bool,
    
    /// Treat warnings as errors (fail on warnings)
    pub warnings_as_errors: bool,
}
```

## Configuration Options

### 1. ParserMode (Strict vs Lenient)

The `ParserMode` enum defines the overall parsing strategy:

#### Strict Mode (`ParserMode::Strict`)

**Purpose**: Reject any malformed or unexpected input with zero tolerance for deviations.

**Behaviors**:
- Unknown fields cause parsing to fail
- Type mismatches are errors (not coerced)
- Duplicate keys are rejected
- All syntax rules are enforced
- Missing required fields cause errors
- No implicit type conversion
- Exact matching of schema definitions

**Use Cases**:
- Configuration files where schema compliance is critical
- Security-sensitive parsing where unexpected input is dangerous
- Production environments where data integrity is paramount
- API contract validation

**Example**:
```rust
use armor::parsers::config::ParserConfig;

let config = ParserConfig::strict();
// Equivalent to:
// ParserConfig::builder()
//     .mode(ParserMode::Strict)
//     .allow_duplicates(false)
//     .strict_types(true)
//     .build()
```

#### Lenient Mode (`ParserMode::Lenient`)

**Purpose**: Attempt to recover from errors and process as much valid input as possible.

**Behaviors**:
- Unknown fields are silently ignored
- Type mismatches are coerced when possible (e.g., string "42" → number 42)
- Last duplicate key wins (with optional warning)
- Some syntax variations are accepted
- Missing optional fields use defaults
- Implicit type conversion enabled
- Forward-compatible with schema additions

**Use Cases**:
- User-provided configuration where graceful degradation is desired
- Development environments where flexibility is more important than strictness
- Migration scenarios where schemas may evolve
- Prototyping and testing

**Example**:
```rust
use armor::parsers::config::ParserConfig;

let config = ParserConfig::lenient();
// Equivalent to:
// ParserConfig::builder()
//     .mode(ParserMode::Lenient)
//     .allow_duplicates(true)
//     .strict_types(false)
//     .build()
```

### 2. Duplicate Key Handling (`allow_duplicates`)

**Default**: `true` in lenient mode, `false` in strict mode

**Purpose**: Control behavior when duplicate keys are encountered in YAML mappings.

**Options**:

- **`true`**: Accept duplicate keys, using the last value encountered
  - May emit warning if `emit_warnings` is enabled
  - Useful for user configs where override behavior is desired
  
- **`false`**: Reject duplicate keys with an error
  - Ensures data integrity
  - Prevents accidental overwrites

**Interaction with ParserMode**:
- In strict mode, this should typically be `false`
- In lenient mode, `true` allows for more flexible configurations

**Example**:
```rust
// YAML with duplicate keys
// yaml_content = r#"
// port: 8080
// port: 9090  # This will be used if allow_duplicates = true
// "#;

let config = ParserConfig::builder()
    .allow_duplicates(true)
    .build();
```

### 3. Type Strictness (`strict_types`)

**Default**: `false` in lenient mode, `true` in strict mode

**Purpose**: Control whether implicit type coercion is allowed.

**Behaviors**:

- **`true`**: No implicit type conversion
  - String "42" will NOT convert to number 42
  - Type must exactly match schema expectation
  - Prevents subtle type-related bugs
  
- **`false`**: Best-effort type coercion
  - String "42" converts to number 42
  - String "true" converts to boolean true
  - More forgiving of input variations

**Common Coercions** (when `strict_types = false`):
- Numeric strings → numbers
- Boolean strings → booleans
- Null strings → null
- Empty strings → defaults (if available)

**Example**:
```rust
// strict_types: true
let input = "timeout: 30s";  // Error: cannot convert "30s" to integer

// strict_types: false  
let input = "timeout: 30s";  // Error: still fails (non-numeric string)

let input = "timeout: 30";   // Success: string "30" coerced to number 30
```

### 4. Maximum Nesting Depth (`max_depth`)

**Default**: `0` (unlimited)

**Purpose**: Prevent deep recursion attacks and stack overflow vulnerabilities.

**Security Consideration**: Setting a reasonable depth limit is crucial for:
- Preventing malicious YAML from causing stack overflow
- Protecting against exponential parsing time
- Ensuring predictable memory usage

**Recommended Values**:
- User-provided YAML: 10-20 levels
- Trusted configuration: 0 (unlimited) or 50+
- Network-facing applications: 5-10 levels

**Example**:
```rust
let config = ParserConfig::builder()
    .max_depth(15)  // Allow up to 15 levels of nesting
    .build();

// This would fail with max_depth=5:
// deeply:
//   nested:
//     structure:
//       exceeds:
//         limit:
//           by:
//             one:
//               level: true
```

### 5. Comment Preservation (`preserve_comments`)

**Default**: `false`

**Purpose**: Control whether comments are retained during parsing.

**Behaviors**:
- **`true`**: Comments are preserved in the parsed AST
  - Useful for round-trip editing
  - Maintains documentation in configuration
  - Increases memory usage
  
- **`false`**: Comments are discarded during parsing
  - Faster parsing
  - Lower memory footprint
  - Loses contextual information

**Limitations**: Comment preservation depends on the underlying YAML library support. Not all YAML parsers support comment retention.

**Example**:
```rust
let config = ParserConfig::builder()
    .preserve_comments(true)
    .build();

// Input YAML with comment:
// server:
//   port: 8080  # HTTP port
//   host: localhost  # Server hostname

// With preserve_comments=true, comments are retained in the AST
```

### 6. Quote Preservation (`preserve_quotes`)

**Default**: `false`

**Purpose**: Control whether quote style information is preserved for string values.

**Behaviors**:
- **`true`**: Distinguishes between plain, single-quoted, and double-quoted strings
  - Useful for precise round-trip conversion
  - Preserves stylistic choices
  
- **`false`**: All string values normalized to plain strings
  - Simpler data model
  - Easier to work with programmatically

**Example**:
```rust
let config = ParserConfig::builder()
    .preserve_quotes(true)
    .build();

// These would be distinct with preserve_quotes=true:
// plain: hello
// single: 'hello'
// double: "hello"
```

### 7. Warning Emission (`emit_warnings`)

**Default**: `false`

**Purpose**: Control whether non-fatal issues generate warnings.

**What Generates Warnings**:
- Duplicate keys (when `allow_duplicates = true`)
- Unknown fields (in lenient mode)
- Type coercion (when `strict_types = false`)
- Deprecated syntax or fields
- Minor schema deviations

**Example**:
```rust
let config = ParserConfig::builder()
    .emit_warnings(true)
    .build();

// Would emit warning for unknown field:
// server:
//   port: 8080
//   unknown_field: value  # Warning: unknown field 'unknown_field'
```

### 8. Warnings as Errors (`warnings_as_errors`)

**Default**: `false`

**Purpose**: Treat warnings as errors, causing parsing to fail on any warning.

**Use Cases**:
- CI/CD pipelines where all warnings should be treated as failures
- Strict compliance environments
- Migration phases where deprecated features must be eliminated

**Interaction with `emit_warnings`**:
- If `emit_warnings = false`, this setting has no effect
- If both are `true`, any warning causes immediate parse failure

**Example**:
```rust
let config = ParserConfig::builder()
    .emit_warnings(true)
    .warnings_as_errors(true)
    .build();

// Any duplicate key, unknown field, or type coercion would cause failure
```

## Custom Type Constructors

Type constructors allow custom logic for converting raw YAML values into specific types.

### Purpose

- **Custom enum parsing**: Convert string representations to enums (e.g., "warn" → LogLevel::Warning)
- **Validation-rich construction**: Ensure values meet constraints during construction
- **Complex type assembly**: Build complex types from simple representations (e.g., Duration from "5s")
- **Default value injection**: Supply defaults for optional fields

### TypeConstructor Structure

```rust
pub struct TypeConstructor {
    pub type_name: String,
    pub constructor: TypeConstructorFn,
}

// TypeConstructorFn signature:
pub type TypeConstructorFn = fn(&str, &serde_yaml::Value) -> Result<serde_yaml::Value, String>;
```

### Example: Log Level Constructor

```rust
use armor::parsers::config::TypeConstructor;

fn log_level_constructor(
    field: &str,
    value: &serde_yaml::Value
) -> Result<serde_yaml::Value, String> {
    let s = value.as_str()
        .ok_or("expected string")?
        .to_lowercase();

    let level = match s.as_str() {
        "debug" => 0,
        "info" => 1,
        "warn" | "warning" => 2,
        "error" => 3,
        _ => return Err(format!("invalid log level: {}", s)),
    };

    Ok(serde_yaml::Value::Number(level.into()))
}

// Usage
let mut config = ParserConfig::default();
config.register_constructor(
    "log_level",
    TypeConstructor::new("LogLevel", log_level_constructor)
);
```

### Example: Duration Constructor

```rust
fn duration_constructor(
    field: &str,
    value: &serde_yaml::Value
) -> Result<serde_yaml::Value, String> {
    let s = value.as_str()
        .ok_or("duration must be a string")?;
    
    let (num_str, unit) = s.split_at(s.len() - 1);
    let magnitude: u64 = num_str.parse()
        .map_err(|_| format!("invalid duration: {}", s))?;
    
    let seconds = match unit {
        "s" => magnitude,
        "m" => magnitude * 60,
        "h" => magnitude * 3600,
        _ => return Err(format!("unknown duration unit: {}", unit)),
    };
    
    Ok(serde_yaml::Value::Number(seconds.into()))
}

// Usage
let mut config = ParserConfig::default();
config.register_constructor(
    "timeout",
    TypeConstructor::new("Duration", duration_constructor)
);
```

## Custom Validation Hooks

Validation hooks allow custom validation logic for specific fields after parsing.

### Purpose

- **Range validation**: Ensure numeric values are within valid ranges (e.g., ports 1-65535)
- **Format validation**: Check string formats (e.g., email addresses, URLs)
- **Business logic validation**: Enforce application-specific constraints
- **Cross-field validation**: Validate relationships between fields

### ValidationHook Structure

```rust
pub struct ValidationHook {
    pub field_pattern: String,  // Supports "*" wildcard
    pub validator: ValidationFn,
}

// ValidationFn signature:
pub type ValidationFn = fn(&str, &serde_yaml::Value) -> Result<(), String>;
```

### Pattern Matching

Validation hooks support wildcard pattern matching:

- **Exact match**: `"port"` matches only field named "port"
- **Prefix match**: `"port_*"` matches "port_http", "port_https", etc.
- **Universal match**: `"*"` matches all fields

### Example: Port Range Validation

```rust
use armor::parsers::config::ValidationHook;

fn validate_port(
    field: &str,
    value: &serde_yaml::Value
) -> Result<(), String> {
    let port = value.as_i64()
        .ok_or("port must be an integer")?;

    if !(1..=65535).contains(&port) {
        return Err(format!("port {} out of valid range (1-65535)", port));
    }

    Ok(())
}

// Usage
let mut config = ParserConfig::default();
config.register_validation(
    ValidationHook::new("port", validate_port)
);

// This would also apply to port_http, port_https, etc.:
config.register_validation(
    ValidationHook::new("port_*", validate_port)
);
```

### Example: URL Format Validation

```rust
fn validate_url(
    field: &str,
    value: &serde_yaml::Value
) -> Result<(), String> {
    let url_str = value.as_str()
        .ok_or("URL must be a string")?;
    
    if !url_str.starts_with("http://") && !url_str.starts_with("https://") {
        return Err(format!("URL must start with http:// or https://, got: {}", url_str));
    }
    
    Ok(())
}

// Usage
let mut config = ParserConfig::default();
config.register_validation(
    ValidationHook::new("*_url", validate_url)
);
```

## Option Interactions

### Strict Mode Cascading

When `mode = ParserMode::Strict`, related options are typically set:

```rust
// Strict mode automatically implies:
ParserConfig::strict()
// Sets:
// - mode: Strict
// - allow_duplicates: false
// - strict_types: true
```

### Warning-Error Interaction

```rust
// For strict compliance:
let config = ParserConfig::builder()
    .emit_warnings(true)        // Enable warnings
    .warnings_as_errors(true)   // Treat them as errors
    .mode(ParserMode::Strict)
    .build();
```

### Type Coercion and Lenient Mode

```rust
// Lenient mode with type coercion (default):
let config = ParserConfig::lenient();
// mode: Lenient
// strict_types: false  → String "42" coerces to number 42

// Lenient mode without type coercion:
let config = ParserConfig::builder()
    .mode(ParserMode::Lenient)
    .strict_types(true)  // → String "42" fails as type mismatch
    .build();
```

### Depth Limits and Strict Mode

```rust
// Recommended for untrusted YAML:
let config = ParserConfig::builder()
    .mode(ParserMode::Strict)
    .max_depth(10)  // Prevent deep nesting attacks
    .warnings_as_errors(true)
    .build();
```

## Validation Rules

### Default Value Validation

The following validation rules apply to configuration options:

| Option | Type | Validation |
|--------|------|------------|
| `mode` | `ParserMode` | Must be `Strict` or `Lenient` |
| `allow_duplicates` | `bool` | No validation |
| `preserve_comments` | `bool` | No validation (may be unsupported by parser) |
| `preserve_quotes` | `bool` | No validation (may be unsupported by parser) |
| `max_depth` | `usize` | 0 means unlimited; >0 imposes limit |
| `strict_types` | `bool` | No validation |
| `emit_warnings` | `bool` | No validation |
| `warnings_as_errors` | `bool` | Requires `emit_warnings = true` to be meaningful |

### Constructor Registration Validation

- Field names must be unique (last registration wins)
- Constructor functions must return `Result<serde_yaml::Value, String>`
- Empty field names are invalid

### Validation Hook Registration Validation

- Field patterns must be valid wildcard patterns
- Validation functions must return `Result<(), String>`
- Multiple hooks can apply to the same field (all are evaluated)

## Builder Pattern

The `ParserConfigBuilder` provides a fluent interface for constructing configurations:

```rust
use armor::parsers::config::{ParserConfig, ParserMode, ValidationHook, TypeConstructor};

let config = ParserConfig::builder()
    .mode(ParserMode::Strict)
    .allow_duplicates(false)
    .max_depth(10)
    .preserve_comments(true)
    .strict_types(true)
    .emit_warnings(true)
    .warnings_as_errors(false)
    .with_constructor("timeout", timeout_constructor)
    .with_validation(port_validation)
    .build();
```

## Best Practices

### For User-Provided Configuration

```rust
let config = ParserConfig::builder()
    .mode(ParserMode::Lenient)           // Be forgiving
    .allow_duplicates(true)               // Allow overrides
    .strict_types(false)                  // Coerce types
    .max_depth(10)                         // Limit depth
    .emit_warnings(true)                   // Warn about issues
    .warnings_as_errors(false)             // Don't fail on warnings
    .build();
```

### For Production/Security-Critical Parsing

```rust
let config = ParserConfig::builder()
    .mode(ParserMode::Strict)             // Strict validation
    .allow_duplicates(false)              // No duplicates
    .strict_types(true)                   // No coercion
    .max_depth(5)                          // Low depth limit
    .emit_warnings(true)                   // Warn on issues
    .warnings_as_errors(true)             // Fail on warnings
    .build();
```

### For Development/Testing

```rust
let config = ParserConfig::builder()
    .mode(ParserMode::Lenient)           // Flexible parsing
    .allow_duplicates(true)              // Allow duplicates
    .strict_types(false)                  // Allow coercion
    .max_depth(0)                         // No depth limit
    .preserve_comments(true)              // Keep comments
    .build();
```

## Migration Guide

### From Default to Strict

```rust
// Before
let config = ParserConfig::default();

// After
let config = ParserConfig::strict();
```

### Adding Validation

```rust
// Before
let config = ParserConfig::default();

// After
let mut config = ParserConfig::default();
config.register_validation(ValidationHook::new("port", validate_port));
```

### Enabling Strict Types

```rust
// Before
let config = ParserConfig::lenient();

// After
let config = ParserConfig::builder()
    .mode(ParserMode::Lenient)
    .strict_types(true)
    .build();
```

## Troubleshooting

### Common Issues

**Issue**: Parser rejects valid YAML with duplicate keys

**Solution**: Set `allow_duplicates = true` if duplicate keys are intentional

```rust
let config = ParserConfig::builder()
    .allow_duplicates(true)
    .build();
```

**Issue**: Type conversion fails unexpectedly

**Solution**: Set `strict_types = false` to enable type coercion

```rust
let config = ParserConfig::builder()
    .strict_types(false)
    .build();
```

**Issue**: Parsing fails on unknown fields

**Solution**: Use lenient mode to ignore unknown fields

```rust
let config = ParserConfig::builder()
    .mode(ParserMode::Lenient)
    .build();
```

**Issue**: Stack overflow on deeply nested YAML

**Solution**: Set `max_depth` to a reasonable value

```rust
let config = ParserConfig::builder()
    .max_depth(10)
    .build();
```

## API Reference

### Convenience Constructors

- `ParserConfig::default()` - Default (lenient) configuration
- `ParserConfig::strict()` - Pre-configured strict mode
- `ParserConfig::lenient()` - Pre-configured lenient mode
- `ParserConfig::builder()` - Start builder pattern

### Configuration Methods

- `is_strict()` - Check if in strict mode
- `is_lenient()` - Check if in lenient mode
- `register_constructor(field, constructor)` - Register type constructor
- `register_validation(hook)` - Register validation hook
- `get_constructor(field)` - Get constructor for field
- `get_validations(field)` - Get applicable validation hooks

## See Also

- [`ParserConfig` struct documentation](../src/parsers/config.rs)
- [Parser trait documentation](parser-trait.md)
- [ParseError handling](parse-error.md)
- [Type constructors guide](type-constructors.md)
- [Validation hooks guide](validation-hooks.md)