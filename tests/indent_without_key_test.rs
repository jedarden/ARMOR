//! Tests for indent change detection without key tokens
//!
//! This test module verifies that the YAML parser can detect indentation changes
//! even when no key tokens are present on the line (e.g., blank lines, comments).

use armor::parsers::yaml::parser::BasicParser;
use armor::parsers::yaml::YamlParser as Parser;

#[test]
fn test_detect_indent_changes_on_blank_lines() {
    let mut parser = BasicParser::new();

    // YAML with blank line that decreases indent
    let yaml = r#"
level1:
  level2:
    key1: value1

key3: value3
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse YAML with indent changes on blank lines");

    let value = result.unwrap();
    assert!(value.get("level1").is_some());
    assert!(value.get("key3").is_some());
}

#[test]
fn test_scope_exit_on_blank_line() {
    let mut parser = BasicParser::new();

    // YAML where scope exit happens on a blank line
    let yaml = r#"
outer:
  inner:
    deep: value1

sibling: value2
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should handle scope exit on blank line");

    let value = result.unwrap();
    let outer = &value["outer"];
    assert_eq!(outer["inner"]["deep"], "value1");
    assert_eq!(value["sibling"], "value2");
}

#[test]
fn test_multiple_blank_lines_with_indent_changes() {
    let mut parser = BasicParser::new();

    // Multiple blank lines at different indent levels
    let yaml = r#"
level1:
  level2:
    key1: value1


  level2_b:
    key2: value2


key3: value3
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should handle multiple blank lines with indent changes");

    let value = result.unwrap();
    assert!(value.get("level1").is_some());
    assert!(value.get("key3").is_some());
}

#[test]
fn test_indent_tracking_distinguishes_key_from_non_key() {
    let mut parser = BasicParser::strict();

    // This test verifies that the parser can distinguish between
    // indent changes with keys vs without keys
    let yaml = r#"
section1:
  key1: value1

  key2: value2
"#;

    let result = parser.validate_str(yaml);
    assert!(result.is_valid(), "Should correctly parse YAML with indent changes on blank lines");
}

#[test]
fn test_no_false_scope_entry_on_blank_line_increase() {
    let mut parser = BasicParser::new();

    // Blank line with increased indent should NOT enter a new scope
    let yaml = r#"
key1: value1


key2: value2
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Blank line with increased indent should not enter scope");

    let value = result.unwrap();
    assert_eq!(value["key1"], "value1");
    assert_eq!(value["key2"], "value2");
}

#[test]
fn test_complex_nesting_with_blank_lines() {
    let mut parser = BasicParser::new();

    // Complex nesting with blank lines at various levels
    let yaml = r#"
app:
  server:
    host: localhost
    port: 8080

    ssl:
      enabled: true
      cert: /path/to/cert

  database:

    primary:
      host: db1.example.com
      port: 5432

    replica:
      host: db2.example.com
      port: 5432
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Complex nesting with blank lines should parse successfully");

    let value = result.unwrap();
    let app = &value["app"];
    assert!(app.get("server").is_some());
    assert!(app.get("database").is_some());
    assert_eq!(app["server"]["host"], "localhost");
    assert_eq!(app["database"]["primary"]["host"], "db1.example.com");
}

#[test]
fn test_blank_lines_dont_create_duplicate_key_errors() {
    let mut parser = BasicParser::strict();

    // Blank lines should not cause false duplicate key detection
    let yaml = r#"
section:
  key1: value1

  key2: value2

section2:
  key1: value3
  key2: value4
"#;

    let result = parser.validate_str(yaml);
    assert!(result.is_valid(), "Blank lines should not cause false duplicate key errors");
}

#[test]
fn test_sequence_with_blank_lines() {
    let mut parser = BasicParser::new();

    // Sequence items separated by blank lines
    let yaml = r#"
items:
  - name: item1
    value: 100

  - name: item2
    value: 200

  - name: item3
    value: 300
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Sequences with blank lines should parse successfully");

    let value = result.unwrap();
    let items = value["items"].as_sequence().unwrap();
    assert_eq!(items.len(), 3);
}

#[test]
fn test_indent_decrease_on_blank_line_between_siblings() {
    let mut parser = BasicParser::new();

    // Blank line between sibling mappings
    let yaml = r#"
parent1:
  child1: value1
  child2: value2

parent2:
  child3: value3
  child4: value4
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Blank line between siblings should parse successfully");

    let value = result.unwrap();
    assert_eq!(value["parent1"]["child1"], "value1");
    assert_eq!(value["parent2"]["child3"], "value3");
}

#[test]
fn test_deep_nesting_with_blank_line_scope_exit() {
    let mut parser = BasicParser::new();

    // Deep nesting where scope exit happens on blank line
    let yaml = r#"
level1:
  level2:
    level3:
      level4:
        deep: value1


level5: value2
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Deep nesting with blank line scope exit should parse successfully");

    let value = result.unwrap();
    assert_eq!(value["level1"]["level2"]["level3"]["level4"]["deep"], "value1");
    assert_eq!(value["level5"], "value2");
}

#[test]
fn test_no_interference_with_existing_key_parsing() {
    let mut parser = BasicParser::new();

    // Ensure indent detection on blank lines doesn't interfere with key parsing
    let yaml = r#"
simple: value
nested:
  key1: value1
  key2: value2
inline:
  key3: value3
  key4: value4
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Indent detection should not interfere with key parsing");

    let value = result.unwrap();
    assert_eq!(value["simple"], "value");
    assert_eq!(value["nested"]["key1"], "value1");
    assert_eq!(value["inline"]["key3"], "value3");
}

#[test]
fn test_comments_with_different_indents() {
    let mut parser = BasicParser::new();

    // Comments at various indent levels (should not affect scope)
    let yaml = r#"
# Root level comment
root:
  # Indented comment
  key1: value1

  # Another comment
  key2: value2
# Another root comment
root2: value3
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Comments with different indents should not affect parsing");

    let value = result.unwrap();
    assert_eq!(value["root"]["key1"], "value1");
    assert_eq!(value["root2"], "value3");
}

#[test]
fn test_blank_line_at_document_start() {
    let mut parser = BasicParser::new();

    // Blank line at document start
    let yaml = r#"

key1: value1
key2: value2
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Blank line at document start should parse successfully");

    let value = result.unwrap();
    assert_eq!(value["key1"], "value1");
}

#[test]
fn test_blank_line_at_document_end() {
    let mut parser = BasicParser::new();

    // Blank line at document end
    let yaml = r#"
key1: value1
key2: value2

"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Blank line at document end should parse successfully");

    let value = result.unwrap();
    assert_eq!(value["key1"], "value1");
    assert_eq!(value["key2"], "value2");
}

#[test]
fn test_mixed_blank_lines_and_keys() {
    let mut parser = BasicParser::new();

    // Mix of blank lines and keys with varying indents
    let yaml = r#"
root:
  level1:
    key1: value1


    key2: value2

  level2:
    key3: value3

key4: value4
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Mixed blank lines and keys should parse successfully");

    let value = result.unwrap();
    assert_eq!(value["root"]["level1"]["key1"], "value1");
    assert_eq!(value["root"]["level1"]["key2"], "value2");
    assert_eq!(value["key4"], "value4");
}
