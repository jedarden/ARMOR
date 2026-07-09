"""
JSON configuration file parser.
Handles standard JSON configuration files.
"""

import json
from pathlib import Path
from typing import Dict, Any, Optional
from dataclasses import dataclass


@dataclass
class JSONParseResult:
    """Result of parsing a JSON file."""
    file_path: str
    status: str  # 'success', 'error', 'warning'
    error_message: Optional[str] = None
    warning_message: Optional[str] = None
    data: Any = None


class JSONParser:
    """Parser for JSON configuration files."""

    def parse_file(self, filepath: str) -> JSONParseResult:
        """
        Parse a JSON file and return structured results.

        Args:
            filepath: Path to the JSON file to parse

        Returns:
            JSONParseResult with parse status and data
        """
        try:
            with open(filepath, 'r') as f:
                content = f.read()

            # Check if file is empty
            if not content.strip():
                return JSONParseResult(
                    file_path=filepath,
                    status='warning',
                    warning_message='File is empty'
                )

            try:
                data = json.loads(content)
                return JSONParseResult(
                    file_path=filepath,
                    status='success',
                    data=data
                )
            except json.JSONDecodeError as e:
                return JSONParseResult(
                    file_path=filepath,
                    status='error',
                    error_message=f'JSON syntax error at line {e.lineno}, column {e.colno}: {e.msg}'
                )

        except FileNotFoundError:
            return JSONParseResult(
                file_path=filepath,
                status='error',
                error_message=f'File not found: {filepath}'
            )
        except Exception as e:
            return JSONParseResult(
                file_path=filepath,
                status='error',
                error_message=f'Unexpected error: {str(e)}'
            )

    def parse_multiple(self, filepaths: list) -> list[JSONParseResult]:
        """
        Parse multiple JSON files.

        Args:
            filepaths: List of paths to JSON files

        Returns:
            List of JSONParseResult objects
        """
        results = []
        for filepath in filepaths:
            results.append(self.parse_file(filepath))
        return results

    def validate_syntax(self, filepath: str) -> tuple[bool, Optional[str]]:
        """
        Validate JSON syntax without loading content.

        Args:
            filepath: Path to the JSON file to validate

        Returns:
            Tuple of (is_valid, error_message)
        """
        result = self.parse_file(filepath)
        return (result.status == 'success', result.error_message)
