# Pytest Assertion Flags Research

**Bead:** bf-1wim6c  
**Date:** 2026-07-13

## Objective

Research and identify pytest flags/options that enable detailed assertion output showing line numbers, expected values, and actual values.

---

## Findings

### 1. Verbosity Options

| Flag | Description |
|------|-------------|
| `-v` / `--verbose` | Increases verbosity of test output, including test session progress and assertion details when tests fail |
| `-vv` | Extra verbose mode (can produce hundreds of lines per failure message) |

**Recommended for assertion details:** `-v` provides assertion details when tests fail.

---

### 2. Traceback Options (`--tb`)

The `--tb` flag controls how tracebacks are displayed when tests fail.

| Option | Description |
|--------|-------------|
| `--tb=auto` | Default mode - displays long tracebacks for first and last entry only |
| `--tb=long` | Displays **full detailed tracebacks** with complete assertion information |
| `--tb=short` | Displays shorter, more concise tracebacks |
| `--tb=line` | Displays one-line traceback format |
| `--tb=no` | Disables traceback output entirely |

**Recommended for detailed assertion info:** `--tb=long` provides the most detailed assertion output.

---

### 3. Additional Output Options

| Flag | Description |
|------|-------------|
| `--full-trace` | Causes very long traces to be printed on error (even longer than `--tb=long`) |
| `--showlocals` / `-l` | Shows local variables in tracebacks |
| `-s` / `--capture=no` | Shows print statements and other output during test execution |

---

### 4. Recommended Flag Combinations

#### For Detailed Assertion Output (Console)
```bash
pytest -v --tb=long
```
or for maximum detail:
```bash
pytest -v --tb=long --showlocals
```

This combination provides:
- Verbose test progress
- Full detailed tracebacks
- Expected vs actual values in assertions
- Line numbers
- Local variable values (with `--showlocals`)

#### For Concise Assertion Details
```bash
pytest -v --tb=short
```

#### For Maximum Detail (including KeyboardInterrupt traces)
```bash
pytest -vv --tb=long --full-trace --showlocals
```

---

### 5. JSON / Structured Output Options

pytest does **not** have built-in JSON output via a simple flag. Structured output requires plugins.

#### pytest-json-report (Most Popular)

**Installation:**
```bash
pip install pytest-json-report
```

**Usage:**
```bash
pytest --json-report --json-report-file=report.json
```

**Additional Options:**
- `--json-report-omit=...` - Exclude certain data from the report
- `--json-report-indent=N` - Control JSON formatting indentation (default: 4)

**Report Contents:**
- Test summary
- Test details
- Captured output
- Logs
- Exception tracebacks
- Assertion details

#### Alternative: pytest-json

**Installation:**
```bash
pip install pytest-json
```

**Usage:**
```bash
pytest --json=report.json
```

#### CTRF Format (Common Test Report Format)

A pytest plugin that generates JSON reports in CTRF format.

---

## Sources

- [pytest documentation on assertions](https://docs.pytest.org/en/stable/how-to/assert.html)
- [Managing pytest's output](https://docs.pytest.org/en/stable/how-to/output.html)
- [Stack Overflow: Show full variable values on error](https://stackoverflow.com/questions/52986691/python-pytest-show-full-variables-values-on-error-no-minimization)
- [pytest-json-report on PyPI](https://pypi.org/project/pytest-json-report/)

---

## Summary

For **detailed console output** showing expected vs actual values with line numbers, the recommended combination is:

```bash
pytest -v --tb=long
```

For **structured/JSON output** for programmatic processing, use:

```bash
pytest --json-report --json-report-file=report.json
```

(after installing `pytest-json-report`)

---

## Related Workspace Learnings

This research relates to bead `bf-1rgu22` (pytest assertion output flags research).
