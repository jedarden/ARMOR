"""
Parser factory for creating appropriate parsers based on file type.
Provides unified interface for parsing different configuration file formats.
"""

from pathlib import Path
from typing import Dict, List, Any, Optional
from enum import Enum

from .yaml_parser import YAMLParser, YAMLParseResult
from .json_parser import JSONParser, JSONParseResult
from .toml_parser import TOMLParser, TOMLParseResult


class FileType(Enum):
    """Configuration file types."""
    YAML = 'yaml'
    JSON = 'json'
    TOML = 'toml'
    UNKNOWN = 'unknown'


class ParserFactory:
    """Factory for creating appropriate parsers for different file types."""

    def __init__(self):
        self.yaml_parser = YAMLParser()
        self.json_parser = JSONParser()
        self.toml_parser = TOMLParser()
        self._extension_map = {
            '.yaml': FileType.YAML,
            '.yml': FileType.YAML,
            '.json': FileType.JSON,
            '.toml': FileType.TOML,
        }

    def detect_file_type(self, filepath: str) -> FileType:
        """
        Detect file type based on extension.

        Args:
            filepath: Path to the file

        Returns:
            FileType enum value
        """
        suffix = Path(filepath).suffix.lower()
        return self._extension_map.get(suffix, FileType.UNKNOWN)

    def parse_file(self, filepath: str) -> Dict[str, Any]:
        """
        Parse a configuration file using the appropriate parser.

        Args:
            filepath: Path to the configuration file

        Returns:
            Dict containing parse results with keys: status, data, error, warnings
        """
        file_type = self.detect_file_type(filepath)

        if file_type == FileType.YAML:
            result = self.yaml_parser.parse_file(filepath)
            return {
                'file_type': 'yaml',
                'status': result.status,
                'documents': result.documents,
                'data': result.data,
                'error': result.error_message,
                'warning': result.warning_message
            }

        elif file_type == FileType.JSON:
            result = self.json_parser.parse_file(filepath)
            return {
                'file_type': 'json',
                'status': result.status,
                'data': result.data,
                'error': result.error_message,
                'warning': result.warning_message
            }

        elif file_type == FileType.TOML:
            result = self.toml_parser.parse_file(filepath)
            return {
                'file_type': 'toml',
                'status': result.status,
                'data': result.data,
                'error': result.error_message,
                'warning': result.warning_message
            }

        else:
            return {
                'file_type': 'unknown',
                'status': 'error',
                'error': f'Unknown file type: {filepath}'
            }

    def parse_multiple(self, filepaths: List[str]) -> List[Dict[str, Any]]:
        """
        Parse multiple configuration files.

        Args:
            filepaths: List of paths to configuration files

        Returns:
            List of parse result dictionaries
        """
        results = []
        for filepath in filepaths:
            results.append(self.parse_file(filepath))
        return results

    def validate_syntax(self, filepath: str) -> tuple[bool, Optional[str]]:
        """
        Validate configuration file syntax.

        Args:
            filepath: Path to the configuration file

        Returns:
            Tuple of (is_valid, error_message)
        """
        file_type = self.detect_file_type(filepath)

        if file_type == FileType.YAML:
            return self.yaml_parser.validate_syntax(filepath)
        elif file_type == FileType.JSON:
            return self.json_parser.validate_syntax(filepath)
        elif file_type == FileType.TOML:
            return self.toml_parser.validate_syntax(filepath)
        else:
            return (False, f'Unknown file type: {filepath}')

    def batch_validate(self, filepaths: List[str]) -> Dict[str, Any]:
        """
        Validate multiple configuration files and return summary.

        Args:
            filepaths: List of paths to configuration files

        Returns:
            Dict with validation summary and results
        """
        results = []
        success_count = 0
        error_count = 0
        warning_count = 0

        for filepath in filepaths:
            result = self.parse_file(filepath)
            results.append({
                'path': filepath,
                'result': result
            })

            if result['status'] == 'success':
                success_count += 1
            elif result['status'] == 'error':
                error_count += 1
            elif result['status'] == 'warning':
                warning_count += 1

        return {
            'total_files': len(filepaths),
            'successful': success_count,
            'errors': error_count,
            'warnings': warning_count,
            'results': results
        }
