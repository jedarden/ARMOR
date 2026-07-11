//! Generic parser traits and interfaces
//!
//! This module defines the core parser traits that different parsing
//! strategies (YAML, JSON, etc.) can implement.

use std::path::Path;
use crate::parsers::yaml::ParseError as YamlParseError;

/// Generic parser trait for different parsing strategies
///
/// This trait defines the core interface for parsers that transform
/// input of one type into structured output. It is designed to be:
///
/// 1. **Generic** - Works with any input/output type combination
/// 2. **Composable** - Multiple parsers can be chained together
/// 3. **Extensible** - Supports streaming and incremental parsing
/// 4. **Error-aware** - Uses structured error types for consistent error handling
///
/// # Relationship to ParseResult and ParseError
///
/// The generic `Parser<Input, Output>` trait uses Rust's standard `Result<Output, ParseError>`
/// for its core parsing method. However, for YAML-specific parsing, the [`ParseResult<T>`](crate::parsers::yaml::ParseResult)
/// type provides a richer result structure that includes:
///
/// - **Metadata**: Lines processed, bytes processed, timing information
/// - **Warnings**: Non-fatal issues that don't prevent parsing (e.g., deprecated fields)
/// - **Detailed error context**: Location information, code snippets, suggestions
///
/// ## When to Use Result vs ParseResult
///
/// - **Use `Result<T, ParseError>`** (this trait): For generic parsers, simple parsing operations,
///   and when you want standard Rust error handling patterns
/// - **Use `ParseResult<T>`** (YAML-specific): For YAML parsing when you need rich metadata,
///   warning collection, and detailed error reporting
///
/// ## Converting Between Result and ParseResult
///
/// [`ParseResult<T>`](crate::parsers::yaml::ParseResult) implements `From<Result<T>>` for seamless
/// integration:
///
/// ```ignore
/// use armor::parsers::yaml::ParseResult;
///
/// // Convert Result<T, ParseError> to ParseResult<T>
/// let result: Result<MyType, ParseError> = parse_value();
/// let parse_result: ParseResult<MyType> = ParseResult::from(result);
/// ```
///
/// # Type Parameters
///
/// * `Input` - The input source type (e.g., `&str`, `&[u8]`, `&Path`)
/// * `Output` - The parsed result type (e.g., configuration struct, AST)
///
/// # Trait Bounds
///
/// For common use cases, the following type combinations are typical:
///
/// - `Input = &str` for string-based formats (YAML, JSON, TOML)
/// - `Input = &[u8]` for binary formats
/// - `Input = &Path` for file-based parsing (delegates to string parsing)
///
/// The `Output` type is the target representation after parsing.
///
/// # Parsing Strategies
///
/// This trait supports multiple parsing strategies that can be configured through
/// [`ParseOptions`] or implementation-specific behavior:
///
/// ## Strict Parsing
///
/// Strict parsing follows format specifications precisely and rejects any deviations:
///
/// ```ignore
/// use armor::parsers::{Parser, ParseOptions};
///
/// let parser = StrictParser::new();
/// let options = ParseOptions::strict();
/// let result = parser.parse_with_options(source, options)?;
///
/// // In strict mode:
/// // - Unknown fields cause errors
/// // - Type mismatches are rejected
/// // - Duplicate keys are not allowed
/// // - Format specifications are enforced strictly
/// ```
///
/// ## Lenient Parsing
///
/// Lenient parsing is more forgiving and attempts to recover from errors:
///
/// ```ignore
/// use armor::parsers::{Parser, ParseOptions};
///
/// let parser = LenientParser::new();
/// let options = ParseOptions::lenient();
/// let result = parser.parse_with_options(source, options)?;
///
/// // In lenient mode:
/// // - Unknown fields are ignored
/// // - Type coercion is attempted
/// // - Duplicate keys use last value
/// // - Minor format issues are tolerated
/// ```
///
/// ## Custom Parsing
///
/// Custom parsers implement domain-specific parsing logic:
///
/// ```ignore
/// use armor::parsers::Parser;
///
/// struct CustomConfigParser {
///     allow_extended_syntax: bool,
///     resolve_variables: bool,
/// }
///
/// impl Parser<&str, CustomConfig> for CustomConfigParser {
///     fn parse(&self, source: &str) -> Result<CustomConfig, ParseError> {
///         // Custom parsing logic here
///         // - Variable substitution
///         // - Extended syntax support
///         // - Domain-specific validation
///         Ok(CustomConfig { /* ... */ })
///     }
/// }
/// ```
///
/// # Core Method
///
/// The [`Parser::parse()`] method is the core parsing operation:
///
/// ```ignore
/// fn parse(&self, source: Input) -> Result<Output, ParseError>
/// ```
///
/// # Examples
///
/// ## Basic String Parsing
///
/// ```ignore
/// use armor::parsers::Parser;
/// use armor::parsers::yaml::ParseError;
///
/// struct ConfigParser;
///
/// struct Config {
///     name: String,
///     value: i32,
/// }
///
/// impl Parser<&str, Config> for ConfigParser {
///     fn parse(&self, source: &str) -> Result<Config, ParseError> {
///         // Parse logic here
///         Ok(Config { name: "test".to_string(), value: 42 })
///     }
/// }
///
/// let parser = ConfigParser;
/// let config = parser.parse("name: test\nvalue: 42")?;
/// ```
///
/// ## Chaining Parsers
///
/// ```ignore
/// use armor::parsers::Parser;
///
/// // Parser 1: Raw string -> Intermediate representation
/// struct FirstParser;
/// impl Parser<&str, Intermediate> for FirstParser { /* ... */ }
///
/// // Parser 2: Intermediate -> Final output
/// struct SecondParser;
/// impl Parser<Intermediate, Output> for SecondParser { /* ... */ }
///
/// let first = FirstParser;
/// let second = SecondParser;
/// let output = second.parse(first.parse(input)?);
/// ```
///
/// ## Error Handling
///
/// ```ignore
/// use armor::parsers::Parser;
///
/// match parser.parse(source) {
///     Ok(output) => println!("Parsed successfully: {:?}", output),
///     Err(ParseError::Syntax(msg)) => eprintln!("Syntax error: {}", msg),
///     Err(ParseError::Io(msg)) => eprintln!("I/O error: {}", msg),
///     Err(e) => eprintln!("Other error: {}", e),
/// }
/// ```
pub trait Parser<Input, Output> {
    /// Parse input and return the parsed result
    ///
    /// This method accepts input of type `Input` and attempts to parse it
    /// into a structured `Output` type. Returns `Ok(Output)` on success,
    /// or `Err(ParseError)` if parsing fails.
    ///
    /// # Arguments
    ///
    /// * `source` - The input source to parse
    ///
    /// # Returns
    ///
    /// * `Ok(Output)` - Successfully parsed result
    /// * `Err(ParseError)` - Parsing error with context
    ///
    /// # Errors
    ///
    /// This method will return an error if:
    /// - The input syntax is invalid (syntax error)
    /// - The input cannot be read (I/O error)
    /// - The input violates constraints (validation error)
    /// - Types don't match expectations (type mismatch)
    ///
    /// # Examples
    ///
    /// ```ignore
    /// use armor::parsers::Parser;
    ///
    /// let parser = MyParser::new();
    /// let result = parser.parse("key: value")?;
    /// ```
    fn parse(&self, source: Input) -> Result<Output, ParseError>;

    /// Parse input with extended options
    ///
    /// This method provides a way to customize parsing behavior through
    /// options. The default implementation delegates to [`Parser::parse()`].
    ///
    /// # Arguments
    ///
    /// * `source` - The input source to parse
    /// * `options` - Parsing options to customize behavior
    ///
    /// # Returns
    ///
    /// * `Ok(Output)` - Successfully parsed result
    /// * `Err(ParseError)` - Parsing error with context
    ///
    /// # Examples
    ///
    /// ```ignore
    /// use armor::parsers::{Parser, ParseOptions};
    ///
    /// let parser = MyParser::new();
    /// let options = ParseOptions {
    ///     strict_mode: true,
    ///     preserve_comments: false,
    ///     ..Default::default()
    /// };
    /// let result = parser.parse_with_options(source, options)?;
    /// ```
    fn parse_with_options(&self, source: Input, options: ParseOptions) -> Result<Output, ParseError> {
        let _ = options; // Ignore options in default implementation
        self.parse(source)
    }

    /// Parse a file at the given path
    ///
    /// Convenience method that reads a file and parses its contents.
    /// The default implementation reads the file and delegates to [`Parser::parse()`].
    ///
    /// # Arguments
    ///
    /// * `path` - Path to the file to parse
    ///
    /// # Returns
    ///
    /// * `Ok(Output)` - Successfully parsed result
    /// * `Err(ParseError)` - I/O or parsing error
    ///
    /// # Errors
    ///
    /// This method will return an error if:
    /// - The file cannot be read (I/O error)
    /// - The file contents are invalid (parsing error)
    ///
    /// # Examples
    ///
    /// ```ignore
    /// use armor::parsers::Parser;
    /// use std::path::Path;
    ///
    /// let parser = MyParser::new();
    /// let result = parser.parse_file(Path::new("config.yaml"))?;
    /// ```
    fn parse_file(&self, path: &Path) -> Result<Output, ParseError>
    where
        Self: Sized,
        Input: From<String>,
    {
        let content = std::fs::read_to_string(path)
            .map_err(|e| ParseError::Io(e.to_string()))?;
        self.parse(content.into())
    }

    /// Validate input without fully parsing
    ///
    /// This method performs lightweight validation to check if the input
    /// is well-formed without constructing the full output structure.
    /// The default implementation attempts parsing and discards the result.
    ///
    /// # Arguments
    ///
    /// * `source` - The input source to validate
    ///
    /// # Returns
    ///
    /// * `Ok(())` - Input is valid
    /// * `Err(ParseError)` - Input is invalid
    ///
    /// # Examples
    ///
    /// ```ignore
    /// use armor::parsers::Parser;
    ///
    /// let parser = MyParser::new();
    /// if parser.validate("key: value").is_ok() {
    ///     println!("Input is valid!");
    /// }
    /// ```
    fn validate(&self, source: Input) -> Result<(), ParseError> {
        self.parse(source)?;
        Ok(())
    }

    /// Get metadata about the parser
    ///
    /// Returns information about the parser's capabilities and configuration.
    ///
    /// # Returns
    ///
    /// Parser metadata including supported formats and features
    ///
    /// # Examples
    ///
    /// ```ignore
    /// use armor::parsers::Parser;
    ///
    /// let parser = MyParser::new();
    /// let metadata = parser.metadata();
    /// println!("Parser: {}", metadata.name());
    /// println!("Supports streaming: {}", metadata.supports_streaming());
    /// ```
    fn metadata(&self) -> ParseMetadata {
        ParseMetadata::default()
    }
}

/// Trait for parsers that support streaming input
///
/// This trait extends [`Parser`] with capabilities for processing large
/// inputs as a stream of chunks or multiple sources.
///
/// # Examples
///
/// ```ignore
/// use armor::parsers::{Parser, StreamingParser};
///
/// let parser = MyStreamingParser::new();
///
/// // Parse multiple sources
/// let sources = vec!["file1.yaml", "file2.yaml", "file3.yaml"];
/// let results = parser.parse_stream(sources)?;
///
/// for result in results {
///     println!("Parsed: {:?}", result);
/// }
/// ```
pub trait StreamingParser<Input, Output>: Parser<Input, Output> {
    /// Parse multiple sources in sequence
    ///
    /// This method processes a collection of input sources and returns
    /// a vector of parsed results. Errors from individual sources are
    /// collected and returned together.
    ///
    /// # Arguments
    ///
    /// * `sources` - Iterator of input sources to parse
    ///
    /// # Returns
    ///
    /// * `Ok(Vec<Output>)` - All sources parsed successfully
    /// * `Err(ParseError)` - One or more sources failed to parse
    ///
    /// # Examples
    ///
    /// ```ignore
    /// use armor::parsers::StreamingParser;
    ///
    /// let parser = MyParser::new();
    /// let sources = vec!["config1.yaml", "config2.yaml"];
    /// let results = parser.parse_stream(sources)?;
    /// ```
    fn parse_stream<'a, I>(
        &self,
        sources: I,
    ) -> Result<Vec<Output>, ParseError>
    where
        Input: 'a,
        I: IntoIterator<Item = Input>;

    /// Parse sources in parallel
    ///
    /// This method processes multiple sources concurrently using parallel
    /// execution. Results are collected in the order of the input iterator.
    ///
    /// # Arguments
    ///
    /// * `sources` - Iterator of input sources to parse
    /// * `parallelism` - Maximum number of concurrent parse operations
    ///
    /// # Returns
    ///
    /// * `Ok(Vec<Output>)` - All sources parsed successfully
    /// * `Err(ParseError)` - One or more sources failed to parse
    ///
    /// # Examples
    ///
    /// ```ignore
    /// use armor::parsers::StreamingParser;
    ///
    /// let parser = MyParser::new();
    /// let sources = vec!["file1.yaml", "file2.yaml", "file3.yaml"];
    /// let results = parser.parse_parallel(sources, 4)?;
    /// ```
    fn parse_parallel<'a, I>(
        &self,
        sources: I,
        parallelism: usize,
    ) -> Result<Vec<Output>, ParseError>
    where
        Input: 'a,
        I: IntoIterator<Item = Input>,
    {
        // Default implementation processes sequentially
        self.parse_stream(sources)
    }
}

/// Trait for parsers that support incremental parsing
///
/// This trait extends [`Parser`] with the ability to parse input in chunks,
/// allowing processing of large inputs without loading the entire source
/// into memory.
///
/// # Examples
///
/// ```ignore
/// use armor::parsers::{Parser, IncrementalParser};
///
/// let parser = MyIncrementalParser::new();
///
/// // Initialize parsing
/// parser.init_parse()?;
///
/// // Feed chunks incrementally
/// parser.feed_chunk(b"key: ")?;
/// parser.feed_chunk(b"value")?;
///
/// // Complete parsing and get result
/// let result = parser.finalize_parse()?;
/// ```
pub trait IncrementalParser<Output>: Parser<Vec<u8>, Output> {
    /// Initialize incremental parsing
    ///
    /// Called before feeding chunks. Sets up internal state for parsing.
    ///
    /// # Returns
    ///
    /// * `Ok(())` - Parser initialized successfully
    /// * `Err(ParseError)` - Initialization failed
    fn init_parse(&mut self) -> Result<(), ParseError>;

    /// Feed a chunk of input to the parser
    ///
    /// Processes a chunk of input without completing the parse. Can be
    /// called multiple times before finalization.
    ///
    /// # Arguments
    ///
    /// * `chunk` - Chunk of input data
    ///
    /// # Returns
    ///
    /// * `Ok(())` - Chunk processed successfully
    /// * `Err(ParseError)` - Chunk processing failed
    fn feed_chunk(&mut self, chunk: Vec<u8>) -> Result<(), ParseError>;

    /// Finalize parsing and return the result
    ///
    /// Called after all chunks have been fed. Completes the parse and
    /// returns the final result.
    ///
    /// # Returns
    ///
    /// * `Ok(Output)` - Parsing completed successfully
    /// * `Err(ParseError)` - Finalization or parsing failed
    fn finalize_parse(&mut self) -> Result<Output, ParseError>;
}

/// Options for customizing parser behavior
///
/// This structure provides configuration options that can be passed to
/// [`Parser::parse_with_options()`].
#[derive(Debug, Clone, Default)]
pub struct ParseOptions {
    /// Enable strict mode (disallow unknown fields, enforce types strictly)
    pub strict_mode: bool,
    /// Preserve comments in the output (if format supports it)
    pub preserve_comments: bool,
    /// Allow duplicate keys (if format normally disallows them)
    pub allow_duplicates: bool,
    /// Maximum depth for nested structures (0 = no limit)
    pub max_depth: usize,
    /// Custom delimiter for key-value pairs (None = use format default)
    pub delimiter: Option<char>,
}

impl ParseOptions {
    /// Create new parse options with default values
    pub fn new() -> Self {
        Self::default()
    }

    /// Create options for strict parsing
    ///
    /// Sets `strict_mode` to true and `allow_duplicates` to false.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::ParseOptions;
    ///
    /// let options = ParseOptions::strict();
    /// assert!(options.strict_mode);
    /// assert!(!options.allow_duplicates);
    /// ```
    pub fn strict() -> Self {
        Self {
            strict_mode: true,
            allow_duplicates: false,
            ..Default::default()
        }
    }

    /// Create options for lenient parsing
    ///
    /// Sets `strict_mode` to false and `allow_duplicates` to true.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::ParseOptions;
    ///
    /// let options = ParseOptions::lenient();
    /// assert!(!options.strict_mode);
    /// assert!(options.allow_duplicates);
    /// ```
    pub fn lenient() -> Self {
        Self {
            strict_mode: false,
            allow_duplicates: true,
            ..Default::default()
        }
    }

    /// Builder method: set strict mode
    pub fn with_strict_mode(mut self, strict: bool) -> Self {
        self.strict_mode = strict;
        self
    }

    /// Builder method: set comment preservation
    pub fn with_preserve_comments(mut self, preserve: bool) -> Self {
        self.preserve_comments = preserve;
        self
    }

    /// Builder method: set duplicate key allowance
    pub fn with_allow_duplicates(mut self, allow: bool) -> Self {
        self.allow_duplicates = allow;
        self
    }

    /// Builder method: set maximum nesting depth
    pub fn with_max_depth(mut self, depth: usize) -> Self {
        self.max_depth = depth;
        self
    }

    /// Builder method: set custom delimiter
    pub fn with_delimiter(mut self, delimiter: char) -> Self {
        self.delimiter = Some(delimiter);
        self
    }
}

/// Metadata about a parser's capabilities
///
/// This structure describes what a parser can do, what formats it supports,
/// and its current configuration.
#[derive(Debug, Clone)]
pub struct ParseMetadata {
    /// Human-readable name of the parser (e.g., "YAML Parser", "JSON Parser")
    pub name: String,
    /// Format name this parser handles (e.g., "YAML 1.2", "JSON")
    pub format: String,
    /// Whether the parser supports streaming operations
    pub supports_streaming: bool,
    /// Whether the parser supports incremental parsing
    pub supports_incremental: bool,
    /// Supported file extensions (e.g., ["yaml", "yml"])
    pub extensions: Vec<String>,
    /// Parser version
    pub version: String,
}

impl ParseMetadata {
    /// Create new parser metadata
    pub fn new(name: impl Into<String>, format: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            format: format.into(),
            supports_streaming: false,
            supports_incremental: false,
            extensions: Vec::new(),
            version: "1.0.0".to_string(),
        }
    }

    /// Builder method: indicate streaming support
    pub fn with_streaming(mut self, supports: bool) -> Self {
        self.supports_streaming = supports;
        self
    }

    /// Builder method: indicate incremental parsing support
    pub fn with_incremental(mut self, supports: bool) -> Self {
        self.supports_incremental = supports;
        self
    }

    /// Builder method: add supported file extension
    pub fn with_extension(mut self, ext: impl Into<String>) -> Self {
        self.extensions.push(ext.into());
        self
    }

    /// Builder method: set parser version
    pub fn with_version(mut self, version: impl Into<String>) -> Self {
        self.version = version.into();
        self
    }

    /// Get the parser name
    pub fn name(&self) -> &str {
        &self.name
    }

    /// Get the format name
    pub fn format(&self) -> &str {
        &self.format
    }

    /// Check if streaming is supported
    pub fn supports_streaming(&self) -> bool {
        self.supports_streaming
    }

    /// Check if incremental parsing is supported
    pub fn supports_incremental(&self) -> bool {
        self.supports_incremental
    }

    /// Get supported file extensions
    pub fn extensions(&self) -> &[String] {
        &self.extensions
    }

    /// Get parser version
    pub fn version(&self) -> &str {
        &self.version
    }
}

impl Default for ParseMetadata {
    fn default() -> Self {
        Self {
            name: "Generic Parser".to_string(),
            format: "Unknown".to_string(),
            supports_streaming: false,
            supports_incremental: false,
            extensions: Vec::new(),
            version: "1.0.0".to_string(),
        }
    }
}

/// Unified parse error type
///
/// This type wraps format-specific errors (e.g., `yaml::ParseError`)
/// into a common type that the generic `Parser` trait can use.
#[derive(Debug, Clone)]
pub enum ParseError {
    /// YAML parsing error
    Yaml(YamlParseError),
    /// I/O error
    Io(String),
    /// Validation error
    Validation(String),
    /// Type mismatch error
    TypeMismatch {
        field: String,
        expected: String,
        actual: String,
    },
    /// Syntax error
    Syntax(String),
    /// Other error
    Other(String),
}

impl ParseError {
    /// Create a syntax error
    pub fn syntax(msg: impl Into<String>) -> Self {
        Self::Syntax(msg.into())
    }

    /// Create an I/O error
    pub fn io(msg: impl Into<String>) -> Self {
        Self::Io(msg.into())
    }

    /// Create a validation error
    pub fn validation(msg: impl Into<String>) -> Self {
        Self::Validation(msg.into())
    }

    /// Create a type mismatch error
    pub fn type_mismatch(field: impl Into<String>, expected: impl Into<String>, actual: impl Into<String>) -> Self {
        Self::TypeMismatch {
            field: field.into(),
            expected: expected.into(),
            actual: actual.into(),
        }
    }

    /// Create an other error
    pub fn other(msg: impl Into<String>) -> Self {
        Self::Other(msg.into())
    }

    /// Check if this is a syntax error
    pub fn is_syntax(&self) -> bool {
        matches!(self, Self::Syntax(_))
            || matches!(self, Self::Yaml(e) if e.is_syntax())
    }

    /// Check if this is an I/O error
    pub fn is_io(&self) -> bool {
        matches!(self, Self::Io(_))
            || matches!(self, Self::Yaml(e) if e.is_io())
    }

    /// Check if this is a validation error
    pub fn is_validation(&self) -> bool {
        matches!(self, Self::Validation(_))
            || matches!(self, Self::Yaml(e) if e.is_validation())
    }

    /// Check if this is a type mismatch error
    pub fn is_type_mismatch(&self) -> bool {
        matches!(self, Self::TypeMismatch { .. })
            || matches!(self, Self::Yaml(e) if e.is_type_mismatch())
    }

    /// Get a brief summary of the error
    pub fn summary(&self) -> String {
        match self {
            Self::Yaml(e) => e.summary(),
            Self::Io(msg) => format!("I/O error: {}", msg),
            Self::Validation(msg) => format!("validation error: {}", msg),
            Self::TypeMismatch { field, expected, actual } => {
                format!("type mismatch at '{}': expected {}, got {}", field, expected, actual)
            }
            Self::Syntax(msg) => format!("syntax error: {}", msg),
            Self::Other(msg) => format!("error: {}", msg),
        }
    }
}

impl std::fmt::Display for ParseError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.summary())
    }
}

impl std::error::Error for ParseError {}

impl From<YamlParseError> for ParseError {
    fn from(err: YamlParseError) -> Self {
        Self::Yaml(err)
    }
}

impl From<std::io::Error> for ParseError {
    fn from(err: std::io::Error) -> Self {
        Self::Io(err.to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_options_default() {
        let options = ParseOptions::default();
        assert!(!options.strict_mode);
        assert!(!options.preserve_comments);
        assert!(!options.allow_duplicates);
        assert_eq!(options.max_depth, 0);
        assert!(options.delimiter.is_none());
    }

    #[test]
    fn test_parse_options_strict() {
        let options = ParseOptions::strict();
        assert!(options.strict_mode);
        assert!(!options.allow_duplicates);
    }

    #[test]
    fn test_parse_options_lenient() {
        let options = ParseOptions::lenient();
        assert!(!options.strict_mode);
        assert!(options.allow_duplicates);
    }

    #[test]
    fn test_parse_options_builder() {
        let options = ParseOptions::new()
            .with_strict_mode(true)
            .with_preserve_comments(true)
            .with_allow_duplicates(false)
            .with_max_depth(10)
            .with_delimiter('=');

        assert!(options.strict_mode);
        assert!(options.preserve_comments);
        assert!(!options.allow_duplicates);
        assert_eq!(options.max_depth, 10);
        assert_eq!(options.delimiter, Some('='));
    }

    #[test]
    fn test_parse_metadata_default() {
        let metadata = ParseMetadata::default();
        assert_eq!(metadata.name, "Generic Parser");
        assert_eq!(metadata.format, "Unknown");
        assert!(!metadata.supports_streaming);
        assert!(!metadata.supports_incremental);
        assert!(metadata.extensions.is_empty());
        assert_eq!(metadata.version, "1.0.0");
    }

    #[test]
    fn test_parse_metadata_builder() {
        let metadata = ParseMetadata::new("YAML Parser", "YAML 1.2")
            .with_streaming(true)
            .with_incremental(false)
            .with_extension("yaml")
            .with_extension("yml")
            .with_version("2.0.0");

        assert_eq!(metadata.name(), "YAML Parser");
        assert_eq!(metadata.format(), "YAML 1.2");
        assert!(metadata.supports_streaming());
        assert!(!metadata.supports_incremental());
        assert_eq!(metadata.extensions().len(), 2);
        assert_eq!(metadata.version(), "2.0.0");
    }

    #[test]
    fn test_parse_error_creation() {
        let syntax_err = ParseError::syntax("test");
        assert!(syntax_err.is_syntax());

        let io_err = ParseError::io("test");
        assert!(io_err.is_io());

        let validation_err = ParseError::validation("test");
        assert!(validation_err.is_validation());

        let type_err = ParseError::type_mismatch("field", "int", "string");
        assert!(type_err.is_type_mismatch());
    }

    #[test]
    fn test_parse_error_summary() {
        let err = ParseError::syntax("invalid token");
        assert!(err.summary().contains("syntax error"));
        assert!(err.summary().contains("invalid token"));

        let err = ParseError::type_mismatch("port", "integer", "string");
        assert!(err.summary().contains("type mismatch"));
        assert!(err.summary().contains("port"));
    }

    #[test]
    fn test_parse_error_display() {
        let err = ParseError::validation("value out of range");
        let display = format!("{}", err);
        assert!(display.contains("validation error"));
        assert!(display.contains("value out of range"));
    }
}
