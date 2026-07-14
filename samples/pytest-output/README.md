# Pytest Output Examples

This collection contains comprehensive pytest output examples covering various failure scenarios to help with parser development and testing.

## File Organization

Each test file (`test_*.py`) has a corresponding output file (`output_*.txt`) containing the captured pytest output.

## Categories

### 1. Simple Assertions (`test_simple_assertions.py` → `output_simple_assertions.txt`)

**Failure Types:**
- **Equality assertion failure** (`test_simple_assertion_equal`): Shows basic `==` comparison with custom message
- **Truth assertion failure** (`test_simple_assertion_true`): Demonstrates assert with boolean value
- **Membership assertion failure** (`test_simple_assertion_in`): Shows `in` operator failure with list
- **Identity assertion failure** (`test_simple_assertion_is`): Demonstrates `is` vs `==` distinction
- **Type assertion failure** (`test_simple_assertion_type`): Shows `isinstance()` failure with type information
- **Exception not raised** (`test_simple_assertion_raises`): Shows "DID NOT RAISE" message pattern

**Key Parser Considerations:**
- Custom assertion messages appear before the assert line
- Shows both the high-level message and the raw assertion
- Type information is displayed with full class path

---

### 2. Comparison Failures (`test_comparison_failures.py` → `output_comparison_failures.txt`)

**Failure Types:**
- **Numeric comparison** (`test_numeric_comparison`): `>=` failure with numeric values
- **Range check failure** (`test_numeric_range`): Chained comparison `0 <= value <= 100`
- **String comparison** (`test_string_comparison`): Shows diff-style output for strings
- **Floating-point comparison** (`test_float_comparison`): Demonstrates precision issues (0.30000000000000004 vs 0.3)
- **Length comparison** (`test_length_comparison`): Shows function call in assertion (`len(items)`)
- **Dictionary key existence** (`test_dictionary_key_comparison`): `in` operator with dict

**Key Parser Considerations:**
- String comparisons show unified diff format (`-` for expected, `+` for actual)
- Function calls in assertions show "where" clauses explaining intermediate values
- Float comparisons can show many decimal places

---

### 3. Multiline Diffs (`test_multiline_diffs.py` → `output_multiline_diffs.txt`)

**Failure Types:**
- **Multiline string diff** (`test_multiline_string_diff`): Shows line-by-line text comparison
- **List comparison** (`test_list_comparison`): Array element differences with index
- **Dictionary comparison** (`test_dict_comparison`): Key-value differences with omitted identical items
- **Nested structure diff** (`test_nested_structure_diff`): Complex nested dict/list structures
- **Long list diff** (`test_long_list_diff`): Truncated output with "use '-vv' to show" message

**Key Parser Considerations:**
- Multiline diffs use unified diff format with `-`/`+` prefixes
- List diffs show "At index N diff: X != Y" to pinpoint location
- Dict diffs show "Omitting N identical items" for cleaner output
- Large outputs get truncated with instructions to show full diff
- Nested structures can produce very deep, hard-to-read diffs

---

### 4. Exception Failures (`test_exception_failures.py` → `output_exception_failures.txt`)

**Failure Types:**
- **Unexpected exception** (`test_unexpected_exception`): ZeroDivisionError with full traceback
- **Wrong exception type** (`test_wrong_exception_type`): ValueError raised but TypeError expected
- **Exception message mismatch** (`test_exception_message_mismatch`): Shows "Regex pattern did not match"
- **Exception not raised** (`test_exception_not_raised`): "DID NOT RAISE" failure pattern

**Key Parser Considerations:**
- Tracebacks show the full call stack from test down to failure point
- Exception messages include both "Regex pattern" and "Input" for mismatch cases
- Some tests pass (2 passed) - parser must handle mixed pass/fail results
- Exception type mismatches show the actual exception raised

---

### 5. Edge Cases (`test_edge_cases.py` → `output_edge_cases.txt`)

**Failure Types:**
- **None vs False** (`test_none_vs_false`): Identity comparison with None
- **Empty vs None** (`test_empty_vs_none`): Empty string vs None distinction
- **Falsy values** (`test_falsy_values`): Various falsy comparisons (`[] == None`, `0 == False`)
- **Object equality vs identity** (`test_object_equality_vs_identity`): Custom class instances with memory addresses
- **Custom repr failure** (`test_custom_repr_failure`): Shows object's `__repr__` in output
- **Recursion limit** (`test_recursion_limit`): Deep nested structures
- **Unicode string** (`test_unicode_string`): Unicode normalization issues (café vs café)
- **Bytes vs string** (`test_bytes_vs_str`): Type mismatch between bytes and str

**Key Parser Considerations:**
- Memory addresses appear in object identity failures (`0x7f60abcad640`)
- Unicode can show both escaped (`\xe9`) and combined forms (`́`)
- Type mismatches are explicit (`b'hello' == 'hello'`)
- Deep nesting causes significant output truncation
- Custom `__repr__` methods appear in assertions

---

## Full Multiline Output Examples

For more detailed diffs, run with `-vv` flag:

```bash
pytest test_multiline_diffs.py -vv --tb=short
```

This produces expanded diffs without truncation, useful for seeing complete differences in large structures.

## Traceback Styles

The examples use `--tb=short` (short traceback). Other styles:
- `--tb=long`: Full tracebacks with local variable values
- `--tb=line`: Single line per failure
- `--tb=no`: No traceback, just summary

## Parser Implementation Notes

When implementing a pytest output parser:

1. **Handle multiple traceback formats** - pytest can show short, long, line, or no tracebacks
2. **Parse diff markers** - Look for `-` (expected/removed) and `+` (actual/added) prefixes
3. **Extract "where" clauses** - Lines starting with `E    +  where` explain intermediate values
4. **Handle truncation** - "Full output truncated" and "use '-vv' to show" messages
5. **Parse omission notices** - "Omitting N identical items" in dict comparisons
6. **Handle unicode escapes** - Both `\xXX` and `\uXXXX` forms appear
7. **Memory addresses** - Object identity failures show `0x...` addresses
8. **Regex pattern failures** - Exception message mismatches show pattern and input
9. **Mixed pass/fail** - Output shows both "X failed" and "Y passed" counts
10. **"DID NOT RAISE" pattern** - Special failure for `pytest.raises` when no exception is raised

## Test Summary Format

Each pytest run ends with a summary line:
```
============================== 6 failed in 0.04s ===============================
```

Or for mixed results:
```
========================= 4 failed, 2 passed in 0.04s =========================
```

The parser should extract:
- Number of failures
- Number of passes (if any)
- Execution time
- Total test count (sum of failed + passed + skipped, etc.)
