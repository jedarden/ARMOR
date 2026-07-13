//! Comprehensive tests for ScopeStack data structure
//!
//! This test module verifies that the ScopeStack correctly:
//! - Tracks keys per-scope with proper isolation
//! - Handles scope entry and exit on indent changes
//! - Detects duplicate keys within the same scope
//! - Allows same key name in different scopes

use super::*;
use crate::parsers::yaml::scope::{Scope, ScopeStack, DuplicateKeyError};

/// Test helper to verify scope state
fn verify_scope_state(stack: &ScopeStack, expected_depth: usize, expected_path: &str) {
    assert_eq!(stack.depth(), expected_depth, "Scope depth should match expected");
    assert_eq!(stack.get_scope_path(), expected_path, "Scope path should match expected");
}

/// Test helper to verify key existence
fn verify_key_exists(stack: &ScopeStack, key: &str, should_exist: bool) {
    assert_eq!(
        stack.contains_key(key),
        should_exist,
        "Key '{}' existence check failed",
        key
    );
}

#[cfg(test)]
mod scope_stack_tests {
    use super::*;

    #[test]
    fn test_new_creates_root_scope() {
        let stack = ScopeStack::new(2);

        assert_eq!(stack.base_indent(), 2, "Base indent should be 2");
        assert_eq!(stack.depth(), 1, "Should have root scope only");
        assert_eq!(stack.current_indent(), 0, "Root scope indent should be 0");
        assert_eq!(stack.get_scope_path(), "", "Root scope has no path");
        assert!(!stack.in_sequence_context(), "Root should not be in sequence context");
    }

    #[test]
    fn test_new_with_different_base_indents() {
        let stack_2 = ScopeStack::new(2);
        let stack_4 = ScopeStack::new(4);

        assert_eq!(stack_2.base_indent(), 2);
        assert_eq!(stack_4.base_indent(), 4);
    }

    #[test]
    fn test_set_base_indent() {
        let mut stack = ScopeStack::new(2);
        stack.set_base_indent(4);

        assert_eq!(stack.base_indent(), 4);
    }

    #[test]
    fn test_enter_scope_creates_new_scope() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("services".to_string()));

        verify_scope_state(&stack, 2, "services");
        assert_eq!(stack.current_indent(), 2);
    }

    #[test]
    fn test_enter_nested_scope() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("services".to_string()));
        stack.enter_scope(4, 2, Some("web".to_string()));
        stack.enter_scope(6, 3, Some("config".to_string()));

        verify_scope_state(&stack, 4, "services.web.config");
        assert_eq!(stack.current_indent(), 6);
    }

    #[test]
    fn test_enter_scope_without_parent_key() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, None);

        verify_scope_state(&stack, 2, "");
        assert_eq!(stack.current_indent(), 2);
    }

    #[test]
    fn test_enter_scope_reuses_existing_level() {
        let mut stack = ScopeStack::new(2);

        // Create first scope at level 2
        stack.enter_scope(2, 1, Some("web".to_string()));
        stack.add_key("host", 2).unwrap();

        // Exit back to root
        stack.exit_to_scope(0);

        // Create second scope at same level 2
        stack.enter_scope(2, 5, Some("database".to_string()));

        // Should be a fresh scope with no keys from previous
        assert!(!stack.contains_key("host"));
        verify_scope_state(&stack, 2, "database");
    }

    #[test]
    fn test_exit_to_single_scope() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("services".to_string()));
        stack.enter_scope(4, 2, Some("web".to_string()));

        verify_scope_state(&stack, 3, "services.web");

        stack.exit_to_scope(2);

        verify_scope_state(&stack, 2, "services");
        assert_eq!(stack.current_indent(), 2);
    }

    #[test]
    fn test_exit_to_multiple_scopes() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("level1".to_string()));
        stack.enter_scope(4, 2, Some("level2".to_string()));
        stack.enter_scope(6, 3, Some("level3".to_string()));
        stack.enter_scope(8, 4, Some("level4".to_string()));

        assert_eq!(stack.depth(), 5);

        // Exit directly to level1 (skip intermediate levels)
        stack.exit_to_scope(2);

        verify_scope_state(&stack, 2, "level1");
    }

    #[test]
    fn test_exit_to_root_scope() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("services".to_string()));
        stack.enter_scope(4, 2, Some("web".to_string()));
        stack.enter_scope(6, 3, Some("config".to_string()));

        stack.exit_to_scope(0);

        verify_scope_state(&stack, 1, "");
        assert_eq!(stack.current_indent(), 0);
    }

    #[test]
    fn test_add_key_to_root_scope() {
        let mut stack = ScopeStack::new(2);

        let result = stack.add_key("root_key", 1);
        assert!(result.is_ok(), "Should successfully add key to root scope");

        verify_key_exists(&stack, "root_key", true);
    }

    #[test]
    fn test_add_key_to_nested_scope() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("services".to_string()));
        stack.enter_scope(4, 2, Some("web".to_string()));

        let result = stack.add_key("host", 3);
        assert!(result.is_ok());

        verify_key_exists(&stack, "host", true);
        verify_scope_state(&stack, 3, "services.web");
    }

    #[test]
    fn test_add_multiple_keys_to_scope() {
        let mut stack = ScopeStack::new(2);

        stack.add_key("first", 1).unwrap();
        stack.add_key("second", 2).unwrap();
        stack.add_key("third", 3).unwrap();

        verify_key_exists(&stack, "first", true);
        verify_key_exists(&stack, "second", true);
        verify_key_exists(&stack, "third", true);

        assert_eq!(stack.current_scope_ref().key_count(), 3);
    }

    #[test]
    fn test_duplicate_key_in_same_scope() {
        let mut stack = ScopeStack::new(2);

        stack.add_key("host", 1).unwrap();

        let result = stack.add_key("host", 2);

        assert!(result.is_err(), "Should detect duplicate key in same scope");

        let error = result.unwrap_err();
        assert_eq!(error.key, "host");
        assert_eq!(error.first_line, 0); // Root scope starts at line 0
        assert_eq!(error.duplicate_line, 2);
    }

    #[test]
    fn test_duplicate_key_error_message() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("services".to_string()));
        stack.add_key("host", 2).unwrap();

        let result = stack.add_key("host", 5);
        let error = result.unwrap_err();

        let message = error.message();
        assert!(message.contains("duplicate key"));
        assert!(message.contains("host"));
        assert!(message.contains("services"));
        assert!(message.contains("Line 5"));
        assert!(message.contains("Line 1")); // Scope started at line 1, not where key was added
    }

    #[test]
    fn test_same_key_different_scopes_allowed() {
        let mut stack = ScopeStack::new(2);

        // Add "host" to root scope
        stack.add_key("host", 1).unwrap();

        // Enter nested scope
        stack.enter_scope(2, 2, Some("services".to_string()));

        // Same key should be allowed in different scope
        let result = stack.add_key("host", 3);
        assert!(result.is_ok(), "Same key should be allowed in different scopes");

        verify_key_exists(&stack, "host", true);
    }

    #[test]
    fn test_same_key_sibling_scopes_allowed() {
        let mut stack = ScopeStack::new(2);

        // First sibling scope
        stack.enter_scope(2, 1, Some("web".to_string()));
        stack.add_key("host", 2).unwrap();
        stack.add_key("port", 3).unwrap();

        // Exit to root and enter second sibling
        stack.exit_to_scope(0);
        stack.enter_scope(2, 5, Some("database".to_string()));

        // Same keys should be allowed in sibling scope
        stack.add_key("host", 6).unwrap();
        stack.add_key("port", 7).unwrap();

        verify_key_exists(&stack, "host", true);
        verify_key_exists(&stack, "port", true);
    }

    #[test]
    fn test_scope_isolation_deep_nesting() {
        let mut stack = ScopeStack::new(2);

        // Create deeply nested scopes with same key at each level
        stack.enter_scope(2, 1, Some("level1".to_string()));
        stack.add_key("value", 2).unwrap();

        stack.enter_scope(4, 3, Some("level2".to_string()));
        stack.add_key("value", 4).unwrap();

        stack.enter_scope(6, 5, Some("level3".to_string()));
        stack.add_key("value", 6).unwrap();

        stack.enter_scope(8, 7, Some("level4".to_string()));
        stack.add_key("value", 8).unwrap();

        // Each scope should have exactly one key
        assert_eq!(stack.current_scope_ref().key_count(), 1);

        // Duplicate in current scope should fail
        assert!(stack.add_key("value", 9).is_err());
    }

    #[test]
    fn test_enter_sequence_scope_creates_isolated_scope() {
        let mut stack = ScopeStack::new(2);

        stack.enter_sequence_scope(2, 1);

        assert!(stack.in_sequence_context());
        assert_eq!(stack.current_indent(), 2);
        assert_eq!(stack.depth(), 2); // Root + sequence scope
    }

    #[test]
    fn test_sequence_scope_isolation() {
        let mut stack = ScopeStack::new(2);

        // First sequence item
        stack.enter_sequence_scope(2, 1);
        stack.add_key("name", 2).unwrap();
        stack.add_key("port", 3).unwrap();
        assert_eq!(stack.current_scope_ref().key_count(), 2);

        // Second sequence item (same indent, new scope)
        stack.enter_sequence_scope(2, 5);

        // Keys should be cleared for new sequence item
        assert_eq!(stack.current_scope_ref().key_count(), 0);
        assert!(!stack.contains_key("name"));
        assert!(!stack.contains_key("port"));
    }

    #[test]
    fn test_sequence_scope_unique_item_ids() {
        let mut stack = ScopeStack::new(2);

        stack.enter_sequence_scope(2, 1);
        let first_id = stack.current_scope_ref().sequence_item_id;

        stack.enter_sequence_scope(2, 3);
        let second_id = stack.current_scope_ref().sequence_item_id;

        stack.enter_sequence_scope(2, 5);
        let third_id = stack.current_scope_ref().sequence_item_id;

        assert_eq!(first_id, Some(1));
        assert_eq!(second_id, Some(2));
        assert_eq!(third_id, Some(3));
    }

    #[test]
    fn test_sequence_scope_allows_same_keys_across_items() {
        let mut stack = ScopeStack::new(2);

        // First item
        stack.enter_sequence_scope(2, 1);
        stack.add_key("host", 2).unwrap();
        stack.add_key("port", 3).unwrap();

        // Second item - same keys should be OK
        stack.enter_sequence_scope(2, 5);
        stack.add_key("host", 6).unwrap();
        stack.add_key("port", 7).unwrap();

        // Third item - again, same keys OK
        stack.enter_sequence_scope(2, 10);
        stack.add_key("host", 11).unwrap();
        stack.add_key("port", 12).unwrap();
    }

    #[test]
    fn test_sequence_scope_duplicate_within_item() {
        let mut stack = ScopeStack::new(2);

        stack.enter_sequence_scope(2, 1);
        stack.add_key("name", 2).unwrap();

        // Duplicate within same sequence item should fail
        let result = stack.add_key("name", 3);
        assert!(result.is_err());
    }

    #[test]
    fn test_mixed_regular_and_sequence_scopes() {
        let mut stack = ScopeStack::new(2);

        // Regular scope
        stack.enter_scope(2, 1, Some("services".to_string()));
        stack.add_key("web", 2).unwrap();

        // Sequence scope inside regular scope
        stack.enter_sequence_scope(4, 3);
        stack.add_key("name", 4).unwrap();

        // Exit sequence scope
        stack.exit_to_scope(2);
        assert!(!stack.in_sequence_context());

        // Add to regular scope again
        stack.add_key("database", 5).unwrap();

        assert!(stack.contains_key("web"));
        assert!(stack.contains_key("database"));
    }

    #[test]
    fn test_reset_clears_all_scopes() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("level1".to_string()));
        stack.enter_scope(4, 2, Some("level2".to_string()));
        stack.add_key("key1", 3).unwrap();
        stack.add_key("key2", 4).unwrap();

        assert_eq!(stack.depth(), 3);

        stack.reset();

        assert_eq!(stack.depth(), 1);
        assert_eq!(stack.current_indent(), 0);
        assert!(!stack.contains_key("key1"));
        assert!(!stack.contains_key("key2"));
    }

    #[test]
    fn test_contains_key_in_current_scope_only() {
        let mut stack = ScopeStack::new(2);

        // Add to root
        stack.add_key("root_key", 1).unwrap();

        // Enter nested scope
        stack.enter_scope(2, 2, Some("nested".to_string()));

        // Root key should not be in current scope
        assert!(!stack.contains_key("root_key"));

        // But should exist in some scope
        assert!(stack.contains_key_in_any_scope("root_key"));
    }

    #[test]
    fn test_contains_key_in_any_scope() {
        let mut stack = ScopeStack::new(2);

        stack.add_key("level0", 1).unwrap();

        stack.enter_scope(2, 2, Some("level1".to_string()));
        stack.add_key("level1_key", 3).unwrap();

        stack.enter_scope(4, 4, Some("level2".to_string()));
        stack.add_key("level2_key", 5).unwrap();

        // From deepest scope, check all levels
        assert!(stack.contains_key_in_any_scope("level0"));
        assert!(stack.contains_key_in_any_scope("level1_key"));
        assert!(stack.contains_key_in_any_scope("level2_key"));

        // Current scope only
        assert!(!stack.contains_key("level0"));
        assert!(!stack.contains_key("level1_key"));
        assert!(stack.contains_key("level2_key"));
    }

    #[test]
    fn test_get_scope_at_level() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("level2".to_string()));
        stack.enter_scope(4, 2, Some("level4".to_string()));
        stack.enter_scope(6, 3, Some("level6".to_string()));

        assert!(stack.get_scope_at_level(0).is_some());
        assert!(stack.get_scope_at_level(2).is_some());
        assert!(stack.get_scope_at_level(4).is_some());
        assert!(stack.get_scope_at_level(6).is_some());
        assert!(stack.get_scope_at_level(8).is_none());
    }

    #[test]
    fn test_scope_display_implementation() {
        let scope = Scope::new(2, 5, Some("parent".to_string()));
        let display = format!("{}", scope);

        assert!(display.contains("Scope"));
        assert!(display.contains("indent=2"));
    }

    #[test]
    fn test_scope_stack_display_implementation() {
        let mut stack = ScopeStack::new(2);
        stack.enter_scope(2, 1, Some("test".to_string()));

        let display = format!("{}", stack);

        assert!(display.contains("ScopeStack"));
        assert!(display.contains("depth="));
        assert!(display.contains("base_indent=2"));
    }

    #[test]
    fn test_exit_to_scope_finds_closest_parent_when_target_missing() {
        let mut stack = ScopeStack::new(2);

        // Create scope at indent 4 (skipping indent 2)
        stack.enter_scope(4, 1, Some("level4".to_string()));

        // Exit to indent 2 (which doesn't exist)
        // Should find closest parent: root at indent 0
        stack.exit_to_scope(2);

        // Should exit to root (closest parent scope)
        assert_eq!(stack.current_indent(), 0);
        verify_scope_state(&stack, 1, ""); // depth=1 (root only), path="" (empty)
    }

    #[test]
    fn test_complex_yaml_structure_simulation() {
        let mut stack = ScopeStack::new(2);

        // Simulate:
        // services:
        //   web:
        //     host: localhost
        //     port: 8080
        //   database:
        //     host: db.example.com
        //     port: 5432

        stack.enter_scope(2, 1, Some("services".to_string()));

        // web section
        stack.enter_scope(4, 2, Some("web".to_string()));
        stack.add_key("host", 3).unwrap();
        stack.add_key("port", 4).unwrap();

        // database section (sibling)
        stack.exit_to_scope(2);
        stack.enter_scope(4, 5, Some("database".to_string()));

        // Same keys should be OK in sibling scope
        stack.add_key("host", 6).unwrap();
        stack.add_key("port", 7).unwrap();

        // Verify final state
        verify_scope_state(&stack, 3, "services.database");
        verify_key_exists(&stack, "host", true);
        verify_key_exists(&stack, "port", true);
    }

    #[test]
    fn test_sequence_within_mapping() {
        let mut stack = ScopeStack::new(2);

        // Simulate:
        // items:
        //   - name: item1
        //   - name: item2

        stack.enter_scope(2, 1, Some("items".to_string()));

        // First sequence item
        stack.enter_sequence_scope(4, 2);
        stack.add_key("name", 3).unwrap();
        assert_eq!(stack.current_scope_ref().key_count(), 1);

        // Second sequence item
        stack.enter_sequence_scope(4, 4);
        stack.add_key("name", 5).unwrap();
        assert_eq!(stack.current_scope_ref().key_count(), 1);

        // Both items should have "name" key without conflict
    }

    #[test]
    fn test_duplicate_key_preserves_scope_path() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("services".to_string()));
        stack.enter_scope(4, 2, Some("web".to_string()));
        stack.add_key("config", 3).unwrap();

        let error = stack.add_key("config", 4).unwrap_err();

        assert_eq!(error.scope_path, "services.web");
        assert_eq!(error.key, "config");
    }
}

#[cfg(test)]
mod scope_tests {
    use super::*;

    #[test]
    fn test_scope_creation_with_all_parameters() {
        let scope = Scope::new(4, 10, Some("parent_key".to_string()));

        assert_eq!(scope.indent_level, 4);
        assert_eq!(scope.start_line, 10);
        assert_eq!(scope.parent_key, Some("parent_key".to_string()));
        assert_eq!(scope.key_count(), 0);
        assert!(!scope.is_flow_style);
        assert!(!scope.in_sequence_context);
        assert!(scope.sequence_item_id.is_none());
    }

    #[test]
    fn test_scope_creation_minimal() {
        let scope = Scope::new(0, 1, None);

        assert_eq!(scope.indent_level, 0);
        assert_eq!(scope.start_line, 1);
        assert!(scope.parent_key.is_none());
    }

    #[test]
    fn test_scope_add_key_returns_false_on_first_add() {
        let mut scope = Scope::new(0, 1, None);

        let result = scope.add_key("new_key");

        assert_eq!(result, false, "First add should return false (not duplicate)");
        assert_eq!(scope.key_count(), 1);
    }

    #[test]
    fn test_scope_add_key_returns_true_on_duplicate() {
        let mut scope = Scope::new(0, 1, None);

        scope.add_key("key");
        let result = scope.add_key("key");

        assert_eq!(result, true, "Duplicate add should return true");
        assert_eq!(scope.key_count(), 1, "Key count should not increase on duplicate");
    }

    #[test]
    fn test_scope_add_multiple_unique_keys() {
        let mut scope = Scope::new(0, 1, None);

        scope.add_key("key1");
        scope.add_key("key2");
        scope.add_key("key3");

        assert_eq!(scope.key_count(), 3);
        assert!(scope.contains_key("key1"));
        assert!(scope.contains_key("key2"));
        assert!(scope.contains_key("key3"));
    }

    #[test]
    fn test_scope_contains_key() {
        let mut scope = Scope::new(0, 1, None);

        assert!(!scope.contains_key("test"));

        scope.add_key("test");

        assert!(scope.contains_key("test"));
    }

    #[test]
    fn test_scope_clear_keys() {
        let mut scope = Scope::new(0, 1, None);

        scope.add_key("key1");
        scope.add_key("key2");
        scope.add_key("key3");
        assert_eq!(scope.key_count(), 3);

        scope.clear_keys();

        assert_eq!(scope.key_count(), 0);
        assert!(!scope.contains_key("key1"));
    }

    #[test]
    fn test_scope_key_count() {
        let mut scope = Scope::new(0, 1, None);

        assert_eq!(scope.key_count(), 0);

        scope.add_key("first");
        assert_eq!(scope.key_count(), 1);

        scope.add_key("second");
        assert_eq!(scope.key_count(), 2);

        scope.add_key("first"); // duplicate
        assert_eq!(scope.key_count(), 2); // Should not change
    }
}

#[cfg(test)]
mod duplicate_key_error_tests {
    use super::*;

    #[test]
    fn test_duplicate_key_error_creation() {
        let error = DuplicateKeyError::new(
            "config".to_string(),
            "services.web.database".to_string(),
            10,
            25,
        );

        assert_eq!(error.key, "config");
        assert_eq!(error.scope_path, "services.web.database");
        assert_eq!(error.first_line, 10);
        assert_eq!(error.duplicate_line, 25);
    }

    #[test]
    fn test_duplicate_key_error_display() {
        let error = DuplicateKeyError::new(
            "host".to_string(),
            "services.web".to_string(),
            5,
            15,
        );

        let display = format!("{}", error);

        assert!(display.contains("Line 15"));
        assert!(display.contains("host"));
        assert!(display.contains("services.web"));
        assert!(display.contains("Line 5"));
    }

    #[test]
    fn test_duplicate_key_error_message_formatting() {
        let error = DuplicateKeyError::new(
            "port".to_string(),
            "database.config".to_string(),
            100,
            200,
        );

        let message = error.message();

        assert!(message.contains("Line 200"));
        assert!(message.contains("duplicate key"));
        assert!(message.contains("'port'"));
        assert!(message.contains("database.config"));
        assert!(message.contains("Line 100"));
    }
}

#[cfg(test)]
mod integration_tests {
    use super::*;

    #[test]
    fn test_real_world_yaml_parsing_scenario() {
        let mut stack = ScopeStack::new(2);

        // Simulate parsing a real YAML config:
        // application:
        //   server:
        //     host: localhost
        //     port: 8080
        //   database:
        //     host: db.example.com
        //     port: 5432
        //     credentials:
        //       username: admin
        //       password: secret
        // logging:
        //   level: info
        //   file: app.log

        // Root scope
        stack.enter_scope(2, 1, Some("application".to_string()));

        // server section
        stack.enter_scope(4, 2, Some("server".to_string()));
        stack.add_key("host", 3).unwrap();
        stack.add_key("port", 4).unwrap();

        // database section
        stack.exit_to_scope(2);
        stack.enter_scope(4, 5, Some("database".to_string()));
        stack.add_key("host", 6).unwrap();
        stack.add_key("port", 7).unwrap();

        // credentials section (nested in database)
        stack.enter_scope(6, 8, Some("credentials".to_string()));
        stack.add_key("username", 9).unwrap();
        stack.add_key("password", 10).unwrap();

        // Exit to root and add logging section
        stack.exit_to_scope(0);
        stack.enter_scope(2, 11, Some("logging".to_string()));
        stack.add_key("level", 12).unwrap();
        stack.add_key("file", 13).unwrap();

        // Verify no duplicate key errors occurred
        verify_scope_state(&stack, 2, "logging");
        assert!(stack.contains_key("level"));
        assert!(stack.contains_key("file"));
    }

    #[test]
    fn test_complex_sequence_scenario() {
        let mut stack = ScopeStack::new(2);

        // Simulate:
        // items:
        //   - id: 1
        //     name: First
        //     config:
        //       enabled: true
        //   - id: 2
        //     name: Second
        //     config:
        //       enabled: false

        stack.enter_scope(2, 1, Some("items".to_string()));

        // First sequence item
        stack.enter_sequence_scope(4, 2);
        stack.add_key("id", 3).unwrap();
        stack.add_key("name", 4).unwrap();
        stack.enter_scope(6, 5, Some("config".to_string()));
        stack.add_key("enabled", 6).unwrap();

        // Second sequence item
        stack.exit_to_scope(2);
        stack.enter_sequence_scope(4, 7);
        stack.add_key("id", 8).unwrap();
        stack.add_key("name", 9).unwrap();
        stack.enter_scope(6, 10, Some("config".to_string()));
        stack.add_key("enabled", 11).unwrap();

        // Both sequence items should have the same structure
        assert!(stack.contains_key("enabled"));
    }

    #[test]
    fn test_error_detection_in_complex_structure() {
        let mut stack = ScopeStack::new(2);

        // Simulate YAML with duplicate in nested scope:
        // config:
        //   server:
        //     host: localhost
        //     host: duplicate  <- This should be detected

        stack.enter_scope(2, 1, Some("config".to_string()));
        stack.enter_scope(4, 2, Some("server".to_string()));
        stack.add_key("host", 3).unwrap();

        let result = stack.add_key("host", 4);
        assert!(result.is_err());

        let error = result.unwrap_err();
        assert_eq!(error.key, "host");
        assert_eq!(error.scope_path, "config.server");
        assert_eq!(error.first_line, 2); // Scope started at line 2
        assert_eq!(error.duplicate_line, 4);
    }
}

#[cfg(test)]
mod sequence_scope_comprehensive_tests {
    use super::*;

    /// Test sequence scope entry is tracked correctly
    #[test]
    fn test_sequence_scope_entry_tracking() {
        let mut stack = ScopeStack::new(2);

        // Enter sequence scope
        stack.enter_sequence_scope(4, 1);

        // Verify scope state after entry
        assert!(stack.in_sequence_context(), "Should be in sequence context");
        assert_eq!(stack.current_indent(), 4, "Current indent should be 4");
        assert_eq!(stack.depth(), 2, "Should have root + sequence scope");

        let scope = stack.current_scope_ref();
        assert!(scope.in_sequence_context, "Scope should be marked as sequence context");
        assert_eq!(scope.sequence_item_id, Some(1), "First sequence item should have ID 1");
    }

    /// Test sequence scope exit maintains proper scope state
    #[test]
    fn test_sequence_scope_exit_maintains_state() {
        let mut stack = ScopeStack::new(2);

        // Create a mapping scope
        stack.enter_scope(2, 1, Some("services".to_string()));

        // Enter sequence scope within mapping
        stack.enter_sequence_scope(4, 2);
        stack.add_key("item1", 3).unwrap();
        assert!(stack.in_sequence_context());

        // Exit sequence scope back to mapping
        stack.exit_to_scope(2);

        // Verify we're back in mapping context, not sequence
        assert!(!stack.in_sequence_context(), "Should not be in sequence context after exit");
        assert_eq!(stack.current_indent(), 2, "Should be back at mapping indent");
        assert_eq!(stack.depth(), 2, "Should have root + mapping scope");
    }

    /// Test sequences nested in mappings
    #[test]
    fn test_sequences_nested_in_mappings() {
        let mut stack = ScopeStack::new(2);

        // Create parent mapping
        stack.enter_scope(2, 1, Some("items".to_string()));

        // First sequence item
        stack.enter_sequence_scope(4, 2);
        stack.add_key("name", 3).unwrap();
        assert_eq!(stack.current_scope_ref().key_count(), 1);

        // Second sequence item (should clear keys from first)
        stack.enter_sequence_scope(4, 4);
        stack.add_key("name", 5).unwrap();
        assert_eq!(stack.current_scope_ref().key_count(), 1);

        // Verify both items had "name" key without conflict
        assert!(stack.in_sequence_context());
        assert_eq!(stack.current_scope_ref().sequence_item_id, Some(2));
    }

    /// Test mappings nested in sequences
    #[test]
    fn test_mappings_nested_in_sequences() {
        let mut stack = ScopeStack::new(2);

        // Enter sequence scope
        stack.enter_sequence_scope(4, 1);

        // Add key to sequence item
        stack.add_key("id", 2).unwrap();

        // Enter nested mapping scope within sequence
        stack.enter_scope(6, 3, Some("config".to_string()));
        stack.add_key("enabled", 4).unwrap();
        stack.add_key("timeout", 5).unwrap();

        // Verify we're in mapping scope nested in sequence
        assert_eq!(stack.depth(), 3); // Root + sequence + mapping
        assert_eq!(stack.current_scope_ref().key_count(), 2);
        assert!(!stack.current_scope_ref().in_sequence_context); // Mapping scope, not sequence

        // Parent scope should be sequence
        stack.exit_to_scope(4);
        assert!(stack.in_sequence_context());
    }

    /// Test complex nested structure with sequences and mappings
    #[test]
    fn test_complex_nested_sequences_mappings() {
        let mut stack = ScopeStack::new(2);

        // Simulate:
        // services:
        //   web:
        //     instances:
        //       - host: localhost
        //         port: 8080
        //       - host: localhost2
        //         port: 8081

        stack.enter_scope(2, 1, Some("services".to_string()));
        stack.enter_scope(4, 2, Some("web".to_string()));
        stack.enter_scope(6, 3, Some("instances".to_string()));

        // First sequence item with nested mapping
        stack.enter_sequence_scope(8, 4);
        stack.add_key("host", 5).unwrap();
        stack.enter_scope(10, 6, Some("config".to_string()));
        stack.add_key("port", 7).unwrap();

        // Exit back to sequence level for second item
        stack.exit_to_scope(6);
        stack.enter_sequence_scope(8, 8);
        stack.add_key("host", 9).unwrap();

        // Verify we're in second sequence item
        assert!(stack.in_sequence_context());
        assert_eq!(stack.current_scope_ref().sequence_item_id, Some(2));
        assert!(stack.contains_key("host"));
    }

    /// Test sequence scope with multiple levels of nesting
    #[test]
    fn test_sequence_deep_nesting() {
        let mut stack = ScopeStack::new(2);

        // Root mapping (depth 1)
        stack.enter_scope(2, 1, Some("root".to_string()));
        assert_eq!(stack.depth(), 2); // Root + mapping

        // Sequence at indent 4 (depth 2)
        stack.enter_sequence_scope(4, 2);
        assert_eq!(stack.depth(), 3); // Root + mapping + sequence

        // Nested mapping at indent 6 (depth 3)
        stack.enter_scope(6, 3, Some("level1".to_string()));
        assert_eq!(stack.depth(), 4); // Root + mapping + sequence + mapping

        // Another sequence at indent 8 (depth 4)
        stack.enter_sequence_scope(8, 4);
        assert_eq!(stack.depth(), 5); // Root + mapping + sequence + mapping + sequence

        // Verify deep nesting structure
        assert!(stack.in_sequence_context());
        assert_eq!(stack.current_indent(), 8);
        assert_eq!(stack.current_scope_ref().sequence_item_id, Some(2)); // Second sequence overall
    }

    /// Test sequence scope clears parent mapping keys at same level
    #[test]
    fn test_sequence_clears_parent_keys_at_level() {
        let mut stack = ScopeStack::new(2);

        // Parent mapping with keys
        stack.enter_scope(2, 1, Some("parent".to_string()));
        stack.enter_scope(4, 2, Some("child".to_string()));
        stack.add_key("key1", 3).unwrap();
        stack.add_key("key2", 4).unwrap();
        assert_eq!(stack.current_scope_ref().key_count(), 2);

        // Enter sequence scope at same indent - should clear
        stack.enter_sequence_scope(4, 5);

        // Keys should be cleared for new sequence scope
        assert_eq!(stack.current_scope_ref().key_count(), 0);
        assert!(!stack.contains_key("key1"));
        assert!(!stack.contains_key("key2"));
        assert!(stack.in_sequence_context());
    }

    /// Test sequence item IDs increment correctly
    #[test]
    fn test_sequence_item_id_increment() {
        let mut stack = ScopeStack::new(2);

        // Create multiple sequence items
        for i in 1..=10 {
            stack.enter_sequence_scope(4, i * 2);
            assert_eq!(stack.current_scope_ref().sequence_item_id, Some(i));
            stack.add_key("id", i * 2 + 1).unwrap();
        }

        // Last item should have ID 10
        assert_eq!(stack.current_scope_ref().sequence_item_id, Some(10));
    }

    /// Test sequence scope after exiting deep nesting
    #[test]
    fn test_sequence_after_deep_nesting_exit() {
        let mut stack = ScopeStack::new(2);

        // Deep nesting
        stack.enter_scope(2, 1, Some("level1".to_string()));
        stack.enter_scope(4, 2, Some("level2".to_string()));
        stack.enter_scope(6, 3, Some("level3".to_string()));
        assert_eq!(stack.depth(), 4);

        // Exit to root and enter sequence
        stack.exit_to_scope(0);
        stack.enter_sequence_scope(2, 4);

        // Verify sequence scope is clean
        assert!(stack.in_sequence_context());
        assert_eq!(stack.depth(), 2); // Root + sequence
        assert_eq!(stack.current_scope_ref().key_count(), 0);
    }

    /// Test sequence with parent key followed by nested content
    #[test]
    fn test_sequence_with_parent_key_nested_content() {
        let mut stack = ScopeStack::new(2);

        // Simulate:
        // items:
        //   - name: item1
        //     config:
        //       enabled: true

        stack.enter_scope(2, 1, Some("items".to_string()));
        assert_eq!(stack.depth(), 2); // Root + items mapping

        // First sequence item
        stack.enter_sequence_scope(4, 2);
        assert_eq!(stack.depth(), 3); // Root + items + sequence
        stack.add_key("name", 3).unwrap();

        // Nested mapping within sequence item
        stack.enter_scope(6, 4, Some("config".to_string()));
        assert_eq!(stack.depth(), 4); // Root + items + sequence + config
        stack.add_key("enabled", 5).unwrap();

        // Verify structure
        assert_eq!(stack.current_scope_ref().key_count(), 1);
        assert!(!stack.in_sequence_context()); // In mapping, not sequence
    }

    /// Test sequence scope handles indent transitions correctly
    #[test]
    fn test_sequence_indent_transitions() {
        let mut stack = ScopeStack::new(2);

        // Start with sequence at indent 4
        stack.enter_sequence_scope(4, 1);
        assert_eq!(stack.current_indent(), 4);

        // Decrease indent (exit sequence)
        stack.exit_to_scope(0);
        assert_eq!(stack.current_indent(), 0);

        // Re-enter sequence at same level
        stack.enter_sequence_scope(4, 2);
        assert_eq!(stack.current_indent(), 4);
        assert!(stack.in_sequence_context());
    }

    /// Test multiple sequences at different levels
    #[test]
    fn test_multiple_sequences_different_levels() {
        let mut stack = ScopeStack::new(2);

        // Outer sequence at indent 2
        stack.enter_sequence_scope(2, 1);
        let outer_id = stack.current_scope_ref().sequence_item_id;
        assert_eq!(outer_id, Some(1));

        // Inner sequence at indent 4 (nested sequence)
        stack.enter_sequence_scope(4, 2);
        let inner_id = stack.current_scope_ref().sequence_item_id;
        // Sequence item ID is global, continues incrementing
        assert_eq!(inner_id, Some(2)); // Continues from previous sequence

        // Verify they're at different depths (both sequences are kept)
        assert_eq!(stack.depth(), 3); // Root + outer sequence + inner sequence
    }

    /// Test sequence scope preserves parent context correctly
    #[test]
    fn test_sequence_preserves_parent_context() {
        let mut stack = ScopeStack::new(2);

        // Create parent mapping
        stack.enter_scope(2, 1, Some("services".to_string()));
        let parent_path = stack.get_scope_path();
        assert_eq!(parent_path, "services");

        // Enter sequence scope
        stack.enter_sequence_scope(4, 2);

        // Exit sequence scope
        stack.exit_to_scope(2);

        // Verify parent mapping context is preserved
        assert_eq!(stack.get_scope_path(), "services");
        assert!(!stack.in_sequence_context());
    }
}
