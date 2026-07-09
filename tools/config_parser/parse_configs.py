#!/usr/bin/env python3
"""
Configuration File Parser for ARMOR Debug Infrastructure

This module provides comprehensive parsing and validation for debug configuration
files in YAML, JSON, and TOML formats. It supports syntax validation, error detection,
and detailed reporting.

Features:
- YAML parsing (via PyYAML)
- JSON parsing (via standard library)
- TOML parsing (via standard library)
- Syntax error detection and reporting
- File type auto-detection
- Batch processing support

Usage:
    python3 parse_configs.py <file_or_directory>
    python3 parse_configs.py --validate-all
"""

import os
import sys
import json
import ast
from pathlib import Path
from typing import Dict, List, Any, Optional, Tuple
from dataclasses import dataclass
from enum import Enum


class FileType(Enum):
    """Supported configuration file types."""
    YAML = "yaml"
    JSON = "json"
    TOML = "toml"
    UNKNOWN = "unknown"


@dataclass
class ParseResult:
    """Result of parsing a configuration file."""
    file_path: str
    file_type: FileType
    success: bool
    data: Optional[Any] = None
    error: Optional[str] = None
    error_line: Optional[int] = None
    error_column: Optional[int] = None


class ConfigParser:
    """Main configuration parser class."""

    def __init__(self):
        self.yaml_available = self._check_yaml_availability()
        if not self.yaml_available:
            print("Warning: PyYAML not available. YAML parsing will use basic syntax check only.")
            print("To enable full YAML parsing, use: nix-shell -p python3Packages.pyyaml")

    def _check_yaml_availability(self) -> bool:
        """Check if PyYAML is available."""
        try:
            import yaml
            return True
        except ImportError:
            return False

    def get_file_type(self, filepath: str) -> FileType:
        """Determine file type based on extension."""
        suffix = Path(filepath).suffix.lower()
        suffix_map = {
            '.yaml': FileType.YAML,
            '.yml': FileType.YAML,
            '.json': FileType.JSON,
            '.toml': FileType.TOML
        }
        return suffix_map.get(suffix, FileType.UNKNOWN)

    def parse_yaml(self, filepath: str) -> ParseResult:
        """Parse a YAML file."""
        try:
            import yaml
            with open(filepath, 'r') as f:
                data = yaml.safe_load(f)
            return ParseResult(
                file_path=filepath,
                file_type=FileType.YAML,
                success=True,
                data=data
            )
        except yaml.YAMLError as e:
            error_line = None
            error_column = None
            error_msg = str(e)

            # Extract line/column info if available
            if hasattr(e, 'problem_mark'):
                error_line = e.problem_mark.line + 1
                error_column = e.problem_mark.column + 1
                error_msg = f"Line {error_line}, Column {error_column}: {e.problem}"

            return ParseResult(
                file_path=filepath,
                file_type=FileType.YAML,
                success=False,
                error=error_msg,
                error_line=error_line,
                error_column=error_column
            )
        except Exception as e:
            return ParseResult(
                file_path=filepath,
                file_type=FileType.YAML,
                success=False,
                error=f"Unexpected error: {str(e)}"
            )

    def basic_yaml_syntax_check(self, filepath: str) -> ParseResult:
        """Basic YAML syntax check without PyYAML."""
        try:
            with open(filepath, 'r') as f:
                content = f.read()

            errors = []
            lines = content.split('\n')

            for i, line in enumerate(lines, 1):
                # Check for common YAML syntax errors
                stripped = line.lstrip()

                # Check for tab characters (YAML requires spaces)
                if '\t' in line:
                    errors.append(f"Line {i}: Contains tab character (use spaces for indentation)")

                # Check for trailing whitespace
                if line.rstrip() != line and line.strip():
                    # Comments can have trailing whitespace
                    if not line.strip().startswith('#'):
                        errors.append(f"Line {i}: Trailing whitespace")

                # Check indentation consistency
                if stripped and not stripped.startswith('#'):
                    indent = len(line) - len(stripped)
                    if indent % 2 != 0:
                        errors.append(f"Line {i}: Indentation not multiple of 2")

            if errors:
                return ParseResult(
                    file_path=filepath,
                    file_type=FileType.YAML,
                    success=False,
                    error="; ".join(errors)
                )

            return ParseResult(
                file_path=filepath,
                file_type=FileType.YAML,
                success=True,
                data=None  # Basic check doesn't parse data
            )
        except Exception as e:
            return ParseResult(
                file_path=filepath,
                file_type=FileType.YAML,
                success=False,
                error=f"Read error: {str(e)}"
            )

    def parse_json(self, filepath: str) -> ParseResult:
        """Parse a JSON file."""
        try:
            with open(filepath, 'r') as f:
                data = json.load(f)
            return ParseResult(
                file_path=filepath,
                file_type=FileType.JSON,
                success=True,
                data=data
            )
        except json.JSONDecodeError as e:
            return ParseResult(
                file_path=filepath,
                file_type=FileType.JSON,
                success=False,
                error=f"Line {e.lineno}, Column {e.colno}: {e.msg}",
                error_line=e.lineno,
                error_column=e.colno
            )
        except Exception as e:
            return ParseResult(
                file_path=filepath,
                file_type=FileType.JSON,
                success=False,
                error=f"Unexpected error: {str(e)}"
            )

    def parse_toml(self, filepath: str) -> ParseResult:
        """Parse a TOML file."""
        try:
            import tomllib
            with open(filepath, 'rb') as f:
                data = tomllib.load(f)
            return ParseResult(
                file_path=filepath,
                file_type=FileType.TOML,
                success=True,
                data=data
            )
        except Exception as e:
            # tomllib raises various exceptions
            error_msg = str(e)
            error_line = None
            error_column = None

            # Try to extract line/column info from error message
            if 'at line' in error_msg.lower():
                parts = error_msg.split('line')
                if len(parts) > 1:
                    try:
                        error_line = int(parts[1].split(',')[0].strip())
                    except (ValueError, IndexError):
                        pass

            return ParseResult(
                file_path=filepath,
                file_type=FileType.TOML,
                success=False,
                error=error_msg,
                error_line=error_line,
                error_column=error_column
            )

    def parse_file(self, filepath: str) -> ParseResult:
        """Parse a configuration file based on its type."""
        file_type = self.get_file_type(filepath)

        if file_type == FileType.YAML:
            if self.yaml_available:
                return self.parse_yaml(filepath)
            else:
                return self.basic_yaml_syntax_check(filepath)
        elif file_type == FileType.JSON:
            return self.parse_json(filepath)
        elif file_type == FileType.TOML:
            return self.parse_toml(filepath)
        else:
            return ParseResult(
                file_path=filepath,
                file_type=FileType.UNKNOWN,
                success=False,
                error=f"Unsupported file type: {Path(filepath).suffix}"
            )

    def find_config_files(self, directory: str, recursive: bool = True) -> List[Path]:
        """Find all configuration files in a directory."""
        directory = Path(directory)
        extensions = ['.yaml', '.yml', '.json', '.toml']

        if recursive:
            config_files = []
            for ext in extensions:
                config_files.extend(directory.rglob(f'*{ext}'))
            return config_files
        else:
            config_files = []
            for ext in extensions:
                config_files.extend(directory.glob(f'*{ext}'))
            return config_files

    def filter_ignored_paths(self, files: List[Path]) -> List[Path]:
        """Filter out files that should be ignored."""
        ignored = ['target', 'node_modules', '.git', '__pycache__', '.pytest_cache']
        return [f for f in files if not any(ign in str(f) for ign in ignored)]


def format_result(result: ParseResult, base_path: str = None) -> str:
    """Format a parse result for display."""
    if base_path:
        display_path = str(Path(result.file_path).relative_to(base_path))
    else:
        display_path = result.file_path

    if result.success:
        return f"✓ {display_path} ({result.file_type.value}) - OK"
    else:
        error_info = result.error
        if result.error_line:
            error_info = f"Line {result.error_line}: {error_info}"
        return f"✗ {display_path} ({result.file_type.value}) - FAILED\n  Error: {error_info}"


def print_summary(results: List[ParseResult]) -> None:
    """Print a summary of parsing results."""
    total = len(results)
    success = sum(1 for r in results if r.success)
    failed = total - success

    print("\n" + "=" * 80)
    print(f"Summary: {total} files, {success} successful, {failed} failed")

    if failed > 0:
        print("\nFailed files:")
        for result in results:
            if not result.success:
                print(f"  - {result.file_path}")
                if result.error_line:
                    print(f"    Line {result.error_line}: {result.error}")
                else:
                    print(f"    {result.error}")


def main():
    """Main entry point."""
    import argparse

    parser = argparse.ArgumentParser(description='Parse and validate configuration files')
    parser.add_argument('path', nargs='?', default='.', help='File or directory to parse')
    parser.add_argument('--recursive', '-r', action='store_true', default=True,
                        help='Search recursively (default: True)')
    parser.add_argument('--no-recursive', action='store_false', dest='recursive',
                        help='Do not search recursively')
    parser.add_argument('--validate-all', action='store_true',
                        help='Validate all configuration files in workspace')
    parser.add_argument('--output-format', choices=['text', 'json'], default='text',
                        help='Output format (default: text)')

    args = parser.parse_args()

    config_parser = ConfigParser()

    if args.validate_all:
        workspace = Path('/home/coding/ARMOR')
        files = config_parser.find_config_files(str(workspace), recursive=True)
        files = config_parser.filter_ignored_paths(files)
        base_path = str(workspace)
    elif Path(args.path).is_file():
        files = [Path(args.path)]
        base_path = str(Path(args.path).parent)
    else:
        files = config_parser.find_config_files(args.path, recursive=args.recursive)
        files = config_parser.filter_ignored_paths(files)
        base_path = args.path

    results = []
    for filepath in sorted(files):
        result = config_parser.parse_file(str(filepath))
        results.append(result)

        if args.output_format == 'text':
            print(format_result(result, base_path))

    if args.output_format == 'text':
        print_summary(results)
        return 0 if all(r.success for r in results) else 1
    elif args.output_format == 'json':
        import json
        output = []
        for result in results:
            output.append({
                'file': result.file_path,
                'type': result.file_type.value,
                'success': result.success,
                'error': result.error,
                'error_line': result.error_line,
                'error_column': result.error_column
            })
        print(json.dumps(output, indent=2))
        return 0 if all(r.success for r in results) else 1


if __name__ == '__main__':
    sys.exit(main())
