//! Test cases for ErrorCode and ErrorType integration from bf-68hqo
//!
//! This test verifies that the error code system provides:
//! - Machine-readable error codes for programmatic handling
//! - Error type categorization
//! - Human-readable error descriptions
//! - Integration with ValidationError

use armor::parsers::yaml::{ErrorCode, ErrorType, ValidationError};

// ============================================================================
// Section 1: ErrorCode categorization
// ============================================================================

#[test]
fn test_error_code_categories() {
    /// Verify that each ErrorCode maps to the correct ErrorType category

    // Syntax errors
    assert_eq!(ErrorCode::YAML_INVALID_SYNTAX.error_type(), ErrorType::Syntax);
    assert_eq!(ErrorCode::YAML_INVALID_INDENTATION.error_type(), ErrorType::Syntax);
    assert_eq!(ErrorCode::YAML_INVALID_DELIMITER.error_type(), ErrorType::Syntax);

    // Type mismatches
    assert_eq!(ErrorCode::TYPE_EXPECTED_INTEGER.error_type(), ErrorType::TypeMismatch);
    assert_eq!(ErrorCode::TYPE_EXPECTED_STRING.error_type(), ErrorType::TypeMismatch);
    assert_eq!(ErrorCode::TYPE_EXPECTED_BOOLEAN.error_type(), ErrorType::TypeMismatch);
    assert_eq!(ErrorCode::TYPE_UNEXPECTED_NULL.error_type(), ErrorType::TypeMismatch);

    // Validation errors
    assert_eq!(ErrorCode::VALIDATION_REQUIRED_FIELD_MISSING.error_type(), ErrorType::Validation);
    assert_eq!(ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE.error_type(), ErrorType::Validation);
    assert_eq!(ErrorCode::VALIDATION_STRING_TOO_SHORT.error_type(), ErrorType::Validation);
    assert_eq!(ErrorCode::VALIDATION_PATTERN_MISMATCH.error_type(), ErrorType::Validation);

    // I/O errors
    assert_eq!(ErrorCode::IO_FILE_NOT_FOUND.error_type(), ErrorType::Io);
    assert_eq!(ErrorCode::IO_PERMISSION_DENIED.error_type(), ErrorType::Io);

    // Encoding errors
    assert_eq!(ErrorCode::ENCODING_INVALID_UTF8.error_type(), ErrorType::InvalidUtf8);

    // Other errors
    assert_eq!(ErrorCode::ANCHOR_UNKNOWN.error_type(), ErrorType::UnknownAnchor);
    assert_eq!(ErrorCode::KEY_DUPLICATE.error_type(), ErrorType::DuplicateKey);
    assert_eq!(ErrorCode::EOF_UNEXPECTED.error_type(), ErrorType::UnexpectedEof);
}

#[test]
fn test_error_code_descriptions() {
    /// Verify that error codes provide human-readable descriptions

    assert_eq!(
        ErrorCode::VALIDATION_REQUIRED_FIELD_MISSING.description(),
        "Required field is missing"
    );
    assert_eq!(
        ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE.description(),
        "Value out of allowed range"
    );
    assert_eq!(
        ErrorCode::TYPE_EXPECTED_INTEGER.description(),
        "Expected integer type"
    );
    assert_eq!(
        ErrorCode::YAML_INVALID_INDENTATION.description(),
        "Invalid YAML indentation"
    );
    assert_eq!(
        ErrorCode::IO_FILE_NOT_FOUND.description(),
        "File not found"
    );
}

// ============================================================================
// Section 2: ErrorType display formatting
// ============================================================================

#[test]
fn test_error_type_display() {
    /// Verify that ErrorType provides user-friendly display strings

    assert_eq!(format!("{}", ErrorType::Validation), "validation error");
    assert_eq!(format!("{}", ErrorType::TypeMismatch), "type mismatch");
    assert_eq!(format!("{}", ErrorType::Syntax), "syntax error");
    assert_eq!(format!("{}", ErrorType::Io), "I/O error");
    assert_eq!(format!("{}", ErrorType::InvalidUtf8), "invalid UTF-8 encoding");
    assert_eq!(format!("{}", ErrorType::UnknownAnchor), "unknown anchor");
    assert_eq!(format!("{}", ErrorType::DuplicateKey), "duplicate key");
}

// ============================================================================
// Section 3: ErrorCode display formatting
// ============================================================================

#[test]
fn test_error_code_display() {
    /// Verify that ErrorCode provides programmatic display strings

    let display = format!("{}", ErrorCode::VALIDATION_REQUIRED_FIELD_MISSING);
    assert!(display.contains("VALIDATION_REQUIRED_FIELD_MISSING"));

    let display = format!("{}", ErrorCode::TYPE_EXPECTED_STRING);
    assert!(display.contains("TYPE_EXPECTED_STRING"));
}

// ============================================================================
// Section 4: ValidationError integration with error codes
// ============================================================================

#[test]
fn test_validation_error_with_error_code() {
    /// Verify that ValidationError integrates with error codes from bf-68hqo

    let error = ValidationError::new("port", "port must be between 1 and 65535")
        .with_code(ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE)
        .with_line(10);

    assert_eq!(error.code, ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE);
    assert_eq!(error.error_type(), ErrorType::Validation);
    assert_eq!(error.line, Some(10));
}

#[test]
fn test_validation_error_default_code() {
    /// Verify that ValidationError has a sensible default error code

    let error = ValidationError::new("field", "invalid value");
    assert_eq!(error.code, ErrorCode::VALIDATION_INVALID_VALUE);
}

#[test]
fn test_validation_error_builder_pattern() {
    /// Verify the builder pattern works with error codes

    let error = ValidationError::new("server.port", "out of range")
        .with_code(ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE)
        .with_line(42)
        .with_column(15);

    assert_eq!(error.path, "server.port");
    assert_eq!(error.message, "out of range");
    assert_eq!(error.code, ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE);
    assert_eq!(error.line, Some(42));
    assert_eq!(error.column, Some(15));
}

// ============================================================================
// Section 5: Error code coverage across categories
// ============================================================================

#[test]
fn test_validation_error_codes() {
    /// Verify all validation error codes are present and categorized correctly

    let validation_codes = vec![
        ErrorCode::VALIDATION_REQUIRED_FIELD_MISSING,
        ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE,
        ErrorCode::VALIDATION_STRING_TOO_SHORT,
        ErrorCode::VALIDATION_STRING_TOO_LONG,
        ErrorCode::VALIDATION_PATTERN_MISMATCH,
        ErrorCode::VALIDATION_INVALID_VALUE,
        ErrorCode::VALIDATION_ARRAY_TOO_FEW_ITEMS,
        ErrorCode::VALIDATION_ARRAY_TOO_MANY_ITEMS,
        ErrorCode::VALIDATION_ARRAY_NOT_UNIQUE,
        ErrorCode::VALIDATION_OBJECT_TOO_FEW_PROPERTIES,
        ErrorCode::VALIDATION_OBJECT_TOO_MANY_PROPERTIES,
    ];

    for code in validation_codes {
        assert_eq!(code.error_type(), ErrorType::Validation);
    }
}

#[test]
fn test_type_mismatch_error_codes() {
    /// Verify all type mismatch error codes are present and categorized correctly

    let type_codes = vec![
        ErrorCode::TYPE_EXPECTED_INTEGER,
        ErrorCode::TYPE_EXPECTED_STRING,
        ErrorCode::TYPE_EXPECTED_BOOLEAN,
        ErrorCode::TYPE_EXPECTED_ARRAY,
        ErrorCode::TYPE_EXPECTED_OBJECT,
        ErrorCode::TYPE_EXPECTED_NUMBER,
        ErrorCode::TYPE_UNEXPECTED_NULL,
    ];

    for code in type_codes {
        assert_eq!(code.error_type(), ErrorType::TypeMismatch);
    }
}

#[test]
fn test_syntax_error_codes() {
    /// Verify all syntax error codes are present and categorized correctly

    let syntax_codes = vec![
        ErrorCode::YAML_INVALID_SYNTAX,
        ErrorCode::YAML_INVALID_INDENTATION,
        ErrorCode::YAML_INVALID_DELIMITER,
        ErrorCode::YAML_INVALID_ESCAPE_SEQUENCE,
        ErrorCode::YAML_INVALID_SCALAR,
    ];

    for code in syntax_codes {
        assert_eq!(code.error_type(), ErrorType::Syntax);
    }
}

// ============================================================================
// Section 6: Real-world error scenarios
// ============================================================================

#[test]
fn test_real_world_validation_error_scenario() {
    /// Documents a realistic validation error with error codes
    ///
    /// Scenario: Port number validation in service configuration
    ///
    /// config.yaml:
    /// ```yaml
    /// services:
    ///   - name: web
    ///     port: 70000  # ERROR: out of range (1-65535)
    /// ```
    ///
    /// Error output:
    /// ```text
    /// 10: validation error at 'services[0].port': port must be between 1 and 65535
    /// ```

    let error = ValidationError::new("services[0].port", "port must be between 1 and 65535")
        .with_code(ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE)
        .with_line(10);

    assert_eq!(error.path, "services[0].port");
    assert_eq!(error.code, ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE);
    assert_eq!(error.error_type(), ErrorType::Validation);
    assert!(error.message.contains("1 and 65535"));

    let display = format!("{}", error);
    assert!(display.contains("10:"));
    assert!(display.contains("validation error at 'services[0].port'"));
}

#[test]
fn test_real_world_type_mismatch_scenario() {
    /// Documents a realistic type mismatch error with error codes
    ///
    /// Scenario: String provided where integer expected
    ///
    /// config.yaml:
    /// ```yaml
    /// database:
    ///   port: "5432"  # ERROR: should be integer, not string
    /// ```

    let error = ValidationError::new("database.port", "expected integer, got string")
        .with_code(ErrorCode::TYPE_EXPECTED_INTEGER)
        .with_line(5);

    assert_eq!(error.code, ErrorCode::TYPE_EXPECTED_INTEGER);
    assert_eq!(error.error_type(), ErrorType::TypeMismatch);
    assert!(error.message.contains("integer"));
}

#[test]
fn test_real_world_required_field_scenario() {
    /// Documents a realistic required field error with error codes
    ///
    /// Scenario: Missing required field in configuration
    ///
    /// config.yaml:
    /// ```yaml
    /// service:
    ///   port: 8080
    ///   # ERROR: missing required 'name' field
    /// ```

    let error = ValidationError::new("service.name", "required field is missing")
        .with_code(ErrorCode::VALIDATION_REQUIRED_FIELD_MISSING)
        .with_line(3);

    assert_eq!(error.code, ErrorCode::VALIDATION_REQUIRED_FIELD_MISSING);
    assert_eq!(error.error_type(), ErrorType::Validation);
}

// ============================================================================
// Section 7: Error code consistency
// ============================================================================

#[test]
fn test_error_code_equality() {
    /// Verify that error codes support equality checks

    assert_eq!(ErrorCode::VALIDATION_INVALID_VALUE, ErrorCode::VALIDATION_INVALID_VALUE);
    assert_ne!(ErrorCode::VALIDATION_INVALID_VALUE, ErrorCode::TYPE_EXPECTED_INTEGER);
}

#[test]
fn test_error_type_equality() {
    /// Verify that error types support equality checks

    assert_eq!(ErrorType::Validation, ErrorType::Validation);
    assert_ne!(ErrorType::Validation, ErrorType::TypeMismatch);
}
