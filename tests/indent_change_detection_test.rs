//! Comprehensive tests for indent change detection without key tokens
//!
//! These tests verify that the YAML parser correctly detects and tracks
//! indentation changes even when no key tokens are present on the line.
//!
//! Bead: bf-4ccrtq
//!
//! Test Coverage:
//! - Indent changes are detected on blank lines
//! - Indent changes are detected on comment lines
//! - Indent changes are detected on lines without keys
//! - Parser distinguishes indent-with-key from indent-without-key
//! - Detection doesn't interfere with existing key parsing

use armor::parsers::yaml::{ScopeStack};
use armor::parsers::yaml::parser::BasicParser;
use armor::parsers::yaml::parser::Parser;

// =============================================================================
// Indent Change Detection on Blank Lines
// =============================================================================

#[test]
fn test_detects_indent_change_on_blank_line() {
    let mut parser = BasicParser::new();

    let yaml = r#"
root:
  nested:
    key: value

    key2: value2
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse YAML with blank line indent transition");
}

#[test]
fn test_tracks_indent_transitions_across_blank_lines() {
    let mut parser = BasicParser::new();
    let yaml = "
root:
  level2:
    level3: value

  level2_b: value2
";

    // This should parse successfully, demonstrating that the parser
    // correctly handles blank lines with indent changes
    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse YAML with blank line indent transition");

    let value = result.unwrap();
    assert!(value.get("root").is_some());
    assert!(value["root"].get("level2").is_some());
}

#[test]
fn test_blank_line_indent_decrease() {
    let mut parser = BasicParser::new();

    let yaml = r#"
level1:
  level2:
    level3: value

level1_b: value
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should handle blank line with decreased indent");
}

// =============================================================================
// Indent Change Detection on Comment Lines
// =============================================================================

#[test]
fn test_detects_indent_change_on_comment_line() {
    let mut parser = BasicParser::new();

    let yaml = r#"
root:
  nested:
    key: value
    # Comment at indent 4
  key2: value2
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse YAML with comment line at different indent");
}

#[test]
fn test_comment_with_decreased_indent() {
    let mut parser = BasicParser::new();

    let yaml = r#"
level1:
  level2:
    level3: value
  # Comment at indent 2
  key2: value2
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should handle comment line with decreased indent");
}

#[test]
fn test_multiple_comments_at_different_indents() {
    let mut parser = BasicParser::new();

    let yaml = r#"
root:
  # Comment at indent 2
  level2:
    # Comment at indent 4
    key: value
  # Comment at indent 2 again
  key2: value2
# Comment at indent 0
key3: value3
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should handle multiple comments at different indents");
}

// =============================================================================
// Distinguish Indent-With-Key vs Indent-Without-Key
// =============================================================================

#[test]
fn test_distinguishes_key_bearing_from_non_key_lines() {
    let mut parser = BasicParser::new();

    let yaml = r#"
key1: value1
  # Non-key line at indent 2
key2: value2
key3: value3
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should distinguish key-bearing from non-key lines");
}

#[test]
fn test_tracks_key_presence_in_indent_transitions() {
    let mut stack = ScopeStack::new(2);

    // Start at indent 0, first line has a key
    // This will record a transition from 0 to 0 (no change) with a key
    // But actually, record_indent_transition only records when indent changes

    // Let's properly test indent changes:
    // 1. Start at indent 0
    // 2. Transition to indent 2 without key (e.g., comment line)
    stack.record_indent_transition(1, 2, false, "  # Comment without key");

    // 3. Transition back to indent 0 with key
    stack.record_indent_transition(2, 0, true, "key2: value2");

    // 4. Transition to indent 2 again with key
    stack.record_indent_transition(3, 2, true, "  key3: value3");

    let transitions = stack.get_indent_transitions();

    // Should have 3 transitions recorded
    assert_eq!(transitions.len(), 3, "Should have 3 indent transitions");

    // Check first transition (without key, indent 0->2)
    assert!(!transitions[0].has_key, "First transition should not have key");
    assert_eq!(transitions[0].from_indent, 0);
    assert_eq!(transitions[0].to_indent, 2);

    // Check second transition (with key, indent 2->0)
    assert!(transitions[1].has_key, "Second transition should have key");
    assert_eq!(transitions[1].from_indent, 2);
    assert_eq!(transitions[1].to_indent, 0);

    // Check third transition (with key, indent 0->2)
    assert!(transitions[2].has_key, "Third transition should have key");
    assert_eq!(transitions[2].from_indent, 0);
    assert_eq!(transitions[2].to_indent, 2);
}

#[test]
fn test_get_transitions_with_keys() {
    let mut stack = ScopeStack::new(2);

    // Record actual indent transitions:
    // 0 -> 2 (without key, comment)
    stack.record_indent_transition(1, 2, false, "  # Comment");

    // 2 -> 4 (with key)
    stack.record_indent_transition(2, 4, true, "    key2: value2");

    // 4 -> 2 (without key, blank line)
    stack.record_indent_transition(3, 2, false, "");

    let with_keys = stack.get_transitions_with_keys();
    let without_keys = stack.get_transitions_without_keys();

    assert_eq!(with_keys.len(), 1, "Should have 1 transition with key");
    assert_eq!(without_keys.len(), 2, "Should have 2 transitions without keys");
}

// =============================================================================
// Detection Doesn't Interfere With Existing Key Parsing
// =============================================================================

#[test]
fn test_indent_tracking_doesnt_break_duplicate_detection() {
    let mut parser = BasicParser::strict();

    let yaml = r#"
root:
  key1: value1

  key2: value2
  key2: duplicate
"#;

    let result = parser.validate_str(yaml);
    assert!(!result.is_valid(), "Should still detect duplicate keys");

    // Verify the error mentions duplicate
    let error_messages: Vec<String> = result.errors.iter()
        .map(|e| e.message.clone())
        .collect();
    assert!(error_messages.iter().any(|msg| msg.contains("duplicate")),
            "Error should mention duplicate key");
}

#[test]
fn test_indent_tracking_preserves_scope_isolation() {
    let mut parser = BasicParser::strict();

    let yaml = r#"
scope1:
  key: value1

scope2:
  key: value2
"#;

    let result = parser.validate_str(yaml);
    assert!(result.is_valid(), "Should allow same key in different scopes");
}

#[test]
fn test_parser_with_complex_indent_changes() {
    let mut parser = BasicParser::new();

    let yaml = r#"
level1:
  # Comment
  level2:
    # Another comment

    level3: value
  # Back to level 2
  key2: value2
# Root level
key3: value3
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should handle complex indent changes with comments");
}

// =============================================================================
// Edge Cases for Indent Change Detection
// =============================================================================

#[test]
fn test_indent_change_on_only_blank_lines() {
    let mut parser = BasicParser::new();

    let yaml = r#"
key1: value1


key2: value2
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should handle multiple blank lines");
}

#[test]
fn test_indent_change_from_blank_to_content() {
    let mut parser = BasicParser::new();

    let yaml = r#"
root:
  level2:

    key: value
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should handle blank line at same indent as content");
}

#[test]
fn test_sequence_with_indent_changes() {
    let mut parser = BasicParser::new();

    let yaml = r#"
items:
  - name: item1

    value: 100
  - name: item2

    value: 200
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should handle sequences with blank lines");
}

#[test]
fn test_deeply_nested_with_indent_transitions() {
    let mut parser = BasicParser::new();

    let yaml = r#"
level1:
  level2:
    level3:
      level4:
        key: value
      # Back to level4
      key2: value2
    # Back to level3
    key3: value3
  # Back to level2
  key4: value4
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should handle deeply nested structures with comments");
}

// =============================================================================
// Real-World Scenarios
// =============================================================================

#[test]
fn test_real_world_config_with_blank_lines() {
    let mut parser = BasicParser::new();

    let yaml = r#"
# Database configuration

database:
  host: localhost
  port: 5432

  # Connection pool settings
  pool:
    min: 2
    max: 10

# Server configuration

server:
  port: 8080
  host: 0.0.0.0
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse real-world config with blank lines and comments");
}

#[test]
fn test_kubernetes_style_yaml() {
    let mut parser = BasicParser::new();

    let yaml = r#"
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
    - name: app
      image: myimage:latest

      # Resource limits
      resources:
        limits:
          memory: "128Mi"
          cpu: "500m"
    - name: sidecar
      image: sidecar:latest
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse Kubernetes-style YAML");
}

#[test]
fn test_complex_nested_sequence() {
    let mut parser = BasicParser::new();

    let yaml = r#"
services:
  - name: web
    config:
      port: 8080

      ssl:
        enabled: true

    endpoints:
      - path: /api
        method: GET

  - name: database
    config:
      port: 5432
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse complex nested sequences with blank lines");
}

// =============================================================================
// Integration Tests with ScopeStack
// =============================================================================

#[test]
fn test_scope_stack_records_all_indent_changes() {
    let mut stack = ScopeStack::new(2);

    // Simulate parsing a YAML document with various indent changes
    // The initial last_indent is 0

    // Line 1: indent 0 with key (first line, no transition from initial 0)
    stack.record_indent_transition(1, 0, true, "root:");

    // Line 2: indent 2 without key (transition from 0→2)
    stack.record_indent_transition(2, 2, false, "  ");

    // Line 3: indent 2 with key (no transition, same indent)
    stack.record_indent_transition(3, 2, true, "  key1: value1");

    // Line 4: indent 0 without key (transition from 2→0)
    stack.record_indent_transition(4, 0, false, "");

    // Line 5: indent 0 with key (no transition, same indent)
    stack.record_indent_transition(5, 0, true, "key2: value2");

    // Line 6: indent 4 with key (transition from 0→4)
    stack.record_indent_transition(6, 4, true, "    nested: value");

    // Line 7: indent 2 without key (transition from 4→2)
    stack.record_indent_transition(7, 2, false, "  ");

    let transitions = stack.get_indent_transitions();

    // We should have recorded the actual indent changes:
    // - 0→2 (line 2, without key)
    // - 2→0 (line 4, without key)
    // - 0→4 (line 6, with key)
    // - 4→2 (line 7, without key)
    assert_eq!(transitions.len(), 4, "Should record actual indent transitions (only when indent changes)");

    // Verify the types of transitions
    let with_keys = stack.get_transitions_with_keys();
    let without_keys = stack.get_transitions_without_keys();

    // Only one transition (0→4 on line 6) has a key
    assert_eq!(with_keys.len(), 1, "Should have 1 transition with a key");
    // Three transitions without keys (0→2, 2→0, 4→2)
    assert_eq!(without_keys.len(), 3, "Should have 3 transitions without keys");

    // Verify the actual transitions
    assert_eq!(transitions[0].from_indent, 0);
    assert_eq!(transitions[0].to_indent, 2);
    assert!(!transitions[0].has_key);

    assert_eq!(transitions[1].from_indent, 2);
    assert_eq!(transitions[1].to_indent, 0);
    assert!(!transitions[1].has_key);

    assert_eq!(transitions[2].from_indent, 0);
    assert_eq!(transitions[2].to_indent, 4);
    assert!(transitions[2].has_key);

    assert_eq!(transitions[3].from_indent, 4);
    assert_eq!(transitions[3].to_indent, 2);
    assert!(!transitions[3].has_key);
}

#[test]
fn test_indent_change_detection_comprehensive() {
    let mut stack = ScopeStack::new(2);

    // Test comprehensive scenario
    stack.record_indent_transition(1, 0, true, "key1: value1");
    stack.record_indent_transition(2, 2, false, "  # Comment");
    stack.record_indent_transition(3, 4, true, "    nested: value");
    stack.record_indent_transition(4, 4, false, "    ");
    stack.record_indent_transition(5, 2, false, "");
    stack.record_indent_transition(6, 2, true, "  key2: value2");
    stack.record_indent_transition(7, 0, true, "key3: value3");

    let transitions = stack.get_indent_transitions();

    // Verify we can distinguish different types of transitions
    let increases: Vec<_> = transitions.iter().filter(|t| t.is_increase()).collect();
    let decreases: Vec<_> = transitions.iter().filter(|t| t.is_decrease()).collect();

    // Should have indent increases and decreases
    assert!(!increases.is_empty(), "Should have indent increases");
    assert!(!decreases.is_empty(), "Should have indent decreases");

    // Verify we can get transitions with and without keys
    let with_keys = stack.get_transitions_with_keys();
    let without_keys = stack.get_transitions_without_keys();

    assert!(!with_keys.is_empty(), "Should have transitions with keys");
    assert!(!without_keys.is_empty(), "Should have transitions without keys");
}

#[test]
fn test_last_indent_tracking() {
    let mut stack = ScopeStack::new(2);

    // Initially, last indent should be 0
    assert_eq!(stack.get_last_indent(), 0, "Initial last indent should be 0");

    // Record some transitions
    stack.record_indent_transition(1, 2, true, "  key: value");
    assert_eq!(stack.get_last_indent(), 2, "Last indent should update to 2");

    stack.record_indent_transition(2, 4, false, "    ");
    assert_eq!(stack.get_last_indent(), 4, "Last indent should update to 4");

    stack.record_indent_transition(3, 0, true, "key2: value");
    assert_eq!(stack.get_last_indent(), 0, "Last indent should update to 0");
}

#[test]
fn test_clear_indent_transitions() {
    let mut stack = ScopeStack::new(2);

    // Add some transitions
    stack.record_indent_transition(1, 2, true, "  key: value");
    stack.record_indent_transition(2, 4, false, "    ");

    assert!(!stack.get_indent_transitions().is_empty(), "Should have transitions");

    // Clear transitions
    stack.clear_indent_transitions();

    assert!(stack.get_indent_transitions().is_empty(), "Transitions should be cleared");
    assert_eq!(stack.get_last_indent(), 0, "Last indent should be reset to 0");
}
