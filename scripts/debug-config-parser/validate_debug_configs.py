#!/usr/bin/env python3
"""
Debug Configuration File Validator
Validates all YAML, JSON, and TOML debug configuration files in ARMOR workspace.

This script serves as the main entry point for debug configuration validation.
It discovers all configuration files and validates their syntax using the
parsing infrastructure.
"""

import sys
import os
from pathlib import Path
from typing import Dict, List, Any

# Add the parent directory to the path to import our modules
sys.path.insert(0, str(Path(__file__).parent.parent))

from parsers import ParserFactory


class DebugConfigValidator:
    """Main validator for debug configuration files."""

    def __init__(self, workspace: str = "/home/coding/ARMOR"):
        self.workspace = Path(workspace)
        self.parser_factory = ParserFactory()
        self.config_patterns = ['*.yaml', '*.yml', '*.json', '*.toml']
        self.exclude_dirs = {'.git', '.beads', 'target', 'node_modules', 'logs', '.cache'}

    def discover_config_files(self) -> List[Path]:
        """
        Discover all configuration files in the workspace.

        Returns:
            List of Path objects for discovered config files
        """
        config_files = []

        for pattern in self.config_patterns:
            for filepath in self.workspace.rglob(pattern):
                # Skip excluded directories
                if any(excluded in str(filepath) for excluded in self.exclude_dirs):
                    continue
                config_files.append(filepath)

        return sorted(config_files, key=str)

    def validate_all_configs(self) -> Dict[str, Any]:
        """
        Validate all discovered configuration files.

        Returns:
            Dict with validation summary and detailed results
        """
        config_files = self.discover_config_files()

        print(f"Debug Configuration File Validator")
        print(f"=" * 70)
        print(f"Workspace: {self.workspace}")
        print(f"Files discovered: {len(config_files)}")
        print(f"Pattern matches: {', '.join(self.config_patterns)}")
        print(f"Excluded directories: {', '.join(self.exclude_dirs)}")
        print()

        if not config_files:
            print("No configuration files found to validate.")
            return {
                'total_files': 0,
                'successful': 0,
                'errors': 0,
                'warnings': 0,
                'results': []
            }

        # Parse and validate all files
        results = []
        success_count = 0
        error_count = 0
        warning_count = 0
        failed_files = []

        for filepath in config_files:
            rel_path = filepath.relative_to(self.workspace)
            result = self.parser_factory.parse_file(str(filepath))

            status_symbol = "✓" if result['status'] == 'success' else "✗"
            file_type = result.get('file_type', 'unknown').upper()

            print(f"{status_symbol} {rel_path} ({file_type})")

            if result['status'] == 'success':
                success_count += 1
                # Show document count for YAML files
                if result.get('documents'):
                    doc_note = f" ({result['documents']} document{'s' if result['documents'] != 1 else ''})"
                    print(f"  {doc_note}")
            elif result['status'] == 'warning':
                warning_count += 1
                print(f"  ⚠ Warning: {result.get('warning', 'Unknown warning')}")
            else:
                error_count += 1
                print(f"  ✗ Error: {result.get('error', 'Unknown error')}")
                failed_files.append(str(rel_path))

            results.append({
                'path': str(rel_path),
                'file_type': result.get('file_type', 'unknown'),
                'status': result['status'],
                'error': result.get('error'),
                'warning': result.get('warning'),
                'documents': result.get('documents')
            })

        # Print summary
        print(f"\n" + "=" * 70)
        print(f"VALIDATION SUMMARY")
        print(f"=" * 70)
        print(f"Total files:    {len(config_files)}")
        print(f"Successful:     {success_count}")
        print(f"Warnings:       {warning_count}")
        print(f"Errors:         {error_count}")

        if error_count > 0:
            print(f"\n✗ Files with syntax errors:")
            for failed_file in failed_files:
                print(f"  - {failed_file}")
        elif warning_count > 0:
            print(f"\n⚠ Files with warnings:")
            for result in results:
                if result['status'] == 'warning':
                    print(f"  - {result['path']}: {result['warning']}")
        else:
            print(f"\n✓ All configuration files are valid!")

        return {
            'total_files': len(config_files),
            'successful': success_count,
            'warnings': warning_count,
            'errors': error_count,
            'failed_files': failed_files,
            'results': results
        }

    def validate_specific_files(self, filepaths: List[str]) -> Dict[str, Any]:
        """
        Validate specific configuration files.

        Args:
            filepaths: List of file paths to validate

        Returns:
            Dict with validation summary and detailed results
        """
        print(f"Validating {len(filepaths)} specific files...")
        print(f"=" * 70)

        results = []
        success_count = 0
        error_count = 0

        for filepath in filepaths:
            result = self.parser_factory.parse_file(filepath)
            results.append(result)

            if result['status'] == 'success':
                success_count += 1
                print(f"✓ {filepath}")
            else:
                error_count += 1
                print(f"✗ {filepath}: {result.get('error', 'Unknown error')}")

        return {
            'total_files': len(filepaths),
            'successful': success_count,
            'errors': error_count,
            'results': results
        }


def main():
    """Main entry point for debug configuration validation."""
    import argparse

    parser = argparse.ArgumentParser(
        description='Validate debug configuration files in ARMOR workspace'
    )
    parser.add_argument(
        '--workspace',
        default='/home/coding/ARMOR',
        help='Path to ARMOR workspace (default: /home/coding/ARMOR)'
    )
    parser.add_argument(
        '--files',
        nargs='+',
        help='Specific files to validate instead of scanning workspace'
    )

    args = parser.parse_args()

    validator = DebugConfigValidator(args.workspace)

    if args.files:
        results = validator.validate_specific_files(args.files)
    else:
        results = validator.validate_all_configs()

    # Exit with error code if there were errors
    sys.exit(1 if results['errors'] > 0 else 0)


if __name__ == "__main__":
    main()
