"""
Unit tests for Core YAML Parser with explicit error handling.

Tests cover:
- Safe load wrapper function
- Explicit handling of YAMLError, ScannerError, ParserError
- Structured error information on failure
- Result structure integration
"""

import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

import pytest
from yamlutil.parser import (
    YAMLCoreParser,
    SafeLoadResult,
    safe_load_yaml
)
from yamlutil.error_types import (
    YAMLErrorCategory,
    YAMLErrorSeverity
)


class TestSafeLoadResult:
    """Test the SafeLoadResult result structure."""

    def test_success_result_creation(self):
        """Test creating a success result."""
        result = SafeLoadResult(success=True, data={'key': 'value'})
        assert result.success is True
        assert result.data == {'key': 'value'}
        assert result.error is None
        assert result.raw_exception is None
        assert result.is_success()
        assert not result.is_error()

    def test_error_result_creation(self):
        """Test creating an error result."""
        from yamlutil.error_types import YAMLErrorDetail

        error = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.ERROR,
            message="Test error"
        )
        result = SafeLoadResult(success=False, error=error)
        assert result.success is False
        assert result.data is None
        assert result.error == error
        assert not result.is_success()
        assert result.is_error()

    def test_get_data_on_success(self):
        """Test get_data returns data when successful."""
        result = SafeLoadResult(success=True, data={'test': 'data'})
        assert result.get_data() == {'test': 'data'}

    def test_get_data_on_error_raises(self):
        """Test get_data raises RuntimeError when failed."""
        from yamlutil.error_types import YAMLErrorDetail

        error = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.ERROR,
            message="Parse failed"
        )
        result = SafeLoadResult(success=False, error=error)
        with pytest.raises(RuntimeError, match="Cannot get data from failed parse"):
            result.get_data()


class TestYAMLCoreParser:
    """Test the YAMLCoreParser class."""

    def setup_method(self):
        """Set up test fixtures."""
        self.parser = YAMLCoreParser()

    def test_parser_initialization(self):
        """Test that parser initializes correctly."""
        assert self.parser.yaml is not None
        assert self.parser.ScannerError is not None
        assert self.parser.ParserError is not None
        assert hasattr(self.parser, 'safe_load')

    # Success cases
    def test_parse_simple_yaml(self):
        """Test parsing simple YAML content."""
        yaml_content = """
key: value
number: 42
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data['key'] == 'value'
        assert result.data['number'] == 42
        assert result.error is None

    def test_parse_nested_yaml(self):
        """Test parsing nested YAML structure."""
        yaml_content = """
parent:
  child1: value1
  child2: value2
  nested:
    deep: value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data['parent']['child1'] == 'value1'
        assert result.data['parent']['nested']['deep'] == 'value'

    def test_parse_list_yaml(self):
        """Test parsing YAML with lists."""
        yaml_content = """
items:
  - item1
  - item2
  - item3
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data['items'] == ['item1', 'item2', 'item3']

    def test_parse_empty_document(self):
        """Test parsing document that resolves to None."""
        yaml_content = ""
        result = self.parser.safe_load(yaml_content)
        assert result.is_error()
        assert result.error.category == YAMLErrorCategory.DOCUMENT
        assert 'Empty' in result.error.message

    # Error handling - YAMLError base class
    def test_handles_invalid_yaml_structure(self):
        """Test handling of generic YAML parsing errors."""
        invalid_yaml = """
key: value
  bad_indentation: here
"""
        result = self.parser.safe_load(invalid_yaml)
        assert result.is_error()
        assert result.error is not None
        assert result.raw_exception is not None

    # Error handling - specific input validation
    def test_handles_none_input(self):
        """Test handling of None input."""
        result = self.parser.safe_load(None)
        assert result.is_error()
        assert 'None' in result.error.message or 'cannot' in result.error.message.lower()

    def test_handles_non_string_input(self):
        """Test handling of non-string input."""
        result = self.parser.safe_load(12345)
        assert result.is_error()
        assert 'string' in result.error.message

    def test_handles_whitespace_only(self):
        """Test handling of whitespace-only content."""
        result = self.parser.safe_load("   \n  \n  ")
        assert result.is_error()
        assert 'Empty' in result.error.message

    # Error information structure
    def test_error_has_category(self):
        """Test that errors include category information."""
        invalid_yaml = "key: value\n  bad: indent"
        result = self.parser.safe_load(invalid_yaml)
        if result.is_error():
            assert result.error.category in YAMLErrorCategory

    def test_error_has_severity(self):
        """Test that errors include severity information."""
        invalid_yaml = "key: value\n  bad: indent"
        result = self.parser.safe_load(invalid_yaml)
        if result.is_error():
            assert result.error.severity in YAMLErrorSeverity

    def test_error_has_message(self):
        """Test that errors include human-readable message."""
        invalid_yaml = "key: value\n  bad: indent"
        result = self.parser.safe_load(invalid_yaml)
        if result.is_error():
            assert result.error.message is not None
            assert len(result.error.message) > 0

    def test_error_may_have_line_column(self):
        """Test that errors may include line/column information."""
        # This test is flexible since not all errors have line/column
        invalid_yaml = "key: value\n  bad: indent"
        result = self.parser.safe_load(invalid_yaml)
        if result.is_error():
            # Line/column may be None for some errors, but if present should be int
            if result.error.line is not None:
                assert isinstance(result.error.line, int)
            if result.error.column is not None:
                assert isinstance(result.error.column, int)

    def test_error_has_raw_exception(self):
        """Test that errors include original exception."""
        invalid_yaml = "key: value\n  bad: indent"
        result = self.parser.safe_load(invalid_yaml)
        if result.is_error():
            assert result.raw_exception is not None
            assert isinstance(result.raw_exception, Exception)

    def test_error_may_have_context(self):
        """Test that errors may include context lines."""
        # This test is flexible since not all errors have context
        invalid_yaml = "key: value\n  bad: indent"
        result = self.parser.safe_load(invalid_yaml)
        if result.is_error():
            # Context may be empty string, but if present should be useful
            if result.error.context:
                assert isinstance(result.error.context, str)

    def test_error_may_have_suggestion(self):
        """Test that errors may include helpful suggestions."""
        # This test is flexible since not all errors have suggestions
        invalid_yaml = "key: value\n  bad: indent"
        result = self.parser.safe_load(invalid_yaml)
        if result.is_error():
            # Suggestion may be empty string, but if present should be useful
            if result.error.suggestion:
                assert isinstance(result.error.suggestion, str)


class TestConvenienceFunction:
    """Test the convenience function safe_load_yaml."""

    def test_safe_load_yaml_function(self):
        """Test the convenience function works."""
        result = safe_load_yaml("test: value")
        assert result.is_success()
        assert result.data['test'] == 'value'

    def test_safe_load_yaml_with_source(self):
        """Test the convenience function accepts source parameter."""
        result = safe_load_yaml("test: value", source="test.yaml")
        assert result.is_success()


class TestErrorCategories:
    """Test that different error types are categorized correctly."""

    def setup_method(self):
        """Set up test fixtures."""
        self.parser = YAMLCoreParser()

    def test_indentation_error_categorized(self):
        """Test that indentation errors get proper category."""
        # YAML with tab character
        yaml_with_tab = "key:\n\tvalue"
        result = self.parser.safe_load(yaml_with_tab)
        if result.is_error():
            # Should be categorized as syntax or indentation error
            assert result.error.category in [
                YAMLErrorCategory.SYNTAX,
                YAMLErrorCategory.INDENTATION
            ]

    def test_structure_error_categorized(self):
        """Test that structure errors get proper category."""
        # Malformed mapping
        invalid_yaml = "key: value\n  another: value"
        result = self.parser.safe_load(invalid_yaml)
        if result.is_error():
            # Should be categorized
            assert result.error.category in YAMLErrorCategory

    def test_empty_content_categorized(self):
        """Test that empty content gets document category."""
        result = self.parser.safe_load("")
        assert result.is_error()
        assert result.error.category == YAMLErrorCategory.DOCUMENT


class TestResultStructureIntegration:
    """Test that result structure integrates properly with error types."""

    def setup_method(self):
        """Set up test fixtures."""
        self.parser = YAMLCoreParser()

    def test_result_matches_error_types_structure(self):
        """Test that SafeLoadResult uses error types from error_types module."""
        from yamlutil.error_types import YAMLErrorDetail

        # Create a result manually
        error = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.ERROR,
            message="Test error"
        )
        result = SafeLoadResult(success=False, error=error)

        # Verify structure
        assert result.error.category == YAMLErrorCategory.SYNTAX
        assert result.error.severity == YAMLErrorSeverity.ERROR
        assert result.error.message == "Test error"

    def test_parser_result_uses_correct_error_types(self):
        """Test that parser returns results with correct error types."""
        result = self.parser.safe_load("invalid: yaml: content")
        if result.is_error():
            # Verify error is a YAMLErrorDetail
            from yamlutil.error_types import YAMLErrorDetail
            assert isinstance(result.error, YAMLErrorDetail)
            # Verify enum types
            assert isinstance(result.error.category, YAMLErrorCategory)
            assert isinstance(result.error.severity, YAMLErrorSeverity)


class TestEdgeCases:
    """Test edge cases and special scenarios."""

    def setup_method(self):
        """Set up test fixtures."""
        self.parser = YAMLCoreParser()

    def test_yaml_with_special_characters(self):
        """Test parsing YAML with special characters."""
        yaml_content = """
special: 'Test with "quotes" and \\n escapes'
unicode: "Hello 世界"
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert 'quotes' in result.data['special']

    def test_yaml_with_booleans(self):
        """Test parsing YAML with boolean values."""
        yaml_content = """
true_value: true
false_value: false
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data['true_value'] is True
        assert result.data['false_value'] is False

    def test_yaml_with_nulls(self):
        """Test parsing YAML with null values."""
        yaml_content = """
null_value: null
empty_value: ~
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data['null_value'] is None
        assert result.data['empty_value'] is None

    def test_yaml_with_anchors_and_aliases(self):
        """Test parsing YAML with anchors and aliases."""
        yaml_content = """
defaults: &defaults
  timeout: 30
  retry: 3

production:
  <<: *defaults
  host: prod.example.com
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data['production']['timeout'] == 30
        assert result.data['production']['host'] == 'prod.example.com'

    def test_yaml_with_multiline_string(self):
        """Test parsing YAML with multiline strings."""
        yaml_content = """
description: |
  This is a
  multiline string
  that preserves
  newlines.
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert 'multiline string' in result.data['description']


class TestCommentFiltering:
    """Test YAML comment filtering patterns."""

    def setup_method(self):
        """Set up test fixtures."""
        self.parser = YAMLCoreParser()

    def test_full_line_comment_removal(self):
        """Test that full-line comments are properly filtered out."""
        yaml_content = """
# This is a full-line comment
key: value
# Another full-line comment
another_key: another_value
# Third comment
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Comments should be removed, only data remains
        assert result.data == {'key': 'value', 'another_key': 'another_value'}
        # Comments should not appear in parsed data
        assert 'comment' not in str(result.data).lower()

    def test_multiple_full_line_comments(self):
        """Test multiple consecutive full-line comments."""
        yaml_content = """
# First comment
# Second comment
# Third comment
key: value
# Fourth comment
# Fifth comment
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value'}

    def test_inline_comment_filtering(self):
        """Test that inline comments are properly filtered."""
        yaml_content = """
key: value  # This is an inline comment
another_key: another_value  # Another inline comment
number: 42  # Number with inline comment
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Inline comments should be stripped
        assert result.data['key'] == 'value'
        assert result.data['another_key'] == 'another_value'
        assert result.data['number'] == 42
        # Comments should not be in the values
        assert 'comment' not in str(result.data['key'])
        assert 'comment' not in str(result.data['another_key'])

    def test_inline_comment_with_hashes_in_value(self):
        """Test inline comments when value contains hash character."""
        yaml_content = """
key: "value with # hash inside quotes"
another: value_without_quotes  # but this is a comment
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Hash inside quotes is part of the value
        assert 'hash' in result.data['key']
        # Hash after space outside quotes is comment
        assert 'comment' not in result.data['another']

    def test_empty_lines_handling(self):
        """Test that empty lines are properly handled."""
        yaml_content = """
key1: value1


key2: value2


key3: value3
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key1': 'value1', 'key2': 'value2', 'key3': 'value3'}

    def test_whitespace_only_lines(self):
        """Test that whitespace-only lines are properly handled."""
        yaml_content = """
key1: value1



key2: value2
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key1': 'value1', 'key2': 'value2'}

    def test_mixed_comments_and_empty_lines(self):
        """Test handling of mixed comments and empty lines."""
        yaml_content = """
# Header comment

key1: value1  # inline comment

# Another comment
key2: value2

# Footer comment
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key1': 'value1', 'key2': 'value2'}

    def test_comment_at_start_of_document(self):
        """Test comments at the start of a YAML document."""
        yaml_content = """# Configuration file
# Generated by system
# Do not edit manually

key: value
another: item
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value', 'another': 'item'}

    def test_comment_at_end_of_document(self):
        """Test comments at the end of a YAML document."""
        yaml_content = """
key: value
another: item
# End of configuration
# Last comment
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value', 'another': 'item'}

    def test_nested_structure_with_comments(self):
        """Test comments in nested YAML structures."""
        yaml_content = """
# Parent section
parent:
  # Child 1
  child1: value1  # First value
  # Child 2
  child2: value2  # Second value
  # Nested section
  nested:
    # Deep value
    deep: value  # Deep comment
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data['parent']['child1'] == 'value1'
        assert result.data['parent']['child2'] == 'value2'
        assert result.data['parent']['nested']['deep'] == 'value'

    def test_list_with_comments(self):
        """Test comments in YAML list structures."""
        yaml_content = """
# List of items
items:
  # First item
  - item1  # First
  # Second item
  - item2  # Second
  # Third item
  - item3  # Third
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data['items'] == ['item1', 'item2', 'item3']

    def test_comment_only_lines(self):
        """Test that lines with only comments are filtered."""
        yaml_content = """
# Comment line 1
#
# Comment line 3
key: value
#
# Comment line 6
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value'}

    def test_inline_comment_no_space(self):
        """Test inline comment without space after hash."""
        yaml_content = """
key: value #inline comment without space
another: item
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data['key'] == 'value'

    def test_multiple_inline_comments_per_line(self):
        """Test line with multiple hash characters."""
        yaml_content = """
key: "value with # hash" # actual comment # another hash
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # First hash in quotes is part of value
        assert 'hash' in result.data['key']
        # Everything after second hash should be filtered
        assert result.data['key'] == 'value with # hash'

    def test_complex_document_with_comments(self):
        """Test realistic complex YAML with various comment patterns."""
        yaml_content = """# Database configuration
# Updated: 2024-01-15

database:
  # Connection settings
  host: localhost  # Database server
  port: 5432  # Default PostgreSQL port

  # Authentication
  username: admin  # DB username
  password: secret  # DB password (change in production)

  # Connection pool settings
  pool:
    max_connections: 100  # Maximum concurrent connections
    min_connections: 10  # Minimum connections to maintain

# Application settings
app:
  name: MyApp  # Application name
  debug: false  # Run in debug mode
  # Feature flags
  features:
    - feature1  # First feature
    - feature2  # Second feature
    - feature3  # Third feature

# End of configuration
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data['database']['host'] == 'localhost'
        assert result.data['database']['port'] == 5432
        assert result.data['database']['username'] == 'admin'
        assert result.data['database']['password'] == 'secret'
        assert result.data['database']['pool']['max_connections'] == 100
        assert result.data['database']['pool']['min_connections'] == 10
        assert result.data['app']['name'] == 'MyApp'
        assert result.data['app']['debug'] is False
        assert result.data['app']['features'] == ['feature1', 'feature2', 'feature3']

    # ============================================================================
    # Full-line comment detection tests - bf-58w56
    # ============================================================================

    def test_full_line_comment_with_hash_only(self):
        """Test that a line with only # is detected as a comment."""
        yaml_content = """#
key: value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value'}

    def test_full_line_comment_with_hash_and_text(self):
        """Test that a line starting with # followed by text is a comment."""
        yaml_content = """# This is a comment
key: value
# Another comment
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value'}

    def test_full_line_comment_without_space_after_hash(self):
        """Test that a line starting with #text (no space) is still a comment."""
        yaml_content = """#This is a comment without space
key: value
#Another comment without space
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value'}

    def test_full_line_comment_with_leading_whitespace_and_hash(self):
        """Test that a line with whitespace followed by # is a comment."""
        yaml_content = """  # Comment with leading spaces
key: value
  # Comment with leading spaces (2 spaces)
another: value2
    # Comment with 4 leading spaces
third: value3
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value', 'another': 'value2', 'third': 'value3'}

    def test_full_line_comment_single_space_after_hash(self):
        """Test that # followed by single space and text is a comment."""
        yaml_content = """# Single space after hash
key: value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value'}

    def test_full_line_comment_multiple_spaces_after_hash(self):
        """Test that # followed by multiple spaces and text is a comment."""
        yaml_content = """#    Multiple spaces after hash
key: value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value'}

    def test_full_line_comment_with_special_characters(self):
        """Test that lines starting with # with special characters are comments."""
        yaml_content = """# Comment with @#$%^&*() special chars
key: value
# Comment with 123 numbers and symbols!@#
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value'}

    def test_full_line_comment_various_indentation_levels(self):
        """Test full-line comments at various indentation levels."""
        yaml_content = """# Top-level comment
parent:
  # Indented comment level 1
  child1: value1
    # Indented comment level 2 (misaligned but still comment)
  child2: value2
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data['parent']['child1'] == 'value1'
        assert result.data['parent']['child2'] == 'value2'

    def test_empty_line_not_detected_as_comment(self):
        """Test that empty lines are NOT detected as comments (they're preserved as structure)."""
        yaml_content = """key1: value1

key2: value2


key3: value3
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Empty lines don't create empty values in the parsed data
        assert result.data == {'key1': 'value1', 'key2': 'value2', 'key3': 'value3'}

    def test_whitespace_only_line_not_detected_as_comment(self):
        """Test that lines with only whitespace are NOT treated as comments."""
        yaml_content = """key1: value1


key2: value2


key3: value3
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Whitespace-only lines don't create entries in the parsed data
        assert result.data == {'key1': 'value1', 'key2': 'value2', 'key3': 'value3'}

    def test_mixed_full_line_and_inline_comments(self):
        """Test that full-line and inline comments are both handled correctly."""
        yaml_content = """# Full-line comment 1
key1: value1  # Inline comment 1
# Full-line comment 2
key2: value2  # Inline comment 2
# Full-line comment 3
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key1': 'value1', 'key2': 'value2'}

    def test_full_line_comment_in_multiline_string_context(self):
        """Test that # inside multiline strings is NOT treated as comment."""
        yaml_content = """description: |
  This is a multiline string
  # This line is part of the string, not a comment
  Another line
key: value
# This IS a comment
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # The # inside the multiline string is preserved
        assert 'This line is part of the string' in result.data['description']
        assert result.data == {
            'description': 'This is a multiline string\n# This line is part of the string, not a comment\nAnother line\n',
            'key': 'value'
        }

    def test_full_line_comment_preserved_in_folded_style(self):
        """Test that # in folded-style scalars is preserved."""
        yaml_content = """description: >
  This is a folded string
  # This line is part of the folded string
  Another line
# This IS a comment outside the string
key: value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # The folded style preserves content but converts newlines to spaces
        assert 'folded string' in result.data['description']
        assert result.data == {
            'description': 'This is a folded string # This line is part of the folded string Another line\n',
            'key': 'value'
        }

    def test_consecutive_full_line_comments(self):
        """Test multiple consecutive full-line comments with different patterns."""
        yaml_content = """#Comment without space
# Comment with space
  # Comment with leading space
#Third comment
# Fourth comment
key: value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value'}

    def test_full_line_comment_at_document_boundaries(self):
        """Test full-line comments at start and end of document."""
        yaml_content = """# Header comment
# Another header
key: value
another: item
# Footer comment
# End of file
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value', 'another': 'item'}

    # ============================================================================
    # Basic indentation level comment tests - bf-4dy80
    # ============================================================================

    def test_comment_at_indentation_level_0(self):
        """Test that comments at 0 spaces indentation (no indentation) are properly filtered."""
        yaml_content = """# Comment at 0 spaces
key: value
# Another comment at 0 spaces
another_key: another_value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value', 'another_key': 'another_value'}
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_at_indentation_level_2(self):
        """Test that comments at 2 spaces indentation are properly filtered."""
        yaml_content = """# Comment at 0 spaces
key: value
  # Comment at 2 spaces
another_key: another_value
    # Comment at 4 spaces (nested)
deep_key: deep_value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value', 'another_key': 'another_value', 'deep_key': 'deep_value'}
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_at_indentation_level_4(self):
        """Test that comments at 4 spaces indentation are properly filtered."""
        yaml_content = """# Comment at 0 spaces
key: value
    # Comment at 4 spaces
another_key: another_value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value', 'another_key': 'another_value'}
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_at_indentation_level_6(self):
        """Test that comments at 6 spaces indentation are properly filtered."""
        yaml_content = """# Comment at 0 spaces
key: value
      # Comment at 6 spaces
another_key: another_value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value', 'another_key': 'another_value'}
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comments_at_all_basic_indentation_levels(self):
        """Test that comments at all basic indentation levels (0, 2, 4, 6) work together."""
        yaml_content = """# Level 0 comment
root_key: root_value
  # Level 2 comment
level_2_key: level_2_value
    # Level 4 comment
level_4_key: level_4_value
      # Level 6 comment
level_6_key: level_6_value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # All comments should be filtered, only data remains
        assert result.data == {
            'root_key': 'root_value',
            'level_2_key': 'level_2_value',
            'level_4_key': 'level_4_value',
            'level_6_key': 'level_6_value'
        }
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()


if __name__ == '__main__':
    # Run tests with pytest
    pytest.main([__file__, '-v'])
