"""
Core YAML Parser with Safe Load and Explicit Error Handling

This module provides the fundamental YAML parsing function with explicit
handling of PyYAML exception types (YAMLError, ScannerError, ParserError).
It serves as the core parsing primitive for higher-level YAML operations.
"""

from typing import Any, Optional, Union
from dataclasses import dataclass

from .error_types import (
    YAMLErrorCategory,
    YAMLErrorSeverity,
    YAMLErrorDetail,
)


# ============================================================================
# Result Structure
# ============================================================================

@dataclass
class SafeLoadResult:
    """
    Result of safe_load YAML parsing operation.

    Attributes:
        success: Whether parsing succeeded
        data: Parsed YAML content (None if failed)
        error: Detailed error information (None if succeeded)
        raw_exception: The original exception for debugging (None if succeeded)
    """
    success: bool
    data: Optional[Any] = None
    error: Optional[YAMLErrorDetail] = None
    raw_exception: Optional[Exception] = None

    def is_success(self) -> bool:
        """Check if parsing was successful."""
        return self.success

    def is_error(self) -> bool:
        """Check if parsing failed."""
        return not self.success

    def get_data(self) -> Any:
        """
        Get parsed data, raising error if parsing failed.

        Returns:
            Parsed YAML data

        Raises:
            RuntimeError: If parsing failed
        """
        if not self.success:
            raise RuntimeError(f"Cannot get data from failed parse: {self.error}")
        return self.data

    def get_error(self) -> Optional[YAMLErrorDetail]:
        """Get error detail if parsing failed."""
        return self.error


# ============================================================================
# Core Parser Implementation
# ============================================================================

class YAMLCoreParser:
    """
    Core YAML parser with explicit exception handling.

    Provides safe_load wrapper with explicit handling of:
    - yaml.YAMLError (base class)
    - yaml.ScannerError (lexical scanning errors)
    - yaml.ParserError (structural parsing errors)

    This class focuses solely on parsing YAML content strings.
    For file operations, see YAMLFileReader in reader.py.
    """

    def __init__(self):
        """Initialize the core YAML parser."""
        self.yaml = None
        self.ScannerError = None
        self.ParserError = None
        self._import_yaml()

    def _import_yaml(self) -> None:
        """
        Import PyYAML module with explicit exception type references.

        In PyYAML 6.0+, exception types are in submodules:
        - yaml.scanner.ScannerError (lexical scanning errors)
        - yaml.parser.ParserError (structural parsing errors)
        - yaml.YAMLError (base class for all YAML errors)

        Raises:
            RuntimeError: If PyYAML is not available
        """
        try:
            import yaml
            self.yaml = yaml
            # In PyYAML 6.0+, exception types are in submodules
            self.ScannerError = yaml.scanner.ScannerError
            self.ParserError = yaml.parser.ParserError
        except (ImportError, AttributeError) as e:
            raise RuntimeError(
                f"PyYAML is required but not available: {e}. "
                "Install it via: nix-shell -p python3.pkgs.pyyaml"
            )

    def safe_load(
        self,
        yaml_content: str,
        source: str = "<string>"
    ) -> SafeLoadResult:
        """
        Parse YAML content using safe_load with explicit error handling.

        This method provides explicit handling for specific PyYAML exception
        types to enable granular error categorization and reporting.

        Args:
            yaml_content: YAML content as string
            source: Source identifier for error messages (default: "<string>")

        Returns:
            SafeLoadResult with parsed data or error details

        Example:
            parser = YAMLCoreParser()
            result = parser.safe_load("key: value")
            if result.is_success():
                print(result.data)
            else:
                print(f"Error: {result.error}")
        """
        # Validate input
        if yaml_content is None:
            return SafeLoadResult(
                success=False,
                error=YAMLErrorDetail(
                    category=YAMLErrorCategory.UNKNOWN,
                    severity=YAMLErrorSeverity.CRITICAL,
                    message="YAML content cannot be None"
                ),
                raw_exception=None
            )

        # Check for empty content
        if not isinstance(yaml_content, str):
            return SafeLoadResult(
                success=False,
                error=YAMLErrorDetail(
                    category=YAMLErrorCategory.UNKNOWN,
                    severity=YAMLErrorSeverity.CRITICAL,
                    message=f"YAML content must be string, got {type(yaml_content).__name__}"
                ),
                raw_exception=None
            )

        if not yaml_content.strip():
            return SafeLoadResult(
                success=False,
                error=YAMLErrorDetail(
                    category=YAMLErrorCategory.DOCUMENT,
                    severity=YAMLErrorSeverity.ERROR,
                    message="Empty YAML content",
                    context="The YAML string contains no content",
                    suggestion="Add YAML content to parse"
                ),
                raw_exception=None
            )

        # Try to parse with explicit exception handling
        try:
            data = self.yaml.safe_load(yaml_content)
            return SafeLoadResult(success=True, data=data)

        except self.ScannerError as e:
            # Lexical scanning errors (indentation, special characters, etc.)
            error_detail = self._extract_scanner_error(e, yaml_content, source)
            return SafeLoadResult(
                success=False,
                error=error_detail,
                raw_exception=e
            )

        except self.ParserError as e:
            # Structural parsing errors (block structure, flow collections, etc.)
            error_detail = self._extract_parser_error(e, yaml_content, source)
            return SafeLoadResult(
                success=False,
                error=error_detail,
                raw_exception=e
            )

        except self.yaml.YAMLError as e:
            # Generic YAML errors (shouldn't reach here if caught above)
            error_detail = self._extract_generic_yaml_error(e, yaml_content, source)
            return SafeLoadResult(
                success=False,
                error=error_detail,
                raw_exception=e
            )

        except Exception as e:
            # Unexpected errors (not YAML-related)
            return SafeLoadResult(
                success=False,
                error=YAMLErrorDetail(
                    category=YAMLErrorCategory.UNKNOWN,
                    severity=YAMLErrorSeverity.CRITICAL,
                    message=f"Unexpected error during YAML parsing: {str(e)}",
                    context="An unexpected non-YAML error occurred"
                ),
                raw_exception=e
            )

    def _extract_scanner_error(
        self,
        error: Exception,
        content: str,
        source: str
    ) -> YAMLErrorDetail:
        """
        Extract detailed information from a ScannerError.

        Scanner errors occur during lexical scanning and typically relate to:
        - Indentation issues
        - Invalid characters
        - Tab vs space issues
        - Quoted string problems

        Args:
            error: The ScannerError exception
            content: YAML content being parsed
            source: Source identifier

        Returns:
            YAMLErrorDetail with scanner error information
        """
        line = None
        column = None
        context_lines = []
        problem = str(error)

        # Extract location information
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
        if hasattr(error, 'problem'):
            problem = error.problem

        # Build context string
        context = ""
        if context_lines:
            context = "\n  ".join(context_lines)

        # Generate scanner-specific suggestion
        suggestion = self._generate_scanner_suggestion(problem, str(error))

        return YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.ERROR,
            line=line,
            column=column,
            message=problem if problem else str(error),
            context=context,
            suggestion=suggestion
        )

    def _extract_parser_error(
        self,
        error: Exception,
        content: str,
        source: str
    ) -> YAMLErrorDetail:
        """
        Extract detailed information from a ParserError.

        Parser errors occur during structural parsing and typically relate to:
        - Invalid block structure
        - Malformed flow collections
        - Document separator issues
        - Invalid mapping/sequence syntax

        Args:
            error: The ParserError exception
            content: YAML content being parsed
            source: Source identifier

        Returns:
            YAMLErrorDetail with parser error information
        """
        line = None
        column = None
        context_lines = []
        problem = str(error)

        # Extract location information
        if hasattr(error, 'problem_mark'):
            mark = error.problem_mark
            if mark:
                line = mark.line + 1
                column = mark.column + 1

                # Get context lines
                content_lines = content.split('\n')
                if 0 <= line - 1 < len(content_lines):
                    context_lines = content_lines[max(0, line-2):min(len(content_lines), line+1)]

        # Extract problem description
        if hasattr(error, 'problem'):
            problem = error.problem

        # Build context string
        context = ""
        if context_lines:
            context = "\n  ".join(context_lines)

        # Generate parser-specific suggestion
        suggestion = self._generate_parser_suggestion(problem, str(error))

        # Categorize based on problem type
        category = YAMLErrorCategory.STRUCTURE
        problem_lower = problem.lower() if problem else ""
        if 'mapping' in problem_lower or 'indentation' in problem_lower:
            category = YAMLErrorCategory.SYNTAX
        elif 'flow' in problem_lower:
            category = YAMLErrorCategory.FLOW

        return YAMLErrorDetail(
            category=category,
            severity=YAMLErrorSeverity.ERROR,
            line=line,
            column=column,
            message=problem if problem else str(error),
            context=context,
            suggestion=suggestion
        )

    def _extract_generic_yaml_error(
        self,
        error: Exception,
        content: str,
        source: str
    ) -> YAMLErrorDetail:
        """
        Extract detailed information from a generic YAMLError.

        Args:
            error: The YAMLError exception
            content: YAML content being parsed
            source: Source identifier

        Returns:
            YAMLErrorDetail with generic error information
        """
        line = None
        column = None

        # Try to extract location
        if hasattr(error, 'problem_mark'):
            mark = error.problem_mark
            if mark:
                line = mark.line + 1
                column = mark.column + 1

        # Extract problem description
        problem = str(error)
        if hasattr(error, 'problem'):
            problem = error.problem

        return YAMLErrorDetail(
            category=YAMLErrorCategory.UNKNOWN,
            severity=YAMLErrorSeverity.ERROR,
            line=line,
            column=column,
            message=problem,
            context="Generic YAML parsing error",
            suggestion="Review YAML syntax and structure"
        )

    def _generate_scanner_suggestion(self, problem: str, error_msg: str) -> str:
        """Generate helpful suggestions for scanner errors."""
        error_lower = error_msg.lower()
        problem_lower = problem.lower() if problem else ""

        # Tab character errors
        if 'tab' in error_lower:
            return "Replace tabs with spaces - YAML requires spaces for indentation"

        # Indentation errors
        if 'indent' in error_lower or 'indentation' in error_lower:
            return "Check that indentation is consistent (use 2 or 4 spaces per level, never mix)"

        # Character errors
        if 'character' in error_lower and 'unexpected' in error_lower:
            return "Check for special characters that need to be quoted or escaped"

        # Scanner flow errors
        if 'flow' in error_lower and 'context' in error_lower:
            return "Ensure flow collections ({}, []) are properly formatted"

        # Quoted string errors
        if 'quoted string' in error_lower:
            return "Ensure quoted strings are properly terminated with matching quotes"

        return "Check YAML syntax at the reported location for common issues"

    def _generate_parser_suggestion(self, problem: str, error_msg: str) -> str:
        """Generate helpful suggestions for parser errors."""
        error_lower = error_msg.lower()
        problem_lower = problem.lower() if problem else ""

        # Mapping value errors
        if 'mapping values' in error_lower and 'not allowed' in error_lower:
            return "Check that key-value pairs use 'key: value' syntax with proper indentation"

        # Block structure errors
        if 'block' in error_lower:
            return "Review block structure - ensure proper indentation for mappings and sequences"

        # Flow collection errors
        if 'flow' in error_lower:
            return "Ensure flow collections are properly formatted: {key: value} for mappings, [items] for sequences"

        # Document separator errors
        if 'document' in error_lower and 'separator' in error_lower:
            return "Use '---' to separate multiple documents in a file"

        # Sequence errors
        if 'sequence' in error_lower:
            return "Check sequence items use '- item' syntax with consistent indentation"

        return "Review YAML structure at the reported location"


# ============================================================================
# Convenience Functions
# ============================================================================

def safe_load_yaml(yaml_content: str, source: str = "<string>") -> SafeLoadResult:
    """
    Convenience function for quick YAML parsing with explicit error handling.

    Args:
        yaml_content: YAML content string
        source: Source identifier for error messages

    Returns:
        SafeLoadResult with parsed data or error details

    Example:
        result = safe_load_yaml("key: value")
        if result.is_success():
            print(result.data)
        else:
            print(f"Error at line {result.error.line}: {result.error.message}")
    """
    parser = YAMLCoreParser()
    return parser.safe_load(yaml_content, source)


# ============================================================================
# Exports
# ============================================================================

__all__ = [
    'YAMLCoreParser',
    'SafeLoadResult',
    'safe_load_yaml',
]
