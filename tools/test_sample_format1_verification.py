#!/usr/bin/env python3
"""
Verification script for pytest parser testing against sample format 1.

This script tests that the parser correctly extracts all expected fields
from sample format 1 (verbose short format with underscore headers).

Author: ARMOR Project (bf-3274tg)
Created: 2026-07-13
"""

import json
import sys
from parse_pytest_output import PytestOutputParser, extract_json_compatible_data


def test_sample_format1():
    """Test parser against sample format 1."""
    print("=" * 70)
    print("Testing Pytest Parser Against Sample Format 1")
    print("=" * 70)
    print()

    # Read sample format 1
    sample_file = "tools/test_samples/sample_format1_vv_short.txt"
    with open(sample_file, 'r') as f:
        sample_output = f.read()

    print(f"✓ Read sample from: {sample_file}")
    print()

    # Parse the output
    parser = PytestOutputParser()
    failures = parser.parse(sample_output)
    summary = parser.parse_test_summary(sample_output)

    print(f"✓ Parsed {len(failures)} failures")
    print()

    # Expected tests in this sample
    expected_tests = [
        "test_simple_equality",
        "test_dict_equality",
        "test_list_comparison"
    ]

    # Verify we got the right number of failures
    if len(failures) != len(expected_tests):
        print(f"✗ FAIL: Expected {len(expected_tests)} failures, got {len(failures)}")
        return False

    print(f"✓ Correct number of failures: {len(failures)}")
    print()

    # Verify each failure
    all_passed = True
    for i, (failure, expected_name) in enumerate(zip(failures, expected_tests), 1):
        print(f"Failure #{i}: {expected_name}")
        print("-" * 70)

        # Verify test name
        if failure.test_name != expected_name:
            print(f"  ✗ FAIL: test_name mismatch - expected '{expected_name}', got '{failure.test_name}'")
            all_passed = False
        else:
            print(f"  ✓ test_name: {failure.test_name}")

        # Verify essential fields are present
        required_fields = {
            'test_file': str,
            'line_number': int,
            'error_type': str,
            'error_message': str,
            'assertion_line': str,
            'assertion_type': str,
            'expected': str,
            'actual': str
        }

        for field, field_type in required_fields.items():
            value = getattr(failure, field)
            if value is None or value == "":
                print(f"  ✗ FAIL: Missing required field '{field}'")
                all_passed = False
            elif not isinstance(value, field_type):
                print(f"  ✗ FAIL: Field '{field}' has wrong type - expected {field_type}, got {type(value)}")
                all_passed = False
            else:
                print(f"  ✓ {field}: {repr(value)}")

        # Verify diff_lines
        if failure.diff_lines:
            print(f"  ✓ diff_lines: {len(failure.diff_lines)} lines")
        else:
            print(f"  ⚠ diff_lines: empty (optional)")

        # Verify expected specific fields per test
        if expected_name == "test_simple_equality":
            # Should have simple diff
            if len(failure.diff_lines) >= 2:
                print(f"  ✓ Has simple expected/actual diff")
            else:
                print(f"  ✗ FAIL: Should have diff lines")
                all_passed = False

            # Should not have index_diff or differing_items
            if failure.index_diff is None:
                print(f"  ✓ No index_diff (correct)")
            if not failure.differing_items:
                print(f"  ✓ No differing_items (correct)")

        elif expected_name == "test_dict_equality":
            # Should have differing_items
            if failure.differing_items:
                print(f"  ✓ differing_items: {len(failure.differing_items)} items")
                for item in failure.differing_items:
                    if 'key' in item and 'expected' in item and 'actual' in item:
                        print(f"    - {item['key']}: {item['actual']} != {item['expected']}")
                    else:
                        print(f"    ✗ FAIL: Invalid differing item structure")
                        all_passed = False
            else:
                print(f"  ✗ FAIL: Should have differing_items")
                all_passed = False

            # Should have diff_lines for full diff
            if len(failure.diff_lines) >= 6:
                print(f"  ✓ Has full dict diff ({len(failure.diff_lines)} lines)")
            else:
                print(f"  ✗ FAIL: Should have full diff lines")
                all_passed = False

        elif expected_name == "test_list_comparison":
            # Should have index_diff
            if failure.index_diff is not None:
                print(f"  ✓ index_diff: {failure.index_diff}")
                if failure.index_diff == 2:
                    print(f"    ✓ Correct index (2)")
                else:
                    print(f"    ✗ FAIL: Wrong index - expected 2, got {failure.index_diff}")
                    all_passed = False
            else:
                print(f"  ✗ FAIL: Should have index_diff")
                all_passed = False

            # Index diff should override expected/actual with specific elements
            if failure.actual == "3" and failure.expected == "10":
                print(f"  ✓ index_diff correctly overrides expected/actual")
            else:
                print(f"  ✗ FAIL: expected/actual not correctly set by index_diff")
                print(f"    Expected: expected='10', actual='3'")
                print(f"    Got: expected='{failure.expected}', actual='{failure.actual}'")
                all_passed = False

        print()

    # Verify JSON serialization works
    print("Verifying JSON serialization...")
    try:
        json_data = extract_json_compatible_data(failures)
        json_output = json.dumps(json_data, indent=2)
        print(f"  ✓ JSON serialization successful ({len(json_output)} chars)")
    except Exception as e:
        print(f"  ✗ FAIL: JSON serialization failed: {e}")
        all_passed = False

    print()
    print("=" * 70)

    if all_passed:
        print("✓ ALL TESTS PASSED")
        print("=" * 70)
        print()
        print("Summary:")
        print(f"  - Parser successfully extracted data from sample format 1")
        print(f"  - All {len(failures)} failures were parsed correctly")
        print(f"  - All expected fields are present")
        print(f"  - No parsing errors occurred")
        print(f"  - JSON serialization works correctly")
        return True
    else:
        print("✗ SOME TESTS FAILED")
        print("=" * 70)
        return False


if __name__ == '__main__':
    success = test_sample_format1()
    sys.exit(0 if success else 1)
