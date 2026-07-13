//! Comprehensive verification tests for sequence scope handling
//!
//! These tests verify that sequence scope tracking works correctly in YAML parsing,
//! especially for nested sequences and mixed mappings.
//!
//! Bead: bf-1ccile
//!
//! Test Coverage:
//! - Sequence scope entry is tracked correctly
//! - Sequence scope exit maintains proper scope state
//! - Nested sequence/mapping combinations work
//! - Test coverage demonstrates correctness

use armor::parsers::yaml::{ScopeStack};
use armor::parsers::yaml::parser::BasicParser;
use armor::parsers::yaml::parser::Parser;

// =============================================================================
// Sequence Scope Entry Tracking
// =============================================================================

#[test]
fn test_sequence_scope_entry_sets_in_sequence_context() {
    let mut stack = ScopeStack::new(2);

    // Before entry: should not be in sequence context
    assert!(!stack.in_sequence_context());

    // Enter sequence scope
    stack.enter_sequence_scope(2, 1);

    // After entry: should be in sequence context
    assert!(stack.in_sequence_context());
    assert_eq!(stack.current_indent(), 2);
}

#[test]
fn test_sequence_scope_entry_clears_previous_keys() {
    let mut stack = ScopeStack::new(2);

    // Add keys to first sequence item
    stack.enter_sequence_scope(2, 1);
    stack.add_key("name", 2).unwrap();
    stack.add_key("value", 3).unwrap();
    assert_eq!(stack.current_scope_ref().key_count(), 2);

    // Enter second sequence item at same indent
    stack.enter_sequence_scope(2, 5);

    // Keys should be cleared for new item
    assert_eq!(stack.current_scope_ref().key_count(), 0);
    assert!(!stack.contains_key("name"));
    assert!(!stack.contains_key("value"));
}

#[test]
fn test_sequence_scope_entry_increments_item_id() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    let id1 = stack.current_scope_ref().sequence_item_id;

    stack.enter_sequence_scope(2, 3);
    let id2 = stack.current_scope_ref().sequence_item_id;

    stack.enter_sequence_scope(2, 5);
    let id3 = stack.current_scope_ref().sequence_item_id;

    assert_eq!(id1, Some(1));
    assert_eq!(id2, Some(2));
    assert_eq!(id3, Some(3));
}

#[test]
fn test_sequence_scope_entry_creates_isolated_scope() {
    let mut stack = ScopeStack::new(2);

    // First sequence item
    stack.enter_sequence_scope(2, 1);
    stack.add_key("host", 2).unwrap();
    stack.add_key("port", 3).unwrap();

    // Second sequence item at same indent
    stack.enter_sequence_scope(2, 5);

    // Should be able to add same keys without duplicate error
    stack.add_key("host", 6).unwrap();
    stack.add_key("port", 7).unwrap();

    assert_eq!(stack.current_scope_ref().key_count(), 2);
}

// =============================================================================
// Sequence Scope Exit Behavior
// =============================================================================

#[test]
fn test_sequence_scope_exit_updates_scope_state() {
    let mut stack = ScopeStack::new(2);

    // Create parent mapping scope
    stack.enter_scope(2, 1, Some("items".to_string()));

    // Enter sequence scope
    stack.enter_sequence_scope(4, 2);
    assert!(stack.in_sequence_context());
    stack.add_key("name", 3).unwrap();

    // Exit from sequence back to parent mapping
    stack.exit_to_scope(2);

    // Should be back at items scope
    assert!(!stack.in_sequence_context());
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.get_scope_path(), "items");
}

#[test]
fn test_sequence_scope_exit_clears_sequence_keys() {
    let mut stack = ScopeStack::new(2);

    // Parent scope
    stack.enter_scope(2, 1, Some("parent".to_string()));
    stack.add_key("parent_key", 2).unwrap();

    // Sequence scope
    stack.enter_sequence_scope(4, 3);
    stack.add_key("seq_key", 4).unwrap();

    // Exit from sequence
    stack.exit_to_scope(2);

    // Parent key should still exist
    assert!(stack.contains_key("parent_key"));
    // Sequence key should be gone
    assert!(!stack.contains_key("seq_key"));
}

#[test]
fn test_sequence_scope_exit_to_different_indents() {
    let mut stack = ScopeStack::new(2);

    // Create nested structure: level1 (2) -> sequence (4) -> level3 (6)
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_sequence_scope(4, 2);
    stack.enter_scope(6, 3, Some("level3".to_string()));

    // Exit from level3 to level1 (skipping sequence)
    stack.exit_to_scope(2);

    // Should be at level1 scope
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.get_scope_path(), "level1");
    assert!(!stack.in_sequence_context());
}

// =============================================================================
// Sequences Nested in Mappings
// =============================================================================

#[test]
fn test_sequence_nested_in_mapping_scope_isolation() {
    let mut stack = ScopeStack::new(2);

    // Create parent mapping
    stack.enter_scope(2, 1, Some("items".to_string()));

    // First sequence item
    stack.enter_sequence_scope(4, 2);
    stack.add_key("id", 3).unwrap();
    stack.add_key("name", 4).unwrap();

    // Second sequence item
    stack.enter_sequence_scope(4, 5);
    stack.add_key("id", 6).unwrap();
    stack.add_key("name", 7).unwrap();

    // Both items should have same keys without conflict
    assert_eq!(stack.current_scope_ref().key_count(), 2);
    assert!(stack.contains_key("id"));
    assert!(stack.contains_key("name"));
}

#[test]
fn test_sequence_nested_in_mapping_parent_context_preserved() {
    let mut stack = ScopeStack::new(2);

    // Parent mapping
    stack.enter_scope(2, 1, Some("services".to_string()));
    stack.add_key("version", 2).unwrap();

    // Sequence nested in mapping
    stack.enter_sequence_scope(4, 3);
    stack.add_key("host", 4).unwrap();

    // Exit to parent
    stack.exit_to_scope(2);

    // Parent context should be preserved
    assert_eq!(stack.get_scope_path(), "services");
    assert!(stack.contains_key("version"));
    assert!(!stack.contains_key("host")); // Host key should be gone
}

#[test]
fn test_multiple_sequences_in_same_mapping() {
    let mut stack = ScopeStack::new(2);

    // Parent mapping
    stack.enter_scope(2, 1, Some("config".to_string()));

    // First sequence
    stack.enter_sequence_scope(4, 2);
    stack.add_key("item1", 3).unwrap();
    stack.exit_to_scope(2);

    // Second sequence (different key in parent)
    stack.enter_sequence_scope(4, 4);
    stack.add_key("item2", 5).unwrap();

    // Should be in second sequence
    assert!(stack.in_sequence_context());
    assert!(stack.contains_key("item2"));
    assert!(!stack.contains_key("item1"));
}

#[test]
fn test_deeply_nested_sequence_in_mapping() {
    let mut stack = ScopeStack::new(2);

    // Create: outer (2) -> middle (4) -> sequence (6)
    stack.enter_scope(2, 1, Some("outer".to_string()));
    stack.enter_scope(4, 2, Some("middle".to_string()));
    stack.enter_sequence_scope(6, 3);

    stack.add_key("deep_key", 4).unwrap();

    // Verify we're at sequence scope
    assert!(stack.in_sequence_context());
    assert_eq!(stack.current_indent(), 6);
    assert_eq!(stack.depth(), 4); // root + outer + middle + sequence
}

// =============================================================================
// Mappings Nested in Sequences
// =============================================================================

#[test]
fn test_mapping_nested_in_sequence_scope() {
    let mut stack = ScopeStack::new(2);

    // Enter sequence scope
    stack.enter_sequence_scope(2, 1);

    // Add keys to sequence item
    stack.add_key("name", 2).unwrap();

    // Enter nested mapping within sequence
    stack.enter_scope(4, 3, Some("config".to_string()));
    stack.add_key("enabled", 4).unwrap();
    stack.add_key("timeout", 5).unwrap();

    // Should be in nested mapping
    assert!(!stack.in_sequence_context()); // Current scope is mapping, not sequence
    assert_eq!(stack.get_scope_path(), "config");
    assert!(stack.contains_key("enabled"));
    assert!(stack.contains_key("timeout"));
}

#[test]
fn test_mapping_nested_in_sequence_isolation_between_items() {
    let mut stack = ScopeStack::new(2);

    // First sequence item with nested mapping
    stack.enter_sequence_scope(2, 1);
    stack.enter_scope(4, 2, Some("config".to_string()));
    stack.add_key("key1", 3).unwrap();
    stack.exit_to_scope(2); // Exit mapping

    // Second sequence item with nested mapping
    stack.enter_sequence_scope(2, 5);
    stack.enter_scope(4, 6, Some("config".to_string()));

    // Should be able to add same key
    stack.add_key("key1", 7).unwrap();
    assert!(stack.contains_key("key1"));
}

#[test]
fn test_multiple_mappings_in_same_sequence_item() {
    let mut stack = ScopeStack::new(2);

    // Sequence item
    stack.enter_sequence_scope(2, 1);

    // First nested mapping
    stack.enter_scope(4, 2, Some("config1".to_string()));
    stack.add_key("setting1", 3).unwrap();
    stack.exit_to_scope(2);

    // Second nested mapping (sibling in sequence item)
    stack.enter_scope(4, 4, Some("config2".to_string()));
    stack.add_key("setting2", 5).unwrap();

    // Should have second mapping's keys
    assert!(stack.contains_key("setting2"));
    assert!(!stack.contains_key("setting1"));
}

#[test]
fn test_deeply_nested_mapping_in_sequence() {
    let mut stack = ScopeStack::new(2);

    // Sequence scope
    stack.enter_sequence_scope(2, 1);

    // Deeply nested mapping: level1 (4) -> level2 (6) -> level3 (8)
    stack.enter_scope(4, 2, Some("level1".to_string()));
    stack.enter_scope(6, 3, Some("level2".to_string()));
    stack.enter_scope(8, 4, Some("level3".to_string()));

    stack.add_key("deep_value", 5).unwrap();

    // Verify deep nesting
    assert_eq!(stack.depth(), 5); // root + seq + level1 + level2 + level3
    assert_eq!(stack.get_scope_path(), "level1.level2.level3");
}

// =============================================================================
// Complex Combinations
// =============================================================================

#[test]
fn test_sequence_mapping_sequence_pattern() {
    let mut stack = ScopeStack::new(2);

    // Pattern: mapping -> sequence -> mapping -> sequence

    // Root mapping
    stack.enter_scope(2, 1, Some("services".to_string()));

    // Sequence in mapping
    stack.enter_sequence_scope(4, 2);
    stack.add_key("name", 3).unwrap();

    // Mapping in sequence
    stack.enter_scope(6, 4, Some("config".to_string()));
    stack.add_key("port", 5).unwrap();

    // Sequence in mapping in sequence
    stack.enter_sequence_scope(8, 6);
    stack.add_key("item", 7).unwrap();

    // Verify complex nesting
    assert!(stack.in_sequence_context());
    assert_eq!(stack.depth(), 5);
    assert!(stack.contains_key("item"));
}

#[test]
fn test_alternating_sequence_and_mapping() {
    let mut stack = ScopeStack::new(2);

    // Sequence
    stack.enter_sequence_scope(2, 1);
    stack.add_key("seq1_key", 2).unwrap();

    // Mapping in sequence
    stack.enter_scope(4, 3, Some("map1".to_string()));
    stack.add_key("map1_key", 4).unwrap();

    // Exit to sequence, enter new sequence
    stack.exit_to_scope(2);
    stack.enter_sequence_scope(2, 5);
    stack.add_key("seq2_key", 6).unwrap();

    // Mapping in second sequence
    stack.enter_scope(4, 7, Some("map2".to_string()));
    stack.add_key("map2_key", 8).unwrap();

    // Verify second mapping scope
    assert_eq!(stack.get_scope_path(), "map2");
    assert!(stack.contains_key("map2_key"));
    assert!(!stack.contains_key("map1_key")); // First mapping's keys gone
    assert!(!stack.contains_key("seq1_key")); // First sequence's keys gone
}

#[test]
fn test_sibling_sequences_with_nested_mappings() {
    let mut stack = ScopeStack::new(2);

    // Parent mapping
    stack.enter_scope(2, 1, Some("items".to_string()));

    // First sequence item with nested mapping
    stack.enter_sequence_scope(4, 2);
    stack.enter_scope(6, 3, Some("config".to_string()));
    stack.add_key("key1", 4).unwrap();
    stack.exit_to_scope(2); // Exit to items
    stack.exit_to_scope(2); // Exit to sequence level

    // Second sequence item with nested mapping
    stack.enter_sequence_scope(4, 6);
    stack.enter_scope(6, 7, Some("config".to_string()));

    // Should be able to add same key
    stack.add_key("key1", 8).unwrap();
    assert!(stack.contains_key("key1"));
}

#[test]
fn test_complex_real_world_structure() {
    let mut stack = ScopeStack::new(2);

    // Simulate:
    // application:
    //   services:
    //     - name: web
    //       config:
    //         port: 8080
    //         host: localhost
    //     - name: database
    //       config:
    //         port: 5432
    //         host: db.example.com

    stack.enter_scope(2, 1, Some("application".to_string()));
    stack.enter_scope(4, 2, Some("services".to_string()));

    // First service item
    stack.enter_sequence_scope(6, 3);
    stack.add_key("name", 4).unwrap();
    stack.enter_scope(8, 5, Some("config".to_string()));
    stack.add_key("port", 6).unwrap();
    stack.add_key("host", 7).unwrap();

    // Verify first service
    assert_eq!(stack.get_scope_path(), "application.services.config");
    assert!(stack.contains_key("port"));
    assert!(stack.contains_key("host"));

    // Exit to services level for second item
    stack.exit_to_scope(4);

    // Second service item
    stack.enter_sequence_scope(6, 9);
    stack.add_key("name", 10).unwrap();
    stack.enter_scope(8, 11, Some("config".to_string()));

    // Same keys should be OK
    stack.add_key("port", 12).unwrap();
    stack.add_key("host", 13).unwrap();

    // Verify second service has same keys
    assert!(stack.contains_key("port"));
    assert!(stack.contains_key("host"));
}

// =============================================================================
// Integration with Parser
// =============================================================================

#[test]
fn test_parser_sequence_nested_in_mapping() {
    let parser = BasicParser::new();

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
    assert!(result.is_success(), "Should parse sequence nested in mapping");

    let value = result.unwrap();
    let items = value["items"].as_sequence().unwrap();
    assert_eq!(items.len(), 3);
}

#[test]
fn test_parser_mapping_nested_in_sequence() {
    let parser = BasicParser::new();

    let yaml = r#"
- name: service1
  config:
    host: localhost
    port: 8080
- name: service2
  config:
    host: example.com
    port: 443
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse mapping nested in sequence");

    let value = result.unwrap();
    let items = value.as_sequence().unwrap();
    assert_eq!(items.len(), 2);
}

#[test]
fn test_parser_complex_nested_structure() {
    let parser = BasicParser::new();

    let yaml = r#"
services:
  web:
    endpoints:
      - path: /api
        method: GET
        config:
          timeout: 30
          cache: true
      - path: /health
        method: GET
        config:
          timeout: 10
          cache: false
  database:
    endpoints:
      - path: /query
        method: POST
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse complex nested structure");

    let value = result.unwrap();
    let services = &value["services"];
    assert!(services.get("web").is_some());
    assert!(services.get("database").is_some());
}

#[test]
fn test_parser_no_false_duplicates_in_sequences_simple() {
    let parser = BasicParser::strict();

    let yaml = r#"
items:
  - name: First
    value: 1
  - name: Second
    value: 2
  - name: Third
    value: 3
"#;

    let validation = parser.validate_str(yaml);
    if !validation.is_valid() {
        for error in &validation.errors {
            eprintln!("Validation error: {}", error.message);
        }
    }
    assert!(validation.is_valid(), "Should not report false duplicates in sequence items with different keys");
}

#[test]
fn test_parser_detects_duplicate_within_sequence_item() {
    let parser = BasicParser::strict();

    let yaml = r#"
items:
  - id: 1
    name: First
    name: Duplicate
"#;

    let validation = parser.validate_str(yaml);
    assert!(!validation.is_valid(), "Should detect duplicate within same sequence item");
}

#[test]
fn test_parser_sequence_at_root_level() {
    let parser = BasicParser::new();

    let yaml = r#"
- name: item1
  value: 100
- name: item2
  value: 200
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse root-level sequence");

    let value = result.unwrap();
    let items = value.as_sequence().unwrap();
    assert_eq!(items.len(), 2);
}

#[test]
fn test_parser_mixed_root_sequences_and_mappings() {
    let parser = BasicParser::new();

    let yaml = r#"
version: "1.0"
items:
  - name: first
    value: 1
settings:
  debug: true
  verbose: false
tags:
  - production
  - api
"#;

    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse mixed root-level structures");

    let value = result.unwrap();
    assert!(value.get("version").is_some());
    assert!(value.get("items").is_some());
    assert!(value.get("settings").is_some());
    assert!(value.get("tags").is_some());
}

// =============================================================================
// Edge Cases
// =============================================================================

#[test]
fn test_sequence_with_empty_items() {
    let mut stack = ScopeStack::new(2);

    // Empty sequence item (just dash, no content)
    stack.enter_sequence_scope(2, 1);

    // Should still be in sequence context
    assert!(stack.in_sequence_context());
    assert_eq!(stack.current_scope_ref().key_count(), 0);
}

#[test]
fn test_sequence_with_single_key() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    stack.add_key("only_key", 2).unwrap();

    assert_eq!(stack.current_scope_ref().key_count(), 1);
    assert!(stack.contains_key("only_key"));
}

#[test]
fn test_sequence_entry_after_deep_nesting() {
    let mut stack = ScopeStack::new(2);

    // Create deep nesting
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(6, 3, Some("level3".to_string()));

    // Enter sequence at deep level
    stack.enter_sequence_scope(8, 4);

    assert!(stack.in_sequence_context());
    assert_eq!(stack.current_indent(), 8);
}

#[test]
fn test_sequence_exit_from_deeply_nested() {
    let mut stack = ScopeStack::new(2);

    // Create deep nesting with sequence
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_sequence_scope(4, 2);
    stack.enter_scope(6, 3, Some("level3".to_string()));
    stack.enter_scope(8, 4, Some("level4".to_string()));

    // Exit from level4 directly to level1
    stack.exit_to_scope(2);

    // Should be at level1
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.get_scope_path(), "level1");
    assert!(!stack.in_sequence_context());
}

#[test]
fn test_multiple_consecutive_sequence_entries_same_indent() {
    let mut stack = ScopeStack::new(2);

    // Multiple sequence entries at same indent without keys
    stack.enter_sequence_scope(2, 1);
    stack.enter_sequence_scope(2, 2);
    stack.enter_sequence_scope(2, 3);

    // Should have incremented item IDs each time
    assert_eq!(stack.current_scope_ref().sequence_item_id, Some(3));
}

#[test]
fn test_sequence_entry_preserves_parent_scopes() {
    let mut stack = ScopeStack::new(2);

    // Create parent scopes
    stack.enter_scope(2, 1, Some("parent1".to_string()));
    stack.enter_scope(4, 2, Some("parent2".to_string()));

    // Add key to parent2
    stack.add_key("parent_key", 3).unwrap();

    // Enter sequence scope
    stack.enter_sequence_scope(6, 4);

    // Parent scopes should still be in stack
    assert_eq!(stack.depth(), 4); // root + parent1 + parent2 + sequence

    // Exit sequence
    stack.exit_to_scope(4);

    // Parent key should still exist
    assert!(stack.contains_key("parent_key"));
    assert_eq!(stack.get_scope_path(), "parent1.parent2");
}
