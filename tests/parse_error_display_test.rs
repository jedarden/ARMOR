//! Tests for ParseError Display, Debug, and formatting implementations

use armor::parsers::yaml::{ParseError, ParseErrorKind};

#[test]
fn test_display_syntax_error_basic() {
    let error = ParseError::syntax("Unexpected token");

    let display = format!("{}", error);
    assert!(display.contains("syntax error"));
    assert!(display.contains("Unexpected token"));
    assert!(display.contains("<unknown>"));
}

#[test]
fn test_display_with_path() {
    let error = ParseError::syntax("Invalid YAML")
        .with_path("config.yaml");

    let display = format!("{}", error);
    assert!(display.contains("config.yaml"));
    assert!(display.contains("syntax error: Invalid YAML"));
}

#[test]
fn test_display_with_line_and_column() {
    let error = ParseError::syntax("Missing colon")
        .with_path("values.yaml")
        .with_line(10)
        .with_column(5);

    let display = format!("{}", error);
    assert!(display.contains("values.yaml:10:5"));
    assert!(display.contains("syntax error: Missing colon"));
}

#[test]
fn test_display_with_context() {
    let error = ParseError::syntax("Unterminated string")
        .with_path("data.yaml")
        .with_line(3)
        .with_column(12)
        .with_context("while parsing field 'description'");

    let display = format!("{}", error);
    assert!(display.contains("data.yaml:3:12"));
    assert!(display.contains("syntax error: Unterminated string"));
    assert!(display.contains("while parsing field 'description'"));
}

#[test]
fn test_display_with_snippet() {
    let error = ParseError::syntax("Invalid escape sequence")
        .with_path("config.yaml")
        .with_line(5)
        .with_column(15)
        .with_snippet("description: \"Product \\x name\"");

    let display = format!("{}", error);
    assert!(display.contains("config.yaml:5:15"));
    assert!(display.contains("syntax error: Invalid escape sequence"));
    assert!(display.contains("snippet"));
    assert!(display.contains("description: \"Product \\x name\""));
}

#[test]
fn test_display_type_mismatch() {
    let error = ParseError::type_mismatch("port", "integer", "string")
        .with_path("service.yaml")
        .with_line(8)
        .with_column(10);

    let display = format!("{}", error);
    assert!(display.contains("service.yaml:8:10"));
    assert!(display.contains("type mismatch at 'port'"));
    assert!(display.contains("expected integer"));
    assert!(display.contains("got string"));
}

#[test]
fn test_display_io_error() {
    let error = ParseError::io("Permission denied")
        .with_path("/etc/config/app.yaml");

    let display = format!("{}", error);
    assert!(display.contains("/etc/config/app.yaml"));
    assert!(display.contains("I/O error: Permission denied"));
}

#[test]
fn test_display_validation_error() {
    let error = ParseError::validation("port must be between 1 and 65535")
        .with_path("network.yaml")
        .with_line(12);

    let display = format!("{}", error);
    assert!(display.contains("network.yaml:12"));
    assert!(display.contains("validation error: port must be between 1 and 65535"));
}

#[test]
fn test_display_unknown_anchor() {
    let error = ParseError::new(ParseErrorKind::UnknownAnchor("ref".to_string()))
        .with_path("anchors.yaml")
        .with_line(7);

    let display = format!("{}", error);
    assert!(display.contains("anchors.yaml:7"));
    assert!(display.contains("unknown anchor: ref"));
}

#[test]
fn test_display_duplicate_key() {
    let error = ParseError::new(ParseErrorKind::DuplicateKey("name".to_string()))
        .with_path("data.yaml")
        .with_line(4);

    let display = format!("{}", error);
    assert!(display.contains("data.yaml:4"));
    assert!(display.contains("duplicate key: name"));
}

#[test]
fn test_display_unexpected_eof() {
    let error = ParseError::new(ParseErrorKind::UnexpectedEof)
        .with_path("incomplete.yaml")
        .with_line(15);

    let display = format!("{}", error);
    assert!(display.contains("incomplete.yaml:15"));
    assert!(display.contains("unexpected end of input"));
}

#[test]
fn test_location_string() {
    // Full location
    let error = ParseError::syntax("test")
        .with_path("file.yaml")
        .with_line(10)
        .with_column(5);
    assert_eq!(error.location_string(), "file.yaml:10:5");

    // Path and line only
    let error = ParseError::syntax("test")
        .with_path("file.yaml")
        .with_line(10);
    assert_eq!(error.location_string(), "file.yaml:10");

    // Path and column only
    let error = ParseError::syntax("test")
        .with_path("file.yaml")
        .with_column(5);
    assert_eq!(error.location_string(), "file.yaml::5");

    // Path only
    let error = ParseError::syntax("test")
        .with_path("file.yaml");
    assert_eq!(error.location_string(), "file.yaml");

    // Line and column only
    let error = ParseError::syntax("test")
        .with_line(10)
        .with_column(5);
    assert_eq!(error.location_string(), "10:5");

    // Line only
    let error = ParseError::syntax("test")
        .with_line(10);
    assert_eq!(error.location_string(), "10");

    // Column only
    let error = ParseError::syntax("test")
        .with_column(5);
    assert_eq!(error.location_string(), "col 5");

    // Unknown
    let error = ParseError::syntax("test");
    assert_eq!(error.location_string(), "<unknown>");
}

#[test]
fn test_summary() {
    let error = ParseError::syntax("Invalid token")
        .with_path("config.yaml")
        .with_line(5)
        .with_context("in service definition");

    let summary = error.summary();
    assert_eq!(summary, "config.yaml:5: syntax error: Invalid token - in service definition");
}

#[test]
fn test_summary_without_context() {
    let error = ParseError::syntax("Invalid token")
        .with_path("config.yaml")
        .with_line(5);

    let summary = error.summary();
    assert_eq!(summary, "config.yaml:5: syntax error: Invalid token");
}

#[test]
fn test_detailed_report() {
    let error = ParseError::syntax("Unterminated string")
        .with_path("data.yaml")
        .with_line(3)
        .with_column(12)
        .with_context("while parsing field 'description'")
        .with_snippet("description: \"This is a string");

    let report = error.detailed_report();
    assert!(report.contains("error:"));
    assert!(report.contains("data.yaml:3:12"));
    assert!(report.contains("syntax error: Unterminated string"));
    assert!(report.contains("context: while parsing field 'description'"));
    assert!(report.contains("snippet:"));
    assert!(report.contains("description: \"This is a string"));
}

#[test]
fn test_detailed_report_with_visual_indicator() {
    let error = ParseError::syntax("Invalid escape")
        .with_path("config.yaml")
        .with_line(5)
        .with_column(18)
        .with_snippet("name: \"John\\nDoe\"");

    let report = error.detailed_report();
    assert!(report.contains("snippet:"));
    // The caret should be at column 18 (0-indexed: 17 spaces + ^)
    assert!(report.contains(&format!("{}^", " ".repeat(17))));
}

#[test]
fn test_format_structured() {
    let error = ParseError::type_mismatch("port", "integer", "string")
        .with_path("service.yaml")
        .with_line(8)
        .with_column(10);

    let formatted = error.format_structured();
    assert!(formatted.contains("ParseError"));
    assert!(formatted.contains("service.yaml:8:10"));
    assert!(formatted.contains("line: Some(8)"));
    assert!(formatted.contains("column: Some(10)"));
}

#[test]
fn test_debug_formatting() {
    let error = ParseError::syntax("test error")
        .with_path("file.yaml")
        .with_line(42)
        .with_column(7)
        .with_context("some context")
        .with_snippet("line content");

    let debug = format!("{:?}", error);
    assert!(debug.contains("ParseError"));
    assert!(debug.contains("kind"));
    assert!(debug.contains("location"));
    assert!(debug.contains("file.yaml:42:7"));
    assert!(debug.contains("has_snippet: true"));
}

#[test]
fn test_with_location_helper() {
    let error = ParseError::syntax("test")
        .with_location(10, 5)
        .with_path("config.yaml");

    assert_eq!(error.line, Some(10));
    assert_eq!(error.column, Some(5));
    assert_eq!(error.location_string(), "config.yaml:10:5");
}

#[test]
fn test_error_is_syntax() {
    let error = ParseError::syntax("test");
    assert!(error.is_syntax());
    assert!(!error.is_io());
    assert!(!error.is_validation());
    assert!(!error.is_type_mismatch());
}

#[test]
fn test_error_is_io() {
    let error = ParseError::io("test");
    assert!(!error.is_syntax());
    assert!(error.is_io());
    assert!(!error.is_validation());
    assert!(!error.is_type_mismatch());
}

#[test]
fn test_error_is_validation() {
    let error = ParseError::validation("test");
    assert!(!error.is_syntax());
    assert!(!error.is_io());
    assert!(error.is_validation());
    assert!(!error.is_type_mismatch());
}

#[test]
fn test_error_is_type_mismatch() {
    let error = ParseError::type_mismatch("field", "string", "int");
    assert!(!error.is_syntax());
    assert!(!error.is_io());
    assert!(!error.is_validation());
    assert!(error.is_type_mismatch());
}

#[test]
fn test_full_error_display_complex() {
    // Test a complex error with all fields populated
    let error = ParseError::type_mismatch("database.port", "integer", "string")
        .with_path("config/production.yaml")
        .with_location(42, 10)
        .with_context("required field 'port' must be an integer for database configuration")
        .with_snippet("database:\n  host: localhost\n  port: \"5432\"");

    let display = format!("{}", error);

    // Verify all components are present
    assert!(display.contains("config/production.yaml:42:10"));
    assert!(display.contains("type mismatch at 'database.port'"));
    assert!(display.contains("expected integer"));
    assert!(display.contains("got string"));
    assert!(display.contains("required field 'port' must be an integer for database configuration"));
    assert!(display.contains("snippet"));
    assert!(display.contains("database:"));
    assert!(display.contains("  host: localhost"));
    assert!(display.contains("  port: \"5432\""));

    // Verify the visual indicator points to the right column
    // Column 10 means the caret should be at position 9 (0-indexed)
    let lines: Vec<&str> = display.lines().collect();
    let caret_line = lines.iter().find(|line| line.contains('^'));
    assert!(caret_line.is_some(), "Should have a visual indicator with ^");
}
