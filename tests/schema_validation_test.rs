//! Schema Validation Unit Tests
//!
//! Comprehensive tests for the Schema trait and Validate() method.
//! These tests cover success paths, error paths, and various validation scenarios.
//!
//! Bead: bf-69knl
//! Acceptance Criteria:
//! - Test module created for Schema interface
//! - Unit tests for Validate() method
//! - Mock schema implementations for testing
//! - Test cases cover success and error paths
//! - All tests pass with `cargo test`

use armor::schema::Schema;
use armor::parsers::yaml::ParseError;

// ============================================================================
// Mock Schema Implementations
// ============================================================================

/// Mock schema that validates positive integers
struct PositiveIntegerSchema;

impl Schema<i32> for PositiveIntegerSchema {
    fn validate(&self, value: &i32) -> Result<(), ParseError> {
        if *value <= 0 {
            return Err(ParseError::validation("must be a positive integer")
                .with_path("value"));
        }
        Ok(())
    }
}

/// Mock schema that validates a range of integers
struct RangeSchema {
    min: i32,
    max: i32,
}

impl Schema<i32> for RangeSchema {
    fn validate(&self, value: &i32) -> Result<(), ParseError> {
        if *value < self.min || *value > self.max {
            return Err(ParseError::validation(
                &format!("must be between {} and {}", self.min, self.max)
            ).with_path("value"));
        }
        Ok(())
    }
}

/// Mock schema that validates non-empty strings
struct NonEmptyStringSchema;

impl Schema<str> for NonEmptyStringSchema {
    fn validate(&self, value: &str) -> Result<(), ParseError> {
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

/// Mock schema that validates port numbers (1-65535)
struct PortSchema;

impl Schema<u16> for PortSchema {
    fn validate(&self, value: &u16) -> Result<(), ParseError> {
        if *value == 0 {
            return Err(ParseError::validation("port cannot be 0")
                .with_path("port"));
        }
        Ok(())
    }
}

/// Mock schema that validates server configuration
struct ServerConfig {
    host: String,
    port: u16,
}

struct ServerConfigSchema;

impl Schema<ServerConfig> for ServerConfigSchema {
    fn validate(&self, config: &ServerConfig) -> Result<(), ParseError> {
        if config.host.is_empty() {
            return Err(ParseError::validation("host cannot be empty")
                .with_path("host"));
        }
        if config.port == 0 {
            return Err(ParseError::validation("port cannot be 0")
                .with_path("port"));
        }
        Ok(())
    }
}

/// Mock schema for username validation
struct UsernameSchema;

impl Schema<String> for UsernameSchema {
    fn validate(&self, username: &String) -> Result<(), ParseError> {
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

/// Mock schema for age validation
struct AgeSchema;

impl Schema<u8> for AgeSchema {
    fn validate(&self, age: &u8) -> Result<(), ParseError> {
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

/// Mock composite user struct
struct User {
    username: String,
    age: u8,
}

/// Mock composite user schema
struct UserSchema;

impl Schema<User> for UserSchema {
    fn validate(&self, user: &User) -> Result<(), ParseError> {
        UsernameSchema.validate(&user.username)
            .map_err(|e| e.with_path("username"))?;
        AgeSchema.validate(&user.age)
            .map_err(|e| e.with_path("age"))?;
        Ok(())
    }
}

/// Mock schema for Option validation
struct RequiredValueSchema;

impl Schema<Option<i32>> for RequiredValueSchema {
    fn validate(&self, value: &Option<i32>) -> Result<(), ParseError> {
        match value {
            None => Err(ParseError::validation("value is required")
                .with_path("value")),
            Some(v) if *v <= 0 => Err(ParseError::validation("value must be positive")
                .with_path("value")),
            Some(_) => Ok(()),
        }
    }
}

// ============================================================================
// Success Path Tests
// ============================================================================

#[test]
fn test_validate_positive_integer_success() {
    let schema = PositiveIntegerSchema;

    // Test various valid positive integers
    assert!(schema.validate(&1).is_ok());
    assert!(schema.validate(&42).is_ok());
    assert!(schema.validate(&100).is_ok());
    assert!(schema.validate(&1000).is_ok());
    assert!(schema.validate(&i32::MAX).is_ok());
}

#[test]
fn test_validate_range_success() {
    let schema = RangeSchema { min: 10, max: 20 };

    // Test boundary values and interior values
    assert!(schema.validate(&10).is_ok());
    assert!(schema.validate(&15).is_ok());
    assert!(schema.validate(&20).is_ok());
}

#[test]
fn test_validate_non_empty_string_success() {
    let schema = NonEmptyStringSchema;

    // Test various valid strings
    assert!(schema.validate("hello").is_ok());
    assert!(schema.validate("test string").is_ok());
    assert!(schema.validate("  trimmed  ").is_ok());
    assert!(schema.validate("a").is_ok());
    assert!(schema.validate("Multiple words with spaces").is_ok());
}

#[test]
fn test_validate_port_success() {
    let schema = PortSchema;

    // Test valid port numbers
    assert!(schema.validate(&1).is_ok());
    assert!(schema.validate(&80).is_ok());
    assert!(schema.validate(&443).is_ok());
    assert!(schema.validate(&8080).is_ok());
    assert!(schema.validate(&65535).is_ok());
}

#[test]
fn test_validate_server_config_success() {
    let schema = ServerConfigSchema;

    // Test valid configurations
    let config1 = ServerConfig {
        host: "localhost".to_string(),
        port: 8080,
    };
    assert!(schema.validate(&config1).is_ok());

    let config2 = ServerConfig {
        host: "example.com".to_string(),
        port: 443,
    };
    assert!(schema.validate(&config2).is_ok());

    let config3 = ServerConfig {
        host: "192.168.1.1".to_string(),
        port: 22,
    };
    assert!(schema.validate(&config3).is_ok());
}

#[test]
fn test_validate_username_success() {
    let schema = UsernameSchema;

    // Test valid usernames (3-20 characters)
    assert!(schema.validate(&"abc".to_string()).is_ok());
    assert!(schema.validate(&"john_doe".to_string()).is_ok());
    assert!(schema.validate(&"user123".to_string()).is_ok());
    assert!(schema.validate(&"a".repeat(20)).is_ok());
}

#[test]
fn test_validate_age_success() {
    let schema = AgeSchema;

    // Test valid ages (18-120)
    assert!(schema.validate(&18).is_ok());
    assert!(schema.validate(&25).is_ok());
    assert!(schema.validate(&50).is_ok());
    assert!(schema.validate(&120).is_ok());
}

#[test]
fn test_validate_composite_user_success() {
    let schema = UserSchema;

    // Test valid user with both fields correct
    let valid_user = User {
        username: "john_doe".to_string(),
        age: 25,
    };
    assert!(schema.validate(&valid_user).is_ok());

    let valid_user2 = User {
        username: "alice".to_string(),
        age: 30,
    };
    assert!(schema.validate(&valid_user2).is_ok());
}

#[test]
fn test_validate_option_success() {
    let schema = RequiredValueSchema;

    // Test Some with positive values
    assert!(schema.validate(&Some(42)).is_ok());
    assert!(schema.validate(&Some(1)).is_ok());
    assert!(schema.validate(&Some(1000)).is_ok());
}

// ============================================================================
// Error Path Tests
// ============================================================================

#[test]
fn test_validate_positive_integer_error() {
    let schema = PositiveIntegerSchema;

    // Test zero
    let result = schema.validate(&0);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert_eq!(error.path, Some("value".to_string()));
    assert!(format!("{}", error).contains("positive"));

    // Test negative numbers
    let result = schema.validate(&-1);
    assert!(result.is_err());
    assert!(result.unwrap_err().is_validation());

    let result = schema.validate(&-100);
    assert!(result.is_err());
    assert!(result.unwrap_err().is_validation());
}

#[test]
fn test_validate_range_error_below_minimum() {
    let schema = RangeSchema { min: 10, max: 20 };

    let result = schema.validate(&9);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    let error_str = format!("{}", error);
    assert!(error_str.contains("between") && error_str.contains("10") && error_str.contains("20"));
    assert_eq!(error.path, Some("value".to_string()));
}

#[test]
fn test_validate_range_error_above_maximum() {
    let schema = RangeSchema { min: 10, max: 20 };

    let result = schema.validate(&21);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    let error_str = format!("{}", error);
    assert!(error_str.contains("between") && error_str.contains("10") && error_str.contains("20"));
    assert_eq!(error.path, Some("value".to_string()));
}

#[test]
fn test_validate_non_empty_string_error_empty() {
    let schema = NonEmptyStringSchema;

    let result = schema.validate("");
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(format!("{}", error).contains("empty"));
    assert_eq!(error.path, Some("value".to_string()));
}

#[test]
fn test_validate_non_empty_string_error_whitespace_only() {
    let schema = NonEmptyStringSchema;

    let result = schema.validate("   ");
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(format!("{}", error).contains("whitespace"));
    assert_eq!(error.path, Some("value".to_string()));
}

#[test]
fn test_validate_port_error_zero() {
    let schema = PortSchema;

    let result = schema.validate(&0);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(format!("{}", error).contains("cannot be 0"));
    assert_eq!(error.path, Some("port".to_string()));
}

#[test]
fn test_validate_server_config_error_empty_host() {
    let schema = ServerConfigSchema;

    let invalid_config = ServerConfig {
        host: "".to_string(),
        port: 8080,
    };

    let result = schema.validate(&invalid_config);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(format!("{}", error).contains("empty"));
    assert_eq!(error.path, Some("host".to_string()));
}

#[test]
fn test_validate_server_config_error_invalid_port() {
    let schema = ServerConfigSchema;

    let invalid_config = ServerConfig {
        host: "localhost".to_string(),
        port: 0,
    };

    let result = schema.validate(&invalid_config);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(format!("{}", error).contains("cannot be 0"));
    assert_eq!(error.path, Some("port".to_string()));
}

#[test]
fn test_validate_username_error_too_short() {
    let schema = UsernameSchema;

    let result = schema.validate(&"ab".to_string());
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(format!("{}", error).contains("at least 3 characters"));
    assert_eq!(error.path, Some("username".to_string()));

    // Test empty string
    let result = schema.validate(&"".to_string());
    assert!(result.is_err());
}

#[test]
fn test_validate_username_error_too_long() {
    let schema = UsernameSchema;

    let result = schema.validate(&"a".repeat(21));
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(format!("{}", error).contains("at most 20 characters"));
    assert_eq!(error.path, Some("username".to_string()));
}

#[test]
fn test_validate_age_error_too_young() {
    let schema = AgeSchema;

    let result = schema.validate(&17);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(format!("{}", error).contains("at least 18"));
    assert_eq!(error.path, Some("age".to_string()));

    // Test boundary
    let result = schema.validate(&0);
    assert!(result.is_err());
}

#[test]
fn test_validate_age_error_too_old() {
    let schema = AgeSchema;

    let result = schema.validate(&121);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(format!("{}", error).contains("at most 120"));
    assert_eq!(error.path, Some("age".to_string()));
}

#[test]
fn test_validate_composite_user_error_invalid_username() {
    let schema = UserSchema;

    let invalid_user = User {
        username: "jo".to_string(),  // Too short
        age: 25,
    };

    let result = schema.validate(&invalid_user);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert_eq!(error.path, Some("username".to_string()));
}

#[test]
fn test_validate_composite_user_error_invalid_age() {
    let schema = UserSchema;

    let invalid_user = User {
        username: "john_doe".to_string(),
        age: 16,  // Too young
    };

    let result = schema.validate(&invalid_user);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert_eq!(error.path, Some("age".to_string()));
}

#[test]
fn test_validate_composite_user_error_both_invalid() {
    let schema = UserSchema;

    let invalid_user = User {
        username: "ab".to_string(),  // Too short
        age: 16,  // Too young
    };

    let result = schema.validate(&invalid_user);
    assert!(result.is_err());
    // Should fail on username validation first (short-circuit)
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert_eq!(error.path, Some("username".to_string()));
}

#[test]
fn test_validate_option_error_none() {
    let schema = RequiredValueSchema;

    let result = schema.validate(&None);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(format!("{}", error).contains("required"));
    assert_eq!(error.path, Some("value".to_string()));
}

#[test]
fn test_validate_option_error_non_positive() {
    let schema = RequiredValueSchema;

    let result = schema.validate(&Some(0));
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(format!("{}", error).contains("positive"));
    assert_eq!(error.path, Some("value".to_string()));

    let result = schema.validate(&Some(-5));
    assert!(result.is_err());
    assert!(result.unwrap_err().is_validation());
}

// ============================================================================
// Edge Case Tests
// ============================================================================

#[test]
fn test_validate_boundary_values_range() {
    let schema = RangeSchema { min: 0, max: 100 };

    // Test exact boundaries
    assert!(schema.validate(&0).is_ok());
    assert!(schema.validate(&100).is_ok());

    // Test just outside boundaries
    assert!(schema.validate(&-1).is_err());
    assert!(schema.validate(&101).is_err());
}

#[test]
fn test_validate_large_values() {
    let schema = PositiveIntegerSchema;

    // Test very large positive integer
    assert!(schema.validate(&i32::MAX).is_ok());

    // Test very large negative integer
    assert!(schema.validate(&i32::MIN).is_err());
}

#[test]
fn test_validate_string_edge_cases() {
    let schema = NonEmptyStringSchema;

    // Test single character
    assert!(schema.validate("a").is_ok());

    // Test string with only newlines
    let result = schema.validate("\n\n");
    assert!(result.is_err());  // Contains only whitespace

    // Test string with tabs
    let result = schema.validate("\t\t");
    assert!(result.is_err());  // Contains only whitespace
}

#[test]
fn test_validate_multiple_errors_in_composite() {
    // This test verifies that validation short-circuits on first error
    struct StrictUserSchema;

    impl Schema<User> for StrictUserSchema {
        fn validate(&self, user: &User) -> Result<(), ParseError> {
            // Validate both fields but return on first error
            if user.username.len() < 3 {
                return Err(ParseError::validation("username too short")
                    .with_path("username"));
            }
            if user.age < 18 {
                return Err(ParseError::validation("age too young")
                    .with_path("age"));
            }
            Ok(())
        }
    }

    let schema = StrictUserSchema;

    let invalid_user = User {
        username: "ab".to_string(),  // Will fail first
        age: 16,  // This won't be checked
    };

    let result = schema.validate(&invalid_user);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert_eq!(error.path, Some("username".to_string()));
}

#[test]
fn test_validate_error_message_formatting() {
    let schema = RangeSchema { min: 1, max: 10 };

    let result = schema.validate(&0);
    assert!(result.is_err());
    let error = result.unwrap_err();

    // Verify error message contains expected information
    assert!(error.message.contains("between"));
    assert!(error.message.contains("1"));
    assert!(error.message.contains("10"));
    assert_eq!(error.path, Some("value".to_string()));
}

#[test]
fn test_validate_error_type_classification() {
    let schema = PositiveIntegerSchema;

    let result = schema.validate(&-5);
    assert!(result.is_err());
    let error = result.unwrap_err();

    // Verify error is classified as validation error
    assert!(error.is_validation());
    assert!(!error.is_syntax());
    assert!(!error.is_io());
}
