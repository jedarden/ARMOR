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
//! use armor::schema::{Schema, ValidationError};
//!
//! struct PortSchema;
//!
//! impl Schema<u16> for PortSchema {
//!     fn validate(&self, value: &u16) -> Result<(), ValidationError> {
//!         if *value == 0 {
//!             return Err(ValidationError::new("port", "port cannot be 0"));
//!         }
//!         if *value > 65535 {
//!             return Err(ValidationError::new("port", "port must be between 1 and 65535"));
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
//!
//! // Validate a configuration struct
//! struct ConfigSchema;
//!
//! impl Schema<Config> for ConfigSchema {
//!     fn validate(&self, config: &Config) -> Result<(), ValidationError> {
//!         // Validate individual fields
//!         if config.port < 1 || config.port > 65535 {
//!             return Err(ValidationError::new("port", "port out of range"));
//!         }
//!         if config.host.is_empty() {
//!             return Err(ValidationError::new("host", "host cannot be empty"));
//!         }
//!         Ok(())
//!     }
//! }
//! ```

use std::fmt;

/// Validation error type for Schema implementations
///
/// This error type represents validation failures that occur when validating
/// data against a schema. It provides structured error information including
/// the field path and a descriptive error message.
///
/// # Fields
///
/// - `path` - The path to the invalid field (e.g., "server.port", "user.email")
/// - `message` - Human-readable error message describing what went wrong
#[derive(Debug, Clone, PartialEq)]
pub struct ValidationError {
    /// Path to the invalid element (e.g., "server.port")
    pub path: String,
    /// Error message describing the validation failure
    pub message: String,
}

impl ValidationError {
    /// Create a new validation error
    ///
    /// # Arguments
    ///
    /// * `path` - The path to the invalid field (e.g., "server.port")
    /// * `message` - Human-readable error message
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::schema::ValidationError;
    ///
    /// let error = ValidationError::new("port", "port must be between 1 and 65535");
    /// assert_eq!(error.path, "port");
    /// assert_eq!(error.message, "port must be between 1 and 65535");
    /// ```
    pub fn new(path: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            path: path.into(),
            message: message.into(),
        }
    }

    /// Get the field path for this error
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::schema::ValidationError;
    ///
    /// let error = ValidationError::new("server.port", "invalid value");
    /// assert_eq!(error.path(), "server.port");
    /// ```
    pub fn path(&self) -> &str {
        &self.path
    }

    /// Get the error message
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::schema::ValidationError;
    ///
    /// let error = ValidationError::new("port", "invalid range");
    /// assert_eq!(error.message(), "invalid range");
    /// ```
    pub fn message(&self) -> &str {
        &self.message
    }
}

impl fmt::Display for ValidationError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "validation error at '{}': {}", self.path, self.message)
    }
}

impl std::error::Error for ValidationError {}

/// Result type for validation operations
///
/// This is a type alias for `Result<(), ValidationError>`, used throughout
/// the Schema trait and its implementations.
pub type ValidationResult = Result<(), ValidationError>;

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
///     fn validate(&self, config: &ServerConfig) -> Result<(), ValidationError> {
///         if config.host.is_empty() {
///             return Err(ValidationError::new("host", "cannot be empty"));
///         }
///         if config.port == 0 {
///             return Err(ValidationError::new("port", "cannot be 0"));
///         }
///         Ok(())
///     }
/// }
/// ```
///
/// ## Composing Validators
///
/// ```ignore
/// use armor::schema::{Schema, ValidationError};
///
/// struct Config {
///     name: String,
///     port: u16,
/// }
///
/// // Individual field validators
/// struct NameSchema;
/// impl Schema<String> for NameSchema {
///     fn validate(&self, name: &String) -> Result<(), ValidationError> {
///         if name.len() < 3 {
///             return Err(ValidationError::new("name", "must be at least 3 characters"));
///         }
///         Ok(())
///     }
/// }
///
/// struct PortSchema;
/// impl Schema<u16> for PortSchema {
///     fn validate(&self, port: &u16) -> Result<(), ValidationError> {
///         if *port == 0 {
///             return Err(ValidationError::new("port", "cannot be 0"));
///         }
///         Ok(())
///     }
/// }
///
/// // Composite validator
/// struct ConfigSchema;
/// impl Schema<Config> for ConfigSchema {
///     fn validate(&self, config: &Config) -> Result<(), ValidationError> {
///         // Delegate to field-specific validators
///         NameSchema.validate(&config.name).map_err(|e| ValidationError::new("name", e.message))?;
///         PortSchema.validate(&config.port).map_err(|e| ValidationError::new("port", e.message))?;
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
    /// * `Err(ValidationError)` - Validation failed with error details
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
    /// use armor::schema::{Schema, ValidationError};
    ///
    /// struct RangeSchema {
    ///     min: i32,
    ///     max: i32,
    /// }
    ///
    /// impl Schema<i32> for RangeSchema {
    ///     fn validate(&self, value: &i32) -> Result<(), ValidationError> {
    ///         if *value < self.min || *value > self.max {
    ///             return Err(ValidationError::new(
    ///                 "value",
    ///                 &format!("must be between {} and {}", self.min, self.max)
    ///             ));
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

    #[test]
    fn test_validation_error_creation() {
        let error = ValidationError::new("port", "invalid range");
        assert_eq!(error.path, "port");
        assert_eq!(error.message, "invalid range");
        assert_eq!(error.path(), "port");
        assert_eq!(error.message(), "invalid range");
    }

    #[test]
    fn test_validation_error_display() {
        let error = ValidationError::new("server.port", "must be between 1 and 65535");
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
                    return Err(ValidationError::new("value", "must be positive"));
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
        assert_eq!(result.unwrap_err().path, "value");

        let result = schema.validate(&-5);
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().message, "must be positive");
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
                    return Err(ValidationError::new(
                        "value",
                        &format!("must be between {} and {}", self.min, self.max)
                    ));
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
    fn test_validation_error_equality() {
        let error1 = ValidationError::new("port", "invalid");
        let error2 = ValidationError::new("port", "invalid");
        let error3 = ValidationError::new("host", "invalid");

        assert_eq!(error1, error2);
        assert_ne!(error1, error3);
    }
}
