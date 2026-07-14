#!/usr/bin/env python3
"""
Comprehensive test suite for parse_pytest_output.py using real sample files.

Tests all 3 pytest output formats with actual collected samples:
- Format 1: -vv --tb=short (full detailed format)
- Format 2: -v --tb=long (long format with code context)
- Format 3: --tb=line (single line format)

Also tests edge cases and core regex patterns.
"""

import json
import sys
import re
sys.path.insert(0, '/home/coding/ARMOR/tools')

from parse_pytest_output import PytestOutputParser, TestFailure


class TestPytestParserComprehensive:
    """Comprehensive tests using real sample files."""

    def __init__(self):
        self.parser = PytestOutputParser()
        self.passed = 0
        self.failed = 0
        self.errors = []

    def test_format1_full_detailed(self):
        """Test Format 1: -vv --tb=short with real sample."""
        print("\n=== Testing Format 1: -vv --tb=short (Full Detailed) ===")

        with open('/home/coding/ARMOR/tools/test_samples/sample_format1_vv_short.txt', 'r') as f:
            sample = f.read()

        failures = self.parser.parse(sample)

        # Should have 3 failures
        assert len(failures) == 3, f"Expected 3 failures, got {len(failures)}"

        # Test 1: test_simple_equality
        f1 = failures[0]
        assert f1.test_name == "test_simple_equality", f"Test name mismatch: {f1.test_name}"
        assert f1.test_file == "tools/test_pytest_flags_minimal.py", f"File mismatch: {f1.test_file}"
        assert f1.line_number == 17, f"Line mismatch: {f1.line_number}"
        assert f1.expected == "'world'", f"Expected mismatch: {f1.expected}"
        assert f1.actual == "'hello'", f"Actual mismatch: {f1.actual}"
        assert f1.assertion_type == "equality", f"Assertion type mismatch: {f1.assertion_type}"

        # Test 2: test_dict_equality
        f2 = failures[1]
        assert f2.test_name == "test_dict_equality", f"Test name mismatch: {f2.test_name}"
        assert f2.line_number == 24, f"Line mismatch: {f2.line_number}"
        assert f2.error_message == "Dictionaries don't match", f"Error message mismatch: {f2.error_message}"
        assert len(f2.differing_items) == 3, f"Expected 3 differing items, got {len(f2.differing_items)}"
        assert len(f2.diff_lines) > 0, "Expected diff lines"

        # Test 3: test_list_comparison
        f3 = failures[2]
        assert f3.test_name == "test_list_comparison", f"Test name mismatch: {f3.test_name}"
        assert f3.line_number == 31, f"Line mismatch: {f3.line_number}"
        assert f3.index_diff == 2, f"Index diff mismatch: {f3.index_diff}"
        assert f3.expected == "10", f"Expected mismatch: {f3.expected}"
        assert f3.actual == "3", f"Actual mismatch: {f3.actual}"

        print("✅ Format 1: All assertions passed")
        print(f"   - Parsed {len(failures)} failures successfully")
        print(f"   - File paths extracted correctly")
        print(f"   - Line numbers extracted correctly")
        print(f"   - Expected/actual values extracted")
        print(f"   - Index diffs captured")
        print(f"   - Dict diffs captured")

    def test_format2_long_format(self):
        """Test Format 2: -v --tb=long with real sample."""
        print("\n=== Testing Format 2: -v --tb=long (Long Format) ===")

        with open('/home/coding/ARMOR/tools/test_samples/sample_format2_v_long.txt', 'r') as f:
            sample = f.read()

        failures = self.parser.parse(sample)

        # Should have 3 failures
        assert len(failures) == 3, f"Expected 3 failures, got {len(failures)}"

        # Test 1: test_simple_equality
        f1 = failures[0]
        assert f1.test_name == "test_simple_equality", f"Test name mismatch: {f1.test_name}"
        assert f1.test_file == "tools/test_pytest_flags_minimal.py", f"File mismatch: {f1.test_file}"
        assert f1.line_number == 17, f"Line mismatch: {f1.line_number}"
        assert f1.error_type == "AssertionError", f"Error type mismatch: {f1.error_type}"

        # Test 2: test_dict_equality
        f2 = failures[1]
        assert f2.test_name == "test_dict_equality", f"Test name mismatch: {f2.test_name}"
        assert f2.line_number == 24, f"Line mismatch: {f2.line_number}"

        # Test 3: test_list_comparison
        f3 = failures[2]
        assert f3.test_name == "test_list_comparison", f"Test name mismatch: {f3.test_name}"
        assert f3.line_number == 31, f"Line mismatch: {f3.line_number}"
        assert f3.index_diff == 2, f"Index diff mismatch: {f3.index_diff}"

        print("✅ Format 2: All assertions passed")
        print(f"   - Parsed {len(failures)} failures successfully")
        print(f"   - Handles truncated output warnings")
        print(f"   - Extracts data from long format")

    def test_format3_line_format(self):
        """Test Format 3: --tb=line with real sample."""
        print("\n=== Testing Format 3: --tb=line (Single Line Format) ===")

        with open('/home/coding/ARMOR/tools/test_samples/sample_format3_tb_line.txt', 'r') as f:
            sample = f.read()

        failures = self.parser.parse(sample)

        # Should have 15 failures
        assert len(failures) == 15, f"Expected 15 failures, got {len(failures)}"

        # Test first failure (has explicit expected/actual in message)
        f1 = failures[0]
        assert f1.test_file == "/home/coding/ARMOR/tools/test_pytest_flags_minimal.py", f"File mismatch: {f1.test_file}"
        assert f1.line_number == 17, f"Line mismatch: {f1.line_number}"
        assert f1.error_message == "Expected 'world', got 'hello'", f"Error message mismatch: {f1.error_message}"
        # Should extract expected/actual from message
        assert f1.expected == "world", f"Expected mismatch: {f1.expected}"
        assert f1.actual == "hello", f"Actual mismatch: {f1.actual}"

        # Test fifth failure (has != pattern in message)
        f5 = failures[4]
        assert f5.line_number == 51, f"Line mismatch: {f5.line_number}"
        assert "5 != 10" in f5.error_message, f"Error message should contain != pattern"
        assert f5.expected == "10", f"Expected mismatch: {f5.expected}"
        assert "5" in f5.actual, f"Actual mismatch: {f5.actual}"

        # Test floating point precision edge case
        f9 = failures[8]
        assert f9.line_number == 89, f"Line mismatch: {f9.line_number}"
        assert "0.30000000000000004" in f9.error_message, f"Should handle float precision: {f9.error_message}"

        print("✅ Format 3: All assertions passed")
        print(f"   - Parsed {len(failures)} failures successfully")
        print(f"   - Extracts expected/actual from messages")
        print(f"   - Handles != patterns")
        print(f"   - Handles floating point precision")

    def test_json_output_structure(self):
        """Test that JSON output contains all expected fields."""
        print("\n=== Testing JSON Output Structure ===")

        sample = """_____________________________ test_equality _____________________________
tools/test_file.py:10: in test_equality
    assert x == y
E   AssertionError: Values don't match
E   assert 42 == 100"""

        failures = self.parser.parse(sample)

        # Convert to JSON
        json_output = json.dumps([f.to_dict() for f in failures], indent=2)
        parsed = json.loads(json_output)

        assert len(parsed) == 1, f"Expected 1 failure in JSON, got {len(parsed)}"

        f = parsed[0]

        # Verify all required fields are present
        required_fields = [
            'test_name', 'test_file', 'line_number', 'error_type',
            'error_message', 'assertion_line', 'assertion_type',
            'expected', 'actual', 'diff_lines', 'index_diff',
            'differing_items', 'where_clause'
        ]

        for field in required_fields:
            assert field in f, f"Missing required field '{field}' in JSON output"

        # Verify values are correct
        assert f['test_file'] == "tools/test_file.py"
        assert f['line_number'] == 10
        assert f['expected'] == "100"
        assert f['actual'] == "42"
        assert isinstance(f['diff_lines'], list)
        assert isinstance(f['differing_items'], list)

        print("✅ JSON output structure: All assertions passed")
        print(f"   - All {len(required_fields)} required fields present")
        print(f"   - Values correctly serialized")
        print(f"   - Lists properly handled")

    def test_edge_cases(self):
        """Test edge cases identified in research phase."""
        print("\n=== Testing Edge Cases ===")

        # Test 1: Truncated output (Format 2)
        truncated = """_____________________________ test_dict _____________________________

    def test_dict():
>       assert actual == expected
E       AssertionError: Dictionaries don't match
E       assert {'age': 25, '...name': 'Bob'} == {'age': 30, '...ame': 'Alice'}
E
E         ...Full output truncated (14 lines hidden), use '-vv' to show

tools/test_file.py:24: AssertionError"""

        failures = self.parser.parse(truncated)
        assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
        assert failures[0].line_number == 24
        print("   ✅ Truncated output handled")

        # Test 2: Multiline string escapes
        multiline = """_____________________________ test_multiline _____________________________
tools/test_file.py:44: in test_multiline
    assert actual == expected
E   AssertionError: Multiline strings don't match
E   assert '\\n    This i...ontent.\\n    ' == '\\n    This i... lines.\\n    '"""

        failures = self.parser.parse(multiline)
        assert len(failures) == 1
        assert "\\n" in failures[0].actual or "..." in failures[0].actual
        print("   ✅ Multiline string escapes handled")

        # Test 3: Floating point precision
        float_precision = """_____________________________ test_float _____________________________
tools/test_file.py:89: in test_float
    assert a == b
E   AssertionError: Floats don't match: 0.30000000000000004 != 0.3
E   assert 0.30000000000000004 == 0.3"""

        failures = self.parser.parse(float_precision)
        assert len(failures) == 1
        assert "0.30000000000000004" in failures[0].actual or "0.3" in failures[0].expected
        print("   ✅ Floating point precision handled")

        # Test 4: Absolute vs relative paths
        abs_path = """/home/coding/ARMOR/tools/test_file.py:17: AssertionError: test failed"""
        failures = self.parser.parse(abs_path)
        assert len(failures) == 1
        assert failures[0].test_file == "/home/coding/ARMOR/tools/test_file.py"
        print("   ✅ Absolute paths handled")

        rel_path = """tools/test_file.py:17: AssertionError: test failed"""
        failures = self.parser.parse(rel_path)
        assert len(failures) == 1
        assert failures[0].test_file == "tools/test_file.py"
        print("   ✅ Relative paths handled")

        # Test 5: Boolean logic (no explicit expected/actual)
        boolean = """_____________________________ test_boolean _____________________________
tools/test_file.py:96: in test_boolean
    assert x and y
E   AssertionError: Both should be True"""

        failures = self.parser.parse(boolean)
        assert len(failures) == 1
        assert failures[0].assertion_type == "boolean" or failures[0].assertion_type == "unknown"
        print("   ✅ Boolean logic handled")

        print("✅ Edge cases: All tests passed")

    def test_regex_patterns(self):
        """Test core regex patterns individually."""
        print("\n=== Testing Core Regex Patterns ===")

        parser = self.parser

        # Test FILE_LOCATION_PATTERN
        match = re.match(parser.FILE_LOCATION_PATTERN, "tools/test_file.py:17:")
        assert match, "FILE_LOCATION_PATTERN should match"
        assert match.group(1) == "tools/test_file.py"
        assert match.group(2) == "17"
        print("   ✅ FILE_LOCATION_PATTERN works")

        # Test SHORT_FAILURE_PATTERN
        match = re.match(parser.SHORT_FAILURE_PATTERN, "tools/test_file.py:17: in test_name")
        assert match, "SHORT_FAILURE_PATTERN should match"
        assert match.group(1) == "tools/test_file.py"
        assert match.group(2) == "17"
        assert match.group(3) == "test_name"
        print("   ✅ SHORT_FAILURE_PATTERN works")

        # Test LINE_FAILURE_PATTERN
        match = re.match(parser.LINE_FAILURE_PATTERN, "tools/test.py:17: AssertionError: test failed")
        assert match, "LINE_FAILURE_PATTERN should match"
        assert match.group(1) == "tools/test.py"
        assert match.group(2) == "17"
        assert match.group(3) == "test failed"
        print("   ✅ LINE_FAILURE_PATTERN works")

        # Test EQUALITY_PATTERN
        match = re.match(parser.EQUALITY_PATTERN, "  assert x == y")
        assert match, "EQUALITY_PATTERN should match"
        assert match.group(1).strip() == "x"
        assert match.group(2).strip() == "y"
        print("   ✅ EQUALITY_PATTERN works")

        # Test INDEX_DIFF_PATTERN
        match = re.match(parser.INDEX_DIFF_PATTERN, "At index 2 diff: 3 != 10")
        assert match, "INDEX_DIFF_PATTERN should match"
        assert match.group(1) == "2"
        assert match.group(2) == "3"
        assert match.group(3) == "10"
        print("   ✅ INDEX_DIFF_PATTERN works")

        # Test DIFF_MINUS_PATTERN
        match = re.match(parser.DIFF_MINUS_PATTERN, "    - world")
        assert match, "DIFF_MINUS_PATTERN should match"
        assert match.group(1) == "world"
        print("   ✅ DIFF_MINUS_PATTERN works")

        # Test DIFF_PLUS_PATTERN
        match = re.match(parser.DIFF_PLUS_PATTERN, "    + hello")
        assert match, "DIFF_PLUS_PATTERN should match"
        assert match.group(1) == "hello"
        print("   ✅ DIFF_PLUS_PATTERN works")

        # Test DICT_DIFF_HEADER
        match = re.match(parser.DICT_DIFF_HEADER, "    Differing items:")
        assert match, "DICT_DIFF_HEADER should match"
        print("   ✅ DICT_DIFF_HEADER works")

        print("✅ Core regex patterns: All tests passed")

    def test_full_sample_parsing(self):
        """Test parsing complete sample files and verify JSON output."""
        print("\n=== Testing Full Sample Parsing ===")

        for i, (sample_file, format_name) in enumerate([
            ('sample_format1_vv_short.txt', 'Format 1 (-vv --tb=short)'),
            ('sample_format2_v_long.txt', 'Format 2 (-v --tb=long)'),
            ('sample_format3_tb_line.txt', 'Format 3 (--tb=line)')
        ], 1):
            path = f'/home/coding/ARMOR/tools/test_samples/{sample_file}'
            with open(path, 'r') as f:
                sample = f.read()

            failures = self.parser.parse(sample)

            # Verify we can convert to JSON without errors
            try:
                json_data = [f.to_dict() for f in failures]
                json_str = json.dumps(json_data, indent=2)

                # Verify we can parse it back
                parsed = json.loads(json_str)
                assert len(parsed) == len(failures)

                # Verify each failure has required fields
                for failure in parsed:
                    assert 'test_file' in failure
                    assert 'line_number' in failure

                print(f"   ✅ {format_name}: {len(failures)} failures parsed to valid JSON")

            except Exception as e:
                raise AssertionError(f"Failed to parse {sample_file} to JSON: {e}")

        print("✅ Full sample parsing: All tests passed")

    def run_all_tests(self):
        """Run all comprehensive tests."""
        print("=" * 80)
        print("COMPREHENSIVE PYTEST PARSER TEST SUITE")
        print("=" * 80)

        tests = [
            self.test_format1_full_detailed,
            self.test_format2_long_format,
            self.test_format3_line_format,
            self.test_json_output_structure,
            self.test_edge_cases,
            self.test_regex_patterns,
            self.test_full_sample_parsing,
        ]

        for test in tests:
            try:
                test()
                self.passed += 1
            except AssertionError as e:
                self.failed += 1
                self.errors.append((test.__name__, str(e)))
                print(f"\n❌ {test.__name__} FAILED: {e}")
            except Exception as e:
                self.failed += 1
                self.errors.append((test.__name__, f"Unexpected error: {e}"))
                print(f"\n❌ {test.__name__} ERROR: {e}")
                import traceback
                traceback.print_exc()

        # Print summary
        print("\n" + "=" * 80)
        print("TEST SUMMARY")
        print("=" * 80)
        print(f"Total tests: {len(tests)}")
        print(f"Passed: {self.passed}")
        print(f"Failed: {self.failed}")

        if self.errors:
            print("\nFailed tests:")
            for name, error in self.errors:
                print(f"  - {name}: {error}")
            return 1
        else:
            print("\n✅ ALL TESTS PASSED")
            print("\nAcceptance Criteria Verified:")
            print("  1. ✅ Parser tested against all 3 collected sample formats")
            print("  2. ✅ JSON output contains all expected fields")
            print("  3. ✅ Edge cases from pytest_patterns.md are handled")
            print("  4. ✅ Unit tests pass for core regex patterns")
            return 0


def main():
    """Main entry point."""
    tester = TestPytestParserComprehensive()
    return tester.run_all_tests()


if __name__ == '__main__':
    sys.exit(main())
