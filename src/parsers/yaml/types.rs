//! Result types for YAML parsing operations
//!
//! This module defines the result types used by the YAML parser.
//! This module also integrates error types from bf-68hqo for comprehensive
//! error categorization and handling.

use crate::parsers::yaml::error::{ParseError, Result};
use std::fmt;

/// Error type categorization for validation and parsing errors
///
/// This enum defines the high-level categories of errors that can occur
/// during YAML processing and schema validation, following the bf-68hqo
/// error hierarchy design.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ErrorType {
    /// Syntax errors in YAML source (invalid structure, malformed tokens)
    Syntax,
    /// I/O errors (file not found, permission denied, read failures)
    Io,
    /// Validation errors (constraint violations, required field missing)
    Validation,
    /// Type mismatch errors (unexpected type for a field)
    TypeMismatch,
    /// UTF-8 encoding errors
    InvalidUtf8,
    /// Unexpected end of input
    UnexpectedEof,
    /// Unknown anchor or alias references
    UnknownAnchor,
    /// Duplicate key in mapping
    DuplicateKey,
    /// Other uncategorized errors
    Other,
}

impl fmt::Display for ErrorType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Syntax => write!(f, "syntax error"),
            Self::Io => write!(f, "I/O error"),
            Self::Validation => write!(f, "validation error"),
            Self::TypeMismatch => write!(f, "type mismatch"),
            Self::InvalidUtf8 => write!(f, "invalid UTF-8 encoding"),
            Self::UnexpectedEof => write!(f, "unexpected end of input"),
            Self::UnknownAnchor => write!(f, "unknown anchor"),
            Self::DuplicateKey => write!(f, "duplicate key"),
            Self::Other => write!(f, "error"),
        }
    }
}

/// Specific error codes for programmatic error handling
///
/// These codes provide machine-readable identifiers for specific error conditions,
/// enabling automated error handling and reporting. Following the bf-68hqo design,
/// each code maps to a specific validation or parsing failure scenario.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[allow(non_camel_case_types)]
pub enum ErrorCode {
    // ========== Syntax Errors ==========
    /// Invalid YAML syntax structure
    YAML_INVALID_SYNTAX,
    /// Invalid indentation in YAML
    YAML_INVALID_INDENTATION,
    /// Invalid delimiter (colon, bracket, brace)
    YAML_INVALID_DELIMITER,
    /// Invalid escape sequence
    YAML_INVALID_ESCAPE_SEQUENCE,
    /// Invalid scalar value
    YAML_INVALID_SCALAR,

    // ========== Type Mismatches ==========
    /// Expected integer, got another type
    TYPE_EXPECTED_INTEGER,
    /// Expected string, got another type
    TYPE_EXPECTED_STRING,
    /// Expected boolean, got another type
    TYPE_EXPECTED_BOOLEAN,
    /// Expected array, got another type
    TYPE_EXPECTED_ARRAY,
    /// Expected object/map, got another type
    TYPE_EXPECTED_OBJECT,
    /// Expected number, got another type
    TYPE_EXPECTED_NUMBER,
    /// Null value where non-null required
    TYPE_UNEXPECTED_NULL,

    // ========== Validation Errors ==========
    /// Required field is missing
    VALIDATION_REQUIRED_FIELD_MISSING,
    /// Value out of allowed range
    VALIDATION_VALUE_OUT_OF_RANGE,
    /// String too short
    VALIDATION_STRING_TOO_SHORT,
    /// String too long
    VALIDATION_STRING_TOO_LONG,
    /// String doesn't match pattern
    VALIDATION_PATTERN_MISMATCH,
    /// Value not in allowed set
    VALIDATION_INVALID_VALUE,
    /// Array has too few items
    VALIDATION_ARRAY_TOO_FEW_ITEMS,
    /// Array has too many items
    VALIDATION_ARRAY_TOO_MANY_ITEMS,
    /// Array items not unique
    VALIDATION_ARRAY_NOT_UNIQUE,
    /// Object has too few properties
    VALIDATION_OBJECT_TOO_FEW_PROPERTIES,
    /// Object has too many properties
    VALIDATION_OBJECT_TOO_MANY_PROPERTIES,

    // ========== I/O Errors ==========
    /// File not found
    IO_FILE_NOT_FOUND,
    /// Permission denied
    IO_PERMISSION_DENIED,
    /// Read failed
    IO_READ_FAILED,
    /// Write failed
    IO_WRITE_FAILED,

    // ========== Encoding Errors ==========
    /// Invalid UTF-8 sequence
    ENCODING_INVALID_UTF8,

    // ========== Other Errors ==========
    /// Unknown anchor reference
    ANCHOR_UNKNOWN,
    /// Duplicate key in mapping
    KEY_DUPLICATE,
    /// Unexpected end of input
    EOF_UNEXPECTED,
}

impl ErrorCode {
    /// Get the error type category for this error code
    pub fn error_type(&self) -> ErrorType {
        match self {
            // Syntax errors
            Self::YAML_INVALID_SYNTAX | Self::YAML_INVALID_INDENTATION |
            Self::YAML_INVALID_DELIMITER | Self::YAML_INVALID_ESCAPE_SEQUENCE |
            Self::YAML_INVALID_SCALAR => ErrorType::Syntax,

            // Type mismatches
            Self::TYPE_EXPECTED_INTEGER | Self::TYPE_EXPECTED_STRING |
            Self::TYPE_EXPECTED_BOOLEAN | Self::TYPE_EXPECTED_ARRAY |
            Self::TYPE_EXPECTED_OBJECT | Self::TYPE_EXPECTED_NUMBER |
            Self::TYPE_UNEXPECTED_NULL => ErrorType::TypeMismatch,

            // Validation errors
            Self::VALIDATION_REQUIRED_FIELD_MISSING | Self::VALIDATION_VALUE_OUT_OF_RANGE |
            Self::VALIDATION_STRING_TOO_SHORT | Self::VALIDATION_STRING_TOO_LONG |
            Self::VALIDATION_PATTERN_MISMATCH | Self::VALIDATION_INVALID_VALUE |
            Self::VALIDATION_ARRAY_TOO_FEW_ITEMS | Self::VALIDATION_ARRAY_TOO_MANY_ITEMS |
            Self::VALIDATION_ARRAY_NOT_UNIQUE | Self::VALIDATION_OBJECT_TOO_FEW_PROPERTIES |
            Self::VALIDATION_OBJECT_TOO_MANY_PROPERTIES => ErrorType::Validation,

            // I/O errors
            Self::IO_FILE_NOT_FOUND | Self::IO_PERMISSION_DENIED |
            Self::IO_READ_FAILED | Self::IO_WRITE_FAILED => ErrorType::Io,

            // Encoding errors
            Self::ENCODING_INVALID_UTF8 => ErrorType::InvalidUtf8,

            // Other errors
            Self::ANCHOR_UNKNOWN => ErrorType::UnknownAnchor,
            Self::KEY_DUPLICATE => ErrorType::DuplicateKey,
            Self::EOF_UNEXPECTED => ErrorType::UnexpectedEof,
        }
    }

    /// Get a human-readable description for this error code
    pub fn description(&self) -> &'static str {
        match self {
            // Syntax errors
            Self::YAML_INVALID_SYNTAX => "Invalid YAML syntax structure",
            Self::YAML_INVALID_INDENTATION => "Invalid YAML indentation",
            Self::YAML_INVALID_DELIMITER => "Invalid YAML delimiter",
            Self::YAML_INVALID_ESCAPE_SEQUENCE => "Invalid escape sequence",
            Self::YAML_INVALID_SCALAR => "Invalid scalar value",

            // Type mismatches
            Self::TYPE_EXPECTED_INTEGER => "Expected integer type",
            Self::TYPE_EXPECTED_STRING => "Expected string type",
            Self::TYPE_EXPECTED_BOOLEAN => "Expected boolean type",
            Self::TYPE_EXPECTED_ARRAY => "Expected array type",
            Self::TYPE_EXPECTED_OBJECT => "Expected object type",
            Self::TYPE_EXPECTED_NUMBER => "Expected number type",
            Self::TYPE_UNEXPECTED_NULL => "Unexpected null value",

            // Validation errors
            Self::VALIDATION_REQUIRED_FIELD_MISSING => "Required field is missing",
            Self::VALIDATION_VALUE_OUT_OF_RANGE => "Value out of allowed range",
            Self::VALIDATION_STRING_TOO_SHORT => "String too short",
            Self::VALIDATION_STRING_TOO_LONG => "String too long",
            Self::VALIDATION_PATTERN_MISMATCH => "Pattern mismatch",
            Self::VALIDATION_INVALID_VALUE => "Invalid value",
            Self::VALIDATION_ARRAY_TOO_FEW_ITEMS => "Array has too few items",
            Self::VALIDATION_ARRAY_TOO_MANY_ITEMS => "Array has too many items",
            Self::VALIDATION_ARRAY_NOT_UNIQUE => "Array items not unique",
            Self::VALIDATION_OBJECT_TOO_FEW_PROPERTIES => "Object has too few properties",
            Self::VALIDATION_OBJECT_TOO_MANY_PROPERTIES => "Object has too many properties",

            // I/O errors
            Self::IO_FILE_NOT_FOUND => "File not found",
            Self::IO_PERMISSION_DENIED => "Permission denied",
            Self::IO_READ_FAILED => "Read failed",
            Self::IO_WRITE_FAILED => "Write failed",

            // Encoding errors
            Self::ENCODING_INVALID_UTF8 => "Invalid UTF-8 encoding",

            // Other errors
            Self::ANCHOR_UNKNOWN => "Unknown anchor reference",
            Self::KEY_DUPLICATE => "Duplicate key in mapping",
            Self::EOF_UNEXPECTED => "Unexpected end of input",
        }
    }
}

impl fmt::Display for ErrorCode {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}

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

/// Result of a YAML operation with status, data, and error fields
///
/// This is a simple dataclass-style result type that holds:
/// - `status`: A Status enum indicating SUCCESS or ERROR
/// - `data`: Optional generic data payload
/// - `error`: Optional error message string
#[derive(Debug, Clone)]
pub struct OperationResult<T> {
    /// The operation status (SUCCESS or ERROR)
    pub status: Status,
    /// The parsed content data, if successful
    pub data: Option<T>,
    /// The error message, if the operation failed
    pub error: Option<String>,
}

impl<T> OperationResult<T> {
    /// Create a new OperationResult with all three fields
    ///
    /// # Arguments
    /// * `status` - The Status enum value (SUCCESS or ERROR)
    /// * `data` - Optional data payload
    /// * `error` - Optional error message
    pub fn new(status: Status, data: Option<T>, error: Option<String>) -> Self {
        Self { status, data, error }
    }

    /// Create a successful OperationResult with data
    ///
    /// # Arguments
    /// * `data` - The successful result data
    pub fn success(data: T) -> Self {
        Self {
            status: Status::SUCCESS,
            data: Some(data),
            error: None,
        }
    }

    /// Create a failed OperationResult with an error message
    ///
    /// # Arguments
    /// * `message` - The error message describing the failure
    pub fn error(message: String) -> Self {
        Self {
            status: Status::ERROR,
            data: None,
            error: Some(message),
        }
    }

    /// Check if the operation was successful
    pub fn is_success(&self) -> bool {
        self.status.is_success()
    }

    /// Check if the operation failed
    pub fn is_error(&self) -> bool {
        self.status.is_error()
    }

    /// Get a reference to the data, if successful
    pub fn get_data(&self) -> Option<&T> {
        self.data.as_ref()
    }

    /// Get the error message as a string slice, if failed
    pub fn get_error(&self) -> Option<&str> {
        self.error.as_deref()
    }
}

/// Result of a YAML parsing operation
///
/// `ParseResult<T>` represents the outcome of a YAML parsing operation, encapsulating
/// either a successful parse with its associated data and metadata, or a failure with
/// detailed error information.
///
/// # Type Parameter
///
/// - `T` - The type of the parsed value. This is typically the target type that the YAML
///   content is being parsed into (e.g., a configuration struct, `serde_yaml::Value`,
///   or any deserializable type).
///
/// # Fields
///
/// ## Core Fields
///
/// - `value: Option<T>` - The successfully parsed value. Present when parsing succeeds.
/// - `error: Option<ParseError>` - Detailed error information. Present when parsing fails.
/// - `metadata: ParseMetadata` - Metadata about the parsing operation (lines processed,
///   bytes processed, processing time, source path). Always present.
/// - `warnings: Vec<ParseWarning>` - Collection of non-fatal warnings that occurred during
///   parsing. Parsing can succeed with warnings present.
///
/// # Design Philosophy
///
/// `ParseResult<T>` follows a structured approach to representing parsing outcomes:
///
/// 1. **Explicit success/failure states** - Success requires both `value` present and
///    `error` absent. Failure requires `error` present and `value` absent.
/// 2. **Rich context** - Both success and failure carry metadata about the operation.
/// 3. **Warnings without failure** - Warnings are non-fatal; parsing can succeed with
///    warnings present (e.g., deprecated field usage, unknown keys in lenient mode).
/// 4. **Composable operations** - Supports `map()` for transforming success values,
///    and `From<Result<T>>` for interoperability with standard `Result` types.
///
/// # Examples
///
/// ## Successful Parse
///
/// ```ignore
/// use armor::parsers::yaml::{ParseResult, ParseMetadata};
///
/// let result = ParseResult::success(42);
/// assert!(result.is_success());
/// assert_eq!(result.value(), Some(&42));
/// assert!(result.warnings().is_empty());
/// ```
///
/// ## Failed Parse
///
/// ```ignore
/// use armor::parsers::yaml::{ParseResult, ParseError};
///
/// let error = ParseError::syntax("invalid YAML");
/// let result = ParseResult::<i32>::failure(error);
/// assert!(result.is_failure());
/// assert!(result.error().is_some());
/// ```
///
/// ## Parse with Warnings
///
/// ```ignore
/// use armor::parsers::yaml::{ParseResult, ParseWarning};
///
/// let mut result = ParseResult::success(42);
/// result.add_warning(ParseWarning::deprecated_field("old_field", "new_field"));
///
/// assert!(result.is_success());
/// assert!(!result.warnings().is_empty());
/// assert_eq!(result.warnings().len(), 1);
/// ```
///
/// ## Mapping Over Success
///
/// ```ignore
/// use armor::parsers::yaml::ParseResult;
///
/// let result: ParseResult<i32> = ParseResult::success(10);
/// let mapped = result.map(|x| x * 2);
///
/// assert_eq!(mapped.value(), Some(&20));
/// ```
#[derive(Debug, Clone)]
pub struct ParseResult<T> {
    /// The parsed value, if successful
    value: Option<T>,
    /// The error, if parsing failed
    error: Option<ParseError>,
    /// Additional metadata about the parse operation
    metadata: ParseMetadata,
    /// Non-fatal warnings that occurred during parsing
    warnings: Vec<ParseWarning>,
}

impl<T> ParseResult<T> {
    /// Create a successful parse result
    ///
    /// # Arguments
    /// * `value` - The successfully parsed value
    ///
    /// # Returns
    /// A `ParseResult` with the value set, no error, empty warnings, and default metadata
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseResult;
    ///
    /// let result = ParseResult::success(42);
    /// assert!(result.is_success());
    /// assert_eq!(result.value(), Some(&42));
    /// ```
    pub fn success(value: T) -> Self {
        Self {
            value: Some(value),
            error: None,
            metadata: ParseMetadata::default(),
            warnings: Vec::new(),
        }
    }

    /// Create a failed parse result
    ///
    /// # Arguments
    /// * `error` - The error that caused parsing to fail
    ///
    /// # Returns
    /// A `ParseResult` with the error set, no value, empty warnings, and default metadata
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::{ParseResult, ParseError};
    ///
    /// let error = ParseError::syntax("invalid YAML");
    /// let result = ParseResult::<i32>::failure(error);
    /// assert!(result.is_failure());
    /// assert!(result.error().is_some());
    /// ```
    pub fn failure(error: ParseError) -> Self {
        Self {
            value: None,
            error: Some(error),
            metadata: ParseMetadata::default(),
            warnings: Vec::new(),
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
    ///
    /// # Returns
    /// A reference to the `ParseMetadata` containing information about lines processed,
    /// bytes processed, processing time, and source path
    pub fn metadata(&self) -> &ParseMetadata {
        &self.metadata
    }

    /// Get the warnings for this parse result
    ///
    /// # Returns
    /// A slice of `ParseWarning` items representing non-fatal issues that occurred during parsing
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::{ParseResult, ParseWarning};
    ///
    /// let mut result = ParseResult::success(42);
    /// result.add_warning(ParseWarning::deprecated_field("old", "new"));
    ///
    /// assert_eq!(result.warnings().len(), 1);
    /// ```
    pub fn warnings(&self) -> &[ParseWarning] {
        &self.warnings
    }

    /// Check if this parse result has any warnings
    ///
    /// # Returns
    /// `true` if there are warnings present, `false` otherwise
    pub fn has_warnings(&self) -> bool {
        !self.warnings.is_empty()
    }

    /// Add a warning to this parse result
    ///
    /// # Arguments
    /// * `warning` - The warning to add
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::{ParseResult, ParseWarning};
    ///
    /// let mut result = ParseResult::success(42);
    /// result.add_warning(ParseWarning::unknown_key("deprecated_field"));
    /// assert!(result.has_warnings());
    /// ```
    pub fn add_warning(&mut self, warning: ParseWarning) {
        self.warnings.push(warning);
    }

    /// Add warnings to this parse result
    ///
    /// # Arguments
    /// * `warnings` - An iterator of warnings to add
    pub fn add_warnings<I>(&mut self, warnings: I)
    where
        I: IntoIterator<Item = ParseWarning>,
    {
        self.warnings.extend(warnings);
    }

    /// Set the metadata for this parse result
    ///
    /// # Arguments
    /// * `metadata` - The metadata to set
    pub fn with_metadata(mut self, metadata: ParseMetadata) -> Self {
        self.metadata = metadata;
        self
    }

    /// Unwrap the value, consuming the result
    ///
    /// # Panics
    /// Panics if the parse failed (i.e., if `error` is present)
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseResult;
    ///
    /// let result = ParseResult::success(42);
    /// assert_eq!(result.unwrap(), 42);
    /// ```
    pub fn unwrap(self) -> T {
        self.value.expect(
            "called unwrap() on a failed ParseResult"
        )
    }

    /// Unwrap the value or return a default
    ///
    /// # Arguments
    /// * `default` - The default value to return if parsing failed
    ///
    /// # Returns
    /// The parsed value if successful, or the provided default
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::{ParseResult, ParseError};
    ///
    /// let success: ParseResult<i32> = ParseResult::success(42);
    /// assert_eq!(success.unwrap_or(0), 42);
    ///
    /// let failure = ParseResult::<i32>::failure(ParseError::syntax("error"));
    /// assert_eq!(failure.unwrap_or(0), 0);
    /// ```
    pub fn unwrap_or(self, default: T) -> T {
        self.value.unwrap_or(default)
    }

    /// Map the success value to a new type
    ///
    /// # Type Parameters
    /// * `U` - The target type after mapping
    /// * `F` - Function type that transforms `T` into `U`
    ///
    /// # Arguments
    /// * `f` - A function that transforms the success value
    ///
    /// # Returns
    /// A new `ParseResult<U>` with the transformed value, preserving error, metadata, and warnings
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::ParseResult;
    ///
    /// let result: ParseResult<i32> = ParseResult::success(10);
    /// let doubled = result.map(|x| x * 2);
    ///
    /// assert_eq!(doubled.value(), Some(&20));
    /// ```
    pub fn map<U, F>(self, f: F) -> ParseResult<U>
    where
        F: FnOnce(T) -> U,
    {
        match self.value {
            Some(v) => ParseResult {
                value: Some(f(v)),
                error: None,
                metadata: self.metadata,
                warnings: self.warnings,
            },
            None => ParseResult {
                value: None,
                error: self.error,
                metadata: self.metadata,
                warnings: self.warnings,
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
///
/// Enhanced with error codes from bf-68hqo for comprehensive error categorization.
#[derive(Debug, Clone)]
pub struct ValidationError {
    /// Path to the invalid element (e.g., "server.port")
    pub path: String,
    /// Error message
    pub message: String,
    /// Machine-readable error code from bf-68hqo hierarchy
    pub code: ErrorCode,
    /// Line number where the error occurred (1-indexed)
    pub line: Option<usize>,
    /// Column number where the error occurred (1-indexed)
    pub column: Option<usize>,
    /// Indentation error type (if this is an indentation error)
    pub indentation_error_type: Option<crate::parsers::yaml::syntax_detector::IndentationErrorType>,
    /// Delimiter error type (if this is a delimiter error)
    pub delimiter_error_type: Option<crate::parsers::yaml::syntax_detector::DelimiterErrorType>,
}

impl ValidationError {
    /// Create a new validation error
    pub fn new(path: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            path: path.into(),
            message: message.into(),
            code: ErrorCode::VALIDATION_INVALID_VALUE, // Default error code
            line: None,
            column: None,
            indentation_error_type: None,
            delimiter_error_type: None,
        }
    }

    /// Set the error code for this validation error
    pub fn with_code(mut self, code: ErrorCode) -> Self {
        self.code = code;
        self
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

    /// Set the indentation error type for this error
    pub fn with_indentation_error_type(mut self, error_type: crate::parsers::yaml::syntax_detector::IndentationErrorType) -> Self {
        self.indentation_error_type = Some(error_type);
        self
    }

    /// Set the delimiter error type for this error
    pub fn with_delimiter_error_type(mut self, error_type: crate::parsers::yaml::syntax_detector::DelimiterErrorType) -> Self {
        self.delimiter_error_type = Some(error_type);
        self
    }

    /// Get the error type category for this error
    pub fn error_type(&self) -> ErrorType {
        self.code.error_type()
    }
}

impl fmt::Display for ValidationError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match (&self.line, &self.column) {
            (Some(line), Some(col)) => {
                write!(f, "{}:{}: validation error at '{}': {}", line, col, self.path, self.message)
            }
            (Some(line), None) => {
                write!(f, "{}: validation error at '{}': {}", line, self.path, self.message)
            }
            (None, Some(col)) => {
                write!(f, "col {}: validation error at '{}': {}", col, self.path, self.message)
            }
            (None, None) => {
                write!(f, "validation error at '{}': {}", self.path, self.message)
            }
        }
    }
}

impl std::error::Error for ValidationError {}

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

impl ValidationWarning {
    /// Create a new validation warning
    pub fn new(path: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            path: path.into(),
            message: message.into(),
            line: None,
        }
    }

    /// Set the line number for this warning
    pub fn with_line(mut self, line: usize) -> Self {
        self.line = Some(line);
        self
    }
}

impl fmt::Display for ValidationWarning {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match &self.line {
            Some(line) => write!(f, "{}: warning at '{}': {}", line, self.path, self.message),
            None => write!(f, "warning at '{}': {}", self.path, self.message),
        }
    }
}

/// A non-fatal warning that occurred during parsing
///
/// `ParseWarning` represents issues that don't prevent parsing from completing
/// but may indicate deprecated usage, potential problems, or other concerns.
///
/// # Warning Types
///
/// - `DeprecatedField` - A field that has been deprecated and should be migrated
/// - `UnknownKey` - An unknown key encountered (only in lenient mode)
/// - `DuplicateKey` - A duplicate key that was handled (lenient mode)
#[derive(Debug, Clone)]
pub struct ParseWarning {
    /// The kind of warning
    pub kind: ParseWarningKind,
    /// Line number where the warning occurred (1-indexed)
    pub line: Option<usize>,
}

impl ParseWarning {
    /// Create a new parse warning
    ///
    /// # Arguments
    /// * `kind` - The type of warning
    pub fn new(kind: ParseWarningKind) -> Self {
        Self { kind, line: None }
    }

    /// Set the line number for this warning
    ///
    /// # Arguments
    /// * `line` - The line number (1-indexed)
    pub fn with_line(mut self, line: usize) -> Self {
        self.line = Some(line);
        self
    }

    /// Create a deprecated field warning
    ///
    /// # Arguments
    /// * `old_field` - The deprecated field name
    /// * `new_field` - The recommended replacement field
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::{ParseWarning, ParseWarningKind};
    ///
    /// let warning = ParseWarning::deprecated_field("old_api", "new_api");
    /// assert!(matches!(warning.kind, ParseWarningKind::DeprecatedField { .. }));
    /// ```
    pub fn deprecated_field(old_field: impl Into<String>, new_field: impl Into<String>) -> Self {
        Self::new(ParseWarningKind::DeprecatedField {
            old_field: old_field.into(),
            new_field: new_field.into(),
        })
    }

    /// Create an unknown key warning
    ///
    /// # Arguments
    /// * `key` - The unknown key name
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::{ParseWarning, ParseWarningKind};
    ///
    /// let warning = ParseWarning::unknown_key("unknown_setting");
    /// assert!(matches!(warning.kind, ParseWarningKind::UnknownKey(_)));
    /// ```
    pub fn unknown_key(key: impl Into<String>) -> Self {
        Self::new(ParseWarningKind::UnknownKey(key.into()))
    }
}

impl std::fmt::Display for ParseWarning {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let location = self.line.map_or_else(|| "<unknown>".to_string(), |l| l.to_string());
        write!(f, "{}: {}", location, self.kind)
    }
}

/// The kind of parse warning
#[derive(Debug, Clone)]
pub enum ParseWarningKind {
    /// A deprecated field was used
    ///
    /// Indicates that a field has been deprecated and should be replaced
    DeprecatedField {
        /// The deprecated field name
        old_field: String,
        /// The recommended replacement field
        new_field: String,
    },

    /// An unknown key was encountered (in lenient mode)
    ///
    /// Indicates a key that was not recognized but was not rejected
    /// due to lenient parsing mode
    UnknownKey(String),

    /// A duplicate key was encountered (in lenient mode)
    ///
    /// Indicates a duplicate key that was handled by taking one of the values
    DuplicateKey(String),
}

impl std::fmt::Display for ParseWarningKind {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::DeprecatedField { old_field, new_field } => {
                write!(
                    f,
                    "warning: field '{}' is deprecated, use '{}' instead",
                    old_field, new_field
                )
            }
            Self::UnknownKey(key) => {
                write!(f, "warning: unknown key '{}'", key)
            }
            Self::DuplicateKey(key) => {
                write!(f, "warning: duplicate key '{}'", key)
            }
        }
    }
}
