#!/usr/bin/env python3
"""
Test indent transition state machine directly (without PyYAML dependency).

This test verifies the indent transition state machine logic works correctly
by testing the ScopeStack class directly.
"""

import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent / 'tools' / 'parse_module'))

from yaml_parser import ScopeStack, IndentTransition, IndentTransitionType


def test_transition_classification():
    """Test that transitions are correctly classified."""
    print("=== Test 1: Transition Classification ===")

    stack = ScopeStack(base_indent=2)

    # Test SAME_LEVEL transition
    stack.record_indent_transition(line_number=1, new_indent=0, has_key=True, raw_line="key: value")
    assert len(stack.indent_transitions) == 0, "No transition should be recorded for same indent"

    # Test ENTER_SCOPE transition (0 -> 2)
    stack.record_indent_transition(line_number=2, new_indent=2, has_key=True, raw_line="  child: value")
    assert len(stack.indent_transitions) == 1, "Should have recorded one transition"
    trans = stack.indent_transitions[0]
    assert trans.transition_type == IndentTransitionType.ENTER_SCOPE, \
        f"Expected ENTER_SCOPE, got {trans.transition_type}"
    assert trans.from_indent == 0, f"Expected from_indent 0, got {trans.from_indent}"
    assert trans.to_indent == 2, f"Expected to_indent 2, got {trans.to_indent}"
    print(f"  ✓ ENTER_SCOPE: 0→2 at line 2")

    # Test SAME_LEVEL within new indent
    stack.record_indent_transition(line_number=3, new_indent=2, has_key=True, raw_line="  sibling: value")
    assert len(stack.indent_transitions) == 1, "No transition for same level"

    # Test EXIT_SCOPE transition (2 -> 0)
    stack.record_indent_transition(line_number=4, new_indent=0, has_key=True, raw_line="root2: value")
    assert len(stack.indent_transitions) == 2, "Should have recorded two transitions"
    trans = stack.indent_transitions[1]
    assert trans.transition_type == IndentTransitionType.EXIT_SCOPE, \
        f"Expected EXIT_SCOPE, got {trans.transition_type}"
    assert trans.from_indent == 2, f"Expected from_indent 2, got {trans.from_indent}"
    assert trans.to_indent == 0, f"Expected to_indent 0, got {trans.to_indent}"
    print(f"  ✓ EXIT_SCOPE: 2→0 at line 4")

    # Test multiple level increases (0 -> 2 -> 4 -> 6)
    stack.clear_indent_transitions()
    stack.record_indent_transition(1, 0, True, "root:")
    stack.record_indent_transition(2, 2, True, "  level1:")
    stack.record_indent_transition(3, 4, True, "    level2:")
    stack.record_indent_transition(4, 6, True, "      level3:")

    assert len(stack.indent_transitions) == 3, f"Expected 3 transitions, got {len(stack.indent_transitions)}"
    for trans in stack.indent_transitions:
        assert trans.is_enter_scope(), "All should be ENTER_SCOPE"
    print(f"  ✓ Multi-level increases: 0→2→4→6")

    # Test multiple level decreases (6 -> 4 -> 2 -> 0)
    stack.clear_indent_transitions()
    stack.last_indent = 6
    stack.record_indent_transition(5, 4, True, "    back_to_level2:")
    stack.record_indent_transition(6, 2, True, "  back_to_level1:")
    stack.record_indent_transition(7, 0, True, "back_to_root:")

    assert len(stack.indent_transitions) == 3, f"Expected 3 transitions, got {len(stack.indent_transitions)}"
    for trans in stack.indent_transitions:
        assert trans.is_exit_scope(), "All should be EXIT_SCOPE"
    print(f"  ✓ Multi-level decreases: 6→4→2→0")

    print("  ✓ All transitions classified correctly")


def test_indent_transition_dataclass():
    """Test that IndentTransition dataclass stores all information."""
    print("\n=== Test 2: IndentTransition Dataclass ===")

    stack = ScopeStack(base_indent=2)
    stack.record_indent_transition(
        line_number=5,
        new_indent=4,
        has_key=True,
        raw_line="    key: value"
    )

    assert len(stack.indent_transitions) == 1
    trans = stack.indent_transitions[0]

    assert trans.line_number == 5
    assert trans.from_indent == 0
    assert trans.to_indent == 4
    assert trans.has_key is True
    assert trans.raw_line == "    key: value"
    assert trans.transition_type == IndentTransitionType.ENTER_SCOPE

    # Test helper methods
    assert trans.is_increase() is True
    assert trans.is_decrease() is False
    assert trans.is_enter_scope() is True
    assert trans.is_exit_scope() is False
    assert trans.is_same_level() is False
    assert trans.is_without_key() is False

    print("  ✓ IndentTransition stores all fields correctly")
    print("  ✓ Helper methods work correctly")


def test_transition_history_maintenance():
    """Test that transition history is maintained."""
    print("\n=== Test 3: Transition History Maintenance ===")

    stack = ScopeStack(base_indent=2)

    # Record a series of transitions
    transitions_data = [
        (1, 0, True, "root:"),
        (2, 2, True, "  level1:"),
        (3, 2, True, "  sibling:"),
        (4, 4, True, "    level2:"),
        (5, 2, True, "  back_to_level1:"),
        (6, 0, True, "back_to_root:"),
    ]

    for line_num, indent, has_key, line in transitions_data:
        stack.record_indent_transition(line_num, indent, has_key, line)

    # Get all transitions
    all_transitions = stack.get_indent_transitions()

    # Should have recorded transitions for lines 2, 4, 5, 6
    # (lines 1 and 3 are same-level as previous, so not recorded)
    expected_transitions = [
        (2, 0, 2, IndentTransitionType.ENTER_SCOPE),
        (4, 2, 4, IndentTransitionType.ENTER_SCOPE),
        (5, 4, 2, IndentTransitionType.EXIT_SCOPE),
        (6, 2, 0, IndentTransitionType.EXIT_SCOPE),
    ]

    assert len(all_transitions) == len(expected_transitions), \
        f"Expected {len(expected_transitions)} transitions, got {len(all_transitions)}"

    for i, (line_num, from_ind, to_ind, trans_type) in enumerate(expected_transitions):
        trans = all_transitions[i]
        assert trans.line_number == line_num
        assert trans.from_indent == from_ind
        assert trans.to_indent == to_ind
        assert trans.transition_type == trans_type

    print(f"  ✓ Recorded {len(all_transitions)} transitions")
    print("  ✓ Transition history is maintained correctly")

    # Test clearing
    stack.clear_indent_transitions()
    assert len(stack.get_indent_transitions()) == 0
    print("  ✓ Transition history can be cleared")


def test_state_machine_with_complex_scenario():
    """Test state machine with a complex indent scenario."""
    print("\n=== Test 4: Complex State Machine Scenario ===")

    stack = ScopeStack(base_indent=2)

    # Simulate parsing a complex YAML structure
    yaml_lines = [
        (1, 0, "services:", True),              # Line 1: root level (no transition)
        (2, 2, "  web:", True),                  # Line 2: enter scope (0->2)
        (3, 4, "    host: localhost", True),     # Line 3: enter scope (2->4)
        (4, 4, "    port: 8080", True),          # Line 4: same level (no transition)
        (5, 2, "  database:", True),             # Line 5: exit scope (4->2)
        (6, 4, "    host: db.example.com", True), # Line 6: enter scope (2->4)
        (7, 0, "config:", True),                 # Line 7: exit scope (4->0)
        (8, 2, "  debug: true", True),           # Line 8: enter scope (0->2)
        (9, 0, "version: 1.0", True),            # Line 9: exit scope (2->0)
    ]

    for line_num, indent, line, has_key in yaml_lines:
        stack.record_indent_transition(line_num, indent, has_key, line)

    transitions = stack.get_indent_transitions()

    # Expected transitions:
    # Line 2: 0->2 ENTER
    # Line 3: 2->4 ENTER
    # Line 5: 4->2 EXIT
    # Line 6: 2->4 ENTER
    # Line 7: 4->0 EXIT
    # Line 8: 0->2 ENTER
    # Line 9: 2->0 EXIT

    expected_count = 7
    assert len(transitions) == expected_count, \
        f"Expected {expected_count} transitions, got {len(transitions)}"

    # Verify specific transitions
    assert transitions[0].line_number == 2
    assert transitions[0].is_enter_scope()
    assert transitions[0].from_indent == 0
    assert transitions[0].to_indent == 2

    assert transitions[2].line_number == 5
    assert transitions[2].is_exit_scope()
    assert transitions[2].from_indent == 4
    assert transitions[2].to_indent == 2

    assert transitions[5].line_number == 8
    assert transitions[5].is_enter_scope()
    assert transitions[5].from_indent == 0
    assert transitions[5].to_indent == 2

    # Count enter vs exit
    enter_count = sum(1 for t in transitions if t.is_enter_scope())
    exit_count = sum(1 for t in transitions if t.is_exit_scope())

    assert enter_count == 4, f"Expected 4 ENTER_SCOPE, got {enter_count}"
    assert exit_count == 3, f"Expected 3 EXIT_SCOPE, got {exit_count}"

    print(f"  ✓ Complex scenario: {len(transitions)} transitions recorded")
    print(f"  ✓ Enter scopes: {enter_count}, Exit scopes: {exit_count}")


def test_transition_without_key():
    """Test transitions on lines without keys."""
    print("\n=== Test 5: Transitions Without Keys ===")

    stack = ScopeStack(base_indent=2)

    # Record transition without a key (indent-only line)
    stack.record_indent_transition(
        line_number=2,
        new_indent=4,
        has_key=False,
        raw_line="    continuation text"
    )

    assert len(stack.indent_transitions) == 1
    trans = stack.indent_transitions[0]

    assert trans.has_key is False
    assert trans.is_without_key() is True
    print("  ✓ Transitions without keys are tracked")


def test_transition_types_enum():
    """Test that IndentTransitionType enum values are correct."""
    print("\n=== Test 6: IndentTransitionType Enum ===")

    assert IndentTransitionType.ENTER_SCOPE.value == "enter-scope"
    assert IndentTransitionType.EXIT_SCOPE.value == "exit-scope"
    assert IndentTransitionType.SAME_LEVEL.value == "same-level"

    print("  ✓ IndentTransitionType enum values are correct")


def main():
    """Run all tests."""
    print("\n" + "=" * 70)
    print("Indent Transition State Machine Tests")
    print("=" * 70 + "\n")

    try:
        test_transition_types_enum()
        test_transition_classification()
        test_indent_transition_dataclass()
        test_transition_history_maintenance()
        test_state_machine_with_complex_scenario()
        test_transition_without_key()

        print("\n" + "=" * 70)
        print("✓ All indent transition state machine tests passed!")
        print("=" * 70)
        print("\nAcceptance Criteria Verified:")
        print("  ✓ All indent transitions are tracked")
        print("  ✓ State machine handles increase/decrease/no-change")
        print("  ✓ Scope history is maintained")
        print("  ✓ Transitions are correctly classified")

        return True

    except AssertionError as e:
        print(f"\n✗ Test failed: {e}")
        import traceback
        traceback.print_exc()
        return False
    except Exception as e:
        print(f"\n✗ Unexpected error: {e}")
        import traceback
        traceback.print_exc()
        return False


if __name__ == '__main__':
    success = main()
    sys.exit(0 if success else 1)
