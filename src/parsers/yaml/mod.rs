//! YAML parser module
//!
//! This module provides YAML parsing functionality for the ARMOR project.
//! It includes types for parsing results, error handling, and the main parser trait.

mod error;
mod types;
mod parser;

// Re-export main types for convenience
pub use error::{ParseError, ParseErrorKind};
pub use types::{ParseResult, ValidationResult, Status};
pub use parser::Parser;

/// Version of the YAML parser module
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

/// Default parser configuration
pub const DEFAULT_PARSER_CONFIG: ParserConfig = ParserConfig {
    strict_mode: false,
    allow_duplicates: true,
    preserve_quotes: false,
};

/// Parser configuration options
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ParserConfig {
    /// Enable strict parsing mode
    pub strict_mode: bool,
    /// Allow duplicate keys in mappings
    pub allow_duplicates: bool,
    /// Preserve quote information in parsed strings
    pub preserve_quotes: bool,
}

impl Default for ParserConfig {
    fn default() -> Self {
        DEFAULT_PARSER_CONFIG
    }
}
