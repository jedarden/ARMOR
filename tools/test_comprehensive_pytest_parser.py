#!/usr/bin/env python3
"""
Comprehensive test suite for pytest output parser.

This test suite covers:
1. All 3 sample formats with real data
2. Edge cases from research phase
3. Unit tests for core regex patterns
4. JSON output validation
5. Complex failure scenarios

Acceptance Criteria:
- Parser successfully extracts data from all 3 sample formats
- Unit tests pass for core regex patterns
- Script outputs valid JSON with correct structure
- Edge cases from pytest_patterns.md are handled
"""

import json
import sys
import re
from pathlib import Path
sys.path.insert(0, '/home/coding/ARMOR/tools')

from parse_pytest_output import PytestOutputParser, TestFailure


class TestRegexPatterns:
    """Unit tests for core regex patterns."""

    def test_file_location_pattern(self):
        """Test FILE_LOCATION_PATTERN extracts file and line."""
        pattern = re.compile(PytestOutputParser.FILE_LOCATION_PATTERN)

        # Test cases from all formats
        test_cases = [
            "tools/test_pytest_flags_minimal.py:17:",
            "/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17:",
            "tests/test_example.py:42:",
            "/absolute/path/to/test.py:100:",
        ]

        for case in test_cases:
            match = pattern.match(case)
            assert match, f"Pattern should match: {case}"
            file_path = match.group(1)
            line_num = match.group(2)
            assert line_num.isdigit(), f"Line number should be digits: {line_num}"
            assert file_path.endswith('.py'), f"File should end with .py: {file_path}"

        print("✅ FILE_LOCATION_PATTERN works")

    def test_short_failure_pattern(self):
        """Test SHORT_FAILURE_PATTERN extracts file, line, and test name."""
        pattern = re.compile(PytestOutputParser.SHORT_FAILURE_PATTERN)

        test_cases = [
            ("tools/test_pytest_flags_minimal.py:17: in test_simple_equality",
             "tools/test_pytest_flags_minimal.py", "17", "test_simple_equality"),
            ("/home/coding/ARMOR/tools/test_file.py:42: in test_complex_assertion",
             "/home/coding/ARMOR/tools/test_file.py", "42", "test_complex_assertion"),
        ]

        for line, expected_file, expected_line, expected_test in test_cases:
            match = pattern.match(line)
            assert match, f"Pattern should match: {line}"
            assert match.group(1) == expected_file
            assert match.group(2) == expected_line
            assert match.group(3) == expected_test

        print("✅ SHORT_FAILURE_PATTERN works")

    def test_line_failure_pattern(self):
        """Test LINE_FAILURE_PATTERN for --tb=short format."""
        pattern = re.compile(PytestOutputParser.LINE_FAILURE_PATTERN)

        test_cases = [
            ("/home/coding/ARMOR/tools/test.py:17: AssertionError: Expected 'world', got 'hello'",
             "/home/coding/ARMOR/tools/test.py", "17", "Expected 'world', got 'hello'"),
            ("tests/test.py:42: AssertionError: Values don't match",
             "tests/test.py", "42", "Values don't match"),
        ]

        for line, expected_file, expected_line, expected_msg in test_cases:
            match = pattern.match(line)
            assert match, f"Pattern should match: {line}"
            assert match.group(1) == expected_file
            assert match.group(2) == expected_line
            assert match.group(3) == expected_msg

        print("✅ LINE_FAILURE_PATTERN works")

    def test_equality_pattern(self):
        """Test EQUALITY_PATTERN extracts left and right sides."""
        pattern = re.compile(PytestOutputParser.EQUALITY_PATTERN)

        test_cases = [
            ("E   assert 'hello' == 'world'", "'hello'", "'world'"),
            ("    assert 5 == 10", "5", "10"),
            ("E   assert [1, 2, 3] == [1, 2, 10]", "[1, 2, 3]", "[1, 2, 10]"),
            ("    assert greeting == \"world\"", "greeting", '"world"'),
        ]

        for line, expected_actual, expected_expected in test_cases:
            match = pattern.match(line)
            assert match, f"Pattern should match: {line}"
            assert match.group(1).strip() == expected_actual, f"For '{line}': got '{match.group(1)}', expected '{expected_actual}'"
            assert match.group(2).strip() == expected_expected, f"For '{line}': got '{match.group(2)}', expected '{expected_expected}'"

        print("✅ EQUALITY_PATTERN works")

    def test_contains_pattern(self):
        """Test CONTAINS_PATTERN for membership assertions."""
        pattern = re.compile(PytestOutputParser.CONTAINS_PATTERN)

        test_cases = [
            ("    assert 'orange' in fruits", "'orange'", "fruits"),
            ("E   assert 5 in [1, 2, 3]", "5", "[1, 2, 3]"),
        ]

        for line, expected_item, expected_container in test_cases:
            match = pattern.match(line)
            assert match, f"Pattern should match: {line}"
            assert match.group(1).strip() == expected_item
            assert match.group(2).strip() == expected_container

        print("✅ CONTAINS_PATTERN works")

    def test_index_diff_pattern(self):
        """Test INDEX_DIFF_PATTERN for list comparison failures."""
        pattern = re.compile(PytestOutputParser.INDEX_DIFF_PATTERN)

        test_cases = [
            ("E     At index 2 diff: 3 != 10", "2", "3", "10"),
            ("At index 5 diff: 10 != 5", "5", "10", "5"),
            ("E   At index 0 diff: 'a' != 'b'", "0", "'a'", "'b'"),
        ]

        for line, expected_index, expected_actual, expected_expected in test_cases:
            match = pattern.match(line)
            assert match, f"Pattern should match: {line}"
            assert match.group(1) == expected_index
            assert match.group(2).strip() == expected_actual
            assert match.group(3).strip() == expected_expected

        print("✅ INDEX_DIFF_PATTERN works")


class TestSampleFormats:
    """Test parser against real sample files."""

    def __init__(self):
        self.samples_dir = Path("/home/coding/ARMOR/tools/test_samples")

    def test_format1_full_detailed(self):
        """Test Format 1: -vv --tb=short with real sample file."""
        sample_file = self.samples_dir / "sample_format1_vv_short.txt"
        sample_content = sample_file.read_text()

        parser = PytestOutputParser()
        failures = parser.parse(sample_content)

        # Should have 3 failures from this sample
        assert len(failures) == 3, f"Expected 3 failures, got {len(failures)}"

        # Check first failure (simple equality)
        f1 = failures[0]
        assert f1.test_file == "tools/test_pytest_flags_minimal.py"
        assert f1.line_number == 17
        assert f1.test_name == "test_simple_equality"
        assert f1.expected == "'world'"
        assert f1.actual == "'hello'"
        assert f1.assertion_type == "equality"

        # Check second failure (dict equality)
        f2 = failures[1]
        assert f2.test_name == "test_dict_equality"
        assert f2.line_number == 24
        assert f2.assertion_type == "equality"
        assert len(f2.differing_items) > 0, "Should extract differing items from dict comparison"

        # Check third failure (list comparison)
        f3 = failures[2]
        assert f3.test_name == "test_list_comparison"
        assert f3.index_diff == 2, f"Should extract index diff, got {f3.index_diff}"

        print("✅ Format 1 (sample_format1_vv_short.txt) parsed correctly")

    def test_format2_long_format(self):
        """Test Format 2: -v --tb=long with real sample file."""
        sample_file = self.samples_dir / "sample_format2_v_long.txt"
        sample_content = sample_file.read_text()

        parser = PytestOutputParser()
        failures = parser.parse(sample_content)

        # Should have 3 failures
        assert len(failures) == 3, f"Expected 3 failures, got {len(failures)}"

        # Format 2 has file:line at the END of failure block
        f1 = failures[0]
        assert f1.test_file == "tools/test_pytest_flags_minimal.py"
        assert f1.line_number == 17
        assert f1.error_type == "AssertionError"

        print("✅ Format 2 (sample_format2_v_long.txt) parsed correctly")

    def test_format3_line_format(self):
        """Test Format 3: --tb=line with real sample file."""
        sample_file = self.samples_dir / "sample_format3_tb_line.txt"
        sample_content = sample_file.read_text()

        parser = PytestOutputParser()
        failures = parser.parse(sample_content)

        # Should have 15 failures
        assert len(failures) == 15, f"Expected 15 failures, got {len(failures)}"

        # Check first few failures
        f1 = failures[0]
        assert f1.test_file == "/home/coding/ARMOR/tools/test_pytest_flags_minimal.py"
        assert f1.line_number == 17
        assert "Expected 'world', got 'hello'" in f1.error_message

        f4 = failures[4]
        assert f4.line_number == 51
        assert "Numbers don't match" in f4.error_message
        # Should extract expected/actual from != pattern
        # Note: The error message parsing extracts from "5 != 10" pattern
        assert "10" in f4.expected or f4.expected == "10"

        print("✅ Format 3 (sample_format3_tb_line.txt) parsed correctly")


class TestEdgeCases:
    """Test edge cases identified in research phase."""

    def test_floating_point_precision(self):
        """Test floating-point precision edge case."""
        sample = """_____________________________ test_float _____________________________
tools/test_file.py:89: in test_float
    assert result == expected
E   AssertionError: Floats don't match: 0.30000000000000004 != 0.3
E   assert 0.30000000000000004 == 0.3"""

        parser = PytestOutputParser()
        failures = parser.parse(sample)

        assert len(failures) == 1
        f = failures[0]
        assert "Floats don't match" in f.error_message
        # Should extract both values including the precision error
        assert f.actual == "0.30000000000000004"

        print("✅ Floating-point precision edge case handled")

    def test_multiline_string_escapes(self):
        """Test multiline string with escape sequences."""
        sample = """_____________________________ test_multiline _____________________________
tools/test_file.py:44: in test_multiline
    assert actual == expected
E   AssertionError: Multiline strings don't match
E   assert '\\n    This i...ontent.\\n    ' == '\\n    This i... lines.\\n    '"""

        parser = PytestOutputParser()
        failures = parser.parse(sample)

        assert len(failures) == 1
        f = failures[0]
        assert f.test_file == "tools/test_file.py"
        assert f.line_number == 44
        assert "\\n" in f.actual or "\\n" in f.expected

        print("✅ Multiline string escapes edge case handled")

    def test_truncated_output(self):
        """Test handling of truncated output in Format 2."""
        sample = """_____________________________ test_dict _____________________________

    def test_dict_equality():
        expected = {"name": "Alice", "age": 30, "city": "NYC"}
        actual = {"name": "Bob", "age": 25, "city": "LA"}
>       assert actual == expected
E       AssertionError: Dictionaries don't match
E       assert {'age': 25, '...ame': 'Bob'} == {'age': 30, '...ame': 'Alice'}

E         ...Full output truncated (14 lines hidden), use '-vv' to show

tools/test_file.py:24: AssertionError"""

        parser = PytestOutputParser()
        failures = parser.parse(sample)

        # Should still parse even with truncation
        assert len(failures) == 1
        f = failures[0]
        assert f.test_file == "tools/test_file.py"
        assert f.line_number == 24
        # Values will have ellipsis from truncation
        assert "..." in f.actual or "..." in f.expected

        print("✅ Truncated output edge case handled")

    def test_absolute_vs_relative_paths(self):
        """Test both absolute and relative path formats."""
        # Relative path
        sample1 = """_____________________________ test_path _____________________________
tools/test_file.py:10: in test_path
    assert True
E   AssertionError: test failed"""

        parser = PytestOutputParser()
        failures = parser.parse(sample1)

        assert len(failures) == 1
        assert failures[0].test_file == "tools/test_file.py"

        # Absolute path
        sample2 = """/home/coding/ARMOR/tools/test_file.py:10: AssertionError: test failed"""

        parser2 = PytestOutputParser()
        failures2 = parser2.parse(sample2)

        assert len(failures2) == 1
        assert failures2[0].test_file == "/home/coding/ARMOR/tools/test_file.py"

        print("✅ Absolute vs relative paths edge case handled")

    def test_whitespace_variations(self):
        """Test different whitespace patterns (E prefix vs no prefix)."""
        # With E prefix (Format 1 & 2)
        sample1 = """_____________________________ test_whitespace _____________________________
tools/test_file.py:10: in test_whitespace
    assert x == y
E   AssertionError: test
E   assert 1 == 2"""

        parser = PytestOutputParser()
        failures = parser.parse(sample1)

        assert len(failures) == 1
        assert failures[0].error_message == "test"

        print("✅ Whitespace variations edge case handled")

    def test_boolean_logic(self):
        """Test boolean logic without explicit expected/actual."""
        sample = """_____________________________ test_boolean _____________________________
tools/test_file.py:96: in test_boolean
    assert (True and False)
E   AssertionError: Both x and y should be True"""

        parser = PytestOutputParser()
        failures = parser.parse(sample)

        assert len(failures) == 1
        f = failures[0]
        assert f.assertion_type == "boolean" or f.assertion_type == "unknown"
        assert "Both x and y should be True" in f.error_message

        print("✅ Boolean logic edge case handled")


class TestJSONOutput:
    """Test JSON output structure and validation."""

    def test_json_structure(self):
        """Test that JSON output contains all required fields."""
        sample = """_____________________________ test_example _____________________________
tools/test_file.py:10: in test_example
    assert x == y
E   AssertionError: Values don't match
E   assert 42 == 100
E
E     - 100
E     + 42"""

        parser = PytestOutputParser()
        failures = parser.parse(sample)

        # Convert to JSON
        json_data = json.dumps([f.to_dict() for f in failures])
        parsed = json.loads(json_data)

        assert len(parsed) == 1
        f = parsed[0]

        # Verify all required fields exist
        required_fields = [
            'test_name', 'test_file', 'line_number', 'error_type',
            'error_message', 'assertion_line', 'assertion_type',
            'expected', 'actual', 'diff_lines', 'index_diff',
            'differing_items', 'where_clause'
        ]

        for field in required_fields:
            assert field in f, f"Missing required field: {field}"

        # Verify field types
        assert isinstance(f['test_file'], str)
        assert isinstance(f['line_number'], int)
        assert isinstance(f['diff_lines'], list)
        assert isinstance(f['differing_items'], list)

        print("✅ JSON output structure is valid")

    def test_json_serialization(self):
        """Test that all data types are JSON-serializable."""
        sample = """_____________________________ test_dict_complex _____________________________
tools/test_file.py:30: in test_dict_complex
    assert actual == expected
E   AssertionError: Dictionaries don't match
E   assert {'name': 'Bob', 'age': 25} == {'name': 'Alice', 'age': 30}

E     Differing items:
E     {'name': 'Bob'} != {'name': 'Alice'}
E     {'age': 25} != {'age': 30}"""

        parser = PytestOutputParser()
        failures = parser.parse(sample)

        # Should not raise an exception
        try:
            json_data = json.dumps({
                'failures': [f.to_dict() for f in failures],
                'summary': parser.parse_test_summary(sample)
            }, indent=2)
            parsed = json.loads(json_data)

            # Verify nested structures are preserved
            assert len(parsed['failures']) == 1
            assert 'differing_items' in parsed['failures'][0]

        except (TypeError, ValueError) as e:
            raise AssertionError(f"JSON serialization failed: {e}")

        print("✅ JSON serialization works for all data types")


class TestComplexScenarios:
    """Test complex real-world scenarios."""

    def test_multiple_failures(self):
        """Test parsing multiple failures in one output."""
        sample = """_____________________________ test_first _____________________________
tools/test.py:10: in test_first
    assert a == b
E   AssertionError: first failed
E   assert 1 == 2
_____________________________ test_second _____________________________
tools/test.py:20: in test_second
    assert c == d
E   AssertionError: second failed
E   assert 'x' == 'y'
_____________________________ test_third _____________________________
tools/test.py:30: in test_third
    assert e == f
E   AssertionError: third failed
E   assert 3.14 == 2.71"""

        parser = PytestOutputParser()
        failures = parser.parse(sample)

        assert len(failures) == 3

        # Verify each failure
        assert failures[0].test_name == "test_first"
        assert failures[0].line_number == 10

        assert failures[1].test_name == "test_second"
        assert failures[1].line_number == 20

        assert failures[2].test_name == "test_third"
        assert failures[2].line_number == 30

        print("✅ Multiple failures parsed correctly")

    def test_nested_structure_comparison(self):
        """Test nested structure comparison failures."""
        sample = """_____________________________ test_nested _____________________________
tools/test.py:81: in test_nested
    assert actual == expected
E   AssertionError: Nested structures should match
E   assert {'users': [{'name': 'Bob', 'scores': [92, 87, 91]}]} == {'users': [{'name': 'Bob', 'scores': [92, 99, 91]}]}

E     At index 0 diff: {'name': 'Bob', 'scores': [92, 87, 91]} != {'name': 'Bob', 'scores': [92, 99, 91]}"""

        parser = PytestOutputParser()
        failures = parser.parse(sample)

        assert len(failures) == 1
        f = failures[0]
        assert f.test_name == "test_nested"
        assert f.index_diff == 0

        print("✅ Nested structure comparison handled")


def run_all_tests():
    """Run all test classes."""
    print("=" * 80)
    print("COMPREHENSIVE PYTEST PARSER TEST SUITE")
    print("=" * 80)
    print()

    test_classes = [
        ("Regex Pattern Unit Tests", TestRegexPatterns),
        ("Sample Format Tests", TestSampleFormats),
        ("Edge Case Tests", TestEdgeCases),
        ("JSON Output Tests", TestJSONOutput),
        ("Complex Scenario Tests", TestComplexScenarios),
    ]

    total_passed = 0
    total_failed = 0

    for class_name, test_class in test_classes:
        print(f"\n{class_name}")
        print("-" * 80)

        instance = test_class()
        test_methods = [m for m in dir(instance) if m.startswith('test_')]

        for method_name in test_methods:
            try:
                method = getattr(instance, method_name)
                method()
                total_passed += 1
            except AssertionError as e:
                print(f"❌ {method_name}: {e}")
                total_failed += 1
            except Exception as e:
                print(f"❌ {method_name}: Unexpected error: {e}")
                import traceback
                traceback.print_exc()
                total_failed += 1

    print()
    print("=" * 80)
    print(f"TEST RESULTS: {total_passed} passed, {total_failed} failed")
    print("=" * 80)

    if total_failed == 0:
        print("\n✅ ALL TESTS PASSED - Parser meets all acceptance criteria")
        print("\nVerified:")
        print("  ✅ All 3 sample formats parse correctly")
        print("  ✅ Regex patterns work for all core patterns")
        print("  ✅ JSON output contains all expected fields")
        print("  ✅ Edge cases from research phase are handled")
        print("  ✅ Complex real-world scenarios work correctly")
        return 0
    else:
        print(f"\n❌ {total_failed} test(s) failed")
        return 1


if __name__ == '__main__':
    sys.exit(run_all_tests())
