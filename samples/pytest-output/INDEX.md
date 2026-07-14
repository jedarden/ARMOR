# Pytest Output Examples - Index

This directory contains pytest output examples for various failure scenarios, useful for parser development and testing.

## Quick Reference

| Category | Test File | Output File | Failure Count | Key Features |
|----------|-----------|-------------|---------------|--------------|
| Simple Assertions | `test_simple_assertions.py` | `output_simple_assertions.txt` | 6 | Basic assert patterns, type checks, pytest.raises |
| Comparison Failures | `test_comparison_failures.py` | `output_comparison_failures.txt` | 6 | Numeric, string, float comparisons, len(), dict key checks |
| Multiline Diffs | `test_multiline_diffs.py` | `output_multiline_diffs.txt` | 5 | Text diffs, list/dict comparisons, nested structures |
| Exception Failures | `test_exception_failures.py` | `output_exception_failures.txt` | 4 | Unexpected exceptions, wrong type, message mismatches |
| Edge Cases | `test_edge_cases.py` | `output_edge_cases.txt` | 8 | None/False, empty values, identity, unicode, bytes |

## Extended Examples (Single Test)

| Test File | Output File | Feature Demonstrated |
|-----------|-------------|---------------------|
| `test_simple_assertions.py::test_simple_assertion_equal` | `output_simple_assertion_long_tb.txt` | Long traceback format (`--tb=long`) |
| `test_multiline_diffs.py::test_dict_comparison` | `output_dict_comparison_verbose.txt` | Verbose diff output (`-vv`) |
| `test_exception_failures.py::test_unexpected_exception` | `output_exception_auto_tb.txt` | Auto traceback format (`--tb=auto`) |

## Viewing Examples

To view any example:
```bash
cat output_simple_assertions.txt
```

To regenerate an example:
```bash
nix-shell -p python3Packages.pytest --run "pytest test_simple_assertions.py -v --tb=short"
```

To generate new examples with different options:
```bash
# Verbose output
pytest -vv

# Long traceback
pytest --tb=long

# No traceback (summary only)
pytest --tb=no

# Line-style traceback
pytest --tb=line
```

## Failure Type Reference

### Assertion Patterns
- `==` - Equality comparison
- `is` - Identity comparison
- `in` - Membership test
- `isinstance()` - Type checking
- `>`, `>=`, `<`, `<=` - Numeric comparisons

### Exception Patterns
- `ZeroDivisionError` - Division by zero
- `ValueError` - Invalid value or type
- `TypeError` - Type mismatch
- `RuntimeError` - General runtime error
- "DID NOT RAISE" - Expected exception not raised

### Diff Patterns
- Unified diff format (`-` expected, `+` actual)
- Index markers ("At index N diff")
- Omission notices ("Omitting N identical items")
- Truncation notices ("Full output truncated")

## File Sizes
- `output_simple_assertions.txt`: ~1.4 KB
- `output_comparison_failures.txt`: ~1.7 KB
- `output_multiline_diffs.txt`: ~2.1 KB
- `output_exception_failures.txt`: ~1.6 KB
- `output_edge_cases.txt`: ~2.2 KB
- `output_simple_assertion_long_tb.txt`: ~870 B
- `output_dict_comparison_verbose.txt`: ~1.7 KB
- `output_exception_auto_tb.txt`: ~850 B

Total: ~12.4 KB of pytest output examples

## See Also
- `README.md` - Detailed documentation with parser implementation notes
