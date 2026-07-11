//! Integration tests for ParseError error propagation
//!
//! This test suite demonstrates error propagation through a call stack using
//! the Result<T, ParseError> pattern and the ? operator.

use armor::parsers::yaml::{ParseError, Result};
use std::io;
use std::path::Path;

/// Simulates reading a configuration file
fn read_config_file(path: &Path) -> Result<String> {
    // This will fail since the file doesn't exist
    // The io::Error automatically converts to ParseError via the From impl
    let content = std::fs::read_to_string(path)?;
    Ok(content)
}

/// Simulates parsing YAML content
fn parse_yaml_content(content: &str) -> Result<serde_yaml::Value> {
    // This will fail if the YAML is invalid
    // serde_yaml::Error automatically converts to ParseError via the From impl
    let value = serde_yaml::from_str(content)?;
    Ok(value)
}

/// Simulates validating a specific field from the parsed YAML
fn extract_database_config(value: &serde_yaml::Value) -> Result<DatabaseConfig> {
    // Extract database configuration with context
    let db_section = value.get("database")
        .ok_or_else(|| ParseError::validation("missing 'database' section")
            .with_context("while extracting database configuration"))?;

    let host = db_section.get("host")
        .and_then(|v| v.as_str())
        .ok_or_else(|| ParseError::type_mismatch("database.host", "string", "null")
            .with_context("while extracting database host"))?;

    let port = db_section.get("port")
        .and_then(|v| v.as_i64())
        .ok_or_else(|| ParseError::type_mismatch("database.port", "integer", "null")
            .with_context("while extracting database port"))?;

    Ok(DatabaseConfig {
        host: host.to_string(),
        port: port as u16,
    })
}

/// Simulates a complete configuration loading workflow
fn load_config_from_file(path: &Path) -> Result<DatabaseConfig> {
    // This demonstrates error propagation through a call stack:
    // 1. read_config_file can fail with io::Error → ParseError
    // 2. parse_yaml_content can fail with serde_yaml::Error → ParseError
    // 3. extract_database_config can fail with custom ParseError

    let content = read_config_file(path)?;
    let yaml = parse_yaml_content(&content)?;
    let config = extract_database_config(&yaml)?;

    Ok(config)
}

#[derive(Debug)]
struct DatabaseConfig {
    host: String,
    port: u16,
}

#[test]
fn test_result_parse_error_compiles() {
    // Test that Result<T, ParseError> compiles and works correctly
    fn returns_parse_error() -> Result<String> {
        Ok("success".to_string())
    }

    let result: Result<String> = returns_parse_error();
    assert!(result.is_ok());
    assert_eq!(result.unwrap(), "success");
}

#[test]
fn test_from_io_error() {
    // Test that std::io::Error converts to ParseError
    let io_err = io::Error::new(io::ErrorKind::NotFound, "file not found");
    let parse_err: ParseError = io_err.into();

    assert!(parse_err.is_io());
    assert!(parse_err.to_string().contains("file not found"));
}

#[test]
fn test_from_serde_yaml_error() {
    // Test that serde_yaml::Error converts to ParseError
    let invalid_yaml = "not: valid: yaml: [";
    let serde_err = serde_yaml::from_str::<serde_yaml::Value>(invalid_yaml).unwrap_err();
    let parse_err: ParseError = serde_err.into();

    // Should classify as an error (syntax or other depending on the error message)
    // Just verify it converted successfully and contains error info
    let err_str = parse_err.to_string();
    assert!(!err_str.is_empty(), "Error message should not be empty");

    // Verify it's one of the expected error kinds
    assert!(parse_err.is_syntax() || parse_err.to_string().contains("error"),
            "Error should be syntax or contain 'error': {}", err_str);
}

#[test]
fn test_from_utf8_error() {
    // Test that std::str::Utf8Error converts to ParseError
    let invalid_utf8 = b"\xFF\xFE";
    let utf8_err = std::str::from_utf8(invalid_utf8).unwrap_err();
    let parse_err: ParseError = utf8_err.into();

    assert!(parse_err.to_string().contains("invalid UTF-8"));
    assert!(!parse_err.context.is_empty());
}

#[test]
fn test_from_from_utf8_error() {
    // Test that std::string::FromUtf8Error converts to ParseError
    let invalid_bytes = vec![0xFF, 0xFE, 0xFD];
    let from_utf8_err = String::from_utf8(invalid_bytes).unwrap_err();
    let parse_err: ParseError = from_utf8_err.into();

    assert!(parse_err.to_string().contains("invalid UTF-8"));
    assert!(!parse_err.context.is_empty());
}

#[test]
fn test_error_propagation_with_question_mark() {
    // Test error propagation through call stack with ? operator
    let nonexistent_path = Path::new("/nonexistent/config.yaml");
    let result = load_config_from_file(nonexistent_path);

    assert!(result.is_err());
    let err = result.unwrap_err();
    assert!(err.is_io()); // io::Error → ParseError
}

#[test]
fn test_error_propagation_with_context() {
    // Test error propagation with context accumulation
    let invalid_yaml = "database:\n  host: null\n  port: wrong";

    let result: Result<DatabaseConfig> = (|| {
        let content = invalid_yaml.to_string();
        let yaml = parse_yaml_content(&content)?;
        extract_database_config(&yaml)
    })();

    assert!(result.is_err());
    let err = result.unwrap_err();

    // Error should have context
    assert!(!err.context.is_empty());
    assert!(err.to_string().contains("while extracting"));
}

#[test]
fn test_successful_propagation_chain() {
    // Test successful propagation through the entire call stack
    let valid_yaml = r#"
database:
  host: localhost
  port: 5432
"#;

    let result: Result<DatabaseConfig> = (|| {
        let yaml = parse_yaml_content(valid_yaml)?;
        extract_database_config(&yaml)
    })();

    assert!(result.is_ok());
    let config = result.unwrap();
    assert_eq!(config.host, "localhost");
    assert_eq!(config.port, 5432);
}

#[test]
fn test_builder_pattern_with_context() {
    // Test that builder pattern works with context
    let err = ParseError::validation("invalid value")
        .with_context("while parsing field 'timeout'")
        .with_line(42)
        .with_column(10)
        .with_path("config.yaml");

    assert_eq!(err.line, Some(42));
    assert_eq!(err.column, Some(10));
    assert_eq!(err.path, Some("config.yaml".to_string()));
    assert!(!err.context.is_empty());
    assert_eq!(err.context, "while parsing field 'timeout'");
}

#[test]
fn test_error_type_checking() {
    // Test error type checking methods
    let syntax_err = ParseError::syntax("test");
    assert!(syntax_err.is_syntax());
    assert!(!syntax_err.is_io());

    let io_err = ParseError::io("test");
    assert!(io_err.is_io());
    assert!(!io_err.is_syntax());

    let validation_err = ParseError::validation("test");
    assert!(validation_err.is_validation());
    assert!(!validation_err.is_syntax());

    let type_err = ParseError::type_mismatch("field", "string", "int");
    assert!(type_err.is_type_mismatch());
    assert!(!type_err.is_syntax());
}

#[test]
fn test_nested_error_propagation() {
    // Test deeply nested error propagation
    fn level_3() -> Result<String> {
        Err(ParseError::io("level 3 error"))
    }

    fn level_2() -> Result<String> {
        level_3()
    }

    fn level_1() -> Result<String> {
        level_2()
    }

    let result = level_1();
    assert!(result.is_err());

    let err = result.unwrap_err();
    assert!(err.is_io());
    assert!(err.to_string().contains("level 3 error"));
}
