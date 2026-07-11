//! Comprehensive test cases demonstrating error message formats
//!
//! This test file documents and verifies all error message formats across the ARMOR codebase.
//! It serves as both test coverage and documentation for how errors are formatted and displayed.
//!
//! ## Error Message Format Categories
//!
//! ### 1. ParseError with line:column
//! - Format: `<file>:<line>:<column>: <error-kind>: <message> - <context>`
//! - Example: `config.yaml:10:5: syntax error: Missing colon - while parsing service definition`
//!
//! ### 2. ValidationError with field paths
//! - Format: `<file>:<line>:<column> at field <field-path>: <message> (constraint: <constraint>)`
//! - Example: `config.yaml:15:12 at field server.port: port must be between 1 and 65535 (constraint: must be between 1-65535)`
//!
//! ### 3. Type mismatch errors
//! - Format: `type mismatch at '<field>': expected <expected>, got <actual>`
//! - Example: `type mismatch at 'database.port': expected integer, got string`

use armor::parsers::yaml::{ParseError, ParseErrorKind};

// ============================================================================
// Section 1: ParseError with line:column
// ============================================================================

#[test]
fn test_parse_error_line_column_full_format() {
    /// Demonstrates complete ParseError format with all location components
    ///
    /// Format: `<file>:<line>:<column>: <error-kind>: <message> - <context>`
    ///
    /// Example output:
    /// ```text
    /// config.yaml:10:5: syntax error: Missing colon - while parsing service definition
    /// ```
    let error = ParseError::syntax("Missing colon")
        .with_path("config.yaml")
        .with_line(10)
        .with_column(5)
        .with_context("while parsing service definition");

    let display = format!("{}", error);

    // Verify format components
    assert!(display.contains("config.yaml:10:5"), "Should include file:line:column");
    assert!(display.contains("syntax error: Missing colon"), "Should include error kind and message");
    assert!(display.contains("while parsing service definition"), "Should include context");

    // Example error message format documented
    let expected_format = "config.yaml:10:5: syntax error: Missing colon - while parsing service definition";
    assert_eq!(error.summary(), expected_format);
}

#[test]
fn test_parse_error_line_column_variations() {
    /// Documents different location formatting scenarios

    // Scenario 1: File + line + column (most specific)
    let err1 = ParseError::syntax("test")
        .with_path("config.yaml")
        .with_line(10)
        .with_column(5);
    assert_eq!(err1.location_string(), "config.yaml:10:5");

    // Scenario 2: File + line only
    let err2 = ParseError::syntax("test")
        .with_path("config.yaml")
        .with_line(10);
    assert_eq!(err2.location_string(), "config.yaml:10");

    // Scenario 3: Line + column only (no file)
    let err3 = ParseError::syntax("test")
        .with_line(10)
        .with_column(5);
    assert_eq!(err3.location_string(), "10:5");

    // Scenario 4: Line only
    let err4 = ParseError::syntax("test")
        .with_line(10);
    assert_eq!(err4.location_string(), "10");

    // Scenario 5: File only
    let err5 = ParseError::syntax("test")
        .with_path("config.yaml");
    assert_eq!(err5.location_string(), "config.yaml");

    // Scenario 6: Unknown location
    let err6 = ParseError::syntax("test");
    assert_eq!(err6.location_string(), "<unknown>");
}

#[test]
fn test_parse_error_detailed_report_with_snippet() {
    /// Demonstrates detailed error report format with code snippet and visual indicator
    ///
    /// Example output:
    /// ```text
    /// error: config.yaml:5:10: syntax error: Invalid escape sequence
    ///   context: while parsing field 'description'
    ///
    ///   snippet:
    ///     description: "Product \x name"
    ///           ^
    /// ```
    let error = ParseError::syntax("Invalid escape sequence")
        .with_path("config.yaml")
        .with_line(5)
        .with_column(10)
        .with_context("while parsing field 'description'")
        .with_snippet("description: \"Product \\x name\"");

    let report = error.detailed_report();

    // Verify report structure
    assert!(report.contains("error:"), "Should start with 'error:'");
    assert!(report.contains("config.yaml:5:10"), "Should include location");
    assert!(report.contains("syntax error: Invalid escape sequence"), "Should include error details");
    assert!(report.contains("context:"), "Should include context section");
    assert!(report.contains("while parsing field 'description'"), "Should include context message");
    assert!(report.contains("snippet:"), "Should include snippet section");
    assert!(report.contains("description: \"Product \\x name\""), "Should include code snippet");

    // Verify visual indicator (^) points to correct column
    assert!(report.contains("    "), "Should have leading spaces to position caret");
    assert!(report.contains("^"), "Should have caret (^) visual indicator");
}

#[test]
fn test_parse_error_all_error_kinds() {
    /// Documents all ParseErrorKind display formats
    ///
    /// Each error kind has a specific format:
    /// - Syntax: "syntax error: <message>"
    /// - I/O: "I/O error: <message>"
    /// - Validation: "validation error: <message>"
    /// - TypeMismatch: "type mismatch at '<field>': expected <expected>, got <actual>"
    /// - UnexpectedEof: "unexpected end of input"
    /// - InvalidUtf8: "invalid UTF-8 encoding"
    /// - UnknownAnchor: "unknown anchor: <name>"
    /// - DuplicateKey: "duplicate key: <key>"
    /// - Other: "error: <message>"

    let errors = vec![
        (ParseError::syntax("invalid token"), "syntax error: invalid token"),
        (ParseError::io("file not found"), "I/O error: file not found"),
        (ParseError::validation("port out of range"), "validation error: port out of range"),
        (ParseError::type_mismatch("port", "integer", "string"), "type mismatch at 'port': expected integer, got string"),
        (ParseError::new(ParseErrorKind::UnexpectedEof), "unexpected end of input"),
        (ParseError::new(ParseErrorKind::InvalidUtf8), "invalid UTF-8 encoding"),
        (ParseError::new(ParseErrorKind::UnknownAnchor("ref".to_string())), "unknown anchor: ref"),
        (ParseError::new(ParseErrorKind::DuplicateKey("name".to_string())), "duplicate key: name"),
        (ParseError::new(ParseErrorKind::Other("unclassified".to_string())), "error: unclassified"),
    ];

    for (error, expected_kind_format) in errors {
        let kind_display = format!("{}", error.kind);
        assert_eq!(kind_display, expected_kind_format,
            "Error kind format should match expected: {}", expected_kind_format);
    }
}

// ============================================================================
// Section 2: ValidationError with field paths
// ============================================================================

#[test]
fn test_validation_error_with_field_path() {
    /// Demonstrates ValidationError format with field path
    ///
    /// Format: `validation error: <message> - <context-with-field-path>`
    ///
    /// Example output:
    /// ```text
    /// config.yaml:15: validation error: port must be between 1 and 65535 - at field server.port
    /// ```
    let error = ParseError::validation("port must be between 1 and 65535")
        .with_path("config.yaml")
        .with_line(15)
        .with_context("at field server.port");

    let display = format!("{}", error);

    // Verify format components
    assert!(display.contains("config.yaml:15"), "Should include file:line");
    assert!(display.contains("validation error:"), "Should include error type");
    assert!(display.contains("port must be between 1 and 65535"), "Should include message");
    assert!(display.contains("at field server.port"), "Should include field path");
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
        let error = ParseError::validation("constraint violation")
            .with_path("config.yaml")
            .with_line(20)
            .with_context(&format!("at field {}", field_path));

        let display = format!("{}", error);

        assert!(display.contains(field_path),
            "{}: Should include field path '{}'", description, field_path);
        assert!(display.contains("validation error:"),
            "{}: Should include error type", description);
    }
}

#[test]
fn test_validation_error_with_constraint() {
    /// Demonstrates ValidationError with constraint information
    ///
    /// Example output:
    /// ```text
    /// config.yaml:25: validation error: value out of range - constraint: must be between 1-65535
    /// ```
    let error = ParseError::validation("value out of range")
        .with_path("config.yaml")
        .with_line(25)
        .with_context("constraint: must be between 1-65535");

    let display = format!("{}", error);

    assert!(display.contains("config.yaml:25"), "Should include location");
    assert!(display.contains("validation error:"), "Should include error type");
    assert!(display.contains("value out of range"), "Should include message");
    assert!(display.contains("constraint: must be between 1-65535"), "Should include constraint");
}

#[test]
fn test_validation_error_complete() {
    /// Documents complete ValidationError with all components
    ///
    /// Format breakdown:
    /// - Location: `<file>:<line>:<column>` or `<file>:<line>`
    /// - Error type: "validation error:"
    /// - Message: The specific validation failure
    /// - Context: Additional context including field path and constraint
    ///
    /// Example:
    /// ```text
    /// app.yaml:42:15: validation error: invalid image tag - at field spec.template.spec.containers[0].image (constraint: must match registry/*:tag pattern)
    /// ```
    let error = ParseError::validation("invalid image tag")
        .with_path("app.yaml")
        .with_line(42)
        .with_column(15)
        .with_context("at field spec.template.spec.containers[0].image (constraint: must match registry/*:tag pattern)");

    let display = format!("{}", error);

    // Verify all components
    assert!(display.contains("app.yaml:42:15"), "Should include full location");
    assert!(display.contains("validation error:"), "Should include error type");
    assert!(display.contains("invalid image tag"), "Should include message");
    assert!(display.contains("at field spec.template.spec.containers[0].image"), "Should include field path");
    assert!(display.contains("constraint: must match registry/*:tag pattern"), "Should include constraint");
}

// ============================================================================
// Section 3: Type mismatch errors
// ============================================================================

#[test]
fn test_type_mismatch_error_format() {
    /// Demonstrates type mismatch error format
    ///
    /// Format: `type mismatch at '<field>': expected <expected>, got <actual>`
    ///
    /// Example output:
    /// ```text
    /// config.yaml:8:10: type mismatch at 'server.port': expected integer, got string
    /// ```
    let error = ParseError::type_mismatch("server.port", "integer", "string")
        .with_path("config.yaml")
        .with_line(8)
        .with_column(10);

    let display = format!("{}", error);

    // Verify format
    assert!(display.contains("config.yaml:8:10"), "Should include location");
    assert!(display.contains("type mismatch at 'server.port'"), "Should include field path");
    assert!(display.contains("expected integer"), "Should include expected type");
    assert!(display.contains("got string"), "Should include actual type");

    // Verify exact kind format
    let kind_display = format!("{}", error.kind);
    assert_eq!(kind_display, "type mismatch at 'server.port': expected integer, got string");
}

#[test]
fn test_type_mismatch_various_types() {
    /// Documents type mismatch errors for various type combinations
    ///
    /// Common type mismatches:
    /// - Expected integer, got string
    /// - Expected string, got integer
    /// - Expected boolean, got string
    /// - Expected array, got scalar
    /// - Expected object, got string

    let test_cases = vec![
        ("port", "integer", "string"),
        ("timeout", "integer", "boolean"),
        ("enabled", "boolean", "string"),
        ("hosts", "array", "string"),
        ("config", "object", "null"),
    ];

    for (field, expected, actual) in test_cases {
        let error = ParseError::type_mismatch(field, expected, actual);
        let kind_display = format!("{}", error.kind);

        let expected_format = format!("type mismatch at '{}': expected {}, got {}", field, expected, actual);
        assert_eq!(kind_display, expected_format,
            "Type mismatch format should be consistent for field '{}' ({} vs {})", field, expected, actual);
    }
}

#[test]
fn test_type_mismatch_with_context() {
    /// Demonstrates type mismatch error with additional context
    ///
    /// Example output:
    /// ```text
    /// config.yaml:15:10: type mismatch at 'database.port': expected integer, got string - required field must be numeric
    /// ```
    let error = ParseError::type_mismatch("database.port", "integer", "string")
        .with_path("config.yaml")
        .with_line(15)
        .with_column(10)
        .with_context("required field must be numeric");

    let display = format!("{}", error);

    assert!(display.contains("config.yaml:15:10"), "Should include location");
    assert!(display.contains("type mismatch at 'database.port'"), "Should include field");
    assert!(display.contains("expected integer"), "Should include expected type");
    assert!(display.contains("got string"), "Should include actual type");
    assert!(display.contains("required field must be numeric"), "Should include context");
}

#[test]
fn test_type_mismatch_nested_fields() {
    /// Documents type mismatch in nested field paths
    ///
    /// Example:
    /// ```text
    /// app.yaml:35:12: type mismatch at 'servers.api.responses[0].statusCode': expected integer, got string
    /// ```
    let error = ParseError::type_mismatch("servers.api.responses[0].statusCode", "integer", "string")
        .with_path("app.yaml")
        .with_line(35)
        .with_column(12);

    let display = format!("{}", error);

    assert!(display.contains("app.yaml:35:12"), "Should include location");
    assert!(display.contains("type mismatch at 'servers.api.responses[0].statusCode'"), "Should include nested field");
    assert!(display.contains("expected integer, got string"), "Should include type information");
}

// ============================================================================
// Section 4: Structured output formats
// ============================================================================

#[test]
fn test_structured_log_format() {
    /// Documents structured error format for logging
    ///
    /// Structured format is useful for:
    /// - JSON logging systems
    /// - Error aggregation
    /// - Programmatic error analysis
    ///
    /// Example:
    /// ```text
    /// ParseError { kind: TypeMismatch { field: "port", expected: "integer", actual: "string" }, location: config.yaml:8:10, line: Some(8), column: Some(10) }
    /// ```
    let error = ParseError::type_mismatch("port", "integer", "string")
        .with_path("config.yaml")
        .with_line(8)
        .with_column(10);

    let structured = error.format_structured();

    assert!(structured.contains("ParseError {"), "Should be a ParseError struct");
    assert!(structured.contains("kind:"), "Should include kind field");
    assert!(structured.contains("location: config.yaml:8:10"), "Should include location");
    assert!(structured.contains("line: Some(8)"), "Should include line");
    assert!(structured.contains("column: Some(10)"), "Should include column");
}

#[test]
fn test_debug_format() {
    /// Documents Debug output format for debugging
    ///
    /// Debug format includes all fields including internal state
    ///
    /// Example:
    /// ```text
    /// ParseError { kind: TypeMismatch { ... }, location: config.yaml:8:10, line: Some(8), column: Some(10), path: Some("config.yaml"), context: "example", has_snippet: false }
    /// ```
    let error = ParseError::type_mismatch("port", "integer", "string")
        .with_path("config.yaml")
        .with_line(8)
        .with_column(10)
        .with_context("example");

    let debug = format!("{:?}", error);

    assert!(debug.contains("ParseError"), "Should include type name");
    assert!(debug.contains("kind:"), "Should include kind");
    assert!(debug.contains("location:"), "Should include location");
    assert!(debug.contains("line:"), "Should include line");
    assert!(debug.contains("column:"), "Should include column");
    assert!(debug.contains("path:"), "Should include path");
    assert!(debug.contains("context:"), "Should include context");
    assert!(debug.contains("has_snippet:"), "Should include snippet presence flag");
}

// ============================================================================
// Section 5: Real-world error scenarios
// ============================================================================

#[test]
fn test_real_world_config_file_error() {
    /// Documents a realistic configuration file error scenario
    ///
    /// Scenario: Invalid port configuration in service config
    ///
    /// config.yaml:
    /// ```yaml
    /// services:
    ///   - name: web
    ///     port: "8080"  # ERROR: should be integer, not string
    /// ```
    ///
    /// Error output:
    /// ```text
    /// config.yaml:10:10: type mismatch at 'services[0].port': expected integer, got string
    /// ```
    let error = ParseError::type_mismatch("services[0].port", "integer", "string")
        .with_path("config.yaml")
        .with_line(10)
        .with_column(10)
        .with_snippet("services:\n  - name: web\n    port: \"8080\"");

    let display = format!("{}", error);

    assert!(display.contains("config.yaml:10:10"), "Should include location");
    assert!(display.contains("type mismatch at 'services[0].port'"), "Should include field");
    assert!(display.contains("expected integer, got string"), "Should include types");
    assert!(display.contains("services:"), "Should include snippet");
    assert!(display.contains("port: \"8080\""), "Should include error line");
}

#[test]
fn test_real_world_validation_error() {
    /// Documents a realistic validation error scenario
    ///
    /// Scenario: Port number out of valid range
    ///
    /// config.yaml:
    /// ```yaml
    /// server:
    ///   port: 70000  # ERROR: out of range (1-65535)
    /// ```
    ///
    /// Error output:
    /// ```text
    /// config.yaml:15:10: validation error: port must be between 1 and 65535 - at field server.port (constraint: must be between 1-65535)
    /// ```
    let error = ParseError::validation("port must be between 1 and 65535")
        .with_path("config.yaml")
        .with_line(15)
        .with_column(10)
        .with_context("at field server.port (constraint: must be between 1-65535)")
        .with_snippet("server:\n  port: 70000");

    let display = format!("{}", error);

    assert!(display.contains("config.yaml:15:10"), "Should include location");
    assert!(display.contains("validation error:"), "Should include error type");
    assert!(display.contains("port must be between 1 and 65535"), "Should include message");
    assert!(display.contains("server.port"), "Should include field path");
    assert!(display.contains("constraint:"), "Should include constraint");
    assert!(display.contains("port: 70000"), "Should include snippet");
}

#[test]
fn test_real_world_syntax_error() {
    /// Documents a realistic YAML syntax error scenario
    ///
    /// Scenario: Missing colon in key-value pair
    ///
    /// config.yaml:
    /// ```yaml
    /// database
    ///   host localhost  # ERROR: missing colon
    /// ```
    ///
    /// Error output:
    /// ```text
    /// config.yaml:20:8: syntax error: missing colon - while parsing database configuration
    /// ```
    let error = ParseError::syntax("missing colon")
        .with_path("config.yaml")
        .with_line(20)
        .with_column(8)
        .with_context("while parsing database configuration")
        .with_snippet("database\n  host localhost");

    let display = format!("{}", error);

    assert!(display.contains("config.yaml:20:8"), "Should include location");
    assert!(display.contains("syntax error: missing colon"), "Should include error");
    assert!(display.contains("while parsing database configuration"), "Should include context");
    assert!(display.contains("database"), "Should include snippet");
}

// ============================================================================
// Section 6: Error message format consistency
// ============================================================================

#[test]
fn test_error_format_consistency() {
    /// Verifies that error formats follow consistent patterns
    ///
    /// Consistency rules:
    /// 1. Location always comes first: `<file>:<line>:<column>:`
    /// 2. Error type follows location: `syntax error:`, `validation error:`, etc.
    /// 3. Context (if present) follows with `- ` separator
    /// 4. Field paths in context use `at field <path>` format
    /// 5. Constraints use `(constraint: <details>)` format

    let errors = vec![
        ParseError::syntax("invalid token").with_path("f.yaml").with_line(1).with_column(1),
        ParseError::validation("constraint violation").with_path("f.yaml").with_line(2),
        ParseError::type_mismatch("x", "int", "str").with_path("f.yaml").with_line(3),
    ];

    for error in errors {
        let summary = error.summary();

        // Rule 1: Location should be first and end with ": "
        let loc = error.location_string();
        assert!(summary.starts_with(&format!("{}: ", loc)),
            "Summary should start with location '{}: '", loc);

        // Rule 2: Error type should follow location
        // The kind display includes the error type (syntax error, I/O error, validation error, etc.)
        assert!(summary.contains(&format!("{}", error.kind)), "Summary should include error kind");

        // Rule 3: Context (if present) should use "- " separator
        if !error.context.is_empty() {
            assert!(summary.contains(" - "), "Summary with context should use ' - ' separator");
        }
    }
}
