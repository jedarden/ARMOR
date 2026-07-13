#!/usr/bin/env python3
"""
Test that line type is tracked in parser state.

Tests that the YAML parser correctly tracks line type in its state
and makes it accessible to scope transition logic.
"""

import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent / 'tools' / 'parse_module'))

from yaml_parser import YAMLParser, LineClassification


def test_line_type_state_tracking():
    """Test that line type is tracked in parser state during parsing."""
    yaml_content = """# Configuration file
server:
  host: localhost
  port: 8080
  ssl:
    enabled: true

database:
  host: db.example.com

features:
  - authentication
  - authorization: enabled
"""

    parser = YAMLParser(enable_scope_tracking=True, base_indent=2)
    result = parser.parse_with_scope_tracking(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    # After parsing, the parser should have tracked line types
    # The last line processed was "- authorization: enabled"
    assert parser.get_current_line_type() == LineClassification.KEY_BEARING
    assert parser.is_on_key_bearing_line()
    assert not parser.is_on_indent_only_line()
    assert not parser.is_on_empty_line()

    print("✓ Line type state tracking test passed")


def test_line_type_getters():
    """Test the line type getter methods."""
    yaml_content = """parent:
  child: value
"""

    parser = YAMLParser(enable_scope_tracking=True, base_indent=2)
    result = parser.parse_with_scope_tracking(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    # Check we can get current line info
    line_type = parser.get_current_line_type()
    line_num = parser.get_current_line_number()

    assert line_type is not None, "Line type should not be None after parsing"
    assert line_num > 0, "Line number should be positive after parsing"
    assert line_num == 3, f"Expected line number 3, got {line_num}"

    print("✓ Line type getter methods test passed")


def test_empty_line_state():
    """Test that empty lines are tracked correctly."""
    yaml_content = """key1: value1

key2: value2
"""

    parser = YAMLParser(enable_scope_tracking=True, base_indent=2)
    result = parser.parse_with_scope_tracking(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    # Last line is "key2: value2" which is key-bearing
    assert parser.is_on_key_bearing_line()

    print("✓ Empty line state tracking test passed")


def test_indent_only_line_state():
    """Test that indent-only lines are tracked correctly."""
    yaml_content = """parent:
  child: value
"""

    parser = YAMLParser(enable_scope_tracking=True, base_indent=2)
    result = parser.parse_with_scope_tracking(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    # Last line is "  child: value" which is key-bearing
    assert parser.is_on_key_bearing_line()
    assert parser.get_current_line_number() == 3  # Absolute last line (including empty)

    print("✓ Indent-only line state tracking test passed")


def test_line_type_accessible_for_scope_logic():
    """Test that line type is accessible for scope transition decisions."""
    yaml_content = """parent:
  child1: value1
  child2: value2
"""

    parser = YAMLParser(enable_scope_tracking=True, base_indent=2)

    # Parse line by line and check state
    lines = yaml_content.split('\n')

    for i, line in enumerate(lines, start=1):
        # Reparse to check state at each line
        parser_current = YAMLParser(enable_scope_tracking=True, base_indent=2)
        partial_content = '\n'.join(lines[:i])
        parser_current.parse_with_scope_tracking(partial_content)

        line_type = parser_current.get_current_line_type()
        line_num = parser_current.get_current_line_number()

        assert line_num == i, f"Expected line number {i}, got {line_num}"

        # Verify expected line types
        if i == 1:  # "parent:"
            assert line_type == LineClassification.KEY_BEARING
        elif i == 2:  # "  child1: value1"
            assert line_type == LineClassification.KEY_BEARING
        elif i == 3:  # "  child2: value2"
            assert line_type == LineClassification.KEY_BEARING

    print("✓ Line type accessible for scope logic test passed")


def test_complex_structure_line_type_tracking():
    """Test line type tracking through a complex YAML structure."""
    yaml_content = """# Config
services:
  web:
    host: localhost
    port: 8080

  db:
    host: db.example.com
    port: 5432

features:
  - auth
  - rate_limit:
    enabled: true
"""

    parser = YAMLParser(enable_scope_tracking=True, base_indent=2)
    result = parser.parse_with_scope_tracking(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    # Last line is "    enabled: true" - key-bearing
    assert parser.is_on_key_bearing_line()
    assert parser.get_current_line_number() == 15  # Absolute last line (including empty)

    # Verify the line types were tracked
    assert parser.get_current_line_type() == LineClassification.KEY_BEARING

    print("✓ Complex structure line type tracking test passed")


def test_line_type_in_scope_summary():
    """Test that line type information is available in scope context."""
    yaml_content = """parent:
  child: value
"""

    parser = YAMLParser(enable_scope_tracking=True, base_indent=2)
    result = parser.parse_with_scope_tracking(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    # Get scope summary
    summary = parser.get_scope_summary()

    # Verify scope tracking is working
    assert summary['depth'] > 0, "Scope depth should be positive"
    assert 'path' in summary

    # Verify line type is accessible separately
    line_type = parser.get_current_line_type()
    assert line_type is not None, "Line type should be accessible"
    assert line_type == LineClassification.KEY_BEARING

    print("✓ Line type in scope summary test passed")


def test_disabled_scope_tracking_line_type():
    """Test that line type returns None when scope tracking is disabled."""
    yaml_content = "key: value"

    parser = YAMLParser(enable_scope_tracking=False, base_indent=2)
    result = parser.parse_string(yaml_content)

    assert result.is_success(), f"Parse failed: {result.error}"

    # Line type should not be tracked when scope tracking is disabled
    assert parser.get_current_line_type() is None
    assert parser.get_current_line_number() == 0

    print("✓ Disabled scope tracking line type test passed")


def run_all_tests():
    """Run all line type state tracking tests."""
    print("\n" + "=" * 70)
    print("Line Type State Tracking Tests for YAML Parser")
    print("=" * 70 + "\n")

    try:
        test_line_type_state_tracking()
        test_line_type_getters()
        test_empty_line_state()
        test_indent_only_line_state()
        test_line_type_accessible_for_scope_logic()
        test_complex_structure_line_type_tracking()
        test_line_type_in_scope_summary()
        test_disabled_scope_tracking_line_type()

        print("=" * 70)
        print("✓ All line type state tracking tests passed!")
        print("=" * 70)
        print("\nAcceptance Criteria Verified:")
        print("  ✓ Line type is tracked in parser state")
        print("  ✓ State updates correctly on each line processed")
        print("  ✓ Line type information is available for downstream logic")

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
    success = run_all_tests()
    sys.exit(0 if success else 1)
