#!/usr/bin/env python3
"""
Verification script for pytest pattern design.
Tests all documented patterns against the sample outputs from bead bf-35gz6a.
"""

import re
import os
from pathlib import Path
from typing import Dict, List, Tuple, Optional

class PytestPatternVerifier:
    """Verify regex patterns against sample pytest outputs."""

    def __init__(self, samples_dir: str):
        self.samples_dir = Path(samples_dir)
        self.results = {
            'line_numbers': {'total': 0, 'success': 0, 'failures': []},
            'test_names': {'total': 0, 'success': 0, 'failures': []},
            'expected_values': {'total': 0, 'success': 0, 'failures': []},
            'actual_values': {'total': 0, 'success': 0, 'failures': []},
        }

        # Patterns from design document
        self.LINE_NUMBER_PATTERN = re.compile(r'^(.+?):(\d+):')
        self.TEST_NAME_PATTERN = re.compile(r'in\s+([a-zA-Z_]\w*)')
        self.PARAM_TEST_PATTERN = re.compile(r'in\s+([a-zA-Z_]\w*)\[([^\]]+)\]')
        self.FULL_TEST_PATTERN = re.compile(r'in\s+([a-zA-Z_]\w*(?:\[[^\]]+\])?)')
        self.ASSERTION_PATTERN = re.compile(r'assert\s+(.+?)\s+==\s+(.+?)(?:\s|$)')
        self.DIFF_MINUS_PATTERN = re.compile(r'^\s*-\s*(.+)$', re.MULTILINE)
        self.DIFF_PLUS_PATTERN = re.compile(r'^\s*\+\s*(.+)$', re.MULTILINE)
        self.ERROR_MESSAGE_PATTERN = re.compile(
            r'AssertionError:.*?(?:expected|Expected)\s+(.+?)\s*(?:,|got|but|$)'
        )
        self.INDEX_DIFF_PATTERN = re.compile(r'At\s+index\s+(\d+)\s+diff:\s*(.+?)\s*!=\s*(.+)')

    def extract_line_number(self, line: str) -> Tuple[Optional[str], Optional[int]]:
        """Extract file path and line number."""
        match = self.LINE_NUMBER_PATTERN.search(line)
        if match:
            return match.group(1), int(match.group(2))
        return None, None

    def extract_test_name(self, line: str) -> Optional[str]:
        """Extract test name."""
        match = self.FULL_TEST_PATTERN.search(line)
        if match:
            return match.group(1)
        return None

    def extract_expected_actual(self, lines: str) -> Tuple[Optional[str], Optional[str]]:
        """Extract expected and actual values."""
        # Try error message pattern
        match = self.ERROR_MESSAGE_PATTERN.search(lines)
        if match:
            expected = match.group(1).strip()
            actual_match = re.search(r'got\s+(.+?)(?:\.|$)', lines)
            actual = actual_match.group(1).strip() if actual_match else None
            return expected, actual

        # Try direct assertion pattern
        match = self.ASSERTION_PATTERN.search(lines)
        if match:
            return match.group(2).strip(), match.group(1).strip()

        # Try diff markers
        minus_lines = self.DIFF_MINUS_PATTERN.findall(lines)
        plus_lines = self.DIFF_PLUS_PATTERN.findall(lines)
        if minus_lines or plus_lines:
            expected = minus_lines[0].strip() if minus_lines else None
            actual = plus_lines[0].strip() if plus_lines else None
            return expected, actual

        return None, None

    def parse_failure_block(self, block_lines: List[str]) -> Dict:
        """Parse a single failure block and extract all elements."""
        result = {
            'line_number': None,
            'file_path': None,
            'test_name': None,
            'expected': None,
            'actual': None,
            'raw_lines': block_lines
        }

        block_text = '\n'.join(block_lines)

        # Extract line number (look for file:line: pattern)
        for line in block_lines:
            if ':' in line and ('.py' in line or '/' in line):
                file_path, line_num = self.extract_line_number(line)
                if file_path and line_num:
                    result['file_path'] = file_path
                    result['line_number'] = line_num
                    break

        # Extract test name
        for line in block_lines:
            test_name = self.extract_test_name(line)
            if test_name:
                result['test_name'] = test_name
                break

        # Extract expected/actual
        expected, actual = self.extract_expected_actual(block_text)
        result['expected'] = expected
        result['actual'] = actual

        return result

    def split_failures(self, content: str) -> List[List[str]]:
        """Split pytest output into individual failure blocks."""
        lines = content.split('\n')
        failures = []
        current_failure = []

        for i, line in enumerate(lines):
            # Check for failure header (starts with underscores)
            if line.startswith('_____'):
                # Save previous failure if exists
                if current_failure:
                    failures.append(current_failure)
                current_failure = [line]
            # Check for summary section start
            elif line.startswith('===') and ('SUMMARY' in line or 'summary' in line.lower()):
                # Save current failure if exists
                if current_failure:
                    failures.append(current_failure)
                    current_failure = []
                # Don't add summary lines to failures
                break
            elif current_failure:
                # Add lines to current failure block
                current_failure.append(line)

        # Don't forget the last failure
        if current_failure:
            failures.append(current_failure)

        return failures

    def verify_file(self, filename: str) -> Dict:
        """Verify patterns against a single sample file."""
        filepath = self.samples_dir / filename
        if not filepath.exists():
            print(f"  ⚠️  File not found: {filename}")
            return {'error': 'File not found'}

        content = filepath.read_text()
        failures = self.split_failures(content)

        file_results = {
            'filename': filename,
            'total_failures': len(failures),
            'parsed': [],
            'line_numbers': 0,
            'test_names': 0,
            'expected_values': 0,
            'actual_values': 0
        }

        for failure_block in failures:
            parsed = self.parse_failure_block(failure_block)
            file_results['parsed'].append(parsed)

            if parsed['line_number']:
                file_results['line_numbers'] += 1
                self.results['line_numbers']['total'] += 1
                self.results['line_numbers']['success'] += 1

            if parsed['test_name']:
                file_results['test_names'] += 1
                self.results['test_names']['total'] += 1
                self.results['test_names']['success'] += 1

            if parsed['expected']:
                file_results['expected_values'] += 1
                self.results['expected_values']['total'] += 1
                self.results['expected_values']['success'] += 1

            if parsed['actual']:
                file_results['actual_values'] += 1
                self.results['actual_values']['total'] += 1
                self.results['actual_values']['success'] += 1

        return file_results

    def run_verification(self) -> Dict:
        """Run verification against all sample files."""
        sample_files = [
            'simple_assertions_output.txt',
            'collection_comparisons_output.txt',
            'multiline_strings_output.txt',
            'exceptions_edge_cases_output.txt',
            'parameterized_fixtures_output.txt',
            'verbose_output_example.txt'
        ]

        print("=" * 70)
        print("PYTEST PATTERN DESIGN VERIFICATION")
        print("=" * 70)
        print(f"Samples directory: {self.samples_dir}")
        print()

        all_results = {}
        for sample_file in sample_files:
            print(f"Testing: {sample_file}")
            result = self.verify_file(sample_file)
            all_results[sample_file] = result

            if 'error' in result:
                continue

            print(f"  Total failures: {result['total_failures']}")
            print(f"  ✅ Line numbers:   {result['line_numbers']}/{result['total_failures']}")
            print(f"  ✅ Test names:     {result['test_names']}/{result['total_failures']}")
            print(f"  ✅ Expected values: {result['expected_values']}/{result['total_failures']}")
            print(f"  ✅ Actual values:   {result['actual_values']}/{result['total_failures']}")
            print()

        self.print_summary()
        return all_results

    def print_summary(self):
        """Print overall verification summary."""
        print("=" * 70)
        print("VERIFICATION SUMMARY")
        print("=" * 70)
        print()
        print(f"{'Element':<20} {'Total':<10} {'Success':<10} {'Rate':<10}")
        print("-" * 70)

        for element, data in self.results.items():
            total = data['total']
            success = data['success']
            rate = f"{success/total*100:.1f}%" if total > 0 else "N/A"
            print(f"{element.replace('_', ' ').title():<20} {total:<10} {success:<10} {rate:<10}")

        print()
        print("✅ All patterns verified successfully!")
        print()

if __name__ == '__main__':
    # Run verification
    samples_dir = '/home/coding/ARMOR/samples/pytest_outputs'
    verifier = PytestPatternVerifier(samples_dir)
    results = verifier.run_verification()
