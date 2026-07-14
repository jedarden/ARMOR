#!/usr/bin/env python3
"""
Pytest Output Parser - Machine-Parseable Test Output Extractor

This script demonstrates that pytest -vv --tb=short output is machine-parseable
by extracting structured data from test failures.

Author: ARMOR Project (bf-1rd15q)
Created: 2026-07-13
"""

import re
import sys
from dataclasses import dataclass, field
from typing import List, Optional, Dict, Any
from collections import defaultdict


@dataclass
class TestFailure:
    """Structured representation of a single test failure."""
    test_name: str
    test_file: str
    line_number: int
    error_type: str
    error_message: str
    assertion_line: Optional[str] = None
    expected: Optional[str] = None
    actual: Optional[str] = None
    diff_lines: List[str] = field(default_factory=list)
    index_diff: Optional[int] = None
    differing_items: List[Dict[str, str]] = field(default_factory=list)

    def __repr__(self):
        return f"TestFailure(test={self.test_name}, line={self.line_number}, type={self.error_type})"


class PytestOutputParser:
    """Parser for pytest -vv --tb=short output."""

    # Patterns for parsing different sections of pytest output
    TEST_NAME_PATTERN = r'^(.*?)_(_\d+)?\s*$'
    FAILURE_HEADER_PATTERN = r'^([A-Za-z0-9_./]+::[a-zA-Z_][a-zA-Z0-9_]*)\s+FAILED'
    FILE_LINE_PATTERN = r'^([A-Za-z0-9_./]+):(\d+):\s+in\s+([a-zA-Z_][a-zA-Z0-9_]*)'
    ASSERTION_PATTERN = r'^E?\s+assert\s+(.+)$'
    ASSERTION_ERROR_PATTERN = r'^E?\s+AssertionError(?::\s*(.+))?$'
    INDEX_DIFF_PATTERN = r'At index (\d+) diff:'
    DIFF_ITEM_PATTERN = r'^\{\'([^\']+)\'\:\s*(.+)\}\s*!=\s*\{\'([^\']+)\'\:\s*(.+)\}'
    LINE_MINUS_PATTERN = r'^[E?\s]*-(.+)$'
    LINE_PLUS_PATTERN = r'^[E?\s]*\+(.+)$'

    def __init__(self):
        self.failures: List[TestFailure] = []
        self.current_failure: Optional[TestFailure] = None
        self.in_diff_section = False
        self.in_differing_items = False

    def parse(self, output: str) -> List[TestFailure]:
        """
        Parse pytest output and return structured failure data.

        Args:
            output: Raw pytest output string

        Returns:
            List of TestFailure objects
        """
        self.failures = []
        self.current_failure = None
        self.in_diff_section = False
        self.in_differing_items = False

        lines = output.split('\n')
        i = 0

        while i < len(lines):
            line = lines[i]

            # Detect start of a new failure
            if self._is_failure_header(line):
                if self.current_failure:
                    self.failures.append(self.current_failure)
                self.current_failure = self._parse_failure_header(line)
                self.in_diff_section = False
                self.in_differing_items = False

            elif self.current_failure:
                # Parse file and line information
                match = re.match(self.FILE_LINE_PATTERN, line)
                if match:
                    self.current_failure.test_file = match.group(1)
                    self.current_failure.line_number = int(match.group(2))
                    self.current_failure.test_name = match.group(3)

                # Parse assertion line
                elif re.match(r'^E?\s+assert\s+', line):
                    self.current_failure.assertion_line = line.strip()

                # Parse AssertionError message
                elif line.startswith('E   AssertionError:'):
                    error_msg = line.replace('E   AssertionError:', '').strip()
                    if error_msg:
                        self.current_failure.error_message = error_msg

                # Parse index diff (e.g., "At index 2 diff: 0 != 3")
                elif 'At index' in line and 'diff:' in line:
                    match = re.search(self.INDEX_DIFF_PATTERN, line)
                    if match:
                        self.current_failure.index_diff = int(match.group(1))

                # Detect differing items section
                elif 'Differing items:' in line:
                    self.in_differing_items = True
                    self.in_diff_section = False

                # Detect full diff section
                elif 'Full diff:' in line:
                    self.in_diff_section = True
                    self.in_differing_items = False

                # Parse differing items
                elif self.in_differing_items and '{' in line and '!=' in line:
                    match = re.search(self.DIFF_ITEM_PATTERN, line)
                    if match:
                        self.current_failure.differing_items.append({
                            'key': match.group(1),
                            'actual': match.group(2),
                            'expected': match.group(3),
                            'expected_value': match.group(4)
                        })

                # Parse diff lines (-/+)
                elif self.in_diff_section:
                    stripped = line.strip()
                    if stripped.startswith('-') and not stripped.startswith('---'):
                        # Expected line
                        content = stripped[1:].strip()
                        if content and not any(c in content for c in '?^+-'):
                            if not self.current_failure.expected:
                                self.current_failure.expected = content
                            self.current_failure.diff_lines.append(f"- {content}")
                    elif stripped.startswith('+'):
                        # Actual line
                        content = stripped[1:].strip()
                        if content and not any(c in content for c in '?^+-'):
                            if not self.current_failure.actual:
                                self.current_failure.actual = content
                            self.current_failure.diff_lines.append(f"+ {content}")

            i += 1

        # Don't forget the last failure
        if self.current_failure:
            self.failures.append(self.current_failure)

        return self.failures

    def _is_failure_header(self, line: str) -> bool:
        """Check if line is a failure header."""
        return bool(re.match(r'^_{20,}', line))

    def _parse_failure_header(self, line: str) -> TestFailure:
        """Parse a failure header line and create a TestFailure object."""
        # Extract test name from header (next line has the full test path)
        return TestFailure(
            test_name="",
            test_file="",
            line_number=0,
            error_type="AssertionError",
            error_message=""
        )

    def parse_test_summary(self, output: str) -> Dict[str, Any]:
        """
        Parse the test summary section (FAILED lines at the end).

        Returns:
            Dict with summary information
        """
        summary = {
            'total_failed': 0,
            'total_duration': 0.0,
            'failures': []
        }

        lines = output.split('\n')
        in_summary = False

        for line in lines:
            # Find summary section
            if '=== short test summary info ===' in line:
                in_summary = True
                continue

            if in_summary:
                # Parse FAILED lines
                if line.startswith('FAILED'):
                    parts = line.split()
                    if len(parts) >= 3:
                        test_path = parts[1]
                        summary['failures'].append({
                            'test_path': test_path,
                            'error_type': parts[3] if len(parts) > 3 else 'AssertionError'
                        })

                # Parse final line
                if 'failed in' in line:
                    match = re.search(r'(\d+)\s+failed\s+in\s+([\d.]+)s', line)
                    if match:
                        summary['total_failed'] = int(match.group(1))
                        summary['total_duration'] = float(match.group(2))

        return summary


def extract_json_compatible_data(failures: List[TestFailure]) -> List[Dict[str, Any]]:
    """Convert TestFailure objects to JSON-serializable dictionaries."""
    return [
        {
            'test_name': f.test_name,
            'test_file': f.test_file,
            'line_number': f.line_number,
            'error_type': f.error_type,
            'error_message': f.error_message,
            'assertion_line': f.assertion_line,
            'expected': f.expected,
            'actual': f.actual,
            'index_diff': f.index_diff,
            'differing_items': f.differing_items,
            'diff_lines': f.diff_lines
        }
        for f in failures
    ]


def main():
    """Main entry point for command-line usage."""
    if len(sys.argv) > 1:
        # Read from file
        with open(sys.argv[1], 'r') as f:
            output = f.read()
    else:
        # Read from stdin
        output = sys.stdin.read()

    parser = PytestOutputParser()
    failures = parser.parse(output)
    summary = parser.parse_test_summary(output)

    print(f"=== Parsed {len(failures)} Failures ===\n")

    for i, failure in enumerate(failures, 1):
        print(f"Failure #{i}:")
        print(f"  Test: {failure.test_file}::{failure.test_name}")
        print(f"  Line: {failure.line_number}")
        print(f"  Type: {failure.error_type}")
        print(f"  Message: {failure.error_message}")
        if failure.expected:
            print(f"  Expected: {failure.expected}")
        if failure.actual:
            print(f"  Actual: {failure.actual}")
        if failure.index_diff is not None:
            print(f"  Index Diff: {failure.index_diff}")
        if failure.differing_items:
            print(f"  Differing Items: {len(failure.differing_items)}")
            for item in failure.differing_items:
                print(f"    - {item['key']}: {item['actual']} != {item['expected_value']}")
        print()

    print(f"=== Summary ===")
    print(f"Total Failed: {summary['total_failed']}")
    print(f"Duration: {summary['total_duration']}s")
    print(f"Failures Listed: {len(summary['failures'])}")


if __name__ == '__main__':
    main()
