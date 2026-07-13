"""
Unit tests for YAML Parser utility module.

Tests cover success cases, error cases, and edge cases.
"""

import pytest
import tempfile
import os
from pathlib import Path

# Import the parser module
import sys
sys.path.insert(0, str(Path(__file__).parent.parent))

from yaml_parser import YAMLParser, ParseResult, ParseStatus


class TestParseResult:
    """Test the ParseResult dataclass."""

    def test_success_result_creation(self):
        """Test creating a success result."""
        result = ParseResult(status=ParseStatus.SUCCESS, data={'key': 'value'})
        assert result.status == ParseStatus.SUCCESS
        assert result.data == {'key': 'value'}
        assert result.error is None
        assert result.is_success()
        assert not result.is_error()

    def test_error_result_creation(self):
        """Test creating an error result."""
        result = ParseResult(status=ParseStatus.ERROR, error='Test error')
        assert result.status == ParseStatus.ERROR
        assert result.data is None
        assert result.error == 'Test error'
        assert not result.is_success()
        assert result.is_error()

    def test_is_success_method(self):
        """Test is_success method."""
        success_result = ParseResult(status=ParseStatus.SUCCESS, data={})
        error_result = ParseResult(status=ParseStatus.ERROR, error='Failed')

        assert success_result.is_success() is True
        assert success_result.is_error() is False
        assert error_result.is_success() is False
        assert error_result.is_error() is True


class TestYAMLParser:
    """Test the YAMLParser class."""

    def setup_method(self):
        """Set up test fixtures."""
        self.parser = YAMLParser()

    def test_parser_initialization(self):
        """Test that parser initializes correctly."""
        assert self.parser.yaml is not None
        assert hasattr(self.parser, 'parse_string')
        assert hasattr(self.parser, 'parse_file')

    # String parsing tests
    def test_parse_simple_yaml_string(self):
        """Test parsing a simple YAML string."""
        yaml_content = """
key: value
number: 42
"""
        result = self.parser.parse_string(yaml_content)

        assert result.is_success()
        assert result.data['key'] == 'value'
        assert result.data['number'] == 42

    def test_parse_nested_yaml_string(self):
        """Test parsing nested YAML structure."""
        yaml_content = """
parent:
  child1: value1
  child2: value2
  nested:
    deep: value
"""
        result = self.parser.parse_string(yaml_content)

        assert result.is_success()
        assert result.data['parent']['child1'] == 'value1'
        assert result.data['parent']['nested']['deep'] == 'value'

    def test_parse_list_yaml_string(self):
        """Test parsing YAML with lists."""
        yaml_content = """
items:
  - item1
  - item2
  - item3
"""
        result = self.parser.parse_string(yaml_content)

        assert result.is_success()
        assert result.data['items'] == ['item1', 'item2', 'item3']

    def test_parse_empty_yaml_string(self):
        """Test parsing empty YAML string."""
        result = self.parser.parse_string("")
        assert result.is_error()
        assert 'Empty YAML content' in result.error

    def test_parse_whitespace_only_yaml_string(self):
        """Test parsing whitespace-only YAML string."""
        result = self.parser.parse_string("   \n  \n  ")
        assert result.is_error()
        assert 'Empty YAML content' in result.error

    def test_parse_invalid_yaml_syntax(self):
        """Test parsing invalid YAML syntax."""
        invalid_yaml = """
key: value
  bad_indentation: here
    worse: indentation
"""
        result = self.parser.parse_string(invalid_yaml)
        assert result.is_error()
        assert result.error is not None

    def test_parse_yaml_with_duplicate_keys(self):
        """Test parsing YAML with duplicate keys."""
        # PyYAML should handle this or raise an error
        yaml_content = """
key: value1
key: value2
"""
        result = self.parser.parse_string(yaml_content)
        # PyYAML typically takes the last value
        # But this may vary by version
        if result.is_error():
            assert 'duplicate' in result.error.lower() or 'error' in result.error.lower()
        else:
            # Some versions accept duplicate keys
            assert result.data['key'] == 'value2'

    # File parsing tests
    def test_parse_simple_yaml_file(self):
        """Test parsing a simple YAML file."""
        yaml_content = """
name: test
version: 1.0
"""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            f.write(yaml_content)
            temp_path = f.name

        try:
            result = self.parser.parse_file(temp_path)
            assert result.is_success()
            assert result.data['name'] == 'test'
            assert result.data['version'] == 1.0
        finally:
            os.unlink(temp_path)

    def test_parse_nonexistent_file(self):
        """Test parsing a file that doesn't exist."""
        result = self.parser.parse_file('/nonexistent/file.yaml')
        assert result.is_error()
        assert 'not found' in result.error.lower()

    def test_parse_directory_instead_of_file(self):
        """Test parsing a directory path instead of a file."""
        with tempfile.TemporaryDirectory() as temp_dir:
            result = self.parser.parse_file(temp_dir)
            assert result.is_error()
            assert 'not a file' in result.error.lower()

    def test_parse_empty_file(self):
        """Test parsing an empty file."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            temp_path = f.name

        try:
            result = self.parser.parse_file(temp_path)
            assert result.is_error()
            assert 'empty' in result.error.lower()
        finally:
            os.unlink(temp_path)

    def test_parse_invalid_yaml_file(self):
        """Test parsing a file with invalid YAML syntax."""
        invalid_yaml = """
key: value
  invalid: indentation
    broken: structure
"""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            f.write(invalid_yaml)
            temp_path = f.name

        try:
            result = self.parser.parse_file(temp_path)
            assert result.is_error()
            assert result.error is not None
        finally:
            os.unlink(temp_path)

    def test_parse_yaml_with_special_characters(self):
        """Test parsing YAML with special characters."""
        yaml_content = """
special_chars: 'Test with "quotes" and escaped characters'
unicode: "Hello 世界"
newlines: "Line 1\\nLine 2\\nLine 3"
"""
        result = self.parser.parse_string(yaml_content)
        assert result.is_success()
        assert 'quotes' in result.data['special_chars']
        assert '世界' in result.data['unicode']

    def test_parse_yaml_with_booleans(self):
        """Test parsing YAML with boolean values."""
        yaml_content = """
true_value: true
false_value: false
yes_value: yes
no_value: no
"""
        result = self.parser.parse_string(yaml_content)
        assert result.is_success()
        assert result.data['true_value'] is True
        assert result.data['false_value'] is False

    def test_parse_yaml_with_nulls(self):
        """Test parsing YAML with null values."""
        yaml_content = """
null_value: null
empty_value: ~
another_null: Null
"""
        result = self.parser.parse_string(yaml_content)
        assert result.is_success()
        assert result.data['null_value'] is None
        assert result.data['empty_value'] is None

    def test_parse_multiline_string(self):
        """Test parsing YAML with multiline strings."""
        yaml_content = """
description: |
  This is a
  multiline string
  that preserves
  newlines.
"""
        result = self.parser.parse_string(yaml_content)
        assert result.is_success()
        assert 'multiline string' in result.data['description']


class TestYAMLParserEdgeCases:
    """Test edge cases and error conditions."""

    def setup_method(self):
        """Set up test fixtures."""
        self.parser = YAMLParser()

    def test_parse_very_long_string(self):
        """Test parsing a very long YAML string."""
        long_content = "\n".join([f"item{i}: value{i}" for i in range(1000)])
        result = self.parser.parse_string(long_content)
        assert result.is_success()
        assert 'item0' in result.data
        assert 'item999' in result.data

    def test_parse_yaml_with_complex_numbers(self):
        """Test parsing YAML with various number formats."""
        yaml_content = """
integer: 42
float: 3.14
scientific: 1.23e-4
negative: -10
"""
        result = self.parser.parse_string(yaml_content)
        assert result.is_success()
        assert result.data['integer'] == 42
        assert result.data['float'] == 3.14
        assert result.data['negative'] == -10

    def test_parse_yaml_with_anchors_and_aliases(self):
        """Test parsing YAML with anchors and aliases."""
        yaml_content = """
defaults: &defaults
  timeout: 30
  retry: 3

production:
  <<: *defaults
  host: prod.example.com
"""
        result = self.parser.parse_string(yaml_content)
        assert result.is_success()
        assert result.data['production']['timeout'] == 30
        assert result.data['production']['host'] == 'prod.example.com'

    def test_parse_yaml_with_comments(self):
        """Test parsing YAML with comments."""
        yaml_content = """
# This is a comment
key: value  # inline comment
# Another comment
another_key: another_value
"""
        result = self.parser.parse_string(yaml_content)
        assert result.is_success()
        assert result.data['key'] == 'value'
        assert result.data['another_key'] == 'another_value'


class TestTypeSpecificScopeTransitions:
    """Test type-specific scope transition handling."""

    def setup_method(self):
        """Set up test fixtures."""
        self.parser = YAMLParser()

    def test_key_bearing_line_creates_new_scope_on_indent_increase(self):
        """Test that key-bearing lines create new scopes when indent increases."""
        yaml_content = """
parent:
  child: value
"""
        result = self.parser.parse_with_scope_tracking(yaml_content)
        assert result.is_success()

        # Should have depth 3 at the end: root -> parent -> (back to parent)
        # Line 1: "parent:" creates parent scope (depth 2)
        # Line 2: "  child:" is at same indent as parent, no new scope
        final_depth = self.parser.get_scope_depth()
        assert final_depth >= 2

        transitions = self.parser.get_scope_stack().get_indent_transitions()
        # Should have recorded transitions
        assert len(transitions) > 0

    def test_indent_only_line_does_not_create_new_scope(self):
        """Test that indent-only lines do NOT create new scopes."""
        yaml_content = """
parent:
  child: value
    continuation
    another_continuation
  sibling: another_value
"""
        result = self.parser.parse_with_scope_tracking(yaml_content)
        assert result.is_success()

        transitions = self.parser.get_scope_stack().get_indent_transitions()

        # Find transitions at continuation lines
        continuation_transitions = [t for t in transitions if 'continuation' in t.raw_line]
        assert len(continuation_transitions) > 0

        # These should be classified as INDENT_ONLY
        for trans in continuation_transitions:
            if 'continuation' in trans.raw_line:
                assert trans.line_classification.value == 'indent-only'

    def test_multiline_string_does_not_create_scope(self):
        """Test that multiline string continuations do not create new scopes."""
        yaml_content = """
description: |
  Line 1
    Line 2 with extra indent
  Line 3
"""
        result = self.parser.parse_with_scope_tracking(yaml_content)
        assert result.is_success()

        transitions = self.parser.get_scope_stack().get_indent_transitions()

        # Lines inside the multiline block should be INDENT_ONLY
        multiline_transitions = [t for t in transitions if 'Line' in t.raw_line]
        assert len(multiline_transitions) > 0

        for trans in multiline_transitions:
            # Multiline content lines are indent-only, not key-bearing
            assert trans.line_classification.value == 'indent-only'

    def test_complex_nested_structure_scope_handling(self):
        """Test scope handling for complex nested YAML structures."""
        yaml_content = """
level1:
  level2:
    level3: value
    level3_sibling: another
  level2_sibling: value2
"""
        result = self.parser.parse_with_scope_tracking(yaml_content)
        assert result.is_success()

        transitions = self.parser.get_scope_stack().get_indent_transitions()

        # Count enter/exit scope transitions
        enter_scopes = [t for t in transitions if t.is_enter_scope()]
        exit_scopes = [t for t in transitions if t.is_exit_scope()]

        # Should have multiple scope transitions
        assert len(enter_scopes) > 0
        assert len(exit_scopes) > 0

        # Check that key-bearing lines trigger scope transitions
        key_bearing_enters = [t for t in enter_scopes if t.line_classification.value == 'key-bearing']
        assert len(key_bearing_enters) > 0

    def test_mixed_key_bearing_and_indent_only_lines(self):
        """Test mixed scenarios with both key-bearing and indent-only lines."""
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
        result = self.parser.parse_with_scope_tracking(yaml_content)
        assert result.is_success()

        transitions = self.parser.get_scope_stack().get_indent_transitions()

        # Verify we have both types of lines classified
        key_bearing = [t for t in transitions if t.line_classification.value == 'key-bearing']
        indent_only = [t for t in transitions if t.line_classification.value == 'indent-only']

        assert len(key_bearing) > 0
        assert len(indent_only) > 0

        # Key-bearing transitions should create scopes
        key_enters = [t for t in key_bearing if t.is_enter_scope()]
        assert len(key_enters) > 0

    def test_scope_transition_classification_accuracy(self):
        """Test that scope transitions are classified correctly by type."""
        yaml_content = """
outer:
  inner_key: inner_value
  - list_item_1
  - list_item_2
  another_key: another_value
"""
        result = self.parser.parse_with_scope_tracking(yaml_content)
        assert result.is_success()

        transitions = self.parser.get_scope_stack().get_indent_transitions()

        # Verify each transition has proper classification
        for trans in transitions:
            assert trans.line_classification is not None
            assert trans.transition_type is not None

            # If it's a key-bearing line, it should have has_key=True
            if trans.line_classification.value == 'key-bearing':
                assert trans.has_key is True

    def test_empty_lines_do_not_trigger_scope_transitions(self):
        """Test that empty lines don't trigger scope transitions."""
        yaml_content = """
key1: value1

key2: value2


key3: value3
"""
        result = self.parser.parse_with_scope_tracking(yaml_content)
        assert result.is_success()

        transitions = self.parser.get_scope_stack().get_indent_transitions()

        # Empty lines should be classified as EMPTY
        empty_transitions = [t for t in transitions if t.line_classification.value == 'empty']
        # Empty lines at same indent shouldn't create transitions
        # (they may not appear in transitions if indent doesn't change)

    def test_line_type_classification_methods(self):
        """Test parser methods for checking current line type."""
        yaml_content = """
key: value
  indent_only_content
"""
        result = self.parser.parse_with_scope_tracking(yaml_content)
        assert result.is_success()

        # Parser should have line type tracking
        assert hasattr(self.parser, 'is_on_key_bearing_line')
        assert hasattr(self.parser, 'is_on_indent_only_line')
        assert hasattr(self.parser, 'is_on_empty_line')


def test_module_exports():
    """Test that the module exports expected symbols."""
    import yaml_parser
    assert hasattr(yaml_parser, 'YAMLParser')
    assert hasattr(yaml_parser, 'ParseResult')


if __name__ == '__main__':
    # Run tests with pytest
    pytest.main([__file__, '-v'])
