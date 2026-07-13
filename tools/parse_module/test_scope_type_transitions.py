"""
Test type-specific scope transition handling (pytest-free).
"""

import sys
from pathlib import Path

# Add parent directory to path
sys.path.insert(0, str(Path(__file__).parent))

from yaml_parser import YAMLParser, ParseResult, ParseStatus, LineClassification


def run_test(test_name, test_func):
    """Run a single test and print result."""
    try:
        test_func()
        print(f"✓ {test_name}")
        return True
    except AssertionError as e:
        print(f"✗ {test_name}: {e}")
        return False
    except Exception as e:
        print(f"✗ {test_name}: Unexpected error: {e}")
        import traceback
        traceback.print_exc()
        return False


def test_key_bearing_line_creates_new_scope_on_indent_increase():
    """Test that key-bearing lines create new scopes when indent increases."""
    parser = YAMLParser()
    yaml_content = """
parent:
  child: value
"""
    result = parser.parse_with_scope_tracking(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"

    final_depth = parser.get_scope_depth()
    assert final_depth >= 2, f"Expected depth >= 2, got {final_depth}"

    transitions = parser.get_scope_stack().get_indent_transitions()
    assert len(transitions) > 0, "Should have recorded transitions"


def test_indent_only_line_does_not_create_new_scope():
    """Test that indent-only lines do NOT create new scopes."""
    parser = YAMLParser()
    yaml_content = """
parent:
  child: value
    continuation
    another_continuation
  sibling: another_value
"""
    result = parser.parse_with_scope_tracking(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"

    transitions = parser.get_scope_stack().get_indent_transitions()

    # Find transitions at continuation lines
    continuation_transitions = [t for t in transitions if 'continuation' in t.raw_line]
    assert len(continuation_transitions) > 0, "Should have continuation transitions"

    # These should be classified as INDENT_ONLY
    for trans in continuation_transitions:
        if 'continuation' in trans.raw_line:
            assert trans.line_classification.value == 'indent-only', \
                f"Expected indent-only, got {trans.line_classification.value}"


def test_multiline_string_does_not_create_scope():
    """Test that multiline string continuations do not create new scopes."""
    parser = YAMLParser()
    yaml_content = """
description: |
  Line 1
    Line 2 with extra indent
  Line 3
"""
    result = parser.parse_with_scope_tracking(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"

    transitions = parser.get_scope_stack().get_indent_transitions()

    # Lines inside the multiline block should be INDENT_ONLY
    multiline_transitions = [t for t in transitions if 'Line' in t.raw_line]
    assert len(multiline_transitions) > 0, "Should have multiline transitions"

    for trans in multiline_transitions:
        # Multiline content lines are indent-only, not key-bearing
        assert trans.line_classification.value == 'indent-only', \
            f"Multiline content should be indent-only, got {trans.line_classification.value}"


def test_complex_nested_structure_scope_handling():
    """Test scope handling for complex nested YAML structures."""
    parser = YAMLParser()
    yaml_content = """
level1:
  level2:
    level3: value
    level3_sibling: another
  level2_sibling: value2
"""
    result = parser.parse_with_scope_tracking(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"

    transitions = parser.get_scope_stack().get_indent_transitions()

    # Count enter/exit scope transitions
    enter_scopes = [t for t in transitions if t.is_enter_scope()]
    exit_scopes = [t for t in transitions if t.is_exit_scope()]

    # Should have multiple scope transitions
    assert len(enter_scopes) > 0, "Should have enter scope transitions"
    assert len(exit_scopes) > 0, "Should have exit scope transitions"

    # Check that key-bearing lines trigger scope transitions
    key_bearing_enters = [t for t in enter_scopes if t.line_classification.value == 'key-bearing']
    assert len(key_bearing_enters) > 0, "Should have key-bearing enter scope transitions"


def test_mixed_key_bearing_and_indent_only_lines():
    """Test mixed scenarios with both key-bearing and indent-only lines."""
    parser = YAMLParser()
    yaml_content = """
service:
  name: my-service
  description: |
    This is a service
      with nested description
    and multiple lines
  config:
    enabled: true
    timeout: 30
"""
    result = parser.parse_with_scope_tracking(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"

    transitions = parser.get_scope_stack().get_indent_transitions()

    # Verify we have both types of lines classified
    key_bearing = [t for t in transitions if t.line_classification.value == 'key-bearing']
    indent_only = [t for t in transitions if t.line_classification.value == 'indent-only']

    assert len(key_bearing) > 0, "Should have key-bearing transitions"
    assert len(indent_only) > 0, "Should have indent-only transitions"

    # Key-bearing transitions should create scopes
    key_enters = [t for t in key_bearing if t.is_enter_scope()]
    assert len(key_enters) > 0, "Key-bearing lines should create scopes"


def test_scope_transition_classification_accuracy():
    """Test that scope transitions are classified correctly by type."""
    parser = YAMLParser()
    yaml_content = """
outer:
  inner_key: inner_value
  - list_item_1
  - list_item_2
  another_key: another_value
"""
    result = parser.parse_with_scope_tracking(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"

    transitions = parser.get_scope_stack().get_indent_transitions()

    # Verify each transition has proper classification
    for trans in transitions:
        assert trans.line_classification is not None, "Line classification should not be None"
        assert trans.transition_type is not None, "Transition type should not be None"

        # If it's a key-bearing line, it should have has_key=True
        if trans.line_classification.value == 'key-bearing':
            assert trans.has_key is True, "Key-bearing lines should have has_key=True"


def test_line_type_classification_methods():
    """Test parser methods for checking current line type."""
    parser = YAMLParser()
    yaml_content = """
key: value
  indent_only_content
"""
    result = parser.parse_with_scope_tracking(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"

    # Parser should have line type tracking methods
    assert hasattr(parser, 'is_on_key_bearing_line'), "Should have is_on_key_bearing_line method"
    assert hasattr(parser, 'is_on_indent_only_line'), "Should have is_on_indent_only_line method"
    assert hasattr(parser, 'is_on_empty_line'), "Should have is_on_empty_line method"


def test_nested_sequence_with_parent_mapping():
    """Test nested sequence items within parent mappings."""
    parser = YAMLParser()
    yaml_content = """
services:
  web:
    - name: frontend
      port: 80
    - name: backend
      port: 8080
  database:
    - name: postgres
      port: 5432
"""
    result = parser.parse_with_scope_tracking(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"

    transitions = parser.get_scope_stack().get_indent_transitions()

    # Should have key-bearing transitions for 'web', 'database', 'name', 'port'
    key_bearing = [t for t in transitions if t.line_classification.value == 'key-bearing']
    assert len(key_bearing) > 0, "Should have key-bearing transitions in nested sequences"


def test_nested_scope_transitions_with_same_indent():
    """Test that same-level key-bearing lines don't create new scopes."""
    parser = YAMLParser()
    yaml_content = """
key1: value1
key2: value2
key3: value3
"""
    result = parser.parse_with_scope_tracking(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"

    final_depth = parser.get_scope_depth()
    # All keys at same level should not increase depth beyond root + current scope
    assert final_depth <= 2, f"Same-level keys should not create deep nesting, got depth {final_depth}"


def main():
    """Run all tests."""
    print("Running Type-Specific Scope Transition Tests")
    print("=" * 60)

    tests = [
        ("Key-bearing line creates new scope on indent increase",
         test_key_bearing_line_creates_new_scope_on_indent_increase),
        ("Indent-only line does not create new scope",
         test_indent_only_line_does_not_create_new_scope),
        ("Multiline string does not create scope",
         test_multiline_string_does_not_create_scope),
        ("Complex nested structure scope handling",
         test_complex_nested_structure_scope_handling),
        ("Mixed key-bearing and indent-only lines",
         test_mixed_key_bearing_and_indent_only_lines),
        ("Scope transition classification accuracy",
         test_scope_transition_classification_accuracy),
        ("Line type classification methods exist",
         test_line_type_classification_methods),
        ("Nested sequence with parent mapping",
         test_nested_sequence_with_parent_mapping),
        ("Nested scope transitions with same indent",
         test_nested_scope_transitions_with_same_indent),
    ]

    passed = 0
    failed = 0

    for test_name, test_func in tests:
        if run_test(test_name, test_func):
            passed += 1
        else:
            failed += 1

    print("=" * 60)
    print(f"Results: {passed} passed, {failed} failed")

    if failed == 0:
        print("✓ All tests passed!")
        return 0
    else:
        print(f"✗ {failed} test(s) failed")
        return 1


if __name__ == '__main__':
    sys.exit(main())
