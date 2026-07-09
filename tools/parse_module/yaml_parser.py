"""
YAML Parser Utility Module.

Provides a simple, safe YAML parser with proper error handling
and structured result objects.
"""

from pathlib import Path
from typing import Any, Optional
try:
    from .result import ParseResult, ParseStatus
except ImportError:
    from result import ParseResult, ParseStatus


class YAMLParser:
    """
    Simple YAML parser with safe_load wrapper and error handling.

    Uses yaml.safe_load() to safely parse YAML files without
    executing arbitrary Python objects.
    """

    def __init__(self):
        """Initialize the YAML parser."""
        self.yaml = None
        self._import_yaml()

    def _import_yaml(self) -> None:
        """
        Import PyYAML module with fallback handling.

        Raises:
            RuntimeError: If PyYAML is not available
        """
        try:
            import yaml
            self.yaml = yaml
        except ImportError:
            raise RuntimeError(
                "PyYAML is required but not available. "
                "Install it via: pip install pyyaml"
            )

    def parse_string(self, yaml_content: str) -> ParseResult:
        """
        Parse YAML from a string.

        Args:
            yaml_content: String containing YAML content

        Returns:
            ParseResult with status, data, and error fields
        """
        if not self.yaml:
            return ParseResult.make_error('PyYAML not available')

        if not yaml_content or not yaml_content.strip():
            return ParseResult.make_error('Empty YAML content')

        try:
            data = self.yaml.safe_load(yaml_content)
            return ParseResult.success(data)
        except self.yaml.YAMLError as e:
            error_msg = self._format_yaml_error(str(e))
            return ParseResult.make_error(error_msg)
        except Exception as e:
            return ParseResult.make_error(f'Unexpected error: {str(e)}')

    def parse_file(self, filepath: str) -> ParseResult:
        """
        Parse YAML from a file.

        Args:
            filepath: Path to the YAML file

        Returns:
            ParseResult with status, data, and error fields
        """
        path = Path(filepath)

        # Check if file exists
        if not path.exists():
            return ParseResult.make_error(f'File not found: {filepath}')

        # Check if it's a file (not directory)
        if not path.is_file():
            return ParseResult.make_error(f'Path is not a file: {filepath}')

        try:
            with open(path, 'r', encoding='utf-8') as f:
                content = f.read()

            return self.parse_string(content)

        except FileNotFoundError:
            return ParseResult.make_error(f'File not found: {filepath}')
        except PermissionError:
            return ParseResult.make_error(f'Permission denied: {filepath}')
        except UnicodeDecodeError as e:
            return ParseResult.make_error(f'Encoding error reading file: {str(e)}')
        except Exception as e:
            return ParseResult.make_error(f'Error reading file: {str(e)}')

    def _format_yaml_error(self, error_message: str) -> str:
        """
        Format YAML error message for better readability.

        Args:
            error_message: Raw error message from PyYAML

        Returns:
            Formatted error message
        """
        # Clean up common PyYAML error patterns
        error_message = error_message.strip()

        # Add context for common errors
        if 'could not find expected' in error_message.lower():
            return f"YAML syntax error: {error_message}. Check indentation and structure."
        elif 'mapping values are not allowed here' in error_message.lower():
            return f"YAML structure error: {error_message}. Check colons and indentation."
        elif 'duplicate key' in error_message.lower():
            return f"YAML validation error: {error_message}"

        return f"YAML parsing error: {error_message}"
