//! Result types for YAML parsing operations
//!
//! This module defines the result types used by the YAML parser.

use crate::parsers::yaml::error::{ParseError, Result};
use std::fmt;

/// Status enum representing success/error states for Result types
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Status {
    /// Operation completed successfully
    SUCCESS,
    /// Operation encountered an error
    ERROR,
}

impl Status {
    /// Check if status is SUCCESS
    pub fn is_success(self) -> bool {
        matches!(self, Status::SUCCESS)
    }

    /// Check if status is ERROR
    pub fn is_error(self) -> bool {
        matches!(self, Status::ERROR)
    }

    /// Convert from boolean (true = SUCCESS, false = ERROR)
    pub fn from_bool(success: bool) -> Self {
        if success {
            Status::SUCCESS
        } else {
            Status::ERROR
        }
    }

    /// Convert to boolean
    pub fn as_bool(self) -> bool {
        self.is_success()
    }
}

impl fmt::Display for Status {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Status::SUCCESS => write!(f, "SUCCESS"),
            Status::ERROR => write!(f, "ERROR"),
        }
    }
}

use std::fmt;

/// Result of a YAML parsing operation
#[derive(Debug, Clone)]
pub struct ParseResult<T> {
    /// The parsed value, if successful
    value: Option<T>,
    /// The error, if parsing failed
    error: Option<ParseError>,
    /// Additional metadata about the parse operation
    metadata: ParseMetadata,
}

impl<T> ParseResult<T> {
    /// Create a successful parse result
    pub fn success(value: T) -> Self {
        Self {
            value: Some(value),
            error: None,
            metadata: ParseMetadata::default(),
        }
    }

    /// Create a failed parse result
    pub fn failure(error: ParseError) -> Self {
        Self {
            value: None,
            error: Some(error),
            metadata: ParseMetadata::default(),
        }
    }

    /// Check if the parse was successful
    pub fn is_success(&self) -> bool {
        self.error.is_none() && self.value.is_some()
    }

    /// Check if the parse failed
    pub fn is_failure(&self) -> bool {
        self.error.is_some()
    }

    /// Get the parsed value
    ///
    /// Returns None if the parse failed
    pub fn value(&self) -> Option<&T> {
        self.value.as_ref()
    }

    /// Get the error, if any
    pub fn error(&self) -> Option<&ParseError> {
        self.error.as_ref()
    }

    /// Get the metadata for this parse result
    pub fn metadata(&self) -> &ParseMetadata {
        &self.metadata
    }

    /// Unwrap the value, consuming the result
    ///
    /// Panics if the parse failed
    pub fn unwrap(self) -> T {
        self.value.expect(
            "called unwrap() on a failed ParseResult"
        )
    }

    /// Unwrap the value or return a default
    pub fn unwrap_or(self, default: T) -> T {
        self.value.unwrap_or(default)
    }

    /// Map the success value to a new type
    pub fn map<U, F>(self, f: F) -> ParseResult<U>
    where
        F: FnOnce(T) -> U,
    {
        match self.value {
            Some(v) => ParseResult {
                value: Some(f(v)),
                error: None,
                metadata: self.metadata,
            },
            None => ParseResult {
                value: None,
                error: self.error,
                metadata: self.metadata,
            },
        }
    }
}

impl<T> From<Result<T>> for ParseResult<T> {
    fn from(result: Result<T>) -> Self {
        match result {
            Ok(value) => Self::success(value),
            Err(error) => Self::failure(error),
        }
    }
}

/// Metadata about a parsing operation
#[derive(Debug, Clone, Default)]
pub struct ParseMetadata {
    /// Number of lines processed
    pub lines_processed: usize,
    /// Number of bytes processed
    pub bytes_processed: usize,
    /// Processing time in nanoseconds
    pub processing_time_ns: Option<u64>,
    /// Source file path, if known
    pub source_path: Option<String>,
}

impl ParseMetadata {
    /// Create new metadata
    pub fn new() -> Self {
        Self::default()
    }

    /// Set the number of lines processed
    pub fn with_lines(mut self, lines: usize) -> Self {
        self.lines_processed = lines;
        self
    }

    /// Set the number of bytes processed
    pub fn with_bytes(mut self, bytes: usize) -> Self {
        self.bytes_processed = bytes;
        self
    }

    /// Set the source path
    pub fn with_source(mut self, path: impl Into<String>) -> Self {
        self.source_path = Some(path.into());
        self
    }
}

/// Result of a YAML validation operation
#[derive(Debug, Clone)]
pub struct ValidationResult {
    /// Whether validation passed
    pub valid: bool,
    /// List of validation errors
    pub errors: Vec<ValidationError>,
    /// List of validation warnings
    pub warnings: Vec<ValidationWarning>,
}

impl ValidationResult {
    /// Create a successful validation result
    pub fn success() -> Self {
        Self {
            valid: true,
            errors: Vec::new(),
            warnings: Vec::new(),
        }
    }

    /// Create a failed validation result
    pub fn failure(errors: Vec<ValidationError>) -> Self {
        Self {
            valid: false,
            errors,
            warnings: Vec::new(),
        }
    }

    /// Check if validation passed
    pub fn is_valid(&self) -> bool {
        self.valid && self.errors.is_empty()
    }

    /// Check if there are any errors
    pub fn has_errors(&self) -> bool {
        !self.errors.is_empty()
    }

    /// Check if there are any warnings
    pub fn has_warnings(&self) -> bool {
        !self.warnings.is_empty()
    }
}

impl Default for ValidationResult {
    fn default() -> Self {
        Self::success()
    }
}

/// A validation error
#[derive(Debug, Clone)]
pub struct ValidationError {
    /// Path to the invalid element (e.g., "server.port")
    pub path: String,
    /// Error message
    pub message: String,
    /// Line number where the error occurred (1-indexed)
    pub line: Option<usize>,
}

/// A validation warning
#[derive(Debug, Clone)]
pub struct ValidationWarning {
    /// Path to the element that triggered the warning
    pub path: String,
    /// Warning message
    pub message: String,
    /// Line number where the warning occurred (1-indexed)
    pub line: Option<usize>,
}
