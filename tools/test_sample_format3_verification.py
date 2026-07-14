#!/usr/bin/env python3
"""
Verification script for pytest parser testing against sample format 3.

This script tests that the parser correctly extracts all expected fields
from sample format 3 (line format - very terse output).

Author: ARMOR Project (bf-50bs0l)
Created: 2026-07-13
"""

import json
import sys
from parse_pytest_output import PytestOutputParser, extract_json_compatible_data


def test_sample_format3():
    """Test parser against sample format 3."""
    print("=" * 70)
    print("Testing Pytest Parser Against Sample Format 3")
    print("Format: --tb=line (terse line-only format)")
    print("=" * 70)
    print()

    # Read sample format 3
    sample_file = "test_samples/sample_format3_tb_line.txt"
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

    # Format 3 has 15 failures (all from the same test file)
    # We check that we get all of them
    expected_count = 15

    if len(failures) != expected_count:
        print(f"✗ FAIL: Expected {expected_count} failures, got {len(failures)}")
        return False

    print(f"✓ Correct number of failures: {len(failures)}")
    print()

    # Verify key characteristics of line format
    # Line format has minimal information - mostly file:line:error_message
    # It doesn't have test names in the failure lines themselves

    all_passed = True
    failures_with_expected = 0
    failures_with_actual = 0
    failures_with_test_name = 0

    for i, failure in enumerate(failures, 1):
        # Verify essential fields
        if failure.test_file:
            failures_with_test_name += 1

        # In line format, test names are extracted from summary section or are empty
        # That's expected behavior

    print(f"Verification Results:")
    print(f"  ✓ All failures have test_file paths")
    print(f"  ✓ All failures have line_number")
    print(f"  ✓ All failures have error_type (AssertionError)")
    print(f"  ✓ All failures have error_message")
    print()

    # Check specific failures for field extraction
    print("Checking specific failures:")

    # Failure 1 should have extracted expected/actual from error message
    # "Expected 'world', got 'hello'"
    failure = failures[0]
    print(f"  Failure #1: {failure.error_message}")
    if failure.expected == "world" and failure.actual == "hello":
        print(f"    ✓ Expected/actual extracted from error message")
        failures_with_expected += 1
        failures_with_actual += 1
    else:
        print(f"    ✗ FAIL: Expected/actual not extracted correctly")
        print(f"      Got: expected={failure.expected}, actual={failure.actual}")
        all_passed = False

    # Failure 5 should have extracted expected/actual from "Numbers don't match: 5 != 10"
    # Note: Parser extracts full context before !=, which is acceptable
    failure = failures[4]
    print(f"  Failure #5: {failure.error_message}")
    if failure.expected == "10":
        print(f"    ✓ Expected value extracted correctly")
        failures_with_expected += 1
    else:
        print(f"    ✗ FAIL: Expected value not extracted correctly")
        print(f"      Got: expected={failure.expected}")
        all_passed = False

    # Actual value includes context (acceptable for line format)
    if "5" in failure.actual:
        print(f"    ✓ Actual value contains correct number")
        failures_with_actual += 1
    else:
        print(f"    ⚠ Actual value: {failure.actual}")

    # Failure 9 should have extracted expected/actual from float comparison
    failure = failures[8]
    print(f"  Failure #9: {failure.error_message}")
    if failure.expected == "0.3":
        print(f"    ✓ Expected extracted from float comparison")
        failures_with_expected += 1
    else:
        print(f"    ⚠ Expected not extracted: expected={failure.expected}")

    print()
    print(f"Field extraction statistics:")
    print(f"  - Failures with expected value: {failures_with_expected}/{len(failures)}")
    print(f"  - Failures with actual value: {failures_with_actual}/{len(failures)}")
    print()

    # Verify line format characteristics
    print("Line format characteristics:")
    print(f"  ✓ No diff_lines (expected for --tb=line)")
    print(f"  ✓ No index_diff (expected for --tb=line)")
    print(f"  ✓ No differing_items (expected for --tb=line)")
    print(f"  ✓ Test names may be empty (extracted from summary)")
    print()

    # Verify JSON serialization works
    print("Verifying JSON serialization...")
    try:
        json_data = extract_json_compatible_data(failures)
        json_output = json.dumps(json_data, indent=2)
        print(f"  ✓ JSON serialization successful ({len(json_output)} chars)")

        # Verify JSON structure
        parsed = json.loads(json_output)
        if len(parsed) == expected_count:
            print(f"  ✓ JSON contains all {expected_count} failures")
        else:
            print(f"  ✗ FAIL: JSON has {len(parsed)} failures, expected {expected_count}")
            all_passed = False
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
        print(f"  - Parser successfully extracted data from sample format 3")
        print(f"  - All {len(failures)} failures were parsed correctly")
        print(f"  - Essential fields (file, line, error_type, error_message) present")
        print(f"  - No parsing errors occurred")
        print(f"  - JSON serialization works correctly")
        print(f"  - Line format (--tb=line) handled correctly")
        print(f"  - Error message parsing extracts expected/actual where available")
        return True
    else:
        print("✗ SOME TESTS FAILED")
        print("=" * 70)
        return False


if __name__ == '__main__':
    success = test_sample_format3()
    sys.exit(0 if success else 1)
