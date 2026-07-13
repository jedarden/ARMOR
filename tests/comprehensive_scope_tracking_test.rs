//! Comprehensive Scope Tracking Tests
//!
//! These tests provide comprehensive coverage of the scope tracking system,
//! including advanced scenarios, edge cases, and stress testing.
//!
//! Bead: bf-bdz6iz
//! Acceptance Criteria:
//! - Comprehensive scope tracking tests added
//! - Edge cases covered (flow style, complex sequences, scope reuse)
//! - Stress tests with many scopes and deep nesting
//! - All tests pass

use armor::parsers::yaml::{
    Scope, ScopeStack, DuplicateKeyError, KeyContext,
    extract_key_context, get_leading_whitespace_length
};

// =============================================================================
// Scope Creation and Initialization Tests
// =============================================================================

#[test]
fn test_scope_creation_with_all_fields() {
    let scope = Scope::new(4, 10, Some("parent".to_string()));
    assert_eq!(scope.indent_level, 4);
    assert_eq!(scope.start_line, 10);
    assert_eq!(scope.parent_key, Some("parent".to_string()));
    assert_eq!(scope.key_count(), 0);
    assert!(!scope.is_flow_style);
    assert!(!scope.in_sequence_context);
    assert!(scope.sequence_item_id.is_none());
}

#[test]
fn test_scope_creation_without_parent() {
    let scope = Scope::new(0, 1, None);
    assert_eq!(scope.indent_level, 0);
    assert_eq!(scope.start_line, 1);
    assert!(scope.parent_key.is_none());
}

#[test]
fn test_scope_stack_with_different_base_indents() {
    let stack_2 = ScopeStack::new(2);
    assert_eq!(stack_2.base_indent(), 2);

    let stack_4 = ScopeStack::new(4);
    assert_eq!(stack_4.base_indent(), 4);
}

#[test]
fn test_scope_stack_reset_clears_all_scopes() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("first".to_string()));
    stack.enter_scope(4, 2, Some("second".to_string()));
    stack.add_key("key1", 3).unwrap();

    assert_eq!(stack.depth(), 3);
    assert!(stack.contains_key("key1"));

    stack.reset();

    assert_eq!(stack.depth(), 1);
    assert!(!stack.contains_key("key1"));
}

#[test]
fn test_scope_stack_set_base_indent() {
    let mut stack = ScopeStack::new(2);
    assert_eq!(stack.base_indent(), 2);

    stack.set_base_indent(4);
    assert_eq!(stack.base_indent(), 4);
}

// =============================================================================
// Key Addition and Duplicate Detection Tests
// =============================================================================

#[test]
fn test_add_key_returns_ok_on_first_addition() {
    let mut scope = Scope::new(0, 1, None);
    assert_eq!(scope.add_key("test"), false);
    assert!(scope.contains_key("test"));
}

#[test]
fn test_add_key_returns_true_on_duplicate() {
    let mut scope = Scope::new(0, 1, None);
    scope.add_key("test");
    assert_eq!(scope.add_key("test"), true);
}

#[test]
fn test_add_multiple_keys() {
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
fn test_clear_keys_removes_all_keys() {
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
fn test_stack_add_key_in_different_scopes() {
    let mut stack = ScopeStack::new(2);

    // Add to root scope
    stack.add_key("host", 1).unwrap();
    assert!(stack.contains_key("host"));

    // Enter nested scope
    stack.enter_scope(2, 2, Some("services".to_string()));

    // Same key should be OK in different scope
    stack.add_key("host", 3).unwrap();
    assert!(stack.contains_key("host"));

    // Verify root scope still has its key
    assert!(stack.contains_key_in_any_scope("host"));
}

#[test]
fn test_stack_add_key_duplicate_in_current_scope() {
    let mut stack = ScopeStack::new(2);
    stack.add_key("key", 1).unwrap();

    let result = stack.add_key("key", 2);
    assert!(result.is_err());

    let error = result.unwrap_err();
    assert_eq!(error.key, "key");
    assert_eq!(error.first_line, 0); // Root scope starts at line 0
    assert_eq!(error.duplicate_line, 2);
}

#[test]
fn test_contains_key_in_any_scope() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("root_key", 1).unwrap();

    stack.enter_scope(2, 2, Some("level1".to_string()));
    stack.add_key("level1_key", 3).unwrap();

    stack.enter_scope(4, 4, Some("level2".to_string()));
    stack.add_key("level2_key", 5).unwrap();

    // Current scope
    assert!(stack.contains_key("level2_key"));
    assert!(!stack.contains_key("level1_key"));
    assert!(!stack.contains_key("root_key"));

    // Any scope
    assert!(stack.contains_key_in_any_scope("level2_key"));
    assert!(stack.contains_key_in_any_scope("level1_key"));
    assert!(stack.contains_key_in_any_scope("root_key"));
}

// =============================================================================
// Scope Transition Tests
// =============================================================================

#[test]
fn test_enter_scope_creates_new_scope() {
    let mut stack = ScopeStack::new(2);
    let initial_depth = stack.depth();

    stack.enter_scope(2, 1, Some("parent".to_string()));

    assert_eq!(stack.depth(), initial_depth + 1);
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.get_scope_path(), "parent");
}

#[test]
fn test_enter_nested_scopes() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    assert_eq!(stack.get_scope_path(), "level1");

    stack.enter_scope(4, 2, Some("level2".to_string()));
    assert_eq!(stack.get_scope_path(), "level1.level2");

    stack.enter_scope(6, 3, Some("level3".to_string()));
    assert_eq!(stack.get_scope_path(), "level1.level2.level3");
}

#[test]
fn test_exit_to_scope_removes_deeper_scopes() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(6, 3, Some("level3".to_string()));

    assert_eq!(stack.depth(), 4);

    stack.exit_to_scope(2);

    assert_eq!(stack.depth(), 2);
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.get_scope_path(), "level1");
}

#[test]
fn test_exit_to_nonexistent_scope_creates_fallback() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));

    // Exit to a scope that doesn't exist (level 3 when we only have 0 and 2)
    stack.exit_to_scope(3);

    // Should create a fallback scope at level 3
    assert_eq!(stack.current_indent(), 3);
}

#[test]
fn test_scope_reuse_at_same_level() {
    let mut stack = ScopeStack::new(2);

    // Enter first scope at level 2
    stack.enter_scope(2, 1, Some("first".to_string()));
    stack.add_key("key1", 2).unwrap();
    stack.add_key("key2", 3).unwrap();

    // Exit back to root
    stack.exit_to_scope(0);

    // Enter second scope at same level 2
    stack.enter_scope(2, 5, Some("second".to_string()));

    // Should have cleared keys from first scope
    assert_eq!(stack.current_scope_ref().key_count(), 0);
    assert!(!stack.contains_key("key1"));

    // Same key should be OK (different scope)
    stack.add_key("key1", 6).unwrap();
}

#[test]
fn test_get_scope_at_level() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(6, 3, Some("level3".to_string()));

    assert!(stack.get_scope_at_level(0).is_some());
    assert!(stack.get_scope_at_level(2).is_some());
    assert!(stack.get_scope_at_level(4).is_some());
    assert!(stack.get_scope_at_level(6).is_some());

    // Level that doesn't exist
    assert!(stack.get_scope_at_level(8).is_none());
}

// =============================================================================
// Sequence Scope Tests
// =============================================================================

#[test]
fn test_enter_sequence_scope_creates_sequence_context() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);

    assert!(stack.in_sequence_context());
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.current_scope_ref().sequence_item_id, Some(1));
}

#[test]
fn test_sequence_item_ids_are_unique() {
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
fn test_sequence_scope_clears_previous_keys() {
    let mut stack = ScopeStack::new(2);

    // First sequence item
    stack.enter_sequence_scope(2, 1);
    stack.add_key("name", 2).unwrap();
    stack.add_key("value", 3).unwrap();
    assert_eq!(stack.current_scope_ref().key_count(), 2);

    // Second sequence item (should clear keys)
    stack.enter_sequence_scope(2, 5);
    assert_eq!(stack.current_scope_ref().key_count(), 0);

    // Keys can be reused in new sequence item
    stack.add_key("name", 6).unwrap();
    stack.add_key("value", 7).unwrap();
}

#[test]
fn test_duplicate_within_sequence_scope_detected() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    stack.add_key("name", 2).unwrap();

    let result = stack.add_key("name", 3);
    assert!(result.is_err());
}

#[test]
fn test_sequence_scopes_at_different_levels() {
    let mut stack = ScopeStack::new(2);

    // Regular scope
    stack.enter_scope(2, 1, Some("items".to_string()));

    // Sequence scope within regular scope
    stack.enter_sequence_scope(4, 2);
    assert!(stack.in_sequence_context());
    assert_eq!(stack.current_scope_ref().sequence_item_id, Some(1));

    // Exit back to regular scope
    stack.exit_to_scope(2);
    assert!(!stack.in_sequence_context());

    // Enter another sequence scope
    stack.enter_sequence_scope(4, 4);
    assert!(stack.in_sequence_context());
    assert_eq!(stack.current_scope_ref().sequence_item_id, Some(2));
}

#[test]
fn test_mixed_regular_and_sequence_scopes() {
    let mut stack = ScopeStack::new(2);

    // Regular scope with keys
    stack.enter_scope(2, 1, Some("config".to_string()));
    stack.add_key("timeout", 2).unwrap();
    stack.add_key("retries", 3).unwrap();

    // Sequence scope (should not affect regular scope)
    stack.enter_sequence_scope(4, 4);
    stack.add_key("name", 5).unwrap();

    // Exit to regular scope
    stack.exit_to_scope(2);

    // Regular scope keys should still be present
    assert!(stack.contains_key("timeout"));
    assert!(stack.contains_key("retries"));
    assert!(!stack.contains_key("name")); // name was in sequence scope
}

// =============================================================================
// Scope Path Generation Tests
// =============================================================================

#[test]
fn test_scope_path_at_root() {
    let stack = ScopeStack::new(2);
    assert_eq!(stack.get_scope_path(), "");
}

#[test]
fn test_scope_path_single_level() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("services".to_string()));
    assert_eq!(stack.get_scope_path(), "services");
}

#[test]
fn test_scope_path_multiple_levels() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(6, 3, Some("level3".to_string()));
    assert_eq!(stack.get_scope_path(), "level1.level2.level3");
}

#[test]
fn test_scope_path_after_exit() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));

    assert_eq!(stack.get_scope_path(), "level1.level2");

    stack.exit_to_scope(2);
    assert_eq!(stack.get_scope_path(), "level1");
}

#[test]
fn test_scope_path_sequence_context() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("items".to_string()));
    stack.enter_sequence_scope(4, 2);

    // Sequence scopes don't have parent keys, so path stays at "items"
    assert_eq!(stack.get_scope_path(), "items");
}

// =============================================================================
// KeyContext Extraction Tests
// =============================================================================

#[test]
fn test_extract_inline_scalar_key_context() {
    let ctx = extract_key_context("host: localhost").unwrap();
    match ctx {
        KeyContext::InlineScalar { key, value } => {
            assert_eq!(key, "host");
            assert_eq!(value, "localhost");
        }
        _ => panic!("Expected InlineScalar"),
    }
}

#[test]
fn test_extract_parent_mapping_key_context() {
    let ctx = extract_key_context("services:").unwrap();
    match ctx {
        KeyContext::ParentMapping { key } => {
            assert_eq!(key, "services");
        }
        _ => panic!("Expected ParentMapping"),
    }
}

#[test]
fn test_extract_key_context_with_spaces_after_colon() {
    let ctx = extract_key_context("key:   value  ").unwrap();
    match ctx {
        KeyContext::InlineScalar { key, value } => {
            assert_eq!(key, "key");
            assert_eq!(value, "value");
        }
        _ => panic!("Expected InlineScalar"),
    }
}

#[test]
fn test_extract_key_context_with_special_chars() {
    let ctx = extract_key_context("my-key: value").unwrap();
    assert_eq!(ctx.key_name(), "my-key");
}

#[test]
fn test_extract_key_context_with_underscores() {
    let ctx = extract_key_context("my_key: value").unwrap();
    assert_eq!(ctx.key_name(), "my_key");
}

#[test]
fn test_extract_key_context_no_colon() {
    assert!(extract_key_context("no colon here").is_none());
}

#[test]
fn test_extract_key_context_empty_key() {
    assert!(extract_key_context(": value").is_none());
}

#[test]
fn test_extract_key_context_flow_style_braces() {
    assert!(extract_key_context("{key: value}").is_none());
}

#[test]
fn test_extract_key_context_flow_style_brackets() {
    assert!(extract_key_context("[item1, item2]").is_none());
}

#[test]
fn test_extract_key_context_with_tabs_in_value() {
    let ctx = extract_key_context("key:\tvalue").unwrap();
    match ctx {
        KeyContext::InlineScalar { key, value } => {
            assert_eq!(key, "key");
            assert_eq!(value, "value");
        }
        _ => panic!("Expected InlineScalar"),
    }
}

#[test]
fn test_extract_key_context_quoted_value() {
    let ctx = extract_key_context("key: \"quoted value\"").unwrap();
    match ctx {
        KeyContext::InlineScalar { key, value } => {
            assert_eq!(key, "key");
            assert_eq!(value, "\"quoted value\"");
        }
        _ => panic!("Expected InlineScalar"),
    }
}

#[test]
fn test_key_context_is_parent_key() {
    let inline = extract_key_context("key: value").unwrap();
    assert!(!inline.is_parent_key());
    assert!(inline.is_inline_scalar());

    let parent = extract_key_context("parent:").unwrap();
    assert!(parent.is_parent_key());
    assert!(!parent.is_inline_scalar());
}

// =============================================================================
// Leading Whitespace Tests
// =============================================================================

#[test]
fn test_leading_whitespace_length_empty() {
    assert_eq!(get_leading_whitespace_length(""), 0);
}

#[test]
fn test_leading_whitespace_length_no_indent() {
    assert_eq!(get_leading_whitespace_length("key: value"), 0);
}

#[test]
fn test_leading_whitespace_length_spaces() {
    assert_eq!(get_leading_whitespace_length("  key: value"), 2);
    assert_eq!(get_leading_whitespace_length("    key: value"), 4);
    assert_eq!(get_leading_whitespace_length("      key: value"), 6);
}

#[test]
fn test_leading_whitespace_length_tabs() {
    assert_eq!(get_leading_whitespace_length("\tkey: value"), 1);
    assert_eq!(get_leading_whitespace_length("\t\tkey: value"), 2);
}

#[test]
fn test_leading_whitespace_length_mixed() {
    // Tabs count as 1 each
    assert_eq!(get_leading_whitespace_length(" \t key: value"), 3);
}

// =============================================================================
// DuplicateKeyError Tests
// =============================================================================

#[test]
fn test_duplicate_key_error_creation() {
    let error = DuplicateKeyError::new(
        "test_key".to_string(),
        "scope.path".to_string(),
        5,
        10
    );

    assert_eq!(error.key, "test_key");
    assert_eq!(error.scope_path, "scope.path");
    assert_eq!(error.first_line, 5);
    assert_eq!(error.duplicate_line, 10);
}

#[test]
fn test_duplicate_key_error_message() {
    let error = DuplicateKeyError::new(
        "host".to_string(),
        "services.web".to_string(),
        3,
        7
    );

    let message = error.message();
    assert!(message.contains("Line 7"));
    assert!(message.contains("duplicate key"));
    assert!(message.contains("host"));
    assert!(message.contains("services.web"));
    assert!(message.contains("Line 3"));
}

#[test]
fn test_duplicate_key_error_display() {
    let error = DuplicateKeyError::new(
        "port".to_string(),
        "config".to_string(),
        1,
        5
    );

    let display = format!("{}", error);
    assert!(display.contains("Line 5"));
    assert!(display.contains("port"));
    assert!(display.contains("config"));
    assert!(display.contains("Line 1"));
}

// =============================================================================
// Display Formatting Tests
// =============================================================================

#[test]
fn test_scope_display() {
    let mut scope = Scope::new(2, 5, Some("parent".to_string()));
    scope.add_key("key1");
    scope.add_key("key2");

    let display = format!("{}", scope);
    assert!(display.contains("Scope"));
    assert!(display.contains("indent=2"));
    assert!(display.contains("parent=parent"));
    assert!(display.contains("keys=2"));
}

#[test]
fn test_scope_display_without_parent() {
    let scope = Scope::new(0, 1, None);
    let display = format!("{}", scope);
    assert!(display.contains("Scope"));
    assert!(display.contains("indent=0"));
    assert!(!display.contains("parent="));
}

#[test]
fn test_scope_stack_display() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));

    let display = format!("{}", stack);
    assert!(display.contains("ScopeStack"));
    assert!(display.contains("depth=3"));
    assert!(display.contains("base_indent=2"));
    assert!(display.contains("current_path=level1.level2"));
}

// =============================================================================
// Stress Tests
// =============================================================================

#[test]
fn test_many_keys_in_single_scope() {
    let mut scope = Scope::new(0, 1, None);

    // Add 1000 unique keys
    for i in 0..1000 {
        let key = format!("key_{}", i);
        assert_eq!(scope.add_key(&key), false);
    }

    assert_eq!(scope.key_count(), 1000);

    // Verify all keys exist
    for i in 0..1000 {
        let key = format!("key_{}", i);
        assert!(scope.contains_key(&key));
    }
}

#[test]
fn test_deep_nesting_stack() {
    let mut stack = ScopeStack::new(2);

    // Create 20 levels of nesting
    let mut expected_path = String::new();
    for i in 1..=20 {
        let key = format!("level{}", i);
        if !expected_path.is_empty() {
            expected_path.push('.');
        }
        expected_path.push_str(&key);

        stack.enter_scope(i * 2, i as usize, Some(key.clone()));
    }

    assert_eq!(stack.depth(), 21); // 20 + root
    assert_eq!(stack.get_scope_path(), expected_path);
}

#[test]
fn test_many_sibling_scopes() {
    let mut stack = ScopeStack::new(2);

    // Create 100 sibling scopes
    for i in 1..=100 {
        let key = format!("sibling{}", i);

        // Exit to root, then enter new sibling
        stack.exit_to_scope(0);
        stack.enter_scope(2, i as usize, Some(key.clone()));

        // Add same key to each sibling (should be OK)
        stack.add_key("shared_key", i as usize).unwrap();
    }

    // Should only have root + current scope
    assert_eq!(stack.depth(), 2);
}

#[test]
fn test_many_sequence_items() {
    let mut stack = ScopeStack::new(2);

    // Create 100 sequence items
    for i in 1..=100 {
        stack.enter_sequence_scope(2, i as usize);

        // Each item should have unique ID
        assert_eq!(stack.current_scope_ref().sequence_item_id, Some(i));

        // Same key should be OK in different sequence items
        stack.add_key("name", i as usize).unwrap();
    }

    assert_eq!(stack.current_scope_ref().sequence_item_id, Some(100));
}

#[test]
fn test_complex_realistic_structure() {
    let mut stack = ScopeStack::new(2);

    // Simulate complex YAML structure
    stack.enter_scope(2, 1, Some("services".to_string()));

    // First sibling scope (web)
    stack.enter_scope(4, 2, Some("web".to_string()));
    stack.add_key("host", 3).unwrap();
    stack.add_key("port", 4).unwrap();
    // Verify keys are in current scope
    assert!(stack.contains_key("host"));
    assert!(stack.contains_key("port"));
    stack.exit_to_scope(2);

    // Second sibling scope (database) - same keys should be OK
    stack.enter_scope(4, 5, Some("database".to_string()));
    stack.add_key("host", 6).unwrap(); // Same key, different scope - OK
    stack.add_key("port", 7).unwrap();  // Same key, different scope - OK
    // Verify keys are in current scope
    assert!(stack.contains_key("host"));
    assert!(stack.contains_key("port"));
    stack.exit_to_scope(2);

    // Third scope with nested sequences
    stack.enter_scope(4, 8, Some("cache".to_string()));
    stack.enter_sequence_scope(6, 9);
    stack.add_key("server", 10).unwrap();
    stack.enter_sequence_scope(6, 11);
    stack.add_key("server", 12).unwrap(); // Same key, different seq item - OK

    // Verify we're in the cache scope with sequences
    assert_eq!(stack.get_scope_path(), "services.cache");
    assert!(stack.in_sequence_context());
}

// =============================================================================
// Edge Cases
// =============================================================================

#[test]
fn test_empty_key_string() {
    let mut scope = Scope::new(0, 1, None);
    // Empty string is a valid key
    assert_eq!(scope.add_key(""), false);
    assert!(scope.contains_key(""));
}

#[test]
fn test_very_long_key_name() {
    let mut scope = Scope::new(0, 1, None);
    let long_key = "a".repeat(10000);
    assert_eq!(scope.add_key(&long_key), false);
    assert!(scope.contains_key(&long_key));
}

#[test]
fn test_key_with_unicode() {
    let mut scope = Scope::new(0, 1, None);

    scope.add_key("ключ"); // Russian
    scope.add_key("键");    // Chinese
    scope.add_key("clave"); // Spanish with accent

    assert!(scope.contains_key("ключ"));
    assert!(scope.contains_key("键"));
    assert!(scope.contains_key("clave"));
}

#[test]
fn test_key_with_special_yaml_chars() {
    let mut scope = Scope::new(0, 1, None);

    // These are valid YAML key characters
    scope.add_key("key-with-dashes");
    scope.add_key("key_with_underscores");
    scope.add_key("key.with.dots");

    assert!(scope.contains_key("key-with-dashes"));
    assert!(scope.contains_key("key_with_underscores"));
    assert!(scope.contains_key("key.with.dots"));
}

#[test]
fn test_scope_at_zero_indent() {
    let stack = ScopeStack::new(2);
    let scope = stack.get_scope_at_level(0);
    assert!(scope.is_some());
    assert_eq!(scope.unwrap().indent_level, 0);
}

#[test]
fn test_current_indent_on_empty_stack() {
    let stack = ScopeStack::new(2);
    assert_eq!(stack.current_indent(), 0);
}

#[test]
fn test_depth_of_new_stack() {
    let stack = ScopeStack::new(2);
    assert_eq!(stack.depth(), 1); // Root scope only
}

#[test]
fn test_exit_to_root_leaves_one_scope() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));

    stack.exit_to_scope(0);

    assert_eq!(stack.depth(), 1);
    assert_eq!(stack.current_indent(), 0);
}
