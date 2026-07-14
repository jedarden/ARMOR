#!/usr/bin/env python3
"""
Comprehensive Parser Verification Script

This script tests the pytest parser against all collected samples to verify
extraction accuracy and consistency across different pytest output formats.

Author: ARMOR Project (bf-jk4fln)
Created: 2026-07-13
Purpose: Verify parser consistency across all collected samples
"""

import json
import os
import sys
from pathlib import Path
from typing import Dict, List, Any, Tuple
from dataclasses import dataclass

# Add parent directory to path to import parser
sys.path.insert(0, str(Path(__file__).parent))
from parse_pytest_output import PytestOutputParser, TestFailure


@dataclass
class VerificationResult:
    """Results of parsing a single sample file."""
    sample_name: str
    total_failures: int
    successful_extractions: Dict[str, int]
    failed_extractions: List[str]
    extraction_rate: float
    issues: List[str]


class ParserVerifier:
    """Verify parser consistency across multiple sample files."""

    def __init__(self, samples_dir: str):
        self.samples_dir = Path(samples_dir)
        self.parser = PytestOutputParser()

    def load_sample(self, filename: str) -> str:
        """Load a sample file."""
        path = self.samples_dir / filename
        with open(path, 'r') as f:
            return f.read()

    def count_expected_failures(self, content: str) -> int:
        """Count expected number of failures in sample content."""
        lines = content.split('\n')
        count = 0

        # Count for --tb=line format (file:line: AssertionError: messages)
        for line in lines:
            if 'AssertionError:' in line and ':' in line.split('AssertionError:')[0]:
                count += 1
            # Count for underscore headers (short/long formats)
            if line.strip().startswith('____') and 'test_' in line:
                count += 1

        return max(count, 1)  # At least 1 if we found content

    def verify_extraction(self, failures: List[TestFailure], sample_name: str) -> VerificationResult:
        """Verify extraction quality for a sample."""
        result = VerificationResult(
            sample_name=sample_name,
            total_failures=len(failures),
            successful_extractions={
                'test_name': 0,
                'test_file': 0,
                'line_number': 0,
                'error_type': 0,
                'error_message': 0,
                'assertion_line': 0,
                'assertion_type': 0,
                'expected': 0,
                'actual': 0,
                'diff_lines': 0,
                'index_diff': 0,
                'differing_items': 0
            },
            failed_extractions=[],
            extraction_rate=0.0,
            issues=[]
        )

        if not failures:
            result.issues.append("No failures extracted from sample")
            return result

        total_fields = len(failures) * len(result.successful_extractions)
        successful_fields = 0

        for failure in failures:
            # Check each field
            checks = [
                ('test_name', failure.test_name),
                ('test_file', failure.test_file),
                ('line_number', failure.line_number),
                ('error_type', failure.error_type),
                ('error_message', failure.error_message),
                ('assertion_line', failure.assertion_line),
                ('assertion_type', failure.assertion_type),
                ('expected', failure.expected),
                ('actual', failure.actual),
                ('diff_lines', failure.diff_lines),
                ('index_diff', failure.index_diff),
                ('differing_items', failure.differing_items)
            ]

            for field_name, value in checks:
                if value is not None and value != "" and value != []:
                    result.successful_extractions[field_name] += 1
                    successful_fields += 1

        result.extraction_rate = (successful_fields / total_fields * 100) if total_fields > 0 else 0

        # Identify missing extractions
        for field_name, count in result.successful_extractions.items():
            if count < len(failures):
                missing = len(failures) - count
                result.failed_extractions.append(f"{field_name}: {missing} missing")

        return result

    def verify_all_samples(self) -> List[VerificationResult]:
        """Verify parser against all sample files."""
        results = []

        # Get all sample files
        sample_files = sorted([
            f for f in os.listdir(self.samples_dir)
            if f.endswith('.txt') and f.startswith('sample')
        ])

        print(f"Found {len(sample_files)} sample files to verify\n")

        for sample_file in sample_files:
            print(f"Testing {sample_file}...")

            try:
                content = self.load_sample(sample_file)
                failures = self.parser.parse(content)

                result = self.verify_extraction(failures, sample_file)
                results.append(result)

                print(f"  ✓ Extracted {result.total_failures} failures")
                print(f"  ✓ Extraction rate: {result.extraction_rate:.1f}%")

                if result.issues:
                    print(f"  ⚠ Issues: {', '.join(result.issues)}")

                print()
            except Exception as e:
                print(f"  ✗ Error: {e}\n")
                results.append(VerificationResult(
                    sample_name=sample_file,
                    total_failures=0,
                    successful_extractions={},
                    failed_extractions=[],
                    extraction_rate=0.0,
                    issues=[f"Parse error: {e}"]
                ))

        return results

    def generate_summary(self, results: List[VerificationResult]) -> Dict[str, Any]:
        """Generate overall verification summary."""
        total_samples = len(results)
        samples_with_failures = sum(1 for r in results if r.total_failures > 0)
        avg_extraction_rate = sum(r.extraction_rate for r in results) / total_samples if total_samples > 0 else 0

        # Count field extraction success rates
        field_success = {}
        for result in results:
            for field, count in result.successful_extractions.items():
                if field not in field_success:
                    field_success[field] = {'success': 0, 'total': 0}
                field_success[field]['success'] += count
                field_success[field]['total'] += result.total_failures

        field_rates = {}
        for field, data in field_success.items():
            rate = (data['success'] / data['total'] * 100) if data['total'] > 0 else 0
            field_rates[field] = rate

        return {
            'total_samples': total_samples,
            'samples_with_failures': samples_with_failures,
            'avg_extraction_rate': avg_extraction_rate,
            'field_extraction_rates': field_rates,
            'total_failures_extracted': sum(r.total_failures for r in results)
        }


def main():
    """Main verification entry point."""
    print("=" * 70)
    print("PYTEST PARSER CONSISTENCY VERIFICATION")
    print("=" * 70)
    print()

    # Initialize verifier
    samples_dir = Path(__file__).parent / "test_samples"
    verifier = ParserVerifier(str(samples_dir))

    # Run verification
    results = verifier.verify_all_samples()

    # Generate and print summary
    summary = verifier.generate_summary(results)

    print("=" * 70)
    print("VERIFICATION SUMMARY")
    print("=" * 70)
    print(f"Total samples tested: {summary['total_samples']}")
    print(f"Samples with failures: {summary['samples_with_failures']}")
    print(f"Total failures extracted: {summary['total_failures_extracted']}")
    print(f"Average extraction rate: {summary['avg_extraction_rate']:.1f}%")
    print()

    print("Field Extraction Rates:")
    for field, rate in sorted(summary['field_extraction_rates'].items()):
        status = "✓" if rate >= 90 else "⚠" if rate >= 70 else "✗"
        print(f"  {status} {field:20s}: {rate:6.1f}%")

    print()

    # Determine if verification passed
    if summary['avg_extraction_rate'] >= 95:
        print("✓ VERIFICATION PASSED: Extraction rate >= 95%")
        return 0
    elif summary['avg_extraction_rate'] >= 90:
        print("⚠ VERIFICATION WARNING: Extraction rate 90-95%")
        return 1
    else:
        print("✗ VERIFICATION FAILED: Extraction rate < 90%")
        return 2


if __name__ == '__main__':
    sys.exit(main())
