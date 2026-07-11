# ParseResult<T> Type Design Documentation

## Overview

The `ParseResult<T>` type represents the outcome of YAML parsing operations, encapsulating either successful parsing with associated data and metadata, or failure with detailed error information. This document provides a comprehensive design specification for the type.

## Type Parameter

**`<T>`** - The type of the parsed value

- Typically the target type that YAML content is being parsed into
- Examples: configuration structs, `serde_yaml::Value`, or any deserializable type
- Enables type-safe parsing with compile-time guarantees

## Structure and Fields

### Core Fields

| Field | Type | Purpose | Presence |
|-------|------|---------|----------|
| `value` | `Option<T>` | The successfully parsed value | Present only on success |
| `error` | `Option<ParseError>` | Detailed error information | Present only on failure |
| `metadata` | `ParseMetadata` | Metadata about the parsing operation | Always present |
| `warnings` | `Vec<ParseWarning>` | Non-fatal warnings | May be present on success or failure |

### Field Relationships

```
Success State:  value = Some(T), error = None,  warnings = [...]
Failure State:  value = None,    error = Some(E), warnings = [...]
```

Key invariants:
- `value` and `error` are mutually exclusive (one is `Some`, the other is `None`)
- `metadata` is always present
- `warnings` may be empty or non-empty in either state

## Field Details

### value: Option<T>

The successfully parsed value when parsing succeeds.

**Characteristics:**
- `Some(T)` when parsing succeeds
- `None` when parsing fails
- The type `T` is determined by the parsing operation

**Access patterns:**
```rust
// Safe access
if let Some(v) = result.value() {
    // Use v
}

// Unwrap with panic on failure
let v = result.unwrap();

// Unwrap with default
let v = result.unwrap_or(default_value);

// Transform the value
let mapped: ParseResult<U> = result.map(|t| transform(t));
```

### error: Option<ParseError>

Detailed error information when parsing fails.

**Characteristics:**
- `Some(ParseError)` when parsing fails
- `None` when parsing succeeds
- Contains structured error information: kind, location, context, snippet

**Access patterns:**
```rust
// Check for error
if result.is_failure() {
    if let Some(e) = result.error() {
        println!("Error: {}", e);
    }
}

// Error types
error.is_syntax()      // Syntax errors
error.is_io()          // I/O errors
error.is_validation()  // Validation errors
error.is_type_mismatch()  // Type mismatches
```

### metadata: ParseMetadata

Metadata about the parsing operation, always present.

**Fields:**
- `lines_processed: usize` - Number of lines processed
- `bytes_processed: usize` - Number of bytes processed
- `processing_time_ns: Option<u64>` - Processing time in nanoseconds
- `source_path: Option<String>` - Source file path if known

**Purpose:**
- Performance tracking and monitoring
- Debugging and diagnostics
- Audit trails for parsing operations

**Access patterns:**
```rust
let meta = result.metadata();
println!("Processed {} lines", meta.lines_processed);
if let Some(path) = &meta.source_path {
    println!("Source: {}", path);
}
```

### warnings: Vec<ParseWarning>

Non-fatal warnings that occurred during parsing.

**Characteristics:**
- May be non-empty even when parsing succeeds
- Indicates deprecated usage, potential issues, or informational messages
- Does not prevent parsing from completing successfully

**Warning Types:**
- `DeprecatedField` - Field is deprecated, use alternative
- `UnknownKey` - Unknown key encountered (lenient mode)
- `DuplicateKey` - Duplicate key handled (lenient mode)

**Access patterns:**
```rust
// Check for warnings
if result.has_warnings() {
    for warning in result.warnings() {
        println!("Warning: {}", warning);
    }
}

// Add warnings programmatically
let mut result = ParseResult::success(value);
result.add_warning(ParseWarning::deprecated_field("old", "new"));
```

## Constructors

### success(value: T) -> Self

Creates a successful `ParseResult` with the given value.

**Post-conditions:**
- `value() == Some(value)`
- `error() == None`
- `warnings().is_empty()`
- `metadata()` has default values

### failure(error: ParseError) -> Self

Creates a failed `ParseResult` with the given error.

**Post-conditions:**
- `value() == None`
- `error() == Some(error)`
- `warnings().is_empty()`
- `metadata()` has default values

### from(Result<T>) -> Self (via From impl)

Converts from a standard `Result<T, ParseError>` to `ParseResult<T>`.

**Behavior:**
- `Ok(value)` → `ParseResult::success(value)`
- `Err(error)` → `ParseResult::failure(error)`

## Query Methods

### State Queries

| Method | Returns | Description |
|--------|---------|-------------|
| `is_success()` | `bool` | `true` if parsing succeeded (value present, error absent) |
| `is_failure()` | `bool` | `true` if parsing failed (error present, value absent) |
| `has_warnings()` | `bool` | `true` if warnings are present |

### Accessor Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `value()` | `Option<&T>` | Reference to the parsed value, if successful |
| `error()` | `Option<&ParseError>` | Reference to the error, if failed |
| `metadata()` | `&ParseMetadata` | Reference to the metadata (always present) |
| `warnings()` | `&[ParseWarning]` | Slice of warnings |

## Manipulation Methods

### Warning Management

| Method | Purpose |
|--------|---------|
| `add_warning(warning)` | Add a single warning |
| `add_warnings(iter)` | Add multiple warnings from an iterator |

### Value Extraction

| Method | Purpose | Behavior on Failure |
|--------|---------|---------------------|
| `unwrap()` | Extract the value, consuming the result | **Panics** |
| `unwrap_or(default)` | Extract the value or return default | Returns default |

### Transformation

| Method | Purpose | Preserves |
|--------|---------|-----------|
| `map(f)` | Transform the success value to a new type | Error, metadata, warnings |
| `with_metadata(metadata)` | Set the metadata | Value, error, warnings |

## Example Usage Patterns

### Basic Success Handling

```rust
let result: ParseResult<MyConfig> = parser.parse_file("config.yaml")?;

if result.is_success() {
    let config = result.unwrap();
    // Use config
}
```

### Error Handling with Context

```rust
match result {
    r if r.is_success() => {
        let value = r.unwrap();
        // Handle success
    }
    r if r.is_failure() => {
        if let Some(error) = r.error() {
            eprintln!("Parse error: {}", error);
        }
    }
    _ => unreachable!(),
}
```

### Handling Warnings

```rust
let result = parser.parse_str(yaml_content)?;

if result.has_warnings() {
    for warning in result.warnings() {
        println!("Warning: {}", warning);
    }
}

if result.is_success() {
    let value = result.unwrap();
    // Use value despite warnings
}
```

### Chaining with map()

```rust
let result: ParseResult<serde_yaml::Value> = parser.parse_str(yaml)?;
let config: ParseResult<MyConfig> = result.map(|v| serde_from_value(v))?;
```

## Design Rationale

### Why `Option<T>` and `Option<ParseError>`?

**Alternative considered:** `enum ParseResult<T> { Success(T), Failure(ParseError) }`

**Trade-off decision:** Using `Option` for both fields provides:
1. Clearer field access patterns (`result.value()` vs match statements)
2. Easier to add additional fields (metadata, warnings) without changing the enum structure
3. More flexible for future extensions (e.g., partial success states)

**Future-proofing:** The current design allows for potential expansion into states like "partial success" where both value and warnings are present but no error.

### Why separate `warnings` from `error`?

**Design principle:** Warnings are non-fatal; errors are fatal.

- A parse can succeed with warnings (e.g., deprecated fields)
- A parse cannot succeed with errors (errors indicate failure)
- This separation enables clear semantics: `is_success()` checks only error, not warnings

### Why generic `<T>` instead of `serde_yaml::Value`?

**Flexibility:** The generic parameter allows:
1. Direct parsing to target types (e.g., `ParseResult<MyConfig>`)
2. Intermediate generic parsing (`ParseResult<serde_yaml::Value>`)
3. Type-safe transformations via `map()`

**Type safety:** Compile-time guarantees that the parsed value matches the expected type.

## Related Types

### ParseWarning

Type for non-fatal warnings. See the `ParseWarning` and `ParseWarningKind` documentation for details.

### ParseError

Type for fatal errors. See the `ParseError` and `ParseErrorKind` documentation for details.

### ParseMetadata

Type for metadata about parsing operations. See the `ParseMetadata` documentation for details.

### ValidationResult

Separate type for validation-only operations (no parsed value, only errors and warnings).

## Testing Considerations

When testing `ParseResult`-based code:

1. **Test both success and failure paths**
2. **Test warning generation and handling**
3. **Test metadata population**
4. **Test `map()` transformations preserve warnings/errors**
5. **Test `unwrap()` panic behavior**

Example test structure:
```rust
#[test]
fn test_parse_with_warnings() {
    let mut result = ParseResult::success(42);
    result.add_warning(ParseWarning::deprecated_field("old", "new"));

    assert!(result.is_success());
    assert!(result.has_warnings());
    assert_eq!(result.warnings().len(), 1);
    assert_eq!(result.unwrap(), 42);
}
```

## Migration Notes

If migrating from a previous version without `warnings`:

1. All existing `ParseResult::success()` and `ParseResult::failure()` calls remain compatible
2. Add warning handling where appropriate:
   ```rust
   if result.has_warnings() {
       for warning in result.warnings() {
           // Handle or log warnings
       }
   }
   ```
3. Update documentation to reflect that successful parses may now have warnings
