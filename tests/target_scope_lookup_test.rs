//! Comprehensive tests for target scope lookup when not in stack
//!
//! Bead: bf-wra91s
//!
//! These tests verify that the scope stack correctly handles cases where
//! a target scope is not found in the current stack:
//!
//! Test Coverage:
//! - Target scope is correctly located in hierarchy
//! - Missing target case is handled without panic
//! - Search scope hierarchy for target
//! - Closest parent scope is used when exact match not found
//! - Graceful fallback when no suitable parent exists

use armor::parsers::yaml::ScopeStack;

/// Test helper to verify scope state
fn verify_scope_state(stack: &ScopeStack, expected_indent: usize, expected_path: &str) {
    assert_eq!(stack.current_indent(), expected_indent, "Current indent should match expected");
    assert_eq!(stack.get_scope_path(), expected_path, "Scope path should match expected");
}

// =============================================================================
// Normal Cases - Target Scope Exists
// =============================================================================

#[test]
fn test_exit_to_scope_when_target_exists() {
    let mut stack = ScopeStack::new(2);

    // Create nested scopes
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(6, 3, Some("level3".to_string()));

    assert_eq!(stack.depth(), 4); // root + 3 levels

    // Exit to level2 (which exists)
    stack.exit_to_scope(4);

    // Should be at level2
    verify_scope_state(&stack, 4, "level1.level2");
    assert_eq!(stack.depth(), 3); // root + level1 + level2
}

#[test]
fn test_exit_to_root_scope() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));

    // Exit to root (indent 0)
    stack.exit_to_scope(0);

    verify_scope_state(&stack, 0, "");
    assert_eq!(stack.depth(), 1); // only root
}

// =============================================================================
// Target Scope Not Found - Closest Parent Exists
// =============================================================================

#[test]
fn test_exit_to_scope_uses_closest_parent_when_exact_not_found() {
    let mut stack = ScopeStack::new(2);

    // Create scopes at indents 0, 4, 6 (skipping indent 2)
    stack.enter_scope(4, 1, Some("level4".to_string()));
    stack.enter_scope(6, 2, Some("level6".to_string()));

    // Try to exit to indent 2 (which doesn't exist)
    stack.exit_to_scope(2);

    // Should exit to closest parent (indent 0 = root)
    verify_scope_state(&stack, 0, "");
    assert_eq!(stack.depth(), 1); // only root
}

#[test]
fn test_exit_to_scope_finds_parent_in_middle_of_stack() {
    let mut stack = ScopeStack::new(2);

    // Create scopes at indents 0, 2, 8 (skipping indents 4, 6)
    stack.enter_scope(2, 1, Some("level2".to_string()));
    stack.enter_scope(8, 2, Some("level8".to_string()));

    assert_eq!(stack.depth(), 3);

    // Try to exit to indent 4 (which doesn't exist)
    // Closest parent is indent 2
    stack.exit_to_scope(4);

    // Should be at level2 (closest parent)
    verify_scope_state(&stack, 2, "level2");
    assert_eq!(stack.depth(), 2); // root + level2
}

#[test]
fn test_exit_to_scope_skips_multiple_levels() {
    let mut stack = ScopeStack::new(2);

    // Create deep nesting
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(8, 3, Some("level4".to_string())); // skipping indent 6

    // Exit from indent 8 to indent 4 (exists)
    stack.exit_to_scope(4);

    verify_scope_state(&stack, 4, "level1.level2");
}

// =============================================================================
// Target Scope Not Found - No Suitable Parent (Uses Root)
// =============================================================================

#[test]
fn test_exit_to_scope_uses_root_when_no_intermediate_parent() {
    let mut stack = ScopeStack::new(2);

    // Manually create scope at indent 6 (skipping indents 2, 4)
    stack.enter_scope(6, 1, Some("level6".to_string()));

    assert_eq!(stack.depth(), 2);

    // Try to exit to indent 4 (which doesn't exist)
    // Should find closest parent: root at indent 0
    stack.exit_to_scope(4);

    // Should exit to root (closest parent scope)
    verify_scope_state(&stack, 0, "");
    assert_eq!(stack.depth(), 1); // only root scope
}

#[test]
fn test_exit_to_scope_exits_to_root_without_intermediate_scopes() {
    let mut stack = ScopeStack::new(2);

    // Create deep scope directly from root
    stack.enter_scope(8, 1, Some("deep".to_string()));

    // Exit to non-existent indent 5
    // Should find closest parent: root at indent 0
    stack.exit_to_scope(5);

    // Should exit to root (closest parent scope)
    assert_eq!(stack.current_indent(), 0);
    assert_eq!(stack.get_scope_path(), "");
    assert!(!stack.current_scope_ref().in_sequence_context);
}

// =============================================================================
// Edge Cases
// =============================================================================

#[test]
fn test_exit_to_scope_same_indent_does_nothing() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    let before_depth = stack.depth();

    // Exit to same indent we're already at
    stack.exit_to_scope(2);

    // Should be unchanged
    assert_eq!(stack.depth(), before_depth);
    verify_scope_state(&stack, 2, "level1");
}

#[test]
fn test_exit_to_scope_ignores_deeper_target() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));

    let before_depth = stack.depth();
    let before_indent = stack.current_indent();

    // Try to exit to deeper indent (should be ignored)
    stack.exit_to_scope(4);

    // Should be unchanged
    assert_eq!(stack.depth(), before_depth);
    assert_eq!(stack.current_indent(), before_indent);
}

#[test]
fn test_exit_to_scope_from_complex_structure() {
    let mut stack = ScopeStack::new(2);

    // Create complex structure with gaps
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(6, 2, Some("level3".to_string())); // skipping indent 4
    stack.enter_scope(10, 3, Some("level5".to_string())); // skipping indents 6, 8

    // Exit to indent 4 (doesn't exist, closest parent is indent 2)
    stack.exit_to_scope(4);

    // Should be at level1
    verify_scope_state(&stack, 2, "level1");
}

#[test]
fn test_exit_to_scope_sequence_of_exits_to_missing_targets() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(6, 2, Some("level3".to_string()));
    stack.enter_scope(10, 3, Some("level5".to_string()));

    // First exit to non-existent indent 8 (should go to 6)
    stack.exit_to_scope(8);
    verify_scope_state(&stack, 6, "level1.level3");

    // Second exit to non-existent indent 4 (should go to 2)
    stack.exit_to_scope(4);
    verify_scope_state(&stack, 2, "level1");
}

// =============================================================================
// Integration with Keys
// =============================================================================

#[test]
fn test_exit_to_scope_preserves_keys_in_parent() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("parent".to_string()));
    stack.add_key("key1", 2).unwrap();

    stack.enter_scope(6, 3, Some("child".to_string()));
    stack.add_key("key2", 4).unwrap();

    // Exit to non-existent indent 4 (should use parent at indent 2)
    stack.exit_to_scope(4);

    // Should have parent's keys
    assert!(stack.contains_key("key1"));
    assert!(!stack.contains_key("key2"));
    verify_scope_state(&stack, 2, "parent");
}

#[test]
fn test_exit_to_scope_with_sequence_context() {
    let mut stack = ScopeStack::new(2);

    // Create parent scope
    stack.enter_scope(2, 1, Some("items".to_string()));

    // Enter sequence scope
    stack.enter_sequence_scope(4, 2);
    stack.add_key("name", 3).unwrap();

    assert!(stack.in_sequence_context());

    // Exit to non-existent indent 3 (should use parent at indent 2)
    stack.exit_to_scope(3);

    // Should be back in parent context
    assert!(!stack.in_sequence_context());
    verify_scope_state(&stack, 2, "items");
}

#[test]
fn test_exit_to_scope_deeply_nested_sequence() {
    let mut stack = ScopeStack::new(2);

    // Create complex structure: mapping -> sequence -> mapping
    stack.enter_scope(2, 1, Some("root".to_string()));
    stack.enter_sequence_scope(4, 2);
    stack.enter_scope(8, 3, Some("config".to_string()));
    stack.add_key("setting", 4).unwrap();

    // Exit from indent 8 to indent 4 (sequence)
    stack.exit_to_scope(4);

    // Should be in sequence context
    assert!(stack.in_sequence_context());
    assert!(!stack.contains_key("setting"));
}

// =============================================================================
// Real-World Scenarios
// =============================================================================]

#[test]
fn test_exit_to_scope_yaml_with_inconsistent_indentation() {
    let mut stack = ScopeStack::new(2);

    // Simulate YAML with inconsistent indentation:
    // level1:
    //       level3:  # inconsistent indent (6 spaces instead of 4)
    //   level2:      # back to 4 spaces

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(6, 2, Some("level3".to_string()));

    // Exit to indent 4 (which doesn't exist)
    stack.exit_to_scope(4);

    // Should handle gracefully - use closest parent (indent 2)
    verify_scope_state(&stack, 2, "level1");

    // Now add the level2 scope properly
    stack.enter_scope(4, 3, Some("level2".to_string()));
    verify_scope_state(&stack, 4, "level1.level2");
}

#[test]
fn test_exit_to_scope_after_blank_line_with_indent_change() {
    let mut stack = ScopeStack::new(2);

    // Simulate:
    // level1:
    //   level2:
    //     key: value
    //
    // sibling:  # blank line at indent 0, then new key

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(6, 3, Some("level3".to_string()));

    // Process blank line indent decrease (from 6 to 0)
    stack.exit_to_scope(0);

    // Should be at root
    verify_scope_state(&stack, 0, "");
}

// =============================================================================
// Stress Tests
// =============================================================================

#[test]
fn test_exit_to_scope_multiple_calls_same_target() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(6, 2, Some("level3".to_string()));

    // Call exit_to_scope multiple times with same target
    stack.exit_to_scope(4);
    let after_first = stack.current_indent();

    stack.exit_to_scope(4);
    let after_second = stack.current_indent();

    // Should stabilize at same indent
    assert_eq!(after_first, after_second);
}

#[test]
fn test_exit_to_scope_handles_zero_indent() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));

    // Exit to root (indent 0)
    stack.exit_to_scope(0);

    verify_scope_state(&stack, 0, "");
    assert_eq!(stack.depth(), 1);
}

#[test]
fn test_exit_to_scope_preserves_scope_integrity() {
    let mut stack = ScopeStack::new(2);

    // Create multiple scopes
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.add_key("key1", 2).unwrap();

    stack.enter_scope(6, 3, Some("level3".to_string()));
    stack.add_key("key3", 4).unwrap();

    stack.enter_scope(10, 5, Some("level5".to_string()));
    stack.add_key("key5", 6).unwrap();

    // Exit to non-existent indent
    stack.exit_to_scope(7);

    // Verify stack integrity:
    // - Should have exited to closest parent
    // - Should not have orphaned scopes
    // - Should maintain consistent depth
    assert!(stack.depth() >= 1); // at least root
    assert!(stack.depth() <= 3); // at most root + 2 levels
    assert!(stack.current_indent() <= 6); // at most level3
}
