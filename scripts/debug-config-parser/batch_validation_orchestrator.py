#!/usr/bin/env python3
"""
Comprehensive Batch Validation Orchestrator for Debug Configuration Files

This script provides complete batch validation of all debug configuration files
in the ARMOR workspace, with comprehensive reporting and CI/CD integration.

Features:
- Batch validation of all debug configuration files from inventory
- Comprehensive validation reports with file-by-file results
- Summary statistics (total, success, failed, warnings)
- List of files with syntax errors
- Proper exit codes for CI/CD integration (0=success, 1=errors found)
- Support for filtering by file type (YAML, JSON, TOML)
- Parallel processing for efficiency
- JSON output format for integration
- Support for custom workspace path

Usage:
    python3 batch_validation_orchestrator.py
    python3 batch_validation_orchestrator.py --workspace /path/to/workspace
    python3 batch_validation_orchestrator.py --file-type yaml
    python3 batch_validation_orchestrator.py --output json
    python3 batch_validation_orchestrator.py --parallel

Exit Codes:
    0: All files validated successfully
    1: One or more files have validation errors
    2: Runtime error or configuration issue
"""

import sys
import json
import time
import argparse
from pathlib import Path
from typing import Dict, List, Any, Optional
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent))
sys.path.insert(0, str(Path(__file__).parent))

from inventory import DebugFileInventoryReader, DebugFileInventory, FileType
from parsers import ParserFactory


class ValidationResult:
    """Individual file validation result."""

    def __init__(
        self,
        file_path: str,
        relative_path: str,
        file_type: str,
        status: str,
        error: Optional[str] = None,
        warning: Optional[str] = None,
        data: Optional[Any] = None
    ):
        self.file_path = file_path
        self.relative_path = relative_path
        self.file_type = file_type
        self.status = status  # 'success', 'error', 'warning'
        self.error = error
        self.warning = warning
        self.data = data

    def to_dict(self) -> Dict[str, Any]:
        """Convert result to dictionary."""
        return {
            'file_path': self.file_path,
            'relative_path': self.relative_path,
            'file_type': self.file_type,
            'status': self.status,
            'error': self.error,
            'warning': self.warning
        }


class ValidationReport:
    """Comprehensive validation report."""

    def __init__(self, workspace: str, start_time: datetime):
        self.workspace = workspace
        self.start_time = start_time
        self.end_time: Optional[datetime] = None
        self.results: List[ValidationResult] = []
        self.summary = {
            'total_files': 0,
            'successful': 0,
            'errors': 0,
            'warnings': 0,
            'yaml_files': 0,
            'json_files': 0,
            'toml_files': 0
        }
        self.errors_by_type: Dict[str, List[str]] = {}

    def add_result(self, result: ValidationResult) -> None:
        """Add a validation result to the report."""
        self.results.append(result)
        self.summary['total_files'] += 1

        # Update summary statistics
        if result.status == 'success':
            self.summary['successful'] += 1
        elif result.status == 'error':
            self.summary['errors'] += 1
        elif result.status == 'warning':
            self.summary['warnings'] += 1

        # Update file type counts
        if result.file_type == 'yaml':
            self.summary['yaml_files'] += 1
        elif result.file_type == 'json':
            self.summary['json_files'] += 1
        elif result.file_type == 'toml':
            self.summary['toml_files'] += 1

        # Track errors by type
        if result.status == 'error' and result.error:
            error_type = result.error.split(':')[0] if ':' in result.error else 'parse_error'
            if error_type not in self.errors_by_type:
                self.errors_by_type[error_type] = []
            self.errors_by_type[error_type].append(result.relative_path)

    def finalize(self) -> None:
        """Finalize the report with end time."""
        self.end_time = datetime.now()

    def get_duration(self) -> float:
        """Get validation duration in seconds."""
        if self.end_time:
            return (self.end_time - self.start_time).total_seconds()
        return 0.0

    def get_failed_files(self) -> List[ValidationResult]:
        """Get list of failed validations."""
        return [r for r in self.results if r.status == 'error']

    def get_warning_files(self) -> List[ValidationResult]:
        """Get list of files with warnings."""
        return [r for r in self.results if r.status == 'warning']

    def get_successful_files(self) -> List[ValidationResult]:
        """Get list of successful validations."""
        return [r for r in self.results if r.status == 'success']

    def to_dict(self) -> Dict[str, Any]:
        """Convert report to dictionary for JSON serialization."""
        return {
            'workspace': self.workspace,
            'start_time': self.start_time.isoformat(),
            'end_time': self.end_time.isoformat() if self.end_time else None,
            'duration_seconds': self.get_duration(),
            'summary': self.summary,
            'errors_by_type': self.errors_by_type,
            'failed_files': [r.to_dict() for r in self.get_failed_files()],
            'warning_files': [r.to_dict() for r in self.get_warning_files()],
            'successful_files': [r.to_dict() for r in self.get_successful_files()]
        }


class BatchValidationOrchestrator:
    """Orchestrates batch validation of debug configuration files."""

    def __init__(
        self,
        workspace: str = "/home/coding/ARMOR",
        file_type_filter: Optional[str] = None,
        parallel: bool = False
    ):
        """
        Initialize the batch validation orchestrator.

        Args:
            workspace: Path to ARMOR workspace
            file_type_filter: Optional file type filter ('yaml', 'json', 'toml')
            parallel: Whether to use parallel processing
        """
        self.workspace = workspace
        self.file_type_filter = file_type_filter
        self.parallel = parallel
        self.parser_factory = ParserFactory()
        self.inventory_reader = DebugFileInventoryReader(workspace)

    def create_inventory(self) -> DebugFileInventory:
        """
        Create file inventory for validation.

        Returns:
            DebugFileInventory containing files to validate
        """
        inventory = self.inventory_reader.create_inventory()

        # Apply file type filter if specified
        if self.file_type_filter:
            filtered_entries = []
            for entry in inventory.entries:
                if entry.file_type.value == self.file_type_filter.lower():
                    filtered_entries.append(entry)

            # Create filtered inventory
            filtered_inventory = DebugFileInventory(
                workspace=inventory.workspace,
                entries=filtered_entries,
                summary=inventory.summary
            )
            return filtered_inventory

        return inventory

    def validate_file(self, file_path: str, relative_path: str) -> ValidationResult:
        """
        Validate a single configuration file.

        Args:
            file_path: Absolute path to file
            relative_path: Relative path for reporting

        Returns:
            ValidationResult with validation details
        """
        result = self.parser_factory.parse_file(file_path)

        # Determine status
        status = 'success'
        if result['status'] == 'error':
            status = 'error'
        elif result['status'] == 'warning':
            status = 'warning'

        return ValidationResult(
            file_path=file_path,
            relative_path=relative_path,
            file_type=result.get('file_type', 'unknown'),
            status=status,
            error=result.get('error'),
            warning=result.get('warning'),
            data=result.get('data')
        )

    def run_validation(self) -> ValidationReport:
        """
        Run batch validation on all inventory files.

        Returns:
            ValidationReport with comprehensive results
        """
        start_time = datetime.now()
        report = ValidationReport(self.workspace, start_time)

        # Create inventory
        inventory = self.create_inventory()

        if not inventory.entries:
            print(f"No configuration files found in {self.workspace}")
            report.finalize()
            return report

        print(f"Validating {inventory.summary.total_files} configuration files...")
        print()

        # Process files
        if self.parallel and len(inventory.entries) > 10:
            # Use parallel processing for large batches
            results = self._process_parallel(inventory)
        else:
            # Use sequential processing
            results = self._process_sequential(inventory)

        # Add results to report
        for result in results:
            report.add_result(result)

        report.finalize()
        return report

    def _process_sequential(self, inventory: DebugFileInventory) -> List[ValidationResult]:
        """Process files sequentially."""
        results = []

        for i, entry in enumerate(inventory.entries, 1):
            result = self.validate_file(str(entry.path), str(entry.relative_path))
            results.append(result)

            # Print progress
            status_symbol = "✓" if result.status == 'success' else ("⚠" if result.status == 'warning' else "✗")
            print(f"[{i}/{len(inventory.entries)}] {status_symbol} {result.relative_path}")

            if result.status == 'error':
                print(f"    Error: {result.error}")
            elif result.status == 'warning':
                print(f"    Warning: {result.warning}")

        return results

    def _process_parallel(self, inventory: DebugFileInventory) -> List[ValidationResult]:
        """Process files in parallel using thread pool."""
        results = [None] * len(inventory.entries)

        with ThreadPoolExecutor(max_workers=4) as executor:
            # Submit all tasks
            future_to_index = {}
            for i, entry in enumerate(inventory.entries):
                future = executor.submit(
                    self.validate_file,
                    str(entry.path),
                    str(entry.relative_path)
                )
                future_to_index[future] = i

            # Collect results as they complete
            completed = 0
            for future in as_completed(future_to_index):
                index = future_to_index[future]
                try:
                    result = future.result()
                    results[index] = result
                    completed += 1

                    # Print progress
                    status_symbol = "✓" if result.status == 'success' else ("⚠" if result.status == 'warning' else "✗")
                    print(f"[{completed}/{len(inventory.entries)}] {status_symbol} {result.relative_path}")

                    if result.status == 'error':
                        print(f"    Error: {result.error}")
                    elif result.status == 'warning':
                        print(f"    Warning: {result.warning}")

                except Exception as e:
                    print(f"    Exception processing file: {e}")
                    results[index] = ValidationResult(
                        file_path=str(inventory.entries[index].path),
                        relative_path=str(inventory.entries[index].relative_path),
                        file_type='unknown',
                        status='error',
                        error=f"Processing exception: {str(e)}"
                    )

        return results

    def print_text_report(self, report: ValidationReport) -> None:
        """Print validation report in text format."""
        print()
        print("=" * 80)
        print("BATCH VALIDATION REPORT")
        print("=" * 80)
        print()
        print(f"Workspace: {report.workspace}")
        print(f"Duration: {report.get_duration():.2f} seconds")
        print(f"Start Time: {report.start_time.strftime('%Y-%m-%d %H:%M:%S')}")
        print(f"End Time: {report.end_time.strftime('%Y-%m-%d %H:%M:%S') if report.end_time else 'N/A'}")
        print()

        # Summary statistics
        print("Summary Statistics:")
        print(f"  Total Files:     {report.summary['total_files']}")
        print(f"  Successful:     {report.summary['successful']}")
        print(f"  Warnings:        {report.summary['warnings']}")
        print(f"  Errors:          {report.summary['errors']}")
        print()
        print(f"  YAML Files:      {report.summary['yaml_files']}")
        print(f"  JSON Files:      {report.summary['json_files']}")
        print(f"  TOML Files:      {report.summary['toml_files']}")
        print()

        # Files with errors
        failed_files = report.get_failed_files()
        if failed_files:
            print(f"Files with Errors ({len(failed_files)}):")
            print("-" * 80)
            for result in failed_files:
                print(f"  ✗ {result.relative_path}")
                print(f"    Type: {result.file_type.upper()}")
                print(f"    Error: {result.error}")
            print()

        # Files with warnings
        warning_files = report.get_warning_files()
        if warning_files:
            print(f"Files with Warnings ({len(warning_files)}):")
            print("-" * 80)
            for result in warning_files:
                print(f"  ⚠ {result.relative_path}")
                print(f"    Type: {result.file_type.upper()}")
                print(f"    Warning: {result.warning}")
            print()

        # Success indicator
        print("=" * 80)
        if report.summary['errors'] == 0:
            print("✓ ALL VALIDATIONS PASSED")
        else:
            print("✗ VALIDATION FAILED - Errors found")
        print("=" * 80)


def main():
    """Main entry point for batch validation orchestrator."""
    parser = argparse.ArgumentParser(
        description='Batch validate all debug configuration files in ARMOR workspace',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Exit Codes:
    0: All files validated successfully
    1: One or more files have validation errors
    2: Runtime error or configuration issue

Examples:
    # Validate all files in default workspace
    python3 batch_validation_orchestrator.py

    # Validate specific workspace
    python3 batch_validation_orchestrator.py --workspace /path/to/workspace

    # Validate only YAML files
    python3 batch_validation_orchestrator.py --file-type yaml

    # Output JSON report
    python3 batch_validation_orchestrator.py --output json > report.json

    # Use parallel processing for large workspaces
    python3 batch_validation_orchestrator.py --parallel
        """
    )

    parser.add_argument(
        '--workspace',
        default='/home/coding/ARMOR',
        help='Path to ARMOR workspace (default: /home/coding/ARMOR)'
    )
    parser.add_argument(
        '--file-type',
        choices=['yaml', 'json', 'toml'],
        help='Filter by file type (validates all if not specified)'
    )
    parser.add_argument(
        '--output',
        choices=['text', 'json'],
        default='text',
        help='Output format (default: text)'
    )
    parser.add_argument(
        '--parallel',
        action='store_true',
        help='Use parallel processing for faster validation'
    )
    parser.add_argument(
        '--output-file',
        help='Write report to specified file (JSON format only)'
    )

    args = parser.parse_args()

    try:
        # Create orchestrator
        orchestrator = BatchValidationOrchestrator(
            workspace=args.workspace,
            file_type_filter=args.file_type,
            parallel=args.parallel
        )

        # Run validation
        report = orchestrator.run_validation()

        # Output report
        if args.output == 'json':
            report_data = report.to_dict()
            report_json = json.dumps(report_data, indent=2)

            if args.output_file:
                with open(args.output_file, 'w') as f:
                    f.write(report_json)
                print(f"Report written to {args.output_file}")
            else:
                print(report_json)
        else:
            orchestrator.print_text_report(report)

        # Exit with appropriate code
        if report.summary['errors'] > 0:
            sys.exit(1)
        else:
            sys.exit(0)

    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    main()
