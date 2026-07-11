//! Acceptance Criteria Verification Test for Contextual Error Message Formatting
//!
//! This test verifies that bead bf-355bv meets all acceptance criteria:
//! - AC1: ParseError messages include "line X, column Y" context
//! - AC2: ValidationError messages include field path (e.g., "spec.replicas")
//! - AC3: Type mismatch errors include expected and actual types
//! - AC4: All error messages follow consistent formatting
//! - AC5: Examples of error message formats in test cases

use armor::parsers::yaml::{ParseError, ValidationError};

#[test]
fn test_acceptance_criteria_parse_error_line_column_context() {
    // AC1: ParseError messages include "line X, column Y" context
    let error = ParseError::syntax("Missing colon")
        .with_path("config.yaml")
        .with_line(10)
        .with_column(5);

    let display = format!("{}", error);

    // Verify line:column context is included
    assert!(display.contains("config.yaml:10:5"),
        "ParseError should include 'file:line:column' format");
    assert!(display.contains("syntax error: Missing colon"),
        "ParseError should include error kind and message");

    // Test line only (no column)
    let error_line_only = ParseError::syntax("Invalid structure")
        .with_path("config.yaml")
        .with_line(15);
    let display_line_only = format!("{}", error_line_only);
    assert!(display_line_only.contains("config.yaml:15"),
        "ParseError with line only should include 'file:line' format");

    // Test line:column only (no path)
    let error_no_path = ParseError::syntax("Unexpected token")
        .with_line(20)
        .with_column(8);
    let display_no_path = format!("{}", error_no_path);
    assert!(display_no_path.contains("20:8"),
        "ParseError without path should include 'line:column' format");
}

#[test]
fn test_acceptance_criteria_validation_error_field_path() {
    // AC2: ValidationError messages include field path (e.g., "spec.replicas")
    let error = ValidationError::new("spec.replicas", "port out of range")
        .with_line(15);

    let display = format!("{}", error);

    // Verify field path is included
    assert!(display.contains("spec.replicas"),
        "ValidationError should include field path 'spec.replicas'");
    assert!(display.contains("15:"),
        "ValidationError should include line number");
    assert!(display.contains("validation error at 'spec.replicas'"),
        "ValidationError should include 'validation error at' format with quoted path");
    assert!(display.contains("port out of range"),
        "ValidationError should include error message");

    // Test without line number
    let error_no_line = ValidationError::new("server.host", "hostname cannot be empty");
    let display_no_line = format!("{}", error_no_line);
    assert!(display_no_line.contains("validation error at 'server.host'"),
        "ValidationError without line should include 'validation error at' format");
    assert!(display_no_line.contains("hostname cannot be empty"),
        "ValidationError should include error message");
}

#[test]
fn test_acceptance_criteria_type_mismatch_expected_actual() {
    // AC3: Type mismatch errors include expected and actual types
    let error = ParseError::type_mismatch("server.port", "integer", "string");

    let display = format!("{}", error);

    // Verify expected vs actual type information
    assert!(display.contains("type mismatch at 'server.port'"),
        "TypeMismatch should include field path");
    assert!(display.contains("expected integer"),
        "TypeMismatch should include expected type");
    assert!(display.contains("got string"),
        "TypeMismatch should include actual type");

    // Test with location
    let error_with_loc = ParseError::type_mismatch("database.connectionPool.maxConnections", "integer", "boolean")
        .with_path("services.yaml")
        .with_line(25)
        .with_column(10);
    let display_with_loc = format!("{}", error_with_loc);
    assert!(display_with_loc.contains("services.yaml:25:10"),
        "TypeMismatch with location should include 'file:line:column'");
    assert!(display_with_loc.contains("type mismatch at 'database.connectionPool.maxConnections'"),
        "TypeMismatch should include nested field path");
    assert!(display_with_loc.contains("expected integer"),
        "TypeMismatch should include expected type");
    assert!(display_with_loc.contains("got boolean"),
        "TypeMismatch should include actual type");
}

#[test]
fn test_acceptance_criteria_consistent_formatting() {
    // AC4: All error messages follow consistent formatting

    // ParseError formatting pattern: "location: error-kind: message"
    let parse_err = ParseError::syntax("invalid token")
        .with_path("config.yaml")
        .with_line(10);
    let parse_display = format!("{}", parse_err);
    assert!(parse_display.contains("config.yaml:10:"),
        "ParseError should start with location");

    // ValidationError formatting pattern: "line: validation error at 'path': message"
    let valid_err = ValidationError::new("field.name", "constraint violation")
        .with_line(15);
    let valid_display = format!("{}", valid_err);
    assert!(valid_display.contains("15:"),
        "ValidationError should start with line number");
    assert!(valid_display.contains("validation error at 'field.name'"),
        "ValidationError should include quoted field path");

    // Verify all messages are human-readable
    assert!(!parse_display.contains("ParseError{"),
        "Error messages should not expose struct names");
    assert!(!parse_display.contains("Some("),
        "Error messages should not expose Option types");
    assert!(!valid_display.contains("ValidationError{"),
        "Error messages should not expose struct names");
}

#[test]
fn test_acceptance_criteria_examples_in_tests() {
    // AC5: Examples of error message formats in test cases

    // Example 1: ParseError with full context
    let parse_example = ParseError::syntax("Invalid escape sequence")
        .with_path("config.yaml")
        .with_line(5)
        .with_column(18)
        .with_context("while parsing service configuration")
        .with_snippet("description: \"Product \\x name\"");

    let summary = parse_example.summary();
    assert!(summary.contains("config.yaml:5:18"),
        "Example should show line:column format");
    assert!(summary.contains("syntax error: Invalid escape sequence"),
        "Example should show error kind and message");
    assert!(summary.contains("while parsing service configuration"),
        "Example should include context");

    let detailed = parse_example.detailed_report();
    assert!(detailed.contains("snippet:"),
        "Example should include code snippet");
    assert!(detailed.contains("^"),
        "Example should include visual indicator");

    // Example 2: ValidationError scenarios
    let validation_examples = vec![
        ("server.port", "port must be between 1 and 65535", Some(10)),
        ("database.host", "hostname cannot be empty", Some(25)),
        ("service.name", "required field is missing", None),
    ];

    for (field_path, message, line) in validation_examples {
        let mut error = ValidationError::new(field_path, message);
        if let Some(ln) = line {
            error = error.with_line(ln);
        }

        let display = format!("{}", error);
        assert!(display.contains(field_path),
            "Example should include field path '{}'", field_path);
        assert!(display.contains(message),
            "Example should include message '{}'", message);
    }

    // Example 3: Type mismatch scenarios
    let type_mismatch_examples = vec![
        ("port", "integer", "string"),
        ("enabled", "boolean", "string"),
        ("count", "integer", "null"),
        ("tags", "array", "scalar"),
    ];

    for (field, expected, actual) in type_mismatch_examples {
        let error = ParseError::type_mismatch(field, expected, actual);
        let display = format!("{}", error);
        assert!(display.contains(&format!("type mismatch at '{}'", field)),
            "Example should include field path '{}'", field);
        assert!(display.contains(&format!("expected {}", expected)),
            "Example should include expected type '{}'", expected);
        assert!(display.contains(&format!("got {}", actual)),
            "Example should include actual type '{}'", actual);
    }
}
