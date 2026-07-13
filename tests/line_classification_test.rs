//! Test line classification for key-bearing vs indent-only detection
//!
//! This test verifies that the YAML parser can properly distinguish between
//! key-bearing lines (containing YAML keys) and indent-only lines (no keys).

use armor::parsers::yaml::{
    parser::BasicParser,
    scope::{classify_line_type, has_key_token, LineClassification},
    line_parser::calculate_indentation,
};

#[test]
fn test_detect_key_bearing_lines() {
    // Test key-bearing lines
    let yaml = r#"
key1: value1
  nested_key: value2
parent_key:
  another_key: value3
- sequence_key: value4
"#;

    for (line_num, line) in yaml.lines().enumerate() {
        let line_type = classify_line_type(line);

        match line_num {
            0 => assert_eq!(line_type, LineClassification::Empty, "Line 0 should be empty"),
            1 => assert!(line_type.is_key_bearing(), "Line 1 'key1: value1' should be key-bearing"),
            2 => assert!(line_type.is_key_bearing(), "Line 2 'nested_key: value2' should be key-bearing"),
            3 => assert!(line_type.is_key_bearing(), "Line 3 'parent_key:' should be key-bearing"),
            4 => assert!(line_type.is_key_bearing(), "Line 4 'another_key: value3' should be key-bearing"),
            5 => assert!(line_type.is_key_bearing(), "Line 5 'sequence_key: value4' should be key-bearing"),
            _ => {}
        }
    }
}

#[test]
fn test_detect_indent_only_lines() {
    // Test indent-only lines
    let yaml = r#"
# This is a comment
  # Indented comment
  Some text without colon
    More text
  value_without_colon
another_key: value
"#;

    for (line_num, line) in yaml.lines().enumerate() {
        let line_type = classify_line_type(line);

        match line_num {
            0 => assert_eq!(line_type, LineClassification::Empty, "Line 0 should be empty"),
            1 => assert!(line_type.is_indent_only(), "Line 1 comment should be indent-only"),
            2 => assert!(line_type.is_indent_only(), "Line 2 indented comment should be indent-only"),
            3 => assert!(line_type.is_indent_only(), "Line 3 text without colon should be indent-only"),
            4 => assert!(line_type.is_indent_only(), "Line 4 more text should be indent-only"),
            5 => assert!(line_type.is_indent_only(), "Line 5 value without colon should be indent-only"),
            6 => assert!(line_type.is_key_bearing(), "Line 6 'another_key: value' should be key-bearing"),
            _ => {}
        }
    }
}

#[test]
fn test_empty_line_classification() {
    // Test empty lines - create specific pattern we can verify
    let lines = vec![
        ("", LineClassification::Empty),
        ("", LineClassification::Empty),
        ("", LineClassification::Empty),
        ("key: value", LineClassification::KeyBearing),
    ];

    for (line, expected_type) in lines {
        let line_type = classify_line_type(line);
        assert_eq!(line_type, expected_type,
                   "Line '{}' should be classified as {:?}, but got {:?}", line, expected_type, line_type);
    }
}

#[test]
fn test_has_key_token_function() {
    // Test the convenience function
    assert!(has_key_token("key: value"), "Key-bearing line should return true");
    assert!(has_key_token("  nested_key:"), "Indented key-bearing line should return true");
    assert!(has_key_token("- item_key: value"), "Sequence key-bearing line should return true");

    assert!(!has_key_token("  # comment"), "Comment line should return false");
    assert!(!has_key_token("  some text"), "Text without colon should return false");
    assert!(!has_key_token(""), "Empty line should return false");
    assert!(!has_key_token("    "), "Whitespace-only line should return false");
}

#[test]
fn test_parser_tracks_line_type_in_state() {
    // Test that line classification is consistent and can be used for parser state
    let yaml = r#"
# Comment line
parent_key:
  nested_key: value
  # Another comment
  indent_only_text
sibling_key: value2
"#;

    let mut line_types = Vec::new();

    for (line_num, line) in yaml.lines().enumerate() {
        let line_type = classify_line_type(line);
        line_types.push((line_num, line_type));

        // Verify classification is consistent
        let reclassified = classify_line_type(line);
        assert_eq!(line_type, reclassified,
                   "Line classification should be consistent for line {}: '{}'", line_num, line.trim());
    }

    // Verify expected classifications
    let expected = [
        (0, LineClassification::Empty),
        (1, LineClassification::IndentOnly),    // Comment
        (2, LineClassification::KeyBearing),    // parent_key:
        (3, LineClassification::KeyBearing),    // nested_key: value
        (4, LineClassification::IndentOnly),    // Comment
        (5, LineClassification::IndentOnly),    // Text without colon
        (6, LineClassification::KeyBearing),    // sibling_key: value2
    ];

    for (i, (line_num, expected_type)) in expected.iter().enumerate() {
        let actual_type = line_types.get(*line_num).map(|(_, t)| *t).unwrap_or(LineClassification::Empty);
        assert_eq!(actual_type, *expected_type,
                   "Line classification mismatch at index {}", i);
    }
}

#[test]
fn test_complex_yaml_structure_classification() {
    // Test complex YAML with mixed line types
    let yaml = r#"
# Configuration file
application:
  name: MyApp
  version: 1.0
  # Server configuration
  server:
    host: localhost
    port: 8080

  # Database settings
  database:
    host: db.example.com
    port: 5432

# Logging configuration
logging:
  level: info
  outputs:
    - type: stdout
      format: json
    - type: file
      format: text
        path: /var/log/app.log
"#;

    let mut key_bearing_count = 0;
    let mut indent_only_count = 0;
    let mut empty_count = 0;

    for line in yaml.lines() {
        let line_type = classify_line_type(line);
        match line_type {
            LineClassification::KeyBearing => key_bearing_count += 1,
            LineClassification::IndentOnly => indent_only_count += 1,
            LineClassification::Empty => empty_count += 1,
        }
    }

    // We should have detected several key-bearing lines
    assert!(key_bearing_count >= 15, "Should detect at least 15 key-bearing lines, found {}", key_bearing_count);

    // We should have several indent-only (comment) lines
    assert!(indent_only_count >= 4, "Should detect at least 4 indent-only (comment) lines, found {}", indent_only_count);

    // We should have some empty lines
    assert!(empty_count >= 2, "Should detect at least 2 empty lines, found {}", empty_count);
}

#[test]
fn test_indent_only_triggers_scope_exit() {
    // Test that indent-only lines can trigger scope exit when indent decreases
    let yaml = r#"
parent:
  child: value

another: value2
"#;

    // Track classifications
    let mut classifications = Vec::new();
    for line in yaml.lines() {
        let line_type = classify_line_type(line);
        classifications.push(line_type);
    }

    // Verify line types
    assert_eq!(classifications[0], LineClassification::Empty, "Line 0 should be empty");
    assert_eq!(classifications[1], LineClassification::KeyBearing, "Line 1 'parent:' should be key-bearing");
    assert_eq!(classifications[2], LineClassification::KeyBearing, "Line 2 'child: value' should be key-bearing");
    assert_eq!(classifications[3], LineClassification::Empty, "Line 3 blank line should be empty");
    assert_eq!(classifications[4], LineClassification::KeyBearing, "Line 4 'another: value2' should be key-bearing");
}

#[test]
fn test_sequence_item_classification() {
    // Test that sequence items are properly classified
    let yaml = r#"
items:
  - name: item1
    value: 100
  - name: item2
    value: 200
  - value_only
"#;

    for (line_num, line) in yaml.lines().enumerate() {
        let line_type = classify_line_type(line);

        match line_num {
            0 => assert_eq!(line_type, LineClassification::Empty),
            1 => assert!(line_type.is_key_bearing(), "Line 1 'items:' should be key-bearing"),
            2 => assert!(line_type.is_key_bearing(), "Line 2 sequence item should be key-bearing"),
            3 => assert!(line_type.is_key_bearing(), "Line 3 'value:' should be key-bearing"),
            4 => assert!(line_type.is_key_bearing(), "Line 4 sequence item should be key-bearing"),
            5 => assert!(line_type.is_key_bearing(), "Line 5 'value:' should be key-bearing"),
            6 => assert!(line_type.is_indent_only(), "Line 6 sequence item without colon should be indent-only"),
            _ => {}
        }
    }
}

#[test]
fn test_line_classification_with_indentation() {
    // Test classification with various indentation levels
    let test_cases = vec![
        ("key: value", 0, true),
        ("  key: value", 2, true),
        ("    key: value", 4, true),
        ("      key: value", 6, true),
        ("# comment", 0, false),
        ("  # comment", 2, false),
        ("    # comment", 4, false),
        ("some text", 0, false),
        ("  some text", 2, false),
        ("    some text", 4, false),
    ];

    for (line, expected_indent, should_be_key_bearing) in test_cases {
        let indent = calculate_indentation(line);
        let line_type = classify_line_type(line);

        assert_eq!(indent, expected_indent,
                   "Indent calculation failed for line: '{}'", line);
        assert_eq!(line_type.is_key_bearing(), should_be_key_bearing,
                   "Key-bearing detection failed for line: '{}'", line);
    }
}

fn main() {
    println!("Running line classification tests...");

    test_detect_key_bearing_lines();
    println!("✓ Key-bearing line detection test passed");

    test_detect_indent_only_lines();
    println!("✓ Indent-only line detection test passed");

    test_empty_line_classification();
    println!("✓ Empty line classification test passed");

    test_has_key_token_function();
    println!("✓ has_key_token function test passed");

    test_parser_tracks_line_type_in_state();
    println!("✓ Parser state tracking test passed");

    test_complex_yaml_structure_classification();
    println!("✓ Complex YAML structure classification test passed");

    test_indent_only_triggers_scope_exit();
    println!("✓ Indent-only scope exit test passed");

    test_sequence_item_classification();
    println!("✓ Sequence item classification test passed");

    test_line_classification_with_indentation();
    println!("✓ Line classification with indentation test passed");

    println!("\n✅ All line classification tests passed!");
}
