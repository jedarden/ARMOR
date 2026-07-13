#!/usr/bin/env python3
"""
Test comprehensive scope exit functionality in YAML parser.

This test validates that the Python YAML parser properly handles
exiting scopes at different levels (parent, grandparent) while
preserving state, covering all edge cases.
"""

import sys
from pathlib import Path

# Add the tools directory to the path
sys.path.insert(0, str(Path(__file__).parent / 'tools' / 'parse_module'))

from yaml_parser import ScopeStack


def test_parent_scope_exit_basic():
    """Test basic parent scope exit (exit_one_level)."""
    print("=== Test 1: Parent Scope Exit Basic ===")

    stack = ScopeStack()

    # Create nested scopes
    stack.enter_scope(indent_level=2, line=1, parent_key="level1")
    stack.add_key("key1", 1)
    assert stack.depth() == 2, f"Expected depth 2, got {stack.depth()}"

    stack.enter_scope(indent_level=4, line=2, parent_key="level2")
    stack.add_key("key2", 2)
    assert stack.depth() == 3, f"Expected depth 3, got {stack.depth()}"

    # Exit one level to parent
    exited = stack.exit_one_level()
    assert exited, "Should have exited successfully"
    assert stack.depth() == 2, f"Expected depth 2 after exit, got {stack.depth()}"
    assert stack.current_indent() == 2, f"Expected indent 2, got {stack.current_indent()}"
    assert stack.current_scope().parent_key == "level1", f"Expected parent_key 'level1', got {stack.current_scope().parent_key}"

    # Verify state preservation - level1 scope should still have key1
    assert stack.contains_key("key1"), "key1 should still exist in level1 scope"
    assert not stack.contains_key("key2"), "key2 should not exist (was in child scope)"

    print("  ✓ Parent scope exit works correctly")
    print("  ✓ State is preserved after exit")


def test_parent_exit_from_root():
    """Test parent scope exit from root scope (should fail)."""
    print("\n=== Test 2: Parent Exit From Root ===")

    stack = ScopeStack()
    assert stack.depth() == 1, "Should start at root scope (depth 1)"

    # Try to exit from root - should fail
    exited = stack.exit_one_level()
    assert not exited, "Should not be able to exit from root"
    assert stack.depth() == 1, "Should still be at root depth 1"

    print("  ✓ Cannot exit from root scope")


def test_grandparent_scope_exit():
    """Test grandparent scope exit (scope_exit_to_level)."""
    print("\n=== Test 3: Grandparent Scope Exit ===")

    stack = ScopeStack()

    # Create deeply nested scopes: root(0) -> level1(2) -> level2(4) -> level3(6) -> level4(8)
    stack.enter_scope(indent_level=2, line=1, parent_key="level1")
    stack.add_key("key1", 1)

    stack.enter_scope(indent_level=4, line=2, parent_key="level2")
    stack.add_key("key2", 2)

    stack.enter_scope(indent_level=6, line=3, parent_key="level3")
    stack.add_key("key3", 3)

    stack.enter_scope(indent_level=8, line=4, parent_key="level4")
    stack.add_key("key4", 4)

    assert stack.depth() == 5, f"Expected depth 5, got {stack.depth()}"

    # Exit from depth 5 to depth 3 (skip level4, exit to level2)
    # Depth: 1(root) -> 2(level1) -> 3(level2) -> 4(level3) -> 5(level4)
    # Target depth 3 means we want to be at level2 scope
    exited = stack.scope_exit_to_level(target_depth=3)
    assert exited, "Should have exited successfully"
    assert stack.depth() == 3, f"Expected depth 3 after exit, got {stack.depth()}"
    assert stack.current_indent() == 4, f"Expected indent 4, got {stack.current_indent()}"
    assert stack.current_scope().parent_key == "level2", f"Expected parent_key 'level2', got {stack.current_scope().parent_key}"

    # Verify state preservation
    assert stack.contains_key("key2"), "key2 should still exist in level2 scope"
    assert not stack.contains_key("key3"), "key3 should not exist (was in child scope)"
    assert not stack.contains_key("key4"), "key4 should not exist (was in grandchild scope)"

    print("  ✓ Grandparent scope exit works correctly")
    print("  ✓ State is preserved after multi-level exit")


def test_exit_to_same_level():
    """Test exit to the same level (no-op)."""
    print("\n=== Test 4: Exit to Same Level ===")

    stack = ScopeStack()

    stack.enter_scope(indent_level=2, line=1, parent_key="level1")
    stack.add_key("key1", 1)
    stack.enter_scope(indent_level=4, line=2, parent_key="level2")

    current_depth = stack.depth()

    # Exit to the same depth we're already at - should succeed but be no-op
    exited = stack.scope_exit_to_level(target_depth=current_depth)
    assert exited, "Should succeed even if already at target"
    assert stack.depth() == current_depth, f"Depth should remain {current_depth}"
    assert stack.current_scope().parent_key == "level2", "Should still be at level2"

    print("  ✓ Exit to same level succeeds as no-op")


def test_exit_to_invalid_depth_negative():
    """Test exit to invalid depth (negative)."""
    print("\n=== Test 5: Exit to Invalid Depth (Negative) ===")

    stack = ScopeStack()
    stack.enter_scope(indent_level=2, line=1, parent_key="level1")

    try:
        stack.scope_exit_to_level(target_depth=-1)
        assert False, "Should have raised ValueError for negative depth"
    except ValueError as e:
        assert "positive" in str(e).lower(), f"Error should mention positive requirement, got: {e}"
        print(f"  ✓ Correctly rejects negative depth: {e}")


def test_exit_to_invalid_depth_too_large():
    """Test exit to invalid depth (greater than current)."""
    print("\n=== Test 6: Exit to Invalid Depth (Too Large) ===")

    stack = ScopeStack()
    stack.enter_scope(indent_level=2, line=1, parent_key="level1")

    current_depth = stack.depth()

    try:
        # Try to exit to depth deeper than current
        stack.scope_exit_to_level(target_depth=current_depth + 5)
        assert False, "Should have raised ValueError for depth > current"
    except ValueError as e:
        assert "current depth" in str(e).lower(), f"Error should mention current depth, got: {e}"
        print(f"  ✓ Correctly rejects depth > current: {e}")


def test_exit_from_single_level_stack():
    """Test exit from single-level (root-only) stack."""
    print("\n=== Test 7: Exit From Single-Level Stack ===")

    stack = ScopeStack()
    assert stack.depth() == 1, "Stack starts at depth 1 (root only)"

    # Try to exit from root-only stack
    exited = stack.exit_one_level()
    assert not exited, "Should not exit from root-only stack"
    assert stack.depth() == 1, "Depth should remain 1"

    print("  ✓ Single-level stack exit handled correctly")


def test_state_preservation_keys():
    """Test that keys are preserved after scope exit."""
    print("\n=== Test 8: State Preservation - Keys ===")

    stack = ScopeStack()

    # Create nested scopes with keys
    stack.enter_scope(indent_level=2, line=1, parent_key="services")
    stack.add_key("web", 1)
    stack.add_key("database", 2)

    stack.enter_scope(indent_level=4, line=3, parent_key="web")
    stack.add_key("host", 3)
    stack.add_key("port", 4)

    stack.enter_scope(indent_level=6, line=5, parent_key="config")
    stack.add_key("debug", 5)

    # Exit from depth 4 to depth 2 (services scope)
    exited = stack.scope_exit_to_level(target_depth=2)
    assert exited, "Exit should succeed"

    # Verify services scope still has its keys
    current_scope = stack.current_scope()
    assert current_scope.parent_key == "services", "Should be at services scope"
    assert "web" in current_scope.keys, "services scope should have 'web' key"
    assert "database" in current_scope.keys, "services scope should have 'database' key"
    assert "host" not in current_scope.keys, "services scope should not have child scope keys"

    print("  ✓ Keys are preserved in target scope after exit")


def test_state_preservation_attributes():
    """Test that scope attributes are preserved after exit."""
    print("\n=== Test 9: State Preservation - Attributes ===")

    stack = ScopeStack()

    # Create scope with specific attributes
    stack.enter_scope(indent_level=2, line=10, parent_key="level1")
    first_scope = stack.current_scope()
    first_scope.is_flow_style = True
    first_scope.in_sequence_context = True
    first_scope.sequence_item_id = 42

    # Go deeper
    stack.enter_scope(indent_level=4, line=20, parent_key="level2")
    stack.enter_scope(indent_level=6, line=30, parent_key="level3")

    # Exit back to level1
    stack.scope_exit_to_level(target_depth=2)

    # Verify attributes are preserved
    restored_scope = stack.current_scope()
    assert restored_scope.parent_key == "level1", "Parent key should be preserved"
    assert restored_scope.start_line == 10, "Start line should be preserved"
    assert restored_scope.indent_level == 2, "Indent level should be preserved"
    assert restored_scope.is_flow_style, "Flow style flag should be preserved"
    assert restored_scope.in_sequence_context, "Sequence context should be preserved"
    assert restored_scope.sequence_item_id == 42, "Sequence item ID should be preserved"

    print("  ✓ Scope attributes are preserved after exit")


def test_nested_structure_comprehensive():
    """Test exit operations on a complex nested structure."""
    print("\n=== Test 10: Nested Structure Comprehensive ===")

    stack = ScopeStack()

    # Build complex nested structure
    # root -> app -> services -> web -> config -> ssl
    stack.enter_scope(indent_level=2, line=1, parent_key="app")
    stack.enter_scope(indent_level=4, line=2, parent_key="services")
    stack.enter_scope(indent_level=6, line=3, parent_key="web")
    stack.enter_scope(indent_level=8, line=4, parent_key="config")
    stack.enter_scope(indent_level=10, line=5, parent_key="ssl")

    assert stack.depth() == 6, f"Should have depth 6, got {stack.depth()}"

    # Exit from ssl to config
    exited = stack.scope_exit_to_level(target_depth=5)
    assert exited, "Exit to config should succeed"
    assert stack.depth() == 5, f"Depth should be 5, got {stack.depth()}"
    assert stack.current_scope().parent_key == "config", "Should be at config scope"

    # Exit from config to services (skip web)
    exited = stack.scope_exit_to_level(target_depth=3)
    assert exited, "Exit to services should succeed"
    assert stack.depth() == 3, f"Depth should be 3, got {stack.depth()}"
    assert stack.current_scope().parent_key == "services", "Should be at services scope"

    # Exit from services to root (skip app)
    exited = stack.scope_exit_to_level(target_depth=1)
    assert exited, "Exit to root should succeed"
    assert stack.depth() == 1, f"Depth should be 1, got {stack.depth()}"

    print("  ✓ Multi-step exit operations work correctly")


def test_exit_one_level_multiple_times():
    """Test calling exit_one_level multiple times."""
    print("\n=== Test 11: Exit One Level Multiple Times ===")

    stack = ScopeStack()

    # Build 5-level stack
    stack.enter_scope(indent_level=2, line=1, parent_key="l1")
    stack.enter_scope(indent_level=4, line=2, parent_key="l2")
    stack.enter_scope(indent_level=6, line=3, parent_key="l3")
    stack.enter_scope(indent_level=8, line=4, parent_key="l4")

    assert stack.depth() == 5, f"Should have depth 5, got {stack.depth()}"

    # Exit one level at a time
    exited = stack.exit_one_level()
    assert exited and stack.depth() == 4, "First exit: depth should be 4"

    exited = stack.exit_one_level()
    assert exited and stack.depth() == 3, "Second exit: depth should be 3"

    exited = stack.exit_one_level()
    assert exited and stack.depth() == 2, "Third exit: depth should be 2"

    exited = stack.exit_one_level()
    assert exited and stack.depth() == 1, "Fourth exit: depth should be 1 (root)"

    # Try to exit from root - should fail
    exited = stack.exit_one_level()
    assert not exited and stack.depth() == 1, "Cannot exit from root"

    print("  ✓ Repeated exit_one_level calls work correctly")


def test_mixed_exit_operations():
    """Test mixing exit_one_level and scope_exit_to_level."""
    print("\n=== Test 12: Mixed Exit Operations ===")

    stack = ScopeStack()

    # Build 6-level stack
    stack.enter_scope(indent_level=2, line=1, parent_key="a")
    stack.enter_scope(indent_level=4, line=2, parent_key="b")
    stack.enter_scope(indent_level=6, line=3, parent_key="c")
    stack.enter_scope(indent_level=8, line=4, parent_key="d")
    stack.enter_scope(indent_level=10, line=5, parent_key="e")

    assert stack.depth() == 6, f"Should have depth 6, got {stack.depth()}"

    # Exit one level
    stack.exit_one_level()
    assert stack.depth() == 5, f"After exit_one_level: depth should be 5"

    # Exit to level 2 (skip multiple levels)
    stack.scope_exit_to_level(target_depth=2)
    assert stack.depth() == 2, f"After scope_exit_to_level(2): depth should be 2"

    # Exit one level again
    stack.exit_one_level()
    assert stack.depth() == 1, f"After exit_one_level: depth should be 1 (root)"

    print("  ✓ Mixed exit operations work correctly")


def test_scope_path_after_exit():
    """Test that scope path is correct after exit operations."""
    print("\n=== Test 13: Scope Path After Exit ===")

    stack = ScopeStack()

    stack.enter_scope(indent_level=2, line=1, parent_key="app")
    stack.enter_scope(indent_level=4, line=2, parent_key="services")
    stack.enter_scope(indent_level=6, line=3, parent_key="database")
    stack.enter_scope(indent_level=8, line=4, parent_key="postgres")

    assert stack.get_scope_path() == "app.services.database.postgres", \
        f"Path should be 'app.services.database.postgres', got '{stack.get_scope_path()}'"

    # Exit to services level
    stack.scope_exit_to_level(target_depth=3)
    assert stack.get_scope_path() == "app.services", \
        f"After exit: path should be 'app.services', got '{stack.get_scope_path()}'"

    # Exit to root
    stack.scope_exit_to_level(target_depth=1)
    assert stack.get_scope_path() == "", \
        f"After exit to root: path should be empty, got '{stack.get_scope_path()}'"

    print("  ✓ Scope path is correctly updated after exit")


def test_indent_preservation_after_exit():
    """Test that indent level is correct after exit operations."""
    print("\n=== Test 14: Indent Preservation After Exit ===")

    stack = ScopeStack()

    stack.enter_scope(indent_level=2, line=1, parent_key="l1")
    stack.enter_scope(indent_level=6, line=2, parent_key="l2")  # Skip to 6
    stack.enter_scope(indent_level=12, line=3, parent_key="l3")

    assert stack.current_indent() == 12, f"Current indent should be 12, got {stack.current_indent()}"

    # Exit to level 2 (l1)
    stack.scope_exit_to_level(target_depth=2)
    assert stack.current_indent() == 2, f"After exit: indent should be 2, got {stack.current_indent()}"

    # Go deeper again
    stack.enter_scope(indent_level=4, line=4, parent_key="l1a")
    stack.enter_scope(indent_level=8, line=5, parent_key="l2a")

    # Exit one level
    stack.exit_one_level()
    assert stack.current_indent() == 4, f"After exit_one_level: indent should be 4, got {stack.current_indent()}"

    print("  ✓ Indent levels are correctly preserved after exit")


def test_sequence_scope_exit():
    """Test exit from sequence context scopes."""
    print("\n=== Test 15: Sequence Scope Exit ===")

    stack = ScopeStack()

    # Enter a sequence scope
    stack.enter_sequence_scope(indent_level=2, line=1)
    seq_scope = stack.current_scope()
    assert seq_scope.in_sequence_context, "Should be in sequence context"
    assert seq_scope.sequence_item_id is not None, "Should have sequence item ID"

    # Go deeper
    stack.enter_scope(indent_level=4, line=2, parent_key="item")

    # Exit back to sequence scope
    stack.exit_one_level()
    restored = stack.current_scope()
    assert restored.in_sequence_context, "Sequence context flag should be preserved"
    assert restored.sequence_item_id == seq_scope.sequence_item_id, "Sequence item ID should be preserved"

    print("  ✓ Sequence scope exit preserves sequence attributes")


def main():
    """Run all tests."""
    print("Testing Comprehensive Scope Exit Functionality\n")
    print("="*60)

    try:
        test_parent_scope_exit_basic()
        test_parent_exit_from_root()
        test_grandparent_scope_exit()
        test_exit_to_same_level()
        test_exit_to_invalid_depth_negative()
        test_exit_to_invalid_depth_too_large()
        test_exit_from_single_level_stack()
        test_state_preservation_keys()
        test_state_preservation_attributes()
        test_nested_structure_comprehensive()
        test_exit_one_level_multiple_times()
        test_mixed_exit_operations()
        test_scope_path_after_exit()
        test_indent_preservation_after_exit()
        test_sequence_scope_exit()

        print("\n" + "="*60)
        print("✓ All comprehensive scope exit tests passed!")
        print("="*60)
        return 0

    except AssertionError as e:
        print(f"\n✗ Test failed: {e}")
        import traceback
        traceback.print_exc()
        return 1
    except Exception as e:
        print(f"\n✗ Unexpected error: {e}")
        import traceback
        traceback.print_exc()
        return 1


if __name__ == '__main__':
    sys.exit(main())
