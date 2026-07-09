"""
TOML configuration file parser.
Handles standard TOML configuration files.
"""

from pathlib import Path
from typing import Dict, Any, Optional
from dataclasses import dataclass


@dataclass
class TOMLParseResult:
    """Result of parsing a TOML file."""
    file_path: str
    status: str  # 'success', 'error', 'warning'
    error_message: Optional[str] = None
    warning_message: Optional[str] = None
    data: Any = None


class TOMLParser:
    """Parser for TOML configuration files."""

    def __init__(self):
        self.tomli = None
        self._import_tomli()

    def _import_tomli(self):
        """Import tomli with fallback handling."""
        try:
            import tomli
            self.tomli = tomli
        except ImportError:
            try:
                import tomllib as tomli  # Python 3.11+
                self.tomli = tomli
            except ImportError:
                # tomli not available, will raise error when used
                pass

    def parse_file(self, filepath: str) -> TOMLParseResult:
        """
        Parse a TOML file and return structured results.

        Args:
            filepath: Path to the TOML file to parse

        Returns:
            TOMLParseResult with parse status and data
        """
        if not self.tomli:
            return TOMLParseResult(
                file_path=filepath,
                status='error',
                error_message='tomli module not available (install: pip install tomli)'
            )

        try:
            with open(filepath, 'rb') as f:
                content = f.read()

            # Check if file is empty
            if not content:
                return TOMLParseResult(
                    file_path=filepath,
                    status='warning',
                    warning_message='File is empty'
                )

            try:
                data = self.tomli.load(content)
                return TOMLParseResult(
                    file_path=filepath,
                    status='success',
                    data=data
                )
            except Exception as e:
                return TOMLParseResult(
                    file_path=filepath,
                    status='error',
                    error_message=f'TOML syntax error: {str(e)}'
                )

        except FileNotFoundError:
            return TOMLParseResult(
                file_path=filepath,
                status='error',
                error_message=f'File not found: {filepath}'
            )
        except Exception as e:
            return TOMLParseResult(
                file_path=filepath,
                status='error',
                error_message=f'Unexpected error: {str(e)}'
            )

    def parse_multiple(self, filepaths: list) -> list[TOMLParseResult]:
        """
        Parse multiple TOML files.

        Args:
            filepaths: List of paths to TOML files

        Returns:
            List of TOMLParseResult objects
        """
        results = []
        for filepath in filepaths:
            results.append(self.parse_file(filepath))
        return results

    def validate_syntax(self, filepath: str) -> tuple[bool, Optional[str]]:
        """
        Validate TOML syntax without loading content.

        Args:
            filepath: Path to the TOML file to validate

        Returns:
            Tuple of (is_valid, error_message)
        """
        result = self.parse_file(filepath)
        return (result.status == 'success', result.error_message)
