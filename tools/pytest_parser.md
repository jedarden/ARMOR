# Pytest Output Parsing Patterns

This document defines the regex patterns and structural markers for parsing pytest failure output across different format variations. Designed for ARMOR's test failure analysis system.

## Overview

Pytest output has three main parseable sections:
1. **Test execution section** - Test names and pass/fail status
2. **Failure details section** - Assertion details with diffs
3. **Summary section** - Short test summary info

## Section 1: Test Execution Parsing

### Format Variations

#### Verbose format (`-v`, `-vv`)
```
tools/test_pytest_flags_minimal.py::test_simple_equality FAILED          [  6%]
tools/test_pytest_flags_minimal.py::test_dict_equality FAILED            [ 13%]
```

#### Concise format (no `-v`)
```
tools/test_pytest_flags_minimal.py FFFFFFFFFFFFFFF                       [100%]
```

### Regex Pattern

```python
# Verbose format: file::test_name STATUS
VERBOSE_TEST_PATTERN = r'^\s*(.+?)::(\w+)\s+(FAILED|PASSED|ERROR|SKIPPED)\s+\[\s*\d+%?\]'

# Concise format: file F/P/E/S marks
CONCISE_TEST_PATTERN = r'^\s*(.+?)\s+([FPE\.]+)\s+\[\s*\d+%?\]'
```

### Capture Groups

| Group | Content | Example |
|-------|---------|---------|
| 1 | File path | `tools/test_pytest_flags_minimal.py` |
| 2 | Test name | `test_simple_equality` |
| 3 | Status | `FAILED`, `PASSED`, etc. |

## Section 2: Failure Location Parsing

### Format Variations

#### `--tb=short` (Recommended)
```
tools/test_pytest_flags_minimal.py:17: in test_simple_equality
    assert greeting == "world", f"Expected 'world', got '{greeting}'"
```

#### `--tb=line` (Best for parsing)
```
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17: AssertionError: Expected 'world', got 'hello'
```

#### `--tb=long`
```
tools/test_pytest_flags_minimal.py:17: AssertionError
```

### Regex Patterns

```python
# Short format: file:line: in test_name
SHORT_FAILURE_PATTERN = r'^\s*(.+?):(\d+):\s+in\s+(\w+)'

# Line format: file:line: AssertionError: message
LINE_FAILURE_PATTERN = r'^\s*(.+?):(\d+):\s+AssertionError:\s*(.+)'

# Generic file location (any format)
FILE_LOCATION_PATTERN = r'^\s*(.+?):(\d+):'
```

### Capture Groups

| Group | Content | Example |
|-------|---------|---------|
| 1 | File path | `tools/test_pytest_flags_minimal.py` |
| 2 | Line number | `17` |
| 3 | Test name or message | `test_simple_equality` or `Expected 'world', got 'hello'` |

## Section 3: Assertion Type Detection

### Common Assertion Patterns

```python
# Equality assertion
ASSERT_PATTERN = r'^E?\s+assert\s+(.+)'

# Pattern: assert X == Y
EQUALITY_PATTERN = r'^E?\s+assert\s+(.+?)\s+==\s+(.+)'

# Pattern: assert X in Y
CONTAINS_PATTERN = r'^E?\s+assert\s+(.+?)\s+in\s+(.+)'

# Pattern: assert isinstance(X, Y)
TYPE_CHECK_PATTERN = r'^E?\s+assert\s+isinstance\((.+?),\s*(.+?)\)'
```

### Assertion Type Classification

| Pattern | Type | Example |
|---------|------|---------|
| `assert X == Y` | Equality comparison | `assert greeting == "world"` |
| `assert X in Y` | Membership test | `assert "orange" in items` |
| `assert isinstance(X, Y)` | Type check | `assert isinstance(value, int)` |
| `assert X and Y` | Boolean logic | `assert x and y` |
| `assert False` | Complex expression | `assert False` (with `where` clause) |

## Section 4: Diff Pattern Parsing

### Expected/Actual Markers

Pytest uses `-` for expected values and `+` for actual values.

```python
# Diff line markers
DIFF_MINUS_PATTERN = r'^\s*-\s*(.+)'
DIFF_PLUS_PATTERN = r'^\s*\+\s*(.+)'

# Character position markers (for multiline strings)
DIFF_POSITION_PATTERN = r'^\s*\?\s+(.+)'
```

### Index-based Diffs (Lists, Tuples)

```python
# Pattern: At index N diff: X != Y
INDEX_DIFF_PATTERN = r'^\s+At\s+index\s+(\d+)\s+diff:\s+(.+?)\s+!=\s+(.+)'
```

### Dictionary Diffs

```python
# Pattern: Differing items
DICT_DIFF_HEADER = r'^\s+Differing items:'
DICT_DIFF_LINE = r'^\s+\{(.+?)\}\s+!=\s+\{(.+?)\}'
```

### Set Diffs

```python
# Pattern: Extra items in left/right set
SET_DIFF_LEFT = r'^\s+Extra items in the left set:'
SET_DIFF_RIGHT = r'^\s+Extra items in the right set:'
```

### Range Diffs

```python
# Pattern: Right contains one more item: X
RANGE_DIFF_PATTERN = r'^\s+(Right|Left) contains (?:one more|\d+) more items?:\s*(.+)'
```

### Where Clauses (Type Checks)

```python
# Pattern: + where False = isinstance(...)
WHERE_CLAUSE_PATTERN = r'^\s*\+\s+where\s+(.+?)\s+=\s+(.+)'
```

## Section 5: Summary Section Parsing

### Short Test Summary Format

```
FAILED tools/test_pytest_flags_minimal.py::test_simple_equality - AssertionError: Expected 'world', got 'hello'
```

```python
SUMMARY_PATTERN = r'^(FAILED|PASSED|ERROR|SKIPPED)\s+(.+?)::(\w+)\s+-\s*(.+)'
```

### Final Count Line

```
============================== 15 failed in 0.05s ==============================
```

```python
COUNT_PATTERN = r'=\s+(\d+)\s+(failed|passed|error|skipped)\s+in\s+([\d.]+)s\s+='
```

## Section 6: Recommended Parsing Strategy

### For Quick Status Checks

Use `--tb=no` format:
```bash
pytest --tb=no test_file.py
```

Parse only the summary line with `COUNT_PATTERN`.

### For Detailed Failure Location

Use `--tb=line` format (most parseable):
```bash
pytest --tb=line test_file.py
```

Parse with `LINE_FAILURE_PATTERN` for exact file:line:error message.

### For Full Assertion Details

Use `-vv --tb=short` format:
```bash
pytest -vv --tb=short test_file.py
```

Parse in sequence:
1. `SHORT_FAILURE_PATTERN` for file:line:test_name
2. `ASSERT_PATTERN` for assertion expression
3. `EQUALITY_PATTERN`, `CONTAINS_PATTERN`, etc. for assertion type
4. Diff patterns (`DIFF_MINUS_PATTERN`, `DIFF_PLUS_PATTERN`) for expected/actual values

### For Machine-Readable Output

Consider pytest-json-report plugin (if available):
```bash
pytest --json-report --json-report-file=report.json test_file.py
```

## Section 7: Example Parser Implementation

```python
import re
from dataclasses import dataclass
from typing import Optional, List

@dataclass
class TestFailure:
    file: str
    line: int
    test_name: Optional[str]
    error_message: str
    assertion_type: Optional[str]
    expected: Optional[str]
    actual: Optional[str]
    
    @classmethod
    def from_line_format(cls, line: str) -> Optional['TestFailure']:
        """Parse from --tb=line format."""
        match = re.match(LINE_FAILURE_PATTERN, line)
        if not match:
            return None
        
        file, line_num, message = match.groups()
        return cls(
            file=file,
            line=int(line_num),
            test_name=None,
            error_message=message,
            assertion_type=None,
            expected=None,
            actual=None
        )
    
    @classmethod
    def from_short_format(cls, lines: List[str]) -> Optional['TestFailure']:
        """Parse from --tb=short format."""
        location = None
        assertion = None
        expected = None
        actual = None
        
        for line in lines:
            # Parse location line
            loc_match = re.match(SHORT_FAILURE_PATTERN, line)
            if loc_match:
                file, line_num, test_name = loc_match.groups()
                location = (file, int(line_num), test_name)
                continue
            
            # Parse assertion
            assert_match = re.match(ASSERT_PATTERN, line)
            if assert_match:
                assertion = assert_match.group(1)
                continue
            
            # Parse expected
            minus_match = re.match(DIFF_MINUS_PATTERN, line)
            if minus_match and not expected:
                expected = minus_match.group(1)
                continue
            
            # Parse actual
            plus_match = re.match(DIFF_PLUS_PATTERN, line)
            if plus_match and not actual:
                actual = plus_match.group(1)
        
        if not location:
            return None
        
        return cls(
            file=location[0],
            line=location[1],
            test_name=location[2],
            error_message=assertion or "",
            assertion_type=cls._classify_assertion(assertion),
            expected=expected,
            actual=actual
        )
    
    @staticmethod
    def _classify_assertion(assertion: Optional[str]) -> Optional[str]:
        """Classify assertion type."""
        if not assertion:
            return None
        
        if ' == ' in assertion:
            return 'equality'
        elif ' in ' in assertion:
            return 'contains'
        elif 'isinstance' in assertion:
            return 'type_check'
        elif ' and ' in assertion or ' or ' in assertion:
            return 'boolean'
        else:
            return 'unknown'
```

## Section 8: Edge Cases and Gotchas

### Truncated Output

When using `-v` without `-vv`, pytest truncates diffs:
```
...Full output truncated (14 lines hidden), use '-vv' to show
```

**Pattern:** `r'^\s+\.\.\.Full output truncated\s+\((\d+) lines hidden\)'`

### Ellipsis in Output

Pytest uses `...` to indicate truncated sections:
```
assert {'age': 25, '...name': 'Bob'} == {'age': 30, '...ame': 'Alice'}
```

**Pattern:** `r'\.\.\.'` (three consecutive dots in values)

### Multiline String Escapes

Multiline strings may show escaped characters:
```
assert '\n    This i...ontent.\n    ' == '\n    This i... lines.\n    '
```

**Pattern:** Look for `\n` and other escape sequences.

### Platform-Specific Paths

Paths may use different separators on different platforms:
- Unix: `/home/coding/ARMOR/tools/test_file.py`
- Windows: `C:\Users\coding\ARMOR\tools\test_file.py`

**Solution:** Use `os.path.normpath()` for consistent path handling.

## Section 9: Regex Pattern Reference

### Core Patterns

```python
# File location (any format)
FILE_LOCATION_PATTERN = r'^(.+?):(\d+):'

# Test execution (verbose)
VERBOSE_TEST_PATTERN = r'^\s*(.+?)::(\w+)\s+(FAILED|PASSED|ERROR|SKIPPED)\s+\[\s*\d+%?\]'

# Test execution (concise)
CONCISE_TEST_PATTERN = r'^\s*(.+?)\s+([FPE\.]+)\s+\[\s*\d+%?\]'

# Failure (short format)
SHORT_FAILURE_PATTERN = r'^\s*(.+?):(\d+):\s+in\s+(\w+)'

# Failure (line format)
LINE_FAILURE_PATTERN = r'^\s*(.+?):(\d+):\s+AssertionError:\s*(.+)'

# Summary
SUMMARY_PATTERN = r'^(FAILED|PASSED|ERROR|SKIPPED)\s+(.+?)::(\w+)\s+-\s*(.+)'

# Count
COUNT_PATTERN = r'=\s+(\d+)\s+(failed|passed|error|skipped)\s+in\s+([\d.]+)s\s+='
```

### Assertion Patterns

```python
ASSERT_PATTERN = r'^E?\s+assert\s+(.+)'
EQUALITY_PATTERN = r'^E?\s+assert\s+(.+?)\s+==\s+(.+)'
CONTAINS_PATTERN = r'^E?\s+assert\s+(.+?)\s+in\s+(.+)'
TYPE_CHECK_PATTERN = r'^E?\s+assert\s+isinstance\((.+?),\s*(.+?)\)'
```

### Diff Patterns

```python
DIFF_MINUS_PATTERN = r'^\s*-\s*(.+)'
DIFF_PLUS_PATTERN = r'^\s*\+\s*(.+)'
DIFF_POSITION_PATTERN = r'^\s*\?\s+(.+)'
INDEX_DIFF_PATTERN = r'^\s+At\s+index\s+(\d+)\s+diff:\s+(.+?)\s+!=\s+(.+)'
DICT_DIFF_HEADER = r'^\s+Differing items:'
SET_DIFF_LEFT = r'^\s+Extra items in the left set:'
SET_DIFF_RIGHT = r'^\s+Extra items in the right set:'
RANGE_DIFF_PATTERN = r'^\s+(Right|Left) contains (?:one more|\d+) more items?:\s*(.+)'
WHERE_CLAUSE_PATTERN = r'^\s*\+\s+where\s+(.+?)\s+=\s+(.+)'
```

### Section Markers

```python
FAILURES_SECTION = r'^===+ FAILURES ===+'
SUMMARY_SECTION = r'^===+ short test summary info ===+'
SESSION_START = r'^===+ test session starts ===+'
SESSION_END = r'^===+ \d+ (?:failed|passed) ===+'
```

## Appendix: Quick Reference

### Parse Priority by Format

1. **Easiest** - `--tb=line`: One line per failure with full error message
2. **Easy** - `--tb=no`: Just summary, use for count only
3. **Medium** - `-vv --tb=short`: Full details but multi-line per failure
4. **Hard** - `--tb=long`: Variable stack depth, complex parsing
5. **Variable** - `--tb=auto`: Inconsistent between failures

### Recommended Format for ARMOR

For ARMOR's test failure analysis, use:
```bash
pytest -vv --tb=short --tb=line test_file.py
```

This provides:
- Detailed assertion rewriting for value extraction (`-vv`)
- Parseable failure locations (`--tb=line`)
- Complete diffs without truncation

### Test Coverage

All patterns tested against 15 assertion types:
1. ✅ Simple equality
2. ✅ Dictionary equality  
3. ✅ List comparison
4. ✅ Multiline strings
5. ✅ Numeric comparison
6. ✅ Membership testing
7. ✅ Long sequences
8. ✅ Nested structures
9. ✅ Floating-point comparison
10. ✅ Boolean logic
11. ✅ String operations
12. ✅ Type checking
13. ✅ Set operations
14. ✅ Range comparison
15. ✅ Tuple comparison

---

**Generated:** 2026-07-13 for ARMOR project (bead: bf-63vue2)  
**Parent Task:** bf-1ym8jw (Pytest Failure Output Samples)
