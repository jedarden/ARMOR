//! Test cases for ValidationError contextual formatting
//!
//! This test verifies that ValidationError messages include:
//! - Field path (e.g., "server.port")
//! - Line number context when available
//! - Human-readable error messages
//! - Consistent formatting with ParseError

use armor::parsers::yaml::ValidationError;

// ============================================================================
// Section 1: ValidationError with field paths
// ============================================================================

#[test]
fn test_validation_error_with_field_path() {
    /// Demonstrates ValidationError format with field path
    ///
    /// Format: `validation error at '<field-path>': <message>`
    ///
    /// Example output:
    /// ```text
    /// validation error at 'server.port': port must be between 1 and 65535
    /// ```
    let error = ValidationError::new("server.port", "port must be between 1 and 65535");
    let display = format!("{}", error);

    assert!(display.contains("validation error at 'server.port'"), "Should include field path");
    assert!(display.contains("port must be between 1 and 65535"), "Should include message");
}

#[test]
fn test_validation_error_with_line_context() {
    /// Demonstrates ValidationError format with line number context
    ///
    /// Format: `<line>: validation error at '<field-path>': <message>`
    ///
    /// Example output:
    /// ```text
    /// 15: validation error at 'server.port': port must be between 1 and 65535
    /// ```
    let error = ValidationError::new("server.port", "port must be between 1 and 65535")
        .with_line(15);
    let display = format!("{}", error);

    assert!(display.contains("15:"), "Should include line number");
    assert!(display.contains("validation error at 'server.port'"), "Should include field path");
    assert!(display.contains("port must be between 1 and 65535"), "Should include message");
}

#[test]
fn test_validation_error_nested_field_paths() {
    /// Documents ValidationError with various field path formats
    ///
    /// Field paths can be:
    /// - Simple: "server.port"
    /// - Nested: "database.connectionPool.maxConnections"
    /// - Array access: "servers.api.responses[0].statusCode"

    let test_cases = vec![
        ("server.port", "Simple dot-notation path"),
        ("database.connectionPool.maxConnections", "Nested dot-notation path"),
        ("servers.api.responses[0].statusCode", "Array access path"),
        ("spec.template.spec.containers[0].image", "Deep Kubernetes-style path"),
    ];

    for (field_path, description) in test_cases {
        let error = ValidationError::new(field_path, "constraint violation");
        let display = format!("{}", error);

        assert!(display.contains(&format!("validation error at '{}'", field_path)),
            "{}: Should include field path '{}'", description, field_path);
        assert!(display.contains("constraint violation"),
            "{}: Should include message", description);
    }
}

#[test]
fn test_validation_error_complete_format() {
    /// Documents complete ValidationError with all components
    ///
    /// Format breakdown:
    /// - Line number (if present): `<line>: `
    /// - Error type: "validation error at"
    /// - Field path: `'<field-path>'`
    /// - Message: The specific validation failure
    ///
    /// Example:
    /// ```text
    /// 42: validation error at 'spec.template.spec.containers[0].image': invalid image tag
    /// ```
    let error = ValidationError::new("spec.template.spec.containers[0].image", "invalid image tag")
        .with_line(42);

    let display = format!("{}", error);

    // Verify all components
    assert!(display.contains("42:"), "Should include line number");
    assert!(display.contains("validation error at 'spec.template.spec.containers[0].image'"), "Should include field path");
    assert!(display.contains("invalid image tag"), "Should include message");
}

// ============================================================================
// Section 2: ValidationError scenarios
// ============================================================================

#[test]
fn test_validation_error_type_mismatch() {
    /// Documents ValidationError for type mismatch scenarios
    ///
    /// Example output:
    /// ```text
    /// validation error at 'database.port': expected integer, got string
    /// ```
    let error = ValidationError::new("database.port", "expected integer, got string");
    let display = format!("{}", error);

    assert!(display.contains("validation error at 'database.port'"), "Should include field path");
    assert!(display.contains("expected integer, got string"), "Should include type information");
}

#[test]
fn test_validation_error_constraint_violation() {
    /// Documents ValidationError for constraint violations
    ///
    /// Example output:
    /// ```text
    /// 25: validation error at 'server.timeout': value must be positive
    /// ```
    let error = ValidationError::new("server.timeout", "value must be positive")
        .with_line(25);
    let display = format!("{}", error);

    assert!(display.contains("25:"), "Should include line number");
    assert!(display.contains("validation error at 'server.timeout'"), "Should include field path");
    assert!(display.contains("value must be positive"), "Should include constraint message");
}

#[test]
fn test_validation_error_missing_required_field() {
    /// Documents ValidationError for missing required fields
    ///
    /// Example output:
    /// ```text
    /// validation error at 'service.name': required field is missing
    /// ```
    let error = ValidationError::new("service.name", "required field is missing");
    let display = format!("{}", error);

    assert!(display.contains("validation error at 'service.name'"), "Should include field path");
    assert!(display.contains("required field is missing"), "Should include message");
}

// ============================================================================
// Section 3: Format consistency
// ============================================================================

#[test]
fn test_validation_error_format_consistency() {
    /// Verifies that ValidationError formats follow consistent patterns
    ///
    /// Consistency rules:
    /// 1. Line number (if present) comes first: `<line>: `
    /// 2. Error type follows: "validation error at"
    /// 3. Field path is quoted: `'<field-path>'`
    /// 4. Message follows with colon separator: `': <message>'`

    let errors = vec![
        ValidationError::new("field1", "error1").with_line(1),
        ValidationError::new("field2", "error2").with_line(2),
        ValidationError::new("field3", "error3"),
    ];

    for error in errors {
        let display = format!("{}", error);

        // Check for error type and field path format
        assert!(display.contains("validation error at '"), "Should include 'validation error at '");
        assert!(display.contains("': "), "Should include field path closing quote and colon");

        // If line is present, it should be at the start
        if let Some(line) = error.line {
            assert!(display.starts_with(&format!("{}: ", line)),
                "With line {}, display should start with '{}: '", line, line);
        }
    }
}

#[test]
fn test_validation_error_builder_pattern() {
    /// Tests the builder pattern for ValidationError
    let error = ValidationError::new("test.field", "test message");
    assert_eq!(error.path, "test.field");
    assert_eq!(error.message, "test message");
    assert_eq!(error.line, None);

    let error_with_line = error.with_line(10);
    assert_eq!(error_with_line.line, Some(10));
    assert_eq!(error_with_line.path, "test.field");
    assert_eq!(error_with_line.message, "test message");
}

// ============================================================================
// Section 4: Human-readability
// ============================================================================

#[test]
fn test_validation_error_human_readable() {
    /// Verifies that ValidationError messages are human-readable
    ///
    /// Human-readable messages should:
    /// 1. Use clear, non-technical language when possible
    /// 2. Provide actionable information
    /// 3. Include field context for location

    let errors = vec![
        ValidationError::new("port", "must be between 1 and 65535"),
        ValidationError::new("timeout", "cannot be negative"),
        ValidationError::new("email", "must be a valid email address"),
        ValidationError::new("url", "must start with http:// or https://"),
    ];

    for error in errors {
        let display = format!("{}", error);

        // Should be readable (not empty, contain field path)
        assert!(!display.is_empty(), "Display should not be empty");
        assert!(display.contains(&format!("validation error at '{}'", error.path)),
            "Should include field path '{}'", error.path);
        assert!(display.contains(&error.message), "Should include message");

        // Should not contain internal implementation details
        assert!(!display.contains("ValidationError{"), "Should not expose struct name");
        assert!(!display.contains("Some("), "Should not expose Option types");
    }
}

#[test]
fn test_validation_error_real_world_example() {
    /// Documents a realistic validation error scenario
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
        .with_line(10);

    let display = format!("{}", error);

    assert!(display.contains("10:"), "Should include line number");
    assert!(display.contains("validation error at 'services[0].port'"), "Should include field path");
    assert!(display.contains("port must be between 1 and 65535"), "Should include message");
    assert!(display.contains("1 and 65535"), "Should include constraint range");
}
