//! Error types for YAML parsing
//!
//! This module defines the error types used throughout the YAML parser.

use std::fmt;

/// Main error type for YAML parsing operations
#[derive(Clone, PartialEq)]
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

    /// Set both line and column for this error
    pub fn with_location(mut self, line: usize, column: usize) -> Self {
        self.line = Some(line);
        self.column = Some(column);
        self
    }

    /// Get a formatted location string (e.g., "file.yaml:10:5" or "<unknown>:10")
    pub fn location_string(&self) -> String {
        match (&self.path, self.line, self.column) {
            (Some(path), Some(line), Some(col)) => format!("{}:{}:{}", path, line, col),
            (Some(path), Some(line), None) => format!("{}:{}", path, line),
            (Some(path), None, None) => path.clone(),
            (None, Some(line), Some(col)) => format!("{}:{}", line, col),
            (None, Some(line), None) => format!("{}", line),
            (None, None, Some(col)) => format!("col {}", col),
            (None, None, None) => "<unknown>".to_string(),
        }
    }

    /// Get a brief summary of the error (single line, suitable for logging)
    pub fn summary(&self) -> String {
        let location = self.location_string();
        if !self.context.is_empty() {
            format!("{}: {} - {}", location, self.kind, self.context)
        } else {
            format!("{}: {}", location, self.kind)
        }
    }

    /// Get a detailed multi-line error report with snippet and visual indicator
    pub fn detailed_report(&self) -> String {
        let mut report = String::new();

        // Header line with location and error kind
        report.push_str(&format!("error: {}\n", self.summary()));

        // Add context section if present
        if !self.context.is_empty() {
            report.push_str(&format!("  context: {}\n", self.context));
        }

        // Add snippet with visual indicator
        if let Some(snippet) = &self.snippet {
            report.push_str("\n  snippet:\n");
            for line in snippet.lines() {
                report.push_str(&format!("    {}\n", line));
            }

            // Add visual indicator for column position
            if let Some(col) = self.column {
                if col > 0 {
                    report.push_str(&format!("    {}^\n", " ".repeat(col.saturating_sub(1))));
                }
            }
        }

        report
    }

    /// Format this error as a structured log entry (JSON-like but readable)
    pub fn format_structured(&self) -> String {
        format!(
            "ParseError {{ kind: {:?}, location: {}, line: {:?}, column: {:?} }}",
            self.kind,
            self.location_string(),
            self.line,
            self.column
        )
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
        // Use the summary method for concise display
        write!(f, "{}", self.summary())?;

        // Add detailed section with snippet if available
        if let Some(snippet) = &self.snippet {
            write!(f, "\n\n  snippet:")?;
            for line in snippet.lines() {
                write!(f, "\n    {}", line)?;
            }

            // Add visual indicator for column position
            if let Some(col) = self.column {
                if col > 0 {
                    write!(f, "\n    {}", " ".repeat(col.saturating_sub(1)))?;
                    write!(f, "^")?;
                }
            }
        }

        Ok(())
    }
}

impl std::error::Error for ParseError {}

impl fmt::Debug for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("ParseError")
            .field("kind", &self.kind)
            .field("location", &self.location_string())
            .field("line", &self.line)
            .field("column", &self.column)
            .field("path", &self.path)
            .field("context", &self.context)
            .field("has_snippet", &self.snippet.is_some())
            .finish()
    }
}

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
