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
}

impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match (&self.line, &self.column) {
            (Some(line), Some(col)) => {
                write!(f, "{}:{}:{}: {}", self.kind, line, col, self.context)
            }
            (Some(line), None) => {
                write!(f, "{}:{}: {}", self.kind, line, self.context)
            }
            (None, None) => {
                write!(f, "{}: {}", self.kind, self.context)
            }
            (None, Some(_)) => {
                write!(f, "{}: {}", self.kind, self.context)
            }
        }
    }
}

impl std::error::Error for ParseError {}

/// The kind of parse error that occurred
#[derive(Debug, Clone, PartialEq)]
pub enum ParseErrorKind {
    /// Syntax error in the YAML source
    Syntax(String),
    /// I/O error (file not found, permission denied, etc.)
    Io(String),
    /// Validation error (schema violation, type mismatch, etc.)
    Validation(String),
    /// Unexpected end of input
    UnexpectedEof,
    /// Invalid UTF-8 encoding
    InvalidUtf8,
    /// Unknown anchor or alias
    UnknownAnchor(String),
    /// Duplicate key in mapping
    DuplicateKey(String),
    /// Other error
    Other(String),
}

impl fmt::Display for ParseErrorKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Syntax(msg) => write!(f, "syntax error: {}", msg),
            Self::Io(msg) => write!(f, "I/O error: {}", msg),
            Self::Validation(msg) => write!(f, "validation error: {}", msg),
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
