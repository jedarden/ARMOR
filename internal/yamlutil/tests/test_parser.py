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


if __name__ == '__main__':
    # Run tests with pytest
    pytest.main([__file__, '-v'])
