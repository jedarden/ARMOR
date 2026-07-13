//! Edge case tests for exit_to_scope
//!
//! These tests verify that exit_to_scope correctly handles various edge cases
//! in scope transitions when indentation decreases.

use armor::parsers::yaml::scope::ScopeStack;

#[test]
fn test_exit_to_scope_with_parent_at_target() {
    let mut stack = ScopeStack::new(2);

    // Create nested scopes: services (2) -> web (4) -> config (6)
    stack.enter_scope(2, 1, Some("services".to_string()));
    stack.enter_scope(4, 2, Some("web".to_string()));
    stack.enter_scope(6, 3, Some("config".to_string()));

    // Exit from config (6) to services (2)
    stack.exit_to_scope(2);

    // Should have: root (0) + services (2)
    assert_eq!(stack.depth(), 2);
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.get_scope_path(), "services");
    assert!(stack.get_scope_at_level(2).is_some());
    assert!(stack.get_scope_at_level(6).is_none());
}

#[test]
fn test_exit_to_scope_without_parent_at_target() {
    let mut stack = ScopeStack::new(2);

    // Create: root (0) -> level4 (4)
    stack.enter_scope(4, 1, Some("level4".to_string()));

    // Exit to level 2 (which doesn't exist)
    stack.exit_to_scope(2);

    // Should create fallback scope at level 2
    assert_eq!(stack.depth(), 2); // root + fallback
    assert_eq!(stack.current_indent(), 2);
    assert!(stack.get_scope_at_level(2).is_some());
}

#[test]
fn test_exit_to_scope_to_root() {
    let mut stack = ScopeStack::new(2);

    // Create deep nesting
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(6, 3, Some("level3".to_string()));

    // Exit all the way to root
    stack.exit_to_scope(0);

    // Should only have root scope
    assert_eq!(stack.depth(), 1);
    assert_eq!(stack.current_indent(), 0);
    assert_eq!(stack.get_scope_path(), "");
}

#[test]
fn test_exit_to_scope_partial_depth() {
    let mut stack = ScopeStack::new(2);

    // Create: services (2) -> web (4) -> config (6) -> debug (8)
    stack.enter_scope(2, 1, Some("services".to_string()));
    stack.enter_scope(4, 2, Some("web".to_string()));
    stack.enter_scope(6, 3, Some("config".to_string()));
    stack.enter_scope(8, 4, Some("debug".to_string()));

    // Exit from debug (8) to web (4) - partial exit
    stack.exit_to_scope(4);

    // Should have: root (0) + services (2) + web (4)
    assert_eq!(stack.depth(), 3);
    assert_eq!(stack.current_indent(), 4);
    assert_eq!(stack.get_scope_path(), "services.web");
    assert!(stack.get_scope_at_level(6).is_none());
    assert!(stack.get_scope_at_level(8).is_none());
}

#[test]
fn test_exit_to_scope_handles_indent_not_multiple_of_base() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));

    // Exit to indent 3 (not a multiple of base_indent=2)
    // Should handle gracefully by creating a fallback or adjusting
    stack.exit_to_scope(3);

    // Current behavior: creates fallback at indent 3
    assert_eq!(stack.current_indent(), 3);
}

#[test]
fn test_exit_to_scope_does_not_exit_to_deeper_level() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));

    let current_depth_before = stack.depth();
    let current_indent_before = stack.current_indent();

    // Try to exit to a deeper level (should be ignored)
    stack.exit_to_scope(6);

    // Should remain unchanged
    assert_eq!(stack.depth(), current_depth_before);
    assert_eq!(stack.current_indent(), current_indent_before);
}

#[test]
fn test_exit_to_scope_from_sequence_context() {
    let mut stack = ScopeStack::new(2);

    // Create parent mapping
    stack.enter_scope(2, 1, Some("items".to_string()));

    // Enter sequence context
    stack.enter_sequence_scope(4, 2);

    // Add key in sequence scope
    stack.add_key("name", 3).unwrap();

    // Exit from sequence back to parent
    stack.exit_to_scope(2);

    // Should be back at items scope
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.get_scope_path(), "items");
    assert!(!stack.contains_key("name")); // Key from exited scope should be gone
}

#[test]
fn test_exit_to_scope_sibling_transition() {
    let mut stack = ScopeStack::new(2);

    // First sibling
    stack.enter_scope(2, 1, Some("sibling1".to_string()));
    stack.add_key("key1", 2).unwrap();
    stack.exit_to_scope(0);

    // Second sibling
    stack.enter_scope(2, 3, Some("sibling2".to_string()));

    // Should not have key from previous sibling
    assert!(!stack.contains_key("key1"));
    assert_eq!(stack.get_scope_path(), "sibling2");
}

#[test]
fn test_exit_to_scope_complex_nesting_with_gaps() {
    let mut stack = ScopeStack::new(2);

    // Create: services (2) -> (gap at 4) -> config (6) -> (gap at 8) -> debug (10)
    stack.enter_scope(2, 1, Some("services".to_string()));
    stack.enter_scope(6, 2, Some("config".to_string()));
    stack.enter_scope(10, 3, Some("debug".to_string()));

    // Exit from debug (10) to services (2)
    stack.exit_to_scope(2);

    // Should have: root (0) + services (2)
    assert_eq!(stack.depth(), 2);
    assert_eq!(stack.current_indent(), 2);
    assert!(stack.get_scope_at_level(10).is_none());
}

#[test]
fn test_exit_to_scope_multiple_times_in_sequence() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("a".to_string()));
    stack.enter_scope(4, 2, Some("b".to_string()));
    stack.enter_scope(6, 3, Some("c".to_string()));

    // Exit from 6 -> 4
    stack.exit_to_scope(4);
    assert_eq!(stack.depth(), 3);
    assert_eq!(stack.current_indent(), 4);

    // Exit from 4 -> 2
    stack.exit_to_scope(2);
    assert_eq!(stack.depth(), 2);
    assert_eq!(stack.current_indent(), 2);

    // Exit from 2 -> 0
    stack.exit_to_scope(0);
    assert_eq!(stack.depth(), 1);
    assert_eq!(stack.current_indent(), 0);
}

#[test]
fn test_exit_to_scope_state_cleanup() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("parent".to_string()));
    stack.add_key("parent_key", 2).unwrap();

    stack.enter_scope(4, 2, Some("child".to_string()));
    stack.add_key("child_key", 3).unwrap();

    // Exit to parent
    stack.exit_to_scope(2);

    // Parent key should still exist
    assert!(stack.contains_key("parent_key"));
    // Child key should be gone
    assert!(!stack.contains_key("child_key"));
    // Should be at parent scope
    assert_eq!(stack.current_scope_ref().parent_key, Some("parent".to_string()));
}

#[test]
fn test_exit_to_scope_when_target_has_no_scope_but_parent_exists() {
    let mut stack = ScopeStack::new(2);

    // Create: root (0) -> level6 (6)
    stack.enter_scope(6, 1, Some("deep".to_string()));

    // Exit to level 4 (which doesn't exist, but we have root at 0)
    stack.exit_to_scope(4);

    // Should create fallback at 4
    assert_eq!(stack.depth(), 2); // root + fallback
    assert_eq!(stack.current_indent(), 4);
    assert!(stack.get_scope_at_level(4).is_some());
}
