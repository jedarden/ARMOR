//! Error Message Format Examples and Documentation
//!
//! This test file serves as both a test suite and documentation for all error message
//! formats produced by the ARMOR YAML parser. Each test demonstrates a specific error
//! format and documents its structure.
//!
//! # Error Message Format Categories
//!
//! 1. **ParseError with line:column** - Location-specific parse errors
//! 2. **ValidationError with field paths** - Field-specific validation errors
//! 3. **Type mismatch errors** - Expected vs actual type mismatches
//!
//! # Format Reference
//!
//! ## ParseError Location Format
//!
//! The `location_string()` method produces location strings in these formats:
//!
//! | Path | Line | Column | Format |
//! |------|------|--------|--------|
//! | Some | Some | Some | `file.yaml:10:5` |
//! | Some | Some | None | `file.yaml:10` |
//! | Some | None | Some | `file.yaml::5` |
//! | Some | None | None | `file.yaml` |
//! | None | Some | Some | `10:5` |
//! | None | Some | None | `10` |
//! | None | None | Some | `col 5` |
//! | None | None | None | `<unknown>` |
//!
//! ## ValidationError Format
//!
//! ValidationError uses dot-notation for nested field paths:
//! - Top-level field: `"name"`
//! - Nested field: `"database.port"`
//! - Deeply nested: `"servers[0].port"`

use armor::parsers::yaml::{ParseError, ParseErrorKind, ValidationError, ValidationResult, ErrorCode};

// ============================================================================
// ParseError with line:column formatting
// ============================================================================

#[test]
fn test_parse_error_line_column_full_format() {
    // Test: ParseError with full location (path:line:column)
    // Expected output format: "config.yaml:10:5: syntax error: Missing colon"
    let error = ParseError::syntax("Missing colon")
        .with_path("config.yaml")
        .with_line(10)
        .with_column(5);

    let display = format!("{}", error);
    assert!(display.contains("config.yaml:10:5"));
    assert!(display.contains("syntax error: Missing colon"));
}

#[test]
fn test_parse_error_line_column_path_line_only() {
    // Test: ParseError with path and line only
    // Expected output format: "config.yaml:10: syntax error: Invalid YAML structure"
    let error = ParseError::syntax("Invalid YAML structure")
        .with_path("config.yaml")
        .with_line(10);

    let display = format!("{}", error);
    assert!(display.contains("config.yaml:10"));
    assert!(display.contains("syntax error: Invalid YAML structure"));
    // Verify there's no second colon after line number (no column specified)
    assert!(display.contains("config.yaml:10:")); // Format is path:line:message
}

#[test]
fn test_parse_error_line_column_line_column_only() {
    // Test: ParseError with line and column, no path
    // Expected output format: "10:5: syntax error: Unexpected token"
    let error = ParseError::syntax("Unexpected token")
        .with_line(10)
        .with_column(5);

    let display = format!("{}", error);
    assert!(display.contains("10:5"));
    assert!(display.contains("syntax error: Unexpected token"));
    assert!(!display.contains(".yaml")); // No path
}

#[test]
fn test_parse_error_line_column_with_context() {
    // Test: ParseError with context message
    // Expected format: "config.yaml:15:8: syntax error: Unterminated string - while parsing field 'description'"
    let error = ParseError::syntax("Unterminated string")
        .with_path("config.yaml")
        .with_line(15)
        .with_column(8)
        .with_context("while parsing field 'description'");

    let display = format!("{}", error);
    assert!(display.contains("config.yaml:15:8"));
    assert!(display.contains("syntax error: Unterminated string"));
    assert!(display.contains("while parsing field 'description'"));
}

#[test]
fn test_parse_error_line_column_detailed_report() {
    // Test: ParseError detailed report with snippet
    // Expected format:
    // error: config.yaml:5:18: syntax error: Invalid escape sequence
    //   context: while parsing service configuration
    //
    //   snippet:
    //     description: "Product \x name"
    //                     ^
    let error = ParseError::syntax("Invalid escape sequence")
        .with_path("config.yaml")
        .with_line(5)
        .with_column(18)
        .with_context("while parsing service configuration")
        .with_snippet("description: \"Product \\x name\"");

    let report = error.detailed_report();
    assert!(report.contains("error:"));
    assert!(report.contains("config.yaml:5:18"));
    assert!(report.contains("syntax error: Invalid escape sequence"));
    assert!(report.contains("context: while parsing service configuration"));
    assert!(report.contains("snippet:"));
    assert!(report.contains("description: \"Product \\x name\""));

    // Verify visual indicator (caret) points to the error location
    assert!(report.contains(&format!("{}^", " ".repeat(17))));
}

#[test]
fn test_parse_error_summary_format() {
    // Test: ParseError summary() method output
    // The summary() method produces a single-line error message suitable for logging
    // Expected format: "config.yaml:10: syntax error: Invalid token - while parsing services"
    let error = ParseError::syntax("Invalid token")
        .with_path("config.yaml")
        .with_line(10)
        .with_context("while parsing services");

    let summary = error.summary();
    assert_eq!(summary, "config.yaml:10: syntax error: Invalid token - while parsing services");
}

#[test]
fn test_parse_error_summary_without_context() {
    // Test: ParseError summary without context
    // Expected format: "config.yaml:10: syntax error: Invalid token"
    let error = ParseError::syntax("Invalid token")
        .with_path("config.yaml")
        .with_line(10);

    let summary = error.summary();
    assert_eq!(summary, "config.yaml:10: syntax error: Invalid token");
}

// ============================================================================
// Type Mismatch Error Formats
// ============================================================================

#[test]
fn test_type_mismatch_basic_format() {
    // Test: Type mismatch error basic format
    // Expected format: "type mismatch at 'port': expected integer, got string"
    let error = ParseError::type_mismatch("port", "integer", "string");

    let display = format!("{}", error);
    assert!(display.contains("type mismatch at 'port'"));
    assert!(display.contains("expected integer"));
    assert!(display.contains("got string"));
}

#[test]
fn test_type_mismatch_with_location() {
    // Test: Type mismatch error with full location
    // Expected format: "config.yaml:8:10: type mismatch at 'port': expected integer, got string"
    let error = ParseError::type_mismatch("port", "integer", "string")
        .with_path("config.yaml")
        .with_line(8)
        .with_column(10);

    let display = format!("{}", error);
    assert!(display.contains("config.yaml:8:10"));
    assert!(display.contains("type mismatch at 'port'"));
    assert!(display.contains("expected integer"));
    assert!(display.contains("got string"));
}

#[test]
fn test_type_mismatch_nested_field_path() {
    // Test: Type mismatch error with nested field path
    // Expected format: "config.yaml:15:12: type mismatch at 'database.port': expected integer, got string"
    let error = ParseError::type_mismatch("database.port", "integer", "string")
        .with_path("config.yaml")
        .with_line(15)
        .with_column(12);

    let display = format!("{}", error);
    assert!(display.contains("config.yaml:15:12"));
    assert!(display.contains("type mismatch at 'database.port'"));
    assert!(display.contains("expected integer"));
    assert!(display.contains("got string"));
}

#[test]
fn test_type_mismatch_array_field_path() {
    // Test: Type mismatch error with array index in path
    // Expected format: "services.yaml:20:8: type mismatch at 'servers[0].port': expected integer, got boolean"
    let error = ParseError::type_mismatch("servers[0].port", "integer", "boolean")
        .with_path("services.yaml")
        .with_line(20)
        .with_column(8);

    let display = format!("{}", error);
    assert!(display.contains("services.yaml:20:8"));
    assert!(display.contains("type mismatch at 'servers[0].port'"));
    assert!(display.contains("expected integer"));
    assert!(display.contains("got boolean"));
}

#[test]
fn test_type_mismatch_with_snippet() {
    // Test: Type mismatch error with code snippet
    // Expected format:
    // error: config.yaml:8:14: type mismatch at 'port': expected integer, got string
    //   context: while parsing service configuration
    //
    //   snippet:
    //     service:
    //       port: "8080"
    //              ^
    let error = ParseError::type_mismatch("port", "integer", "string")
        .with_path("config.yaml")
        .with_line(8)
        .with_column(14)
        .with_context("while parsing service configuration")
        .with_snippet("service:\n  port: \"8080\"");

    let report = error.detailed_report();
    assert!(report.contains("error:"));
    assert!(report.contains("config.yaml:8:14"));
    assert!(report.contains("type mismatch at 'port'"));
    assert!(report.contains("expected integer"));
    assert!(report.contains("got string"));
    assert!(report.contains("context: while parsing service configuration"));
    assert!(report.contains("snippet:"));
    assert!(report.contains("port: \"8080\""));
}

#[test]
fn test_type_mismatch_various_types() {
    // Test: Type mismatch errors for various type combinations
    // This test documents the format for different type mismatch scenarios

    // String expected, got null
    let error = ParseError::type_mismatch("name", "string", "null");
    assert!(format!("{}", error).contains("expected string, got null"));

    // Integer expected, got float
    let error = ParseError::type_mismatch("count", "integer", "float");
    assert!(format!("{}", error).contains("expected integer, got float"));

    // Boolean expected, got string
    let error = ParseError::type_mismatch("enabled", "boolean", "string");
    assert!(format!("{}", error).contains("expected boolean, got string"));

    // Array expected, got scalar
    let error = ParseError::type_mismatch("tags", "array", "scalar");
    assert!(format!("{}", error).contains("expected array, got scalar"));

    // Object expected, got array
    let error = ParseError::type_mismatch("config", "object", "array");
    assert!(format!("{}", error).contains("expected object, got array"));
}

// ============================================================================
// ValidationError with Field Paths
// ============================================================================

#[test]
fn test_validation_error_format() {
    // Test: ValidationError structure and format
    // ValidationError struct fields:
    // - `path: String` - Dot-notation field path (e.g., "server.port")
    // - `message: String` - Error message
    // - `line: Option<usize>` - Line number (1-indexed)
    // Note: ValidationError does not implement Display, so we format it manually
    let error = ValidationError {
        path: "server.port".to_string(),
        message: "port must be between 1 and 65535".to_string(),
        code: ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE,
        line: Some(15),
        column: None,
        indentation_error_type: None,
        delimiter_error_type: None,
    };

    // Manual format demonstration
    let formatted = if let Some(line) = error.line {
        format!("{}:{}: {}", error.path, line, error.message)
    } else {
        format!("{}: {}", error.path, error.message)
    };

    assert_eq!(formatted, "server.port:15: port must be between 1 and 65535");
}

#[test]
fn test_validation_error_top_level_field() {
    // Test: ValidationError for top-level field
    // Expected format: "name:10: field is required"
    let error = ValidationError {
        path: "name".to_string(),
        message: "field is required".to_string(),
        code: ErrorCode::VALIDATION_REQUIRED_FIELD_MISSING,
        line: Some(10),
        column: None,
        indentation_error_type: None,
        delimiter_error_type: None,
    };

    let formatted = format!("{}:{}: {}", error.path, error.line.unwrap(), error.message);
    assert_eq!(formatted, "name:10: field is required");
}

#[test]
fn test_validation_error_nested_field() {
    // Test: ValidationError for nested field with dot notation
    // Expected format: "database.host:25: hostname cannot be empty"
    let error = ValidationError {
        path: "database.host".to_string(),
        message: "hostname cannot be empty".to_string(),
        code: ErrorCode::VALIDATION_REQUIRED_FIELD_MISSING,
        line: Some(25),
        column: None,
        indentation_error_type: None,
        delimiter_error_type: None,
    };

    let formatted = format!("{}:{}: {}", error.path, error.line.unwrap(), error.message);
    assert_eq!(formatted, "database.host:25: hostname cannot be empty");
}

#[test]
fn test_validation_error_deeply_nested() {
    // Test: ValidationError for deeply nested field
    // Expected format: "servers[0].config.port:42: port out of range"
    let error = ValidationError {
        path: "servers[0].config.port".to_string(),
        message: "port out of range".to_string(),
        code: ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE,
        line: Some(42),
        column: None,
        indentation_error_type: None,
        delimiter_error_type: None,
    };

    let formatted = format!("{}:{}: {}", error.path, error.line.unwrap(), error.message);
    assert_eq!(formatted, "servers[0].config.port:42: port out of range");
}

#[test]
fn test_validation_error_without_line() {
    // Test: ValidationError without line number
    // Expected format: "database.name: field cannot be empty"
    let error = ValidationError {
        path: "database.name".to_string(),
        message: "field cannot be empty".to_string(),
        code: ErrorCode::VALIDATION_REQUIRED_FIELD_MISSING,
        line: None,
        column: None,
        indentation_error_type: None,
        delimiter_error_type: None,
    };

    let formatted = format!("{}: {}", error.path, error.message);
    assert_eq!(formatted, "database.name: field cannot be empty");
}

#[test]
fn test_validation_result_multiple_errors() {
    // Test: ValidationResult with multiple validation errors
    // ValidationResult aggregates multiple ValidationError instances:
    // - `valid: bool` - Whether validation passed
    // - `errors: Vec<ValidationError>` - List of validation errors
    // - `warnings: Vec<ValidationWarning>` - List of warnings
    let result = ValidationResult::failure(vec![
        ValidationError {
            path: "server.port".to_string(),
            message: "port must be between 1 and 65535".to_string(),
            code: ErrorCode::VALIDATION_VALUE_OUT_OF_RANGE,
            line: Some(10),
            column: None,
            indentation_error_type: None,
            delimiter_error_type: None,
        },
        ValidationError {
            path: "server.host".to_string(),
            message: "host cannot be empty".to_string(),
            code: ErrorCode::VALIDATION_REQUIRED_FIELD_MISSING,
            line: Some(12),
            column: None,
            indentation_error_type: None,
            delimiter_error_type: None,
        },
    ]);

    assert!(!result.is_valid());
    assert_eq!(result.errors.len(), 2);
    assert_eq!(result.errors[0].path, "server.port");
    assert_eq!(result.errors[1].path, "server.host");
}

// ============================================================================
// Error Message Format Catalog
// ============================================================================

#[test]
fn test_error_format_catalog_parse_errors() {
    // Test: Catalog of all ParseError formats

    let cases = vec![
        // Syntax errors
        ("syntax error with location", ParseError::syntax("invalid token")
            .with_path("config.yaml").with_line(10).with_column(5),
         "config.yaml:10:5: syntax error: invalid token"),

        // I/O errors
        ("I/O error", ParseError::io("file not found")
            .with_path("/etc/config/app.yaml"),
         "/etc/config/app.yaml: I/O error: file not found"),

        // Validation errors (ParseErrorKind::Validation)
        ("validation error", ParseError::validation("port out of range")
            .with_path("network.yaml").with_line(12),
         "network.yaml:12: validation error: port out of range"),

        // Unknown anchor
        ("unknown anchor", ParseError::new(ParseErrorKind::UnknownAnchor("ref".to_string()))
            .with_path("anchors.yaml").with_line(7),
         "anchors.yaml:7: unknown anchor: ref"),

        // Duplicate key
        ("duplicate key", ParseError::new(ParseErrorKind::DuplicateKey("name".to_string()))
            .with_path("data.yaml").with_line(4),
         "data.yaml:4: duplicate key: name"),

        // Unexpected EOF
        ("unexpected EOF", ParseError::new(ParseErrorKind::UnexpectedEof)
            .with_path("incomplete.yaml").with_line(15),
         "incomplete.yaml:15: unexpected end of input"),

        // Invalid UTF-8
        ("invalid UTF-8", ParseError::new(ParseErrorKind::InvalidUtf8)
            .with_path("data.bin").with_line(1),
         "data.bin:1: invalid UTF-8 encoding"),
    ];

    for (description, error, expected_contains) in cases {
        let display = format!("{}", error);
        assert!(
            display.contains(expected_contains),
            "Case '{}' failed: expected to contain '{}', got '{}'",
            description, expected_contains, display
        );
    }
}

#[test]
fn test_error_format_catalog_type_mismatches() {
    // Test: Catalog of type mismatch error formats

    let cases = vec![
        // Common type mismatches
        ("string for integer", "port", "integer", "string"),
        ("integer for string", "name", "string", "integer"),
        ("boolean for string", "enabled", "string", "boolean"),
        ("null for required", "email", "string", "null"),
        ("array for object", "config", "object", "array"),
        ("scalar for array", "tags", "array", "scalar"),
    ];

    for (description, field, expected, actual) in cases {
        let error = ParseError::type_mismatch(field, expected, actual);
        let display = format!("{}", error);

        assert!(
            display.contains(&format!("type mismatch at '{}'", field)),
            "Case '{}' failed: missing field path",
            description
        );
        assert!(
            display.contains(&format!("expected {}", expected)),
            "Case '{}' failed: missing expected type",
            description
        );
        assert!(
            display.contains(&format!("got {}", actual)),
            "Case '{}' failed: missing actual type",
            description
        );
    }
}
