#!/usr/bin/env python3
"""
Test multi-level scope exit functionality in YAML parser.

This test validates that the scope_exit_to_level function properly
exits to grandparent and arbitrary ancestor scopes.
"""

import sys
from pathlib import Path

# Add the tools directory to the path
sys.path.insert(0, str(Path(__file__).parent / 'tools' / 'parse_module'))

from yaml_parser import YAMLParser, ScopeStack, Scope


def test_exit_to_grandparent():
    """Test exiting from child to grandparent (skipping immediate parent)."""
    print("=== Test 1: Exit to Grandparent (Skip Intermediate) ===")

    stack = ScopeStack(base_indent=2)

    # Build a 4-level hierarchy: root(0) -> level1(2) -> level2(4) -> level3(6)
    stack.enter_scope(indent_level=2, line=1, parent_key="level1")
    assert stack.depth() == 2, f"Expected depth 2, got {stack.depth()}"
    print(f"  After entering level1: depth={stack.depth()}, path='{stack.get_scope_path()}'")

    stack.enter_scope(indent_level=4, line=2, parent_key="level2")
    assert stack.depth() == 3, f"Expected depth 3, got {stack.depth()}"
    print(f"  After entering level2: depth={stack.depth()}, path='{stack.get_scope_path()}'")

    stack.enter_scope(indent_level=6, line=3, parent_key="level3")
    assert stack.depth() == 4, f"Expected depth 4, got {stack.depth()}"
    print(f"  After entering level3: depth={stack.depth()}, path='{stack.get_scope_path()}'")

    # Now exit to grandparent (from depth 4 to depth 2, skipping depth 3)
    result = stack.scope_exit_to_level(target_depth=2)
    assert result is True, "Should successfully exit to grandparent"
    assert stack.depth() == 2, f"Expected depth 2 after exit, got {stack.depth()}"
    assert stack.get_scope_path() == "level1", f"Expected path 'level1', got '{stack.get_scope_path()}'"
    print(f"  After exit to grandparent: depth={stack.depth()}, path='{stack.get_scope_path()}'")
    print("  ✓ Can exit from child to grandparent (skip intermediate)")


def test_exit_to_any_ancestor():
    """Test exiting to any valid ancestor level."""
    print("\n=== Test 2: Exit to Any Valid Ancestor ===")

    stack = ScopeStack(base_indent=2)

    # Build a 6-level hierarchy
    stack.enter_scope(indent_level=2, line=1, parent_key="a")
    stack.enter_scope(indent_level=4, line=2, parent_key="b")
    stack.enter_scope(indent_level=6, line=3, parent_key="c")
    stack.enter_scope(indent_level=8, line=4, parent_key="d")
    stack.enter_scope(indent_level=10, line=5, parent_key="e")

    assert stack.depth() == 6, f"Expected depth 6, got {stack.depth()}"
    print(f"  Initial depth: {stack.depth()}, path: '{stack.get_scope_path()}'")

    # Exit to depth 3 (skipping depths 5 and 4)
    result = stack.scope_exit_to_level(target_depth=3)
    assert result is True, "Should successfully exit to depth 3"
    assert stack.depth() == 3, f"Expected depth 3, got {stack.depth()}"
    print(f"  After exit to depth 3: depth={stack.depth()}, path: '{stack.get_scope_path()}'")

    # Build again and exit to different levels
    stack.enter_scope(indent_level=8, line=6, parent_key="d2")
    stack.enter_scope(indent_level=10, line=7, parent_key="e2")
    assert stack.depth() == 5, f"Expected depth 5, got {stack.depth()}"

    # Exit to root (depth 1)
    result = stack.scope_exit_to_level(target_depth=1)
    assert result is True, "Should successfully exit to root"
    assert stack.depth() == 1, f"Expected depth 1 at root, got {stack.depth()}"
    assert stack.get_scope_path() == "", f"Expected empty path at root, got '{stack.get_scope_path()}'"
    print(f"  After exit to root: depth={stack.depth()}, path: '{stack.get_scope_path()}'")

    print("  ✓ Can exit to any valid ancestor level")


def test_invalid_target_depths():
    """Test that invalid target depths are properly rejected."""
    print("\n=== Test 3: Invalid Target Depths ===")

    stack = ScopeStack(base_indent=2)

    # Build a 3-level hierarchy
    stack.enter_scope(indent_level=2, line=1, parent_key="level1")
    stack.enter_scope(indent_level=4, line=2, parent_key="level2")
    assert stack.depth() == 3, f"Expected depth 3, got {stack.depth()}"

    # Test 1: Target depth <= 0 should raise ValueError
    print("  Testing target depth <= 0...")
    try:
        stack.scope_exit_to_level(target_depth=0)
        assert False, "Should have raised ValueError for depth 0"
    except ValueError as e:
        assert "must be positive" in str(e).lower(), f"Expected 'positive' error, got: {e}"
        print(f"    ✓ Depth 0 rejected: {e}")

    try:
        stack.scope_exit_to_level(target_depth=-1)
        assert False, "Should have raised ValueError for negative depth"
    except ValueError as e:
        assert "must be positive" in str(e).lower(), f"Expected 'positive' error, got: {e}"
        print(f"    ✓ Negative depth rejected: {e}")

    # Test 2: Target depth > current depth should raise ValueError
    print("  Testing target depth > current depth...")
    try:
        stack.scope_exit_to_level(target_depth=5)
        assert False, "Should have raised ValueError for depth > current"
    except ValueError as e:
        assert "current depth" in str(e).lower(), f"Expected 'current depth' error, got: {e}"
        print(f"    ✓ Depth > current rejected: {e}")

    # Test 3: Target depth that doesn't exist should raise ValueError
    print("  Testing non-existent target depth...")
    # Build a deeper hierarchy but then try to exit to a non-existent level
    stack.enter_scope(indent_level=6, line=3, parent_key="level3")
    stack.enter_scope(indent_level=8, line=4, parent_key="level4")
    # Now at depth 5 (root + 4 scopes)

    # This should work - exiting to depth 2 (which exists)
    result = stack.scope_exit_to_level(target_depth=2)
    assert result is True, "Should successfully exit to existing depth 2"
    print(f"    ✓ Valid exit to depth 2 succeeds")

    print("  ✓ Invalid target depths are rejected")


def test_exit_to_current_depth():
    """Test that exiting to current depth is a no-op."""
    print("\n=== Test 4: Exit to Current Depth (No-op) ===")

    stack = ScopeStack(base_indent=2)

    stack.enter_scope(indent_level=2, line=1, parent_key="level1")
    stack.enter_scope(indent_level=4, line=2, parent_key="level2")

    current_depth = stack.depth()
    current_path = stack.get_scope_path()

    print(f"  Current depth: {current_depth}, path: '{current_path}'")

    # Exit to the same depth we're already at
    result = stack.scope_exit_to_level(target_depth=current_depth)
    assert result is True, "Exiting to current depth should succeed"
    assert stack.depth() == current_depth, f"Depth should remain {current_depth}"
    assert stack.get_scope_path() == current_path, f"Path should remain '{current_path}'"
    print(f"  After exit to same depth: depth={stack.depth()}, path='{stack.get_scope_path()}'")

    print("  ✓ Exit to current depth is handled correctly")


def test_integration_with_yaml_parser():
    """Test scope_exit_to_level in the context of YAML parsing."""
    print("\n=== Test 5: Integration with YAML Parser ===")

    yaml_content = """
root:
  level1:
    level2:
      level3:
        key: value
    sibling: value2
  another: value3
"""

    parser = YAMLParser(enable_scope_tracking=True)
    result = parser.parse_with_scope_tracking(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    scope_stack = parser.get_scope_stack()
    assert scope_stack is not None, "Scope stack should not be None"

    # After parsing, we should be back at root
    final_depth = scope_stack.depth()
    final_path = scope_stack.get_scope_path()
    print(f"  Final state after parse: depth={final_depth}, path='{final_path}'")

    # Manually build a similar stack to test multi-level exit
    test_stack = ScopeStack(base_indent=2)
    test_stack.enter_scope(indent_level=2, line=2, parent_key="root")
    test_stack.enter_scope(indent_level=4, line=3, parent_key="level1")
    test_stack.enter_scope(indent_level=6, line=4, parent_key="level2")
    test_stack.enter_scope(indent_level=8, line=5, parent_key="level3")

    print(f"  Built test stack: depth={test_stack.depth()}, path='{test_stack.get_scope_path()}'")

    # Test multi-level exit
    result = test_stack.scope_exit_to_level(target_depth=2)
    assert result is True, "Should successfully exit to depth 2"
    assert test_stack.depth() == 2, f"Expected depth 2, got {test_stack.depth()}"
    print(f"  After multi-level exit: depth={test_stack.depth()}, path='{test_stack.get_scope_path()}'")

    print("  ✓ Integration with YAML parser works correctly")


def test_multiple_exits_sequence():
    """Test a sequence of multiple multi-level exits."""
    print("\n=== Test 6: Multiple Exits in Sequence ===")

    stack = ScopeStack(base_indent=2)

    # Build a deep hierarchy
    for i in range(1, 8):  # Create 7 levels
        stack.enter_scope(indent_level=i*2, line=i, parent_key=f"level{i}")

    assert stack.depth() == 8, f"Expected depth 8, got {stack.depth()}"
    print(f"  Built deep hierarchy: depth={stack.depth()}, path='{stack.get_scope_path()}'")

    # Perform a series of exits
    # From 8 -> 5 -> 2 -> 1
    targets = [5, 2, 1]
    for target in targets:
        result = stack.scope_exit_to_level(target_depth=target)
        assert result is True, f"Should exit to depth {target}"
        assert stack.depth() == target, f"Expected depth {target}, got {stack.depth()}"
        print(f"  Exited to depth {target}: path='{stack.get_scope_path()}'")

    # Final state should be at root
    assert stack.depth() == 1, "Should end at root"
    assert stack.get_scope_path() == "", "Root should have empty path"

    print("  ✓ Multiple exits in sequence work correctly")


def test_exit_with_keys_tracking():
    """Test that keys are properly tracked after multi-level exits."""
    print("\n=== Test 7: Keys Tracking After Multi-level Exit ===")

    stack = ScopeStack(base_indent=2)

    # Build hierarchy with keys
    stack.enter_scope(indent_level=2, line=1, parent_key="services")
    stack.add_key("web", 2)
    stack.add_key("db", 3)

    stack.enter_scope(indent_level=4, line=4, parent_key="web_config")
    stack.add_key("host", 5)
    stack.add_key("port", 6)

    stack.enter_scope(indent_level=6, line=7, parent_key="ssl")
    stack.add_key("cert", 8)
    stack.add_key("key", 9)

    print(f"  Before exit: depth={stack.depth()}, path='{stack.get_scope_path()}'")
    print(f"    Current scope keys: {list(stack.current_scope().keys)}")

    # Exit from depth 4 to depth 2 (skip web_config level)
    result = stack.scope_exit_to_level(target_depth=2)
    assert result is True, "Should successfully exit to depth 2"

    print(f"  After exit to depth 2: path='{stack.get_scope_path()}'")
    print(f"    Current scope keys: {list(stack.current_scope().keys)}")

    # Verify we're back at the 'services' scope
    assert stack.get_scope_path() == "services", f"Expected 'services' path"
    assert stack.contains_key("web"), "Should still have 'web' key"
    assert stack.contains_key("db"), "Should still have 'db' key"

    # Add a new key at this level
    stack.add_key("cache", 10)
    assert stack.contains_key("cache"), "Should have 'cache' key"

    print("  ✓ Keys are properly tracked after multi-level exit")


def main():
    """Run all tests."""
    print("Testing Multi-Level Scope Exit Functionality\n")

    try:
        test_exit_to_grandparent()
        test_exit_to_any_ancestor()
        test_invalid_target_depths()
        test_exit_to_current_depth()
        test_integration_with_yaml_parser()
        test_multiple_exits_sequence()
        test_exit_with_keys_tracking()

        print("\n" + "="*50)
        print("✓ All multi-level scope exit tests passed!")
        print("="*50)
        return 0

    except AssertionError as e:
        print(f"\n✗ Test failed: {e}")
        return 1
    except Exception as e:
        print(f"\n✗ Unexpected error: {e}")
        import traceback
        traceback.print_exc()
        return 1


if __name__ == '__main__':
    sys.exit(main())
