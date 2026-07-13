//! Comprehensive Scope Tracking Test Cases
//!
//! These tests provide comprehensive coverage of the scope tracking system,
//! including ScopeStack, sequence scope handling, key context extraction,
//! and integration with the YAML parser.
//!
//! Bead: bf-bdz6iz
//!
//! Test Coverage:
//! - ScopeStack basic operations
//! - Sequence scope handling
//! - Key context classification
//! - Scope path generation
//! - Edge cases and boundary conditions
//! - Integration with SyntaxDetector

use armor::parsers::yaml::{
    Scope, ScopeStack, DuplicateKeyError, KeyContext,
    extract_key_context, get_leading_whitespace_length
};

// =============================================================================
// Scope Creation and Basic Operations
// =============================================================================

#[test]
fn test_scope_default_values() {
    let scope = Scope::new(0, 1, None);
    assert_eq!(scope.indent_level, 0);
    assert_eq!(scope.start_line, 1);
    assert_eq!(scope.parent_key, None);
    assert!(!scope.is_flow_style);
    assert!(!scope.in_sequence_context);
    assert_eq!(scope.sequence_item_id, None);
    assert_eq!(scope.key_count(), 0);
}

#[test]
fn test_scope_with_parent_key() {
    let scope = Scope::new(2, 5, Some("services".to_string()));
    assert_eq!(scope.indent_level, 2);
    assert_eq!(scope.start_line, 5);
    assert_eq!(scope.parent_key, Some("services".to_string()));
}

#[test]
fn test_scope_flow_style_flag() {
    let mut scope = Scope::new(2, 1, Some("key".to_string()));
    assert!(!scope.is_flow_style);

    scope.is_flow_style = true;
    assert!(scope.is_flow_style);
}

#[test]
fn test_scope_sequence_context() {
    let mut scope = Scope::new(2, 1, None);
    assert!(!scope.in_sequence_context);
    assert_eq!(scope.sequence_item_id, None);

    scope.in_sequence_context = true;
    scope.sequence_item_id = Some(1);
    assert!(scope.in_sequence_context);
    assert_eq!(scope.sequence_item_id, Some(1));
}

#[test]
fn test_scope_add_single_key() {
    let mut scope = Scope::new(0, 1, None);
    assert!(!scope.add_key("first_key"));
    assert_eq!(scope.key_count(), 1);
    assert!(scope.contains_key("first_key"));
}

#[test]
fn test_scope_add_multiple_keys() {
    let mut scope = Scope::new(0, 1, None);
    assert!(!scope.add_key("key1"));
    assert!(!scope.add_key("key2"));
    assert!(!scope.add_key("key3"));
    assert_eq!(scope.key_count(), 3);
    assert!(scope.contains_key("key1"));
    assert!(scope.contains_key("key2"));
    assert!(scope.contains_key("key3"));
}

#[test]
fn test_scope_duplicate_detection() {
    let mut scope = Scope::new(0, 1, None);
    assert!(!scope.add_key("unique"));
    assert_eq!(scope.key_count(), 1);

    // Adding the same key should return true (duplicate)
    assert!(scope.add_key("unique"));
    assert_eq!(scope.key_count(), 1); // Count unchanged
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

// =============================================================================
// ScopeStack Creation and Configuration
// =============================================================================

#[test]
fn test_scope_stack_with_different_base_indents() {
    let stack_2 = ScopeStack::new(2);
    assert_eq!(stack_2.base_indent(), 2);

    let stack_4 = ScopeStack::new(4);
    assert_eq!(stack_4.base_indent(), 4);

    let stack_1 = ScopeStack::new(1);
    assert_eq!(stack_1.base_indent(), 1);
}

#[test]
fn test_scope_stack_change_base_indent() {
    let mut stack = ScopeStack::new(2);
    assert_eq!(stack.base_indent(), 2);

    stack.set_base_indent(4);
    assert_eq!(stack.base_indent(), 4);
}

#[test]
fn test_scope_stack_initial_state() {
    let stack = ScopeStack::new(2);
    assert_eq!(stack.depth(), 1); // Root scope
    assert_eq!(stack.current_indent(), 0);
    assert_eq!(stack.get_scope_path(), "");
}

#[test]
fn test_scope_stack_reset() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("parent".to_string()));
    stack.add_key("key", 2).unwrap();
    assert_eq!(stack.depth(), 2);

    stack.reset();
    assert_eq!(stack.depth(), 1);
    assert!(!stack.contains_key("key"));
}

// =============================================================================
// ScopeStack Enter/Exit Operations
// =============================================================================

#[test]
fn test_enter_scope_creates_new_level() {
    let mut stack = ScopeStack::new(2);
    assert_eq!(stack.depth(), 1);

    stack.enter_scope(2, 1, Some("level1".to_string()));
    assert_eq!(stack.depth(), 2);
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.get_scope_path(), "level1");
}

#[test]
fn test_enter_multiple_scopes() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    stack.enter_scope(6, 3, Some("level3".to_string()));

    assert_eq!(stack.depth(), 4); // Root + 3 levels
    assert_eq!(stack.get_scope_path(), "level1.level2.level3");
}

#[test]
fn test_exit_to_parent_scope() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    assert_eq!(stack.depth(), 3);

    stack.exit_to_scope(2);
    assert_eq!(stack.depth(), 2);
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.get_scope_path(), "level1");
}

#[test]
fn test_exit_multiple_levels() {
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
fn test_re_enter_sibling_scope() {
    let mut stack = ScopeStack::new(2);

    // Enter first sibling
    stack.enter_scope(2, 1, Some("sibling1".to_string()));
    stack.add_key("key1", 2).unwrap();
    assert_eq!(stack.get_scope_path(), "sibling1");

    // Exit to parent
    stack.exit_to_scope(0);

    // Enter second sibling at same level
    stack.enter_scope(2, 5, Some("sibling2".to_string()));
    assert_eq!(stack.get_scope_path(), "sibling2");
    assert!(!stack.contains_key("key1")); // Keys cleared for new scope
}

#[test]
fn test_enter_scope_with_no_parent_key() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, None);
    assert_eq!(stack.depth(), 2);
    assert_eq!(stack.get_scope_path(), ""); // No parent key in path
}

// =============================================================================
// ScopeStack Key Operations
// =============================================================================

#[test]
fn test_add_key_to_root_scope() {
    let mut stack = ScopeStack::new(2);
    assert!(stack.add_key("root_key", 1).is_ok());
    assert!(stack.contains_key("root_key"));
    assert_eq!(stack.current_scope_ref().key_count(), 1);
}

#[test]
fn test_add_key_to_nested_scope() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("parent".to_string()));
    assert!(stack.add_key("nested_key", 2).is_ok());
    assert!(stack.contains_key("nested_key"));
}

#[test]
fn test_duplicate_key_in_same_scope_fails() {
    let mut stack = ScopeStack::new(2);
    assert!(stack.add_key("unique", 1).is_ok());

    let result = stack.add_key("unique", 2);
    assert!(result.is_err());

    if let Err(err) = result {
        assert_eq!(err.key, "unique");
        // Root scope has start_line of 0 (initialized in ScopeStack::new)
        assert_eq!(err.first_line, 0);
        assert_eq!(err.duplicate_line, 2);
    }
}

#[test]
fn test_same_key_in_different_scopes_succeeds() {
    let mut stack = ScopeStack::new(2);

    // Add to root
    stack.add_key("host", 1).unwrap();
    assert!(stack.contains_key("host"));

    // Enter nested scope
    stack.enter_scope(2, 2, Some("services".to_string()));

    // Same key should be OK in different scope
    stack.add_key("host", 3).unwrap();
    assert!(stack.contains_key("host"));
}

#[test]
fn test_keys_isolated_between_siblings() {
    let mut stack = ScopeStack::new(2);

    // First sibling
    stack.enter_scope(2, 1, Some("sibling1".to_string()));
    stack.add_key("key1", 2).unwrap();
    stack.add_key("key2", 3).unwrap();
    stack.exit_to_scope(0);

    // Second sibling
    stack.enter_scope(2, 5, Some("sibling2".to_string()));
    // Keys from first sibling should not be present
    assert!(!stack.contains_key("key1"));
    assert!(!stack.contains_key("key2"));

    // Can add same keys to second sibling
    stack.add_key("key1", 6).unwrap();
    stack.add_key("key2", 7).unwrap();
    assert!(stack.contains_key("key1"));
    assert!(stack.contains_key("key2"));
}

#[test]
fn test_contains_key_in_any_scope() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("root_key", 1).unwrap();
    stack.enter_scope(2, 2, Some("level1".to_string()));
    stack.add_key("nested_key", 3).unwrap();

    assert!(stack.contains_key_in_any_scope("root_key"));
    assert!(stack.contains_key_in_any_scope("nested_key"));

    stack.enter_scope(4, 4, Some("level2".to_string()));
    // From deep scope, can still find keys in parent scopes
    assert!(stack.contains_key_in_any_scope("root_key"));
    assert!(stack.contains_key_in_any_scope("nested_key"));
}

// =============================================================================
// Sequence Scope Operations
// =============================================================================

#[test]
fn test_enter_sequence_scope() {
    let mut stack = ScopeStack::new(2);
    stack.enter_sequence_scope(2, 1);

    assert!(stack.in_sequence_context());
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.current_scope_ref().in_sequence_context, true);
}

#[test]
fn test_sequence_scope_unique_ids() {
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

    stack.enter_sequence_scope(2, 1);
    stack.add_key("name", 2).unwrap();
    stack.add_key("value", 3).unwrap();
    assert_eq!(stack.current_scope_ref().key_count(), 2);

    // New sequence item should clear keys
    stack.enter_sequence_scope(2, 5);
    assert_eq!(stack.current_scope_ref().key_count(), 0);
    assert!(!stack.contains_key("name"));
    assert!(!stack.contains_key("value"));
}

#[test]
fn test_same_key_in_different_sequence_items() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    stack.add_key("name", 2).unwrap();
    stack.add_key("value", 3).unwrap();

    // New sequence item - same keys should be OK
    stack.enter_sequence_scope(2, 5);
    stack.add_key("name", 6).unwrap();
    stack.add_key("value", 7).unwrap();

    assert_eq!(stack.current_scope_ref().key_count(), 2);
}

#[test]
fn test_duplicate_within_sequence_item_fails() {
    let mut stack = ScopeStack::new(2);

    stack.enter_sequence_scope(2, 1);
    stack.add_key("name", 2).unwrap();

    let result = stack.add_key("name", 3);
    assert!(result.is_err());
}

#[test]
fn test_mixed_regular_and_sequence_scopes() {
    let mut stack = ScopeStack::new(2);

    // Regular scope
    stack.enter_scope(2, 1, Some("items".to_string()));
    stack.add_key("item1", 2).unwrap();
    assert!(!stack.in_sequence_context());

    // Sequence scope within regular
    stack.enter_sequence_scope(4, 3);
    stack.add_key("name", 4).unwrap();
    assert!(stack.in_sequence_context());

    // Exit back to regular scope
    stack.exit_to_scope(2);
    assert!(!stack.in_sequence_context());
}

// =============================================================================
// Scope Path Generation
// =============================================================================

#[test]
fn test_scope_path_single_level() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("config".to_string()));
    assert_eq!(stack.get_scope_path(), "config");
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
fn test_scope_path_updates_on_sibling_entry() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("parent1".to_string()));
    stack.enter_scope(4, 2, Some("child1".to_string()));
    assert_eq!(stack.get_scope_path(), "parent1.child1");

    stack.exit_to_scope(2);
    stack.enter_scope(4, 5, Some("child2".to_string()));
    assert_eq!(stack.get_scope_path(), "parent1.child2");
}

#[test]
fn test_scope_path_at_root() {
    let stack = ScopeStack::new(2);
    assert_eq!(stack.get_scope_path(), "");
}

#[test]
fn test_scope_path_after_reset() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("level1".to_string()));
    stack.enter_scope(4, 2, Some("level2".to_string()));
    assert_ne!(stack.get_scope_path(), "");

    stack.reset();
    assert_eq!(stack.get_scope_path(), "");
}

// =============================================================================
// DuplicateKeyError Structure
// =============================================================================

#[test]
fn test_duplicate_key_error_creation() {
    let error = DuplicateKeyError::new(
        "host".to_string(),
        "services.web".to_string(),
        5,
        10,
    );

    assert_eq!(error.key, "host");
    assert_eq!(error.scope_path, "services.web");
    assert_eq!(error.first_line, 5);
    assert_eq!(error.duplicate_line, 10);
}

#[test]
fn test_duplicate_key_error_message() {
    let error = DuplicateKeyError::new(
        "port".to_string(),
        "config.database".to_string(),
        15,
        25,
    );

    let message = error.message();
    assert!(message.contains("duplicate key"));
    assert!(message.contains("port"));
    assert!(message.contains("config.database"));
    assert!(message.contains("Line 25"));
    assert!(message.contains("Line 15"));
}

#[test]
fn test_duplicate_key_error_display() {
    let error = DuplicateKeyError::new(
        "timeout".to_string(),
        "settings".to_string(),
        3,
        8,
    );

    let display = format!("{}", error);
    assert!(display.contains("duplicate key"));
    assert!(display.contains("timeout"));
    assert!(display.contains("settings"));
}

// =============================================================================
// Key Context Extraction
// =============================================================================

#[test]
fn test_extract_inline_scalar_context() {
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
fn test_extract_parent_mapping_context() {
    let ctx = extract_key_context("services:").unwrap();
    match ctx {
        KeyContext::ParentMapping { key } => {
            assert_eq!(key, "services");
        }
        _ => panic!("Expected ParentMapping"),
    }
}

#[test]
fn test_extract_key_context_with_spaces() {
    let ctx = extract_key_context("  timeout: 30  ").unwrap();
    assert!(matches!(ctx, KeyContext::InlineScalar { .. }));
    assert_eq!(ctx.key_name(), "timeout");
}

#[test]
fn test_extract_key_context_with_extra_spaces_after_colon() {
    let ctx = extract_key_context("key:   value").unwrap();
    match ctx {
        KeyContext::InlineScalar { key, value } => {
            assert_eq!(key, "key");
            assert_eq!(value, "value");
        }
        _ => panic!("Expected InlineScalar"),
    }
}

#[test]
fn test_extract_key_context_empty_after_colon() {
    let ctx = extract_key_context("parent:").unwrap();
    assert!(matches!(ctx, KeyContext::ParentMapping { .. }));
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
fn test_key_context_is_parent_key() {
    let inline = extract_key_context("key: value").unwrap();
    assert!(!inline.is_parent_key());
    assert!(inline.is_inline_scalar());

    let parent = extract_key_context("parent:").unwrap();
    assert!(parent.is_parent_key());
    assert!(!parent.is_inline_scalar());
}

#[test]
fn test_key_context_key_name() {
    let inline = extract_key_context("host: localhost").unwrap();
    assert_eq!(inline.key_name(), "host");

    let parent = extract_key_context("services:").unwrap();
    assert_eq!(parent.key_name(), "services");
}

// =============================================================================
// Leading Whitespace Detection
// =============================================================================

#[test]
fn test_leading_whitespace_no_indent() {
    assert_eq!(get_leading_whitespace_length("key: value"), 0);
}

#[test]
fn test_leading_whitespace_single_space() {
    assert_eq!(get_leading_whitespace_length(" key: value"), 1);
}

#[test]
fn test_leading_whitespace_two_spaces() {
    assert_eq!(get_leading_whitespace_length("  key: value"), 2);
}

#[test]
fn test_leading_whitespace_four_spaces() {
    assert_eq!(get_leading_whitespace_length("    key: value"), 4);
}

#[test]
fn test_leading_whitespace_many_spaces() {
    assert_eq!(get_leading_whitespace_length("        key: value"), 8);
}

#[test]
fn test_leading_whitespace_with_tabs() {
    // Tab counts as 1 whitespace character
    assert_eq!(get_leading_whitespace_length("\tkey: value"), 1);
}

#[test]
fn test_leading_whitespace_mixed_spaces_and_tabs() {
    // Counts all leading whitespace characters
    assert_eq!(get_leading_whitespace_length("  \t key: value"), 4);
}

#[test]
fn test_leading_whitespace_empty_line() {
    assert_eq!(get_leading_whitespace_length(""), 0);
}

#[test]
fn test_leading_whitespace_only_whitespace() {
    assert_eq!(get_leading_whitespace_length("    "), 4);
}

// =============================================================================
// Display Formatting
// =============================================================================

#[test]
fn test_scope_display_format() {
    let scope = Scope::new(2, 5, Some("parent".to_string()));
    let display = format!("{}", scope);

    assert!(display.contains("Scope"));
    assert!(display.contains("indent=2"));
    assert!(display.contains("parent=parent"));
}

#[test]
fn test_scope_display_without_parent() {
    let scope = Scope::new(0, 1, None);
    let display = format!("{}", scope);

    assert!(display.contains("Scope"));
    assert!(display.contains("indent=0"));
    // Should not contain "parent="
}

#[test]
fn test_scope_stack_display_format() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("level1".to_string()));

    let display = format!("{}", stack);

    assert!(display.contains("ScopeStack"));
    assert!(display.contains("depth="));
    assert!(display.contains("base_indent="));
    assert!(display.contains("current_path="));
}

// =============================================================================
// Edge Cases and Boundary Conditions
// =============================================================================

#[test]
fn test_very_deep_nesting() {
    let mut stack = ScopeStack::new(2);

    // Simulate 20 levels of nesting
    for i in 0..20 {
        let indent = i * 2;
        stack.enter_scope(indent, i + 1, Some(format!("level{}", i)));
    }

    assert_eq!(stack.depth(), 21); // Root + 20 levels
}

#[test]
fn test_many_keys_in_single_scope() {
    let mut stack = ScopeStack::new(2);

    // Add 100 keys
    for i in 0..100 {
        let key = format!("key{}", i);
        stack.add_key(&key, i + 1).unwrap();
    }

    assert_eq!(stack.current_scope_ref().key_count(), 100);
}

#[test]
fn test_empty_scope_path_at_root() {
    let stack = ScopeStack::new(2);
    assert_eq!(stack.get_scope_path(), "");
}

#[test]
fn test_scope_with_special_characters_in_parent_key() {
    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 1, Some("key-with-dashes".to_string()));
    assert_eq!(stack.get_scope_path(), "key-with-dashes");

    stack.enter_scope(4, 2, Some("key_with_underscores".to_string()));
    assert_eq!(stack.get_scope_path(), "key-with-dashes.key_with_underscores");

    stack.enter_scope(6, 3, Some("key.with.dots".to_string()));
    assert_eq!(stack.get_scope_path(), "key-with-dashes.key_with_underscores.key.with.dots");
}

#[test]
fn test_scope_with_numeric_keys() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("123", 1).unwrap();
    stack.add_key("456", 2).unwrap();

    assert!(stack.contains_key("123"));
    assert!(stack.contains_key("456"));
}

#[test]
fn test_scope_with_unicode_keys() {
    let mut stack = ScopeStack::new(2);

    stack.add_key("config", 1).unwrap();
    stack.add_key("配置", 2).unwrap(); // Chinese characters
    stack.add_key("конфиг", 3).unwrap(); // Cyrillic characters

    assert!(stack.contains_key("config"));
    assert!(stack.contains_key("配置"));
    assert!(stack.contains_key("конфиг"));
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
    assert!(stack.get_scope_at_level(8).is_none());
}

#[test]
fn test_current_indent_returns_correct_level() {
    let mut stack = ScopeStack::new(2);

    assert_eq!(stack.current_indent(), 0);

    stack.enter_scope(2, 1, Some("l1".to_string()));
    assert_eq!(stack.current_indent(), 2);

    stack.enter_scope(6, 2, Some("l2".to_string()));
    assert_eq!(stack.current_indent(), 6);

    stack.exit_to_scope(2);
    assert_eq!(stack.current_indent(), 2);
}

#[test]
fn test_depth_tracking() {
    let mut stack = ScopeStack::new(2);

    assert_eq!(stack.depth(), 1);

    stack.enter_scope(2, 1, Some("l1".to_string()));
    assert_eq!(stack.depth(), 2);

    stack.enter_scope(4, 2, Some("l2".to_string()));
    assert_eq!(stack.depth(), 3);

    stack.enter_scope(6, 3, Some("l3".to_string()));
    assert_eq!(stack.depth(), 4);

    stack.reset();
    assert_eq!(stack.depth(), 1);
}

// =============================================================================
// Integration with YAML Parsing
// =============================================================================

#[test]
fn test_simple_yaml_structure() {
    let yaml = r#"
key1: value1
key2: value2
key3: value3
"#;

    let mut stack = ScopeStack::new(2);

    for (line_num, line) in yaml.lines().enumerate() {
        let trimmed = line.trim();
        if trimmed.is_empty() || trimmed.starts_with('#') {
            continue;
        }

        if let Some(ctx) = extract_key_context(line) {
            if ctx.is_inline_scalar() {
                let _ = stack.add_key(ctx.key_name(), line_num + 1);
            }
        }
    }

    assert_eq!(stack.current_scope_ref().key_count(), 3);
}

#[test]
fn test_nested_yaml_structure() {
    let yaml = r#"
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
"#;

    let mut stack = ScopeStack::new(2);

    for (line_num, line) in yaml.lines().enumerate() {
        let trimmed = line.trim();
        if trimmed.is_empty() {
            continue;
        }

        let indent = get_leading_whitespace_length(line);

        // Handle scope transitions
        use std::cmp::Ordering;
        match indent.cmp(&stack.current_indent()) {
            Ordering::Greater => {
                // Entering deeper scope
                if let Some(ctx) = extract_key_context(line) {
                    if ctx.is_parent_key() {
                        stack.add_key(ctx.key_name(), line_num + 1).unwrap();
                        stack.enter_scope(indent, line_num + 1, Some(ctx.key_name().to_string()));
                    }
                }
            }
            Ordering::Less => {
                // Exiting to parent scope
                stack.exit_to_scope(indent);
                // After exiting, check if this line has a key
                if let Some(ctx) = extract_key_context(line) {
                    if ctx.is_parent_key() {
                        stack.add_key(ctx.key_name(), line_num + 1).unwrap();
                        stack.enter_scope(indent, line_num + 1, Some(ctx.key_name().to_string()));
                    } else if ctx.is_inline_scalar() {
                        stack.add_key(ctx.key_name(), line_num + 1).unwrap();
                    }
                }
            }
            Ordering::Equal => {
                // Same scope - check for keys
                if let Some(ctx) = extract_key_context(line) {
                    if ctx.is_parent_key() {
                        // This is a sibling scope at same indent level
                        stack.exit_to_scope(indent);
                        stack.add_key(ctx.key_name(), line_num + 1).unwrap();
                        stack.enter_scope(indent, line_num + 1, Some(ctx.key_name().to_string()));
                    } else if ctx.is_inline_scalar() {
                        stack.add_key(ctx.key_name(), line_num + 1).unwrap();
                    }
                }
            }
        }
    }

    // Should have keys in multiple scopes without duplicates
    assert!(stack.contains_key_in_any_scope("services"));
    assert!(stack.contains_key_in_any_scope("web"));
    assert!(stack.contains_key_in_any_scope("database"));
}

#[test]
fn test_sequence_yaml_structure() {
    let yaml = r#"
items:
  - name: item1
    value: 100
  - name: item2
    value: 200
  - name: item3
    value: 300
"#;

    let mut stack = ScopeStack::new(2);

    stack.enter_scope(2, 2, Some("items".to_string()));

    for (line_num, line) in yaml.lines().enumerate() {
        let trimmed = line.trim();
        if trimmed.is_empty() || trimmed.starts_with('#') {
            continue;
        }

        // Detect sequence items
        if trimmed.starts_with("- ") {
            stack.enter_sequence_scope(4, line_num + 1);
        } else if let Some(ctx) = extract_key_context(line) {
            if ctx.is_inline_scalar() {
                let _ = stack.add_key(ctx.key_name(), line_num + 1);
            }
        }
    }

    // Should have processed sequence items successfully
    assert!(stack.in_sequence_context());
}
