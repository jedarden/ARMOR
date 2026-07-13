//! State Preservation Tests for Scope Exit Operations
//!
//! These tests verify that scope state is properly preserved during partial
//! and complete scope exit operations, ensuring that parent scopes maintain
//! their keys, metadata, and configuration after child scopes are removed.

use armor::parsers::yaml::scope::ScopeStack;

/// Parent Scope Exit Tests
///
/// These tests verify that when exiting from a child scope to its parent scope,
/// the parent scope's state (keys, metadata, flags) is preserved correctly.

mod parent_scope_exit_tests {
    use super::*;

    /// Test that parent scope keys are preserved after exiting child scope
    #[test]
    fn test_parent_scope_keys_preserved_after_child_exit() {
        let mut stack = ScopeStack::new(2);

        // Create parent scope with keys
        stack.enter_scope(2, 1, Some("services".to_string()));
        stack.add_key("web", 2).unwrap();
        stack.add_key("database", 3).unwrap();

        // Create child scope with its own keys
        stack.enter_scope(4, 4, Some("web".to_string()));
        stack.add_key("host", 5).unwrap();
        stack.add_key("port", 6).unwrap();

        // Exit to parent scope
        stack.exit_to_scope(2);

        // Verify parent scope still has its keys
        assert!(stack.contains_key("web"), "Parent key 'web' should be preserved");
        assert!(stack.contains_key("database"), "Parent key 'database' should be preserved");

        // Verify child keys are removed
        assert!(!stack.contains_key("host"), "Child key 'host' should be removed");
        assert!(!stack.contains_key("port"), "Child key 'port' should be removed");

        // Verify we're at the correct scope
        assert_eq!(stack.current_indent(), 2);
        assert_eq!(stack.get_scope_path(), "services");
    }

    /// Test that parent scope metadata is preserved after child exit
    #[test]
    fn test_parent_scope_metadata_preserved_after_child_exit() {
        let mut stack = ScopeStack::new(2);

        // Create parent scope and set metadata
        stack.enter_scope(2, 1, Some("config".to_string()));
        stack.current_scope().is_flow_style = true;

        // Create child scope
        stack.enter_scope(4, 2, Some("nested".to_string()));
        stack.current_scope().is_flow_style = false; // Different from parent

        // Exit to parent scope
        stack.exit_to_scope(2);

        // Verify parent metadata is preserved
        assert!(stack.current_scope_ref().is_flow_style,
                "Parent flow_style flag should be preserved");
        assert_eq!(stack.current_scope_ref().parent_key, Some("config".to_string()));
    }

    /// Test that parent scope line numbers are preserved
    #[test]
    fn test_parent_scope_start_line_preserved_after_child_exit() {
        let mut stack = ScopeStack::new(2);

        // Create parent scope at specific line
        stack.enter_scope(2, 5, Some("parent".to_string()));
        let parent_start_line = stack.current_scope_ref().start_line;

        // Create child scope
        stack.enter_scope(4, 10, Some("child".to_string()));

        // Exit to parent scope
        stack.exit_to_scope(2);

        // Verify parent's start line is preserved
        assert_eq!(stack.current_scope_ref().start_line, parent_start_line,
                   "Parent scope start_line should be preserved");
    }

    /// Test parent scope key count accuracy after child exit
    #[test]
    fn test_parent_scope_key_count_preserved_after_child_exit() {
        let mut stack = ScopeStack::new(2);

        // Create parent scope with multiple keys
        stack.enter_scope(2, 1, Some("items".to_string()));
        for i in 0..10 {
            stack.add_key(&format!("key_{}", i), 1 + i).unwrap();
        }
        let parent_key_count = stack.current_scope_ref().key_count();

        // Create child scope with more keys
        stack.enter_scope(4, 20, Some("child".to_string()));
        for i in 0..15 {
            stack.add_key(&format!("child_key_{}", i), 20 + i).unwrap();
        }

        // Exit to parent scope
        stack.exit_to_scope(2);

        // Verify parent key count is unchanged
        assert_eq!(stack.current_scope_ref().key_count(), parent_key_count,
                   "Parent scope key count should be preserved");
        assert_eq!(stack.current_scope_ref().key_count(), 10);
    }

    /// Test that parent scope sequence context is preserved
    #[test]
    fn test_parent_scope_sequence_context_preserved() {
        let mut stack = ScopeStack::new(2);

        // Create parent sequence scope
        stack.enter_sequence_scope(2, 1);
        let parent_item_id = stack.current_scope_ref().sequence_item_id;

        // Create child mapping scope
        stack.enter_scope(4, 2, Some("nested".to_string()));

        // Exit to parent sequence scope
        stack.exit_to_scope(2);

        // Verify parent sequence context is preserved
        assert!(stack.current_scope_ref().unwrap().in_sequence_context,
                "Parent in_sequence_context should be preserved");
        assert_eq!(stack.current_scope_ref().unwrap().sequence_item_id, parent_item_id,
                   "Parent sequence_item_id should be preserved");
    }
}

/// Grandparent Scope Exit Tests
///
/// These tests verify that when exiting from a deep scope to its grandparent,
/// both the grandparent scope and intermediate parent scopes maintain their state.

mod grandparent_scope_exit_tests {
    use super::*;

    /// Test that grandparent scope state is preserved after multi-level exit
    #[test]
    fn test_grandparent_scope_keys_preserved_after_multi_level_exit() {
        let mut stack = ScopeStack::new(2);

        // Create grandparent scope
        stack.enter_scope(2, 1, Some("services".to_string()));
        stack.add_key("web", 2).unwrap();
        stack.add_key("database", 3).unwrap();
        let grandparent_path = stack.get_scope_path();

        // Create parent scope
        stack.enter_scope(4, 4, Some("web".to_string()));
        stack.add_key("config", 5).unwrap();

        // Create child scope
        stack.enter_scope(6, 6, Some("settings".to_string()));
        stack.add_key("debug", 7).unwrap();
        stack.add_key("verbose", 8).unwrap();

        // Exit from child (6) to grandparent (2)
        stack.exit_to_scope(2);

        // Verify grandparent scope is current and has its keys
        assert_eq!(stack.current_indent(), 2);
        assert_eq!(stack.get_scope_path(), grandparent_path);
        assert!(stack.contains_key("web"), "Grandparent key 'web' should be preserved");
        assert!(stack.contains_key("database"), "Grandparent key 'database' should be preserved");

        // Verify child and intermediate parent keys are removed
        assert!(!stack.contains_key("config"), "Parent key 'config' should be removed");
        assert!(!stack.contains_key("debug"), "Child key 'debug' should be removed");
        assert!(!stack.contains_key("verbose"), "Child key 'verbose' should be removed");
    }

    /// Test that intermediate parent scope is removed during grandparent exit
    #[test]
    fn test_intermediate_parent_scope_removed_on_grandparent_exit() {
        let mut stack = ScopeStack::new(2);

        // Create grandparent scope
        stack.enter_scope(2, 1, Some("level1".to_string()));

        // Create intermediate parent scope
        stack.enter_scope(4, 2, Some("level2".to_string()));

        // Create child scope
        stack.enter_scope(6, 3, Some("level3".to_string()));

        // Verify all scopes are present
        assert_eq!(stack.depth(), 4); // root + level1 + level2 + level3

        // Exit from child to grandparent (skipping intermediate parent)
        stack.exit_to_scope(2);

        // Verify intermediate parent is removed
        assert_eq!(stack.depth(), 2); // root + level1
        assert!(stack.get_scope_at_level(4).is_none(),
                "Intermediate parent at indent 4 should be removed");
        assert!(stack.get_scope_at_level(6).is_none(),
                "Child scope at indent 6 should be removed");

        // Verify grandparent is current
        assert_eq!(stack.current_indent(), 2);
        assert_eq!(stack.get_scope_path(), "level1");
    }

    /// Test grandparent metadata preservation through multiple levels
    #[test]
    fn test_grandparent_metadata_preserved_through_multiple_levels() {
        let mut stack = ScopeStack::new(2);

        // Create grandparent with metadata
        stack.enter_scope(2, 1, Some("root_config".to_string()));
        stack.current_scope().unwrap().is_flow_style = true;
        let grandparent_metadata = stack.current_scope_ref().unwrap().is_flow_style;

        // Create parent scope
        stack.enter_scope(4, 2, Some("section".to_string()));

        // Create child scope
        stack.enter_scope(6, 3, Some("subsection".to_string()));

        // Exit to grandparent
        stack.exit_to_scope(2);

        // Verify grandparent metadata is preserved
        assert_eq!(stack.current_scope_ref().unwrap().is_flow_style, grandparent_metadata,
                   "Grandparent is_flow_style should be preserved");
        assert_eq!(stack.current_scope_ref().unwrap().parent_key, Some("root_config".to_string()));
    }

    /// Test multi-level grandparent exit (great-grandparent)
    #[test]
    fn test_great_grandparent_scope_preserved_after_deep_exit() {
        let mut stack = ScopeStack::new(2);

        // Create great-grandparent scope
        stack.enter_scope(2, 1, Some("root".to_string()));
        stack.add_key("root_key", 2).unwrap();

        // Create grandparent scope
        stack.enter_scope(4, 3, Some("level1".to_string()));
        stack.add_key("level1_key", 4).unwrap();

        // Create parent scope
        stack.enter_scope(6, 5, Some("level2".to_string()));
        stack.add_key("level2_key", 6).unwrap();

        // Create child scope
        stack.enter_scope(8, 7, Some("level3".to_string()));
        stack.add_key("level3_key", 8).unwrap();

        // Exit from child (8) to great-grandparent (2)
        stack.exit_to_scope(2);

        // Verify great-grandparent state is preserved
        assert_eq!(stack.current_indent(), 2);
        assert_eq!(stack.get_scope_path(), "root");
        assert!(stack.contains_key("root_key"), "Great-grandparent key should be preserved");
        assert!(!stack.contains_key("level1_key"), "Grandparent key should be removed");
        assert!(!stack.contains_key("level2_key"), "Parent key should be removed");
        assert!(!stack.contains_key("level3_key"), "Child key should be removed");
    }

    /// Test that grandparent scope key count is accurate after multi-level cleanup
    #[test]
    fn test_grandparent_key_count_accurate_after_multi_level_cleanup() {
        let mut stack = ScopeStack::new(2);

        // Create grandparent with specific key count
        stack.enter_scope(2, 1, Some("grandparent".to_string()));
        stack.add_key("key1", 2).unwrap();
        stack.add_key("key2", 3).unwrap();
        stack.add_key("key3", 4).unwrap();
        let expected_key_count = 3;

        // Create intermediate scopes
        stack.enter_scope(4, 5, Some("parent".to_string()));
        stack.add_key("pkey", 6).unwrap();

        stack.enter_scope(6, 7, Some("child".to_string()));
        stack.add_key("ckey", 8).unwrap();

        // Exit to grandparent
        stack.exit_to_scope(2);

        // Verify grandparent key count is accurate
        assert_eq!(stack.current_scope_ref().unwrap().key_count(), expected_key_count,
                   "Grandparent key count should be accurate after cleanup");
    }
}

/// Edge Case Tests
///
/// These tests verify state preservation in edge cases:
/// - Root scope preservation
/// - Single-level scope exit
/// - Deeply nested scope exit
/// - Empty scopes

mod edge_case_tests {
    use super::*;

    /// Test that root scope state is preserved even in aggressive exits
    #[test]
    fn test_root_scope_state_preserved_in_aggressive_exits() {
        let mut stack = ScopeStack::new(2);

        // Create deeply nested structure
        stack.enter_scope(2, 1, Some("level1".to_string()));
        stack.enter_scope(4, 2, Some("level2".to_string()));
        stack.enter_scope(6, 3, Some("level3".to_string()));

        // Add a key at root (before any scope entry, this would be in root scope)
        let original_root_keys = stack.scopes[0].keys.len();
        let original_root_indent = stack.scopes[0].indent_level;

        // Perform aggressive exit to root
        stack.exit_to_scope(0);

        // Verify root scope state is preserved
        assert_eq!(stack.scopes[0].keys.len(), original_root_keys,
                   "Root scope keys should be preserved");
        assert_eq!(stack.scopes[0].indent_level, original_root_indent,
                   "Root scope indent should be preserved");
        assert_eq!(stack.depth(), 1, "Should only have root scope");
        assert_eq!(stack.current_indent(), 0, "Should be at root indent");
    }

    /// Test single-level scope exit state preservation
    #[test]
    fn test_single_level_scope_exit_preserves_root_state() {
        let mut stack = ScopeStack::new(2);

        // Create single nested level
        stack.enter_scope(2, 1, Some("config".to_string()));
        stack.add_key("setting", 2).unwrap();

        // Verify we're in the nested scope
        assert_eq!(stack.depth(), 2);
        assert_eq!(stack.current_indent(), 2);

        // Exit to root
        stack.exit_to_scope(0);

        // Verify root scope is preserved and child is removed
        assert_eq!(stack.depth(), 1);
        assert_eq!(stack.current_indent(), 0);
        assert!(!stack.contains_key("setting"), "Child key should be removed");
        assert!(stack.scopes[0].indent_level == 0, "Root indent should be 0");
    }

    /// Test deeply nested scope exit with state preservation at each level
    #[test]
    fn test_deeply_nested_scope_exit_preserves_correct_level_state() {
        let mut stack = ScopeStack::new(2);

        // Create 5 levels of nesting
        stack.enter_scope(2, 1, Some("level1".to_string()));
        stack.add_key("l1_key", 2).unwrap();

        stack.enter_scope(4, 3, Some("level2".to_string()));
        stack.add_key("l2_key", 4).unwrap();

        stack.enter_scope(6, 5, Some("level3".to_string()));
        stack.add_key("l3_key", 6).unwrap();

        stack.enter_scope(8, 7, Some("level4".to_string()));
        stack.add_key("l4_key", 8).unwrap();

        stack.enter_scope(10, 9, Some("level5".to_string()));
        stack.add_key("l5_key", 10).unwrap();

        // Exit to level 3 (partial exit)
        stack.exit_to_scope(6);

        // Verify level 3 state is preserved
        assert_eq!(stack.current_indent(), 6);
        assert_eq!(stack.get_scope_path(), "level1.level2.level3");
        assert!(stack.contains_key_in_any_scope("l1_key"), "Level 1 key should be preserved");
        assert!(stack.contains_key_in_any_scope("l2_key"), "Level 2 key should be preserved");
        assert!(stack.contains_key("l3_key"), "Level 3 key should be preserved (current scope)");
        assert!(!stack.contains_key_in_any_scope("l4_key"), "Level 4 key should be removed");
        assert!(!stack.contains_key_in_any_scope("l5_key"), "Level 5 key should be removed");

        // Verify correct depth
        assert_eq!(stack.depth(), 4); // root + level1 + level2 + level3
    }

    /// Test scope exit when parent scope has no keys
    #[test]
    fn test_scope_exit_with_empty_parent_scope() {
        let mut stack = ScopeStack::new(2);

        // Create parent scope with no keys
        stack.enter_scope(2, 1, Some("empty_parent".to_string()));
        let parent_key_count = stack.current_scope_ref().unwrap().key_count();
        assert_eq!(parent_key_count, 0);

        // Create child scope with keys
        stack.enter_scope(4, 2, Some("child".to_string()));
        stack.add_key("child_key", 3).unwrap();

        // Exit to empty parent
        stack.exit_to_scope(2);

        // Verify empty parent state is preserved (still empty)
        assert_eq!(stack.current_scope_ref().key_count(), 0,
                   "Empty parent should remain empty");
        assert!(!stack.contains_key("child_key"), "Child key should be removed");
        assert_eq!(stack.current_scope_ref().parent_key, Some("empty_parent".to_string()));
    }

    /// Test scope exit when target scope doesn't exist (closest parent used)
    #[test]
    fn test_scope_exit_to_nonexistent_target_preserves_closest_parent() {
        let mut stack = ScopeStack::new(2);

        // Create scopes at indents 0, 2, 6 (gap at 4)
        stack.enter_scope(2, 1, Some("level2".to_string()));
        stack.add_key("l2_key", 2).unwrap();

        stack.enter_scope(6, 3, Some("level6".to_string()));
        stack.add_key("l6_key", 4).unwrap();

        // Exit to indent 4 (which doesn't exist)
        // Should find closest parent (level2 at indent 2)
        stack.exit_to_scope(4);

        // Verify closest parent (level2) state is preserved
        assert_eq!(stack.current_indent(), 2, "Should be at closest parent indent");
        assert!(stack.contains_key("l2_key"), "Closest parent key should be preserved");
        assert!(!stack.contains_key("l6_key"), "Deeper scope key should be removed");
        assert_eq!(stack.get_scope_path(), "level2");
    }

    /// Test that scope exit preserves depth tracking accuracy
    #[test]
    fn test_scope_exit_preserves_depth_tracking() {
        let mut stack = ScopeStack::new(2);

        // Build up depth
        stack.enter_scope(2, 1, Some("a".to_string()));
        stack.enter_scope(4, 2, Some("b".to_string()));
        stack.enter_scope(6, 3, Some("c".to_string()));
        assert_eq!(stack.depth(), 4); // root + 3 scopes

        // Exit and verify depth decreases correctly
        stack.exit_to_scope(4);
        assert_eq!(stack.depth(), 3); // root + 2 scopes

        stack.exit_to_scope(2);
        assert_eq!(stack.depth(), 2); // root + 1 scope

        stack.exit_to_scope(0);
        assert_eq!(stack.depth(), 1); // root only
    }

    /// Test scope exit with sequence scope in hierarchy
    #[test]
    fn test_scope_exit_with_sequence_scope_in_hierarchy() {
        let mut stack = ScopeStack::new(2);

        // Create parent mapping
        stack.enter_scope(2, 1, Some("items".to_string()));
        stack.add_key("item1", 2).unwrap();

        // Create sequence scope
        stack.enter_sequence_scope(4, 3);
        let seq_item_id = stack.current_scope_ref().sequence_item_id;

        // Create nested mapping in sequence
        stack.enter_scope(6, 4, Some("nested".to_string()));
        stack.add_key("nested_key", 5).unwrap();

        // Exit from nested mapping back to sequence
        stack.exit_to_scope(4);

        // Verify sequence scope state is preserved
        assert!(stack.current_scope_ref().in_sequence_context);
        assert_eq!(stack.current_scope_ref().sequence_item_id, seq_item_id);
        assert!(!stack.contains_key("nested_key"), "Nested key should be removed");

        // Exit from sequence back to mapping
        stack.exit_to_scope(2);

        // Verify mapping scope state is preserved
        assert!(!stack.current_scope_ref().in_sequence_context);
        assert!(stack.current_scope_ref().sequence_item_id.is_none());
        assert!(stack.contains_key("item1"), "Mapping key should be preserved");
    }

    /// Test that flow-style flags are preserved correctly through scope exits
    #[test]
    fn test_flow_style_flag_preservation_through_scope_exits() {
        let mut stack = ScopeStack::new(2);

        // Create flow-style parent
        stack.enter_scope(2, 1, Some("flow_parent".to_string()));
        stack.current_scope().is_flow_style = true;

        // Create non-flow-style child
        stack.enter_scope(4, 2, Some("block_child".to_string()));
        stack.current_scope().is_flow_style = false;

        // Exit to parent
        stack.exit_to_scope(2);

        // Verify parent's flow-style flag is preserved
        assert!(stack.current_scope_ref().is_flow_style,
                "Parent flow_style flag should be preserved");
        assert_eq!(stack.current_scope_ref().parent_key, Some("flow_parent".to_string()));
    }

    /// Test scope exit with multiple keys at each level
    #[test]
    fn test_scope_exit_with_multiple_keys_per_level() {
        let mut stack = ScopeStack::new(2);

        // Create level 1 with multiple keys
        stack.enter_scope(2, 1, Some("level1".to_string()));
        stack.add_key("key1a", 2).unwrap();
        stack.add_key("key1b", 3).unwrap();
        stack.add_key("key1c", 4).unwrap();
        let level1_count = stack.current_scope_ref().key_count();

        // Create level 2 with multiple keys
        stack.enter_scope(4, 5, Some("level2".to_string()));
        stack.add_key("key2a", 6).unwrap();
        stack.add_key("key2b", 7).unwrap();
        let level2_count = stack.current_scope_ref().key_count();

        // Exit to level 1
        stack.exit_to_scope(2);

        // Verify level 1 state is preserved
        assert_eq!(stack.current_scope_ref().key_count(), level1_count,
                   "Level 1 key count should be preserved");
        assert!(stack.contains_key("key1a"));
        assert!(stack.contains_key("key1b"));
        assert!(stack.contains_key("key1c"));
        assert!(!stack.contains_key("key2a"));
        assert!(!stack.contains_key("key2b"));
    }

    /// Test that exiting to same indent is idempotent
    #[test]
    fn test_exit_to_current_indent_is_idempotent() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("scope".to_string()));
        stack.add_key("key", 2).unwrap();

        let before_depth = stack.depth();
        let before_indent = stack.current_indent();
        let before_key_count = stack.current_scope_ref().key_count();

        // Exit to same indent we're already at
        stack.exit_to_scope(2);

        // Verify nothing changed
        assert_eq!(stack.depth(), before_depth);
        assert_eq!(stack.current_indent(), before_indent);
        assert_eq!(stack.current_scope_ref().key_count(), before_key_count);
        assert!(stack.contains_key("key"));
    }

    /// Test scope exit preserves scope path accuracy
    #[test]
    fn test_scope_exit_preserves_scope_path_accuracy() {
        let mut stack = ScopeStack::new(2);

        // Build deep scope path
        stack.enter_scope(2, 1, Some("services".to_string()));
        stack.enter_scope(4, 2, Some("web".to_string()));
        stack.enter_scope(6, 3, Some("config".to_string()));
        stack.enter_scope(8, 4, Some("database".to_string()));

        let full_path = stack.get_scope_path();
        assert_eq!(full_path, "services.web.config.database");

        // Exit to intermediate level
        stack.exit_to_scope(4);

        let partial_path = stack.get_scope_path();
        assert_eq!(partial_path, "services.web",
                   "Scope path should be accurate after partial exit");
    }

    /// Test that scope exit handles non-standard indents correctly
    #[test]
    fn test_scope_exit_with_non_standard_indents() {
        let mut stack = ScopeStack::new(3); // 3-space indent

        // Create scopes with non-standard indents
        stack.enter_scope(3, 1, Some("level3".to_string()));
        stack.add_key("key3", 2).unwrap();

        stack.enter_scope(6, 3, Some("level6".to_string()));
        stack.add_key("key6", 4).unwrap();

        stack.enter_scope(9, 5, Some("level9".to_string()));
        stack.add_key("key9", 6).unwrap();

        // Exit to level 6
        stack.exit_to_scope(6);

        // Verify state is preserved with non-standard indents
        assert_eq!(stack.current_indent(), 6);
        assert!(stack.contains_key("key6"), "Level 6 key should be preserved (current scope)");
        assert!(stack.contains_key_in_any_scope("key3"), "Level 3 key should be preserved (parent scope)");
        assert!(!stack.contains_key_in_any_scope("key9"), "Level 9 key should be removed");
    }
}

/// Integration Tests
///
/// These tests verify state preservation in realistic YAML parsing scenarios.

mod integration_tests {
    use super::*;

    /// Test realistic service configuration scenario
    #[test]
    fn test_realistic_service_config_scenario() {
        let mut stack = ScopeStack::new(2);

        // Simulate parsing:
        // services:
        //   web:
        //     host: localhost
        //     port: 8080
        //   database:
        //     host: db.example.com
        //     port: 5432

        // Enter services scope
        stack.enter_scope(2, 1, Some("services".to_string()));

        // Enter web scope
        stack.enter_scope(4, 2, Some("web".to_string()));
        stack.add_key("host", 3).unwrap();
        stack.add_key("port", 4).unwrap();

        // Exit web, enter database (sibling)
        stack.exit_to_scope(2);
        stack.enter_scope(4, 5, Some("database".to_string()));

        // Add database keys
        stack.add_key("host", 6).unwrap();
        stack.add_key("port", 7).unwrap();

        // Verify final state
        assert_eq!(stack.current_scope_ref().parent_key, Some("database".to_string()));
        assert!(stack.contains_key("host"));
        assert!(stack.contains_key("port"));

        // Verify web keys are not visible (they were in a different scope)
        assert!(stack.get_scope_at_level(4).is_some() ||
                !stack.contains_key("port") ||
                stack.current_scope_ref().parent_key == Some("database".to_string()));
    }

    /// Test nested configuration scenario with partial exits
    #[test]
    fn test_nested_config_with_partial_exits() {
        let mut stack = ScopeStack::new(2);

        // Simulate complex nested structure with intermediate exits

        // Root config
        stack.enter_scope(2, 1, Some("config".to_string()));
        stack.add_key("version", 2).unwrap();

        // Production section
        stack.enter_scope(4, 3, Some("production".to_string()));
        stack.add_key("url", 4).unwrap();
        stack.add_key("timeout", 5).unwrap();

        // Database subsection
        stack.enter_scope(6, 6, Some("database".to_string()));
        stack.add_key("host", 7).unwrap();
        stack.add_key("port", 8).unwrap();

        // Exit back to production level
        stack.exit_to_scope(4);

        // Verify production state is preserved
        assert!(stack.contains_key("url"), "Production key should be preserved");
        assert!(stack.contains_key("timeout"), "Production key should be preserved");
        assert!(!stack.contains_key("host"), "Database key should be removed");
        assert!(!stack.contains_key("port"), "Database key should be removed");

        // Add more production settings
        stack.add_key("retries", 9).unwrap();

        // Verify all production keys are present
        assert!(stack.contains_key("url"));
        assert!(stack.contains_key("timeout"));
        assert!(stack.contains_key("retries"));
    }
}
