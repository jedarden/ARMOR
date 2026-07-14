#!/usr/bin/env python3
"""
Verify that all documented pytest parsing patterns work against the collected samples.

This script tests each pattern in pytest_parser.md against actual sample outputs.
"""

import re
import sys
from pathlib import Path
from dataclasses import dataclass
from typing import List, Dict, Any


@dataclass
class PatternTest:
    """Test result for a single pattern."""
    pattern_name: str
    regex: str
    sample_file: str
    matches_found: int
    sample_matches: List[str]


def load_samples() -> Dict[str, str]:
    """Load all pytest output samples."""
    samples_dir = Path("/home/coding/ARMOR/tools/test_samples")
    samples = {}

    for sample_file in samples_dir.glob("sample*.txt"):
        samples[sample_file.name] = sample_file.read_text()

    return samples


def test_pattern(pattern_name: str, pattern: str, samples: Dict[str, str]) -> List[PatternTest]:
    """Test a pattern against all samples."""
    results = []
    compiled = re.compile(pattern, re.MULTILINE)

    for sample_name, sample_content in samples.items():
        matches = compiled.findall(sample_content)
        if matches:
            # Show first few matches as examples
            sample_matches = [str(m)[:100] for m in matches[:3]]
            results.append(PatternTest(
                pattern_name=pattern_name,
                regex=pattern,
                sample_file=sample_name,
                matches_found=len(matches),
                sample_matches=sample_matches
            ))

    return results


def main():
    """Run all pattern tests."""
    samples = load_samples()

    # Define all patterns from pytest_parser.md
    patterns = {
        # Section 2: Failure Location
        "SHORT_FAILURE_PATTERN": r'^\s*(.+?):(\d+):\s+in\s+(\w+)',
        "LINE_FAILURE_PATTERN": r'^\s*(.+?):(\d+):\s+AssertionError:\s*(.+)',
        "FILE_LOCATION_PATTERN": r'^\s*(.+?):(\d+):',

        # Section 3: Assertion Types
        "ASSERT_PATTERN": r'^E?\s+assert\s+(.+)',
        "EQUALITY_PATTERN": r'^E?\s+assert\s+(.+?)\s+==\s+(.+)',
        "CONTAINS_PATTERN": r'^E?\s+assert\s+(.+?)\s+in\s+(.+)',
        "TYPE_CHECK_PATTERN": r'^E?\s+assert\s+isinstance\((.+?),\s*(.+?)\)',

        # Section 4: Diff Patterns
        "DIFF_MINUS_PATTERN": r'^\s*-\s*(.+)',
        "DIFF_PLUS_PATTERN": r'^\s*\+\s*(.+)',
        "DIFF_POSITION_PATTERN": r'^\s*\?\s+(.+)',
        "INDEX_DIFF_PATTERN": r'^\s+At\s+index\s+(\d+)\s+diff:\s+(.+?)\s+!=\s+(.+)',
        "DICT_DIFF_HEADER": r'^\s+Differing items:',
        "SET_DIFF_LEFT": r'^\s+Extra items in the left set:',
        "SET_DIFF_RIGHT": r'^\s+Extra items in the right set:',
        "RANGE_DIFF_PATTERN": r'^\s*(?:E\s+)?(Right|Left) contains (?:one more item|\d+ more items?)\s*:\s*(.+)',
        "WHERE_CLAUSE_PATTERN": r'^\s*\+\s+where\s+(.+?)\s+=\s+(.+)',

        # Section 5: Summary
        "SUMMARY_PATTERN": r'^(FAILED|PASSED|ERROR|SKIPPED)\s+(.+?)::(\w+)\s+-\s*(.+)',
        "COUNT_PATTERN": r'=\s+(\d+)\s+(failed|passed|error|skipped)\s+in\s+([\d.]+)s\s+=',

        # Section Markers
        "FAILURES_SECTION": r'^===+ FAILURES ===+',
        "SUMMARY_SECTION": r'^===+ short test summary info ===+',
        "SESSION_START": r'^===+ test session starts ===+',
        "SESSION_END": r'^===+ \d+ (?:failed|passed) in [\d.]+s ===+',
    }

    print("=" * 80)
    print("PYTEST PATTERN VERIFICATION")
    print("=" * 80)
    print(f"\nTesting {len(patterns)} patterns against {len(samples)} samples\n")

    all_results = {}
    for pattern_name, pattern in patterns.items():
        results = test_pattern(pattern_name, pattern, samples)
        all_results[pattern_name] = results

    # Print results
    for pattern_name, results in sorted(all_results.items()):
        if results:
            total_matches = sum(r.matches_found for r in results)
            print(f"✅ {pattern_name}")
            print(f"   Total matches: {total_matches}")
            for result in results:
                print(f"   - {result.sample_file}: {result.matches_found} matches")
                if result.sample_matches:
                    print(f"     Example: {result.sample_matches[0]}")
        else:
            print(f"❌ {pattern_name} - NO MATCHES")
        print()

    # Summary
    working = sum(1 for results in all_results.values() if results)
    print(f"\n{'='*80}")
    print(f"SUMMARY: {working}/{len(patterns)} patterns have matches")
    print(f"{'='*80}")


if __name__ == '__main__':
    main()
