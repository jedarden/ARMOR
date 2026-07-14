#!/usr/bin/env python3
"""
Comprehensive verification script for pytest parser against all sample formats.

This script tests that the parser correctly handles all three collected pytest
output formats:
- Format 1: --tb=short (verbose short format with underscore headers)
- Format 2: --tb=long (verbose long format with detailed diffs)
- Format 3: --tb=line (terse line-only format)

Author: ARMOR Project (bf-50bs0l)
Created: 2026-07-13
"""

import sys
import subprocess


def run_test(script_name, format_name):
    """Run a verification script and return success status."""
    print(f"\n{'=' * 70}")
    print(f"Testing {format_name}")
    print(f"{'=' * 70}\n")

    result = subprocess.run(
        ['python3', script_name],
        capture_output=False,
        text=True
    )

    return result.returncode == 0


def main():
    """Run all format verification tests."""
    print("=" * 70)
    print("COMPREHENSIVE PARSER VERIFICATION - ALL SAMPLE FORMATS")
    print("=" * 70)
    print()

    tests = [
        ('test_sample_format1_verification.py', 'Sample Format 1 (--tb=short)'),
        ('test_sample_format2_verification.py', 'Sample Format 2 (--tb=long)'),
        ('test_sample_format3_verification.py', 'Sample Format 3 (--tb=line)')
    ]

    results = {}
    for script, name in tests:
        results[name] = run_test(script, name)

    print("\n" + "=" * 70)
    print("FINAL RESULTS")
    print("=" * 70)
    print()

    all_passed = True
    for name, passed in results.items():
        status = "✓ PASS" if passed else "✗ FAIL"
        print(f"{status}: {name}")
        if not passed:
            all_passed = False

    print()
    print("=" * 70)

    if all_passed:
        print("✓ ALL SAMPLE FORMATS VERIFIED SUCCESSFULLY")
        print("=" * 70)
        print()
        print("Summary:")
        print("  - Parser successfully handles all 3 pytest output formats")
        print("  - Format 1 (--tb=short): ✓")
        print("  - Format 2 (--tb=long): ✓")
        print("  - Format 3 (--tb=line): ✓")
        print("  - All expected fields extracted correctly")
        print("  - JSON serialization works for all formats")
        print("  - No parsing errors occurred")
        return 0
    else:
        print("✗ SOME FORMATS FAILED VERIFICATION")
        print("=" * 70)
        return 1


if __name__ == '__main__':
    sys.exit(main())
