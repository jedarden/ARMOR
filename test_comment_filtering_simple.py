#!/usr/bin/env python3
"""
Simple test runner for YAML comment filtering tests.
This script runs basic comment filtering tests without requiring pytest.
"""

import sys
from pathlib import Path

# Add project root to path
sys.path.insert(0, str(Path(__file__).parent))

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


def test_full_line_comment_detection():
    """Test for full-line comment detection."""
    parser = YAMLCoreParser()
    yaml_content = """# This is a full-line comment
key: value
# Another comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data == {'key': 'value'}, "Comments should be filtered out"
    assert 'comment' not in str(result.data).lower(), "Comment text should not appear in data"


def test_inline_comment_detection():
    """Test for inline comment detection."""
    parser = YAMLCoreParser()
    yaml_content = """key: value  # This is an inline comment
another_key: another_value  # Another inline comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['key'] == 'value', "Inline comment should be stripped"
    assert result.data['another_key'] == 'another_value', "Inline comment should be stripped"
    assert 'comment' not in str(result.data['key']), "Comment should not be in value"
    assert 'comment' not in str(result.data['another_key']), "Comment should not be in value"


def test_comment_at_start_of_line():
    """Test for comments at start of line."""
    parser = YAMLCoreParser()
    yaml_content = """# Comment at start
key: value
# Another comment at start
another: item
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data == {'key': 'value', 'another': 'item'}


def test_comment_at_middle_of_document():
    """Test for comments in middle of document."""
    parser = YAMLCoreParser()
    yaml_content = """key1: value1
# Comment in middle
key2: value2
# Another middle comment
key3: value3
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data == {'key1': 'value1', 'key2': 'value2', 'key3': 'value3'}


def test_comment_at_end_of_document():
    """Test for comments at end of document."""
    parser = YAMLCoreParser()
    yaml_content = """key: value
another: item
# Footer comment
# End of file
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data == {'key': 'value', 'another': 'item'}


def test_comment_without_space_after_hash():
    """Test comment without space after hash (#text)."""
    parser = YAMLCoreParser()
    yaml_content = """#Comment without space
key: value
#Another comment without space
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data == {'key': 'value'}


def test_comment_with_leading_whitespace():
    """Test comment with leading whitespace before hash."""
    parser = YAMLCoreParser()
    yaml_content = """  # Comment with leading spaces
key: value
    # Comment with more spaces (4 spaces)
another: value2
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data == {'key': 'value', 'another': 'value2'}


def test_inline_comment_no_space():
    """Test inline comment without space after value."""
    parser = YAMLCoreParser()
    yaml_content = """key: value #inline comment without space
another: item
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['key'] == 'value', "Inline comment should be stripped"


def test_hash_in_quoted_string_preserved():
    """Test that hash inside quoted strings is preserved."""
    parser = YAMLCoreParser()
    yaml_content = """key: "value with # hash inside quotes"
another: value_without_quotes  # but this is a comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert 'hash' in result.data['key'], "Hash inside quotes should be preserved"
    assert 'comment' not in result.data['another'], "Hash outside quotes should be comment"


def test_mixed_full_and_inline_comments():
    """Test mixed full-line and inline comments."""
    parser = YAMLCoreParser()
    yaml_content = """# Full-line comment
key1: value1  # Inline comment
# Another full-line comment
key2: value2  # Another inline comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data == {'key1': 'value1', 'key2': 'value2'}


def test_consecutive_full_line_comments():
    """Test multiple consecutive full-line comments."""
    parser = YAMLCoreParser()
    yaml_content = """# First comment
# Second comment
# Third comment
key: value
# Fourth comment
# Fifth comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data == {'key': 'value'}


def test_empty_lines_with_comments():
    """Test handling of empty lines mixed with comments."""
    parser = YAMLCoreParser()
    yaml_content = """# Header comment

key1: value1  # inline comment

# Another comment
key2: value2

# Footer comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data == {'key1': 'value1', 'key2': 'value2'}


def main():
    """Run all comment filtering tests."""
    print("Running YAML Comment Filtering Tests")
    print("=" * 50)

    tests = [
        ("Full-line comment detection", test_full_line_comment_detection),
        ("Inline comment detection", test_inline_comment_detection),
        ("Comment at start of document", test_comment_at_start_of_line),
        ("Comment at middle of document", test_comment_at_middle_of_document),
        ("Comment at end of document", test_comment_at_end_of_document),
        ("Comment without space after hash", test_comment_without_space_after_hash),
        ("Comment with leading whitespace", test_comment_with_leading_whitespace),
        ("Inline comment no space", test_inline_comment_no_space),
        ("Hash in quoted string preserved", test_hash_in_quoted_string_preserved),
        ("Mixed full and inline comments", test_mixed_full_and_inline_comments),
        ("Consecutive full-line comments", test_consecutive_full_line_comments),
        ("Empty lines with comments", test_empty_lines_with_comments),
    ]

    passed = 0
    failed = 0

    for test_name, test_func in tests:
        if run_test(test_name, test_func):
            passed += 1
        else:
            failed += 1

    print("=" * 50)
    print(f"Results: {passed} passed, {failed} failed")

    if failed > 0:
        sys.exit(1)

    print("\n✓ All comment filtering tests passed!")
    sys.exit(0)


if __name__ == '__main__':
    main()
