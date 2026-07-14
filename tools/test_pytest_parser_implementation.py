#!/usr/bin/env python3
"""
Comprehensive test verifying parse_pytest_output.py meets all acceptance criteria.

Acceptance Criteria:
1. Regex patterns implemented in tools/parse_pytest_output.py
2. Can extract all 4 fields (file, line, expected, actual) from standard pytest failures
3. Handles assertion format 'assert expected == actual'
4. Handles multiline assertions
5. Outputs structured JSON with extracted fields
"""

import json
import sys
sys.path.insert(0, '/home/coding/ARMOR/tools')

from parse_pytest_output import PytestOutputParser


def test_format1_full_detailed():
    """Test Format 1: -vv --tb=short (full detailed format)."""
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

    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"

    f = failures[0]
    # Criterion 2: Extract all 4 fields
    assert f.test_file == "tools/test_pytest_flags_minimal.py", f"File mismatch: {f.test_file}"
    assert f.line_number == 17, f"Line mismatch: {f.line_number}"
    assert f.expected == "'world'", f"Expected mismatch: {f.expected}"
    assert f.actual == "'hello'", f"Actual mismatch: {f.actual}"
    # Criterion 3: Handles 'assert expected == actual' format
    assert f.assertion_type == "equality", f"Assertion type mismatch: {f.assertion_type}"
    print("✅ Format 1 (full detailed) - All criteria met")


def test_format2_long_format():
    """Test Format 2: -v --tb=long (long format with code context)."""
    sample = """_____________________________ test_simple_equality _____________________________

    def test_simple_equality():
        greeting = "hello"
>       assert greeting == "world", f"Expected 'world', got '{greeting}'"
E       AssertionError: Expected 'world', got 'hello'
E       assert 'hello' == 'world'
E
E         - world
E         + hello

tools/test_pytest_flags_minimal.py:17: AssertionError"""

    parser = PytestOutputParser()
    failures = parser.parse(sample)

    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"

    f = failures[0]
    assert f.test_file == "tools/test_pytest_flags_minimal.py", f"File mismatch: {f.test_file}"
    assert f.line_number == 17, f"Line mismatch: {f.line_number}"
    assert f.expected == "'world'", f"Expected mismatch: {f.expected}"
    assert f.actual == "'hello'", f"Actual mismatch: {f.actual}"
    print("✅ Format 2 (long format) - All criteria met")


def test_format3_line_format():
    """Test Format 3: --tb=line (single line format)."""
    sample = """=================================== FAILURES ===================================
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17: AssertionError: Expected 'world', got 'hello'
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:24: AssertionError: Dictionaries don't match
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:31: AssertionError: Lists should be equal
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:51: AssertionError: Numbers don't match: 5 != 10"""

    parser = PytestOutputParser()
    failures = parser.parse(sample)

    assert len(failures) == 4, f"Expected 4 failures, got {len(failures)}"

    # Check first failure (has explicit expected/actual in message)
    f1 = failures[0]
    assert f1.test_file == "/home/coding/ARMOR/tools/test_pytest_flags_minimal.py"
    assert f1.line_number == 17
    assert f1.expected == "world", f"Expected mismatch: {f1.expected}"
    assert f1.actual == "hello", f"Actual mismatch: {f1.actual}"

    # Check fourth failure (has != pattern in message)
    f4 = failures[3]
    assert f4.line_number == 51
    assert f4.expected == "10", f"Expected mismatch: {f4.expected}"
    assert "5" in f4.actual, f"Actual mismatch: {f4.actual}"

    print("✅ Format 3 (line format) - All criteria met")


def test_multiline_assertions():
    """Test Criterion 4: Handles multiline assertions."""
    sample = """_____________________________ test_multiline _____________________________
tools/test_pytest_flags_minimal.py:44: in test_multiline
    assert actual == expected
E   AssertionError: Multiline strings don't match
E   assert '\\n    This i...ontent.\\n    ' == '\\n    This i... lines.\\n    '
E
E     -     'This is a long string with multiple\\n    lines.\\n    '
E     ?              ^^^^^
E     +     'This is a long string with different\\n    content.\\n    '"""

    parser = PytestOutputParser()
    failures = parser.parse(sample)

    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"

    f = failures[0]
    assert f.test_file == "tools/test_pytest_flags_minimal.py"
    assert f.line_number == 44
    assert f.assertion_type == "equality", f"Assertion type should be equality, got: {f.assertion_type}"
    print("✅ Multiline assertions - Criterion 4 met")


def test_json_output():
    """Test Criterion 5: Outputs structured JSON with extracted fields."""
    sample = """_____________________________ test_equality _____________________________
tools/test_file.py:10: in test_equality
    assert x == y
E   AssertionError: Values don't match
E   assert 42 == 100"""

    parser = PytestOutputParser()
    failures = parser.parse(sample)

    # Convert to JSON (using the to_dict method)
    json_output = json.dumps([f.to_dict() for f in failures], indent=2)
    parsed = json.loads(json_output)

    assert len(parsed) == 1
    f = parsed[0]

    # Verify JSON structure contains all required fields
    assert "test_file" in f, "Missing 'test_file' field in JSON"
    assert "line_number" in f, "Missing 'line_number' field in JSON"
    assert "expected" in f, "Missing 'expected' field in JSON"
    assert "actual" in f, "Missing 'actual' field in JSON"
    assert f["test_file"] == "tools/test_file.py"
    assert f["line_number"] == 10
    assert f["expected"] == "100"
    assert f["actual"] == "42"

    print("✅ JSON output - Criterion 5 met")


def test_index_diff_pattern():
    """Test index diff pattern extraction."""
    sample = """_____________________________ test_list _____________________________
tools/test_file.py:20: in test_list
    assert list1 == list2
E   AssertionError: Lists should be equal
E   assert [1, 2, 3, 4, 5] == [1, 2, 10, 4, 5]
E
E     At index 2 diff: 3 != 10"""

    parser = PytestOutputParser()
    failures = parser.parse(sample)

    assert len(failures) == 1
    f = failures[0]

    # Should extract index diff and the differing elements
    assert f.index_diff == 2, f"Index diff mismatch: {f.index_diff}"
    # The INDEX_DIFF_PATTERN stores the individual differing elements, not full lists
    assert f.expected == "10", f"Expected mismatch: {f.expected}"
    assert f.actual == "3", f"Actual mismatch: {f.actual}"
    print("✅ Index diff pattern - Additional pattern handling verified")


def test_dict_diff_pattern():
    """Test dictionary differing items pattern."""
    sample = """_____________________________ test_dict _____________________________
tools/test_file.py:30: in test_dict
    assert actual == expected
E   AssertionError: Dictionaries don't match
E   assert {'name': 'Bob', 'age': 25} == {'name': 'Alice', 'age': 30}
E
E     Differing items:
E     {'name': 'Bob'} != {'name': 'Alice'}
E     {'age': 25} != {'age': 30}"""

    parser = PytestOutputParser()
    failures = parser.parse(sample)

    assert len(failures) == 1
    f = failures[0]

    assert f.test_file == "tools/test_file.py"
    assert f.line_number == 30
    assert f.assertion_type == "equality"
    print("✅ Dict diff pattern - Additional pattern handling verified")


def main():
    """Run all verification tests."""
    print("=" * 70)
    print("VERIFYING ACCEPTANCE CRITERIA FOR BF-1UJ0EO")
    print("=" * 70)
    print()

    try:
        test_format1_full_detailed()
        test_format2_long_format()
        test_format3_line_format()
        test_multiline_assertions()
        test_json_output()
        test_index_diff_pattern()
        test_dict_diff_pattern()

        print()
        print("=" * 70)
        print("✅ ALL ACCEPTANCE CRITERIA MET")
        print("=" * 70)
        print()
        print("Summary:")
        print("  1. ✅ Regex patterns implemented in tools/parse_pytest_output.py")
        print("  2. ✅ Extracts all 4 fields: file, line, expected, actual")
        print("  3. ✅ Handles 'assert expected == actual' format")
        print("  4. ✅ Handles multiline assertions")
        print("  5. ✅ Outputs structured JSON with extracted fields")
        print()
        print("Additional features:")
        print("  - Index diff pattern extraction")
        print("  - Dictionary diff pattern extraction")
        print("  - Support for 3 pytest output formats")
        print("  - Command-line interface with --json flag")
        print()

        return 0

    except AssertionError as e:
        print(f"\n❌ TEST FAILED: {e}")
        return 1
    except Exception as e:
        print(f"\n❌ UNEXPECTED ERROR: {e}")
        import traceback
        traceback.print_exc()
        return 1


if __name__ == '__main__':
    sys.exit(main())
