# Pytest Failure Output Samples

This directory contains representative pytest failure output samples showing different assertion formats and failure modes, collected for machine-parseable output validation (bead: bf-1ym8jw).

## Sample Files

### 1. `sample1_standard_vv_tbshort.txt`
**Command:** `pytest -vv --tb=short tools/test_pytest_flags_minimal.py`

**Format Characteristics:**
- Full verbosity (`-vv`): Shows complete diff output
- Short traceback: Shows function context but not full call stack
- Detailed assertion rewriting with `+`/`-` markers
- Index-based diffs for sequences ("At index 2 diff: 3 != 10")
- "Differing items" for dictionary mismatches
- "Extra items in left/right set" for set operations

**Assertion Types Covered:** 15 different patterns

### 2. `sample2_v_tb_long.txt`
**Command:** `pytest -v --tb=long tools/test_pytest_flags_minimal.py`

**Format Characteristics:**
- Verbose (`-v`): Shows test names and progress
- Long traceback: Full function call stack with source code context
- Shows the function definition lines leading to the assertion
- Truncated diffs with "Full output truncated (X lines hidden), use '-vv' to show"
- More context for debugging but harder to parse

**Assertion Types Covered:** 15 different patterns

### 3. `sample3_tb_line.txt`
**Command:** `pytest --tb=line tools/test_pytest_flags_minimal.py`

**Format Characteristics:**
- Line traceback: Single line per failure
- No function context or source code
- Extremely concise format
- Shows only file:line and error message
- Best for parsing programmatically when you just need failure location

**Assertion Types Covered:** 15 different patterns

### 4. `sample4_tb_no.txt`
**Command:** `pytest --tb=no tools/test_pytest_flags_minimal.py`

**Format Characteristics:**
- No traceback: Shows only summary
- Minimal output - just test session info and failure summary
- No assertion details or error messages
- Useful for quick pass/fail checks

**Assertion Types Covered:** 15 different patterns

### 5. `sample5_v_tb_auto.txt`
**Command:** `pytest -v --tb=auto tools/test_pytest_flags_minimal.py`

**Format Characteristics:**
- Auto traceback: Default pytest behavior
- Full traceback for first failure, truncated for subsequent similar failures
- Balances detail with verbosity
- "Full output truncated" messages for long diffs

**Assertion Types Covered:** 15 different patterns

### 6. `sample3_json_report.txt`
**Command:** `pytest --json-report --json-report-file=test_report.json tools/test_pytest_flags_minimal.py`

**Format Characteristics:**
- JSON output format (requires pytest-json-report plugin)
- Machine-readable structured data
- Plugin not installed in this environment (shows error)

## Assertion Patterns Covered

All samples demonstrate these 15 distinct assertion failure patterns:

1. **Simple Equality** - `assert greeting == "world"`
   - Basic string/number comparison with custom message

2. **Dictionary Equality** - `assert actual == expected`
   - Shows "Differing items" with full diff
   - Key-by-key mismatch highlighting

3. **List Comparison** - `assert list1 == list2`
   - Index-based diffs ("At index 2 diff: 3 != 10")
   - Full diff with line numbers

4. **Multiline String** - `assert actual_text == expected_text`
   - Character-level diffs with `?` markers
   - Shows whitespace and position differences

5. **Numeric Comparison** - `assert x == y`
   - Simple number mismatch

6. **In Operator** - `assert "orange" in items`
   - Membership test failure
   - Shows the collection being tested

7. **Long Sequence** - `assert seq1 == seq2`
   - Same as list comparison but with longer sequences
   - Tests diff formatting for larger collections

8. **Nested Structure** - `assert data1 == data2`
   - Deeply nested dictionaries and lists
   - Traverses hierarchy to find mismatch

9. **Approximate Numbers** - `assert result == expected`
   - Floating-point precision issues
   - Shows 0.30000000000000004 vs 0.3

10. **Boolean Logic** - `assert x and y`
    - Shows evaluated expression: `assert (True and False)`

11. **String Operations** - `assert text.upper() == "HELLO"`
    - Method call results in assertion
    - Shows actual output vs expected

12. **Type Checking** - `assert isinstance(value, int)`
    - Shows "where False = isinstance('123', int)"
    - Explains why assertion failed

13. **Set Operations** - `assert set1 == set2`
    - "Extra items in the left set" / "Extra items in the right set"
    - Symmetric difference reporting

14. **Range Comparison** - `assert range(0, 10) == range(0, 11)`
    - "Right contains one more item: 10"
    - Specialized range diff format

15. **Tuple Comparison** - `assert t1 == t2`
    - Same as list but with parentheses
    - Index-based diffs

## Format Variations Summary

| Format | Verbosity | Traceback | Diff Detail | Parseability |
|--------|-----------|-----------|-------------|--------------|
| `-vv --tb=short` | High | Short | Complete | Medium |
| `-v --tb=long` | Medium | Long | Truncated | Low |
| `--tb=line` | Low | Line | None | High |
| `--tb=no` | Low | None | None | High |
| `-v --tb=auto` | Medium | Auto | Truncated | Medium |

## Machine Parseability Notes

**Easiest to parse:**
1. `--tb=line` - Consistent single-line format per failure
2. `--tb=no` - Just summary counts
3. JSON report (if plugin available)

**Hardest to parse:**
1. `--tb=long` - Multi-line stack traces with variable depth
2. `--tb=auto` - Inconsistent formatting between failures

**Key parsing patterns:**
- Failure lines start with `FAILED ` or show `=== FAILURES ===` section
- Assertion lines show `assert ` with expected/actual
- Diff lines show `- expected` and `+ actual`
- File locations follow format: `path/to/file.py:line: error`

## Source Test File

All samples generated from: `/home/coding/ARMOR/tools/test_pytest_flags_minimal.py`

This test file contains 15 intentionally failing tests covering common assertion patterns.

## Generated

2026-07-13 for ARMOR project (bead: bf-1ym8jw)
