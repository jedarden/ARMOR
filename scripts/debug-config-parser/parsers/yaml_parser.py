"""
YAML configuration file parser.
Handles standard YAML, multi-document YAML, and YAML-like custom formats.
"""

import os
from pathlib import Path
from typing import Dict, List, Any, Optional
from dataclasses import dataclass
from enum import Enum


class YAMLErrorType(Enum):
    """Categories of YAML errors."""
    SYNTAX = "syntax"
    INDENTATION = "indentation"
    STRUCTURE = "structure"
    SCHEMA = "schema"
    IO = "io"
    UNKNOWN = "unknown"


@dataclass
class YAMLErrorDetail:
    """Detailed information about a YAML error."""
    error_type: YAMLErrorType
    message: str
    line: Optional[int] = None
    column: Optional[int] = None
    context: Optional[str] = None
    suggestion: Optional[str] = None


@dataclass
class YAMLParseResult:
    """Result of parsing a YAML file."""
    file_path: str
    status: str  # 'success', 'error', 'warning'
    documents: int
    error_message: Optional[str] = None
    warning_message: Optional[str] = None
    data: Any = None
    error_detail: Optional[YAMLErrorDetail] = None
    error_type: Optional[YAMLErrorType] = None


class YAMLParser:
    """Parser for YAML configuration files."""

    def __init__(self):
        self.yaml = None
        self._import_yaml()

    def _import_yaml(self):
        """Import PyYAML with fallback handling."""
        try:
            import yaml
            self.yaml = yaml
        except ImportError:
            # Try to install via nix-shell
            if self._try_nix_shell():
                import yaml
                self.yaml = yaml
            else:
                raise RuntimeError(
                    "PyYAML is required but not available. "
                    "Install it via: nix-shell -p python3.pkgs.pyyaml"
                )

    def _try_nix_shell(self) -> bool:
        """Attempt to use nix-shell for dependencies."""
        try:
            # Check if we're in a nix-shell
            if os.path.exists('/nix'):
                import yaml
                return True
            return False
        except ImportError:
            return False

    def _extract_error_context(self, error: Exception, filepath: str, content: str) -> YAMLErrorDetail:
        """
        Extract detailed error information from a YAML exception.

        Args:
            error: The YAML exception
            filepath: Path to the file being parsed
            content: File content

        Returns:
            YAMLErrorDetail with parsed error information
        """
        error_msg = str(error)
        line = None
        column = None
        error_type = YAMLErrorType.UNKNOWN
        suggestion = None
        context = None

        # Try to extract line/column from error message
        if hasattr(error, 'problem_mark'):
            mark = error.problem_mark
            if mark:
                line = mark.line + 1  # PyYAML uses 0-based indexing
                column = mark.column + 1

                # Extract context line
                lines = content.split('\n')
                if 0 <= line - 1 < len(lines):
                    context = lines[line - 1].strip()

        # Categorize error based on message content
        error_msg_lower = error_msg.lower()

        if any(term in error_msg_lower for term in ['indentation', 'inconsistent indentation']):
            error_type = YAMLErrorType.INDENTATION
            suggestion = "Check that all indentation uses consistent spacing (spaces or tabs, not both)"
        elif any(term in error_msg_lower for term in ['mapping', 'sequence', 'expected', 'unexpected']):
            error_type = YAMLErrorType.STRUCTURE
            suggestion = "Verify the YAML structure matches expected format (mappings use ':', sequences use '-'"
        elif any(term in error_msg_lower for term in ['syntax', 'parse', 'scanner']):
            error_type = YAMLErrorType.SYNTAX
            suggestion = "Check for malformed YAML syntax (unmatched quotes, brackets, etc.)"
        elif any(term in error_msg_lower for term in ['file', 'not found', 'permission', 'read']):
            error_type = YAMLErrorType.IO
            suggestion = "Verify the file exists and is readable"
        else:
            error_type = YAMLErrorType.UNKNOWN
            suggestion = "Review the YAML syntax and structure"

        # Build detailed message
        detailed_message = error_msg
        if line is not None:
            location = f"Line {line}"
            if column is not None:
                location += f", Column {column}"
            detailed_message = f"{location}: {error_msg}"

            if context:
                detailed_message += f"\n  Context: {context}"

        return YAMLErrorDetail(
            error_type=error_type,
            message=detailed_message,
            line=line,
            column=column,
            context=context,
            suggestion=suggestion
        )

    def _format_error_message(self, error_detail: YAMLErrorDetail) -> str:
        """
        Format a human-readable error message.

        Args:
            error_detail: Detailed error information

        Returns:
            Formatted error message
        """
        lines = []

        # Add error type and location
        if error_detail.line is not None:
            location = f"Line {error_detail.line}"
            if error_detail.column is not None:
                location += f", Column {error_detail.column}"
            lines.append(f"Location: {location}")

        # Add error type
        lines.append(f"Error Type: {error_detail.error_type.value.upper()}")

        # Add the actual error message
        lines.append(f"Message: {error_detail.message}")

        # Add context if available
        if error_detail.context:
            lines.append(f"Context: {error_detail.context}")

        # Add suggestion
        if error_detail.suggestion:
            lines.append(f"Suggestion: {error_detail.suggestion}")

        return "\n  ".join(lines)

    def parse_file(self, filepath: str) -> YAMLParseResult:
        """
        Parse a YAML file and return structured results.

        Args:
            filepath: Path to the YAML file to parse

        Returns:
            YAMLParseResult with parse status and data
        """
        if not self.yaml:
            return YAMLParseResult(
                file_path=filepath,
                status='error',
                documents=0,
                error_message='PyYAML not available',
                error_type=YAMLErrorType.UNKNOWN
            )

        content = ""
        try:
            with open(filepath, 'r') as f:
                content = f.read()

            # Check if file is empty
            if not content.strip():
                return YAMLParseResult(
                    file_path=filepath,
                    status='warning',
                    documents=0,
                    warning_message='File is empty'
                )

            # Try parsing as multi-document YAML first
            try:
                docs = list(self.yaml.safe_load_all(content))
                return YAMLParseResult(
                    file_path=filepath,
                    status='success',
                    documents=len(docs),
                    data=docs if len(docs) > 1 else docs[0] if docs else None
                )
            except self.yaml.YAMLError as e:
                # If multi-document fails, try single document
                try:
                    data = self.yaml.safe_load(content)
                    return YAMLParseResult(
                        file_path=filepath,
                        status='success',
                        documents=1,
                        data=data
                    )
                except self.yaml.YAMLError as e2:
                    error_detail = self._extract_error_context(e2, filepath, content)
                    return YAMLParseResult(
                        file_path=filepath,
                        status='error',
                        documents=0,
                        error_message=self._format_error_message(error_detail),
                        error_detail=error_detail,
                        error_type=error_detail.error_type
                    )

        except FileNotFoundError as e:
            error_detail = YAMLErrorDetail(
                error_type=YAMLErrorType.IO,
                message=f'File not found: {filepath}',
                suggestion='Verify the file path is correct'
            )
            return YAMLParseResult(
                file_path=filepath,
                status='error',
                documents=0,
                error_message=self._format_error_message(error_detail),
                error_detail=error_detail,
                error_type=error_detail.error_type
            )
        except Exception as e:
            error_detail = YAMLErrorDetail(
                error_type=YAMLErrorType.UNKNOWN,
                message=f'Unexpected error: {str(e)}',
                suggestion='Check file permissions and format'
            )
            return YAMLParseResult(
                file_path=filepath,
                status='error',
                documents=0,
                error_message=self._format_error_message(error_detail),
                error_detail=error_detail,
                error_type=error_detail.error_type
            )

    def parse_multiple(self, filepaths: List[str]) -> List[YAMLParseResult]:
        """
        Parse multiple YAML files.

        Args:
            filepaths: List of paths to YAML files

        Returns:
            List of YAMLParseResult objects
        """
        results = []
        for filepath in filepaths:
            results.append(self.parse_file(filepath))
        return results

    def validate_syntax(self, filepath: str) -> tuple[bool, Optional[str], Optional[YAMLErrorDetail]]:
        """
        Validate YAML syntax with detailed error reporting.

        Args:
            filepath: Path to the YAML file to validate

        Returns:
            Tuple of (is_valid, error_message, error_detail)
        """
        result = self.parse_file(filepath)
        return (result.status == 'success', result.error_message, result.error_detail)

    def categorize_error(self, filepath: str) -> Dict[str, Any]:
        """
        Categorize YAML errors by type and provide summary.

        Args:
            filepath: Path to the YAML file to analyze

        Returns:
            Dict with error categorization and statistics
        """
        result = self.parse_file(filepath)

        if result.status == 'success':
            return {
                'valid': True,
                'error_type': None,
                'error_count': 0,
                'summary': 'No errors detected'
            }

        error_detail = result.error_detail
        if not error_detail:
            return {
                'valid': False,
                'error_type': YAMLErrorType.UNKNOWN,
                'error_count': 1,
                'summary': 'Unknown error occurred'
            }

        return {
            'valid': False,
            'error_type': error_detail.error_type.value,
            'error_category': error_detail.error_type.value.upper(),
            'error_count': 1,
            'line': error_detail.line,
            'column': error_detail.column,
            'has_context': error_detail.context is not None,
            'has_suggestion': error_detail.suggestion is not None,
            'summary': f"{error_detail.error_type.value.upper()} error at line {error_detail.line}"
        }

    def detect_custom_format(self, filepath: str) -> Dict[str, Any]:
        """
        Detect if a YAML file uses custom/non-standard formatting.

        Args:
            filepath: Path to the YAML file

        Returns:
            Dict with format analysis results
        """
        try:
            with open(filepath, 'r') as f:
                content = f.read()

            analysis = {
                'uses_indentation': True,
                'has_colon_separators': ':' in content,
                'has_hashes': '#' in content,
                'likely_custom': False,
                'custom_indicators': []
            }

            # Check for custom format indicators
            if 'export ' in content:
                analysis['custom_indicators'].append('shell_export_statements')
                analysis['likely_custom'] = True

            if content.count('=') > content.count(':'):
                analysis['custom_indicators'].append('key_equals_value_pairs')
                analysis['likely_custom'] = True

            return analysis

        except Exception as e:
            return {
                'error': str(e),
                'likely_custom': False
            }
