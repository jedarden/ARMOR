# Pytest Output Format Patterns

**Task:** bf-5x5xz1 (Research and document pytest output formats)  
**Date:** 2026-07-14  
**Status:** ✅ Complete

## Overview

This document analyzes the three primary pytest output formats collected from actual test runs and documents the exact patterns for extracting file paths, line numbers, expected values, and actual values.

## The Three Sample Formats

Based on the parent bead (bf-1ym8jw) sample collection, pytest produces three distinct output formats depending on flags:

### Format 1: Full Detailed Format (`-vv --tb=short`)

**Flags:** `pytest -vv --tb=short`  
**Characteristics:** Complete diffs, multi-line assertions, detailed context

#### Sample Output

```
_____________________________ test_simple_equality _____________________________
tools/test_pytest_flags_minimal.py:17: in test_simple_equality
    assert greeting == "world", f"Expected 'world', got '{greeting}'"
E   AssertionError: Expected 'world', got 'hello'
E   assert 'hello' == 'world'
E     
E     - world
E     + hello
______________________________ test_dict_equality ______________________________
tools/test_pytest_flags_minimal.py:24: in test_dict_equality
    assert actual == expected, f"Dictionaries don't match"
E   AssertionError: Dictionaries don't match
E   assert {'name': 'Bob', 'age': 25, 'city': 'LA'} == {'name': 'Alice', 'age': 30, 'city': 'NYC'}
E     
E     Differing items:
E     {'city': 'LA'} != {'city': 'NYC'}
E     {'age': 25} != {'age': 30}
E     {'name': 'Bob'} != {'name': 'Alice'}
E     
E     Full diff:
E       {
E     -     'age': 30,
E     ?            ^^
E     +     'age': 25,
E     ?            ^^
E     -     'city': 'NYC',
E     ?              ^^^
E     +     'city': 'LA',
E     ?              ^^
E     -     'name': 'Alice',
E     ?              ^^^^^
E     +     'name': 'Bob',
E     ?              ^^^
E       }
_____________________________ test_list_comparison _____________________________
tools/test_pytest_flags_minimal.py:31: in test_list_comparison
    assert list1 == list2, "Lists should be equal"
E   AssertionError: Lists should be equal
E   assert [1, 2, 3, 4, 5] == [1, 2, 10, 4, 5]
E     
E     At index 2 diff: 3 != 10
E     
E     Full diff:
E       [
E           1,
E           2,
E     -     10,
E     ?     ^^
E     +     3,
E     ?     ^
E           4,
E           5,
E       ]
```

### Format 2: Long Format with Code Context (`-v --tb=long`)

**Flags:** `pytest -v --tb=long`  
**Characteristics:** Shows source code context, truncates diffs with `-v` (needs `-vv` for complete diffs)

#### Sample Output

```
_____________________________ test_simple_equality _____________________________

    def test_simple_equality():
        """Simple equality assertion failure."""
        greeting = "hello"
>       assert greeting == "world", f"Expected 'world', got '{greeting}'"
E       AssertionError: Expected 'world', got 'hello'
E       assert 'hello' == 'world'
E         
E         - world
E         + hello

tools/test_pytest_flags_minimal.py:17: AssertionError
______________________________ test_dict_equality ______________________________

    def test_dict_equality():
        """Dictionary equality assertion failure."""
        expected = {"name": "Alice", "age": 30, "city": "NYC"}
        actual = {"name": "Bob", "age": 25, "city": "LA"}
>       assert actual == expected, f"Dictionaries don't match"
E       AssertionError: Dictionaries don't match
E       assert {'age': 25, '...'name': 'Bob'} == {'age': 30, '...ame': 'Alice'}
E         
E         Differing items:
E         {'city': 'LA'} != {'city': 'NYC'}
E         {'age': 25} != {'age': 30}
E         {'name': 'Bob'} != {'name': 'Alice'}
E         
E         Full diff:...
E         
E         ...Full output truncated (14 lines hidden), use '-vv' to show

tools/test_pytest_flags_minimal.py:24: AssertionError
_____________________________ test_list_comparison _____________________________

    def test_list_comparison():
        """List comparison with index diff."""
        list1 = [1, 2, 3, 4, 5]
        list2 = [1, 2, 10, 4, 5]
>       assert list1 == list2, "Lists should be equal"
E       AssertionError: Lists should be equal
E       assert [1, 2, 3, 4, 5] == [1, 2, 10, 4, 5]
E         
E         At index 2 diff: 3 != 10
E         
E         Full diff:
E           [
E               1,
E               2,...
E         
E         ...Full output truncated (7 lines hidden), use '-vv' to show

tools/test_pytest_flags_minimal.py:31: AssertionError
```

### Format 3: Single Line Format (`--tb=line`)

**Flags:** `pytest --tb=line`  
**Characteristics:** One line per failure, most parseable, minimal context

#### Sample Output

```
=================================== FAILURES ===================================
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17: AssertionError: Expected 'world', got 'hello'
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:24: AssertionError: Dictionaries don't match
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:31: AssertionError: Lists should be equal
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:44: AssertionError: assert '\n    This i...ontent.\n    ' == '\n    This i... lines.\n    '
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:51: AssertionError: Numbers don't match: 5 != 10
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:57: AssertionError: orange should be in the list
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:64: AssertionError: Sequences should match
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:81: AssertionError: Nested structures should match
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:89: AssertionError: Floats don't match: 0.30000000000000004 != 0.3
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:96: AssertionError: Both x and y should be True
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:102: AssertionError: Uppercase conversion failed
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:108: AssertionError: Value should be int, got str
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:115: AssertionError: Sets should be equal
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:122: AssertionError: Ranges should be equal
/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:129: AssertionError: Tuples should be equal
```

## Common Patterns Across All Formats

### 1. File Location Pattern

**Regex:** `r'^(.+?):(\d+):'`

**Extracts:**
- Group 1: File path (e.g., `tools/test_pytest_flags_minimal.py` or `/home/coding/ARMOR/tools/test_pytest_flags_minimal.py`)
- Group 2: Line number (e.g., `17`, `24`, `31`)

**Works in all three formats:**
- Format 1: `tools/test_pytest_flags_minimal.py:17:`
- Format 2: `tools/test_pytest_flags_minimal.py:17:`
- Format 3: `/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17:`

### 2. Test Name Pattern (Formats 1 & 2)

**Regex:** `r'in\s+(\w+)'`

**Extracts:**
- Group 1: Test function name (e.g., `test_simple_equality`, `test_dict_equality`)

**Works in:**
- Format 1: `tools/test_pytest_flags_minimal.py:17: in test_simple_equality`
- Format 2: `tools/test_pytest_flags_minimal.py:17: in test_simple_equality`
- Format 3: Not available

### 3. Error Message Pattern (Format 3)

**Regex:** `r'AssertionError:\s*(.+)'`

**Extracts:**
- Group 1: Full error message (e.g., `Expected 'world', got 'hello'`)

**Works in:**
- Format 3: `/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17: AssertionError: Expected 'world', got 'hello'`
- Formats 1 & 2: `E   AssertionError: Expected 'world', got 'hello'` (needs `E   ` prefix handling)

## Assertion-Specific Patterns

### Equality Assertion Pattern

**Regex:** `r'assert\s+(.+?)\s+==\s+(.+)'`

**Extracts:**
- Group 1: Actual value (left side)
- Group 2: Expected value (right side)

**Examples:**
- `assert 'hello' == 'world'` → actual=`'hello'`, expected=`'world'`
- `assert 5 == 10` → actual=`5`, expected=`10`
- `assert [1, 2, 3] == [1, 2, 10]` → actual=`[1, 2, 3]`, expected=`[1, 2, 10]`

### Index Diff Pattern

**Regex:** `r'At\s+index\s+(\d+)\s+diff:\s+(.+?)\s+!=\s+(.+)'`

**Extracts:**
- Group 1: Index position (e.g., `2`, `5`)
- Group 2: Actual value at index
- Group 3: Expected value at index

**Examples:**
- `At index 2 diff: 3 != 10` → index=`2`, actual=`3`, expected=`10`
- `At index 5 diff: 10 != 5` → index=`5`, actual=`10`, expected=`5`

### Differing Items Pattern

**Regex:** `r'\{(.+?)\}\s+!=\s+\{(.+?)\}'`

**Extracts:**
- Group 1: Left side (actual) key-value
- Group 2: Right side (expected) key-value

**Examples:**
- `{'city': 'LA'} != {'city': 'NYC'}` → actual=`{'city': 'LA'}`, expected=`{'city': 'NYC'}`
- `{'age': 25} != {'age': 30}` → actual=`{'age': 25}`, expected=`{'age': 30}`

### Set Operations Pattern

**Left set regex:** `r'Extra items in the left set:'`  
**Right set regex:** `r'Extra items in the right set:'`

**Followed by values on subsequent lines:**
- `4` (in left set)
- `5` (in right set)

### Range Diff Pattern

**Regex:** `r'(Right|Left) contains (?:one more item|\d+ more items?)\s*:\s*(.+)'`

**Extracts:**
- Group 1: Side (Right/Left)
- Group 2: Extra item value

**Examples:**
- `Right contains one more item: 10` → side=`Right`, item=`10`
- `Left contains 2 more items: 3, 4` → side=`Left`, items=`3, 4`

### Where Clause Pattern (Type Checks)

**Regex:** `r'where\s+(.+?)\s+=\s+(.+)'`

**Extracts:**
- Group 1: Expression (e.g., `False`)
- Group 2: Computation (e.g., `isinstance('123', int)`)

**Examples:**
- `where False = isinstance('123', int)` → expr=`False`, computation=`isinstance('123', int)`

## Expected/Actual Extraction from Diffs

### Diff Line Markers

**Expected (minus) pattern:** `r'^\s*-\s*(.+)'`  
**Actual (plus) pattern:** `r'^\s*\+\s*(.+)'`

**Extracts:**
- Group 1: The value on that line

**Examples:**
- `    - world` → `world`
- `    + hello` → `hello`
- `  -     'age': 30,` → `'age': 30,`
- `  +     'age': 25,` → `'age': 25,`

### Position Markers

**Regex:** `r'^\s*\?\s+(.+)'`

**Extracts:**
- Group 1: Position indicator (shows which characters differ)

**Examples:**
- `?            ^^` → positions 2-3 differ
- `?              ^^^` → positions 3-5 differ

## Edge Cases and Format Variations

### 1. Truncated Output

**Pattern:** `r'\.\.\.Full output truncated\s+\((\d+) lines hidden\)'`  
**Occurs in:** Format 2 when using `-v` without `-vv`  
**Example:** `...Full output truncated (14 lines hidden), use '-vv' to show`

### 2. Ellipsis in Values

**Pattern:** Three consecutive dots in truncated values  
**Occurs in:** Format 2 and Format 3 for long values  
**Examples:**
- `assert '\n    This i...ontent.\n    ' == '\n    This i... lines.\n    '`
- `assert [0, 1, 2, 3, 4, 10, ...] == [0, 1, 2, 3, 4, 5, ...]`

### 3. Multiline String Escapes

**Pattern:** Escape sequences like `\n`, `\t` in string diffs  
**Occurs in:** All formats  
**Example:** `assert '\n    This i...ontent.\n    ' == '\n    This i... lines.\n    '`

### 4. Path Format Variations

**Relative paths** (Formats 1 & 2): `tools/test_pytest_flags_minimal.py:17`  
**Absolute paths** (Format 3): `/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17`

**Solution:** Use `os.path.normpath()` for consistent path handling.

### 5. Whitespace Variations

**Format 1 & 2:** `E   ` prefix (3 spaces after E)  
**Format 3:** No prefix, direct text

**Pattern variations:**
- `E   AssertionError:` (Format 1 & 2)
- `AssertionError:` (Format 3)
- `E   assert` (Format 1 & 2)
- `assert` (implied in Format 3)

### 6. Floating-Point Precision

**Pattern:** Numbers like `0.30000000000000004` vs `0.3`  
**Occurs in:** All formats  
**Example:** `AssertionError: Floats don't match: 0.30000000000000004 != 0.3`

### 7. Boolean Logic

**Pattern:** Complex expressions without explicit expected/actual  
**Example:** `assert (True and False)`  
**Note:** No diff shown, just the assertion

### 8. Nested Structure Truncation

**Pattern:** Deep nesting gets truncated in Format 2  
**Example:** `assert {'users': [{'...92, 87, 91]}]} == {'users': [{'...92, 99, 91]}]}`  
**Note:** Use `-vv` for complete nested diffs

## Recommended Parsing Strategy by Format

### Format 1: Full Detailed (`-vv --tb=short`)

**Parse order:**
1. Extract file:line using `r'^(.+?):(\d+):'`
2. Extract test name using `r'in\s+(\w+)'`
3. Extract assertion using `r'assert\s+(.+)'`
4. Classify assertion type (equality, contains, isinstance, etc.)
5. Extract diff lines using `r'^\s*[-+]\s*(.+)'`
6. Parse expected from `-` lines, actual from `+` lines

### Format 2: Long Context (`-v --tb=long`)

**Parse order:**
1. Extract file:line using `r'^(.+?):(\d+):'`
2. Extract test name using `r'in\s+(\w+)'`
3. Look for `>` marker to find failing line
4. Parse assertion from line with `>`
5. Handle truncated output warnings
6. Extract available diffs (may be incomplete)

### Format 3: Single Line (`--tb=line`)

**Parse order:**
1. Extract file:line using `r'^(.+?):(\d+):'`
2. Extract error message using `r'AssertionError:\s*(.+)'`
3. Parse expected/actual from message if available (e.g., "Expected X, got Y")
4. No test name available in this format

## Summary Table

| Field | Format 1 | Format 2 | Format 3 | Pattern |
|-------|----------|----------|----------|---------|
| File path | ✅ Yes | ✅ Yes | ✅ Yes | `r'^(.+?):(\d+):'` |
| Line number | ✅ Yes | ✅ Yes | ✅ Yes | Same as above |
| Test name | ✅ Yes | ✅ Yes | ❌ No | `r'in\s+(\w+)'` |
| Error message | ✅ Yes | ✅ Yes | ✅ Yes | `r'AssertionError:\s*(.+)'` |
| Expected value | ✅ Yes | ⚠️ Partial | ❌ No | From `-` diff lines |
| Actual value | ✅ Yes | ⚠️ Partial | ❌ No | From `+` diff lines |
| Index diffs | ✅ Yes | ✅ Yes | ❌ No | `r'At\s+index\s+(\d+)'` |
| Dict diffs | ✅ Yes | ✅ Yes | ❌ No | `r'Differing items:'` |
| Complete diffs | ✅ Yes | ⚠️ Truncated | ❌ No | N/A |

## Conclusion

The three formats serve different purposes:

- **Format 1 (`-vv --tb=short`)**: Best for detailed value extraction and debugging
- **Format 2 (`-v --tb=long`)**: Good for understanding code context, but diffs are truncated
- **Format 3 (`--tb=line`)**: Best for quick status checks and failure counting

For ARMOR's test failure analysis system, **Format 1** provides the most complete information for extracting file paths, line numbers, expected values, and actual values.

---

**Generated:** 2026-07-14 for ARMOR project (bead: bf-5x5xz1)  
**Parent Task:** bf-1ym8jw (Collect sample pytest failure outputs)  
**Source Samples:** Collected from actual pytest runs on test_pytest_flags_minimal.py
