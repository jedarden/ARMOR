# Parser Configuration Implementation (bf-2laoh)

## Overview
Successfully defined comprehensive configuration options for parsing behavior in the ARMOR project.

## Implementation Details

### 1. ParserMode Enum
Defined strict vs lenient parsing modes with clear behavioral specifications:
- **Strict Mode**: Rejects malformed input, enforces all syntax rules, no type coercion
- **Lenient Mode**: Attempts error recovery, allows type coercion, ignores unknown fields

### 2. ParserConfig Struct
Created comprehensive configuration struct with the following options:
- `mode: ParserMode` - Strict vs lenient parsing
- `allow_duplicates: bool` - Allow duplicate keys in mappings
- `preserve_comments: bool` - Preserve comments in output
- `preserve_quotes: bool` - Preserve quote information
- `max_depth: usize` - Maximum nesting depth (0 = unlimited)
- `strict_types: bool` - Enforce strict type checking
- `type_constructors: HashMap<String, TypeConstructor>` - Custom type constructors
- `validation_hooks: Vec<ValidationHook>` - Custom validation hooks
- `emit_warnings: bool` - Emit warnings for recoverable errors
- `warnings_as_errors: bool` - Treat warnings as errors

### 3. Custom Type Constructor Hooks
- `TypeConstructor` struct for custom type construction logic
- `TypeConstructorFn` type signature: `fn(&str, &serde_yaml::Value) -> Result<serde_yaml::Value, String>`
- Methods for registration and invocation
- Example use cases: enum parsing, validation-rich construction, complex type assembly

### 4. Validation Hooks
- `ValidationHook` struct for field-specific validation
- `ValidationFn` type signature: `fn(&str, &serde_yaml::Value) -> Result<(), String>`
- Pattern matching support with wildcards
- Examples: port range validation, type checking

### 5. Builder Pattern
- `ParserConfigBuilder` for fluent configuration construction
- Convenience methods: `ParserConfig::strict()` and `ParserConfig::lenient()`
- All builder methods return `Self` for chaining

## Default Configuration
```rust
ParserConfig {
    mode: Lenient,           // User-friendly default
    allow_duplicates: true,  // Forgiving parsing
    preserve_comments: false, // Performance optimization
    preserve_quotes: false,   // Standard YAML behavior
    max_depth: 0,            // Unlimited nesting
    strict_types: false,     // Allow type coercion
    type_constructors: {},   // No custom constructors
    validation_hooks: [],    // No custom validation
    emit_warnings: false,   // Quiet by default
    warnings_as_errors: false, // Warnings don't fail
}
```

## Files Modified
- **src/parsers/config.rs** (new) - Main configuration implementation
- **src/parsers/mod.rs** (modified) - Re-export configuration types

## Testing
All 12 configuration tests pass:
- Parser mode display and checks
- Type constructor functionality
- Validation hook pattern matching
- Parser config defaults, strict, and lenient modes
- Constructor and validation registration
- Builder pattern usage

## Usage Examples

### Strict Mode
```rust
let config = ParserConfig::strict();
assert!(config.is_strict());
assert!(!config.allow_duplicates);
```

### Builder Pattern
```rust
let config = ParserConfig::builder()
    .mode(ParserMode::Strict)
    .allow_duplicates(false)
    .max_depth(10)
    .with_constructor("timeout", TypeConstructor::new("Duration", make_duration))
    .with_validation(ValidationHook::new("port", validate_port))
    .build();
```

### Custom Type Constructor
```rust
fn log_level_constructor(field: &str, value: &serde_yaml::Value) -> Result<serde_yaml::Value, String> {
    let s = value.as_str().ok_or("expected string")?.to_lowercase();
    let level = match s.as_str() {
        "debug" => 0,
        "info" => 1,
        "warn" | "warning" => 2,
        "error" => 3,
        _ => return Err(format!("invalid log level: {}", s)),
    };
    Ok(serde_yaml::Value::Number(level.into()))
}
```

## Acceptance Criteria Met
✓ ParserConfig type defined
✓ Strict vs lenient modes documented
✓ Custom constructor hooks specified
✓ Builder pattern implemented
✓ Default configurations documented
✓ All tests passing
