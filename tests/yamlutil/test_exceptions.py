"""
Tests for YAML Custom Exception Classes

Comprehensive tests for YAML parsing exceptions including:
- Exception creation and formatting
- Exception inheritance hierarchy
- Exception to/from result conversion
- Error message formatting with line/column info
"""

import tempfile
import pytest
from pathlib import Path

# Add parent directory to path for imports
import sys
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from internal.yamlutil.error_types import (
    YAMLParserError,
    YAMLFileNotFoundError,
    YAMLSyntaxError,
    YAMLStructureError,
    YAMLEmptyFileError,
    YAMLErrorCategory,
    YAMLErrorSeverity
)
from internal.yamlutil.reader import (
    YAMLFileReader,
    read_yaml_file
)


class TestYAMLParserError:
    """Test cases for base YAMLParserError class."""

    def test_basic_exception_creation(self):
        """Test creating a basic YAMLParserError."""
        exc = YAMLParserError("Test error message")
        assert str(exc) == "Test error message"
        assert exc.message == "Test error message"
        assert exc.filepath is None
        assert exc.line is None
        assert exc.column is None

    def test_exception_with_file_path(self):
        """Test exception with file path."""
        exc = YAMLParserError("Test error", filepath="/test/file.yaml")
        assert "File: /test/file.yaml" in str(exc)
        assert "Test error" in str(exc)

    def test_exception_with_line_number(self):
        """Test exception with line number."""
        exc = YAMLParserError("Test error", line=10)
        assert "Line 10" in str(exc)
        assert "Test error" in str(exc)

    def test_exception_with_line_and_column(self):
        """Test exception with line and column numbers."""
        exc = YAMLParserError("Test error", line=10, column=5)
        assert "Line 10, Column 5" in str(exc)
        assert "Test error" in str(exc)

    def test_exception_with_all_location_info(self):
        """Test exception with all location information."""
        exc = YAMLParserError(
            "Test error",
            filepath="/test/file.yaml",
            line=10,
            column=5
        )
        assert "File: /test/file.yaml | Line 10, Column 5: Test error" == str(exc)

    def test_exception_is_base_class(self):
        """Test that YAMLParserError is the base exception."""
        exc = YAMLParserError("Test")
        assert isinstance(exc, Exception)
        assert isinstance(exc, YAMLParserError)


class TestYAMLFileNotFoundError:
    """Test cases for YAMLFileNotFoundError class."""

    def test_file_not_found_creation(self):
        """Test creating YAMLFileNotFoundError."""
        exc = YAMLFileNotFoundError("File not found", filepath="/missing/file.yaml")
        assert isinstance(exc, YAMLParserError)
        assert exc.filepath == "/missing/file.yaml"
        assert "File not found" in str(exc)

    def test_file_not_found_inheritance(self):
        """Test that YAMLFileNotFoundError inherits from YAMLParserError."""
        exc = YAMLFileNotFoundError("Missing file", filepath="test.yaml")
        assert isinstance(exc, YAMLParserError)
        assert isinstance(exc, Exception)

    def test_catch_as_base_exception(self):
        """Test catching YAMLFileNotFoundError as YAMLParserError."""
        try:
            raise YAMLFileNotFoundError("File missing", filepath="test.yaml")
        except YAMLParserError as e:
            assert isinstance(e, YAMLFileNotFoundError)
            assert "File missing" in str(e)


class TestYAMLSyntaxError:
    """Test cases for YAMLSyntaxError class."""

    def test_syntax_error_creation(self):
        """Test creating YAMLSyntaxError."""
        exc = YAMLSyntaxError("Syntax error", filepath="test.yaml", line=5, column=3)
        assert isinstance(exc, YAMLParserError)
        assert exc.line == 5
        assert exc.column == 3
        assert exc.filepath == "test.yaml"

    def test_syntax_error_with_context(self):
        """Test YAMLSyntaxError with context."""
        context = "key:\n  - item1\n  - item2\n    bad_indent"
        exc = YAMLSyntaxError(
            "Indentation error",
            filepath="test.yaml",
            line=4,
            column=4,
            context=context,
            suggestion="Use consistent indentation"
        )
        assert "Indentation error" in str(exc)
        assert "Context:" in str(exc)
        assert "Suggestion: Use consistent indentation" in str(exc)

    def test_syntax_error_formatting(self):
        """Test YAMLSyntaxError string formatting."""
        exc = YAMLSyntaxError(
            "Invalid syntax",
            filepath="/path/to/file.yaml",
            line=10,
            column=5,
            context="key: value\n  bad: indent",
            suggestion="Check indentation"
        )
        error_str = str(exc)
        assert "File: /path/to/file.yaml | Line 10, Column 5: Invalid syntax" in error_str
        assert "Context:" in error_str
        assert "Suggestion: Check indentation" in error_str


class TestYAMLStructureError:
    """Test cases for YAMLStructureError class."""

    def test_structure_error_creation(self):
        """Test creating YAMLStructureError."""
        exc = YAMLStructureError("Duplicate key", filepath="test.yaml", line=5)
        assert isinstance(exc, YAMLParserError)
        assert exc.message == "Duplicate key"
        assert exc.filepath == "test.yaml"
        assert exc.line == 5

    def test_structure_error_inheritance(self):
        """Test that YAMLStructureError inherits from YAMLParserError."""
        exc = YAMLStructureError("Invalid structure", filepath="test.yaml")
        assert isinstance(exc, YAMLParserError)
        assert isinstance(exc, Exception)


class TestYAMLEmptyFileError:
    """Test cases for YAMLEmptyFileError class."""

    def test_empty_file_error_creation(self):
        """Test creating YAMLEmptyFileError."""
        exc = YAMLEmptyFileError("Empty file", filepath="/empty/file.yaml")
        assert isinstance(exc, YAMLParserError)
        assert exc.filepath == "/empty/file.yaml"
        assert "Empty file" in str(exc)

    def test_empty_file_error_inheritance(self):
        """Test that YAMLEmptyFileError inherits from YAMLParserError."""
        exc = YAMLEmptyFileError("No content", filepath="test.yaml")
        assert isinstance(exc, YAMLParserError)
        assert isinstance(exc, Exception)


class TestResultToException:
    """Test cases for converting YAMLReadResult to exceptions."""

    def test_successful_result_no_exception(self):
        """Test that successful result returns None for to_exception."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("key: value\n")
            temp_path = f.name

        try:
            result = read_yaml_file(temp_path)
            assert result.success is True
            assert result.to_exception() is None
        finally:
            import os
            os.remove(temp_path)

    def test_file_not_found_to_exception(self):
        """Test converting file not found error to exception."""
        result = read_yaml_file("/nonexistent/file.yaml")
        assert result.success is False

        exc = result.to_exception()
        assert isinstance(exc, YAMLFileNotFoundError)
        assert "not found" in str(exc).lower()

    def test_empty_file_to_exception(self):
        """Test converting empty file error to exception."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            # Write nothing
            temp_path = f.name

        try:
            result = read_yaml_file(temp_path)
            assert result.success is False

            exc = result.to_exception()
            assert isinstance(exc, YAMLEmptyFileError)
            assert "empty" in str(exc).lower()
        finally:
            import os
            os.remove(temp_path)

    def test_syntax_error_to_exception(self):
        """Test converting syntax error to exception."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            # Write invalid YAML with bad indentation
            f.write("key:\n  value\n    bad_indent: true\n")
            temp_path = f.name

        try:
            result = read_yaml_file(temp_path)
            assert result.success is False

            exc = result.to_exception()
            assert isinstance(exc, YAMLSyntaxError)
            assert exc.line is not None or exc.message  # Should have line info or message
        finally:
            import os
            os.remove(temp_path)

    def test_unclosed_quotes_to_exception(self):
        """Test converting unclosed quotes error to exception."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write('key: "unclosed string\n')
            temp_path = f.name

        try:
            result = read_yaml_file(temp_path)
            assert result.success is False

            exc = result.to_exception()
            assert isinstance(exc, (YAMLSyntaxError, YAMLParserError))
        finally:
            import os
            os.remove(temp_path)

    def test_invalid_flow_style_to_exception(self):
        """Test converting invalid flow style error to exception."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("items: [item1, item2,\n")
            temp_path = f.name

        try:
            result = read_yaml_file(temp_path)
            assert result.success is False

            exc = result.to_exception()
            assert isinstance(exc, (YAMLSyntaxError, YAMLParserError))
        finally:
            import os
            os.remove(temp_path)


class TestRaiseIfError:
    """Test cases for raise_if_error method."""

    def test_raise_if_error_on_success(self):
        """Test raise_if_error doesn't raise on success."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("key: value\n")
            temp_path = f.name

        try:
            result = read_yaml_file(temp_path)
            assert result.success is True
            # Should not raise
            result.raise_if_error()
        finally:
            import os
            os.remove(temp_path)

    def test_raise_if_error_on_failure(self):
        """Test raise_if_error raises exception on failure."""
        result = read_yaml_file("/nonexistent/file.yaml")
        assert result.success is False

        with pytest.raises(YAMLFileNotFoundError):
            result.raise_if_error()

    def test_raise_if_error_syntax_error(self):
        """Test raise_if_error raises syntax error exception."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("key:\n  value\n    bad_indent: true\n")
            temp_path = f.name

        try:
            result = read_yaml_file(temp_path)
            assert result.success is False

            with pytest.raises((YAMLSyntaxError, YAMLParserError)):
                result.raise_if_error()
        finally:
            import os
            os.remove(temp_path)


class TestExceptionUsagePatterns:
    """Test real-world usage patterns for exceptions."""

    def test_try_except_pattern(self):
        """Test using try/except pattern with YAML reading."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("name: test\nvalue: 123\n")
            temp_path = f.name

        try:
            result = read_yaml_file(temp_path)
            if not result.success:
                raise result.to_exception()
            data = result.data
            assert data['name'] == 'test'
            assert data['value'] == 123
        finally:
            import os
            os.remove(temp_path)

    def test_raise_if_error_pattern(self):
        """Test using raise_if_error pattern."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("config:\n  debug: true\n")
            temp_path = f.name

        try:
            result = read_yaml_file(temp_path)
            result.raise_if_error()
            data = result.data
            assert data['config']['debug'] is True
        finally:
            import os
            os.remove(temp_path)

    def test_catch_specific_exception(self):
        """Test catching specific exception types."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("invalid: yaml: content:\n    bad\n")
            temp_path = f.name

        try:
            result = read_yaml_file(temp_path)
            try:
                result.raise_if_error()
                assert False, "Should have raised exception"
            except YAMLSyntaxError as e:
                # Expected to catch syntax error
                assert isinstance(e, YAMLParserError)
            except YAMLParserError as e:
                # Also acceptable to catch as base exception
                assert isinstance(e, YAMLParserError)
        finally:
            import os
            os.remove(temp_path)


if __name__ == '__main__':
    pytest.main([__file__, '-v'])
