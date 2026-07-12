//! YAML parser module
//!
//! This module provides YAML parsing functionality for the ARMOR project.
//! It includes types for parsing results, error handling, and the main parser trait.
//!
//! ## Architecture
//!
//! The YAML parser module is organized into several sub-modules:
//!
//! - [`error`] - Comprehensive error types and error handling
//! - [`types`] - Result types for parsing operations
//! - [`parser`] - YAML-specific parser trait and implementations
//! - [`syntax_validator`] - Syntax validation functionality
//! - [`syntax_detector`] - Syntax error detection (indentation, delimiters, structure)
//! - [`line_parser`] - Core data structures for representing parsed YAML lines
//!
//! ## Quick Start
//!
//! ```no_run
//! use armor::parsers::yaml::{parse_yaml, ParseResult};
//!
//! fn main() {
//!     let yaml = r#"
//!         name: example
//!         value: 42
//!     "#;
//!
//!     let result = parse_yaml(yaml);
//!     if result.is_success() {
//!         println!("Parsed successfully");
//!     }
//! }
//! ```
//!
//! ## Line Parser
//!
//! The [`line_parser`] module provides the foundational types for line-based YAML analysis:
//!
//! ```ignore
//! use armor::parsers::yaml::line_parser::{YamlLine, LineType};
//!
//! let line = YamlLine::new(1, "key: value", 0, LineType::MappingKey);
//! assert_eq!(line.line_number(), 1);
//! assert_eq!(line.raw_content(), "key: value");
//! assert_eq!(line.line_type(), LineType::MappingKey);
//! ```

mod error;
mod types;
mod parser;
mod syntax_validator;
mod syntax_detector;
mod line_parser;

#[cfg(test)]
mod syntax_detector_tests;

// Re-export main types for convenience
pub use error::{ParseError, ParseErrorKind, Result};
pub use types::{
    OperationResult, ParseMetadata, ParseResult, ParseWarning, ParseWarningKind,
    ValidationResult, ValidationError, ValidationWarning, Status,
};
pub use parser::Parser as YamlParser;
pub use syntax_validator::SyntaxValidator;
pub use syntax_detector::{SyntaxDetector, DelimiterErrorType, IndentationErrorType};
pub use line_parser::{
    LineType, YamlLine, LineContent, LineParseResult,
    MappingKeyInfo, detect_mapping_key, is_comment_line, strip_inline_comment,
};

// Re-export comprehensive configuration from config module
pub use crate::parsers::config::{
    ParserConfig, ParserMode, ParserConfigBuilder, TypeConstructor, ValidationHook,
    ValidatorConfig, ValidationMode, ValidatorConfigBuilder,
    default_parser_config, default_validator_config,
};

/// Version of the YAML parser module
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

/// Create a new YAML parser with default configuration
///
/// This is a convenience function that creates a `BasicParser` with
/// default (lenient) configuration.
pub fn new_parser() -> parser::BasicParser {
    parser::BasicParser::new()
}

/// Create a new strict YAML parser
///
/// This is a convenience function that creates a `BasicParser` with
/// strict configuration (rejects unknown fields, disallows duplicates).
pub fn new_strict_parser() -> parser::BasicParser {
    parser::BasicParser::strict()
}

/// Parse YAML content from a string using default configuration
///
/// This is a convenience function for quick parsing operations.
pub fn parse_yaml(content: &str) -> ParseResult<serde_yaml::Value> {
    new_parser().parse_str(content)
}

/// Parse YAML content from a file using default configuration
///
/// This is a convenience function for quick file parsing operations.
pub fn parse_yaml_file(path: &std::path::Path) -> ParseResult<serde_yaml::Value> {
    new_parser().parse_file(path)
}
