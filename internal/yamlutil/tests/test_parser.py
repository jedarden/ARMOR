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

    # ============================================================================
    # Deep indentation level comment tests - bf-455dc
    # ============================================================================

    def test_comment_at_indentation_level_8(self):
        """Test that comments at 8 spaces indentation are properly filtered."""
        yaml_content = """# Comment at 0 spaces
key: value
        # Comment at 8 spaces
another_key: another_value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value', 'another_key': 'another_value'}
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_at_indentation_level_10(self):
        """Test that comments at 10 spaces indentation are properly filtered."""
        yaml_content = """# Comment at 0 spaces
key: value
          # Comment at 10 spaces
another_key: another_value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value', 'another_key': 'another_value'}
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_at_indentation_level_12(self):
        """Test that comments at 12 spaces indentation are properly filtered."""
        yaml_content = """# Comment at 0 spaces
key: value
            # Comment at 12 spaces
another_key: another_value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        assert result.data == {'key': 'value', 'another_key': 'another_value'}
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comments_at_all_deep_indentation_levels(self):
        """Test that comments at all deep indentation levels (8, 10, 12) work together."""
        yaml_content = """# Level 0 comment
root_key: root_value
        # Level 8 comment
level_8_key: level_8_value
          # Level 10 comment
level_10_key: level_10_value
            # Level 12 comment
level_12_key: level_12_value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # All comments should be filtered, only data remains
        assert result.data == {
            'root_key': 'root_value',
            'level_8_key': 'level_8_value',
            'level_10_key': 'level_10_value',
            'level_12_key': 'level_12_value'
        }
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comments_combined_basic_and_deep_levels(self):
        """Test that comments at all indentation levels (0, 2, 4, 6, 8, 10, 12) work together."""
        yaml_content = """# Level 0 comment
root_key: root_value
  # Level 2 comment
level_2_key: level_2_value
    # Level 4 comment
level_4_key: level_4_value
      # Level 6 comment
level_6_key: level_6_value
        # Level 8 comment
level_8_key: level_8_value
          # Level 10 comment
level_10_key: level_10_value
            # Level 12 comment
level_12_key: level_12_value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # All comments should be filtered, only data remains
        assert result.data == {
            'root_key': 'root_value',
            'level_2_key': 'level_2_value',
            'level_4_key': 'level_4_value',
            'level_6_key': 'level_6_value',
            'level_8_key': 'level_8_value',
            'level_10_key': 'level_10_value',
            'level_12_key': 'level_12_value'
        }
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_deeply_nested_structure_with_comments(self):
        """Test comments in a deeply nested YAML structure with realistic data."""
        yaml_content = """# Top-level configuration
database:
  # Connection settings
  connection:
    # Primary database
    primary:
      # Host configuration
      host: localhost  # Database host
      port: 5432  # Database port
      # Authentication settings
      credentials:
        # Username
        username: admin  # DB username
        # Password
        password: secret  # DB password (change in production)
      # Fallback connection
      fallback: true  # Use fallback connection
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify deeply nested data is parsed correctly
        assert result.data['database']['connection']['primary']['host'] == 'localhost'
        assert result.data['database']['connection']['primary']['port'] == 5432
        assert result.data['database']['connection']['primary']['credentials']['username'] == 'admin'
        assert result.data['database']['connection']['primary']['credentials']['password'] == 'secret'
        assert result.data['database']['connection']['primary']['fallback'] is True
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()


class TestNestedStructureCommentDetection:
    """Test YAML comment detection in nested structures - bf-48lyo."""

    def setup_method(self):
        """Set up test fixtures."""
        self.parser = YAMLCoreParser()

    def test_comment_in_nested_map(self):
        """Test that comments in nested map structures are properly filtered."""
        yaml_content = """# Top-level comment
outer_map:
  # Comment for inner map 1
  inner_map_1:
    # Comment for key in inner map 1
    key1: value1  # Inline comment for value1
    key2: value2
  # Comment for inner map 2
  inner_map_2:
    # Comment for key in inner map 2
    key3: value3
    key4: value4  # Inline comment for value4
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify nested map structure is parsed correctly
        assert result.data['outer_map']['inner_map_1']['key1'] == 'value1'
        assert result.data['outer_map']['inner_map_1']['key2'] == 'value2'
        assert result.data['outer_map']['inner_map_2']['key3'] == 'value3'
        assert result.data['outer_map']['inner_map_2']['key4'] == 'value4'
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_in_nested_list(self):
        """Test that comments in nested list structures are properly filtered."""
        yaml_content = """# Top-level comment
outer_list:
  # Comment for inner list 1
  - - item1_1  # Inline comment for item1_1
    - item1_2
    - item1_3
  # Comment for inner list 2
  - - item2_1
    - item2_2
    - item2_3  # Inline comment for item2_3
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify nested list structure is parsed correctly
        assert result.data['outer_list'][0] == ['item1_1', 'item1_2', 'item1_3']
        assert result.data['outer_list'][1] == ['item2_1', 'item2_2', 'item2_3']
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_in_map_within_list_structure(self):
        """Test that comments in map-within-list structures are properly filtered."""
        yaml_content = """# Configuration list
configs:
  # First config
  - name: config1  # Config 1 name
    enabled: true  # Config 1 enabled
    # Nested settings for config 1
    settings:
      timeout: 30  # Timeout value
      retry: 3  # Retry count
  # Second config
  - name: config2
    enabled: false
    # Nested settings for config 2
    settings:
      timeout: 60
      retry: 5  # Retry count for config 2
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify map-within-list structure is parsed correctly
        assert result.data['configs'][0]['name'] == 'config1'
        assert result.data['configs'][0]['enabled'] is True
        assert result.data['configs'][0]['settings']['timeout'] == 30
        assert result.data['configs'][0]['settings']['retry'] == 3
        assert result.data['configs'][1]['name'] == 'config2'
        assert result.data['configs'][1]['enabled'] is False
        assert result.data['configs'][1]['settings']['timeout'] == 60
        assert result.data['configs'][1]['settings']['retry'] == 5
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_in_list_within_map_structure(self):
        """Test that comments in list-within-map structures are properly filtered."""
        yaml_content = """# Service configuration
service:
  # Service name
  name: my-service  # Service name value
  # Service ports
  ports:
    # HTTP port
    - 80  # HTTP port number
    # HTTPS port
    - 443  # HTTPS port number
    # Admin port
    - 8080  # Admin port number
  # Service endpoints
  endpoints:
    # Health check endpoint
    - /health  # Health check path
    # Metrics endpoint
    - /metrics  # Metrics path
    # API endpoint
    - /api  # API path
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify list-within-map structure is parsed correctly
        assert result.data['service']['name'] == 'my-service'
        assert result.data['service']['ports'] == [80, 443, 8080]
        assert result.data['service']['endpoints'] == ['/health', '/metrics', '/api']
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_deeply_nested_map_with_comments(self):
        """Test comments in deeply nested map structures (3+ levels)."""
        yaml_content = """# Level 0 comment
database:
  # Level 1 comment
  connection:
    # Level 2 comment
    primary:
      # Level 3 comment
      credentials:
        # Level 4 comment
        username: admin  # Username
        password: secret  # Password
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify deeply nested structure is parsed correctly
        assert result.data['database']['connection']['primary']['credentials']['username'] == 'admin'
        assert result.data['database']['connection']['primary']['credentials']['password'] == 'secret'
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_deeply_nested_list_with_comments(self):
        """Test comments in deeply nested list structures (3+ levels)."""
        yaml_content = """# Level 0 comment
matrix:
  # Level 1 comment
  - # Level 2 comment
    - # Level 3 comment
      - deep_item_1  # Deep item 1
      - deep_item_2  # Deep item 2
      - deep_item_3
      # Level 3 comment for second group
      - deep_item_4
      - deep_item_5
  # Level 1 comment for second row
  - - another_row_item
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify deeply nested list structure is parsed correctly
        # Actual structure: 3-level nested list
        assert result.data['matrix'][0][0] == ['deep_item_1', 'deep_item_2', 'deep_item_3', 'deep_item_4', 'deep_item_5']
        assert result.data['matrix'][1] == ['another_row_item']
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_complex_nested_structure_with_mixed_comments(self):
        """Test realistic complex nested structure with various comment patterns."""
        yaml_content = """# Application configuration
app:
  # App metadata
  name: MyApp  # Application name
  version: 1.0  # Version number

  # Cluster configuration (list within map)
  clusters:
    # Primary cluster
    - name: primary  # Primary cluster name
      region: us-east-1  # Region
      # Nodes configuration (map within list within map)
      nodes:
        master:
          count: 3  # Master node count
          size: large  # Master node size
        worker:
          count: 10  # Worker node count
          size: medium  # Worker node size
    # Secondary cluster
    - name: secondary
      region: us-west-2
      nodes:
        master:
          count: 2
          size: large
        worker:
          count: 5  # Worker count for secondary
          size: small

  # Feature flags (list within map)
  features:
    # Feature list items
    - feature_a  # Feature A
    - feature_b  # Feature B
    - feature_c
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify complex nested structure is parsed correctly
        assert result.data['app']['name'] == 'MyApp'
        assert result.data['app']['version'] == 1.0
        assert result.data['app']['clusters'][0]['name'] == 'primary'
        assert result.data['app']['clusters'][0]['region'] == 'us-east-1'
        assert result.data['app']['clusters'][0]['nodes']['master']['count'] == 3
        assert result.data['app']['clusters'][0]['nodes']['worker']['size'] == 'medium'
        assert result.data['app']['clusters'][1]['name'] == 'secondary'
        assert result.data['app']['clusters'][1]['nodes']['worker']['count'] == 5
        assert result.data['app']['features'] == ['feature_a', 'feature_b', 'feature_c']
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()


class TestMixedScenarios:
    """Test YAML comment detection in mixed scenarios - bf-3f73z.

    These tests cover comments alongside regular values, anchors (&), aliases (*),
    and documents containing all three elements together.
    """

    def setup_method(self):
        """Set up test fixtures."""
        self.parser = YAMLCoreParser()

    def test_comment_with_anchor_definition(self):
        """Test that comments work alongside anchor definitions (&anchor)."""
        yaml_content = """# Default configuration
defaults: &defaults
  timeout: 30  # Connection timeout
  retry: 3  # Number of retries

# Production environment
production:
  <<: *defaults
  host: prod.example.com  # Production host
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify anchor merge worked
        assert result.data['production']['timeout'] == 30
        assert result.data['production']['retry'] == 3
        assert result.data['production']['host'] == 'prod.example.com'
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_multiple_anchors(self):
        """Test comments with multiple anchor definitions and aliases."""
        yaml_content = """# Base configuration
base: &base
  enabled: true  # Feature enabled by default
  debug: false  # Debug mode off

# Extended configuration
extended: &extended
  <<: *base  # Merge base settings
  timeout: 60  # Custom timeout

# Production uses extended config
production:
  <<: *extended
  host: prod.example.com  # Production server
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify multi-level anchor merge worked
        assert result.data['production']['enabled'] is True
        assert result.data['production']['debug'] is False
        assert result.data['production']['timeout'] == 60
        assert result.data['production']['host'] == 'prod.example.com'
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_alias_reference(self):
        """Test comments with alias references (*alias)."""
        yaml_content = """# Common settings
common: &common
  timeout: 30  # Timeout in seconds
  retries: 3  # Retry attempts

# Development environment
development:
  settings: *common  # Use common settings
  env: dev  # Development environment

# Staging environment
staging:
  settings: *common  # Use common settings
  env: staging  # Staging environment
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify alias reference worked (both point to same object reference)
        assert result.data['development']['settings']['timeout'] == 30
        assert result.data['staging']['settings']['timeout'] == 30
        assert result.data['development']['env'] == 'dev'
        assert result.data['staging']['env'] == 'staging'
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_merge_and_override(self):
        """Test comments with merge keys that override values."""
        yaml_content = """# Default configuration
default_config: &default_config
  timeout: 30  # Default timeout
  port: 8080  # Default port
  enabled: true  # Enabled by default

# Custom configuration with overrides
custom_config:
  <<: *default_config  # Merge defaults
  timeout: 60  # Override timeout
  port: 9090  # Override port
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify override worked (custom values take precedence)
        assert result.data['custom_config']['timeout'] == 60
        assert result.data['custom_config']['port'] == 9090
        assert result.data['custom_config']['enabled'] is True  # Inherited from default
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_nested_anchor_in_list(self):
        """Test comments with anchors within list items."""
        yaml_content = """# Item template
item_template: &item_template
  name: default  # Default name
  count: 1  # Default count

# Items list using template
items:
  # First item - uses template
  - <<: *item_template
    name: item1  # Override name
    id: 1  # Unique ID
  # Second item - uses template
  - <<: *item_template
    name: item2  # Override name
    id: 2  # Unique ID
  # Third item - custom without template
  - name: item3  # Custom item
    count: 5  # Custom count
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify nested anchor in list worked
        assert result.data['items'][0]['name'] == 'item1'
        assert result.data['items'][0]['count'] == 1  # Inherited from template
        assert result.data['items'][0]['id'] == 1
        assert result.data['items'][1]['name'] == 'item2'
        assert result.data['items'][2]['name'] == 'item3'
        assert result.data['items'][2]['count'] == 5
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_complex_document_with_all_elements(self):
        """Test realistic complex document with values, comments, and anchors."""
        yaml_content = """# Application configuration file
# Version: 1.0

# Base service configuration
base_service: &base_service
  enabled: true  # Services enabled by default
  timeout: 30  # Default timeout (seconds)
  retries: 3  # Retry attempts

# Services configuration
services:
  # Web service
  web:
    <<: *base_service  # Use base config
    port: 80  # HTTP port
    ssl_port: 443  # HTTPS port
    endpoints:  # API endpoints
      - /api  # Main API
      - /health  # Health check

  # Database service
  database:
    <<: *base_service  # Use base config
    port: 5432  # PostgreSQL port
    host: localhost  # DB host
    credentials:  # DB credentials
      username: admin  # DB username
      password: secret  # DB password

  # Cache service
  cache:
    <<: *base_service
    port: 6379  # Redis port
    host: localhost  # Cache host

# Feature flags
features:
  # Feature A - enabled
  - name: feature_a  # Feature A name
    enabled: true  # Feature A enabled
  # Feature B - disabled
  - name: feature_b  # Feature B name
    enabled: false  # Feature B disabled
  # Feature C - enabled with custom config
  - name: feature_c  # Feature C name
    enabled: true  # Feature C enabled
    config:  # Custom config for feature C
      timeout: 60  # Custom timeout
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify complex structure with all elements
        assert result.data['services']['web']['enabled'] is True
        assert result.data['services']['web']['timeout'] == 30
        assert result.data['services']['web']['port'] == 80
        assert result.data['services']['web']['endpoints'] == ['/api', '/health']
        assert result.data['services']['database']['port'] == 5432
        assert result.data['services']['database']['credentials']['username'] == 'admin'
        assert result.data['services']['cache']['port'] == 6379
        assert result.data['features'][0]['name'] == 'feature_a'
        assert result.data['features'][0]['enabled'] is True
        assert result.data['features'][1]['enabled'] is False
        assert result.data['features'][2]['config']['timeout'] == 60
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_anchor_in_nested_structure(self):
        """Test comments with anchors in deeply nested structures."""
        yaml_content = """# Level 0 comment
database: &database_config
  # Level 1 comment
  connection:
    # Level 2 comment
    primary:
      # Level 3 comment
      host: localhost  # Database host
      port: 5432  # Database port
      # Level 4 comment
      credentials:
        # Level 5 comment
        username: admin  # DB username
        password: secret  # DB password

# Use same config for replica
replica:
  <<: *database_config  # Reuse database config
  connection:
    primary:
      host: replica.example.com  # Replica host
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify nested structure with anchor
        assert result.data['database']['connection']['primary']['host'] == 'localhost'
        assert result.data['database']['connection']['primary']['port'] == 5432
        assert result.data['database']['connection']['primary']['credentials']['username'] == 'admin'
        assert result.data['replica']['connection']['primary']['port'] == 5432  # Inherited
        assert result.data['replica']['connection']['primary']['host'] == 'replica.example.com'  # Overridden
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_multiple_aliases_same_anchor(self):
        """Test comments when multiple aliases reference the same anchor."""
        yaml_content = """# Common schema definition
schema: &schema
  type: string  # Field type
  required: true  # Required field

# User fields
user:
  username:  # Username field
    <<: *schema  # Use schema
    min_length: 3  # Min 3 characters
  email:  # Email field
    <<: *schema  # Use schema
    format: email  # Email format

# Product fields
product:
  name:  # Product name
    <<: *schema  # Use schema
    max_length: 100  # Max 100 characters
  description:  # Product description
    <<: *schema  # Use schema
    required: false  # Optional field
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify multiple aliases to same anchor
        assert result.data['user']['username']['type'] == 'string'
        assert result.data['user']['username']['required'] is True
        assert result.data['user']['username']['min_length'] == 3
        assert result.data['user']['email']['format'] == 'email'
        assert result.data['product']['name']['max_length'] == 100
        assert result.data['product']['description']['required'] is False
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_inline_anchor_and_alias(self):
        """Test comments with inline anchor/alias syntax."""
        yaml_content = """# Inline anchor example
defaults: &defaults {timeout: 30, retry: 3}  # Default settings

# Use inline alias
production:
  settings: *defaults  # Reference defaults
  host: prod.example.com  # Production host

staging:
  settings: *defaults  # Reference defaults
  host: staging.example.com  # Staging host
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify inline anchor/alias worked
        assert result.data['production']['settings']['timeout'] == 30
        assert result.data['staging']['settings']['retry'] == 3
        assert result.data['production']['host'] == 'prod.example.com'
        assert result.data['staging']['host'] == 'staging.example.com'
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_edge_case_anchor_without_alias(self):
        """Test comments when anchor is defined but never used."""
        yaml_content = """# Unused anchor definition
unused_config: &unused_config
  timeout: 30  # This will never be referenced
  retries: 3  # This will never be referenced

# Regular configuration
actual_config:
  timeout: 60  # Actual timeout
  retries: 5  # Actual retries
  host: localhost  # Actual host
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify unused anchor is still parsed
        assert result.data['unused_config']['timeout'] == 30
        assert result.data['actual_config']['timeout'] == 60
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_edge_case_alias_without_anchor(self):
        """Test comments when alias references non-existent anchor (should fail)."""
        yaml_content = """# Valid config
valid_key: valid_value  # Valid key-value pair

# Invalid alias reference (undefined anchor)
invalid:
  <<: *undefined_anchor  # This references non-existent anchor
"""
        result = self.parser.safe_load(yaml_content)
        # This should be an error - undefined anchor
        assert result.is_error()
        # Error should mention the undefined anchor
        assert 'anchor' in result.error.message.lower() or 'not found' in result.error.message.lower()

    def test_comment_with_array_anchor_and_alias(self):
        """Test comments with anchors/aliases on array elements."""
        yaml_content = """# Common tasks list
common_tasks: &common_tasks
  - task1  # First common task
  - task2  # Second common task

# Deployment tasks
deployment:
  tasks:  # Deployment tasks
    - <<: *common_tasks  # Include common tasks
    - deploy_build  # Deployment-specific task
    - run_tests  # Test after deployment

# Development tasks
development:
  tasks:  # Development tasks
    - <<: *common_tasks  # Include common tasks
    - run_linter  # Development-specific task
    - format_code  # Format code
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify array anchor/alias worked
        # Note: YAML's << merge key works with maps, not arrays directly
        # The structure will be parsed as-is
        assert result.data['common_tasks'] == ['task1', 'task2']
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_conditional_anchor_merge(self):
        """Test comments with conditional anchor merge patterns."""
        yaml_content = """# Base configuration
base: &base
  timeout: 30  # Base timeout
  retries: 3  # Base retries

# Configuration 1 - uses base
config1:
  <<: *base  # Merge base
  custom: value1  # Custom value

# Configuration 2 - uses base with override
config2:
  <<: *base  # Merge base
  timeout: 60  # Override timeout
  custom: value2  # Custom value

# Configuration 3 - no base
config3:
  timeout: 90  # Custom timeout only
  custom: value3  # Custom value
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify conditional merge worked
        assert result.data['config1']['timeout'] == 30  # From base
        assert result.data['config1']['retries'] == 3  # From base
        assert result.data['config1']['custom'] == 'value1'
        assert result.data['config2']['timeout'] == 60  # Overridden
        assert result.data['config2']['retries'] == 3  # From base
        assert result.data['config3']['timeout'] == 90  # Custom
        assert result.data['config3'].get('retries') is None  # No base merged
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_flow_collections_and_anchors(self):
        """Test comments with flow collections containing anchors."""
        yaml_content = """# Flow map with anchor
defaults: &defaults
  items: [item1, item2, item3]  # Default items
  config: {key1: val1, key2: val2}  # Default config

# Use anchor with flow collections
production:
  <<: *defaults  # Merge defaults
  items: [prod1, prod2, prod3]  # Override items
  config: {key1: prod_val1, key3: prod_val3}  # Override config
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify flow collections with anchors work
        assert result.data['production']['items'] == ['prod1', 'prod2', 'prod3']
        assert result.data['production']['config']['key1'] == 'prod_val1'
        assert result.data['production']['config']['key3'] == 'prod_val3'
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_various_scalar_types_and_anchors(self):
        """Test comments with all scalar types mixed with anchors."""
        yaml_content = """# Mixed scalar types with anchor
scalars: &scalars
  string_val: "hello"  # String value
  int_val: 42  # Integer value
  float_val: 3.14  # Float value
  bool_val: true  # Boolean value
  null_val: null  # Null value
  # List value
  list_val:
    - item1  # First item
    - item2  # Second item
  # Map value
  map_val:
    key1: value1  # Key 1
    key2: value2  # Key 2

# Reuse scalar types with overrides
override:
  <<: *scalars  # Merge scalars
  string_val: "world"  # Override string
  int_val: 100  # Override integer
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify all scalar types with anchors work
        assert result.data['override']['string_val'] == 'world'
        assert result.data['override']['int_val'] == 100
        assert result.data['override']['float_val'] == 3.14  # Inherited
        assert result.data['override']['bool_val'] is True  # Inherited
        assert result.data['override']['null_val'] is None  # Inherited
        assert result.data['override']['list_val'] == ['item1', 'item2']
        assert result.data['override']['map_val'] == {'key1': 'value1', 'key2': 'value2'}
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_edge_case_hash_in_anchored_values(self):
        """Test hash character handling in anchored values with comments."""
        yaml_content = """# Anchor with hash in values
url_config: &url_config
  base_url: "http://example.com"  # Base URL
  endpoint: "/api#endpoint"  # Endpoint with anchor
  full_url: "http://example.com/api#anchor"  # Full URL with anchor

# Use anchor with hash values
production:
  <<: *url_config  # Merge URL config
  # Override specific values
  base_url: "http://prod.example.com"  # Production base URL

staging:
  <<: *url_config  # Merge URL config
  base_url: "http://staging.example.com"  # Staging base URL
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify hash in anchored values is preserved
        assert 'endpoint' in result.data['url_config']
        assert '#anchor' in result.data['url_config']['full_url']
        assert result.data['production']['endpoint'] == '/api#endpoint'  # Inherited
        assert result.data['production']['base_url'] == 'http://prod.example.com'
        assert result.data['staging']['full_url'] == 'http://example.com/api#anchor'
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_multiline_and_anchors(self):
        """Test comments with multiline strings alongside anchors."""
        yaml_content = """# Multiline string with anchor
description_template: &desc_template
  short: "Default description"  # Short description
  # Multiline description
  long: |
    This is a long description
    that spans multiple lines
    and includes # hash characters
    which should be preserved.

# Use template for product
product:
  <<: *desc_template  # Merge description template
  short: "Product A"  # Override short description
  # Long description inherited

# Use template for service
service:
  <<: *desc_template  # Merge description template
  short: "Service B"  # Override short description
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify multiline strings with anchors work
        assert 'long description' in result.data['description_template']['long']
        assert result.data['product']['short'] == 'Product A'
        assert 'long description' in result.data['product']['long']  # Inherited
        assert result.data['service']['short'] == 'Service B'
        assert 'hash characters' in result.data['service']['long']  # Inherited
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_merge_key_variations_and_lists(self):
        """Test comments with complex merge key scenarios including lists."""
        yaml_content = """# Multiple anchor definitions
defaults: &defaults
  timeout: 30  # Default timeout
  retries: 3  # Default retries

list_defaults: &list_defaults
  - item1  # Default item 1
  - item2  # Default item 2

map_defaults: &map_defaults
  key1: value1  # Default key 1
  key2: value2  # Default key 2

# Complex merge scenario
complex_config:
  <<: [*defaults, *map_defaults]  # Merge both defaults (maps)
  timeout: 60  # Override timeout
  # List values
  items: *list_defaults  # Reference list defaults
  custom_list:  # Custom list
    - custom1  # Custom item 1
    - custom2  # Custom item 2
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify complex merge scenario works
        assert result.data['complex_config']['timeout'] == 60
        assert result.data['complex_config']['retries'] == 3
        assert result.data['complex_config']['key1'] == 'value1'
        assert result.data['complex_config']['items'] == ['item1', 'item2']
        assert result.data['complex_config']['custom_list'] == ['custom1', 'custom2']
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_deeply_nested_merge_and_overrides(self):
        """Test comments with deeply nested merge chains and multiple overrides."""
        yaml_content = """# Level 0 base
level0: &level0
  key0: value0  # Level 0 key
  # Level 1 nested
  level1: &level1
    key1: value1  # Level 1 key
    # Level 2 nested
    level2: &level2
      key2: value2  # Level 2 key
      # Level 3 nested
      level3: &level3
        key3: value3  # Level 3 key
        key4: value4  # Another level 3 key

# Complex nested merge
nested_config:
  <<: *level0  # Merge level 0
  # Override at various levels
  key0: overridden0  # Override level 0
  level1:
    <<: *level1  # Merge level 1
    key1: overridden1  # Override level 1
    level2:
      <<: *level2  # Merge level 2
      key2: overridden2  # Override level 2
      level3:
        <<: *level3  # Merge level 3
        key3: overridden3  # Override level 3
        # key4 inherited from level3
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify deeply nested merge with overrides
        assert result.data['nested_config']['key0'] == 'overridden0'
        assert result.data['nested_config']['level1']['key1'] == 'overridden1'
        assert result.data['nested_config']['level1']['level2']['key2'] == 'overridden2'
        assert result.data['nested_config']['level1']['level2']['level3']['key3'] == 'overridden3'
        assert result.data['nested_config']['level1']['level2']['level3']['key4'] == 'value4'  # Inherited
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()

    def test_comment_with_empty_and_null_values_in_anchors(self):
        """Test comments with empty strings and nulls in anchored values."""
        yaml_content = """# Anchor with null/empty values
nullable_defaults: &nullable_defaults
  empty_string: ""  # Empty string
  null_value: null  # Explicit null
  tilde_null: ~  # Tilde null
  normal_value: "normal"  # Normal value
  # Mixed in collections
  list_with_nulls:
    - item1  # Normal item
    - ""  # Empty string
    - null  # Null item
    - item4  # Another normal
  map_with_nulls:
    key1: value1  # Normal key
    key2: ""  # Empty value
    key3: null  # Null value

# Use nullable defaults
production:
  <<: *nullable_defaults  # Merge nullable defaults
  empty_string: "not empty"  # Override empty string
  normal_value: "production normal"  # Override normal

staging:
  <<: *nullable_defaults  # Merge nullable defaults
  null_value: "not null"  # Override null
"""
        result = self.parser.safe_load(yaml_content)
        assert result.is_success()
        # Verify nullable values in anchors work
        assert result.data['production']['empty_string'] == 'not empty'
        assert result.data['production']['null_value'] is None  # Inherited
        assert result.data['production']['normal_value'] == 'production normal'
        assert result.data['production']['list_with_nulls'] == ['item1', '', None, 'item4']
        assert result.data['staging']['null_value'] == 'not null'
        assert result.data['staging']['empty_string'] == ''  # Inherited
        # Comments should be filtered out
        assert 'comment' not in str(result.data).lower()


if __name__ == '__main__':
    # Run tests with pytest
    pytest.main([__file__, '-v'])
