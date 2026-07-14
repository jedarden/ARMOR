# Pytest Assertion Output Flags Research

## Task: Research pytest assertion output flags

Bead ID: bf-1rgu22
Date: 2026-07-13

## Overview

This document summarizes pytest command-line options and flags that provide detailed assertion information including line numbers, expected values, and actual values.

## Core Verbosity Flags

### `-v` / `--verbose`
- **Purpose**: Controls verbosity of pytest output in multiple aspects
- **Effects**:
  - Test session progress
  - Assertion details when tests fail
  - Fixture details when used with other options
- **Use case**: Standard verbose output for debugging

### `-vv` (Double Verbose)
- **Purpose**: Even more verbose output than `-v`
- **Effects**:
  - Maximum detail in tracebacks and test output
  - Additional context for assertions
- **Use case**: Deep debugging when you need maximum information

Sources:
- [Managing pytest's output](https://docs.pytest.org/en/stable/how-to/output.html)
- [My Most Used pytest Commandline Flags](https://adamj.eu/tech/2019/10/03/my-most-used-pytest-commandline-flags/)

## Traceback Format Flags (`--tb`)

The `--tb` argument controls the traceback format:

### `--tb=auto` (Default)
- Standard behavior, equivalent to `short` in modern pytest

### `--tb=long`
- Shows full detailed traceback entries for the entire stack
- Displays complete function context and source code for each stack frame
- More verbose - useful for deep debugging
- Was the default in older pytest versions (before 2.6.0)

### `--tb=short` (Default in modern pytest)
- Condensed traceback format
- Only displays the first entry (test function) and the last entry (failure location) in full detail
- Intermediate entries shown in compact format
- More familiar to Python developers

### `--tb=line`
- Even shorter - shows just a single line per failure
- Quick overview of test failures

### `--tb=native`
- Uses Python's standard traceback formatting
- No pytest enhancements

### `--tb=no`
- Suppresses traceback output entirely
- Only pass/fail status

### `--full-trace`
- Shows even more detailed traces than `--tb=long`
- Includes KeyboardInterrupt stack traces

Sources:
- [Managing pytest's output](https://docs.pytest.org/en/stable/how-to/output.html)

## Local Variable Display Flags

### `-l` / `--showlocals`
- **Purpose**: Shows local variables in tracebacks
- **Effects**:
  - Displays values of all local variables in stack trace
  - Shows full context where assertion failed
  - Helps debug by revealing actual state at moment of failure
- **Aliases**: `-l` is shorthand for `--showlocals`

Sources:
- [Managing pytest's output](https://docs.pytest.org/en/stable/how-to/output.html)
- [Stack Overflow: show full variables values on error](https://stackoverflow.com/questions/52986691/python-pytest-show-full-variable-values-on-error-no-minimization)

## Output Capture Flags

### `-s` / `--capture=no`
- **Purpose**: Shows print statements and output in real-time
- **Effects**: Disables pytest's output capturing
- **Use case**: Debugging with print statements

Source: [Managing pytest's output](https://docs.pytest.org/en/stable/how-to/output.html)

## Assertion Rewriting Control

### `--assert=MODE`
Controls pytest's assertion rewriting behavior:

- **`--assert=rewrite`**: Enable assertion rewriting (default)
  - Rewrites assert statements to provide introspection information
  - Shows expected vs actual values automatically
  
- **`--assert=plain`**: Disable assertion rewriting
  - Uses standard Python assert behavior
  - No introspection or detailed output

### `PYTEST_DONT_REWRITE` (Docstring Marker)
- **NOT an environment variable** - it's a module docstring marker
- **Purpose**: Disable assertion rewriting for specific modules
- **Usage**:
```python
"""
PYTEST_DONT_REWRITE
"""
def test_something():
    # This module's assertions won't be rewritten
    assert something
```

### `PYTEST_ADDOPTS` (Environment Variable)
- **Purpose**: Add command-line options to pytest via environment
- **Usage**:
```bash
export PYTEST_ADDOPTS="-v --tb=long"
pytest
```

Sources:
- [How to write and report assertions in tests](https://docs.pytest.org/en/7.1.x/how-to/assert.html)
- [API Reference](https://docs.pytest.org/en/stable/reference/reference.html)
- [GitHub Issue: Assertion Rewriting](https://github.com/pytest-dev/pytest/issues/2203)

## Assertion Output Format Details

### How Pytest Displays Assertion Failures

Pytest automatically provides detailed assertion information when tests fail:

1. **Line Numbers**: Always shown in traceback
2. **Expected vs Actual Values**: Shown via assertion rewriting
3. **Diff Format**: Uses `-` for expected (missing from actual) and `+` for actual (extra in actual)
4. **Introspection**: Automatic display of variable values in assertions

### Diff Output Symbols
- **`-`**: Expected value (present in expected, missing from actual)
- **`+`**: Actual value (present in actual, missing from expected)

Example pattern with `assert actual == expected`:
```python
# With assertion rewriting enabled:
assert actual == expected
# Fails with diff showing:
#   - expected_value
#   + actual_value
```

Sources:
- [GitHub Discussion: diff output of assert failures](https://github.com/pytest-dev/pytest/discussions/14674)
- [Demo of Python failure reports with pytest](https://docs.pytest.org/en/stable/example/reportingdemo.html)

## Structured Output Options

### JUnit XML (Built-in)
- **Flag**: `--junit-xml=path/to/result.xml`
- **Purpose**: Machine-readable XML output
- **Use case**: CI/CD integration, test reporting systems

### pytest-json-report (Plugin)
- **Plugin**: `pytest-json-report`
- **Purpose**: JSON format test reports
- **Features**:
  - Structured output for machine processing
  - Summary and detailed test information
  - Assertion failure details in JSON

Sources:
- [pytest-json-report on PyPI](https://pypi.org/project/pytest-json-report/)

## Candidate Flag Combinations to Test

Based on research, these flag combinations should provide maximum assertion detail:

### Maximum Detail
```bash
pytest -vv --tb=long -l
# -vv: Maximum verbosity
# --tb=long: Full detailed tracebacks
# -l: Show local variables
```

### Standard Detail
```bash
pytest -v --tb=short -l
# -v: Standard verbosity
# --tb=short: Condensed but detailed tracebacks
# -l: Show local variables
```

### Quick Overview
```bash
pytest -v --tb=line
# -v: Standard verbosity
# --tb=line: Single line per failure
```

### Machine-Readable
```bash
pytest -vv --tb=long -l --junit-xml=results.xml
# Same as maximum detail, plus XML output
```

## Summary

**Flags that capture line numbers, expected, and actual values:**

1. **Line numbers**: All traceback modes (`--tb=long/short/line/native/auto`)
2. **Expected vs Actual**: Assertion rewriting (`--assert=rewrite`, default)
3. **Detailed context**: `-v`, `-vv`, `-l`/`--showlocals`
4. **Full tracebacks**: `--tb=long`, `--full-trace`

**Recommended default combination:**
```bash
pytest -vv --tb=long -l
```

This provides the most comprehensive assertion output for debugging.
