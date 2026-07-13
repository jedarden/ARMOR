//! Edge case tests for exit_to_scope
//!
//! These tests verify that exit_to_scope correctly handles various edge cases
//! in scope transitions when indentation decreases.

use armor::parsers::yaml::scope::ScopeStack;

/// Scenario: Exit from a deeply nested scope to a parent scope that exists in the stack
/// Expected: Should exit to the parent scope, removing all deeper scopes
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

/// Scenario: Exit to a target indent level where no scope exists
/// Expected: Should find and use the closest parent scope (root at indent 0)
#[test]
fn test_exit_to_scope_without_parent_at_target() {
    let mut stack = ScopeStack::new(2);

    // Create: root (0) -> level4 (4)
    stack.enter_scope(4, 1, Some("level4".to_string()));

    // Exit to level 2 (which doesn't exist)
    stack.exit_to_scope(2);

    // Should find closest parent (root at indent 0)
    assert_eq!(stack.depth(), 1); // only root
    assert_eq!(stack.current_indent(), 0);
    assert!(stack.get_scope_at_level(0).is_some());
}

/// Scenario: Exit all the way from a deeply nested scope back to root
/// Expected: Should remove all scopes except root, returning to initial state
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

/// Scenario: Exit from a deep scope to an intermediate parent (partial exit, not all the way to root)
/// Expected: Should exit to the intermediate scope, preserving parent chain
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

/// Scenario: Exit to an indent level that is not a multiple of the base_indent
/// Expected: Should handle gracefully by creating a fallback scope
/// Scenario: Exit to an indent level that is not a multiple of the base_indent
/// Expected: Should find and use the closest parent scope (level2 at indent 2)
#[test]
fn test_exit_to_scope_handles_indent_not_multiple_of_base() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));

    // Exit to indent 3 (not a multiple of base_indent=2)
    // Should handle gracefully by finding closest parent
    stack.exit_to_scope(3);

    // Current behavior: finds closest parent (level2 at indent 2)
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.get_scope_path(), "level1");
}

/// Scenario: Attempt to exit to a deeper indent level than current (invalid operation)
/// Expected: Should ignore the request and remain at current indent
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

/// Scenario: Exit from a sequence scope back to its parent mapping scope
/// Expected: Should properly clean up sequence scope state and return to parent
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

/// Scenario: Exit from one sibling scope and enter another at the same level
/// Expected: Keys from first sibling should not be visible in second sibling
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

/// Scenario: Exit from deep scope through a stack that has gaps in indent levels
/// Expected: Should correctly unwind through gaps to reach target scope
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

/// Scenario: Perform multiple sequential exit operations at decreasing indent levels
/// Expected: Should correctly unwind the stack step by step
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

/// Scenario: Exit from child scope back to parent, verify state cleanup
/// Expected: Parent keys should remain, child keys should be removed
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
    assert_eq!(stack.current_scope_ref().unwrap().parent_key, Some("parent".to_string()));
}

/// Scenario: Exit to an indent level that doesn't exist when root is the only parent
/// Expected: Should find and use the closest parent scope (root at indent 0)
#[test]
fn test_exit_to_scope_when_target_has_no_scope_but_parent_exists() {
    let mut stack = ScopeStack::new(2);

    // Create: root (0) -> level6 (6)
    stack.enter_scope(6, 1, Some("deep".to_string()));

    // Exit to level 4 (which doesn't exist, but we have root at 0)
    stack.exit_to_scope(4);

    // Should find closest parent (root at indent 0)
    assert_eq!(stack.depth(), 1); // only root
    assert_eq!(stack.current_indent(), 0);
    assert!(stack.get_scope_at_level(0).is_some());
}

/// Scenario: Attempting to exit to root when already at root
/// Expected: Operation is idempotent (no-op), stack state unchanged
#[test]
fn test_exit_to_scope_from_root_to_root_is_idempotent() {
    let mut stack = ScopeStack::new(2);

    // Start at root (initial state)
    let depth_before = stack.depth();
    let indent_before = stack.current_indent();

    // Try to exit to root (already at root)
    stack.exit_to_scope(0);

    // Should remain unchanged
    assert_eq!(stack.depth(), depth_before);
    assert_eq!(stack.current_indent(), indent_before);
    assert_eq!(stack.get_scope_path(), "");
}

/// Scenario: Stack only has root scope, attempt to exit to root
/// Expected: Should remain in root state (no-op)
#[test]
fn test_exit_to_scope_from_stack_with_only_root() {
    let mut stack = ScopeStack::new(2);

    // Stack only has root scope
    assert_eq!(stack.depth(), 1);
    assert_eq!(stack.current_indent(), 0);

    // Try to exit to root (should be idempotent)
    stack.exit_to_scope(0);

    // Should still have only root scope
    assert_eq!(stack.depth(), 1);
    assert_eq!(stack.current_indent(), 0);
}

/// Scenario: Exit to an indent level that doesn't exist but falls between two existing scopes
/// Expected: Should find and use the closest parent scope (level2 at indent 2)
#[test]
fn test_exit_to_scope_to_nonexistent_level_between_existing_scopes() {
    let mut stack = ScopeStack::new(2);

    // Create: root (0) -> level2 (2) -> level6 (6)
    stack.enter_scope(2, 1, Some("level2".to_string()));
    stack.enter_scope(6, 2, Some("level6".to_string()));

    // Exit to level 4 (doesn't exist, between 2 and 6)
    stack.exit_to_scope(4);

    // Should find closest parent (level2 at indent 2) and keep scope at 2
    assert_eq!(stack.depth(), 2); // root (0) + level2 (2)
    assert_eq!(stack.current_indent(), 2);
    assert!(stack.get_scope_at_level(2).is_some());
    assert!(stack.get_scope_at_level(6).is_none());
    assert_eq!(stack.get_scope_path(), "level2");
}

/// Scenario: Perform multiple scope exits in rapid succession without intermediate operations
/// Expected: Should correctly unwind the stack to each target level
#[test]
fn test_exit_to_scope_rapid_successive_exits() {
    let mut stack = ScopeStack::new(2);

    // Create deep nesting
    stack.enter_scope(2, 1, Some("a".to_string()));
    stack.enter_scope(4, 2, Some("b".to_string()));
    stack.enter_scope(6, 3, Some("c".to_string()));
    stack.enter_scope(8, 4, Some("d".to_string()));

    // Rapid successive exits without intermediate operations
    stack.exit_to_scope(6);  // 8 -> 6
    stack.exit_to_scope(2);  // 6 -> 2
    stack.exit_to_scope(0);  // 2 -> 0

    // Should end at root
    assert_eq!(stack.depth(), 1);
    assert_eq!(stack.current_indent(), 0);
    assert_eq!(stack.get_scope_path(), "");
}

/// Scenario: Verify that root scope is never removed even during aggressive scope exits
/// Expected: Root scope (indent 0) should always be preserved in the stack
#[test]
fn test_exit_to_scope_preserves_root_scope_even_in_edge_cases() {
    let mut stack = ScopeStack::new(2);

    // Create deep nesting
    stack.enter_scope(10, 1, Some("deep".to_string()));

    // Exit to various levels, root should always be preserved
    stack.exit_to_scope(5);
    assert!(stack.get_scope_at_level(0).is_some()); // Root still exists

    stack.exit_to_scope(2);
    assert!(stack.get_scope_at_level(0).is_some()); // Root still exists

    stack.exit_to_scope(0);
    assert!(stack.get_scope_at_level(0).is_some()); // Root still exists
    assert_eq!(stack.depth(), 1); // Only root remains
}

/// Scenario: Exit from a sequence scope back to parent mapping scope
/// Expected: Parent scope should not inherit sequence item ID from child
#[test]
fn test_exit_to_scope_sequence_item_id_preservation_in_parent() {
    let mut stack = ScopeStack::new(2);

    // Create parent mapping
    stack.enter_scope(2, 1, Some("items".to_string()));

    // Enter sequence scope
    stack.enter_sequence_scope(4, 2);
    let item_id = stack.current_scope_ref().unwrap().sequence_item_id;

    // Exit back to parent
    stack.exit_to_scope(2);

    // Parent scope should not have sequence item ID
    assert!(stack.current_scope_ref().unwrap().sequence_item_id.is_none());
    assert_eq!(stack.current_scope_ref().unwrap().parent_key, Some("items".to_string()));
}

/// Scenario: Exit from child scope back to parent that was marked as flow-style
/// Expected: Parent scope's flow_style flag should be preserved
#[test]
fn test_exit_to_scope_with_flow_style_preservation() {
    let mut stack = ScopeStack::new(2);

    // Create scope and mark as flow style
    stack.enter_scope(2, 1, Some("flow_style_scope".to_string()));
    stack.current_scope().unwrap().is_flow_style = true;

    // Enter child scope
    stack.enter_scope(4, 2, Some("child".to_string()));

    // Exit back to parent
    stack.exit_to_scope(2);

    // Flow style should be preserved on parent scope
    assert!(stack.current_scope_ref().unwrap().is_flow_style);
    assert_eq!(stack.current_scope_ref().unwrap().parent_key, Some("flow_style_scope".to_string()));
}

/// Scenario: Attempt to exit to the same indent level we're currently at
/// Expected: Should be a no-op (already at target)
#[test]
fn test_exit_to_scope_when_target_is_same_as_current_indent() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));

    let depth_before = stack.depth();
    let indent_before = stack.current_indent();

    // Exit to current indent (4)
    stack.exit_to_scope(4);

    // Should remain unchanged (already at target)
    assert_eq!(stack.depth(), depth_before);
    assert_eq!(stack.current_indent(), indent_before);
    assert_eq!(stack.get_scope_path(), "level1.level2");
}

/// Scenario: Verify that scopes with many keys are properly cleaned up on exit
/// Expected: All keys should be cleared and no references remain to the exited scope
#[test]
fn test_exit_to_scope_clears_large_scope_data() {
    let mut stack = ScopeStack::new(2);

    // Create a parent scope with many keys
    stack.enter_scope(2, 1, Some("parent".to_string()));
    for i in 0..100 {
        stack.add_key(&format!("key_{}", i), 2 + i).unwrap();
    }
    let parent_key_count = stack.current_scope_ref().unwrap().key_count();
    assert_eq!(parent_key_count, 100);

    // Create a child scope with even more keys
    stack.enter_scope(4, 102, Some("child".to_string()));
    for i in 0..200 {
        stack.add_key(&format!("child_key_{}", i), 103 + i).unwrap();
    }
    let child_key_count = stack.current_scope_ref().unwrap().key_count();
    assert_eq!(child_key_count, 200);

    // Exit to parent - child should be cleaned up
    stack.exit_to_scope(2);

    // Parent should still have its keys
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 100);
    assert!(stack.contains_key("key_0"));
    assert!(stack.contains_key("key_99"));

    // Child keys should be completely gone
    assert!(!stack.contains_key("child_key_0"));
    assert!(!stack.contains_key("child_key_199"));
}

/// Scenario: Verify that sequence context flags are properly reset on scope exit
/// Expected: Exiting from sequence scope should reset sequence flags in parent
#[test]
fn test_exit_to_scope_resets_sequence_context_flags() {
    let mut stack = ScopeStack::new(2);

    // Create parent mapping
    stack.enter_scope(2, 1, Some("items".to_string()));

    // Enter sequence scope (sets in_sequence_context=true on new scope)
    stack.enter_sequence_scope(4, 2);
    assert!(stack.current_scope_ref().unwrap().in_sequence_context);
    assert!(stack.current_scope_ref().unwrap().sequence_item_id.is_some());

    // Add some keys in sequence scope
    stack.add_key("seq_key1", 3).unwrap();
    stack.add_key("seq_key2", 4).unwrap();

    // Exit back to parent mapping
    stack.exit_to_scope(2);

    // Verify we're back in mapping context (not sequence)
    assert!(!stack.current_scope_ref().unwrap().in_sequence_context);
    assert!(stack.current_scope_ref().unwrap().sequence_item_id.is_none());

    // Verify sequence keys are gone
    assert!(!stack.contains_key("seq_key1"));
    assert!(!stack.contains_key("seq_key2"));
}

/// Scenario: Verify that nested scope cleanup doesn't leave intermediate state
/// Expected: Multiple levels of nested scopes should all be cleaned up properly
#[test]
fn test_exit_to_scope_cleanup_multiple_nested_levels() {
    let mut stack = ScopeStack::new(2);

    // Create deep nesting with keys at each level
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.add_key("l1_key", 2).unwrap();

    stack.enter_scope(4, 3, Some("level2".to_string()));
    stack.add_key("l2_key", 4).unwrap();

    stack.enter_scope(6, 5, Some("level3".to_string()));
    stack.add_key("l3_key", 6).unwrap();

    stack.enter_scope(8, 7, Some("level4".to_string()));
    stack.add_key("l4_key", 8).unwrap();

    // Exit directly to root - all nested scopes should be cleaned up
    stack.exit_to_scope(0);

    // Should be at root with no keys from nested scopes
    assert_eq!(stack.depth(), 1);
    assert_eq!(stack.current_indent(), 0);
    assert!(!stack.contains_key("l1_key"));
    assert!(!stack.contains_key("l2_key"));
    assert!(!stack.contains_key("l3_key"));
    assert!(!stack.contains_key("l4_key"));
}

/// Scenario: Verify that scope cleanup works when exiting through gaps in indent levels
/// Expected: Gaps in indent levels should not prevent proper cleanup
#[test]
fn test_exit_to_scope_cleanup_with_indent_gaps() {
    let mut stack = ScopeStack::new(2);

    // Create scopes with gaps: 0 -> 2 -> 6 -> 10
    stack.enter_scope(2, 1, Some("gap1".to_string()));
    stack.add_key("gap1_key", 2).unwrap();

    stack.enter_scope(6, 3, Some("gap2".to_string()));
    stack.add_key("gap2_key", 4).unwrap();

    stack.enter_scope(10, 5, Some("gap3".to_string()));
    stack.add_key("gap3_key", 6).unwrap();

    // Exit from indent 10 to indent 2 (skipping indent 6)
    stack.exit_to_scope(2);

    // Should have cleaned up indent 6 and 10
    assert_eq!(stack.depth(), 2); // root + gap1
    assert_eq!(stack.current_indent(), 2);
    assert!(stack.contains_key("gap1_key"));
    assert!(!stack.contains_key("gap2_key"));
    assert!(!stack.contains_key("gap3_key"));
}

/// Scenario: Verify that rapid successive exits don't leave stale state
/// Expected: Each exit should fully clean up before the next exit
#[test]
fn test_exit_to_scope_rapid_exits_no_stale_state() {
    let mut stack = ScopeStack::new(2);

    // Create deep nesting
    stack.enter_scope(2, 1, Some("a".to_string()));
    stack.add_key("a_key", 2).unwrap();

    stack.enter_scope(4, 3, Some("b".to_string()));
    stack.add_key("b_key", 4).unwrap();

    stack.enter_scope(6, 5, Some("c".to_string()));
    stack.add_key("c_key", 6).unwrap();

    stack.enter_scope(8, 7, Some("d".to_string()));
    stack.add_key("d_key", 8).unwrap();

    // Rapid exits
    stack.exit_to_scope(6);  // Remove d
    stack.exit_to_scope(2);  // Remove c and b
    stack.exit_to_scope(0);  // Remove a

    // Verify all state is cleaned up
    assert_eq!(stack.depth(), 1);
    assert_eq!(stack.current_indent(), 0);
    assert_eq!(stack.get_scope_path(), "");

    // Verify none of the keys exist
    assert!(!stack.contains_key("a_key"));
    assert!(!stack.contains_key("b_key"));
    assert!(!stack.contains_key("c_key"));
    assert!(!stack.contains_key("d_key"));
}

/// Scenario: Verify that re-entering a scope after exit gets fresh state
/// Expected: Re-entering should not inherit state from previous scope at same level
#[test]
fn test_exit_to_scope_allows_clean_reentry() {
    let mut stack = ScopeStack::new(2);

    // First scope at indent 2
    stack.enter_scope(2, 1, Some("first".to_string()));
    stack.add_key("old_key", 2).unwrap();
    stack.current_scope().unwrap().is_flow_style = true;

    // Exit and re-enter at same level
    stack.exit_to_scope(0);
    stack.enter_scope(2, 3, Some("second".to_string()));

    // Should have fresh state, not inheriting from previous scope
    assert!(!stack.contains_key("old_key"));
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 0);
    assert!(!stack.current_scope_ref().unwrap().is_flow_style);
}
