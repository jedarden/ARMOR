//! Tests for handling indent changes without keys
//!
//! This test suite verifies that the parser correctly handles YAML structures where
//! indentation changes but no new keys are present, such as blank lines with visual
//! indentation, scalar continuations, and other edge cases.
//!
//! Bead: bf-18g7jk
//! Acceptance Criteria:
//! - Indent changes without keys are detected correctly
//! - Scope stack remains consistent after such events
//! - No false duplicate key errors from indent-only changes
//! - Edge cases are covered by tests

use armor::parsers::yaml::parser::{Parser, BasicParser};

#[test]
fn test_blank_lines_no_indent_changes() {
    // Blank lines without indentation changes should be transparent
    let yaml = r#"
key1: value1

key2: value2


key3: value3
"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should parse successfully with blank lines: {:#?}", result);

    let value = result.unwrap();
    assert_eq!(value["key1"], "value1");
    assert_eq!(value["key2"], "value2");
    assert_eq!(value["key3"], "value3");
}

#[test]
fn test_blank_lines_with_indent_ignored() {
    // Blank lines with visual indentation should be ignored
    // They should not affect scope tracking
    let yaml = r#"
key1: value1
  # This blank line has 2 spaces but should be ignored

key2: value2
"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should parse successfully: {:#?}", result);

    let value = result.unwrap();
    assert_eq!(value["key1"], "value1");
    assert_eq!(value["key2"], "value2");
}

#[test]
fn test_nested_structure_with_blank_lines() {
    // Blank lines in nested structures should work correctly
    let yaml = r#"
parent:
  child1: value1

  child2: value2

  child3: value3
"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should parse nested structure with blank lines: {:#?}", result);

    let value = result.unwrap();
    assert_eq!(value["parent"]["child1"], "value1");
    assert_eq!(value["parent"]["child2"], "value2");
    assert_eq!(value["parent"]["child3"], "value3");
}

#[test]
fn test_deeply_nested_with_blank_lines() {
    // Deep nesting with blank lines at various levels
    let yaml = r#"
level1:
    level2a:
        level3a: value1


        level3b: value2

    level2b:
        level3: value3
"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should parse deeply nested structure: {:#?}", result);

    let value = result.unwrap();
    assert_eq!(value["level1"]["level2a"]["level3a"], "value1");
    assert_eq!(value["level1"]["level2a"]["level3b"], "value2");
    assert_eq!(value["level1"]["level2b"]["level3"], "value3");
}

#[test]
fn test_blank_lines_between_siblings() {
    // Blank lines between sibling keys should work
    let yaml = r#"
parent:
  sibling1: value1

  sibling2: value2

  sibling3: value3
"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should parse siblings with blank lines: {:#?}", result);

    let value = result.unwrap();
    assert_eq!(value["parent"]["sibling1"], "value1");
    assert_eq!(value["parent"]["sibling2"], "value2");
    assert_eq!(value["parent"]["sibling3"], "value3");
}

#[test]
fn test_multiline_scalar_with_blank_lines() {
    // Multi-line scalars should work with blank lines
    let yaml = r#"
key1: |
  Line 1
  Line 2

  Line 4 (after blank)
key2: value2
"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should parse multiline scalars: {:#?}", result);

    let value = result.unwrap();
    // Multi-line scalars preserve newlines
    assert!(value["key1"].as_str().unwrap().contains("Line 1"));
    assert!(value["key1"].as_str().unwrap().contains("Line 2"));
    assert_eq!(value["key2"], "value2");
}

#[test]
fn test_scope_consistency_after_blank_lines() {
    // Verify scope stack consistency after blank lines with different indents
    let yaml = r#"
root:
  level1:
    level2a: value1

    level2b: value2
  # Blank line at root level (but visually indented)

  level1b: value3
"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should maintain scope consistency: {:#?}", result);

    let value = result.unwrap();
    assert_eq!(value["root"]["level1"]["level2a"], "value1");
    assert_eq!(value["root"]["level1"]["level2b"], "value2");
    assert_eq!(value["root"]["level1b"], "value3");
}

#[test]
fn test_no_false_duplicate_from_blank_lines() {
    // Ensure blank lines don't cause false duplicate key errors
    let yaml = r#"
config:
  host: localhost

  port: 8080

  host: backup.example.com
  # This is a REAL duplicate and should be detected
"#;

    let parser = BasicParser::strict(); // Use strict parser to detect duplicates
    let result = parser.validate_str(yaml);

    // Should detect the duplicate 'host' key
    assert!(!result.is_valid(), "Should detect duplicate key");

    // The error should be about the duplicate 'host' key
    let error_messages: Vec<String> = result.errors.iter()
        .map(|e| e.message.clone())
        .collect();

    assert!(error_messages.iter().any(|msg| msg.contains("duplicate") && msg.contains("host")),
            "Should have error about duplicate 'host' key");
}

#[test]
fn test_comments_with_indent_ignored() {
    // Comments with indentation should be ignored for scope tracking
    let yaml = r#"
key1: value1
  # Indented comment should not affect scope

key2: value2
    # Deeply indented comment (ignored)

key3: value3
"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should parse with indented comments: {:#?}", result);

    let value = result.unwrap();
    assert_eq!(value["key1"], "value1");
    assert_eq!(value["key2"], "value2");
    assert_eq!(value["key3"], "value3");
}

#[test]
fn test_sequence_with_blank_lines() {
    // Sequences should handle blank lines correctly
    let yaml = r#"
items:
  - name: item1
    value: 1

  - name: item2
    value: 2


  - name: item3
    value: 3
"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should parse sequences with blank lines: {:#?}", result);

    let value = result.unwrap();
    let items = value["items"].as_sequence().unwrap();
    assert_eq!(items.len(), 3);
    assert_eq!(items[0]["name"], "item1");
    assert_eq!(items[1]["name"], "item2");
    assert_eq!(items[2]["name"], "item3");
}

#[test]
fn test_indent_changes_only_on_blank_lines() {
    // When only blank lines have indent changes, scope should remain consistent
    let yaml = r#"
parent:
  child1: value1

    # These blank lines at deeper indent shouldn't affect scope

  child2: value2
"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should handle indent changes on blank lines: {:#?}", result);

    let value = result.unwrap();
    assert_eq!(value["parent"]["child1"], "value1");
    assert_eq!(value["parent"]["child2"], "value2");
}

#[test]
fn test_complex_real_world_yaml() {
    // Complex real-world YAML with blank lines at various indents
    let yaml = r#"
# Configuration file
app:
  name: MyApp
  version: 1.0

  # Database configuration
  database:
    host: localhost
    port: 5432

    # Connection pool settings
    pool:
      min: 2
      max: 10

  # Server settings
  server:
    host: 0.0.0.0

    port: 8080

    ssl:
      enabled: true

      cert_path: /path/to/cert.pem
"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should parse complex real-world YAML: {:#?}", result);

    let value = result.unwrap();
    assert_eq!(value["app"]["name"], "MyApp");
    assert_eq!(value["app"]["version"], 1.0);
    assert_eq!(value["app"]["database"]["host"], "localhost");
    assert_eq!(value["app"]["database"]["port"], 5432);
    assert_eq!(value["app"]["database"]["pool"]["min"], 2);
    assert_eq!(value["app"]["database"]["pool"]["max"], 10);
    assert_eq!(value["app"]["server"]["host"], "0.0.0.0");
    assert_eq!(value["app"]["server"]["port"], 8080);
    assert_eq!(value["app"]["server"]["ssl"]["enabled"], true);
}

#[test]
fn test_blank_lines_at_root() {
    // Blank lines at root level should be ignored
    let yaml = r#"


key1: value1


key2: value2


"#;

    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    assert!(result.is_success(), "Should handle blank lines at root: {:#?}", result);

    let value = result.unwrap();
    assert_eq!(value["key1"], "value1");
    assert_eq!(value["key2"], "value2");
}
