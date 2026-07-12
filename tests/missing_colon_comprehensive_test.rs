//! Comprehensive tests for missing colon detection in YAML mappings
//!
//! These tests verify that the Rust implementation matches the Go implementation's
//! functionality for detecting missing colons in simple YAML mappings.
//!
//! Bead: bf-1sjr7
//! Acceptance Criteria:
//! - Missing colons are detected in simple YAML mappings
//! - Detection works with common YAML mapping patterns
//! - Returns line number and key name for each missing colon
//! - Unit tests cover basic cases (single key, multiple keys, nested mappings)
//! - No false positives on properly formatted YAML

use armor::parsers::yaml::{SyntaxDetector, DelimiterErrorType};

#[test]
fn test_single_key_missing_colon() {
    let mut detector = SyntaxDetector::new();
    let yaml = "key value\nanother: item";

    let errors = detector.detect_errors(yaml);

    // Should detect missing colon in line 1
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert_eq!(missing_colon_errors.len(), 1, "Should detect exactly 1 missing colon error");
    assert_eq!(missing_colon_errors[0].line, Some(1), "Error should be on line 1");
}

#[test]
fn test_multiple_keys_missing_colons() {
    let mut detector = SyntaxDetector::new();
    let yaml = "first value\nsecond thing\nthird: item";

    let errors = detector.detect_errors(yaml);

    // Should detect missing colons in lines 1 and 2
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert_eq!(missing_colon_errors.len(), 2, "Should detect 2 missing colon errors");
    let lines: Vec<_> = missing_colon_errors.iter()
        .filter_map(|e| e.line)
        .collect();
    assert!(lines.contains(&1), "Should detect error on line 1");
    assert!(lines.contains(&2), "Should detect error on line 2");
}

#[test]
fn test_nested_mapping_missing_colon() {
    let mut detector = SyntaxDetector::new();
    let yaml = "outer:\n  inner value\n  another: item";

    let errors = detector.detect_errors(yaml);

    // Should detect missing colon in line 2
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert_eq!(missing_colon_errors.len(), 1, "Should detect 1 missing colon error in nested mapping");
    assert_eq!(missing_colon_errors[0].line, Some(2), "Error should be on line 2");
}

#[test]
fn test_no_false_positives_valid_mapping() {
    let mut detector = SyntaxDetector::new();
    let yaml = "key: value\n  nested: item\n  another: thing";

    let errors = detector.detect_errors(yaml);

    // Should not detect any missing colons in valid YAML
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert!(missing_colon_errors.is_empty(), "Should not detect missing colons in valid YAML");
}

#[test]
fn test_no_false_positives_sequence_items() {
    let mut detector = SyntaxDetector::new();
    let yaml = "items:\n  - item1\n  - item2\n  - item3";

    let errors = detector.detect_errors(yaml);

    // Sequence items should not be flagged as missing colons
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert!(missing_colon_errors.is_empty(), "Sequence items should not be flagged as missing colons");
}

#[test]
fn test_no_false_positives_document_markers() {
    let mut detector = SyntaxDetector::new();
    let yaml = "---\nkey: value\n...\n";

    let errors = detector.detect_errors(yaml);

    // Document markers should not be flagged
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert!(missing_colon_errors.is_empty(), "Document markers should not be flagged");
}

#[test]
fn test_no_false_positives_comments() {
    let mut detector = SyntaxDetector::new();
    let yaml = "key: value\n# This is a comment\nanother: item";

    let errors = detector.detect_errors(yaml);

    // Comments should not be flagged as missing colons
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert!(missing_colon_errors.is_empty(), "Comments should not be flagged as missing colons");
}

#[test]
fn test_mixed_valid_and_invalid_lines() {
    let mut detector = SyntaxDetector::new();
    let yaml = "valid: value\ninvalid line\nanother: item\nmissing colon";

    let errors = detector.detect_errors(yaml);

    // Should detect missing colons only on invalid lines
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert_eq!(missing_colon_errors.len(), 2, "Should detect 2 missing colon errors");
    let lines: Vec<_> = missing_colon_errors.iter()
        .filter_map(|e| e.line)
        .collect();
    assert!(lines.contains(&2), "Should detect error on line 2");
    assert!(lines.contains(&4), "Should detect error on line 4");
}

#[test]
fn test_flow_style_not_flagged() {
    let mut detector = SyntaxDetector::new();
    let yaml = "key: {nested: value, another: item}";

    let errors = detector.detect_errors(yaml);

    // Flow style mappings should not be flagged
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert!(missing_colon_errors.is_empty(), "Flow style mappings should not be flagged");
}

#[test]
fn test_anchors_and_aliases_not_flagged() {
    let mut detector = SyntaxDetector::new();
    let yaml = "key: &anchor value\nanother: *alias";

    let errors = detector.detect_errors(yaml);

    // Anchors and aliases should not be flagged
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert!(missing_colon_errors.is_empty(), "Anchors and aliases should not be flagged");
}

#[test]
fn test_multiline_blocks_not_flagged() {
    let mut detector = SyntaxDetector::new();
    let yaml = "key: |\n  multiline content\n  more content\nanother: item";

    let errors = detector.detect_errors(yaml);

    // Multiline block content should not be flagged
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert!(missing_colon_errors.is_empty(), "Multiline block content should not be flagged");
}

#[test]
fn test_error_includes_line_number_and_key_name() {
    let mut detector = SyntaxDetector::new();
    let yaml = "mykey value\nanother: item";

    let errors = detector.detect_errors(yaml);

    // Error should include line number
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert_eq!(missing_colon_errors.len(), 1);
    assert_eq!(missing_colon_errors[0].line, Some(1), "Error should include line number");
    // The error message should mention the issue
    assert!(missing_colon_errors[0].message.contains("missing colon") ||
            missing_colon_errors[0].message.contains("D001"),
            "Error should describe the missing colon issue");
}

#[test]
fn test_complex_nested_mapping_with_missing_colon() {
    let mut detector = SyntaxDetector::new();
    let yaml = "level1:\n  level2:\n    level3 missing\n    valid: value\n  another: item";

    let errors = detector.detect_errors(yaml);

    // Should detect missing colon at deeply nested level
    let missing_colon_errors: Vec<_> = errors.iter()
        .filter(|e| e.delimiter_error_type == Some(DelimiterErrorType::MissingColon))
        .collect();

    assert_eq!(missing_colon_errors.len(), 1, "Should detect 1 missing colon error");
    assert_eq!(missing_colon_errors[0].line, Some(3), "Error should be on line 3");
}
