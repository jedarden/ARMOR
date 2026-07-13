//! Unit tests for ScopeStack data structure
//!
//! These tests provide focused unit test coverage for each ScopeStack method,
//! verifying correct behavior for:
//! - Scope tracking with proper isolation
//! - Scope entry and exit on indent changes
//! - Duplicate key detection within scope
//! - Same key name allowance in different scopes
//!
//! Bead: bf-68arep

use armor::parsers::yaml::ScopeStack;

// =============================================================================
// ScopeStack::new() Tests
// =============================================================================

#[test]
fn test_scope_stack_new_creates_root_scope() {
    let stack = ScopeStack::new(2);
    assert_eq!(stack.depth(), 1, "Should start with root scope");
    assert_eq!(stack.current_indent(), 0, "Root scope should have indent 0");
    assert_eq!(stack.base_indent(), 2, "Base indent should be 2");
}

#[test]
fn test_scope_stack_new_with_different_base_indents() {
    let stack_2 = ScopeStack::new(2);
    let stack_4 = ScopeStack::new(4);
    let stack_8 = ScopeStack::new(8);

    assert_eq!(stack_2.base_indent(), 2);
    assert_eq!(stack_4.base_indent(), 4);
    assert_eq!(stack_8.base_indent(), 8);
}

#[test]
fn test_scope_stack_new_creates_empty_root_scope() {
    let stack = ScopeStack::new(2);
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 0);
    assert!(!stack.contains_key("any_key"));
}

// =============================================================================
// ScopeStack::add_key() Tests
// =============================================================================

#[test]
fn test_add_key_to_empty_scope_succeeds() {
    let mut stack = ScopeStack::new(2);
    let result = stack.add_key("new_key", 1);
    assert!(result.is_ok(), "Adding new key should succeed");
    assert!(stack.contains_key("new_key"));
}

#[test]
fn test_add_key_to_root_scope() {
    let mut stack = ScopeStack::new(2);
    stack.add_key("root_key", 1).unwrap();
    assert!(stack.contains_key("root_key"));
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 1);
}

#[test]
fn test_add_multiple_keys_to_same_scope() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("key1", 1).unwrap();
    stack.add_key("key2", 2).unwrap();
    stack.add_key("key3", 3).unwrap();

    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 3);
    assert!(stack.contains_key("key1"));
    assert!(stack.contains_key("key2"));
    assert!(stack.contains_key("key3"));
}

#[test]
fn test_add_duplicate_key_in_same_scope_fails() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("duplicate", 1).unwrap();
    let result = stack.add_key("duplicate", 2);

    assert!(result.is_err(), "Duplicate key should fail");

    if let Err(err) = result {
        assert_eq!(err.key, "duplicate");
        assert_eq!(err.duplicate_line, 2);
        assert_eq!(err.first_line, 0); // Root scope start line
        assert_eq!(err.scope_path, "");
    }
}

#[test]
fn test_add_duplicate_key_returns_error_with_correct_fields() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 5, Some("parent".to_string()));
    stack.add_key("test_key", 6).unwrap();

    let result = stack.add_key("test_key", 10);
    assert!(result.is_err());

    let err = result.unwrap_err();
    assert_eq!(err.key, "test_key");
    assert_eq!(err.first_line, 5);
    assert_eq!(err.duplicate_line, 10);
    assert_eq!(err.scope_path, "parent");
}

#[test]
fn test_add_key_does_not_affect_parent_scope() {
    let mut stack = ScopeStack::new(2);

    // Add to root
    stack.add_key("root_key", 1).unwrap();

    // Enter nested scope
    stack.enter_scope(2, 2, Some("child".to_string()));
    stack.add_key("child_key", 3).unwrap();

    // Exit to root and verify it still only has root_key
    stack.exit_to_scope(0);
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 1);
    assert!(stack.contains_key("root_key"));
    assert!(!stack.contains_key("child_key"));
}

#[test]
fn test_add_key_in_nested_scope_isolated_from_parent() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("host", 1).unwrap();
    stack.enter_scope(2, 2, Some("services".to_string()));

    // Same key name in different scope should succeed
    let result = stack.add_key("host", 3);
    assert!(result.is_ok(), "Same key in different scope should succeed");
    assert!(stack.contains_key("host"));
}

// =============================================================================
// ScopeStack::enter_scope() Tests
// =============================================================================

#[test]
fn test_enter_scope_increases_depth() {
    let mut stack = ScopeStack::new(2);

    assert_eq!(stack.depth(), 1);
    stack.enter_scope(2, 1, Some("level1".to_string()));
    assert_eq!(stack.depth(), 2);
    stack.enter_scope(4, 2, Some("level2".to_string()));
    assert_eq!(stack.depth(), 3);
}

#[test]
fn test_enter_scope_updates_current_indent() {
    let mut stack = ScopeStack::new(2);

    assert_eq!(stack.current_indent(), 0);
    stack.enter_scope(2, 1, Some("level1".to_string()));
    assert_eq!(stack.current_indent(), 2);
    stack.enter_scope(4, 2, Some("level2".to_string()));
    assert_eq!(stack.current_indent(), 4);
}

#[test]
fn test_enter_scope_creates_new_scope_with_parent_key() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 5, Some("parent_key".to_string()));
    let scope = stack.current_scope_ref();

    assert_eq!(scope.indent_level, 2);
    assert_eq!(scope.start_line, 5);
    assert_eq!(scope.parent_key, Some("parent_key".to_string()));
}

#[test]
fn test_enter_scope_with_none_parent_key() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, None);
    let scope = stack.current_scope_ref();

    assert_eq!(scope.indent_level, 2);
    assert_eq!(scope.parent_key, None);
}

#[test]
fn test_enter_scope_reuses_existing_level() {
    let mut stack = ScopeStack::new(2);

    // Enter first scope at level 2
    stack.enter_scope(2, 1, Some("first".to_string()));
    stack.add_key("key1", 2).unwrap();
    stack.exit_to_scope(0);

    // Re-enter at same level 2
    stack.enter_scope(2, 5, Some("second".to_string()));

    // Should not have keys from first scope
    assert!(!stack.contains_key("key1"));
    assert_eq!(stack.current_scope_ref().parent_key, Some("second".to_string()));
}

#[test]
fn test_enter_scope_clears_deeper_scopes() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(6, 3, Some("level3".to_string()));

    assert_eq!(stack.depth(), 4);

    // Re-enter at level 2 should clear deeper levels
    stack.enter_scope(2, 10, Some("new_level1".to_string()));

    // Depth is 3: root (0) + old scope at level 2 + new scope at level 2
    assert_eq!(stack.depth(), 3);
    assert!(!stack.contains_key("level2"));
    assert!(!stack.contains_key("level3"));
    assert!(stack.get_scope_at_level(4).is_none());
    assert!(stack.get_scope_at_level(6).is_none());
}

#[test]
fn test_enter_nested_scopes_updates_path() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("services".to_string()));
    assert_eq!(stack.get_scope_path(), "services");

    stack.enter_scope(4, 2, Some("web".to_string()));
    assert_eq!(stack.get_scope_path(), "services.web");

    stack.enter_scope(6, 3, Some("config".to_string()));
    assert_eq!(stack.get_scope_path(), "services.web.config");
}

// =============================================================================
// ScopeStack::exit_to_scope() Tests
// =============================================================================

#[test]
fn test_exit_to_scope_reduces_depth() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(6, 3, Some("level3".to_string()));

    assert_eq!(stack.depth(), 4);

    stack.exit_to_scope(2);
    assert_eq!(stack.depth(), 2);

    stack.exit_to_scope(0);
    assert_eq!(stack.depth(), 1);
}

#[test]
fn test_exit_to_scope_updates_current_indent() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    assert_eq!(stack.current_indent(), 4);

    stack.exit_to_scope(2);
    assert_eq!(stack.current_indent(), 2);

    stack.exit_to_scope(0);
    assert_eq!(stack.current_indent(), 0);
}

#[test]
fn test_exit_to_scope_clears_keys_from_exited_scopes() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.add_key("level1_key", 2).unwrap();

    stack.enter_scope(4, 3, Some("level2".to_string()));
    stack.add_key("level2_key", 4).unwrap();

    stack.exit_to_scope(2);

    // Should be at level1 now
    assert!(stack.contains_key("level1_key"));
    assert!(!stack.contains_key("level2_key"));
}

#[test]
fn test_exit_to_root_scope() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(6, 3, Some("level3".to_string()));

    stack.exit_to_scope(0);

    assert_eq!(stack.depth(), 1);
    assert_eq!(stack.current_indent(), 0);
    assert_eq!(stack.get_scope_path(), "");
}

#[test]
fn test_exit_to_non_existent_scope_uses_closest_parent() {
    let mut stack = ScopeStack::new(2);

    // Enter scope at level 4 (deeper than we'll exit to)
    stack.enter_scope(4, 1, Some("level4".to_string()));

    // Exit to level 2 which doesn't exist (only root and level 4 exist)
    // New behavior: find closest parent scope (root at level 0)
    stack.exit_to_scope(2);

    // Should be at root scope (closest parent)
    // Depth is 1: only root (0)
    // Level 4 was removed because it's deeper than target 2
    // No fallback is created at level 2
    assert_eq!(stack.depth(), 1);
    assert_eq!(stack.current_indent(), 0);
    assert!(stack.get_scope_at_level(2).is_none());
    assert!(stack.get_scope_at_level(4).is_none());
}

#[test]
fn test_exit_to_sibling_scope() {
    let mut stack = ScopeStack::new(2);

    // First sibling
    stack.enter_scope(2, 1, Some("sibling1".to_string()));
    stack.add_key("key1", 2).unwrap();
    stack.exit_to_scope(0);

    // Second sibling
    stack.enter_scope(2, 5, Some("sibling2".to_string()));

    assert!(!stack.contains_key("key1"));
    assert_eq!(stack.current_scope_ref().parent_key, Some("sibling2".to_string()));
}

// =============================================================================
// ScopeStack::enter_sequence_scope() Tests
// =============================================================================

#[test]
fn test_enter_sequence_scope_creates_sequence_context() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);

    assert!(stack.in_sequence_context());
    assert_eq!(stack.current_indent(), 2);
}

#[test]
fn test_enter_sequence_scope_sets_sequence_flags() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    let scope = stack.current_scope_ref().unwrap();

    assert!(scope.in_sequence_context);
    assert!(scope.sequence_item_id.is_some());
}

#[test]
fn test_enter_sequence_scope_generates_unique_ids() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    let id1 = stack.current_scope_ref().unwrap().sequence_item_id;

    stack.enter_sequence_scope(2, 3);
    let id2 = stack.current_scope_ref().unwrap().sequence_item_id;

    stack.enter_sequence_scope(2, 5);
    let id3 = stack.current_scope_ref().unwrap().sequence_item_id;

    assert_eq!(id1, Some(1));
    assert_eq!(id2, Some(2));
    assert_eq!(id3, Some(3));
}

#[test]
fn test_enter_sequence_scope_clears_previous_keys() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    stack.add_key("name", 2).unwrap();
    stack.add_key("value", 3).unwrap();
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 2);

    // New sequence item should clear keys
    stack.enter_sequence_scope(2, 5);
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 0);
}

#[test]
fn test_enter_sequence_scope_allows_same_key_in_different_items() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    stack.add_key("name", 2).unwrap();
    stack.add_key("value", 3).unwrap();

    // New sequence item - same keys should be OK
    stack.enter_sequence_scope(2, 5);
    stack.add_key("name", 6).unwrap();
    stack.add_key("value", 7).unwrap();

    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 2);
}

#[test]
fn test_enter_sequence_scope_clears_deeper_scopes() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("parent".to_string()));
    stack.enter_scope(4, 2, Some("child".to_string()));
    assert_eq!(stack.depth(), 3);

    // Enter sequence scope at level 2 should clear level 4
    stack.enter_sequence_scope(2, 5);
    assert_eq!(stack.depth(), 2);
    assert!(stack.get_scope_at_level(4).is_none());
}

#[test]
fn test_enter_sequence_scope_at_different_levels() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    assert_eq!(stack.current_indent(), 2);

    stack.exit_to_scope(0);

    stack.enter_sequence_scope(4, 3);
    assert_eq!(stack.current_indent(), 4);

    stack.exit_to_scope(0);

    stack.enter_sequence_scope(6, 5);
    assert_eq!(stack.current_indent(), 6);
}

// =============================================================================
// Scope Isolation Tests
// =============================================================================

#[test]
fn test_scope_isolation_keys_not_shared_between_scopes() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("scope1".to_string()));
    stack.add_key("key1", 2).unwrap();
    stack.add_key("key2", 3).unwrap();

    stack.exit_to_scope(0);
    stack.enter_scope(2, 5, Some("scope2".to_string()));

    // scope2 should not have keys from scope1
    assert!(!stack.contains_key("key1"));
    assert!(!stack.contains_key("key2"));
}

#[test]
fn test_scope_isolation_with_deeply_nested_scopes() {
    let mut stack = ScopeStack::new(2);

    // Level 1
    stack.enter_scope(2, 1, Some("l1".to_string()));
    stack.add_key("host", 2).unwrap();

    // Level 2
    stack.enter_scope(4, 3, Some("l2".to_string()));
    stack.add_key("host", 4).unwrap();

    // Level 3
    stack.enter_scope(6, 5, Some("l3".to_string()));
    stack.add_key("host", 6).unwrap();

    // Exit to level 2
    stack.exit_to_scope(4);
    assert!(stack.contains_key("host"));

    // Add another key at level 2
    stack.add_key("port", 7).unwrap();

    // Verify each scope has correct keys
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 2); // host, port at level 2
}

#[test]
fn test_scope_isolation_sibling_scopes_at_same_level() {
    let mut stack = ScopeStack::new(2);

    // Sibling 1
    stack.enter_scope(2, 1, Some("sibling1".to_string()));
    stack.add_key("key1", 2).unwrap();
    stack.add_key("key2", 3).unwrap();
    stack.exit_to_scope(0);

    // Sibling 2
    stack.enter_scope(2, 5, Some("sibling2".to_string()));

    // Sibling 2 should not have keys from sibling 1
    assert!(!stack.contains_key("key1"));
    assert!(!stack.contains_key("key2"));

    // Can add same keys to sibling 2
    stack.add_key("key1", 6).unwrap();
    stack.add_key("key2", 7).unwrap();
}

#[test]
fn test_scope_isolation_with_sequences() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("items".to_string()));

    // Sequence item 1
    stack.enter_sequence_scope(4, 2);
    stack.add_key("name", 3).unwrap();
    stack.add_key("value", 4).unwrap();

    // Sequence item 2
    stack.enter_sequence_scope(4, 5);

    // Item 2 should not have keys from item 1
    assert!(!stack.contains_key("name"));
    assert!(!stack.contains_key("value"));

    // Can add same keys to item 2
    stack.add_key("name", 6).unwrap();
    stack.add_key("value", 7).unwrap();
}

#[test]
fn test_scope_isolation_parent_vs_child() {
    let mut stack = ScopeStack::new(2);

    // Parent scope
    stack.enter_scope(2, 1, Some("parent".to_string()));
    stack.add_key("config", 2).unwrap();
    stack.add_key("timeout", 3).unwrap();

    // Child scope
    stack.enter_scope(4, 4, Some("child".to_string()));
    stack.add_key("config", 5).unwrap(); // Same key name OK
    stack.add_key("database", 6).unwrap();

    // Child should see its own keys
    assert!(stack.contains_key("config"));
    assert!(stack.contains_key("database"));

    // Exit to parent
    stack.exit_to_scope(2);

    // Parent should see its keys, not child's
    assert!(stack.contains_key("config"));
    assert!(stack.contains_key("timeout"));
    assert!(!stack.contains_key("database"));
}

// =============================================================================
// Duplicate Detection Tests
// =============================================================================

#[test]
fn test_duplicate_detection_in_root_scope() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("key1", 1).unwrap();
    let result = stack.add_key("key1", 2);

    assert!(result.is_err());

    let err = result.unwrap_err();
    assert_eq!(err.key, "key1");
    assert_eq!(err.duplicate_line, 2);
}

#[test]
fn test_duplicate_detection_in_nested_scope() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("parent".to_string()));
    stack.add_key("key1", 2).unwrap();

    let result = stack.add_key("key1", 3);
    assert!(result.is_err());

    let err = result.unwrap_err();
    assert_eq!(err.key, "key1");
    assert_eq!(err.scope_path, "parent");
}

#[test]
fn test_duplicate_detection_in_sequence_scope() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    stack.add_key("name", 2).unwrap();

    let result = stack.add_key("name", 3);
    assert!(result.is_err());

    let err = result.unwrap_err();
    assert_eq!(err.key, "name");
    assert_eq!(err.duplicate_line, 3);
}

#[test]
fn test_no_duplicate_false_positive_across_scopes() {
    let mut stack = ScopeStack::new(2);

    // Add to root
    stack.add_key("host", 1).unwrap();

    // Enter nested scope
    stack.enter_scope(2, 2, Some("services".to_string()));
    let result = stack.add_key("host", 3);

    assert!(result.is_ok(), "Same key in different scope should succeed");
}

#[test]
fn test_no_duplicate_false_positive_in_sequence_items() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    stack.add_key("name", 2).unwrap();

    // New sequence item
    stack.enter_sequence_scope(2, 4);
    let result = stack.add_key("name", 5);

    assert!(result.is_ok(), "Same key in different sequence item should succeed");
}

#[test]
fn test_no_duplicate_false_positive_sibling_scopes() {
    let mut stack = ScopeStack::new(2);

    // First sibling
    stack.enter_scope(2, 1, Some("sibling1".to_string()));
    stack.add_key("key", 2).unwrap();
    stack.exit_to_scope(0);

    // Second sibling
    stack.enter_scope(2, 4, Some("sibling2".to_string()));
    let result = stack.add_key("key", 5);

    assert!(result.is_ok(), "Same key in sibling scope should succeed");
}

// =============================================================================
// Helper Method Tests
// =============================================================================

#[test]
fn test_current_scope_returns_mutable_reference() {
    let mut stack = ScopeStack::new(2);

    stack.current_scope().add_key("test");
    assert!(stack.contains_key("test"));
}

#[test]
fn test_current_scope_ref_returns_immutable_reference() {
    let stack = ScopeStack::new(2);
    let scope = stack.current_scope_ref();
    assert_eq!(scope.indent_level, 0);
}

#[test]
fn test_get_scope_at_level_finds_existing_scope() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level2".to_string()));
    stack.enter_scope(4, 2, Some("level4".to_string()));
    stack.enter_scope(6, 3, Some("level6".to_string()));

    assert!(stack.get_scope_at_level(0).is_some());
    assert!(stack.get_scope_at_level(2).is_some());
    assert!(stack.get_scope_at_level(4).is_some());
    assert!(stack.get_scope_at_level(6).is_some());
}

#[test]
fn test_get_scope_at_level_returns_none_for_nonexistent() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level2".to_string()));
    stack.enter_scope(4, 2, Some("level4".to_string()));

    assert!(stack.get_scope_at_level(6).is_none());
    assert!(stack.get_scope_at_level(8).is_none());
}

#[test]
fn test_contains_key_checks_current_scope_only() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("root_key", 1).unwrap();
    stack.enter_scope(2, 2, Some("child".to_string()));
    stack.add_key("child_key", 3).unwrap();

    assert!(!stack.contains_key("root_key")); // Not in current scope
    assert!(stack.contains_key("child_key")); // In current scope
}

#[test]
fn test_contains_key_in_any_scope_checks_all_scopes() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("root_key", 1).unwrap();
    stack.enter_scope(2, 2, Some("level1".to_string()));
    stack.add_key("level1_key", 3).unwrap();
    stack.enter_scope(4, 4, Some("level2".to_string()));
    stack.add_key("level2_key", 5).unwrap();

    assert!(stack.contains_key_in_any_scope("root_key"));
    assert!(stack.contains_key_in_any_scope("level1_key"));
    assert!(stack.contains_key_in_any_scope("level2_key"));
}

#[test]
fn test_get_scope_path_builds_dot_separated_path() {
    let mut stack = ScopeStack::new(2);

    assert_eq!(stack.get_scope_path(), "");

    stack.enter_scope(2, 1, Some("services".to_string()));
    assert_eq!(stack.get_scope_path(), "services");

    stack.enter_scope(4, 2, Some("web".to_string()));
    assert_eq!(stack.get_scope_path(), "services.web");

    stack.enter_scope(6, 3, Some("config".to_string()));
    assert_eq!(stack.get_scope_path(), "services.web.config");
}

#[test]
fn test_depth_returns_number_of_scopes() {
    let stack = ScopeStack::new(2);
    assert_eq!(stack.depth(), 1);

    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("l1".to_string()));
    assert_eq!(stack.depth(), 2);

    stack.enter_scope(4, 2, Some("l2".to_string()));
    assert_eq!(stack.depth(), 3);

    stack.enter_scope(6, 3, Some("l3".to_string()));
    assert_eq!(stack.depth(), 4);
}

#[test]
fn test_reset_clears_all_scopes() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.add_key("key1", 2).unwrap();
    stack.enter_scope(4, 3, Some("level2".to_string()));
    stack.add_key("key2", 4).unwrap();

    assert_eq!(stack.depth(), 3);

    stack.reset();

    assert_eq!(stack.depth(), 1);
    assert!(!stack.contains_key("key1"));
    assert!(!stack.contains_key("key2"));
    assert_eq!(stack.current_indent(), 0);
}

#[test]
fn test_set_base_indent_changes_indent_size() {
    let mut stack = ScopeStack::new(2);

    assert_eq!(stack.base_indent(), 2);

    stack.set_base_indent(4);
    assert_eq!(stack.base_indent(), 4);

    stack.set_base_indent(8);
    assert_eq!(stack.base_indent(), 8);
}

#[test]
fn test_in_sequence_context_returns_correct_state() {
    let mut stack = ScopeStack::new(2);

    assert!(!stack.in_sequence_context());

    stack.enter_sequence_scope(2, 1);
    assert!(stack.in_sequence_context());

    stack.exit_to_scope(0);
    assert!(!stack.in_sequence_context());
}

// =============================================================================
// Edge Cases and Boundary Conditions
// =============================================================================

#[test]
fn test_multiple_enters_to_same_indent_clears_previous() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("first".to_string()));
    stack.add_key("key1", 2).unwrap();

    stack.exit_to_scope(0);

    stack.enter_scope(2, 5, Some("second".to_string()));

    // Should not have keys from first scope
    assert!(!stack.contains_key("key1"));
}

#[test]
fn test_enter_sequence_preserves_parent_until_exit() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("parent".to_string()));
    stack.add_key("parent_key", 2).unwrap();

    stack.enter_sequence_scope(4, 3);
    stack.add_key("seq_key", 4).unwrap();

    stack.exit_to_scope(2);

    // Parent scope should still have its key
    assert!(stack.contains_key("parent_key"));
    assert!(!stack.contains_key("seq_key"));
}

#[test]
fn test_very_deeply_nested_structure() {
    let mut stack = ScopeStack::new(2);

    for i in 0..10 {
        let indent = i * 2;
        stack.enter_scope(indent, i + 1, Some(format!("level{}", i)));
    }

    assert_eq!(stack.depth(), 11); // Root + 10 levels
}

#[test]
fn test_many_keys_at_single_level() {
    let mut stack = ScopeStack::new(2);

    for i in 0..50 {
        stack.add_key(&format!("key{}", i), i + 1).unwrap();
    }

    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 50);
}

#[test]
fn test_special_characters_in_keys() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("key-with-dashes", 1).unwrap();
    stack.add_key("key_with_underscores", 2).unwrap();
    stack.add_key("key.with.dots", 3).unwrap();

    assert!(stack.contains_key("key-with-dashes"));
    assert!(stack.contains_key("key_with_underscores"));
    assert!(stack.contains_key("key.with.dots"));
}

#[test]
fn test_unicode_keys() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("配置", 1).unwrap(); // Chinese
    stack.add_key("конфигурация", 2).unwrap(); // Cyrillic
    stack.add_key("configuración", 3).unwrap(); // Spanish

    assert!(stack.contains_key("配置"));
    assert!(stack.contains_key("конфигурация"));
    assert!(stack.contains_key("configuración"));
}

#[test]
fn test_numeric_string_keys() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("123", 1).unwrap();
    stack.add_key("456", 2).unwrap();

    assert!(stack.contains_key("123"));
    assert!(stack.contains_key("456"));

    // Should detect duplicate
    let result = stack.add_key("123", 3);
    assert!(result.is_err());
}

// =============================================================================
// Integration-style Tests
// =============================================================================

#[test]
fn test_complex_yaml_like_structure() {
    let mut stack = ScopeStack::new(2);

    // services:
    stack.enter_scope(2, 1, Some("services".to_string()));

    //   web:
    stack.enter_scope(4, 2, Some("web".to_string()));
    stack.add_key("host", 3).unwrap();
    stack.add_key("port", 4).unwrap();

    //   database:
    stack.exit_to_scope(2);
    stack.enter_scope(4, 5, Some("database".to_string()));
    // Same keys should be OK in different scope
    stack.add_key("host", 6).unwrap();
    stack.add_key("port", 7).unwrap();
    // Duplicate detection should work
    let result = stack.add_key("host", 8);
    assert!(result.is_err());
}

#[test]
fn test_sequence_of_mappings() {
    let mut stack = ScopeStack::new(2);

    // items:
    stack.enter_scope(2, 1, Some("items".to_string()));

    //   - name: item1
    stack.enter_sequence_scope(4, 2);
    stack.add_key("name", 3).unwrap();
    stack.add_key("value", 4).unwrap();

    //   - name: item2
    stack.enter_sequence_scope(4, 5);
    // Same keys OK in different sequence item
    stack.add_key("name", 6).unwrap();
    stack.add_key("value", 7).unwrap();

    // But duplicate within same item fails
    let result = stack.add_key("name", 8);
    assert!(result.is_err());
}

#[test]
fn test_mixed_regular_and_sequence_scopes_complex() {
    let mut stack = ScopeStack::new(2);

    // config:
    stack.enter_scope(2, 1, Some("config".to_string()));
    stack.add_key("timeout", 2).unwrap();

    //   services:
    stack.enter_scope(4, 3, Some("services".to_string()));

    //     - name: web
    stack.enter_sequence_scope(6, 4);
    stack.add_key("name", 5).unwrap();
    stack.add_key("port", 6).unwrap();

    //     - name: db
    stack.enter_sequence_scope(6, 7);
    stack.add_key("name", 8).unwrap();
    stack.add_key("port", 9).unwrap();

    // After sequence, still in services scope
    stack.exit_to_scope(4);
    assert!(!stack.in_sequence_context());
}
