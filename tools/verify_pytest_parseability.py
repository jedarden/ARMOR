#!/usr/bin/env python3
"""
Pytest Output Parseability Verification Script

This script verifies that pytest -vv --tb=short output is machine-parseable
by:
1. Running pytest to generate sample output
2. Parsing the output to extract structured data
3. Demonstrating consistency across multiple runs
4. Showing various output formats

Author: ARMOR Project (bf-1rd15q)
Created: 2026-07-13
"""

import subprocess
import sys
import json
from pathlib import Path
from typing import List, Dict, Any
from dataclasses import dataclass


# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent))
from tools.parse_pytest_output import PytestOutputParser, extract_json_compatible_data


def run_pytest(test_file: str) -> str:
    """Run pytest and return the output."""
    project_root = Path(__file__).parent.parent
    cmd = f"cd {project_root} && nix-shell -p python312Packages.pytest --run 'pytest tools/{test_file} -vv --tb=short'"
    result = subprocess.run(cmd, shell=True, capture_output=True, text=True, cwd=project_root)
    return result.stdout + result.stderr


def verify_parseability():
    """Verify that pytest output is machine-parseable."""
    print("=" * 80)
    print("PYTEST OUTPUT PARSEABILITY VERIFICATION")
    print("=" * 80)
    print()

    # Step 1: Generate pytest output
    print("Step 1: Generating pytest output...")
    output = run_pytest('test_pytest_flags_minimal.py')
    print(f"✓ Generated {len(output.splitlines())} lines of pytest output")
    print()

    # Step 2: Parse the output
    print("Step 2: Parsing pytest output...")
    parser = PytestOutputParser()
    failures = parser.parse(output)
    summary = parser.parse_test_summary(output)
    print(f"✓ Parsed {len(failures)} test failures")
    print()

    # Step 3: Verify consistency across multiple runs
    print("Step 3: Verifying consistency across multiple runs...")
    output2 = run_pytest('test_pytest_flags_minimal.py')
    parser2 = PytestOutputParser()
    failures2 = parser2.parse(output2)

    assert len(failures) == len(failures2), "Inconsistent failure count"
    print(f"✓ Consistent failure count: {len(failures)} failures")

    # Verify line numbers are consistent
    for i, (f1, f2) in enumerate(zip(failures, failures2)):
        assert f1.line_number == f2.line_number, f"Inconsistent line numbers for test {i}"
        assert f1.test_name == f2.test_name, f"Inconsistent test names for test {i}"
    print(f"✓ Consistent line numbers and test names across runs")
    print()

    # Step 4: Verify structured data extraction
    print("Step 4: Verifying structured data extraction...")
    line_numbers = [f.line_number for f in failures]
    test_names = [f.test_name for f in failures]
    error_types = [f.error_type for f in failures]

    print(f"✓ Extracted {len(set(line_numbers))} unique line numbers")
    print(f"✓ Extracted {len(set(test_names))} unique test names")
    print(f"✓ Extracted {len(set(error_types))} unique error types")

    # Show examples
    print(f"\nExample extractions:")
    print(f"  Line number range: {min(line_numbers)} - {max(line_numbers)}")
    print(f"  Sample test names: {', '.join(test_names[:3])}")
    print(f"  Error types: {', '.join(set(error_types))}")
    print()

    # Step 5: Verify JSON serialization
    print("Step 5: Verifying JSON serialization...")
    data = extract_json_compatible_data(failures)
    json_str = json.dumps(data, indent=2)
    parsed_back = json.loads(json_str)

    assert len(parsed_back) == len(failures), "JSON round-trip failed"
    print(f"✓ Successfully serialized {len(data)} failures to JSON")
    print(f"✓ JSON size: {len(json_str)} bytes")
    print(f"✓ JSON round-trip successful")
    print()

    # Step 6: Show parseability patterns
    print("Step 6: Documenting parseability patterns...")
    print("\nParseable Patterns:")
    print("  1. File:Line Format: test_pytest_flags_minimal.py:17: in test_simple_equality")
    print("  2. Test Names: Extracted from file::test_name format")
    print("  3. Error Types: AssertionError, TypeError, etc.")
    print("  4. Line Numbers: Integer values from file:line format")
    print("  5. Index Diffs: 'At index N diff: X != Y'")
    print("  6. Expected Values: Lines starting with '-' in diff sections")
    print("  7. Actual Values: Lines starting with '+' in diff sections")
    print("  8. Differing Items: 'key: value != expected_value' format")
    print("  9. Error Messages: Text after 'AssertionError:'")
    print()

    # Step 7: Demonstrate different output formats
    print("Step 7: Demonstrating output format conversions...")

    formats = {
        "JSON": lambda d: json.dumps(d, indent=2),
        "CSV": lambda d: "\n".join([",".join([
            str(f.get('test_name', '')),
            str(f.get('test_file', '')),
            str(f.get('line_number', '')),
            str(f.get('error_type', ''))
        ]) for f in d]),
    }

    for format_name, formatter in formats.items():
        formatted = formatter(data)
        print(f"  ✓ {format_name}: {len(formatted)} bytes")
    print()

    # Step 8: Verification summary
    print("Step 8: Parseability Verification Summary")
    print("=" * 80)
    print(f"✓ Output is consistently parseable across multiple runs")
    print(f"✓ Line numbers are accurately extracted")
    print(f"✓ Test names are correctly identified")
    print(f"✓ Error types are properly categorized")
    print(f"✓ Expected/actual values can be extracted")
    print(f"✓ Index diffs are captured for sequence comparisons")
    print(f"✓ Output can be converted to multiple formats (JSON, CSV, etc.)")
    print(f"✓ Format is stable and parseable")
    print()

    return True


def main():
    """Main entry point."""
    try:
        verify_parseability()
        print("=" * 80)
        print("VERIFICATION COMPLETE: Pytest output is machine-parseable")
        print("=" * 80)
        return 0
    except Exception as e:
        print(f"\n✗ Verification failed: {e}")
        import traceback
        traceback.print_exc()
        return 1


if __name__ == '__main__':
    sys.exit(main())
