#!/usr/bin/env python3
"""
Test suite for pytest output parser validation.

This script tests the parser against different failure patterns to verify
machine-parseability and consistency.

Author: ARMOR Project (bf-1rd15q)
Created: 2026-07-13
"""

import subprocess
import sys
import json
from pathlib import Path


def run_pytest(test_file: str) -> str:
    """Run pytest and return the output."""
    # Change to project root to ensure paths are correct
    project_root = Path(__file__).parent.parent
    cmd = f"cd {project_root} && nix-shell -p python312Packages.pytest --run 'pytest {test_file} -vv --tb=short'"
    result = subprocess.run(
        cmd,
        shell=True,
        capture_output=True,
        text=True,
        cwd=project_root
    )
    return result.stdout + result.stderr


def test_parser_consistency():
    """Test that the parser produces consistent results across multiple runs."""
    print("=== Testing Parser Consistency ===\n")

    # Import the parser
    sys.path.insert(0, str(Path(__file__).parent.parent))
    from parse_pytest_output import PytestOutputParser, extract_json_compatible_data

    # Run pytest twice to verify consistency
    print("Running pytest (first run)...")
    output1 = run_pytest('test_pytest_flags_minimal.py')

    print("Running pytest (second run)...")
    output2 = run_pytest('test_pytest_flags_minimal.py')

    parser1 = PytestOutputParser()
    parser2 = PytestOutputParser()

    failures1 = parser1.parse(output1)
    failures2 = parser2.parse(output2)

    # Verify we get the same number of failures
    assert len(failures1) == len(failures2), \
        f"Inconsistent failure count: {len(failures1)} vs {len(failures2)}"
    print(f"✓ Consistent failure count: {len(failures1)}")

    # Verify each failure has required fields
    for i, f in enumerate(failures1):
        assert f.test_name, f"Failure {i}: missing test_name"
        assert f.test_file, f"Failure {i}: missing test_file"
        assert f.line_number > 0, f"Failure {i}: invalid line_number {f.line_number}"
        assert f.error_type, f"Failure {i}: missing error_type"
    print(f"✓ All {len(failures1)} failures have required fields")

    # Verify summary parsing
    summary1 = parser1.parse_test_summary(output1)
    summary2 = parser2.parse_test_summary(output2)

    assert summary1['total_failed'] == summary2['total_failed'], \
        f"Inconsistent summary failed count: {summary1['total_failed']} vs {summary2['total_failed']}"
    print(f"✓ Consistent summary: {summary1['total_failed']} failed")

    return failures1, summary1


def test_specific_patterns():
    """Test parsing of specific failure patterns."""
    print("\n=== Testing Specific Failure Patterns ===\n")

    sys.path.insert(0, str(Path(__file__).parent.parent))
    from parse_pytest_output import PytestOutputParser

    output = run_pytest('test_pytest_flags_minimal.py')
    parser = PytestOutputParser()
    failures = parser.parse(output)

    # Test 1: Simple equality (test_simple_equality)
    simple_fail = [f for f in failures if f.test_name == 'test_simple_equality'][0]
    assert simple_fail.line_number == 22, f"Wrong line number: {simple_fail.line_number}"
    assert 'hello' in simple_fail.error_message.lower(), f"Missing expected text in error message"
    print(f"✓ Simple equality: line={simple_fail.line_number}, message extracted")

    # Test 2: Index diff (test_list_comparison)
    list_fail = [f for f in failures if f.test_name == 'test_list_comparison'][0]
    assert list_fail.index_diff == 2, f"Wrong index diff: {list_fail.index_diff}"
    print(f"✓ List comparison: index_diff={list_fail.index_diff}")

    # Test 3: Long sequence (test_long_sequence)
    long_fail = [f for f in failures if f.test_name == 'test_long_sequence'][0]
    assert long_fail.index_diff == 5, f"Wrong index diff: {long_fail.index_diff}"
    print(f"✓ Long sequence: index_diff={long_fail.index_diff}")

    # Test 4: Dictionary comparison (test_dict_equality)
    dict_fail = [f for f in failures if f.test_name == 'test_dict_equality'][0]
    assert dict_fail.line_number == 39, f"Wrong line number: {dict_fail.line_number}"
    print(f"✓ Dictionary comparison: line={dict_fail.line_number}")

    # Test 5: Nested structure (test_nested_structure)
    nested_fail = [f for f in failures if f.test_name == 'test_nested_structure'][0]
    assert nested_fail.line_number == 122, f"Wrong line number: {nested_fail.line_number}"
    print(f"✓ Nested structure: line={nested_fail.line_number}")

    # Test 6: Numeric comparison (test_numeric_comparison)
    num_fail = [f for f in failures if f.test_name == 'test_numeric_comparison'][0]
    assert '5' in num_fail.error_message and '10' in num_fail.error_message, \
        f"Missing expected numbers in error message"
    print(f"✓ Numeric comparison: error_message contains expected values")


def test_json_serialization():
    """Test that parsed data can be serialized to JSON."""
    print("\n=== Testing JSON Serialization ===\n")

    sys.path.insert(0, str(Path(__file__).parent.parent))
    from parse_pytest_output import PytestOutputParser, extract_json_compatible_data

    output = run_pytest('test_pytest_flags_minimal.py')
    parser = PytestOutputParser()
    failures = parser.parse(output)

    # Convert to JSON-compatible format
    data = extract_json_compatible_data(failures)

    # Serialize to JSON
    json_str = json.dumps(data, indent=2)

    # Deserialize and verify
    parsed_back = json.loads(json_str)
    assert len(parsed_back) == len(failures), \
        f"JSON round-trip failed: {len(parsed_back)} vs {len(failures)}"

    print(f"✓ Successfully serialized {len(data)} failures to JSON")
    print(f"✓ JSON size: {len(json_str)} bytes")

    # Verify structure
    for item in parsed_back:
        assert 'test_name' in item
        assert 'line_number' in item
        assert 'error_type' in item
    print("✓ All required fields present in JSON")


def test_line_number_extraction():
    """Test that line numbers are accurately extracted."""
    print("\n=== Testing Line Number Extraction ===\n")

    sys.path.insert(0, str(Path(__file__).parent.parent))
    from parse_pytest_output import PytestOutputParser

    output = run_pytest('test_pytest_flags_minimal.py')
    parser = PytestOutputParser()
    failures = parser.parse(output)

    expected_line_numbers = {
        'test_simple_equality': 22,
        'test_dict_equality': 39,
        'test_list_comparison': 46,
        'test_multiline_string': 61,
        'test_numeric_comparison': 66,
        'test_in_operator': 78,
        'test_long_sequence': 87,
        'test_nested_structure': 122,
        'test_approximate_numbers': 133,
        'test_boolean_logic': 142,
        'test_string_operations': 153,
    }

    for failure in failures:
        expected = expected_line_numbers.get(failure.test_name)
        if expected:
            assert failure.line_number == expected, \
                f"{failure.test_name}: expected line {expected}, got {failure.line_number}"
            print(f"✓ {failure.test_name}: line {failure.line_number}")

    print(f"\n✓ All {len(expected_line_numbers)} line numbers extracted correctly")


def test_expected_actual_extraction():
    """Test extraction of expected vs actual values."""
    print("\n=== Testing Expected/Actual Extraction ===\n")

    sys.path.insert(0, str(Path(__file__).parent.parent))
    from parse_pytest_output import PytestOutputParser

    output = run_pytest('test_pytest_flags_minimal.py')
    parser = PytestOutputParser()
    failures = parser.parse(output)

    # Count failures with expected/actual data
    with_diff = sum(1 for f in failures if f.diff_lines)
    with_index = sum(1 for f in failures if f.index_diff is not None)

    print(f"✓ Failures with diff lines: {with_diff}")
    print(f"✓ Failures with index diff: {with_index}")

    # Verify specific cases
    list_fail = [f for f in failures if f.test_name == 'test_list_comparison'][0]
    assert list_fail.index_diff == 2, f"List comparison failed to extract index diff"
    print(f"✓ List index diff correctly extracted: {list_fail.index_diff}")


def main():
    """Run all tests."""
    print("=" * 70)
    print("Pytest Output Parser Validation Suite")
    print("=" * 70)
    print()

    tests = [
        test_parser_consistency,
        test_specific_patterns,
        test_json_serialization,
        test_line_number_extraction,
        test_expected_actual_extraction,
    ]

    passed = 0
    failed = 0

    for test in tests:
        try:
            test()
            passed += 1
        except AssertionError as e:
            print(f"\n✗ Test failed: {test.__name__}")
            print(f"  Error: {e}")
            failed += 1
        except Exception as e:
            print(f"\n✗ Test error: {test.__name__}")
            print(f"  Error: {e}")
            failed += 1

    print("\n" + "=" * 70)
    print(f"Results: {passed} passed, {failed} failed")
    print("=" * 70)

    return 0 if failed == 0 else 1


if __name__ == '__main__':
    sys.exit(main())
