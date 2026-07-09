//! Core parser trait and implementations for YAML parsing
//!
//! This module defines the Parser trait and provides functionality for
//! parsing YAML content from various sources.

use crate::parsers::yaml::{
    error::{ParseError, Result},
    types::{ParseResult, ValidationResult},
    ParserConfig,
};

/// Trait for YAML parsers
///
/// This trait defines the core interface for parsing YAML content.
/// Implementations can parse from strings, files, or other sources.
pub trait Parser {
    /// Parse YAML content from a string
    ///
    /// # Arguments
    /// * `content` - The YAML content as a string
    ///
    /// # Returns
    /// A ParseResult containing the parsed data or an error
    ///
    /// # Example
    /// ```no_run
    /// use armor::parsers::yaml::Parser;
    /// let parser = MyParser::new();
    /// let yaml = "name: test\nvalue: 42";
    /// let result = parser.parse_str(yaml);
    /// ```
    fn parse_str(&self, content: &str) -> ParseResult<serde_yaml::Value>;

    /// Parse YAML content from a byte slice
    ///
    /// # Arguments
    /// * `content` - The YAML content as bytes
    ///
    /// # Returns
    /// A ParseResult containing the parsed data or an error
    fn parse_bytes(&self, content: &[u8]) -> ParseResult<serde_yaml::Value>;

    /// Parse YAML content from a file
    ///
    /// # Arguments
    /// * `path` - Path to the YAML file
    ///
    /// # Returns
    /// A ParseResult containing the parsed data or an error
    fn parse_file(&self, path: &std::path::Path) -> ParseResult<serde_yaml::Value>;

    /// Validate YAML content without fully parsing it
    ///
    /// # Arguments
    /// * `content` - The YAML content as a string
    ///
    /// # Returns
    /// A ValidationResult indicating if the content is valid
    fn validate_str(&self, content: &str) -> ValidationResult;

    /// Validate a YAML file without fully parsing it
    ///
    /// # Arguments
    /// * `path` - Path to the YAML file
    ///
    /// # Returns
    /// A ValidationResult indicating if the file is valid
    fn validate_file(&self, path: &std::path::Path) -> ValidationResult;

    /// Get the parser configuration
    ///
    /// # Returns
    /// A reference to the parser's configuration
    fn config(&self) -> &ParserConfig;

    /// Set the parser configuration
    ///
    /// # Arguments
    /// * `config` - The new configuration
    ///
    /// # Returns
    /// The parser with the new configuration
    fn with_config(self, config: ParserConfig) -> Self
    where
        Self: Sized;
}

/// Basic YAML parser implementation
///
/// This is a minimal implementation of the Parser trait that
/// provides basic YAML parsing functionality.
#[derive(Debug, Clone)]
pub struct BasicParser {
    config: ParserConfig,
}

impl BasicParser {
    /// Create a new BasicParser with default configuration
    pub fn new() -> Self {
        Self {
            config: ParserConfig::default(),
        }
    }

    /// Create a new BasicParser with the specified configuration
    pub fn with_config(config: ParserConfig) -> Self {
        Self { config }
    }

    /// Create a new strict parser
    ///
    /// A strict parser enables strict mode and disallows duplicate keys
    pub fn strict() -> Self {
        Self {
            config: ParserConfig {
                strict_mode: true,
                allow_duplicates: false,
                preserve_quotes: false,
            },
        }
    }
}

impl Default for BasicParser {
    fn default() -> Self {
        Self::new()
    }
}

impl Parser for BasicParser {
    fn parse_str(&self, content: &str) -> ParseResult<serde_yaml::Value> {
        // Stub implementation
        ParseResult::success(serde_yaml::Value::Null)
    }

    fn parse_bytes(&self, content: &[u8]) -> ParseResult<serde_yaml::Value> {
        // Stub implementation
        ParseResult::success(serde_yaml::Value::Null)
    }

    fn parse_file(&self, path: &std::path::Path) -> ParseResult<serde_yaml::Value> {
        // Stub implementation
        ParseResult::success(serde_yaml::Value::Null)
    }

    fn validate_str(&self, content: &str) -> ValidationResult {
        // Stub implementation
        ValidationResult::success()
    }

    fn validate_file(&self, path: &std::path::Path) -> ValidationResult {
        // Stub implementation
        ValidationResult::success()
    }

    fn config(&self) -> &ParserConfig {
        &self.config
    }

    fn with_config(self, config: ParserConfig) -> Self
    where
        Self: Sized,
    {
        Self { config }
    }
}

/// Create a new YAML parser with default configuration
pub fn new_parser() -> BasicParser {
    BasicParser::new()
}

/// Create a new strict YAML parser
pub fn new_strict_parser() -> BasicParser {
    BasicParser::strict()
}

/// Convenience function to parse YAML from a string
///
/// # Arguments
/// * `content` - The YAML content as a string
///
/// # Returns
/// A ParseResult containing the parsed data or an error
///
/// # Example
/// ```no_run
/// use armor::parsers::yaml::parse_yaml;
/// let yaml = "name: test\nvalue: 42";
/// let result = parse_yaml(yaml);
/// ```
pub fn parse_yaml(content: &str) -> ParseResult<serde_yaml::Value> {
    new_parser().parse_str(content)
}

/// Convenience function to parse YAML from a file
///
/// # Arguments
/// * `path` - Path to the YAML file
///
/// # Returns
/// A ParseResult containing the parsed data or an error
pub fn parse_yaml_file(path: &std::path::Path) -> ParseResult<serde_yaml::Value> {
    new_parser().parse_file(path)
}
