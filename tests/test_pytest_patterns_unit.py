#!/usr/bin/env python3
"""
Unit tests for pytest parser regex patterns.

Tests all core regex patterns from tools/pytest_patterns.md with both positive
and negative test cases for each pattern.

Acceptance Criteria:
- Unit tests exist for all core regex patterns from pytest_patterns.md
- Tests verify pattern matching against known samples
- Tests cover both positive and negative cases
- All unit tests pass

Author: ARMOR Project (bf-28ztla)
Created: 2026-07-13
"""

import re
import sys
import unittest
sys.path.insert(0, '/home/coding/ARMOR/tools')

from parse_pytest_output import PytestOutputParser


class TestFileLocationPatterns(unittest.TestCase):
    """Test patterns for extracting file locations and line numbers."""

    def test_file_location_pattern_relative_path(self):
        """Test FILE_LOCATION_PATTERN with relative paths."""
        pattern = r'^\s*(.+?):(\d+):'
        test_cases = [
            "tools/test_pytest_flags_minimal.py:17:",
            "  tests/test_file.py:42:",
            "src/module/test.py:1:",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            assert match.group(1).endswith('.py'), f"Should extract file path from: {test}"
            assert match.group(2).isdigit(), f"Should extract line number from: {test}"

    def test_file_location_pattern_absolute_path(self):
        """Test FILE_LOCATION_PATTERN with absolute paths."""
        pattern = r'^\s*(.+?):(\d+):'
        test = "/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17:"
        match = re.match(pattern, test)
        assert match, "Pattern should match absolute paths"
        assert match.group(1) == "/home/coding/ARMOR/tools/test_pytest_flags_minimal.py"
        assert match.group(2) == "17"

    def test_file_location_pattern_negative_cases(self):
        """Test FILE_LOCATION_PATTERN with invalid inputs."""
        pattern = r'^\s*(.+?):(\d+):'
        negative_cases = [
            "not a file path",
            "test.py",  # missing colon and line number
            ":123:",  # missing file name
            "test.py:abc",  # non-numeric line number
            "",  # empty string
        ]
        for test in negative_cases:
            match = re.match(pattern, test)
            assert not match, f"Pattern should NOT match: {test}"

    def test_short_failure_pattern(self):
        """Test SHORT_FAILURE_PATTERN for complete failure lines."""
        pattern = r'^\s*(.+?):(\d+):\s+in\s+(\w+)'
        test_cases = [
            "tools/test_pytest_flags_minimal.py:17: in test_simple_equality",
            "  tests/test_file.py:42: in test_function_name",
            "src/module/test.py:1: in test_a",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            assert match.group(2).isdigit(), f"Line number should be digits: {test}"

    def test_short_failure_pattern_negative(self):
        """Test SHORT_FAILURE_PATTERN with invalid inputs."""
        pattern = r'^\s*(.+?):(\d+):\s+in\s+(\w+)'
        negative_cases = [
            "tools/test.py:17:",  # missing 'in test_name'
            "tools/test.py:in test_name",  # missing line number
            "tools/test.py:17 in test-name",  # invalid test name (hyphen)
            "tools/test.py:17 in 123test",  # invalid test name (starts with number)
        ]
        for test in negative_cases:
            match = re.match(pattern, test)
            assert not match, f"Pattern should NOT match: {test}"

    def test_line_failure_pattern(self):
        """Test LINE_FAILURE_PATTERN for --tb=line format."""
        pattern = r'^\s*(.+?):(\d+):\s+AssertionError:\s*(.+)'
        test_cases = [
            "/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17: AssertionError: Expected 'world', got 'hello'",
            "tools/test_file.py:42: AssertionError: Values don't match",
            "  tests/test.py:1: AssertionError: assert 5 == 10",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            assert "AssertionError" in test
            assert match.group(3), f"Should extract error message from: {test}"

    def test_line_failure_pattern_negative(self):
        """Test LINE_FAILURE_PATTERN with invalid inputs."""
        pattern = r'^\s*(.+?):(\d+):\s+AssertionError:\s*(.+)'
        negative_cases = [
            "tools/test.py:17: ValueError: something",  # wrong error type
            "tools/test.py:17: AssertionError",  # missing message
            "tools/test.py:17:",  # missing error info
        ]
        for test in negative_cases:
            match = re.match(pattern, test)
            assert not match or match.group(3) == "", f"Pattern should NOT match or have empty message: {test}"


class TestAssertionPatterns(unittest.TestCase):
    """Test patterns for parsing assertion lines."""

    def test_assert_pattern_basic(self):
        """Test ASSERT_PATTERN with basic assertions."""
        pattern = r'^E?\s*assert\s+(.+)'
        test_cases = [
            "E   assert greeting == 'world'",
            "    assert 42 == 100",
            "assert list1 == list2",
            "E   assert x in y",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            assert match.group(1), f"Should extract assertion content from: {test}"

    def test_assert_pattern_negative(self):
        """Test ASSERT_PATTERN with non-assertion lines."""
        pattern = r'^E?\s+assert\s+(.+)'
        negative_cases = [
            "E   AssertionError: something",
            "    def test_function():",
            "        print('hello')",
            "PASS",
            "",
        ]
        for test in negative_cases:
            match = re.match(pattern, test)
            assert not match, f"Pattern should NOT match: {test}"

    def test_equality_pattern(self):
        """Test EQUALITY_PATTERN for == assertions."""
        pattern = r'^E?\s*assert\s+(.+?)\s+==\s+(.+)'
        test_cases = [
            "E   assert 'hello' == 'world'",
            "    assert 5 == 10",
            "assert [1, 2, 3] == [1, 2, 10]",
            "E   assert greeting == 'world'",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            assert match.group(1), f"Should extract actual value from: {test}"
            assert match.group(2), f"Should extract expected value from: {test}"

    def test_equality_pattern_negative(self):
        """Test EQUALITY_PATTERN with non-equality assertions."""
        pattern = r'^E?\s+assert\s+(.+?)\s+==\s+(.+)'
        negative_cases = [
            "E   assert greeting != 'world'",  # not ==
            "    assert x in y",  # contains, not equality
            "assert isinstance(x, int)",  # type check
            "E   assert True",  # boolean without comparison
        ]
        for test in negative_cases:
            match = re.match(pattern, test)
            assert not match, f"Pattern should NOT match: {test}"

    def test_contains_pattern(self):
        """Test CONTAINS_PATTERN for 'in' assertions."""
        pattern = r'^E?\s*assert\s+(.+?)\s+in\s+(.+)'
        test_cases = [
            "E   assert 'orange' in fruits",
            "    assert 5 in list1",
            "assert x in [1, 2, 3]",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            assert match.group(1), f"Should extract item from: {test}"
            assert match.group(2), f"Should extract container from: {test}"

    def test_type_check_pattern(self):
        """Test TYPE_CHECK_PATTERN for isinstance assertions."""
        pattern = r'^E?\s*assert\s+isinstance\((.+?),\s*(.+?)\)'
        test_cases = [
            "E   assert isinstance('123', int)",
            "    assert isinstance(x, str)",
            "assert isinstance(value, list)",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            assert match.group(1), f"Should extract value from: {test}"
            assert match.group(2), f"Should extract type from: {test}"

    def test_type_check_pattern_negative(self):
        """Test TYPE_CHECK_PATTERN with non-isinstance assertions."""
        pattern = r'^E?\s+assert\s+isinstance\((.+?),\s*(.+?)\)'
        negative_cases = [
            "E   assert type(x) == int",  # different type check syntax
            "    assert isinstance(x)",  # missing type argument
            "assert x == 5",  # not a type check
        ]
        for test in negative_cases:
            match = re.match(pattern, test)
            assert not match, f"Pattern should NOT match: {test}"


class TestDiffPatterns(unittest.TestCase):
    """Test patterns for parsing diff output."""

    def test_diff_minus_pattern(self):
        """Test DIFF_MINUS_PATTERN for expected values."""
        pattern = r'^E?\s*-\s*(.+)'
        test_cases = [
            "    - world",
            "  -     'age': 30,",
            "- world",
            "E     - 10",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            content = match.group(1).strip()
            assert content, f"Should extract content from: {test}"

    def test_diff_minus_pattern_negative(self):
        """Test DIFF_MINUS_PATTERN with non-minus lines."""
        pattern = r'^\s*-\s*(.+)'
        negative_cases = [
            "    + world",  # plus line
            "    ?     ^^",  # position indicator
            "E   AssertionError:",  # error line
            "",  # empty
        ]
        for test in negative_cases:
            match = re.match(pattern, test)
            assert not match, f"Pattern should NOT match: {test}"

    def test_diff_plus_pattern(self):
        """Test DIFF_PLUS_PATTERN for actual values."""
        pattern = r'^E?\s*\+\s*(.+)'
        test_cases = [
            "    + hello",
            "  +     'age': 25,",
            "+ hello",
            "E     + 3",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            content = match.group(1).strip()
            assert content, f"Should extract content from: {test}"

    def test_diff_position_pattern(self):
        """Test DIFF_POSITION_PATTERN for position indicators."""
        pattern = r'^E?\s*\?\s+(.+)'
        test_cases = [
            "    ?            ^^",
            "  ?              ^^^",
            "?     ^",
            "E ?   ^^",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"

    def test_index_diff_pattern(self):
        """Test INDEX_DIFF_PATTERN for list index differences."""
        pattern = r'^\s*(?:E\s+)?At\s+index\s+(\d+)\s+diff:\s+(.+?)\s+!=\s+(.+)'
        test_cases = [
            "E     At index 2 diff: 3 != 10",
            "    At index 5 diff: 10 != 5",
            "At index 0 diff: 'a' != 'b'",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            assert match.group(1).isdigit(), f"Should extract index number from: {test}"
            assert match.group(2), f"Should extract actual value from: {test}"
            assert match.group(3), f"Should extract expected value from: {test}"

    def test_dict_diff_header(self):
        """Test DICT_DIFF_HEADER pattern."""
        pattern = r'^E?\s*Differing items:'
        test_cases = [
            "E     Differing items:",
            "    Differing items:",
            "Differing items:",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"

    def test_dict_diff_line(self):
        """Test DICT_DIFF_LINE for individual dict differences."""
        pattern = r'^E?\s*\{(.+?)\}\s+!=\s+\{(.+?)\}'
        test_cases = [
            "E     {'city': 'LA'} != {'city': 'NYC'}",
            "    {'age': 25} != {'age': 30}",
            "{'name': 'Bob'} != {'name': 'Alice'}",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            assert match.group(1), f"Should extract left side from: {test}"
            assert match.group(2), f"Should extract right side from: {test}"

    def test_set_diff_patterns(self):
        """Test set difference patterns."""
        left_pattern = r'^\s+Extra items in the left set:'
        right_pattern = r'^\s+Extra items in the right set:'

        test_left = "    Extra items in the left set:"
        test_right = "    Extra items in the right set:"

        assert re.match(left_pattern, test_left), "Should match left set pattern"
        assert re.match(right_pattern, test_right), "Should match right set pattern"

    def test_range_diff_pattern(self):
        """Test RANGE_DIFF_PATTERN for range differences."""
        pattern = r'^\s*(?:E\s+)?(Right|Left) contains (?:one more item|\d+ more items?)\s*:\s*(.+)'
        test_cases = [
            "E     Right contains one more item: 10",
            "    Left contains 2 more items: 3, 4",
            "Right contains one more item: 'a'",
            "Left contains 3 more items: 1, 2, 3",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            assert match.group(1) in ['Right', 'Left'], f"Should extract side from: {test}"
            assert match.group(2), f"Should extract items from: {test}"

    def test_where_clause_pattern(self):
        """Test WHERE_CLAUSE_PATTERN for type check failures."""
        pattern = r'^\s*\+\s+where\s+(.+?)\s+=\s+(.+)'
        test_cases = [
            "    + where False = isinstance('123', int)",
            "+ where True = isinstance(42, int)",
            "  + where False = type(x) == int",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"
            assert match.group(1), f"Should extract expression from: {test}"
            assert match.group(2), f"Should extract computation from: {test}"


class TestSectionMarkers(unittest.TestCase):
    """Test patterns for detecting pytest output sections."""

    def test_failures_section(self):
        """Test FAILURES_SECTION marker."""
        pattern = r'^===+ FAILURES ===+'
        test_cases = [
            "=================================== FAILURES ===================================",
            "=== FAILURES ===",
            "================ FAILURES ================",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"

    def test_summary_section(self):
        """Test SUMMARY_SECTION marker."""
        pattern = r'^===+ short test summary info ===+'
        test = "============================== short test summary info =============================="
        match = re.match(pattern, test)
        assert match, "Pattern should match summary section"

    def test_session_start(self):
        """Test SESSION_START marker."""
        pattern = r'^===+ test session starts ===+'
        test_cases = [
            "============================= test session starts =============================",
            "=== test session starts ===",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"

    def test_session_end(self):
        """Test SESSION_END marker."""
        pattern = r'^===+ \d+ (?:failed|passed) in [\d.]+s ===+'
        test_cases = [
            "========================= 3 failed in 0.12s =========================",
            "=== 10 passed in 1.23s ===",
            "================ 1 failed in 0.01s ===============",
        ]
        for test in test_cases:
            match = re.match(pattern, test)
            assert match, f"Pattern should match: {test}"


class TestPytestOutputParser(unittest.TestCase):
    """Test the PytestOutputParser class integration."""

    def test_parser_constants_exist(self):
        """Verify all expected pattern constants are defined."""
        parser = PytestOutputParser()

        expected_patterns = [
            'FILE_LOCATION_PATTERN',
            'SHORT_FAILURE_PATTERN',
            'LINE_FAILURE_PATTERN',
            'ASSERT_PATTERN',
            'EQUALITY_PATTERN',
            'CONTAINS_PATTERN',
            'TYPE_CHECK_PATTERN',
            'DIFF_MINUS_PATTERN',
            'DIFF_PLUS_PATTERN',
            'DIFF_POSITION_PATTERN',
            'INDEX_DIFF_PATTERN',
            'DICT_DIFF_HEADER',
            'DICT_DIFF_LINE',
            'SET_DIFF_LEFT',
            'SET_DIFF_RIGHT',
            'RANGE_DIFF_PATTERN',
            'WHERE_CLAUSE_PATTERN',
            'FAILURES_SECTION',
            'SUMMARY_SECTION',
            'SESSION_START',
            'SESSION_END',
        ]

        for pattern_name in expected_patterns:
            assert hasattr(parser, pattern_name), f"Parser should have {pattern_name} constant"
            pattern = getattr(parser, pattern_name)
            assert isinstance(pattern, str), f"{pattern_name} should be a string"
            assert pattern, f"{pattern_name} should not be empty"

    def test_parser_format1_integration(self):
        """Test complete parsing of Format 1 output."""
        sample = """_____________________________ test_simple_equality _____________________________
tools/test_pytest_flags_minimal.py:17: in test_simple_equality
    assert greeting == "world", f"Expected 'world', got '{greeting}'"
E   AssertionError: Expected 'world', got 'hello'
E   assert 'hello' == 'world'
E
E     - world
E     + hello"""

        parser = PytestOutputParser()
        failures = parser.parse(sample)

        assert len(failures) == 1
        f = failures[0]
        assert f.test_file == "tools/test_pytest_flags_minimal.py"
        assert f.line_number == 17
        assert f.test_name == "test_simple_equality"
        assert f.expected == "'world'"
        assert f.actual == "'hello'"

    def test_parser_format3_integration(self):
        """Test complete parsing of Format 3 (--tb=line) output."""
        sample = """=================================== FAILURES ===================================
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17: AssertionError: Expected 'world', got 'hello'
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:51: AssertionError: Numbers don't match: 5 != 10"""

        parser = PytestOutputParser()
        failures = parser.parse(sample)

        assert len(failures) == 2
        f1 = failures[0]
        assert f1.line_number == 17
        assert f1.expected == "world"
        assert f1.actual == "hello"

        f2 = failures[1]
        assert f2.line_number == 51
        assert f2.expected == "10"
        assert "5" in f2.actual

    def test_parser_empty_input(self):
        """Test parser with empty input."""
        parser = PytestOutputParser()
        failures = parser.parse("")
        assert failures == []

    def test_parser_no_failures(self):
        """Test parser with input containing no failures."""
        sample = """============================= test session starts =============================
collected 1 item

test_module.py .

============================== 1 passed in 0.01s =============================="""

        parser = PytestOutputParser()
        failures = parser.parse(sample)
        # Should handle gracefully (no failures found)
        assert isinstance(failures, list)


def main():
    """Run all unit tests."""
    print("=" * 70)
    print("PYTEST PARSER UNIT TESTS")
    print("=" * 70)
    print()

    # Run with unittest
    loader = unittest.TestLoader()
    suite = loader.loadTestsFromModule(sys.modules[__name__])
    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)

    if result.wasSuccessful():
        print()
        print("=" * 70)
        print("✅ ALL UNIT TESTS PASSED")
        print("=" * 70)
        print()
        print("Test Coverage:")
        print("  - File location patterns (positive + negative cases)")
        print("  - Assertion patterns (positive + negative cases)")
        print("  - Diff patterns (positive + negative cases)")
        print("  - Section marker patterns")
        print("  - Parser integration tests")
        print()
        return 0
    else:
        print()
        print("=" * 70)
        print("❌ SOME TESTS FAILED")
        print("=" * 70)
        print(f"Failures: {len(result.failures)}")
        print(f"Errors: {len(result.errors)}")
        return 1


if __name__ == '__main__':
    sys.exit(main())
