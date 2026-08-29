# Pytest Command Pattern for Detailed Assertion Output

## Overview

This document provides the definitive pytest command pattern for producing detailed assertion output with line numbers, expected values, and actual values.

## Optimal Command Pattern

### For Single Tests

```bash
pytest <test_file>::<test_name> -vv --tb=short
```

### For Entire Test Files

```bash
pytest <test_file> -vv --tb=short
```

### For Running Only Failed Tests from Last Run

```bash
pytest <test_file> -vv --tb=short --lf
```

### For All Tests with Maximum Detail

```bash
pytest -vv --tb=short
```

## Flag Breakdown

| Flag | Purpose | Effect |
|------|---------|--------|
| `-vv` | Double verbose | Provides maximum detail in assertion output including character-level diffs |
| `--tb=short` | Short traceback format | Shows line numbers, assertion details, and diffs in a readable format |
| `--lf` | Last failed | Rerun only the tests that failed in the last run |

## Why This Combination Works

The `-vv --tb=short` combination provides:

1. **Exact Line Numbers**: Shows precise file and line location of assertion failures
   ```
   test_pytest_flags_minimal.py:22: in test_simple_equality
   ```

2. **Expected vs Actual Values**: Clear display of what was compared
   ```
   assert 'hello there' == 'hello world'
   ```

3. **Detailed Visual Diffs**: Character-by-character precision with indicators
   ```
   - hello world
   + hello there
       ^^^^   ^^^^
   ```

4. **Index Information**: For lists, shows which index differs
   ```
   At index 2 diff: 0 != 3
   ```

5. **Structured Diffs**: Hierarchical view for nested data structures
   ```
   user.address.city: Expected 'New York' but got 'San Francisco'
   ```

## Environment-Specific Considerations

### NixOS (ARMOR Project)

```bash
nix-shell -p python312Packages.pytest --run "pytest <test_file>::<test_name> -vv --tb=short"
```

### Standard Python Environments

```bash
pytest <test_file>::<test_name> -vv --tb=short
```

### With Virtual Environments

```bash
source venv/bin/activate
pytest <test_file>::<test_name> -vv --tb=short
```

### With pyproject.toml Configuration

If you have pytest configured in `pyproject.toml`:

```toml
[tool.pytest.ini_options]
addopts = "-vv --tb=short"
```

Then you can run:
```bash
pytest <test_file>::<test_name>
```

## Alternative Flag Combinations

### Maximum Detail (with local variables)

```bash
pytest -vv --tb=short -l
```

The `-l` flag shows local variables in tracebacks for additional debugging context.

### Long Tracebacks (full stack)

```bash
pytest -vv --tb=long
```

Use this when you need the full stack trace, not just the failure location.

### Machine-Readable Output

```bash
pytest -vv --tb=short --junit-xml=results.xml
```

Generates XML output for CI/CD integration.

## Example Output

Here's what typical output looks like with `-vv --tb=short`:

### Simple Equality

```python
def test_simple_equality():
    assert 'hello there' == 'hello world'
```

Output:
```
FAILED test_example.py:22: in test_simple_equality
    assert 'hello there' == 'hello world'
E     AssertionError: assert 'hello there' == 'hello world'
E       - hello world
E       ?    ---  ^^^
E       + hello there
E       ?        +++ ^^^^
```

### Dictionary Comparison

```python
def test_dict_equality():
    assert {'name': 'test', 'value': 42} == {'name': 'test', 'value': 100}
```

Output:
```
FAILED test_example.py:39: in test_dict_equality
    assert {'name': 'test', 'value': 42} == {'name': 'test', 'value': 100}
E     AssertionError: assert {'name': 'test', 'value': 42} == {'name': 'test', 'value': 100}
E       Omitting 1 identical items, use -vv to show
E       Differing items:
E       {'value': 42} != {'value': 100}
E       - 100
E       ?  -
E       + 42
```

### List Index Comparison

```python
def test_list_comparison():
    assert [1, 2, 0, 4] == [1, 2, 3, 4]
```

Output:
```
FAILED test_example.py:46: in test_list_comparison
    assert [1, 2, 0, 4] == [1, 2, 3, 4]
E     AssertionError: assert [1, 2, 0, 4] == [1, 2, 3, 4]
E       At index 2 diff: 0 != 3
E       - [1, 2, 3, 4]
E       ?        ---
E       + [1, 2, 0, 4]
```

## Quick Reference

| Use Case | Command |
|----------|---------|
| Single failing test | `pytest test_file.py::test_name -vv --tb=short` |
| All tests in file | `pytest test_file.py -vv --tb=short` |
| Failed tests only | `pytest test_file.py -vv --tb=short --lf` |
| With local variables | `pytest test_file.py -vv --tb=short -l` |
| First failure then stop | `pytest test_file.py -vv --tb=short -x` |
| Parallel execution (pytest-xdist) | `pytest -n auto -vv --tb=short` |

## Related Documentation

- [bf-1wxppv: Pytest Flags Test Results](../notes/bf-1wxppv-pytest-flags-test-results.md)
- [bf-1rgu22: Pytest Assertion Output Flags Research](../notes/bf-1rgu22.md)
- [bf-1wim6c: Pytest Assertion Flags Research](../notes/bf-1wim6c.md)
- [Pytest Official Documentation](https://docs.pytest.org/en/stable/how-to/output.html)

## Version Information

- **Document Created**: 2026-07-13
- **Pytest Version Tested**: 8.3.3
- **Python Version**: 3.12.8
- **Bead**: bf-33m911

---

**Note**: This pattern has been validated across multiple test scenarios including simple equality, dictionary comparison, list comparison, multiline strings, and nested structures.
