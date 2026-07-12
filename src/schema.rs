//! Schema validation interface
//!
//! This module defines the core [`Schema`] trait for validating structured data.
//! It provides a generic interface for implementing validation logic across different
//! data types and validation strategies.
//!
//! # Overview
//!
//! The [`Schema`] trait defines a contract for validating data structures. Implementors
//! can define domain-specific validation rules and provide detailed error information
//! when validation fails.
//!
//! # Basic Usage
//!
//! ```ignore
//! use armor::schema::Schema;
//! use armor::parsers::yaml::ParseError;
//!
//! struct PortSchema;
//!
//! impl Schema<u16> for PortSchema {
//!     fn validate(&self, value: &u16) -> Result<(), ParseError> {
//!         if *value == 0 {
//!             return Err(ParseError::validation("port cannot be 0")
//!                 .with_path("port"));
//!         }
//!         if *value > 65535 {
//!             return Err(ParseError::validation("port must be between 1 and 65535")
//!                 .with_path("port"));
//!         }
//!         Ok(())
//!     }
//! }
//! ```
//!
//! # Generic Validation
//!
//! The Schema trait is generic over the type being validated, allowing you to:
//!
//! - Validate primitive types (integers, strings, etc.)
//! - Validate complex structures (structs, enums)
//! - Validate collections (vectors, hash maps)
//! - Compose multiple validators together
//!
//! ```ignore
//! use armor::schema::Schema;
//! use armor::parsers::yaml::ParseError;
//!
//! // Validate a configuration struct
//! struct ConfigSchema;
//!
//! impl Schema<Config> for ConfigSchema {
//!     fn validate(&self, config: &Config) -> Result<(), ParseError> {
//!         // Validate individual fields
//!         if config.port < 1 || config.port > 65535 {
//!             return Err(ParseError::validation("port out of range")
//!                 .with_path("port"));
//!         }
//!         if config.host.is_empty() {
//!             return Err(ParseError::validation("host cannot be empty")
//!                 .with_path("host"));
//!         }
//!         Ok(())
//!     }
//! }
//! ```

use std::fmt;
use crate::parsers::yaml::ParseError;

/// Result type for validation operations
///
/// This is a type alias for `Result<(), ParseError>`, used throughout
/// the Schema trait and its implementations. This integrates the Schema
/// validation with the comprehensive error type hierarchy from the YAML parser.
///
/// # Error Types
///
/// The [`ParseError`] type provides rich error information including:
/// - Error categorization (syntax, validation, type mismatch, I/O, etc.)
/// - Location information (file path, line, column)
/// - Context messages and code snippets
/// - Builder pattern for adding contextual information
///
/// # Examples
///
/// ```
/// use armor::schema::{Schema, ValidationResult};
/// use armor::parsers::yaml::ParseError;
///
/// struct PortSchema;
///
/// impl Schema<u16> for PortSchema {
///     fn validate(&self, value: &u16) -> ValidationResult {
///         if *value == 0 {
///             return Err(ParseError::validation("port cannot be 0")
///                 .with_path("port"));
///         }
///         if *value > 65535 {
///             return Err(ParseError::validation("port must be between 1 and 65535")
///                 .with_path("port"));
///         }
///         Ok(())
///     }
/// }
/// ```
pub type ValidationResult = Result<(), ParseError>;

/// Schema validation trait
///
/// This trait defines a generic interface for validating structured data.
/// Implementors can define domain-specific validation rules for their data types.
///
/// # Type Parameters
///
/// * `T` - The type being validated (e.g., a struct, primitive type, or collection)
///
/// # Required Methods
///
/// Implementors must define the [`validate`](Self::validate) method, which accepts
/// a reference to a value of type `T` and returns a [`ValidationResult`].
///
/// # Examples
///
/// ## Validating Primitive Types
///
/// ```
/// use armor::schema::{Schema, ValidationError};
///
/// struct PositiveIntegerSchema;
///
/// impl Schema<i32> for PositiveIntegerSchema {
///     fn validate(&self, value: &i32) -> Result<(), ValidationError> {
///         if *value <= 0 {
///             return Err(ValidationError::new(
///                 "value",
///                 "must be a positive integer"
///             ));
///         }
///         Ok(())
///     }
/// }
///
/// let schema = PositiveIntegerSchema;
/// assert!(schema.validate(&42).is_ok());
/// assert!(schema.validate(&-5).is_err());
/// ```
///
/// ## Validating Structs
///
/// ```ignore
/// use armor::schema::{Schema, ValidationError};
///
/// struct ServerConfig {
///     host: String,
///     port: u16,
/// }
///
/// struct ServerConfigSchema;
///
/// impl Schema<ServerConfig> for ServerConfigSchema {
///     fn validate(&self, config: &ServerConfig) -> Result<(), ParseError> {
///         if config.host.is_empty() {
///             return Err(ParseError::validation("cannot be empty")
///                 .with_path("host"));
///         }
///         if config.port == 0 {
///             return Err(ParseError::validation("cannot be 0")
///                 .with_path("port"));
///         }
///         Ok(())
///     }
/// }
/// ```
///
/// ## Composing Validators
///
/// ```ignore
/// use armor::schema::Schema;
/// use armor::parsers::yaml::ParseError;
///
/// struct Config {
///     name: String,
///     port: u16,
/// }
///
/// // Individual field validators
/// struct NameSchema;
/// impl Schema<String> for NameSchema {
///     fn validate(&self, name: &String) -> Result<(), ParseError> {
///         if name.len() < 3 {
///             return Err(ParseError::validation("must be at least 3 characters")
///                 .with_path("name"));
///         }
///         Ok(())
///     }
/// }
///
/// struct PortSchema;
/// impl Schema<u16> for PortSchema {
///     fn validate(&self, port: &u16) -> Result<(), ParseError> {
///         if *port == 0 {
///             return Err(ParseError::validation("cannot be 0")
///                 .with_path("port"));
///         }
///         Ok(())
///     }
/// }
///
/// // Composite validator
/// struct ConfigSchema;
/// impl Schema<Config> for ConfigSchema {
///     fn validate(&self, config: &Config) -> Result<(), ParseError> {
///         // Delegate to field-specific validators
///         NameSchema.validate(&config.name)
///             .map_err(|e| e.with_path("name"))?;
///         PortSchema.validate(&config.port)
///             .map_err(|e| e.with_path("port"))?;
///         Ok(())
///     }
/// }
/// ```
pub trait Schema<T> {
    /// Validate a value according to the schema rules
    ///
    /// This method validates the provided value and returns `Ok(())` if validation
    /// passes, or an error with details about what failed.
    ///
    /// # Arguments
    ///
    /// * `value` - A reference to the value being validated
    ///
    /// # Returns
    ///
    /// * `Ok(())` - Validation passed
    /// * `Err(ParseError)` - Validation failed with error details
    ///
    /// # Errors
    ///
    /// This method returns an error if:
    /// - Required fields are missing or empty
    /// - Values are outside allowed ranges
    /// - String values don't match required patterns
    /// - Any other schema constraint is violated
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::schema::Schema;
    /// use armor::parsers::yaml::ParseError;
    ///
    /// struct RangeSchema {
    ///     min: i32,
    ///     max: i32,
    /// }
    ///
    /// impl Schema<i32> for RangeSchema {
    ///     fn validate(&self, value: &i32) -> Result<(), ParseError> {
    ///         if *value < self.min || *value > self.max {
    ///             return Err(ParseError::validation(
    ///                 &format!("must be between {} and {}", self.min, self.max)
    ///             ).with_path("value"));
    ///         }
    ///         Ok(())
    ///     }
    /// }
    ///
    /// let schema = RangeSchema { min: 1, max: 100 };
    /// assert!(schema.validate(&50).is_ok());
    /// assert!(schema.validate(&0).is_err());
    /// assert!(schema.validate(&101).is_err());
    /// ```
    fn validate(&self, value: &T) -> ValidationResult;
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::parsers::yaml::ParseError;

    #[test]
    fn test_parse_error_validation_creation() {
        let error = ParseError::validation("invalid range")
            .with_path("port");
        assert!(error.is_validation());
        assert_eq!(error.path, Some("port".to_string()));
    }

    #[test]
    fn test_parse_error_display() {
        let error = ParseError::validation("must be between 1 and 65535")
            .with_path("server.port")
            .with_line(10);
        let display = format!("{}", error);
        assert!(display.contains("server.port"));
        assert!(display.contains("must be between 1 and 65535"));
        assert!(display.contains("validation error"));
    }

    #[test]
    fn test_schema_trait_basic() {
        struct PositiveSchema;

        impl Schema<i32> for PositiveSchema {
            fn validate(&self, value: &i32) -> ValidationResult {
                if *value <= 0 {
                    return Err(ParseError::validation("must be positive")
                        .with_path("value"));
                }
                Ok(())
            }
        }

        let schema = PositiveSchema;

        // Valid values
        assert!(schema.validate(&1).is_ok());
        assert!(schema.validate(&100).is_ok());

        // Invalid values
        let result = schema.validate(&0);
        assert!(result.is_err());
        assert!(result.unwrap_err().is_validation());

        let result = schema.validate(&-5);
        assert!(result.is_err());
        assert!(result.unwrap_err().is_validation());
    }

    #[test]
    fn test_schema_trait_range_validation() {
        struct RangeSchema {
            min: i32,
            max: i32,
        }

        impl Schema<i32> for RangeSchema {
            fn validate(&self, value: &i32) -> ValidationResult {
                if *value < self.min || *value > self.max {
                    return Err(ParseError::validation(
                        &format!("must be between {} and {}", self.min, self.max)
                    ).with_path("value"));
                }
                Ok(())
            }
        }

        let schema = RangeSchema { min: 10, max: 20 };

        // Valid values
        assert!(schema.validate(&10).is_ok());
        assert!(schema.validate(&15).is_ok());
        assert!(schema.validate(&20).is_ok());

        // Invalid values
        assert!(schema.validate(&9).is_err());
        assert!(schema.validate(&21).is_err());
    }

    #[test]
    fn test_parse_error_equality() {
        let error1 = ParseError::validation("invalid")
            .with_path("port")
            .with_line(5);
        let error2 = ParseError::validation("invalid")
            .with_path("port")
            .with_line(5);
        let error3 = ParseError::validation("invalid")
            .with_path("host")
            .with_line(5);

        assert_eq!(error1, error2);
        assert_ne!(error1, error3);
    }

    #[test]
    fn test_parse_error_builder_pattern() {
        let error = ParseError::type_mismatch("port", "integer", "string")
            .with_path("config.yaml")
            .with_line(10)
            .with_column(15)
            .with_context("while parsing service configuration");

        assert!(error.is_type_mismatch());
        assert_eq!(error.path, Some("config.yaml".to_string()));
        assert_eq!(error.line, Some(10));
        assert_eq!(error.column, Some(15));
        assert_eq!(error.context, "while parsing service configuration");
        assert_eq!(error.location_string(), "config.yaml:10:15");
    }

    #[test]
    fn test_parse_error_with_snippet() {
        let error = ParseError::syntax("invalid port value")
            .with_path("config.yaml")
            .with_line(5)
            .with_column(10)
            .with_snippet("service:\n  port: abc");

        let report = error.detailed_report();
        assert!(report.contains("config.yaml:5"));
        assert!(report.contains("syntax error"));
        assert!(report.contains("service:"));
        assert!(report.contains("port: abc"));
    }
}
