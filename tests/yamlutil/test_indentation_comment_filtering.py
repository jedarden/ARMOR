#!/usr/bin/env python3
"""
Tests for YAML comment filtering across various indentation levels.

This module tests comment filtering behavior at different indentation depths:
- Zero indentation (root level)
- Single indentation (2 spaces)
- Double indentation (4 spaces)
- Deep indentation (8+ spaces)

These tests ensure that comments are properly filtered regardless of their
indentation level in nested YAML structures.
"""

import sys
from pathlib import Path

# Add project root to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from internal.yamlutil.parser import YAMLCoreParser


def run_test(test_name, test_func):
    """Run a single test and report results."""
    try:
        test_func()
        print(f"✓ {test_name}")
        return True
    except AssertionError as e:
        print(f"✗ {test_name}: {e}")
        return False
    except Exception as e:
        print(f"✗ {test_name}: Unexpected error: {e}")
        return False


# ============================================================================
# Zero Indentation Tests (Root Level Comments)
# ============================================================================

def test_root_level_full_line_comments():
    """Test full-line comments at root level (zero indentation)."""
    parser = YAMLCoreParser()
    yaml_content = """# Root level comment 1
# Root level comment 2
key1: value1
# Another root comment
key2: value2
# Final root comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data == {'key1': 'value1', 'key2': 'value2'}, \
        "Root level comments should be filtered out"
    assert 'comment' not in str(result.data).lower(), \
        "No comment text should appear in parsed data"


def test_root_level_inline_comments():
    """Test inline comments at root level (zero indentation)."""
    parser = YAMLCoreParser()
    yaml_content = """key1: value1  # Inline comment at root
key2: value2  # Another inline comment
key3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['key1'] == 'value1', "Inline comment should be stripped"
    assert result.data['key2'] == 'value2', "Inline comment should be stripped"
    assert 'comment' not in str(result.data['key1']).lower(), \
        "Comment should not be in value"


def test_root_level_mixed_comments():
    """Test mixed full-line and inline comments at root level."""
    parser = YAMLCoreParser()
    yaml_content = """# Header comment at root
key1: value1  # Inline comment
# Another full-line comment
key2: value2  # Another inline
# Footer comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data == {'key1': 'value1', 'key2': 'value2'}, \
        "All comments should be filtered"


# ============================================================================
# Single Indentation Tests (2 spaces)
# ============================================================================

def test_single_indent_full_line_comments():
    """Test full-line comments at single indentation level (2 spaces)."""
    parser = YAMLCoreParser()
    yaml_content = """# Root comment
parent:
  # Indented comment at 2 spaces
  child1: value1
  # Another 2-space comment
  child2: value2
  # One more 2-space comment
  child3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert 'child1' in result.data['parent'], "Child should be parsed"
    assert 'child2' in result.data['parent'], "Child should be parsed"
    assert 'child3' in result.data['parent'], "Child should be parsed"
    assert result.data['parent']['child1'] == 'value1', \
        "Value should be correct"
    assert 'comment' not in str(result.data['parent']).lower(), \
        "No comment text should appear in parent data"


def test_single_indent_inline_comments():
    """Test inline comments at single indentation level (2 spaces)."""
    parser = YAMLCoreParser()
    yaml_content = """# Root level
parent:
  child1: value1  # Inline at 2 spaces
  child2: value2  # Another inline at 2 spaces
  child3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['parent']['child1'] == 'value1', \
        "Inline comment should be stripped"
    assert result.data['parent']['child2'] == 'value2', \
        "Inline comment should be stripped"
    assert 'comment' not in str(result.data['parent']['child1']).lower(), \
        "Comment should not be in value"


def test_single_indent_mixed_comments():
    """Test mixed full-line and inline comments at single indentation."""
    parser = YAMLCoreParser()
    yaml_content = """# Root comment
parent:
  # Full-line at 2 spaces
  child1: value1  # Inline at 2 spaces
  # Another full-line at 2 spaces
  child2: value2  # Another inline
  # Final full-line at 2 spaces
  child3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['parent']['child1'] == 'value1', \
        "All comments should be filtered"
    assert result.data['parent']['child2'] == 'value2', \
        "All comments should be filtered"
    assert result.data['parent']['child3'] == 'value3', \
        "All comments should be filtered"


# ============================================================================
# Double Indentation Tests (4 spaces)
# ============================================================================

def test_double_indent_full_line_comments():
    """Test full-line comments at double indentation level (4 spaces)."""
    parser = YAMLCoreParser()
    yaml_content = """# Root comment
parent:
  child:
    # Comment at 4 spaces
    grandchild1: value1
    # Another 4-space comment
    grandchild2: value2
    # One more 4-space comment
    grandchild3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert 'grandchild1' in result.data['parent']['child'], \
        "Grandchild should be parsed"
    assert 'grandchild2' in result.data['parent']['child'], \
        "Grandchild should be parsed"
    assert 'grandchild3' in result.data['parent']['child'], \
        "Grandchild should be parsed"
    assert result.data['parent']['child']['grandchild1'] == 'value1', \
        "Value should be correct"
    assert 'comment' not in str(result.data['parent']['child']).lower(), \
        "No comment text should appear in nested data"


def test_double_indent_inline_comments():
    """Test inline comments at double indentation level (4 spaces)."""
    parser = YAMLCoreParser()
    yaml_content = """# Root level
parent:
  child:
    grandchild1: value1  # Inline at 4 spaces
    grandchild2: value2  # Another inline at 4 spaces
    grandchild3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['parent']['child']['grandchild1'] == 'value1', \
        "Inline comment should be stripped"
    assert result.data['parent']['child']['grandchild2'] == 'value2', \
        "Inline comment should be stripped"
    assert 'comment' not in str(result.data['parent']['child']['grandchild1']).lower(), \
        "Comment should not be in value"


def test_double_indent_mixed_comments():
    """Test mixed full-line and inline comments at double indentation."""
    parser = YAMLCoreParser()
    yaml_content = """# Root comment
parent:
  child:
    # Full-line at 4 spaces
    grandchild1: value1  # Inline at 4 spaces
    # Another full-line at 4 spaces
    grandchild2: value2  # Another inline
    # Final full-line at 4 spaces
    grandchild3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['parent']['child']['grandchild1'] == 'value1', \
        "All comments should be filtered"
    assert result.data['parent']['child']['grandchild2'] == 'value2', \
        "All comments should be filtered"
    assert result.data['parent']['child']['grandchild3'] == 'value3', \
        "All comments should be filtered"


# ============================================================================
# Deep Indentation Tests (8+ spaces)
# ============================================================================

def test_deep_indent_8_spaces_full_line_comments():
    """Test full-line comments at 8-space indentation level."""
    parser = YAMLCoreParser()
    yaml_content = """# Root comment
level1:
  level2:
    level3:
      # Comment at 8 spaces
      item1: value1
      # Another 8-space comment
      item2: value2
      # One more 8-space comment
      item3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert 'item1' in result.data['level1']['level2']['level3'], \
        "Deeply nested item should be parsed"
    assert 'item2' in result.data['level1']['level2']['level3'], \
        "Deeply nested item should be parsed"
    assert 'item3' in result.data['level1']['level2']['level3'], \
        "Deeply nested item should be parsed"
    assert result.data['level1']['level2']['level3']['item1'] == 'value1', \
        "Value should be correct at deep indentation"
    assert 'comment' not in str(result.data['level1']['level2']['level3']).lower(), \
        "No comment text should appear in deeply nested data"


def test_deep_indent_8_spaces_inline_comments():
    """Test inline comments at 8-space indentation level."""
    parser = YAMLCoreParser()
    yaml_content = """# Root level
level1:
  level2:
    level3:
      item1: value1  # Inline at 8 spaces
      item2: value2  # Another inline at 8 spaces
      item3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['level1']['level2']['level3']['item1'] == 'value1', \
        "Inline comment should be stripped at deep indentation"
    assert result.data['level1']['level2']['level3']['item2'] == 'value2', \
        "Inline comment should be stripped at deep indentation"
    assert 'comment' not in str(result.data['level1']['level2']['level3']['item1']).lower(), \
        "Comment should not be in value at deep indentation"


def test_deep_indent_10_spaces_full_line_comments():
    """Test full-line comments at 10-space indentation level."""
    parser = YAMLCoreParser()
    yaml_content = """# Root comment
level1:
  level2:
    level3:
      level4:
        # Comment at 10 spaces
        deep_item1: value1
        # Another 10-space comment
        deep_item2: value2
        # One more 10-space comment
        deep_item3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert 'deep_item1' in result.data['level1']['level2']['level3']['level4'], \
        "Very deeply nested item should be parsed"
    assert 'deep_item2' in result.data['level1']['level2']['level3']['level4'], \
        "Very deeply nested item should be parsed"
    assert 'deep_item3' in result.data['level1']['level2']['level3']['level4'], \
        "Very deeply nested item should be parsed"
    assert result.data['level1']['level2']['level3']['level4']['deep_item1'] == 'value1', \
        "Value should be correct at very deep indentation"
    assert 'comment' not in str(result.data['level1']['level2']['level3']['level4']).lower(), \
        "No comment text should appear at very deep indentation"


def test_deep_indent_12_spaces_mixed_comments():
    """Test mixed comments at 12-space indentation level (deepest test)."""
    parser = YAMLCoreParser()
    yaml_content = """# Root comment
level1:
  level2:
    level3:
      level4:
        level5:
          # Full-line at 12 spaces
          deepest_item1: value1  # Inline at 12 spaces
          # Another full-line at 12 spaces
          deepest_item2: value2  # Another inline
          # Final full-line at 12 spaces
          deepest_item3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['level1']['level2']['level3']['level4']['level5']['deepest_item1'] == 'value1', \
        "All comments should be filtered at deepest indentation"
    assert result.data['level1']['level2']['level3']['level4']['level5']['deepest_item2'] == 'value2', \
        "All comments should be filtered at deepest indentation"
    assert result.data['level1']['level2']['level3']['level4']['level5']['deepest_item3'] == 'value3', \
        "All comments should be filtered at deepest indentation"


# ============================================================================
# Complex Multi-Level Comment Tests
# ============================================================================

def test_comments_at_multiple_indentation_levels():
    """Test comments appearing at multiple indentation levels in same document."""
    parser = YAMLCoreParser()
    yaml_content = """# Root level comment (0 spaces)
root_key: root_value  # Root inline comment

level1:
  # Level 1 comment (2 spaces)
  key1: value1  # Level 1 inline
  level2:
    # Level 2 comment (4 spaces)
    key2: value2  # Level 2 inline
    level3:
      # Level 3 comment (6 spaces)
      key3: value3  # Level 3 inline
      level4:
        # Level 4 comment (8 spaces)
        key4: value4  # Level 4 inline

# Another root comment
final_key: final_value
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['root_key'] == 'root_value', \
        "Root value should be correct"
    assert result.data['level1']['key1'] == 'value1', \
        "Level 1 value should be correct"
    assert result.data['level1']['level2']['key2'] == 'value2', \
        "Level 2 value should be correct"
    assert result.data['level1']['level2']['level3']['key3'] == 'value3', \
        "Level 3 value should be correct"
    assert result.data['level1']['level2']['level3']['level4']['key4'] == 'value4', \
        "Level 4 value should be correct"
    assert result.data['final_key'] == 'final_value', \
        "Final value should be correct"
    assert 'comment' not in str(result.data).lower(), \
        "No comment text should appear anywhere in data"


def test_comment_filtering_in_nested_sequences():
    """Test comment filtering in nested sequences at various indentation levels."""
    parser = YAMLCoreParser()
    yaml_content = """# Root comment
sequence_test:
  # Level 1 comment before sequence
  - item1  # Inline at 2 spaces
  # Comment between items
  - item2
  - nested_sequence:  # Sequence with nested value
      # Level 2 comment (4 spaces)
      - nested_item1  # Inline at 4 spaces
      # Another level 2 comment
      - nested_item2
  - final_item  # Final inline comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert 'sequence_test' in result.data, "Sequence should be parsed"
    assert 'item1' in result.data['sequence_test'], \
        "First item should be present"
    assert 'item2' in result.data['sequence_test'], \
        "Second item should be present"
    assert 'comment' not in str(result.data['sequence_test']).lower(), \
        "No comment text should appear in sequence data"


def test_comment_filtering_in_complex_nested_structure():
    """Test comment filtering in a complex nested structure with mixed types."""
    parser = YAMLCoreParser()
    yaml_content = """# Configuration file comment
config:
  # Database configuration comment
  database:
    # Primary database comment
    primary:
      # Database host comment
      host: localhost  # Inline host comment
      # Database port comment
      port: 5432  # Inline port comment
      # Database name comment
      name: mydb  # Inline name comment
    # Replica database comment
    replica:
      # Replica host comment
      host: replica.example.com  # Inline replica host
      # Replica port comment
      port: 5433  # Inline replica port
  # Cache configuration comment
  cache:
    # Redis configuration comment
    redis:
      # Redis host comment
      host: localhost  # Inline redis host
      # Redis port comment
      port: 6379  # Inline redis port
      # Redis DB comment
      db: 0  # Inline redis db
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['config']['database']['primary']['host'] == 'localhost', \
        "Deep nested primary host should be correct"
    assert result.data['config']['database']['primary']['port'] == 5432, \
        "Deep nested primary port should be correct"
    assert result.data['config']['database']['primary']['name'] == 'mydb', \
        "Deep nested primary name should be correct"
    assert result.data['config']['database']['replica']['host'] == 'replica.example.com', \
        "Deep nested replica host should be correct"
    assert result.data['config']['database']['replica']['port'] == 5433, \
        "Deep nested replica port should be correct"
    assert result.data['config']['cache']['redis']['host'] == 'localhost', \
        "Deep nested cache host should be correct"
    assert result.data['config']['cache']['redis']['port'] == 6379, \
        "Deep nested cache port should be correct"
    assert result.data['config']['cache']['redis']['db'] == 0, \
        "Deep nested cache db should be correct"
    assert 'comment' not in str(result.data).lower(), \
        "No comment text should appear anywhere in complex structure"


# ============================================================================
# Main Test Runner
# ============================================================================

def main():
    """Run all indentation-based comment filtering tests."""
    print("Running YAML Comment Filtering Tests Across Indentation Levels")
    print("=" * 70)

    tests = [
        # Zero indentation (root level) tests
        ("Root level full-line comments", test_root_level_full_line_comments),
        ("Root level inline comments", test_root_level_inline_comments),
        ("Root level mixed comments", test_root_level_mixed_comments),

        # Single indentation (2 spaces) tests
        ("Single indent full-line comments", test_single_indent_full_line_comments),
        ("Single indent inline comments", test_single_indent_inline_comments),
        ("Single indent mixed comments", test_single_indent_mixed_comments),

        # Double indentation (4 spaces) tests
        ("Double indent full-line comments", test_double_indent_full_line_comments),
        ("Double indent inline comments", test_double_indent_inline_comments),
        ("Double indent mixed comments", test_double_indent_mixed_comments),

        # Deep indentation (8+ spaces) tests
        ("8-space full-line comments", test_deep_indent_8_spaces_full_line_comments),
        ("8-space inline comments", test_deep_indent_8_spaces_inline_comments),
        ("10-space full-line comments", test_deep_indent_10_spaces_full_line_comments),
        ("12-space mixed comments", test_deep_indent_12_spaces_mixed_comments),

        # Complex multi-level tests
        ("Comments at multiple levels", test_comments_at_multiple_indentation_levels),
        ("Comments in nested sequences", test_comment_filtering_in_nested_sequences),
        ("Comments in complex structure", test_comment_filtering_in_complex_nested_structure),
    ]

    passed = 0
    failed = 0

    for test_name, test_func in tests:
        if run_test(test_name, test_func):
            passed += 1
        else:
            failed += 1

    print("=" * 70)
    print(f"Results: {passed} passed, {failed} failed")

    if failed > 0:
        sys.exit(1)

    print("\n✓ All indentation-based comment filtering tests passed!")
    sys.exit(0)


if __name__ == '__main__':
    main()
