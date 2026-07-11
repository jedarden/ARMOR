//! Integration tests for ParseError
//!
//! This test suite covers:
//! - Error creation and formatting workflows
//! - Error propagation through functions
//! - Context building patterns
//! - Multi-layer error scenarios
//! - Integration with Result type

use armor::parsers::yaml::{ParseError, ParseErrorKind, Result};
use std::io;

// ===========================================================================
// Error Creation Workflows
// ===========================================================================

#[test]
fn test_complete_error_creation_workflow() {
    // Simulate a typical error found during YAML parsing
    let error = ParseError::syntax("invalid mapping syntax")
        .with_path("config/services.yaml")
        .with_location(15, 8)
        .with_context("while parsing service definitions")
        .with_snippet("services:\n  - name: web\n    port: abc");

    // Verify all components are properly set
    assert!(error.is_syntax());
    assert_eq!(error.line, Some(15));
    assert_eq!(error.column, Some(8));
    assert_eq!(error.path, Some("config/services.yaml".to_string()));
    assert_eq!(error.context, "while parsing service definitions");
    assert_eq!(error.snippet, Some("services:\n  - name: web\n    port: abc".to_string()));

    // Verify formatting produces useful output
    let display = format!("{}", error);
    assert!(display.contains("config/services.yaml:15:8"));
    assert!(display.contains("syntax error: invalid mapping syntax"));
    assert!(display.contains("while parsing service definitions"));
    assert!(display.contains("services:"));
}

#[test]
fn test_error_workflow_from_validation() {
    // Simulate validation error with context
    let port = 70000; // Invalid port number
    let error = ParseError::validation(format!("port must be between 1 and 65535, got {}", port))
        .with_path("config/database.yaml")
        .with_location(25, 10)
        .with_context("while validating database configuration");

    assert!(error.is_validation());
    assert!(error.summary().contains("validation error"));
    assert!(error.summary().contains("port must be between 1 and 65535"));
    assert!(error.summary().contains("while validating database configuration"));
}

#[test]
fn test_error_workflow_type_mismatch_nested() {
    // Simulate type mismatch in nested structure
    let error = ParseError::type_mismatch("database.connection.timeout", "integer", "boolean")
        .with_path("production.yaml")
        .with_location(42, 18)
        .with_context("while parsing database connection settings")
        .with_snippet("database:\n  connection:\n    timeout: true");

    assert!(error.is_type_mismatch());
    let display = format!("{}", error);
    assert!(display.contains("database.connection.timeout"));
    assert!(display.contains("expected integer"));
    assert!(display.contains("got boolean"));
}

// ===========================================================================
// Error Propagation Patterns
// ===========================================================================

fn read_file_content(path: &str) -> Result<String> {
    // Simulate file reading
    Err(ParseError::io("file not found")
        .with_path(path)
        .with_context(format!("while reading file: {}", path)))
}

fn parse_yaml_content(content: &str) -> Result<()> {
    // Simulate YAML parsing
    Err(ParseError::syntax("invalid YAML structure")
        .with_line(1)
        .with_column(1)
        .with_context("while parsing YAML document"))
}

fn validate_config(value: &str) -> Result<()> {
    // Simulate validation
    Err(ParseError::validation("value must be positive")
        .with_line(10)
        .with_context("while validating configuration"))
}

#[test]
fn test_error_propagation_io_to_parse_error() {
    let result = read_file_content("config.yaml");

    match result {
        Err(error) => {
            assert!(error.is_io());
            assert_eq!(error.path, Some("config.yaml".to_string()));
            assert!(error.context.contains("while reading file"));
        }
        Ok(_) => panic!("Expected error"),
    }
}

#[test]
fn test_error_propagation_syntax_to_parse_error() {
    let content = "invalid: yaml: content:";
    let result = parse_yaml_content(content);

    match result {
        Err(error) => {
            assert!(error.is_syntax());
            assert_eq!(error.line, Some(1));
            assert!(error.context.contains("while parsing YAML"));
        }
        Ok(_) => panic!("Expected error"),
    }
}

#[test]
fn test_error_propagation_validation_to_parse_error() {
    let result = validate_config("-5");

    match result {
        Err(error) => {
            assert!(error.is_validation());
            assert_eq!(error.line, Some(10));
            assert!(error.context.contains("while validating"));
        }
        Ok(_) => panic!("Expected error"),
    }
}

// ===========================================================================
// Context Building Patterns
// ===========================================================================

fn parse_service_config(value: &str) -> Result<()> {
    // Multi-layer error with nested context
    Err(ParseError::type_mismatch("port", "integer", "string")
        .with_line(5)
        .with_column(10)
        .with_context("while parsing service configuration")
        .with_snippet("service:\n  name: web\n  port: \"80\""))
}

fn parse_database_config(value: &str) -> Result<()> {
    // Context with file information
    Err(ParseError::validation("host cannot be empty")
        .with_path("config/database.yaml")
        .with_line(12)
        .with_context("while parsing database connection string")
        .with_snippet("database:\n  host: \"\""))
}

#[test]
fn test_context_building_pattern_service() {
    let result = parse_service_config("test");

    match result {
        Err(error) => {
            assert!(error.is_type_mismatch());
            assert!(error.context.contains("while parsing service configuration"));
            assert!(error.snippet.is_some());
            let snippet = error.snippet.unwrap();
            assert!(snippet.contains("port: \"80\""));
        }
        Ok(_) => panic!("Expected error"),
    }
}

#[test]
fn test_context_building_pattern_database() {
    let result = parse_database_config("test");

    match result {
        Err(error) => {
            assert!(error.is_validation());
            assert_eq!(error.path, Some("config/database.yaml".to_string()));
            assert!(error.context.contains("while parsing database connection string"));
            assert!(error.summary().contains("host cannot be empty"));
        }
        Ok(_) => panic!("Expected error"),
    }
}

// ===========================================================================
// Multi-Layer Error Scenarios
// ===========================================================================

fn process_config_file(path: &str) -> Result<()> {
    // Layer 1: File reading
    let content = read_file_content(path)?;

    // Layer 2: YAML parsing
    parse_yaml_content(&content)?;

    // Layer 3: Validation
    validate_config(&content)?;

    Ok(())
}

#[test]
fn test_multi_layer_error_propagation() {
    let result = process_config_file("nonexistent.yaml");

    match result {
        Err(error) => {
            // Should fail at first layer (file reading)
            assert!(error.is_io());
            assert!(error.context.contains("while reading file"));
        }
        Ok(_) => panic!("Expected error at file reading layer"),
    }
}

#[test]
fn test_error_accumulation_workflow() {
    // Simulate multiple errors in a single document
    let errors = vec![
        ParseError::syntax("invalid indentation")
            .with_line(5)
            .with_column(4)
            .with_path("config.yaml"),
        ParseError::type_mismatch("port", "integer", "string")
            .with_line(10)
            .with_column(12)
            .with_path("config.yaml"),
        ParseError::validation("port out of range")
            .with_line(10)
            .with_column(12)
            .with_path("config.yaml"),
    ];

    assert_eq!(errors.len(), 3);

    // Verify each error
    assert!(errors[0].is_syntax());
    assert!(errors[1].is_type_mismatch());
    assert!(errors[2].is_validation());

    // Verify all have the same path
    assert_eq!(errors[0].path, Some("config.yaml".to_string()));
    assert_eq!(errors[1].path, Some("config.yaml".to_string()));
    assert_eq!(errors[2].path, Some("config.yaml".to_string()));
}

// ===========================================================================
// Error Formatting Integration
// ===========================================================================

#[test]
fn test_error_report_generation_workflow() {
    let error = ParseError::syntax("invalid escape sequence")
        .with_path("config/app.yaml")
        .with_location(8, 25)
        .with_context("while parsing application name")
        .with_snippet("app:\n  name: \"My\\nApp\"");

    // Generate different report formats
    let summary = error.summary();
    let detailed = error.detailed_report();
    let structured = error.format_structured();
    let display = format!("{}", error);
    let debug = format!("{:?}", error);

    // Verify each format contains expected information
    assert!(summary.contains("config/app.yaml:8:25"));
    assert!(summary.contains("syntax error: invalid escape sequence"));
    assert!(summary.contains("while parsing application name"));

    assert!(detailed.contains("error:"));
    assert!(detailed.contains("context:"));
    assert!(detailed.contains("snippet:"));
    assert!(detailed.contains("My\\nApp"));

    assert!(structured.contains("ParseError"));
    assert!(structured.contains("config/app.yaml:8:25"));
    assert!(structured.contains("line: Some(8)"));

    assert!(display.contains("config/app.yaml:8:25"));
    assert!(display.contains("syntax error"));

    assert!(debug.contains("ParseError"));
    assert!(debug.contains("kind"));
}

#[test]
fn test_error_logging_workflow() {
    let errors = vec![
        ParseError::io("permission denied").with_path("/etc/config.yaml"),
        ParseError::syntax("invalid YAML").with_path("user.yaml").with_line(10),
        ParseError::validation("invalid port").with_path("service.yaml").with_line(5),
    ];

    // Simulate logging each error
    let log_entries: Vec<String> = errors
        .iter()
        .map(|e| e.summary())
        .collect();

    assert_eq!(log_entries.len(), 3);
    assert!(log_entries[0].contains("I/O error"));
    assert!(log_entries[1].contains("syntax error"));
    assert!(log_entries[2].contains("validation error"));
}

// ===========================================================================
// Result Type Integration
// ===========================================================================

fn returns_result_ok() -> Result<String> {
    Ok("success".to_string())
}

fn returns_result_err() -> Result<String> {
    Err(ParseError::validation("failed validation"))
}

#[test]
fn test_result_type_integration_ok() {
    let result = returns_result_ok();
    assert!(result.is_ok());
    assert_eq!(result.unwrap(), "success");
}

#[test]
fn test_result_type_integration_err() {
    let result = returns_result_err();
    assert!(result.is_err());

    match result {
        Ok(_) => panic!("Expected error"),
        Err(error) => {
            assert!(error.is_validation());
            // The error message is in the kind, not context
            assert!(error.to_string().contains("failed validation"));
        }
    }
}

#[test]
fn test_result_type_with_question_operator() {
    fn function_with_question() -> Result<String> {
        let value = returns_result_ok()?;
        Ok(value)
    }

    let result = function_with_question();
    assert!(result.is_ok());
    assert_eq!(result.unwrap(), "success");
}

#[test]
fn test_result_type_with_question_operator_error() {
    fn function_with_question() -> Result<String> {
        let value = returns_result_err()?;
        Ok(value)
    }

    let result = function_with_question();
    assert!(result.is_err());

    match result {
        Err(error) => {
            assert!(error.is_validation());
        }
        Ok(_) => panic!("Expected error"),
    }
}

// ===========================================================================
// Real-World Error Scenarios
// ===========================================================================

#[test]
fn test_real_world_scenario_config_file_not_found() {
    let error = ParseError::io("No such file or directory")
        .with_path("config/production.yaml")
        .with_context("while loading configuration file");

    let display = format!("{}", error);
    assert!(display.contains("config/production.yaml"));
    assert!(display.contains("I/O error"));
    assert!(display.contains("while loading configuration file"));
}

#[test]
fn test_real_world_scenario_invalid_yaml_syntax() {
    let error = ParseError::syntax("found unexpected ':' while parsing a flow sequence")
        .with_path("values.yaml")
        .with_location(15, 12)
        .with_context("while parsing Helm values file")
        .with_snippet("services:\n  - [web, database]: invalid");

    let report = error.detailed_report();
    assert!(report.contains("values.yaml:15:12"));
    assert!(report.contains("while parsing Helm values file"));
    assert!(report.contains("[web, database]: invalid"));
}

#[test]
fn test_real_world_scenario_database_config_validation() {
    let error = ParseError::type_mismatch("database.port", "integer", "string")
        .with_path("docker-compose.yaml")
        .with_location(42, 16)
        .with_context("while parsing database environment variables")
        .with_snippet("environment:\n  - DB_PORT=\"5432\"");

    let display = format!("{}", error);
    assert!(display.contains("docker-compose.yaml:42:16"));
    assert!(display.contains("database.port"));
    assert!(display.contains("expected integer"));
    assert!(display.contains("got string"));
    assert!(display.contains("DB_PORT=\"5432\""));
}

#[test]
fn test_real_world_scenario_duplicate_key() {
    let error = ParseError::new(ParseErrorKind::DuplicateKey("name".to_string()))
        .with_path("config/services.yaml")
        .with_line(25)
        .with_context("duplicate service name found")
        .with_snippet("services:\n  - name: web\n    port: 8080\n  - name: web\n    port: 8081");

    let summary = error.summary();
    assert!(summary.contains("config/services.yaml:25"));
    assert!(summary.contains("duplicate key: name"));
    assert!(summary.contains("duplicate service name found"));
}

#[test]
fn test_real_world_scenario_unexpected_eof() {
    let error = ParseError::new(ParseErrorKind::UnexpectedEof)
        .with_path("config/incomplete.yaml")
        .with_line(100)
        .with_context("file ended prematurely while parsing mapping")
        .with_snippet("database:\n  host: localhost\n  port:");

    let report = error.detailed_report();
    assert!(report.contains("config/incomplete.yaml:100"));
    assert!(report.contains("unexpected end of input"));
    assert!(report.contains("file ended prematurely"));
}

#[test]
fn test_real_world_scenario_invalid_utf8() {
    let error = ParseError::new(ParseErrorKind::InvalidUtf8)
        .with_path("config/mixed-encoding.yaml")
        .with_line(50)
        .with_column(8)
        .with_context("file contains invalid UTF-8 byte sequence");

    let summary = error.summary();
    assert!(summary.contains("config/mixed-encoding.yaml:50:8"));
    assert!(summary.contains("invalid UTF-8 encoding"));
    assert!(summary.contains("invalid UTF-8 byte sequence"));
}

#[test]
fn test_real_world_scenario_unknown_anchor() {
    let error = ParseError::new(ParseErrorKind::UnknownAnchor("db_config".to_string()))
        .with_path("config/anchors.yaml")
        .with_line(30)
        .with_column(15)
        .with_context("anchor reference not defined")
        .with_snippet("production:\n  database: *db_config");

    let display = format!("{}", error);
    assert!(display.contains("config/anchors.yaml:30:15"));
    assert!(display.contains("unknown anchor: db_config"));
    assert!(display.contains("anchor reference not defined"));
}

// ===========================================================================
// Error Conversion Workflow
// ===========================================================================

#[test]
fn test_error_workflow_from_io_error() {
    // Simulate converting std::io::Error to ParseError
    let io_error = io::Error::new(io::ErrorKind::NotFound, "config.yaml not found");

    // In real code, this would use the From trait implementation
    let parse_error = ParseError::io(io_error.to_string())
        .with_path("config.yaml")
        .with_context("while reading configuration file");

    assert!(parse_error.is_io());
    assert!(parse_error.context.contains("while reading configuration file"));
}

#[test]
fn test_error_workflow_with_custom_conversion() {
    // Simulate a custom error type conversion
    enum CustomError {
        InvalidConfig(String),
        MissingField(String),
    }

    let custom_error = CustomError::InvalidConfig("port value is invalid".to_string());

    let parse_error = match custom_error {
        CustomError::InvalidConfig(msg) => {
            ParseError::validation(msg)
                .with_path("config.yaml")
                .with_line(10)
                .with_context("while validating custom configuration")
        }
        CustomError::MissingField(field) => {
            ParseError::validation(format!("missing required field: {}", field))
                .with_path("config.yaml")
                .with_context("while checking required fields")
        }
    };

    assert!(parse_error.is_validation());
    assert!(parse_error.context.contains("while validating custom configuration"));
}

// ===========================================================================
// Complex Error Scenarios
// ===========================================================================

#[test]
fn test_complex_error_multiple_issues() {
    // Simulate a file with multiple related errors
    let content = r#"services:
  - name: web
    port: "8080"
  - name: database
    port: abc
    host: localhost"#;

    let mut errors = Vec::new();

    // Error 1: Type mismatch for first service port
    errors.push(
        ParseError::type_mismatch("services[0].port", "integer", "string")
            .with_path("config.yaml")
            .with_line(3)
            .with_column(10)
            .with_snippet(content)
    );

    // Error 2: Type mismatch for second service port
    errors.push(
        ParseError::type_mismatch("services[1].port", "integer", "string")
            .with_path("config.yaml")
            .with_line(5)
            .with_column(10)
            .with_snippet(content)
    );

    assert_eq!(errors.len(), 2);
    assert!(errors.iter().all(|e| e.is_type_mismatch()));
}

#[test]
fn test_complex_error_with_chained_context() {
    // Simulate error passing through multiple layers
    let base_error = ParseError::syntax("invalid escape sequence");

    let level1 = base_error
        .with_path("config.yaml")
        .with_line(10)
        .with_column(15);

    let level2 = level1
        .with_context("while parsing string value");

    let level3 = level2
        .with_snippet("value: \"test\\nstring\"")
        .with_context("while parsing service configuration");

    // All layers preserved
    assert_eq!(level3.path, Some("config.yaml".to_string()));
    assert_eq!(level3.line, Some(10));
    assert_eq!(level3.column, Some(15));
    assert_eq!(level3.snippet, Some("value: \"test\\nstring\"".to_string()));
    assert!(level3.context.contains("while parsing service configuration"));
}

#[test]
fn test_error_workflow_error_recovery() {
    // Simulate continuing after an error
    let results: Vec<Result<String>> = vec![
        Err(ParseError::syntax("error 1").with_line(1)),
        Err(ParseError::validation("error 2").with_line(2)),
        Err(ParseError::io("error 3").with_line(3)),
    ];

    let errors: Vec<ParseError> = results
        .into_iter()
        .filter_map(|r| r.err())
        .collect();

    assert_eq!(errors.len(), 3);
    assert!(errors[0].is_syntax());
    assert!(errors[1].is_validation());
    assert!(errors[2].is_io());

    // All errors have their line numbers
    assert_eq!(errors[0].line, Some(1));
    assert_eq!(errors[1].line, Some(2));
    assert_eq!(errors[2].line, Some(3));
}
