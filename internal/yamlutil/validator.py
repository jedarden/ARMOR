"""
YAML Syntax Validator with Comprehensive Error Detection
"""
import re
from pathlib import Path
from typing import Optional, List, Tuple
from dataclasses import dataclass

from .error_types import (
    YAMLErrorCategory,
    YAMLErrorSeverity,
    YAMLErrorDetail,
    YAMLValidationResult
)


class YAMLSyntaxValidator:
    """
    Comprehensive YAML syntax validator that detects and categorizes
    syntax errors with detailed line/column information.
    """

    def __init__(self):
        self.yaml = None
        self._import_yaml()

    def _import_yaml(self):
        """Import PyYAML with fallback handling."""
        try:
            import yaml
            self.yaml = yaml
        except ImportError:
            raise RuntimeError(
                "PyYAML is required but not available. "
                "Install it via: nix-shell -p python3.pkgs.pyyaml"
            )

    def validate_file(self, filepath: str) -> YAMLValidationResult:
        """
        Validate a YAML file for syntax errors.

        Args:
            filepath: Path to the YAML file to validate

        Returns:
            YAMLValidationResult with detailed error information
        """
        try:
            with open(filepath, 'r') as f:
                content = f.read()
        except Exception as e:
            return YAMLValidationResult(
                is_valid=False,
                errors=[YAMLErrorDetail(
                    category=YAMLErrorCategory.UNKNOWN,
                    severity=YAMLErrorSeverity.CRITICAL,
                    message=f"Failed to read file: {str(e)}"
                )],
                warnings=[]
            )

        return self.validate_content(content, filepath)

    def validate_content(self, content: str, source: str = "<string>") -> YAMLValidationResult:
        """
        Validate YAML content for syntax errors.

        Args:
            content: YAML content to validate
            source: Source identifier (filename or "<string>")

        Returns:
            YAMLValidationResult with detailed error information
        """
        errors = []
        warnings = []

        # Check for empty content
        if not content.strip():
            return YAMLValidationResult(
                is_valid=False,
                errors=[YAMLErrorDetail(
                    category=YAMLErrorCategory.DOCUMENT,
                    severity=YAMLErrorSeverity.ERROR,
                    message="Empty YAML document",
                    context="The file contains no content",
                    suggestion="Add YAML content to the file"
                )],
                warnings=[]
            )

        # Pre-validation checks for common issues
        pre_check_errors = self._pre_validate(content)
        errors.extend(pre_check_errors)

        # Try to parse the YAML
        try:
            # Try multi-document first
            docs = list(self.yaml.safe_load_all(content))

            # Check for parse warnings
            if len(docs) == 0:
                warnings.append(YAMLErrorDetail(
                    category=YAMLErrorCategory.DOCUMENT,
                    severity=YAMLErrorSeverity.INFO,
                    message="No documents found in YAML",
                    context="The file appears to be empty or contain only comments"
                ))

        except self.yaml.YAMLError as e:
            # Extract detailed error information
            error_detail = self._extract_error_detail(e, content, source)
            errors.append(error_detail)

        except Exception as e:
            errors.append(YAMLErrorDetail(
                category=YAMLErrorCategory.UNKNOWN,
                severity=YAMLErrorSeverity.CRITICAL,
                message=f"Unexpected error during parsing: {str(e)}"
            ))

        # Determine overall validity
        is_valid = not any(
            e.severity in [YAMLErrorSeverity.CRITICAL, YAMLErrorSeverity.ERROR]
            for e in errors
        )

        return YAMLValidationResult(
            is_valid=is_valid,
            errors=errors,
            warnings=warnings
        )

    def _pre_validate(self, content: str) -> List[YAMLErrorDetail]:
        """
        Perform pre-validation checks for common YAML issues.

        Args:
            content: YAML content to check

        Returns:
            List of YAMLErrorDetail objects found
        """
        errors = []
        lines = content.split('\n')

        for line_num, line in enumerate(lines, 1):
            # Check for tab characters (YAML forbids tabs for indentation)
            if '\t' in line:
                # Find column of first tab
                tab_col = line.index('\t') + 1
                errors.append(YAMLErrorDetail(
                    category=YAMLErrorCategory.INDENTATION,
                    severity=YAMLErrorSeverity.ERROR,
                    line=line_num,
                    column=tab_col,
                    message="Tab character found in YAML",
                    context="YAML requires spaces for indentation, not tabs",
                    suggestion="Replace tabs with spaces (typically 2 spaces per indentation level)"
                ))

            # Check for trailing whitespace on non-comment lines
            stripped = line.rstrip()
            if line != stripped and not stripped.strip().startswith('#'):
                trailing_ws = line[len(stripped):]
                if trailing_ws:
                    errors.append(YAMLErrorDetail(
                        category=YAMLErrorCategory.SYNTAX,
                        severity=YAMLErrorSeverity.WARNING,
                        line=line_num,
                        column=len(stripped) + 1,
                        message=f"Trailing whitespace: {repr(trailing_ws)}",
                        context="Line has trailing whitespace characters",
                        suggestion="Remove trailing whitespace for cleaner YAML"
                    ))

        return errors

    def _extract_error_detail(self, error: Exception, content: str, source: str) -> YAMLErrorDetail:
        """
        Extract detailed error information from a PyYAML exception.

        Args:
            error: The PyYAML exception
            content: The YAML content being parsed
            source: Source identifier

        Returns:
            YAMLErrorDetail with comprehensive error information
        """
        line = None
        column = None
        error_msg = str(error)
        context_lines = []

        # Extract line/column from problem_mark
        if hasattr(error, 'problem_mark'):
            mark = error.problem_mark
            if mark:
                line = mark.line + 1  # Convert to 1-indexed
                column = mark.column + 1

                # Get context lines
                content_lines = content.split('\n')
                if 0 <= line - 1 < len(content_lines):
                    context_lines = content_lines[max(0, line-2):min(len(content_lines), line+1)]

        # Extract problem description
        problem = ""
        if hasattr(error, 'problem'):
            problem = error.problem

        # Categorize the error
        category, severity = self._categorize_error(error, error_msg, problem)

        # Build helpful message
        message = problem if problem else error_msg

        # Add context if available
        context = ""
        if context_lines:
            context = "\n  ".join(context_lines)

        # Generate suggestion based on error type
        suggestion = self._generate_suggestion(category, problem, error_msg)

        return YAMLErrorDetail(
            category=category,
            severity=severity,
            line=line,
            column=column,
            message=message,
            context=context,
            suggestion=suggestion
        )

    def _categorize_error(self, error: Exception, error_msg: str, problem: str) -> Tuple[YAMLErrorCategory, YAMLErrorSeverity]:
        """
        Categorize a YAML error by type and severity.

        Args:
            error: The PyYAML exception
            error_msg: Full error message
            problem: Problem description

        Returns:
            Tuple of (category, severity)
        """
        error_msg_lower = error_msg.lower()
        problem_lower = problem.lower() if problem else ""

        # Indentation errors
        if any(term in error_msg_lower for term in ['indentation', 'indent', 'unexpected indent']):
            return YAMLErrorCategory.INDENTATION, YAMLErrorSeverity.ERROR

        # Syntax errors
        if any(term in error_msg_lower for term in ['syntax', 'while scanning', 'mapping values are not allowed here']):
            return YAMLErrorCategory.SYNTAX, YAMLErrorSeverity.ERROR

        # Structure errors
        if any(term in error_msg_lower for term in ['structure', 'block scalar', 'block sequence']):
            return YAMLErrorCategory.STRUCTURE, YAMLErrorSeverity.ERROR

        # Flow style errors ({} or [])
        if any(term in error_msg_lower for term in ['flow', 'flow collection', 'flow sequence']):
            return YAMLErrorCategory.FLOW, YAMLErrorSeverity.ERROR

        # Tag errors (!tag, !!type)
        if any(term in error_msg_lower for term in ['tag', 'resolver', 'constructor']):
            return YAMLErrorCategory.TAG, YAMLErrorSeverity.ERROR

        # Anchor/alias errors (&anchor, *alias)
        if any(term in error_msg_lower for term in ['anchor', 'alias', 'undefined alias']):
            if 'undefined' in error_msg_lower:
                return YAMLErrorCategory.ALIAS, YAMLErrorSeverity.ERROR
            return YAMLErrorCategory.ANCHOR, YAMLErrorSeverity.ERROR

        # Scalar errors (strings, numbers, booleans)
        if any(term in error_msg_lower for term in ['scalar', 'quoted string']):
            return YAMLErrorCategory.SCALAR, YAMLErrorSeverity.ERROR

        # Document errors
        if any(term in error_msg_lower for term in ['document', 'directives end mark']):
            return YAMLErrorCategory.DOCUMENT, YAMLErrorSeverity.ERROR

        # Default to syntax error for unknown issues
        return YAMLErrorCategory.SYNTAX, YAMLErrorSeverity.ERROR

    def _generate_suggestion(self, category: YAMLErrorCategory, problem: str, error_msg: str) -> str:
        """
        Generate a helpful suggestion for fixing the error.

        Args:
            category: The error category
            problem: The problem description
            error_msg: Full error message

        Returns:
            Helpful suggestion message
        """
        problem_lower = problem.lower() if problem else ""
        error_msg_lower = error_msg.lower()

        # Indentation-specific suggestions
        if category == YAMLErrorCategory.INDENTATION:
            if 'unexpected indent' in error_msg_lower:
                return "Check that indentation is consistent (use 2 or 4 spaces per level)"
            return "Ensure all indentation uses spaces, not tabs, and is consistent"

        # Flow collection suggestions
        if 'flow' in error_msg_lower:
            if 'not properly closed' in error_msg_lower:
                return "Ensure all { } and [ ] brackets are properly closed and matched"
            return "Check flow collection syntax: {key: value} for mappings, [item1, item2] for sequences"

        # Anchor/alias suggestions
        if category in [YAMLErrorCategory.ANCHOR, YAMLErrorCategory.ALIAS]:
            if 'undefined' in error_msg_lower:
                return "Ensure all aliases (*alias) reference a previously defined anchor (&anchor)"
            return "Define anchors with &name and reference them with *name"

        # Tag suggestions
        if 'tag' in error_msg_lower:
            return "Check custom tag syntax: !tagname or !!typename, ensure tag is properly registered"

        # Scalar suggestions
        if 'quoted string' in error_msg_lower:
            return "Ensure quoted strings are properly terminated with matching quotes"

        # Document suggestions
        if 'document' in error_msg_lower:
            if 'end mark' in error_msg_lower:
                return "Use '---' to separate multiple documents in a file"

        # General suggestions
        if 'mapping values' in error_msg_lower:
            return "Check that key-value pairs use 'key: value' syntax with proper indentation"

        if 'sequence' in error_msg_lower:
            return "Check sequence items use '- item' syntax with consistent indentation"

        return "Review the YAML syntax at the reported location and check for common errors"

    def validate_multiple_files(self, filepaths: List[str]) -> List[YAMLValidationResult]:
        """
        Validate multiple YAML files.

        Args:
            filepaths: List of paths to YAML files

        Returns:
            List of YAMLValidationResult objects
        """
        results = []
        for filepath in filepaths:
            results.append(self.validate_file(filepath))
        return results


# Convenience functions for quick validation
def validate_yaml_file(filepath: str) -> YAMLValidationResult:
    """Quick validation of a single YAML file."""
    validator = YAMLSyntaxValidator()
    return validator.validate_file(filepath)


def validate_yaml_string(content: str) -> YAMLValidationResult:
    """Quick validation of YAML content string."""
    validator = YAMLSyntaxValidator()
    return validator.validate_content(content)