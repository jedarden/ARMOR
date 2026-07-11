//! Error types for YAML parsing
//!
//! This module defines the error types used throughout the YAML parser.

use std::fmt;

/// Main error type for YAML parsing operations
#[derive(Debug, Clone, PartialEq)]
pub struct ParseError {
    /// The kind of error that occurred
    pub kind: ParseErrorKind,
    /// The line number where the error occurred (1-indexed)
    pub line: Option<usize>,
    /// The column number where the error occurred (1-indexed)
    pub column: Option<usize>,
    /// The file or source path where the error occurred
    pub path: Option<String>,
    /// A code snippet showing the problematic segment
    pub snippet: Option<String>,
    /// Additional context about the error
    pub context: String,
}

impl ParseError {
    /// Create a new ParseError with the given kind
    pub fn new(kind: ParseErrorKind) -> Self {
        Self {
            kind,
            line: None,
            column: None,
            path: None,
            snippet: None,
            context: String::new(),
        }
    }

    /// Set the line number for this error
    pub fn with_line(mut self, line: usize) -> Self {
        self.line = Some(line);
        self
    }

    /// Set the column number for this error
    pub fn with_column(mut self, column: usize) -> Self {
        self.column = Some(column);
        self
    }

    /// Set the file/source path for this error
    pub fn with_path(mut self, path: impl Into<String>) -> Self {
        self.path = Some(path.into());
        self
    }

    /// Set the code snippet for this error
    pub fn with_snippet(mut self, snippet: impl Into<String>) -> Self {
        self.snippet = Some(snippet.into());
        self
    }

    /// Set the context message for this error
    pub fn with_context(mut self, context: impl Into<String>) -> Self {
        self.context = context.into();
        self
    }

    /// Create a syntax error
    pub fn syntax(msg: impl Into<String>) -> Self {
        Self::new(ParseErrorKind::Syntax(msg.into()))
    }

    /// Create an I/O error
    pub fn io(msg: impl Into<String>) -> Self {
        Self::new(ParseErrorKind::Io(msg.into()))
    }

    /// Create a validation error
    pub fn validation(msg: impl Into<String>) -> Self {
        Self::new(ParseErrorKind::Validation(msg.into()))
    }

    /// Create a type mismatch error
    ///
    /// # Arguments
    /// * `field` - The field path where the error occurred
    /// * `expected` - The expected type description
    /// * `actual` - The actual type that was received
    pub fn type_mismatch(field: impl Into<String>, expected: impl Into<String>, actual: impl Into<String>) -> Self {
        Self::new(ParseErrorKind::TypeMismatch {
            field: field.into(),
            expected: expected.into(),
            actual: actual.into(),
        })
    }

    /// Check if this is a syntax error
    pub fn is_syntax(&self) -> bool {
        matches!(self.kind, ParseErrorKind::Syntax(_))
    }

    /// Check if this is an I/O error
    pub fn is_io(&self) -> bool {
        matches!(self.kind, ParseErrorKind::Io(_))
    }

    /// Check if this is a validation error
    pub fn is_validation(&self) -> bool {
        matches!(self.kind, ParseErrorKind::Validation(_))
    }

    /// Check if this is a type mismatch error
    pub fn is_type_mismatch(&self) -> bool {
        matches!(self.kind, ParseErrorKind::TypeMismatch { .. })
    }
}

impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match (&self.path, &self.line, &self.column) {
            (Some(path), Some(line), Some(col)) => {
                write!(f, "{}:{}:{}: {}", path, line, col, self.kind)
            }
            (Some(path), Some(line), None) => {
                write!(f, "{}:{}: {}", path, line, self.kind)
            }
            (Some(path), None, None) => {
                write!(f, "{}: {}", path, self.kind)
            }
            (Some(path), None, Some(col)) => {
                write!(f, "{}::{}: {}", path, col, self.kind)
            }
            (None, Some(line), Some(col)) => {
                write!(f, "{}:{}: {}", line, col, self.kind)
            }
            (None, Some(line), None) => {
                write!(f, "{}: {}", line, self.kind)
            }
            (None, None, Some(_)) => {
                write!(f, "{}: {}", self.kind, self.context)
            }
            (None, None, None) => {
                write!(f, "{}: {}", self.kind, self.context)
            }
        }?;

        // Add context if present
        if !self.context.is_empty() {
            write!(f, ": {}", self.context)?;
        }

        // Add snippet if present
        if let Some(snippet) = &self.snippet {
            write!(f, "\n\nSnippet:\n{}", snippet)?;
        }

        Ok(())
    }
}

impl std::error::Error for ParseError {}

/// The kind of parse error that occurred
///
/// This enum represents the core categories of errors that can occur during
/// YAML parsing operations. Each variant represents a distinct class of error.
#[derive(Debug, Clone, PartialEq)]
pub enum ParseErrorKind {
    /// Syntax error in the YAML source
    ///
    /// This variant covers errors related to invalid YAML structure, malformed
    /// syntax, or violations of the YAML grammar rules.
    Syntax(String),

    /// I/O error (file not found, permission denied, etc.)
    ///
    /// This variant covers errors related to file system operations such as
    /// reading, writing, or accessing files.
    Io(String),

    /// Validation error (constraint violations)
    ///
    /// This variant covers errors related to constraint violations such as
    /// required field violations, range violations, or other schema validation failures.
    Validation(String),

    /// Type mismatch error (unexpected type for a field)
    ///
    /// This variant covers errors where a value has an unexpected type, such as
    /// expecting a string but receiving a number, or expecting a sequence but receiving a scalar.
    TypeMismatch {
        /// The field path where the error occurred
        field: String,
        /// The expected type
        expected: String,
        /// The actual type that was received
        actual: String,
    },

    /// Unexpected end of input
    ///
    /// This variant covers errors where the YAML source ends prematurely
    /// and cannot be fully parsed.
    UnexpectedEof,

    /// Invalid UTF-8 encoding
    ///
    /// This variant covers errors where the input contains invalid UTF-8 sequences.
    InvalidUtf8,

    /// Unknown anchor or alias
    ///
    /// This variant covers errors where an anchor or alias reference cannot be resolved.
    UnknownAnchor(String),

    /// Duplicate key in mapping
    ///
    /// This variant covers errors where duplicate keys are found in a YAML mapping.
    DuplicateKey(String),

    /// Other error
    ///
    /// This variant provides a catch-all for errors that don't fit into any of the
    /// other specific categories, allowing for extensibility.
    Other(String),
}

impl fmt::Display for ParseErrorKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Syntax(msg) => write!(f, "syntax error: {}", msg),
            Self::Io(msg) => write!(f, "I/O error: {}", msg),
            Self::Validation(msg) => write!(f, "validation error: {}", msg),
            Self::TypeMismatch { field, expected, actual } => {
                write!(f, "type mismatch at '{}': expected {}, got {}", field, expected, actual)
            }
            Self::UnexpectedEof => write!(f, "unexpected end of input"),
            Self::InvalidUtf8 => write!(f, "invalid UTF-8 encoding"),
            Self::UnknownAnchor(name) => write!(f, "unknown anchor: {}", name),
            Self::DuplicateKey(key) => write!(f, "duplicate key: {}", key),
            Self::Other(msg) => write!(f, "error: {}", msg),
        }
    }
}

/// Result type alias for parse operations
pub type Result<T> = std::result::Result<T, ParseError>;
