#!/usr/bin/env python3
"""
Format-Aware Parser Verification

This script verifies parser accuracy while accounting for the inherent
limitations of different pytest output formats (--tb=line vs --tb=short).

Author: ARMOR Project (bf-jk4fln)
Created: 2026-07-13
"""

import json
import os
import sys
from pathlib import Path
from typing import Dict, List, Any
from dataclasses import dataclass

sys.path.insert(0, str(Path(__file__).parent))
from parse_pytest_output import PytestOutputParser, TestFailure


@dataclass
class FormatSpec:
    """Expected fields available in each format."""
    name: str
    # Core fields available in all formats
    core_fields: List[str]
    # Extended fields available only in detailed formats
    extended_fields: List[str]
    # Fields that should never be empty for this format
    required_fields: List[str]


# Define what each format supports
FORMAT_SPECS = {
    'tb_line': FormatSpec(
        name='--tb=line',
        core_fields=['test_file', 'line_number', 'error_type', 'error_message'],
        extended_fields=['expected', 'actual'],  # Sometimes extractable from error message
        required_fields=['test_file', 'line_number', 'error_type']
    ),
    'tb_short': FormatSpec(
        name='--tb=short',
        core_fields=['test_name', 'test_file', 'line_number', 'error_type', 'error_message',
                    'assertion_line', 'assertion_type'],
        extended_fields=['expected', 'actual', 'diff_lines', 'index_diff', 'differing_items'],
        required_fields=['test_name', 'test_file', 'line_number', 'error_type']
    ),
    'tb_long': FormatSpec(
        name='--tb=long',
        core_fields=['test_name', 'test_file', 'line_number', 'error_type', 'error_message',
                    'assertion_line', 'assertion_type'],
        extended_fields=['expected', 'actual', 'diff_lines', 'index_diff', 'differing_items'],
        required_fields=['test_name', 'test_file', 'line_number', 'error_type']
    )
}


class FormatAwareVerifier:
    """Verify parser accuracy accounting for format limitations."""

    def __init__(self, samples_dir: str):
        self.samples_dir = Path(samples_dir)
        self.parser = PytestOutputParser()

    def detect_format(self, filename: str, content: str) -> str:
        """Detect pytest output format from filename or content."""
        # Check filename hints
        if 'line' in filename.lower() or 'tb_line' in filename.lower():
            return 'tb_line'
        if 'long' in filename.lower() or 'v_long' in filename.lower():
            return 'tb_long'
        if 'short' in filename.lower() or 'vv_short' in filename.lower():
            return 'tb_short'
        if 'full_detailed' in filename.lower() or 'format1' in filename.lower():
            return 'tb_short'

        # Analyze content as fallback
        lines = content.split('\n')
        has_underscore_headers = any('____' in line and 'test_' in line for line in lines[:50])
        has_single_line_failures = any(
            'AssertionError:' in line and line.count(':') >= 2 and not '____' in line
            for line in lines[:50]
        )

        if has_single_line_failures and not has_underscore_headers:
            return 'tb_line'
        elif has_underscore_headers:
            return 'tb_short'
        else:
            return 'tb_short'  # Default assumption

    def verify_required_fields(self, failures: List[TestFailure], format_spec: FormatSpec) -> Dict[str, int]:
        """Verify that required fields are extracted."""
        missing_counts = {field: 0 for field in format_spec.required_fields}

        for failure in failures:
            for field in format_spec.required_fields:
                value = getattr(failure, field)
                if value is None or value == "":
                    missing_counts[field] += 1

        return missing_counts

    def verify_extraction_quality(self, failures: List[TestFailure], format_spec: FormatSpec) -> Dict[str, Any]:
        """Verify extraction quality for a format."""
        if not failures:
            return {
                'total_failures': 0,
                'required_field_completeness': 0.0,
                'core_field_coverage': 0.0,
                'extended_field_coverage': 0.0,
                'missing_required': {},
                'issues': ['No failures extracted']
            }

        # Check required fields
        missing_required = self.verify_required_fields(failures, format_spec)
        required_completeness = 100.0  # Will calculate below

        total_required_checks = len(failures) * len(format_spec.required_fields)
        successful_required_checks = 0

        for failure in failures:
            for field in format_spec.required_fields:
                value = getattr(failure, field)
                if value is not None and value != "":
                    successful_required_checks += 1

        required_completeness = (successful_required_checks / total_required_checks * 100) if total_required_checks > 0 else 0

        # Check core field coverage
        total_core_checks = len(failures) * len(format_spec.core_fields)
        successful_core_checks = 0

        for failure in failures:
            for field in format_spec.core_fields:
                value = getattr(failure, field)
                if value is not None and value != "" and value != []:
                    successful_core_checks += 1

        core_coverage = (successful_core_checks / total_core_checks * 100) if total_core_checks > 0 else 0

        # Check extended field coverage (these are optional - some failures might not have them)
        extended_coverage = 0.0
        if format_spec.extended_fields:
            total_extended_checks = len(failures) * len(format_spec.extended_fields)
            successful_extended_checks = 0

            for failure in failures:
                for field in format_spec.extended_fields:
                    value = getattr(failure, field)
                    if value is not None and value != "" and value != []:
                        successful_extended_checks += 1

            extended_coverage = (successful_extended_checks / total_extended_checks * 100) if total_extended_checks > 0 else 0

        # Identify issues
        issues = []
        if required_completeness < 100:
            for field, count in missing_required.items():
                if count > 0:
                    issues.append(f"Missing {field} in {count} failures")
        if core_coverage < 90:
            issues.append(f"Low core field coverage: {core_coverage:.1f}%")

        return {
            'total_failures': len(failures),
            'required_field_completeness': required_completeness,
            'core_field_coverage': core_coverage,
            'extended_field_coverage': extended_coverage,
            'missing_required': missing_required,
            'issues': issues
        }

    def verify_all_samples(self) -> List[Dict[str, Any]]:
        """Verify all samples with format-aware analysis."""
        results = []

        sample_files = sorted([
            f for f in os.listdir(self.samples_dir)
            if f.endswith('.txt') and f.startswith('sample')
        ])

        print(f"Found {len(sample_files)} sample files\n")

        for sample_file in sample_files:
            try:
                content = (self.samples_dir / sample_file).read_text()
                failures = self.parser.parse(content)
                format_type = self.detect_format(sample_file, content)
                format_spec = FORMAT_SPECS[format_type]

                quality = self.verify_extraction_quality(failures, format_spec)

                results.append({
                    'sample': sample_file,
                    'format': format_type,
                    'format_name': format_spec.name,
                    'quality': quality
                })

                status = "✓" if quality['required_field_completeness'] >= 95 else "⚠" if quality['required_field_completeness'] >= 80 else "✗"
                print(f"{status} {sample_file}")
                print(f"   Format: {format_spec.name}")
                print(f"   Failures: {quality['total_failures']}")
                print(f"   Required completeness: {quality['required_field_completeness']:.1f}%")
                print(f"   Core coverage: {quality['core_field_coverage']:.1f}%")
                if format_type != 'tb_line':
                    print(f"   Extended coverage: {quality['extended_field_coverage']:.1f}%")
                if quality['issues']:
                    for issue in quality['issues']:
                        print(f"   Issue: {issue}")
                print()

            except Exception as e:
                print(f"✗ {sample_file}: Error - {e}\n")
                results.append({
                    'sample': sample_file,
                    'format': 'unknown',
                    'format_name': 'Unknown',
                    'quality': {'issues': [str(e)]}
                })

        return results

    def generate_overall_assessment(self, results: List[Dict[str, Any]]) -> Dict[str, Any]:
        """Generate overall assessment of parser quality."""
        total_samples = len(results)
        samples_passing = sum(1 for r in results if r['quality'].get('required_field_completeness', 0) >= 95)

        # Calculate averages per format
        format_stats = {}
        for result in results:
            fmt = result['format']
            if fmt not in format_stats:
                format_stats[fmt] = {
                    'count': 0,
                    'total_failures': 0,
                    'avg_required_completeness': 0.0,
                    'avg_core_coverage': 0.0
                }

            if result['quality'].get('total_failures', 0) > 0:
                format_stats[fmt]['count'] += 1
                format_stats[fmt]['total_failures'] += result['quality']['total_failures']
                format_stats[fmt]['avg_required_completeness'] += result['quality']['required_field_completeness']
                format_stats[fmt]['avg_core_coverage'] += result['quality']['core_field_coverage']

        for fmt in format_stats:
            count = format_stats[fmt]['count']
            if count > 0:
                format_stats[fmt]['avg_required_completeness'] /= count
                format_stats[fmt]['avg_core_coverage'] /= count

        return {
            'total_samples': total_samples,
            'samples_passing': samples_passing,
            'pass_rate': (samples_passing / total_samples * 100) if total_samples > 0 else 0,
            'format_stats': format_stats
        }


def main():
    """Main verification entry point."""
    print("=" * 70)
    print("FORMAT-AWARE PARSER VERIFICATION")
    print("=" * 70)
    print()

    samples_dir = Path(__file__).parent / "test_samples"
    verifier = FormatAwareVerifier(str(samples_dir))

    results = verifier.verify_all_samples()
    assessment = verifier.generate_overall_assessment(results)

    print("=" * 70)
    print("OVERALL ASSESSMENT")
    print("=" * 70)
    print(f"Total samples tested: {assessment['total_samples']}")
    print(f"Samples passing (≥95% required completeness): {assessment['samples_passing']}")
    print(f"Pass rate: {assessment['pass_rate']:.1f}%")
    print()

    print("Results by Format:")
    for fmt, stats in assessment['format_stats'].items():
        format_name = FORMAT_SPECS[fmt].name
        print(f"\n  {format_name}:")
        print(f"    Samples: {stats['count']}")
        print(f"    Failures extracted: {stats['total_failures']}")
        print(f"    Avg required completeness: {stats['avg_required_completeness']:.1f}%")
        print(f"    Avg core coverage: {stats['avg_core_coverage']:.1f}%")

    print()

    # Determine overall result
    if assessment['pass_rate'] >= 95:
        print("✓ VERIFICATION PASSED: Format-aware extraction is robust")
        return 0
    elif assessment['pass_rate'] >= 80:
        print("⚠ VERIFICATION WARNING: Some formats have extraction issues")
        return 1
    else:
        print("✗ VERIFICATION FAILED: Significant extraction problems detected")
        return 2


if __name__ == '__main__':
    sys.exit(main())
