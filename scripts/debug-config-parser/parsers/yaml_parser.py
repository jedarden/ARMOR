"""
YAML configuration file parser.
Handles standard YAML, multi-document YAML, and YAML-like custom formats.
"""

import os
from pathlib import Path
from typing import Dict, List, Any, Optional
from dataclasses import dataclass


@dataclass
class YAMLParseResult:
    """Result of parsing a YAML file."""
    file_path: str
    status: str  # 'success', 'error', 'warning'
    documents: int
    error_message: Optional[str] = None
    warning_message: Optional[str] = None
    data: Any = None


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
                error_message='PyYAML not available'
            )

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
                    return YAMLParseResult(
                        file_path=filepath,
                        status='error',
                        documents=0,
                        error_message=f'YAML syntax error: {str(e2)}'
                    )

        except FileNotFoundError:
            return YAMLParseResult(
                file_path=filepath,
                status='error',
                documents=0,
                error_message=f'File not found: {filepath}'
            )
        except Exception as e:
            return YAMLParseResult(
                file_path=filepath,
                status='error',
                documents=0,
                error_message=f'Unexpected error: {str(e)}'
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

    def validate_syntax(self, filepath: str) -> tuple[bool, Optional[str]]:
        """
        Validate YAML syntax without loading content.

        Args:
            filepath: Path to the YAML file to validate

        Returns:
            Tuple of (is_valid, error_message)
        """
        result = self.parse_file(filepath)
        return (result.status == 'success', result.error_message)

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
