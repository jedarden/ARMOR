# Minimal Failing Test Case - Pytest Flags Evaluation

This document describes the `test_pytest_flags_minimal.py` test file and how to use it to evaluate different pytest flags.

## Purpose

This test file provides a **controlled testbed** for evaluating how different pytest flags display assertion failures. Each test is designed to fail with specific assertion patterns, allowing you to compare:

- Default output vs verbose output
- Different traceback formats (`--tb=short`, `--tb=long`, `--tb=line`, `--tb=no`)
- Assertion rewriting options (`--assert=plain`, `--assert=rewrite`)
- Output verbosity levels (`-v`, `-vv`, `-vvv`, `-q`, `--quiet`)
- Other display flags (`--showlocals`, `--full-trace`)

## Test File Location

`/home/coding/ARMOR/test_pytest_flags_minimal.py`

## Running the Tests

### Basic runs (default behavior)
```bash
# Run with default settings
pytest test_pytest_flags_minimal.py

# Run with verbose output
pytest test_pytest_flags_minimal.py -v

# Run with very verbose output
pytest test_pytest_flags_minimal.py -vv
```

### Traceback format comparisons
```bash
# Short traceback (concise)
pytest test_pytest_flags_minimal.py --tb=short

# Long traceback (detailed)
pytest test_pytest_flags_minimal.py --tb=long

# Line-only traceback (one line per failure)
pytest test_pytest_flags_minimal.py --tb=line

# No traceback (just summary)
pytest test_pytest_flags_minimal.py --tb=no
```

### Assertion rewriting comparisons
```bash
# Use assertion rewriting (default, shows diffs)
pytest test_pytest_flags_minimal.py --assert=rewrite

# Disable assertion rewriting (no diffs)
pytest test_pytest_flags_minimal.py --assert=plain
```

### With additional context
```bash
# Show local variables in tracebacks
pytest test_pytest_flags_minimal.py --showlocals

# Full traceback including earlier frames
pytest test_pytest_flags_minimal.py --full-trace

# Quiet mode (minimal output)
pytest test_pytest_flags_minimal.py -q

# Extra-quiet mode (only show failures)
pytest test_pytest_flags_minimal.py -qq
```

### Combined flags for comprehensive testing
```bash
# Verbose + short tracebacks
pytest test_pytest_flags_minimal.py -v --tb=short

# Verbose + long tracebacks + showlocals
pytest test_pytest_flags_minimal.py -vv --tb=long --showlocals

# Quiet mode + line tracebacks
pytest test_pytest_flags_minimal.py -q --tb=line
```

## Expected Failure Patterns

| Test | Type | What It Shows |
|------|------|---------------|
| `test_simple_equality` | Basic equality | Simple string/number diff |
| `test_dict_equality` | Dictionary | Key/value structure diffs |
| `test_list_comparison` | List | Element-wise diffs, indices |
| `test_multiline_string` | Text | Line-by-line diff |
| `test_numeric_comparison` | Numbers | Comparison operator failures |
| `test_in_operator` | Membership | What's missing from collections |
| `test_long_sequence` | Longer list | How diffs scale with sequence length |
| `test_nested_structure` | Nested data | Hierarchical diffs, path to mismatch |
| `test_approximate_numbers` | Floats | Precision diffs, floating point issues |
| `test_boolean_logic` | Boolean | Logic evaluation failures |
| `test_string_operations` | Strings | String method comparison failures |

## Use Cases

### 1. Comparing assertion output formats
```bash
# Compare plain vs rewritten assertions
pytest test_pytest_flags_minimal.py --assert=plain > output_plain.txt
pytest test_pytest_flags_minimal.py --assert=rewrite > output_rewrite.txt
diff output_plain.txt output_rewrite.txt
```

### 2. Evaluating traceback verbosity
```bash
# Compare all traceback modes
for tb_mode in short long line no; do
    echo "=== --tb=$tb_mode ===" > tb_$tb_mode.log
    pytest test_pytest_flags_minimal.py --tb=$tb_mode >> tb_$tb_mode.log 2>&1
done
```

### 3. Testing output flags
```bash
# Compare verbosity levels
for level in q qq v vv vvv; do
    echo "=== -$level ===" > verbose_$level.log
    pytest test_pytest_flags_minimal.py -$level >> verbose_$level.log 2>&1
done
```

## Isolation Note

This test file is **isolated and self-contained**:
- No imports from the project codebase
- No dependencies on external modules (except `pytest` and `math` from stdlib)
- Will not affect other tests in the project
- Safe to run in any environment

## Documentation of Failures

Each test in the file includes:
- A docstring describing what type of assertion it tests
- Comments explaining why it fails
- Expected vs actual values clearly shown

This makes it easy to:
1. Understand what each test is checking
2. Predict what the failure output should look like
3. Verify that pytest flags are working as expected

## Example Output Comparison

### With `--assert=rewrite` (default):
```
AssertionError: assert [1, 2, 0, 4, 6] == [1, 2, 3, 4, 5]
  At index 2 diff: 0 != 3
  At index 4 diff: 6 != 5
  Full diff:
  - [1, 2, 3, 4, 5]
  ?        ^     ^
  + [1, 2, 0, 4, 6]
  ?        ^     ^
```

### With `--assert=plain`:
```
AssertionError
```

The difference is dramatic - this is why testing different flags is important for understanding how pytest displays assertion failures.
