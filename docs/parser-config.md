# Parser Configuration Guide

This document describes the comprehensive configuration system for controlling parsing behavior in ARMOR.

## Overview

The `ParserConfig` struct provides fine-grained control over parsing behavior across different parser implementations. It supports:

- **Strict vs Lenient modes** - Control how strictly the parser enforces rules
- **Custom type constructors** - Register hooks for building complex types
- **Custom validation hooks** - Add application-specific validation
- **Builder pattern** - Fluent configuration API

## Module Location

```rust
use armor::parsers::config::{
    ParserConfig, ParserMode, ParserConfigBuilder,
    TypeConstructor, ValidationHook
};
```

## ParserMode

The `ParserMode` enum defines the strictness level for parsing operations.

### Strict Mode

Rejects any malformed or unexpected input:

- **Unknown fields** → Parse error
- **Type mismatches** → Error (no coercion)
- **Duplicate keys** → Rejected
- **Missing required fields** → Error
- **All syntax rules** → Strictly enforced

### Lenient Mode (Default)

Attempts to recover from errors:

- **Unknown fields** → Silently ignored
- **Type mismatches** → Coerced when possible (e.g., string → number)
- **Duplicate keys** → Last wins (with optional warning)
- **Missing optional fields** → Use defaults
- **Syntax variations** → Accepted when possible

```rust
// Strict mode example
let config = ParserConfig::builder()
    .mode(ParserMode::Strict)
    .build();

// Lenient mode example (also available as convenience method)
let config = ParserConfig::lenient();
```

## ParserConfig Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `mode` | `ParserMode` | `Lenient` | Strict vs lenient parsing |
| `allow_duplicates` | `bool` | `true` | Allow duplicate keys in mappings |
| `preserve_comments` | `bool` | `false` | Preserve comments in output |
| `preserve_quotes` | `bool` | `false` | Preserve quote information |
| `max_depth` | `usize` | `0` | Maximum nesting depth (0 = unlimited) |
| `strict_types` | `bool` | `false` | Enforce strict type checking |
| `type_constructors` | `HashMap` | empty | Custom type constructors by field |
| `validation_hooks` | `Vec` | empty | Custom validation hooks |
| `emit_warnings` | `bool` | `false` | Emit warnings for recoverable errors |
| `warnings_as_errors` | `bool` | `false` | Treat warnings as errors |

## Predefined Configurations

### Default Configuration

```rust
let config = ParserConfig::default();
```

Equivalent to:

```rust
ParserConfig {
    mode: ParserMode::Lenient,
    allow_duplicates: true,
    preserve_comments: false,
    preserve_quotes: false,
    max_depth: 0,           // unlimited
    strict_types: false,
    type_constructors: HashMap::new(),
    validation_hooks: Vec::new(),
    emit_warnings: false,
    warnings_as_errors: false,
}
```

### Strict Configuration

```rust
let config = ParserConfig::strict();
```

Equivalent to:

```rust
ParserConfig::builder()
    .mode(ParserMode::Strict)
    .allow_duplicates(false)
    .strict_types(true)
    .build()
```

### Lenient Configuration

```rust
let config = ParserConfig::lenient();
```

Equivalent to:

```rust
ParserConfig::builder()
    .mode(ParserMode::Lenient)
    .allow_duplicates(true)
    .strict_types(false)
    .build()
```

## Builder Pattern

The builder pattern provides a fluent interface for creating configurations:

```rust
let config = ParserConfig::builder()
    .mode(ParserMode::Strict)
    .allow_duplicates(false)
    .max_depth(10)
    .preserve_comments(true)
    .strict_types(true)
    .emit_warnings(true)
    .build();
```

## Custom Type Constructors

Type constructors allow you to register custom logic for constructing specific types during parsing. This is useful for:

- **Custom enum parsing** (e.g., `"warn"` → `LogLevel::Warning`)
- **Validation-rich construction** (e.g., ensure ports are in range)
- **Complex type assembly** (e.g., `Duration` from `"5s"` string)
- **Default value injection** for optional fields

### Example: Duration Constructor

```rust
use armor::parsers::config::{ParserConfig, TypeConstructor};
use serde_yaml::Value;

fn parse_duration(
    field: &str,
    value: &Value
) -> Result<Value, String> {
    let s = value.as_str()
        .ok_or("duration must be a string")?;

    // Parse "5s", "100ms", etc.
    let (num_str, unit) = if let Some(pos) = s.find(|c: char| !c.is_numeric()) {
        (&s[..pos], &s[pos..])
    } else {
        return Err(format!("invalid duration format: {}", s));
    };

    let num: u64 = num_str.parse()
        .map_err(|_| format!("invalid number: {}", num_str))?;

    let millis = match unit {
        "ms" => num,
        "s" => num * 1_000,
        "m" => num * 60_000,
        "h" => num * 3_600_000,
        _ => return Err(format!("unknown unit: {}", unit)),
    };

    // Return as number (milliseconds)
    Ok(Value::Number(millis.into()))
}

let mut config = ParserConfig::default();
config.register_constructor(
    "timeout",
    TypeConstructor::new("Duration", parse_duration)
);
```

### Example: LogLevel Constructor

```rust
use armor::parsers::config::TypeConstructor;
use serde_yaml::Value;

fn parse_log_level(
    field: &str,
    value: &Value
) -> Result<Value, String> {
    let s = value.as_str()
        .ok_or("log level must be a string")?
        .to_lowercase();

    let level = match s.as_str() {
        "debug" => 0,
        "info" => 1,
        "warn" | "warning" => 2,
        "error" => 3,
        _ => return Err(format!("invalid log level: {}", s)),
    };

    Ok(Value::Number(level.into()))
}

let mut config = ParserConfig::default();
config.register_constructor(
    "log_level",
    TypeConstructor::new("LogLevel", parse_log_level)
);
```

## Custom Validation Hooks

Validation hooks allow you to register custom validation logic for specific fields or types.

### Example: Port Range Validation

```rust
use armor::parsers::config::{ParserConfig, ValidationHook};
use serde_yaml::Value;

fn validate_port(
    field: &str,
    value: &Value
) -> Result<(), String> {
    let port = value.as_i64()
        .ok_or("port must be an integer")?;

    if !(1..=65535).contains(&port) {
        return Err(format!(
            "port {} out of valid range (1-65535)",
            port
        ));
    }

    Ok(())
}

let mut config = ParserConfig::default();
config.register_validation(
    ValidationHook::new("port", validate_port)
);
```

### Example: Wildcard Pattern Matching

```rust
// Apply validation to all fields matching "port_*"
config.register_validation(
    ValidationHook::new("port_*", validate_port)
);

// Now validates: port_http, port_https, port_admin, etc.

// Universal validation for all fields
config.register_validation(
    ValidationHook::new("*", universal_validator)
);
```

## Complete Example

```rust
use armor::parsers::config::{
    ParserConfig, ParserMode, TypeConstructor, ValidationHook
};
use serde_yaml::Value;

fn main() {
    // Custom constructor for timeouts
    let timeout_constructor = TypeConstructor::new("Duration", |field, value| {
        let s = value.as_str().ok_or("must be string")?;
        Ok(Value::Number(1000.into())) // Simplified example
    });

    // Custom validator for ports
    let port_validator = ValidationHook::new("port_*", |field, value| {
        let port = value.as_i64().ok_or("must be integer")?;
        if !(1..=65535).contains(&port) {
            return Err(format!("port {} out of range", port));
        }
        Ok(())
    });

    // Build comprehensive configuration
    let config = ParserConfig::builder()
        .mode(ParserMode::Lenient)
        .allow_duplicates(false)
        .max_depth(20)
        .preserve_comments(true)
        .strict_types(false)
        .emit_warnings(true)
        .with_constructor("timeout", timeout_constructor)
        .with_validation(port_validator)
        .build();

    // Use with parser
    let parser = armor::parsers::yaml::Parser::new(config);
    // ...
}
```

## Migration from Legacy Config

The old `ParserConfig` (from `yaml` module) has been superseded by the new `parsers::config::ParserConfig`. Migration guide:

### Before (Legacy)

```rust
use armor::parsers::yaml::ParserConfig;

let config = ParserConfig {
    strict_mode: false,
    allow_duplicates: true,
    preserve_quotes: false,
};
```

### After (New)

```rust
use armor::parsers::config::{ParserConfig, ParserMode};

let config = ParserConfig::builder()
    .mode(ParserMode::Lenient)
    .allow_duplicates(true)
    .preserve_quotes(false)
    .build();
```

Or use the convenience method:

```rust
let config = ParserConfig::lenient();
```

## Best Practices

1. **Start with defaults** - Use `ParserConfig::default()` or `ParserConfig::lenient()` for most cases
2. **Enable strict mode for configs** - Use `ParserConfig::strict()` when parsing user-provided configs
3. **Register constructors early** - Register all type constructors before parsing
4. **Use wildcard patterns** - Leverage `"field_*"` patterns for related fields
5. **Validate after construction** - Use validation hooks to catch errors early

## Thread Safety

`ParserConfig` is `Clone` and can be safely shared across threads:

```rust
use std::sync::Arc;

let config = Arc::new(ParserConfig::strict());
// Share config across threads
```
