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

mod error;
mod types;
mod parser;

// Re-export main types for convenience
pub use error::{ParseError, ParseErrorKind, Result};
pub use types::{
    OperationResult, ParseMetadata, ParseResult, ParseWarning, ParseWarningKind,
    ValidationResult, ValidationError, ValidationWarning, Status,
};
pub use parser::Parser as YamlParser;

// Re-export comprehensive configuration from config module
pub use crate::parsers::config::{ParserConfig, ParserMode, ParserConfigBuilder, TypeConstructor, ValidationHook};

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
