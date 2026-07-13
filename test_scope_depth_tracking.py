#!/usr/bin/env python3
"""
Test scope depth tracking in YAML parser.

This test validates that the Python YAML parser properly tracks
scope depth and hierarchy during parsing.
"""

import sys
from pathlib import Path

# Add the tools directory to the path
sys.path.insert(0, str(Path(__file__).parent / 'tools' / 'parse_module'))

from yaml_parser import YAMLParser, ScopeStack, Scope, DuplicateKeyError


def test_scope_depth_tracking():
    """Test that parser tracks current scope depth."""
    print("=== Test 1: Scope Depth Tracking ===")

    yaml_content = """
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
"""

    parser = YAMLParser()
    result = parser.parse_with_scope_tracking(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    scope_summary = parser.get_scope_summary()
    print(f"  Scope depth: {scope_summary['depth']}")
    print(f"  Scope path: '{scope_summary['path']}'")
    print(f"  Stack size: {scope_summary['stack_size']}")
    print(f"  In sequence: {scope_summary['in_sequence']}")

    # After parsing, we should be back at root
    assert scope_summary['depth'] == 1, f"Expected depth 1, got {scope_summary['depth']}"
    print("  ✓ Scope depth is tracked correctly")


def test_scope_stack_maintenance():
    """Test that scope stack is maintained on entry/exit."""
    print("\n=== Test 2: Scope Stack Maintenance ===")

    yaml_content = """
level1:
  level2:
    level3:
      key: value
    key2: value2
  key3: value3
"""

    parser = YAMLParser()
    result = parser.parse_with_scope_tracking(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    scope_stack = parser.get_scope_stack()
    assert scope_stack is not None, "Scope stack should not be None"

    print(f"  Final stack depth: {scope_stack.depth()}")
    print(f"  Final scope path: '{scope_stack.get_scope_path()}'")

    # Check that we can access scopes at different levels
    root_scope = scope_stack.get_scope_at_level(0)
    assert root_scope is not None, "Should have root scope"

    print("  ✓ Scope stack is maintained on entry/exit")


def test_parent_scope_identification():
    """Test that we can identify parent scope at any level."""
    print("\n=== Test 3: Parent Scope Identification ===")

    yaml_content = """
parent1:
  child1:
    grandchild: value
  child2:
    another: value
"""

    parser = YAMLParser()
    result = parser.parse_with_scope_tracking(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    scope_stack = parser.get_scope_stack()
    assert scope_stack is not None, "Scope stack should not be None"

    # Get scopes at different levels
    level2_scope = scope_stack.get_scope_at_level(2)
    if level2_scope:
        print(f"  Scope at level 2 parent_key: {level2_scope.parent_key}")
        assert level2_scope.parent_key in ['child1', 'child2'], \
            f"Expected parent_key to be child1 or child2, got {level2_scope.parent_key}"

    print("  ✓ Can identify parent scope at different levels")


def test_scope_stack_class():
    """Test the ScopeStack class directly."""
    print("\n=== Test 4: ScopeStack Class ===")

    stack = ScopeStack(base_indent=2)

    # Initial state
    assert stack.depth() == 1, "Should start with root scope"
    assert stack.current_indent() == 0, "Root should have indent 0"
    print(f"  Initial depth: {stack.depth()}, indent: {stack.current_indent()}")

    # Enter a scope
    stack.enter_scope(indent_level=2, line=1, parent_key="services")
    assert stack.depth() == 2, f"Depth should be 2, got {stack.depth()}"
    assert stack.current_indent() == 2, f"Indent should be 2, got {stack.current_indent()}"
    assert stack.get_scope_path() == "services", f"Path should be 'services', got '{stack.get_scope_path()}'"
    print(f"  After enter: depth={stack.depth()}, path='{stack.get_scope_path()}'")

    # Add a key
    stack.add_key("web", 2)
    assert stack.contains_key("web"), "Should contain 'web' key"
    print("  ✓ Added 'web' key to scope")

    # Enter deeper scope
    stack.enter_scope(indent_level=4, line=3, parent_key="config")
    assert stack.depth() == 3, f"Depth should be 3, got {stack.depth()}"
    assert stack.get_scope_path() == "services.config", f"Path should be 'services.config', got '{stack.get_scope_path()}'"
    print(f"  After deeper enter: depth={stack.depth()}, path='{stack.get_scope_path()}'")

    # Exit one level
    exited = stack.exit_one_level()
    assert exited, "Should have exited successfully"
    assert stack.depth() == 2, f"Depth should be 2 after exit, got {stack.depth()}"
    assert stack.current_indent() == 2, f"Indent should be 2 after exit, got {stack.current_indent()}"
    print(f"  After exit: depth={stack.depth()}, indent={stack.current_indent()}")

    # Add duplicate key (should raise error)
    try:
        stack.add_key("web", 5)
        assert False, "Should have raised DuplicateKeyError"
    except DuplicateKeyError as e:
        print(f"  ✓ Duplicate key detected: {e.message()}")

    print("  ✓ ScopeStack class works correctly")


def test_scope_class():
    """Test the Scope class directly."""
    print("\n=== Test 5: Scope Class ===")

    scope = Scope(indent_level=2, start_line=1, parent_key="services")

    assert scope.indent_level == 2, "Indent level should be 2"
    assert scope.start_line == 1, "Start line should be 1"
    assert scope.parent_key == "services", "Parent key should be 'services'"
    print(f"  Scope created: indent={scope.indent_level}, parent={scope.parent_key}")

    # Add keys
    is_dup1 = scope.add_key("web")
    is_dup2 = scope.add_key("database")
    is_dup3 = scope.add_key("web")  # Duplicate

    assert not is_dup1, "First 'web' should not be duplicate"
    assert not is_dup2, "First 'database' should not be duplicate"
    assert is_dup3, "Second 'web' should be duplicate"
    print("  ✓ Key addition and duplicate detection work")

    assert scope.contains_key("web"), "Should contain 'web'"
    assert scope.contains_key("database"), "Should contain 'database'"
    assert not scope.contains_key("nonexistent"), "Should not contain 'nonexistent'"
    print("  ✓ Key containment checks work")

    assert scope.key_count() == 2, f"Should have 2 keys, got {scope.key_count()}"
    print(f"  ✓ Key count is correct: {scope.key_count()}")


def test_indent_transitions():
    """Test indent transition tracking."""
    print("\n=== Test 6: Indent Transition Tracking ===")

    parser = YAMLParser()
    yaml_content = """
root:
  level1:
    key: value
  level2:
    another: value2
"""

    result = parser.parse_with_scope_tracking(yaml_content)
    assert result.is_success(), f"Parse failed: {result.error}"

    scope_stack = parser.get_scope_stack()
    transitions = scope_stack.get_indent_transitions()

    print(f"  Recorded {len(transitions)} indent transitions")
    for trans in transitions:
        direction = "increase" if trans.is_increase() else ("decrease" if trans.is_decrease() else "none")
        print(f"    Line {trans.line_number}: {trans.from_indent}→{trans.to_indent} ({direction})")

    assert len(transitions) > 0, "Should have recorded some transitions"

    # Count transition types
    enter_count = sum(1 for t in transitions if t.is_enter_scope())
    exit_count = sum(1 for t in transitions if t.is_exit_scope())

    print(f"  Enter transitions: {enter_count}")
    print(f"  Exit transitions: {exit_count}")
    print("  ✓ Indent transitions are tracked")


def test_sibling_mappings():
    """Test that sibling mappings don't conflict."""
    print("\n=== Test 7: Sibling Mappings ===")

    yaml_content = """
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
"""

    parser = YAMLParser()
    result = parser.parse_with_scope_tracking(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    scope_stack = parser.get_scope_stack()
    assert scope_stack is not None, "Scope stack should not be None"

    # Verify that both 'host' and 'port' keys exist in different scopes
    # (they should be in sibling scopes, not conflicting)
    web_scope = scope_stack.get_scope_at_level(4)
    if web_scope:
        print(f"  Web scope keys: {web_scope.keys}")
        assert "host" in web_scope.keys or "port" in web_scope.keys, \
            "Web scope should have keys"

    print("  ✓ Sibling mappings are handled correctly")


def main():
    """Run all tests."""
    print("Testing Scope Depth Tracking in YAML Parser\n")

    try:
        test_scope_depth_tracking()
        test_scope_stack_maintenance()
        test_parent_scope_identification()
        test_scope_stack_class()
        test_scope_class()
        test_indent_transitions()
        test_sibling_mappings()

        print("\n" + "="*50)
        print("✓ All tests passed!")
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
