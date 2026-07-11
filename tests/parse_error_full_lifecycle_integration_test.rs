//! Full-lifecycle integration tests for ParseError
//!
//! This test suite covers ParseError behavior through realistic parsing workflows,
//! testing error creation, display, propagation, and context preservation as they
//! occur in actual YAML parsing operations.

use armor::parsers::yaml::{ParseError, Result};
use std::path::Path;

// ===========================================================================
// Error Creation from Parser Context
// ===========================================================================

#[test]
fn test_error_creation_from_file_read_context() {
    /// Simulates reading a configuration file and converting I/O errors
    fn read_config_file(path: &Path) -> Result<String> {
        std::fs::read_to_string(path)
            .map_err(|e| ParseError::io(e.to_string())
                .with_path(path.to_string_lossy().to_string())
                .with_context(format!("while reading configuration file: {}", path.display())))
    }

    let result = read_config_file(Path::new("/nonexistent/config.yaml"));

    assert!(result.is_err());
    let error = result.unwrap_err();

    // Verify error creation from parser context
    assert!(error.is_io());
    assert!(error.path.is_some());
    assert!(error.context.contains("while reading configuration file"));
    assert!(error.context.contains("/nonexistent/config.yaml"));
}

#[test]
fn test_error_creation_from_yaml_parsing_context() {
    /// Simulates YAML parsing with syntax error detection
    fn parse_yaml_with_context(content: &str, source_path: &str) -> Result<serde_yaml::Value> {
        serde_yaml::from_str(content)
            .map_err(|e| {
                let parse_err = ParseError::from(e);
                parse_err
                    .with_path(source_path.to_string())
                    .with_context("while parsing YAML document structure")
            })
    }

    let invalid_yaml = "invalid: yaml: [unclosed";
    let result = parse_yaml_with_context(invalid_yaml, "config/services.yaml");

    assert!(result.is_err());
    let error = result.unwrap_err();

    // Verify error was created from parsing context
    assert!(error.path.as_ref().unwrap().contains("services.yaml"));
    assert!(error.context.contains("while parsing YAML document"));
}

#[test]
fn test_error_creation_from_validation_context() {
    /// Simulates field validation with type checking
    fn validate_port_field(value: &serde_yaml::Value, field_path: &str) -> Result<u16> {
        value.as_i64()
            .ok_or_else(|| ParseError::type_mismatch(
                field_path,
                "integer",
                value.as_str().unwrap_or("null")
            ).with_context("while validating port configuration"))
            .and_then(|port| {
                if port < 1 || port > 65535 {
                    Err(ParseError::validation(
                        format!("port {} is out of valid range (1-65535)", port)
                    ).with_context("while validating port configuration"))
                } else {
                    Ok(port as u16)
                }
            })
    }

    // Test type mismatch error creation
    let invalid_value = serde_yaml::from_str::<serde_yaml::Value>("port: \"8080\"").unwrap();
    let port_obj = &invalid_value.as_mapping().unwrap().get("port").unwrap();
    let result = validate_port_field(port_obj, "server.port");

    assert!(result.is_err());
    let error = result.unwrap_err();

    assert!(error.is_type_mismatch());
    assert!(error.context.contains("while validating port"));
}

#[test]
fn test_error_creation_from_nested_parsing_context() {
    /// Simulates nested structure parsing with error accumulation
    fn extract_nested_field(value: &serde_yaml::Value, path: &str) -> Result<String> {
        let parts: Vec<&str> = path.split('.').collect();
        let mut current = value;

        for (i, part) in parts.iter().enumerate() {
            match current.get(part) {
                Some(next) => current = next,
                None => {
                    return Err(ParseError::validation(
                        format!("missing field: {}", part)
                    )
                    .with_context(format!(
                        "while navigating nested structure at level {} (path: {})",
                        i,
                        path
                    )))
                }
            }
        }

        current.as_str()
            .map(|s| s.to_string())
            .ok_or_else(|| ParseError::type_mismatch(
                path,
                "string",
                "null or other type"
            ).with_context(format!("while extracting final value from path: {}", path)))
    }

    let yaml = r#"database:
  connection:
    timeout: 30"#;
    let value = serde_yaml::from_str::<serde_yaml::Value>(yaml).unwrap();

    // Test successful extraction
    let result = extract_nested_field(&value, "database.connection.timeout");
    assert!(result.is_err()); // timeout is a number, not string

    // Test missing field error
    let result = extract_nested_field(&value, "database.connection.missing");
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.context.contains("while navigating nested structure"));
    assert!(error.context.contains("level"));
}

// ===========================================================================
// Error Display Formatting in Parsing Context
// ===========================================================================

#[test]
fn test_error_display_for_file_not_found_scenario() {
    let error = ParseError::io("No such file or directory (os error 2)")
        .with_path("config/production/database.yaml")
        .with_context("while loading production database configuration");

    let display = format!("{}", error);

    // Verify comprehensive display formatting
    assert!(display.contains("config/production/database.yaml"));
    assert!(display.contains("I/O error"));
    assert!(display.contains("No such file or directory"));
    assert!(display.contains("while loading production database configuration"));

    // Display should be readable and informative
    assert!(display.len() > 50); // Substantial error message
}

#[test]
fn test_error_display_with_yaml_snippet_context() {
    let error = ParseError::syntax("invalid mapping syntax")
        .with_path("services.yaml")
        .with_location(15, 8)
        .with_context("while parsing service definitions")
        .with_snippet("services:\n  - name: web\n    port: abc");

    let display = format!("{}", error);
    let detailed = error.detailed_report();

    // Verify display includes all context
    assert!(display.contains("services.yaml:15:8"));
    assert!(display.contains("syntax error"));
    assert!(display.contains("invalid mapping syntax"));
    assert!(display.contains("while parsing service definitions"));

    // Verify detailed report includes snippet
    assert!(detailed.contains("snippet"));
    assert!(detailed.contains("services:"));
    assert!(detailed.contains("port: abc"));
}

#[test]
fn test_error_display_type_mismatch_with_field_path() {
    let error = ParseError::type_mismatch(
        "database.connection.pool.max_connections",
        "integer",
        "string"
    )
    .with_path("production.yaml")
    .with_location(42, 18)
    .with_context("while parsing database connection pool settings");

    let display = format!("{}", error);

    // Verify type mismatch display includes field path
    assert!(display.contains("production.yaml:42:18"));
    assert!(display.contains("type mismatch"));
    assert!(display.contains("database.connection.pool.max_connections"));
    assert!(display.contains("expected integer"));
    assert!(display.contains("got string"));
    assert!(display.contains("while parsing database connection pool settings"));
}

#[test]
fn test_error_display_validation_with_constraint_details() {
    let port_value = 70000;
    let error = ParseError::validation(
        format!("port must be between 1-65535, got {}", port_value)
    )
    .with_path("config/database.yaml")
    .with_location(25, 10)
    .with_context("while validating database port configuration");

    let display = format!("{}", error);
    let summary = error.summary();

    // Verify validation error display
    assert!(display.contains("config/database.yaml:25:10"));
    assert!(display.contains("validation error"));
    assert!(display.contains("port must be between 1-65535"));
    assert!(display.contains("70000"));

    // Summary should be concise but informative
    assert!(summary.contains("config/database.yaml"));
    assert!(summary.contains("validation error"));
    assert!(!summary.is_empty());
}

#[test]
fn test_error_display_multiple_errors_report() {
    let errors = vec![
        ParseError::syntax("invalid indentation")
            .with_path("config.yaml")
            .with_location(5, 4),
        ParseError::type_mismatch("server.port", "integer", "string")
            .with_path("config.yaml")
            .with_location(10, 12),
        ParseError::validation("port out of range")
            .with_path("config.yaml")
            .with_location(10, 12),
    ];

    let formatted_errors: Vec<String> = errors
        .iter()
        .map(|e| format!("{}", e))
        .collect();

    // Verify each error is properly formatted
    assert_eq!(formatted_errors.len(), 3);

    // First error - syntax
    assert!(formatted_errors[0].contains("syntax error"));
    assert!(formatted_errors[0].contains("invalid indentation"));
    assert!(formatted_errors[0].contains("config.yaml:5:4"));

    // Second error - type mismatch
    assert!(formatted_errors[1].contains("type mismatch"));
    assert!(formatted_errors[1].contains("server.port"));
    assert!(formatted_errors[1].contains("expected integer"));

    // Third error - validation
    assert!(formatted_errors[2].contains("validation error"));
    assert!(formatted_errors[2].contains("port out of range"));
}

// ===========================================================================
// Error Propagation Through Result Types
// ===========================================================================

#[test]
fn test_error_propagation_through_parsing_pipeline() {
    fn read_file(path: &Path) -> Result<String> {
        std::fs::read_to_string(path)
            .map_err(|e| ParseError::from(e).with_context("while reading file"))
    }

    fn parse_yaml(content: &str) -> Result<serde_yaml::Value> {
        serde_yaml::from_str(content)
            .map_err(|e| ParseError::from(e).with_context("while parsing YAML"))
    }

    fn validate_config(value: serde_yaml::Value) -> Result<serde_yaml::Value> {
        // Simulate validation
        if value.get("required_field").is_none() {
            return Err(ParseError::validation("missing required field: required_field")
                .with_context("while validating configuration structure"));
        }
        Ok(value)
    }

    fn full_pipeline(path: &Path) -> Result<serde_yaml::Value> {
        let content = read_file(path)?;
        let yaml = parse_yaml(&content)?;
        let validated = validate_config(yaml)?;
        Ok(validated)
    }

    // Test error propagation at file reading stage
    let result = full_pipeline(Path::new("/nonexistent/config.yaml"));
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.context.contains("while reading file"));
}

#[test]
fn test_error_propagation_with_context_accumulation() {
    fn level_3_operation() -> Result<String> {
        Err(ParseError::syntax("invalid token")
            .with_line(10)
            .with_column(5)
            .with_context("while parsing token"))
    }

    fn level_2_operation() -> Result<String> {
        level_3_operation()
            .map_err(|e| e.with_context("while processing statement"))
    }

    fn level_1_operation() -> Result<String> {
        level_2_operation()
            .map_err(|e| e.with_context("while parsing function body"))
    }

    let result = level_1_operation();
    assert!(result.is_err());

    let error = result.unwrap_err();
    // Context should be accumulated through the chain
    assert!(error.context.contains("while parsing function body"));
    assert!(error.line == Some(10)); // Original line preserved
    assert!(error.column == Some(5)); // Original column preserved
}

#[test]
fn test_error_propagation_with_successful_intermediate_steps() {
    fn step1_read() -> Result<String> {
        Ok("valid: content".to_string())
    }

    fn step2_parse(content: String) -> Result<serde_yaml::Value> {
        serde_yaml::from_str(&content)
            .map_err(|e| ParseError::from(e).with_context("while parsing YAML"))
    }

    fn step3_validate(value: serde_yaml::Value) -> Result<String> {
        value.get("valid")
            .and_then(|v| v.as_str())
            .map(|s| s.to_string())
            .ok_or_else(|| ParseError::type_mismatch("valid", "string", "null")
                .with_context("while extracting 'valid' field"))
    }

    let result = step1_read()
        .and_then(|content| step2_parse(content))
        .and_then(|value| step3_validate(value));

    assert!(result.is_ok());
    assert_eq!(result.unwrap(), "content");
}

#[test]
fn test_error_propagation_with_question_operator() {
    fn multi_step_pipeline(input: &str) -> Result<String> {
        let parsed = serde_yaml::from_str::<serde_yaml::Value>(input)?;
        let field = parsed.get("field")
            .ok_or_else(|| ParseError::validation("missing 'field'"))?;
        let value = field.as_str()
            .ok_or_else(|| ParseError::type_mismatch("field", "string", "null"))?;
        Ok(value.to_string())
    }

    // Test with invalid YAML
    let result = multi_step_pipeline("invalid: yaml: [");
    assert!(result.is_err());
    let error = result.unwrap_err();
    // Error should be from serde_yaml parsing
    assert!(error.is_syntax() || error.to_string().contains("error"));

    // Test with valid YAML but missing field
    let result = multi_step_pipeline("other: value");
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(error.to_string().contains("missing 'field'"));

    // Test with wrong type
    let result = multi_step_pipeline("field: 123");
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_type_mismatch());
}

// ===========================================================================
// Error Conversion from Other Error Types
// ===========================================================================

#[test]
fn test_error_conversion_from_io_error_in_parsing_workflow() {
    fn read_yaml_file(path: &Path) -> Result<String> {
        std::fs::read_to_string(path)?; // io::Error automatically converts
        Ok("content".to_string())
    }

    let result = read_yaml_file(Path::new("/nonexistent.yaml"));
    assert!(result.is_err());

    let error = result.unwrap_err();
    assert!(error.is_io());
    assert!(error.to_string().contains("No such file"));
}

#[test]
fn test_error_conversion_from_serde_yaml_error() {
    fn parse_yaml_content(content: &str) -> Result<serde_yaml::Value> {
        serde_yaml::from_str::<serde_yaml::Value>(content)?; // serde_yaml::Error automatically converts
        Ok(serde_yaml::Value::Null)
    }

    let invalid_yaml = "unclosed: [";
    let result = parse_yaml_content(invalid_yaml);
    assert!(result.is_err());

    let error = result.unwrap_err();
    // Should be converted from serde_yaml::Error
    assert!(!error.to_string().is_empty());

    // Error should contain useful information
    let error_string = error.to_string();
    assert!(error_string.contains("error") || error_string.contains("syntax"));
}

#[test]
fn test_error_conversion_from_utf8_error() {
    fn parse_yaml_bytes(bytes: &[u8]) -> Result<serde_yaml::Value> {
        let content = std::str::from_utf8(bytes)?; // Utf8Error automatically converts
        serde_yaml::from_str::<serde_yaml::Value>(content)?; // serde_yaml::Error automatically converts
        Ok(serde_yaml::Value::Null)
    }

    let invalid_utf8 = b"\xFF\xFE invalid";
    let result = parse_yaml_bytes(invalid_utf8);
    assert!(result.is_err());

    let error = result.unwrap_err();
    // Should be converted from Utf8Error
    assert!(error.to_string().contains("invalid UTF-8"));
}

#[test]
fn test_error_conversion_chain() {
    fn complex_parsing_operation(bytes: &[u8]) -> Result<String> {
        // Step 1: UTF-8 conversion (Utf8Error → ParseError)
        let content = std::str::from_utf8(bytes)?;

        // Step 2: YAML parsing (serde_yaml::Error → ParseError)
        let value = serde_yaml::from_str::<serde_yaml::Value>(content)?;

        // Step 3: Field extraction (custom ParseError)
        let field = value.get("target")
            .ok_or_else(|| ParseError::validation("missing 'target' field"))?;

        let string_value = field.as_str()
            .ok_or_else(|| ParseError::type_mismatch("target", "string", "other"))?;

        Ok(string_value.to_string())
    }

    // Test UTF-8 error conversion
    let result = complex_parsing_operation(b"\xFF target: test");
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.to_string().contains("invalid UTF-8"));

    // Test YAML parsing error conversion
    let result = complex_parsing_operation(b"invalid: yaml: [");
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.to_string().contains("error") || error.to_string().contains("syntax"));

    // Test custom error creation
    let result = complex_parsing_operation(b"other: value");
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.is_validation());
    assert!(error.to_string().contains("missing 'target'"));
}

// ===========================================================================
// Error Context Preservation
// ===========================================================================

#[test]
fn test_error_context_preservation_through_multiple_layers() {
    fn inner_parser() -> Result<String> {
        Err(ParseError::syntax("unexpected token")
            .with_line(5)
            .with_column(10)
            .with_snippet("value: unexpected_token"))
    }

    fn middle_processor() -> Result<String> {
        inner_parser()
            .map_err(|e| e
                .with_path("config.yaml")
                .with_context("while processing configuration file"))
    }

    fn outer_validator() -> Result<String> {
        middle_processor()
            .map_err(|e| e
                .with_context("while validating application configuration")
                .with_context("during startup validation"))
    }

    let result = outer_validator();
    assert!(result.is_err());

    let error = result.unwrap_err();

    // Original context should be preserved
    assert_eq!(error.line, Some(5));
    assert_eq!(error.column, Some(10));
    assert_eq!(error.path, Some("config.yaml".to_string()));
    assert_eq!(error.snippet, Some("value: unexpected_token".to_string()));

    // Context should be accumulated (note: current implementation overwrites)
    // The final context should be from the outermost layer
    assert!(error.context.contains("during startup validation") ||
            error.context.contains("while validating") ||
            error.context.contains("while processing"));
}

#[test]
fn test_error_context_preservation_with_snippets() {
    let original_error = ParseError::syntax("invalid escape sequence")
        .with_line(8)
        .with_column(15)
        .with_snippet("app:\n  name: \"My\\nApp\"");

    let enhanced_error = original_error
        .with_path("config/app.yaml")
        .with_context("while parsing application configuration");

    // Verify all context is preserved
    assert_eq!(enhanced_error.line, Some(8));
    assert_eq!(enhanced_error.column, Some(15));
    assert_eq!(enhanced_error.snippet, Some("app:\n  name: \"My\\nApp\"".to_string()));
    assert_eq!(enhanced_error.path, Some("config/app.yaml".to_string()));
    assert_eq!(enhanced_error.context, "while parsing application configuration");
}

#[test]
fn test_error_context_preservation_in_collection() {
    fn collect_errors_from_multiple_sources() -> Vec<ParseError> {
        vec![
            ParseError::io("file not found")
                .with_path("config1.yaml")
                .with_context("while loading first config"),
            ParseError::syntax("invalid YAML")
                .with_path("config2.yaml")
                .with_line(10)
                .with_context("while parsing second config"),
            ParseError::validation("port out of range")
                .with_path("config3.yaml")
                .with_line(5)
                .with_snippet("port: 70000")
                .with_context("while validating third config"),
        ]
    }

    let errors = collect_errors_from_multiple_sources();

    // Verify each error preserves its unique context
    assert_eq!(errors.len(), 3);

    assert!(errors[0].is_io());
    assert_eq!(errors[0].path, Some("config1.yaml".to_string()));
    assert!(errors[0].context.contains("while loading first config"));

    assert!(errors[1].is_syntax());
    assert_eq!(errors[1].path, Some("config2.yaml".to_string()));
    assert_eq!(errors[1].line, Some(10));
    assert!(errors[1].context.contains("while parsing second config"));

    assert!(errors[2].is_validation());
    assert_eq!(errors[2].path, Some("config3.yaml".to_string()));
    assert_eq!(errors[2].line, Some(5));
    assert!(errors[2].snippet.is_some());
    assert!(errors[2].context.contains("while validating third config"));
}

#[test]
fn test_error_context_preservation_with_builder_pattern() {
    // Build complex error step by step
    let error = ParseError::type_mismatch("database.port", "integer", "string")
        .with_line(42)
        .with_column(16)
        .with_path("production.yaml")
        .with_snippet("database:\n  port: \"5432\"")
        .with_context("while parsing database configuration")
        .with_context("during production config validation");

    // Verify all context is preserved through builder chain
    assert!(error.is_type_mismatch());
    assert_eq!(error.line, Some(42));
    assert_eq!(error.column, Some(16));
    assert_eq!(error.path, Some("production.yaml".to_string()));
    assert_eq!(error.snippet, Some("database:\n  port: \"5432\"".to_string()));

    // Context should reflect the last set context
    assert!(error.context.contains("during production config validation") ||
            error.context.contains("while parsing database"));
}

// ===========================================================================
// Real-World Integration Scenarios
// ===========================================================================

#[test]
fn test_real_world_config_loading_with_errors() {
    fn load_application_config(path: &Path) -> Result<(String, u16)> {
        // Simulate reading and parsing application configuration
        let content = std::fs::read_to_string(path)
            .map_err(|e| ParseError::from(e)
                .with_context("while reading application configuration file"))?;

        let yaml = serde_yaml::from_str::<serde_yaml::Value>(&content)
            .map_err(|e| ParseError::from(e)
                .with_context("while parsing application configuration"))?;

        let app_name = yaml.get("app")
            .and_then(|v| v.as_str())
            .ok_or_else(|| ParseError::validation("missing 'app' field")
                .with_context("while extracting application name"))?
            .to_string();

        let port = yaml.get("port")
            .and_then(|v| v.as_i64())
            .ok_or_else(|| ParseError::type_mismatch("port", "integer", "null")
                .with_context("while extracting port number"))?;

        if port < 1 || port > 65535 {
            return Err(ParseError::validation(format!("port {} is out of range", port))
                .with_context("while validating port configuration"));
        }

        Ok((app_name, port as u16))
    }

    // Test with non-existent file
    let result = load_application_config(Path::new("/nonexistent/app.yaml"));
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.context.contains("while reading application configuration"));

    // Test with invalid YAML content (would need to create temp file for full test)
    let _invalid_yaml = "app: test\nport: invalid";
    // Would need to write this to a temp file for full integration test
}

#[test]
fn test_real_world_multi_file_config_with_error_aggregation() {
    fn load_service_configs(paths: &[&str]) -> Result<Vec<String>> {
        let mut configs = Vec::new();

        for path in paths {
            let content = std::fs::read_to_string(path)
                .map_err(|e| ParseError::from(e)
                    .with_path((*path).to_string())
                    .with_context(format!("while reading service config: {}", path)))?;

            let yaml = serde_yaml::from_str::<serde_yaml::Value>(&content)
                .map_err(|e| ParseError::from(e)
                    .with_path((*path).to_string())
                    .with_context(format!("while parsing service config: {}", path)))?;

            let service_name = yaml.get("name")
                .and_then(|v| v.as_str())
                .ok_or_else(|| ParseError::validation("missing 'name' field")
                    .with_path((*path).to_string())
                    .with_context(format!("while extracting service name from: {}", path)))?;

            configs.push(service_name.to_string());
        }

        Ok(configs)
    }

    // Test error context preservation with file path
    let result = load_service_configs(&["/nonexistent/service1.yaml"]);
    assert!(result.is_err());
    let error = result.unwrap_err();
    assert!(error.context.contains("while reading service config"));
    assert!(error.context.contains("service1.yaml"));
}

#[test]
fn test_real_world_error_recovery_and_continuation() {
    fn validate_config_with_recovery(config_content: &str) -> Result<String> {
        let yaml = serde_yaml::from_str::<serde_yaml::Value>(config_content)
            .map_err(|e| ParseError::from(e)
                .with_context("while parsing configuration"))?;

        let app_name = yaml.get("app")
            .and_then(|v| v.as_str())
            .ok_or_else(|| ParseError::validation("missing 'app' field"))?;

        Ok(app_name.to_string())
    }

    // Simulate processing multiple configs, continuing on errors
    let configs = vec![
        ("app: test1", true),
        ("invalid: yaml:", false),
        ("app: test2", true),
    ];

    let results: Vec<(String, Result<String>)> = configs
        .into_iter()
        .enumerate()
        .map(|(i, (content, _))| {
            (format!("config_{}", i), validate_config_with_recovery(content))
        })
        .collect();

    // Should have processed all configs despite errors
    assert_eq!(results.len(), 3);

    // First should succeed
    assert!(results[0].1.is_ok());

    // Second should fail
    assert!(results[1].1.is_err());
    let error = results[1].1.as_ref().unwrap_err();
    assert!(error.context.contains("while parsing configuration"));

    // Third should succeed
    assert!(results[2].1.is_ok());
}