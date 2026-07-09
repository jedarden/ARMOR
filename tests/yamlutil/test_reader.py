"""
Tests for YAML File Reader

Comprehensive tests for YAML file reading functionality including:
- File path validation
- File existence checking
- YAML parsing
- Multi-document support
- Error handling
"""

import os
import tempfile
from pathlib import Path
import pytest

# Add parent directory to path for imports
import sys
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from internal.yamlutil.reader import (
    YAMLFileReader,
    YAMLReadResult,
    read_yaml_file,
    read_yaml_file_simple
)


class TestYAMLFileReader:
    """Test cases for YAMLFileReader class."""

    def test_reader_initialization(self):
        """Test that reader can be initialized."""
        reader = YAMLFileReader()
        assert reader.yaml is not None
        assert reader.resolve_absolute is True

    def test_reader_with_no_absolute_resolution(self):
        """Test reader without absolute path resolution."""
        reader = YAMLFileReader(resolve_absolute=False)
        assert reader.resolve_absolute is False


class TestFilePathValidation:
    """Test cases for file path validation."""

    def test_nonexistent_file(self):
        """Test reading a file that doesn't exist."""
        reader = YAMLFileReader()
        result = reader.read_file("/nonexistent/file.yaml")

        assert result.success is False
        assert len(result.errors) > 0
        assert "not found" in result.errors[0].message.lower()
        assert result.data is None

    def test_empty_file_path(self):
        """Test reading with empty file path."""
        reader = YAMLFileReader()
        result = reader.read_file("")

        assert result.success is False
        assert "empty" in result.errors[0].message.lower()

    def test_directory_instead_of_file(self):
        """Test reading a directory instead of a file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            reader = YAMLFileReader()
            result = reader.read_file(tmpdir)

            assert result.success is False
            assert "not a file" in result.errors[0].message.lower()

    def test_unreadable_file(self):
        """Test reading a file without read permissions."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("test: data")
            temp_path = f.name

        try:
            # Remove read permissions
            os.chmod(temp_path, 0o000)

            reader = YAMLFileReader()
            result = reader.read_file(temp_path)

            assert result.success is False
            assert "not readable" in result.errors[0].message.lower() or "permission" in result.errors[0].message.lower()
        finally:
            # Restore permissions for cleanup
            os.chmod(temp_path, 0o644)
            os.remove(temp_path)


class TestYAMLParsing:
    """Test cases for YAML parsing functionality."""

    def test_simple_key_value_pairs(self):
        """Test reading simple key-value pairs."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("""
name: test
value: 123
active: true
""")
            temp_path = f.name

        try:
            reader = YAMLFileReader()
            result = reader.read_file(temp_path)

            assert result.success is True
            assert result.data is not None
            assert result.data['name'] == 'test'
            assert result.data['value'] == 123
            assert result.data['active'] is True
            assert len(result.errors) == 0
        finally:
            os.remove(temp_path)

    def test_nested_structure(self):
        """Test reading nested YAML structures."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("""
server:
  host: localhost
  port: 8080
  ssl:
    enabled: true
    cert: /path/to/cert.pem
database:
  host: db.example.com
  port: 5432
""")
            temp_path = f.name

        try:
            reader = YAMLFileReader()
            result = reader.read_file(temp_path)

            assert result.success is True
            assert result.data is not None
            assert result.data['server']['host'] == 'localhost'
            assert result.data['server']['port'] == 8080
            assert result.data['server']['ssl']['enabled'] is True
            assert result.data['database']['host'] == 'db.example.com'
            assert result.data['database']['port'] == 5432
        finally:
            os.remove(temp_path)

    def test_list_values(self):
        """Test reading YAML with list values."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("""
items:
  - item1
  - item2
  - item3
numbers:
  - 1
  - 2
  - 3
  - 4
""")
            temp_path = f.name

        try:
            reader = YAMLFileReader()
            result = reader.read_file(temp_path)

            assert result.success is True
            assert result.data is not None
            assert result.data['items'] == ['item1', 'item2', 'item3']
            assert result.data['numbers'] == [1, 2, 3, 4]
        finally:
            os.remove(temp_path)

    def test_complex_structure(self):
        """Test reading complex YAML structures."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("""
services:
  - name: web
    port: 8080
    endpoints:
      - /api
      - /health
  - name: db
    port: 5432
config:
  debug: true
  log_level: info
  features:
    - auth
    - caching
    - monitoring
""")
            temp_path = f.name

        try:
            reader = YAMLFileReader()
            result = reader.read_file(temp_path)

            assert result.success is True
            assert result.data is not None
            assert len(result.data['services']) == 2
            assert result.data['services'][0]['name'] == 'web'
            assert result.data['services'][0]['endpoints'] == ['/api', '/health']
            assert result.data['config']['debug'] is True
            assert len(result.data['config']['features']) == 3
        finally:
            os.remove(temp_path)


class TestMultiDocumentSupport:
    """Test cases for multi-document YAML support."""

    def test_multi_document_reading(self):
        """Test reading multi-document YAML files."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("""
---
name: document1
value: 1
---
name: document2
value: 2
---
name: document3
value: 3
""")
            temp_path = f.name

        try:
            reader = YAMLFileReader()
            result = reader.read_file(temp_path, multi_document=True)

            assert result.success is True
            assert result.data is not None
            assert isinstance(result.data, list)
            assert len(result.data) == 3
            assert result.data[0]['name'] == 'document1'
            assert result.data[1]['name'] == 'document2'
            assert result.data[2]['name'] == 'document3'
        finally:
            os.remove(temp_path)

    def test_single_document_multi_mode(self):
        """Test reading single document with multi_document=True."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("""
name: single
value: 42
""")
            temp_path = f.name

        try:
            reader = YAMLFileReader()
            result = reader.read_file(temp_path, multi_document=True)

            # Single document should be unwrapped
            assert result.success is True
            assert result.data is not None
            assert isinstance(result.data, dict)
            assert result.data['name'] == 'single'
            assert result.data['value'] == 42
        finally:
            os.remove(temp_path)


class TestErrorHandling:
    """Test cases for error handling."""

    def test_invalid_yaml_syntax(self):
        """Test reading file with invalid YAML syntax."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("""
key:
  nested_key: value
    bad_indentation: true
""")
            temp_path = f.name

        try:
            reader = YAMLFileReader()
            result = reader.read_file(temp_path)

            assert result.success is False
            assert len(result.errors) > 0
            assert result.data is None
        finally:
            os.remove(temp_path)

    def test_empty_file(self):
        """Test reading empty YAML file."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            # Write nothing
            temp_path = f.name

        try:
            reader = YAMLFileReader()
            result = reader.read_file(temp_path)

            assert result.success is False
            assert len(result.errors) > 0
            assert "empty" in result.errors[0].message.lower()
        finally:
            os.remove(temp_path)

    def test_unclosed_quotes(self):
        """Test reading YAML with unclosed quotes."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("""
key: "unclosed string
""")
            temp_path = f.name

        try:
            reader = YAMLFileReader()
            result = reader.read_file(temp_path)

            assert result.success is False
            assert len(result.errors) > 0
        finally:
            os.remove(temp_path)

    def test_invalid_flow_style(self):
        """Test reading YAML with invalid flow style."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("""
items: [item1, item2,
""")
            temp_path = f.name

        try:
            reader = YAMLFileReader()
            result = reader.read_file(temp_path)

            assert result.success is False
            assert len(result.errors) > 0
        finally:
            os.remove(temp_path)


class TestConvenienceFunctions:
    """Test cases for convenience functions."""

    def test_read_yaml_file_function(self):
        """Test read_yaml_file convenience function."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("""
test: value
number: 42
""")
            temp_path = f.name

        try:
            result = read_yaml_file(temp_path)

            assert isinstance(result, YAMLReadResult)
            assert result.success is True
            assert result.data['test'] == 'value'
            assert result.data['number'] == 42
        finally:
            os.remove(temp_path)

    def test_read_yaml_file_simple_success(self):
        """Test read_yaml_file_simple on successful read."""
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
            f.write("""
key: value
count: 10
""")
            temp_path = f.name

        try:
            data = read_yaml_file_simple(temp_path)

            assert data is not None
            assert data['key'] == 'value'
            assert data['count'] == 10
        finally:
            os.remove(temp_path)

    def test_read_yaml_file_simple_failure(self):
        """Test read_yaml_file_simple on failed read."""
        result = read_yaml_file_simple("/nonexistent/file.yaml")

        assert result is None


class TestYAMLReadResult:
    """Test cases for YAMLReadResult class."""

    def test_result_has_errors(self):
        """Test has_errors method."""
        from internal.yamlutil.error_types import YAMLErrorDetail, YAMLErrorCategory, YAMLErrorSeverity

        result_no_errors = YAMLReadResult(
            success=True,
            data={'key': 'value'},
            errors=[],
            warnings=[],
            filepath='/test.yaml'
        )
        assert result_no_errors.has_errors() is False

        error = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.ERROR,
            message="Test error"
        )
        result_with_errors = YAMLReadResult(
            success=False,
            data=None,
            errors=[error],
            warnings=[],
            filepath='/test.yaml'
        )
        assert result_with_errors.has_errors() is True

    def test_result_has_warnings(self):
        """Test has_warnings method."""
        from internal.yamlutil.error_types import YAMLErrorDetail, YAMLErrorCategory, YAMLErrorSeverity

        result_no_warnings = YAMLReadResult(
            success=True,
            data={'key': 'value'},
            errors=[],
            warnings=[],
            filepath='/test.yaml'
        )
        assert result_no_warnings.has_warnings() is False

        warning = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.WARNING,
            message="Test warning"
        )
        result_with_warnings = YAMLReadResult(
            success=True,
            data={'key': 'value'},
            errors=[],
            warnings=[warning],
            filepath='/test.yaml'
        )
        assert result_with_warnings.has_warnings() is True

    def test_get_data_success(self):
        """Test get_data on successful read."""
        result = YAMLReadResult(
            success=True,
            data={'key': 'value'},
            errors=[],
            warnings=[],
            filepath='/test.yaml'
        )
        data = result.get_data()
        assert data == {'key': 'value'}

    def test_get_data_failure(self):
        """Test get_data on failed read raises error."""
        result = YAMLReadResult(
            success=False,
            data=None,
            errors=[],
            warnings=[],
            filepath='/test.yaml'
        )

        with pytest.raises(RuntimeError, match="Cannot get data from failed YAML read"):
            result.get_data()


class TestMultipleFileReading:
    """Test cases for reading multiple files."""

    def test_read_multiple_files(self):
        """Test reading multiple YAML files at once."""
        with tempfile.TemporaryDirectory() as tmpdir:
            # Create multiple YAML files
            files = []
            for i in range(3):
                path = os.path.join(tmpdir, f'file{i}.yaml')
                with open(path, 'w') as f:
                    f.write(f'index: {i}\nvalue: test{i}')
                files.append(path)

            reader = YAMLFileReader()
            results = reader.read_multiple_files(files)

            assert len(results) == 3
            for i, result in enumerate(results):
                assert result.success is True
                assert result.data['index'] == i
                assert result.data['value'] == f'test{i}'

    def test_read_multiple_files_with_errors(self):
        """Test reading multiple files where some have errors."""
        with tempfile.TemporaryDirectory() as tmpdir:
            # Create valid file
            valid_path = os.path.join(tmpdir, 'valid.yaml')
            with open(valid_path, 'w') as f:
                f.write('valid: true')

            # Invalid file path
            invalid_path = os.path.join(tmpdir, 'nonexistent.yaml')

            # File with invalid YAML
            bad_yaml_path = os.path.join(tmpdir, 'bad.yaml')
            with open(bad_yaml_path, 'w') as f:
                f.write('key:\n  bad_indent: value\n    too_far: true')

            reader = YAMLFileReader()
            results = reader.read_multiple_files([valid_path, invalid_path, bad_yaml_path])

            assert len(results) == 3
            assert results[0].success is True  # valid file
            assert results[1].success is False  # nonexistent
            assert results[2].success is False  # invalid YAML


if __name__ == '__main__':
    pytest.main([__file__, '-v'])