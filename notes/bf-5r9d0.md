# ParseResult<T> Type Design Documentation

## Overview

`ParseResult<T>` is the core result type for YAML parsing operations in the ARMOR project. It represents successful parsing outcomes with rich context including metadata, warnings, and the parsed value.

## Type Structure

### Generic Type Parameter `<T>`

- **Purpose**: Represents the type of the successfully parsed value
- **Usage**: Can be any deserializable type, including:
  - `serde_yaml::Value` for generic YAML parsing
  - Custom configuration structs for typed parsing
  - Any type implementing `serde::Deserialize`

### Core Fields

```rust
pub struct ParseResult<T> {
    /// The successfully parsed value (Some if success, None if failure)
    value: Option<T>,

    /// Detailed error information (Some if failure, None if success)
    error: Option<ParseError>,

    /// Metadata about the parsing operation (always present)
    metadata: ParseMetadata,

    /// Non-fatal warnings (empty if none, can be present even during success)
    warnings: Vec<ParseWarning>,
}
```

## Design Philosophy

The `ParseResult<T>` type follows four key principles:

### 1. Explicit Success/Failure States

- **Success**: Requires `value` present AND `error` absent
- **Failure**: Requires `error` present AND `value` absent
- This prevents ambiguous states where both fields could be `None`

### 2. Rich Context

Both success and failure outcomes carry metadata:
- **ParseMetadata**: Lines processed, bytes processed, processing time, source path
- **Warnings**: Non-fatal issues that don't prevent successful parsing

### 3. Warnings Without Failure

Parsing can succeed even with warnings present:
- **Deprecated field usage**: Using old field names that should be migrated
- **Unknown keys**: In lenient mode, unrecognized keys are ignored with a warning
- **Type coercion**: Automatic type conversions that may lose precision
- **Duplicate keys**: Last-write-wins behavior with warning

### 4. Composable Operations

Supports functional composition patterns:
- **`map()`**: Transform success values to new types
- **`From<Result<T>>`**: Interoperability with standard Rust `Result` types
- **Builder pattern**: `with_metadata()` for chaining metadata updates

## Field Purposes

### `value: Option<T>`

- **Purpose**: Holds the successfully parsed value
- **When present**: Parsing succeeded, value is valid
- **When absent**: Parsing failed, check `error` field for details
- **Access methods**: `value()`, `unwrap()`, `unwrap_or()`

### `error: Option<ParseError>`

- **Purpose**: Holds detailed error information for failed parses
- **When present**: Parsing failed, contains structured error details
- **When absent**: Parsing succeeded
- **Access methods**: `error()`, `is_failure()`

### `metadata: ParseMetadata`

- **Purpose**: Provides context about the parsing operation
- **Always present**: Never `None`, always contains operation metadata
- **Fields**:
  - `lines_processed`: Number of YAML lines processed
  - `bytes_processed`: Total bytes read
  - `processing_time_ns`: Duration of parse operation (nanoseconds)
  - `source_path`: Original file path (if applicable)
- **Access methods**: `metadata()`, `with_metadata()`

### `warnings: Vec<ParseWarning>`

- **Purpose**: Non-fatal issues that occurred during parsing
- **Can be present**: Even when parsing succeeds
- **Warning categories**:
  - `DeprecatedField`: Old field should be replaced
  - `UnknownKey`: Unrecognized key in lenient mode
  - `DuplicateKey`: Last-write-wins with warning
- **Access methods**: `warnings()`, `has_warnings()`, `add_warning()`

## Constructor Methods

### `ParseResult::success(value: T)`

Creates a successful parse result:
```rust
let result = ParseResult::success(42);
```

- Sets `value` to `Some(value)`
- Sets `error` to `None`
- Initializes `metadata` with defaults
- Initializes `warnings` as empty vector

### `ParseResult::failure(error: ParseError)`

Creates a failed parse result:
```rust
let result = ParseResult::<i32>::failure(ParseError::syntax("invalid YAML"));
```

- Sets `value` to `None`
- Sets `error` to `Some(error)`
- Initializes `metadata` with defaults
- Initializes `warnings` as empty vector

## Query Methods

### State Queries

- **`is_success()`**: Returns `true` if value present and error absent
- **`is_failure()`**: Returns `true` if error present
- **`has_warnings()`**: Returns `true` if warnings vector non-empty

### Accessor Methods

- **`value()`**: Returns `Option<&T>` - reference to parsed value
- **`error()`**: Returns `Option<&ParseError>` - reference to error
- **`warnings()`**: Returns `&[ParseWarning]` - slice of warnings
- **`metadata()`**: Returns `&ParseMetadata` - reference to metadata

## Transformation Methods

### `map<U, F>(self, f: F) -> ParseResult<U>`

Transforms the success value to a new type:
```rust
let result: ParseResult<i32> = ParseResult::success(10);
let doubled = result.map(|x| x * 2);
```

- **On success**: Applies function to value, preserves metadata and warnings
- **On failure**: Preserves error and metadata, returns new `ParseResult<U>`

### `with_metadata(self, metadata: ParseMetadata) -> Self`

Sets metadata via builder pattern:
```rust
let result = ParseResult::success(config)
    .with_metadata(ParseMetadata::new()
        .with_lines(100)
        .with_bytes(4096)
        .with_source("config.yaml"));
```

## Unwrap Methods

### `unwrap(self) -> T`

Consumes the result and returns the value:
```rust
let value = result.unwrap();
```

- **Panics**: If called on failed parse result
- **Returns**: The parsed value on success

### `unwrap_or(self, default: T) -> T`

Consumes the result and returns value or default:
```rust
let value = result.unwrap_or(0);
```

- **On success**: Returns the parsed value
- **On failure**: Returns the provided default

## Warning Management

### Adding Warnings

```rust
// Add a single warning
result.add_warning(ParseWarning::deprecated_field("old", "new"));

// Add multiple warnings
result.add_warnings(vec![
    ParseWarning::unknown_key("unknown_field"),
    ParseWarning::duplicate_key("config"),
]);
```

Warnings are non-fatal and don't affect success/failure state.

### ParseWarning Types

1. **DeprecatedField**: Field migration guidance
   ```rust
   ParseWarning::deprecated_field("old_api", "new_api")
   ```

2. **UnknownKey**: Lenient mode unknown key handling
   ```rust
   ParseWarning::unknown_key("unknown_field")
   ```

3. **DuplicateKey**: Duplicate key detection
   ```rust
   ParseWarning::duplicate_key("config_key")
   ```

## Interoperability

### From `Result<T>`

Automatic conversion from standard `Result<T, ParseError>`:
```rust
let std_result: Result<T, ParseError> = ...;
let parse_result: ParseResult<T> = std_result.into();
```

### Conversion to `Result<T>`

Manual conversion when needed:
```rust
let parse_result: ParseResult<T> = ...;
let std_result: Result<T, ParseError> = match parse_result.error {
    None => Ok(parse_result.unwrap()),
    Some(err) => Err(err),
};
```

## Usage Examples

### Successful Parse

```rust
use armor::parsers::yaml::{ParseResult, ParseMetadata};

let result = ParseResult::success(42);
assert!(result.is_success());
assert_eq!(result.value(), Some(&42));
assert!(result.warnings().is_empty());
```

### Failed Parse

```rust
use armor::parsers::yaml::{ParseResult, ParseError};

let error = ParseError::syntax("invalid YAML");
let result = ParseResult::<i32>::failure(error);
assert!(result.is_failure());
assert!(result.error().is_some());
```

### Parse with Warnings

```rust
use armor::parsers::yaml::{ParseResult, ParseWarning};

let mut result = ParseResult::success(42);
result.add_warning(ParseWarning::deprecated_field("old", "new"));

assert!(result.is_success());
assert!(!result.warnings().is_empty());
```

### Mapping Operations

```rust
use armor::parsers::yaml::ParseResult;

let result: ParseResult<i32> = ParseResult::success(10);
let doubled = result.map(|x| x * 2);

assert_eq!(doubled.value(), Some(&20));
```

## Design Benefits

1. **Type Safety**: Generic `<T>` parameter provides compile-time type checking
2. **Rich Context**: Metadata and warnings provide debugging information
3. **Composability**: `map()` and `From` traits enable functional composition
4. **User Friendly**: Clear success/failure semantics without ambiguous states
5. **Flexible**: Works with any deserializable type via serde integration

## Implementation Status

✅ **Complete** - All features implemented and documented:
- Type structure with generic parameter
- Core fields (value, error, metadata, warnings)
- Constructor methods (success, failure)
- Query methods (is_success, is_failure, accessors)
- Transformation methods (map, with_metadata)
- Warning management (add_warning, add_warnings)
- Unwrap methods (unwrap, unwrap_or)
- ParseWarning type with comprehensive categories
- Full documentation with examples
- Working implementation (cargo check passes)

---
*Documentation generated for bead bf-5r9d0: Design ParseResult type for successful parsing*
