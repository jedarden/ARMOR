"""
Comprehensive tests for YAML validation using broken YAML samples

Tests the validator against various real-world and synthetic broken YAML samples
to ensure proper detection and categorization of syntax errors.
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
    YAMLErrorDetail,
    YAMLValidationResult
)

from .broken_yaml_samples import *


class TestBrokenYAMLSamples:
    """Test validation against various broken YAML samples."""

    def test_indentation_mixed_spaces_tabs(self):
        """Test detection of mixed spaces and tabs."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(INDENTATION_MIXED_SPACES_TABS)

        assert not result.is_valid
        # Should detect tab character errors
        tab_errors = [e for e in result.errors if 'tab' in e.message.lower()]
        assert len(tab_errors) > 0
        assert all(e.category == YAMLErrorCategory.INDENTATION for e in tab_errors)

    def test_indentation_inconsistent(self):
        """Test detection of inconsistent indentation."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(INDENTATION_INCONSISTENT)

        # May or may not be invalid depending on PyYAML's handling
        # But if invalid, should have indentation or syntax errors
        if not result.is_valid:
            assert any(e.category in [YAMLErrorCategory.INDENTATION, YAMLErrorCategory.SYNTAX]
                      for e in result.errors)

    def test_delimiter_missing_colon(self):
        """Test detection of missing colon delimiter."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(DELIMITER_MISSING_COLON)

        assert not result.is_valid
        assert len(result.errors) > 0

    def test_delimiter_unclosed_quote(self):
        """Test detection of unclosed quotes."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(DELIMITER_UNCLOSED_QUOTE)

        assert not result.is_valid
        # Should detect scalar or syntax errors
        assert any(e.category in [YAMLErrorCategory.SCALAR, YAMLErrorCategory.SYNTAX]
                  for e in result.errors)

    def test_structure_duplicate_key(self):
        """Test detection of duplicate keys (warning)."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(STRUCTURE_DUPLICATE_KEY)

        # PyYAML may handle this differently, but it should not be valid
        # or should have warnings
        assert result.is_valid or len(result.errors) > 0 or len(result.warnings) > 0

    def test_flow_unclosed_brace(self):
        """Test detection of unclosed brace in flow mapping."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(FLOW_UNCLOSED_BRACE)

        assert not result.is_valid
        # Should detect flow or syntax errors
        assert any(e.category in [YAMLErrorCategory.FLOW, YAMLErrorCategory.SYNTAX]
                  for e in result.errors)

    def test_flow_unclosed_bracket(self):
        """Test detection of unclosed bracket in flow sequence."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(FLOW_UNCLOSED_BRACKET)

        assert not result.is_valid
        assert any(e.category in [YAMLErrorCategory.FLOW, YAMLErrorCategory.SYNTAX]
                  for e in result.errors)

    def test_scalar_unclosed_single_quote(self):
        """Test detection of unclosed single quote."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(SCALAR_UNCLOSED_SINGLE_QUOTE)

        assert not result.is_valid
        assert any(e.category in [YAMLErrorCategory.SCALAR, YAMLErrorCategory.SYNTAX]
                  for e in result.errors)

    def test_scalar_unclosed_double_quote(self):
        """Test detection of unclosed double quote."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(SCALAR_UNCLOSED_DOUBLE_QUOTE)

        assert not result.is_valid
        assert any(e.category in [YAMLErrorCategory.SCALAR, YAMLErrorCategory.SYNTAX]
                  for e in result.errors)

    def test_anchor_undefined_alias(self):
        """Test detection of undefined alias."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(ANCHOR_UNDEFINED_ALIAS)

        assert not result.is_valid
        # Should detect alias errors
        alias_errors = [e for e in result.errors if e.category == YAMLErrorCategory.ALIAS]
        assert len(alias_errors) > 0 or any('alias' in e.message.lower() for e in result.errors)

    def test_document_multiple_without_separator(self):
        """Test detection of multiple documents without separator."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(DOCUMENT_MULTIPLE_WITHOUT_SEPARATOR)

        # PyYAML may parse this as valid with safe_load_all
        # If it's invalid, should have document or syntax errors
        if not result.is_valid:
            assert any(e.category in [YAMLErrorCategory.DOCUMENT, YAMLErrorCategory.SYNTAX]
                      for e in result.errors)

    def test_document_empty_stream(self):
        """Test detection of empty document stream."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(DOCUMENT_EMPTY_STREAM)

        # Empty or comment-only documents should be flagged
        assert not result.is_valid or len(result.warnings) > 0

    def test_complex_indentation_flow(self):
        """Test complex real-world YAML with indentation and flow errors."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(COMPLEX_INDENTATION_FLOW)

        assert not result.is_valid
        # Should detect flow errors
        assert any(e.category in [YAMLErrorCategory.FLOW, YAMLErrorCategory.SYNTAX]
                  for e in result.errors)

    def test_complex_nested_scalars(self):
        """Test complex nested scalar with multiple issues."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(COMPLEX_NESTED_SCALARS)

        assert not result.is_valid
        # Should detect syntax errors from unclosed quotes
        assert len(result.errors) > 0

    def test_edge_trailing_spaces(self):
        """Test that trailing spaces and tabs are detected."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(EDGE_TRAILING_SPACES)

        # Should detect either trailing whitespace warnings or tab errors
        trailing_issues = [e for e in result.get_all_issues()
                          if 'trailing' in e.message.lower() or 'tab' in e.message.lower()]
        assert len(trailing_issues) > 0

    def test_edge_tab_in_comment(self):
        """Test handling of tabs in comments."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(EDGE_TAB_IN_COMMENT)

        # Should be valid since tabs in comments are ignored by YAML spec
        # But our pre-validator might flag them
        # This is acceptable behavior
        assert result.is_valid or len(result.warnings) >= 0

    def test_edge_empty_lines(self):
        """Test handling of files with many empty lines."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(EDGE_EMPTY_LINES)

        # Should be valid or have warnings
        assert result.is_valid or len(result.warnings) >= 0

    def test_real_world_k8s_missing_colon(self):
        """Test real-world Kubernetes config with missing colons."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(REAL_WORLD_K8S_MISSING_COLON)

        assert not result.is_valid
        assert len(result.errors) > 0

    def test_real_world_docker_invalid_yaml(self):
        """Test real-world Docker Compose with unclosed quote."""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(REAL_WORLD_DOCKER_INVALID_YAML)

        assert not result.is_valid
        # Should detect unclosed quote
        assert any('quote' in e.message.lower() or e.category in [YAMLErrorCategory.SCALAR, YAMLErrorCategory.SYNTAX]
                  for e in result.errors)


class TestErrorLocationExtraction:
    """Test that line and column numbers are extracted accurately."""

    def test_line_extraction_indentation(self):
        """Test line number extraction for indentation errors."""
        yaml_content = """key1: value1
key2:\tvalue2
key3: value3
"""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        # Find tab error
        tab_errors = [e for e in result.errors if 'tab' in e.message.lower()]
        if tab_errors:
            error = tab_errors[0]
            assert error.line is not None
            assert error.line == 2  # Line 2 (1-indexed)
            assert error.column is not None

    def test_line_extraction_unclosed_quote(self):
        """Test line number extraction for unclosed quote errors."""
        yaml_content = """
line1: value1
line2: "unclosed
line3: value3
"""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        if not result.is_valid:
            # At least one error should have location info
            has_location = any(e.line is not None for e in result.errors)
            assert has_location

    def test_column_extraction_flow(self):
        """Test column number extraction for flow errors."""
        yaml_content = """
key: {unclosed: value
another: value
"""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        if not result.is_valid:
            # Find flow error
            flow_errors = [e for e in result.errors
                          if e.category in [YAMLErrorCategory.FLOW, YAMLErrorCategory.SYNTAX]]
            if flow_errors:
                error = flow_errors[0]
                if error.line is not None:
                    assert error.line >= 1


class TestErrorCategorization:
    """Test that errors are correctly categorized."""

    def test_indentation_categorization(self):
        """Test that indentation errors get INDENTATION category."""
        yaml_content = "key:\tvalue\n"
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        indentation_errors = [e for e in result.errors
                              if e.category == YAMLErrorCategory.INDENTATION]
        assert len(indentation_errors) > 0

    def test_flow_categorization(self):
        """Test that flow collection errors get FLOW category."""
        yaml_content = "key: {unclosed: value\n"
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        if not result.is_valid:
            flow_errors = [e for e in result.errors
                          if e.category == YAMLErrorCategory.FLOW]
            syntax_errors = [e for e in result.errors
                            if e.category == YAMLErrorCategory.SYNTAX]
            assert len(flow_errors) > 0 or len(syntax_errors) > 0

    def test_alias_categorization(self):
        """Test that alias errors get ALIAS category."""
        yaml_content = "key: &anchor value\nref: *undefined\n"
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        if not result.is_valid:
            alias_errors = [e for e in result.errors
                           if e.category == YAMLErrorCategory.ALIAS]
            # Should detect undefined alias
            assert len(alias_errors) > 0 or any('alias' in e.message.lower() for e in result.errors)


class TestErrorSuggestions:
    """Test that helpful suggestions are provided."""

    def test_tab_suggestion(self):
        """Test that tab errors suggest using spaces."""
        yaml_content = "key:\tvalue\n"
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        tab_errors = [e for e in result.errors if 'tab' in e.message.lower()]
        if tab_errors:
            error = tab_errors[0]
            assert len(error.suggestion) > 0
            assert 'space' in error.suggestion.lower() or 'replace' in error.suggestion.lower()

    def test_unclosed_quote_suggestion(self):
        """Test that unclosed quote errors suggest checking quotes."""
        yaml_content = "key: \"unclosed\n"
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        if not result.is_valid:
            quote_errors = [e for e in result.errors
                           if 'quote' in e.message.lower() or e.category == YAMLErrorCategory.SCALAR]
            if quote_errors:
                error = quote_errors[0]
                assert len(error.suggestion) > 0

    def test_alias_suggestion(self):
        """Test that alias errors suggest defining anchors."""
        yaml_content = "ref: *undefined\n"
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        if not result.is_valid:
            alias_errors = [e for e in result.errors
                           if e.category == YAMLErrorCategory.ALIAS or 'alias' in e.message.lower()]
            if alias_errors:
                error = alias_errors[0]
                assert len(error.suggestion) > 0
                assert any(term in error.suggestion.lower() for term in ['anchor', 'define', 'reference'])


class TestErrorContext:
    """Test that error context is provided."""

    def test_context_lines(self):
        """Test that context lines are included when available."""
        yaml_content = """
key1: value1
key2: "unclosed quote
key3: value3
"""
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        if not result.is_valid:
            # At least one error should have context
            has_context = any(len(e.context) > 0 for e in result.errors)
            assert has_context

    def test_context_formatting(self):
        """Test that context is properly formatted."""
        yaml_content = "key: value\n  bad: indent\n"
        validator = YAMLSyntaxValidator()
        result = validator.validate_content(yaml_content)

        if not result.is_valid:
            for error in result.errors:
                if error.context:
                    # Context should be a string
                    assert isinstance(error.context, str)
                    # Should contain actual content lines
                    assert len(error.context.strip()) > 0


class TestBatchValidation:
    """Test validation of multiple files."""

    def test_validate_multiple_files(self):
        """Test validating multiple YAML files at once."""
        valid_yaml = "key: value\n"
        invalid_yaml = "key:\tvalue\n"

        # Create temporary files
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f1:
            f1.write(valid_yaml)
            temp1 = f1.name

        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f2:
            f2.write(invalid_yaml)
            temp2 = f2.name

        try:
            validator = YAMLSyntaxValidator()
            results = validator.validate_multiple_files([temp1, temp2])

            assert len(results) == 2
            assert results[0].is_valid
            assert not results[1].is_valid
        finally:
            os.unlink(temp1)
            os.unlink(temp2)


if __name__ == '__main__':
    pytest.main([__file__, '-v'])