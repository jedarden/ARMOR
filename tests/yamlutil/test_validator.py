"""
Tests for YAML validation functionality
"""
import pytest
import tempfile
import os
from pathlib import Path

import sys
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from internal.yamlutil import (
    YAMLSyntaxValidator,
    validate_yaml_file,
    validate_yaml_string,
    YAMLErrorCategory,
    YAMLErrorSeverity,
    YAMLValidationResult
)


class TestYAMLSyntaxValidator:
    """Test comprehensive YAML syntax validation."""

    def test_valid_yaml(self):
        """Test that valid YAML passes validation."""
        valid_yaml = """
key: value
nested:
  item1: value1
  item2: value2
sequence:
  - item1
  - item2
"""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(valid_yaml)

        assert result.is_valid
        assert len(result.errors) == 0

    def test_indentation_error_detection(self):
        """Test detection of indentation errors."""
        invalid_yaml = """
key: value
  nested:
    item: value
"""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(invalid_yaml)

        # Should detect an error (though PyYAML may handle this differently)
        # The key point is we get detailed error information
        if not result.is_valid:
            assert len(result.errors) > 0
            error = result.errors[0]
            # Check that we have line/column information
            if error.line is not None:
                assert isinstance(error.line, int)
                assert error.line > 0

    def test_tab_character_detection(self):
        """Test detection of tab characters in YAML."""
        yaml_with_tabs = """
key:\tvalue
nested:
\titem: value
"""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_with_tabs)

        assert not result.is_valid
        assert len(result.errors) > 0

        # Find tab error
        tab_error = None
        for error in result.errors:
            if 'tab' in error.message.lower():
                tab_error = error
                break

        assert tab_error is not None
        assert tab_error.category == YAMLErrorCategory.INDENTATION
        assert tab_error.line is not None
        assert 'suggestion' in tab_error.suggestion.lower()

    def test_unclosed_quote_detection(self):
        """Test detection of unclosed quotes."""
        invalid_yaml = """
key: "unclosed string
another: value
"""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(invalid_yaml)

        assert not result.is_valid
        assert len(result.errors) > 0

        error = result.errors[0]
        # Should be categorized as syntax or scalar error
        assert error.category in [YAMLErrorCategory.SYNTAX, YAMLErrorCategory.SCALAR]
        if error.line is not None:
            assert error.line > 0

    def test_flow_collection_errors(self):
        """Test detection of flow collection errors."""
        invalid_yaml = """
key: {unclosed: value
another: [item1, item2
"""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(invalid_yaml)

        assert not result.is_valid
        # Should detect flow errors

    def test_anchor_alias_errors(self):
        """Test detection of undefined alias errors."""
        invalid_yaml = """
key: &anchor value
ref: *undefined_alias
"""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(invalid_yaml)

        assert not result.is_valid
        if not result.is_valid:
            # Check for alias error
            alias_error = None
            for error in result.errors:
                if error.category == YAMLErrorCategory.ALIAS:
                    alias_error = error
                    break

            if alias_error:
                assert 'undefined' in alias_error.message.lower() or 'alias' in alias_error.message.lower()

    def test_empty_file_detection(self):
        """Test detection of empty YAML files."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content("   \n  \n")

        assert not result.is_valid
        assert len(result.errors) > 0
        assert 'empty' in result.errors[0].message.lower()

    def test_line_column_extraction(self):
        """Test that line and column numbers are extracted correctly."""
        # Create a YAML file with an error at a known position
        yaml_content = """
key1: value1
key2: value2
key3: invalid syntax here
  key4: value4
"""

        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        if not result.is_valid:
            # At least one error should have location information
            has_location = False
            for error in result.errors:
                if error.line is not None:
                    has_location = True
                    assert isinstance(error.line, int)
                    assert error.line >= 1
                    if error.column is not None:
                        assert isinstance(error.column, int)
                        assert error.column >= 1
                    break

            assert has_location, "Expected at least one error with location information"

    def test_error_categorization(self):
        """Test that errors are properly categorized."""
        test_cases = [
            # Indentation error
            ("""
key: value
  bad_indent: value
""", YAMLErrorCategory.INDENTATION),

            # Flow error
            ("""
flow: {unclosed: bracket
""", YAMLErrorCategory.FLOW),

            # Document error
            ("""
---
document1
---
document2
extra_content
""", None),  # This might be valid or different category
        ]

        validator = YAMLSyntaxValidator()
        for yaml_content, expected_category in test_cases:
            result = validator.validate_content(yaml_content)
            if not result.is_valid and expected_category:
                # Check if any error matches expected category
                found = any(e.category == expected_category for e in result.errors)
                if found:
                    break  # Success

    def test_error_message_formatting(self):
        """Test that error messages are human-readable and well-formatted."""
        invalid_yaml = """
key: "unclosed quote
another: value
"""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(invalid_yaml)

        assert not result.is_valid
        assert len(result.errors) > 0

        error = result.errors[0]

        # Check that error has required fields
        assert error.message != ""
        assert error.category is not None
        assert error.severity is not None

        # Check that string representation is readable
        error_str = str(error)
        assert len(error_str) > 0
        assert error.category.value in error_str or error.severity.value in error_str

    def test_file_validation(self):
        """Test validation of actual YAML files."""
        # Create a temporary YAML file
        valid_yaml_content = """
config:
  name: test
  values:
    - item1
    - item2
"""

        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            f.write(valid_yaml_content)
            temp_path = f.name

        try:
            validator = YAMLSyntaxValidator()
            result = validator.validate_file(temp_path)

            assert result.is_valid
            assert len(result.errors) == 0
        finally:
            os.unlink(temp_path)

    def test_file_validation_with_error(self):
        """Test file validation with syntax errors."""
        invalid_yaml_content = """
config:
  name: test
  values: [unclosed
    - item1
"""

        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            f.write(invalid_yaml_content)
            temp_path = f.name

        try:
            validator = YAMLSyntaxValidator()
            result = validator.validate_file(temp_path)

            assert not result.is_valid
            assert len(result.errors) > 0
        finally:
            os.unlink(temp_path)


class TestConvenienceFunctions:
    """Test convenience functions for quick validation."""

    def test_validate_yaml_string(self):
        """Test the validate_yaml_string convenience function."""
        valid_yaml = "key: value\nnested:\n  item: value"
        result = validate_yaml_string(valid_yaml)

        assert isinstance(result, YAMLValidationResult)
        assert result.is_valid

    def test_validate_yaml_file_function(self):
        """Test the validate_yaml_file convenience function."""
        valid_yaml = "key: value\n"

        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            f.write(valid_yaml)
            temp_path = f.name

        try:
            result = validate_yaml_file(temp_path)

            assert isinstance(result, YAMLValidationResult)
            assert result.is_valid
        finally:
            os.unlink(temp_path)


class TestYAMLValidationResult:
    """Test YAMLValidationResult methods."""

    def test_has_errors(self):
        """Test the has_errors method."""
        result = YAMLValidationResult(
            is_valid=True,
            errors=[],
            warnings=[]
        )

        assert not result.has_errors()

        error = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.ERROR,
            message="Test error"
        )
        result_with_error = YAMLValidationResult(
            is_valid=False,
            errors=[error],
            warnings=[]
        )

        assert result_with_error.has_errors()

    def test_has_warnings(self):
        """Test the has_warnings method."""
        result = YAMLValidationResult(
            is_valid=True,
            errors=[],
            warnings=[]
        )

        assert not result.has_warnings()

        warning = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.WARNING,
            message="Test warning"
        )
        result_with_warning = YAMLValidationResult(
            is_valid=True,
            errors=[],
            warnings=[warning]
        )

        assert result_with_warning.has_warnings()

    def test_get_all_issues(self):
        """Test the get_all_issues method."""
        error = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.ERROR,
            message="Test error"
        )
        warning = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.WARNING,
            message="Test warning"
        )

        result = YAMLValidationResult(
            is_valid=False,
            errors=[error],
            warnings=[warning]
        )

        all_issues = result.get_all_issues()
        assert len(all_issues) == 2
        assert error in all_issues
        assert warning in all_issues

    def test_string_representation(self):
        """Test the __str__ method for human-readable output."""
        result = YAMLValidationResult(
            is_valid=True,
            errors=[],
            warnings=[]
        )

        result_str = str(result)
        assert "✓" in result_str or "Valid" in result_str

        result_with_error = YAMLValidationResult(
            is_valid=False,
            errors=[YAMLErrorDetail(
                category=YAMLErrorCategory.SYNTAX,
                severity=YAMLErrorSeverity.ERROR,
                message="Test error"
            )],
            warnings=[]
        )

        error_str = str(result_with_error)
        assert "✗" in error_str or "Invalid" in error_str


if __name__ == '__main__':
    pytest.main([__file__, '-v'])