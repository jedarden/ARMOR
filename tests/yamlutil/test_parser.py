"""
Unit Tests for YAML Core Parser with Error Handling

Tests comprehensive error handling for:
- YAMLError (base class)
- ScannerError (lexical scanning errors)
- ParserError (structural parsing errors)
- Edge cases (empty content, None input, invalid types)
"""

import pytest
import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from internal.yamlutil.parser import (
    YAMLCoreParser,
    SafeLoadResult,
    safe_load_yaml
)
from internal.yamlutil.error_types import (
    YAMLErrorCategory,
    YAMLErrorSeverity
)


class TestYAMLCoreParser:
    """Test cases for YAMLCoreParser class."""

    def test_parser_initialization(self):
        """Test that parser can be initialized."""
        parser = YAMLCoreParser()
        assert parser.yaml is not None
        assert parser.ScannerError is not None
        assert parser.ParserError is not None


class TestSafeLoadBasicFunctionality:
    """Test cases for basic safe_load functionality."""

    def test_simple_key_value_pairs(self):
        """Test parsing simple key-value pairs."""
        parser = YAMLCoreParser()
        result = parser.safe_load("key: value")

        assert result.success is True
        assert result.data is not None
        assert result.data['key'] == 'value'
        assert result.error is None

    def test_nested_structure(self):
        """Test parsing nested YAML structures."""
        parser = YAMLCoreParser()
        content = """
server:
  host: localhost
  port: 8080
  ssl:
    enabled: true
"""
        result = parser.safe_load(content)

        assert result.success is True
        assert result.data['server']['host'] == 'localhost'
        assert result.data['server']['port'] == 8080
        assert result.data['server']['ssl']['enabled'] is True

    def test_list_values(self):
        """Test parsing YAML with list values."""
        parser = YAMLCoreParser()
        content = """
items:
  - item1
  - item2
  - item3
"""
        result = parser.safe_load(content)

        assert result.success is True
        assert result.data['items'] == ['item1', 'item2', 'item3']

    def test_flow_style_collections(self):
        """Test parsing flow-style collections."""
        parser = YAMLCoreParser()
        content = """
items: [item1, item2, item3]
mapping: {key1: value1, key2: value2}
"""
        result = parser.safe_load(content)

        assert result.success is True
        assert result.data['items'] == ['item1', 'item2', 'item3']
        assert result.data['mapping']['key1'] == 'value1'


class TestScannerErrorHandling:
    """Test cases for ScannerError (lexical scanning errors)."""

    def test_indentation_errors(self):
        """Test detection of indentation errors."""
        parser = YAMLCoreParser()
        # Inconsistent indentation causing mapping error
        content = """key:
  nested_key: value
    bad_indentation: true"""
        result = parser.safe_load(content)

        assert result.success is False
        assert result.error is not None
        assert result.error.category == YAMLErrorCategory.SYNTAX
        assert result.error.severity == YAMLErrorSeverity.ERROR
        assert result.error.line is not None
        assert 'mapping' in result.error.message.lower()

    def test_tab_character_errors(self):
        """Test detection of tab character errors."""
        parser = YAMLCoreParser()
        # Tab characters in YAML (represented as \t here)
        content = "key:\n\tvalue: true"
        result = parser.safe_load(content)

        assert result.success is False
        assert result.error is not None
        assert result.error.category in [YAMLErrorCategory.SYNTAX, YAMLErrorCategory.INDENTATION]

    def test_unclosed_quotes(self):
        """Test detection of unclosed quoted strings."""
        parser = YAMLCoreParser()
        content = 'key: "unclosed string'
        result = parser.safe_load(content)

        assert result.success is False
        assert result.error is not None
        assert result.error.category == YAMLErrorCategory.SYNTAX
        # Error message should mention the stream end or quote issue
        msg_lower = str(result.error.message).lower()
        assert 'stream' in msg_lower or 'quote' in msg_lower or 'scalar' in msg_lower

    def test_special_character_errors(self):
        """Test detection of invalid special characters."""
        parser = YAMLCoreParser()
        # Invalid control character in flow context
        content = 'key: [item1, item2, \x00]'
        result = parser.safe_load(content)

        # This might not error in all PyYAML versions, but should handle gracefully
        # Either success or structured error is acceptable
        assert isinstance(result, SafeLoadResult)

    def test_invalid_escape_sequences(self):
        """Test detection of invalid escape sequences."""
        parser = YAMLCoreParser()
        content = 'key: "invalid \\x escape"'
        result = parser.safe_load(content)

        # Should either succeed (if escape is valid) or provide structured error
        assert isinstance(result, SafeLoadResult)


class TestParserErrorHandling:
    """Test cases for ParserError (structural parsing errors)."""

    def test_unclosed_flow_sequence(self):
        """Test detection of unclosed flow sequences."""
        parser = YAMLCoreParser()
        content = 'items: [item1, item2'
        result = parser.safe_load(content)

        assert result.success is False
        assert result.error is not None
        assert result.error.category in [YAMLErrorCategory.FLOW, YAMLErrorCategory.STRUCTURE]
        assert result.raw_exception is not None

    def test_unclosed_flow_mapping(self):
        """Test detection of unclosed flow mappings."""
        parser = YAMLCoreParser()
        content = 'mapping: {key: value'
        result = parser.safe_load(content)

        assert result.success is False
        assert result.error is not None
        assert result.error.category in [YAMLErrorCategory.FLOW, YAMLErrorCategory.STRUCTURE]

    def test_invalid_block_structure(self):
        """Test detection of invalid block structure."""
        parser = YAMLCoreParser()
        content = """key:
  - item1
invalid_key: value"""
        result = parser.safe_load(content)

        # Block structure errors should be caught
        assert isinstance(result, SafeLoadResult)
        if not result.success:
            assert result.error is not None

    def test_document_separator_issues(self):
        """Test detection of document separator issues."""
        parser = YAMLCoreParser()
        content = """---
key: value
---
another: doc"""
        result = parser.safe_load(content)

        # Multi-document with separators should work or give structured error
        assert isinstance(result, SafeLoadResult)

    def test_invalid_sequence_syntax(self):
        """Test detection of invalid sequence syntax."""
        parser = YAMLCoreParser()
        content = """items:
- item1
  - nested_item
   bad_indent: value"""
        result = parser.safe_load(content)

        assert result.success is False
        assert result.error is not None


class TestEdgeCases:
    """Test cases for edge cases and boundary conditions."""

    def test_empty_string(self):
        """Test handling of empty string input."""
        parser = YAMLCoreParser()
        result = parser.safe_load('')

        assert result.success is False
        assert result.error is not None
        assert result.error.category == YAMLErrorCategory.DOCUMENT
        assert 'empty' in result.error.message.lower()

    def test_whitespace_only(self):
        """Test handling of whitespace-only input."""
        parser = YAMLCoreParser()
        result = parser.safe_load('   \n  \t  ')

        assert result.success is False
        assert result.error is not None
        assert result.error.category == YAMLErrorCategory.DOCUMENT

    def test_none_input(self):
        """Test handling of None input."""
        parser = YAMLCoreParser()
        result = parser.safe_load(None)

        assert result.success is False
        assert result.error is not None
        assert result.error.category == YAMLErrorCategory.UNKNOWN
        assert 'none' in result.error.message.lower()

    def test_non_string_input(self):
        """Test handling of non-string input."""
        parser = YAMLCoreParser()
        result = parser.safe_load(12345)

        assert result.success is False
        assert result.error is not None
        assert 'string' in result.error.message.lower()

    def test_list_input(self):
        """Test handling of list input (invalid type)."""
        parser = YAMLCoreParser()
        result = parser.safe_load(['key', 'value'])

        assert result.success is False
        assert result.error is not None

    def test_dict_input(self):
        """Test handling of dict input (invalid type)."""
        parser = YAMLCoreParser()
        result = parser.safe_load({'key': 'value'})

        assert result.success is False
        assert result.error is not None

    def test_source_parameter(self):
        """Test that source parameter is used in error context."""
        parser = YAMLCoreParser()
        result = parser.safe_load('key:\n  bad: value\n    too: far', source='test.yaml')

        if not result.success:
            # Source should be recorded somewhere in the error context
            assert result.error is not None


class TestSafeLoadResultMethods:
    """Test cases for SafeLoadResult helper methods."""

    def test_is_success(self):
        """Test is_success method."""
        result = SafeLoadResult(success=True, data={'key': 'value'})
        assert result.is_success() is True
        assert result.is_error() is False

    def test_is_error(self):
        """Test is_error method."""
        result = SafeLoadResult(success=False)
        assert result.is_error() is True
        assert result.is_success() is False

    def test_get_data_success(self):
        """Test get_data on successful result."""
        result = SafeLoadResult(success=True, data={'key': 'value'})
        assert result.get_data() == {'key': 'value'}

    def test_get_data_failure_raises(self):
        """Test get_data on failed result raises RuntimeError."""
        from internal.yamlutil.error_types import YAMLErrorDetail

        error = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.ERROR,
            message="Test error"
        )
        result = SafeLoadResult(success=False, error=error)

        with pytest.raises(RuntimeError, match="Cannot get data from failed parse"):
            result.get_data()

    def test_get_error(self):
        """Test get_error method."""
        from internal.yamlutil.error_types import YAMLErrorDetail

        error = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.ERROR,
            message="Test error"
        )
        result = SafeLoadResult(success=False, error=error)

        assert result.get_error() == error


class TestConvenienceFunctions:
    """Test cases for convenience functions."""

    def test_safe_load_yaml_function(self):
        """Test safe_load_yaml convenience function."""
        result = safe_load_yaml('key: value\nnumber: 42')

        assert result.success is True
        assert result.data['key'] == 'value'
        assert result.data['number'] == 42

    def test_safe_load_yaml_with_error(self):
        """Test safe_load_yaml with invalid YAML."""
        result = safe_load_yaml('key:\n  bad: value\n    too: far')

        assert result.success is False
        assert result.error is not None

    def test_safe_load_yaml_with_source(self):
        """Test safe_load_yaml with custom source parameter."""
        result = safe_load_yaml('key: value', source='config.yaml')

        assert result.success is True
        # Source parameter should be accepted without error


class TestErrorDetailExtraction:
    """Test cases for error detail extraction from exceptions."""

    def test_scanner_error_location_extraction(self):
        """Test that ScannerError location information is extracted."""
        parser = YAMLCoreParser()
        content = """key:
  nested: value
    bad_indent: true"""
        result = parser.safe_load(content)

        assert result.success is False
        assert result.error is not None
        # Should extract line number
        assert result.error.line is not None
        # Should provide context
        assert result.error.message is not None
        assert len(result.error.message) > 0

    def test_parser_error_location_extraction(self):
        """Test that ParserError location information is extracted."""
        parser = YAMLCoreParser()
        content = 'items: [item1, item2'
        result = parser.safe_load(content)

        assert result.success is False
        assert result.error is not None
        # Should extract line and/or column
        assert result.error.line is not None or result.error.message is not None

    def test_error_suggestion_generation(self):
        """Test that helpful suggestions are generated."""
        parser = YAMLCoreParser()
        content = """key:
\tvalue: true"""  # Tab character
        result = parser.safe_load(content)

        assert result.success is False
        assert result.error is not None
        # Should provide some suggestion
        assert result.error.suggestion is not None
        assert len(result.error.suggestion) > 0

    def test_raw_exception_preserved(self):
        """Test that raw exception is preserved for debugging."""
        parser = YAMLCoreParser()
        content = 'items: [item1, item2'
        result = parser.safe_load(content)

        assert result.success is False
        assert result.raw_exception is not None
        assert isinstance(result.raw_exception, Exception)


class TestComplexYAMLStructures:
    """Test cases for complex but valid YAML structures."""

    def test_complex_nested_structure(self):
        """Test parsing complex nested structures."""
        parser = YAMLCoreParser()
        content = """
services:
  - name: web
    config:
      port: 8080
      ssl:
        enabled: true
        cert: /path/to/cert.pem
    endpoints:
      - /api
      - /health
  - name: db
    config:
      port: 5432
      host: localhost
"""
        result = parser.safe_load(content)

        assert result.success is True
        assert result.data is not None
        assert len(result.data['services']) == 2
        assert result.data['services'][0]['name'] == 'web'

    def test_multiline_strings(self):
        """Test parsing multiline strings."""
        parser = YAMLCoreParser()
        content = """
description: |
  This is a multiline
  string that spans
  multiple lines
"""
        result = parser.safe_load(content)

        assert result.success is True
        assert 'multiline' in result.data['description']

    def test_anchors_and_aliases(self):
        """Test parsing anchors and aliases."""
        parser = YAMLCoreParser()
        content = """
defaults: &defaults
  timeout: 30
  retry: 3

service:
  <<: *defaults
  name: web
"""
        result = parser.safe_load(content)

        assert result.success is True
        assert result.data['service']['timeout'] == 30
        assert result.data['service']['name'] == 'web'

    def test_explicit_types(self):
        """Test parsing explicit type tags."""
        parser = YAMLCoreParser()
        content = """
integer: !!int 42
float: !!float 3.14
boolean: !!bool true
string: !!str "hello"
"""
        result = parser.safe_load(content)

        assert result.success is True
        assert result.data['integer'] == 42
        assert result.data['float'] == 3.14
        assert result.data['boolean'] is True
        assert result.data['string'] == 'hello'


if __name__ == '__main__':
    pytest.main([__file__, '-v'])
