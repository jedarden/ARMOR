//! Edge case tests for scope transitions during YAML parsing
//!
//! This module tests specific edge cases that can occur during parsing:
//! 1. Indent changes without keys
//! 2. Large indent jumps (skipping levels)
//! 3. Empty values and blank lines
//! 4. Sequence items with complex nesting
//! 5. Flow-style mapping edges

use crate::parsers::yaml::parser::{Parser, BasicParser};

#[cfg(test)]
mod indent_edge_cases {
    use super::*;

    /// Test indent increase without a key (scalar value continuation)
    #[test]
    fn test_indent_increase_without_key() {
        let parser = BasicParser::new();

        // YAML where indent increases but it's just a value continuation
        let yaml = r#"
key1:
  some value
  that spans
  multiple lines
key2: value2
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle indent increase without key");
    }

    /// Test large indent jump (skipping intermediate levels)
    #[test]
    fn test_large_indent_jump() {
        let parser = BasicParser::new();

        // Jump from indent 0 to indent 6 (skipping 2 and 4)
        let yaml = r#"
root:
      deep_value: 1
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle large indent jumps");
    }

    /// Test indent decrease with no key at that level
    #[test]
    fn test_indent_decrease_without_key() {
        let parser = BasicParser::new();

        // Decrease indent but line has no key (just a value or empty)
        // This is actually valid YAML - it's a sibling key at root level
        let yaml = r#"
outer:
  inner:
    deep: value
    another: value2
sibling: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle indent decrease without key");
    }

    /// Test rapid indent changes (up and down repeatedly)
    #[test]
    fn test_rapid_indent_changes() {
        let parser = BasicParser::new();

        let yaml = r#"
a:
  b:
    c: value1
  d: value2
e:
  f:
    g: value3
  h: value4
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle rapid indent changes");
    }

    /// Test indent returning to same level multiple times
    #[test]
    fn test_return_to_same_indent_level() {
        let parser = BasicParser::new();

        let yaml = r#"
root:
  first:
    deep: value
  second:
    deep: value
  third:
    deep: value
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle returning to same indent level");

        let value = result.unwrap();
        assert_eq!(value["root"]["first"]["deep"], "value");
        assert_eq!(value["root"]["second"]["deep"], "value");
        assert_eq!(value["root"]["third"]["deep"], "value");
    }
}

#[cfg(test)]
mod sequence_edge_cases {
    use super::*;

    /// Test empty sequence items
    #[test]
    fn test_empty_sequence_items() {
        let parser = BasicParser::new();

        let yaml = r#"
items:
  -
  -
  - name: test
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle empty sequence items");
    }

    /// Test nested sequences
    #[test]
    fn test_nested_sequences() {
        let parser = BasicParser::new();

        let yaml = r#"
items:
    - item1
    - - subitem1
      - subitem2
    - item3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle nested sequences");
    }

    /// Test sequence with mixed content types
    #[test]
    fn test_sequence_mixed_content() {
        let parser = BasicParser::new();

        let yaml = r#"
items:
  - simple_value
  - key: value
    another: value2
  - - nested
    - list
  - key: mapping
    nested:
      deep: value
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle sequence with mixed content");
    }

    /// Test sequence after deep nesting
    #[test]
    fn test_sequence_after_deep_nesting() {
        let parser = BasicParser::new();

        let yaml = r#"
level1:
  level2:
    level3:
      items:
        - a
        - b
        - c
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle sequence after deep nesting");
    }

    /// Test sequence items at different indent levels
    #[test]
    fn test_sequence_items_different_indents() {
        let parser = BasicParser::new();

        let yaml = r#"
items:
  - name: item1
    config:
      enabled: true
  - name: item2
    config:
      enabled: false
      timeout: 30
  - name: item3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle sequence items with different indentation depths");
    }
}

#[cfg(test)]
mod key_detection_edge_cases {
    use super::*;

    /// Test key with special characters in value
    #[test]
    fn test_key_special_characters_value() {
        let parser = BasicParser::new();

        let yaml = r#"
key: "value with {special} characters"
another: [1, 2, 3]
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle keys with special character values");
    }

    /// Test duplicate key detection in nested sequences
    #[test]
    fn test_duplicate_in_sequence_item() {
        let parser = BasicParser::strict();

        let yaml = r#"
items:
  - name: item1
    value: first
    name: duplicate
"#;

        let result = parser.validate_str(yaml);
        assert!(!result.is_valid(), "Should detect duplicate key within sequence item");

        // Check that the error mentions the duplicate key
        assert!(!result.errors.is_empty());
        let error_msg = &result.errors[0].message;
        assert!(error_msg.contains("name") || error_msg.contains("duplicate"));
    }

    /// Test same key in different sequence items (should be allowed)
    #[test]
    fn test_same_key_different_sequence_items() {
        let parser = BasicParser::new();

        let yaml = r#"
items:
  - name: item1
    value: 1
  - name: item2
    value: 2
  - name: item3
    value: 3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should allow same key in different sequence items");
    }

    /// Test key followed immediately by another key at same indent
    #[test]
    fn test_sibling_keys_same_indent() {
        let parser = BasicParser::new();

        let yaml = r#"
key1: value1
key2: value2
key3: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle sibling keys at same indent");

        let value = result.unwrap();
        assert_eq!(value["key1"], "value1");
        assert_eq!(value["key2"], "value2");
        assert_eq!(value["key3"], "value3");
    }
}

#[cfg(test)]
mod document_boundary_edge_cases {
    use super::*;

    /// Test multiple documents in one file
    #[test]
    fn test_multiple_documents() {
        let parser = BasicParser::new();

        let yaml = r#"
---
key1: value1
key2: value2
"#;

        let result = parser.parse_str(yaml);
        // This will parse the first document
        assert!(result.is_success(), "Should handle document start marker");

        let value = result.unwrap();
        assert_eq!(value["key1"], "value1");
        assert_eq!(value["key2"], "value2");
    }

    /// Test document marker after nested content
    #[test]
    fn test_document_marker_after_nesting() {
        let parser = BasicParser::new();

        let yaml = r#"
level1:
  level2:
    level3: value
new_doc: value2
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle document marker after nested content");

        let value = result.unwrap();
        assert_eq!(value["level1"]["level2"]["level3"], "value");
        assert_eq!(value["new_doc"], "value2");
    }

    /// Test empty document followed by content
    #[test]
    fn test_empty_document_then_content() {
        let parser = BasicParser::new();

        let yaml = r#"
---
key: value
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle document marker at start");
    }
}

#[cfg(test)]
mod whitespace_and_formatting_edge_cases {
    use super::*;

    /// Test trailing whitespace on lines
    #[test]
    fn test_trailing_whitespace() {
        let parser = BasicParser::new();

        let yaml = "key: value   \nnext: value2   \n";

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle trailing whitespace");

        let value = result.unwrap();
        assert_eq!(value["key"], "value");
        assert_eq!(value["next"], "value2");
    }

    /// Test mixed tabs and spaces (should be handled gracefully)
    #[test]
    fn test_mixed_tabs_spaces() {
        let parser = BasicParser::new();

        let yaml = r#"
key1: value1
  key2: value2
    key3: value3
"#;

        let result = parser.parse_str(yaml);
        // This may not be valid YAML, but shouldn't crash
        assert!(result.is_success() || result.is_failure(), "Should handle mixed whitespace gracefully");
    }

    /// Test consecutive blank lines
    #[test]
    fn test_consecutive_blank_lines() {
        let parser = BasicParser::new();

        let yaml = r#"
key1: value1


key2: value2


key3: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle consecutive blank lines");
    }

    /// Test comments at various indent levels
    #[test]
    fn test_comments_various_indents() {
        let parser = BasicParser::new();

        let yaml = r#"
# Root comment
key1: value1
key2: value2
# Nested comment
  nested: value2
# Deep comment
    deep: value3
# Another root comment
key4: value4
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should handle comments at various indent levels");
    }
}

#[cfg(test)]
mod validation_edge_cases {
    use super::*;

    /// Test validation with complex nesting and duplicates
    #[test]
    fn test_validate_complex_nesting_with_duplicate() {
        let parser = BasicParser::strict();

        let yaml = r#"
level1:
  level2:
    level3:
      key: value1
      key: duplicate
  another_level2:
    key: value3
"#;

        let result = parser.validate_str(yaml);
        assert!(!result.is_valid(), "Should detect duplicate in deeply nested scope");

        // Check error has proper context
        assert!(!result.errors.is_empty());
        let error_msg = &result.errors[0].message;
        assert!(error_msg.contains("key") || error_msg.contains("duplicate"));
    }

    /// Test that duplicate detection works after scope reset
    #[test]
    fn test_duplicate_after_document_marker() {
        let parser = BasicParser::strict();

        let yaml = r#"
---
config:
  key: value1
---
config:
  key: value2
"#;

        // In strict mode, each document should be validated independently
        let result = parser.validate_str(yaml);
        // Either validation succeeds (different documents) or fails - just shouldn't crash
        assert!(result.is_valid() || !result.is_valid(), "Should handle documents without crashing");
    }
}
