# Pytest Assertion Output Flags Research

## Overview

This document researches pytest flags and options that provide detailed assertion information including line numbers, expected values, and actual values. This is pure research—no test runs were performed.

## Key Pytest Flags for Assertion Output

### 1. Verbosity Flags: `-v`, `-vv`, `-vvv`

**Purpose:** Control the verbosity level of pytest output.

**Behavior:**
- **`-v` / `--verbose`**: Increases verbosity, showing more detailed test session progress and assertion details when tests fail
- **`-vv`**: Higher verbosity level with even more detailed assertion details
- **`-vvv`**: Maximum verbosity level

**What they provide:**
- More detailed test session progress
- Enhanced assertion details when tests fail
- Fixture details with `--fixtures`
- At higher verbosity levels (`-vv`), less truncation occurs (default is ~240 characters in saferepr)
- More complete variable values in assertion failures

**Sources:**
- [Managing pytest's output - Official Documentation](https://docs.pytest.org/en/stable/how-to/output.html)
- [How to write and report assertions in tests](https://docs.pytest.org/en/stable/how-to/assert.html)

### 2. Traceback Flags: `--tb`

**Purpose:** Control the traceback print mode when tests fail.

**Available options:**
- **`--tb=auto`** (default): Automatically selects an appropriate traceback format
- **`--tb=long`**: Full, detailed traceback information
- **`--tb=short`**: Concise traceback (commonly recommended as default)
- **`--tb=line`**: Single line per failure with file/line number
- **`--tb=native`**: Python's standard traceback format (like `print_exc()`)
- **`--tb=no`**: Suppress traceback output entirely

**What they provide:**
- **`long`**: Full tracebacks with complete stack information
- **`short`**: Essential traceback information without excessive verbosity
- **`line`**: Quick overview showing only `{file}:{line}` per failure
- **`native`**: Standard Python traceback format

**Usage Examples:**
```bash
pytest --tb=short           # Concise traceback format
pytest --tb=long            # Full detailed traceback
pytest --tb=line            # Single line per failure with file:line
pytest --tb=no              # No traceback output
pytest --tb=native          # Python standard format
```

**Sources:**
- [Managing pytest's output - Official Documentation](https://docs.pytest.org/en/stable/how-to/output.html)
- [pytest API Reference - lineno](https://docs.pytest.org/en/stable/reference/reference.html)

### 3. Assertion Control Flags: `--assert`

**Purpose:** Control assertion rewriting behavior.

**Available options:**
- **`--assert=rewrite`** (default): Enables assertion introspection—pytest rewrites assert statements to provide detailed introspection information
- **`--assert=plain`**: Disables assertion debugging, uses standard Python assert

**What they provide:**
- **`rewrite` mode** (default): Automatic introspection that shows variable values, expected vs actual in assertion failures
- **`plain` mode**: Standard Python assert behavior (generic AssertionError with no context)

**Key insight:** pytest's assertion rewriting injects introspection information into assertion failure messages, unlike standard Python asserts that require explicit error messages.

**Sources:**
- [How to write and report assertions in tests](https://docs.pytest.org/en/stable/how-to/assert.html)
- [pytest 7.1.x Documentation on assertions](https://docs.pytest.org/en/7.1.x/how-to/assert.html)

### 4. Truncation Control Options

**Purpose:** Control how much of assertion output is truncated.

**Configuration options:**
- **`truncation_limit_lines`**: Controls line truncation limit
- **`truncation_limit_chars`**: Controls character truncation limit

**Default behavior:**
- Truncates assertion explanations at ~8 terminal lines by default
- At certain verbosity levels, output is truncated to 240 characters (hardcoded in `saferepr`)

**Override via command line:**
```bash
pytest -o truncation_limit_chars=500 -o truncation_limit_lines=20
```

**Configuration in `pytest.ini` or `pyproject.toml`:**
```toml
[pytest]
truncation_limit_lines = 10
truncation_limit_chars = 90
```

**Using `-vv` mode** can provide more verbose output with less truncation.

**Sources:**
- [Managing pytest's output - Official Documentation](https://docs.pytest.org/en/stable/how-to/output.html)
- [GitHub Issue #6267 - Avoid truncation](https://github.com/pytest-dev/pytest/issues/6267)

### 5. JUnit XML Output: `--junit-xml` / `--junitxml`

**Purpose:** Generate test results in JUnit-XML format.

**Behavior:**
- Generates XML reports that include line numbers for assertions
- The API reference specifically mentions "lineno – Line number of the assert statement"
- Conforms to JUnit XML standards for CI/CD integration

**Usage:**
```bash
pytest --junitxml=result.xml
pytest --junit-xml=result.xml
```

**Sources:**
- [_pytest.junitxml module source code](https://docs.pytest.org/en/stable/_modules/_pytest/junitxml.html)
- [pytest API Reference](https://docs.pytest.org/en/stable/reference/reference.html)

## What Information Each Flag Combination Provides

### Line Numbers
- **`--tb=line`**: Single line format showing `{file}:{line}` per failure
- **`--tb=long` / `--tb=short`**: Full tracebacks include line numbers in stack frames
- **`--junitxml`**: XML output includes `lineno` attribute for assertions

### Expected vs Actual Values
- **Assertion introspection** (enabled by `--assert=rewrite`, which is default): Automatically shows variable values and expected vs actual when assertions fail
- **`-vv` / `-vvv`**: More verbose output shows more complete values (less truncation)
- Higher verbosity levels reduce character truncation from default 240 chars

### Detailed Assertion Messages
- **`--tb=long`**: Most detailed traceback with complete context
- **`-vv`**: Enhanced assertion details with less truncation
- **`-o truncation_limit_chars=500`**: Override to show more content

## Recommended Flag Combinations to Test

Based on the research, here are candidate flag combinations to test for getting detailed assertion information:

### Combination 1: Maximum detail for console output
```bash
pytest -vv --tb=long
```
**Expected to provide:**
- Maximum verbosity with detailed assertion values
- Full tracebacks with line numbers
- Less truncation of assertion output

### Combination 2: Concise but informative
```bash
pytest -v --tb=short
```
**Expected to provide:**
- Moderate verbosity
- Concise tracebacks
- Essential assertion information without excessive output

### Combination 3: Quick scan with file:line
```bash
pytest --tb=line
```
**Expected to provide:**
- Quick overview showing `{file}:{line}` per failure
- Minimal but precise failure location

### Combination 4: Machine-readable with line numbers
```bash
pytest --junitxml=result.xml
```
**Expected to provide:**
- Structured XML output
- Line numbers via `lineno` attribute
- All assertion details in parseable format

### Combination 5: Override truncation for long strings
```bash
pytest -vv -o truncation_limit_chars=1000 -o truncation_limit_lines=50 --tb=long
```
**Expected to provide:**
- Maximum verbosity
- Custom truncation limits to show more content
- Full detailed tracebacks

### Combination 6: Disable assertion introspection (for comparison)
```bash
pytest --assert=plain
```
**Expected to provide:**
- Standard Python assert behavior
- No introspection (generic AssertionError)
- Useful as a baseline to understand what assertion rewriting adds

## Summary Table

| Flag/Option | Purpose | Line Numbers | Expected/Actual Values | Detail Level |
|-------------|---------|--------------|------------------------|--------------|
| `-v` | Verbosity level 1 | ✓ | ✓ (limited by truncation) | Medium |
| `-vv` | Verbosity level 2 | ✓ | ✓ (less truncation) | High |
| `-vvv` | Verbosity level 3 | ✓ | ✓ (minimal truncation) | Very High |
| `--tb=long` | Full traceback | ✓ (in stack frames) | ✓ (via introspection) | High |
| `--tb=short` | Concise traceback | ✓ (in stack frames) | ✓ (via introspection) | Medium |
| `--tb=line` | Single line failure | ✓ (`{file}:{line}`) | ✗ | Low |
| `--tb=native` | Python traceback | ✓ (in stack frames) | ✗ (no introspection) | Medium |
| `--assert=rewrite` | Enable introspection (default) | - | ✓ | - |
| `--assert=plain` | Disable introspection | - | ✗ | - |
| `--junitxml` | XML output | ✓ (in `lineno` attr) | ✓ (in failure message) | Parseable |
| `-o truncation_limit_chars=N` | Override char truncation | - | ✓ (more content) | Custom |
| `-o truncation_limit_lines=N` | Override line truncation | - | ✓ (more content) | Custom |

## Key Takeaways

1. **Default behavior is good**: By default, pytest uses `--assert=rewrite`, which provides automatic introspection showing expected vs actual values.

2. **Verbosity matters**: Increasing verbosity (`-v`, `-vv`, `-vvv`) provides more complete assertion values by reducing truncation.

3. **Traceback style controls format**: `--tb=long` for full detail, `--tb=short` for concise output, `--tb=line` for quick `{file}:{line}` overview.

4. **Line numbers are always available**: Any traceback mode shows line numbers; `--tb=line` provides the most compact `{file}:{line}` format.

5. **Truncation can be overridden**: Use `-o truncation_limit_chars=N` and `-o truncation_limit_lines=N` to show more content in assertion failures.

6. **Machine-readable option**: `--junitxml` provides structured XML output with line numbers and assertion details.

## Next Steps

To validate this research, run actual pytest tests with the recommended flag combinations and observe:
1. What assertion information is actually displayed
2. How line numbers are formatted
3. The completeness of expected vs actual values
4. The effect of truncation overrides

**Sources:**
- [Managing pytest's output - Official Documentation](https://docs.pytest.org/en/stable/how-to/output.html)
- [How to write and report assertions in tests](https://docs.pytest.org/en/stable/how-to/assert.html)
- [pytest 7.1.x Documentation on assertions](https://docs.pytest.org/en/7.1.x/how-to/assert.html)
- [pytest API Reference](https://docs.pytest.org/en/stable/reference/reference.html)
- [_pytest.junitxml module source code](https://docs.pytest.org/en/stable/_modules/_pytest/junitxml.html)
- [Demo of Python failure reports with pytest](https://docs.pytest.org/en/stable/example/reportingdemo.html)
- [GitHub Issue #6267 - Avoid truncation](https://github.com/pytest-dev/pytest/issues/6267)
- [GitHub Issue #8403 - Truncation behavior](https://github.com/pytest-dev/pytest/issues/8403)
- [GitHub Issue #9318 - pytest.main assertion output](https://github.com/pytest-dev/pytest/issues/9318)
- [GitHub Discussion #14674 - Confused about diff output](https://github.com/pytest-dev/pytest/discussions/14674)
- [GitHub Issue #8850 - Summary report line number formatting](https://github.com/pytest-dev/pytest/issues/8850)
