//! Error types for YAML parsing
//!
//! This module defines the error types used throughout the YAML parser.
//!
//! # Error Handling Philosophy
//!
//! The `ParseError` type follows a structured approach to error handling that emphasizes:
//!
//! 1. **Clear categorization** - Each error falls into a distinct category (syntax, I/O, validation, etc.)
//! 2. **Rich context** - Errors carry location information, context messages, and code snippets
//! 3. **Composability** - Errors propagate cleanly through the call stack using Rust's `?` operator
//! 4. **User-friendly output** - Multiple formatting options for different use cases (logging, UI, debugging)
//!
//! ## When to Use Each Variant
//!
//! ### `ParseErrorKind::Io` vs Other Variants
//!
//! Use [`ParseErrorKind::Io`] for **file system and I/O operation failures**:
//! - File not found (`std::io::ErrorKind::NotFound`)
//! - Permission denied (`std::io::ErrorKind::PermissionDenied`)
//! - Read/write failures (`std::io::ErrorKind::Other`)
//! - Network I/O errors (if applicable)
//!
//! **Do not use** `Io` for:
//! - YAML syntax errors → use [`ParseErrorKind::Syntax`]
//! - Type mismatches → use [`ParseErrorKind::TypeMismatch`]
//! - Constraint violations → use [`ParseErrorKind::Validation`]
//!
//! ```ignore
//! // ✅ Correct: Use Io for file system errors
//! let content = std::fs::read_to_string(path)?;  // io::Error → Io
//!
//! // ❌ Incorrect: Don't use Io for parsing errors
//! // let error = ParseError::io("invalid YAML structure");  // Wrong! Use Syntax instead
//! ```
//!
//! ### `ParseErrorKind::InvalidUtf8` vs Other Variants
//!
//! Use [`ParseErrorKind::InvalidUtf8`] specifically for **encoding errors**:
//! - Invalid UTF-8 byte sequences in input
//! - Failed UTF-8 validation when converting bytes to strings
//!
//! **Do not use** `InvalidUtf8` for:
//! - I/O errors during file reading → use [`ParseErrorKind::Io`]
//! - General parsing failures → use [`ParseErrorKind::Syntax`]
//!
//! ```ignore
//! // ✅ Correct: Use InvalidUtf8 for encoding errors
//! let string = String::from_utf8(bytes)?;  // FromUtf8Error → InvalidUtf8
//!
//! // ❌ Incorrect: Don't use InvalidUtf8 for file reading errors
//! // let error = ParseError::new(ParseErrorKind::InvalidUtf8);  // Wrong! Use Io instead
//! ```
//!
//! ### `ParseErrorKind::UnexpectedEof` vs `ParseErrorKind::Syntax`
//!
//! Use [`ParseErrorKind::UnexpectedEof`] when **input ends prematurely**:
//! - Incomplete YAML documents (missing closing brackets, braces, quotes)
//! - Truncated files or streams
//! - Multi-document YAML streams that end mid-document
//!
//! Use [`ParseErrorKind::Syntax`] for **general YAML syntax violations**:
//! - Invalid indentation
//! - Invalid escape sequences
//! - Malformed scalars or mappings
//! - Any YAML grammar violation that's not specifically EOF-related
//!
//! ```ignore
//! // ✅ Correct: Use UnexpectedEof for incomplete input
//! if input.ends_with("key: ") {
//!     return Err(ParseError::new(ParseErrorKind::UnexpectedEof));
//! }
//!
//! // ✅ Correct: Use Syntax for general YAML errors
//! if !is_valid_indentation(line) {
//!     return Err(ParseError::syntax("invalid indentation"));
//! }
//! ```
//!
//! ### `ParseErrorKind::TypeMismatch` vs `ParseErrorKind::Validation`
//!
//! Use [`ParseErrorKind::TypeMismatch`] when **a value has the wrong Rust/YAML type**:
//! - Expecting an integer but finding a string
//! - Expecting a sequence but finding a scalar
//! - Expecting a boolean but finding a number
//!
//! Use [`ParseErrorKind::Validation`] for **semantic constraint violations**:
//! - Value out of allowed range (e.g., port number > 65535)
//! - String doesn't match required pattern (e.g., invalid email format)
//! - Array length constraints violated
//! - Business logic or schema validation failures
//!
//! The key distinction: `TypeMismatch` is about **type**, `Validation` is about **value**.
//!
//! ```ignore
//! // ✅ Correct: Use TypeMismatch for type errors
//! let port = value["port"].as_i64()
//!     .ok_or_else(|| ParseError::type_mismatch("port", "integer", "string"))?;
//!
//! // ✅ Correct: Use Validation for value constraints
//! if port < 1 || port > 65535 {
//!     return Err(ParseError::validation("port must be between 1 and 65535"));
//! }
//! ```
//!
//! ### `ParseErrorKind::DuplicateKey` vs `ParseErrorKind::Validation`
//!
//! Use [`ParseErrorKind::DuplicateKey`] specifically for **duplicate key errors**:
//! - YAML mappings with repeated keys (YAML 1.2 spec violation)
//! - Configuration files with conflicting entries
//!
//! Use [`ParseErrorKind::Validation`] for **general validation failures** including:
//! - Missing required fields (not duplicated, but absent)
//! - Inter-field constraint violations (e.g., "start_time > end_time")
//! - Schema validation failures beyond type/duplicate checking
//!
//! ```ignore
//! // ✅ Correct: Use DuplicateKey for duplicate keys
//! if keys.contains(&key) {
//!     return Err(ParseError::new(ParseErrorKind::DuplicateKey(key)));
//! }
//!
//! // ✅ Correct: Use Validation for missing required fields
//! if value["name"].is_null() {
//!     return Err(ParseError::validation("missing required field: 'name'"));
//! }
//! ```
//!
//! ### `ParseErrorKind::Other` (Catch-all) vs Specific Variants
//!
//! Use [`ParseErrorKind::Other`] **only as a last resort** for errors that don't fit other categories:
//! - Unclassifiable serde_yaml errors
//! - Errors from external libraries that don't map cleanly
//! - Temporary error cases during refactoring
//!
//! **Prefer specific variants** whenever possible:
//! - Unknown anchors → [`ParseErrorKind::UnknownAnchor`]
//! - Syntax errors → [`ParseErrorKind::Syntax`]
//! - I/O errors → [`ParseErrorKind::Io`]
//! - Type errors → [`ParseErrorKind::TypeMismatch`]
//!
//! If you find yourself using `Other` frequently, consider adding a new specific variant.
//!
//! ```ignore
//! // ✅ Prefer specific variants
//! return Err(ParseError::syntax("invalid YAML"));
//!
//! // ⚠️ Use Other only as a catch-all
//! return Err(ParseError::new(ParseErrorKind::Other("unclassified error".to_string())));
//! ```
//!
//! # Error Propagation Strategy
//!
//! The `ParseError` type is designed to work seamlessly with Rust's error propagation
//! mechanisms. It implements `From` for common error types, enabling the use of the `?`
//! operator for automatic error conversion.
//!
//! ## Basic Propagation with `?`
//!
//! ```ignore
//! use armor::parsers::yaml::{ParseError, Result};
//!
//! fn parse_config(path: &Path) -> Result<Config> {
//!     let content = std::fs::read_to_string(path)?;  // io::Error → ParseError
//!     parse_yaml(&content)
//! }
//! ```
//!
//! ## Adding Context with `.context()`
//!
//! Use the builder-style `.context()` method to add contextual information:
//!
//! ```ignore
//! fn parse_database_config(value: &serde_yaml::Value) -> Result<DatabaseConfig> {
//!     let port = value["port"]
//!         .as_i64()
//!         .ok_or_else(|| ParseError::type_mismatch("port", "integer", "null"))
//!         .context("while parsing database configuration")?;
//!
//!     Ok(DatabaseConfig { port })
//! }
//! ```
//!
//! ## Converting from Other Error Types
//!
//! `ParseError` implements `From` for:
//! - `std::io::Error` → [`ParseErrorKind::Io`]
//! - `serde_yaml::Error` → Appropriate [`ParseErrorKind`] based on error classification
//! - `std::str::Utf8Error` → [`ParseErrorKind::InvalidUtf8`]
//! - `std::string::FromUtf8Error` → [`ParseErrorKind::InvalidUtf8`]
//!
//! ## Custom Error Creation
//!
//! For domain-specific errors, create them directly:
//!
//! ```ignore
//! fn validate_port(port: i64) -> Result<u16> {
//!     if port < 1 || port > 65535 {
//!         return Err(ParseError::validation("port must be between 1 and 65535"));
//!     }
//!     Ok(port as u16)
//! }
//! ```
//!
//! # Examples
//!
//! ## Basic Error Creation
//!
//! Creating errors using convenience constructors:
//!
//! ```
//! use armor::parsers::yaml::ParseError;
//!
//! // Create a syntax error
//! let syntax_err = ParseError::syntax("invalid YAML indentation");
//! assert!(syntax_err.is_syntax());
//!
//! // Create a validation error
//! let validation_err = ParseError::validation("port must be between 1 and 65535");
//! assert!(validation_err.is_validation());
//!
//! // Create a type mismatch error
//! let type_err = ParseError::type_mismatch("port", "integer", "string");
//! assert!(type_err.is_type_mismatch());
//! assert_eq!(type_err.summary(), "<unknown>: type mismatch at 'port': expected integer, got string");
//! ```
//!
//! ## Error Propagation with `?`
//!
//! Using the `?` operator to automatically convert errors:
//!
//! ```
//! use armor::parsers::yaml::{ParseError, Result};
//! use std::fs;
//!
//! fn read_config(path: &str) -> Result<String> {
//!     // io::Error is automatically converted to ParseError via From impl
//!     let content = fs::read_to_string(path)?;
//!     Ok(content)
//! }
//!
//! // This function demonstrates automatic error conversion
//! fn parse_config_size(path: &str) -> Result<usize> {
//!     let content = read_config(path)?;  // ParseError propagates via ?
//!     Ok(content.len())
//! }
//!
//! // Example: successful read
//! assert!(parse_config_size("/dev/null").is_ok());
//! ```
//!
//! ## Custom Error Handling with Builder Pattern
//!
//! Using builder methods to add context and location information:
//!
//! ```
//! use armor::parsers::yaml::ParseError;
//!
//! fn validate_service_config(yaml_content: &str) -> Result<(), ParseError> {
//!     // Simulating an error during validation
//!     if yaml_content.contains("port: abc") {
//!         let error = ParseError::type_mismatch("service.port", "integer", "string")
//!             .with_path("config/services.yaml")
//!             .with_line(5)
//!             .with_column(10)
//!             .with_context("while validating service configuration")
//!             .with_snippet("services:\n  - name: web\n    port: abc");
//!
//!         return Err(error);
//!     }
//!     Ok(())
//! }
//!
//! // Create an error to verify the builder pattern
//! let error = ParseError::validation("invalid port")
//!     .with_path("config.yaml")
//!     .with_line(10)
//!     .with_column(5)
//!     .with_context("while parsing configuration");
//!
//! assert_eq!(error.path, Some("config.yaml".to_string()));
//! assert_eq!(error.line, Some(10));
//! assert_eq!(error.column, Some(5));
//! assert_eq!(error.context, "while parsing configuration");
//! ```
//!
//! ## Error Display and Formatting
//!
//! Different ways to format and display errors:
//!
//! ```
//! use armor::parsers::yaml::ParseError;
//!
//! // Create an error with full context
//! let error = ParseError::syntax("unexpected colon")
//!     .with_path("config/database.yaml")
//!     .with_line(15)
//!     .with_column(8)
//!     .with_context("while parsing database configuration")
//!     .with_snippet("database:\n  host: localhost\n  port: 5432:\n    invalid");
//!
//! // Get a single-line summary (good for logging)
//! let summary = error.summary();
//! assert!(summary.contains("config/database.yaml:15"));
//! assert!(summary.contains("syntax error"));
//! assert!(summary.contains("while parsing database configuration"));
//!
//! // Get a detailed multi-line report (good for user display)
//! let detailed = error.detailed_report();
//! assert!(detailed.contains("error:"));
//! assert!(detailed.contains("snippet:"));
//!
//! // Get a structured representation (good for debugging)
//! let structured = error.format_structured();
//! assert!(structured.contains("ParseError"));
//! assert!(structured.contains("config/database.yaml:15:8"));
//! assert!(structured.contains("line: Some(15)"));
//!
//! // Display implementation provides user-friendly output
//! let display_string = format!("{}", error);
//! assert!(display_string.contains("config/database.yaml"));
//! assert!(display_string.contains("syntax error"));
//! ```
//!
//! ## Error Conversion from Standard Types
//!
//! Automatic conversion from standard library errors:
//!
//! ```
//! use armor::parsers::yaml::ParseError;
//! use std::io;
//!
//! // io::Error converts to ParseError::Io
//! let io_err = io::Error::new(io::ErrorKind::NotFound, "file not found");
//! let parse_err: ParseError = io_err.into();
//! assert!(parse_err.is_io());
//! assert!(parse_err.summary().contains("I/O error"));
//! assert!(parse_err.summary().contains("file not found"));
//!
//! // String::from_utf8 Error converts to ParseError::InvalidUtf8
//! let invalid_bytes = b"\xff\xfe\xfd";
//! let utf8_err = String::from_utf8(invalid_bytes.to_vec()).unwrap_err();
//! let parse_err: ParseError = utf8_err.into();
//! assert!(matches!(parse_err.kind, armor::parsers::yaml::ParseErrorKind::InvalidUtf8));
//! ```
//!
//! ## Working with Error Types
//!
//! Checking error types and handling specific cases:
//!
//! ```
//! use armor::parsers::yaml::ParseError;
//!
//! let errors = vec![
//!     ParseError::syntax("invalid token"),
//!     ParseError::io("file not found"),
//!     ParseError::validation("port out of range"),
//!     ParseError::type_mismatch("field", "string", "integer"),
//! ];
//!
//! // Check specific error types
//! assert!(errors[0].is_syntax());
//! assert!(errors[1].is_io());
//! assert!(errors[2].is_validation());
//! assert!(errors[3].is_type_mismatch());
//!
//! // Filter errors by type
//! let syntax_errors: Vec<_> = errors.iter()
//!     .filter(|e| e.is_syntax())
//!     .collect();
//! assert_eq!(syntax_errors.len(), 1);
//! ```

use std::fmt;

/// Main error type for YAML parsing operations
#[derive(Clone)]
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
    ///
    /// This is the base constructor for creating a ParseError. All fields except
    /// `kind` are set to their default values (None or empty). Use the builder methods
    /// to add additional context and location information.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::{ParseError, ParseErrorKind};
    ///
    /// let error = ParseError::new(ParseErrorKind::UnexpectedEof);
    /// assert!(matches!(error.kind, ParseErrorKind::UnexpectedEof));
    /// assert_eq!(error.line, None);
    /// ```
    ///
    /// For common error types, prefer using the convenience constructors:
    /// - [`ParseError::syntax()`] for syntax errors
    /// - [`ParseError::io()`] for I/O errors
    /// - [`ParseError::validation()`] for validation errors
    /// - [`ParseError::type_mismatch()`] for type mismatch errors
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
    ///
    /// Line numbers are 1-indexed, matching typical text editor conventions.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseError;
    ///
    /// let error = ParseError::syntax("invalid token")
    ///     .with_line(42);
    /// assert_eq!(error.line, Some(42));
    /// ```
    pub fn with_line(mut self, line: usize) -> Self {
        self.line = Some(line);
        self
    }

    /// Set the column number for this error
    ///
    /// Column numbers are 1-indexed, indicating the character position within the line.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseError;
    ///
    /// let error = ParseError::syntax("invalid token")
    ///     .with_column(15);
    /// assert_eq!(error.column, Some(15));
    /// ```
    pub fn with_column(mut self, column: usize) -> Self {
        self.column = Some(column);
        self
    }

    /// Set the file/source path for this error
    ///
    /// The path can be absolute or relative. It's displayed in error messages to help
    /// users locate the source of the error.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseError;
    ///
    /// let error = ParseError::syntax("invalid YAML")
    ///     .with_path("config/services.yaml");
    /// assert_eq!(error.path, Some("config/services.yaml".to_string()));
    /// ```
    pub fn with_path(mut self, path: impl Into<String>) -> Self {
        self.path = Some(path.into());
        self
    }

    /// Set the code snippet for this error
    ///
    /// The snippet should contain the relevant lines of source code where the error occurred.
    /// This is displayed in detailed error reports to help users understand the context.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseError;
    ///
    /// let error = ParseError::syntax("invalid port value")
    ///     .with_line(5)
    ///     .with_snippet("service:\n  name: web\n  port: abc");
    ///
    /// let report = error.detailed_report();
    /// assert!(report.contains("service:"));
    /// assert!(report.contains("port: abc"));
    /// ```
    pub fn with_snippet(mut self, snippet: impl Into<String>) -> Self {
        self.snippet = Some(snippet.into());
        self
    }

    /// Set the context message for this error
    ///
    /// Context provides additional information about what operation was being performed
    /// when the error occurred. This helps users understand the broader scenario.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseError;
    ///
    /// let error = ParseError::validation("port out of range")
    ///     .with_context("while parsing database configuration");
    ///
    /// let summary = error.summary();
    /// assert!(summary.contains("while parsing database configuration"));
    /// ```
    pub fn with_context(mut self, context: impl Into<String>) -> Self {
        self.context = context.into();
        self
    }

    /// Set both line and column for this error
    ///
    /// Convenience method for setting location with a single call.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseError;
    ///
    /// let error = ParseError::syntax("invalid token")
    ///     .with_location(10, 25);
    /// assert_eq!(error.line, Some(10));
    /// assert_eq!(error.column, Some(25));
    /// assert_eq!(error.location_string(), "10:25");
    /// ```
    pub fn with_location(mut self, line: usize, column: usize) -> Self {
        self.line = Some(line);
        self.column = Some(column);
        self
    }

    /// Get a formatted location string (e.g., "file.yaml:10:5" or "<unknown>:10")
    ///
    /// This method generates a human-readable location string based on the available
    /// location information (path, line, column). The format adapts based on which fields are set.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseError;
    ///
    /// // Full location with path, line, and column
    /// let error = ParseError::syntax("test")
    ///     .with_path("config.yaml")
    ///     .with_location(10, 5);
    /// assert_eq!(error.location_string(), "config.yaml:10:5");
    ///
    /// // Line and column only
    /// let error = ParseError::syntax("test")
    ///     .with_location(42, 15);
    /// assert_eq!(error.location_string(), "42:15");
    ///
    /// // Path only
    /// let error = ParseError::syntax("test")
    ///     .with_path("config.yaml");
    /// assert_eq!(error.location_string(), "config.yaml");
    ///
    /// // Unknown location
    /// let error = ParseError::syntax("test");
    /// assert_eq!(error.location_string(), "<unknown>");
    /// ```
    pub fn location_string(&self) -> String {
        match (&self.path, self.line, self.column) {
            (Some(path), Some(line), Some(col)) => format!("{}:{}:{}", path, line, col),
            (Some(path), Some(line), None) => format!("{}:{}", path, line),
            (Some(path), None, Some(col)) => format!("{}::{}", path, col),
            (Some(path), None, None) => path.clone(),
            (None, Some(line), Some(col)) => format!("{}:{}", line, col),
            (None, Some(line), None) => format!("{}", line),
            (None, None, Some(col)) => format!("col {}", col),
            (None, None, None) => "<unknown>".to_string(),
        }
    }

    /// Get a brief summary of the error (single line, suitable for logging)
    ///
    /// The summary includes location, error kind, and context (if set). This is ideal
    /// for logging purposes where a compact, single-line error message is desired.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseError;
    ///
    /// // Summary with context
    /// let error = ParseError::syntax("invalid token")
    ///     .with_path("config.yaml")
    ///     .with_line(10)
    ///     .with_context("while parsing services");
    /// assert_eq!(error.summary(),
    ///     "config.yaml:10: syntax error: invalid token - while parsing services");
    ///
    /// // Summary without context
    /// let error = ParseError::syntax("invalid token")
    ///     .with_path("config.yaml")
    ///     .with_line(10);
    /// assert_eq!(error.summary(),
    ///     "config.yaml:10: syntax error: invalid token");
    /// ```
    pub fn summary(&self) -> String {
        let location = self.location_string();
        if !self.context.is_empty() {
            format!("{}: {} - {}", location, self.kind, self.context)
        } else {
            format!("{}: {}", location, self.kind)
        }
    }

    /// Get a detailed multi-line error report with snippet and visual indicator
    ///
    /// This method generates a comprehensive error report suitable for displaying
    /// to users. It includes the error summary, context (if set), and a code snippet
    /// with a visual indicator (^) pointing to the error location.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseError;
    ///
    /// let error = ParseError::syntax("invalid port value")
    ///     .with_path("config.yaml")
    ///     .with_line(5)
    ///     .with_column(10)
    ///     .with_context("while parsing service configuration")
    ///     .with_snippet("service:\n  port: abc");
    ///
    /// let report = error.detailed_report();
    /// assert!(report.contains("error:"));
    /// assert!(report.contains("config.yaml:5:10"));
    /// assert!(report.contains("syntax error: invalid port value"));
    /// assert!(report.contains("while parsing service configuration"));
    /// assert!(report.contains("snippet:"));
    /// ```
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
    ///
    /// This method produces a structured representation of the error suitable for
    /// programmatic analysis or structured logging systems.
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseError;
    ///
    /// let error = ParseError::type_mismatch("port", "integer", "string")
    ///     .with_path("config.yaml")
    ///     .with_line(8)
    ///     .with_column(10);
    ///
    /// let formatted = error.format_structured();
    /// assert!(formatted.contains("ParseError"));
    /// assert!(formatted.contains("config.yaml:8:10"));
    /// assert!(formatted.contains("line: Some(8)"));
    /// assert!(formatted.contains("column: Some(10)"));
    /// ```
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

impl PartialEq for ParseError {
    fn eq(&self, other: &Self) -> bool {
        // Only compare core error identification fields
        // Context and snippet are NOT included in equality
        self.kind == other.kind
            && self.line == other.line
            && self.column == other.column
            && self.path == other.path
    }
}

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

// ============================================================================
// From implementations for error propagation
// ============================================================================

impl From<std::io::Error> for ParseError {
    fn from(err: std::io::Error) -> Self {
        ParseError::new(ParseErrorKind::Io(err.to_string()))
    }
}

impl From<serde_yaml::Error> for ParseError {
    fn from(err: serde_yaml::Error) -> Self {
        // Classify serde_yaml errors into appropriate ParseError kinds
        // based on the error message content
        let err_msg = err.to_string().to_lowercase();

        let kind = if err_msg.contains("syntax") || err_msg.contains("unexpected") || err_msg.contains("expected") {
            ParseErrorKind::Syntax(err.to_string())
        } else if err_msg.contains("duplicate") {
            ParseErrorKind::DuplicateKey(err.to_string())
        } else if err_msg.contains("io") || err_msg.contains("failed to read") {
            ParseErrorKind::Io(err.to_string())
        } else {
            ParseErrorKind::Other(err.to_string())
        };

        ParseError::new(kind)
    }
}

impl From<std::str::Utf8Error> for ParseError {
    fn from(err: std::str::Utf8Error) -> Self {
        ParseError::new(ParseErrorKind::InvalidUtf8).with_context(err.to_string())
    }
}

impl From<std::string::FromUtf8Error> for ParseError {
    fn from(err: std::string::FromUtf8Error) -> Self {
        ParseError::new(ParseErrorKind::InvalidUtf8).with_context(err.to_string())
    }
}
