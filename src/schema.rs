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
/// * `T: ?Sized` - The type being validated (e.g., a struct, primitive type, or collection).
///   The `?Sized` bound allows the trait to be implemented for both sized and unsized types,
///   providing maximum flexibility for different validation scenarios.
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
/// use armor::schema::Schema;
/// use armor::parsers::yaml::ParseError;
///
/// struct PositiveIntegerSchema;
///
/// impl Schema<i32> for PositiveIntegerSchema {
///     fn validate(&self, value: &i32) -> Result<(), ParseError> {
///         if *value <= 0 {
///             return Err(ParseError::validation("must be a positive integer")
///                 .with_path("value"));
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
/// use armor::schema::Schema;
/// use armor::parsers::yaml::ParseError;
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
pub trait Schema<T: ?Sized> {
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

    // Generic value validation tests

    #[test]
    fn test_generic_string_validation() {
        struct NonEmptyStringSchema;

        impl Schema<str> for NonEmptyStringSchema {
            fn validate(&self, value: &str) -> ValidationResult {
                if value.is_empty() {
                    return Err(ParseError::validation("string cannot be empty")
                        .with_path("value"));
                }
                if value.trim().is_empty() {
                    return Err(ParseError::validation("string cannot be only whitespace")
                        .with_path("value"));
                }
                Ok(())
            }
        }

        let schema = NonEmptyStringSchema;

        // Valid strings
        assert!(schema.validate("hello").is_ok());
        assert!(schema.validate("test string").is_ok());
        assert!(schema.validate("  trimmed  ").is_ok());

        // Invalid strings
        let result = schema.validate("");
        assert!(result.is_err());
        assert!(result.unwrap_err().is_validation());

        let result = schema.validate("   ");
        assert!(result.is_err());
        assert!(result.unwrap_err().is_validation());
    }

    #[test]
    fn test_generic_vec_validation() {
        struct NonEmptyVecSchema;

        impl Schema<Vec<String>> for NonEmptyVecSchema {
            fn validate(&self, value: &Vec<String>) -> ValidationResult {
                if value.is_empty() {
                    return Err(ParseError::validation("vector cannot be empty")
                        .with_path("items"));
                }
                Ok(())
            }
        }

        let schema = NonEmptyVecSchema;

        // Valid vectors
        assert!(schema.validate(&vec!["item1".to_string()]).is_ok());
        assert!(schema.validate(&vec!["a".to_string(), "b".to_string()]).is_ok());

        // Invalid empty vector
        let result = schema.validate(&vec![]);
        assert!(result.is_err());
        assert!(result.unwrap_err().is_validation());
    }

    #[test]
    fn test_generic_custom_struct_validation() {
        struct ServerConfig {
            host: String,
            port: u16,
        }

        struct ServerConfigSchema;

        impl Schema<ServerConfig> for ServerConfigSchema {
            fn validate(&self, config: &ServerConfig) -> ValidationResult {
                if config.host.is_empty() {
                    return Err(ParseError::validation("host cannot be empty")
                        .with_path("host"));
                }
                if config.port == 0 {
                    return Err(ParseError::validation("port cannot be 0")
                        .with_path("port"));
                }
                if config.port > 65535 {
                    return Err(ParseError::validation("port must be <= 65535")
                        .with_path("port"));
                }
                Ok(())
            }
        }

        let schema = ServerConfigSchema;

        // Valid configurations
        let valid_config = ServerConfig {
            host: "localhost".to_string(),
            port: 8080,
        };
        assert!(schema.validate(&valid_config).is_ok());

        let valid_config2 = ServerConfig {
            host: "example.com".to_string(),
            port: 443,
        };
        assert!(schema.validate(&valid_config2).is_ok());

        // Invalid: empty host
        let invalid_host = ServerConfig {
            host: "".to_string(),
            port: 8080,
        };
        let result = schema.validate(&invalid_host);
        assert!(result.is_err());
        assert!(result.unwrap_err().is_validation());

        // Invalid: port is 0
        let invalid_port = ServerConfig {
            host: "localhost".to_string(),
            port: 0,
        };
        let result = schema.validate(&invalid_port);
        assert!(result.is_err());
        assert!(result.unwrap_err().is_validation());
    }

    #[test]
    fn test_generic_option_validation() {
        struct PositiveValueSchema;

        impl Schema<Option<i32>> for PositiveValueSchema {
            fn validate(&self, value: &Option<i32>) -> ValidationResult {
                match value {
                    None => Err(ParseError::validation("value is required")
                        .with_path("value")),
                    Some(v) if *v <= 0 => Err(ParseError::validation("value must be positive")
                        .with_path("value")),
                    Some(_) => Ok(()),
                }
            }
        }

        let schema = PositiveValueSchema;

        // Valid: Some positive value
        assert!(schema.validate(&Some(42)).is_ok());
        assert!(schema.validate(&Some(1)).is_ok());

        // Invalid: None
        let result = schema.validate(&None);
        assert!(result.is_err());
        assert!(result.unwrap_err().is_validation());

        // Invalid: Some non-positive value
        let result = schema.validate(&Some(0));
        assert!(result.is_err());
        assert!(result.unwrap_err().is_validation());

        let result = schema.validate(&Some(-5));
        assert!(result.is_err());
        assert!(result.unwrap_err().is_validation());
    }

    #[test]
    fn test_generic_numeric_type_validation() {
        struct RangeSchema<T> {
            min: T,
            max: T,
        }

        // Implement for i32
        impl Schema<i32> for RangeSchema<i32> {
            fn validate(&self, value: &i32) -> ValidationResult {
                if *value < self.min || *value > self.max {
                    return Err(ParseError::validation(
                        &format!("must be between {} and {}", self.min, self.max)
                    ).with_path("value"));
                }
                Ok(())
            }
        }

        // Implement for u64
        impl Schema<u64> for RangeSchema<u64> {
            fn validate(&self, value: &u64) -> ValidationResult {
                if *value < self.min || *value > self.max {
                    return Err(ParseError::validation(
                        &format!("must be between {} and {}", self.min, self.max)
                    ).with_path("value"));
                }
                Ok(())
            }
        }

        // Test i32 implementation
        let i32_schema = RangeSchema { min: 10, max: 100 };
        assert!(i32_schema.validate(&50).is_ok());
        assert!(i32_schema.validate(&10).is_ok());
        assert!(i32_schema.validate(&100).is_ok());
        assert!(i32_schema.validate(&9).is_err());
        assert!(i32_schema.validate(&101).is_err());

        // Test u64 implementation
        let u64_schema = RangeSchema { min: 1000, max: 10000 };
        assert!(u64_schema.validate(&5000).is_ok());
        assert!(u64_schema.validate(&1000).is_ok());
        assert!(u64_schema.validate(&10000).is_ok());
        assert!(u64_schema.validate(&999).is_err());
        assert!(u64_schema.validate(&10001).is_err());
    }

    #[test]
    fn test_generic_composable_validation() {
        // Field-specific validators
        struct UsernameSchema;
        impl Schema<String> for UsernameSchema {
            fn validate(&self, username: &String) -> ValidationResult {
                if username.len() < 3 {
                    return Err(ParseError::validation("username must be at least 3 characters")
                        .with_path("username"));
                }
                if username.len() > 20 {
                    return Err(ParseError::validation("username must be at most 20 characters")
                        .with_path("username"));
                }
                Ok(())
            }
        }

        struct AgeSchema;
        impl Schema<u8> for AgeSchema {
            fn validate(&self, age: &u8) -> ValidationResult {
                if *age < 18 {
                    return Err(ParseError::validation("age must be at least 18")
                        .with_path("age"));
                }
                if *age > 120 {
                    return Err(ParseError::validation("age must be at most 120")
                        .with_path("age"));
                }
                Ok(())
            }
        }

        // Composite validator
        struct User {
            username: String,
            age: u8,
        }

        struct UserSchema;
        impl Schema<User> for UserSchema {
            fn validate(&self, user: &User) -> ValidationResult {
                // Delegate to field-specific validators
                UsernameSchema.validate(&user.username)
                    .map_err(|e| e.with_path("username"))?;
                AgeSchema.validate(&user.age)
                    .map_err(|e| e.with_path("age"))?;
                Ok(())
            }
        }

        let schema = UserSchema;

        // Valid user
        let valid_user = User {
            username: "john_doe".to_string(),
            age: 25,
        };
        assert!(schema.validate(&valid_user).is_ok());

        // Invalid: username too short
        let invalid_username = User {
            username: "jo".to_string(),
            age: 25,
        };
        let result = schema.validate(&invalid_username);
        assert!(result.is_err());
        let error = result.unwrap_err();
        assert!(error.is_validation());
        assert_eq!(error.path, Some("username".to_string()));

        // Invalid: age too young
        let invalid_age = User {
            username: "john_doe".to_string(),
            age: 16,
        };
        let result = schema.validate(&invalid_age);
        assert!(result.is_err());
        let error = result.unwrap_err();
        assert!(error.is_validation());
        assert_eq!(error.path, Some("age".to_string()));
    }
}
