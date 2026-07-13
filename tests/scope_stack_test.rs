//! Comprehensive scope stack verification tests
//!
//! This test file verifies the scope stack behavior with focus on:
//! - Empty stack initialization at startup
//! - Push/pop (enter/exit) sequence state maintenance
//! - Stack depth tracking matching nested scope depth

use armor::parsers::yaml::scope::ScopeStack;

/// Test that verifies empty stack at startup
#[test]
fn test_empty_stack_at_startup() {
    let stack = ScopeStack::new(2);

    // Verify stack starts empty (no scopes)
    assert_eq!(stack.depth(), 0, "Stack should start with depth 0");
    assert_eq!(stack.current_indent(), 0, "Current indent should be 0 when empty");
    assert!(stack.current_scope_ref().is_none(), "No current scope when stack is empty");
    assert!(stack.scopes.is_empty(), "Scopes vector should be empty");

    println!("✓ Empty stack at startup verified:");
    println!("  - depth: {}", stack.depth());
    println!("  - current_indent: {}", stack.current_indent());
    println!("  - current_scope: None");
}

/// Test that verifies push (enter_scope) and pop (exit_to_scope) sequence maintains correct state
#[test]
fn test_push_pop_sequence_maintains_state() {
    let mut stack = ScopeStack::new(2);

    // Auto-create root scope by adding a key
    stack.add_key("root_key", 1).unwrap();
    assert_eq!(stack.depth(), 1, "Should have root scope after first add_key");
    assert_eq!(stack.current_indent(), 0, "Root scope at indent 0");

    // Push: Enter nested scope (level 1)
    stack.enter_scope(2, 2, Some("level1".to_string()));
    assert_eq!(stack.depth(), 2, "Depth should be 2 after entering level1");
    assert_eq!(stack.current_indent(), 2, "Current indent should be 2");
    assert_eq!(stack.get_scope_path(), "level1", "Path should show level1");

    // Push: Enter deeper scope (level 2)
    stack.enter_scope(4, 3, Some("level2".to_string()));
    assert_eq!(stack.depth(), 3, "Depth should be 3 after entering level2");
    assert_eq!(stack.current_indent(), 4, "Current indent should be 4");
    assert_eq!(stack.get_scope_path(), "level1.level2", "Path should show full hierarchy");

    // Add keys to verify scope isolation
    stack.add_key("key1", 4).unwrap();
    stack.add_key("key2", 5).unwrap();
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 2, "Level2 should have 2 keys");

    // Pop: Exit to level1
    stack.exit_to_scope(2);
    assert_eq!(stack.depth(), 2, "Depth should be 2 after exiting to level1");
    assert_eq!(stack.current_indent(), 2, "Current indent should be 2");
    assert_eq!(stack.get_scope_path(), "level1", "Path should show only level1");

    // Verify keys were cleared (different scope)
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 0, "Level1 should have no keys from level2");

    // Pop: Exit to root
    stack.exit_to_scope(0);
    assert_eq!(stack.depth(), 1, "Depth should be 1 after exiting to root");
    assert_eq!(stack.current_indent(), 0, "Current indent should be 0");
    assert_eq!(stack.get_scope_path(), "", "Root path should be empty");

    println!("✓ Push/pop sequence maintains correct state:");
    println!("  - Root scope auto-created on first add_key");
    println!("  - Enter scope increases depth and updates indent");
    println!("  - Exit scope decreases depth and restores parent state");
    println!("  - Scope isolation maintained across transitions");
}

/// Test that verifies stack depth tracking matches nested scope depth
#[test]
fn test_stack_depth_tracking_matches_nested_scope_depth() {
    let mut stack = ScopeStack::new(2);

    // Auto-create root scope
    stack.add_key("root", 1).unwrap();
    assert_eq!(stack.depth(), 1, "Initial depth = 1 (root only)");

    // Create nested scopes and verify depth increases with each level
    stack.enter_scope(2, 2, Some("level1".to_string()));
    assert_eq!(stack.depth(), 2, "Depth = 2 (root + level1)");
    assert_eq!(stack.scopes.len(), 2, "Scopes vector length = 2");

    stack.enter_scope(4, 3, Some("level2".to_string()));
    assert_eq!(stack.depth(), 3, "Depth = 3 (root + level1 + level2)");
    assert_eq!(stack.scopes.len(), 3, "Scopes vector length = 3");

    stack.enter_scope(6, 4, Some("level3".to_string()));
    assert_eq!(stack.depth(), 4, "Depth = 4 (root + level1 + level2 + level3)");
    assert_eq!(stack.scopes.len(), 4, "Scopes vector length = 4");

    stack.enter_scope(8, 5, Some("level4".to_string()));
    assert_eq!(stack.depth(), 5, "Depth = 5 (root + 4 nested levels)");
    assert_eq!(stack.scopes.len(), 5, "Scopes vector length = 5");

    // Verify each scope in the hierarchy
    let hierarchy = &stack.scopes;
    assert_eq!(hierarchy[0].indent_level, 0, "Root at indent 0");
    assert_eq!(hierarchy[1].indent_level, 2, "Level1 at indent 2");
    assert_eq!(hierarchy[2].indent_level, 4, "Level2 at indent 4");
    assert_eq!(hierarchy[3].indent_level, 6, "Level3 at indent 6");
    assert_eq!(hierarchy[4].indent_level, 8, "Level4 at indent 8");

    // Verify scope path reflects full depth
    let path = stack.get_scope_path();
    assert_eq!(path, "level1.level2.level3.level4", "Scope path shows all 4 nested levels");

    println!("✓ Stack depth tracking matches nested scope depth:");
    println!("  - Created 5 nested scopes (root + 4 levels)");
    println!("  - depth() correctly reports {}", stack.depth());
    println!("  - scopes.len() correctly reports {}", hierarchy.len());
    println!("  - Each level tracked at correct indent");
    println!("  - Full scope path: {}", path);
}

/// Test that verifies state preservation across multiple push/pop cycles
#[test]
fn test_state_preservation_across_cycles() {
    let mut stack = ScopeStack::new(2);

    // First cycle: push and pop
    stack.add_key("root", 1).unwrap();
    stack.enter_scope(2, 2, Some("first".to_string()));
    stack.add_key("key1", 3).unwrap();

    assert_eq!(stack.depth(), 2, "Depth after first push");
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 1, "One key in first scope");

    stack.exit_to_scope(0);
    assert_eq!(stack.depth(), 1, "Back to root after first pop");

    // Second cycle: push again at same level (sibling scope)
    stack.enter_scope(2, 4, Some("second".to_string()));
    stack.add_key("key2", 5).unwrap();
    stack.add_key("key3", 6).unwrap();

    assert_eq!(stack.depth(), 2, "Depth after second push");
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 2, "Two keys in second scope");
    assert!(!stack.contains_key("key1"), "Previous scope's keys not present");

    // Third cycle: deeper nesting
    stack.enter_scope(4, 7, Some("nested".to_string()));
    stack.enter_scope(6, 8, Some("deep".to_string()));

    assert_eq!(stack.depth(), 4, "Depth after deeper nesting");
    assert_eq!(stack.get_scope_path(), "second.nested.deep", "Path shows full hierarchy");

    println!("✓ State preserved across multiple push/pop cycles:");
    println!("  - Sibling scopes properly isolated");
    println!("  - Keys cleared between scopes at same level");
    println!("  - Deep nesting tracked correctly");
}

/// Test that verifies push/pop with sequence scopes
#[test]
fn test_push_pop_with_sequence_scopes() {
    let mut stack = ScopeStack::new(2);

    // Auto-create root scope
    stack.add_key("root", 1).unwrap();

    // Push: Enter sequence scope
    stack.enter_sequence_scope(2, 2);
    assert_eq!(stack.depth(), 2, "Depth after entering sequence scope");
    assert!(stack.in_sequence_context(), "Should be in sequence context");
    assert_eq!(stack.current_scope_ref().unwrap().sequence_item_id, Some(1), "First sequence item");

    // Add keys to first sequence item
    stack.add_key("name", 3).unwrap();
    stack.add_key("value", 4).unwrap();
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 2, "First item has 2 keys");

    // Pop: Exit sequence scope
    stack.exit_to_scope(0);
    assert_eq!(stack.depth(), 1, "Depth after exiting sequence");
    assert!(!stack.in_sequence_context(), "Should not be in sequence context");

    // Push: Enter second sequence item
    stack.enter_sequence_scope(2, 5);
    assert_eq!(stack.depth(), 2, "Depth after entering second sequence item");
    assert_eq!(stack.current_scope_ref().unwrap().sequence_item_id, Some(2), "Second sequence item");
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 0, "Second item starts with no keys");

    // Verify same keys can be added to second item
    stack.add_key("name", 6).unwrap();
    stack.add_key("value", 7).unwrap();
    assert_eq!(stack.current_scope_ref().unwrap().key_count(), 2, "Second item also has 2 keys");

    println!("✓ Push/pop with sequence scopes works correctly:");
    println!("  - Sequence context properly set and cleared");
    println!("  - Sequence item IDs increment");
    println!("  - Keys isolated between sequence items");
}

/// Test that verifies empty state after reset
#[test]
fn test_reset_returns_to_empty_state() {
    let mut stack = ScopeStack::new(2);

    // Build up complex state
    stack.add_key("root", 1).unwrap();
    stack.enter_scope(2, 2, Some("level1".to_string()));
    stack.enter_scope(4, 3, Some("level2".to_string()));
    stack.enter_scope(6, 4, Some("level3".to_string()));
    stack.add_key("key1", 5).unwrap();
    stack.add_key("key2", 6).unwrap();

    assert_eq!(stack.depth(), 4, "Complex state built");

    // Reset
    stack.reset();

    // Verify empty state
    assert_eq!(stack.depth(), 0, "Depth = 0 after reset");
    assert_eq!(stack.current_indent(), 0, "Indent = 0 after reset");
    assert!(stack.current_scope_ref().is_none(), "No current scope after reset");
    assert!(stack.scopes.is_empty(), "Scopes vector empty after reset");

    // Verify can start fresh
    stack.add_key("new_root", 7).unwrap();
    assert_eq!(stack.depth(), 1, "Can auto-create new root after reset");

    println!("✓ Reset returns to empty state:");
    println!("  - All scopes cleared");
    println!("  - Can rebuild from scratch");
}

/// Test main function for running tests manually
fn main() {
    println!("Running scope stack verification tests...\n");

    test_empty_stack_at_startup();
    println!();

    test_push_pop_sequence_maintains_state();
    println!();

    test_stack_depth_tracking_matches_nested_scope_depth();
    println!();

    test_state_preservation_across_cycles();
    println!();

    test_push_pop_with_sequence_scopes();
    println!();

    test_reset_returns_to_empty_state();
    println!();

    println!("✅ All scope stack verification tests passed!");
}
