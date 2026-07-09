#!/usr/bin/env python3
"""
Batch Validation Integration Example
Demonstrates using the inventory reader for batch configuration validation.
"""

import sys
from pathlib import Path

# Add parent directory to path
sys.path.insert(0, str(Path(__file__).parent.parent))
sys.path.insert(0, str(Path(__file__).parent))

from inventory import DebugFileInventoryReader
from parsers import ParserFactory


def batch_validate_workspace(workspace: str = "/home/coding/ARMOR"):
    """
    Perform batch validation of all configuration files in workspace.

    This demonstrates the integration between the inventory reader and
    the parser factory for efficient batch validation.
    """
    print("Batch Configuration File Validation")
    print("=" * 70)
    print()

    # Step 1: Create inventory of all config files
    print("Step 1: Discovering configuration files...")
    inventory_reader = DebugFileInventoryReader(workspace)
    inventory = inventory_reader.create_inventory()

    print(f"  Found {inventory.summary.total_files} configuration files")
    print(f"  YAML: {inventory.summary.yaml_files}")
    print(f"  JSON: {inventory.summary.json_files}")
    print(f"  TOML: {inventory.summary.toml_files}")
    print()

    # Step 2: Batch validate all files using parser factory
    print("Step 2: Validating file syntax...")
    parser_factory = ParserFactory()

    results = {
        'total': len(inventory.entries),
        'success': 0,
        'error': 0,
        'warning': 0,
        'errors': []
    }

    for entry in inventory.entries:
        result = parser_factory.parse_file(str(entry.path))

        status_symbol = "✓" if result['status'] == 'success' else "✗"
        rel_path = entry.relative_path

        if result['status'] == 'success':
            results['success'] += 1
            print(f"  {status_symbol} {rel_path} ({result['file_type'].upper()})")
        elif result['status'] == 'warning':
            results['warning'] += 1
            print(f"  ⚠ {rel_path}: {result.get('warning', 'Unknown warning')}")
        else:
            results['error'] += 1
            error_msg = result.get('error', 'Unknown error')
            print(f"  ✗ {rel_path}: {error_msg}")
            results['errors'].append({
                'path': str(rel_path),
                'error': error_msg
            })

    print()
    print("=" * 70)
    print("VALIDATION SUMMARY")
    print("=" * 70)
    print(f"Total files:   {results['total']}")
    print(f"Successful:    {results['success']}")
    print(f"Warnings:      {results['warning']}")
    print(f"Errors:        {results['error']}")

    if results['error'] > 0:
        print()
        print("Files with errors:")
        for error in results['errors']:
            print(f"  ✗ {error['path']}")
            print(f"    {error['error']}")

    return results


def main():
    """Main entry point."""
    import argparse

    parser = argparse.ArgumentParser(
        description='Batch validate all configuration files in ARMOR workspace'
    )
    parser.add_argument(
        '--workspace',
        default='/home/coding/ARMOR',
        help='Path to ARMOR workspace (default: /home/coding/ARMOR)'
    )

    args = parser.parse_args()

    results = batch_validate_workspace(args.workspace)

    # Exit with error code if there were errors
    sys.exit(1 if results['error'] > 0 else 0)


if __name__ == "__main__":
    main()
