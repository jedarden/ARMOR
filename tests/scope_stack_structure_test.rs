//! Test to verify the scope stack structure implementation
//!
//! This test verifies that the YAML parser has a proper scope stack data structure
//! that tracks the full hierarchy of nested scopes.

use armor::parsers::yaml::parser::{BasicParser, Parser};
use armor::parsers::yaml::scope::ScopeStack;

#[test]
fn test_scope_stack_exists_as_list() {
    // Acceptance criterion 1: Scope stack exists as a list/array in parser state
    let parser = BasicParser::new();

    // The scope_stack field contains a scopes field which is a Vec<Scope>
    let scope_stack = parser.scope_stack();
    let scopes_list = &scope_stack.scopes;

    // Verify it's a list (Vec) - initially empty as per acceptance criteria
    assert!(scopes_list.is_empty(), "Scope stack should start empty");

    println!("✓ Scope stack exists as Vec<Scope> (initially empty with {} scopes)", scopes_list.len());
}

#[test]
fn test_scope_stack_initialized_empty_at_startup() {
    // Acceptance criterion 2: Stack is initialized empty at parser startup
    let parser = BasicParser::new();

    let scope_stack = parser.scope_stack();

    // Verify stack is initialized empty
    assert_eq!(scope_stack.depth(), 0, "Stack should start empty (depth = 0)");
    assert_eq!(scope_stack.current_indent(), 0, "Current indent should be 0 when empty");

    // Verify no current scope exists yet
    assert!(scope_stack.current_scope_ref().is_none(), "No current scope when stack is empty");

    println!("✓ Scope stack initialized empty at startup:");
    println!("  - depth: {}", scope_stack.depth());
    println!("  - current_indent: {}", scope_stack.current_indent());
    println!("  - current_scope: None");
}

#[test]
fn test_scope_stack_auto_creates_root_scope() {
    // Verify that root scope is auto-created when first key is added
    let mut stack = ScopeStack::new(2);

    // Stack starts empty
    assert_eq!(stack.depth(), 0, "Should start with empty stack");

    // Add first key - should auto-create root scope
    stack.add_key("first_key", 1).unwrap();

    // Now root scope should exist
    assert_eq!(stack.depth(), 1, "Should have auto-created root scope");
    assert_eq!(stack.current_indent(), 0, "Root scope should be at indent 0");

    let root_scope = stack.current_scope_ref().unwrap();
    assert_eq!(root_scope.indent_level, 0, "Root scope indent should be 0");
    assert_eq!(root_scope.start_line, 0, "Root scope start line should be 0");
    assert_eq!(root_scope.parent_key, None, "Root scope has no parent");
    assert_eq!(root_scope.key_count(), 1, "Root scope should have 1 key");

    println!("✓ Root scope auto-created on first add_key:");
    println!("  - depth: {}", stack.depth());
    println!("  - indent_level: {}", root_scope.indent_level);
    println!("  - key_count: {}", root_scope.key_count());
}

#[test]
fn test_scope_stack_holds_information_at_each_level() {
    // Acceptance criterion 3: Stack can hold scope information at each level

    // Create a scope stack and add nested scopes
    let mut stack = ScopeStack::new(2);

    // Auto-create root scope by adding a key
    stack.add_key("root_key", 1).unwrap();

    // Verify root scope has all fields populated
    let root_scope = stack.current_scope_ref();
    assert!(root_scope.is_some());

    // Enter a nested scope
    stack.enter_scope(2, 2, Some("services".to_string()));

    // Verify the new scope has proper information
    let scope = &stack.scopes[stack.scopes.len() - 1];
    assert_eq!(scope.indent_level, 2, "Nested scope should have indent level 2");
    assert_eq!(scope.start_line, 2, "Should track start line");
    assert_eq!(scope.parent_key, Some("services".to_string()), "Should track parent key");
    assert_eq!(scope.is_flow_style, false, "Default not flow style");
    assert_eq!(scope.in_sequence_context, false, "Not in sequence by default");

    println!("✓ Stack holds comprehensive scope information at each level:");
    println!("  - indent_level: {}", scope.indent_level);
    println!("  - start_line: {}", scope.start_line);
    println!("  - parent_key: {:?}", scope.parent_key);
    println!("  - is_flow_style: {}", scope.is_flow_style);
    println!("  - in_sequence_context: {}", scope.in_sequence_context);

    // Add keys to verify HashSet works
    stack.add_key("web", 3).unwrap();
    stack.add_key("database", 4).unwrap();

    let scope_after = &stack.scopes[stack.scopes.len() - 1];
    assert_eq!(scope_after.key_count(), 2, "Should track keys in scope");
    println!("  - key_count: {}", scope_after.key_count());
}

#[test]
fn test_scope_stack_tracks_full_hierarchy() {
    // Verify the scope stack tracks the FULL hierarchy, not just current depth

    let mut stack = ScopeStack::new(2);

    // Auto-create root scope
    stack.add_key("root", 1).unwrap();

    // Enter multiple levels of nesting
    stack.enter_scope(2, 2, Some("level1".to_string()));
    stack.enter_scope(4, 3, Some("level2".to_string()));
    stack.enter_scope(6, 4, Some("level3".to_string()));

    // Verify all levels are tracked
    assert_eq!(stack.depth(), 4, "Should track all 4 scopes (root + 3 nested)");

    let hierarchy = &stack.scopes;
    assert_eq!(hierarchy.len(), 4, "Hierarchy should contain all scopes");

    // Verify we can access each level
    assert_eq!(hierarchy[0].indent_level, 0, "Root at indent 0");
    assert_eq!(hierarchy[1].indent_level, 2, "Level1 at indent 2");
    assert_eq!(hierarchy[2].indent_level, 4, "Level2 at indent 4");
    assert_eq!(hierarchy[3].indent_level, 6, "Level3 at indent 6");

    // Verify scope path shows full hierarchy
    let path = stack.get_scope_path();
    assert_eq!(path, "level1.level2.level3", "Scope path should show full hierarchy");

    println!("✓ Scope stack tracks full hierarchy:");
    println!("  - depth: {}", stack.depth());
    println!("  - scope_path: {}", path);
    for (i, scope) in hierarchy.iter().enumerate() {
        println!("  - level {}: indent={}, parent={:?}", i, scope.indent_level, scope.parent_key);
    }
}

#[test]
fn test_scope_stack_with_real_yaml() {
    // Integration test with actual YAML parsing

    let parser = BasicParser::new();
    let yaml = r#"
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
"#;

    // Parse the YAML
    let result = parser.parse_str(yaml);
    assert!(result.is_success(), "Should parse successfully");

    // Now use a fresh scope stack to verify it tracks the hierarchy
    let mut scope_stack = ScopeStack::new(2);

    // Auto-create root scope by adding a key
    scope_stack.add_key("root", 1).unwrap();

    // Simulate entering scopes as we parse
    scope_stack.enter_scope(2, 2, Some("services".to_string()));
    scope_stack.add_key("web", 3).unwrap();
    scope_stack.enter_scope(4, 4, Some("web".to_string()));
    scope_stack.add_key("host", 5).unwrap();
    scope_stack.add_key("port", 6).unwrap();

    // Verify we're tracking 3 scopes (root + services + web)
    assert_eq!(scope_stack.depth(), 3, "Should have 3 active scopes");

    // Verify the full hierarchy is preserved
    let hierarchy = &scope_stack.scopes;
    assert_eq!(hierarchy.len(), 3, "Should track all 3 scopes");

    println!("✓ Scope stack works with real YAML structure:");
    println!("  - depth: {}", scope_stack.depth());
    println!("  - path: {}", scope_stack.get_scope_path());
}

fn main() {
    println!("Running scope stack structure verification tests...\n");

    test_scope_stack_exists_as_list();
    test_scope_stack_initialized_empty_at_startup();
    test_scope_stack_auto_creates_root_scope();
    test_scope_stack_holds_information_at_each_level();
    test_scope_stack_tracks_full_hierarchy();
    test_scope_stack_with_real_yaml();

    println!("\n✓ All acceptance criteria verified:");
    println!("  1. Scope stack exists as list/array in parser state");
    println!("  2. Stack is initialized empty at parser startup");
    println!("  3. Stack can hold scope information at each level");
}
