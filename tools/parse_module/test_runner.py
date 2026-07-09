"""
Simple test runner for YAML parser (pytest-free).
"""

import tempfile
import os
import sys
from pathlib import Path

# Add parent directory to path
sys.path.insert(0, str(Path(__file__).parent))

from yaml_parser import YAMLParser, ParseResult, ParseStatus


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
        return False


def test_success_result():
    """Test creating a success result."""
    result = ParseResult.success({'key': 'value'})
    assert result.status == ParseStatus.SUCCESS
    assert result.data == {'key': 'value'}
    assert result.error is None
    assert result.is_success()
    assert not result.is_error()


def test_error_result():
    """Test creating an error result."""
    result = ParseResult.error('Test error')
    assert result.status == ParseStatus.ERROR
    assert result.data is None
    assert result.error == 'Test error'
    assert not result.is_success()
    assert result.is_error()


def test_parse_simple_yaml():
    """Test parsing simple YAML string."""
    parser = YAMLParser()
    yaml_content = """
key: value
number: 42
"""
    result = parser.parse_string(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"
    assert result.data['key'] == 'value'
    assert result.data['number'] == 42


def test_parse_nested_yaml():
    """Test parsing nested YAML."""
    parser = YAMLParser()
    yaml_content = """
parent:
  child1: value1
  child2: value2
"""
    result = parser.parse_string(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"
    assert result.data['parent']['child1'] == 'value1'


def test_parse_yaml_list():
    """Test parsing YAML with lists."""
    parser = YAMLParser()
    yaml_content = """
items:
  - item1
  - item2
  - item3
"""
    result = parser.parse_string(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"
    assert result.data['items'] == ['item1', 'item2', 'item3']


def test_parse_empty_yaml():
    """Test parsing empty YAML string."""
    parser = YAMLParser()
    result = parser.parse_string("")
    assert result.is_error()
    assert 'Empty YAML content' in result.error


def test_parse_invalid_yaml():
    """Test parsing invalid YAML syntax."""
    parser = YAMLParser()
    invalid_yaml = """
key: value
  bad_indentation: here
"""
    result = parser.parse_string(invalid_yaml)
    assert result.is_error(), "Expected error for invalid YAML"
    assert result.error is not None


def test_parse_yaml_file():
    """Test parsing YAML from file."""
    parser = YAMLParser()
    yaml_content = """
name: test
version: 1.0
"""
    with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
        f.write(yaml_content)
        temp_path = f.name

    try:
        result = parser.parse_file(temp_path)
        assert result.is_success(), f"Expected success, got error: {result.error}"
        assert result.data['name'] == 'test'
        assert result.data['version'] == 1.0
    finally:
        os.unlink(temp_path)


def test_parse_nonexistent_file():
    """Test parsing file that doesn't exist."""
    parser = YAMLParser()
    result = parser.parse_file('/nonexistent/file.yaml')
    assert result.is_error()
    assert 'not found' in result.error.lower()


def test_parse_yaml_with_special_chars():
    """Test parsing YAML with special characters and unicode."""
    parser = YAMLParser()
    yaml_content = """
special: 'Test with "quotes"'
unicode: "Hello 世界"
"""
    result = parser.parse_string(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"
    assert 'quotes' in result.data['special']
    assert '世界' in result.data['unicode']


def test_parse_yaml_with_booleans():
    """Test parsing YAML with boolean values."""
    parser = YAMLParser()
    yaml_content = """
true_value: true
false_value: false
"""
    result = parser.parse_string(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"
    assert result.data['true_value'] is True
    assert result.data['false_value'] is False


def test_parse_yaml_with_nulls():
    """Test parsing YAML with null values."""
    parser = YAMLParser()
    yaml_content = """
null_value: null
empty_value: ~
"""
    result = parser.parse_string(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"
    assert result.data['null_value'] is None


def test_parse_multiline_string():
    """Test parsing YAML with multiline strings."""
    parser = YAMLParser()
    yaml_content = """
description: |
  This is a
  multiline string
"""
    result = parser.parse_string(yaml_content)
    assert result.is_success(), f"Expected success, got error: {result.error}"
    assert 'multiline string' in result.data['description']


def test_get_data_raises_on_error():
    """Test that get_data raises RuntimeError on error result."""
    result = ParseResult.error('Test error')
    try:
        result.get_data()
        assert False, "Expected RuntimeError"
    except RuntimeError as e:
        assert 'Test error' in str(e)


def main():
    """Run all tests."""
    print("Running YAML Parser Tests")
    print("=" * 50)

    tests = [
        # ParseResult tests
        ("Success result creation", test_success_result),
        ("Error result creation", test_error_result),

        # YAMLParser string parsing tests
        ("Parse simple YAML string", test_parse_simple_yaml),
        ("Parse nested YAML", test_parse_nested_yaml),
        ("Parse YAML with lists", test_parse_yaml_list),
        ("Parse empty YAML string", test_parse_empty_yaml),
        ("Parse invalid YAML syntax", test_parse_invalid_yaml),
        ("Parse YAML with special characters", test_parse_yaml_with_special_chars),
        ("Parse YAML with booleans", test_parse_yaml_with_booleans),
        ("Parse YAML with nulls", test_parse_yaml_with_nulls),
        ("Parse multiline strings", test_parse_multiline_string),

        # YAMLParser file parsing tests
        ("Parse YAML file", test_parse_yaml_file),
        ("Parse nonexistent file", test_parse_nonexistent_file),

        # Error handling tests
        ("get_data raises on error", test_get_data_raises_on_error),
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

    if failed == 0:
        print("✓ All tests passed!")
        return 0
    else:
        print(f"✗ {failed} test(s) failed")
        return 1


if __name__ == '__main__':
    sys.exit(main())
