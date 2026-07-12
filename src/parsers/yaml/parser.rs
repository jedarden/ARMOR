//! Core parser trait and implementations for YAML parsing
//!
//! This module defines the Parser trait and provides functionality for
//! parsing YAML content from various sources.

use crate::parsers::yaml::{
    types::{ParseResult, ValidationResult, ValidationError},
    ParserConfig,
    syntax_validator::SyntaxValidator,
    syntax_detector::SyntaxDetector,
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
    fn parse_str(&self, content: &str) -> ParseResult<serde_yaml::Value>;

    /// Parse YAML content from a byte slice
    fn parse_bytes(&self, content: &[u8]) -> ParseResult<serde_yaml::Value>;

    /// Parse YAML content from a file
    fn parse_file(&self, path: &std::path::Path) -> ParseResult<serde_yaml::Value>;

    /// Validate YAML content without fully parsing it
    fn validate_str(&self, content: &str) -> ValidationResult;

    /// Validate a YAML file without fully parsing it
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
    /// A strict parser enables strict mode and disallows duplicate keys.
    /// Uses the comprehensive strict configuration from ParserConfig.
    pub fn strict() -> Self {
        Self {
            config: ParserConfig::strict(),
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
        // Create syntax validator based on parser mode
        let validator = if self.config.is_strict() {
            SyntaxValidator::strict()
        } else {
            SyntaxValidator::lenient()
        };

        // Run syntax validation
        let mut result = validator.validate(content);

        // If no errors from basic validation, run enhanced detection
        if result.is_valid() {
            let mut detector = SyntaxDetector::new();
            let detector_result = detector.detect_to_validation_result(content);

            // Merge errors from detector
            if !detector_result.is_valid() {
                result.valid = false;
                result.errors.extend(detector_result.errors);
            }
        }

        result
    }

    fn validate_file(&self, path: &std::path::Path) -> ValidationResult {
        // Read file content
        let content = match std::fs::read_to_string(path) {
            Ok(content) => content,
            Err(err) => {
                return ValidationResult {
                    valid: false,
                    errors: vec![ValidationError::new(
                        path.display().to_string(),
                        format!("failed to read file: {}", err)
                    )],
                    warnings: Vec::new(),
                };
            }
        };

        // Validate the content
        self.validate_str(&content)
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

