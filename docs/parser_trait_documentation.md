# Parser Trait Documentation

## Overview

The `Parser<Input, Output>` trait defines a generic interface for parsing strategies that transform input of one type into structured output. This trait is designed to be:

- **Generic** - Works with any input/output type combination
- **Composable** - Multiple parsers can be chained together
- **Extensible** - Supports streaming and incremental parsing
- **Error-aware** - Uses structured error types for consistent error handling

## Trait Definition

```rust
pub trait Parser<Input, Output> {
    /// Parse input and return the parsed result
    fn parse(&self, source: Input) -> Result<Output, ParseError>;

    /// Parse input with extended options
    fn parse_with_options(&self, source: Input, options: ParseOptions) -> Result<Output, ParseError>;

    /// Parse a file at the given path
    fn parse_file(&self, path: &Path) -> Result<Output, ParseError>;

    /// Validate input without fully parsing
    fn validate(&self, source: Input) -> Result<(), ParseError>;

    /// Get metadata about the parser
    fn metadata(&self) -> ParseMetadata;
}
```

## Method Signature Design

### Core Method: `parse()`

The primary parsing method has the following signature:

```rust
fn parse(&self, source: Input) -> Result<Output, ParseError>
```

**Design Decision: `Result<Output, ParseError>` vs `Result<ParseResult<T>, ParseError>`**

The trait uses Rust's standard `Result<Output, ParseError>` instead of the richer `ParseResult<T>` type for the following reasons:

1. **Simplicity** - The trait is format-agnostic and should work with any parser implementation
2. **Flexibility** - Implementations can choose to use `ParseResult<T>` internally if they need richer metadata
3. **Standard Rust patterns** - Uses familiar `Result` type that integrates well with Rust's error handling
4. **No overhead** - Simple cases don't need the complexity of metadata and warnings

### When to Use Each Result Type

#### Use `Result<T, ParseError>` (this trait)

- For generic parsers that work across multiple formats
- For simple parsing operations where detailed metadata isn't needed
- For standard Rust error handling patterns
- For parser chaining and composition

#### Use `ParseResult<T>` (YAML-specific)

- For YAML parsing when you need rich metadata
- When you need warning collection for non-fatal issues
- When you need detailed error reporting with location information
- For production YAML parsing where observability is important

### Converting Between Result Types

The `ParseResult<T>` type implements `From<Result<T>>` for seamless integration:

```rust
use armor::parsers::yaml::ParseResult;

// Convert Result<T, ParseError> to ParseResult<T>
let result: Result<MyType, ParseError> = parse_value();
let parse_result: ParseResult<MyType> = ParseResult::from(result);
```

## Type Parameters and Trait Bounds

### Generic Parameters

The trait uses two generic parameters:

- **`Input`** - The input source type
- **`Output`** - The parsed result type

### Common Input Types

```rust
// String-based formats (YAML, JSON, TOML)
impl Parser<&str, MyConfig> for MyParser

// Binary formats
impl Parser<&[u8], BinaryData> for MyBinaryParser

// File-based parsing
impl Parser<&Path, MyConfig> for MyFileParser
```

### Trait Bounds

The trait itself has no bounds on `Input` or `Output`, but certain methods have where clauses:

```rust
fn parse_file(&self, path: &Path) -> Result<Output, ParseError>
where
    Self: Sized,
    Input: From<String>,
```

This means `parse_file` is only available when the `Input` type can be created from a `String` (e.g., `&str`).

## Parsing Strategies

Different parser implementations can conform to the trait with different behaviors:

### 1. Strict Parsing

Strict parsing follows format specifications precisely and rejects any deviations:

```rust
use armor::parsers::{Parser, ParseOptions};

pub struct StrictYamlParser;

impl Parser<&str, serde_yaml::Value> for StrictYamlParser {
    fn parse(&self, source: &str) -> Result<serde_yaml::Value, ParseError> {
        // Parse with strict options
        let options = ParseOptions::strict();
        
        // Implementation:
        // - Unknown fields cause errors
        // - Type mismatches are rejected
        // - Duplicate keys are not allowed
        // - Format specifications are enforced strictly
        
        parse_yaml_strict(source)
    }
}

// Usage:
let parser = StrictYamlParser;
let result = parser.parse("name: test\nvalue: 42")?;
```

**Characteristics:**
- Unknown fields cause errors
- Type mismatches are rejected
- Duplicate keys are not allowed
- Format specifications are enforced strictly
- Fails fast on any deviation

### 2. Lenient Parsing

Lenient parsing is more forgiving and attempts to recover from errors:

```rust
use armor::parsers::{Parser, ParseOptions};

pub struct LenientYamlParser {
    allow_unknown_fields: bool,
    coerce_types: bool,
}

impl Parser<&str, serde_yaml::Value> for LenientYamlParser {
    fn parse(&self, source: &str) -> Result<serde_yaml::Value, ParseError> {
        // Parse with lenient options
        let options = ParseOptions::lenient();
        
        // Implementation:
        // - Unknown fields are ignored
        // - Type coercion is attempted
        // - Duplicate keys use last value
        // - Minor format issues are tolerated
        
        parse_yaml_lenient(source, self.allow_unknown_fields, self.coerce_types)
    }
}

// Usage:
let parser = LenientYamlParser {
    allow_unknown_fields: true,
    coerce_types: true,
};
let result = parser.parse("name: test\nvalue: 42")?;
```

**Characteristics:**
- Unknown fields are ignored (with warnings in `ParseResult`)
- Type coercion is attempted when safe
- Duplicate keys use last value (with warnings)
- Minor format issues are tolerated
- Succeeds where possible rather than failing

### 3. Custom Parsing

Custom parsers implement domain-specific parsing logic:

```rust
use armor::parsers::Parser;

pub struct CustomConfigParser {
    allow_extended_syntax: bool,
    resolve_variables: bool,
    variables: std::collections::HashMap<String, String>,
}

impl Parser<&str, CustomConfig> for CustomConfigParser {
    fn parse(&self, source: &str) -> Result<CustomConfig, ParseError> {
        // Custom parsing logic:
        // 1. Variable substitution
        // 2. Extended syntax support
        // 3. Domain-specific validation
        // 4. Custom error messages
        
        let mut content = source.to_string();
        
        // Variable substitution
        if self.resolve_variables {
            content = substitute_variables(&content, &self.variables)?;
        }
        
        // Parse with extended syntax
        let config = parse_extended_syntax(&content, self.allow_extended_syntax)?;
        
        // Domain-specific validation
        validate_config(&config)?;
        
        Ok(config)
    }
}

// Usage:
let mut parser = CustomConfigParser {
    allow_extended_syntax: true,
    resolve_variables: true,
    variables: std::collections::HashMap::new(),
};

parser.variables.insert("ENV".to_string(), "production".to_string());

let config = parser.parse("environment: ${ENV}\nport: 8080")?;
```

**Characteristics:**
- Domain-specific parsing logic
- Custom preprocessing (variable substitution, includes, etc.)
- Extended syntax support
- Business logic validation
- Custom error messages for domain errors

## Parser Composition

Multiple parsers can be chained together for complex processing:

```rust
use armor::parsers::Parser;

// Parser 1: Raw string -> Intermediate representation
pub struct FirstParser;
impl Parser<&str, Intermediate> for FirstParser {
    fn parse(&self, source: &str) -> Result<Intermediate, ParseError> {
        // Parse raw YAML into intermediate format
        Ok(Intermediate::from_yaml(source)?)
    }
}

// Parser 2: Intermediate -> Final output
pub struct SecondParser;
impl Parser<Intermediate, Output> for SecondParser {
    fn parse(&self, source: Intermediate) -> Result<Output, ParseError> {
        // Transform intermediate format to final output
        Ok(source.to_output())
    }
}

// Usage: Chain parsers
let first = FirstParser;
let second = SecondParser;

let output = second.parse(first.parse(input)?);
```

## Extended Traits

### StreamingParser

For processing large inputs as a stream:

```rust
pub trait StreamingParser<Input, Output>: Parser<Input, Output> {
    fn parse_stream<'a, I>(&self, sources: I) -> Result<Vec<Output>, ParseError>
    where
        Input: 'a,
        I: IntoIterator<Item = Input>;
        
    fn parse_parallel<'a, I>(&self, sources: I, parallelism: usize) -> Result<Vec<Output>, ParseError>
    where
        Input: 'a,
        I: IntoIterator<Item = Input>;
}
```

### IncrementalParser

For parsing input in chunks:

```rust
pub trait IncrementalParser<Output>: Parser<Vec<u8>, Output> {
    fn init_parse(&mut self) -> Result<(), ParseError>;
    fn feed_chunk(&mut self, chunk: Vec<u8>) -> Result<(), ParseError>;
    fn finalize_parse(&mut self) -> Result<Output, ParseError>;
}
```

## Error Handling

All parser methods return `Result<T, ParseError>` for consistent error handling:

```rust
use armor::parsers::Parser;

match parser.parse(source) {
    Ok(output) => println!("Parsed successfully: {:?}", output),
    Err(ParseError::Syntax(msg)) => eprintln!("Syntax error: {}", msg),
    Err(ParseError::Io(msg)) => eprintln!("I/O error: {}", msg),
    Err(ParseError::Validation(msg)) => eprintln!("Validation error: {}", msg),
    Err(ParseError::TypeMismatch { field, expected, actual }) => {
        eprintln!("Type mismatch at '{}': expected {}, got {}", field, expected, actual);
    }
    Err(e) => eprintln!("Other error: {}", e),
}
```

### Error Propagation

Use the `?` operator for clean error propagation:

```rust
use armor::parsers::{Parser, ParseError};

fn parse_and_validate(source: &str) -> Result<Config, ParseError> {
    let config = parser.parse(source)?;  // Auto-converts errors
    
    if config.port < 1 || config.port > 65535 {
        return Err(ParseError::validation("port must be between 1 and 65535"));
    }
    
    Ok(config)
}
```

## Examples

### Example 1: Simple String Parser

```rust
use armor::parsers::Parser;

struct ConfigParser;

struct Config {
    name: String,
    value: i32,
}

impl Parser<&str, Config> for ConfigParser {
    fn parse(&self, source: &str) -> Result<Config, ParseError> {
        // Parse logic here
        Ok(Config { 
            name: "test".to_string(), 
            value: 42 
        })
    }
}

let parser = ConfigParser;
let config = parser.parse("name: test\nvalue: 42")?;
```

### Example 2: Parser with Options

```rust
use armor::parsers::{Parser, ParseOptions};

let parser = MyParser::new();
let options = ParseOptions {
    strict_mode: true,
    preserve_comments: false,
    ..Default::default()
};
let result = parser.parse_with_options(source, options)?;
```

### Example 3: File Parser

```rust
use armor::parsers::Parser;
use std::path::Path;

let parser = MyParser::new();
let result = parser.parse_file(Path::new("config.yaml"))?;
```

### Example 4: Validation

```rust
use armor::parsers::Parser;

let parser = MyParser::new();

if parser.validate("key: value").is_ok() {
    println!("Input is valid!");
}
```

## Implementation Checklist

When implementing the `Parser` trait, ensure:

- [ ] `parse()` method correctly transforms input to output
- [ ] Errors use appropriate `ParseError` variants
- [ ] `parse_with_options()` respects provided options (or defaults to `parse()`)
- [ ] `parse_file()` correctly handles file I/O errors
- [ ] `validate()` performs lightweight validation
- [ ] `metadata()` returns accurate parser information
- [ ] All methods have proper error handling
- [ ] Documentation is clear and includes examples

## Relationship to ARMOR Types

### ParseResult<T>

The YAML-specific `ParseResult<T>` type provides richer output:

```rust
use armor::parsers::yaml::ParseResult;

// Fields:
pub value: Option<T>              // The parsed value
pub error: Option<ParseError>     // Detailed error
pub metadata: ParseMetadata       // Lines, bytes, timing
pub warnings: Vec<ParseWarning>   // Non-fatal issues
```

### ParseError

Unified error type for all parsing operations:

```rust
pub enum ParseError {
    Yaml(YamlParseError),      // YAML-specific errors
    Io(String),                 // I/O errors
    Validation(String),         // Validation failures
    TypeMismatch { ... },      // Type mismatches
    Syntax(String),            // Syntax errors
    Other(String),             // Other errors
}
```

### ParseOptions

Configuration for parser behavior:

```rust
pub struct ParseOptions {
    pub strict_mode: bool,           // Reject unknown fields
    pub preserve_comments: bool,     // Keep comments in output
    pub allow_duplicates: bool,      // Allow duplicate keys
    pub max_depth: usize,           // Maximum nesting depth
    pub delimiter: Option<char>,    // Custom delimiter
}
```

## Conclusion

The `Parser<Input, Output>` trait provides a flexible, composable interface for parsing operations. By implementing this trait, parsers can:

- Work with any input/output types
- Use standard Rust error handling
- Compose with other parsers
- Support streaming and incremental parsing
- Provide consistent behavior across implementations

The design prioritizes simplicity and flexibility while allowing implementations to provide richer functionality (like `ParseResult<T>`) when needed.
