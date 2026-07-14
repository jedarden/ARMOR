#!/usr/bin/env python3
"""
Pytest Output Parser - Machine-Parseable Test Output Extractor

This script parses pytest failure output according to documented patterns,
extracting file paths, line numbers, expected values, and actual values.

Author: ARMOR Project (bf-29wbke)
Created: 2026-07-13
Based on: pytest_patterns.md research (bf-5x5xz1)
"""

import json
import re
import sys
import argparse
from dataclasses import dataclass, field
from typing import List, Optional, Dict, Any
from collections import defaultdict


@dataclass
class TestFailure:
    """Structured representation of a single test failure."""
    test_name: str = ""
    test_file: str = ""
    line_number: Optional[int] = None
    error_type: str = "AssertionError"
    error_message: str = ""
    assertion_line: Optional[str] = None
    assertion_type: Optional[str] = None
    expected: Optional[str] = None
    actual: Optional[str] = None
    diff_lines: List[str] = field(default_factory=list)
    index_diff: Optional[int] = None
    differing_items: List[Dict[str, str]] = field(default_factory=list)
    where_clause: Optional[str] = None

    def to_dict(self) -> Dict[str, Any]:
        """Convert to JSON-serializable dictionary."""
        return {
            'test_name': self.test_name,
            'test_file': self.test_file,
            'line_number': self.line_number,
            'error_type': self.error_type,
            'error_message': self.error_message,
            'assertion_line': self.assertion_line,
            'assertion_type': self.assertion_type,
            'expected': self.expected,
            'actual': self.actual,
            'diff_lines': self.diff_lines,
            'index_diff': self.index_diff,
            'differing_items': self.differing_items,
            'where_clause': self.where_clause
        }


class PytestOutputParser:
    """Parser for pytest output using documented patterns from pytest_parser.md."""

    # Core patterns from pytest_parser.md
    FILE_LOCATION_PATTERN = r'^\s*(.+?):(\d+):'
    SHORT_FAILURE_PATTERN = r'^\s*(.+?):(\d+):\s+in\s+(\w+)'
    LINE_FAILURE_PATTERN = r'^\s*(.+?):(\d+):\s+(AssertionError|KeyError|AttributeError):\s*(.+)'
    LONG_FAILURE_END_PATTERN = r'^\s*(.+?):(\d+):\s+(AssertionError|KeyError|AttributeError)\s*$'
    VERBOSE_TEST_PATTERN = r'^\s*(.+?)::(\w+)\s+(FAILED|PASSED|ERROR|SKIPPED)\s+\[\s*\d+%?\]'
    CONCISE_TEST_PATTERN = r'^\s*(.+?)\s+([FPE\.]+)\s+\[\s*\d+%?\]'

    # Assertion patterns
    ASSERT_PATTERN = r'^E?\s+assert\s+(.+)'
    EQUALITY_PATTERN = r'^E?\s+assert\s+(.+?)\s+==\s+(.+)'
    CONTAINS_PATTERN = r'^E?\s+assert\s+(.+?)\s+in\s+(.+)'
    TYPE_CHECK_PATTERN = r'^E?\s+assert\s+isinstance\((.+?),\s*(.+?)\)'

    # Diff patterns
    DIFF_MINUS_PATTERN = r'^\s*-\s*(.+)'
    DIFF_PLUS_PATTERN = r'^\s*\+\s*(.+)'
    DIFF_POSITION_PATTERN = r'^\s*\?\s+(.+)'
    INDEX_DIFF_PATTERN = r'^\s*(?:E\s+)?At\s+index\s+(\d+)\s+diff:\s+(.+?)\s+!=\s+(.+)'
    DICT_DIFF_HEADER = r'^E?\s+Differing items:'
    DICT_DIFF_LINE = r'^E?\s+\{(.+?)\}\s+!=\s+\{(.+?)\}'
    SET_DIFF_LEFT = r'^\s+Extra items in the left set:'
    SET_DIFF_RIGHT = r'^\s+Extra items in the right set:'
    RANGE_DIFF_PATTERN = r'^\s*(?:E\s+)?(Right|Left) contains (?:one more item|\d+ more items?)\s*:\s*(.+)'
    WHERE_CLAUSE_PATTERN = r'^\s*\+\s+where\s+(.+?)\s+=\s+(.+)'

    # Section markers
    FAILURES_SECTION = r'^===+ FAILURES ===+'
    SUMMARY_SECTION = r'^===+ short test summary info ===+'
    SESSION_START = r'^===+ test session starts ===+'
    SESSION_END = r'^===+ \d+ (?:failed|passed) in [\d.]+s ===+'

    # Edge case patterns for different error types
    KEYERROR_PATTERN = r'^E?\s+KeyError:\s+(.+)'
    ATTRIBUTEERROR_PATTERN = r'^E?\s+AttributeError:\s+(.+)'
    NONE_ASSERT_PATTERN = r'^E?\s+assert\s+None\s*(?:$|\s)'

    def __init__(self):
        self.failures: List[TestFailure] = []

    def parse(self, output: str) -> List[TestFailure]:
        """
        Parse pytest output and return structured failure data.

        Handles multiple pytest output formats:
        - --tb=line: One line per failure with file:line: AssertionError: message
        - --tb=short: Multi-line failures with detailed diffs
        - --tb=no: Only summary section

        Args:
            output: Raw pytest output string

        Returns:
            List of TestFailure objects
        """
        self.failures = []

        # First, check if this is --tb=line format (no underscore headers)
        if self._parse_line_format(output):
            return self.failures

        # Otherwise, parse with --tb=short format (has underscore headers)
        self._parse_short_format(output)

        return self.failures

    def _parse_line_format(self, output: str) -> bool:
        """
        Parse --tb=line format.

        Format: file:line: AssertionError: message
        Each failure is on a single line.
        """
        lines = output.split('\n')
        failures_found = False

        for line in lines:
            match = re.match(self.LINE_FAILURE_PATTERN, line)
            if match:
                failures_found = True
                file_path, line_num, error_type, error_message = match.groups()

                # Extract test name from the path if available
                test_name = self._extract_test_name_from_path(file_path)

                failure = TestFailure(
                    test_file=file_path,
                    line_number=int(line_num),
                    test_name=test_name,
                    error_message=error_message.strip(),
                    error_type=error_type
                )

                # Try to extract expected/actual from error message
                self._extract_error_message_details(failure)

                self.failures.append(failure)

        return failures_found

    def _parse_short_format(self, output: str) -> None:
        """
        Parse --tb=short format with underscore headers.

        Format:
        _________________________ test_name ___________________________
        file:line: in test_name
            assert line
        E   AssertionError: message
        E   assert expr1 == expr2
        E
        E       - expected
        E       + actual
        """
        lines = output.split('\n')
        current_failure: Optional[TestFailure] = None
        in_diff_section = False
        in_differing_items = False

        for i, line in enumerate(lines):
            # Check for failure header (line with many underscores and test name)
            if re.match(r'^_{20,}\s+\w+', line):
                # Save previous failure if exists
                if current_failure and current_failure.test_name:
                    self.failures.append(current_failure)

                # Extract test name from header
                test_name = self._extract_test_name_from_header(line)
                current_failure = TestFailure(test_name=test_name)
                in_diff_section = False
                in_differing_items = False
                continue

            if not current_failure:
                continue

            # Parse file:line: in test_name (Format 1)
            match = re.match(self.SHORT_FAILURE_PATTERN, line)
            if match:
                current_failure.test_file = match.group(1)
                current_failure.line_number = int(match.group(2))
                current_failure.test_name = match.group(3) or current_failure.test_name
                continue

            # Parse file:line: ErrorType at end of block (Format 2)
            # This handles AssertionError, KeyError, AttributeError, etc.
            match = re.match(self.LONG_FAILURE_END_PATTERN, line)
            if match:
                current_failure.test_file = match.group(1)
                current_failure.line_number = int(match.group(2))
                current_failure.error_type = match.group(3)  # Get the actual error type
                # This is the end of the failure block - save it
                if current_failure.test_name:
                    self.failures.append(current_failure)
                current_failure = None
                in_diff_section = False
                in_differing_items = False
                continue

            # Parse KeyError edge case (different from AssertionError)
            match = re.match(self.KEYERROR_PATTERN, line)
            if match and current_failure:
                current_failure.error_type = "KeyError"
                current_failure.error_message = f"KeyError: {match.group(1).strip()}"
                continue

            # Parse AttributeError edge case (different from AssertionError)
            match = re.match(self.ATTRIBUTEERROR_PATTERN, line)
            if match and current_failure:
                current_failure.error_type = "AttributeError"
                current_failure.error_message = f"AttributeError: {match.group(1).strip()}"
                continue

            # Parse None assertion edge case
            match = re.match(self.NONE_ASSERT_PATTERN, line)
            if match and current_failure:
                current_failure.actual = "None"
                continue

            # Parse assertion line
            if re.match(r'^E?\s+assert\s+', line):
                current_failure.assertion_line = line.strip()
                # Classify assertion type
                current_failure.assertion_type = self._classify_assertion(line)
                # Try to extract expected/actual from assertion
                self._extract_assertion_details(current_failure, line)
                continue

            # Parse AssertionError message (handle variable whitespace after E)
            match = re.match(r'^E\s+AssertionError:\s*(.+)', line)
            if match:
                error_msg = match.group(1).strip()
                if error_msg:
                    current_failure.error_message = error_msg
                continue

            # Parse KeyError edge case in Format 1/2
            match = re.match(r'^E\s+KeyError:\s*(.+)', line)
            if match:
                current_failure.error_type = "KeyError"
                error_msg = match.group(1).strip()
                if error_msg:
                    current_failure.error_message = f"KeyError: {error_msg}"
                continue

            # Parse AttributeError edge case in Format 1/2
            match = re.match(r'^E\s+AttributeError:\s*(.+)', line)
            if match:
                current_failure.error_type = "AttributeError"
                error_msg = match.group(1).strip()
                if error_msg:
                    current_failure.error_message = f"AttributeError: {error_msg}"
                continue

            # Detect index diff (overwrite with specific element values)
            match = re.match(self.INDEX_DIFF_PATTERN, line)
            if match:
                current_failure.index_diff = int(match.group(1))
                # Store the specific differing elements (group 2 is actual, group 3 is expected)
                current_failure.actual = match.group(2).strip()
                current_failure.expected = match.group(3).strip()
                continue

            # Detect differing items section
            if re.match(self.DICT_DIFF_HEADER, line):
                in_differing_items = True
                in_diff_section = False
                continue

            # Parse dict diff items
            if in_differing_items:
                match = re.match(self.DICT_DIFF_LINE, line)
                if match:
                    # Parse key-value pairs from the diff
                    try:
                        left_dict = self._parse_dict_value(match.group(1))
                        right_dict = self._parse_dict_value(match.group(2))
                        for key, left_val in left_dict.items():
                            right_val = right_dict.get(key)
                            if right_val and left_val != right_val:
                                current_failure.differing_items.append({
                                    'key': key,
                                    'expected': right_val,
                                    'actual': left_val
                                })
                    except:
                        pass
                    continue  # Skip dict diff lines
                # Exit differing items section when we see Full diff, blank line, or diff markers
                if 'Full diff:' in line or not line.strip() or line.strip() == 'E':
                    in_differing_items = False
                # Exit differing items section when we see diff markers (- or +)
                line_content = re.sub(r'^E\s+', '', line) if line.startswith('E') else line
                if line_content.strip().startswith(('-', '+', '?')):
                    in_differing_items = False

            # Detect full diff section
            if 'Full diff:' in line or (in_diff_section and line.strip() and not line.startswith('E')):
                in_diff_section = True
                in_differing_items = False

            # Parse diff lines (both inside and outside of "Full diff:" sections)
            # Strip 'E' prefix if present (Format 1 uses 'E     -', Format 2 uses '    -')
            line_content = re.sub(r'^E\s+', '', line) if line.startswith('E') else line

            # Edge Case: Parse where clause BEFORE regular diff lines
            # A where clause looks like: "+  where False = isinstance(...)"
            # and must be distinguished from regular diff "+ value" lines
            # This check must come after stripping 'E' prefix and before diff line checks
            match = re.match(self.WHERE_CLAUSE_PATTERN, line_content)
            if match:
                current_failure.where_clause = f"{match.group(1).strip()} = {match.group(2).strip()}"
                continue

            # Check if this is a diff line (but not a where clause)
            # Edge Case: Handle different diff scenarios with appropriate precedence:
            # 1. Index diffs and dict diffs: already set accurate values, preserve them
            # 2. Multiline string diffs: use diff lines (variable names in assertion aren't useful)
            # 3. Simple equality: diff lines provide more specific values than assertion
            if line_content.strip().startswith('-'):
                content = re.sub(r'^\s*-\s*', '', line_content).strip()
                if content and not content.startswith('?'):
                    # Only overwrite if we don't have more specific values from index/dict diffs
                    if not current_failure.index_diff and not current_failure.differing_items:
                        current_failure.expected = content
                    current_failure.diff_lines.append(f"- {content}")
                continue
            elif line_content.strip().startswith('+'):
                content = re.sub(r'^\s*\+\s*', '', line_content).strip()
                if content and not content.startswith('?'):
                    # Only overwrite if we don't have more specific values from index/dict diffs
                    if not current_failure.index_diff and not current_failure.differing_items:
                        current_failure.actual = content
                    current_failure.diff_lines.append(f"+ {content}")
                continue

            # Skip position indicator lines
            if line.strip().startswith('?'):
                continue

        # Don't forget the last failure
        if current_failure and current_failure.test_name:
            self.failures.append(current_failure)

    def parse_test_summary(self, output: str) -> Dict[str, Any]:
        """
        Parse the test summary section (FAILED lines at the end).

        Returns:
            Dict with summary information
        """
        summary = {
            'total_failed': 0,
            'total_passed': 0,
            'total_duration': 0.0,
            'failures': []
        }

        lines = output.split('\n')
        in_summary = False

        for line in lines:
            # Find summary section
            if re.search(self.SUMMARY_SECTION, line):
                in_summary = True
                continue

            if in_summary:
                # Parse FAILED lines
                if line.startswith('FAILED'):
                    parts = line.split()
                    if len(parts) >= 2:
                        test_path = parts[1]
                        error_type = 'AssertionError'
                        if len(parts) > 2:
                            error_msg = ' '.join(parts[3:])
                            if error_msg:
                                error_type = error_msg.split(':')[0] if ':' in error_msg else error_type
                        summary['failures'].append({
                            'test_path': test_path,
                            'error_type': error_type
                        })
                elif line.startswith('PASSED'):
                    summary['total_passed'] += 1

                # Parse final count line
                match = re.search(r'(\d+)\s+(failed|passed)\s+in\s+([\d.]+)s', line)
                if match:
                    count = int(match.group(1))
                    status = match.group(2)
                    duration = float(match.group(3))
                    if status == 'failed':
                        summary['total_failed'] = count
                    elif status == 'passed':
                        summary['total_passed'] = count
                    summary['total_duration'] = duration

        return summary

    def _extract_test_name_from_header(self, line: str) -> str:
        """Extract test name from underscore header line."""
        # Remove underscores and whitespace
        cleaned = re.sub(r'^_+|_+$', '', line).strip()
        # Extract the middle part (test name)
        match = re.search(r'(\w+)', cleaned)
        return match.group(1) if match else ""

    def _extract_test_name_from_path(self, path: str) -> str:
        """Extract test name from file path if in ::test_name format."""
        if '::' in path:
            return path.split('::')[-1]
        return ""

    def _classify_assertion(self, line: str) -> Optional[str]:
        """Classify assertion type based on pattern."""
        if re.match(self.EQUALITY_PATTERN, line):
            return 'equality'
        elif re.match(self.CONTAINS_PATTERN, line):
            return 'contains'
        elif re.match(self.TYPE_CHECK_PATTERN, line):
            return 'type_check'
        elif ' and ' in line or ' or ' in line:
            return 'boolean'
        elif 'isinstance' in line:
            return 'type_check'
        return 'unknown'

    def _extract_assertion_details(self, failure: TestFailure, line: str) -> None:
        """Extract expected/actual values from assertion line."""
        # Try equality pattern: assert X == Y
        match = re.match(self.EQUALITY_PATTERN, line)
        if match:
            failure.expected = match.group(2).strip()
            failure.actual = match.group(1).strip()
            return

        # Try contains pattern: assert X in Y
        match = re.match(self.CONTAINS_PATTERN, line)
        if match:
            failure.actual = match.group(1).strip()
            failure.expected = f"in {match.group(2).strip()}"
            return

    def _extract_error_message_details(self, failure: TestFailure) -> None:
        """Extract expected/actual from error message if available."""
        msg = failure.error_message

        # Look for patterns like "Expected X, got Y"
        match = re.search(r"Expected\s+['\"]?([^'\"]+)['\"]?\s*,\s*got\s+['\"]?([^'\"]+)['\"]?", msg, re.IGNORECASE)
        if match:
            failure.expected = match.group(1).strip()
            failure.actual = match.group(2).strip()
            return

        # Look for patterns like "X != Y"
        match = re.search(r'(.+?)\s*!=\s*(.+)', msg)
        if match:
            failure.expected = match.group(2).strip()
            failure.actual = match.group(1).strip()

    def _parse_dict_value(self, value: str) -> Dict[str, str]:
        """Parse a simple dict value from diff line."""
        result = {}
        # Simple parsing for 'key': value format
        matches = re.findall(r"'([^']+)'\s*:\s*([^,}]+)", value)
        for key, val in matches:
            result[key.strip("'\" ")] = val.strip()
        return result


def extract_json_compatible_data(failures: List[TestFailure]) -> List[Dict[str, Any]]:
    """Convert TestFailure objects to JSON-serializable dictionaries."""
    return [f.to_dict() for f in failures]


def main():
    """Main entry point for command-line usage."""
    parser = argparse.ArgumentParser(
        description='Parse pytest output and extract structured failure data',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Parse from file
  python parse_pytest_output.py output.txt

  # Parse from stdin
  pytest --tb=line test_file.py | python parse_pytest_output.py

  # Output JSON
  python parse_pytest_output.py --json output.txt

  # Parse and save to file
  python parse_pytest_output.py --json -o parsed.json output.txt
        """
    )

    parser.add_argument(
        'input_file',
        nargs='?',
        help='Input file containing pytest output (reads from stdin if not provided)'
    )
    parser.add_argument(
        '--json',
        action='store_true',
        help='Output results as JSON instead of human-readable text'
    )
    parser.add_argument(
        '--output',
        '-o',
        help='Write output to file instead of stdout'
    )

    args = parser.parse_args()

    # Read input
    try:
        if args.input_file:
            with open(args.input_file, 'r') as f:
                output = f.read()
        else:
            output = sys.stdin.read()
    except Exception as e:
        print(f"Error reading input: {e}", file=sys.stderr)
        sys.exit(1)

    # Parse
    parser_instance = PytestOutputParser()
    failures = parser_instance.parse(output)
    summary = parser_instance.parse_test_summary(output)

    # Prepare output
    if args.json:
        result = {
            'failures': extract_json_compatible_data(failures),
            'summary': summary
        }
        output_text = json.dumps(result, indent=2, sort_keys=False)
    else:
        lines = []
        lines.append(f"=== Parsed {len(failures)} Failures ===\n")

        for i, failure in enumerate(failures, 1):
            lines.append(f"Failure #{i}:")
            lines.append(f"  Test: {failure.test_file}::{failure.test_name}")
            lines.append(f"  Line: {failure.line_number}")
            lines.append(f"  Type: {failure.error_type}")
            if failure.assertion_type:
                lines.append(f"  Assertion Type: {failure.assertion_type}")
            if failure.error_message:
                lines.append(f"  Message: {failure.error_message}")
            if failure.assertion_line:
                lines.append(f"  Assertion: {failure.assertion_line}")
            if failure.expected:
                lines.append(f"  Expected: {failure.expected}")
            if failure.actual:
                lines.append(f"  Actual: {failure.actual}")
            if failure.index_diff is not None:
                lines.append(f"  Index Diff: {failure.index_diff}")
            if failure.differing_items:
                lines.append(f"  Differing Items: {len(failure.differing_items)}")
                for item in failure.differing_items:
                    lines.append(f"    - {item['key']}: expected={item['expected']}, actual={item['actual']}")
            if failure.where_clause:
                lines.append(f"  Where: {failure.where_clause}")
            lines.append("")

        lines.append("=== Summary ===")
        lines.append(f"Total Failed: {summary['total_failed']}")
        lines.append(f"Total Passed: {summary['total_passed']}")
        lines.append(f"Duration: {summary['total_duration']}s")
        output_text = "\n".join(lines)

    # Write output
    try:
        if args.output:
            with open(args.output, 'w') as f:
                f.write(output_text)
        else:
            print(output_text)
    except Exception as e:
        print(f"Error writing output: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
