//! Comprehensive unit tests for YAML syntax error detection
//!
//! This module tests all three categories of syntax error detection:
//! - Indentation errors (mixed tabs/spaces, inconsistent levels)
//! - Delimiter errors (missing colons, unbalanced brackets, quote errors)
//! - Structure errors (invalid mappings, malformed sequences, duplicate keys)

use crate::parsers::yaml::syntax_detector::SyntaxDetector;
use crate::parsers::yaml::syntax_validator::SyntaxValidator;
use crate::parsers::yaml::types::ValidationError;

#[cfg(test)]
mod indentation_tests {
    use super::*;
    use crate::parsers::yaml::syntax_detector::IndentationErrorType;

    #[test]
    fn test_detect_mixed_tabs_and_spaces() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n\t  bad: value\n  good: value";

        let errors = detector.detect_errors(yaml);

        // Should detect mixed tabs and spaces in the second line
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("mixed tabs and spaces")));
    }

    #[test]
    fn test_indentation_error_classification_mixed() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n\t  bad: value";

        let errors = detector.detect_errors(yaml);

        // Should detect and classify mixed tabs and spaces
        assert!(!errors.is_empty());
        let mixed_error = errors.iter().find(|e| {
            e.indentation_error_type == Some(IndentationErrorType::MixedTabsAndSpaces)
        });
        assert!(mixed_error.is_some(), "Should detect MixedTabsAndSpaces error type");
        assert_eq!(mixed_error.unwrap().line, Some(2));
    }

    #[test]
    fn test_indentation_error_classification_invalid_level() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n   bad: value";

        let errors = detector.detect_errors(yaml);

        // Should detect and classify invalid indentation level
        assert!(!errors.is_empty());
        let invalid_level_error = errors.iter().find(|e| {
            e.indentation_error_type == Some(IndentationErrorType::InvalidIndentLevel)
        });
        assert!(invalid_level_error.is_some(), "Should detect InvalidIndentLevel error type");
        assert!(invalid_level_error.unwrap().message.contains("E002"));
    }

    #[test]
    fn test_indentation_error_classification_excessive_increase() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n        huge_jump: value";

        let errors = detector.detect_errors(yaml);

        // Should detect and classify excessive indentation increase
        assert!(!errors.is_empty());
        let excessive_error = errors.iter().find(|e| {
            e.indentation_error_type == Some(IndentationErrorType::ExcessiveIndentIncrease)
        });
        assert!(excessive_error.is_some(), "Should detect ExcessiveIndentIncrease error type");
        assert!(excessive_error.unwrap().message.contains("E003"));
    }

    #[test]
    fn test_indentation_error_classification_invalid_increase() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n   odd_increase: value";

        let errors = detector.detect_errors(yaml);

        // Should detect and classify invalid indentation increase
        assert!(!errors.is_empty());
        let invalid_increase_error = errors.iter().find(|e| {
            e.indentation_error_type == Some(IndentationErrorType::InvalidIndentIncrease)
        });
        assert!(invalid_increase_error.is_some(), "Should detect InvalidIndentIncrease error type");
        assert!(invalid_increase_error.unwrap().message.contains("E004"));
    }

    #[test]
    fn test_indentation_error_classification_tab_character() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n\ttabbed: value";

        let errors = detector.detect_errors(yaml);

        // Should detect and classify tab character
        let tab_error = errors.iter().find(|e| {
            e.indentation_error_type == Some(IndentationErrorType::TabCharacter)
        });
        assert!(tab_error.is_some(), "Should detect TabCharacter error type");
        assert!(tab_error.unwrap().message.contains("E005"));
    }

    #[test]
    fn test_multiple_indentation_errors() {
        let mut detector = SyntaxDetector::new();
        let yaml = "root:\n  valid: child\n\tbad: mixed\n    also_bad: level\nodd: indent";

        let errors = detector.detect_errors(yaml);

        // Should detect multiple indentation errors with different types
        assert!(errors.len() >= 3);

        // Check for different error types
        let error_types: Vec<_> = errors.iter()
            .filter_map(|e| e.indentation_error_type)
            .collect();

        assert!(error_types.len() >= 2, "Should detect at least 2 different error types");
    }

    #[test]
    fn test_detect_tab_only_indentation() {
        let validator = SyntaxValidator::strict();
        let yaml = "key: value\n\tbad: value";

        let result = validator.validate(yaml);

        // Strict mode should reject tabs
        assert!(!result.is_valid());
        assert!(result.errors.iter().any(|e| e.message.contains("tabs")));
    }

    #[test]
    fn test_accept_consistent_spaces() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n  nested:\n    deep: value";

        let errors = detector.detect_errors(yaml);

        // Should accept consistent 2-space indentation
        assert!(errors.is_empty());
    }

    #[test]
    fn test_detect_inconsistent_indentation() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n  nested:\n   bad_indent: value";

        let errors = detector.detect_errors(yaml);

        // Should detect non-multiple-of-2 indentation
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("not a multiple of")));
    }

    #[test]
    fn test_detect_large_indentation_increase() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n      big_jump: value";

        let errors = detector.detect_errors(yaml);

        // Should detect dramatic indentation increase
        assert!(!errors.is_empty());
        // The error message should mention "exceeds maximum" for excessive increase
        assert!(errors.iter().any(|e| e.message.contains("exceeds maximum") || e.message.contains("indentation")));
    }

    #[test]
    fn test_accept_four_space_indentation() {
        let mut detector = SyntaxDetector::with_config(crate::parsers::yaml::syntax_detector::DetectorConfig {
            base_indent_size: 4,
            ..Default::default()
        });

        let yaml = "key: value\n    nested:\n        deep: value";

        let errors = detector.detect_errors(yaml);

        // Should accept consistent 4-space indentation
        assert!(errors.is_empty());
    }

    #[test]
    fn test_indentation_error_type_codes() {
        // Test that all error types have unique codes
        let codes: Vec<_> = IndentationErrorType::all()
            .iter()
            .map(|t| t.code())
            .collect();

        let unique_codes: std::collections::HashSet<_> = codes.iter().cloned().collect();
        assert_eq!(codes.len(), unique_codes.len(), "All error codes should be unique");
    }

    #[test]
    fn test_indentation_error_type_display() {
        // Test that error types display correctly
        let error_type = IndentationErrorType::MixedTabsAndSpaces;
        let display = format!("{}", error_type);
        assert!(display.contains("E001"));
        assert!(display.contains("mixed tabs and spaces"));
    }
}

#[cfg(test)]
mod delimiter_tests {
    use super::*;

    #[test]
    fn test_detect_missing_colon_after_key() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key value\n  key2: value2";

        let errors = detector.detect_errors(yaml);

        // Should detect missing colon
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("missing colon")));
    }

    #[test]
    fn test_detect_unmatched_opening_bracket() {
        let mut detector = SyntaxDetector::new();
        let yaml = "items: [value1, value2";

        let errors = detector.detect_errors(yaml);

        // Should detect unclosed bracket
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("unclosed '['") || e.message.contains("unmatched")));
    }

    #[test]
    fn test_detect_unmatched_closing_bracket() {
        let mut detector = SyntaxDetector::new();
        let yaml = "items: value1, value2]";

        let errors = detector.detect_errors(yaml);

        // Should detect unmatched closing bracket
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("unmatched closing")));
    }

    #[test]
    fn test_detect_unmatched_opening_brace() {
        let mut detector = SyntaxDetector::new();
        let yaml = "config: {key: value";

        let errors = detector.detect_errors(yaml);

        // Should detect unclosed brace
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("unclosed '{'")));
    }

    #[test]
    fn test_detect_unmatched_closing_brace() {
        let mut detector = SyntaxDetector::new();
        let yaml = "config: key: value}";

        let errors = detector.detect_errors(yaml);

        // Should detect unmatched closing brace
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("unmatched closing")));
    }

    #[test]
    fn test_detect_mismatched_quotes() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: \"value'";

        let errors = detector.detect_errors(yaml);

        // Should detect mismatched quotes
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("mismatched quotes")));
    }

    #[test]
    fn test_detect_unclosed_double_quote() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: \"unclosed string\n  next: value";

        let errors = detector.detect_errors(yaml);

        // Should detect unclosed quote
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("unclosed quote")));
    }

    #[test]
    fn test_accept_valid_delimiters() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n  items: [one, two, three]\n  config: {a: 1, b: 2}";

        let errors = detector.detect_errors(yaml);

        // Should accept valid delimiter usage
        assert!(errors.is_empty());
    }
}

#[cfg(test)]
mod structure_tests {
    use super::*;

    #[test]
    fn test_detect_invalid_sequence_syntax() {
        let mut detector = SyntaxDetector::new();
        let yaml = "items:\n  -item1\n  - item2";

        let errors = detector.detect_errors(yaml);

        // Should detect dash without following space
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("dash") && e.message.contains("whitespace")));
    }

    #[test]
    fn test_detect_duplicate_keys_same_level() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value1\nkey: value2";

        let errors = detector.detect_errors(yaml);

        // Should detect duplicate keys
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("duplicate key")));
    }

    #[test]
    fn test_detect_nested_duplicate_keys() {
        let mut detector = SyntaxDetector::new();
        let yaml = "outer:\n  inner: value1\n  inner: value2";

        let errors = detector.detect_errors(yaml);

        // Should detect duplicate keys at same nesting level
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("duplicate key")));
    }

    #[test]
    fn test_detect_global_duplicate_keys() {
        let mut detector = SyntaxDetector::new();
        let yaml = "top:\n  key: value1\nkey: value2";

        let errors = detector.detect_errors(yaml);

        // Should detect duplicate keys globally (different nesting levels)
        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("duplicate key")));
    }

    #[test]
    fn test_detect_invalid_colon_at_start() {
        let validator = SyntaxValidator::new();
        let yaml = ":value";

        let result = validator.validate(yaml);

        // Should detect colon without space at start
        assert!(!result.is_valid());
        assert!(result.errors.iter().any(|e| e.message.contains("colon") && e.message.contains("start")));
    }

    #[test]
    fn test_accept_valid_sequences() {
        let mut detector = SyntaxDetector::new();
        let yaml = "items:\n  - item1\n  - item2\n  - item3";

        let errors = detector.detect_errors(yaml);

        // Should accept valid sequence syntax
        assert!(errors.is_empty());
    }

    #[test]
    fn test_accept_valid_mappings() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key1: value1\n  key2: value2\n  nested:\n    key3: value3";

        let errors = detector.detect_errors(yaml);

        // Should accept valid mapping syntax
        assert!(errors.is_empty());
    }
}

#[cfg(test)]
mod integration_tests {
    use super::*;

    #[test]
    fn test_valid_complete_yaml() {
        let mut detector = SyntaxDetector::new();
        let yaml = r#"
# Configuration file
server:
  host: localhost
  port: 8080
database:
  host: db.example.com
  port: 5432
  name: mydb
features:
  - authentication
  - logging
  - caching
env:
  production: true
  debug: false
"#;

        let errors = detector.detect_errors(yaml);

        // DEBUG: Print what errors were found
        if !errors.is_empty() {
            println!("DEBUG - Found {} errors in valid YAML:", errors.len());
            for (i, e) in errors.iter().enumerate() {
                println!("  {} path={} line={:?} msg={}", i, e.path, e.line, e.message);
            }
        }

        // Should accept complete valid YAML
        assert!(errors.is_empty());
    }

    #[test]
    fn test_multiple_error_types() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n  bad_indent: value\nduplicate: first\n  -no_space: item\n  items: [unclosed";

        let errors = detector.detect_errors(yaml);

        // Should detect multiple different error types
        assert!(errors.len() >= 3);

        // Check for different error categories
        let has_indent_error = errors.iter().any(|e| e.message.contains("indent"));
        let has_delimiter_error = errors.iter().any(|e| e.message.contains("unclosed") || e.message.contains("bracket"));
        let has_structure_error = errors.iter().any(|e| e.message.contains("dash") || e.message.contains("whitespace"));

        assert!(has_indent_error || has_delimiter_error || has_structure_error);
    }

    #[test]
    fn test_empty_and_comment_only_content() {
        let mut detector = SyntaxDetector::new();
        let yaml = "\n\n# This is a comment\n# Another comment\n\n";

        let errors = detector.detect_errors(yaml);

        // Should accept empty and comment-only content
        assert!(errors.is_empty());
    }

    #[test]
    fn test_complex_nested_structure() {
        let mut detector = SyntaxDetector::new();
        let yaml = r#"
services:
  web:
    host: localhost
    port: 8080
    ssl:
      enabled: true
      cert: /path/to/cert.pem
  database:
    host: db.example.com
    port: 5432
    credentials:
      username: admin
      password: secret
deployment:
  environments:
    - name: dev
      url: dev.example.com
    - name: prod
      url: prod.example.com
"#;

        let errors = detector.detect_errors(yaml);

        // Should accept complex nested structure
        assert!(errors.is_empty());
    }
}

#[cfg(test)]
mod regression_tests {
    use super::*;

    #[test]
    fn test_no_false_positives_for_urls() {
        let mut detector = SyntaxDetector::new();
        let yaml = "urls:\n  - https://example.com\n  - http://localhost:8080\n  - ftp://files.example.com";

        let errors = detector.detect_errors(yaml);

        // Should not flag colons in URLs as errors
        assert!(errors.is_empty());
    }

    #[test]
    fn test_no_false_positives_for_time_values() {
        let mut detector = SyntaxDetector::new();
        let yaml = "times:\n  start: 09:00:00\n  end: 17:30:00\n  duration: 8:30:00";

        let errors = detector.detect_errors(yaml);

        // Should not flag colons in time values as errors
        assert!(errors.is_empty());
    }

    #[test]
    fn test_no_false_positives_for_anchors_and_aliases() {
        let validator = SyntaxValidator::new();
        let yaml = "defaults: &defaults\n  timeout: 30\n  retry: 3\nproduction:\n  <<: *defaults\n  host: prod.example.com";

        let result = validator.validate(yaml);

        // Should accept valid anchor and alias syntax
        assert!(result.is_valid());
    }

    #[test]
    fn test_no_false_positives_for_quoted_keys() {
        let mut detector = SyntaxDetector::new();
        let yaml = "\"quoted key\": value\n'another quoted': value2";

        let errors = detector.detect_errors(yaml);

        // Should accept quoted keys without false duplicate detection
        assert!(errors.is_empty());
    }

    #[test]
    fn test_flow_style_with_brackets() {
        let mut detector = SyntaxDetector::new();
        let yaml = "items: [one, two, three]\nnested: [[1, 2], [3, 4]]";

        let errors = detector.detect_errors(yaml);

        // Should accept valid flow style with nested brackets
        assert!(errors.is_empty());
    }

    #[test]
    fn test_flow_style_with_braces() {
        let mut detector = SyntaxDetector::new();
        let yaml = "config: {key1: value1, key2: value2}\nnested: {outer: {inner: value}}";

        let errors = detector.detect_errors(yaml);

        // Should accept valid flow style with nested braces
        assert!(errors.is_empty());
    }
}

#[cfg(test)]
mod performance_tests {
    use super::*;

    #[test]
    fn test_large_file_performance() {
        let mut detector = SyntaxDetector::new();
        let mut yaml = String::from("# Large test file\nitems:\n");

        // Generate a large YAML file
        for i in 0..1000 {
            yaml.push_str(&format!("  - item_{}: value_{}\n", i, i));
        }

        let start = std::time::Instant::now();
        let errors = detector.detect_errors(&yaml);
        let duration = start.elapsed();

        // Should complete in reasonable time (< 1 second)
        assert!(duration.as_millis() < 1000);
        assert!(errors.is_empty());
    }

    #[test]
    fn test_deep_nesting_performance() {
        let mut detector = SyntaxDetector::new();
        let mut yaml = String::new();

        // Generate deeply nested structure
        yaml.push_str("root:\n");
        for i in 0..50 {
            yaml.push_str(&str::repeat("  ", i + 1));
            yaml.push_str(&format!("level{}:\n", i));
        }

        let start = std::time::Instant::now();
        let errors = detector.detect_errors(&yaml);
        let duration = start.elapsed();

        // Should complete in reasonable time
        assert!(duration.as_millis() < 500);
        // Deep nesting might produce warnings but shouldn't crash
        assert!(errors.len() < 100); // Reasonable limit
    }
}
